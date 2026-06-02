package KCES

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

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
			if envelope.Catalog == nil || envelope.Catalog.Name != sample.name {
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
				if enl.Extension != ext || len(enl.Data) == 0 {
					t.Fatalf("incomplete ExtensionNameList %q: %+v", ext, enl)
				}
			}

			tmpDir := t.TempDir()
			jsonPath := filepath.Join(tmpDir, sample.file+".json")
			outPath := filepath.Join(tmpDir, sample.file)
			if err := service.ConvertCtToJson(inputPath, jsonPath); err != nil {
				t.Fatalf("ConvertCtToJson: %v", err)
			}
			if !IsKCESCtJSONFile(jsonPath) {
				t.Fatalf("converted JSON was not detected as KCES .ct JSON")
			}
			if err := service.ConvertJsonToCt(jsonPath, outPath); err != nil {
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

	for _, ext := range wantCatalog.ExtensionList {
		wantEnl, err := ct.DecodeExtensionNameListFromCt(wantTable, ext)
		if err != nil {
			t.Fatalf("decode original ExtensionNameList %q: %v", ext, err)
		}
		gotEnl, err := ct.DecodeExtensionNameListFromCt(gotTable, ext)
		if err != nil {
			t.Fatalf("decode round-trip ExtensionNameList %q: %v", ext, err)
		}
		if !reflect.DeepEqual(gotEnl, wantEnl) {
			t.Fatalf("ExtensionNameList %q changed after round-trip\ngot:  %+v\nwant: %+v", ext, gotEnl, wantEnl)
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

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
