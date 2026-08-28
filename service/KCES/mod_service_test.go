package KCES

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/internal/kcesfixtures"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/aba"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/ct"
)

func TestPackModManifest_Integration(t *testing.T) {
	tmpDir := t.TempDir()

	// 准备测试资源
	os.WriteFile(filepath.Join(tmpDir, "menu.bin"), []byte("fake menu data"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "material.bin"), []byte("fake material data"), 0644)

	manifest := ModManifest{
		Name:        "integration_test",
		CatalogType: "Parts",
		PackageType: "Plugin",
		Priority:    100,
		Assets: []ModAsset{
			{Name: "test.menuassets", Path: "menu.bin", Kind: "textasset"},
			{Name: "test.materialassets", Path: "material.bin", Kind: "textasset"},
		},
	}
	// 打包
	if err := packModManifest(manifest, tmpDir, tmpDir); err != nil {
		t.Fatalf("packModManifest failed: %v", err)
	}

	// 验证 .ct
	ctPath := filepath.Join(tmpDir, "integration_test.ct")
	ctFile, err := os.Open(ctPath)
	if err != nil {
		t.Fatalf("open .ct: %v", err)
	}
	defer ctFile.Close()

	table, err := ct.ReadContentTable(ctFile)
	if err != nil {
		t.Fatalf("ReadContentTable: %v", err)
	}

	catalog, err := ct.DecodeCatalogFromCt(table)
	if err != nil {
		t.Fatalf("DecodeCatalogFromCt: %v", err)
	}
	if want := ct.HashStringIgnoreCase("integration_test.aba"); catalog.Hash != want {
		t.Fatalf("catalog hash=%d, want resource .aba hash %d", catalog.Hash, want)
	}
	if !sort.StringsAreSorted(testStringValues(catalog.ExtensionList)) {
		t.Fatalf("catalog extension list is not sorted: %v", catalog.ExtensionList)
	}

	// 验证 catalog items 按 hash 升序排序
	for i := 1; i < len(catalog.Items); i++ {
		if catalog.Items[i-1] == nil || catalog.Items[i] == nil {
			t.Fatalf("catalog contains a null item: %+v", catalog.Items)
		}
		if catalog.Items[i].Hash < catalog.Items[i-1].Hash {
			t.Errorf("catalog items not sorted by hash: [%d].hash=%d > [%d].hash=%d",
				i-1, catalog.Items[i-1].Hash, i, catalog.Items[i].Hash)
		}
	}

	// 验证 .aba
	abaPath := filepath.Join(tmpDir, "integration_test.aba")
	abaData, err := os.ReadFile(abaPath)
	if err != nil {
		t.Fatalf("read .aba: %v", err)
	}

	abaFile, err := aba.ReadAba(bytes.NewReader(abaData))
	if err != nil {
		t.Fatalf("ReadAba: %v", err)
	}

	fileData, err := abaFile.GetFileData(0)
	if err != nil {
		t.Fatalf("GetFileData: %v", err)
	}

	af, err := aba.ReadAssetsFile(fileData)
	if err != nil {
		t.Fatalf("ReadAssetsFile: %v", err)
	}

	assetEntries := af.GetAssetEntries()
	assetNames := map[string]int32{}
	for _, e := range assetEntries {
		assetNames[e.Name] = e.TypeId
	}

	// 验证 catalog 每个 item 都能在 AssetBundle 中找到同名对象
	for _, item := range catalog.Items {
		if item == nil {
			t.Fatal("catalog contains a null item")
		}
		itemName := testStringValue(item.Name)
		typeId, found := assetNames[itemName]
		if !found {
			t.Errorf("catalog item %q not found in .aba AssetBundle", itemName)
			continue
		}
		// TextAsset 类型应为 49
		if typeId != 49 {
			t.Errorf("catalog item %q: expected ClassID 49 (TextAsset), got %d", itemName, typeId)
		}
	}

	// 验证 ExtensionNameList 也按 hash 排序
	for _, ext := range catalog.ExtensionList {
		extension := testStringValue(ext)
		enl, err := ct.DecodeExtensionNameListFromCt(table, extension)
		if err != nil {
			t.Errorf("decode ExtensionNameList %q: %v", extension, err)
			continue
		}
		if !sort.SliceIsSorted(enl.Data, func(i, j int) bool {
			return enl.Data[i].Hash < enl.Data[j].Hash
		}) {
			t.Errorf("ExtensionNameList %q not sorted by hash", extension)
		}
	}
}

func TestPackModManifestCatalogsExtensionlessTextAssetsUnderNullGroup(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "maid_collider.bin"), []byte("extensionless TextAsset payload"), 0644); err != nil {
		t.Fatal(err)
	}
	nativeTex, err := aba.NewNativeTexture2DObject("dependency", 1, 1, []byte{1, 2, 3, 4})
	if err != nil {
		t.Fatal(err)
	}
	var nativeBuf bytes.Buffer
	if err := aba.WriteNativeUnityObject(&nativeBuf, nativeTex); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "dependency.bin"), nativeBuf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	manifest := ModManifest{
		Name:        "extensionless_test",
		CatalogType: "System",
		PackageType: "Plugin",
		Assets: []ModAsset{
			{Name: "maid_collider", Path: "maid_collider.bin", Kind: "textasset"},
			// Extensionless standalone Unity objects remain m_Container-only by default.
			{Name: "dependency", Path: "dependency.bin", Kind: "rawtexture2d", nativeObjectFile: true},
		},
	}
	if err := packModManifest(manifest, tmpDir, tmpDir); err != nil {
		t.Fatalf("packModManifest: %v", err)
	}

	ctFile, err := os.Open(filepath.Join(tmpDir, "extensionless_test.ct"))
	if err != nil {
		t.Fatal(err)
	}
	defer ctFile.Close()
	table, err := ct.ReadContentTable(ctFile)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := ct.DecodeCatalogFromCt(table)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.ExtensionList) != 1 || testStringValue(catalog.ExtensionList[0]) != "null" {
		t.Fatalf("extensionList = %v, want official extensionless group [null]", catalog.ExtensionList)
	}
	if len(catalog.Items) != 1 || catalog.Items[0] == nil || testStringValue(catalog.Items[0].Name) != "maid_collider" || catalog.Items[0].Hash != ct.HashStringIgnoreCase("maid_collider") {
		t.Fatalf("catalog items = %+v, want extensionless maid_collider only", catalog.Items)
	}

	enl, err := ct.DecodeExtensionNameListFromCt(table, "null")
	if err != nil {
		t.Fatal(err)
	}
	if testStringValue(enl.Extension) != "null" || len(enl.Data) != 1 || enl.Data[0] == nil || testStringValue(enl.Data[0].Name) != "maid_collider" || enl.Data[0].Hash != ct.HashStringIgnoreCase("maid_collider") {
		t.Fatalf("null ExtensionNameList = %+v", enl)
	}
}

func TestPackModManifest_Texture2D(t *testing.T) {
	tmpDir := t.TempDir()

	// 用标准库生成有效的 1x1 PNG
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, G: 128, B: 64, A: 255})
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}
	os.WriteFile(filepath.Join(tmpDir, "test.png"), pngBuf.Bytes(), 0644)

	manifest := ModManifest{
		Name:        "tex_test",
		CatalogType: "Parts",
		PackageType: "Plugin",
		Priority:    1,
		Assets: []ModAsset{
			{Name: "test.tex", Path: "test.png", Kind: "texture2d"},
		},
	}
	if err := packModManifest(manifest, tmpDir, tmpDir); err != nil {
		t.Fatalf("packModManifest failed: %v", err)
	}

	// 验证 .aba 中的 Texture2D
	abaData, _ := os.ReadFile(filepath.Join(tmpDir, "tex_test.aba"))
	abaFile, _ := aba.ReadAba(bytes.NewReader(abaData))
	fileData, _ := abaFile.GetFileData(0)
	af, _ := aba.ReadAssetsFile(fileData)

	entries := af.GetAssetEntries()
	foundTex := false
	for _, e := range entries {
		if e.Name == "test.tex" {
			foundTex = true
			if e.TypeId != 28 { // ClassIDTexture2D
				t.Errorf("test.tex: expected ClassID 28 (Texture2D), got %d", e.TypeId)
			}
		}
	}
	if !foundTex {
		t.Error("test.tex not found in .aba")
	}
}

func TestPackModManifest_RejectsBareSidecarPayloadInferredFromPath(t *testing.T) {
	tmpDir := t.TempDir()

	rawTexPath, _ := kcesfixtures.RawObjectPath(t, "cm3d2_megane002.aba", "cm3d2_megane002.tex", aba.ClassIDTexture2D, "raw.tex.bytes")
	rawTexData, err := os.ReadFile(rawTexPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "raw.tex.bytes"), rawTexData, 0644); err != nil {
		t.Fatal(err)
	}

	manifest := ModManifest{
		Name:        "raw_asset_test",
		CatalogType: "Parts",
		PackageType: "Plugin",
		Assets: []ModAsset{
			{Name: "raw.tex", Path: "raw.tex.bytes"},
		},
	}
	// Bare sidecar payloads carry no TypeTree, and files without one per type can no
	// longer be produced because the game cannot read them.
	err = packModManifest(manifest, tmpDir, tmpDir)
	if err == nil || !strings.Contains(err.Error(), "has no TypeTree") {
		t.Fatalf("packModManifest error = %v, want missing-TypeTree rejection", err)
	}
}

func TestPackModManifest_RejectsBarePayloadForExplicitRawKinds(t *testing.T) {
	tmpDir := t.TempDir()

	rawSourcePath, _ := kcesfixtures.RawObjectPath(t, "system.aba", "DepthLUT", aba.ClassIDMonoScript, "material.bytes")
	rawData, err := os.ReadFile(rawSourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "material.bytes"), rawData, 0644); err != nil {
		t.Fatal(err)
	}

	manifest := ModManifest{
		Name:        "raw_kind_test",
		CatalogType: "Parts",
		PackageType: "Plugin",
		Assets: []ModAsset{
			{Name: "mat_internal", Path: "material.bytes", Kind: "material"},
		},
	}
	err = packModManifest(manifest, tmpDir, tmpDir)
	if err == nil || !strings.Contains(err.Error(), "has no TypeTree") {
		t.Fatalf("packModManifest error = %v, want missing-TypeTree rejection", err)
	}
}

func TestBuildCanonicalLoadNamesResolvesCrossTypeDuplicates(t *testing.T) {
	assets := []ModAsset{
		{Name: "ER_Sky.tex", Path: "Material/ER_Sky.bytes"},
		{Name: "ER_Sky.tex", Path: "Cubemap/ER_Sky.bytes"},
		{Name: "ER_Sky_0002.tex", Path: "Shader/ER_Sky_0002.bytes"},
	}
	paths := []string{assets[0].Path, assets[1].Path, assets[2].Path}
	got, err := buildCanonicalLoadNames(assets, paths)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ER_Sky_0003.tex", "ER_Sky.tex", "ER_Sky_0002.tex"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("load names = %v, want %v", got, want)
	}
}

func assetTypesByLoadName(t *testing.T, af *aba.AssetsFile) map[string]int32 {
	t.Helper()
	containerNames, err := af.GetAssetBundleContainerMap()
	if err != nil {
		t.Fatalf("GetAssetBundleContainerMap: %v", err)
	}
	out := map[string]int32{}
	for _, e := range af.GetAssetEntries() {
		name := e.Name
		if containerName, ok := containerNames[e.PathId]; ok {
			name = containerName
		}
		out[name] = e.TypeId
	}
	return out
}
