package KCES

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/internal/kcesfixtures"
)

func TestKCESPartsSamplesHaveKnownSuffixes(t *testing.T) {
	knownSuffixes := map[string]struct{}{
		".menuassets":     {},
		".materialassets": {},
		".model":          {},
		".pmatassets":     {},
	}
	for _, path := range allPartsSamplePaths(t) {
		ext := strings.ToLower(filepath.Ext(path))
		if _, ok := knownSuffixes[ext]; !ok {
			t.Fatalf("unexpected parts sample suffix %q for %s", ext, filepath.Base(path))
		}
	}
}

func assertPartsSamplesForSuffixRoundTrip[T any](
	t *testing.T,
	suffix string,
	decode func([]byte) (*T, error),
	encode func(*T) ([]byte, error),
	normalizeExpected ...func(*T),
) {
	t.Helper()
	for _, path := range partsSamplePathsBySuffix(t, suffix) {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			assertPartsSampleRoundTripDeepEqual(t, path, decode, encode, normalizeExpected...)
		})
	}
}

func partsSamplePathsBySuffix(t *testing.T, suffix string) []string {
	t.Helper()
	var matches []string
	for _, path := range allPartsSamplePaths(t) {
		if strings.EqualFold(filepath.Ext(path), suffix) {
			matches = append(matches, path)
		}
	}
	if len(matches) == 0 {
		t.Skipf("no parts samples with suffix %s", suffix)
	}
	return matches
}

func allPartsSamplePaths(t *testing.T) []string {
	t.Helper()
	return kcesfixtures.PartsSamplePaths(t)
}

func assertPartsSampleRoundTripDeepEqual[T any](
	t *testing.T,
	path string,
	decode func([]byte) (*T, error),
	encode func(*T) ([]byte, error),
	normalizeExpected ...func(*T),
) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read parts sample %s: %v", path, err)
	}
	original, err := decode(data)
	if err != nil {
		t.Fatalf("decode %s: %v", filepath.Base(path), err)
	}
	encoded, err := encode(original)
	if err != nil {
		t.Fatalf("encode %s: %v", filepath.Base(path), err)
	}
	for _, normalize := range normalizeExpected {
		normalize(original)
	}
	decoded, err := decode(encoded)
	if err != nil {
		t.Fatalf("re-decode %s: %v", filepath.Base(path), err)
	}
	if !reflect.DeepEqual(decoded, original) {
		t.Fatalf("%s changed after decode/encode/decode: got %#v, want %#v", filepath.Base(path), decoded, original)
	}
}
