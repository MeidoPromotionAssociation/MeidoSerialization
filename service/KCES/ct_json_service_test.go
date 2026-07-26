package KCES

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/msgpack"
)

func TestCtEnvelopeVirtualAssetCatalogRoundTrip(t *testing.T) {
	itemName := "local_test.menuassets"
	catalogName := "local_test"
	extension := ".menuassets"
	assetPath := "Assets/GameData/parts/local_test/local_test.menuassets"
	catalog := &ct.AssetBundleCatalog{
		Kind:          ct.CatalogKindVirtualAsset,
		Version:       1000,
		CatalogType:   ct.CatalogTypeParts,
		PackageType:   ct.PackageTypePlugin,
		Priority:      5,
		Name:          &catalogName,
		Hash:          ct.HashStringIgnoreCase("local_test"),
		CreateTime:    123456789,
		ExtensionList: []*string{&extension},
		VirtualItems: []*ct.VirtualCatalogItem{{
			AssetPath: &assetPath,
			Name:      &itemName,
			Hash:      ct.HashStringIgnoreCase(itemName),
		}},
	}
	catalogData, err := ct.EncodeCatalog(catalog)
	if err != nil {
		t.Fatalf("EncodeCatalog: %v", err)
	}
	extensionList := &ct.ExtensionNameList{
		Extension: &extension,
		Data:      []*ct.ExtensionNamePack{{Name: &itemName, Hash: ct.HashStringIgnoreCase(itemName)}},
	}
	extensionData, err := ct.EncodeExtensionNameList(extensionList)
	if err != nil {
		t.Fatalf("EncodeExtensionNameList: %v", err)
	}
	compressedCatalog, err := msgpack.CompressLz4BlockArray(catalogData)
	if err != nil {
		t.Fatalf("Compress catalog: %v", err)
	}
	compressedExtension, err := msgpack.CompressLz4BlockArray(extensionData)
	if err != nil {
		t.Fatalf("Compress extension list: %v", err)
	}
	table := &ct.ContentTable{
		Version: 1000,
		Directories: map[string]ct.VirtualDirectoryMetadata{
			"empty": {
				Version: 1000,
			},
		},
	}
	table.AddFile("catalog", compressedCatalog)
	table.AddFile(".menuassets", compressedExtension)
	envelope, err := readCtEnvelopeFromTable(table)
	if err != nil {
		t.Fatalf("readCtEnvelopeFromTable: %v", err)
	}
	if envelope.Catalog.Kind != ct.CatalogKindVirtualAsset {
		t.Fatalf("envelope catalog kind = %q", envelope.Catalog.Kind)
	}
	if envelope.Version != table.Version || !reflect.DeepEqual(envelope.Directories, table.Directories) {
		t.Fatalf("content-table typed metadata missing from envelope: %+v", envelope)
	}
	jsonData, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("Marshal envelope: %v", err)
	}
	if !bytes.Contains(jsonData, []byte(`"kind":"virtualAsset"`)) {
		t.Fatalf("envelope JSON lacks explicit virtual catalog kind: %s", jsonData)
	}
	var fromJSON CtEnvelope
	if err := json.Unmarshal(jsonData, &fromJSON); err != nil {
		t.Fatalf("Unmarshal envelope: %v", err)
	}
	roundTripTable, err := buildContentTableFromCtEnvelope(&fromJSON)
	if err != nil {
		t.Fatalf("buildContentTableFromCtEnvelope: %v", err)
	}
	decoded, err := ct.DecodeCatalogFromCt(roundTripTable)
	if err != nil {
		t.Fatalf("DecodeCatalogFromCt: %v", err)
	}
	if !reflect.DeepEqual(decoded, catalog) {
		t.Fatalf("virtual catalog changed after envelope round-trip\ngot:  %+v\nwant: %+v", decoded, catalog)
	}
	if roundTripTable.Version != table.Version || !reflect.DeepEqual(roundTripTable.Directories, table.Directories) {
		t.Fatalf("content-table typed metadata changed after envelope round-trip:\n got  %+v\n want %+v", roundTripTable, table)
	}
	var container bytes.Buffer
	if err := ct.WriteContentTable(&container, roundTripTable); err != nil {
		t.Fatalf("WriteContentTable: %v", err)
	}
	redecodedTable, err := ct.ReadContentTable(bytes.NewReader(container.Bytes()))
	if err != nil {
		t.Fatalf("ReadContentTable: %v", err)
	}
	if redecodedTable.Version != table.Version || !reflect.DeepEqual(redecodedTable.Directories, table.Directories) {
		t.Fatalf("content-table typed metadata changed on final wire:\n got  %+v\n want %+v", redecodedTable, table)
	}
}

func TestCtEnvelopeRequiresNonNullCatalog(t *testing.T) {
	var envelope CtEnvelope
	if err := decodeStrictJSON(
		[]byte(`{"format":"kces-content-table","version":1000,"catalog":null}`),
		&envelope,
		"KCES content-table JSON",
	); err == nil {
		t.Fatal("content-table JSON accepted catalog:null")
	}

	if err := decodeStrictJSON(
		[]byte(`{"format":"kces-content-table","version":1000}`),
		&envelope,
		"KCES content-table JSON",
	); err == nil {
		t.Fatal("content-table JSON accepted a missing catalog")
	}
}

func TestCtEnvelopeRejectsMissingRequiredFieldsAndEmptyExtensionKey(t *testing.T) {
	tests := map[string]string{
		"missing envelope version": `{"format":"kces-content-table","catalog":{"kind":"assetBundle","version":1000,"catalogType":4096,"packageType":1,"priority":0,"name":null,"subName":null,"hash":0,"createTime":0,"extensionList":[],"isEncrypted":false,"resourceFileNames":[],"items":[]}}`,
		"missing catalog field":    `{"format":"kces-content-table","version":1000,"catalog":{"kind":"assetBundle","catalogType":4096,"packageType":1,"priority":0,"name":null,"subName":null,"hash":0,"createTime":0,"extensionList":[],"isEncrypted":false,"resourceFileNames":[],"items":[]}}`,
		"missing catalog item":     `{"format":"kces-content-table","version":1000,"catalog":{"kind":"assetBundle","version":1000,"catalogType":4096,"packageType":1,"priority":0,"name":null,"subName":null,"hash":0,"createTime":0,"extensionList":[],"isEncrypted":false,"resourceFileNames":[],"items":[{"resourceIndex":0,"name":null}]}}`,
		"missing extension data":   `{"format":"kces-content-table","version":1000,"catalog":{"kind":"assetBundle","version":1000,"catalogType":4096,"packageType":1,"priority":0,"name":null,"subName":null,"hash":0,"createTime":0,"extensionList":[".x"],"isEncrypted":false,"resourceFileNames":[],"items":[]},"extensionNameLists":{".x":{"extention":".x"}}}`,
		"missing extension hash":   `{"format":"kces-content-table","version":1000,"catalog":{"kind":"assetBundle","version":1000,"catalogType":4096,"packageType":1,"priority":0,"name":null,"subName":null,"hash":0,"createTime":0,"extensionList":[".x"],"isEncrypted":false,"resourceFileNames":[],"items":[]},"extensionNameLists":{".x":{"extention":".x","data":[{"name":"a"}]}}}`,
		"missing file payload":     `{"format":"kces-content-table","version":1000,"catalog":{"kind":"assetBundle","version":1000,"catalogType":4096,"packageType":1,"priority":0,"name":null,"subName":null,"hash":0,"createTime":0,"extensionList":[],"isEncrypted":false,"resourceFileNames":[],"items":[]},"files":[{"name":"x"}]}`,
		"empty extension key":      `{"format":"kces-content-table","version":1000,"catalog":{"kind":"assetBundle","version":1000,"catalogType":4096,"packageType":1,"priority":0,"name":null,"subName":null,"hash":0,"createTime":0,"extensionList":[],"isEncrypted":false,"resourceFileNames":[],"items":[]},"extensionNameLists":{"":{"extention":".x","data":[]}}}`,
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			var envelope CtEnvelope
			if err := decodeStrictJSON([]byte(data), &envelope, "KCES content-table JSON"); err == nil {
				t.Fatalf("accepted invalid content-table JSON %s", data)
			}
		})
	}

	envelope := &CtEnvelope{
		Catalog: ct.AssetBundleCatalog{
			Kind:              ct.CatalogKindAssetBundle,
			ResourceFileNames: []*string{},
			ExtensionList:     []*string{},
			Items:             []*ct.CatalogItem{},
		},
		ExtensionNameLists: map[string]*ct.ExtensionNameList{
			"": {Extension: nil, Data: []*ct.ExtensionNamePack{}},
		},
	}
	if _, err := buildContentTableFromCtEnvelope(envelope); err == nil {
		t.Fatal("programmatic content-table envelope accepted an empty ExtensionNameLists key")
	}
}

func TestCtService_FixedSamplesJSONRoundTrip(t *testing.T) {
	samples := []struct {
		file     string
		name     string
		wantExts []string
	}{
		{
			file:     "cm3d2_megane002.ct",
			name:     "cm3d2_megane002",
			wantExts: []string{".materialassets", ".menuassets", ".mmesh", ".model", ".partsatlas", ".tex"},
		},
		{
			file:     "nt008_team_star_glass.ct",
			name:     "nt008_team_star_glass",
			wantExts: []string{".materialassets", ".menuassets", ".mmesh", ".model", ".partsassets", ".tex"},
		},
		{
			file:     "partsmeta.ct",
			name:     "partsmeta",
			wantExts: []string{".db2conf", ".dbcol", ".dbconf", ".dsb2conf", ".dsbconf", ".dsl2conf", ".dslcol", ".pmatassets", ".psk"},
		},
	}

	service := &CtService{}
	for _, sample := range samples {
		sample := sample
		t.Run(sample.file, func(t *testing.T) {
			inputPath := filepath.Join("..", "..", "testdata", "aba", sample.file)
			envelope, err := service.ReadCtEnvelope(inputPath)
			if err != nil {
				t.Fatalf("ReadCtEnvelope: %v", err)
			}
			if envelope.Format != CtEnvelopeFormat {
				t.Fatalf("format got %q, want %q", envelope.Format, CtEnvelopeFormat)
			}
			if envelope.Catalog.Name == nil || *envelope.Catalog.Name != sample.name {
				t.Fatalf("unexpected catalog: %+v", envelope.Catalog)
			}
			if len(envelope.Catalog.ResourceFileNames) == 0 || len(envelope.Catalog.Items) == 0 {
				t.Fatalf("incomplete catalog: %+v", envelope.Catalog)
			}
			for _, ext := range sample.wantExts {
				enl := envelope.ExtensionNameLists[ext]
				if enl == nil {
					t.Fatalf("missing ExtensionNameList %q in %+v", ext, envelope.Catalog.ExtensionList)
				}
				if enl.Extension == nil || *enl.Extension != ext || len(enl.Data) == 0 {
					t.Fatalf("incomplete ExtensionNameList %q: %+v", ext, enl)
				}
			}

			tmpDir := t.TempDir()
			jsonPath := filepath.Join(tmpDir, sample.file+".json")
			outPath := filepath.Join(tmpDir, sample.file)
			if err := service.ConvertCtToJson(TestConversionContext, inputPath, jsonPath, TestConversionMaxOutput); err != nil {
				t.Fatalf("ConvertCtToJson: %v", err)
			}
			if !IsKCESCtJSONFile(jsonPath) {
				t.Fatalf("converted JSON was not detected as KCES .ct JSON")
			}
			if err := service.ConvertJsonToCt(TestConversionContext, jsonPath, outPath, TestConversionMaxOutput); err != nil {
				t.Fatalf("ConvertJsonToCt: %v", err)
			}
			assertCtSemanticallyEqual(t, inputPath, outPath)
		})
	}
}

func assertCtSemanticallyEqual(t *testing.T, wantPath string, gotPath string) {
	t.Helper()
	wantTable := readContentTableForTest(t, wantPath)
	gotTable := readContentTableForTest(t, gotPath)

	wantCatalog, err := ct.DecodeCatalogFromCt(wantTable)
	if err != nil {
		t.Fatalf("decode original catalog: %v", err)
	}
	gotCatalog, err := ct.DecodeCatalogFromCt(gotTable)
	if err != nil {
		t.Fatalf("decode round-trip catalog: %v", err)
	}
	if !reflect.DeepEqual(gotCatalog, wantCatalog) {
		t.Fatalf("catalog changed after round-trip\ngot:  %+v\nwant: %+v", gotCatalog, wantCatalog)
	}

	for index, ext := range wantCatalog.ExtensionList {
		if ext == nil {
			t.Fatalf("catalog extensionList[%d] is null", index)
		}
		wantEnl, err := ct.DecodeExtensionNameListFromCt(wantTable, *ext)
		if err != nil {
			t.Fatalf("decode original ExtensionNameList %q: %v", *ext, err)
		}
		gotEnl, err := ct.DecodeExtensionNameListFromCt(gotTable, *ext)
		if err != nil {
			t.Fatalf("decode round-trip ExtensionNameList %q: %v", *ext, err)
		}
		if !reflect.DeepEqual(gotEnl, wantEnl) {
			t.Fatalf("ExtensionNameList %q changed after round-trip\ngot:  %+v\nwant: %+v", *ext, gotEnl, wantEnl)
		}
	}

	wantNames := wantTable.GetFileNames()
	gotNames := gotTable.GetFileNames()
	if len(gotNames) != len(wantNames) {
		t.Fatalf("virtual file count got %d, want %d", len(gotNames), len(wantNames))
	}
	for _, name := range wantNames {
		if name == "catalog" || containsString(wantCatalog.ExtensionList, name) {
			continue
		}
		wantData, err := wantTable.GetFileData(name)
		if err != nil {
			t.Fatalf("read original virtual file %q: %v", name, err)
		}
		gotData, err := gotTable.GetFileData(name)
		if err != nil {
			t.Fatalf("read round-trip virtual file %q: %v", name, err)
		}
		if !bytes.Equal(gotData, wantData) {
			t.Fatalf("virtual file %q changed after round-trip", name)
		}
	}
}

func readContentTableForTest(t *testing.T, path string) *ct.ContentTable {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	table, err := ct.ReadContentTable(f)
	if err != nil {
		t.Fatalf("ReadContentTable(%s): %v", path, err)
	}
	return table
}

func containsString(values []*string, want string) bool {
	for _, value := range values {
		if value != nil && *value == want {
			return true
		}
	}
	return false
}
