package KCES

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

// system.dat
// KCES 用户系统数据容器，使用 VirtualDirectory 保存 EditData 下的界面、调色板和颜色预设等虚拟文件。
// 外层使用 KCES VirtualDirectory 版本控制；已知虚拟文件按各自 MessagePack 布局解析，未知文件逐字节保留。
//
// system.dat
// KCES user system-data container using VirtualDirectory to store UI, palette, and color-preset virtual files below EditData.
// The outer layer uses KCES VirtualDirectory versioning; known virtual files use their MessagePack schemas and unknown files are preserved byte-for-byte.

const KCESSystemDataFormat = "kces-system-data"

type KCESEditDataKind string

const (
	KCESEditDataPresetPanelNames KCESEditDataKind = "preset-panel-names"
	KCESEditDataPaletteColor     KCESEditDataKind = "palette-color"
	KCESEditDataGradPoints       KCESEditDataKind = "gradation-points"
	KCESEditDataMoveablePanel    KCESEditDataKind = "moveable-panel"
	KCESEditDataPresetOrderList  KCESEditDataKind = "color-preset-order-list"
	KCESEditDataColorPreset      KCESEditDataKind = "color-preset"
)

const (
	kcesEditDataPrefix                 = "EditData/"
	kcesPresetPanelNameSaveDataPath    = "EditData/PresetPanelNameSaveData::SceneEdit::savedata"
	kcesMoveablePanelSaveDataPath      = "EditData/MoveablePanelManager::SceneEdit::savedata"
	kcesPaletteColorSaveDataNamePrefix = "PaletteColorSave"
	kcesGradPointsDataNamePrefix       = "GradSv"
)

// KCESSystemData is the semantic view of system.dat's VirtualDirectory.
// Known EditData payloads are typed, while every unrecognized virtual file is
// retained byte-for-byte in ExtraFiles so a JSON or programmatic round trip
// does not discard game data introduced by another subsystem or future build.
type KCESSystemData struct {
	Format         string                                 `json:"format"`
	Version        int32                                  `json:"version"`
	Versionless    bool                                   `json:"versionless,omitempty"`
	FilesOnly      bool                                   `json:"filesOnly,omitempty"`
	DirectoriesNil bool                                   `json:"directoriesNil,omitempty"`
	FilesNil       bool                                   `json:"filesNil,omitempty"`
	FieldCount     *int32                                 `json:"fieldCount,omitempty"`
	FutureSlots    [][]byte                               `json:"futureSlots,omitempty"`
	Directories    map[string]ct.VirtualDirectoryMetadata `json:"directories,omitempty"`
	VirtualFiles   map[string]ct.VirtualFileMetadata      `json:"virtualFiles,omitempty"`
	EditData       []KCESEditDataFile                     `json:"editData,omitempty"`
	ExtraFiles     map[string][]byte                      `json:"extraFiles,omitempty"`
}

// KCESEditDataFile is one recognized file below system.dat/EditData. Kind and
// Path are both retained deliberately: Path carries the save-slot index and
// Kind prevents an editing JSON document from silently interpreting one typed
// object as another.
type KCESEditDataFile struct {
	Path             string                   `json:"path"`
	Kind             KCESEditDataKind         `json:"kind"`
	PresetPanelNames *PresetPanelNameSaveData `json:"presetPanelNames,omitempty"`
	PaletteColor     *PaletteColorSaveData    `json:"paletteColor,omitempty"`
	GradPoints       *GradPointsData          `json:"gradPoints,omitempty"`
	MoveablePanel    *MoveablePanelSaveData   `json:"moveablePanel,omitempty"`
	PresetOrderList  *ColorPresetOrderList    `json:"presetOrderList,omitempty"`
	ColorPreset      *ColorPreset             `json:"colorPreset,omitempty"`
}

// KCESEditDataKindForPath returns the schema consumed by the game for an exact
// VirtualDirectory path. Unknown EditData files deliberately return an empty
// kind and stay opaque.
func KCESEditDataKindForPath(path string) KCESEditDataKind {
	if path == kcesPresetPanelNameSaveDataPath {
		return KCESEditDataPresetPanelNames
	}
	if path == kcesMoveablePanelSaveDataPath {
		return KCESEditDataMoveablePanel
	}
	if !strings.HasPrefix(path, kcesEditDataPrefix) {
		return ""
	}
	name := strings.TrimPrefix(path, kcesEditDataPrefix)
	if strings.HasPrefix(name, "color_preset/") {
		relative := strings.TrimPrefix(name, "color_preset/")
		if relative == "preset_orderlist" || strings.HasSuffix(relative, "/preset_orderlist") {
			return KCESEditDataPresetOrderList
		}
		// PresetSaveDirectory always contributes at least one directory name
		// before the preset ID. Every non-order-list file in that directory is
		// passed to CustomColorPresetBase.Deserializ by the game.
		lastSlash := strings.LastIndexByte(relative, '/')
		if lastSlash > 0 && lastSlash < len(relative)-1 {
			return KCESEditDataColorPreset
		}
		return ""
	}
	if hasNonNegativeDecimalSuffix(name, kcesPaletteColorSaveDataNamePrefix) {
		return KCESEditDataPaletteColor
	}
	if hasNonNegativeDecimalSuffix(name, kcesGradPointsDataNamePrefix) {
		return KCESEditDataGradPoints
	}
	return ""
}

func hasNonNegativeDecimalSuffix(value, prefix string) bool {
	if !strings.HasPrefix(value, prefix) || len(value) == len(prefix) {
		return false
	}
	for _, b := range []byte(value[len(prefix):]) {
		if b < '0' || b > '9' {
			return false
		}
	}
	return true
}

// DecodeKCESSystemData validates a VirtualDirectory container and decodes all
// the strongly typed EditData schemas known to KCES 1.34.4. A malformed known file is an
// error rather than an opaque fallback because the game will select that same
// parser by its virtual path and fail while loading system.dat.
func DecodeKCESSystemData(data []byte) (*KCESSystemData, error) {
	table, err := ct.ReadContentTable(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode KCES system.dat VirtualDirectory: %w", err)
	}
	result := &KCESSystemData{
		Format:         KCESSystemDataFormat,
		Version:        table.Version,
		Versionless:    table.Versionless,
		FilesOnly:      table.FilesOnly,
		DirectoriesNil: table.DirectoriesNil,
		FilesNil:       table.FilesNil,
		FieldCount:     table.FieldCount,
		FutureSlots:    table.FutureSlots,
		Directories:    table.GetVirtualDirectoryMetadata(),
		VirtualFiles:   table.GetVirtualFileMetadata(),
	}
	for _, path := range table.GetFileNames() {
		payload, err := table.GetFileData(path)
		if err != nil {
			return nil, fmt.Errorf("read KCES system.dat virtual file %q: %w", path, err)
		}
		kind := KCESEditDataKindForPath(path)
		if kind == "" {
			if result.ExtraFiles == nil {
				result.ExtraFiles = make(map[string][]byte)
			}
			result.ExtraFiles[path] = append([]byte(nil), payload...)
			continue
		}

		entry := KCESEditDataFile{Path: path, Kind: kind}
		switch kind {
		case KCESEditDataPresetPanelNames:
			entry.PresetPanelNames, err = DecodePresetPanelNameSaveData(payload)
		case KCESEditDataPaletteColor:
			entry.PaletteColor, err = DecodePaletteColorSaveData(payload)
		case KCESEditDataGradPoints:
			entry.GradPoints, err = DecodeGradPointsData(payload)
		case KCESEditDataMoveablePanel:
			entry.MoveablePanel, err = DecodeMoveablePanelSaveData(payload)
		case KCESEditDataPresetOrderList:
			entry.PresetOrderList, err = DecodeColorPresetOrderList(payload)
		case KCESEditDataColorPreset:
			entry.ColorPreset, err = DecodeColorPreset(payload)
		}
		if err != nil {
			return nil, fmt.Errorf("decode KCES system.dat %s file %q: %w", kind, path, err)
		}
		result.EditData = append(result.EditData, entry)
	}
	return result, nil
}

// EncodeKCESSystemData writes the VirtualDirectory representation without
// changing or defaulting the stored version.
func EncodeKCESSystemData(value *KCESSystemData) ([]byte, error) {
	if value == nil {
		return nil, fmt.Errorf("nil KCES system.dat")
	}
	if value.Format != "" && value.Format != KCESSystemDataFormat {
		return nil, fmt.Errorf("unsupported KCES system.dat JSON format %q", value.Format)
	}
	table := &ct.ContentTable{
		Version:        value.Version,
		Versionless:    value.Versionless,
		FilesOnly:      value.FilesOnly,
		DirectoriesNil: value.DirectoriesNil,
		FilesNil:       value.FilesNil,
		FieldCount:     value.FieldCount,
		FutureSlots:    value.FutureSlots,
		Directories:    value.Directories,
		Raw:            make([]byte, ct.HeaderSize),
		Files:          make(map[string]ct.VirtualFile),
	}
	seenPaths := make(map[string]struct{}, len(value.EditData)+len(value.ExtraFiles))
	for index := range value.EditData {
		entry := &value.EditData[index]
		if err := validateSystemVirtualPath(entry.Path); err != nil {
			return nil, fmt.Errorf("editData[%d].path: %w", index, err)
		}
		if _, exists := seenPaths[entry.Path]; exists {
			return nil, fmt.Errorf("duplicate KCES system.dat virtual path %q", entry.Path)
		}
		seenPaths[entry.Path] = struct{}{}

		expectedKind := KCESEditDataKindForPath(entry.Path)
		if expectedKind == "" {
			return nil, fmt.Errorf("editData[%d] path %q is not a recognized KCES 1.34.4 EditData path", index, entry.Path)
		}
		kind := entry.Kind
		if kind == "" {
			kind = expectedKind
		}
		if kind != expectedKind {
			return nil, fmt.Errorf("editData[%d] kind %q does not match path %q (expected %q)", index, kind, entry.Path, expectedKind)
		}
		if err := validateEditDataUnion(entry, kind); err != nil {
			return nil, fmt.Errorf("editData[%d]: %w", index, err)
		}

		var payload []byte
		var err error
		switch kind {
		case KCESEditDataPresetPanelNames:
			payload, err = EncodePresetPanelNameSaveData(entry.PresetPanelNames)
		case KCESEditDataPaletteColor:
			payload, err = EncodePaletteColorSaveData(entry.PaletteColor)
		case KCESEditDataGradPoints:
			payload, err = EncodeGradPointsData(entry.GradPoints)
		case KCESEditDataMoveablePanel:
			payload, err = EncodeMoveablePanelSaveData(entry.MoveablePanel)
		case KCESEditDataPresetOrderList:
			payload, err = EncodeColorPresetOrderList(entry.PresetOrderList)
		case KCESEditDataColorPreset:
			payload, err = EncodeColorPreset(entry.ColorPreset)
		default:
			err = fmt.Errorf("unsupported EditData kind %q", kind)
		}
		if err != nil {
			return nil, fmt.Errorf("encode KCES system.dat %s file %q: %w", kind, entry.Path, err)
		}
		if err := table.AddFile(entry.Path, payload); err != nil {
			return nil, err
		}
	}

	extraPaths := make([]string, 0, len(value.ExtraFiles))
	for path := range value.ExtraFiles {
		extraPaths = append(extraPaths, path)
	}
	sort.Strings(extraPaths)
	for _, path := range extraPaths {
		if err := validateSystemVirtualPath(path); err != nil {
			return nil, fmt.Errorf("extraFiles[%q]: %w", path, err)
		}
		if _, exists := seenPaths[path]; exists {
			return nil, fmt.Errorf("KCES system.dat extraFiles collides with typed path %q", path)
		}
		seenPaths[path] = struct{}{}
		if kind := KCESEditDataKindForPath(path); kind != "" {
			return nil, fmt.Errorf("KCES system.dat extraFiles contains recognized %s path %q; use editData instead", kind, path)
		}
		if err := table.AddFile(path, append([]byte(nil), value.ExtraFiles[path]...)); err != nil {
			return nil, err
		}
	}
	if err := table.ApplyVirtualFileMetadata(value.VirtualFiles); err != nil {
		return nil, fmt.Errorf("system.dat: %w", err)
	}

	var out bytes.Buffer
	if err := ct.WriteContentTable(&out, table); err != nil {
		return nil, fmt.Errorf("encode KCES system.dat VirtualDirectory: %w", err)
	}
	return out.Bytes(), nil
}

func NewKCESSystemData() *KCESSystemData {
	return &KCESSystemData{
		Format:  KCESSystemDataFormat,
		Version: 1000,
	}
}

func validateEditDataUnion(entry *KCESEditDataFile, kind KCESEditDataKind) error {
	if entry == nil {
		return fmt.Errorf("nil EditData entry")
	}
	if kind != KCESEditDataPresetPanelNames && entry.PresetPanelNames != nil {
		return fmt.Errorf("presetPanelNames is set for kind %q", kind)
	}
	if kind != KCESEditDataPaletteColor && entry.PaletteColor != nil {
		return fmt.Errorf("paletteColor is set for kind %q", kind)
	}
	if kind != KCESEditDataGradPoints && entry.GradPoints != nil {
		return fmt.Errorf("gradPoints is set for kind %q", kind)
	}
	if kind != KCESEditDataMoveablePanel && entry.MoveablePanel != nil {
		return fmt.Errorf("moveablePanel is set for kind %q", kind)
	}
	if kind != KCESEditDataPresetOrderList && entry.PresetOrderList != nil {
		return fmt.Errorf("presetOrderList is set for kind %q", kind)
	}
	if kind != KCESEditDataColorPreset && entry.ColorPreset != nil {
		return fmt.Errorf("colorPreset is set for kind %q", kind)
	}
	return nil
}

func validateSystemVirtualPath(path string) error {
	if path == "" {
		return fmt.Errorf("virtual path is empty")
	}
	if !utf8.ValidString(path) {
		return fmt.Errorf("virtual path is not valid UTF-8")
	}
	if strings.IndexByte(path, 0) >= 0 {
		return fmt.Errorf("virtual path contains NUL")
	}
	if filepath.IsAbs(path) || filepath.VolumeName(path) != "" || strings.Contains(path, "\\") {
		return fmt.Errorf("virtual path must be a relative forward-slash path")
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("virtual path contains unsafe segment %q", segment)
		}
	}
	return nil
}
