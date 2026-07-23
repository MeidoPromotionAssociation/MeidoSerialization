package schemagen

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"

	serializationCOM3D2 "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/COM3D2"
	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
	KCESService "github.com/MeidoPromotionAssociation/MeidoSerialization/service/KCES"
	"github.com/google/jsonschema-go/jsonschema"
)

const standardBase64Pattern = `^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$`

type reflectSchemaBuilder struct {
	defs     map[string]*jsonschema.Schema
	building map[reflect.Type]bool
	unions   map[reflect.Type][]reflect.Type
	bytes    map[reflect.Type]bool
}

func newReflectSchemaBuilder() *reflectSchemaBuilder {
	return &reflectSchemaBuilder{
		defs:     make(map[string]*jsonschema.Schema),
		building: make(map[reflect.Type]bool),
		unions: map[reflect.Type][]reflect.Type{
			typeOf[serializationCOM3D2.Property](): {
				typeOf[serializationCOM3D2.TexProperty](), typeOf[serializationCOM3D2.ColProperty](),
				typeOf[serializationCOM3D2.VecProperty](), typeOf[serializationCOM3D2.FProperty](),
				typeOf[serializationCOM3D2.RangeProperty](), typeOf[serializationCOM3D2.TexOffsetProperty](),
				typeOf[serializationCOM3D2.TexScaleProperty](), typeOf[serializationCOM3D2.KeywordProperty](),
			},
			typeOf[serializationCOM3D2.ICollider](): {
				typeOf[serializationCOM3D2.DynamicBoneCollider](), typeOf[serializationCOM3D2.DynamicBonePlaneCollider](),
				typeOf[serializationCOM3D2.DynamicBoneMuneCollider](), typeOf[serializationCOM3D2.MissingCollider](),
			},
			typeOf[serializationCOM3D2.TimelineTrack](): {
				typeOf[serializationCOM3D2.TranslationTrack](), typeOf[serializationCOM3D2.RotationTrack](),
				typeOf[serializationCOM3D2.PropertyTrack](), typeOf[serializationCOM3D2.EventTrack](),
			},
			typeOf[serializationKCES.ColliderStatusUnion](): {
				typeOf[serializationKCES.ColliderPlane](), typeOf[serializationKCES.ColliderCapsule](),
				typeOf[serializationKCES.ColliderSphere](), typeOf[serializationKCES.ColliderMaidProp](),
			},
		},
		bytes: map[reflect.Type]bool{
			typeOf[serializationKCES.RawMessagePackSlot](): true,
			typeOf[serializationKCES.GradaBytes]():         true,
		},
	}
}

func buildReflectSchema(root reflect.Type) (*jsonschema.Schema, map[string]*jsonschema.Schema) {
	builder := newReflectSchemaBuilder()
	value := builder.schema(root, true)
	return value, builder.defs
}

// validateFixedWidthIntegerTypes 验证一个 Schema 类型图只使用固定宽度整数，并覆盖生成器登记的联合类型分支。
// validateFixedWidthIntegerTypes verifies that a schema type graph uses only fixed-width integers, including registered union variants.
func validateFixedWidthIntegerTypes(root reflect.Type) error {
	builder := newReflectSchemaBuilder()
	visited := make(map[reflect.Type]bool)
	var visit func(reflect.Type, string) error
	visit = func(typ reflect.Type, path string) error {
		for typ.Kind() == reflect.Pointer {
			typ = typ.Elem()
		}
		if typ == reflect.TypeOf(json.RawMessage(nil)) || builder.bytes[typ] || visited[typ] {
			return nil
		}
		visited[typ] = true

		switch typ.Kind() {
		case reflect.Int, reflect.Uint, reflect.Uintptr:
			return fmt.Errorf("%s uses architecture-dependent %s", path, typ.Kind())
		}
		if variants, ok := builder.unions[typ]; ok {
			for _, variant := range variants {
				if err := visit(variant, path+"<"+variant.Name()+">"); err != nil {
					return err
				}
			}
			return nil
		}

		switch typ.Kind() {
		case reflect.Map:
			if err := visit(typ.Key(), path+"{key}"); err != nil {
				return err
			}
			return visit(typ.Elem(), path+"{value}")
		case reflect.Slice, reflect.Array:
			return visit(typ.Elem(), path+"[]")
		case reflect.Struct:
			for _, field := range reflect.VisibleFields(typ) {
				if len(field.Index) > 1 || field.PkgPath != "" {
					continue
				}
				_, _, omit := jsonField(field)
				if omit {
					continue
				}
				if err := visit(field.Type, path+"."+field.Name); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return visit(root, root.String())
}

func (builder *reflectSchemaBuilder) schema(typ reflect.Type, root bool) *jsonschema.Schema {
	nullable := false
	for typ.Kind() == reflect.Pointer {
		nullable = true
		typ = typ.Elem()
	}
	if typ == reflect.TypeOf(json.RawMessage(nil)) {
		return &jsonschema.Schema{}
	}
	if builder.bytes[typ] || (typ.Kind() == reflect.Slice && typ.Elem().Kind() == reflect.Uint8) {
		return base64StringSchema(true)
	}
	if variants, ok := builder.unions[typ]; ok {
		result := &jsonschema.Schema{OneOf: make([]*jsonschema.Schema, 0, len(variants))}
		for _, variant := range variants {
			result.OneOf = append(result.OneOf, builder.variant(variant, unionDiscriminator(typ, variant)))
		}
		return nullableSchema(result, nullable)
	}

	var result *jsonschema.Schema
	switch typ.Kind() {
	case reflect.Bool:
		result = &jsonschema.Schema{Type: "boolean"}
	case reflect.String:
		result = &jsonschema.Schema{Type: "string"}
	case reflect.Int:
		result = integerSchema(intBits(typ), true)
	case reflect.Int8:
		result = integerSchema(8, true)
	case reflect.Int16:
		result = integerSchema(16, true)
	case reflect.Int32:
		result = integerSchema(32, true)
	case reflect.Int64:
		result = integerSchema(64, true)
	case reflect.Uint:
		result = integerSchema(intBits(typ), false)
	case reflect.Uint8:
		result = integerSchema(8, false)
	case reflect.Uint16:
		result = integerSchema(16, false)
	case reflect.Uint32:
		result = integerSchema(32, false)
	case reflect.Uint64, reflect.Uintptr:
		result = integerSchema(64, false)
	case reflect.Float32:
		result = &jsonschema.Schema{
			Type: "number", Minimum: floatPtr(-math.MaxFloat32), Maximum: floatPtr(math.MaxFloat32),
		}
	case reflect.Float64:
		result = &jsonschema.Schema{Type: "number"}
	case reflect.Interface:
		result = &jsonschema.Schema{}
	case reflect.Map:
		result = &jsonschema.Schema{Types: []string{"null", "object"}, AdditionalProperties: builder.schema(typ.Elem(), false)}
	case reflect.Slice:
		result = &jsonschema.Schema{Types: []string{"null", "array"}, Items: builder.schema(typ.Elem(), false)}
	case reflect.Array:
		result = &jsonschema.Schema{Type: "array", Items: builder.schema(typ.Elem(), false)}
		result.MinItems = intPtr(typ.Len())
		result.MaxItems = intPtr(typ.Len())
	case reflect.Struct:
		if !root && typ.Name() != "" {
			name := definitionName(typ)
			ref := &jsonschema.Schema{Ref: "#/$defs/" + name}
			if builder.building[typ] {
				return nullableSchema(ref, nullable)
			}
			if _, exists := builder.defs[name]; !exists {
				builder.building[typ] = true
				builder.defs[name] = nil
				builder.defs[name] = builder.structSchema(typ)
				delete(builder.building, typ)
			}
			return nullableSchema(ref, nullable)
		}
		result = builder.structSchema(typ)
	default:
		result = &jsonschema.Schema{}
	}
	return nullableSchema(result, nullable)
}

func (builder *reflectSchemaBuilder) variant(typ reflect.Type, discriminator string) *jsonschema.Schema {
	value := builder.structSchema(typ)
	if discriminator != "" {
		if value.Properties == nil {
			value.Properties = make(map[string]*jsonschema.Schema)
		}
		value.Properties["TypeName"] = &jsonschema.Schema{Type: "string", Const: anyPtr(discriminator)}
		if !contains(value.Required, "TypeName") {
			value.Required = append(value.Required, "TypeName")
		}
	}
	return value
}

func (builder *reflectSchemaBuilder) structSchema(typ reflect.Type) *jsonschema.Schema {
	result := &jsonschema.Schema{Type: "object", AdditionalProperties: falseSchema(), Properties: make(map[string]*jsonschema.Schema)}
	for _, field := range reflect.VisibleFields(typ) {
		// VisibleFields includes promoted members as well as the anonymous field
		// that owns them. The anonymous branch below already flattens that child;
		// processing promoted members again would duplicate required entries and
		// can produce an invalid Draft 2020-12 `required` array.
		if len(field.Index) > 1 {
			continue
		}
		if field.PkgPath != "" {
			continue
		}
		name, optional, omit := jsonField(field)
		if omit {
			continue
		}
		if field.Anonymous && name == "" {
			embedded := dereference(field.Type)
			if embedded.Kind() == reflect.Struct && !builder.building[embedded] {
				child := builder.structSchema(embedded)
				for childName, childSchema := range child.Properties {
					result.Properties[childName] = childSchema
				}
				for _, required := range child.Required {
					if !contains(result.Required, required) {
						result.Required = append(result.Required, required)
					}
				}
				continue
			}
		}
		if name == "" {
			name = field.Name
		}
		result.Properties[name] = builder.schema(field.Type, false)
		if !optional {
			result.Required = append(result.Required, name)
		}
	}
	customizeKnownStruct(typ, result, builder)
	return result
}

func customizeKnownStruct(typ reflect.Type, schema *jsonschema.Schema, builder *reflectSchemaBuilder) {
	if typ == typeOf[serializationKCES.ColliderRef]() {
		customizeColliderRefSchema(schema, builder)
	}
	if typ == typeOf[serializationKCES.LimbColliderItem]() {
		customizeLimbColliderItemSchema(schema, builder)
	}
	if typ == typeOf[serializationKCES.KCESPayloadEnvelope]() {
		schema.Properties["msgpackBase64"] = base64StringSchema(false)
	}
	if typ == typeOf[KCESService.RawUnityObjectEnvelope]() {
		schema.Properties["dataBase64"] = base64StringSchema(false)
	}
	if typ == typeOf[KCESService.CtEnvelopeFile]() {
		schema.Properties["dataBase64"] = base64StringSchema(false)
	}
	if typ == typeOf[KCESService.TypeTreeJSONBytes]() {
		schema.Properties["dataBase64"] = base64StringSchema(false)
		schema.Properties["previewBase64"] = base64StringSchema(false)
	}
	if typ == typeOf[serializationKCES.PartsColor]() {
		schema.Properties["m_grada"] = &jsonschema.Schema{Types: []string{"null", "array"}, Items: builder.schema(typeOf[serializationKCES.PartsColorGrada](), false)}
		schema.Properties["m_gradaDecodeError"] = &jsonschema.Schema{Type: "string"}
		schema.Properties["m_gradaTrailingBytes"] = base64StringSchema(true)
	}
}

func customizeLimbColliderItemSchema(schema *jsonschema.Schema, builder *reflectSchemaBuilder) {
	// LimbColliderData.Key(2) is statically NativeMaidPropColliderStatus in
	// the game. It is not ANativeColliderStatus's tagged union.
	schema.Properties["collider"] = &jsonschema.Schema{AnyOf: []*jsonschema.Schema{
		{Type: "null"},
		builder.schema(typeOf[serializationKCES.ColliderMaidProp](), false),
	}}
	// A raw slot is retained only for a value which the fixed formatter could
	// not decode. It is mutually exclusive with the typed value.
	schema.OneOf = []*jsonschema.Schema{
		{Not: &jsonschema.Schema{Required: []string{"colliderRaw"}}},
		{
			Properties: map[string]*jsonschema.Schema{
				"collider":    {Type: "null"},
				"colliderRaw": base64StringSchema(false),
			},
			Required: []string{"colliderRaw"},
		},
	}
}

func customizeColliderRefSchema(schema *jsonschema.Schema, builder *reflectSchemaBuilder) {
	// The wire discriminator and the concrete object are one logical union.
	// Keeping them as independent properties lets a capsule object validate as
	// a plane and would silently discard its capsule-only fields on decode.
	type branchSpec struct {
		tag     int32
		variant reflect.Type
	}
	branches := []branchSpec{
		{tag: serializationKCES.ColliderTypePlane, variant: typeOf[serializationKCES.ColliderPlane]()},
		{tag: serializationKCES.ColliderTypeCapsule, variant: typeOf[serializationKCES.ColliderCapsule]()},
		{tag: serializationKCES.ColliderTypeSphere, variant: typeOf[serializationKCES.ColliderSphere]()},
		{tag: serializationKCES.ColliderTypeMaidProp, variant: typeOf[serializationKCES.ColliderMaidProp]()},
	}
	result := make([]*jsonschema.Schema, 0, len(branches)+1)
	for _, branch := range branches {
		collider := builder.schema(branch.variant, false)
		typeSchema := integerSchema(32, true)
		typeSchema.Const = anyPtr(branch.tag)
		result = append(result, &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"type":     typeSchema,
				"collider": {AnyOf: []*jsonschema.Schema{{Type: "null"}, collider}},
			},
			Required: []string{"type", "collider"},
			Not:      &jsonschema.Schema{Required: []string{"colliderRaw"}},
		})
	}
	// Unknown union tags remain lossless only when the complete MessagePack
	// value is present in colliderRaw. Known tags must use their typed object.
	result = append(result, &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"type":        integerSchema(32, true),
			"collider":    {Type: "null"},
			"colliderRaw": {Type: "string", ContentEncoding: "base64"},
		},
		Required: []string{"type", "collider", "colliderRaw"},
	})
	schema.OneOf = result
}

func jsonField(field reflect.StructField) (name string, optional, omit bool) {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return "", false, true
	}
	parts := strings.Split(tag, ",")
	name = parts[0]
	for _, option := range parts[1:] {
		if option == "omitempty" || option == "omitzero" {
			optional = true
		}
	}
	if tag == "" && !field.Anonymous {
		name = field.Name
	}
	return name, optional, false
}

func unionDiscriminator(union, variant reflect.Type) string {
	switch union {
	case typeOf[serializationCOM3D2.Property]():
		values := map[reflect.Type]string{
			typeOf[serializationCOM3D2.TexProperty](): "tex", typeOf[serializationCOM3D2.ColProperty](): "col",
			typeOf[serializationCOM3D2.VecProperty](): "vec", typeOf[serializationCOM3D2.FProperty](): "f",
			typeOf[serializationCOM3D2.RangeProperty](): "range", typeOf[serializationCOM3D2.TexOffsetProperty](): "tex_offset",
			typeOf[serializationCOM3D2.TexScaleProperty](): "tex_scale", typeOf[serializationCOM3D2.KeywordProperty](): "keyword",
		}
		return values[variant]
	case typeOf[serializationCOM3D2.ICollider]():
		values := map[reflect.Type]string{
			typeOf[serializationCOM3D2.DynamicBoneCollider](): "dbc", typeOf[serializationCOM3D2.DynamicBonePlaneCollider](): "dpc",
			typeOf[serializationCOM3D2.DynamicBoneMuneCollider](): "dbm", typeOf[serializationCOM3D2.MissingCollider](): "missing",
		}
		return values[variant]
	case typeOf[serializationCOM3D2.TimelineTrack]():
		values := map[reflect.Type]string{
			typeOf[serializationCOM3D2.TranslationTrack](): serializationCOM3D2.TrackTranslation,
			typeOf[serializationCOM3D2.RotationTrack]():    serializationCOM3D2.TrackRotation,
			typeOf[serializationCOM3D2.PropertyTrack]():    serializationCOM3D2.TrackProperty,
			typeOf[serializationCOM3D2.EventTrack]():       serializationCOM3D2.TrackEvent,
		}
		return values[variant]
	}
	return ""
}

func definitionName(typ reflect.Type) string {
	packageName := typ.PkgPath()
	if index := strings.LastIndexByte(packageName, '/'); index >= 0 {
		packageName = packageName[index+1:]
	}
	if packageName == "" {
		packageName = "root"
	}
	return strings.NewReplacer(".", "_", "-", "_", "/", "_").Replace(packageName + "_" + typ.Name())
}

func nullableSchema(value *jsonschema.Schema, nullable bool) *jsonschema.Schema {
	if !nullable {
		return value
	}
	if value.Ref != "" || len(value.AnyOf) != 0 || len(value.OneOf) != 0 {
		return &jsonschema.Schema{AnyOf: []*jsonschema.Schema{{Type: "null"}, value}}
	}
	if len(value.Types) != 0 {
		if !contains(value.Types, "null") {
			value.Types = append([]string{"null"}, value.Types...)
		}
		return value
	}
	if value.Type != "" {
		value.Types = []string{"null", value.Type}
		value.Type = ""
	}
	return value
}

func integerSchema(bits int, signed bool) *jsonschema.Schema {
	value := &jsonschema.Schema{Type: "integer"}
	if signed {
		if bits == 8 {
			value.Minimum, value.Maximum = floatPtr(math.MinInt8), floatPtr(math.MaxInt8)
		} else if bits == 16 {
			value.Minimum, value.Maximum = floatPtr(math.MinInt16), floatPtr(math.MaxInt16)
		} else if bits == 32 {
			value.Minimum, value.Maximum = floatPtr(math.MinInt32), floatPtr(math.MaxInt32)
		}
	} else {
		value.Minimum = floatPtr(0)
		if bits == 8 {
			value.Maximum = floatPtr(math.MaxUint8)
		} else if bits == 16 {
			value.Maximum = floatPtr(math.MaxUint16)
		} else if bits == 32 {
			value.Maximum = floatPtr(math.MaxUint32)
		}
	}
	if value.Extra == nil {
		value.Extra = make(map[string]any)
	}
	value.Extra["x-meido-integer-bits"] = bits
	value.Extra["x-meido-integer-signed"] = signed
	return value
}

func base64StringSchema(nullable bool) *jsonschema.Schema {
	types := []string{"string"}
	if nullable {
		types = []string{"null", "string"}
	}
	return &jsonschema.Schema{
		Types: types, ContentEncoding: "base64", Pattern: standardBase64Pattern,
	}
}

func floatPtr(value float64) *float64 { return &value }

func intBits(typ reflect.Type) int {
	if typ.Bits() != 0 {
		return typ.Bits()
	}
	return 64
}

func dereference(typ reflect.Type) reflect.Type {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return typ
}

func intPtr(value int) *int { return &value }

func contains(values []string, value string) bool {
	for _, current := range values {
		if current == value {
			return true
		}
	}
	return false
}
