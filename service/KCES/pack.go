package KCES

import (
	"fmt"
	"os"
	pathpkg "path"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
)

// PackService 提供将纯资源目录打包为 KCES ABA 和 CT 的服务 / PackService packs a plain resource directory into KCES ABA and CT files
type PackService struct{}

// PackToAbaAndCt 扫描纯资源目录并在其父目录生成固定 Unity 2022.3.35f1 的 ABA 和 CT
// PackToAbaAndCt scans a plain resource directory and emits fixed Unity 2022.3.35f1 ABA and CT files in its parent directory
func (s *PackService) PackToAbaAndCt(dirPath string, outputBaseName string) error {
	return s.packToAbaAndCt(dirPath, outputBaseName, true)
}

// packToAbaAndCt 扫描纯资源目录，并允许包内测试选择是否压缩 ABA 数据块
// packToAbaAndCt scans a plain resource directory and lets in-package tests choose whether ABA data blocks are compressed
func (s *PackService) packToAbaAndCt(dirPath string, outputBaseName string, compressAba bool) error {
	if outputBaseName == "" {
		// 默认输出名剥掉 unpackAba 输出目录的 .aba_unpacked 后缀，因为游戏只从名为 <包名>.menuassets 的文件读取部件定义，包名带解包后缀会使 MOD 在游戏内不显示
		// The default output name strips the .aba_unpacked suffix of unpackAba output directories, because the game reads parts definitions only from a file named <bundle name>.menuassets and a bundle name carrying the unpack suffix makes the MOD invisible in game
		outputBaseName = filepath.Base(dirPath)
		outputBaseName = strings.TrimSuffix(outputBaseName, "_unpacked")
		outputBaseName = strings.TrimSuffix(outputBaseName, ".aba")
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
		kind := inferKindForPack(name, relPath)
		nativeObjectFile := isNativeUnityObjectPackPath(relPath, kind)
		// 独立 Unity 对象文件按文件头识别，因此 convert2texture2d 等工具的产物放在类型目录之外也不会被当作图像重新编码
		// Standalone Unity object files are recognized by their header, so output from tools such as convert2texture2d is not re-encoded as an image when stored outside a type directory
		if !nativeObjectFile && kind != "abaraw" {
			if header, headerErr := readNativeUnityObjectFileHeader(filePath); headerErr == nil {
				if detectedKind, ok := unityRawKindForClassID(header.ClassID); ok {
					kind = detectedKind
					nativeObjectFile = true
				}
			}
		}
		manifest.Assets = append(manifest.Assets, ModAsset{
			Name:             name,
			Path:             relPath,
			Kind:             kind,
			preserveRawData:  true,
			nativeObjectFile: nativeObjectFile,
		})
		return nil
	})
	if err != nil {
		return fmt.Errorf("scan directory failed: %w", err)
	}
	if len(manifest.Assets) == 0 {
		return fmt.Errorf("no resource files found in directory")
	}
	if warning := menuAssetsNameMismatchWarning(manifest); warning != "" {
		fmt.Fprintln(os.Stderr, "warning: "+warning)
	}
	return packModManifestWithOptions(manifest, dirPath, filepath.Dir(dirPath), modPackOptions{
		CompressAba: compressAba,
	})
}

// menuAssetsNameMismatchWarning 在包内存在部件定义但游戏无法识别时返回提示文本
// 游戏 BasePartsManager 仅从名为 <包名>.menuassets 的文件读取部件列表，且要求包名为小写，名称不匹配的 MOD 可以加载但不会在游戏内显示
// menuAssetsNameMismatchWarning returns a hint when the package carries parts definitions the game cannot recognize
// The game's BasePartsManager reads the parts list only from a file named <bundle name>.menuassets and requires a lowercase bundle name, so a mismatched MOD loads but never shows up in game
func menuAssetsNameMismatchWarning(manifest ModManifest) string {
	expected := manifest.Name + ".menuassets"
	hasMenuAssets := false
	for _, asset := range manifest.Assets {
		name := strings.ToLower(asset.Name)
		if !strings.HasSuffix(name, ".menuassets") {
			continue
		}
		hasMenuAssets = true
		if name == expected {
			return ""
		}
	}
	if !hasMenuAssets {
		return ""
	}
	return fmt.Sprintf("\n\nno %q found in the aba; the game reads parts definitions only from a lowercase file named exactly <aba name>.menuassets(There should be an xxx.menuassets in xxx.aba), so this MOD will not appear in game (rename the output with -o or rename the .menuassets file)\n\n在 aba 文件中未找到 %q；游戏仅从名为 <aba 名称>.menuassets 的小写文件名中读取部件定义（xxx.aba 中应该有一个 xxx.menuassets 文件），因此该 MOD 将不会出现在游戏中（请使用 -o 重命名输出文件或重命名 .menuassets 文件）\n", expected, expected)
}

// unityRawKindForClassID 将 Unity ClassID 映射回原始对象 kind，供按文件头识别独立 Unity 对象文件使用
// unityRawKindForClassID maps a Unity ClassID back to its raw object kind for header-based recognition of standalone Unity object files
func unityRawKindForClassID(classID int32) (string, bool) {
	switch classID {
	case aba.ClassIDTexture2D:
		return "rawtexture2d", true
	case aba.ClassIDMesh:
		return "mesh", true
	case aba.ClassIDSprite:
		return "sprite", true
	case aba.ClassIDSpriteAtlas:
		return "spriteatlas", true
	case aba.ClassIDAnimationClip:
		return "animationclip", true
	case aba.ClassIDGameObject:
		return "gameobject", true
	case aba.ClassIDTransform:
		return "transform", true
	case aba.ClassIDMaterial:
		return "material", true
	case aba.ClassIDMeshRenderer:
		return "meshrenderer", true
	case aba.ClassIDMeshFilter:
		return "meshfilter", true
	case aba.ClassIDShader:
		return "shader", true
	case aba.ClassIDAudioClip:
		return "audioclip", true
	case aba.ClassIDCubemap:
		return "cubemap", true
	case aba.ClassIDMonoBehaviour:
		return "monobehaviour", true
	case aba.ClassIDMonoScript:
		return "monoscript", true
	case aba.ClassIDFont:
		return "font", true
	default:
		if classID > 0 {
			return fmt.Sprintf("type_%d", classID), true
		}
		return "", false
	}
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
	case strings.HasSuffix(lower, ".texture2d"):
		return name[:len(name)-len(".texture2d")]
	case strings.HasSuffix(lower, ".sprite"):
		return name[:len(name)-len(".sprite")]
	case strings.HasSuffix(lower, ".material"):
		return name[:len(name)-len(".material")]
	case strings.HasSuffix(lower, ".audioclip"):
		return name[:len(name)-len(".audioclip")]
	case strings.HasSuffix(lower, ".monobehaviour"):
		return name[:len(name)-len(".monobehaviour")]
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
	if kind, ok := unityRawKindForPackPath(packPath); ok && isNativeUnityObjectPackPath(packPath, kind) {
		return kind
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

// isNativeUnityObjectPackPath 判断纯目录路径是否使用带内嵌 TypeTree 的独立 Unity 对象扩展名
// isNativeUnityObjectPackPath reports whether a pure-directory path uses a standalone Unity object extension with an embedded TypeTree
func isNativeUnityObjectPackPath(packPath string, kind string) bool {
	if _, ok := unityRawClassIDForKind(kind); !ok {
		return false
	}
	directoryKind, ok := unityRawKindForPackPath(packPath)
	if !ok || directoryKind != strings.ToLower(kind) {
		return false
	}
	lower := strings.ToLower(filepath.Base(packPath))
	switch directoryKind {
	case "rawtexture2d":
		return strings.HasSuffix(lower, ".tex") || strings.HasSuffix(lower, ".texture2d") || strings.HasSuffix(lower, ".bytes")
	case "mesh":
		return strings.HasSuffix(lower, ".mmesh") || strings.HasSuffix(lower, ".mesh") || strings.HasSuffix(lower, ".bytes")
	case "sprite":
		return strings.HasSuffix(lower, ".sprite") || strings.HasSuffix(lower, ".bytes")
	case "spriteatlas":
		return strings.HasSuffix(lower, ".partsatlas") || strings.HasSuffix(lower, ".partsassets") || strings.HasSuffix(lower, ".bytes")
	case "animationclip":
		return strings.HasSuffix(lower, ".anm") || strings.HasSuffix(lower, ".bytes")
	case "material":
		return strings.HasSuffix(lower, ".material") || strings.HasSuffix(lower, ".bytes")
	case "audioclip":
		return strings.HasSuffix(lower, ".audioclip") || strings.HasSuffix(lower, ".bytes")
	case "monobehaviour":
		return strings.HasSuffix(lower, ".monobehaviour") || strings.HasSuffix(lower, ".bytes")
	default:
		return strings.HasSuffix(lower, ".bytes")
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

	if _, typedDirectory := unityRawKindForPackPath(filePath); typedDirectory && strings.HasSuffix(lower, ".json") {
		return []string{filepath.Join(directory, name[:len(name)-len(".json")])}
	}
	switch {
	case parent == "texture2d" && hasAnySuffix(lower, ".png", ".dds"):
		base := strings.TrimSuffix(name, filepath.Ext(name))
		return []string{
			filepath.Join(directory, base+".tex"),
			filepath.Join(directory, base+".texture2d"),
		}
	case parent == "sprite" && strings.HasSuffix(lower, ".png"):
		return []string{filepath.Join(directory, strings.TrimSuffix(name, filepath.Ext(name))+".sprite")}
	case parent == "mesh" && hasAnySuffix(lower, ".glb", ".gltf"):
		base := strings.TrimSuffix(name, filepath.Ext(name))
		return []string{filepath.Join(directory, base+".mmesh"), filepath.Join(directory, base+".mesh")}
	case parent == "animationclip" && hasAnySuffix(lower, ".glb", ".gltf"):
		return []string{filepath.Join(directory, strings.TrimSuffix(name, filepath.Ext(name))+".anm")}
	case parent == "audioclip" && hasAnySuffix(lower, ".ogg", ".wav", ".fsb", ".audio"):
		return []string{filepath.Join(directory, strings.TrimSuffix(name, filepath.Ext(name))+".audioclip")}
	default:
		return nil
	}
}

// hasAnySuffix 判断字符串是否拥有任一忽略大小写后缀，调用方应先将值转成小写
// hasAnySuffix reports whether a string has any case-insensitive suffix after the caller has lowercased the value
func hasAnySuffix(lower string, suffixes ...string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(lower, strings.ToLower(suffix)) {
			return true
		}
	}
	return false
}
