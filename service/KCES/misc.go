package KCES

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

// MiscService 为旧调用方提供所有 KCES 杂项格式的兼容分派入口 / MiscService provides compatibility dispatch for all KCES miscellaneous formats
type MiscService struct{}

// IsKCESMiscFile 判断路径是否为受支持的 KCES 杂项文件
// IsKCESMiscFile reports whether a path is a supported KCES miscellaneous file
func IsKCESMiscFile(path string) bool {
	return IsKCESHitCheckFile(path) || IsKCESUndressDataFile(path) || IsKCESUndressPartsDataFile(path) || IsKCESNSONFile(path)
}

// IsKCESMiscJSONFile 判断路径是否为受支持杂项格式的编辑 JSON
// IsKCESMiscJSONFile reports whether a path is editing JSON for a supported miscellaneous format
func IsKCESMiscJSONFile(path string) bool {
	return IsKCESHitCheckJSONFile(path) || IsKCESUndressDataJSONFile(path) || IsKCESUndressPartsDataJSONFile(path) || IsKCESNSONJSONFile(path)
}

// ConvertMiscToJson 根据输入扩展名调用对应的独立杂项 service
// ConvertMiscToJson dispatches to the independent miscellaneous service selected by the input extension
func (s *MiscService) ConvertMiscToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	switch strings.ToLower(filepath.Ext(inputPath)) {
	case hitCheckExtension:
		return (&HitCheckService{}).ConvertHitCheckToJson(ctx, inputPath, outputPath, maxOutputBytes)
	case serializationKCES.KCESUndressDataExtension:
		return (&UndressDataService{}).ConvertUndressDataToJson(ctx, inputPath, outputPath, maxOutputBytes)
	case serializationKCES.KCESUndressPartsDataExtension:
		return (&UndressPartsDataService{}).ConvertUndressPartsDataToJson(ctx, inputPath, outputPath, maxOutputBytes)
	case serializationKCES.KCESNSONExtension:
		return (&NSONService{}).ConvertNSONToJson(ctx, inputPath, outputPath, maxOutputBytes)
	default:
		return fmt.Errorf("unsupported KCES misc file type: %s", filepath.Ext(inputPath))
	}
}

// ConvertJsonToMisc 根据编辑 JSON 或输出路径的扩展名调用对应的独立杂项 service
// ConvertJsonToMisc dispatches to the independent miscellaneous service selected by the editing JSON or output extension
func (s *MiscService) ConvertJsonToMisc(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	extension := miscExtFromJSONPath(inputPath)
	if extension == "" {
		extension = strings.ToLower(filepath.Ext(outputPath))
	}
	switch extension {
	case hitCheckExtension:
		return (&HitCheckService{}).ConvertJsonToHitCheck(ctx, inputPath, outputPath, maxOutputBytes)
	case serializationKCES.KCESUndressDataExtension:
		return (&UndressDataService{}).ConvertJsonToUndressData(ctx, inputPath, outputPath, maxOutputBytes)
	case serializationKCES.KCESUndressPartsDataExtension:
		return (&UndressPartsDataService{}).ConvertJsonToUndressPartsData(ctx, inputPath, outputPath, maxOutputBytes)
	case serializationKCES.KCESNSONExtension:
		return (&NSONService{}).ConvertJsonToNSON(ctx, inputPath, outputPath, maxOutputBytes)
	default:
		return fmt.Errorf("unsupported KCES misc JSON type: %s", extension)
	}
}

// ReadMiscFile 根据路径扩展名调用对应的独立杂项 service
// ReadMiscFile dispatches to the independent miscellaneous service selected by the path extension
func (s *MiscService) ReadMiscFile(path string) (any, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case hitCheckExtension:
		return (&HitCheckService{}).ReadHitCheckFile(path)
	case serializationKCES.KCESUndressDataExtension:
		return (&UndressDataService{}).ReadUndressDataFile(path)
	case serializationKCES.KCESUndressPartsDataExtension:
		return (&UndressPartsDataService{}).ReadUndressPartsDataFile(path)
	case serializationKCES.KCESNSONExtension:
		return (&NSONService{}).ReadNSONFile(path)
	default:
		return nil, fmt.Errorf("unsupported KCES misc file type: %s", filepath.Ext(path))
	}
}

// WriteMiscFile 根据目标扩展名调用对应的独立杂项 service
// WriteMiscFile dispatches to the independent miscellaneous service selected by the destination extension
func (s *MiscService) WriteMiscFile(path string, value any) error {
	extension := strings.ToLower(filepath.Ext(path))
	switch extension {
	case hitCheckExtension:
		hitCheck, ok := value.(*serializationKCES.HitCheck)
		if !ok {
			return fmt.Errorf(".hitcheck output requires *KCES.HitCheck, got %T", value)
		}
		return (&HitCheckService{}).WriteHitCheckFile(path, hitCheck)
	case serializationKCES.KCESUndressDataExtension:
		archive, ok := value.(*serializationKCES.UndressArchiveTarget)
		if !ok {
			return fmt.Errorf(".undressdat output requires *KCES.UndressArchiveTarget, got %T", value)
		}
		return (&UndressDataService{}).WriteUndressDataFile(path, archive)
	case serializationKCES.KCESUndressPartsDataExtension:
		precompute, ok := value.(*serializationKCES.UndressPrecomputeTarget)
		if !ok {
			return fmt.Errorf(".undresspdat output requires *KCES.UndressPrecomputeTarget, got %T", value)
		}
		return (&UndressPartsDataService{}).WriteUndressPartsDataFile(path, precompute)
	case serializationKCES.KCESNSONExtension:
		jsonText, ok := value.(json.RawMessage)
		if !ok {
			return fmt.Errorf("%s output requires a json.RawMessage document, got %T", extension, value)
		}
		return (&NSONService{}).WriteNSONFile(path, jsonText)
	default:
		return fmt.Errorf("unsupported KCES misc output type: %s", extension)
	}
}

// encodeMiscJSON 为文件探测与旧测试保留杂项 JSON 编码分派
// encodeMiscJSON preserves miscellaneous JSON encoding dispatch for file probing and legacy tests
func encodeMiscJSON(extension string, data []byte) ([]byte, error) {
	switch strings.ToLower(extension) {
	case hitCheckExtension:
		return encodeHitCheckJSON(data)
	case serializationKCES.KCESUndressDataExtension, serializationKCES.KCESUndressPartsDataExtension:
		return encodeKCESUnityJSONDocumentJSON(data, extension)
	case serializationKCES.KCESNSONExtension:
		return encodeKCESJSONTextJSON(data, extension)
	default:
		return nil, fmt.Errorf("unsupported KCES misc JSON type: %s", extension)
	}
}

// miscExtFromJSONPath 从双扩展名编辑 JSON 路径提取杂项扩展名
// miscExtFromJSONPath extracts the miscellaneous extension from a double-extension editing JSON path
func miscExtFromJSONPath(path string) string {
	if !strings.HasSuffix(strings.ToLower(path), ".json") {
		return ""
	}
	base := strings.TrimSuffix(path, filepath.Ext(path))
	return strings.ToLower(filepath.Ext(base))
}
