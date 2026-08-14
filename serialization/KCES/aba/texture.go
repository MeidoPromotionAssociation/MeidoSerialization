package aba

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
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

// Texture2DData 是导出 Unity Texture2D 所需的字段子集 / Texture2DData is the subset of Unity Texture2D fields needed for export
type Texture2DData struct {
	Name          string        // 贴图名称 / Texture name
	Width         int32         // 贴图宽度 / Texture width
	Height        int32         // 贴图高度 / Texture height
	TextureFormat int32         // Unity TextureFormat 枚举值 / Unity TextureFormat enum value
	MipCount      int32         // mipmap 层数 / Mipmap count
	ImageData     []byte        // 原始编码图像数据 / Raw encoded image data
	StreamData    StreamingInfo // 外部流式数据引用 / External streamed data reference
}

// StreamingInfo 表示 Unity 对象指向 .resS、.resource 或 .resources 条目的流式载荷范围 / StreamingInfo represents a streamed payload range in a Unity .resS, .resource, or .resources entry
type StreamingInfo struct {
	Offset int64  // sidecar 文件内偏移 / Offset inside the sidecar file
	Size   uint64 // 数据大小，AudioClip 在线格式使用 UInt64 / Payload size, stored as UInt64 by the AudioClip wire format
	Path   string // sidecar 文件路径 / Sidecar file path
}

// AbaFileResolver 解析同一 AssetBundle 内的非序列化文件，典型用途是读取 Texture2D 的 .resS 载荷
// AbaFileResolver resolves non-serialized files in the same AssetBundle, typically Texture2D .resS payload data
type AbaFileResolver func(name string) ([]byte, error)

// AbaFileRangeResolver 解析非序列化 .aba 条目中的字节范围，Texture2D m_StreamData 通常指向大型 .resS sidecar，因此范围读取可避免每张贴图加载整文件
// AbaFileRangeResolver resolves a byte range within a non-serialized .aba entry; Texture2D m_StreamData commonly points into a large .resS sidecar, so range reads avoid loading the whole file for every texture
type AbaFileRangeResolver func(name string, offset int64, size int64) ([]byte, error)

// abaRangeResolverAdapter 在整文件 resolver 和范围 resolver 之间适配 / abaRangeResolverAdapter adapts between whole-file and range resolvers
type abaRangeResolverAdapter struct {
	whole   AbaFileResolver      // 整文件读取 resolver / Whole-file resolver
	rangeFn AbaFileRangeResolver // 范围读取 resolver / Range-read resolver
}

// ResolveAbaFile 使用整文件 resolver 读取指定 .aba 条目
// ResolveAbaFile reads an .aba entry using the whole-file resolver
func (r abaRangeResolverAdapter) ResolveAbaFile(name string) ([]byte, error) {
	if r.whole == nil {
		return nil, fmt.Errorf(".aba file resolver is not available")
	}
	return r.whole(name)
}

// ResolveAbaFileRange 读取指定 .aba 条目的字节范围，缺少范围 resolver 时回退到整文件读取
// ResolveAbaFileRange reads a byte range from an .aba entry and falls back to whole-file reading when no range resolver is available
func (r abaRangeResolverAdapter) ResolveAbaFileRange(name string, offset int64, size int64) ([]byte, error) {
	if r.rangeFn != nil {
		return r.rangeFn(name, offset, size)
	}
	if r.whole == nil {
		return nil, fmt.Errorf(".aba file resolver is not available")
	}
	data, err := r.whole(name)
	if err != nil {
		return nil, err
	}
	end, ok := addNonNegativeInt64(offset, size)
	if !ok || end > int64(len(data)) {
		return nil, fmt.Errorf("range [%d,%d) out of bounds for %q (%d bytes)", offset, end, name, len(data))
	}
	return append([]byte(nil), data[offset:end]...), nil
}

// GetTexture2DData 解码 Texture2D 元数据并返回原始编码图像数据，m_StreamData 中的外部数据通过 resolver 读取
// GetTexture2DData decodes Texture2D metadata and returns raw encoded image data, resolving external m_StreamData through resolver
func (af *AssetsFile) GetTexture2DData(info *AssetInfo, resolver AbaFileResolver) (*Texture2DData, error) {
	var rangeResolver AbaFileRangeResolver
	if resolver != nil {
		rangeResolver = abaRangeResolverAdapter{whole: resolver}.ResolveAbaFileRange
	}
	return af.GetTexture2DDataRange(info, rangeResolver)
}

// GetTexture2DDataRange 是 GetTexture2DData 的范围读取版本
// GetTexture2DDataRange is the range-read variant of GetTexture2DData
func (af *AssetsFile) GetTexture2DDataRange(info *AssetInfo, resolver AbaFileRangeResolver) (*Texture2DData, error) {
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
		Width:         int32(width),
		Height:        int32(height),
		TextureFormat: int32(format),
		MipCount:      int32(mipCount),
	}

	if imageData, ok := root.Field("image data").Bytes(); ok {
		tex.ImageData = imageData
	} else if imageData, ok := root.Field("m_ImageData").Bytes(); ok {
		tex.ImageData = imageData
	}

	if stream := root.Field("m_StreamData"); stream != nil {
		tex.StreamData, err = readStreamingInfo(stream)
		if err != nil {
			return tex, err
		}
	}

	if len(tex.ImageData) == 0 && tex.StreamData.Size > 0 && resolver != nil {
		streamName := normalizeStreamDataPath(tex.StreamData.Path)
		if streamName == "" {
			goto doneTextureDataCheck
		}
		start := tex.StreamData.Offset
		if tex.StreamData.Size > math.MaxInt64 {
			return tex, fmt.Errorf("texture stream size %d exceeds Int64 resolver range", tex.StreamData.Size)
		}
		size := int64(tex.StreamData.Size)
		end, ok := addNonNegativeInt64(start, size)
		if !ok {
			return tex, fmt.Errorf("invalid stream data range offset=%d size=%d", start, size)
		}
		streamData, err := resolver(streamName, start, size)
		if err != nil {
			return tex, fmt.Errorf("read stream data %q[%d:%d]: %w", streamName, start, end, err)
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

// InlineTexture2DStreamData 将 Texture2D 的外部流式载荷写回 image data 并清空 m_StreamData
// InlineTexture2DStreamData writes a Texture2D external stream payload back into image data and clears m_StreamData
func (af *AssetsFile) InlineTexture2DStreamData(info *AssetInfo, resolver AbaFileRangeResolver) ([]byte, bool, error) {
	return af.inlineTextureStreamData(info, resolver, ClassIDTexture2D, "Texture2D")
}

// InlineCubemapStreamData 将 Cubemap 的外部流式载荷写回 image data 并清空 m_StreamData
// InlineCubemapStreamData writes a Cubemap external stream payload back into image data and clears m_StreamData
func (af *AssetsFile) InlineCubemapStreamData(info *AssetInfo, resolver AbaFileRangeResolver) ([]byte, bool, error) {
	return af.inlineTextureStreamData(info, resolver, ClassIDCubemap, "Cubemap")
}

// inlineTextureStreamData 内联共享 Texture 布局的图像载荷
// inlineTextureStreamData inlines image payloads for objects that share the Texture layout
func (af *AssetsFile) inlineTextureStreamData(info *AssetInfo, resolver AbaFileRangeResolver, expectedClassID int32, typeName string) ([]byte, bool, error) {
	if af == nil {
		return nil, false, fmt.Errorf("nil assets file")
	}
	if info == nil {
		return nil, false, fmt.Errorf("nil asset info")
	}
	if info.TypeId != expectedClassID {
		return nil, false, fmt.Errorf("asset PathID=%d has class ID %d instead of %s", info.PathId, info.TypeId, typeName)
	}

	root, err := af.ReadAssetValue(info)
	if err != nil {
		return nil, false, err
	}
	stream := root.Field("m_StreamData")
	if stream == nil {
		data, err := af.GetAssetData(info)
		if err != nil {
			return nil, false, err
		}
		return append([]byte(nil), data...), false, nil
	}
	streamInfo, err := readStreamingInfo(stream)
	if err != nil {
		return nil, false, err
	}

	imageDataValue := root.Field("image data")
	if imageDataValue == nil {
		imageDataValue = root.Field("m_ImageData")
	}
	if imageDataValue == nil {
		return nil, false, fmt.Errorf("%s PathID=%d has no image data field", typeName, info.PathId)
	}
	imageData, ok := imageDataValue.Bytes()
	if !ok {
		return nil, false, fmt.Errorf("%s PathID=%d image data is not a byte array", typeName, info.PathId)
	}

	changed := streamInfo.Offset != 0 || streamInfo.Size != 0 || streamInfo.Path != ""
	if streamInfo.Size > 0 {
		if len(imageData) != 0 {
			return nil, false, fmt.Errorf("%s PathID=%d has both inline image data and external stream data", typeName, info.PathId)
		}
		if resolver == nil {
			return nil, false, fmt.Errorf("%s PathID=%d requires an external stream resolver", typeName, info.PathId)
		}
		streamName := normalizeStreamDataPath(streamInfo.Path)
		if streamName == "" {
			return nil, false, fmt.Errorf("%s PathID=%d has stream size %d with an empty stream path", typeName, info.PathId, streamInfo.Size)
		}
		if streamInfo.Size > math.MaxInt64 {
			return nil, false, fmt.Errorf("%s PathID=%d stream size %d exceeds Int64 resolver range", typeName, info.PathId, streamInfo.Size)
		}
		imageData, err = resolver(streamName, streamInfo.Offset, int64(streamInfo.Size))
		if err != nil {
			return nil, false, fmt.Errorf("read %s PathID=%d stream data %q: %w", typeName, info.PathId, streamName, err)
		}
		if int64(len(imageData)) != int64(streamInfo.Size) {
			return nil, false, fmt.Errorf("%s PathID=%d stream resolver returned %d bytes instead of %d", typeName, info.PathId, len(imageData), streamInfo.Size)
		}
		imageDataValue.Value = append([]byte(nil), imageData...)
		imageDataValue.Children = nil
	}

	if uint64(len(imageData)) > uint64(math.MaxInt32) {
		return nil, false, fmt.Errorf("%s PathID=%d inline image data length %d exceeds Int32 wire range", typeName, info.PathId, len(imageData))
	}
	if completeSize := root.Field("m_CompleteImageSize"); completeSize != nil {
		completeSize.Value = int64(len(imageData))
	}
	clearStreamingInfo(stream)

	if !changed {
		data, err := af.GetAssetData(info)
		if err != nil {
			return nil, false, err
		}
		return append([]byte(nil), data...), false, nil
	}
	data, err := af.EncodeAssetValue(info, root)
	if err != nil {
		return nil, false, fmt.Errorf("encode inline %s PathID=%d: %w", typeName, info.PathId, err)
	}
	return data, true, nil
}

// readStreamingInfo 从 Texture、Mesh 或 AudioClip 的 TypeTreeValue 读取流路径、偏移和大小字段
// readStreamingInfo reads streamed path, offset, and size fields from a Texture, Mesh, or AudioClip TypeTreeValue
func readStreamingInfo(stream *TypeTreeValue) (StreamingInfo, error) {
	var info StreamingInfo
	if stream == nil {
		return info, nil
	}
	if field := firstTypeTreeField(stream, "offset", "m_Offset"); field != nil {
		offset, ok := field.UInt64()
		if !ok || offset > math.MaxInt64 {
			return info, fmt.Errorf("stream offset is outside Int64 range")
		}
		info.Offset = int64(offset)
	}
	if field := firstTypeTreeField(stream, "size", "m_Size"); field != nil {
		size, ok := field.UInt64()
		if !ok {
			return info, fmt.Errorf("stream size is not an unsigned integer")
		}
		info.Size = size
	}
	if field := firstTypeTreeField(stream, "path", "m_Source"); field != nil {
		path, ok := field.String()
		if !ok {
			return info, fmt.Errorf("stream path is not a string")
		}
		info.Path = path
	}
	return info, nil
}

// firstTypeTreeField 返回候选名称中第一个存在的直接字段
// firstTypeTreeField returns the first existing direct field among the candidate names
func firstTypeTreeField(value *TypeTreeValue, names ...string) *TypeTreeValue {
	for _, name := range names {
		if field := value.Field(name); field != nil {
			return field
		}
	}
	return nil
}

// clearStreamingInfo 将 Texture、Mesh 或 AudioClip 的流路径和偏移清零，并保留由调用方决定的内联大小
// clearStreamingInfo clears streamed paths and offsets for Texture, Mesh, or AudioClip while leaving the inline size decision to the caller
func clearStreamingInfo(stream *TypeTreeValue) {
	if stream == nil {
		return
	}
	if offset := firstTypeTreeField(stream, "offset", "m_Offset"); offset != nil {
		offset.Value = uint64(0)
	}
	if size := firstTypeTreeField(stream, "size", "m_Size"); size != nil {
		size.Value = uint64(0)
	}
	if streamPath := firstTypeTreeField(stream, "path", "m_Source"); streamPath != nil {
		streamPath.Value = ""
	}
}

// normalizeStreamDataPath 将 Unity stream data 路径转换为 .aba 条目查找所需的文件名
// normalizeStreamDataPath converts a Unity stream-data path to the file name used for .aba entry lookup
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

// WriteTexturePNG 通过 ImageMagick 将 Unity Texture2D 载荷转换为 PNG 文件
// WriteTexturePNG converts a Unity Texture2D payload to a PNG file through ImageMagick
func WriteTexturePNG(tex *Texture2DData, outPath string) error {
	if tex == nil {
		return fmt.Errorf("nil texture")
	}
	if err := tools.CheckMagick(); err != nil {
		return err
	}
	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create texture PNG %q: %w", outPath, err)
	}
	writeErr := WriteTexturePNGTo(tex, f)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return fmt.Errorf("close texture PNG %q: %w", outPath, closeErr)
	}
	return nil
}

// WriteTexturePNGTo 转换 Unity Texture2D 载荷并写入已打开的目标，ImageMagick 输出到 stdout 以便调用方使用受 os.Root 管理的文件
// WriteTexturePNGTo converts a Unity Texture2D payload and writes the PNG to an open destination; keeping ImageMagick on stdout lets callers use os.Root-backed files without reopening a path in the external process
func WriteTexturePNGTo(tex *Texture2DData, out io.Writer) error {
	if tex == nil {
		return fmt.Errorf("nil texture")
	}
	if out == nil {
		return fmt.Errorf("nil texture PNG writer")
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
	args = append(args, inputFormat+":-")
	// Unity 贴图载荷以左下角为原点自下而上存储，而 DDS 与原始像素输入都按自上而下解释，因此导出前必须垂直翻转才能得到正立图像
	// Unity texture payloads are stored bottom-up from the lower-left origin while DDS and raw pixel inputs are interpreted top-down, so exporting requires a vertical flip to produce an upright image
	args = append(args, "-flip", "png32:-")
	cmd := exec.Command("magick", args...)
	tools.SetHideWindow(cmd)
	cmd.Stdin = bytes.NewReader(inputData)
	cmd.Stdout = out
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("magick convert %s texture %q failed: %w, stderr: %s", textureFormatName(tex.TextureFormat), tex.Name, err, stderr.String())
	}
	return nil
}

// TexturePNGBytes 通过 ImageMagick 将 Unity Texture2D 载荷转换为 PNG 字节
// TexturePNGBytes converts a Unity Texture2D payload to PNG bytes through ImageMagick
func TexturePNGBytes(tex *Texture2DData) ([]byte, error) {
	var out bytes.Buffer
	if err := WriteTexturePNGTo(tex, &out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// textureInputForMagick 根据 Unity TextureFormat 选择 ImageMagick 输入格式和字节
// textureInputForMagick selects the ImageMagick input format and bytes for a Unity TextureFormat
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

// isRawMagickInputFormat 判断 ImageMagick 输入是否为需要尺寸参数的原始像素格式
// isRawMagickInputFormat reports whether an ImageMagick input is a raw pixel format requiring dimensions
func isRawMagickInputFormat(format string) bool {
	switch format {
	case "rgb", "rgba", "bgra", "gray":
		return true
	default:
		return false
	}
}

// argbToRGBA 将每个像素的 ARGB 通道顺序转换为 RGBA
// argbToRGBA converts each pixel from ARGB channel order to RGBA
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

// FlipRGBA32Rows 在自上而下的图像行序与 Unity 自下而上的贴图行序之间垂直翻转 RGBA32 像素
// 该变换是自身的逆运算，因此图像转 Texture2D 与 Texture2D 转图像共用同一函数，尺寸与数据长度不一致时原样返回并交由编码器报告具体错误
// FlipRGBA32Rows vertically flips RGBA32 pixels between top-down image row order and Unity's bottom-up texture row order
// The transform is its own inverse, so image-to-Texture2D and Texture2D-to-image conversion share this function; mismatched dimensions return the input unchanged and leave the encoder to report the exact error
func FlipRGBA32Rows(width int64, height int64, rgba []byte) []byte {
	if width <= 0 || height <= 0 || width > math.MaxInt32 || height > math.MaxInt32 {
		return rgba
	}
	stride := width * 4
	if height > math.MaxInt64/stride || int64(len(rgba)) != stride*height {
		return rgba
	}
	flipped := make([]byte, len(rgba))
	for row := int64(0); row < height; row++ {
		source := row * stride
		destination := (height - 1 - row) * stride
		copy(flipped[destination:destination+stride], rgba[source:source+stride])
	}
	return flipped
}

// makeDDS 为压缩纹理载荷补充 DDS 头，已有 DDS 数据则原样返回
// 块压缩载荷按 Unity 自下而上的原始块顺序透传，因为无损翻转 BC7 与 BC6H 块需要解压重压，导出的 DDS 因此相对 DirectX 约定上下颠倒
// makeDDS adds a DDS header to compressed texture payloads and returns existing DDS data unchanged
// Block-compressed payloads pass through in Unity's original bottom-up block order because losslessly flipping BC7 and BC6H blocks would require decompressing and recompressing them, so exported DDS is upside down relative to the DirectX convention
func makeDDS(tex *Texture2DData) []byte {
	if len(tex.ImageData) >= 4 && string(tex.ImageData[:4]) == "DDS " {
		return tex.ImageData
	}
	if requiresDX10DDS(tex.TextureFormat) {
		header := createDX10DDSHeader(tex.Width, tex.Height, tex.TextureFormat, tex.MipCount, int64(len(tex.ImageData)))
		return append(header, tex.ImageData...)
	}
	header := createLegacyDDSHeader(tex.Width, tex.Height, tex.TextureFormat, tex.MipCount, int64(len(tex.ImageData)))
	return append(header, tex.ImageData...)
}

// createLegacyDDSHeader 创建使用传统 FourCC 的 DDS 头
// createLegacyDDSHeader creates a DDS header using the legacy FourCC representation
func createLegacyDDSHeader(width, height int32, format int32, mipCount int32, dataLen int64) []byte {
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

// createDX10DDSHeader 创建包含 DX10 扩展头的 DDS 头
// createDX10DDSHeader creates a DDS header with a DX10 extension header
func createDX10DDSHeader(width, height int32, format int32, mipCount int32, dataLen int64) []byte {
	if mipCount <= 0 {
		mipCount = 1
	}
	buf := createLegacyDDSHeader(width, height, format, mipCount, dataLen)
	le := binary.LittleEndian
	dx10 := make([]byte, 20)
	le.PutUint32(dx10[0:4], dxgiFormat(format))
	// DX10 扩展头声明 D3D10 资源维度为 Texture2D
	// The DX10 extension header declares the D3D10 resource dimension as Texture2D
	le.PutUint32(dx10[4:8], 3)
	le.PutUint32(dx10[8:12], 0)
	le.PutUint32(dx10[12:16], 1)
	le.PutUint32(dx10[16:20], 0)
	return append(buf, dx10...)
}

// requiresDX10DDS 判断纹理格式是否必须使用 DDS DX10 扩展头
// requiresDX10DDS reports whether a texture format requires the DDS DX10 extension header
func requiresDX10DDS(format int32) bool {
	switch format {
	case TextureFormatBC4, TextureFormatBC5, TextureFormatBC6H, TextureFormatBC7:
		return true
	default:
		return false
	}
}

// dxgiFormat 将 Unity 压缩纹理格式转换为 DXGI 格式编号
// dxgiFormat converts a Unity compressed texture format to its DXGI format number
func dxgiFormat(format int32) uint32 {
	switch format {
	case TextureFormatBC4:
		// BC4 无符号归一化格式
		// BC4 unsigned normalized format
		return 80
	case TextureFormatBC5:
		// BC5 无符号归一化格式
		// BC5 unsigned normalized format
		return 83
	case TextureFormatBC6H:
		// BC6H 半精度无符号浮点格式
		// BC6H half-precision unsigned-float format
		return 95
	case TextureFormatBC7:
		// BC7 无符号归一化格式
		// BC7 unsigned normalized format
		return 98
	default:
		return 0
	}
}

// textureFormatName 返回 Unity TextureFormat 的可读名称
// textureFormatName returns a readable name for a Unity TextureFormat value
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

// WriteDDS 将原始 Unity 纹理载荷包裹 DDS 头后写入文件
// WriteDDS writes the raw Unity texture payload wrapped in a DDS header
func WriteDDS(tex *Texture2DData, outPath string) error {
	if tex == nil {
		return fmt.Errorf("nil texture")
	}
	if err := os.WriteFile(outPath, makeDDS(tex), 0644); err != nil {
		return err
	}
	return nil
}

// TextureDDSBytes 将 Texture2D 编码为可由常用图像工具读取的 DDS 字节
// TextureDDSBytes encodes a Texture2D into DDS bytes readable by common image tools
func TextureDDSBytes(tex *Texture2DData) ([]byte, error) {
	if tex == nil {
		return nil, fmt.Errorf("nil texture")
	}
	return makeDDS(tex), nil
}

// WriteRawMagickInput 将供 ImageMagick 使用的原始输入字节写入目标，便于测试和调试
// WriteRawMagickInput writes the raw bytes prepared for ImageMagick to a destination for tests and debugging
func WriteRawMagickInput(tex *Texture2DData, w io.Writer) error {
	if tex == nil {
		return fmt.Errorf("nil texture")
	}
	if w == nil {
		return fmt.Errorf("nil raw texture writer")
	}
	_, data, err := textureInputForMagick(tex)
	if err != nil {
		return err
	}
	return writeAbaBytes(w, data)
}
