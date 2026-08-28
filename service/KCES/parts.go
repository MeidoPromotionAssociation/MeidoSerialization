package KCES

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES"
)

// PartsService 为旧调用方提供所有 KCES 部件载荷的兼容分派入口 / PartsService provides compatibility dispatch for all KCES parts payloads
type PartsService struct{}

// IsKCESPartsFile 判断路径是否为受支持的 KCES 部件载荷
// IsKCESPartsFile reports whether a path is a supported KCES parts payload
func IsKCESPartsFile(path string) bool {
	return IsKCESMenuAssetsFile(path) || IsKCESMaterialAssetsFile(path) || IsKCESPriorityMaterialAssetsFile(path) || IsKCESModelFile(path) || IsKCESKCMenuFile(path) || IsKCESKCMatFile(path) || IsKCESKCModelFile(path)
}

// IsKCESPartsJSONFile 判断路径是否为受支持部件载荷的编辑 JSON
// IsKCESPartsJSONFile reports whether a path is editing JSON for a supported KCES parts payload
func IsKCESPartsJSONFile(path string) bool {
	return IsKCESMenuAssetsJSONFile(path) || IsKCESMaterialAssetsJSONFile(path) || IsKCESPriorityMaterialAssetsJSONFile(path) || IsKCESModelJSONFile(path) || IsKCESKCMenuJSONFile(path) || IsKCESKCMatJSONFile(path) || IsKCESKCModelJSONFile(path)
}

// ConvertPartsToJson 根据输入扩展名调用对应的独立部件 service
// ConvertPartsToJson dispatches to the independent parts service selected by the input extension
func (s *PartsService) ConvertPartsToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	switch strings.ToLower(filepath.Ext(inputPath)) {
	case menuAssetsExtension:
		return (&MenuAssetsService{}).ConvertMenuAssetsToJson(ctx, inputPath, outputPath, maxOutputBytes)
	case materialAssetsExtension:
		return (&MaterialAssetsService{}).ConvertMaterialAssetsToJson(ctx, inputPath, outputPath, maxOutputBytes)
	case priorityMaterialAssetsExtension:
		return (&PriorityMaterialAssetsService{}).ConvertPriorityMaterialAssetsToJson(ctx, inputPath, outputPath, maxOutputBytes)
	case modelExtension:
		return (&ModelService{}).ConvertModelToJson(ctx, inputPath, outputPath, maxOutputBytes)
	case serializationKCES.KCMenuExtension:
		return (&KCMenuService{}).ConvertKCMenuToJson(ctx, inputPath, outputPath, maxOutputBytes)
	case kcMatExtension:
		return (&KCMatService{}).ConvertKCMatToJson(ctx, inputPath, outputPath, maxOutputBytes)
	case kcModelExtension:
		return (&KCModelService{}).ConvertKCModelToJson(ctx, inputPath, outputPath, maxOutputBytes)
	default:
		return fmt.Errorf("unsupported KCES parts file type: %s", filepath.Ext(inputPath))
	}
}

// ConvertJsonToParts 根据编辑 JSON 或输出路径的扩展名调用对应的独立部件 service
// ConvertJsonToParts dispatches to the independent parts service selected by the editing JSON or output extension
func (s *PartsService) ConvertJsonToParts(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	extension := partsExtFromJSONPath(inputPath)
	if extension == "" {
		extension = strings.ToLower(filepath.Ext(outputPath))
	}
	switch extension {
	case menuAssetsExtension:
		return (&MenuAssetsService{}).ConvertJsonToMenuAssets(ctx, inputPath, outputPath, maxOutputBytes)
	case materialAssetsExtension:
		return (&MaterialAssetsService{}).ConvertJsonToMaterialAssets(ctx, inputPath, outputPath, maxOutputBytes)
	case priorityMaterialAssetsExtension:
		return (&PriorityMaterialAssetsService{}).ConvertJsonToPriorityMaterialAssets(ctx, inputPath, outputPath, maxOutputBytes)
	case modelExtension:
		return (&ModelService{}).ConvertJsonToModel(ctx, inputPath, outputPath, maxOutputBytes)
	case serializationKCES.KCMenuExtension:
		return (&KCMenuService{}).ConvertJsonToKCMenu(ctx, inputPath, outputPath, maxOutputBytes)
	case kcMatExtension:
		return (&KCMatService{}).ConvertJsonToKCMat(ctx, inputPath, outputPath, maxOutputBytes)
	case kcModelExtension:
		return (&KCModelService{}).ConvertJsonToKCModel(ctx, inputPath, outputPath, maxOutputBytes)
	default:
		return fmt.Errorf("unsupported KCES parts JSON type: %s", extension)
	}
}

// ReadPartsFile 根据路径扩展名调用对应的独立部件 service
// ReadPartsFile dispatches to the independent parts service selected by the path extension
func (s *PartsService) ReadPartsFile(path string) (any, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case menuAssetsExtension:
		return (&MenuAssetsService{}).ReadMenuAssetsFile(path)
	case materialAssetsExtension:
		return (&MaterialAssetsService{}).ReadMaterialAssetsFile(path)
	case priorityMaterialAssetsExtension:
		return (&PriorityMaterialAssetsService{}).ReadPriorityMaterialAssetsFile(path)
	case modelExtension:
		return (&ModelService{}).ReadModelFile(path)
	case serializationKCES.KCMenuExtension:
		return (&KCMenuService{}).ReadKCMenuFile(path)
	case kcMatExtension:
		return (&KCMatService{}).ReadKCMatFile(path)
	case kcModelExtension:
		return (&KCModelService{}).ReadKCModelFile(path)
	default:
		return nil, fmt.Errorf("unsupported KCES parts file type: %s", filepath.Ext(path))
	}
}

// WritePartsFile 根据目标扩展名调用对应的独立部件 service
// WritePartsFile dispatches to the independent parts service selected by the destination extension
func (s *PartsService) WritePartsFile(path string, value any) error {
	switch strings.ToLower(filepath.Ext(path)) {
	case menuAssetsExtension:
		assets, ok := value.(*serializationKCES.MenuAssets)
		if !ok && value != nil {
			return fmt.Errorf(".menuassets output requires *KCES.MenuAssets, got %T", value)
		}
		return (&MenuAssetsService{}).WriteMenuAssetsFile(path, assets)
	case materialAssetsExtension:
		assets, ok := value.(*serializationKCES.MaterialAssets)
		if !ok && value != nil {
			return fmt.Errorf(".materialassets output requires *KCES.MaterialAssets, got %T", value)
		}
		return (&MaterialAssetsService{}).WriteMaterialAssetsFile(path, assets)
	case priorityMaterialAssetsExtension:
		assets, ok := value.(*serializationKCES.PriorityMaterialAssets)
		if !ok && value != nil {
			return fmt.Errorf(".pmatassets output requires *KCES.PriorityMaterialAssets, got %T", value)
		}
		return (&PriorityMaterialAssetsService{}).WritePriorityMaterialAssetsFile(path, assets)
	case modelExtension:
		model, ok := value.(*serializationKCES.Model)
		if !ok && value != nil {
			return fmt.Errorf(".model output requires *KCES.Model, got %T", value)
		}
		return (&ModelService{}).WriteModelFile(path, model)
	case serializationKCES.KCMenuExtension:
		menu, ok := value.(*serializationKCES.Menu)
		if !ok && value != nil {
			return fmt.Errorf(".kcmenu output requires *KCES.Menu, got %T", value)
		}
		return (&KCMenuService{}).WriteKCMenuFile(path, menu)
	case kcMatExtension:
		material, ok := value.(*serializationKCES.Material)
		if !ok && value != nil {
			return fmt.Errorf(".kcmat output requires *KCES.Material, got %T", value)
		}
		return (&KCMatService{}).WriteKCMatFile(path, material)
	case kcModelExtension:
		model, ok := value.(*serializationKCES.Model)
		if !ok && value != nil {
			return fmt.Errorf(".kcmodel output requires *KCES.Model, got %T", value)
		}
		return (&KCModelService{}).WriteKCModelFile(path, model)
	default:
		return fmt.Errorf("unsupported KCES parts output type: %s", filepath.Ext(path))
	}
}

// encodePartsJSON 为文件探测与旧测试保留部件 JSON 编码分派
// encodePartsJSON preserves parts JSON encoding dispatch for file probing and legacy tests
func encodePartsJSON(extension string, data []byte) ([]byte, error) {
	switch strings.ToLower(extension) {
	case menuAssetsExtension:
		return encodeMenuAssetsJSON(data)
	case materialAssetsExtension:
		return encodeMaterialAssetsJSON(data)
	case priorityMaterialAssetsExtension:
		return encodePriorityMaterialAssetsJSON(data)
	case modelExtension:
		return encodeModelJSON(data)
	case serializationKCES.KCMenuExtension:
		return encodeKCMenuJSON(data)
	case kcMatExtension:
		return encodeKCMatJSON(data)
	case kcModelExtension:
		return encodeKCModelJSON(data)
	default:
		return nil, fmt.Errorf("unsupported KCES parts JSON type: %s", extension)
	}
}

// partsExtFromJSONPath 从双扩展名编辑 JSON 路径提取部件扩展名
// partsExtFromJSONPath extracts the parts extension from a double-extension editing JSON path
func partsExtFromJSONPath(path string) string {
	if !strings.HasSuffix(strings.ToLower(path), ".json") {
		return ""
	}
	base := strings.TrimSuffix(path, filepath.Ext(path))
	return strings.ToLower(filepath.Ext(base))
}
