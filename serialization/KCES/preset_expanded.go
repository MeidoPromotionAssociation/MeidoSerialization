package KCES

import (
	"fmt"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

// .preset 的完整编辑视图，在保留 VirtualDirectory 元数据的同时展开 maiddata 内部的属性、颜色和身体块
// Fully expanded editing view of .preset, retaining VirtualDirectory metadata while decoding maiddata property, color, and body blocks

// ExpandedKCESPreset 是 KCES VirtualDirectory 预设的完整解码编辑表示
// 与 KCESPreset 不同，三个 MaidPresetCore 字节串以实际游戏结构而不是 base64 块表示，容器元数据与未知未来槽位仍被保留，使外层线格式可按原版本写回
// ExpandedKCESPreset is the fully decoded editing representation of a KCES VirtualDirectory preset
// Unlike KCESPreset, its three MaidPresetCore byte strings are represented by actual game structures instead of base64 blobs, while container metadata and unknown future slots remain available so the surrounding wire can be written with original versions
type ExpandedKCESPreset struct {
	Format                  string                                 `json:"format"`                            // JSON 表示格式标识 / JSON representation format identifier
	ContainerVersion        int                                    `json:"containerVersion"`                  // VirtualDirectory 容器版本 / VirtualDirectory container version
	ContainerVersionless    bool                                   `json:"containerVersionless,omitempty"`    // 容器是否使用无版本布局 / Whether the container uses the versionless layout
	ContainerFilesOnly      bool                                   `json:"containerFilesOnly,omitempty"`      // 容器是否使用仅文件根布局 / Whether the container uses the files-only root layout
	ContainerDirectoriesNil bool                                   `json:"containerDirectoriesNil,omitempty"` // 目录映射在线格式中是否为 nil / Whether the directory map was nil on the wire
	ContainerFilesNil       bool                                   `json:"containerFilesNil,omitempty"`       // 文件映射在线格式中是否为 nil / Whether the file map was nil on the wire
	ContainerFieldCount     *int                                   `json:"containerFieldCount,omitempty"`     // 保存的容器 indexed-array 宽度 / Preserved container indexed-array width
	ContainerFutureSlots    [][]byte                               `json:"containerFutureSlots,omitempty"`    // 当前模型之外的原始未来槽位 / Raw future slots beyond the current model
	ContainerDirectories    map[string]ct.VirtualDirectoryMetadata `json:"containerDirectories,omitempty"`    // 虚拟目录线格式元数据 / Virtual-directory wire metadata
	ContainerVirtualFiles   map[string]ct.VirtualFileMetadata      `json:"containerVirtualFiles,omitempty"`   // 虚拟文件线格式元数据 / Virtual-file wire metadata
	Thumbnail               []byte                                 `json:"thumbnail"`                         // thumbnail 虚拟文件字节 / Bytes of the thumbnail virtual file
	MaidData                *ExpandedKCESPresetCore                `json:"maidData"`                          // 展开三个内部块的 maiddata 核心 / Maiddata core with all three inner blocks expanded
	Meta                    *KCESPresetMeta                        `json:"meta,omitempty"`                    // 可选 meta 虚拟文件记录 / Optional record from the meta virtual file
	ExtraFiles              map[string][]byte                      `json:"extraFiles,omitempty"`              // 除三个已知名称外的虚拟文件 / Virtual files other than the three known names
}

// ExpandedKCESPresetCore 是将 propData、colorData 和 bodyData 解码为各自 BinaryWriter 结构的 MaidPresetCore
// ExpandedKCESPresetCore is MaidPresetCore with propData, colorData, and bodyData decoded into their respective BinaryWriter structures
type ExpandedKCESPresetCore struct {
	*IndexedObjectMetadata                         // MaidPresetCore 的线格式元数据 / MaidPresetCore wire metadata
	Version                int                     `json:"version"`                // MaidPresetCore 对象版本 / MaidPresetCore object version
	PropData               *KCESPresetPropertyList `json:"propData"`               // 展开的 Maid.SerializeProp 块 / Expanded Maid.SerializeProp block
	ColorData              *KCESPresetColorData    `json:"colorData"`              // 展开的 Maid.SerializeMultiColor 块 / Expanded Maid.SerializeMultiColor block
	BodyData               *KCESPresetBodyData     `json:"bodyData"`               // 展开的 Maid.SerializeBody 块 / Expanded Maid.SerializeBody block
	RootNil                bool                    `json:"rootNil,omitempty"`      // 原 maiddata 根值是否为 MessagePack nil / Whether the original maiddata root was MessagePack nil
	TrailingData           []byte                  `json:"trailingData,omitempty"` // maiddata 根值之后未读的字节 / Unread bytes after the maiddata root value
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
		Format:                  preset.Format,
		ContainerVersion:        preset.ContainerVersion,
		ContainerVersionless:    preset.ContainerVersionless,
		ContainerFilesOnly:      preset.ContainerFilesOnly,
		ContainerDirectoriesNil: preset.ContainerDirectoriesNil,
		ContainerFilesNil:       preset.ContainerFilesNil,
		ContainerFieldCount:     preset.ContainerFieldCount,
		ContainerFutureSlots:    preset.ContainerFutureSlots,
		ContainerDirectories:    preset.ContainerDirectories,
		ContainerVirtualFiles:   preset.ContainerVirtualFiles,
		Thumbnail:               preset.Thumbnail,
		Meta:                    preset.Meta,
		ExtraFiles:              preset.ExtraFiles,
	}
	if preset.MaidData == nil {
		return nil, fmt.Errorf("KCES preset maidData is required")
	}

	core := &ExpandedKCESPresetCore{
		IndexedObjectMetadata: preset.MaidData.IndexedObjectMetadata,
		Version:               preset.MaidData.Version,
		RootNil:               preset.MaidData.RootNil,
		TrailingData:          preset.MaidData.TrailingData,
	}
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
	if value.MaidData == nil {
		return nil, fmt.Errorf("expanded KCES preset maidData is required")
	}

	core := &KCESPresetCore{
		IndexedObjectMetadata: value.MaidData.IndexedObjectMetadata,
		Version:               value.MaidData.Version,
		RootNil:               value.MaidData.RootNil,
		TrailingData:          value.MaidData.TrailingData,
	}
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
		Format:                  value.Format,
		ContainerVersion:        value.ContainerVersion,
		ContainerVersionless:    value.ContainerVersionless,
		ContainerFilesOnly:      value.ContainerFilesOnly,
		ContainerDirectoriesNil: value.ContainerDirectoriesNil,
		ContainerFilesNil:       value.ContainerFilesNil,
		ContainerFieldCount:     value.ContainerFieldCount,
		ContainerFutureSlots:    value.ContainerFutureSlots,
		ContainerDirectories:    value.ContainerDirectories,
		ContainerVirtualFiles:   value.ContainerVirtualFiles,
		Thumbnail:               value.Thumbnail,
		MaidData:                core,
		Meta:                    value.Meta,
		ExtraFiles:              value.ExtraFiles,
	}, nil
}
