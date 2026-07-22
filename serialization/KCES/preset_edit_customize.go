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
	*IndexedObjectMetadata              // ColorPreset 引用对象的线格式元数据 / Wire metadata for the ColorPreset reference object
	ID                     *string      `json:"id"`                              // 颜色预设标识，游戏也会保存菜单文件名 / Color-preset identifier, also populated with a menu filename by the game
	SerializedPreset       *ColorPreset `json:"serializedPreset"`                // 解码后的嵌套颜色预设 / Decoded nested color preset
	SerializedPresetEmpty  bool         `json:"serializedPresetEmpty,omitempty"` // serializeBinary 是否为非 nil 空数组 / Whether serializeBinary was a non-nil empty array
}

// KCESPresetEditBaseData 是保存在 PropBase.editBaseData 中的 Standard MessagePack 对象
// KCESPresetEditBaseData is the Standard MessagePack object stored in PropBase.editBaseData
type KCESPresetEditBaseData struct {
	MessagePackRootMetadata                            // 根值 nil 与尾部字节元数据 / Root nil and trailing-byte metadata
	*IndexedObjectMetadata                             // BaseData 的线格式元数据 / BaseData wire metadata
	Version                 int                        `json:"version"`     // BaseData 对象版本 / BaseData object version
	ColorPreset             *KCESPresetEditColorPreset `json:"colorPreset"` // 属性颜色预设引用 / Property color-preset reference
	Flags                   map[string]string          `json:"flags"`       // 编辑自定义字符串标志字典 / Edit-customization string flag map
}

// KCESPresetEditUnitData 是保存在 SubProp.editUnitData 中的 Standard MessagePack 对象
// KCESPresetEditUnitData is the Standard MessagePack object stored in SubProp.editUnitData
type KCESPresetEditUnitData struct {
	MessagePackRootMetadata         // 根值 nil 与尾部字节元数据 / Root nil and trailing-byte metadata
	*IndexedObjectMetadata          // UnitData 的线格式元数据 / UnitData wire metadata
	Version                 int     `json:"version"`      // UnitData 对象版本 / UnitData object version
	PositionX               float32 `json:"positionX"`    // 编辑单位的 X 位置 / X position of the edit unit
	PositionY               float32 `json:"positionY"`    // 编辑单位的 Y 位置 / Y position of the edit unit
	WarpointName            *string `json:"warpointName"` // 编辑单位的定位点名称 / Positioning-point name of the edit unit
}

// kcesPresetEditColorPresetWire 表示 EditCustomizeData.ColorPreset 的原始二槽布局
// kcesPresetEditColorPresetWire represents the raw two-slot layout of EditCustomizeData.ColorPreset
type kcesPresetEditColorPresetWire struct {
	_struct                struct{}    `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"` // ColorPreset 引用对象的线格式元数据 / Wire metadata for the ColorPreset reference object
	ID                     *string     `json:"-"` // 颜色预设标识 / Color-preset identifier
	SerializeBinary        []byte      `json:"-"` // 嵌套颜色预设 MessagePack 字节 / Nested color-preset MessagePack bytes
}

// kcesPresetEditBaseDataWire 表示 BaseData 的原始 indexed-array 布局
// kcesPresetEditBaseDataWire represents the raw indexed-array layout of BaseData
type kcesPresetEditBaseDataWire struct {
	_struct                struct{}                       `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`                    // BaseData 的线格式元数据 / BaseData wire metadata
	Version                int                            `json:"-"` // BaseData 对象版本 / BaseData object version
	ColorPreset            *kcesPresetEditColorPresetWire `json:"-"` // 原始颜色预设引用 / Raw color-preset reference
	Flags                  map[string]string              `json:"-"` // 编辑自定义标志字典 / Edit-customization flag map
}

// kcesPresetEditUnitDataWire 表示 UnitData 的原始 indexed-array 布局
// kcesPresetEditUnitDataWire represents the raw indexed-array layout of UnitData
type kcesPresetEditUnitDataWire struct {
	_struct                struct{}    `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"` // UnitData 的线格式元数据 / UnitData wire metadata
	Version                int         `json:"-"` // UnitData 对象版本 / UnitData object version
	PositionX              float32     `json:"-"` // 编辑单位的 X 位置 / X position of the edit unit
	PositionY              float32     `json:"-"` // 编辑单位的 Y 位置 / Y position of the edit unit
	WarpointName           *string     `json:"-"` // 编辑单位的定位点名称 / Positioning-point name of the edit unit
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
	root, trailing, rootNil, err := splitKCESPresetEditRoot(data, "EditCustomizeData.BaseData")
	if err != nil {
		return nil, err
	}
	if rootNil {
		return &KCESPresetEditBaseData{MessagePackRootMetadata: MessagePackRootMetadata{RootNil: true, TrailingData: trailing}}, nil
	}
	var wire kcesPresetEditBaseDataWire
	if err := decodeKCESPresetEditWire(root, &wire, "EditCustomizeData.BaseData"); err != nil {
		return nil, err
	}
	if err := requireInt32("EditCustomizeData.BaseData.version", wire.Version); err != nil {
		return nil, err
	}
	value := &KCESPresetEditBaseData{
		MessagePackRootMetadata: MessagePackRootMetadata{TrailingData: trailing},
		IndexedObjectMetadata:   wire.IndexedObjectMetadata,
		Version:                 wire.Version,
		Flags:                   wire.Flags,
	}
	if wire.ColorPreset != nil {
		value.ColorPreset, err = expandKCESPresetEditColorPreset(wire.ColorPreset)
		if err != nil {
			return nil, fmt.Errorf("decode EditCustomizeData.BaseData.colorPreset: %w", err)
		}
	}
	return value, nil
}

// EncodeKCESPresetEditBaseData 编码 PropBase.editBaseData 并保留根值尾部
// EncodeKCESPresetEditBaseData encodes PropBase.editBaseData and preserves root trailing bytes
func EncodeKCESPresetEditBaseData(value *KCESPresetEditBaseData) ([]byte, error) {
	if value == nil {
		return []byte{0xc0}, nil
	}
	if out, handled, err := encodeNilMessagePackRootIfRequested(
		value.MessagePackRootMetadata,
		value.Version != 0 || value.ColorPreset != nil || value.Flags != nil || kcesPresetEditMetadataHasPayload(value.IndexedObjectMetadata),
		"EditCustomizeData.BaseData",
	); handled {
		return out, err
	}
	if err := requireInt32("EditCustomizeData.BaseData.version", value.Version); err != nil {
		return nil, err
	}
	wire := kcesPresetEditBaseDataWire{
		IndexedObjectMetadata: value.IndexedObjectMetadata,
		Version:               value.Version,
		Flags:                 value.Flags,
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
	return appendMessagePackRootTrailing(root, value.MessagePackRootMetadata), nil
}

// DecodeKCESPresetEditUnitData 解码 SubProp.editUnitData 的 Standard MessagePack 根值
// DecodeKCESPresetEditUnitData decodes the Standard MessagePack root stored in SubProp.editUnitData
func DecodeKCESPresetEditUnitData(data []byte) (*KCESPresetEditUnitData, error) {
	root, trailing, rootNil, err := splitKCESPresetEditRoot(data, "EditCustomizeData.UnitData")
	if err != nil {
		return nil, err
	}
	if rootNil {
		return &KCESPresetEditUnitData{MessagePackRootMetadata: MessagePackRootMetadata{RootNil: true, TrailingData: trailing}}, nil
	}
	var wire kcesPresetEditUnitDataWire
	if err := decodeKCESPresetEditWire(root, &wire, "EditCustomizeData.UnitData"); err != nil {
		return nil, err
	}
	if err := requireInt32("EditCustomizeData.UnitData.version", wire.Version); err != nil {
		return nil, err
	}
	return &KCESPresetEditUnitData{
		MessagePackRootMetadata: MessagePackRootMetadata{TrailingData: trailing},
		IndexedObjectMetadata:   wire.IndexedObjectMetadata,
		Version:                 wire.Version,
		PositionX:               wire.PositionX,
		PositionY:               wire.PositionY,
		WarpointName:            wire.WarpointName,
	}, nil
}

// EncodeKCESPresetEditUnitData 编码 SubProp.editUnitData 并保留根值尾部
// EncodeKCESPresetEditUnitData encodes SubProp.editUnitData and preserves root trailing bytes
func EncodeKCESPresetEditUnitData(value *KCESPresetEditUnitData) ([]byte, error) {
	if value == nil {
		return []byte{0xc0}, nil
	}
	if out, handled, err := encodeNilMessagePackRootIfRequested(
		value.MessagePackRootMetadata,
		value.Version != 0 || value.PositionX != 0 || value.PositionY != 0 || value.WarpointName != nil || kcesPresetEditMetadataHasPayload(value.IndexedObjectMetadata),
		"EditCustomizeData.UnitData",
	); handled {
		return out, err
	}
	if err := requireInt32("EditCustomizeData.UnitData.version", value.Version); err != nil {
		return nil, err
	}
	wire := kcesPresetEditUnitDataWire{
		IndexedObjectMetadata: value.IndexedObjectMetadata,
		Version:               value.Version,
		PositionX:             value.PositionX,
		PositionY:             value.PositionY,
		WarpointName:          value.WarpointName,
	}
	root, err := ct.EncodeIndexedMsgpack(&wire)
	if err != nil {
		return nil, fmt.Errorf("encode EditCustomizeData.UnitData MessagePack: %w", err)
	}
	return appendMessagePackRootTrailing(root, value.MessagePackRootMetadata), nil
}

// NewKCESPresetEditBaseData 创建使用当前版本和游戏字段默认值的新 BaseData
// NewKCESPresetEditBaseData creates new BaseData with the current version and game field defaults
func NewKCESPresetEditBaseData() *KCESPresetEditBaseData {
	return &KCESPresetEditBaseData{
		Version:     KCESPresetEditBaseDataVersion,
		ColorPreset: &KCESPresetEditColorPreset{},
		Flags:       map[string]string{},
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
		IndexedObjectMetadata: wire.IndexedObjectMetadata,
		ID:                    wire.ID,
	}
	if wire.SerializeBinary != nil {
		if len(wire.SerializeBinary) == 0 {
			value.SerializedPresetEmpty = true
		} else {
			preset, err := DecodeColorPreset(wire.SerializeBinary)
			if err != nil {
				return nil, fmt.Errorf("decode serializeBinary ColorPreset: %w", err)
			}
			value.SerializedPreset = preset
		}
	}
	return value, nil
}

// collapseKCESPresetEditColorPreset 将强类型颜色预设折叠回 serializeBinary 字节
// collapseKCESPresetEditColorPreset collapses a typed color preset back into serializeBinary bytes
func collapseKCESPresetEditColorPreset(value *KCESPresetEditColorPreset) (*kcesPresetEditColorPresetWire, error) {
	if value.SerializedPresetEmpty && value.SerializedPreset != nil {
		return nil, fmt.Errorf("serializedPresetEmpty conflicts with populated serializedPreset")
	}
	wire := &kcesPresetEditColorPresetWire{
		IndexedObjectMetadata: value.IndexedObjectMetadata,
		ID:                    value.ID,
	}
	if value.SerializedPresetEmpty {
		wire.SerializeBinary = []byte{}
	} else if value.SerializedPreset != nil {
		data, err := EncodeColorPreset(value.SerializedPreset)
		if err != nil {
			return nil, fmt.Errorf("encode serializedPreset ColorPreset: %w", err)
		}
		wire.SerializeBinary = data
	}
	return wire, nil
}

// splitKCESPresetEditRoot 拆分首个 MessagePack 根值、尾部字节和 nil 状态
// splitKCESPresetEditRoot splits the first MessagePack root value, trailing bytes, and nil state
func splitKCESPresetEditRoot(data []byte, name string) (root, trailing []byte, rootNil bool, err error) {
	root, trailing, err = ct.SplitFirstMsgpackValue(data)
	if err != nil {
		return nil, nil, false, fmt.Errorf("decode %s root MessagePack: %w", name, err)
	}
	return root, trailing, len(root) == 1 && root[0] == 0xc0, nil
}

// decodeKCESPresetEditWire 解码一个必须占满根值的 EditCustomizeData 线格式对象
// decodeKCESPresetEditWire decodes an EditCustomizeData wire object that must consume the complete root value
func decodeKCESPresetEditWire(root []byte, out interface{}, name string) error {
	consumed, err := ct.DecodeMsgpackWithConsumed(root, out)
	if err != nil {
		return fmt.Errorf("decode %s MessagePack: %w", name, err)
	}
	if consumed != len(root) {
		return fmt.Errorf("decode %s consumed %d of %d root bytes", name, consumed, len(root))
	}
	return nil
}

// kcesPresetEditMetadataHasPayload 判断 indexed-object 元数据是否包含非默认线格式状态
// kcesPresetEditMetadataHasPayload reports whether indexed-object metadata contains nondefault wire state
func kcesPresetEditMetadataHasPayload(metadata *IndexedObjectMetadata) bool {
	return metadata != nil && indexedObjectMetadataHasPayload(*metadata)
}
