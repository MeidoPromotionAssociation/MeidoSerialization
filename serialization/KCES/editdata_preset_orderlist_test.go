package KCES

import (
	"bytes"
	"encoding/binary"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

func TestColorPresetOrderListHandWrittenGameWireRoundTrip(t *testing.T) {
	// ColorPresetProvider.PresetOrderList is an indexed object:
	// [version, List<string> idOrderList]. The inner MessagePack bytes below
	// are written by hand; only the game's StandardLz4BlockArray wrapper is
	// delegated to the already independently tested compressor.
	raw := []byte{
		0x92,
		0xcd, 0x03, 0xe8, // version = 1000
		0x93,
		0xa6, 'g', 'u', 'i', 'd', '-', 'a',
		0xc0,
		0xa6, 0xe6, 0x97, 0xa5, 0xe6, 0x9c, 0xac, // "日本"
	}
	// The game's CompressionMinLength is 64, so this 20-byte value is emitted
	// as raw MessagePack despite the StandardLz4BlockArray option.
	wire := append([]byte(nil), raw...)

	decoded, err := DecodeColorPresetOrderList(wire)
	if err != nil {
		t.Fatalf("DecodeColorPresetOrderList() error = %v", err)
	}
	want := &ColorPresetOrderList{
		Version:     ColorPresetOrderListVersion,
		IDOrderList: []*string{colorPresetOrderString("guid-a"), nil, colorPresetOrderString("日本")},
	}
	if !reflect.DeepEqual(decoded, want) {
		t.Fatalf("decoded = %#v, want %#v", decoded, want)
	}

	encoded, err := EncodeColorPresetOrderList(decoded)
	if err != nil {
		t.Fatalf("EncodeColorPresetOrderList() error = %v", err)
	}
	if !bytes.Equal(encoded, raw) {
		t.Fatalf("sub-64-byte canonical wire = %x, want uncompressed hand-written wire %x", encoded, raw)
	}
}

func TestColorPresetOrderListMatchesGameCompressionThresholdAndHasNoLengthPrefix(t *testing.T) {
	tests := []struct {
		name           string
		idLength       int
		wantRawLength  int
		wantCompressed bool
	}{
		{name: "63 bytes remains raw", idLength: 56, wantRawLength: 63, wantCompressed: false},
		{name: "64 bytes is Lz4BlockArray", idLength: 57, wantRawLength: 64, wantCompressed: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			id := strings.Repeat("x", test.idLength)
			// Hand-written [1000,[id]]. idLength >= 32 selects str8.
			raw := []byte{0x92, 0xcd, 0x03, 0xe8, 0x91, 0xd9, byte(test.idLength)}
			raw = append(raw, id...)
			if len(raw) != test.wantRawLength {
				t.Fatalf("test raw length = %d, want %d", len(raw), test.wantRawLength)
			}

			encoded, err := EncodeColorPresetOrderList(&ColorPresetOrderList{Version: ColorPresetOrderListVersion, IDOrderList: []*string{&id}})
			if err != nil {
				t.Fatalf("EncodeColorPresetOrderList() error = %v", err)
			}
			if !test.wantCompressed {
				if !bytes.Equal(encoded, raw) {
					t.Fatalf("63-byte output = %x, want raw MessagePack %x", encoded, raw)
				}
				return
			}
			if bytes.Equal(encoded, raw) {
				t.Fatal("64-byte output remained raw; game starts Lz4BlockArray compression at 64 bytes")
			}
			// One block whose uncompressed size (64) takes one MessagePack byte:
			// array(2), fixext1, extension type 98. This starts at byte zero,
			// proving there is no BinaryWriter int32 length prefix.
			if len(encoded) < 3 || encoded[0] != 0x92 || encoded[1] != 0xd4 || encoded[2] != byte(ct.Lz4ArrayType) {
				t.Fatalf("64-byte output starts %x, want direct array(2)/fixext1/ext(98) Lz4BlockArray", encoded)
			}
			decodedRaw, err := ct.DecompressLz4BlockArray(encoded)
			if err != nil {
				t.Fatalf("DecompressLz4BlockArray() error = %v", err)
			}
			if !bytes.Equal(decodedRaw, raw) {
				t.Fatalf("decompressed wire = %x, want %x", decodedRaw, raw)
			}
		})
	}
}

func TestColorPresetOrderListVersionPreservationAndIndexedCompatibility(t *testing.T) {
	t.Run("empty short array keeps zero serialized value", func(t *testing.T) {
		decoded := colorPresetOrderListDecodeRaw(t, []byte{0x90})
		if decoded.Version != 0 || decoded.IDOrderList != nil {
			t.Fatalf("decoded short object = %#v, want version=0 and nil list", decoded)
		}
	})

	t.Run("legacy version is preserved because Migrate is empty", func(t *testing.T) {
		decoded := colorPresetOrderListDecodeRaw(t, []byte{0x91, 0xcd, 0x03, 0xe7}) // [999]
		if decoded.Version != 999 || decoded.IDOrderList != nil {
			t.Fatalf("decoded legacy object = %#v, want version=999 and nil list", decoded)
		}
	})

	t.Run("future version and slots are preserved", func(t *testing.T) {
		raw := []byte{
			0x94,
			0xcd, 0x04, 0x4c, // version = 1100
			0x91, 0xa1, 'x',
			0x82, 0xa1, 'a', 0x91, 0xc0, 0xa1, 'b', 0xc7, 0x01, 0x2a, 0xff,
			0xd6, 0x01, 0, 0, 0, 7,
		}
		decoded := colorPresetOrderListDecodeRaw(t, raw)
		fieldCount := 4
		want := &ColorPresetOrderList{
			Version:     1100,
			IDOrderList: []*string{colorPresetOrderString("x")},
			FieldCount:  &fieldCount,
			FutureSlots: [][]byte{
				{0x82, 0xa1, 'a', 0x91, 0xc0, 0xa1, 'b', 0xc7, 0x01, 0x2a, 0xff},
				{0xd6, 0x01, 0, 0, 0, 7},
			},
		}
		if !reflect.DeepEqual(decoded, want) {
			t.Fatalf("decoded future object = %#v, want %#v", decoded, want)
		}
		reencoded, err := EncodeColorPresetOrderList(decoded)
		if err != nil {
			t.Fatal(err)
		}
		reencoded, err = ct.DecompressLz4BlockArray(reencoded)
		if err != nil || !bytes.Equal(reencoded, raw) {
			t.Fatalf("future object wire changed: equal=%v err=%v", bytes.Equal(reencoded, raw), err)
		}
	})

	t.Run("encoding preserves version without mutating caller", func(t *testing.T) {
		name := "guid"
		value := &ColorPresetOrderList{Version: -123, IDOrderList: []*string{&name}}
		before := &ColorPresetOrderList{Version: value.Version, IDOrderList: append([]*string(nil), value.IDOrderList...)}
		encoded, err := EncodeColorPresetOrderList(value)
		if err != nil {
			t.Fatalf("EncodeColorPresetOrderList() error = %v", err)
		}
		if !reflect.DeepEqual(value, before) {
			t.Fatalf("encoder modified caller: got %#v, want %#v", value, before)
		}
		decoded, err := DecodeColorPresetOrderList(encoded)
		if err != nil {
			t.Fatalf("DecodeColorPresetOrderList(encoded) error = %v", err)
		}
		if decoded.Version != -123 {
			t.Fatalf("encoded version = %d, want preserved -123", decoded.Version)
		}
	})
}

func TestColorPresetOrderListNullableStringsAndUTF8(t *testing.T) {
	t.Run("nil list", func(t *testing.T) {
		decoded := colorPresetOrderListDecodeRaw(t, []byte{0x92, 0xcd, 0x03, 0xe8, 0xc0})
		if decoded.IDOrderList != nil {
			t.Fatalf("nil list decoded as %#v", decoded.IDOrderList)
		}
		encoded, err := EncodeColorPresetOrderList(decoded)
		if err != nil {
			t.Fatalf("EncodeColorPresetOrderList(nil list) error = %v", err)
		}
		raw, err := ct.DecompressLz4BlockArray(encoded)
		if err != nil {
			t.Fatal(err)
		}
		if want := []byte{0x92, 0xcd, 0x03, 0xe8, 0xc0}; !bytes.Equal(raw, want) || !bytes.Equal(encoded, want) {
			t.Fatalf("nil-list wire = %x, want %x", raw, want)
		}
	})

	t.Run("invalid wire UTF-8 follows MessagePack-CSharp replacement fallback", func(t *testing.T) {
		decoded := colorPresetOrderListDecodeRaw(t, []byte{0x92, 0xcd, 0x03, 0xe8, 0x91, 0xa1, 0xff})
		if len(decoded.IDOrderList) != 1 || decoded.IDOrderList[0] == nil || *decoded.IDOrderList[0] != "\uFFFD" {
			t.Fatalf("invalid UTF-8 decoded as %#v, want U+FFFD replacement", decoded.IDOrderList)
		}
	})

	t.Run("invalid Go UTF-8 is not serializable as a dotnet string", func(t *testing.T) {
		invalid := "\xff"
		_, err := EncodeColorPresetOrderList(&ColorPresetOrderList{IDOrderList: []*string{&invalid}})
		if err == nil || !strings.Contains(err.Error(), "UTF-8") {
			t.Fatalf("EncodeColorPresetOrderList(invalid UTF-8) error = %v, want UTF-8 rejection", err)
		}
	})
}

func TestColorPresetOrderListRejectsMalformedWire(t *testing.T) {
	validRaw := []byte{0x92, 0xcd, 0x03, 0xe8, 0x91, 0xa1, 'x'}
	uint32Overflow := []byte{0x92, 0xce, 0x80, 0, 0, 0, 0x90}
	int64Underflow := []byte{0x92, 0xd3, 0xff, 0xff, 0xff, 0xff, 0x7f, 0xff, 0xff, 0xff, 0x90}
	tests := map[string][]byte{
		"empty":                   nil,
		"root map":                {0x80},
		"version nil":             {0x91, 0xc0},
		"version uint overflow":   uint32Overflow,
		"version int underflow":   int64Underflow,
		"list wrong type":         {0x92, 0xcd, 0x03, 0xe8, 0x80},
		"element wrong type":      {0x92, 0xcd, 0x03, 0xe8, 0x91, 0x01},
		"declared list bomb":      {0x92, 0xcd, 0x03, 0xe8, 0xdd, 0xff, 0xff, 0xff, 0xff},
		"truncated future slot":   {0x93, 0xcd, 0x03, 0xe8, 0x90, 0xc7, 0x02, 0x2a, 0x00},
		"length-prefixed payload": append([]byte{7, 0, 0, 0}, validRaw...),
	}
	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			wire := colorPresetOrderListGameFrame(t, raw)
			if name == "empty" {
				wire = nil
			}
			if name == "length-prefixed payload" {
				wire = raw // A BinaryWriter prefix is outside the compressed value.
			}
			if _, err := DecodeColorPresetOrderList(wire); err == nil {
				t.Fatalf("DecodeColorPresetOrderList(%x) unexpectedly succeeded", wire)
			}
		})
	}

	largeRaw := []byte{0x92, 0xcd, 0x03, 0xe8, 0x91, 0xd9, 80}
	largeRaw = append(largeRaw, bytes.Repeat([]byte{'x'}, 80)...)
	compressed := colorPresetOrderListCompress(t, largeRaw)
	for length := 0; length < len(compressed); length++ {
		if _, err := DecodeColorPresetOrderList(compressed[:length]); err == nil {
			t.Fatalf("truncated compressed prefix %d/%d unexpectedly succeeded", length, len(compressed))
		}
	}
}

func TestColorPresetOrderListCollectionBoundaryAndDeterminism(t *testing.T) {
	const formerImplementationLimit = 1 << 20
	tooManyRaw := []byte{0x92, 0xcd, 0x03, 0xe8, 0xdd, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(tooManyRaw[len(tooManyRaw)-4:], uint32(formerImplementationLimit+1))
	tooManyRaw = append(tooManyRaw, bytes.Repeat([]byte{0xc0}, formerImplementationLimit+1)...)
	largeDecoded, err := DecodeColorPresetOrderList(tooManyRaw)
	if err != nil {
		t.Fatalf("valid list above former implementation limit: %v", err)
	}
	if len(largeDecoded.IDOrderList) != formerImplementationLimit+1 {
		t.Fatalf("valid list above former implementation limit: length=%d", len(largeDecoded.IDOrderList))
	}

	large := make([]*string, formerImplementationLimit+1)
	largeWire, err := EncodeColorPresetOrderList(&ColorPresetOrderList{IDOrderList: large})
	if err != nil {
		t.Fatalf("EncodeColorPresetOrderList(valid large list): %v", err)
	}
	largeRoundTrip, err := DecodeColorPresetOrderList(largeWire)
	if err != nil {
		t.Fatalf("decode encoded large list: %v", err)
	}
	if len(largeRoundTrip.IDOrderList) != len(large) {
		t.Fatalf("large list round trip: length=%d", len(largeRoundTrip.IDOrderList))
	}

	a, b := strings.Repeat("a", 80), "b"
	value := &ColorPresetOrderList{Version: math.MinInt32, IDOrderList: []*string{&a, nil, &b}}
	before := append([]*string(nil), value.IDOrderList...)
	first, err := EncodeColorPresetOrderList(value)
	if err != nil {
		t.Fatalf("first EncodeColorPresetOrderList() error = %v", err)
	}
	second, err := EncodeColorPresetOrderList(value)
	if err != nil {
		t.Fatalf("second EncodeColorPresetOrderList() error = %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("encoding is not deterministic: first=%x second=%x", first, second)
	}
	if value.Version != math.MinInt32 || !reflect.DeepEqual(value.IDOrderList, before) {
		t.Fatalf("encoder modified caller: %#v", value)
	}
}

func colorPresetOrderListDecodeRaw(t *testing.T, raw []byte) *ColorPresetOrderList {
	t.Helper()
	decoded, err := DecodeColorPresetOrderList(colorPresetOrderListGameFrame(t, raw))
	if err != nil {
		t.Fatalf("DecodeColorPresetOrderList(%x) error = %v", raw, err)
	}
	return decoded
}

func colorPresetOrderListGameFrame(t *testing.T, raw []byte) []byte {
	t.Helper()
	if len(raw) < colorPresetOrderListCompressionMinLength {
		return append([]byte(nil), raw...)
	}
	return colorPresetOrderListCompress(t, raw)
}

func colorPresetOrderListCompress(t *testing.T, raw []byte) []byte {
	t.Helper()
	compressed, err := ct.CompressLz4BlockArray(raw)
	if err != nil {
		t.Fatalf("CompressLz4BlockArray(%x) error = %v", raw, err)
	}
	return compressed
}

func colorPresetOrderString(value string) *string {
	return &value
}
