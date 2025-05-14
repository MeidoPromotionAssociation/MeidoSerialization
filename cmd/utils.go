package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	servicecom "github.com/MeidoPromotionAssociation/MeidoSerialization/service/COM3D2"
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
func isModFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".menu", ".mate", ".pmat", ".col", ".phy", ".psk", ".tex", ".anm", ".model":
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
	case ".menu", ".mate", ".pmat", ".col", ".phy", ".psk", ".tex", ".anm", ".model":
		return true
	default:
		return false
	}
}

// determineFileType determines the type of the file using the CommonService
func determineFileType(path string) error {
	commonService := &servicecom.CommonService{}
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

// convertToJson converts a MOD file to JSON
func convertToJson(path string) error {
	ext := strings.ToLower(filepath.Ext(path))
	outputPath := path + ".json"

	var err error
	switch ext {
	case ".menu":
		service := &servicecom.MenuService{}
		err = service.ConvertMenuToJson(path, outputPath)
	case ".mate":
		service := &servicecom.MateService{}
		err = service.ConvertMateToJson(path, outputPath)
	case ".pmat":
		service := &servicecom.PMatService{}
		err = service.ConvertPMatToJson(path, outputPath)
	case ".col":
		service := &servicecom.ColService{}
		err = service.ConvertColToJson(path, outputPath)
	case ".phy":
		service := &servicecom.PhyService{}
		err = service.ConvertPhyToJson(path, outputPath)
	case ".psk":
		service := &servicecom.PskService{}
		err = service.ConvertPskToJson(path, outputPath)
	case ".tex":
		service := &servicecom.TexService{}
		// For .tex files, we'll convert to PNG and also save JSON metadata
		tex, readErr := service.ReadTexFile(path)
		if readErr != nil {
			return fmt.Errorf("failed to read tex file: %w", readErr)
		}

		// Convert to image
		pngPath := strings.TrimSuffix(outputPath, ".json") + ".png"
		err = service.ConvertTexToImageAndWrite(tex, pngPath, true)
		if err != nil {
			return fmt.Errorf("failed to convert tex to image: %w", err)
		}

		// Also save the JSON metadata
		jsonData, err := json.Marshal(tex)
		if err != nil {
			return fmt.Errorf("failed to marshal tex data: %w", err)
		}

		f, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("unable to create tex.json file: %w", err)
		}
		defer func() {
			if closeErr := f.Close(); closeErr != nil && err == nil {
				err = fmt.Errorf("error closing output file: %w", closeErr)
			}
		}()

		_, err = f.Write(jsonData)
		if err != nil {
			return fmt.Errorf("failed to write tex.json file: %w", err)
		}
	case ".anm":
		service := &servicecom.AnmService{}
		err = service.ConvertAnmToJson(path, outputPath)
	case ".model":
		service := &servicecom.ModelService{}
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
		service := &servicecom.MenuService{}
		err = service.ConvertJsonToMenu(path, outputPath)
	case ".mate":
		service := &servicecom.MateService{}
		err = service.ConvertJsonToMate(path, outputPath)
	case ".pmat":
		service := &servicecom.PMatService{}
		err = service.ConvertJsonToPMat(path, outputPath)
	case ".col":
		service := &servicecom.ColService{}
		err = service.ConvertJsonToCol(path, outputPath)
	case ".phy":
		service := &servicecom.PhyService{}
		err = service.ConvertJsonToPhy(path, outputPath)
	case ".psk":
		service := &servicecom.PskService{}
		err = service.ConvertJsonToPsk(path, outputPath)
	case ".tex":
		// For .tex files, we need to handle differently
		service := &servicecom.TexService{}
		// We need a texture name, use the base name without extension
		texName := strings.TrimSuffix(baseName, ext)
		err = service.ConvertImageToTexAndWrite(path, texName, false, true, outputPath)
	case ".anm":
		service := &servicecom.AnmService{}
		err = service.ConvertJsonToAnm(path, outputPath)
	case ".model":
		service := &servicecom.ModelService{}
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

	return fmt.Errorf("unsupported file type for conversion: %s", path)
}
