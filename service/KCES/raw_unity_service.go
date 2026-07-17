package KCES

import (
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

// RawUnityObjectEnvelope 是从 .aba 提取的原始 Unity 序列化对象字节的 JSON 可编辑封套 / RawUnityObjectEnvelope is the JSON-editable wrapper for raw Unity serialized object bytes extracted from .aba bundles
// 对象数据本身无损保存，sidecar 元数据保留 PathID、加载名以及解释原始对象布局所需的 Unity 版本上下文 / Object data is preserved losslessly while sidecar metadata keeps PathID, load name, and the Unity version context needed to interpret the original object layout
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
	BundleVersion         uint32                    `json:"bundleVersion,omitempty"`         // UnityFS 格式版本 / UnityFS format version
	GenerationVersion     string                    `json:"generationVersion,omitempty"`     // UnityFS generation version / UnityFS generation version
	SerializedFileVersion uint32                    `json:"serializedFileVersion,omitempty"` // SerializedFile 格式版本 / SerializedFile format version
	DataBase64            string                    `json:"dataBase64"`                      // 原始序列化对象数据 base64 / Base64 of raw serialized object data
	TypeTree              *RawUnityTypeTreeEnvelope `json:"typeTree,omitempty"`              // 可选 TypeTree 只读视图 / Optional read-only TypeTree view
}

// RawUnityObjectService 提供原始 Unity 对象字节与 JSON 封套的转换服务 / RawUnityObjectService converts raw Unity object bytes to and from JSON envelopes
type RawUnityObjectService struct{}

func IsKCESRawUnityBytesFile(path string) bool {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".json") || !strings.HasSuffix(lower, ".bytes") {
		return false
	}
	_, _, ok := inferRawUnityObjectKind(path)
	return ok
}

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

func (s *RawUnityObjectService) ConvertRawUnityObjectToJson(inputPath string, outputPath string) error {
	envelope, err := s.ReadRawUnityObjectFile(inputPath)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal KCES raw Unity object json: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("write %q: %w", outputPath, err)
	}
	return nil
}

func (s *RawUnityObjectService) ConvertJsonToRawUnityObject(inputPath string, outputPath string) error {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read %q: %w", inputPath, err)
	}
	envelope, raw, err := decodeRawUnityObjectEditingJSON(data)
	if err != nil {
		return fmt.Errorf("parse KCES raw Unity object json: %w", err)
	}
	if err := os.WriteFile(outputPath, raw, 0644); err != nil {
		return fmt.Errorf("write %q: %w", outputPath, err)
	}
	meta := rawAssetMeta{
		PathID:                envelope.PathID,
		LoadName:              envelope.LoadName,
		UnityVersion:          envelope.UnityVersion,
		EngineVersion:         envelope.EngineVersion,
		TargetPlatform:        envelope.TargetPlatform,
		BundleVersion:         envelope.BundleVersion,
		GenerationVersion:     envelope.GenerationVersion,
		SerializedFileVersion: envelope.SerializedFileVersion,
	}
	if hasRawAssetMeta(meta) {
		if err := writeRawAssetMeta(outputPath, meta); err != nil {
			return fmt.Errorf("write %q: %w", assetMetaPath(outputPath), err)
		}
	}
	if envelope.TypeTree != nil {
		if err := writeRawUnityTypeTreeEnvelope(outputPath, envelope.TypeTree); err != nil {
			return fmt.Errorf("write %q: %w", typeTreeSidecarPath(outputPath), err)
		}
	}
	return nil
}

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
		BundleVersion:         meta.BundleVersion,
		GenerationVersion:     meta.GenerationVersion,
		SerializedFileVersion: meta.SerializedFileVersion,
		DataBase64:            base64.StdEncoding.EncodeToString(data),
		TypeTree:              typeTree,
	}, nil
}

func hasRawAssetMeta(meta rawAssetMeta) bool {
	return meta.PathID != 0 || meta.LoadName != "" || meta.UnityVersion != "" || meta.EngineVersion != "" ||
		meta.TargetPlatform != nil || meta.BundleVersion != 0 || meta.GenerationVersion != "" || meta.SerializedFileVersion != 0
}

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

func readRawUnityLeadingName(data []byte) (string, bool) {
	if len(data) < 4 {
		return "", false
	}
	n := int(binary.LittleEndian.Uint32(data[:4]))
	if n <= 0 || n > 4096 || 4+n > len(data) {
		return "", false
	}
	name := string(data[4 : 4+n])
	for _, r := range name {
		if r < 0x20 && r != '\t' && r != '\n' && r != '\r' {
			return "", false
		}
	}
	return name, true
}

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
