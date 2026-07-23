package aba

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio"
)

// SerializedFileWriter 用于生成 Unity SerializedFile v22 格式
// 支持写入 TextAsset、Texture2D、原始对象和 Unity AssetBundle 容器对象，生成的文件可被 KCES 游戏通过 AssetBundle.LoadFromFile 加载
// SerializedFileWriter generates Unity SerializedFile v22 data
// It supports TextAsset, Texture2D, raw objects, and Unity AssetBundle container objects; KCES can load generated files through AssetBundle.LoadFromFile
type SerializedFileWriter struct {
	UnityVersion   string             // Unity 版本字符串，如 "2021.3.37f1" / Unity version string such as "2021.3.37f1"
	TargetPlatform uint32             // 目标平台 ID，5 表示 Windows Standalone / Target platform ID, with 5 meaning Windows Standalone
	objects        []sfObject         // 待写入的对象列表 / Object list to write
	nextPathId     int64              // 下一次自动分配的 PathID / Next automatically allocated PathID
	usedPathIds    map[int64]struct{} // 已占用的 PathID 集合 / Set of already used PathIDs
	err            error              // 延迟到 Write 返回的构建错误 / Build error deferred until Write
}

// sfObject 表示待写入的一个序列化对象
// sfObject represents one serialized object to write
type sfObject struct {
	pathId   int64  // 对象 PathID / Object PathID
	classId  int32  // Unity 类 ID / Unity class ID
	name     string // 对象内部 m_Name / Internal object m_Name
	loadName string // AssetBundle m_Container 加载名 / AssetBundle m_Container load name
	data     []byte // 序列化后的对象数据 / Serialized object data
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
		unityVersion = "2021.3.37f1"
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
	actualPathID := w.reserveOrAllocatePathID(pathID)
	w.objects = append(w.objects, sfObject{
		pathId:   actualPathID,
		classId:  ClassIDTextAsset,
		name:     name,
		loadName: nonEmptyLoadName(loadName, name),
		data:     data,
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
	actualPathID := w.reserveOrAllocatePathID(pathID)
	w.objects = append(w.objects, sfObject{
		pathId:   actualPathID,
		classId:  ClassIDTexture2D,
		name:     name,
		loadName: nonEmptyLoadName(loadName, name),
		data:     data,
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
	actualPathID := w.reserveOrAllocatePathID(pathID)
	w.objects = append(w.objects, sfObject{
		pathId:   actualPathID,
		classId:  classId,
		name:     name,
		loadName: nonEmptyLoadName(loadName, name),
		data:     data,
	})
	return actualPathID
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
	for i := range w.objects {
		if _, err := uint32WireLength(fmt.Sprintf("object[%d] %q byte size", i, w.objects[i].name), uint64(len(w.objects[i].data))); err != nil {
			return err
		}
	}

	// 追加一个保存 m_Container 加载映射的 AssetBundle 对象
	// Append an AssetBundle object containing the m_Container load mapping
	containerData, err := w.encodeAssetBundleObject()
	if err != nil {
		return fmt.Errorf("encode AssetBundle object: %w", err)
	}
	abPathId := w.nextAvailablePathID()
	allObjects := make([]sfObject, 0, len(w.objects)+1)
	allObjects = append(allObjects, w.objects...)
	allObjects = append(allObjects, sfObject{
		pathId:  abPathId,
		classId: ClassIDAssetBundle,
		name:    "CAB-generated",
		data:    containerData,
	})

	// 按首次出现顺序收集唯一 class ID
	// Collect unique class IDs in first-seen order
	classIds := collectClassIds(allObjects)

	// 首次构建 metadata 以确定其长度
	// Build metadata once to determine its length
	metadataBuf, err := w.buildMetadata(allObjects, classIds)
	if err != nil {
		return fmt.Errorf("build metadata: %w", err)
	}

	// v22 Header 固定为 48 字节
	// The v22 Header is fixed at 48 bytes
	headerSize := int64(48)

	// 数据区起点对齐到 16 字节
	// Align the data-section start to 16 bytes
	dataOffset := binaryio.AlignOffset(headerSize+int64(len(metadataBuf)), 16)

	// 构建数据区并将每个对象起点对齐到八字节
	// Build the data section with every object start aligned to eight bytes
	dataBuf, objectOffsets, err := buildDataSection(allObjects)
	if err != nil {
		return fmt.Errorf("build data section: %w", err)
	}

	// 使用最终对象偏移重新构建 metadata
	// Rebuild metadata with final object offsets
	metadataBuf, err = w.buildMetadataWithOffsets(allObjects, classIds, objectOffsets)
	if err != nil {
		return fmt.Errorf("build metadata with offsets: %w", err)
	}
	dataOffset = binaryio.AlignOffset(headerSize+int64(len(metadataBuf)), 16)

	fileSize, ok := addNonNegativeInt64(dataOffset, int64(len(dataBuf)))
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
	// v22 扩展头依次保存 MetadataSize、Int64 FileSize、Int64 DataOffset 和未使用的 Int64 零值
	// The v22 extended header stores MetadataSize, Int64 FileSize, Int64 DataOffset, and an unused Int64 zero in order
	if err := hw.WriteUInt32(metadataSize); err != nil { // MetadataSize
		return fmt.Errorf("write extended header metadata size: %w", err)
	}
	if err := hw.WriteInt64(fileSize); err != nil { // FileSize (int64)
		return fmt.Errorf("write extended header file size: %w", err)
	}
	if err := hw.WriteInt64(dataOffset); err != nil { // DataOffset (int64)
		return fmt.Errorf("write extended header data offset: %w", err)
	}
	if err := hw.WriteInt64(0); err != nil { // unused
		return fmt.Errorf("write extended header unused field: %w", err)
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
	if err := writeAbaBytes(out, dataBuf); err != nil {
		return fmt.Errorf("write data: %w", err)
	}
	return nil
}

// buildMetadata 构建对象偏移暂为零的 metadata，用于计算布局
// buildMetadata builds metadata with provisional zero object offsets for layout calculation
func (w *SerializedFileWriter) buildMetadata(objects []sfObject, classIds []int32) ([]byte, error) {
	return w.buildMetadataWithOffsets(objects, classIds, nil)
}

// buildMetadataWithOffsets 构建包含 SerializedTypes、AssetInfos 和空尾部表的 Little-Endian v22 metadata
// buildMetadataWithOffsets builds Little-Endian v22 metadata containing SerializedTypes, AssetInfos, and empty tail tables
func (w *SerializedFileWriter) buildMetadataWithOffsets(objects []sfObject, classIds []int32, offsets []int64) ([]byte, error) {
	if offsets != nil && len(offsets) != len(objects) {
		return nil, fmt.Errorf("object offset count %d does not match object count %d", len(offsets), len(objects))
	}
	typeCount, err := int32WireLength("serialized type count", uint64(len(classIds)))
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

	// TypeTreeEnabled 写为 false，不嵌入类型树，由 Unity 通过 class ID 识别内置对象
	// TypeTreeEnabled is false with no embedded tree, allowing Unity to identify built-in objects by class ID
	if err := bw.WriteByte(0); err != nil {
		return nil, fmt.Errorf("write type tree enabled: %w", err)
	}

	// 写入 SerializedTypes 数量，各 v22 类型记录依次包含 TypeId、IsStrippedType、ScriptTypeIndex、条件 ScriptIdHash 和 TypeHash
	// Write SerializedTypes count; each v22 type record contains TypeId, IsStrippedType, ScriptTypeIndex, conditional ScriptIdHash, and TypeHash in order
	if err := bw.WriteInt32(typeCount); err != nil {
		return nil, fmt.Errorf("write type count: %w", err)
	}
	for _, cid := range classIds {
		if err := bw.WriteInt32(cid); err != nil { // TypeId
			return nil, fmt.Errorf("write type id %d: %w", cid, err)
		}
		if err := bw.WriteByte(0); err != nil { // IsStrippedType
			return nil, fmt.Errorf("write stripped flag for type %d: %w", cid, err)
		}
		if err := bw.WriteInt16(-1); err != nil { // ScriptTypeIndex
			return nil, fmt.Errorf("write script type index for type %d: %w", cid, err)
		}
		if cid == ClassIDMonoBehaviour {
			if err := bw.WriteZeroes(16); err != nil { // ScriptIdHash
				return nil, fmt.Errorf("write script id hash for type %d: %w", cid, err)
			}
		}
		if err := bw.WriteZeroes(16); err != nil { // TypeHash (zeroed)
			return nil, fmt.Errorf("write type hash for type %d: %w", cid, err)
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
		byteSize, err := uint32WireLength(fmt.Sprintf("asset info[%d] byte size", i), uint64(len(obj.data)))
		if err != nil {
			return nil, err
		}
		if err := bw.WriteUInt32(byteSize); err != nil { // ByteSize
			return nil, fmt.Errorf("write asset info[%d] byte size: %w", i, err)
		}
		typeIndex := classIdIndex(classIds, obj.classId)
		if typeIndex < 0 {
			return nil, fmt.Errorf("asset info[%d] class ID %d is absent from serialized types", i, obj.classId)
		}
		if err := bw.WriteInt32(typeIndex); err != nil { // TypeIndex
			return nil, fmt.Errorf("write asset info[%d] type index: %w", i, err)
		}
	}

	// SerializedFile metadata 即使没有 MonoBehaviour 脚本引用，也在 AssetInfos 与 ExternalFiles 之间保存 ScriptTypes 计数，本写入器写零
	// SerializedFile metadata stores ScriptTypes count between AssetInfos and ExternalFiles even without MonoBehaviour references; this writer emits zero
	if err := bw.WriteUInt32(0); err != nil {
		return nil, fmt.Errorf("write script type count: %w", err)
	}

	// ExternalFiles 数量写零
	// ExternalFiles count is zero
	if err := bw.WriteUInt32(0); err != nil {
		return nil, fmt.Errorf("write external file count: %w", err)
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

	// Unity 2020.2 至 2022.3 的全部 KCES 样本都含 m_ExplicitDataLayout、m_PathFlags 和 m_SceneHashes
	// 省略这些字段会产生 Unity 内置 TypeTree 无法反序列化的截断 AssetBundle 对象
	// Every KCES sample from Unity 2020.2 through 2022.3 contains m_ExplicitDataLayout, m_PathFlags, and m_SceneHashes
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

// encodeTexture2DData 按 KCES 对应 Unity 版本的内置 TypeTree 编码内联 RGBA32 Texture2D
// encodeTexture2DData encodes an inline RGBA32 Texture2D using the built-in TypeTree for the corresponding KCES Unity version
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
	newMipmapLimitLayout, err := texture2DUsesMipmapLimitGroup(unityVersion)
	if err != nil {
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

	// m_IsReadable、m_IsPreProcessed 和版本特定的 mipmap limit 字段位于 m_StreamingMipmaps 之前
	// m_IsReadable, m_IsPreProcessed, and version-specific mipmap-limit fields are serialized before m_StreamingMipmaps
	if err := bw.WriteByte(1); err != nil {
		return nil, fmt.Errorf("write Texture2D m_IsReadable: %w", err)
	}
	if err := bw.WriteByte(0); err != nil {
		return nil, fmt.Errorf("write Texture2D m_IsPreProcessed: %w", err)
	}
	if newMipmapLimitLayout {
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
	} else {
		// Unity 2020.2 与 2021.3 使用旧 m_IgnoreMasterTextureLimit 字段
		// Unity 2020.2 and 2021.3 use the legacy m_IgnoreMasterTextureLimit field
		if err := bw.WriteByte(0); err != nil {
			return nil, fmt.Errorf("write Texture2D m_IgnoreMasterTextureLimit: %w", err)
		}
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

// validateSerializedFileUnityVersion 将生成的内置对象布局限制为 KCES 样本中观察到的 Unity 版本线
// Unity 会按该版本字符串解释原始对象字节，静默接受未知版本可能使其余字节完全相同的重打包文件无法读取
// validateSerializedFileUnityVersion limits generated built-in object layouts to Unity lines observed in KCES samples
// Unity interprets raw object bytes according to this version string, so accepting an unknown line could make an otherwise byte-identical repack unreadable
func validateSerializedFileUnityVersion(unityVersion string) error {
	major, minor, err := parseUnityMajorMinor(unityVersion)
	if err != nil {
		return err
	}
	if (major == 2020 && minor == 2) || (major == 2021 && minor == 3) || (major == 2022 && minor == 3) {
		return nil
	}
	return fmt.Errorf("unsupported KCES Unity version %q: supported lines are 2020.2, 2021.3, and 2022.3", unityVersion)
}

// texture2DUsesMipmapLimitGroup 判断 Texture2D 是否使用 Unity 2022 的 mipmap limit group 布局
// texture2DUsesMipmapLimitGroup reports whether Texture2D uses the Unity 2022 mipmap-limit-group layout
func texture2DUsesMipmapLimitGroup(unityVersion string) (bool, error) {
	if err := validateSerializedFileUnityVersion(unityVersion); err != nil {
		return false, err
	}
	major, _, _ := parseUnityMajorMinor(unityVersion)
	return major >= 2022, nil
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

// buildDataSection 将对象数据按八字节边界排列，并返回各对象相对于数据区的偏移
// buildDataSection lays out object data on eight-byte boundaries and returns offsets relative to the data section
func buildDataSection(objects []sfObject) ([]byte, []int64, error) {
	var buf bytes.Buffer
	bw := binaryio.NewEndianWriter(&buf, binary.LittleEndian)
	offsets := make([]int64, len(objects))
	for i, obj := range objects {
		if err := bw.Align(8); err != nil {
			return nil, nil, fmt.Errorf("align object[%d]: %w", i, err)
		}
		offsets[i] = int64(buf.Len())
		if err := bw.WriteBytes(obj.data); err != nil {
			return nil, nil, fmt.Errorf("write object[%d] data: %w", i, err)
		}
	}
	if err := bw.Align(8); err != nil {
		return nil, nil, fmt.Errorf("align final object data: %w", err)
	}
	return buf.Bytes(), offsets, nil
}

// collectClassIds 按首次出现顺序返回对象使用的唯一 class ID
// collectClassIds returns unique class IDs used by objects in first-seen order
func collectClassIds(objects []sfObject) []int32 {
	seen := map[int32]bool{}
	var ids []int32
	for _, obj := range objects {
		if !seen[obj.classId] {
			seen[obj.classId] = true
			ids = append(ids, obj.classId)
		}
	}
	return ids
}

// classIdIndex 返回 class ID 在 SerializedTypes 列表中的索引，未找到时返回 -1
// classIdIndex returns a class ID index in SerializedTypes or -1 when absent
func classIdIndex(classIds []int32, id int32) int32 {
	for i, cid := range classIds {
		if cid == id {
			return int32(i)
		}
	}
	return -1
}
