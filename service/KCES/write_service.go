package KCES

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	serializationCOM3D2 "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/COM3D2"
	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

// writeKCESNativeFile 将已完成编码的 KCES 原生数据写入目标文件
// writeKCESNativeFile writes fully encoded native KCES data to the destination file
func writeKCESNativeFile(path string, kind string, data []byte) error {
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write KCES %s file %q: %w", kind, path, err)
	}
	return nil
}

// WriteBridgeFile 将完整的 GP03 bridge 编辑模型直接编码并写入 .brd 文件
// WriteBridgeFile directly encodes a complete GP03 bridge editing model and writes it to a .brd file
func (s *GP03BridgeService) WriteBridgeFile(path string, value *GP03BridgeEditing) error {
	encoded, err := encodeGP03BridgeEditing(value)
	if err != nil {
		return fmt.Errorf("encode GP03 bridge: %w", err)
	}
	return writeKCESNativeFile(path, "GP03 bridge", encoded)
}

// WriteBridgeSessionFile 将 KCES bridge session 结构直接编码并写入 bridge_session.vd 文件
// WriteBridgeSessionFile directly encodes a KCES bridge session value and writes it to a bridge_session.vd file
func (s *BridgeSessionService) WriteBridgeSessionFile(path string, value *serializationKCES.KCESBridgeSession) error {
	encoded, err := serializationKCES.EncodeKCESBridgeSession(value)
	if err != nil {
		return fmt.Errorf("encode KCES bridge session: %w", err)
	}
	return writeKCESNativeFile(path, "bridge session", encoded)
}

// WriteMaidColliderFile 将女仆胶囊碰撞体结构直接编码并写入原生载荷文件
// WriteMaidColliderFile directly encodes a maid capsule collider value and writes it to a native payload file
func (s *MaidColliderService) WriteMaidColliderFile(path string, value *serializationKCES.MaidColliderFile) error {
	encoded, err := serializationKCES.EncodeMaidCollider(value)
	if err != nil {
		return fmt.Errorf("encode KCES maid collider: %w", err)
	}
	return writeKCESNativeFile(path, "maid collider", encoded)
}

// WriteExportNameMapFile 将导出名称映射结构直接编码并写入原生 .enm 文件
// WriteExportNameMapFile directly encodes an export name map value and writes it to a native .enm file
func (s *ExportNameMapService) WriteExportNameMapFile(path string, value *serializationKCES.KCESExportNameMap) error {
	encoded, err := serializationKCES.EncodeKCESExportNameMap(value)
	if err != nil {
		return fmt.Errorf("encode KCES export name map: %w", err)
	}
	return writeKCESNativeFile(path, "export name map", encoded)
}

// WritePathsFile 将资源搜索路径结构直接编码并写入 paths.dat 文件
// WritePathsFile directly encodes a resource search path value and writes it to a paths.dat file
func (s *PathsService) WritePathsFile(path string, value *serializationKCES.KCESPathsFile) error {
	encoded, err := serializationKCES.EncodeKCESPaths(value)
	if err != nil {
		return fmt.Errorf("encode KCES paths.dat: %w", err)
	}
	return writeKCESNativeFile(path, "paths.dat", encoded)
}

// WritePayloadFile 将物理或碰撞载荷封套直接编码并写入对应的原生文件
// WritePayloadFile directly encodes a physics or collider payload envelope and writes it to its native file
func (s *PayloadService) WritePayloadFile(path string, value *serializationKCES.KCESPayloadEnvelope) error {
	expectedExtension := serializationKCES.NormalizeKCESPayloadExtension(path)
	actualExtension := ""
	if value != nil {
		actualExtension = serializationKCES.NormalizeKCESPayloadExtension(value.Extension)
	}
	if expectedExtension == "" {
		return fmt.Errorf("unsupported KCES payload output path %q", path)
	}
	if actualExtension != expectedExtension {
		return fmt.Errorf("KCES payload extension %q does not match output extension %q", actualExtension, expectedExtension)
	}
	encoded, err := serializationKCES.EncodeKCESPayload(value)
	if err != nil {
		return fmt.Errorf("encode KCES payload: %w", err)
	}
	return writeKCESNativeFile(path, "payload", encoded)
}

// WritePresetFile 将完整展开的 KCES 预设直接编码并写入 .preset 或 .perset 文件
// WritePresetFile directly encodes a fully expanded KCES preset and writes it to a .preset or .perset file
func (s *PresetService) WritePresetFile(path string, value *serializationKCES.ExpandedKCESPreset) error {
	encoded, err := serializationKCES.EncodeExpandedKCESPreset(value)
	if err != nil {
		return fmt.Errorf("encode KCES preset: %w", err)
	}
	return writeKCESNativeFile(path, "preset", encoded)
}

// WriteSavedAttachFile 将保存的附着物结构直接编码并写入 .sad 文件
// WriteSavedAttachFile directly encodes a saved-attach value and writes it to a .sad file
func (s *SavedAttachService) WriteSavedAttachFile(path string, value *serializationKCES.SavedAttachFile) error {
	encoded, err := serializationKCES.EncodeSavedAttach(value)
	if err != nil {
		return fmt.Errorf("encode KCES saved-attach data: %w", err)
	}
	return writeKCESNativeFile(path, "saved-attach", encoded)
}

// WriteSystemDataFile 将 KCES 系统数据结构直接编码并写入 system.dat 文件
// WriteSystemDataFile directly encodes a KCES system data value and writes it to a system.dat file
func (s *SystemDataService) WriteSystemDataFile(path string, value *serializationKCES.KCESSystemData) error {
	encoded, err := serializationKCES.EncodeKCESSystemData(value)
	if err != nil {
		return fmt.Errorf("encode KCES system.dat: %w", err)
	}
	return writeKCESNativeFile(path, "system.dat", encoded)
}

// WriteMenuAssetsFile 将菜单资源结构直接编码并写入 .menuassets 文件
// WriteMenuAssetsFile directly encodes a menu-assets value and writes it to a .menuassets file
func (s *PartsService) WriteMenuAssetsFile(path string, value *serializationKCES.MenuAssets) error {
	encoded, err := serializationKCES.EncodeMenuAssets(value)
	if err != nil {
		return fmt.Errorf("encode KCES menuassets: %w", err)
	}
	return writeKCESNativeFile(path, "menuassets", encoded)
}

// WriteMaterialAssetsFile 将材质资源结构直接编码并写入 .materialassets 文件
// WriteMaterialAssetsFile directly encodes a material-assets value and writes it to a .materialassets file
func (s *PartsService) WriteMaterialAssetsFile(path string, value *serializationKCES.MaterialAssets) error {
	encoded, err := serializationKCES.EncodeMaterialAssets(value)
	if err != nil {
		return fmt.Errorf("encode KCES materialassets: %w", err)
	}
	return writeKCESNativeFile(path, "materialassets", encoded)
}

// WritePriorityMaterialAssetsFile 将优先级材质资源结构直接编码并写入 .pmatassets 文件
// WritePriorityMaterialAssetsFile directly encodes a priority-material-assets value and writes it to a .pmatassets file
func (s *PartsService) WritePriorityMaterialAssetsFile(path string, value *serializationKCES.PriorityMaterialAssets) error {
	encoded, err := serializationKCES.EncodePriorityMaterialAssets(value)
	if err != nil {
		return fmt.Errorf("encode KCES pmatassets: %w", err)
	}
	return writeKCESNativeFile(path, "pmatassets", encoded)
}

// WriteModelFile 将 KCES 模型结构直接编码并写入 .model 文件
// WriteModelFile directly encodes a KCES model value and writes it to a .model file
func (s *PartsService) WriteModelFile(path string, value *serializationKCES.Model) error {
	encoded, err := serializationKCES.EncodeModel(value)
	if err != nil {
		return fmt.Errorf("encode KCES model: %w", err)
	}
	return writeKCESNativeFile(path, "model", encoded)
}

// WritePartsFile 根据目标扩展名将对应的部件结构直接编码并写入原生文件
// WritePartsFile directly encodes the matching parts value selected by the destination extension and writes its native file
func (s *PartsService) WritePartsFile(path string, value any) error {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".menuassets":
		assets, ok := value.(*serializationKCES.MenuAssets)
		if !ok && value != nil {
			return fmt.Errorf(".menuassets output requires *KCES.MenuAssets, got %T", value)
		}
		return s.WriteMenuAssetsFile(path, assets)
	case ".materialassets":
		assets, ok := value.(*serializationKCES.MaterialAssets)
		if !ok && value != nil {
			return fmt.Errorf(".materialassets output requires *KCES.MaterialAssets, got %T", value)
		}
		return s.WriteMaterialAssetsFile(path, assets)
	case ".pmatassets":
		assets, ok := value.(*serializationKCES.PriorityMaterialAssets)
		if !ok && value != nil {
			return fmt.Errorf(".pmatassets output requires *KCES.PriorityMaterialAssets, got %T", value)
		}
		return s.WritePriorityMaterialAssetsFile(path, assets)
	case ".model":
		model, ok := value.(*serializationKCES.Model)
		if !ok && value != nil {
			return fmt.Errorf(".model output requires *KCES.Model, got %T", value)
		}
		return s.WriteModelFile(path, model)
	default:
		return fmt.Errorf("unsupported KCES parts output type: %s", filepath.Ext(path))
	}
}

// WriteHitCheckFile 将碰撞检测结构直接编码并写入 .hitcheck 文件
// WriteHitCheckFile directly encodes a hit-check value and writes it to a .hitcheck file
func (s *MiscService) WriteHitCheckFile(path string, value *serializationKCES.HitCheck) error {
	encoded, err := serializationKCES.EncodeHitCheck(value)
	if err != nil {
		return fmt.Errorf("encode KCES hitcheck: %w", err)
	}
	return writeKCESNativeFile(path, "hitcheck", encoded)
}

// WriteJSONTextFile 将 KCES 明文 JSON 结构直接编码并写入对应的原生资源文件
// WriteJSONTextFile directly encodes a KCES plain-JSON value and writes it to its native resource file
func (s *MiscService) WriteJSONTextFile(path string, value *serializationKCES.KCESJSONText) error {
	expectedExtension := serializationKCES.NormalizeKCESJSONTextExtension(path)
	actualExtension := ""
	if value != nil {
		actualExtension = serializationKCES.NormalizeKCESJSONTextExtension(value.Extension)
	}
	if expectedExtension == "" {
		return fmt.Errorf("unsupported KCES JSON-text output path %q", path)
	}
	if actualExtension != expectedExtension {
		return fmt.Errorf("KCES JSON-text extension %q does not match output extension %q", actualExtension, expectedExtension)
	}
	encoded, err := serializationKCES.EncodeKCESJSONText(value)
	if err != nil {
		return fmt.Errorf("encode KCES JSON-text resource: %w", err)
	}
	return writeKCESNativeFile(path, "JSON-text", encoded)
}

// WriteMiscFile 根据目标扩展名将对应的杂项结构直接编码并写入原生文件
// WriteMiscFile directly encodes the matching miscellaneous value selected by the destination extension and writes its native file
func (s *MiscService) WriteMiscFile(path string, value any) error {
	extension := strings.ToLower(filepath.Ext(path))
	if extension == ".hitcheck" {
		hitCheck, ok := value.(*serializationKCES.HitCheck)
		if !ok {
			return fmt.Errorf(".hitcheck output requires *KCES.HitCheck, got %T", value)
		}
		return s.WriteHitCheckFile(path, hitCheck)
	}
	if serializationKCES.IsKCESJSONTextExtension(extension) {
		jsonText, ok := value.(*serializationKCES.KCESJSONText)
		if !ok {
			return fmt.Errorf("%s output requires *KCES.KCESJSONText, got %T", extension, value)
		}
		return s.WriteJSONTextFile(path, jsonText)
	}
	return fmt.Errorf("unsupported KCES misc output type: %s", extension)
}

// WriteDataFile 根据目标扩展名将 KCES 与 COM3D2 共用的数据结构直接编码并写入原生文件
// WriteDataFile directly encodes shared KCES and COM3D2 data selected by the destination extension and writes its native file
func (s *DataService) WriteDataFile(path string, value any) error {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".psk":
		psk, ok := value.(*serializationCOM3D2.Psk)
		if !ok {
			return fmt.Errorf(".psk output requires *COM3D2.Psk, got %T", value)
		}
		return s.WritePskFile(path, psk)
	case ".nei":
		nei, ok := value.(*serializationCOM3D2.Nei)
		if !ok {
			return fmt.Errorf(".nei output requires *COM3D2.Nei, got %T", value)
		}
		return s.WriteNeiFile(path, nei)
	default:
		return fmt.Errorf("unsupported KCES shared data output type: %s", filepath.Ext(path))
	}
}

// WritePskFile 将姿势结构直接编码并写入 .psk 文件
// WritePskFile directly encodes a pose value and writes it to a .psk file
func (s *DataService) WritePskFile(path string, value *serializationCOM3D2.Psk) error {
	if value == nil {
		return fmt.Errorf("encode KCES shared psk: nil psk")
	}
	var encoded bytes.Buffer
	if err := value.Dump(&encoded); err != nil {
		return fmt.Errorf("encode KCES shared psk: %w", err)
	}
	return writeKCESNativeFile(path, "shared psk", encoded.Bytes())
}

// WriteNeiFile 将表格结构直接编码并写入 .nei 文件
// WriteNeiFile directly encodes a table value and writes it to a .nei file
func (s *DataService) WriteNeiFile(path string, value *serializationCOM3D2.Nei) error {
	if value == nil {
		return fmt.Errorf("encode KCES shared nei: nil nei")
	}
	var encoded bytes.Buffer
	if err := value.Dump(&encoded); err != nil {
		return fmt.Errorf("encode KCES shared nei: %w", err)
	}
	return writeKCESNativeFile(path, "shared nei", encoded.Bytes())
}

// WriteCtFile 将内容表结构直接编码并写入 .ct 或 VirtualDirectory 文件
// WriteCtFile directly encodes a content table value and writes it to a .ct or VirtualDirectory file
func (s *CtService) WriteCtFile(path string, value *ct.ContentTable) error {
	var encoded bytes.Buffer
	if err := ct.WriteContentTable(&encoded, value); err != nil {
		return fmt.Errorf("encode KCES content table: %w", err)
	}
	return writeKCESNativeFile(path, "content table", encoded.Bytes())
}

// WriteCtEnvelopeFile 将可编辑内容表封套直接构建并写入 .ct 或 VirtualDirectory 文件
// WriteCtEnvelopeFile directly builds an editable content table envelope and writes it to a .ct or VirtualDirectory file
func (s *CtService) WriteCtEnvelopeFile(path string, value *CtEnvelope) error {
	if value == nil {
		return fmt.Errorf("encode KCES content table envelope: nil envelope")
	}
	if value.Format != CtEnvelopeFormat {
		return fmt.Errorf("unsupported ct envelope format %q", value.Format)
	}
	table, err := buildContentTableFromCtEnvelope(value)
	if err != nil {
		return fmt.Errorf("encode KCES content table envelope: %w", err)
	}
	return s.WriteCtFile(path, table)
}

// WriteRawUnityObjectFile 将原始 Unity 对象封套及其可选旁车直接写入对应文件
// WriteRawUnityObjectFile directly writes a raw Unity object envelope and its optional sidecars to their corresponding files
func (s *RawUnityObjectService) WriteRawUnityObjectFile(path string, value *RawUnityObjectEnvelope) error {
	raw, err := rawUnityObjectData(value)
	if err != nil {
		return fmt.Errorf("encode KCES raw Unity object: %w", err)
	}

	meta := rawAssetMeta{
		PathID:                value.PathID,
		LoadName:              value.LoadName,
		UnityVersion:          value.UnityVersion,
		EngineVersion:         value.EngineVersion,
		TargetPlatform:        value.TargetPlatform,
		AbaVersion:            value.AbaVersion,
		GenerationVersion:     value.GenerationVersion,
		SerializedFileVersion: value.SerializedFileVersion,
	}
	var metaData []byte
	if hasRawAssetMeta(meta) {
		metaData, err = marshalRawAssetMeta(meta)
		if err != nil {
			return err
		}
	}
	var typeTreeData []byte
	if value.TypeTree != nil {
		typeTreeData, err = marshalRawUnityTypeTreeEnvelope(value.TypeTree)
		if err != nil {
			return err
		}
	}

	if err := writeKCESNativeFile(path, "raw Unity object", raw); err != nil {
		return err
	}
	if metaData != nil {
		if err := writeKCESNativeFile(assetMetaPath(path), "raw Unity object metadata", metaData); err != nil {
			return err
		}
	}
	if typeTreeData != nil {
		if err := writeKCESNativeFile(typeTreeSidecarPath(path), "raw Unity object TypeTree", typeTreeData); err != nil {
			return err
		}
	}
	return nil
}

// rawUnityObjectData 校验原始 Unity 对象封套并解码其中的 Base64 数据
// rawUnityObjectData validates a raw Unity object envelope and decodes its Base64 data
func rawUnityObjectData(value *RawUnityObjectEnvelope) ([]byte, error) {
	if value == nil {
		return nil, fmt.Errorf("nil raw Unity object envelope")
	}
	if value.Format != RawUnityObjectFormat {
		return nil, fmt.Errorf("unsupported raw Unity object format %q", value.Format)
	}
	raw, err := base64.StdEncoding.DecodeString(value.DataBase64)
	if err != nil {
		return nil, fmt.Errorf("decode dataBase64: %w", err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("raw Unity object data is empty")
	}
	if value.TypeTree != nil && value.TypeTree.Format != "" && value.TypeTree.Format != RawUnityTypeTreeFormat {
		return nil, fmt.Errorf("unsupported TypeTree sidecar format %q", value.TypeTree.Format)
	}
	return raw, nil
}
