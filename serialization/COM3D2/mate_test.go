package COM3D2

import (
	"bufio"
	"bytes"
	"os"
	"testing"
)

func TestMate(t *testing.T) {
	filePath := "../../testdata/test.mate"
	f, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("failed to open test file: %v", err)
	}
	defer f.Close()

	br := bufio.NewReader(f)
	mate, err := ReadMate(br)
	if err != nil {
		t.Fatalf("failed to read mate: %v", err)
	}

	// Test Dump
	var buf bytes.Buffer
	err = mate.Dump(&buf)
	if err != nil {
		t.Fatalf("failed to dump mate: %v", err)
	}

	// Re-read from dumped buffer
	br2 := bufio.NewReader(&buf)
	mate2, err := ReadMate(br2)
	if err != nil {
		t.Fatalf("failed to re-read dumped mate: %v", err)
	}

	// Compare basic fields
	if mate.Name != mate2.Name {
		t.Errorf("name mismatch: %s != %s", mate.Name, mate2.Name)
	}
}
