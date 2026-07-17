package aba

import (
	"bytes"
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

func TestReadAssetsFileRejectsNegativeMetadataCountsWithoutPanic(t *testing.T) {
	data := minimalSerializedFileForRobustness([]byte{
		'x', 0,
		0, 0, 0, 0, // target platform
		0,                      // type tree disabled
		0xff, 0xff, 0xff, 0xff, // type count = -1
	})

	assertAssetsErrorWithoutPanic(t, func() error {
		_, err := ReadAssetsFile(data)
		return err
	}, "negative type count")
}

func TestReadAssetsFileRejectsNegativeTypeIndexWithoutPanic(t *testing.T) {
	var metadata bytes.Buffer
	metadata.WriteString("x\x00")
	_ = binary.Write(&metadata, binary.LittleEndian, uint32(0))
	metadata.WriteByte(0) // type tree disabled
	_ = binary.Write(&metadata, binary.LittleEndian, int32(1))
	_ = binary.Write(&metadata, binary.LittleEndian, int32(ClassIDTextAsset))
	metadata.WriteByte(0)
	_ = binary.Write(&metadata, binary.LittleEndian, uint16(0xffff))
	metadata.Write(make([]byte, 16))
	_ = binary.Write(&metadata, binary.LittleEndian, int32(1))
	for (20+metadata.Len())%4 != 0 {
		metadata.WriteByte(0)
	}
	_ = binary.Write(&metadata, binary.LittleEndian, int64(1))
	_ = binary.Write(&metadata, binary.LittleEndian, uint32(0))
	_ = binary.Write(&metadata, binary.LittleEndian, uint32(0))
	_ = binary.Write(&metadata, binary.LittleEndian, int32(-1))

	assertAssetsErrorWithoutPanic(t, func() error {
		_, err := ReadAssetsFile(minimalSerializedFileForRobustness(metadata.Bytes()))
		return err
	}, "type tree index -1 out of range")
}

func TestGetAssetDataRejectsOverflowAndNegativeRangesWithoutPanic(t *testing.T) {
	tests := []struct {
		name   string
		header AssetsFileHeader
		info   AssetInfo
	}{
		{
			name:   "data offset overflow",
			header: AssetsFileHeader{DataOffset: math.MaxInt64 - 1},
			info:   AssetInfo{ByteOffset: 2, ByteSize: 1},
		},
		{
			name:   "negative byte offset",
			header: AssetsFileHeader{DataOffset: 8},
			info:   AssetInfo{ByteOffset: -1, ByteSize: 1},
		},
		{
			name:   "size beyond end",
			header: AssetsFileHeader{DataOffset: 8},
			info:   AssetInfo{ByteOffset: 7, ByteSize: 2},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			af := &AssetsFile{Header: tc.header, Data: make([]byte, 16)}
			assertAssetsErrorWithoutPanic(t, func() error {
				_, err := af.GetAssetData(&tc.info)
				return err
			}, "out of bounds")
		})
	}
}

func minimalSerializedFileForRobustness(metadata []byte) []byte {
	const headerSize = 20
	data := make([]byte, headerSize, headerSize+len(metadata))
	binary.BigEndian.PutUint32(data[0:4], uint32(len(metadata)))
	binary.BigEndian.PutUint32(data[4:8], uint32(headerSize+len(metadata)))
	binary.BigEndian.PutUint32(data[8:12], 17)
	binary.BigEndian.PutUint32(data[12:16], uint32(headerSize+len(metadata)))
	return append(data, metadata...)
}

func assertAssetsErrorWithoutPanic(t *testing.T, fn func() error, want string) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("unexpected panic: %v", recovered)
		}
	}()
	err := fn()
	if err == nil {
		t.Fatal("expected an error")
	}
	if want != "" && !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q does not contain %q", err, want)
	}
}
