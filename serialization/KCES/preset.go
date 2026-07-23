package KCES

import (
	"bytes"
	"fmt"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

// .preset
// KCES 角色预设文件，外层是版本 1000 的 VirtualDirectory，包含 thumbnail、maiddata 和 meta 虚拟文件
// maiddata 内部保存属性、颜色和身体数据，该格式与同扩展名的旧版 CM3D2_PRESET 线格式不兼容
// .preset
// KCES character-preset file whose outer version-1000 VirtualDirectory contains thumbnail, maiddata, and meta virtual files
// maiddata stores property, color, and body data, and this wire format is incompatible with legacy CM3D2_PRESET despite sharing the extension

const (
	KCESPresetExtension = ".preset"

	// KCESPresetFormat 用于区分基于 VirtualDirectory 的 KCES 预设 JSON 表示与使用同一 .preset 扩展名的旧版 CM3D2_PRESET 表示
	// KCESPresetFormat distinguishes the VirtualDirectory-based KCES preset JSON representation from the legacy CM3D2_PRESET representation using the same .preset extension
	KCESPresetFormat = "kces-virtual-directory-preset"

	kcesPresetVersion = 1000
)

const (
	kcesPresetThumbnailFile = "thumbnail"
	kcesPresetMaidDataFile  = "maiddata"
	kcesPresetMetaFile      = "meta"
)

// KCESPreset 表示当前 KCES 使用的 VirtualDirectory 预设格式
// 三个内部字节数组分别由 Maid.SerializeProp、Maid.SerializeMultiColor 和 Maid.SerializeBody 生成，在 JSON 中以 base64 无损编辑，内部辅助函数可在不执行游戏版本迁移的情况下检查或重建它们
// KCESPreset represents the VirtualDirectory-based preset format used by current KCES releases
// Its three inner byte arrays are produced by Maid.SerializeProp, Maid.SerializeMultiColor, and Maid.SerializeBody and remain losslessly editable as base64 in JSON, while inner helpers can inspect or rebuild them without applying game version migrations
type KCESPreset struct {
	Format                  string                                 `json:"format"`                            // JSON 表示格式标识 / JSON representation format identifier
	ContainerVersion        int32                                  `json:"containerVersion"`                  // VirtualDirectory 容器版本 / VirtualDirectory container version
	ContainerVersionless    bool                                   `json:"containerVersionless,omitempty"`    // 容器是否使用无版本布局 / Whether the container uses the versionless layout
	ContainerFilesOnly      bool                                   `json:"containerFilesOnly,omitempty"`      // 容器是否使用仅文件根布局 / Whether the container uses the files-only root layout
	ContainerDirectoriesNil bool                                   `json:"containerDirectoriesNil,omitempty"` // 目录映射在线格式中是否为 nil / Whether the directory map was nil on the wire
	ContainerFilesNil       bool                                   `json:"containerFilesNil,omitempty"`       // 文件映射在线格式中是否为 nil / Whether the file map was nil on the wire
	ContainerFieldCount     *int32                                 `json:"containerFieldCount,omitempty"`     // 保存的容器 indexed-array 宽度 / Preserved container indexed-array width
	ContainerFutureSlots    [][]byte                               `json:"containerFutureSlots,omitempty"`    // 当前模型之外的原始未来槽位 / Raw future slots beyond the current model
	ContainerDirectories    map[string]ct.VirtualDirectoryMetadata `json:"containerDirectories,omitempty"`    // 虚拟目录线格式元数据 / Virtual-directory wire metadata
	ContainerVirtualFiles   map[string]ct.VirtualFileMetadata      `json:"containerVirtualFiles,omitempty"`   // 虚拟文件线格式元数据 / Virtual-file wire metadata
	Thumbnail               []byte                                 `json:"thumbnail"`                         // thumbnail 虚拟文件字节 / Bytes of the thumbnail virtual file
	MaidData                *KCESPresetCore                        `json:"maidData"`                          // maiddata 虚拟文件中的核心记录 / Core record from the maiddata virtual file
	Meta                    *KCESPresetMeta                        `json:"meta,omitempty"`                    // 可选 meta 虚拟文件记录 / Optional record from the meta virtual file
	ExtraFiles              map[string][]byte                      `json:"extraFiles,omitempty"`              // 除三个已知名称外的虚拟文件 / Virtual files other than the three known names
}

// KCESPresetCore 对应 MaidPreset.MaidPresetCore 的 indexed MessagePack 数组，依次保存版本、propData、colorData 和 bodyData
// KCESPresetCore matches the indexed MessagePack array of MaidPreset.MaidPresetCore, storing version, propData, colorData, and bodyData in order
type KCESPresetCore struct {
	_struct                struct{}    `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"` // 索引对象的线格式元数据 / Indexed-object wire metadata
	Version                int32       `json:"version"`                          // MaidPresetCore 对象版本 / MaidPresetCore object version
	PropData               []byte      `json:"propData"`                         // Maid.SerializeProp 生成的属性块 / Property block produced by Maid.SerializeProp
	ColorData              []byte      `json:"colorData"`                        // Maid.SerializeMultiColor 生成的颜色块 / Color block produced by Maid.SerializeMultiColor
	BodyData               []byte      `json:"bodyData"`                         // Maid.SerializeBody 生成的身体块 / Body block produced by Maid.SerializeBody
	RootNil                bool        `codec:"-" json:"rootNil,omitempty"`      // 根 MessagePack 值是否为 nil / Whether the root MessagePack value was nil
	TrailingData           []byte      `codec:"-" json:"trailingData,omitempty"` // 根值之后游戏未读取的字节 / Bytes left unread by the game after the root value
}

// getMessagePackTrailing 返回核心根值后的保留字节
// getMessagePackTrailing returns preserved bytes after the core root value
func (c *KCESPresetCore) getMessagePackTrailing() []byte { return c.TrailingData }

// setMessagePackTrailing 设置核心根值后的保留字节
// setMessagePackTrailing sets preserved bytes after the core root value
func (c *KCESPresetCore) setMessagePackTrailing(data []byte) { c.TrailingData = data }

// getMessagePackRootNil 返回核心根值是否为 nil
// getMessagePackRootNil reports whether the core root value was nil
func (c *KCESPresetCore) getMessagePackRootNil() bool { return c.RootNil }

// setMessagePackRootNil 设置核心根值的 nil 标记
// setMessagePackRootNil sets the nil marker for the core root value
func (c *KCESPresetCore) setMessagePackRootNil(value bool) { c.RootNil = value }

// KCESPresetMeta 对应 MaidPreset.MaidPresetMeta 的 indexed MessagePack 数组，依次保存版本和 metaData 字符串字典
// KCESPresetMeta matches the indexed MessagePack array of MaidPreset.MaidPresetMeta, storing version followed by the metaData string map
type KCESPresetMeta struct {
	_struct                struct{}          `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`       // 索引对象的线格式元数据 / Indexed-object wire metadata
	Version                int32             `json:"version"`                          // MaidPresetMeta 对象版本 / MaidPresetMeta object version
	Data                   map[string]string `json:"metaData"`                         // 预设元数据字符串字典，游戏明确读取 presetName / Preset metadata string map, with presetName explicitly read by the game
	RootNil                bool              `codec:"-" json:"rootNil,omitempty"`      // 根 MessagePack 值是否为 nil / Whether the root MessagePack value was nil
	TrailingData           []byte            `codec:"-" json:"trailingData,omitempty"` // 根值之后游戏未读取的字节 / Bytes left unread by the game after the root value
}

// getMessagePackTrailing 返回元数据根值后的保留字节
// getMessagePackTrailing returns preserved bytes after the metadata root value
func (m *KCESPresetMeta) getMessagePackTrailing() []byte { return m.TrailingData }

// setMessagePackTrailing 设置元数据根值后的保留字节
// setMessagePackTrailing sets preserved bytes after the metadata root value
func (m *KCESPresetMeta) setMessagePackTrailing(data []byte) { m.TrailingData = data }

// getMessagePackRootNil 返回元数据根值是否为 nil
// getMessagePackRootNil reports whether the metadata root value was nil
func (m *KCESPresetMeta) getMessagePackRootNil() bool { return m.RootNil }

// setMessagePackRootNil 设置元数据根值的 nil 标记
// setMessagePackRootNil sets the nil marker for the metadata root value
func (m *KCESPresetMeta) setMessagePackRootNil(value bool) { m.RootNil = value }

// IsKCESPresetData 判断数据是否以当前 KCES 预设使用的 VirtualDirectory 签名开头
// 探测不包含序列化类型字节，因此不支持的 MemoryPack 预设仍会交给 KCES 解析器得到准确错误，而不会被误判为旧版 COM3D2 预设
// IsKCESPresetData reports whether data begins with the VirtualDirectory signature used by current KCES presets
// Detection excludes the serialize-type byte so an unsupported MemoryPack preset is still routed to the KCES parser for an accurate error instead of being mistaken for a legacy COM3D2 preset
func IsKCESPresetData(data []byte) bool {
	return len(data) >= len(ct.FileSignature) && bytes.Equal(data[:len(ct.FileSignature)], ct.FileSignature)
}

// DecodeKCESPreset 解码当前 KCES VirtualDirectory 预设并保留容器元数据与额外文件
// DecodeKCESPreset decodes a current KCES VirtualDirectory preset and preserves container metadata and extra files
func DecodeKCESPreset(data []byte) (*KCESPreset, error) {
	if !IsKCESPresetData(data) {
		return nil, fmt.Errorf("not a KCES VirtualDirectory preset")
	}

	table, err := ct.ReadContentTable(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode KCES preset VirtualDirectory: %w", err)
	}

	thumbnail, err := table.GetFileData(kcesPresetThumbnailFile)
	if err != nil {
		return nil, fmt.Errorf("decode KCES preset: required virtual file %q: %w", kcesPresetThumbnailFile, err)
	}

	coreRaw, err := table.GetFileData(kcesPresetMaidDataFile)
	if err != nil {
		return nil, fmt.Errorf("decode KCES preset: required virtual file %q: %w", kcesPresetMaidDataFile, err)
	}
	core := &KCESPresetCore{}
	if err := decodeCompressedMsgpack(coreRaw, core, "KCES preset maiddata"); err != nil {
		return nil, fmt.Errorf("decode KCES preset %q: %w", kcesPresetMaidDataFile, err)
	}
	if err := validateKCESPresetCore(core); err != nil {
		return nil, fmt.Errorf("decode KCES preset %q: %w", kcesPresetMaidDataFile, err)
	}

	preset := &KCESPreset{
		Format:                  KCESPresetFormat,
		ContainerVersion:        table.Version,
		ContainerVersionless:    table.Versionless,
		ContainerFilesOnly:      table.FilesOnly,
		ContainerDirectoriesNil: table.DirectoriesNil,
		ContainerFilesNil:       table.FilesNil,
		ContainerFieldCount:     table.FieldCount,
		ContainerFutureSlots:    table.FutureSlots,
		ContainerDirectories:    table.GetVirtualDirectoryMetadata(),
		ContainerVirtualFiles:   table.GetVirtualFileMetadata(),
		Thumbnail:               append([]byte(nil), thumbnail...),
		MaidData:                core,
	}

	if _, ok := table.Files[kcesPresetMetaFile]; ok {
		metaRaw, err := table.GetFileData(kcesPresetMetaFile)
		if err != nil {
			return nil, fmt.Errorf("decode KCES preset %q: %w", kcesPresetMetaFile, err)
		}
		meta := &KCESPresetMeta{}
		if err := decodeCompressedMsgpack(metaRaw, meta, "KCES preset meta"); err != nil {
			return nil, fmt.Errorf("decode KCES preset %q: %w", kcesPresetMetaFile, err)
		}
		preset.Meta = meta
	}

	for _, name := range table.GetFileNames() {
		switch name {
		case kcesPresetThumbnailFile, kcesPresetMaidDataFile, kcesPresetMetaFile:
			continue
		}
		fileData, err := table.GetFileData(name)
		if err != nil {
			return nil, fmt.Errorf("decode KCES preset extra virtual file %q: %w", name, err)
		}
		if preset.ExtraFiles == nil {
			preset.ExtraFiles = make(map[string][]byte)
		}
		preset.ExtraFiles[name] = append([]byte(nil), fileData...)
	}

	return preset, nil
}

// EncodeKCESPreset 在不调用游戏 OnBeforeSerialize 回调的情况下编码 KCES VirtualDirectory 预设
// 所有保存的版本整数均按提供值写出，新建当前预设的调用方应使用下方构造函数而不能依赖隐式版本升级
// EncodeKCESPreset encodes a KCES VirtualDirectory preset without invoking game OnBeforeSerialize callbacks
// All stored version integers are emitted exactly as supplied, so callers creating a new current preset should use the constructors below instead of relying on implicit version upgrades
func EncodeKCESPreset(preset *KCESPreset) ([]byte, error) {
	if preset == nil {
		return nil, fmt.Errorf("nil KCES preset")
	}
	if preset.Format != "" && preset.Format != KCESPresetFormat {
		return nil, fmt.Errorf("unsupported KCES preset format %q", preset.Format)
	}
	if preset.MaidData == nil {
		return nil, fmt.Errorf("KCES preset maidData is required")
	}
	if err := validateKCESPresetCore(preset.MaidData); err != nil {
		return nil, fmt.Errorf("KCES preset maidData: %w", err)
	}

	core := *preset.MaidData
	maidData, err := encodeCompressedMsgpack(&core, "KCES preset maiddata")
	if err != nil {
		return nil, err
	}

	table := &ct.ContentTable{
		Version:        preset.ContainerVersion,
		Versionless:    preset.ContainerVersionless,
		FilesOnly:      preset.ContainerFilesOnly,
		DirectoriesNil: preset.ContainerDirectoriesNil,
		FilesNil:       preset.ContainerFilesNil,
		FieldCount:     preset.ContainerFieldCount,
		FutureSlots:    preset.ContainerFutureSlots,
		Directories:    preset.ContainerDirectories,
		Raw:            make([]byte, ct.HeaderSize),
		Files:          make(map[string]ct.VirtualFile),
	}
	if err := table.AddFile(kcesPresetThumbnailFile, append([]byte(nil), preset.Thumbnail...)); err != nil {
		return nil, err
	}
	if err := table.AddFile(kcesPresetMaidDataFile, maidData); err != nil {
		return nil, err
	}

	if preset.Meta != nil {
		meta := *preset.Meta
		if meta.Data != nil {
			meta.Data = cloneStringMap(meta.Data)
		}
		metaData, err := encodeCompressedMsgpack(&meta, "KCES preset meta")
		if err != nil {
			return nil, err
		}
		if err := table.AddFile(kcesPresetMetaFile, metaData); err != nil {
			return nil, err
		}
	}

	for name, fileData := range preset.ExtraFiles {
		switch name {
		case kcesPresetThumbnailFile, kcesPresetMaidDataFile, kcesPresetMetaFile:
			return nil, fmt.Errorf("KCES preset extraFiles contains reserved virtual file %q", name)
		}
		if err := table.AddFile(name, append([]byte(nil), fileData...)); err != nil {
			return nil, err
		}
	}
	if err := table.ApplyVirtualFileMetadata(preset.ContainerVirtualFiles); err != nil {
		return nil, fmt.Errorf("KCES preset: %w", err)
	}

	var out bytes.Buffer
	if err := ct.WriteContentTable(&out, table); err != nil {
		return nil, fmt.Errorf("encode KCES preset VirtualDirectory: %w", err)
	}
	return out.Bytes(), nil
}

// validateKCESPresetCore 验证核心对象版本而保持三个内部 byte[] 载荷不透明
// validateKCESPresetCore validates the core object version while keeping all three inner byte[] payloads opaque
func validateKCESPresetCore(core *KCESPresetCore) error {
	if core == nil {
		return fmt.Errorf("maidData is nil")
	}
	// PropData、ColorData 和 BodyData 在此层是独立 byte[] 载荷，MaidPreset.Load 稍后将其交给游戏专用反序列化器，在这里拒绝或重建旧版、未来版或格式错误的内部块会阻止外层无损序列化，并错误地让容器编解码器承担游戏版本处理
	// PropData, ColorData, and BodyData are independent byte[] payloads at this layer and MaidPreset.Load later passes them to game-specific deserializers, so rejecting or rebuilding an old, future, or malformed inner block here would prevent faithful outer serialization and incorrectly make the container codec perform game version handling
	return nil
}

// NewKCESPresetCore 创建新 KCES 1.34.4 预设使用的三个明确当前线格式块，不会更改从旧版本解码的核心
// NewKCESPresetCore creates the three explicit current wire blocks used by a newly created KCES 1.34.4 preset without altering cores decoded from older versions
func NewKCESPresetCore() (*KCESPresetCore, error) {
	propertyData, err := EncodeKCESPresetPropertyData(NewKCESPresetPropertyList())
	if err != nil {
		return nil, fmt.Errorf("create KCES preset propData: %w", err)
	}
	colorData, err := EncodeKCESPresetColorData(NewKCESPresetColorData())
	if err != nil {
		return nil, fmt.Errorf("create KCES preset colorData: %w", err)
	}
	bodyData, err := EncodeKCESPresetBodyData(NewKCESPresetBodyData())
	if err != nil {
		return nil, fmt.Errorf("create KCES preset bodyData: %w", err)
	}
	return &KCESPresetCore{
		Version:   kcesPresetVersion,
		PropData:  propertyData,
		ColorData: colorData,
		BodyData:  bodyData,
	}, nil
}

// NewKCESPreset 在有效的当前 MaidPresetCore 外创建空的当前容器，缩略图可为 nil，与游戏可选 createThumbnail 路径一致
// NewKCESPreset creates an empty current container around a valid current MaidPresetCore, allowing nil thumbnail data to match the game's optional createThumbnail path
func NewKCESPreset() (*KCESPreset, error) {
	core, err := NewKCESPresetCore()
	if err != nil {
		return nil, err
	}
	return &KCESPreset{
		Format:           KCESPresetFormat,
		ContainerVersion: kcesPresetVersion,
		MaidData:         core,
		Meta: &KCESPresetMeta{
			Version: kcesPresetVersion,
			Data:    map[string]string{},
		},
	}, nil
}

// cloneStringMap 复制字符串映射以避免编码过程共享调用方可变状态
// cloneStringMap copies a string map so encoding does not share caller-owned mutable state
func cloneStringMap(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
