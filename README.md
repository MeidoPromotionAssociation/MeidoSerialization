[English](#english) | [简体中文](#简体中文) | [日本語](#日本語)

[AI Agent Guide](docs/ai-agent.md) | [Development](#how-to-dev) | [KISS Rule](#kiss-rule) | [Disclaimer](#disclaimer) | [Credits](#credit)

[![Go Report Card](https://goreportcard.com/badge/github.com/MeidoPromotionAssociation/MeidoSerialization)](https://goreportcard.com/report/github.com/MeidoPromotionAssociation/MeidoSerialization)
[![GitHub All Releases](https://img.shields.io/github/downloads/MeidoPromotionAssociation/MeidoSerialization/total.svg)](https://github.com/MeidoPromotionAssociation/MeidoSerialization/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/MeidoPromotionAssociation/MeidoSerialization.svg)](https://pkg.go.dev/github.com/MeidoPromotionAssociation/MeidoSerialization)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/MeidoPromotionAssociation/MeidoSerialization)

# English

# MeidoSerialization

## Introduction

MeidoSerialization is a Go serialization and format-conversion toolkit for proprietary file formats used by
[KISS](https://www.kisskiss.tv) games. It supports legacy [CM3D2](https://www.kisskiss.tv/cm3d2/) and
[COM3D2](https://com3d2.jp/) formats, as well as the later [KCES](https://kces.jp/) character-editing formats used by
COM3D2.5 and CRC3D3. The same implementation is available through Go packages, a command-line tool, a versioned gRPC
API, and an MCP stdio server.

<br>

If this project is useful to you, please consider giving it a star.

Please report bugs and feature requests through GitHub Issues or Discussions.

You can also find me in the Discord [Custom Maid Server](https://discord.gg/custommaid).

Please ask questions or provide feedback in the group instead of sending direct messages.

## Features

- Read, validate, and write COM3D2/CM3D2 and KCES data
- Convert supported native formats to strict editing JSON and back
- Detect formats by content and batch-convert individual files or directories
- List, extract, unpack, and pack COM3D2 ARC and KCES CT/ABA containers
- Export KCES Texture2D, Sprite, AudioClip, Mesh, and AnimationClip data
- Convert COM3D2 TEX files
- Expose the same conversion capabilities through versioned protobuf/gRPC
- Support MCP stdio so that AI agents can perform edits
- Publish Draft 2020-12 Schemas and source-reviewed field Guides for strongly typed tools and AI-assisted editing

KCES, gRPC, and MCP support is available in MeidoSerialization v2.0.0 and later.

## Supported Formats

Compatibility is currently checked against COM3D2 v2.48.0, COM3D2.5 v3.48.0, and KCES 1.34.4.

Unless stated otherwise, “JSON” below means the strict editing JSON representation shared by the CLI, services, gRPC,
and MCP.

### CM3D2 / COM3D2

| File or extension                                        | Description        | Supported operations        | Notes                                                                        |
|----------------------------------------------------------|--------------------|-----------------------------|------------------------------------------------------------------------------|
| `.menu`                                                  | Menu script        | Native ↔ JSON               | All currently known versions                                                 |
| `.mate`, `.mat`                                          | Material file      | Native ↔ JSON               | Includes COM3D2.5-only material fields; `.mat` is accepted as an alias       |
| `.pmat`                                                  | Render order       | Native ↔ JSON               | All currently known versions                                                 |
| `.col`                                                   | Collider           | Native ↔ JSON               | All currently known versions                                                 |
| `.phy`                                                   | Physics parameters | Native ↔ JSON               | All currently known versions                                                 |
| `.psk`                                                   | Skirt physics      | Native ↔ JSON               | Shared with KCES; no structural changes after version 217                    |
| `.anm`                                                   | Animation          | Native ↔ JSON               | All currently known versions                                                 |
| `.model`                                                 | Model              | Native ↔ JSON               | Versions 1000–2200                                                           |
| `.preset`                                                | Character preset   | Native ↔ JSON               | All currently known versions; embedded KCES data is preserved                |
| `timeline_data.bytes`                                    | Dance timeline     | Native ↔ JSON               | Transport format ID: `com3d2.timeline`                                       |
| `maid_data.bytes`, `item_data.bytes`, `event_data.bytes` | Dance object data  | Native ↔ JSON               | Format ID: `com3d2.object_data`; subtype is detected from content            |
| `.tex`                                                   | Texture            | TEX ↔ image                 | Version 1000 is read-only; writes use 1010 or 1011 and support DXT1/DXT5     |
| `.nei`                                                   | Encrypted CSV      | NEI ↔ CSV                   | Shared with KCES; native text uses Shift-JIS and CSV I/O uses UTF-8 with BOM |
| `.arc`                                                   | Archive            | List, extract, pack, unpack | Encrypted ARC files are not supported                                        |
| `.save`                                                  | Save data          | Detect only                 | Conversion is not provided                                                   |

### KCES / COM3D2.5

| File or extension                                   | Description                                      | Supported operations                   | Notes                                                                                          |
|-----------------------------------------------------|--------------------------------------------------|----------------------------------------|------------------------------------------------------------------------------------------------|
| `.ct`                                               | VirtualDirectory/content table                   | JSON conversion and archive operations | Supports CT catalog inspection and directory packing/unpacking                                 |
| `.aba`                                              | UnityFS AssetBundle                              | List, pack, unpack                     | `packAba` produces a paired `.aba` + `.ct`; encrypted `abap` bundles cannot be decrypted       |
| `.asset_bg`, `.asset_scene`                         | UnityFS AssetBundle                              | List, extract, unpack                  | Uses the same format as ABA                                                                    |
| `.menuassets`                                       | Menu-file container                              | Native ↔ JSON                          |                                                                                                |
| `.materialassets`                                   | Material-file container                          | Native ↔ JSON                          |                                                                                                |
| `.pmatassets`                                       | Render-order-file container                      | Native ↔ JSON                          |                                                                                                |
| `.model`                                            | Model data without mesh geometry                 | Native ↔ JSON                          |                                                                                                |
| `.dbconf`                                           | Physics and collider payload                     | Native ↔ JSON                          |                                                                                                |
| `.dbcol`                                            | Physics and collider payload                     | Native ↔ JSON                          |                                                                                                |
| `.db2conf`                                          | Physics and collider payload                     | Native ↔ JSON                          |                                                                                                |
| `.dsbconf`                                          | Physics and collider payload                     | Native ↔ JSON                          |                                                                                                |
| `.dsb2conf`                                         | Physics and collider payload                     | Native ↔ JSON                          |                                                                                                |
| `.dslconf`                                          | Physics and collider payload                     | Native ↔ JSON                          |                                                                                                |
| `.dsl2conf`                                         | Physics and collider payload                     | Native ↔ JSON                          |                                                                                                |
| `.dslcol`                                           | Physics and collider payload                     | Native ↔ JSON                          |                                                                                                |
| `.ikcol`                                            | Physics and collider payload                     | Native ↔ JSON                          |                                                                                                |
| `.ikcol.bytes`                                      | Physics and collider payload                     | Native ↔ JSON                          |                                                                                                |
| `.limbcol`                                          | Physics and collider payload                     | Native ↔ JSON                          |                                                                                                |
| `.hitcheck`                                         | Hit-check data                                   | Native ↔ editing JSON                  |                                                                                                |
| `.undressdat`                                       | Hit-check data                                   | Native ↔ editing JSON                  | The native format is JSON                                                                      |
| `.undresspdat`                                      | Hit-check data                                   | Native ↔ editing JSON                  | The native format is JSON                                                                      |
| `.nson`                                             | Binary JSON variant                              | Native ↔ editing JSON                  | The native format is JSON                                                                      |
| `.preset`                                           | Character preset                                 | Native ↔ JSON                          |                                                                                                |
| `system.dat`                                        | System state                                     | Native ↔ editing JSON                  |                                                                                                |
| `paths.dat`                                         | Resource search paths                            | Native ↔ editing JSON                  |                                                                                                |
| `bridge_session.vd`                                 | Inter-game bridge-transfer file                  | Native ↔ JSON                          |                                                                                                |
| `.brd`                                              | Bridge, name-map, attachment, and collider state | Native ↔ JSON                          |                                                                                                |
| `.enm`                                              | Bridge, name-map, attachment, and collider state | Native ↔ JSON                          |                                                                                                |
| `.sad`                                              | Bridge, name-map, attachment, and collider state | Native ↔ JSON                          |                                                                                                |
| `maid_collider.bytes`                               | Bridge, name-map, attachment, and collider state | Native ↔ JSON                          |                                                                                                |
| Raw Unity `.bytes` object plus adjacent sidecars    | Unity serialized object                          | Raw object ↔ JSON                      | `.meta.json` and optional `.typetree.json` travel with the primary file as one artifact bundle |
| Native Texture2D and Sprite object files            | Image                                            | Texture2D → PNG/DDS; Sprite → PNG      | One-way conversion                                                                             |
| Native Mesh `.mmesh` and AnimationClip object files | 3D and animation data                            | → glTF 2.0/GLB                         | One-way conversion; some data remains in `.model`, so output is intended for preview           |
| Native AudioClip object files                       | Encoded audio                                    | Lossless inline-payload extraction     | One-way conversion; recognizes OGG, WAV, and FSB5 signatures without transcoding               |

The serializer implementations are under [`serialization/COM3D2`](serialization/COM3D2) and
[`serialization/KCES`](serialization/KCES).

For the authoritative application-level capability list, call gRPC `GetCapabilities` or read the MCP
`meido://capabilities` resource.

## References

- This library was originally developed for [COM3D2_MOD_EDITOR](https://github.com/90135/COM3D2_MOD_EDITOR) and was
  later separated for easier reuse; that project also provides useful usage examples.
- API reference: [pkg.go.dev](https://pkg.go.dev/github.com/MeidoPromotionAssociation/MeidoSerialization)
- Automatically generated project
  overview: [DeepWiki](https://deepwiki.com/MeidoPromotionAssociation/MeidoSerialization)
  (may contain AI hallucinations)
- AI agent installation and basics: [docs/ai-agent.md](docs/ai-agent.md)

## Requirements

- A prebuilt CLI does not require a Go toolchain
- Building from source requires Go 1.26.5 or later, matching `go.mod`
- COM3D2 `.tex` image conversion requires ImageMagick 7 or later with `magick` available on `PATH`

## Usage

### Use as a CLI

Download a prebuilt executable from
[GitHub Releases](https://github.com/MeidoPromotionAssociation/MeidoSerialization/releases).

Read the [complete CLI documentation](cmd/README.md).

**CLI quick start**

```powershell
# Detect and convert a file, or recursively process supported files in a directory
MeidoSerialization.exe convert .\example.menu
MeidoSerialization.exe convert .\mods

# Explicit native/editing JSON conversion
MeidoSerialization.exe convert2json .\example.menu
MeidoSerialization.exe convert2mod .\example.menu.json

# Inspect and unpack KCES containers
MeidoSerialization.exe listCt .\example.ct
MeidoSerialization.exe listAba .\example.aba
MeidoSerialization.exe unpackAba .\example.aba -o .\unpacked

# Export standalone native Unity objects
MeidoSerialization.exe convert2image .\texture.texture2d.bytes
MeidoSerialization.exe convert2gltf .\mesh.mesh.bytes --format glb
MeidoSerialization.exe convert2audio .\voice.audioclip.bytes
```

Command groups:

- Conversion and detection: `convert`, `convert2json`, `convert2mod`, `determine`
- Images, models, animations, and audio: `convert2tex`, `convert2image`, `convert2gltf`, `convert2audio`
- NEI/CSV: `convert2csv`, `convert2nei`
- COM3D2 ARC: `listArc`, `extractArc`, `packArc`, `unpackArc`
- KCES CT/ABA: `listCt`, `packCt`, `unpackCt`, `listAba`, `packAba`, `unpackAba`
- KCES MOD workflow: `inspectKcesCatalog`, `packKcesMod`
- APIs: `serve grpc`, `mcp`
- Utilities: `version`, `completion`

Use `MeidoSerialization.exe --help` or `<command> --help` for current flags and examples. The complete command reference
is in [cmd/README.md](cmd/README.md). `--strict` (`-s`) enables content-based type determination; `--type` (`-t`)
filters directory operations by a native type or `<type>.json`.

### Use the MCP service

The transport mode is always `stdio`: the MCP Host launches `MeidoSerialization.exe mcp` as a child process, exchanges
MCP protocol messages through stdin/stdout, and receives diagnostics through stderr. This server does not expose SSE,
HTTP, or Streamable HTTP endpoints. The `restricted` and `unrestricted` choices below are filesystem access modes, not
transport modes. If the Host presents a transport selector, choose `stdio`.

Start the server:

```powershell
# MCP convenience mode; direct paths have the filesystem permissions of the process account
MeidoSerialization.exe mcp

# MCP restricted mode; path tools use root IDs
MeidoSerialization.exe mcp --root mods=D:\Games\COM3D2\Mod --write-root work=D:\MeidoWork
```

Send this message to your agent to install and configure the tool:

```text
Install and configure MeidoSerialization MCP by following this document:
https://github.com/MeidoPromotionAssociation/MeidoSerialization/blob/main/docs/ai-agent.md

After connecting, read meido://capabilities, then follow the document to obtain the matching Schema, Guide, and editing skill.
```

For manual configuration, start with this convenience-mode example, which does not restrict readable or writable
directories.

Configuration file locations and outer wrappers vary by Host. The transport is stdio; `command` plus `args` launches
the stdio child process. A typical configuration is:

```json
{
  "mcpServers": {
    "meido-serialization": {
      "command": "D:\\Tools\\MeidoSerialization.exe",
      "args": [
        "mcp"
      ]
    }
  }
}
```

To restrict readable and writable directories:

```json
{
  "mcpServers": {
    "meido-serialization": {
      "command": "D:\\Tools\\MeidoSerialization.exe",
      "args": [
        "mcp",
        "--root",
        "mods=D:\\Games\\COM3D2\\Mod",
        "--write-root",
        "work=D:\\MeidoWork"
      ]
    }
  }
}
```

Replace the executable and directory paths with actual local paths.

- `--root mods=...` creates a read-only directory named `mods`.
- `--write-root work=...` creates a readable and writable directory named `work`.

### Use the gRPC service (other programming languages, including strongly typed clients)

```powershell
# Convenience mode; input.path may read any regular file allowed by the server process account
MeidoSerialization.exe serve grpc

# Restricted mode; local files must use file { root_id, relative_path }
MeidoSerialization.exe serve grpc --root mods=D:\Games\COM3D2\Mod
```

With no `--root` or `--restrict-paths`, gRPC uses unrestricted convenience mode and accepts absolute or relative
server-local paths in `ArtifactInput.path`. Supplying either option switches it to restricted mode, where direct paths
are rejected and configured roots are read-only. `GetCapabilities.filesystem_mode` reports the active mode. Conversion
results still return inline or as blobs; gRPC does not install output files on the server.

The transport layer is documented in [docs/transport-api.md](docs/transport-api.md). Inline artifact bundles are capped
at 3 MiB; larger data uses quota-bounded temporary blobs, and archive listings are paged. KCES raw Unity `.bytes`
inputs and outputs preserve adjacent metadata and TypeTree sidecars as one artifact bundle. Dance transport IDs are
`com3d2.timeline` and `com3d2.object_data`.

#### Schema-first editing

Callers that need strongly typed definitions do not have to convert a sample file first.

First call gRPC `GetCapabilities`, select a format with `has_editing_schema: true`, and then call `GetFormatSchema` with
its `format_id`.

The response contains the complete Draft 2020-12 document as `application/schema+json` bytes.

Alternatively, obtain the checked-in Schema JSON files directly from the repository:
[location](https://github.com/MeidoPromotionAssociation/MeidoSerialization/tree/main/schemas/editing/v1).

Pass the Schema to a JSON Schema code generator such as
[quicktype](https://github.com/glideapps/quicktype), `json-schema-to-typescript`, NJsonSchema, or `typify`.

This generates usable code and exposes the structure. Callers should also verify `sha256` and cache by `schema_id` and
`schema_version`.

<details>

<summary>Detailed explanation</summary>

The generated types describe the editing JSON accepted by `Validate` and `Convert`, including polymorphic `oneOf`
branches, recursive `$defs`, base64 byte fields, and standard exact integer ranges. MCP clients fetch the same bytes
from `meido://schemas/{format_id}`. Native-only formats such as `.arc` have no editing Schema and are intentionally not
exposed as typed editing documents. JSON Schema covers the structural contract; call `Validate` after editing to enforce
cross-field and native-wire invariants before conversion.

For semantic editing help, fetch `GetFormatGuide` after the Schema. The Guide maps JSON paths and Schema pointers to
game usage, edit roles, risks, invariants, enum meanings, and evidence. The same verification model is also embedded in
the JSON Schema: `x-meido-format-verification` appears at the root, and reviewed properties carry
`x-meido-verification` beside their title, description, game usage, and editing guidance. Script-like formats can also
publish structured command forms, positional arguments, target-build notes, and shared game-constant `value_sets`; an
argument's `value_set_refs` identifies the exact enum or slot table to use.

Whole-file `format_verification.level` has only two values: `serialization_verified` confirms the file's serialization
contract, while `schema_only` means that only the generated structure is known. It never claims that every field's game
meaning is known. A completed format verification records `authority: ai` or `authority: human`; the non-certified
`schema_only` baseline uses `authority: generated`.

Field verification is independent and structured. `serialization` confirms format, position, or read/write behavior
without claiming game meaning. `source_semantics` confirms the documented purpose or consumption path against game
source and includes serialization verification. `game_behavior` requires an actual game-runtime observation; source
review alone never creates that claim. Every present claim has `status: verified` and an explicit `authority` of `ai`
or `human`. An empty `verification` object means schema-derived only: preserve the field and never infer behavior from
its name. `field_coverage` is only a count summary and never upgrades an individual field. Every published editing
format has a checked-in profile. The normal workflow is Capabilities -> Schema -> Guide -> skill/`meido.edit_format`
-> inspect -> minimal edit -> Validate -> Convert.

MCP exposes the portable workflow at `meido://skills/editing/{format_id}` as a dynamic `text/markdown` resource; it is
not automatically installed as a WorkBuddy skill or MCP-host plugin. Reading it does not replace the Schema and Guide.
The `meido.edit_format` Prompt is the convenient entry point because it returns the skill plus the complete Schema and
Guide together, but it only prepares context and does not edit files by itself. Use whole-file `format_verification` to
decide whether the serialization contract is known, then inspect the exact field's `verification` claims before
assigning game meaning.

</details>

### Use in Go projects

```powershell
go get github.com/MeidoPromotionAssociation/MeidoSerialization@latest
```

The public implementation is split into two package families for each game:

- `service/COM3D2` and `service/KCES` provide path-based reading, writing, and conversion services
- `serialization/COM3D2` and `serialization/KCES` provide typed wire encoders and decoders

<details>

<summary>Example usage</summary>

```go
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"

	serialcom "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/COM3D2"
	com3d2service "github.com/MeidoPromotionAssociation/MeidoSerialization/service/COM3D2"
)

func main() {
	// Example 1: Use the service package to handle files directly
	// Create a service for material files
	mateService := &com3d2service.MateService{}
	ctx := context.Background()
	const maxOutputBytes int64 = 64 << 20

	// Convert a binary material file to JSON
	err := mateService.ConvertMateToJson(ctx, "example.mate", "example.mate.json", maxOutputBytes)
	if err != nil {
		fmt.Printf("Error converting material file: %v\n", err)
		return
	}

	// Convert JSON back to the binary material format
	err = mateService.ConvertJsonToMate(ctx, "example.mate.json", "new_example.mate", maxOutputBytes)
	if err != nil {
		fmt.Printf("Error converting JSON to material file: %v\n", err)
	}

	// Example 2: Use the serialization package to work with a structure directly
	// Read a .phy file; refer to service package examples for complete file handling
	f, err := os.Open("example.phy")
	if err != nil {
		fmt.Printf("Cannot open file: %v\n", err)
		return
	}
	defer f.Close()

	br := bufio.NewReader(f)
	phyData, err := serialcom.ReadPhy(br)
	if err != nil {
		fmt.Printf("Failed to parse .phy file: %v\n", err)
		return
	}

	phyData.Damping = 0.8

	newFile, err := os.Create("modified.phy")
	if err != nil {
		fmt.Printf("Failed to create new file: %v\n", err)
		return
	}
	defer newFile.Close()

	bw := bufio.NewWriter(newFile)
	if err := phyData.Dump(bw); err != nil {
		fmt.Printf("Failed to write .phy file: %v\n", err)
		return
	}
	if err := bw.Flush(); err != nil {
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

- New fields:
    - Version 1011 adds a `Rects` (texture atlas) array to the binary structure. Its elements are four `float32` values:
      `x, y, w, h`, representing rectangles in normalized UV space.
- When converting an image to `.tex`:
    - If a `.uv.csv` file with the same name exists in the same directory (e.g., `foo.png.uv.csv`), the rectangles in it
      will be read and the 1011 version of the tex file will be generated.
    - If no `.uv.csv` file exists, the 1010 version (without `Rects`) will be generated.
- When converting `.tex` to an image:
    - If the source `.tex` is 1011 and contains `Rects`, a `.uv.csv` file with the same name will be generated next to
      the output image (e.g., `output.png.uv.csv`).
- .uv.csv format:
    - Encoding must be: UTF-8 with BOM.
    - Delimiter: English comma `,`.
    - Number of columns: 4 columns per row, in the order `x, y, w, h` (x, y, width, height); values are typically in the
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
encoding. .nei files use Shift-JIS encoding internally, and we can't do anything about it. Please remove the unsupported
characters.

- `failed to write to .neiData file: failed to encode string: encoding: rune not supported by encoding.`
- `failed to write to .nei file: failed to encode string: encoding: rune not supported by encoding.`

### About CSV format

All CSV files used in this program are encoded using UTF-8-BOM, separated by `,`, and follow
the [RFC4180](https://datatracker.ietf.org/doc/html/rfc4180) standard.

## License

This project is licensed under the BSD-3-Clause License - see the LICENSE file for details.

## Also check out other repositories

- [COM3D2 MOD Editor](https://github.com/MeidoPromotionAssociation/COM3D2_MOD_EDITOR)
- [COM3D2 Batch File Converter tool and Serialization Library](https://github.com/MeidoPromotionAssociation/MeidoSerialization)
- [COM3D2 Simple Chinese MOD Tutorial](https://github.com/MeidoPromotionAssociation/COM3D2_Simple_MOD_Guide_Chinese)
- [Another COM3D2 Translation Plugin JAT](https://github.com/MeidoPromotionAssociation/COM3D2.JustAnotherTranslator.Plugin)
- [90135's COM3D2 Chinese Guide](https://github.com/90135/COM3D2_GUIDE_CHINESE)
- [90135's COM3D2 Script Collection](https://github.com/90135/COM3D2_Scripts_901)
- [90135's COM3D2 Tools](https://github.com/90135/COM3D2_Tools_901)

<br>
<br>
<br>

--------

<br>
<br>
<br>

# 简体中文

# MeidoSerialization

## 简介

MeidoSerialization 是一个使用 Go 编写的序列化与格式转换工具集，专门处理 [KISS](https://www.kisskiss.tv)
游戏使用的专有文件格式。目前同时支持传统 [CM3D2](https://www.kisskiss.tv/cm3d2/)、
[COM3D2](https://com3d2.jp/) 格式，以及 COM3D2.5 和 CRC3D3 使用的后续 [KCES](https://kces.jp/) 角色编辑系统格式。同一套实现可以通过
Go 包、命令行工具、版本化 gRPC API 和 MCP stdio 服务器调用。

<br>

如果本项目对您有帮助，欢迎点亮 Star~

如需报告 Bug 或提出功能建议，请使用 GitHub Issues 或 Discussions。

您也可以在 Discord [Custom Maid Server](https://discord.gg/custommaid) 找到我。

有问题请在群内提问/反馈，请勿私聊

## 功能特点

- 读取、校验并写出 COM3D2/CM3D2 与 KCES 数据
- 在受支持的原生格式与严格 editing JSON 之间双向转换
- 按内容识别文件格式，并对单个文件或目录执行批量转换
- 列出、提取、解包和打包 COM3D2 ARC 及 KCES CT/ABA 容器
- 导出 KCES Texture2D、Sprite、AudioClip、Mesh 和 AnimationClip 数据
- 转换 COM3D2 TEX
- 通过版本化 protobuf/gRPC 接口提供统一的转换能力
- 支持 MCP stdio 协议，让您可以使用 AI 进行编辑
- 为强类型工具和 AI 辅助编辑提供 Draft 2020-12 Schema 与经过源码审阅的字段 Guide

KCES、gRPC、MCP 支持在 MeidoSerialization v2.0.0 版本后可用。

## 支持的格式

当前兼容性以 COM3D2 v2.48.0 和 COM3D2.5 v3.48.0 以及 KCES 1.34.4 为基准进行检查。

除非另有说明，下表中的“JSON”均指 CLI、service、gRPC 和 MCP 共用的严格 editing JSON 表示。

### CM3D2 / COM3D2

| 文件或扩展名                                             | 内容/描述    | 支持的操作             | 备注                                                                  |
|----------------------------------------------------------|--------------|------------------------|-----------------------------------------------------------------------|
| `.menu`                                                  | 菜单脚本     | 原生格式 ↔ JSON        | 支持目前已知的全部版本                                                |
| `.mate`、`.mat`                                          | 材质文件     | 原生格式 ↔ JSON        | 包含仅 COM3D2.5 使用的材质字段；`.mat` 可作为别名                     |
| `.pmat`                                                  | 渲染顺序     | 原生格式 ↔ JSON        | 支持目前已知的全部版本                                                |
| `.col`                                                   | 碰撞体       | 原生格式 ↔ JSON        | 支持目前已知的全部版本                                                |
| `.phy`                                                   | 物理参数     | 原生格式 ↔ JSON        | 支持目前已知的全部版本                                                |
| `.psk`                                                   | 裙子专用物理 | 原生格式 ↔ JSON        | 与 KCES 共用；版本 217 以后没有结构变化                               |
| `.anm`                                                   | 动画文件     | 原生格式 ↔ JSON        | 支持目前已知的全部版本                                                |
| `.model`                                                 | 模型         | 原生格式 ↔ JSON        | 支持版本 1000–2200                                                    |
| `.preset`                                                | 角色预设     | 原生格式 ↔ JSON        | 支持目前已知的全部版本，并保留内嵌 KCES 数据                          |
| `timeline_data.bytes`                                    | 舞蹈时间线   | 原生格式 ↔ JSON        | 传输 API 中的格式 ID 为 `com3d2.timeline`                             |
| `maid_data.bytes`、`item_data.bytes`、`event_data.bytes` | 舞蹈对象数据 | 原生格式 ↔ JSON        | 传输 API 中的格式 ID 为 `com3d2.object_data`，按内容识别子类型        |
| `.tex`                                                   | 纹理         | TEX ↔ 图片             | 版本 1000 只读；写出使用 1010 或 1011，并支持 DXT1/DXT5               |
| `.nei`                                                   | 加密 CSV     | NEI ↔ CSV              | 与 KCES 共用；原生文本使用 Shift-JIS，CSV 输入输出使用带 BOM 的 UTF-8 |
| `.arc`                                                   | 归档文件     | 列出、提取、打包、解包 | 不支持加密 ARC                                                        |
| `.save`                                                  | 存档         | 仅识别                 | 不提供转换功能                                                        |

### KCES / COM3D2.5

| 文件或扩展名                               | 内容/描述                        | 支持的操作                        | 备注                                                                         |
|--------------------------------------------|----------------------------------|-----------------------------------|------------------------------------------------------------------------------|
| `.ct`                                      | VirtualDirectory/内容表          | JSON 转换和归档操作               | 支持 CT catalog 检查及目录打包/解包                                          |
| `.aba`                                     | UnityFS AssetBundle              | 列出、打包、解包                  | `packAba` 会生成配套的 `.aba` + `.ct`；不支持解密 `abap` 加密包              |
| `.asset_bg`、`.asset_scene`                | UnityFS AssetBundle              | 列出、提取、解包                  | 与 aba 完全相同                                                              |
| `.menuassets`                              | 菜单文件容器                     | 原生格式 ↔ JSON                   |                                                                              |
| `.materialassets`                          | 材质文件容器                     | 原生格式 ↔ JSON                   |                                                                              |
| `.pmatassets`                              | 渲染顺序文件容器                 | 原生格式 ↔ JSON                   |                                                                              |
| `.model`                                   | 模型数据（不含网格）             | 原生格式 ↔ JSON                   |                                                                              |
| `.dbconf`                                  | 物理与碰撞载荷                   | 原生格式 ↔ JSON                   |                                                                              |
| `.dbcol`                                   | 物理与碰撞载荷                   | 原生格式 ↔ JSON                   |                                                                              |
| `.db2conf`                                 | 物理与碰撞载荷                   | 原生格式 ↔ JSON                   |                                                                              |
| `.dsbconf`                                 | 物理与碰撞载荷                   | 原生格式 ↔ JSON                   |                                                                              |
| `.dsb2conf`                                | 物理与碰撞载荷                   | 原生格式 ↔ JSON                   |                                                                              |
| `.dslconf`                                 | 物理与碰撞载荷                   | 原生格式 ↔ JSON                   |                                                                              |
| `.dsl2conf`                                | 物理与碰撞载荷                   | 原生格式 ↔ JSON                   |                                                                              |
| `.dslcol`                                  | 物理与碰撞载荷                   | 原生格式 ↔ JSON                   |                                                                              |
| `.ikcol`                                   | 物理与碰撞载荷                   | 原生格式 ↔ JSON                   |                                                                              |
| `.ikcol.bytes`                             | 物理与碰撞载荷                   | 原生格式 ↔ JSON                   |                                                                              |
| `.limbcol`                                 | 物理与碰撞载荷                   | 原生格式 ↔ JSON                   |                                                                              |
| `.hitcheck`                                | 碰撞检测数据                     | 原生格式 ↔ editing JSON           |                                                                              |
| `.undressdat`                              | 碰撞检测数据                     | 原生格式 ↔ editing JSON           | 原生格式本身就是 JSON                                                        |
| `.undresspdat`                             | 碰撞检测数据                     | 原生格式 ↔ editing JSON           | 原生格式本身就是 JSON                                                        |
| `.nson`                                    | 二进制 JSON 变体                 | 原生格式 ↔ editing JSON           | 原生格式本身就是 JSON                                                        |
| `.preset`                                  | 角色预设                         | 原生格式 ↔ JSON                   |                                                                              |
| `system.dat`                               | 系统状态                         | 原生格式 ↔ editing JSON           |                                                                              |
| `paths.dat`                                | 资源搜索路径                     | 原生格式 ↔ editing JSON           |                                                                              |
| `bridge_session.vd`                        | 游戏间的桥接传输文件             | 原生格式 ↔ JSON                   |                                                                              |
| `.brd`                                     | 桥接、名称映射、附件与碰撞体状态 | 原生格式 ↔ JSON                   |                                                                              |
| `.enm`                                     | 桥接、名称映射、附件与碰撞体状态 | 原生格式 ↔ JSON                   |                                                                              |
| `.sad`                                     | 桥接、名称映射、附件与碰撞体状态 | 原生格式 ↔ JSON                   |                                                                              |
| `maid_collider.bytes`                      | 桥接、名称映射、附件与碰撞体状态 | 原生格式 ↔ JSON                   |                                                                              |
| raw Unity 对象 `.bytes` 及相邻 sidecar     | Unity 序列化对象                 | raw 对象 ↔ JSON                   | `.meta.json` 和可选 `.typetree.json` 与主文件作为同一个 artifact bundle 传输 |
| 原生 Texture2D、Sprite 对象文件            | 图片                             | Texture2D → PNG/DDS；Sprite → PNG | 单向转换                                                                     |
| 原生 Mesh `.mmesh`、AnimationClip 对象文件 | 3D 与动画数据                    | → glTF 2.0/GLB                    | 单向转换，部分数据位于 .model 非完整转换，仅建议用于预览                     |
| 原生 AudioClip 对象文件                    | 编码音频                         | 无损提取内联载荷                  | 单向转换，识别 OGG、WAV 和 FSB5 签名，不执行转码                             |

序列化实现位于 [`serialization/COM3D2`](serialization/COM3D2) 和 [`serialization/KCES`](serialization/KCES)。

如需应用层能力的权威列表，请调用 gRPC `GetCapabilities`，或读取 MCP 的 `meido://capabilities` 资源。

## 参考

- 本库最初是为了 [COM3D2_MOD_EDITOR](https://github.com/90135/COM3D2_MOD_EDITOR) 项目开发的，后来独立出来以方便各位使用，您也可以参考该项目的使用方法。
- API 参考：[pkg.go.dev](https://pkg.go.dev/github.com/MeidoPromotionAssociation/MeidoSerialization)
- 自动生成的项目概览：[DeepWiki](https://deepwiki.com/MeidoPromotionAssociation/MeidoSerialization)（可能包含 AI 幻觉）
- AI agent 安装与基础使用：[docs/ai-agent.md](docs/ai-agent.md)

## 环境要求

- 下载预编译 CLI 无需安装 Go 工具链
- 从源码构建需要 Go 1.26.5 或更高版本，与 `go.mod` 保持一致
- 转换 COM3D2 `.tex` 图片需要安装 ImageMagick 7 或更高版本，并确保 `magick` 位于 `PATH`

## 使用

### 作为 CLI 命令行程序使用

从 [GitHub Releases](https://github.com/MeidoPromotionAssociation/MeidoSerialization/releases) 下载预编译程序

查看 CLI 使用说明：[说明](https://github.com/MeidoPromotionAssociation/MeidoSerialization/blob/main/cmd/README.md)

**CLI 快速上手**

```powershell
# 自动识别并转换单个文件，或递归转换目录中的受支持文件
MeidoSerialization.exe convert .\example.menu
MeidoSerialization.exe convert .\mods

# 明确执行原生格式/editing JSON 转换
MeidoSerialization.exe convert2json .\example.menu
MeidoSerialization.exe convert2mod .\example.menu.json

# 检查并解包 KCES 容器
MeidoSerialization.exe listCt .\example.ct
MeidoSerialization.exe listAba .\example.aba
MeidoSerialization.exe unpackAba .\example.aba -o .\unpacked

# 导出独立的原生 Unity 对象
MeidoSerialization.exe convert2image .\texture.texture2d.bytes
MeidoSerialization.exe convert2gltf .\mesh.mesh.bytes --format glb
MeidoSerialization.exe convert2audio .\voice.audioclip.bytes
```

命令分组如下：

- 转换与识别：`convert`、`convert2json`、`convert2mod`、`determine`
- 图片、模型、动画与音频：`convert2tex`、`convert2image`、`convert2gltf`、`convert2audio`
- NEI/CSV：`convert2csv`、`convert2nei`
- COM3D2 ARC：`listArc`、`extractArc`、`packArc`、`unpackArc`
- KCES CT/ABA：`listCt`、`packCt`、`unpackCt`、`listAba`、`packAba`、`unpackAba`
- KCES MOD 工作流：`inspectKcesCatalog`、`packKcesMod`
- API：`serve grpc`、`mcp`
- 辅助命令：`version`、`completion`

使用 `MeidoSerialization.exe --help` 或 `<command> --help` 查看当前参数与示例。完整命令说明位于
[cmd/README.md](cmd/README.md)。`--strict`（`-s`）启用基于内容的类型判断；`--type`（`-t`）按原生 类型或 `<type>.json` 过滤目录操作。

### 使用 MCP 服务

传输模式始终为 `stdio`：MCP Host 会把 `MeidoSerialization.exe mcp` 作为子进程启动，通过 stdin/stdout 交换 MCP 协议消息，并从
stderr 接收诊断日志。本服务不提供 SSE、HTTP 或 Streamable HTTP 端点。下面的
`restricted` 与 `unrestricted` 是文件系统访问模式，不是传输模式。如果 Host 界面要求选择传输模式，请选择 `stdio`。

启动服务：

```powershell
# MCP 便捷模式；直接路径拥有运行进程账号可访问的文件系统权限
MeidoSerialization.exe mcp

# MCP 受限模式；路径工具改用 root ID
MeidoSerialization.exe mcp --root mods=D:\Games\COM3D2\Mod --write-root work=D:\MeidoWork
```

把以下信息发送给你的 agent 即可安装和配置：

```text
请按照以下文档安装并配置 MeidoSerialization MCP：
https://github.com/MeidoPromotionAssociation/MeidoSerialization/blob/main/docs/ai-agent.md

连接后先读取 meido://capabilities，再按文档获取对应的 Schema、Guide 和 editing skill。
```

手动配置时，可参考下面的便捷模式示例（不限定可读写目录）。

不同 Host 的配置文件位置和外层结构可能不同；

传输模式为 stdio 其中 `command` 与 `args` 用于启动 stdio 子进程，典型配置如下：

```json
{
  "mcpServers": {
    "meido-serialization": {
      "command": "D:\\Tools\\MeidoSerialization.exe",
      "args": [
        "mcp"
      ]
    }
  }
}
```

若要限制可读写目录

```json
{
  "mcpServers": {
    "meido-serialization": {
      "command": "D:\\Tools\\MeidoSerialization.exe",
      "args": [
        "mcp",
        "--root",
        "mods=D:\\Games\\COM3D2\\Mod",
        "--write-root",
        "work=D:\\MeidoWork"
      ]
    }
  }
}
```

请把可执行文件和目录替换为本机实际路径。

- `--root mods=...` 创建名为 `mods` 的只读目录；
- `--write-root work=...` 创建名为 `work` 的可读写目录。

### 使用 gRPC 服务（其他编程语言使用，含强类型）

```powershell
# 便捷模式；input.path 可读取服务进程账号有权限访问的任意普通文件
MeidoSerialization.exe serve grpc

# 受限模式；本地文件必须使用 file { root_id, relative_path }
MeidoSerialization.exe serve grpc --root mods=D:\Games\COM3D2\Mod
```

不提供 `--root` 或 `--restrict-paths` 时，gRPC 使用 unrestricted 便捷模式，`ArtifactInput.path`
可使用服务端本地绝对路径，或相对于服务进程当前目录的路径。提供其中任意参数后会切换到 restricted 模式，拒绝直接路径，已配置
root 始终只读。当前模式由 `GetCapabilities.filesystem_mode` 返回。 转换结果仍以内联数据或 blob 返回；gRPC 不会在服务端安装输出文件。

传输层的完整说明位于 [docs/transport-api.md](docs/transport-api.md)。inline artifact bundle 上限为 3 MiB；更大的数据使用受配额限制的临时
blob，归档列表使用分页。KCES raw Unity `.bytes` 输入输出会将 相邻 metadata 与 TypeTree sidecar 作为同一个 artifact bundle
保留。舞蹈数据的传输格式 ID 为
`com3d2.timeline` 和 `com3d2.object_data`。

#### Schema-first 编辑

需要强类型的调用方无需先转换样本文件。

先调用 gRPC `GetCapabilities`，选择`has_editing_schema: true` 的格式，再使用其 `format_id` 调用 `GetFormatSchema`。

响应以`application/schema+json` 字节返回完整 Draft 2020-12 文档。

或者直接从仓库中获取
schema.json：[位置](https://github.com/MeidoPromotionAssociation/MeidoSerialization/tree/main/schemas/editing/v1)

调用方可用直接使用 JSON Schema 代码生成器，例如 [quicktype](https://github.com/glideapps/quicktype)、
`json-schema-to-typescript`、NJsonSchema 或 `typify`。

来生产可用的代码并得到结构，调用方还应校验 `sha256`，按`schema_id` 与 `schema_version` 缓存

<details>

<summary>详细说明</summary>

生成出的类型描述 `Validate` 与 `Convert` 接受的 editing JSON，包括多态 `oneOf` 分支、递归
`$defs`、base64 字节字段和标准精确整数范围。MCP 客户端可从
`meido://schemas/{format_id}` 获取相同内容。`.arc` 等 native-only 格式没有 editing Schema， 不会被伪装成强类型 editing
文档。JSON Schema 负责结构契约；编辑完成后仍应调用 `Validate`， 在转换前执行跨字段和 native-wire 不变量检查。

需要语义编辑帮助时，应在获取 Schema 后调用 `GetFormatGuide`。Guide 会把 JSON path 和 Schema pointer 映射到游戏用途、编辑
role、risk、不变量、enum 含义与 evidence。同一套认证模型也会写入 JSON Schema：根节点包含 `x-meido-format-verification`，经过审阅的
property 会在 title、 description、game usage 与 editing guidance 旁包含 `x-meido-verification`。脚本类格式还可以发布 结构化
command form、位置参数、目标 build 注记和共享游戏常量 `value_sets`；参数通过
`value_set_refs` 精确引用应使用的 enum 或 slot 表。

文件级 `format_verification.level` 只有 `serialization_verified` 和 `schema_only` 两个值。
`serialization_verified` 表示已经核对整个文件的序列化契约；`schema_only` 表示目前只知道生成出的
结构，不代表每个字段的游戏含义都已确认。完成的文件认证记录 `authority: ai` 或
`authority: human`；不属于认证的 `schema_only` 基线使用 `authority: generated`。

字段认证是相互独立的结构化 claim。`serialization` 只确认格式、位置或读写方式，不声明游戏含义；
`source_semantics` 表示已对照游戏源码确认用途或消费路径，并且包含序列化认证；`game_behavior`
必须有实际游戏运行观察，单纯核对源码不会产生该 claim。每个存在的 claim 都有
`status: verified` 和明确的 `authority: ai|human`。空 `verification` 对象表示字段仅由 Schema 派生，调用方必须保留它，也不得根据字段名推断行为。
`field_coverage` 只统计数量，不会提升任何字段。 每个已发布的 editing format 都有签入仓库的 profile。标准流程为
Capabilities -> Schema -> Guide ->
skill/`meido.edit_format` -> inspect -> 最小修改 -> Validate -> Convert。

MCP 会把 portable 工作流作为 `meido://skills/editing/{format_id}` 的动态 `text/markdown` 资源公开； 它不会自动安装为
WorkBuddy skill 或 MCP Host 插件，单独读取也不能替代 Schema 与 Guide。
`meido.edit_format` Prompt 会一次返回 skill、完整 Schema 与完整 Guide，是更方便的入口，但它只准备 上下文，不会自行编辑文件。应先通过文件级
`format_verification` 判断序列化契约是否已知，再查看目标 字段自己的 `verification` claim，之后才能解释其游戏含义。

</details>

### 在 Golang 项目中使用

```powershell
go get github.com/MeidoPromotionAssociation/MeidoSerialization@latest
```

公开实现按游戏拆分为两组 package：

- `service/COM3D2` 与 `service/KCES` 提供基于路径的读取、写入和转换服务
- `serialization/COM3D2` 与 `serialization/KCES` 提供强类型 wire 编解码器

<details>

<summary>使用示例</summary>

```go
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"

	serialcom "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/COM3D2"
	com3d2service "github.com/MeidoPromotionAssociation/MeidoSerialization/service/COM3D2"
)

func main() {
	// 示例1：使用 service 包直接处理文件
	// 创建一个用于处理材质文件的服务
	mateService := &com3d2service.MateService{}
	ctx := context.Background()
	const maxOutputBytes int64 = 64 << 20

	// 将二进制材质文件转换为 JSON
	err := mateService.ConvertMateToJson(ctx, "example.mate", "example.mate.json", maxOutputBytes)
	if err != nil {
		fmt.Printf("转换材质文件时出错：%v\n", err)
		return
	}

	// 将 JSON 文件转换回二进制材质格式
	err = mateService.ConvertJsonToMate(ctx, "example.mate.json", "new_example.mate", maxOutputBytes)
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

## FAQ

### ImageMagick 问题

如果在处理纹理（.tex）文件时遇到错误：

- 确保已安装 ImageMagick 7.0 或更高版本
- 验证 ImageMagick 在您的系统 PATH 中（您应该能够从任何终端运行 'magick' 命令）
- 安装 ImageMagick 后重启应用程序

### 关于 1011 版本的 .tex

- 新增字段：
    - 1011 版本在二进制结构中新增 `Rects`（纹理图集）数组，元素为 `x, y, w, h` 四个 `float32`，表示归一化 UV 空间内的矩形。
- 将图片转换为 `.tex` 时：
    - 若同目录存在同名的 `.uv.csv`（如 `foo.png.uv.csv`），会读取其中的矩形并生成 1011 版本的 tex。
    - 若不存在 `.uv.csv`，则生成 1010 版本（不含 `Rects`）。
- 将 `.tex` 转换为图片时:
    - 若源 `.tex` 为 1011 且包含 `Rects`，在输出图片旁会生成同名 `.uv.csv`（如 `output.png.uv.csv`）
- .uv.csv 格式：
    - 编码必须为：UTF-8-BOM。
    - 分隔符：英文逗号`,`。
    - 列数：每行 4 列，依次为 `x, y, w, h`（x, y, width, height）；取值通常位于区间 `[0, 1]`（归一化 UV），建议保留最多 6
      位小数，精度为
      `float32`。
    - 示例：
  ```csv
  x,y,w,h
  0.000000,0.000000,0.500000,0.500000
  0.500000,0.000000,0.500000,0.500000
  0.000000,0.500000,0.500000,0.500000
  ```

### 在 `.nei` 文件中使用某些字符时无法保存

如果您遇到下面的错误，这是因为您使用了 Shift-JIS 编码不支持的字符。 .nei 文件内部使用 Shift-JIS 编码，我们对此无能为力。请删除不支持的字符。

- `failed to write to .neiData file: failed to encode string: encoding: rune not supported by encoding.`
- `failed to write to .nei file: failed to encode string: encoding: rune not supported by encoding.`

### 关于 CSV 格式

本程序中使用的所有 CSV 文件均采用 UTF-8-BOM 编码，以 `,`
分隔，并遵循 [RFC4180](https://datatracker.ietf.org/doc/html/rfc4180) 标准。

## 许可证

本项目采用 BSD-3-Clause License 许可 - 详情请参阅 LICENSE 文件。

## 也可以看看其他仓库

- [COM3D2 MOD 编辑器](https://github.com/MeidoPromotionAssociation/COM3D2_MOD_EDITOR)
- [COM3D2 文件批量转换器及序列化库](https://github.com/MeidoPromotionAssociation/MeidoSerialization)
- [COM3D2 简明中文 MOD 教程](https://github.com/MeidoPromotionAssociation/COM3D2_Simple_MOD_Guide_Chinese)
- [另一个 COM3D2 翻译插件 JAT](https://github.com/MeidoPromotionAssociation/COM3D2.JustAnotherTranslator.Plugin)
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

# 日本語

# MeidoSerialization

## はじめに

MeidoSerialization は Go で書かれた、[KISS](https://www.kisskiss.tv) ゲームの独自ファイル形式を扱う
シリアライズ・形式変換ツールセットです。従来の [CM3D2](https://www.kisskiss.tv/cm3d2/) と
[COM3D2](https://com3d2.jp/) に加え、COM3D2.5 と CRC3D3 で使用される後継の
[KCES](https://kces.jp/) キャラクター編集システム形式にも対応します。同じ実装を Go package、CLI、versioned gRPC API、MCP
stdio server から利用できます。

<br>

このプロジェクトが役に立った場合は、Star を付けていただけると幸いです。

Bug 報告や機能要望は GitHub Issues または Discussions を利用してください。

Discord の [Custom Maid Server](https://discord.gg/custommaid) でも連絡できます。

質問やフィードバックはグループ内で行い、direct message は送らないでください。

## 機能

- COM3D2/CM3D2 と KCES data の読み取り、検証、書き出し
- 対応 native 形式と厳密な editing JSON の双方向変換
- 内容による形式判定と、単一ファイルまたはディレクトリの一括変換
- COM3D2 ARC と KCES CT/ABA container の一覧、抽出、unpack、pack
- KCES Texture2D、Sprite、AudioClip、Mesh、AnimationClip data の出力
- COM3D2 TEX の変換
- versioned protobuf/gRPC interface による共通変換機能
- AI による編集のための MCP stdio 対応
- strongly typed tool と AI-assisted editing のための Draft 2020-12 Schema と source-reviewed field Guide

KCES、gRPC、MCP のサポートは MeidoSerialization v2.0.0 以降で利用できます。

## 対応形式

互換性は現在 COM3D2 v2.48.0、COM3D2.5 v3.48.0、KCES 1.34.4 を基準に確認しています。

特記がない限り、以下の「JSON」は CLI、service、gRPC、MCP で共通の厳密な editing JSON を意味します。

### CM3D2 / COM3D2

| ファイルまたは拡張子                                     | 内容                     | 対応操作                 | 備考                                                             |
|----------------------------------------------------------|--------------------------|--------------------------|------------------------------------------------------------------|
| `.menu`                                                  | メニュースクリプト       | native ↔ JSON            | 現在判明している全 version                                       |
| `.mate`、`.mat`                                          | マテリアルファイル       | native ↔ JSON            | COM3D2.5 専用 field を含み、`.mat` は alias として使用可能       |
| `.pmat`                                                  | 描画順序                 | native ↔ JSON            | 現在判明している全 version                                       |
| `.col`                                                   | コライダー               | native ↔ JSON            | 現在判明している全 version                                       |
| `.phy`                                                   | 物理パラメータ           | native ↔ JSON            | 現在判明している全 version                                       |
| `.psk`                                                   | スカート専用物理         | native ↔ JSON            | KCES と共有。version 217 以降は構造変更なし                      |
| `.anm`                                                   | アニメーション           | native ↔ JSON            | 現在判明している全 version                                       |
| `.model`                                                 | モデル                   | native ↔ JSON            | version 1000–2200                                                |
| `.preset`                                                | キャラクタープリセット   | native ↔ JSON            | 現在判明している全 version。埋め込み KCES data を保持            |
| `timeline_data.bytes`                                    | ダンスタイムライン       | native ↔ JSON            | transport format ID は `com3d2.timeline`                         |
| `maid_data.bytes`、`item_data.bytes`、`event_data.bytes` | ダンスオブジェクトデータ | native ↔ JSON            | format ID は `com3d2.object_data`。内容から subtype を判定       |
| `.tex`                                                   | テクスチャ               | TEX ↔ 画像               | version 1000 は read-only。書き出しは 1010/1011、DXT1/DXT5 対応  |
| `.nei`                                                   | 暗号化 CSV               | NEI ↔ CSV                | KCES と共有。native text は Shift-JIS、CSV I/O は BOM 付き UTF-8 |
| `.arc`                                                   | アーカイブ               | 一覧、抽出、pack、unpack | 暗号化 ARC は非対応                                              |
| `.save`                                                  | セーブデータ             | 判定のみ                 | 変換機能は提供しない                                             |

### KCES / COM3D2.5

| ファイルまたは拡張子                            | 内容                                         | 対応操作                          | 備考                                                                                 |
|-------------------------------------------------|----------------------------------------------|-----------------------------------|--------------------------------------------------------------------------------------|
| `.ct`                                           | VirtualDirectory/content table               | JSON 変換と archive 操作          | CT catalog の検査と directory の pack/unpack に対応                                  |
| `.aba`                                          | UnityFS AssetBundle                          | 一覧、pack、unpack                | `packAba` は対応する `.aba` + `.ct` を生成。暗号化 `abap` は復号不可                 |
| `.asset_bg`、`.asset_scene`                     | UnityFS AssetBundle                          | 一覧、抽出、unpack                | ABA と同一形式                                                                       |
| `.menuassets`                                   | メニューファイル container                   | native ↔ JSON                     |                                                                                      |
| `.materialassets`                               | マテリアルファイル container                 | native ↔ JSON                     |                                                                                      |
| `.pmatassets`                                   | 描画順序ファイル container                   | native ↔ JSON                     |                                                                                      |
| `.model`                                        | mesh geometry を含まないモデルデータ         | native ↔ JSON                     |                                                                                      |
| `.dbconf`                                       | 物理・コライダー payload                     | native ↔ JSON                     |                                                                                      |
| `.dbcol`                                        | 物理・コライダー payload                     | native ↔ JSON                     |                                                                                      |
| `.db2conf`                                      | 物理・コライダー payload                     | native ↔ JSON                     |                                                                                      |
| `.dsbconf`                                      | 物理・コライダー payload                     | native ↔ JSON                     |                                                                                      |
| `.dsb2conf`                                     | 物理・コライダー payload                     | native ↔ JSON                     |                                                                                      |
| `.dslconf`                                      | 物理・コライダー payload                     | native ↔ JSON                     |                                                                                      |
| `.dsl2conf`                                     | 物理・コライダー payload                     | native ↔ JSON                     |                                                                                      |
| `.dslcol`                                       | 物理・コライダー payload                     | native ↔ JSON                     |                                                                                      |
| `.ikcol`                                        | 物理・コライダー payload                     | native ↔ JSON                     |                                                                                      |
| `.ikcol.bytes`                                  | 物理・コライダー payload                     | native ↔ JSON                     |                                                                                      |
| `.limbcol`                                      | 物理・コライダー payload                     | native ↔ JSON                     |                                                                                      |
| `.hitcheck`                                     | hit-check data                               | native ↔ editing JSON             |                                                                                      |
| `.undressdat`                                   | hit-check data                               | native ↔ editing JSON             | native 形式自体が JSON                                                               |
| `.undresspdat`                                  | hit-check data                               | native ↔ editing JSON             | native 形式自体が JSON                                                               |
| `.nson`                                         | binary JSON variant                          | native ↔ editing JSON             | native 形式自体が JSON                                                               |
| `.preset`                                       | キャラクタープリセット                       | native ↔ JSON                     |                                                                                      |
| `system.dat`                                    | システム状態                                 | native ↔ editing JSON             |                                                                                      |
| `paths.dat`                                     | リソース検索 path                            | native ↔ editing JSON             |                                                                                      |
| `bridge_session.vd`                             | ゲーム間 bridge transfer file                | native ↔ JSON                     |                                                                                      |
| `.brd`                                          | bridge、name map、attachment、collider state | native ↔ JSON                     |                                                                                      |
| `.enm`                                          | bridge、name map、attachment、collider state | native ↔ JSON                     |                                                                                      |
| `.sad`                                          | bridge、name map、attachment、collider state | native ↔ JSON                     |                                                                                      |
| `maid_collider.bytes`                           | bridge、name map、attachment、collider state | native ↔ JSON                     |                                                                                      |
| raw Unity `.bytes` object と隣接 sidecar        | Unity serialized object                      | raw object ↔ JSON                 | `.meta.json` と任意の `.typetree.json` を primary file と同じ artifact bundle で転送 |
| native Texture2D、Sprite object file            | 画像                                         | Texture2D → PNG/DDS、Sprite → PNG | 一方向変換                                                                           |
| native Mesh `.mmesh`、AnimationClip object file | 3D・アニメーションデータ                     | → glTF 2.0/GLB                    | 一方向変換。一部 data は `.model` に残るため、出力は preview 向け                    |
| native AudioClip object file                    | encode 済み音声                              | inline payload を無劣化抽出       | 一方向変換。OGG、WAV、FSB5 signature を認識し、transcode は行わない                  |

serializer 実装は [`serialization/COM3D2`](serialization/COM3D2) と
[`serialization/KCES`](serialization/KCES) にあります。

application-level capability の正式な一覧は gRPC `GetCapabilities` または MCP の
`meido://capabilities` resource から取得してください。

## 参考資料

- この library は当初 [COM3D2_MOD_EDITOR](https://github.com/90135/COM3D2_MOD_EDITOR) 用に開発され、後に再利用しやすいよう
  独立しました。同 project の使用方法も参考にできます。
- API reference：[pkg.go.dev](https://pkg.go.dev/github.com/MeidoPromotionAssociation/MeidoSerialization)
- 自動生成された project overview：[DeepWiki](https://deepwiki.com/MeidoPromotionAssociation/MeidoSerialization)
  （AI hallucination を含む可能性があります）
- AI agent のインストールと基本操作：[docs/ai-agent.md](docs/ai-agent.md)

## 必要環境

- build 済み CLI の利用に Go toolchain は不要
- source からの build には `go.mod` と同じ Go 1.26.5 以上が必要
- COM3D2 `.tex` の画像変換には ImageMagick 7 以上と、`PATH` から実行できる `magick` が必要

## 使用方法

### CLI として使用

[GitHub Releases](https://github.com/MeidoPromotionAssociation/MeidoSerialization/releases) から build 済み executable を
ダウンロードしてください。

[完全な CLI ドキュメント](cmd/README.md)も参照してください。

**CLI クイックスタート**

```powershell
# ファイルを判定して変換、またはディレクトリ内の対応ファイルを再帰処理
MeidoSerialization.exe convert .\example.menu
MeidoSerialization.exe convert .\mods

# native/editing JSON を明示的に変換
MeidoSerialization.exe convert2json .\example.menu
MeidoSerialization.exe convert2mod .\example.menu.json

# KCES container を検査・unpack
MeidoSerialization.exe listCt .\example.ct
MeidoSerialization.exe listAba .\example.aba
MeidoSerialization.exe unpackAba .\example.aba -o .\unpacked

# 単体 native Unity object を出力
MeidoSerialization.exe convert2image .\texture.texture2d.bytes
MeidoSerialization.exe convert2gltf .\mesh.mesh.bytes --format glb
MeidoSerialization.exe convert2audio .\voice.audioclip.bytes
```

command group：

- 変換と判定：`convert`、`convert2json`、`convert2mod`、`determine`
- 画像、model、animation、audio：`convert2tex`、`convert2image`、`convert2gltf`、`convert2audio`
- NEI/CSV：`convert2csv`、`convert2nei`
- COM3D2 ARC：`listArc`、`extractArc`、`packArc`、`unpackArc`
- KCES CT/ABA：`listCt`、`packCt`、`unpackCt`、`listAba`、`packAba`、`unpackAba`
- KCES MOD workflow：`inspectKcesCatalog`、`packKcesMod`
- API：`serve grpc`、`mcp`
- utility：`version`、`completion`

現在の flag と例は `MeidoSerialization.exe --help` または `<command> --help` で確認できます。完全な command reference は
[cmd/README.md](cmd/README.md) にあります。`--strict`（`-s`）は内容に基づく形式判定を有効にし、`--type`（`-t`）は native type
または `<type>.json` で directory 操作を絞り込みます。

### MCP service を使用

転送モードは常に `stdio` です。MCP Host は `MeidoSerialization.exe mcp` を子プロセスとして起動し、MCP プロトコルメッセージを
stdin/stdout で交換します。診断ログは stderr に出力されます。SSE、HTTP、Streamable HTTP エンドポイントは提供しません。以下の
`restricted` と `unrestricted` はファイルシステムのアクセスモードであり、転送モードではありません。Host
の画面で転送モードの選択を求められた場合は、`stdio` を選択してください。

server を起動します：

```powershell
# MCP convenience mode。direct path は process account の filesystem permission を使用
MeidoSerialization.exe mcp

# MCP restricted mode。path tool は root ID を使用
MeidoSerialization.exe mcp --root mods=D:\Games\COM3D2\Mod --write-root work=D:\MeidoWork
```

次の message を agent に送ると install と設定を依頼できます：

```text
次のドキュメントに従って MeidoSerialization MCP をインストールし、設定してください：
https://github.com/MeidoPromotionAssociation/MeidoSerialization/blob/main/docs/ai-agent.md

接続後は meido://capabilities を読み、ドキュメントに従って対応する Schema、Guide、editing skill を取得してください。
```

手動設定では、読み書き可能な directory を制限しない次の convenience mode の例から始められます。

設定ファイルの場所と外側の構造は Host ごとに異なります。transport mode は stdio で、`command` と `args` が stdio
子プロセスを起動します。一般的な設定例：

```json
{
  "mcpServers": {
    "meido-serialization": {
      "command": "D:\\Tools\\MeidoSerialization.exe",
      "args": [
        "mcp"
      ]
    }
  }
}
```

読み書き可能な directory を制限する場合：

```json
{
  "mcpServers": {
    "meido-serialization": {
      "command": "D:\\Tools\\MeidoSerialization.exe",
      "args": [
        "mcp",
        "--root",
        "mods=D:\\Games\\COM3D2\\Mod",
        "--write-root",
        "work=D:\\MeidoWork"
      ]
    }
  }
}
```

実行ファイルと directory は実際のローカルパスに置き換えてください。

- `--root mods=...` は `mods` という名前の読み取り専用 directory を作成します。
- `--write-root work=...` は `work` という名前の読み書き可能な directory を作成します。

### gRPC service を使用（他のプログラミング言語・strongly typed client 向け）

```powershell
# convenience mode。input.path は server process account が許可する任意の regular file を読み取り可能
MeidoSerialization.exe serve grpc

# restricted mode。local file は file { root_id, relative_path } を使用
MeidoSerialization.exe serve grpc --root mods=D:\Games\COM3D2\Mod
```

`--root` と `--restrict-paths` のどちらも指定しない場合、gRPC は unrestricted convenience mode を使用し、
`ArtifactInput.path` に server-local の absolute path、または server process の current directory からの relative path
を指定できます。いずれかを指定すると restricted mode に切り替わり、direct path は拒否され、configured root は常に
read-only です。現在の mode は `GetCapabilities.filesystem_mode` で確認できます。conversion result は引き続き inline または
blob で返され、gRPC は server に output file を install しません。

transport layer の完全な説明は [docs/transport-api.md](docs/transport-api.md) にあります。gRPC inline artifact bundle は最大
3 MiB で、それより大きい data は quota-bounded temporary blob を使用し、archive listing は pagination されます。KCES raw
Unity `.bytes` の input/output は隣接する metadata と TypeTree sidecar を一つの artifact bundle として保持します。dance
transport ID は `com3d2.timeline` と `com3d2.object_data` です。

#### Schema-first editing

strongly typed definition を必要とする caller は、最初に sample file を変換する必要がありません。

まず gRPC `GetCapabilities` を呼び、`has_editing_schema: true` の format を選び、その `format_id` で `GetFormatSchema` を呼びます。

response は完全な Draft 2020-12 document を `application/schema+json` bytes として返します。

または repository から checked-in Schema JSON を直接取得できます：
[場所](https://github.com/MeidoPromotionAssociation/MeidoSerialization/tree/main/schemas/editing/v1)

Schema を [quicktype](https://github.com/glideapps/quicktype)、`json-schema-to-typescript`、NJsonSchema、`typify` などの JSON
Schema code generator に渡せます。

これにより利用可能な code と structure を生成できます。caller は `sha256` も検証し、`schema_id` と `schema_version` で cache
してください。

<details>

<summary>詳細説明</summary>

生成された type は `Validate` と `Convert` が受け付ける editing JSON を記述し、polymorphic `oneOf` branch、recursive
`$defs`、base64 byte field、標準の exact integer range を含みます。MCP client は `meido://schemas/{format_id}`
から同じ内容を取得できます。`.arc` などの native-only format は editing Schema を持たず、typed editing document として
公開されません。JSON Schema は structural contract を扱います。編集後は `Validate` を呼び、変換前に cross-field と
native-wire invariant を確認してください。

semantic editing の支援には、Schema の後で `GetFormatGuide` を取得します。Guide は JSON path と Schema pointer を game usage、
edit role、risk、invariant、enum meaning、evidence に対応付けます。同じ verification model は JSON Schema にも入り、root には
`x-meido-format-verification`、review 済み property には title、description、game usage、editing guidance とともに
`x-meido-verification` があります。script-like format は structured command form、位置引数、target-build note、共有 game
constant `value_sets` も公開でき、argument の `value_set_refs` が使用すべき enum または slot table を特定します。

whole-file `format_verification.level` は `serialization_verified` と `schema_only` の2値です。前者は file の serialization
contract を確認済みであること、後者は生成された構造だけが既知であることを表し、全 field の game meaning を保証しません。完了した
format verification は `authority: ai` または `authority: human` を記録し、認証ではない `schema_only` baseline は
`authority: generated` を使用します。

field verification は独立した structured claim です。`serialization` は format、位置、read/write behavior を確認しますが、game
meaning は主張しません。`source_semantics` は game source で用途または consumption path を確認し、serialization verification を
含みます。`game_behavior` には実際の game-runtime observation が必要で、source review だけでは付与されません。存在する claim は
`status: verified` と明示的な `authority: ai|human` を持ちます。空の `verification` object は Schema 由来だけであるため、値を
保持し、名前から behavior を推測してはいけません。`field_coverage` は件数 summary にすぎず、個別 field を昇格させません。公開済みの
各 editing format には checked-in profile があります。標準 workflow は Capabilities -> Schema -> Guide ->
skill/`meido.edit_format` -> inspect -> 最小編集 -> Validate -> Convert です。

MCP は portable workflow を `meido://skills/editing/{format_id}` の dynamic `text/markdown` resource として公開します。これは
WorkBuddy skill や MCP-host plugin として自動 install されず、単独で読んでも Schema と Guide の代わりにはなりません。
`meido.edit_format` Prompt は skill、完全な Schema、完全な Guide を一度に返す便利な入口ですが、context を準備するだけで file を
編集しません。whole-file `format_verification` で serialization contract が既知か確認し、その後に exact field 自身の
`verification` claim を読んでから game meaning を割り当ててください。

</details>

### Go project で使用

```powershell
go get github.com/MeidoPromotionAssociation/MeidoSerialization@latest
```

公開実装は game ごとに2つの package family に分かれています：

- `service/COM3D2` と `service/KCES` は path-based read、write、conversion service を提供
- `serialization/COM3D2` と `serialization/KCES` は typed wire encoder/decoder を提供

<details>

<summary>使用例</summary>

```go
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"

	serialcom "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/COM3D2"
	com3d2service "github.com/MeidoPromotionAssociation/MeidoSerialization/service/COM3D2"
)

func main() {
	// 例1：service package を使用して file を直接処理
	// material file 用 service を作成
	mateService := &com3d2service.MateService{}
	ctx := context.Background()
	const maxOutputBytes int64 = 64 << 20

	// binary material file を JSON に変換
	err := mateService.ConvertMateToJson(ctx, "example.mate", "example.mate.json", maxOutputBytes)
	if err != nil {
		fmt.Printf("material file の変換中にエラー：%v\n", err)
		return
	}

	// JSON を binary material format に戻す
	err = mateService.ConvertJsonToMate(ctx, "example.mate.json", "new_example.mate", maxOutputBytes)
	if err != nil {
		fmt.Printf("JSON から material file への変換中にエラー：%v\n", err)
	}

	// 例2：serialization package を使用して structure を直接操作
	// .phy file を読み込む。完全な file handling は service package の例を参照
	f, err := os.Open("example.phy")
	if err != nil {
		fmt.Printf("file を開けません：%v\n", err)
		return
	}
	defer f.Close()

	br := bufio.NewReader(f)
	phyData, err := serialcom.ReadPhy(br)
	if err != nil {
		fmt.Printf(".phy file の解析に失敗：%v\n", err)
		return
	}

	phyData.Damping = 0.8

	newFile, err := os.Create("modified.phy")
	if err != nil {
		fmt.Printf("新しい file の作成に失敗：%v\n", err)
		return
	}
	defer newFile.Close()

	bw := bufio.NewWriter(newFile)
	if err := phyData.Dump(bw); err != nil {
		fmt.Printf(".phy file の書き込みに失敗：%v\n", err)
		return
	}
	if err := bw.Flush(); err != nil {
		fmt.Printf("buffer の flush 中にエラー：%v\n", err)
		return
	}

	fmt.Println("すべての操作が正常に完了しました！")
}
```

</details>

## よくある質問

### ImageMagick の問題

テクスチャ（.tex）ファイルの処理中にエラーが発生した場合：

- ImageMagick 7.0 以上がインストールされていることを確認
- ImageMagick がシステム PATH に含まれていることを確認（任意のターミナルから 'magick' コマンドを実行できる必要があります）
- ImageMagick インストール後にアプリケーションを再起動

### .tex ファイルのバージョン 1011 について

- 新しいフィールド：
    - バージョン 1011 では、バイナリ構造に `Rects`（テクスチャアトラス）配列が追加されました。要素は `x, y, w, h` の4つの
      `float32` で、正規化された UV 空間内の矩形を表します。
- 画像を `.tex` に変換する場合：
    - 同じディレクトリに同名の `.uv.csv` ファイルが存在する場合（例：`foo.png.uv.csv`）、その中の矩形を読み込み、バージョン
      1011 の tex ファイルを生成します。
    - `.uv.csv` ファイルが存在しない場合は、バージョン 1010（`Rects` なし）を生成します。
- `.tex` を画像に変換する場合：
    - ソースの `.tex` がバージョン 1011 で `Rects` を含む場合、出力画像の横に同名の `.uv.csv` ファイルが生成されます（例：
      `output.png.uv.csv`）
- .uv.csv 形式：
    - エンコーディング：UTF-8-BOM 必須
    - 区切り文字：英語のカンマ `,`
    - 列数：各行 4 列、順番は `x, y, w, h`（x, y, 幅, 高さ）。値は通常 `[0, 1]` の範囲（正規化された UV）。小数点以下最大 6
      桁、精度は `float32` を推奨。
    - 例：
  ```csv
  x,y,w,h
  0.000000,0.000000,0.500000,0.500000
  0.500000,0.000000,0.500000,0.500000
  0.000000,0.500000,0.500000,0.500000
  ```

### `.nei` ファイルで特定の文字を使用すると保存できない

以下のエラーが発生した場合、Shift-JIS エンコーディングでサポートされていない文字を使用しています。 .nei ファイルは内部的に
Shift-JIS エンコーディングを使用しており、これについては対処できません。サポートされていない文字を削除してください。

- `failed to write to .neiData file: failed to encode string: encoding: rune not supported by encoding.`
- `failed to write to .nei file: failed to encode string: encoding: rune not supported by encoding.`

### CSV 形式について

本プログラムで使用されるすべての CSV ファイルは、UTF-8-BOM エンコーディングで、`,`
で区切られ、[RFC4180](https://datatracker.ietf.org/doc/html/rfc4180) 標準に準拠しています。

## ライセンス

このプロジェクトは BSD-3-Clause License の下でライセンスされています - 詳細は LICENSE ファイルを参照してください。

## 他のリポジトリもチェック

- [COM3D2 MODエディタ](https://github.com/MeidoPromotionAssociation/COM3D2_MOD_EDITOR)
- [COM3D2 バッチファイルコンバータツールおよびシリアル化ライブラリ](https://github.com/MeidoPromotionAssociation/MeidoSerialization)
- [COM3D2 中国語 MOD チュートリアル](https://github.com/MeidoPromotionAssociation/COM3D2_Simple_MOD_Guide_Chinese)
- [COM3D2 翻訳プラグイン JAT](https://github.com/MeidoPromotionAssociation/COM3D2.JustAnotherTranslator.Plugin)
- [90135 の COM3D2 中国語ガイド](https://github.com/90135/COM3D2_GUIDE_CHINESE)
- [90135 の COM3D2 スクリプトコレクション](https://github.com/90135/COM3D2_Scripts_901)
- [90135 の COM3D2 ツール](https://github.com/90135/COM3D2_Tools_901)

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

# How to Dev

1. Install [Go](https://go.dev/) 1.26.5 or later
2. Clone this repository and change to the project root
3. Run `go test ./...`
4. Build the CLI with `go build -o MeidoSerialization.exe .`

If a public editing model changes, regenerate the checked-in JSON Schemas with
`go run ./internal/schemagen/cmd -out ./schemas/editing/v1`. If the protobuf API changes, edit
[`api/proto/meido/serialization/v1/serialization.proto`](api/proto/meido/serialization/v1/serialization.proto) and
regenerate the checked-in bindings as described
in [docs/transport-api.md](docs/transport-api.md#regenerating-protobuf-code).

<br>

# KISS Rule

[KISS](https://www.kisskiss.tv/) is the company/brand that makes these games.

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
    This tool is a local-first data processing tool with no content generation capabilities whatsoever. Its optional self-hosted gRPC transport can transfer data only between the caller and the process operated by the user; the Author operates no upload/download service.
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
      - The optional gRPC transport is self-hosted by the user. The Author receives no transferred data, and all content processing remains on the device running that process.
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
   本工具为本地优先的数据处理工具，不具备任何内容生成能力。可选的自托管 gRPC 传输仅在调用方与用户自行运行的进程之间传输数据；作者不运营任何上传下载服务。
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
     - 可选的 gRPC 传输由用户自行托管，作者不会接收任何传输数据；所有内容处理仍在运行该进程的用户设备上完成
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
   本ツールはローカル優先のデータ処理ツールであり、いかなるコンテンツ生成能力も有していません。任意のセルフホスト gRPC 転送は、利用者が運用する呼び出し元とプロセスの間でのみデータを転送し、作者はいかなるアップロード・ダウンロードサービスも運営しません。
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
     - 任意の gRPC 転送は利用者自身がホストします。作者が転送データを受信することはなく、すべてのコンテンツ処理は当該プロセスを実行する利用者のデバイス上で完了します。
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
