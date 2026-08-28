package KCES

import (
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"unicode/utf8"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/msgpack"
)

// system.dat 内 color_preset 虚拟文件与 KCES2 独立 .presetcolor 文件共用的 MessagePack 和 LZ4 布局
// MessagePack and LZ4 layout shared by color_preset virtual files in system.dat and standalone KCES2 .presetcolor files

const (
	// ColorPresetVersion 是 KCES 1.34.4 中 CustomColorPresetBase<T>.FixVersion 的值
	// ColorPresetVersion is the value of CustomColorPresetBase<T>.FixVersion in KCES 1.34.4
	ColorPresetVersion = 1004
	// ColorPresetNoAssetMigrationMinVersion 是 OnDeserializeVersionCheck 不再检查已安装头发预设菜单的首个根版本
	// ColorPresetNoAssetMigrationMinVersion is the first root version for which OnDeserializeVersionCheck no longer inspects installed hair-preset menus
	ColorPresetNoAssetMigrationMinVersion = 1003
	// ColorPresetPackVersion 是 CustomColorPresetColorPack.FixVersion 的值
	// ColorPresetPackVersion is the value of CustomColorPresetColorPack.FixVersion
	ColorPresetPackVersion = 1001
	// ColorPresetColorVersion 是 KCES 1.34.4 中 FreeColor、LayerFreeColor 与 GradationColor 共用的版本
	// ColorPresetColorVersion is the version shared by FreeColor, LayerFreeColor, and GradationColor in KCES 1.34.4
	ColorPresetColorVersion = 1000

	colorPresetCompressionMinLength = 64
)

// ColorPresetPackType 表示 CustomColorPresetColorPack.Type 的 Int32 线格式值
// ColorPresetPackType represents the Int32 wire value of CustomColorPresetColorPack.Type
type ColorPresetPackType int32

const (
	ColorPresetPackColorAndAlpha ColorPresetPackType = iota
	ColorPresetPackOnlyColor
	ColorPresetPackOnlyAlpha
)

// ColorPreset 表示 MaidEdit.ColorPreset 与 MaidEdit.ColorPresetSlot 共用的序列化基类
// 两个派生类没有增加 MessagePack Key，因此线格式字节完全相同
// ID、BaseMenuFile 与 InstanceGUID 使用指针，因为 C# 字符串格式化器允许 nil
// 普通解码不会重现游戏 Guid.NewGuid 回调，非空线格式值保持不透明，因为游戏不会对它调用 Guid.Parse
// LegacyInstanceGUIDOmitted 仅保留 instanceGuid 加入前的数组宽度，不对应游戏类成员
// ColorPreset represents the serialized base shared by MaidEdit.ColorPreset and MaidEdit.ColorPresetSlot
// Neither derived class adds MessagePack keys, so their wire bytes are identical
// ID, BaseMenuFile, and InstanceGUID use pointers because the C# string formatter permits nil
// Ordinary decoding does not reproduce the game Guid.NewGuid callback, and a non-empty wire value remains opaque because the game never passes it through Guid.Parse
// LegacyInstanceGUIDOmitted only preserves the array width from before instanceGuid was added and does not correspond to a game-class member
type ColorPreset struct {
	Version                   int32                   `json:"version"`                             // Key 0 的预设版本，KCES FixVersion 为 1004，KCES2 为 1005 / Preset version at Key 0, with FixVersion 1004 in KCES and 1005 in KCES2
	ID                        *string                 `json:"id"`                                  // Key 1 的预设标识，用户预设保存时也作为虚拟文件名 / Preset identifier at Key 1, also used as the virtual filename for user presets
	BaseMenuFile              *string                 `json:"baseMenuFile"`                        // Key 2 的基础颜色预设菜单文件名 / Base color-preset menu filename at Key 2
	UserCreated               bool                    `json:"userCreated"`                         // Key 3 的用户创建标志，游戏据此决定是否保存预设二进制 / User-created flag at Key 3, used by the game to decide whether to save preset bytes
	IsAdvancedMode            bool                    `json:"isAdvancedMode"`                      // Key 4 的高级模式状态 / Advanced-mode state at Key 4
	ColorPackList             []*ColorPresetColorPack `json:"colorPackList"`                       // Key 5 的颜色层包列表 / Color-layer pack list at Key 5
	InstanceGUID              *string                 `json:"instanceGuid"`                        // Key 6 的可空实例标识，非空值由游戏作为不透明字符串使用 / Nullable instance identifier at Key 6, with non-empty values treated as opaque strings by the game
	SaveLocationHash          uint64                  `json:"saveLocationHash"`                    // Key 7 的保存目录哈希，KCES2 新增 / Save-directory hash at Key 7, added by KCES2
	CreationTicks             int64                   `json:"creationTicks"`                       // Key 8 的创建时间 DateTime ticks，KCES2 新增 / Creation time as DateTime ticks at Key 8, added by KCES2
	LastUpdateTicks           int64                   `json:"lastUpdateTicks"`                     // Key 9 的最后更新时间 DateTime ticks，KCES2 新增 / Last update time as DateTime ticks at Key 9, added by KCES2
	MetaTexts                 map[string]*string      `json:"metaTexts"`                           // Key 10 的可空元数据文本字典，KCES2 新增 / Nullable metadata text map at Key 10, added by KCES2
	IndexedArrayWidth         int32                   `json:"indexedArrayWidth,omitempty"`         // 解码时记录的 6、7 或 11 槽线格式宽度，并非游戏成员 / Decoded 6-slot, 7-slot, or 11-slot wire width, not a game member
	LegacyInstanceGUIDOmitted bool                    `json:"legacyInstanceGuidOmitted,omitempty"` // 旧版六槽对象未保存 Key 6 的线格式标记，并非游戏成员 / Wire marker for a legacy six-slot object that did not store Key 6, not a game member
}

// ColorPresetSlot 是 ColorPreset 的线格式别名，游戏派生类型没有新增 Key
// ColorPresetSlot is a wire alias of ColorPreset because the game-derived type adds no keys
type ColorPresetSlot = ColorPreset

// ColorPresetColorPack 保留 CustomColorPresetColorPack 的全部序列化字段，包括私有 mpnNames 与 allowedMpnOverRide
// 游戏 CopyTo 方法意外漏掉 allowedMpnOverRide，但本编解码器面向线格式本身，因此不会丢弃它
// LegacyMPNNamesOmitted 仅保留 mpnNames 加入前的数组宽度，不对应游戏类成员
// ColorPresetColorPack preserves every serialized CustomColorPresetColorPack field including private mpnNames and allowedMpnOverRide
// The game CopyTo method accidentally omits allowedMpnOverRide, but this codec models the wire itself and therefore does not discard it
// LegacyMPNNamesOmitted only preserves the array width from before mpnNames was added and does not correspond to a game-class member
type ColorPresetColorPack struct {
	Version               int32                        `json:"version"`                         // Key 0 的色包版本，当前 FixVersion 为 1001 / Color-pack version at Key 0, with a current FixVersion of 1001
	MPNs                  []int32                      `json:"mpns"`                            // Key 1 的 MPN 数值数组，当前版本反序列化回调会用 MPNNames 解析结果覆盖它 / Numeric MPN array at Key 1, overwritten by parsed MPNNames in the current-version deserialization callback
	LayerName             *string                      `json:"layerName"`                       // Key 2 的 SavedTexData 层名称 / SavedTexData layer name at Key 2
	ViewName              *string                      `json:"viewName"`                        // Key 3 的界面显示名称 / UI display name at Key 3
	Type                  ColorPresetPackType          `json:"type"`                            // Key 4 的颜色与透明度应用模式 / Color and alpha application mode at Key 4
	ColorList             []*ColorPresetLayerFreeColor `json:"colorList"`                       // Key 5 的普通层颜色列表 / Normal layer-color list at Key 5
	GradationColorList    []*ColorPresetGradationColor `json:"gradationColorList"`              // Key 6 的渐变颜色点列表 / Gradation-color point list at Key 6
	Alpha                 float32                      `json:"alpha"`                           // Key 7 的乘算透明度 / Multiplied alpha at Key 7
	AllowedMPNOverride    bool                         `json:"allowedMpnOverRide"`              // Key 8 的 MPN 覆盖许可标志 / MPN override permission flag at Key 8
	MPNNames              []*string                    `json:"mpnNames"`                        // Key 9 的项目可空 MPN 名称数组，当前反序列化回调用它重建 MPNs / MPN-name array with nullable entries at Key 9, used by the current deserialization callback to rebuild MPNs
	LegacyMPNNamesOmitted bool                         `json:"legacyMpnNamesOmitted,omitempty"` // 旧版九槽对象未保存 Key 9 的线格式标记，并非游戏成员 / Wire marker for a legacy nine-slot object that did not store Key 9, not a game member
}

// ColorPresetFreeColor 公开 FreeColor 的四个私有原始字段
// MessagePack 私有成员解析器不会让这些值经过带范围钳制的公开属性
// ColorPresetFreeColor exposes the four private raw fields of FreeColor
// The MessagePack private-member resolver does not pass these values through the clamping public properties
type ColorPresetFreeColor struct {
	Version    int32 `json:"version"`    // Key 0 的 FreeColor 版本，当前 FixVersion 为 1000 / FreeColor version at Key 0, with a current FixVersion of 1000
	Hue        int32 `json:"hue"`        // Key 1 的私有原始色相 hue_ / Private raw hue_ value at Key 1
	Saturation int32 `json:"saturation"` // Key 2 的私有原始饱和度 saturation_ / Private raw saturation_ value at Key 2
	Brightness int32 `json:"brightness"` // Key 3 的私有原始亮度 brightness_，不是 rawBrighteness 加 255 表示 / Private raw brightness_ value at Key 3, not the rawBrighteness plus-255 representation
	Contrast   int32 `json:"contrast"`   // Key 4 的私有原始对比度 contrast_ / Private raw contrast_ value at Key 4
}

// ColorPresetLayerFreeColor 表示 LayerFreeColor 包含继承版本在内的四槽 indexed object
// ColorPresetLayerFreeColor represents the four-slot LayerFreeColor indexed object including its inherited version
type ColorPresetLayerFreeColor struct {
	Version     int32                 `json:"version"`     // Key 0 的 LayerFreeColor 版本，当前 FixVersion 为 1000 / LayerFreeColor version at Key 0, with a current FixVersion of 1000
	BaseColor   *ColorPresetFreeColor `json:"baseColor"`   // Key 1 的可空基础 FreeColor / Nullable base FreeColor at Key 1
	ShadowColor *ColorPresetFreeColor `json:"shadowColor"` // Key 2 的可空阴影 FreeColor / Nullable shadow FreeColor at Key 2
	ShadowRate  int32                 `json:"shadowRate"`  // Key 3 的私有原始阴影比例 shadowRate_ / Private raw shadowRate_ value at Key 3
}

// ColorPresetControlSlider 只包含 ControlSlider 唯一序列化的私有 value_ 字段
// readonly range 被 MessagePack 忽略，并由 C# 构造函数重建为 0 至 1
// ColorPresetControlSlider contains the only serialized private value_ field of ControlSlider
// Its readonly range is ignored by MessagePack and rebuilt as 0 through 1 by the C# constructor
type ColorPresetControlSlider struct {
	Value float32 `json:"value"` // Key 0 的私有原始滑块值 value_，私有成员解析不会钳制它 / Private raw slider value_ at Key 0, not clamped by private-member deserialization
}

// ColorPresetGradationColor 在 LayerFreeColor 四个槽位后增加三个 ControlSlider
// ColorPresetGradationColor extends the four LayerFreeColor slots with three ControlSlider values
type ColorPresetGradationColor struct {
	Version     int32                     `json:"version"`                 // Key 0 的 GradationColor 版本，当前 FixVersion 为 1000 / GradationColor version at Key 0, with a current FixVersion of 1000
	BaseColor   *ColorPresetFreeColor     `json:"baseColor"`               // Key 1 的可空基础 FreeColor / Nullable base FreeColor at Key 1
	ShadowColor *ColorPresetFreeColor     `json:"shadowColor"`             // Key 2 的可空阴影 FreeColor / Nullable shadow FreeColor at Key 2
	ShadowRate  int32                     `json:"shadowRate"`              // Key 3 的私有原始阴影比例 shadowRate_ / Private raw shadowRate_ value at Key 3
	Position    *ColorPresetControlSlider `json:"controlPointPosition"`    // Key 4 的渐变控制点中心位置滑块 / Gradation control-point center-position slider at Key 4
	RangeBefore *ColorPresetControlSlider `json:"controlPointRangeBefore"` // Key 5 的控制点前侧范围滑块 / Control-point before-range slider at Key 5
	RangeAfter  *ColorPresetControlSlider `json:"controlPointRangeAfter"`  // Key 6 的控制点后侧范围滑块 / Control-point after-range slider at Key 6
}

// NewColorPreset 创建与 C# 构造函数一致的确定性默认值，但要求调用者提供 GUID，避免 Guid.NewGuid 在序列化代码中隐藏身份变化
// NewColorPreset creates the same deterministic defaults as the C# constructor but requires the caller to supply the GUID so Guid.NewGuid cannot hide identity changes inside serialization code
func NewColorPreset(instanceGUID string) (*ColorPreset, error) {
	if err := validateColorPresetConstructorGUID(instanceGUID, "ColorPreset.instanceGuid"); err != nil {
		return nil, err
	}
	return newColorPresetDefaults(instanceGUID), nil
}

// NewColorPresetSlot 是线格式相同的 ColorPresetSlot 对应显式构造函数
// NewColorPresetSlot is the corresponding explicit constructor for the wire-identical ColorPresetSlot type
func NewColorPresetSlot(instanceGUID string) (*ColorPresetSlot, error) {
	return NewColorPreset(instanceGUID)
}

// newColorPresetDefaults 创建 CustomColorPresetBase 构造后的版本、空色包列表和指定实例标识
// newColorPresetDefaults creates the version, empty color-pack list, and supplied instance identifier produced after CustomColorPresetBase construction
func newColorPresetDefaults(instanceGUID string) *ColorPreset {
	return &ColorPreset{
		Version:           ColorPresetVersion,
		ColorPackList:     make([]*ColorPresetColorPack, 0),
		InstanceGUID:      &instanceGUID,
		IndexedArrayWidth: 7,
	}
}

// newColorPresetPackDefaults 创建 CustomColorPresetColorPack 字段初始化器产生的当前版本与两个空颜色列表
// newColorPresetPackDefaults creates the current version and two empty color lists produced by CustomColorPresetColorPack field initializers
func newColorPresetPackDefaults() *ColorPresetColorPack {
	return &ColorPresetColorPack{
		Version:            ColorPresetPackVersion,
		ColorList:          make([]*ColorPresetLayerFreeColor, 0),
		GradationColorList: make([]*ColorPresetGradationColor, 0),
	}
}

// newColorPresetFreeColorDefaults 创建只设置当前 FixVersion 的 FreeColor
// newColorPresetFreeColorDefaults creates a FreeColor with only its current FixVersion set
func newColorPresetFreeColorDefaults() *ColorPresetFreeColor {
	return &ColorPresetFreeColor{Version: ColorPresetColorVersion}
}

// newColorPresetLayerDefaults 创建带当前版本及两个非 nil 默认 FreeColor 的 LayerFreeColor
// newColorPresetLayerDefaults creates a LayerFreeColor with the current version and two non-nil default FreeColor values
func newColorPresetLayerDefaults() *ColorPresetLayerFreeColor {
	return &ColorPresetLayerFreeColor{
		Version:     ColorPresetColorVersion,
		BaseColor:   newColorPresetFreeColorDefaults(),
		ShadowColor: newColorPresetFreeColorDefaults(),
	}
}

// newColorPresetGradationDefaults 创建带默认基础色、阴影色和三个 ControlSlider 的 GradationColor
// newColorPresetGradationDefaults creates a GradationColor with default base color, shadow color, and three ControlSlider values
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

// DecodeColorPreset 解码完整 PrivateLz4BlockArray 用户预设而不调用构造默认值、迁移或序列化回调
// DecodeColorPreset decodes a complete PrivateLz4BlockArray user preset without invoking constructor defaults, migrations, or serialization callbacks
func DecodeColorPreset(data []byte) (*ColorPreset, error) {
	return decodeColorPreset(data, "")
}

// DecodeColorPresetWithInstanceGUID 在 Key 6 缺失、nil 或为空时提供 C# 对象构造函数与 OnAfterDeserialize 回调原本会生成的确定值
// 这是显式选择的便利接口，非空线格式值仍具权威性并作为不透明文本处理
// DecodeColorPresetWithInstanceGUID supplies the deterministic value otherwise generated by the C# object constructor and OnAfterDeserialize callback when Key 6 is absent, nil, or empty
// This is an explicit opt-in convenience while a non-empty wire value remains authoritative and is treated as opaque text
func DecodeColorPresetWithInstanceGUID(data []byte, constructorGUID string) (*ColorPreset, error) {
	if err := validateColorPresetConstructorGUID(constructorGUID, "ColorPreset constructor instanceGuid"); err != nil {
		return nil, err
	}
	return decodeColorPreset(data, constructorGUID)
}

// DecodeColorPresetSlot 解码与 ColorPreset 线格式相同的 ColorPresetSlot 载荷
// DecodeColorPresetSlot decodes the ColorPresetSlot payload whose wire form is identical to ColorPreset
func DecodeColorPresetSlot(data []byte) (*ColorPresetSlot, error) {
	return DecodeColorPreset(data)
}

// DecodeColorPresetSlotWithInstanceGUID 为 instanceGuid 缺失的 ColorPresetSlot 载荷提供确定性构造形式
// DecodeColorPresetSlotWithInstanceGUID provides the deterministic constructor form for a ColorPresetSlot payload with a missing instanceGuid field
func DecodeColorPresetSlotWithInstanceGUID(data []byte, constructorGUID string) (*ColorPresetSlot, error) {
	return DecodeColorPresetWithInstanceGUID(data, constructorGUID)
}

// decodeColorPreset 解压颜色预设根值，兼容六槽、七槽与 KCES2 十一槽布局，并要求完整消费输入
// decodeColorPreset decompresses a color-preset root, accepts the 6-slot, 7-slot, and KCES2 11-slot layouts, and requires complete input consumption
func decodeColorPreset(data []byte, constructorGUID string) (*ColorPreset, error) {
	raw, err := msgpack.DecompressLz4BlockArray(data)
	if err != nil {
		return nil, fmt.Errorf("decompress ColorPreset PrivateLz4BlockArray: %w", err)
	}

	r := simpleEditDataReader{data: raw}
	if r.tryReadNil() {
		if err := r.requireEOF("ColorPreset"); err != nil {
			return nil, err
		}
		return nil, nil
	}
	fieldCount, err := colorPresetReadObjectHeader(&r, "ColorPreset")
	if err != nil {
		return nil, err
	}
	if fieldCount != 6 && fieldCount != 7 && fieldCount != 11 {
		return nil, fmt.Errorf("unsupported ColorPreset indexed-array width %d, expected 6, 7, or 11", fieldCount)
	}
	value := &ColorPreset{
		IndexedArrayWidth:         int32(fieldCount),
		LegacyInstanceGUIDOmitted: fieldCount == 6,
	}
	value.Version, err = r.readInt32("ColorPreset.version")
	if err != nil {
		return nil, err
	}
	value.ID, err = colorPresetReadNullableString(&r, "ColorPreset.id")
	if err != nil {
		return nil, err
	}
	value.BaseMenuFile, err = colorPresetReadNullableString(&r, "ColorPreset.baseMenuFile")
	if err != nil {
		return nil, err
	}
	value.UserCreated, err = colorPresetReadBool(&r, "ColorPreset.userCreated")
	if err != nil {
		return nil, err
	}
	value.IsAdvancedMode, err = colorPresetReadBool(&r, "ColorPreset.isAdvancedMode_")
	if err != nil {
		return nil, err
	}
	value.ColorPackList, err = colorPresetReadPackList(&r, "ColorPreset.colorPackList")
	if err != nil {
		return nil, err
	}
	if fieldCount >= 7 {
		value.InstanceGUID, err = colorPresetReadNullableString(&r, "ColorPreset.instanceGuid")
		if err != nil {
			return nil, err
		}
	}
	if fieldCount == 11 {
		value.SaveLocationHash, err = colorPresetReadUint64(&r, "ColorPreset.saveLocationHash")
		if err != nil {
			return nil, err
		}
		value.CreationTicks, err = colorPresetReadInt64(&r, "ColorPreset.creationTicks")
		if err != nil {
			return nil, err
		}
		value.LastUpdateTicks, err = colorPresetReadInt64(&r, "ColorPreset.lastUpdateTicks")
		if err != nil {
			return nil, err
		}
		value.MetaTexts, err = colorPresetReadStringMap(&r, "ColorPreset.metaTexts")
		if err != nil {
			return nil, err
		}
	}
	if constructorGUID != "" && (value.InstanceGUID == nil || *value.InstanceGUID == "") {
		value.InstanceGUID = &constructorGUID
		value.LegacyInstanceGUIDOmitted = false
		if value.IndexedArrayWidth == 6 {
			value.IndexedArrayWidth = 7
		}
	}
	if err := r.requireEOF("ColorPreset"); err != nil {
		return nil, err
	}
	if err := validateDecodedColorPreset(value); err != nil {
		return nil, err
	}
	return value, nil
}

// EncodeColorPreset 按记录的六槽、七槽或 KCES2 十一槽布局写出预设，不调用游戏迁移或序列化回调
// 所有显式版本以及数值和名称两组 MPN 数组都按调用者提供内容保留
// EncodeColorPreset emits the recorded 6-slot, 7-slot, or KCES2 11-slot layout without invoking game migrations or serialization callbacks
// Every explicit version and both numeric and named MPN arrays are preserved as supplied by the caller
func EncodeColorPreset(value *ColorPreset) ([]byte, error) {
	if value == nil {
		return []byte{0xc0}, nil
	}
	if err := validateColorPresetForEncoding(value); err != nil {
		return nil, err
	}

	fieldCount, err := colorPresetWireWidth(value)
	if err != nil {
		return nil, err
	}
	raw := simpleEditDataAppendArrayHeader(nil, int64(fieldCount))
	raw = simpleEditDataAppendInt32(raw, value.Version)
	raw = colorPresetAppendNullableString(raw, value.ID)
	raw = colorPresetAppendNullableString(raw, value.BaseMenuFile)
	raw = colorPresetAppendBool(raw, value.UserCreated)
	raw = colorPresetAppendBool(raw, value.IsAdvancedMode)
	if value.ColorPackList == nil {
		raw = append(raw, 0xc0)
	} else {
		raw = simpleEditDataAppendArrayHeader(raw, int64(len(value.ColorPackList)))
		for index, pack := range value.ColorPackList {
			var err error
			raw, err = colorPresetAppendPack(raw, pack, fmt.Sprintf("ColorPreset.colorPackList[%d]", index))
			if err != nil {
				return nil, err
			}
		}
	}
	if fieldCount >= 7 {
		raw = colorPresetAppendNullableString(raw, value.InstanceGUID)
	}
	if fieldCount == 11 {
		raw = colorPresetAppendUint64(raw, value.SaveLocationHash)
		raw = colorPresetAppendInt64(raw, value.CreationTicks)
		raw = colorPresetAppendInt64(raw, value.LastUpdateTicks)
		raw = colorPresetAppendStringMap(raw, value.MetaTexts)
	}
	return colorPresetCompress(raw)
}

// colorPresetWireWidth 返回调用方选择的颜色预设根数组宽度并拒绝不可表示字段
// colorPresetWireWidth returns the caller-selected color-preset root width and rejects fields that cannot be represented
func colorPresetWireWidth(value *ColorPreset) (int32, error) {
	width := value.IndexedArrayWidth
	if width == 0 {
		if value.LegacyInstanceGUIDOmitted {
			width = 6
		} else {
			width = 7
		}
	}
	if width != 6 && width != 7 && width != 11 {
		return 0, fmt.Errorf("unsupported ColorPreset indexed-array width %d, expected 6, 7, or 11", width)
	}
	if value.LegacyInstanceGUIDOmitted && width != 6 {
		return 0, fmt.Errorf("legacyInstanceGuidOmitted requires ColorPreset indexed-array width 6")
	}
	if width == 6 && value.InstanceGUID != nil {
		return 0, fmt.Errorf("ColorPreset.instanceGuid is not representable in indexed-array width 6")
	}
	if width < 11 && (value.SaveLocationHash != 0 || value.CreationTicks != 0 || value.LastUpdateTicks != 0 || value.MetaTexts != nil) {
		return 0, fmt.Errorf("KCES2 ColorPreset metadata fields are not representable in indexed-array width %d", width)
	}
	return width, nil
}

// EncodeColorPresetSlot 写出与 ColorPreset 线格式相同的 ColorPresetSlot 载荷
// EncodeColorPresetSlot emits the ColorPresetSlot payload whose wire form is identical to ColorPreset
func EncodeColorPresetSlot(value *ColorPresetSlot) ([]byte, error) {
	return EncodeColorPreset(value)
}

// validateDecodedColorPreset 对解码结果应用与无损重编码相同的范围和嵌套值校验
// validateDecodedColorPreset applies the same range and nested-value checks required for lossless re-encoding to a decoded result
func validateDecodedColorPreset(value *ColorPreset) error {
	return validateColorPresetForEncoding(value)
}

// validateColorPresetForEncoding 验证根字符串、Int32、集合长度及所有嵌套色包可由目标 MessagePack 线格式表示
// validateColorPresetForEncoding verifies that root strings, Int32 values, collection lengths, and every nested color pack fit the target MessagePack wire form
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
	if err := colorPresetValidateNullableString(value.InstanceGUID, "ColorPreset.instanceGuid"); err != nil {
		return err
	}
	if _, err := colorPresetWireWidth(value); err != nil {
		return err
	}
	if uint64(len(value.MetaTexts)) > math.MaxUint32 {
		return fmt.Errorf("ColorPreset.metaTexts length %d exceeds the MessagePack map32 limit", len(value.MetaTexts))
	}
	for key, text := range value.MetaTexts {
		if err := colorPresetValidateString(key, "ColorPreset.metaTexts key"); err != nil {
			return err
		}
		if text != nil {
			if err := colorPresetValidateString(*text, fmt.Sprintf("ColorPreset.metaTexts[%q]", key)); err != nil {
				return err
			}
		}
	}
	for index, pack := range value.ColorPackList {
		if err := validateColorPresetPack(pack, fmt.Sprintf("ColorPreset.colorPackList[%d]", index), false); err != nil {
			return err
		}
	}
	return nil
}

// 游戏构造函数使用 Guid.NewGuid().ToString 初始化 instanceGuid
// 模拟这条随机构造路径的调用者必须提供 D 格式 GUID，已有非空线格式标识仍保持不透明
// Game constructors initialize instanceGuid with Guid.NewGuid().ToString
// Callers simulating that random constructor path must provide a D-format GUID while existing non-empty wire identifiers remain opaque
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

// colorPresetReadPackList 读取可为 nil 且项目也可为 nil 的 CustomColorPresetColorPack 列表
// colorPresetReadPackList reads a CustomColorPresetColorPack list that may be nil and whose entries may also be nil
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

// colorPresetReadPack 读取一个可空色包，兼容 mpnNames 加入前的九槽布局与当前十槽布局
// colorPresetReadPack reads one nullable color pack and accepts both the nine-slot layout predating mpnNames and the current ten-slot layout
func colorPresetReadPack(r *simpleEditDataReader, path string) (*ColorPresetColorPack, error) {
	if r.tryReadNil() {
		return nil, nil
	}
	fieldCount, err := colorPresetReadObjectHeader(r, path)
	if err != nil {
		return nil, err
	}
	if fieldCount != 9 && fieldCount != 10 {
		return nil, fmt.Errorf("unsupported %s indexed-array width %d, expected 9 or 10", path, fieldCount)
	}
	value := &ColorPresetColorPack{LegacyMPNNamesOmitted: fieldCount == 9}
	value.Version, err = r.readInt32(path + ".version")
	if err != nil {
		return nil, err
	}
	value.MPNs, err = colorPresetReadInt32Array(r, path+".mpns")
	if err != nil {
		return nil, err
	}
	value.LayerName, err = colorPresetReadNullableString(r, path+".layerName")
	if err != nil {
		return nil, err
	}
	value.ViewName, err = colorPresetReadNullableString(r, path+".viewName")
	if err != nil {
		return nil, err
	}
	typeValue, readErr := r.readInt32(path + ".type")
	if readErr != nil {
		return nil, readErr
	}
	value.Type = ColorPresetPackType(typeValue)
	value.ColorList, err = colorPresetReadLayerList(r, path+".colorList")
	if err != nil {
		return nil, err
	}
	value.GradationColorList, err = colorPresetReadGradationList(r, path+".gradationColorList")
	if err != nil {
		return nil, err
	}
	value.Alpha, err = colorPresetReadSingle(r, path+".alpha")
	if err != nil {
		return nil, err
	}
	value.AllowedMPNOverride, err = colorPresetReadBool(r, path+".allowedMpnOverRide")
	if err != nil {
		return nil, err
	}
	if fieldCount == 10 {
		value.MPNNames, err = colorPresetReadStringArray(r, path+".mpnNames")
		if err != nil {
			return nil, err
		}
	}
	if err := validateColorPresetPack(value, path, true); err != nil {
		return nil, err
	}
	return value, nil
}

// validateColorPresetPack 验证色包字段、MPN 数组及嵌套颜色列表可由目标类型表示
// validateColorPresetPack verifies that color-pack fields, MPN arrays, and nested color lists fit their target types
func validateColorPresetPack(value *ColorPresetColorPack, path string, decoded bool) error {
	if value == nil {
		return nil
	}

	if uint64(len(value.MPNs)) > math.MaxUint32 || uint64(len(value.MPNNames)) > math.MaxUint32 {
		return fmt.Errorf("%s MPN collection exceeds the MessagePack array32 limit", path)
	}
	for index, name := range value.MPNNames {
		if name == nil {
			continue
		}
		if err := colorPresetValidateString(*name, fmt.Sprintf("%s.mpnNames[%d]", path, index)); err != nil {
			return err
		}
	}
	if value.LegacyMPNNamesOmitted && value.MPNNames != nil {
		return fmt.Errorf("%s.mpnNames must be nil when legacyMpnNamesOmitted is true", path)
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

// colorPresetAppendPack 按解码时记录的九槽旧布局或当前十槽布局写入一个可空 CustomColorPresetColorPack
// colorPresetAppendPack writes one nullable CustomColorPresetColorPack using either the recorded legacy nine-slot layout or the current ten-slot layout
func colorPresetAppendPack(dst []byte, value *ColorPresetColorPack, path string) ([]byte, error) {
	if value == nil {
		return append(dst, 0xc0), nil
	}
	if err := validateColorPresetPack(value, path, false); err != nil {
		return nil, err
	}
	fieldCount := int64(10)
	if value.LegacyMPNNamesOmitted {
		fieldCount = 9
	}
	dst = simpleEditDataAppendArrayHeader(dst, fieldCount)
	dst = simpleEditDataAppendInt32(dst, value.Version)
	if value.MPNs == nil {
		dst = append(dst, 0xc0)
	} else {
		dst = simpleEditDataAppendArrayHeader(dst, int64(len(value.MPNs)))
		for _, mpn := range value.MPNs {
			dst = simpleEditDataAppendInt32(dst, mpn)
		}
	}
	dst = colorPresetAppendNullableString(dst, value.LayerName)
	dst = colorPresetAppendNullableString(dst, value.ViewName)
	dst = simpleEditDataAppendInt32(dst, int32(value.Type))
	if value.ColorList == nil {
		dst = append(dst, 0xc0)
	} else {
		dst = simpleEditDataAppendArrayHeader(dst, int64(len(value.ColorList)))
		for index, color := range value.ColorList {
			var err error
			dst, err = colorPresetAppendLayer(dst, color, fmt.Sprintf("%s.colorList[%d]", path, index))
			if err != nil {
				return nil, err
			}
		}
	}
	if value.GradationColorList == nil {
		dst = append(dst, 0xc0)
	} else {
		dst = simpleEditDataAppendArrayHeader(dst, int64(len(value.GradationColorList)))
		for index, color := range value.GradationColorList {
			var err error
			dst, err = colorPresetAppendGradation(dst, color, fmt.Sprintf("%s.gradationColorList[%d]", path, index))
			if err != nil {
				return nil, err
			}
		}
	}
	dst = colorPresetAppendFloat32(dst, value.Alpha)
	dst = colorPresetAppendBool(dst, value.AllowedMPNOverride)
	if !value.LegacyMPNNamesOmitted {
		if value.MPNNames == nil {
			dst = append(dst, 0xc0)
		} else {
			dst = simpleEditDataAppendArrayHeader(dst, int64(len(value.MPNNames)))
			for _, name := range value.MPNNames {
				if name == nil {
					dst = append(dst, 0xc0)
				} else {
					dst = simpleEditDataAppendString(dst, *name)
				}
			}
		}
	}
	return dst, nil
}

// colorPresetReadLayerList 读取可为 nil 且项目也可为 nil 的 LayerFreeColor 列表
// colorPresetReadLayerList reads a LayerFreeColor list that may be nil and whose entries may also be nil
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

// colorPresetReadLayer 按 LayerFreeColor 的固定四槽布局读取一个可空普通层颜色
// colorPresetReadLayer reads one nullable normal layer color using the fixed four-slot LayerFreeColor layout
func colorPresetReadLayer(r *simpleEditDataReader, path string) (*ColorPresetLayerFreeColor, error) {
	if r.tryReadNil() {
		return nil, nil
	}
	fieldCount, err := colorPresetReadObjectHeader(r, path)
	if err != nil {
		return nil, err
	}
	if fieldCount != 4 {
		return nil, fmt.Errorf("unsupported %s indexed-array width %d, expected 4", path, fieldCount)
	}
	value := &ColorPresetLayerFreeColor{}
	value.Version, err = r.readInt32(path + ".version")
	if err != nil {
		return nil, err
	}
	value.BaseColor, err = colorPresetReadFreeColor(r, path+".baseColor")
	if err != nil {
		return nil, err
	}
	value.ShadowColor, err = colorPresetReadFreeColor(r, path+".shadowColor")
	if err != nil {
		return nil, err
	}
	value.ShadowRate, err = r.readInt32(path + ".shadowRate_")
	if err != nil {
		return nil, err
	}
	return value, nil
}

// validateColorPresetLayer 验证 LayerFreeColor 版本、嵌套 FreeColor 及私有阴影比例均可表示
// validateColorPresetLayer verifies that the LayerFreeColor version, nested FreeColor values, and private shadow rate are representable
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

// colorPresetAppendLayer 按固定四槽布局写入一个可空 LayerFreeColor
// colorPresetAppendLayer writes one nullable LayerFreeColor using the fixed four-slot layout
func colorPresetAppendLayer(dst []byte, value *ColorPresetLayerFreeColor, path string) ([]byte, error) {
	if value == nil {
		return append(dst, 0xc0), nil
	}
	if err := validateColorPresetLayer(value, path, false); err != nil {
		return nil, err
	}
	dst = simpleEditDataAppendArrayHeader(dst, 4)
	dst = simpleEditDataAppendInt32(dst, value.Version)
	var err error
	dst, err = colorPresetAppendFree(dst, value.BaseColor, path+".baseColor")
	if err != nil {
		return nil, err
	}
	dst, err = colorPresetAppendFree(dst, value.ShadowColor, path+".shadowColor")
	if err != nil {
		return nil, err
	}
	dst = simpleEditDataAppendInt32(dst, value.ShadowRate)
	return dst, nil
}

// colorPresetReadFreeColor 按 FreeColor 的固定五槽布局读取版本和四个私有原始字段
// colorPresetReadFreeColor reads one nullable color using the fixed five-slot FreeColor layout containing its version and four private raw fields
func colorPresetReadFreeColor(r *simpleEditDataReader, path string) (*ColorPresetFreeColor, error) {
	if r.tryReadNil() {
		return nil, nil
	}
	fieldCount, err := colorPresetReadObjectHeader(r, path)
	if err != nil {
		return nil, err
	}
	if fieldCount != 5 {
		return nil, fmt.Errorf("unsupported %s indexed-array width %d, expected 5", path, fieldCount)
	}
	value := &ColorPresetFreeColor{}
	fields := []*int32{&value.Version, &value.Hue, &value.Saturation, &value.Brightness, &value.Contrast}
	names := []string{"version", "hue_", "saturation_", "brightness_", "contrast_"}
	for index := int64(0); index < 5; index++ {
		*fields[index], err = r.readInt32(path + "." + names[index])
		if err != nil {
			return nil, err
		}
	}
	return value, nil
}

// validateColorPresetFree 验证 FreeColor 版本和四个私有原始整数均位于 System.Int32 范围
// validateColorPresetFree verifies that the FreeColor version and four private raw integers fit System.Int32
func validateColorPresetFree(value *ColorPresetFreeColor, path string, decoded bool) error {
	if value == nil {
		return nil
	}

	return nil
}

// colorPresetAppendFree 按固定五槽布局写入一个可空 FreeColor
// colorPresetAppendFree writes one nullable FreeColor using the fixed five-slot layout
func colorPresetAppendFree(dst []byte, value *ColorPresetFreeColor, path string) ([]byte, error) {
	if value == nil {
		return append(dst, 0xc0), nil
	}
	fields := []struct {
		name  string // 用于字段丢弃错误的名称 / Name used in field-discard errors
		value int32  // 对应槽位的原始整数值 / Raw integer value for the corresponding slot
	}{{"version", value.Version}, {"hue", value.Hue}, {"saturation", value.Saturation}, {"brightness", value.Brightness}, {"contrast", value.Contrast}}
	if err := validateColorPresetFree(value, path, false); err != nil {
		return nil, err
	}
	dst = simpleEditDataAppendArrayHeader(dst, 5)
	for index := int64(0); index < 5; index++ {
		dst = simpleEditDataAppendInt32(dst, fields[index].value)
	}
	return dst, nil
}

// colorPresetReadGradationList 读取可为 nil 且项目也可为 nil 的 GradationColor 列表
// colorPresetReadGradationList reads a GradationColor list that may be nil and whose entries may also be nil
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

// colorPresetReadGradation 按 GradationColor 的固定七槽布局读取一个可空渐变颜色
// colorPresetReadGradation reads one nullable gradation color using the fixed seven-slot GradationColor layout
func colorPresetReadGradation(r *simpleEditDataReader, path string) (*ColorPresetGradationColor, error) {
	if r.tryReadNil() {
		return nil, nil
	}
	fieldCount, err := colorPresetReadObjectHeader(r, path)
	if err != nil {
		return nil, err
	}
	if fieldCount != 7 {
		return nil, fmt.Errorf("unsupported %s indexed-array width %d, expected 7", path, fieldCount)
	}
	value := &ColorPresetGradationColor{}
	value.Version, err = r.readInt32(path + ".version")
	if err == nil {
		value.BaseColor, err = colorPresetReadFreeColor(r, path+".baseColor")
	}
	if err == nil {
		value.ShadowColor, err = colorPresetReadFreeColor(r, path+".shadowColor")
	}
	if err == nil {
		value.ShadowRate, err = r.readInt32(path + ".shadowRate_")
	}
	if err == nil {
		value.Position, err = colorPresetReadControlSlider(r, path+".controlPointPosition")
	}
	if err == nil {
		value.RangeBefore, err = colorPresetReadControlSlider(r, path+".controlPointRangeBefore")
	}
	if err == nil {
		value.RangeAfter, err = colorPresetReadControlSlider(r, path+".controlPointRangeAfter")
	}
	if err != nil {
		return nil, err
	}
	return value, nil
}

// validateColorPresetGradation 验证 GradationColor 版本、嵌套 FreeColor 和私有阴影比例均可表示
// validateColorPresetGradation verifies that the GradationColor version, nested FreeColor values, and private shadow rate are representable
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

// colorPresetAppendGradation 按固定七槽布局写入一个可空 GradationColor 及三个滑块
// colorPresetAppendGradation writes one nullable GradationColor and its three sliders using the fixed seven-slot layout
func colorPresetAppendGradation(dst []byte, value *ColorPresetGradationColor, path string) ([]byte, error) {
	if value == nil {
		return append(dst, 0xc0), nil
	}
	if err := validateColorPresetGradation(value, path, false); err != nil {
		return nil, err
	}
	dst = simpleEditDataAppendArrayHeader(dst, 7)
	dst = simpleEditDataAppendInt32(dst, value.Version)
	var err error
	dst, err = colorPresetAppendFree(dst, value.BaseColor, path+".baseColor")
	if err != nil {
		return nil, err
	}
	dst, err = colorPresetAppendFree(dst, value.ShadowColor, path+".shadowColor")
	if err != nil {
		return nil, err
	}
	dst = simpleEditDataAppendInt32(dst, value.ShadowRate)
	sliders := []struct {
		value *ColorPresetControlSlider // 待写入的可空滑块 / Nullable slider to write
		name  string                    // 用于字段路径的滑块名称 / Slider name used in field paths
	}{{value.Position, "controlPointPosition"}, {value.RangeBefore, "controlPointRangeBefore"}, {value.RangeAfter, "controlPointRangeAfter"}}
	for _, slider := range sliders {
		dst, err = colorPresetAppendControlSlider(dst, slider.value, path+"."+slider.name)
		if err != nil {
			return nil, err
		}
	}
	return dst, nil
}

// colorPresetAppendControlSlider 按固定一槽布局写入一个可空 ControlSlider 原始值
// colorPresetAppendControlSlider writes one nullable raw ControlSlider value using the fixed one-slot layout
func colorPresetAppendControlSlider(dst []byte, value *ColorPresetControlSlider, path string) ([]byte, error) {
	if value == nil {
		return append(dst, 0xc0), nil
	}
	dst = simpleEditDataAppendArrayHeader(dst, 1)
	dst = colorPresetAppendFloat32(dst, value.Value)
	return dst, nil
}

// colorPresetReadControlSlider 读取一个固定一槽的可空 ControlSlider 私有 value_
// colorPresetReadControlSlider reads the private value_ of one nullable fixed one-slot ControlSlider
func colorPresetReadControlSlider(r *simpleEditDataReader, path string) (*ColorPresetControlSlider, error) {
	if r.tryReadNil() {
		return nil, nil
	}
	fieldCount, err := colorPresetReadObjectHeader(r, path)
	if err != nil {
		return nil, err
	}
	if fieldCount != 1 {
		return nil, fmt.Errorf("unsupported %s indexed-array width %d, expected 1", path, fieldCount)
	}
	value := &ColorPresetControlSlider{}
	value.Value, err = colorPresetReadSingle(r, path+".value_")
	if err != nil {
		return nil, err
	}
	return value, nil
}

// colorPresetReadInt32Array 读取可空的 System.Int32 数组
// colorPresetReadInt32Array reads a nullable System.Int32 array
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

// colorPresetReadStringArray 读取列表及其可空字符串项目
// colorPresetReadStringArray reads a list whose string entries may be nil
func colorPresetReadStringArray(r *simpleEditDataReader, path string) ([]*string, error) {
	if r.tryReadNil() {
		return nil, nil
	}
	count, err := colorPresetReadCollectionHeader(r, path)
	if err != nil {
		return nil, err
	}
	result := makeKCESCountedSliceForAppend[*string](uint64(count))
	for index := int64(0); index < count; index++ {
		if r.tryReadNil() {
			result = append(result, nil)
			continue
		}
		value, readErr := r.readString(fmt.Sprintf("%s[%d]", path, index))
		err = readErr
		if err != nil {
			return nil, err
		}
		valueCopy := value
		result = append(result, &valueCopy)
	}
	return result, nil
}

// colorPresetReadNullableString 读取 MessagePack nil 或一个字符串并返回指针表示
// colorPresetReadNullableString reads MessagePack nil or one string and returns a pointer representation
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

// colorPresetReadBool 只接受 MessagePack false 或 true 标记
// colorPresetReadBool accepts only the MessagePack false or true markers
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

// colorPresetReadSingle 镜像 MessagePackReader.ReadSingle，接受所有整数编码以及 float32 和 float64 并转换为 System.Single
// colorPresetReadSingle mirrors MessagePackReader.ReadSingle by accepting every integer encoding plus float32 and float64 and converting to System.Single
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

// colorPresetReadUint64 按 MessagePackReader.ReadUInt64 兼容的整数标记读取 UInt64
// colorPresetReadUint64 reads a UInt64 from integer markers accepted by MessagePackReader.ReadUInt64
func colorPresetReadUint64(r *simpleEditDataReader, path string) (uint64, error) {
	marker, err := r.readByte(path)
	if err != nil {
		return 0, err
	}
	if marker <= 0x7f {
		return uint64(marker), nil
	}
	if marker >= 0xe0 {
		return 0, fmt.Errorf("%s cannot decode negative integer %d as UInt64", path, int8(marker))
	}
	switch marker {
	case 0xcc:
		value, err := r.readByte(path + " uint8")
		return uint64(value), err
	case 0xcd:
		data, err := r.readBytes(2, path+" uint16")
		if err != nil {
			return 0, err
		}
		return uint64(binary.BigEndian.Uint16(data)), nil
	case 0xce:
		data, err := r.readBytes(4, path+" uint32")
		if err != nil {
			return 0, err
		}
		return uint64(binary.BigEndian.Uint32(data)), nil
	case 0xcf:
		data, err := r.readBytes(8, path+" uint64")
		if err != nil {
			return 0, err
		}
		return binary.BigEndian.Uint64(data), nil
	case 0xd0:
		value, err := r.readByte(path + " int8")
		if err != nil {
			return 0, err
		}
		signed := int8(value)
		if signed < 0 {
			return 0, fmt.Errorf("%s cannot decode negative integer %d as UInt64", path, signed)
		}
		return uint64(signed), nil
	case 0xd1:
		data, err := r.readBytes(2, path+" int16")
		if err != nil {
			return 0, err
		}
		signed := int16(binary.BigEndian.Uint16(data))
		if signed < 0 {
			return 0, fmt.Errorf("%s cannot decode negative integer %d as UInt64", path, signed)
		}
		return uint64(signed), nil
	case 0xd2:
		data, err := r.readBytes(4, path+" int32")
		if err != nil {
			return 0, err
		}
		signed := int32(binary.BigEndian.Uint32(data))
		if signed < 0 {
			return 0, fmt.Errorf("%s cannot decode negative integer %d as UInt64", path, signed)
		}
		return uint64(signed), nil
	case 0xd3:
		data, err := r.readBytes(8, path+" int64")
		if err != nil {
			return 0, err
		}
		signed := int64(binary.BigEndian.Uint64(data))
		if signed < 0 {
			return 0, fmt.Errorf("%s cannot decode negative integer %d as UInt64", path, signed)
		}
		return uint64(signed), nil
	default:
		return 0, fmt.Errorf("%s must be a MessagePack UInt64-compatible integer, got marker 0x%02x", path, marker)
	}
}

// colorPresetReadInt64 按 MessagePackReader.ReadInt64 兼容的整数标记读取 Int64
// colorPresetReadInt64 reads an Int64 from integer markers accepted by MessagePackReader.ReadInt64
func colorPresetReadInt64(r *simpleEditDataReader, path string) (int64, error) {
	marker, err := r.readByte(path)
	if err != nil {
		return 0, err
	}
	if marker <= 0x7f {
		return int64(marker), nil
	}
	if marker >= 0xe0 {
		return int64(int8(marker)), nil
	}
	switch marker {
	case 0xcc:
		value, err := r.readByte(path + " uint8")
		return int64(value), err
	case 0xcd:
		data, err := r.readBytes(2, path+" uint16")
		if err != nil {
			return 0, err
		}
		return int64(binary.BigEndian.Uint16(data)), nil
	case 0xce:
		data, err := r.readBytes(4, path+" uint32")
		if err != nil {
			return 0, err
		}
		return int64(binary.BigEndian.Uint32(data)), nil
	case 0xcf:
		data, err := r.readBytes(8, path+" uint64")
		if err != nil {
			return 0, err
		}
		value := binary.BigEndian.Uint64(data)
		if value > math.MaxInt64 {
			return 0, fmt.Errorf("%s=%d exceeds Int64", path, value)
		}
		return int64(value), nil
	case 0xd0:
		value, err := r.readByte(path + " int8")
		return int64(int8(value)), err
	case 0xd1:
		data, err := r.readBytes(2, path+" int16")
		if err != nil {
			return 0, err
		}
		return int64(int16(binary.BigEndian.Uint16(data))), nil
	case 0xd2:
		data, err := r.readBytes(4, path+" int32")
		if err != nil {
			return 0, err
		}
		return int64(int32(binary.BigEndian.Uint32(data))), nil
	case 0xd3:
		data, err := r.readBytes(8, path+" int64")
		if err != nil {
			return 0, err
		}
		return int64(binary.BigEndian.Uint64(data)), nil
	default:
		return 0, fmt.Errorf("%s must be a MessagePack Int64-compatible integer, got marker 0x%02x", path, marker)
	}
}

// colorPresetReadStringMap 读取可为 nil 且值也可为 nil 的字符串字典并拒绝重复键
// colorPresetReadStringMap reads a nullable string map with nullable values and rejects duplicate keys
func colorPresetReadStringMap(r *simpleEditDataReader, path string) (map[string]*string, error) {
	if r.tryReadNil() {
		return nil, nil
	}
	count, err := r.readMapLength(path)
	if err != nil {
		return nil, err
	}
	if count > r.remaining()/2 {
		return nil, fmt.Errorf("%s count %d exceeds the capacity of %d remaining bytes", path, count, r.remaining())
	}
	result := makeKCESCountedMap[string, *string](uint64(count))
	for index := int64(0); index < count; index++ {
		key, err := r.readString(fmt.Sprintf("%s key %d", path, index))
		if err != nil {
			return nil, err
		}
		if _, duplicate := result[key]; duplicate {
			return nil, fmt.Errorf("%s contains duplicate key %q", path, key)
		}
		value, err := colorPresetReadNullableString(r, fmt.Sprintf("%s[%q]", path, key))
		if err != nil {
			return nil, err
		}
		result[key] = value
	}
	return result, nil
}

// colorPresetReadObjectHeader 读取 indexed object 数组头并按剩余字节限制字段数量
// colorPresetReadObjectHeader reads an indexed object array header and bounds its field count by remaining bytes
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

// colorPresetReadCollectionHeader 读取集合数组头并按剩余字节限制项目数量
// colorPresetReadCollectionHeader reads a collection array header and bounds its entry count by remaining bytes
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

// colorPresetValidateNullableString 在非 nil 时验证字符串的 UTF-8 与 str32 长度
// colorPresetValidateNullableString validates UTF-8 and str32 length when a string is non-nil
func colorPresetValidateNullableString(value *string, path string) error {
	if value == nil {
		return nil
	}
	return colorPresetValidateString(*value, path)
}

// colorPresetValidateString 验证字符串为 UTF-8 且字节长度可由 MessagePack str32 表示
// colorPresetValidateString verifies that a string is UTF-8 and its byte length fits MessagePack str32
func colorPresetValidateString(value, path string) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("%s is not valid UTF-8", path)
	}
	if uint64(len(value)) > math.MaxUint32 {
		return fmt.Errorf("%s is too large for MessagePack str32", path)
	}
	return nil
}

// colorPresetAppendNullableString 追加 MessagePack nil 或非空指针指向的字符串
// colorPresetAppendNullableString appends MessagePack nil or the string referenced by a non-nil pointer
func colorPresetAppendNullableString(dst []byte, value *string) []byte {
	if value == nil {
		return append(dst, 0xc0)
	}
	return simpleEditDataAppendString(dst, *value)
}

// colorPresetAppendUint64 以可表示给定 UInt64 值的最短 MessagePack 整数形式追加值
// colorPresetAppendUint64 appends a UInt64 using the shortest MessagePack integer representation
func colorPresetAppendUint64(dst []byte, value uint64) []byte {
	switch {
	case value <= 0x7f:
		return append(dst, byte(value))
	case value <= math.MaxUint8:
		return append(dst, 0xcc, byte(value))
	case value <= math.MaxUint16:
		return append(dst, 0xcd, byte(value>>8), byte(value))
	case value <= math.MaxUint32:
		dst = append(dst, 0xce, 0, 0, 0, 0)
		binary.BigEndian.PutUint32(dst[len(dst)-4:], uint32(value))
		return dst
	default:
		dst = append(dst, 0xcf, 0, 0, 0, 0, 0, 0, 0, 0)
		binary.BigEndian.PutUint64(dst[len(dst)-8:], value)
		return dst
	}
}

// colorPresetAppendInt64 以可表示给定 Int64 值的最短 MessagePack 整数形式追加值
// colorPresetAppendInt64 appends an Int64 using the shortest MessagePack integer representation
func colorPresetAppendInt64(dst []byte, value int64) []byte {
	if value >= 0 {
		return colorPresetAppendUint64(dst, uint64(value))
	}
	switch {
	case value >= -32:
		return append(dst, byte(int8(value)))
	case value >= math.MinInt8:
		return append(dst, 0xd0, byte(int8(value)))
	case value >= math.MinInt16:
		return append(dst, 0xd1, byte(int16(value)>>8), byte(int16(value)))
	case value >= math.MinInt32:
		dst = append(dst, 0xd2, 0, 0, 0, 0)
		binary.BigEndian.PutUint32(dst[len(dst)-4:], uint32(value))
		return dst
	default:
		dst = append(dst, 0xd3, 0, 0, 0, 0, 0, 0, 0, 0)
		binary.BigEndian.PutUint64(dst[len(dst)-8:], uint64(value))
		return dst
	}
}

// colorPresetAppendStringMap 以规范键顺序追加可为 nil 且值也可为 nil 的字符串字典
// colorPresetAppendStringMap appends a nullable string map with nullable values using canonical key order
func colorPresetAppendStringMap(dst []byte, value map[string]*string) []byte {
	if value == nil {
		return append(dst, 0xc0)
	}
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	dst = simpleEditDataAppendMapHeader(dst, int64(len(keys)))
	for _, key := range keys {
		dst = simpleEditDataAppendString(dst, key)
		dst = colorPresetAppendNullableString(dst, value[key])
	}
	return dst
}

// colorPresetAppendBool 追加标准 MessagePack false 或 true 标记
// colorPresetAppendBool appends the canonical MessagePack false or true marker
func colorPresetAppendBool(dst []byte, value bool) []byte {
	if value {
		return append(dst, 0xc3)
	}
	return append(dst, 0xc2)
}

// colorPresetAppendFloat32 使用标准 MessagePack float32 标记和大端位模式追加值
// colorPresetAppendFloat32 appends a value using the canonical MessagePack float32 marker and big-endian bit pattern
func colorPresetAppendFloat32(dst []byte, value float32) []byte {
	bits := math.Float32bits(value)
	return append(dst, 0xca, byte(bits>>24), byte(bits>>16), byte(bits>>8), byte(bits))
}

// colorPresetCompress 按 MessagePack-CSharp 的 64 字节阈值决定原样写出或 PrivateLz4BlockArray 包装
// colorPresetCompress chooses unchanged output or PrivateLz4BlockArray wrapping using the MessagePack-CSharp 64-byte threshold
func colorPresetCompress(raw []byte) ([]byte, error) {
	if len(raw) < colorPresetCompressionMinLength {
		return raw, nil
	}
	wire, err := msgpack.CompressLz4BlockArray(raw)
	if err != nil {
		return nil, fmt.Errorf("compress ColorPreset PrivateLz4BlockArray: %w", err)
	}
	return wire, nil
}
