package KCES

import (
	"context"
	"fmt"
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
func (s *PayloadService) WritePayloadFile(path string, value *serializationKCES.KCESPayloadEnvelope) error {
	switch serializationKCES.NormalizeKCESPayloadExtension(path) {
	case serializationKCES.KCESDBConfExtension:
		return (&DBConfService{}).WriteDBConfFile(path, value)
	case serializationKCES.KCESDBColExtension:
		return (&DBColService{}).WriteDBColFile(path, value)
	case serializationKCES.KCESDB2ConfExtension:
		return (&DB2ConfService{}).WriteDB2ConfFile(path, value)
	case serializationKCES.KCESDSBConfExtension:
		return (&DSBConfService{}).WriteDSBConfFile(path, value)
	case serializationKCES.KCESDSB2ConfExtension:
		return (&DSB2ConfService{}).WriteDSB2ConfFile(path, value)
	case serializationKCES.KCESDSLConfExtension:
		return (&DSLConfService{}).WriteDSLConfFile(path, value)
	case serializationKCES.KCESDSL2ConfExtension:
		return (&DSL2ConfService{}).WriteDSL2ConfFile(path, value)
	case serializationKCES.KCESDSLColExtension:
		return (&DSLColService{}).WriteDSLColFile(path, value)
	case serializationKCES.KCESIKColExtension:
		return (&IKColService{}).WriteIKColFile(path, value)
	case serializationKCES.KCESIKColBytesExtension:
		return (&IKColBytesService{}).WriteIKColBytesFile(path, value)
	case serializationKCES.KCESLimbColExtension:
		return (&LimbColService{}).WriteLimbColFile(path, value)
	default:
		return fmt.Errorf("unsupported KCES payload output path %q", path)
	}
}

// decodeKCESPayloadEditingJSON 严格解码并校验 KCES 载荷编辑封套
// decodeKCESPayloadEditingJSON strictly decodes and validates a KCES payload editing envelope
func decodeKCESPayloadEditingJSON(data []byte, expectedExtension string) (*serializationKCES.KCESPayloadEnvelope, error) {
	var envelope serializationKCES.KCESPayloadEnvelope
	if err := decodeStrictJSON(data, &envelope, "KCES payload JSON"); err != nil {
		return nil, err
	}
	if envelope.Format != serializationKCES.PayloadFormatKCESMessagePack && envelope.Format != serializationKCES.PayloadFormatKCESExportCM {
		return nil, fmt.Errorf("unsupported KCES payload JSON format %q", envelope.Format)
	}
	expected := serializationKCES.NormalizeKCESPayloadExtension(expectedExtension)
	actual := serializationKCES.NormalizeKCESPayloadExtension(envelope.Extension)
	if actual == "" {
		actual = expected
		envelope.Extension = expected
	}
	if actual == "" {
		return nil, fmt.Errorf("unsupported or missing KCES payload extension %q", envelope.Extension)
	}
	if expected != "" && actual != expected {
		return nil, fmt.Errorf("KCES payload envelope extension %q does not match file extension %q", actual, expected)
	}
	envelope.Extension = actual
	if _, err := serializationKCES.EncodeKCESPayload(&envelope); err != nil {
		return nil, err
	}
	return &envelope, nil
}
