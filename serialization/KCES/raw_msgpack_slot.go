package KCES

import (
	"fmt"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	"github.com/ugorji/go/codec"
)

// KCES 各 MessagePack 格式共用的未知槽位逐字节保留类型
// Byte-preserving unknown-slot type shared by KCES MessagePack formats

// RawMessagePackSlot 逐字节保留一个完整的 MessagePack 值，用于 C# 类中没有成员的稀疏 int-key 空槽，游戏生成的格式化器通常在这些位置写入 nil，但无损编辑器不能重新解释或规范化现有文件中的异常值，nil 或空值表示普通的 MessagePack nil
// RawMessagePackSlot preserves one complete MessagePack value byte-for-byte for sparse int-key holes with no C# member, where the generated game formatter normally writes nil but a lossless editor must not reinterpret or canonicalize anomalous existing values, and a nil or empty value represents the normal MessagePack nil
type RawMessagePackSlot []byte

// CodecEncodeSelf 验证并原样编码一个完整的 MessagePack 槽位值
// CodecEncodeSelf validates and encodes one complete MessagePack slot value byte-for-byte
func (v RawMessagePackSlot) CodecEncodeSelf(e *codec.Encoder) {
	raw := []byte(v)
	if len(raw) == 0 {
		raw = []byte{0xc0}
	}
	root, trailing, err := ct.SplitFirstMsgpackValue(raw)
	if err != nil {
		panic(fmt.Errorf("raw MessagePack slot: %w", err))
	}
	if len(trailing) != 0 || len(root) != len(raw) {
		panic(fmt.Errorf("raw MessagePack slot contains %d trailing bytes", len(trailing)))
	}
	e.MustEncode(codec.Raw(append([]byte(nil), raw...)))
}

// CodecDecodeSelf 原样保存解码器读取的下一个完整 MessagePack 值
// CodecDecodeSelf preserves the next complete MessagePack value read by the decoder byte-for-byte
func (v *RawMessagePackSlot) CodecDecodeSelf(d *codec.Decoder) {
	var raw codec.Raw
	d.MustDecode(&raw)
	if len(raw) == 0 {
		*v = nil
		return
	}
	*v = append((*v)[:0], raw...)
}
