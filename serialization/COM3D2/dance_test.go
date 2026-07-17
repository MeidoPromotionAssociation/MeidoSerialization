package COM3D2

import (
	"bytes"
	"encoding/json"
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

func TestTimelineUnknownPropertyValueTypeUsesIndexCountDirectly(t *testing.T) {
	original := &TimelineData{
		TotalFrame: 10,
		FrameRate:  60,
		Tracks: []TimelineTrack{&PropertyTrack{
			TrackID:        7,
			TotalFrame:     10,
			ObjectTreePath: "Root/Object",
			ValueType:      99,
			ComponentName:  "FutureComponent",
			PropertyName:   "FutureProperty",
			IndexArray:     []KeyValuePairInt{{Key: 9, Value: 3}},
		}},
	}

	var wire bytes.Buffer
	if err := original.Dump(&wire); err != nil {
		t.Fatalf("Dump: %v", err)
	}
	encoded := append([]byte(nil), wire.Bytes()...)
	decoded, err := ReadTimelineData(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("ReadTimelineData: %v", err)
	}
	if !reflect.DeepEqual(decoded, original) {
		t.Fatalf("unknown PropertyTrack changed:\n got %#v\nwant %#v", decoded, original)
	}
	wire.Reset()
	if err := decoded.Dump(&wire); err != nil {
		t.Fatalf("re-Dump: %v", err)
	}
	if !bytes.Equal(wire.Bytes(), encoded) {
		t.Fatalf("unknown PropertyTrack wire changed:\n got %x\nwant %x", wire.Bytes(), encoded)
	}
}

func TestTimelineDumpRejectsContradictoryOrNilFieldsBeforeWriting(t *testing.T) {
	tests := []struct {
		name string
		data *TimelineData
	}{
		{
			name: "translation frame count mismatch",
			data: &TimelineData{TotalFrame: 2, Tracks: []TimelineTrack{&TranslationTrack{
				TotalFrame: 2,
				PosArray:   []Vector3{{}},
			}}},
		},
		{
			name: "typed nil track",
			data: &TimelineData{Tracks: []TimelineTrack{(*RotationTrack)(nil)}},
		},
		{
			name: "nil vector parameter",
			data: &TimelineData{Tracks: []TimelineTrack{&EventTrack{MethodDataArray: []MethodData{{
				Params: []EventParameter{{ValueType: 4}},
			}}}}},
		},
		{
			name: "property array for another value type",
			data: &TimelineData{Tracks: []TimelineTrack{&PropertyTrack{
				ValueType:     0,
				FloatValArray: []float32{1},
			}}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var wire bytes.Buffer
			if err := test.data.Dump(&wire); err == nil {
				t.Fatal("Dump accepted contradictory timeline data")
			}
			if wire.Len() != 0 {
				t.Fatalf("rejected timeline wrote %d bytes", wire.Len())
			}
		})
	}
}

func TestTimelineDumpRejectsExcessiveEventParameterNesting(t *testing.T) {
	parameter := EventParameter{ValueType: 0}
	for i := 0; i < maxEventParameterDepth; i++ {
		parameter = EventParameter{ValueType: 12, Array: []EventParameter{parameter}}
	}
	data := &TimelineData{Tracks: []TimelineTrack{&EventTrack{MethodDataArray: []MethodData{{Params: []EventParameter{parameter}}}}}}
	var wire bytes.Buffer
	if err := data.Dump(&wire); err == nil {
		t.Fatal("Dump accepted excessive EventParameter nesting")
	}
	if wire.Len() != 0 {
		t.Fatalf("rejected nested parameter wrote %d bytes", wire.Len())
	}
}

func TestTimelineMarshalJSONRejectsNilTracksWithoutPanic(t *testing.T) {
	tests := []struct {
		name  string
		track TimelineTrack
	}{
		{name: "nil interface", track: nil},
		{name: "typed nil", track: (*PropertyTrack)(nil)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := &TimelineData{Tracks: []TimelineTrack{test.track}}
			if _, err := json.Marshal(value); err == nil {
				t.Fatal("json.Marshal unexpectedly accepted a nil track")
			}
		})
	}
}
