package ct

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/msgpack"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio"
	"github.com/ugorji/go/codec"
)

// .ct
// KCES 的 Content Table 资源目录文件，外层使用 VirtualDirectory 序列化格式
// 文件头之后连续保存各 VirtualFile 的原始数据，尾部保存 LZ4 Block Array 压缩的 MessagePack 目录结构及其长度
//
//	[7 bytes]  FileSignature: bb c3 aa 9a a6 4d ad
//	[1 byte]   SerializeType: 8e = MessagePack, 00 = MemoryPack
//	[N bytes]  原始文件数据（各 VirtualFile 连续存放）
//	[M bytes]  MessagePack + LZ4 Block Array 压缩的 VirtualDirectory 结构
//	[4 bytes]  M 的值（Little-Endian Int32）
// .ct
// KCES Content Table resource catalog using the serialized VirtualDirectory format
// Raw VirtualFile data is stored contiguously after the header; the tail stores the LZ4 Block Array-compressed
// MessagePack directory structure followed by its length
//	[7 bytes]  FileSignature: bb c3 aa 9a a6 4d ad
//	[1 byte]   SerializeType: 8e = MessagePack, 00 = MemoryPack
//	[N bytes]  Raw file data with VirtualFiles stored contiguously
//	[M bytes]  MessagePack + LZ4 Block Array-compressed VirtualDirectory structure
//	[4 bytes]  M encoded as a Little-Endian Int32

// FileSignature 是 .ct 文件的魔数签名（7 字节），用于验证文件格式
// 对应 C# VirtualDirectory.FileSignature = {0xbb, 0xc3, 0xaa, 0x9a, 0xa6, 0x4d, 0xad}
// FileSignature is the 7-byte magic signature used to validate .ct files
// It matches C# VirtualDirectory.FileSignature = {0xbb, 0xc3, 0xaa, 0x9a, 0xa6, 0x4d, 0xad}
var FileSignature = []byte{0xbb, 0xc3, 0xaa, 0x9a, 0xa6, 0x4d, 0xad}

const (
	// HeaderSize 是文件头大小，由 7 字节签名和 1 字节序列化类型组成
	// HeaderSize is the header width made up of a 7-byte signature and a 1-byte serialization type
	HeaderSize = 8
	// SerializeTypeMsgPack 表示使用 MessagePack 序列化（0x8e）
	// 另一种可能值 0x00 表示 MemoryPack，本库不支持
	// SerializeTypeMsgPack selects MessagePack serialization (0x8e)
	// The other defined value 0x00 selects MemoryPack, which this package does not support
	SerializeTypeMsgPack = 0x8e
	// footerSizeLen 是文件末尾存储 MessagePack 数据长度的字节数（Little-Endian UInt32）
	// footerSizeLen is the number of bytes used for the trailing MessagePack length (Little-Endian UInt32)
	footerSizeLen = 4
	// ctVersion 是 VirtualDirectory 的固定版本号，对应 C# VirtualDirectory.FixVersion = 1000
	// ctVersion is the fixed VirtualDirectory version matching C# VirtualDirectory.FixVersion = 1000
	ctVersion = 1000
)

// ContentTable 表示解析后的 .ct 文件 VirtualDirectory 序列化结构
// .ct 文件是 KCES 游戏的资源目录容器，内部存储 catalog、ExtensionNameList 等虚拟文件
// 游戏通过 CatalogUtility.FromCatalog<T> 读取 .ct 中的 "catalog" 文件获取资源索引
// ContentTable represents a parsed .ct file in serialized VirtualDirectory form
// A .ct file is a KCES resource catalog container storing virtual files such as catalog and ExtensionNameList
// The game reads the "catalog" virtual file through CatalogUtility.FromCatalog<T> to obtain resource indexes
type ContentTable struct {
	Version     int32                               `json:"Version"`               // 保存的根 VirtualDirectory 版本 / Stored root VirtualDirectory version
	Directories map[string]VirtualDirectoryMetadata `json:"Directories,omitempty"` // 子目录路径及其真实版本字段，包含空目录 / Child-directory paths and their real version fields, including empty directories
	Files       map[string]VirtualFile              `json:"Files"`                 // 虚拟文件表，键为文件名，值为位置和大小 / Virtual file table keyed by file name with position and size values
	Raw         []byte                              `json:"-"`                     // 完整文件原始字节，用于按偏移提取虚拟文件内容 / Raw bytes of the full file used to slice virtual file contents by offset
	dataEnd     int64                               // 实际虚拟文件数据区末尾，零值表示使用 len(Raw) / End of the virtual-file payload area; zero means len(Raw)
}

// VirtualDirectoryMetadata 保存子 VirtualDirectory 的真实字段
// 路径在 ContentTable.Directories 中以规范化的斜杠分隔键保存，解码得到的每个目录都会有条目
// VirtualDirectoryMetadata stores the real fields of a child VirtualDirectory
// Paths use canonical slash-separated keys in ContentTable.Directories and every decoded directory has an entry
type VirtualDirectoryMetadata struct {
	Version int32 `json:"Version"` // 子 VirtualDirectory 的版本值 / Child VirtualDirectory version value
}

// VirtualFile 表示虚拟文件系统中的一个文件条目
// 对应 C# VirtualFile 的 MessagePack indexed array [Key(0)=position, Key(1)=size]
// VirtualFile represents one file entry in the virtual file system
// It matches the C# VirtualFile MessagePack indexed array [Key(0)=position, Key(1)=size]
type VirtualFile struct {
	Position int64 `json:"Position"` // 文件数据在 .ct 文件中的绝对字节偏移，从文件开头计算且包含 header / Absolute byte offset of file data inside the .ct file, counted from file start including header
	Size     int32 `json:"Size"`     // 文件数据的字节大小 / File data size in bytes
}

// ReadContentTable 从 reader 中读取并解析 .ct 文件
// 解析流程：验证签名、读取末尾长度、提取 MessagePack 数据、LZ4 解压并解码 VirtualDirectory
// ReadContentTable reads and parses a .ct file from reader
// The procedure validates the signature, reads the trailing length, extracts MessagePack data, decompresses LZ4, and decodes VirtualDirectory
func ReadContentTable(r io.Reader) (*ContentTable, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read .ct file failed: %w", err)
	}

	if len(data) < HeaderSize+footerSizeLen+4 {
		return nil, fmt.Errorf("file too small: %d bytes", len(data))
	}

	for i, b := range FileSignature {
		if data[i] != b {
			return nil, fmt.Errorf("invalid file signature at byte %d: got 0x%02x, want 0x%02x", i, data[i], b)
		}
	}

	if data[7] != SerializeTypeMsgPack {
		return nil, fmt.Errorf("unsupported serialize type: 0x%02x (only MessagePack 0x%02x supported)", data[7], SerializeTypeMsgPack)
	}

	msgpackSize := int64(binary.LittleEndian.Uint32(data[len(data)-footerSizeLen:]))
	if msgpackSize <= 0 || msgpackSize > int64(len(data)-HeaderSize-footerSizeLen) {
		return nil, fmt.Errorf("invalid msgpack size: %d (file size: %d)", msgpackSize, len(data))
	}

	msgpackStart := int64(len(data)-footerSizeLen) - msgpackSize
	msgpackData := data[msgpackStart : len(data)-footerSizeLen]

	decompressed, err := msgpack.DecompressLz4BlockArray(msgpackData)
	if err != nil {
		return nil, fmt.Errorf("decompress VirtualDirectory failed: %w", err)
	}

	ct := &ContentTable{Raw: data, dataEnd: msgpackStart}
	if err := ct.decodeVirtualDirectory(decompressed); err != nil {
		return nil, fmt.Errorf("decode VirtualDirectory failed: %w", err)
	}
	for name := range ct.Files {
		vf := ct.Files[name]
		if vf.Size > 0 && vf.Position < HeaderSize {
			return nil, fmt.Errorf("virtual file %q starts inside the .ct header at %d", name, vf.Position)
		}
		if _, err := ct.GetFileData(name); err != nil {
			return nil, err
		}
	}

	return ct, nil
}

// WriteContentTable 将 ContentTable 序列化为 .ct 格式并写入 writer
// 写入顺序为签名、序列化类型、各文件原始数据、LZ4 压缩的 VirtualDirectory 和长度尾部
// WriteContentTable serializes ContentTable in .ct format and writes it to writer
// The write order is the signature, serialization type, raw file data, LZ4-compressed VirtualDirectory, and trailing length
func WriteContentTable(w io.Writer, ct *ContentTable) error {
	if w == nil {
		return fmt.Errorf("nil content table writer")
	}
	if ct == nil {
		return fmt.Errorf("nil content table")
	}
	type fileEntry struct {
		name string      // 虚拟文件名 / Virtual file name
		file VirtualFile // VirtualFile 线格式形状元数据 / VirtualFile wire-shape metadata
		data []byte      // 虚拟文件数据 / Virtual file data
	}
	names := make([]string, 0, len(ct.Files))
	for name := range ct.Files {
		names = append(names, name)
	}
	sort.Strings(names)
	entries := make([]fileEntry, 0, len(names))
	canonicalNames := make(map[string]string, len(names))
	for _, name := range names {
		file := ct.Files[name]
		canonicalName, err := canonicalVirtualPath(name)
		if err != nil {
			return fmt.Errorf("invalid virtual file name %q: %w", name, err)
		}
		if previous, exists := canonicalNames[canonicalName]; exists {
			return fmt.Errorf("virtual file names %q and %q normalize to the same path %q", previous, name, canonicalName)
		}
		canonicalNames[canonicalName] = name
		fileData, err := ct.GetFileData(name)
		if err != nil {
			return fmt.Errorf("read file data %q for writing: %w", name, err)
		}
		entries = append(entries, fileEntry{name: canonicalName, file: file, data: fileData})
	}

	updatedFiles := make(map[string]VirtualFile, len(ct.Files))
	var offset int64 = HeaderSize
	for _, entry := range entries {
		fileSize, err := checkedVirtualFileSize(entry.name, int64(len(entry.data)))
		if err != nil {
			return err
		}
		updated := entry.file
		updated.Position = offset
		updated.Size = fileSize
		updatedFiles[entry.name] = updated
		offset += int64(len(entry.data))
	}

	rootMetadata := VirtualDirectoryMetadata{Version: ct.Version}
	dirArray, err := encodeVirtualDirectoryTree(rootMetadata, ct.Directories, updatedFiles)
	if err != nil {
		return err
	}

	msgpackData, err := msgpack.EncodeMsgpack(dirArray)
	if err != nil {
		return fmt.Errorf("msgpack encode VirtualDirectory failed: %w", err)
	}

	compressed, err := msgpack.CompressLz4BlockArray(msgpackData)
	if err != nil {
		return fmt.Errorf("compress VirtualDirectory failed: %w", err)
	}
	if uint64(len(compressed)) > uint64(^uint32(0)) {
		return fmt.Errorf("compressed VirtualDirectory is too large: %d bytes", len(compressed))
	}

	// 所有模型和封装校验都在首次写入前完成，被拒绝的元数据形状不会留下部分写入的输出流
	// All model and framing validation is complete before the first write, so rejected metadata cannot leave a partially written output stream
	if err := binaryio.WriteBytes(w, FileSignature); err != nil {
		return fmt.Errorf("write file signature failed: %w", err)
	}
	if err := binaryio.WriteByte(w, SerializeTypeMsgPack); err != nil {
		return fmt.Errorf("write serialize type failed: %w", err)
	}
	for _, entry := range entries {
		if err := binaryio.WriteBytes(w, entry.data); err != nil {
			return fmt.Errorf("write file data %q failed: %w", entry.name, err)
		}
	}

	if err := binaryio.WriteBytes(w, compressed); err != nil {
		return fmt.Errorf("write msgpack data failed: %w", err)
	}

	sizeBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(sizeBuf, uint32(len(compressed)))
	if err := binaryio.WriteBytes(w, sizeBuf); err != nil {
		return fmt.Errorf("write msgpack size failed: %w", err)
	}

	return nil
}

// NewContentTableFromDir 从磁盘目录创建 ContentTable，将目录中所有文件作为虚拟文件
// 文件路径相对于 dirPath，并使用正斜杠分隔
// NewContentTableFromDir creates a ContentTable from a disk directory and treats every file as a virtual file
// File paths are relative to dirPath and use slash separators
func NewContentTableFromDir(dirPath string) (*ContentTable, error) {
	ct := &ContentTable{
		Version: ctVersion,
		Files:   make(map[string]VirtualFile),
	}

	var rawBuf []byte
	var offset int64 = HeaderSize

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(dirPath, path)
		if err != nil {
			return fmt.Errorf("get relative path failed: %w", err)
		}
		relPath = filepath.ToSlash(relPath)

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read file %q failed: %w", relPath, err)
		}
		fileSize, err := checkedVirtualFileSize(relPath, int64(len(data)))
		if err != nil {
			return err
		}

		ct.Files[relPath] = VirtualFile{Position: offset, Size: fileSize}
		rawBuf = append(rawBuf, data...)
		offset += int64(len(data))

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk directory failed: %w", err)
	}

	ct.Raw = make([]byte, HeaderSize+len(rawBuf))
	copy(ct.Raw[:7], FileSignature)
	ct.Raw[7] = SerializeTypeMsgPack
	copy(ct.Raw[HeaderSize:], rawBuf)

	return ct, nil
}

// GetFileData 根据虚拟文件名提取原始字节数据
// 函数使用 Files 表中的 Position 和 Size 在 Raw 中切片返回
// GetFileData extracts raw bytes for a virtual file name
// It slices Raw using Position and Size from the Files table
func (ct *ContentTable) GetFileData(name string) ([]byte, error) {
	if ct == nil {
		return nil, fmt.Errorf("nil content table")
	}
	vf, ok := ct.Files[name]
	if !ok {
		return nil, fmt.Errorf("file %q not found in content table", name)
	}

	dataLen := int64(len(ct.Raw))
	if ct.dataEnd > 0 && ct.dataEnd < dataLen {
		dataLen = ct.dataEnd
	}
	if vf.Position < 0 || vf.Position > dataLen || vf.Size < 0 || int64(vf.Size) > dataLen-vf.Position {
		return nil, fmt.Errorf("file %q out of bounds: position=%d size=%d in data of %d bytes", name, vf.Position, vf.Size, len(ct.Raw))
	}
	start := vf.Position
	end := start + int64(vf.Size)
	return ct.Raw[start:end], nil
}

// GetFileNames 返回按字典序排列的所有虚拟文件名
// GetFileNames returns all virtual file names in lexicographic order
func (ct *ContentTable) GetFileNames() []string {
	if ct == nil {
		return nil
	}
	names := make([]string, 0, len(ct.Files))
	for name := range ct.Files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// GetVirtualDirectoryMetadata 返回子目录真实字段的深拷贝
// GetVirtualDirectoryMetadata returns a deep copy of the real child-directory fields
func (ct *ContentTable) GetVirtualDirectoryMetadata() map[string]VirtualDirectoryMetadata {
	if ct == nil || len(ct.Directories) == 0 {
		return nil
	}
	result := make(map[string]VirtualDirectoryMetadata, len(ct.Directories))
	for path, metadata := range ct.Directories {
		result[path] = metadata
	}
	return result
}

// AddFile 向 ContentTable 追加一个虚拟文件
// 数据会追加到 Raw 的载荷区末尾，并自动更新 Position 和 Size
// AddFile appends a virtual file to ContentTable
// Data is appended to the end of the Raw payload area and Position and Size are updated automatically
func (ct *ContentTable) AddFile(name string, data []byte) error {
	if ct == nil {
		return fmt.Errorf("nil content table")
	}
	fileSize, err := checkedVirtualFileSize(name, int64(len(data)))
	if err != nil {
		return err
	}
	if ct.Files == nil {
		ct.Files = make(map[string]VirtualFile)
	}
	// ReadContentTable 返回的表会在 Raw 中保留序列化目录与尾部，dataEnd 标记虚拟文件数据终点
	// 追加前先分离该尾部，否则新位置会落入元数据或其后并被 GetFileData 拒绝；载荷前缀不变，因此已有文件偏移仍然有效
	// A table returned by ReadContentTable retains the serialized directory and footer in Raw, with dataEnd marking the end of virtual-file data
	// The tail is detached before appending so the new position does not point into or beyond metadata; existing offsets remain valid because the payload prefix is unchanged
	if ct.dataEnd > 0 && ct.dataEnd <= int64(len(ct.Raw)) {
		ct.Raw = append([]byte(nil), ct.Raw[:ct.dataEnd]...)
		ct.dataEnd = 0
	}
	position := int64(len(ct.Raw))
	ct.Raw = append(ct.Raw, data...)
	ct.Files[name] = VirtualFile{Position: position, Size: fileSize}
	return nil
}

// checkedVirtualFileSize 将内存或文件系统长度安全窄化为游戏 VirtualFile 使用的 C# Int32。
// checkedVirtualFileSize safely narrows an in-memory or filesystem length to the C# Int32 used by the game's VirtualFile.
func checkedVirtualFileSize(name string, length int64) (int32, error) {
	if length < 0 || length > math.MaxInt32 {
		return 0, fmt.Errorf("virtual file %q size %d is outside the C# Int32 range", name, length)
	}
	return int32(length), nil
}

// DecodeMsgpackFile 提取虚拟文件并解码 MessagePack，同时自动处理 Lz4BlockArray 压缩
// 适用于读取 catalog、ExtensionNameList 等 MessagePack 序列化文件
// DecodeMsgpackFile extracts a virtual file and decodes MessagePack while handling Lz4BlockArray compression automatically
// It is suitable for MessagePack files such as catalog and ExtensionNameList
func (ct *ContentTable) DecodeMsgpackFile(name string, out interface{}) error {
	raw, err := ct.GetFileData(name)
	if err != nil {
		return err
	}

	decoded, err := msgpack.DecompressLz4BlockArray(raw)
	if err != nil {
		return fmt.Errorf("decompress content table file %q: %w", name, err)
	}

	return msgpack.DecodeMsgpack(decoded, out)
}

// decodeVirtualDirectory 解码一个固定三槽 MessagePack VirtualDirectory
// decodeVirtualDirectory decodes one fixed three-slot MessagePack VirtualDirectory
func (ct *ContentTable) decodeVirtualDirectory(data []byte) error {
	root, err := decodeRawMsgpackArray(data, "VirtualDirectory root")
	if err != nil {
		return err
	}
	if len(root) != 3 {
		return fmt.Errorf("unsupported VirtualDirectory root indexed-array width %d, expected 3", len(root))
	}
	ct.Version = 0
	ct.Directories = make(map[string]VirtualDirectoryMetadata)
	ct.Files = make(map[string]VirtualFile)
	return ct.extractDirectoryFilesRaw(root, "")
}

// extractDirectoryFilesRaw 从一个 VirtualDirectory indexed array 递归提取目录和文件
// extractDirectoryFilesRaw recursively extracts directories and files from one VirtualDirectory indexed array
func (ct *ContentTable) extractDirectoryFilesRaw(arr []codec.Raw, prefix string) error {
	if len(arr) != 3 {
		return fmt.Errorf("unsupported VirtualDirectory %q indexed-array width %d, expected 3", prefix, len(arr))
	}
	var rawVersion interface{}
	if err := decodeSingleRawMsgpackValue(arr[0], &rawVersion, fmt.Sprintf("VirtualDirectory %q version", prefix)); err != nil {
		return err
	}
	version, ok := toInt32(rawVersion)
	if !ok {
		return fmt.Errorf("VirtualDirectory %q version must be an Int32 MessagePack integer, got %T", prefix, rawVersion)
	}
	metadata := VirtualDirectoryMetadata{Version: version}
	directories, directoriesNil, err := decodeVirtualDirectoryRawMap(arr[1], fmt.Sprintf("VirtualDirectory %q allDirectorys", prefix))
	if err != nil {
		return err
	}
	if directoriesNil {
		return fmt.Errorf("VirtualDirectory %q allDirectorys must not be nil", prefix)
	}
	files, filesNil, err := decodeVirtualDirectoryRawMap(arr[2], fmt.Sprintf("VirtualDirectory %q allFiles", prefix))
	if err != nil {
		return err
	}
	if filesNil {
		return fmt.Errorf("VirtualDirectory %q allFiles must not be nil", prefix)
	}

	if prefix == "" {
		ct.Version = metadata.Version
	} else {
		if _, exists := ct.Directories[prefix]; exists {
			return fmt.Errorf("duplicate VirtualDirectory path %q", prefix)
		}
		ct.Directories[prefix] = metadata
	}

	for name, child := range directories {
		component, err := canonicalVirtualComponent(name)
		if err != nil {
			return fmt.Errorf("VirtualDirectory %q child %q: %w", prefix, name, err)
		}
		childPrefix := component
		if prefix != "" {
			childPrefix = prefix + "/" + component
		}
		childArray, err := decodeRawMsgpackArray(child, fmt.Sprintf("VirtualDirectory %q", childPrefix))
		if err != nil {
			return err
		}
		if err := ct.extractDirectoryFilesRaw(childArray, childPrefix); err != nil {
			return err
		}
	}

	for name, rawFile := range files {
		component, err := canonicalVirtualComponent(name)
		if err != nil {
			return fmt.Errorf("VirtualDirectory %q file %q: %w", prefix, name, err)
		}
		fullName := component
		if prefix != "" {
			fullName = prefix + "/" + component
		}
		file, err := decodeVirtualFileRaw(rawFile)
		if err != nil {
			return fmt.Errorf("VirtualDirectory file %q: %w", fullName, err)
		}
		if _, exists := ct.Files[fullName]; exists {
			return fmt.Errorf("duplicate VirtualDirectory file path %q", fullName)
		}
		ct.Files[fullName] = file
	}
	return nil
}

// decodeRawMsgpackArray 将单个 MessagePack 数组解码为保留原始编码的槽位
// decodeRawMsgpackArray decodes one MessagePack array into slots that retain their raw encoding
func decodeRawMsgpackArray(data []byte, label string) ([]codec.Raw, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("%s: empty MessagePack value", label)
	}
	pos := int64(0)
	count, err := msgpack.ReadArrayHeaderStrict(data, &pos)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	if count > int64(len(data))-pos {
		return nil, fmt.Errorf("%s array length %d exceeds the capacity of %d remaining bytes", label, count, int64(len(data))-pos)
	}
	var values []codec.Raw
	if err := decodeSingleRawMsgpackValue(data, &values, label); err != nil {
		return nil, err
	}
	if int64(len(values)) != count {
		return nil, fmt.Errorf("%s decoded %d fields, header declares %d", label, len(values), count)
	}
	return values, nil
}

// decodeVirtualDirectoryRawMap 解码 allDirectorys 或 allFiles 映射，并单独报告其是否为 nil
// decodeVirtualDirectoryRawMap decodes an allDirectorys or allFiles map and separately reports whether it was nil
func decodeVirtualDirectoryRawMap(data []byte, label string) (map[string]codec.Raw, bool, error) {
	// ugorji 将 codec.Raw 中解码得到的 MessagePack nil 表示为 nil 切片，而非单字节 0xc0
	// 调用方会另行跟踪槽位是否存在，因此两种形式在此都明确表示一个存在但值为 nil 的映射
	// ugorji represents a decoded MessagePack nil held in codec.Raw as a nil slice rather than the one-byte 0xc0 span
	// The caller tracks slot presence separately, so both forms unambiguously mean a present nil map here
	if len(data) == 0 || (len(data) == 1 && data[0] == 0xc0) {
		return nil, true, nil
	}
	position := int64(0)
	count, err := msgpack.ReadMapHeader(data, &position)
	if err != nil {
		return nil, false, fmt.Errorf("%s: %w", label, err)
	}
	headerSize := position
	if count > (int64(len(data))-headerSize)/2 {
		return nil, false, fmt.Errorf("%s map length %d exceeds the capacity of %d remaining bytes", label, count, int64(len(data))-headerSize)
	}
	var generic map[interface{}]codec.Raw
	if err := decodeSingleRawMsgpackValue(data, &generic, label); err != nil {
		return nil, false, err
	}
	if int64(len(generic)) != count {
		return nil, false, fmt.Errorf("%s contains duplicate string keys that the Go map model cannot preserve", label)
	}
	values := make(map[string]codec.Raw, len(generic))
	for key, value := range generic {
		stringKey, ok := key.(string)
		if !ok {
			return nil, false, fmt.Errorf("%s map key must be string, got %T", label, key)
		}
		values[stringKey] = value
	}
	return values, false, nil
}

// decodeSingleRawMsgpackValue 解码恰好一个原始 MessagePack 值，并拒绝尾随字节
// decodeSingleRawMsgpackValue decodes exactly one raw MessagePack value and rejects trailing bytes
func decodeSingleRawMsgpackValue(data []byte, out interface{}, label string) error {
	if len(data) == 0 {
		data = []byte{0xc0}
	}
	if err := msgpack.DecodeMsgpack(data, out); err != nil {
		return fmt.Errorf("%s MessagePack decode failed: %w", label, err)
	}
	return nil
}

// decodeVirtualFileRaw 解码固定两槽 VirtualFile indexed array
// decodeVirtualFileRaw decodes the fixed two-slot VirtualFile indexed array
func decodeVirtualFileRaw(data []byte) (VirtualFile, error) {
	fields, err := decodeRawMsgpackArray(data, "VirtualFile")
	if err != nil {
		return VirtualFile{}, err
	}
	if len(fields) != 2 {
		return VirtualFile{}, fmt.Errorf("unsupported VirtualFile indexed-array width %d, expected 2", len(fields))
	}
	file := VirtualFile{}
	var position interface{}
	if err := decodeSingleRawMsgpackValue(fields[0], &position, "VirtualFile.position"); err != nil {
		return VirtualFile{}, err
	}
	var ok bool
	file.Position, ok = toInt64(position)
	if !ok {
		return VirtualFile{}, fmt.Errorf("position/size: position must be an Int64-compatible MessagePack integer, got %T", position)
	}
	var size interface{}
	if err := decodeSingleRawMsgpackValue(fields[1], &size, "VirtualFile.size"); err != nil {
		return VirtualFile{}, err
	}
	file.Size, ok = toInt32(size)
	if !ok {
		if isIntegerValue(size) {
			return VirtualFile{}, fmt.Errorf("position/size: size is outside the Int32 range")
		}
		return VirtualFile{}, fmt.Errorf("position/size: size must be an Int32 MessagePack integer, got %T", size)
	}
	return file, nil
}

// decodeVirtualFile 将 MessagePack 解码后的 indexed array [position, size] 转为 VirtualFile
// decodeVirtualFile converts a decoded MessagePack indexed array [position, size] to VirtualFile
func decodeVirtualFile(val interface{}) (VirtualFile, error) {
	arr, ok := val.([]interface{})
	if !ok || len(arr) != 2 {
		return VirtualFile{}, fmt.Errorf("VirtualFile: expected array(2), got %T", val)
	}

	pos, ok1 := toInt64(arr[0])
	size, ok2 := toInt32(arr[1])
	if !ok1 {
		return VirtualFile{}, fmt.Errorf("VirtualFile position/size: position expected Int64 integer, got %T", arr[0])
	}
	if !ok2 {
		if isIntegerValue(arr[1]) {
			return VirtualFile{}, fmt.Errorf("VirtualFile position/size: size is outside the Int32 range")
		}
		return VirtualFile{}, fmt.Errorf("VirtualFile position/size: size expected Int32 integer, got %T", arr[1])
	}

	return VirtualFile{Position: pos, Size: size}, nil
}

// virtualDirNode 是将扁平路径重建为 VirtualDirectory 层级时使用的内部节点
// virtualDirNode is an internal node used to rebuild the VirtualDirectory hierarchy from flattened paths
type virtualDirNode struct {
	dirs     map[string]*virtualDirNode // 按当前层名称索引的子目录 / Child directories keyed by their names at this level
	files    map[string]VirtualFile     // 当前目录中的虚拟文件 / Virtual files in the current directory
	metadata *VirtualDirectoryMetadata  // 当前目录显式保留的线格式元数据 / Explicit wire metadata retained for the current directory
}

// encodeVirtualDirectoryTree 从扁平目录元数据和文件路径构建可编码的 VirtualDirectory 树
// encodeVirtualDirectoryTree builds an encodable VirtualDirectory tree from flattened directory metadata and file paths
func encodeVirtualDirectoryTree(rootMetadata VirtualDirectoryMetadata, metadata map[string]VirtualDirectoryMetadata, files map[string]VirtualFile) ([]interface{}, error) {
	root := &virtualDirNode{
		dirs:     map[string]*virtualDirNode{},
		files:    map[string]VirtualFile{},
		metadata: &rootMetadata,
	}
	canonicalMetadata := make(map[string]string, len(metadata))
	for path, value := range metadata {
		canonicalPath, err := canonicalVirtualPath(path)
		if err != nil {
			return nil, fmt.Errorf("invalid VirtualDirectory metadata path %q: %w", path, err)
		}
		if previous, exists := canonicalMetadata[canonicalPath]; exists {
			return nil, fmt.Errorf("VirtualDirectory metadata paths %q and %q normalize to %q", previous, path, canonicalPath)
		}
		canonicalMetadata[canonicalPath] = path
		node := ensureVirtualDirNode(root, splitVirtualPath(canonicalPath))
		copyValue := value
		node.metadata = &copyValue
	}
	for name, vf := range files {
		parts := splitVirtualPath(name)
		if len(parts) == 0 {
			return nil, fmt.Errorf("virtual file path %q has no components", name)
		}
		node := ensureVirtualDirNode(root, parts[:len(parts)-1])
		node.files[parts[len(parts)-1]] = vf
	}
	encoded, err := encodeVirtualDirNode(root, rootMetadata, "")
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

// ensureVirtualDirNode 确保路径各层节点存在并返回末级目录
// ensureVirtualDirNode ensures every node on a path exists and returns the final directory
func ensureVirtualDirNode(root *virtualDirNode, parts []string) *virtualDirNode {
	node := root
	for _, part := range parts {
		child := node.dirs[part]
		if child == nil {
			child = &virtualDirNode{dirs: map[string]*virtualDirNode{}, files: map[string]VirtualFile{}}
			node.dirs[part] = child
		}
		node = child
	}
	return node
}

// encodeVirtualDirNode 按游戏当前固定布局递归编码一个 VirtualDirectory 节点
// encodeVirtualDirNode recursively encodes a VirtualDirectory node using the game's current fixed layout
func encodeVirtualDirNode(node *virtualDirNode, inherited VirtualDirectoryMetadata, path string) ([]interface{}, error) {
	metadata := inherited
	if node.metadata != nil {
		metadata = *node.metadata
	}
	label := path
	if label == "" {
		label = "<root>"
	}
	dirs := make(map[string]interface{}, len(node.dirs))
	for name, child := range node.dirs {
		childPath := name
		if path != "" {
			childPath = path + "/" + name
		}
		childDefault := VirtualDirectoryMetadata{Version: metadata.Version}
		encoded, err := encodeVirtualDirNode(child, childDefault, childPath)
		if err != nil {
			return nil, err
		}
		dirs[name] = encoded
	}
	files, err := encodeVirtualFilesMap(node.files, label)
	if err != nil {
		return nil, err
	}

	return []interface{}{metadata.Version, dirs, files}, nil
}

// encodeVirtualFilesMap 编码一个 VirtualDirectory 的 allFiles 映射
// encodeVirtualFilesMap encodes the allFiles map of one VirtualDirectory
func encodeVirtualFilesMap(files map[string]VirtualFile, directoryLabel string) (map[string]interface{}, error) {
	result := make(map[string]interface{}, len(files))
	for name, file := range files {
		encoded, err := encodeVirtualFile(file, fmt.Sprintf("VirtualDirectory %q file %q", directoryLabel, name))
		if err != nil {
			return nil, err
		}
		result[name] = encoded
	}
	return result, nil
}

// encodeVirtualFile 按固定两槽布局编码 VirtualFile
// encodeVirtualFile encodes a VirtualFile using the fixed two-slot layout
func encodeVirtualFile(file VirtualFile, label string) ([]interface{}, error) {
	if file.Position < 0 {
		return nil, fmt.Errorf("%s position must be non-negative", label)
	}
	if file.Size < 0 {
		return nil, fmt.Errorf("%s size must be non-negative", label)
	}
	return []interface{}{file.Position, file.Size}, nil
}

// splitVirtualPath 将虚拟路径拆分为非空且非点号的组成部分
// splitVirtualPath splits a virtual path into non-empty, non-dot components
func splitVirtualPath(name string) []string {
	cleaned := strings.ReplaceAll(filepath.ToSlash(name), "\\", "/")
	raw := strings.Split(cleaned, "/")
	parts := raw[:0]
	for _, part := range raw {
		if part != "" && part != "." {
			parts = append(parts, part)
		}
	}
	return parts
}

// joinVirtualPath 连接并规范化两个供内部使用的虚拟路径片段
// joinVirtualPath joins and normalizes two virtual-path fragments for internal use
func joinVirtualPath(prefix string, name string) string {
	parts := append(splitVirtualPath(prefix), splitVirtualPath(name)...)
	return strings.Join(parts, "/")
}

// joinVirtualPathChecked 校验并连接两个可序列化的虚拟路径
// joinVirtualPathChecked validates and joins two serializable virtual paths
func joinVirtualPathChecked(prefix string, name string) (string, error) {
	canonicalName, err := canonicalVirtualPath(name)
	if err != nil {
		return "", err
	}
	if prefix == "" {
		return canonicalName, nil
	}
	canonicalPrefix, err := canonicalVirtualPath(prefix)
	if err != nil {
		return "", err
	}
	return canonicalPrefix + "/" + canonicalName, nil
}

// canonicalVirtualComponent 校验 allDirectorys 或 allFiles 中的一个字典键
// 键内分隔符无法由扁平 ContentTable 路径模型表示且会在重新编码时改变目录层级，因此会作为不可表示的线格式形状被拒绝而不是规范化
// canonicalVirtualComponent validates one dictionary key from allDirectorys or allFiles
// A separator inside a key cannot be represented by the flattened ContentTable path model without changing directory depth on re-encode, so it is rejected as an unrepresentable wire shape instead of normalized
func canonicalVirtualComponent(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("name is empty")
	}
	if strings.IndexByte(name, 0) >= 0 {
		return "", fmt.Errorf("name contains NUL")
	}
	if strings.ContainsAny(name, `/\\`) {
		return "", fmt.Errorf("name contains a path separator")
	}
	if name == "." || name == ".." {
		return "", fmt.Errorf("unsafe name %q", name)
	}
	return name, nil
}

// canonicalVirtualPath 校验虚拟路径并统一使用正斜杠分隔
// canonicalVirtualPath validates a virtual path and normalizes its separators to slashes
func canonicalVirtualPath(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("path is empty")
	}
	if strings.IndexByte(name, 0) >= 0 {
		return "", fmt.Errorf("path contains NUL")
	}
	if filepath.IsAbs(name) || filepath.VolumeName(name) != "" {
		return "", fmt.Errorf("absolute or volume-qualified path is not allowed")
	}
	normalized := strings.ReplaceAll(name, "\\", "/")
	if strings.HasPrefix(normalized, "/") {
		return "", fmt.Errorf("absolute path is not allowed")
	}
	parts := strings.Split(normalized, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("path contains unsafe segment %q", part)
		}
	}
	return strings.Join(parts, "/"), nil
}

// strictStringMap 将通用 MessagePack 映射转换为仅允许字符串键的映射
// strictStringMap converts a generic MessagePack map to a map that permits only string keys
func strictStringMap(v interface{}) (map[string]interface{}, error) {
	switch m := v.(type) {
	case map[string]interface{}:
		return m, nil
	case map[interface{}]interface{}:
		result := make(map[string]interface{}, len(m))
		for k, val := range m {
			ks, ok := k.(string)
			if !ok {
				return nil, fmt.Errorf("map key must be string, got %T", k)
			}
			result[ks] = val
		}
		return result, nil
	}
	return nil, fmt.Errorf("expected map, got %T", v)
}

const (
	csharpInt32Min int64 = -1 << 31
	csharpInt32Max int64 = 1<<31 - 1
)

// checkedInt32Count 将内部 Int64 集合长度安全窄化为 C# 集合 API 使用的 Int32。
// checkedInt32Count safely narrows an internal Int64 collection length to the Int32 used by C# collection APIs.
func checkedInt32Count(count int64, label string) (int32, error) {
	if count < 0 || count > csharpInt32Max {
		return 0, fmt.Errorf("%s %d is outside the C# Int32 range [0,%d]", label, count, csharpInt32Max)
	}
	return int32(count), nil
}

// toInt32 将 MessagePack 整数转换为 C# int 使用的 wire 宽度，并始终执行 Int32 范围校验。
// toInt32 converts a MessagePack integer to the wire width used by C# int and always enforces the Int32 range.
func toInt32(v interface{}) (int32, bool) {
	switch n := v.(type) {
	case int64:
		if n < csharpInt32Min || n > csharpInt32Max {
			return 0, false
		}
		return int32(n), true
	case uint64:
		if n > uint64(csharpInt32Max) {
			return 0, false
		}
		return int32(n), true
	}
	return 0, false
}

// isIntegerValue 判断解码值是否属于支持的 MessagePack 整数表示
// isIntegerValue reports whether a decoded value uses one of the supported MessagePack integer representations
func isIntegerValue(value interface{}) bool {
	switch value.(type) {
	case int64, uint64:
		return true
	default:
		return false
	}
}

// toInt64 将支持的 MessagePack 整数转换为有符号 Int64
// toInt64 converts a supported MessagePack integer to signed Int64
func toInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case uint64:
		if n > uint64(^uint64(0)>>1) {
			return 0, false
		}
		return int64(n), true
	}
	return 0, false
}

// lenOf 返回通用数组的长度，非数组值返回 -1
// lenOf returns the length of a generic array or -1 for a non-array value
func lenOf(v interface{}) int64 {
	if arr, ok := v.([]interface{}); ok {
		return int64(len(arr))
	}
	return -1
}
