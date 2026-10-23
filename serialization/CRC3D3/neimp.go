package CRC3D3

import (
	"fmt"
	"io"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/msgpack"
)

// .neimp
// .neimp 是 KISS 游戏中 .nei 的后继格式：CsvManager 的 csvExtensio 常量是 .neimp，而 .nei 退为 csvLegacyExtensio，两者在新游戏中并存，同一张表可能同时有 .nei 与 .neimp 两份
// 文件本体是 MessagePack-CSharp 序列化出的对象图，未压缩时直接是 MessagePack 根值，压缩时是 msgpack 包已实现的 Lz4BlockArray 分帧：array N+1、记录每块解压大小的 ext 98 头与后续 bin 块
// 游戏使用 MessagePackOption.PrivateEnumStringLz4BlockArray，即 DynamicEnumAsStringResolver 加 StandardResolverAllowPrivate 再加 MessagePackCompression.Lz4BlockArray，并按 CompressionMinLength 默认值 64 决定是否压缩
// 本包只负责容器与压缩策略，值树及其 JSON 表示由 msgpack 包提供
// .neimp
// .neimp is the successor of .nei in KISS games: CsvManager's csvExtensio constant is .neimp while .nei became csvLegacyExtensio, both coexist in newer games, and one table may exist as both a .nei and a .neimp file
// The file body is an object graph serialized by MessagePack-CSharp: uncompressed it is a bare MessagePack root value, and compressed it is the Lz4BlockArray framing the msgpack package already implements, namely an array of N+1 elements, an ext 98 header carrying each block's decompressed size, and the following bin blocks
// The game uses MessagePackOption.PrivateEnumStringLz4BlockArray, which is DynamicEnumAsStringResolver plus StandardResolverAllowPrivate plus MessagePackCompression.Lz4BlockArray, and decides whether to compress by the CompressionMinLength default of 64
// This package only owns the container and the compression policy, while the msgpack package provides the value tree and its JSON representation

// NeimpExtension 是 .neimp 原生文件的后缀
// NeimpExtension is the suffix of a native .neimp file
const NeimpExtension = ".neimp"

// NeimpJSONExtension 是 .neimp JSON 的复合后缀
// NeimpJSONExtension is the compound suffix of .neimp JSON
const NeimpJSONExtension = ".neimp.json"

// neimpCompressionMinLength 是游戏的 MessagePackSerializerOptions.CompressionMinLength 默认值
// 序列化结果短于该长度时 MessagePack-CSharp 原样写出未压缩字节，否则写成 Lz4BlockArray，本包按同一规则决定写出形式
// neimpCompressionMinLength is the default MessagePackSerializerOptions.CompressionMinLength of the game
// MessagePack-CSharp writes the bytes uncompressed when the serialized form is shorter than this length and writes Lz4BlockArray otherwise, and this package selects the written form by the same rule
const neimpCompressionMinLength = 64

// Value 是 .neimp 值树的类型别名，值树本身实现在 msgpack 包
// Value is a type alias of the .neimp value tree, which the msgpack package implements
type Value = msgpack.Value

// DecodeNeimp 解析一个 .neimp 文件的完整内容为值树
// 未压缩与 Lz4BlockArray 压缩两种形式都接受，值树的解码复用 msgpack 包的 DecodeValue
// DecodeNeimp parses the complete content of one .neimp file into a value tree
// Both the uncompressed form and the Lz4BlockArray form are accepted, and the value tree is decoded through the msgpack package's DecodeValue
func DecodeNeimp(data []byte) (Value, error) {
	if len(data) == 0 {
		return Value{}, fmt.Errorf(".neimp file is empty")
	}
	payload, err := msgpack.DecompressLz4BlockArray(data)
	if err != nil {
		return Value{}, fmt.Errorf("decompress .neimp payload: %w", err)
	}
	value, err := msgpack.DecodeValue(payload)
	if err != nil {
		return Value{}, fmt.Errorf("decode .neimp MessagePack: %w", err)
	}
	return value, nil
}

// EncodeNeimp 将值树编码为 .neimp 文件内容
// 序列化结果短于 64 字节时原样写出，否则写成 Lz4BlockArray，与游戏的写出规则一致
// 分块边界与压缩强度只影响体积，游戏按 ext 98 头记录的每块解压大小还原数据
// EncodeNeimp encodes a value tree into .neimp file content
// The result is written verbatim when shorter than 64 bytes and as Lz4BlockArray otherwise, matching the rule the game writes by
// Block boundaries and compression strength affect only the size, because the game restores data from the per-block decompressed sizes recorded in the ext 98 header
func EncodeNeimp(value Value) ([]byte, error) {
	payload, err := msgpack.EncodeValue(value)
	if err != nil {
		return nil, fmt.Errorf("encode .neimp MessagePack: %w", err)
	}
	if len(payload) < neimpCompressionMinLength {
		return payload, nil
	}
	compressed, err := msgpack.CompressLz4BlockArray(payload)
	if err != nil {
		return nil, fmt.Errorf("compress .neimp payload: %w", err)
	}
	return compressed, nil
}

// ReadNeimp 从 r 中读取一个 .neimp 文件并解析为值树
// ReadNeimp reads one .neimp file from r and parses it into a value tree
func ReadNeimp(r io.Reader) (Value, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Value{}, fmt.Errorf("read .neimp data: %w", err)
	}
	return DecodeNeimp(data)
}

// DumpNeimp 将值树写出为 .neimp 文件内容
// DumpNeimp writes a value tree as .neimp file content
func DumpNeimp(w io.Writer, value Value) error {
	encoded, err := EncodeNeimp(value)
	if err != nil {
		return err
	}
	if _, err := w.Write(encoded); err != nil {
		return fmt.Errorf("write .neimp data: %w", err)
	}
	return nil
}

// DecodeNeimpJSON 解析 .neimp 的 JSON 表示为值树
// JSON 的根就是值树本身，没有额外封套，具体表示见 msgpack 包的 DecodeValueJSON
// DecodeNeimpJSON parses the JSON representation of .neimp into a value tree
// The JSON root is the value tree itself with no surrounding envelope, and the msgpack package's DecodeValueJSON documents the representation
func DecodeNeimpJSON(data []byte) (Value, error) {
	return msgpack.DecodeValueJSON(data)
}

// EncodeNeimpJSON 将值树写为两空格缩进并以换行结尾的 UTF-8 JSON
// EncodeNeimpJSON writes the value tree as UTF-8 JSON indented with two spaces and terminated by a newline
func EncodeNeimpJSON(value Value) ([]byte, error) {
	return msgpack.EncodeValueJSON(value)
}
