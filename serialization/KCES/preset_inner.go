package KCES

import (
	"bytes"
	"fmt"
	"math"
	"unicode/utf8"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
)

// .preset 的 maiddata 内部 BinaryWriter 模型，定义属性列表、颜色数据和身体数据的签名、版本及结构。
//
// BinaryWriter models inside .preset maiddata, defining signatures, versions, and structures for property, color, and body data.

const (
	KCESPresetPropertyListSignature       = "GP03_MPROP_LIST"
	KCESPresetPropertyListVersion   int32 = 1270
	KCESPresetPropertySignature           = "GP03_MPROP"
	KCESPresetPropertyVersion       int32 = 2100
	KCESPresetColorSignature              = "CM3D2_MULTI_COL"
	KCESPresetColorVersion          int32 = 1270
	KCESPresetBodySignature               = "CM3D2_MAID_BODY"
	KCESPresetBodyVersion           int32 = 1270

	maxKCESPresetInnerDepth = 64
)

// KCESPresetPropertyList is the GP03_MPROP_LIST block stored in
// MaidPresetCore.propData. A slice is used instead of a map so the binary
// dictionary enumeration order and even game-readable repeated keys survive a
// JSON round trip.
type KCESPresetPropertyList struct {
	Signature    string                    `json:"signature"`
	Version      int32                     `json:"version"`
	Properties   []KCESPresetNamedProperty `json:"properties"`
	TrailingData []byte                    `json:"trailingData,omitempty"`
}

type KCESPresetNamedProperty struct {
	Key      string             `json:"key"`
	Property KCESPresetProperty `json:"property"`
}

// KCESPresetProperty is MaidProp.Serialize's current v2100 wire followed by
// the inherited PropBase binary block.
type KCESPresetProperty struct {
	Signature          string                           `json:"signature"`
	Version            int32                            `json:"version"`
	Name               string                           `json:"name"`
	DefaultValue       int32                            `json:"defaultValue"`
	Value              int32                            `json:"value"`
	TempValue          int32                            `json:"tempValue"`
	FileNameRID        uint64                           `json:"fileNameRid"`
	Enabled            bool                             `json:"enabled"`
	Max                int32                            `json:"max"`
	Min                int32                            `json:"min"`
	MaterialProperties []KCESPresetMaterialPropertySlot `json:"materialProperties"`
	Base               KCESPresetPropBase               `json:"base"`
}

type KCESPresetMaterialPropertySlot struct {
	SlotID     string                            `json:"slotId"`
	SlotValue  int32                             `json:"slotValue"`
	Properties []KCESPresetNamedMaterialProperty `json:"properties"`
}

type KCESPresetNamedMaterialProperty struct {
	Key      string                          `json:"key"`
	RID      uint64                          `json:"rid"`
	Property KCESPresetMaterialPropertyValue `json:"property"`
}

type KCESPresetMaterialPropertyValue struct {
	MaterialNumber int32   `json:"materialNumber"`
	PropertyName   *string `json:"propertyName"`
	TypeName       *string `json:"typeName"`
	Value          *string `json:"value"`
}

// KCESPresetPropBase preserves the current PropBase.Serialize layout. Nil and
// non-nil empty slices are intentionally distinct because the wire writes a
// presence Boolean before each collection.
type KCESPresetPropBase struct {
	Index                    int32                         `json:"index"`
	Type                     string                        `json:"type"`
	SubType                  string                        `json:"subType"`
	FileName                 *string                       `json:"fileName"`
	FileNameRID              uint64                        `json:"fileNameRid"`
	Enabled                  bool                          `json:"enabled"`
	BeforeFileNameRID        uint64                        `json:"beforeFileNameRid"`
	Defines                  uint64                        `json:"defines"`
	SavedTextureDataRID      uint64                        `json:"savedTextureDataRid"`
	SavedTextureDataDefines  uint64                        `json:"savedTextureDataDefines"`
	SavedTextureData         []KCESPresetNamedSavedTexture `json:"savedTextureData"`
	ShareInfinityColorData   bool                          `json:"shareInfinityColorData"`
	EditBaseData             *KCESPresetEditBaseData       `json:"editBaseData"`
	SavedCutoutMaskRID       uint64                        `json:"savedCutoutMaskRid"`
	SavedCutoutMask          *KCESPresetCutoutMask         `json:"savedCutoutMask"`
	SavedPartHideRID         uint64                        `json:"savedPartHideRid"`
	SavedPartHide            []KCESPresetPartHide          `json:"savedPartHide"`
	UsePartHide              bool                          `json:"usePartHide"`
	SavedAttachPositionRID   uint64                        `json:"savedAttachPositionRid"`
	SavedAttachPositions     []SavedAttachData             `json:"savedAttachPositions"`
	NoScale                  bool                          `json:"noScale"`
	SubPropertyIsTuftTexture bool                          `json:"subPropertyIsTuftTexture"`
	SavedHairLengthRID       uint64                        `json:"savedHairLengthRid"`
	SavedHairLengths         []KCESPresetSavedHairLength   `json:"savedHairLengths"`
	SubProperties            []*KCESPresetSubProperty      `json:"subProperties"`
}

type KCESPresetNamedSavedTexture struct {
	Key   string                     `json:"key"`
	Value KCESPresetSavedTextureData `json:"value"`
}

type KCESPresetSavedTextureData struct {
	UseLayer               bool                          `json:"useLayer"`
	UseMultiplyAlpha       bool                          `json:"useMultiplyAlpha"`
	MultiplyAlpha          float32                       `json:"multiplyAlpha"`
	Masks                  []KCESPresetTextureMask       `json:"masks"`
	Transforms             []*KCESPresetTextureTransform `json:"transforms"`
	InfinityColor          *KCESPresetInfinityColorData  `json:"infinityColor"`
	InfinityColorLinkLayer *string                       `json:"infinityColorLinkLayer"`
	UseAlphaMaskTransform  bool                          `json:"useAlphaMaskTransform"`
}

type KCESPresetTextureMask struct {
	Name *string `json:"name"`
	Mask bool    `json:"mask"`
}

type KCESPresetTextureTransform struct {
	AreaUVDefault Vector4                     `json:"areaUvDefault"`
	ScaleDefault  Vector2                     `json:"scaleDefault"`
	Position      Vector2                     `json:"position"`
	Scale         Vector2                     `json:"scale"`
	Rotation      float32                     `json:"rotation"`
	AreaUV        Vector4                     `json:"areaUv"`
	SourcePixels  Vector2Int                  `json:"sourcePixels"`
	Default       *KCESPresetTextureTransform `json:"default"`
}

type KCESPresetInfinityColorData struct {
	Independent    bool                         `json:"independent"`
	ColorType      string                       `json:"colorType"`
	PartsColorType string                       `json:"partsColorType"`
	Color          KCESPresetInfinityPartsColor `json:"color"`
	PartColors     []KCESPresetPartColorDef     `json:"partColors"`
	Gradation      *KCESPresetGradationColorDef `json:"gradation"`
	GradationMugen bool                         `json:"gradationMugen"`
}

type KCESPresetInfinityPartsColor struct {
	MainHue          int32                               `json:"mainHue"`
	MainChroma       int32                               `json:"mainChroma"`
	MainBrightness   int32                               `json:"mainBrightness"`
	MainContrast     int32                               `json:"mainContrast"`
	ShadowRate       int32                               `json:"shadowRate"`
	ShadowHue        int32                               `json:"shadowHue"`
	ShadowChroma     int32                               `json:"shadowChroma"`
	ShadowBrightness int32                               `json:"shadowBrightness"`
	ShadowContrast   int32                               `json:"shadowContrast"`
	Gradation        []KCESPresetInfinityPartsColorPoint `json:"gradation"`
}

type KCESPresetInfinityPartsColorPoint struct {
	MainHue          int32 `json:"mainHue"`
	MainChroma       int32 `json:"mainChroma"`
	MainBrightness   int32 `json:"mainBrightness"`
	MainContrast     int32 `json:"mainContrast"`
	ShadowRate       int32 `json:"shadowRate"`
	ShadowHue        int32 `json:"shadowHue"`
	ShadowChroma     int32 `json:"shadowChroma"`
	ShadowBrightness int32 `json:"shadowBrightness"`
	ShadowContrast   int32 `json:"shadowContrast"`
}

type KCESPresetPartColorDef struct {
	PartName     *string                      `json:"partName"`
	Color        KCESPresetInfinityPartsColor `json:"color"`
	PatternScale Vector2                      `json:"patternScale"`
	PatternRot   float32                      `json:"patternRotation"`
}

type KCESPresetGradationColorDef struct {
	NotUse     *string                      `json:"notUse"`
	PointCount int32                        `json:"pointCount"`
	Rates      []float32                    `json:"rates"`
	Ranges     []Vector4                    `json:"ranges"`
	Color      KCESPresetInfinityPartsColor `json:"color"`
}

type KCESPresetCutoutMask struct {
	MaxLevel int32 `json:"maxLevel"`
	NowLevel int32 `json:"nowLevel"`
	Enabled  bool  `json:"enabled"`
}

type KCESPresetPartHide struct {
	PartName *string `json:"partName"`
	Enabled  bool    `json:"enabled"`
}

type KCESPresetSavedHairLength struct {
	PartName *string `json:"partName"`
	Value    float32 `json:"value"`
}

type KCESPresetSubProperty struct {
	Number                      int32                   `json:"number"`
	DefaultHokuroTattooSlotID   string                  `json:"defaultHokuroTattooSlotId"`
	EditUnitData                *KCESPresetEditUnitData `json:"editUnitData"`
	SavedDefaultHokuroTattooRID uint64                  `json:"savedDefaultHokuroTattooRid"`
	Base                        KCESPresetPropBase      `json:"base"`
}

// KCESPresetColorData is MaidInfinityColor.Serialize's preset block. Current
// v1270 writes only the 22 enum names and a MAX terminator; actual customized
// colors live in PropBase.savedTexDatas.
type KCESPresetColorData struct {
	Signature    string                  `json:"signature"`
	Version      int32                   `json:"version"`
	PartCount    int32                   `json:"partCount"`
	LegacyParts  []KCESPresetLegacyColor `json:"legacyParts,omitempty"`
	PartNames    []string                `json:"partNames,omitempty"`
	TrailingData []byte                  `json:"trailingData,omitempty"`
}

// KCESPresetLegacyColor is the pre-v1201 CM3D2_MULTI_COL entry written before
// KCES switched this block to a list of PARTS_COLOR names. No migration or
// default-color expansion is applied: these are exactly the ten wire values.
type KCESPresetLegacyColor struct {
	Use              bool  `json:"use"`
	MainHue          int32 `json:"mainHue"`
	MainChroma       int32 `json:"mainChroma"`
	MainBrightness   int32 `json:"mainBrightness"`
	MainContrast     int32 `json:"mainContrast"`
	ShadowRate       int32 `json:"shadowRate"`
	ShadowHue        int32 `json:"shadowHue"`
	ShadowChroma     int32 `json:"shadowChroma"`
	ShadowBrightness int32 `json:"shadowBrightness"`
	ShadowContrast   int32 `json:"shadowContrast"`
}

type KCESPresetBodyData struct {
	Signature    string `json:"signature"`
	Version      int32  `json:"version"`
	TrailingData []byte `json:"trailingData,omitempty"`
}

type kcesPresetInnerReader struct {
	r  *bytes.Reader
	br *stream.BinaryReader
}

func newKCESPresetInnerReader(data []byte) *kcesPresetInnerReader {
	r := bytes.NewReader(data)
	return &kcesPresetInnerReader{r: r, br: stream.NewBinaryReader(r)}
}

func (r *kcesPresetInnerReader) readString(path string) (string, error) {
	value, err := r.br.ReadString()
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	if !utf8.ValidString(value) {
		return "", fmt.Errorf("%s is not valid UTF-8", path)
	}
	return value, nil
}

func (r *kcesPresetInnerReader) readNullableString(path string) (*string, error) {
	present, err := r.br.ReadBool()
	if err != nil {
		return nil, fmt.Errorf("read %s presence: %w", path, err)
	}
	if !present {
		return nil, nil
	}
	value, err := r.readString(path)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func (r *kcesPresetInnerReader) readCount(path string, minimumBytes int) (int, error) {
	count, err := r.br.ReadInt32()
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", path, err)
	}
	if count < 0 {
		return 0, fmt.Errorf("negative %s %d", path, count)
	}
	if minimumBytes > 0 && int64(count) > int64(r.r.Len()/minimumBytes) {
		return 0, fmt.Errorf("%s %d cannot fit in %d remaining bytes", path, count, r.r.Len())
	}
	return int(count), nil
}

func (r *kcesPresetInnerReader) readBlob(path string) ([]byte, error) {
	length, err := r.br.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read %s length: %w", path, err)
	}
	if length < 0 {
		return nil, fmt.Errorf("negative %s length %d", path, length)
	}
	if int64(length) > int64(r.r.Len()) {
		return nil, fmt.Errorf("%s length %d exceeds %d remaining bytes", path, length, r.r.Len())
	}
	data, err := r.br.ReadBytes(int(length))
	if err != nil {
		return nil, fmt.Errorf("read %s data: %w", path, err)
	}
	return data, nil
}

func validateKCESPresetInnerString(value, path string) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("%s is not valid UTF-8", path)
	}
	if uint64(len(value)) > uint64(math.MaxInt32) {
		return fmt.Errorf("%s is %d bytes, exceeds Int32", path, len(value))
	}
	return nil
}

func validateKCESPresetInnerNullableString(value *string, path string) error {
	if value == nil {
		return nil
	}
	return validateKCESPresetInnerString(*value, path)
}

func writeKCESPresetInnerNullableString(bw *stream.BinaryWriter, value *string) error {
	if err := bw.WriteBool(value != nil); err != nil {
		return err
	}
	if value != nil {
		return bw.WriteString(*value)
	}
	return nil
}

func validateKCESPresetInnerSliceLength(length int, path string) error {
	if uint64(length) > uint64(math.MaxInt32) {
		return fmt.Errorf("%s length %d exceeds Int32", path, length)
	}
	return nil
}

func validateKCESPresetInnerBlob(data []byte, path string) error {
	if uint64(len(data)) > uint64(math.MaxInt32) {
		return fmt.Errorf("%s length %d exceeds Int32", path, len(data))
	}
	return nil
}

func writeKCESPresetInnerBlob(bw *stream.BinaryWriter, data []byte) error {
	if err := validateKCESPresetInnerBlob(data, "blob"); err != nil {
		return err
	}
	if err := bw.WriteInt32(int32(len(data))); err != nil {
		return err
	}
	return bw.WriteBytes(data)
}

func DecodeKCESPresetColorData(data []byte) (*KCESPresetColorData, error) {
	r := newKCESPresetInnerReader(data)
	signature, err := r.readString("KCES preset color signature")
	if err != nil {
		return nil, err
	}
	if signature != KCESPresetColorSignature {
		return nil, fmt.Errorf("invalid KCES preset color signature %q", signature)
	}
	version, err := r.br.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read KCES preset color version: %w", err)
	}
	partCount, err := r.br.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read KCES preset color partCount: %w", err)
	}
	if partCount < 0 {
		return nil, fmt.Errorf("negative KCES preset color partCount %d", partCount)
	}
	result := &KCESPresetColorData{Signature: signature, Version: version, PartCount: partCount}
	if version <= 1200 {
		const legacyColorBytes = 1 + 9*4
		if int64(partCount) > int64(r.r.Len()/legacyColorBytes) {
			return nil, fmt.Errorf("KCES preset legacy color partCount %d cannot fit in %d remaining bytes", partCount, r.r.Len())
		}
		result.LegacyParts = makeKCESCountedSliceForAppend[KCESPresetLegacyColor](uint64(partCount))
		for index := int32(0); index < partCount; index++ {
			entry := KCESPresetLegacyColor{}
			if entry.Use, err = r.br.ReadBool(); err != nil {
				return nil, fmt.Errorf("read KCES preset legacyParts[%d].use: %w", index, err)
			}
			fields := []*int32{&entry.MainHue, &entry.MainChroma, &entry.MainBrightness, &entry.MainContrast, &entry.ShadowRate, &entry.ShadowHue, &entry.ShadowChroma, &entry.ShadowBrightness, &entry.ShadowContrast}
			for fieldIndex, field := range fields {
				v, readErr := r.br.ReadInt32()
				if readErr != nil {
					return nil, fmt.Errorf("read KCES preset legacyParts[%d] field[%d]: %w", index, fieldIndex, readErr)
				}
				*field = v
			}
			result.LegacyParts = append(result.LegacyParts, entry)
		}
	} else {
		// The current game deliberately ignores partCount in this branch and
		// reads names until the exact, case-sensitive string "MAX". Preserve the
		// stored count independently instead of normalizing it to len(PartNames).
		for index := 0; ; index++ {
			name, readErr := r.readString(fmt.Sprintf("KCES preset color partNames[%d]", index))
			if readErr != nil {
				return nil, readErr
			}
			if name == "MAX" {
				break
			}
			result.PartNames = append(result.PartNames, name)
		}
	}
	if r.r.Len() != 0 {
		result.TrailingData, err = r.br.ReadBytes(r.r.Len())
		if err != nil {
			return nil, fmt.Errorf("read KCES preset color trailingData: %w", err)
		}
	}
	return result, nil
}

func EncodeKCESPresetColorData(value *KCESPresetColorData) ([]byte, error) {
	if value == nil {
		return nil, fmt.Errorf("nil KCES preset colorData")
	}
	signature := value.Signature
	if signature != KCESPresetColorSignature {
		return nil, fmt.Errorf("invalid KCES preset color signature %q", signature)
	}
	version := value.Version
	if err := validateKCESPresetInnerSliceLength(len(value.LegacyParts), "KCES preset color legacyParts"); err != nil {
		return nil, err
	}
	if err := validateKCESPresetInnerSliceLength(len(value.PartNames), "KCES preset color partNames"); err != nil {
		return nil, err
	}
	if value.PartCount < 0 {
		return nil, fmt.Errorf("negative KCES preset color partCount %d", value.PartCount)
	}
	if version <= 1200 {
		if int(value.PartCount) != len(value.LegacyParts) {
			return nil, fmt.Errorf("KCES preset legacy color partCount %d does not match legacyParts length %d", value.PartCount, len(value.LegacyParts))
		}
		if len(value.PartNames) != 0 {
			return nil, fmt.Errorf("KCES preset color version %d uses legacyParts, not partNames", version)
		}
	} else {
		if len(value.LegacyParts) != 0 {
			return nil, fmt.Errorf("KCES preset color version %d uses partNames, not legacyParts", version)
		}
		for index, name := range value.PartNames {
			if err := validateKCESPresetInnerString(name, fmt.Sprintf("KCES preset color partNames[%d]", index)); err != nil {
				return nil, err
			}
			if name == "MAX" {
				return nil, fmt.Errorf("KCES preset color partNames[%d] is the reserved terminator MAX", index)
			}
		}
	}
	var out bytes.Buffer
	bw := stream.NewBinaryWriter(&out)
	for _, writeErr := range []error{bw.WriteString(signature), bw.WriteInt32(version), bw.WriteInt32(value.PartCount)} {
		if writeErr != nil {
			return nil, writeErr
		}
	}
	if version <= 1200 {
		for index := range value.LegacyParts {
			entry := &value.LegacyParts[index]
			if err := bw.WriteBool(entry.Use); err != nil {
				return nil, err
			}
			for _, field := range []int32{entry.MainHue, entry.MainChroma, entry.MainBrightness, entry.MainContrast, entry.ShadowRate, entry.ShadowHue, entry.ShadowChroma, entry.ShadowBrightness, entry.ShadowContrast} {
				if err := bw.WriteInt32(field); err != nil {
					return nil, err
				}
			}
		}
	} else {
		for _, name := range value.PartNames {
			if err := bw.WriteString(name); err != nil {
				return nil, err
			}
		}
		if err := bw.WriteString("MAX"); err != nil {
			return nil, err
		}
	}
	if err := bw.WriteBytes(value.TrailingData); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func NewKCESPresetColorData() *KCESPresetColorData {
	return &KCESPresetColorData{
		Signature: KCESPresetColorSignature,
		Version:   KCESPresetColorVersion,
		PartCount: 22,
		PartNames: []string{"HAIR", "EYE_BROW", "UNDER_HAIR", "ASS_HAIR", "SKIN", "HAIR_OUTLINE", "SKIN_OUTLINE", "EYE_WHITE", "HOKURO", "TATOO", "SOBAKASU", "MATSUGE_UP", "MATSUGE_LOW", "FUTAE", "PART_COLOR", "GRADA_COLOR", "MAKE", "MUGEN_COLOR", "HIGE", "SHIMI", "SHIWA", "BODY_HAIR"},
	}
}

func DecodeKCESPresetBodyData(data []byte) (*KCESPresetBodyData, error) {
	r := newKCESPresetInnerReader(data)
	signature, err := r.readString("KCES preset body signature")
	if err != nil {
		return nil, err
	}
	if signature != KCESPresetBodySignature {
		return nil, fmt.Errorf("invalid KCES preset body signature %q", signature)
	}
	version, err := r.br.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read KCES preset body version: %w", err)
	}
	result := &KCESPresetBodyData{Signature: signature, Version: version}
	if r.r.Len() != 0 {
		result.TrailingData, err = r.br.ReadBytes(r.r.Len())
		if err != nil {
			return nil, fmt.Errorf("read KCES preset body trailingData: %w", err)
		}
	}
	return result, nil
}

func EncodeKCESPresetBodyData(value *KCESPresetBodyData) ([]byte, error) {
	if value == nil {
		return nil, fmt.Errorf("nil KCES preset bodyData")
	}
	signature := value.Signature
	version := value.Version
	if signature != KCESPresetBodySignature {
		return nil, fmt.Errorf("invalid KCES preset body signature %q", signature)
	}
	var out bytes.Buffer
	bw := stream.NewBinaryWriter(&out)
	if err := bw.WriteString(signature); err != nil {
		return nil, err
	}
	if err := bw.WriteInt32(version); err != nil {
		return nil, err
	}
	if err := bw.WriteBytes(value.TrailingData); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func NewKCESPresetBodyData() *KCESPresetBodyData {
	return &KCESPresetBodyData{Signature: KCESPresetBodySignature, Version: KCESPresetBodyVersion}
}

func readKCESPresetFloat32(r *kcesPresetInnerReader, path string) (float32, error) {
	value, err := r.br.ReadFloat32()
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", path, err)
	}
	return value, nil
}

func writeKCESPresetVector2(bw *stream.BinaryWriter, value Vector2) error {
	return bw.WriteFloat2([2]float32{value.X, value.Y})
}

func writeKCESPresetVector4(bw *stream.BinaryWriter, value Vector4) error {
	return bw.WriteFloat4([4]float32{value.X, value.Y, value.Z, value.W})
}

func readKCESPresetVector2(r *kcesPresetInnerReader, path string) (Vector2, error) {
	value, err := r.br.ReadFloat2()
	if err != nil {
		return Vector2{}, fmt.Errorf("read %s: %w", path, err)
	}
	return Vector2{X: value[0], Y: value[1]}, nil
}

func readKCESPresetVector4(r *kcesPresetInnerReader, path string) (Vector4, error) {
	value, err := r.br.ReadFloat4()
	if err != nil {
		return Vector4{}, fmt.Errorf("read %s: %w", path, err)
	}
	return Vector4{X: value[0], Y: value[1], Z: value[2], W: value[3]}, nil
}

func readKCESPresetVector2Int(r *kcesPresetInnerReader, path string) (Vector2Int, error) {
	x, err := r.br.ReadInt32()
	if err != nil {
		return Vector2Int{}, fmt.Errorf("read %s.x: %w", path, err)
	}
	y, err := r.br.ReadInt32()
	if err != nil {
		return Vector2Int{}, fmt.Errorf("read %s.y: %w", path, err)
	}
	return Vector2Int{X: int(x), Y: int(y)}, nil
}

func writeKCESPresetVector2Int(bw *stream.BinaryWriter, value Vector2Int) error {
	if err := requireInt32("KCES preset Vector2Int.x", value.X); err != nil {
		return err
	}
	if err := requireInt32("KCES preset Vector2Int.y", value.Y); err != nil {
		return err
	}
	if err := bw.WriteInt32(int32(value.X)); err != nil {
		return err
	}
	return bw.WriteInt32(int32(value.Y))
}
