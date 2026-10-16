[English](#english) | [简体中文](#简体中文) | [日本語](#日本語)

# English

# Standalone Unity Object Files

`MSKCESO1` is the container MeidoSerialization writes when it takes one Unity object out of a KCES `.aba` bundle  
and stores it as a file that can be read, edited, and packed back on its own. It is a private format. Unity, UABE,  
AssetStudio, AssetRipper, and every other asset tool see such a file as opaque bytes.

## 1. What This Format Is

An `.aba` bundle stores Unity objects as bare payloads. The SerializedFile around them keeps a type table that  
describes each object's fields, but the object itself carries no type name, no field names, and no field offsets.  
Extracting a payload on its own therefore produces a file that nothing can interpret.

This format wraps that payload in a self-describing header carrying:

- the Unity `ClassID`
- the complete TypeTree node table for that class
- the payload's byte order
- the script ID hash and the type hash

The result is one file that stands alone. It needs no sidecar, no external class database, and no Unity  
installation.

The magic reads as `MS` (MeidoSerialization) + `KCES` + `O1` (object, version 1). The format version is also  
stored as a separate `UInt32` immediately after the magic.

## 2. Why It Exists

Three requirements forced a private container.

1. **Stream payloads must be inlined.** Official bundles commonly keep texture pixels and mesh geometry in  
   separate `.resS` entries that the object only references by offset and size. A pure resource directory has no  
   such entries, so unpacking has to fold the stream data into the object itself.
2. **The directory must be independently editable.** Users edit one object at a time, and each edit has to be  
   re-encodable without the rest of the bundle.
3. **The directory must be independently packable.** Packing rebuilds a SerializedFile, which requires a  
   complete TypeTree for every type it writes.

A bare payload satisfies none of the three. A real single-object SerializedFile would satisfy all three and would  
additionally be readable by third-party tools, at the cost of carrying Unity metadata boilerplate (a Unity version  
string, a target platform, script and external-file tables, reference types, and `UserInformation`) and of  
following Unity's version-dependent node layout. This format instead pins the SerializedFile v22 node layout and  
drops everything else.

## 3. Where It Appears

Produced by:

| Command             | Output                                                    |
|---------------------|-----------------------------------------------------------|
| `unpackAba`         | one file per object, except TextAsset                     |
| `convert2texture2d` | native Texture2D                                          |
| `gltf2model`        | native Mesh                                               |
| `convert2json`      | native object JSON, which round-trips back to this format |

Consumed by:

| Command         | Input                                                             |
|-----------------|-------------------------------------------------------------------|
| `packAba`       | reads the header back and writes only the payload into the bundle |
| `convert2image` | native Texture2D and Sprite                                       |
| `convert2gltf`  | native Mesh and AnimationClip                                     |
| `convert2audio` | native AudioClip                                                  |
| `determine`     | reports the type name from the header                             |

`unpackAba` names each file after the object's class:

| ClassID               | Output path                                       |
|-----------------------|---------------------------------------------------|
| 28 Texture2D          | `Texture2D/<name>.tex` or `.texture2d`            |
| 43 Mesh               | `Mesh/<name>.mmesh`                               |
| 213 Sprite            | `Sprite/<name>.sprite`                            |
| 687078895 SpriteAtlas | `SpriteAtlas/<name>.partsatlas` or `.partsassets` |
| 74 AnimationClip      | `AnimationClip/<name>.anm`                        |
| 21 Material           | `Material/<name>.material`                        |
| 83 AudioClip          | `AudioClip/<name>.audioclip`                      |
| 114 MonoBehaviour     | `MonoBehaviour/<name>.monobehaviour`              |
| any other class       | `<TypeName>/<name>.bytes`                         |
| 49 TextAsset          | `TextAsset/<name>`, plain text, not this format   |

91 `AnimatorController`, 95 `Animator`, and 221 `AnimatorOverrideController` are named classes and use the  
generic form: `AnimatorController/<name>.bytes`, `Animator/<name>.bytes`, and  
`AnimatorOverrideController/<name>.bytes`. The IDs run opposite to intuition: 91 is the controller asset,  
95 is the component attached to a GameObject. A directory still named `Type_91` keeps packing correctly.

### The `.bytes` extension is ambiguous

`.bytes` appears in two unrelated roles:

- **this format**, for an object whose class has no dedicated suffix, or for a file produced by  
  `convert2texture2d` and friends
- **a bare payload**, in the legacy `convert` raw-object path and in files exported by third-party tools

The suffix cannot distinguish them. The magic can. Every consumer therefore checks the header first and treats  
the extension only as a cheap prefilter. A bare payload has no header and cannot be packed back into a bundle.

`determine` reports the distinction directly: a file in this format gives `Signature: MSKCESO1` and  
`Version: 1`, while a bare payload gives `Signature: <TypeName>` and `Version: 0`. `Type` stays `bytes` for  
both, because that token belongs to the CLI `--type` vocabulary and must not change.

## 4. Binary Layout

Every integer is little-endian. The file has five consecutive sections:

```
+-------------+---------------+--------------+---------------+-----------+
| header 72 B | nodes N x 32 B | string buffer | deps M x 4 B | payload   |
+-------------+---------------+--------------+---------------+-----------+
```

The total size must equal exactly `72 + N*32 + stringBufferSize + M*4 + dataSize`. Trailing slack is rejected.

Field types use a compact notation: `uN` is an unsigned `N`-bit integer, `iN` is a two's-complement signed  
`N`-bit integer, and `byte[N]` is `N` raw bytes.

### 4.1 Header

| Offset | Type       | Field            | Meaning                                |
|--------|------------|------------------|----------------------------------------|
| 0      | `byte[8]`  | Magic            | literal ASCII `MSKCESO1`               |
| 8      | `u32`      | FormatVersion    | format revision, currently 1           |
| 12     | `i32`      | ClassID          | Unity class ID                         |
| 16     | `u32`      | Flags            | bit 0 stripped, bit 1 big-endian       |
| 20     | `u32`      | NodeCount        | node record count                      |
| 24     | `u32`      | StringBufferSize | local string buffer size in bytes      |
| 28     | `u32`      | DependencyCount  | type dependency count                  |
| 32     | `u32`      | DataSize         | payload size in bytes                  |
| 36     | `byte[16]` | ScriptIdHash     | script ID hash, all zero when unused   |
| 52     | `byte[16]` | TypeHash         | MD4 digest of the node table           |
| 68     | `u32`      | Reserved         | written as zero, rejected when nonzero |

`Flags` is assembled from two booleans: `IsStrippedType` sets bit 0 and `BigEndian` sets bit 1. Both bits are  
read back on load, and any other bit makes the file invalid.

### 4.2 Node Record

The 32-byte layout is the SerializedFile v22 layout, pinned so that no version branching is needed.

| Offset | Type  | Field       | Meaning                              |
|--------|-------|-------------|--------------------------------------|
| 0      | `u16` | Version     | node version, hashed by the TypeHash |
| 2      | `u8`  | Level       | depth, root is 0                     |
| 3      | `u8`  | TypeFlags   | `0x01` means IsArray                 |
| 4      | `u32` | TypeStrOff  | type-name string offset              |
| 8      | `u32` | NameStrOff  | field-name string offset             |
| 12     | `i32` | ByteSize    | `-1` for a composite node            |
| 16     | `i32` | Index       | index within the parent node         |
| 20     | `u32` | MetaFlags   | `0x4000` means AlignBytes            |
| 24     | `u64` | RefTypeHash | referenced-type hash                 |

The widths are exactly the Go field types the writer holds, in this order: `uint16`, `byte`, `byte`, `uint32`,  
`uint32`, `int32`, `int32`, `uint32`, `uint64`.

Both string offsets follow one rule: when the high bit is set, the remaining bits index Unity's built-in common  
string table; otherwise the value is a NUL-terminated string offset inside this file's own string buffer.

### 4.3 Trailing Sections

After the node records come the local string buffer, then the type dependencies as `Int32` entries, then the  
payload. The payload is the object's serialized bytes in Unity's own layout, already migrated to Unity 2022.3 and  
with any external stream data inlined.

### 4.4 Write Order

The writer emits the sections in this exact order, and the reader mirrors it:

1. `Magic`, 8 raw bytes with no length prefix
2. `FormatVersion` as `u32`
3. `ClassID` as `i32`
4. `Flags`, `NodeCount`, `StringBufferSize`, `DependencyCount`, and `DataSize` as five consecutive `u32`
5. `ScriptIdHash`, 16 raw bytes
6. `TypeHash`, 16 raw bytes
7. `Reserved`, one `u32` zero
8. `NodeCount` node records, each 32 bytes in the field order of section 4.2
9. `StringBuffer`, exactly `StringBufferSize` raw bytes
10. `DependencyCount` `i32` values
11. `Data`, exactly `DataSize` raw bytes

Five rules hold for every field:

- **All integers are little-endian**, signed ones included and written in two's complement.
- **There is no alignment and no padding anywhere.** Unity's SerializedFile aligns object data; this container  
  does not, and the size formula has no padding term, so even one inserted pad byte fails the size check.  
  Alignment *inside* the payload is the payload's own business, such as a Texture2D's four-byte-aligned image data.
- **Fixed-width fields are written verbatim.** `ScriptIdHash` and `TypeHash` are 16 raw bytes each, with no  
  length prefix and no terminator.
- **`StringBuffer` is one raw block**, not a list of length-prefixed strings. Its length is `StringBufferSize`,  
  node offsets are relative to its start, and the strings inside are UTF-8 and NUL-terminated.
- **Lengths are validated before anything is emitted.** Every length must fit `u32`, and the three TypeTree  
  lengths must additionally fit the `Int32` range, because packing turns them into SerializedFile `Int32`  
  counts. `DataSize` is bounded only by `u32`. The total must not overflow `Int64`.

Because the four trailing sections are concatenated with no separators, the reader derives each section's start  
from the preceding lengths rather than from any marker.

## 5. Validation Rules

A reader rejects a file when any of these fails:

- the file is shorter than 72 bytes
- the magic is not `MSKCESO1`
- `FormatVersion` is not 1
- `Flags` sets any bit other than 0 and 1
- `Reserved` is not zero
- `NodeCount`, `StringBufferSize`, or `DependencyCount` exceeds the `Int32` range
- the declared total size does not equal the actual file size
- `ClassID` is zero, or differs from the class ID in the node table
- the node table is empty
- a local string offset falls outside the string buffer, or its string is not NUL-terminated

High-bit offsets are deliberately not validated, because they address Unity's common string table rather than  
this file and Unity keeps adding entries to it.

## 6. TypeHash

`TypeHash` is an MD4 digest over the node table, byte for byte identical to the algorithm Unity uses and to  
`ClassDatabaseToTypeTree.ComputeHash` in AssetsTools.NET. For each node, in table order:

```
UTF8(typeName) || UTF8(fieldName) || LE-Int32(ByteSize)
               || LE-Int32(TypeFlags) || LE-Int32(Version)
               || LE-Int32(MetaFlags & 0x4000)
```

The digest is never copied from a sample. It is derived from the node table every time the table is built or  
modified, so an edited table cannot leave a stale hash behind. When a migration reproduces the official node  
table exactly, the derived hash equals the official hash.

## 7. Relationship to `.aba`

Unpacking an object runs the migration to Unity 2022.3 and inlines its stream data, then writes this container.

Packing reads only the payload and the header. The `ClassID` and the node table become one SerializedType record  
in the rebuilt SerializedFile; the payload becomes the object body. The bundle's own `AssetBundle` container  
object and every TextAsset are built from node tables compiled into the library instead.

A SerializedFile cannot mix types that have a TypeTree with types that do not, so the writer requires a complete  
node table for every type it emits. This is why a bare payload fails to pack with `class <n> has no TypeTree`.

## 8. JSON Envelope

`convert2json` turns a file in this format into a JSON envelope with `"format": "kces-unity-native-object"`. The  
envelope carries the decoded field tree, the payload as base64, and `schemaBase64`, which is this same container  
with an empty payload. Editing the field tree and converting back re-encodes the payload with the embedded node  
table and rebuilds the container.

Eight classes are editable. Every other class produces a read-only envelope with `"readOnly": true`, because the  
library cannot promise that an arbitrary edit still matches the class's real layout:

| ClassID   | Class         |
|-----------|---------------|
| 21        | Material      |
| 28        | Texture2D     |
| 43        | Mesh          |
| 74        | AnimationClip |
| 83        | AudioClip     |
| 114       | MonoBehaviour |
| 213       | Sprite        |
| 687078895 | SpriteAtlas   |


The legacy `"format": "kces-unity-raw-object"` envelope is a different thing. It carries a bare payload in  
`dataBase64` plus optional `.meta.json` and `.typetree.json` sidecars. Its `typeTree` field holds a decoded  
*value* view, not a node table, so it cannot rebuild a container and cannot be packed.

## 9. Interoperability

| Tool               | Can open a file in this format |
|--------------------|--------------------------------|
| MeidoSerialization | yes                            |
| Unity Editor       | no                             |
| UABE               | no                             |
| AssetStudio        | no                             |
| AssetRipper        | no                             |

Third-party tools implement Unity's own formats, and this header matches none of them: a Unity SerializedFile  
starts with a version `UInt32` where this format starts with an eight-byte magic. Expect a file in this format  
to be unreadable everywhere except MeidoSerialization.

## 10. Worked Example

`Mesh/crc2_dress006_skrt.mmesh`, produced by `unpackAba` from an official KCES bundle:

```
0000  4d 53 4b 43 45 53 4f 31 01 00 00 00 2b 00 00 00  MSKCESO1....+...
0010  00 00 00 00 ef 00 00 00 4c 04 00 00 00 00 00 00  ........L.......
0020  9c 71 06 00 00 00 00 00 00 00 00 00 00 00 00 00  .q..............
0030  00 00 00 00 c1 8f d5 29 f7 1d bf c0 f9 b3 cd bf  .......)........
0040  b9 8e a7 cd 00 00 00 00 0b 00 00 00 00 00 00 00  ................
```

Decoding the header:

| Field            | Value                              |
|------------------|------------------------------------|
| Magic            | `MSKCESO1`                         |
| FormatVersion    | 1                                  |
| ClassID          | 43, Mesh                           |
| Flags            | 0                                  |
| NodeCount        | 239                                |
| StringBufferSize | 1100                               |
| DependencyCount  | 0                                  |
| DataSize         | 422300                             |
| ScriptIdHash     | all zero                           |
| TypeHash         | `c18fd529f71dbfc0f9b3cdbfb98ea7cd` |
| Reserved         | 0                                  |

The size check holds: `72 + 239*32 + 1100 + 0*4 + 422300 = 431120`, which is the file size. The `TypeHash`  
matches the official Unity 2022.3 Mesh hash, and node 0 begins at offset 72 with version 11, level 0, and  
`MetaFlags` `0x8000`.

---

# 简体中文

# 独立 Unity 对象文件

`MSKCESO1` 是本库把 KCES `.aba` 包里的单个 Unity 对象取出来、存成可独立读取、编辑并回包的文件时使用的容器。  
它是私有格式。Unity、UABE、AssetStudio、AssetRipper 以及其他所有素材工具都只会把它当成一段无法解读的字节。

## 1. 这是什么格式

`.aba` 包把 Unity 对象存成裸载荷。包里的 SerializedFile 维护一张类型表来描述每个对象的字段，但对象自身  
不带类型名、不带字段名、也不带字段偏移。因此单独把载荷抠出来，得到的文件没有任何东西能解读。

本格式把这段载荷包进一个自描述头部，头部携带：

- Unity `ClassID`
- 该类型的完整 TypeTree 节点表
- 载荷的字节序
- 脚本 ID 哈希与类型哈希

于是得到的是一个完全独立的文件，不需要旁车、不需要外部类数据库、也不需要装 Unity。

魔数读作 `MS`（MeidoSerialization）+ `KCES` + `O1`（对象，版本 1）。格式版本另外用紧跟在魔数之后的  
`UInt32` 单独保存。

## 2. 为什么要自造

三个需求逼出了这个私有容器。

1. **流式载荷必须内联。** 官方包常把贴图像素和网格几何放在独立的 `.resS` 条目里，对象只按偏移和长度引用  
   它们。纯资源目录没有这类条目，所以解包必须把流数据折进对象自身。
2. **目录必须能独立编辑。** 用户一次编辑一个对象，每次编辑都要能脱离包内其余部分重新编码。
3. **目录必须能独立回包。** 回包要重建 SerializedFile，而重建要求每个写出的类型都有完整 TypeTree。

裸载荷三条都不满足。一个真·单对象 SerializedFile 三条都满足，而且额外还能被第三方工具读取，代价是要带上  
Unity 的元数据样板（Unity 版本字符串、目标平台、脚本表、外部文件表、引用类型、`UserInformation`），  
并且要跟随 Unity 随版本变化的节点布局。本格式改为钉死 SerializedFile v22 的节点布局，其余全部剥掉。

## 3. 出现在哪里

产出方：

| 命令                | 产出                             |
|---------------------|----------------------------------|
| `unpackAba`         | 每个对象一个文件，TextAsset 除外 |
| `convert2texture2d` | 原生 Texture2D                   |
| `gltf2model`        | 原生 Mesh                        |
| `convert2json`      | 原生对象 JSON，可再转回本格式    |

消费方：

| 命令            | 输入                       |
|-----------------|----------------------------|
| `packAba`       | 读回头部，只把载荷写进包   |
| `convert2image` | 原生 Texture2D 与 Sprite   |
| `convert2gltf`  | 原生 Mesh 与 AnimationClip |
| `convert2audio` | 原生 AudioClip             |
| `determine`     | 从头部报出类型名           |

`unpackAba` 按对象的类型命名文件：

| ClassID               | 输出路径                                        |
|-----------------------|-------------------------------------------------|
| 28 Texture2D          | `Texture2D/<名>.tex` 或 `.texture2d`            |
| 43 Mesh               | `Mesh/<名>.mmesh`                               |
| 213 Sprite            | `Sprite/<名>.sprite`                            |
| 687078895 SpriteAtlas | `SpriteAtlas/<名>.partsatlas` 或 `.partsassets` |
| 74 AnimationClip      | `AnimationClip/<名>.anm`                        |
| 21 Material           | `Material/<名>.material`                        |
| 83 AudioClip          | `AudioClip/<名>.audioclip`                      |
| 114 MonoBehaviour     | `MonoBehaviour/<名>.monobehaviour`              |
| 其他类型              | `<类型名>/<名>.bytes`                           |
| 49 TextAsset          | `TextAsset/<名>`，纯文本，不是本格式            |

91 `AnimatorController`、95 `Animator`、221 `AnimatorOverrideController` 都是有名字的类型，走通用形式：  
`AnimatorController/<名>.bytes`、`Animator/<名>.bytes`、`AnimatorOverrideController/<名>.bytes`。  
这三个 ID 与直觉相反：91 是控制器资源，95 才是挂在 GameObject 上的组件。仍叫 `Type_91` 的旧目录  
依然能正确回包。

### `.bytes` 后缀是有歧义的

`.bytes` 出现在两种互不相干的位置：

- **本格式**，用于类型没有专属后缀的对象，或 `convert2texture2d` 之类的命令产出的文件
- **裸载荷**，出现在旧版 `convert` 的 raw 对象通道，以及第三方工具导出的文件里

后缀无法区分二者，魔数可以。所以每个消费方都先检查文件头，只把后缀当作廉价预筛。裸载荷没有文件头，  
也无法回包。

`determine` 会直接报出这个区别：本格式的文件给出 `Signature: MSKCESO1` 与 `Version: 1`，裸载荷给出  
`Signature: <类型名>` 与 `Version: 0`。两者的 `Type` 都仍是 `bytes`，因为该令牌属于 CLI 的 `--type`  
词汇表，不能改动。

## 4. 二进制布局

所有整数均为小端。文件由五个连续区段组成：

```
+-------------+---------------+--------------+---------------+-----------+
| 头部 72 字节 | 节点 N x 32 字节 | 字符串表      | 依赖 M x 4 字节 | 载荷      |
+-------------+---------------+--------------+---------------+-----------+
```

总长度必须恰好等于 `72 + N*32 + 字符串表长度 + M*4 + 载荷长度`。尾部留有多余字节会被拒绝。

字段类型用紧凑写法表示：`uN` 是 `N` 位无符号整数，`iN` 是 `N` 位补码有符号整数，`byte[N]` 是 `N` 个原始字节。

### 4.1 头部

| 偏移 | 类型       | 字段         | 含义                              |
|------|------------|--------------|-----------------------------------|
| 0    | `byte[8]`  | 魔数         | 字面量 ASCII `MSKCESO1`           |
| 8    | `u32`      | 格式版本     | 格式修订号，当前为 1              |
| 12   | `i32`      | ClassID      | Unity 类型 ID                     |
| 16   | `u32`      | 标志         | 位 0 表示 stripped，位 1 表示大端 |
| 20   | `u32`      | 节点数       | 节点记录条数                      |
| 24   | `u32`      | 字符串表长度 | 本地字符串表字节数                |
| 28   | `u32`      | 依赖数       | 类型依赖条数                      |
| 32   | `u32`      | 载荷长度     | 载荷字节数                        |
| 36   | `byte[16]` | 脚本 ID 哈希 | 未使用时全为零                    |
| 52   | `byte[16]` | 类型哈希     | 节点表的 MD4 摘要                 |
| 68   | `u32`      | 保留         | 写入时为零，读取时非零即拒绝      |

`标志` 由两个布尔值拼成：`IsStrippedType` 置位 0，`BigEndian` 置位 1。加载时两个位都会被读回，
出现其他位则该文件无效。

### 4.2 节点记录

32 字节布局就是 SerializedFile v22 的布局，钉死以便不需要版本分支。

| 偏移 | 类型  | 字段         | 含义                      |
|------|-------|--------------|---------------------------|
| 0    | `u16` | 版本         | 节点版本，参与类型哈希    |
| 2    | `u8`  | 层级         | 深度，根节点为 0          |
| 3    | `u8`  | 类型标志     | `0x01` 表示 IsArray       |
| 4    | `u32` | 类型名偏移   | 类型名字符串偏移          |
| 8    | `u32` | 字段名偏移   | 字段名字符串偏移          |
| 12   | `i32` | 字节数       | 复合节点为 `-1`           |
| 16   | `i32` | 索引         | 父节点内的索引            |
| 20   | `u32` | 元标志       | 位 `0x4000` 为 AlignBytes |
| 24   | `u64` | 引用类型哈希 | 引用类型哈希              |

这些宽度就是写入器持有的 Go 字段类型，顺序为：`uint16`、`byte`、`byte`、`uint32`、`uint32`、
`int32`、`int32`、`uint32`、`uint64`。

两个字符串偏移遵循同一条规则：最高位为 1 时，其余位索引 Unity 内置的公共字符串表；否则该值是本文件自身  
字符串表内一个以 NUL 结尾的字符串偏移。

### 4.3 后续区段

节点记录之后依次是本地字符串表、以 `Int32` 条目表示的类型依赖，以及载荷。载荷是对象按 Unity 自身布局  
序列化后的字节，已经迁移到 Unity 2022.3，并且外部流数据已内联。

### 4.4 写入顺序

写入器按下面的确切顺序输出各区段，读取器与之镜像：

1. `魔数`，8 个原始字节，无长度前缀
2. `格式版本`，`u32`
3. `ClassID`，`i32`
4. `标志`、`节点数`、`字符串表长度`、`依赖数`、`载荷长度`，连续五个 `u32`
5. `脚本 ID 哈希`，16 个原始字节
6. `类型哈希`，16 个原始字节
7. `保留`，一个 `u32` 零
8. `节点数` 条节点记录，每条 32 字节，字段顺序见 4.2
9. `字符串表`，恰好 `字符串表长度` 个原始字节
10. `依赖数` 个 `i32`
11. `载荷`，恰好 `载荷长度` 个原始字节

每个字段都遵守五条规则：

- **所有整数都是小端**，有符号数也不例外，按补码写入。
- **任何地方都没有对齐和填充。** Unity 的 SerializedFile 会对对象数据做对齐；本容器不做，长度公式里也没有  
  填充项，因此插入哪怕一个填充字节都会让长度校验失败。载荷**内部**的对齐属于载荷自己的事，例如  
  Texture2D 的 image data 按四字节对齐。
- **定宽字段原样写入。** `脚本 ID 哈希` 与 `类型哈希` 各是 16 个原始字节，没有长度前缀也没有终止符。
- **`字符串表` 是一整块原始字节**，不是一串带长度前缀的字符串。它的长度是 `字符串表长度`，节点偏移相对它  
  的起点，内部的字符串是 UTF-8 且以 NUL 结尾。
- **写出任何内容之前先校验长度。** 每个长度都必须放得进 `u32`；三个 TypeTree 长度还必须放得进 `Int32`  
  范围，因为回包会把它们变成 SerializedFile 的 `Int32` 计数。`载荷长度` 只受 `u32` 约束。总长度不得  
  溢出 `Int64`。

由于尾部四个区段之间没有任何分隔符，读取器只能靠前面的长度推算出每个区段的起点，而不能依赖任何标记。

## 5. 校验规则

以下任一条不成立，读取方就会拒绝该文件：

- 文件短于 72 字节
- 魔数不是 `MSKCESO1`
- 格式版本不是 1
- 标志置了 0 和 1 以外的位
- 保留字段不为零
- 节点数、字符串表长度或依赖数超出 `Int32` 范围
- 声明的总长度与实际文件长度不符
- `ClassID` 为零，或与节点表中的类型 ID 不一致
- 节点表为空
- 本地字符串偏移越出字符串表，或对应字符串没有 NUL 终止符

最高位偏移被有意跳过校验，因为它们指向 Unity 的公共字符串表而不是本文件，而且 Unity 会持续往里面加条目。

## 6. 类型哈希

`类型哈希` 是对节点表求的 MD4 摘要，逐字节与 Unity 使用的算法、以及 AssetsTools.NET 的  
`ClassDatabaseToTypeTree.ComputeHash` 一致。按节点表顺序，对每个节点依次喂入：

```
UTF8(类型名) || UTF8(字段名) || LE-Int32(字节数)
             || LE-Int32(类型标志) || LE-Int32(版本)
             || LE-Int32(元标志 & 0x4000)
```

摘要从不从样本抄录，而是每次构建或修改节点表后重新推导，因此改过的节点表不会留下过期哈希。当迁移恰好  
复现官方节点表时，推导值就等于官方哈希。

## 7. 与 `.aba` 的关系

解包一个对象时，先执行到 Unity 2022.3 的迁移并内联它的流数据，然后写出本容器。

回包时只读载荷和头部。`ClassID` 与节点表变成重建后的 SerializedFile 里的一条 SerializedType 记录，载荷  
变成对象正文。包自身的 `AssetBundle` 容器对象以及全部 TextAsset 则用编译进库里的节点表构建。

一个 SerializedFile 不能把有 TypeTree 的类型和没有的类型混在一起，因此写入器要求它产出的每个类型都有  
完整节点表。这就是裸载荷回包会报 `class <n> has no TypeTree` 的原因。

## 8. JSON 封套

`convert2json` 把本格式的文件转成 `"format": "kces-unity-native-object"` 的 JSON 封套。封套携带解码后的  
字段树、base64 的载荷，以及 `schemaBase64` —— 后者是同一个容器去掉载荷后的形态。编辑字段树再转回去时，  
会用内嵌的节点表重编码载荷并重建容器。

可编辑的类型有八种。其余类型产出 `"readOnly": true` 的只读封套，因为本库无法承诺任意编辑仍然符合该类型  
的真实布局：

| ClassID   | 类型          |
|-----------|---------------|
| 21        | Material      |
| 28        | Texture2D     |
| 43        | Mesh          |
| 74        | AnimationClip |
| 83        | AudioClip     |
| 114       | MonoBehaviour |
| 213       | Sprite        |
| 687078895 | SpriteAtlas   |

旧版的 `"format": "kces-unity-raw-object"` 封套是另一回事。它把裸载荷放在 `dataBase64` 里，并可选携带  
`.meta.json` 与 `.typetree.json` 旁车。它的 `typeTree` 字段装的是解码后的**值**视图而不是节点表，  
因此既重建不出容器，也无法回包。

## 9. 互操作性

| 工具               | 能否打开本格式的文件 |
|--------------------|----------------------|
| MeidoSerialization | 能                   |
| Unity 编辑器       | 不能                 |
| UABE               | 不能                 |
| AssetStudio        | 不能                 |
| AssetRipper        | 不能                 |

第三方工具实现的是 Unity 自己的格式，而本头部与它们没有一个对得上：Unity 的 SerializedFile 以版本  
`UInt32` 开头，本格式以 8 字节魔数开头。请预期本格式的文件除 MeidoSerialization 之外处处打不开。

## 10. 实例

`Mesh/crc2_dress006_skrt.mmesh`，由 `unpackAba` 从官方 KCES 包解出：

```
0000  4d 53 4b 43 45 53 4f 31 01 00 00 00 2b 00 00 00  MSKCESO1....+...
0010  00 00 00 00 ef 00 00 00 4c 04 00 00 00 00 00 00  ........L.......
0020  9c 71 06 00 00 00 00 00 00 00 00 00 00 00 00 00  .q..............
0030  00 00 00 00 c1 8f d5 29 f7 1d bf c0 f9 b3 cd bf  .......)........
0040  b9 8e a7 cd 00 00 00 00 0b 00 00 00 00 00 00 00  ................
```

头部解码：

| 字段         | 值                                 |
|--------------|------------------------------------|
| 魔数         | `MSKCESO1`                         |
| 格式版本     | 1                                  |
| ClassID      | 43，Mesh                           |
| 标志         | 0                                  |
| 节点数       | 239                                |
| 字符串表长度 | 1100                               |
| 依赖数       | 0                                  |
| 载荷长度     | 422300                             |
| 脚本 ID 哈希 | 全零                               |
| 类型哈希     | `c18fd529f71dbfc0f9b3cdbfb98ea7cd` |
| 保留         | 0                                  |

长度校验成立：`72 + 239*32 + 1100 + 0*4 + 422300 = 431120`，正是文件长度。类型哈希与官方 Unity 2022.3  
的 Mesh 哈希一致，节点 0 从偏移 72 开始，版本 11、层级 0、元标志 `0x8000`。

---

# 日本語

# 単独 Unity オブジェクトファイル

*AI Translated / AI 翻訳*

`MSKCESO1` は、本ライブラリが KCES の `.aba` バンドルから Unity オブジェクトを 1 つ取り出し、単独で読み込み・  
編集・再パッケージできるファイルとして保存するときに使うコンテナです。これは独自形式です。Unity、UABE、  
AssetStudio、AssetRipper など、その他のアセットツールはこのファイルを解釈不能なバイト列として扱います。

## 1. この形式とは

`.aba` バンドルは Unity オブジェクトを生のペイロードとして保存します。バンドル内の SerializedFile は各  
オブジェクトのフィールドを記述する型テーブルを保持しますが、オブジェクト自身は型名もフィールド名も  
フィールドオフセットも持ちません。したがってペイロードだけを取り出すと、解釈できるものがないファイルに  
なります。

本形式はそのペイロードを自己記述ヘッダで包みます。ヘッダが持つ情報は次のとおりです。

- Unity の `ClassID`
- その型の完全な TypeTree ノードテーブル
- ペイロードのバイト順
- スクリプト ID ハッシュと型ハッシュ


これにより完全に独立した 1 ファイルになります。サイドカーも、外部クラスデータベースも、Unity の  
インストールも不要です。

マジックは `MS`（MeidoSerialization）+ `KCES` + `O1`（オブジェクト、バージョン 1）と読みます。形式  
バージョンはマジックの直後に別の `UInt32` としても保存されます。

## 2. なぜ独自形式なのか

3 つの要件が独自コンテナを必要としました。

1. **ストリームペイロードをインライン化する必要がある。** 公式バンドルはテクスチャのピクセルやメッシュの  
   ジオメトリを独立した `.resS` エントリに置き、オブジェクトはオフセットと長さで参照するだけのことが  
   よくあります。純粋なリソースディレクトリにはそうしたエントリがないため、展開時にストリームデータを  
   オブジェクト自身へ折り込む必要があります。
2. **ディレクトリは単独で編集できる必要がある。** ユーザーは一度に 1 つのオブジェクトを編集し、その編集は  
   バンドルの残り部分なしで再エンコードできなければなりません。
3. **ディレクトリは単独で再パッケージできる必要がある。** 再パッケージは SerializedFile を再構築し、  
   そのためには書き出すすべての型に完全な TypeTree が必要です。

生のペイロードはこの 3 つをいずれも満たしません。真の単一オブジェクト SerializedFile は 3 つすべてを満たし、  
さらに第三者のツールからも読めますが、その代償として Unity のメタデータ定型（Unity バージョン文字列、  
ターゲットプラットフォーム、スクリプトテーブル、外部ファイルテーブル、参照型、`UserInformation`）を  
持ち歩き、バージョンごとに変わるノードレイアウトに従う必要があります。本形式は代わりに SerializedFile  
v22 のノードレイアウトを固定し、それ以外をすべて削ぎ落としています。

## 3. どこで現れるか

生成する側：

| コマンド            | 出力                                            |
|---------------------|-------------------------------------------------|
| `unpackAba`         | TextAsset を除き、オブジェクトごとに 1 ファイル |
| `convert2texture2d` | ネイティブ Texture2D                            |
| `gltf2model`        | ネイティブ Mesh                                 |
| `convert2json`      | ネイティブオブジェクト JSON。本形式へ戻せる     |

消費する側：

| コマンド        | 入力                                                 |
|-----------------|------------------------------------------------------|
| `packAba`       | ヘッダを読み戻し、ペイロードだけをバンドルへ書き込む |
| `convert2image` | ネイティブ Texture2D と Sprite                       |
| `convert2gltf`  | ネイティブ Mesh と AnimationClip                     |
| `convert2audio` | ネイティブ AudioClip                                 |
| `determine`     | ヘッダから型名を報告する                             |

`unpackAba` はオブジェクトの型に応じてファイル名を決めます。

| ClassID               | 出力パス                                              |
|-----------------------|-------------------------------------------------------|
| 28 Texture2D          | `Texture2D/<名前>.tex` または `.texture2d`            |
| 43 Mesh               | `Mesh/<名前>.mmesh`                                   |
| 213 Sprite            | `Sprite/<名前>.sprite`                                |
| 687078895 SpriteAtlas | `SpriteAtlas/<名前>.partsatlas` または `.partsassets` |
| 74 AnimationClip      | `AnimationClip/<名前>.anm`                            |
| 21 Material           | `Material/<名前>.material`                            |
| 83 AudioClip          | `AudioClip/<名前>.audioclip`                          |
| 114 MonoBehaviour     | `MonoBehaviour/<名前>.monobehaviour`                  |
| その他の型            | `<型名>/<名前>.bytes`                                 |
| 49 TextAsset          | `TextAsset/<名前>`、プレーンテキスト、本形式ではない  |

91 `AnimatorController`、95 `Animator`、221 `AnimatorOverrideController` は名前を持つ型で、汎用形式を  
使います：`AnimatorController/<名前>.bytes`、`Animator/<名前>.bytes`、  
`AnimatorOverrideController/<名前>.bytes`。この 3 つの ID は直感と逆で、91 がコントローラアセット、  
95 が GameObject に付くコンポーネントです。`Type_91` のままの旧ディレクトリも正しくパックできます。

### `.bytes` 拡張子は曖昧である

`.bytes` は無関係な 2 つの役割で現れます。

- **本形式**。専用の拡張子を持たない型のオブジェクト、または `convert2texture2d` などが生成したファイル
- **生のペイロード**。旧来の `convert` の raw オブジェクト経路、および第三者のツールが出力したファイル

拡張子では両者を区別できません。マジックなら区別できます。そのため各消費側はまずヘッダを確認し、拡張子は  
安価な事前フィルタとしてのみ扱います。生のペイロードにはヘッダがなく、再パッケージもできません。

`determine` はこの違いをそのまま報告します。本形式のファイルは `Signature: MSKCESO1` と `Version: 1` を、  
生のペイロードは `Signature: <型名>` と `Version: 0` を返します。`Type` はどちらも `bytes` のままです。  
このトークンは CLI の `--type` 語彙に属するため変更できません。

## 4. バイナリレイアウト

整数はすべてリトルエンディアンです。ファイルは 5 つの連続した区画からなります。

```
+-------------+---------------+--------------+---------------+-----------+
| ヘッダ 72 B  | ノード N x 32 B | 文字列バッファ | 依存 M x 4 B   | ペイロード |
+-------------+---------------+--------------+---------------+-----------+
```

総サイズは `72 + N*32 + 文字列バッファ長 + M*4 + ペイロード長` に正確に一致しなければなりません。末尾の  
余剰バイトは拒否されます。

フィールド型は簡潔な表記を使います。`uN` は `N` ビットの符号なし整数、`iN` は `N` ビットの 2 の補数の  
符号付き整数、`byte[N]` は `N` 個の生バイトです。

### 4.1 ヘッダ

| オフセット | 型         | フィールド             | 意味                                                |
|------------|------------|------------------------|-----------------------------------------------------|
| 0          | `byte[8]`  | マジック               | リテラル ASCII `MSKCESO1`                           |
| 8          | `u32`      | 形式バージョン         | 形式リビジョン、現在は 1                            |
| 12         | `i32`      | ClassID                | Unity の型 ID                                       |
| 16         | `u32`      | フラグ                 | ビット 0 は stripped、ビット 1 はビッグエンディアン |
| 20         | `u32`      | ノード数               | ノードレコード数                                    |
| 24         | `u32`      | 文字列バッファ長       | ローカル文字列バッファのバイト数                    |
| 28         | `u32`      | 依存数                 | 型依存の件数                                        |
| 32         | `u32`      | ペイロード長           | ペイロードのバイト数                                |
| 36         | `byte[16]` | スクリプト ID ハッシュ | 未使用時はすべてゼロ                                |
| 52         | `byte[16]` | 型ハッシュ             | ノードテーブルの MD4 ダイジェスト                   |
| 68         | `u32`      | 予約                   | 書き込み時はゼロ、読み込み時に非ゼロなら拒否        |

`フラグ` は 2 つの真偽値から組み立てます。`IsStrippedType` がビット 0、`BigEndian` がビット 1 を立てます。  
読み込み時に両方のビットが復元され、それ以外のビットが立っているとそのファイルは無効です。

### 4.2 ノードレコード

32 バイトのレイアウトは SerializedFile v22 のレイアウトそのもので、バージョン分岐が不要になるよう  
固定されています。

| オフセット | 型    | フィールド             | 意味                                   |
|------------|-------|------------------------|----------------------------------------|
| 0          | `u16` | バージョン             | ノードバージョン、型ハッシュに寄与する |
| 2          | `u8`  | レベル                 | 深さ、ルートは 0                       |
| 3          | `u8`  | 型フラグ               | `0x01` は IsArray                      |
| 4          | `u32` | 型名オフセット         | 型名文字列のオフセット                 |
| 8          | `u32` | フィールド名オフセット | フィールド名文字列のオフセット         |
| 12         | `i32` | バイト数               | 複合ノードでは `-1`                    |
| 16         | `i32` | インデックス           | 親ノード内のインデックス               |
| 20         | `u32` | メタフラグ             | ビット `0x4000` が AlignBytes          |
| 24         | `u64` | 参照型ハッシュ         | 参照型ハッシュ                         |

これらの幅は書き込み側が保持する Go のフィールド型そのもので、順に `uint16`、`byte`、`byte`、`uint32`、  
`uint32`、`int32`、`int32`、`uint32`、`uint64` です。

2 つの文字列オフセットは同じ規則に従います。最上位ビットが 1 のとき、残りのビットは Unity 組み込みの  
共通文字列テーブルを指します。そうでなければ、その値は本ファイル自身の文字列バッファ内にある  
NUL 終端文字列のオフセットです。

### 4.3 後続の区画

ノードレコードの後に、ローカル文字列バッファ、`Int32` エントリとしての型依存、そしてペイロードが続きます。  
ペイロードはオブジェクトを Unity 自身のレイアウトでシリアライズしたバイト列で、すでに Unity 2022.3 へ  
移行済みであり、外部ストリームデータもインライン化済みです。

### 4.4 書き込み順序

書き込み側は以下の正確な順序で各区画を出力し、読み込み側はそれを鏡写しにたどります。

1. `マジック`、長さ接頭辞のない 8 生バイト
2. `形式バージョン`、`u32`
3. `ClassID`、`i32`
4. `フラグ`、`ノード数`、`文字列バッファ長`、`依存数`、`ペイロード長`、連続する 5 つの `u32`
5. `スクリプト ID ハッシュ`、16 生バイト
6. `型ハッシュ`、16 生バイト
7. `予約`、`u32` のゼロ 1 つ
8. `ノード数` 個のノードレコード、各 32 バイト、フィールド順は 4.2 のとおり
9. `文字列バッファ`、ちょうど `文字列バッファ長` 生バイト
10. `依存数` 個の `i32`
11. `ペイロード`、ちょうど `ペイロード長` 生バイト

すべてのフィールドに 5 つの規則が当てはまります。

- **整数はすべてリトルエンディアン**で、符号付きも例外ではなく 2 の補数で書きます。
- **アラインメントもパディングもどこにもありません。** Unity の SerializedFile はオブジェクトデータを  
  整列しますが、本コンテナは整列せず、サイズ式にもパディング項がないため、1 バイトのパッドを挿入しただけでも  
  サイズ検証に失敗します。ペイロード**内部**の整列はペイロード自身の都合で、例えば Texture2D の image data は  
  4 バイト境界に整列します。
- **固定幅フィールドはそのまま書きます。** `スクリプト ID ハッシュ` と `型ハッシュ` はそれぞれ 16 生バイトで、  
  長さ接頭辞も終端もありません。
- **`文字列バッファ` は 1 つの生ブロック**であり、長さ接頭辞付き文字列の並びではありません。長さは  
  `文字列バッファ長`、ノードのオフセットはその先頭からの相対値、内部の文字列は UTF-8 かつ NUL 終端です。
- **何かを出力する前に長さを検証します。** 各長さは `u32` に収まる必要があり、3 つの TypeTree 長はさらに  
  `Int32` の範囲に収まる必要があります。再パッケージがそれらを SerializedFile の `Int32` カウントに  
  変えるためです。`ペイロード長` は `u32` によってのみ制限されます。総サイズは `Int64` をオーバーフローしては  
  なりません。

末尾の 4 区画の間には区切りがまったくないため、読み込み側は先行する長さから各区画の開始位置を導くしかなく、  
目印に頼ることはできません。

## 5. 検証規則

次のいずれかが成り立たない場合、読み込み側はそのファイルを拒否します。

- ファイルが 72 バイト未満
- マジックが `MSKCESO1` でない
- 形式バージョンが 1 でない
- フラグが 0 と 1 以外のビットを立てている
- 予約フィールドがゼロでない
- ノード数、文字列バッファ長、依存数のいずれかが `Int32` の範囲を超える
- 宣言された総サイズが実際のファイルサイズと一致しない
- `ClassID` がゼロ、またはノードテーブル内の型 ID と一致しない
- ノードテーブルが空
- ローカル文字列オフセットが文字列バッファの外を指す、またはその文字列が NUL 終端されていない

最上位ビット付きのオフセットは意図的に検証しません。これらは本ファイルではなく Unity の共通文字列テーブルを  
指し、Unity はそこに項目を追加し続けるためです。

## 6. 型ハッシュ

`型ハッシュ` はノードテーブルに対する MD4 ダイジェストで、Unity が使うアルゴリズムおよび AssetsTools.NET の  
`ClassDatabaseToTypeTree.ComputeHash` とバイト単位で一致します。ノードテーブルの順に、各ノードについて  
次を順に投入します。

```
UTF8(型名) || UTF8(フィールド名) || LE-Int32(バイト数)
           || LE-Int32(型フラグ) || LE-Int32(バージョン)
           || LE-Int32(メタフラグ & 0x4000)
```

ダイジェストはサンプルから転記されることはなく、ノードテーブルを構築または変更するたびに導出されるため、  
編集済みのノードテーブルが古いハッシュを残すことはありません。移行が公式のノードテーブルを正確に再現した  
とき、導出値は公式ハッシュと一致します。

## 7. `.aba` との関係

オブジェクトを展開するときは、まず Unity 2022.3 への移行を実行してストリームデータをインライン化し、  
その後で本コンテナを書き出します。

再パッケージ時はペイロードとヘッダだけを読みます。`ClassID` とノードテーブルは再構築される SerializedFile  
の 1 つの SerializedType レコードになり、ペイロードはオブジェクト本体になります。バンドル自身の  
`AssetBundle` コンテナオブジェクトとすべての TextAsset は、ライブラリに組み込まれたノードテーブルから  
構築されます。

1 つの SerializedFile は TypeTree を持つ型と持たない型を混在させられないため、ライターは書き出すすべての型に  
完全なノードテーブルを要求します。これが、生のペイロードの再パッケージが `class <n> has no TypeTree` で  
失敗する理由です。

## 8. JSON エンベロープ

`convert2json` は本形式のファイルを `"format": "kces-unity-native-object"` の JSON エンベロープに変換します。  
エンベロープはデコード済みのフィールドツリー、base64 のペイロード、そして `schemaBase64` を運びます。  
`schemaBase64` は同じコンテナからペイロードを除いた形です。フィールドツリーを編集して戻すと、埋め込まれた  
ノードテーブルでペイロードを再エンコードし、コンテナを再構築します。

編集可能な型は 8 つです。それ以外の型は `"readOnly": true` の読み取り専用エンベロープになります。任意の編集が  
その型の実際のレイアウトに依然として適合することを本ライブラリは保証できないためです。

| ClassID   | 型            |
|-----------|---------------|
| 21        | Material      |
| 28        | Texture2D     |
| 43        | Mesh          |
| 74        | AnimationClip |
| 83        | AudioClip     |
| 114       | MonoBehaviour |
| 213       | Sprite        |
| 687078895 | SpriteAtlas   |

旧来の `"format": "kces-unity-raw-object"` エンベロープは別物です。生のペイロードを `dataBase64` に持ち、  
任意で `.meta.json` と `.typetree.json` のサイドカーを伴います。その `typeTree` フィールドに入っているのは  
ノードテーブルではなくデコード済みの**値**ビューであるため、コンテナを再構築できず、再パッケージもできません。

## 9. 相互運用性

| ツール             | 本形式のファイルを開けるか |
|--------------------|----------------------------|
| MeidoSerialization | 開ける                     |
| Unity エディタ     | 開けない                   |
| UABE               | 開けない                   |
| AssetStudio        | 開けない                   |
| AssetRipper        | 開けない                   |

第三者のツールが実装しているのは Unity 自身の形式であり、本ヘッダはそのいずれとも一致しません。Unity の  
SerializedFile はバージョンの `UInt32` で始まりますが、本形式は 8 バイトのマジックで始まります。本形式の  
ファイルは MeidoSerialization 以外ではどこでも開けないと想定してください。

## 10. 実例

`Mesh/crc2_dress006_skrt.mmesh`。公式 KCES バンドルから `unpackAba` で展開したものです。

```
0000  4d 53 4b 43 45 53 4f 31 01 00 00 00 2b 00 00 00  MSKCESO1....+...
0010  00 00 00 00 ef 00 00 00 4c 04 00 00 00 00 00 00  ........L.......
0020  9c 71 06 00 00 00 00 00 00 00 00 00 00 00 00 00  .q..............
0030  00 00 00 00 c1 8f d5 29 f7 1d bf c0 f9 b3 cd bf  .......)........
0040  b9 8e a7 cd 00 00 00 00 0b 00 00 00 00 00 00 00  ................
```

ヘッダのデコード結果：

| フィールド             | 値                                 |
|------------------------|------------------------------------|
| マジック               | `MSKCESO1`                         |
| 形式バージョン         | 1                                  |
| ClassID                | 43、Mesh                           |
| フラグ                 | 0                                  |
| ノード数               | 239                                |
| 文字列バッファ長       | 1100                               |
| 依存数                 | 0                                  |
| ペイロード長           | 422300                             |
| スクリプト ID ハッシュ | すべてゼロ                         |
| 型ハッシュ             | `c18fd529f71dbfc0f9b3cdbfb98ea7cd` |
| 予約                   | 0                                  |

サイズ検証は成立します。`72 + 239*32 + 1100 + 0*4 + 422300 = 431120` で、これはファイルサイズです。  
型ハッシュは公式 Unity 2022.3 の Mesh ハッシュと一致し、ノード 0 はオフセット 72 から始まり、バージョン 11、  
レベル 0、メタフラグ `0x8000` です。
