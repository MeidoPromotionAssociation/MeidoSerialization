package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/application"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/COM3D2"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/COM3D2/arc"
	COM3D2Service "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/service/COM3D2"
	KCESService "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/service/KCES"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/tools"
)

// isDirectory 判断给定路径当前是否指向目录
// isDirectory reports whether the given path currently names a directory
func isDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// processFile 使用提供的处理函数处理单个路径
// processFile processes one path with the supplied processor
func processFile(path string, processor func(string) error) error {
	return processor(path)
}

// processDirectory 递归处理目录中通过筛选的文件并打印单文件错误后继续
// processDirectory recursively processes filtered files and continues after printing individual file errors
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

// processDirectoryConcurrent 使用固定工作协程池并发处理目录中通过筛选的文件
// processDirectoryConcurrent processes filtered directory files concurrently with a fixed worker pool
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
				if err := processor(p); err != nil {
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

// isModFile 判断路径是否匹配受支持的 COM3D2 或 KCES 原生 MOD 数据文件
// isModFile reports whether a path matches a supported native COM3D2 or KCES MOD data file
func isModFile(path string) bool {
	if KCESService.IsKCESBridgeSessionFile(path) {
		return true
	}
	if KCESService.IsKCESGP03BridgeFile(path) {
		return true
	}
	if KCESService.IsKCESExportNameMapFile(path) {
		return true
	}
	if KCESService.IsKCESSavedAttachFile(path) {
		return true
	}
	if KCESService.IsKCESSystemDataFile(path) {
		return true
	}
	if KCESService.IsKCESPathsFile(path) {
		return true
	}
	if KCESService.IsKCESMaidColliderFile(path) {
		return true
	}
	if KCESService.IsKCESPayloadFile(path) {
		return true
	}
	if KCESService.IsKCESMiscFile(path) {
		return true
	}
	if KCESService.IsKCESRawUnityBytesFile(path) {
		return true
	}
	if KCESService.IsKCESCtFile(path) {
		return true
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".menu", ".mate", ".mat", ".pmat", ".col", ".phy", ".psk", ".anm", ".model", ".preset", ".perset", ".menuassets", ".materialassets", ".pmatassets", ".kcmenu", ".kcmat", ".kcmodel":
		return true
	default:
		return false
	}
}

// isJsonFile 不区分大小写地判断路径是否以 JSON 后缀结尾
// isJsonFile reports case-insensitively whether a path ends with a JSON suffix
func isJsonFile(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".json")
}

// trimLastExtension 按实际大小写移除路径最后一个后缀以安全处理大写 JSON 名称
// trimLastExtension removes the final suffix using its actual casing to safely handle uppercase JSON names
func trimLastExtension(path string) string {
	ext := filepath.Ext(path)
	if ext == "" {
		return path
	}
	return path[:len(path)-len(ext)]
}

// canonicalLegacyFileType 将物理 mat 后缀映射为既有的逻辑 mate 类型名称
// canonicalLegacyFileType maps the physical mat suffix to the established logical mate type name
func canonicalLegacyFileType(fileType string) string {
	fileType = strings.ToLower(fileType)
	if fileType == "mat" {
		return "mate"
	}
	return fileType
}

// convertCOM3D2ToJSONByType 按规范 COM3D2 文件类型路由到对应编辑 JSON 转换器
// convertCOM3D2ToJSONByType routes a canonical COM3D2 file type to its editing JSON converter
func convertCOM3D2ToJSONByType(fileType string, inputPath string, outputPath string) (bool, error) {
	ctx := context.Background()
	switch canonicalLegacyFileType(strings.TrimPrefix(strings.ToLower(fileType), ".")) {
	case "menu":
		return true, (&COM3D2Service.MenuService{}).ConvertMenuToJson(ctx, inputPath, outputPath, application.DefaultMaxOutputBytes)
	case "mate":
		return true, (&COM3D2Service.MateService{}).ConvertMateToJson(ctx, inputPath, outputPath, application.DefaultMaxOutputBytes)
	case "pmat":
		return true, (&COM3D2Service.PMatService{}).ConvertPMatToJson(ctx, inputPath, outputPath, application.DefaultMaxOutputBytes)
	case "col":
		return true, (&COM3D2Service.ColService{}).ConvertColToJson(ctx, inputPath, outputPath, application.DefaultMaxOutputBytes)
	case "phy":
		return true, (&COM3D2Service.PhyService{}).ConvertPhyToJson(ctx, inputPath, outputPath, application.DefaultMaxOutputBytes)
	case "psk":
		return true, (&COM3D2Service.PskService{}).ConvertPskToJson(ctx, inputPath, outputPath, application.DefaultMaxOutputBytes)
	case "anm":
		return true, (&COM3D2Service.AnmService{}).ConvertAnmToJson(ctx, inputPath, outputPath, application.DefaultMaxOutputBytes)
	case "model":
		return true, (&COM3D2Service.ModelService{}).ConvertModelToJson(ctx, inputPath, outputPath, application.DefaultMaxOutputBytes)
	case "preset":
		return true, (&COM3D2Service.PresetService{}).ConvertPresetToJson(ctx, inputPath, outputPath, application.DefaultMaxOutputBytes)
	default:
		return false, nil
	}
}

// convertCOM3D2JSONToModByType 按规范 COM3D2 文件类型路由到对应原生格式转换器
// convertCOM3D2JSONToModByType routes a canonical COM3D2 file type to its native-format converter
func convertCOM3D2JSONToModByType(fileType string, inputPath string, outputPath string) (bool, error) {
	ctx := context.Background()
	switch canonicalLegacyFileType(strings.TrimPrefix(strings.ToLower(fileType), ".")) {
	case "menu":
		return true, (&COM3D2Service.MenuService{}).ConvertJsonToMenu(ctx, inputPath, outputPath, application.DefaultMaxOutputBytes)
	case "mate":
		return true, (&COM3D2Service.MateService{}).ConvertJsonToMate(ctx, inputPath, outputPath, application.DefaultMaxOutputBytes)
	case "pmat":
		return true, (&COM3D2Service.PMatService{}).ConvertJsonToPMat(ctx, inputPath, outputPath, application.DefaultMaxOutputBytes)
	case "col":
		return true, (&COM3D2Service.ColService{}).ConvertJsonToCol(ctx, inputPath, outputPath, application.DefaultMaxOutputBytes)
	case "phy":
		return true, (&COM3D2Service.PhyService{}).ConvertJsonToPhy(ctx, inputPath, outputPath, application.DefaultMaxOutputBytes)
	case "psk":
		return true, (&COM3D2Service.PskService{}).ConvertJsonToPsk(ctx, inputPath, outputPath, application.DefaultMaxOutputBytes)
	case "anm":
		return true, (&COM3D2Service.AnmService{}).ConvertJsonToAnm(ctx, inputPath, outputPath, application.DefaultMaxOutputBytes)
	case "model":
		return true, (&COM3D2Service.ModelService{}).ConvertJsonToModel(ctx, inputPath, outputPath, application.DefaultMaxOutputBytes)
	case "preset":
		return true, (&COM3D2Service.PresetService{}).ConvertJsonToPreset(ctx, inputPath, outputPath, application.DefaultMaxOutputBytes)
	default:
		return false, nil
	}
}

// isModJsonFile 判断路径是否匹配受支持原生 MOD 数据的编辑 JSON 文件名
// isModJsonFile reports whether a path matches an editing JSON filename for supported native MOD data
func isModJsonFile(path string) bool {
	if !isJsonFile(path) {
		return false
	}
	if KCESService.IsKCESBridgeSessionJSONFile(path) {
		return true
	}
	if KCESService.IsKCESGP03BridgeJSONFile(path) {
		return true
	}
	if KCESService.IsKCESExportNameMapJSONFile(path) || strings.HasSuffix(strings.ToLower(path), ".enm.json") {
		return true
	}
	if KCESService.IsKCESSavedAttachJSONFile(path) {
		return true
	}
	if KCESService.IsKCESSystemDataJSONFile(path) || strings.HasSuffix(strings.ToLower(path), "system.dat.json") {
		return true
	}
	if KCESService.IsKCESPathsJSONFile(path) || strings.HasSuffix(strings.ToLower(path), "paths.dat.json") {
		return true
	}
	maidColliderBase := trimLastExtension(path)
	if KCESService.IsKCESMaidColliderJSONFile(path) || KCESService.IsKCESMaidColliderFile(maidColliderBase) {
		return true
	}
	if KCESService.IsKCESPayloadJSONFile(path) {
		return true
	}
	if KCESService.IsKCESMiscJSONFile(path) {
		return true
	}
	rawUnityBase := trimLastExtension(path)
	if KCESService.IsKCESRawUnityBytesJSONFile(path) || KCESService.IsKCESRawUnityBytesFile(rawUnityBase) {
		return true
	}
	if KCESService.IsKCESCtJSONFile(path) || strings.HasSuffix(strings.ToLower(path), ".ct.json") {
		return true
	}

	// Check if it has a pattern like .menu.json, .mate.json, etc.
	baseName := filepath.Base(path)
	baseName = trimLastExtension(baseName)
	ext := filepath.Ext(baseName)

	// Otherwise check if it's any supported MOD file
	// We need to check directly without using isModFile because it also considers fileType
	switch strings.ToLower(ext) {
	case ".menu", ".mate", ".mat", ".pmat", ".col", ".phy", ".psk", ".anm", ".model", ".preset", ".perset", ".bytes", ".ct", ".menuassets", ".materialassets", ".pmatassets", ".kcmenu", ".kcmat", ".kcmodel":
		return true
	default:
		return false
	}
}

// isTexFile 不区分大小写地判断路径是否以 TEX 后缀结尾
// isTexFile reports case-insensitively whether a path ends with a TEX suffix
func isTexFile(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".tex")
}

// isImageFile 判断路径是否使用工具层支持的图像类型
// isImageFile reports whether a path uses an image type supported by the tools layer
func isImageFile(path string) bool {
	return tools.IsSupportedImageType(path) == nil
}

// isArcFile 不区分大小写地判断路径是否以 ARC 后缀结尾
// isArcFile reports case-insensitively whether a path ends with an ARC suffix
func isArcFile(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".arc")
}

// isAbaFile 不区分大小写地判断路径是否以 ABA 后缀结尾
// isAbaFile reports case-insensitively whether a path ends with an ABA suffix
func isAbaFile(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".aba")
}

// isCtFile 不区分大小写地判断路径是否以 CT 后缀结尾
// isCtFile reports case-insensitively whether a path ends with a CT suffix
func isCtFile(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".ct")
}

// convertToJson 检测原生 COM3D2 或 KCES 文件并将其转换为相邻编辑 JSON
// convertToJson detects a native COM3D2 or KCES file and converts it to adjacent editing JSON
func convertToJson(path string) error {
	ctx := context.Background()
	ext := strings.ToLower(filepath.Ext(path))
	outputPath := path + ".json"

	var err error
	legacyInfo, legacyMatched, legacyProbeErr := (&COM3D2Service.CommonService{}).TryFileTypeDetermine(path)
	if legacyProbeErr != nil {
		return fmt.Errorf("failed to probe %s as COM3D2: %w", path, legacyProbeErr)
	}
	if legacyMatched {
		if handled, legacyErr := convertCOM3D2ToJSONByType(legacyInfo.FileType, path, outputPath); handled {
			if legacyErr != nil {
				return fmt.Errorf("failed to convert %s to JSON: %w", path, legacyErr)
			}
			fmt.Printf("Converted %s to %s\n", path, outputPath)
			return nil
		}
	}
	if KCESService.IsKCESBridgeSessionFile(path) {
		service := &KCESService.BridgeSessionService{}
		err = service.ConvertBridgeSessionToJSON(ctx, path, outputPath, application.DefaultMaxOutputBytes)
		if err != nil {
			return fmt.Errorf("failed to convert %s to KCES bridge session JSON: %w", path, err)
		}
		fmt.Printf("Converted %s to %s\n", path, outputPath)
		return nil
	}
	if KCESService.IsKCESGP03BridgeFile(path) {
		service := &KCESService.GP03BridgeService{}
		err = service.ConvertBridgeToJSON(ctx, path, outputPath, application.DefaultMaxOutputBytes)
		if err != nil {
			return fmt.Errorf("failed to convert %s to KCES GP03 bridge JSON: %w", path, err)
		}
		fmt.Printf("Converted %s to %s\n", path, outputPath)
		return nil
	}
	if KCESService.IsKCESExportNameMapFile(path) {
		service := &KCESService.ExportNameMapService{}
		err = service.ConvertExportNameMapToJSON(ctx, path, outputPath, application.DefaultMaxOutputBytes)
		if err != nil {
			return fmt.Errorf("failed to convert %s to KCES export name map JSON: %w", path, err)
		}
		fmt.Printf("Converted %s to %s\n", path, outputPath)
		return nil
	}
	if KCESService.IsKCESSavedAttachFile(path) {
		service := &KCESService.SavedAttachService{}
		err = service.ConvertSavedAttachToJSON(ctx, path, outputPath, application.DefaultMaxOutputBytes)
		if err != nil {
			return fmt.Errorf("failed to convert %s to KCES saved-attach JSON: %w", path, err)
		}
		fmt.Printf("Converted %s to %s\n", path, outputPath)
		return nil
	}
	if KCESService.IsKCESSystemDataFile(path) {
		service := &KCESService.SystemDataService{}
		err = service.ConvertSystemDataToJSON(ctx, path, outputPath, application.DefaultMaxOutputBytes)
		if err != nil {
			return fmt.Errorf("failed to convert %s to KCES system.dat JSON: %w", path, err)
		}
		fmt.Printf("Converted %s to %s\n", path, outputPath)
		return nil
	}
	if KCESService.IsKCESPathsFile(path) {
		service := &KCESService.PathsService{}
		err = service.ConvertPathsToJSON(ctx, path, outputPath, application.DefaultMaxOutputBytes)
		if err != nil {
			return fmt.Errorf("failed to convert %s to paths.dat JSON: %w", path, err)
		}
		fmt.Printf("Converted %s to %s\n", path, outputPath)
		return nil
	}
	if KCESService.IsKCESMaidColliderFile(path) {
		service := &KCESService.MaidColliderService{}
		err = service.ConvertMaidColliderToJSON(ctx, path, outputPath, application.DefaultMaxOutputBytes)
		if err != nil {
			return fmt.Errorf("failed to convert %s to KCES maid collider JSON: %w", path, err)
		}
		fmt.Printf("Converted %s to %s\n", path, outputPath)
		return nil
	}
	if KCESService.IsKCESPayloadFile(path) {
		service := &KCESService.PayloadService{}
		err = service.ConvertPayloadToJson(ctx, path, outputPath, application.DefaultMaxOutputBytes)
		if err != nil {
			return fmt.Errorf("failed to convert %s to KCES payload JSON: %w", path, err)
		}
		fmt.Printf("Converted %s to %s\n", path, outputPath)
		return nil
	}
	if KCESService.IsKCESPartsFile(path) {
		service := &KCESService.PartsService{}
		err = service.ConvertPartsToJson(ctx, path, outputPath, application.DefaultMaxOutputBytes)
		if err != nil {
			return fmt.Errorf("failed to convert %s to KCES parts JSON: %w", path, err)
		}
		fmt.Printf("Converted %s to %s\n", path, outputPath)
		return nil
	}
	if KCESService.IsKCESMiscFile(path) {
		service := &KCESService.MiscService{}
		err = service.ConvertMiscToJson(ctx, path, outputPath, application.DefaultMaxOutputBytes)
		if err != nil {
			return fmt.Errorf("failed to convert %s to KCES misc JSON: %w", path, err)
		}
		fmt.Printf("Converted %s to %s\n", path, outputPath)
		printMissingUndressPairWarning(path)
		return nil
	}
	if KCESService.IsKCESRawUnityBytesFile(path) {
		service := &KCESService.RawUnityObjectService{}
		err = service.ConvertRawUnityObjectToJson(ctx, path, outputPath, application.DefaultMaxOutputBytes)
		if err != nil {
			return fmt.Errorf("failed to convert %s to KCES raw Unity JSON: %w", path, err)
		}
		fmt.Printf("Converted %s to %s\n", path, outputPath)
		return nil
	}
	if KCESService.IsKCESCtFile(path) {
		service := &KCESService.CtService{}
		err = service.ConvertCtToJson(ctx, path, outputPath, application.DefaultMaxOutputBytes)
		if err != nil {
			return fmt.Errorf("failed to convert %s to KCES ct JSON: %w", path, err)
		}
		fmt.Printf("Converted %s to %s\n", path, outputPath)
		return nil
	}
	if KCESService.IsKCESPresetFile(path) {
		service := &KCESService.PresetService{}
		err = service.ConvertPresetToJson(ctx, path, outputPath, application.DefaultMaxOutputBytes)
		if err != nil {
			return fmt.Errorf("failed to convert %s to KCES preset JSON: %w", path, err)
		}
		fmt.Printf("Converted %s to %s\n", path, outputPath)
		return nil
	}
	if KCESService.IsKCESDataFile(path) {
		service := &KCESService.DataService{}
		err = service.ConvertDataToJson(ctx, path, outputPath, application.DefaultMaxOutputBytes)
		if err != nil {
			return fmt.Errorf("failed to convert %s to KCES data JSON: %w", path, err)
		}
		fmt.Printf("Converted %s to %s\n", path, outputPath)
		return nil
	}
	if handled, legacyErr := convertCOM3D2ToJSONByType(ext, path, outputPath); handled {
		err = legacyErr
	} else if ext == ".bytes" {
		err = convertBytesToJson(path, outputPath)
	} else {
		return fmt.Errorf("unsupported file type: %s", ext)
	}

	if err != nil {
		return fmt.Errorf("failed to convert %s to JSON: %w", path, err)
	}

	fmt.Printf("Converted %s to %s\n", path, outputPath)
	return nil
}

// convertToMod 检测 COM3D2 或 KCES 编辑 JSON 并将其转换回相邻原生文件
// convertToMod detects COM3D2 or KCES editing JSON and converts it back to an adjacent native file
func convertToMod(path string) error {
	ctx := context.Background()
	if !strings.HasSuffix(strings.ToLower(path), ".json") {
		return fmt.Errorf("not a JSON file: %s", path)
	}

	baseName := filepath.Base(path)
	baseName = trimLastExtension(baseName)
	ext := filepath.Ext(baseName)
	outputPath := trimLastExtension(path)

	var err error
	legacyInfo, legacyMatched, legacyProbeErr := (&COM3D2Service.CommonService{}).TryFileTypeDetermine(path)
	if legacyProbeErr != nil {
		return fmt.Errorf("failed to probe %s as COM3D2 JSON: %w", path, legacyProbeErr)
	}
	if legacyMatched {
		if handled, legacyErr := convertCOM3D2JSONToModByType(legacyInfo.FileType, path, outputPath); handled {
			if legacyErr != nil {
				return fmt.Errorf("failed to convert %s to MOD: %w", path, legacyErr)
			}
			fmt.Printf("Converted %s to %s\n", path, outputPath)
			return nil
		}
	}
	if KCESService.IsKCESBridgeSessionJSONFile(path) {
		service := &KCESService.BridgeSessionService{}
		err = service.ConvertJSONToBridgeSession(ctx, path, outputPath, application.DefaultMaxOutputBytes)
		if err != nil {
			return fmt.Errorf("failed to convert %s to KCES bridge session file: %w", path, err)
		}
		fmt.Printf("Converted %s to %s\n", path, outputPath)
		return nil
	}
	if KCESService.IsKCESGP03BridgeJSONFile(path) {
		service := &KCESService.GP03BridgeService{}
		err = service.ConvertJSONToBridge(ctx, path, outputPath, application.DefaultMaxOutputBytes)
		if err != nil {
			return fmt.Errorf("failed to convert %s to KCES GP03 bridge file: %w", path, err)
		}
		fmt.Printf("Converted %s to %s\n", path, outputPath)
		return nil
	}
	if KCESService.IsKCESExportNameMapJSONFile(path) || strings.HasSuffix(strings.ToLower(path), ".enm.json") {
		service := &KCESService.ExportNameMapService{}
		err = service.ConvertJSONToExportNameMap(ctx, path, outputPath, application.DefaultMaxOutputBytes)
		if err != nil {
			return fmt.Errorf("failed to convert %s to KCES export name map: %w", path, err)
		}
		fmt.Printf("Converted %s to %s\n", path, outputPath)
		return nil
	}
	if KCESService.IsKCESSavedAttachJSONFile(path) {
		service := &KCESService.SavedAttachService{}
		err = service.ConvertJSONToSavedAttach(ctx, path, outputPath, application.DefaultMaxOutputBytes)
		if err != nil {
			return fmt.Errorf("failed to convert %s to KCES saved-attach file: %w", path, err)
		}
		fmt.Printf("Converted %s to %s\n", path, outputPath)
		return nil
	}
	if KCESService.IsKCESSystemDataJSONFile(path) || strings.HasSuffix(strings.ToLower(path), "system.dat.json") {
		service := &KCESService.SystemDataService{}
		err = service.ConvertJSONToSystemData(ctx, path, outputPath, application.DefaultMaxOutputBytes)
		if err != nil {
			return fmt.Errorf("failed to convert %s to KCES system.dat: %w", path, err)
		}
		fmt.Printf("Converted %s to %s\n", path, outputPath)
		return nil
	}
	if KCESService.IsKCESPathsJSONFile(path) || strings.HasSuffix(strings.ToLower(path), "paths.dat.json") {
		service := &KCESService.PathsService{}
		err = service.ConvertJSONToPaths(ctx, path, outputPath, application.DefaultMaxOutputBytes)
		if err != nil {
			return fmt.Errorf("failed to convert %s to paths.dat: %w", path, err)
		}
		fmt.Printf("Converted %s to %s\n", path, outputPath)
		return nil
	}
	maidColliderBase := trimLastExtension(path)
	if KCESService.IsKCESMaidColliderJSONFile(path) || KCESService.IsKCESMaidColliderFile(maidColliderBase) {
		service := &KCESService.MaidColliderService{}
		err = service.ConvertJSONToMaidCollider(ctx, path, outputPath, application.DefaultMaxOutputBytes)
		if err != nil {
			return fmt.Errorf("failed to convert %s to KCES maid collider: %w", path, err)
		}
		fmt.Printf("Converted %s to %s\n", path, outputPath)
		return nil
	}
	if KCESService.IsKCESPayloadJSONFile(path) {
		service := &KCESService.PayloadService{}
		err = service.ConvertJsonToPayload(ctx, path, outputPath, application.DefaultMaxOutputBytes)
		if err != nil {
			return fmt.Errorf("failed to convert %s to KCES payload: %w", path, err)
		}
		fmt.Printf("Converted %s to %s\n", path, outputPath)
		return nil
	}
	if KCESService.IsKCESPartsJSONFile(path) {
		service := &KCESService.PartsService{}
		err = service.ConvertJsonToParts(ctx, path, outputPath, application.DefaultMaxOutputBytes)
		if err != nil {
			return fmt.Errorf("failed to convert %s to KCES parts payload: %w", path, err)
		}
		fmt.Printf("Converted %s to %s\n", path, outputPath)
		return nil
	}
	if KCESService.IsKCESMiscJSONFile(path) {
		service := &KCESService.MiscService{}
		err = service.ConvertJsonToMisc(ctx, path, outputPath, application.DefaultMaxOutputBytes)
		if err != nil {
			return fmt.Errorf("failed to convert %s to KCES misc payload: %w", path, err)
		}
		fmt.Printf("Converted %s to %s\n", path, outputPath)
		printMissingUndressPairWarning(outputPath)
		return nil
	}
	rawUnityBase := trimLastExtension(path)
	if KCESService.IsKCESRawUnityBytesJSONFile(path) || KCESService.IsKCESRawUnityBytesFile(rawUnityBase) {
		service := &KCESService.RawUnityObjectService{}
		err = service.ConvertJsonToRawUnityObject(ctx, path, outputPath, application.DefaultMaxOutputBytes)
		if err != nil {
			return fmt.Errorf("failed to convert %s to KCES raw Unity payload: %w", path, err)
		}
		fmt.Printf("Converted %s to %s\n", path, outputPath)
		return nil
	}
	if KCESService.IsKCESCtJSONFile(path) || strings.HasSuffix(strings.ToLower(path), ".ct.json") {
		service := &KCESService.CtService{}
		err = service.ConvertJsonToCt(ctx, path, outputPath, application.DefaultMaxOutputBytes)
		if err != nil {
			return fmt.Errorf("failed to convert %s to KCES ct: %w", path, err)
		}
		fmt.Printf("Converted %s to %s\n", path, outputPath)
		return nil
	}
	if KCESService.IsKCESPresetJSONFile(path) {
		service := &KCESService.PresetService{}
		err = service.ConvertJsonToPreset(ctx, path, outputPath, application.DefaultMaxOutputBytes)
		if err != nil {
			return fmt.Errorf("failed to convert %s to KCES preset: %w", path, err)
		}
		fmt.Printf("Converted %s to %s\n", path, outputPath)
		return nil
	}
	if KCESService.IsKCESDataJSONFile(path) {
		service := &KCESService.DataService{}
		err = service.ConvertJsonToData(ctx, path, outputPath, application.DefaultMaxOutputBytes)
		if err != nil {
			return fmt.Errorf("failed to convert %s to KCES data: %w", path, err)
		}
		fmt.Printf("Converted %s to %s\n", path, outputPath)
		return nil
	}
	if handled, legacyErr := convertCOM3D2JSONToModByType(ext, path, outputPath); handled {
		err = legacyErr
	} else if strings.EqualFold(ext, ".bytes") {
		err = convertJsonToBytes(path, outputPath)
	} else {
		return fmt.Errorf("unsupported file type: %s", ext)
	}

	if err != nil {
		return fmt.Errorf("failed to convert %s to MOD: %w", path, err)
	}

	fmt.Printf("Converted %s to %s\n", path, outputPath)
	return nil
}

// convertToImage 将 COM3D2 TEX 或 KCES Texture2D 和 Sprite 主文件转换为图像
// convertToImage converts a COM3D2 TEX or KCES Texture2D and Sprite primary file to an image
func convertToImage(path string, format string) error {
	isNativeTexture := KCESService.IsKCESNativeTexture2DFile(path)
	isNativeSprite := KCESService.IsKCESNativeSpriteFile(path)
	if !isTexFile(path) && !isNativeTexture && !isNativeSprite {
		return fmt.Errorf("not a TEX, Texture2D, or Sprite file: %s", path)
	}

	if format == "" {
		format = "png"
	}

	outputPath := strings.TrimSuffix(path, filepath.Ext(path)) + "." + format
	if isNativeSprite {
		if !strings.EqualFold(format, "png") {
			return fmt.Errorf("native Sprite output format %q is unsupported; use png", format)
		}
		err := (&KCESService.NativeUnityMediaService{}).ConvertSpriteToPNG(context.Background(), path, outputPath, application.DefaultMaxOutputBytes)
		if err != nil {
			return fmt.Errorf("failed to convert %s to image: %w", path, err)
		}
		fmt.Printf("Converted %s to %s\n", path, outputPath)
		return nil
	}
	if isNativeTexture {
		err := (&KCESService.NativeUnityMediaService{}).ConvertTexture2DToImage(context.Background(), path, outputPath, format, application.DefaultMaxOutputBytes)
		if err != nil {
			return fmt.Errorf("failed to convert %s to image: %w", path, err)
		}
		fmt.Printf("Converted %s to %s\n", path, outputPath)
		return nil
	}

	service := &COM3D2Service.TexService{}
	err := service.ConvertAnyToAnyAndWrite(path, "", false, false, outputPath)
	if err != nil {
		return fmt.Errorf("failed to convert %s to image: %w", path, err)
	}

	fmt.Printf("Converted %s to %s\n", path, outputPath)
	return nil
}

// convertToTex 将受支持图像转换为 COM3D2 TEX 并应用所选载荷压缩策略
// convertToTex converts a supported image to COM3D2 TEX using the selected payload compression policy
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

// determineGameFileType 依次使用精确 COM3D2 探测、KCES 探测和旧式启发规则识别文件
// determineGameFileType identifies a file using exact COM3D2 probing, KCES probing, and legacy heuristics in order
func determineGameFileType(path string, strict bool) (COM3D2Service.FileInfo, error) {
	commonService := &COM3D2Service.CommonService{}
	fileInfo, matched, err := commonService.TryFileTypeDetermine(path)
	if matched {
		if err != nil {
			return fileInfo, err
		}
		return fileInfo, nil
	}
	if err != nil {
		return fileInfo, err
	}

	kcesService := &KCESService.FileTypeService{}
	fileInfo, matched, err = kcesService.TryFileTypeDetermine(path)
	if matched {
		if err != nil {
			return fileInfo, err
		}
		return fileInfo, nil
	}
	if err != nil {
		return fileInfo, err
	}

	return commonService.FileTypeDetermine(path, strict)
}

// determineFileType 检测 COM3D2 或 KCES 文件并以单次输出打印完整结果
// determineFileType detects a COM3D2 or KCES file and prints the complete result in one output operation
func determineFileType(path string) error {
	fileInfo, err := determineGameFileType(path, strictMode)
	if err != nil {
		return fmt.Errorf("failed to determine file type: %w", err)
	}

	// Directory determination runs concurrently. Emit one complete block with a
	// single write so fields from different files cannot interleave.
	fmt.Printf("File: %s\n  Type: %s\n  Format: %s\n  Game: %s\n  Signature: %s\n  Version: %d\n  Size: %d bytes\n",
		path, fileInfo.FileType, fileInfo.StorageFormat, fileInfo.Game,
		fileInfo.Signature, fileInfo.Version, fileInfo.Size)

	return nil
}

// isNeiFile 不区分大小写地判断路径是否以 NEI 后缀结尾
// isNeiFile reports case-insensitively whether a path ends with an NEI suffix
func isNeiFile(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".nei")
}

// isBytesFile 不区分大小写地判断路径是否以 bytes 后缀结尾
// isBytesFile reports case-insensitively whether a path ends with a bytes suffix
func isBytesFile(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".bytes")
}

// isBytesJsonFile 不区分大小写地判断路径是否以 bytes JSON 双后缀结尾
// isBytesJsonFile reports case-insensitively whether a path ends with the bytes JSON compound suffix
func isBytesJsonFile(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".bytes.json")
}

// isCsvFile 不区分大小写地判断路径是否以 CSV 后缀结尾
// isCsvFile reports case-insensitively whether a path ends with a CSV suffix
func isCsvFile(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".csv")
}

// convertToCsv 将加密 NEI 表格转换为相邻 UTF-8 CSV 文件，单元格编码按内容自动探测
// convertToCsv converts an encrypted NEI table to an adjacent UTF-8 CSV file, detecting the cell encoding from content
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

// convertToNei 将 UTF-8 CSV 表格转换为相邻的加密 NEI 文件，单元格按 encoding 编码
// convertToNei converts a UTF-8 CSV table to an adjacent encrypted NEI file, writing cells in encoding
func convertToNei(path string, encoding COM3D2.NeiTextEncoding) error {
	if !isCsvFile(path) {
		return fmt.Errorf("not a CSV file: %s", path)
	}

	service := &COM3D2Service.NeiService{}
	outputPath := strings.TrimSuffix(path, ".csv") + ".nei"
	if err := service.CSVFileToNeiFileWithEncoding(path, outputPath, encoding); err != nil {
		return fmt.Errorf("failed to convert %s to NEI: %w", path, err)
	}

	fmt.Printf("Converted %s to %s (%s)\n", path, outputPath, encoding)
	return nil
}

// parseNeiTextEncoding 将命令行编码名称解析为 NEI 单元格编码
// parseNeiTextEncoding parses a command-line encoding name into a NEI cell encoding
func parseNeiTextEncoding(name string) (COM3D2.NeiTextEncoding, error) {
	switch strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(name, "_", ""), "-", "")) {
	case "shiftjis", "sjis", "cp932", "com3d2":
		return COM3D2.NeiTextEncodingShiftJIS, nil
	case "utf8", "kces":
		return COM3D2.NeiTextEncodingUTF8, nil
	default:
		return "", fmt.Errorf("unknown NEI text encoding %q, expected shift-jis or utf-8", name)
	}
}

// unpackArc 将 ARC 解包到根据输入路径派生的默认目录
// unpackArc unpacks an ARC file into the default directory derived from its input path
func unpackArc(path string) error {
	return unpackArcTo(path, "")
}

// unpackArcTo 将 ARC 解包到显式目录或输入路径派生的默认目录
// unpackArcTo unpacks an ARC file into an explicit directory or the default derived directory
func unpackArcTo(path string, outDir string) error {
	service := &COM3D2Service.ArcService{}
	outputPath := outDir
	if outputPath == "" {
		outputPath = path + "_unpacked"
	}
	if err := service.UnpackArc(path, outputPath); err != nil {
		return fmt.Errorf("failed to unpack %s: %w", path, err)
	}

	fmt.Printf("Unpacked %s to %s\n", path, outputPath)
	return nil
}

// listArcFiles 读取 ARC 并打印其中保存的全部文件路径
// listArcFiles reads an ARC file and prints every file path stored in it
func listArcFiles(path string) error {
	service := &COM3D2Service.ArcService{}
	arcFs, err := service.ReadArc(path)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}

	files := service.GetFileList(arcFs)
	for _, f := range files {
		fmt.Println(f)
	}
	fmt.Printf("\nTotal: %d files\n", len(files))
	return nil
}

// extractArcByExt 保持 ARC 延迟读取句柄打开并提取匹配指定后缀的全部文件
// extractArcByExt keeps the lazy ARC handle open and extracts every file matching a specified suffix
func extractArcByExt(path string, ext string, outDir string) error {
	// Normalize extension: ensure leading dot, lowercase
	ext = strings.ToLower(strings.TrimSpace(ext))
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	service := &COM3D2Service.ArcService{}
	arcFs, closer, err := service.ReadArcLazy(path)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}
	defer closer.Close()

	allFiles := arcFs.GetFileList()
	var matched []string
	for _, fp := range allFiles {
		if strings.ToLower(filepath.Ext(fp)) == ext {
			matched = append(matched, fp)
		}
	}

	if len(matched) == 0 {
		fmt.Printf("No files with extension '%s' found in %s\n", ext, path)
		return nil
	}

	if err := service.ExtractFiles(arcFs, matched, outDir); err != nil {
		return fmt.Errorf("failed to extract files from %s: %w", path, err)
	}

	for _, fp := range matched {
		fmt.Printf("Extracted %s\n", fp)
	}
	fmt.Printf("\nExtracted %d files to %s\n", len(matched), outDir)
	return nil
}

// extractArcFile 按完整路径或唯一基本名称解析并提取单个 ARC 条目
// extractArcFile resolves and extracts one ARC entry by full path or unique base name
func extractArcFile(path string, filePath string, outDir string) error {
	service := &COM3D2Service.ArcService{}
	arcFs, closer, err := service.ReadArcLazy(path)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}
	defer closer.Close()

	// Resolve filePath: try exact match first, then fall back to filename matching
	resolvedPath, err := resolveArcFilePath(arcFs, filePath)
	if err != nil {
		return fmt.Errorf("%w (archive: %s)", err, path)
	}

	outPath := filepath.Join(outDir, resolvedPath)
	if err := service.ExtractFile(arcFs, resolvedPath, outPath); err != nil {
		return fmt.Errorf("failed to extract %s from %s: %w", resolvedPath, path, err)
	}

	fmt.Printf("Extracted %s to %s\n", resolvedPath, outPath)
	return nil
}

// resolveArcFilePath 优先按完整路径再按不区分大小写的唯一基本名称解析 ARC 条目
// resolveArcFilePath resolves an ARC entry first by full path and then by unique case-insensitive base name
func resolveArcFilePath(arcFs *arc.Arc, filePath string) (string, error) {
	allFiles := arcFs.GetFileList()

	// Try exact match first
	for _, fp := range allFiles {
		if fp == filePath {
			return filePath, nil
		}
	}

	// Fall back to filename matching (case-insensitive)
	target := strings.ToLower(filepath.Base(filePath))
	var matched []string
	for _, fp := range allFiles {
		if strings.ToLower(filepath.Base(fp)) == target {
			matched = append(matched, fp)
		}
	}

	switch len(matched) {
	case 0:
		return "", fmt.Errorf("file '%s' not found in archive", filePath)
	case 1:
		return matched[0], nil
	default:
		return "", fmt.Errorf("multiple files match '%s', please specify the full path:\n  %s",
			filePath, strings.Join(matched, "\n  "))
	}
}

// packArc 将目录树打包为 ARC 文件并打印生成路径
// packArc packs a directory tree into an ARC file and prints the generated path
func packArc(dirPath string, arcPath string) error {
	service := &COM3D2Service.ArcService{}
	if err := service.PackArc(dirPath, arcPath); err != nil {
		return fmt.Errorf("failed to pack %s: %w", dirPath, err)
	}

	fmt.Printf("Packed %s to %s\n", dirPath, arcPath)
	return nil
}

// convertBytesToJson 通过内容探测选择舞蹈时间轴或对象数据转换器
// convertBytesToJson selects a dance timeline or object-data converter through content probing
func convertBytesToJson(path string, outputPath string) error {
	service := &COM3D2Service.DanceService{}
	bytesType, err := service.SniffDanceBytesType(path)
	if err != nil {
		return fmt.Errorf("failed to sniff .bytes file type: %w", err)
	}

	switch bytesType {
	case COM3D2Service.DanceBytesTimeline:
		return service.ConvertTimelineDataToJson(context.Background(), path, outputPath, application.DefaultMaxOutputBytes)
	case COM3D2Service.DanceBytesObjectData:
		return service.ConvertDanceObjectDataToJson(context.Background(), path, outputPath, application.DefaultMaxOutputBytes)
	default:
		return fmt.Errorf("unrecognized .bytes file content")
	}
}

// convertJsonToBytes 根据时间轴标记字段将舞蹈编辑 JSON 转换回对应 bytes 文件
// convertJsonToBytes converts dance editing JSON back to the corresponding bytes file using timeline marker fields
func convertJsonToBytes(path string, outputPath string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot open json file: %w", err)
	}
	defer f.Close()

	headerBytes := make([]byte, 4096)
	n, _ := f.Read(headerBytes)
	headerBytes = headerBytes[:n]
	f.Close()

	service := &COM3D2Service.DanceService{}
	header := string(headerBytes)
	if strings.Contains(header, "\"TotalFrame\"") && strings.Contains(header, "\"FrameRate\"") {
		return service.ConvertJsonToTimelineData(context.Background(), path, outputPath, application.DefaultMaxOutputBytes)
	}
	return service.ConvertJsonToDanceObjectData(context.Background(), path, outputPath, application.DefaultMaxOutputBytes)
}

// convertFile 根据文件类型自动选择原生与通用格式之间的转换方向
// convertFile automatically selects the conversion direction between native and common formats from the file type
func convertFile(path string) error {
	if !fileTypeFilter(path) {
		fmt.Printf("Skip file %s Because, filetype not match", path)
		return nil // silent skip
	}

	// If it's a JSON file, convert to MOD
	if isJsonFile(path) && (isModJsonFile(path) || isBytesJsonFile(path) || KCESService.IsKCESPartsJSONFile(path) || KCESService.IsKCESPayloadJSONFile(path) || KCESService.IsKCESMiscJSONFile(path) || KCESService.IsKCESDataJSONFile(path) || KCESService.IsKCESCtJSONFile(path) || KCESService.IsKCESSystemDataJSONFile(path)) {
		return convertToMod(path)
	}

	// If it's a MOD file, convert to JSON
	if isModFile(path) || KCESService.IsKCESPartsFile(path) || KCESService.IsKCESPayloadFile(path) || KCESService.IsKCESMiscFile(path) || KCESService.IsKCESDataFile(path) || KCESService.IsKCESCtFile(path) || KCESService.IsKCESSystemDataFile(path) {
		return convertToJson(path)
	}

	// If it's a .bytes file, convert to JSON
	if isBytesFile(path) {
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
		return convertToNei(path, COM3D2.NeiTextEncodingShiftJIS)
	}

	// If it's an ARC file, unpack it
	if isArcFile(path) {
		return unpackArc(path)
	}

	return fmt.Errorf("unsupported file type for conversion: %s", path)
}

// fileTypeFilter 根据全局类型参数和严格模式判断路径是否应被处理
// fileTypeFilter reports whether a path should be processed according to the global type flag and strict mode
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
	ft = canonicalLegacyFileType(ft)

	// Strict mode: identify types based on content
	if strictMode {
		info, err := determineGameFileType(path, true)
		if err != nil {
			return false
		}
		requestedType := ft
		if requestedType == "perset" {
			requestedType = "preset"
		}
		// Type name matching (ignoring case)
		if !strings.EqualFold(info.FileType, requestedType) {
			return false
		}
		// The .json selector describes the editable filename form. Some native
		// KCES TextAssets (notably .undressdat/.undresspdat) are themselves JSON,
		// so StorageFormat alone cannot distinguish source from editing envelope.
		if wantJson {
			return isJsonFile(path) && strings.EqualFold(info.StorageFormat, "json")
		}
		return !isJsonFile(path)
	}

	// Non-strict mode: retain the original extension/detection-based logic
	if wantJson {
		// Only match files of the form .<type>.json
		if !isJsonFile(path) {
			return false
		}
		base := filepath.Base(path)
		base = trimLastExtension(base)
		if ft == "bridge_session" {
			return KCESService.IsKCESBridgeSessionJSONFile(path)
		}
		if ft == "system" {
			return strings.EqualFold(base, "system.dat")
		}
		innerExt := canonicalLegacyFileType(strings.TrimPrefix(filepath.Ext(base), "."))
		return innerExt == ft
	}

	// General type matching
	switch ft {
	case "menu", "mate", "pmat", "col", "phy", "psk", "anm", "model", "preset", "perset", "ct", "aba", "asset_scene", "system", "virtualdirectory", "bridge_session", "paths", "enm", "sad", "brd", "maid_collider", "menuassets", "materialassets", "pmatassets", "dbconf", "dbcol", "db2conf", "dsbconf", "dsb2conf", "dslconf", "dsl2conf", "dslcol", "ikcol", "limbcol", "ikcol.bytes", "hitcheck", "undressdat", "undresspdat", "nson":
		// Pure type: only matches binary .<type>, not .<type>.json
		if isJsonFile(path) {
			return false
		}
		if ft == "maid_collider" {
			return KCESService.IsKCESMaidColliderFile(path)
		}
		if ft == "bridge_session" {
			return KCESService.IsKCESBridgeSessionFile(path)
		}
		if ft == "system" {
			return strings.EqualFold(filepath.Base(path), "system.dat")
		}
		if ft == "paths" {
			return KCESService.IsKCESPathsFile(path)
		}
		if ft == "virtualdirectory" {
			return strings.EqualFold(filepath.Ext(path), ".dat")
		}
		if ft == "ikcol.bytes" {
			return strings.HasSuffix(strings.ToLower(path), ".ikcol.bytes")
		}
		if ft == "perset" {
			return strings.EqualFold(filepath.Ext(path), ".perset")
		}
		ext := canonicalLegacyFileType(strings.TrimPrefix(filepath.Ext(path), "."))
		return ext == ft
	case "tex":
		return isTexFile(path)
	case "nei":
		return isNeiFile(path)
	case "csv":
		return isCsvFile(path)
	case "image":
		return isImageFile(path)
	case "arc":
		return isArcFile(path)
	case "bytes":
		if isJsonFile(path) {
			return false
		}
		return isBytesFile(path)
	default:
		// Fallback: compare directly with the file extension; if it is .json, compare the internal extension
		ext := canonicalLegacyFileType(strings.TrimPrefix(filepath.Ext(path), "."))
		if ext == ft {
			return true
		}
		if isJsonFile(path) {
			base := filepath.Base(path)
			base = trimLastExtension(base)
			innerExt := canonicalLegacyFileType(strings.TrimPrefix(filepath.Ext(base), "."))
			return innerExt == ft
		}
		return false
	}
}

// printMissingUndressPairWarning 在 .undressdat 或 .undresspdat 缺少配对文件时向 stderr 打印一次提示
// 两个文件缺一个游戏就完全不加载脱衣设置，因此这属于转换成功但结果不可用的情况，只提示不报错
// printMissingUndressPairWarning prints one hint to stderr when a .undressdat or .undresspdat has no paired file
// The game loads no undress setup at all when either file is missing, so this is a successful conversion with an unusable result and stays a hint rather than an error
func printMissingUndressPairWarning(nativePath string) {
	if warning := KCESService.MissingUndressPairWarning(nativePath); warning != "" {
		fmt.Fprintln(os.Stderr, "warning: "+warning)
	}
}
