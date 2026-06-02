package KCES

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

// ModManifest 定义 KCES MOD 的打包清单 / ModManifest defines the packing manifest for a KCES MOD
type ModManifest struct {
	Name        string     `json:"name"`        // MOD 名称，同时作为输出文件名 name.ct 和 name.aba / MOD name, also used for output file names name.ct and name.aba
	CatalogType string     `json:"catalogType"` // 资源分类，如 Parts=4096 / Resource category such as Parts=4096
	PackageType string     `json:"packageType"` // 包类型，如 Plugin=1 / Package type such as Plugin=1
	Priority    int        `json:"priority"`    // 加载优先级 / Load priority
	Assets      []ModAsset `json:"assets"`      // 资源列表 / Asset list
}

// ModAsset 定义 MOD 中的单个资源文件 / ModAsset defines one asset file in a MOD
// Kind 决定资源在 .aba 中的 Unity 对象类型 / Kind controls the Unity object type written into .aba:
//   - "textasset"（默认）: TextAsset，适用于 .menuassets/.materialassets/.pmatassets/.model
//   - "texture2d": Texture2D，适用于 .tex（源文件为 PNG/JPEG，自动解码为 RGBA32）
//   - "rawtexture2d": Texture2D，适用于 .tex.bytes 原始对象数据透传
//   - "mesh": Mesh，适用于 .mmesh（源文件为 raw mesh 数据透传）
//   - "sprite": Sprite，适用于 .sprite.bytes 原始对象数据透传
//   - "spriteatlas": SpriteAtlas，适用于 .partsatlas/.partsassets 原始对象数据透传
//   - "animationclip": AnimationClip，适用于 .anm 原始对象数据透传
//   - 其他 Unity 原生类型可用小写 Class 名透传，如 "gameobject"、"transform"、"material"、
//     "meshrenderer"、"meshfilter"、"shader"、"audioclip"、"monobehaviour"、"monoscript"、"font"
type ModAsset struct {
	Name     string `json:"name"`               // 资源名称，如 xxx.menuassets，游戏通过此名称加载 / Resource name such as xxx.menuassets, used by the game for loading
	LoadName string `json:"loadName,omitempty"` // AssetBundle m_Container 中的加载 key，通常与 Name 相同 / Load key in AssetBundle m_Container, usually same as Name
	Path     string `json:"path"`               // 源文件路径，相对于 manifest 所在目录 / Source file path relative to the manifest directory
	Kind     string `json:"kind"`               // 资源类型，如 textasset、texture2d、mesh、sprite / Asset kind such as textasset, texture2d, mesh, or sprite
}

// ModPackService 提供 KCES MOD 打包服务 / ModPackService provides KCES MOD packing services
type ModPackService struct{}

// PackMod 根据 manifest 生成 .ct + .aba 文件
func (s *ModPackService) PackMod(manifestPath string, outputDir string) error {
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}

	var manifest ModManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}

	if manifest.Name == "" {
		return fmt.Errorf("manifest: name is required")
	}

	baseDir := filepath.Dir(manifestPath)
	if outputDir == "" {
		outputDir = baseDir
	}

	return packModManifest(manifest, baseDir, outputDir)
}

func packModManifest(manifest ModManifest, baseDir string, outputDir string) error {
	// 读取所有资源并按 kind 分流写入 SerializedFile
	sfWriter := aba.NewSerializedFileWriter("2021.3.37f1")

	type catalogEntry struct {
		name string // catalog 资源名称 / Catalog resource name
		ext  string // 资源扩展名 / Resource extension
	}
	var entries []catalogEntry

	for _, a := range manifest.Assets {
		srcPath := filepath.Join(baseDir, a.Path)
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("read asset %q: %w", a.Path, err)
		}

		name := a.Name
		if name == "" {
			name = filepath.Base(a.Path)
		}
		loadName := a.LoadName
		if loadName == "" {
			loadName = name
		}

		kind := strings.ToLower(a.Kind)
		if kind == "" {
			kind = inferKindForPack(name, a.Path)
		}

		if classID, ok := unityRawClassIDForKind(kind); ok {
			meta := readAssetMeta(srcPath)
			if meta.LoadName != "" {
				loadName = meta.LoadName
			}
			sfWriter.AddRawObjectWithLoadNameAndPathID(classID, name, loadName, data, meta.PathID)
		} else {
			meta := readAssetMeta(srcPath)
			if meta.LoadName != "" {
				loadName = meta.LoadName
			}
			switch kind {
			case "texture2d":
				width, height, rgba, err := decodeImageToRGBA32(data)
				if err != nil {
					return fmt.Errorf("decode image %q: %w", name, err)
				}
				sfWriter.AddTexture2DWithLoadNameAndPathID(name, loadName, width, height, rgba, meta.PathID)
			default:
				sfWriter.AddTextAssetWithLoadNameAndPathID(name, loadName, data, meta.PathID)
			}
		}

		ext := strings.ToLower(filepath.Ext(name))
		entries = append(entries, catalogEntry{name: name, ext: ext})
	}

	// 写入 SerializedFile → UnityFS bundle
	var sfBuf bytes.Buffer
	if err := sfWriter.Write(&sfBuf); err != nil {
		return fmt.Errorf("write SerializedFile: %w", err)
	}

	bundleEntries := []aba.BundleFileEntry{
		{Name: "CAB-" + manifest.Name, Data: sfBuf.Bytes(), IsSerialized: true},
	}
	var abaBuf bytes.Buffer
	if err := aba.WriteBundle(&abaBuf, bundleEntries, &aba.BundleWriteOptions{Compress: true}); err != nil {
		return fmt.Errorf("write .aba bundle: %w", err)
	}

	// 生成 catalog
	catalogType := parseCatalogType(manifest.CatalogType)
	packageType := parsePackageType(manifest.PackageType)

	catalog := &ct.AssetBundleCatalog{
		Version:           1000,
		CatalogType:       catalogType,
		PackageType:       packageType,
		Priority:          manifest.Priority,
		Name:              manifest.Name,
		Hash:              ct.HashStringIgnoreCase(manifest.Name),
		ResourceFileNames: []string{manifest.Name + ".aba"},
	}

	extGroups := map[string][]ct.ExtensionNamePack{}
	for _, e := range entries {
		if e.ext == "" {
			continue
		}
		hash := ct.HashStringIgnoreCase(e.name)
		extGroups[e.ext] = append(extGroups[e.ext], ct.ExtensionNamePack{Name: e.name, Hash: hash})
		catalog.Items = append(catalog.Items, ct.CatalogItem{ResourceIndex: 0, Name: e.name, Hash: hash})
	}

	// 按 hash 升序排序 catalog items（游戏使用 Array.BinarySearch）
	sort.Slice(catalog.Items, func(i, j int) bool {
		return catalog.Items[i].Hash < catalog.Items[j].Hash
	})

	for ext := range extGroups {
		catalog.ExtensionList = append(catalog.ExtensionList, ext)
	}
	// ExtensionNameList 内部也按 hash 排序
	for ext := range extGroups {
		sort.Slice(extGroups[ext], func(i, j int) bool {
			return extGroups[ext][i].Hash < extGroups[ext][j].Hash
		})
	}

	// 编码并压缩 catalog
	catalogData, err := ct.EncodeCatalog(catalog)
	if err != nil {
		return fmt.Errorf("encode catalog: %w", err)
	}
	compressedCatalog, err := ct.CompressLz4BlockArray(catalogData)
	if err != nil {
		return fmt.Errorf("compress catalog: %w", err)
	}

	// 构建 ContentTable
	table := &ct.ContentTable{
		Version: 1000,
		Files:   make(map[string]ct.VirtualFile),
		Raw:     make([]byte, ct.HeaderSize),
	}
	copy(table.Raw[:7], ct.FileSignature)
	table.Raw[7] = ct.SerializeTypeMsgPack

	table.AddFile("catalog", compressedCatalog)

	for ext, packs := range extGroups {
		enl := &ct.ExtensionNameList{Extension: ext, Data: packs}
		enlData, err := ct.EncodeExtensionNameList(enl)
		if err != nil {
			return fmt.Errorf("encode ExtensionNameList %q: %w", ext, err)
		}
		compressedEnl, err := ct.CompressLz4BlockArray(enlData)
		if err != nil {
			return fmt.Errorf("compress ExtensionNameList %q: %w", ext, err)
		}
		table.AddFile(ext, compressedEnl)
	}

	// 写入 .ct
	ctPath := filepath.Join(outputDir, manifest.Name+".ct")
	ctFile, err := os.Create(ctPath)
	if err != nil {
		return fmt.Errorf("create .ct: %w", err)
	}
	defer ctFile.Close()
	if err := ct.WriteContentTable(ctFile, table); err != nil {
		return fmt.Errorf("write .ct: %w", err)
	}

	// 写入 .aba
	abaPath := filepath.Join(outputDir, manifest.Name+".aba")
	if err := os.WriteFile(abaPath, abaBuf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write .aba: %w", err)
	}

	return nil
}

func readAssetMeta(assetPath string) rawAssetMeta {
	data, err := os.ReadFile(assetMetaPath(assetPath))
	if err != nil {
		return rawAssetMeta{}
	}
	var meta rawAssetMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return rawAssetMeta{}
	}
	return meta
}

// inferKind 根据资源名称的扩展名推断 kind
func inferKind(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
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
	default:
		return "textasset"
	}
}

func unityRawClassIDForKind(kind string) (int32, bool) {
	switch strings.ToLower(kind) {
	case "rawtexture2d":
		return aba.ClassIDTexture2D, true
	case "mesh":
		return aba.ClassIDMesh, true
	case "sprite":
		return aba.ClassIDSprite, true
	case "spriteatlas":
		return aba.ClassIDSpriteAtlas, true
	case "animationclip":
		return aba.ClassIDAnimationClip, true
	case "gameobject":
		return aba.ClassIDGameObject, true
	case "transform":
		return aba.ClassIDTransform, true
	case "material":
		return aba.ClassIDMaterial, true
	case "meshrenderer":
		return aba.ClassIDMeshRenderer, true
	case "meshfilter":
		return aba.ClassIDMeshFilter, true
	case "shader":
		return aba.ClassIDShader, true
	case "audioclip":
		return aba.ClassIDAudioClip, true
	case "monobehaviour":
		return aba.ClassIDMonoBehaviour, true
	case "monoscript":
		return aba.ClassIDMonoScript, true
	case "font":
		return aba.ClassIDFont, true
	default:
		if strings.HasPrefix(kind, "type_") {
			id, err := strconv.ParseInt(strings.TrimPrefix(kind, "type_"), 10, 32)
			if err == nil {
				return int32(id), true
			}
		}
		return 0, false
	}
}

// decodeImageToRGBA32 将 PNG/JPEG 图片解码为 RGBA32 像素数据
func decodeImageToRGBA32(data []byte) (width, height int, rgba []byte, err error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return 0, 0, nil, err
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	pixels := make([]byte, w*h*4)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, a := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			offset := (y*w + x) * 4
			pixels[offset] = byte(r >> 8)
			pixels[offset+1] = byte(g >> 8)
			pixels[offset+2] = byte(b >> 8)
			pixels[offset+3] = byte(a >> 8)
		}
	}

	return w, h, pixels, nil
}

func parseCatalogType(s string) ct.CatalogType {
	switch strings.ToLower(s) {
	case "parts":
		return ct.CatalogTypeParts
	case "partsmeta":
		return ct.CatalogTypePartsMeta
	case "motion":
		return ct.CatalogTypeMotion
	case "bg":
		return ct.CatalogTypeBg
	case "system":
		return ct.CatalogTypeSystem
	default:
		return ct.CatalogTypeParts
	}
}

func parsePackageType(s string) ct.CatalogPackageType {
	switch strings.ToLower(s) {
	case "base":
		return ct.PackageTypeBase
	case "plugin":
		return ct.PackageTypePlugin
	case "pluginpatch":
		return ct.PackageTypePluginPatch
	case "basepatch":
		return ct.PackageTypeBasePatch
	default:
		return ct.PackageTypePlugin
	}
}
