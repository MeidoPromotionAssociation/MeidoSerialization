package knowledgev1

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Resolve 将编辑模式字段清单与可选源码审核 profile 合并为有效指南文档
// Resolve merges an editing-schema field inventory with an optional source-reviewed profile into an effective guide document
func Resolve(formatID, schemaID string, schemaJSON []byte) (Document, error) {
	id := strings.ToLower(strings.TrimSpace(formatID))
	if id == "" || !json.Valid(schemaJSON) {
		return Document{}, fmt.Errorf("format ID and valid schema JSON are required")
	}
	var schema map[string]any
	if err := json.Unmarshal(schemaJSON, &schema); err != nil {
		return Document{}, fmt.Errorf("decode schema %s: %w", id, err)
	}
	if schemaFormatID, _ := schema["x-meido-format-id"].(string); schemaFormatID != id {
		return Document{}, fmt.Errorf("schema format ID %q does not match %q", schemaFormatID, id)
	}
	if embeddedID, _ := schema["$id"].(string); embeddedID != schemaID {
		return Document{}, fmt.Errorf("schema ID %q does not match %q", embeddedID, schemaID)
	}

	guide := genericGuide(id, schemaID, schema)
	profile, found, err := Decode(id)
	if err != nil {
		return Document{}, err
	}
	if found {
		if profile.SchemaID != schemaID || profile.SchemaURI != "meido://schemas/"+id {
			return Document{}, fmt.Errorf("guide %s references an inconsistent schema", id)
		}
		if err := validateProfileFields(profile, schema); err != nil {
			return Document{}, err
		}
		guide = mergeGuide(guide, profile)
	}
	data, err := json.MarshalIndent(guide, "", "  ")
	if err != nil {
		return Document{}, fmt.Errorf("marshal effective guide %s: %w", id, err)
	}
	data = append(data, '\n')
	digest := sha256.Sum256(data)
	return Document{
		FormatID: id, Version: Version, ID: IDPrefix + id, MediaType: MediaType,
		SHA256: fmt.Sprintf("%x", digest[:]), SchemaID: schemaID,
		Coverage: guide.Coverage.Level, JSON: data,
	}, nil
}

// genericGuide 从编辑模式生成只有结构覆盖的基础格式指南
// genericGuide generates a base format guide with schema-only structural coverage from an editing schema
func genericGuide(formatID, schemaID string, schema map[string]any) Guide {
	fields := collectSchemaFields(schema)
	return Guide{
		ID:        IDPrefix + formatID,
		FormatID:  formatID,
		Version:   Version,
		SchemaURI: "meido://schemas/" + formatID,
		SchemaID:  schemaID,
		Title:     formatID + " editing guide",
		Summary:   "This guide enumerates the published editing JSON fields. No format-specific source-review profile is available.",
		Coverage: Coverage{
			Level: CoverageSchemaOnly,
			Notes: "Field shape is derived from the embedded JSON Schema. Runtime behavior must not be inferred from field names alone.",
		},
		Fields: fields,
		Rules: []Rule{
			{
				ID: "respect-schema-fields", Severity: "error",
				Summary: "Retain modeled values without inventing unsupported wire state.",
				Details: "Only the published serialization shape is confirmed. Keep unrequested typed values unchanged and retain base64 only where the schema represents a real byte-array field. Never add raw, reserved, future-slot, trailing-data, or parse-fallback fields that are absent from the schema.",
			},
		},
		Workflow: []string{
			"Read the format JSON Schema and this guide before editing.",
			"Inspect a real file to obtain values; do not invent required identifiers or resources.",
			"Change only fields required by the stated objective, retain other typed values and semantic binary assets, and never invent fields absent from the schema.",
			"Call meido.validate_editing_json after editing, then convert only after validation succeeds.",
		},
		Warnings: []string{
			"Schema-only coverage confirms JSON shape, not how the game uses a value.",
			"Validation cannot prove that referenced game assets, bones, materials, hashes, IDs, or enum values exist in the target game build.",
		},
	}
}

// mergeGuide 使用源码审核字段覆盖基础字段并保持完整有序清单
// mergeGuide overlays source-reviewed fields on base fields while retaining a complete ordered inventory
func mergeGuide(base, profile Guide) Guide {
	fields := make(map[string]Field, len(base.Fields)+len(profile.Fields))
	for _, field := range base.Fields {
		fields[field.JSONPath] = field
	}
	for _, field := range profile.Fields {
		fields[field.JSONPath] = field
	}
	profile.Fields = make([]Field, 0, len(fields))
	for _, field := range fields {
		profile.Fields = append(profile.Fields, field)
	}
	sort.Slice(profile.Fields, func(i, j int) bool { return profile.Fields[i].JSONPath < profile.Fields[j].JSONPath })
	return profile
}

// validateProfileFields 校验 profile 字段元数据及其路径和模式指针可达性
// validateProfileFields validates profile field metadata and reachability of paths and schema pointers
func validateProfileFields(guide Guide, schema map[string]any) error {
	generated := collectSchemaFields(schema)
	paths := make(map[string]bool, len(generated))
	for _, field := range generated {
		paths[field.JSONPath] = true
	}
	for _, field := range guide.Fields {
		if field.JSONPath == "" || field.Title == "" || field.Description == "" || field.GameUsage == "" || field.EditRole == "" || field.EditGuidance == "" || field.Confidence == "" {
			return fmt.Errorf("guide %s has incomplete semantic metadata for %q", guide.FormatID, field.JSONPath)
		}
		if !paths[field.JSONPath] {
			return fmt.Errorf("guide %s field %q is not reachable from its schema", guide.FormatID, field.JSONPath)
		}
		if field.SchemaPointer == "" || schemaAtPointer(schema, field.SchemaPointer) == nil {
			return fmt.Errorf("guide %s field %q has invalid schema pointer %q", guide.FormatID, field.JSONPath, field.SchemaPointer)
		}
	}
	return nil
}

// collectSchemaFields 遍历编辑模式并返回按 JSON 路径排序的字段清单
// collectSchemaFields walks an editing schema and returns a field inventory sorted by JSON path
func collectSchemaFields(schema map[string]any) []Field {
	fields := make(map[string]Field)
	walkSchema(schema, schema, "", "#", make(map[string]bool), fields)
	result := make([]Field, 0, len(fields))
	for _, field := range fields {
		result = append(result, field)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].JSONPath < result[j].JSONPath })
	return result
}

// walkSchema 递归跟随本地引用、组合分支、属性、数组和动态属性收集字段
// walkSchema recursively follows local references, composition branches, properties, arrays, and dynamic properties to collect fields
func walkSchema(root, node map[string]any, jsonPath, schemaPointer string, refStack map[string]bool, fields map[string]Field) {
	if ref, _ := node["$ref"].(string); strings.HasPrefix(ref, "#/") {
		if refStack[ref] {
			return
		}
		resolved, _ := schemaAtPointer(root, ref).(map[string]any)
		if resolved != nil {
			refStack[ref] = true
			walkSchema(root, resolved, jsonPath, ref, refStack, fields)
			delete(refStack, ref)
		}
	}
	for _, keyword := range []string{"allOf", "anyOf", "oneOf"} {
		branches, _ := node[keyword].([]any)
		for index, branch := range branches {
			object, _ := branch.(map[string]any)
			if object != nil {
				walkSchema(root, object, jsonPath, schemaPointer+"/"+keyword+"/"+strconv.Itoa(index), refStack, fields)
			}
		}
	}
	properties, _ := node["properties"].(map[string]any)
	propertyNames := make([]string, 0, len(properties))
	for name := range properties {
		propertyNames = append(propertyNames, name)
	}
	sort.Strings(propertyNames)
	for _, name := range propertyNames {
		property, _ := properties[name].(map[string]any)
		metadata := property
		if metadata == nil {
			metadata = map[string]any{}
		}
		path := jsonPath + "/" + escapeJSONPointer(name)
		pointer := schemaPointer + "/properties/" + escapeJSONPointer(name)
		if _, exists := fields[path]; !exists {
			fields[path] = genericField(path, pointer, name, metadata)
		}
		if property != nil {
			walkSchema(root, property, path, pointer, refStack, fields)
		}
	}
	if items, _ := node["items"].(map[string]any); items != nil {
		walkSchema(root, items, jsonPath+"/*", schemaPointer+"/items", refStack, fields)
	}
	if additional, _ := node["additionalProperties"].(map[string]any); additional != nil {
		walkSchema(root, additional, jsonPath+"/*", schemaPointer+"/additionalProperties", refStack, fields)
	}
}

// genericField 从模式元数据构建保守的结构级字段说明
// genericField builds conservative schema-level field guidance from schema metadata
func genericField(path, pointer, name string, schema map[string]any) Field {
	title, _ := schema["title"].(string)
	if title == "" {
		title = name
	}
	description, _ := schema["description"].(string)
	if description == "" {
		description = "Published editing JSON field " + path + "."
	}
	role := "schema_field"
	risk := "medium"
	guidance := "Retain the typed value unless the editing objective and a reviewed guide explicitly require changing it."
	if encoding, _ := schema["contentEncoding"].(string); encoding == "base64" {
		role = "binary_payload"
		risk = "critical"
		guidance = "Treat this as native byte-array data only; retain the exact bytes unless a reviewed asset-specific transformation applies, and never use it as a typed-decoder fallback."
	}
	return Field{
		JSONPath: path, SchemaPointer: pointer, Title: title, Description: description,
		GameUsage: "Only serialization shape is confirmed; game-runtime use has no source-reviewed profile.",
		EditRole:  role, EditGuidance: guidance,
		Risk: risk, Confidence: ConfidenceSchemaOnly,
	}
}

// schemaAtPointer 解析 JSON Schema 文档中的本地 JSON Pointer
// schemaAtPointer resolves a local JSON Pointer within a JSON Schema document
func schemaAtPointer(root map[string]any, pointer string) any {
	if pointer == "#" || pointer == "" {
		return root
	}
	if !strings.HasPrefix(pointer, "#/") {
		return nil
	}
	var current any = root
	for _, raw := range strings.Split(strings.TrimPrefix(pointer, "#/"), "/") {
		segment := unescapeJSONPointer(raw)
		switch value := current.(type) {
		case map[string]any:
			current = value[segment]
		case []any:
			index, err := strconv.Atoi(segment)
			if err != nil || index < 0 || index >= len(value) {
				return nil
			}
			current = value[index]
		default:
			return nil
		}
		if current == nil {
			return nil
		}
	}
	return current
}

// escapeJSONPointer 转义 JSON Pointer 路径段中的保留字符
// escapeJSONPointer escapes reserved characters in a JSON Pointer path segment
func escapeJSONPointer(value string) string {
	return strings.NewReplacer("~", "~0", "/", "~1").Replace(value)
}

// unescapeJSONPointer 还原 JSON Pointer 路径段中的保留字符
// unescapeJSONPointer restores reserved characters in a JSON Pointer path segment
func unescapeJSONPointer(value string) string {
	return strings.NewReplacer("~1", "/", "~0", "~").Replace(value)
}
