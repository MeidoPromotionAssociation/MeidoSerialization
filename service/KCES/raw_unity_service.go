package KCES

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
)

const RawUnityObjectFormat = "kces-unity-raw-object"

// RawUnityObjectEnvelope 是从 ABA 提取的原始 Unity 序列化对象字节的 JSON 可编辑封套 / RawUnityObjectEnvelope is the JSON-editable wrapper for raw Unity serialized object bytes extracted from an ABA
type RawUnityObjectEnvelope struct {
	Format                string                    `json:"format"`                          // 封套格式标识，固定为 kces-unity-raw-object / Envelope format marker, fixed to kces-unity-raw-object
	ClassID               int32                     `json:"classId"`                         // Unity ClassID / Unity ClassID
	TypeName              string                    `json:"typeName,omitempty"`              // Unity 类型名 / Unity type name
	Kind                  string                    `json:"kind,omitempty"`                  // 打包时使用的资源 kind / Asset kind used during packing
	Name                  string                    `json:"name,omitempty"`                  // 对象内部名称 / Internal object name
	PathID                int64                     `json:"pathId,omitempty"`                // Unity PathID / Unity PathID
	LoadName              string                    `json:"loadName,omitempty"`              // AssetBundle 加载名 / AssetBundle load name
	UnityVersion          string                    `json:"unityVersion,omitempty"`          // SerializedFile Unity 版本 / SerializedFile Unity version
	EngineVersion         string                    `json:"engineVersion,omitempty"`         // UnityFS header 引擎版本 / UnityFS header engine version
	TargetPlatform        *uint32                   `json:"targetPlatform,omitempty"`        // SerializedFile 目标平台 / SerializedFile target platform
	AbaVersion            uint32                    `json:"abaVersion,omitempty"`            // UnityFS 格式版本 / UnityFS format version
	GenerationVersion     string                    `json:"generationVersion,omitempty"`     // UnityFS generation version / UnityFS generation version
	SerializedFileVersion uint32                    `json:"serializedFileVersion,omitempty"` // SerializedFile 格式版本 / SerializedFile format version
	DataBase64            string                    `json:"dataBase64"`                      // 原始序列化对象数据 base64 / Base64 of raw serialized object data
	TypeTree              *RawUnityTypeTreeEnvelope `json:"typeTree,omitempty"`              // 可选 TypeTree 只读视图 / Optional read-only TypeTree view
}

// RawUnityObjectService 提供原始 Unity 对象字节与 JSON 封套之间的转换服务 / RawUnityObjectService converts raw Unity object bytes to and from JSON envelopes
type RawUnityObjectService struct{}

// IsKCESRawUnityBytesFile 判断路径是否为受支持的 KCES 原始 Unity 对象字节文件
// IsKCESRawUnityBytesFile reports whether a path is a supported KCES raw Unity object byte file
func IsKCESRawUnityBytesFile(path string) bool {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".json") || !strings.HasSuffix(lower, ".bytes") {
		return false
	}
	_, _, ok := inferRawUnityObjectKind(path)
	return ok
}

// IsKCESRawUnityBytesJSONFile 判断路径是否为受支持的 KCES 原始 Unity 对象 JSON 文件
// IsKCESRawUnityBytesJSONFile reports whether a path is a supported KCES raw Unity object JSON file
func IsKCESRawUnityBytesJSONFile(path string) bool {
	if !strings.HasSuffix(strings.ToLower(path), ".bytes.json") {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var header struct {
		Format string `json:"format"`
	}
	if err := json.Unmarshal(trimJSONUTF8BOM(data), &header); err != nil {
		return false
	}
	return header.Format == RawUnityObjectFormat
}

// ConvertRawUnityObjectToJson 将 KCES 原始 Unity 对象字节转换为可编辑 JSON 封套
// ConvertRawUnityObjectToJson converts KCES raw Unity object bytes into an editable JSON envelope
func (s *RawUnityObjectService) ConvertRawUnityObjectToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if err := checkConversionContext(ctx); err != nil {
		return err
	}
	envelope, err := s.ReadRawUnityObjectFile(inputPath)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal KCES raw Unity object json: %w", err)
	}
	data = append(data, '\n')
	if err := checkConversionContext(ctx); err != nil {
		return err
	}
	if err := writeConversionFile(ctx, outputPath, data, maxOutputBytes); err != nil {
		return fmt.Errorf("write %q: %w", outputPath, err)
	}
	return nil
}

// ConvertJsonToRawUnityObject 将可编辑 JSON 封套转换回 KCES 原始 Unity 对象字节
// ConvertJsonToRawUnityObject converts an editable JSON envelope back into KCES raw Unity object bytes
func (s *RawUnityObjectService) ConvertJsonToRawUnityObject(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	data, err := readConversionFile(ctx, inputPath)
	if err != nil {
		return fmt.Errorf("read %q: %w", inputPath, err)
	}
	envelope, raw, err := decodeRawUnityObjectEditingJSON(data)
	if err != nil {
		return fmt.Errorf("parse KCES raw Unity object json: %w", err)
	}
	if err := checkConversionContext(ctx); err != nil {
		return err
	}
	budget, err := newConversionBudget(ctx, maxOutputBytes)
	if err != nil {
		return err
	}
	if err := budget.WriteFile(outputPath, raw, 0644); err != nil {
		return fmt.Errorf("write %q: %w", outputPath, err)
	}
	meta := rawAssetMeta{
		PathID:                envelope.PathID,
		LoadName:              envelope.LoadName,
		UnityVersion:          envelope.UnityVersion,
		EngineVersion:         envelope.EngineVersion,
		TargetPlatform:        envelope.TargetPlatform,
		AbaVersion:            envelope.AbaVersion,
		GenerationVersion:     envelope.GenerationVersion,
		SerializedFileVersion: envelope.SerializedFileVersion,
	}
	if hasRawAssetMeta(meta) {
		metaData, err := marshalRawAssetMeta(meta)
		if err != nil {
			return err
		}
		if err := budget.WriteFile(assetMetaPath(outputPath), metaData, 0644); err != nil {
			return fmt.Errorf("write %q: %w", assetMetaPath(outputPath), err)
		}
	}
	if envelope.TypeTree != nil {
		typeTreeData, err := marshalRawUnityTypeTreeEnvelope(envelope.TypeTree)
		if err != nil {
			return err
		}
		if err := budget.WriteFile(typeTreeSidecarPath(outputPath), typeTreeData, 0644); err != nil {
			return fmt.Errorf("write %q: %w", typeTreeSidecarPath(outputPath), err)
		}
	}
	return nil
}

// decodeRawUnityObjectEditingJSON 严格解码原始 Unity 对象编辑封套及其 Base64 数据
// decodeRawUnityObjectEditingJSON strictly decodes a raw Unity object editing envelope and its Base64 data
func decodeRawUnityObjectEditingJSON(data []byte) (*RawUnityObjectEnvelope, []byte, error) {
	var envelope RawUnityObjectEnvelope
	if err := decodeStrictJSON(data, &envelope, "KCES raw Unity object JSON"); err != nil {
		return nil, nil, err
	}
	if envelope.Format != RawUnityObjectFormat {
		return nil, nil, fmt.Errorf("unsupported raw Unity object JSON format %q", envelope.Format)
	}
	raw, err := base64.StdEncoding.DecodeString(envelope.DataBase64)
	if err != nil {
		return nil, nil, fmt.Errorf("decode dataBase64: %w", err)
	}
	if len(raw) == 0 {
		return nil, nil, fmt.Errorf("raw Unity object data is empty")
	}
	if envelope.TypeTree != nil && envelope.TypeTree.Format != "" && envelope.TypeTree.Format != RawUnityTypeTreeFormat {
		return nil, nil, fmt.Errorf("unsupported TypeTree sidecar format %q", envelope.TypeTree.Format)
	}
	return &envelope, raw, nil
}

// ReadRawUnityObjectFile 读取原始 Unity 对象字节文件并构建 JSON 封套
// ReadRawUnityObjectFile reads a raw Unity object byte file and builds its JSON envelope
func (s *RawUnityObjectService) ReadRawUnityObjectFile(path string) (*RawUnityObjectEnvelope, error) {
	kind, classID, ok := inferRawUnityObjectKind(path)
	if !ok {
		return nil, fmt.Errorf("not a supported KCES raw Unity .bytes path: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("raw Unity object %q is empty", path)
	}

	meta, err := readAssetMetaStrict(path)
	if err != nil {
		return nil, err
	}
	name := inferRawUnityObjectName(path, data, meta)
	typeTree, _ := readRawUnityTypeTreeSidecar(path)
	return &RawUnityObjectEnvelope{
		Format:                RawUnityObjectFormat,
		ClassID:               classID,
		TypeName:              unityClassName(classID),
		Kind:                  kind,
		Name:                  name,
		PathID:                meta.PathID,
		LoadName:              meta.LoadName,
		UnityVersion:          meta.UnityVersion,
		EngineVersion:         meta.EngineVersion,
		TargetPlatform:        meta.TargetPlatform,
		AbaVersion:            meta.AbaVersion,
		GenerationVersion:     meta.GenerationVersion,
		SerializedFileVersion: meta.SerializedFileVersion,
		DataBase64:            base64.StdEncoding.EncodeToString(data),
		TypeTree:              typeTree,
	}, nil
}

// hasRawAssetMeta 判断原始资源元数据是否包含需要保存的值
// hasRawAssetMeta reports whether raw asset metadata contains values that need to be stored
func hasRawAssetMeta(meta rawAssetMeta) bool {
	return meta.PathID != 0 || meta.LoadName != "" || meta.UnityVersion != "" || meta.EngineVersion != "" ||
		meta.TargetPlatform != nil || meta.AbaVersion != 0 || meta.GenerationVersion != "" || meta.SerializedFileVersion != 0
}

// inferRawUnityObjectKind 根据路径后缀推断原始 Unity 对象种类和 ClassID
// inferRawUnityObjectKind infers a raw Unity object kind and ClassID from the path suffix
func inferRawUnityObjectKind(path string) (string, int32, bool) {
	lowerName := strings.ToLower(filepath.Base(path))
	if strings.HasSuffix(lowerName, ".json") {
		lowerName = strings.TrimSuffix(lowerName, ".json")
	}
	if !strings.HasSuffix(lowerName, ".bytes") {
		return "", 0, false
	}

	for _, candidate := range []struct {
		suffix string
		kind   string
	}{
		{".texture2d.bytes", "rawtexture2d"},
		{".texture.bytes", "rawtexture2d"},
		{".tex.bytes", "rawtexture2d"},
		{".sprite.bytes", "sprite"},
		{".mmesh.bytes", "mesh"},
		{".partsatlas.bytes", "spriteatlas"},
		{".partsassets.bytes", "spriteatlas"},
		{".anm.bytes", "animationclip"},
		{".monoscript.bytes", "monoscript"},
		{".monobehaviour.bytes", "monobehaviour"},
		{".material.bytes", "material"},
		{".shader.bytes", "shader"},
		{".audioclip.bytes", "audioclip"},
		{".font.bytes", "font"},
	} {
		if strings.HasSuffix(lowerName, candidate.suffix) {
			classID, ok := unityRawClassIDForKind(candidate.kind)
			return candidate.kind, classID, ok
		}
	}

	kind, ok := unityRawKindForPackPath(path)
	if !ok {
		return "", 0, false
	}
	classID, ok := unityRawClassIDForKind(kind)
	return kind, classID, ok
}

// inferRawUnityObjectName 从对象数据、加载名或文件名推断内部对象名称
// inferRawUnityObjectName infers the internal object name from object data, the load name, or the file name
func inferRawUnityObjectName(path string, data []byte, meta rawAssetMeta) string {
	if name, ok := readRawUnityLeadingName(data); ok {
		return name
	}
	if meta.LoadName != "" {
		return filepath.Base(filepath.ToSlash(meta.LoadName))
	}
	name := filepath.Base(path)
	lower := strings.ToLower(name)
	for _, suffix := range []string{
		".texture2d.bytes",
		".texture.bytes",
		".tex.bytes",
		".sprite.bytes",
		".mmesh.bytes",
		".partsatlas.bytes",
		".partsassets.bytes",
		".anm.bytes",
		".monoscript.bytes",
		".monobehaviour.bytes",
		".material.bytes",
		".shader.bytes",
		".audioclip.bytes",
		".font.bytes",
		".bytes",
	} {
		if strings.HasSuffix(lower, suffix) {
			return name[:len(name)-len(suffix)]
		}
	}
	return strings.TrimSuffix(name, filepath.Ext(name))
}

// readRawUnityLeadingName 读取 Unity 原始对象开头的受限长度名称
// readRawUnityLeadingName reads the bounded length-prefixed name at the start of a raw Unity object
func readRawUnityLeadingName(data []byte) (string, bool) {
	if len(data) < 4 {
		return "", false
	}
	nameLength := int64(binary.LittleEndian.Uint32(data[:4]))
	if nameLength <= 0 || nameLength > 4096 || 4+nameLength > int64(len(data)) {
		return "", false
	}
	name := string(data[4 : 4+nameLength])
	for _, r := range name {
		if r < 0x20 && r != '\t' && r != '\n' && r != '\r' {
			return "", false
		}
	}
	return name, true
}

// unityClassName 返回已知 Unity ClassID 对应的类型名称
// unityClassName returns the type name for a known Unity ClassID
func unityClassName(classID int32) string {
	switch classID {
	case aba.ClassIDGameObject:
		return "GameObject"
	case aba.ClassIDTransform:
		return "Transform"
	case aba.ClassIDMaterial:
		return "Material"
	case aba.ClassIDMeshRenderer:
		return "MeshRenderer"
	case aba.ClassIDTexture2D:
		return "Texture2D"
	case aba.ClassIDMeshFilter:
		return "MeshFilter"
	case aba.ClassIDMesh:
		return "Mesh"
	case aba.ClassIDShader:
		return "Shader"
	case aba.ClassIDTextAsset:
		return "TextAsset"
	case aba.ClassIDAnimationClip:
		return "AnimationClip"
	case aba.ClassIDAudioClip:
		return "AudioClip"
	case aba.ClassIDCubemap:
		return "Cubemap"
	case aba.ClassIDMonoBehaviour:
		return "MonoBehaviour"
	case aba.ClassIDMonoScript:
		return "MonoScript"
	case aba.ClassIDFont:
		return "Font"
	case aba.ClassIDAssetBundle:
		return "AssetBundle"
	case aba.ClassIDSprite:
		return "Sprite"
	case aba.ClassIDSpriteAtlas:
		return "SpriteAtlas"
	default:
		return fmt.Sprintf("Type_%d", classID)
	}
}
