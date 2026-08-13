package KCES

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	serializationCOM3D2 "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/COM3D2"
	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

// DataService 为旧调用方提供 KCES 与 COM3D2 共用格式的兼容分派入口 / DataService provides compatibility dispatch for formats shared by KCES and COM3D2
type DataService struct{}

// IsKCESDataFile 判断路径是否为 KCES 使用的共用数据文件
// IsKCESDataFile reports whether a path is a shared data file used by KCES
func IsKCESDataFile(path string) bool {
	return IsKCESPskFile(path)
}

// IsKCESDataJSONFile 判断路径是否为 KCES 共用数据文件的编辑 JSON
// IsKCESDataJSONFile reports whether a path is editing JSON for a shared KCES data file
func IsKCESDataJSONFile(path string) bool {
	return IsKCESPskJSONFile(path)
}

// ConvertDataToJson 将路径所指定的 KCES 共用数据文件转换为 JSON
// ConvertDataToJson converts the shared KCES data file selected by its path to JSON
func (s *DataService) ConvertDataToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	switch strings.ToLower(filepath.Ext(inputPath)) {
	case pskExtension:
		return (&PskService{}).ConvertPskToJson(ctx, inputPath, outputPath, maxOutputBytes)
	default:
		return fmt.Errorf("unsupported KCES shared data file type: %s", filepath.Ext(inputPath))
	}
}

// ConvertJsonToData 将 JSON 转换为路径所指定的 KCES 共用数据文件
// ConvertJsonToData converts JSON to the shared KCES data file selected by its path
func (s *DataService) ConvertJsonToData(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	extension := strings.ToLower(filepath.Ext(strings.TrimSuffix(inputPath, filepath.Ext(inputPath))))
	if extension == "" || extension == ".json" {
		extension = strings.ToLower(filepath.Ext(outputPath))
	}
	switch extension {
	case pskExtension:
		return (&PskService{}).ConvertJsonToPsk(ctx, inputPath, outputPath, maxOutputBytes)
	default:
		return fmt.Errorf("unsupported KCES shared data JSON type: %s", extension)
	}
}

// ConvertNeiToCSV 将 .nei 文件转换为 CSV
// ConvertNeiToCSV converts a .nei file to CSV
func (s *DataService) ConvertNeiToCSV(inputPath string, outputPath string) error {
	return (&NeiService{}).ConvertNeiToCSV(inputPath, outputPath)
}

// ConvertCSVToNei 将 CSV 转换为 .nei 文件
// ConvertCSVToNei converts CSV to a .nei file
func (s *DataService) ConvertCSVToNei(inputPath string, outputPath string) error {
	return (&NeiService{}).ConvertCSVToNei(inputPath, outputPath)
}

// WriteDataFile 根据目标扩展名编码并写入 KCES 共用数据文件
// WriteDataFile encodes and writes the shared KCES data file selected by the destination extension
func (s *DataService) WriteDataFile(path string, value any) error {
	switch strings.ToLower(filepath.Ext(path)) {
	case pskExtension:
		psk, ok := value.(*serializationCOM3D2.Psk)
		if !ok {
			return fmt.Errorf(".psk output requires *COM3D2.Psk, got %T", value)
		}
		return s.WritePskFile(path, psk)
	case neiExtension:
		nei, ok := value.(*serializationKCES.Nei)
		if !ok {
			return fmt.Errorf(".nei output requires *KCES.Nei, got %T", value)
		}
		return s.WriteNeiFile(path, nei)
	default:
		return fmt.Errorf("unsupported KCES shared data output type: %s", filepath.Ext(path))
	}
}
