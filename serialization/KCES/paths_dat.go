package KCES

import (
	"bytes"
	"fmt"
	"math"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio"
)

// paths.dat (CM3D2_PATHS)
// KCES 原生资源搜索路径列表，由 NativeFileManager.ReadAutoPathFile 读取
// 布局为 BinaryWriter 字符串签名、Int32 版本、Int32 数量及对应数量的字符串，当前版本为 1000
// paths.dat (CM3D2_PATHS)
// KCES native resource search-path list consumed by NativeFileManager.ReadAutoPathFile
// The layout is a BinaryWriter string signature, Int32 version, Int32 count, and that many strings, with current version 1000

const (
	KCESPathsFormat    = "kces-auto-paths"
	KCESPathsSignature = "CM3D2_PATHS"
	kcesPathsVersion   = int32(1000)
)

// KCESPathsFile 表示 NativeFileManager.ReadAutoPathFile 读取的 paths.dat 路径列表，依次包含 .NET 字符串签名、Int32 版本、Int32 数量和对应数量的 .NET 字符串
// KCESPathsFile represents the paths.dat list consumed by NativeFileManager.ReadAutoPathFile, containing a .NET string signature, Int32 version, Int32 count, and that many .NET strings
type KCESPathsFile struct {
	Format       string   `json:"format"`                 // JSON 表示格式标识 / JSON representation format identifier
	Signature    string   `json:"signature"`              // 文件签名 CM3D2_PATHS / File signature CM3D2_PATHS
	Version      int32    `json:"version"`                // 路径列表格式版本 / Path-list format version
	Paths        []string `json:"paths"`                  // 原生资源搜索路径 / Native resource search paths
	TrailingData []byte   `json:"trailingData,omitempty"` // 游戏读取声明路径后忽略的尾部字节 / Trailing bytes ignored by the game after reading the declared paths
}

// DecodeKCESPaths 解码 paths.dat 路径列表并保留游戏忽略的尾部数据
// DecodeKCESPaths decodes a paths.dat path list and preserves trailing data ignored by the game
func DecodeKCESPaths(data []byte) (*KCESPathsFile, error) {
	r := bytes.NewReader(data)
	signature, err := binaryio.ReadString(r)
	if err != nil {
		return nil, fmt.Errorf("read paths.dat signature: %w", err)
	}
	version, err := binaryio.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("read paths.dat version: %w", err)
	}
	count, err := binaryio.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("read paths.dat count: %w", err)
	}
	if count < 0 {
		return nil, fmt.Errorf("negative paths.dat count %d", count)
	}
	// 每个 BinaryWriter 字符串至少需要一个长度字节
	// Every BinaryWriter string requires at least one length byte
	if int64(count) > int64(r.Len()) {
		return nil, fmt.Errorf("paths.dat count %d cannot fit in %d remaining bytes", count, r.Len())
	}

	paths := makeKCESCountedSliceForAppend[string](uint64(count))
	for i := int32(0); i < count; i++ {
		path, readErr := binaryio.ReadString(r)
		if readErr != nil {
			return nil, fmt.Errorf("read paths.dat paths[%d]: %w", i, readErr)
		}
		paths = append(paths, path)
	}
	result := &KCESPathsFile{
		Format:    KCESPathsFormat,
		Signature: signature,
		Version:   version,
		Paths:     paths,
	}
	if r.Len() != 0 {
		result.TrailingData = make([]byte, r.Len())
		if _, err := r.Read(result.TrailingData); err != nil {
			return nil, fmt.Errorf("read paths.dat trailingData: %w", err)
		}
	}
	return result, nil
}

// EncodeKCESPaths 编码 paths.dat 路径列表及其保留的尾部数据
// EncodeKCESPaths encodes a paths.dat path list and its preserved trailing data
func EncodeKCESPaths(value *KCESPathsFile) ([]byte, error) {
	if value == nil {
		return nil, fmt.Errorf("nil paths.dat value")
	}
	if value.Format != "" && value.Format != KCESPathsFormat {
		return nil, fmt.Errorf("unsupported paths.dat JSON format %q", value.Format)
	}
	signature := value.Signature
	version := value.Version
	if len(value.Paths) > math.MaxInt32 {
		return nil, fmt.Errorf("paths.dat path count %d exceeds Int32", len(value.Paths))
	}
	count := int32(len(value.Paths))

	var out bytes.Buffer
	if err := binaryio.WriteString(&out, signature); err != nil {
		return nil, fmt.Errorf("write paths.dat signature: %w", err)
	}
	if err := binaryio.WriteInt32(&out, version); err != nil {
		return nil, fmt.Errorf("write paths.dat version: %w", err)
	}
	if err := binaryio.WriteInt32(&out, count); err != nil {
		return nil, fmt.Errorf("write paths.dat count: %w", err)
	}
	for i, path := range value.Paths {
		if err := binaryio.WriteString(&out, path); err != nil {
			return nil, fmt.Errorf("write paths.dat paths[%d]: %w", i, err)
		}
	}
	if _, err := out.Write(value.TrailingData); err != nil {
		return nil, fmt.Errorf("write paths.dat trailingData: %w", err)
	}
	return out.Bytes(), nil
}

// NewKCESPathsFile 创建使用当前签名和版本的新路径列表
// NewKCESPathsFile creates a new path list with the current signature and version
func NewKCESPathsFile() *KCESPathsFile {
	return &KCESPathsFile{
		Format:    KCESPathsFormat,
		Signature: KCESPathsSignature,
		Version:   kcesPathsVersion,
	}
}
