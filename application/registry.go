package application

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	editingv1 "github.com/MeidoPromotionAssociation/MeidoSerialization/schemas/editing/v1"
	knowledgev1 "github.com/MeidoPromotionAssociation/MeidoSerialization/schemas/knowledge/v1"
	COM3D2Service "github.com/MeidoPromotionAssociation/MeidoSerialization/service/COM3D2"
	KCESService "github.com/MeidoPromotionAssociation/MeidoSerialization/service/KCES"
)

// Representation 表示应用层文件内容的形态 / Representation identifies the application-level form of file content
type Representation string

const (
	// RepresentationNative 表示游戏使用的原生文件格式 / RepresentationNative identifies the native file format consumed by the game
	RepresentationNative Representation = "native"
	// RepresentationEditingJSON 表示用于编辑和交换的 JSON 形式 / RepresentationEditingJSON identifies the JSON form used for editing and interchange
	RepresentationEditingJSON Representation = "editing_json"
)

// Capability 描述一个格式在应用层支持的操作 / Capability describes the application operations supported by a format
type Capability struct {
	// Detect 表示是否支持格式检测 / Detect reports whether format detection is supported
	Detect bool
	// Convert 表示是否支持原生格式与编辑 JSON 互转 / Convert reports whether native and editing JSON conversion is supported
	Convert bool
	// Validate 表示是否支持完整格式校验 / Validate reports whether full format validation is supported
	Validate bool
	// Archive 表示是否支持归档列表和条目提取 / Archive reports whether archive listing and entry extraction are supported
	Archive bool
}

// Format 描述一个具有稳定应用层标识符的游戏文件格式 / Format describes a game file format with a stable application-level identifier
type Format struct {
	// ID 是由游戏名和文件类型组成的稳定格式标识符 / ID is the stable format identifier composed from the game and file type
	ID string
	// Game 是拥有该格式的游戏或工具名称 / Game is the name of the game or tool that owns the format
	Game string
	// FileType 是服务层使用的规范文件类型名称 / FileType is the canonical file type name used by the service layer
	FileType string
	// NativeSuffixes 是该格式接受的原生文件后缀 / NativeSuffixes contains the native file suffixes accepted for the format
	NativeSuffixes []string
	// DefaultName 是缺少可用输入名称时采用的原生文件名 / DefaultName is the native filename used when no suitable input name is available
	DefaultName string
	// Capability 描述该格式支持的应用操作 / Capability describes the application operations supported by the format
	Capability Capability
	// SchemaVersion 是已发布编辑模式的版本 / SchemaVersion is the version of the published editing schema
	SchemaVersion string
	// SchemaID 是已发布编辑模式的规范标识符 / SchemaID is the canonical identifier of the published editing schema
	SchemaID string
	// SchemaSHA256 是已发布编辑模式的 SHA-256 摘要 / SchemaSHA256 is the SHA-256 digest of the published editing schema
	SchemaSHA256 string
	// GuideVersion 是已发布格式指南的版本 / GuideVersion is the version of the published format guide
	GuideVersion string
	// GuideID 是已发布格式指南的规范标识符 / GuideID is the canonical identifier of the published format guide
	GuideID string
	// GuideSHA256 是已发布格式指南的 SHA-256 摘要 / GuideSHA256 is the SHA-256 digest of the published format guide
	GuideSHA256 string
	// GuideVerification 描述已发布指南对整个文件格式的认证等级 / GuideVerification describes the whole-file verification level of the published guide
	GuideVerification string
	// convert 保存不向注册表调用方公开的路径转换器 / convert stores the path converter hidden from registry callers
	convert pathConverter
}

// pathConverter 保存原生格式与编辑 JSON 之间的双向路径转换函数 / pathConverter stores bidirectional path conversions between native data and editing JSON
type pathConverter struct {
	// toEditing 将原生文件转换为编辑 JSON / toEditing converts a native file to editing JSON
	toEditing pathConversion
	// toNative 将编辑 JSON 转换为原生文件 / toNative converts editing JSON to a native file
	toNative pathConversion
}

// pathConversion 定义受上下文和输出大小限制约束的路径转换函数 / pathConversion defines a path conversion constrained by a context and output-size limit
type pathConversion func(context.Context, string, string, int64) error

// run 选择目标表示的转换函数并检查上下文与输出资源限制
// run selects the conversion for a target representation and enforces context and output resource limits
func (c pathConverter) run(ctx context.Context, to Representation, inputPath, outputPath string, maxOutputBytes int64) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if maxOutputBytes <= 0 {
		return fmt.Errorf("positive conversion output limit is required")
	}
	var convert pathConversion
	switch to {
	case RepresentationEditingJSON:
		if c.toEditing == nil {
			return fmt.Errorf("conversion to editing JSON is unavailable")
		}
		convert = c.toEditing
	case RepresentationNative:
		if c.toNative == nil {
			return fmt.Errorf("conversion to native format is unavailable")
		}
		convert = c.toNative
	default:
		return fmt.Errorf("unsupported target representation %q", to)
	}
	if err := convert(ctx, inputPath, outputPath, maxOutputBytes); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := inspectArtifactFiles(ctx, outputPath, maxOutputBytes)
	return err
}

// Registry 保存以稳定标识符索引的受支持格式 / Registry stores supported formats indexed by stable identifiers
type Registry struct {
	// formats 将规范化格式标识符映射到不可变的格式元数据副本 / formats maps normalized format identifiers to immutable copies of format metadata
	formats map[string]Format
}

// NewRegistry 校验格式定义并构建带有已发布模式和指南元数据的注册表
// NewRegistry validates format definitions and builds a registry enriched with published schema and guide metadata
func NewRegistry(formats []Format) (*Registry, error) {
	r := &Registry{formats: make(map[string]Format, len(formats))}
	for _, format := range formats {
		format.ID = strings.ToLower(strings.TrimSpace(format.ID))
		if format.ID == "" || !strings.Contains(format.ID, ".") {
			return nil, fmt.Errorf("invalid format ID %q", format.ID)
		}
		if _, exists := r.formats[format.ID]; exists {
			return nil, fmt.Errorf("duplicate format ID %q", format.ID)
		}
		format.NativeSuffixes = append([]string(nil), format.NativeSuffixes...)
		format.SchemaVersion = ""
		format.SchemaID = ""
		format.SchemaSHA256 = ""
		format.GuideVersion = ""
		format.GuideID = ""
		format.GuideSHA256 = ""
		format.GuideVerification = ""
		if format.Capability.Convert {
			document, found, err := editingv1.Lookup(format.ID)
			if err != nil {
				return nil, fmt.Errorf("load editing schema for %q: %w", format.ID, err)
			}
			if found {
				if !slices.Equal(format.NativeSuffixes, document.NativeSuffixes) {
					return nil, fmt.Errorf("editing schema suffixes for %q are %v, registry suffixes are %v", format.ID, document.NativeSuffixes, format.NativeSuffixes)
				}
				format.SchemaVersion = document.Version
				format.SchemaID = document.ID
				format.SchemaSHA256 = document.SHA256
				guide, guideErr := knowledgev1.Resolve(format.ID, document.ID, document.JSON)
				if guideErr != nil {
					return nil, fmt.Errorf("load format guide for %q: %w", format.ID, guideErr)
				}
				format.GuideVersion = guide.Version
				format.GuideID = guide.ID
				format.GuideSHA256 = guide.SHA256
				format.GuideVerification = guide.FormatVerification
			}
		}
		r.formats[format.ID] = format
	}
	return r, nil
}

// Lookup 按不区分大小写的格式标识符返回格式元数据副本
// Lookup returns a copy of format metadata by a case-insensitive format identifier
func (r *Registry) Lookup(id string) (Format, bool) {
	if r == nil {
		return Format{}, false
	}
	f, ok := r.formats[strings.ToLower(strings.TrimSpace(id))]
	f.NativeSuffixes = append([]string(nil), f.NativeSuffixes...)
	return f, ok
}

// Formats 返回按格式标识符排序且隐藏内部转换函数的格式元数据副本
// Formats returns format metadata copies sorted by identifier with internal converters hidden
func (r *Registry) Formats() []Format {
	if r == nil {
		return nil
	}
	result := make([]Format, 0, len(r.formats))
	for _, f := range r.formats {
		f.convert = pathConverter{}
		f.NativeSuffixes = append([]string(nil), f.NativeSuffixes...)
		result = append(result, f)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

// format 从游戏、文件类型、后缀和转换器构建基础格式定义
// format builds a base format definition from a game, file type, suffixes, and converter
func format(game, fileType, defaultName string, suffixes []string, converter pathConverter) Format {
	canConvert := converter.toEditing != nil && converter.toNative != nil
	return Format{
		ID:             strings.ToLower(game) + "." + strings.ToLower(fileType),
		Game:           game,
		FileType:       fileType,
		DefaultName:    defaultName,
		NativeSuffixes: suffixes,
		Capability:     Capability{Detect: true, Convert: canConvert, Validate: true},
		convert:        converter,
	}
}

// archiveFormat 构建仅支持原生归档操作的格式定义
// archiveFormat builds a format definition supporting native archive operations only
func archiveFormat(game, fileType, defaultName string, suffixes []string) Format {
	f := format(game, fileType, defaultName, suffixes, pathConverter{})
	f.Capability.Archive = true
	return f
}

// archiveConvertibleFormat 构建同时支持归档操作和编辑 JSON 转换的格式定义
// archiveConvertibleFormat builds a format definition supporting both archive operations and editing JSON conversion
func archiveConvertibleFormat(game, fileType, defaultName string, suffixes []string, converter pathConverter) Format {
	f := format(game, fileType, defaultName, suffixes, converter)
	f.Capability.Archive = true
	return f
}

// detectOnlyFormat 构建只能检测而不能执行完整校验的格式定义
// detectOnlyFormat builds a format definition that can be detected but not fully validated
func detectOnlyFormat(game, fileType, defaultName string, suffixes []string) Format {
	f := format(game, fileType, defaultName, suffixes, pathConverter{})
	f.Capability.Validate = false
	return f
}

// DefaultRegistry 返回包含 COM3D2 与 KCES 转换器及受支持归档容器的默认注册表
// DefaultRegistry returns the default registry containing COM3D2 and KCES converters and supported archive containers
func DefaultRegistry() *Registry {
	formats := []Format{
		format("COM3D2", "menu", "input.menu", []string{".menu"}, pathConverter{(&COM3D2Service.MenuService{}).ConvertMenuToJson, (&COM3D2Service.MenuService{}).ConvertJsonToMenu}),
		format("COM3D2", "mate", "input.mate", []string{".mate", ".mat"}, pathConverter{(&COM3D2Service.MateService{}).ConvertMateToJson, (&COM3D2Service.MateService{}).ConvertJsonToMate}),
		format("COM3D2", "pmat", "input.pmat", []string{".pmat"}, pathConverter{(&COM3D2Service.PMatService{}).ConvertPMatToJson, (&COM3D2Service.PMatService{}).ConvertJsonToPMat}),
		format("COM3D2", "col", "input.col", []string{".col"}, pathConverter{(&COM3D2Service.ColService{}).ConvertColToJson, (&COM3D2Service.ColService{}).ConvertJsonToCol}),
		format("COM3D2", "phy", "input.phy", []string{".phy"}, pathConverter{(&COM3D2Service.PhyService{}).ConvertPhyToJson, (&COM3D2Service.PhyService{}).ConvertJsonToPhy}),
		format("COM3D2", "psk", "input.psk", []string{".psk"}, pathConverter{(&COM3D2Service.PskService{}).ConvertPskToJson, (&COM3D2Service.PskService{}).ConvertJsonToPsk}),
		format("COM3D2", "anm", "input.anm", []string{".anm"}, pathConverter{(&COM3D2Service.AnmService{}).ConvertAnmToJson, (&COM3D2Service.AnmService{}).ConvertJsonToAnm}),
		format("COM3D2", "model", "input.model", []string{".model"}, pathConverter{(&COM3D2Service.ModelService{}).ConvertModelToJson, (&COM3D2Service.ModelService{}).ConvertJsonToModel}),
		format("COM3D2", "preset", "input.preset", []string{".preset"}, pathConverter{(&COM3D2Service.PresetService{}).ConvertPresetToJson, (&COM3D2Service.PresetService{}).ConvertJsonToPreset}),
		format("COM3D2", "timeline", "timeline_data.bytes", []string{".bytes"}, pathConverter{(&COM3D2Service.DanceService{}).ConvertTimelineDataToJson, (&COM3D2Service.DanceService{}).ConvertJsonToTimelineData}),
		format("COM3D2", "object_data", "maid_data.bytes", []string{".bytes"}, pathConverter{(&COM3D2Service.DanceService{}).ConvertDanceObjectDataToJson, (&COM3D2Service.DanceService{}).ConvertJsonToDanceObjectData}),
		detectOnlyFormat("COM3D2", "tex", "input.tex", []string{".tex"}),
		detectOnlyFormat("COM3D2", "save", "input.save", []string{".save"}),
		archiveFormat("COM3D2", "arc", "input.arc", []string{".arc"}),
		format("KCES", "bridge_session", "bridge_session.vd", []string{".vd"}, pathConverter{(&KCESService.BridgeSessionService{}).ConvertBridgeSessionToJSON, (&KCESService.BridgeSessionService{}).ConvertJSONToBridgeSession}),
		format("KCES", "brd", "input.brd", []string{".brd"}, pathConverter{(&KCESService.GP03BridgeService{}).ConvertBridgeToJSON, (&KCESService.GP03BridgeService{}).ConvertJSONToBridge}),
		format("KCES", "enm", "export_map.enm", []string{".enm"}, pathConverter{(&KCESService.ExportNameMapService{}).ConvertExportNameMapToJSON, (&KCESService.ExportNameMapService{}).ConvertJSONToExportNameMap}),
		format("KCES", "sad", "input.sad", []string{".sad"}, pathConverter{(&KCESService.SavedAttachService{}).ConvertSavedAttachToJSON, (&KCESService.SavedAttachService{}).ConvertJSONToSavedAttach}),
		format("KCES", "system", "system.dat", []string{"system.dat"}, pathConverter{(&KCESService.SystemDataService{}).ConvertSystemDataToJSON, (&KCESService.SystemDataService{}).ConvertJSONToSystemData}),
		format("KCES", "paths", "paths.dat", []string{"paths.dat"}, pathConverter{(&KCESService.PathsService{}).ConvertPathsToJSON, (&KCESService.PathsService{}).ConvertJSONToPaths}),
		format("KCES", "maid_collider", "maid_collider.bytes", []string{".bytes"}, pathConverter{(&KCESService.MaidColliderService{}).ConvertMaidColliderToJSON, (&KCESService.MaidColliderService{}).ConvertJSONToMaidCollider}),
		format("KCES", "menuassets", "input.menuassets", []string{".menuassets"}, pathConverter{(&KCESService.MenuAssetsService{}).ConvertMenuAssetsToJson, (&KCESService.MenuAssetsService{}).ConvertJsonToMenuAssets}),
		format("KCES", "materialassets", "input.materialassets", []string{".materialassets"}, pathConverter{(&KCESService.MaterialAssetsService{}).ConvertMaterialAssetsToJson, (&KCESService.MaterialAssetsService{}).ConvertJsonToMaterialAssets}),
		format("KCES", "pmatassets", "input.pmatassets", []string{".pmatassets"}, pathConverter{(&KCESService.PriorityMaterialAssetsService{}).ConvertPriorityMaterialAssetsToJson, (&KCESService.PriorityMaterialAssetsService{}).ConvertJsonToPriorityMaterialAssets}),
		format("KCES", "model", "input.model", []string{".model"}, pathConverter{(&KCESService.ModelService{}).ConvertModelToJson, (&KCESService.ModelService{}).ConvertJsonToModel}),
		format("KCES", "hitcheck", "input.hitcheck", []string{".hitcheck"}, pathConverter{(&KCESService.HitCheckService{}).ConvertHitCheckToJson, (&KCESService.HitCheckService{}).ConvertJsonToHitCheck}),
		format("KCES", "undressdat", "input.undressdat", []string{".undressdat"}, pathConverter{(&KCESService.UndressDataService{}).ConvertUndressDataToJson, (&KCESService.UndressDataService{}).ConvertJsonToUndressData}),
		format("KCES", "undresspdat", "input.undresspdat", []string{".undresspdat"}, pathConverter{(&KCESService.UndressPartsDataService{}).ConvertUndressPartsDataToJson, (&KCESService.UndressPartsDataService{}).ConvertJsonToUndressPartsData}),
		format("KCES", "nson", "input.nson", []string{".nson"}, pathConverter{(&KCESService.NSONService{}).ConvertNSONToJson, (&KCESService.NSONService{}).ConvertJsonToNSON}),
		format("KCES", "bytes", "input.material.bytes", []string{".bytes"}, pathConverter{(&KCESService.RawUnityObjectService{}).ConvertRawUnityObjectToJson, (&KCESService.RawUnityObjectService{}).ConvertJsonToRawUnityObject}),
		format("KCES", "preset", "input.preset", []string{".preset", ".perset"}, pathConverter{(&KCESService.PresetService{}).ConvertPresetToJson, (&KCESService.PresetService{}).ConvertJsonToPreset}),
		archiveConvertibleFormat("KCES", "ct", "input.ct", []string{".ct"}, pathConverter{(&KCESService.CtService{}).ConvertCtToJson, (&KCESService.CtService{}).ConvertJsonToCt}),
		archiveConvertibleFormat("KCES", "virtualdirectory", "input.vd", []string{".vd"}, pathConverter{(&KCESService.CtService{}).ConvertCtToJson, (&KCESService.CtService{}).ConvertJsonToCt}),
		archiveFormat("KCES", "aba", "input.aba", []string{".aba"}),
		archiveFormat("KCES", "asset_bg", "input.asset_bg", []string{".asset_bg"}),
		archiveFormat("KCES", "asset_scene", "input.asset_scene", []string{".asset_scene"}),
		format("KCES", "dbconf", "input.dbconf", []string{".dbconf"}, pathConverter{(&KCESService.DBConfService{}).ConvertDBConfToJson, (&KCESService.DBConfService{}).ConvertJsonToDBConf}),
		format("KCES", "dbcol", "input.dbcol", []string{".dbcol"}, pathConverter{(&KCESService.DBColService{}).ConvertDBColToJson, (&KCESService.DBColService{}).ConvertJsonToDBCol}),
		format("KCES", "db2conf", "input.db2conf", []string{".db2conf"}, pathConverter{(&KCESService.DB2ConfService{}).ConvertDB2ConfToJson, (&KCESService.DB2ConfService{}).ConvertJsonToDB2Conf}),
		format("KCES", "dsbconf", "input.dsbconf", []string{".dsbconf"}, pathConverter{(&KCESService.DSBConfService{}).ConvertDSBConfToJson, (&KCESService.DSBConfService{}).ConvertJsonToDSBConf}),
		format("KCES", "dsb2conf", "input.dsb2conf", []string{".dsb2conf"}, pathConverter{(&KCESService.DSB2ConfService{}).ConvertDSB2ConfToJson, (&KCESService.DSB2ConfService{}).ConvertJsonToDSB2Conf}),
		format("KCES", "dslconf", "input.dslconf", []string{".dslconf"}, pathConverter{(&KCESService.DSLConfService{}).ConvertDSLConfToJson, (&KCESService.DSLConfService{}).ConvertJsonToDSLConf}),
		format("KCES", "dsl2conf", "input.dsl2conf", []string{".dsl2conf"}, pathConverter{(&KCESService.DSL2ConfService{}).ConvertDSL2ConfToJson, (&KCESService.DSL2ConfService{}).ConvertJsonToDSL2Conf}),
		format("KCES", "dslcol", "input.dslcol", []string{".dslcol"}, pathConverter{(&KCESService.DSLColService{}).ConvertDSLColToJson, (&KCESService.DSLColService{}).ConvertJsonToDSLCol}),
		format("KCES", "ikcol", "input.ikcol", []string{".ikcol"}, pathConverter{(&KCESService.IKColService{}).ConvertIKColToJson, (&KCESService.IKColService{}).ConvertJsonToIKCol}),
		format("KCES", "ikcol.bytes", "input.ikcol.bytes", []string{".ikcol.bytes"}, pathConverter{(&KCESService.IKColBytesService{}).ConvertIKColBytesToJson, (&KCESService.IKColBytesService{}).ConvertJsonToIKColBytes}),
		format("KCES", "limbcol", "input.limbcol", []string{".limbcol"}, pathConverter{(&KCESService.LimbColService{}).ConvertLimbColToJson, (&KCESService.LimbColService{}).ConvertJsonToLimbCol}),
	}

	r, err := NewRegistry(formats)
	if err != nil {
		panic(err)
	}
	return r
}
