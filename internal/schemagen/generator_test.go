package schemagen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"testing"
	"unicode"

	editingv1 "github.com/MeidoPromotionAssociation/MeidoSerialization/schemas/editing/v1"
	serializationCOM3D2 "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/COM3D2"
	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
	serializationKCESCT "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	KCESService "github.com/MeidoPromotionAssociation/MeidoSerialization/service/KCES"
	"github.com/google/jsonschema-go/jsonschema"
	strictschema "github.com/santhosh-tekuri/jsonschema/v6"
)

func TestGenerateAll(t *testing.T) {
	documents, err := GenerateAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(documents) < 30 {
		t.Fatalf("generated %d schemas", len(documents))
	}
	for id, document := range documents {
		var value jsonschema.Schema
		if err := json.Unmarshal(document.JSON, &value); err != nil {
			t.Fatalf("%s: invalid schema JSON: %v", id, err)
		}
		if value.ID != document.ID || value.Schema != SchemaDialect {
			t.Fatalf("%s: metadata = id=%q schema=%q", id, value.ID, value.Schema)
		}
		if _, err := value.Resolve(nil); err != nil {
			t.Fatalf("%s: schema does not resolve: %v", id, err)
		}
		strictDocument, err := strictschema.UnmarshalJSON(bytes.NewReader(document.JSON))
		if err != nil {
			t.Fatalf("%s: runtime schema decode: %v", id, err)
		}
		compiler := strictschema.NewCompiler()
		compiler.DefaultDraft(strictschema.Draft2020)
		compiler.AssertContent()
		if err := compiler.AddResource(document.ID, strictDocument); err != nil {
			t.Fatalf("%s: runtime schema load: %v", id, err)
		}
		if _, err := compiler.Compile(document.ID); err != nil {
			t.Fatalf("%s: runtime schema compile: %v", id, err)
		}
		var raw any
		if err := json.Unmarshal(document.JSON, &raw); err != nil {
			t.Fatalf("%s: decode schema tree: %v", id, err)
		}
		assertUniqueRequired(t, id, "#", raw)
		if document.SHA256 == "" || document.Version == "" {
			t.Fatalf("%s: missing digest/version", id)
		}
	}
}

func TestGeneratedCOM3D2UnionArraysRejectNullElements(t *testing.T) {
	tests := []struct {
		formatID string
		value    any
		array    func(map[string]any) []any
	}{
		{
			formatID: "com3d2.mate",
			value: &serializationCOM3D2.Mate{Material: &serializationCOM3D2.Material{
				Properties: []serializationCOM3D2.Property{&serializationCOM3D2.FProperty{TypeName: "f"}},
			}},
			array: func(root map[string]any) []any {
				return root["Material"].(map[string]any)["Properties"].([]any)
			},
		},
		{
			formatID: "com3d2.col",
			value: &serializationCOM3D2.Col{
				Colliders: []serializationCOM3D2.ICollider{&serializationCOM3D2.MissingCollider{TypeName: "missing"}},
			},
			array: func(root map[string]any) []any { return root["Colliders"].([]any) },
		},
		{
			formatID: "com3d2.timeline",
			value: &serializationCOM3D2.TimelineData{
				Tracks: []serializationCOM3D2.TimelineTrack{&serializationCOM3D2.TranslationTrack{}},
			},
			array: func(root map[string]any) []any { return root["Tracks"].([]any) },
		},
	}

	for _, test := range tests {
		t.Run(test.formatID, func(t *testing.T) {
			schema := compileGeneratedSchema(t, test.formatID)
			instance := jsonObject(t, test.value)
			assertSchemaValidation(t, schema, instance, true)
			test.array(instance)[0] = nil
			assertSchemaValidation(t, schema, instance, false)
		})
	}
}

func TestGeneratedFloat32SchemasEnforceFiniteRange(t *testing.T) {
	schema := compileGeneratedSchema(t, "com3d2.mate")
	newInstance := func(value float64) map[string]any {
		root := jsonObject(t, &serializationCOM3D2.Mate{Material: &serializationCOM3D2.Material{
			Properties: []serializationCOM3D2.Property{&serializationCOM3D2.FProperty{TypeName: "f"}},
		}})
		properties := root["Material"].(map[string]any)["Properties"].([]any)
		properties[0].(map[string]any)["Number"] = value
		return root
	}

	maximum := float64(math.MaxFloat32)
	for _, test := range []struct {
		name  string
		value float64
		valid bool
	}{
		{name: "maximum", value: maximum, valid: true},
		{name: "minimum", value: -maximum, valid: true},
		{name: "above maximum", value: math.Nextafter(maximum, math.Inf(1)), valid: false},
		{name: "below minimum", value: math.Nextafter(-maximum, math.Inf(-1)), valid: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			assertSchemaValidation(t, schema, newInstance(test.value), test.valid)
		})
	}
}

func TestGeneratedKCESPayloadSchemasRejectRemovedRawFields(t *testing.T) {
	schema := compileGeneratedSchema(t, "kces.dbconf")
	descriptor, ok := serializationKCES.DescribeKCESPayload(".dbconf")
	if !ok {
		t.Fatal("missing .dbconf descriptor")
	}
	for _, test := range []struct {
		name  string
		value any
	}{
		{name: "base64-shaped value", value: "AQ=="},
		{name: "invalid base64 value", value: "AA*A"},
		{name: "null value", value: nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			instance := jsonObject(t, nativePayloadRoot(descriptor))
			instance["msgpackTrailingData"] = test.value
			assertSchemaValidation(t, schema, instance, false)
		})
	}
}

func TestGeneratedLimbColliderSchemaRequiresMaidPropCollider(t *testing.T) {
	descriptor, ok := serializationKCES.DescribeKCESPayload(".limbcol")
	if !ok {
		t.Fatal("missing .limbcol descriptor")
	}
	schema := compileGeneratedSchema(t, "kces.limbcol")
	tests := []struct {
		name     string
		collider serializationKCES.ColliderStatusUnion
		valid    bool
	}{
		{name: "maid prop", collider: serializationKCES.NewColliderMaidProp(), valid: true},
		{name: "plane", collider: serializationKCES.NewColliderPlane(), valid: false},
		{name: "capsule", collider: serializationKCES.NewColliderCapsule(), valid: false},
		{name: "sphere", collider: serializationKCES.NewColliderSphere(), valid: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			instance := jsonObject(t, nativePayloadRoot(descriptor))
			items := instance["items"].([]any)
			items[0].(map[string]any)["collider"] = jsonObject(t, test.collider)
			assertSchemaValidation(t, schema, instance, test.valid)
		})
	}
}

func TestGeneratedKCESPayloadSchemasBindDeclaredRootsAndRejectRemovedEnvelope(t *testing.T) {
	for _, extension := range payloadDescriptorExtensions() {
		descriptor, ok := serializationKCES.DescribeKCESPayload(extension)
		if !ok {
			t.Fatalf("missing payload descriptor for %s", extension)
		}
		formatID := "kces." + strings.TrimPrefix(extension, ".")
		t.Run(formatID, func(t *testing.T) {
			schema := compileGeneratedSchema(t, formatID)
			native := nativePayloadRoot(descriptor)
			assertSchemaValidation(t, schema, jsonObject(t, native), true)
			// 根为 JSON null 表示 MessagePack 根值为 nil
			// A JSON null root represents a nil MessagePack root value
			assertSchemaValidation(t, schema, nil, true)

			// 编辑封套连同 ExportCM 变体一起移除，封套的判别字段和分支根现在都只是未知属性
			// The editing envelope was removed together with the ExportCM variants, so its discriminator
			// fields and branch roots are now merely unknown properties
			for _, property := range []string{
				"format", "extension", "storageVariant", "kind",
				"msgpackRootNil", "dynamicBoneStatus", "colliderPackage",
				"limbColliderPackage", "ikColliderPackage", "clothParams", "json",
			} {
				property := property
				t.Run("reject removed envelope property "+property, func(t *testing.T) {
					instance := jsonObject(t, native)
					instance[property] = nil
					assertSchemaValidation(t, schema, instance, false)
				})
			}

			t.Run("reject removed ExportCM sidecar tuple", func(t *testing.T) {
				instance := jsonObject(t, native)
				instance["format"] = "kces-exportcm-sidecar"
				instance["storageVariant"] = "exportcm-unity-json"
				assertSchemaValidation(t, schema, instance, false)
			})
		})
	}
}

func assertUniqueRequired(t *testing.T, formatID, path string, value any) {
	t.Helper()
	switch current := value.(type) {
	case map[string]any:
		if required, ok := current["required"].([]any); ok {
			seen := make(map[string]struct{}, len(required))
			for index, item := range required {
				name, ok := item.(string)
				if !ok {
					t.Fatalf("%s: %s/required/%d is not a string", formatID, path, index)
				}
				if _, duplicate := seen[name]; duplicate {
					t.Fatalf("%s: %s/required contains duplicate %q", formatID, path, name)
				}
				seen[name] = struct{}{}
			}
		}
		for key, child := range current {
			assertUniqueRequired(t, formatID, path+"/"+key, child)
		}
	case []any:
		for index, child := range current {
			assertUniqueRequired(t, formatID, path+"/"+fmt.Sprint(index), child)
		}
	}
}

func TestKnowledgeAnnotationsArePublishedForEveryEditingSchema(t *testing.T) {
	for _, spec := range formatSpecs() {
		document, err := Generate(spec.id)
		if err != nil {
			t.Fatal(err)
		}
		payload := string(document.JSON)
		if !strings.Contains(payload, `"x-meido-game-usage"`) || !strings.Contains(payload, `"x-meido-edit-guidance"`) || !strings.Contains(payload, `"x-meido-source-evidence"`) {
			t.Fatalf("reviewed schema %s is missing Guide annotations", spec.id)
		}
		for _, value := range payload {
			if unicode.Is(unicode.Han, value) {
				t.Fatalf("reviewed schema %s contains a non-English Han annotation", spec.id)
			}
		}
	}
}

func TestGeneratedSchemasAcceptRootEditingJSON(t *testing.T) {
	for _, spec := range formatSpecs() {
		document, err := Generate(spec.id)
		if err != nil {
			t.Fatalf("%s: %v", spec.id, err)
		}
		var schema jsonschema.Schema
		if err := json.Unmarshal(document.JSON, &schema); err != nil {
			t.Fatalf("%s: decode schema: %v", spec.id, err)
		}
		resolved, err := schema.Resolve(nil)
		if err != nil {
			t.Fatalf("%s: resolve schema: %v", spec.id, err)
		}

		rootType := spec.root
		for rootType.Kind() == reflect.Pointer {
			rootType = rootType.Elem()
		}
		root := reflect.New(rootType)
		if root.Elem().Type() == typeOf[KCESService.CtEnvelope]() {
			envelope := root.Interface().(*KCESService.CtEnvelope)
			envelope.Catalog.Kind = serializationKCESCT.CatalogKindAssetBundle
		} else if root.Elem().Type() == typeOf[serializationKCES.KCESExportNameMap]() {
			nameMap := root.Interface().(*serializationKCES.KCESExportNameMap)
			nameMap.Entries = []serializationKCES.KCESExportNameMapEntry{}
		} else if extension := root.Elem().FieldByName("Extension"); extension.IsValid() && extension.CanSet() && extension.Kind() == reflect.String && strings.HasPrefix(spec.id, "kces.") {
			extension.SetString("." + strings.TrimPrefix(spec.id, "kces."))
		}
		editingJSON, err := json.Marshal(root.Interface())
		if err != nil {
			t.Fatalf("%s: marshal root editing JSON: %v", spec.id, err)
		}
		var instance any
		if err := json.Unmarshal(editingJSON, &instance); err != nil {
			t.Fatalf("%s: decode root editing JSON: %v", spec.id, err)
		}
		if err := resolved.Validate(instance); err != nil {
			t.Fatalf("%s: root editing JSON %s does not match schema: %v", spec.id, editingJSON, err)
		}
	}
}

func TestGeneratedKCESPartsSchemasAcceptRootNull(t *testing.T) {
	for _, formatID := range []string{"kces.menuassets", "kces.materialassets", "kces.pmatassets", "kces.model"} {
		t.Run(formatID, func(t *testing.T) {
			schema := compileGeneratedSchema(t, formatID)
			assertSchemaValidation(t, schema, nil, true)
		})
	}
}

func TestGeneratedVirtualDirectoryFramingSchemaRejectsUnknownValues(t *testing.T) {
	schema := compileGeneratedSchema(t, "kces.system")
	instance := jsonObject(t, serializationKCES.NewKCESSystemData())
	for _, test := range []struct {
		value float64
		valid bool
	}{
		{value: float64(serializationKCESCT.VirtualDirectoryFramingLegacy), valid: true},
		{value: float64(serializationKCESCT.VirtualDirectoryFramingExtended), valid: true},
		{value: 2, valid: false},
		{value: 255, valid: false},
		{value: -1, valid: false},
	} {
		instance["containerFraming"] = test.value
		assertSchemaValidation(t, schema, instance, test.valid)
	}
}

func TestGeneratedRequiredKCESObjectsRejectNull(t *testing.T) {
	opaquePreset, err := serializationKCES.NewKCESPreset()
	if err != nil {
		t.Fatal(err)
	}
	preset, err := serializationKCES.ExpandKCESPreset(opaquePreset)
	if err != nil {
		t.Fatal(err)
	}
	bridge := serializationKCES.NewKCESBridgeSession("session")
	contentTable := &KCESService.CtEnvelope{
		Format: KCESService.CtEnvelopeFormat,
		Catalog: serializationKCESCT.AssetBundleCatalog{
			Kind: serializationKCESCT.CatalogKindAssetBundle,
		},
	}

	tests := []struct {
		formatID string
		value    any
		property string
	}{
		{formatID: "kces.preset", value: preset, property: "maidData"},
		{formatID: "kces.bridge_session", value: bridge, property: "sessionData"},
		{formatID: "kces.ct", value: contentTable, property: "catalog"},
	}
	for _, test := range tests {
		t.Run(test.formatID, func(t *testing.T) {
			schema := compileGeneratedSchema(t, test.formatID)
			instance := jsonObject(t, test.value)
			assertSchemaValidation(t, schema, instance, true)
			instance[test.property] = nil
			assertSchemaValidation(t, schema, instance, false)
		})
	}

	t.Run("kces.bridge_session sessionId", func(t *testing.T) {
		schema := compileGeneratedSchema(t, "kces.bridge_session")
		instance := jsonObject(t, bridge)
		instance["sessionData"].(map[string]any)["sessionId"] = nil
		assertSchemaValidation(t, schema, instance, false)
	})
}

func TestGeneratedKCESEditingSchemasRejectNullEntriesAndEmptyExtensionKeys(t *testing.T) {
	t.Run("kces.enm entries", func(t *testing.T) {
		schema := compileGeneratedSchema(t, "kces.enm")
		valid := map[string]any{
			"format":  serializationKCES.KCESExportNameMapFormat,
			"version": float64(serializationKCES.KCESExportNameMapVersion),
			"entries": []any{},
		}
		assertSchemaValidation(t, schema, valid, true)
		valid["entries"] = nil
		assertSchemaValidation(t, schema, valid, false)
	})

	t.Run("kces.ct extensionNameLists key", func(t *testing.T) {
		schema := compileGeneratedSchema(t, "kces.ct")
		instance := map[string]any{
			"format":  KCESService.CtEnvelopeFormat,
			"version": float64(1000),
			"catalog": map[string]any{
				"kind":              string(serializationKCESCT.CatalogKindAssetBundle),
				"version":           float64(1000),
				"catalogType":       float64(serializationKCESCT.CatalogTypeParts),
				"packageType":       float64(serializationKCESCT.PackageTypePlugin),
				"priority":          float64(0),
				"name":              nil,
				"subName":           nil,
				"hash":              float64(0),
				"createTime":        float64(0),
				"extensionList":     []any{},
				"isEncrypted":       false,
				"resourceFileNames": []any{},
				"items":             []any{},
			},
			"extensionNameLists": map[string]any{
				".x": map[string]any{"extention": ".x", "data": []any{}},
			},
		}
		assertSchemaValidation(t, schema, instance, true)
		instance["extensionNameLists"] = map[string]any{
			"": map[string]any{"extention": ".x", "data": []any{}},
		}
		assertSchemaValidation(t, schema, instance, false)
	})
}

func TestPublishedSchemasMatchGeneratedSchemas(t *testing.T) {
	documents, err := GenerateAll()
	if err != nil {
		t.Fatal(err)
	}
	for id, expected := range documents {
		actual, found, err := editingv1.Lookup(id)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if !found {
			t.Fatalf("%s: published schema is missing", id)
		}
		if string(actual.JSON) != string(expected.JSON) || actual.SHA256 != expected.SHA256 {
			t.Fatalf("%s: published schema differs from generator", id)
		}
	}
	published, err := editingv1.Formats()
	if err != nil {
		t.Fatal(err)
	}
	expected := make([]string, 0, len(documents))
	for id := range documents {
		expected = append(expected, id)
	}
	sort.Strings(expected)
	if len(published) != len(expected) {
		t.Fatalf("published schema count = %d, generated count = %d (%v vs %v)", len(published), len(expected), published, expected)
	}
	for i := range expected {
		if published[i] != expected[i] {
			t.Fatalf("published schema IDs = %v, generated IDs = %v", published, expected)
		}
	}
}

func TestColliderRefSchemaBindsDiscriminatorToConcreteObject(t *testing.T) {
	document, err := Generate("kces.dbcol")
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(document.JSON, &root); err != nil {
		t.Fatal(err)
	}
	definitions, ok := root["$defs"].(map[string]any)
	if !ok {
		t.Fatalf("schema definitions = %#v", root["$defs"])
	}
	colliderRef, ok := definitions["KCES_ColliderRef"].(map[string]any)
	if !ok {
		t.Fatalf("ColliderRef schema = %#v", definitions["KCES_ColliderRef"])
	}
	branches, ok := colliderRef["oneOf"].([]any)
	if !ok || len(branches) != 4 {
		t.Fatalf("ColliderRef oneOf = %#v", colliderRef["oneOf"])
	}
	seen := make(map[int]bool)
	for _, rawBranch := range branches {
		branch := rawBranch.(map[string]any)
		properties := branch["properties"].(map[string]any)
		typeSchema := properties["type"].(map[string]any)
		tag, ok := typeSchema["const"].(float64)
		if !ok {
			t.Fatalf("collider branch type = %#v", typeSchema)
		}
		seen[int(tag)] = true
		if _, ok := properties["collider"].(map[string]any); !ok {
			t.Fatalf("collider branch has no concrete object schema: %#v", branch)
		}
	}
	for tag := 0; tag <= 3; tag++ {
		if !seen[tag] {
			t.Fatalf("ColliderRef oneOf is missing tag %d", tag)
		}
	}
}

func TestGeneratedSchemasPublishExact64BitIntegerBounds(t *testing.T) {
	for _, formatID := range []string{"kces.bytes", "kces.ct"} {
		document, err := Generate(formatID)
		if err != nil {
			t.Fatal(err)
		}
		decoder := json.NewDecoder(strings.NewReader(string(document.JSON)))
		decoder.UseNumber()
		var root any
		if err := decoder.Decode(&root); err != nil {
			t.Fatal(err)
		}
		foundSigned, foundUnsigned := false, false
		walkExactIntegerBounds(root, &foundSigned, &foundUnsigned)
		if !foundSigned {
			t.Fatalf("%s has no exact signed 64-bit bounds", formatID)
		}
		if formatID == "kces.ct" && !foundUnsigned {
			t.Fatalf("%s has no exact unsigned 64-bit bounds", formatID)
		}
	}
}

func TestSchemaRootsUseFixedWidthIntegerTypes(t *testing.T) {
	for _, spec := range formatSpecs() {
		if err := validateFixedWidthIntegerTypes(spec.root); err != nil {
			t.Errorf("%s: %v", spec.id, err)
		}
	}
	if err := validateFixedWidthIntegerTypes(reflect.TypeOf(struct {
		Value int `json:"value"`
	}{})); err == nil {
		t.Fatal("fixed-width validation accepted a host int")
	}
}

func TestGeneratedIntegerWidthsMatchWireTypes(t *testing.T) {
	tests := []struct {
		name     string
		formatID string
		path     []string
		bits     string
		signed   bool
		minimum  string
		maximum  string
	}{
		{
			name: "collider package version", formatID: "kces.dbcol",
			path: []string{"properties", "version"},
			bits: "32", signed: true, minimum: "-2147483648", maximum: "2147483647",
		},
		{
			name: "catalog priority", formatID: "kces.ct",
			path: []string{"$defs", definitionName(typeOf[serializationKCESCT.AssetBundleCatalog]()), "properties", "priority"},
			bits: "32", signed: true, minimum: "-2147483648", maximum: "2147483647",
		},
		{
			name: "catalog resource index", formatID: "kces.ct",
			path: []string{"$defs", definitionName(typeOf[serializationKCESCT.CatalogItem]()), "properties", "resourceIndex"},
			bits: "32", signed: true, minimum: "-2147483648", maximum: "2147483647",
		},
		{
			name: "unity path id", formatID: "kces.bytes",
			path: []string{"properties", "pathId"},
			bits: "64", signed: true, minimum: "-9223372036854775808", maximum: "9223372036854775807",
		},
		{
			name: "catalog hash", formatID: "kces.ct",
			path: []string{"$defs", definitionName(typeOf[serializationKCESCT.AssetBundleCatalog]()), "properties", "hash"},
			bits: "64", signed: false, minimum: "0", maximum: "18446744073709551615",
		},
		{
			name: "animation property index", formatID: "com3d2.anm",
			path: []string{"$defs", definitionName(typeOf[serializationCOM3D2.PropertyCurve]()), "properties", "PropertyIndex"},
			bits: "32", signed: true, minimum: "-2147483648", maximum: "2147483647",
		},
		{
			name: "model morph index", formatID: "com3d2.model",
			path: []string{"$defs", definitionName(typeOf[serializationCOM3D2.MorphData]()), "properties", "Indices", "items"},
			bits: "32", signed: true, minimum: "-2147483648", maximum: "2147483647",
		},
		{
			name: "preset slot order", formatID: "com3d2.preset",
			path: []string{"$defs", definitionName(typeOf[serializationCOM3D2.PresetProperty]()), "properties", "SkinPositionOrder", "items"},
			bits: "32", signed: true, minimum: "-2147483648", maximum: "2147483647",
		},
		{
			name: "preset attach RID", formatID: "com3d2.preset",
			path: []string{"$defs", definitionName(typeOf[serializationCOM3D2.BoneAttachPosEntry]()), "properties", "RID"},
			bits: "32", signed: true, minimum: "-2147483648", maximum: "2147483647",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			node := generatedSchemaNode(t, test.formatID, test.path...)
			if got := node["x-meido-integer-bits"].(json.Number).String(); got != test.bits {
				t.Errorf("bits = %s, want %s", got, test.bits)
			}
			if got := node["x-meido-integer-signed"]; got != test.signed {
				t.Errorf("signed = %v, want %v", got, test.signed)
			}
			if got := node["minimum"].(json.Number).String(); got != test.minimum {
				t.Errorf("minimum = %s, want %s", got, test.minimum)
			}
			if got := node["maximum"].(json.Number).String(); got != test.maximum {
				t.Errorf("maximum = %s, want %s", got, test.maximum)
			}
		})
	}
}

func generatedSchemaNode(t *testing.T, formatID string, path ...string) map[string]any {
	t.Helper()
	document, err := Generate(formatID)
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(document.JSON))
	decoder.UseNumber()
	var current any
	if err := decoder.Decode(&current); err != nil {
		t.Fatal(err)
	}
	for _, segment := range path {
		object, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("%s path %v reached %T before %q", formatID, path, current, segment)
		}
		current, ok = object[segment]
		if !ok {
			t.Fatalf("%s path %v is missing %q", formatID, path, segment)
		}
	}
	node, ok := current.(map[string]any)
	if !ok {
		t.Fatalf("%s path %v = %T, want schema object", formatID, path, current)
	}
	return node
}

func TestPublishedSchemasEnforceExact64BitIntegerBounds(t *testing.T) {
	document, found, err := editingv1.Lookup("kces.ct")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("published kces.ct schema is missing")
	}
	decoder := json.NewDecoder(strings.NewReader(string(document.JSON)))
	decoder.UseNumber()
	var root any
	if err := decoder.Decode(&root); err != nil {
		t.Fatal(err)
	}
	nodes := make(map[bool]map[string]any)
	collect64BitIntegerSchemas(root, nodes)

	tests := map[bool][]struct {
		value string
		valid bool
	}{
		true: {
			{value: "-9223372036854775808", valid: true},
			{value: "9223372036854775807", valid: true},
			{value: "-9223372036854775809", valid: false},
			{value: "9223372036854775808", valid: false},
		},
		false: {
			{value: "0", valid: true},
			{value: "18446744073709551615", valid: true},
			{value: "-1", valid: false},
			{value: "18446744073709551616", valid: false},
		},
	}
	for signed, cases := range tests {
		node, ok := nodes[signed]
		if !ok {
			t.Fatalf("published kces.ct schema has no signed=%v 64-bit integer", signed)
		}
		compiler := strictschema.NewCompiler()
		compiler.DefaultDraft(strictschema.Draft2020)
		if err := compiler.AddResource("schema.json", node); err != nil {
			t.Fatal(err)
		}
		schema, err := compiler.Compile("schema.json")
		if err != nil {
			t.Fatal(err)
		}
		for _, test := range cases {
			err := schema.Validate(json.Number(test.value))
			if (err == nil) != test.valid {
				t.Errorf("signed=%v value %s validation error = %v, valid=%v", signed, test.value, err, test.valid)
			}
		}
	}
}

func collect64BitIntegerSchemas(value any, result map[bool]map[string]any) {
	switch current := value.(type) {
	case map[string]any:
		bits, bitsOK := current["x-meido-integer-bits"].(json.Number)
		signed, signedOK := current["x-meido-integer-signed"].(bool)
		if bitsOK && bits.String() == "64" && signedOK {
			if _, exists := result[signed]; !exists {
				result[signed] = current
			}
		}
		for _, child := range current {
			collect64BitIntegerSchemas(child, result)
		}
	case []any:
		for _, child := range current {
			collect64BitIntegerSchemas(child, result)
		}
	}
}

func walkExactIntegerBounds(value any, signed, unsigned *bool) {
	switch current := value.(type) {
	case map[string]any:
		bits, bitsOK := current["x-meido-integer-bits"].(json.Number)
		kind, signedOK := current["x-meido-integer-signed"].(bool)
		if bitsOK && bits.String() == "64" && signedOK {
			minimum, minOK := current["minimum"].(json.Number)
			maximum, maxOK := current["maximum"].(json.Number)
			if !minOK || !maxOK {
				panic("64-bit schema node is missing standard bounds")
			}
			if kind && minimum.String() == "-9223372036854775808" && maximum.String() == "9223372036854775807" {
				*signed = true
			}
			if !kind && minimum.String() == "0" && maximum.String() == "18446744073709551615" {
				*unsigned = true
			}
		}
		for _, child := range current {
			walkExactIntegerBounds(child, signed, unsigned)
		}
	case []any:
		for _, child := range current {
			walkExactIntegerBounds(child, signed, unsigned)
		}
	}
}

func compileGeneratedSchema(t *testing.T, formatID string) *strictschema.Schema {
	t.Helper()
	document, err := Generate(formatID)
	if err != nil {
		t.Fatal(err)
	}
	resource, err := strictschema.UnmarshalJSON(bytes.NewReader(document.JSON))
	if err != nil {
		t.Fatal(err)
	}
	compiler := strictschema.NewCompiler()
	compiler.DefaultDraft(strictschema.Draft2020)
	compiler.AssertContent()
	if err := compiler.AddResource(document.ID, resource); err != nil {
		t.Fatal(err)
	}
	compiled, err := compiler.Compile(document.ID)
	if err != nil {
		t.Fatal(err)
	}
	return compiled
}

func assertSchemaValidation(t *testing.T, schema *strictschema.Schema, instance any, valid bool) {
	t.Helper()
	err := schema.Validate(instance)
	if (err == nil) != valid {
		t.Fatalf("schema validation error = %v, valid=%v, instance=%#v", err, valid, instance)
	}
}

func jsonObject(t *testing.T, value any) map[string]any {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var result map[string]any
	if err := decoder.Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result
}

func payloadDescriptorExtensions() []string {
	return []string{
		serializationKCES.KCESDBConfExtension,
		serializationKCES.KCESDBColExtension,
		serializationKCES.KCESDB2ConfExtension,
		serializationKCES.KCESDSBConfExtension,
		serializationKCES.KCESDSB2ConfExtension,
		serializationKCES.KCESDSLConfExtension,
		serializationKCES.KCESDSL2ConfExtension,
		serializationKCES.KCESDSLColExtension,
		serializationKCES.KCESIKColExtension,
		serializationKCES.KCESIKColBytesExtension,
		serializationKCES.KCESLimbColExtension,
	}
}

// nativePayloadRoot 返回一个载荷扩展名声明的编辑 JSON 根对象
// nativePayloadRoot returns the editing JSON root object declared by one payload extension
func nativePayloadRoot(descriptor serializationKCES.KCESPayloadDescriptor) any {
	switch descriptor.Kind {
	case serializationKCES.PayloadKindDynamicBoneStatus:
		return serializationKCES.NewDynamicBoneStatus()
	case serializationKCES.PayloadKindColliderPackage:
		return &serializationKCES.ColliderPackage{Version: 1000}
	case serializationKCES.PayloadKindLimbCollider:
		return &serializationKCES.LimbColliderPackage{
			Version: 1000,
			Items: []*serializationKCES.LimbColliderItem{{
				Version: 1000, Collider: serializationKCES.NewColliderMaidProp(),
			}},
		}
	case serializationKCES.PayloadKindIKCollider:
		return &serializationKCES.IKColliderPackage{Version: 1000}
	case serializationKCES.PayloadKindClothParams:
		return serializationKCES.NewClothParams()
	case serializationKCES.PayloadKindJSONString:
		return &serializationKCES.MagicaClothSerializeData{}
	default:
		panic("unsupported native payload kind " + descriptor.Kind)
	}
}
