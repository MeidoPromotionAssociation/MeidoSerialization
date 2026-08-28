package aba

import (
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/binaryio"
)

// 下列常量是提取器识别的常用 Unity class ID
// The constants below are common Unity class IDs recognized by the extractors
const (
	ClassIDGameObject    int32 = 1
	ClassIDTransform     int32 = 4
	ClassIDMaterial      int32 = 21
	ClassIDMeshRenderer  int32 = 23
	ClassIDTexture2D     int32 = 28
	ClassIDMeshFilter    int32 = 33
	ClassIDMesh          int32 = 43
	ClassIDShader        int32 = 48
	ClassIDTextAsset     int32 = 49
	ClassIDAnimationClip int32 = 74
	ClassIDAudioClip     int32 = 83
	ClassIDCubemap       int32 = 89
	ClassIDMonoBehaviour int32 = 114
	ClassIDMonoScript    int32 = 115
	ClassIDFont          int32 = 128
	ClassIDAssetBundle   int32 = 142
	ClassIDSprite        int32 = 213
	ClassIDSpriteAtlas   int32 = 687078895
)

const classIDAssetBundle = ClassIDAssetBundle

// AssetEntry 表示一个可提取的资源条目，包含名称、类型和数据范围信息 / AssetEntry represents one extractable asset entry with name, type, and data-range metadata
type AssetEntry struct {
	PathId   int64  // SerializedFile 中唯一标识对象的路径 ID / Path ID identifying the object within the SerializedFile
	TypeId   int32  // 类型 ID / Unity class ID
	TypeName string // 内置映射得到的类型名称 / Type name obtained from the built-in mapping
	Name     string // 已知布局中位于开头的 m_Name，无法读取时为空 / Leading m_Name from known layouts, or empty when unavailable
	Size     uint32 // 序列化对象数据的字节数 / Serialized object data size in bytes
	Offset   int64  // 对象数据相对于 SerializedFile 数据区的偏移 / Object-data offset relative to the SerializedFile data section
}

// GetAssetEntries 返回 AssetsFile 中全部对象的提取条目，并尝试读取已知布局的 m_Name
// GetAssetEntries returns extraction entries for every object in AssetsFile and attempts to read m_Name from known layouts
func (af *AssetsFile) GetAssetEntries() []AssetEntry {
	if af == nil {
		return nil
	}
	entries := make([]AssetEntry, 0, len(af.Metadata.AssetInfos))
	for _, info := range af.Metadata.AssetInfos {
		entry := AssetEntry{
			PathId:   info.PathId,
			TypeId:   info.TypeId,
			TypeName: classIdToName(info.TypeId),
			Size:     info.ByteSize,
			Offset:   info.ByteOffset,
		}
		// 仅对已确认以 m_Name 开头的原始对象布局尝试读取名称
		// Attempt to read the name only for raw object layouts known to begin with m_Name
		entry.Name = af.tryReadAssetName(&info)
		entries = append(entries, entry)
	}
	return entries
}

// GetTextAssetData 提取 TextAsset 的 m_Name 和 m_Script 字节数组
// 当前支持的对象布局依次为 m_Name 对齐字符串和 m_Script 字节数组
// GetTextAssetData extracts the m_Name and m_Script byte array from a TextAsset
// The supported object layout contains an aligned m_Name string followed by the m_Script byte array
func (af *AssetsFile) GetTextAssetData(info *AssetInfo) (name string, script []byte, err error) {
	data, err := af.GetAssetData(info)
	if err != nil {
		return "", nil, err
	}
	if len(data) < 8 {
		return "", nil, fmt.Errorf("TextAsset data too short: %d bytes", len(data))
	}

	var order binary.ByteOrder
	if af.Header.Endianness {
		order = binary.BigEndian
	} else {
		order = binary.LittleEndian
	}

	r := binaryio.NewEndianReader(data, order)

	// 1.读取 m_Name，其线格式为 Int32 长度、字符串字节和四字节对齐填充
	// 1. read m_Name, encoded as an Int32 length, string bytes, and four-byte alignment padding
	name, err = r.ReadAlignedString()
	if err != nil {
		return "", nil, fmt.Errorf("read m_Name failed: %w", err)
	}

	// 2. m_Script，其线格式为 Int32 长度和原始字节
	// 2. read m_Script, encoded as an Int32 length followed by raw bytes
	scriptLen, err := r.ReadInt32()
	if err != nil {
		return name, nil, fmt.Errorf("read m_Script length failed: %w", err)
	}
	if scriptLen < 0 || int64(scriptLen) > r.Remaining() {
		return name, nil, fmt.Errorf("invalid m_Script length: %d", scriptLen)
	}
	script = make([]byte, scriptLen)
	if err := r.ReadFull(script); err != nil {
		return name, nil, fmt.Errorf("read m_Script data failed: %w", err)
	}
	if err := alignReader4(r); err != nil {
		return name, nil, fmt.Errorf("align m_Script failed: %w", err)
	}
	if r.Remaining() != 0 {
		return name, nil, fmt.Errorf("TextAsset has %d unread bytes", r.Remaining())
	}

	return name, script, nil
}

// tryReadAssetName 对已确认具有前置 m_Name 的对象类型尝试读取名称
// tryReadAssetName attempts to read a name only for object types confirmed to have a leading m_Name
func (af *AssetsFile) tryReadAssetName(info *AssetInfo) string {
	if af == nil || info == nil || !rawObjectHasLeadingName(info.TypeId) {
		return ""
	}
	data, err := af.GetAssetData(info)
	if err != nil {
		return ""
	}

	var order binary.ByteOrder
	if af.Header.Endianness {
		order = binary.BigEndian
	} else {
		order = binary.LittleEndian
	}

	r := binaryio.NewEndianReader(data, order)
	name, err := r.ReadAlignedString()
	if err != nil {
		return ""
	}
	return name
}

// classIdToName 将提取器已知的 Unity class ID 转换为可读名称
// classIdToName converts Unity class IDs known to the extractor into readable names
func classIdToName(id int32) string {
	switch id {
	case ClassIDGameObject:
		return "GameObject"
	case ClassIDTransform:
		return "Transform"
	case ClassIDMaterial:
		return "Material"
	case ClassIDMeshRenderer:
		return "MeshRenderer"
	case ClassIDTexture2D:
		return "Texture2D"
	case ClassIDMeshFilter:
		return "MeshFilter"
	case ClassIDMesh:
		return "Mesh"
	case ClassIDShader:
		return "Shader"
	case ClassIDTextAsset:
		return "TextAsset"
	case ClassIDAnimationClip:
		return "AnimationClip"
	case ClassIDAudioClip:
		return "AudioClip"
	case ClassIDCubemap:
		return "Cubemap"
	case ClassIDMonoBehaviour:
		return "MonoBehaviour"
	case ClassIDMonoScript:
		return "MonoScript"
	case ClassIDFont:
		return "Font"
	case ClassIDAssetBundle:
		return "AssetBundle"
	case ClassIDSprite:
		return "Sprite"
	case ClassIDSpriteAtlas:
		return "SpriteAtlas"
	default:
		return fmt.Sprintf("Type_%d", id)
	}
}

// GetTypeTreeString 从 StringBuffer 或 Unity 内置公共字符串表解析类型树节点字符串
// GetTypeTreeString resolves a type-tree node string from StringBuffer or Unity's built-in common string table
func (tt *TypeTreeType) GetTypeTreeString(node *TypeTreeNode, isType bool) string {
	var offset uint32
	if isType {
		offset = node.TypeStrOff
	} else {
		offset = node.NameStrOff
	}

	// 偏移最高位为 1 时，其余位索引 Unity 内置公共字符串表
	// When the high bit is set, the remaining bits index Unity's built-in common string table
	if offset&0x80000000 != 0 {
		return getBuiltinString(offset & 0x7FFFFFFF)
	}

	// 普通偏移从当前 TypeTree 的 StringBuffer 读取以 NUL 结尾的字符串
	// A regular offset reads a NUL-terminated string from this TypeTree's StringBuffer
	if int64(offset) >= int64(len(tt.StringBuffer)) {
		return ""
	}
	end := int64(offset)
	for end < int64(len(tt.StringBuffer)) && tt.StringBuffer[end] != 0 {
		end++
	}
	return string(tt.StringBuffer[offset:end])
}

// getBuiltinString 按偏移读取 Unity 内置公共类型或字段名称
// getBuiltinString reads a Unity built-in common type or field name by offset
func getBuiltinString(offset uint32) string {
	return readCommonString(commonStringTable, offset)
}

const commonStringTable = "AABB\x00AnimationClip\x00AnimationCurve\x00AnimationState\x00Array\x00Base\x00BitField\x00bitset\x00bool\x00char\x00ColorRGBA\x00Component\x00data\x00deque\x00double\x00dynamic_array\x00FastPropertyName\x00first\x00float\x00Font\x00GameObject\x00Generic Mono\x00GradientNEW\x00GUID\x00GUIStyle\x00int\x00list\x00long long\x00map\x00Matrix4x4f\x00MdFour\x00MonoBehaviour\x00MonoScript\x00m_ByteSize\x00m_Curve\x00m_EditorClassIdentifier\x00m_EditorHideFlags\x00m_Enabled\x00m_ExtensionPtr\x00m_GameObject\x00m_Index\x00m_IsArray\x00m_IsStatic\x00m_MetaFlag\x00m_Name\x00m_ObjectHideFlags\x00m_PrefabInternal\x00m_PrefabParentObject\x00m_Script\x00m_StaticEditorFlags\x00m_Type\x00m_Version\x00Object\x00pair\x00PPtr<Component>\x00PPtr<GameObject>\x00PPtr<Material>\x00PPtr<MonoBehaviour>\x00PPtr<MonoScript>\x00PPtr<Object>\x00PPtr<Prefab>\x00PPtr<Sprite>\x00PPtr<TextAsset>\x00PPtr<Texture>\x00PPtr<Texture2D>\x00PPtr<Transform>\x00Prefab\x00Quaternionf\x00Rectf\x00RectInt\x00RectOffset\x00second\x00set\x00short\x00size\x00SInt16\x00SInt32\x00SInt64\x00SInt8\x00staticvector\x00string\x00TextAsset\x00TextMesh\x00Texture\x00Texture2D\x00Transform\x00TypelessData\x00UInt16\x00UInt32\x00UInt64\x00UInt8\x00unsigned int\x00unsigned long long\x00unsigned short\x00vector\x00Vector2f\x00Vector3f\x00Vector4f\x00m_ScriptingClassIdentifier\x00Gradient\x00Type*\x00int2_storage\x00int3_storage\x00BoundsInt\x00m_CorrespondingSourceObject\x00m_PrefabInstance\x00m_PrefabAsset\x00FileSize\x00Hash128\x00RenderingLayerMask\x00"

// readCommonString 从 NUL 分隔的公共字符串表中读取指定偏移处的字符串
// readCommonString reads the string at an offset in a NUL-separated common string table
func readCommonString(table string, offset uint32) string {
	if int64(offset) >= int64(len(table)) {
		return fmt.Sprintf("Unknown_%d", offset)
	}
	end := int64(offset)
	for end < int64(len(table)) && table[end] != 0 {
		end++
	}
	if end <= int64(offset) {
		return ""
	}
	return strings.TrimSuffix(table[int64(offset):end], "\x00")
}
