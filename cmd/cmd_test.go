package cmd

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/internal/kcesfixtures"
	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/aba"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/ct"
	KCESService "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/service/KCES"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/tools"
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
	// pflag keeps the previous value of a flag that is absent from the arguments, so the
	// NEI format selections have to be restored to their defaults to keep runs independent
	neiFormatFlag = "auto"
	convertNeiFormatFlag = "auto"
	outputFormat = "png"

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

func TestDetermineCommandPrefersCOM3D2AndFallsBackToKCES(t *testing.T) {
	modelPath := kcesfixtures.TextAssetPath(t, "cm3d2_megane002.aba", "cm3d2_megane002.model")
	menuAssetsPath := kcesfixtures.TextAssetPath(t, "cm3d2_eyes.aba", "cm3d2_eyes.menuassets")
	payloadPath := kcesfixtures.TextAssetPath(t, "partsmeta.aba", "default_hairf.dbconf")
	hitCheckPath := kcesfixtures.TextAssetPath(t, "system.aba", "IK.hitcheck")
	nativeJSONPath := kcesfixtures.TextAssetPath(t, "parts_bv001.aba", "crc2_Underwear038_pants.undressdat")

	tests := []struct {
		name   string
		path   string
		typeID string
		format string
		game   string
	}{
		{name: "content_table", path: "../testdata/KCES/cm3d2_eyes.ct", typeID: "ct", format: "binary", game: "KCES"},
		{name: "unityfs", path: "../testdata/KCES/cm3d2_eyes.aba", typeID: "aba", format: "binary", game: "KCES"},
		{name: "model", path: modelPath, typeID: "model", format: "binary", game: "KCES"},
		{name: "menuassets", path: menuAssetsPath, typeID: "menuassets", format: "binary", game: "KCES"},
		{name: "payload", path: payloadPath, typeID: "dbconf", format: "binary", game: "KCES"},
		{name: "hitcheck", path: hitCheckPath, typeID: "hitcheck", format: "binary", game: "KCES"},
		{name: "native_json", path: nativeJSONPath, typeID: "undressdat", format: "json", game: "KCES"},
		{name: "legacy_model", path: "../testdata/test.model", typeID: "model", format: "binary", game: "COM3D2"},
		{name: "legacy_preset", path: "../testdata/test.preset", typeID: "preset", format: "binary", game: "COM3D2"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output, err := executeCommand(RootCmd, "determine", "--strict", test.path)
			if err != nil {
				t.Fatalf("determine --strict %q: %v\n%s", test.path, err, output)
			}
			for _, want := range []string{
				"Type: " + test.typeID,
				"Format: " + test.format,
				"Game: " + test.game,
			} {
				if !strings.Contains(output, want) {
					t.Fatalf("output for %q does not contain %q:\n%s", test.path, want, output)
				}
			}
		})
	}
}

func TestStrictFileTypeFilterIncludesKCESModelAndNativeJSON(t *testing.T) {
	modelPath := kcesfixtures.TextAssetPath(t, "cm3d2_megane002.aba", "cm3d2_megane002.model")
	menuAssetsPath := kcesfixtures.TextAssetPath(t, "cm3d2_eyes.aba", "cm3d2_eyes.menuassets")
	nativeJSONPath := kcesfixtures.TextAssetPath(t, "parts_bv001.aba", "crc2_Underwear038_pants.undressdat")

	oldStrict, oldType := strictMode, fileType
	t.Cleanup(func() {
		strictMode = oldStrict
		fileType = oldType
	})
	strictMode = true

	fileType = "model"
	if !fileTypeFilter(modelPath) {
		t.Fatal("strict model filter rejected a decoded KCES model")
	}
	if fileTypeFilter(menuAssetsPath) {
		t.Fatal("strict model filter accepted menuassets")
	}

	// A native .undressdat is a Unity JsonUtility document on the wire, but it is
	// not an editing .json envelope. The non-.json selector must still include it.
	fileType = "undressdat"
	if !fileTypeFilter(nativeJSONPath) {
		t.Fatal("strict undressdat filter rejected native JSON-text payload")
	}
	fileType = "undressdat.json"
	if fileTypeFilter(nativeJSONPath) {
		t.Fatal("strict undressdat.json filter accepted the native non-envelope path")
	}
}

func TestConcurrentDeterminePrintsAtomicFileBlocks(t *testing.T) {
	dir := t.TempDir()
	samples := []struct {
		aba  string
		name string
	}{
		{"cm3d2_megane002.aba", "cm3d2_megane002.model"},
		{"parts_bcc2_gp003.aba", "crc_acchead003.model"},
		{"parts_personal002.aba", "hair_aho007.model"},
	}
	for _, sample := range samples {
		path := kcesfixtures.TextAssetPath(t, sample.aba, sample.name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, sample.name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}

	output, err := executeCommand(RootCmd, "determine", "--strict", "--type", "model", dir)
	if err != nil {
		t.Fatalf("concurrent determine: %v\n%s", err, output)
	}
	sections := strings.Split(output, "File: ")
	if len(sections)-1 != 3 {
		t.Fatalf("got %d file blocks, want 3:\n%s", len(sections)-1, output)
	}
	for i, section := range sections[1:] {
		for _, field := range []string{"\n  Type: ", "\n  Format: ", "\n  Game: ", "\n  Signature: ", "\n  Version: ", "\n  Size: "} {
			if strings.Count(section, field) != 1 {
				t.Fatalf("file block %d has %d occurrences of %q; output was interleaved:\n%s", i, strings.Count(section, field), field, output)
			}
		}
	}
}

func TestKCESPathsCommandRoutes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "paths.dat")
	paths := serializationKCES.NewKCESPathsFile()
	paths.Paths = []string{"system", "parts"}
	data, err := serializationKCES.EncodeKCESPaths(paths)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	output, err := executeCommand(RootCmd, "determine", "--strict", path)
	if err != nil {
		t.Fatalf("determine paths.dat: %v\n%s", err, output)
	}
	for _, want := range []string{"Type: paths", "Game: KCES", "Signature: CM3D2_PATHS"} {
		if !strings.Contains(output, want) {
			t.Fatalf("paths.dat determine output lacks %q:\n%s", want, output)
		}
	}

	if output, err = executeCommand(RootCmd, "convert2json", path); err != nil {
		t.Fatalf("convert2json paths.dat: %v\n%s", err, output)
	}
	jsonPath := path + ".json"
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if output, err = executeCommand(RootCmd, "convert2mod", jsonPath); err != nil {
		t.Fatalf("convert2mod paths.dat JSON: %v\n%s", err, output)
	}
	back, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := serializationKCES.DecodeKCESPaths(back)
	if err != nil || !reflect.DeepEqual(decoded.Paths, []string{"system", "parts"}) {
		t.Fatalf("paths.dat command round trip: value=%+v err=%v", decoded, err)
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
	assetsFileName := "test.pmatassets"
	materialFileName := "test.pmat"
	assets := &serializationKCES.PriorityMaterialAssets{
		FileName: &assetsFileName,
		Assets: []*serializationKCES.PriorityMaterial{
			{
				Version:     1000,
				ID:          12345,
				FileName:    &materialFileName,
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
	if decoded.FileName == nil || assets.FileName == nil || *decoded.FileName != *assets.FileName || len(decoded.Assets) != 1 || decoded.Assets[0] == nil || decoded.Assets[0].FileName == nil || *decoded.Assets[0].FileName != "test.pmat" {
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

	rawPath, rawMetaPath := kcesfixtures.RawObjectPath(t, "cm3d2_megane002.aba", "cm3d2_megane002.tex", aba.ClassIDTexture2D, "cm3d2_megane002.tex.bytes")
	data := mustReadFile(t, rawPath)
	meta := mustReadFile(t, rawMetaPath)
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
	data := mustReadFile(t, filepath.Join("../testdata", "KCES", "cm3d2_megane002.ct"))
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
	if envelope.Format != KCESService.CtEnvelopeFormat || envelope.Catalog.Name == nil || *envelope.Catalog.Name != "cm3d2_megane002" {
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
	if catalog.Name == nil || *catalog.Name != "cm3d2_megane002" || len(catalog.Items) == 0 {
		t.Fatalf("unexpected round-trip catalog: %+v", catalog)
	}
}

func TestKCESAutoConvertDirectoryIncludesCtAndRawUnityBytes(t *testing.T) {
	tempDir := t.TempDir()

	ctPath := filepath.Join(tempDir, "cm3d2_megane002.ct")
	ctData := mustReadFile(t, filepath.Join("../testdata", "KCES", "cm3d2_megane002.ct"))
	if err := os.WriteFile(ctPath, ctData, 0644); err != nil {
		t.Fatal(err)
	}

	rawPath := filepath.Join(tempDir, "DepthLUT.monoscript.bytes")
	rawSourcePath, rawSourceMetaPath := kcesfixtures.RawObjectPath(t, "system.aba", "DepthLUT", aba.ClassIDMonoScript, "DepthLUT.monoscript.bytes")
	rawData := mustReadFile(t, rawSourcePath)
	rawMeta := mustReadFile(t, rawSourceMetaPath)
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
	t.Run("list and unpack Unity 2022.3 samples", func(t *testing.T) {
		tempDir := t.TempDir()
		abaPath := filepath.Join(tempDir, "system.aba")
		ctPath := filepath.Join(tempDir, "system.ct")
		if err := os.WriteFile(abaPath, mustReadFile(t, filepath.Join("../testdata", "KCES", "system.aba")), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(ctPath, mustReadFile(t, filepath.Join("../testdata", "KCES", "system.ct")), 0644); err != nil {
			t.Fatal(err)
		}

		out, err := executeCommand(RootCmd, "listAba", abaPath)
		if err != nil {
			t.Fatalf("listAba failed: %v", err)
		}
		if !strings.Contains(out, "manIKCollider.ikcol") || !strings.Contains(out, "TextAsset") {
			t.Fatalf("unexpected listAba output: %s", out)
		}
		out, err = executeCommand(RootCmd, "listCt", ctPath)
		if err != nil {
			t.Fatalf("listCt failed: %v", err)
		}
		if !strings.Contains(out, "catalog") || !strings.Contains(out, ".hitcheck") {
			t.Fatalf("unexpected listCt output: %s", out)
		}

		abaOutDir := filepath.Join(tempDir, "aba_unpacked")
		if _, err := executeCommand(RootCmd, "unpackAba", abaPath, "-o", abaOutDir); err != nil {
			t.Fatalf("unpackAba failed: %v", err)
		}
		if _, err := os.Stat(filepath.Join(abaOutDir, "TextAsset", "manIKCollider.ikcol")); err != nil {
			t.Fatalf("expected unpacked TextAsset: %v", err)
		}
		if _, err := os.Stat(filepath.Join(abaOutDir, "MonoScript", "DepthLUT.bytes")); err != nil {
			t.Fatalf("expected unpacked MonoScript: %v", err)
		}
		if err := filepath.Walk(abaOutDir, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if info.IsDir() {
				return nil
			}
			lower := strings.ToLower(path)
			for _, suffix := range []string{".meta.json", ".typetree.json", ".ress", ".resource", ".resources", ".png"} {
				if strings.HasSuffix(lower, suffix) {
					t.Fatalf("unpackAba emitted forbidden artifact %q", path)
				}
			}
			return nil
		}); err != nil {
			t.Fatalf("walk unpacked ABA directory: %v", err)
		}

		genCtPath := filepath.Join(tempDir, "generated.ct")
		if _, err := executeCommand(RootCmd, "genCt", abaPath, "-o", genCtPath); err != nil {
			t.Fatalf("genCt failed: %v", err)
		}
		genOut, err := executeCommand(RootCmd, "listCt", genCtPath)
		if err != nil {
			t.Fatalf("listCt generated ct failed: %v", err)
		}
		if !strings.Contains(genOut, "catalog") || !strings.Contains(genOut, ".hitcheck") {
			t.Fatalf("unexpected generated ct contents: %s", genOut)
		}
	})

	t.Run("unpack UnityFS 7 sample", func(t *testing.T) {
		legacyPath := filepath.Join("../testdata", "KCES", "cm3d2_megane002.aba")
		outDir := filepath.Join(t.TempDir(), "unityfs7")
		if _, err := executeCommand(RootCmd, "unpackAba", legacyPath, "-o", outDir); err != nil {
			t.Fatalf("unpackAba UnityFS 7 failed: %v", err)
		}
		entries, err := os.ReadDir(outDir)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) == 0 {
			t.Fatal("unpackAba UnityFS 7 produced an empty directory")
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

		genCtPath := filepath.Join(tempDir, "packed_cli_gen.ct")
		if _, err := executeCommand(RootCmd, "genCt", filepath.Join(tempDir, "packed_cli.aba"), "-o", genCtPath); err != nil {
			t.Fatalf("genCt failed: %v", err)
		}
		packedList, err := executeCommand(RootCmd, "listCt", filepath.Join(tempDir, "packed_cli.ct"))
		if err != nil {
			t.Fatalf("listCt packed ct failed: %v", err)
		}
		generatedList, err := executeCommand(RootCmd, "listCt", genCtPath)
		if err != nil {
			t.Fatalf("listCt generated ct failed: %v", err)
		}
		if packedList != generatedList {
			t.Fatalf("genCt virtual files diverge from packAba output:\npacked: %s\ngenerated: %s", packedList, generatedList)
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
		hitCheck := serializationKCES.NewHitCheck()
		hitCheck.Entries = []serializationKCES.HitCheckEntry{
			{
				Type:      0,
				Radius:    0.2,
				RadiusSqr: 0.04,
				Name:      "Sphere",
				Parent:    "Bip01 Head",
				Position:  serializationKCES.Vector3{Y: 0.1},
				SKRT:      0,
				RL:        1,
			},
		}
		encoded, err := serializationKCES.EncodeHitCheck(hitCheck)
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
		if err := os.WriteFile(inputPath, []byte(`{"format":"1.2.2","editVer":13,"dataGroup":[{"label":"Group_0000","layer":0,"indices":[7,11]}]}`), 0644); err != nil {
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
		if _, err := serializationKCES.DecodeKCESUndressData(mustReadFile(t, inputPath)); err != nil {
			t.Fatalf("converted undressdat is invalid: %v", err)
		}
	})

	t.Run("nson", func(t *testing.T) {
		tempDir := t.TempDir()
		inputPath := filepath.Join(tempDir, "dance_enabled_list.nson")
		if err := os.WriteFile(inputPath, []byte(`{"version":1000,"_ids":[1,2,3]}`), 0644); err != nil {
			t.Fatal(err)
		}

		if _, err := executeCommand(RootCmd, "convert", inputPath); err != nil {
			t.Fatalf("auto convert nson failed: %v", err)
		}
		jsonPath := inputPath + ".json"
		if _, err := os.Stat(jsonPath); err != nil {
			t.Fatalf("expected JSON file %s: %v", jsonPath, err)
		}

		if err := os.Remove(inputPath); err != nil {
			t.Fatal(err)
		}
		if _, err := executeCommand(RootCmd, "convert", jsonPath); err != nil {
			t.Fatalf("auto convert nson JSON failed: %v", err)
		}
		value, err := serializationKCES.DecodeKCESJSONText(mustReadFile(t, inputPath), ".nson")
		if err != nil {
			t.Fatalf("converted nson is invalid: %v", err)
		}
		if string(value) != `{"version":1000,"_ids":[1,2,3]}` {
			t.Fatalf("unexpected nson document: %s", value)
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

// assertNeiCellEncoding 读取 NEI 并断言其单元格编码与首个单元格内容
// assertNeiCellEncoding reads a NEI and asserts its cell encoding and first cell content
func assertNeiCellEncoding(t *testing.T, path string, encoding serializationKCES.NeiTextEncoding, wantCell string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	table, err := serializationKCES.ReadNei(bytes.NewReader(data), nil)
	if err != nil {
		t.Fatalf("decode NEI %s: %v", path, err)
	}
	if table.TextEncoding != encoding {
		t.Errorf("%s cell encoding = %q, want %q", path, table.TextEncoding, encoding)
	}
	if len(table.Data) == 0 || len(table.Data[0]) == 0 {
		t.Fatalf("%s decoded to an empty table", path)
	}
	if table.Data[0][0] != wantCell {
		t.Errorf("%s cell = %q, want %q", path, table.Data[0][0], wantCell)
	}
}

// TestNeiFormatAutoReuse 固定 --format 的自动沿用与显式覆盖
// TestNeiFormatAutoReuse pins the automatic reuse and the explicit override of --format
func TestNeiFormatAutoReuse(t *testing.T) {
	tempDir := t.TempDir()
	neiPath := filepath.Join(tempDir, "kces.nei")
	csvPath := filepath.Join(tempDir, "kces.csv")

	// KCES 的单元格是 UTF-8，用含日文的表把两种编码区分开
	table := serializationKCES.NewNei(1, 1, [][]string{{"日本語テスト"}})
	var encoded bytes.Buffer
	if err := table.Dump(&encoded); err != nil {
		t.Fatalf("encode KCES NEI: %v", err)
	}
	if err := os.WriteFile(neiPath, encoded.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := executeCommand(RootCmd, "convert2csv", neiPath); err != nil {
		t.Fatalf("convert2csv failed: %v", err)
	}

	// CSV 不携带来源信息，自动模式必须沿用同名 .nei 的格式
	if _, err := executeCommand(RootCmd, "convert2nei", csvPath); err != nil {
		t.Fatalf("convert2nei failed: %v", err)
	}
	assertNeiCellEncoding(t, neiPath, serializationKCES.NeiTextEncodingUTF8, "日本語テスト")

	// 显式指定 com3d2 必须覆盖自动推断
	if _, err := executeCommand(RootCmd, "convert2nei", csvPath, "--format", "com3d2"); err != nil {
		t.Fatalf("convert2nei --format com3d2 failed: %v", err)
	}
	assertNeiCellEncoding(t, neiPath, serializationKCES.NeiTextEncodingShiftJIS, "日本語テスト")

	// 显式指定 kces 同样覆盖，并写回 UTF-8；-f 是 --format 的简写
	if _, err := executeCommand(RootCmd, "convert2nei", csvPath, "-f", "kces"); err != nil {
		t.Fatalf("convert2nei -f kces failed: %v", err)
	}
	assertNeiCellEncoding(t, neiPath, serializationKCES.NeiTextEncodingUTF8, "日本語テスト")

	// 目标不存在时回落到 COM3D2 的 Shift-JIS
	if err := os.Remove(neiPath); err != nil {
		t.Fatal(err)
	}
	if _, err := executeCommand(RootCmd, "convert2nei", csvPath); err != nil {
		t.Fatalf("convert2nei without an existing .nei failed: %v", err)
	}
	assertNeiCellEncoding(t, neiPath, serializationKCES.NeiTextEncodingShiftJIS, "日本語テスト")

	// 旧取值不再是合法输入，错误信息必须指出新取值
	_, err := executeCommand(RootCmd, "convert2nei", csvPath, "--format", "utf-8")
	if err == nil {
		t.Fatalf("expected the old encoding vocabulary to be rejected")
	}
	if !strings.Contains(err.Error(), "auto, com3d2 or kces") {
		t.Errorf("unexpected error for an unknown format: %v", err)
	}
}

// writeCmdTestPNG 写出一张可直接解码为 RGBA32 的测试 PNG，供原生 Texture2D 相关用例使用
// writeCmdTestPNG writes a test PNG that decodes directly to RGBA32 for the native Texture2D cases
func writeCmdTestPNG(t *testing.T, path string, width int32, height int32) {
	t.Helper()

	img := image.NewNRGBA(image.Rect(0, 0, int(width), int(height)))
	for y := int32(0); y < height; y++ {
		for x := int32(0); x < width; x++ {
			img.SetNRGBA(int(x), int(y), color.NRGBA{R: uint8(10 + x*40), G: uint8(20 + y*40), B: uint8(30 + x + y), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatalf("write PNG: %v", err)
	}
}

func TestConvertDirectoryCoversArcAndStandaloneNativeTexture(t *testing.T) {
	tempDir := t.TempDir()

	arcPath := filepath.Join(tempDir, "test.arc")
	if err := os.WriteFile(arcPath, mustReadFile(t, filepath.Join("../testdata", "csv_CanKnowTwoClose.arc")), 0644); err != nil {
		t.Fatal(err)
	}

	// 不在 Texture2D 目录下的原生 Texture2D 只能靠内容识别，目录遍历必须把它算进可转换集合
	sourceDir := t.TempDir()
	sourcePNG := filepath.Join(sourceDir, "standalone.png")
	writeCmdTestPNG(t, sourcePNG, 3, 2)
	if _, err := executeCommand(RootCmd, "convert2texture2d", sourcePNG); err != nil {
		t.Fatalf("convert2texture2d failed: %v", err)
	}
	standalonePath := filepath.Join(tempDir, "standalone.texture2d")
	if err := os.WriteFile(standalonePath, mustReadFile(t, filepath.Join(sourceDir, "standalone.tex")), 0644); err != nil {
		t.Fatal(err)
	}
	if !KCESService.IsKCESNativeTexture2DFile(standalonePath) {
		t.Fatalf("expected %s to be a native Texture2D", standalonePath)
	}

	if _, err := executeCommand(RootCmd, "convert", tempDir); err != nil {
		t.Fatalf("convert on directory failed: %v", err)
	}

	if _, err := os.Stat(arcPath + "_unpacked"); err != nil {
		t.Errorf("expected the directory run to unpack %s: %v", arcPath, err)
	}
	if _, err := os.Stat(filepath.Join(tempDir, "standalone.png")); err != nil {
		t.Errorf("expected the directory run to convert the standalone Texture2D to PNG: %v", err)
	}
}

func TestConvertImageReusesSameNamedNativeTexture2D(t *testing.T) {
	tempDir := t.TempDir()
	pngPath := filepath.Join(tempDir, "body.png")
	texPath := filepath.Join(tempDir, "body.tex")
	writeCmdTestPNG(t, pngPath, 3, 2)

	// 先用 convert2texture2d 造出一个真正的 KCES 原生 Texture2D 作为同名文件
	if _, err := executeCommand(RootCmd, "convert2texture2d", pngPath); err != nil {
		t.Fatalf("convert2texture2d failed: %v", err)
	}
	if !KCESService.IsKCESNativeTexture2DFile(texPath) {
		t.Fatalf("expected %s to be a native Texture2D", texPath)
	}

	if _, err := executeCommand(RootCmd, "convert", pngPath); err != nil {
		t.Fatalf("convert on the image failed: %v", err)
	}

	// .tex 同时对应 COM3D2 TEX 与 KCES 原生 Texture2D，回写必须沿用同名文件原本的格式
	if !KCESService.IsKCESNativeTexture2DFile(texPath) {
		t.Fatalf("expected the image to be written back as a native Texture2D, matching its sibling")
	}
}

func TestConvertDirectoryReportsFailures(t *testing.T) {
	tempDir := t.TempDir()

	// 后缀是 .nei 但内容不是加密表格，转换必然失败
	brokenPath := filepath.Join(tempDir, "broken.nei")
	if err := os.WriteFile(brokenPath, []byte("not an encrypted nei table"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := executeCommand(RootCmd, "convert", tempDir)
	if err == nil {
		t.Fatalf("expected convert on a directory holding a broken .nei to fail")
	}
	if !strings.Contains(err.Error(), "failed to process 1 file") {
		t.Errorf("unexpected error: %v", err)
	}
}

// decodeCmdTestImage 解码图像并返回逐像素的 RGBA 字节，用于按内容比较两张图
// decodeCmdTestImage decodes an image and returns per-pixel RGBA bytes so two images can be compared by content
func decodeCmdTestImage(t *testing.T, path string) []byte {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open image %s: %v", path, err)
	}
	defer file.Close()

	src, _, err := image.Decode(file)
	if err != nil {
		t.Fatalf("decode image %s: %v", path, err)
	}
	bounds := src.Bounds()
	pixels := make([]byte, 0, bounds.Dx()*bounds.Dy()*4)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := src.At(x, y).RGBA()
			pixels = append(pixels, byte(r>>8), byte(g>>8), byte(b>>8), byte(a>>8))
		}
	}
	return pixels
}

// renderCmdTestTexToPNG 把 TEX 复制到独立目录再导出为 PNG，返回 PNG 路径
// renderCmdTestTexToPNG copies a TEX into a scratch directory, exports it as PNG, and returns the PNG path
func renderCmdTestTexToPNG(t *testing.T, texPath string) string {
	t.Helper()

	scratch := t.TempDir()
	scratchTex := filepath.Join(scratch, filepath.Base(texPath))
	if err := os.WriteFile(scratchTex, mustReadFile(t, texPath), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := executeCommand(RootCmd, "convert2image", scratchTex); err != nil {
		t.Fatalf("convert2image on %s failed: %v", scratchTex, err)
	}
	return trimLastExtension(scratchTex) + ".png"
}

// TestConvertDirectoryConvergesOnImageTexPair 固定同名 PNG 与原生 Texture2D 互相覆盖的预期结果
// 同名文件互相覆盖是双向自动转换的既定行为，关键要求是必须收敛成一致的一对：格式沿用 .tex，内容以 .png 为准
// TestConvertDirectoryConvergesOnImageTexPair pins the expected outcome of a same-named PNG and native Texture2D overwriting each other
// Overwriting between same-named files is established behaviour for a bidirectional automatic command, and the requirement is that the pair converges consistently: the format follows the .tex and the content follows the .png
func TestConvertDirectoryConvergesOnImageTexPair(t *testing.T) {
	tempDir := t.TempDir()
	pngPath := filepath.Join(tempDir, "body.png")
	texPath := filepath.Join(tempDir, "body.tex")
	writeCmdTestPNG(t, pngPath, 3, 2)

	// 先造出同名原生 Texture2D，再把 PNG 换成另一个尺寸的版本，最终结果即可反映收敛方向
	if _, err := executeCommand(RootCmd, "convert2texture2d", pngPath); err != nil {
		t.Fatalf("convert2texture2d failed: %v", err)
	}
	if !KCESService.IsKCESNativeTexture2DFile(texPath) {
		t.Fatalf("expected %s to be a native Texture2D", texPath)
	}
	writeCmdTestPNG(t, pngPath, 4, 2)

	if _, err := executeCommand(RootCmd, "convert", tempDir); err != nil {
		t.Fatalf("convert on directory failed: %v", err)
	}

	if !KCESService.IsKCESNativeTexture2DFile(texPath) {
		t.Fatalf("expected the pair to converge on a native Texture2D")
	}
	if got := decodeCmdTestImage(t, pngPath); len(got) != 4*2*4 {
		t.Fatalf("expected the .png to come from the 4x2 image, got %d pixel bytes", len(got))
	}
	if !bytes.Equal(decodeCmdTestImage(t, renderCmdTestTexToPNG(t, texPath)), decodeCmdTestImage(t, pngPath)) {
		t.Fatalf("expected the .tex to hold the same pixels as the .png")
	}
}

// TestConvertDirectoryConvergesOnCOM3D2TexPair 固定同名 PNG 与 COM3D2 TEX 互相覆盖的预期结果
// TestConvertDirectoryConvergesOnCOM3D2TexPair pins the expected outcome of a same-named PNG and COM3D2 TEX overwriting each other
func TestConvertDirectoryConvergesOnCOM3D2TexPair(t *testing.T) {
	if err := tools.CheckMagick(); err != nil {
		t.Skipf("ImageMagick is required to write a COM3D2 TEX: %v", err)
	}

	tempDir := t.TempDir()
	pngPath := filepath.Join(tempDir, "body.png")
	texPath := filepath.Join(tempDir, "body.tex")
	writeCmdTestPNG(t, pngPath, 3, 2)

	if _, err := executeCommand(RootCmd, "convert2tex", pngPath); err != nil {
		t.Fatalf("convert2tex failed: %v", err)
	}
	if KCESService.IsKCESNativeTexture2DFile(texPath) {
		t.Fatalf("expected %s to be a COM3D2 TEX", texPath)
	}
	writeCmdTestPNG(t, pngPath, 4, 2)

	if _, err := executeCommand(RootCmd, "convert", tempDir); err != nil {
		t.Fatalf("convert on directory failed: %v", err)
	}

	// COM3D2 TEX 不是原生 Texture2D，回写必须仍然产出 COM3D2 TEX
	if KCESService.IsKCESNativeTexture2DFile(texPath) {
		t.Fatalf("expected the pair to stay on a COM3D2 TEX")
	}
	if !bytes.Equal(decodeCmdTestImage(t, renderCmdTestTexToPNG(t, texPath)), decodeCmdTestImage(t, pngPath)) {
		t.Fatalf("expected the .tex to hold the same pixels as the .png")
	}
}

// TestConvertImageCoversFormatOutsideSuffixTable 固定表外格式的两种处理路径
// 单文件路径由 ImageMagick 兜底；目录模式按后缀预筛会跳过，需要 --type image 让 ImageMagick 最终裁定
// TestConvertImageCoversFormatOutsideSuffixTable pins both ways a format outside the table is handled
// The single-file path falls back to ImageMagick, while directory mode skips it through its suffix pre-filter and needs --type image to let ImageMagick have the final say
func TestConvertImageCoversFormatOutsideSuffixTable(t *testing.T) {
	if err := tools.CheckMagick(); err != nil {
		t.Skipf("ImageMagick is required: %v", err)
	}

	// 后缀不在预筛表内，但内容是真正的 PNG，只有 ImageMagick 靠内容才能识别
	singleDir := t.TempDir()
	singlePath := filepath.Join(singleDir, "odd.weird")
	writeCmdTestPNG(t, singlePath, 3, 2)
	if _, err := executeCommand(RootCmd, "convert", singlePath); err != nil {
		t.Fatalf("convert on a format outside the suffix table failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(singleDir, "odd.tex")); err != nil {
		t.Fatalf("expected the single-file fallback to produce a TEX: %v", err)
	}

	dir := t.TempDir()
	dirPath := filepath.Join(dir, "odd.weird")
	writeCmdTestPNG(t, dirPath, 3, 2)
	if _, err := executeCommand(RootCmd, "convert", dir); err != nil {
		t.Fatalf("convert on the directory failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "odd.tex")); !os.IsNotExist(err) {
		t.Fatalf("expected the plain directory run to pre-filter a format outside the suffix table, got %v", err)
	}
	if _, err := executeCommand(RootCmd, "convert", "--type", "image", dir); err != nil {
		t.Fatalf("convert --type image on the directory failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "odd.tex")); err != nil {
		t.Fatalf("expected --type image to cover a format outside the suffix table: %v", err)
	}
}

// TestConvertDirectoryCoversVectorImage 固定目录预筛表覆盖矢量图
// TestConvertDirectoryCoversVectorImage pins that the directory pre-filter table covers vector images
func TestConvertDirectoryCoversVectorImage(t *testing.T) {
	if err := tools.CheckMagick(); err != nil {
		t.Skipf("ImageMagick is required to read SVG: %v", err)
	}

	tempDir := t.TempDir()
	svgPath := filepath.Join(tempDir, "a.svg")
	const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="4" height="4"><rect width="4" height="4" fill="red"/></svg>`
	if err := os.WriteFile(svgPath, []byte(svg), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := executeCommand(RootCmd, "convert", tempDir); err != nil {
		t.Fatalf("convert on directory failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tempDir, "a.tex")); err != nil {
		t.Fatalf("expected the directory run to convert the vector image: %v", err)
	}
}

// TestConvertDirectoryConvergesOnTripleBaseName 固定三个同基名文件也满足收敛契约
// 两个同基名原生 Texture2D 必须都被写成同一份内容，否则后遍历到的那个会把 PNG 回退成旧内容
// TestConvertDirectoryConvergesOnTripleBaseName pins that three files sharing a base name also satisfy the convergence contract
// Both same-named native Texture2D files have to receive the same content, otherwise whichever is walked later rolls the PNG back to stale content
// writeSameNamedNativeTexture2Ds 用给定尺寸的 PNG 造出原生 Texture2D，并复制成同基名的 .tex 与 .texture2d
// writeSameNamedNativeTexture2Ds builds a native Texture2D from a PNG of the given size and copies it to a same-named .tex and .texture2d
func writeSameNamedNativeTexture2Ds(t *testing.T, dir string, width int32, height int32) (string, string) {
	t.Helper()

	sourceDir := t.TempDir()
	sourcePNG := filepath.Join(sourceDir, "body.png")
	writeCmdTestPNG(t, sourcePNG, width, height)
	if _, err := executeCommand(RootCmd, "convert2texture2d", sourcePNG); err != nil {
		t.Fatalf("convert2texture2d failed: %v", err)
	}
	native := mustReadFile(t, filepath.Join(sourceDir, "body.tex"))

	texPath := filepath.Join(dir, "body.tex")
	texture2DPath := filepath.Join(dir, "body.texture2d")
	if err := os.WriteFile(texPath, native, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(texture2DPath, native, 0644); err != nil {
		t.Fatal(err)
	}
	return texPath, texture2DPath
}

func TestConvertDirectoryConvergesOnTripleBaseName(t *testing.T) {
	tempDir := t.TempDir()
	pngPath := filepath.Join(tempDir, "body.png")
	texPath, texture2DPath := writeSameNamedNativeTexture2Ds(t, tempDir, 3, 2)
	writeCmdTestPNG(t, pngPath, 4, 2)

	if _, err := executeCommand(RootCmd, "convert", tempDir); err != nil {
		t.Fatalf("convert on directory failed: %v", err)
	}

	want := decodeCmdTestImage(t, pngPath)
	if len(want) != 4*2*4 {
		t.Fatalf("expected the .png to come from the 4x2 image, got %d pixel bytes", len(want))
	}
	for _, path := range []string{texPath, texture2DPath} {
		if !KCESService.IsKCESNativeTexture2DFile(path) {
			t.Fatalf("expected %s to stay a native Texture2D", path)
		}
		if got := decodeCmdTestImage(t, renderCmdTestTexToPNG(t, path)); !bytes.Equal(got, want) {
			t.Fatalf("expected %s to hold the same pixels as the .png", path)
		}
	}
}

// TestConvert2Texture2DUpdatesEverySameNamedNativeTexture 固定 convert2texture2d 会更新全部同名原生 Texture2D
// 只更新其中一个会让另一个留着旧内容，重新打包时就会用到过期的贴图
// TestConvert2Texture2DUpdatesEverySameNamedNativeTexture pins that convert2texture2d updates every same-named native Texture2D
// Updating only one of them would leave the other with stale content that repacking could pick up
func TestConvert2Texture2DUpdatesEverySameNamedNativeTexture(t *testing.T) {
	tempDir := t.TempDir()
	pngPath := filepath.Join(tempDir, "body.png")
	texPath, texture2DPath := writeSameNamedNativeTexture2Ds(t, tempDir, 3, 2)
	writeCmdTestPNG(t, pngPath, 4, 2)

	if _, err := executeCommand(RootCmd, "convert2texture2d", pngPath); err != nil {
		t.Fatalf("convert2texture2d failed: %v", err)
	}

	want := decodeCmdTestImage(t, pngPath)
	if len(want) != 4*2*4 {
		t.Fatalf("expected the source image to be 4x2, got %d pixel bytes", len(want))
	}
	for _, path := range []string{texPath, texture2DPath} {
		if !KCESService.IsKCESNativeTexture2DFile(path) {
			t.Fatalf("expected %s to stay a native Texture2D", path)
		}
		if got := decodeCmdTestImage(t, renderCmdTestTexToPNG(t, path)); !bytes.Equal(got, want) {
			t.Fatalf("expected %s to be updated to the 4x2 image", path)
		}
	}
}

// TestConvertDirectorySkipsFileThatOnlyLooksLikeAnImage 固定「候选后缀但并非图像」的文件只被跳过，不让整批失败
// TestConvertDirectorySkipsFileThatOnlyLooksLikeAnImage pins that a file which merely looks like an image is skipped rather than failing the whole batch
func TestConvertDirectorySkipsFileThatOnlyLooksLikeAnImage(t *testing.T) {
	if err := tools.CheckMagick(); err != nil {
		t.Skipf("ImageMagick is required: %v", err)
	}

	tempDir := t.TempDir()
	// 后缀在预筛表内，内容却不是图像
	fakePath := filepath.Join(tempDir, "broken.psd")
	if err := os.WriteFile(fakePath, []byte("this is not an image"), 0644); err != nil {
		t.Fatal(err)
	}
	// 同一批里放一个真图像，确认它仍然被处理
	realPath := filepath.Join(tempDir, "real.png")
	writeCmdTestPNG(t, realPath, 3, 2)

	if _, err := executeCommand(RootCmd, "convert", tempDir); err != nil {
		t.Fatalf("convert on a directory holding a non-image with an image suffix failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tempDir, "real.tex")); err != nil {
		t.Fatalf("expected the real image to be converted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tempDir, "broken.tex")); !os.IsNotExist(err) {
		t.Fatalf("expected the non-image to be skipped, got %v", err)
	}
}
