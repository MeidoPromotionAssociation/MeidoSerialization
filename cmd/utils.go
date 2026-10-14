package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
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
// 全部文件处理完后返回汇总错误，使调用方能从退出码感知失败，而不是把失败当成成功
// processDirectory recursively processes filtered files and continues after printing individual file errors
// It returns an aggregate error once every file has been processed so the caller can observe failures through the exit code instead of treating them as success
func processDirectory(dirPath string, processor func(string) error, filter func(string) bool) error {
	var failures []string
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filter(path) {
			if processErr := processor(path); processErr != nil {
				fmt.Printf("Error processing file %s: %v\n", path, processErr)
				// 继续处理其余文件，最后统一报告失败
				// Continue processing other files and report the failures at the end
				failures = append(failures, path)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return summarizeProcessFailures(dirPath, failures)
}

// processDirectoryConcurrent 使用固定工作协程池并发处理目录中通过筛选的文件
// 与串行版本一样在结束后返回汇总错误
// processDirectoryConcurrent processes filtered directory files concurrently with a fixed worker pool
// Like the serial version it returns an aggregate error once the work is done
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
	var failureMutex sync.Mutex
	var failures []string

	// Start workers
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range pathsCh {
				if processErr := processor(p); processErr != nil {
					fmt.Printf("Error processing file %s: %v\n", p, processErr)
					failureMutex.Lock()
					failures = append(failures, p)
					failureMutex.Unlock()
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

	// 让报告顺序稳定，便于对比日志
	// Keep the report order stable so logs stay comparable
	sort.Strings(failures)
	return summarizeProcessFailures(dirPath, failures)
}

// summarizeProcessFailures 将目录处理中失败的文件汇总成一个错误
// 单文件错误已经逐个打印，这里只返回错误交给命令层输出，避免重复
// summarizeProcessFailures aggregates the files that failed during directory processing into one error
// Per-file errors are already printed one by one, so this only returns the error for the command layer to report and avoids duplication
func summarizeProcessFailures(dirPath string, failures []string) error {
	if len(failures) == 0 {
		return nil
	}
	return fmt.Errorf("failed to process %d file(s) under %s", len(failures), dirPath)
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

// isConvertibleNativeFile 判断路径是否为 convert2json 在目录模式下会转成编辑 JSON 的原生文件
// isConvertibleNativeFile reports whether a path names a native file that convert2json turns into editing JSON in directory mode
func isConvertibleNativeFile(path string) bool {
	return isModFile(path) || KCESService.IsKCESPartsFile(path) || KCESService.IsKCESPayloadFile(path) ||
		KCESService.IsKCESMiscFile(path) || KCESService.IsKCESDataFile(path) ||
		KCESService.IsKCESRawUnityBytesFile(path) || KCESService.IsKCESCtFile(path)
}

// isConvertibleEditingJSONFile 判断路径是否为 convert2mod 在目录模式下会转回原生格式的编辑 JSON
// isConvertibleEditingJSONFile reports whether a path names editing JSON that convert2mod turns back into a native file in directory mode
func isConvertibleEditingJSONFile(path string) bool {
	return isModJsonFile(path) || KCESService.IsKCESPartsJSONFile(path) || KCESService.IsKCESPayloadJSONFile(path) ||
		KCESService.IsKCESMiscJSONFile(path) || KCESService.IsKCESDataJSONFile(path) ||
		KCESService.IsKCESRawUnityBytesJSONFile(path) || KCESService.IsKCESCtJSONFile(path)
}

// isAutoConvertibleFile 判断路径是否为 convert 自动命令能处理的原生文件或通用格式
// 目录遍历用它做筛选，与 convertFile 的分派保持一致，避免目录模式漏掉单文件模式已经支持的类型
// 图片一项只做后缀预筛，因此目录模式比单文件模式窄，单文件由 isImageFile 兜底，目录可用 --type image 穷举
// isAutoConvertibleFile reports whether a path is a native file or common format the automatic convert command can handle
// Directory traversal filters with it and it stays aligned with convertFile's dispatch so directory mode cannot miss a type that single-file mode already supports
// The image entry is only a suffix pre-filter, so directory mode is narrower than single-file mode, where isImageFile backs it up and where --type image makes a directory exhaustive
func isAutoConvertibleFile(path string) bool {
	if isJsonFile(path) {
		return isModJsonFile(path) || isBytesJsonFile(path) || KCESService.IsKCESPartsJSONFile(path) ||
			KCESService.IsKCESPayloadJSONFile(path) || KCESService.IsKCESMiscJSONFile(path) ||
			KCESService.IsKCESDataJSONFile(path) || KCESService.IsKCESCtJSONFile(path) ||
			KCESService.IsKCESSystemDataJSONFile(path)
	}
	return isModFile(path) || KCESService.IsKCESPartsFile(path) || KCESService.IsKCESPayloadFile(path) ||
		KCESService.IsKCESMiscFile(path) || KCESService.IsKCESDataFile(path) ||
		isBytesFile(path) || isTexFile(path) || isNativeUnityImageFile(path) ||
		isImageCandidateFile(path) || isNeiFile(path) || isCsvFile(path) || isArcFile(path)
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
// 该判断会调用 ImageMagick 的 identify，目录遍历为每个文件启动一次外部进程代价过高，因此过滤器改用 isImageCandidateFile 预筛
// isImageFile reports whether a path uses an image type supported by the tools layer
// The check invokes ImageMagick's identify and starting an external process once per file is too costly for directory traversal, so filters pre-screen with isImageCandidateFile instead
func isImageFile(path string) bool {
	return tools.IsSupportedImageType(path) == nil
}

// isImageCandidateFile 仅按后缀判断路径是否可能是受支持的图像，用于目录遍历筛选
// 该判断不启动任何外部进程，因此可以安全地放进目录过滤器，而单文件路径上 isImageFile 会再兜一次底
// 后缀表覆盖常见位图与矢量图，但有意不含 PDF、PostScript、EPS、AI、WMF 这类文档格式：它们在目录扫描里会把文档变成 TEX，属于噪声
// 后缀表之外的真实图像格式仍可用单文件方式转换，或对目录显式加 --type image
// isImageCandidateFile reports whether a path may be a supported image based on its suffix alone, for directory traversal filtering
// It starts no external process so it is safe inside a directory filter, while isImageFile backs it up on the single-file path
// The suffix table covers common raster and vector images and deliberately omits document formats such as PDF, PostScript, EPS, AI, and WMF, because a directory scan would turn documents into TEX noise
// A real image format outside the table can still be converted as a single file, or by passing an explicit --type image for a directory
func isImageCandidateFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	// 常见位图与摄影格式
	// Common raster and photography formats
	case ".png", ".apng", ".jpg", ".jpeg", ".jpe", ".jfif", ".jif", ".gif", ".bmp", ".dib",
		".tga", ".targa", ".icb", ".vda", ".tif", ".tiff", ".webp", ".avif", ".heic", ".heif",
		".dds", ".psd", ".psb", ".pcx", ".xpm", ".xbm", ".xcf", ".ico", ".cur", ".qoi", ".jxl",
		".jp2", ".j2c", ".j2k", ".jpc", ".jng", ".exr", ".hdr", ".pict", ".sgi",
		".rgb", ".rgba", ".bgr", ".bgra", ".yuv", ".ycbcr", ".uyvy", ".cin", ".dpx", ".dcm",
		".fits", ".fit", ".ras", ".sun", ".viff", ".wbmp", ".wpg", ".xwd", ".xv", ".six", ".sixel",
		".tim", ".tm2", ".palm", ".pcd", ".rla", ".rle", ".sct", ".sfw", ".vips", ".ipl",
		".matte", ".miff", ".mpc", ".flif", ".farbfeld", ".x3f", ".avs", ".jbig", ".jbg",
		".pnm", ".ppm", ".pgm", ".pbm", ".pam", ".pfm", ".pgx", ".phm", ".bw", ".mono", ".gray", ".graya":
		return true
	// 矢量图，ImageMagick 会先栅格化再写出
	// Vector images, which ImageMagick rasterizes before writing
	case ".svg", ".svgz", ".msvg", ".rsvg":
		return true
	default:
		return false
	}
}

// isImageTypeFilter 判断全局类型参数是否显式要求只处理图片
// isImageTypeFilter reports whether the global type flag explicitly asks for images only
func isImageTypeFilter() bool {
	return strings.EqualFold(strings.TrimPrefix(strings.TrimSpace(fileType), "."), "image")
}

// isNativeUnityImageFile 判断路径是否为带内嵌 TypeTree 的 KCES 原生 Texture2D 或 Sprite 主文件
// isNativeUnityImageFile reports whether a path names a native KCES Texture2D or Sprite primary file with an embedded TypeTree
func isNativeUnityImageFile(path string) bool {
	return KCESService.IsKCESNativeTexture2DFile(path) || KCESService.IsKCESNativeSpriteFile(path)
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
// 图像支持情况依赖 ImageMagick，这里把失败原因直接带出来，便于区分格式不支持与 ImageMagick 缺失
// convertToTex converts a supported image to COM3D2 TEX using the selected payload compression policy
// Image support depends on ImageMagick and the failure reason is surfaced directly so an unsupported format can be told apart from a missing ImageMagick
func convertToTex(path string, compress bool, forcePng bool) error {
	if err := tools.IsSupportedImageType(path); err != nil {
		return fmt.Errorf("cannot convert %s as an image: %w", path, err)
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

// existingNativeTexture2DPaths 返回与图像同目录同基名的全部现有 KCES 原生 Texture2D 路径
// existingNativeTexture2DPaths returns every existing native KCES Texture2D sharing the image's directory and base name
func existingNativeTexture2DPaths(imagePath string) []string {
	base := trimLastExtension(imagePath)
	var found []string
	for _, extension := range []string{".tex", ".texture2d"} {
		candidate := base + extension
		if KCESService.IsKCESNativeTexture2DFile(candidate) {
			found = append(found, candidate)
		}
	}
	return found
}

// convertImageToMatchingTex 按同目录同基名的现有文件决定重建 KCES 原生 Texture2D 还是写出 COM3D2 TEX
// .tex 这一个后缀同时对应 COM3D2 的 TEX 和 KCES 的原生 Texture2D，从图像回写时无法只靠后缀判断，因此沿用即将被覆盖的那个文件的格式
// 只有 PNG 和 JPEG 能重建为原生 Texture2D，其余图像格式一律按 COM3D2 格式写出，重建原生 Texture2D 不需要 ImageMagick
// 同一基名可能同时存在 .tex 与 .texture2d 两个原生 Texture2D，此时把图像写进每一个，否则后遍历到的那个会把另一个的内容回退掉
// convertImageToMatchingTex picks between rebuilding a native KCES Texture2D and writing a COM3D2 TEX from the existing same-named sibling
// The single .tex suffix covers both the COM3D2 TEX and the native KCES Texture2D, so a write from an image cannot be decided by suffix alone and reuses the format of the file it is about to overwrite
// Only PNG and JPEG can be rebuilt as a native Texture2D and every other image format is written as COM3D2 TEX, while rebuilding a native Texture2D needs no ImageMagick
// One base name may hold both a .tex and a .texture2d native Texture2D, so the image is written into each of them, otherwise whichever is walked later rolls the other back
func convertImageToMatchingTex(path string) error {
	if isRGBA32DecodableImageFile(path) {
		targets := existingNativeTexture2DPaths(path)
		if len(targets) > 0 {
			for _, target := range targets {
				// 泛用 convert 只做同格式回写，因此探测并沿用被覆盖文件原有的 mip 层数
				// The generic convert command only writes back into the same format, so it probes and keeps the mip level count of the file it overwrites
				if err := convertImageToNativeTexture2DAt(path, target, KCESService.Texture2DMipModeAuto); err != nil {
					return err
				}
			}
			return nil
		}
	}
	return convertToTex(path, false, true)
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
	outputPath := trimLastExtension(path) + ".csv"
	if err := service.NeiFileToCSVFile(path, outputPath); err != nil {
		return fmt.Errorf("failed to convert %s to CSV: %w", path, err)
	}

	fmt.Printf("Converted %s to %s\n", path, outputPath)
	return nil
}

// convertToNei 将 UTF-8 CSV 表格转换为相邻的加密 NEI 文件，单元格按 encoding 编码
// encoding 为空表示自动，沿用即将被覆盖的同名 .nei 的编码
// convertToNei converts a UTF-8 CSV table to an adjacent encrypted NEI file, writing cells in encoding
// An empty encoding means automatic and reuses the encoding of the same-named .nei that is about to be overwritten
func convertToNei(path string, encoding COM3D2.NeiTextEncoding) error {
	if !isCsvFile(path) {
		return fmt.Errorf("not a CSV file: %s", path)
	}

	service := &COM3D2Service.NeiService{}
	outputPath := trimLastExtension(path) + ".nei"
	encoding = resolveNeiTextEncoding(outputPath, encoding)
	if err := service.CSVFileToNeiFileWithEncoding(path, outputPath, encoding); err != nil {
		return fmt.Errorf("failed to convert %s to NEI: %w", path, err)
	}

	fmt.Printf("Converted %s to %s (%s)\n", path, outputPath, encoding)
	return nil
}

// resolveNeiTextEncoding 解析写出 NEI 时要使用的单元格编码，空值表示自动
// COM3D2 与 KCES 的 .nei 共用容器结构，唯一差异是单元格文本的编码，而 CSV 里不携带来源信息，因此只能由外部决定
// 自动模式沿用即将被覆盖的同名 .nei 的编码，该文件不存在或无法读取时回落到 COM3D2 的 Shift-JIS
// 纯 ASCII 表格的两种编码字节完全相同，探测得到 Shift-JIS 不影响游戏读取
// resolveNeiTextEncoding resolves the cell encoding to write an NEI with, where an empty value means automatic
// The COM3D2 and KCES .nei share a container structure and differ only in the encoding of cell text, and a CSV carries no trace of its origin, so the choice has to come from outside the file
// Automatic mode reuses the encoding of the same-named .nei that is about to be overwritten and falls back to COM3D2's Shift-JIS when that file is absent or unreadable
// The two encodings produce identical bytes for ASCII, so detecting Shift-JIS on an ASCII-only table does not affect how the game reads it
func resolveNeiTextEncoding(outputPath string, encoding COM3D2.NeiTextEncoding) COM3D2.NeiTextEncoding {
	if encoding != "" {
		return encoding
	}
	if _, err := os.Stat(outputPath); err != nil {
		return COM3D2.NeiTextEncodingShiftJIS
	}

	existing, err := (&COM3D2Service.NeiService{}).ReadNeiFile(outputPath)
	if err != nil {
		fmt.Printf("Cannot detect the cell encoding of %s, writing Shift-JIS: %v\n", outputPath, err)
		return COM3D2.NeiTextEncodingShiftJIS
	}
	if existing.TextEncoding == "" {
		return COM3D2.NeiTextEncodingShiftJIS
	}

	fmt.Printf("Reusing the %s cell encoding of the existing %s\n", existing.TextEncoding, outputPath)
	return existing.TextEncoding
}

// parseNeiOutputFormat 将命令行的输出格式名称解析为 NEI 单元格编码，空值与 auto 表示自动
// 命令行按「这张表给哪个游戏读」来命名，因此取值是游戏名，转换到单元格编码是本层唯一的映射点
// parseNeiOutputFormat parses a command-line output format name into a NEI cell encoding, where an empty value and auto mean automatic
// The command line names the game that will read the table, so the accepted values are game names and this is the single place mapping them onto a cell encoding
func parseNeiOutputFormat(name string) (COM3D2.NeiTextEncoding, error) {
	switch strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(name, "_", ""), "-", "")) {
	case "", "auto":
		return "", nil
	case "com3d2":
		return COM3D2.NeiTextEncodingShiftJIS, nil
	case "kces":
		return COM3D2.NeiTextEncodingUTF8, nil
	default:
		return "", fmt.Errorf("unknown NEI output format %q, expected auto, com3d2 or kces", name)
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
// encoding 为空表示自动决定 CSV 转 NEI 时的单元格编码
// convertFile automatically selects the conversion direction between native and common formats from the file type
// An empty encoding lets the CSV-to-NEI direction decide the cell encoding automatically
func convertFile(path string, encoding COM3D2.NeiTextEncoding) error {
	if !fileTypeFilter(path) {
		fmt.Printf("Skip file %s because the file type does not match the filter\n", path)
		return nil // silent skip
	}

	// 编辑 JSON 转回原生格式
	// Editing JSON goes back to the native format
	if isJsonFile(path) {
		if !isAutoConvertibleFile(path) {
			return fmt.Errorf("unsupported file type for conversion: %s", path)
		}
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

	// TEX、原生 Texture2D 与 Sprite 转图片
	// TEX, native Texture2D, and Sprite go to an image
	if isTexFile(path) || isNativeUnityImageFile(path) {
		return convertToImage(path, "png")
	}

	// 图片按同名的原生文件决定回写格式
	// 后缀表只是目录遍历用的廉价预筛，且比 ImageMagick 宽：候选后缀但不是图像的文件按「不是图像」跳过，不算失败
	// 后缀表之外的格式由 ImageMagick 兜底，单文件路径必须能转换
	// An image is written back in the format of its same-named native sibling
	// The suffix table is only a cheap pre-filter for directory traversal and is wider than ImageMagick, so a candidate suffix that is not an image is skipped as not being an image rather than counted as a failure
	// Formats outside the table fall back to ImageMagick, which the single-file path has to convert
	if isImageCandidateFile(path) {
		if !isImageFile(path) {
			return nil
		}
		return convertImageToMatchingTex(path)
	}
	if isImageFile(path) {
		return convertImageToMatchingTex(path)
	}

	// If it's a NEI file, convert to CSV
	if isNeiFile(path) {
		return convertToCsv(path)
	}

	// If it's a CSV file, convert to NEI
	if isCsvFile(path) {
		return convertToNei(path, encoding)
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
