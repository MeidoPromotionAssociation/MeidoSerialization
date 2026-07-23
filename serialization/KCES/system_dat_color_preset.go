package KCES

import (
	"encoding/binary"
	"fmt"
	"math"
	"unicode/utf8"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

// system.dat 内 color_preset 目录中自定义颜色预设虚拟文件的 MessagePack/LZ4 布局。
// 该载荷没有独立磁盘扩展名。
//
// MessagePack/LZ4 layout for custom color-preset virtual files below color_preset inside system.dat.
// This payload has no standalone disk extension.

const (
	// ColorPresetVersion is CustomColorPresetBase<T>.FixVersion in KCES 1.34.4.
	ColorPresetVersion = 1004
	// ColorPresetNoAssetMigrationMinVersion is the first root version for which
	// OnDeserializeVersionCheck does not inspect the installed hair preset menus.
	ColorPresetNoAssetMigrationMinVersion = 1003
	// ColorPresetPackVersion is CustomColorPresetColorPack.FixVersion.
	ColorPresetPackVersion = 1001
	// ColorPresetColorVersion is shared by FreeColor, LayerFreeColor, and
	// GradationColor in KCES 1.34.4.
	ColorPresetColorVersion = 1000

	colorPresetCompressionMinLength = 64
)

// ColorPresetPackType is CustomColorPresetColorPack.Type's Int32 wire value.
type ColorPresetPackType int32

const (
	ColorPresetPackColorAndAlpha ColorPresetPackType = iota
	ColorPresetPackOnlyColor
	ColorPresetPackOnlyAlpha
)

// ColorPreset models the common serialized base of MaidEdit.ColorPreset and
// MaidEdit.ColorPresetSlot. Those two derived classes add no MessagePack keys,
// so their bytes are identical.
//
// ID, BaseMenuFile, LayerName, and ViewName use pointers because the C# string
// formatter permits nil. InstanceGUID is a string for API compatibility, so
// this model cannot distinguish a nil wire string from an empty one. Ordinary
// decoding keeps either representation as the empty string and deliberately
// does not reproduce the game's Guid.NewGuid callback. A non-empty wire value
// is opaque; the game never passes it through Guid.Parse.
type ColorPreset struct {
	MessagePackRootMetadata
	Version           int32                   `json:"version"`
	ID                *string                 `json:"id"`
	BaseMenuFile      *string                 `json:"baseMenuFile"`
	UserCreated       bool                    `json:"userCreated"`
	IsAdvancedMode    bool                    `json:"isAdvancedMode"`
	ColorPackList     []*ColorPresetColorPack `json:"colorPackList"`
	InstanceGUID      string                  `json:"instanceGuid"`
	InstanceGUIDIsNil bool                    `json:"instanceGuidIsNil,omitempty"`
	FieldCount        *int32                  `json:"fieldCount,omitempty"`
	FutureSlots       [][]byte                `json:"futureSlots,omitempty"`
}

// ColorPresetSlot is a wire alias: ColorPresetSlot has no additional keys.
type ColorPresetSlot = ColorPreset

// ColorPresetColorPack preserves every serialized field, including the
// private mpnNames and allowedMpnOverRide members. The game's CopyTo method
// accidentally omits allowedMpnOverRide; this codec models the wire itself and
// therefore does not discard it.
type ColorPresetColorPack struct {
	Version            int32                        `json:"version"`
	MPNs               []int32                      `json:"mpns"`
	LayerName          *string                      `json:"layerName"`
	ViewName           *string                      `json:"viewName"`
	Type               ColorPresetPackType          `json:"type"`
	ColorList          []*ColorPresetLayerFreeColor `json:"colorList"`
	GradationColorList []*ColorPresetGradationColor `json:"gradationColorList"`
	Alpha              float32                      `json:"alpha"`
	AllowedMPNOverride bool                         `json:"allowedMpnOverRide"`
	MPNNames           []string                     `json:"mpnNames"`
	MPNNameNulls       []bool                       `json:"mpnNameNulls,omitempty"`
	FieldCount         *int32                       `json:"fieldCount,omitempty"`
	FutureSlots        [][]byte                     `json:"futureSlots,omitempty"`
}

// ColorPresetFreeColor exposes FreeColor's four private raw fields. They are
// not passed through the clamping public properties by the private resolver.
type ColorPresetFreeColor struct {
	Version     int32    `json:"version"`
	Hue         int32    `json:"hue"`
	Saturation  int32    `json:"saturation"`
	Brightness  int32    `json:"brightness"`
	Contrast    int32    `json:"contrast"`
	FieldCount  *int32   `json:"fieldCount,omitempty"`
	FutureSlots [][]byte `json:"futureSlots,omitempty"`
}

// ColorPresetLayerFreeColor is LayerFreeColor's inherited four-slot object.
type ColorPresetLayerFreeColor struct {
	Version     int32                 `json:"version"`
	BaseColor   *ColorPresetFreeColor `json:"baseColor"`
	ShadowColor *ColorPresetFreeColor `json:"shadowColor"`
	ShadowRate  int32                 `json:"shadowRate"`
	FieldCount  *int32                `json:"fieldCount,omitempty"`
	FutureSlots [][]byte              `json:"futureSlots,omitempty"`
}

// ColorPresetControlSlider contains the only serialized ControlSlider field,
// private value_. Its readonly range is ignored by MessagePack and is rebuilt
// as [0,1] by the C# constructor.
type ColorPresetControlSlider struct {
	Value       float32  `json:"value"`
	FieldCount  *int32   `json:"fieldCount,omitempty"`
	FutureSlots [][]byte `json:"futureSlots,omitempty"`
}

// ColorPresetGradationColor extends LayerFreeColor with three ControlSliders.
type ColorPresetGradationColor struct {
	Version     int32                     `json:"version"`
	BaseColor   *ColorPresetFreeColor     `json:"baseColor"`
	ShadowColor *ColorPresetFreeColor     `json:"shadowColor"`
	ShadowRate  int32                     `json:"shadowRate"`
	Position    *ColorPresetControlSlider `json:"controlPointPosition"`
	RangeBefore *ColorPresetControlSlider `json:"controlPointRangeBefore"`
	RangeAfter  *ColorPresetControlSlider `json:"controlPointRangeAfter"`
	FieldCount  *int32                    `json:"fieldCount,omitempty"`
	FutureSlots [][]byte                  `json:"futureSlots,omitempty"`
}

// NewColorPreset creates the same deterministic defaults as the C#
// constructor, except the caller supplies the GUID instead of permitting
// Guid.NewGuid to hide identity changes inside serialization code.
func NewColorPreset(instanceGUID string) (*ColorPreset, error) {
	if err := validateColorPresetConstructorGUID(instanceGUID, "ColorPreset.instanceGuid"); err != nil {
		return nil, err
	}
	return newColorPresetDefaults(instanceGUID), nil
}

// NewColorPresetSlot is the corresponding explicit constructor for the
// wire-identical ColorPresetSlot type.
func NewColorPresetSlot(instanceGUID string) (*ColorPresetSlot, error) {
	return NewColorPreset(instanceGUID)
}

func newColorPresetDefaults(instanceGUID string) *ColorPreset {
	return &ColorPreset{
		Version:       ColorPresetVersion,
		ColorPackList: make([]*ColorPresetColorPack, 0),
		InstanceGUID:  instanceGUID,
	}
}

func newColorPresetPackDefaults() *ColorPresetColorPack {
	return &ColorPresetColorPack{
		Version:            ColorPresetPackVersion,
		ColorList:          make([]*ColorPresetLayerFreeColor, 0),
		GradationColorList: make([]*ColorPresetGradationColor, 0),
	}
}

func newColorPresetFreeColorDefaults() *ColorPresetFreeColor {
	return &ColorPresetFreeColor{Version: ColorPresetColorVersion}
}

func newColorPresetLayerDefaults() *ColorPresetLayerFreeColor {
	return &ColorPresetLayerFreeColor{
		Version:     ColorPresetColorVersion,
		BaseColor:   newColorPresetFreeColorDefaults(),
		ShadowColor: newColorPresetFreeColorDefaults(),
	}
}

func newColorPresetGradationDefaults() *ColorPresetGradationColor {
	return &ColorPresetGradationColor{
		Version:     ColorPresetColorVersion,
		BaseColor:   newColorPresetFreeColorDefaults(),
		ShadowColor: newColorPresetFreeColorDefaults(),
		Position:    &ColorPresetControlSlider{},
		RangeBefore: &ColorPresetControlSlider{},
		RangeAfter:  &ColorPresetControlSlider{},
	}
}

// DecodeColorPreset decodes a complete PrivateLz4BlockArray user preset without
// invoking constructor defaults, migrations, or serialization callbacks.
func DecodeColorPreset(data []byte) (*ColorPreset, error) {
	return decodeColorPreset(data, "")
}

// DecodeColorPresetWithInstanceGUID supplies the deterministic value that the
// C# object constructor/OnAfterDeserialize callback would otherwise generate
// when Key(6) is absent, nil, or empty. This is an explicit opt-in convenience;
// a non-empty wire value remains authoritative and is treated as opaque text.
func DecodeColorPresetWithInstanceGUID(data []byte, constructorGUID string) (*ColorPreset, error) {
	if err := validateColorPresetConstructorGUID(constructorGUID, "ColorPreset constructor instanceGuid"); err != nil {
		return nil, err
	}
	return decodeColorPreset(data, constructorGUID)
}

// DecodeColorPresetSlot decodes the wire-identical ColorPresetSlot payload.
func DecodeColorPresetSlot(data []byte) (*ColorPresetSlot, error) {
	return DecodeColorPreset(data)
}

// DecodeColorPresetSlotWithInstanceGUID is the deterministic constructor form
// for a ColorPresetSlot payload with a missing instanceGuid field.
func DecodeColorPresetSlotWithInstanceGUID(data []byte, constructorGUID string) (*ColorPresetSlot, error) {
	return DecodeColorPresetWithInstanceGUID(data, constructorGUID)
}

func decodeColorPreset(data []byte, constructorGUID string) (*ColorPreset, error) {
	raw, err := ct.DecompressLz4BlockArray(data)
	if err != nil {
		return nil, fmt.Errorf("decompress ColorPreset PrivateLz4BlockArray: %w", err)
	}

	r := simpleEditDataReader{data: raw}
	if r.tryReadNil() {
		trailing, err := messagePackRootTrailingAfterParsed(raw, r.pos, "ColorPreset")
		if err != nil {
			return nil, err
		}
		if len(trailing) != 0 {
			return &ColorPreset{MessagePackRootMetadata: MessagePackRootMetadata{RootNil: true, TrailingData: trailing}}, nil
		}
		return nil, nil
	}
	fieldCount, err := colorPresetReadObjectHeader(&r, "ColorPreset")
	if err != nil {
		return nil, err
	}
	value := &ColorPreset{InstanceGUID: constructorGUID}
	if fieldCount != 7 {
		storedFieldCount := int32(fieldCount)
		value.FieldCount = &storedFieldCount
	}
	if fieldCount >= 1 {
		value.Version, err = r.readInt32("ColorPreset.version")
		if err != nil {
			return nil, err
		}
	}
	if fieldCount >= 2 {
		value.ID, err = colorPresetReadNullableString(&r, "ColorPreset.id")
		if err != nil {
			return nil, err
		}
	}
	if fieldCount >= 3 {
		value.BaseMenuFile, err = colorPresetReadNullableString(&r, "ColorPreset.baseMenuFile")
		if err != nil {
			return nil, err
		}
	}
	if fieldCount >= 4 {
		value.UserCreated, err = colorPresetReadBool(&r, "ColorPreset.userCreated")
		if err != nil {
			return nil, err
		}
	}
	if fieldCount >= 5 {
		value.IsAdvancedMode, err = colorPresetReadBool(&r, "ColorPreset.isAdvancedMode_")
		if err != nil {
			return nil, err
		}
	}
	if fieldCount >= 6 {
		value.ColorPackList, err = colorPresetReadPackList(&r, "ColorPreset.colorPackList")
		if err != nil {
			return nil, err
		}
	}
	if fieldCount >= 7 {
		guid, readErr := colorPresetReadNullableString(&r, "ColorPreset.instanceGuid")
		if readErr != nil {
			return nil, readErr
		}
		if (guid == nil || *guid == "") && constructorGUID != "" {
			value.InstanceGUID = constructorGUID
		} else if guid != nil {
			value.InstanceGUID = *guid
		}
		value.InstanceGUIDIsNil = guid == nil
	}
	value.FutureSlots, err = colorPresetReadFutureFields(&r, 7, fieldCount, "ColorPreset")
	if err != nil {
		return nil, err
	}
	trailing, err := messagePackRootTrailingAfterParsed(raw, r.pos, "ColorPreset")
	if err != nil {
		return nil, err
	}
	if err := validateDecodedColorPreset(value); err != nil {
		return nil, err
	}
	value.TrailingData = trailing
	return value, nil
}

// EncodeColorPreset emits the indexed-object width represented by FieldCount
// and FutureSlots without invoking any game migration or serialization
// callback. Every explicit version and both the numeric/name MPN arrays are
// preserved as supplied.
func EncodeColorPreset(value *ColorPreset) ([]byte, error) {
	if value == nil {
		return []byte{0xc0}, nil
	}
	if raw, handled, err := encodeNilMessagePackRootIfRequested(
		value.MessagePackRootMetadata,
		value.Version != 0 || value.ID != nil || value.BaseMenuFile != nil || value.UserCreated || value.IsAdvancedMode ||
			value.ColorPackList != nil || value.InstanceGUID != "" || value.InstanceGUIDIsNil || value.FieldCount != nil || len(value.FutureSlots) != 0,
		"ColorPreset",
	); handled {
		if err != nil {
			return nil, err
		}
		return colorPresetCompress(raw)
	}
	fieldCount, err := resolveIndexedFieldCount(value.FieldCount, 7, value.FutureSlots, "ColorPreset")
	if err != nil {
		return nil, err
	}
	if fieldCount < 1 && value.Version != 0 {
		return nil, fmt.Errorf("ColorPreset fieldCount %d would discard version=%d", fieldCount, value.Version)
	}
	if fieldCount < 2 && value.ID != nil {
		return nil, fmt.Errorf("ColorPreset fieldCount %d would discard id", fieldCount)
	}
	if fieldCount < 3 && value.BaseMenuFile != nil {
		return nil, fmt.Errorf("ColorPreset fieldCount %d would discard baseMenuFile", fieldCount)
	}
	if fieldCount < 4 && value.UserCreated {
		return nil, fmt.Errorf("ColorPreset fieldCount %d would discard userCreated", fieldCount)
	}
	if fieldCount < 5 && value.IsAdvancedMode {
		return nil, fmt.Errorf("ColorPreset fieldCount %d would discard isAdvancedMode", fieldCount)
	}
	if fieldCount < 6 && value.ColorPackList != nil {
		return nil, fmt.Errorf("ColorPreset fieldCount %d would discard colorPackList", fieldCount)
	}
	if err := validateColorPresetForEncoding(value); err != nil {
		return nil, err
	}

	raw := simpleEditDataAppendArrayHeader(nil, fieldCount)
	if fieldCount >= 1 {
		raw = simpleEditDataAppendInt32(raw, value.Version)
	}
	if fieldCount >= 2 {
		raw = colorPresetAppendNullableString(raw, value.ID)
	}
	if fieldCount >= 3 {
		raw = colorPresetAppendNullableString(raw, value.BaseMenuFile)
	}
	if fieldCount >= 4 {
		raw = colorPresetAppendBool(raw, value.UserCreated)
	}
	if fieldCount >= 5 {
		raw = colorPresetAppendBool(raw, value.IsAdvancedMode)
	}
	if fieldCount >= 6 {
		if value.ColorPackList == nil {
			raw = append(raw, 0xc0)
		} else {
			raw = simpleEditDataAppendArrayHeader(raw, int64(len(value.ColorPackList)))
			for index, pack := range value.ColorPackList {
				raw, err = colorPresetAppendPack(raw, pack, fmt.Sprintf("ColorPreset.colorPackList[%d]", index))
				if err != nil {
					return nil, err
				}
			}
		}
	}
	if fieldCount >= 7 {
		if value.InstanceGUIDIsNil {
			raw = append(raw, 0xc0)
		} else {
			raw = simpleEditDataAppendString(raw, value.InstanceGUID)
		}
	}
	for _, slot := range value.FutureSlots {
		raw = append(raw, slot...)
	}
	raw = appendMessagePackRootTrailing(raw, value.MessagePackRootMetadata)
	return colorPresetCompress(raw)
}

// EncodeColorPresetSlot emits the wire-identical ColorPresetSlot payload.
func EncodeColorPresetSlot(value *ColorPresetSlot) ([]byte, error) {
	return EncodeColorPreset(value)
}

func validateDecodedColorPreset(value *ColorPreset) error {
	return validateColorPresetForEncoding(value)
}

func validateColorPresetForEncoding(value *ColorPreset) error {
	if value == nil {
		return fmt.Errorf("ColorPreset is nil")
	}

	if uint64(len(value.ColorPackList)) > math.MaxUint32 {
		return fmt.Errorf("ColorPreset.colorPackList length %d exceeds the MessagePack array32 limit", len(value.ColorPackList))
	}
	if err := colorPresetValidateNullableString(value.ID, "ColorPreset.id"); err != nil {
		return err
	}
	if err := colorPresetValidateNullableString(value.BaseMenuFile, "ColorPreset.baseMenuFile"); err != nil {
		return err
	}
	if err := colorPresetValidateString(value.InstanceGUID, "ColorPreset.instanceGuid"); err != nil {
		return err
	}
	for index, pack := range value.ColorPackList {
		if err := validateColorPresetPack(pack, fmt.Sprintf("ColorPreset.colorPackList[%d]", index), false); err != nil {
			return err
		}
	}
	return nil
}

// Constructors in the game initialize instanceGuid with Guid.NewGuid().ToString().
// Callers simulating that otherwise-random constructor path must therefore
// provide a D-format GUID. Existing non-empty wire identifiers remain opaque.
func validateColorPresetConstructorGUID(value, path string) error {
	if len(value) != 36 {
		return fmt.Errorf("%s must be a non-empty D-format GUID (8-4-4-4-12 hex digits)", path)
	}
	for index := 0; index < len(value); index++ {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if value[index] != '-' {
				return fmt.Errorf("%s must be a D-format GUID (8-4-4-4-12 hex digits)", path)
			}
			continue
		}
		b := value[index]
		if !((b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')) {
			return fmt.Errorf("%s must be a D-format GUID (8-4-4-4-12 hex digits)", path)
		}
	}
	return nil
}

func colorPresetReadPackList(r *simpleEditDataReader, path string) ([]*ColorPresetColorPack, error) {
	if r.tryReadNil() {
		return nil, nil
	}
	count, err := colorPresetReadCollectionHeader(r, path)
	if err != nil {
		return nil, err
	}
	result := makeKCESCountedSliceForAppend[*ColorPresetColorPack](uint64(count))
	for index := int64(0); index < count; index++ {
		pack, err := colorPresetReadPack(r, fmt.Sprintf("%s[%d]", path, index))
		if err != nil {
			return nil, err
		}
		result = append(result, pack)
	}
	return result, nil
}

func colorPresetReadPack(r *simpleEditDataReader, path string) (*ColorPresetColorPack, error) {
	if r.tryReadNil() {
		return nil, nil
	}
	fieldCount, err := colorPresetReadObjectHeader(r, path)
	if err != nil {
		return nil, err
	}
	value := &ColorPresetColorPack{}
	if fieldCount != 10 {
		storedFieldCount := int32(fieldCount)
		value.FieldCount = &storedFieldCount
	}
	if fieldCount >= 1 {
		value.Version, err = r.readInt32(path + ".version")
		if err != nil {
			return nil, err
		}
	}
	if fieldCount >= 2 {
		value.MPNs, err = colorPresetReadInt32Array(r, path+".mpns")
		if err != nil {
			return nil, err
		}
	}
	if fieldCount >= 3 {
		value.LayerName, err = colorPresetReadNullableString(r, path+".layerName")
		if err != nil {
			return nil, err
		}
	}
	if fieldCount >= 4 {
		value.ViewName, err = colorPresetReadNullableString(r, path+".viewName")
		if err != nil {
			return nil, err
		}
	}
	if fieldCount >= 5 {
		typeValue, readErr := r.readInt32(path + ".type")
		if readErr != nil {
			return nil, readErr
		}
		value.Type = ColorPresetPackType(typeValue)
	}
	if fieldCount >= 6 {
		value.ColorList, err = colorPresetReadLayerList(r, path+".colorList")
		if err != nil {
			return nil, err
		}
	}
	if fieldCount >= 7 {
		value.GradationColorList, err = colorPresetReadGradationList(r, path+".gradationColorList")
		if err != nil {
			return nil, err
		}
	}
	if fieldCount >= 8 {
		value.Alpha, err = colorPresetReadSingle(r, path+".alpha")
		if err != nil {
			return nil, err
		}
	}
	if fieldCount >= 9 {
		value.AllowedMPNOverride, err = colorPresetReadBool(r, path+".allowedMpnOverRide")
		if err != nil {
			return nil, err
		}
	}
	if fieldCount >= 10 {
		value.MPNNames, value.MPNNameNulls, err = colorPresetReadStringArray(r, path+".mpnNames")
		if err != nil {
			return nil, err
		}
	}
	value.FutureSlots, err = colorPresetReadFutureFields(r, 10, fieldCount, path)
	if err != nil {
		return nil, err
	}
	if err := validateColorPresetPack(value, path, true); err != nil {
		return nil, err
	}
	return value, nil
}

func validateColorPresetPack(value *ColorPresetColorPack, path string, decoded bool) error {
	if value == nil {
		return nil
	}

	if uint64(len(value.MPNs)) > math.MaxUint32 || uint64(len(value.MPNNames)) > math.MaxUint32 {
		return fmt.Errorf("%s MPN collection exceeds the MessagePack array32 limit", path)
	}
	if value.MPNNameNulls != nil && len(value.MPNNameNulls) != len(value.MPNNames) {
		return fmt.Errorf("%s.mpnNameNulls length %d does not match mpnNames length %d", path, len(value.MPNNameNulls), len(value.MPNNames))
	}
	for index, name := range value.MPNNames {
		if value.MPNNameNulls != nil && value.MPNNameNulls[index] {
			if name != "" {
				return fmt.Errorf("%s.mpnNameNulls[%d] would discard non-empty mpnNames value", path, index)
			}
			continue
		}
		if err := colorPresetValidateString(name, fmt.Sprintf("%s.mpnNames[%d]", path, index)); err != nil {
			return err
		}
	}
	if err := colorPresetValidateNullableString(value.LayerName, path+".layerName"); err != nil {
		return err
	}
	if err := colorPresetValidateNullableString(value.ViewName, path+".viewName"); err != nil {
		return err
	}

	if uint64(len(value.ColorList)) > math.MaxUint32 || uint64(len(value.GradationColorList)) > math.MaxUint32 {
		return fmt.Errorf("%s color collection exceeds the MessagePack array32 limit", path)
	}
	for index, color := range value.ColorList {
		if err := validateColorPresetLayer(color, fmt.Sprintf("%s.colorList[%d]", path, index), decoded); err != nil {
			return err
		}
	}
	for index, color := range value.GradationColorList {
		if err := validateColorPresetGradation(color, fmt.Sprintf("%s.gradationColorList[%d]", path, index), decoded); err != nil {
			return err
		}
	}
	return nil
}

func colorPresetAppendPack(dst []byte, value *ColorPresetColorPack, path string) ([]byte, error) {
	if value == nil {
		return append(dst, 0xc0), nil
	}
	fieldCount, err := resolveIndexedFieldCount(value.FieldCount, 10, value.FutureSlots, path)
	if err != nil {
		return nil, err
	}
	if fieldCount < 1 && value.Version != 0 {
		return nil, fmt.Errorf("%s fieldCount %d would discard version=%d", path, fieldCount, value.Version)
	}
	if fieldCount < 2 && value.MPNs != nil {
		return nil, fmt.Errorf("%s fieldCount %d would discard mpns", path, fieldCount)
	}
	if fieldCount < 3 && value.LayerName != nil {
		return nil, fmt.Errorf("%s fieldCount %d would discard layerName", path, fieldCount)
	}
	if fieldCount < 4 && value.ViewName != nil {
		return nil, fmt.Errorf("%s fieldCount %d would discard viewName", path, fieldCount)
	}
	if fieldCount < 5 && value.Type != 0 {
		return nil, fmt.Errorf("%s fieldCount %d would discard type=%d", path, fieldCount, value.Type)
	}
	if fieldCount < 6 && value.ColorList != nil {
		return nil, fmt.Errorf("%s fieldCount %d would discard colorList", path, fieldCount)
	}
	if fieldCount < 7 && value.GradationColorList != nil {
		return nil, fmt.Errorf("%s fieldCount %d would discard gradationColorList", path, fieldCount)
	}
	if fieldCount < 8 && math.Float32bits(value.Alpha) != 0 {
		return nil, fmt.Errorf("%s fieldCount %d would discard alpha", path, fieldCount)
	}
	if fieldCount < 9 && value.AllowedMPNOverride {
		return nil, fmt.Errorf("%s fieldCount %d would discard allowedMpnOverRide", path, fieldCount)
	}
	if fieldCount < 10 && (value.MPNNames != nil || value.MPNNameNulls != nil) {
		return nil, fmt.Errorf("%s fieldCount %d would discard mpnNames", path, fieldCount)
	}
	if err := validateColorPresetPack(value, path, false); err != nil {
		return nil, err
	}
	dst = simpleEditDataAppendArrayHeader(dst, fieldCount)
	if fieldCount >= 1 {
		dst = simpleEditDataAppendInt32(dst, value.Version)
	}
	if fieldCount >= 2 {
		if value.MPNs == nil {
			dst = append(dst, 0xc0)
		} else {
			dst = simpleEditDataAppendArrayHeader(dst, int64(len(value.MPNs)))
			for _, mpn := range value.MPNs {
				dst = simpleEditDataAppendInt32(dst, mpn)
			}
		}
	}
	if fieldCount >= 3 {
		dst = colorPresetAppendNullableString(dst, value.LayerName)
	}
	if fieldCount >= 4 {
		dst = colorPresetAppendNullableString(dst, value.ViewName)
	}
	if fieldCount >= 5 {
		dst = simpleEditDataAppendInt32(dst, int32(value.Type))
	}
	if fieldCount >= 6 {
		if value.ColorList == nil {
			dst = append(dst, 0xc0)
		} else {
			dst = simpleEditDataAppendArrayHeader(dst, int64(len(value.ColorList)))
			for index, color := range value.ColorList {
				dst, err = colorPresetAppendLayer(dst, color, fmt.Sprintf("%s.colorList[%d]", path, index))
				if err != nil {
					return nil, err
				}
			}
		}
	}
	if fieldCount >= 7 {
		if value.GradationColorList == nil {
			dst = append(dst, 0xc0)
		} else {
			dst = simpleEditDataAppendArrayHeader(dst, int64(len(value.GradationColorList)))
			for index, color := range value.GradationColorList {
				dst, err = colorPresetAppendGradation(dst, color, fmt.Sprintf("%s.gradationColorList[%d]", path, index))
				if err != nil {
					return nil, err
				}
			}
		}
	}
	if fieldCount >= 8 {
		dst = colorPresetAppendFloat32(dst, value.Alpha)
	}
	if fieldCount >= 9 {
		dst = colorPresetAppendBool(dst, value.AllowedMPNOverride)
	}
	if fieldCount >= 10 {
		if value.MPNNames == nil {
			dst = append(dst, 0xc0)
		} else {
			dst = simpleEditDataAppendArrayHeader(dst, int64(len(value.MPNNames)))
			for index, name := range value.MPNNames {
				if value.MPNNameNulls != nil && value.MPNNameNulls[index] {
					dst = append(dst, 0xc0)
				} else {
					dst = simpleEditDataAppendString(dst, name)
				}
			}
		}
	}
	for _, slot := range value.FutureSlots {
		dst = append(dst, slot...)
	}
	return dst, nil
}

func colorPresetReadLayerList(r *simpleEditDataReader, path string) ([]*ColorPresetLayerFreeColor, error) {
	if r.tryReadNil() {
		return nil, nil
	}
	count, err := colorPresetReadCollectionHeader(r, path)
	if err != nil {
		return nil, err
	}
	result := makeKCESCountedSliceForAppend[*ColorPresetLayerFreeColor](uint64(count))
	for index := int64(0); index < count; index++ {
		value, err := colorPresetReadLayer(r, fmt.Sprintf("%s[%d]", path, index))
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}

func colorPresetReadLayer(r *simpleEditDataReader, path string) (*ColorPresetLayerFreeColor, error) {
	if r.tryReadNil() {
		return nil, nil
	}
	fieldCount, err := colorPresetReadObjectHeader(r, path)
	if err != nil {
		return nil, err
	}
	value := &ColorPresetLayerFreeColor{}
	if fieldCount != 4 {
		storedFieldCount := int32(fieldCount)
		value.FieldCount = &storedFieldCount
	}
	if fieldCount >= 1 {
		value.Version, err = r.readInt32(path + ".version")
		if err != nil {
			return nil, err
		}
	}
	if fieldCount >= 2 {
		value.BaseColor, err = colorPresetReadFreeColor(r, path+".baseColor")
		if err != nil {
			return nil, err
		}
	}
	if fieldCount >= 3 {
		value.ShadowColor, err = colorPresetReadFreeColor(r, path+".shadowColor")
		if err != nil {
			return nil, err
		}
	}
	if fieldCount >= 4 {
		value.ShadowRate, err = r.readInt32(path + ".shadowRate_")
		if err != nil {
			return nil, err
		}
	}
	value.FutureSlots, err = colorPresetReadFutureFields(r, 4, fieldCount, path)
	if err != nil {
		return nil, err
	}
	return value, nil
}

func validateColorPresetLayer(value *ColorPresetLayerFreeColor, path string, decoded bool) error {
	if value == nil {
		return nil
	}

	if err := validateColorPresetFree(value.BaseColor, path+".baseColor", decoded); err != nil {
		return err
	}
	if err := validateColorPresetFree(value.ShadowColor, path+".shadowColor", decoded); err != nil {
		return err
	}
	return nil
}

func colorPresetAppendLayer(dst []byte, value *ColorPresetLayerFreeColor, path string) ([]byte, error) {
	if value == nil {
		return append(dst, 0xc0), nil
	}
	fieldCount, err := resolveIndexedFieldCount(value.FieldCount, 4, value.FutureSlots, path)
	if err != nil {
		return nil, err
	}
	if fieldCount < 1 && value.Version != 0 {
		return nil, fmt.Errorf("%s fieldCount %d would discard version=%d", path, fieldCount, value.Version)
	}
	if fieldCount < 2 && value.BaseColor != nil {
		return nil, fmt.Errorf("%s fieldCount %d would discard baseColor", path, fieldCount)
	}
	if fieldCount < 3 && value.ShadowColor != nil {
		return nil, fmt.Errorf("%s fieldCount %d would discard shadowColor", path, fieldCount)
	}
	if fieldCount < 4 && value.ShadowRate != 0 {
		return nil, fmt.Errorf("%s fieldCount %d would discard shadowRate=%d", path, fieldCount, value.ShadowRate)
	}
	if err := validateColorPresetLayer(value, path, false); err != nil {
		return nil, err
	}
	dst = simpleEditDataAppendArrayHeader(dst, fieldCount)
	if fieldCount >= 1 {
		dst = simpleEditDataAppendInt32(dst, value.Version)
	}
	if fieldCount >= 2 {
		dst, err = colorPresetAppendFree(dst, value.BaseColor, path+".baseColor")
		if err != nil {
			return nil, err
		}
	}
	if fieldCount >= 3 {
		dst, err = colorPresetAppendFree(dst, value.ShadowColor, path+".shadowColor")
		if err != nil {
			return nil, err
		}
	}
	if fieldCount >= 4 {
		dst = simpleEditDataAppendInt32(dst, value.ShadowRate)
	}
	for _, slot := range value.FutureSlots {
		dst = append(dst, slot...)
	}
	return dst, nil
}

func colorPresetReadFreeColor(r *simpleEditDataReader, path string) (*ColorPresetFreeColor, error) {
	if r.tryReadNil() {
		return nil, nil
	}
	fieldCount, err := colorPresetReadObjectHeader(r, path)
	if err != nil {
		return nil, err
	}
	value := &ColorPresetFreeColor{}
	if fieldCount != 5 {
		storedFieldCount := int32(fieldCount)
		value.FieldCount = &storedFieldCount
	}
	fields := []*int32{&value.Version, &value.Hue, &value.Saturation, &value.Brightness, &value.Contrast}
	names := []string{"version", "hue_", "saturation_", "brightness_", "contrast_"}
	for index := int64(0); index < fieldCount && index < int64(len(fields)); index++ {
		*fields[index], err = r.readInt32(path + "." + names[index])
		if err != nil {
			return nil, err
		}
	}
	value.FutureSlots, err = colorPresetReadFutureFields(r, 5, fieldCount, path)
	if err != nil {
		return nil, err
	}
	return value, nil
}

func validateColorPresetFree(value *ColorPresetFreeColor, path string, decoded bool) error {
	if value == nil {
		return nil
	}

	return nil
}

func colorPresetAppendFree(dst []byte, value *ColorPresetFreeColor, path string) ([]byte, error) {
	if value == nil {
		return append(dst, 0xc0), nil
	}
	fieldCount, err := resolveIndexedFieldCount(value.FieldCount, 5, value.FutureSlots, path)
	if err != nil {
		return nil, err
	}
	fields := []struct {
		name  string
		value int32
	}{{"version", value.Version}, {"hue", value.Hue}, {"saturation", value.Saturation}, {"brightness", value.Brightness}, {"contrast", value.Contrast}}
	for index, field := range fields {
		if fieldCount <= int64(index) && field.value != 0 {
			return nil, fmt.Errorf("%s fieldCount %d would discard %s=%d", path, fieldCount, field.name, field.value)
		}
	}
	if err := validateColorPresetFree(value, path, false); err != nil {
		return nil, err
	}
	dst = simpleEditDataAppendArrayHeader(dst, fieldCount)
	for index := int64(0); index < fieldCount && index < int64(len(fields)); index++ {
		dst = simpleEditDataAppendInt32(dst, fields[index].value)
	}
	for _, slot := range value.FutureSlots {
		dst = append(dst, slot...)
	}
	return dst, nil
}

func colorPresetReadGradationList(r *simpleEditDataReader, path string) ([]*ColorPresetGradationColor, error) {
	if r.tryReadNil() {
		return nil, nil
	}
	count, err := colorPresetReadCollectionHeader(r, path)
	if err != nil {
		return nil, err
	}
	result := makeKCESCountedSliceForAppend[*ColorPresetGradationColor](uint64(count))
	for index := int64(0); index < count; index++ {
		value, err := colorPresetReadGradation(r, fmt.Sprintf("%s[%d]", path, index))
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}

func colorPresetReadGradation(r *simpleEditDataReader, path string) (*ColorPresetGradationColor, error) {
	if r.tryReadNil() {
		return nil, nil
	}
	fieldCount, err := colorPresetReadObjectHeader(r, path)
	if err != nil {
		return nil, err
	}
	value := &ColorPresetGradationColor{}
	if fieldCount != 7 {
		storedFieldCount := int32(fieldCount)
		value.FieldCount = &storedFieldCount
	}
	if fieldCount >= 1 {
		value.Version, err = r.readInt32(path + ".version")
	}
	if err == nil && fieldCount >= 2 {
		value.BaseColor, err = colorPresetReadFreeColor(r, path+".baseColor")
	}
	if err == nil && fieldCount >= 3 {
		value.ShadowColor, err = colorPresetReadFreeColor(r, path+".shadowColor")
	}
	if err == nil && fieldCount >= 4 {
		value.ShadowRate, err = r.readInt32(path + ".shadowRate_")
	}
	if err == nil && fieldCount >= 5 {
		value.Position, err = colorPresetReadControlSlider(r, path+".controlPointPosition")
	}
	if err == nil && fieldCount >= 6 {
		value.RangeBefore, err = colorPresetReadControlSlider(r, path+".controlPointRangeBefore")
	}
	if err == nil && fieldCount >= 7 {
		value.RangeAfter, err = colorPresetReadControlSlider(r, path+".controlPointRangeAfter")
	}
	if err != nil {
		return nil, err
	}
	value.FutureSlots, err = colorPresetReadFutureFields(r, 7, fieldCount, path)
	if err != nil {
		return nil, err
	}
	return value, nil
}

func validateColorPresetGradation(value *ColorPresetGradationColor, path string, decoded bool) error {
	if value == nil {
		return nil
	}

	if err := validateColorPresetFree(value.BaseColor, path+".baseColor", decoded); err != nil {
		return err
	}
	if err := validateColorPresetFree(value.ShadowColor, path+".shadowColor", decoded); err != nil {
		return err
	}

	return nil
}

func colorPresetAppendGradation(dst []byte, value *ColorPresetGradationColor, path string) ([]byte, error) {
	if value == nil {
		return append(dst, 0xc0), nil
	}
	fieldCount, err := resolveIndexedFieldCount(value.FieldCount, 7, value.FutureSlots, path)
	if err != nil {
		return nil, err
	}
	if fieldCount < 1 && value.Version != 0 {
		return nil, fmt.Errorf("%s fieldCount %d would discard version=%d", path, fieldCount, value.Version)
	}
	if fieldCount < 2 && value.BaseColor != nil {
		return nil, fmt.Errorf("%s fieldCount %d would discard baseColor", path, fieldCount)
	}
	if fieldCount < 3 && value.ShadowColor != nil {
		return nil, fmt.Errorf("%s fieldCount %d would discard shadowColor", path, fieldCount)
	}
	if fieldCount < 4 && value.ShadowRate != 0 {
		return nil, fmt.Errorf("%s fieldCount %d would discard shadowRate=%d", path, fieldCount, value.ShadowRate)
	}
	if fieldCount < 5 && value.Position != nil {
		return nil, fmt.Errorf("%s fieldCount %d would discard controlPointPosition", path, fieldCount)
	}
	if fieldCount < 6 && value.RangeBefore != nil {
		return nil, fmt.Errorf("%s fieldCount %d would discard controlPointRangeBefore", path, fieldCount)
	}
	if fieldCount < 7 && value.RangeAfter != nil {
		return nil, fmt.Errorf("%s fieldCount %d would discard controlPointRangeAfter", path, fieldCount)
	}
	if err := validateColorPresetGradation(value, path, false); err != nil {
		return nil, err
	}
	dst = simpleEditDataAppendArrayHeader(dst, fieldCount)
	if fieldCount >= 1 {
		dst = simpleEditDataAppendInt32(dst, value.Version)
	}
	if fieldCount >= 2 {
		dst, err = colorPresetAppendFree(dst, value.BaseColor, path+".baseColor")
		if err != nil {
			return nil, err
		}
	}
	if fieldCount >= 3 {
		dst, err = colorPresetAppendFree(dst, value.ShadowColor, path+".shadowColor")
		if err != nil {
			return nil, err
		}
	}
	if fieldCount >= 4 {
		dst = simpleEditDataAppendInt32(dst, value.ShadowRate)
	}
	sliders := []struct {
		value *ColorPresetControlSlider
		name  string
	}{{value.Position, "controlPointPosition"}, {value.RangeBefore, "controlPointRangeBefore"}, {value.RangeAfter, "controlPointRangeAfter"}}
	for index, slider := range sliders {
		if fieldCount < 5+int64(index) {
			break
		}
		dst, err = colorPresetAppendControlSlider(dst, slider.value, path+"."+slider.name)
		if err != nil {
			return nil, err
		}
	}
	for _, slot := range value.FutureSlots {
		dst = append(dst, slot...)
	}
	return dst, nil
}

func colorPresetAppendControlSlider(dst []byte, value *ColorPresetControlSlider, path string) ([]byte, error) {
	if value == nil {
		return append(dst, 0xc0), nil
	}
	fieldCount, err := resolveIndexedFieldCount(value.FieldCount, 1, value.FutureSlots, path)
	if err != nil {
		return nil, err
	}
	if fieldCount < 1 && math.Float32bits(value.Value) != 0 {
		return nil, fmt.Errorf("%s fieldCount %d would discard value", path, fieldCount)
	}
	dst = simpleEditDataAppendArrayHeader(dst, fieldCount)
	if fieldCount >= 1 {
		dst = colorPresetAppendFloat32(dst, value.Value)
	}
	for _, slot := range value.FutureSlots {
		dst = append(dst, slot...)
	}
	return dst, nil
}

func colorPresetReadControlSlider(r *simpleEditDataReader, path string) (*ColorPresetControlSlider, error) {
	if r.tryReadNil() {
		return nil, nil
	}
	fieldCount, err := colorPresetReadObjectHeader(r, path)
	if err != nil {
		return nil, err
	}
	value := &ColorPresetControlSlider{}
	if fieldCount != 1 {
		storedFieldCount := int32(fieldCount)
		value.FieldCount = &storedFieldCount
	}
	if fieldCount >= 1 {
		value.Value, err = colorPresetReadSingle(r, path+".value_")
		if err != nil {
			return nil, err
		}
	}
	value.FutureSlots, err = colorPresetReadFutureFields(r, 1, fieldCount, path)
	if err != nil {
		return nil, err
	}
	return value, nil
}

func colorPresetReadInt32Array(r *simpleEditDataReader, path string) ([]int32, error) {
	if r.tryReadNil() {
		return nil, nil
	}
	count, err := colorPresetReadCollectionHeader(r, path)
	if err != nil {
		return nil, err
	}
	result := makeKCESCountedSliceForAppend[int32](uint64(count))
	for index := int64(0); index < count; index++ {
		value, readErr := r.readInt32(fmt.Sprintf("%s[%d]", path, index))
		err = readErr
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}

func colorPresetReadStringArray(r *simpleEditDataReader, path string) ([]string, []bool, error) {
	if r.tryReadNil() {
		return nil, nil, nil
	}
	count, err := colorPresetReadCollectionHeader(r, path)
	if err != nil {
		return nil, nil, err
	}
	result := makeKCESCountedSliceForAppend[string](uint64(count))
	var nulls []bool
	for index := int64(0); index < count; index++ {
		if r.tryReadNil() {
			if nulls == nil {
				nulls = makeKCESCountedSliceForAppend[bool](uint64(count))
				for range result {
					nulls = append(nulls, false)
				}
			}
			result = append(result, "")
			nulls = append(nulls, true)
			continue
		}
		value, readErr := r.readString(fmt.Sprintf("%s[%d]", path, index))
		err = readErr
		if err != nil {
			return nil, nil, err
		}
		result = append(result, value)
		if nulls != nil {
			nulls = append(nulls, false)
		}
	}
	return result, nulls, nil
}

func colorPresetReadNullableString(r *simpleEditDataReader, path string) (*string, error) {
	if r.tryReadNil() {
		return nil, nil
	}
	value, err := r.readString(path)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func colorPresetReadBool(r *simpleEditDataReader, path string) (bool, error) {
	marker, err := r.readByte(path)
	if err != nil {
		return false, err
	}
	switch marker {
	case 0xc2:
		return false, nil
	case 0xc3:
		return true, nil
	default:
		return false, fmt.Errorf("%s must be a MessagePack bool, got marker 0x%02x", path, marker)
	}
}

// colorPresetReadSingle mirrors MessagePackReader.ReadSingle: all integer
// codes plus float32/float64 are accepted and converted to System.Single.
func colorPresetReadSingle(r *simpleEditDataReader, path string) (float32, error) {
	marker, err := r.readByte(path)
	if err != nil {
		return 0, err
	}
	if marker <= 0x7f {
		return float32(marker), nil
	}
	if marker >= 0xe0 {
		return float32(int8(marker)), nil
	}
	switch marker {
	case 0xca:
		data, err := r.readBytes(4, path+" float32")
		if err != nil {
			return 0, err
		}
		return math.Float32frombits(binary.BigEndian.Uint32(data)), nil
	case 0xcb:
		data, err := r.readBytes(8, path+" float64")
		if err != nil {
			return 0, err
		}
		return float32(math.Float64frombits(binary.BigEndian.Uint64(data))), nil
	case 0xcc:
		value, err := r.readByte(path + " uint8")
		return float32(value), err
	case 0xcd:
		data, err := r.readBytes(2, path+" uint16")
		if err != nil {
			return 0, err
		}
		return float32(binary.BigEndian.Uint16(data)), nil
	case 0xce:
		data, err := r.readBytes(4, path+" uint32")
		if err != nil {
			return 0, err
		}
		return float32(binary.BigEndian.Uint32(data)), nil
	case 0xcf:
		data, err := r.readBytes(8, path+" uint64")
		if err != nil {
			return 0, err
		}
		return float32(binary.BigEndian.Uint64(data)), nil
	case 0xd0:
		value, err := r.readByte(path + " int8")
		return float32(int8(value)), err
	case 0xd1:
		data, err := r.readBytes(2, path+" int16")
		if err != nil {
			return 0, err
		}
		return float32(int16(binary.BigEndian.Uint16(data))), nil
	case 0xd2:
		data, err := r.readBytes(4, path+" int32")
		if err != nil {
			return 0, err
		}
		return float32(int32(binary.BigEndian.Uint32(data))), nil
	case 0xd3:
		data, err := r.readBytes(8, path+" int64")
		if err != nil {
			return 0, err
		}
		return float32(int64(binary.BigEndian.Uint64(data))), nil
	default:
		return 0, fmt.Errorf("%s must be a MessagePack number accepted by ReadSingle, got marker 0x%02x", path, marker)
	}
}

func colorPresetReadObjectHeader(r *simpleEditDataReader, path string) (int64, error) {
	count, err := r.readArrayLength(path)
	if err != nil {
		return 0, err
	}
	if err := r.requirePossibleValues(count, path+" fields"); err != nil {
		return 0, err
	}
	return count, nil
}

func colorPresetReadCollectionHeader(r *simpleEditDataReader, path string) (int64, error) {
	count, err := r.readArrayLength(path)
	if err != nil {
		return 0, err
	}
	if err := r.requirePossibleValues(count, path+" entries"); err != nil {
		return 0, err
	}
	return count, nil
}

func colorPresetReadFutureFields(r *simpleEditDataReader, known, count int64, path string) ([][]byte, error) {
	if count <= known {
		return nil, nil
	}
	result := makeKCESCountedSliceForAppend[[]byte](uint64(count - known))
	for field := known; field < count; field++ {
		start := r.pos
		if err := r.skipValue(0); err != nil {
			return nil, fmt.Errorf("skip %s future Key(%d): %w", path, field, err)
		}
		result = append(result, append([]byte(nil), r.data[start:r.pos]...))
	}
	return result, nil
}

func colorPresetValidateNullableString(value *string, path string) error {
	if value == nil {
		return nil
	}
	return colorPresetValidateString(*value, path)
}

func colorPresetValidateString(value, path string) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("%s is not valid UTF-8", path)
	}
	if uint64(len(value)) > math.MaxUint32 {
		return fmt.Errorf("%s is too large for MessagePack str32", path)
	}
	return nil
}

func colorPresetAppendNullableString(dst []byte, value *string) []byte {
	if value == nil {
		return append(dst, 0xc0)
	}
	return simpleEditDataAppendString(dst, *value)
}

func colorPresetAppendBool(dst []byte, value bool) []byte {
	if value {
		return append(dst, 0xc3)
	}
	return append(dst, 0xc2)
}

func colorPresetAppendFloat32(dst []byte, value float32) []byte {
	bits := math.Float32bits(value)
	return append(dst, 0xca, byte(bits>>24), byte(bits>>16), byte(bits>>8), byte(bits))
}

func colorPresetCompress(raw []byte) ([]byte, error) {
	if len(raw) < colorPresetCompressionMinLength {
		return raw, nil
	}
	wire, err := ct.CompressLz4BlockArray(raw)
	if err != nil {
		return nil, fmt.Errorf("compress ColorPreset PrivateLz4BlockArray: %w", err)
	}
	return wire, nil
}
