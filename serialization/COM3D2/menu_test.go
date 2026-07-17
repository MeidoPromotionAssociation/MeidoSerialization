package COM3D2

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMenu(t *testing.T) {
	files, err := filepath.Glob("../../testdata/*.menu")
	if err != nil {
		t.Fatal(err)
	}

	for _, filePath := range files {
		t.Run(filepath.Base(filePath), func(t *testing.T) {
			f, err := os.Open(filePath)
			if err != nil {
				t.Fatalf("failed to open test file: %v", err)
			}
			defer f.Close()

			br := bufio.NewReader(f)
			menu, err := ReadMenu(br)
			if err != nil {
				t.Fatalf("failed to read menu: %v", err)
			}

			// Test Dump
			var buf bytes.Buffer
			err = menu.Dump(&buf)
			if err != nil {
				t.Fatalf("failed to dump menu: %v", err)
			}

			// Re-read from dumped buffer
			br2 := bufio.NewReader(&buf)
			menu2, err := ReadMenu(br2)
			if err != nil {
				t.Fatalf("failed to re-read dumped menu: %v", err)
			}

			// Compare complete structure
			if !reflect.DeepEqual(menu, menu2) {
				t.Errorf("data mismatch after dump and re-read")
			}
		})
	}
}

func TestMenuDumpRecalculatesBodySizeAndRejectsEmptyCommand(t *testing.T) {
	menu := &Menu{
		Signature:   MenuSignature,
		Version:     777,
		SrcFileName: "source.menu",
		ItemName:    "item",
		Category:    "category",
		InfoText:    "info",
		BodySize:    123,
		Commands:    []Command{{Command: "command", Args: []string{"x"}}},
	}

	var wire bytes.Buffer
	if err := menu.Dump(&wire); err != nil {
		t.Fatalf("Dump: %v", err)
	}
	wantBodySize, err := menu.CalculateBodySize()
	if err != nil {
		t.Fatal(err)
	}
	if menu.BodySize != wantBodySize || menu.BodySize == 123 {
		t.Fatalf("Dump BodySize = %d, want recalculated %d", menu.BodySize, wantBodySize)
	}
	decoded, err := ReadMenu(bufio.NewReader(bytes.NewReader(wire.Bytes())))
	if err != nil {
		t.Fatalf("ReadMenu: %v", err)
	}
	if decoded.BodySize != wantBodySize || len(decoded.Commands) != 1 || decoded.Commands[0].Command != "command" || !reflect.DeepEqual(decoded.Commands[0].Args, []string{"x"}) {
		t.Fatalf("stored menu fields changed: %#v", decoded)
	}

	var reencoded bytes.Buffer
	if err := decoded.Dump(&reencoded); err != nil {
		t.Fatalf("re-Dump: %v", err)
	}
	if !bytes.Equal(reencoded.Bytes(), wire.Bytes()) {
		t.Fatal("menu wire changed after round-trip")
	}

	invalid := *menu
	invalid.Commands = []Command{{Command: "", Args: []string{"x"}}}
	wire.Reset()
	if err := invalid.Dump(&wire); err == nil {
		t.Fatal("Dump accepted an empty command name")
	}
	if wire.Len() != 0 {
		t.Fatalf("rejected empty command wrote %d bytes", wire.Len())
	}
}

func TestMenuDumpReplacesStaleNegativeBodySize(t *testing.T) {
	menu := &Menu{Signature: MenuSignature, BodySize: -1}
	var wire bytes.Buffer
	if err := menu.Dump(&wire); err != nil {
		t.Fatalf("Dump should recalculate BodySize: %v", err)
	}
	if menu.BodySize != 1 {
		t.Fatalf("recalculated BodySize = %d, want 1", menu.BodySize)
	}
}
