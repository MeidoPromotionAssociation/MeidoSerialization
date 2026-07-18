package KCES

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
)

func TestResolveUnityPackSettingsUsesOneConsistentSourceVersion(t *testing.T) {
	target := uint32(19)
	metas := []rawAssetMeta{
		{
			UnityVersion:          "2022.3.35f1",
			EngineVersion:         "2022.3.35f1",
			TargetPlatform:        &target,
			AbaVersion:            8,
			GenerationVersion:     "5.x.x",
			SerializedFileVersion: 22,
		},
		{
			UnityVersion:          "2022.3.35f1",
			EngineVersion:         "2022.3.35f1",
			TargetPlatform:        &target,
			AbaVersion:            8,
			GenerationVersion:     "5.x.x",
			SerializedFileVersion: 22,
		},
		{}, // Old sidecar or no sidecar: it must not override known context.
	}

	got, err := resolveUnityPackSettings(metas, []string{"texture.bytes", "sprite.bytes", "legacy.bin"})
	if err != nil {
		t.Fatalf("resolveUnityPackSettings: %v", err)
	}
	want := unityPackSettings{
		UnityVersion:          "2022.3.35f1",
		EngineVersion:         "2022.3.35f1",
		TargetPlatform:        19,
		AbaVersion:            8,
		GenerationVersion:     "5.x.x",
		SerializedFileVersion: 22,
	}
	if got != want {
		t.Fatalf("settings got %+v, want %+v", got, want)
	}
}

func TestResolveUnityPackSettingsBackwardCompatibleDefaults(t *testing.T) {
	got, err := resolveUnityPackSettings([]rawAssetMeta{{PathID: 7, LoadName: "legacy"}}, []string{"legacy.bin"})
	if err != nil {
		t.Fatalf("resolveUnityPackSettings: %v", err)
	}
	want := unityPackSettings{
		UnityVersion:          defaultKCESUnityVersion,
		EngineVersion:         defaultKCESUnityVersion,
		TargetPlatform:        defaultKCESTargetPlatform,
		AbaVersion:            7,
		GenerationVersion:     defaultKCESGenerationVersion,
		SerializedFileVersion: supportedSerializedFileVersion,
	}
	if got != want {
		t.Fatalf("legacy settings got %+v, want %+v", got, want)
	}

	// A partial new sidecar containing only UnityVersion is valid. The matching
	// UnityFS engine version and v8 header are derived for a 2022.3 source.
	got, err = resolveUnityPackSettings([]rawAssetMeta{{UnityVersion: "2022.3.62f2"}}, []string{"partial.bytes"})
	if err != nil {
		t.Fatalf("resolve partial sidecar: %v", err)
	}
	if got.UnityVersion != "2022.3.62f2" || got.EngineVersion != got.UnityVersion || got.AbaVersion != 8 {
		t.Fatalf("partial sidecar settings are inconsistent: %+v", got)
	}
}

func TestResolveUnityPackSettingsRejectsConflicts(t *testing.T) {
	targetWindows := uint32(19)
	targetOther := uint32(13)
	tests := []struct {
		name string
		meta []rawAssetMeta
		want string
	}{
		{
			name: "unity version",
			meta: []rawAssetMeta{{UnityVersion: "2021.3.3f1"}, {UnityVersion: "2022.3.35f1"}},
			want: "conflicting unityVersion sidecars",
		},
		{
			name: "engine versus serialized metadata",
			meta: []rawAssetMeta{{UnityVersion: "2021.3.3f1"}, {EngineVersion: "2022.3.35f1"}},
			want: "Unity version contract conflict",
		},
		{
			name: "target platform",
			meta: []rawAssetMeta{{TargetPlatform: &targetWindows}, {TargetPlatform: &targetOther}},
			want: "conflicting targetPlatform sidecars",
		},
		{
			name: "aba version",
			meta: []rawAssetMeta{{AbaVersion: 7}, {AbaVersion: 8}},
			want: "conflicting abaVersion sidecars",
		},
		{
			name: "generation version",
			meta: []rawAssetMeta{{GenerationVersion: "5.x.x"}, {GenerationVersion: "custom"}},
			want: "conflicting generationVersion sidecars",
		},
		{
			name: "serialized file version",
			meta: []rawAssetMeta{{SerializedFileVersion: 21}, {SerializedFileVersion: 22}},
			want: "conflicting serializedFileVersion sidecars",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolveUnityPackSettings(tt.meta, []string{"first.bytes", "second.bytes"})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error got %v, want substring %q", err, tt.want)
			}
			if !strings.Contains(err.Error(), "first.bytes") || !strings.Contains(err.Error(), "second.bytes") {
				t.Fatalf("conflict error must identify both sources: %v", err)
			}
		})
	}
}

func TestPackModUsesSidecarVersionForSerializedFileAndAba(t *testing.T) {
	tmpDir := t.TempDir()
	target := uint32(19)
	meta := rawAssetMeta{
		UnityVersion:          "2022.3.35f1",
		EngineVersion:         "2022.3.35f1",
		TargetPlatform:        &target,
		AbaVersion:            8,
		GenerationVersion:     "5.x.x",
		SerializedFileVersion: 22,
	}
	assets := []ModAsset{
		{Name: "first.menuassets", Path: "first.bin", Kind: "textasset"},
		{Name: "second.materialassets", Path: "second.bin", Kind: "textasset"},
	}
	for i, asset := range assets {
		path := filepath.Join(tmpDir, asset.Path)
		if err := os.WriteFile(path, []byte{byte(i + 1), 2, 3}, 0644); err != nil {
			t.Fatal(err)
		}
		assetMeta := meta
		assetMeta.PathID = int64(100 + i)
		assetMeta.LoadName = asset.Name
		if err := writeRawAssetMeta(path, assetMeta); err != nil {
			t.Fatal(err)
		}
	}

	manifest := ModManifest{
		Name:        "version_contract",
		CatalogType: "Parts",
		PackageType: "Plugin",
		Assets:      assets,
	}
	if err := packModManifest(manifest, tmpDir, tmpDir); err != nil {
		t.Fatalf("packModManifest: %v", err)
	}

	abaData, err := os.ReadFile(filepath.Join(tmpDir, "version_contract.aba"))
	if err != nil {
		t.Fatal(err)
	}
	abaFile, err := aba.ReadAba(bytes.NewReader(abaData))
	if err != nil {
		t.Fatalf("ReadAba: %v", err)
	}
	if abaFile.Header.Version != 8 || abaFile.Header.EngineVersion != "2022.3.35f1" || abaFile.Header.GenerationVersion != "5.x.x" {
		t.Fatalf(".aba version contract was not preserved: %+v", abaFile.Header)
	}
	serialized, err := abaFile.GetFileData(0)
	if err != nil {
		t.Fatalf("GetFileData: %v", err)
	}
	af, err := aba.ReadAssetsFile(serialized)
	if err != nil {
		t.Fatalf("ReadAssetsFile: %v", err)
	}
	if af.Header.Version != 22 || af.Metadata.UnityVersion != "2022.3.35f1" || af.Metadata.TargetPlatform != 19 {
		t.Fatalf("SerializedFile version contract was not preserved: header=%+v metadata=%+v", af.Header, af.Metadata)
	}
	if abaFile.Header.EngineVersion != af.Metadata.UnityVersion {
		t.Fatalf("UnityFS engine version %q differs from SerializedFile version %q", abaFile.Header.EngineVersion, af.Metadata.UnityVersion)
	}
}

func TestPackModWithoutVersionSidecarUsesMatchingLegacyDefault(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "legacy.bin"), []byte("payload"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := packModManifest(ModManifest{
		Name:        "legacy_default",
		CatalogType: "Parts",
		PackageType: "Plugin",
		Assets: []ModAsset{
			{Name: "legacy.menuassets", Path: "legacy.bin", Kind: "textasset"},
		},
	}, tmpDir, tmpDir); err != nil {
		t.Fatalf("packModManifest: %v", err)
	}

	abaData, err := os.ReadFile(filepath.Join(tmpDir, "legacy_default.aba"))
	if err != nil {
		t.Fatal(err)
	}
	abaFile, err := aba.ReadAba(bytes.NewReader(abaData))
	if err != nil {
		t.Fatalf("ReadAba: %v", err)
	}
	serialized, err := abaFile.GetFileData(0)
	if err != nil {
		t.Fatalf("GetFileData: %v", err)
	}
	af, err := aba.ReadAssetsFile(serialized)
	if err != nil {
		t.Fatalf("ReadAssetsFile: %v", err)
	}
	if abaFile.Header.Version != 7 || abaFile.Header.EngineVersion != defaultKCESUnityVersion ||
		af.Metadata.UnityVersion != defaultKCESUnityVersion || af.Metadata.TargetPlatform != defaultKCESTargetPlatform {
		t.Fatalf("legacy default version contract mismatch: aba=%+v assets=%+v", abaFile.Header, af.Metadata)
	}
}

func TestPackModRejectsConflictingSidecarVersionsBeforeWriting(t *testing.T) {
	tmpDir := t.TempDir()
	assets := []ModAsset{
		{Name: "first.menuassets", Path: "first.bin", Kind: "textasset"},
		{Name: "second.menuassets", Path: "second.bin", Kind: "textasset"},
	}
	versions := []string{"2021.3.3f1", "2022.3.35f1"}
	for i, asset := range assets {
		path := filepath.Join(tmpDir, asset.Path)
		if err := os.WriteFile(path, []byte("payload"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := writeRawAssetMeta(path, rawAssetMeta{UnityVersion: versions[i]}); err != nil {
			t.Fatal(err)
		}
	}

	err := packModManifest(ModManifest{
		Name:        "version_conflict",
		CatalogType: "Parts",
		PackageType: "Plugin",
		Assets:      assets,
	}, tmpDir, tmpDir)
	if err == nil || !strings.Contains(err.Error(), "conflicting unityVersion sidecars") ||
		!strings.Contains(err.Error(), "first.bin") || !strings.Contains(err.Error(), "second.bin") {
		t.Fatalf("conflict error got %v", err)
	}
	for _, ext := range []string{".aba", ".ct"} {
		if _, statErr := os.Stat(filepath.Join(tmpDir, "version_conflict"+ext)); !os.IsNotExist(statErr) {
			t.Fatalf("conflicting input unexpectedly wrote %s (stat error %v)", ext, statErr)
		}
	}
}

func TestPackModRejectsMalformedVersionSidecar(t *testing.T) {
	tmpDir := t.TempDir()
	assetPath := filepath.Join(tmpDir, "broken.bin")
	if err := os.WriteFile(assetPath, []byte("payload"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(assetMetaPath(assetPath), []byte(`{"unityVersion":`), 0644); err != nil {
		t.Fatal(err)
	}

	err := packModManifest(ModManifest{
		Name:        "malformed_meta",
		CatalogType: "Parts",
		PackageType: "Plugin",
		Assets: []ModAsset{
			{Name: "broken.menuassets", Path: "broken.bin", Kind: "textasset"},
		},
	}, tmpDir, tmpDir)
	if err == nil || !strings.Contains(err.Error(), "parse asset metadata") || !strings.Contains(err.Error(), "broken.bin.meta.json") {
		t.Fatalf("malformed sidecar error got %v", err)
	}
}

func TestRawUnityObjectJSONRoundTripPreservesUnityVersionContext(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "source.texture2d.bytes")
	raw := []byte{4, 0, 0, 0, 'n', 'a', 'm', 'e', 0, 0, 0, 0}
	if err := os.WriteFile(inputPath, raw, 0644); err != nil {
		t.Fatal(err)
	}
	target := uint32(19)
	wantMeta := rawAssetMeta{
		PathID:                -12345,
		LoadName:              "assets/source.tex.png",
		UnityVersion:          "2020.2.6f1",
		EngineVersion:         "2020.2.6f1",
		TargetPlatform:        &target,
		AbaVersion:            7,
		GenerationVersion:     "5.x.x",
		SerializedFileVersion: 22,
	}
	if err := writeRawAssetMeta(inputPath, wantMeta); err != nil {
		t.Fatal(err)
	}

	service := &RawUnityObjectService{}
	jsonPath := inputPath + ".json"
	if err := service.ConvertRawUnityObjectToJson(inputPath, jsonPath); err != nil {
		t.Fatalf("ConvertRawUnityObjectToJson: %v", err)
	}
	var envelope RawUnityObjectEnvelope
	if err := json.Unmarshal(mustReadServiceFile(t, jsonPath), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.UnityVersion != wantMeta.UnityVersion || envelope.EngineVersion != wantMeta.EngineVersion ||
		envelope.TargetPlatform == nil || *envelope.TargetPlatform != target || envelope.AbaVersion != 7 ||
		envelope.GenerationVersion != "5.x.x" || envelope.SerializedFileVersion != 22 {
		t.Fatalf("JSON envelope lost Unity version context: %+v", envelope)
	}

	outputPath := filepath.Join(tmpDir, "roundtrip.texture2d.bytes")
	if err := service.ConvertJsonToRawUnityObject(jsonPath, outputPath); err != nil {
		t.Fatalf("ConvertJsonToRawUnityObject: %v", err)
	}
	if got := mustReadServiceFile(t, outputPath); !bytes.Equal(got, raw) {
		t.Fatalf("raw object bytes changed: got %x, want %x", got, raw)
	}
	gotMeta := readAssetMeta(outputPath)
	if !reflect.DeepEqual(gotMeta, wantMeta) {
		t.Fatalf("round-trip metadata got %+v, want %+v", gotMeta, wantMeta)
	}
}

func TestKCESSampleVersionFamiliesUseMatchingHeadersAndPreserveTargetPlatform(t *testing.T) {
	tests := []struct {
		file           string
		unityVersion   string
		abaVersion     uint32
		targetPlatform uint32
	}{
		{file: "parts_bcc2_gp003.aba", unityVersion: "2020.2.4f1", abaVersion: 7, targetPlatform: 19},
		{file: "cm3d2_megane002.aba", unityVersion: "2021.3.6f1", abaVersion: 7, targetPlatform: 5},
		{file: "csv.aba", unityVersion: "2022.3.62f2", abaVersion: 8, targetPlatform: 19},
	}

	for _, tt := range tests {
		t.Run(tt.unityVersion, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", "aba", tt.file)
			f, err := os.Open(path)
			if err != nil {
				t.Skipf("sample not available: %v", err)
			}
			defer f.Close()
			abaFile, err := aba.ReadAba(f)
			if err != nil {
				t.Fatalf("ReadAba: %v", err)
			}
			if abaFile.Header.Version != tt.abaVersion || abaFile.Header.EngineVersion != tt.unityVersion {
				t.Fatalf(".aba got version=%d engine=%q, want version=%d engine=%q",
					abaFile.Header.Version, abaFile.Header.EngineVersion, tt.abaVersion, tt.unityVersion)
			}

			var assetsFile *aba.AssetsFile
			for i, entry := range abaFile.BlockInfo.DirectoryInfos {
				if !entry.IsSerialized() {
					continue
				}
				data, readErr := abaFile.GetFileData(i)
				if readErr != nil {
					t.Fatalf("GetFileData: %v", readErr)
				}
				assetsFile, err = aba.ReadAssetsFile(data)
				if err != nil {
					t.Fatalf("ReadAssetsFile: %v", err)
				}
				break
			}
			if assetsFile == nil {
				t.Fatal("sample contains no SerializedFile")
			}
			if assetsFile.Header.Version != 22 || assetsFile.Metadata.UnityVersion != tt.unityVersion ||
				assetsFile.Metadata.TargetPlatform != tt.targetPlatform {
				t.Fatalf("SerializedFile got format=%d unity=%q target=%d, want format=22 unity=%q target=%d",
					assetsFile.Header.Version, assetsFile.Metadata.UnityVersion, assetsFile.Metadata.TargetPlatform,
					tt.unityVersion, tt.targetPlatform)
			}
			if abaFile.Header.EngineVersion != assetsFile.Metadata.UnityVersion {
				t.Fatalf(".aba engine %q differs from SerializedFile Unity version %q",
					abaFile.Header.EngineVersion, assetsFile.Metadata.UnityVersion)
			}
		})
	}
}

func TestUnpackAbaWritesSourceUnityVersionContext(t *testing.T) {
	sample := filepath.Join("..", "..", "testdata", "aba", "csv.aba")
	f, err := os.Open(sample)
	if err != nil {
		t.Skipf("sample not available: %v", err)
	}
	abaFile, err := aba.ReadAba(f)
	if err != nil {
		f.Close()
		t.Fatalf("ReadAba: %v", err)
	}
	defer f.Close()

	var sourceAssets *aba.AssetsFile
	for i, entry := range abaFile.BlockInfo.DirectoryInfos {
		if !entry.IsSerialized() {
			continue
		}
		data, readErr := abaFile.GetFileData(i)
		if readErr != nil {
			t.Fatalf("GetFileData: %v", readErr)
		}
		sourceAssets, err = aba.ReadAssetsFile(data)
		if err != nil {
			t.Fatalf("ReadAssetsFile: %v", err)
		}
		break
	}
	if sourceAssets == nil {
		t.Fatal("sample contains no SerializedFile")
	}
	if abaFile.Header.Version != 8 || abaFile.Header.EngineVersion != "2022.3.62f2" ||
		abaFile.Header.GenerationVersion != "5.x.x" || sourceAssets.Header.Version != 22 ||
		sourceAssets.Metadata.UnityVersion != "2022.3.62f2" || sourceAssets.Metadata.TargetPlatform != 19 {
		t.Fatalf("sample version evidence changed: aba=%+v assetsHeader=%+v unity=%q targetPlatform=%d",
			abaFile.Header, sourceAssets.Header, sourceAssets.Metadata.UnityVersion, sourceAssets.Metadata.TargetPlatform)
	}

	outDir := filepath.Join(t.TempDir(), "unpacked")
	if err := (&AbaService{}).UnpackAba(sample, outDir); err != nil {
		t.Fatalf("UnpackAba: %v", err)
	}
	metaPaths, err := filepath.Glob(filepath.Join(outDir, "TextAsset", "*.meta.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(metaPaths) == 0 {
		t.Fatal("UnpackAba wrote no TextAsset metadata sidecars")
	}
	var got rawAssetMeta
	if err := json.Unmarshal(mustReadServiceFile(t, metaPaths[0]), &got); err != nil {
		t.Fatalf("decode unpacked sidecar: %v", err)
	}
	if got.UnityVersion != sourceAssets.Metadata.UnityVersion || got.EngineVersion != abaFile.Header.EngineVersion ||
		got.TargetPlatform == nil || *got.TargetPlatform != sourceAssets.Metadata.TargetPlatform ||
		got.AbaVersion != abaFile.Header.Version || got.GenerationVersion != abaFile.Header.GenerationVersion ||
		got.SerializedFileVersion != sourceAssets.Header.Version {
		t.Fatalf("unpacked sidecar lost source version context: got %+v, aba=%+v assets=%+v",
			got, abaFile.Header, sourceAssets.Metadata)
	}
}
