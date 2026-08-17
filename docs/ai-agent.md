# MeidoSerialization: AI Agent Setup and Basics

Use this document when an AI agent must install, configure, or operate MeidoSerialization. Prefer the MCP interface for
agent-driven file work, use restricted filesystem mode by default, and treat the published Schema and Guide as the
format authority.

## 1. What This Tool Does

MeidoSerialization reads, validates, converts, and writes supported CM3D2, COM3D2, COM3D2.5, and KCES files. The same
application engine is available through:

- a command-line executable;
- a versioned protobuf/gRPC service;
- an MCP stdio server;
- Go packages under `service/COM3D2`, `service/KCES`, `serialization/COM3D2`, and `serialization/KCES`.

Do not edit proprietary binary files as arbitrary bytes. Convert supported files to editing JSON, make the smallest
required change, validate the complete JSON document, and convert it back to the native format.

## 2. Install the Executable

### Preferred: use a release build

MCP functionality is available only in versions 2.0.0 and above. If release build is lower than this version, please build from source or download from the Github Actions.

1. Open the latest [GitHub Release](https://github.com/MeidoPromotionAssociation/MeidoSerialization/releases).
2. Select the executable for the current operating system and architecture.
3. Extract it to a stable location that the MCP Host can execute.
4. On Windows, verify the installation:

```powershell
D:\Tools\MeidoSerialization.exe version
D:\Tools\MeidoSerialization.exe --help
```

Use the actual installation path. Do not assume `D:\Tools` exists.

### Alternative: build from source

Building requires Go 1.26.5 or later, matching `go.mod`.

```powershell
git clone https://github.com/MeidoPromotionAssociation/MeidoSerialization.git
Set-Location MeidoSerialization
go build -o MeidoSerialization.exe .
.\MeidoSerialization.exe version
```

Do not run `go install ...@latest` when reproducibility matters; clone the requested revision and build that revision.

Or you can download form Github action form nightly.link: https://nightly.link/MeidoPromotionAssociation/MeidoSerialization/workflows/auto_build/main

The auto build action file is located at https://github.com/MeidoPromotionAssociation/MeidoSerialization/blob/main/.github/workflows/auto_build.yml

### Optional ImageMagick dependency

COM3D2 `.tex` image conversion requires ImageMagick 7 or later and a working `magick` command on `PATH`. Other format
operations do not require ImageMagick merely to start the CLI, gRPC service, or MCP server.

## 3. Configure MCP

Transport is always stdio. The MCP Host launches `MeidoSerialization.exe mcp` as a child process, exchanges MCP protocol
messages through stdin/stdout, and receives diagnostics through stderr.

The server does not expose SSE, HTTP, or Streamable HTTP endpoints.

Ask the user whether filesystem access should use restricted or unrestricted mode; these are
filesystem modes, not transport choices. If the Host presents a transport selector, choose stdio.

### Recommended restricted mode

Restricted mode gives the agent named roots instead of arbitrary host paths:

```powershell
D:\Tools\MeidoSerialization.exe mcp `
  --root mods=D:\Games\COM3D2\Mod `
  --write-root work=D:\MeidoWork
```

- `--root id=path` is read-only.
- `--write-root id=path` is readable and writable.
- Use a dedicated writable work directory instead of a game installation directory.
- Tools receive `root_id` and `relative_path` in restricted mode.

A typical restricted-mode MCP Host entry is:

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

MCP Host configuration wrappers and file locations vary by product. Preserve the executable and argument sequence when
adapting the example: `command` plus `args` launches the stdio child process. Some Hosts display this as a `stdio`
transport. Do not replace it with an HTTP URL or endpoint.

### Unrestricted convenience mode

```powershell
D:\Tools\MeidoSerialization.exe mcp
```

This mode accepts direct `path` and `output_path` values and can access any regular file permitted to the process
account. Use it only in a fully trusted local MCP Host. Never silently replace a restricted configuration with
unrestricted mode.

## 4. First Connection

After the MCP server is connected:

1. Read `meido://capabilities`.
2. Select the exact advertised `format_id`; do not invent format IDs.
3. For editable formats, read `meido://schemas/{format_id}`.
4. Read `meido://guides/{format_id}`.
5. Read `meido://skills/editing/{format_id}`, or call the `meido.edit_format` Prompt with the editing objective.

The Prompt is a context-preparation entry point. It returns the portable skill, complete Schema, and complete Guide; it
does not edit a file by itself. Its required arguments, `format_id` and `objective`, are declared in the Prompt's own
`arguments` list, because an MCP Prompt has no input schema.

Native-only or detect-only formats may not publish an editing Schema or Guide. Use their dedicated archive or conversion
operations instead of inventing editing JSON.

The advertised `formats` list is the complete MCP support set, as `format_support_boundary` states in the same resource.
A file type absent from that list is not detected, converted, validated, or listed through MCP, and `meido.detect_file`
reports it as not recognized. Read `cli_only_operations` to see which conversions require the command line instead; it
currently covers COM3D2 `.nei`, COM3D2 `.tex`, the native Unity Texture2D, Sprite, Mesh, AnimationClip, and AudioClip
primary files, and whole-container packing or unpacking.

## 5. Standard Agent Editing Workflow

Follow this sequence for every structured edit:

1. Call `meido.detect_file` when the format is not already certain.
2. Read capabilities and select the returned or advertised `format_id`.
3. Obtain the exact Schema, Guide, and portable editing skill.
4. Call `meido.inspect_file` to obtain editing JSON from the real input.
5. Preserve every unrelated value and make only the requested changes.
6. Call `meido.validate_editing_json` with the complete edited document.
7. Call `meido.convert_file` only after validation succeeds.
8. Reinspect or otherwise verify the produced artifact before reporting completion.

For large editing JSON documents, use file-based conversion instead of forcing an oversized inline result.

## 6. Verification Rules

Whole-file `format_verification.level` has two possible values:

- `serialization_verified`: the whole-file serialization contract was checked;
- `schema_only`: only the generated structure is known, and this is not a certification.

A completed certification has `authority: ai` or `authority: human`. The non-certified `schema_only` baseline uses
`authority: generated`.

Field `verification` contains independent claims:

- `serialization`: field format, position, or read/write behavior was checked; it does not establish game meaning;
- `source_semantics`: purpose or a consumption path was checked against game source and includes serialization
  verification;
- `game_behavior`: behavior was confirmed through an actual game-runtime observation.

Each present claim has `status: verified` and `authority: ai|human`. An empty `verification` object means the field is
Schema-derived only. Preserve it and do not infer behavior from its name. `field_coverage` is only a count summary and
never promotes an individual field.

## 7. MCP Tool Map

| Tool                          | Use                                                                   |
|-------------------------------|-----------------------------------------------------------------------|
| `meido.detect_file`           | Detect format, version, representation, and metadata                  |
| `meido.inspect_file`          | Convert a reasonably small native file to inline editing JSON         |
| `meido.validate_editing_json` | Validate editing JSON with the published Schema and native serializer |
| `meido.convert_file`          | Convert and install the primary artifact plus managed sidecars        |
| `meido.list_archive`          | List one bounded page of exact archive entries                        |
| `meido.extract_archive_entry` | Extract one exact listed archive entry                                |

Do not guess archive entry names. List entries first, then pass an exact returned name to extraction.

Three argument contracts are worth reading before the first call:

- `meido.validate_editing_json` accepts either a file location or inline `editing_json`; inline JSON also requires
  `name`, including the native double extension such as `sample.menu.json`.
- `meido.convert_file` derives the required input representation from `target`. Use `target=editing_json` on a native
  game file, and `target=native` on an editing JSON document. Native data with `target=native` is rejected as invalid
  editing JSON.
- `meido.list_archive` accepts `page_size` up to 1000, treats 0 or an omitted value as the default 128, rejects an
  out-of-range value, and returns the value that actually applied as `page_size`.

## 8. Artifact and Write Rules

- KCES raw Unity `.bytes` files may use adjacent `.meta.json` and `.typetree.json` sidecars. Treat them as one artifact
  bundle.
- Preserve unknown, opaque, and Schema-derived values unless the explicit objective requires changing them.
- Do not add fields absent from the published Schema.
- Do not reinterpret base64 fields as arbitrary unknown binary storage; use them only where the Schema models a real
  byte array.
- In restricted mode, write only through an advertised writable root.
- Prefer a new output name or dedicated work root when replacing valuable game data is not explicitly required.
- A successful validation proves supported structure and serializer invariants, not that referenced assets, bones,
  materials, IDs, hashes, or enum values exist in the target installation.

## 9. CLI Basics

```powershell
# Detect and convert a file, or recursively process a directory
MeidoSerialization.exe convert .\example.menu
MeidoSerialization.exe convert .\mods

# Explicit native/editing JSON conversion
MeidoSerialization.exe convert2json .\example.menu
MeidoSerialization.exe convert2mod .\example.menu.json

# Inspect KCES containers
MeidoSerialization.exe listCt .\example.ct
MeidoSerialization.exe listAba .\example.aba
MeidoSerialization.exe unpackAba .\example.aba -o .\unpacked

# Export supported raw Unity objects
MeidoSerialization.exe convert2image .\texture.texture2d.bytes
# A .mmesh on its own exports geometry only; convert the .model below for a complete model
MeidoSerialization.exe convert2gltf .\mesh.mmesh --format glb
MeidoSerialization.exe convert2audio .\voice.audioclip.bytes

# Convert a KCES model to and from glTF (skeleton, skin, morphs, and material names included)
# convert2gltf finds the .mmesh through meshFileName, so always feed it the .model, not the .mmesh
MeidoSerialization.exe convert2gltf .\TextAsset\dress.model
MeidoSerialization.exe gltf2model .\dress.glb -o .\out
```

Read `MeidoSerialization.exe --help`, `<command> --help`, and [the complete CLI reference](cli-document.md) before
constructing less common commands.

## 10. gRPC Basics

For a trusted local client, the shortest launch enables unrestricted server-local path inputs:

```powershell
MeidoSerialization.exe serve grpc
```

Use restricted mode for shared clients or least-privilege access:

```powershell
MeidoSerialization.exe serve grpc --root mods=D:\Games\COM3D2\Mod
```

With no `--root` or `--restrict-paths`, `ArtifactInput.path` accepts absolute paths and paths relative to the server
process working directory. Supplying either option disables direct paths; use `file { root_id, relative_path }` instead.
`--restrict-paths` with no roots denies all server-local file inputs while inline and blob inputs remain available.

Use `GetCapabilities` for discovery, `GetFormatSchema` and `GetFormatGuide` for editing contracts, and
`Detect`/`Convert`/`Validate` for control operations. The protobuf definition is
[`serialization.proto`](../api/proto/meido/serialization/v1/serialization.proto). See
[the transport API documentation](transport-api.md) for blob transfer, archive pagination, roots, limits, and security
behavior. Check `filesystem_mode` before choosing `path` or `file`; do not enable unrestricted mode or combine it with
`--allow-remote` without explicit user authorization. Results remain inline or blob-based and are not installed to a
server-local output path.

## 11. Completion Checklist

Before reporting that an agent task is complete, confirm:

- the installed executable and interface were actually available;
- capabilities were read instead of assumed;
- the exact Schema and Guide matched the selected format;
- only requested fields changed;
- Schema-derived and opaque values were preserved;
- validation succeeded before native conversion;
- the primary output and required sidecars were written to the intended destination;
- no unrestricted filesystem access was enabled without explicit authorization.

## 12. Extra Note

MeidoSerialization is open-source software licensed under the BSD-3 license.

If you are unsure about a particular feature, you can view or modify its source code (please confirm with your users).

GitHub
Repository: [https://github.com/MeidoPromotionAssociation/MeidoSerialization](https://github.com/MeidoPromotionAssociation/MeidoSerialization)
