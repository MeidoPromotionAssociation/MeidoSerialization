package KCES

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

// ModManifest 定义 KCES MOD 的打包清单 / ModManifest defines the packing manifest for a KCES MOD
type ModManifest struct {
	Name        string     `json:"name"`              // MOD 名称，同时作为输出文件名 name.ct 和 name.aba / MOD name, also used for output file names name.ct and name.aba
	SubName     string     `json:"subName,omitempty"` // PluginPatch/ExtraPatch 的依赖子名称 / Dependency sub-name for PluginPatch and ExtraPatch
	CatalogType string     `json:"catalogType"`       // 资源分类，可用 | 组合 Flags，如 Parts|PartsMeta / Resource category flags, combinable with |
	PackageType string     `json:"packageType"`       // 包类型，如 Plugin=1 / Package type such as Plugin=1
	Priority    int32      `json:"priority"`          // 加载优先级 / Load priority
	Assets      []ModAsset `json:"assets"`            // 资源列表 / Asset list
}

// Kind 决定资源在 .aba 中的 Unity 对象类型 / Kind controls the Unity object type written into .aba:
//   - "textasset"（默认）: TextAsset，适用于 .menuassets/.materialassets/.pmatassets/.model
//   - "texture2d": Texture2D，适用于 .tex（源文件为 PNG/JPEG，自动解码为 RGBA32）
//   - "rawtexture2d": Texture2D，适用于 .tex.bytes 原始对象数据透传
//   - "mesh": Mesh，适用于 .mmesh（源文件为 raw mesh 数据透传）
//   - "sprite": Sprite，适用于 .sprite.bytes 原始对象数据透传
//   - "spriteatlas": SpriteAtlas，适用于 .partsatlas/.partsassets 原始对象数据透传
//   - "animationclip": AnimationClip，适用于 .anm 原始对象数据透传
//   - "cubemap": Cubemap 原始对象数据透传
//   - 其他 Unity 原生类型可用小写 Class 名透传，如 "gameobject"、"transform"、"material"、
//     "meshrenderer"、"meshfilter"、"shader"、"audioclip"、"monobehaviour"、"monoscript"、"font"
//
// ModAsset 定义 MOD 中的单个资源文件 / ModAsset defines one asset file in a MOD
type ModAsset struct {
	Name            string `json:"name"` // 资源短名称，同时作为 m_Name、m_Container 键和 CT 名称 / Short resource name used as m_Name, the m_Container key, and the CT name
	Path            string `json:"path"` // 源文件路径，相对于 manifest 所在目录 / Source file path relative to the manifest directory
	Kind            string `json:"kind"` // 资源类型，如 textasset、texture2d、mesh、sprite / Asset kind such as textasset, texture2d, mesh, or sprite
	preserveRawData bool   // 纯目录打包时是否严格保留原始对象字节 / Whether pure-directory packing must preserve raw object bytes exactly
}

// ModPackService 提供 KCES MOD 打包服务 / ModPackService provides KCES MOD packing services
type ModPackService struct{}

// 当前写入器以字节切片接收对象，因此在 io.ReadAll 前限制不可信输入大小
// The current writer accepts object byte slices, so untrusted input sizes are bounded before io.ReadAll
const maxPackInMemoryAssetSize int64 = 1 << 30

// PackMod 根据 manifest 生成 .ct + .aba 文件
// PackMod generates paired .ct and .aba files from a manifest
func (s *ModPackService) PackMod(manifestPath string, outputDir string) error {
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}

	var manifest ModManifest
	if err := json.Unmarshal(trimJSONUTF8BOM(manifestData), &manifest); err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}

	if manifest.Name == "" {
		return fmt.Errorf("manifest: name is required")
	}
	if err := validateModOutputName(manifest.Name); err != nil {
		return fmt.Errorf("manifest: invalid name %q: %w", manifest.Name, err)
	}

	baseDir := filepath.Dir(manifestPath)
	if outputDir == "" {
		outputDir = baseDir
	}

	return packModManifest(manifest, baseDir, outputDir)
}

// packModManifest 根据清单中的纯资源文件构建固定 Unity 2022.3.35f1 的 ABA 和对应 CT
// packModManifest builds a fixed Unity 2022.3.35f1 ABA and matching CT from the manifest's plain resource files
func packModManifest(manifest ModManifest, baseDir string, outputDir string) error {
	if err := validateModOutputName(manifest.Name); err != nil {
		return fmt.Errorf("manifest: invalid name %q: %w", manifest.Name, err)
	}
	if len(manifest.Assets) == 0 {
		return fmt.Errorf("manifest: assets must contain at least one resource")
	}
	catalogType, err := parseCatalogType(manifest.CatalogType)
	if err != nil {
		return fmt.Errorf("manifest: %w", err)
	}
	packageType, err := parsePackageType(manifest.PackageType)
	if err != nil {
		return fmt.Errorf("manifest: %w", err)
	}
	if (packageType == ct.PackageTypePluginPatch || packageType == ct.PackageTypeExtraPatch) && strings.TrimSpace(manifest.SubName) == "" {
		return fmt.Errorf("manifest: subName is required for packageType %q", manifest.PackageType)
	}

	sourceRoot, err := os.OpenRoot(baseDir)
	if err != nil {
		return fmt.Errorf("open manifest asset root %q: %w", baseDir, err)
	}
	defer sourceRoot.Close()

	assetPaths := make([]string, len(manifest.Assets))
	for i, asset := range manifest.Assets {
		relPath, err := normalizeModAssetPath(asset.Path)
		if err != nil {
			return fmt.Errorf("asset %q: unsafe source path: %w", asset.Path, err)
		}
		assetPaths[i] = relPath
	}
	canonicalLoadNames, err := buildCanonicalLoadNames(manifest.Assets, assetPaths)
	if err != nil {
		return fmt.Errorf("build canonical AssetBundle load names: %w", err)
	}
	versionSettings := resolveUnityPackSettings()

	// 所有对象共享一个 SerializedFile，原始 Unity 对象必须使用同一版本上下文
	// All objects share one SerializedFile, so raw Unity objects must use the same version context
	sfWriter := aba.NewSerializedFileWriter(versionSettings.UnityVersion)
	sfWriter.TargetPlatform = versionSettings.TargetPlatform
	sfWriter.SetExternalFiles(canonicalKCESExternalFiles())

	// catalogEntry 保存生成一个 CT 条目所需的短名称和扩展组 / catalogEntry stores the short name and extension group needed for one generated CT entry
	type catalogEntry struct {
		name string // catalog 资源名称 / Catalog resource name
		ext  string // 资源扩展名 / Resource extension
	}
	var entries []catalogEntry
	assetNames := make(map[uint64]string, len(manifest.Assets))
	pathIDs := make(map[int64]string, len(manifest.Assets))
	canonicalPaths := make([]string, 0, len(assetPaths))
	for assetIndex, asset := range manifest.Assets {
		kind := strings.ToLower(asset.Kind)
		if kind == "" {
			kind = inferKindForPack(asset.Name, asset.Path)
		}
		if kind == "abaraw" {
			continue
		}
		canonicalPaths = append(canonicalPaths, filepath.ToSlash(assetPaths[assetIndex]))
	}
	canonicalPathIDs, err := buildCanonicalPathIDs(canonicalPaths)
	if err != nil {
		return fmt.Errorf("build canonical PathIDs: %w", err)
	}

	for assetIndex, a := range manifest.Assets {
		relPath := assetPaths[assetIndex]
		name := a.Name
		if name == "" {
			name = filepath.Base(relPath)
		}
		loadName := canonicalLoadNames[assetIndex]
		if name == "" || strings.IndexByte(name, 0) >= 0 {
			return fmt.Errorf("asset %q: resolved name is empty or contains NUL", a.Path)
		}

		kind := strings.ToLower(a.Kind)
		if kind == "" {
			kind = inferKindForPack(name, a.Path)
		}
		if kind == "abaraw" {
			return fmt.Errorf("asset %q: .resS/.resource sidecars are not packable; stream payloads must be inlined into their Unity objects", a.Path)
		}
		canonicalPath, err := canonicalAssetPathForID(filepath.ToSlash(relPath))
		if err != nil {
			return fmt.Errorf("asset %q: canonicalize path: %w", a.Path, err)
		}
		pathID := canonicalPathIDs[canonicalPath]
		if pathID != 0 {
			if previous, exists := pathIDs[pathID]; exists {
				return fmt.Errorf("assets %q and %q request duplicate Unity PathID %d; silently reassigning one would break raw-object PPtr references", previous, name, pathID)
			}
			pathIDs[pathID] = name
		}
		catalogName := name
		if a.preserveRawData {
			catalogName = loadName
		}
		ext := strings.ToLower(filepath.Ext(catalogName))
		cataloged := ext != "" || kind == "textasset"
		// 只有通过 Catalog.Items 暴露的名称需要唯一查找哈希，m_Container 键会在后面独立检查
		// Only names exposed through Catalog.Items require unique lookup hashes, while m_Container keys are checked independently below
		if cataloged {
			nameHash := ct.HashStringIgnoreCase(catalogName)
			if previous, exists := assetNames[nameHash]; exists {
				return fmt.Errorf("assets %q and %q have the same case-insensitive catalog hash %d", previous, catalogName, nameHash)
			}
			assetNames[nameHash] = catalogName
		}
		if classID, ok := unityRawClassIDForKind(kind); ok {
			if a.preserveRawData {
				source, err := newPackRootSerializedObjectSource(sourceRoot, relPath)
				if err != nil {
					return fmt.Errorf("open raw asset %q: %w", a.Path, err)
				}
				sfWriter.AddRawObjectSourcePreservingDataWithLoadNameAndPathID(classID, name, loadName, source, pathID)
			} else {
				data, err := readPackRootRegularFile(sourceRoot, relPath)
				if err != nil {
					return fmt.Errorf("read raw asset %q: %w", a.Path, err)
				}
				sfWriter.AddRawObjectWithLoadNameAndPathID(classID, name, loadName, data, pathID)
			}
		} else {
			data, err := readPackRootRegularFile(sourceRoot, relPath)
			if err != nil {
				return fmt.Errorf("read asset %q: %w", a.Path, err)
			}
			switch kind {
			case "texture2d":
				width, height, rgba, err := decodeImageToRGBA32(data)
				if err != nil {
					return fmt.Errorf("decode image %q: %w", name, err)
				}
				sfWriter.AddTexture2DWithLoadNameAndPathID(name, loadName, width, height, rgba, pathID)
			case "textasset":
				sfWriter.AddTextAssetWithLoadNameAndPathID(name, loadName, data, pathID)
			default:
				return fmt.Errorf("asset %q: unsupported kind %q", a.Path, a.Kind)
			}
		}

		if cataloged {
			// 官方 system.ct 将无后缀资源的 ExtensionNameList 存在名为 null 的虚拟文件和分组中，catalog 条目仍使用真实的无扩展名资源名称
			// Official system.ct stores the ExtensionNameList for extensionless resources under the virtual-file and group name null, while the catalog item still uses the real extensionless asset name
			if ext == "" {
				ext = "null"
			}
			entries = append(entries, catalogEntry{name: catalogName, ext: ext})
		}
	}

	// 预先计算 SerializedFile 大小，UnityFS 随后两次调用同一流式生成器完成块表计算和最终写入
	// Compute the SerializedFile size up front, then let UnityFS invoke the same streaming generator twice for block-table calculation and final output
	serializedSize, err := sfWriter.Size()
	if err != nil {
		return fmt.Errorf("calculate SerializedFile size: %w", err)
	}
	abaEntries := []aba.AbaFileEntry{
		{Name: "CAB-" + manifest.Name, WriteTo: sfWriter.Write, Size: serializedSize, IsSerialized: true},
	}
	abaOptions := &aba.AbaWriteOptions{
		EngineVersion:     versionSettings.EngineVersion,
		GenerationVersion: versionSettings.GenerationVersion,
		Version:           versionSettings.AbaVersion,
		Compress:          true,
	}

	catalogName := manifest.Name
	catalogSubName := strings.TrimSpace(manifest.SubName)
	resourceFileName := manifest.Name + ".aba"
	catalog := &ct.AssetBundleCatalog{
		Kind:              ct.CatalogKindAssetBundle,
		Version:           1000,
		CatalogType:       catalogType,
		PackageType:       packageType,
		Priority:          manifest.Priority,
		Name:              &catalogName,
		SubName:           &catalogSubName,
		Hash:              ct.HashStringIgnoreCase(manifest.Name + ".aba"),
		ResourceFileNames: []*string{&resourceFileName},
	}

	extGroups := map[string][]*ct.ExtensionNamePack{}
	for _, e := range entries {
		hash := ct.HashStringIgnoreCase(e.name)
		name := e.name
		extGroups[e.ext] = append(extGroups[e.ext], &ct.ExtensionNamePack{Name: &name, Hash: hash})
		catalog.Items = append(catalog.Items, &ct.CatalogItem{ResourceIndex: 0, Name: &name, Hash: hash})
	}

	// 按 hash 升序排序 catalog items（游戏使用 Array.BinarySearch）
	sort.Slice(catalog.Items, func(i, j int) bool {
		return catalog.Items[i].Hash < catalog.Items[j].Hash
	})

	extensions := make([]string, 0, len(extGroups))
	for ext := range extGroups {
		extensions = append(extensions, ext)
	}
	sort.Strings(extensions)
	for index := range extensions {
		catalog.ExtensionList = append(catalog.ExtensionList, &extensions[index])
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

	if err := table.AddFile("catalog", compressedCatalog); err != nil {
		return err
	}

	for ext, packs := range extGroups {
		extension := ext
		enl := &ct.ExtensionNameList{Extension: &extension, Data: packs}
		enlData, err := ct.EncodeExtensionNameList(enl)
		if err != nil {
			return fmt.Errorf("encode ExtensionNameList %q: %w", ext, err)
		}
		compressedEnl, err := ct.CompressLz4BlockArray(enlData)
		if err != nil {
			return fmt.Errorf("compress ExtensionNameList %q: %w", ext, err)
		}
		if err := table.AddFile(ext, compressedEnl); err != nil {
			return err
		}
	}

	// 成对提交器将两个最终文件直接写入同目录临时文件，再备份旧目标并逐一原子重命名，任何提交错误都会删除本次新目标并恢复旧目标
	// The paired committer writes both final files directly to same-directory temporary files, then backs up old targets, atomically renames each file, and restores the old targets after any commit failure
	if err := writePackOutputPairWithWriters(
		outputDir,
		manifest.Name+".ct", func(out io.Writer) error {
			if err := ct.WriteContentTable(out, table); err != nil {
				return fmt.Errorf("write .ct: %w", err)
			}
			return nil
		},
		manifest.Name+".aba", func(out io.Writer) error {
			if err := aba.WriteAba(out, abaEntries, abaOptions); err != nil {
				return fmt.Errorf("write .aba file: %w", err)
			}
			return nil
		},
	); err != nil {
		return fmt.Errorf("commit .ct/.aba output pair: %w", err)
	}

	return nil
}

// buildCanonicalLoadNames 按规范路径为重复资源短名称分配稳定且不冲突的 AssetBundle 加载键
// buildCanonicalLoadNames assigns stable non-conflicting AssetBundle load keys to duplicate short resource names using canonical paths
func buildCanonicalLoadNames(assets []ModAsset, assetPaths []string) ([]string, error) {
	if len(assets) != len(assetPaths) {
		return nil, fmt.Errorf("asset count %d does not match path count %d", len(assets), len(assetPaths))
	}
	resolvedNames := make([]string, len(assets))
	groups := make(map[string][]int64, len(assets))
	reserved := make(map[string]struct{}, len(assets))
	for assetIndex := int64(0); assetIndex < int64(len(assets)); assetIndex++ {
		name := assets[assetIndex].Name
		if name == "" {
			name = filepath.Base(assetPaths[assetIndex])
		}
		if name == "" || strings.IndexByte(name, 0) >= 0 {
			return nil, fmt.Errorf("asset %q resolves to an empty name or a name containing NUL", assets[assetIndex].Path)
		}
		resolvedNames[assetIndex] = name
		key := strings.ToLower(name)
		groups[key] = append(groups[key], assetIndex)
		reserved[key] = struct{}{}
	}
	groupKeys := make([]string, 0, len(groups))
	for key := range groups {
		groupKeys = append(groupKeys, key)
	}
	sort.Strings(groupKeys)
	result := make([]string, len(assets))
	assigned := make(map[string]struct{}, len(assets))
	for _, groupKey := range groupKeys {
		indexes := groups[groupKey]
		sort.Slice(indexes, func(i int, j int) bool {
			left := strings.ToLower(filepath.ToSlash(assetPaths[indexes[i]]))
			right := strings.ToLower(filepath.ToSlash(assetPaths[indexes[j]]))
			if left != right {
				return left < right
			}
			return indexes[i] < indexes[j]
		})
		for ordinal := int64(0); ordinal < int64(len(indexes)); ordinal++ {
			assetIndex := indexes[ordinal]
			candidate := resolvedNames[assetIndex]
			if ordinal != 0 {
				for suffix := int64(2); ; suffix++ {
					candidate = canonicalOrdinalAssetName(resolvedNames[assetIndex], suffix)
					key := strings.ToLower(candidate)
					_, baseExists := reserved[key]
					_, assignedExists := assigned[key]
					if !baseExists && !assignedExists {
						break
					}
					if suffix == math.MaxInt64 {
						return nil, fmt.Errorf("cannot allocate a unique load name for %q", resolvedNames[assetIndex])
					}
				}
			}
			result[assetIndex] = candidate
			assigned[strings.ToLower(candidate)] = struct{}{}
		}
	}
	return result, nil
}

// normalizeModAssetPath 在交给 os.Root 前验证清单路径，并统一把两种斜杠解释为分隔符以阻止跨平台路径穿越
// normalizeModAssetPath validates a manifest path before os.Root use and treats both slash styles as separators to prevent cross-platform traversal
func normalizeModAssetPath(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("path is empty")
	}
	if !utf8.ValidString(name) {
		return "", fmt.Errorf("path is not valid UTF-8")
	}
	if strings.IndexByte(name, 0) >= 0 {
		return "", fmt.Errorf("path contains NUL")
	}

	portable := strings.ReplaceAll(name, `\`, "/")
	if strings.HasPrefix(portable, "/") || filepath.IsAbs(name) || filepath.VolumeName(name) != "" {
		return "", fmt.Errorf("absolute, drive-qualified, and UNC paths are not allowed")
	}
	parts := strings.Split(portable, "/")
	for _, part := range parts {
		if part == "" {
			return "", fmt.Errorf("path contains an empty component")
		}
		if part == "." || part == ".." {
			return "", fmt.Errorf("path contains unsafe component %q", part)
		}
		// 即使路径没有盘符，冒号在 Windows 上仍可访问备用数据流
		// A colon can address an alternate data stream on Windows even without a drive prefix
		if strings.ContainsRune(part, ':') {
			return "", fmt.Errorf("path component %q contains ':'", part)
		}
	}
	rel := filepath.Join(parts...)
	if rel == "." || filepath.IsAbs(rel) || filepath.VolumeName(rel) != "" {
		return "", fmt.Errorf("path is not relative")
	}
	return rel, nil
}

// readPackRootRegularFile 通过 os.Root 打开普通文件，并以 Lstat 和文件身份复查阻止链接、reparse point 与并发替换
// readPackRootRegularFile opens a regular file through os.Root and uses Lstat plus identity checks to reject links, reparse points, and concurrent replacement
func readPackRootRegularFile(root *os.Root, relPath string) ([]byte, error) {
	if root == nil {
		return nil, fmt.Errorf("nil manifest asset root")
	}
	before, err := root.Lstat(relPath)
	if err != nil {
		return nil, err
	}
	if isLinkOrReparse(before) {
		return nil, fmt.Errorf("refusing symlink or reparse point %q", relPath)
	}
	if !before.Mode().IsRegular() {
		return nil, fmt.Errorf("source %q is not a regular file", relPath)
	}
	if before.Size() < 0 || before.Size() > maxPackInMemoryAssetSize {
		return nil, fmt.Errorf("source %q size %d exceeds in-memory packing limit %d", relPath, before.Size(), maxPackInMemoryAssetSize)
	}

	f, err := root.Open(relPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect opened source %q: %w", relPath, err)
	}
	after, err := root.Lstat(relPath)
	if err != nil {
		return nil, fmt.Errorf("reinspect source %q: %w", relPath, err)
	}
	if isLinkOrReparse(after) || !after.Mode().IsRegular() ||
		!os.SameFile(before, opened) || !os.SameFile(opened, after) {
		return nil, fmt.Errorf("source %q changed or became a symlink/reparse point while opening", relPath)
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// newPackRootSerializedObjectSource 建立经过身份校验且可重复打开的原始对象流式数据源
// newPackRootSerializedObjectSource creates an identity-checked streamed raw-object source that can be reopened
func newPackRootSerializedObjectSource(root *os.Root, relPath string) (aba.SerializedObjectDataSource, error) {
	if root == nil {
		return aba.SerializedObjectDataSource{}, fmt.Errorf("nil manifest asset root")
	}
	before, err := root.Lstat(relPath)
	if err != nil {
		return aba.SerializedObjectDataSource{}, err
	}
	if isLinkOrReparse(before) {
		return aba.SerializedObjectDataSource{}, fmt.Errorf("refusing symlink or reparse point %q", relPath)
	}
	if !before.Mode().IsRegular() {
		return aba.SerializedObjectDataSource{}, fmt.Errorf("source %q is not a regular file", relPath)
	}
	if before.Size() < 0 || uint64(before.Size()) > uint64(math.MaxUint32) {
		return aba.SerializedObjectDataSource{}, fmt.Errorf("source %q size %d exceeds SerializedFile UInt32 object-size range", relPath, before.Size())
	}
	prefixSize := before.Size()
	const monoBehaviourPrefixSize int64 = 28
	if prefixSize > monoBehaviourPrefixSize {
		prefixSize = monoBehaviourPrefixSize
	}
	prefix, err := readVerifiedPackRootFilePrefix(root, relPath, before, prefixSize)
	if err != nil {
		return aba.SerializedObjectDataSource{}, err
	}
	return aba.SerializedObjectDataSource{
		Size:   uint32(before.Size()),
		Prefix: prefix,
		WriteTo: func(out io.Writer) error {
			return writeVerifiedPackRootFile(root, relPath, before, out)
		},
	}, nil
}

// readVerifiedPackRootFilePrefix 读取已检查普通文件的有界前缀并再次确认文件身份
// readVerifiedPackRootFilePrefix reads a bounded prefix from a checked regular file and confirms its identity again
func readVerifiedPackRootFilePrefix(root *os.Root, relPath string, expected os.FileInfo, size int64) ([]byte, error) {
	if expected == nil || size < 0 || size > expected.Size() {
		return nil, fmt.Errorf("invalid prefix size %d for source %q", size, relPath)
	}
	f, err := openVerifiedPackRootFile(root, relPath, expected)
	if err != nil {
		return nil, err
	}
	prefix := make([]byte, size)
	_, readErr := io.ReadFull(f, prefix)
	closeErr := f.Close()
	if readErr != nil {
		return nil, fmt.Errorf("read source %q prefix: %w", relPath, readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close source %q after prefix read: %w", relPath, closeErr)
	}
	if err := verifyPackRootFileIdentity(root, relPath, expected); err != nil {
		return nil, err
	}
	return prefix, nil
}

// writeVerifiedPackRootFile 将已检查普通文件按精确大小复制到目标并复查身份
// writeVerifiedPackRootFile copies a checked regular file at its exact size and verifies its identity afterward
func writeVerifiedPackRootFile(root *os.Root, relPath string, expected os.FileInfo, out io.Writer) error {
	if expected == nil || out == nil {
		return fmt.Errorf("nil file identity or destination for source %q", relPath)
	}
	f, err := openVerifiedPackRootFile(root, relPath, expected)
	if err != nil {
		return err
	}
	written, copyErr := io.CopyN(out, f, expected.Size())
	closeErr := f.Close()
	if copyErr != nil {
		return fmt.Errorf("copy source %q after %d bytes: %w", relPath, written, copyErr)
	}
	if written != expected.Size() {
		return fmt.Errorf("copy source %q wrote %d bytes, want %d", relPath, written, expected.Size())
	}
	if closeErr != nil {
		return fmt.Errorf("close source %q after copy: %w", relPath, closeErr)
	}
	return verifyPackRootFileIdentity(root, relPath, expected)
}

// openVerifiedPackRootFile 打开预先检查的普通文件并确认句柄身份和大小未改变
// openVerifiedPackRootFile opens a prechecked regular file and confirms that its handle identity and size are unchanged
func openVerifiedPackRootFile(root *os.Root, relPath string, expected os.FileInfo) (*os.File, error) {
	if root == nil || expected == nil {
		return nil, fmt.Errorf("nil root or expected file identity for %q", relPath)
	}
	f, err := root.Open(relPath)
	if err != nil {
		return nil, err
	}
	opened, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("inspect opened source %q: %w", relPath, err)
	}
	if !opened.Mode().IsRegular() || opened.Size() != expected.Size() || !os.SameFile(expected, opened) {
		f.Close()
		return nil, fmt.Errorf("source %q changed while opening", relPath)
	}
	if err := verifyPackRootFileIdentity(root, relPath, expected); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// verifyPackRootFileIdentity 通过 Lstat 确认路径仍指向原来的普通非 reparse 文件
// verifyPackRootFileIdentity uses Lstat to confirm that a path still names the original regular non-reparse file
func verifyPackRootFileIdentity(root *os.Root, relPath string, expected os.FileInfo) error {
	if root == nil || expected == nil {
		return fmt.Errorf("nil root or expected file identity for %q", relPath)
	}
	after, err := root.Lstat(relPath)
	if err != nil {
		return fmt.Errorf("reinspect source %q: %w", relPath, err)
	}
	if isLinkOrReparse(after) || !after.Mode().IsRegular() || after.Size() != expected.Size() || !os.SameFile(expected, after) {
		return fmt.Errorf("source %q changed or became a symlink/reparse point", relPath)
	}
	return nil
}

// packOutputState 记录成对输出提交期间单个文件的临时和备份状态 / packOutputState records one file's temporary and backup state during paired output commit
type packOutputState struct {
	name         string              // 最终文件名 / Final file name
	data         []byte              // 待提交字节 / Bytes to commit
	writeTo      packOutputWriteFunc // 直接生成待提交文件的回调 / Callback that directly generates the staged file
	tempName     string              // 同目录临时文件名 / Temporary file name in the same directory
	tempInfo     os.FileInfo         // 临时文件身份 / Temporary file identity
	backupName   string              // 旧目标备份名 / Previous-target backup name
	installed    bool                // 新目标是否已安装 / Whether the new target was installed
	hadOldTarget bool                // 提交前是否存在旧目标 / Whether a previous target existed before commit
}

// packRenameFunc 表示可注入的根目录内重命名操作 / packRenameFunc represents an injectable rename operation within an opened root
type packRenameFunc func(root *os.Root, oldName, newName string) error

// packOutputWriteFunc 将一个完整输出写入事务临时文件
// packOutputWriteFunc writes one complete output into a transactional temporary file
type packOutputWriteFunc func(io.Writer) error

// writePackOutputPair 以可回滚事务提交匹配的 CT 和 ABA 输出
// writePackOutputPair commits matching CT and ABA outputs with rollback support
func writePackOutputPair(outputDir, firstName string, firstData []byte, secondName string, secondData []byte) error {
	return writePackOutputPairWithRename(outputDir, firstName, firstData, secondName, secondData, nil)
}

// writePackOutputPairWithRename 使用可注入重命名操作提交输出，使回滚分支可被确定性测试
// writePackOutputPairWithRename commits outputs through an injectable rename operation so rollback can be tested deterministically
func writePackOutputPairWithRename(outputDir, firstName string, firstData []byte, secondName string, secondData []byte, rename packRenameFunc) error {
	outputs := []*packOutputState{
		{name: firstName, data: firstData},
		{name: secondName, data: secondData},
	}
	return writePackOutputStatesWithRename(outputDir, outputs, rename)
}

// writePackOutputPairWithWriters 以流式生成器和可回滚事务提交匹配的 CT 与 ABA 输出
// writePackOutputPairWithWriters commits matching CT and ABA outputs from streaming generators with rollback support
func writePackOutputPairWithWriters(outputDir string, firstName string, firstWriter packOutputWriteFunc, secondName string, secondWriter packOutputWriteFunc) error {
	if firstWriter == nil || secondWriter == nil {
		return fmt.Errorf("output writer is nil")
	}
	outputs := []*packOutputState{
		{name: firstName, writeTo: firstWriter},
		{name: secondName, writeTo: secondWriter},
	}
	return writePackOutputStatesWithRename(outputDir, outputs, nil)
}

// writePackOutputStatesWithRename 暂存、同步并以可注入重命名操作提交一组输出
// writePackOutputStatesWithRename stages, synchronizes, and commits outputs through an injectable rename operation
func writePackOutputStatesWithRename(outputDir string, outputs []*packOutputState, rename packRenameFunc) error {
	root, err := openPackOutputRoot(outputDir)
	if err != nil {
		return err
	}
	defer root.Close()
	if rename == nil {
		rename = func(root *os.Root, oldName, newName string) error {
			return root.Rename(oldName, newName)
		}
	}

	for _, output := range outputs {
		if output == nil {
			return fmt.Errorf("nil output state")
		}
		if output.writeTo != nil && output.data != nil {
			return fmt.Errorf("output %q has both bytes and a writer", output.name)
		}
		if err := validateModOutputName(strings.TrimSuffix(strings.TrimSuffix(output.name, ".ct"), ".aba")); err != nil {
			return fmt.Errorf("unsafe output file name %q: %w", output.name, err)
		}
		if err := inspectPackOutputTarget(root, output.name); err != nil {
			return err
		}
	}

	// 临时文件始终清理，但此处不会删除备份，因为回滚本身失败时必须以错误中报告的备份名保留旧数据
	// Temporary files are always cleaned, but this defer deliberately preserves backups because old data must remain under the reported backup name if rollback itself fails
	defer func() {
		for _, output := range outputs {
			if output.tempName != "" {
				_ = root.Remove(output.tempName)
			}
		}
	}()
	for _, output := range outputs {
		if err := stagePackOutput(root, output); err != nil {
			return err
		}
	}

	if err := commitPackOutputs(root, outputs, rename); err != nil {
		return err
	}
	return nil
}

// openPackOutputRoot 安全打开非链接输出目录并确认打开前后的文件身份一致
// openPackOutputRoot safely opens a non-link output directory and confirms its identity did not change
func openPackOutputRoot(outputDir string) (*os.Root, error) {
	if outputDir == "" {
		return nil, fmt.Errorf("output directory is empty")
	}
	absPath, err := filepath.Abs(outputDir)
	if err != nil {
		return nil, fmt.Errorf("resolve output directory %q: %w", outputDir, err)
	}
	info, err := os.Lstat(absPath)
	if err != nil {
		return nil, fmt.Errorf("inspect output directory %q: %w", absPath, err)
	}
	if isLinkOrReparse(info) {
		return nil, fmt.Errorf("output directory %q is a symlink or reparse point", absPath)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("output path %q is not a directory", absPath)
	}
	root, err := os.OpenRoot(absPath)
	if err != nil {
		return nil, fmt.Errorf("open output directory %q: %w", absPath, err)
	}
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(info, opened) {
		root.Close()
		if err != nil {
			return nil, fmt.Errorf("inspect opened output directory %q: %w", absPath, err)
		}
		return nil, fmt.Errorf("output directory %q changed while opening", absPath)
	}
	return root, nil
}

// inspectPackOutputTarget 确认已有输出目标是可替换的普通文件
// inspectPackOutputTarget confirms that an existing output target is a replaceable regular file
func inspectPackOutputTarget(root *os.Root, name string) error {
	info, err := root.Lstat(name)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect output target %q: %w", name, err)
	}
	if isLinkOrReparse(info) {
		return fmt.Errorf("refusing symlink or reparse point output target %q", name)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("output target %q is not a regular file", name)
	}
	return nil
}

// stagePackOutput 在输出目录中创建并同步一个唯一临时文件
// stagePackOutput creates and synchronizes one unique temporary file in the output directory
func stagePackOutput(root *os.Root, output *packOutputState) error {
	for attempt := int64(0); attempt < 32; attempt++ {
		name, err := randomPackWorkName("tmp")
		if err != nil {
			return err
		}
		f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("create temporary output for %q: %w", output.name, err)
		}
		output.tempName = name
		var writeErr error
		if output.writeTo != nil {
			writeErr = output.writeTo(f)
		} else {
			_, writeErr = io.Copy(f, bytes.NewReader(output.data))
		}
		if writeErr == nil {
			writeErr = f.Sync()
		}
		closeErr := f.Close()
		if writeErr != nil {
			return fmt.Errorf("write temporary output for %q: %w", output.name, writeErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close temporary output for %q: %w", output.name, closeErr)
		}
		output.tempInfo, err = root.Stat(name)
		if err != nil {
			return fmt.Errorf("inspect temporary output for %q: %w", output.name, err)
		}
		return nil
	}
	return fmt.Errorf("could not allocate temporary output for %q", output.name)
}

// randomPackWorkName 生成不可预测的事务工作文件名
// randomPackWorkName generates an unpredictable transaction work-file name
func randomPackWorkName(kind string) (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate %s file name: %w", kind, err)
	}
	return ".meido-pack-" + hex.EncodeToString(value[:]) + "." + kind, nil
}

// unusedPackBackupName 在输出根目录中寻找尚未占用的随机备份名
// unusedPackBackupName finds an unused random backup name inside the output root
func unusedPackBackupName(root *os.Root) (string, error) {
	for attempt := int64(0); attempt < 32; attempt++ {
		name, err := randomPackWorkName("backup")
		if err != nil {
			return "", err
		}
		if _, err := root.Lstat(name); os.IsNotExist(err) {
			return name, nil
		} else if err != nil {
			return "", fmt.Errorf("inspect backup path %q: %w", name, err)
		}
	}
	return "", fmt.Errorf("could not allocate backup file name")
}

// commitPackOutputs 备份旧目标并安装全部暂存输出，任一步失败都会尝试回滚
// commitPackOutputs backs up previous targets and installs all staged outputs, attempting rollback after any failure
func commitPackOutputs(root *os.Root, outputs []*packOutputState, rename packRenameFunc) error {
	rollbackFailure := func(commitErr error) error {
		if rollbackErr := rollbackPackOutputs(root, outputs, rename); rollbackErr != nil {
			return errors.Join(commitErr, fmt.Errorf("rollback output pair: %w", rollbackErr))
		}
		return commitErr
	}

	// Windows 无法通过 os.Root 提供可移植的覆盖式重命名契约，因此先移开每个旧目标，使安装始终重命名到空闲名称并保留旧文件对用于回滚
	// Windows does not provide a portable rename-over-existing contract through os.Root, so each old target is moved aside first to install onto a vacant name while preserving the previous pair for rollback
	for _, output := range outputs {
		info, err := root.Lstat(output.name)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return rollbackFailure(fmt.Errorf("inspect existing output %q: %w", output.name, err))
		}
		if isLinkOrReparse(info) || !info.Mode().IsRegular() {
			return rollbackFailure(fmt.Errorf("existing output %q is not a regular non-reparse file", output.name))
		}
		backupName, err := unusedPackBackupName(root)
		if err != nil {
			return rollbackFailure(err)
		}
		if err := rename(root, output.name, backupName); err != nil {
			return rollbackFailure(fmt.Errorf("back up existing output %q: %w", output.name, err))
		}
		output.backupName = backupName
		output.hadOldTarget = true
	}

	for _, output := range outputs {
		if err := rename(root, output.tempName, output.name); err != nil {
			return rollbackFailure(fmt.Errorf("install output %q: %w", output.name, err))
		}
		output.installed = true
	}

	var cleanupErrs []error
	for _, output := range outputs {
		if output.backupName == "" {
			continue
		}
		if err := root.Remove(output.backupName); err != nil {
			cleanupErrs = append(cleanupErrs, fmt.Errorf("outputs committed but remove backup %q: %w", output.backupName, err))
			continue
		}
		output.backupName = ""
	}
	return errors.Join(cleanupErrs...)
}

// rollbackPackOutputs 删除本次安装的文件并恢复提交前备份
// rollbackPackOutputs removes files installed by the current transaction and restores previous backups
func rollbackPackOutputs(root *os.Root, outputs []*packOutputState, rename packRenameFunc) error {
	var rollbackErrs []error
	for outputIndex := int64(len(outputs)) - 1; outputIndex >= 0; outputIndex-- {
		output := outputs[outputIndex]
		if !output.installed {
			continue
		}
		info, err := root.Lstat(output.name)
		if err != nil {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("inspect newly installed output %q: %w", output.name, err))
			continue
		}
		if !os.SameFile(info, output.tempInfo) {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("newly installed output %q was replaced concurrently; refusing to remove it", output.name))
			continue
		}
		if err := root.Remove(output.name); err != nil {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("remove newly installed output %q: %w", output.name, err))
			continue
		}
		output.installed = false
	}
	for outputIndex := int64(len(outputs)) - 1; outputIndex >= 0; outputIndex-- {
		output := outputs[outputIndex]
		if !output.hadOldTarget || output.backupName == "" {
			continue
		}
		if _, err := root.Lstat(output.name); err == nil {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("cannot restore %q because the target name is occupied; old data remains in %q", output.name, output.backupName))
			continue
		} else if !os.IsNotExist(err) {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("inspect restore target %q: %w", output.name, err))
			continue
		}
		if err := rename(root, output.backupName, output.name); err != nil {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("restore old output %q from %q: %w", output.name, output.backupName, err))
			continue
		}
		output.backupName = ""
		output.hadOldTarget = false
	}
	return errors.Join(rollbackErrs...)
}

// unityRawClassIDForKind 将原始对象 kind 映射到精确 Unity class ID
// unityRawClassIDForKind maps a raw object kind to its exact Unity class ID
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
	case "cubemap":
		return aba.ClassIDCubemap, true
	case "monobehaviour":
		return aba.ClassIDMonoBehaviour, true
	case "monoscript":
		return aba.ClassIDMonoScript, true
	case "font":
		return aba.ClassIDFont, true
	default:
		if strings.HasPrefix(kind, "type_") {
			id, err := strconv.ParseInt(strings.TrimPrefix(kind, "type_"), 10, 32)
			if err == nil && id > 0 {
				return int32(id), true
			}
		}
		return 0, false
	}
}

// decodeImageToRGBA32 将 PNG 或 JPEG 图片解码为 RGBA32 像素数据，并以固定宽度返回尺寸
// decodeImageToRGBA32 decodes a PNG or JPEG image into RGBA32 pixels and returns fixed-width dimensions
func decodeImageToRGBA32(data []byte) (width, height int64, rgba []byte, err error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return 0, 0, nil, err
	}

	bounds := img.Bounds()
	width = int64(bounds.Dx())
	height = int64(bounds.Dy())
	pixels := make([]byte, width*height*4)

	for y := int64(0); y < height; y++ {
		for x := int64(0); x < width; x++ {
			r, g, b, a := img.At(bounds.Min.X+int(x), bounds.Min.Y+int(y)).RGBA()
			offset := (y*width + x) * 4
			pixels[offset] = byte(r >> 8)
			pixels[offset+1] = byte(g >> 8)
			pixels[offset+2] = byte(b >> 8)
			pixels[offset+3] = byte(a >> 8)
		}
	}

	return width, height, pixels, nil
}

// parseCatalogType 解析可用竖线、逗号或加号组合的 KCES CatalogType 标志
// parseCatalogType parses KCES CatalogType flags combined with pipes, commas, or plus signs
func parseCatalogType(s string) (ct.CatalogType, error) {
	values := map[string]ct.CatalogType{
		"unknown":   ct.CatalogTypeUnknown,
		"language":  ct.CatalogTypeLanguage,
		"product":   ct.CatalogTypeProduct,
		"movie":     ct.CatalogTypeMovie,
		"script":    ct.CatalogTypeScript,
		"sound":     ct.CatalogTypeSound,
		"voice":     ct.CatalogTypeVoice,
		"csv":       ct.CatalogTypeCsv,
		"system":    ct.CatalogTypeSystem,
		"bg":        ct.CatalogTypeBg,
		"motion":    ct.CatalogTypeMotion,
		"partsmeta": ct.CatalogTypePartsMeta,
		"parts":     ct.CatalogTypeParts,
	}
	tokens := strings.FieldsFunc(strings.TrimSpace(s), func(r rune) bool {
		return r == '|' || r == ',' || r == '+'
	})
	if len(tokens) == 0 {
		return 0, fmt.Errorf("catalogType is required")
	}
	var result ct.CatalogType
	for _, token := range tokens {
		name := strings.ToLower(strings.TrimSpace(token))
		value, ok := values[name]
		if !ok || name == "" {
			return 0, fmt.Errorf("unsupported catalogType %q", strings.TrimSpace(token))
		}
		if result&value != 0 {
			return 0, fmt.Errorf("duplicate catalogType flag %q", strings.TrimSpace(token))
		}
		result |= value
	}
	return result, nil
}

// parsePackageType 将清单包类型名称解析为 KCES CatalogPackageType
// parsePackageType parses a manifest package-type name into KCES CatalogPackageType
func parsePackageType(s string) (ct.CatalogPackageType, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "base":
		return ct.PackageTypeBase, nil
	case "plugin":
		return ct.PackageTypePlugin, nil
	case "pluginpatch":
		return ct.PackageTypePluginPatch, nil
	case "basepatch":
		return ct.PackageTypeBasePatch, nil
	case "extrabase":
		return ct.PackageTypeExtraBase, nil
	case "extrapatch":
		return ct.PackageTypeExtraPatch, nil
	default:
		return 0, fmt.Errorf("unsupported packageType %q", strings.TrimSpace(s))
	}
}

// validateModOutputName 验证 MOD 输出名是安全的单个相对文件名组件
// validateModOutputName validates that a MOD output name is one safe relative file-name component
func validateModOutputName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("name is empty")
	}
	if strings.IndexByte(name, 0) >= 0 {
		return fmt.Errorf("name contains NUL")
	}
	if name == "." || name == ".." || filepath.IsAbs(name) || filepath.VolumeName(name) != "" ||
		strings.ContainsAny(name, `/\\:`) {
		return fmt.Errorf("name must be a single relative file-name component")
	}
	return nil
}
