package KCES

import (
	"fmt"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	"github.com/ugorji/go/codec"
)

// KCES 各 MessagePack 格式共用的未知槽位逐字节保留类型。
//
// Byte-preserving unknown-slot type shared by KCES MessagePack formats.

// RawMessagePackSlot preserves one complete MessagePack value byte-for-byte.
// It is used for sparse int-key holes that have no C# member: the generated
// game formatter normally writes nil there, but a faithful editor must not
// reinterpret or canonicalize an anomalous value found in an existing file.
// A nil/empty RawMessagePackSlot represents the normal MessagePack nil value.
type RawMessagePackSlot []byte

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

func (v *RawMessagePackSlot) CodecDecodeSelf(d *codec.Decoder) {
	var raw codec.Raw
	d.MustDecode(&raw)
	if len(raw) == 0 {
		*v = nil
		return
	}
	*v = append((*v)[:0], raw...)
}
