package KCES

import (
	"fmt"
	"os"
	pathpkg "path"
	"path/filepath"
	"reflect"
	"strings"
)

// PackService 提供将纯资源目录打包为 KCES ABA 和 CT 的服务 / PackService packs a plain resource directory into KCES ABA and CT files
type PackService struct{}

// PackToAbaAndCt 扫描纯资源目录并在其父目录生成固定 Unity 2022.3.35f1 的 ABA 和 CT
// PackToAbaAndCt scans a plain resource directory and emits fixed Unity 2022.3.35f1 ABA and CT files in its parent directory
func (s *PackService) PackToAbaAndCt(dirPath string, outputBaseName string) error {
	if outputBaseName == "" {
		outputBaseName = filepath.Base(dirPath)
	}

	manifest := ModManifest{
		Name:        outputBaseName,
		CatalogType: "Parts",
		PackageType: "Plugin",
	}
	err := filepath.Walk(dirPath, func(filePath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		if shouldSkipPackInput(filePath, outputBaseName) {
			return nil
		}
		if isLinkOrReparse(info) {
			return fmt.Errorf("refusing symlink or reparse point input %q", filePath)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refusing non-regular input %q", filePath)
		}
		relPath, err := filepath.Rel(dirPath, filePath)
		if err != nil {
			return fmt.Errorf("get relative path for %q: %w", filePath, err)
		}
		relPath = filepath.ToSlash(relPath)
		name := inferAssetNameForPack(relPath)
		manifest.Assets = append(manifest.Assets, ModAsset{
			Name:            name,
			Path:            relPath,
			Kind:            inferKindForPack(name, relPath),
			preserveRawData: true,
		})
		return nil
	})
	if err != nil {
		return fmt.Errorf("scan directory failed: %w", err)
	}
	if len(manifest.Assets) == 0 {
		return fmt.Errorf("no resource files found in directory")
	}
	return packModManifest(manifest, dirPath, filepath.Dir(dirPath))
}

// isLinkOrReparse 判断文件信息是否表示符号链接或 Windows reparse point
// isLinkOrReparse reports whether file information represents a symbolic link or Windows reparse point
func isLinkOrReparse(info os.FileInfo) bool {
	if info == nil {
		return false
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return true
	}
	value := reflect.ValueOf(info.Sys())
	for value.IsValid() && value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return false
		}
		value = value.Elem()
	}
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return false
	}
	field := value.FieldByName("FileAttributes")
	if !field.IsValid() || !field.CanUint() {
		return false
	}
	const fileAttributeReparsePoint uint64 = 0x00000400
	return field.Uint()&fileAttributeReparsePoint != 0
}

// inferAssetNameForPack 从纯目录文件名恢复 Unity 资源短名称
// inferAssetNameForPack restores the short Unity resource name from a pure-directory file name
func inferAssetNameForPack(packPath string) string {
	name := filepath.Base(packPath)
	lower := strings.ToLower(name)
	if suffix, _, ok := rawUnityRootByteSuffixForPackName(lower); ok {
		return name[:len(name)-len(suffix)]
	}
	switch {
	case strings.HasSuffix(lower, ".tex.png"):
		return name[:len(name)-len(".png")]
	case strings.HasSuffix(lower, ".tex.jpg"):
		return name[:len(name)-len(".jpg")]
	case strings.HasSuffix(lower, ".tex.jpeg"):
		return name[:len(name)-len(".jpeg")]
	case strings.HasSuffix(lower, ".tex.bytes"):
		return name[:len(name)-len(".bytes")]
	case strings.HasSuffix(lower, ".texture2d.bytes"):
		return name[:len(name)-len(".texture2d.bytes")]
	case strings.HasSuffix(lower, ".sprite.bytes"):
		return name[:len(name)-len(".sprite.bytes")]
	case strings.HasSuffix(lower, ".mmesh.bytes"):
		return name[:len(name)-len(".bytes")]
	case strings.HasSuffix(lower, ".partsatlas.bytes"):
		return name[:len(name)-len(".bytes")]
	case strings.HasSuffix(lower, ".partsassets.bytes"):
		return name[:len(name)-len(".bytes")]
	case strings.HasSuffix(lower, ".anm.bytes"):
		return name[:len(name)-len(".bytes")]
	case strings.HasSuffix(lower, ".bytes") && isUnityRawObjectPackPath(packPath):
		return name[:len(name)-len(".bytes")]
	case strings.HasSuffix(lower, ".png"):
		return name[:len(name)-len(".png")] + ".tex"
	case strings.HasSuffix(lower, ".jpg"):
		return name[:len(name)-len(".jpg")] + ".tex"
	case strings.HasSuffix(lower, ".jpeg"):
		return name[:len(name)-len(".jpeg")] + ".tex"
	default:
		return name
	}
}

// inferKindForPack 根据纯目录路径和资源短名称确定 Unity 对象类型
// inferKindForPack determines the Unity object type from a pure-directory path and short resource name
func inferKindForPack(name string, packPath string) string {
	lowerPath := strings.ToLower(packPath)
	switch strings.ToLower(filepath.Ext(lowerPath)) {
	case ".ress", ".resource", ".resources":
		return "abaraw"
	}
	if _, kind, ok := rawUnityRootByteSuffixForPackName(strings.ToLower(filepath.Base(packPath))); ok {
		return kind
	}
	switch {
	case strings.HasSuffix(lowerPath, ".texture2d.bytes"):
		return "rawtexture2d"
	case strings.HasSuffix(lowerPath, ".texture.bytes"):
		return "rawtexture2d"
	case strings.HasSuffix(lowerPath, ".tex.bytes"):
		return "rawtexture2d"
	case strings.HasSuffix(lowerPath, ".sprite.bytes"):
		return "sprite"
	case strings.HasSuffix(lowerPath, ".mmesh.bytes"):
		return "mesh"
	case strings.HasSuffix(lowerPath, ".partsatlas.bytes"):
		return "spriteatlas"
	case strings.HasSuffix(lowerPath, ".partsassets.bytes"):
		return "spriteatlas"
	case strings.HasSuffix(lowerPath, ".anm.bytes"):
		return "animationclip"
	case strings.HasSuffix(lowerPath, ".monoscript.bytes"):
		return "monoscript"
	case strings.HasSuffix(lowerPath, ".monobehaviour.bytes"):
		return "monobehaviour"
	case strings.HasSuffix(lowerPath, ".material.bytes"):
		return "material"
	case strings.HasSuffix(lowerPath, ".shader.bytes"):
		return "shader"
	case strings.HasSuffix(lowerPath, ".audioclip.bytes"):
		return "audioclip"
	case strings.HasSuffix(lowerPath, ".font.bytes"):
		return "font"
	}
	if strings.HasSuffix(lowerPath, ".bytes") {
		if kind, ok := unityRawKindForPackPath(packPath); ok {
			return kind
		}
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".tex":
		return "texture2d"
	case ".sprite":
		return "sprite"
	case ".mmesh":
		return "mesh"
	case ".partsatlas", ".partsassets":
		return "spriteatlas"
	case ".anm":
		return "animationclip"
	}
	switch strings.ToLower(filepath.Ext(packPath)) {
	case ".mmesh", ".mesh":
		return "mesh"
	case ".partsatlas", ".partsassets":
		return "spriteatlas"
	case ".anm":
		return "animationclip"
	default:
		return "textasset"
	}
}

// rawUnityRootByteSuffixForPackName 识别可放在纯目录根部的原始 Unity 对象后缀
// rawUnityRootByteSuffixForPackName recognizes raw Unity object suffixes allowed at the pure-directory root
func rawUnityRootByteSuffixForPackName(lowerName string) (string, string, bool) {
	for _, candidate := range []struct {
		suffix string
		kind   string
	}{
		{suffix: ".texture.bytes", kind: "rawtexture2d"},
		{suffix: ".monoscript.bytes", kind: "monoscript"},
		{suffix: ".monobehaviour.bytes", kind: "monobehaviour"},
		{suffix: ".material.bytes", kind: "material"},
		{suffix: ".shader.bytes", kind: "shader"},
		{suffix: ".audioclip.bytes", kind: "audioclip"},
		{suffix: ".font.bytes", kind: "font"},
	} {
		if strings.HasSuffix(lowerName, candidate.suffix) {
			return candidate.suffix, candidate.kind, true
		}
	}
	return "", "", false
}

// isUnityRawObjectPackPath 判断路径是否位于可识别的原始 Unity 对象类型目录
// isUnityRawObjectPackPath reports whether a path belongs to a recognized raw Unity object type directory
func isUnityRawObjectPackPath(packPath string) bool {
	_, ok := unityRawKindForPackPath(packPath)
	return ok
}

// unityRawKindForPackPath 根据纯目录的一级类型目录返回原始 Unity 对象 kind
// unityRawKindForPackPath returns a raw Unity object kind from the pure directory's top-level type directory
func unityRawKindForPackPath(packPath string) (string, bool) {
	clean := filepath.ToSlash(packPath)
	directory := strings.ToLower(pathpkg.Base(pathpkg.Dir(clean)))
	switch directory {
	case "texture2d":
		return "rawtexture2d", true
	case "mesh":
		return "mesh", true
	case "sprite":
		return "sprite", true
	case "spriteatlas":
		return "spriteatlas", true
	case "animationclip":
		return "animationclip", true
	case "gameobject", "transform", "material", "meshrenderer", "meshfilter", "shader", "audioclip", "cubemap", "monobehaviour", "monoscript", "font":
		return directory, true
	default:
		if _, ok := unityRawClassIDForKind(directory); ok {
			return directory, true
		}
		return "", false
	}
}

// shouldSkipPackInput 排除控制文件、旧 sidecar、当前输出和可从原始对象派生的预览文件
// shouldSkipPackInput excludes control files, old sidecars, current outputs, and previews derived from raw objects
func shouldSkipPackInput(filePath string, outputBaseName string) bool {
	name := strings.ToLower(filepath.Base(filePath))
	if name == "manifest.json" || strings.HasSuffix(name, ".meta.json") || strings.HasSuffix(name, ".typetree.json") {
		return true
	}
	if outputBaseName == "" {
		return shouldSkipDerivedPackInput(filePath)
	}
	base := strings.ToLower(outputBaseName)
	return name == base+".aba" || name == base+".ct" || shouldSkipDerivedPackInput(filePath)
}

// shouldSkipDerivedPackInput 判断文件是否是同目录原始对象的派生预览
// shouldSkipDerivedPackInput reports whether a file is a preview derived from a raw object in the same directory
func shouldSkipDerivedPackInput(filePath string) bool {
	for _, rawPath := range derivedPackArtifactRawPaths(filePath) {
		if _, err := os.Stat(rawPath); err == nil {
			return true
		}
	}
	return false
}

// derivedPackArtifactRawPaths 返回可能生成当前预览文件的原始对象路径
// derivedPackArtifactRawPaths returns raw object paths that may have produced the current preview file
func derivedPackArtifactRawPaths(filePath string) []string {
	directory := filepath.Dir(filePath)
	name := filepath.Base(filePath)
	lower := strings.ToLower(name)
	parent := strings.ToLower(filepath.Base(directory))

	switch {
	case parent == "texture2d" && strings.HasSuffix(lower, ".tex.png"):
		return []string{filepath.Join(directory, name[:len(name)-len(".png")]+".bytes")}
	case parent == "texture2d" && strings.HasSuffix(lower, ".tex.jpg"):
		return []string{filepath.Join(directory, name[:len(name)-len(".jpg")]+".bytes")}
	case parent == "texture2d" && strings.HasSuffix(lower, ".tex.jpeg"):
		return []string{filepath.Join(directory, name[:len(name)-len(".jpeg")]+".bytes")}
	case parent == "texture2d" && strings.HasSuffix(lower, ".png"):
		base := name[:len(name)-len(".png")]
		return []string{
			filepath.Join(directory, base+".bytes"),
			filepath.Join(directory, base+".texture2d.bytes"),
		}
	case parent == "sprite" && strings.HasSuffix(lower, ".png"):
		return []string{filepath.Join(directory, name[:len(name)-len(".png")]+".sprite.bytes")}
	case strings.HasSuffix(lower, ".tex.png"):
		return []string{filepath.Join(directory, name[:len(name)-len(".png")]+".bytes")}
	case strings.HasSuffix(lower, ".tex.jpg"):
		return []string{filepath.Join(directory, name[:len(name)-len(".jpg")]+".bytes")}
	case strings.HasSuffix(lower, ".tex.jpeg"):
		return []string{filepath.Join(directory, name[:len(name)-len(".jpeg")]+".bytes")}
	default:
		return nil
	}
}
