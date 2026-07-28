package KCES

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/msgpack"
	"github.com/ugorji/go/codec"
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

// KCESPreset 表示仅供二进制封装层使用的 KCES VirtualDirectory 预设
// 面向 editing JSON 的服务使用 ExpandedKCESPreset，不公开三个已知内部块的原始字节
// KCESPreset represents the KCES VirtualDirectory preset used only by the binary envelope layer
// Editing JSON services use ExpandedKCESPreset and do not expose raw bytes for the three known inner blocks
type KCESPreset struct {
	Format               string                                 `json:"format"`                         // JSON 表示格式标识 / JSON representation format identifier
	ContainerVersion     int32                                  `json:"containerVersion"`               // VirtualDirectory 容器版本 / VirtualDirectory container version
	ContainerFraming     ct.VirtualDirectoryFraming             `json:"containerFraming,omitempty"`     // VirtualDirectory MessagePack 目录的外层尾部封装 / Outer footer frame around the VirtualDirectory MessagePack directory
	ContainerDirectories map[string]ct.VirtualDirectoryMetadata `json:"containerDirectories,omitempty"` // 虚拟目录的真实版本字段 / Real version fields of virtual directories
	Thumbnail            []byte                                 `json:"thumbnail"`                      // thumbnail 虚拟文件字节 / Bytes of the thumbnail virtual file
	MaidData             *KCESPresetCore                        `json:"maidData"`                       // maiddata 虚拟文件中的内部线记录 / Inner wire record from the maiddata virtual file
	Meta                 *KCESPresetMeta                        `json:"meta,omitempty"`                 // 可选 meta 虚拟文件记录 / Optional record from the meta virtual file
	ExtraFiles           map[string][]byte                      `json:"extraFiles,omitempty"`           // 除三个已知名称外的虚拟文件 / Virtual files other than the three known names
}

// KCESPresetCore 对应 MaidPreset.MaidPresetCore 的 indexed MessagePack 数组，依次保存版本、propData、colorData 和 bodyData
// KCESPresetCore matches the indexed MessagePack array of MaidPreset.MaidPresetCore, storing version, propData, colorData, and bodyData in order
type KCESPresetCore struct {
	_struct   struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	Version   int32    `json:"version"`   // MaidPresetCore 对象版本 / MaidPresetCore object version
	PropData  []byte   `json:"-"`         // Maid.SerializeProp 生成的内部线块 / Inner wire block produced by Maid.SerializeProp
	ColorData []byte   `json:"-"`         // Maid.SerializeMultiColor 生成的内部线块 / Inner wire block produced by Maid.SerializeMultiColor
	BodyData  []byte   `json:"-"`         // Maid.SerializeBody 生成的内部线块 / Inner wire block produced by Maid.SerializeBody
}

// KCESPresetMeta 对应 MaidPreset.MaidPresetMeta 的 indexed MessagePack 数组，依次保存版本和 metaData 字符串字典
// KCESPresetMeta matches the indexed MessagePack array of MaidPreset.MaidPresetMeta, storing version followed by the metaData string map
type KCESPresetMeta struct {
	_struct struct{}           `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	Version int32              `json:"version"`   // MaidPresetMeta 对象版本 / MaidPresetMeta object version
	Data    map[string]*string `json:"metaData"`  // 预设元数据字符串字典，游戏明确读取 presetName / Preset metadata string map, with presetName explicitly read by the game
}

// MarshalJSON 将底层预设封套转换为完整类型化编辑表示，禁止公开三个已知内部块的 Base64 字节
// MarshalJSON converts the binary preset envelope to the fully typed editing representation and forbids exposing the three known inner blocks as Base64 bytes
func (preset KCESPreset) MarshalJSON() ([]byte, error) {
	expanded, err := ExpandKCESPreset(&preset)
	if err != nil {
		return nil, err
	}
	return json.Marshal(expanded)
}

// UnmarshalJSON 严格读取完整类型化编辑表示并重建底层预设封套
// UnmarshalJSON strictly reads the fully typed editing representation and rebuilds the binary preset envelope
func (preset *KCESPreset) UnmarshalJSON(data []byte) error {
	if preset == nil {
		return fmt.Errorf("nil KCES preset JSON target")
	}
	var expanded ExpandedKCESPreset
	if err := decodeKCESJSONStrict(data, &expanded); err != nil {
		return err
	}
	collapsed, err := CollapseExpandedKCESPreset(&expanded)
	if err != nil {
		return err
	}
	*preset = *collapsed
	return nil
}

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
	var core *KCESPresetCore
	if err := decodeCompressedMsgpack(coreRaw, &core, "KCES preset maiddata"); err != nil {
		return nil, fmt.Errorf("decode KCES preset %q: %w", kcesPresetMaidDataFile, err)
	}
	if core == nil {
		return nil, fmt.Errorf("decode KCES preset %q: root must not be null", kcesPresetMaidDataFile)
	}
	if err := validateKCESPresetCore(core); err != nil {
		return nil, fmt.Errorf("decode KCES preset %q: %w", kcesPresetMaidDataFile, err)
	}

	preset := &KCESPreset{
		Format:               KCESPresetFormat,
		ContainerVersion:     table.Version,
		ContainerFraming:     table.Framing,
		ContainerDirectories: table.GetVirtualDirectoryMetadata(),
		Thumbnail:            append([]byte(nil), thumbnail...),
		MaidData:             core,
	}

	if _, ok := table.Files[kcesPresetMetaFile]; ok {
		metaRaw, err := table.GetFileData(kcesPresetMetaFile)
		if err != nil {
			return nil, fmt.Errorf("decode KCES preset %q: %w", kcesPresetMetaFile, err)
		}
		var meta *KCESPresetMeta
		if err := decodeCompressedMsgpack(metaRaw, &meta, "KCES preset meta"); err != nil {
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
		Version:     preset.ContainerVersion,
		Framing:     preset.ContainerFraming,
		Directories: preset.ContainerDirectories,
		Raw:         make([]byte, ct.HeaderSize),
		Files:       make(map[string]ct.VirtualFile),
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
	var out bytes.Buffer
	if err := ct.WriteContentTable(&out, table); err != nil {
		return nil, fmt.Errorf("encode KCES preset VirtualDirectory: %w", err)
	}
	return out.Bytes(), nil
}

// validateKCESPresetCore 完整验证三个已知内部块
// validateKCESPresetCore fully validates all three known inner blocks
func validateKCESPresetCore(core *KCESPresetCore) error {
	if core == nil {
		return fmt.Errorf("maidData is nil")
	}
	if core.PropData != nil {
		if _, err := DecodeKCESPresetPropertyData(core.PropData); err != nil {
			return fmt.Errorf("propData: %w", err)
		}
	}
	if core.ColorData != nil {
		if _, err := DecodeKCESPresetColorData(core.ColorData); err != nil {
			return fmt.Errorf("colorData: %w", err)
		}
	}
	if core.BodyData != nil {
		if _, err := DecodeKCESPresetBodyData(core.BodyData); err != nil {
			return fmt.Errorf("bodyData: %w", err)
		}
	}
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
			Data:    map[string]*string{},
		},
	}, nil
}

// cloneStringMap 复制字符串映射以避免编码过程共享调用方可变状态
// cloneStringMap copies a string map so encoding does not share caller-owned mutable state
func cloneStringMap(src map[string]*string) map[string]*string {
	dst := make(map[string]*string, len(src))
	for key, value := range src {
		if value == nil {
			dst[key] = nil
			continue
		}
		copyValue := *value
		dst[key] = &copyValue
	}
	return dst
}

// CodecEncodeSelf 按共享 indexed-object 规则编码 KCESPresetCore
// CodecEncodeSelf encodes KCESPresetCore using the shared indexed-object rules
func (v KCESPresetCore) CodecEncodeSelf(e *codec.Encoder) { msgpack.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 KCESPresetCore
// CodecDecodeSelf decodes KCESPresetCore using the shared indexed-object rules
func (v *KCESPresetCore) CodecDecodeSelf(d *codec.Decoder) { msgpack.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 KCESPresetMeta
// CodecEncodeSelf encodes KCESPresetMeta using the shared indexed-object rules
func (v KCESPresetMeta) CodecEncodeSelf(e *codec.Encoder) { msgpack.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 KCESPresetMeta
// CodecDecodeSelf decodes KCESPresetMeta using the shared indexed-object rules
func (v *KCESPresetMeta) CodecDecodeSelf(d *codec.Decoder) { msgpack.DecodeIndexedObjectSelf(d, v) }
