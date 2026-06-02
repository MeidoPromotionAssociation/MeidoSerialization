package ct

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ugorji/go/codec"
)

// .ct 文件是 KCES 游戏的资源目录文件 Content Table，实际格式为 VirtualDirectory 序列化格式 / .ct files are KCES Content Table resource catalogs using the serialized VirtualDirectory format
//
//	[7 bytes]  FileSignature: bb c3 aa 9a a6 4d ad
//	[1 byte]   SerializeType: 8e = MessagePack, 00 = MemoryPack
//	[N bytes]  Raw file data（各 VirtualFile 的原始数据连续存放）
//	[M bytes]  MessagePack+Lz4BlockArray 压缩的 VirtualDirectory 结构
//	[4 bytes]  M 的值（little-endian int32，指示 MessagePack 部分的长度）

// FileSignature 是 .ct 文件的魔数签名（7 字节），用于验证文件格式 / FileSignature is the 7-byte magic signature used to validate .ct files
// 对应 C# VirtualDirectory.FileSignature = {0xbb, 0xc3, 0xaa, 0x9a, 0xa6, 0x4d, 0xad}
var FileSignature = []byte{0xbb, 0xc3, 0xaa, 0x9a, 0xa6, 0x4d, 0xad}

const (
	// HeaderSize 是文件头大小：7 字节签名 + 1 字节序列化类型
	HeaderSize = 8
	// SerializeTypeMsgPack 表示使用 MessagePack 序列化（0x8e）
	// 另一种可能值 0x00 表示 MemoryPack，本库不支持
	SerializeTypeMsgPack = 0x8e
	// footerSizeLen 是文件末尾存储 msgpack 数据长度的字节数（little-endian uint32）
	footerSizeLen = 4
	// ctVersion 是 VirtualDirectory 的固定版本号，对应 C# VirtualDirectory.FixVersion = 1000
	ctVersion = 1000
)

// ContentTable 表示解析后的 .ct 文件 VirtualDirectory 序列化结构 / ContentTable represents a parsed .ct file in serialized VirtualDirectory form
// .ct 文件是 KCES 游戏的资源目录容器，内部存储 catalog、ExtensionNameList 等虚拟文件 / A .ct file is a KCES resource catalog container storing virtual files such as catalog and ExtensionNameList
// 游戏通过 CatalogUtility.FromCatalog<T> 读取 .ct 中的 "catalog" 文件获取资源索引 / The game reads the "catalog" virtual file through CatalogUtility.FromCatalog<T> to obtain resource indexes
type ContentTable struct {
	Version int                    `json:"Version"` // VirtualDirectory 版本号，固定为 1000 / VirtualDirectory version, fixed to 1000
	Files   map[string]VirtualFile `json:"Files"`   // 虚拟文件表，key 为文件名，value 为位置和大小 / Virtual file table keyed by file name with position and size values
	Raw     []byte                 `json:"-"`       // 完整文件原始字节，用于按偏移提取虚拟文件内容 / Raw bytes of the full file used to slice virtual file contents by offset
}

// VirtualFile 表示虚拟文件系统中的一个文件条目 / VirtualFile represents one file entry in the virtual file system
// 对应 C# VirtualFile 的 MessagePack indexed array: [Key(0)=position, Key(1)=size] / Matches the C# VirtualFile MessagePack indexed array [Key(0)=position, Key(1)=size]
type VirtualFile struct {
	Position int64 `json:"Position"` // 文件数据在 .ct 文件中的绝对字节偏移，从文件开头计算且包含 header / Absolute byte offset of file data inside the .ct file, counted from file start including header
	Size     int   `json:"Size"`     // 文件数据的字节大小 / File data size in bytes
}

// ReadContentTable 从 reader 中读取并解析 .ct 文件。
// 解析流程：验证签名 → 读取末尾长度 → 提取 MessagePack 数据 → LZ4 解压 → 解码 VirtualDirectory。
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

	msgpackSize := int(binary.LittleEndian.Uint32(data[len(data)-footerSizeLen:]))
	if msgpackSize <= 0 || msgpackSize > len(data)-HeaderSize-footerSizeLen {
		return nil, fmt.Errorf("invalid msgpack size: %d (file size: %d)", msgpackSize, len(data))
	}

	msgpackStart := len(data) - footerSizeLen - msgpackSize
	msgpackData := data[msgpackStart : len(data)-footerSizeLen]

	decompressed, err := DecompressLz4BlockArray(msgpackData)
	if err != nil {
		return nil, fmt.Errorf("decompress VirtualDirectory failed: %w", err)
	}

	ct := &ContentTable{Raw: data}
	if err := ct.decodeVirtualDirectory(decompressed); err != nil {
		return nil, fmt.Errorf("decode VirtualDirectory failed: %w", err)
	}

	return ct, nil
}

// WriteContentTable 将 ContentTable 序列化为 .ct 格式并写入 writer。
// 写入流程：签名 → 序列化类型 → 各文件原始数据 → LZ4 压缩的 VirtualDirectory → 长度尾部。
func WriteContentTable(w io.Writer, ct *ContentTable) error {
	if _, err := w.Write(FileSignature); err != nil {
		return fmt.Errorf("write file signature failed: %w", err)
	}
	if _, err := w.Write([]byte{SerializeTypeMsgPack}); err != nil {
		return fmt.Errorf("write serialize type failed: %w", err)
	}

	updatedFiles := make(map[string]VirtualFile, len(ct.Files))
	var offset int64 = HeaderSize

	type fileEntry struct {
		name string // 虚拟文件名 / Virtual file name
		data []byte // 虚拟文件数据 / Virtual file data
	}
	var entries []fileEntry
	for name, vf := range ct.Files {
		var fileData []byte
		if ct.Raw != nil {
			start := int(vf.Position)
			end := start + vf.Size
			if start >= 0 && end <= len(ct.Raw) {
				fileData = ct.Raw[start:end]
			}
		}
		if fileData == nil {
			fileData = []byte{}
		}
		entries = append(entries, fileEntry{name: name, data: fileData})
	}

	for _, entry := range entries {
		if _, err := w.Write(entry.data); err != nil {
			return fmt.Errorf("write file data %q failed: %w", entry.name, err)
		}
		updatedFiles[entry.name] = VirtualFile{Position: offset, Size: len(entry.data)}
		offset += int64(len(entry.data))
	}

	version := ct.Version
	if version == 0 {
		version = ctVersion
	}
	dirArray := []interface{}{
		int64(version),
		map[string]interface{}{},
		encodeFilesMap(updatedFiles),
	}

	h := &codec.MsgpackHandle{}
	var msgpackData []byte
	enc := codec.NewEncoderBytes(&msgpackData, h)
	if err := enc.Encode(dirArray); err != nil {
		return fmt.Errorf("msgpack encode VirtualDirectory failed: %w", err)
	}

	compressed, err := CompressLz4BlockArray(msgpackData)
	if err != nil {
		return fmt.Errorf("compress VirtualDirectory failed: %w", err)
	}

	if _, err := w.Write(compressed); err != nil {
		return fmt.Errorf("write msgpack data failed: %w", err)
	}

	sizeBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(sizeBuf, uint32(len(compressed)))
	if _, err := w.Write(sizeBuf); err != nil {
		return fmt.Errorf("write msgpack size failed: %w", err)
	}

	return nil
}

// NewContentTableFromDir 从磁盘目录创建 ContentTable，将目录中所有文件作为虚拟文件。
// 文件路径使用正斜杠分隔，相对于 dirPath。
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

		ct.Files[relPath] = VirtualFile{Position: offset, Size: len(data)}
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

// GetFileData 根据虚拟文件名提取原始字节数据。
// 通过 Files 表中的 Position/Size 在 Raw 中切片返回。
func (ct *ContentTable) GetFileData(name string) ([]byte, error) {
	vf, ok := ct.Files[name]
	if !ok {
		return nil, fmt.Errorf("file %q not found in content table", name)
	}

	start := int(vf.Position)
	end := start + vf.Size
	if start < 0 || end > len(ct.Raw) {
		return nil, fmt.Errorf("file %q out of bounds: [%d, %d) in data of %d bytes", name, start, end, len(ct.Raw))
	}

	return ct.Raw[start:end], nil
}

// GetFileNames 返回所有虚拟文件名（无序）
func (ct *ContentTable) GetFileNames() []string {
	names := make([]string, 0, len(ct.Files))
	for name := range ct.Files {
		names = append(names, name)
	}
	return names
}

// AddFile 向 ContentTable 追加一个虚拟文件。
// 数据会追加到 Raw 末尾，自动更新 Position 和 Size。
func (ct *ContentTable) AddFile(name string, data []byte) {
	if ct.Files == nil {
		ct.Files = make(map[string]VirtualFile)
	}
	position := int64(len(ct.Raw))
	ct.Raw = append(ct.Raw, data...)
	ct.Files[name] = VirtualFile{Position: position, Size: len(data)}
}

// DecodeMsgpackFile 提取虚拟文件并解码 MessagePack（自动处理 Lz4BlockArray 压缩）。
// 适用于读取 catalog、ExtensionNameList 等 MessagePack 序列化的文件。
func (ct *ContentTable) DecodeMsgpackFile(name string, out interface{}) error {
	raw, err := ct.GetFileData(name)
	if err != nil {
		return err
	}

	decoded, err := DecompressLz4BlockArray(raw)
	if err != nil {
		decoded = raw
	}

	return DecodeMsgpack(decoded, out)
}

// decodeVirtualDirectory 解码 MessagePack 格式的 VirtualDirectory。
// 标准结构为 indexed array [version, allDirectorys, allFiles]，
// 部分文件省略空 allDirectorys 字段，呈现为 [version, allFiles] 或 [allDirectorys, allFiles]。
func (ct *ContentTable) decodeVirtualDirectory(data []byte) error {
	h := &codec.MsgpackHandle{}
	h.RawToString = true

	var raw interface{}
	dec := codec.NewDecoderBytes(data, h)
	if err := dec.Decode(&raw); err != nil {
		return fmt.Errorf("msgpack decode failed: %w", err)
	}

	arr, ok := raw.([]interface{})
	if !ok || len(arr) < 2 {
		return fmt.Errorf("expected array(2+), got %T len=%d", raw, lenOf(raw))
	}

	ct.Files = make(map[string]VirtualFile)

	switch len(arr) {
	case 3:
		if v, ok := toInt(arr[0]); ok {
			ct.Version = v
		}
		ct.extractFiles(arr[2])
	case 2:
		if v, ok := toInt(arr[0]); ok && v >= 100 && v <= 10000 {
			ct.Version = v
			ct.extractFiles(arr[1])
		} else {
			ct.Version = ctVersion
			ct.extractFiles(arr[1])
		}
	default:
		if v, ok := toInt(arr[0]); ok {
			ct.Version = v
		}
		ct.extractFiles(arr[len(arr)-1])
	}

	if len(ct.Files) == 0 {
		for _, elem := range arr {
			ct.extractFiles(elem)
			if len(ct.Files) > 0 {
				break
			}
		}
	}

	return nil
}

// extractFiles 从 MessagePack map 值中提取 VirtualFile 条目并填充到 ct.Files
func (ct *ContentTable) extractFiles(val interface{}) {
	filesMap := toStringMap(val)
	for name, v := range filesMap {
		vf, err := decodeVirtualFile(v)
		if err == nil {
			ct.Files[name] = vf
		}
	}
}

// decodeVirtualFile 将 MessagePack 解码后的 indexed array [position, size] 转为 VirtualFile
func decodeVirtualFile(val interface{}) (VirtualFile, error) {
	arr, ok := val.([]interface{})
	if !ok || len(arr) < 2 {
		return VirtualFile{}, fmt.Errorf("VirtualFile: expected array(2+), got %T", val)
	}

	pos, ok1 := toInt64(arr[0])
	size, ok2 := toInt(arr[1])
	if !ok1 || !ok2 {
		return VirtualFile{}, fmt.Errorf("VirtualFile: invalid position/size types")
	}

	return VirtualFile{Position: pos, Size: size}, nil
}

func encodeFilesMap(files map[string]VirtualFile) map[string]interface{} {
	result := make(map[string]interface{}, len(files))
	for name, vf := range files {
		result[name] = []interface{}{vf.Position, int64(vf.Size)}
	}
	return result
}

func toStringMap(v interface{}) map[string]interface{} {
	switch m := v.(type) {
	case map[string]interface{}:
		return m
	case map[interface{}]interface{}:
		result := make(map[string]interface{}, len(m))
		for k, val := range m {
			if ks, ok := k.(string); ok {
				result[ks] = val
			}
		}
		return result
	}
	return nil
}

func toInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int64:
		return int(n), true
	case uint64:
		return int(n), true
	case int:
		return n, true
	case uint:
		return int(n), true
	}
	return 0, false
}

func toInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case uint64:
		return int64(n), true
	case int:
		return int64(n), true
	case uint:
		return int64(n), true
	}
	return 0, false
}

func lenOf(v interface{}) int {
	if arr, ok := v.([]interface{}); ok {
		return len(arr)
	}
	return -1
}
