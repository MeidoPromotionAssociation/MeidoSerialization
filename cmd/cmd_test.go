package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

	// Also capture stdout as many parts of the code use fmt.Printf
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = root.Execute()

	w.Close()
	os.Stdout = old
	var outBuf bytes.Buffer
	outBuf.ReadFrom(r)

	return buf.String() + outBuf.String(), err
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
