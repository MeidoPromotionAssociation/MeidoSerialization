package KCES

import (
	"bytes"
	"fmt"
	"io"
	"math"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/binaryio/stream"
)

// .brd (GP03_BRIDGE)
// KCES 与 COM3D2_5 之间传递角色预设的 GP03 桥接文件
// KCES 1.34.4 写入外层版本 2001，COM3D2_5 的反向传输写入版本 2000
// .brd (GP03_BRIDGE)
// GP03 bridge file used to transfer character presets between KCES and COM3D2_5
// KCES 1.34.4 writes outer version 2001, while COM3D2_5 writes version 2000 for the reverse transfer

const (
	// GP03BridgeSignature 是 KCES ExportCM.ExportMaidData 所写每个桥接文件开头的 BinaryWriter 字符串
	// GP03BridgeSignature is the BinaryWriter string at the start of every bridge file emitted by KCES ExportCM.ExportMaidData
	GP03BridgeSignature = "GP03_BRIDGE"
	// GP03BridgeVersion 是 KCES 1.34.4 写出的桥接版本
	// GP03BridgeVersion is the bridge version emitted by KCES 1.34.4
	GP03BridgeVersion int32 = 2001
	// GP03BridgeCOM3D2Version 由 COM3D2_5 Maid.ExportBridgeGP03 为反向 COM3D2 到 KCES 传输写出，该版本的旧预设块必须为空
	// GP03BridgeCOM3D2Version is emitted by COM3D2_5 Maid.ExportBridgeGP03 for reverse COM3D2-to-KCES transfer and requires an empty legacy-preset block
	GP03BridgeCOM3D2Version int32 = 2000
	// KCESGP03BridgeFormat 是文件类型探测报告的格式标签，不再作为编辑 JSON 的字段写出
	// KCESGP03BridgeFormat is the format label reported by file-type detection and is no longer written as an editing JSON field
	KCESGP03BridgeFormat = "kces-gp03-bridge"
)

// GP03BridgeFile 表示 KCES ExportCM.cs v2001 与 COM3D2_5 Maid.ExportBridgeGP03 v2000 共用的内部 wire framing，两个预设块会在 service 层立即解码且不进入 editing JSON / GP03BridgeFile represents the internal wire framing shared by KCES ExportCM.cs v2001 and COM3D2_5 Maid.ExportBridgeGP03 v2000, with both preset blocks immediately decoded by the service layer and excluded from editing JSON
type GP03BridgeFile struct {
	Signature     string // 文件签名 GP03_BRIDGE / File signature GP03_BRIDGE
	Version       int32  // 外层桥接版本 / Outer bridge version
	GUID          string // 传输角色的 GUID / GUID of the transferred character
	LegacyPreset  []byte `json:"-"` // v2001 的 COM3D2 preset wire 块，v2000 必须为空 / COM3D2 preset wire block for v2001, required to be empty for v2000
	CurrentPreset []byte `json:"-"` // 可空 KCES preset wire 块 / Nullable KCES preset wire block
}

// IsGP03BridgeData 判断数据是否以 GP03_BRIDGE 的 BinaryWriter 编码开头，完整校验仍由 DecodeGP03Bridge 负责
// IsGP03BridgeData reports whether data begins with BinaryWriter's encoding of GP03_BRIDGE, while DecodeGP03Bridge remains responsible for full validation
func IsGP03BridgeData(data []byte) bool {
	// GP03_BRIDGE 是 11 个 UTF-8 字节，因此 BinaryWriter 使用单个 7 位长度字节，固定前缀可避免探测无关文件时按不可信长度分配内存
	// GP03_BRIDGE is eleven UTF-8 bytes, so BinaryWriter uses one 7-bit length byte, and a fixed prefix avoids allocating from an untrusted length while probing unrelated files
	return bytes.HasPrefix(data, []byte("\x0b"+GP03BridgeSignature))
}

// DecodeGP03Bridge 解码共用桥接 framing 并严格校验支持的版本、块长度和文件结尾
// DecodeGP03Bridge decodes the shared bridge framing and strictly validates the supported version, block lengths, and file end
func DecodeGP03Bridge(data []byte) (*GP03BridgeFile, error) {
	r := bytes.NewReader(data)
	br := stream.NewBinaryReader(r)

	signature, err := readGP03BridgeString(br, "signature")
	if err != nil {
		return nil, err
	}
	if signature != GP03BridgeSignature {
		return nil, fmt.Errorf("invalid GP03 bridge signature %q (expected %q)", signature, GP03BridgeSignature)
	}

	version, err := br.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read GP03 bridge version: %w", err)
	}
	if version != GP03BridgeVersion && version != GP03BridgeCOM3D2Version {
		return nil, fmt.Errorf("unsupported GP03 bridge version %d", version)
	}

	guid, err := readGP03BridgeString(br, "GUID")
	if err != nil {
		return nil, err
	}

	legacyLength, err := br.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read GP03 bridge legacy preset length: %w", err)
	}
	if legacyLength < 0 {
		return nil, fmt.Errorf("negative GP03 bridge legacy preset length %d", legacyLength)
	}
	// 旧预设载荷后必须至少保留当前预设的 Int32 长度
	// An Int32 current-preset length must remain after the legacy payload
	if r.Len() < 4 || int64(legacyLength) > int64(r.Len()-4) {
		return nil, fmt.Errorf("GP03 bridge legacy preset length %d cannot fit in %d bytes while preserving the current preset length", legacyLength, r.Len())
	}
	legacyPreset := make([]byte, int64(legacyLength))
	if _, err := io.ReadFull(r, legacyPreset); err != nil {
		return nil, fmt.Errorf("read GP03 bridge legacy preset payload: %w", err)
	}
	if version == GP03BridgeCOM3D2Version && len(legacyPreset) != 0 {
		return nil, fmt.Errorf("GP03 bridge version %d legacy preset block must be empty", version)
	}

	currentLength, err := br.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read GP03 bridge current preset length: %w", err)
	}
	if currentLength < 0 {
		return nil, fmt.Errorf("negative GP03 bridge current preset length %d", currentLength)
	}
	if int64(currentLength) > int64(r.Len()) {
		return nil, fmt.Errorf("GP03 bridge current preset length %d exceeds %d remaining bytes", currentLength, r.Len())
	}
	currentPreset := make([]byte, int64(currentLength))
	if _, err := io.ReadFull(r, currentPreset); err != nil {
		return nil, fmt.Errorf("read GP03 bridge current preset payload: %w", err)
	}
	result := &GP03BridgeFile{
		Signature:     signature,
		Version:       version,
		GUID:          guid,
		LegacyPreset:  legacyPreset,
		CurrentPreset: currentPreset,
	}
	if r.Len() != 0 {
		return nil, fmt.Errorf("GP03 bridge has %d bytes of trailing data", r.Len())
	}
	return result, nil
}

// EncodeGP03Bridge 写出准确的共用 BinaryWriter 布局，不会补默认版本或升级版本，新建 KCES 到 COM3D2 桥接时应使用 NewGP03BridgeFile
// EncodeGP03Bridge writes the exact shared BinaryWriter layout without defaulting or upgrading the version, and NewGP03BridgeFile should be used for a new KCES-to-COM3D2 bridge
func EncodeGP03Bridge(value *GP03BridgeFile) ([]byte, error) {
	if value == nil {
		return nil, fmt.Errorf("nil GP03 bridge")
	}
	signature := value.Signature
	if signature != GP03BridgeSignature {
		return nil, fmt.Errorf("invalid GP03 bridge signature %q (expected %q)", signature, GP03BridgeSignature)
	}
	version := value.Version
	if version != GP03BridgeVersion && version != GP03BridgeCOM3D2Version {
		return nil, fmt.Errorf("unsupported GP03 bridge version %d", version)
	}
	if version == GP03BridgeCOM3D2Version && len(value.LegacyPreset) != 0 {
		return nil, fmt.Errorf("GP03 bridge version %d legacy preset block must be empty", version)
	}
	if uint64(len(value.LegacyPreset)) > uint64(math.MaxInt32) {
		return nil, fmt.Errorf("GP03 bridge legacy preset length %d exceeds Int32", len(value.LegacyPreset))
	}
	if uint64(len(value.CurrentPreset)) > uint64(math.MaxInt32) {
		return nil, fmt.Errorf("GP03 bridge current preset length %d exceeds Int32", len(value.CurrentPreset))
	}

	var out bytes.Buffer
	bw := stream.NewBinaryWriter(&out)
	if err := bw.WriteString(signature); err != nil {
		return nil, fmt.Errorf("write GP03 bridge signature: %w", err)
	}
	if err := bw.WriteInt32(version); err != nil {
		return nil, fmt.Errorf("write GP03 bridge version: %w", err)
	}
	if err := bw.WriteString(value.GUID); err != nil {
		return nil, fmt.Errorf("write GP03 bridge GUID: %w", err)
	}
	if err := bw.WriteInt32(int32(len(value.LegacyPreset))); err != nil {
		return nil, fmt.Errorf("write GP03 bridge legacy preset length: %w", err)
	}
	if err := bw.WriteBytes(value.LegacyPreset); err != nil {
		return nil, fmt.Errorf("write GP03 bridge legacy preset: %w", err)
	}
	if err := bw.WriteInt32(int32(len(value.CurrentPreset))); err != nil {
		return nil, fmt.Errorf("write GP03 bridge current preset length: %w", err)
	}
	if err := bw.WriteBytes(value.CurrentPreset); err != nil {
		return nil, fmt.Errorf("write GP03 bridge current preset: %w", err)
	}
	return out.Bytes(), nil
}

// NewGP03BridgeFile 创建使用 KCES 当前签名和版本的新桥接文件
// NewGP03BridgeFile creates a new bridge file with the current KCES signature and version
func NewGP03BridgeFile() *GP03BridgeFile {
	return &GP03BridgeFile{
		Signature: GP03BridgeSignature,
		Version:   GP03BridgeVersion,
	}
}

// readGP03BridgeString 读取桥接文件中的一个 BinaryWriter 字符串字段
// readGP03BridgeString reads one BinaryWriter string field from a bridge file
func readGP03BridgeString(br *stream.BinaryReader, field string) (string, error) {
	value, err := br.ReadString()
	if err != nil {
		return "", fmt.Errorf("read GP03 bridge %s: %w", field, err)
	}
	return value, nil
}
