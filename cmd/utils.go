package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

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
// if filter returns true, the file will be processed
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

// processDirectoryConcurrent processes all files in a directory (recursively) based on the provided function
// if filter returns true, the file will be processed
// Now uses concurrent workers to speed up processing while preserving error handling semantics.
func processDirectoryConcurrent(dirPath string, processor func(string) error, filter func(string) bool) error {
	fmt.Printf("Concurrent processing folder %s\n", dirPath)

	var files []string
	// First collect all eligible files
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filter(path) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return nil
	}

	// Determine worker count
	workerCount := runtime.NumCPU()
	if workerCount < 1 {
		workerCount = 1
	}

	pathsCh := make(chan string, workerCount*2)
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range pathsCh {
				err = processor(p)
				if err != nil {
					fmt.Printf("Error processing file %s: %v\n", p, err)
					// continue other files
				}
			}
		}()
	}

	// Feed paths
	for _, p := range files {
		pathsCh <- p
	}
	close(pathsCh)

	wg.Wait()
	return nil
}

// isModFile checks if the file has a supported MOD file extension
// In addition to .tex and .nei
func isModFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".menu", ".mate", ".pmat", ".col", ".phy", ".psk", ".anm", ".model", ".preset":
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

	// Otherwise check if it's any supported MOD file
	// We need to check directly without using isModFile because it also considers fileType
	switch strings.ToLower(ext) {
	case ".menu", ".mate", ".pmat", ".col", ".phy", ".psk", ".anm", ".model", ".preset":
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
	case ".preset":
		service := &COM3D2Service.PresetService{}
		err = service.ConvertPresetToJson(path, outputPath)
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
	case ".preset":
		service := &COM3D2Service.PresetService{}
		err = service.ConvertJsonToPreset(path, outputPath)
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
	if !fileTypeFilter(path) {
		fmt.Printf("Skip file %s Because, filetype not match", path)
		return nil // silent skip
	}

	// If it's a JSON file, convert to MOD
	if isJsonFile(path) && isModJsonFile(path) {
		return convertToMod(path)
	}

	// If it's a MOD file, convert to JSON
	if isModFile(path) {
		return convertToJson(path)
	}

	// If it's a TEX file, convert to image
	if isTexFile(path) {
		return convertToImage(path, "png")
	}

	// If it's an image file, convert to TEX
	if isImageFile(path) {
		return convertToTex(path, false, true)
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

// fileTypeFilter filters files based on the fileType flag
// return true mean file should be processed
func fileTypeFilter(path string) bool {
	// Empty means no filtering
	ft := strings.ToLower(strings.TrimSpace(fileType))
	if ft == "" {
		return true
	}

	// Compatible with names starting with a dot, such as ".menu" or ".menu.json"
	ft = strings.TrimPrefix(ft, ".")

	// Parse whether it is in the "<type>.json" format
	wantJson := false
	if strings.HasSuffix(ft, ".json") {
		ft = strings.TrimSuffix(ft, ".json")
		wantJson = true
	}

	// Strict mode: identify types based on content
	if strictMode {
		commonService := &COM3D2Service.CommonService{}
		info, err := commonService.FileTypeDetermine(path, true)
		if err != nil {
			return false
		}
		// Type name matching (ignoring case)
		if !strings.EqualFold(info.FileType, ft) {
			return false
		}
		// If <type>.json is explicitly required, the storage format must be JSON
		if wantJson {
			return strings.EqualFold(info.StorageFormat, "json")
		}
		// non-<type>.json: only matches non-JSON (binary)
		return !strings.EqualFold(info.StorageFormat, "json")
	}

	// Non-strict mode: retain the original extension/detection-based logic
	if wantJson {
		// Only match files of the form .<type>.json
		if !isJsonFile(path) {
			return false
		}
		base := filepath.Base(path)
		base = strings.TrimSuffix(base, ".json")
		innerExt := strings.ToLower(strings.TrimPrefix(filepath.Ext(base), "."))
		return innerExt == ft
	}

	// General type matching
	switch ft {
	case "menu", "mate", "pmat", "col", "phy", "psk", "anm", "model", "preset":
		// Pure type: only matches binary .<type>, not .<type>.json
		if isJsonFile(path) {
			return false
		}
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
		return ext == ft
	case "tex":
		return isTexFile(path)
	case "nei":
		return isNeiFile(path)
	case "csv":
		return isCsvFile(path)
	case "image":
		return isImageFile(path)
	default:
		// Fallback: compare directly with the file extension; if it is .json, compare the internal extension
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
		if ext == ft {
			return true
		}
		if isJsonFile(path) {
			base := filepath.Base(path)
			base = strings.TrimSuffix(base, ".json")
			innerExt := strings.ToLower(strings.TrimPrefix(filepath.Ext(base), "."))
			return innerExt == ft
		}
		return false
	}
}
