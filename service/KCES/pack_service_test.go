package KCES

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

func TestPackService_PackToAbaAndCtProducesCatalogedBundle(t *testing.T) {
	tmpDir := t.TempDir()
	inputDir := filepath.Join(tmpDir, "sample_pack")
	if err := os.MkdirAll(inputDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(inputDir, "sample.menuassets"), []byte("menu data"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "sample.menuassets.typetree.json"), []byte(`{"format":"kces-unity-typetree"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "sample.model"), []byte("model data"), 0644); err != nil {
		t.Fatal(err)
	}
	monoScriptDir := filepath.Join(inputDir, "MonoScript")
	if err := os.MkdirAll(monoScriptDir, 0755); err != nil {
		t.Fatal(err)
	}
	monoScriptData, err := os.ReadFile(filepath.Join("..", "..", "testdata", "kces_assets", "DepthLUT.monoscript.bytes"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(monoScriptDir, "DepthLUT.bytes"), monoScriptData, 0644); err != nil {
		t.Fatal(err)
	}
	monoBehaviourDir := filepath.Join(inputDir, "MonoBehaviour")
	if err := os.MkdirAll(monoBehaviourDir, 0755); err != nil {
		t.Fatal(err)
	}
	monoBehaviourData, err := os.ReadFile(filepath.Join("..", "..", "testdata", "kces_assets", "DepthLUT.monobehaviour.bytes"))
	if err != nil {
		t.Fatal(err)
	}
	monoBehaviourPath := filepath.Join(monoBehaviourDir, "asset_-1466831684398908746.bytes")
	if err := os.WriteFile(monoBehaviourPath, monoBehaviourData, 0644); err != nil {
		t.Fatal(err)
	}
	if err := writeAssetMeta(monoBehaviourPath, -1466831684398908746, "DepthLUT"); err != nil {
		t.Fatal(err)
	}
	type95Dir := filepath.Join(inputDir, "Type_95")
	if err := os.MkdirAll(type95Dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(type95Dir, "type95_internal.bytes"), monoScriptData, 0644); err != nil {
		t.Fatal(err)
	}
	rawTexData, err := os.ReadFile(filepath.Join("..", "..", "testdata", "kces_assets", "cm3d2_megane002.tex.bytes"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "sample_raw.tex.bytes"), rawTexData, 0644); err != nil {
		t.Fatal(err)
	}
	rawSpriteData, err := os.ReadFile(filepath.Join("..", "..", "testdata", "kces_assets", "cm3d2_megane002_i_.tex.sprite.bytes"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "sample_sprite.tex.sprite.bytes"), rawSpriteData, 0644); err != nil {
		t.Fatal(err)
	}
	meshData, err := os.ReadFile(filepath.Join("..", "..", "testdata", "kces_assets", "cm3d2_megane002.mmesh.bytes"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "sample.mmesh.bytes"), meshData, 0644); err != nil {
		t.Fatal(err)
	}
	atlasData, err := os.ReadFile(filepath.Join("..", "..", "testdata", "kces_assets", "cm3d2_megane002_icon.partsatlas.bytes"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "sample.partsatlas"), atlasData, 0644); err != nil {
		t.Fatal(err)
	}
	anmData, err := os.ReadFile(filepath.Join("..", "..", "testdata", "kces_assets", "crc_stand_kihon2.anm.bytes"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "sample.anm.bytes"), anmData, 0644); err != nil {
		t.Fatal(err)
	}

	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 1, G: 2, B: 3, A: 255})
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "sample_generated.tex.png"), pngBuf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	service := &PackService{}
	if err := service.PackToAbaAndCt(inputDir, "sample_pack"); err != nil {
		t.Fatalf("PackToAbaAndCt: %v", err)
	}

	ctFile, err := os.Open(filepath.Join(tmpDir, "sample_pack.ct"))
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
	if len(catalog.ResourceFileNames) != 1 || catalog.ResourceFileNames[0] != "sample_pack.aba" {
		t.Fatalf("unexpected resource files: %+v", catalog.ResourceFileNames)
	}

	wantExts := map[string]bool{".anm": true, ".menuassets": true, ".mmesh": true, ".model": true, ".partsatlas": true, ".tex": true}
	for _, ext := range catalog.ExtensionList {
		delete(wantExts, ext)
		enl, err := ct.DecodeExtensionNameListFromCt(table, ext)
		if err != nil {
			t.Fatalf("DecodeExtensionNameListFromCt(%s): %v", ext, err)
		}
		if len(enl.Data) == 0 {
			t.Fatalf("empty ExtensionNameList for %s", ext)
		}
	}
	if len(wantExts) != 0 {
		t.Fatalf("missing catalog extensions: %+v", wantExts)
	}

	abaData, err := os.ReadFile(filepath.Join(tmpDir, "sample_pack.aba"))
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

	assetTypes := assetTypesByLoadName(t, af)
	wantTypes := map[string]int32{
		"sample.anm":           aba.ClassIDAnimationClip,
		"sample.menuassets":    aba.ClassIDTextAsset,
		"sample.model":         aba.ClassIDTextAsset,
		"sample.mmesh":         aba.ClassIDMesh,
		"sample.partsatlas":    aba.ClassIDSpriteAtlas,
		"sample_raw.tex":       aba.ClassIDTexture2D,
		"sample_generated.tex": aba.ClassIDTexture2D,
		"sample_sprite.tex":    aba.ClassIDSprite,
	}
	for _, item := range catalog.Items {
		typeID, ok := assetTypes[item.Name]
		if !ok {
			t.Fatalf("catalog item %q not found in .aba", item.Name)
		}
		wantType, ok := wantTypes[item.Name]
		if !ok {
			t.Fatalf("unexpected catalog item %q", item.Name)
		}
		if typeID != wantType {
			t.Fatalf("%s type got %d, want %d", item.Name, typeID, wantType)
		}
	}
	if _, ok := assetTypes["sample.menuassets.typetree.json"]; ok {
		t.Fatalf("TypeTree sidecar was packed as an asset")
	}
	if assetTypes["DepthLUT"] != aba.ClassIDMonoScript {
		t.Fatalf("DepthLUT type got %d, want MonoScript", assetTypes["DepthLUT"])
	}
	foundMonoBehaviour := false
	for _, entry := range af.GetAssetEntries() {
		if entry.TypeId == aba.ClassIDMonoBehaviour {
			foundMonoBehaviour = true
			if entry.PathId != -1466831684398908746 {
				t.Fatalf("MonoBehaviour PathID got %d, want -1466831684398908746", entry.PathId)
			}
			break
		}
	}
	if !foundMonoBehaviour {
		t.Fatalf("MonoBehaviour raw object not found in packed .aba")
	}
	containerNames, err := af.GetAssetBundleContainerMap()
	if err != nil {
		t.Fatalf("GetAssetBundleContainerMap: %v", err)
	}
	if containerNames[-1466831684398908746] != "DepthLUT" {
		t.Fatalf("MonoBehaviour load name got %q, want DepthLUT", containerNames[-1466831684398908746])
	}
	if assetTypes["type95_internal"] != 95 {
		t.Fatalf("type95_internal type got %d, want Type_95", assetTypes["type95_internal"])
	}
}

func TestPackService_PackToAbaAndCtSkipsDerivedUnpackArtifacts(t *testing.T) {
	tmpDir := t.TempDir()
	inputDir := filepath.Join(tmpDir, "unpacked")
	for _, dir := range []string{"Texture2D", "Sprite", "Mesh"} {
		if err := os.MkdirAll(filepath.Join(inputDir, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}

	rawTexData, err := os.ReadFile(filepath.Join("..", "..", "testdata", "kces_assets", "cm3d2_megane002.tex.bytes"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "Texture2D", "cm3d2_megane002.tex.bytes"), rawTexData, 0644); err != nil {
		t.Fatal(err)
	}
	texPNG, err := os.ReadFile(filepath.Join("..", "..", "testdata", "kces_assets", "cm3d2_megane002.tex.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "Texture2D", "cm3d2_megane002.tex.png"), texPNG, 0644); err != nil {
		t.Fatal(err)
	}
	hashedTextureName := "sactx-0-128x64-DXT5_BC3-nt008_team_star_glass.partsassets-e3baac46"
	hashedTextureData, err := os.ReadFile(filepath.Join("..", "..", "testdata", "kces_assets", "cm3d2_megane002.tex.bytes"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "Texture2D", hashedTextureName+".bytes"), hashedTextureData, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "Texture2D", hashedTextureName+".png"), texPNG, 0644); err != nil {
		t.Fatal(err)
	}

	rawSpriteData, err := os.ReadFile(filepath.Join("..", "..", "testdata", "kces_assets", "cm3d2_megane002_i_.tex.sprite.bytes"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "Sprite", "cm3d2_megane002_i_.tex.sprite.bytes"), rawSpriteData, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "Sprite", "cm3d2_megane002_i_.tex.png"), texPNG, 0644); err != nil {
		t.Fatal(err)
	}

	rawMeshData, err := os.ReadFile(filepath.Join("..", "..", "testdata", "kces_assets", "cm3d2_megane002.mmesh.bytes"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "Mesh", "cm3d2_megane002.mmesh.bytes"), rawMeshData, 0644); err != nil {
		t.Fatal(err)
	}
	crmesh, err := os.ReadFile(filepath.Join("..", "..", "testdata", "kces_assets", "cm3d2_megane002.mmesh.crmesh"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "Mesh", "cm3d2_megane002.mmesh.crmesh"), crmesh, 0644); err != nil {
		t.Fatal(err)
	}

	service := &PackService{}
	if err := service.PackToAbaAndCt(inputDir, "unpacked"); err != nil {
		t.Fatalf("PackToAbaAndCt: %v", err)
	}

	ctFile, err := os.Open(filepath.Join(tmpDir, "unpacked.ct"))
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
	gotNames := map[string]bool{}
	for _, item := range catalog.Items {
		gotNames[item.Name] = true
	}
	wantNames := []string{
		"cm3d2_megane002.tex",
		hashedTextureName,
		"cm3d2_megane002_i_.tex",
		"cm3d2_megane002.mmesh",
	}
	for _, name := range wantNames {
		if !gotNames[name] {
			t.Fatalf("missing catalog item %q in %+v", name, gotNames)
		}
		delete(gotNames, name)
	}
	if len(gotNames) != 0 {
		t.Fatalf("unexpected derived catalog items: %+v", gotNames)
	}

	abaData, err := os.ReadFile(filepath.Join(tmpDir, "unpacked.aba"))
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
	assetTypes := assetTypesByLoadName(t, af)
	if len(assetTypes) != 5 {
		t.Fatalf("expected 4 assets plus AssetBundle, got %+v", assetTypes)
	}
	if assetTypes["cm3d2_megane002.tex"] != aba.ClassIDTexture2D {
		t.Fatalf("texture type got %d", assetTypes["cm3d2_megane002.tex"])
	}
	if assetTypes[hashedTextureName] != aba.ClassIDTexture2D {
		t.Fatalf("hashed texture type got %d", assetTypes[hashedTextureName])
	}
	if assetTypes["cm3d2_megane002_i_.tex"] != aba.ClassIDSprite {
		t.Fatalf("sprite type got %d", assetTypes["cm3d2_megane002_i_.tex"])
	}
	if assetTypes["cm3d2_megane002.mmesh"] != aba.ClassIDMesh {
		t.Fatalf("mesh type got %d", assetTypes["cm3d2_megane002.mmesh"])
	}
}

func TestPackService_PackToAbaAndCtUsesRawMetaLoadName(t *testing.T) {
	tmpDir := t.TempDir()
	inputDir := filepath.Join(tmpDir, "load_name_pack")
	textureDir := filepath.Join(inputDir, "Texture2D")
	if err := os.MkdirAll(textureDir, 0755); err != nil {
		t.Fatal(err)
	}

	rawTexData, err := os.ReadFile(filepath.Join("..", "..", "testdata", "kces_assets", "cm3d2_megane002.tex.bytes"))
	if err != nil {
		t.Fatal(err)
	}
	rawPath := filepath.Join(textureDir, "sactx-0-128x64-DXT5_BC3-nt008_team_star_glass.partsassets-e3baac46.bytes")
	if err := os.WriteFile(rawPath, rawTexData, 0644); err != nil {
		t.Fatal(err)
	}
	loadName := "sactx-0-128x64-DXT5|BC3-nt008_team_star_glass.partsassets-e3baac46"
	if err := writeAssetMeta(rawPath, -123456789, loadName); err != nil {
		t.Fatal(err)
	}

	service := &PackService{}
	if err := service.PackToAbaAndCt(inputDir, "load_name_pack"); err != nil {
		t.Fatalf("PackToAbaAndCt: %v", err)
	}

	ctFile, err := os.Open(filepath.Join(tmpDir, "load_name_pack.ct"))
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
	catalogName := "sactx-0-128x64-DXT5_BC3-nt008_team_star_glass.partsassets-e3baac46"
	if len(catalog.Items) != 1 || catalog.Items[0].Name != catalogName {
		t.Fatalf("catalog item got %+v, want %q", catalog.Items, catalogName)
	}
	enl, err := ct.DecodeExtensionNameListFromCt(table, ".partsassets-e3baac46")
	if err != nil {
		t.Fatalf("DecodeExtensionNameListFromCt: %v", err)
	}
	if len(enl.Data) != 1 || enl.Data[0].Name != catalogName {
		t.Fatalf("ExtensionNameList got %+v, want %q", enl.Data, catalogName)
	}

	abaData, err := os.ReadFile(filepath.Join(tmpDir, "load_name_pack.aba"))
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
	info := af.GetAssetInfoByPathID(-123456789)
	if info == nil || info.TypeId != aba.ClassIDTexture2D {
		t.Fatalf("raw Texture2D PathID not preserved: %+v", info)
	}
	containerNames, err := af.GetAssetBundleContainerMap()
	if err != nil {
		t.Fatalf("GetAssetBundleContainerMap: %v", err)
	}
	if containerNames[-123456789] != loadName {
		t.Fatalf("container load name got %q, want %q", containerNames[-123456789], loadName)
	}
}

func TestPackService_PackToAbaAndCtUsesTextAssetMeta(t *testing.T) {
	tmpDir := t.TempDir()
	inputDir := filepath.Join(tmpDir, "textasset_meta_pack")
	textDir := filepath.Join(inputDir, "TextAsset")
	if err := os.MkdirAll(textDir, 0755); err != nil {
		t.Fatal(err)
	}

	assetPath := filepath.Join(textDir, "parts_personal002.menuassets")
	if err := os.WriteFile(assetPath, []byte("menu data"), 0644); err != nil {
		t.Fatal(err)
	}
	loadName := "assets/gamedata/parts/parts_personal002/parts_personal002.menuassets.bytes"
	if err := writeAssetMeta(assetPath, -2222, loadName); err != nil {
		t.Fatal(err)
	}

	service := &PackService{}
	if err := service.PackToAbaAndCt(inputDir, "textasset_meta_pack"); err != nil {
		t.Fatalf("PackToAbaAndCt: %v", err)
	}

	ctFile, err := os.Open(filepath.Join(tmpDir, "textasset_meta_pack.ct"))
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
	if len(catalog.Items) != 1 || catalog.Items[0].Name != "parts_personal002.menuassets" {
		t.Fatalf("catalog item got %+v, want parts_personal002.menuassets", catalog.Items)
	}
	enl, err := ct.DecodeExtensionNameListFromCt(table, ".menuassets")
	if err != nil {
		t.Fatalf("DecodeExtensionNameListFromCt: %v", err)
	}
	if len(enl.Data) != 1 || enl.Data[0].Name != "parts_personal002.menuassets" {
		t.Fatalf("ExtensionNameList got %+v, want parts_personal002.menuassets", enl.Data)
	}

	abaData, err := os.ReadFile(filepath.Join(tmpDir, "textasset_meta_pack.aba"))
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
	info := af.GetAssetInfoByPathID(-2222)
	if info == nil || info.TypeId != aba.ClassIDTextAsset {
		t.Fatalf("TextAsset PathID not preserved: %+v", info)
	}
	containerNames, err := af.GetAssetBundleContainerMap()
	if err != nil {
		t.Fatalf("GetAssetBundleContainerMap: %v", err)
	}
	if containerNames[-2222] != loadName {
		t.Fatalf("container load name got %q, want %q", containerNames[-2222], loadName)
	}
}

func TestPackService_PackToAbaAndCtInfersRootRawUnityByteSuffixes(t *testing.T) {
	tmpDir := t.TempDir()
	inputDir := filepath.Join(tmpDir, "root_raw_suffixes")
	if err := os.MkdirAll(inputDir, 0755); err != nil {
		t.Fatal(err)
	}

	monoScriptData, err := os.ReadFile(filepath.Join("..", "..", "testdata", "kces_assets", "DepthLUT.monoscript.bytes"))
	if err != nil {
		t.Fatal(err)
	}
	monoScriptPath := filepath.Join(inputDir, "DepthLUT.monoscript.bytes")
	if err := os.WriteFile(monoScriptPath, monoScriptData, 0644); err != nil {
		t.Fatal(err)
	}
	if err := writeAssetMeta(monoScriptPath, -63133960937130332, "DepthLUT"); err != nil {
		t.Fatal(err)
	}

	monoBehaviourData, err := os.ReadFile(filepath.Join("..", "..", "testdata", "kces_assets", "DepthLUT.monobehaviour.bytes"))
	if err != nil {
		t.Fatal(err)
	}
	monoBehaviourPath := filepath.Join(inputDir, "DepthLUT.monobehaviour.bytes")
	if err := os.WriteFile(monoBehaviourPath, monoBehaviourData, 0644); err != nil {
		t.Fatal(err)
	}
	if err := writeAssetMeta(monoBehaviourPath, -1466831684398908746, "DepthLUT.behaviour"); err != nil {
		t.Fatal(err)
	}
	rootRawSuffixSamples := []struct {
		fileName string
		loadName string
		pathID   int64
		classID  int32
	}{
		{"RootTexture.texture.bytes", "RootTexture", -2101, aba.ClassIDTexture2D},
		{"RootMaterial.material.bytes", "RootMaterial", -2102, aba.ClassIDMaterial},
		{"RootShader.shader.bytes", "RootShader", -2103, aba.ClassIDShader},
		{"RootAudio.audioclip.bytes", "RootAudio", -2104, aba.ClassIDAudioClip},
		{"RootFont.font.bytes", "RootFont", -2105, aba.ClassIDFont},
	}
	for _, sample := range rootRawSuffixSamples {
		rawPath := filepath.Join(inputDir, sample.fileName)
		if err := os.WriteFile(rawPath, monoScriptData, 0644); err != nil {
			t.Fatal(err)
		}
		if err := writeAssetMeta(rawPath, sample.pathID, sample.loadName); err != nil {
			t.Fatal(err)
		}
	}

	service := &PackService{}
	if err := service.PackToAbaAndCt(inputDir, "root_raw_suffixes"); err != nil {
		t.Fatalf("PackToAbaAndCt: %v", err)
	}

	abaData, err := os.ReadFile(filepath.Join(tmpDir, "root_raw_suffixes.aba"))
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

	if info := af.GetAssetInfoByPathID(-63133960937130332); info == nil || info.TypeId != aba.ClassIDMonoScript {
		t.Fatalf("MonoScript raw suffix was not preserved as MonoScript: %+v", info)
	}
	if info := af.GetAssetInfoByPathID(-1466831684398908746); info == nil || info.TypeId != aba.ClassIDMonoBehaviour {
		t.Fatalf("MonoBehaviour raw suffix was not preserved as MonoBehaviour: %+v", info)
	}
	for _, sample := range rootRawSuffixSamples {
		if info := af.GetAssetInfoByPathID(sample.pathID); info == nil || info.TypeId != sample.classID {
			t.Fatalf("%s raw suffix got %+v, want ClassID %d", sample.fileName, info, sample.classID)
		}
	}
	containerNames, err := af.GetAssetBundleContainerMap()
	if err != nil {
		t.Fatalf("GetAssetBundleContainerMap: %v", err)
	}
	if containerNames[-63133960937130332] != "DepthLUT" {
		t.Fatalf("MonoScript load name got %q", containerNames[-63133960937130332])
	}
	if containerNames[-1466831684398908746] != "DepthLUT.behaviour" {
		t.Fatalf("MonoBehaviour load name got %q", containerNames[-1466831684398908746])
	}
	for _, sample := range rootRawSuffixSamples {
		if containerNames[sample.pathID] != sample.loadName {
			t.Fatalf("%s load name got %q, want %q", sample.fileName, containerNames[sample.pathID], sample.loadName)
		}
	}
}
