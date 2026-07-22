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

type Representation string

const (
	RepresentationNative      Representation = "native"
	RepresentationEditingJSON Representation = "editing_json"
)

type Capability struct {
	Detect   bool
	Convert  bool
	Validate bool
	Archive  bool
}

// Format describes one stable application-level format ID.
type Format struct {
	ID             string
	Game           string
	FileType       string
	NativeSuffixes []string
	DefaultName    string
	Capability     Capability
	SchemaVersion  string
	SchemaID       string
	SchemaSHA256   string
	GuideVersion   string
	GuideID        string
	GuideSHA256    string
	GuideCoverage  string
	convert        pathConverter
}

type pathConverter struct {
	toEditing pathConversion
	toNative  pathConversion
}

type pathConversion func(context.Context, string, string, int64) error

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

type Registry struct{ formats map[string]Format }

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
		format.GuideCoverage = ""
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
				format.GuideCoverage = guide.Coverage
			}
		}
		r.formats[format.ID] = format
	}
	return r, nil
}

func (r *Registry) Lookup(id string) (Format, bool) {
	if r == nil {
		return Format{}, false
	}
	f, ok := r.formats[strings.ToLower(strings.TrimSpace(id))]
	f.NativeSuffixes = append([]string(nil), f.NativeSuffixes...)
	return f, ok
}

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

func archiveFormat(game, fileType, defaultName string, suffixes []string) Format {
	f := format(game, fileType, defaultName, suffixes, pathConverter{})
	f.Capability.Archive = true
	return f
}

func archiveConvertibleFormat(game, fileType, defaultName string, suffixes []string, converter pathConverter) Format {
	f := format(game, fileType, defaultName, suffixes, converter)
	f.Capability.Archive = true
	return f
}

func detectOnlyFormat(game, fileType, defaultName string, suffixes []string) Format {
	f := format(game, fileType, defaultName, suffixes, pathConverter{})
	f.Capability.Validate = false
	return f
}

// DefaultRegistry exposes all path-based JSON conversions currently provided
// by service/COM3D2 and service/KCES, plus the supported archive containers.
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
		format("KCES", "menuassets", "input.menuassets", []string{".menuassets"}, pathConverter{(&KCESService.PartsService{}).ConvertPartsToJson, (&KCESService.PartsService{}).ConvertJsonToParts}),
		format("KCES", "materialassets", "input.materialassets", []string{".materialassets"}, pathConverter{(&KCESService.PartsService{}).ConvertPartsToJson, (&KCESService.PartsService{}).ConvertJsonToParts}),
		format("KCES", "pmatassets", "input.pmatassets", []string{".pmatassets"}, pathConverter{(&KCESService.PartsService{}).ConvertPartsToJson, (&KCESService.PartsService{}).ConvertJsonToParts}),
		format("KCES", "model", "input.model", []string{".model"}, pathConverter{(&KCESService.PartsService{}).ConvertPartsToJson, (&KCESService.PartsService{}).ConvertJsonToParts}),
		format("KCES", "hitcheck", "input.hitcheck", []string{".hitcheck"}, pathConverter{(&KCESService.MiscService{}).ConvertMiscToJson, (&KCESService.MiscService{}).ConvertJsonToMisc}),
		format("KCES", "undressdat", "input.undressdat", []string{".undressdat"}, pathConverter{(&KCESService.MiscService{}).ConvertMiscToJson, (&KCESService.MiscService{}).ConvertJsonToMisc}),
		format("KCES", "undresspdat", "input.undresspdat", []string{".undresspdat"}, pathConverter{(&KCESService.MiscService{}).ConvertMiscToJson, (&KCESService.MiscService{}).ConvertJsonToMisc}),
		format("KCES", "nson", "input.nson", []string{".nson"}, pathConverter{(&KCESService.MiscService{}).ConvertMiscToJson, (&KCESService.MiscService{}).ConvertJsonToMisc}),
		format("KCES", "bytes", "input.material.bytes", []string{".bytes"}, pathConverter{(&KCESService.RawUnityObjectService{}).ConvertRawUnityObjectToJson, (&KCESService.RawUnityObjectService{}).ConvertJsonToRawUnityObject}),
		format("KCES", "preset", "input.preset", []string{".preset", ".perset"}, pathConverter{(&KCESService.PresetService{}).ConvertPresetToJson, (&KCESService.PresetService{}).ConvertJsonToPreset}),
		archiveConvertibleFormat("KCES", "ct", "input.ct", []string{".ct"}, pathConverter{(&KCESService.CtService{}).ConvertCtToJson, (&KCESService.CtService{}).ConvertJsonToCt}),
		archiveConvertibleFormat("KCES", "virtualdirectory", "input.vd", []string{".vd"}, pathConverter{(&KCESService.CtService{}).ConvertCtToJson, (&KCESService.CtService{}).ConvertJsonToCt}),
		archiveFormat("KCES", "aba", "input.aba", []string{".aba", ".asset_bg", ".asset_scene"}),
		archiveFormat("KCES", "asset_scene", "input.asset_scene", []string{".asset_scene"}),
	}

	payloadExtensions := []string{"dbconf", "dbcol", "db2conf", "dsbconf", "dsb2conf", "dslconf", "dsl2conf", "dslcol", "ikcol", "ikcol.bytes", "limbcol"}
	for _, ext := range payloadExtensions {
		suffix := "." + ext
		formats = append(formats, format("KCES", ext, "input"+suffix, []string{suffix}, pathConverter{(&KCESService.PayloadService{}).ConvertPayloadToJson, (&KCESService.PayloadService{}).ConvertJsonToPayload}))
	}

	r, err := NewRegistry(formats)
	if err != nil {
		panic(err)
	}
	return r
}
