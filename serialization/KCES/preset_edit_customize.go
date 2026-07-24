package KCES

import (
	"fmt"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	"github.com/ugorji/go/codec"
)

// .preset 的 EditCustomizeData 内部 MessagePack 模型，负责颜色预设引用以及部件位置等编辑数据
// 这些对象只存在于 maiddata 的属性块中，不是独立磁盘格式
// Internal EditCustomizeData MessagePack models for .preset covering color-preset references and part-position editing data
// These objects exist only inside maiddata property blocks and are not standalone disk formats

const (
	KCESPresetEditBaseDataVersion = 1000
	KCESPresetEditUnitDataVersion = 1000
)

// KCESPresetEditColorPreset 对应 EditCustomizeData.ColorPreset
// 嵌套 serializeBinary 字节串本身是完整且已知的 ColorPreset 或 ColorPresetSlot MessagePack 值，因此公开为强类型模型
// KCESPresetEditColorPreset corresponds to EditCustomizeData.ColorPreset
// Its nested serializeBinary byte string is a complete known ColorPreset or ColorPresetSlot MessagePack value and is therefore exposed as a typed model
type KCESPresetEditColorPreset struct {
	ID               *string      `json:"id"`               // 颜色预设标识，游戏也会保存菜单文件名 / Color-preset identifier, also populated with a menu filename by the game
	SerializedPreset *ColorPreset `json:"serializedPreset"` // 解码后的嵌套颜色预设 / Decoded nested color preset
}

// KCESPresetEditBaseData 是保存在 PropBase.editBaseData 中的 Standard MessagePack 对象
// KCESPresetEditBaseData is the Standard MessagePack object stored in PropBase.editBaseData
type KCESPresetEditBaseData struct {
	Version     int32                      `json:"version"`     // BaseData 对象版本 / BaseData object version
	ColorPreset *KCESPresetEditColorPreset `json:"colorPreset"` // 属性颜色预设引用 / Property color-preset reference
	Flags       map[string]*string         `json:"flags"`       // 编辑自定义字符串标志字典，值可为 nil / Edit-customization string flag map with nullable values
}

// KCESPresetEditUnitData 是保存在 SubProp.editUnitData 中的 Standard MessagePack 对象
// KCESPresetEditUnitData is the Standard MessagePack object stored in SubProp.editUnitData
type KCESPresetEditUnitData struct {
	Version      int32   `json:"version"`      // UnitData 对象版本 / UnitData object version
	PositionX    float32 `json:"positionX"`    // 编辑单位的 X 位置 / X position of the edit unit
	PositionY    float32 `json:"positionY"`    // 编辑单位的 Y 位置 / Y position of the edit unit
	WarpointName *string `json:"warpointName"` // 编辑单位的定位点名称 / Positioning-point name of the edit unit
}

// kcesPresetEditColorPresetWire 表示 EditCustomizeData.ColorPreset 的原始二槽布局
// kcesPresetEditColorPresetWire represents the raw two-slot layout of EditCustomizeData.ColorPreset
type kcesPresetEditColorPresetWire struct {
	_struct         struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	ID              *string  `json:"-"`         // 颜色预设标识 / Color-preset identifier
	SerializeBinary []byte   `json:"-"`         // 嵌套颜色预设 MessagePack 字节 / Nested color-preset MessagePack bytes
}

// kcesPresetEditBaseDataWire 表示 BaseData 的原始 indexed-array 布局
// kcesPresetEditBaseDataWire represents the raw indexed-array layout of BaseData
type kcesPresetEditBaseDataWire struct {
	_struct     struct{}                       `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	Version     int32                          `json:"-"`         // BaseData 对象版本 / BaseData object version
	ColorPreset *kcesPresetEditColorPresetWire `json:"-"`         // 原始颜色预设引用 / Raw color-preset reference
	Flags       map[string]*string             `json:"-"`         // 编辑自定义标志字典，值可为 nil / Edit-customization flag map with nullable values
}

// kcesPresetEditUnitDataWire 表示 UnitData 的原始 indexed-array 布局
// kcesPresetEditUnitDataWire represents the raw indexed-array layout of UnitData
type kcesPresetEditUnitDataWire struct {
	_struct      struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	Version      int32    `json:"-"`         // UnitData 对象版本 / UnitData object version
	PositionX    float32  `json:"-"`         // 编辑单位的 X 位置 / X position of the edit unit
	PositionY    float32  `json:"-"`         // 编辑单位的 Y 位置 / Y position of the edit unit
	WarpointName *string  `json:"-"`         // 编辑单位的定位点名称 / Positioning-point name of the edit unit
}

// CodecEncodeSelf 按共享 indexed-object 规则编码颜色预设引用线格式
// CodecEncodeSelf encodes the color-preset reference wire form using shared indexed-object rules
func (v kcesPresetEditColorPresetWire) CodecEncodeSelf(e *codec.Encoder) {
	ct.EncodeIndexedObjectSelf(e, &v)
}

// CodecDecodeSelf 按共享 indexed-object 规则解码颜色预设引用线格式
// CodecDecodeSelf decodes the color-preset reference wire form using shared indexed-object rules
func (v *kcesPresetEditColorPresetWire) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}

// CodecEncodeSelf 按共享 indexed-object 规则编码 BaseData 线格式
// CodecEncodeSelf encodes the BaseData wire form using shared indexed-object rules
func (v kcesPresetEditBaseDataWire) CodecEncodeSelf(e *codec.Encoder) {
	ct.EncodeIndexedObjectSelf(e, &v)
}

// CodecDecodeSelf 按共享 indexed-object 规则解码 BaseData 线格式
// CodecDecodeSelf decodes the BaseData wire form using shared indexed-object rules
func (v *kcesPresetEditBaseDataWire) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}

// CodecEncodeSelf 按共享 indexed-object 规则编码 UnitData 线格式
// CodecEncodeSelf encodes the UnitData wire form using shared indexed-object rules
func (v kcesPresetEditUnitDataWire) CodecEncodeSelf(e *codec.Encoder) {
	ct.EncodeIndexedObjectSelf(e, &v)
}

// CodecDecodeSelf 按共享 indexed-object 规则解码 UnitData 线格式
// CodecDecodeSelf decodes the UnitData wire form using shared indexed-object rules
func (v *kcesPresetEditUnitDataWire) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}

// DecodeKCESPresetEditBaseData 解码 PropBase.editBaseData 的 Standard MessagePack 根值
// DecodeKCESPresetEditBaseData decodes the Standard MessagePack root stored in PropBase.editBaseData
func DecodeKCESPresetEditBaseData(data []byte) (*KCESPresetEditBaseData, error) {
	root, rootNil, err := splitKCESPresetEditRoot(data, "EditCustomizeData.BaseData")
	if err != nil {
		return nil, err
	}
	if rootNil {
		return nil, nil
	}
	var wire kcesPresetEditBaseDataWire
	if err := decodeKCESPresetEditWire(root, &wire, "EditCustomizeData.BaseData"); err != nil {
		return nil, err
	}
	value := &KCESPresetEditBaseData{
		Version: wire.Version,
		Flags:   wire.Flags,
	}
	if wire.ColorPreset != nil {
		value.ColorPreset, err = expandKCESPresetEditColorPreset(wire.ColorPreset)
		if err != nil {
			return nil, fmt.Errorf("decode EditCustomizeData.BaseData.colorPreset: %w", err)
		}
	}
	return value, nil
}

// EncodeKCESPresetEditBaseData 按固定三槽布局编码 PropBase.editBaseData
// EncodeKCESPresetEditBaseData encodes PropBase.editBaseData using its fixed three-slot layout
func EncodeKCESPresetEditBaseData(value *KCESPresetEditBaseData) ([]byte, error) {
	if value == nil {
		return []byte{0xc0}, nil
	}
	wire := kcesPresetEditBaseDataWire{
		Version: value.Version,
		Flags:   value.Flags,
	}
	var err error
	if value.ColorPreset != nil {
		wire.ColorPreset, err = collapseKCESPresetEditColorPreset(value.ColorPreset)
		if err != nil {
			return nil, fmt.Errorf("encode EditCustomizeData.BaseData.colorPreset: %w", err)
		}
	}
	root, err := ct.EncodeIndexedMsgpack(&wire)
	if err != nil {
		return nil, fmt.Errorf("encode EditCustomizeData.BaseData MessagePack: %w", err)
	}
	return root, nil
}

// DecodeKCESPresetEditUnitData 解码 SubProp.editUnitData 的 Standard MessagePack 根值
// DecodeKCESPresetEditUnitData decodes the Standard MessagePack root stored in SubProp.editUnitData
func DecodeKCESPresetEditUnitData(data []byte) (*KCESPresetEditUnitData, error) {
	root, rootNil, err := splitKCESPresetEditRoot(data, "EditCustomizeData.UnitData")
	if err != nil {
		return nil, err
	}
	if rootNil {
		return nil, nil
	}
	var wire kcesPresetEditUnitDataWire
	if err := decodeKCESPresetEditWire(root, &wire, "EditCustomizeData.UnitData"); err != nil {
		return nil, err
	}
	return &KCESPresetEditUnitData{
		Version:      wire.Version,
		PositionX:    wire.PositionX,
		PositionY:    wire.PositionY,
		WarpointName: wire.WarpointName,
	}, nil
}

// EncodeKCESPresetEditUnitData 按固定四槽布局编码 SubProp.editUnitData
// EncodeKCESPresetEditUnitData encodes SubProp.editUnitData using its fixed four-slot layout
func EncodeKCESPresetEditUnitData(value *KCESPresetEditUnitData) ([]byte, error) {
	if value == nil {
		return []byte{0xc0}, nil
	}
	wire := kcesPresetEditUnitDataWire{
		Version:      value.Version,
		PositionX:    value.PositionX,
		PositionY:    value.PositionY,
		WarpointName: value.WarpointName,
	}
	root, err := ct.EncodeIndexedMsgpack(&wire)
	if err != nil {
		return nil, fmt.Errorf("encode EditCustomizeData.UnitData MessagePack: %w", err)
	}
	return root, nil
}

// NewKCESPresetEditBaseData 创建使用当前版本和游戏字段默认值的新 BaseData
// NewKCESPresetEditBaseData creates new BaseData with the current version and game field defaults
func NewKCESPresetEditBaseData() *KCESPresetEditBaseData {
	return &KCESPresetEditBaseData{
		Version:     KCESPresetEditBaseDataVersion,
		ColorPreset: &KCESPresetEditColorPreset{},
		Flags:       map[string]*string{},
	}
}

// NewKCESPresetEditUnitData 创建使用当前版本的新 UnitData
// NewKCESPresetEditUnitData creates new UnitData with the current version
func NewKCESPresetEditUnitData() *KCESPresetEditUnitData {
	return &KCESPresetEditUnitData{Version: KCESPresetEditUnitDataVersion}
}

// expandKCESPresetEditColorPreset 将嵌套 serializeBinary 展开为强类型颜色预设
// expandKCESPresetEditColorPreset expands nested serializeBinary into a typed color preset
func expandKCESPresetEditColorPreset(wire *kcesPresetEditColorPresetWire) (*KCESPresetEditColorPreset, error) {
	value := &KCESPresetEditColorPreset{
		ID: wire.ID,
	}
	if wire.SerializeBinary != nil {
		preset, err := DecodeColorPreset(wire.SerializeBinary)
		if err != nil {
			return nil, fmt.Errorf("decode serializeBinary ColorPreset: %w", err)
		}
		value.SerializedPreset = preset
	}
	return value, nil
}

// collapseKCESPresetEditColorPreset 将强类型颜色预设折叠回 serializeBinary 字节
// collapseKCESPresetEditColorPreset collapses a typed color preset back into serializeBinary bytes
func collapseKCESPresetEditColorPreset(value *KCESPresetEditColorPreset) (*kcesPresetEditColorPresetWire, error) {
	wire := &kcesPresetEditColorPresetWire{
		ID: value.ID,
	}
	if value.SerializedPreset != nil {
		data, err := EncodeColorPreset(value.SerializedPreset)
		if err != nil {
			return nil, fmt.Errorf("encode serializedPreset ColorPreset: %w", err)
		}
		wire.SerializeBinary = data
	}
	return wire, nil
}

// splitKCESPresetEditRoot 拆分唯一完整的 MessagePack 根值并返回 nil 状态
// splitKCESPresetEditRoot splits the sole complete MessagePack root value and reports its nil state
func splitKCESPresetEditRoot(data []byte, name string) (root []byte, rootNil bool, err error) {
	if len(data) == 1 && data[0] == 0xc0 {
		return append([]byte(nil), data...), true, nil
	}
	if len(data) == 0 {
		return nil, false, fmt.Errorf("decode %s root MessagePack: empty input", name)
	}
	return append([]byte(nil), data...), false, nil
}

// decodeKCESPresetEditWire 解码一个必须占满根值的 EditCustomizeData 线格式对象
// decodeKCESPresetEditWire decodes an EditCustomizeData wire object that must consume the complete root value
func decodeKCESPresetEditWire(root []byte, out interface{}, name string) error {
	if err := ct.DecodeMsgpack(root, out); err != nil {
		return fmt.Errorf("decode %s MessagePack: %w", name, err)
	}
	return nil
}
