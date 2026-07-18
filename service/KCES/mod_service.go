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
	Name         string     `json:"name"`                   // MOD 名称，同时作为输出文件名 name.ct 和 name.aba / MOD name, also used for output file names name.ct and name.aba
	SubName      string     `json:"subName,omitempty"`      // PluginPatch/ExtraPatch 的依赖子名称 / Dependency sub-name for PluginPatch and ExtraPatch
	CatalogType  string     `json:"catalogType"`            // 资源分类，可用 | 组合 Flags，如 Parts|PartsMeta / Resource category flags, combinable with |
	PackageType  string     `json:"packageType"`            // 包类型，如 Plugin=1 / Package type such as Plugin=1
	Priority     int        `json:"priority"`               // 加载优先级 / Load priority
	UnityVersion string     `json:"unityVersion,omitempty"` // 新建对象使用的 Unity 版本；原始 sidecar 必须与其一致 / Unity version for generated objects; raw sidecars must agree
	Assets       []ModAsset `json:"assets"`                 // 资源列表 / Asset list
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
//   - "abaraw": UnityFS 非序列化 sidecar（.resS/.resource/.resources），不进入 catalog/m_Container
//   - 其他 Unity 原生类型可用小写 Class 名透传，如 "gameobject"、"transform"、"material"、
//     "meshrenderer"、"meshfilter"、"shader"、"audioclip"、"monobehaviour"、"monoscript"、"font"
type ModAsset struct {
	Name     string `json:"name"`               // 资源名称，如 xxx.menuassets，游戏通过此名称加载 / Resource name such as xxx.menuassets, used by the game for loading
	LoadName string `json:"loadName,omitempty"` // AssetBundle m_Container 中的加载 key，通常与 Name 相同 / Load key in AssetBundle m_Container, usually same as Name
	Path     string `json:"path"`               // 源文件路径，相对于 manifest 所在目录 / Source file path relative to the manifest directory
	Kind     string `json:"kind"`               // 资源类型，如 textasset、texture2d、mesh、sprite / Asset kind such as textasset, texture2d, mesh, or sprite
	// Catalog controls whether this object receives Catalog/ExtensionNameList
	// entries. nil keeps the safe default: extension-bearing objects and
	// extensionless TextAssets are catalogued, while extensionless raw Unity
	// dependency objects are only placed in AssetBundle.m_Container.
	Catalog *bool `json:"catalog,omitempty"`
}

// ModPackService 提供 KCES MOD 打包服务。
//
// ModPackService provides KCES MOD packing services.
type ModPackService struct{}

// The current writer accepts byte slices for every object and .aba sidecar,
// so packing is necessarily in-memory. Bound attacker-controlled source sizes
// before io.ReadAll; larger multi-gigabyte KCES .resS files require a future
// streaming AbaFileEntry API.
const maxPackInMemoryAssetSize int64 = 1 << 30

// PackMod 根据 manifest 生成 .ct + .aba 文件
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

	assetMetas := make([]rawAssetMeta, len(manifest.Assets))
	metaSources := make([]string, len(manifest.Assets))
	assetPaths := make([]string, len(manifest.Assets))
	for i, asset := range manifest.Assets {
		relPath, err := normalizeModAssetPath(asset.Path)
		if err != nil {
			return fmt.Errorf("asset %q: unsafe source path: %w", asset.Path, err)
		}
		meta, err := readAssetMetaStrictFromRoot(sourceRoot, relPath)
		if err != nil {
			return fmt.Errorf("asset %q: %w", asset.Path, err)
		}
		assetPaths[i] = relPath
		assetMetas[i] = meta
		metaSources[i] = asset.Path
	}
	versionMetas := assetMetas
	versionSources := metaSources
	if strings.TrimSpace(manifest.UnityVersion) != "" {
		manifestVersion := strings.TrimSpace(manifest.UnityVersion)
		versionMetas = append(append([]rawAssetMeta(nil), assetMetas...), rawAssetMeta{
			UnityVersion:  manifestVersion,
			EngineVersion: manifestVersion,
		})
		versionSources = append(append([]string(nil), metaSources...), "manifest unityVersion")
	}
	versionSettings, err := resolveUnityPackSettings(versionMetas, versionSources)
	if err != nil {
		return fmt.Errorf("resolve Unity version context: %w", err)
	}

	// 所有对象共享一个 SerializedFile；原始 Unity 对象必须使用同一版本上下文。
	sfWriter := aba.NewSerializedFileWriter(versionSettings.UnityVersion)
	sfWriter.TargetPlatform = versionSettings.TargetPlatform

	type catalogEntry struct {
		name string // catalog 资源名称 / Catalog resource name
		ext  string // 资源扩展名 / Resource extension
	}
	var entries []catalogEntry
	var abaSidecars []aba.AbaFileEntry
	assetNames := make(map[uint64]string, len(manifest.Assets))
	loadNames := make(map[string]string, len(manifest.Assets))
	pathIDs := make(map[int64]string, len(manifest.Assets))
	sidecarNames := make(map[string]string)

	for assetIndex, a := range manifest.Assets {
		relPath := assetPaths[assetIndex]
		data, err := readPackRootRegularFile(sourceRoot, relPath)
		if err != nil {
			return fmt.Errorf("read asset %q: %w", a.Path, err)
		}

		name := a.Name
		if name == "" {
			name = filepath.Base(relPath)
		}
		loadName := a.LoadName
		if loadName == "" {
			loadName = name
		}
		if name == "" || strings.IndexByte(name, 0) >= 0 {
			return fmt.Errorf("asset %q: resolved name is empty or contains NUL", a.Path)
		}

		kind := strings.ToLower(a.Kind)
		if kind == "" {
			kind = inferKindForPack(name, a.Path)
		}
		meta := assetMetas[assetIndex]
		if kind == "abaraw" {
			if a.Catalog != nil && *a.Catalog {
				return fmt.Errorf("asset %q: .aba sidecar cannot be inserted into the asset catalog", a.Path)
			}
			if meta.PathID != 0 || meta.LoadName != "" {
				return fmt.Errorf("asset %q: .aba sidecar must not have Unity PathID/loadName metadata", a.Path)
			}
			key := strings.ToLower(filepath.ToSlash(name))
			if previous, exists := sidecarNames[key]; exists {
				return fmt.Errorf(".aba sidecars %q and %q use the same case-insensitive directory name", previous, name)
			}
			sidecarNames[key] = name
			abaSidecars = append(abaSidecars, aba.AbaFileEntry{
				Name: name,
				Data: data,
			})
			continue
		}
		if meta.LoadName != "" {
			loadName = meta.LoadName
		}
		if meta.PathID != 0 {
			if previous, exists := pathIDs[meta.PathID]; exists {
				return fmt.Errorf("assets %q and %q request duplicate Unity PathID %d; silently reassigning one would break raw-object PPtr references", previous, name, meta.PathID)
			}
			pathIDs[meta.PathID] = name
		}
		ext := strings.ToLower(filepath.Ext(name))
		cataloged := ext != "" || kind == "textasset"
		if a.Catalog != nil {
			cataloged = *a.Catalog
		}
		// Only names exposed through Catalog.Items require unique lookup hashes;
		// m_Container keys are checked independently below.
		if cataloged {
			nameHash := ct.HashStringIgnoreCase(name)
			if previous, exists := assetNames[nameHash]; exists {
				return fmt.Errorf("assets %q and %q have the same case-insensitive catalog hash %d", previous, name, nameHash)
			}
			assetNames[nameHash] = name
		}
		loadKey := strings.ToLower(loadName)
		if previous, exists := loadNames[loadKey]; exists {
			return fmt.Errorf("assets %q and %q use duplicate AssetBundle loadName %q", previous, name, loadName)
		}
		loadNames[loadKey] = name

		if classID, ok := unityRawClassIDForKind(kind); ok {
			sfWriter.AddRawObjectWithLoadNameAndPathID(classID, name, loadName, data, meta.PathID)
		} else {
			switch kind {
			case "texture2d":
				width, height, rgba, err := decodeImageToRGBA32(data)
				if err != nil {
					return fmt.Errorf("decode image %q: %w", name, err)
				}
				sfWriter.AddTexture2DWithLoadNameAndPathID(name, loadName, width, height, rgba, meta.PathID)
			case "textasset":
				sfWriter.AddTextAssetWithLoadNameAndPathID(name, loadName, data, meta.PathID)
			default:
				return fmt.Errorf("asset %q: unsupported kind %q", a.Path, a.Kind)
			}
		}

		if cataloged {
			// Official system.ct stores the ExtensionNameList for resources with
			// no suffix under the virtual-file/group name "null". The catalog item
			// itself still uses the real extensionless asset name.
			if ext == "" {
				ext = "null"
			}
			entries = append(entries, catalogEntry{name: name, ext: ext})
		}
	}

	// 写入 SerializedFile → UnityFS .aba 文件
	var sfBuf bytes.Buffer
	if err := sfWriter.Write(&sfBuf); err != nil {
		return fmt.Errorf("write SerializedFile: %w", err)
	}

	abaEntries := []aba.AbaFileEntry{
		{Name: "CAB-" + manifest.Name, Data: sfBuf.Bytes(), IsSerialized: true},
	}
	abaEntries = append(abaEntries, abaSidecars...)
	var abaBuf bytes.Buffer
	if err := aba.WriteAba(&abaBuf, abaEntries, &aba.AbaWriteOptions{
		EngineVersion:     versionSettings.EngineVersion,
		GenerationVersion: versionSettings.GenerationVersion,
		Version:           versionSettings.AbaVersion,
		Compress:          true,
	}); err != nil {
		return fmt.Errorf("write .aba file: %w", err)
	}

	catalog := &ct.AssetBundleCatalog{
		Version:           1000,
		CatalogType:       catalogType,
		PackageType:       packageType,
		Priority:          manifest.Priority,
		Name:              manifest.Name,
		SubName:           strings.TrimSpace(manifest.SubName),
		Hash:              ct.HashStringIgnoreCase(manifest.Name + ".aba"),
		ResourceFileNames: []string{manifest.Name + ".aba"},
	}

	extGroups := map[string][]ct.ExtensionNamePack{}
	for _, e := range entries {
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
	sort.Strings(catalog.ExtensionList)
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

	// 两个最终文件在触碰输出目录前都完整生成到内存。落盘由成对提交器
	// 处理：先写同目录临时文件，再备份旧目标并逐一原子 rename；任何提交
	// 错误都会删除本次新目标并恢复旧目标。
	var ctBuf bytes.Buffer
	if err := ct.WriteContentTable(&ctBuf, table); err != nil {
		return fmt.Errorf("write .ct: %w", err)
	}
	if err := writePackOutputPair(
		outputDir,
		manifest.Name+".ct", ctBuf.Bytes(),
		manifest.Name+".aba", abaBuf.Bytes(),
	); err != nil {
		return fmt.Errorf("commit .ct/.aba output pair: %w", err)
	}

	return nil
}

// normalizeModAssetPath validates a manifest path before it is passed to
// os.Root. Both slash styles are interpreted as separators so a manifest made
// on another platform cannot hide a traversal from the current host.
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
		// A colon can address an alternate data stream on Windows even when the
		// path has no drive prefix.
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

// readPackRootRegularFile uses os.Root for the actual open, not merely for a
// lexical containment check. Lstat checks reject symlink/reparse-point inputs;
// the repeated identity check also detects ordinary replacements around open.
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

func readAssetMetaStrictFromRoot(root *os.Root, assetRelPath string) (rawAssetMeta, error) {
	metaRelPath := assetRelPath + ".meta.json"
	if _, err := root.Lstat(metaRelPath); err != nil {
		if os.IsNotExist(err) {
			return rawAssetMeta{}, nil
		}
		return rawAssetMeta{}, fmt.Errorf("inspect asset metadata %q: %w", metaRelPath, err)
	}
	data, err := readPackRootRegularFile(root, metaRelPath)
	if err != nil {
		return rawAssetMeta{}, fmt.Errorf("read asset metadata %q: %w", metaRelPath, err)
	}
	var meta rawAssetMeta
	if err := json.Unmarshal(trimJSONUTF8BOM(data), &meta); err != nil {
		return rawAssetMeta{}, fmt.Errorf("parse asset metadata %q: %w", metaRelPath, err)
	}
	return meta, nil
}

type packOutputState struct {
	name         string
	data         []byte
	tempName     string
	tempInfo     os.FileInfo
	backupName   string
	installed    bool
	hadOldTarget bool
}

type packRenameFunc func(root *os.Root, oldName, newName string) error

func writePackOutputPair(outputDir, firstName string, firstData []byte, secondName string, secondData []byte) error {
	return writePackOutputPairWithRename(outputDir, firstName, firstData, secondName, secondData, nil)
}

// writePackOutputPairWithRename has an injectable rename operation so the
// rollback branch can be exercised deterministically without weakening the
// production path.
func writePackOutputPairWithRename(outputDir, firstName string, firstData []byte, secondName string, secondData []byte, rename packRenameFunc) error {
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

	outputs := []*packOutputState{
		{name: firstName, data: firstData},
		{name: secondName, data: secondData},
	}
	for _, output := range outputs {
		if err := validateModOutputName(strings.TrimSuffix(strings.TrimSuffix(output.name, ".ct"), ".aba")); err != nil {
			return fmt.Errorf("unsafe output file name %q: %w", output.name, err)
		}
		if err := inspectPackOutputTarget(root, output.name); err != nil {
			return err
		}
	}

	// Temp files are always cleaned. Backups are deliberately not removed by
	// this defer: if rollback itself fails, preserving the old bytes under the
	// reported backup name is safer than destroying the recovery copy.
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

func stagePackOutput(root *os.Root, output *packOutputState) error {
	for attempt := 0; attempt < 32; attempt++ {
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
		n, writeErr := f.Write(output.data)
		if writeErr == nil && n != len(output.data) {
			writeErr = io.ErrShortWrite
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

func randomPackWorkName(kind string) (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate %s file name: %w", kind, err)
	}
	return ".meido-pack-" + hex.EncodeToString(value[:]) + "." + kind, nil
}

func unusedPackBackupName(root *os.Root) (string, error) {
	for attempt := 0; attempt < 32; attempt++ {
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

func commitPackOutputs(root *os.Root, outputs []*packOutputState, rename packRenameFunc) error {
	rollbackFailure := func(commitErr error) error {
		if rollbackErr := rollbackPackOutputs(root, outputs, rename); rollbackErr != nil {
			return errors.Join(commitErr, fmt.Errorf("rollback output pair: %w", rollbackErr))
		}
		return commitErr
	}

	// Windows does not provide a portable rename-over-existing contract through
	// os.Root. Move each old target aside first, so installation always renames
	// onto a vacant name and the previous pair remains available for rollback.
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

func rollbackPackOutputs(root *os.Root, outputs []*packOutputState, rename packRenameFunc) error {
	var rollbackErrs []error
	for i := len(outputs) - 1; i >= 0; i-- {
		output := outputs[i]
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
	for i := len(outputs) - 1; i >= 0; i-- {
		output := outputs[i]
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

func readAssetMeta(assetPath string) rawAssetMeta {
	meta, err := readAssetMetaStrict(assetPath)
	if err != nil {
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
			if err == nil && id > 0 {
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
