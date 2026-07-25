package aba

import (
	"bytes"
	"fmt"
	"testing"
)

func TestReadAssetsFileRangeLoadsMetadataBeforeObjectData(t *testing.T) {
	w := NewSerializedFileWriter("2022.3.35f1")
	pathID := w.AddTextAsset("sample.txt", []byte("payload"))
	var serialized bytes.Buffer
	if err := w.Write(&serialized); err != nil {
		t.Fatalf("Write: %v", err)
	}
	data := serialized.Bytes()
	var ranges [][2]int64
	resolver := func(offset int64, size int64) ([]byte, error) {
		end := offset + size
		if offset < 0 || size < 0 || end < offset || end > int64(len(data)) {
			return nil, fmt.Errorf("range [%d,%d) is invalid", offset, end)
		}
		ranges = append(ranges, [2]int64{offset, end})
		return data[offset:end], nil
	}
	af, err := ReadAssetsFileRange(int64(len(data)), resolver)
	if err != nil {
		t.Fatalf("ReadAssetsFileRange: %v", err)
	}
	if af.Data != nil {
		t.Fatal("range-backed AssetsFile unexpectedly retained complete data")
	}
	if len(ranges) != 2 || ranges[0] != [2]int64{0, 48} || ranges[1][0] != 48 || ranges[1][1] > af.Header.DataOffset {
		t.Fatalf("initial ranges = %v", ranges)
	}
	info := af.GetAssetInfoByPathID(pathID)
	if info == nil {
		t.Fatalf("PathID %d not found", pathID)
	}
	got, err := af.GetAssetData(info)
	if err != nil {
		t.Fatalf("GetAssetData: %v", err)
	}
	want, err := ReadAssetsFile(data)
	if err != nil {
		t.Fatalf("ReadAssetsFile: %v", err)
	}
	wantData, err := want.GetAssetData(want.GetAssetInfoByPathID(pathID))
	if err != nil {
		t.Fatalf("whole-file GetAssetData: %v", err)
	}
	if !bytes.Equal(got, wantData) {
		t.Fatalf("object data differs: got % x, want % x", got, wantData)
	}
	if len(ranges) != 3 || ranges[2] != [2]int64{af.Header.DataOffset + info.ByteOffset, af.Header.DataOffset + info.ByteOffset + int64(info.ByteSize)} {
		t.Fatalf("ranges after object read = %v", ranges)
	}
}

func TestReadAssetsFileRangeRejectsIncorrectResolverLength(t *testing.T) {
	_, err := ReadAssetsFileRange(48, func(offset int64, size int64) ([]byte, error) {
		return make([]byte, size-1), nil
	})
	if err == nil {
		t.Fatal("ReadAssetsFileRange accepted a short range")
	}
}
