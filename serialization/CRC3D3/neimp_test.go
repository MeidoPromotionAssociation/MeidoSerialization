package CRC3D3

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/msgpack"
)

// neimpSampleDirectory 是 CRC3D3 的 csv.aba 解包后的 TextAsset 目录，内含全部 .neimp 样本
// neimpSampleDirectory is the TextAsset directory unpacked from CRC3D3 csv.aba and holds every .neimp sample
const neimpSampleDirectory = "../../testdata/CRC3D3/csv.aba_unpacked/TextAsset"

// TestNeimpSamplesRoundTrip 验证每个真实样本的载荷在二进制与 JSON 往返后语义不变
// 只要求游戏能读出同样的数据，不要求字节与体积与游戏写出的接近：解码走 ugorji，它还原的映射是 Go map 且没有顺序，映射键一律排序写出
// 游戏读取依赖的是字段位置与键值本身，不依赖键的先后，也不依赖整数用哪种宽度写出
// TestNeimpSamplesRoundTrip verifies that every real sample keeps its semantics through the binary and the JSON round trip
// Only the game reading the same data matters, so neither the bytes nor the size has to resemble what the game wrote: decoding goes through ugorji, whose maps are unordered Go maps, so map keys are always written in sorted order
// The game's reading depends on field positions and key-value pairs rather than on key order or on which integer width is used
func TestNeimpSamplesRoundTrip(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join(neimpSampleDirectory, "*"+NeimpExtension))
	if err != nil {
		t.Fatalf("glob samples: %v", err)
	}
	if len(paths) == 0 {
		t.Fatalf("no %s samples under %s", NeimpExtension, neimpSampleDirectory)
	}
	for _, path := range paths {
		name := filepath.Base(path)
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read sample: %v", err)
			}
			value, err := DecodeNeimp(data)
			if err != nil {
				t.Fatalf("DecodeNeimp: %v", err)
			}

			reencoded, err := EncodeNeimp(value)
			if err != nil {
				t.Fatalf("EncodeNeimp: %v", err)
			}
			decodedAgain, err := DecodeNeimp(reencoded)
			if err != nil {
				t.Fatalf("DecodeNeimp after EncodeNeimp: %v", err)
			}
			if !reflect.DeepEqual(value, decodedAgain) {
				t.Fatalf("value tree changed through the binary round trip")
			}

			jsonData, err := EncodeNeimpJSON(value)
			if err != nil {
				t.Fatalf("EncodeNeimpJSON: %v", err)
			}
			if !json.Valid(jsonData) {
				t.Fatalf("EncodeNeimpJSON produced invalid JSON")
			}
			fromJSON, err := DecodeNeimpJSON(jsonData)
			if err != nil {
				t.Fatalf("DecodeNeimpJSON: %v", err)
			}
			if !reflect.DeepEqual(value, fromJSON) {
				t.Fatalf("value tree changed through the JSON round trip")
			}
		})
	}
}

// TestNeimpCompressionFollowsGameThreshold 验证写出形式按游戏的最小压缩长度切换
// TestNeimpCompressionFollowsGameThreshold verifies that the written form switches at the game's minimum compression length
func TestNeimpCompressionFollowsGameThreshold(t *testing.T) {
	short := Value{Kind: msgpack.KindArray, Array: []Value{
		{Kind: msgpack.KindUint, Uint: 0},
		{Kind: msgpack.KindUint, Uint: 10},
	}}
	encoded, err := EncodeNeimp(short)
	if err != nil {
		t.Fatalf("EncodeNeimp short: %v", err)
	}
	if encoded[0] != 0x92 {
		t.Fatalf("short value must stay uncompressed, first byte = 0x%02x", encoded[0])
	}

	long := Value{Kind: msgpack.KindArray}
	for index := 0; index < 40; index++ {
		long.Array = append(long.Array, Value{Kind: msgpack.KindString, Str: "heroine"})
	}
	encoded, err = EncodeNeimp(long)
	if err != nil {
		t.Fatalf("EncodeNeimp long: %v", err)
	}
	if !isNeimpLz4Framed(encoded) {
		t.Fatalf("long value must be written as Lz4BlockArray")
	}
	decoded, err := DecodeNeimp(encoded)
	if err != nil {
		t.Fatalf("DecodeNeimp long: %v", err)
	}
	if !reflect.DeepEqual(decoded, long) {
		t.Fatalf("compressed value did not survive the round trip")
	}
}

// isNeimpLz4Framed 判断字节是否为 MessagePack-CSharp 的 Lz4BlockArray 分帧
// isNeimpLz4Framed reports whether the bytes use MessagePack-CSharp Lz4BlockArray framing
func isNeimpLz4Framed(data []byte) bool {
	position := int64(0)
	if len(data) == 0 {
		return false
	}
	if _, err := msgpack.ReadArrayHeaderStrict(data, &position); err != nil {
		return false
	}
	if position >= int64(len(data)) {
		return false
	}
	switch data[position] {
	case 0xc7:
		return position+2 < int64(len(data)) && int8(data[position+2]) == msgpack.Lz4ArrayType
	case 0xc8:
		return position+3 < int64(len(data)) && int8(data[position+3]) == msgpack.Lz4ArrayType
	case 0xc9:
		return position+5 < int64(len(data)) && int8(data[position+5]) == msgpack.Lz4ArrayType
	default:
		return false
	}
}
