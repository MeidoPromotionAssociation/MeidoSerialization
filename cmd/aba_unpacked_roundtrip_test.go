package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	KCESService "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/service/KCES"
)

// abaUnpackedRoundTripEnv gates the exhaustive unpacked round-trip test
const abaUnpackedRoundTripEnv = "KCES_ABA_HEAVY_TESTS"

// abaUnpackedRoundTripWorkerLimit caps concurrency because each worker holds two editing JSON documents at once
const abaUnpackedRoundTripWorkerLimit = 4

// TestKCESUnpackedAbaFilesEditingJSONRoundTrip unpacks every ABA in testdata and runs each routable unpacked file
// through native -> editing JSON -> native -> editing JSON, requiring both editing JSON documents to agree.
// The criterion is editing JSON equality rather than native byte equality because some formats pad out to a newer
// indexed-array width or switch to the shortest integer encoding when re-encoded, and neither changes what the game reads.
// Menu.guid is the one tolerated difference because encoding recalculates it from a fresh UUID v4, and view-only
// native Unity objects are only required to have their editing JSON rejected on the way back.
func TestKCESUnpackedAbaFilesEditingJSONRoundTrip(t *testing.T) {
	if os.Getenv(abaUnpackedRoundTripEnv) == "" {
		t.Skipf("set %s=1 to convert every unpacked KCES ABA file in both directions", abaUnpackedRoundTripEnv)
	}
	samples, err := filepath.Glob(filepath.Join("..", "testdata", "KCES", "*.aba"))
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) == 0 {
		t.Skip("no ABA samples available")
	}
	sort.Strings(samples)
	resetConversionFlags(t)
	discardConversionProgress(t)

	tally := &unpackedRoundTripTally{}
	for _, sample := range samples {
		sample := sample
		t.Run(filepath.Base(sample), func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "unpacked")
			if err := (&KCESService.AbaService{}).UnpackAba(sample, root); err != nil {
				if strings.Contains(strings.ToLower(err.Error()), "encrypted") {
					tally.addEncrypted()
					t.Skipf("encrypted ABA rejected as expected: %v", err)
				}
				t.Fatalf("unpack %s: %v", sample, err)
			}
			before := listUnpackedFiles(t, root)
			if len(before) == 0 {
				t.Fatalf("%s unpacked into an empty directory", sample)
			}
			roundTripUnpackedFiles(t, root, before, tally)
			if after := listUnpackedFiles(t, root); !slicesEqualStrings(before, after) {
				t.Fatalf("round trip changed the unpacked file set of %s: %d files before, %d after", sample, len(before), len(after))
			}
		})
	}
	tally.report(t)
}

// roundTripUnpackedFiles round trips every file in a pure directory with a fixed worker pool
func roundTripUnpackedFiles(t *testing.T, root string, relativePaths []string, tally *unpackedRoundTripTally) {
	t.Helper()
	workerCount := runtime.NumCPU()
	if workerCount > abaUnpackedRoundTripWorkerLimit {
		workerCount = abaUnpackedRoundTripWorkerLimit
	}
	if workerCount < 1 {
		workerCount = 1
	}
	paths := make(chan string, workerCount)
	failures := make(chan string, len(relativePaths))
	var wg sync.WaitGroup
	for worker := 0; worker < workerCount; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for relativePath := range paths {
				path := filepath.Join(root, filepath.FromSlash(relativePath))
				if !isConvertibleNativeFile(path) {
					tally.addUnroutable(relativePath)
					continue
				}
				difference, err := roundTripEditingJSON(path)
				if err != nil {
					failures <- fmt.Sprintf("%s: %v", relativePath, err)
					continue
				}
				tally.addRoundTripped(relativePath, difference)
			}
		}()
	}
	for _, relativePath := range relativePaths {
		paths <- relativePath
	}
	close(paths)
	wg.Wait()
	close(failures)
	for failure := range failures {
		t.Errorf("editing JSON round trip failed for %s", failure)
	}
}

// roundTripEditingJSON converts a native file to editing JSON, back to native, and to editing JSON again.
// Editing JSON marked readOnly cannot be converted back, so it only has to be rejected.
func roundTripEditingJSON(path string) (editingJSONDifference, error) {
	jsonPath := path + ".json"
	if err := convertToJson(path); err != nil {
		return editingJSONChanged, err
	}
	defer func() { _ = os.Remove(jsonPath) }()
	first, err := os.ReadFile(jsonPath)
	if err != nil {
		return editingJSONChanged, err
	}
	if !isConvertibleEditingJSONFile(jsonPath) {
		return editingJSONChanged, fmt.Errorf("editing JSON %q is not routed back to its native format", filepath.Base(jsonPath))
	}
	if isReadOnlyEditingJSON(first) {
		if err := convertToMod(jsonPath); err == nil {
			return editingJSONChanged, fmt.Errorf("read-only editing JSON was accepted as a native-format source")
		}
		return editingJSONReadOnly, nil
	}
	if err := convertToMod(jsonPath); err != nil {
		return editingJSONChanged, err
	}
	if err := os.Remove(jsonPath); err != nil {
		return editingJSONChanged, err
	}
	if err := convertToJson(path); err != nil {
		return editingJSONChanged, fmt.Errorf("re-convert the round-tripped native file: %w", err)
	}
	second, err := os.ReadFile(jsonPath)
	if err != nil {
		return editingJSONChanged, err
	}
	difference, err := compareEditingJSON(first, second)
	if err != nil {
		return editingJSONChanged, fmt.Errorf("compare the editing JSON produced before and after the native round trip: %w", err)
	}
	if difference == editingJSONChanged {
		return difference, fmt.Errorf("editing JSON changed after the native round trip: %d bytes became %d bytes, first difference at offset %d",
			len(first), len(second), firstByteDifference(first, second))
	}
	return difference, nil
}

// isReadOnlyEditingJSON reports whether editing JSON declares itself view-only
func isReadOnlyEditingJSON(data []byte) bool {
	var header struct {
		ReadOnly bool `json:"readOnly"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return false
	}
	return header.ReadOnly
}

// editingJSONDifference 描述互转前后两份编辑 JSON 的差异类型 / editingJSONDifference describes how two editing JSON documents differ across a round trip
type editingJSONDifference int

const (
	// editingJSONIdentical 表示两份编辑 JSON 逐字节相同 / editingJSONIdentical means both documents are byte-identical
	editingJSONIdentical editingJSONDifference = iota
	// editingJSONRecalculatedGUIDOnly 表示两份编辑 JSON 仅在重算的数值 guid 上不同 / editingJSONRecalculatedGUIDOnly means both documents differ only in the recalculated numeric guid
	editingJSONRecalculatedGUIDOnly
	// editingJSONReadOnly 表示编辑 JSON 仅供查看，无法转回原生格式 / editingJSONReadOnly means the editing JSON is view-only and cannot be converted back
	editingJSONReadOnly
	// editingJSONChanged 表示两份编辑 JSON 存在数值 guid 之外的差异 / editingJSONChanged means both documents differ beyond the numeric guid
	editingJSONChanged
)

// compareEditingJSON compares the editing JSON produced before and after a native round trip.
// Menu.guid is the only tolerated difference: KCES part writers recalculate it, and without a
// HairMake.exportedGuid source it comes from a fresh UUID v4 that the game never stores either.
func compareEditingJSON(first []byte, second []byte) (editingJSONDifference, error) {
	if bytes.Equal(first, second) {
		return editingJSONIdentical, nil
	}
	firstValue, err := decodeEditingJSONWithoutNumericGUID(first)
	if err != nil {
		return editingJSONChanged, err
	}
	secondValue, err := decodeEditingJSONWithoutNumericGUID(second)
	if err != nil {
		return editingJSONChanged, err
	}
	if reflect.DeepEqual(firstValue, secondValue) {
		return editingJSONRecalculatedGUIDOnly, nil
	}
	return editingJSONChanged, nil
}

// decodeEditingJSONWithoutNumericGUID parses editing JSON and drops numeric guid members, keeping numbers as
// json.Number so that uint64 values stay exact
func decodeEditingJSONWithoutNumericGUID(data []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return stripNumericGUIDMembers(value), nil
}

// stripNumericGUIDMembers removes numeric guid members and keeps string guid members, which are not recalculated
func stripNumericGUIDMembers(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, member := range typed {
			if key == "guid" {
				if _, numeric := member.(json.Number); numeric {
					delete(typed, key)
					continue
				}
			}
			typed[key] = stripNumericGUIDMembers(member)
		}
		return typed
	case []any:
		for index, member := range typed {
			typed[index] = stripNumericGUIDMembers(member)
		}
		return typed
	default:
		return value
	}
}

// firstByteDifference returns the offset of the first difference, or the shorter length when one is a prefix of the other
func firstByteDifference(first []byte, second []byte) int {
	shorter := len(first)
	if len(second) < shorter {
		shorter = len(second)
	}
	for offset := 0; offset < shorter; offset++ {
		if first[offset] != second[offset] {
			return offset
		}
	}
	return shorter
}

// listUnpackedFiles returns the sorted relative paths of every file in a pure directory
func listUnpackedFiles(t *testing.T, root string) []string {
	t.Helper()
	var result []string
	err := filepath.Walk(root, func(name string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		result = append(result, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		t.Fatalf("walk %q: %v", root, err)
	}
	sort.Strings(result)
	return result
}

// slicesEqualStrings reports whether two string slices are element-wise equal
func slicesEqualStrings(want []string, got []string) bool {
	if len(want) != len(got) {
		return false
	}
	for index := range want {
		if want[index] != got[index] {
			return false
		}
	}
	return true
}

// resetConversionFlags clears the global type-filter flags so the round trip covers every unpacked file
func resetConversionFlags(t *testing.T) {
	t.Helper()
	oldType, oldStrict := fileType, strictMode
	t.Cleanup(func() { fileType, strictMode = oldType, oldStrict })
	fileType = ""
	strictMode = false
}

// discardConversionProgress drops the per-file progress that the conversion functions print to standard output
func discardConversionProgress(t *testing.T) {
	t.Helper()
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	old := os.Stdout
	os.Stdout = devNull
	t.Cleanup(func() {
		os.Stdout = old
		devNull.Close()
	})
}

// unpackedRoundTripTally 按解包产物后缀统计互转结果 / unpackedRoundTripTally counts round-trip outcomes by unpacked-artifact suffix
type unpackedRoundTripTally struct {
	// mutex 保护所有计数，因为工作协程会并发上报 / mutex guards every counter because workers report concurrently
	mutex sync.Mutex
	// roundTripped 记录每个后缀完成互转的文件数 / roundTripped counts files per suffix that completed a round trip
	roundTripped map[string]int
	// recalculatedGUID 记录每个后缀只在重算 guid 上不同的文件数 / recalculatedGUID counts files per suffix that differ only in the recalculated guid
	recalculatedGUID map[string]int
	// readOnly 记录每个后缀编辑 JSON 仅供查看的文件数 / readOnly counts files per suffix whose editing JSON is view-only
	readOnly map[string]int
	// unroutable 记录每个后缀没有编辑 JSON 路由的文件数 / unroutable counts files per suffix without an editing JSON route
	unroutable map[string]int
	// encrypted 记录被拒绝的加密 ABA 数量 / encrypted counts rejected encrypted ABAs
	encrypted int
}

// addRoundTripped records one file that completed a round trip together with how its editing JSON differed
func (tally *unpackedRoundTripTally) addRoundTripped(relativePath string, difference editingJSONDifference) {
	suffix := unpackedArtifactSuffix(relativePath)
	tally.mutex.Lock()
	defer tally.mutex.Unlock()
	if difference == editingJSONReadOnly {
		if tally.readOnly == nil {
			tally.readOnly = map[string]int{}
		}
		tally.readOnly[suffix]++
		return
	}
	if tally.roundTripped == nil {
		tally.roundTripped = map[string]int{}
	}
	tally.roundTripped[suffix]++
	if difference != editingJSONRecalculatedGUIDOnly {
		return
	}
	if tally.recalculatedGUID == nil {
		tally.recalculatedGUID = map[string]int{}
	}
	tally.recalculatedGUID[suffix]++
}

// addUnroutable records one file without an editing JSON route
func (tally *unpackedRoundTripTally) addUnroutable(relativePath string) {
	suffix := unpackedArtifactSuffix(relativePath)
	tally.mutex.Lock()
	defer tally.mutex.Unlock()
	if tally.unroutable == nil {
		tally.unroutable = map[string]int{}
	}
	tally.unroutable[suffix]++
}

// addEncrypted records one rejected encrypted ABA
func (tally *unpackedRoundTripTally) addEncrypted() {
	tally.mutex.Lock()
	defer tally.mutex.Unlock()
	tally.encrypted++
}

// report prints the round-trip summary by suffix and requires every major format to stay covered
func (tally *unpackedRoundTripTally) report(t *testing.T) {
	t.Helper()
	tally.mutex.Lock()
	defer tally.mutex.Unlock()
	total := 0
	for _, count := range tally.roundTripped {
		total += count
	}
	if total == 0 {
		t.Fatal("no unpacked file completed an editing JSON round trip")
	}
	t.Logf("round tripped %d unpacked files through editing JSON; %d encrypted ABAs were rejected", total, tally.encrypted)
	t.Logf("round tripped by suffix: %s", formatSuffixTally(tally.roundTripped))
	if len(tally.recalculatedGUID) > 0 {
		t.Logf("editing JSON differed only in the recalculated guid by suffix: %s", formatSuffixTally(tally.recalculatedGUID))
	}
	if len(tally.readOnly) > 0 {
		t.Logf("editing JSON is view-only and was rejected as a native-format source by suffix: %s", formatSuffixTally(tally.readOnly))
	}
	if len(tally.unroutable) > 0 {
		t.Logf("no editing JSON route by suffix: %s", formatSuffixTally(tally.unroutable))
	}
	// 覆盖门槛，避免某种格式在路由变化后被静默跳过
	// Coverage gate that keeps a format from being silently skipped after a routing change
	for _, suffix := range []string{
		".mmesh", ".tex", ".texture2d", ".sprite", ".partsatlas", ".monobehaviour",
		".model", ".menuassets", ".materialassets", ".pmatassets",
		".dbcol", ".dbconf", ".db2conf", ".dsbconf", ".dslcol", ".ikcol", ".limbcol", ".hitcheck", ".psk",
	} {
		if tally.roundTripped[suffix] == 0 {
			t.Errorf("the round trip did not cover any %s artifact", suffix)
		}
	}
}

// unpackedArtifactSuffix returns the lowercase tally suffix, keeping the compound .bytes suffix of raw Unity objects
func unpackedArtifactSuffix(relativePath string) string {
	name := strings.ToLower(filepath.Base(relativePath))
	suffix := filepath.Ext(name)
	if suffix == ".bytes" {
		if inner := filepath.Ext(strings.TrimSuffix(name, suffix)); inner != "" {
			return inner + suffix
		}
	}
	if suffix == "" {
		return "(no suffix)"
	}
	return suffix
}

// formatSuffixTally formats suffix counts as a stable single-line summary sorted by suffix
func formatSuffixTally(counts map[string]int) string {
	suffixes := make([]string, 0, len(counts))
	for suffix := range counts {
		suffixes = append(suffixes, suffix)
	}
	sort.Strings(suffixes)
	parts := make([]string, 0, len(suffixes))
	for _, suffix := range suffixes {
		parts = append(parts, fmt.Sprintf("%s=%d", suffix, counts[suffix]))
	}
	return strings.Join(parts, " ")
}
