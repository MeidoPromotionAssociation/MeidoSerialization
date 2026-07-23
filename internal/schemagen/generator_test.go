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

func TestGeneratedSchemasRejectMalformedBase64(t *testing.T) {
	schema := compileGeneratedSchema(t, "kces.dbconf")
	for _, test := range []struct {
		name  string
		value string
		valid bool
	}{
		{name: "valid padded", value: "AQ==", valid: true},
		{name: "valid unpadded quartet", value: "YWJj", valid: true},
		{name: "invalid character", value: "AA*A", valid: false},
		{name: "invalid padding", value: "A===", valid: false},
		{name: "non-multiple length", value: "AAA", valid: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			descriptor, ok := serializationKCES.DescribeKCESPayload(".dbconf")
			if !ok {
				t.Fatal("missing .dbconf descriptor")
			}
			envelope := nativeNilPayloadEnvelope(descriptor)
			instance := jsonObject(t, envelope)
			instance["msgpackTrailingData"] = test.value
			assertSchemaValidation(t, schema, instance, test.valid)
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
			envelope := nativePayloadEnvelope(descriptor)
			envelope.LimbCollider.Items[0].Collider = test.collider
			assertSchemaValidation(t, schema, jsonObject(t, envelope), test.valid)
		})
	}
}

func TestGeneratedKCESPayloadSchemasBindDescriptorTuplesAndRoots(t *testing.T) {
	for _, extension := range payloadDescriptorExtensions() {
		descriptor, ok := serializationKCES.DescribeKCESPayload(extension)
		if !ok {
			t.Fatalf("missing payload descriptor for %s", extension)
		}
		formatID := "kces." + strings.TrimPrefix(extension, ".")
		t.Run(formatID, func(t *testing.T) {
			schema := compileGeneratedSchema(t, formatID)
			native := nativePayloadEnvelope(descriptor)
			assertSchemaValidation(t, schema, jsonObject(t, native), true)
			assertSchemaValidation(t, schema, jsonObject(t, nativeNilPayloadEnvelope(descriptor)), true)

			for _, mismatch := range []struct {
				name     string
				property string
				value    any
			}{
				{name: "format", property: "format", value: "wrong-format"},
				{name: "storage", property: "storageVariant", value: "wrong-storage"},
				{name: "kind", property: "kind", value: "wrong-kind"},
				{name: "length prefix", property: "lengthPrefixed", value: !descriptor.LengthPrefixed},
			} {
				t.Run("reject "+mismatch.name, func(t *testing.T) {
					instance := jsonObject(t, native)
					instance[mismatch.property] = mismatch.value
					assertSchemaValidation(t, schema, instance, false)
				})
			}

			t.Run("reject missing active root", func(t *testing.T) {
				instance := jsonObject(t, native)
				if descriptor.Kind == serializationKCES.PayloadKindJSONString {
					delete(instance, "text")
					delete(instance, "json")
				} else {
					delete(instance, nativePayloadRootField(descriptor.Kind))
				}
				assertSchemaValidation(t, schema, instance, false)
			})

			t.Run("reject explicit null inactive root", func(t *testing.T) {
				instance := jsonObject(t, native)
				instance[inactivePayloadRootField(descriptor.Kind)] = nil
				assertSchemaValidation(t, schema, instance, false)
			})

			t.Run("reject nil root conflict", func(t *testing.T) {
				instance := jsonObject(t, native)
				instance["msgpackRootNil"] = true
				assertSchemaValidation(t, schema, instance, false)
			})

			if descriptor.ExportCMKind != "" {
				exportEnvelope := &serializationKCES.KCESPayloadEnvelope{
					Format:         serializationKCES.PayloadFormatKCESExportCM,
					Extension:      descriptor.Extension,
					LengthPrefixed: false,
					StorageVariant: descriptor.ExportCMStorageVariant,
					Kind:           descriptor.ExportCMKind,
					JSON:           json.RawMessage(`{"ok":true}`),
				}
				assertSchemaValidation(t, schema, jsonObject(t, exportEnvelope), true)
				t.Run("reject ExportCM native field", func(t *testing.T) {
					instance := jsonObject(t, exportEnvelope)
					instance["dynamicBoneStatus"] = nil
					assertSchemaValidation(t, schema, instance, false)
				})
			}
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

		root := reflect.New(spec.root)
		if root.Elem().Type() == typeOf[serializationKCES.KCESPayloadEnvelope]() {
			extension := "." + strings.TrimPrefix(spec.id, "kces.")
			descriptor, ok := serializationKCES.DescribeKCESPayload(extension)
			if !ok {
				t.Fatalf("%s: no payload descriptor", spec.id)
			}
			envelope := root.Interface().(*serializationKCES.KCESPayloadEnvelope)
			envelope.Format = serializationKCES.PayloadFormatKCESMessagePack
			envelope.Extension = descriptor.Extension
			envelope.LengthPrefixed = descriptor.LengthPrefixed
			envelope.StorageVariant = serializationKCES.PayloadStorageInt32LZ4MessagePack
			envelope.Kind = descriptor.Kind
			envelope.MsgpackRootNil = true
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
	if !ok || len(branches) != 5 {
		t.Fatalf("ColliderRef oneOf = %#v", colliderRef["oneOf"])
	}
	seen := make(map[int]bool)
	for _, rawBranch := range branches[:4] {
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
	rawBranch := branches[4].(map[string]any)
	required := rawBranch["required"].([]any)
	if !containsAny(required, "colliderRaw") {
		t.Fatalf("raw union branch required = %#v", required)
	}
}

func containsAny(values []any, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
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

func TestKCESSchemaRootsUseFixedWidthIntegerTypes(t *testing.T) {
	for _, spec := range formatSpecs() {
		if !strings.HasPrefix(spec.id, "kces.") {
			continue
		}
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

func TestGeneratedKCESIntegerWidthsMatchWireTypes(t *testing.T) {
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
			path: []string{"$defs", definitionName(typeOf[serializationKCES.ColliderPackage]()), "properties", "version"},
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

func nativePayloadEnvelope(descriptor serializationKCES.KCESPayloadDescriptor) *serializationKCES.KCESPayloadEnvelope {
	envelope := &serializationKCES.KCESPayloadEnvelope{
		Format:         serializationKCES.PayloadFormatKCESMessagePack,
		Extension:      descriptor.Extension,
		LengthPrefixed: descriptor.LengthPrefixed,
		StorageVariant: serializationKCES.PayloadStorageInt32LZ4MessagePack,
		Kind:           descriptor.Kind,
	}
	switch descriptor.Kind {
	case serializationKCES.PayloadKindDynamicBoneStatus:
		envelope.DynamicBone = serializationKCES.NewDynamicBoneStatus()
	case serializationKCES.PayloadKindColliderPackage:
		envelope.ColliderPackage = &serializationKCES.ColliderPackage{Version: 1000}
	case serializationKCES.PayloadKindLimbCollider:
		envelope.LimbCollider = &serializationKCES.LimbColliderPackage{
			Version: 1000,
			Items: []serializationKCES.LimbColliderItem{{
				Version: 1000, Collider: serializationKCES.NewColliderMaidProp(),
			}},
		}
	case serializationKCES.PayloadKindIKCollider:
		envelope.IKCollider = &serializationKCES.IKColliderPackage{Version: 1000}
	case serializationKCES.PayloadKindClothParams:
		envelope.ClothParams = serializationKCES.NewClothParams()
	case serializationKCES.PayloadKindJSONString:
		envelope.Text = "{}"
	default:
		panic("unsupported native payload kind " + descriptor.Kind)
	}
	return envelope
}

func nativeNilPayloadEnvelope(descriptor serializationKCES.KCESPayloadDescriptor) *serializationKCES.KCESPayloadEnvelope {
	return &serializationKCES.KCESPayloadEnvelope{
		Format:         serializationKCES.PayloadFormatKCESMessagePack,
		Extension:      descriptor.Extension,
		LengthPrefixed: descriptor.LengthPrefixed,
		StorageVariant: serializationKCES.PayloadStorageInt32LZ4MessagePack,
		Kind:           descriptor.Kind,
		MsgpackRootNil: true,
	}
}

func inactivePayloadRootField(kind string) string {
	if kind == serializationKCES.PayloadKindDynamicBoneStatus {
		return "colliderPackage"
	}
	return "dynamicBoneStatus"
}
