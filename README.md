[English](#english) | [简体中文](#简体中文)

[![Github All Releases](https://img.shields.io/github/downloads/MeidoPromotionAssociation/MeidoSerialization/total.svg)]()  [![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/MeidoPromotionAssociation/MeidoSerialization)

# English

## MeidoSerialization

### Introduction

MeidoSerialization is a serialization library written in Golang, designed to handle file formats used in KISS games. It
currently supports CM3D2 and COM3D2 game file formats.

### Features

- Read and write various file formats used in CM3D2 and COM3D2 games
- Convert binary game files to JSON format for easy editing
- Convert JSON files back to binary game formats
- Support for multiple file types including: .menu, .mate, .pmat, .col, .phy, .psk, .tex, .anm, and .model

### Supported File Types

Current Game Version COM3D2 v2.44.1 & COM3D2.5 v3.44.1

| Extension | Description           | Version Support    | Note                                                                                                                                    |
|-----------|-----------------------|--------------------|-----------------------------------------------------------------------------------------------------------------------------------------|
| .menu     | Menu files            | All versions       | No structural changes so far, so version numbers are irrelevant                                                                         |
| .mate     | Material files        | All versions       | No structural changes so far, but there are some 2.5-only features                                                                      |
| .pmat     | Rendering order files | All versions       | No structural changes so far, so version numbers are irrelevant                                                                         |
| .col      | Collider files        | All versions       | No structural changes so far, so version numbers are irrelevant                                                                         |
| .phy      | Physics files         | All versions       | No structural changes so far, so version numbers are irrelevant                                                                         |
| .psk      | Panier skirt files    | All versions       | No structural change since version 217                                                                                                  |
| .tex      | Texture files         | All versions       | not support write version 1000, because version 1000 is poorly designed (CM3D2 also supports version 1010,so there is no reason to use) |
| .anm      | Animation files       | All versions       |                                                                                                                                         |
| .model    | Model files           | Versions 1000-2200 |                                                                                                                                         |


Each file corresponds to a .go file：[https://github.com/MeidoPromotionAssociation/MeidoSerialization/tree/main/serialization/COM3D2](https://github.com/MeidoPromotionAssociation/MeidoSerialization/tree/main/serialization/COM3D2)

### References

- This library was originally developed for the [COM3D2_MOD_EDITOR](https://github.com/90135/COM3D2_MOD_EDITOR) project and was later made independent for easier use. You can also refer to that project for usage examples.
- pkg.go.dev: [https://pkg.go.dev/github.com/MeidoPromotionAssociation/MeidoSerialization](https://pkg.go.dev/github.com/MeidoPromotionAssociation/MeidoSerialization)
- DeepWiki (Note: May contain AI hallucinations): [https://deepwiki.com/MeidoPromotionAssociation/MeidoSerialization](https://deepwiki.com/MeidoPromotionAssociation/MeidoSerialization)

### External Dependencies

- For texture file (.tex) conversion, ImageMagick version 7 or higher is required

### Usage

#### Using as a Go Package

1. This repository is published as
   a [Go package](https://pkg.go.dev/github.com/MeidoPromotionAssociation/MeidoSerialization)
2. Install using the go get command:
   ```bash
   go get github.com/MeidoPromotionAssociation/MeidoSerialization
   ```
3. For texture (.tex) file processing, ensure ImageMagick 7.0 or higher is installed and added to your system PATH

#### Use as a command line interface

Currently a command line interface is provided to convert between COM3D2 MOD files and JSON format directly from the command line.

For details, please see the separate instructions: [cmd instructions](https://github.com/MeidoPromotionAssociation/MeidoSerialization/blob/main/cmd/README.md)

### Usage examples

#### Use as a command line interface

See separate instructions for details: [cmd instructions](https://github.com/MeidoPromotionAssociation/MeidoSerialization/blob/main/cmd/README.md)

The CLI provides the following main commands:

`convert2json`: Convert MOD files to JSON format.

`convert2mod`: Convert JSON files back to MOD format.

`convert`: Automatically detect and convert files between MOD and JSON formats.

`determine`: Determine the type of files in a directory or a single file.

`version`: Get version information for MeidoSerialization.

#### Global flags

- `--strict` or `-s`: use strict mode for file type determination (based on content rather than file extension)
- `--type` or `-t`: filter by file type (menu, mate, pmat, col, phy, psk, tex, anm, model)

#### In Go Projects

The library provides two main packages:

- `service` package: Provides methods for reading and writing files directly
- `serialization` package: Provides methods for serializing and deserializing structures

Example usage:

```go
package main

import (
	"fmt"
	servicecom "github.com/MeidoPromotionAssociation/MeidoSerialization/service/COM3D2"
	serialcom "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/COM3D2"
	"os"
	"bufio"
)

func main() {
	// Example 1: Using the service package to handle files directly
	// Create a service for handling material files
	mateService := &servicecom.MateService{}

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

### Troubleshooting

#### ImageMagick Issues

If you encounter errors when working with texture (.tex) files:

- Ensure ImageMagick version 7 or higher is installed
- Verify that ImageMagick is in your system PATH (you should be able to run the 'magick' command from any terminal)
- Restart the application after installing ImageMagick

### License

This project is licensed under the BSD-3-Clause License - see the LICENSE file for details.

---

# 简体中文

## MeidoSerialization

### 简介

MeidoSerialization 是一个用 Golang 编写的序列化库，专为处理 KISS 游戏中使用的文件格式而设计。目前支持 CM3D2 和 COM3D2
游戏文件格式。

### 功能特点

- 读取和写入 CM3D2 和 COM3D2 游戏中使用的各种文件格式
- 将二进制游戏文件转换为 JSON 格式以便于编辑
- 将 JSON 文件转换回二进制游戏格式
- 支持多种文件类型，包括：.menu、.mate、.pmat、.col、.phy、.psk、.tex、.anm 和 .model

### 支持的文件类型

当前游戏版本 COM3D2 v2.44.1 和 COM3D2.5 v3.44.1

| 扩展名    | 描述     | 版本支持         | 备注                                                     |
|--------|--------|--------------|--------------------------------------------------------|
| .menu  | 菜单文件   | 所有版本         | 目前为止未发生过结构更改，因此版本号无关紧要                                 |
| .mate  | 材质文件   | 所有版本         | 目前为止未发生过结构更改，但有一些属性只在 2.5 有效                           |
| .pmat  | 渲染顺序文件 | 所有版本         | 目前为止未发生过结构更改，因此版本号无关紧要                                 |
| .col   | 碰撞体文件  | 所有版本         | 目前为止未发生过结构更改，因此版本号无关紧要                                 |
| .phy   | 物理文件   | 所有版本         | 目前为止未发生过结构更改，因此版本号无关紧要                                 |
| .psk   | 裙撑文件   | 所有版本         | 自版本 217 以后没有发生结构变化                                     |
| .tex   | 纹理文件   | 所有版本         | 不支持写出版本 1000，因为版本 1000 设计不佳（CM3D2 也支持版本 1010，因此没有理由使用） |
| .anm   | 动画文件   | 所有版本         |                                                        |
| .model | 模型文件   | 1000-2200 版本 |                                                        |

每种文件都对应一个 .go 文件：[https://github.com/MeidoPromotionAssociation/MeidoSerialization/tree/main/serialization/COM3D2](https://github.com/MeidoPromotionAssociation/MeidoSerialization/tree/main/serialization/COM3D2)

### 参考

- 本库最初是为了 [COM3D2_MOD_EDITOR](https://github.com/90135/COM3D2_MOD_EDITOR) 项目开发的，后来独立出来以方便各位使用，您也可以参考该项目的使用方法。
- pkg.go.dev：[https://pkg.go.dev/github.com/MeidoPromotionAssociation/MeidoSerialization](https://pkg.go.dev/github.com/MeidoPromotionAssociation/MeidoSerialization)
- DeepWiki（请注意 AI 幻觉，有很多内容是它瞎编的）：[https://deepwiki.com/MeidoPromotionAssociation/MeidoSerialization](https://deepwiki.com/MeidoPromotionAssociation/MeidoSerialization)

### 外部依赖

- 对于纹理文件（.tex）转换，需要 ImageMagick 7.0 或更高版本，且已添加到系统 PATH 中，可以使用 `magick` 命令。

### 使用

#### 作为 Go 包使用

1. 本仓库已作为 [Go 包](https://pkg.go.dev/github.com/MeidoPromotionAssociation/MeidoSerialization)发布
2. 使用 go get 命令安装：
   ```bash
   go get github.com/MeidoPromotionAssociation/MeidoSerialization
   ```
3. 对于纹理（.tex）文件处理，确保已安装 ImageMagick 7.0 或更高版本，并将其添加到系统 PATH 中

#### 作为命令行界面使用

目前提供一个命令行界面，可以直接从命令行在 COM3D2 MOD 文件和 JSON 格式之间进行转换。

详情请见单独的说明： [cmd 说明](https://github.com/MeidoPromotionAssociation/MeidoSerialization/blob/main/cmd/README.md)

### 使用参考

#### 作为命令行界面使用

详情请见单独的说明： [cmd 说明](https://github.com/MeidoPromotionAssociation/MeidoSerialization/blob/main/cmd/README.md)

CLI 提供以下主要命令：

`convert2json`：将 MOD 文件转换为 JSON 格式。

`convert2mod`：将 JSON 文件转换回 MOD 格式。

`convert`：自动检测并在 MOD 和 JSON 格式之间转换文件。

`determine`：确定目录中的文件或单个文件的类型。

`version`：获取 MeidoSerialization 的版本信息。

#### 全局标志

- `--strict` 或 `-s`：使用严格模式进行文件类型判断（基于内容而非文件扩展名）
- `--type` 或 `-t`：按文件类型过滤（menu, mate, pmat, col, phy, psk, tex, anm, model）

#### 在 Go 项目中使用

本库提供了两个主要包：

- `service` 包：提供直接读取和写入文件的方法
- `serialization` 包：提供序列化和反序列化结构体的方法

使用示例：

```go
package main

import (
	"fmt"
	servicecom "github.com/MeidoPromotionAssociation/MeidoSerialization/service/COM3D2"
	serialcom "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/COM3D2"
	"os"
	"bufio"
)

func main() {
	// 示例1：使用 service 包直接处理文件
	// 创建一个用于处理材质文件的服务
	mateService := &servicecom.MateService{}

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

### 故障排除

#### ImageMagick 问题

如果在处理纹理（.tex）文件时遇到错误：

- 确保已安装 ImageMagick 7.0 或更高版本
- 验证 ImageMagick 在您的系统 PATH 中（您应该能够从任何终端运行 'magick' 命令）
- 安装 ImageMagick 后重启应用程序

### 许可证

本项目采用 BSD-3-Clause License 许可 - 详情请参阅 LICENSE 文件。
