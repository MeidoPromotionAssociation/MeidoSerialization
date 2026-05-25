package COM3D2

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDanceService_DanceObjectData(t *testing.T) {
	patterns := []string{
		"../../testdata/bytes/*/maid_data.bytes",
		"../../testdata/bytes/*/item_data.bytes",
		"../../testdata/bytes/*/event_data.bytes",
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

	s := &DanceService{}
	for _, inputPath := range files {
		t.Run(filepath.Dir(inputPath)+"/"+filepath.Base(inputPath), func(t *testing.T) {
			tempDir := t.TempDir()
			jsonPath := filepath.Join(tempDir, "test.bytes.json")
			backPath := filepath.Join(tempDir, "test.bytes")

			data, err := s.ReadDanceObjectDataFile(inputPath)
			if err != nil {
				t.Fatalf("ReadDanceObjectDataFile failed: %v", err)
			}

			if err := s.ConvertDanceObjectDataToJson(inputPath, jsonPath); err != nil {
				t.Fatalf("ConvertDanceObjectDataToJson failed: %v", err)
			}

			if err := s.ConvertJsonToDanceObjectData(jsonPath, backPath); err != nil {
				t.Fatalf("ConvertJsonToDanceObjectData failed: %v", err)
			}

			dataBack, err := s.ReadDanceObjectDataFile(backPath)
			if err != nil {
				t.Fatalf("Read re-converted data failed: %v", err)
			}

			if !reflect.DeepEqual(data, dataBack) {
				t.Errorf("data mismatch after JSON conversion cycle")
			}

			origBytes, _ := os.ReadFile(inputPath)
			backBytes, _ := os.ReadFile(backPath)
			if !reflect.DeepEqual(origBytes, backBytes) {
				t.Errorf("byte-level mismatch: orig=%d bytes, back=%d bytes",
					len(origBytes), len(backBytes))
			}
		})
	}
}

func TestDanceService_TimelineData(t *testing.T) {
	files, err := filepath.Glob("../../testdata/bytes/*/timeline_data.bytes")
	if err != nil {
		t.Fatal(err)
	}

	if len(files) == 0 {
		t.Skip("no timeline_data.bytes test files found")
	}

	s := &DanceService{}
	for _, inputPath := range files {
		t.Run(filepath.Dir(inputPath)+"/"+filepath.Base(inputPath), func(t *testing.T) {
			tempDir := t.TempDir()
			jsonPath := filepath.Join(tempDir, "test.bytes.json")
			backPath := filepath.Join(tempDir, "test.bytes")

			data, err := s.ReadTimelineDataFile(inputPath)
			if err != nil {
				t.Fatalf("ReadTimelineDataFile failed: %v", err)
			}

			if err := s.ConvertTimelineDataToJson(inputPath, jsonPath); err != nil {
				t.Fatalf("ConvertTimelineDataToJson failed: %v", err)
			}

			if err := s.ConvertJsonToTimelineData(jsonPath, backPath); err != nil {
				t.Fatalf("ConvertJsonToTimelineData failed: %v", err)
			}

			dataBack, err := s.ReadTimelineDataFile(backPath)
			if err != nil {
				t.Fatalf("Read re-converted data failed: %v", err)
			}

			if !reflect.DeepEqual(data, dataBack) {
				t.Errorf("data mismatch after JSON conversion cycle")
			}

			origBytes, _ := os.ReadFile(inputPath)
			backBytes, _ := os.ReadFile(backPath)
			if !reflect.DeepEqual(origBytes, backBytes) {
				t.Errorf("byte-level mismatch: orig=%d bytes, back=%d bytes",
					len(origBytes), len(backBytes))
			}
		})
	}
}

func TestDanceService_SniffDanceBytesType(t *testing.T) {
	s := &DanceService{}

	timelineFiles, _ := filepath.Glob("../../testdata/bytes/*/timeline_data.bytes")
	for _, f := range timelineFiles {
		bytesType, err := s.SniffDanceBytesType(f)
		if err != nil {
			t.Fatalf("SniffDanceBytesType(%s) failed: %v", f, err)
		}
		if bytesType != DanceBytesTimeline {
			t.Errorf("SniffDanceBytesType(%s) = %q, want %q", f, bytesType, DanceBytesTimeline)
		}
	}

	objectFiles, _ := filepath.Glob("../../testdata/bytes/*/maid_data.bytes")
	for _, f := range objectFiles {
		bytesType, err := s.SniffDanceBytesType(f)
		if err != nil {
			t.Fatalf("SniffDanceBytesType(%s) failed: %v", f, err)
		}
		if bytesType != DanceBytesObjectData {
			t.Errorf("SniffDanceBytesType(%s) = %q, want %q", f, bytesType, DanceBytesObjectData)
		}
	}
}
