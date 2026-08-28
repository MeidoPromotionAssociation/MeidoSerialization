package aba

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/binaryio"
)

const (
	// NativeUnityObjectFormatVersion 是独立 Unity 对象文件的当前格式版本
	// NativeUnityObjectFormatVersion is the current standalone Unity object file format version
	NativeUnityObjectFormatVersion uint32 = 1

	nativeUnityObjectHeaderSize    int64  = 72
	nativeUnityObjectNodeSize      int64  = 32
	nativeUnityObjectFlagStripped  uint32 = 1 << 0
	nativeUnityObjectFlagBigEndian uint32 = 1 << 1
)

var nativeUnityObjectMagic = [8]byte{'M', 'S', 'K', 'C', 'E', 'S', 'O', '1'}

// NativeUnityObjectHeader 描述独立 Unity 对象文件中自带的 ClassID、TypeTree 和正文范围 / NativeUnityObjectHeader describes the embedded ClassID, TypeTree, and payload range of a standalone Unity object file
type NativeUnityObjectHeader struct {
	ClassID    int32        // Unity ClassID / Unity ClassID
	BigEndian  bool         // 对象正文是否使用大端字节序 / Whether the object payload uses big-endian byte order
	TypeTree   TypeTreeType // 解码和重编码正文所需的完整 TypeTree / Complete TypeTree required to decode and re-encode the payload
	DataOffset int64        // 对象正文相对于文件开头的偏移 / Object payload offset relative to the beginning of the file
	DataSize   uint32       // 对象正文在线格式字节数 / Object payload byte size on the wire
}

// NativeUnityObject 表示无需 sidecar 即可解析和重编码的独立 Unity 对象 / NativeUnityObject represents a standalone Unity object that can be decoded and re-encoded without sidecars
type NativeUnityObject struct {
	ClassID   int32        // Unity ClassID / Unity ClassID
	BigEndian bool         // 对象正文是否使用大端字节序 / Whether the object payload uses big-endian byte order
	TypeTree  TypeTreeType // 解码和重编码正文所需的完整 TypeTree / Complete TypeTree required to decode and re-encode the payload
	Data      []byte       // Unity 序列化对象正文 / Unity serialized object payload
}

// NativeUnityObjectSize 校验独立对象并返回编码后的精确字节数
// NativeUnityObjectSize validates a standalone object and returns its exact encoded byte size
func NativeUnityObjectSize(object *NativeUnityObject) (int64, error) {
	if object == nil {
		return 0, fmt.Errorf("nil native Unity object")
	}
	if err := validateNativeUnityObjectSchema(object.ClassID, &object.TypeTree); err != nil {
		return 0, err
	}
	if uint64(len(object.Data)) > uint64(math.MaxUint32) {
		return 0, fmt.Errorf("native Unity object data length %d exceeds UInt32 range", len(object.Data))
	}
	nodeBytes := int64(len(object.TypeTree.Nodes)) * nativeUnityObjectNodeSize
	size := nativeUnityObjectHeaderSize
	for _, part := range []int64{nodeBytes, int64(len(object.TypeTree.StringBuffer)), int64(len(object.TypeTree.TypeDependencies)) * 4, int64(len(object.Data))} {
		var added bool
		size, added = addNonNegativeInt64(size, part)
		if !added {
			return 0, fmt.Errorf("native Unity object size overflows Int64")
		}
	}
	return size, nil
}

// WriteNativeUnityObject 写入带完整 TypeTree 和 Unity 对象正文的独立文件
// WriteNativeUnityObject writes a standalone file containing a complete TypeTree and Unity object payload
func WriteNativeUnityObject(out io.Writer, object *NativeUnityObject) error {
	if out == nil {
		return fmt.Errorf("nil native Unity object writer")
	}
	if _, err := NativeUnityObjectSize(object); err != nil {
		return err
	}
	nodeCount, err := uint32WireLength("native Unity object TypeTree node count", uint64(len(object.TypeTree.Nodes)))
	if err != nil {
		return err
	}
	stringSize, err := uint32WireLength("native Unity object TypeTree string buffer size", uint64(len(object.TypeTree.StringBuffer)))
	if err != nil {
		return err
	}
	dependencyCount, err := uint32WireLength("native Unity object TypeTree dependency count", uint64(len(object.TypeTree.TypeDependencies)))
	if err != nil {
		return err
	}
	dataSize, err := uint32WireLength("native Unity object data size", uint64(len(object.Data)))
	if err != nil {
		return err
	}

	if err := binaryio.WriteBytes(out, nativeUnityObjectMagic[:]); err != nil {
		return fmt.Errorf("write native Unity object magic: %w", err)
	}
	bw := binaryio.NewEndianWriter(out, binary.LittleEndian)
	if err := bw.WriteUInt32(NativeUnityObjectFormatVersion); err != nil {
		return fmt.Errorf("write native Unity object format version: %w", err)
	}
	if err := bw.WriteInt32(object.ClassID); err != nil {
		return fmt.Errorf("write native Unity object ClassID: %w", err)
	}
	var flags uint32
	if object.TypeTree.IsStrippedType {
		flags |= nativeUnityObjectFlagStripped
	}
	if object.BigEndian {
		flags |= nativeUnityObjectFlagBigEndian
	}
	for fieldIndex, value := range []uint32{flags, nodeCount, stringSize, dependencyCount, dataSize} {
		if err := bw.WriteUInt32(value); err != nil {
			return fmt.Errorf("write native Unity object header field[%d]: %w", fieldIndex, err)
		}
	}
	if err := bw.WriteBytes(object.TypeTree.ScriptIdHash[:]); err != nil {
		return fmt.Errorf("write native Unity object script ID hash: %w", err)
	}
	if err := bw.WriteBytes(object.TypeTree.TypeHash[:]); err != nil {
		return fmt.Errorf("write native Unity object type hash: %w", err)
	}
	if err := bw.WriteUInt32(0); err != nil {
		return fmt.Errorf("write native Unity object reserved field: %w", err)
	}
	for nodeIndex := range object.TypeTree.Nodes {
		if err := writeNativeUnityObjectNode(bw, &object.TypeTree.Nodes[nodeIndex]); err != nil {
			return fmt.Errorf("write native Unity object TypeTree node[%d]: %w", nodeIndex, err)
		}
	}
	if err := bw.WriteBytes(object.TypeTree.StringBuffer); err != nil {
		return fmt.Errorf("write native Unity object TypeTree string buffer: %w", err)
	}
	for dependencyIndex, dependency := range object.TypeTree.TypeDependencies {
		if err := bw.WriteInt32(dependency); err != nil {
			return fmt.Errorf("write native Unity object TypeTree dependency[%d]: %w", dependencyIndex, err)
		}
	}
	if err := bw.WriteBytes(object.Data); err != nil {
		return fmt.Errorf("write native Unity object data: %w", err)
	}
	return nil
}

// EncodeNativeUnityObject 将独立 Unity 对象编码为字节切片
// EncodeNativeUnityObject encodes a standalone Unity object into a byte slice
func EncodeNativeUnityObject(object *NativeUnityObject) ([]byte, error) {
	var out bytes.Buffer
	if err := WriteNativeUnityObject(&out, object); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// ReadNativeUnityObjectHeader 读取并校验独立 Unity 对象头与 TypeTree，读取结束后 reader 位于正文开头
// ReadNativeUnityObjectHeader reads and validates a standalone Unity object header and TypeTree and leaves the reader at the payload start
func ReadNativeUnityObjectHeader(in io.Reader, fileSize int64) (*NativeUnityObjectHeader, error) {
	if in == nil {
		return nil, fmt.Errorf("nil native Unity object reader")
	}
	if fileSize < nativeUnityObjectHeaderSize {
		return nil, fmt.Errorf("native Unity object size %d is smaller than header size %d", fileSize, nativeUnityObjectHeaderSize)
	}
	var magic [8]byte
	if _, err := io.ReadFull(in, magic[:]); err != nil {
		return nil, fmt.Errorf("read native Unity object magic: %w", err)
	}
	if magic != nativeUnityObjectMagic {
		return nil, fmt.Errorf("invalid native Unity object magic %q", magic)
	}
	version, err := binaryio.ReadUInt32(in)
	if err != nil {
		return nil, fmt.Errorf("read native Unity object format version: %w", err)
	}
	if version != NativeUnityObjectFormatVersion {
		return nil, fmt.Errorf("unsupported native Unity object format version %d", version)
	}
	classID, err := binaryio.ReadInt32(in)
	if err != nil {
		return nil, fmt.Errorf("read native Unity object ClassID: %w", err)
	}
	flags, err := binaryio.ReadUInt32(in)
	if err != nil {
		return nil, fmt.Errorf("read native Unity object flags: %w", err)
	}
	if flags & ^uint32(nativeUnityObjectFlagStripped|nativeUnityObjectFlagBigEndian) != 0 {
		return nil, fmt.Errorf("native Unity object has unsupported flags 0x%x", flags)
	}
	counts := make([]uint32, 4)
	for countIndex := range counts {
		counts[countIndex], err = binaryio.ReadUInt32(in)
		if err != nil {
			return nil, fmt.Errorf("read native Unity object length field[%d]: %w", countIndex, err)
		}
	}
	nodeCount, stringSize, dependencyCount, dataSize := counts[0], counts[1], counts[2], counts[3]
	if nodeCount > math.MaxInt32 || stringSize > math.MaxInt32 || dependencyCount > math.MaxInt32 {
		return nil, fmt.Errorf("native Unity object TypeTree lengths exceed SerializedFile Int32 ranges")
	}
	var scriptIDHash [16]byte
	if _, err := io.ReadFull(in, scriptIDHash[:]); err != nil {
		return nil, fmt.Errorf("read native Unity object script ID hash: %w", err)
	}
	var typeHash [16]byte
	if _, err := io.ReadFull(in, typeHash[:]); err != nil {
		return nil, fmt.Errorf("read native Unity object type hash: %w", err)
	}
	reserved, err := binaryio.ReadUInt32(in)
	if err != nil {
		return nil, fmt.Errorf("read native Unity object reserved field: %w", err)
	}
	if reserved != 0 {
		return nil, fmt.Errorf("native Unity object reserved field is %d instead of zero", reserved)
	}

	nodeBytes := int64(nodeCount) * nativeUnityObjectNodeSize
	dataOffset := nativeUnityObjectHeaderSize
	ok := true
	for _, part := range []int64{nodeBytes, int64(stringSize), int64(dependencyCount) * 4} {
		dataOffset, ok = addNonNegativeInt64(dataOffset, part)
		if !ok {
			return nil, fmt.Errorf("native Unity object metadata size overflows Int64")
		}
	}
	expectedSize, ok := addNonNegativeInt64(dataOffset, int64(dataSize))
	if !ok || expectedSize != fileSize {
		return nil, fmt.Errorf("native Unity object declares %d bytes but file size is %d", expectedSize, fileSize)
	}

	tree := TypeTreeType{
		TypeId:          classID,
		IsStrippedType:  flags&nativeUnityObjectFlagStripped != 0,
		ScriptTypeIndex: -1,
		ScriptIdHash:    scriptIDHash,
		TypeHash:        typeHash,
		Nodes:           make([]TypeTreeNode, nodeCount),
	}
	for nodeIndex := int64(0); nodeIndex < int64(nodeCount); nodeIndex++ {
		if err := readNativeUnityObjectNode(in, &tree.Nodes[nodeIndex]); err != nil {
			return nil, fmt.Errorf("read native Unity object TypeTree node[%d]: %w", nodeIndex, err)
		}
	}
	tree.StringBuffer, err = binaryio.ReadBytes(in, int64(stringSize))
	if err != nil {
		return nil, fmt.Errorf("read native Unity object TypeTree string buffer: %w", err)
	}
	tree.TypeDependencies = make([]int32, dependencyCount)
	for dependencyIndex := int64(0); dependencyIndex < int64(dependencyCount); dependencyIndex++ {
		tree.TypeDependencies[dependencyIndex], err = binaryio.ReadInt32(in)
		if err != nil {
			return nil, fmt.Errorf("read native Unity object TypeTree dependency[%d]: %w", dependencyIndex, err)
		}
	}
	if err := validateNativeUnityObjectSchema(classID, &tree); err != nil {
		return nil, err
	}
	return &NativeUnityObjectHeader{
		ClassID:    classID,
		BigEndian:  flags&nativeUnityObjectFlagBigEndian != 0,
		TypeTree:   tree,
		DataOffset: dataOffset,
		DataSize:   dataSize,
	}, nil
}

// ReadNativeUnityObject 读取包含完整 TypeTree 和正文的独立 Unity 对象字节
// ReadNativeUnityObject reads standalone Unity object bytes containing a complete TypeTree and payload
func ReadNativeUnityObject(data []byte) (*NativeUnityObject, error) {
	reader := bytes.NewReader(data)
	header, err := ReadNativeUnityObjectHeader(reader, int64(len(data)))
	if err != nil {
		return nil, err
	}
	payload, err := binaryio.ReadBytes(reader, int64(header.DataSize))
	if err != nil {
		return nil, fmt.Errorf("read native Unity object data: %w", err)
	}
	return &NativeUnityObject{
		ClassID:   header.ClassID,
		BigEndian: header.BigEndian,
		TypeTree:  header.TypeTree,
		Data:      payload,
	}, nil
}

// DecodeValue 使用文件自带的 TypeTree 解码 Unity 对象正文
// DecodeValue decodes the Unity object payload using the TypeTree embedded in the file
func (object *NativeUnityObject) DecodeValue() (*TypeTreeValue, error) {
	root, _, err := object.DecodeValueAndTrailingData()
	return root, err
}

// DecodeValueAndTrailingData 使用文件自带的 TypeTree 解码对象字段，并返回 AudioClip 的内联尾随载荷
// DecodeValueAndTrailingData decodes object fields using the embedded TypeTree and returns an AudioClip's inline trailing payload
func (object *NativeUnityObject) DecodeValueAndTrailingData() (*TypeTreeValue, []byte, error) {
	if object == nil {
		return nil, nil, fmt.Errorf("nil native Unity object")
	}
	af, info, err := object.asAssetsFile()
	if err != nil {
		return nil, nil, err
	}
	root, consumed, objectSize, err := af.readAssetValuePrefix(info)
	if err != nil {
		return nil, nil, err
	}
	if consumed == objectSize {
		return root, nil, nil
	}
	if object.ClassID != ClassIDAudioClip {
		return nil, nil, fmt.Errorf("type tree for class %d left %d unread object bytes", info.TypeId, objectSize-consumed)
	}
	resource := root.Field("m_Resource")
	if resource == nil {
		return nil, nil, fmt.Errorf("AudioClip has %d trailing bytes but no m_Resource field", objectSize-consumed)
	}
	streamInfo, err := readStreamingInfo(resource)
	if err != nil {
		return nil, nil, err
	}
	if streamInfo.Path != "" {
		return nil, nil, fmt.Errorf("AudioClip standalone object still references external stream %q", streamInfo.Path)
	}
	trailing := object.Data[consumed:objectSize]
	if err := validateInlineAudioPayload(1, trailing, streamInfo.Size); err != nil {
		return nil, nil, err
	}
	return root, append([]byte(nil), trailing...), nil
}

// EncodeValue 使用文件自带的 TypeTree 重编码一个已修改的值树
// EncodeValue re-encodes a modified value tree using the TypeTree embedded in the file
func (object *NativeUnityObject) EncodeValue(root *TypeTreeValue) ([]byte, error) {
	if object == nil {
		return nil, fmt.Errorf("nil native Unity object")
	}
	var trailing []byte
	if object.ClassID == ClassIDAudioClip && len(object.Data) != 0 {
		_, preserved, err := object.DecodeValueAndTrailingData()
		if err != nil {
			return nil, err
		}
		trailing = preserved
	}
	return object.EncodeValueWithTrailingData(root, trailing)
}

// EncodeValueWithTrailingData 使用文件自带的 TypeTree 重编码字段，并为 AudioClip 追加经过校验的内联载荷
// EncodeValueWithTrailingData re-encodes fields using the embedded TypeTree and appends validated inline payload data for an AudioClip
func (object *NativeUnityObject) EncodeValueWithTrailingData(root *TypeTreeValue, trailing []byte) ([]byte, error) {
	if object == nil {
		return nil, fmt.Errorf("nil native Unity object")
	}
	af, info, err := object.asAssetsFile()
	if err != nil {
		return nil, err
	}
	encoded, err := af.EncodeAssetValue(info, root)
	if err != nil {
		return nil, err
	}
	if len(trailing) == 0 {
		return encoded, nil
	}
	if object.ClassID != ClassIDAudioClip {
		return nil, fmt.Errorf("class %d object cannot carry trailing payload data", object.ClassID)
	}
	resource := root.Field("m_Resource")
	if resource == nil {
		return nil, fmt.Errorf("AudioClip value has no m_Resource field")
	}
	streamInfo, err := readStreamingInfo(resource)
	if err != nil {
		return nil, err
	}
	if streamInfo.Path != "" || streamInfo.Offset != 0 {
		return nil, fmt.Errorf("AudioClip value references external stream %q at offset %d", streamInfo.Path, streamInfo.Offset)
	}
	if err := validateInlineAudioPayload(1, trailing, streamInfo.Size); err != nil {
		return nil, err
	}
	return append(encoded, trailing...), nil
}

// AudioData 返回 AudioClip 的实际内联音频载荷，不包含尾部对齐填充
// AudioData returns an AudioClip's actual inline audio payload without trailing alignment padding
func (object *NativeUnityObject) AudioData() ([]byte, error) {
	if object == nil || object.ClassID != ClassIDAudioClip {
		return nil, fmt.Errorf("native Unity object is not an AudioClip")
	}
	root, trailing, err := object.DecodeValueAndTrailingData()
	if err != nil {
		return nil, err
	}
	resource := root.Field("m_Resource")
	streamInfo, err := readStreamingInfo(resource)
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), trailing[:streamInfo.Size]...), nil
}

// AssetsFileView 返回仅用于现有 Unity 对象提取器的单对象 SerializedFile 视图
// AssetsFileView returns a one-object SerializedFile view intended for existing Unity object extractors
func (object *NativeUnityObject) AssetsFileView() (*AssetsFile, *AssetInfo, error) {
	if object == nil {
		return nil, nil, fmt.Errorf("nil native Unity object")
	}
	return object.asAssetsFile()
}

// asAssetsFile 构造仅用于复用 TypeTree 编解码器的单对象 SerializedFile 视图
// asAssetsFile constructs a one-object SerializedFile view used only to reuse the TypeTree codec
func (object *NativeUnityObject) asAssetsFile() (*AssetsFile, *AssetInfo, error) {
	if err := validateNativeUnityObjectSchema(object.ClassID, &object.TypeTree); err != nil {
		return nil, nil, err
	}
	dataSize, err := uint32WireLength("native Unity object data size", uint64(len(object.Data)))
	if err != nil {
		return nil, nil, err
	}
	info := &AssetInfo{PathId: 1, ByteSize: dataSize, TypeIdOrIndex: 0, TypeId: object.ClassID, ScriptTypeIndex: -1}
	af := &AssetsFile{
		Header: AssetsFileHeader{Version: 22, FileSize: int64(len(object.Data)), Endianness: object.BigEndian},
		Metadata: AssetsMetadata{
			TypeTreeEnabled: true,
			TypeTreeTypes:   []TypeTreeType{cloneTypeTreeType(&object.TypeTree)},
			AssetInfos:      []AssetInfo{*info},
		},
		Data: object.Data,
	}
	return af, info, nil
}

// validateNativeUnityObjectSchema 验证独立对象中的 ClassID 与 TypeTree 结构
// validateNativeUnityObjectSchema validates the ClassID and TypeTree structure in a standalone object
func validateNativeUnityObjectSchema(classID int32, tree *TypeTreeType) error {
	if classID == 0 {
		return fmt.Errorf("native Unity object ClassID is zero")
	}
	if tree == nil {
		return fmt.Errorf("native Unity object TypeTree is nil")
	}
	if tree.TypeId != classID {
		return fmt.Errorf("native Unity object ClassID %d does not match TypeTree class ID %d", classID, tree.TypeId)
	}
	if len(tree.Nodes) == 0 {
		return fmt.Errorf("native Unity object TypeTree for class %d has no nodes", classID)
	}
	if uint64(len(tree.Nodes)) > uint64(math.MaxInt32) || uint64(len(tree.StringBuffer)) > uint64(math.MaxInt32) || uint64(len(tree.TypeDependencies)) > uint64(math.MaxInt32) {
		return fmt.Errorf("native Unity object TypeTree lengths exceed SerializedFile Int32 ranges")
	}
	if err := validateTypeTreeStringOffsets(tree); err != nil {
		return fmt.Errorf("native Unity object TypeTree: %w", err)
	}
	return nil
}

// writeNativeUnityObjectNode 按 SerializedFile v22 的固定 32 字节布局写入 TypeTree 节点
// writeNativeUnityObjectNode writes a TypeTree node in the fixed 32-byte SerializedFile v22 layout
func writeNativeUnityObjectNode(out *binaryio.EndianWriter, node *TypeTreeNode) error {
	if out == nil || node == nil {
		return fmt.Errorf("nil TypeTree node writer or node")
	}
	if err := out.WriteUInt16(node.Version); err != nil {
		return err
	}
	if err := out.WriteByte(node.Level); err != nil {
		return err
	}
	if err := out.WriteByte(node.TypeFlags); err != nil {
		return err
	}
	if err := out.WriteUInt32(node.TypeStrOff); err != nil {
		return err
	}
	if err := out.WriteUInt32(node.NameStrOff); err != nil {
		return err
	}
	if err := out.WriteInt32(node.ByteSize); err != nil {
		return err
	}
	if err := out.WriteInt32(node.Index); err != nil {
		return err
	}
	if err := out.WriteUInt32(node.MetaFlags); err != nil {
		return err
	}
	return out.WriteUInt64(node.RefTypeHash)
}

// readNativeUnityObjectNode 按 SerializedFile v22 的固定 32 字节布局读取 TypeTree 节点
// readNativeUnityObjectNode reads a TypeTree node in the fixed 32-byte SerializedFile v22 layout
func readNativeUnityObjectNode(in io.Reader, node *TypeTreeNode) error {
	if in == nil || node == nil {
		return fmt.Errorf("nil TypeTree node reader or node")
	}
	var err error
	if node.Version, err = binaryio.ReadUInt16(in); err != nil {
		return err
	}
	if node.Level, err = binaryio.ReadByte(in); err != nil {
		return err
	}
	if node.TypeFlags, err = binaryio.ReadByte(in); err != nil {
		return err
	}
	if node.TypeStrOff, err = binaryio.ReadUInt32(in); err != nil {
		return err
	}
	if node.NameStrOff, err = binaryio.ReadUInt32(in); err != nil {
		return err
	}
	if node.ByteSize, err = binaryio.ReadInt32(in); err != nil {
		return err
	}
	if node.Index, err = binaryio.ReadInt32(in); err != nil {
		return err
	}
	if node.MetaFlags, err = binaryio.ReadUInt32(in); err != nil {
		return err
	}
	node.RefTypeHash, err = binaryio.ReadUInt64(in)
	return err
}
