package aba

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio"
)

// SerializedFileWriter 生成包含 TextAsset、Texture2D、原始对象和 AssetBundle 容器的 Unity SerializedFile v22 / SerializedFileWriter generates Unity SerializedFile v22 data containing TextAsset, Texture2D, raw objects, and an AssetBundle container
type SerializedFileWriter struct {
	UnityVersion   string             // Unity 版本字符串，默认固定为 2022.3.35f1 / Unity version string, fixed to 2022.3.35f1 by default
	TargetPlatform uint32             // 目标平台 ID，5 表示 Windows Standalone / Target platform ID, with 5 meaning Windows Standalone
	objects        []sfObject         // 待写入的对象列表 / Object list to write
	nextPathId     int64              // 下一次自动分配的 PathID / Next automatically allocated PathID
	usedPathIds    map[int64]struct{} // 已占用的 PathID 集合 / Set of already used PathIDs
	externalFiles  []ExternalFile     // metadata 中按 fileID 顺序写入的外部文件 / External files written to metadata in fileID order
	err            error              // 延迟到 Write 返回的构建错误 / Build error deferred until Write
}

// SerializedObjectWriteFunc 将一个对象的完整序列化字节写入目标
// SerializedObjectWriteFunc writes the complete serialized bytes of one object to a destination
type SerializedObjectWriteFunc func(io.Writer) error

// SerializedObjectDataSource 描述无需整体载入内存的序列化对象数据源 / SerializedObjectDataSource describes a serialized object source that does not need to be loaded completely into memory
type SerializedObjectDataSource struct {
	Size    uint32                    // 对象在线格式字节数 / Object byte size on the wire
	Prefix  []byte                    // 用于读取稳定基类字段的对象前缀 / Object prefix used to inspect stable base fields
	WriteTo SerializedObjectWriteFunc // 每次调用都写出恰好 Size 字节的回调 / Callback that writes exactly Size bytes on every call
}

// sfObject 表示待写入的一个序列化对象 / sfObject represents one serialized object to write
type sfObject struct {
	pathId       int64                     // 对象 PathID / Object PathID
	classId      int32                     // Unity 类 ID / Unity class ID
	scriptPathId int64                     // MonoBehaviour 的 m_Script 目标 PathID，其他类型为零 / MonoBehaviour m_Script target PathID, or zero for other classes
	name         string                    // 对象内部 m_Name / Internal object m_Name
	loadName     string                    // AssetBundle m_Container 加载名 / AssetBundle m_Container load name
	data         []byte                    // 已在内存中的序列化对象数据 / Serialized object data already held in memory
	dataSize     uint32                    // 对象在线格式字节数 / Object byte size on the wire
	writeData    SerializedObjectWriteFunc // 流式对象写入回调 / Streaming object writer callback
	typeTree     *TypeTreeType             // 可选的完整对象 TypeTree / Optional complete object TypeTree
}

// sfSerializedType 表示写入 metadata 的一个 SerializedType / sfSerializedType represents one SerializedType emitted into metadata
type sfSerializedType struct {
	classId         int32         // Unity 类 ID / Unity class ID
	scriptTypeIndex int16         // MonoBehaviour 对应的 ScriptTypes 索引，非脚本类型为 -1 / ScriptTypes index for a MonoBehaviour, or -1 for non-script types
	scriptPathId    int64         // 用于区分同一 class ID 下不同脚本类型的 MonoScript PathID / MonoScript PathID distinguishing script types that share one class ID
	scriptIdHash    [16]byte      // 脚本 ID 哈希，新建无 TypeTree 文件允许使用零值 / Script ID hash, which may be zero for a newly built file without TypeTree data
	typeHash        [16]byte      // 类型树哈希，新建无 TypeTree 文件允许使用零值 / Type-tree hash, which may be zero for a newly built file without TypeTree data
	isStrippedType  bool          // 类型是否标记为已剥离 / Whether the type is marked as stripped
	typeTree        *TypeTreeType // 可选的完整对象 TypeTree / Optional complete object TypeTree
}

const (
	sfVersion uint32 = 22
	// defaultPlatform 是 Windows Standalone 的 TargetPlatform 值
	// defaultPlatform is the TargetPlatform value for Windows Standalone
	defaultPlatform uint32 = 5
)

// NewSerializedFileWriter 创建一个新的 SerializedFile v22 写入器并验证 Unity 版本
// NewSerializedFileWriter creates a new SerializedFile v22 writer and validates the Unity version
func NewSerializedFileWriter(unityVersion string) *SerializedFileWriter {
	if unityVersion == "" {
		unityVersion = "2022.3.35f1"
	}
	w := &SerializedFileWriter{
		UnityVersion:   unityVersion,
		TargetPlatform: defaultPlatform,
		nextPathId:     1,
		usedPathIds:    map[int64]struct{}{},
	}
	if err := validateSerializedFileUnityVersion(unityVersion); err != nil {
		w.setError(err)
	}
	return w
}

// SetExternalFiles 设置 metadata 外部文件表并复制输入，使后续调用方修改不会改变写入结果
// SetExternalFiles sets the metadata external-file table and copies the input so later caller mutations cannot alter the output
func (w *SerializedFileWriter) SetExternalFiles(externalFiles []ExternalFile) {
	if w == nil {
		return
	}
	if _, err := int32WireLength("external file count", uint64(len(externalFiles))); err != nil {
		w.setError(err)
		return
	}
	for externalIndex := range externalFiles {
		external := externalFiles[externalIndex]
		if strings.IndexByte(external.AssetPath, 0) >= 0 {
			w.setError(fmt.Errorf("external file[%d] asset path contains NUL", externalIndex))
			return
		}
		if strings.IndexByte(external.PathName, 0) >= 0 {
			w.setError(fmt.Errorf("external file[%d] path name contains NUL", externalIndex))
			return
		}
	}
	w.externalFiles = append(w.externalFiles[:0], externalFiles...)
}

// AddTextAsset 添加一个 TextAsset 对象，name 是资源名称，script 是 m_Script 数据
// 返回分配的 PathID
// AddTextAsset adds a TextAsset whose name is the resource name and whose script is m_Script data
// It returns the allocated PathID
func (w *SerializedFileWriter) AddTextAsset(name string, script []byte) int64 {
	return w.AddTextAssetWithLoadName(name, name, script)
}

// AddTextAssetWithPathID 使用首选 PathID 添加 TextAsset
// 重打包已解包 .aba 时可用它保留内部 Unity PPtr 引用指向相同对象
// AddTextAssetWithPathID adds a TextAsset using a preferred PathID
// It can preserve internal Unity PPtr targets when repacking extracted .aba files
func (w *SerializedFileWriter) AddTextAssetWithPathID(name string, script []byte, pathID int64) int64 {
	return w.AddTextAssetWithLoadNameAndPathID(name, name, script, pathID)
}

// AddTextAssetWithLoadName 添加内部 m_Name 可不同于 AssetBundle m_Container LoadAsset 键的 TextAsset
// AddTextAssetWithLoadName adds a TextAsset whose internal m_Name can differ from the AssetBundle m_Container key used by LoadAsset
func (w *SerializedFileWriter) AddTextAssetWithLoadName(name string, loadName string, script []byte) int64 {
	return w.AddTextAssetWithLoadNameAndPathID(name, loadName, script, 0)
}

// AddTextAssetWithLoadNameAndPathID 添加带独立 m_Name、AssetBundle 加载键和首选 PathID 的 TextAsset
// AddTextAssetWithLoadNameAndPathID adds a TextAsset with separate internal m_Name, AssetBundle load key, and preferred PathID
func (w *SerializedFileWriter) AddTextAssetWithLoadNameAndPathID(name string, loadName string, script []byte, pathID int64) int64 {
	data, err := encodeTextAssetData(name, script)
	if err != nil {
		w.setError(fmt.Errorf("encode TextAsset %q: %w", name, err))
		return 0
	}
	dataSize, err := uint32WireLength(fmt.Sprintf("TextAsset %q byte size", name), uint64(len(data)))
	if err != nil {
		w.setError(err)
		return 0
	}
	actualPathID := w.reserveOrAllocatePathID(pathID)
	w.objects = append(w.objects, sfObject{
		pathId:   actualPathID,
		classId:  ClassIDTextAsset,
		name:     name,
		loadName: nonEmptyLoadName(loadName, name),
		data:     data,
		dataSize: dataSize,
	})
	return actualPathID
}

// AddTexture2D 添加一个 Texture2D 对象，imageData 是 RGBA32 像素数据，width 和 height 是尺寸
// 返回分配的 PathID
// AddTexture2D adds a Texture2D with RGBA32 imageData and the supplied width and height
// It returns the allocated PathID
func (w *SerializedFileWriter) AddTexture2D(name string, width, height int64, imageData []byte) int64 {
	return w.AddTexture2DWithLoadName(name, name, width, height, imageData)
}

// AddTexture2DWithPathID 使用首选 PathID 添加生成的 Texture2D
// AddTexture2DWithPathID adds a generated Texture2D with a preferred PathID
func (w *SerializedFileWriter) AddTexture2DWithPathID(name string, width, height int64, imageData []byte, pathID int64) int64 {
	return w.AddTexture2DWithLoadNameAndPathID(name, name, width, height, imageData, pathID)
}

// AddTexture2DWithLoadName 添加内部 m_Name 可不同于 AssetBundle m_Container LoadAsset 键的 Texture2D
// AddTexture2DWithLoadName adds a generated Texture2D whose internal m_Name can differ from the AssetBundle m_Container key used by LoadAsset
func (w *SerializedFileWriter) AddTexture2DWithLoadName(name string, loadName string, width, height int64, imageData []byte) int64 {
	return w.AddTexture2DWithLoadNameAndPathID(name, loadName, width, height, imageData, 0)
}

// AddTexture2DWithLoadNameAndPathID 添加带独立 m_Name、AssetBundle 加载键和首选 PathID 的 Texture2D
// AddTexture2DWithLoadNameAndPathID adds a generated Texture2D with separate internal m_Name, AssetBundle load key, and preferred PathID
func (w *SerializedFileWriter) AddTexture2DWithLoadNameAndPathID(name string, loadName string, width, height int64, imageData []byte, pathID int64) int64 {
	data, err := encodeTexture2DData(w.UnityVersion, name, width, height, imageData)
	if err != nil {
		w.setError(fmt.Errorf("encode Texture2D %q: %w", name, err))
		return 0
	}
	dataSize, err := uint32WireLength(fmt.Sprintf("Texture2D %q byte size", name), uint64(len(data)))
	if err != nil {
		w.setError(err)
		return 0
	}
	actualPathID := w.reserveOrAllocatePathID(pathID)
	typeTree := unity2022Texture2DTypeTree()
	w.objects = append(w.objects, sfObject{
		pathId:   actualPathID,
		classId:  ClassIDTexture2D,
		name:     name,
		loadName: nonEmptyLoadName(loadName, name),
		data:     data,
		dataSize: dataSize,
		typeTree: &typeTree,
	})
	return actualPathID
}

// AddRawObject 添加一个原始数据对象，如 Mesh，并返回分配的 PathID
// AddRawObject adds a raw data object such as Mesh and returns the allocated PathID
func (w *SerializedFileWriter) AddRawObject(classId int32, name string, data []byte) int64 {
	return w.AddRawObjectWithLoadNameAndPathID(classId, name, name, data, 0)
}

// AddRawObjectWithPathID 使用首选 PathID 添加原始 Unity 序列化对象
// 如果请求的 PathID 为零或已占用，会重新分配以保持 SerializedFile 有效
// AddRawObjectWithPathID adds a raw serialized Unity object with a preferred PathID
// If the requested PathID is zero or already used, a fresh PathID is allocated to keep the SerializedFile valid
func (w *SerializedFileWriter) AddRawObjectWithPathID(classId int32, name string, data []byte, pathID int64) int64 {
	return w.AddRawObjectWithLoadNameAndPathID(classId, name, name, data, pathID)
}

// AddRawObjectWithLoadName 添加内部 m_Name 可不同于 AssetBundle m_Container LoadAsset 键的原始 Unity 对象
// AddRawObjectWithLoadName adds a raw serialized Unity object whose internal m_Name can differ from the AssetBundle m_Container key used by LoadAsset
func (w *SerializedFileWriter) AddRawObjectWithLoadName(classId int32, name string, loadName string, data []byte) int64 {
	return w.AddRawObjectWithLoadNameAndPathID(classId, name, loadName, data, 0)
}

// AddRawObjectWithLoadNameAndPathID 添加带独立 m_Name、AssetBundle 加载键和首选 PathID 的原始 Unity 对象
// AddRawObjectWithLoadNameAndPathID adds a raw serialized Unity object with separate internal m_Name, AssetBundle load key, and preferred PathID
func (w *SerializedFileWriter) AddRawObjectWithLoadNameAndPathID(classId int32, name string, loadName string, data []byte, pathID int64) int64 {
	if rawObjectHasLeadingName(classId) {
		if rewritten, err := rewriteLeadingAlignedName(data, name); err == nil {
			data = rewritten
		}
	}
	return w.addRawObject(classId, name, loadName, data, pathID)
}

// AddRawObjectPreservingDataWithLoadNameAndPathID 添加原始 Unity 对象并严格保留对象数据字节，仅使用 name 和 loadName 构建索引
// AddRawObjectPreservingDataWithLoadNameAndPathID adds a raw Unity object while preserving its data bytes exactly and uses name and loadName only for indexing
func (w *SerializedFileWriter) AddRawObjectPreservingDataWithLoadNameAndPathID(classId int32, name string, loadName string, data []byte, pathID int64) int64 {
	return w.addRawObject(classId, name, loadName, data, pathID)
}

// AddRawObjectSourcePreservingDataWithLoadNameAndPathID 添加流式原始 Unity 对象并严格保留其数据字节
// AddRawObjectSourcePreservingDataWithLoadNameAndPathID adds a streamed raw Unity object while preserving its data bytes exactly
func (w *SerializedFileWriter) AddRawObjectSourcePreservingDataWithLoadNameAndPathID(classId int32, name string, loadName string, source SerializedObjectDataSource, pathID int64) int64 {
	if source.WriteTo == nil {
		w.setError(fmt.Errorf("raw object %q has a nil data-source writer", name))
		return 0
	}
	if uint64(len(source.Prefix)) > uint64(source.Size) {
		w.setError(fmt.Errorf("raw object %q prefix size %d exceeds object size %d", name, len(source.Prefix), source.Size))
		return 0
	}
	actualPathID := w.reserveOrAllocatePathID(pathID)
	var scriptPathID int64
	if classId == ClassIDMonoBehaviour {
		fileID, candidatePathID, ok := readMonoBehaviourScriptPPtr(source.Prefix)
		if ok && fileID == 0 {
			scriptPathID = candidatePathID
		}
	}
	w.objects = append(w.objects, sfObject{
		pathId:       actualPathID,
		classId:      classId,
		scriptPathId: scriptPathID,
		name:         name,
		loadName:     nonEmptyLoadName(loadName, name),
		dataSize:     source.Size,
		writeData:    source.WriteTo,
	})
	return actualPathID
}

// AddNativeUnityObjectWithLoadNameAndPathID 添加携带完整 TypeTree 的独立 Unity 对象，并仅把其正文写入 SerializedFile
// AddNativeUnityObjectWithLoadNameAndPathID adds a standalone Unity object with a complete TypeTree and writes only its payload into the SerializedFile
func (w *SerializedFileWriter) AddNativeUnityObjectWithLoadNameAndPathID(object *NativeUnityObject, name string, loadName string, pathID int64) int64 {
	if object == nil {
		w.setError(fmt.Errorf("native Unity object %q is nil", name))
		return 0
	}
	if err := validateNativeUnityObjectSchema(object.ClassID, &object.TypeTree); err != nil {
		w.setError(fmt.Errorf("native Unity object %q: %w", name, err))
		return 0
	}
	if object.BigEndian {
		w.setError(fmt.Errorf("native Unity object %q uses big-endian data but the generated SerializedFile is little-endian", name))
		return 0
	}
	dataSize, err := uint32WireLength(fmt.Sprintf("native Unity object %q byte size", name), uint64(len(object.Data)))
	if err != nil {
		w.setError(err)
		return 0
	}
	return w.addNativeUnityObjectSource(object.ClassID, name, loadName, SerializedObjectDataSource{
		Size:   dataSize,
		Prefix: object.Data,
		WriteTo: func(out io.Writer) error {
			return writeAbaBytes(out, object.Data)
		},
	}, &object.TypeTree, pathID)
}

// AddNativeUnityObjectSourceWithLoadNameAndPathID 添加可流式读取正文且携带完整 TypeTree 的独立 Unity 对象
// AddNativeUnityObjectSourceWithLoadNameAndPathID adds a standalone Unity object with streamed payload data and a complete TypeTree
func (w *SerializedFileWriter) AddNativeUnityObjectSourceWithLoadNameAndPathID(classID int32, tree TypeTreeType, name string, loadName string, source SerializedObjectDataSource, pathID int64) int64 {
	return w.addNativeUnityObjectSource(classID, name, loadName, source, &tree, pathID)
}

// addNativeUnityObjectSource 校验独立对象布局并将正文数据源加入写入队列
// addNativeUnityObjectSource validates a standalone object layout and appends its payload source to the write queue
func (w *SerializedFileWriter) addNativeUnityObjectSource(classID int32, name string, loadName string, source SerializedObjectDataSource, tree *TypeTreeType, pathID int64) int64 {
	if source.WriteTo == nil {
		w.setError(fmt.Errorf("native Unity object %q has a nil data-source writer", name))
		return 0
	}
	if uint64(len(source.Prefix)) > uint64(source.Size) {
		w.setError(fmt.Errorf("native Unity object %q prefix size %d exceeds object size %d", name, len(source.Prefix), source.Size))
		return 0
	}
	if err := validateNativeUnityObjectSchema(classID, tree); err != nil {
		w.setError(fmt.Errorf("native Unity object %q: %w", name, err))
		return 0
	}
	clonedTree := cloneTypeTreeType(tree)
	actualPathID := w.reserveOrAllocatePathID(pathID)
	var scriptPathID int64
	if classID == ClassIDMonoBehaviour {
		fileID, candidatePathID, ok := readMonoBehaviourScriptPPtr(source.Prefix)
		if ok && fileID == 0 {
			scriptPathID = candidatePathID
		}
	}
	w.objects = append(w.objects, sfObject{
		pathId:       actualPathID,
		classId:      classID,
		scriptPathId: scriptPathID,
		name:         name,
		loadName:     nonEmptyLoadName(loadName, name),
		dataSize:     source.Size,
		writeData:    source.WriteTo,
		typeTree:     &clonedTree,
	})
	return actualPathID
}

// addRawObject 将准备好的对象字节和索引信息加入写入队列
// addRawObject appends prepared object bytes and index information to the write queue
func (w *SerializedFileWriter) addRawObject(classId int32, name string, loadName string, data []byte, pathID int64) int64 {
	dataSize, err := uint32WireLength(fmt.Sprintf("raw object %q byte size", name), uint64(len(data)))
	if err != nil {
		w.setError(err)
		return 0
	}
	actualPathID := w.reserveOrAllocatePathID(pathID)
	var scriptPathID int64
	if classId == ClassIDMonoBehaviour {
		fileID, candidatePathID, ok := readMonoBehaviourScriptPPtr(data)
		if ok && fileID == 0 {
			scriptPathID = candidatePathID
		}
	}
	w.objects = append(w.objects, sfObject{
		pathId:       actualPathID,
		classId:      classId,
		scriptPathId: scriptPathID,
		name:         name,
		loadName:     nonEmptyLoadName(loadName, name),
		data:         data,
		dataSize:     dataSize,
	})
	return actualPathID
}

// readMonoBehaviourScriptPPtr 从 Unity 2019 及以后发布版 MonoBehaviour 的稳定基类前缀读取 m_Script 引用
// readMonoBehaviourScriptPPtr reads the m_Script reference from the stable release-player MonoBehaviour base prefix used by Unity 2019 and later
func readMonoBehaviourScriptPPtr(data []byte) (int32, int64, bool) {
	const scriptFileIDOffset int64 = 16
	const scriptPathIDOffset int64 = 20
	const prefixSize int64 = 28
	if int64(len(data)) < prefixSize {
		return 0, 0, false
	}
	fileID := int32(binary.LittleEndian.Uint32(data[scriptFileIDOffset:scriptPathIDOffset]))
	pathID := int64(binary.LittleEndian.Uint64(data[scriptPathIDOffset:prefixSize]))
	return fileID, pathID, true
}

// nonEmptyLoadName 返回非空的 AssetBundle 加载键，空值时回退到对象名称
// nonEmptyLoadName returns a non-empty AssetBundle load key, falling back to the object name
func nonEmptyLoadName(loadName string, name string) string {
	if loadName != "" {
		return loadName
	}
	return name
}

// allocatePathID 分配下一个未使用的正数 PathID
// allocatePathID allocates the next unused positive PathID
func (w *SerializedFileWriter) allocatePathID() int64 {
	w.ensurePathIDState()
	for w.nextPathId == 0 || w.isPathIDUsed(w.nextPathId) {
		w.nextPathId++
	}
	pathID := w.nextPathId
	w.usedPathIds[pathID] = struct{}{}
	w.nextPathId++
	return pathID
}

// setError 保存第一个构建错误，使公开添加方法可以延迟到 Write 返回错误
// setError stores the first build error so public add methods can defer failure until Write
func (w *SerializedFileWriter) setError(err error) {
	if w.err == nil {
		w.err = err
	}
}

// reserveOrAllocatePathID 保留可用的首选 PathID，否则分配新的 PathID
// reserveOrAllocatePathID reserves an available preferred PathID or allocates a new one
func (w *SerializedFileWriter) reserveOrAllocatePathID(pathID int64) int64 {
	w.ensurePathIDState()
	if pathID == 0 || w.isPathIDUsed(pathID) {
		return w.allocatePathID()
	}
	w.usedPathIds[pathID] = struct{}{}
	if pathID > 0 && pathID >= w.nextPathId && pathID < math.MaxInt64 {
		w.nextPathId = pathID + 1
	}
	return pathID
}

// nextAvailablePathID 查找不与已占用集合冲突的下一个 PathID
// nextAvailablePathID finds the next PathID that does not conflict with the used set
func (w *SerializedFileWriter) nextAvailablePathID() int64 {
	w.ensurePathIDState()
	pathID := w.nextPathId
	for pathID == 0 || w.isPathIDUsed(pathID) {
		if pathID == math.MaxInt64 {
			pathID = math.MinInt64
		} else {
			pathID++
		}
	}
	return pathID
}

// ensurePathIDState 初始化 PathID 游标和已占用集合
// ensurePathIDState initializes the PathID cursor and used set
func (w *SerializedFileWriter) ensurePathIDState() {
	if w.nextPathId == 0 {
		w.nextPathId = 1
	}
	if w.usedPathIds == nil {
		w.usedPathIds = map[int64]struct{}{}
		for _, obj := range w.objects {
			w.usedPathIds[obj.pathId] = struct{}{}
		}
	}
}

// isPathIDUsed 判断 PathID 是否已被对象占用
// isPathIDUsed reports whether a PathID is already used by an object
func (w *SerializedFileWriter) isPathIDUsed(pathID int64) bool {
	_, ok := w.usedPathIds[pathID]
	return ok
}

// rawObjectHasLeadingName 判断当前支持的原始对象布局是否以对齐 m_Name 开始
// rawObjectHasLeadingName reports whether a supported raw-object layout begins with an aligned m_Name
func rawObjectHasLeadingName(classId int32) bool {
	switch classId {
	case ClassIDMaterial,
		ClassIDTexture2D,
		ClassIDMesh,
		ClassIDShader,
		ClassIDTextAsset,
		ClassIDAnimationClip,
		ClassIDAudioClip,
		ClassIDCubemap,
		ClassIDMonoScript,
		ClassIDFont,
		ClassIDSprite,
		ClassIDSpriteAtlas,
		ClassIDAssetBundle:
		return true
	default:
		return false
	}
}

// rewriteLeadingAlignedName 替换原始对象开头的对齐 m_Name，同时保留其余字节
// rewriteLeadingAlignedName replaces the leading aligned m_Name while preserving all remaining bytes
func rewriteLeadingAlignedName(data []byte, name string) ([]byte, error) {
	r := binaryio.NewEndianReader(data, binary.LittleEndian)
	if _, err := r.ReadAlignedString(); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	bw := binaryio.NewEndianWriter(&buf, binary.LittleEndian)
	if err := bw.WriteAlignedString(name); err != nil {
		return nil, err
	}
	buf.Write(data[r.Pos():])
	return buf.Bytes(), nil
}

// Size 计算完整 SerializedFile 的精确字节数且不读取流式对象数据
// Size computes the exact complete SerializedFile byte size without reading streamed object data
func (w *SerializedFileWriter) Size() (int64, error) {
	if w == nil {
		return 0, fmt.Errorf("nil SerializedFileWriter")
	}
	if w.err != nil {
		return 0, w.err
	}
	if err := validateSerializedFileUnityVersion(w.UnityVersion); err != nil {
		return 0, err
	}
	if _, err := int32WireLength("serialized object count", uint64(len(w.objects))+1); err != nil {
		return 0, err
	}
	containerData, err := w.encodeAssetBundleObject()
	if err != nil {
		return 0, fmt.Errorf("encode AssetBundle object: %w", err)
	}
	containerDataSize, err := uint32WireLength("AssetBundle object byte size", uint64(len(containerData)))
	if err != nil {
		return 0, err
	}
	allObjects := make([]sfObject, 0, len(w.objects)+1)
	allObjects = append(allObjects, w.objects...)
	allObjects = append(allObjects, sfObject{
		pathId:   w.nextAvailablePathID(),
		classId:  ClassIDAssetBundle,
		name:     "CAB-generated",
		data:     containerData,
		dataSize: containerDataSize,
	})
	serializedTypes, scriptTypes, err := collectSerializedTypes(allObjects)
	if err != nil {
		return 0, err
	}
	metadata, err := w.buildMetadata(allObjects, serializedTypes, scriptTypes)
	if err != nil {
		return 0, fmt.Errorf("build metadata: %w", err)
	}
	offsets, dataSectionSize, err := layoutDataSection(allObjects)
	if err != nil {
		return 0, fmt.Errorf("layout data section: %w", err)
	}
	metadata, err = w.buildMetadataWithOffsets(allObjects, serializedTypes, scriptTypes, offsets)
	if err != nil {
		return 0, fmt.Errorf("build metadata with offsets: %w", err)
	}
	const headerSize int64 = 48
	dataOffset := binaryio.AlignOffset(headerSize+int64(len(metadata)), 16)
	fileSize, ok := addNonNegativeInt64(dataOffset, dataSectionSize)
	if !ok {
		return 0, fmt.Errorf("serialized file size overflows Int64")
	}
	return fileSize, nil
}

// Write 将所有对象写入完整的 SerializedFile v22 格式，并自动追加 ClassID 142 AssetBundle 对象作为 m_Container 映射
// Write emits a complete SerializedFile v22 and appends a ClassID 142 AssetBundle object containing the m_Container mapping
func (w *SerializedFileWriter) Write(out io.Writer) error {
	if w == nil {
		return fmt.Errorf("nil SerializedFileWriter")
	}
	if out == nil {
		return fmt.Errorf("nil SerializedFile output writer")
	}
	if w.err != nil {
		return w.err
	}
	if err := validateSerializedFileUnityVersion(w.UnityVersion); err != nil {
		return err
	}
	if _, err := int32WireLength("serialized object count", uint64(len(w.objects))+1); err != nil {
		return err
	}
	for objectIndex := range w.objects {
		object := w.objects[objectIndex]
		if object.writeData == nil && uint64(len(object.data)) != uint64(object.dataSize) {
			return fmt.Errorf("object[%d] %q has %d in-memory bytes but declares %d", objectIndex, object.name, len(object.data), object.dataSize)
		}
	}

	// 追加一个保存 m_Container 加载映射的 AssetBundle 对象
	// Append an AssetBundle object containing the m_Container load mapping
	containerData, err := w.encodeAssetBundleObject()
	if err != nil {
		return fmt.Errorf("encode AssetBundle object: %w", err)
	}
	containerDataSize, err := uint32WireLength("AssetBundle object byte size", uint64(len(containerData)))
	if err != nil {
		return err
	}
	abPathId := w.nextAvailablePathID()
	allObjects := make([]sfObject, 0, len(w.objects)+1)
	allObjects = append(allObjects, w.objects...)
	allObjects = append(allObjects, sfObject{
		pathId:   abPathId,
		classId:  ClassIDAssetBundle,
		name:     "CAB-generated",
		data:     containerData,
		dataSize: containerDataSize,
	})

	// 按首次出现顺序收集 SerializedType，并为不同 MonoScript 目标创建独立的 MonoBehaviour 类型
	// Collect SerializedTypes in first-seen order and create a distinct MonoBehaviour type for each MonoScript target
	serializedTypes, scriptTypes, err := collectSerializedTypes(allObjects)
	if err != nil {
		return err
	}

	// 首次构建 metadata 以确定其长度
	// Build metadata once to determine its length
	metadataBuf, err := w.buildMetadata(allObjects, serializedTypes, scriptTypes)
	if err != nil {
		return fmt.Errorf("build metadata: %w", err)
	}

	// v22 Header 固定为 48 字节
	// The v22 Header is fixed at 48 bytes
	headerSize := int64(48)

	// 数据区起点对齐到 16 字节
	// Align the data-section start to 16 bytes
	dataOffset := binaryio.AlignOffset(headerSize+int64(len(metadataBuf)), 16)

	// 计算数据区布局而不复制对象内容，并将每个对象起点对齐到八字节
	// Compute the data-section layout without copying object contents and align every object start to eight bytes
	objectOffsets, dataSectionSize, err := layoutDataSection(allObjects)
	if err != nil {
		return fmt.Errorf("layout data section: %w", err)
	}

	// 使用最终对象偏移重新构建 metadata
	// Rebuild metadata with final object offsets
	metadataBuf, err = w.buildMetadataWithOffsets(allObjects, serializedTypes, scriptTypes, objectOffsets)
	if err != nil {
		return fmt.Errorf("build metadata with offsets: %w", err)
	}
	dataOffset = binaryio.AlignOffset(headerSize+int64(len(metadataBuf)), 16)

	fileSize, ok := addNonNegativeInt64(dataOffset, dataSectionSize)
	if !ok {
		return fmt.Errorf("serialized file size overflows Int64")
	}
	metadataSize, err := uint32WireLength("serialized metadata size", uint64(len(metadataBuf)))
	if err != nil {
		return err
	}

	// 以 Big-Endian 构建 SerializedFile Header
	// Build the SerializedFile Header in Big-Endian order
	var header bytes.Buffer
	hw := binaryio.NewEndianWriter(&header, binary.BigEndian)
	// v22 的四个旧头部数值依次为 MetadataSize、FileSize、Version 和 DataOffset，其中除 Version 外均写零
	// The four legacy v22 header numbers are MetadataSize, FileSize, Version, and DataOffset in order, with all but Version written as zero
	if err := hw.WriteUInt32(0); err != nil { // MetadataSize (legacy; zero for v22)
		return fmt.Errorf("write header metadata size: %w", err)
	}
	if err := hw.WriteUInt32(0); err != nil { // FileSize (legacy; zero for v22)
		return fmt.Errorf("write header file size: %w", err)
	}
	if err := hw.WriteUInt32(sfVersion); err != nil { // Version
		return fmt.Errorf("write header version: %w", err)
	}
	if err := hw.WriteUInt32(0); err != nil { // DataOffset (legacy; zero for v22)
		return fmt.Errorf("write header data offset: %w", err)
	}
	// 对象和 metadata 使用 Little-Endian，随后写入三个填充字节
	// Object data and metadata use Little-Endian, followed by three padding bytes
	if err := hw.WriteByte(0); err != nil { // Endianness = Little
		return fmt.Errorf("write header endianness: %w", err)
	}
	if err := hw.WriteZeroes(3); err != nil { // padding
		return fmt.Errorf("write header padding: %w", err)
	}
	// v22 扩展头依次保存 MetadataSize、Int64 FileSize、Int64 DataOffset 和用途未知的 Int64 字段，新建文件写零
	// The v22 extended header stores MetadataSize, Int64 FileSize, Int64 DataOffset, and an Int64 field of unknown purpose, written as zero for new files
	if err := hw.WriteUInt32(metadataSize); err != nil { // MetadataSize
		return fmt.Errorf("write extended header metadata size: %w", err)
	}
	if err := hw.WriteInt64(fileSize); err != nil { // FileSize (int64)
		return fmt.Errorf("write extended header file size: %w", err)
	}
	if err := hw.WriteInt64(dataOffset); err != nil { // DataOffset (int64)
		return fmt.Errorf("write extended header data offset: %w", err)
	}
	if err := hw.WriteInt64(0); err != nil { // unknown field
		return fmt.Errorf("write extended header unknown field: %w", err)
	}

	if err := writeAbaBytes(out, header.Bytes()); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	if err := writeAbaBytes(out, metadataBuf); err != nil {
		return fmt.Errorf("write metadata: %w", err)
	}
	// 在 metadata 后写零填充直到 DataOffset
	// Write zero padding after metadata up to DataOffset
	padding := dataOffset - headerSize - int64(len(metadataBuf))
	if padding > 0 {
		var paddingBytes [15]byte
		if err := writeAbaBytes(out, paddingBytes[:padding]); err != nil {
			return fmt.Errorf("write padding: %w", err)
		}
	}
	if err := writeDataSection(out, allObjects, objectOffsets, dataSectionSize); err != nil {
		return fmt.Errorf("write data: %w", err)
	}
	return nil
}

// buildMetadata 构建对象偏移暂为零的 metadata，用于计算布局
// buildMetadata builds metadata with provisional zero object offsets for layout calculation
func (w *SerializedFileWriter) buildMetadata(objects []sfObject, serializedTypes []sfSerializedType, scriptTypes []LocalSerializedObjectIdentifier) ([]byte, error) {
	return w.buildMetadataWithOffsets(objects, serializedTypes, scriptTypes, nil)
}

// buildMetadataWithOffsets 构建包含 SerializedTypes、AssetInfos、ScriptTypes 和尾部表的 Little-Endian v22 metadata
// buildMetadataWithOffsets builds Little-Endian v22 metadata containing SerializedTypes, AssetInfos, ScriptTypes, and the remaining tail tables
func (w *SerializedFileWriter) buildMetadataWithOffsets(objects []sfObject, serializedTypes []sfSerializedType, scriptTypes []LocalSerializedObjectIdentifier, offsets []int64) ([]byte, error) {
	if offsets != nil && len(offsets) != len(objects) {
		return nil, fmt.Errorf("object offset count %d does not match object count %d", len(offsets), len(objects))
	}
	typeCount, err := int32WireLength("serialized type count", uint64(len(serializedTypes)))
	if err != nil {
		return nil, err
	}
	objectCount, err := int32WireLength("serialized object count", uint64(len(objects)))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	bw := binaryio.NewEndianWriter(&buf, binary.LittleEndian)

	// metadata 首先保存 NUL 结尾的 UnityVersion
	// Metadata begins with NUL-terminated UnityVersion
	if err := bw.WriteNullString(w.UnityVersion); err != nil { // UnityVersion (null-terminated)
		return nil, fmt.Errorf("write unity version: %w", err)
	}

	// 随后保存 TargetPlatform
	// TargetPlatform follows
	if err := bw.WriteUInt32(w.TargetPlatform); err != nil { // TargetPlatform
		return nil, fmt.Errorf("write target platform: %w", err)
	}

	// 只要任一独立对象携带 TypeTree，就为整个 SerializedFile 启用标准 TypeTree metadata
	// Enable standard TypeTree metadata for the SerializedFile when any standalone object carries a TypeTree
	typeTreeEnabled := serializedTypesContainTypeTree(serializedTypes)
	if err := bw.WriteBool(typeTreeEnabled); err != nil {
		return nil, fmt.Errorf("write type tree enabled: %w", err)
	}

	// 写入 SerializedTypes 数量，各 v22 类型记录依次包含 TypeId、IsStrippedType、ScriptTypeIndex、条件 ScriptIdHash 和 TypeHash
	// Write SerializedTypes count; each v22 type record contains TypeId, IsStrippedType, ScriptTypeIndex, conditional ScriptIdHash, and TypeHash in order
	if err := bw.WriteInt32(typeCount); err != nil {
		return nil, fmt.Errorf("write type count: %w", err)
	}
	for _, serializedType := range serializedTypes {
		if err := bw.WriteInt32(serializedType.classId); err != nil { // TypeId
			return nil, fmt.Errorf("write type id %d: %w", serializedType.classId, err)
		}
		if err := bw.WriteBool(serializedType.isStrippedType); err != nil { // IsStrippedType
			return nil, fmt.Errorf("write stripped flag for type %d: %w", serializedType.classId, err)
		}
		if err := bw.WriteInt16(serializedType.scriptTypeIndex); err != nil { // ScriptTypeIndex
			return nil, fmt.Errorf("write script type index for type %d: %w", serializedType.classId, err)
		}
		if serializedType.classId == ClassIDMonoBehaviour {
			if err := bw.WriteBytes(serializedType.scriptIdHash[:]); err != nil { // ScriptIdHash
				return nil, fmt.Errorf("write script id hash for type %d: %w", serializedType.classId, err)
			}
		}
		if err := bw.WriteBytes(serializedType.typeHash[:]); err != nil { // TypeHash
			return nil, fmt.Errorf("write type hash for type %d: %w", serializedType.classId, err)
		}
		if typeTreeEnabled {
			var tree TypeTreeType
			if serializedType.typeTree != nil {
				tree = *serializedType.typeTree
			}
			nodeCount, err := int32WireLength(fmt.Sprintf("class %d TypeTree node count", serializedType.classId), uint64(len(tree.Nodes)))
			if err != nil {
				return nil, err
			}
			stringSize, err := int32WireLength(fmt.Sprintf("class %d TypeTree string buffer size", serializedType.classId), uint64(len(tree.StringBuffer)))
			if err != nil {
				return nil, err
			}
			if err := bw.WriteInt32(nodeCount); err != nil {
				return nil, fmt.Errorf("write TypeTree node count for type %d: %w", serializedType.classId, err)
			}
			if err := bw.WriteInt32(stringSize); err != nil {
				return nil, fmt.Errorf("write TypeTree string buffer size for type %d: %w", serializedType.classId, err)
			}
			for nodeIndex := range tree.Nodes {
				if err := writeNativeUnityObjectNode(bw, &tree.Nodes[nodeIndex]); err != nil {
					return nil, fmt.Errorf("write TypeTree node[%d] for type %d: %w", nodeIndex, serializedType.classId, err)
				}
			}
			if err := bw.WriteBytes(tree.StringBuffer); err != nil {
				return nil, fmt.Errorf("write TypeTree string buffer for type %d: %w", serializedType.classId, err)
			}
			dependencyCount, err := int32WireLength(fmt.Sprintf("class %d TypeTree dependency count", serializedType.classId), uint64(len(tree.TypeDependencies)))
			if err != nil {
				return nil, err
			}
			if err := bw.WriteInt32(dependencyCount); err != nil {
				return nil, fmt.Errorf("write TypeTree dependency count for type %d: %w", serializedType.classId, err)
			}
			for dependencyIndex, dependency := range tree.TypeDependencies {
				if err := bw.WriteInt32(dependency); err != nil {
					return nil, fmt.Errorf("write TypeTree dependency[%d] for type %d: %w", dependencyIndex, serializedType.classId, err)
				}
			}
		}
	}

	// SerializedTypes 后写入 AssetInfos 数量和对象表
	// Write AssetInfos count and object table after SerializedTypes
	if err := bw.WriteInt32(objectCount); err != nil {
		return nil, fmt.Errorf("write asset info count: %w", err)
	}
	for i, obj := range objects {
		// v22 每个对象表条目前对齐到四字节
		// Align to four bytes before each v22 object-table entry
		if err := bw.Align(4); err != nil {
			return nil, fmt.Errorf("align asset info[%d]: %w", i, err)
		}
		if err := bw.WriteInt64(obj.pathId); err != nil { // PathId
			return nil, fmt.Errorf("write asset info[%d] path id: %w", i, err)
		}
		var offset int64
		if offsets != nil {
			offset = offsets[i]
		}
		if err := bw.WriteInt64(offset); err != nil { // ByteOffset
			return nil, fmt.Errorf("write asset info[%d] byte offset: %w", i, err)
		}
		if err := bw.WriteUInt32(obj.dataSize); err != nil { // ByteSize
			return nil, fmt.Errorf("write asset info[%d] byte size: %w", i, err)
		}
		typeIndex := serializedTypeIndex(serializedTypes, obj)
		if typeIndex < 0 {
			return nil, fmt.Errorf("asset info[%d] class ID %d is absent from serialized types", i, obj.classId)
		}
		if err := bw.WriteInt32(typeIndex); err != nil { // TypeIndex
			return nil, fmt.Errorf("write asset info[%d] type index: %w", i, err)
		}
	}

	// ScriptTypes 将每个 MonoBehaviour SerializedType 连接到同一文件中的 MonoScript 对象
	// ScriptTypes connect each MonoBehaviour SerializedType to its MonoScript object in the same file
	scriptTypeCount, err := int32WireLength("script type count", uint64(len(scriptTypes)))
	if err != nil {
		return nil, err
	}
	if err := bw.WriteInt32(scriptTypeCount); err != nil {
		return nil, fmt.Errorf("write script type count: %w", err)
	}
	for scriptIndex, identifier := range scriptTypes {
		if err := bw.WriteInt32(identifier.LocalSerializedFileIndex); err != nil {
			return nil, fmt.Errorf("write script type[%d] file index: %w", scriptIndex, err)
		}
		if err := bw.Align(4); err != nil {
			return nil, fmt.Errorf("align script type[%d]: %w", scriptIndex, err)
		}
		if err := bw.WriteInt64(identifier.LocalIdentifierInFile); err != nil {
			return nil, fmt.Errorf("write script type[%d] PathID: %w", scriptIndex, err)
		}
	}

	// ExternalFiles 按一基 fileID 对应的数组顺序写入完整引用记录
	// ExternalFiles are emitted as complete reference records in the array order addressed by one-based fileIDs
	externalFileCount, err := int32WireLength("external file count", uint64(len(w.externalFiles)))
	if err != nil {
		return nil, err
	}
	if err := bw.WriteInt32(externalFileCount); err != nil {
		return nil, fmt.Errorf("write external file count: %w", err)
	}
	for externalIndex := range w.externalFiles {
		external := w.externalFiles[externalIndex]
		if err := bw.WriteNullString(external.AssetPath); err != nil {
			return nil, fmt.Errorf("write external file[%d] asset path: %w", externalIndex, err)
		}
		if err := bw.WriteBytes(external.Guid[:]); err != nil {
			return nil, fmt.Errorf("write external file[%d] GUID: %w", externalIndex, err)
		}
		if err := bw.WriteInt32(external.Type); err != nil {
			return nil, fmt.Errorf("write external file[%d] type: %w", externalIndex, err)
		}
		if err := bw.WriteNullString(external.PathName); err != nil {
			return nil, fmt.Errorf("write external file[%d] path name: %w", externalIndex, err)
		}
	}

	// RefTypes 数量写零
	// RefTypes count is zero
	if err := bw.WriteUInt32(0); err != nil {
		return nil, fmt.Errorf("write ref type count: %w", err)
	}

	// UserInformation 写为空 NUL 结尾字符串
	// UserInformation is an empty NUL-terminated string
	if err := bw.WriteByte(0); err != nil {
		return nil, fmt.Errorf("write user information: %w", err)
	}
	return buf.Bytes(), nil
}

// encodeAssetBundleObject 编码包含所有用户对象加载名和 PPtr 的 Unity AssetBundle 容器对象
// encodeAssetBundleObject encodes a Unity AssetBundle container object with load names and PPtrs for all user objects
func (w *SerializedFileWriter) encodeAssetBundleObject() ([]byte, error) {
	containerCount, err := int32WireLength("AssetBundle m_Container count", uint64(len(w.objects)))
	if err != nil {
		return nil, err
	}
	// 按 KCES 使用的内置 AssetBundle TypeTree 顺序写入对象字段
	// Write object fields in the built-in AssetBundle TypeTree order used by KCES
	var buf bytes.Buffer
	bw := binaryio.NewEndianWriter(&buf, binary.LittleEndian)

	// m_Name 是对齐字符串
	// m_Name is an aligned string
	if err := bw.WriteAlignedString("CAB-generated"); err != nil {
		return nil, fmt.Errorf("write AssetBundle m_Name: %w", err)
	}

	// m_PreloadTable 写为空数组
	// m_PreloadTable is an empty array
	if err := bw.WriteUInt32(0); err != nil {
		return nil, fmt.Errorf("write AssetBundle m_PreloadTable size: %w", err)
	}

	// m_Container 将加载名映射到对象 PPtr
	// m_Container maps load names to object PPtrs
	if err := bw.WriteInt32(containerCount); err != nil {
		return nil, fmt.Errorf("write AssetBundle m_Container size: %w", err)
	}
	for _, obj := range w.objects {
		// map 键是对齐加载名字符串
		// The map key is an aligned load-name string
		loadName := nonEmptyLoadName(obj.loadName, obj.name)
		if err := bw.WriteAlignedString(loadName); err != nil {
			return nil, fmt.Errorf("write AssetBundle m_Container key %q: %w", loadName, err)
		}
		// value: AssetInfo { preloadIndex, preloadSize, asset PPtr }
		// map 值依次保存零 preloadIndex、零 preloadSize、当前文件 fileIndex 和对象 pathID
		// The map value stores zero preloadIndex, zero preloadSize, current-file fileIndex, and object pathID in order
		if err := bw.WriteUInt32(0); err != nil { // preloadIndex
			return nil, fmt.Errorf("write AssetBundle m_Container[%q].preloadIndex: %w", loadName, err)
		}
		if err := bw.WriteUInt32(0); err != nil { // preloadSize
			return nil, fmt.Errorf("write AssetBundle m_Container[%q].preloadSize: %w", loadName, err)
		}
		if err := bw.WriteUInt32(0); err != nil { // PPtr fileIndex (0 = this file)
			return nil, fmt.Errorf("write AssetBundle m_Container[%q].fileIndex: %w", loadName, err)
		}
		if err := bw.WriteInt64(obj.pathId); err != nil { // PPtr pathId
			return nil, fmt.Errorf("write AssetBundle m_Container[%q].pathId: %w", loadName, err)
		}
	}

	// m_MainAsset 写为所有字段为零的空 AssetInfo
	// m_MainAsset is a null AssetInfo with all fields set to zero
	if err := bw.WriteUInt32(0); err != nil {
		return nil, fmt.Errorf("write AssetBundle m_MainAsset preload index: %w", err)
	}
	if err := bw.WriteUInt32(0); err != nil {
		return nil, fmt.Errorf("write AssetBundle m_MainAsset preload size: %w", err)
	}
	if err := bw.WriteUInt32(0); err != nil {
		return nil, fmt.Errorf("write AssetBundle m_MainAsset file index: %w", err)
	}
	if err := bw.WriteInt64(0); err != nil {
		return nil, fmt.Errorf("write AssetBundle m_MainAsset path id: %w", err)
	}

	// m_RuntimeCompatibility 写零
	// m_RuntimeCompatibility is zero
	if err := bw.WriteUInt32(0); err != nil {
		return nil, fmt.Errorf("write AssetBundle m_RuntimeCompatibility: %w", err)
	}

	// m_AssetBundleName 写为空字符串
	// m_AssetBundleName is an empty string
	if err := bw.WriteAlignedString(""); err != nil {
		return nil, fmt.Errorf("write AssetBundle m_AssetBundleName: %w", err)
	}

	// m_Dependencies 写为空数组
	// m_Dependencies is an empty array
	if err := bw.WriteUInt32(0); err != nil {
		return nil, fmt.Errorf("write AssetBundle m_Dependencies size: %w", err)
	}

	// m_IsStreamedSceneAssetBundle 写为 false 并按 TypeTree 对齐
	// m_IsStreamedSceneAssetBundle is false and aligned according to the TypeTree
	if err := bw.WriteByte(0); err != nil {
		return nil, fmt.Errorf("write AssetBundle m_IsStreamedSceneAssetBundle: %w", err)
	}
	if err := bw.Align(4); err != nil {
		return nil, fmt.Errorf("align AssetBundle m_IsStreamedSceneAssetBundle: %w", err)
	}

	// Unity 2022.3 KCES 样本含 m_ExplicitDataLayout、m_PathFlags 和 m_SceneHashes
	// 省略这些字段会产生 Unity 内置 TypeTree 无法反序列化的截断 AssetBundle 对象
	// Unity 2022.3 KCES samples contain m_ExplicitDataLayout, m_PathFlags, and m_SceneHashes
	// Omitting them leaves a truncated AssetBundle object that Unity's built-in TypeTree cannot deserialize
	if err := bw.WriteInt32(0); err != nil { // m_ExplicitDataLayout
		return nil, fmt.Errorf("write AssetBundle m_ExplicitDataLayout: %w", err)
	}
	if err := bw.WriteInt32(0); err != nil { // m_PathFlags
		return nil, fmt.Errorf("write AssetBundle m_PathFlags: %w", err)
	}
	if err := bw.WriteUInt32(0); err != nil { // m_SceneHashes (empty map)
		return nil, fmt.Errorf("write AssetBundle m_SceneHashes size: %w", err)
	}
	return buf.Bytes(), nil
}

// encodeTextAssetData 按内置 TypeTree 顺序编码 TextAsset 的 m_Name 和 m_Script
// encodeTextAssetData encodes TextAsset m_Name and m_Script in built-in TypeTree order
func encodeTextAssetData(name string, script []byte) ([]byte, error) {
	scriptLength, err := int32WireLength("TextAsset m_Script length", uint64(len(script)))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	bw := binaryio.NewEndianWriter(&buf, binary.LittleEndian)

	// m_Name 是对齐字符串
	// m_Name is an aligned string
	if err := bw.WriteAlignedString(name); err != nil {
		return nil, fmt.Errorf("write TextAsset m_Name: %w", err)
	}

	// m_Script 依次保存长度、字节数据和四字节对齐填充
	// m_Script stores length, byte data, and four-byte alignment padding in order
	if err := bw.WriteInt32(scriptLength); err != nil {
		return nil, fmt.Errorf("write TextAsset m_Script length: %w", err)
	}
	if err := bw.WriteBytes(script); err != nil {
		return nil, fmt.Errorf("write TextAsset m_Script data: %w", err)
	}
	if err := bw.Align(4); err != nil {
		return nil, fmt.Errorf("align TextAsset m_Script: %w", err)
	}

	return buf.Bytes(), nil
}

// encodeTexture2DData 按 Unity 2022.3 内置 TypeTree 编码内联 RGBA32 Texture2D
// encodeTexture2DData encodes an inline RGBA32 Texture2D using the Unity 2022.3 built-in TypeTree
func encodeTexture2DData(unityVersion string, name string, width, height int64, imageData []byte) ([]byte, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid Texture2D dimensions %dx%d", width, height)
	}
	if uint64(width) > uint64(math.MaxInt32) || uint64(height) > uint64(math.MaxInt32) {
		return nil, fmt.Errorf("Texture2D dimensions %dx%d exceed Int32 wire range", width, height)
	}
	const bytesPerPixel = uint64(4)
	if uint64(width) > uint64(math.MaxInt32)/bytesPerPixel/uint64(height) {
		return nil, fmt.Errorf("Texture2D dimensions %dx%d exceed inline RGBA32 size limit", width, height)
	}
	expectedSize := uint64(width) * uint64(height) * bytesPerPixel
	if uint64(len(imageData)) != expectedSize {
		return nil, fmt.Errorf("Texture2D RGBA32 data size %d does not match %dx%d (%d bytes)", len(imageData), width, height, expectedSize)
	}
	imageDataLength, err := int32WireLength("Texture2D image data length", uint64(len(imageData)))
	if err != nil {
		return nil, err
	}
	if err := validateSerializedFileUnityVersion(unityVersion); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	bw := binaryio.NewEndianWriter(&buf, binary.LittleEndian)

	// m_Name 是对齐字符串
	// m_Name is an aligned string
	if err := bw.WriteAlignedString(name); err != nil {
		return nil, fmt.Errorf("write Texture2D m_Name: %w", err)
	}

	// m_ForcedFallbackFormat 写为 RGBA32 枚举值
	// m_ForcedFallbackFormat is the RGBA32 enum value
	if err := bw.WriteUInt32(uint32(TextureFormatRGBA32)); err != nil {
		return nil, fmt.Errorf("write Texture2D m_ForcedFallbackFormat: %w", err)
	}
	// m_DownscaleFallback 与 m_IsAlphaChannelOptional 是相邻 bool，KCES Unity TypeTree 将 AlignBytes 附在第二个字段上
	// m_DownscaleFallback and m_IsAlphaChannelOptional are adjacent booleans, with AlignBytes attached to the second field in KCES Unity TypeTrees
	if err := bw.WriteByte(0); err != nil {
		return nil, fmt.Errorf("write Texture2D m_DownscaleFallback: %w", err)
	}
	if err := bw.WriteByte(0); err != nil {
		return nil, fmt.Errorf("write Texture2D m_IsAlphaChannelOptional: %w", err)
	}
	if err := bw.Align(4); err != nil {
		return nil, fmt.Errorf("align Texture2D m_IsAlphaChannelOptional: %w", err)
	}

	// m_Width 保存像素宽度
	// m_Width stores pixel width
	if err := bw.WriteInt32(int32(width)); err != nil {
		return nil, fmt.Errorf("write Texture2D m_Width: %w", err)
	}
	// m_Height 保存像素高度
	// m_Height stores pixel height
	if err := bw.WriteInt32(int32(height)); err != nil {
		return nil, fmt.Errorf("write Texture2D m_Height: %w", err)
	}

	// m_CompleteImageSize 保存内联 RGBA32 数据长度
	// m_CompleteImageSize stores inline RGBA32 data length
	if err := bw.WriteInt32(imageDataLength); err != nil {
		return nil, fmt.Errorf("write Texture2D m_CompleteImageSize: %w", err)
	}

	// m_MipsStripped 写零
	// m_MipsStripped is zero
	if err := bw.WriteInt32(0); err != nil {
		return nil, fmt.Errorf("write Texture2D m_MipsStripped: %w", err)
	}

	// m_TextureFormat 写为 4，即 RGBA32
	// m_TextureFormat is 4 for RGBA32
	if err := bw.WriteInt32(4); err != nil {
		return nil, fmt.Errorf("write Texture2D m_TextureFormat: %w", err)
	}

	// m_MipCount 写为单层
	// m_MipCount is one
	if err := bw.WriteInt32(1); err != nil {
		return nil, fmt.Errorf("write Texture2D m_MipCount: %w", err)
	}

	// m_IsReadable、m_IsPreProcessed 和 Unity 2022.3 mipmap limit 字段位于 m_StreamingMipmaps 之前
	// m_IsReadable, m_IsPreProcessed, and the Unity 2022.3 mipmap-limit fields are serialized before m_StreamingMipmaps
	if err := bw.WriteByte(1); err != nil {
		return nil, fmt.Errorf("write Texture2D m_IsReadable: %w", err)
	}
	if err := bw.WriteByte(0); err != nil {
		return nil, fmt.Errorf("write Texture2D m_IsPreProcessed: %w", err)
	}
	// Unity 2022.3 写 m_IgnoreMipmapLimit 并随后写 m_MipmapLimitGroupName
	// Unity 2022.3 writes m_IgnoreMipmapLimit followed by m_MipmapLimitGroupName
	if err := bw.WriteByte(0); err != nil {
		return nil, fmt.Errorf("write Texture2D m_IgnoreMipmapLimit: %w", err)
	}
	if err := bw.Align(4); err != nil {
		return nil, fmt.Errorf("align Texture2D m_IgnoreMipmapLimit: %w", err)
	}
	if err := bw.WriteAlignedString(""); err != nil {
		return nil, fmt.Errorf("write Texture2D m_MipmapLimitGroupName: %w", err)
	}

	// m_StreamingMipmaps 写为 false 并对齐到四字节
	// m_StreamingMipmaps is false and aligned to four bytes
	if err := bw.WriteByte(0); err != nil {
		return nil, fmt.Errorf("write Texture2D m_StreamingMipmaps: %w", err)
	}
	if err := bw.Align(4); err != nil {
		return nil, fmt.Errorf("align Texture2D m_StreamingMipmaps: %w", err)
	}

	// m_StreamingMipmapsPriority 写零
	// m_StreamingMipmapsPriority is zero
	if err := bw.WriteInt32(0); err != nil {
		return nil, fmt.Errorf("write Texture2D m_StreamingMipmapsPriority: %w", err)
	}

	// m_ImageCount 写为一张图像
	// m_ImageCount is one image
	if err := bw.WriteInt32(1); err != nil {
		return nil, fmt.Errorf("write Texture2D m_ImageCount: %w", err)
	}

	// m_TextureDimension 写为 2D 枚举值 2
	// m_TextureDimension is the 2D enum value 2
	if err := bw.WriteInt32(2); err != nil {
		return nil, fmt.Errorf("write Texture2D m_TextureDimension: %w", err)
	}

	// m_TextureSettings 依次写 filterMode、aniso、mipBias、wrapU、wrapV 和 wrapW
	// m_TextureSettings writes filterMode, aniso, mipBias, wrapU, wrapV, and wrapW in order
	if err := bw.WriteInt32(1); err != nil {
		return nil, fmt.Errorf("write Texture2D m_TextureSettings.filterMode: %w", err)
	}
	if err := bw.WriteInt32(0); err != nil {
		return nil, fmt.Errorf("write Texture2D m_TextureSettings.aniso: %w", err)
	}
	if err := bw.WriteFloat32(0); err != nil {
		return nil, fmt.Errorf("write Texture2D m_TextureSettings.mipBias: %w", err)
	}
	if err := bw.WriteInt32(0); err != nil {
		return nil, fmt.Errorf("write Texture2D m_TextureSettings.wrapU: %w", err)
	}
	if err := bw.WriteInt32(0); err != nil {
		return nil, fmt.Errorf("write Texture2D m_TextureSettings.wrapV: %w", err)
	}
	if err := bw.WriteInt32(0); err != nil {
		return nil, fmt.Errorf("write Texture2D m_TextureSettings.wrapW: %w", err)
	}

	// m_LightmapFormat 写零
	// m_LightmapFormat is zero
	if err := bw.WriteInt32(0); err != nil {
		return nil, fmt.Errorf("write Texture2D m_LightmapFormat: %w", err)
	}
	// m_ColorSpace 写为 Linear 枚举值 1
	// m_ColorSpace is the Linear enum value 1
	if err := bw.WriteInt32(1); err != nil {
		return nil, fmt.Errorf("write Texture2D m_ColorSpace: %w", err)
	}

	// m_PlatformBlob 写为空并对齐
	// m_PlatformBlob is empty and aligned
	if err := bw.WriteUInt32(0); err != nil {
		return nil, fmt.Errorf("write Texture2D m_PlatformBlob length: %w", err)
	}
	if err := bw.Align(4); err != nil {
		return nil, fmt.Errorf("align Texture2D m_PlatformBlob: %w", err)
	}

	// image data 保存长度、内联 RGBA32 字节和四字节对齐填充
	// image data stores length, inline RGBA32 bytes, and four-byte alignment padding
	if err := bw.WriteInt32(imageDataLength); err != nil {
		return nil, fmt.Errorf("write Texture2D image data length: %w", err)
	}
	if err := bw.WriteBytes(imageData); err != nil {
		return nil, fmt.Errorf("write Texture2D image data: %w", err)
	}
	if err := bw.Align(4); err != nil {
		return nil, fmt.Errorf("align Texture2D image data: %w", err)
	}

	// m_StreamData 的 offset、size 和 path 均写零值，因为图像数据已内联
	// m_StreamData offset, size, and path are zero values because image data is inline
	if err := bw.WriteUInt64(0); err != nil {
		return nil, fmt.Errorf("write Texture2D m_StreamData offset: %w", err)
	}
	if err := bw.WriteUInt32(0); err != nil {
		return nil, fmt.Errorf("write Texture2D m_StreamData size: %w", err)
	}
	if err := bw.WriteAlignedString(""); err != nil {
		return nil, fmt.Errorf("write Texture2D m_StreamData path: %w", err)
	}

	return buf.Bytes(), nil
}

// unity2022Texture2DTypeTree 构造与生成的内联 RGBA32 Texture2D 正文严格一致的 Unity 2022.3 TypeTree
// unity2022Texture2DTypeTree constructs a Unity 2022.3 TypeTree that exactly matches generated inline RGBA32 Texture2D payloads
func unity2022Texture2DTypeTree() TypeTreeType {
	tree := TypeTreeType{TypeId: ClassIDTexture2D, ScriptTypeIndex: -1}
	stringOffset := func(value string) uint32 {
		offset := uint32(len(tree.StringBuffer))
		tree.StringBuffer = append(tree.StringBuffer, value...)
		tree.StringBuffer = append(tree.StringBuffer, 0)
		return offset
	}
	appendNode := func(level byte, typeName string, name string, byteSize int32, metaFlags uint32) {
		tree.Nodes = append(tree.Nodes, TypeTreeNode{
			Version:    1,
			Level:      level,
			TypeStrOff: stringOffset(typeName),
			NameStrOff: stringOffset(name),
			ByteSize:   byteSize,
			Index:      int32(len(tree.Nodes)),
			MetaFlags:  metaFlags,
		})
	}
	appendNode(0, "Texture2D", "Base", -1, 0)
	appendNode(1, "string", "m_Name", -1, 0x4000)
	appendNode(1, "int", "m_ForcedFallbackFormat", 4, 0)
	appendNode(1, "bool", "m_DownscaleFallback", 1, 0)
	appendNode(1, "bool", "m_IsAlphaChannelOptional", 1, 0x4000)
	appendNode(1, "int", "m_Width", 4, 0)
	appendNode(1, "int", "m_Height", 4, 0)
	appendNode(1, "int", "m_CompleteImageSize", 4, 0)
	appendNode(1, "int", "m_MipsStripped", 4, 0)
	appendNode(1, "int", "m_TextureFormat", 4, 0)
	appendNode(1, "int", "m_MipCount", 4, 0)
	appendNode(1, "bool", "m_IsReadable", 1, 0)
	appendNode(1, "bool", "m_IsPreProcessed", 1, 0)
	appendNode(1, "bool", "m_IgnoreMipmapLimit", 1, 0x4000)
	appendNode(1, "string", "m_MipmapLimitGroupName", -1, 0x4000)
	appendNode(1, "bool", "m_StreamingMipmaps", 1, 0x4000)
	appendNode(1, "int", "m_StreamingMipmapsPriority", 4, 0)
	appendNode(1, "int", "m_ImageCount", 4, 0)
	appendNode(1, "int", "m_TextureDimension", 4, 0)
	appendNode(1, "GLTextureSettings", "m_TextureSettings", 24, 0)
	appendNode(2, "int", "m_FilterMode", 4, 0)
	appendNode(2, "int", "m_Aniso", 4, 0)
	appendNode(2, "float", "m_MipBias", 4, 0)
	appendNode(2, "int", "m_WrapU", 4, 0)
	appendNode(2, "int", "m_WrapV", 4, 0)
	appendNode(2, "int", "m_WrapW", 4, 0)
	appendNode(1, "int", "m_LightmapFormat", 4, 0)
	appendNode(1, "int", "m_ColorSpace", 4, 0)
	appendNode(1, "TypelessData", "m_PlatformBlob", -1, 0x4000)
	appendNode(1, "TypelessData", "image data", -1, 0x4000)
	appendNode(1, "StreamingInfo", "m_StreamData", -1, 0)
	appendNode(2, "unsigned long long", "offset", 8, 0)
	appendNode(2, "unsigned int", "size", 4, 0)
	appendNode(2, "string", "path", -1, 0x4000)
	return tree
}

// validateSerializedFileUnityVersion 将生成的内置对象布局限制为纯目录契约使用的 Unity 2022.3 版本线
// Unity 会按该版本字符串解释原始对象字节，接受旧版或未知布局会使重打包文件无法读取
// validateSerializedFileUnityVersion limits generated built-in object layouts to the Unity 2022.3 line used by the pure-directory contract
// Unity interprets raw object bytes according to this version string, so accepting legacy or unknown layouts could make repacked files unreadable
func validateSerializedFileUnityVersion(unityVersion string) error {
	major, minor, err := parseUnityMajorMinor(unityVersion)
	if err != nil {
		return err
	}
	if major == 2022 && minor == 3 {
		return nil
	}
	return fmt.Errorf("unsupported KCES Unity version %q: writer requires Unity 2022.3", unityVersion)
}

// parseUnityMajorMinor 从 Unity 版本字符串开头解析 major 和 minor 数字
// parseUnityMajorMinor parses major and minor numbers from the start of a Unity version string
func parseUnityMajorMinor(unityVersion string) (int64, int64, error) {
	var major, minor int64
	if n, err := fmt.Sscanf(unityVersion, "%d.%d", &major, &minor); err != nil || n != 2 {
		return 0, 0, fmt.Errorf("invalid Unity version %q", unityVersion)
	}
	return major, minor, nil
}

// layoutDataSection 计算按八字节边界排列的对象偏移和数据区总大小
// layoutDataSection computes eight-byte-aligned object offsets and the total data-section size
func layoutDataSection(objects []sfObject) ([]int64, int64, error) {
	offsets := make([]int64, len(objects))
	var offset int64
	for objectIndex := range objects {
		offset = binaryio.AlignOffset(offset, 8)
		offsets[objectIndex] = offset
		var ok bool
		offset, ok = addNonNegativeInt64(offset, int64(objects[objectIndex].dataSize))
		if !ok {
			return nil, 0, fmt.Errorf("object[%d] data range overflows Int64", objectIndex)
		}
	}
	dataSectionSize := binaryio.AlignOffset(offset, 8)
	if dataSectionSize < offset {
		return nil, 0, fmt.Errorf("final object-data alignment overflows Int64")
	}
	return offsets, dataSectionSize, nil
}

// writeDataSection 按预先计算的偏移流式写入全部对象及对齐填充
// writeDataSection streams all objects and alignment padding at their precomputed offsets
func writeDataSection(out io.Writer, objects []sfObject, offsets []int64, dataSectionSize int64) error {
	if out == nil {
		return fmt.Errorf("nil data-section writer")
	}
	if len(objects) != len(offsets) {
		return fmt.Errorf("object count %d does not match offset count %d", len(objects), len(offsets))
	}
	var position int64
	for objectIndex := range objects {
		if offsets[objectIndex] < position {
			return fmt.Errorf("object[%d] offset %d precedes current position %d", objectIndex, offsets[objectIndex], position)
		}
		if err := writeSerializedZeroes(out, offsets[objectIndex]-position); err != nil {
			return fmt.Errorf("write object[%d] alignment: %w", objectIndex, err)
		}
		if err := writeSerializedObjectData(out, objects[objectIndex]); err != nil {
			return fmt.Errorf("write object[%d] %q: %w", objectIndex, objects[objectIndex].name, err)
		}
		position = offsets[objectIndex] + int64(objects[objectIndex].dataSize)
	}
	if position > dataSectionSize {
		return fmt.Errorf("object data ends at %d beyond section size %d", position, dataSectionSize)
	}
	return writeSerializedZeroes(out, dataSectionSize-position)
}

// writeSerializedObjectData 写入一个内存或流式对象并验证精确字节数
// writeSerializedObjectData writes one in-memory or streamed object and validates its exact byte count
func writeSerializedObjectData(out io.Writer, object sfObject) error {
	if object.writeData == nil {
		if uint64(len(object.data)) != uint64(object.dataSize) {
			return fmt.Errorf("in-memory size %d does not match declared size %d", len(object.data), object.dataSize)
		}
		return writeAbaBytes(out, object.data)
	}
	exact := &serializedExactWriter{writer: out, remaining: int64(object.dataSize)}
	if err := object.writeData(exact); err != nil {
		return err
	}
	if exact.remaining != 0 {
		return fmt.Errorf("stream wrote %d fewer bytes than declared", exact.remaining)
	}
	return nil
}

// serializedExactWriter 限制流式对象写入的剩余字节数 / serializedExactWriter limits the remaining bytes of a streamed object write
type serializedExactWriter struct {
	writer    io.Writer // 下游写入器 / Downstream writer
	remaining int64     // 尚须写入的字节数 / Bytes still required
}

// Write 将字节转发到下游并拒绝超过对象声明大小的写入
// Write forwards bytes downstream and rejects writes beyond the declared object size
func (w *serializedExactWriter) Write(data []byte) (int, error) {
	if w == nil || w.writer == nil {
		return 0, fmt.Errorf("nil exact-size writer")
	}
	if int64(len(data)) > w.remaining {
		return 0, fmt.Errorf("stream attempted to write %d bytes with only %d remaining", len(data), w.remaining)
	}
	n, err := w.writer.Write(data)
	if n < 0 || n > len(data) {
		return 0, fmt.Errorf("writer returned invalid byte count %d for %d-byte buffer", n, len(data))
	}
	w.remaining -= int64(n)
	if err == nil && n == 0 && len(data) != 0 {
		return 0, io.ErrShortWrite
	}
	return n, err
}

// writeSerializedZeroes 以有界缓冲区写入指定数量的零填充
// writeSerializedZeroes writes a requested amount of zero padding through a bounded buffer
func writeSerializedZeroes(out io.Writer, count int64) error {
	if count < 0 {
		return fmt.Errorf("negative zero-padding size %d", count)
	}
	var zeroes [4096]byte
	for count > 0 {
		size := int64(len(zeroes))
		if size > count {
			size = count
		}
		if err := writeAbaBytes(out, zeroes[:size]); err != nil {
			return err
		}
		count -= size
	}
	return nil
}

// collectSerializedTypes 按对象首次出现顺序建立 SerializedTypes 和 ScriptTypes，并验证 MonoBehaviour 的本地脚本目标
// collectSerializedTypes builds SerializedTypes and ScriptTypes in first-object order and validates local MonoScript targets used by MonoBehaviours
func collectSerializedTypes(objects []sfObject) ([]sfSerializedType, []LocalSerializedObjectIdentifier, error) {
	objectClasses := make(map[int64]int32, len(objects))
	for _, obj := range objects {
		if previous, exists := objectClasses[obj.pathId]; exists {
			return nil, nil, fmt.Errorf("objects with class IDs %d and %d use duplicate PathID %d", previous, obj.classId, obj.pathId)
		}
		objectClasses[obj.pathId] = obj.classId
	}

	var serializedTypes []sfSerializedType
	var scriptTypes []LocalSerializedObjectIdentifier
	scriptIndexes := make(map[int64]int16)
	for _, obj := range objects {
		if serializedTypeIndex(serializedTypes, obj) >= 0 {
			continue
		}
		serializedType := sfSerializedType{
			classId:         obj.classId,
			scriptTypeIndex: -1,
			scriptPathId:    obj.scriptPathId,
		}
		if obj.typeTree != nil {
			clonedTree := cloneTypeTreeType(obj.typeTree)
			serializedType.typeTree = &clonedTree
			serializedType.isStrippedType = clonedTree.IsStrippedType
			serializedType.scriptIdHash = clonedTree.ScriptIdHash
			serializedType.typeHash = clonedTree.TypeHash
		}
		if obj.classId == ClassIDMonoBehaviour && obj.scriptPathId != 0 {
			targetClassID, exists := objectClasses[obj.scriptPathId]
			if !exists {
				return nil, nil, fmt.Errorf("MonoBehaviour PathID %d references missing MonoScript PathID %d", obj.pathId, obj.scriptPathId)
			}
			if targetClassID != ClassIDMonoScript {
				return nil, nil, fmt.Errorf("MonoBehaviour PathID %d m_Script targets class ID %d at PathID %d instead of MonoScript", obj.pathId, targetClassID, obj.scriptPathId)
			}
			scriptIndex, exists := scriptIndexes[obj.scriptPathId]
			if !exists {
				if int64(len(scriptTypes)) > int64(math.MaxInt16) {
					return nil, nil, fmt.Errorf("script type count %d exceeds Int16 ScriptTypeIndex range", len(scriptTypes)+1)
				}
				scriptIndex = int16(len(scriptTypes))
				scriptIndexes[obj.scriptPathId] = scriptIndex
				scriptTypes = append(scriptTypes, LocalSerializedObjectIdentifier{
					LocalSerializedFileIndex: 0,
					LocalIdentifierInFile:    obj.scriptPathId,
				})
			}
			serializedType.scriptTypeIndex = scriptIndex
		}
		serializedTypes = append(serializedTypes, serializedType)
	}
	return serializedTypes, scriptTypes, nil
}

// serializedTypeIndex 返回对象应使用的 SerializedType 索引，并按 MonoScript PathID 区分 MonoBehaviour 类型
// serializedTypeIndex returns the SerializedType index for an object and distinguishes MonoBehaviour types by MonoScript PathID
func serializedTypeIndex(serializedTypes []sfSerializedType, obj sfObject) int32 {
	for typeIndex, serializedType := range serializedTypes {
		if serializedType.classId != obj.classId {
			continue
		}
		if obj.classId == ClassIDMonoBehaviour && serializedType.scriptPathId != obj.scriptPathId {
			continue
		}
		if typeTreesEquivalent(serializedType.typeTree, obj.typeTree) {
			return int32(typeIndex)
		}
	}
	return -1
}

// serializedTypesContainTypeTree 判断类型表中是否至少有一个完整 TypeTree
// serializedTypesContainTypeTree reports whether the serialized type table contains at least one complete TypeTree
func serializedTypesContainTypeTree(serializedTypes []sfSerializedType) bool {
	for typeIndex := range serializedTypes {
		if serializedTypes[typeIndex].typeTree != nil {
			return true
		}
	}
	return false
}

// typeTreesEquivalent 判断两个对象是否可以安全共享同一个 SerializedType 项
// typeTreesEquivalent reports whether two objects can safely share one SerializedType entry
func typeTreesEquivalent(left *TypeTreeType, right *TypeTreeType) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.TypeId == right.TypeId &&
		left.IsStrippedType == right.IsStrippedType &&
		left.ScriptIdHash == right.ScriptIdHash &&
		left.TypeHash == right.TypeHash &&
		slices.Equal(left.Nodes, right.Nodes) &&
		slices.Equal(left.StringBuffer, right.StringBuffer) &&
		slices.Equal(left.TypeDependencies, right.TypeDependencies)
}
