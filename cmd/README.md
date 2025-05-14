# MeidoSerialization CLI

[English](#english) | [简体中文](#简体中文)

## English

### Introduction

MeidoSerialization CLI is a command-line interface for the MeidoSerialization library, allowing you to convert between COM3D2 MOD files and JSON formats directly from the command line.

### Build

1. Make sure you have Go installed (version 1.24 or higher)
2. Clone the repository:
   ```bash
   git clone https://github.com/MeidoPromotionAssociation/MeidoSerialization.git
   ```
3. Build the CLI:
   ```bash
   cd MeidoSerialization
   go build -o MeidoSerialization.exe
   ```

### Usage

The CLI provides four main commands:

#### convert2json

Convert MOD files to JSON format.

```bash
MeidoSerialization.exe convert2json [file/directory]
```

Examples:
```bash
MeidoSerialization.exe convert2json example.menu
MeidoSerialization.exe convert2json ./mods_directory
MeidoSerialization.exe convert2json --type menu ./mods_directory  # Only convert .menu files
```

#### convert2mod

Convert JSON files back to MOD format.

```bash
MeidoSerialization.exe convert2mod [file/directory]
```

Examples:
```bash
MeidoSerialization.exe convert2mod example.menu.json
MeidoSerialization.exe convert2mod ./json_directory
MeidoSerialization.exe convert2mod --type mate ./json_directory  # Only convert .mate.json files
```

#### convert

Auto-detect and convert files between MOD and JSON formats.

```bash
MeidoSerialization.exe convert [file/directory]
```

Examples:
```bash
MeidoSerialization.exe convert example.menu
MeidoSerialization.exe convert example.menu.json
MeidoSerialization.exe convert ./mixed_directory
MeidoSerialization.exe convert --type tex ./mixed_directory  # Only convert .tex and .tex.json files
```

#### determine

Determine the types of files in a directory or a single file.

```bash
MeidoSerialization.exe determine [file/directory]
```

Examples:
```bash
MeidoSerialization.exe determine example.menu
MeidoSerialization.exe determine --strict ./mods_directory
```

### Global Flags

- `--strict` or `-s`: Use strict mode for file type determination (based on content rather than file extension)
- `--type` or `-t`: Filter by file type (menu, mate, pmat, col, phy, psk, tex, anm, model)

### Supported File Types

- `.menu`: Menu files
- `.mate`: Material files
- `.pmat`: Particle material files
- `.col`: Collision files
- `.phy`: Physics files
- `.psk`: Skeleton files
- `.tex`: Texture files
- `.anm`: Animation files
- `.model`: Model files

### External Dependencies

- For texture file (.tex) conversion, ImageMagick version 7 or higher is required and must be in your system PATH

## 简体中文

### 简介

MeidoSerialization CLI 是 MeidoSerialization 库的命令行界面，允许您直接从命令行在 COM3D2 MOD 文件和 JSON 格式之间进行转换。

### 构建

1. 确保已安装 Go（版本 1.24 或更高）
2. 克隆仓库：
   ```bash
   git clone https://github.com/MeidoPromotionAssociation/MeidoSerialization.git
   ```
3. 构建 CLI：
   ```bash
   cd MeidoSerialization
   go build -o MeidoSerialization.exe
   ```

### 使用方法

CLI 提供四个主要命令：

#### convert2json

将 MOD 文件转换为 JSON 格式。

```bash
MeidoSerialization.exe convert2json [文件/目录]
```

示例：
```bash
MeidoSerialization.exe convert2json example.menu
MeidoSerialization.exe convert2json ./mods_directory
MeidoSerialization.exe convert2json --type menu ./mods_directory  # 只转换 .menu 文件
```

#### convert2mod

将 JSON 文件转换回 MOD 格式。

```bash
MeidoSerialization.exe convert2mod [文件/目录]
```

示例：
```bash
MeidoSerialization.exe convert2mod example.menu.json
MeidoSerialization.exe convert2mod ./json_directory
MeidoSerialization.exe convert2mod --type mate ./json_directory  # 只转换 .mate.json 文件
```

#### convert

自动检测并在 MOD 和 JSON 格式之间转换文件。

```bash
MeidoSerialization.exe convert [文件/目录]
```

示例：
```bash
MeidoSerialization.exe convert example.menu
MeidoSerialization.exe convert example.menu.json
MeidoSerialization.exe convert ./mixed_directory
MeidoSerialization.exe convert --type tex ./mixed_directory  # 只转换 .tex 和 .tex.json 文件
```

#### determine

确定目录中的文件或单个文件的类型。

```bash
MeidoSerialization.exe determine [文件/目录]
```

示例：
```bash
MeidoSerialization.exe determine example.menu
MeidoSerialization.exe determine --strict ./mods_directory
```

### 全局标志

- `--strict` 或 `-s`：使用严格模式进行文件类型判断（基于内容而非文件扩展名）
- `--type` 或 `-t`：按文件类型过滤（menu, mate, pmat, col, phy, psk, tex, anm, model）

### 支持的文件类型

- `.menu`：菜单文件
- `.mate`：材质文件
- `.pmat`：粒子材质文件
- `.col`：碰撞文件
- `.phy`：物理文件
- `.psk`：骨骼文件
- `.tex`：纹理文件
- `.anm`：动画文件
- `.model`：模型文件

### 外部依赖

- 对于纹理文件（.tex）转换，需要 ImageMagick 7.0 或更高版本，且已添加到系统 PATH 中
