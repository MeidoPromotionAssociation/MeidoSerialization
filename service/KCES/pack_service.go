package KCES

import (
	"fmt"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
)

// PackService 提供将目录打包为 KCES 格式 .aba + .ct 的服务 / PackService provides services for packing directories into KCES .aba + .ct format
type PackService struct{}

// PackToAbaAndCt 将指定目录打包为可由 KCES catalog 发现的 .aba + .ct。
// 目录内文件会作为 AssetBundle 对象写入 .aba；.ct 只保存 catalog 和
// ExtensionNameList。PNG/JPEG 会按资源名推断为 Texture2D，.tex.bytes 会透传为
// Texture2D，.sprite.bytes 会透传为 Sprite，.mmesh 透传为 Mesh，
// .partsatlas/.partsassets 透传为 SpriteAtlas，其他文件默认写为 TextAsset。
func (s *PackService) PackToAbaAndCt(dirPath string, outputBaseName string) error {
	if outputBaseName == "" {
		outputBaseName = filepath.Base(dirPath)
	}

	outputDir := filepath.Dir(dirPath)
	manifest := ModManifest{
		Name:        outputBaseName,
		CatalogType: "Parts",
		PackageType: "Plugin",
		Assets:      []ModAsset{},
	}

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if shouldSkipPackInput(path, outputBaseName) {
			return nil
		}
		relPath, relErr := filepath.Rel(dirPath, path)
		if relErr != nil {
			return fmt.Errorf("get relative path for %q: %w", path, relErr)
		}
		relPath = filepath.ToSlash(relPath)
		name := inferAssetNameForPack(relPath)
		loadName := readAssetMeta(path).LoadName
		manifest.Assets = append(manifest.Assets, ModAsset{
			Name:     name,
			LoadName: loadName,
			Path:     relPath,
			Kind:     inferKindForPack(name, relPath),
		})
		return nil
	})
	if err != nil {
		return fmt.Errorf("scan directory failed: %w", err)
	}

	if len(manifest.Assets) == 0 {
		return fmt.Errorf("no files found in directory")
	}
	return packModManifest(manifest, dirPath, outputDir)
}

// RepackAba 将已解压的 .aba 目录重新打包为 .aba 文件
func (s *PackService) RepackAba(dirPath string, outPath string) error {
	var entries []aba.BundleFileEntry

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		relPath, _ := filepath.Rel(dirPath, path)
		relPath = filepath.ToSlash(relPath)
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read file %q failed: %w", relPath, err)
		}
		entries = append(entries, aba.BundleFileEntry{
			Name:         relPath,
			Data:         data,
			IsSerialized: isSerializedFile(relPath),
		})
		return nil
	})
	if err != nil {
		return fmt.Errorf("scan directory failed: %w", err)
	}

	if len(entries) == 0 {
		return fmt.Errorf("no files found in directory")
	}

	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create output file failed: %w", err)
	}
	defer f.Close()

	opts := &aba.BundleWriteOptions{Compress: true}
	return aba.WriteBundle(f, entries, opts)
}

// isSerializedFile 判断文件是否为 Unity 序列化文件（AssetsFile）
func isSerializedFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	// .resS 和 .resource 不是序列化文件
	if ext == ".ress" || ext == ".resource" || ext == ".resources" {
		return false
	}
	// CAB- 开头的文件通常是序列化文件
	base := filepath.Base(name)
	if strings.HasPrefix(base, "CAB-") && ext == "" {
		return true
	}
	// 没有扩展名或 .assets 扩展名的通常是序列化文件
	return ext == "" || ext == ".assets"
}

func inferAssetNameForPack(path string) string {
	name := filepath.Base(path)
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
	case strings.HasSuffix(lower, ".bytes") && isUnityRawObjectPackPath(path):
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

func inferKindForPack(name string, path string) string {
	lowerPath := strings.ToLower(path)
	if _, kind, ok := rawUnityRootByteSuffixForPackName(strings.ToLower(filepath.Base(path))); ok {
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
		if kind, ok := unityRawKindForPackPath(path); ok {
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
	switch strings.ToLower(filepath.Ext(path)) {
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

func rawUnityRootByteSuffixForPackName(lowerName string) (suffix string, kind string, ok bool) {
	for _, candidate := range []struct {
		suffix string
		kind   string
	}{
		{".texture.bytes", "rawtexture2d"},
		{".monoscript.bytes", "monoscript"},
		{".monobehaviour.bytes", "monobehaviour"},
		{".material.bytes", "material"},
		{".shader.bytes", "shader"},
		{".audioclip.bytes", "audioclip"},
		{".font.bytes", "font"},
	} {
		if strings.HasSuffix(lowerName, candidate.suffix) {
			return candidate.suffix, candidate.kind, true
		}
	}
	return "", "", false
}

func isUnityRawObjectPackPath(packPath string) bool {
	_, ok := unityRawKindForPackPath(packPath)
	return ok
}

func unityRawKindForPackPath(packPath string) (string, bool) {
	clean := filepath.ToSlash(packPath)
	dir := strings.ToLower(pathpkg.Base(pathpkg.Dir(clean)))
	switch dir {
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
	case "gameobject", "transform", "material", "meshrenderer", "meshfilter", "shader", "audioclip", "monobehaviour", "monoscript", "font":
		return dir, true
	default:
		if _, ok := unityRawClassIDForKind(dir); ok {
			return dir, true
		}
		return "", false
	}
}

func shouldSkipPackInput(path string, outputBaseName string) bool {
	name := strings.ToLower(filepath.Base(path))
	if name == "manifest.json" {
		return true
	}
	if strings.HasSuffix(name, ".meta.json") {
		return true
	}
	if strings.HasSuffix(name, ".typetree.json") {
		return true
	}
	if outputBaseName == "" {
		return shouldSkipDerivedPackInput(path)
	}
	base := strings.ToLower(outputBaseName)
	return name == base+".aba" || name == base+".ct" || shouldSkipDerivedPackInput(path)
}

func shouldSkipDerivedPackInput(path string) bool {
	for _, rawPath := range derivedPackArtifactRawPaths(path) {
		if _, err := os.Stat(rawPath); err == nil {
			return true
		}
	}
	return false
}

func derivedPackArtifactRawPaths(path string) []string {
	dir := filepath.Dir(path)
	name := filepath.Base(path)
	lower := strings.ToLower(name)
	parent := strings.ToLower(filepath.Base(dir))

	switch {
	case parent == "texture2d" && strings.HasSuffix(lower, ".tex.png"):
		return []string{filepath.Join(dir, name[:len(name)-len(".png")]+".bytes")}
	case parent == "texture2d" && strings.HasSuffix(lower, ".tex.jpg"):
		return []string{filepath.Join(dir, name[:len(name)-len(".jpg")]+".bytes")}
	case parent == "texture2d" && strings.HasSuffix(lower, ".tex.jpeg"):
		return []string{filepath.Join(dir, name[:len(name)-len(".jpeg")]+".bytes")}
	case parent == "texture2d" && strings.HasSuffix(lower, ".png"):
		base := name[:len(name)-len(".png")]
		return []string{
			filepath.Join(dir, base+".bytes"),
			filepath.Join(dir, base+".texture2d.bytes"),
		}
	case parent == "sprite" && strings.HasSuffix(lower, ".png"):
		return []string{filepath.Join(dir, name[:len(name)-len(".png")]+".sprite.bytes")}
	case parent == "mesh" && strings.HasSuffix(lower, ".crmesh"):
		return []string{filepath.Join(dir, name[:len(name)-len(".crmesh")]+".bytes")}
	case strings.HasSuffix(lower, ".tex.png"):
		return []string{filepath.Join(dir, name[:len(name)-len(".png")]+".bytes")}
	case strings.HasSuffix(lower, ".tex.jpg"):
		return []string{filepath.Join(dir, name[:len(name)-len(".jpg")]+".bytes")}
	case strings.HasSuffix(lower, ".tex.jpeg"):
		return []string{filepath.Join(dir, name[:len(name)-len(".jpeg")]+".bytes")}
	default:
		return nil
	}
}
