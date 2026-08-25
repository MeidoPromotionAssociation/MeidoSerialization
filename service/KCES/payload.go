package KCES

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

// PayloadService 为旧调用方提供所有 KCES 物理与碰撞载荷的兼容分派入口 / PayloadService provides compatibility dispatch for all KCES physics and collider payloads
type PayloadService struct{}

// IsKCESPayloadFile 判断路径是否为受支持的 KCES 物理或碰撞载荷
// IsKCESPayloadFile reports whether a path is a supported KCES physics or collider payload
func IsKCESPayloadFile(path string) bool {
	return !strings.HasSuffix(strings.ToLower(path), ".json") && serializationKCES.NormalizeKCESPayloadExtension(path) != ""
}

// IsKCESPayloadJSONFile 判断路径是否为受支持载荷的编辑 JSON
// IsKCESPayloadJSONFile reports whether a path is editing JSON for a supported payload
func IsKCESPayloadJSONFile(path string) bool {
	if !strings.HasSuffix(strings.ToLower(path), ".json") {
		return false
	}
	base := strings.TrimSuffix(path, filepath.Ext(path))
	return serializationKCES.NormalizeKCESPayloadExtension(base) != ""
}

// ConvertPayloadToJson 根据输入扩展名调用对应的独立载荷 service
// ConvertPayloadToJson dispatches to the independent payload service selected by the input extension
func (s *PayloadService) ConvertPayloadToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	switch serializationKCES.NormalizeKCESPayloadExtension(inputPath) {
	case serializationKCES.KCESDBConfExtension:
		return (&DBConfService{}).ConvertDBConfToJson(ctx, inputPath, outputPath, maxOutputBytes)
	case serializationKCES.KCESDBColExtension:
		return (&DBColService{}).ConvertDBColToJson(ctx, inputPath, outputPath, maxOutputBytes)
	case serializationKCES.KCESDB2ConfExtension:
		return (&DB2ConfService{}).ConvertDB2ConfToJson(ctx, inputPath, outputPath, maxOutputBytes)
	case serializationKCES.KCESDSBConfExtension:
		return (&DSBConfService{}).ConvertDSBConfToJson(ctx, inputPath, outputPath, maxOutputBytes)
	case serializationKCES.KCESDSB2ConfExtension:
		return (&DSB2ConfService{}).ConvertDSB2ConfToJson(ctx, inputPath, outputPath, maxOutputBytes)
	case serializationKCES.KCESDSLConfExtension:
		return (&DSLConfService{}).ConvertDSLConfToJson(ctx, inputPath, outputPath, maxOutputBytes)
	case serializationKCES.KCESDSL2ConfExtension:
		return (&DSL2ConfService{}).ConvertDSL2ConfToJson(ctx, inputPath, outputPath, maxOutputBytes)
	case serializationKCES.KCESDSLColExtension:
		return (&DSLColService{}).ConvertDSLColToJson(ctx, inputPath, outputPath, maxOutputBytes)
	case serializationKCES.KCESIKColExtension:
		return (&IKColService{}).ConvertIKColToJson(ctx, inputPath, outputPath, maxOutputBytes)
	case serializationKCES.KCESIKColBytesExtension:
		return (&IKColBytesService{}).ConvertIKColBytesToJson(ctx, inputPath, outputPath, maxOutputBytes)
	case serializationKCES.KCESLimbColExtension:
		return (&LimbColService{}).ConvertLimbColToJson(ctx, inputPath, outputPath, maxOutputBytes)
	default:
		return fmt.Errorf("unsupported KCES payload file type: %s", inputPath)
	}
}

// ConvertJsonToPayload 根据编辑 JSON 或输出路径的扩展名调用对应的独立载荷 service
// ConvertJsonToPayload dispatches to the independent payload service selected by the editing JSON or output extension
func (s *PayloadService) ConvertJsonToPayload(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	base := strings.TrimSuffix(inputPath, filepath.Ext(inputPath))
	extension := serializationKCES.NormalizeKCESPayloadExtension(base)
	if extension == "" {
		extension = serializationKCES.NormalizeKCESPayloadExtension(outputPath)
	}
	switch extension {
	case serializationKCES.KCESDBConfExtension:
		return (&DBConfService{}).ConvertJsonToDBConf(ctx, inputPath, outputPath, maxOutputBytes)
	case serializationKCES.KCESDBColExtension:
		return (&DBColService{}).ConvertJsonToDBCol(ctx, inputPath, outputPath, maxOutputBytes)
	case serializationKCES.KCESDB2ConfExtension:
		return (&DB2ConfService{}).ConvertJsonToDB2Conf(ctx, inputPath, outputPath, maxOutputBytes)
	case serializationKCES.KCESDSBConfExtension:
		return (&DSBConfService{}).ConvertJsonToDSBConf(ctx, inputPath, outputPath, maxOutputBytes)
	case serializationKCES.KCESDSB2ConfExtension:
		return (&DSB2ConfService{}).ConvertJsonToDSB2Conf(ctx, inputPath, outputPath, maxOutputBytes)
	case serializationKCES.KCESDSLConfExtension:
		return (&DSLConfService{}).ConvertJsonToDSLConf(ctx, inputPath, outputPath, maxOutputBytes)
	case serializationKCES.KCESDSL2ConfExtension:
		return (&DSL2ConfService{}).ConvertJsonToDSL2Conf(ctx, inputPath, outputPath, maxOutputBytes)
	case serializationKCES.KCESDSLColExtension:
		return (&DSLColService{}).ConvertJsonToDSLCol(ctx, inputPath, outputPath, maxOutputBytes)
	case serializationKCES.KCESIKColExtension:
		return (&IKColService{}).ConvertJsonToIKCol(ctx, inputPath, outputPath, maxOutputBytes)
	case serializationKCES.KCESIKColBytesExtension:
		return (&IKColBytesService{}).ConvertJsonToIKColBytes(ctx, inputPath, outputPath, maxOutputBytes)
	case serializationKCES.KCESLimbColExtension:
		return (&LimbColService{}).ConvertJsonToLimbCol(ctx, inputPath, outputPath, maxOutputBytes)
	default:
		return fmt.Errorf("unsupported KCES payload JSON type: %s", inputPath)
	}
}

// WritePayloadFile 根据目标扩展名调用对应的独立载荷 service
// WritePayloadFile dispatches to the independent payload service selected by the destination extension
func (s *PayloadService) WritePayloadFile(path string, value any) error {
	encoded, err := serializationKCES.EncodeKCESPayload(value, path)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return fmt.Errorf("write KCES payload file %q: %w", path, err)
	}
	return nil
}

// decodeKCESPayloadEditingJSON 严格解码指定扩展名的 KCES 载荷编辑 JSON 并校验其可写回
// decodeKCESPayloadEditingJSON strictly decodes KCES payload editing JSON for one extension and validates that it can be encoded back
func decodeKCESPayloadEditingJSON(data []byte, expectedExtension string) (any, error) {
	extension := serializationKCES.NormalizeKCESPayloadExtension(expectedExtension)
	if extension == "" {
		return nil, fmt.Errorf("unsupported or missing KCES payload extension %q", expectedExtension)
	}
	value, err := decodeKCESPayloadRootJSON(data, extension)
	if err != nil {
		return nil, err
	}
	if _, err := serializationKCES.EncodeKCESPayload(value, extension); err != nil {
		return nil, err
	}
	return value, nil
}

// decodeKCESPayloadRootJSON 按扩展名声明的载荷类型严格解码编辑 JSON 根
// decodeKCESPayloadRootJSON strictly decodes an editing JSON root into the payload type declared by an extension
func decodeKCESPayloadRootJSON(data []byte, extension string) (any, error) {
	descriptor, ok := serializationKCES.DescribeKCESPayload(extension)
	if !ok {
		return nil, fmt.Errorf("unsupported KCES payload extension %q", extension)
	}
	label := "KCES " + descriptor.Extension + " JSON"
	trimmed := trimJSONUTF8BOM(data)
	switch descriptor.Kind {
	case serializationKCES.PayloadKindDynamicBoneStatus:
		var value *serializationKCES.DynamicBoneStatus
		err := decodeStrictJSON(trimmed, &value, label)
		return value, err
	case serializationKCES.PayloadKindColliderPackage:
		var value *serializationKCES.ColliderPackage
		err := decodeStrictJSON(trimmed, &value, label)
		return value, err
	case serializationKCES.PayloadKindLimbCollider:
		var value *serializationKCES.LimbColliderPackage
		err := decodeStrictJSON(trimmed, &value, label)
		return value, err
	case serializationKCES.PayloadKindIKCollider:
		var value *serializationKCES.IKColliderPackage
		err := decodeStrictJSON(trimmed, &value, label)
		return value, err
	case serializationKCES.PayloadKindClothParams:
		var value *serializationKCES.ClothParams
		err := decodeStrictJSON(trimmed, &value, label)
		return value, err
	case serializationKCES.PayloadKindJSONString:
		var value *serializationKCES.MagicaClothSerializeData
		err := decodeStrictJSON(trimmed, &value, label)
		return value, err
	default:
		return nil, fmt.Errorf("unsupported KCES payload kind %q for extension %q", descriptor.Kind, descriptor.Extension)
	}
}
