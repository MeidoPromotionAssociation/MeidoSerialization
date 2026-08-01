package KCES

import (
	"bytes"
	"fmt"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
)

const (
	// KCTexExtension 是 KCTexture 文件扩展名
	// KCTexExtension is the KCTexture file extension
	KCTexExtension         = ".kctex"
	kcTextureVersion int32 = 1000
)

// KCTexture 对应 ExportKCES.ExportTexture 写出的 CM3D2_TEX 1000 载荷 / KCTexture matches the CM3D2_TEX version-1000 payload written by ExportKCES.ExportTexture
type KCTexture struct {
	Signature   string `json:"signature"`   // 文件签名，KCES2 写入 CM3D2_TEX / File signature written as CM3D2_TEX by KCES2
	Version     int32  `json:"version"`     // 纹理线格式版本，KCES2 写入 1000 / Texture wire version written as 1000 by KCES2
	TextureName string `json:"textureName"` // 原始 Unity 纹理名称 / Original Unity texture name
	Data        []byte `json:"data"`        // PNG 图像载荷 / PNG image payload
}

// DecodeKCTex 解码 ExportKCES 写出的 CM3D2_TEX 1000 .kctex 数据并拒绝尾部字节
// DecodeKCTex decodes CM3D2_TEX version-1000 .kctex data written by ExportKCES and rejects trailing bytes
func DecodeKCTex(data []byte) (*KCTexture, error) {
	raw := bytes.NewReader(data)
	reader := stream.NewBinaryReader(raw)
	signature, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read KCTex signature: %w", err)
	}
	version, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read KCTex version: %w", err)
	}
	if version != kcTextureVersion {
		return nil, fmt.Errorf("unsupported KCTex version %d, expected %d", version, kcTextureVersion)
	}
	textureName, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read KCTex texture name: %w", err)
	}
	dataLength, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read KCTex data length: %w", err)
	}
	if dataLength < 0 {
		return nil, fmt.Errorf("invalid KCTex data length %d", dataLength)
	}
	payload, err := reader.ReadBytes(int64(dataLength))
	if err != nil {
		return nil, fmt.Errorf("read KCTex image payload: %w", err)
	}
	if raw.Len() != 0 {
		return nil, fmt.Errorf("trailing data after KCTex payload: %d bytes", raw.Len())
	}
	return &KCTexture{Signature: signature, Version: version, TextureName: textureName, Data: payload}, nil
}

// EncodeKCTex 编码 CM3D2_TEX 1000 .kctex 数据而不改写签名或版本
// EncodeKCTex encodes CM3D2_TEX version-1000 .kctex data without rewriting its signature or version
func EncodeKCTex(value *KCTexture) ([]byte, error) {
	if value == nil {
		return nil, fmt.Errorf("nil KCTex")
	}
	if value.Signature != "CM3D2_TEX" {
		return nil, fmt.Errorf("unsupported KCTex signature %q", value.Signature)
	}
	if value.Version != kcTextureVersion {
		return nil, fmt.Errorf("unsupported KCTex version %d, expected %d", value.Version, kcTextureVersion)
	}
	if int64(len(value.Data)) > gameInt32Max {
		return nil, fmt.Errorf("KCTex image payload length %d exceeds Int32", len(value.Data))
	}
	var out bytes.Buffer
	writer := stream.NewBinaryWriter(&out)
	if err := writer.WriteString(value.Signature); err != nil {
		return nil, fmt.Errorf("write KCTex signature: %w", err)
	}
	if err := writer.WriteInt32(value.Version); err != nil {
		return nil, fmt.Errorf("write KCTex version: %w", err)
	}
	if err := writer.WriteString(value.TextureName); err != nil {
		return nil, fmt.Errorf("write KCTex texture name: %w", err)
	}
	if err := writer.WriteInt32(int32(len(value.Data))); err != nil {
		return nil, fmt.Errorf("write KCTex data length: %w", err)
	}
	if err := writer.WriteBytes(value.Data); err != nil {
		return nil, fmt.Errorf("write KCTex image payload: %w", err)
	}
	return out.Bytes(), nil
}

// NewKCTex 创建当前 1000 版本且使用 CM3D2_TEX 签名的新 KCTexture
// NewKCTex creates a new KCTexture using version 1000 and the CM3D2_TEX signature
func NewKCTex() *KCTexture {
	return &KCTexture{Signature: "CM3D2_TEX", Version: kcTextureVersion}
}
