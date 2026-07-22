package KCES

import (
	"fmt"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	"github.com/ugorji/go/codec"
)

// .preset 的 EditCustomizeData 内部 MessagePack 模型，负责颜色预设引用以及部件位置等编辑数据。
// 这些对象只存在于 maiddata 的属性块中，不是独立磁盘格式。
//
// Internal EditCustomizeData MessagePack models for .preset, covering color-preset references and part-position editing data.
// These objects exist only inside maiddata property blocks and are not standalone disk formats.

const (
	KCESPresetEditBaseDataVersion = 1000
	KCESPresetEditUnitDataVersion = 1000
)

// KCESPresetEditColorPreset is EditCustomizeData.ColorPreset. The nested
// serializeBinary byte string is itself a fully known ColorPreset/
// ColorPresetSlot MessagePack value and is therefore exposed as a typed model.
type KCESPresetEditColorPreset struct {
	*IndexedObjectMetadata
	ID                    *string      `json:"id"`
	SerializedPreset      *ColorPreset `json:"serializedPreset"`
	SerializedPresetEmpty bool         `json:"serializedPresetEmpty,omitempty"`
}

// KCESPresetEditBaseData is the Standard MessagePack object stored in
// PropBase.editBaseData.
type KCESPresetEditBaseData struct {
	MessagePackRootMetadata
	*IndexedObjectMetadata
	Version     int                        `json:"version"`
	ColorPreset *KCESPresetEditColorPreset `json:"colorPreset"`
	Flags       map[string]string          `json:"flags"`
}

// KCESPresetEditUnitData is the Standard MessagePack object stored in
// SubProp.editUnitData.
type KCESPresetEditUnitData struct {
	MessagePackRootMetadata
	*IndexedObjectMetadata
	Version      int     `json:"version"`
	PositionX    float32 `json:"positionX"`
	PositionY    float32 `json:"positionY"`
	WarpointName *string `json:"warpointName"`
}

type kcesPresetEditColorPresetWire struct {
	_struct                struct{} `codec:",toarray"`
	*IndexedObjectMetadata `codec:"-"`
	ID                     *string `json:"-"`
	SerializeBinary        []byte  `json:"-"`
}

type kcesPresetEditBaseDataWire struct {
	_struct                struct{} `codec:",toarray"`
	*IndexedObjectMetadata `codec:"-"`
	Version                int                            `json:"-"`
	ColorPreset            *kcesPresetEditColorPresetWire `json:"-"`
	Flags                  map[string]string              `json:"-"`
}

type kcesPresetEditUnitDataWire struct {
	_struct                struct{} `codec:",toarray"`
	*IndexedObjectMetadata `codec:"-"`
	Version                int     `json:"-"`
	PositionX              float32 `json:"-"`
	PositionY              float32 `json:"-"`
	WarpointName           *string `json:"-"`
}

func (v kcesPresetEditColorPresetWire) CodecEncodeSelf(e *codec.Encoder) {
	ct.EncodeIndexedObjectSelf(e, &v)
}

func (v *kcesPresetEditColorPresetWire) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}

func (v kcesPresetEditBaseDataWire) CodecEncodeSelf(e *codec.Encoder) {
	ct.EncodeIndexedObjectSelf(e, &v)
}

func (v *kcesPresetEditBaseDataWire) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}

func (v kcesPresetEditUnitDataWire) CodecEncodeSelf(e *codec.Encoder) {
	ct.EncodeIndexedObjectSelf(e, &v)
}

func (v *kcesPresetEditUnitDataWire) CodecDecodeSelf(d *codec.Decoder) {
	ct.DecodeIndexedObjectSelf(d, v)
}

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

func NewKCESPresetEditBaseData() *KCESPresetEditBaseData {
	return &KCESPresetEditBaseData{
		Version:     KCESPresetEditBaseDataVersion,
		ColorPreset: &KCESPresetEditColorPreset{},
		Flags:       map[string]string{},
	}
}

func NewKCESPresetEditUnitData() *KCESPresetEditUnitData {
	return &KCESPresetEditUnitData{Version: KCESPresetEditUnitDataVersion}
}

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

func splitKCESPresetEditRoot(data []byte, name string) (root, trailing []byte, rootNil bool, err error) {
	root, trailing, err = ct.SplitFirstMsgpackValue(data)
	if err != nil {
		return nil, nil, false, fmt.Errorf("decode %s root MessagePack: %w", name, err)
	}
	return root, trailing, len(root) == 1 && root[0] == 0xc0, nil
}

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

func kcesPresetEditMetadataHasPayload(metadata *IndexedObjectMetadata) bool {
	return metadata != nil && indexedObjectMetadataHasPayload(*metadata)
}
