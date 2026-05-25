package COM3D2

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDanceObjectData(t *testing.T) {
	patterns := []string{
		"../../testdata/*/*/maid_data.bytes",
		"../../testdata/*/*/item_data.bytes",
		"../../testdata/*/*/event_data.bytes",
		"../../testdata/*/maid_data.bytes",
		"../../testdata/*/item_data.bytes",
		"../../testdata/*/event_data.bytes",
	}

	var files []string
	for _, p := range patterns {
		matches, err := filepath.Glob(p)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, matches...)
	}

	if len(files) == 0 {
		t.Skip("no dance object data test files found")
	}

	for _, filePath := range files {
		t.Run(filepath.Dir(filePath)+"/"+filepath.Base(filePath), func(t *testing.T) {
			f, err := os.Open(filePath)
			if err != nil {
				t.Fatalf("failed to open test file: %v", err)
			}
			defer f.Close()

			data, err := ReadDanceObjectData(f)
			if err != nil {
				t.Fatalf("failed to read dance object data: %v", err)
			}

			var buf bytes.Buffer
			if err := data.Dump(&buf); err != nil {
				t.Fatalf("failed to dump dance object data: %v", err)
			}

			data2, err := ReadDanceObjectData(&buf)
			if err != nil {
				t.Fatalf("failed to re-read dumped dance object data: %v", err)
			}

			if !reflect.DeepEqual(data, data2) {
				t.Errorf("data mismatch after dump and re-read")
			}

			origBytes, err := os.ReadFile(filePath)
			if err != nil {
				t.Fatalf("failed to read original file bytes: %v", err)
			}
			var roundTripBuf bytes.Buffer
			if err := data.Dump(&roundTripBuf); err != nil {
				t.Fatalf("failed to dump for byte comparison: %v", err)
			}
			if !bytes.Equal(origBytes, roundTripBuf.Bytes()) {
				t.Errorf("byte-level mismatch after roundtrip: orig=%d bytes, dumped=%d bytes",
					len(origBytes), roundTripBuf.Len())
			}
		})
	}
}

func TestTimelineData(t *testing.T) {
	patterns := []string{
		"../../testdata/*/*/timeline_data.bytes",
		"../../testdata/*/timeline_data.bytes",
	}

	var files []string
	for _, p := range patterns {
		matches, err := filepath.Glob(p)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, matches...)
	}

	if len(files) == 0 {
		t.Skip("no timeline_data.bytes test files found")
	}

	for _, filePath := range files {
		t.Run(filepath.Dir(filePath)+"/"+filepath.Base(filePath), func(t *testing.T) {
			f, err := os.Open(filePath)
			if err != nil {
				t.Fatalf("failed to open test file: %v", err)
			}
			defer f.Close()

			data, err := ReadTimelineData(f)
			if err != nil {
				t.Fatalf("failed to read timeline data: %v", err)
			}

			var buf bytes.Buffer
			if err := data.Dump(&buf); err != nil {
				t.Fatalf("failed to dump timeline data: %v", err)
			}

			data2, err := ReadTimelineData(&buf)
			if err != nil {
				t.Fatalf("failed to re-read dumped timeline data: %v", err)
			}

			if !reflect.DeepEqual(data, data2) {
				t.Errorf("data mismatch after dump and re-read")
			}

			origBytes, err := os.ReadFile(filePath)
			if err != nil {
				t.Fatalf("failed to read original file bytes: %v", err)
			}
			var roundTripBuf bytes.Buffer
			if err := data.Dump(&roundTripBuf); err != nil {
				t.Fatalf("failed to dump for byte comparison: %v", err)
			}
			if !bytes.Equal(origBytes, roundTripBuf.Bytes()) {
				t.Errorf("byte-level mismatch after roundtrip: orig=%d bytes, dumped=%d bytes",
					len(origBytes), roundTripBuf.Len())
			}
		})
	}
}
