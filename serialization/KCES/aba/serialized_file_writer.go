package aba

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
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
	data := encodeTextAssetData(name, script)
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
	data := encodeTexture2DData(name, width, height, imageData)
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
		data = rewriteLeadingAlignedName(data, name)
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

func rewriteLeadingAlignedName(data []byte, name string) []byte {
	if name == "" || len(data) < 4 {
		return data
	}
	oldLen := int(binary.LittleEndian.Uint32(data[:4]))
	if oldLen <= 0 || oldLen > 4096 {
		return data
	}
	oldEnd := alignN(4+oldLen, 4)
	if oldEnd > len(data) {
		return data
	}
	for _, r := range string(data[4 : 4+oldLen]) {
		if r < 0x20 && r != '\t' && r != '\n' && r != '\r' {
			return data
		}
	}

	var buf bytes.Buffer
	writeAlignedString(&buf, binary.LittleEndian, name)
	buf.Write(data[oldEnd:])
	return buf.Bytes()
}

// Write 将所有对象写入为完整的 SerializedFile v22 格式。
// 自动追加 AssetBundle 对象（ClassID 142）作为 m_Container 映射。
func (w *SerializedFileWriter) Write(out io.Writer) error {
	// 追加 AssetBundle 对象
	containerData := w.encodeAssetBundleObject()
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
	metadataBuf := w.buildMetadata(allObjects, classIds)

	// 计算 header 大小（v22 固定 48 字节）
	headerSize := 48

	// 数据区偏移需要对齐到 16 字节
	dataOffset := align16(headerSize + len(metadataBuf))

	// 构建数据区（每个对象数据 4 字节对齐）
	dataBuf, objectOffsets := buildDataSection(allObjects)

	// 更新 metadata 中的 offset/size
	metadataBuf = w.buildMetadataWithOffsets(allObjects, classIds, objectOffsets)
	dataOffset = align16(headerSize + len(metadataBuf))

	fileSize := int64(dataOffset) + int64(len(dataBuf))

	// 写入 header（Big-Endian）
	var header bytes.Buffer
	binary.Write(&header, binary.BigEndian, uint32(len(metadataBuf))) // MetadataSize (legacy)
	binary.Write(&header, binary.BigEndian, uint32(fileSize))         // FileSize (legacy)
	binary.Write(&header, binary.BigEndian, sfVersion)                // Version
	binary.Write(&header, binary.BigEndian, uint32(dataOffset))       // DataOffset (legacy)
	header.WriteByte(0)                                               // Endianness = Little
	header.Write([]byte{0, 0, 0})                                     // padding
	// v22 extended header
	binary.Write(&header, binary.BigEndian, uint32(len(metadataBuf))) // MetadataSize
	binary.Write(&header, binary.BigEndian, fileSize)                 // FileSize (int64)
	binary.Write(&header, binary.BigEndian, int64(dataOffset))        // DataOffset (int64)
	binary.Write(&header, binary.BigEndian, int64(0))                 // unused

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

func (w *SerializedFileWriter) buildMetadata(objects []sfObject, classIds []int32) []byte {
	return w.buildMetadataWithOffsets(objects, classIds, nil)
}

func (w *SerializedFileWriter) buildMetadataWithOffsets(objects []sfObject, classIds []int32, offsets []int64) []byte {
	var buf bytes.Buffer
	le := binary.LittleEndian

	// UnityVersion (null-terminated)
	buf.WriteString(w.UnityVersion)
	buf.WriteByte(0)

	// TargetPlatform
	writU32(&buf, le, w.TargetPlatform)

	// TypeTreeEnabled = false（不写类型树，游戏通过 ClassID 识别）
	buf.WriteByte(0)

	// TypeCount
	writU32(&buf, le, uint32(len(classIds)))
	for _, cid := range classIds {
		writI32(&buf, le, cid)    // TypeId
		buf.WriteByte(0)          // IsStrippedType
		writU16(&buf, le, 0xFFFF) // ScriptTypeIndex
		if cid == ClassIDMonoBehaviour {
			buf.Write(make([]byte, 16)) // ScriptIdHash
		}
		buf.Write(make([]byte, 16)) // TypeHash (zeroed)
		writI32(&buf, le, 0)        // TypeDependencies count (v21+)
	}

	// AssetInfos count
	writU32(&buf, le, uint32(len(objects)))
	for i, obj := range objects {
		// 4-byte align before each entry
		alignBuf(&buf, 4)
		writI64(&buf, le, obj.pathId) // PathId
		var offset int64
		if offsets != nil && i < len(offsets) {
			offset = offsets[i]
		}
		writI64(&buf, le, offset)                              // ByteOffset
		writU32(&buf, le, uint32(len(obj.data)))               // ByteSize
		writI32(&buf, le, classIdIndex(classIds, obj.classId)) // TypeIndex
	}

	// ExternalFiles count = 0
	writU32(&buf, le, 0)

	// RefTypes count = 0
	writU32(&buf, le, 0)

	// UserInformation (empty string)
	buf.WriteByte(0)

	return buf.Bytes()
}

func (w *SerializedFileWriter) encodeAssetBundleObject() []byte {
	// AssetBundle 对象的最小序列化：m_Name + m_Container
	var buf bytes.Buffer
	le := binary.LittleEndian

	// m_Name (aligned string)
	writeAlignedString(&buf, le, "CAB-generated")

	// m_PreloadTable (empty array)
	writU32(&buf, le, 0)

	// m_Container (map: name → PPtr)
	writU32(&buf, le, uint32(len(w.objects)))
	for _, obj := range w.objects {
		// key: string
		writeAlignedString(&buf, le, nonEmptyLoadName(obj.loadName, obj.name))
		// value: AssetInfo { preloadIndex, preloadSize, asset PPtr }
		writU32(&buf, le, 0)          // preloadIndex
		writU32(&buf, le, 0)          // preloadSize
		writU32(&buf, le, 0)          // PPtr fileIndex (0 = this file)
		writI64(&buf, le, obj.pathId) // PPtr pathId
	}

	// m_MainAsset (empty PPtr)
	writU32(&buf, le, 0)
	writI64(&buf, le, 0)

	// m_RuntimeCompatibility
	writU32(&buf, le, 0)

	// m_AssetBundleName
	writeAlignedString(&buf, le, "")

	// m_Dependencies (empty)
	writU32(&buf, le, 0)

	// m_IsStreamedSceneAssetBundle
	buf.WriteByte(0)
	alignBuf(&buf, 4)

	return buf.Bytes()
}

func encodeTextAssetData(name string, script []byte) []byte {
	var buf bytes.Buffer
	le := binary.LittleEndian

	// m_Name (aligned string)
	writeAlignedString(&buf, le, name)

	// m_Script (byte array: length + data + align)
	writU32(&buf, le, uint32(len(script)))
	buf.Write(script)
	alignBuf(&buf, 4)

	return buf.Bytes()
}

func encodeTexture2DData(name string, width, height int, imageData []byte) []byte {
	var buf bytes.Buffer
	le := binary.LittleEndian

	// m_Name
	writeAlignedString(&buf, le, name)

	// m_ForcedFallbackFormat (int)
	writU32(&buf, le, 0)
	// m_DownscaleFallback (bool + align)
	buf.WriteByte(0)
	alignBuf(&buf, 4)

	// m_Width
	writI32(&buf, le, int32(width))
	// m_Height
	writI32(&buf, le, int32(height))

	// m_CompleteImageSize
	writU32(&buf, le, uint32(len(imageData)))

	// m_MipsStripped
	writI32(&buf, le, 0)

	// m_TextureFormat = 4 (RGBA32)
	writI32(&buf, le, 4)

	// m_MipCount
	writI32(&buf, le, 1)

	// m_IsReadable (bool + align)
	buf.WriteByte(1)
	alignBuf(&buf, 4)

	// m_StreamingMipmaps (bool + align)
	buf.WriteByte(0)
	alignBuf(&buf, 4)

	// m_StreamingMipmapsPriority
	writI32(&buf, le, 0)

	// m_ImageCount
	writI32(&buf, le, 1)

	// m_TextureDimension
	writI32(&buf, le, 2) // 2D

	// m_TextureSettings
	writI32(&buf, le, 1)               // filterMode
	writI32(&buf, le, 0)               // aniso
	binary.Write(&buf, le, float32(0)) // mipBias
	writI32(&buf, le, 0)               // wrapU
	writI32(&buf, le, 0)               // wrapV
	writI32(&buf, le, 0)               // wrapW

	// m_LightmapFormat
	writI32(&buf, le, 0)
	// m_ColorSpace
	writI32(&buf, le, 1) // Linear

	// m_PlatformBlob (empty)
	writU32(&buf, le, 0)
	alignBuf(&buf, 4)

	// image data (byte array)
	writU32(&buf, le, uint32(len(imageData)))
	buf.Write(imageData)
	alignBuf(&buf, 4)

	// m_StreamData (offset=0, size=0, path="")
	writU32(&buf, le, 0)
	writU32(&buf, le, 0)
	writeAlignedString(&buf, le, "")

	return buf.Bytes()
}

func buildDataSection(objects []sfObject) ([]byte, []int64) {
	var buf bytes.Buffer
	offsets := make([]int64, len(objects))
	for i, obj := range objects {
		alignBuf(&buf, 4)
		offsets[i] = int64(buf.Len())
		buf.Write(obj.data)
	}
	return buf.Bytes(), offsets
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

func writeAlignedString(buf *bytes.Buffer, order binary.ByteOrder, s string) {
	b := []byte(s)
	writU32(buf, order, uint32(len(b)))
	buf.Write(b)
	alignBuf(buf, 4)
}

func writU32(buf *bytes.Buffer, order binary.ByteOrder, v uint32) {
	b := make([]byte, 4)
	order.PutUint32(b, v)
	buf.Write(b)
}

func writI32(buf *bytes.Buffer, order binary.ByteOrder, v int32) {
	b := make([]byte, 4)
	order.PutUint32(b, uint32(v))
	buf.Write(b)
}

func writU16(buf *bytes.Buffer, order binary.ByteOrder, v uint16) {
	b := make([]byte, 2)
	order.PutUint16(b, v)
	buf.Write(b)
}

func writI64(buf *bytes.Buffer, order binary.ByteOrder, v int64) {
	b := make([]byte, 8)
	order.PutUint64(b, uint64(v))
	buf.Write(b)
}

func align16(n int) int {
	return (n + 15) &^ 15
}

func alignN(n int, alignment int) int {
	if alignment <= 0 {
		return n
	}
	return (n + alignment - 1) &^ (alignment - 1)
}

func alignBuf(buf *bytes.Buffer, alignment int) {
	rem := buf.Len() % alignment
	if rem != 0 {
		buf.Write(make([]byte, alignment-rem))
	}
}
