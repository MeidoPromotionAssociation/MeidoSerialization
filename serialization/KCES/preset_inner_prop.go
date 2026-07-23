package KCES

import (
	"bytes"
	"fmt"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
)

// .preset 内 GP03_MPROP_LIST/GP03_MPROP 属性块的详细读写与校验实现。
//
// Detailed reader, writer, and validation implementation for GP03_MPROP_LIST/GP03_MPROP property blocks inside .preset.

func DecodeKCESPresetPropertyData(data []byte) (*KCESPresetPropertyList, error) {
	r := newKCESPresetInnerReader(data)
	signature, err := r.readString("KCES preset property-list signature")
	if err != nil {
		return nil, err
	}
	if signature != KCESPresetPropertyListSignature {
		return nil, fmt.Errorf("invalid KCES preset property-list signature %q", signature)
	}
	version, err := r.br.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read KCES preset property-list version: %w", err)
	}
	count, err := r.readCount("KCES preset property-list count", 2)
	if err != nil {
		return nil, err
	}
	result := &KCESPresetPropertyList{
		Signature:  signature,
		Version:    version,
		Properties: makeKCESCountedSliceForAppend[KCESPresetNamedProperty](uint64(count)),
	}
	for index := int64(0); index < count; index++ {
		path := fmt.Sprintf("KCES preset properties[%d]", index)
		key, err := r.readString(path + ".key")
		if err != nil {
			return nil, err
		}
		property, err := readKCESPresetProperty(r, path+".property")
		if err != nil {
			return nil, err
		}
		result.Properties = append(result.Properties, KCESPresetNamedProperty{Key: key, Property: property})
	}
	if r.r.Len() != 0 {
		result.TrailingData, err = r.br.ReadBytes(r.r.Len())
		if err != nil {
			return nil, fmt.Errorf("read KCES preset property-list trailingData: %w", err)
		}
	}
	return result, nil
}

func readKCESPresetProperty(r *kcesPresetInnerReader, path string) (KCESPresetProperty, error) {
	value := KCESPresetProperty{}
	var err error
	value.Signature, err = r.readString(path + ".signature")
	if err != nil {
		return value, err
	}
	if value.Signature != KCESPresetPropertySignature {
		return value, fmt.Errorf("invalid %s.signature %q", path, value.Signature)
	}
	value.Version, err = r.br.ReadInt32()
	if err != nil {
		return value, fmt.Errorf("read %s.version: %w", path, err)
	}
	value.Name, err = r.readString(path + ".name")
	if err != nil {
		return value, err
	}
	if value.DefaultValue, err = r.br.ReadInt32(); err != nil {
		return value, fmt.Errorf("read %s.defaultValue: %w", path, err)
	}
	if value.Value, err = r.br.ReadInt32(); err != nil {
		return value, fmt.Errorf("read %s.value: %w", path, err)
	}
	if value.TempValue, err = r.br.ReadInt32(); err != nil {
		return value, fmt.Errorf("read %s.tempValue: %w", path, err)
	}
	if value.FileNameRID, err = r.br.ReadUInt64(); err != nil {
		return value, fmt.Errorf("read %s.fileNameRid: %w", path, err)
	}
	if value.Enabled, err = r.br.ReadBool(); err != nil {
		return value, fmt.Errorf("read %s.enabled: %w", path, err)
	}
	if value.Max, err = r.br.ReadInt32(); err != nil {
		return value, fmt.Errorf("read %s.max: %w", path, err)
	}
	if value.Min, err = r.br.ReadInt32(); err != nil {
		return value, fmt.Errorf("read %s.min: %w", path, err)
	}
	materialCount, err := r.readCount(path+".materialProperties count", 3)
	if err != nil {
		return value, err
	}
	value.MaterialProperties = makeKCESCountedSliceForAppend[KCESPresetMaterialPropertySlot](uint64(materialCount))
	for index := int64(0); index < materialCount; index++ {
		slotPath := fmt.Sprintf("%s.materialProperties[%d]", path, index)
		slot, err := readKCESPresetMaterialSlot(r, slotPath, value.Version)
		if err != nil {
			return value, err
		}
		value.MaterialProperties = append(value.MaterialProperties, slot)
	}
	value.Base, err = readKCESPresetPropBase(r, path+".base", value.Version, value.Name, 0)
	if err != nil {
		return value, err
	}
	return value, nil
}

func readKCESPresetMaterialSlot(r *kcesPresetInnerReader, path string, version int32) (KCESPresetMaterialPropertySlot, error) {
	value := KCESPresetMaterialPropertySlot{SlotValue: -1}
	var err error
	value.SlotID, err = r.readString(path + ".slotId")
	if err != nil {
		return value, err
	}
	if version >= 2002 {
		value.SlotValue, err = r.br.ReadInt32()
		if err != nil {
			return value, fmt.Errorf("read %s.slotValue: %w", path, err)
		}
	}
	count, err := r.readCount(path+".properties count", 10)
	if err != nil {
		return value, err
	}
	value.Properties = makeKCESCountedSliceForAppend[KCESPresetNamedMaterialProperty](uint64(count))
	for index := int64(0); index < count; index++ {
		itemPath := fmt.Sprintf("%s.properties[%d]", path, index)
		key, err := r.readString(itemPath + ".key")
		if err != nil {
			return value, err
		}
		rid, err := r.br.ReadUInt64()
		if err != nil {
			return value, fmt.Errorf("read %s.rid: %w", itemPath, err)
		}
		property, err := readKCESPresetMaterialValue(r, itemPath+".property")
		if err != nil {
			return value, err
		}
		value.Properties = append(value.Properties, KCESPresetNamedMaterialProperty{Key: key, RID: rid, Property: property})
	}
	return value, nil
}

func readKCESPresetMaterialValue(r *kcesPresetInnerReader, path string) (KCESPresetMaterialPropertyValue, error) {
	value := KCESPresetMaterialPropertyValue{}
	var err error
	if value.MaterialNumber, err = r.br.ReadInt32(); err != nil {
		return value, fmt.Errorf("read %s.materialNumber: %w", path, err)
	}
	if value.PropertyName, err = r.readNullableString(path + ".propertyName"); err != nil {
		return value, err
	}
	if value.TypeName, err = r.readNullableString(path + ".typeName"); err != nil {
		return value, err
	}
	if value.Value, err = r.readNullableString(path + ".value"); err != nil {
		return value, err
	}
	return value, nil
}

func readKCESPresetPropBase(r *kcesPresetInnerReader, path string, version int32, mpnName string, depth int64) (KCESPresetPropBase, error) {
	value := KCESPresetPropBase{}
	if depth > maxKCESPresetInnerDepth {
		return value, fmt.Errorf("%s nesting exceeds limit %d", path, maxKCESPresetInnerDepth)
	}
	var err error
	if value.Index, err = r.br.ReadInt32(); err != nil {
		return value, fmt.Errorf("read %s.index: %w", path, err)
	}
	value.Type, err = r.readString(path + ".type")
	if err != nil {
		return value, err
	}
	value.SubType, err = r.readString(path + ".subType")
	if err != nil {
		return value, err
	}
	if value.FileName, err = r.readNullableString(path + ".fileName"); err != nil {
		return value, err
	}
	if value.FileNameRID, err = r.br.ReadUInt64(); err != nil {
		return value, fmt.Errorf("read %s.fileNameRid: %w", path, err)
	}
	if value.Enabled, err = r.br.ReadBool(); err != nil {
		return value, fmt.Errorf("read %s.enabled: %w", path, err)
	}
	if value.BeforeFileNameRID, err = r.br.ReadUInt64(); err != nil {
		return value, fmt.Errorf("read %s.beforeFileNameRid: %w", path, err)
	}
	if value.Defines, err = r.br.ReadUInt64(); err != nil {
		return value, fmt.Errorf("read %s.defines: %w", path, err)
	}
	if value.SavedTextureDataRID, err = r.br.ReadUInt64(); err != nil {
		return value, fmt.Errorf("read %s.savedTextureDataRid: %w", path, err)
	}
	if value.SavedTextureDataDefines, err = r.br.ReadUInt64(); err != nil {
		return value, fmt.Errorf("read %s.savedTextureDataDefines: %w", path, err)
	}
	value.SavedTextureData, err = readKCESPresetSavedTextureMap(r, path+".savedTextureData", depth)
	if err != nil {
		return value, err
	}
	if value.ShareInfinityColorData, err = r.br.ReadBool(); err != nil {
		return value, fmt.Errorf("read %s.shareInfinityColorData: %w", path, err)
	}
	editBaseData, err := r.readBlob(path + ".editBaseData")
	if err != nil {
		return value, err
	}
	if len(editBaseData) != 0 {
		value.EditBaseData, err = DecodeKCESPresetEditBaseData(editBaseData)
		if err != nil {
			return value, fmt.Errorf("decode %s.editBaseData: %w", path, err)
		}
	}
	if value.SavedCutoutMaskRID, err = r.br.ReadUInt64(); err != nil {
		return value, fmt.Errorf("read %s.savedCutoutMaskRid: %w", path, err)
	}
	if value.SavedCutoutMask, err = readKCESPresetCutoutMask(r, path+".savedCutoutMask"); err != nil {
		return value, err
	}
	if value.SavedPartHideRID, err = r.br.ReadUInt64(); err != nil {
		return value, fmt.Errorf("read %s.savedPartHideRid: %w", path, err)
	}
	if value.SavedPartHide, err = readKCESPresetPartHides(r, path+".savedPartHide"); err != nil {
		return value, err
	}
	if value.UsePartHide, err = r.br.ReadBool(); err != nil {
		return value, fmt.Errorf("read %s.usePartHide: %w", path, err)
	}
	if value.SavedAttachPositionRID, err = r.br.ReadUInt64(); err != nil {
		return value, fmt.Errorf("read %s.savedAttachPositionRid: %w", path, err)
	}
	if value.SavedAttachPositions, err = readKCESPresetSavedAttachPositions(r, path+".savedAttachPositions"); err != nil {
		return value, err
	}
	if value.NoScale, err = r.br.ReadBool(); err != nil {
		return value, fmt.Errorf("read %s.noScale: %w", path, err)
	}
	if value.SubPropertyIsTuftTexture, err = r.br.ReadBool(); err != nil {
		return value, fmt.Errorf("read %s.subPropertyIsTuftTexture: %w", path, err)
	}
	if value.SavedHairLengthRID, err = r.br.ReadUInt64(); err != nil {
		return value, fmt.Errorf("read %s.savedHairLengthRid: %w", path, err)
	}
	if value.SavedHairLengths, err = readKCESPresetHairLengths(r, path+".savedHairLengths"); err != nil {
		return value, err
	}
	if value.SubProperties, err = readKCESPresetSubProperties(r, path+".subProperties", version, mpnName, depth); err != nil {
		return value, err
	}
	return value, nil
}

func readKCESPresetSavedTextureMap(r *kcesPresetInnerReader, path string, depth int64) ([]KCESPresetNamedSavedTexture, error) {
	present, err := r.br.ReadBool()
	if err != nil {
		return nil, fmt.Errorf("read %s presence: %w", path, err)
	}
	if !present {
		return nil, nil
	}
	count, err := r.readCount(path+" count", 2)
	if err != nil {
		return nil, err
	}
	result := makeKCESCountedSliceForAppend[KCESPresetNamedSavedTexture](uint64(count))
	for index := int64(0); index < count; index++ {
		itemPath := fmt.Sprintf("%s[%d]", path, index)
		key, err := r.readString(itemPath + ".key")
		if err != nil {
			return nil, err
		}
		value, err := readKCESPresetSavedTexture(r, itemPath+".value", depth)
		if err != nil {
			return nil, err
		}
		result = append(result, KCESPresetNamedSavedTexture{Key: key, Value: value})
	}
	return result, nil
}

func readKCESPresetSavedTexture(r *kcesPresetInnerReader, path string, depth int64) (KCESPresetSavedTextureData, error) {
	value := KCESPresetSavedTextureData{}
	var err error
	if value.UseLayer, err = r.br.ReadBool(); err != nil {
		return value, fmt.Errorf("read %s.useLayer: %w", path, err)
	}
	if value.UseMultiplyAlpha, err = r.br.ReadBool(); err != nil {
		return value, fmt.Errorf("read %s.useMultiplyAlpha: %w", path, err)
	}
	if value.MultiplyAlpha, err = readKCESPresetFloat32(r, path+".multiplyAlpha"); err != nil {
		return value, err
	}
	if value.Masks, err = readKCESPresetTextureMasks(r, path+".masks"); err != nil {
		return value, err
	}
	if value.Transforms, err = readKCESPresetTextureTransforms(r, path+".transforms", depth); err != nil {
		return value, err
	}
	infPresent, err := r.br.ReadBool()
	if err != nil {
		return value, fmt.Errorf("read %s.infinityColor presence: %w", path, err)
	}
	if infPresent {
		inf, err := readKCESPresetInfinityColor(r, path+".infinityColor", depth)
		if err != nil {
			return value, err
		}
		value.InfinityColor = &inf
	}
	if value.InfinityColorLinkLayer, err = r.readNullableString(path + ".infinityColorLinkLayer"); err != nil {
		return value, err
	}
	if value.UseAlphaMaskTransform, err = r.br.ReadBool(); err != nil {
		return value, fmt.Errorf("read %s.useAlphaMaskTransform: %w", path, err)
	}
	return value, nil
}

func readKCESPresetTextureMasks(r *kcesPresetInnerReader, path string) ([]KCESPresetTextureMask, error) {
	present, err := r.br.ReadBool()
	if err != nil {
		return nil, fmt.Errorf("read %s presence: %w", path, err)
	}
	if !present {
		return nil, nil
	}
	count, err := r.readCount(path+" count", 2)
	if err != nil {
		return nil, err
	}
	result := makeKCESCountedSliceForAppend[KCESPresetTextureMask](uint64(count))
	for index := int64(0); index < count; index++ {
		itemPath := fmt.Sprintf("%s[%d]", path, index)
		entry := KCESPresetTextureMask{}
		entry.Name, err = r.readNullableString(itemPath + ".name")
		if err != nil {
			return nil, err
		}
		entry.Mask, err = r.br.ReadBool()
		if err != nil {
			return nil, fmt.Errorf("read %s.mask: %w", itemPath, err)
		}
		result = append(result, entry)
	}
	return result, nil
}

func readKCESPresetTextureTransforms(r *kcesPresetInnerReader, path string, depth int64) ([]*KCESPresetTextureTransform, error) {
	present, err := r.br.ReadBool()
	if err != nil {
		return nil, fmt.Errorf("read %s presence: %w", path, err)
	}
	if !present {
		return nil, nil
	}
	count, err := r.readCount(path+" count", 49)
	if err != nil {
		return nil, err
	}
	result := makeKCESCountedSliceForAppend[*KCESPresetTextureTransform](uint64(count))
	for index := int64(0); index < count; index++ {
		value, err := readKCESPresetTextureTransform(r, fmt.Sprintf("%s[%d]", path, index), depth+1)
		if err != nil {
			return nil, err
		}
		result = append(result, &value)
	}
	return result, nil
}

func readKCESPresetTextureTransform(r *kcesPresetInnerReader, path string, depth int64) (KCESPresetTextureTransform, error) {
	value := KCESPresetTextureTransform{}
	if depth > maxKCESPresetInnerDepth {
		return value, fmt.Errorf("%s nesting exceeds limit %d", path, maxKCESPresetInnerDepth)
	}
	var err error
	if value.AreaUVDefault, err = readKCESPresetVector4(r, path+".areaUvDefault"); err != nil {
		return value, err
	}
	if value.ScaleDefault, err = readKCESPresetVector2(r, path+".scaleDefault"); err != nil {
		return value, err
	}
	if value.Position, err = readKCESPresetVector2(r, path+".position"); err != nil {
		return value, err
	}
	if value.Scale, err = readKCESPresetVector2(r, path+".scale"); err != nil {
		return value, err
	}
	if value.Rotation, err = readKCESPresetFloat32(r, path+".rotation"); err != nil {
		return value, err
	}
	if value.AreaUV, err = readKCESPresetVector4(r, path+".areaUv"); err != nil {
		return value, err
	}
	if value.SourcePixels, err = readKCESPresetVector2Int(r, path+".sourcePixels"); err != nil {
		return value, err
	}
	present, err := r.br.ReadBool()
	if err != nil {
		return value, fmt.Errorf("read %s.default presence: %w", path, err)
	}
	if present {
		child, err := readKCESPresetTextureTransform(r, path+".default", depth+1)
		if err != nil {
			return value, err
		}
		value.Default = &child
	}
	return value, nil
}

func readKCESPresetInfinityColor(r *kcesPresetInnerReader, path string, depth int64) (KCESPresetInfinityColorData, error) {
	value := KCESPresetInfinityColorData{}
	var err error
	if value.Independent, err = r.br.ReadBool(); err != nil {
		return value, fmt.Errorf("read %s.independent: %w", path, err)
	}
	if value.ColorType, err = r.readString(path + ".colorType"); err != nil {
		return value, err
	}
	if value.PartsColorType, err = r.readString(path + ".partsColorType"); err != nil {
		return value, err
	}
	if value.Color, err = readKCESPresetInfinityPartsColor(r, path+".color"); err != nil {
		return value, err
	}
	partsPresent, err := r.br.ReadBool()
	if err != nil {
		return value, fmt.Errorf("read %s.partColors presence: %w", path, err)
	}
	if partsPresent {
		count, err := r.readCount(path+".partColors count", 43)
		if err != nil {
			return value, err
		}
		value.PartColors = makeKCESCountedSliceForAppend[KCESPresetPartColorDef](uint64(count))
		for index := int64(0); index < count; index++ {
			itemPath := fmt.Sprintf("%s.partColors[%d]", path, index)
			entry, readErr := readKCESPresetPartColorDef(r, itemPath)
			if readErr != nil {
				err = readErr
				return value, err
			}
			value.PartColors = append(value.PartColors, entry)
		}
	}
	gradationPresent, err := r.br.ReadBool()
	if err != nil {
		return value, fmt.Errorf("read %s.gradation presence: %w", path, err)
	}
	if gradationPresent {
		gradation, err := readKCESPresetGradationDef(r, path+".gradation")
		if err != nil {
			return value, err
		}
		value.Gradation = &gradation
	}
	if value.GradationMugen, err = r.br.ReadBool(); err != nil {
		return value, fmt.Errorf("read %s.gradationMugen: %w", path, err)
	}
	return value, nil
}

func readKCESPresetInfinityPartsColor(r *kcesPresetInnerReader, path string) (KCESPresetInfinityPartsColor, error) {
	value := KCESPresetInfinityPartsColor{}
	fields := []*int32{&value.MainHue, &value.MainChroma, &value.MainBrightness, &value.MainContrast, &value.ShadowRate, &value.ShadowHue, &value.ShadowChroma, &value.ShadowBrightness, &value.ShadowContrast}
	names := []string{"mainHue", "mainChroma", "mainBrightness", "mainContrast", "shadowRate", "shadowHue", "shadowChroma", "shadowBrightness", "shadowContrast"}
	for index := range fields {
		field, err := r.br.ReadInt32()
		if err != nil {
			return value, fmt.Errorf("read %s.%s: %w", path, names[index], err)
		}
		*fields[index] = field
	}
	count, err := r.readCount(path+".gradation count", 36)
	if err != nil {
		return value, err
	}
	if count > 0 {
		value.Gradation = makeKCESCountedSliceForAppend[KCESPresetInfinityPartsColorPoint](uint64(count))
	}
	for index := int64(0); index < count; index++ {
		point, err := readKCESPresetInfinityPoint(r, fmt.Sprintf("%s.gradation[%d]", path, index))
		if err != nil {
			return value, err
		}
		value.Gradation = append(value.Gradation, point)
	}
	return value, nil
}

func readKCESPresetInfinityPoint(r *kcesPresetInnerReader, path string) (KCESPresetInfinityPartsColorPoint, error) {
	value := KCESPresetInfinityPartsColorPoint{}
	fields := []*int32{&value.MainHue, &value.MainChroma, &value.MainBrightness, &value.MainContrast, &value.ShadowRate, &value.ShadowHue, &value.ShadowChroma, &value.ShadowBrightness, &value.ShadowContrast}
	for index, field := range fields {
		v, err := r.br.ReadInt32()
		if err != nil {
			return value, fmt.Errorf("read %s field[%d]: %w", path, index, err)
		}
		*field = v
	}
	return value, nil
}

func readKCESPresetPartColorDef(r *kcesPresetInnerReader, path string) (KCESPresetPartColorDef, error) {
	value := KCESPresetPartColorDef{}
	var err error
	if value.PartName, err = r.readNullableString(path + ".partName"); err != nil {
		return value, err
	}
	if value.Color, err = readKCESPresetInfinityPartsColor(r, path+".color"); err != nil {
		return value, err
	}
	if value.PatternScale, err = readKCESPresetVector2(r, path+".patternScale"); err != nil {
		return value, err
	}
	if value.PatternRot, err = readKCESPresetFloat32(r, path+".patternRotation"); err != nil {
		return value, err
	}
	return value, nil
}

func readKCESPresetGradationDef(r *kcesPresetInnerReader, path string) (KCESPresetGradationColorDef, error) {
	value := KCESPresetGradationColorDef{}
	var err error
	if value.NotUse, err = r.readNullableString(path + ".notUse"); err != nil {
		return value, err
	}
	if value.PointCount, err = r.br.ReadInt32(); err != nil {
		return value, fmt.Errorf("read %s.pointCount: %w", path, err)
	}
	if value.Rates, err = readKCESPresetFloat32Array(r, path+".rates"); err != nil {
		return value, err
	}
	if value.Ranges, err = readKCESPresetVector4Array(r, path+".ranges"); err != nil {
		return value, err
	}
	if value.Color, err = readKCESPresetInfinityPartsColor(r, path+".color"); err != nil {
		return value, err
	}
	return value, nil
}

func readKCESPresetFloat32Array(r *kcesPresetInnerReader, path string) ([]float32, error) {
	present, err := r.br.ReadBool()
	if err != nil {
		return nil, fmt.Errorf("read %s presence: %w", path, err)
	}
	if !present {
		return nil, nil
	}
	count, err := r.readCount(path+" count", 4)
	if err != nil {
		return nil, err
	}
	result := makeKCESCountedSliceForAppend[float32](uint64(count))
	for index := int64(0); index < count; index++ {
		value, readErr := readKCESPresetFloat32(r, fmt.Sprintf("%s[%d]", path, index))
		err = readErr
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}

func readKCESPresetVector4Array(r *kcesPresetInnerReader, path string) ([]Vector4, error) {
	present, err := r.br.ReadBool()
	if err != nil {
		return nil, fmt.Errorf("read %s presence: %w", path, err)
	}
	if !present {
		return nil, nil
	}
	count, err := r.readCount(path+" count", 16)
	if err != nil {
		return nil, err
	}
	result := makeKCESCountedSliceForAppend[Vector4](uint64(count))
	for index := int64(0); index < count; index++ {
		value, readErr := readKCESPresetVector4(r, fmt.Sprintf("%s[%d]", path, index))
		err = readErr
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}

func readKCESPresetCutoutMask(r *kcesPresetInnerReader, path string) (*KCESPresetCutoutMask, error) {
	present, err := r.br.ReadBool()
	if err != nil {
		return nil, fmt.Errorf("read %s presence: %w", path, err)
	}
	if !present {
		return nil, nil
	}
	value := &KCESPresetCutoutMask{}
	if value.MaxLevel, err = r.br.ReadInt32(); err != nil {
		return nil, fmt.Errorf("read %s.maxLevel: %w", path, err)
	}
	if value.NowLevel, err = r.br.ReadInt32(); err != nil {
		return nil, fmt.Errorf("read %s.nowLevel: %w", path, err)
	}
	if value.Enabled, err = r.br.ReadBool(); err != nil {
		return nil, fmt.Errorf("read %s.enabled: %w", path, err)
	}
	return value, nil
}

func readKCESPresetPartHides(r *kcesPresetInnerReader, path string) ([]KCESPresetPartHide, error) {
	present, err := r.br.ReadBool()
	if err != nil {
		return nil, fmt.Errorf("read %s presence: %w", path, err)
	}
	if !present {
		return nil, nil
	}
	count, err := r.readCount(path+" count", 2)
	if err != nil {
		return nil, err
	}
	result := makeKCESCountedSliceForAppend[KCESPresetPartHide](uint64(count))
	for index := int64(0); index < count; index++ {
		itemPath := fmt.Sprintf("%s[%d]", path, index)
		entry := KCESPresetPartHide{}
		entry.PartName, err = r.readNullableString(itemPath + ".partName")
		if err != nil {
			return nil, err
		}
		entry.Enabled, err = r.br.ReadBool()
		if err != nil {
			return nil, fmt.Errorf("read %s.enabled: %w", itemPath, err)
		}
		result = append(result, entry)
	}
	return result, nil
}

func readKCESPresetSavedAttachPositions(r *kcesPresetInnerReader, path string) ([]SavedAttachData, error) {
	present, err := r.br.ReadBool()
	if err != nil {
		return nil, fmt.Errorf("read %s presence: %w", path, err)
	}
	if !present {
		return nil, nil
	}
	count, err := r.readCount(path+" count", minSavedAttachItemBytes)
	if err != nil {
		return nil, err
	}
	result := makeKCESCountedSliceForAppend[SavedAttachData](uint64(count))
	for index := int64(0); index < count; index++ {
		value, err := decodeSavedAttachItemWithSlotValidator(r.br, r.r, index, validateKCESPresetInnerString)
		if err != nil {
			return nil, fmt.Errorf("read %s[%d]: %w", path, index, err)
		}
		result = append(result, value)
	}
	return result, nil
}

func readKCESPresetHairLengths(r *kcesPresetInnerReader, path string) ([]KCESPresetSavedHairLength, error) {
	present, err := r.br.ReadBool()
	if err != nil {
		return nil, fmt.Errorf("read %s presence: %w", path, err)
	}
	if !present {
		return nil, nil
	}
	count, err := r.readCount(path+" count", 5)
	if err != nil {
		return nil, err
	}
	result := makeKCESCountedSliceForAppend[KCESPresetSavedHairLength](uint64(count))
	for index := int64(0); index < count; index++ {
		itemPath := fmt.Sprintf("%s[%d]", path, index)
		entry := KCESPresetSavedHairLength{}
		entry.PartName, err = r.readNullableString(itemPath + ".partName")
		if err != nil {
			return nil, err
		}
		entry.Value, err = readKCESPresetFloat32(r, itemPath+".value")
		if err != nil {
			return nil, err
		}
		result = append(result, entry)
	}
	return result, nil
}

func readKCESPresetSubProperties(r *kcesPresetInnerReader, path string, version int32, mpnName string, depth int64) ([]*KCESPresetSubProperty, error) {
	present, err := r.br.ReadBool()
	if err != nil {
		return nil, fmt.Errorf("read %s presence: %w", path, err)
	}
	if !present {
		return nil, nil
	}
	count, err := r.readCount(path+" count", 1)
	if err != nil {
		return nil, err
	}
	result := makeKCESCountedSliceForAppend[*KCESPresetSubProperty](uint64(count))
	for index := int64(0); index < count; index++ {
		itemPath := fmt.Sprintf("%s[%d]", path, index)
		exists, err := r.br.ReadBool()
		if err != nil {
			return nil, fmt.Errorf("read %s presence: %w", itemPath, err)
		}
		if !exists {
			result = append(result, nil)
			continue
		}
		value, err := readKCESPresetSubProperty(r, itemPath, version, mpnName, depth+1)
		if err != nil {
			return nil, err
		}
		result = append(result, &value)
	}
	return result, nil
}

func readKCESPresetSubProperty(r *kcesPresetInnerReader, path string, version int32, mpnName string, depth int64) (KCESPresetSubProperty, error) {
	value := KCESPresetSubProperty{}
	if depth > maxKCESPresetInnerDepth {
		return value, fmt.Errorf("%s nesting exceeds limit %d", path, maxKCESPresetInnerDepth)
	}
	var err error
	if value.Number, err = r.br.ReadInt32(); err != nil {
		return value, fmt.Errorf("read %s.number: %w", path, err)
	}
	if value.DefaultHokuroTattooSlotID, err = r.readString(path + ".defaultHokuroTattooSlotId"); err != nil {
		return value, err
	}
	editUnitData, err := r.readBlob(path + ".editUnitData")
	if err != nil {
		return value, err
	}
	if len(editUnitData) != 0 {
		value.EditUnitData, err = DecodeKCESPresetEditUnitData(editUnitData)
		if err != nil {
			return value, fmt.Errorf("decode %s.editUnitData: %w", path, err)
		}
	}
	if version >= 2001 {
		if value.SavedDefaultHokuroTattooRID, err = r.br.ReadUInt64(); err != nil {
			return value, fmt.Errorf("read %s.savedDefaultHokuroTattooRid: %w", path, err)
		}
	}
	if value.Base, err = readKCESPresetPropBase(r, path+".base", version, mpnName, depth); err != nil {
		return value, err
	}
	return value, nil
}

func EncodeKCESPresetPropertyData(value *KCESPresetPropertyList) ([]byte, error) {
	if value == nil {
		return nil, fmt.Errorf("nil KCES preset property data")
	}
	signature := value.Signature
	if signature != KCESPresetPropertyListSignature {
		return nil, fmt.Errorf("invalid KCES preset property-list signature %q", signature)
	}
	version := value.Version
	if err := validateKCESPresetInnerSliceLength(int64(len(value.Properties)), "KCES preset properties"); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	bw := stream.NewBinaryWriter(&out)
	if err := bw.WriteString(signature); err != nil {
		return nil, err
	}
	if err := bw.WriteInt32(version); err != nil {
		return nil, err
	}
	if err := bw.WriteInt32(int32(len(value.Properties))); err != nil {
		return nil, err
	}
	for index := range value.Properties {
		entry := &value.Properties[index]
		path := fmt.Sprintf("KCES preset properties[%d]", index)
		if err := validateKCESPresetInnerString(entry.Key, path+".key"); err != nil {
			return nil, err
		}
		if err := bw.WriteString(entry.Key); err != nil {
			return nil, err
		}
		if err := writeKCESPresetProperty(bw, &entry.Property, path+".property"); err != nil {
			return nil, err
		}
	}
	if err := bw.WriteBytes(value.TrailingData); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func NewKCESPresetPropertyList() *KCESPresetPropertyList {
	return &KCESPresetPropertyList{
		Signature: KCESPresetPropertyListSignature,
		Version:   KCESPresetPropertyListVersion,
	}
}

func writeKCESPresetProperty(bw *stream.BinaryWriter, value *KCESPresetProperty, path string) error {
	if value == nil {
		return fmt.Errorf("%s is nil", path)
	}
	signature := value.Signature
	if signature != KCESPresetPropertySignature {
		return fmt.Errorf("invalid %s.signature %q", path, signature)
	}
	version := value.Version
	if err := validateKCESPresetInnerString(value.Name, path+".name"); err != nil {
		return err
	}
	if err := validateKCESPresetInnerSliceLength(int64(len(value.MaterialProperties)), path+".materialProperties"); err != nil {
		return err
	}
	writeErrors := []error{
		bw.WriteString(signature), bw.WriteInt32(version), bw.WriteString(value.Name),
		bw.WriteInt32(value.DefaultValue), bw.WriteInt32(value.Value), bw.WriteInt32(value.TempValue),
		bw.WriteUInt64(value.FileNameRID), bw.WriteBool(value.Enabled), bw.WriteInt32(value.Max), bw.WriteInt32(value.Min),
		bw.WriteInt32(int32(len(value.MaterialProperties))),
	}
	for _, err := range writeErrors {
		if err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	for index := range value.MaterialProperties {
		if err := writeKCESPresetMaterialSlot(bw, &value.MaterialProperties[index], fmt.Sprintf("%s.materialProperties[%d]", path, index), version); err != nil {
			return err
		}
	}
	return writeKCESPresetPropBase(bw, &value.Base, path+".base", version, value.Name, 0)
}

func writeKCESPresetMaterialSlot(bw *stream.BinaryWriter, value *KCESPresetMaterialPropertySlot, path string, version int32) error {
	if err := validateKCESPresetInnerString(value.SlotID, path+".slotId"); err != nil {
		return err
	}
	if err := validateKCESPresetInnerSliceLength(int64(len(value.Properties)), path+".properties"); err != nil {
		return err
	}
	if version < 2002 && value.SlotValue != -1 {
		return fmt.Errorf("%s.slotValue %d cannot be encoded by property version %d; the wire value is implicitly -1", path, value.SlotValue, version)
	}
	if err := bw.WriteString(value.SlotID); err != nil {
		return fmt.Errorf("write %s.slotId: %w", path, err)
	}
	if version >= 2002 {
		if err := bw.WriteInt32(value.SlotValue); err != nil {
			return fmt.Errorf("write %s.slotValue: %w", path, err)
		}
	}
	if err := bw.WriteInt32(int32(len(value.Properties))); err != nil {
		return fmt.Errorf("write %s count: %w", path, err)
	}
	for index := range value.Properties {
		entry := &value.Properties[index]
		itemPath := fmt.Sprintf("%s.properties[%d]", path, index)
		if err := validateKCESPresetInnerString(entry.Key, itemPath+".key"); err != nil {
			return err
		}
		if err := bw.WriteString(entry.Key); err != nil {
			return err
		}
		if err := bw.WriteUInt64(entry.RID); err != nil {
			return err
		}
		if err := writeKCESPresetMaterialValue(bw, &entry.Property, itemPath+".property"); err != nil {
			return err
		}
	}
	return nil
}

func writeKCESPresetMaterialValue(bw *stream.BinaryWriter, value *KCESPresetMaterialPropertyValue, path string) error {
	for _, field := range []struct {
		name  string
		value *string
	}{{"propertyName", value.PropertyName}, {"typeName", value.TypeName}, {"value", value.Value}} {
		if err := validateKCESPresetInnerNullableString(field.value, path+"."+field.name); err != nil {
			return err
		}
	}
	if err := bw.WriteInt32(value.MaterialNumber); err != nil {
		return err
	}
	for _, field := range []*string{value.PropertyName, value.TypeName, value.Value} {
		if err := writeKCESPresetInnerNullableString(bw, field); err != nil {
			return err
		}
	}
	return nil
}

func writeKCESPresetPropBase(bw *stream.BinaryWriter, value *KCESPresetPropBase, path string, version int32, mpnName string, depth int64) error {
	if value == nil {
		return fmt.Errorf("%s is nil", path)
	}
	if depth > maxKCESPresetInnerDepth {
		return fmt.Errorf("%s nesting exceeds limit %d", path, maxKCESPresetInnerDepth)
	}
	if err := validateKCESPresetInnerString(value.Type, path+".type"); err != nil {
		return err
	}
	if err := validateKCESPresetInnerString(value.SubType, path+".subType"); err != nil {
		return err
	}
	if err := validateKCESPresetInnerNullableString(value.FileName, path+".fileName"); err != nil {
		return err
	}
	editBaseData, err := encodeKCESPresetEditBaseDataBlock(value.EditBaseData)
	if err != nil {
		return fmt.Errorf("encode %s.editBaseData: %w", path, err)
	}
	if err := validateKCESPresetInnerBlob(editBaseData, path+".editBaseData"); err != nil {
		return err
	}
	if err := bw.WriteInt32(value.Index); err != nil {
		return err
	}
	if err := bw.WriteString(value.Type); err != nil {
		return err
	}
	if err := bw.WriteString(value.SubType); err != nil {
		return err
	}
	if err := writeKCESPresetInnerNullableString(bw, value.FileName); err != nil {
		return err
	}
	for _, err := range []error{
		bw.WriteUInt64(value.FileNameRID), bw.WriteBool(value.Enabled), bw.WriteUInt64(value.BeforeFileNameRID),
		bw.WriteUInt64(value.Defines), bw.WriteUInt64(value.SavedTextureDataRID), bw.WriteUInt64(value.SavedTextureDataDefines),
	} {
		if err != nil {
			return err
		}
	}
	if err := writeKCESPresetSavedTextureMap(bw, value.SavedTextureData, path+".savedTextureData", depth); err != nil {
		return err
	}
	if err := bw.WriteBool(value.ShareInfinityColorData); err != nil {
		return err
	}
	if err := writeKCESPresetInnerBlob(bw, editBaseData); err != nil {
		return err
	}
	if err := bw.WriteUInt64(value.SavedCutoutMaskRID); err != nil {
		return err
	}
	if err := writeKCESPresetCutoutMask(bw, value.SavedCutoutMask); err != nil {
		return err
	}
	if err := bw.WriteUInt64(value.SavedPartHideRID); err != nil {
		return err
	}
	if err := writeKCESPresetPartHides(bw, value.SavedPartHide, path+".savedPartHide"); err != nil {
		return err
	}
	if err := bw.WriteBool(value.UsePartHide); err != nil {
		return err
	}
	if err := bw.WriteUInt64(value.SavedAttachPositionRID); err != nil {
		return err
	}
	if err := writeKCESPresetSavedAttachPositions(bw, value.SavedAttachPositions, path+".savedAttachPositions"); err != nil {
		return err
	}
	if err := bw.WriteBool(value.NoScale); err != nil {
		return err
	}
	if err := bw.WriteBool(value.SubPropertyIsTuftTexture); err != nil {
		return err
	}
	if err := bw.WriteUInt64(value.SavedHairLengthRID); err != nil {
		return err
	}
	if err := writeKCESPresetHairLengths(bw, value.SavedHairLengths, path+".savedHairLengths"); err != nil {
		return err
	}
	return writeKCESPresetSubProperties(bw, value.SubProperties, path+".subProperties", version, mpnName, depth)
}

func writeKCESPresetSavedTextureMap(bw *stream.BinaryWriter, values []KCESPresetNamedSavedTexture, path string, depth int64) error {
	if err := bw.WriteBool(values != nil); err != nil {
		return err
	}
	if values == nil {
		return nil
	}
	if err := validateKCESPresetInnerSliceLength(int64(len(values)), path); err != nil {
		return err
	}
	if err := bw.WriteInt32(int32(len(values))); err != nil {
		return err
	}
	for index := range values {
		entry := &values[index]
		itemPath := fmt.Sprintf("%s[%d]", path, index)
		if err := validateKCESPresetInnerString(entry.Key, itemPath+".key"); err != nil {
			return err
		}
		if err := bw.WriteString(entry.Key); err != nil {
			return err
		}
		if err := writeKCESPresetSavedTexture(bw, &entry.Value, itemPath+".value", depth); err != nil {
			return err
		}
	}
	return nil
}

func writeKCESPresetSavedTexture(bw *stream.BinaryWriter, value *KCESPresetSavedTextureData, path string, depth int64) error {
	if err := validateKCESPresetInnerNullableString(value.InfinityColorLinkLayer, path+".infinityColorLinkLayer"); err != nil {
		return err
	}
	if err := bw.WriteBool(value.UseLayer); err != nil {
		return err
	}
	if err := bw.WriteBool(value.UseMultiplyAlpha); err != nil {
		return err
	}
	if err := bw.WriteFloat32(value.MultiplyAlpha); err != nil {
		return err
	}
	if err := writeKCESPresetTextureMasks(bw, value.Masks, path+".masks"); err != nil {
		return err
	}
	if err := writeKCESPresetTextureTransforms(bw, value.Transforms, path+".transforms", depth); err != nil {
		return err
	}
	if err := bw.WriteBool(value.InfinityColor != nil); err != nil {
		return err
	}
	if value.InfinityColor != nil {
		if err := writeKCESPresetInfinityColor(bw, value.InfinityColor, path+".infinityColor", depth); err != nil {
			return err
		}
	}
	if err := writeKCESPresetInnerNullableString(bw, value.InfinityColorLinkLayer); err != nil {
		return err
	}
	return bw.WriteBool(value.UseAlphaMaskTransform)
}

func writeKCESPresetTextureMasks(bw *stream.BinaryWriter, values []KCESPresetTextureMask, path string) error {
	if err := bw.WriteBool(values != nil); err != nil {
		return err
	}
	if values == nil {
		return nil
	}
	if err := validateKCESPresetInnerSliceLength(int64(len(values)), path); err != nil {
		return err
	}
	if err := bw.WriteInt32(int32(len(values))); err != nil {
		return err
	}
	for index := range values {
		if err := validateKCESPresetInnerNullableString(values[index].Name, fmt.Sprintf("%s[%d].name", path, index)); err != nil {
			return err
		}
		if err := writeKCESPresetInnerNullableString(bw, values[index].Name); err != nil {
			return err
		}
		if err := bw.WriteBool(values[index].Mask); err != nil {
			return err
		}
	}
	return nil
}

func writeKCESPresetTextureTransforms(bw *stream.BinaryWriter, values []*KCESPresetTextureTransform, path string, depth int64) error {
	if err := bw.WriteBool(values != nil); err != nil {
		return err
	}
	if values == nil {
		return nil
	}
	if err := validateKCESPresetInnerSliceLength(int64(len(values)), path); err != nil {
		return err
	}
	if err := bw.WriteInt32(int32(len(values))); err != nil {
		return err
	}
	for index, value := range values {
		if value == nil {
			return fmt.Errorf("%s[%d] is nil; TransTexData list entries are not nullable", path, index)
		}
		if err := writeKCESPresetTextureTransform(bw, value, fmt.Sprintf("%s[%d]", path, index), depth+1); err != nil {
			return err
		}
	}
	return nil
}

func writeKCESPresetTextureTransform(bw *stream.BinaryWriter, value *KCESPresetTextureTransform, path string, depth int64) error {
	if depth > maxKCESPresetInnerDepth {
		return fmt.Errorf("%s nesting exceeds limit %d", path, maxKCESPresetInnerDepth)
	}
	if err := writeKCESPresetVector4(bw, value.AreaUVDefault); err != nil {
		return err
	}
	if err := writeKCESPresetVector2(bw, value.ScaleDefault); err != nil {
		return err
	}
	if err := writeKCESPresetVector2(bw, value.Position); err != nil {
		return err
	}
	if err := writeKCESPresetVector2(bw, value.Scale); err != nil {
		return err
	}
	if err := bw.WriteFloat32(value.Rotation); err != nil {
		return err
	}
	if err := writeKCESPresetVector4(bw, value.AreaUV); err != nil {
		return err
	}
	if err := writeKCESPresetVector2Int(bw, value.SourcePixels); err != nil {
		return err
	}
	if err := bw.WriteBool(value.Default != nil); err != nil {
		return err
	}
	if value.Default != nil {
		return writeKCESPresetTextureTransform(bw, value.Default, path+".default", depth+1)
	}
	return nil
}

func writeKCESPresetInfinityColor(bw *stream.BinaryWriter, value *KCESPresetInfinityColorData, path string, depth int64) error {
	if err := validateKCESPresetInnerString(value.ColorType, path+".colorType"); err != nil {
		return err
	}
	if err := validateKCESPresetInnerString(value.PartsColorType, path+".partsColorType"); err != nil {
		return err
	}
	if err := validateKCESPresetInnerSliceLength(int64(len(value.PartColors)), path+".partColors"); err != nil {
		return err
	}
	if err := bw.WriteBool(value.Independent); err != nil {
		return err
	}
	if err := bw.WriteString(value.ColorType); err != nil {
		return err
	}
	if err := bw.WriteString(value.PartsColorType); err != nil {
		return err
	}
	if err := writeKCESPresetInfinityPartsColor(bw, &value.Color, path+".color"); err != nil {
		return err
	}
	if err := bw.WriteBool(value.PartColors != nil); err != nil {
		return err
	}
	if value.PartColors != nil {
		if err := bw.WriteInt32(int32(len(value.PartColors))); err != nil {
			return err
		}
		for index := range value.PartColors {
			if err := writeKCESPresetPartColorDef(bw, &value.PartColors[index], fmt.Sprintf("%s.partColors[%d]", path, index)); err != nil {
				return err
			}
		}
	}
	if err := bw.WriteBool(value.Gradation != nil); err != nil {
		return err
	}
	if value.Gradation != nil {
		if err := writeKCESPresetGradationDef(bw, value.Gradation, path+".gradation"); err != nil {
			return err
		}
	}
	return bw.WriteBool(value.GradationMugen)
}

func writeKCESPresetInfinityPartsColor(bw *stream.BinaryWriter, value *KCESPresetInfinityPartsColor, path string) error {
	if err := validateKCESPresetInnerSliceLength(int64(len(value.Gradation)), path+".gradation"); err != nil {
		return err
	}
	for _, field := range []int32{value.MainHue, value.MainChroma, value.MainBrightness, value.MainContrast, value.ShadowRate, value.ShadowHue, value.ShadowChroma, value.ShadowBrightness, value.ShadowContrast} {
		if err := bw.WriteInt32(field); err != nil {
			return err
		}
	}
	if err := bw.WriteInt32(int32(len(value.Gradation))); err != nil {
		return err
	}
	for index := range value.Gradation {
		if err := writeKCESPresetInfinityPoint(bw, &value.Gradation[index]); err != nil {
			return fmt.Errorf("write %s.gradation[%d]: %w", path, index, err)
		}
	}
	return nil
}

func writeKCESPresetInfinityPoint(bw *stream.BinaryWriter, value *KCESPresetInfinityPartsColorPoint) error {
	for _, field := range []int32{value.MainHue, value.MainChroma, value.MainBrightness, value.MainContrast, value.ShadowRate, value.ShadowHue, value.ShadowChroma, value.ShadowBrightness, value.ShadowContrast} {
		if err := bw.WriteInt32(field); err != nil {
			return err
		}
	}
	return nil
}

func writeKCESPresetPartColorDef(bw *stream.BinaryWriter, value *KCESPresetPartColorDef, path string) error {
	if err := validateKCESPresetInnerNullableString(value.PartName, path+".partName"); err != nil {
		return err
	}
	if err := writeKCESPresetInnerNullableString(bw, value.PartName); err != nil {
		return err
	}
	if err := writeKCESPresetInfinityPartsColor(bw, &value.Color, path+".color"); err != nil {
		return err
	}
	if err := writeKCESPresetVector2(bw, value.PatternScale); err != nil {
		return err
	}
	return bw.WriteFloat32(value.PatternRot)
}

func writeKCESPresetGradationDef(bw *stream.BinaryWriter, value *KCESPresetGradationColorDef, path string) error {
	if err := validateKCESPresetInnerNullableString(value.NotUse, path+".notUse"); err != nil {
		return err
	}
	if err := writeKCESPresetInnerNullableString(bw, value.NotUse); err != nil {
		return err
	}
	if err := bw.WriteInt32(value.PointCount); err != nil {
		return err
	}
	if err := writeKCESPresetFloat32Array(bw, value.Rates, path+".rates"); err != nil {
		return err
	}
	if err := writeKCESPresetVector4Array(bw, value.Ranges, path+".ranges"); err != nil {
		return err
	}
	return writeKCESPresetInfinityPartsColor(bw, &value.Color, path+".color")
}

func writeKCESPresetFloat32Array(bw *stream.BinaryWriter, values []float32, path string) error {
	if err := bw.WriteBool(values != nil); err != nil {
		return err
	}
	if values == nil {
		return nil
	}
	if err := validateKCESPresetInnerSliceLength(int64(len(values)), path); err != nil {
		return err
	}
	if err := bw.WriteInt32(int32(len(values))); err != nil {
		return err
	}
	for _, value := range values {
		if err := bw.WriteFloat32(value); err != nil {
			return err
		}
	}
	return nil
}

func writeKCESPresetVector4Array(bw *stream.BinaryWriter, values []Vector4, path string) error {
	if err := bw.WriteBool(values != nil); err != nil {
		return err
	}
	if values == nil {
		return nil
	}
	if err := validateKCESPresetInnerSliceLength(int64(len(values)), path); err != nil {
		return err
	}
	if err := bw.WriteInt32(int32(len(values))); err != nil {
		return err
	}
	for _, value := range values {
		if err := writeKCESPresetVector4(bw, value); err != nil {
			return err
		}
	}
	return nil
}

func writeKCESPresetCutoutMask(bw *stream.BinaryWriter, value *KCESPresetCutoutMask) error {
	if err := bw.WriteBool(value != nil); err != nil {
		return err
	}
	if value == nil {
		return nil
	}
	if err := bw.WriteInt32(value.MaxLevel); err != nil {
		return err
	}
	if err := bw.WriteInt32(value.NowLevel); err != nil {
		return err
	}
	return bw.WriteBool(value.Enabled)
}

func writeKCESPresetPartHides(bw *stream.BinaryWriter, values []KCESPresetPartHide, path string) error {
	if err := bw.WriteBool(values != nil); err != nil {
		return err
	}
	if values == nil {
		return nil
	}
	if err := validateKCESPresetInnerSliceLength(int64(len(values)), path); err != nil {
		return err
	}
	if err := bw.WriteInt32(int32(len(values))); err != nil {
		return err
	}
	for index := range values {
		if err := validateKCESPresetInnerNullableString(values[index].PartName, fmt.Sprintf("%s[%d].partName", path, index)); err != nil {
			return err
		}
		if err := writeKCESPresetInnerNullableString(bw, values[index].PartName); err != nil {
			return err
		}
		if err := bw.WriteBool(values[index].Enabled); err != nil {
			return err
		}
	}
	return nil
}

func writeKCESPresetSavedAttachPositions(bw *stream.BinaryWriter, values []SavedAttachData, path string) error {
	if err := bw.WriteBool(values != nil); err != nil {
		return err
	}
	if values == nil {
		return nil
	}
	if err := validateKCESPresetInnerSliceLength(int64(len(values)), path); err != nil {
		return err
	}
	if err := bw.WriteInt32(int32(len(values))); err != nil {
		return err
	}
	for index := range values {
		if err := encodeSavedAttachItemWithSlotValidator(bw, &values[index], int64(index), validateKCESPresetInnerString); err != nil {
			return fmt.Errorf("write %s[%d]: %w", path, index, err)
		}
	}
	return nil
}

func writeKCESPresetHairLengths(bw *stream.BinaryWriter, values []KCESPresetSavedHairLength, path string) error {
	if err := bw.WriteBool(values != nil); err != nil {
		return err
	}
	if values == nil {
		return nil
	}
	if err := validateKCESPresetInnerSliceLength(int64(len(values)), path); err != nil {
		return err
	}
	if err := bw.WriteInt32(int32(len(values))); err != nil {
		return err
	}
	for index := range values {
		if err := validateKCESPresetInnerNullableString(values[index].PartName, fmt.Sprintf("%s[%d].partName", path, index)); err != nil {
			return err
		}
		if err := writeKCESPresetInnerNullableString(bw, values[index].PartName); err != nil {
			return err
		}
		if err := bw.WriteFloat32(values[index].Value); err != nil {
			return err
		}
	}
	return nil
}

func writeKCESPresetSubProperties(bw *stream.BinaryWriter, values []*KCESPresetSubProperty, path string, version int32, mpnName string, depth int64) error {
	if err := bw.WriteBool(values != nil); err != nil {
		return err
	}
	if values == nil {
		return nil
	}
	if err := validateKCESPresetInnerSliceLength(int64(len(values)), path); err != nil {
		return err
	}
	if err := bw.WriteInt32(int32(len(values))); err != nil {
		return err
	}
	for index, value := range values {
		if err := bw.WriteBool(value != nil); err != nil {
			return err
		}
		if value == nil {
			continue
		}
		if err := writeKCESPresetSubProperty(bw, value, fmt.Sprintf("%s[%d]", path, index), version, mpnName, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func writeKCESPresetSubProperty(bw *stream.BinaryWriter, value *KCESPresetSubProperty, path string, version int32, mpnName string, depth int64) error {
	if depth > maxKCESPresetInnerDepth {
		return fmt.Errorf("%s nesting exceeds limit %d", path, maxKCESPresetInnerDepth)
	}
	if version < 2001 && value.SavedDefaultHokuroTattooRID != 0 {
		return fmt.Errorf("%s.savedDefaultHokuroTattooRid %d cannot be encoded by property version %d", path, value.SavedDefaultHokuroTattooRID, version)
	}
	if err := validateKCESPresetInnerString(value.DefaultHokuroTattooSlotID, path+".defaultHokuroTattooSlotId"); err != nil {
		return err
	}
	editUnitData, err := encodeKCESPresetEditUnitDataBlock(value.EditUnitData)
	if err != nil {
		return fmt.Errorf("encode %s.editUnitData: %w", path, err)
	}
	if err := validateKCESPresetInnerBlob(editUnitData, path+".editUnitData"); err != nil {
		return err
	}
	if err := bw.WriteInt32(value.Number); err != nil {
		return err
	}
	if err := bw.WriteString(value.DefaultHokuroTattooSlotID); err != nil {
		return err
	}
	if err := writeKCESPresetInnerBlob(bw, editUnitData); err != nil {
		return err
	}
	if version >= 2001 {
		if err := bw.WriteUInt64(value.SavedDefaultHokuroTattooRID); err != nil {
			return err
		}
	}
	return writeKCESPresetPropBase(bw, &value.Base, path+".base", version, mpnName, depth)
}

func encodeKCESPresetEditBaseDataBlock(value *KCESPresetEditBaseData) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	return EncodeKCESPresetEditBaseData(value)
}

func encodeKCESPresetEditUnitDataBlock(value *KCESPresetEditUnitData) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	return EncodeKCESPresetEditUnitData(value)
}
