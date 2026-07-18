package KCES

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

func TestPackServiceRepackAbaPreservesBundleHeaderContext(t *testing.T) {
	var source bytes.Buffer
	wantOptions := &aba.BundleWriteOptions{
		EngineVersion:     "2022.3.62f2",
		GenerationVersion: "custom-generation",
		Version:           8,
		Compress:          true,
	}
	if err := aba.WriteBundle(&source, []aba.BundleFileEntry{{
		Name: "resources/sample.resource",
		Data: []byte("resource-data"),
	}}, wantOptions); err != nil {
		t.Fatalf("write source bundle: %v", err)
	}

	tmpDir := t.TempDir()
	sourcePath := filepath.Join(tmpDir, "source.aba")
	if err := os.WriteFile(sourcePath, source.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	unpackedDir := filepath.Join(tmpDir, "unpacked")
	if err := (&AbaService{}).UnpackAba(sourcePath, unpackedDir); err != nil {
		t.Fatalf("UnpackAba: %v", err)
	}
	metaData, err := os.ReadFile(filepath.Join(unpackedDir, abaBundleMetaFileName))
	if err != nil {
		t.Fatalf("read bundle metadata sidecar: %v", err)
	}
	if !bytes.Contains(metaData, []byte(`"bundleVersion": 8`)) || !bytes.Contains(metaData, []byte(`"engineVersion": "2022.3.62f2"`)) {
		t.Fatalf("bundle metadata sidecar lacks source context: %s", metaData)
	}

	outPath := filepath.Join(tmpDir, "repacked.aba")
	if err := (&PackService{}).RepackAba(unpackedDir, outPath); err != nil {
		t.Fatalf("RepackAba: %v", err)
	}
	repackedData, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	repacked, err := aba.ReadBundle(bytes.NewReader(repackedData))
	if err != nil {
		t.Fatalf("read repacked bundle: %v", err)
	}
	if repacked.Header.Version != wantOptions.Version || repacked.Header.EngineVersion != wantOptions.EngineVersion || repacked.Header.GenerationVersion != wantOptions.GenerationVersion {
		t.Fatalf("repacked header got %+v, want version=%d engine=%q generation=%q", repacked.Header, wantOptions.Version, wantOptions.EngineVersion, wantOptions.GenerationVersion)
	}
	if len(repacked.BlockInfo.DirectoryInfos) != 1 || repacked.BlockInfo.DirectoryInfos[0].Name != "resources/sample.resource" {
		t.Fatalf("repacked directories include metadata sidecar or lost payload: %+v", repacked.BlockInfo.DirectoryInfos)
	}
	payload, err := repacked.GetFileData(0)
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) != "resource-data" {
		t.Fatalf("repacked payload = %q", payload)
	}
}

func TestPackServiceRepackAbaUsesExistingAssetMetaContext(t *testing.T) {
	dir := t.TempDir()
	assetPath := filepath.Join(dir, "sample.resource")
	if err := os.WriteFile(assetPath, []byte("resource-data"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := writeRawAssetMeta(assetPath, rawAssetMeta{
		EngineVersion:     "2022.3.35f1",
		BundleVersion:     8,
		GenerationVersion: "legacy-sidecar-generation",
	}); err != nil {
		t.Fatal(err)
	}

	outPath := filepath.Join(t.TempDir(), "repacked.aba")
	if err := (&PackService{}).RepackAba(dir, outPath); err != nil {
		t.Fatalf("RepackAba: %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := aba.ReadBundle(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Header.Version != 8 || bundle.Header.EngineVersion != "2022.3.35f1" || bundle.Header.GenerationVersion != "legacy-sidecar-generation" {
		t.Fatalf("repacked header did not use existing asset metadata: %+v", bundle.Header)
	}
	if len(bundle.BlockInfo.DirectoryInfos) != 1 || bundle.BlockInfo.DirectoryInfos[0].Name != "sample.resource" {
		t.Fatalf("asset metadata sidecar was packed as a bundle entry: %+v", bundle.BlockInfo.DirectoryInfos)
	}
}

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
	// MonoScript and MonoBehaviour are distinct m_Container entries. Reusing
	// "DepthLUT" for both made the fixture ambiguous to AssetBundle.LoadAsset;
	// the extracted sidecar must preserve a unique load key per object.
	if err := writeAssetMeta(monoBehaviourPath, -1466831684398908746, "DepthLUT.behaviour"); err != nil {
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
	if containerNames[-1466831684398908746] != "DepthLUT.behaviour" {
		t.Fatalf("MonoBehaviour load name got %q, want DepthLUT.behaviour", containerNames[-1466831684398908746])
	}
	if assetTypes["type95_internal"] != 95 {
		t.Fatalf("type95_internal type got %d, want Type_95", assetTypes["type95_internal"])
	}
}

func TestPackServicePreservesStreamingSidecarAsRawBundleEntry(t *testing.T) {
	tmpDir := t.TempDir()
	inputDir := filepath.Join(tmpDir, "stream_pack")
	if err := os.MkdirAll(inputDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "asset.menuassets"), []byte("menu"), 0644); err != nil {
		t.Fatal(err)
	}
	sidecarName := "CAB-original.resS"
	sidecarData := []byte{1, 2, 3, 4, 5}
	if err := os.WriteFile(filepath.Join(inputDir, sidecarName), sidecarData, 0644); err != nil {
		t.Fatal(err)
	}

	if err := (&PackService{}).PackToAbaAndCt(inputDir, "stream_pack"); err != nil {
		t.Fatalf("PackToAbaAndCt: %v", err)
	}
	bundleData, err := os.ReadFile(filepath.Join(tmpDir, "stream_pack.aba"))
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := aba.ReadBundle(bytes.NewReader(bundleData))
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.BlockInfo.DirectoryInfos) != 2 {
		t.Fatalf("bundle directory count = %d, want CAB + sidecar", len(bundle.BlockInfo.DirectoryInfos))
	}
	var found bool
	for i, dir := range bundle.BlockInfo.DirectoryInfos {
		if dir.Name != sidecarName {
			continue
		}
		found = true
		if dir.IsSerialized() {
			t.Fatalf("sidecar %q was incorrectly marked as SerializedFile", dir.Name)
		}
		got, err := bundle.GetFileData(i)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, sidecarData) {
			t.Fatalf("sidecar bytes = %v, want %v", got, sidecarData)
		}
	}
	if !found {
		t.Fatalf("sidecar %q missing from bundle directories", sidecarName)
	}

	ctFile, err := os.Open(filepath.Join(tmpDir, "stream_pack.ct"))
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
	for _, item := range catalog.Items {
		if item.Name == sidecarName {
			t.Fatalf("raw sidecar was incorrectly inserted into Catalog.Items: %+v", item)
		}
	}
	for _, ext := range catalog.ExtensionList {
		if strings.EqualFold(ext, ".resS") {
			t.Fatalf("raw sidecar extension was incorrectly cataloged: %q", ext)
		}
	}
}

func TestUnpackThenPackPreservesRealStreamedTextureData(t *testing.T) {
	sample := filepath.Join("..", "..", "testdata", "aba", "nt008_chignon.aba")
	if _, err := os.Stat(sample); err != nil {
		t.Skipf("streamed-texture sample unavailable: %v", err)
	}
	originalFile, err := os.Open(sample)
	if err != nil {
		t.Fatal(err)
	}
	originalBundle, err := aba.ReadBundle(originalFile)
	if err != nil {
		originalFile.Close()
		t.Fatal(err)
	}
	var sidecarName string
	var originalSidecar []byte
	for i, dir := range originalBundle.BlockInfo.DirectoryInfos {
		if dir.IsSerialized() || !strings.EqualFold(filepath.Ext(dir.Name), ".resS") {
			continue
		}
		sidecarName = dir.Name
		originalSidecar, err = originalBundle.GetFileData(i)
		if err != nil {
			originalFile.Close()
			t.Fatal(err)
		}
		break
	}
	originalFile.Close()
	if sidecarName == "" {
		t.Fatal("sample does not contain the expected .resS sidecar")
	}
	tmpDir := t.TempDir()
	unpacked := filepath.Join(tmpDir, "stream_roundtrip")
	if err := (&AbaService{}).UnpackAba(sample, unpacked); err != nil {
		t.Fatalf("UnpackAba: %v", err)
	}
	if err := (&PackService{}).PackToAbaAndCt(unpacked, "stream_roundtrip"); err != nil {
		t.Fatalf("PackToAbaAndCt: %v", err)
	}

	bundleFile, err := os.Open(filepath.Join(tmpDir, "stream_roundtrip.aba"))
	if err != nil {
		t.Fatal(err)
	}
	defer bundleFile.Close()
	bundle, err := aba.ReadBundle(bundleFile)
	if err != nil {
		t.Fatal(err)
	}
	var assetsFile *aba.AssetsFile
	var repackedSidecar []byte
	for i, dir := range bundle.BlockInfo.DirectoryInfos {
		if !dir.IsSerialized() {
			if dir.Name == sidecarName {
				repackedSidecar, err = bundle.GetFileData(i)
				if err != nil {
					t.Fatal(err)
				}
			}
			continue
		}
		data, err := bundle.GetFileData(i)
		if err != nil {
			t.Fatal(err)
		}
		assetsFile, err = aba.ReadAssetsFile(data)
		if err != nil {
			t.Fatal(err)
		}
	}
	if repackedSidecar == nil || assetsFile == nil {
		t.Fatalf("repacked bundle missing SerializedFile or .resS sidecar")
	}
	if !bytes.Equal(repackedSidecar, originalSidecar) {
		t.Fatalf("repacked .resS bytes differ: got %d bytes, want %d", len(repackedSidecar), len(originalSidecar))
	}

	streamedTextures := 0
	for i := range assetsFile.Metadata.AssetInfos {
		info := &assetsFile.Metadata.AssetInfos[i]
		if info.TypeId != aba.ClassIDTexture2D {
			continue
		}
		raw, err := assetsFile.GetAssetData(info)
		if err != nil {
			t.Fatalf("read repacked Texture2D PathID %d: %v", info.PathId, err)
		}
		// The generated SerializedFile intentionally has no embedded TypeTree,
		// so verify the preserved raw object reference directly. Unity's stream
		// path contains the exact sidecar basename as an aligned string.
		if bytes.Contains(raw, []byte(sidecarName)) {
			streamedTextures++
		}
	}
	if streamedTextures == 0 {
		t.Fatal("sample round-trip did not exercise any streamed Texture2D")
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
