package KCES

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/aba"
)

// canonicalUnpackAsset 是纯目录解包阶段收集的对象计划 / canonicalUnpackAsset is an object plan collected before pure-directory extraction
type canonicalUnpackAsset struct {
	DirectoryIndex int64           // 源 ABA 目录表索引 / Source ABA directory-table index
	SourceName     string          // 源 SerializedFile 目录名 / Source SerializedFile directory name
	AssetsFile     *aba.AssetsFile // 源 SerializedFile / Source SerializedFile
	Info           *aba.AssetInfo  // 源对象元数据 / Source object metadata
	Entry          aba.AssetEntry  // 源对象提取信息 / Source object extraction information
	Name           string          // 规范资源名 / Canonical asset name
	RelativePath   string          // 纯目录相对路径 / Pure-directory relative path
	GeneratedName  bool            // 名称是否由无名对象摘要生成 / Whether the name was generated from an unnamed-object digest
}

// canonicalUnpackContext 保存跨 SerializedFile 引用、规范 PathID 和流数据消费状态 / canonicalUnpackContext stores cross-file references, canonical PathIDs, and stream-consumption state
type canonicalUnpackContext struct {
	AbaFile         *aba.Aba                                           // 源 UnityFS ABA / Source UnityFS ABA
	Serialized      map[int64]*aba.AssetsFile                          // 目录索引到 SerializedFile / Directory index to SerializedFile
	SerializedNames map[string]*aba.AssetsFile                         // SerializedFile 名称别名 / SerializedFile name aliases
	ContainerNames  map[int64]map[int64]string                         // 源 m_Container 加载名 / Source m_Container load names
	Plans           []*canonicalUnpackAsset                            // 全部输出对象计划 / All output object plans
	BySource        map[canonicalSourceObjectKey]*canonicalUnpackAsset // 源对象到输出计划 / Source object to output plan
	PathIDs         map[string]int64                                   // 规范路径到新 PathID / Canonical path to new PathID
	Streams         map[string]*canonicalStreamUsage                   // 流文件基名到消费状态 / Stream basename to consumption state
	Resolver        aba.AssetResolver                                  // 跨 SerializedFile 对象解析器 / Cross-SerializedFile object resolver
}

// canonicalSourceObjectKey 标识源 SerializedFile 中的对象 / canonicalSourceObjectKey identifies an object in a source SerializedFile
type canonicalSourceObjectKey struct {
	AssetsFile *aba.AssetsFile // 源 SerializedFile / Source SerializedFile
	PathID     int64           // 源对象 PathID / Source object PathID
}

// canonicalStreamUsage 记录非序列化流条目的已消费区间 / canonicalStreamUsage records consumed ranges of a non-serialized stream entry
type canonicalStreamUsage struct {
	DirectoryIndex int64                // ABA 目录表索引 / ABA directory-table index
	Name           string               // ABA 条目名称 / ABA entry name
	Size           int64                // 解压后总字节数 / Total decompressed byte count
	Ranges         []canonicalByteRange // 已消费的半开区间 / Consumed half-open intervals
}

// canonicalByteRange 表示半开字节区间 / canonicalByteRange represents a half-open byte interval
type canonicalByteRange struct {
	Start int64 // 起始偏移 / Start offset
	End   int64 // 结束偏移 / End offset
}

// unpackUnityFSBundlePureDirectory 通过扩展名专用 reader 将 UnityFS 资源包转换为不含 sidecar 和外部流文件的规范资源目录
// unpackUnityFSBundlePureDirectory converts a UnityFS bundle into a canonical directory without sidecars or external stream files through an extension-specific reader
func unpackUnityFSBundlePureDirectory(bundlePath string, outDir string, readBundle func(string) (*aba.Aba, *os.File, error)) error {
	abaf, file, err := readBundle(bundlePath)
	if err != nil {
		return err
	}
	defer file.Close()
	if outDir == "" {
		outDir = bundlePath + "_unpacked"
	}
	ctx, err := buildCanonicalUnpackContext(abaf)
	if err != nil {
		return err
	}
	root, err := openExtractionRoot(outDir)
	if err != nil {
		return err
	}
	defer root.Close()

	claims := make(map[string]string, len(ctx.Plans))
	for _, plan := range ctx.Plans {
		if err := claimExtractionPaths(claims, "asset "+plan.SourceName, plan.RelativePath); err != nil {
			return err
		}
	}
	streamResolver := ctx.streamRangeResolver()
	for _, plan := range ctx.Plans {
		var data []byte
		var err error
		if plan.Entry.TypeId == aba.ClassIDTextAsset {
			_, data, err = plan.AssetsFile.GetTextAssetData(plan.Info)
			if err != nil {
				return fmt.Errorf("read TextAsset %q PathID %d from %q: %w", plan.Name, plan.Entry.PathId, plan.SourceName, err)
			}
		} else {
			object, objectErr := ctx.canonicalNativeUnityObject(plan, streamResolver)
			if objectErr != nil {
				err = objectErr
			} else {
				data, err = aba.EncodeNativeUnityObject(object)
			}
			if err != nil {
				return fmt.Errorf("prepare %s PathID %d from %q: %w", plan.Entry.TypeName, plan.Entry.PathId, plan.SourceName, err)
			}
		}
		if err := root.WriteFile(plan.RelativePath, data, 0644); err != nil {
			return fmt.Errorf("write %q: %w", plan.RelativePath, err)
		}
	}
	if err := ctx.validateStreams(); err != nil {
		return err
	}
	return nil
}

// buildCanonicalUnpackContext 读取全部 SerializedFile 并建立对象图
// buildCanonicalUnpackContext reads all SerializedFiles and builds the object graph
func buildCanonicalUnpackContext(abaf *aba.Aba) (*canonicalUnpackContext, error) {
	if abaf == nil {
		return nil, fmt.Errorf("nil .aba file")
	}
	if err := validateCanonicalSourceUnityVersion(abaf.Header.EngineVersion); err != nil {
		return nil, fmt.Errorf("UnityFS engine version: %w", err)
	}
	ctx := &canonicalUnpackContext{
		AbaFile:         abaf,
		Serialized:      make(map[int64]*aba.AssetsFile),
		SerializedNames: make(map[string]*aba.AssetsFile),
		ContainerNames:  make(map[int64]map[int64]string),
		BySource:        make(map[canonicalSourceObjectKey]*canonicalUnpackAsset),
		Streams:         make(map[string]*canonicalStreamUsage),
	}
	for i, dir := range abaf.BlockInfo.DirectoryInfos {
		directoryIndex := int64(i)
		if _, err := normalizeExtractionPath(dir.Name); err != nil {
			return nil, fmt.Errorf("unsafe .aba entry name %q: %w", dir.Name, err)
		}
		if dir.IsSerialized() {
			af, err := aba.ReadAssetsFileRange(dir.DecompressedSize, func(offset int64, size int64) ([]byte, error) {
				return abaf.GetFileDataRange(directoryIndex, offset, size)
			})
			if err != nil {
				return nil, fmt.Errorf("parse SerializedFile %q: %w", dir.Name, err)
			}
			if af.Header.Version != supportedSerializedFileVersion {
				return nil, fmt.Errorf("SerializedFile %q uses version %d; pure-directory ABA unpacking requires version %d", dir.Name, af.Header.Version, supportedSerializedFileVersion)
			}
			if err := validateCanonicalSourceUnityVersion(af.Metadata.UnityVersion); err != nil {
				return nil, fmt.Errorf("SerializedFile %q: %w", dir.Name, err)
			}
			ctx.Serialized[directoryIndex] = af
			if err := addSerializedAlias(ctx.SerializedNames, dir.Name, af); err != nil {
				return nil, err
			}
			if err := addSerializedAlias(ctx.SerializedNames, filepath.Base(dir.Name), af); err != nil {
				return nil, err
			}
			containerNames, err := af.GetAssetBundleContainerMap()
			if err != nil {
				return nil, fmt.Errorf("read AssetBundle container map from %q: %w", dir.Name, err)
			}
			ctx.ContainerNames[directoryIndex] = containerNames
			continue
		}
		if dir.DecompressedSize < 0 {
			return nil, fmt.Errorf("non-serialized .aba entry %q has negative size", dir.Name)
		}
		if isStreamDirectoryName(dir.Name) {
			key := strings.ToLower(path.Base(strings.ReplaceAll(dir.Name, "\\", "/")))
			if _, exists := ctx.Streams[key]; exists {
				return nil, fmt.Errorf("non-serialized stream entries %q and %q have the same basename", ctx.Streams[key].Name, dir.Name)
			}
			ctx.Streams[key] = &canonicalStreamUsage{DirectoryIndex: directoryIndex, Name: dir.Name, Size: dir.DecompressedSize}
			continue
		}
		if dir.DecompressedSize != 0 {
			return nil, fmt.Errorf("non-serialized .aba entry %q is not a supported .resS/.resource stream and cannot be represented in the pure directory", dir.Name)
		}
	}

	serializedIndexes := make([]int64, 0, len(ctx.Serialized))
	for directoryIndex := range ctx.Serialized {
		serializedIndexes = append(serializedIndexes, directoryIndex)
	}
	sort.Slice(serializedIndexes, func(i, j int) bool { return serializedIndexes[i] < serializedIndexes[j] })
	for _, directoryIndex := range serializedIndexes {
		af := ctx.Serialized[directoryIndex]
		dir := abaf.BlockInfo.DirectoryInfos[directoryIndex]
		entries := af.GetAssetEntries()
		for entryIndex := range entries {
			entry := entries[entryIndex]
			if entry.TypeId == aba.ClassIDAssetBundle {
				continue
			}
			name := entry.Name
			generatedName := false
			if name == "" {
				loadName := ctx.ContainerNames[directoryIndex][entry.PathId]
				if loadName != "" {
					name = path.Base(strings.ReplaceAll(loadName, "\\", "/"))
				}
				if name == "" {
					name = stableUnnamedAssetName(af, af.GetAssetInfoByPathID(entry.PathId), int64(entryIndex))
					generatedName = true
				}
			}
			name = sanitizeName(name)
			plan := &canonicalUnpackAsset{
				DirectoryIndex: directoryIndex,
				SourceName:     dir.Name,
				AssetsFile:     af,
				Info:           af.GetAssetInfoByPathID(entry.PathId),
				Entry:          entry,
				Name:           name,
				GeneratedName:  generatedName,
			}
			if plan.Info == nil {
				return nil, fmt.Errorf("PathID %d from %q has no AssetInfo", entry.PathId, dir.Name)
			}
			ctx.Plans = append(ctx.Plans, plan)
			key := canonicalSourceObjectKey{AssetsFile: af, PathID: entry.PathId}
			if _, exists := ctx.BySource[key]; exists {
				return nil, fmt.Errorf("duplicate source object PathID %d in %q", entry.PathId, dir.Name)
			}
			ctx.BySource[key] = plan
		}
	}
	if err := assignCanonicalUnpackPaths(ctx.Plans); err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(ctx.Plans))
	for _, plan := range ctx.Plans {
		paths = append(paths, plan.RelativePath)
	}
	pathIDs, err := buildCanonicalPathIDs(paths)
	if err != nil {
		return nil, fmt.Errorf("build canonical PathIDs: %w", err)
	}
	ctx.PathIDs = pathIDs
	for _, plan := range ctx.Plans {
		if plan.Entry.TypeId == aba.ClassIDTextAsset || plan.AssetsFile.AssetHasTypeTree(plan.Info) {
			continue
		}
		return nil, fmt.Errorf("SerializedFile %q object %q has no TypeTree and cannot be represented as a standalone native file", plan.SourceName, plan.RelativePath)
	}
	ctx.Resolver = aba.AbaAssetResolver(ctx.SerializedNames)
	return ctx, nil
}

// assignCanonicalUnpackPaths 优先保留互不冲突的原始路径，并为全部撞名对象追加确定性序号
// assignCanonicalUnpackPaths preserves every non-conflicting original path first and appends deterministic ordinals to all colliding objects
func assignCanonicalUnpackPaths(plans []*canonicalUnpackAsset) error {
	plannedPaths := make(map[string]string, len(plans))
	pending := make([]*canonicalUnpackAsset, 0)
	for _, generatedName := range []bool{false, true} {
		for _, plan := range plans {
			if plan == nil || plan.GeneratedName != generatedName {
				continue
			}
			relPath, err := canonicalUnpackRelativePath(plan.Entry.TypeId, plan.Entry.TypeName, plan.Name)
			if err != nil {
				return fmt.Errorf("plan object PathID %d from %q: %w", plan.Entry.PathId, plan.SourceName, err)
			}
			pathKey := strings.ToLower(filepath.ToSlash(relPath))
			if _, exists := plannedPaths[pathKey]; exists {
				pending = append(pending, plan)
				continue
			}
			plannedPaths[pathKey] = canonicalUnpackPlanOwner(plan)
			plan.RelativePath = relPath
		}
	}
	for _, plan := range pending {
		baseName := plan.Name
		owner := canonicalUnpackPlanOwner(plan)
		for suffix := int64(2); ; suffix++ {
			candidateName := canonicalOrdinalAssetName(baseName, suffix)
			relPath, err := canonicalUnpackRelativePath(plan.Entry.TypeId, plan.Entry.TypeName, candidateName)
			if err != nil {
				return fmt.Errorf("plan object PathID %d from %q: %w", plan.Entry.PathId, plan.SourceName, err)
			}
			pathKey := strings.ToLower(filepath.ToSlash(relPath))
			if _, exists := plannedPaths[pathKey]; !exists {
				plannedPaths[pathKey] = owner
				plan.Name = candidateName
				plan.RelativePath = relPath
				break
			}
			if suffix == math.MaxInt64 {
				return fmt.Errorf("cannot assign a unique output path for %s", owner)
			}
		}
	}
	return nil
}

// canonicalUnpackPlanOwner 返回用于路径冲突诊断的对象描述
// canonicalUnpackPlanOwner returns an object description used in path-conflict diagnostics
func canonicalUnpackPlanOwner(plan *canonicalUnpackAsset) string {
	if plan == nil {
		return "invalid object plan"
	}
	return fmt.Sprintf("%s PathID %d from %s", plan.Entry.TypeName, plan.Entry.PathId, plan.SourceName)
}

// addSerializedAlias 添加 SerializedFile 名称别名并拒绝指向不同文件的歧义
// addSerializedAlias adds a SerializedFile alias and rejects ambiguity across different files
func addSerializedAlias(aliases map[string]*aba.AssetsFile, name string, af *aba.AssetsFile) error {
	key := strings.ToLower(filepath.ToSlash(name))
	if previous, exists := aliases[key]; exists && previous != af {
		return fmt.Errorf("SerializedFile alias %q refers to multiple files", name)
	}
	aliases[key] = af
	return nil
}

// stableUnnamedAssetName 为没有 m_Name 和 loadName 的对象生成与 PPtr 目标无关的稳定名称
// stableUnnamedAssetName creates a stable name independent of PPtr targets for objects without m_Name or loadName
func stableUnnamedAssetName(af *aba.AssetsFile, info *aba.AssetInfo, fallbackIndex int64) string {
	if af != nil && info != nil && af.AssetHasTypeTree(info) {
		if root, err := af.ReadAssetValue(info); err == nil {
			var fingerprint strings.Builder
			appendStableValueFingerprint(&fingerprint, root)
			sum := sha256.Sum256([]byte(fingerprint.String()))
			return "unnamed_" + hex.EncodeToString(sum[:6])
		}
	}
	if af != nil && info != nil {
		if data, err := af.GetAssetData(info); err == nil {
			sum := sha256.Sum256(data)
			return "unnamed_" + hex.EncodeToString(sum[:6])
		}
	}
	return fmt.Sprintf("unnamed_%04d", fallbackIndex)
}

// appendStableValueFingerprint 将 TypeTree 值编码为稳定摘要输入并忽略 PPtr 目标
// appendStableValueFingerprint encodes a TypeTree value into stable digest input while ignoring PPtr targets
func appendStableValueFingerprint(out *strings.Builder, value *aba.TypeTreeValue) {
	if value == nil {
		out.WriteString("<nil>")
		return
	}
	if strings.HasPrefix(strings.TrimSpace(value.TypeName), "PPtr<") {
		out.WriteString("PPtr")
		return
	}
	out.WriteString(value.TypeName)
	out.WriteByte(0)
	out.WriteString(value.Name)
	out.WriteByte(0)
	switch raw := value.Value.(type) {
	case []byte:
		sum := sha256.Sum256(raw)
		out.WriteString(hex.EncodeToString(sum[:]))
	default:
		out.WriteString(fmt.Sprintf("%T:%v", raw, raw))
	}
	for _, child := range value.Children {
		appendStableValueFingerprint(out, child)
	}
}

// classIDCanBeReidentifiedWithoutTypeTree 判断对象布局是否不含需要重写的 PPtr
// classIDCanBeReidentifiedWithoutTypeTree reports whether an object layout has no PPtrs that require rewriting
func classIDCanBeReidentifiedWithoutTypeTree(classID int32) bool {
	switch classID {
	case aba.ClassIDTextAsset, aba.ClassIDTexture2D, aba.ClassIDMesh, aba.ClassIDAudioClip, aba.ClassIDMonoScript:
		return true
	default:
		return false
	}
}

// canonicalUnpackRelativePath 根据 ClassID 生成稳定的纯目录文件名
// canonicalUnpackRelativePath creates a stable pure-directory filename from a ClassID
func canonicalUnpackRelativePath(classID int32, typeName string, name string) (string, error) {
	safe := sanitizeName(name)
	var rel string
	switch classID {
	case aba.ClassIDTextAsset:
		rel = filepath.Join("TextAsset", safe)
	case aba.ClassIDTexture2D:
		rel = filepath.Join("Texture2D", canonicalTexture2DFileName(safe))
	case aba.ClassIDSprite:
		rel = filepath.Join("Sprite", canonicalSuffixedFileName(safe, ".sprite"))
	case aba.ClassIDSpriteAtlas:
		rel = filepath.Join("SpriteAtlas", canonicalIntrinsicFileName(safe, ".partsatlas", ".partsassets"))
	case aba.ClassIDMesh:
		rel = filepath.Join("Mesh", canonicalIntrinsicFileName(safe, ".mmesh"))
	case aba.ClassIDAnimationClip:
		rel = filepath.Join("AnimationClip", canonicalIntrinsicFileName(safe, ".anm"))
	case aba.ClassIDMaterial:
		rel = filepath.Join("Material", canonicalSuffixedFileName(safe, ".material"))
	case aba.ClassIDAudioClip:
		rel = filepath.Join("AudioClip", canonicalSuffixedFileName(safe, ".audioclip"))
	case aba.ClassIDMonoBehaviour:
		rel = filepath.Join("MonoBehaviour", canonicalSuffixedFileName(safe, ".monobehaviour"))
	default:
		if typeName == "" {
			typeName = fmt.Sprintf("Type_%d", classID)
		}
		rel = filepath.Join(sanitizeName(typeName), safe+".bytes")
	}
	return normalizeExtractionPath(rel)
}

// canonicalTexture2DFileName 为 Texture2D 选择原有 .tex 名称或追加无歧义的 .texture2d 后缀
// canonicalTexture2DFileName keeps an existing .tex name or appends an unambiguous .texture2d suffix for a Texture2D
func canonicalTexture2DFileName(name string) string {
	if strings.HasSuffix(strings.ToLower(name), ".tex") {
		return name
	}
	return canonicalSuffixedFileName(name, ".texture2d")
}

// canonicalIntrinsicFileName 保留游戏资源本身已有的扩展名，否则追加首选扩展名
// canonicalIntrinsicFileName preserves an extension already intrinsic to the game resource or appends the preferred extension
func canonicalIntrinsicFileName(name string, extensions ...string) string {
	lower := strings.ToLower(name)
	for _, extension := range extensions {
		if strings.HasSuffix(lower, strings.ToLower(extension)) {
			return name
		}
	}
	if len(extensions) == 0 {
		return name
	}
	return name + extensions[0]
}

// canonicalSuffixedFileName 为工具原生对象追加一个用于打包时恢复资源名的类型后缀
// canonicalSuffixedFileName appends a type suffix that packing removes to restore the resource name
func canonicalSuffixedFileName(name string, suffix string) string {
	return name + suffix
}

// isStreamDirectoryName 判断 ABA 条目是否为必须内联的流式资源
// isStreamDirectoryName reports whether an ABA entry is a stream resource that must be inlined
func isStreamDirectoryName(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".ress" || ext == ".resource" || ext == ".resources"
}

// streamRangeResolver 返回带消费记录的流式范围读取器
// streamRangeResolver returns a range reader that records consumed intervals
func (ctx *canonicalUnpackContext) streamRangeResolver() aba.AbaFileRangeResolver {
	return func(name string, offset int64, size int64) ([]byte, error) {
		key := strings.ToLower(path.Base(strings.ReplaceAll(name, "\\", "/")))
		usage := ctx.Streams[key]
		if usage == nil {
			return nil, fmt.Errorf("stream entry %q is not present in the ABA", name)
		}
		end := offset + size
		if offset < 0 || size < 0 || end < offset || end > usage.Size {
			return nil, fmt.Errorf("stream range %q[%d,%d) is outside %d bytes", usage.Name, offset, end, usage.Size)
		}
		if size > 0 {
			usage.Ranges = append(usage.Ranges, canonicalByteRange{Start: offset, End: end})
		}
		return ctx.AbaFile.GetFileDataRange(usage.DirectoryIndex, offset, size)
	}
}

// canonicalNativeUnityObject 内联外部流数据、重写对象引用并生成自带 Unity 2022.3 TypeTree 的独立对象
// canonicalNativeUnityObject inlines external stream data, rewrites object references, and creates a standalone object carrying its Unity 2022.3 TypeTree
func (ctx *canonicalUnpackContext) canonicalNativeUnityObject(plan *canonicalUnpackAsset, streamResolver aba.AbaFileRangeResolver) (*aba.NativeUnityObject, error) {
	if plan == nil || plan.AssetsFile == nil || plan.Info == nil {
		return nil, fmt.Errorf("invalid canonical asset plan")
	}
	if !plan.AssetsFile.AssetHasTypeTree(plan.Info) {
		return nil, fmt.Errorf("class %d object has no TypeTree and cannot become a standalone native file", plan.Entry.TypeId)
	}
	raw, err := plan.AssetsFile.GetAssetData(plan.Info)
	if err != nil {
		return nil, err
	}
	data := append([]byte(nil), raw...)
	streamChanged := false
	switch plan.Entry.TypeId {
	case aba.ClassIDTexture2D:
		data, streamChanged, err = plan.AssetsFile.InlineTexture2DStreamData(plan.Info, streamResolver)
	case aba.ClassIDCubemap:
		data, streamChanged, err = plan.AssetsFile.InlineCubemapStreamData(plan.Info, streamResolver)
	case aba.ClassIDMesh:
		data, streamChanged, err = plan.AssetsFile.InlineMeshStreamData(plan.Info, streamResolver)
	case aba.ClassIDAudioClip:
		data, streamChanged, err = plan.AssetsFile.InlineAudioClipStreamData(plan.Info, streamResolver)
	}
	if err != nil {
		return nil, err
	}
	tree, err := plan.AssetsFile.AssetTypeTree(plan.Info)
	if err != nil {
		return nil, err
	}
	if plan.Entry.TypeId == aba.ClassIDAudioClip {
		return &aba.NativeUnityObject{ClassID: plan.Entry.TypeId, BigEndian: plan.AssetsFile.Header.Endianness, TypeTree: tree, Data: data}, nil
	}
	clone := *plan.AssetsFile
	clone.Data = data
	clone.Header.DataOffset = 0
	clone.Header.FileSize = int64(len(data))
	clone.Header.MetadataSize = 0
	if uint64(len(data)) > uint64(^uint32(0)) {
		return nil, fmt.Errorf("object data length %d exceeds SerializedFile uint32 object-size range", len(data))
	}
	info := *plan.Info
	info.ByteOffset = 0
	info.ByteSize = uint32(len(data))
	root, err := clone.ReadAssetValue(&info)
	if err != nil {
		return nil, fmt.Errorf("decode object with TypeTree: %w", err)
	}
	var pptrCount int64
	if !classIDCanBeReidentifiedWithoutTypeTree(plan.Entry.TypeId) {
		pptrCount, err = aba.RewritePPtrReferences(root, func(fileID int32, pathID int64) (int32, int64, error) {
			return ctx.remapPPtr(plan, fileID, pathID)
		})
		if err != nil {
			return nil, err
		}
	}
	encoded, targetTree, schemaChanged, err := clone.EncodeAssetValueAndTypeTreeForUnity2022(&info, root)
	if err != nil {
		return nil, fmt.Errorf("migrate class %d object to Unity 2022.3: %w", plan.Entry.TypeId, err)
	}
	if pptrCount == 0 && !streamChanged && !schemaChanged {
		encoded = data
		targetTree = tree
	}
	return &aba.NativeUnityObject{ClassID: plan.Entry.TypeId, BigEndian: plan.AssetsFile.Header.Endianness, TypeTree: targetTree, Data: encoded}, nil
}

// remapPPtr 将源文件引用扁平化为当前输出 SerializedFile 的规范 PathID
// remapPPtr flattens a source reference to a canonical PathID in the output SerializedFile
func (ctx *canonicalUnpackContext) remapPPtr(from *canonicalUnpackAsset, fileID int32, pathID int64) (int32, int64, error) {
	if pathID == 0 {
		return 0, 0, nil
	}
	if from == nil || from.AssetsFile == nil {
		return 0, 0, fmt.Errorf("source object plan is unavailable")
	}
	if fileID > 0 {
		externalIndex := int64(fileID) - 1
		if externalIndex < int64(len(from.AssetsFile.Metadata.ExternalFiles)) {
			external := from.AssetsFile.Metadata.ExternalFiles[externalIndex]
			canonicalFileID, matched, err := canonicalKCESBuiltInExternalFileID(external)
			if err != nil {
				return 0, 0, fmt.Errorf("external PPtr fileID=%d PathID=%d: %w", fileID, pathID, err)
			}
			if matched {
				return canonicalFileID, pathID, nil
			}
		}
	}
	if ctx.Resolver == nil {
		return 0, 0, fmt.Errorf("source resolver is unavailable")
	}
	targetFile, targetInfo, err := ctx.Resolver(from.AssetsFile, fileID, pathID)
	if err != nil {
		return 0, 0, err
	}
	target := ctx.BySource[canonicalSourceObjectKey{AssetsFile: targetFile, PathID: targetInfo.PathId}]
	if target == nil {
		return 0, 0, fmt.Errorf("PPtr fileID=%d PathID=%d targets an omitted or unsupported object", fileID, pathID)
	}
	canonicalPath, err := canonicalAssetPathForID(target.RelativePath)
	if err != nil {
		return 0, 0, err
	}
	newPathID, ok := ctx.PathIDs[canonicalPath]
	if !ok || newPathID == 0 {
		return 0, 0, fmt.Errorf("no canonical PathID for %q", target.RelativePath)
	}
	return 0, newPathID, nil
}

// canonicalKCESExternalFiles 返回纯目录输出固定使用的 Unity 内置外部资源表
// canonicalKCESExternalFiles returns the fixed Unity built-in external-resource table used by pure-directory output
func canonicalKCESExternalFiles() []aba.ExternalFile {
	return []aba.ExternalFile{
		{Guid: [16]byte{8: 0x0f}, Type: 0, PathName: "Resources/unity_builtin_extra"},
		{Guid: [16]byte{8: 0x0e}, Type: 0, PathName: "library/unity default resources"},
	}
}

// canonicalKCESBuiltInExternalFileID 将 Unity 内置外部资源别名映射为纯目录输出的固定 fileID
// canonicalKCESBuiltInExternalFileID maps Unity built-in external-resource aliases to the fixed fileID used by pure-directory output
func canonicalKCESBuiltInExternalFileID(external aba.ExternalFile) (int32, bool, error) {
	name := strings.ToLower(path.Base(strings.ReplaceAll(strings.TrimSpace(external.PathName), "\\", "/")))
	var canonicalFileID int32
	switch name {
	case "unity_builtin_extra", "unity builtin extra":
		canonicalFileID = 1
	case "unity_default_resources", "unity default resources":
		canonicalFileID = 2
	default:
		return 0, false, nil
	}
	expected := canonicalKCESExternalFiles()[int64(canonicalFileID)-1]
	if external.Type != expected.Type {
		return 0, true, fmt.Errorf("Unity built-in external resource %q has type %d, want %d", external.PathName, external.Type, expected.Type)
	}
	var zeroGUID [16]byte
	if external.Guid != zeroGUID && external.Guid != expected.Guid {
		return 0, true, fmt.Errorf("Unity built-in external resource %q has GUID %x, want %x", external.PathName, external.Guid, expected.Guid)
	}
	return canonicalFileID, true, nil
}

// validateStreams 确认所有流式载荷已消费，纯零尾部允许作为对齐填充
// validateStreams confirms that every stream payload was consumed and permits only all-zero alignment tails
func (ctx *canonicalUnpackContext) validateStreams() error {
	for _, usage := range ctx.Streams {
		if usage.Size == 0 {
			continue
		}
		ranges := append([]canonicalByteRange(nil), usage.Ranges...)
		sort.Slice(ranges, func(i, j int) bool {
			if ranges[i].Start != ranges[j].Start {
				return ranges[i].Start < ranges[j].Start
			}
			return ranges[i].End < ranges[j].End
		})
		cursor := int64(0)
		for _, r := range ranges {
			if r.Start > cursor {
				if err := ctx.requireZeroStreamRange(usage, cursor, r.Start); err != nil {
					return err
				}
			}
			if r.End > cursor {
				cursor = r.End
			}
		}
		if cursor < usage.Size {
			if err := ctx.requireZeroStreamRange(usage, cursor, usage.Size); err != nil {
				return err
			}
		}
	}
	return nil
}

// requireZeroStreamRange 检查未消费范围是否仅由零填充构成
// requireZeroStreamRange checks that an unconsumed range contains only zero padding
func (ctx *canonicalUnpackContext) requireZeroStreamRange(usage *canonicalStreamUsage, start int64, end int64) error {
	const chunkSize int64 = 1 << 20
	for cursor := start; cursor < end; {
		size := end - cursor
		if size > chunkSize {
			size = chunkSize
		}
		data, err := ctx.AbaFile.GetFileDataRange(usage.DirectoryIndex, cursor, size)
		if err != nil {
			return fmt.Errorf("read unconsumed stream range %q[%d,%d): %w", usage.Name, cursor, cursor+size, err)
		}
		for _, b := range data {
			if b != 0 {
				return fmt.Errorf("stream entry %q has unconsumed non-zero data in range [%d,%d); object type is not supported for .resS/.resource splitting", usage.Name, start, end)
			}
		}
		cursor += size
	}
	return nil
}
