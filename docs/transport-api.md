# Transport API

MeidoSerialization exposes a versioned application API through protobuf/gRPC
and through an MCP stdio server. Both transports call the same
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

Protobuf is a control and transport schema. It does not replace the native
game wire formats, and it does not attempt to map every game struct into
`google.protobuf.Struct`. Editing JSON is sent as UTF-8 bytes so exact JSON
integer tokens, raw MessagePack slots, and custom polymorphic JSON are not
coerced by protobuf. Editing JSON is still standard JSON: NaN and positive or
negative infinity are not representable and are rejected by conversion and
validation.

## gRPC

The control schema is [serialization.proto](../api/proto/meido/serialization/v1/serialization.proto).
Generated Go files are committed under `api/gen/go`. Editing JSON contracts are
versioned Draft 2020-12 documents under `schemas/editing/v1` and are embedded in
the server binary. The service provides:

- `GetCapabilities`, `GetFormatSchema`, `GetFormatGuide`, `Detect`, `Convert`,
  and `Validate` for unary control operations.
- `Upload` (client streaming) and `Download` (server streaming) for blobs.
- `DeleteBlob` with a process-local TTL/size-limited blob store.
- `ListArchive` and `ExtractArchiveEntry` for COM3D2 ARC and KCES CT/ABA
  containers.

Input artifacts use exactly one of:

- `inline_data` with a required filename;
- a server-issued `blob.id`;
- `file { root_id, relative_path }`.

An input can also contain repeated `attachments`. Each attachment declares a
supported suffix (`.meta.json` or `.typetree.json`) and its own inline, blob,
or rooted location. Rooted and local file sources discover adjacent sidecars
automatically. Inline and blob callers must submit them explicitly. Duplicate
or unsupported suffixes are rejected. The inline byte budget applies to the
whole unary artifact bundle, not independently to each inline file.

An empty `format_id` means content detection. A conversion target is either
`REPRESENTATION_NATIVE` or `REPRESENTATION_EDITING_JSON`. Results are returned
inline while the bundle's cumulative inline bytes fit the limit; files that do
not fit are stored as blobs. The default inline limit is 3 MiB (never above
gRPC's default 4 MiB message limit). When a converter emits sidecars,
`ArtifactResult.attachments` returns metadata and an independent inline/blob
location for every file; the primary metadata size continues to describe the
primary file only.

### Contract discovery and strongly typed editing

`GetCapabilities` is the discovery call. Each convertible format advertises
`has_editing_schema`, `editing_schema_version`, `editing_schema_id`, and
`editing_schema_sha256`. It also advertises `has_format_guide`, the Guide ID,
version and SHA-256, and `format_guide_coverage`. Before showing an editor,
fetch both contracts:

```text
GetFormatSchema { format_id: "com3d2.menu" }
GetFormatGuide  { format_id: "com3d2.menu" }
```

The response contains `schema_json` as UTF-8 `application/schema+json` bytes.
Feed those bytes to a Draft 2020-12 validator or code generator such as
quicktype, json-schema-to-typescript, NJsonSchema, or typify. Cache by
`schema_id` and `schema_version`, and verify `sha256` before using a cached
copy. The document describes the exact JSON produced by
`Convert(..., REPRESENTATION_EDITING_JSON)`, including polymorphic `oneOf`
branches, recursive `$defs`, base64 byte fields, and integer ranges.
`Validate` first parses exactly one JSON document and applies this published
Schema, including `additionalProperties` and discriminator constraints. It
then runs the native serializer to enforce cross-field relationships and
wire-level rules that are not encoded in the Schema. Conversion from editing
JSON to native performs the same structural check, so callers cannot bypass
the contract by skipping `Validate`.

`GetFormatGuide` returns UTF-8
`application/vnd.meido.format-guide+json`. A Guide maps JSON paths to Schema
pointers and records field purpose, the code path used by the game, edit role,
risk, constraints, enum meanings, editing guidance, cross-field invariants,
workflow, warnings, and source evidence. For script-like formats it can also
describe command `forms`, their positional arguments, target-build coverage,
and shared source-reviewed `value_sets`. An argument's `value_set_refs` lets a
client resolve values such as a game enum or slot name without guessing from a
C# type name. Its `schema_id` must equal the `GetFormatSchema` response, so a
client never combines documentation with the wrong structural contract.

The checked-in Guides are source-review profiles embedded in the binary. Schema
generation and runtime serving do not read or require the repository's `game`
directory. Unprefixed states record AI source review: `runtime_verified` means
the documented game-runtime paths were checked against the cited source, and
`serialization_verified` means the codec or wire-preservation boundary was
checked without claiming a domain schema for every payload member. Neither is a
human approval claim. The `human_` prefix is reserved for explicit human review;
recognized examples are `human_runtime_verified`,
`human_serialization_verified`, and field-level `human_verified`.
`schema_only` means only the structural shape is known; callers must preserve
those fields and must not infer game behavior from their names. Every published
editing format has a checked-in reviewed profile. The effective Guide merges it
with the complete Schema-derived field inventory, so an individual field can
remain `schema_only` even when the Guide's profile-level coverage is
`runtime_verified`.

The recommended editing sequence is:

1. Call `GetCapabilities` and select the advertised `format_id`.
2. Fetch `GetFormatSchema`, verify its SHA-256, and build or load generated
   types by Schema ID/version.
3. Fetch `GetFormatGuide`, verify its SHA-256 and matching `schema_id`, then
   use only claims whose confidence and coverage support the intended edit.
4. For an LLM, also fetch the MCP editing skill or use the
   `meido.edit_format` prompt described below.
5. Detect and inspect the actual input to obtain its existing values.
6. Make the smallest requested change and preserve every unrelated field.
7. Call `Validate` (or `meido.validate_editing_json`) on the complete document.
8. Convert to native only after validation succeeds.

Native-only formats such as `com3d2.arc` deliberately return `UNIMPLEMENTED`;
an archive listing is not presented as an editing JSON document or Guide.

Upload chunks are capped at 1 MiB. `GetCapabilities` also reports the maximum
single-blob bytes, total in-flight/stored bytes, object count, and display-name
bytes. The CLI defaults are 4 GiB per blob, 16 GiB total, 4096 objects, and a
30-minute TTL. Blobs are process-local temporary objects, not persistent
storage.

When `--blob-dir` is supplied, that exact directory is exclusively locked for
the lifetime of the server. A second process using the same directory fails to
start before stale-file cleanup, so it cannot delete active blobs owned by the
first process. The operating system releases the lock if the process exits or
crashes. With no flag, the server creates and owns a private temporary
directory as before.

`ListArchive` is paged with `page_size` (default 128, maximum 1000) and the
`page_token`/`next_page_token` fields. A response is kept below 2 MiB even when
entry names are unusually long; an individual entry that cannot fit is
reported as `RESOURCE_EXHAUSTED`.

The server is intentionally local-first:

```powershell
MeidoSerialization.exe serve grpc `
  --listen 127.0.0.1:50051 `
  --root mods=C:\Games\COM3D2\Mod `
  --root work=C:\Users\me\MeidoWork
```

gRPC roots are read-only. Conversion results are returned inline or as blobs;
the gRPC service never installs output beneath a configured root.

The listener rejects non-loopback addresses unless `--allow-remote` is
explicitly supplied. That flag enables an unencrypted, unauthenticated
endpoint and is suitable only behind an operator-controlled trusted network
boundary. The project does not run a hosted service and does not provide TLS,
authentication, or authorization middleware yet.

The root IDs are an allow-list, not filesystem paths supplied by clients. A
client may request `mods\hair\foo.menu`, but cannot request an absolute path,
`..`, a volume name, or a path that escapes the configured root. The server
holds `os.Root` handles for the lifetime of the process, which also prevents a
configured directory path from being swapped underneath a running server.

## MCP stdio

Run the MCP server as a child process of an MCP host. With no path-related
flags, MCP uses convenience mode:

```powershell
MeidoSerialization.exe mcp
```

Convenience mode is reported as `filesystem_mode: "unrestricted"` by
`meido://capabilities`. File tools accept `path` and, for tools that write,
`output_path`. Both may be absolute or relative to the MCP server's current
working directory. The server can read or replace any regular file permitted
by its operating-system process account. This mode is intended for a trusted
local MCP host where easy access to files in different directories matters
more than application-level confinement.

For untrusted prompts, shared MCP hosts, or a least-privilege setup, enable
restricted mode by configuring any root:

```powershell
MeidoSerialization.exe mcp `
  --root mods=C:\Games\COM3D2\Mod `
  --write-root work=C:\Users\me\MeidoWork
```

Supplying `--root` or `--write-root` automatically selects
`filesystem_mode: "restricted"`; existing rooted configurations therefore
remain restricted. `--restrict-paths` explicitly selects the same mode and can
be used without roots to deny all file access. Restricted tool schemas expose
only `root_id` plus `relative_path`, and writing tools expose only
`output_root_id` plus `output_relative_path`; direct `path` arguments are not
registered and cannot bypass the root policy.

`--root` grants read access only. `meido.convert_file` and
`meido.extract_archive_entry` require `output_root_id` to name a root supplied
with `--write-root`; a writable root can also be used as an input root. Writes
stage every primary/sidecar file in the destination directory, sync it, and
verify its expected size and SHA-256 before moving any existing target.
Failures before every new file is installed restore the previous bundle. Once
all staged files have been installed, the bundle is committed; removal of
rollback backups is best-effort and cannot turn that committed installation
into a reported failure. A successful replacement removes obsolete managed
sidecars from their target names so stale metadata cannot be paired with a new
primary file.
`--max-write-mib` defaults to 512 MiB for the combined bundle.
Unrestricted `output_path` writes use the same staging, rollback, sidecar
replacement, and size-limit implementation.

`meido.inspect_file` returns editing JSON once as text content and keeps only
artifact metadata in structured content. `meido.list_archive` returns one
bounded structured page at a time (default 128, maximum 1000 entries); use its
`next_page_token` for the next page.

Only protocol messages are written to stdout. Logs go to stderr. The first
MCP implementation intentionally uses stdio; Streamable HTTP can be added as a
separate deployment surface without changing the engine.

Registered tools:

| Tool                          | Purpose                                                                                                  |
|-------------------------------|----------------------------------------------------------------------------------------------------------|
| `meido.detect_file`           | Detect a COM3D2/KCES file and return its format ID, version, and representation.                         |
| `meido.inspect_file`          | Return a small editing JSON document inline. Larger documents should use `meido.convert_file`.           |
| `meido.validate_editing_json` | Validate one JSON document against the published Schema, then re-encode it with the native serializer.   |
| `meido.convert_file`          | Convert native/editing JSON and install the complete primary/sidecar bundle at the selected destination. |
| `meido.list_archive`          | List exact entries in ARC, CT, or ABA.                                                                   |
| `meido.extract_archive_entry` | Extract one exact listed entry at the selected destination.                                              |

The `meido://capabilities` resource describes the active `filesystem_mode`,
registered formats, limits, root IDs, Schema/Guide versions and hashes, Guide
coverage, and the following MCP entry points. Restricted mode does not reveal
absolute root paths:

| Resource or prompt                   | Purpose                                                                                                     |
|--------------------------------------|-------------------------------------------------------------------------------------------------------------|
| `meido://schemas/{format_id}`        | The same `application/schema+json` structural contract returned by gRPC.                                    |
| `meido://guides/{format_id}`         | The effective field inventory and source-reviewed game-runtime semantics returned by gRPC.                  |
| `meido://skills/editing/{format_id}` | A portable Markdown workflow that tells an LLM how to preserve opaque and unreviewed fields.                |
| `meido.edit_format`                  | MCP prompt that embeds the exact Schema, Guide, portable skill, objective, and optional input/output paths. |

`meido.edit_format` requires `format_id` and `objective`. Its Schema and Guide
are returned as two `EmbeddedResource` messages, not merely URI hints, so the
LLM receives the complete structural and semantic context before it edits a
file. In unrestricted mode, optional `input_path` and `output_path` arguments
carry the concrete workflow into the prompt. In restricted mode, the prompt
instead exposes `input_root_id`, `input_relative_path`, `output_root_id`, and
`output_relative_path`.

## Format IDs

Format IDs are extensible strings rather than a closed protobuf enum. Examples
include:

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
menu/mate/pmat/col/phy/psk/anm/model/preset and KCES parts, payload, misc,
preset, bridge, saved-attach, paths, system data, raw Unity objects, and CT
editing envelopes. Archive adapters cover ARC, CT/VirtualDirectory, and ABA.
Dance files are split into `com3d2.timeline` (`timeline_data.bytes`) and
`com3d2.object_data` (`maid_data.bytes`, `item_data.bytes`, or
`event_data.bytes`). KCES presets accept both `.preset` and the native
`.perset` suffix.
Formats which are detectable but have no complete validator or safe JSON
conversion (for example COM3D2 textures) are advertised as detect-only.

The checked-in catalog contains one document for every convertible registry
format. Regenerate it after changing a public editing model:

```powershell
go run ./internal/schemagen/cmd -out ./schemas/editing/v1
```

Tests compare generated documents with the embedded catalog, so an editing
model cannot drift from its published schema silently.

## Regenerating protobuf code

The repository pins Buf and remote plugin versions in `buf.yaml` and
`buf.gen.yaml`. With Buf 1.72 or newer:

```text
buf lint
buf generate
```

Generated files are part of the source tree. A CI check fails if generation
would modify them. Do not hand-edit files under `api/gen/go`; edit the proto
and regenerate instead.

## 简体中文说明

gRPC 和 MCP 都直接调用统一的 `application.Engine`，不会绕过现有
`service/COM3D2` 或 `service/KCES`。protobuf 只负责 RPC 控制面和传输，原生
游戏格式仍由 serialization 包处理；编辑 JSON 作为 UTF-8 `bytes` 传输，避免
`google.protobuf.Struct` 改写整数 token、MessagePack 保留槽位和多态 JSON。
编辑 JSON 仍是标准 JSON，不能表示 NaN 或正负无穷；转换和验证会拒绝这些值。

gRPC 默认监听 `127.0.0.1:50051`，大文件使用 Upload/Download 流和临时 blob；
MCP 第一阶段为 stdio，stdout 只输出协议消息，日志输出 stderr。MCP 提供两种文件
访问模式：不带路径相关参数运行 `MeidoSerialization.exe mcp` 时，默认进入便捷的
`unrestricted` 模式，工具直接使用 `path`/`output_path`，可传绝对路径，也可传相对
MCP 进程当前目录的路径。此模式可访问运行账号有权访问的所有文件，只适合完全可信的
本地 MCP Host。任意配置 `--root` 或 `--write-root` 都会自动切换为 `restricted`
模式；也可以显式添加 `--restrict-paths`，且不配置任何 root 时即为拒绝全部文件访问。
安全模式只注册 `root_id + relative_path` 和
`output_root_id + output_relative_path`，不注册直接路径参数，因此不能通过绝对路径
绕过白名单。`meido://capabilities` 的 `filesystem_mode` 会报告当前模式。
`--allow-remote` 会开启无 TLS、无认证的远程监听，只应在用户自行控制的可信网络中使用。

```powershell
# 便捷模式：直接传 path / output_path
MeidoSerialization.exe mcp

# 安全模式：只允许声明的根目录
MeidoSerialization.exe mcp `
  --root mods=C:\Games\COM3D2\Mod `
  --write-root work=C:\Users\me\MeidoWork
```

调用方可以在转换前构建强类型编辑器。先调用 `GetCapabilities`，再获取
`GetFormatSchema { format_id }` 和 `GetFormatGuide { format_id }`。Schema 是完整
Draft 2020-12 `application/schema+json`，可以直接用于 TypeScript、C#、Rust 等
代码生成器；Guide 则包含每个 JSON 路径对应的 Schema 指针、字段用途、游戏实际
执行路径、编辑角色、风险、枚举含义、跨字段不变量、建议和源码证据。调用方应
分别校验二者的 `sha256`，并确认 Guide 的 `schema_id` 与 Schema 完全一致。

Guide 是对照源码生成并嵌入二进制的审阅 profile。Schema 生成器和运行时都不会读取
或依赖 `game` 目录。无前缀状态表示 AI 源码审阅：`runtime_verified` 表示已对照引用
源码核对游戏运行路径；`serialization_verified` 表示已核对编解码或 wire 保真边界，
但不代表任意载荷成员都有已知领域语义。二者都不表示真人批准。`human_` 前缀只保留
给明确的真人复核，例如 `human_runtime_verified`、
`human_serialization_verified` 和字段级 `human_verified`。`schema_only` 只保证结构，
不得根据字段名推断游戏行为。现在每个已发布的 editing format 都有审阅后的 profile；有效 Guide 会将该
profile 与 Schema 派生的完整字段目录合并，因此 profile 级覆盖为
`runtime_verified` 时，个别未审阅字段仍可能明确标为 `schema_only`。

推荐顺序为：先能力发现，再获取 Schema，再获取 Guide；LLM 场景继续读取
`meido://skills/editing/{format_id}` 或直接使用 `meido.edit_format` Prompt；随后
检测并 inspect 真实文件，只修改目标涉及的最少字段，提交完整 editing JSON 到
`Validate`/`meido.validate_editing_json`，验证成功后才转换回原生格式。MCP 的
`meido.edit_format` 会把 Schema 和 Guide 作为两个 `EmbeddedResource` 直接返回给
LLM，而不是只给链接。协议源文件位于
`api/proto/meido/serialization/v1/serialization.proto`。

在 MCP 安全模式中，`--root id=目录` 只读，`--write-root id=目录` 可读写；转换和
提取的输出必须指定可写根。便捷模式的 `output_path` 与安全模式共用同一套暂存、
同步、提交前 size/SHA-256 校验、回滚、sidecar 替换和大小限制逻辑。所有新文件安装
后即视为提交成功，随后只会尽力清理回滚备份，清理失败不会把已提交安装报告成失败。
gRPC 默认内联上限为 3 MiB，blob 还受
单对象、总字节数、对象数量和名称字节数限制。归档列表使用
`page_size`/`page_token` 分页，单页响应不超过 2 MiB。`page_token` 是由每个 server
实例的随机密钥签名的 opaque HMAC cursor，绑定 token 版本、offset、format ID、源归档
内容和当前排序后的 name/size/kind 列表；篡改、换 server、换归档或归档内容变化都会使
旧 token 失效。格式 ID 包括
`com3d2.timeline`、`com3d2.object_data`，KCES 预设同时支持 `.preset` 和 `.perset`。

归档列表解析使用独立预算，默认最多读取 10 GiB 输入并接受 100,000 个条目；实际值以
gRPC/MCP capabilities 中的 `max_archive_listing_bytes` 和 `max_archive_entries` 为准。
`page_size` 只限制单次响应中的条目数，不会降低解析整个归档目录所需的 CPU、内存或输入
字节，因此不能替代这些解析预算。归档遍历和排序边界会检查请求 context。

KCES raw Unity 文件由主 `.bytes` 文件及可选 `.meta.json`、`.typetree.json`
sidecar 组成。rooted 输入会自动读取相邻 sidecar；inline/blob 输入通过
`ArtifactInput.attachments` 分别提交。gRPC 一个 unary artifact bundle 的内联字节
共享同一个 3 MiB 预算，超出预算的文件会返回 blob。转换结果在
`ArtifactResult.attachments` 中逐文件返回 inline/blob 位置，MCP 则把完整 bundle 一起安装，并删除目标位置上
本次未产生的陈旧 sidecar。

`Validate` 会严格读取一个 JSON 文档，先执行发布的 Draft 2020-12 Schema，再执行
原生 serializer 的跨字段和 wire 校验；editing JSON 转 native 的 `Convert` 也走
同一结构校验。显式 `--blob-dir` 在服务生命周期内由进程独占锁定，第二个使用同一
目录的实例会在清理任何文件前启动失败；不指定时仍使用进程私有临时目录。

转换器直接接收请求 context 和精确输出预算。受控文件读取、写入、sidecar 总量和
serializer writer 会在 I/O 边界执行取消检查及硬上限；但部分第三方/既有格式解析、
压缩或解压 API 是不可分割的同步调用，无法在函数内部强制中断。此类调用会在进入前和
返回后检查 context，所以取消能阻止后续输出和 artifact 交付，但可能要等当前调用返回。
