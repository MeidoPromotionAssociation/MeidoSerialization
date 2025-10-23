[English](#english) | [简体中文](#简体中文)

[Disclaimer/Credit/KISS Rule](#kiss-rule)

[![Go Report Card](https://goreportcard.com/badge/github.com/MeidoPromotionAssociation/MeidoSerialization)](https://goreportcard.com/report/github.com/MeidoPromotionAssociation/MeidoSerialization)
[![Github All Releases](https://img.shields.io/github/downloads/MeidoPromotionAssociation/MeidoSerialization/total.svg)]()
[![Go Reference](https://pkg.go.dev/badge/github.com/MeidoPromotionAssociation/MeidoSerialization.svg)](https://pkg.go.dev/github.com/MeidoPromotionAssociation/MeidoSerialization)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/MeidoPromotionAssociation/MeidoSerialization)

# English

# MeidoSerialization

## Introduction

MeidoSerialization is a serialization library written in Golang, designed to handle file formats used in KISS games. It
currently supports CM3D2 and COM3D2 game file formats.

## Features

- Read and write various file formats used in CM3D2 and COM3D2 games
- Convert binary game files to JSON format for easy editing
- Convert JSON files back to binary game formats
- Support for multiple file types including: .menu, .mate, .pmat, .col, .phy, .psk, .tex, .anm, .model, .nei

## Supported File Types

Current Game Version COM3D2 v2.46.1 & COM3D2.5 v3.46.1

| Extension | Description           | Version Support    | Note                                                                                                                                                                              |
|-----------|-----------------------|--------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| .menu     | Menu files            | All versions       | No structural changes so far, so version numbers are irrelevant                                                                                                                   |
| .mate     | Material files        | All versions       | No structural changes so far, but there are some 2.5-only features                                                                                                                |
| .pmat     | Rendering order files | All versions       | No structural changes so far, so version numbers are irrelevant                                                                                                                   |
| .col      | Collider files        | All versions       | No structural changes so far, so version numbers are irrelevant                                                                                                                   |
| .phy      | Physics files         | All versions       | No structural changes so far, so version numbers are irrelevant                                                                                                                   |
| .psk      | Panier skirt files    | All versions       | No structural change since version 217                                                                                                                                            |
| .tex      | Texture files         | All versions       | not support write version 1000, because version 1000 is poorly designed (CM3D2 also supports version 1010,so there is no reason to use)                                           |
| .anm      | Animation files       | All versions       |                                                                                                                                                                                   |
| .model    | Model files           | Versions 1000-2200 |                                                                                                                                                                                   |
| .nei      | Encrypted CSV File    | All Versions       | .nei files use Shift-JIS encoding internally, but we use UTF-8-BOM encoding when reading and writing CSV files. Using characters not supported by Shift-JIS may result in errors. |
| .preset   | Preset files          | All versions       |                                                                                                                                                                                   |

Each file corresponds to a .go
file：[https://github.com/MeidoPromotionAssociation/MeidoSerialization/tree/main/serialization/COM3D2](https://github.com/MeidoPromotionAssociation/MeidoSerialization/tree/main/serialization/COM3D2)

## References

- This library was originally developed for the [COM3D2_MOD_EDITOR](https://github.com/90135/COM3D2_MOD_EDITOR) project
  and was later made independent for easier use. You can also refer to that project for usage examples.
-
pkg.go.dev: [https://pkg.go.dev/github.com/MeidoPromotionAssociation/MeidoSerialization](https://pkg.go.dev/github.com/MeidoPromotionAssociation/MeidoSerialization)
- DeepWiki (Note: May contain AI
  hallucinations): [https://deepwiki.com/MeidoPromotionAssociation/MeidoSerialization](https://deepwiki.com/MeidoPromotionAssociation/MeidoSerialization)

## External Dependencies

- For texture file (.tex) conversion, ImageMagick version 7 or higher is required

## Usage

### Using as a Go Package

1. This repository is published as
   a [Go package](https://pkg.go.dev/github.com/MeidoPromotionAssociation/MeidoSerialization)
2. Install using the go get command:
   ```bash
   go get github.com/MeidoPromotionAssociation/MeidoSerialization
   ```
3. For texture (.tex) file processing, ensure ImageMagick 7.0 or higher is installed and added to your system PATH

#### Use as a command line interface

The MeidoSerialization CLI is a command-line interface for the MeidoSerialization library.

It allows you to convert between COM3D2 MOD files and JSON format using the command line. It also allows you to perform
single or batch conversions between .tex and images, or between .nei and .csv.

JSON files converted by this tool can also be read
by  [COM3D2 MOD EDITOR V2](https://github.com/MeidoPromotionAssociation/COM3D2_MOD_EDITOR).

For details, please see the separate
instructions: [cmd instructions](https://github.com/MeidoPromotionAssociation/MeidoSerialization/blob/main/cmd/README.md)

### Usage examples

#### Use as a command line interface

See separate instructions for
details: [cmd instructions](https://github.com/MeidoPromotionAssociation/MeidoSerialization/blob/main/cmd/README.md)

The CLI provides the following main commands:

- `convert`: Automatically detects and converts files between MOD and JSON formats, TEX and image formats, and NEI and
  CSV formats.
- `convert2json`: Converts MOD files to JSON format.
- `convert2mod`: Converts JSON files back to MOD format.
- `convert2tex`: Converts regular image files to texture files (.tex).
- `convert2image`: Converts .tex files to regular image formats.
- `convert2csv`: Converts .nei files to .csv format.
- `convert2nei`: Converts .csv files to .nei format.
- `determine`: Determines the type of files in a directory or a single file.
- `version`: Gets the version information of MeidoSerialization.

#### Global flags

- `--strict` or `-s`: Use strict mode for file type determination (based on content rather than file extension)
- `--type` or `-t`: Filter by file type. Supported values:
    - `menu, mate, pmat, col, phy, psk, anm, model, tex, nei, csv, image`
    - or `'<type>.json'` for MOD JSON files (e.g., `menu.json`)
    - Note: `<type>` (without `.json`) matches binary only; `<type>.json` matches JSON only.

### In Go Projects

The library provides two main packages:

- `service` package: Provides methods for reading and writing files directly
- `serialization` package: Provides methods for serializing and deserializing structures

<details>

<summary>Example usage</summary>

```go
package main

import (

"bufio"
"fmt"
"os"

serialcom "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/COM3D2"
COM3D2Service "github.com/MeidoPromotionAssociation/MeidoSerialization/service/COM3D2"
)

func main() {
	// Example 1: Using the service package to handle files directly
	// Create a service for handling material files
	mateService := &COM3D2Service.MateService{}

	// Convert a binary material file to JSON
	err := mateService.ConvertMateToJson("example.mate", "example.mate.json")
	if err != nil {
		fmt.Printf("Error converting material file: %v\n", err)
		return
	}

	// Convert a JSON file back to binary material format
	err = mateService.ConvertJsonToMate("example.mate.json", "new_example.mate")
	if err != nil {
		fmt.Printf("Error converting JSON to material file: %v\n", err)
	}

	// Example 2: Using the serialization package to work with structures directly
	// Read a .phy file
	// Be sure to refer to the sample code in the service package to ensure that file reading is handled correctly
	f, err := os.Open("example.phy")
	if err != nil {
		fmt.Printf("Cannot open file: %v\n", err)
		return
	}
	defer f.Close()

	// Use a buffered reader
	br := bufio.NewReader(f)

	// Use the serialization package function to read file content into a structure
	phyData, err := serialcom.ReadPhy(br)
	if err != nil {
		fmt.Printf("Failed to parse .phy file: %v\n", err)
		return
	}

	// Modify data in the structure
	phyData.Damping = 0.8

	// Create a new file and write the modified data
	newFile, err := os.Create("modified.phy")
	if err != nil {
		fmt.Printf("Failed to create new file: %v\n", err)
		return
	}
	defer newFile.Close()

	// Use a buffered writer
	bw := bufio.NewWriter(newFile)

	// Use the Dump method to write the structure to the file
	err = phyData.Dump(bw)
	if err != nil {
		fmt.Printf("Failed to write .phy file: %v\n", err)
		return
	}

	// Flush the buffer
	err = bw.Flush()
	if err != nil {
		fmt.Printf("Error flushing buffer: %v\n", err)
		return
	}

	fmt.Println("All operations completed successfully!")
}
```

</details>

## FAQ

### ImageMagick Issues

If you encounter errors when working with texture (.tex) files:

- Ensure ImageMagick version 7 or higher is installed
- Verify that ImageMagick is in your system PATH (you should be able to run the 'magick' command from any terminal)
- Restart the application after installing ImageMagick

### About version 1011 of the .tex file

- __New fields__: Version 1011 adds a `Rects` (texture atlas) array to the binary structure. Its elements are four
  `float32` values: `x, y, w, h`, representing rectangles in normalized UV space.
	- __When converting an image to `.tex`:
	- If a `.uv.csv` file with the same name exists in the same directory (e.g., `foo.png.uv.csv`), the rectangles in it
	  will be read and the 1011 version of the tex file will be generated.
	- If no `.uv.csv` file exists, the 1010 version (without `Rects`) will be generated.
- __When converting `.tex` to an image__:
	- If the source `.tex` is 1011 and contains `Rects`, a `.uv.csv` file with the same name will be generated next to the
	  output image (e.g., `output.png.uv.csv`).
- __.uv.csv format__:
	- Encoding must be: UTF-8 with BOM.
	- Delimiter: English comma `,`.
	- Number of columns: 4 columns per row, in the order `x, y, w, h` (x, y, width, height); values ​​are typically in the
	  range `[0, 1]` (normalized UVs). It is recommended to retain up to 6 decimal places and use `float32` precision.
	  Example:

```csv
x,y,w,h
0.000000,0.000000,0.500000,0.500000
0.500000,0.000000,0.500000,0.500000
0.000000,0.500000,0.500000,0.500000
```

### Unable to save when using certain characters in `.nei` file

If you encounter the following error, it's because you're using characters that aren't supported by the Shift-JIS
encoding.
.nei files use Shift-JIS encoding internally, and we can't do anything about it. Please remove the unsupported
characters.

- `failed to write to .neiData file: failed to encode string: encoding: rune not supported by encoding.`
- `failed to write to .nei file: failed to encode string: encoding: rune not supported by encoding.`

### About CSV format

All CSV files used in this program are encoded using UTF-8-BOM, separated by ',', and follow
the [RFC4180](https://datatracker.ietf.org/doc/html/rfc4180) standard.

## License

This project is licensed under the BSD-3-Clause License - see the LICENSE file for details.

## Also check out my other repositories

- [COM3D2 Simple MOD Guide Chinese](https://github.com/90135/COM3D2_Simple_MOD_Guide_Chinese)
- [COM3D2 MOD Editor](https://github.com/90135/COM3D2_MOD_EDITOR)
- [COM3D2 Plugin Chinese Translation](https://github.com/90135/COM3D2_Plugin_Translate_Chinese)
- [90135's COM3D2 Chinese Guide](https://github.com/90135/COM3D2_GUIDE_CHINESE)
- [90135's COM3D2 Script Collection](https://github.com/90135/COM3D2_Scripts_901)
- [90135's COM3D2 Tools](https://github.com/90135/COM3D2_Tools_901)

<br>
<br>
<br>
<br>
<br>
<br>

--------

<br>
<br>
<br>
<br>
<br>
<br>

# 简体中文

# MeidoSerialization

## 简介

MeidoSerialization 是一个用 Golang 编写的序列化库，专为处理 KISS 游戏中使用的文件格式而设计。目前支持 CM3D2 和 COM3D2
游戏文件格式。

## 功能特点

- 读取和写入 CM3D2 和 COM3D2 游戏中使用的各种文件格式
- 将二进制游戏文件转换为 JSON 格式以便于编辑
- 将 JSON 文件转换回二进制游戏格式
- 支持多种文件类型，包括：.menu、.mate、.pmat、.col、.phy、.psk、.tex、.anm, .model, .nei

## 支持的文件类型

当前游戏版本 COM3D2 v2.46.1 和 COM3D2.5 v3.46.1

| 扩展名     | 描述        | 版本支持         | 备注                                                                               |
|---------|-----------|--------------|----------------------------------------------------------------------------------|
| .menu   | 菜单文件      | 所有版本         | 目前为止未发生过结构更改，因此版本号无关紧要                                                           |
| .mate   | 材质文件      | 所有版本         | 目前为止未发生过结构更改，但有一些属性只在 2.5 有效                                                     |
| .pmat   | 渲染顺序文件    | 所有版本         | 目前为止未发生过结构更改，因此版本号无关紧要                                                           |
| .col    | 碰撞体文件     | 所有版本         | 目前为止未发生过结构更改，因此版本号无关紧要                                                           |
| .phy    | 物理文件      | 所有版本         | 目前为止未发生过结构更改，因此版本号无关紧要                                                           |
| .psk    | 裙撑文件      | 所有版本         | 自版本 217 以后没有发生结构变化                                                               |
| .tex    | 纹理文件      | 所有版本         | 不支持写出版本 1000，因为版本 1000 设计不佳（CM3D2 也支持版本 1010，因此没有理由使用）                           |
| .anm    | 动画文件      | 所有版本         |                                                                                  |
| .model  | 模型文件      | 1000-2200 版本 |                                                                                  |
| .nei    | 加密 CSV 文件 | 所有版本         | .nei 内部使用 Shift-JIS 编码，但我们在读写时 CSV 时会使用 UTF-8-BOM 编码，如果使用了 Shift-JIS 不支持字符则可能会出错 |
| .preset | 角色预设文件    | 所有版本         |                                                                                  |

每种文件都对应一个 .go
文件：[https://github.com/MeidoPromotionAssociation/MeidoSerialization/tree/main/serialization/COM3D2](https://github.com/MeidoPromotionAssociation/MeidoSerialization/tree/main/serialization/COM3D2)

## 参考

- 本库最初是为了 [COM3D2_MOD_EDITOR](https://github.com/90135/COM3D2_MOD_EDITOR) 项目开发的，后来独立出来以方便各位使用，您也可以参考该项目的使用方法。
-
pkg.go.dev：[https://pkg.go.dev/github.com/MeidoPromotionAssociation/MeidoSerialization](https://pkg.go.dev/github.com/MeidoPromotionAssociation/MeidoSerialization)
- DeepWiki（请注意 AI
  幻觉，有很多内容是它瞎编的）：[https://deepwiki.com/MeidoPromotionAssociation/MeidoSerialization](https://deepwiki.com/MeidoPromotionAssociation/MeidoSerialization)

## 外部依赖

- 对于纹理文件（.tex）转换，需要 ImageMagick 7.0 或更高版本，且已添加到系统 PATH 中，可以使用 `magick` 命令。

## 使用

### 作为 Go 包使用

1. 本仓库已作为 [Go 包](https://pkg.go.dev/github.com/MeidoPromotionAssociation/MeidoSerialization)发布
2. 使用 go get 命令安装：
   ```bash
   go get github.com/MeidoPromotionAssociation/MeidoSerialization
   ```
3. 对于纹理（.tex）文件处理，确保已安装 ImageMagick 7.0 或更高版本，并将其添加到系统 PATH 中

### 作为命令行界面使用

MeidoSerialization CLI 是 MeidoSerialization 库的命令行界面

它允许您使用命令行在 COM3D2 MOD 文件和 JSON 格式之间进行转换，也允许您在 .tex 和图片，或是在 .nei 和 .csv 之间进行单个或批量转换。

由此工具转换的 JSON 文件也可以被 [COM3D2 MOD EDITOR V2](https://github.com/MeidoPromotionAssociation/COM3D2_MOD_EDITOR)
读取。

详情请见单独的说明： [cmd 说明](https://github.com/MeidoPromotionAssociation/MeidoSerialization/blob/main/cmd/README.md)

### 使用参考

#### 作为命令行界面使用

详情请见单独的说明： [cmd 说明](https://github.com/MeidoPromotionAssociation/MeidoSerialization/blob/main/cmd/README.md)

CLI 提供以下主要命令：

- `convert`：自动检测并在 MOD 和 JSON 格式、TEX 和图片格式，NEI 和 CSV 格式之间转换文件。
- `convert2json`：将 MOD 文件转换为 JSON 格式。
- `convert2mod`：将 JSON 文件转换回 MOD 格式。
- `convert2tex`：将普通图片文件转换为纹理文件（.tex）。
- `convert2image`：将 .tex 文件转换为普通图片格式。
- `convert2csv`：将 .nei 文件转换为 .csv 格式。
- `convert2nei`：将 .csv 文件转换为 .nei 格式。
- `determine`：确定目录中的文件或单个文件的类型。
- `version`：获取 MeidoSerialization 的版本信息。

#### 全局标志

- `--strict` 或 `-s`：使用严格模式进行文件类型判断（基于文件内容而非扩展名）
- `--type` 或 `-t`：按类型过滤。支持：
    - `menu, mate, pmat, col, phy, psk, anm, model, tex, nei, csv, image`
    - 或使用 `'<type>.json'` 过滤 MOD 的 JSON 文件（如 `menu.json`）
    - 注意：不带 `.json` 的 `<type>` 仅匹配二进制；带 `.json` 的 `<type>.json` 仅匹配 JSON。

### 在 Go 项目中使用

本库提供了两个主要包：

- `service` 包：提供直接读取和写入文件的方法
- `serialization` 包：提供序列化和反序列化结构体的方法

<details>

<summary>使用示例</summary>

```go
package main

import (
	"bufio"
	"fmt"
	"os"

	serialcom "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/COM3D2"
	COM3D2Service "github.com/MeidoPromotionAssociation/MeidoSerialization/service/COM3D2"
)

func main() {
	// 示例1：使用 service 包直接处理文件
	// 创建一个用于处理材质文件的服务
	mateService := &COM3D2Service.MateService{}

	// 将二进制材质文件转换为 JSON
	err := mateService.ConvertMateToJson("example.mate", "example.mate.json")
	if err != nil {
		fmt.Printf("转换材质文件时出错：%v\n", err)
		return
	}

	// 将 JSON 文件转换回二进制材质格式
	err = mateService.ConvertJsonToMate("example.mate.json", "new_example.mate")
	if err != nil {
		fmt.Printf("将 JSON 转换为材质文件时出错：%v\n", err)
	}

	// 示例2：使用 serialization 包直接操作结构体
	// 读取一个 .phy 文件
	// 请务必参考 service 包中的示例代码，确保正确处理文件读取
	f, err := os.Open("example.phy")
	if err != nil {
		fmt.Printf("无法打开文件：%v\n", err)
		return
	}
	defer f.Close()

	// 使用缓冲读取器
	br := bufio.NewReader(f)

	// 使用 serialization 包中的函数读取文件内容到结构体
	phyData, err := serialcom.ReadPhy(br)
	if err != nil {
		fmt.Printf("解析 .phy 文件失败：%v\n", err)
		return
	}

	// 修改结构体中的数据
	phyData.Damping = 0.8

	// 创建新文件并写入修改后的数据
	newFile, err := os.Create("modified.phy")
	if err != nil {
		fmt.Printf("创建新文件失败：%v\n", err)
		return
	}
	defer newFile.Close()

	// 使用缓冲写入器
	bw := bufio.NewWriter(newFile)

	// 使用 Dump 方法将结构体写入文件
	err = phyData.Dump(bw)
	if err != nil {
		fmt.Printf("写入 .phy 文件失败：%v\n", err)
		return
	}

	// 刷新缓冲区
	err = bw.Flush()
	if err != nil {
		fmt.Printf("刷新缓冲区时出错：%v\n", err)
		return
	}

	fmt.Println("所有操作已成功完成！")
}
```

</details>

## 问与答

### ImageMagick 问题

如果在处理纹理（.tex）文件时遇到错误：

- 确保已安装 ImageMagick 7.0 或更高版本
- 验证 ImageMagick 在您的系统 PATH 中（您应该能够从任何终端运行 'magick' 命令）
- 安装 ImageMagick 后重启应用程序

### 关于 1011 版本的 .tex

- __新增字段__：1011 版本在二进制结构中新增 `Rects`（纹理图集）数组，元素为 `x, y, w, h` 四个 `float32`，表示归一化 UV
  空间内的矩形。
- __将图片转换为 `.tex` 时__：
    - 若同目录存在同名的 `.uv.csv`（如 `foo.png.uv.csv`），会读取其中的矩形并生成 1011 版本的 tex。
    - 若不存在 `.uv.csv`，则生成 1010 版本（不含 `Rects`）。
- __将 `.tex` 转换为图片时__:
    - 若源 `.tex` 为 1011 且包含 `Rects`，在输出图片旁会生成同名 `.uv.csv`（如 `output.png.uv.csv`）
- __.uv.csv 格式__：
    - 编码必须为：UTF-8-BOM。
    - 分隔符：英文逗号`,`。
    - 列数：每行 4 列，依次为 `x, y, w, h` (x, y, width, heigh)；取值通常位于区间 `[0,1]`（归一化 UV），建议保留最多 6 位小数，精度为
      `float32`。
    - 示例：

```csv
x,y,w,h
0.000000,0.000000,0.500000,0.500000
0.500000,0.000000,0.500000,0.500000
0.000000,0.500000,0.500000,0.500000
```

### 在 `.nei` 文件中使用某些字符时无法保存

如果您遇到下面的错误，这是因为您使用了 Shift-JIS 编码不支持的字符。
.nei 文件内部使用 Shift-JIS 编码，我们对此无能为力。请删除不支持的字符。

- `failed to write to .neiData file: failed to encode string: encoding: rune not supported by encoding.`
- `failed to write to .nei file: failed to encode string: encoding: rune not supported by encoding.`

### 关于 CSV 格式

本程序中使用的所有 CSV 文件均采用 UTF-8-BOM 编码，以 ','
分隔，并遵循 [RFC4180](https://datatracker.ietf.org/doc/html/rfc4180) 标准。

## 许可证

本项目采用 BSD-3-Clause License 许可 - 详情请参阅 LICENSE 文件。

## 也可以看看我的其他仓库

- [COM3D2 简明 MOD 教程中文](https://github.com/90135/COM3D2_Simple_MOD_Guide_Chinese)
- [COM3D2 MOD 编辑器](https://github.com/90135/COM3D2_MOD_EDITOR)
- [COM3D2 插件中文翻译](https://github.com/90135/COM3D2_Plugin_Translate_Chinese)
- [90135 的 COM3D2 中文指北](https://github.com/90135/COM3D2_GUIDE_CHINESE)
- [90135 的 COM3D2 脚本收藏集](https://github.com/90135/COM3D2_Scripts_901)
- [90135 的 COM3D2 工具](https://github.com/90135/COM3D2_Tools_901)

<br>
<br>
<br>
<br>
<br>
<br>

--------

<br>
<br>
<br>
<br>
<br>
<br>

# KISS Rule

*This Project is not owned or endorsed by KISS.

*MODs are not supported by KISS.

*KISS cannot be held responsible for any problems that may arise when using MODs.

*If any problem occurs, please do not contact KISS.

```
KISS 規約

・原作がMOD作成者にある場合、又は、原作が「カスタムメイド3D2」のみに存在する内部データの場合、又は、原作が「カスタムメイド3D2」と「カスタムオーダーメイド3D2」の両方に存在する内部データの場合。
※MODはKISSサポート対象外です。
※MODを利用するに当たり、問題が発生してもKISSは一切の責任を負いかねます。
※「カスタムメイド3D2」か「カスタムオーダーメイド3D2」か「CR EditSystem」を購入されている方のみが利用できます。
※「カスタムメイド3D2」か「カスタムオーダーメイド3D2」か「CR EditSystem」上で表示する目的以外の利用は禁止します。
※これらの事項は https://kisskiss.tv/kiss/diary.php?no=558 を優先します。

・原作が「カスタムオーダーメイド3D2(GP01含む)」の内部データのみにある場合。
※MODはKISSサポート対象外です。
※MODを利用するに当たり、問題が発生してもKISSは一切の責任を負いかねます。
※「カスタムオーダーメイド3D2」か「CR EditSystem」をを購入されている方のみが利用できます。
※「カスタムオーダーメイド3D2」か「CR EditSystem」上で表示する目的以外の利用は禁止します。
※「カスタムメイド3D2」上では利用しないで下さい。
※これらの事項は https://kisskiss.tv/kiss/diary.php?no=558 を優先します。

・原作が「CR EditSystem」の内部データのみにある場合。
※MODはKISSサポート対象外です。
※MODを利用するに当たり、問題が発生してもKISSは一切の責任を負いかねます。
※「CR EditSystem」を購入されている方のみが利用できます。
※「CR EditSystem」上で表示する目的以外の利用は禁止します。
※「カスタムメイド3D2」「カスタムオーダーメイド3D2」上では利用しないで下さい。
※これらの事項は https://kisskiss.tv/kiss/diary.php?no=558 を優先します。
```

<br>

# Disclaimer

By downloading this software, you agree to read, accept and abide by this Disclaimer, this is a developer protection
measure and we apologize for any inconvenience this may cause.

下载此软件即表示您已阅读且接受并同意遵守此免责声明，这是为了保护开发人员而采取的措施，对于由此造成的不便，我们深表歉意。

本ソフトウェアをダウンロードすることにより、利用者は本免責事項を読み、内容を理解し、全ての条項に同意し、遵守することを表明したものとみなされます。これは開発者保護のための措置であることをご理解いただき、ご不便をおかけする場合もあらかじめご了承ください。

```
English

In case of any discrepancy between the translated versions, the Simplified Chinese version shall prevail.

1. Tool Nature Statement
    This project is an open-source tool released under the BSD-3-Clause license. The developer(s) (hereinafter referred to as "the Author") are individual technical researchers only. The Author does not derive any commercial benefit from this tool and does not provide any form of online service or user account system.
    This tool is a purely local data processing tool with no content generation capabilities whatsoever. It possesses no online upload or download functionality.
    At its core, this tool is a format converter. All output content is the result of format conversion applied to the user's original input data. The tool itself does not generate, modify, or inject any new data content.

2. Usage restrictions
  This software shall not be used for any illegal purposes. This includes, but is not limited to, creating or disseminating obscene or illegal materials, infringing upon the intellectual property rights of others, violating platform user agreements, or any other actions that may contravene the laws and regulations of the user's jurisdiction.
    Users shall bear full responsibility for any consequences arising from violations of the law.
  
  Users must commit to:
      - Not creating, publishing, transmitting, disseminating, or storing any content that violates the laws and regulations of their jurisdiction.
      - Not creating, publishing, transmitting, disseminating, or storing obscene or illegal materials.
      - Not creating, publishing, transmitting, disseminating, or storing content that infringes upon the intellectual property rights of others.
      - Not creating, publishing, transmitting, disseminating, or storing content that violates platform user agreements.
      - Not using the tool for any activities that endanger national security or undermine social stability.
      - Not using the tool to conduct cyber attacks or crack licensed software.
      - The Author has no legal association with user-generated content.
      - Any content created using this tool that violates local laws and regulations (including but not limited to pornography, violence, or infringing content) entails legal liability borne solely by the content creator.

3. Liability exemption
  Given the nature of open-source projects:
      - The Author cannot monitor the use of all derivative code.
      - The Author is not responsible for modified versions compiled/distributed by users.
      - The Author assumes no liability for any legal consequences resulting from illegal use by users.
      - The Author provides no technical guarantee for content review or filtering.
      - The tool's operational mechanism inherently prevents it from recognizing or filtering content nature.
      - All data processing occurs solely on the user's local device; the Author cannot access or control any user data.

  Users acknowledge and agree that:
      - This tool possesses no content generation capabilities; the final content depends entirely on the input files. The tool merely performs format conversion operations and cannot be held responsible for the legality, nature, or usage context of the user's input data.
      - This tool contains no data upload/download capabilities; all content processing is completed on the user's local device.
      - If illegal activities involving this tool are discovered, they must be reported immediately to the public security authorities.
      - The Author reserves the right to cease distribution of specific versions suspected of being abused.

4. Age and guardianship responsibility
  Users must be persons with full civil capacity (18 years of age or older). Minors are prohibited from downloading, installing or using this tool. Guardians must assume full management responsibility for device access.

5. Agreement Update
  The author has the right to update this statement through the GitHub repository. Continued use is deemed to accept the latest version of the terms.

6. Disclaimer of Warranty
  This tool is provided "AS IS" and the developer expressly disclaims any express or implied warranties, including but not limited to:
    - Warranty of merchantability
    - Warranty of fitness for a particular purpose
    - Warranty of code freedom from defects or potential risks
    - Warranty of continuous availability and technical support

7. Waiver of liability for damages
  Regardless of the use/inability to use this tool resulting in:
    - Direct/indirect property loss
    - Data loss or business interruption
    - Third-party claims or administrative penalties
  The developer shall not bear any civil, administrative or criminal liability

8. Waiver of liability for third-party reliance
  If the third-party libraries/components included or relied upon by this tool have:
    - Intellectual property disputes
    - Security vulnerabilities
    - Content that violates local laws
    - Subject to criminal or civil penalties
  The developer shall not bear joint and several liability, and users should review the relevant licenses on their own

9. Version iteration risk
  Users understand and accept:
    - Different versions of code may have compatibility issues
    - Developers are not obliged to maintain the security of old versions
    - Modifying the code on your own may lead to unforeseen legal risks

简体中文

1. 工具性质声明  
   本项目是基于 BSD-3-Clause 许可证的开源工具。开发者（以下简称"作者"）仅为个人技术研究者，不通过本工具获取任何商业利益，亦不提供任何形式的在线服务及用户账号体系。
   本工具为纯本地化数据处理工具，不具备任何内容生成能力，无任何在线上传下载功能。
   本工具本质上是一个格式转换器，所有输出内容均为用户提供的原始数据的格式转换结果，工具本身不产生、修改或注入任何新数据内容。

2. 使用限制
   本软件不得用于任何违法用途，包括但不限于制作、传播淫秽违法物品、侵害他人知识产权、违反平台用户协议的行为等可能违反所在地法律法规的违法行为。
   使用者因违反法律造成的后果需自行承担全部责任。

   用户必须承诺：  
     - 不制作、发布、传送、传播、储存任何违反所在地法律法规的内容
     - 不制作、发布、传送、传播、储存淫秽违法物品
     - 不制作、发布、传送、传播、储存侵害他人知识产权的内容
     - 不制作、发布、传送、传播、储存违反平台用户协议的内容
     - 不将工具用于任何危害国家安全或破坏社会稳定的活动
     - 不使用本工具实施网络攻击或破解正版软件
     - 开发者与用户生成内容无法律关联性
     - 任何使用本工具创建违反当地法律法规的内容（包括但不限于色情、暴力、侵权内容），其法律责任由内容创建者独立承担

3. 责任豁免  
   鉴于开源项目特性：  
     - 作者无法监控所有衍生代码的使用
     - 不负责用户自行编译/分发的修改版本
     - 不承担用户非法使用导致的任何法律责任
     - 不提供内容审核或过滤的技术保证
     - 工具运行机制决定其无法识别或过滤内容性质
     - 所有数据处理均在用户本地设备完成，开发者无法访问或控制任何用户数据

   用户知悉并同意：
     - 本工具不具备任何内容生成能力，最终内容完全取决于其输入文件。工具仅执行格式转换操作，无法对用户输入数据的合法性、内容性质及使用场景负责。
     - 本工具不包含任何数据上传/下载功能，所有内容生成均在用户本地设备完成
     - 如发现有人利用本工具从事违法活动，应立即向公安机关举报
     - 开发者保留停止分发涉嫌被滥用的特定版本的权利

4. 年龄及监护责任  
   用户须为完全民事行为能力人（18 周岁及以上），禁止未成年人下载、安装或使用。监护人须对设备访问承担完全管理责任。

5. 协议更新  
   作者有权通过 GitHub 仓库更新本声明，继续使用视为接受最新版本条款。

6. 担保免责  
  此工具按"原样"提供，不附带任何明示或暗示的保证，包括但不限于：
     - 适销性担保  
     - 特定用途适用性担保  
     - 代码无缺陷或潜在风险担保  
     - 持续可用性及技术支持担保  

7. 损害赔偿责任免除  
   无论使用/无法使用本工具导致：  
     - 直接/间接财产损失
     - 数据丢失或业务中断
     - 第三方索赔或行政处罚
     - 受到刑事或民事处罚
   开发者均不承担民事、行政或刑事责任  

8. 第三方依赖免责  
   本工具包含或依赖的第三方库/组件如存在：  
     - 知识产权纠纷  
     - 安全漏洞  
     - 违反当地法律的内容  
   开发者不承担连带责任，用户应自行审查相关许可  

9. 版本迭代风险  
    用户理解并接受：  
     - 不同版本代码可能存在兼容性问题  
     - 开发者无义务维护旧版本安全性  
     - 自行修改代码可能导致不可预见的法律风险


日本語

本声明の翻訳版（日本語を含む）と簡体中文原文に解釈上の相違がある場合は、簡体中文版が優先的に有効とします。

1. ツールの性質に関する声明
   本プロジェクトは、BSD-3-Clause ライセンスに基づくオープンソースツールです。開発者（以下「作者」）は個人の技術研究者に過ぎず、本ツールを通じていかなる商業的利益も得ておらず、いかなる形式のオンラインサービス及びユーザーアカウントシステムも提供しません。
   本ツールは純粋にローカル環境でのデータ処理ツールであり、いかなるコンテンツ生成能力も有しておらず、いかなるオンラインアップロード・ダウンロード機能も備えていません。
   本ツールは本質的にフォーマット変換ツールであり、すべての出力内容はユーザーが提供したオリジナルデータのフォーマット変換結果です。ツール自体は、いかなる新しいデータ内容も生成、修正、または注入しません。

2. 使用制限
   本ソフトウェアは、以下のような、所在地の法令に違反する可能性のある違法行為を含むがこれに限定されない、いかなる違法目的にも使用してはなりません：
     - わいせつ物や違法物の作成・頒布
     - 他人の知的財産権の侵害
     - プラットフォーム利用規約違反行為
   使用者は、法律違反によって生じた結果について、自ら全ての責任を負うものとします。

   ユーザーは以下を確約しなければなりません：
     - 所在地の法令に違反する内容を、作成、公開、送信、拡散、保存しないこと。
     - わいせつ物や違法物を、作成、公開、送信、拡散、保存しないこと。
     - 他人の知的財産権を侵害する内容を、作成、公開、送信、拡散、保存しないこと。
     - プラットフォーム利用規約に違反する内容を、作成、公開、送信、拡散、保存しないこと。
     - 本ツールを国家安全を脅かす、または社会の安定を破壊する活動に使用しないこと。
     - 本ツールを使用してネットワーク攻撃を実行したり、正規ソフトウェアのクラッキングを行わないこと。
     - 開発者はユーザー生成コンテンツとの法的関連性を一切有しないこと。
     - 本ツールを使用して作成された、当地の法令に違反するコンテンツ（ポルノ、暴力、著作権侵害等を含むがこれに限定されない）についての法的責任は、コンテンツ作成者が単独で負うこと。

3. 免責事項
   オープンソースプロジェクトの性質上：
     - 作者はすべての派生コードの使用状況を監視することはできません。
     - ユーザー自身がコンパイル/配布する修正版について責任を負いません。
     - ユーザーの違法使用に起因するいかなる法的責任も負いません。
     - コンテンツ審査やフィルタリングの技術的保証は提供しません。
     - ツールの動作メカニズム上、コンテンツの性質を識別またはフィルタリングすることはできません。
     - すべてのデータ処理はユーザーのローカルデバイス上で完了し、開発者はユーザーデータにアクセスまたは制御することはできません。

   ユーザーはこれを理解し同意するものとします：
     - 本ツールはコンテンツ生成能力を一切有しておらず、最終的なコンテンツは完全に入力ファイルに依存します。ツールはフォーマット変換操作のみを実行し、ユーザー入力データの合法性、内容の性質、および使用シナリオについて責任を負うことはできません。
     - 本ツールにはいかなるデータアップロード/ダウンロード機能も含まれておらず、すべてのコンテンツ生成はユーザーのローカルデバイス上で完了します。
     - 本ツールを利用した違法行為を発見した場合は、直ちに公安機関に通報すること。
     - 開発者は、悪用の疑いのある特定バージョンの配布停止権利を留保します。

4. 年齢及び監督責任
   ユーザーは完全民事行為能力者（18歳以上）でなければなりません。未成年者のダウンロード、インストール、または使用は禁止されています。保護者はデバイスへのアクセスについて完全な管理責任を負うものとします。

5. 規約の更新
   作者は、GitHub リポジトリを通じて本声明を更新する権利を有します。継続的な使用は最新版の条項の受諾とみなされます。

6. 保証の免責
   本ツールは「現状のまま」提供され、商品性、特定目的への適合性、コードの欠陥や潜在リスクの不存在、継続的な利用可能性及び技術サポートの保証を含むがこれらに限定されない、明示または黙示を問わず、いかなる保証も付帯しません。

7. 損害賠償責任の免責
   本ツールの使用または使用不能によって生じた以下の事項について、開発者は民事、行政、または刑事上のいかなる責任も負いません：
     - 直接的または間接的な財産上の損害
     - データ損失または業務中断
     - 第三者からの請求または行政処分
     - 刑事罰または民事罰の適用

8. 第三者依存関係に関する免責
   本ツールに含まれる、または依存するサードパーティライブラリ/コンポーネントに関して：
     - 知的財産権に関する紛争
     - セキュリティ上の脆弱性
     - 当地の法律に違反する内容
   が存在する場合でも、開発者は連帯責任を負わず、ユーザーは関連ライセンスを自ら確認するものとします。

9. バージョン更新リスク
   ユーザーは以下を理解し受諾するものとします：
     - 異なるバージョンのコード間で互換性の問題が生じる可能性があること。
     - 開発者は旧バージョンのセキュリティを維持する義務を負わないこと。
     - コードの独自修正は予期せぬ法的リスクを招く可能性があること。
```

<br>

# Credit

- [Golang](https://golang.org/)
- [CM3D2.Serialization](https://github.com/luvoid/CM3D2.Serialization) (I got some file structure information from here)
- [CM3D2.ToolKit](https://github.com/usagirei/CM3D2.Toolkit) by usagirei
- [CM3D2.ToolKit](https://github.com/JustAGuest4168/CM3D2.Toolkit) by JustAGuest4168 (I got .arc and .nei file structure
  information from here)
- [ImageMagick](https://imagemagick.org/) by ImageMagick Studio LLC
