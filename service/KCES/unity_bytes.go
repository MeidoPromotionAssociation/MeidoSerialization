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

// RawUnityObjectFormat 是旧原始 Unity 对象 JSON 封套的格式标识
// RawUnityObjectFormat is the format marker for the legacy raw Unity object JSON envelope
const RawUnityObjectFormat = "kces-unity-raw-object"

// NativeUnityObjectJSONFormat 是可编辑自描述 Unity 对象 JSON 的格式标识
// NativeUnityObjectJSONFormat is the format marker for editable self-describing Unity object JSON
const NativeUnityObjectJSONFormat = "kces-unity-native-object"

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
	ReadOnly              bool                      `json:"readOnly,omitempty"`              // TypeTree 值是否仅供查看 / Whether the TypeTree value is read-only
	SchemaBase64          string                    `json:"schemaBase64,omitempty"`          // 不含正文的独立对象头与完整 TypeTree / Standalone object header and complete TypeTree without payload data
	ResourceDataBase64    string                    `json:"resourceDataBase64,omitempty"`    // AudioClip 内联音频载荷 / Inline AudioClip payload data
	DataBase64            string                    `json:"dataBase64,omitempty"`            // 旧格式原始序列化对象数据 base64 / Base64 of legacy raw serialized object data
	TypeTree              *RawUnityTypeTreeEnvelope `json:"typeTree,omitempty"`              // 可选 TypeTree 只读视图 / Optional read-only TypeTree view
}

// RawUnityObjectService 提供原始 Unity 对象字节与 JSON 封套之间的转换服务 / RawUnityObjectService converts raw Unity object bytes to and from JSON envelopes
type RawUnityObjectService struct{}

// IsKCESRawUnityBytesFile 判断路径是否为受支持的 KCES 原始 Unity 对象字节文件
// IsKCESRawUnityBytesFile reports whether a path is a supported KCES raw Unity object byte file
func IsKCESRawUnityBytesFile(path string) bool {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".json") {
		return false
	}
	if _, _, ok := inferRawUnityObjectKind(path); !ok {
		return false
	}
	if data, err := os.ReadFile(path); err == nil {
		if _, err := aba.ReadNativeUnityObject(data); err == nil {
			return true
		}
	}
	return strings.HasSuffix(lower, ".bytes")
}

// IsKCESRawUnityBytesJSONFile 判断路径是否为受支持的 KCES 原始 Unity 对象 JSON 文件
// IsKCESRawUnityBytesJSONFile reports whether a path is a supported KCES raw Unity object JSON file
func IsKCESRawUnityBytesJSONFile(path string) bool {
	if !strings.HasSuffix(strings.ToLower(path), ".json") {
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
	return header.Format == RawUnityObjectFormat || header.Format == NativeUnityObjectJSONFormat
}

// ConvertRawUnityObjectToJson 将 KCES 原始 Unity 对象字节转换为可编辑 JSON 封套
// ConvertRawUnityObjectToJson converts KCES raw Unity object bytes into an editable JSON envelope
func (s *RawUnityObjectService) ConvertRawUnityObjectToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
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
	budget, err := newRawUnityConversionBudget(ctx, maxOutputBytes)
	if err != nil {
		return err
	}
	return budget.WriteFile(outputPath, data)
}

// ConvertJsonToRawUnityObject 将可编辑 JSON 封套转换回 KCES 原始 Unity 对象字节
// ConvertJsonToRawUnityObject converts an editable JSON envelope back into KCES raw Unity object bytes
func (s *RawUnityObjectService) ConvertJsonToRawUnityObject(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read %q: %w", inputPath, err)
	}
	var formatHeader struct {
		Format string `json:"format"`
	}
	if err := json.Unmarshal(trimJSONUTF8BOM(data), &formatHeader); err != nil {
		return fmt.Errorf("parse KCES raw Unity object json header: %w", err)
	}
	if formatHeader.Format == NativeUnityObjectJSONFormat {
		_, nativeData, err := decodeNativeUnityObjectEditingJSON(data)
		if err != nil {
			return fmt.Errorf("parse KCES native Unity object json: %w", err)
		}
		budget, err := newRawUnityConversionBudget(ctx, maxOutputBytes)
		if err != nil {
			return err
		}
		if err := budget.WriteFile(outputPath, nativeData); err != nil {
			return fmt.Errorf("write %q: %w", outputPath, err)
		}
		return nil
	}
	envelope, raw, err := decodeRawUnityObjectEditingJSON(data)
	if err != nil {
		return fmt.Errorf("parse KCES raw Unity object json: %w", err)
	}
	budget, err := newRawUnityConversionBudget(ctx, maxOutputBytes)
	if err != nil {
		return err
	}
	if err := budget.WriteFile(outputPath, raw); err != nil {
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
		if err := budget.WriteFile(assetMetaPath(outputPath), metaData); err != nil {
			return fmt.Errorf("write %q: %w", assetMetaPath(outputPath), err)
		}
	}
	if envelope.TypeTree != nil {
		typeTreeData, err := marshalRawUnityTypeTreeEnvelope(envelope.TypeTree)
		if err != nil {
			return err
		}
		if err := budget.WriteFile(typeTreeSidecarPath(outputPath), typeTreeData); err != nil {
			return fmt.Errorf("write %q: %w", typeTreeSidecarPath(outputPath), err)
		}
	}
	return nil
}

// rawUnityConversionBudget 记录一个 .bytes 转换及其旁车共享的剩余输出字节数 / rawUnityConversionBudget tracks the remaining output bytes shared by a .bytes conversion and its sidecars
type rawUnityConversionBudget struct {
	Context   context.Context // 转换上下文 / Conversion context
	Remaining int64           // 剩余可写字节数 / Remaining writable bytes
}

// newRawUnityConversionBudget 创建 .bytes 转换及其旁车共享的输出预算
// newRawUnityConversionBudget creates an output budget shared by a .bytes conversion and its sidecars
func newRawUnityConversionBudget(ctx context.Context, maxOutputBytes int64) (*rawUnityConversionBudget, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	if maxOutputBytes <= 0 {
		return nil, fmt.Errorf("positive raw Unity conversion output limit is required")
	}
	return &rawUnityConversionBudget{Context: ctx, Remaining: maxOutputBytes}, nil
}

// WriteFile 在共享预算内直接写入一个 .bytes 转换输出或旁车
// WriteFile directly writes one .bytes conversion output or sidecar within the shared budget
func (b *rawUnityConversionBudget) WriteFile(path string, data []byte) error {
	if b == nil {
		return fmt.Errorf("raw Unity conversion output budget is nil")
	}
	if b.Context != nil {
		if err := b.Context.Err(); err != nil {
			return err
		}
	}
	dataSize := int64(len(data))
	if dataSize > b.Remaining {
		return fmt.Errorf("%w: raw Unity conversion output needs %d bytes but only %d remain", ErrConversionOutputLimitExceeded, dataSize, b.Remaining)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write raw Unity conversion output %q: %w", path, err)
	}
	b.Remaining -= dataSize
	if b.Context != nil {
		return b.Context.Err()
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

// decodeNativeUnityObjectEditingJSON 解码自描述 Unity 对象 JSON，并按内嵌 TypeTree 重编码修改后的值
// decodeNativeUnityObjectEditingJSON decodes self-describing Unity object JSON and re-encodes its modified value using the embedded TypeTree
func decodeNativeUnityObjectEditingJSON(data []byte) (*RawUnityObjectEnvelope, []byte, error) {
	var envelope RawUnityObjectEnvelope
	if err := decodeStrictJSON(data, &envelope, "KCES native Unity object JSON"); err != nil {
		return nil, nil, err
	}
	if envelope.Format != NativeUnityObjectJSONFormat {
		return nil, nil, fmt.Errorf("unsupported native Unity object JSON format %q", envelope.Format)
	}
	if envelope.ReadOnly || !isEditableNativeUnityClassID(envelope.ClassID) {
		return nil, nil, fmt.Errorf("ClassID %d TypeTree JSON is read-only", envelope.ClassID)
	}
	if envelope.SchemaBase64 == "" {
		return nil, nil, fmt.Errorf("native Unity object JSON has no schemaBase64")
	}
	schemaData, err := base64.StdEncoding.DecodeString(envelope.SchemaBase64)
	if err != nil {
		return nil, nil, fmt.Errorf("decode schemaBase64: %w", err)
	}
	object, err := aba.ReadNativeUnityObject(schemaData)
	if err != nil {
		return nil, nil, fmt.Errorf("decode embedded native Unity object schema: %w", err)
	}
	if object.ClassID != envelope.ClassID {
		return nil, nil, fmt.Errorf("JSON ClassID %d does not match embedded schema ClassID %d", envelope.ClassID, object.ClassID)
	}
	if envelope.TypeTree == nil || envelope.TypeTree.Value == nil {
		return nil, nil, fmt.Errorf("native Unity object JSON has no TypeTree value")
	}
	if envelope.TypeTree.Format != RawUnityTypeTreeFormat || envelope.TypeTree.ClassID != envelope.ClassID {
		return nil, nil, fmt.Errorf("native Unity object TypeTree header does not match ClassID %d", envelope.ClassID)
	}
	root, err := editableTypeTreeValueFromJSON(envelope.TypeTree.Value)
	if err != nil {
		return nil, nil, fmt.Errorf("decode editable TypeTree value: %w", err)
	}
	var trailing []byte
	if envelope.ClassID == aba.ClassIDAudioClip {
		trailing, err = base64.StdEncoding.DecodeString(envelope.ResourceDataBase64)
		if err != nil {
			return nil, nil, fmt.Errorf("decode resourceDataBase64: %w", err)
		}
		if err := setEditableAudioResourceData(root, uint64(len(trailing))); err != nil {
			return nil, nil, err
		}
	} else if envelope.ResourceDataBase64 != "" {
		return nil, nil, fmt.Errorf("ClassID %d cannot contain resourceDataBase64", envelope.ClassID)
	}
	object.Data, err = object.EncodeValueWithTrailingData(root, trailing)
	if err != nil {
		return nil, nil, fmt.Errorf("encode edited ClassID %d object: %w", envelope.ClassID, err)
	}
	nativeData, err := aba.EncodeNativeUnityObject(object)
	if err != nil {
		return nil, nil, err
	}
	return &envelope, nativeData, nil
}

// isEditableNativeUnityClassID 判断指定 ClassID 是否属于已承诺可修改并重编码的原生对象类型
// isEditableNativeUnityClassID reports whether a ClassID belongs to the native object types promised for modification and re-encoding
func isEditableNativeUnityClassID(classID int32) bool {
	switch classID {
	case aba.ClassIDMesh,
		aba.ClassIDTexture2D,
		aba.ClassIDSprite,
		aba.ClassIDSpriteAtlas,
		aba.ClassIDAnimationClip,
		aba.ClassIDMaterial,
		aba.ClassIDAudioClip,
		aba.ClassIDMonoBehaviour:
		return true
	default:
		return false
	}
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
	if object, nativeErr := aba.ReadNativeUnityObject(data); nativeErr == nil {
		if object.ClassID != classID {
			return nil, fmt.Errorf("native Unity object %q contains ClassID %d but its path requires ClassID %d", path, object.ClassID, classID)
		}
		root, _, err := object.DecodeValueAndTrailingData()
		if err != nil {
			return nil, fmt.Errorf("decode native Unity object %q: %w", path, err)
		}
		name := inferRawUnityObjectName(path, object.Data, rawAssetMeta{})
		if fieldName, ok := root.Field("m_Name").String(); ok && fieldName != "" {
			name = fieldName
		}
		readOnly := !isEditableNativeUnityClassID(classID)
		typeTreeValue := typeTreeJSONValue(root)
		var schemaBase64 string
		var resourceDataBase64 string
		if !readOnly {
			typeTreeValue = editableTypeTreeJSONValue(root)
			schemaObject := *object
			schemaObject.Data = nil
			schema, err := aba.EncodeNativeUnityObject(&schemaObject)
			if err != nil {
				return nil, fmt.Errorf("encode native Unity object schema: %w", err)
			}
			schemaBase64 = base64.StdEncoding.EncodeToString(schema)
			if classID == aba.ClassIDAudioClip {
				resourceData, err := object.AudioData()
				if err != nil {
					return nil, fmt.Errorf("read native AudioClip payload: %w", err)
				}
				resourceDataBase64 = base64.StdEncoding.EncodeToString(resourceData)
			}
		}
		return &RawUnityObjectEnvelope{
			Format:             NativeUnityObjectJSONFormat,
			ClassID:            classID,
			TypeName:           unityClassName(classID),
			Kind:               kind,
			Name:               name,
			ReadOnly:           readOnly,
			SchemaBase64:       schemaBase64,
			ResourceDataBase64: resourceDataBase64,
			TypeTree: &RawUnityTypeTreeEnvelope{
				Format:   RawUnityTypeTreeFormat,
				ClassID:  classID,
				TypeName: unityClassName(classID),
				Name:     name,
				Value:    typeTreeValue,
			},
		}, nil
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

// setEditableAudioResourceData 将 AudioClip 资源描述改为确定长度的内联载荷
// setEditableAudioResourceData changes an AudioClip resource descriptor to an inline payload with a known length
func setEditableAudioResourceData(root *aba.TypeTreeValue, size uint64) error {
	if root == nil {
		return fmt.Errorf("nil AudioClip TypeTree value")
	}
	resource := root.Field("m_Resource")
	if resource == nil {
		return fmt.Errorf("AudioClip TypeTree value has no m_Resource field")
	}
	if source := firstEditableTypeTreeField(resource, "m_Source", "source", "path"); source != nil {
		source.Value = ""
	}
	if offset := firstEditableTypeTreeField(resource, "m_Offset", "offset"); offset != nil {
		offset.Value = uint64(0)
	}
	sizeField := firstEditableTypeTreeField(resource, "m_Size", "size")
	if sizeField == nil {
		return fmt.Errorf("AudioClip m_Resource has no size field")
	}
	sizeField.Value = size
	return nil
}

// firstEditableTypeTreeField 返回首个存在的直接字段
// firstEditableTypeTreeField returns the first existing direct field
func firstEditableTypeTreeField(parent *aba.TypeTreeValue, names ...string) *aba.TypeTreeValue {
	for _, name := range names {
		if field := parent.Field(name); field != nil {
			return field
		}
	}
	return nil
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
	if kind, ok := unityRawKindForPackPath(path); ok && isNativeUnityObjectPackPath(path, kind) {
		classID, classOK := unityRawClassIDForKind(kind)
		return kind, classID, classOK
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
	if kind, ok := unityRawKindForPackPath(path); ok && isNativeUnityObjectPackPath(path, kind) {
		return inferAssetNameForPack(path)
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

	if err := os.WriteFile(path, raw, 0644); err != nil {
		return fmt.Errorf("write raw Unity object %q: %w", path, err)
	}
	if metaData != nil {
		if err := os.WriteFile(assetMetaPath(path), metaData, 0644); err != nil {
			return fmt.Errorf("write raw Unity object metadata %q: %w", assetMetaPath(path), err)
		}
	}
	if typeTreeData != nil {
		if err := os.WriteFile(typeTreeSidecarPath(path), typeTreeData, 0644); err != nil {
			return fmt.Errorf("write raw Unity object TypeTree %q: %w", typeTreeSidecarPath(path), err)
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
