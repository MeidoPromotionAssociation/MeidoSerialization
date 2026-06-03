package aba

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio"
)

// SerializedFileWriter 用于生成 Unity SerializedFile v22 格式 / SerializedFileWriter generates Unity SerializedFile v22 data
// 支持写入 TextAsset、Texture2D、原始对象和 AssetBundle 容器对象 / It supports TextAsset, Texture2D, raw objects, and AssetBundle container objects
// 生成的文件可被 KCES 游戏通过 AssetBundle.LoadFromFile 加载 / The generated file can be loaded by KCES through AssetBundle.LoadFromFile
type SerializedFileWriter struct {
	UnityVersion   string             // Unity 版本字符串（如 "2021.3.37f1"）/ Unity version string such as "2021.3.37f1"
	TargetPlatform uint32             // 目标平台 ID（5=Windows Standalone）/ Target platform ID, 5 means Windows Standalone
	objects        []sfObject         // 待写入的对象列表 / Object list to write
	nextPathId     int64              // 下一次自动分配的 PathID / Next automatically allocated PathID
	usedPathIds    map[int64]struct{} // 已占用的 PathID 集合 / Set of already used PathIDs
	err            error              // 延迟到 Write 返回的构建错误 / Build error deferred until Write
}

// sfObject 表示待写入的序列化对象 / sfObject represents one serialized object to write
type sfObject struct {
	pathId   int64  // 对象 PathID / Object PathID
	classId  int32  // Unity ClassID / Unity ClassID
	name     string // 对象内部 m_Name / Internal object m_Name
	loadName string // AssetBundle m_Container 加载名 / AssetBundle m_Container load name
	data     []byte // 序列化后的对象数据 / Serialized object data
}

const (
	sfVersion       uint32 = 22
	defaultPlatform uint32 = 5 // Windows Standalone
)

// NewSerializedFileWriter 创建一个新的 SerializedFile 写入器
func NewSerializedFileWriter(unityVersion string) *SerializedFileWriter {
	if unityVersion == "" {
		unityVersion = "2021.3.37f1"
	}
	return &SerializedFileWriter{
		UnityVersion:   unityVersion,
		TargetPlatform: defaultPlatform,
		nextPathId:     1,
		usedPathIds:    map[int64]struct{}{},
	}
}

// AddTextAsset 添加一个 TextAsset 对象。
// name 为资源名称（如 "xxx.menuassets"），script 为 m_Script 数据。
// 返回分配的 PathId。
func (w *SerializedFileWriter) AddTextAsset(name string, script []byte) int64 {
	return w.AddTextAssetWithLoadName(name, name, script)
}

// AddTextAssetWithPathID 使用首选 PathID 添加 TextAsset / AddTextAssetWithPathID adds a TextAsset using a preferred PathID
// 重打包已解包 bundle 时用于保留内部 Unity PPtr 引用 / Used when repacking extracted bundles so internal Unity PPtr references keep pointing at the same objects
func (w *SerializedFileWriter) AddTextAssetWithPathID(name string, script []byte, pathID int64) int64 {
	return w.AddTextAssetWithLoadNameAndPathID(name, name, script, pathID)
}

// AddTextAssetWithLoadName 添加内部 m_Name 可不同于 LoadAsset key 的 TextAsset / AddTextAssetWithLoadName adds a TextAsset whose internal m_Name can differ from the AssetBundle m_Container key used for LoadAsset
func (w *SerializedFileWriter) AddTextAssetWithLoadName(name string, loadName string, script []byte) int64 {
	return w.AddTextAssetWithLoadNameAndPathID(name, loadName, script, 0)
}

// AddTextAssetWithLoadNameAndPathID 添加带独立 m_Name、加载 key 和首选 PathID 的 TextAsset / AddTextAssetWithLoadNameAndPathID adds a TextAsset with separate internal m_Name, AssetBundle load key, and preferred PathID
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

// AddTexture2D 添加一个 Texture2D 对象。
// name 为资源名称，imageData 为 RGBA32 像素数据，width/height 为尺寸。
// 返回分配的 PathId。
func (w *SerializedFileWriter) AddTexture2D(name string, width, height int, imageData []byte) int64 {
	return w.AddTexture2DWithLoadName(name, name, width, height, imageData)
}

// AddTexture2DWithPathID 使用首选 PathID 添加生成的 Texture2D / AddTexture2DWithPathID adds a generated Texture2D with a preferred PathID
func (w *SerializedFileWriter) AddTexture2DWithPathID(name string, width, height int, imageData []byte, pathID int64) int64 {
	return w.AddTexture2DWithLoadNameAndPathID(name, name, width, height, imageData, pathID)
}

// AddTexture2DWithLoadName 添加内部 m_Name 可不同于 LoadAsset key 的 Texture2D / AddTexture2DWithLoadName adds a generated Texture2D whose internal m_Name can differ from the AssetBundle m_Container key used for LoadAsset
func (w *SerializedFileWriter) AddTexture2DWithLoadName(name string, loadName string, width, height int, imageData []byte) int64 {
	return w.AddTexture2DWithLoadNameAndPathID(name, loadName, width, height, imageData, 0)
}

// AddTexture2DWithLoadNameAndPathID 添加带独立 m_Name、加载 key 和首选 PathID 的 Texture2D / AddTexture2DWithLoadNameAndPathID adds a generated Texture2D with separate internal m_Name, AssetBundle load key, and preferred PathID
func (w *SerializedFileWriter) AddTexture2DWithLoadNameAndPathID(name string, loadName string, width, height int, imageData []byte, pathID int64) int64 {
	data, err := encodeTexture2DData(name, width, height, imageData)
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

// AddRawObject 添加一个原始数据对象（如 Mesh）。
// 返回分配的 PathId。
func (w *SerializedFileWriter) AddRawObject(classId int32, name string, data []byte) int64 {
	return w.AddRawObjectWithLoadNameAndPathID(classId, name, name, data, 0)
}

// AddRawObjectWithPathID 使用首选 PathID 添加原始 Unity 序列化对象 / AddRawObjectWithPathID adds a raw serialized Unity object with a preferred PathID
// 如果请求的 PathID 为 0 或已占用，会重新分配以保持 SerializedFile 有效 / If the requested PathID is zero or already used, a fresh PathID is allocated to keep the SerializedFile valid
func (w *SerializedFileWriter) AddRawObjectWithPathID(classId int32, name string, data []byte, pathID int64) int64 {
	return w.AddRawObjectWithLoadNameAndPathID(classId, name, name, data, pathID)
}

// AddRawObjectWithLoadName 添加内部 m_Name 可不同于 LoadAsset key 的原始 Unity 对象 / AddRawObjectWithLoadName adds a raw serialized Unity object whose internal m_Name can differ from the AssetBundle m_Container key used for LoadAsset
func (w *SerializedFileWriter) AddRawObjectWithLoadName(classId int32, name string, loadName string, data []byte) int64 {
	return w.AddRawObjectWithLoadNameAndPathID(classId, name, loadName, data, 0)
}

// AddRawObjectWithLoadNameAndPathID 添加带独立 m_Name、加载 key 和首选 PathID 的原始 Unity 对象 / AddRawObjectWithLoadNameAndPathID adds a raw serialized Unity object with separate internal m_Name, AssetBundle load key, and preferred PathID
func (w *SerializedFileWriter) AddRawObjectWithLoadNameAndPathID(classId int32, name string, loadName string, data []byte, pathID int64) int64 {
	if rawObjectHasLeadingName(classId) {
		rewritten, err := rewriteLeadingAlignedName(data, name)
		if err != nil {
			w.setError(fmt.Errorf("rewrite raw object %q leading name: %w", name, err))
			return 0
		}
		data = rewritten
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

func nonEmptyLoadName(loadName string, name string) string {
	if loadName != "" {
		return loadName
	}
	return name
}

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

func (w *SerializedFileWriter) setError(err error) {
	if w.err == nil {
		w.err = err
	}
}

func (w *SerializedFileWriter) reserveOrAllocatePathID(pathID int64) int64 {
	w.ensurePathIDState()
	if pathID == 0 || w.isPathIDUsed(pathID) {
		return w.allocatePathID()
	}
	w.usedPathIds[pathID] = struct{}{}
	if pathID > 0 && pathID >= w.nextPathId {
		w.nextPathId = pathID + 1
	}
	return pathID
}

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

func (w *SerializedFileWriter) isPathIDUsed(pathID int64) bool {
	_, ok := w.usedPathIds[pathID]
	return ok
}

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
		ClassIDSpriteAtlas:
		return true
	default:
		return false
	}
}

func rewriteLeadingAlignedName(data []byte, name string) ([]byte, error) {
	if name == "" || len(data) < 4 {
		return data, nil
	}
	oldLen := int(binary.LittleEndian.Uint32(data[:4]))
	if oldLen <= 0 || oldLen > 4096 {
		return data, nil
	}
	oldEnd := binaryio.AlignOffset(4+oldLen, 4)
	if oldEnd > len(data) {
		return data, nil
	}
	for _, r := range string(data[4 : 4+oldLen]) {
		if r < 0x20 && r != '\t' && r != '\n' && r != '\r' {
			return data, nil
		}
	}

	var buf bytes.Buffer
	bw := binaryio.NewEndianWriter(&buf, binary.LittleEndian)
	if err := bw.WriteAlignedString(name); err != nil {
		return nil, err
	}
	buf.Write(data[oldEnd:])
	return buf.Bytes(), nil
}

// Write 将所有对象写入为完整的 SerializedFile v22 格式。
// 自动追加 AssetBundle 对象（ClassID 142）作为 m_Container 映射。
func (w *SerializedFileWriter) Write(out io.Writer) error {
	if w.err != nil {
		return w.err
	}

	// 追加 AssetBundle 对象
	containerData, err := w.encodeAssetBundleObject()
	if err != nil {
		return fmt.Errorf("encode AssetBundle object: %w", err)
	}
	abPathId := w.allocatePathID()
	allObjects := append(w.objects, sfObject{
		pathId:  abPathId,
		classId: ClassIDAssetBundle,
		name:    "CAB-generated",
		data:    containerData,
	})

	// 收集所有 classId（去重）
	classIds := collectClassIds(allObjects)

	// 构建 metadata
	metadataBuf, err := w.buildMetadata(allObjects, classIds)
	if err != nil {
		return fmt.Errorf("build metadata: %w", err)
	}

	// 计算 header 大小（v22 固定 48 字节）
	headerSize := 48

	// 数据区偏移需要对齐到 16 字节
	dataOffset := binaryio.AlignOffset(headerSize+len(metadataBuf), 16)

	// 构建数据区（每个对象数据 4 字节对齐）
	dataBuf, objectOffsets, err := buildDataSection(allObjects)
	if err != nil {
		return fmt.Errorf("build data section: %w", err)
	}

	// 更新 metadata 中的 offset/size
	metadataBuf, err = w.buildMetadataWithOffsets(allObjects, classIds, objectOffsets)
	if err != nil {
		return fmt.Errorf("build metadata with offsets: %w", err)
	}
	dataOffset = binaryio.AlignOffset(headerSize+len(metadataBuf), 16)

	fileSize := int64(dataOffset) + int64(len(dataBuf))

	// 写入 header（Big-Endian）
	var header bytes.Buffer
	hw := binaryio.NewEndianWriter(&header, binary.BigEndian)
	if err := hw.WriteUInt32(uint32(len(metadataBuf))); err != nil { // MetadataSize (legacy)
		return fmt.Errorf("write header metadata size: %w", err)
	}
	if err := hw.WriteUInt32(uint32(fileSize)); err != nil { // FileSize (legacy)
		return fmt.Errorf("write header file size: %w", err)
	}
	if err := hw.WriteUInt32(sfVersion); err != nil { // Version
		return fmt.Errorf("write header version: %w", err)
	}
	if err := hw.WriteUInt32(uint32(dataOffset)); err != nil { // DataOffset (legacy)
		return fmt.Errorf("write header data offset: %w", err)
	}
	if err := hw.WriteByte(0); err != nil { // Endianness = Little
		return fmt.Errorf("write header endianness: %w", err)
	}
	if err := hw.WriteZeroes(3); err != nil { // padding
		return fmt.Errorf("write header padding: %w", err)
	}
	// v22 extended header
	if err := hw.WriteUInt32(uint32(len(metadataBuf))); err != nil { // MetadataSize
		return fmt.Errorf("write extended header metadata size: %w", err)
	}
	if err := hw.WriteInt64(fileSize); err != nil { // FileSize (int64)
		return fmt.Errorf("write extended header file size: %w", err)
	}
	if err := hw.WriteInt64(int64(dataOffset)); err != nil { // DataOffset (int64)
		return fmt.Errorf("write extended header data offset: %w", err)
	}
	if err := hw.WriteInt64(0); err != nil { // unused
		return fmt.Errorf("write extended header unused field: %w", err)
	}

	if _, err := out.Write(header.Bytes()); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	if _, err := out.Write(metadataBuf); err != nil {
		return fmt.Errorf("write metadata: %w", err)
	}
	// 填充到 dataOffset
	padding := dataOffset - headerSize - len(metadataBuf)
	if padding > 0 {
		if _, err := out.Write(make([]byte, padding)); err != nil {
			return fmt.Errorf("write padding: %w", err)
		}
	}
	if _, err := out.Write(dataBuf); err != nil {
		return fmt.Errorf("write data: %w", err)
	}
	return nil
}

func (w *SerializedFileWriter) buildMetadata(objects []sfObject, classIds []int32) ([]byte, error) {
	return w.buildMetadataWithOffsets(objects, classIds, nil)
}

func (w *SerializedFileWriter) buildMetadataWithOffsets(objects []sfObject, classIds []int32, offsets []int64) ([]byte, error) {
	var buf bytes.Buffer
	bw := binaryio.NewEndianWriter(&buf, binary.LittleEndian)

	// UnityVersion (null-terminated)
	if err := bw.WriteNullString(w.UnityVersion); err != nil {
		return nil, fmt.Errorf("write unity version: %w", err)
	}

	// TargetPlatform
	if err := bw.WriteUInt32(w.TargetPlatform); err != nil {
		return nil, fmt.Errorf("write target platform: %w", err)
	}

	// TypeTreeEnabled = false（不写类型树，游戏通过 ClassID 识别）
	if err := bw.WriteByte(0); err != nil {
		return nil, fmt.Errorf("write type tree enabled: %w", err)
	}

	// TypeCount
	if err := bw.WriteUInt32(uint32(len(classIds))); err != nil {
		return nil, fmt.Errorf("write type count: %w", err)
	}
	for _, cid := range classIds {
		if err := bw.WriteInt32(cid); err != nil { // TypeId
			return nil, fmt.Errorf("write type id %d: %w", cid, err)
		}
		if err := bw.WriteByte(0); err != nil { // IsStrippedType
			return nil, fmt.Errorf("write stripped flag for type %d: %w", cid, err)
		}
		if err := bw.WriteUInt16(0xFFFF); err != nil { // ScriptTypeIndex
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
		if err := bw.WriteInt32(0); err != nil { // TypeDependencies count (v21+)
			return nil, fmt.Errorf("write type dependencies count for type %d: %w", cid, err)
		}
	}

	// AssetInfos count
	if err := bw.WriteUInt32(uint32(len(objects))); err != nil {
		return nil, fmt.Errorf("write asset info count: %w", err)
	}
	for i, obj := range objects {
		// 4-byte align before each entry
		if err := bw.Align(4); err != nil {
			return nil, fmt.Errorf("align asset info[%d]: %w", i, err)
		}
		if err := bw.WriteInt64(obj.pathId); err != nil { // PathId
			return nil, fmt.Errorf("write asset info[%d] path id: %w", i, err)
		}
		var offset int64
		if offsets != nil && i < len(offsets) {
			offset = offsets[i]
		}
		if err := bw.WriteInt64(offset); err != nil { // ByteOffset
			return nil, fmt.Errorf("write asset info[%d] byte offset: %w", i, err)
		}
		if err := bw.WriteUInt32(uint32(len(obj.data))); err != nil { // ByteSize
			return nil, fmt.Errorf("write asset info[%d] byte size: %w", i, err)
		}
		if err := bw.WriteInt32(classIdIndex(classIds, obj.classId)); err != nil { // TypeIndex
			return nil, fmt.Errorf("write asset info[%d] type index: %w", i, err)
		}
	}

	// ExternalFiles count = 0
	if err := bw.WriteUInt32(0); err != nil {
		return nil, fmt.Errorf("write external file count: %w", err)
	}

	// RefTypes count = 0
	if err := bw.WriteUInt32(0); err != nil {
		return nil, fmt.Errorf("write ref type count: %w", err)
	}

	// UserInformation (empty string)
	if err := bw.WriteByte(0); err != nil {
		return nil, fmt.Errorf("write user information: %w", err)
	}
	return buf.Bytes(), nil
}

func (w *SerializedFileWriter) encodeAssetBundleObject() ([]byte, error) {
	// AssetBundle 对象的最小序列化：m_Name + m_Container
	var buf bytes.Buffer
	bw := binaryio.NewEndianWriter(&buf, binary.LittleEndian)

	// m_Name (aligned string)
	if err := bw.WriteAlignedString("CAB-generated"); err != nil {
		return nil, fmt.Errorf("write AssetBundle m_Name: %w", err)
	}

	// m_PreloadTable (empty array)
	if err := bw.WriteUInt32(0); err != nil {
		return nil, fmt.Errorf("write AssetBundle m_PreloadTable size: %w", err)
	}

	// m_Container (map: name → PPtr)
	if err := bw.WriteUInt32(uint32(len(w.objects))); err != nil {
		return nil, fmt.Errorf("write AssetBundle m_Container size: %w", err)
	}
	for _, obj := range w.objects {
		// key: string
		loadName := nonEmptyLoadName(obj.loadName, obj.name)
		if err := bw.WriteAlignedString(loadName); err != nil {
			return nil, fmt.Errorf("write AssetBundle m_Container key %q: %w", loadName, err)
		}
		// value: AssetInfo { preloadIndex, preloadSize, asset PPtr }
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

	// m_MainAsset (empty PPtr)
	if err := bw.WriteUInt32(0); err != nil {
		return nil, fmt.Errorf("write AssetBundle m_MainAsset file index: %w", err)
	}
	if err := bw.WriteInt64(0); err != nil {
		return nil, fmt.Errorf("write AssetBundle m_MainAsset path id: %w", err)
	}

	// m_RuntimeCompatibility
	if err := bw.WriteUInt32(0); err != nil {
		return nil, fmt.Errorf("write AssetBundle m_RuntimeCompatibility: %w", err)
	}

	// m_AssetBundleName
	if err := bw.WriteAlignedString(""); err != nil {
		return nil, fmt.Errorf("write AssetBundle m_AssetBundleName: %w", err)
	}

	// m_Dependencies (empty)
	if err := bw.WriteUInt32(0); err != nil {
		return nil, fmt.Errorf("write AssetBundle m_Dependencies size: %w", err)
	}

	// m_IsStreamedSceneAssetBundle
	if err := bw.WriteByte(0); err != nil {
		return nil, fmt.Errorf("write AssetBundle m_IsStreamedSceneAssetBundle: %w", err)
	}
	if err := bw.Align(4); err != nil {
		return nil, fmt.Errorf("align AssetBundle object: %w", err)
	}
	return buf.Bytes(), nil
}

func encodeTextAssetData(name string, script []byte) ([]byte, error) {
	var buf bytes.Buffer
	bw := binaryio.NewEndianWriter(&buf, binary.LittleEndian)

	// m_Name (aligned string)
	if err := bw.WriteAlignedString(name); err != nil {
		return nil, fmt.Errorf("write TextAsset m_Name: %w", err)
	}

	// m_Script (byte array: length + data + align)
	if err := bw.WriteUInt32(uint32(len(script))); err != nil {
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

func encodeTexture2DData(name string, width, height int, imageData []byte) ([]byte, error) {
	var buf bytes.Buffer
	bw := binaryio.NewEndianWriter(&buf, binary.LittleEndian)

	// m_Name
	if err := bw.WriteAlignedString(name); err != nil {
		return nil, fmt.Errorf("write Texture2D m_Name: %w", err)
	}

	// m_ForcedFallbackFormat (int)
	if err := bw.WriteUInt32(0); err != nil {
		return nil, fmt.Errorf("write Texture2D m_ForcedFallbackFormat: %w", err)
	}
	// m_DownscaleFallback (bool + align)
	if err := bw.WriteByte(0); err != nil {
		return nil, fmt.Errorf("write Texture2D m_DownscaleFallback: %w", err)
	}
	if err := bw.Align(4); err != nil {
		return nil, fmt.Errorf("align Texture2D m_DownscaleFallback: %w", err)
	}

	// m_Width
	if err := bw.WriteInt32(int32(width)); err != nil {
		return nil, fmt.Errorf("write Texture2D m_Width: %w", err)
	}
	// m_Height
	if err := bw.WriteInt32(int32(height)); err != nil {
		return nil, fmt.Errorf("write Texture2D m_Height: %w", err)
	}

	// m_CompleteImageSize
	if err := bw.WriteUInt32(uint32(len(imageData))); err != nil {
		return nil, fmt.Errorf("write Texture2D m_CompleteImageSize: %w", err)
	}

	// m_MipsStripped
	if err := bw.WriteInt32(0); err != nil {
		return nil, fmt.Errorf("write Texture2D m_MipsStripped: %w", err)
	}

	// m_TextureFormat = 4 (RGBA32)
	if err := bw.WriteInt32(4); err != nil {
		return nil, fmt.Errorf("write Texture2D m_TextureFormat: %w", err)
	}

	// m_MipCount
	if err := bw.WriteInt32(1); err != nil {
		return nil, fmt.Errorf("write Texture2D m_MipCount: %w", err)
	}

	// m_IsReadable (bool + align)
	if err := bw.WriteByte(1); err != nil {
		return nil, fmt.Errorf("write Texture2D m_IsReadable: %w", err)
	}
	if err := bw.Align(4); err != nil {
		return nil, fmt.Errorf("align Texture2D m_IsReadable: %w", err)
	}

	// m_StreamingMipmaps (bool + align)
	if err := bw.WriteByte(0); err != nil {
		return nil, fmt.Errorf("write Texture2D m_StreamingMipmaps: %w", err)
	}
	if err := bw.Align(4); err != nil {
		return nil, fmt.Errorf("align Texture2D m_StreamingMipmaps: %w", err)
	}

	// m_StreamingMipmapsPriority
	if err := bw.WriteInt32(0); err != nil {
		return nil, fmt.Errorf("write Texture2D m_StreamingMipmapsPriority: %w", err)
	}

	// m_ImageCount
	if err := bw.WriteInt32(1); err != nil {
		return nil, fmt.Errorf("write Texture2D m_ImageCount: %w", err)
	}

	// m_TextureDimension
	if err := bw.WriteInt32(2); err != nil { // 2D
		return nil, fmt.Errorf("write Texture2D m_TextureDimension: %w", err)
	}

	// m_TextureSettings
	if err := bw.WriteInt32(1); err != nil { // filterMode
		return nil, fmt.Errorf("write Texture2D m_TextureSettings.filterMode: %w", err)
	}
	if err := bw.WriteInt32(0); err != nil { // aniso
		return nil, fmt.Errorf("write Texture2D m_TextureSettings.aniso: %w", err)
	}
	if err := bw.WriteFloat32(0); err != nil { // mipBias
		return nil, fmt.Errorf("write Texture2D m_TextureSettings.mipBias: %w", err)
	}
	if err := bw.WriteInt32(0); err != nil { // wrapU
		return nil, fmt.Errorf("write Texture2D m_TextureSettings.wrapU: %w", err)
	}
	if err := bw.WriteInt32(0); err != nil { // wrapV
		return nil, fmt.Errorf("write Texture2D m_TextureSettings.wrapV: %w", err)
	}
	if err := bw.WriteInt32(0); err != nil { // wrapW
		return nil, fmt.Errorf("write Texture2D m_TextureSettings.wrapW: %w", err)
	}

	// m_LightmapFormat
	if err := bw.WriteInt32(0); err != nil {
		return nil, fmt.Errorf("write Texture2D m_LightmapFormat: %w", err)
	}
	// m_ColorSpace
	if err := bw.WriteInt32(1); err != nil { // Linear
		return nil, fmt.Errorf("write Texture2D m_ColorSpace: %w", err)
	}

	// m_PlatformBlob (empty)
	if err := bw.WriteUInt32(0); err != nil {
		return nil, fmt.Errorf("write Texture2D m_PlatformBlob length: %w", err)
	}
	if err := bw.Align(4); err != nil {
		return nil, fmt.Errorf("align Texture2D m_PlatformBlob: %w", err)
	}

	// image data (byte array)
	if err := bw.WriteUInt32(uint32(len(imageData))); err != nil {
		return nil, fmt.Errorf("write Texture2D image data length: %w", err)
	}
	if err := bw.WriteBytes(imageData); err != nil {
		return nil, fmt.Errorf("write Texture2D image data: %w", err)
	}
	if err := bw.Align(4); err != nil {
		return nil, fmt.Errorf("align Texture2D image data: %w", err)
	}

	// m_StreamData (offset=0, size=0, path="")
	if err := bw.WriteUInt32(0); err != nil {
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

func buildDataSection(objects []sfObject) ([]byte, []int64, error) {
	var buf bytes.Buffer
	bw := binaryio.NewEndianWriter(&buf, binary.LittleEndian)
	offsets := make([]int64, len(objects))
	for i, obj := range objects {
		if err := bw.Align(4); err != nil {
			return nil, nil, fmt.Errorf("align object[%d]: %w", i, err)
		}
		offsets[i] = int64(buf.Len())
		if err := bw.WriteBytes(obj.data); err != nil {
			return nil, nil, fmt.Errorf("write object[%d] data: %w", i, err)
		}
	}
	return buf.Bytes(), offsets, nil
}

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

func classIdIndex(classIds []int32, id int32) int32 {
	for i, cid := range classIds {
		if cid == id {
			return int32(i)
		}
	}
	return 0
}
