[English](#english) | [日本語](#日本語) | [简体中文](#简体中文)

# English

# MeidoSerialization CLI

MeidoSerialization CLI exposes the repository's COM3D2/CM3D2 and KCES serializers as ordinary file commands, a versioned
protobuf/gRPC service, and an MCP stdio server. It can convert one file or recursively process a directory. For the
complete format matrix, see the [main README](../README.md#supported-formats).

Converted JSON may be compact or formatted depending on the source format. Whitespace is not part of the editing
contract, so a JSON formatter may be used for readability. Files produced by this CLI can also be opened by
[COM3D2 MOD EDITOR V2](https://github.com/MeidoPromotionAssociation/COM3D2_MOD_EDITOR) where that editor supports the
format.

## Download and requirements

- Download a prebuilt executable
  from [GitHub Releases](https://github.com/MeidoPromotionAssociation/MeidoSerialization/releases)
- Building from source requires Go 1.26.5 or later, matching `go.mod`
- COM3D2 TEX/image conversion requires ImageMagick 7 or later, with `magick` available on `PATH`

All examples below use PowerShell and assume `MeidoSerialization.exe` is available in the current directory or on
`PATH`. Prefix it with `.\` when PowerShell requires an explicit current-directory path.

## Start here

### Common conversions

```powershell
# Native game file -> adjacent editing JSON
MeidoSerialization.exe convert2json .\example.menu
# Output: .\example.menu.json

# Editing JSON -> adjacent native game file
MeidoSerialization.exe convert2mod .\example.menu.json
# Output: .\example.menu

# Let the CLI choose the direction
MeidoSerialization.exe convert .\example.menu
MeidoSerialization.exe convert .\example.menu.json

# Identify a file before changing it
MeidoSerialization.exe determine --strict .\unknown_file
```

The conversion commands normally write next to the input and may replace an existing file with the derived output name.
Keep a backup when editing valuable game data.

A complete, low-risk editing round trip looks like this:

```powershell
# 1. Back up the native file
Copy-Item .\body.menu .\body.menu.bak

# 2. Confirm what the CLI detects
MeidoSerialization.exe determine --strict .\body.menu

# 3. Create editing JSON
MeidoSerialization.exe convert2json .\body.menu

# 4. Change body.menu.json with an editor you trust
notepad .\body.menu.json

# 5. Encode it back; the default output is body.menu
MeidoSerialization.exe convert2mod .\body.menu.json
```

| Input example           | Command         | Default output                                                      |
|-------------------------|-----------------|---------------------------------------------------------------------|
| `example.menu`          | `convert2json`  | `example.menu.json`                                                 |
| `example.menu.json`     | `convert2mod`   | `example.menu`                                                      |
| `texture.tex`           | `convert2image` | `texture.png`                                                       |
| `image.png`             | `convert2tex`   | `image.tex`                                                         |
| `Texture2D\my_tex.png`  | `convert2texture2d` | native KCES `my_tex.tex`                                        |
| `body.mmesh`            | `convert2gltf`  | `body.glb`                                                          |
| `dress.model`           | `convert2gltf`  | `dress.glb` with the skeleton, skin, and morphs from its `.mmesh`   |
| `dress.glb`             | `gltf2model`    | `dress.model` and `dress.mmesh`                                     |
| `voice.audioclip`       | `convert2audio` | `voice.ogg`, `.wav`, or `.fsb` according to its signature           |
| `table.nei`             | `convert2csv`   | `table.csv`                                                         |
| `table.csv`             | `convert2nei`   | `table.nei`                                                         |
| `example.arc`           | `unpackArc`     | `example.arc_unpacked\`                                             |
| `example.ct`            | `convert`       | `example.ct.json`                                                   |
| `example.aba`           | `unpackAba`     | `example.aba_unpacked\`                                             |
| `example.aba`           | `genCt`         | `example.ct`                                                        |

### Batch conversion

Pass a directory instead of a file to process matching files recursively:

```powershell
# Convert every supported native editing format below the directory
MeidoSerialization.exe convert2json .\mods

# Convert only native KCES .dbconf files
MeidoSerialization.exe convert2json --type dbconf .\mods

# Convert only .menu editing JSON files back to native
MeidoSerialization.exe convert2mod --type menu.json .\editing

# Inspect every recognized file using content-based filtering
MeidoSerialization.exe determine --strict .\mods
```

Most dedicated directory commands use a worker pool. A bad file is printed as an error while the remaining files
continue, so review the complete console output after a batch operation. The generic `convert` command processes
directories sequentially and is more convenient for mixed input, but slower than the dedicated commands.

## File conversion commands

### Native formats and editing JSON

| Command                            | Purpose                                                                           |
|------------------------------------|-----------------------------------------------------------------------------------|
| `convert2json <file-or-directory>` | Convert supported COM3D2/KCES native data to adjacent editing JSON                |
| `convert2mod <file-or-directory>`  | Remove the final `.json` suffix and encode editing JSON back to the native format |
| `convert <file-or-directory>`      | Auto-select native ↔ JSON, TEX ↔ image, NEI ↔ CSV, or unpack a single ARC         |
| `determine <file-or-directory>`    | Report game, file type, representation, signature, version, and size              |

Examples:

```powershell
# COM3D2
MeidoSerialization.exe convert2json .\body.menu
MeidoSerialization.exe convert2json .\body.mate
MeidoSerialization.exe convert2mod .\body.mate.json

# KCES parts and physics/collider payloads
MeidoSerialization.exe convert2json .\parts\hair.menuassets
MeidoSerialization.exe convert2json .\physics\default_hairf.dbconf
MeidoSerialization.exe convert2mod .\physics\default_hairf.dbconf.json

# KCES containers and state files
MeidoSerialization.exe convert2json .\character.perset
MeidoSerialization.exe convert2json .\system.dat
MeidoSerialization.exe convert2json .\catalog.ct

# Standalone native Unity object; the file's embedded TypeTree is preserved by the JSON envelope
MeidoSerialization.exe convert2json .\Texture2D\body.tex
MeidoSerialization.exe convert2mod .\Texture2D\body.tex.json
```

`convert2json` does not convert `.tex`; use `convert2image`. It also does not unpack `.aba`; use `unpackAba`. The
generic `convert` unpacks a single `.arc`, but directory ARC workflows should use `unpackArc`.

### Images and COM3D2 TEX

```powershell
# COM3D2 TEX -> PNG (default)
MeidoSerialization.exe convert2image .\texture.tex

# COM3D2 TEX -> any ImageMagick-supported output format
MeidoSerialization.exe convert2image .\texture.tex --format webp

# Native KCES Texture2D -> PNG or DDS
MeidoSerialization.exe convert2image .\Texture2D\body.tex --format png
MeidoSerialization.exe convert2image .\Texture2D\body.tex --format dds

# Native KCES Sprite -> PNG only
MeidoSerialization.exe convert2image .\Sprite\icon.sprite --format png

# Image -> COM3D2 TEX, preserving PNG payload by default
MeidoSerialization.exe convert2tex .\texture.png

# Image -> TEX with DXT compression; --compress disables force-PNG automatically
MeidoSerialization.exe convert2tex .\texture.png --compress

# PNG or JPEG -> native KCES Texture2D (rebuilt as inline RGBA32)
MeidoSerialization.exe convert2texture2d .\my_texture.png
```

`convert2tex` writes TEX version 1010 unless a valid sibling `.uv.csv` supplies version-1011 atlas rectangles. See
the [TEX 1011 FAQ](../README.md#about-version-1011-of-the-tex-file). `--forcePng=false` can be used explicitly;
`--compress` takes precedence.

### KCES Model, Mesh, AnimationClip, and AudioClip

These commands operate on KCES `.model` files and standalone native Unity object files with an embedded TypeTree,
typically extracted from an ABA by this library:

```powershell
# Model plus its referenced .mmesh -> complete skinned binary glTF 2.0 (default)
# The skeleton, bone weights, morph targets, and material names are included, and
# KCES-only fields are stored in the kcesModel extras for the reverse conversion
MeidoSerialization.exe convert2gltf .\TextAsset\dress.model

# glTF or GLB -> KCES .model and .mmesh (a Blender export works when it has one triangle mesh)
MeidoSerialization.exe gltf2model .\dress.glb -o .\out

# Standalone Mesh or AnimationClip -> binary glTF 2.0 (default)
MeidoSerialization.exe convert2gltf .\Mesh\body.mmesh
MeidoSerialization.exe convert2gltf .\dance.animationclip.bytes

# JSON glTF with an embedded data URI
MeidoSerialization.exe convert2gltf .\Mesh\body.mmesh --format gltf

# Extract inline encoded AudioClip data without transcoding
MeidoSerialization.exe convert2audio .\AudioClip\voice.audioclip

# Batch-export all matching objects below a directory
MeidoSerialization.exe convert2gltf .\unpacked
MeidoSerialization.exe convert2audio .\unpacked
```

Model conversion is bidirectional: `convert2gltf` looks up the `.mmesh` next to the `.model` or in the sibling
`Mesh` directory of an unpacked ABA, and `gltf2model` writes an official Unity 2022.3 native Mesh that packs
straight back into an ABA. A glTF scene without a skin is bound rigidly to its mesh node through a synthesized
single-bone skin.

Blender import tip: in the glTF importer's `Bones & Skin` panel, uncheck `Guess Original Bind Pose` and set
`Bone Dir` to "Temperance" to get a clean octahedral skeleton. KCES bind poses are baked against a body-scaled
skeleton and intentionally differ from the node rest pose, and Blender's guess drops the scale component while
reconstructing it, which derails the whole armature of clothing models where nearly every node is a joint.
With the default settings every joint also displays as a small icosphere by design; accessories with few joints
import the rest of the skeleton as plain-axis empties instead. Keep the default "Blender" `Bone Dir` only when
the most accurate re-export round trip through `gltf2model` matters more than the viewport display.

Material appearance is intentionally not converted in either direction. KCES materials are entries packed inside
the bundle's `.materialassets` container, each carrying a virtual file name such as `crc_dress044_shoe.mate` along
with game-specific shader parameters and texture references that have no glTF equivalent, so `convert2gltf`
exports each sub-mesh material as a name-only placeholder, and `gltf2model` ignores every glTF PBR parameter and
texture and stores only each material's name in the Model. At runtime the game appends `.mate` to an extensionless
name, hashes it ignoring case, and looks it up among the registered `.materialassets` entries; a name that matches
nothing logs `CreateMaterial not found material` and leaves that sub-mesh invisible. Therefore name every glTF
material after the target material entry (for example `crc_dress044_shoe`), and author the entry itself through
the existing `.materialassets` JSON workflow.

Animation export supports explicit rotation, position, scale, and Euler curves with Transform paths. Audio export
recognizes OGG, WAV, and FSB5 signatures and chooses the corresponding suffix; it does not transcode audio.

### NEI and CSV

```powershell
MeidoSerialization.exe convert2csv .\table.nei
MeidoSerialization.exe convert2nei .\table.csv
```

NEI text is Shift-JIS. CSV files are read and written as RFC 4180-style comma-separated UTF-8 with BOM. A character that
cannot be encoded in Shift-JIS causes `convert2nei` to fail instead of silently replacing it.

## Archive commands

### COM3D2 ARC

| Command                          | Purpose                                                  |
|----------------------------------|----------------------------------------------------------|
| `listArc <file>`                 | List every stored path                                   |
| `unpackArc <file-or-directory>`  | Unpack complete ARC files                                |
| `packArc <directory>`            | Pack a directory while preserving its relative structure |
| `extractArc <file-or-directory>` | Extract selected entries by extension or exact path/name |

```powershell
# Browse and unpack
MeidoSerialization.exe listArc .\game.arc
MeidoSerialization.exe unpackArc .\game.arc
MeidoSerialization.exe unpackArc .\game.arc -o .\game_files

# Repack a directory; default output is <directory-name>.arc
MeidoSerialization.exe packArc .\game_files
MeidoSerialization.exe packArc .\game_files -o .\custom.arc

# Extract all .menu files
MeidoSerialization.exe extractArc .\game.arc --ext menu

# Extract one exact path
MeidoSerialization.exe extractArc .\game.arc --file menu\parts\body.menu -o .\selected

# A bare filename works only when it is unique inside the ARC
MeidoSerialization.exe extractArc .\game.arc --file body.menu
```

Exactly one of `--ext` or `--file` is required. `--file` is only available for one ARC, not a directory of ARCs. Full
stored paths are matched first; bare filenames are matched case-insensitively and rejected when ambiguous. Encrypted ARC
files are not supported.

### KCES CT and ABA

| Command                         | Purpose                                                             |
|---------------------------------|---------------------------------------------------------------------|
| `listCt <file-or-directory>`    | List virtual files stored in CT/VirtualDirectory containers         |
| `genCt <file-or-directory>`     | Generate the companion .ct catalog from a .aba file                 |
| `listAba <file-or-directory>`   | List Unity objects with PathID, type, size, and name                |
| `unpackAba <file-or-directory>` | Extract supported UnityFS assets into type directories              |
| `packAba <directory>`           | Scan a plain resource directory and create a matching ABA + CT pair |

```powershell
# CT
MeidoSerialization.exe listCt .\my_mod.ct
MeidoSerialization.exe genCt .\my_mod.aba

# ABA
MeidoSerialization.exe listAba .\my_mod.aba
MeidoSerialization.exe unpackAba .\my_mod.aba -o .\aba_files

# Creates my_mod.aba and my_mod.ct in the parent directory of .\aba_files
MeidoSerialization.exe packAba .\aba_files -o my_mod
```

For `packAba`, `--output` is a base name, not an output directory or full filename. With no `--output`, the input
directory name is used. The packer targets the library's canonical Unity 2022.3.35f1 layout. Encrypted `abap` bundles
can be detected but not decrypted.

`.ct` files are lookup tables (catalog plus ExtensionNameList data), so they are not unpacked into directories.
To view one, use `listCt` or `inspectKcesCatalog`; to edit one, use the `convert` command, which round-trips a
`.ct` through an editable `.ct.json` envelope.

### KCES MOD editing workflow

`packAba` builds the packing manifest automatically from the directory contents, and `genCt` rebuilds the catalog
for an existing `.aba`. To edit a texture, convert it to PNG, edit it, then convert it back so repacking picks up
the change:

```powershell
MeidoSerialization.exe unpackAba .\my_mod.aba -o .\aba_files
MeidoSerialization.exe convert2image .\aba_files\Texture2D\my_texture.tex
# edit my_texture.png, then rebuild the native Texture2D in place
MeidoSerialization.exe convert2texture2d .\aba_files\Texture2D\my_texture.png
MeidoSerialization.exe packAba .\aba_files -o my_mod
```

The game reads parts definitions only from a file named exactly `<aba_name>.menuassets` (lowercase), so the
output name passed to `-o` must match the `.menuassets` file inside the package, or rename the `menuassets`, and cannot be the same as an existing filename.. When packing an unpack
directory without `-o`, the default output name strips the `.aba_unpacked` suffix so the original name is kept,
and `packAba` prints a warning when the names do not match.

`packAba` and `genCt` write default catalog metadata (`catalogType` Parts, `packageType` Plugin, priority 0).
To customize the metadata, edit the generated `.ct` through its JSON envelope:

```powershell
MeidoSerialization.exe convert .\my_mod.ct        # produces my_mod.ct.json
# edit catalog fields such as catalogType, packageType, priority, subName
MeidoSerialization.exe convert .\my_mod.ct.json   # writes my_mod.ct back
```

`inspectKcesCatalog` prints the `AssetBundleCatalog` and `ExtensionNameList` data from a CT:

```powershell
MeidoSerialization.exe inspectKcesCatalog .\existing_mod.ct
```

## Global flags

`--strict` (`-s`) asks type filtering and `determine` to validate content instead of relying only on names and
extensions. `--type` (`-t`) limits directory operations to one native type or its editing JSON form.

```powershell
# Leading dots are optional
MeidoSerialization.exe convert2json --type .menu .\mods

# Native only
MeidoSerialization.exe convert2json --type dbconf .\mods

# Editing JSON only
MeidoSerialization.exe convert2mod --type dbconf.json .\editing

# Strict content-based matching
MeidoSerialization.exe determine --strict --type preset .\presets
```

Common filters include COM3D2 types (`menu`, `mate`, `pmat`, `col`, `phy`, `psk`, `anm`, `model`, `preset`,
`tex`, `nei`, `arc`, `bytes`), generic types (`image`, `csv`), and KCES types such as `menuassets`,
`materialassets`, `pmatassets`, `dbconf`, `dbcol`, `db2conf`, `dsbconf`, `dsb2conf`, `dslconf`, `dsl2conf`,
`dslcol`, `ikcol`, `ikcol.bytes`, `limbcol`, `hitcheck`, `undressdat`, `undresspdat`, `nson`, `ct`, `aba`,
`bridge_session`, `brd`, `enm`, `sad`, `system`, `paths`, and `maid_collider`.

Use `MeidoSerialization.exe <command> --help` for the flags registered by the current build.

## gRPC server

### Start a local server

```powershell
# Unrestricted convenience mode for a trusted local client
MeidoSerialization.exe serve grpc --listen 127.0.0.1:50051

# Restricted root-ID mode
MeidoSerialization.exe serve grpc `
  --listen 127.0.0.1:50051 `
  --root mods=C:\Games\COM3D2\Mod
```

With no `--root` or `--restrict-paths`, the server reports `FILESYSTEM_MODE_UNRESTRICTED` and accepts direct
server-local `input.path` values. Supplying either option reports `FILESYSTEM_MODE_RESTRICTED`; direct paths are then
rejected and local files must use a configured root. `--restrict-paths` with no roots denies all server-local file
inputs while inline and blob inputs remain available.

The versioned service is `meido.serialization.v1.SerializationService`. Server reflection and the standard gRPC health
service are enabled. If [grpcurl](https://github.com/fullstorydev/grpcurl) is installed, discovery can be checked with:

```powershell
grpcurl -plaintext 127.0.0.1:50051 list
grpcurl -plaintext -d '{}' 127.0.0.1:50051 meido.serialization.v1.SerializationService/GetCapabilities
```

The following examples detect the same file in convenience and restricted modes:

```powershell
# Convenience mode: absolute path on the gRPC server machine
grpcurl -plaintext `
  -d '{"input":{"path":"C:\\Games\\COM3D2\\Mod\\menu\\sample.menu"}}' `
  127.0.0.1:50051 meido.serialization.v1.SerializationService/Detect

# Restricted mode: public root ID plus a path beneath that root
grpcurl -plaintext `
  -d '{"input":{"file":{"rootId":"mods","relativePath":"menu\\sample.menu"}}}' `
  127.0.0.1:50051 meido.serialization.v1.SerializationService/Detect
```

Registered RPCs:

| RPC                                  | Purpose                                                                                           |
|--------------------------------------|---------------------------------------------------------------------------------------------------|
| `GetCapabilities`                    | Discover filesystem mode, root IDs, formats, limits, and Schema/Guide metadata                    |
| `GetFormatSchema`                    | Return the Draft 2020-12 editing JSON Schema for one convertible format                           |
| `GetFormatGuide`                     | Return the source-reviewed semantic Guide associated with that Schema                             |
| `Detect`                             | Detect a native or editing JSON artifact                                                          |
| `Convert`                            | Convert native ↔ editing JSON                                                                     |
| `Validate`                           | Apply the published Schema, cross-field rules, and native serializer validation                   |
| `Upload`, `Download`, `DeleteBlob`   | Manage process-local, TTL-limited large temporary blobs                                           |
| `ListArchive`, `ExtractArchiveEntry` | Page through or extract ARC, CT/VirtualDirectory, ABA, `.asset_bg`, and `.asset_scene` containers |

An input artifact uses exactly one source: inline bytes with a filename, a server-issued blob ID, unrestricted
`path`, or restricted `file { root_id, relative_path }`. Direct and rooted inputs automatically discover adjacent
`.meta.json` and `.typetree.json`; inline/blob callers submit those supported attachments explicitly. The inline limit
is shared by the complete primary/sidecar bundle. Results that do not fit inline are returned as blob references.

Important security and storage behavior:

- No root flags select unrestricted mode; `path` may be absolute or relative to the server process working directory
- `--root id=directory` is repeatable, read-only, and automatically selects restricted mode
- `--restrict-paths` explicitly selects restricted mode; direct `path` inputs are rejected
- Root-relative paths cannot be absolute, contain `..`, use a volume name, or escape the selected root
- The default listener is `127.0.0.1:50051`
- Non-loopback listeners require `--allow-remote`, which still provides no TLS, authentication, or authorization;
  combining it with unrestricted mode lets remote clients read any regular file allowed by the server process account
- gRPC never installs converted output into a local path or root; results remain inline or blob-based
- Default limits are 4 GiB per blob, 16 GiB total, 4096 blobs, a 30-minute TTL, and 3 MiB inline per artifact bundle
- `--blob-dir` is exclusively locked for the server lifetime; a second process using it fails before cleanup
- Archive pages default to 128 entries and accept at most 1000 entries per request

Relevant flags are `--root`, `--restrict-paths`, `--max-blob-mib`, `--max-total-blob-mib`, `--max-blobs`, `--blob-ttl`,
`--inline-mib`, `--blob-dir`, and `--allow-remote`. The inline limit cannot exceed 3 MiB. See the complete
[transport API reference](../docs/transport-api.md).

## MCP stdio server

### Add it to an MCP host

MCP runs over stdio as a child process; it does not expose SSE, HTTP, or Streamable HTTP endpoints. If the Host presents
a transport selector, choose `stdio`. Host configuration formats vary; this generic JSON example uses restricted roots
and is the recommended starting point:

```json
{
  "mcpServers": {
    "meido-serialization": {
      "command": "C:\\Tools\\MeidoSerialization.exe",
      "args": [
        "mcp",
        "--root",
        "mods=C:\\Games\\COM3D2\\Mod",
        "--write-root",
        "work=C:\\MeidoWork"
      ]
    }
  }
}
```

For a trusted local host, the shortest launch is:

```powershell
MeidoSerialization.exe mcp
```

| Mode           | How it is selected                                  | Read arguments              | Write arguments                           | Security boundary                                      |
|----------------|-----------------------------------------------------|-----------------------------|-------------------------------------------|--------------------------------------------------------|
| `unrestricted` | No path-related flags                               | `path`                      | `output_path`                             | Any regular file allowed by the server process account |
| `restricted`   | Any `--root`, `--write-root`, or `--restrict-paths` | `root_id` + `relative_path` | `output_root_id` + `output_relative_path` | Configured roots only                                  |

`--root` is read-only. `--write-root` is writable and can also be read. `--restrict-paths` with no roots intentionally
denies all file access. Only MCP protocol messages go to stdout; diagnostics go to stderr.

### MCP tools

| Tool                          | Purpose                                                                                  |
|-------------------------------|------------------------------------------------------------------------------------------|
| `meido.detect_file`           | Detect format ID, version, representation, and file metadata                             |
| `meido.inspect_file`          | Convert a reasonably small native file to inline editing JSON for inspection             |
| `meido.validate_editing_json` | Validate inline or file-based editing JSON with Schema plus the native serializer        |
| `meido.convert_file`          | Convert native/editing JSON and atomically install the primary file and managed sidecars |
| `meido.list_archive`          | Return one bounded page of exact archive entry names                                     |
| `meido.extract_archive_entry` | Extract one exact listed entry to the authorized destination                             |

`--max-result-mib` defaults to 2 MiB and limits inline inspect/list results. Use `meido.convert_file` for a larger JSON
document. `--max-write-mib` defaults to 512 MiB for the combined primary/sidecar output bundle. Writes are staged,
size/hash checked, and rolled back as a bundle if installation fails.

### MCP resources, Prompt, and portable editing skill

| Entry point                          | What it returns                                                                                             |
|--------------------------------------|-------------------------------------------------------------------------------------------------------------|
| `meido://capabilities`               | Active filesystem mode, root IDs, writable root IDs, limits, format capabilities, and Schema/Guide metadata |
| `meido://schemas/{format_id}`        | Exact Draft 2020-12 structural contract for editing JSON                                                    |
| `meido://guides/{format_id}`         | Field inventory, semantic evidence, edit roles, risks, invariants, commands, and value sets                 |
| `meido://skills/editing/{format_id}` | Portable Markdown editing workflow plus the current filesystem write policy                                 |
| `meido.edit_format`                  | Prompt that combines an objective with the rendered skill and embeds the exact Schema and Guide             |

The portable skill is an MCP `text/markdown` resource, not an automatically installed Codex/host plugin. Reading the
skill resource alone does not replace reading the Schema and Guide: it links to both and defines the preservation,
validation, and write workflow. `meido.edit_format` is the convenient option because it returns the skill text and the
complete Schema and Guide in the same Prompt result. The Prompt prepares context; it does not edit a file by itself.

The verification data is split by scope:

| Scope | Values or claims | Meaning |
|---|---|---|
| Whole-file `format_verification.level` | `serialization_verified`, `schema_only` | Whether the file's serialization contract, or only its generated structure, is known |
| Field `verification.serialization` | `status: verified`, `authority: ai\|human` | Format, position, or read/write behavior is checked; game meaning is not implied |
| Field `verification.source_semantics` | `status: verified`, `authority: ai\|human` | Purpose or consumption path is checked against game source and includes serialization verification |
| Field `verification.game_behavior` | `status: verified`, `authority: ai\|human` | Behavior is confirmed from an actual game-runtime observation |

`schema_only` is not a certification and therefore uses `authority: generated`. An empty field `verification` object
means schema-derived only: preserve the value and do not infer behavior from its name. `field_coverage` is a count
summary, not a way to promote a field. The Schema mirrors this information through root
`x-meido-format-verification` and property-level `x-meido-verification` annotations.

For example, this field has source-reviewed semantics, but not an actual in-game behavior observation:

```json
{
  "format_verification": {
    "level": "serialization_verified",
    "authority": "ai"
  },
  "field_coverage": {
    "total": 42,
    "serialization_verified": 42,
    "source_verified": 8,
    "game_behavior_verified": 0,
    "schema_derived": 0
  },
  "fields": [
    {
      "json_path": "/example",
      "verification": {
        "serialization": {"status": "verified", "authority": "ai"},
        "source_semantics": {"status": "verified", "authority": "ai"}
      }
    }
  ]
}
```

The skill requires the model/client to:

1. Read `meido://capabilities` and choose a format with `has_editing_schema` and `has_format_guide`
2. Read and verify the Schema and Guide, including matching `schema_id`
3. Detect and inspect the actual input rather than constructing a replacement document from defaults
4. Make only the requested change and preserve ordering, duplicates, null/empty distinctions, integer width, raw fields,
   identifiers, hashes, unknown future data, and base64 bytes
5. Follow reviewed `edit_role`, invariants, command ordering, and `value_set_refs`; never invent game constants
6. Submit the complete document to `meido.validate_editing_json`
7. Convert only after validation succeeds and follow the write policy rendered into the skill
8. Treat visual/behavioral correctness as requiring in-game verification; Schema/native validation cannot prove it

`meido.edit_format` requires `format_id` and `objective`. Its optional path arguments depend on the active mode:

| Mode           | Optional input/output Prompt arguments                                           |
|----------------|----------------------------------------------------------------------------------|
| `unrestricted` | `input_path`, `output_path`                                                      |
| `restricted`   | `input_root_id`, `input_relative_path`, `output_root_id`, `output_relative_path` |

Example objective for a restricted MCP host:

```text
Prompt: meido.edit_format
format_id: com3d2.menu
objective: Change only the menu display name and preserve every unrelated command
input_root_id: mods
input_relative_path: menu/parts/example.menu
output_root_id: work
output_relative_path: menu/parts/example.menu
```

Native-only/detect-only formats such as `com3d2.arc` or `com3d2.tex` do not expose an editing Schema, Guide, skill, or
edit Prompt workflow. Always discover capabilities instead of guessing a resource URI.

## Build from source

```powershell
git clone https://github.com/MeidoPromotionAssociation/MeidoSerialization.git
Set-Location MeidoSerialization
go build -o MeidoSerialization.exe .
```

Other useful commands:

```powershell
MeidoSerialization.exe version
MeidoSerialization.exe completion powershell
MeidoSerialization.exe <command> --help
```

---

# 简体中文

# MeidoSerialization 命令行工具

MeidoSerialization CLI 把仓库中的 COM3D2/CM3D2 与 KCES 序列化功能做成了普通文件命令，同时提供版本化的 protobuf/gRPC 服务和
MCP stdio 服务。既可以处理单个文件，也可以递归处理整个目录。完整的格式清单请查看[主 README](../README.md#支持的格式)。

转换得到的 JSON 可能是紧凑格式，也可能带有缩进，这取决于原格式。空白不属于编辑协议，可以放心使用 JSON
格式化工具改善可读性。对于编辑器已经支持的格式，也可以使用
[COM3D2 MOD EDITOR V2](https://github.com/MeidoPromotionAssociation/COM3D2_MOD_EDITOR)
打开本工具生成的文件。

## 下载和运行前准备

- 普通用户可以从 [GitHub Releases](https://github.com/MeidoPromotionAssociation/MeidoSerialization/releases) 下载预编译的
  Windows 可执行文件
- 从源码构建需要 Go 1.26.5 或更高版本，与 `go.mod` 保持一致
- COM3D2 TEX 与普通图片互转需要 ImageMagick 7 或更高版本，并确保 `magick` 命令已加入 `PATH`

以下示例均使用 PowerShell。示例假设 `MeidoSerialization.exe` 位于当前目录，因此使用 `.\`
前缀；如果已经把它加入 `PATH`，也可以省略该前缀。路径中有空格时，请用双引号包住完整路径。

## 从这里开始

### 最常用的转换

不知道该用哪个命令时，可以先看这四条：

~~~powershell
# 原生游戏文件 -> 同目录下的编辑 JSON
.\MeidoSerialization.exe convert2json .\example.menu
# 输出：.\example.menu.json

# 编辑 JSON -> 同目录下的原生游戏文件
.\MeidoSerialization.exe convert2mod .\example.menu.json
# 输出：.\example.menu

# 让工具根据输入自动选择转换方向
.\MeidoSerialization.exe convert .\example.menu
.\MeidoSerialization.exe convert .\example.menu.json

# 不确定文件类型时，先按内容识别
.\MeidoSerialization.exe determine --strict .\unknown_file
~~~

转换命令通常把结果写到输入文件旁边，并可能覆盖同名的既有输出文件。编辑重要游戏数据前，建议先备份。下面是一个可以直接照着操作的完整流程：

~~~powershell
# 1. 备份原文件
Copy-Item .\body.menu .\body.menu.bak

# 2. 确认工具识别到的格式
.\MeidoSerialization.exe determine --strict .\body.menu

# 3. 转成可编辑 JSON
.\MeidoSerialization.exe convert2json .\body.menu

# 4. 使用熟悉的编辑器修改 body.menu.json
notepad .\body.menu.json

# 5. 转回原生文件；默认会写回 body.menu
.\MeidoSerialization.exe convert2mod .\body.menu.json
~~~

常见默认输出名如下：

| 输入示例                | 命令            | 默认输出                                                 |
|-------------------------|-----------------|----------------------------------------------------------|
| `example.menu`          | `convert2json`  | `example.menu.json`                                      |
| `example.menu.json`     | `convert2mod`   | `example.menu`                                           |
| `texture.tex`           | `convert2image` | `texture.png`                                            |
| `image.png`             | `convert2tex`   | `image.tex`                                              |
| `Texture2D\my_tex.png`  | `convert2texture2d` | 原生 KCES `my_tex.tex`                               |
| `body.mmesh`            | `convert2gltf`  | `body.glb`                                               |
| `dress.model`           | `convert2gltf`  | 含其 `.mmesh` 骨架、蒙皮与 morph 的 `dress.glb`          |
| `dress.glb`             | `gltf2model`    | `dress.model` 与 `dress.mmesh`                           |
| `voice.audioclip`       | `convert2audio` | 根据数据签名输出 `voice.ogg`、`.wav` 或 `.fsb`           |
| `table.nei`             | `convert2csv`   | `table.csv`                                              |
| `table.csv`             | `convert2nei`   | `table.nei`                                              |
| `example.arc`           | `unpackArc`     | `example.arc_unpacked\`                                  |
| `example.ct`            | `convert`       | `example.ct.json`                                        |
| `example.aba`           | `unpackAba`     | `example.aba_unpacked\`                                  |
| `example.aba`           | `genCt`         | `example.ct`                                             |

### 批量转换目录

把目录而不是单个文件传给命令，就会递归处理其中匹配的文件：

~~~powershell
# 把目录下所有支持编辑的原生格式转成 JSON
.\MeidoSerialization.exe convert2json .\mods

# 只转换原生 KCES .dbconf 文件
.\MeidoSerialization.exe convert2json --type dbconf .\mods

# 只把 .menu 编辑 JSON 转回原生文件
.\MeidoSerialization.exe convert2mod --type menu.json .\editing

# 按文件内容严格识别目录中的所有已知文件
.\MeidoSerialization.exe determine --strict .\mods
~~~

大多数专用目录命令会并发处理文件。遇到一个坏文件时，它会打印错误并继续处理其余文件，所以批量任务结束后要检查完整的控制台输出。泛用
`convert` 命令按顺序处理目录，适合混合输入，但通常比专用命令慢。

## 文件转换命令

### 原生格式与编辑 JSON

| 命令                        | 用途                                                       |
|-----------------------------|------------------------------------------------------------|
| `convert2json <文件或目录>` | 把支持的 COM3D2/KCES 原生数据转换为相邻的编辑 JSON         |
| `convert2mod <文件或目录>`  | 去掉末尾的 `.json`，把编辑 JSON 编码回原生格式             |
| `convert <文件或目录>`      | 自动选择原生 ↔ JSON、TEX ↔ 图片、NEI ↔ CSV，或解包单个 ARC |
| `determine <文件或目录>`    | 报告游戏、文件类型、表示形式、签名、版本和大小             |

示例：

~~~powershell
# COM3D2
.\MeidoSerialization.exe convert2json .\body.menu
.\MeidoSerialization.exe convert2json .\body.mate
.\MeidoSerialization.exe convert2mod .\body.mate.json

# KCES 部件、物理与碰撞数据
.\MeidoSerialization.exe convert2json .\parts\hair.menuassets
.\MeidoSerialization.exe convert2json .\physics\default_hairf.dbconf
.\MeidoSerialization.exe convert2mod .\physics\default_hairf.dbconf.json

# KCES 容器与状态文件
.\MeidoSerialization.exe convert2json .\character.perset
.\MeidoSerialization.exe convert2json .\system.dat
.\MeidoSerialization.exe convert2json .\catalog.ct

# 独立的 Unity 原生对象；JSON envelope 会保留文件内嵌的 TypeTree 信息
.\MeidoSerialization.exe convert2json .\Texture2D\body.tex
.\MeidoSerialization.exe convert2mod .\Texture2D\body.tex.json
~~~

`convert2json` 不负责 `.tex`，请使用 `convert2image`；也不会解包 `.aba`，请使用
`unpackAba`。泛用 `convert` 可以解包单个 `.arc`，但处理目录中的 ARC 时应使用
`unpackArc`。

### 图片与 COM3D2 TEX

~~~powershell
# COM3D2 TEX -> PNG（默认）
.\MeidoSerialization.exe convert2image .\texture.tex

# COM3D2 TEX -> ImageMagick 支持的其他图片格式
.\MeidoSerialization.exe convert2image .\texture.tex --format webp

# KCES 原生 Texture2D -> PNG 或 DDS
.\MeidoSerialization.exe convert2image .\Texture2D\body.tex --format png
.\MeidoSerialization.exe convert2image .\Texture2D\body.tex --format dds

# KCES 原生 Sprite 目前只输出 PNG
.\MeidoSerialization.exe convert2image .\Sprite\icon.sprite --format png

# 图片 -> COM3D2 TEX；默认保留 PNG 数据
.\MeidoSerialization.exe convert2tex .\texture.png

# 图片 -> 使用 DXT 压缩的 TEX；--compress 会自动关闭强制 PNG
.\MeidoSerialization.exe convert2tex .\texture.png --compress

# PNG 或 JPEG -> 原生 KCES Texture2D（重建为内联 RGBA32）
.\MeidoSerialization.exe convert2texture2d .\my_texture.png
~~~

`convert2tex` 默认写出 TEX 1010。只有旁边存在有效的 `.uv.csv` 图集矩形信息时，才会写出
1011。详情见 [TEX 1011 常见问题](../README.md#关于-1011-版本的-tex)。也可以显式使用 `--forcePng=false`；`--compress`
的优先级更高。

### KCES Model、Mesh、AnimationClip 与 AudioClip

这些命令处理 KCES `.model` 文件和带内嵌 TypeTree 的独立 Unity 原生对象，后者通常来自本库解包的 ABA：

~~~powershell
# Model 及其引用的 .mmesh -> 完整蒙皮的二进制 glTF 2.0（默认）
# 包含骨架、蒙皮权重、变形目标和材质名，KCES 专有字段保存在 kcesModel extras 中供反向转换使用
.\MeidoSerialization.exe convert2gltf .\TextAsset\dress.model

# glTF 或 GLB -> KCES .model 与 .mmesh（Blender 导出的单三角网格场景可直接转换）
.\MeidoSerialization.exe gltf2model .\dress.glb -o .\out

# 独立 Mesh 或 AnimationClip -> 二进制 glTF 2.0（默认）
.\MeidoSerialization.exe convert2gltf .\Mesh\body.mmesh
.\MeidoSerialization.exe convert2gltf .\dance.animationclip.bytes

# 输出 JSON glTF，并把数据作为 data URI 内嵌
.\MeidoSerialization.exe convert2gltf .\Mesh\body.mmesh --format gltf

# 提取 AudioClip 中内嵌的编码音频，不进行转码
.\MeidoSerialization.exe convert2audio .\AudioClip\voice.audioclip

# 批量导出目录下所有匹配对象
.\MeidoSerialization.exe convert2gltf .\unpacked
.\MeidoSerialization.exe convert2audio .\unpacked
~~~

Model 转换是双向的：`convert2gltf` 会在 `.model` 同目录或 ABA 解包目录的同级 `Mesh`
目录中查找 `.mmesh`；`gltf2model` 写出官方 Unity 2022.3 原生 Mesh，可直接重新打包进 ABA。
不带蒙皮的 glTF 场景会合成单骨骼蒙皮，把网格刚性绑定到其挂载节点。

Blender 导入提示：在 glTF 导入器的 `Bones & Skin` 面板中，取消勾选 `Guess Original Bind Pose`（猜测原始绑定姿态），
并把 `Bone Dir` 设为 "Temperance"，即可得到干净的八面锥骨架。KCES 的 bindpose 是在带体型缩放的骨架下烘焙的，
与节点 rest 姿态本就不同，而 Blender 反推绑定姿态时会丢弃缩放分量——服装模型几乎每个节点都是关节，
整个骨架会因此错乱。默认设置下每个关节还会刻意显示为小棱角球；关节很少的配饰则把骨架其余节点导入为十字轴
empty。只有当经 `gltf2model` 再导出的往返精度比视口显示更重要时，才保留默认的 "Blender" `Bone Dir`。

材质外观在两个方向上都刻意不转换。KCES 材质是打包在资源包 `.materialassets` 容器中的条目，每个条目带有形如
`crc_dress044_shoe.mate` 的虚拟文件名，以及游戏专有的着色器参数和贴图引用，在 glTF 中没有对应物，因此
`convert2gltf` 只为每个子网格导出一个仅有名字的占位材质，`gltf2model` 会忽略 glTF 的全部 PBR
参数和贴图，只把每个材质的名字写入 Model。游戏运行时会给无扩展名的名字补上 `.mate`，做忽略大小写的哈希后在已注册的
`.materialassets` 条目中查找；名字对不上会输出 `CreateMaterial not found material`
错误日志，对应子网格渲染缺失。因此每个 glTF 材质都要按目标材质条目命名（例如
`crc_dress044_shoe`），材质条目本体请通过现有的 `.materialassets` JSON 流程制作。

动画导出支持带 Transform 路径的显式旋转、位置、缩放和欧拉曲线。音频导出根据 OGG、WAV 或 FSB5
数据签名选择后缀，只提取现有编码数据，不会把音频转成另一种编码。

### NEI 与 CSV

~~~powershell
.\MeidoSerialization.exe convert2csv .\table.nei
.\MeidoSerialization.exe convert2nei .\table.csv
~~~

NEI 文本使用 Shift-JIS。CSV 按类似 RFC 4180 的逗号分隔格式读写，编码为带 BOM 的 UTF-8。如果 CSV 中的字符无法编码为
Shift-JIS，`convert2nei` 会直接报错，不会静默替换成错误字符。

## 归档命令

### COM3D2 ARC

| 命令                      | 用途                                |
|---------------------------|-------------------------------------|
| `listArc <文件>`          | 列出 ARC 中保存的全部路径           |
| `unpackArc <文件或目录>`  | 完整解包 ARC                        |
| `packArc <目录>`          | 保持目录相对结构并打包为 ARC        |
| `extractArc <文件或目录>` | 按扩展名或精确路径/文件名选择性提取 |

~~~powershell
# 查看内容并完整解包
.\MeidoSerialization.exe listArc .\game.arc
.\MeidoSerialization.exe unpackArc .\game.arc
.\MeidoSerialization.exe unpackArc .\game.arc -o .\game_files

# 重新打包目录；默认输出为 <目录名>.arc
.\MeidoSerialization.exe packArc .\game_files
.\MeidoSerialization.exe packArc .\game_files -o .\custom.arc

# 提取所有 .menu 文件
.\MeidoSerialization.exe extractArc .\game.arc --ext menu

# 按 ARC 内的完整路径提取一个文件
.\MeidoSerialization.exe extractArc .\game.arc --file menu\parts\body.menu -o .\selected

# 只写文件名时，该名称在 ARC 中必须唯一
.\MeidoSerialization.exe extractArc .\game.arc --file body.menu
~~~

`--ext` 与 `--file` 必须且只能选择一个。`--file` 只支持单个 ARC，不支持输入 ARC
目录。工具优先匹配归档内完整路径；裸文件名不区分大小写，但重名时会拒绝提取，避免选错。目前不支持加密 ARC。

### KCES CT 与 ABA

| 命令                     | 用途                                       |
|--------------------------|--------------------------------------------|
| `listCt <文件或目录>`    | 列出 CT/VirtualDirectory 容器中的虚拟文件  |
| `genCt <文件或目录>`     | 从 .aba 文件生成配套的 .ct catalog         |
| `listAba <文件或目录>`   | 列出 Unity 对象的 PathID、类型、大小与名称 |
| `unpackAba <文件或目录>` | 按类型目录提取 UnityFS 中支持的资源        |
| `packAba <目录>`         | 扫描普通资源目录并生成配套的 ABA + CT      |

~~~powershell
# CT
.\MeidoSerialization.exe listCt .\my_mod.ct
.\MeidoSerialization.exe genCt .\my_mod.aba

# ABA
.\MeidoSerialization.exe listAba .\my_mod.aba
.\MeidoSerialization.exe unpackAba .\my_mod.aba -o .\aba_files

# 在 .\aba_files 的父目录生成 my_mod.aba 与 my_mod.ct
.\MeidoSerialization.exe packAba .\aba_files -o my_mod
~~~

`packAba --output`/`-o` 表示“输出基础名称”，不是输出目录，也不是完整文件名。省略时会使用输入目录名。打包器以本库规范化的
Unity 2022.3.35f1 布局为目标。工具可以识别加密
`abap` bundle，但无法解密。

`.ct` 是查找表（catalog 与 ExtensionNameList 数据），因此不再解包成目录。查看请使用 `listCt` 或
`inspectKcesCatalog`；编辑请使用 `convert` 命令，它会在 `.ct` 与可编辑的 `.ct.json` 封套之间往返转换。

### KCES MOD 编辑流程

`packAba` 会根据目录内容自动构建打包清单，`genCt` 可以为已有的 `.aba` 重建 catalog。要编辑纹理，先把它转换成
PNG 并编辑，然后转换回原生 Texture2D，这样重新打包时才能带上改动：

~~~powershell
.\MeidoSerialization.exe unpackAba .\my_mod.aba -o .\aba_files
.\MeidoSerialization.exe convert2image .\aba_files\Texture2D\my_texture.tex
# 编辑 my_texture.png 后，就地重建原生 Texture2D
.\MeidoSerialization.exe convert2texture2d .\aba_files\Texture2D\my_texture.png
.\MeidoSerialization.exe packAba .\aba_files -o my_mod
~~~

游戏只从名字恰好为 `<aba名>.menuassets`（小写）的文件读取部件定义，因此 `-o` 指定的输出名必须与包内的
`.menuassets` 文件名一致，或者修改 menuassets 名称，且不能与现有的文件重名。不指定 `-o` 直接打包解包目录时，默认输出名会去掉 `.aba_unpacked` 后缀以保持
原名；名字不匹配时 `packAba` 会输出警告。

`packAba` 和 `genCt` 会写入默认的 catalog 元数据（`catalogType` Parts、`packageType` Plugin、priority 0）。
需要自定义元数据时，通过 JSON 封套编辑生成的 `.ct`：

~~~powershell
.\MeidoSerialization.exe convert .\my_mod.ct        # 生成 my_mod.ct.json
# 编辑 catalogType、packageType、priority、subName 等 catalog 字段
.\MeidoSerialization.exe convert .\my_mod.ct.json   # 写回 my_mod.ct
~~~

`inspectKcesCatalog` 会打印 CT 中的 `AssetBundleCatalog` 与 `ExtensionNameList`：

~~~powershell
.\MeidoSerialization.exe inspectKcesCatalog .\existing_mod.ct
~~~

## 全局筛选参数

`--strict`（`-s`）要求类型筛选和 `determine` 根据文件内容验证，而不是只依赖名称与扩展名。`--type`（`-t`）把目录操作限制到一种原生类型或对应的编辑
JSON。

~~~powershell
# 类型名前面的点可以省略
.\MeidoSerialization.exe convert2json --type .menu .\mods

# 只处理原生文件
.\MeidoSerialization.exe convert2json --type dbconf .\mods

# 只处理编辑 JSON
.\MeidoSerialization.exe convert2mod --type dbconf.json .\editing

# 根据内容严格匹配
.\MeidoSerialization.exe determine --strict --type preset .\presets
~~~

常见筛选值包括 COM3D2 类型（`menu`、`mate`、`pmat`、`col`、`phy`、`psk`、
`anm`、`model`、`preset`、`tex`、`nei`、`arc`、`bytes`），通用类型 （`image`、`csv`），以及 KCES 类型，例如 `menuassets`、
`materialassets`、
`pmatassets`、`dbconf`、`dbcol`、`db2conf`、`dsbconf`、`dsb2conf`、
`dslconf`、`dsl2conf`、`dslcol`、`ikcol`、`ikcol.bytes`、`limbcol`、
`hitcheck`、`undressdat`、`undresspdat`、`nson`、`ct`、`aba`、
`bridge_session`、`brd`、`enm`、`sad`、`system`、`paths` 和
`maid_collider`。

当前版本到底注册了哪些参数，请以 `.\MeidoSerialization.exe <命令> --help` 为准。

## gRPC 服务

### 启动本地服务

~~~powershell
# 完全可信的本地客户端可以使用 unrestricted 便捷模式
.\MeidoSerialization.exe serve grpc --listen 127.0.0.1:50051

# restricted root-ID 模式
.\MeidoSerialization.exe serve grpc `
  --listen 127.0.0.1:50051 `
  --root mods=C:\Games\COM3D2\Mod
~~~

不提供 `--root` 或 `--restrict-paths` 时，服务会报告 `FILESYSTEM_MODE_UNRESTRICTED`，并接受服务端本地
`input.path`。提供其中任意参数后会报告 `FILESYSTEM_MODE_RESTRICTED`，拒绝直接路径，本地文件必须使用已配置
root。只提供 `--restrict-paths` 而不配置 root 时，会拒绝全部服务端本地文件输入，但 inline 与 blob 输入仍然可用。

版本化服务名为 `meido.serialization.v1.SerializationService`，已启用 server reflection 和标准 gRPC health
service。安装 [grpcurl](https://github.com/fullstorydev/grpcurl) 后，可以这样检查服务发现：

~~~powershell
grpcurl -plaintext 127.0.0.1:50051 list
grpcurl -plaintext -d '{}' 127.0.0.1:50051 meido.serialization.v1.SerializationService/GetCapabilities
~~~

下面分别使用便捷模式和受限模式识别同一个文件：

~~~powershell
# 便捷模式：gRPC 服务所在机器上的绝对路径
grpcurl -plaintext `
  -d '{"input":{"path":"C:\\Games\\COM3D2\\Mod\\menu\\sample.menu"}}' `
  127.0.0.1:50051 meido.serialization.v1.SerializationService/Detect

# 受限模式：公开的 root ID 加上该 root 下的路径
grpcurl -plaintext `
  -d '{"input":{"file":{"rootId":"mods","relativePath":"menu\\sample.menu"}}}' `
  127.0.0.1:50051 meido.serialization.v1.SerializationService/Detect
~~~

已注册的 RPC：

| RPC                                  | 用途                                                                        |
|--------------------------------------|-----------------------------------------------------------------------------|
| `GetCapabilities`                    | 发现文件系统模式、root ID、格式、限制和 Schema/Guide 元数据                  |
| `GetFormatSchema`                    | 返回某个可转换格式的 Draft 2020-12 编辑 JSON Schema                         |
| `GetFormatGuide`                     | 返回与该 Schema 配套、经过源码核对的语义 Guide                              |
| `Detect`                             | 识别原生或编辑 JSON artifact                                                |
| `Convert`                            | 原生格式与编辑 JSON 互转                                                    |
| `Validate`                           | 执行公开 Schema、跨字段规则和原生 serializer 验证                           |
| `Upload`、`Download`、`DeleteBlob`   | 管理进程内、有 TTL 限制的大型临时 blob                                      |
| `ListArchive`、`ExtractArchiveEntry` | 分页浏览或提取 ARC、CT/VirtualDirectory、ABA、`.asset_bg` 与 `.asset_scene` |

每个输入 artifact 必须且只能使用一种来源：带文件名的 inline bytes、服务端签发的 blob ID、unrestricted `path`，或
restricted `file { root_id, relative_path }`。direct/rooted 输入会自动发现旁边的 `.meta.json` 与
`.typetree.json`；inline/blob 调用方必须把这些受支持的附件显式提交。主文件与 sidecar 共用同一个 inline 总预算。放不进
inline 的结果会改用 blob 引用。

安全与存储行为：

- 不提供 root 参数时使用 unrestricted 模式；`path` 可使用绝对路径，或相对于服务进程当前目录的路径
- `--root id=目录` 可以重复指定，始终只读，并自动启用 restricted 模式
- `--restrict-paths` 显式启用 restricted 模式，拒绝直接 `path` 输入
- root 相对路径不能是绝对路径，不能包含 `..` 或卷名，也不能逃出所选 root
- 默认监听 `127.0.0.1:50051`
- 非 loopback 地址必须显式添加 `--allow-remote`，但这仍然没有 TLS、认证或授权；与 unrestricted
  模式组合时，远程客户端可读取服务进程账号有权限访问的任意普通文件
- gRPC 不会把转换结果安装到本地路径或 root，结果仍以内联数据或 blob 返回
- 默认限制为单 blob 4 GiB、总计 16 GiB、4096 个 blob、30 分钟 TTL，以及每个完整 artifact bundle 3 MiB inline
- `--blob-dir` 在服务生命周期内使用独占锁；第二个进程不能同时使用同一目录
- 归档分页默认每页 128 条，每个请求最多 1000 条

相关参数包括 `--root`、`--restrict-paths`、`--max-blob-mib`、`--max-total-blob-mib`、`--max-blobs`、
`--blob-ttl`、`--inline-mib`、`--blob-dir` 和 `--allow-remote`。inline 上限不能超过 3
MiB。完整协议细节见[传输 API 参考](../docs/transport-api.md)。

## MCP stdio 服务

### 添加到 MCP Host

MCP 通过 stdio 作为 Host 的子进程运行，不提供 SSE、HTTP 或 Streamable HTTP 端点。如果 Host 界面要求选择传输模式，
请选择 `stdio`。不同 Host 的配置文件格式可能不同；下面是通用 JSON 示例，使用受限 root，适合作为安全的起点：

~~~json
{
  "mcpServers": {
    "meido-serialization": {
      "command": "C:\\Tools\\MeidoSerialization.exe",
      "args": [
        "mcp",
        "--root",
        "mods=C:\\Games\\COM3D2\\Mod",
        "--write-root",
        "work=C:\\MeidoWork"
      ]
    }
  }
}
~~~

完全可信的本地 Host 也可以使用最短启动方式：

~~~powershell
.\MeidoSerialization.exe mcp
~~~

| 模式           | 如何启用                                            | 读取参数                    | 写入参数                                  | 访问边界                         |
|----------------|-----------------------------------------------------|-----------------------------|-------------------------------------------|----------------------------------|
| `unrestricted` | 不传任何路径相关参数                                | `path`                      | `output_path`                             | 进程账号有权限访问的任意普通文件 |
| `restricted`   | 任意 `--root`、`--write-root` 或 `--restrict-paths` | `root_id` + `relative_path` | `output_root_id` + `output_relative_path` | 仅限已配置 root                  |

`--root` 只读；`--write-root` 可写，也可作为输入读取。只指定 `--restrict-paths` 而不配置 root 时，会有意拒绝所有文件访问。stdout
只写 MCP 协议消息，诊断日志写入 stderr。

### MCP 工具

| 工具                          | 用途                                                            |
|-------------------------------|-----------------------------------------------------------------|
| `meido.detect_file`           | 识别格式 ID、版本、表示形式和文件元数据                         |
| `meido.inspect_file`          | 把大小合理的原生文件转换成 inline 编辑 JSON，便于查看           |
| `meido.validate_editing_json` | 使用 Schema 与原生 serializer 验证 inline 或文件形式的编辑 JSON |
| `meido.convert_file`          | 转换原生/编辑 JSON，并原子安装主文件与受管理 sidecar            |
| `meido.list_archive`          | 返回一页有上限的精确归档条目名                                  |
| `meido.extract_archive_entry` | 把一个精确条目提取到已授权的目标位置                            |

`--max-result-mib` 默认 2 MiB，限制 inspect/list 的 inline 结果；更大的 JSON 文档应使用
`meido.convert_file`。`--max-write-mib` 默认 512 MiB，按主文件与 sidecar 的完整输出 bundle 计算。写入会先暂存，校验大小与
SHA-256；安装失败时会按 bundle 回滚。

### MCP 资源、Prompt 与 portable editing skill

| 入口                                 | 返回内容                                                                     |
|--------------------------------------|------------------------------------------------------------------------------|
| `meido://capabilities`               | 当前文件系统模式、root ID、可写 root、限制、格式能力以及 Schema/Guide 元数据 |
| `meido://schemas/{format_id}`        | 编辑 JSON 的精确 Draft 2020-12 结构协议                                      |
| `meido://guides/{format_id}`         | 字段目录、语义证据、编辑角色、风险、不变量、命令和 value set                 |
| `meido://skills/editing/{format_id}` | portable Markdown 编辑流程，以及当前文件系统模式对应的写入策略               |
| `meido.edit_format`                  | 把编辑目标与渲染后的 skill 组合，并内嵌完整 Schema 和 Guide 的 Prompt        |

这里的 portable skill 是 MCP `text/markdown` 资源，不是自动安装到 Codex 或 MCP Host 中的插件。只读取 skill 也不能替代
Schema 与 Guide：skill 定义保留、验证和写入流程，Schema 给出精确 JSON 结构，Guide 给出经审阅的字段语义。`meido.edit_format`
对普通调用方最方便，因为一次 Prompt 结果会同时返回 skill 文本、完整 Schema 与完整 Guide。这个 Prompt 只准备编辑上下文，不会自行修改文件。

认证数据按作用范围拆开：

| 范围 | 值或 claim | 含义 |
|---|---|---|
| 文件级 `format_verification.level` | `serialization_verified`、`schema_only` | 表示整个文件的序列化契约已知，或目前只有生成出的结构 |
| 字段 `verification.serialization` | `status: verified`、`authority: ai\|human` | 已核对格式、位置或读写方式，不表示游戏含义已知 |
| 字段 `verification.source_semantics` | `status: verified`、`authority: ai\|human` | 已对照游戏源码确认用途或消费路径，并包含序列化认证 |
| 字段 `verification.game_behavior` | `status: verified`、`authority: ai\|human` | 已根据实际游戏运行观察确认行为 |

`schema_only` 不是认证，因此使用 `authority: generated`。字段的空 `verification` 对象表示仅由 Schema
派生：必须保留其值，不能根据名称猜测行为。`field_coverage` 只是数量汇总，不能提升任何字段。Schema
也保存同一套模型，根节点使用 `x-meido-format-verification`，经过描述的属性使用
`x-meido-verification`。

下面这个例子表示字段已经完成源码认证，但还没有实际游戏行为认证：

```json
{
  "format_verification": {
    "level": "serialization_verified",
    "authority": "ai"
  },
  "field_coverage": {
    "total": 42,
    "serialization_verified": 42,
    "source_verified": 8,
    "game_behavior_verified": 0,
    "schema_derived": 0
  },
  "fields": [
    {
      "json_path": "/example",
      "verification": {
        "serialization": {"status": "verified", "authority": "ai"},
        "source_semantics": {"status": "verified", "authority": "ai"}
      }
    }
  ]
}
```

skill 要求模型或客户端按以下顺序操作：

1. 读取 `meido://capabilities`，选择同时具有 `has_editing_schema` 和 `has_format_guide` 的格式
2. 读取并核对 Schema 与 Guide，包括二者一致的 `schema_id`
3. 识别并 inspect 真实输入，不要用默认值重新构造整个文档
4. 只完成用户要求的修改，并保留顺序、重复项、null/空集合区别、整数宽度、raw 字段、ID、哈希、未知新字段和 base64 bytes
5. 遵守已审阅的 `edit_role`、不变量、命令顺序和 `value_set_refs`，不要编造游戏常量
6. 把完整文档提交给 `meido.validate_editing_json`
7. 只有验证成功后才转换回原生格式，并遵守 skill 中动态写入的当前 write policy
8. 把视觉与行为正确性留给游戏内验证；Schema/原生验证无法证明 Unity 中的实际效果

`meido.edit_format` 必填 `format_id` 和 `objective`。可选路径参数随当前模式变化：

| 模式           | 可选的输入/输出 Prompt 参数                                                      |
|----------------|----------------------------------------------------------------------------------|
| `unrestricted` | `input_path`、`output_path`                                                      |
| `restricted`   | `input_root_id`、`input_relative_path`、`output_root_id`、`output_relative_path` |

受限 MCP Host 的目标示例：

~~~text
Prompt: meido.edit_format
format_id: com3d2.menu
objective: 只修改菜单显示名称，并保留所有无关命令
input_root_id: mods
input_relative_path: menu/parts/example.menu
output_root_id: work
output_relative_path: menu/parts/example.menu
~~~

`com3d2.arc`、`com3d2.tex` 这类 native-only/detect-only 格式不会提供编辑 Schema、Guide、skill 或 edit Prompt 流程。应先发现
capabilities，不要猜测资源 URI。

## 从源码构建

~~~powershell
git clone https://github.com/MeidoPromotionAssociation/MeidoSerialization.git
Set-Location MeidoSerialization
go build -o MeidoSerialization.exe .
~~~

其他常用命令：

~~~powershell
.\MeidoSerialization.exe version
.\MeidoSerialization.exe completion powershell
.\MeidoSerialization.exe <命令> --help
~~~

---

---

# 日本語

# MeidoSerialization コマンドラインツール

_AI Translated / AI 翻訳_

MeidoSerialization CLI は、リポジトリの COM3D2/CM3D2 および KCES シリアライザーを通常のファイルコマンドとして提供し、バージョン管理された
protobuf/gRPC サービスと MCP stdio サーバーも公開します。単一ファイルとディレクトリの再帰処理の両方に対応します。
完全な形式一覧は[メイン README](../README.md#対応形式)を参照してください。

変換後の JSON がコンパクト形式になるか整形済みになるかは、元の形式によって異なります。空白は編集契約に含まれないため、読みやすくする目的で
JSON formatter を使用できます。対応形式であれば、この CLI が生成したファイルを
[COM3D2 MOD EDITOR V2](https://github.com/MeidoPromotionAssociation/COM3D2_MOD_EDITOR)
で開くこともできます。

## ダウンロードと必要環境

- 一般ユーザーは [GitHub Releases](https://github.com/MeidoPromotionAssociation/MeidoSerialization/releases) からビルド済み
  Windows 実行ファイルをダウンロードできます
- ソースからのビルドには `go.mod` と同じ Go 1.26.5 以降が必要です
- COM3D2 TEX と画像の相互変換には ImageMagick 7 以降が必要で、`magick` を `PATH` から実行できるようにしてください

以下はすべて PowerShell の例です。`MeidoSerialization.exe` を現在のディレクトリに置いた場合を想定して `.\` を付けています。
`PATH` に追加済みであれば省略できます。空白を含むパスは、パス全体をダブルクォートで囲んでください。

## 最初に試す操作

### よく使う変換

どのコマンドを選ぶか迷った場合は、まず次の例を使用してください。

~~~powershell
# ゲームのネイティブファイル -> 同じディレクトリの編集用 JSON
.\MeidoSerialization.exe convert2json .\example.menu
# 出力：.\example.menu.json

# 編集用 JSON -> 同じディレクトリのネイティブファイル
.\MeidoSerialization.exe convert2mod .\example.menu.json
# 出力：.\example.menu

# 入力から変換方向を自動選択
.\MeidoSerialization.exe convert .\example.menu
.\MeidoSerialization.exe convert .\example.menu.json

# 形式が不明な場合は、変更前に内容から判定
.\MeidoSerialization.exe determine --strict .\unknown_file
~~~

通常、変換結果は入力ファイルの隣に書き込まれ、派生した出力名と同名の既存ファイルを
置き換える場合があります。重要なゲームデータを編集する前にバックアップしてください。安全な一連の作業例は次のとおりです。

~~~powershell
# 1. 元ファイルをバックアップ
Copy-Item .\body.menu .\body.menu.bak

# 2. 判定された形式を確認
.\MeidoSerialization.exe determine --strict .\body.menu

# 3. 編集用 JSON に変換
.\MeidoSerialization.exe convert2json .\body.menu

# 4. body.menu.json を使い慣れたエディターで変更
notepad .\body.menu.json

# 5. ネイティブ形式へ戻す（既定では body.menu に書き込み）
.\MeidoSerialization.exe convert2mod .\body.menu.json
~~~

主な既定出力名：

| 入力例                  | コマンド        | 既定の出力                                                      |
|-------------------------|-----------------|-----------------------------------------------------------------|
| `example.menu`          | `convert2json`  | `example.menu.json`                                             |
| `example.menu.json`     | `convert2mod`   | `example.menu`                                                  |
| `texture.tex`           | `convert2image` | `texture.png`                                                   |
| `image.png`             | `convert2tex`   | `image.tex`                                                     |
| `Texture2D\my_tex.png`  | `convert2texture2d` | ネイティブ KCES `my_tex.tex`                                |
| `body.mmesh`            | `convert2gltf`  | `body.glb`                                                      |
| `dress.model`           | `convert2gltf`  | その `.mmesh` の skeleton・skin・morph を含む `dress.glb`       |
| `dress.glb`             | `gltf2model`    | `dress.model` と `dress.mmesh`                                  |
| `voice.audioclip`       | `convert2audio` | シグネチャに応じて `voice.ogg`、`.wav`、または `.fsb`           |
| `table.nei`             | `convert2csv`   | `table.csv`                                                     |
| `table.csv`             | `convert2nei`   | `table.nei`                                                     |
| `example.arc`           | `unpackArc`     | `example.arc_unpacked\`                                         |
| `example.ct`            | `convert`       | `example.ct.json`                                               |
| `example.aba`           | `unpackAba`     | `example.aba_unpacked\`                                         |
| `example.aba`           | `genCt`         | `example.ct`                                                    |

### ディレクトリの一括変換

ファイルの代わりにディレクトリを渡すと、一致するファイルを再帰的に処理します。

~~~powershell
# ディレクトリ以下の対応ネイティブ編集形式をすべて JSON に変換
.\MeidoSerialization.exe convert2json .\mods

# ネイティブ KCES .dbconf だけを変換
.\MeidoSerialization.exe convert2json --type dbconf .\mods

# .menu 編集 JSON だけをネイティブ形式に戻す
.\MeidoSerialization.exe convert2mod --type menu.json .\editing

# 認識可能な全ファイルを内容ベースで厳密に判定
.\MeidoSerialization.exe determine --strict .\mods
~~~

多くの専用ディレクトリコマンドは worker pool を使用します。不正なファイルがある場合、
そのエラーを表示して残りの処理を続けるため、一括処理後はコンソール出力全体を確認してください。汎用 `convert`
はディレクトリを順番に処理します。混在した入力には便利ですが、専用コマンドより遅くなります。

## ファイル変換コマンド

### ネイティブ形式と編集用 JSON

| コマンド                                    | 用途                                                                       |
|---------------------------------------------|----------------------------------------------------------------------------|
| `convert2json <ファイルまたはディレクトリ>` | 対応する COM3D2/KCES ネイティブデータを隣接する編集用 JSON に変換          |
| `convert2mod <ファイルまたはディレクトリ>`  | 末尾の `.json` を除き、編集用 JSON をネイティブ形式に再エンコード          |
| `convert <ファイルまたはディレクトリ>`      | ネイティブ ↔ JSON、TEX ↔ 画像、NEI ↔ CSV を自動選択、または単一 ARC を展開 |
| `determine <ファイルまたはディレクトリ>`    | ゲーム、ファイル形式、表現、シグネチャ、バージョン、サイズを表示           |

例：

~~~powershell
# COM3D2
.\MeidoSerialization.exe convert2json .\body.menu
.\MeidoSerialization.exe convert2json .\body.mate
.\MeidoSerialization.exe convert2mod .\body.mate.json

# KCES パーツ、物理、コライダーペイロード
.\MeidoSerialization.exe convert2json .\parts\hair.menuassets
.\MeidoSerialization.exe convert2json .\physics\default_hairf.dbconf
.\MeidoSerialization.exe convert2mod .\physics\default_hairf.dbconf.json

# KCES コンテナーおよび状態ファイル
.\MeidoSerialization.exe convert2json .\character.perset
.\MeidoSerialization.exe convert2json .\system.dat
.\MeidoSerialization.exe convert2json .\catalog.ct

# 単独の Unity ネイティブオブジェクト。ファイル内蔵の TypeTree は JSON envelope に保持
.\MeidoSerialization.exe convert2json .\Texture2D\body.tex
.\MeidoSerialization.exe convert2mod .\Texture2D\body.tex.json
~~~

`convert2json` は `.tex` を変換しません。`convert2image` を使用してください。
`.aba` の展開には `unpackAba` を使用します。汎用 `convert` は単一 `.arc` を展開できますが、ディレクトリ内の ARC には
`unpackArc` を使用してください。

### 画像と COM3D2 TEX

~~~powershell
# COM3D2 TEX -> PNG（既定）
.\MeidoSerialization.exe convert2image .\texture.tex

# COM3D2 TEX -> ImageMagick が対応する別の形式
.\MeidoSerialization.exe convert2image .\texture.tex --format webp

# KCES ネイティブ Texture2D -> PNG または DDS
.\MeidoSerialization.exe convert2image .\Texture2D\body.tex --format png
.\MeidoSerialization.exe convert2image .\Texture2D\body.tex --format dds

# KCES ネイティブ Sprite -> PNG のみ
.\MeidoSerialization.exe convert2image .\Sprite\icon.sprite --format png

# 画像 -> COM3D2 TEX（既定では PNG ペイロードを保持）
.\MeidoSerialization.exe convert2tex .\texture.png

# DXT 圧縮 TEX。--compress は force-PNG を自動的に無効化
.\MeidoSerialization.exe convert2tex .\texture.png --compress

# PNG または JPEG -> ネイティブ KCES Texture2D（インライン RGBA32 として再構築）
.\MeidoSerialization.exe convert2texture2d .\my_texture.png
~~~

`convert2tex` は通常 TEX 1010 を書き出します。有効な隣接 `.uv.csv` に 1011 atlas rectangle がある場合のみ 1011 を生成します。詳細は
[TEX 1011 FAQ](../README.md#tex-ファイルのバージョン-1011-について)を参照してください。
`--forcePng=false` も明示的に指定できますが、`--compress` が優先されます。

### KCES Model、Mesh、AnimationClip、AudioClip

これらのコマンドは、KCES `.model` ファイルと、埋め込み TypeTree を持つ単独の Unity ネイティブオブジェクトを処理します。後者は通常本ライブラリで ABA
から展開したファイルです。

~~~powershell
# Model と参照先 .mmesh -> skeleton・skin・morph を含む完全な binary glTF 2.0（既定）
# KCES 固有フィールドは kcesModel extras に保存され、逆変換で復元されます
.\MeidoSerialization.exe convert2gltf .\TextAsset\dress.model

# glTF または GLB -> KCES .model と .mmesh
.\MeidoSerialization.exe gltf2model .\dress.glb -o .\out

# 単独の Mesh または AnimationClip -> binary glTF 2.0（既定）
.\MeidoSerialization.exe convert2gltf .\Mesh\body.mmesh
.\MeidoSerialization.exe convert2gltf .\dance.animationclip.bytes

# data URI を埋め込んだ JSON glTF
.\MeidoSerialization.exe convert2gltf .\Mesh\body.mmesh --format gltf

# AudioClip のエンコード済み音声を、変換せずに抽出
.\MeidoSerialization.exe convert2audio .\AudioClip\voice.audioclip

# ディレクトリ以下の一致する全オブジェクトを一括出力
.\MeidoSerialization.exe convert2gltf .\unpacked
.\MeidoSerialization.exe convert2audio .\unpacked
~~~

Model 変換は双方向です。`convert2gltf` は `.model` と同じディレクトリ、または展開済み ABA の隣接 `Mesh`
ディレクトリから `.mmesh` を探します。`gltf2model` は公式 Unity 2022.3 ネイティブ Mesh を書き出し、そのまま ABA
に再パックできます。skin のない glTF シーンは単一ボーンの skin を合成してメッシュノードに剛体バインドされます。

Blender インポートのヒント：glTF インポーターの `Bones & Skin` パネルで `Guess Original Bind Pose`
のチェックを外し、`Bone Dir` を "Temperance" にすると、きれいな八面体スケルトンになります。KCES の bindpose
は体型スケール込みのスケルトンでベイクされておりノードの rest ポーズと一致しません。Blender
の推測はスケール成分を捨てて再構築するため、ほぼ全ノードがジョイントである衣装モデルではスケルトン全体が崩れます。
既定設定では各ジョイントが仕様として小さな ico 球で表示され、ジョイントの少ないアクセサリーでは残りのノードが
十字軸の empty として読み込まれます。`gltf2model` での再出力精度を表示より優先する場合のみ、既定の "Blender"
`Bone Dir` を維持してください。

マテリアルの見た目は双方向とも意図的に変換しません。KCES マテリアルはバンドルの `.materialassets`
コンテナにパックされたエントリで、`crc_dress044_shoe.mate` のような仮想ファイル名と、glTF
に対応物のないゲーム固有のシェーダーパラメーターおよびテクスチャ参照を持ちます。`convert2gltf`
は各サブメッシュに名前だけのプレースホルダーマテリアルを出力し、`gltf2model` は glTF の PBR
パラメーターとテクスチャをすべて無視して各マテリアルの名前のみを Model に書き込みます。ゲームは実行時、拡張子のない名前に
`.mate` を補い、大文字小文字を無視したハッシュで登録済みの `.materialassets`
エントリを検索します。一致しない名前は `CreateMaterial not found material`
エラーになり、そのサブメッシュは表示されません。したがって各 glTF マテリアルには対象マテリアルエントリの名前（例：
`crc_dress044_shoe`）を付け、エントリ本体は既存の `.materialassets` JSON ワークフローで作成してください。

アニメーション出力は Transform パスを持つ明示的な回転、位置、スケール、Euler curve に対応します。音声出力は OGG、WAV、FSB5
シグネチャから拡張子を選びます。音声のトランスコードは行いません。

### NEI と CSV

~~~powershell
.\MeidoSerialization.exe convert2csv .\table.nei
.\MeidoSerialization.exe convert2nei .\table.csv
~~~

NEI テキストは Shift-JIS です。CSV は RFC 4180 形式に近いカンマ区切りで、BOM 付き UTF-8 として読み書きされます。Shift-JIS
で表現できない文字がある場合、`convert2nei`
は黙って置換せずエラーにします。

## アーカイブコマンド

### COM3D2 ARC

| コマンド                                  | 用途                                        |
|-------------------------------------------|---------------------------------------------|
| `listArc <ファイル>`                      | 保存されている全パスを一覧表示              |
| `unpackArc <ファイルまたはディレクトリ>`  | ARC 全体を展開                              |
| `packArc <ディレクトリ>`                  | 相対ディレクトリ構造を保持して ARC にパック |
| `extractArc <ファイルまたはディレクトリ>` | 拡張子または正確なパス/名前で選択して抽出   |

~~~powershell
# 一覧表示と完全展開
.\MeidoSerialization.exe listArc .\game.arc
.\MeidoSerialization.exe unpackArc .\game.arc
.\MeidoSerialization.exe unpackArc .\game.arc -o .\game_files

# ディレクトリを再パック。既定の出力は <ディレクトリ名>.arc
.\MeidoSerialization.exe packArc .\game_files
.\MeidoSerialization.exe packArc .\game_files -o .\custom.arc

# すべての .menu を抽出
.\MeidoSerialization.exe extractArc .\game.arc --ext menu

# アーカイブ内の正確なパスで一つを抽出
.\MeidoSerialization.exe extractArc .\game.arc --file menu\parts\body.menu -o .\selected

# ファイル名だけを指定する場合、ARC 内で一意であることが必要
.\MeidoSerialization.exe extractArc .\game.arc --file body.menu
~~~

`--ext` と `--file` のどちらか一つだけを必ず指定します。`--file` は単一 ARC 専用で、ARC
ディレクトリには使用できません。まず保存された完全パスを照合し、ファイル名だけの
場合は大文字小文字を区別せず照合します。重複名があれば誤抽出を避けるため拒否します。暗号化 ARC は未対応です。

### KCES CT と ABA

| コマンド                                 | 用途                                                         |
|------------------------------------------|--------------------------------------------------------------|
| `listCt <ファイルまたはディレクトリ>`    | CT/VirtualDirectory 内の仮想ファイルを一覧表示               |
| `genCt <ファイルまたはディレクトリ>`     | .aba ファイルから対応する .ct catalog を生成                 |
| `listAba <ファイルまたはディレクトリ>`   | Unity オブジェクトの PathID、型、サイズ、名前を一覧表示      |
| `unpackAba <ファイルまたはディレクトリ>` | 対応 UnityFS asset を型別ディレクトリへ抽出                  |
| `packAba <ディレクトリ>`                 | 通常のリソースディレクトリを走査して対応する ABA + CT を生成 |

~~~powershell
# CT
.\MeidoSerialization.exe listCt .\my_mod.ct
.\MeidoSerialization.exe genCt .\my_mod.aba

# ABA
.\MeidoSerialization.exe listAba .\my_mod.aba
.\MeidoSerialization.exe unpackAba .\my_mod.aba -o .\aba_files

# .\aba_files の親ディレクトリに my_mod.aba と my_mod.ct を生成
.\MeidoSerialization.exe packAba .\aba_files -o my_mod
~~~

`packAba --output`（`-o`）は出力先ディレクトリや完全なファイル名ではなく、「出力ベース名」です。省略時は入力ディレクトリ名を使用します。packer
はライブラリの canonical Unity 2022.3.35f1 レイアウトを対象にします。暗号化された `abap` bundle は判定できますが、復号できません。

`.ct` はルックアップテーブル（catalog と ExtensionNameList）なので、ディレクトリへは展開しません。閲覧には
`listCt` や `inspectKcesCatalog` を、編集には `.ct` を編集可能な `.ct.json` envelope と相互変換する `convert`
コマンドを使用してください。

### KCES MOD 編集ワークフロー

`packAba` はディレクトリ内容から packing manifest を自動的に構築し、`genCt` は既存の `.aba` から catalog
を再生成します。テクスチャを編集するには、PNG に変換して編集した後、再パックが変更を取り込めるように
ネイティブ Texture2D へ変換し直します。

~~~powershell
.\MeidoSerialization.exe unpackAba .\my_mod.aba -o .\aba_files
.\MeidoSerialization.exe convert2image .\aba_files\Texture2D\my_texture.tex
# my_texture.png を編集した後、ネイティブ Texture2D をその場で再構築
.\MeidoSerialization.exe convert2texture2d .\aba_files\Texture2D\my_texture.png
.\MeidoSerialization.exe packAba .\aba_files -o my_mod
~~~

ゲームは `<aba名>.menuassets`（小文字）という名前のファイルからのみパーツ定義を読み込むため、`-o` に
渡す出力名はパッケージ内の `.menuassets` ファイル名と一致させる必要があります，あるいは、menuassets の名前を変更することもできますが、既存のファイルと同じ名前にすることはできません。`-o` を指定せずに解凍
ディレクトリをパックする場合、既定の出力名は `.aba_unpacked` サフィックスを取り除いて元の名前を維持し、
名前が一致しない場合 `packAba` は警告を出力します。

`packAba` と `genCt` は既定の catalog metadata（`catalogType` Parts、`packageType` Plugin、priority 0）を書き込みます。
metadata をカスタマイズするには、生成された `.ct` を JSON envelope 経由で編集します。

~~~powershell
.\MeidoSerialization.exe convert .\my_mod.ct        # my_mod.ct.json を生成
# catalogType、packageType、priority、subName などの catalog フィールドを編集
.\MeidoSerialization.exe convert .\my_mod.ct.json   # my_mod.ct へ書き戻し
~~~

`inspectKcesCatalog` は CT 内の `AssetBundleCatalog` と `ExtensionNameList` を表示します。

~~~powershell
.\MeidoSerialization.exe inspectKcesCatalog .\existing_mod.ct
~~~

## 共通フィルター

`--strict`（`-s`）は名前と拡張子だけでなく内容も検査して、type filter と `determine`
を実行します。`--type`（`-t`）はディレクトリ操作を一つのネイティブ形式またはその編集 JSON 形式に限定します。

~~~powershell
# 先頭のドットは省略可能
.\MeidoSerialization.exe convert2json --type .menu .\mods

# ネイティブのみ
.\MeidoSerialization.exe convert2json --type dbconf .\mods

# 編集 JSON のみ
.\MeidoSerialization.exe convert2mod --type dbconf.json .\editing

# 内容ベースで厳密に照合
.\MeidoSerialization.exe determine --strict --type preset .\presets
~~~

主なフィルターには COM3D2 形式（`menu`、`mate`、`pmat`、`col`、`phy`、`psk`、
`anm`、`model`、`preset`、`tex`、`nei`、`arc`、`bytes`）、汎用形式 （`image`、`csv`）、KCES 形式（`menuassets`、`materialassets`、
`pmatassets`、
`dbconf`、`dbcol`、`db2conf`、`dsbconf`、`dsb2conf`、`dslconf`、
`dsl2conf`、`dslcol`、`ikcol`、`ikcol.bytes`、`limbcol`、`hitcheck`、
`undressdat`、`undresspdat`、`nson`、`ct`、`aba`、`bridge_session`、
`brd`、`enm`、`sad`、`system`、`paths`、`maid_collider`）があります。

現在のビルドに登録されている正確な flags は
`.\MeidoSerialization.exe <コマンド> --help` で確認してください。

## gRPC サーバー

### ローカルサーバーの起動

~~~powershell
# 完全に信頼できる local client 向け unrestricted convenience mode
.\MeidoSerialization.exe serve grpc --listen 127.0.0.1:50051

# restricted root-ID mode
.\MeidoSerialization.exe serve grpc `
  --listen 127.0.0.1:50051 `
  --root mods=C:\Games\COM3D2\Mod
~~~

`--root` と `--restrict-paths` のどちらも指定しない場合、server は `FILESYSTEM_MODE_UNRESTRICTED` を報告し、server-local の
`input.path` を受け付けます。いずれかを指定すると `FILESYSTEM_MODE_RESTRICTED` になり、direct path は拒否され、local file
は configured root を使う必要があります。root なしの `--restrict-paths` は server-local file input をすべて拒否しますが、inline
と blob input は引き続き利用できます。

バージョン管理されたサービス名は
`meido.serialization.v1.SerializationService` です。server reflection と標準 gRPC health service
が有効です。[grpcurl](https://github.com/fullstorydev/grpcurl) がインストール済みなら、次のコマンドで discovery を確認できます。

~~~powershell
grpcurl -plaintext 127.0.0.1:50051 list
grpcurl -plaintext -d '{}' 127.0.0.1:50051 meido.serialization.v1.SerializationService/GetCapabilities
~~~

次の例は、同じ file を convenience mode と restricted mode で判定します。

~~~powershell
# convenience mode：gRPC server machine 上の absolute path
grpcurl -plaintext `
  -d '{"input":{"path":"C:\\Games\\COM3D2\\Mod\\menu\\sample.menu"}}' `
  127.0.0.1:50051 meido.serialization.v1.SerializationService/Detect

# restricted mode：公開 root ID と、その root の下の path
grpcurl -plaintext `
  -d '{"input":{"file":{"rootId":"mods","relativePath":"menu\\sample.menu"}}}' `
  127.0.0.1:50051 meido.serialization.v1.SerializationService/Detect
~~~

登録 RPC：

| RPC                                  | 用途                                                                              |
|--------------------------------------|-----------------------------------------------------------------------------------|
| `GetCapabilities`                    | filesystem mode、root ID、format、limit、Schema/Guide metadata を検出             |
| `GetFormatSchema`                    | 変換可能な形式の Draft 2020-12 編集 JSON Schema を返す                            |
| `GetFormatGuide`                     | Schema に対応する、ソース確認済み semantic Guide を返す                           |
| `Detect`                             | ネイティブまたは編集 JSON artifact を判定                                         |
| `Convert`                            | ネイティブ ↔ 編集 JSON を変換                                                     |
| `Validate`                           | 公開 Schema、cross-field rule、ネイティブ serializer validation を実行            |
| `Upload`、`Download`、`DeleteBlob`   | process-local かつ TTL 制限付きの大容量一時 blob を管理                           |
| `ListArchive`、`ExtractArchiveEntry` | ARC、CT/VirtualDirectory、ABA、`.asset_bg`、`.asset_scene` をページングまたは抽出 |

入力 artifact は、ファイル名付き inline bytes、server 発行 blob ID、unrestricted `path`、または restricted
`file { root_id, relative_path }` のいずれか一つだけを使用します。direct/rooted input は隣接する `.meta.json` と
`.typetree.json` を自動検出します。inline/blob caller は対応 attachment を明示的に送信します。primary file と sidecar は一つの
inline 上限を共有し、収まらない結果は blob reference になります。

主なセキュリティおよびストレージ動作：

- root flag なしでは unrestricted mode になり、`path` は absolute、または server process の current directory からの relative path を使用できます
- `--root id=ディレクトリ` は繰り返し指定でき、read-only で、自動的に restricted mode を選択します
- `--restrict-paths` は restricted mode を明示的に選択し、direct `path` input を拒否します
- root-relative path は absolute path、`..`、volume name を使用できず、選択 root の外へ出られません
- 既定の listener は `127.0.0.1:50051` です
- loopback 以外には `--allow-remote` が必要ですが、TLS、authentication、authorization は追加されません。unrestricted mode
  と組み合わせると、remote client は server process account が許可する任意の regular file を読み取れます
- gRPC は local path や root に conversion output を install せず、result は inline または blob のままです
- 既定値は blob ごとに 4 GiB、合計 16 GiB、4096 blobs、TTL 30 分、artifact bundle ごとに inline 3 MiB です
- `--blob-dir` はサーバー実行中に排他 lock され、二つ目の process は同じディレクトリを使用できません
- archive page は既定 128 entries、1 request あたり最大 1000 entries です

関連 flags は `--root`、`--restrict-paths`、`--max-blob-mib`、`--max-total-blob-mib`、`--max-blobs`、
`--blob-ttl`、`--inline-mib`、`--blob-dir`、`--allow-remote` です。inline 上限は 3 MiB を超えられません。完全な仕様は
[Transport API リファレンス](../docs/transport-api.md)を参照してください。

## MCP stdio サーバー

### MCP Host への追加

MCP は stdio を通じて Host の子プロセスとして実行され、SSE、HTTP、Streamable HTTP エンドポイントは提供しません。Host
の画面で転送モードの選択を求められた場合は、`stdio` を選択してください。設定形式は Host ごとに異なります。次の一般的な
JSON は restricted root を使用する推奨開始設定です。

~~~json
{
  "mcpServers": {
    "meido-serialization": {
      "command": "C:\\Tools\\MeidoSerialization.exe",
      "args": [
        "mcp",
        "--root",
        "mods=C:\\Games\\COM3D2\\Mod",
        "--write-root",
        "work=C:\\MeidoWork"
      ]
    }
  }
}
~~~

完全に信頼できるローカル Host では、最短の起動方法も使用できます。

~~~powershell
.\MeidoSerialization.exe mcp
~~~

| モード         | 選択方法                                                | 読み取り引数                | 書き込み引数                              | 境界                                                        |
|----------------|---------------------------------------------------------|-----------------------------|-------------------------------------------|-------------------------------------------------------------|
| `unrestricted` | path 関連 flag なし                                     | `path`                      | `output_path`                             | server process account がアクセスできるすべての通常ファイル |
| `restricted`   | `--root`、`--write-root`、`--restrict-paths` のいずれか | `root_id` + `relative_path` | `output_root_id` + `output_relative_path` | 設定済み root のみ                                          |

`--root` は読み取り専用です。`--write-root` は書き込み可能で、入力としても読めます。root なしで `--restrict-paths`
だけを指定すると、すべてのファイルアクセスを意図的に拒否します。stdout には MCP protocol message だけを書き、diagnostic
log は stderr に出力します。

### MCP tools

| Tool                          | 用途                                                                         |
|-------------------------------|------------------------------------------------------------------------------|
| `meido.detect_file`           | format ID、version、representation、file metadata を判定                     |
| `meido.inspect_file`          | 適度なサイズのネイティブファイルを inline 編集 JSON に変換して確認           |
| `meido.validate_editing_json` | inline または file-based 編集 JSON を Schema と native serializer で検証     |
| `meido.convert_file`          | ネイティブ/編集 JSON を変換し、primary file と管理 sidecar を atomic install |
| `meido.list_archive`          | 正確な archive entry name を制限付きの一ページとして返す                     |
| `meido.extract_archive_entry` | 一つの正確な entry を許可済み destination へ抽出                             |

`--max-result-mib` の既定値は 2 MiB で、inspect/list の inline result を制限します。より大きい JSON には
`meido.convert_file` を使用します。`--max-write-mib` は primary と sidecar の全 output bundle に対して既定 512 MiB
です。書き込みは staging 後に size と SHA-256 を確認し、install に失敗すれば bundle 単位で rollback します。

### MCP resources、Prompt、portable editing skill

| Entry point                          | 戻り値                                                                                          |
|--------------------------------------|-------------------------------------------------------------------------------------------------|
| `meido://capabilities`               | 現在の filesystem mode、root ID、writable root、limit、format capability、Schema/Guide metadata |
| `meido://schemas/{format_id}`        | 編集 JSON の正確な Draft 2020-12 構造契約                                                       |
| `meido://guides/{format_id}`         | field inventory、semantic evidence、edit role、risk、invariant、command、value set              |
| `meido://skills/editing/{format_id}` | portable Markdown 編集 workflow と現在の filesystem write policy                                |
| `meido.edit_format`                  | objective と rendered skill を結合し、完全な Schema と Guide を埋め込む Prompt                  |

portable skill は MCP の `text/markdown` resource であり、Codex skill や MCP Host plugin として
自動インストールされるものではありません。skill だけを読んでも Schema と Guide の代わりにはなりません。skill は保存・検証・書き込み
workflow を定義し、Schema は正確な JSON 構造、Guide はレビュー済み field semantics を提供します。`meido.edit_format` は一つの
Prompt result に skill text、完全な Schema、完全な Guide を含むため便利です。この Prompt は context を準備するだけで、ファイルを自動編集しません。

verification data は scope ごとに分かれています。

| Scope | 値または claim | 意味 |
|---|---|---|
| whole-file `format_verification.level` | `serialization_verified`、`schema_only` | ファイルの serialization contract、または生成済み構造だけが既知かを表す |
| field `verification.serialization` | `status: verified`、`authority: ai\|human` | 形式、位置、read/write behavior を確認済み。ゲーム内意味は含まない |
| field `verification.source_semantics` | `status: verified`、`authority: ai\|human` | ゲームソースで用途または consumption path を確認済みで、serialization verification を含む |
| field `verification.game_behavior` | `status: verified`、`authority: ai\|human` | 実際のゲーム実行観察で behavior を確認済み |

`schema_only` は認証ではないため `authority: generated` を使用します。field の空の `verification` object は
Schema 由来だけという意味です。値を保持し、名前から挙動を推測しないでください。`field_coverage` は件数の
summary で、field を昇格させるものではありません。Schema には root の
`x-meido-format-verification` と property の `x-meido-verification` として同じ model が入ります。

次の例は source semantics まで確認済みですが、実ゲームでの behavior observation はまだありません。

```json
{
  "format_verification": {
    "level": "serialization_verified",
    "authority": "ai"
  },
  "field_coverage": {
    "total": 42,
    "serialization_verified": 42,
    "source_verified": 8,
    "game_behavior_verified": 0,
    "schema_derived": 0
  },
  "fields": [
    {
      "json_path": "/example",
      "verification": {
        "serialization": {"status": "verified", "authority": "ai"},
        "source_semantics": {"status": "verified", "authority": "ai"}
      }
    }
  ]
}
```

skill が model/client に要求する順序：

1. `meido://capabilities` を読み、`has_editing_schema` と `has_format_guide` を持つ形式を選択
2. Schema と Guide を読み、`schema_id` が一致することを確認
3. default から文書を再構築せず、実際の入力を detect/inspect
4. 指定された変更だけを行い、順序、重複、null/empty の差、integer width、raw field、ID、hash、未知の future data、base64 bytes
   を保持
5. review 済み `edit_role`、invariant、command ordering、`value_set_refs` に従い、game constant を創作しない
6. 完全な文書を `meido.validate_editing_json` に送信
7. validation 成功後のみネイティブ形式へ変換し、skill に動的に追加された write policy に従う
8. 見た目や動作の正しさはゲーム内で検証する。Schema/native validation は Unity での結果を保証しない

`meido.edit_format` は `format_id` と `objective` が必須です。任意の path 引数は現在の mode によって変わります。

| モード         | 任意の input/output Prompt 引数                                                  |
|----------------|----------------------------------------------------------------------------------|
| `unrestricted` | `input_path`、`output_path`                                                      |
| `restricted`   | `input_root_id`、`input_relative_path`、`output_root_id`、`output_relative_path` |

restricted MCP Host での objective 例：

~~~text
Prompt: meido.edit_format
format_id: com3d2.menu
objective: メニュー表示名だけを変更し、無関係なすべてのコマンドを保持する
input_root_id: mods
input_relative_path: menu/parts/example.menu
output_root_id: work
output_relative_path: menu/parts/example.menu
~~~

`com3d2.arc` や `com3d2.tex` などの native-only/detect-only 形式は、編集 Schema、Guide、skill、edit Prompt workflow
を提供しません。resource URI を推測せず、capabilities から discovery してください。

## ソースからビルド

~~~powershell
git clone https://github.com/MeidoPromotionAssociation/MeidoSerialization.git
Set-Location MeidoSerialization
go build -o MeidoSerialization.exe .
~~~

その他の便利なコマンド：

~~~powershell
.\MeidoSerialization.exe version
.\MeidoSerialization.exe completion powershell
.\MeidoSerialization.exe <コマンド> --help
~~~
