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

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/tools"
)

// CM3D2_TEX
// 纹理文件
//
// CM3D2 支持 1000 - 1010 版本
// COM3D2 支持 1000 - 1011 版本
//
// 1000 版本
// 没有显式的宽高字段
// 宽高存储在图像数据头的固定位置（16-23 字节）
//
// 1010 版本
// 增加显式的宽高和纹理格式字段
// 支持 DXT5/DXT1
// 格式为 DXT5/DXT1 时不含 DDS 文件头，而只包含原始像素块
//
// 1011 版本
// 新增矩形数组（用于纹理图集）
// 每个矩形包含(x, y, width, height)
//
// 注意，TextureFormat 为 ARGB32、RGB24 时数据位是 PNG 或 JPG 格式，为 DXT5、DXT1 时数据位是 DDS 格式
// 本序列化不支持写出 1000 版本
// 部分错误的 .tex 文件虽然使用 RGB24 格式，但嵌入的却是 PNG 数据。因此本程序会在格式为 RGB24 和 ARGB32 时尝试解析数据魔数以确定真实格式

// From unity 5.6.4
// COM3D2 supported only
const (
	RGB24  int32 = 3
	ARGB32 int32 = 5
	DXT1   int32 = 10
	DXT5   int32 = 12
)

type TexRect struct {
	X float32 `json:"X"`
	Y float32 `json:"Y"`
	W float32 `json:"W"`
	H float32 `json:"H"`
}

type Tex struct {
	Signature     string    `json:"Signature"`     // 一般是 "CM3D2_TEX"
	Version       int32     `json:"Version"`       // 版本号
	TextureName   string    `json:"TextureName"`   // 纹理文件名
	Rects         []TexRect `json:"Rects"`         // 如果版本 >= 1011 才会有，纹理图集使用
	Width         int32     `json:"Width"`         // 版本 >= 1010 才会有，否则可能需要从 data 解析
	Height        int32     `json:"Height"`        // 版本 >= 1010 才会有，否则可能需要从 data 解析
	TextureFormat int32     `json:"TextureFormat"` // 读取到的原始格式枚举，Go 参考顶部常量
	Data          []byte    `json:"Data"`          // DDS/Bitmap 原始二进制数据
}

// ReadTex 从二进制流中读取 Tex 数据。
// 需要数据流是 .tex 格式
func ReadTex(r io.Reader) (*Tex, error) {
	reader := stream.NewBinaryReader(r)

	// 1. Signature
	sig, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read .tex signature failed: %w", err)
	}
	//if sig != TexSignature {
	//	return nil, fmt.Errorf("invalid .tex signature: got %q, want %v", sig, TexSignature)
	//}

	// 2. Version
	ver, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read .tex version failed: %w", err)
	}

	// 3. TextureName
	texName, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read .tex textureName failed: %w", err)
	}

	// 4. 如果 version >= 1011，读取 rects
	var rects []TexRect
	if ver >= 1011 {
		rectCount, err := reader.ReadInt32()
		if err != nil {
			return nil, fmt.Errorf("read .tex rectCount failed: %w", err)
		}
		if rectCount > 0 {
			rects = make([]TexRect, rectCount)
			for i := 0; i < int(rectCount); i++ {
				x, err := reader.ReadFloat32()
				if err != nil {
					return nil, err
				}
				y, err := reader.ReadFloat32()
				if err != nil {
					return nil, err
				}
				w, err := reader.ReadFloat32()
				if err != nil {
					return nil, err
				}
				h, err := reader.ReadFloat32()
				if err != nil {
					return nil, err
				}
				rects[i] = TexRect{x, y, w, h}
			}
		}
	}

	// 5. 如果 version >= 1010，读取 width, height, textureFormat
	var width, height, texFmt int32
	if ver >= 1010 {
		w, err := reader.ReadInt32()
		if err != nil {
			return nil, err
		}
		h, err := reader.ReadInt32()
		if err != nil {
			return nil, err
		}
		f, err := reader.ReadInt32()
		if err != nil {
			return nil, err
		}
		width, height, texFmt = w, h, f
	}

	// 6. 读取 dataLength
	dataLen, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read .tex dataLength failed: %w", err)
	}

	// 7. 读取数据块
	data := make([]byte, dataLen)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, fmt.Errorf("read .tex raw data failed: %w", err)
	}

	// 8. 如果 version == 1000，需要从 data 头解析 width / height
	if ver == 1000 {
		if len(data) < 24 {
			return nil, fmt.Errorf(".tex data too short for version=1000")
		}
		// C# 示例：data[16..19] 存储宽度(小端序), data[20..23] 存储高度(小端序)
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

// Dump 将 Tex 数据写入二进制流。
// 输出的数据流是 .tex 格式
func (t *Tex) Dump(w io.Writer) error {
	writer := stream.NewBinaryWriter(w)

	// 1. Signature
	if err := writer.WriteString(t.Signature); err != nil {
		return fmt.Errorf("write signature failed: %w", err)
	}
	// 2. Version
	if err := writer.WriteInt32(t.Version); err != nil {
		return fmt.Errorf("write version failed: %w", err)
	}
	// 3. TextureName
	if err := writer.WriteString(t.TextureName); err != nil {
		return fmt.Errorf("write textureName failed: %w", err)
	}

	if t.Version == 1000 {
		return fmt.Errorf("dump version 1000 is not supported, You should at least convert it to 1010 version, " +
			"maybe you can convert it to image and convert back to .tex")
	}

	// 4. 如果 version >= 1011, 写出 rects
	if t.Version >= 1011 {
		rectCount := int32(len(t.Rects))
		if err := writer.WriteInt32(rectCount); err != nil {
			return fmt.Errorf("write rectCount failed: %w", err)
		}
		for _, rect := range t.Rects {
			if err := writer.WriteFloat32(rect.X); err != nil {
				return err
			}
			if err := writer.WriteFloat32(rect.Y); err != nil {
				return err
			}
			if err := writer.WriteFloat32(rect.W); err != nil {
				return err
			}
			if err := writer.WriteFloat32(rect.H); err != nil {
				return err
			}
		}
	}

	// 5. 如果 version >= 1010, 写出 width, height, textureFormat
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

	// 6. 写出 dataLength
	dataLen := int32(len(t.Data))
	if err := writer.WriteInt32(dataLen); err != nil {
		return fmt.Errorf("write dataLen failed: %w", err)
	}
	// 7. 写出 data
	if _, err := w.Write(t.Data); err != nil {
		return fmt.Errorf("write data block failed: %w", err)
	}

	return nil
}

// ConvertImageToTex 将任意 ImageMagick 支持的文件格式转换为 tex 格式，但不写出
// 依赖外部库 ImageMagick，且有 Path 环境变量可以直接调用 magick 命令
// 如果 forcePNG 为 true，且 compress 为 false，则 tex 的数据位是原始 PNG 数据或转换为 PNG
// 如果 forcePNG 为 false，且 compress 为 false，那么检查输入格式是否是 PNG 或 JPG，如果是则数据位直接使用原始图片，否则如果原始格式有损且无透明通道则转换为 JPG，否则转换为 PNG
// 如果 forcePNG 为 true，且 compress 为 true，那么 compress 标识会被忽略，结果同 forcePNG 为 true，且 compress 为 false
// 如果 forcePNG 为 false，且 compress 为 true，那么会对结果进行 DXT 压缩，数据位为 DDS 数据，根据有无透明通道选择 DXT1 或 DXT5
// 如果要生成 1011 版本的 tex（纹理图集），需要在图片目录下有一个同名的 .uv.csv 文件（例如 foo.png 对应 foo.png.uv.csv），文件内容为矩形数组 x, y, w, h 一行一组
// 否则生成 1010 版本的 tex
func ConvertImageToTex(inputPath string, texName string, compress bool, forcePNG bool) (*Tex, error) {
	// 1. 检查 ImageMagick 是否安装
	err := tools.CheckMagick()
	if err != nil {
		return nil, err
	}

	// 2.尝试读取 .uv.csv 文件（纹理图集）
	var rects []TexRect
	rectsPath := inputPath + ".uv.csv"
	if data, err := os.ReadFile(rectsPath); err == nil {
		// 优先按逗号分隔读取，失败则回退到分号
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

	var version int32
	// 如果有 rects 则设置版本为 1011
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

	// 解析输出结果（格式示例："512x768 rgba 8 JPEG"）
	parts := strings.SplitN(strings.TrimSpace(string(out)), " ", 4)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid identify output: %q", out)
	}

	// 获取图像格式（如果可用）
	var imageFormat string
	if len(parts) >= 4 {
		imageFormat = strings.ToUpper(parts[3])
	} else {
		// 如果无法获取格式，使用文件扩展名作为后备方案
		ext := strings.ToUpper(filepath.Ext(inputPath))
		if len(ext) > 0 {
			imageFormat = ext[1:] // 去掉点号
		}
	}

	// 判断是否为有损压缩格式
	isLossyFormat := isLossyCompression(imageFormat)

	// 检查图像实际格式是否为PNG或JPG/JPEG
	isPNG := imageFormat == "PNG"
	isJPEG := imageFormat == "JPEG" || imageFormat == "JPG"

	// 解析宽高
	sizeParts := strings.Split(parts[0], "x")
	if len(sizeParts) != 2 {
		return nil, fmt.Errorf("invalid size format: %q", parts[0])
	}
	width, err := strconv.Atoi(sizeParts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid width: %w", err)
	}
	height, err := strconv.Atoi(sizeParts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid height: %w", err)
	}

	channels := strings.ToLower(parts[1])
	useAlpha := strings.Contains(channels, "a")

	// 4. 生成图片数据位
	// COM3D2 2.42.0 只支持 DXT5、DXT1、ARGB32、RGB24, 见  Texture2D CreateTexture2D()
	// DXT5、DXT1 时数据位是 DDS，因为调用的 texture2D.LoadRawTextureData
	// ARGB32、RGB24 时数据位是 PNG 或 JPG，因为调用的 texture2D2.LoadImage
	var data []byte
	var textureFormat int32

	// 如果需要压缩，则转换为 DXT5 或 DXT1
	if compress && !forcePNG {
		// 使用内存管道
		pr, pw := io.Pipe()

		// 创建一个goroutine来执行转换并写入管道
		go func() {
			dxtType := "dxt1"
			textureFormat = DXT1
			if useAlpha {
				dxtType = "dxt5"
				textureFormat = DXT5
			}

			// 使用stdout将输出直接写入管道
			cmd := exec.Command(
				"magick", inputPath,
				"-define", fmt.Sprintf("dds:compression=%s", dxtType),
				"dds:-", // 输出到stdout
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

			pw.Close() // 正常关闭
		}()

		// 从管道读取数据
		data, err = io.ReadAll(pr)
		if err != nil {
			return nil, err
		}

		// 如果是 DXT 压缩，则剥离 DDS 头部 (128 字节)
		if (textureFormat == DXT1 || textureFormat == DXT5) && len(data) > 128 {
			if string(data[:4]) == "DDS " {
				data = data[128:]
			}
		}
	} else {
		// forcePNG 为 true 时，强制转换为 PNG
		if forcePNG {
			// 检查是否可以直接使用原始文件
			isDirectlyUsable := isPNG && useAlpha

			if isDirectlyUsable {
				// 直接读取原始文件
				data, err = os.ReadFile(inputPath)
				if err != nil {
					return nil, fmt.Errorf("failed to read image file: %w", err)
				}

				textureFormat = ARGB32
			} else { // 需要转换
				// 使用管道处理转换
				pr, pw := io.Pipe()

				go func() {
					// 转换为PNG格式，保留alpha通道
					cmd := exec.Command("magick", inputPath, "png:-")
					tools.SetHideWindow(cmd)
					cmd.Stdout = pw
					err := cmd.Run()
					if err != nil {
						pw.CloseWithError(fmt.Errorf("failed to convert image to PNG: %w", err))
						return
					}
					pw.Close()
				}()

				// 从管道读取数据
				data, err = io.ReadAll(pr)
				if err != nil {
					return nil, err
				}

				// 设置纹理格式为ARGB32（PNG格式）
				textureFormat = ARGB32
			}
		} else {
			// 检查是否可以直接使用原始文件
			isDirectlyUsable := (isPNG && useAlpha) || (isJPEG && !useAlpha)

			if isDirectlyUsable {
				// 直接读取原始文件
				data, err = os.ReadFile(inputPath)
				if err != nil {
					return nil, fmt.Errorf("failed to read image file: %w", err)
				}

				// 设置纹理格式
				if isPNG {
					textureFormat = ARGB32
				} else {
					textureFormat = RGB24
				}
			} else {
				// 需要转换
				pr, pw := io.Pipe()

				go func() {
					var cmd *exec.Cmd

					if useAlpha || !isLossyFormat {
						// 转换为PNG
						cmd = exec.Command("magick", inputPath, "png:-")
						tools.SetHideWindow(cmd)
						textureFormat = ARGB32
					} else {
						// 转换为JPG
						quality := "90"
						//if isLossyFormat {
						//	quality = "85" // 对已经有损的图像使用稍低的质量
						//}
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

					pw.Close() // 正常关闭
				}()

				// 从管道读取数据
				data, err = io.ReadAll(pr)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	// 6. 组装 Tex 结构
	tex := &Tex{
		Signature:     "CM3D2_TEX",
		Version:       version,
		TextureName:   texName,
		Rects:         rects,
		Width:         int32(width),
		Height:        int32(height),
		TextureFormat: textureFormat,
		Data:          data,
	}

	return tex, nil
}

// ConvertImageToTexAndWrite 将任意 ImageMagick 支持的文件格式转换为 tex 格式，并写出
// 依赖外部库 ImageMagick，且有 Path 环境变量可以直接调用 magick 命令
// 如果 forcePNG 为 true，且 compress 为 false，则 tex 的数据位是原始 PNG 数据或转换为 PNG
// 如果 forcePNG 为 false，且 compress 为 false，那么检查输入格式是否是 PNG 或 JPG，如果是则数据位直接使用原始图片，否则如果原始格式有损且无透明通道则转换为 JPG，否则转换为 PNG
// 如果 forcePNG 为 true，且 compress 为 true，那么 compress 标识会被忽略，结果同 forcePNG 为 true，且 compress 为 false
// 如果 forcePNG 为 false，且 compress 为 true，那么会对结果进行 DXT 压缩，数据位为 DDS 数据，根据有无透明通道选择 DXT1 或 DXT5
// 如果要生成 1011 版本的 tex（纹理图集），需要在图片目录下有一个同名的 .uv.csv 文件（例如 foo.png 对应 foo.png.uv.csv），文件内容为矩形数组 x, y, w, h 一行一组
// 否则生成 1010 版本的 tex
func ConvertImageToTexAndWrite(inputPath string, texName string, compress bool, forcePNG bool, outputPath string) error {
	tex, err := ConvertImageToTex(inputPath, texName, compress, forcePNG)
	if err != nil {
		return err
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

// ConvertTexToImage 将 Tex 数据转换为图像数据，但不写出
// 依赖外部库 ImageMagick，且有 Path 环境变量可以直接调用 magick 命令
// 如果 forcePNG 为 false 那么如果图像数据位是 JPG 或 PNG 则直接返回数据为，否则根据有没有透明通道保存为 JPG 或 PNG
// 如果 forcePNG 为 true 则强制保存为 PNG，不考虑图像格式和透明通道
// 如果是 1011 版本的 tex（纹理图集），则还会返回 rects
func ConvertTexToImage(tex *Tex, forcePNG bool) (imgData []byte, format string, rects []TexRect, err error) {
	if tex.Version == 1011 {
		rects = tex.Rects
	}

	// 1. 检查 ImageMagick 是否安装
	if err := tools.CheckMagick(); err != nil {
		return nil, "", nil, err
	}

	// 2. 根据 TextureFormat 判断输入数据格式，并判断是否带 Alpha 通道
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
		// 尝试从数据中检测格式
		if len(tex.Data) >= 8 && bytes.Equal(tex.Data[:8], []byte("\x89PNG\r\n\x1a\n")) {
			inputFormat = "png"
			hasAlpha = true
		} else if len(tex.Data) >= 3 && bytes.Equal(tex.Data[:3], []byte("\xff\xd8\xff")) {
			inputFormat = "jpg"
			hasAlpha = false
		} else {
			// 如果检测失败，回退到原始逻辑
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

	// 3. 决定是否跳过转换，直接写出
	//    - 当格式为 ARGB32 (PNG) 时直接返回原始数据
	//    - 当格式为 RGB24 (JPG) 时，如果不强制 PNG，就直接返回原始数据
	skipConversion := false
	if tex.TextureFormat == ARGB32 || (tex.TextureFormat == RGB24 && !forcePNG) {
		skipConversion = true
	}

	if skipConversion {
		return tex.Data, inputFormat, rects, nil
	} else {
		// 4. 使用 ImageMagick 进行内存转换
		//       - forcePNG：强制输出 PNG
		//       - 否则直接写到 outputPath

		if forcePNG {
			// 输出肯定是 PNG
			cmd := exec.Command("magick", inputFormat+":-", "png:-")
			tools.SetHideWindow(cmd)

			var stderrBuf bytes.Buffer
			cmd.Stderr = &stderrBuf

			d := tex.Data
			if tex.TextureFormat == DXT1 || tex.TextureFormat == DXT5 {
				d = ensureDDSHeader(d, tex.Width, tex.Height, tex.TextureFormat)
			}
			cmd.Stdin = bytes.NewReader(d)

			// 从 stdout 读取转换后的 PNG 数据
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
		} else {
			var args []string
			if hasAlpha {
				args = []string{inputFormat + ":-", "png:-"}
				format = "png"
			} else {
				args = []string{inputFormat + ":-", "jpg:-", "-quality", "90"}
				format = "jpg"
			}

			// 输出 JPEG
			cmd := exec.Command("magick", args...)
			tools.SetHideWindow(cmd)

			var stderrBuf bytes.Buffer
			cmd.Stderr = &stderrBuf

			d := tex.Data
			if tex.TextureFormat == DXT1 || tex.TextureFormat == DXT5 {
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
	}
}

// ConvertTexToImageAndWrite 将 .tex 文件转换为图像文件，并写出
// 依赖外部库 ImageMagick，且有 Path 环境变量可以直接调用 magick 命令
// 如果 forcePNG 为 false 那么根据输出路径的后缀名决定输出格式，如果输出路径没有后缀，则根据图像格式来判断，如果是有损格式且没有透明通道，则保存为 JPG，否则保存为 PNG
// 如果 forcePNG 为 true 则强制保存为 PNG，不考虑图像格式和透明通道
// 如果是 1011 版本的 tex（纹理图集），则还会生成一个 .uv.csv 文件（例如 foo.png 对应 foo.png.uv.csv），文件内容为矩形数组 x, y, w, h 一行一组
// 如果输出是 .tex 则原样写出
func ConvertTexToImageAndWrite(tex *Tex, outputPath string, forcePNG bool) error {
	// 如果输入是 tex，直接写出
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

	// 1. 检查 ImageMagick 是否安装
	if err := tools.CheckMagick(); err != nil {
		return err
	}

	// 2. 根据 TextureFormat 判断输入数据格式，并判断是否带 Alpha 通道
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
		// 尝试从数据中检测格式
		if len(tex.Data) >= 8 && bytes.Equal(tex.Data[:8], []byte("\x89PNG\r\n\x1a\n")) {
			inputFormat = "png"
			hasAlpha = true
		} else if len(tex.Data) >= 3 && bytes.Equal(tex.Data[:3], []byte("\xff\xd8\xff")) {
			inputFormat = "jpg"
			hasAlpha = false
		} else {
			// 如果检测失败，回退到原始逻辑
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

	// 3. 如果用户没有指定后缀，则根据实际情况添加
	// 如果强制 PNG，则把后缀改成 PNG
	if forcePNG {
		outputPath = strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".png"
	}

	ext := filepath.Ext(outputPath)
	if ext == "" {
		if forcePNG {
			outputPath += ".png"
		} else {
			// 否则根据是否有 Alpha，决定默认输出是 PNG 还是 JPG
			if hasAlpha {
				outputPath += ".png"
			} else {
				outputPath += ".jpg"
			}
		}
	}

	// 输入是 tex 输出也是 tex 则直接写出
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

	// 4. 决定是否跳过转换，直接写出
	//    - 当格式为 ARGB32 (PNG) 时直接返回原始数据
	//    - 当格式为 RGB24 (JPG) 时，如果不强制 PNG，就直接返回原始数据
	skipConversion := false
	if tex.TextureFormat == ARGB32 || (tex.TextureFormat == RGB24 && !forcePNG) {
		skipConversion = true
	}

	// 4.1 原始数据是 PNG 或 JPG 的情况
	if skipConversion {
		// 如果后缀是 .png 或 .jpg 直接写出数据，避免质量损失
		if ext == ".png" || ext == ".jpg" {
			if err := os.WriteFile(outputPath, tex.Data, 0755); err != nil {
				return fmt.Errorf("failed to write file directly: %w", err)
			}
		}
		// 否则使用 ImageMagick 进行转换成用户想要的格式
		cmd := exec.Command("magick", inputFormat+":-", outputPath)
		tools.SetHideWindow(cmd)

		d := tex.Data
		if tex.TextureFormat == DXT1 || tex.TextureFormat == DXT5 {
			d = ensureDDSHeader(d, tex.Width, tex.Height, tex.TextureFormat)
		}
		cmd.Stdin = bytes.NewReader(d)

		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to convert image: %w, output: %s", err, string(output))
		}
	} else {
		// 4.2 使用 ImageMagick 进行转换并写出
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
			d = ensureDDSHeader(d, tex.Width, tex.Height, tex.TextureFormat)
		}
		cmd.Stdin = bytes.NewReader(d)

		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to convert image: %w, output: %s", err, string(output))
		}
	}

	// 5. 如有 Rects UV 信息，把 UV 信息写入 CSV
	if len(tex.Rects) > 0 {
		uvFilePath := outputPath + ".uv.csv"
		file, err := os.Create(uvFilePath)
		if err != nil {
			return fmt.Errorf("failed to create UV file: %w", err)
		}
		defer file.Close()

		records := make([][]string, 0, len(tex.Rects)+1)
		records = append(records, []string{"X", "Y", "W", "H"}) // 表头
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

// createDDSHeader 为 DXT1/DXT5 创建一个基本的 128 字节 DDS 头部。
func createDDSHeader(width, height int32, format int32) []byte {
	buf := make([]byte, 128)
	copy(buf[0:4], "DDS ")
	binary.LittleEndian.PutUint32(buf[4:8], 124) // Header Size

	// Flags: DDSD_CAPS | DDSD_HEIGHT | DDSD_WIDTH | DDSD_PIXELFORMAT | DDSD_LINEARSIZE
	flags := uint32(0x1 | 0x2 | 0x4 | 0x1000 | 0x80000)
	binary.LittleEndian.PutUint32(buf[8:12], flags)

	binary.LittleEndian.PutUint32(buf[12:16], uint32(height))
	binary.LittleEndian.PutUint32(buf[16:20], uint32(width))

	// PitchOrLinearSize
	var blockSize uint32 = 8
	if format == DXT5 {
		blockSize = 16
	}
	linearSize := uint32((width+3)/4) * uint32((height+3)/4) * blockSize
	binary.LittleEndian.PutUint32(buf[20:24], linearSize)

	binary.LittleEndian.PutUint32(buf[24:28], 0) // Depth
	binary.LittleEndian.PutUint32(buf[28:32], 1) // MipMapCount

	// Pixel Format
	pfOff := 76
	binary.LittleEndian.PutUint32(buf[pfOff:pfOff+4], 32)    // PF Size
	binary.LittleEndian.PutUint32(buf[pfOff+4:pfOff+8], 0x4) // DDPF_FOURCC
	if format == DXT1 {
		copy(buf[pfOff+8:pfOff+12], "DXT1")
	} else {
		copy(buf[pfOff+8:pfOff+12], "DXT5")
	}

	// Caps
	binary.LittleEndian.PutUint32(buf[108:112], 0x1000) // DDSCAPS_TEXTURE

	return buf
}

// ensureDDSHeader 确保 DXT 数据具有 DDS 头部。
// 如果数据已经有 "DDS " 签名，则按原样返回。
// 否则，它会根据提供的宽度、高度和格式合成一个头部。
func ensureDDSHeader(data []byte, width, height int32, format int32) []byte {
	if len(data) >= 4 && string(data[:4]) == "DDS " {
		return data
	}
	header := createDDSHeader(width, height, format)
	return append(header, data...)
}

// isLossyCompression 检查是否为有损压缩格式
// format 为 ImageMagick 输出的文件格式，如 "JPEG"
func isLossyCompression(format string) bool {
	// 大部分有损压缩格式（magick -list format）
	lossyFormats := map[string]bool{
		// 图像格式
		"JPEG":  true, // image/jpeg
		"JPG":   true, // image/jpeg
		"PJPEG": true, // 渐进式JPEG
		"JPS":   true, // 立体JPEG格式
		"MPO":   true, // Multi Picture Object (使用JPEG压缩)
		"JXL":   true, // image/jxl
		//"WEBP":  true, // image/webp
		"AVIF": true, // image/avif
		"HEIC": true, // image/heic
		"HEIF": true, // image/heif

		// 特殊格式
		"WDP": true, // JPEG XR
		"HDP": true, // JPEG XR
		"JNG": true, // JPEG Network Graphics

		// JPEG 2000系列
		"JP2": true, // image/jp2
		"J2C": true, // image/j2c
		"J2K": true, // image/j2k
		"JPC": true, // image/jpc
		"MJ2": true, // image/mj2

		// 其他
		"PCD": true, // Kodak Photo CD
	}

	// 对于 WebP，需要进一步检查是否是有损模式，但这需要更复杂的检测
	// 所以这里简化处理，默认 WebP 为无损

	return lossyFormats[format]
}
