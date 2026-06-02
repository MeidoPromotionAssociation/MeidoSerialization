package KCES

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

func TestPackMod_Integration(t *testing.T) {
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
	manifestData, _ := json.Marshal(manifest)
	manifestPath := filepath.Join(tmpDir, "manifest.json")
	os.WriteFile(manifestPath, manifestData, 0644)

	// 打包
	service := &ModPackService{}
	if err := service.PackMod(manifestPath, tmpDir); err != nil {
		t.Fatalf("PackMod failed: %v", err)
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

	// 验证 catalog items 按 hash 升序排序
	for i := 1; i < len(catalog.Items); i++ {
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

	bundle, err := aba.ReadBundle(bytes.NewReader(abaData))
	if err != nil {
		t.Fatalf("ReadBundle: %v", err)
	}

	fileData, err := bundle.GetFileData(0)
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
		typeId, found := assetNames[item.Name]
		if !found {
			t.Errorf("catalog item %q not found in .aba AssetBundle", item.Name)
			continue
		}
		// TextAsset 类型应为 49
		if typeId != 49 {
			t.Errorf("catalog item %q: expected ClassID 49 (TextAsset), got %d", item.Name, typeId)
		}
	}

	// 验证 ExtensionNameList 也按 hash 排序
	for _, ext := range catalog.ExtensionList {
		enl, err := ct.DecodeExtensionNameListFromCt(table, ext)
		if err != nil {
			t.Errorf("decode ExtensionNameList %q: %v", ext, err)
			continue
		}
		if !sort.SliceIsSorted(enl.Data, func(i, j int) bool {
			return enl.Data[i].Hash < enl.Data[j].Hash
		}) {
			t.Errorf("ExtensionNameList %q not sorted by hash", ext)
		}
	}
}

func TestPackMod_Texture2D(t *testing.T) {
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
	manifestData, _ := json.Marshal(manifest)
	manifestPath := filepath.Join(tmpDir, "manifest.json")
	os.WriteFile(manifestPath, manifestData, 0644)

	service := &ModPackService{}
	if err := service.PackMod(manifestPath, tmpDir); err != nil {
		t.Fatalf("PackMod failed: %v", err)
	}

	// 验证 .aba 中的 Texture2D
	abaData, _ := os.ReadFile(filepath.Join(tmpDir, "tex_test.aba"))
	bundle, _ := aba.ReadBundle(bytes.NewReader(abaData))
	fileData, _ := bundle.GetFileData(0)
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

func TestPackMod_InferRawTextureAndSpriteFromPath(t *testing.T) {
	tmpDir := t.TempDir()

	rawTexData, err := os.ReadFile(filepath.Join("..", "..", "testdata", "kces_assets", "cm3d2_megane002.tex.bytes"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "raw.tex.bytes"), rawTexData, 0644); err != nil {
		t.Fatal(err)
	}
	rawSpriteData, err := os.ReadFile(filepath.Join("..", "..", "testdata", "kces_assets", "cm3d2_megane002_i_.tex.sprite.bytes"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "sprite.tex.sprite.bytes"), rawSpriteData, 0644); err != nil {
		t.Fatal(err)
	}

	manifest := ModManifest{
		Name:        "raw_asset_test",
		CatalogType: "Parts",
		PackageType: "Plugin",
		Assets: []ModAsset{
			{Name: "raw.tex", Path: "raw.tex.bytes"},
			{Name: "sprite.tex", Path: "sprite.tex.sprite.bytes"},
		},
	}
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(tmpDir, "manifest.json")
	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		t.Fatal(err)
	}

	service := &ModPackService{}
	if err := service.PackMod(manifestPath, tmpDir); err != nil {
		t.Fatalf("PackMod failed: %v", err)
	}

	abaData, err := os.ReadFile(filepath.Join(tmpDir, "raw_asset_test.aba"))
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := aba.ReadBundle(bytes.NewReader(abaData))
	if err != nil {
		t.Fatalf("ReadBundle: %v", err)
	}
	fileData, err := bundle.GetFileData(0)
	if err != nil {
		t.Fatalf("GetFileData: %v", err)
	}
	af, err := aba.ReadAssetsFile(fileData)
	if err != nil {
		t.Fatalf("ReadAssetsFile: %v", err)
	}

	assetTypes := map[string]int32{}
	for _, e := range af.GetAssetEntries() {
		assetTypes[e.Name] = e.TypeId
	}
	if assetTypes["raw.tex"] != aba.ClassIDTexture2D {
		t.Fatalf("raw.tex type got %d, want Texture2D", assetTypes["raw.tex"])
	}
	if assetTypes["sprite.tex"] != aba.ClassIDSprite {
		t.Fatalf("sprite.tex type got %d, want Sprite", assetTypes["sprite.tex"])
	}
}

func TestPackMod_ExplicitUnityRawObjectKinds(t *testing.T) {
	tmpDir := t.TempDir()

	rawData, err := os.ReadFile(filepath.Join("..", "..", "testdata", "kces_assets", "DepthLUT.monoscript.bytes"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "material.bytes"), rawData, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "type95.bytes"), rawData, 0644); err != nil {
		t.Fatal(err)
	}
	if err := writeAssetMeta(filepath.Join(tmpDir, "type95.bytes"), -95, "type95_internal"); err != nil {
		t.Fatal(err)
	}

	manifest := ModManifest{
		Name:        "raw_kind_test",
		CatalogType: "Parts",
		PackageType: "Plugin",
		Assets: []ModAsset{
			{Name: "mat_internal", Path: "material.bytes", Kind: "material"},
			{Name: "type95_internal", Path: "type95.bytes", Kind: "type_95"},
		},
	}
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(tmpDir, "manifest.json")
	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		t.Fatal(err)
	}

	service := &ModPackService{}
	if err := service.PackMod(manifestPath, tmpDir); err != nil {
		t.Fatalf("PackMod failed: %v", err)
	}

	abaData, err := os.ReadFile(filepath.Join(tmpDir, "raw_kind_test.aba"))
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := aba.ReadBundle(bytes.NewReader(abaData))
	if err != nil {
		t.Fatalf("ReadBundle: %v", err)
	}
	fileData, err := bundle.GetFileData(0)
	if err != nil {
		t.Fatalf("GetFileData: %v", err)
	}
	af, err := aba.ReadAssetsFile(fileData)
	if err != nil {
		t.Fatalf("ReadAssetsFile: %v", err)
	}

	assetTypes := assetTypesByLoadName(t, af)
	if assetTypes["mat_internal"] != aba.ClassIDMaterial {
		t.Fatalf("mat_internal type got %d, want Material", assetTypes["mat_internal"])
	}
	if assetTypes["type95_internal"] != 95 {
		t.Fatalf("type95_internal type got %d, want Type_95", assetTypes["type95_internal"])
	}
	if info := af.GetAssetInfoByPathID(-95); info == nil || info.TypeId != 95 {
		t.Fatalf("type95 PathID not preserved: %+v", info)
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
