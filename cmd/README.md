[English](#english) | [简体中文](#简体中文) | [日本語](#日本語)

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

### Transport servers

The CLI also provides transport entry points that share the application
engine with the conversion commands:

```bash
# Local protobuf/gRPC endpoint. Roots are read-only.
MeidoSerialization.exe serve grpc --root mods=C:\Games\COM3D2\Mod

# MCP convenience mode: tools use path and output_path directly.
MeidoSerialization.exe mcp

# MCP restricted mode: root flags enable confinement automatically.
MeidoSerialization.exe mcp `
  --root mods=C:\Games\COM3D2\Mod `
  --write-root work=C:\Users\me\MeidoWork
```

The gRPC listener defaults to `127.0.0.1:50051`; use `--allow-remote` only
when an external trusted boundary provides network security. See
[`docs/transport-api.md`](../docs/transport-api.md) for the protobuf schema,
blob streaming, MCP filesystem modes, tools, and Buf regeneration commands.

With no root flags, MCP is intentionally unrestricted and can access every
regular file allowed by the server process account. Supplying `--root` or
`--write-root` switches to restricted root-ID tool schemas. `--restrict-paths`
selects restricted mode explicitly and, with no roots, denies all file access.
Existing MCP commands that already configure roots remain restricted.

Transport limits can be adjusted with `serve grpc --max-blob-mib`,
`--max-total-blob-mib`, `--max-blobs`, `--blob-ttl`, and `--inline-mib` (the
bundle-wide inline limit cannot exceed 3 MiB). MCP uses `--max-result-mib` for inline
editing JSON and `--max-write-mib` for installed conversion/extraction output.
An explicit `--blob-dir` is exclusively locked for the server lifetime; a
second server using the same directory fails before it can clean active blobs.
KCES raw Unity `.bytes`, `.meta.json`, and `.typetree.json` files are carried
and installed as one artifact bundle by the transport APIs.

For a strongly typed editor, use this order:

1. Call gRPC `GetCapabilities` and select an advertised `format_id` with
   `has_editing_schema`.
2. Fetch `GetFormatSchema`, verify `sha256`, cache by `schema_id` and
   `schema_version`, and feed `schema_json` (Draft 2020-12 JSON Schema) to the
   target-language code generator.
3. Fetch `GetFormatGuide` and verify its SHA-256 and matching `schema_id`.
   The Guide is an embedded source-review profile describing JSON paths,
   Schema pointers, actual game usage, edit roles, risks, invariants, enum
   meanings, and source evidence. Script Guides may also expose structured
   command forms and shared `value_sets`; resolve an argument's
   `value_set_refs` before selecting an enum, MPN, slot, or other game constant.
   Unprefixed `runtime_verified` and
   `serialization_verified` record AI source review, not human approval. The
   `human_` prefix is reserved for explicit human review. `schema_only` confirms
   shape only and must be preserved as opaque data. Every editing format has a
   reviewed profile, while individual unreviewed fields remain `schema_only`.
4. For an LLM, fetch `meido://skills/editing/{format_id}` or use the
   `meido.edit_format` MCP prompt, which embeds the exact Schema and Guide.
5. Detect and inspect the real input, make the smallest requested edit, call
   `Validate`, and convert only after validation succeeds.

MCP exposes the same Schema as `meido://schemas/{format_id}` and the Guide as
`meido://guides/{format_id}`. Guide and Schema generation do not read the local
`game` source tree. Native-only formats do not have an editing schema or Guide;
cross-field and native-wire constraints intentionally remain in the serializers.

The CLI provides the following main commands:

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

Auto-detect and convert files (does not use concurrency when processing directories, so it is slower than dedicated
commands like `convert2json` or `convert2mod` which process directories concurrently).

- MOD <-> JSON
- TEX <-> Image
- NEI <-> CSV
- Bytes (dance data) <-> JSON
- ARC -> Folder

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
MeidoSerialization.exe convert --type bytes ./mixed_directory     # Only convert .bytes to .json
MeidoSerialization.exe convert timeline_data.bytes               # Convert dance timeline to JSON
MeidoSerialization.exe convert maid_data.bytes.json              # Convert JSON back to dance object data
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

### listArc

List all files inside a .arc archive.

```bash
MeidoSerialization.exe listArc [file]
```

Examples:

```bash
MeidoSerialization.exe listArc example.arc
```

### extractArc

Extract files from a .arc archive by extension or file path.

Use `--ext` to extract all files with a given extension.
Use `--file` to extract a single file by its full path or filename within the archive (single .arc only).
If only a filename is given, the archive is searched for a matching entry.

Execution Order:

1. First, attempt an exact match for the complete path.
2. If no match is found, fall back to matching by filename (case-insensitive).
3. If a unique file is matched, return its complete path.
4. If multiple files are matched, report an error and list all matches, prompting the user to specify the complete path.
5. If no match is found, report an error indicating the file was not found.

```bash
MeidoSerialization.exe extractArc [file/directory]
```

Examples:

```bash
MeidoSerialization.exe extractArc example.arc --ext .menu
MeidoSerialization.exe extractArc example.arc --ext tex
MeidoSerialization.exe extractArc example.arc --file folder/texture.tex
MeidoSerialization.exe extractArc example.arc --file texture.tex
MeidoSerialization.exe extractArc example.arc --ext .menu -o ./output_dir
MeidoSerialization.exe extractArc ./arc_directory --ext .tex
```

### Global Flags

- `--strict` or `-s`: Use strict mode for file type determination (based on content rather than file extension)
- `--type` or `-t`: Filter by file type. Supported values:
    - `menu, mate, pmat, col, phy, psk, anm, model, tex, preset, nei, csv, image, arc, bytes`
    - KCES payload filters:
      `menuassets, materialassets, pmatassets, dbconf, dbcol, db2conf, dsbconf, dsb2conf, dslconf, dsl2conf, dslcol, ikcol, limbcol, ikcol.bytes`
    - image refers to any image format supported by ImageMagick (such as .png, .jpg, .gif, .webp, etc.)
    - bytes refers to dance binary data files (timeline_data.bytes, maid_data.bytes, item_data.bytes, event_data.bytes)
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

### gRPC/MCP 传输与强类型编辑器

CLI 的 gRPC 和 MCP 入口与普通转换命令共用同一个 application engine：

```powershell
# gRPC：--root 只读
MeidoSerialization.exe serve grpc --root mods=C:\Games\COM3D2\Mod

# MCP 便捷模式：工具直接使用 path / output_path
MeidoSerialization.exe mcp

# MCP 安全模式：配置任意 root 后自动限制到白名单
MeidoSerialization.exe mcp `
  --root mods=C:\Games\COM3D2\Mod `
  --write-root work=C:\Users\me\MeidoWork
```

不带 root 参数时，MCP 默认进入 `unrestricted` 便捷模式，可访问运行账号有权限访问的
所有普通文件。添加 `--root` 或 `--write-root` 会自动切换为使用 root ID 的安全模式；
也可显式添加 `--restrict-paths`，且不配置 root 时会拒绝全部文件访问。已有的 root 配置
仍保持安全模式，不需要修改启动命令。

显式 `--blob-dir` 在 gRPC 服务生命周期内独占锁定；第二个使用相同目录的实例会在
清理任何活动 blob 前启动失败。KCES raw Unity 的 `.bytes`、`.meta.json` 和
`.typetree.json` 由传输 API 作为一个 artifact bundle 一起读取、返回和安装。

调用方在转换前构建编辑器时，按以下顺序调用：

1. `GetCapabilities`，选择 `has_editing_schema` 的 `format_id`。
2. `GetFormatSchema`，校验 `sha256`，按 `schema_id`/`schema_version` 缓存，并把
   `schema_json` 交给目标语言的 Draft 2020-12 JSON Schema 生成器。
3. `GetFormatGuide`，校验 Guide 的 `sha256`，并确认 Guide 的 `schema_id` 与 Schema
   一致。Guide 是对照源码生成并嵌入程序的审阅 profile，说明 JSON 路径、
   Schema 指针、游戏实际用途、编辑角色、风险、枚举、不变量和证据。
4. LLM 场景读取 `meido://skills/editing/{format_id}`，或使用
   `meido.edit_format` Prompt；Prompt 会直接嵌入 Schema 和 Guide。
5. inspect 真实文件，只做目标所需的最小修改，调用 `Validate`，成功后再 Convert。

无前缀的 `runtime_verified` 和 `serialization_verified` 表示 AI 源码审阅，不代表真人
批准；`human_` 前缀只保留给明确的真人复核。`schema_only` 只确认 JSON 结构，不能从
字段名推断游戏行为，未审阅字段应原样保留。每个 editing format 都有审阅后的 profile，
但个别未审阅字段仍为 `schema_only`。Guide 和 Schema 的生成器
及运行时都不读取本地 `game` 源码目录。完整协议、资源 URI 和安全限制见
[`docs/transport-api.md`](../docs/transport-api.md)。

CLI 提供以下主要命令：

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

自动检测并进行转换（处理目录时不使用并发，因此比 `convert2json`、`convert2mod` 等支持并发的专用命令更慢）：

- MOD <-> JSON
- TEX <-> 图片
- NEI <-> CSV
- Bytes（舞蹈数据）<-> JSON
- ARC -> 文件夹

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
MeidoSerialization.exe convert --type bytes ./mixed_directory     # 仅将 .bytes 转为 .json
MeidoSerialization.exe convert timeline_data.bytes               # 将舞蹈时间线转为 JSON
MeidoSerialization.exe convert maid_data.bytes.json              # 将 JSON 转回舞蹈对象数据
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

### listArc

列出 .arc 存档中的所有文件。

```bash
MeidoSerialization.exe listArc [文件]
```

示例：

```bash
MeidoSerialization.exe listArc example.arc
```

### extractArc

按扩展名或文件路径从 .arc 存档中提取文件。

使用 `--ext` 提取所有具有指定扩展名的文件。
使用 `--file` 按完整路径或文件名提取单个文件（仅限单个 .arc 文件）。
如果仅提供文件名，将在存档中搜索匹配的条目。

执行顺序

1. 先尝试精确匹配完整路径
2. 若未匹配到，退回到按文件名匹配（大小写不敏感）
3. 如果匹配到唯一文件，直接返回其完整路径
4. 如果匹配到多个文件，报错并列出所有匹配项，提示用户指定完整路径
5. 如果没有匹配，报错提示文件未找到

```bash
MeidoSerialization.exe extractArc [文件/目录]
```

示例：

```bash
MeidoSerialization.exe extractArc example.arc --ext .menu
MeidoSerialization.exe extractArc example.arc --ext tex
MeidoSerialization.exe extractArc example.arc --file folder/texture.tex
MeidoSerialization.exe extractArc example.arc --file texture.tex
MeidoSerialization.exe extractArc example.arc --ext .menu -o ./output_dir
MeidoSerialization.exe extractArc ./arc_directory --ext .tex
```

### 全局参数

- `--strict` 或 `-s`：使用严格模式进行文件类型判断（基于文件内容而非扩展名）
- `--type` 或 `-t`：按类型过滤。支持：
    - `menu, mate, pmat, col, phy, psk, anm, model, tex, preset, nei, csv, image, arc, bytes`
    - image 指任意被 ImageMagick 支持的图片格式（如 .png, .jpg, .gif, .webp 等）
    - bytes 指舞蹈二进制数据文件（timeline_data.bytes、maid_data.bytes、item_data.bytes、event_data.bytes）
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

# 日本語

# MeidoSerialization CLI

AI Translated

MeidoSerialization CLI は MeidoSerialization ライブラリのコマンドラインインターフェースで、コマンドラインから直接 COM3D2
MOD ファイルと JSON 形式間の変換が可能です。

.tex ファイルについては、一般的な画像形式と .tex 形式間の変換を行います。

[COM3D2 MOD EDITOR V2](https://github.com/90135/COM3D2_MOD_EDITOR) を使用して、変換された JSON ファイルまたは未変換のファイルを開くこともできます。

JSON テキストに変換した後、キーワード置換などのバッチ処理ツールをより便利に使用できます。

変換された JSON には改行が含まれていないことに注意してください。Visual Studio Code などのツールを使用してフォーマットする必要があるかもしれません。

キーワード置換やリネームなどのバッチ処理には、このシンプルな GUI
ツールを使用できます。差分作成に便利です（中国語のみ）：[https://github.com/90135/COM3D2_Tools_901](https://github.com/90135/COM3D2_Tools_901)

## ダウンロード

[Release](https://github.com/MeidoPromotionAssociation/MeidoSerialization/releases) からダウンロード

## 使用方法

### gRPC/MCP トランスポートと強い型のエディタ

gRPC と MCP は通常の変換コマンドと同じ application engine を使用します。
MCP は root オプションなしでは `path`/`output_path` を直接使う unrestricted
モードです。`--root`、`--write-root`、または `--restrict-paths` を指定すると、
root ID に限定された restricted モードへ自動的に切り替わります。
変換前に `GetCapabilities` -> `GetFormatSchema` -> `GetFormatGuide` の順で取得し、
Schema の `sha256`/ID を検証して型を生成し、Guide の `schema_id` が一致することを
確認してください。Guide はソースと照合した埋め込み review profile で、
JSON パス、実際の用途、編集ロール、リスク、列挙値、根拠を説明します。
接頭辞なしの `runtime_verified` と `serialization_verified` は AI のソース確認で、
人間の承認ではありません。`human_` 接頭辞は明示的な人間レビュー専用です。
`schema_only` は形状だけ確認済みです。すべての editing format
にレビュー済み profile があり、未レビューの個別フィールドは `schema_only` のままです。LLM では
`meido://skills/editing/{format_id}` または `meido.edit_format` を使うと Schema と
Guide が直接埋め込まれます。その後 inspect -> 最小編集 -> `Validate` -> Convert の
順で進めます。生成器と実行時にローカルの `game` ソースは必要ありません。
明示した `--blob-dir` はサーバーの存続中に排他的にロックされます。同じディレクトリ
を使う 2 つ目のサーバーは active blob を削除する前に起動を拒否されます。KCES raw
Unity の `.bytes`、`.meta.json`、`.typetree.json` は 1 つの artifact bundle として
扱われます。

CLI は以下の主要コマンドを提供します：

### convert2json

MOD ファイルを JSON 形式に変換します。

.tex 変換には対応していません。

```bash
MeidoSerialization.exe convert2json [ファイル/ディレクトリ]
```

例：

```bash
MeidoSerialization.exe convert2json example.menu
MeidoSerialization.exe convert2json ./mods_directory
MeidoSerialization.exe convert2json --type menu ./mods_directory  # .menu ファイルのみ変換
```

### convert2mod

JSON ファイルを MOD 形式に変換します。

.tex.json 変換には対応していません。

```bash
MeidoSerialization.exe convert2mod [ファイル/ディレクトリ]
```

例：

```bash
MeidoSerialization.exe convert2mod example.menu.json
MeidoSerialization.exe convert2mod ./json_directory
MeidoSerialization.exe convert2mod --type mate.json ./json_directory  # .mate.json ファイルのみ変換
```

### convert2image

.tex ファイルを画像形式に変換します。

```bash
MeidoSerialization.exe convert2image [ファイル/ディレクトリ]
```

例：

```bash
MeidoSerialization.exe convert2image example.tex
MeidoSerialization.exe convert2image example.tex --format jpg  # JPG 形式に変換
MeidoSerialization.exe convert2image ./textures_directory
MeidoSerialization.exe convert2image ./textures_directory --format webp # WebP 形式に変換
# ディレクトリモードでもタイプでフィルタリング可能
MeidoSerialization.exe convert2image ./textures_directory --type tex
```

### convert2tex

画像ファイルを .tex 形式に変換します。

```bash
MeidoSerialization.exe convert2tex [ファイル/ディレクトリ]
```

例：

```bash
MeidoSerialization.exe convert2tex example.png
MeidoSerialization.exe convert2tex example.jpg --compress # DXT 圧縮を使用
MeidoSerialization.exe convert2tex example.png --forcePng false
MeidoSerialization.exe convert2tex example.png --forcePng true # .tex ファイルのデータ部分に PNG 形式（ロスレス）を強制使用
MeidoSerialization.exe convert2tex ./images_directory
MeidoSerialization.exe convert2tex ./images_directory --compress --forcePng false
# ディレクトリモードで画像のみフィルタリング
MeidoSerialization.exe convert2tex ./images_directory --type image
```

### convert2csv

.nei ファイル（暗号化された Shift-JIS CSV）を .csv 形式に変換します。

```bash
MeidoSerialization.exe convert2csv [ファイル/ディレクトリ]
```

例：

```bash
MeidoSerialization.exe convert2csv example.nei
MeidoSerialization.exe convert2csv ./nei_directory
# ディレクトリモードで .nei のみフィルタリング
MeidoSerialization.exe convert2csv ./nei_directory --type nei
```

### convert2nei

.csv ファイルを .nei 形式（暗号化された Shift-JIS CSV）に変換します。

```bash
MeidoSerialization.exe convert2nei [ファイル/ディレクトリ]
```

例：

```bash
MeidoSerialization.exe convert2nei example.csv
MeidoSerialization.exe convert2nei ./csv_directory
# ディレクトリモードで .csv のみフィルタリング
MeidoSerialization.exe convert2nei ./csv_directory --type csv
```

### convert

自動検出して変換（ディレクトリ処理時は並行処理を使用しないため、`convert2json` や `convert2mod` などの並行処理対応の専用コマンドより遅くなります）：

- MOD <-> JSON
- TEX <-> 画像
- NEI <-> CSV
- Bytes（ダンスデータ）<-> JSON
- ARC -> フォルダ

```bash
MeidoSerialization.exe convert [ファイル/ディレクトリ]
```

例：

```bash
MeidoSerialization.exe convert example.menu
MeidoSerialization.exe convert example.menu.json
MeidoSerialization.exe convert example.tex
MeidoSerialization.exe convert example.nei
MeidoSerialization.exe convert ./mixed_directory
# ディレクトリモードでタイプでフィルタリング可能
MeidoSerialization.exe convert --type pmat ./mixed_directory      # .pmat（バイナリ）のみ変換
MeidoSerialization.exe convert --type pmat.json ./mixed_directory # .pmat.json のみ変換
MeidoSerialization.exe convert --type tex ./mixed_directory       # .tex を画像に変換のみ
MeidoSerialization.exe convert --type image ./mixed_directory     # 画像ファイルを .tex に変換のみ
MeidoSerialization.exe convert --type nei ./mixed_directory       # .nei を .csv に変換のみ
MeidoSerialization.exe convert --type csv ./mixed_directory       # .csv を .nei に変換のみ
MeidoSerialization.exe convert --type bytes ./mixed_directory     # .bytes を .json に変換のみ
MeidoSerialization.exe convert timeline_data.bytes               # ダンスタイムラインを JSON に変換
MeidoSerialization.exe convert maid_data.bytes.json              # JSON をダンスオブジェクトデータに変換
```

### determine

ディレクトリ内のファイルまたは単一ファイルのタイプを判定します。

```bash
MeidoSerialization.exe determine [ファイル/ディレクトリ]
```

例：

```bash
MeidoSerialization.exe determine example.menu
MeidoSerialization.exe determine --strict ./mods_directory
# タイプフィルタリングも対応（'<type>.json' を含む）
MeidoSerialization.exe determine --type menu ./mods_directory
MeidoSerialization.exe determine --type menu.json ./mods_directory
```

### unpackArc

.arc ファイルをフォルダに展開します

例：

```bash
MeidoSerialization.exe unpackArc example.arc
MeidoSerialization.exe unpackArc example.arc -o ./output_dir
MeidoSerialization.exe unpackArc ./arc_directory
```

### packArc

ディレクトリを .arc ファイルにパックします

例：

```bash
MeidoSerialization packArc ./my_folder
MeidoSerialization packArc ./my_folder -o custom.arc
```

### listArc

.arc アーカイブ内のすべてのファイルを一覧表示します。

```bash
MeidoSerialization.exe listArc [ファイル]
```

例：

```bash
MeidoSerialization.exe listArc example.arc
```

### extractArc

拡張子またはファイルパスで .arc アーカイブからファイルを抽出します。

`--ext` を使用して、指定した拡張子のすべてのファイルを抽出します。
`--file` を使用して、フルパスまたはファイル名で単一ファイルを抽出します（単一の .arc ファイルのみ）。
ファイル名のみが指定された場合、アーカイブ内で一致するエントリを検索します。

実行順序：

1. まず、完全なパスで完全一致を試みます。
2. 一致するものが見つからない場合は、ファイル名（大文字と小文字を区別しない）による一致にフォールバックします。
3. 一致するファイルが 1 つだけの場合は、その完全なパスを返します。
4. 一致するファイルが複数ある場合は、エラーを報告し、すべての一致を一覧表示して、ユーザーに完全なパスを指定するよう促します。
5. 一致するものが見つからない場合は、ファイルが見つからなかったことを示すエラーを報告します。

```bash
MeidoSerialization.exe extractArc [ファイル/ディレクトリ]
```

例：

```bash
MeidoSerialization.exe extractArc example.arc --ext .menu
MeidoSerialization.exe extractArc example.arc --ext tex
MeidoSerialization.exe extractArc example.arc --file folder/texture.tex
MeidoSerialization.exe extractArc example.arc --file texture.tex
MeidoSerialization.exe extractArc example.arc --ext .menu -o ./output_dir
MeidoSerialization.exe extractArc ./arc_directory --ext .tex
```

### グローバルフラグ

- `--strict` または `-s`：厳密モードでファイルタイプを判定（拡張子ではなくファイル内容に基づく）
- `--type` または `-t`：タイプでフィルタリング。対応値：
    - `menu, mate, pmat, col, phy, psk, anm, model, tex, preset, nei, csv, image, arc, bytes`
    - image は ImageMagick でサポートされている任意の画像形式を指します（.png, .jpg, .gif, .webp など）
    - bytes はダンスバイナリデータファイルを指します（timeline_data.bytes、maid_data.bytes、item_data.bytes、event_data.bytes）
    - または `'<type>.json'` で MOD の JSON ファイルをフィルタリング（例：`menu.json`）
    - 注意：`.json` なしの `<type>` はバイナリのみにマッチ、`.json` 付きの `<type>.json` は JSON のみにマッチ

## 対応ファイル形式

メインの [README](https://github.com/MeidoPromotionAssociation/MeidoSerialization/blob/main/README.md) を参照

## ビルド

1. Go がインストールされていることを確認（バージョン 1.25 以上）
2. リポジトリをクローン：
   ```bash
   git clone https://github.com/MeidoPromotionAssociation/MeidoSerialization.git
   ```
3. CLI をビルド：
   ```bash
   cd MeidoSerialization
   go build -o MeidoSerialization.exe
   ```
