package KCES

import (
	"bytes"
	"fmt"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

// .preset
// KCES 角色预设文件，外层是版本 1000 的 VirtualDirectory，包含 thumbnail、maiddata 和 meta 虚拟文件。
// maiddata 内部保存属性、颜色和身体数据；该格式与同扩展名的旧版 CM3D2_PRESET wire 不兼容。
//
// .preset
// KCES character-preset file whose outer version-1000 VirtualDirectory contains thumbnail, maiddata, and meta virtual files.
// maiddata stores property, color, and body data; this wire format is incompatible with legacy CM3D2_PRESET despite sharing the extension.

const (
	KCESPresetExtension = ".preset"

	// KCESPresetFormat distinguishes the VirtualDirectory-based KCES preset
	// JSON representation from the legacy CM3D2_PRESET representation, which
	// uses the same .preset extension.
	KCESPresetFormat = "kces-virtual-directory-preset"

	kcesPresetVersion = 1000
)

const (
	kcesPresetThumbnailFile = "thumbnail"
	kcesPresetMaidDataFile  = "maiddata"
	kcesPresetMetaFile      = "meta"
)

// KCESPreset represents the VirtualDirectory-based preset format used by
// current KCES releases. The three inner byte arrays are the data produced by
// Maid.SerializeProp, Maid.SerializeMultiColor, and Maid.SerializeBody. They
// remain losslessly editable as base64 in JSON. The exported inner helpers can
// inspect or rebuild them without applying the game's version migrations.
type KCESPreset struct {
	Format                  string                                 `json:"format"`
	ContainerVersion        int32                                  `json:"containerVersion"`
	ContainerVersionless    bool                                   `json:"containerVersionless,omitempty"`
	ContainerFilesOnly      bool                                   `json:"containerFilesOnly,omitempty"`
	ContainerDirectoriesNil bool                                   `json:"containerDirectoriesNil,omitempty"`
	ContainerFilesNil       bool                                   `json:"containerFilesNil,omitempty"`
	ContainerFieldCount     *int32                                 `json:"containerFieldCount,omitempty"`
	ContainerFutureSlots    [][]byte                               `json:"containerFutureSlots,omitempty"`
	ContainerDirectories    map[string]ct.VirtualDirectoryMetadata `json:"containerDirectories,omitempty"`
	ContainerVirtualFiles   map[string]ct.VirtualFileMetadata      `json:"containerVirtualFiles,omitempty"`
	Thumbnail               []byte                                 `json:"thumbnail"`
	MaidData                *KCESPresetCore                        `json:"maidData"`
	Meta                    *KCESPresetMeta                        `json:"meta,omitempty"`
	ExtraFiles              map[string][]byte                      `json:"extraFiles,omitempty"`
}

// KCESPresetCore matches MaidPreset.MaidPresetCore's indexed MessagePack
// array: [version, propData, colorData, bodyData].
type KCESPresetCore struct {
	_struct                struct{} `codec:",toarray"`
	*IndexedObjectMetadata `codec:"-"`
	Version                int32  `json:"version"`
	PropData               []byte `json:"propData"`
	ColorData              []byte `json:"colorData"`
	BodyData               []byte `json:"bodyData"`
	RootNil                bool   `codec:"-" json:"rootNil,omitempty"`
	TrailingData           []byte `codec:"-" json:"trailingData,omitempty"`
}

func (c *KCESPresetCore) getMessagePackTrailing() []byte     { return c.TrailingData }
func (c *KCESPresetCore) setMessagePackTrailing(data []byte) { c.TrailingData = data }
func (c *KCESPresetCore) getMessagePackRootNil() bool        { return c.RootNil }
func (c *KCESPresetCore) setMessagePackRootNil(value bool)   { c.RootNil = value }

// KCESPresetMeta matches MaidPreset.MaidPresetMeta's indexed MessagePack
// array: [version, metaData].
type KCESPresetMeta struct {
	_struct                struct{} `codec:",toarray"`
	*IndexedObjectMetadata `codec:"-"`
	Version                int32             `json:"version"`
	Data                   map[string]string `json:"metaData"`
	RootNil                bool              `codec:"-" json:"rootNil,omitempty"`
	TrailingData           []byte            `codec:"-" json:"trailingData,omitempty"`
}

func (m *KCESPresetMeta) getMessagePackTrailing() []byte     { return m.TrailingData }
func (m *KCESPresetMeta) setMessagePackTrailing(data []byte) { m.TrailingData = data }
func (m *KCESPresetMeta) getMessagePackRootNil() bool        { return m.RootNil }
func (m *KCESPresetMeta) setMessagePackRootNil(value bool)   { m.RootNil = value }

// IsKCESPresetData reports whether data starts with the VirtualDirectory
// signature used by current KCES presets. The serialize-type byte is not part
// of detection so an unsupported MemoryPack preset is still routed to the
// KCES parser and receives an accurate error instead of being mistaken for a
// legacy COM3D2 preset.
func IsKCESPresetData(data []byte) bool {
	return len(data) >= len(ct.FileSignature) && bytes.Equal(data[:len(ct.FileSignature)], ct.FileSignature)
}

// DecodeKCESPreset decodes a current KCES VirtualDirectory preset.
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

// EncodeKCESPreset encodes a KCES VirtualDirectory preset without invoking the
// game's OnBeforeSerialize callbacks. All stored version integers are emitted
// exactly as supplied; callers creating a new current preset should use the
// constructors below instead of relying on implicit version upgrades.
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

func validateKCESPresetCore(core *KCESPresetCore) error {
	if core == nil {
		return fmt.Errorf("maidData is nil")
	}
	// PropData, ColorData, and BodyData are independent byte[] payloads at this
	// layer. MaidPreset.Load later hands them to game-specific deserializers;
	// rejecting or rebuilding an old/future/malformed inner block here would
	// prevent faithful outer serialization and would incorrectly make this
	// container codec perform the game's version handling.
	return nil
}

// NewKCESPresetCore creates the three explicit current wire blocks used by a
// newly-created KCES 1.34.4 preset. It does not alter cores decoded from older
// versions.
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

// NewKCESPreset creates an empty current container around a valid current
// MaidPresetCore. Thumbnail data may be nil, matching the game's optional
// createThumbnail path.
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

func cloneStringMap(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
