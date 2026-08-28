package KCES

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES"
)

// KCES 中原生文件本身就是 Unity JsonUtility 文档的扩展名共用的 service 层调度
// 这些格式的编辑 JSON 与原生文件是同一份文档，唯一的区分标记是 KCES 独有的双扩展名
// Shared service-layer dispatch for the KCES extensions whose native file is itself a Unity JsonUtility document
// The editing JSON of these formats is the same document as the native file, and the only distinguishing marker is their KCES-only double extension

// validateKCESUnityJSONDocument 按扩展名声明的领域模型严格校验一份 Unity JsonUtility 文档
// validateKCESUnityJSONDocument strictly validates one Unity JsonUtility document against the domain model its extension declares
func validateKCESUnityJSONDocument(data []byte, extension string) error {
	switch serializationKCES.NormalizeKCESUnityJSONDocumentExtension(extension) {
	case serializationKCES.KCESUndressDataExtension:
		_, err := serializationKCES.DecodeKCESUndressData(data)
		return err
	case serializationKCES.KCESUndressPartsDataExtension:
		_, err := serializationKCES.DecodeKCESUndressPartsData(data)
		return err
	default:
		return fmt.Errorf("unsupported KCES Unity JSON document extension %q", extension)
	}
}

// encodeKCESUnityJSONDocumentJSON 严格解码编辑 JSON 并编码指定扩展名的原生 Unity JsonUtility 文档
// encodeKCESUnityJSONDocumentJSON strictly decodes editing JSON and encodes the native Unity JsonUtility document for the requested extension
func encodeKCESUnityJSONDocumentJSON(data []byte, extension string) ([]byte, error) {
	switch serializationKCES.NormalizeKCESUnityJSONDocumentExtension(extension) {
	case serializationKCES.KCESUndressDataExtension:
		return encodeUndressDataJSON(data)
	case serializationKCES.KCESUndressPartsDataExtension:
		return encodeUndressPartsDataJSON(data)
	default:
		return nil, fmt.Errorf("unsupported KCES Unity JSON document extension %q", extension)
	}
}

// UndressPairExtension 返回与给定 .undressdat 或 .undresspdat 配对的另一个扩展名，其余输入返回空串
// UndressPairExtension returns the extension paired with a given .undressdat or .undresspdat and an empty string for any other input
func UndressPairExtension(pathOrExt string) string {
	switch serializationKCES.NormalizeKCESUnityJSONDocumentExtension(pathOrExt) {
	case serializationKCES.KCESUndressDataExtension:
		return serializationKCES.KCESUndressPartsDataExtension
	case serializationKCES.KCESUndressPartsDataExtension:
		return serializationKCES.KCESUndressDataExtension
	default:
		return ""
	}
}

// MissingUndressPairWarning 检查同目录下的配对文件，缺失时返回双语提示文本，其余情况返回空串
// 编辑 JSON 也算配对文件存在，因为分两步转换一对文件时另一半可能还只有 .json 形式
// MissingUndressPairWarning checks for the paired file in the same directory and returns bilingual guidance when it is absent, and an empty string otherwise
// Editing JSON counts as the paired file being present, because converting a pair in two steps can leave the other half in .json form only
func MissingUndressPairWarning(nativePath string) string {
	pairExtension := UndressPairExtension(nativePath)
	if pairExtension == "" {
		return ""
	}
	pairPath := strings.TrimSuffix(nativePath, filepath.Ext(nativePath)) + pairExtension
	for _, candidate := range []string{pairPath, pairPath + ".json"} {
		if _, err := os.Stat(candidate); err == nil {
			return ""
		}
	}
	return undressPairWarningText(filepath.Base(nativePath), filepath.Base(pairPath))
}

// undressPairWarningText 返回一段说明配对文件缺失后果的双语提示
// WearSetuper 同时读取 <内衣名>.undressdat 与 <内衣名>.undresspdat，任一缺失或长度为零都会直接中止整套脱衣设置，
// 因此内衣仍能加载但完全没有扒开效果，而不是退化成无缓存的可用状态
// undressPairWarningText returns bilingual guidance describing what a missing paired file causes
// WearSetuper reads both <garment>.undressdat and <garment>.undresspdat and aborts the whole undress setup when either one is
// missing or zero length, so the garment still loads but has no peel behavior at all rather than degrading to a cache-less usable state
func undressPairWarningText(presentName string, missingName string) string {
	return fmt.Sprintf("\n\n%q has no paired %q; WearSetuper reads both files and aborts the whole undress setup when either one is missing, so the garment loads without any peel behavior (keep and edit the two files together)\n\n%q 缺少配对的 %q；WearSetuper 会同时读取两个文件，任一缺失就会中止整套脱衣设置，因此该内衣加载后完全没有扒开效果（这两个文件需要一起保留和编辑）\n",
		presentName, missingName, presentName, missingName)
}
