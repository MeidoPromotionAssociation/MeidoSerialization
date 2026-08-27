package KCES

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/internal/strictjson"
)

// KCES 中原生文件本身就是 Unity JsonUtility 明文 JSON 文档的扩展名共用的编解码约定
//
// 这些资源由 JsonUtility.FromJson<T> 直接读入一个 [Serializable] 类，因此本库为它们建模了完整的
// 领域结构，而不是当作不透明 JSON 值保留
//
// JsonUtility 的成员语义决定了两条建模规则：
//   - FromJson 忽略未知成员、并让缺失成员保持默认值，所以每个成员都是可选指针，解码保留原文件出现过
//     的成员，编码只写回这些成员，不会注入原文件没有的成员
//   - 枚举型成员按线格式的整数原样保留，取值集合记录在格式指南里而不在解码器中强制，因为游戏自身的
//     版本迁移逻辑会保留已废弃的旧取值
//
// Shared codec conventions for the extensions whose native KCES file is itself a Unity JsonUtility
// plain-JSON document
//
// The game reads these resources with JsonUtility.FromJson<T> straight into a [Serializable] class,
// so this library models their full domain structure instead of retaining an opaque JSON value
//
// JsonUtility member semantics dictate two modeling rules:
//   - FromJson ignores unknown members and leaves missing members at their defaults, so every member
//     is an optional pointer: decoding keeps exactly the members the original file contained and
//     encoding writes back only those members without injecting new ones
//   - Enum-typed members are preserved as the stored integer and their value sets are recorded in the
//     format guide rather than enforced by the decoder, because the game's own version-migration
//     logic keeps obsolete legacy values readable

// kcesUnityJSONDocumentIndent 是 Unity JsonUtility 美化输出使用的缩进，原生文件按同样的缩进写回
// kcesUnityJSONDocumentIndent is the indent Unity JsonUtility pretty-printing uses, and native files are written back with the same indent
const kcesUnityJSONDocumentIndent = "    "

// kcesUnityJSONDocumentExtensions 列出原生文件为 Unity JsonUtility 文档的受支持扩展名
// kcesUnityJSONDocumentExtensions lists the supported extensions whose native file is a Unity JsonUtility document
var kcesUnityJSONDocumentExtensions = [...]string{
	KCESUndressDataExtension,
	KCESUndressPartsDataExtension,
}

// IsKCESUnityJSONDocumentExtension 判断扩展名或路径的原生文件是否为受支持的 Unity JsonUtility 文档
// IsKCESUnityJSONDocumentExtension reports whether the native file of an extension or path is a supported Unity JsonUtility document
func IsKCESUnityJSONDocumentExtension(pathOrExt string) bool {
	return NormalizeKCESUnityJSONDocumentExtension(pathOrExt) != ""
}

// NormalizeKCESUnityJSONDocumentExtension 从路径或扩展名提取规范化的受支持扩展名
// NormalizeKCESUnityJSONDocumentExtension extracts a normalized supported extension from a path or extension
func NormalizeKCESUnityJSONDocumentExtension(pathOrExt string) string {
	lower := strings.ToLower(strings.TrimSpace(filepath.ToSlash(pathOrExt)))
	if lower == "" {
		return ""
	}
	ext := filepath.Ext(lower)
	for _, supported := range kcesUnityJSONDocumentExtensions {
		if ext == supported {
			return supported
		}
	}
	return ""
}

// decodeUnityJSONDocument 严格解码一份 Unity JsonUtility 文档，拒绝未知成员、尾随内容和不可表达的 null
// decodeUnityJSONDocument strictly decodes one Unity JsonUtility document and rejects unknown members, trailing content, and inexpressible null
func decodeUnityJSONDocument(data []byte, out any, description string) error {
	trimmed := bytes.TrimSpace(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}))
	if len(trimmed) == 0 {
		return fmt.Errorf("%s is empty", description)
	}
	if err := strictjson.Decode(trimmed, out); err != nil {
		return fmt.Errorf("decode %s: %w", description, err)
	}
	return nil
}

// encodeUnityJSONDocument 按 Unity JsonUtility 的美化输出约定写出一份文档
// 不转义 HTML 字符并省略末尾换行，使输出与游戏侧写出的原生文件保持同一形状
// encodeUnityJSONDocument writes one document using the Unity JsonUtility pretty-print convention
// It leaves HTML characters unescaped and omits the trailing newline so the output keeps the same shape as the native file written by the game side
func encodeUnityJSONDocument(value any, description string) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", kcesUnityJSONDocumentIndent)
	if err := encoder.Encode(value); err != nil {
		return nil, fmt.Errorf("encode %s: %w", description, err)
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}
