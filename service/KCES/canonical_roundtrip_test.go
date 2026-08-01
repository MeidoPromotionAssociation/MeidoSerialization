package KCES

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

const canonicalAbaHeavyTestEnv = "KCES_ABA_HEAVY_TESTS"

func TestCanonicalAbaPureDirectoryRoundTrip(t *testing.T) {
	sample := filepath.Join("..", "..", "testdata", "aba", "parts_personal_om015_gp003.aba")
	if _, err := os.Stat(sample); err != nil {
		t.Skipf("sample not available: %v", err)
	}
	work := t.TempDir()
	first := filepath.Join(work, "first")
	if err := (&AbaService{}).UnpackAba(sample, first); err != nil {
		t.Fatalf("first unpack: %v", err)
	}
	firstFiles := validateCanonicalDirectoryFiles(t, first, nil)
	if len(firstFiles) == 0 {
		t.Fatal("pure directory is empty")
	}
	assertPureDirectoryFileSet(t, firstFiles)
	if err := (&PackService{}).PackToAbaAndCt(first, "roundtrip"); err != nil {
		t.Fatalf("pack: %v", err)
	}
	abPath := filepath.Join(work, "roundtrip.aba")
	ctPath := filepath.Join(work, "roundtrip.ct")
	verifyCanonicalAbaAndCatalog(t, abPath, ctPath)
	verifyPackedAbaMatchesDirectory(t, first, firstFiles, abPath)

	second := filepath.Join(work, "second")
	if err := (&AbaService{}).UnpackAba(abPath, second); err != nil {
		t.Fatalf("second unpack: %v", err)
	}
	secondFiles := validateCanonicalDirectoryFiles(t, second, nil)
	assertCanonicalDirectoryPathsEqual(t, firstFiles, secondFiles)
}

func TestCanonicalAbaAllReadableSamplesPureDirectoryRoundTrip(t *testing.T) {
	if os.Getenv(canonicalAbaHeavyTestEnv) == "" {
		t.Skipf("set %s=1 to round trip every readable KCES ABA sample", canonicalAbaHeavyTestEnv)
	}
	const parallelSlots = 10
	samples, err := filepath.Glob(filepath.Join("..", "..", "testdata", "aba", "*.aba"))
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) == 0 {
		t.Skip("no ABA samples available")
	}
	var readableCount atomic.Int64
	var encryptedCount atomic.Int64
	typeCounts := map[int32]*atomic.Int64{
		aba.ClassIDMesh:          {},
		aba.ClassIDTexture2D:     {},
		aba.ClassIDSprite:        {},
		aba.ClassIDSpriteAtlas:   {},
		aba.ClassIDAnimationClip: {},
		aba.ClassIDMaterial:      {},
		aba.ClassIDAudioClip:     {},
		aba.ClassIDMonoBehaviour: {},
	}
	slots := make(chan struct{}, parallelSlots)
	var slotAcquireMu sync.Mutex
	acquireSlots := func(count int) {
		slotAcquireMu.Lock()
		defer slotAcquireMu.Unlock()
		for slot := 0; slot < count; slot++ {
			slots <- struct{}{}
		}
	}
	releaseSlots := func(count int) {
		for slot := 0; slot < count; slot++ {
			<-slots
		}
	}
	t.Cleanup(func() {
		readable := readableCount.Load()
		encrypted := encryptedCount.Load()
		if readable == 0 {
			t.Fatal("no readable ABA samples were tested")
		}
		for classID, count := range typeCounts {
			if count.Load() == 0 {
				t.Errorf("full round trip did not cover required ClassID %d", classID)
			}
		}
		t.Logf("full round trip completed for %d readable ABA files; %d encrypted files were rejected", readable, encrypted)
	})
	for _, sample := range samples {
		sample := sample
		t.Run(filepath.Base(sample), func(t *testing.T) {
			sampleInfo, err := os.Stat(sample)
			if err != nil {
				t.Fatalf("stat source ABA: %v", err)
			}
			slotCount := canonicalRoundTripSlotCount(sampleInfo.Size())
			t.Parallel()
			acquireSlots(slotCount)
			defer releaseSlots(slotCount)

			_, sourceFile, err := (&AbaService{}).ReadAba(sample)
			if err != nil {
				if strings.Contains(strings.ToLower(err.Error()), "encrypted") {
					encryptedCount.Add(1)
					t.Skipf("encrypted ABA rejected as expected: %v", err)
				}
				t.Fatalf("read source ABA: %v", err)
			}
			if err := sourceFile.Close(); err != nil {
				t.Fatal(err)
			}
			readableCount.Add(1)

			work := t.TempDir()
			first := filepath.Join(work, "first")
			if err := (&AbaService{}).UnpackAba(sample, first); err != nil {
				t.Fatalf("first unpack: %v", err)
			}
			firstFiles := validateCanonicalDirectoryHeaders(t, first, typeCounts)
			if len(firstFiles) == 0 {
				t.Fatal("pure directory is empty")
			}
			assertPureDirectoryFileSet(t, firstFiles)

			if err := (&PackService{}).packToAbaAndCt(first, "roundtrip", false); err != nil {
				t.Fatalf("pack: %v", err)
			}
			abaPath := filepath.Join(work, "roundtrip.aba")
			ctPath := filepath.Join(work, "roundtrip.ct")
			verifyCanonicalAbaAndCatalog(t, abaPath, ctPath)
			verifyPackedAbaMatchesDirectory(t, first, firstFiles, abaPath)
			if err := os.RemoveAll(first); err != nil {
				t.Fatalf("remove first unpack directory: %v", err)
			}

			second := filepath.Join(work, "second")
			if err := (&AbaService{}).UnpackAba(abaPath, second); err != nil {
				t.Fatalf("second unpack: %v", err)
			}
			secondFiles := validateCanonicalDirectoryHeaders(t, second, nil)
			assertCanonicalDirectoryPathsEqual(t, firstFiles, secondFiles)
		})
	}
}

func canonicalRoundTripSlotCount(fileSize int64) int {
	switch {
	case fileSize >= 384<<20:
		return 5
	case fileSize >= 96<<20:
		return 2
	default:
		return 1
	}
}

func TestCanonicalAbaLegacyVersionFamiliesPureDirectoryRoundTrip(t *testing.T) {
	tests := []struct {
		name           string
		file           string
		unityVersion   string
		abaVersion     uint32
		targetPlatform uint32
	}{
		{name: "Unity 2020.2.4", file: "parts_bcc2_gp003.aba", unityVersion: "2020.2.4f1", abaVersion: 7, targetPlatform: 19},
		{name: "Unity 2020.2.6", file: "motion.aba", unityVersion: "2020.2.6f1", abaVersion: 7, targetPlatform: 19},
		{name: "Unity 2020.2.6 audio stream", file: "sound.aba", unityVersion: "2020.2.6f1", abaVersion: 7, targetPlatform: 19},
		{name: "Unity 2021.3.3", file: "parts_personal002.aba", unityVersion: "2021.3.3f1", abaVersion: 7, targetPlatform: 19},
		{name: "Unity 2021.3.6 target 5", file: "cm3d2_megane002.aba", unityVersion: "2021.3.6f1", abaVersion: 7, targetPlatform: 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sample := filepath.Join("..", "..", "testdata", "aba", tt.file)
			if _, err := os.Stat(sample); err != nil {
				t.Skipf("sample not available: %v", err)
			}
			verifyCanonicalSourceContract(t, sample, tt.abaVersion, tt.unityVersion, tt.targetPlatform)

			work := t.TempDir()
			first := filepath.Join(work, "first")
			if err := (&AbaService{}).UnpackAba(sample, first); err != nil {
				t.Fatalf("first unpack: %v", err)
			}
			firstFiles := validateCanonicalDirectoryFiles(t, first, nil)
			if len(firstFiles) == 0 {
				t.Fatal("pure directory is empty")
			}
			if err := (&PackService{}).PackToAbaAndCt(first, "legacy_roundtrip"); err != nil {
				t.Fatalf("pack: %v", err)
			}
			abaPath := filepath.Join(work, "legacy_roundtrip.aba")
			ctPath := filepath.Join(work, "legacy_roundtrip.ct")
			verifyCanonicalAbaAndCatalog(t, abaPath, ctPath)
			verifyPackedAbaMatchesDirectory(t, first, firstFiles, abaPath)

			second := filepath.Join(work, "second")
			if err := (&AbaService{}).UnpackAba(abaPath, second); err != nil {
				t.Fatalf("second unpack: %v", err)
			}
			secondFiles := validateCanonicalDirectoryFiles(t, second, nil)
			assertCanonicalDirectoryPathsEqual(t, firstFiles, secondFiles)
		})
	}
}

func TestCanonicalAbaKnownVersionSamplesBuildPureDirectoryPlan(t *testing.T) {
	tests := []struct {
		name           string
		file           string
		unityVersion   string
		abaVersion     uint32
		targetPlatform uint32
	}{
		{name: "2020.2.4f1", file: "parts_bcc2_gp003.aba", unityVersion: "2020.2.4f1", abaVersion: 7, targetPlatform: 19},
		{name: "2020.2.6f1", file: "motion.aba", unityVersion: "2020.2.6f1", abaVersion: 7, targetPlatform: 19},
		{name: "2020.3.2f1", file: "bg.aba", unityVersion: "2020.3.2f1", abaVersion: 7, targetPlatform: 19},
		{name: "2020.3.3f1", file: "parts_dlc391_gp003.aba", unityVersion: "2020.3.3f1", abaVersion: 7, targetPlatform: 19},
		{name: "2020.3.4f1", file: "parts_dlc410_gp003.aba", unityVersion: "2020.3.4f1", abaVersion: 7, targetPlatform: 19},
		{name: "2020.3.8f1", file: "parts_dlc391_gp003_2.aba", unityVersion: "2020.3.8f1", abaVersion: 7, targetPlatform: 19},
		{name: "2020.3.22f1", file: "parts_yomeidokanteidan3_elega_gp003.aba", unityVersion: "2020.3.22f1", abaVersion: 7, targetPlatform: 19},
		{name: "2020.3.27f1", file: "parts_dlc266_2.aba", unityVersion: "2020.3.27f1", abaVersion: 7, targetPlatform: 19},
		{name: "2020.3.29f1", file: "system_cres2.aba", unityVersion: "2020.3.29f1", abaVersion: 7, targetPlatform: 19},
		{name: "2021.3.2f1", file: "parts_charaevent_anesan.aba", unityVersion: "2021.3.2f1", abaVersion: 7, targetPlatform: 19},
		{name: "2021.3.3f1", file: "parts_personal004.aba", unityVersion: "2021.3.3f1", abaVersion: 7, targetPlatform: 19},
		{name: "2021.3.6f1", file: "nt008_舌.aba", unityVersion: "2021.3.6f1", abaVersion: 7, targetPlatform: 5},
		{name: "2021.3.8f1", file: "nt008_でら強頬線チーク.aba", unityVersion: "2021.3.8f1", abaVersion: 7, targetPlatform: 5},
		{name: "2022.3.35f1", file: "parts_cafesp001_2.aba", unityVersion: "2022.3.35f1", abaVersion: 8, targetPlatform: 19},
		{name: "2022.3.62f2", file: "parts_dlc410_gp003_2.aba", unityVersion: "2022.3.62f2", abaVersion: 8, targetPlatform: 19},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sample := filepath.Join("..", "..", "testdata", "aba", tt.file)
			if _, err := os.Stat(sample); err != nil {
				t.Skipf("sample not available: %v", err)
			}
			verifyCanonicalSourceContract(t, sample, tt.abaVersion, tt.unityVersion, tt.targetPlatform)
			bundle, file, err := (&AbaService{}).ReadAba(sample)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			if _, err := buildCanonicalUnpackContext(bundle); err != nil {
				t.Fatalf("build pure-directory plan: %v", err)
			}
		})
	}
}

func TestCanonicalKCESBuiltInExternalFileIDs(t *testing.T) {
	tests := []struct {
		name     string
		external aba.ExternalFile
		wantID   int32
		matched  bool
	}{
		{name: "builtin observed alias", external: aba.ExternalFile{Guid: [16]byte{8: 0x0f}, PathName: "Resources/unity_builtin_extra"}, wantID: 1, matched: true},
		{name: "builtin spaced alias", external: aba.ExternalFile{Guid: [16]byte{8: 0x0f}, PathName: "resources/unity builtin extra"}, wantID: 1, matched: true},
		{name: "default resource alias", external: aba.ExternalFile{Guid: [16]byte{8: 0x0e}, PathName: "Library/unity default resources"}, wantID: 2, matched: true},
		{name: "ordinary dependency", external: aba.ExternalFile{PathName: "archive:/shared.assets"}, matched: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, matched, err := canonicalKCESBuiltInExternalFileID(tt.external)
			if err != nil {
				t.Fatal(err)
			}
			if gotID != tt.wantID || matched != tt.matched {
				t.Fatalf("got ID=%d matched=%v, want ID=%d matched=%v", gotID, matched, tt.wantID, tt.matched)
			}
		})
	}
}

func TestCanonicalAbaSystemRoundTripPreservesMonoBehaviourScriptTypes(t *testing.T) {
	sample := filepath.Join("..", "..", "testdata", "aba", "system.aba")
	if _, err := os.Stat(sample); err != nil {
		t.Skipf("sample not available: %v", err)
	}
	work := t.TempDir()
	first := filepath.Join(work, "first")
	if err := (&AbaService{}).UnpackAba(sample, first); err != nil {
		t.Fatalf("first unpack: %v", err)
	}
	firstFiles := validateCanonicalDirectoryFiles(t, first, nil)
	if err := (&PackService{}).PackToAbaAndCt(first, "system_roundtrip"); err != nil {
		t.Fatalf("pack: %v", err)
	}
	abaPath := filepath.Join(work, "system_roundtrip.aba")
	ctPath := filepath.Join(work, "system_roundtrip.ct")
	verifyCanonicalAbaAndCatalog(t, abaPath, ctPath)
	verifyPackedAbaMatchesDirectory(t, first, firstFiles, abaPath)
	verifyMonoBehaviourScriptTypes(t, abaPath)

	second := filepath.Join(work, "second")
	if err := (&AbaService{}).UnpackAba(abaPath, second); err != nil {
		t.Fatalf("second unpack: %v", err)
	}
	secondFiles := validateCanonicalDirectoryFiles(t, second, nil)
	assertCanonicalDirectoryPathsEqual(t, firstFiles, secondFiles)
}

func verifyMonoBehaviourScriptTypes(t *testing.T, abaPath string) {
	t.Helper()
	f, err := os.Open(abaPath)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := aba.ReadAba(f)
	if err != nil {
		f.Close()
		t.Fatal(err)
	}
	defer f.Close()
	af := readCanonicalTestAssetsFile(t, bundle, 0)
	if len(af.Metadata.ScriptTypes) != 2 {
		t.Fatalf("ScriptTypes count = %d, want 2", len(af.Metadata.ScriptTypes))
	}
	classes := make(map[int64]int32, len(af.Metadata.AssetInfos))
	for _, info := range af.Metadata.AssetInfos {
		classes[info.PathId] = info.TypeId
	}
	monoTypeIndexes := map[int16]bool{}
	monoCount := int64(0)
	for infoIndex := range af.Metadata.AssetInfos {
		info := &af.Metadata.AssetInfos[infoIndex]
		if info.TypeId != aba.ClassIDMonoBehaviour {
			continue
		}
		monoCount++
		serializedType := af.Metadata.TypeTreeTypes[info.TypeIdOrIndex]
		if serializedType.ScriptTypeIndex < 0 || int64(serializedType.ScriptTypeIndex) >= int64(len(af.Metadata.ScriptTypes)) {
			t.Fatalf("MonoBehaviour PathID %d has invalid ScriptTypeIndex %d", info.PathId, serializedType.ScriptTypeIndex)
		}
		monoTypeIndexes[serializedType.ScriptTypeIndex] = true
		identifier := af.Metadata.ScriptTypes[serializedType.ScriptTypeIndex]
		if classes[identifier.LocalIdentifierInFile] != aba.ClassIDMonoScript {
			t.Fatalf("ScriptTypes[%d] targets class ID %d", serializedType.ScriptTypeIndex, classes[identifier.LocalIdentifierInFile])
		}
		data, err := af.GetAssetData(info)
		if err != nil {
			t.Fatal(err)
		}
		if len(data) < 28 || int32(binary.LittleEndian.Uint32(data[16:20])) != 0 || int64(binary.LittleEndian.Uint64(data[20:28])) != identifier.LocalIdentifierInFile {
			t.Fatalf("MonoBehaviour PathID %d m_Script does not match ScriptTypes[%d]", info.PathId, serializedType.ScriptTypeIndex)
		}
	}
	if monoCount != 2 || len(monoTypeIndexes) != 2 {
		t.Fatalf("MonoBehaviour objects=%d distinct script types=%d, want 2 and 2", monoCount, len(monoTypeIndexes))
	}
}

func validateCanonicalDirectoryFiles(t *testing.T, root string, classCounts map[int32]*atomic.Int64) []string {
	t.Helper()
	var result []string
	err := filepath.Walk(root, func(name string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		result = append(result, rel)
		kind, native := unityRawKindForPackPath(rel)
		if !native {
			return nil
		}
		classID, ok := unityRawClassIDForKind(kind)
		if !ok || !isNativeUnityObjectPackPath(rel, kind) {
			return &canonicalDirectoryValidationError{path: rel, message: "path does not use the native object naming contract"}
		}
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		object, err := aba.ReadNativeUnityObject(data)
		if err != nil {
			return &canonicalDirectoryValidationError{path: rel, message: "read native Unity object: " + err.Error()}
		}
		if object.ClassID != classID {
			return &canonicalDirectoryValidationError{path: rel, message: "native object ClassID does not match its type directory"}
		}
		rootValue, trailing, err := object.DecodeValueAndTrailingData()
		if err != nil {
			return &canonicalDirectoryValidationError{path: rel, message: "decode native Unity object: " + err.Error()}
		}
		encodedPayload, err := object.EncodeValueWithTrailingData(rootValue, trailing)
		if err != nil {
			return &canonicalDirectoryValidationError{path: rel, message: "re-encode native Unity object value: " + err.Error()}
		}
		reencoded := *object
		reencoded.Data = encodedPayload
		reencodedFile, err := aba.EncodeNativeUnityObject(&reencoded)
		if err != nil {
			return &canonicalDirectoryValidationError{path: rel, message: "re-encode native Unity object file: " + err.Error()}
		}
		if _, err := aba.ReadNativeUnityObject(reencodedFile); err != nil {
			return &canonicalDirectoryValidationError{path: rel, message: "read re-encoded native Unity object file: " + err.Error()}
		}
		if counter := classCounts[object.ClassID]; counter != nil {
			counter.Add(1)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %q: %v", root, err)
	}
	sort.Strings(result)
	return result
}

func validateCanonicalDirectoryHeaders(t *testing.T, root string, classCounts map[int32]*atomic.Int64) []string {
	t.Helper()
	var result []string
	err := filepath.Walk(root, func(name string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		result = append(result, rel)
		kind, native := unityRawKindForPackPath(rel)
		if !native {
			return nil
		}
		classID, ok := unityRawClassIDForKind(kind)
		if !ok || !isNativeUnityObjectPackPath(rel, kind) {
			return &canonicalDirectoryValidationError{path: rel, message: "path does not use the native object naming contract"}
		}
		file, err := os.Open(name)
		if err != nil {
			return err
		}
		header, readErr := aba.ReadNativeUnityObjectHeader(file, info.Size())
		closeErr := file.Close()
		if readErr != nil {
			return &canonicalDirectoryValidationError{path: rel, message: "read native Unity object header: " + readErr.Error()}
		}
		if closeErr != nil {
			return closeErr
		}
		if header.ClassID != classID {
			return &canonicalDirectoryValidationError{path: rel, message: "native object ClassID does not match its type directory"}
		}
		if counter := classCounts[header.ClassID]; counter != nil {
			counter.Add(1)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %q: %v", root, err)
	}
	sort.Strings(result)
	return result
}

type canonicalDirectoryValidationError struct {
	path    string
	message string
}

func (err *canonicalDirectoryValidationError) Error() string {
	return err.path + ": " + err.message
}

func assertCanonicalDirectoryPathsEqual(t *testing.T, want []string, got []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("file count changed from %d to %d", len(want), len(got))
	}
	for fileIndex := range want {
		if got[fileIndex] != want[fileIndex] {
			t.Fatalf("file[%d] changed from %q to %q", fileIndex, want[fileIndex], got[fileIndex])
		}
	}
}

func assertPureDirectoryFileSet(t *testing.T, files []string) {
	t.Helper()
	for _, name := range files {
		lower := strings.ToLower(name)
		if strings.HasSuffix(lower, ".meta.json") || strings.HasSuffix(lower, ".typetree.json") ||
			strings.HasSuffix(lower, ".ress") || strings.HasSuffix(lower, ".resource") ||
			strings.HasSuffix(lower, ".resources") || strings.HasSuffix(lower, ".png") {
			t.Fatalf("pure directory contains derived or stream sidecar %q", name)
		}
	}
}

func verifyPackedAbaMatchesDirectory(t *testing.T, root string, relativePaths []string, abaPath string) {
	t.Helper()
	f, err := os.Open(abaPath)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := aba.ReadAba(f)
	if err != nil {
		f.Close()
		t.Fatal(err)
	}
	defer f.Close()
	af := readCanonicalTestAssetsFile(t, bundle, 0)
	pathIDs, err := buildCanonicalPathIDs(relativePaths)
	if err != nil {
		t.Fatal(err)
	}

	for _, relativePath := range relativePaths {
		canonicalPath, err := canonicalAssetPathForID(relativePath)
		if err != nil {
			t.Fatal(err)
		}
		pathID := pathIDs[canonicalPath]
		info := af.GetAssetInfoByPathID(pathID)
		if info == nil {
			t.Fatalf("packed ABA has no object for %q at PathID %d", relativePath, pathID)
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relativePath)))
		if err != nil {
			t.Fatal(err)
		}
		kind, native := unityRawKindForPackPath(relativePath)
		if !native {
			if info.TypeId != aba.ClassIDTextAsset {
				t.Fatalf("packed %q has ClassID %d instead of TextAsset", relativePath, info.TypeId)
			}
			name, script, err := af.GetTextAssetData(info)
			if err != nil {
				t.Fatalf("decode packed TextAsset %q: %v", relativePath, err)
			}
			if name != inferAssetNameForPack(relativePath) || !bytes.Equal(script, data) {
				t.Fatalf("packed TextAsset %q changed its name or script payload", relativePath)
			}
			continue
		}

		classID, ok := unityRawClassIDForKind(kind)
		if !ok || info.TypeId != classID {
			t.Fatalf("packed %q has ClassID %d, want %d", relativePath, info.TypeId, classID)
		}
		directoryObject, err := aba.ReadNativeUnityObject(data)
		if err != nil {
			t.Fatalf("read directory native object %q: %v", relativePath, err)
		}
		tree, err := af.AssetTypeTree(info)
		if err != nil {
			t.Fatalf("read packed TypeTree for %q: %v", relativePath, err)
		}
		packedData, err := af.GetAssetData(info)
		if err != nil {
			t.Fatalf("read packed object %q: %v", relativePath, err)
		}
		packedObject := &aba.NativeUnityObject{
			ClassID:   info.TypeId,
			BigEndian: af.Header.Endianness,
			TypeTree:  tree,
			Data:      packedData,
		}
		directoryValue, _, err := directoryObject.DecodeValueAndTrailingData()
		if err != nil {
			t.Fatalf("decode directory object %q: %v", relativePath, err)
		}
		packedValue, _, err := packedObject.DecodeValueAndTrailingData()
		if err != nil {
			t.Fatalf("decode packed object %q: %v", relativePath, err)
		}
		assertTypeTreeValuesEquivalent(t, relativePath, directoryValue, packedValue)
		if classID == aba.ClassIDAudioClip {
			directoryAudio, err := directoryObject.AudioData()
			if err != nil {
				t.Fatalf("read directory AudioClip %q: %v", relativePath, err)
			}
			packedAudio, err := packedObject.AudioData()
			if err != nil {
				t.Fatalf("read packed AudioClip %q: %v", relativePath, err)
			}
			if !bytes.Equal(directoryAudio, packedAudio) {
				t.Fatalf("packed AudioClip %q changed its encoded audio payload", relativePath)
			}
		}
	}
}

func assertTypeTreeValuesEquivalent(t *testing.T, path string, want *aba.TypeTreeValue, got *aba.TypeTreeValue) {
	t.Helper()
	if want == nil || got == nil {
		if want != got {
			t.Fatalf("%s TypeTree nil mismatch", path)
		}
		return
	}
	if want.TypeName != got.TypeName || want.Name != got.Name || len(want.Children) != len(got.Children) || !typeTreeScalarEqual(want.Value, got.Value) {
		t.Fatalf("%s TypeTree field %s/%s changed during packing", path, want.TypeName, want.Name)
	}
	for childIndex := range want.Children {
		assertTypeTreeValuesEquivalent(t, path, want.Children[childIndex], got.Children[childIndex])
	}
}

func typeTreeScalarEqual(want interface{}, got interface{}) bool {
	switch value := want.(type) {
	case float32:
		other, ok := got.(float32)
		return ok && math.Float32bits(value) == math.Float32bits(other)
	case float64:
		other, ok := got.(float64)
		return ok && math.Float64bits(value) == math.Float64bits(other)
	default:
		return reflect.DeepEqual(want, got)
	}
}

func verifyCanonicalSourceContract(t *testing.T, abaPath string, abaVersion uint32, unityVersion string, targetPlatform uint32) {
	t.Helper()
	f, err := os.Open(abaPath)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := aba.ReadAba(f)
	if err != nil {
		f.Close()
		t.Fatal(err)
	}
	defer f.Close()
	if bundle.Header.Version != abaVersion || bundle.Header.EngineVersion != unityVersion {
		t.Fatalf("source ABA got version=%d engine=%q, want version=%d engine=%q", bundle.Header.Version, bundle.Header.EngineVersion, abaVersion, unityVersion)
	}
	for directoryIndex, entry := range bundle.BlockInfo.DirectoryInfos {
		if !entry.IsSerialized() {
			continue
		}
		af := readCanonicalTestAssetsFile(t, bundle, int64(directoryIndex))
		if af.Header.Version != supportedSerializedFileVersion || af.Metadata.UnityVersion != unityVersion || af.Metadata.TargetPlatform != targetPlatform || !af.Metadata.TypeTreeEnabled {
			t.Fatalf("source SerializedFile got version=%d Unity=%q target=%d TypeTree=%t", af.Header.Version, af.Metadata.UnityVersion, af.Metadata.TargetPlatform, af.Metadata.TypeTreeEnabled)
		}
		return
	}
	t.Fatal("source ABA contains no SerializedFile")
}

func verifyCanonicalAbaAndCatalog(t *testing.T, abaPath string, ctPath string) {
	t.Helper()
	f, err := os.Open(abaPath)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := aba.ReadAba(f)
	if err != nil {
		f.Close()
		t.Fatal(err)
	}
	defer f.Close()
	if bundle.Header.Version != defaultKCESAbaVersion || bundle.Header.GenerationVersion != defaultKCESGenerationVersion || bundle.Header.EngineVersion != defaultKCESUnityVersion {
		t.Fatalf("ABA header got version=%d generation=%q engine=%q", bundle.Header.Version, bundle.Header.GenerationVersion, bundle.Header.EngineVersion)
	}
	if len(bundle.BlockInfo.DirectoryInfos) != 1 || !bundle.BlockInfo.DirectoryInfos[0].IsSerialized() {
		t.Fatalf("ABA directory entries = %+v", bundle.BlockInfo.DirectoryInfos)
	}
	af := readCanonicalTestAssetsFile(t, bundle, 0)
	if af.Header.Version != 22 || af.Metadata.UnityVersion != defaultKCESUnityVersion || af.Metadata.TargetPlatform != defaultKCESTargetPlatform {
		t.Fatalf("SerializedFile contract got version=%d unity=%q platform=%d", af.Header.Version, af.Metadata.UnityVersion, af.Metadata.TargetPlatform)
	}
	wantExternalFiles := canonicalKCESExternalFiles()
	if len(af.Metadata.ExternalFiles) != len(wantExternalFiles) {
		t.Fatalf("SerializedFile external files = %+v, want %+v", af.Metadata.ExternalFiles, wantExternalFiles)
	}
	for externalIndex := range wantExternalFiles {
		if af.Metadata.ExternalFiles[externalIndex] != wantExternalFiles[externalIndex] {
			t.Fatalf("SerializedFile external file[%d] = %+v, want %+v", externalIndex, af.Metadata.ExternalFiles[externalIndex], wantExternalFiles[externalIndex])
		}
	}
	container, err := af.GetAssetBundleContainerMap()
	if err != nil {
		t.Fatal(err)
	}
	loadNames := make([]string, 0, len(container))
	for _, name := range container {
		loadNames = append(loadNames, strings.ToLower(name))
	}
	sort.Strings(loadNames)

	cf, err := os.Open(ctPath)
	if err != nil {
		t.Fatal(err)
	}
	table, err := ct.ReadContentTable(cf)
	cf.Close()
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := ct.DecodeCatalogFromCt(table)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range catalog.Items {
		if item == nil || item.Name == nil {
			t.Fatal("catalog contains a nil item")
		}
		name := strings.ToLower(*item.Name)
		index := sort.SearchStrings(loadNames, name)
		if index >= len(loadNames) || loadNames[index] != name {
			t.Fatalf("catalog item %q is absent from AssetBundle m_Container", *item.Name)
		}
	}
}

func readCanonicalTestAssetsFile(t *testing.T, bundle *aba.Aba, directoryIndex int64) *aba.AssetsFile {
	t.Helper()
	if bundle == nil || directoryIndex < 0 || directoryIndex >= int64(len(bundle.BlockInfo.DirectoryInfos)) {
		t.Fatalf("invalid ABA directory index %d", directoryIndex)
	}
	entry := bundle.BlockInfo.DirectoryInfos[directoryIndex]
	af, err := aba.ReadAssetsFileRange(entry.DecompressedSize, func(offset int64, size int64) ([]byte, error) {
		return bundle.GetFileDataRange(directoryIndex, offset, size)
	})
	if err != nil {
		t.Fatalf("read SerializedFile %q: %v", entry.Name, err)
	}
	return af
}
