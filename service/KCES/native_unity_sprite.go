package KCES

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/aba"
)

// nativeUnityDirectoryAsset 保存一个按规范 PathID 索引的独立对象视图 / nativeUnityDirectoryAsset stores a standalone object view indexed by its canonical PathID
type nativeUnityDirectoryAsset struct {
	assetsFile *aba.AssetsFile        // 单对象 SerializedFile 视图 / One-object SerializedFile view
	assetInfo  *aba.AssetInfo         // 使用规范 PathID 的对象信息 / Object information using the canonical PathID
	object     *aba.NativeUnityObject // 带完整 TypeTree 的独立对象 / Standalone object with its complete TypeTree
}

// nativeUnityDirectoryIndex 为纯目录中的独立对象提供延迟加载 PPtr 解析 / nativeUnityDirectoryIndex provides lazy PPtr resolution for standalone objects in a pure directory
type nativeUnityDirectoryIndex struct {
	root      string                               // 纯目录根路径 / Pure-directory root path
	paths     map[int64]string                     // 规范 PathID 到主文件路径的映射 / Canonical PathID to primary-file path mapping
	classIDs  map[int64]int32                      // 规范 PathID 到实际 Unity ClassID 的映射 / Canonical PathID to actual Unity ClassID mapping
	cache     map[int64]*nativeUnityDirectoryAsset // 已加载对象缓存 / Loaded object cache
	cacheLock sync.Mutex                           // 缓存互斥锁 / Cache mutex
}

// ConvertSpriteToPNG 从 Sprite 主文件所在纯目录解析 Texture2D 或 SpriteAtlas 引用并导出 PNG
// ConvertSpriteToPNG resolves Texture2D or SpriteAtlas references from the Sprite primary file's pure directory and exports PNG
func (s *NativeUnityMediaService) ConvertSpriteToPNG(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	index, pathID, err := newNativeUnityDirectoryIndex(inputPath)
	if err != nil {
		return fmt.Errorf("index native Unity directory for %q: %w", inputPath, err)
	}
	assetsFile, assetInfo, err := index.spriteView(pathID)
	if err != nil {
		return fmt.Errorf("load native Sprite %q: %w", inputPath, err)
	}
	if assetInfo.TypeId != aba.ClassIDSprite {
		return fmt.Errorf("native Unity object %q is ClassID %d instead of Sprite", inputPath, assetInfo.TypeId)
	}
	sprite, err := assetsFile.GetSpriteExportRange(assetInfo, index.resolve, nil)
	if err != nil {
		return fmt.Errorf("resolve native Sprite %q: %w", inputPath, err)
	}
	var output bytes.Buffer
	if err := aba.WriteSpritePNGTo(sprite, &output); err != nil {
		return fmt.Errorf("encode native Sprite %q as PNG: %w", inputPath, err)
	}
	return writeNativeUnityMediaOutput(ctx, outputPath, output.Bytes(), maxOutputBytes)
}

// newNativeUnityDirectoryIndex 重建纯目录规范 PathID 表并返回输入主文件的 PathID
// newNativeUnityDirectoryIndex rebuilds the pure directory's canonical PathID table and returns the input primary file's PathID
func newNativeUnityDirectoryIndex(inputPath string) (*nativeUnityDirectoryIndex, int64, error) {
	absoluteInput, err := filepath.Abs(inputPath)
	if err != nil {
		return nil, 0, err
	}
	typeDirectory := filepath.Dir(absoluteInput)
	root := filepath.Dir(typeDirectory)
	if root == typeDirectory || filepath.Base(typeDirectory) == "." {
		return nil, 0, fmt.Errorf("input is not below a Unity type directory")
	}
	var relativePaths []string
	err = filepath.Walk(root, func(filePath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		if isLinkOrReparse(info) {
			return fmt.Errorf("refusing symlink or reparse point %q", filePath)
		}
		if !info.Mode().IsRegular() || shouldSkipPackInput(filePath, "") {
			return nil
		}
		relativePath, err := filepath.Rel(root, filePath)
		if err != nil {
			return err
		}
		relativePaths = append(relativePaths, filepath.ToSlash(relativePath))
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	pathIDs, err := buildCanonicalPathIDs(relativePaths)
	if err != nil {
		return nil, 0, err
	}
	index := &nativeUnityDirectoryIndex{
		root:     root,
		paths:    make(map[int64]string),
		classIDs: make(map[int64]int32),
		cache:    make(map[int64]*nativeUnityDirectoryAsset),
	}
	for _, relativePath := range relativePaths {
		kind, ok := unityRawKindForPackPath(relativePath)
		if !ok || !isNativeUnityObjectPackPath(relativePath, kind) {
			continue
		}
		canonicalPath, err := canonicalAssetPathForID(relativePath)
		if err != nil {
			return nil, 0, err
		}
		pathID := pathIDs[canonicalPath]
		objectPath := filepath.Join(root, filepath.FromSlash(relativePath))
		header, err := readNativeUnityObjectFileHeader(objectPath)
		if err != nil {
			return nil, 0, fmt.Errorf("read native Unity object header %q: %w", objectPath, err)
		}
		index.paths[pathID] = objectPath
		index.classIDs[pathID] = header.ClassID
	}
	inputRelativePath, err := filepath.Rel(root, absoluteInput)
	if err != nil {
		return nil, 0, err
	}
	canonicalInput, err := canonicalAssetPathForID(filepath.ToSlash(inputRelativePath))
	if err != nil {
		return nil, 0, err
	}
	inputPathID := pathIDs[canonicalInput]
	if inputPathID == 0 || index.paths[inputPathID] == "" {
		return nil, 0, fmt.Errorf("input is not a recognized native Unity primary file")
	}
	return index, inputPathID, nil
}

// spriteView 将目标 Sprite 和目录内全部 SpriteAtlas 重建为同一个临时 SerializedFile 视图
// spriteView rebuilds the target Sprite and every SpriteAtlas in the directory into one temporary SerializedFile view
func (index *nativeUnityDirectoryIndex) spriteView(spritePathID int64) (*aba.AssetsFile, *aba.AssetInfo, error) {
	if index == nil {
		return nil, nil, fmt.Errorf("nil native Unity directory index")
	}
	if index.classIDs[spritePathID] != aba.ClassIDSprite {
		return nil, nil, fmt.Errorf("native Unity object PathID %d is ClassID %d instead of Sprite", spritePathID, index.classIDs[spritePathID])
	}
	pathIDs := []int64{spritePathID}
	for pathID, classID := range index.classIDs {
		if classID == aba.ClassIDSpriteAtlas {
			pathIDs = append(pathIDs, pathID)
		}
	}
	sort.Slice(pathIDs, func(left int, right int) bool {
		return pathIDs[left] < pathIDs[right]
	})

	writer := aba.NewSerializedFileWriter(defaultKCESUnityVersion)
	for _, pathID := range pathIDs {
		asset, err := index.load(pathID)
		if err != nil {
			return nil, nil, err
		}
		relativePath, err := filepath.Rel(index.root, index.paths[pathID])
		if err != nil {
			return nil, nil, err
		}
		loadName := filepath.ToSlash(relativePath)
		writer.AddNativeUnityObjectWithLoadNameAndPathID(asset.object, inferAssetNameForPack(loadName), loadName, pathID)
	}
	var output bytes.Buffer
	if err := writer.Write(&output); err != nil {
		return nil, nil, fmt.Errorf("build temporary Sprite and SpriteAtlas view: %w", err)
	}
	assetsFile, err := aba.ReadAssetsFile(output.Bytes())
	if err != nil {
		return nil, nil, fmt.Errorf("read temporary Sprite and SpriteAtlas view: %w", err)
	}
	assetInfo := assetsFile.GetAssetInfoByPathID(spritePathID)
	if assetInfo == nil {
		return nil, nil, fmt.Errorf("temporary Sprite and SpriteAtlas view is missing Sprite PathID %d", spritePathID)
	}
	return assetsFile, assetInfo, nil
}

// load 延迟读取独立对象并把单对象视图的 PathID 改为纯目录规范值
// load lazily reads a standalone object and changes its one-object view PathID to the pure-directory canonical value
func (index *nativeUnityDirectoryIndex) load(pathID int64) (*nativeUnityDirectoryAsset, error) {
	if index == nil {
		return nil, fmt.Errorf("nil native Unity directory index")
	}
	index.cacheLock.Lock()
	defer index.cacheLock.Unlock()
	if cached := index.cache[pathID]; cached != nil {
		return cached, nil
	}
	path := index.paths[pathID]
	if path == "" {
		return nil, fmt.Errorf("native Unity object PathID %d is absent from %q", pathID, index.root)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	object, err := aba.ReadNativeUnityObject(data)
	if err != nil {
		return nil, err
	}
	assetsFile, _, err := object.AssetsFileView()
	if err != nil {
		return nil, err
	}
	if len(assetsFile.Metadata.AssetInfos) != 1 {
		return nil, fmt.Errorf("native Unity object view contains %d objects instead of one", len(assetsFile.Metadata.AssetInfos))
	}
	assetsFile.Metadata.AssetInfos[0].PathId = pathID
	asset := &nativeUnityDirectoryAsset{assetsFile: assetsFile, assetInfo: &assetsFile.Metadata.AssetInfos[0], object: object}
	index.cache[pathID] = asset
	return asset, nil
}

// resolve 按纯目录规范 PathID 解析已扁平化的 Unity PPtr
// resolve resolves flattened Unity PPtrs by their pure-directory canonical PathID
func (index *nativeUnityDirectoryIndex) resolve(relativeTo *aba.AssetsFile, fileID int32, pathID int64) (*aba.AssetsFile, *aba.AssetInfo, error) {
	if pathID == 0 {
		return nil, nil, fmt.Errorf("null PPtr")
	}
	if fileID != 0 {
		return nil, nil, fmt.Errorf("external PPtr fileID=%d PathID=%d cannot be resolved from a pure directory", fileID, pathID)
	}
	if relativeTo != nil {
		if info := relativeTo.GetAssetInfoByPathID(pathID); info != nil {
			return relativeTo, info, nil
		}
	}
	asset, err := index.load(pathID)
	if err != nil {
		return nil, nil, err
	}
	return asset.assetsFile, asset.assetInfo, nil
}
