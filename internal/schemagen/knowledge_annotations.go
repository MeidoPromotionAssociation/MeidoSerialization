package schemagen

import (
	"fmt"
	"strconv"
	"strings"

	knowledgev1 "github.com/MeidoPromotionAssociation/MeidoSerialization/schemas/knowledge/v1"
	"github.com/google/jsonschema-go/jsonschema"
)

func applyKnowledgeAnnotations(formatID string, root *jsonschema.Schema) error {
	guide, found, err := knowledgev1.Decode(formatID)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	for _, field := range guide.Fields {
		target, err := schemaPointer(root, field.SchemaPointer)
		if err != nil {
			return fmt.Errorf("field %s: %w", field.JSONPath, err)
		}
		target.Title = field.Title
		target.Description = field.Description
		if target.Extra == nil {
			target.Extra = make(map[string]any)
		}
		target.Extra["x-meido-game-usage"] = field.GameUsage
		target.Extra["x-meido-edit-role"] = field.EditRole
		target.Extra["x-meido-edit-guidance"] = field.EditGuidance
		target.Extra["x-meido-confidence"] = field.Confidence
		if field.Risk != "" {
			target.Extra["x-meido-risk"] = field.Risk
		}
		if len(field.Constraints) != 0 {
			target.Extra["x-meido-constraints"] = append([]string(nil), field.Constraints...)
		}
		if field.Recommended != nil {
			target.Extra["x-meido-recommended-range"] = map[string]float64{
				"minimum": field.Recommended.Minimum, "maximum": field.Recommended.Maximum,
			}
		}
		if len(field.EnumValues) != 0 {
			target.Extra["x-meido-enum-values"] = field.EnumValues
		}
		if len(field.Evidence) != 0 {
			target.Extra["x-meido-source-evidence"] = field.Evidence
		}
	}
	return nil
}

func schemaPointer(root *jsonschema.Schema, pointer string) (*jsonschema.Schema, error) {
	if pointer == "#" || pointer == "" {
		return root, nil
	}
	if !strings.HasPrefix(pointer, "#/") {
		return nil, fmt.Errorf("invalid schema pointer %q", pointer)
	}
	current := root
	parts := strings.Split(strings.TrimPrefix(pointer, "#/"), "/")
	var err error
	for index := 0; index < len(parts); {
		part := unescapeSchemaPointer(parts[index])
		switch part {
		case "$defs", "definitions":
			if index+1 >= len(parts) {
				return nil, fmt.Errorf("missing definition name in %q", pointer)
			}
			name := unescapeSchemaPointer(parts[index+1])
			if current.Defs == nil || current.Defs[name] == nil {
				return nil, fmt.Errorf("definition %q is not present", name)
			}
			current = current.Defs[name]
			index += 2
		case "properties":
			if index+1 >= len(parts) {
				return nil, fmt.Errorf("missing property name in %q", pointer)
			}
			name := unescapeSchemaPointer(parts[index+1])
			if current.Properties == nil || current.Properties[name] == nil {
				return nil, fmt.Errorf("property %q is not present", name)
			}
			current = current.Properties[name]
			index += 2
		case "items":
			if current.Items == nil {
				return nil, fmt.Errorf("items is not present")
			}
			current = current.Items
			index++
		case "anyOf":
			current, index, err = schemaBranch(current.AnyOf, parts, index, pointer)
			if err != nil {
				return nil, err
			}
		case "oneOf":
			current, index, err = schemaBranch(current.OneOf, parts, index, pointer)
			if err != nil {
				return nil, err
			}
		case "allOf":
			current, index, err = schemaBranch(current.AllOf, parts, index, pointer)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unsupported pointer component %q in %q", part, pointer)
		}
		if current == nil {
			return nil, fmt.Errorf("pointer %q resolved to nil", pointer)
		}
	}
	return current, nil
}

func schemaBranch(branches []*jsonschema.Schema, parts []string, index int, pointer string) (*jsonschema.Schema, int, error) {
	if index+1 >= len(parts) {
		return nil, index, fmt.Errorf("missing branch index in %q", pointer)
	}
	branchIndex, err := strconv.Atoi(unescapeSchemaPointer(parts[index+1]))
	if err != nil || branchIndex < 0 || branchIndex >= len(branches) {
		return nil, index, fmt.Errorf("invalid branch index in %q", pointer)
	}
	return branches[branchIndex], index + 2, nil
}

func unescapeSchemaPointer(value string) string {
	return strings.NewReplacer("~1", "/", "~0", "~").Replace(value)
}
