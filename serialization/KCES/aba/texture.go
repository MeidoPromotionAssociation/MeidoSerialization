package aba

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/tools"
)

const (
	TextureFormatAlpha8 int32 = 1
	TextureFormatRGB24  int32 = 3
	TextureFormatRGBA32 int32 = 4
	TextureFormatARGB32 int32 = 5
	TextureFormatDXT1   int32 = 10
	TextureFormatDXT5   int32 = 12
	TextureFormatBGRA32 int32 = 14
	TextureFormatBC6H   int32 = 24
	TextureFormatBC7    int32 = 25
	TextureFormatBC4    int32 = 26
	TextureFormatBC5    int32 = 27
	TextureFormatR8     int32 = 63
	TextureFormatRGBA64 int32 = 72
)

// Texture2DData 是导出所需的 Unity Texture2D 有用字段子集 / Texture2DData is the useful subset of Unity Texture2D needed for export
type Texture2DData struct {
	Name          string        // 贴图名称 / Texture name
	Width         int           // 贴图宽度 / Texture width
	Height        int           // 贴图高度 / Texture height
	TextureFormat int32         // Unity TextureFormat 枚举值 / Unity TextureFormat enum value
	MipCount      int           // mipmap 层数 / Mipmap count
	ImageData     []byte        // 原始编码图像数据 / Raw encoded image data
	StreamData    StreamingInfo // 外部流式数据引用 / External streamed data reference
}

// StreamingInfo 指向 bundle sidecar 文件中保存的贴图载荷 / StreamingInfo points to texture payload stored in a bundle sidecar file
type StreamingInfo struct {
	Offset int64  // sidecar 文件内偏移 / Offset inside the sidecar file
	Size   uint32 // 数据大小 / Data size
	Path   string // sidecar 文件路径 / Sidecar file path
}

// BundleFileResolver 解析同一 AssetBundle 内的非序列化文件 / BundleFileResolver resolves non-serialized files in the same AssetBundle
// 典型用途是 Texture2D 的 .resS 载荷 / A typical use is Texture2D .resS payload data
type BundleFileResolver func(name string) ([]byte, error)

// BundleFileRangeResolver 解析非序列化 bundle 文件中的字节范围 / BundleFileRangeResolver resolves a byte range within a non-serialized bundle file
// Texture2D m_StreamData 通常指向大型 .resS sidecar，范围读取避免每张贴图都加载整文件 / Texture2D m_StreamData commonly points into a large .resS sidecar, and range reads avoid loading the whole file for every texture
type BundleFileRangeResolver func(name string, offset int64, size int64) ([]byte, error)

// bundleRangeResolverAdapter 在整文件 resolver 和范围 resolver 之间适配 / bundleRangeResolverAdapter adapts between whole-file and range resolvers
type bundleRangeResolverAdapter struct {
	whole   BundleFileResolver      // 整文件读取 resolver / Whole-file resolver
	rangeFn BundleFileRangeResolver // 范围读取 resolver / Range-read resolver
}

func (r bundleRangeResolverAdapter) ResolveBundleFile(name string) ([]byte, error) {
	if r.whole == nil {
		return nil, fmt.Errorf("bundle file resolver is not available")
	}
	return r.whole(name)
}

func (r bundleRangeResolverAdapter) ResolveBundleFileRange(name string, offset int64, size int64) ([]byte, error) {
	if r.rangeFn != nil {
		return r.rangeFn(name, offset, size)
	}
	if r.whole == nil {
		return nil, fmt.Errorf("bundle file resolver is not available")
	}
	data, err := r.whole(name)
	if err != nil {
		return nil, err
	}
	end := offset + size
	if offset < 0 || size < 0 || end < offset || end > int64(len(data)) {
		return nil, fmt.Errorf("range [%d,%d) out of bounds for %q (%d bytes)", offset, end, name, len(data))
	}
	return append([]byte(nil), data[offset:end]...), nil
}

// GetTexture2DData decodes Texture2D metadata and returns its raw encoded
// picture data. If image data is stored in m_StreamData, resolver is used.
func (af *AssetsFile) GetTexture2DData(info *AssetInfo, resolver BundleFileResolver) (*Texture2DData, error) {
	var rangeResolver BundleFileRangeResolver
	if resolver != nil {
		rangeResolver = bundleRangeResolverAdapter{whole: resolver}.ResolveBundleFileRange
	}
	return af.GetTexture2DDataRange(info, rangeResolver)
}

// GetTexture2DDataRange is the range-read variant of GetTexture2DData.
func (af *AssetsFile) GetTexture2DDataRange(info *AssetInfo, resolver BundleFileRangeResolver) (*Texture2DData, error) {
	root, err := af.ReadAssetValue(info)
	if err != nil {
		return nil, err
	}
	name, _ := root.Field("m_Name").String()
	width, _ := root.Field("m_Width").Int64()
	height, _ := root.Field("m_Height").Int64()
	format, _ := root.Field("m_TextureFormat").Int64()
	mipCount, _ := root.Field("m_MipCount").Int64()

	tex := &Texture2DData{
		Name:          name,
		Width:         int(width),
		Height:        int(height),
		TextureFormat: int32(format),
		MipCount:      int(mipCount),
	}

	if imageData, ok := root.Field("image data").Bytes(); ok {
		tex.ImageData = imageData
	} else if imageData, ok := root.Field("m_ImageData").Bytes(); ok {
		tex.ImageData = imageData
	}

	if stream := root.Field("m_StreamData"); stream != nil {
		if off, ok := stream.Field("offset").Int64(); ok {
			tex.StreamData.Offset = off
		}
		if size, ok := stream.Field("size").Int64(); ok && size >= 0 {
			tex.StreamData.Size = uint32(size)
		}
		if p, ok := stream.Field("path").String(); ok {
			tex.StreamData.Path = p
		}
	}

	if len(tex.ImageData) == 0 && tex.StreamData.Size > 0 && resolver != nil {
		streamName := normalizeStreamDataPath(tex.StreamData.Path)
		if streamName == "" {
			goto doneTextureDataCheck
		}
		start := tex.StreamData.Offset
		size := int64(tex.StreamData.Size)
		streamData, err := resolver(streamName, start, size)
		if err != nil {
			return tex, fmt.Errorf("read stream data %q[%d:%d]: %w", streamName, start, start+size, err)
		}
		tex.ImageData = streamData
	}

doneTextureDataCheck:
	if tex.Width <= 0 || tex.Height <= 0 {
		return tex, fmt.Errorf("invalid texture dimensions %dx%d", tex.Width, tex.Height)
	}
	if len(tex.ImageData) == 0 {
		return tex, fmt.Errorf("texture has no image data")
	}
	return tex, nil
}

func normalizeStreamDataPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" || p == "." {
		return ""
	}
	p = strings.TrimPrefix(p, "archive:/")
	p = strings.TrimPrefix(p, "archive://")
	p = strings.TrimLeft(p, "/\\")
	if p == "" || p == "." {
		return ""
	}
	return path.Base(strings.ReplaceAll(p, "\\", "/"))
}

// WriteTexturePNG converts a Unity Texture2D payload to PNG via ImageMagick.
func WriteTexturePNG(tex *Texture2DData, outPath string) error {
	if tex == nil {
		return fmt.Errorf("nil texture")
	}
	if err := tools.CheckMagick(); err != nil {
		return err
	}

	inputFormat, inputData, err := textureInputForMagick(tex)
	if err != nil {
		return err
	}

	args := []string{}
	if isRawMagickInputFormat(inputFormat) {
		args = append(args, "-size", fmt.Sprintf("%dx%d", tex.Width, tex.Height), "-depth", "8")
	}
	args = append(args, inputFormat+":-", "png32:"+outPath)
	cmd := exec.Command("magick", args...)
	tools.SetHideWindow(cmd)
	cmd.Stdin = bytes.NewReader(inputData)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("magick convert %s texture %q failed: %w, stderr: %s", textureFormatName(tex.TextureFormat), tex.Name, err, stderr.String())
	}
	return nil
}

// TexturePNGBytes converts a Unity Texture2D payload to PNG bytes via ImageMagick.
func TexturePNGBytes(tex *Texture2DData) ([]byte, error) {
	if tex == nil {
		return nil, fmt.Errorf("nil texture")
	}
	if err := tools.CheckMagick(); err != nil {
		return nil, err
	}
	inputFormat, inputData, err := textureInputForMagick(tex)
	if err != nil {
		return nil, err
	}
	args := []string{}
	if isRawMagickInputFormat(inputFormat) {
		args = append(args, "-size", fmt.Sprintf("%dx%d", tex.Width, tex.Height), "-depth", "8")
	}
	args = append(args, inputFormat+":-", "png32:-")
	cmd := exec.Command("magick", args...)
	tools.SetHideWindow(cmd)
	cmd.Stdin = bytes.NewReader(inputData)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("magick convert %s texture %q failed: %w, stderr: %s", textureFormatName(tex.TextureFormat), tex.Name, err, stderr.String())
	}
	return out, nil
}

func textureInputForMagick(tex *Texture2DData) (string, []byte, error) {
	switch tex.TextureFormat {
	case TextureFormatDXT1, TextureFormatDXT5, TextureFormatBC4, TextureFormatBC5, TextureFormatBC6H, TextureFormatBC7:
		return "dds", makeDDS(tex), nil
	case TextureFormatRGB24:
		return "rgb", tex.ImageData, nil
	case TextureFormatRGBA32:
		return "rgba", tex.ImageData, nil
	case TextureFormatARGB32:
		return "rgba", argbToRGBA(tex.ImageData), nil
	case TextureFormatBGRA32:
		return "bgra", tex.ImageData, nil
	case TextureFormatAlpha8, TextureFormatR8:
		return "gray", tex.ImageData, nil
	default:
		if len(tex.ImageData) >= 4 && string(tex.ImageData[:4]) == "DDS " {
			return "dds", tex.ImageData, nil
		}
		return "", nil, fmt.Errorf("unsupported Unity TextureFormat %d (%s)", tex.TextureFormat, textureFormatName(tex.TextureFormat))
	}
}

func isRawMagickInputFormat(format string) bool {
	switch format {
	case "rgb", "rgba", "bgra", "gray":
		return true
	default:
		return false
	}
}

func argbToRGBA(data []byte) []byte {
	out := make([]byte, len(data))
	for i := 0; i+3 < len(data); i += 4 {
		out[i+0] = data[i+1]
		out[i+1] = data[i+2]
		out[i+2] = data[i+3]
		out[i+3] = data[i+0]
	}
	return out
}

func makeDDS(tex *Texture2DData) []byte {
	if len(tex.ImageData) >= 4 && string(tex.ImageData[:4]) == "DDS " {
		return tex.ImageData
	}
	if requiresDX10DDS(tex.TextureFormat) {
		header := createDX10DDSHeader(tex.Width, tex.Height, tex.TextureFormat, tex.MipCount, len(tex.ImageData))
		return append(header, tex.ImageData...)
	}
	header := createLegacyDDSHeader(tex.Width, tex.Height, tex.TextureFormat, tex.MipCount, len(tex.ImageData))
	return append(header, tex.ImageData...)
}

func createLegacyDDSHeader(width, height int, format int32, mipCount int, dataLen int) []byte {
	if mipCount <= 0 {
		mipCount = 1
	}
	buf := make([]byte, 128)
	copy(buf[0:4], "DDS ")
	le := binary.LittleEndian
	le.PutUint32(buf[4:8], 124)
	flags := uint32(0x1 | 0x2 | 0x4 | 0x1000 | 0x80000)
	if mipCount > 1 {
		flags |= 0x20000
	}
	le.PutUint32(buf[8:12], flags)
	le.PutUint32(buf[12:16], uint32(height))
	le.PutUint32(buf[16:20], uint32(width))
	le.PutUint32(buf[20:24], uint32(dataLen))
	le.PutUint32(buf[28:32], uint32(mipCount))

	pf := 76
	le.PutUint32(buf[pf:pf+4], 32)
	le.PutUint32(buf[pf+4:pf+8], 0x4)
	switch format {
	case TextureFormatDXT1:
		copy(buf[pf+8:pf+12], "DXT1")
	case TextureFormatDXT5:
		copy(buf[pf+8:pf+12], "DXT5")
	default:
		copy(buf[pf+8:pf+12], "DX10")
	}

	caps := uint32(0x1000)
	if mipCount > 1 {
		caps |= 0x8 | 0x400000
	}
	le.PutUint32(buf[108:112], caps)
	return buf
}

func createDX10DDSHeader(width, height int, format int32, mipCount int, dataLen int) []byte {
	if mipCount <= 0 {
		mipCount = 1
	}
	buf := createLegacyDDSHeader(width, height, format, mipCount, dataLen)
	le := binary.LittleEndian
	dx10 := make([]byte, 20)
	le.PutUint32(dx10[0:4], dxgiFormat(format))
	le.PutUint32(dx10[4:8], 3) // D3D10_RESOURCE_DIMENSION_TEXTURE2D
	le.PutUint32(dx10[8:12], 0)
	le.PutUint32(dx10[12:16], 1)
	le.PutUint32(dx10[16:20], 0)
	return append(buf, dx10...)
}

func requiresDX10DDS(format int32) bool {
	switch format {
	case TextureFormatBC4, TextureFormatBC5, TextureFormatBC6H, TextureFormatBC7:
		return true
	default:
		return false
	}
}

func dxgiFormat(format int32) uint32 {
	switch format {
	case TextureFormatBC4:
		return 80 // DXGI_FORMAT_BC4_UNORM
	case TextureFormatBC5:
		return 83 // DXGI_FORMAT_BC5_UNORM
	case TextureFormatBC6H:
		return 95 // DXGI_FORMAT_BC6H_UF16
	case TextureFormatBC7:
		return 98 // DXGI_FORMAT_BC7_UNORM
	default:
		return 0
	}
}

func textureFormatName(format int32) string {
	switch format {
	case TextureFormatAlpha8:
		return "Alpha8"
	case TextureFormatRGB24:
		return "RGB24"
	case TextureFormatRGBA32:
		return "RGBA32"
	case TextureFormatARGB32:
		return "ARGB32"
	case TextureFormatDXT1:
		return "DXT1"
	case TextureFormatDXT5:
		return "DXT5"
	case TextureFormatBGRA32:
		return "BGRA32"
	case TextureFormatBC6H:
		return "BC6H"
	case TextureFormatBC7:
		return "BC7"
	case TextureFormatBC4:
		return "BC4"
	case TextureFormatBC5:
		return "BC5"
	case TextureFormatR8:
		return "R8"
	case TextureFormatRGBA64:
		return "RGBA64"
	default:
		return fmt.Sprintf("TextureFormat_%d", format)
	}
}

// WriteDDS writes the raw Unity texture payload wrapped in a DDS header.
func WriteDDS(tex *Texture2DData, outPath string) error {
	if tex == nil {
		return fmt.Errorf("nil texture")
	}
	if err := os.WriteFile(outPath, makeDDS(tex), 0644); err != nil {
		return err
	}
	return nil
}

// WriteRawMagickInput is a small debugging helper used by tests and callers
// that need to inspect ImageMagick input bytes.
func WriteRawMagickInput(tex *Texture2DData, w io.Writer) error {
	_, data, err := textureInputForMagick(tex)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}
