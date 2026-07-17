package KCES

import (
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	"github.com/ugorji/go/codec"
)

// These tiny codec.Selfer forwarders keep all indexed-array mechanics in one
// implementation (ct.Encode/DecodeIndexedObjectSelf). Declaring the methods on
// the concrete game types lets ugorji/codec invoke that implementation at any
// nesting depth, including slice elements and map values.

func (v BezierParam) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *BezierParam) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v ClothParams) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *ClothParams) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

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

func (v Vector2) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *Vector2) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v Vector2Int) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *Vector2Int) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v Vector3) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *Vector3) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v Vector4) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *Vector4) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v PartsColor) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *PartsColor) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v PreMulTexDatas) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *PreMulTexDatas) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v TransTexData) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *TransTexData) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v InfColorParam) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *InfColorParam) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v MaskData) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *MaskData) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v MaskParam) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *MaskParam) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v PartColDef) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *PartColDef) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v GradaColDef) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *GradaColDef) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v InfColData) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *InfColData) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v Colvari) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *Colvari) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v ColvariData) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *ColvariData) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v BlendData) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *BlendData) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v SkinThickness) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *SkinThickness) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v ThicknessGroup) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *ThicknessGroup) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v ThicknessPoint) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *ThicknessPoint) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v ThicknessDefPerAngle) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *ThicknessDefPerAngle) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}

func (v TupleStringInt) CodecEncodeSelf(e *codec.Encoder)  { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *TupleStringInt) CodecDecodeSelf(d *codec.Decoder) { ct.DecodeIndexedObjectSelf(d, v) }

func (v DynamicBoneStatus) CodecEncodeSelf(e *codec.Encoder) { ct.EncodeIndexedObjectSelf(e, &v) }
func (v *DynamicBoneStatus) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}

func (v DynamicBoneAnimationFrame) CodecEncodeSelf(e *codec.Encoder) {
	ct.EncodeIndexedObjectSelf(e, &v)
}
func (v *DynamicBoneAnimationFrame) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}

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

func (v ColliderPackage) CodecEncodeSelf(e *codec.Encoder) {
	ct.EncodeIndexedObjectSelf(e, &v)
}
func (v *ColliderPackage) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}

func (v ColliderPlane) CodecEncodeSelf(e *codec.Encoder) {
	ct.EncodeIndexedObjectSelf(e, &v)
}
func (v *ColliderPlane) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}

func (v ColliderCapsule) CodecEncodeSelf(e *codec.Encoder) {
	ct.EncodeIndexedObjectSelf(e, &v)
}
func (v *ColliderCapsule) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}

func (v ColliderSphere) CodecEncodeSelf(e *codec.Encoder) {
	ct.EncodeIndexedObjectSelf(e, &v)
}
func (v *ColliderSphere) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}

func (v ColliderMaidProp) CodecEncodeSelf(e *codec.Encoder) {
	ct.EncodeIndexedObjectSelf(e, &v)
}
func (v *ColliderMaidProp) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}

func (v ColliderState) CodecEncodeSelf(e *codec.Encoder) {
	ct.EncodeIndexedObjectSelf(e, &v)
}
func (v *ColliderState) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}

func (v LimbColliderPackage) CodecEncodeSelf(e *codec.Encoder) {
	ct.EncodeIndexedObjectSelf(e, &v)
}
func (v *LimbColliderPackage) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}

func (v IKColliderPackage) CodecEncodeSelf(e *codec.Encoder) {
	ct.EncodeIndexedObjectSelf(e, &v)
}
func (v *IKColliderPackage) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}

func (v IKColliderGroup) CodecEncodeSelf(e *codec.Encoder) {
	ct.EncodeIndexedObjectSelf(e, &v)
}
func (v *IKColliderGroup) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}
