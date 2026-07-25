package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
)

func TestParseSerializedPrefixV22(t *testing.T) {
	data := make([]byte, 256)
	binary.BigEndian.PutUint32(data[8:12], 22)
	data[16] = 0
	binary.BigEndian.PutUint32(data[20:24], 128)
	binary.BigEndian.PutUint64(data[24:32], 4096)
	binary.BigEndian.PutUint64(data[32:40], 256)
	version := "2021.3.6f1"
	copy(data[48:], version)
	versionEnd := int64(48 + len(version))
	data[versionEnd] = 0
	binary.LittleEndian.PutUint32(data[versionEnd+1:versionEnd+5], 19)
	data[versionEnd+5] = 1

	summary, err := parseSerializedPrefix(data)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Version != 22 || summary.UnityVersion != version || summary.TargetPlatform != 19 || !summary.TypeTreeEnabled {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if summary.FileSize != 4096 || summary.MetadataSize != 128 || summary.DataOffset != 256 || summary.Endianness != "little" {
		t.Fatalf("unexpected header summary: %+v", summary)
	}
}

func TestParseSerializedPrefixRejectsTruncatedUnityVersion(t *testing.T) {
	data := make([]byte, 64)
	binary.BigEndian.PutUint32(data[8:12], 22)
	binary.BigEndian.PutUint32(data[20:24], 128)
	binary.BigEndian.PutUint64(data[24:32], 4096)
	binary.BigEndian.PutUint64(data[32:40], 256)
	for offset := int64(48); offset < int64(len(data)); offset++ {
		data[offset] = 'x'
	}
	if _, err := parseSerializedPrefix(data); err == nil || !strings.Contains(err.Error(), "not terminated") {
		t.Fatalf("parseSerializedPrefix error = %v", err)
	}
}

func TestScanAbaReadsKnownUnity2022Sample(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "aba", "csv.aba")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("sample not available: %v", err)
	}
	rows := scanAba(path)
	if len(rows) != 1 {
		t.Fatalf("scan rows = %d, want 1: %+v", len(rows), rows)
	}
	row := rows[0]
	if row.Status != "ok" || row.UnityFSVersion != 8 || row.EngineVersion != "2022.3.62f2" {
		t.Fatalf("unexpected ABA summary: %+v", row)
	}
	if row.SerializedFileVersion != 22 || row.SerializedUnityVersion != "2022.3.62f2" {
		t.Fatalf("unexpected SerializedFile summary: %+v", row)
	}
}

func TestParseOptionalClassID(t *testing.T) {
	if classID, err := parseOptionalClassID(""); err != nil || classID != nil {
		t.Fatalf("empty class ID = %v, %v", classID, err)
	}
	classID, err := parseOptionalClassID("74")
	if err != nil || classID == nil || *classID != aba.ClassIDAnimationClip {
		t.Fatalf("AnimationClip class ID = %v, %v", classID, err)
	}
	for _, value := range []string{"invalid", "2147483648"} {
		if _, err := parseOptionalClassID(value); err == nil {
			t.Fatalf("class ID %q unexpectedly succeeded", value)
		}
	}
}

func TestScanAbaFindsAnimationClips(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "aba", "motion.aba")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("sample not available: %v", err)
	}
	classID := aba.ClassIDAnimationClip
	rows := scanAbaWithOptions(path, scanOptions{FindClassID: &classID})
	var count int64
	var names int64
	for _, row := range rows {
		if row.Status != "ok" {
			t.Fatalf("object scan failed: %+v", row)
		}
		if !row.ClassScanEnabled || row.MatchedClassID != classID {
			t.Fatalf("class scan metadata is missing: %+v", row)
		}
		count += row.MatchedObjectCount
		names += int64(len(row.MatchedObjectNames))
	}
	if count == 0 || names != count {
		t.Fatalf("AnimationClip matches = %d objects and %d names", count, names)
	}
}
