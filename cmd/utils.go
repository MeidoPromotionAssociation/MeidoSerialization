package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	COM3D2Service "github.com/MeidoPromotionAssociation/MeidoSerialization/service/COM3D2"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/tools"
)

// isDirectory checks if the given path is a directory
func isDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// processFile processes a single file based on the provided function
func processFile(path string, processor func(string) error) error {
	return processor(path)
}

// processDirectory processes all files in a directory (recursively) based on the provided function
func processDirectory(dirPath string, processor func(string) error, filter func(string) bool) error {
	return filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filter(path) {
			if err := processor(path); err != nil {
				fmt.Printf("Error processing file %s: %v\n", path, err)
				// Continue processing other files even if one fails
				return nil
			}
		}
		return nil
	})
}

// isModFile checks if the file has a supported MOD file extension
// In addition to .tex and .nei
func isModFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".menu", ".mate", ".pmat", ".col", ".phy", ".psk", ".anm", ".model":
		// If fileType is specified, check if it matches
		if fileType != "" {
			// Remove the leading dot
			return strings.TrimPrefix(ext, ".") == fileType
		}
		return true
	default:
		return false
	}
}

// isJsonFile checks if the file has a .json extension
func isJsonFile(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".json")
}

// isModJsonFile checks if the file is a JSON file that corresponds to a MOD file
// In addition to .tex
func isModJsonFile(path string) bool {
	if !isJsonFile(path) {
		return false
	}

	// Check if it has a pattern like .menu.json, .mate.json, etc.
	baseName := filepath.Base(path)
	baseName = strings.TrimSuffix(baseName, ".json")
	ext := filepath.Ext(baseName)

	// If fileType is specified, check if it matches
	if fileType != "" {
		// Remove the leading dot
		return strings.TrimPrefix(ext, ".") == fileType
	}

	// Otherwise check if it's any supported MOD file
	// We need to check directly without using isModFile because it also considers fileType
	switch strings.ToLower(ext) {
	case ".menu", ".mate", ".pmat", ".col", ".phy", ".psk", ".anm", ".model":
		return true
	default:
		return false
	}
}

// isTexFile checks if the file has a .tex extension
func isTexFile(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".tex")
}

// isImageFile checks if the file has a supported image extension
func isImageFile(path string) bool {
	return tools.IsSupportedImageType(path) == nil
}

// convertToJson converts a MOD file to JSON
func convertToJson(path string) error {
	ext := strings.ToLower(filepath.Ext(path))
	outputPath := path + ".json"

	var err error
	switch ext {
	case ".menu":
		service := &COM3D2Service.MenuService{}
		err = service.ConvertMenuToJson(path, outputPath)
	case ".mate":
		service := &COM3D2Service.MateService{}
		err = service.ConvertMateToJson(path, outputPath)
	case ".pmat":
		service := &COM3D2Service.PMatService{}
		err = service.ConvertPMatToJson(path, outputPath)
	case ".col":
		service := &COM3D2Service.ColService{}
		err = service.ConvertColToJson(path, outputPath)
	case ".phy":
		service := &COM3D2Service.PhyService{}
		err = service.ConvertPhyToJson(path, outputPath)
	case ".psk":
		service := &COM3D2Service.PskService{}
		err = service.ConvertPskToJson(path, outputPath)
	case ".anm":
		service := &COM3D2Service.AnmService{}
		err = service.ConvertAnmToJson(path, outputPath)
	case ".model":
		service := &COM3D2Service.ModelService{}
		err = service.ConvertModelToJson(path, outputPath)
	default:
		return fmt.Errorf("unsupported file type: %s", ext)
	}

	if err != nil {
		return fmt.Errorf("failed to convert %s to JSON: %w", path, err)
	}

	fmt.Printf("Converted %s to %s\n", path, outputPath)
	return nil
}

// convertToMod converts a JSON file to a MOD file
func convertToMod(path string) error {
	if !strings.HasSuffix(strings.ToLower(path), ".json") {
		return fmt.Errorf("not a JSON file: %s", path)
	}

	baseName := filepath.Base(path)
	baseName = strings.TrimSuffix(baseName, ".json")
	ext := filepath.Ext(baseName)
	outputPath := strings.TrimSuffix(path, ".json")

	var err error
	switch strings.ToLower(ext) {
	case ".menu":
		service := &COM3D2Service.MenuService{}
		err = service.ConvertJsonToMenu(path, outputPath)
	case ".mate":
		service := &COM3D2Service.MateService{}
		err = service.ConvertJsonToMate(path, outputPath)
	case ".pmat":
		service := &COM3D2Service.PMatService{}
		err = service.ConvertJsonToPMat(path, outputPath)
	case ".col":
		service := &COM3D2Service.ColService{}
		err = service.ConvertJsonToCol(path, outputPath)
	case ".phy":
		service := &COM3D2Service.PhyService{}
		err = service.ConvertJsonToPhy(path, outputPath)
	case ".psk":
		service := &COM3D2Service.PskService{}
		err = service.ConvertJsonToPsk(path, outputPath)
	case ".anm":
		service := &COM3D2Service.AnmService{}
		err = service.ConvertJsonToAnm(path, outputPath)
	case ".model":
		service := &COM3D2Service.ModelService{}
		err = service.ConvertJsonToModel(path, outputPath)
	default:
		return fmt.Errorf("unsupported file type: %s", ext)
	}

	if err != nil {
		return fmt.Errorf("failed to convert %s to MOD: %w", path, err)
	}

	fmt.Printf("Converted %s to %s\n", path, outputPath)
	return nil
}

// convertToImage converts a TEX file to an image file
func convertToImage(path string, format string) error {
	if !isTexFile(path) {
		return fmt.Errorf("not a TEX file: %s", path)
	}

	if format == "" {
		format = "png"
	}

	service := &COM3D2Service.TexService{}

	outputPath := strings.TrimSuffix(path, ".tex") + "." + format
	err := service.ConvertAnyToAnyAndWrite(path, "", false, false, outputPath)
	if err != nil {
		return fmt.Errorf("failed to convert %s to image: %w", path, err)
	}

	fmt.Printf("Converted %s to %s\n", path, outputPath)
	return nil
}

// convertToTex converts an image file to TEX
func convertToTex(path string, compress bool, forcePng bool) error {
	if !isImageFile(path) {
		return fmt.Errorf("not a supported image file: %s", path)
	}

	if compress {
		forcePng = false
	}

	ext := filepath.Ext(path)
	outputPath := strings.TrimSuffix(path, ext) + ".tex"

	service := &COM3D2Service.TexService{}
	err := service.ConvertAnyToAnyAndWrite(path, "", compress, forcePng, outputPath)
	if err != nil {
		return fmt.Errorf("failed to convert %s to TEX: %w", path, err)
	}
	fmt.Printf("Converted %s to %s\n", path, outputPath)
	return nil
}

// determineFileType determines the type of the file using the CommonService
func determineFileType(path string) error {
	commonService := &COM3D2Service.CommonService{}
	fileInfo, err := commonService.FileTypeDetermine(path, strictMode)
	if err != nil {
		return fmt.Errorf("failed to determine file type: %w", err)
	}

	fmt.Printf("File: %s\n", path)
	fmt.Printf("  Type: %s\n", fileInfo.FileType)
	fmt.Printf("  Format: %s\n", fileInfo.StorageFormat)
	fmt.Printf("  Game: %s\n", fileInfo.Game)
	fmt.Printf("  Signature: %s\n", fileInfo.Signature)
	fmt.Printf("  Version: %d\n", fileInfo.Version)
	fmt.Printf("  Size: %d bytes\n", fileInfo.Size)

	return nil
}

// isNeiFile checks if the file has a .nei extension
func isNeiFile(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".nei")
}

// isCsvFile checks if the file has a .csv extension
func isCsvFile(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".csv")
}

// convertToCsv converts a NEI file to CSV
func convertToCsv(path string) error {
	if !isNeiFile(path) {
		return fmt.Errorf("not a NEI file: %s", path)
	}

	service := &COM3D2Service.NeiService{}
	outputPath := strings.TrimSuffix(path, ".nei") + ".csv"
	if err := service.NeiFileToCSVFile(path, outputPath); err != nil {
		return fmt.Errorf("failed to convert %s to CSV: %w", path, err)
	}

	fmt.Printf("Converted %s to %s\n", path, outputPath)
	return nil
}

// convertToNei converts a CSV file to NEI
func convertToNei(path string) error {
	if !isCsvFile(path) {
		return fmt.Errorf("not a CSV file: %s", path)
	}

	service := &COM3D2Service.NeiService{}
	outputPath := strings.TrimSuffix(path, ".csv") + ".nei"
	if err := service.CSVFileToNeiFile(path, outputPath); err != nil {
		return fmt.Errorf("failed to convert %s to NEI: %w", path, err)
	}

	fmt.Printf("Converted %s to %s\n", path, outputPath)
	return nil
}

// convertFile automatically determines the direction of conversion
func convertFile(path string) error {
	// If it's a JSON file, convert to MOD
	if isJsonFile(path) && isModJsonFile(path) {
		return convertToMod(path)
	}

	// If it's a MOD file, convert to JSON
	if isModFile(path) {
		return convertToJson(path)
	}

	// If it's a NEI file, convert to CSV
	if isNeiFile(path) {
		return convertToCsv(path)
	}

	// If it's a CSV file, convert to NEI
	if isCsvFile(path) {
		return convertToNei(path)
	}

	return fmt.Errorf("unsupported file type for conversion: %s", path)
}
