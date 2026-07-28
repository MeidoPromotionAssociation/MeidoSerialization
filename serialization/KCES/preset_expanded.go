package KCES

import (
	"encoding/json"
	"fmt"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/internal/strictjson"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

// .preset 的完整编辑视图，保留真实 VirtualDirectory 字段并展开 maiddata 内部的属性、颜色和身体块
// Fully expanded editing view of .preset retaining real VirtualDirectory fields and decoding maiddata property, color, and body blocks

// ExpandedKCESPreset 是 KCES VirtualDirectory 预设的完整解码编辑表示，三个 MaidPresetCore 字节串只以实际游戏结构表示 / ExpandedKCESPreset is the fully decoded editing representation of a KCES VirtualDirectory preset whose three MaidPresetCore byte strings are represented only by actual game structures
type ExpandedKCESPreset struct {
	Format               string                                 `json:"format"`                         // JSON 表示格式标识 / JSON representation format identifier
	ContainerVersion     int32                                  `json:"containerVersion"`               // VirtualDirectory 容器版本 / VirtualDirectory container version
	ContainerFraming     ct.VirtualDirectoryFraming             `json:"containerFraming,omitempty"`     // VirtualDirectory MessagePack 目录的外层尾部封装 / Outer footer frame around the VirtualDirectory MessagePack directory
	ContainerDirectories map[string]ct.VirtualDirectoryMetadata `json:"containerDirectories,omitempty"` // 虚拟目录的真实版本字段 / Real version fields of virtual directories
	Thumbnail            []byte                                 `json:"thumbnail"`                      // thumbnail 虚拟文件字节 / Bytes of the thumbnail virtual file
	MaidData             ExpandedKCESPresetCore                 `json:"maidData"`                       // 展开三个内部块的必需 maiddata 核心 / Required maiddata core with all three inner blocks expanded
	Meta                 *KCESPresetMeta                        `json:"meta,omitempty"`                 // 可选 meta 虚拟文件记录 / Optional record from the meta virtual file
	ExtraFiles           map[string][]byte                      `json:"extraFiles,omitempty"`           // 除三个已知名称外的虚拟文件 / Virtual files other than the three known names
}

// ExpandedKCESPresetCore 是将 propData、colorData 和 bodyData 解码为各自 BinaryWriter 结构的 MaidPresetCore / ExpandedKCESPresetCore is MaidPresetCore with propData, colorData, and bodyData decoded into their respective BinaryWriter structures
type ExpandedKCESPresetCore struct {
	Version   int32                   `json:"version"`   // MaidPresetCore 对象版本 / MaidPresetCore object version
	PropData  *KCESPresetPropertyList `json:"propData"`  // 展开的 Maid.SerializeProp 块 / Expanded Maid.SerializeProp block
	ColorData *KCESPresetColorData    `json:"colorData"` // 展开的 Maid.SerializeMultiColor 块 / Expanded Maid.SerializeMultiColor block
	BodyData  *KCESPresetBodyData     `json:"bodyData"`  // 展开的 Maid.SerializeBody 块 / Expanded Maid.SerializeBody block
}

// UnmarshalJSON 严格解码 maiddata 核心并要求版本及三个可空内部块字段显式出现
// UnmarshalJSON strictly decodes the maiddata core and requires the version and all three nullable inner-block fields to be explicitly present
func (value *ExpandedKCESPresetCore) UnmarshalJSON(data []byte) error {
	if value == nil {
		return fmt.Errorf("nil expanded KCES preset core JSON target")
	}
	type plainExpandedKCESPresetCore ExpandedKCESPresetCore
	var decoded plainExpandedKCESPresetCore
	if err := decodeKCESJSONStrict(data, &decoded); err != nil {
		return err
	}
	if err := strictjson.RequireObjectFields(data, "maidData", "version", "propData", "colorData", "bodyData"); err != nil {
		return err
	}
	*value = ExpandedKCESPresetCore(decoded)
	return nil
}

// UnmarshalJSON 严格解码完整预设编辑对象并要求所有非可选封套字段显式存在
// UnmarshalJSON strictly decodes a complete preset editing object and requires every non-optional envelope field to be present
func (value *ExpandedKCESPreset) UnmarshalJSON(data []byte) error {
	if value == nil {
		return fmt.Errorf("nil expanded KCES preset JSON target")
	}
	type expandedKCESPresetJSON ExpandedKCESPreset
	var decoded expandedKCESPresetJSON
	if err := decodeKCESJSONStrict(data, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for _, name := range []string{"format", "containerVersion", "thumbnail", "maidData"} {
		if _, ok := fields[name]; !ok {
			return fmt.Errorf("%s is required", name)
		}
	}
	*value = ExpandedKCESPreset(decoded)
	return nil
}

// DecodeExpandedKCESPreset 解码 VirtualDirectory 和 MessagePack 封套以及三个已知 MaidPresetCore 字节串载荷
// DecodeExpandedKCESPreset decodes the VirtualDirectory and MessagePack envelope together with all three known MaidPresetCore byte-string payloads
func DecodeExpandedKCESPreset(data []byte) (*ExpandedKCESPreset, error) {
	preset, err := DecodeKCESPreset(data)
	if err != nil {
		return nil, err
	}
	return ExpandKCESPreset(preset)
}

// ExpandKCESPreset 将已解码的不透明 KCESPreset 转换为完整强类型编辑表示
// nil 内部字节串保持为 nil，存在但格式错误的块会被拒绝而不是作为无法解释的字节数组公开
// ExpandKCESPreset converts an already decoded opaque KCESPreset into its fully typed editing representation
// Nil inner byte strings remain nil, while a present but malformed block is rejected instead of being exposed as an unexplained byte array
func ExpandKCESPreset(preset *KCESPreset) (*ExpandedKCESPreset, error) {
	if preset == nil {
		return nil, fmt.Errorf("nil KCES preset")
	}

	expanded := &ExpandedKCESPreset{
		Format:               preset.Format,
		ContainerVersion:     preset.ContainerVersion,
		ContainerFraming:     preset.ContainerFraming,
		ContainerDirectories: preset.ContainerDirectories,
		Thumbnail:            preset.Thumbnail,
		Meta:                 preset.Meta,
		ExtraFiles:           preset.ExtraFiles,
	}
	if preset.MaidData == nil {
		return nil, fmt.Errorf("KCES preset maidData is required")
	}

	core := ExpandedKCESPresetCore{Version: preset.MaidData.Version}
	var err error
	if preset.MaidData.PropData != nil {
		core.PropData, err = DecodeKCESPresetPropertyData(preset.MaidData.PropData)
		if err != nil {
			return nil, fmt.Errorf("decode KCES preset maidData.propData: %w", err)
		}
	}
	if preset.MaidData.ColorData != nil {
		core.ColorData, err = DecodeKCESPresetColorData(preset.MaidData.ColorData)
		if err != nil {
			return nil, fmt.Errorf("decode KCES preset maidData.colorData: %w", err)
		}
	}
	if preset.MaidData.BodyData != nil {
		core.BodyData, err = DecodeKCESPresetBodyData(preset.MaidData.BodyData)
		if err != nil {
			return nil, fmt.Errorf("decode KCES preset maidData.bodyData: %w", err)
		}
	}
	expanded.MaidData = core
	return expanded, nil
}

// EncodeExpandedKCESPreset 重建三个内部 BinaryWriter 块后写出普通 KCES 预设封套
// 每个保存版本均来自 value，不调用游戏迁移回调或当前版本构造函数
// EncodeExpandedKCESPreset rebuilds all three inner BinaryWriter blocks and then writes the ordinary KCES preset envelope
// Every stored version comes from value without invoking a game migration callback or current-version constructor
func EncodeExpandedKCESPreset(value *ExpandedKCESPreset) ([]byte, error) {
	preset, err := CollapseExpandedKCESPreset(value)
	if err != nil {
		return nil, err
	}
	return EncodeKCESPreset(preset)
}

// CollapseExpandedKCESPreset 将编辑表示转换回 MaidPresetCore MessagePack 线格式需要的字节串形式
// CollapseExpandedKCESPreset converts the editing representation back to the byte-string form required by the MaidPresetCore MessagePack wire layout
func CollapseExpandedKCESPreset(value *ExpandedKCESPreset) (*KCESPreset, error) {
	if value == nil {
		return nil, fmt.Errorf("nil expanded KCES preset")
	}
	core := &KCESPresetCore{Version: value.MaidData.Version}
	var err error
	if value.MaidData.PropData != nil {
		core.PropData, err = EncodeKCESPresetPropertyData(value.MaidData.PropData)
		if err != nil {
			return nil, fmt.Errorf("encode expanded KCES preset maidData.propData: %w", err)
		}
	}
	if value.MaidData.ColorData != nil {
		core.ColorData, err = EncodeKCESPresetColorData(value.MaidData.ColorData)
		if err != nil {
			return nil, fmt.Errorf("encode expanded KCES preset maidData.colorData: %w", err)
		}
	}
	if value.MaidData.BodyData != nil {
		core.BodyData, err = EncodeKCESPresetBodyData(value.MaidData.BodyData)
		if err != nil {
			return nil, fmt.Errorf("encode expanded KCES preset maidData.bodyData: %w", err)
		}
	}

	return &KCESPreset{
		Format:               value.Format,
		ContainerVersion:     value.ContainerVersion,
		ContainerFraming:     value.ContainerFraming,
		ContainerDirectories: value.ContainerDirectories,
		Thumbnail:            value.Thumbnail,
		MaidData:             core,
		Meta:                 value.Meta,
		ExtraFiles:           value.ExtraFiles,
	}, nil
}
