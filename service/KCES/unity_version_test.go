package KCES

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/aba"
)

func TestResolveUnityPackSettingsIsFixed(t *testing.T) {
	got := resolveUnityPackSettings()
	want := unityPackSettings{
		UnityVersion:          "2022.3.35f1",
		EngineVersion:         "2022.3.35f1",
		TargetPlatform:        19,
		AbaVersion:            8,
		GenerationVersion:     "5.x.x",
		SerializedFileVersion: 22,
	}
	if got != want {
		t.Fatalf("resolveUnityPackSettings() = %+v, want %+v", got, want)
	}
}

func TestValidateCanonicalSourceUnityVersionAcceptsKnownAndFutureVersions(t *testing.T) {
	for _, version := range []string{
		"2020.2.4f1",
		"2020.2.6f1",
		"2020.3.2f1",
		"2020.3.3f1",
		"2020.3.4f1",
		"2020.3.8f1",
		"2020.3.22f1",
		"2020.3.27f1",
		"2020.3.29f1",
		"2021.3.2f1",
		"2021.3.3f1",
		"2021.3.6f1",
		"2021.3.8f1",
		"2022.3.35f1",
		"2022.3.62f2",
		"2023.1.0f1",
		"6000.0.58f2",
	} {
		if err := validateCanonicalSourceUnityVersion(version); err != nil {
			t.Errorf("validateCanonicalSourceUnityVersion(%q): %v", version, err)
		}
	}
	for _, version := range []string{"", "not-a-version", "2019.4.40f1", "2020.1.17f1"} {
		if err := validateCanonicalSourceUnityVersion(version); err == nil {
			t.Errorf("validateCanonicalSourceUnityVersion(%q) unexpectedly succeeded", version)
		}
	}
}

func TestUnpackAbaAcceptsUnityFS7KCESSample(t *testing.T) {
	sample := filepath.Join("..", "..", "testdata", "KCES", "parts_personal002.aba")
	if _, err := os.Stat(sample); err != nil {
		t.Skipf("sample not available: %v", err)
	}
	outDir := filepath.Join(t.TempDir(), "out")
	if err := (&AbaService{}).UnpackAba(sample, outDir); err != nil {
		t.Fatalf("UnpackAba UnityFS 7 sample: %v", err)
	}
	if files := validateCanonicalDirectoryFiles(t, outDir, nil); len(files) == 0 {
		t.Fatal("UnityFS 7 sample unpacked to an empty directory")
	}
}

func TestPackModIgnoresLegacyVersionSidecarAndWritesFixedContract(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "asset.bin"), []byte("payload"), 0644); err != nil {
		t.Fatal(err)
	}
	legacy := `{"pathId":42,"loadName":"assets/old/path","unityVersion":"2020.2.6f1","engineVersion":"2020.2.6f1","targetPlatform":19,"abaVersion":7,"serializedFileVersion":21}`
	if err := os.WriteFile(filepath.Join(dir, "asset.bin.meta.json"), []byte(legacy), 0644); err != nil {
		t.Fatal(err)
	}
	manifest := ModManifest{
		Name:        "fixed_contract",
		CatalogType: "Parts",
		PackageType: "Plugin",
		Assets: []ModAsset{{
			Name: "asset.menuassets", Path: "asset.bin", Kind: "textasset",
		}},
	}
	if err := packModManifest(manifest, dir, dir); err != nil {
		t.Fatal(err)
	}
	bundleBytes, err := os.ReadFile(filepath.Join(dir, manifest.Name+".aba"))
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := aba.ReadAba(bytes.NewReader(bundleBytes))
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Header.Version != defaultKCESAbaVersion || bundle.Header.EngineVersion != defaultKCESUnityVersion || bundle.Header.GenerationVersion != defaultKCESGenerationVersion {
		t.Fatalf("UnityFS contract = version %d generation %q engine %q", bundle.Header.Version, bundle.Header.GenerationVersion, bundle.Header.EngineVersion)
	}
	serialized, err := bundle.GetFileData(0)
	if err != nil {
		t.Fatal(err)
	}
	af, err := aba.ReadAssetsFile(serialized)
	if err != nil {
		t.Fatal(err)
	}
	if af.Header.Version != supportedSerializedFileVersion || af.Metadata.UnityVersion != defaultKCESUnityVersion || af.Metadata.TargetPlatform != defaultKCESTargetPlatform {
		t.Fatalf("SerializedFile contract = version %d Unity %q platform %d", af.Header.Version, af.Metadata.UnityVersion, af.Metadata.TargetPlatform)
	}
	container, err := af.GetAssetBundleContainerMap()
	if err != nil {
		t.Fatal(err)
	}
	for _, loadName := range container {
		if strings.Contains(loadName, "old/path") {
			t.Fatalf("legacy loadName leaked into m_Container: %q", loadName)
		}
	}
}
