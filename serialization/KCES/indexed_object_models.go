package KCES

import (
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	"github.com/ugorji/go/codec"
)

// 共用线格式类型的 MessagePack indexed-object Selfer 转发实现
// MessagePack indexed-object Selfer forwarders for shared wire types

// CodecEncodeSelf 按共享 indexed-object 规则编码 BezierParam
// CodecEncodeSelf encodes BezierParam using the shared indexed-object rules
func (v BezierParam) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 BezierParam
// CodecDecodeSelf decodes BezierParam using the shared indexed-object rules
func (v *BezierParam) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 ClothParams
// CodecEncodeSelf encodes ClothParams using the shared indexed-object rules
func (v ClothParams) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 ClothParams
// CodecDecodeSelf decodes ClothParams using the shared indexed-object rules
func (v *ClothParams) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 Vector2
// CodecEncodeSelf encodes Vector2 using the shared indexed-object rules
func (v Vector2) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 Vector2
// CodecDecodeSelf decodes Vector2 using the shared indexed-object rules
func (v *Vector2) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 Vector2Int
// CodecEncodeSelf encodes Vector2Int using the shared indexed-object rules
func (v Vector2Int) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 Vector2Int
// CodecDecodeSelf decodes Vector2Int using the shared indexed-object rules
func (v *Vector2Int) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 Vector3
// CodecEncodeSelf encodes Vector3 using the shared indexed-object rules
func (v Vector3) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 Vector3
// CodecDecodeSelf decodes Vector3 using the shared indexed-object rules
func (v *Vector3) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 Vector4
// CodecEncodeSelf encodes Vector4 using the shared indexed-object rules
func (v Vector4) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 Vector4
// CodecDecodeSelf decodes Vector4 using the shared indexed-object rules
func (v *Vector4) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 PreMulTexDatas
// CodecEncodeSelf encodes PreMulTexDatas using the shared indexed-object rules
func (v PreMulTexDatas) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 PreMulTexDatas
// CodecDecodeSelf decodes PreMulTexDatas using the shared indexed-object rules
func (v *PreMulTexDatas) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 TransTexData
// CodecEncodeSelf encodes TransTexData using the shared indexed-object rules
func (v TransTexData) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 TransTexData
// CodecDecodeSelf decodes TransTexData using the shared indexed-object rules
func (v *TransTexData) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 InfColorParam
// CodecEncodeSelf encodes InfColorParam using the shared indexed-object rules
func (v InfColorParam) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 InfColorParam
// CodecDecodeSelf decodes InfColorParam using the shared indexed-object rules
func (v *InfColorParam) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 MaskData
// CodecEncodeSelf encodes MaskData using the shared indexed-object rules
func (v MaskData) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 MaskData
// CodecDecodeSelf decodes MaskData using the shared indexed-object rules
func (v *MaskData) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 MaskParam
// CodecEncodeSelf encodes MaskParam using the shared indexed-object rules
func (v MaskParam) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 MaskParam
// CodecDecodeSelf decodes MaskParam using the shared indexed-object rules
func (v *MaskParam) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 PartColDef
// CodecEncodeSelf encodes PartColDef using the shared indexed-object rules
func (v PartColDef) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 PartColDef
// CodecDecodeSelf decodes PartColDef using the shared indexed-object rules
func (v *PartColDef) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 GradaColDef
// CodecEncodeSelf encodes GradaColDef using the shared indexed-object rules
func (v GradaColDef) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 GradaColDef
// CodecDecodeSelf decodes GradaColDef using the shared indexed-object rules
func (v *GradaColDef) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 InfColData
// CodecEncodeSelf encodes InfColData using the shared indexed-object rules
func (v InfColData) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 InfColData
// CodecDecodeSelf decodes InfColData using the shared indexed-object rules
func (v *InfColData) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 Colvari
// CodecEncodeSelf encodes Colvari using the shared indexed-object rules
func (v Colvari) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 Colvari
// CodecDecodeSelf decodes Colvari using the shared indexed-object rules
func (v *Colvari) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 ColvariData
// CodecEncodeSelf encodes ColvariData using the shared indexed-object rules
func (v ColvariData) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 ColvariData
// CodecDecodeSelf decodes ColvariData using the shared indexed-object rules
func (v *ColvariData) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 BlendData
// CodecEncodeSelf encodes BlendData using the shared indexed-object rules
func (v BlendData) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 BlendData
// CodecDecodeSelf decodes BlendData using the shared indexed-object rules
func (v *BlendData) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 SkinThickness
// CodecEncodeSelf encodes SkinThickness using the shared indexed-object rules
func (v SkinThickness) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 SkinThickness
// CodecDecodeSelf decodes SkinThickness using the shared indexed-object rules
func (v *SkinThickness) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 ThicknessGroup
// CodecEncodeSelf encodes ThicknessGroup using the shared indexed-object rules
func (v ThicknessGroup) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 ThicknessGroup
// CodecDecodeSelf decodes ThicknessGroup using the shared indexed-object rules
func (v *ThicknessGroup) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 ThicknessPoint
// CodecEncodeSelf encodes ThicknessPoint using the shared indexed-object rules
func (v ThicknessPoint) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 ThicknessPoint
// CodecDecodeSelf decodes ThicknessPoint using the shared indexed-object rules
func (v *ThicknessPoint) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 ThicknessDefPerAngle
// CodecEncodeSelf encodes ThicknessDefPerAngle using the shared indexed-object rules
func (v ThicknessDefPerAngle) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 ThicknessDefPerAngle
// CodecDecodeSelf decodes ThicknessDefPerAngle using the shared indexed-object rules
func (v *ThicknessDefPerAngle) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}

// CodecEncodeSelf 按共享 indexed-object 规则编码 TupleStringInt
// CodecEncodeSelf encodes TupleStringInt using the shared indexed-object rules
func (v TupleStringInt) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 TupleStringInt
// CodecDecodeSelf decodes TupleStringInt using the shared indexed-object rules
func (v *TupleStringInt) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 ColliderPackage
// CodecEncodeSelf encodes ColliderPackage using the shared indexed-object rules
func (v ColliderPackage) CodecEncodeSelf(e *codec.Encoder) {
	ct.EncodeIndexedObjectSelf(e, &v)
}

// CodecDecodeSelf 按共享 indexed-object 规则解码 ColliderPackage
// CodecDecodeSelf decodes ColliderPackage using the shared indexed-object rules
func (v *ColliderPackage) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}

// CodecEncodeSelf 按共享 indexed-object 规则编码 ColliderPlane
// CodecEncodeSelf encodes ColliderPlane using the shared indexed-object rules
func (v ColliderPlane) CodecEncodeSelf(e *codec.Encoder) {
	ct.EncodeIndexedObjectSelf(e, &v)
}

// CodecDecodeSelf 按共享 indexed-object 规则解码 ColliderPlane
// CodecDecodeSelf decodes ColliderPlane using the shared indexed-object rules
func (v *ColliderPlane) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}

// CodecEncodeSelf 按共享 indexed-object 规则编码 ColliderCapsule
// CodecEncodeSelf encodes ColliderCapsule using the shared indexed-object rules
func (v ColliderCapsule) CodecEncodeSelf(e *codec.Encoder) {
	ct.EncodeIndexedObjectSelf(e, &v)
}

// CodecDecodeSelf 按共享 indexed-object 规则解码 ColliderCapsule
// CodecDecodeSelf decodes ColliderCapsule using the shared indexed-object rules
func (v *ColliderCapsule) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}

// CodecEncodeSelf 按共享 indexed-object 规则编码 ColliderSphere
// CodecEncodeSelf encodes ColliderSphere using the shared indexed-object rules
func (v ColliderSphere) CodecEncodeSelf(e *codec.Encoder) {
	ct.EncodeIndexedObjectSelf(e, &v)
}

// CodecDecodeSelf 按共享 indexed-object 规则解码 ColliderSphere
// CodecDecodeSelf decodes ColliderSphere using the shared indexed-object rules
func (v *ColliderSphere) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}

// CodecEncodeSelf 按共享 indexed-object 规则编码 ColliderMaidProp
// CodecEncodeSelf encodes ColliderMaidProp using the shared indexed-object rules
func (v ColliderMaidProp) CodecEncodeSelf(e *codec.Encoder) {
	ct.EncodeIndexedObjectSelf(e, &v)
}

// CodecDecodeSelf 按共享 indexed-object 规则解码 ColliderMaidProp
// CodecDecodeSelf decodes ColliderMaidProp using the shared indexed-object rules
func (v *ColliderMaidProp) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}

// CodecEncodeSelf 按共享 indexed-object 规则编码 ColliderState
// CodecEncodeSelf encodes ColliderState using the shared indexed-object rules
func (v ColliderState) CodecEncodeSelf(e *codec.Encoder) {
	ct.EncodeIndexedObjectSelf(e, &v)
}

// CodecDecodeSelf 按共享 indexed-object 规则解码 ColliderState
// CodecDecodeSelf decodes ColliderState using the shared indexed-object rules
func (v *ColliderState) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}

// CodecEncodeSelf 按共享 indexed-object 规则编码 LimbColliderPackage
// CodecEncodeSelf encodes LimbColliderPackage using the shared indexed-object rules
func (v LimbColliderPackage) CodecEncodeSelf(e *codec.Encoder) {
	ct.EncodeIndexedObjectSelf(e, &v)
}

// CodecDecodeSelf 按共享 indexed-object 规则解码 LimbColliderPackage
// CodecDecodeSelf decodes LimbColliderPackage using the shared indexed-object rules
func (v *LimbColliderPackage) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}

// CodecEncodeSelf 按共享 indexed-object 规则编码 IKColliderPackage
// CodecEncodeSelf encodes IKColliderPackage using the shared indexed-object rules
func (v IKColliderPackage) CodecEncodeSelf(e *codec.Encoder) {
	ct.EncodeIndexedObjectSelf(e, &v)
}

// CodecDecodeSelf 按共享 indexed-object 规则解码 IKColliderPackage
// CodecDecodeSelf decodes IKColliderPackage using the shared indexed-object rules
func (v *IKColliderPackage) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}

// CodecEncodeSelf 按共享 indexed-object 规则编码 IKColliderGroup
// CodecEncodeSelf encodes IKColliderGroup using the shared indexed-object rules
func (v IKColliderGroup) CodecEncodeSelf(e *codec.Encoder) {
	ct.EncodeIndexedObjectSelf(e, &v)
}

// CodecDecodeSelf 按共享 indexed-object 规则解码 IKColliderGroup
// CodecDecodeSelf decodes IKColliderGroup using the shared indexed-object rules
func (v *IKColliderGroup) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}
