[English](#english) | [简体中文](#简体中文)

# English

# MeidoSerialization CLI

MeidoSerialization CLI is a command-line interface for the MeidoSerialization library, allowing you to convert between
COM3D2 MOD files and JSON formats directly from the command line.

For .tex files, it converts between common image formats and the .tex format.

You can also use [COM3D2 MOD EDITOR V2](https://github.com/90135/COM3D2_MOD_EDITOR) to open the converted json file or
unconverted files.

After converting to JSON text, you can more conveniently use batch processing tools for tasks like keyword replacement.

Please note that the converted JSON does not contain newlines. You may need to use tools like Visual Studio Code to
format it for readability.

You can use this simple GUI tool for batch processing like keyword replacement and renaming, which is useful for
creating variations (Chinese
only): [https://github.com/90135/COM3D2_Tools_901](https://github.com/90135/COM3D2_Tools_901)

## Download

Download in [Release](https://github.com/MeidoPromotionAssociation/MeidoSerialization/releases)

## Usage

The CLI provides eight main commands:

### convert2json

Convert MOD files to JSON format.

Does not support .tex conversion.

```bash
MeidoSerialization.exe convert2json [file/directory]
```

Examples:

```bash
MeidoSerialization.exe convert2json example.menu
MeidoSerialization.exe convert2json ./mods_directory
MeidoSerialization.exe convert2json --type menu ./mods_directory  # Only convert .menu files
```

### convert2mod

Convert JSON files back to MOD format.

Does not support .tex.json conversion.

```bash
MeidoSerialization.exe convert2mod [file/directory]
```

Examples:

```bash
MeidoSerialization.exe convert2mod example.menu.json
MeidoSerialization.exe convert2mod ./json_directory
MeidoSerialization.exe convert2mod --type mate.json ./json_directory  # Only convert .mate.json files
```

### convert2image

Convert .tex files to image format.

```bash
MeidoSerialization.exe convert2image [file/directory]
```

Examples:

```bash
MeidoSerialization.exe convert2image example.tex
MeidoSerialization.exe convert2image example.tex --format jpg  # Convert to JPG format
MeidoSerialization.exe convert2image ./textures_directory
MeidoSerialization.exe convert2image ./textures_directory --format webp # Convert to WebP format
# You can also filter by type in directory mode
MeidoSerialization.exe convert2image ./textures_directory --type tex
```

### convert2tex

Convert image files to .tex format.

```bash
MeidoSerialization.exe convert2tex [file/directory]
```

Examples:

```bash
MeidoSerialization.exe convert2tex example.png
MeidoSerialization.exe convert2tex example.jpg --compress # Use DXT compression
MeidoSerialization.exe convert2tex example.png --forcePng false
MeidoSerialization.exe convert2tex example.png --forcePng true # Force using PNG format (lossless) for the data part of the .tex file
MeidoSerialization.exe convert2tex ./images_directory
MeidoSerialization.exe convert2tex ./images_directory --compress --forcePng false
# Filter only images in directory mode
MeidoSerialization.exe convert2tex ./images_directory --type image
```

### convert2csv

Convert .nei files (encrypted Shift-JIS CSV) to .csv format.

```bash
MeidoSerialization.exe convert2csv [file/directory]
```

Examples:

```bash
MeidoSerialization.exe convert2csv example.nei
MeidoSerialization.exe convert2csv ./nei_directory
# Filter only .nei in directory mode
MeidoSerialization.exe convert2csv ./nei_directory --type nei
```

### convert2nei

Convert .csv files to .nei format (encrypted Shift-JIS CSV).

```bash
MeidoSerialization.exe convert2nei [file/directory]
```

Examples:

```bash
MeidoSerialization.exe convert2nei example.csv
MeidoSerialization.exe convert2nei ./csv_directory
# Filter only .csv in directory mode
MeidoSerialization.exe convert2nei ./csv_directory --type csv
```

### convert

Auto-detect and convert files:

- MOD <-> JSON
- TEX <-> Image
- NEI <-> CSV

```bash
MeidoSerialization.exe convert [file/directory]
```

Examples:

```bash
MeidoSerialization.exe convert example.menu
MeidoSerialization.exe convert example.menu.json
MeidoSerialization.exe convert example.tex
MeidoSerialization.exe convert example.nei
MeidoSerialization.exe convert ./mixed_directory
# In directory mode, you can filter by type
MeidoSerialization.exe convert --type pmat ./mixed_directory      # Only convert .pmat (binary)
MeidoSerialization.exe convert --type pmat.json ./mixed_directory # Only convert .pmat.json
MeidoSerialization.exe convert --type tex ./mixed_directory       # Only convert .tex to image
MeidoSerialization.exe convert --type image ./mixed_directory     # Only convert image files to .tex
MeidoSerialization.exe convert --type nei ./mixed_directory       # Only convert .nei to .csv
MeidoSerialization.exe convert --type csv ./mixed_directory       # Only convert .csv to .nei
```

### determine

Determine the types of files in a directory or a single file.

```bash
MeidoSerialization.exe determine [file/directory]
```

Examples:

```bash
MeidoSerialization.exe determine example.menu
MeidoSerialization.exe determine --strict ./mods_directory
# Type filtering also supported (including '<type>.json')
MeidoSerialization.exe determine --type menu ./mods_directory
MeidoSerialization.exe determine --type menu.json ./mods_directory
```

### unpackArc

Unpack a .arc file into a folder

Examples:

```bash
MeidoSerialization.exe unpackArc example.arc
MeidoSerialization.exe unpackArc example.arc -o ./output_dir
MeidoSerialization.exe unpackArc ./arc_directory
```

### packArc

Pack a directory into a .arc file

Examples:

```bash
MeidoSerialization packArc ./my_folder
MeidoSerialization packArc ./my_folder -o custom.arc
```

### Global Flags

- `--strict` or `-s`: Use strict mode for file type determination (based on content rather than file extension)
- `--type` or `-t`: Filter by file type. Supported values:
    - `menu, mate, pmat, col, phy, psk, anm, model, tex, preset, nei, csv, image`
    - image refers to any image format supported by ImageMagick (such as .png, .jpg, .gif, .webp, etc.)
    - or `'<type>.json'` for MOD JSON files (e.g., `menu.json`)
    - Note: `<type>` (without `.json`) matches binary only; `<type>.json` matches JSON only.

## Supported File Types

see main [README](https://github.com/MeidoPromotionAssociation/MeidoSerialization/blob/main/README.md)

## Build

1. Make sure you have Go installed (version 1.25 or higher)
2. Clone the repository:
   ```bash
   git clone https://github.com/MeidoPromotionAssociation/MeidoSerialization.git
   ```
3. Build the CLI:
   ```bash
   cd MeidoSerialization
   go build -o MeidoSerialization.exe
   ```

<br>
<br>
<br>
<br>
<br>
<br>

------

<br>
<br>
<br>
<br>
<br>
<br>

# 简体中文

# MeidoSerialization CLI

MeidoSerialization CLI 是 MeidoSerialization 库的命令行界面，允许您直接从命令行在 COM3D2 MOD 文件和 JSON 格式之间进行转换。

对于 .tex 文件，则是在普通图片格式和 .tex 格式之间进行转换。

您也可以使用 [COM3D2 MOD EDITOR V2](https://github.com/90135/COM3D2_MOD_EDITOR) 打开转换后的 json 文件或是未转换的文件。

转换为 JSON 文本以后，您可以更为方便地使用一些批处理工具进行批量处理，例如关键词替换等。

请注意转换后的 JSON 是没有换行符的，进行关键词替换时需要注意，您也可以使用 Visual Studio Code 等工具进行格式化。

您可以使用这里提供的简单 GUI
工具来进行简单的关键词替换，重命名等批处理，制作差分很有用（仅中文）：[https://github.com/90135/COM3D2_Tools_901](https://github.com/90135/COM3D2_Tools_901)

## 下载

在 [Release](https://github.com/MeidoPromotionAssociation/MeidoSerialization/releases) 中下载

## 使用方法

CLI 提供了八个主要命令：

### convert2json

将 MOD 文件转换为 JSON 格式。

不支持 .tex 转换

```bash
MeidoSerialization.exe convert2json [文件/目录]
```

示例：

```bash
MeidoSerialization.exe convert2json example.menu
MeidoSerialization.exe convert2json ./mods_directory
MeidoSerialization.exe convert2json --type menu ./mods_directory  # 仅转换 .menu 文件
```

### convert2mod

将 JSON 文件转换回 MOD 格式。

不支持 .tex.json 转换

```bash
MeidoSerialization.exe convert2mod [文件/目录]
```

示例：

```bash
MeidoSerialization.exe convert2mod example.menu.json
MeidoSerialization.exe convert2mod ./json_directory
MeidoSerialization.exe convert2mod --type mate.json ./json_directory  # 仅转换 .mate.json 文件
```

### convert2image

将 .tex 文件转换为图片格式。

```bash
MeidoSerialization.exe convert2image [文件/目录]
```

示例：

```bash
MeidoSerialization.exe convert2image example.tex
MeidoSerialization.exe convert2image example.tex --format jpg  # 转换为 JPG 格式
MeidoSerialization.exe convert2image ./textures_directory
MeidoSerialization.exe convert2image ./textures_directory --format webp # 转换为 WebP 格式
# 目录模式下也可以用类型过滤
MeidoSerialization.exe convert2image ./textures_directory --type tex
```

### convert2tex

将图片文件转换为 .tex 格式。

```bash
MeidoSerialization.exe convert2tex [文件/目录]
```

示例：

```bash
MeidoSerialization.exe convert2tex example.png
MeidoSerialization.exe convert2tex example.jpg --compress # 使用 DXT 压缩
MeidoSerialization.exe convert2tex example.png --forcePng false
MeidoSerialization.exe convert2tex example.png --forcePng true # 强制使用 PNG 格式（无损）进行 .tex 文件的数据部分
MeidoSerialization.exe convert2tex ./images_directory
MeidoSerialization.exe convert2tex ./images_directory --compress --forcePng false
# 目录模式下按类型过滤
MeidoSerialization.exe convert2tex ./images_directory --type image
```

### convert2csv

将 .nei 文件（加密的 Shift-JIS CSV）转换为 .csv 格式。

```bash
MeidoSerialization.exe convert2csv [文件/目录]
```

示例：

```bash
MeidoSerialization.exe convert2csv example.nei
MeidoSerialization.exe convert2csv ./nei_directory
# 目录模式下按类型过滤
MeidoSerialization.exe convert2csv ./nei_directory --type nei
```

### convert2nei

将 .csv 文件转换为 .nei 格式（加密的 Shift-JIS CSV）。

```bash
MeidoSerialization.exe convert2nei [文件/目录]
```

示例：

```bash
MeidoSerialization.exe convert2nei example.csv
MeidoSerialization.exe convert2nei ./csv_directory
# 目录模式下按类型过滤
MeidoSerialization.exe convert2nei ./csv_directory --type csv
```

### convert

自动检测并进行转换：

- MOD <-> JSON
- TEX <-> 图片
- NEI <-> CSV

```bash
MeidoSerialization.exe convert [文件/目录]
```

示例：

```bash
MeidoSerialization.exe convert example.menu
MeidoSerialization.exe convert example.menu.json
MeidoSerialization.exe convert example.tex
MeidoSerialization.exe convert example.nei
MeidoSerialization.exe convert ./mixed_directory
# 目录模式下可按类型过滤
MeidoSerialization.exe convert --type pmat ./mixed_directory      # 仅转换 .pmat（二进制）
MeidoSerialization.exe convert --type pmat.json ./mixed_directory # 仅转换 .pmat.json
MeidoSerialization.exe convert --type tex ./mixed_directory       # 仅将 .tex 转为图片
MeidoSerialization.exe convert --type image ./mixed_directory     # 仅将图片转为 .tex
MeidoSerialization.exe convert --type nei ./mixed_directory       # 仅将 .nei 转为 .csv
MeidoSerialization.exe convert --type csv ./mixed_directory       # 仅将 .csv 转为 .nei
```

### determine

确定目录中的文件或单个文件的类型。

```bash
MeidoSerialization.exe determine [文件/目录]
```

示例：

```bash
MeidoSerialization.exe determine example.menu
MeidoSerialization.exe determine --strict ./mods_directory
# 也支持类型过滤（包含 '<type>.json'）
MeidoSerialization.exe determine --type menu ./mods_directory
MeidoSerialization.exe determine --type menu.json ./mods_directory
```

### unpackArc

将 .arc 文件解压到指定文件夹。

示例：

```bash

MeidoSerialization.exe unpackArc example.arc
MeidoSerialization.exe unpackArc example.arc -o ./output_dir
MeidoSerialization.exe unpackArc ./arc_directory

```

### packArc

将目录打包成 .arc 文件。

示例：

```bash

MeidoSerialization packArc ./my_folder
MeidoSerialization packArc ./my_folder -o custom.arc
```

### 全局参数

- `--strict` 或 `-s`：使用严格模式进行文件类型判断（基于文件内容而非扩展名）
- `--type` 或 `-t`：按类型过滤。支持：
    - `menu, mate, pmat, col, phy, psk, anm, model, tex, preset, nei, csv, image`
    - image 指任意被 ImageMagick 支持的图片格式（如 .png, .jpg, .gif, .webp 等）
    - 或使用 `'<type>.json'` 过滤 MOD 的 JSON 文件（如 `menu.json`）
    - 注意：不带 `.json` 的 `<type>` 仅匹配二进制；带 `.json` 的 `<type>.json` 仅匹配 JSON。

## 支持的文件类型

见主 [README](https://github.com/MeidoPromotionAssociation/MeidoSerialization/blob/main/README.md)

## 构建

1. 确保已安装 Go（版本 1.25 或更高）
2. 克隆仓库：
   ```bash
   git clone https://github.com/MeidoPromotionAssociation/MeidoSerialization.git
   ```
3. 构建 CLI：
   ```bash
   cd MeidoSerialization
   go build -o MeidoSerialization.exe
   ```
