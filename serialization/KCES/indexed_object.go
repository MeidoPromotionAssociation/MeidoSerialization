package KCES

import (
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	"github.com/ugorji/go/codec"
)

// 必须在 KCES 包内为各扩展名本地类型声明的 MessagePack indexed-object Selfer 转发实现。
//
// MessagePack indexed-object Selfer forwarders that must be declared in package KCES for extension-local types.

// These tiny codec.Selfer forwarders keep all indexed-array mechanics in one
// implementation (ct.Encode/DecodeIndexedObjectSelf). Declaring the methods on
// the concrete game types lets ugorji/codec invoke that implementation at any
// nesting depth, including slice elements and map values.

func (v Material) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *Material) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v TextureProp) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *TextureProp) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v ColorProp) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *ColorProp) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v VectorProp) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *VectorProp) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v FloatProp) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *FloatProp) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v MaterialAssets) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *MaterialAssets) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v Menu) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *Menu) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v Command) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *Command) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v MenuAssets) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *MenuAssets) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v Model) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *Model) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v TransData) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *TransData) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v ModelAssets) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *ModelAssets) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v KCESPresetCore) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *KCESPresetCore) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v KCESPresetMeta) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *KCESPresetMeta) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v PriorityMaterial) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *PriorityMaterial) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}

func (v PriorityMaterialAssets) CodecEncodeSelf(e *codec.Encoder) {
	ct.EncodeIndexedObjectSelf(e, &v)
}
func (v *PriorityMaterialAssets) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}
