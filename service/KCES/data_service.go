package KCES

import (
	"fmt"
	"path/filepath"
	"strings"

	COM3D2Service "github.com/MeidoPromotionAssociation/MeidoSerialization/service/COM3D2"
)

// IsKCESDataFile reports whether path is a KCES TextAsset payload whose
// concrete format is shared with existing COM3D2 serializers.
func IsKCESDataFile(path string) bool {
	if strings.HasSuffix(strings.ToLower(path), ".json") {
		return false
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".psk":
		return true
	default:
		return false
	}
}

// IsKCESDataJSONFile reports whether path is a JSON representation of a KCES
// shared data file.
func IsKCESDataJSONFile(path string) bool {
	if !strings.HasSuffix(strings.ToLower(path), ".json") {
		return false
	}
	base := strings.TrimSuffix(path, filepath.Ext(path))
	return IsKCESDataFile(base)
}

// DataService 通过委托现有 COM3D2 实现转换 KCES 共享数据文件 / DataService converts KCES shared data files by delegating to existing COM3D2 implementations
// 这样避免重复实现二进制格式逻辑 / This avoids duplicating binary format logic
type DataService struct{}

func (s *DataService) ConvertDataToJson(inputPath string, outputPath string) error {
	switch strings.ToLower(filepath.Ext(inputPath)) {
	case ".psk":
		service := &COM3D2Service.PskService{}
		return service.ConvertPskToJson(inputPath, outputPath)
	default:
		return fmt.Errorf("unsupported KCES shared data file type: %s", filepath.Ext(inputPath))
	}
}

func (s *DataService) ConvertJsonToData(inputPath string, outputPath string) error {
	ext := strings.ToLower(filepath.Ext(strings.TrimSuffix(inputPath, filepath.Ext(inputPath))))
	if ext == "" || ext == ".json" {
		ext = strings.ToLower(filepath.Ext(outputPath))
	}
	switch ext {
	case ".psk":
		service := &COM3D2Service.PskService{}
		return service.ConvertJsonToPsk(inputPath, outputPath)
	default:
		return fmt.Errorf("unsupported KCES shared data JSON type: %s", ext)
	}
}

func (s *DataService) ConvertNeiToCSV(inputPath string, outputPath string) error {
	service := &COM3D2Service.NeiService{}
	return service.NeiFileToCSVFile(inputPath, outputPath)
}

func (s *DataService) ConvertCSVToNei(inputPath string, outputPath string) error {
	service := &COM3D2Service.NeiService{}
	return service.CSVFileToNeiFile(inputPath, outputPath)
}
