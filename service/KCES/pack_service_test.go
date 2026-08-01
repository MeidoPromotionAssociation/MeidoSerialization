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
		{path: "Texture2D/sample.tex", name: "sample.tex", kind: "rawtexture2d"},
		{path: "Texture2D/sample.texture2d", name: "sample", kind: "rawtexture2d"},
		{path: "Mesh/sample.mmesh", name: "sample.mmesh", kind: "mesh"},
		{path: "Sprite/sample.sprite", name: "sample", kind: "sprite"},
		{path: "SpriteAtlas/sample.partsatlas", name: "sample.partsatlas", kind: "spriteatlas"},
		{path: "SpriteAtlas/sample.partsassets", name: "sample.partsassets", kind: "spriteatlas"},
		{path: "AnimationClip/sample.anm", name: "sample.anm", kind: "animationclip"},
		{path: "Material/sample.material", name: "sample", kind: "material"},
		{path: "AudioClip/sample.audioclip", name: "sample", kind: "audioclip"},
		{path: "MonoBehaviour/sample.monobehaviour", name: "sample", kind: "monobehaviour"},
		{path: "Type_95/sample.bytes", name: "sample", kind: "type_95"},
	}
	for _, test := range tests {
		if got := inferAssetNameForPack(test.path); got != test.name {
			t.Errorf("inferAssetNameForPack(%q) = %q, want %q", test.path, got, test.name)
		}
		if got := inferKindForPack(test.name, test.path); got != test.kind {
			t.Errorf("inferKindForPack(%q, %q) = %q, want %q", test.name, test.path, got, test.kind)
		}
		if !isNativeUnityObjectPackPath(test.path, test.kind) {
			t.Errorf("isNativeUnityObjectPackPath(%q, %q) = false", test.path, test.kind)
		}
	}
}

func TestPackServiceSkipsDerivedNativeObjectArtifacts(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		primary string
		derived string
	}{
		{primary: "Texture2D/sample.tex", derived: "Texture2D/sample.png"},
		{primary: "Texture2D/sample.tex", derived: "Texture2D/sample.dds"},
		{primary: "Mesh/sample.mmesh", derived: "Mesh/sample.glb"},
		{primary: "Sprite/sample.sprite", derived: "Sprite/sample.png"},
		{primary: "AnimationClip/sample.anm", derived: "AnimationClip/sample.gltf"},
		{primary: "AudioClip/sample.audioclip", derived: "AudioClip/sample.ogg"},
		{primary: "Material/sample.material", derived: "Material/sample.material.json"},
		{primary: "Type_95/sample.bytes", derived: "Type_95/sample.bytes.json"},
	}
	for _, test := range tests {
		primary := filepath.Join(root, filepath.FromSlash(test.primary))
		derived := filepath.Join(root, filepath.FromSlash(test.derived))
		if err := os.MkdirAll(filepath.Dir(primary), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(primary, []byte("primary"), 0644); err != nil {
			t.Fatal(err)
		}
		if !shouldSkipDerivedPackInput(derived) {
			t.Errorf("derived artifact %q was not skipped for %q", test.derived, test.primary)
		}
		if err := os.Remove(primary); err != nil {
			t.Fatal(err)
		}
		if shouldSkipDerivedPackInput(derived) {
			t.Errorf("derived artifact %q was skipped without its primary file", test.derived)
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

func TestPackDefaultOutputNameStripsUnpackSuffix(t *testing.T) {
	tmpDir := t.TempDir()
	contentDir := filepath.Join(tmpDir, "mymod.aba_unpacked")
	if err := os.MkdirAll(contentDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(contentDir, "mymod.menuassets"), []byte("menu"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := (&PackService{}).PackToAbaAndCt(contentDir, ""); err != nil {
		t.Fatalf("PackToAbaAndCt: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "mymod.aba")); err != nil {
		t.Fatalf("default output name did not strip the .aba_unpacked suffix: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "mymod.ct")); err != nil {
		t.Fatalf("ct output missing: %v", err)
	}
}

func TestMenuAssetsNameMismatchWarning(t *testing.T) {
	tests := []struct {
		name     string
		manifest ModManifest
		warn     bool
	}{
		{
			name: "matching",
			manifest: ModManifest{Name: "mymod", Assets: []ModAsset{
				{Name: "mymod.menuassets"}, {Name: "body.tex"},
			}},
		},
		{
			name: "mismatched",
			manifest: ModManifest{Name: "mymod.aba_unpacked", Assets: []ModAsset{
				{Name: "mymod.menuassets"},
			}},
			warn: true,
		},
		{
			name: "uppercase bundle name",
			manifest: ModManifest{Name: "MyMod", Assets: []ModAsset{
				{Name: "MyMod.menuassets"},
			}},
			warn: true,
		},
		{
			name: "no menuassets",
			manifest: ModManifest{Name: "whatever", Assets: []ModAsset{
				{Name: "body.tex"},
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warning := menuAssetsNameMismatchWarning(tt.manifest)
			if (warning != "") != tt.warn {
				t.Fatalf("warning = %q, want warn=%v", warning, tt.warn)
			}
		})
	}
}
