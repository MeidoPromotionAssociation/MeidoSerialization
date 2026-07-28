package KCES

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/internal/strictjson"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

// system.dat
// KCES 用户系统数据容器，使用 VirtualDirectory 保存 EditData 下的界面、调色板和颜色预设等虚拟文件
// 外层使用 KCES VirtualDirectory 版本控制，已知虚拟文件按各自 MessagePack 布局解析，只有不匹配任何已知路径的独立虚拟文件才逐字节保留
// system.dat
// KCES user system-data container using VirtualDirectory to store UI, palette, and color-preset virtual files below EditData
// The outer layer uses KCES VirtualDirectory versioning, known virtual files use their MessagePack schemas, and only independent virtual files matching no known path are preserved byte-for-byte

const KCESSystemDataFormat = "kces-system-data"

// KCESEditDataKind 标识游戏根据 system.dat 虚拟路径选择的已知 EditData 载荷模式
// KCESEditDataKind identifies a known EditData payload schema selected by the game from its system.dat virtual path
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

// KCESSystemData 是 system.dat 的 VirtualDirectory 语义视图
// 已知 EditData 载荷公开为强类型，只有不匹配任何已知路径的独立虚拟文件才逐字节保存在 ExtraFiles 中，已知路径解析失败必须返回错误
// KCESSystemData is the semantic view of the system.dat VirtualDirectory
// Known EditData payloads are typed while only independent virtual files matching no known path are retained byte-for-byte in ExtraFiles, and a parse failure at a known path must return an error
type KCESSystemData struct {
	Format           string                                 `json:"format"`                     // 库的可编辑表示标识，不写入游戏文件 / Library editing-representation identifier, not written to the game file
	Version          int32                                  `json:"version"`                    // VirtualDirectory 对象版本 / VirtualDirectory object version
	ContainerFraming ct.VirtualDirectoryFraming             `json:"containerFraming,omitempty"` // VirtualDirectory MessagePack 目录的外层尾部封装 / Outer footer frame around the VirtualDirectory MessagePack directory
	Directories      map[string]ct.VirtualDirectoryMetadata `json:"directories,omitempty"`      // 各虚拟目录的真实版本字段 / Real version fields of each virtual directory
	EditData         []KCESEditDataFile                     `json:"editData,omitempty"`         // 按虚拟路径识别并解码的 EditData 文件 / EditData files recognized and decoded by virtual path
	ExtraFiles       map[string][]byte                      `json:"extraFiles,omitempty"`       // 未识别虚拟文件的真实 byte[] 载荷 / Real byte-array payloads of unrecognized virtual files
}

// UnmarshalJSON 严格解码 system.dat 编辑封套并要求 format 与 version 显式出现
// UnmarshalJSON strictly decodes the system.dat editing envelope and requires format and version to be explicitly present
func (value *KCESSystemData) UnmarshalJSON(data []byte) error {
	if value == nil {
		return fmt.Errorf("nil KCES system.dat JSON target")
	}
	type plainKCESSystemData KCESSystemData
	var decoded plainKCESSystemData
	if err := decodeKCESJSONStrict(data, &decoded); err != nil {
		return err
	}
	if err := strictjson.RequireObjectFields(data, "systemData", "format", "version"); err != nil {
		return err
	}
	*value = KCESSystemData(decoded)
	return nil
}

// KCESEditDataFile 表示 system.dat 的 EditData 目录下一个已识别文件
// Path 保留存档槽位索引，Kind 防止编辑后的 JSON 将一种强类型对象静默解释为另一种
// KCESEditDataFile represents one recognized file below the EditData directory in system.dat
// Path retains the save-slot index while Kind prevents edited JSON from silently interpreting one typed object as another
type KCESEditDataFile struct {
	Path             string                   `json:"path"`                       // 游戏用于选择解析器的完整 VirtualDirectory 路径 / Complete VirtualDirectory path used by the game to select a parser
	Kind             KCESEditDataKind         `json:"kind"`                       // 从路径确定的强类型载荷种类 / Typed payload kind determined from the path
	PresetPanelNames *PresetPanelNameSaveData `json:"presetPanelNames,omitempty"` // 预设面板框名称载荷，仅用于对应种类 / Preset-panel box-name payload used only for its matching kind
	PaletteColor     *PaletteColorSaveData    `json:"paletteColor,omitempty"`     // 调色板颜色槽载荷，仅用于对应种类 / Palette-color slot payload used only for its matching kind
	GradPoints       *GradPointsData          `json:"gradPoints,omitempty"`       // 渐变控制点载荷，仅用于对应种类 / Gradation control-point payload used only for its matching kind
	MoveablePanel    *MoveablePanelSaveData   `json:"moveablePanel,omitempty"`    // 可移动面板状态载荷，仅用于对应种类 / Moveable-panel state payload used only for its matching kind
	PresetOrderList  *ColorPresetOrderList    `json:"presetOrderList,omitempty"`  // 颜色预设顺序载荷，仅用于对应种类 / Color-preset order payload used only for its matching kind
	ColorPreset      *ColorPreset             `json:"colorPreset,omitempty"`      // 用户颜色预设载荷，仅用于对应种类 / User color-preset payload used only for its matching kind
}

// kcesEditDataFileJSON 以原始 JSON 值区分 EditData union 分支字段缺失与显式 null / kcesEditDataFileJSON distinguishes missing EditData union fields from explicit null by retaining each JSON value
type kcesEditDataFileJSON struct {
	Path             string           `json:"path"`                       // 完整虚拟路径 / Complete virtual path
	Kind             KCESEditDataKind `json:"kind"`                       // union 判别类型 / Union discriminator
	PresetPanelNames json.RawMessage  `json:"presetPanelNames,omitempty"` // 预设面板框名称分支 / Preset-panel box-name branch
	PaletteColor     json.RawMessage  `json:"paletteColor,omitempty"`     // 调色板颜色分支 / Palette-color branch
	GradPoints       json.RawMessage  `json:"gradPoints,omitempty"`       // 渐变控制点分支 / Gradation control-point branch
	MoveablePanel    json.RawMessage  `json:"moveablePanel,omitempty"`    // 可移动面板分支 / Moveable-panel branch
	PresetOrderList  json.RawMessage  `json:"presetOrderList,omitempty"`  // 颜色预设顺序分支 / Color-preset order branch
	ColorPreset      json.RawMessage  `json:"colorPreset,omitempty"`      // 用户颜色预设分支 / User color-preset branch
}

// MarshalJSON 仅写出 Kind 对应的活动 EditData 分支并让类型化 nil 根显式成为 JSON null
// MarshalJSON emits only the EditData branch selected by Kind and represents a typed nil root as explicit JSON null
func (entry KCESEditDataFile) MarshalJSON() ([]byte, error) {
	active := editDataRootFieldName(entry.Kind)
	if active == "" {
		return nil, fmt.Errorf("unsupported EditData kind %q", entry.Kind)
	}
	if err := validateEditDataJSONInactiveRoots(&entry, active); err != nil {
		return nil, err
	}

	raw := kcesEditDataFileJSON{Path: entry.Path, Kind: entry.Kind}
	var err error
	switch active {
	case "presetPanelNames":
		raw.PresetPanelNames, err = json.Marshal(entry.PresetPanelNames)
	case "paletteColor":
		raw.PaletteColor, err = json.Marshal(entry.PaletteColor)
	case "gradPoints":
		raw.GradPoints, err = json.Marshal(entry.GradPoints)
	case "moveablePanel":
		raw.MoveablePanel, err = json.Marshal(entry.MoveablePanel)
	case "presetOrderList":
		raw.PresetOrderList, err = json.Marshal(entry.PresetOrderList)
	case "colorPreset":
		raw.ColorPreset, err = json.Marshal(entry.ColorPreset)
	}
	if err != nil {
		return nil, fmt.Errorf("marshal active EditData field %s: %w", active, err)
	}
	return json.Marshal(raw)
}

// UnmarshalJSON 严格解码一个 EditData union，要求活动分支存在并拒绝未知字段或任何非活动分支
// UnmarshalJSON strictly decodes one EditData union, requires the active branch, and rejects unknown fields or every inactive branch
func (entry *KCESEditDataFile) UnmarshalJSON(data []byte) error {
	var raw kcesEditDataFileJSON
	if err := decodeKCESJSONStrict(data, &raw); err != nil {
		return err
	}
	active := editDataRootFieldName(raw.Kind)
	if active == "" {
		return fmt.Errorf("unsupported EditData kind %q", raw.Kind)
	}
	if err := validateEditDataJSONRootPresence(&raw, active); err != nil {
		return err
	}

	value := KCESEditDataFile{Path: raw.Path, Kind: raw.Kind}
	var err error
	switch active {
	case "presetPanelNames":
		err = decodeKCESJSONStrict(raw.PresetPanelNames, &value.PresetPanelNames)
	case "paletteColor":
		err = decodeKCESJSONStrict(raw.PaletteColor, &value.PaletteColor)
	case "gradPoints":
		err = decodeKCESJSONStrict(raw.GradPoints, &value.GradPoints)
	case "moveablePanel":
		err = decodeKCESJSONStrict(raw.MoveablePanel, &value.MoveablePanel)
	case "presetOrderList":
		err = decodeKCESJSONStrict(raw.PresetOrderList, &value.PresetOrderList)
	case "colorPreset":
		err = decodeKCESJSONStrict(raw.ColorPreset, &value.ColorPreset)
	}
	if err != nil {
		return fmt.Errorf("decode active EditData field %s: %w", active, err)
	}
	*entry = value
	return nil
}

// editDataRootFieldName 返回 EditData kind 在 editing JSON 中选择的活动 union 字段
// editDataRootFieldName returns the active editing-JSON union field selected by an EditData kind
func editDataRootFieldName(kind KCESEditDataKind) string {
	switch kind {
	case KCESEditDataPresetPanelNames:
		return "presetPanelNames"
	case KCESEditDataPaletteColor:
		return "paletteColor"
	case KCESEditDataGradPoints:
		return "gradPoints"
	case KCESEditDataMoveablePanel:
		return "moveablePanel"
	case KCESEditDataPresetOrderList:
		return "presetOrderList"
	case KCESEditDataColorPreset:
		return "colorPreset"
	default:
		return ""
	}
}

// validateEditDataJSONInactiveRoots 拒绝 Go 值中与 Kind 不匹配且实际携带值的 EditData 分支
// validateEditDataJSONInactiveRoots rejects populated Go EditData branches that do not match Kind
func validateEditDataJSONInactiveRoots(entry *KCESEditDataFile, active string) error {
	for _, root := range []struct {
		name    string
		present bool
	}{
		{name: "presetPanelNames", present: entry.PresetPanelNames != nil},
		{name: "paletteColor", present: entry.PaletteColor != nil},
		{name: "gradPoints", present: entry.GradPoints != nil},
		{name: "moveablePanel", present: entry.MoveablePanel != nil},
		{name: "presetOrderList", present: entry.PresetOrderList != nil},
		{name: "colorPreset", present: entry.ColorPreset != nil},
	} {
		if root.present && root.name != active {
			return fmt.Errorf("%s is inactive for EditData kind %q", root.name, entry.Kind)
		}
	}
	return nil
}

// validateEditDataJSONRootPresence 要求活动字段出现并拒绝所有出现的非活动字段，包括显式 null
// validateEditDataJSONRootPresence requires the active field and rejects every present inactive field including explicit null
func validateEditDataJSONRootPresence(raw *kcesEditDataFileJSON, active string) error {
	for _, root := range []struct {
		name string
		data json.RawMessage
	}{
		{name: "presetPanelNames", data: raw.PresetPanelNames},
		{name: "paletteColor", data: raw.PaletteColor},
		{name: "gradPoints", data: raw.GradPoints},
		{name: "moveablePanel", data: raw.MoveablePanel},
		{name: "presetOrderList", data: raw.PresetOrderList},
		{name: "colorPreset", data: raw.ColorPreset},
	} {
		if root.name == active && len(root.data) == 0 {
			return fmt.Errorf("EditData kind %q requires field %s", raw.Kind, active)
		}
		if root.name != active && len(root.data) != 0 {
			return fmt.Errorf("%s is inactive for EditData kind %q", root.name, raw.Kind)
		}
	}
	return nil
}

// KCESEditDataKindForPath 返回游戏对给定精确 VirtualDirectory 路径采用的载荷模式
// 未知 EditData 文件有意返回空种类并保持不透明
// KCESEditDataKindForPath returns the payload schema consumed by the game for an exact VirtualDirectory path
// Unknown EditData files deliberately return an empty kind and remain opaque
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
		// PresetSaveDirectory 在预设 ID 前至少会加入一个目录名
		// 游戏会将该目录中顺序列表以外的每个文件传给 CustomColorPresetBase.Deserializ
		// PresetSaveDirectory always contributes at least one directory name before the preset ID
		// The game passes every non-order-list file in that directory to CustomColorPresetBase.Deserializ
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

// hasNonNegativeDecimalSuffix 判断字符串是否由给定前缀和至少一位十进制数字组成
// hasNonNegativeDecimalSuffix reports whether a string consists of the given prefix followed by at least one decimal digit
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

// DecodeKCESSystemData 验证 VirtualDirectory 容器并解码 KCES 1.34.4 已知的所有强类型 EditData 模式
// 已知路径中的畸形文件会返回错误而不会退回不透明数据，因为游戏也会按该路径选择同一解析器并在加载 system.dat 时失败
// DecodeKCESSystemData validates a VirtualDirectory container and decodes every strongly typed EditData schema known to KCES 1.34.4
// A malformed file at a known path returns an error instead of falling back to opaque data because the game selects the same parser by path and fails while loading system.dat
func DecodeKCESSystemData(data []byte) (*KCESSystemData, error) {
	table, err := ct.ReadContentTable(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode KCES system.dat VirtualDirectory: %w", err)
	}
	result := &KCESSystemData{
		Format:           KCESSystemDataFormat,
		Version:          table.Version,
		ContainerFraming: table.Framing,
		Directories:      table.GetVirtualDirectoryMetadata(),
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

// EncodeKCESSystemData 写入 VirtualDirectory 表示而不更改或补默认存储版本
// EncodeKCESSystemData writes the VirtualDirectory representation without changing or defaulting the stored version
func EncodeKCESSystemData(value *KCESSystemData) ([]byte, error) {
	if value == nil {
		return nil, fmt.Errorf("nil KCES system.dat")
	}
	if value.Format != "" && value.Format != KCESSystemDataFormat {
		return nil, fmt.Errorf("unsupported KCES system.dat JSON format %q", value.Format)
	}
	table := &ct.ContentTable{
		Version:     value.Version,
		Framing:     value.ContainerFraming,
		Directories: value.Directories,
		Raw:         make([]byte, ct.HeaderSize),
		Files:       make(map[string]ct.VirtualFile),
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
	var out bytes.Buffer
	if err := ct.WriteContentTable(&out, table); err != nil {
		return nil, fmt.Errorf("encode KCES system.dat VirtualDirectory: %w", err)
	}
	return out.Bytes(), nil
}

// NewKCESSystemData 创建显式使用当前 VirtualDirectory 版本 1000 的空系统数据
// NewKCESSystemData creates empty system data explicitly using the current VirtualDirectory version 1000
func NewKCESSystemData() *KCESSystemData {
	return &KCESSystemData{
		Format:  KCESSystemDataFormat,
		Version: 1000,
	}
}

// validateEditDataUnion 确认已识别文件只设置与其 Kind 对应的强类型载荷
// validateEditDataUnion verifies that a recognized file sets only the typed payload matching its Kind
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

// validateSystemVirtualPath 拒绝不能安全表示为相对正斜杠 VirtualDirectory 路径的名称
// validateSystemVirtualPath rejects names that cannot safely represent relative forward-slash VirtualDirectory paths
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
