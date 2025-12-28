package arc

import (
	"os"
	"testing"
)

func TestArc(t *testing.T) {
	filePath := "../../../testdata/test.arc"

	// Skip if test file doesn't exist
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Skip("test.arc not found")
	}

	arc, err := ReadArc(filePath)
	if err != nil {
		t.Fatalf("failed to read arc: %v", err)
	}

	if arc == nil {
		t.Fatal("arc is nil")
	}

	if arc.Root == nil {
		t.Error("arc root is nil")
	}

	// Test Dump
	tempArc := "test_output.arc"
	err = arc.Dump(tempArc)
	if err != nil {
		t.Fatalf("failed to dump arc: %v", err)
	}
	defer os.Remove(tempArc)

	fi, err := os.Stat(tempArc)
	if err == nil {
		t.Logf("dumped arc size: %d bytes", fi.Size())
	}

	// Re-read
	arc2, err := ReadArc(tempArc)
	if err != nil {
		t.Fatalf("failed to re-read dumped arc from %s: %v", tempArc, err)
	}

	if arc2 == nil {
		t.Fatal("re-read arc is nil")
	}
}
