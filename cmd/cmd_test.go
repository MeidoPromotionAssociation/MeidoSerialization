package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	KCESService "github.com/MeidoPromotionAssociation/MeidoSerialization/service/KCES"
	"github.com/spf13/cobra"
)

func executeCommand(root *cobra.Command, args ...string) (output string, err error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	// Reset global flags before each execution
	strictMode = false
	fileType = ""
	extractExt = ""
	extractFile = ""
	extractOutputFlag = ""

	// Also capture stdout as many parts of the code use fmt.Printf
	old := os.Stdout
	r, w, errPipe := os.Pipe()
	if errPipe != nil {
		return "", errPipe
	}
	os.Stdout = w

	outChan := make(chan string)
	go func() {
		var outBuf bytes.Buffer
		outBuf.ReadFrom(r)
		outChan <- outBuf.String()
	}()

	err = root.Execute()

	w.Close()
	stdoutStr := <-outChan
	os.Stdout = old

	return buf.String() + stdoutStr, err
}

func TestVersionCommand(t *testing.T) {
	output, err := executeCommand(RootCmd, "version")
	if err != nil {
		t.Fatalf("version command failed: %v", err)
	}
	if !strings.Contains(output, "MeidoSerialization") {
		t.Errorf("expected output to contain 'MeidoSerialization', got %q", output)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestDetermineCommand(t *testing.T) {
	// Test single file
	output, err := executeCommand(RootCmd, "determine", "../testdata/test.menu")
	if err != nil {
		t.Fatalf("determine command failed: %v", err)
	}
	if !strings.Contains(output, "test.menu") {
		t.Errorf("expected output to contain 'test.menu', got %q", output)
	}

	// Test directory
	output, err = executeCommand(RootCmd, "determine", "../testdata")
	if err != nil {
		t.Fatalf("determine command failed: %v", err)
	}
	if !strings.Contains(output, "Analyzing directory") {
		t.Errorf("expected output to contain 'Analyzing directory', got %q", output)
	}
}

func TestConvertCommands(t *testing.T) {
	tempDir := t.TempDir()

	// Copy a test file to temp dir
	testFile := "test.menu"
	inputPath := filepath.Join("../testdata", testFile)
	tempInputPath := filepath.Join(tempDir, testFile)

	data, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(tempInputPath, data, 0644)
	if err != nil {
		t.Fatal(err)
	}

	// 1. Test convert2json
	_, err = executeCommand(RootCmd, "convert2json", tempInputPath)
	if err != nil {
		t.Fatalf("convert2json failed: %v", err)
	}

	jsonPath := tempInputPath + ".json"
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		t.Errorf("expected JSON file %s to be created", jsonPath)
	}

	// 2. Test convert2mod
	// Delete original mod file first to ensure it's recreated
	os.Remove(tempInputPath)
	_, err = executeCommand(RootCmd, "convert2mod", jsonPath)
	if err != nil {
		t.Fatalf("convert2mod failed: %v", err)
	}
	if _, err := os.Stat(tempInputPath); os.IsNotExist(err) {
		t.Errorf("expected MOD file %s to be re-created", tempInputPath)
	}

	// 3. Test convert (auto-detect)
	os.Remove(jsonPath)
	_, err = executeCommand(RootCmd, "convert", tempInputPath)
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		t.Errorf("expected JSON file %s to be created by auto-convert", jsonPath)
	}
}

func TestTexImageCommands(t *testing.T) {
	tempDir := t.TempDir()
	testFile := "test.tex"
	inputPath := filepath.Join("../testdata", testFile)
	tempInputPath := filepath.Join(tempDir, testFile)

	data, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(tempInputPath, data, 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Test convert2image
	_, err = executeCommand(RootCmd, "convert2image", tempInputPath)
	pngPath := strings.TrimSuffix(tempInputPath, ".tex") + ".png"
	if err != nil {
		t.Logf("convert2image failed (expected if no ImageMagick): %v", err)
	} else if _, err := os.Stat(pngPath); os.IsNotExist(err) {
		t.Logf("PNG file %s not created, might be due to missing tools", pngPath)
	} else {
		// If PNG was created, test convert2tex
		os.Remove(tempInputPath)
		_, err = executeCommand(RootCmd, "convert2tex", pngPath)
		if err != nil {
			t.Errorf("convert2tex failed: %v", err)
		}
		if _, err := os.Stat(tempInputPath); os.IsNotExist(err) {
			t.Errorf("expected TEX file %s to be re-created", tempInputPath)
		}
	}
}

func TestDirectoryProcessing(t *testing.T) {
	tempDir := t.TempDir()

	// Copy multiple files to temp dir
	files := []string{"test.menu", "test.pmat"}
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join("../testdata", f))
		if err != nil {
			t.Fatal(err)
		}
		err = os.WriteFile(filepath.Join(tempDir, f), data, 0644)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Test convert directory to JSON
	output, err := executeCommand(RootCmd, "convert2json", tempDir)
	if err != nil {
		t.Fatalf("convert2json on directory failed: %v", err)
	}
	if !strings.Contains(output, "Processing directory") {
		t.Errorf("expected output to contain 'Processing directory', got %q", output)
	}

	for _, f := range files {
		jsonPath := filepath.Join(tempDir, f+".json")
		if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
			t.Errorf("expected JSON file %s to be created", jsonPath)
		}
	}
}

func TestKCESPartsConvertCommands(t *testing.T) {
	tempDir := t.TempDir()
	assets := &serializationKCES.PriorityMaterialAssets{
		FileName: "test.pmatassets",
		Assets: []serializationKCES.PriorityMaterial{
			{
				Version:     1000,
				ID:          12345,
				FileName:    "test.pmat",
				RenderQueue: 3000,
				TargetID:    67890,
			},
		},
	}

	encoded, err := serializationKCES.EncodePriorityMaterialAssets(assets)
	if err != nil {
		t.Fatalf("EncodePriorityMaterialAssets failed: %v", err)
	}

	inputPath := filepath.Join(tempDir, "test.pmatassets")
	if err := os.WriteFile(inputPath, encoded, 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := executeCommand(RootCmd, "convert2json", inputPath); err != nil {
		t.Fatalf("convert2json KCES parts failed: %v", err)
	}
	jsonPath := inputPath + ".json"
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		t.Fatalf("expected JSON file %s to be created", jsonPath)
	}

	if err := os.Remove(inputPath); err != nil {
		t.Fatal(err)
	}
	if _, err := executeCommand(RootCmd, "convert2mod", jsonPath); err != nil {
		t.Fatalf("convert2mod KCES parts failed: %v", err)
	}
	roundTrip, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("read converted KCES parts: %v", err)
	}
	decoded, err := serializationKCES.DecodePriorityMaterialAssets(roundTrip)
	if err != nil {
		t.Fatalf("DecodePriorityMaterialAssets failed: %v", err)
	}
	if decoded.FileName != assets.FileName || len(decoded.Assets) != 1 || decoded.Assets[0].FileName != "test.pmat" {
		t.Fatalf("unexpected decoded KCES parts: %+v", decoded)
	}

	if err := os.Remove(jsonPath); err != nil {
		t.Fatal(err)
	}
	if _, err := executeCommand(RootCmd, "convert", inputPath); err != nil {
		t.Fatalf("auto convert KCES parts failed: %v", err)
	}
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		t.Fatalf("expected JSON file %s to be created by auto convert", jsonPath)
	}
}

func TestKCESRawUnityBytesConvertCommands(t *testing.T) {
	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "cm3d2_megane002.tex.bytes")
	metaPath := inputPath + ".meta.json"

	data := mustReadFile(t, filepath.Join("../testdata", "kces_assets", "cm3d2_megane002.tex.bytes"))
	meta := mustReadFile(t, filepath.Join("../testdata", "kces_assets", "cm3d2_megane002.tex.bytes.meta.json"))
	if err := os.WriteFile(inputPath, data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metaPath, meta, 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := executeCommand(RootCmd, "convert2json", inputPath); err != nil {
		t.Fatalf("convert2json raw Unity .bytes failed: %v", err)
	}
	jsonPath := inputPath + ".json"
	if !KCESService.IsKCESRawUnityBytesJSONFile(jsonPath) {
		t.Fatalf("expected raw Unity JSON at %s", jsonPath)
	}

	jsonData := mustReadFile(t, jsonPath)
	var envelope KCESService.RawUnityObjectEnvelope
	if err := json.Unmarshal(jsonData, &envelope); err != nil {
		t.Fatalf("decode raw Unity JSON: %v", err)
	}
	if envelope.Format != KCESService.RawUnityObjectFormat || envelope.TypeName != "Texture2D" || envelope.Kind != "rawtexture2d" {
		t.Fatalf("unexpected raw Unity JSON envelope: %+v", envelope)
	}

	if err := os.Remove(inputPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(metaPath); err != nil {
		t.Fatal(err)
	}
	if _, err := executeCommand(RootCmd, "convert2mod", jsonPath); err != nil {
		t.Fatalf("convert2mod raw Unity JSON failed: %v", err)
	}
	roundTrip := mustReadFile(t, inputPath)
	if !bytes.Equal(roundTrip, data) {
		t.Fatalf("raw Unity command round-trip changed bytes")
	}
	if _, err := os.Stat(metaPath); err != nil {
		t.Fatalf("expected raw Unity sidecar to be restored: %v", err)
	}
}

func TestKCESCtConvertCommands(t *testing.T) {
	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "cm3d2_megane002.ct")
	data := mustReadFile(t, filepath.Join("../testdata", "aba", "cm3d2_megane002.ct"))
	if err := os.WriteFile(inputPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := executeCommand(RootCmd, "convert2json", inputPath); err != nil {
		t.Fatalf("convert2json .ct failed: %v", err)
	}
	jsonPath := inputPath + ".json"
	if !KCESService.IsKCESCtJSONFile(jsonPath) {
		t.Fatalf("expected KCES .ct JSON at %s", jsonPath)
	}

	jsonData := mustReadFile(t, jsonPath)
	var envelope KCESService.CtEnvelope
	if err := json.Unmarshal(jsonData, &envelope); err != nil {
		t.Fatalf("decode .ct JSON: %v", err)
	}
	if envelope.Format != KCESService.CtEnvelopeFormat || envelope.Catalog == nil || envelope.Catalog.Name != "cm3d2_megane002" {
		t.Fatalf("unexpected .ct JSON envelope: %+v", envelope)
	}
	if envelope.ExtensionNameLists[".menuassets"] == nil || envelope.ExtensionNameLists[".model"] == nil {
		t.Fatalf("missing expected ExtensionNameList entries: %+v", envelope.ExtensionNameLists)
	}

	if err := os.Remove(inputPath); err != nil {
		t.Fatal(err)
	}
	if _, err := executeCommand(RootCmd, "convert2mod", jsonPath); err != nil {
		t.Fatalf("convert2mod .ct JSON failed: %v", err)
	}
	catalog := readCatalogFromCtForCommandTest(t, inputPath)
	if catalog.Name != "cm3d2_megane002" || len(catalog.Items) == 0 {
		t.Fatalf("unexpected round-trip catalog: %+v", catalog)
	}
}

func TestKCESAutoConvertDirectoryIncludesCtAndRawUnityBytes(t *testing.T) {
	tempDir := t.TempDir()

	ctPath := filepath.Join(tempDir, "cm3d2_megane002.ct")
	ctData := mustReadFile(t, filepath.Join("../testdata", "aba", "cm3d2_megane002.ct"))
	if err := os.WriteFile(ctPath, ctData, 0644); err != nil {
		t.Fatal(err)
	}

	rawPath := filepath.Join(tempDir, "DepthLUT.monoscript.bytes")
	rawData := mustReadFile(t, filepath.Join("../testdata", "kces_assets", "DepthLUT.monoscript.bytes"))
	rawMeta := mustReadFile(t, filepath.Join("../testdata", "kces_assets", "DepthLUT.monoscript.bytes.meta.json"))
	if err := os.WriteFile(rawPath, rawData, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rawPath+".meta.json", rawMeta, 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := executeCommand(RootCmd, "convert", tempDir); err != nil {
		t.Fatalf("auto convert directory failed: %v", err)
	}
	if !KCESService.IsKCESCtJSONFile(ctPath + ".json") {
		t.Fatalf("expected directory auto convert to create .ct JSON")
	}
	if !KCESService.IsKCESRawUnityBytesJSONFile(rawPath + ".json") {
		t.Fatalf("expected directory auto convert to create raw Unity .bytes JSON")
	}
}

func TestKCESAbaCtPackListUnpackCommands(t *testing.T) {
	t.Run("list and unpack fixed samples", func(t *testing.T) {
		tempDir := t.TempDir()
		abaPath := filepath.Join(tempDir, "cm3d2_megane002.aba")
		ctPath := filepath.Join(tempDir, "cm3d2_megane002.ct")
		if err := os.WriteFile(abaPath, mustReadFile(t, filepath.Join("../testdata", "aba", "cm3d2_megane002.aba")), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(ctPath, mustReadFile(t, filepath.Join("../testdata", "aba", "cm3d2_megane002.ct")), 0644); err != nil {
			t.Fatal(err)
		}

		out, err := executeCommand(RootCmd, "listAba", abaPath)
		if err != nil {
			t.Fatalf("listAba failed: %v", err)
		}
		if !strings.Contains(out, "cm3d2_megane002.menuassets") || !strings.Contains(out, "Texture2D") {
			t.Fatalf("unexpected listAba output: %s", out)
		}
		out, err = executeCommand(RootCmd, "listCt", ctPath)
		if err != nil {
			t.Fatalf("listCt failed: %v", err)
		}
		if !strings.Contains(out, "catalog") || !strings.Contains(out, ".menuassets") {
			t.Fatalf("unexpected listCt output: %s", out)
		}

		abaOutDir := filepath.Join(tempDir, "aba_unpacked")
		if _, err := executeCommand(RootCmd, "unpackAba", abaPath, "-o", abaOutDir); err != nil {
			t.Fatalf("unpackAba failed: %v", err)
		}
		if _, err := os.Stat(filepath.Join(abaOutDir, "TextAsset", "cm3d2_megane002.menuassets")); err != nil {
			t.Fatalf("expected unpacked TextAsset: %v", err)
		}
		if _, err := os.Stat(filepath.Join(abaOutDir, "Texture2D", "cm3d2_megane002.tex.bytes.typetree.json")); err != nil {
			t.Fatalf("expected unpacked TypeTree sidecar: %v", err)
		}

		ctOutDir := filepath.Join(tempDir, "ct_unpacked")
		if _, err := executeCommand(RootCmd, "unpackCt", ctPath, "-o", ctOutDir); err != nil {
			t.Fatalf("unpackCt failed: %v", err)
		}
		if _, err := os.Stat(filepath.Join(ctOutDir, "catalog")); err != nil {
			t.Fatalf("expected unpacked catalog: %v", err)
		}
	})

	t.Run("pack commands", func(t *testing.T) {
		tempDir := t.TempDir()
		inputDir := filepath.Join(tempDir, "pack_input")
		if err := os.MkdirAll(inputDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(inputDir, "sample.menuassets"), []byte("menu"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(inputDir, "sample.model"), []byte("model"), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := executeCommand(RootCmd, "packAba", inputDir, "-o", "packed_cli"); err != nil {
			t.Fatalf("packAba failed: %v", err)
		}
		if _, err := os.Stat(filepath.Join(tempDir, "packed_cli.aba")); err != nil {
			t.Fatalf("expected packed .aba: %v", err)
		}
		if _, err := os.Stat(filepath.Join(tempDir, "packed_cli.ct")); err != nil {
			t.Fatalf("expected packed .ct: %v", err)
		}

		ctDir := filepath.Join(tempDir, "ct_input")
		if err := os.MkdirAll(ctDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ctDir, "catalog"), []byte("catalog"), 0644); err != nil {
			t.Fatal(err)
		}
		ctOut := filepath.Join(tempDir, "packed_cli_raw.ct")
		if _, err := executeCommand(RootCmd, "packCt", ctDir, "-o", ctOut); err != nil {
			t.Fatalf("packCt failed: %v", err)
		}
		if _, err := os.Stat(ctOut); err != nil {
			t.Fatalf("expected packed raw .ct: %v", err)
		}
	})
}

func TestKCESPayloadConvertCommands(t *testing.T) {
	tempDir := t.TempDir()
	encoded, err := serializationKCES.EncodeDynamicBoneStatusFile(&serializationKCES.DynamicBoneStatus{
		Version:    1000,
		Damping:    0.4,
		Elasticity: 0.2,
		Gravity:    serializationKCES.Vector3{Y: -0.05},
	})
	if err != nil {
		t.Fatalf("EncodeDynamicBoneStatusFile failed: %v", err)
	}

	inputPath := filepath.Join(tempDir, "sample.dbconf")
	if err := os.WriteFile(inputPath, encoded, 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := executeCommand(RootCmd, "convert2json", inputPath); err != nil {
		t.Fatalf("convert2json KCES payload failed: %v", err)
	}
	jsonPath := inputPath + ".json"
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		t.Fatalf("expected JSON file %s to be created", jsonPath)
	}

	if err := os.Remove(inputPath); err != nil {
		t.Fatal(err)
	}
	if _, err := executeCommand(RootCmd, "convert2mod", jsonPath); err != nil {
		t.Fatalf("convert2mod KCES payload failed: %v", err)
	}
	if _, err := serializationKCES.DecodeDynamicBoneStatusFile(mustReadFile(t, inputPath)); err != nil {
		t.Fatalf("converted .dbconf is invalid: %v", err)
	}

	if err := os.Remove(jsonPath); err != nil {
		t.Fatal(err)
	}
	if _, err := executeCommand(RootCmd, "convert", inputPath); err != nil {
		t.Fatalf("auto convert KCES payload failed: %v", err)
	}
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		t.Fatalf("expected JSON file %s to be created by auto convert", jsonPath)
	}
}

func readCatalogFromCtForCommandTest(t *testing.T, path string) *ct.AssetBundleCatalog {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	table, err := ct.ReadContentTable(f)
	if err != nil {
		t.Fatalf("ReadContentTable: %v", err)
	}
	catalog, err := ct.DecodeCatalogFromCt(table)
	if err != nil {
		t.Fatalf("DecodeCatalogFromCt: %v", err)
	}
	return catalog
}

func TestKCESMiscConvertCommands(t *testing.T) {
	t.Run("hitcheck", func(t *testing.T) {
		tempDir := t.TempDir()
		tail := int32(0)
		encoded, err := serializationKCES.EncodeHitCheck(&serializationKCES.HitCheck{
			Header: 1,
			Entries: []serializationKCES.HitCheckEntry{
				{
					Radius:     0.2,
					RadiusSqr:  0.04,
					ShapeName:  "Sphere",
					BoneName:   "Bip01 Head",
					Position:   serializationKCES.Vector3{Y: 0.1},
					TargetType: 0,
					Side:       1,
					Tail:       &tail,
				},
			},
		})
		if err != nil {
			t.Fatalf("EncodeHitCheck failed: %v", err)
		}

		inputPath := filepath.Join(tempDir, "sample.hitcheck")
		if err := os.WriteFile(inputPath, encoded, 0644); err != nil {
			t.Fatal(err)
		}

		if _, err := executeCommand(RootCmd, "convert2json", inputPath); err != nil {
			t.Fatalf("convert2json hitcheck failed: %v", err)
		}
		jsonPath := inputPath + ".json"
		if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
			t.Fatalf("expected JSON file %s to be created", jsonPath)
		}

		if err := os.Remove(inputPath); err != nil {
			t.Fatal(err)
		}
		if _, err := executeCommand(RootCmd, "convert2mod", jsonPath); err != nil {
			t.Fatalf("convert2mod hitcheck failed: %v", err)
		}
		if _, err := serializationKCES.DecodeHitCheck(mustReadFile(t, inputPath)); err != nil {
			t.Fatalf("converted hitcheck is invalid: %v", err)
		}
	})

	t.Run("undressdat", func(t *testing.T) {
		tempDir := t.TempDir()
		inputPath := filepath.Join(tempDir, "sample.undressdat")
		if err := os.WriteFile(inputPath, []byte(`{"editVer":13,"items":["a"]}`), 0644); err != nil {
			t.Fatal(err)
		}

		if _, err := executeCommand(RootCmd, "convert", inputPath); err != nil {
			t.Fatalf("auto convert undressdat failed: %v", err)
		}
		jsonPath := inputPath + ".json"
		if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
			t.Fatalf("expected JSON file %s to be created", jsonPath)
		}

		if err := os.Remove(inputPath); err != nil {
			t.Fatal(err)
		}
		if _, err := executeCommand(RootCmd, "convert", jsonPath); err != nil {
			t.Fatalf("auto convert undressdat JSON failed: %v", err)
		}
		if _, err := serializationKCES.DecodeKCESJSONText(mustReadFile(t, inputPath), ".undressdat"); err != nil {
			t.Fatalf("converted undressdat is invalid: %v", err)
		}
	})
}

func TestNeiCsvCommands(t *testing.T) {
	tempDir := t.TempDir()
	testFile := "test.nei"
	inputPath := filepath.Join("../testdata", testFile)
	tempInputPath := filepath.Join(tempDir, testFile)

	data, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(tempInputPath, data, 0644)
	if err != nil {
		t.Fatal(err)
	}

	// 1. Test convert2csv
	_, err = executeCommand(RootCmd, "convert2csv", tempInputPath)
	if err != nil {
		t.Fatalf("convert2csv failed: %v", err)
	}
	csvPath := strings.TrimSuffix(tempInputPath, ".nei") + ".csv"
	if _, err := os.Stat(csvPath); os.IsNotExist(err) {
		t.Errorf("expected CSV file %s to be created", csvPath)
	}

	// 2. Test convert2nei
	os.Remove(tempInputPath)
	_, err = executeCommand(RootCmd, "convert2nei", csvPath)
	if err != nil {
		t.Fatalf("convert2nei failed: %v", err)
	}
	if _, err := os.Stat(tempInputPath); os.IsNotExist(err) {
		t.Errorf("expected NEI file %s to be re-created", tempInputPath)
	}
}

func TestArcCommands(t *testing.T) {
	tempDir := t.TempDir()
	testFile := "test.arc"
	inputPath := filepath.Join("../testdata", testFile)
	tempInputPath := filepath.Join(tempDir, testFile)

	data, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(tempInputPath, data, 0644)
	if err != nil {
		t.Fatal(err)
	}

	// 1. Test unpackArc
	unpackDir := filepath.Join(tempDir, "unpacked")
	_, err = executeCommand(RootCmd, "unpackArc", tempInputPath, "-o", unpackDir)
	if err != nil {
		t.Fatalf("unpackArc failed: %v", err)
	}
	if _, err := os.Stat(unpackDir); os.IsNotExist(err) {
		t.Errorf("expected unpack directory %s to be created", unpackDir)
	}

	// 2. Test packArc
	repackPath := filepath.Join(tempDir, "repacked.arc")
	_, err = executeCommand(RootCmd, "packArc", unpackDir, "-o", repackPath)
	if err != nil {
		t.Fatalf("packArc failed: %v", err)
	}
	if _, err := os.Stat(repackPath); os.IsNotExist(err) {
		t.Errorf("expected repacked ARC file %s to be created", repackPath)
	}
}

func TestListArcCommand(t *testing.T) {
	tempDir := t.TempDir()
	testFile := "test.arc"
	inputPath := filepath.Join("../testdata", testFile)
	tempInputPath := filepath.Join(tempDir, testFile)

	data, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(tempInputPath, data, 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Test listArc
	output, err := executeCommand(RootCmd, "listArc", tempInputPath)
	if err != nil {
		t.Fatalf("listArc failed: %v", err)
	}

	// Should contain file listings
	if !strings.Contains(output, ".tex") {
		t.Errorf("expected output to contain '.tex' files, got %q", output)
	}

	// Should contain the total count
	if !strings.Contains(output, "Total:") {
		t.Errorf("expected output to contain 'Total:', got %q", output)
	}
	if !strings.Contains(output, "1137 files") {
		t.Errorf("expected output to contain '1137 files', got %q", output)
	}
}

func TestExtractArcCommand(t *testing.T) {
	tempDir := t.TempDir()
	testFile := "test.arc"
	inputPath := filepath.Join("../testdata", testFile)
	tempInputPath := filepath.Join(tempDir, testFile)

	data, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(tempInputPath, data, 0644)
	if err != nil {
		t.Fatal(err)
	}

	// 1. Test extractArc --ext (extract by extension)
	extOutDir := filepath.Join(tempDir, "ext_output")
	output, err := executeCommand(RootCmd, "extractArc", tempInputPath, "--ext", ".preset", "-o", extOutDir)
	if err != nil {
		t.Fatalf("extractArc --ext failed: %v", err)
	}
	if !strings.Contains(output, "Extracted") {
		t.Errorf("expected output to contain 'Extracted', got %q", output)
	}
	// test.arc contains 4 .preset files
	if !strings.Contains(output, "4 files") {
		t.Errorf("expected output to contain '4 files', got %q", output)
	}
	if _, err := os.Stat(extOutDir); os.IsNotExist(err) {
		t.Errorf("expected output directory %s to be created", extOutDir)
	}

	// 2. Test extractArc --ext without leading dot
	extOutDir2 := filepath.Join(tempDir, "ext_output2")
	output, err = executeCommand(RootCmd, "extractArc", tempInputPath, "--ext", "xml", "-o", extOutDir2)
	if err != nil {
		t.Fatalf("extractArc --ext (no dot) failed: %v", err)
	}
	// test.arc contains 5 .xml files
	if !strings.Contains(output, "5 files") {
		t.Errorf("expected output to contain '5 files', got %q", output)
	}

	// 3. Test extractArc --file (extract single file)
	fileOutDir := filepath.Join(tempDir, "file_output")
	targetFile := "system\\facilitycostume\\com3d2_facility_costume_bar_lounge.tex"
	_, err = executeCommand(RootCmd, "extractArc", tempInputPath, "--file", targetFile, "-o", fileOutDir)
	if err != nil {
		t.Fatalf("extractArc --file failed: %v", err)
	}
	extractedPath := filepath.Join(fileOutDir, targetFile)
	if _, err := os.Stat(extractedPath); os.IsNotExist(err) {
		t.Errorf("expected extracted file %s to exist", extractedPath)
	}

	// 4. Test error: neither --ext nor --file
	_, err = executeCommand(RootCmd, "extractArc", tempInputPath)
	if err == nil {
		t.Error("expected error when neither --ext nor --file is provided")
	}

	// 5. Test error: both --ext and --file
	_, err = executeCommand(RootCmd, "extractArc", tempInputPath, "--ext", ".tex", "--file", "some/file.tex")
	if err == nil {
		t.Error("expected error when both --ext and --file are provided")
	}
}

func TestExtractArcDirectoryCommand(t *testing.T) {
	tempDir := t.TempDir()
	testFile := "test.arc"
	inputPath := filepath.Join("../testdata", testFile)

	// Create a subdirectory with two copies of the arc
	arcDir := filepath.Join(tempDir, "arcs")
	os.MkdirAll(arcDir, 0755)

	data, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(arcDir, "a.arc"), data, 0644)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(arcDir, "b.arc"), data, 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Test extractArc on a directory
	output, err := executeCommand(RootCmd, "extractArc", arcDir, "--ext", "preset")
	if err != nil {
		t.Fatalf("extractArc directory failed: %v", err)
	}
	if !strings.Contains(output, "Extracted") {
		t.Errorf("expected output to contain 'Extracted', got %q", output)
	}

	// Both arc files should have been processed — check output dirs exist
	aOut := filepath.Join(arcDir, "a.arc_extracted")
	bOut := filepath.Join(arcDir, "b.arc_extracted")
	if _, err := os.Stat(aOut); os.IsNotExist(err) {
		t.Errorf("expected output directory %s to be created", aOut)
	}
	if _, err := os.Stat(bOut); os.IsNotExist(err) {
		t.Errorf("expected output directory %s to be created", bOut)
	}
}
