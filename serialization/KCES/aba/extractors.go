package aba

import (
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio"
)

// 常用 Unity 类型 ID / Common Unity class IDs
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
	ClassIDMonoBehaviour int32 = 114
	ClassIDMonoScript    int32 = 115
	ClassIDFont          int32 = 128
	ClassIDAssetBundle   int32 = 142
	ClassIDSprite        int32 = 213
	ClassIDSpriteAtlas   int32 = 687078895
)

const classIDAssetBundle = ClassIDAssetBundle

// AssetEntry 表示一个可提取的资源条目（包含名称和类型信息）/ AssetEntry represents one extractable asset entry with name and type metadata
type AssetEntry struct {
	PathId   int64  // 资源路径 ID / Asset PathID
	TypeId   int32  // 类型 ID / Unity class ID
	TypeName string // 类型名称（如 "Texture2D", "TextAsset"）/ Type name such as "Texture2D" or "TextAsset"
	Name     string // 资源名称（从 m_Name 字段读取）/ Asset name read from m_Name
	Size     uint32 // 数据大小 / Asset data size
	Offset   int64  // 数据偏移 / Asset data offset
}

// GetAssetEntries 返回 AssetsFile 中所有资源的条目列表（包含名称）
func (af *AssetsFile) GetAssetEntries() []AssetEntry {
	entries := make([]AssetEntry, 0, len(af.Metadata.AssetInfos))
	for _, info := range af.Metadata.AssetInfos {
		entry := AssetEntry{
			PathId:   info.PathId,
			TypeId:   info.TypeId,
			TypeName: classIdToName(info.TypeId),
			Size:     info.ByteSize,
			Offset:   info.ByteOffset,
		}
		// 尝试读取 m_Name 字段
		entry.Name = af.tryReadAssetName(&info)
		entries = append(entries, entry)
	}
	return entries
}

// GetTextAssetData 提取 TextAsset 的 m_Script 字段内容
// TextAsset 结构: m_Name(AlignedString) + m_Script(byte[])
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

	// 1. m_Name (AlignedString: int32 length + bytes + align4)
	name, err = r.ReadAlignedString()
	if err != nil {
		return "", nil, fmt.Errorf("read m_Name failed: %w", err)
	}

	// 2. m_Script (byte[]: int32 length + bytes)
	scriptLen, err := r.ReadInt32()
	if err != nil {
		return name, nil, fmt.Errorf("read m_Script length failed: %w", err)
	}
	if scriptLen < 0 || int(scriptLen) > r.Remaining() {
		return name, nil, fmt.Errorf("invalid m_Script length: %d", scriptLen)
	}
	script = make([]byte, scriptLen)
	if err := r.ReadFull(script); err != nil {
		return name, nil, fmt.Errorf("read m_Script data failed: %w", err)
	}

	return name, script, nil
}

// tryReadAssetName 尝试从资源数据中读取 m_Name 字段
// 大多数 Unity 资源类型的第一个字段都是 m_Name (AlignedString)
func (af *AssetsFile) tryReadAssetName(info *AssetInfo) string {
	data, err := af.GetAssetData(info)
	if err != nil || len(data) < 5 {
		return ""
	}

	var order binary.ByteOrder
	if af.Header.Endianness {
		order = binary.BigEndian
	} else {
		order = binary.LittleEndian
	}

	// 读取 AlignedString: int32 length + bytes
	nameLen := int32(order.Uint32(data[0:4]))
	if nameLen <= 0 || nameLen > 1024 || int(nameLen)+4 > len(data) {
		return ""
	}
	name := string(data[4 : 4+nameLen])

	// 验证名称是否为合理的 ASCII/UTF-8 字符串
	for _, c := range name {
		if c < 0x20 && c != '\t' && c != '\n' && c != '\r' {
			return ""
		}
	}
	return name
}

// classIdToName 将 Unity 类型 ID 转换为可读名称
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

// GetTypeTreeString 获取类型树节点的字符串（从 StringBuffer 或内置字符串表）
func (tt *TypeTreeType) GetTypeTreeString(node *TypeTreeNode, isType bool) string {
	var offset uint32
	if isType {
		offset = node.TypeStrOff
	} else {
		offset = node.NameStrOff
	}

	// 高位为 1 表示使用内置字符串表
	if offset&0x80000000 != 0 {
		return getBuiltinString(offset & 0x7FFFFFFF)
	}

	// 否则从 StringBuffer 中读取
	if int(offset) >= len(tt.StringBuffer) {
		return ""
	}
	end := int(offset)
	for end < len(tt.StringBuffer) && tt.StringBuffer[end] != 0 {
		end++
	}
	return string(tt.StringBuffer[offset:end])
}

// getBuiltinString 返回 Unity 内置的类型/字段名称字符串
func getBuiltinString(offset uint32) string {
	return readCommonString(commonStringTable, offset)
}

const commonStringTable = "AABB\x00AnimationClip\x00AnimationCurve\x00AnimationState\x00Array\x00Base\x00BitField\x00bitset\x00bool\x00char\x00ColorRGBA\x00Component\x00data\x00deque\x00double\x00dynamic_array\x00FastPropertyName\x00first\x00float\x00Font\x00GameObject\x00Generic Mono\x00GradientNEW\x00GUID\x00GUIStyle\x00int\x00list\x00long long\x00map\x00Matrix4x4f\x00MdFour\x00MonoBehaviour\x00MonoScript\x00m_ByteSize\x00m_Curve\x00m_EditorClassIdentifier\x00m_EditorHideFlags\x00m_Enabled\x00m_ExtensionPtr\x00m_GameObject\x00m_Index\x00m_IsArray\x00m_IsStatic\x00m_MetaFlag\x00m_Name\x00m_ObjectHideFlags\x00m_PrefabInternal\x00m_PrefabParentObject\x00m_Script\x00m_StaticEditorFlags\x00m_Type\x00m_Version\x00Object\x00pair\x00PPtr<Component>\x00PPtr<GameObject>\x00PPtr<Material>\x00PPtr<MonoBehaviour>\x00PPtr<MonoScript>\x00PPtr<Object>\x00PPtr<Prefab>\x00PPtr<Sprite>\x00PPtr<TextAsset>\x00PPtr<Texture>\x00PPtr<Texture2D>\x00PPtr<Transform>\x00Prefab\x00Quaternionf\x00Rectf\x00RectInt\x00RectOffset\x00second\x00set\x00short\x00size\x00SInt16\x00SInt32\x00SInt64\x00SInt8\x00staticvector\x00string\x00TextAsset\x00TextMesh\x00Texture\x00Texture2D\x00Transform\x00TypelessData\x00UInt16\x00UInt32\x00UInt64\x00UInt8\x00unsigned int\x00unsigned long long\x00unsigned short\x00vector\x00Vector2f\x00Vector3f\x00Vector4f\x00m_ScriptingClassIdentifier\x00Gradient\x00Type*\x00int2_storage\x00int3_storage\x00BoundsInt\x00m_CorrespondingSourceObject\x00m_PrefabInstance\x00m_PrefabAsset\x00FileSize\x00Hash128\x00RenderingLayerMask\x00"

func readCommonString(table string, offset uint32) string {
	if int(offset) >= len(table) {
		return fmt.Sprintf("Unknown_%d", offset)
	}
	end := int(offset)
	for end < len(table) && table[end] != 0 {
		end++
	}
	if end <= int(offset) {
		return ""
	}
	return strings.TrimSuffix(table[int(offset):end], "\x00")
}
