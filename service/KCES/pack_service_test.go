package KCES

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

func TestPackServicePackToAbaAndCtUsesPureDirectoryContract(t *testing.T) {
	parent := t.TempDir()
	input := filepath.Join(parent, "input")
	if err := os.MkdirAll(filepath.Join(input, "TextAsset"), 0755); err != nil {
		t.Fatal(err)
	}
	assetRelPath := filepath.Join("TextAsset", "sample.menuassets")
	if err := os.WriteFile(filepath.Join(input, assetRelPath), []byte("payload"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(input, assetRelPath+".meta.json"), []byte(`{"pathId":42,"loadName":"assets/old/path"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(input, assetRelPath+".typetree.json"), []byte(`not used`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := (&PackService{}).PackToAbaAndCt(input, "pure_pack"); err != nil {
		t.Fatal(err)
	}
	bundle, af := readPackedAbaForTest(t, filepath.Join(parent, "pure_pack.aba"))
	if bundle.Header.Version != defaultKCESAbaVersion || bundle.Header.EngineVersion != defaultKCESUnityVersion || bundle.Header.GenerationVersion != defaultKCESGenerationVersion {
		t.Fatalf("UnityFS contract = version %d generation %q engine %q", bundle.Header.Version, bundle.Header.GenerationVersion, bundle.Header.EngineVersion)
	}
	if len(bundle.BlockInfo.DirectoryInfos) != 1 || !bundle.BlockInfo.DirectoryInfos[0].IsSerialized() {
		t.Fatalf("ABA directory = %+v", bundle.BlockInfo.DirectoryInfos)
	}
	if af.Header.Version != supportedSerializedFileVersion || af.Metadata.UnityVersion != defaultKCESUnityVersion || af.Metadata.TargetPlatform != defaultKCESTargetPlatform {
		t.Fatalf("SerializedFile contract = version %d Unity %q platform %d", af.Header.Version, af.Metadata.UnityVersion, af.Metadata.TargetPlatform)
	}
	wantIDs, err := buildCanonicalPathIDs([]string{filepath.ToSlash(assetRelPath)})
	if err != nil {
		t.Fatal(err)
	}
	wantID := wantIDs[strings.ToLower(filepath.ToSlash(assetRelPath))]
	var textInfo *aba.AssetInfo
	for infoIndex := range af.Metadata.AssetInfos {
		if af.Metadata.AssetInfos[infoIndex].TypeId == aba.ClassIDTextAsset {
			textInfo = &af.Metadata.AssetInfos[infoIndex]
		}
	}
	if textInfo == nil || textInfo.PathId != wantID || textInfo.PathId == 42 {
		t.Fatalf("TextAsset info = %+v, want canonical PathID %d", textInfo, wantID)
	}
	container, err := af.GetAssetBundleContainerMap()
	if err != nil {
		t.Fatal(err)
	}
	if container[wantID] != "sample.menuassets" {
		t.Fatalf("m_Container[%d] = %q", wantID, container[wantID])
	}

	catalogFile, err := os.Open(filepath.Join(parent, "pure_pack.ct"))
	if err != nil {
		t.Fatal(err)
	}
	table, err := ct.ReadContentTable(catalogFile)
	catalogFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := ct.DecodeCatalogFromCt(table)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Items) != 1 || testStringValue(catalog.Items[0].Name) != "sample.menuassets" {
		t.Fatalf("catalog items = %+v", catalog.Items)
	}
}

func TestPackServiceRejectsExternalStreamFiles(t *testing.T) {
	parent := t.TempDir()
	input := filepath.Join(parent, "input")
	if err := os.Mkdir(input, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(input, "sample.menuassets"), []byte("payload"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"CAB-generated.resS", "voice.resource", "data.resources"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(input, name)
			if err := os.WriteFile(path, []byte("external stream"), 0644); err != nil {
				t.Fatal(err)
			}
			err := (&PackService{}).PackToAbaAndCt(input, "stream_rejected")
			if err == nil || !strings.Contains(err.Error(), "stream payloads must be inlined") {
				t.Fatalf("PackToAbaAndCt error = %v", err)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			assertPackPairAbsent(t, parent, "stream_rejected")
		})
	}
}

func TestPackServiceInfersCanonicalRawObjectKinds(t *testing.T) {
	tests := []struct {
		path string
		name string
		kind string
	}{
		{path: "Texture2D/sample.tex.bytes", name: "sample.tex", kind: "rawtexture2d"},
		{path: "Mesh/sample.mmesh.bytes", name: "sample.mmesh", kind: "mesh"},
		{path: "Sprite/sample.sprite.bytes", name: "sample", kind: "sprite"},
		{path: "SpriteAtlas/sample.partsatlas.bytes", name: "sample.partsatlas", kind: "spriteatlas"},
		{path: "AnimationClip/sample.anm.bytes", name: "sample.anm", kind: "animationclip"},
		{path: "sample.monoscript.bytes", name: "sample", kind: "monoscript"},
		{path: "sample.monobehaviour.bytes", name: "sample", kind: "monobehaviour"},
		{path: "AudioClip/sample.bytes", name: "sample", kind: "audioclip"},
	}
	for _, test := range tests {
		if got := inferAssetNameForPack(test.path); got != test.name {
			t.Errorf("inferAssetNameForPack(%q) = %q, want %q", test.path, got, test.name)
		}
		if got := inferKindForPack(test.name, test.path); got != test.kind {
			t.Errorf("inferKindForPack(%q, %q) = %q, want %q", test.name, test.path, got, test.kind)
		}
	}
}

func readPackedAbaForTest(t *testing.T, path string) (*aba.Aba, *aba.AssetsFile) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := aba.ReadAba(f)
	if err != nil {
		f.Close()
		t.Fatal(err)
	}
	serialized, err := bundle.GetFileData(0)
	if err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	af, err := aba.ReadAssetsFile(serialized)
	if err != nil {
		t.Fatal(err)
	}
	return bundle, af
}
