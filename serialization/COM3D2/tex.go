package COM3D2

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/binaryio/stream"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/tools"
)

// CM3D2_TEX
// 纹理文件
//
// CM3D2 支持 1000 至 1010 版本
// COM3D2 支持 1000 至 1011 版本
//
// 1000 版本没有显式的宽高字段，宽高存储在图像数据头的固定位置 16 至 23 字节
// 1010 版本增加显式的宽高和纹理格式字段并支持 DXT5 与 DXT1，DXT 载荷不含 DDS 文件头，只包含原始像素块
// 1011 版本新增用于纹理图集的矩形数组，每个矩形包含 x、y、width、height
//
// TextureFormat 为 ARGB32 或 RGB24 时数据是 PNG 或 JPG，DXT5 或 DXT1 时数据是原始 DXT 块
// 本序列化不支持写出 1000 版本
// 部分错误的 .tex 文件虽然使用 RGB24 格式，但嵌入的却是 PNG 数据，因此本程序会在格式为 RGB24 和 ARGB32 时尝试解析数据魔数以确定真实格式
//
// Unity 的 UV 坐标原点在图像左下角且 V 轴向上增长，DirectX DDS 的纹理坐标原点在图像左上角且 V 轴向下增长
// 因为标准 DDS 与 Unity 使用的 DXT 块在垂直方向上存在翻转，本库会在读写 DXT1 和 DXT5 时反转块顺序，使写入 TEX 的数据符合 Unity 顺序而导出的 DDS 符合 DirectX 顺序
// CM3D2_TEX
// Texture file
//
// CM3D2 supports versions 1000 through 1010
// COM3D2 supports versions 1000 through 1011
//
// Version 1000 has no explicit width and height fields, with both values stored at fixed image-header byte positions 16 through 23
// Version 1010 adds explicit width, height, and texture-format fields plus DXT5 and DXT1 support, with DXT payloads containing raw pixel blocks without a DDS header
// Version 1011 adds a texture-atlas rectangle array whose entries contain x, y, width, and height
//
// Data is PNG or JPG when TextureFormat is ARGB32 or RGB24 and raw DXT blocks when it is DXT5 or DXT1
// This serializer does not write version 1000
// Some malformed .tex files declare RGB24 while embedding PNG data, so this implementation checks the data signature for RGB24 and ARGB32 payloads to determine the actual format
//
// Unity uses a bottom-left UV origin with V increasing upward, while DirectX DDS uses a top-left texture origin with V increasing downward
// Because standard DDS and Unity DXT block order are vertically reversed, this library flips DXT1 and DXT5 blocks so TEX output follows Unity order and exported DDS follows DirectX order

// 下列值来自 Unity 5.6.4 的 TextureFormat 枚举并仅由 COM3D2 使用
// The following values come from the Unity 5.6.4 TextureFormat enum and are used only by COM3D2
const (
	RGB24  int32 = 3
	ARGB32 int32 = 5
	DXT1   int32 = 10
	DXT5   int32 = 12
)

// TexRect 保存版本 1011 纹理图集中的一个 Unity Rect
// TexRect stores one Unity Rect in a version-1011 texture atlas
type TexRect struct {
	X float32 `json:"X"` // 矩形左下角 X 坐标 / Rectangle lower-left X coordinate
	Y float32 `json:"Y"` // 矩形左下角 Y 坐标 / Rectangle lower-left Y coordinate
	W float32 `json:"W"` // 矩形宽度 / Rectangle width
	H float32 `json:"H"` // 矩形高度 / Rectangle height
}

// Tex 保存 CM3D2_TEX 头部、可选图集矩形和图像载荷
// Tex stores the CM3D2_TEX header, optional atlas rectangles, and image payload
type Tex struct {
	Signature     string    `json:"Signature"`     // 文件签名，通常为 CM3D2_TEX / File signature, normally CM3D2_TEX
	Version       int32     `json:"Version"`       // 控制尺寸和图集字段的格式版本 / Format version controlling dimension and atlas fields
	TextureName   string    `json:"TextureName"`   // 游戏读取后未使用的编译源纹理名称 / Compiled source texture name read but unused by the game
	Rects         []TexRect `json:"Rects"`         // 版本 1011 及以上的图集矩形 / Atlas rectangles for version 1011 and later
	Width         int32     `json:"Width"`         // 纹理像素宽度 / Texture width in pixels
	Height        int32     `json:"Height"`        // 纹理像素高度 / Texture height in pixels
	TextureFormat int32     `json:"TextureFormat"` // Unity TextureFormat 枚举值 / Unity TextureFormat enum value
	Data          []byte    `json:"Data"`          // 编码图像或原始 DXT 块 / Encoded image or raw DXT blocks
}

// ReadTex 从二进制流中读取 Tex 数据，输入数据流需要使用 .tex 格式
// ReadTex reads Tex data from a binary stream whose contents must use the .tex format
func ReadTex(r io.Reader) (*Tex, error) {
	reader := stream.NewBinaryReader(r)

	// 读取签名
	// Read the signature
	sig, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read .tex signature failed: %w", err)
	}
	// 读取版本号
	// Read the version

	ver, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read .tex version failed: %w", err)
	}

	// 读取纹理名称
	// Read the texture name
	texName, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read .tex textureName failed: %w", err)
	}

	// 版本不小于 1011 时读取纹理图集矩形
	// Read texture-atlas rectangles for version 1011 or later
	var rects []TexRect
	if ver >= 1011 {
		rectCount, err := reader.ReadInt32()
		if err != nil {
			return nil, fmt.Errorf("read .tex rectCount failed: %w", err)
		}
		if rectCount < 0 {
			return nil, fmt.Errorf("invalid .tex rectCount: %d", rectCount)
		}
		// 限制矩形数量以防损坏文件触发过量分配
		// Limit the rectangle count so corrupt files cannot trigger excessive allocation
		if rectCount > 100000 {
			return nil, fmt.Errorf("too many rects in .tex: %d", rectCount)
		}
		if rectCount > 0 {
			rects = make([]TexRect, rectCount)
			for i := int32(0); i < rectCount; i++ {
				x, err := reader.ReadFloat32()
				if err != nil {
					return nil, fmt.Errorf("read .tex rects[%d].x failed: %w", i, err)
				}
				y, err := reader.ReadFloat32()
				if err != nil {
					return nil, fmt.Errorf("read .tex rects[%d].y failed: %w", i, err)
				}
				w, err := reader.ReadFloat32()
				if err != nil {
					return nil, fmt.Errorf("read .tex rects[%d].w failed: %w", i, err)
				}
				h, err := reader.ReadFloat32()
				if err != nil {
					return nil, fmt.Errorf("read .tex rects[%d].h failed: %w", i, err)
				}
				rects[i] = TexRect{x, y, w, h}
			}
		}
	}

	// 版本不小于 1010 时读取宽度、高度和纹理格式
	// Read width, height, and texture format for version 1010 or later
	var width, height, texFmt int32
	if ver >= 1010 {
		w, err := reader.ReadInt32()
		if err != nil {
			return nil, fmt.Errorf("read .tex width failed: %w", err)
		}
		h, err := reader.ReadInt32()
		if err != nil {
			return nil, fmt.Errorf("read .tex height failed: %w", err)
		}
		f, err := reader.ReadInt32()
		if err != nil {
			return nil, fmt.Errorf("read .tex textureFormat failed: %w", err)
		}
		width, height, texFmt = w, h, f
	}

	// 读取数据块长度
	// Read the data-block length
	dataLen, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read .tex dataLength failed: %w", err)
	}
	if dataLen < 0 {
		return nil, fmt.Errorf("invalid .tex dataLength: %d", dataLen)
	}

	// 读取图像数据块
	// Read the image data block
	data, err := reader.ReadBytes(int64(dataLen))
	if err != nil {
		return nil, fmt.Errorf("read .tex raw data failed: %w", err)
	}

	// 版本 1000 从图像数据头的 16 至 23 字节解析宽度和高度
	// Version 1000 parses width and height from image-header bytes 16 through 23
	if ver == 1000 {
		if len(data) < 24 {
			return nil, fmt.Errorf(".tex data too short for version=1000")
		}
		width = int32(binary.LittleEndian.Uint32(data[16:20]))
		height = int32(binary.LittleEndian.Uint32(data[20:24]))
	}

	tex := &Tex{
		Signature:     sig,
		Version:       ver,
		TextureName:   texName,
		Rects:         rects,
		Width:         width,
		Height:        height,
		TextureFormat: texFmt,
		Data:          data,
	}
	return tex, nil
}

// Dump 将 Tex 数据写入 .tex 格式的二进制流，本实现不支持写出版本 1000
// Dump writes Tex data to a binary stream in .tex format and does not support writing version 1000
func (t *Tex) Dump(w io.Writer) error {
	writer := stream.NewBinaryWriter(w)

	// 写入签名
	// Write the signature
	if err := writer.WriteString(t.Signature); err != nil {
		return fmt.Errorf("write signature failed: %w", err)
	}
	// 写入版本号
	// Write the version
	if err := writer.WriteInt32(t.Version); err != nil {
		return fmt.Errorf("write version failed: %w", err)
	}
	// 写入纹理名称
	// Write the texture name
	if err := writer.WriteString(t.TextureName); err != nil {
		return fmt.Errorf("write textureName failed: %w", err)
	}

	if t.Version == 1000 {
		return fmt.Errorf("dump version 1000 is not supported, You should at least convert it to 1010 version, " +
			"maybe you can convert it to image and convert back to .tex")
	}

	// 版本不小于 1011 时写出纹理图集矩形
	// Write texture-atlas rectangles for version 1011 or later
	if t.Version >= 1011 {
		rectCount := int32(len(t.Rects))
		if err := writer.WriteInt32(rectCount); err != nil {
			return fmt.Errorf("write rectCount failed: %w", err)
		}
		for _, rect := range t.Rects {
			if err := writer.WriteFloat32(rect.X); err != nil {
				return fmt.Errorf("write rect.X failed: %w", err)
			}
			if err := writer.WriteFloat32(rect.Y); err != nil {
				return fmt.Errorf("write rect.Y failed: %w", err)
			}
			if err := writer.WriteFloat32(rect.W); err != nil {
				return fmt.Errorf("write rect.W failed: %w", err)
			}
			if err := writer.WriteFloat32(rect.H); err != nil {
				return fmt.Errorf("write rect.H failed: %w", err)
			}
		}
	}

	// 版本不小于 1010 时写出宽度、高度和纹理格式
	// Write width, height, and texture format for version 1010 or later
	if t.Version >= 1010 {
		if err := writer.WriteInt32(t.Width); err != nil {
			return fmt.Errorf("write width failed: %w", err)
		}
		if err := writer.WriteInt32(t.Height); err != nil {
			return fmt.Errorf("write height failed: %w", err)
		}
		if err := writer.WriteInt32(t.TextureFormat); err != nil {
			return fmt.Errorf("write textureFormat failed: %w", err)
		}
	}

	// 写出数据块长度
	// Write the data-block length
	dataLen := int32(len(t.Data))
	if err := writer.WriteInt32(dataLen); err != nil {
		return fmt.Errorf("write dataLen failed: %w", err)
	}
	// 写出图像数据块
	// Write the image data block
	if _, err := w.Write(t.Data); err != nil {
		return fmt.Errorf("write data block failed: %w", err)
	}

	return nil
}

// ConvertImageToTex 将任意 ImageMagick 支持的文件格式转换为 tex 格式，但不写出
// 依赖外部库 ImageMagick，并要求 PATH 环境变量可以直接调用 magick 命令
// forcePNG 为 true 且 compress 为 false 时，tex 数据是原始 PNG 或转换后的 PNG
// forcePNG 为 false 且 compress 为 false 时，PNG 或 JPG 输入会直接使用，否则有损且无透明通道的输入转换为 JPG，其余输入转换为 PNG
// forcePNG 为 true 且 compress 为 true 时忽略 compress，结果与 forcePNG 为 true 且 compress 为 false 相同
// forcePNG 为 false 且 compress 为 true 时进行 DXT 压缩，根据有无透明通道选择 DXT1 或 DXT5
// 生成版本 1011 的纹理图集需要图片目录中存在同名 .uv.csv 文件，例如 foo.png 对应 foo.png.uv.csv，文件内容每行保存一组 x、y、w、h
// 没有有效纹理图集矩形时生成版本 1010
// ConvertImageToTex converts any ImageMagick-supported file format to tex without writing it
// It depends on ImageMagick and requires the magick command to be directly available through PATH
// When forcePNG is true and compress is false, the tex data is the original PNG or a converted PNG
// When forcePNG and compress are both false, PNG or JPG input is used directly, otherwise lossy input without alpha becomes JPG and all other input becomes PNG
// When forcePNG and compress are both true, compress is ignored and the result matches forcePNG true with compress false
// When forcePNG is false and compress is true, DXT compression selects DXT1 or DXT5 according to alpha presence
// Producing a version-1011 texture atlas requires a sibling .uv.csv file such as foo.png.uv.csv for foo.png, with each row storing x, y, w, and h
// Version 1010 is produced when no valid texture-atlas rectangles are available
func ConvertImageToTex(inputPath string, texName string, compress bool, forcePNG bool) (*Tex, error) {
	// 检查 ImageMagick 是否安装
	// Check whether ImageMagick is installed
	err := tools.CheckMagick()
	if err != nil {
		return nil, err
	}

	// 尝试读取同名 .uv.csv 文件中的纹理图集矩形
	// Try to read texture-atlas rectangles from the sibling .uv.csv file
	var rects []TexRect
	rectsPath := inputPath + ".uv.csv"
	if data, err := os.ReadFile(rectsPath); err == nil {
		// 优先按逗号分隔读取，失败时回退到分号
		// Read with comma delimiters first and fall back to semicolons on failure
		reader := tools.NewCSVReaderSkipUTF8BOM(bytes.NewReader(data), 0)
		records, rErr := reader.ReadAll()
		if rErr != nil {
			reader2 := tools.NewCSVReaderSkipUTF8BOM(bytes.NewReader(data), ';')
			records, rErr = reader2.ReadAll()
		}
		if rErr == nil {
			for _, rec := range records {
				if len(rec) != 4 {
					continue
				}
				x, xErr := strconv.ParseFloat(strings.TrimSpace(rec[0]), 64)
				y, yErr := strconv.ParseFloat(strings.TrimSpace(rec[1]), 64)
				w, wErr := strconv.ParseFloat(strings.TrimSpace(rec[2]), 64)
				h, hErr := strconv.ParseFloat(strings.TrimSpace(rec[3]), 64)
				if xErr != nil || yErr != nil || wErr != nil || hErr != nil {
					continue
				}
				rects = append(rects, TexRect{
					X: float32(x),
					Y: float32(y),
					W: float32(w),
					H: float32(h),
				})
			}
		}
	}

	// 存在矩形时使用版本 1011，否则使用版本 1010
	// Use version 1011 when rectangles exist and version 1010 otherwise
	var version int32
	if len(rects) > 0 {
		version = 1011
	} else {
		version = 1010
		rects = nil
	}

	cmdIdentify := exec.Command("magick", "identify", "-format", "%wx%h %[channels] %[depth] %m", inputPath)
	tools.SetHideWindow(cmdIdentify)

	out, err := cmdIdentify.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to identify image: %w", err)
	}

	// 解析类似 512x768 rgba 8 JPEG 的 identify 输出
	// Parse identify output such as 512x768 rgba 8 JPEG
	parts := strings.SplitN(strings.TrimSpace(string(out)), " ", 4)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid identify output: %q", out)
	}

	// 获取 ImageMagick 报告的图像格式
	// Obtain the image format reported by ImageMagick
	var imageFormat string
	if len(parts) >= 4 {
		imageFormat = strings.ToUpper(parts[3])
	} else {
		// 无法获取格式时使用文件扩展名作为后备
		// Fall back to the file extension when the format is unavailable
		ext := strings.ToUpper(filepath.Ext(inputPath))
		if len(ext) > 0 {
			imageFormat = ext[1:]
		}
	}

	// 判断输入是否采用有损压缩格式
	// Determine whether the input uses a lossy compression format
	isLossyFormat := isLossyCompression(imageFormat)

	// 检查图像实际格式是否为 PNG 或 JPEG
	// Check whether the actual image format is PNG or JPEG
	isPNG := imageFormat == "PNG"
	isJPEG := imageFormat == "JPEG" || imageFormat == "JPG"

	// 解析图像宽度和高度
	// Parse the image width and height
	sizeParts := strings.Split(parts[0], "x")
	if len(sizeParts) != 2 {
		return nil, fmt.Errorf("invalid size format: %q", parts[0])
	}
	widthRaw, err := strconv.ParseInt(sizeParts[0], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid width: %w", err)
	}
	width := int32(widthRaw)
	heightRaw, err := strconv.ParseInt(sizeParts[1], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid height: %w", err)
	}
	height := int32(heightRaw)

	channels := strings.ToLower(parts[1])
	useAlpha := strings.Contains(channels, "a")

	// COM3D2 的 TextureResource.CreateTexture2D 仅支持 DXT5、DXT1、ARGB32 和 RGB24
	// DXT5 与 DXT1 载荷传给 LoadRawTextureData，ARGB32 与 RGB24 载荷传给 LoadImage
	// COM3D2 TextureResource.CreateTexture2D supports only DXT5, DXT1, ARGB32, and RGB24
	// DXT5 and DXT1 payloads go to LoadRawTextureData while ARGB32 and RGB24 payloads go to LoadImage
	var data []byte
	var textureFormat int32

	// 请求压缩且未强制 PNG 时转换为 DXT5 或 DXT1
	// Convert to DXT5 or DXT1 when compression is requested and PNG is not forced
	if compress && !forcePNG {
		// 使用内存管道接收 ImageMagick 输出
		// Use an in-memory pipe to receive ImageMagick output
		pr, pw := io.Pipe()

		// 在 goroutine 中执行转换并写入管道
		// Run the conversion in a goroutine and write it to the pipe
		go func() {
			dxtType := "dxt1"
			textureFormat = DXT1
			if useAlpha {
				dxtType = "dxt5"
				textureFormat = DXT5
			}

			// 让 ImageMagick 通过标准输出写出 DDS
			// Have ImageMagick write DDS through standard output
			cmd := exec.Command(
				"magick", inputPath,
				"-define", fmt.Sprintf("dds:compression=%s", dxtType),
				"dds:-",
			)
			tools.SetHideWindow(cmd)
			cmd.Stdout = pw

			err := cmd.Run()
			if err != nil {
				err = pw.CloseWithError(fmt.Errorf("failed to convert image to DDS: %w", err))
				if err != nil {
					return
				}
				return
			}

			// 正常关闭管道写端
			// Close the pipe writer normally
			pw.Close()
		}()

		// 从管道读取转换结果
		// Read the conversion result from the pipe
		data, err = io.ReadAll(pr)
		if err != nil {
			return nil, err
		}

		// DXT 结果剥离 128 字节 DDS 头部后转换为游戏使用的垂直块顺序
		// Strip the 128-byte DDS header from DXT output and convert it to the vertical block order used by the game
		if (textureFormat == DXT1 || textureFormat == DXT5) && len(data) > 128 {
			if string(data[:4]) == "DDS " {
				data = data[128:]
			}
			data, err = flipBlockCompressedTextureVertically(data, width, height, textureFormat)
			if err != nil {
				return nil, err
			}
		}
	} else {
		// forcePNG 为 true 时强制转换为 PNG
		// Force conversion to PNG when forcePNG is true
		if forcePNG {
			// 检查原始文件能否直接作为带透明通道的 PNG 使用
			// Check whether the source can be used directly as a PNG with alpha
			isDirectlyUsable := isPNG && useAlpha

			if isDirectlyUsable {
				// 直接读取原始文件以避免重复编码
				// Read the source file directly to avoid re-encoding
				data, err = os.ReadFile(inputPath)
				if err != nil {
					return nil, fmt.Errorf("failed to read image file: %w", err)
				}

				textureFormat = ARGB32
			} else {
				// 使用内存管道执行 PNG 转换
				// Use an in-memory pipe for PNG conversion
				pr, pw := io.Pipe()

				go func() {
					// 转换为保留透明通道的 PNG
					// Convert to PNG while retaining the alpha channel
					cmd := exec.Command("magick", inputPath, "png:-")
					tools.SetHideWindow(cmd)
					cmd.Stdout = pw
					err := cmd.Run()
					if err != nil {
						err := pw.CloseWithError(fmt.Errorf("failed to convert image to PNG: %w", err))
						if err != nil {
							return
						}
						return
					}
					// 正常关闭管道写端
					// Close the pipe writer normally
					pw.Close()
				}()

				// 从管道读取 PNG 数据
				// Read PNG data from the pipe
				data, err = io.ReadAll(pr)
				if err != nil {
					return nil, err
				}

				// PNG 数据使用 ARGB32 纹理格式
				// PNG data uses the ARGB32 texture format
				textureFormat = ARGB32
			}
		} else {
			// 检查原始 PNG 或 JPEG 能否直接使用
			// Check whether the source PNG or JPEG can be used directly
			isDirectlyUsable := (isPNG && useAlpha) || (isJPEG && !useAlpha)

			if isDirectlyUsable {
				// 直接读取原始文件以避免重复编码
				// Read the source file directly to avoid re-encoding
				data, err = os.ReadFile(inputPath)
				if err != nil {
					return nil, fmt.Errorf("failed to read image file: %w", err)
				}

				// 根据直接使用的图像格式设置纹理格式
				// Set the texture format from the directly used image format
				if isPNG {
					textureFormat = ARGB32
				} else {
					textureFormat = RGB24
				}
			} else {
				// 使用内存管道转换不兼容的输入格式
				// Use an in-memory pipe to convert an incompatible input format
				pr, pw := io.Pipe()

				go func() {
					var cmd *exec.Cmd

					if useAlpha || !isLossyFormat {
						// 有透明通道或输入无损时转换为 PNG
						// Convert to PNG when alpha is present or the input is lossless
						cmd = exec.Command("magick", inputPath, "png:-")
						tools.SetHideWindow(cmd)
						textureFormat = ARGB32
					} else {
						// 无透明通道的有损输入转换为 JPEG
						// Convert lossy input without alpha to JPEG
						quality := "90"
						cmd = exec.Command("magick", inputPath, "-quality", quality, "jpg:-")
						tools.SetHideWindow(cmd)
						textureFormat = RGB24
					}

					cmd.Stdout = pw
					err := cmd.Run()
					if err != nil {
						err = pw.CloseWithError(fmt.Errorf("failed to convert image: %w", err))
						if err != nil {
							return
						}
						return
					}

					// 正常关闭管道写端
					// Close the pipe writer normally
					pw.Close()
				}()

				// 从管道读取转换结果
				// Read the conversion result from the pipe
				data, err = io.ReadAll(pr)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	// 组装最终 Tex 结构
	// Assemble the final Tex value
	tex := &Tex{
		Signature:     "CM3D2_TEX",
		Version:       version,
		TextureName:   texName,
		Rects:         rects,
		Width:         width,
		Height:        height,
		TextureFormat: textureFormat,
		Data:          data,
	}

	return tex, nil
}

// ConvertImageToTexAndWrite 将任意 ImageMagick 支持的文件格式转换为 tex 格式并写出
// 转换规则与 ConvertImageToTex 相同，包括 forcePNG、compress 和同名 .uv.csv 的处理
// ConvertImageToTexAndWrite converts any ImageMagick-supported file format to tex and writes it
// It uses the same forcePNG, compress, and sibling .uv.csv behavior as ConvertImageToTex
func ConvertImageToTexAndWrite(inputPath string, texName string, compress bool, forcePNG bool, outputPath string) error {
	tex, err := ConvertImageToTex(inputPath, texName, compress, forcePNG)
	if err != nil {
		return fmt.Errorf("failed to convert image to tex: %w", err)
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("unable to create .tex file: %w", err)
	}
	defer f.Close()

	bw := bufio.NewWriter(f)
	if err := tex.Dump(bw); err != nil {
		return fmt.Errorf("failed to write to .tex file: %w", err)
	}
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("an error occurred while flush bufio: %w", err)
	}
	return nil
}

// ConvertTexToImage 将 Tex 数据转换为图像数据但不写出
// 依赖外部库 ImageMagick，并要求 PATH 环境变量可以直接调用 magick 命令
// forcePNG 为 false 时，PNG 或 JPG 图像载荷可直接返回，否则根据透明通道输出 JPG 或 PNG
// forcePNG 为 true 时不考虑原始格式和透明通道并强制输出 PNG
// 版本 1011 的纹理图集还会返回 rects
// ConvertTexToImage converts Tex data to image data without writing it
// It depends on ImageMagick and requires the magick command to be directly available through PATH
// When forcePNG is false, PNG or JPG image payloads may be returned directly, otherwise the output is JPG or PNG according to alpha presence
// When forcePNG is true, PNG output is forced regardless of the original format and alpha presence
// Version-1011 texture atlases also return rects
func ConvertTexToImage(tex *Tex, forcePNG bool) (imgData []byte, format string, rects []TexRect, err error) {
	if tex.Version == 1011 {
		rects = tex.Rects
	}

	// 检查 ImageMagick 是否安装
	// Check whether ImageMagick is installed
	if err := tools.CheckMagick(); err != nil {
		return nil, "", nil, err
	}

	// 根据 TextureFormat 判断输入格式和透明通道
	// Determine the input format and alpha presence from TextureFormat
	var inputFormat string
	var hasAlpha bool

	switch tex.TextureFormat {
	case DXT1:
		inputFormat = "dds"
		hasAlpha = false
	case DXT5:
		inputFormat = "dds"
		hasAlpha = true
	case ARGB32, RGB24, 0:
		// 优先从数据魔数检测实际格式
		// Prefer detecting the actual format from the data signature
		if len(tex.Data) >= 8 && bytes.Equal(tex.Data[:8], []byte("\x89PNG\r\n\x1a\n")) {
			inputFormat = "png"
			hasAlpha = true
		} else if len(tex.Data) >= 3 && bytes.Equal(tex.Data[:3], []byte("\xff\xd8\xff")) {
			inputFormat = "jpg"
			hasAlpha = false
		} else {
			// 检测失败时回退到 TextureFormat 对应的默认格式
			// Fall back to the default format associated with TextureFormat when detection fails
			if tex.TextureFormat == RGB24 {
				inputFormat = "jpg"
				hasAlpha = false
			} else {
				inputFormat = "png"
				hasAlpha = true
			}
		}
	default:
		return nil, inputFormat, nil, fmt.Errorf("unsupported texture format: %d", tex.TextureFormat)
	}

	// ARGB32 以及未强制 PNG 的 RGB24 可直接返回原始编码数据
	// ARGB32 and RGB24 without forced PNG can return the original encoded data directly
	skipConversion := false
	if tex.TextureFormat == ARGB32 || (tex.TextureFormat == RGB24 && !forcePNG) {
		skipConversion = true
	}

	if skipConversion {
		return tex.Data, inputFormat, rects, nil
	}
	// forcePNG 为 true 时使用 ImageMagick 强制输出 PNG
	// Use ImageMagick to force PNG output when forcePNG is true

	if forcePNG {
		cmd := exec.Command("magick", inputFormat+":-", "png:-")
		tools.SetHideWindow(cmd)

		var stderrBuf bytes.Buffer
		cmd.Stderr = &stderrBuf

		d := tex.Data
		if tex.TextureFormat == DXT1 || tex.TextureFormat == DXT5 {
			d, err = flipBlockCompressedTextureVertically(d, tex.Width, tex.Height, tex.TextureFormat)
			if err != nil {
				return nil, "", nil, err
			}
			d = ensureDDSHeader(d, tex.Width, tex.Height, tex.TextureFormat)
		}
		cmd.Stdin = bytes.NewReader(d)

		// 从标准输出读取转换后的 PNG 数据
		// Read converted PNG data from standard output
		outPipe, err := cmd.StdoutPipe()
		if err != nil {
			return nil, "", nil, fmt.Errorf("failed to get stdout pipe: %w", err)
		}
		if err = cmd.Start(); err != nil {
			return nil, "", nil, fmt.Errorf("failed to start magick command: %w", err)
		}

		convertedBytes, err := io.ReadAll(outPipe)
		if err != nil {
			_ = cmd.Wait()
			return nil, "", nil, fmt.Errorf("failed to read converted data: %w, stderr: %s", err, stderrBuf.String())
		}
		if err = cmd.Wait(); err != nil {
			return nil, "", nil, fmt.Errorf("magick command error: %w, stderr: %s", err, stderrBuf.String())
		}

		return convertedBytes, "png", rects, nil
	}

	var args []string
	if hasAlpha {
		args = []string{inputFormat + ":-", "png:-"}
		format = "png"
	} else {
		args = []string{inputFormat + ":-", "jpg:-", "-quality", "90"}
		format = "jpg"
	}

	// 无透明通道时输出 JPEG，否则输出 PNG
	// Output JPEG without alpha and PNG otherwise
	cmd := exec.Command("magick", args...)
	tools.SetHideWindow(cmd)

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	d := tex.Data
	if tex.TextureFormat == DXT1 || tex.TextureFormat == DXT5 {
		d, err = flipBlockCompressedTextureVertically(d, tex.Width, tex.Height, tex.TextureFormat)
		if err != nil {
			return nil, "", nil, err
		}
		d = ensureDDSHeader(d, tex.Width, tex.Height, tex.TextureFormat)
	}
	cmd.Stdin = bytes.NewReader(d)

	outPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	if err = cmd.Start(); err != nil {
		return nil, "", nil, fmt.Errorf("failed to start magick command: %w", err)
	}

	convertedBytes, err := io.ReadAll(outPipe)
	if err != nil {
		_ = cmd.Wait()
		return nil, "", nil, fmt.Errorf("failed to read converted data: %w, stderr: %s", err, stderrBuf.String())
	}
	if err = cmd.Wait(); err != nil {
		return nil, "", nil, fmt.Errorf("magick command error: %w, stderr: %s", err, stderrBuf.String())
	}

	return convertedBytes, format, rects, nil
}

// ConvertTexToImageAndWrite 将 .tex 文件转换为图像文件并写出
// 依赖外部库 ImageMagick，并要求 PATH 环境变量可以直接调用 magick 命令
// forcePNG 为 false 时根据输出路径后缀决定格式，没有后缀时有损且无透明通道的数据保存为 JPG，其余保存为 PNG
// forcePNG 为 true 时不考虑原始格式和透明通道并强制保存为 PNG
// 版本 1011 的纹理图集还会生成同名 .uv.csv 文件，每行保存一组 x、y、w、h
// 输出路径使用 .tex 后缀时原样写出 Tex
// ConvertTexToImageAndWrite converts a .tex file to an image file and writes it
// It depends on ImageMagick and requires the magick command to be directly available through PATH
// When forcePNG is false, the output suffix selects the format, while a missing suffix uses JPG for lossy data without alpha and PNG otherwise
// When forcePNG is true, PNG is forced regardless of the original format and alpha presence
// Version-1011 texture atlases also produce a sibling .uv.csv file with one x, y, w, h group per row
// An output path ending in .tex writes the Tex unchanged
func ConvertTexToImageAndWrite(tex *Tex, outputPath string, forcePNG bool) error {
	// 输出为 .tex 时直接写出原始 Tex
	// Write the original Tex directly when the output is .tex
	if strings.HasSuffix(strings.ToLower(outputPath), ".tex") {
		f, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("unable to create .tex file: %w", err)
		}
		defer f.Close()
		bw := bufio.NewWriter(f)
		if err := tex.Dump(bw); err != nil {
			return fmt.Errorf("failed to write to.tex file: %w", err)
		}
		if err := bw.Flush(); err != nil {
			return fmt.Errorf("an error occurred while flush bufio: %w", err)
		}
	}

	// 检查 ImageMagick 是否安装
	// Check whether ImageMagick is installed
	if err := tools.CheckMagick(); err != nil {
		return err
	}

	// 根据 TextureFormat 判断输入格式和透明通道
	// Determine the input format and alpha presence from TextureFormat
	var inputFormat string
	var hasAlpha bool

	switch tex.TextureFormat {
	case DXT1:
		inputFormat = "dds"
		hasAlpha = false
	case DXT5:
		inputFormat = "dds"
		hasAlpha = true
	case ARGB32, RGB24, 0:
		// 优先从数据魔数检测实际格式
		// Prefer detecting the actual format from the data signature
		if len(tex.Data) >= 8 && bytes.Equal(tex.Data[:8], []byte("\x89PNG\r\n\x1a\n")) {
			inputFormat = "png"
			hasAlpha = true
		} else if len(tex.Data) >= 3 && bytes.Equal(tex.Data[:3], []byte("\xff\xd8\xff")) {
			inputFormat = "jpg"
			hasAlpha = false
		} else {
			// 检测失败时回退到 TextureFormat 对应的默认格式
			// Fall back to the default format associated with TextureFormat when detection fails
			if tex.TextureFormat == RGB24 {
				inputFormat = "jpg"
				hasAlpha = false
			} else {
				inputFormat = "png"
				hasAlpha = true
			}
		}
	default:
		return fmt.Errorf("unsupported texture format: %d", tex.TextureFormat)
	}

	// forcePNG 为 true 时将输出后缀改为 .png
	// Change the output suffix to .png when forcePNG is true
	if forcePNG {
		outputPath = strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".png"
	}

	// 未指定后缀时根据透明通道选择 PNG 或 JPG
	// Choose PNG or JPG from alpha presence when no suffix is specified
	ext := filepath.Ext(outputPath)
	if ext == "" {
		if forcePNG {
			outputPath += ".png"
		} else {
			if hasAlpha {
				outputPath += ".png"
			} else {
				outputPath += ".jpg"
			}
		}
	}

	// 扩展名为 .tex 时直接写出 Tex
	// Write Tex directly when the extension is .tex
	if ext == ".tex" {
		f, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("unable to create.tex file: %w", err)
		}
		defer f.Close()
		bw := bufio.NewWriter(f)
		if err := tex.Dump(bw); err != nil {
			return fmt.Errorf("failed to write to.tex file: %w", err)
		}
		if err := bw.Flush(); err != nil {
			return fmt.Errorf("an error occurred while flush bufio: %w", err)
		}
		return nil
	}

	// ARGB32 以及未强制 PNG 的 RGB24 可跳过重新编码
	// ARGB32 and RGB24 without forced PNG can skip re-encoding
	skipConversion := false
	if tex.TextureFormat == ARGB32 || (tex.TextureFormat == RGB24 && !forcePNG) {
		skipConversion = true
	}

	// 原始数据为 PNG 或 JPG 时优先直接写出以避免质量损失
	// Prefer writing original PNG or JPG data directly to avoid quality loss
	if skipConversion {
		if ext == ".png" || ext == ".jpg" {
			if err := os.WriteFile(outputPath, tex.Data, 0755); err != nil {
				return fmt.Errorf("failed to write file directly: %w", err)
			}
		}
		// 目标后缀不匹配时用 ImageMagick 转换为用户指定格式
		// Use ImageMagick when the target suffix requests another format
		cmd := exec.Command("magick", inputFormat+":-", outputPath)
		tools.SetHideWindow(cmd)

		d := tex.Data
		if tex.TextureFormat == DXT1 || tex.TextureFormat == DXT5 {
			flipped, err := flipBlockCompressedTextureVertically(d, tex.Width, tex.Height, tex.TextureFormat)
			if err != nil {
				return err
			}
			d = flipped
			d = ensureDDSHeader(d, tex.Width, tex.Height, tex.TextureFormat)
		}
		cmd.Stdin = bytes.NewReader(d)

		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to convert image: %w, output: %s", err, string(output))
		}
	} else {
		// 需要转换时由 ImageMagick 直接写入目标文件
		// Have ImageMagick write the target file when conversion is required
		var args []string
		if strings.HasSuffix(strings.ToLower(outputPath), ".jpg") {
			args = []string{inputFormat + ":-", "-quality", "90", outputPath}
		} else {
			args = []string{inputFormat + ":-", outputPath}
		}

		cmd := exec.Command("magick", args...)
		tools.SetHideWindow(cmd)

		d := tex.Data
		if tex.TextureFormat == DXT1 || tex.TextureFormat == DXT5 {
			flipped, err := flipBlockCompressedTextureVertically(d, tex.Width, tex.Height, tex.TextureFormat)
			if err != nil {
				return err
			}
			d = flipped
			d = ensureDDSHeader(d, tex.Width, tex.Height, tex.TextureFormat)
		}
		cmd.Stdin = bytes.NewReader(d)

		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to convert image: %w, output: %s", err, string(output))
		}
	}

	// 存在纹理图集矩形时把 UV 信息写入同名 CSV
	// Write atlas UV data to a sibling CSV when rectangles are present
	if len(tex.Rects) > 0 {
		uvFilePath := outputPath + ".uv.csv"
		file, err := os.Create(uvFilePath)
		if err != nil {
			return fmt.Errorf("failed to create UV file: %w", err)
		}
		defer file.Close()

		records := make([][]string, 0, len(tex.Rects)+1)
		// 写入 CSV 表头
		// Write the CSV header
		records = append(records, []string{"X", "Y", "W", "H"})
		for _, rect := range tex.Rects {
			records = append(records, []string{
				fmt.Sprintf("%.6f", rect.X),
				fmt.Sprintf("%.6f", rect.Y),
				fmt.Sprintf("%.6f", rect.W),
				fmt.Sprintf("%.6f", rect.H),
			})
		}
		if err := tools.WriteCSVWithUTF8BOM(file, records); err != nil {
			return fmt.Errorf("failed to write UV data: %w", err)
		}
	}
	return nil
}

// createDDSHeader 为 DXT1 或 DXT5 创建一个基本的 128 字节 DDS 头部
// createDDSHeader creates a basic 128-byte DDS header for DXT1 or DXT5
func createDDSHeader(width, height int32, format int32) []byte {
	buf := make([]byte, 128)
	copy(buf[0:4], "DDS ")
	// 写入 DDS 头部大小
	// Write the DDS header size
	binary.LittleEndian.PutUint32(buf[4:8], 124)

	// 标志包含 DDSD_CAPS、DDSD_HEIGHT、DDSD_WIDTH、DDSD_PIXELFORMAT 和 DDSD_LINEARSIZE
	// Flags include DDSD_CAPS, DDSD_HEIGHT, DDSD_WIDTH, DDSD_PIXELFORMAT, and DDSD_LINEARSIZE
	flags := uint32(0x1 | 0x2 | 0x4 | 0x1000 | 0x80000)
	binary.LittleEndian.PutUint32(buf[8:12], flags)

	binary.LittleEndian.PutUint32(buf[12:16], uint32(height))
	binary.LittleEndian.PutUint32(buf[16:20], uint32(width))

	// 根据块数量计算 PitchOrLinearSize
	// Compute PitchOrLinearSize from the block count
	var blockSize uint32 = 8
	if format == DXT5 {
		blockSize = 16
	}
	linearSize := uint32((width+3)/4) * uint32((height+3)/4) * blockSize
	binary.LittleEndian.PutUint32(buf[20:24], linearSize)

	// 二维纹理的 Depth 为零
	// Depth is zero for a two-dimensional texture
	binary.LittleEndian.PutUint32(buf[24:28], 0)
	// 当前生成器只写一个 MipMap 层级
	// The current generator writes one MipMap level
	binary.LittleEndian.PutUint32(buf[28:32], 1)

	// 写入像素格式结构
	// Write the pixel-format structure
	var pfOff int64 = 76
	// 像素格式结构大小为 32 字节
	// The pixel-format structure size is 32 bytes
	binary.LittleEndian.PutUint32(buf[pfOff:pfOff+4], 32)
	// DDPF_FOURCC 表示格式由四字符代码指定
	// DDPF_FOURCC means the format is identified by a four-character code
	binary.LittleEndian.PutUint32(buf[pfOff+4:pfOff+8], 0x4)
	if format == DXT1 {
		copy(buf[pfOff+8:pfOff+12], "DXT1")
	} else {
		copy(buf[pfOff+8:pfOff+12], "DXT5")
	}

	// Caps 使用 DDSCAPS_TEXTURE
	// Caps uses DDSCAPS_TEXTURE
	binary.LittleEndian.PutUint32(buf[108:112], 0x1000)

	return buf
}

// ensureDDSHeader 确保 DXT 数据具有 DDS 头部
// 数据已经有 DDS 签名时按原样返回，否则根据宽度、高度和格式合成头部
// ensureDDSHeader ensures that DXT data has a DDS header
// Data with an existing DDS signature is returned unchanged, otherwise a header is synthesized from width, height, and format
func ensureDDSHeader(data []byte, width, height int32, format int32) []byte {
	if len(data) >= 4 && string(data[:4]) == "DDS " {
		return data
	}
	header := createDDSHeader(width, height, format)
	return append(header, data...)
}

// flipBlockCompressedTextureVertically 在 COM3D2 原始 DXT 块顺序与标准 DDS 块顺序之间进行垂直翻转
// 此变换是自身的逆运算，因此 tex 转图像和图像转 tex 共用同一函数，Tex.Data 中的 DXT1 与 DXT5 块相对标准 DDS 采用垂直翻转顺序
// flipBlockCompressedTextureVertically converts between COM3D2 raw DXT block order and standard DDS block order by flipping vertically
// The transform is its own inverse, so tex-to-image and image-to-tex conversion share this function, with DXT1 and DXT5 blocks in Tex.Data vertically reversed relative to standard DDS
func flipBlockCompressedTextureVertically(data []byte, width, height int32, format int32) ([]byte, error) {
	if format != DXT1 && format != DXT5 {
		return data, nil
	}
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid DXT dimensions: %dx%d", width, height)
	}

	var blockSize int64 = 8
	if format == DXT5 {
		blockSize = 16
	}

	blocksWide := (int64(width) + 3) / 4
	blocksHigh := (int64(height) + 3) / 4
	expectedLen := blocksWide * blocksHigh * blockSize
	if int64(len(data)) != expectedLen {
		return nil, fmt.Errorf("unexpected DXT data size: got %d bytes, want %d for %dx%d format %d", len(data), expectedLen, width, height, format)
	}

	flipped := make([]byte, len(data))
	rowStride := blocksWide * blockSize
	for blockY := int64(0); blockY < blocksHigh; blockY++ {
		srcRowOffset := blockY * rowStride
		dstRowOffset := (blocksHigh - 1 - blockY) * rowStride
		for blockX := int64(0); blockX < blocksWide; blockX++ {
			srcBlockStart := srcRowOffset + blockX*blockSize
			dstBlockStart := dstRowOffset + blockX*blockSize
			srcBlock := data[srcBlockStart : srcBlockStart+blockSize]
			dstBlock := flipped[dstBlockStart : dstBlockStart+blockSize]
			if format == DXT1 {
				flipDXT1Block(dstBlock, srcBlock)
			} else {
				flipDXT5Block(dstBlock, srcBlock)
			}
		}
	}

	return flipped, nil
}

// flipDXT1Block 垂直翻转一个 DXT1 块中的四行颜色索引
// flipDXT1Block vertically flips the four color-index rows in one DXT1 block
func flipDXT1Block(dst []byte, src []byte) {
	copy(dst[:4], src[:4])
	dst[4] = src[7]
	dst[5] = src[6]
	dst[6] = src[5]
	dst[7] = src[4]
}

// flipDXT5Block 垂直翻转一个 DXT5 块中的 Alpha 索引行和颜色索引行
// flipDXT5Block vertically flips the alpha-index and color-index rows in one DXT5 block
func flipDXT5Block(dst []byte, src []byte) {
	copy(dst[:2], src[:2])

	var alphaBits uint64
	for i := int64(0); i < 6; i++ {
		alphaBits |= uint64(src[2+i]) << (8 * i)
	}

	var rows [4]uint16
	for row := int64(0); row < 4; row++ {
		rows[row] = uint16((alphaBits >> (12 * row)) & 0x0FFF)
	}

	flippedAlphaBits := uint64(rows[3]) |
		(uint64(rows[2]) << 12) |
		(uint64(rows[1]) << 24) |
		(uint64(rows[0]) << 36)
	for i := int64(0); i < 6; i++ {
		dst[2+i] = byte(flippedAlphaBits >> (8 * i))
	}

	copy(dst[8:12], src[8:12])
	dst[12] = src[15]
	dst[13] = src[14]
	dst[14] = src[13]
	dst[15] = src[12]
}

// isLossyCompression 检查 ImageMagick 输出的格式名是否属于有损压缩格式，例如 JPEG
// isLossyCompression checks whether an ImageMagick output format name is a lossy compression format such as JPEG
func isLossyCompression(format string) bool {
	// 下列格式来自 magick -list format 中常见的有损图像格式
	// The following entries are common lossy image formats from magick -list format
	lossyFormats := map[string]bool{
		"JPEG":  true,
		"JPG":   true,
		"PJPEG": true,
		"JPS":   true,
		"MPO":   true,
		"JXL":   true,
		"AVIF":  true,
		"HEIC":  true,
		"HEIF":  true,

		"WDP": true,
		"HDP": true,
		"JNG": true,

		"JP2": true,
		"J2C": true,
		"J2K": true,
		"JPC": true,
		"MJ2": true,

		"PCD": true,
	}

	// WebP 需要进一步检查编码模式，因此这里默认按无损格式处理
	// WebP requires inspecting its encoding mode, so it is treated as lossless here

	return lossyFormats[format]
}
