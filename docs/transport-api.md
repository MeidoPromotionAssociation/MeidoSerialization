[English](#english) | [简体中文](#简体中文) | [日本語](#日本語)

# English

# Transport API

MeidoSerialization exposes a versioned application API through protobuf/gRPC and through an MCP stdio server. Both
transports call the same
`application.Engine` and the existing COM3D2/KCES services:

```text
CLI / gRPC / MCP
       |
       v
application.Engine + format registry
       |
       v
service/COM3D2, service/KCES
       |
       v
serialization/COM3D2, serialization/KCES
```

Protobuf is a control and transport schema. It does not replace the native game wire formats, and it does not attempt to
map every game struct into
`google.protobuf.Struct`. Editing JSON is sent as UTF-8 bytes so exact JSON integer tokens, raw MessagePack slots, and
custom polymorphic JSON are not coerced by protobuf. Editing JSON is still standard JSON: NaN and positive or negative
infinity are not representable and are rejected by conversion and validation.

## gRPC

The control schema is [serialization.proto](../api/proto/meido/serialization/v1/serialization.proto). Generated Go files
are committed under `api/gen/go`. Editing JSON contracts are versioned Draft 2020-12 documents under
`schemas/editing/v1` and are embedded in the server binary. The service provides:

- `GetCapabilities`, `GetFormatSchema`, `GetFormatGuide`, `Detect`, `Convert`, and `Validate` for unary control
  operations.
- `Upload` (client streaming) and `Download` (server streaming) for blobs.
- `DeleteBlob` with a process-local TTL/size-limited blob store.
- `ListArchive` and `ExtractArchiveEntry` for COM3D2 ARC and KCES CT/VirtualDirectory, ABA, `.asset_bg`, and
  `.asset_scene` containers.

Input artifacts use exactly one of:

- `inline_data` with a required filename;
- a server-issued `blob.id`;
- `path`, for a direct server-local path in unrestricted filesystem mode;
- `file { root_id, relative_path }`.

An input can also contain repeated `attachments`. Each attachment declares a supported suffix (`.meta.json` or
`.typetree.json`) and its own inline, blob, direct-path, or rooted location. Rooted and local file sources discover
adjacent sidecars automatically. Inline and blob callers must submit them explicitly. Duplicate or unsupported suffixes
are rejected. The inline byte budget applies to the whole unary artifact bundle, not independently to each inline file.

An empty `format_id` means content detection. A conversion target is either
`REPRESENTATION_NATIVE` or `REPRESENTATION_EDITING_JSON`. Results are returned inline while the bundle's cumulative
inline bytes fit the limit; files that do not fit are stored as blobs. The default inline limit is 3 MiB (never above
gRPC's default 4 MiB message limit). When a converter emits sidecars,
`ArtifactResult.attachments` returns metadata and an independent inline/blob location for every file; the primary
metadata size continues to describe the primary file only.

### Contract discovery and strongly typed editing

`GetCapabilities` is the discovery call. It reports `filesystem_mode`, configured root IDs, and active resource limits.
Each convertible format advertises
`has_editing_schema`, `editing_schema_version`, `editing_schema_id`, and
`editing_schema_sha256`. It also advertises `has_format_guide`, the Guide ID, version and SHA-256, and
`format_guide_verification`. Before showing an editor, fetch both contracts:

```text
GetFormatSchema { format_id: "com3d2.menu" }
GetFormatGuide  { format_id: "com3d2.menu" }
```

The response contains `schema_json` as UTF-8 `application/schema+json` bytes. Feed those bytes to a Draft 2020-12
validator or code generator such as quicktype, json-schema-to-typescript, NJsonSchema, or typify. Cache by
`schema_id` and `schema_version`, and verify `sha256` before using a cached copy. The document describes the exact JSON
produced by
`Convert(..., REPRESENTATION_EDITING_JSON)`, including polymorphic `oneOf`
branches, recursive `$defs`, base64 byte fields, and integer ranges.
`Validate` first parses exactly one JSON document and applies this published Schema, including `additionalProperties`
and discriminator constraints. It then runs the native serializer to enforce cross-field relationships and wire-level
rules that are not encoded in the Schema. Conversion from editing JSON to native performs the same structural check, so
callers cannot bypass the contract by skipping `Validate`.

`GetFormatGuide` returns UTF-8
`application/vnd.meido.format-guide+json`. A Guide maps JSON paths to Schema pointers and records field purpose, the
code path used by the game, edit role, risk, constraints, enum meanings, editing guidance, cross-field invariants,
workflow, warnings, and evidence. For script-like formats it can also describe command `forms`, their positional
arguments, target-build notes, and shared source-reviewed `value_sets`. An argument's `value_set_refs` lets a client
resolve values such as a game enum or slot name without guessing from a C# type name. Its `schema_id` must equal the
`GetFormatSchema` response, so a client never combines documentation with the wrong structural contract.

The same verification model appears in both contracts. The Schema root contains
`x-meido-format-verification`; reviewed properties contain `x-meido-verification` beside their descriptions and editing
annotations. The Guide contains `format_verification`, a complete field inventory, independent field `verification`
claims, and an actual `field_coverage` count summary. Whole-file verification has only
`serialization_verified` and `schema_only`. The former confirms the whole-file serialization contract and records an
`ai` or `human` authority. The latter is a generator-derived structural baseline, not a certification, and therefore
uses `authority: generated`.

Field claims are independent. `serialization` confirms format, position, or read/write behavior without claiming game
meaning. `source_semantics` confirms purpose or a consumption path against game source and includes serialization
verification. `game_behavior` requires an actual game-runtime observation; checking game source alone never creates
that claim. Every present claim has `status: verified` and `authority: ai|human`. An empty field `verification` object
means schema-derived only, so callers must preserve it and must not infer behavior from its name. `field_coverage` only
counts exact fields and never promotes an individual claim. No current profile claims `game_behavior` without
runtime-observation evidence.

The recommended editing sequence is:

1. Call `GetCapabilities` and select the advertised `format_id`.
2. Fetch `GetFormatSchema`, verify its SHA-256, and build or load generated types by Schema ID/version.
3. Fetch `GetFormatGuide`, verify its SHA-256 and matching `schema_id`, then use only the exact field claims whose
   verification evidence supports the intended edit.
4. For an LLM, also fetch the MCP editing skill or use the
   `meido.edit_format` prompt described below.
5. Detect and inspect the actual input to obtain its existing values.
6. Make the smallest requested change and preserve every unrelated field.
7. Call `Validate` (or `meido.validate_editing_json`) on the complete document.
8. Convert to native only after validation succeeds.

Native-only formats such as `com3d2.arc` deliberately return `UNIMPLEMENTED`
from the editing Schema/Guide calls; an archive listing is not presented as an editing JSON document or Guide. Use the
archive RPCs for ARC, CT/VirtualDirectory, ABA, `.asset_bg`, and `.asset_scene` containers.

### Blob, archive pagination, and resource limits

Upload chunks are capped at 1 MiB. `GetCapabilities` also reports the maximum single-blob bytes, total in-flight/stored
bytes, object count, and display-name bytes. The CLI defaults are 4 GiB per blob, 16 GiB total, 4096 objects, and a
30-minute TTL. Blobs are process-local temporary objects, not persistent storage.

When `--blob-dir` is supplied, that exact directory is exclusively locked for the lifetime of the server. A second
process using the same directory fails to start before stale-file cleanup, so it cannot delete active blobs owned by the
first process. The operating system releases the lock if the process exits or crashes. With no flag, the server creates
and owns a private temporary directory as before.

`ListArchive` is paged with `page_size` (default 128, maximum 1000) and the
`page_token`/`next_page_token` fields. A response is kept below 2 MiB even when entry names are unusually long; an
individual entry that cannot fit is reported as `RESOURCE_EXHAUSTED`. The gRPC RPC caps a larger request at the maximum,
while `meido.list_archive` publishes `minimum: 0` and `maximum: 1000` in its input schema, rejects an out-of-range
`page_size` instead of silently rewriting the request, and reports the value that actually applied as `page_size` in
every page.

Each server instance signs opaque page cursors with a process-local random HMAC key. A cursor binds its version, offset,
normalized format ID, source archive digest, and the sorted entry name/size/kind inventory. Tampering, using a different
server instance or archive, or changing the archive invalidates the cursor.

Archive listing has independent parsing budgets: by default it materializes at most 10 GiB of input and accepts at most
100,000 entries. The active values are advertised as `max_archive_listing_bytes` and `max_archive_entries` in gRPC/MCP
capabilities. `page_size` limits only one response; it does not reduce the CPU, memory, or input bytes needed to parse
the complete archive directory. Archive traversal, hashing, and sorting boundaries check the request context.

### Filesystem modes and local-first security

The server is intentionally local-first. With no path restriction flags, it starts in convenience mode:

```powershell
MeidoSerialization.exe serve grpc --listen 127.0.0.1:50051
```

`GetCapabilities.filesystem_mode` reports `FILESYSTEM_MODE_UNRESTRICTED`. `ArtifactInput.path` and attachment `path`
accept absolute paths or paths relative to the server process working directory. The server can read any regular file
permitted to its operating-system process account, and direct primary inputs automatically discover adjacent managed
sidecars. Use this mode only for a fully trusted local client.

Configure any root, or use `--restrict-paths`, to enable restricted mode:

```powershell
MeidoSerialization.exe serve grpc `
  --listen 127.0.0.1:50051 `
  --root mods=C:\Games\COM3D2\Mod
```

`GetCapabilities.filesystem_mode` then reports `FILESYSTEM_MODE_RESTRICTED`. Direct `path` locations are rejected and
server-local files must use `file { root_id, relative_path }`. `--root` is repeatable and automatically selects this
mode. `--restrict-paths` selects it explicitly; with no roots, it denies all server-local file access while inline and
blob inputs remain available.

gRPC roots are read-only. Conversion and extraction results are returned inline or as blobs; the service never installs
output beneath a configured root or direct server-local path.

The listener rejects non-loopback addresses unless `--allow-remote` is explicitly supplied. That flag enables an
unencrypted, unauthenticated endpoint and is suitable only behind an operator-controlled trusted network boundary.
Combining `--allow-remote` with unrestricted mode lets remote clients read any regular file allowed by the server
process account. The project does not run a hosted service and does not provide TLS, authentication, or authorization
middleware yet.

In restricted mode, root IDs are an allow-list, not filesystem paths supplied by clients. A client may request
`mods\hair\foo.menu`, but cannot request an absolute path, `..`, a volume name, or a path that escapes the configured
root. The server holds `os.Root` handles for the lifetime of the process, which also prevents a configured directory
path from being swapped underneath a running server.

## MCP stdio

### Filesystem modes

Run the MCP server as a child process of an MCP host. With no path-related flags, MCP uses convenience mode:

```powershell
MeidoSerialization.exe mcp
```

Convenience mode is reported as `filesystem_mode: "unrestricted"` by
`meido://capabilities`. File tools accept `path` and, for tools that write,
`output_path`. Both may be absolute or relative to the MCP server's current working directory. The server can read or
replace any regular file permitted by its operating-system process account. This mode is intended for a trusted local
MCP host where easy access to files in different directories matters more than application-level confinement.

For untrusted prompts, shared MCP hosts, or a least-privilege setup, enable restricted mode by configuring any root:

```powershell
MeidoSerialization.exe mcp `
  --root mods=C:\Games\COM3D2\Mod `
  --write-root work=C:\Users\me\MeidoWork
```

Supplying `--root` or `--write-root` automatically selects
`filesystem_mode: "restricted"`; existing rooted configurations therefore remain restricted. `--restrict-paths`
explicitly selects the same mode and can be used without roots to deny all file access. Restricted tool schemas expose
only `root_id` plus `relative_path`, and writing tools expose only
`output_root_id` plus `output_relative_path`; direct `path` arguments are not registered and cannot bypass the root
policy.

### Transactional writes and result limits

`--root` grants read access only. `meido.convert_file` and
`meido.extract_archive_entry` require `output_root_id` to name a root supplied with `--write-root`; a writable root can
also be used as an input root. Writes stage every primary/sidecar file in the destination directory, sync it, and verify
its expected size and SHA-256 before moving any existing target. Failures before every new file is installed restore the
previous bundle. Once all staged files have been installed, the bundle is committed; removal of rollback backups is
best-effort and cannot turn that committed installation into a reported failure. A successful replacement removes
obsolete managed sidecars from their target names so stale metadata cannot be paired with a new primary file.
`--max-write-mib` defaults to 512 MiB for the combined bundle. Unrestricted `output_path` writes use the same staging,
rollback, sidecar replacement, and size-limit implementation.

`meido.inspect_file` returns editing JSON once as text content and keeps only artifact metadata in structured content.
`meido.list_archive` returns one bounded structured page at a time (default 128, maximum 1000 entries) and echoes the
effective `page_size`; use its `next_page_token` for the next page.

Two tool contracts are declared instead of being discovered through a failed call.
`meido.validate_editing_json` accepts either `root_id` with `relative_path` (`path` in unrestricted mode) or
`editing_json` with `name`, and its input schema requires `name` whenever a non-empty `editing_json` is supplied.
`meido.convert_file` decides the required input representation from `target`: `target=editing_json` reads a native game
file, while `target=native` reads an editing JSON document produced by `meido.inspect_file` or by an earlier
`target=editing_json` conversion. Passing native game data with `target=native` is rejected as invalid editing JSON.

Only protocol messages are written to stdout. Logs go to stderr. The first MCP implementation intentionally uses stdio;
Streamable HTTP can be added as a separate deployment surface without changing the engine.

### MCP tools

| Tool                          | Purpose                                                                                                  |
|-------------------------------|----------------------------------------------------------------------------------------------------------|
| `meido.detect_file`           | Detect a COM3D2/KCES file and return its format ID, version, and representation.                         |
| `meido.inspect_file`          | Return a small editing JSON document inline. Larger documents should use `meido.convert_file`.           |
| `meido.validate_editing_json` | Validate one JSON document against the published Schema, then re-encode it with the native serializer. Inline `editing_json` requires `name`. |
| `meido.convert_file`          | Convert native/editing JSON and install the complete primary/sidecar bundle at the selected destination. `target` decides the required input representation. |
| `meido.list_archive`          | List exact entries in ARC, CT/VirtualDirectory, ABA, `.asset_bg`, or `.asset_scene`.                     |
| `meido.extract_archive_entry` | Extract one exact listed entry at the selected destination.                                              |

### MCP resources, Prompt, and portable editing skill

The `meido://capabilities` resource describes the active `filesystem_mode`, registered formats, limits, root IDs,
Schema/Guide versions and hashes, whole-file Guide verification, and the following MCP entry points. Restricted mode does not reveal
absolute root paths:

| Resource or prompt                   | Purpose                                                                                                     |
|--------------------------------------|-------------------------------------------------------------------------------------------------------------|
| `meido://schemas/{format_id}`        | The same `application/schema+json` structural contract returned by gRPC.                                    |
| `meido://guides/{format_id}`         | The effective field inventory and source-reviewed game-runtime semantics returned by gRPC.                  |
| `meido://skills/editing/{format_id}` | A portable Markdown workflow that tells an LLM how to preserve opaque and unreviewed fields.                |
| `meido.edit_format`                  | MCP prompt that embeds the exact Schema, Guide, portable skill, objective, and optional input/output paths. |

The same resource declares the MCP support boundary so that a client does not have to discover it through a failing
detection. `format_support_boundary` states that the advertised `formats` list is the complete MCP support set: a file
type absent from it is not detected, converted, validated, or listed through MCP, and `meido.detect_file` reports it as
not recognized. `cli_only_operations` lists the conversions that only the command line performs, each with its game,
file type, native suffixes, CLI commands, and the reason for the boundary. It currently covers COM3D2 `.nei` (CSV
conversion), COM3D2 `.tex` (image conversion), the native Unity Texture2D, Sprite, Mesh, AnimationClip, and AudioClip
primary files that are recognized by class ID rather than by suffix, and whole-container packing/unpacking.

The portable editing skill is a `text/markdown` MCP resource. It is not an automatically installed Codex skill or
MCP-host plugin. Reading the resource alone also does not replace its linked Schema and Guide: the resource defines the
preservation, validation, and active-filesystem write workflow, while the two contracts supply the exact structure and
reviewed semantics. The rendered skill dynamically includes the write policy for the server's current filesystem mode.

`meido.edit_format` requires `format_id` and `objective`. An MCP prompt has no input schema in the protocol, so those
required parameters are declared in the prompt's own `arguments` list and repeated in its description. Its Schema and
Guide are returned as two `EmbeddedResource`
messages, not merely URI hints, so the LLM receives the complete structural and semantic context before it edits a file.
In unrestricted mode, optional `input_path` and `output_path` arguments carry the concrete workflow into the prompt. In
restricted mode, the prompt instead exposes `input_root_id`, `input_relative_path`, `output_root_id`, and
`output_relative_path`. This prompt is the convenient way to obtain the skill, complete Schema, and complete Guide in
one result; it prepares context but does not modify a file by itself.

Verification is split by scope:

| Scope | Values or claims | Meaning |
|---|---|---|
| `format_verification.level` | `serialization_verified`, `schema_only` | Whole-file serialization contract or generated structure only |
| `verification.serialization` | `status: verified`, `authority: ai\|human` | Field format or read/write behavior, without game meaning |
| `verification.source_semantics` | `status: verified`, `authority: ai\|human` | Game-source semantics, including serialization verification |
| `verification.game_behavior` | `status: verified`, `authority: ai\|human` | Actual observed game behavior |

An empty field `verification` object is the schema-derived baseline, not a certification. Read `field_coverage` as a
count summary only, and always inspect the exact field before editing it.

## Cancellation and hard limits

Converters receive the request `context.Context` and an exact output budget. Controlled file reads and writes, combined
sidecar sizes, artifact delivery, archive traversal, and serializer writers enforce cancellation checks and hard byte
limits at their I/O boundaries. Some third-party or existing format parsers, compression routines, and decompression
APIs are indivisible synchronous calls. Those calls check cancellation before entry and after return, so cancellation
prevents later output and artifact delivery but may wait for the active call to finish.

## Format IDs

Format IDs are extensible strings rather than a closed protobuf enum. Examples include:

```text
com3d2.menu
com3d2.mate
com3d2.arc
com3d2.timeline
com3d2.object_data
kces.menuassets
kces.dbconf
kces.ct
kces.aba
kces.system
```

The registry currently covers the existing JSON conversion services for COM3D2
menu/mate/pmat/col/phy/psk/anm/model/preset and KCES parts, payload, misc, preset, bridge, saved-attach, paths, system
data, raw Unity objects, and CT editing envelopes. Archive adapters cover ARC, CT/VirtualDirectory, ABA,
`.asset_bg`, and `.asset_scene`. Dance files are split into `com3d2.timeline` (`timeline_data.bytes`) and
`com3d2.object_data` (`maid_data.bytes`, `item_data.bytes`, or
`event_data.bytes`). KCES presets accept both `.preset` and the native
`.perset` suffix. Formats which are detectable but have no complete validator or safe JSON conversion (for example
COM3D2 textures) are advertised as detect-only.

## Regenerating editing schemas

The checked-in catalog contains one document for every convertible registry format. Regenerate it after changing a
public editing model:

```powershell
go run ./internal/schemagen/cmd -out ./schemas/editing/v1
```

Tests compare generated documents with the embedded catalog, so an editing model cannot drift from its published schema
silently.

## Regenerating protobuf code

The repository pins Buf and remote plugin versions in `buf.yaml` and
`buf.gen.yaml`. With Buf 1.72 or newer:

```text
buf lint
buf generate
```

Generated files are part of the source tree. A CI check fails if generation would modify them. Do not hand-edit files
under `api/gen/go`; edit the proto and regenerate instead.

---

# 简体中文

# 传输 API

MeidoSerialization 通过 protobuf/gRPC 和 MCP stdio 服务器提供版本化的应用 API。两种传输方式都调用同一个
`application.Engine`，并复用现有的 COM3D2/KCES service：

```text
CLI / gRPC / MCP
       |
       v
application.Engine + format registry
       |
       v
service/COM3D2, service/KCES
       |
       v
serialization/COM3D2, serialization/KCES
```

protobuf 只是控制与传输协议，不会取代游戏原生 wire 格式，也不会尝试把所有游戏结构体映射为
`google.protobuf.Struct`。editing JSON 以 UTF-8 字节传输，避免 protobuf 改写精确的 JSON 整数 token、MessagePack
原始保留槽位或自定义多态 JSON。editing JSON 仍然是标准 JSON，无法表示 NaN 或正负无穷；转换和验证都会拒绝这些值。

## gRPC

控制协议位于
[serialization.proto](../api/proto/meido/serialization/v1/serialization.proto)。生成的 Go 文件提交在
`api/gen/go` 下。editing JSON 协议是 `schemas/editing/v1` 下带版本的 Draft 2020-12 文档，并嵌入服务器二进制。服务提供：

- `GetCapabilities`、`GetFormatSchema`、`GetFormatGuide`、`Detect`、`Convert` 和 `Validate`，用于 unary 控制操作
- `Upload`（client streaming）与 `Download`（server streaming），用于传输 blob
- `DeleteBlob`，用于管理进程内、有 TTL 和大小限制的 blob store
- `ListArchive` 与 `ExtractArchiveEntry`，用于 COM3D2 ARC，以及 KCES CT/VirtualDirectory、ABA、
  `.asset_bg` 和 `.asset_scene` 容器

输入 artifact 必须且只能使用以下一种来源：

- 带必填文件名的 `inline_data`
- 服务器签发的 `blob.id`
- unrestricted 文件系统模式下的服务端本地直接 `path`
- `file { root_id, relative_path }`

输入还可以包含多个 `attachments`。每个附件声明受支持的后缀（`.meta.json` 或 `.typetree.json`），并分别使用 inline、blob、
direct-path 或 rooted 位置。rooted 和 local 文件来源会自动发现相邻 sidecar；inline 与 blob 调用方必须显式提交。重复或不支持的附件后缀会被拒绝。
inline 字节预算作用于整个 unary artifact bundle，不是分别作用于每个 inline 文件。

空 `format_id` 表示按内容检测。转换目标只能是 `REPRESENTATION_NATIVE` 或
`REPRESENTATION_EDITING_JSON`。当整个 bundle 的累计 inline 字节未超过上限时，结果直接内联返回；放不下的文件会存为 blob。默认
inline 上限是 3 MiB，确保不超过 gRPC 默认的 4 MiB message 上限。如果转换器生成 sidecar，`ArtifactResult.attachments`
会为每个文件分别返回 metadata 和 inline/blob 位置；主 artifact 的 metadata size 仍只表示主文件大小。

### 协议发现与强类型编辑

`GetCapabilities` 是能力发现入口，会返回 `filesystem_mode`、已配置 root ID 和当前资源限制。每个可转换格式都会公开
`has_editing_schema`、
`editing_schema_version`、`editing_schema_id` 和 `editing_schema_sha256`，还会公开
`has_format_guide`、Guide ID、版本、SHA-256 以及 `format_guide_verification`。在显示编辑器之前，应同时获取两份协议：

```text
GetFormatSchema { format_id: "com3d2.menu" }
GetFormatGuide  { format_id: "com3d2.menu" }
```

`GetFormatSchema` 响应中的 `schema_json` 是 UTF-8 `application/schema+json` 字节。可以把它交给 Draft 2020-12 validator，或
quicktype、json-schema-to-typescript、NJsonSchema、typify 等代码生成器。缓存时使用 `schema_id` 与 `schema_version`
，并在使用缓存前校验 `sha256`。Schema 精确描述
`Convert(..., REPRESENTATION_EDITING_JSON)` 生成的 JSON，包括多态 `oneOf` 分支、递归 `$defs`、base64 字节字段和整数范围。

`Validate` 会先严格解析一个 JSON 文档，再应用公开 Schema，包括 `additionalProperties` 与 discriminator 约束；随后调用原生
serializer，检查 Schema 无法表达的跨字段关系和 wire-level 规则。editing JSON 转原生格式时也会执行同一结构检查，因此跳过
`Validate` 不能绕过协议。

`GetFormatGuide` 返回 UTF-8 `application/vnd.meido.format-guide+json`。Guide 把 JSON 路径映射到 Schema
pointer，并记录字段用途、游戏使用的代码路径、编辑角色、风险、约束、枚举含义、编辑建议、跨字段不变量、workflow、警告和证据。对于脚本类格式，Guide
还可以描述命令 `forms`、位置参数、目标游戏版本说明，以及经过源码核对的共享 `value_sets`。参数的 `value_set_refs`
允许客户端直接解析游戏 enum、slot name 等值，而不必根据 C# 类型名猜测。Guide 的 `schema_id` 必须与
`GetFormatSchema` 响应一致，避免把语义说明与错误的结构协议组合。

Schema 和 Guide 使用同一套认证模型。Schema 根节点包含 `x-meido-format-verification`，经过 profile
描述的属性会在说明与编辑注解旁包含 `x-meido-verification`。Guide 则包含
`format_verification`、完整字段目录、字段自己的独立 `verification` claim，以及真实数量汇总
`field_coverage`。文件级认证只有 `serialization_verified` 和 `schema_only`：前者表示整个文件的
序列化契约已经核对，并记录 `ai` 或 `human` 主体；后者只是生成器派生的结构基线，并不是认证，因此
使用 `authority: generated`。

字段 claim 互相独立。`serialization` 只确认格式、位置或读写方式，不声明游戏含义；
`source_semantics` 表示已对照游戏源码确认用途或消费路径，并且包含序列化认证；`game_behavior`
必须具有实际游戏运行观察，单纯核对游戏源码不会产生该 claim。每个存在的 claim 都有
`status: verified` 和 `authority: ai|human`。空的字段 `verification` 对象表示仅由 Schema 派生，
调用方必须保留它，也不得根据字段名推断行为。`field_coverage` 只统计精确字段数量，绝不会提升单个
字段。当前没有任何缺少运行观察证据却声称 `game_behavior` 的 profile。

推荐编辑顺序：

1. 调用 `GetCapabilities`，选择其公开的 `format_id`
2. 获取 `GetFormatSchema`，校验 SHA-256，并按 Schema ID/版本构建或加载生成类型
3. 获取 `GetFormatGuide`，校验 SHA-256 与一致的 `schema_id`，只使用目标字段中有证据支持本次编辑的 verification claim
4. LLM 场景还应获取 MCP editing skill，或使用下文的 `meido.edit_format` Prompt
5. 检测并 inspect 真实输入，取得现有值
6. 只做要求的最小修改，保留所有无关字段
7. 把完整文档提交给 `Validate` 或 `meido.validate_editing_json`
8. 只有验证成功后才转换回原生格式

`com3d2.arc` 等 native-only 格式会在 editing Schema/Guide 调用中明确返回 `UNIMPLEMENTED`；归档列表不会被伪装成 editing
JSON 或 Guide。ARC、CT/VirtualDirectory、ABA、`.asset_bg` 与
`.asset_scene` 应使用归档 RPC。

### Blob、归档分页与资源限制

Upload chunk 上限为 1 MiB。`GetCapabilities` 还会报告单个 blob 最大字节数、存储中与传输中的总字节数、对象数量和 display
name 字节数。CLI 默认限制为单 blob 4 GiB、总计 16 GiB、4096 个对象和 30 分钟 TTL。blob 是进程内临时对象，不是持久存储。

显式指定 `--blob-dir` 时，服务器会在整个生命周期内独占锁定该目录。第二个使用相同目录的进程会在清理
旧文件之前启动失败，因此无法删除第一个进程仍在使用的 blob。进程退出或崩溃后，操作系统会释放锁。不指定该参数时，服务器会创建并独占自己的私有临时目录。

`ListArchive` 使用 `page_size`（默认 128，最大 1000）以及 `page_token` /
`next_page_token` 分页。即使条目名异常长，单页响应也会保持在 2 MiB 以下；单个条目本身无法放入时，返回 `RESOURCE_EXHAUSTED`。gRPC
接口会把超出上限的请求钳制到最大值；`meido.list_archive` 则在 input schema 中公开 `minimum: 0` 与 `maximum: 1000`，对越界
`page_size` 直接报错而不是静默改写请求，并在每一页中以 `page_size` 返回实际生效的值。

每个服务器实例使用进程内随机 HMAC key 签名 opaque 分页 cursor。cursor 会绑定版本、offset、规范化 format ID、源归档摘要，以及排序后的
entry name/size/kind 目录。篡改 cursor、换用其他服务器实例或归档、或者归档内容发生变化，都会使旧 cursor 失效。

归档列表使用独立解析预算：默认最多物化 10 GiB 输入，并接受最多 100,000 个条目。当前值通过 gRPC/MCP capabilities 的
`max_archive_listing_bytes` 与 `max_archive_entries` 公开。`page_size` 只限制单次响应，不会减少解析完整归档目录所需的
CPU、内存或输入字节。归档遍历、hash 和排序边界会检查请求 context。

### 文件系统模式与本地优先安全模型

服务器有意采用 local-first 设计。不提供路径限制参数时，会以便捷模式启动：

```powershell
MeidoSerialization.exe serve grpc --listen 127.0.0.1:50051
```

`GetCapabilities.filesystem_mode` 会返回 `FILESYSTEM_MODE_UNRESTRICTED`。`ArtifactInput.path` 与附件的 `path`
可以是绝对路径，也可以相对于服务进程当前目录。服务器可以读取操作系统进程账号有权限访问的任意普通文件；直接路径形式的主文件还会自动发现相邻的受管理
sidecar。该模式只适合完全可信的本地客户端。

配置任意 root，或使用 `--restrict-paths`，即可启用受限模式：

```powershell
MeidoSerialization.exe serve grpc `
  --listen 127.0.0.1:50051 `
  --root mods=C:\Games\COM3D2\Mod
```

此时 `GetCapabilities.filesystem_mode` 返回 `FILESYSTEM_MODE_RESTRICTED`。服务会拒绝直接 `path`，服务端本地文件必须使用
`file { root_id, relative_path }`。`--root` 可重复指定，并会自动选择该模式；`--restrict-paths` 可以显式选择该模式，不配置
root 时会拒绝全部服务端本地文件访问，但 inline 与 blob 输入仍然可用。

gRPC root 始终只读。转换和提取结果以内联数据或 blob 返回；服务不会把输出安装到已配置 root 或服务端本地直接路径。

除非显式指定 `--allow-remote`，listener 会拒绝非 loopback 地址。该参数开启的仍是未加密、无认证的 endpoint，只适合放在操作者自行控制的
可信网络边界后。把 `--allow-remote` 与 unrestricted 模式组合，会允许远程客户端读取服务进程账号有权限访问的任意普通文件。项目不运营托管服务，
目前也没有提供 TLS、authentication 或 authorization middleware。

在 restricted 模式下，root ID 是 allow-list 标识符，不是由客户端任意提交的文件系统路径。客户端可以请求
`mods\hair\foo.menu`，但不能请求绝对路径、`..`、卷名或任何逃出所选 root 的路径。服务器在进程生命周期内持有 `os.Root`
handle，也能防止运行期间用另一个目录替换已配置路径。

## MCP stdio

### 文件访问模式

MCP server 作为 MCP Host 的子进程运行。不传路径相关参数时，MCP 使用便捷模式：

```powershell
MeidoSerialization.exe mcp
```

`meido://capabilities` 将便捷模式报告为 `filesystem_mode: "unrestricted"`。文件工具使用
`path`，写入工具另使用 `output_path`；两者都可以是绝对路径，也可以相对于 MCP server 当前目录。
服务器可以读取或替换运行进程账号有权限访问的任意普通文件。该模式只适合完全可信的本地 MCP Host，用于需要跨多个目录便捷访问文件的场景。

对于不可信 Prompt、共享 MCP Host 或最小权限部署，应配置任意 root 进入 restricted 模式：

```powershell
MeidoSerialization.exe mcp `
  --root mods=C:\Games\COM3D2\Mod `
  --write-root work=C:\Users\me\MeidoWork
```

指定 `--root` 或 `--write-root` 会自动选择 `filesystem_mode: "restricted"`，因此现有 rooted 配置仍保持受限。
`--restrict-paths` 可显式选择同一模式；不配置 root 时使用该参数，会拒绝全部文件访问。restricted 工具 Schema 只公开
`root_id` + `relative_path`，写入工具只公开 `output_root_id` +
`output_relative_path`；不会注册直接 `path` 参数，因此无法绕过 root policy。

### 事务式写入与结果限制

`--root` 只授予读取权限。`meido.convert_file` 与 `meido.extract_archive_entry` 要求
`output_root_id` 指向 `--write-root` 提供的 root；可写 root 同时也可以作为输入 root。

写入会先在目标目录暂存所有主文件与 sidecar，执行同步，并在移动任何现有目标之前校验预期大小与 SHA-256。
如果全部新文件安装完成前发生失败，会恢复之前的 bundle。所有暂存文件安装完毕后即视为提交成功；随后清理 rollback backup 属于
best-effort，即使清理失败也不会把已提交安装报告为失败。成功替换会删除目标位置上本次不再生成的受管理 sidecar，避免旧
metadata 与新主文件错误配对。`--max-write-mib` 对主文件与 sidecar 的组合 bundle 默认限制为 512 MiB。unrestricted
`output_path` 使用同一套暂存、回滚、sidecar 替换和大小限制实现。

`meido.inspect_file` 会把 editing JSON 作为 text content 返回一次，structured content 只保留 artifact metadata。
`meido.list_archive` 每次返回一页有界 structured result，默认 128 条、最多 1000 条，并回显实际生效的 `page_size`；下一页使用
`next_page_token`。

有两条工具约定是显式声明的，不需要通过失败调用去发现。`meido.validate_editing_json` 接受 `root_id` + `relative_path`
（unrestricted 模式为 `path`），或者 `editing_json` + `name`；只要提供了非空 `editing_json`，其 input schema 就要求同时提供
`name`。`meido.convert_file` 由 `target` 决定输入必须持有的 representation：`target=editing_json` 读取原生游戏文件，
`target=native` 读取由 `meido.inspect_file` 或先前 `target=editing_json` 转换产生的 editing JSON 文档。用
`target=native` 提交原生游戏数据会作为无效 editing JSON 被拒绝。

stdout 只写 MCP 协议消息，日志写入 stderr。第一版 MCP 有意只实现 stdio；以后可以把 Streamable HTTP 作为独立部署面加入，而不需要修改
engine。

### MCP 工具

| 工具                          | 用途                                                                       |
|-------------------------------|----------------------------------------------------------------------------|
| `meido.detect_file`           | 检测 COM3D2/KCES 文件，返回 format ID、版本和 representation               |
| `meido.inspect_file`          | 内联返回较小的 editing JSON；较大文档应使用 `meido.convert_file`           |
| `meido.validate_editing_json` | 先按公开 Schema 验证一个 JSON 文档，再使用原生 serializer 重新编码；内联 `editing_json` 必须同时提供 `name` |
| `meido.convert_file`          | 转换原生/editing JSON，并在目标位置安装完整主文件/sidecar bundle；`target` 决定输入必须持有的 representation |
| `meido.list_archive`          | 精确列出 ARC、CT/VirtualDirectory、ABA、`.asset_bg` 或 `.asset_scene` 条目 |
| `meido.extract_archive_entry` | 把一个精确列出的条目提取到选定目标                                         |

### MCP 资源、Prompt 与 portable editing skill

`meido://capabilities` 资源描述当前 `filesystem_mode`、已注册格式、限制、root ID、Schema/Guide 版本与 hash、Guide
文件级认证，以及下列 MCP 入口。restricted 模式不会公开 root 的绝对路径：

| 资源或 Prompt                        | 用途                                                                               |
|--------------------------------------|------------------------------------------------------------------------------------|
| `meido://schemas/{format_id}`        | gRPC 返回的同一份 `application/schema+json` 结构协议                               |
| `meido://guides/{format_id}`         | gRPC 返回的完整字段目录与经过源码核对的游戏运行语义                                |
| `meido://skills/editing/{format_id}` | 告诉 LLM 如何保留不透明与未审阅字段的 portable Markdown workflow                   |
| `meido.edit_format`                  | 内嵌精确 Schema、Guide、portable skill、objective 和可选输入/输出路径的 MCP Prompt |

同一份资源还声明了 MCP 的格式支持边界，客户端不需要通过失败的检测去摸索。`format_support_boundary` 说明公开的 `formats`
列表就是 MCP 的完整支持集：不在其中的文件类型在 MCP 上不会被检测、转换、校验或列出，`meido.detect_file` 会报告
not recognized。`cli_only_operations` 列出只有命令行才提供的转换，每条包含游戏、文件类型、原生后缀、CLI 命令，以及该边界的原因。
当前覆盖 COM3D2 `.nei`（CSV 转换）、COM3D2 `.tex`（图片转换）、按 class ID 而非后缀识别的原生 Unity Texture2D、Sprite、Mesh、
AnimationClip 与 AudioClip 主文件，以及整包封装/解包。

portable editing skill 是 MCP `text/markdown` 资源，不会自动安装成 Codex skill 或 MCP Host 插件。单独读取该资源也不能替代它链接的
Schema 与 Guide：skill 定义保留、验证以及当前文件系统的写入流程，另外两份协议提供精确结构和经审阅语义。渲染后的 skill
会动态附带服务器当前 filesystem mode 对应的 write policy。

`meido.edit_format` 必填 `format_id` 与 `objective`。MCP 协议中的 Prompt 没有 input schema，因此这两个必填参数声明在 Prompt
自身的 `arguments` 列表中，并在其 description 中重复说明。Schema 和 Guide 会作为两个完整
`EmbeddedResource` 返回，而不仅是 URI 提示，因此 LLM 在编辑前会收到完整结构与语义上下文。unrestricted 模式可以选填
`input_path` 和 `output_path`；restricted 模式改为
`input_root_id`、`input_relative_path`、`output_root_id` 和 `output_relative_path`。这是一次取得 skill、完整 Schema 与完整
Guide 的便捷入口，但 Prompt 只准备上下文，不会自行修改文件。

认证按作用范围拆开：

| 范围 | 值或 claim | 含义 |
|---|---|---|
| `format_verification.level` | `serialization_verified`、`schema_only` | 整个文件的序列化契约已知，或目前只有生成出的结构 |
| `verification.serialization` | `status: verified`、`authority: ai\|human` | 字段格式或读写方式，不包含游戏含义 |
| `verification.source_semantics` | `status: verified`、`authority: ai\|human` | 游戏源码语义，并包含序列化认证 |
| `verification.game_behavior` | `status: verified`、`authority: ai\|human` | 实际观察到的游戏行为 |

空字段 `verification` 对象是 Schema 派生基线，不是认证。`field_coverage` 只能作为数量汇总读取；
编辑前始终要检查目标字段自己的 claim。

## 取消与硬限制

转换器直接接收请求的 `context.Context` 和精确输出预算。受控文件读取与写入、sidecar 总量、artifact 交付、归档遍历和
serializer writer 会在 I/O 边界执行取消检查与硬字节上限。部分第三方或既有格式 parser、压缩和解压 API
是不可分割的同步调用，无法在函数内部强制中断。这些调用会在进入前和返回后检查取消，因此取消能够阻止后续输出与 artifact
交付，但可能需要等待当前同步调用返回。

## 格式 ID

format ID 是可扩展字符串，不是封闭的 protobuf enum。例如：

```text
com3d2.menu
com3d2.mate
com3d2.arc
com3d2.timeline
com3d2.object_data
kces.menuassets
kces.dbconf
kces.ct
kces.aba
kces.system
```

registry 当前覆盖 COM3D2 menu/mate/pmat/col/phy/psk/anm/model/preset，以及 KCES parts、payload、
misc、preset、bridge、saved-attach、paths、system data、raw Unity object 和 CT editing envelope 的现有 JSON 转换 service。归档
adapter 覆盖 ARC、CT/VirtualDirectory、ABA、`.asset_bg` 与
`.asset_scene`。

舞蹈文件分为 `com3d2.timeline`（`timeline_data.bytes`）和 `com3d2.object_data`
（`maid_data.bytes`、`item_data.bytes` 或 `event_data.bytes`）。KCES preset 同时接受
`.preset` 与原生 `.perset` 后缀。可以检测但没有完整 validator 或安全 JSON 转换的格式 （例如 COM3D2 texture）会公开为
detect-only。

## 重新生成 editing Schema

仓库内 catalog 为每个可转换 registry format 保存一份文档。公开 editing model 变化后，应重新生成：

```powershell
go run ./internal/schemagen/cmd -out ./schemas/editing/v1
```

测试会比较生成文档与嵌入 catalog，因此 editing model 无法在没有提示的情况下偏离已发布 Schema。

## 重新生成 protobuf 代码

仓库在 `buf.yaml` 和 `buf.gen.yaml` 中固定 Buf 与远程插件版本。使用 Buf 1.72 或更高版本：

```text
buf lint
buf generate
```

生成文件属于源码树的一部分。如果重新生成会修改文件，CI 检查会失败。不要手工修改 `api/gen/go`；应修改 proto 后重新生成。

---

# 日本語

# Transport API

_AI Translated / AI 翻訳_

MeidoSerialization は protobuf/gRPC と MCP stdio server を通じて、バージョン管理された application API を提供します。どちらの
transport も同じ `application.Engine` と既存の COM3D2/KCES service を呼び出します。

```text
CLI / gRPC / MCP
       |
       v
application.Engine + format registry
       |
       v
service/COM3D2, service/KCES
       |
       v
serialization/COM3D2, serialization/KCES
```

protobuf は control/transport schema です。ゲーム固有の native wire format を置き換えるものではなく、すべての game struct
を `google.protobuf.Struct` に変換するものでもありません。editing JSON は UTF-8 bytes として送信されるため、正確な JSON
integer token、raw MessagePack slot、custom polymorphic JSON が protobuf によって変換されません。editing JSON 自体は標準
JSON であり、NaN と正負の infinity は表現できないため、conversion と validation で拒否されます。

## gRPC

control schema は
[serialization.proto](../api/proto/meido/serialization/v1/serialization.proto) にあります。生成済み Go file は
`api/gen/go` に commit されています。editing JSON contract は `schemas/editing/v1` にある versioned Draft 2020-12
document で、server binary に埋め込まれています。service は以下を提供します。

- unary control operation 用の `GetCapabilities`、`GetFormatSchema`、`GetFormatGuide`、`Detect`、`Convert`、`Validate`
- blob 用の `Upload`（client streaming）と `Download`（server streaming）
- process-local で TTL/size 制限付き blob store の `DeleteBlob`
- COM3D2 ARC、および KCES CT/VirtualDirectory、ABA、`.asset_bg`、`.asset_scene` 用の
  `ListArchive` と `ExtractArchiveEntry`

input artifact は次の source のうち一つだけを使用します。

- 必須 filename 付き `inline_data`
- server が発行した `blob.id`
- unrestricted filesystem mode で使用する server-local direct `path`
- `file { root_id, relative_path }`

input は複数の `attachments` を持つこともできます。各 attachment は対応 suffix （`.meta.json` または `.typetree.json`
）と、それぞれの inline、blob、direct-path、rooted location を指定します。rooted/local file source は隣接 sidecar を自動検出します。
inline/blob caller は明示的に送信してください。重複または未対応 suffix は拒否されます。inline byte budget は各 file 個別ではなく、
unary artifact bundle 全体に適用されます。

空の `format_id` は content detection を意味します。conversion target は
`REPRESENTATION_NATIVE` または `REPRESENTATION_EDITING_JSON` のどちらかです。bundle 全体の累積 inline bytes が上限に収まる間は
inline で返し、収まらない file は blob に格納します。既定 inline 上限は 3 MiB で、gRPC の既定 4 MiB message limit
を超えません。converter が sidecar を生成する場合、
`ArtifactResult.attachments` は各 file の metadata と独立した inline/blob location を返します。primary artifact の
metadata size は引き続き primary file だけの大きさです。

### Contract discovery と strongly typed editing

`GetCapabilities` が discovery call で、`filesystem_mode`、configured root ID、現在の resource limit を返します。変換可能な各 format は
`has_editing_schema`、
`editing_schema_version`、`editing_schema_id`、`editing_schema_sha256` を公開し、さらに
`has_format_guide`、Guide ID、version、SHA-256、`format_guide_verification` を公開します。editor を表示する前に、両方の contract
を取得してください。

```text
GetFormatSchema { format_id: "com3d2.menu" }
GetFormatGuide  { format_id: "com3d2.menu" }
```

`GetFormatSchema` response の `schema_json` は UTF-8 `application/schema+json` bytes です。Draft 2020-12 validator、または
quicktype、json-schema-to-typescript、NJsonSchema、typify などの code generator に渡せます。`schema_id` と `schema_version`
で cache し、cache を使う前に `sha256`
を検証してください。この Schema は `Convert(..., REPRESENTATION_EDITING_JSON)` が生成する JSON を正確に記述し、polymorphic
`oneOf` branch、recursive `$defs`、base64 byte field、integer range を含みます。

`Validate` は厳密に一つの JSON document を parse し、`additionalProperties` と discriminator constraint を含む公開 Schema
を適用します。その後 native serializer を実行し、Schema に記述できない cross-field relationship と wire-level rule
を検証します。editing JSON から native への conversion も同じ構造検証を行うため、`Validate` を省略して contract
を回避することはできません。

`GetFormatGuide` は UTF-8 `application/vnd.meido.format-guide+json` を返します。Guide は JSON path を Schema pointer
に対応付け、field purpose、ゲームが使用する code path、edit role、risk、constraint、enum meaning、editing guidance、cross-field
invariant、workflow、warning、evidence を記録します。script-like format では command `forms`、positional
argument、target-build note、source-reviewed `value_sets` も記述できます。argument の `value_set_refs` により、client は C# type
name から推測せず、game enum や slot name を解決できます。Guide の `schema_id` は `GetFormatSchema` response と一致する必要があり、
異なる structural contract の documentation を組み合わせることを防ぎます。

Schema と Guide は同じ verification model を使用します。Schema root には
`x-meido-format-verification` があり、profile で説明された property には description や editing annotation とともに
`x-meido-verification` があります。Guide は `format_verification`、完全な field inventory、field ごとの独立した
`verification` claim、実際の件数を示す `field_coverage` を含みます。whole-file verification は
`serialization_verified` と `schema_only` だけです。前者はファイル全体の serialization contract を確認済みで、
`ai` または `human` authority を記録します。後者は generator 由来の structural baseline で認証ではないため、
`authority: generated` を使用します。

field claim は独立しています。`serialization` は形式、位置、read/write behavior だけを確認し、ゲーム内意味を主張しません。
`source_semantics` はゲームソースで用途または consumption path を確認済みで、serialization verification を含みます。
`game_behavior` には実際の game-runtime observation が必要で、ゲームソースの確認だけでは付与されません。存在する claim は
`status: verified` と `authority: ai|human` を持ちます。空の field `verification` object は Schema 由来だけであるため、caller は
値を保持し、名前から behavior を推測してはいけません。`field_coverage` は exact field の件数だけを集計し、個別 claim を昇格
させません。runtime-observation evidence なしで `game_behavior` を主張する現在の profile はありません。

推奨 editing sequence：

1. `GetCapabilities` を呼び出し、公開された `format_id` を選択
2. `GetFormatSchema` を取得して SHA-256 を検証し、Schema ID/version で generated type を build または load
3. `GetFormatGuide` を取得し、SHA-256 と一致する `schema_id` を検証して、目的の edit を evidence が支える exact field の
   verification claim だけを使用
4. LLM では MCP editing skill も取得するか、後述の `meido.edit_format` Prompt を使用
5. 実際の input を detect/inspect して既存値を取得
6. 要求された最小変更だけを行い、無関係なすべての field を保持
7. 完全な document を `Validate` または `meido.validate_editing_json` に送信
8. validation 成功後だけ native 形式へ変換

`com3d2.arc` などの native-only format は editing Schema/Guide call に対して `UNIMPLEMENTED` を返します。archive listing
は editing JSON document や Guide として扱われません。ARC、CT/VirtualDirectory、ABA、
`.asset_bg`、`.asset_scene` には archive RPC を使用してください。

### Blob、archive pagination、resource limit

Upload chunk は最大 1 MiB です。`GetCapabilities` は single blob bytes、in-flight/stored total bytes、object
count、display-name bytes の上限も報告します。CLI の既定値は blob ごとに 4 GiB、合計 16 GiB、4096 object、TTL 30 分です。blob
は process-local temporary object であり、persistent storage ではありません。

`--blob-dir` を指定すると、その directory は server lifetime 全体で exclusive lock されます。同じ directory を使う二つ目の
process は stale-file cleanup より前に起動失敗するため、一つ目の process が所有する active blob を削除できません。process
の終了または crash 時は OS が lock を解放します。flag を省略すると、server は private temporary directory を作成して所有します。

`ListArchive` は `page_size`（既定 128、最大 1000）と `page_token` / `next_page_token` で pagination します。entry name
が非常に長くても response は 2 MiB 未満に保たれ、一つの entry 自体が収まらない場合は `RESOURCE_EXHAUSTED` になります。gRPC RPC
は上限を超える要求を最大値に切り詰めますが、`meido.list_archive` は input schema に `minimum: 0` と `maximum: 1000` を公開し、
範囲外の `page_size` を黙って書き換えるのではなく拒否し、実際に適用された値を各 page の `page_size` として返します。

各 server instance は process-local random HMAC key で opaque page cursor を署名します。cursor は version、
offset、normalized format ID、source archive digest、sorted entry name/size/kind inventory に結び付けられます。cursor
の改ざん、別 server instance/別 archive での使用、archive content の変更は cursor を無効にします。

archive listing には独立した parsing budget があります。既定では最大 10 GiB の input を materialize し、最大 100,000
entry を受け付けます。現在値は gRPC/MCP capabilities の
`max_archive_listing_bytes` と `max_archive_entries` で公開されます。`page_size` は一つの response だけを制限し、archive
directory 全体を parse するための CPU、memory、input bytes を減らしません。archive traversal、hashing、sorting の境界で
request context を確認します。

### Filesystem mode と local-first security model

server は意図的に local-first です。path restriction flag を指定しない場合、convenience mode で起動します。

```powershell
MeidoSerialization.exe serve grpc --listen 127.0.0.1:50051
```

`GetCapabilities.filesystem_mode` は `FILESYSTEM_MODE_UNRESTRICTED` を返します。`ArtifactInput.path` と attachment の `path`
には absolute path、または server process の current directory からの relative path を指定できます。server は OS process account
が許可する任意の regular file を読み取れ、direct primary input は隣接する managed sidecar を自動検出します。この mode は完全に
trusted な local client 専用です。

任意の root を設定するか、`--restrict-paths` を使用すると restricted mode になります。

```powershell
MeidoSerialization.exe serve grpc `
  --listen 127.0.0.1:50051 `
  --root mods=C:\Games\COM3D2\Mod
```

`GetCapabilities.filesystem_mode` は `FILESYSTEM_MODE_RESTRICTED` を返します。direct `path` location は拒否され、server-local file は
`file { root_id, relative_path }` を使用する必要があります。`--root` は繰り返し指定でき、自動的にこの mode を選択します。
`--restrict-paths` は同じ mode を明示的に選択し、root なしではすべての server-local file access を拒否しますが、inline/blob input
は引き続き利用できます。

gRPC root は read-only です。conversion/extraction result は inline または blob として返し、service は configured root や direct
server-local path に output を install しません。

`--allow-remote` を明示しない限り、listener は non-loopback address を拒否します。この flag が有効にする endpoint も
unencrypted/unauthenticated であり、operator が管理する trusted network boundary の内側でのみ使用してください。
`--allow-remote` と unrestricted mode を組み合わせると、remote client は server process account が許可する任意の regular file
を読み取れます。project は hosted service を運営せず、現時点で TLS、authentication、authorization middleware を提供しません。

restricted mode の root ID は allow-list identifier であり、client が任意に指定する filesystem path ではありません。client は
`mods\hair\foo.menu` を要求できますが、absolute path、`..`、volume name、configured root から外へ出る path は要求できません。server
は process lifetime 中 `os.Root` handle を保持し、実行中に configured directory path を別 directory に差し替えることも防ぎます。

## MCP stdio

### Filesystem mode

MCP server は MCP Host の child process として実行します。path 関連 flag を付けない場合、convenience mode を使用します。

```powershell
MeidoSerialization.exe mcp
```

`meido://capabilities` は convenience mode を `filesystem_mode: "unrestricted"` として報告します。file tool は `path`
、write tool は追加で `output_path` を受け取ります。どちらも absolute path、または MCP server の current working directory
からの relative path を使用できます。server は OS process account が許可する任意の regular file を読み取りまたは置き換えられます。この
mode は、application-level confinement より複数 directory への簡単な access を優先する、完全に trusted な local MCP Host
専用です。

untrusted Prompt、shared MCP Host、least-privilege setup では、任意の root を設定して restricted mode を有効にします。

```powershell
MeidoSerialization.exe mcp `
  --root mods=C:\Games\COM3D2\Mod `
  --write-root work=C:\Users\me\MeidoWork
```

`--root` または `--write-root` を指定すると自動的に `filesystem_mode: "restricted"` になり、既存の rooted configuration も
restricted のままです。`--restrict-paths` は同じ mode を明示的に選択し、root なしで指定するとすべての file access
を拒否します。restricted tool schema は `root_id` + `relative_path`
だけを公開し、write tool は `output_root_id` + `output_relative_path` だけを公開します。直接 `path`
argument は登録されず、root policy を迂回できません。

### Transactional write と result limit

`--root` は read access だけを付与します。`meido.convert_file` と
`meido.extract_archive_entry` の `output_root_id` は `--write-root` で指定した root でなければなりません。writable root
は input root としても利用できます。

write は destination directory に primary/sidecar file をすべて stage し、sync して、既存 target を動かす前に expected
size と SHA-256 を検証します。すべての新 file が install される前に失敗すると previous bundle を復元します。すべての
staged file を install した時点で bundle は commit 済みです。その後の rollback backup removal は best-effort で、cleanup
failure により commit 済み install が failure として報告されることはありません。成功した replacement は、今回生成されない
obsolete managed sidecar を target name から削除し、古い metadata と新しい primary file の誤った組み合わせを防ぎます。
`--max-write-mib` は combined bundle に対して既定 512 MiB です。unrestricted `output_path` も同じ
staging、rollback、sidecar replacement、size limit を使用します。

`meido.inspect_file` は editing JSON を text content として一度返し、structured content には artifact metadata だけを保持します。
`meido.list_archive` は一度に bounded structured page を一つ返し （既定 128、最大 1000 entries）、実際に適用された
`page_size` を返します。次の page には `next_page_token` を使用します。

二つの tool contract は失敗した call から推測するのではなく明示的に宣言されています。`meido.validate_editing_json` は
`root_id` + `relative_path`（unrestricted mode では `path`）、または `editing_json` + `name` を受け付け、空でない
`editing_json` を指定した場合は input schema が `name` を必須にします。`meido.convert_file` は `target` によって input が
持つべき representation を決めます。`target=editing_json` は native game file を読み、`target=native` は
`meido.inspect_file` または以前の `target=editing_json` 変換が生成した editing JSON document を読みます。
`target=native` に native game data を渡すと invalid editing JSON として拒否されます。

stdout には protocol message だけを書き、log は stderr に出力します。最初の MCP implementation は意図的に stdio
のみです。Streamable HTTP は engine を変更せず、別 deployment surface として追加できます。

### MCP tools

| Tool                          | 用途                                                                                   |
|-------------------------------|----------------------------------------------------------------------------------------|
| `meido.detect_file`           | COM3D2/KCES file を detect し、format ID、version、representation を返す               |
| `meido.inspect_file`          | 小さい editing JSON を inline で返す。大きい document には `meido.convert_file` を使用 |
| `meido.validate_editing_json` | 一つの JSON document を公開 Schema で検証し、native serializer で再エンコード。inline `editing_json` には `name` が必須 |
| `meido.convert_file`          | native/editing JSON を変換し、完全な primary/sidecar bundle を destination に install。`target` が input の representation を決める |
| `meido.list_archive`          | ARC、CT/VirtualDirectory、ABA、`.asset_bg`、`.asset_scene` の正確な entry を一覧表示   |
| `meido.extract_archive_entry` | 一つの正確な listed entry を選択 destination へ抽出                                    |

### MCP resources、Prompt、portable editing skill

`meido://capabilities` resource は active `filesystem_mode`、registered format、limit、root ID、Schema/Guide version と
hash、whole-file Guide verification、および次の MCP entry point を記述します。restricted mode は absolute root path を公開しません。

| Resource または Prompt               | 用途                                                                                         |
|--------------------------------------|----------------------------------------------------------------------------------------------|
| `meido://schemas/{format_id}`        | gRPC が返すものと同じ `application/schema+json` structural contract                          |
| `meido://guides/{format_id}`         | gRPC が返す effective field inventory と source-reviewed game-runtime semantics              |
| `meido://skills/editing/{format_id}` | opaque/unreviewed field の保持方法を LLM に示す portable Markdown workflow                   |
| `meido.edit_format`                  | exact Schema、Guide、portable skill、objective、任意 input/output path を埋め込む MCP Prompt |

同じ resource は MCP の format support boundary も宣言するため、client は失敗した detection から境界を推測する必要がありません。
`format_support_boundary` は公開された `formats` list が MCP の完全な support set であることを示します。list に無い file type
は MCP 経由で detect、convert、validate、list されず、`meido.detect_file` は not recognized と報告します。
`cli_only_operations` は command line だけが行う変換を、game、file type、native suffix、CLI command、境界の理由とともに列挙します。
現在は COM3D2 `.nei`（CSV 変換）、COM3D2 `.tex`（image 変換）、suffix ではなく class ID で識別される native Unity Texture2D、
Sprite、Mesh、AnimationClip、AudioClip の primary file、および container 全体の pack/unpack を対象にしています。

portable editing skill は MCP `text/markdown` resource です。Codex skill や MCP Host plugin として
自動インストールされるものではありません。resource だけを読んでも、link された Schema と Guide の代わりにはなりません。skill
は preservation、validation、active filesystem write workflow を定義し、二つの contract は exact structure と reviewed
semantics を提供します。rendered skill には server の現在の filesystem mode に対応する write policy が動的に含まれます。

`meido.edit_format` は `format_id` と `objective` が必須です。MCP protocol の Prompt には input schema が無いため、これらの必須
parameter は Prompt 自身の `arguments` list で宣言され、description でも繰り返されます。Schema と Guide は URI hint
だけではなく、二つの完全な
`EmbeddedResource` として返されるため、LLM は編集前に完全な structural/semantic context を受け取ります。unrestricted mode
では `input_path` と `output_path`、restricted mode では
`input_root_id`、`input_relative_path`、`output_root_id`、`output_relative_path` を任意指定できます。skill、完全な
Schema、完全な Guide を一度に取得する便利な entry point ですが、Prompt は context を準備するだけで file を自動編集しません。

verification は scope ごとに分かれています。

| Scope | 値または claim | 意味 |
|---|---|---|
| `format_verification.level` | `serialization_verified`、`schema_only` | whole-file serialization contract、または生成済み構造だけ |
| `verification.serialization` | `status: verified`、`authority: ai\|human` | field format または read/write behavior。ゲーム内意味は含まない |
| `verification.source_semantics` | `status: verified`、`authority: ai\|human` | game-source semantics。serialization verification を含む |
| `verification.game_behavior` | `status: verified`、`authority: ai\|human` | 実際に観察された game behavior |

空の field `verification` object は Schema 由来の baseline であり、認証ではありません。`field_coverage` は件数 summary として
だけ読み、編集前に必ず exact field の claim を確認してください。

## Cancellation と hard limit

converter は request の `context.Context` と exact output budget を直接受け取ります。controlled file read/write、combined
sidecar size、artifact delivery、archive traversal、serializer writer は I/O boundary で cancellation check と hard byte
limit を適用します。一部の third-party/existing format parser、compression、decompression API は分割できない synchronous
call であり、function 内部から強制停止できません。これらは entry 前と return 後に cancellation を確認するため、cancellation
は後続 output と artifact delivery を防ぎますが、現在の call が戻るまで待つ場合があります。

## Format IDs

format ID は closed protobuf enum ではなく extensible string です。例：

```text
com3d2.menu
com3d2.mate
com3d2.arc
com3d2.timeline
com3d2.object_data
kces.menuassets
kces.dbconf
kces.ct
kces.aba
kces.system
```

registry は現在、COM3D2 menu/mate/pmat/col/phy/psk/anm/model/preset、および KCES parts、payload、
misc、preset、bridge、saved-attach、paths、system data、raw Unity object、CT editing envelope の既存 JSON conversion service
を含みます。archive adapter は ARC、CT/VirtualDirectory、ABA、`.asset_bg`、
`.asset_scene` を扱います。

dance file は `com3d2.timeline`（`timeline_data.bytes`）と `com3d2.object_data`
（`maid_data.bytes`、`item_data.bytes`、`event_data.bytes`）に分かれます。KCES preset は `.preset`
と native `.perset` suffix の両方を受け付けます。detect 可能でも complete validator または safe JSON conversion がない
format（COM3D2 texture など）は detect-only として公開されます。

## Editing Schema の再生成

checked-in catalog には、変換可能な各 registry format の document が一つずつあります。public editing model
を変更した後は再生成してください。

```powershell
go run ./internal/schemagen/cmd -out ./schemas/editing/v1
```

test は generated document と embedded catalog を比較するため、editing model が published Schema から気付かれずにずれることはありません。

## protobuf code の再生成

repository は `buf.yaml` と `buf.gen.yaml` で Buf と remote plugin version を固定しています。Buf 1.72 以降を使用してください。

```text
buf lint
buf generate
```

generated file は source tree の一部です。generation により変更が生じる場合、CI check は失敗します。
`api/gen/go` を手作業で変更せず、proto を変更して再生成してください。
