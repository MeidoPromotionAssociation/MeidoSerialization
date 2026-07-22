package KCES

import (
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	"github.com/ugorji/go/codec"
)

// 必须在 KCES 包内为各扩展名本地类型声明的 MessagePack indexed-object Selfer 转发实现
// MessagePack indexed-object Selfer forwarders that must be declared in package KCES for extension-local types

// 这些 codec.Selfer 转发方法将全部 indexed-array 机制集中在 ct.EncodeIndexedObjectSelf 和 ct.DecodeIndexedObjectSelf 中，在具体游戏类型上声明方法可使 ugorji/codec 在切片元素、映射值等任意嵌套深度调用同一实现
// These codec.Selfer forwarders keep all indexed-array mechanics in ct.EncodeIndexedObjectSelf and ct.DecodeIndexedObjectSelf, while methods on concrete game types let ugorji/codec invoke the same implementation at any nesting depth including slice elements and map values

// CodecEncodeSelf 按共享 indexed-object 规则编码 Material
// CodecEncodeSelf encodes Material using the shared indexed-object rules
func (v Material) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 Material
// CodecDecodeSelf decodes Material using the shared indexed-object rules
func (v *Material) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 TextureProp
// CodecEncodeSelf encodes TextureProp using the shared indexed-object rules
func (v TextureProp) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 TextureProp
// CodecDecodeSelf decodes TextureProp using the shared indexed-object rules
func (v *TextureProp) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 ColorProp
// CodecEncodeSelf encodes ColorProp using the shared indexed-object rules
func (v ColorProp) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 ColorProp
// CodecDecodeSelf decodes ColorProp using the shared indexed-object rules
func (v *ColorProp) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 VectorProp
// CodecEncodeSelf encodes VectorProp using the shared indexed-object rules
func (v VectorProp) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 VectorProp
// CodecDecodeSelf decodes VectorProp using the shared indexed-object rules
func (v *VectorProp) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 FloatProp
// CodecEncodeSelf encodes FloatProp using the shared indexed-object rules
func (v FloatProp) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 FloatProp
// CodecDecodeSelf decodes FloatProp using the shared indexed-object rules
func (v *FloatProp) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 MaterialAssets
// CodecEncodeSelf encodes MaterialAssets using the shared indexed-object rules
func (v MaterialAssets) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 MaterialAssets
// CodecDecodeSelf decodes MaterialAssets using the shared indexed-object rules
func (v *MaterialAssets) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 Menu
// CodecEncodeSelf encodes Menu using the shared indexed-object rules
func (v Menu) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 Menu
// CodecDecodeSelf decodes Menu using the shared indexed-object rules
func (v *Menu) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 Command
// CodecEncodeSelf encodes Command using the shared indexed-object rules
func (v Command) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 Command
// CodecDecodeSelf decodes Command using the shared indexed-object rules
func (v *Command) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 MenuAssets
// CodecEncodeSelf encodes MenuAssets using the shared indexed-object rules
func (v MenuAssets) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 MenuAssets
// CodecDecodeSelf decodes MenuAssets using the shared indexed-object rules
func (v *MenuAssets) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 Model
// CodecEncodeSelf encodes Model using the shared indexed-object rules
func (v Model) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 Model
// CodecDecodeSelf decodes Model using the shared indexed-object rules
func (v *Model) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 TransData
// CodecEncodeSelf encodes TransData using the shared indexed-object rules
func (v TransData) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 TransData
// CodecDecodeSelf decodes TransData using the shared indexed-object rules
func (v *TransData) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 ModelAssets
// CodecEncodeSelf encodes ModelAssets using the shared indexed-object rules
func (v ModelAssets) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 ModelAssets
// CodecDecodeSelf decodes ModelAssets using the shared indexed-object rules
func (v *ModelAssets) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 KCESPresetCore
// CodecEncodeSelf encodes KCESPresetCore using the shared indexed-object rules
func (v KCESPresetCore) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 KCESPresetCore
// CodecDecodeSelf decodes KCESPresetCore using the shared indexed-object rules
func (v *KCESPresetCore) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 KCESPresetMeta
// CodecEncodeSelf encodes KCESPresetMeta using the shared indexed-object rules
func (v KCESPresetMeta) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 KCESPresetMeta
// CodecDecodeSelf decodes KCESPresetMeta using the shared indexed-object rules
func (v *KCESPresetMeta) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 PriorityMaterial
// CodecEncodeSelf encodes PriorityMaterial using the shared indexed-object rules
func (v PriorityMaterial) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 PriorityMaterial
// CodecDecodeSelf decodes PriorityMaterial using the shared indexed-object rules
func (v *PriorityMaterial) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}

// CodecEncodeSelf 按共享 indexed-object 规则编码 PriorityMaterialAssets
// CodecEncodeSelf encodes PriorityMaterialAssets using the shared indexed-object rules
func (v PriorityMaterialAssets) CodecEncodeSelf(e *codec.Encoder) {
	ct.EncodeIndexedObjectSelf(e, &v)
}

// CodecDecodeSelf 按共享 indexed-object 规则解码 PriorityMaterialAssets
// CodecDecodeSelf decodes PriorityMaterialAssets using the shared indexed-object rules
func (v *PriorityMaterialAssets) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}
