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

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio"
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
	Version        int                                 `json:"Version"`                  // Stored root VirtualDirectory version; zero is preserved / 保存的根 VirtualDirectory 版本；零值原样保留
	Versionless    bool                                `json:"Versionless,omitempty"`    // Historical root [directories, files] form with no version slot / 历史根无版本 [目录, 文件] 形式
	FilesOnly      bool                                `json:"FilesOnly,omitempty"`      // Historical root [version, files] form / 历史根 [版本, 文件] 形式
	DirectoriesNil bool                                `json:"DirectoriesNil,omitempty"` // Root allDirectorys was MessagePack nil / 根 allDirectorys 为 MessagePack nil
	FilesNil       bool                                `json:"FilesNil,omitempty"`       // Root allFiles was MessagePack nil / 根 allFiles 为 MessagePack nil
	FieldCount     *int                                `json:"FieldCount,omitempty"`     // Non-canonical root indexed-array width / 非标准根 indexed-array 槽数
	FutureSlots    [][]byte                            `json:"FutureSlots,omitempty"`    // Verbatim root MessagePack values after known slots / 根已知槽后的原始 MessagePack 值
	Directories    map[string]VirtualDirectoryMetadata `json:"Directories,omitempty"`    // Metadata for every child directory path, including empty directories / 每个子目录路径的元数据（含空目录）
	Files          map[string]VirtualFile              `json:"Files"`                    // 虚拟文件表，key 为文件名，value 为位置和大小 / Virtual file table keyed by file name with position and size values
	Raw            []byte                              `json:"-"`                        // 完整文件原始字节，用于按偏移提取虚拟文件内容 / Raw bytes of the full file used to slice virtual file contents by offset
	dataEnd        int64                               // 实际虚拟文件数据区末尾；零值表示使用 len(Raw) / End of the virtual-file payload area; zero means len(Raw)
}

// VirtualDirectoryMetadata preserves the indexed-object shape of one child
// VirtualDirectory. Paths are stored as canonical slash-separated keys in
// ContentTable.Directories. A missing entry denotes a newly-created directory
// and inherits the root layout; every decoded directory receives an entry.
type VirtualDirectoryMetadata struct {
	Version        int      `json:"Version"`
	Versionless    bool     `json:"Versionless,omitempty"`
	FilesOnly      bool     `json:"FilesOnly,omitempty"`
	DirectoriesNil bool     `json:"DirectoriesNil,omitempty"`
	FilesNil       bool     `json:"FilesNil,omitempty"`
	FieldCount     *int     `json:"FieldCount,omitempty"`
	FutureSlots    [][]byte `json:"FutureSlots,omitempty"`
}

// VirtualFile 表示虚拟文件系统中的一个文件条目 / VirtualFile represents one file entry in the virtual file system
// 对应 C# VirtualFile 的 MessagePack indexed array: [Key(0)=position, Key(1)=size] / Matches the C# VirtualFile MessagePack indexed array [Key(0)=position, Key(1)=size]
type VirtualFile struct {
	Position    int64    `json:"Position"`              // 文件数据在 .ct 文件中的绝对字节偏移，从文件开头计算且包含 header / Absolute byte offset of file data inside the .ct file, counted from file start including header
	Size        int      `json:"Size"`                  // 文件数据的字节大小 / File data size in bytes
	FieldCount  *int     `json:"FieldCount,omitempty"`  // Non-canonical indexed-array width / 非标准 indexed-array 槽数
	FutureSlots [][]byte `json:"FutureSlots,omitempty"` // Verbatim MessagePack values after position and size / position、size 后的原始 MessagePack 值
}

// VirtualFileMetadata is the position-independent portion of a VirtualFile's
// indexed-object shape. Semantic wrappers use it when they rebuild payload
// offsets but still need to retain short arrays and future slots.
type VirtualFileMetadata struct {
	FieldCount  *int     `json:"FieldCount,omitempty"`
	FutureSlots [][]byte `json:"FutureSlots,omitempty"`
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

	ct := &ContentTable{Raw: data, dataEnd: int64(msgpackStart)}
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

// WriteContentTable 将 ContentTable 序列化为 .ct 格式并写入 writer。
// 写入流程：签名 → 序列化类型 → 各文件原始数据 → LZ4 压缩的 VirtualDirectory → 长度尾部。
func WriteContentTable(w io.Writer, ct *ContentTable) error {
	if w == nil {
		return fmt.Errorf("nil content table writer")
	}
	if ct == nil {
		return fmt.Errorf("nil content table")
	}
	type fileEntry struct {
		name string      // 虚拟文件名 / Virtual file name
		file VirtualFile // VirtualFile wire-shape metadata / VirtualFile wire 形态元数据
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
		if err := validateInt32Field(file.Size, fmt.Sprintf("virtual file %q size", name)); err != nil {
			return err
		}
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
		updated := entry.file
		updated.Position = offset
		updated.Size = len(entry.data)
		// A zero-width historical VirtualFile has no position slot. Empty data
		// does not need relocating, so retain its decoded zero value instead of
		// manufacturing a position that the preserved shape cannot encode.
		if entry.file.FieldCount != nil && *entry.file.FieldCount < 1 && len(entry.data) == 0 {
			updated.Position = entry.file.Position
		}
		updatedFiles[entry.name] = updated
		offset += int64(len(entry.data))
	}

	rootMetadata := VirtualDirectoryMetadata{
		Version:        ct.Version,
		Versionless:    ct.Versionless,
		FilesOnly:      ct.FilesOnly,
		DirectoriesNil: ct.DirectoriesNil,
		FilesNil:       ct.FilesNil,
		FieldCount:     ct.FieldCount,
		FutureSlots:    ct.FutureSlots,
	}
	dirArray, err := encodeVirtualDirectoryTree(rootMetadata, ct.Directories, updatedFiles)
	if err != nil {
		return err
	}

	h := newMsgpackEncoderHandle(true)
	var msgpackData []byte
	enc := codec.NewEncoderBytes(&msgpackData, h)
	if err := enc.Encode(dirArray); err != nil {
		return fmt.Errorf("msgpack encode VirtualDirectory failed: %w", err)
	}

	compressed, err := CompressLz4BlockArray(msgpackData)
	if err != nil {
		return fmt.Errorf("compress VirtualDirectory failed: %w", err)
	}
	if uint64(len(compressed)) > uint64(^uint32(0)) {
		return fmt.Errorf("compressed VirtualDirectory is too large: %d bytes", len(compressed))
	}

	// All model and framing validation is complete before the first write, so a
	// rejected metadata shape cannot leave a partially-written output stream.
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
	start := int(vf.Position)
	end := start + vf.Size
	return ct.Raw[start:end], nil
}

// GetFileNames 返回按字典序排列的所有虚拟文件名。
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

// GetVirtualDirectoryMetadata returns the minimal metadata needed to recreate
// the decoded directory tree. Canonical non-empty directories are derivable
// from flattened file/child paths and are omitted; empty directories and any
// node whose version/layout differs from its parent remain explicit.
func (ct *ContentTable) GetVirtualDirectoryMetadata() map[string]VirtualDirectoryMetadata {
	if ct == nil || len(ct.Directories) == 0 {
		return nil
	}
	root := VirtualDirectoryMetadata{Version: ct.Version, Versionless: ct.Versionless}
	result := make(map[string]VirtualDirectoryMetadata)
	for path, metadata := range ct.Directories {
		parent := root
		if separator := strings.LastIndexByte(path, '/'); separator >= 0 {
			parentPath := path[:separator]
			if parentMetadata, ok := ct.Directories[parentPath]; ok {
				parent = parentMetadata
			}
		}
		expected := VirtualDirectoryMetadata{Version: parent.Version, Versionless: parent.Versionless}
		hasContent := false
		prefix := path + "/"
		for fileName := range ct.Files {
			if strings.HasPrefix(fileName, prefix) {
				hasContent = true
				break
			}
		}
		if !hasContent {
			for childPath := range ct.Directories {
				if childPath != path && strings.HasPrefix(childPath, prefix) {
					hasContent = true
					break
				}
			}
		}
		if hasContent && virtualDirectoryMetadataEqual(metadata, expected) {
			continue
		}
		result[path] = cloneVirtualDirectoryMetadata(metadata)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func virtualDirectoryMetadataEqual(left, right VirtualDirectoryMetadata) bool {
	if left.Version != right.Version || left.Versionless != right.Versionless || left.FilesOnly != right.FilesOnly || left.DirectoriesNil != right.DirectoriesNil || left.FilesNil != right.FilesNil {
		return false
	}
	if (left.FieldCount == nil) != (right.FieldCount == nil) || left.FieldCount != nil && *left.FieldCount != *right.FieldCount {
		return false
	}
	if len(left.FutureSlots) != len(right.FutureSlots) {
		return false
	}
	for index := range left.FutureSlots {
		if string(left.FutureSlots[index]) != string(right.FutureSlots[index]) {
			return false
		}
	}
	return true
}

func cloneVirtualDirectoryMetadata(value VirtualDirectoryMetadata) VirtualDirectoryMetadata {
	result := value
	if value.FieldCount != nil {
		fieldCount := *value.FieldCount
		result.FieldCount = &fieldCount
	}
	result.FutureSlots = cloneRawByteSlots(value.FutureSlots)
	return result
}

// GetVirtualFileMetadata returns the non-canonical, position-independent wire
// metadata for files in the table. The returned map and byte slices are deep
// copies and may be edited independently.
func (ct *ContentTable) GetVirtualFileMetadata() map[string]VirtualFileMetadata {
	if ct == nil {
		return nil
	}
	result := make(map[string]VirtualFileMetadata)
	for name, file := range ct.Files {
		if file.FieldCount == nil && len(file.FutureSlots) == 0 {
			continue
		}
		metadata := VirtualFileMetadata{FutureSlots: cloneRawByteSlots(file.FutureSlots)}
		if file.FieldCount != nil {
			fieldCount := *file.FieldCount
			metadata.FieldCount = &fieldCount
		}
		result[name] = metadata
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// ApplyVirtualFileMetadata overlays short-array/future-slot metadata after a
// semantic wrapper has rebuilt file payloads and offsets. Metadata for a file
// that does not exist is rejected because silently ignoring it would lose wire
// state from the editing document.
func (ct *ContentTable) ApplyVirtualFileMetadata(metadata map[string]VirtualFileMetadata) error {
	if ct == nil {
		return fmt.Errorf("nil content table")
	}
	for name, shape := range metadata {
		file, exists := ct.Files[name]
		if !exists {
			return fmt.Errorf("VirtualFile metadata refers to missing file %q", name)
		}
		file.FieldCount = nil
		if shape.FieldCount != nil {
			fieldCount := *shape.FieldCount
			file.FieldCount = &fieldCount
		}
		file.FutureSlots = cloneRawByteSlots(shape.FutureSlots)
		ct.Files[name] = file
	}
	return nil
}

func cloneRawByteSlots(values [][]byte) [][]byte {
	if values == nil {
		return nil
	}
	result := make([][]byte, len(values))
	for index := range values {
		result[index] = append([]byte(nil), values[index]...)
	}
	return result
}

// AddFile 向 ContentTable 追加一个虚拟文件。
// 数据会追加到 Raw 末尾，自动更新 Position 和 Size。
func (ct *ContentTable) AddFile(name string, data []byte) {
	if ct.Files == nil {
		ct.Files = make(map[string]VirtualFile)
	}
	// A table returned by ReadContentTable retains the serialized directory
	// metadata and footer in Raw, with dataEnd marking where virtual-file data
	// stops. Detach that tail before appending; otherwise the new position would
	// point into/after metadata and GetFileData would reject it. Existing file
	// offsets remain valid because the payload prefix is unchanged.
	if ct.dataEnd > 0 && ct.dataEnd <= int64(len(ct.Raw)) {
		ct.Raw = append([]byte(nil), ct.Raw[:ct.dataEnd]...)
		ct.dataEnd = 0
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
		return fmt.Errorf("decompress content table file %q: %w", name, err)
	}

	return DecodeMsgpack(decoded, out)
}

// decodeVirtualDirectory decodes one MessagePack VirtualDirectory while
// retaining the shape of every nested indexed object. codec.Raw is used for
// future slots so unknown values (including extension values) can be written
// back byte-for-byte instead of being interpreted or dropped.
func (ct *ContentTable) decodeVirtualDirectory(data []byte) error {
	root, err := decodeRawMsgpackArray(data, "VirtualDirectory root")
	if err != nil {
		return err
	}

	ct.Version = 0
	ct.Versionless = false
	ct.FilesOnly = false
	ct.DirectoriesNil = false
	ct.FilesNil = false
	ct.FieldCount = nil
	ct.FutureSlots = nil
	ct.Directories = make(map[string]VirtualDirectoryMetadata)
	ct.Files = make(map[string]VirtualFile)
	return ct.extractDirectoryFilesRaw(root, "")
}

func (ct *ContentTable) extractDirectoryFilesRaw(arr []codec.Raw, prefix string) error {
	metadata := VirtualDirectoryMetadata{}
	var directoriesRaw codec.Raw
	var filesRaw codec.Raw
	var directoriesPresent bool
	var filesPresent bool
	knownFieldCount := 3

	if len(arr) == 0 {
		// There is no version value from which to infer the versioned layout.
		// Keep the zero-width array exactly and expose the absence explicitly.
		metadata.Versionless = true
		knownFieldCount = 2
	} else {
		var first interface{}
		if err := decodeSingleRawMsgpackValue(arr[0], &first, "VirtualDirectory layout discriminator"); err != nil {
			return fmt.Errorf("VirtualDirectory %q: %w", prefix, err)
		}
		if isIntegerValue(first) {
			version, ok := toInt(first)
			if !ok {
				return fmt.Errorf("VirtualDirectory %q version is outside the Int32 range", prefix)
			}
			metadata.Version = version
			switch len(arr) {
			case 1:
				// Short generated indexed object: only Key(0) is present.
			case 2:
				// Historical compact form [version, allFiles].
				metadata.FilesOnly = true
				knownFieldCount = 2
				filesRaw = arr[1]
				filesPresent = true
			default:
				directoriesRaw = arr[1]
				filesRaw = arr[2]
				directoriesPresent = true
				filesPresent = true
			}
		} else {
			// Historical form [allDirectorys, allFiles] without Key(0).
			metadata.Versionless = true
			knownFieldCount = 2
			directoriesRaw = arr[0]
			directoriesPresent = true
			if len(arr) >= 2 {
				filesRaw = arr[1]
				filesPresent = true
			}
		}
	}

	if len(arr) != knownFieldCount {
		fieldCount := len(arr)
		metadata.FieldCount = &fieldCount
	}
	if len(arr) > knownFieldCount {
		metadata.FutureSlots = cloneCodecRawSlots(arr[knownFieldCount:])
	}

	var directories map[string]codec.Raw
	if directoriesPresent {
		var err error
		directories, metadata.DirectoriesNil, err = decodeVirtualDirectoryRawMap(directoriesRaw, fmt.Sprintf("VirtualDirectory %q allDirectorys", prefix))
		if err != nil {
			return err
		}
	}
	var files map[string]codec.Raw
	if filesPresent {
		var err error
		files, metadata.FilesNil, err = decodeVirtualDirectoryRawMap(filesRaw, fmt.Sprintf("VirtualDirectory %q allFiles", prefix))
		if err != nil {
			return err
		}
	}

	if prefix == "" {
		ct.Version = metadata.Version
		ct.Versionless = metadata.Versionless
		ct.FilesOnly = metadata.FilesOnly
		ct.DirectoriesNil = metadata.DirectoriesNil
		ct.FilesNil = metadata.FilesNil
		ct.FieldCount = metadata.FieldCount
		ct.FutureSlots = metadata.FutureSlots
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

func decodeRawMsgpackArray(data []byte, label string) ([]codec.Raw, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("%s: empty MessagePack value", label)
	}
	pos := 0
	count, err := readArrayHeader(data, &pos)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	if count > len(data)-pos {
		return nil, fmt.Errorf("%s array length %d exceeds the capacity of %d remaining bytes", label, count, len(data)-pos)
	}
	var values []codec.Raw
	if err := decodeSingleRawMsgpackValue(data, &values, label); err != nil {
		return nil, err
	}
	if len(values) != count {
		return nil, fmt.Errorf("%s decoded %d fields, header declares %d", label, len(values), count)
	}
	return values, nil
}

func decodeVirtualDirectoryRawMap(data []byte, label string) (map[string]codec.Raw, bool, error) {
	// ugorji represents a decoded MessagePack nil held in codec.Raw as a nil
	// slice rather than the one-byte 0xc0 span. Slot presence is tracked by the
	// caller, so both forms unambiguously mean a present nil map here.
	if len(data) == 0 || (len(data) == 1 && data[0] == 0xc0) {
		return nil, true, nil
	}
	count, err := messagePackMapLength(data)
	if err != nil {
		return nil, false, fmt.Errorf("%s: %w", label, err)
	}
	headerSize, err := messagePackMapHeaderSize(data)
	if err != nil {
		return nil, false, fmt.Errorf("%s: %w", label, err)
	}
	if count > (len(data)-headerSize)/2 {
		return nil, false, fmt.Errorf("%s map length %d exceeds the capacity of %d remaining bytes", label, count, len(data)-headerSize)
	}
	var generic map[interface{}]codec.Raw
	if err := decodeSingleRawMsgpackValue(data, &generic, label); err != nil {
		return nil, false, err
	}
	if len(generic) != count {
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

func messagePackMapLength(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, fmt.Errorf("empty MessagePack map value")
	}
	switch marker := data[0]; {
	case marker >= 0x80 && marker <= 0x8f:
		return int(marker & 0x0f), nil
	case marker == 0xde:
		if len(data) < 3 {
			return 0, fmt.Errorf("truncated map16 header")
		}
		return int(binary.BigEndian.Uint16(data[1:3])), nil
	case marker == 0xdf:
		if len(data) < 5 {
			return 0, fmt.Errorf("truncated map32 header")
		}
		count := uint64(binary.BigEndian.Uint32(data[1:5]))
		if count > uint64(^uint(0)>>1) {
			return 0, fmt.Errorf("map32 length %d exceeds platform int", count)
		}
		return int(count), nil
	default:
		return 0, fmt.Errorf("expected map or nil, got marker 0x%02x", marker)
	}
}

func messagePackMapHeaderSize(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, fmt.Errorf("empty MessagePack map value")
	}
	switch marker := data[0]; {
	case marker >= 0x80 && marker <= 0x8f:
		return 1, nil
	case marker == 0xde:
		if len(data) < 3 {
			return 0, fmt.Errorf("truncated map16 header")
		}
		return 3, nil
	case marker == 0xdf:
		if len(data) < 5 {
			return 0, fmt.Errorf("truncated map32 header")
		}
		return 5, nil
	default:
		return 0, fmt.Errorf("expected map, got marker 0x%02x", marker)
	}
}

func decodeSingleRawMsgpackValue(data []byte, out interface{}, label string) error {
	if len(data) == 0 {
		data = []byte{0xc0}
	}
	h := newMsgpackHandle()
	h.MaxDepth = 256
	dec := codec.NewDecoderBytes(data, h)
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("%s MessagePack decode failed: %w", label, err)
	}
	if len(data) == 1 && data[0] == 0xc0 {
		return nil
	}
	// ugorji reports an exhausted byte decoder as a wrapped "unexpected EOF"
	// rather than io.EOF. Decode the first value as Raw in a fresh decoder and
	// compare its captured span instead of probing with a second Decode call.
	var captured codec.Raw
	spanDecoder := codec.NewDecoderBytes(data, h)
	if err := spanDecoder.Decode(&captured); err != nil {
		return fmt.Errorf("%s MessagePack span decode failed: %w", label, err)
	}
	if len(captured) != len(data) {
		return fmt.Errorf("%s has %d trailing MessagePack bytes", label, len(data)-len(captured))
	}
	return nil
}

func cloneCodecRawSlots(values []codec.Raw) [][]byte {
	if values == nil {
		return nil
	}
	result := make([][]byte, len(values))
	for index := range values {
		if len(values[index]) == 0 {
			result[index] = []byte{0xc0}
		} else {
			result[index] = append([]byte(nil), values[index]...)
		}
	}
	return result
}

func decodeVirtualFileRaw(data []byte) (VirtualFile, error) {
	fields, err := decodeRawMsgpackArray(data, "VirtualFile")
	if err != nil {
		return VirtualFile{}, err
	}
	file := VirtualFile{}
	if len(fields) != 2 {
		fieldCount := len(fields)
		file.FieldCount = &fieldCount
	}
	if len(fields) >= 1 {
		var position interface{}
		if err := decodeSingleRawMsgpackValue(fields[0], &position, "VirtualFile.position"); err != nil {
			return VirtualFile{}, err
		}
		var ok bool
		file.Position, ok = toInt64(position)
		if !ok {
			return VirtualFile{}, fmt.Errorf("position/size: position must be an Int64-compatible MessagePack integer, got %T", position)
		}
	}
	if len(fields) >= 2 {
		var size interface{}
		if err := decodeSingleRawMsgpackValue(fields[1], &size, "VirtualFile.size"); err != nil {
			return VirtualFile{}, err
		}
		var ok bool
		file.Size, ok = toInt(size)
		if !ok {
			if isIntegerValue(size) {
				return VirtualFile{}, fmt.Errorf("position/size: size is outside the Int32 range")
			}
			return VirtualFile{}, fmt.Errorf("position/size: size must be an Int32 MessagePack integer, got %T", size)
		}
	}
	if len(fields) > 2 {
		file.FutureSlots = cloneCodecRawSlots(fields[2:])
	}
	return file, nil
}

// decodeVirtualFile 将 MessagePack 解码后的 indexed array [position, size] 转为 VirtualFile
func decodeVirtualFile(val interface{}) (VirtualFile, error) {
	arr, ok := val.([]interface{})
	if !ok || len(arr) < 2 {
		return VirtualFile{}, fmt.Errorf("VirtualFile: expected array(2+), got %T", val)
	}

	pos, ok1 := toInt64(arr[0])
	size, ok2 := toInt(arr[1])
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

type virtualDirNode struct {
	dirs     map[string]*virtualDirNode
	files    map[string]VirtualFile
	metadata *VirtualDirectoryMetadata
}

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

func encodeVirtualDirNode(node *virtualDirNode, inherited VirtualDirectoryMetadata, path string) ([]interface{}, error) {
	metadata := inherited
	if node.metadata != nil {
		metadata = *node.metadata
	}
	label := path
	if label == "" {
		label = "<root>"
	}
	if metadata.Versionless && metadata.FilesOnly {
		return nil, fmt.Errorf("VirtualDirectory %q cannot be both versionless and filesOnly", label)
	}
	if metadata.FilesOnly && len(node.dirs) != 0 {
		return nil, fmt.Errorf("VirtualDirectory %q filesOnly layout cannot represent %d child directories", label, len(node.dirs))
	}
	if metadata.DirectoriesNil && len(node.dirs) != 0 {
		return nil, fmt.Errorf("VirtualDirectory %q directoriesNil would discard %d child directories", label, len(node.dirs))
	}
	if metadata.FilesNil && len(node.files) != 0 {
		return nil, fmt.Errorf("VirtualDirectory %q filesNil would discard %d files", label, len(node.files))
	}

	dirs := make(map[string]interface{}, len(node.dirs))
	for name, child := range node.dirs {
		childPath := name
		if path != "" {
			childPath = path + "/" + name
		}
		childDefault := VirtualDirectoryMetadata{Version: metadata.Version, Versionless: metadata.Versionless}
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

	var directoriesValue interface{} = dirs
	if metadata.DirectoriesNil {
		directoriesValue = nil
	}
	var filesValue interface{} = files
	if metadata.FilesNil {
		filesValue = nil
	}

	known := []interface{}{int64(metadata.Version), directoriesValue, filesValue}
	if metadata.Versionless {
		known = []interface{}{directoriesValue, filesValue}
	} else if metadata.FilesOnly {
		known = []interface{}{int64(metadata.Version), filesValue}
	}
	fieldCount, err := resolveIndexedObjectFieldCount(metadata.FieldCount, len(known), metadata.FutureSlots, "VirtualDirectory "+label)
	if err != nil {
		return nil, err
	}
	if !metadata.Versionless && fieldCount < 1 && metadata.Version != 0 {
		return nil, fmt.Errorf("VirtualDirectory %q fieldCount %d would discard version=%d", label, fieldCount, metadata.Version)
	}
	if metadata.Versionless {
		if fieldCount < 1 && len(node.dirs) != 0 {
			return nil, fmt.Errorf("VirtualDirectory %q fieldCount %d would discard child directories", label, fieldCount)
		}
		if fieldCount < 2 && len(node.files) != 0 {
			return nil, fmt.Errorf("VirtualDirectory %q fieldCount %d would discard files", label, fieldCount)
		}
	} else if metadata.FilesOnly {
		if fieldCount < 2 && len(node.files) != 0 {
			return nil, fmt.Errorf("VirtualDirectory %q fieldCount %d would discard files", label, fieldCount)
		}
	} else {
		if fieldCount < 2 && len(node.dirs) != 0 {
			return nil, fmt.Errorf("VirtualDirectory %q fieldCount %d would discard child directories", label, fieldCount)
		}
		if fieldCount < 3 && len(node.files) != 0 {
			return nil, fmt.Errorf("VirtualDirectory %q fieldCount %d would discard files", label, fieldCount)
		}
	}
	if !metadata.Versionless && fieldCount >= 1 {
		if err := validateInt32Field(metadata.Version, "VirtualDirectory "+label+" version"); err != nil {
			return nil, err
		}
	}

	result := make([]interface{}, 0, fieldCount)
	for index := 0; index < fieldCount && index < len(known); index++ {
		result = append(result, known[index])
	}
	for _, raw := range metadata.FutureSlots {
		result = append(result, codec.Raw(append([]byte(nil), raw...)))
	}
	return result, nil
}

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

func encodeVirtualFile(file VirtualFile, label string) ([]interface{}, error) {
	fieldCount, err := resolveIndexedObjectFieldCount(file.FieldCount, 2, file.FutureSlots, label)
	if err != nil {
		return nil, err
	}
	if fieldCount < 1 && file.Position != 0 {
		return nil, fmt.Errorf("%s fieldCount %d would discard position=%d", label, fieldCount, file.Position)
	}
	if fieldCount < 2 && file.Size != 0 {
		return nil, fmt.Errorf("%s fieldCount %d would discard size=%d", label, fieldCount, file.Size)
	}
	if fieldCount >= 2 {
		if err := validateInt32Field(file.Size, label+" size"); err != nil {
			return nil, err
		}
	}
	known := []interface{}{file.Position, int64(file.Size)}
	result := make([]interface{}, 0, fieldCount)
	for index := 0; index < fieldCount && index < len(known); index++ {
		result = append(result, known[index])
	}
	for _, raw := range file.FutureSlots {
		result = append(result, codec.Raw(append([]byte(nil), raw...)))
	}
	return result, nil
}

func resolveIndexedObjectFieldCount(fieldCount *int, known int, futureSlots [][]byte, label string) (int, error) {
	count64 := uint64(known)
	if uint64(len(futureSlots)) > math.MaxUint32 {
		return 0, fmt.Errorf("%s futureSlots length %d exceeds the MessagePack array32 limit", label, len(futureSlots))
	}
	if fieldCount != nil {
		if *fieldCount < 0 || uint64(*fieldCount) > math.MaxUint32 {
			return 0, fmt.Errorf("%s fieldCount %d is outside the MessagePack array32 range", label, *fieldCount)
		}
		count64 = uint64(*fieldCount)
	} else if len(futureSlots) != 0 {
		count64 += uint64(len(futureSlots))
	}
	if count64 > math.MaxUint32 {
		return 0, fmt.Errorf("%s fieldCount %d exceeds the MessagePack array32 limit", label, count64)
	}
	count := int(count64)
	expectedFuture := 0
	if count > known {
		expectedFuture = count - known
	}
	if len(futureSlots) != expectedFuture {
		return 0, fmt.Errorf("%s fieldCount %d requires %d futureSlots, got %d", label, count, expectedFuture, len(futureSlots))
	}
	for index, raw := range futureSlots {
		var value codec.Raw
		if err := decodeSingleRawMsgpackValue(raw, &value, fmt.Sprintf("%s futureSlots[%d]", label, index)); err != nil {
			return 0, err
		}
	}
	return count, nil
}

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

func joinVirtualPath(prefix string, name string) string {
	parts := append(splitVirtualPath(prefix), splitVirtualPath(name)...)
	return strings.Join(parts, "/")
}

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

// canonicalVirtualComponent validates one dictionary key from allDirectorys or
// allFiles. A separator inside a key cannot be represented by the flattened
// ContentTable path model without changing the directory level on re-encode,
// so it is rejected as an unrepresentable wire shape rather than normalized.
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
	csharpInt32Min = int64(-1 << 31)
	csharpInt32Max = int64(1<<31 - 1)
)

// toInt converts a MessagePack integer to the wire width used by C# int.
// It intentionally enforces Int32 even when Go is running on a 64-bit host.
func toInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int64:
		if n < csharpInt32Min || n > csharpInt32Max {
			return 0, false
		}
		return int(n), true
	case uint64:
		if n > uint64(csharpInt32Max) {
			return 0, false
		}
		return int(n), true
	case int:
		if int64(n) < csharpInt32Min || int64(n) > csharpInt32Max {
			return 0, false
		}
		return n, true
	case uint:
		if uint64(n) > uint64(csharpInt32Max) {
			return 0, false
		}
		return int(n), true
	}
	return 0, false
}

func validateInt32Field(value int, name string) error {
	if int64(value) < csharpInt32Min || int64(value) > csharpInt32Max {
		return fmt.Errorf("%s %d is outside the Int32 range", name, value)
	}
	return nil
}

func isIntegerValue(value interface{}) bool {
	switch value.(type) {
	case int64, uint64, int, uint:
		return true
	default:
		return false
	}
}

func toInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case uint64:
		if n > uint64(^uint64(0)>>1) {
			return 0, false
		}
		return int64(n), true
	case int:
		return int64(n), true
	case uint:
		if uint64(n) > uint64(^uint64(0)>>1) {
			return 0, false
		}
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
