package KCES

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDecodeKCESMisc_FromTestdataSamples(t *testing.T) {
	pathsByExt := groupMiscSamplesByExt(t)
	for ext, paths := range pathsByExt {
		ext := ext
		paths := paths
		t.Run(ext, func(t *testing.T) {
			for _, path := range paths {
				path := path
				t.Run(filepath.Base(path), func(t *testing.T) {
					assertMiscSampleRoundTripDeepEqual(t, path)
				})
			}
		})
	}
}

func groupMiscSamplesByExt(t *testing.T) map[string][]string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(miscSampleDir(), "*"))
	if err != nil {
		t.Fatalf("glob misc samples: %v", err)
	}
	if len(paths) == 0 {
		t.Skip("no misc samples found in testdata/kces_misc")
	}

	pathsByExt := map[string][]string{}
	for _, path := range paths {
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".hitcheck", ".undressdat", ".undresspdat":
			pathsByExt[ext] = append(pathsByExt[ext], path)
		default:
			t.Fatalf("unexpected misc sample suffix %q for %s", ext, filepath.Base(path))
		}
	}
	return pathsByExt
}

func assertMiscSampleRoundTripDeepEqual(t *testing.T, path string) {
	t.Helper()
	name := filepath.Base(path)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read misc sample %s: %v", path, err)
	}

	switch strings.ToLower(filepath.Ext(path)) {
	case ".hitcheck":
		hitCheck, err := DecodeHitCheck(data)
		if err != nil {
			t.Fatalf("DecodeHitCheck: %v", err)
		}
		if hitCheck.Signature != HitCheckSignature {
			t.Fatalf("signature got %q, want %q", hitCheck.Signature, HitCheckSignature)
		}
		if len(hitCheck.Entries) == 0 {
			t.Fatalf("expected hitcheck entries")
		}
		encoded, err := EncodeHitCheck(hitCheck)
		if err != nil {
			t.Fatalf("EncodeHitCheck: %v", err)
		}
		if !bytes.Equal(encoded, data) {
			t.Fatalf("%s changed after binary encode", name)
		}
		decoded, err := DecodeHitCheck(encoded)
		if err != nil {
			t.Fatalf("re-decode hitcheck: %v", err)
		}
		if !reflect.DeepEqual(decoded, hitCheck) {
			t.Fatalf("%s changed after decode/encode/decode: got %#v, want %#v", name, decoded, hitCheck)
		}
	case ".undressdat", ".undresspdat":
		value, err := DecodeKCESJSONText(data, name)
		if err != nil {
			t.Fatalf("DecodeKCESJSONText: %v", err)
		}
		if value.Extension != NormalizeKCESJSONTextExtension(name) {
			t.Fatalf("extension got %q, want %q", value.Extension, NormalizeKCESJSONTextExtension(name))
		}
		if len(value.JSON) == 0 {
			t.Fatalf("missing JSON payload")
		}
		encoded, err := EncodeKCESJSONText(value)
		if err != nil {
			t.Fatalf("EncodeKCESJSONText: %v", err)
		}
		decoded, err := DecodeKCESJSONText(encoded, name)
		if err != nil {
			t.Fatalf("re-decode JSON text: %v", err)
		}
		if !reflect.DeepEqual(decoded, value) {
			t.Fatalf("%s changed after decode/encode/decode: got %#v, want %#v", name, decoded, value)
		}
	default:
		t.Fatalf("unexpected misc sample %q", name)
	}
}

func readMiscSample(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(miscSampleDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read misc sample %s: %v", path, err)
	}
	return data
}

func miscSampleDir() string {
	return filepath.Join("..", "..", "testdata", "kces_misc")
}
