package schemagen

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	serializationCOM3D2 "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/COM3D2"
	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
	KCESService "github.com/MeidoPromotionAssociation/MeidoSerialization/service/KCES"
	"github.com/google/jsonschema-go/jsonschema"
)

const (
	SchemaVersion   = "1.0.0"
	SchemaDialect   = "https://json-schema.org/draft/2020-12/schema"
	SchemaMediaType = "application/schema+json"
	SchemaIDPrefix  = "urn:meido-serialization:editing-json:v1:"
)

type Document struct {
	FormatID  string
	Version   string
	ID        string
	Dialect   string
	MediaType string
	JSON      []byte
	SHA256    string
}

type spec struct {
	id        string
	root      reflect.Type
	customize func(*jsonschema.Schema) error
}

func typeOf[T any]() reflect.Type { return reflect.TypeOf((*T)(nil)).Elem() }

func Specs() []string {
	result := make([]string, 0, len(formatSpecs()))
	for _, value := range formatSpecs() {
		result = append(result, value.id)
	}
	sort.Strings(result)
	return result
}

func GenerateAll() (map[string]Document, error) {
	result := make(map[string]Document, len(formatSpecs()))
	for _, value := range formatSpecs() {
		document, err := Generate(value.id)
		if err != nil {
			return nil, err
		}
		result[value.id] = document
	}
	return result, nil
}

func Generate(formatID string) (Document, error) {
	id := strings.ToLower(strings.TrimSpace(formatID))
	var selected *spec
	for _, value := range formatSpecs() {
		if value.id == id {
			copy := value
			selected = &copy
			break
		}
	}
	if selected == nil {
		return Document{}, fmt.Errorf("no editing schema is registered for %q", formatID)
	}
	if err := validateFixedWidthIntegerTypes(selected.root); err != nil {
		return Document{}, fmt.Errorf("validate fixed-width integer types for %s: %w", id, err)
	}

	root, definitions := buildReflectSchema(selected.root)
	if selected.customize != nil {
		if err := selected.customize(root); err != nil {
			return Document{}, fmt.Errorf("customize schema %s: %w", id, err)
		}
	}
	if len(definitions) != 0 {
		if root.Defs == nil {
			root.Defs = make(map[string]*jsonschema.Schema)
		}
		for name, value := range definitions {
			root.Defs[name] = value
		}
	}
	if err := applyKnowledgeAnnotations(id, root); err != nil {
		return Document{}, fmt.Errorf("apply knowledge annotations %s: %w", id, err)
	}
	root.Schema = SchemaDialect
	root.ID = SchemaIDPrefix + id
	root.Title = id + " editing JSON"
	root.Description = "Lossless editing JSON contract for " + id + "."
	if root.Extra == nil {
		root.Extra = make(map[string]any)
	}
	root.Extra["x-meido-format-id"] = id
	root.Extra["x-meido-representation"] = "editing_json"
	root.Extra["x-meido-schema-version"] = SchemaVersion
	root.Extra["x-meido-native-suffixes"] = nativeSuffixes(id)

	data, err := marshalSchemaWithExactIntegerBounds(root)
	if err != nil {
		return Document{}, fmt.Errorf("marshal schema %s: %w", id, err)
	}
	data = append(data, '\n')
	digest := sha256.Sum256(data)
	return Document{
		FormatID: id, Version: SchemaVersion, ID: root.ID, Dialect: SchemaDialect,
		MediaType: SchemaMediaType, JSON: data, SHA256: fmt.Sprintf("%x", digest[:]),
	}, nil
}

func marshalSchemaWithExactIntegerBounds(root *jsonschema.Schema) ([]byte, error) {
	// jsonschema-go stores numeric bounds as float64, which cannot represent
	// either endpoint of a 64-bit integer range. Decode the generated document
	// as a JSON tree and replace only those bounds with json.Number values before
	// the final marshal; all published keywords remain standard JSON Schema.
	initial, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(initial))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	injectExactIntegerBounds(value)
	return json.MarshalIndent(value, "", "  ")
}

func injectExactIntegerBounds(value any) {
	switch current := value.(type) {
	case map[string]any:
		bits, bitsOK := integerJSONNumber(current["x-meido-integer-bits"])
		signed, signedOK := current["x-meido-integer-signed"].(bool)
		if bitsOK && signedOK && bits == 64 {
			if signed {
				current["minimum"] = json.Number("-9223372036854775808")
				current["maximum"] = json.Number("9223372036854775807")
			} else {
				current["minimum"] = json.Number("0")
				current["maximum"] = json.Number("18446744073709551615")
			}
		}
		for _, child := range current {
			injectExactIntegerBounds(child)
		}
	case []any:
		for _, child := range current {
			injectExactIntegerBounds(child)
		}
	}
}

func integerJSONNumber(value any) (int64, bool) {
	number, ok := value.(json.Number)
	if !ok {
		return 0, false
	}
	result, err := strconv.ParseInt(number.String(), 10, 64)
	return result, err == nil
}

func formatSpecs() []spec {
	return []spec{
		{id: "com3d2.menu", root: typeOf[serializationCOM3D2.Menu]()},
		{id: "com3d2.mate", root: typeOf[serializationCOM3D2.Mate]()},
		{id: "com3d2.pmat", root: typeOf[serializationCOM3D2.PMat]()},
		{id: "com3d2.col", root: typeOf[serializationCOM3D2.Col]()},
		{id: "com3d2.phy", root: typeOf[serializationCOM3D2.Phy]()},
		{id: "com3d2.psk", root: typeOf[serializationCOM3D2.Psk]()},
		{id: "com3d2.anm", root: typeOf[serializationCOM3D2.Anm]()},
		{id: "com3d2.model", root: typeOf[serializationCOM3D2.Model]()},
		{id: "com3d2.preset", root: typeOf[serializationCOM3D2.Preset]()},
		{id: "com3d2.timeline", root: typeOf[serializationCOM3D2.TimelineData]()},
		{id: "com3d2.object_data", root: typeOf[serializationCOM3D2.DanceObjectData]()},
		{id: "kces.bridge_session", root: typeOf[serializationKCES.KCESBridgeSession]()},
		{id: "kces.brd", root: typeOf[serializationKCES.GP03BridgeFile]()},
		{id: "kces.enm", root: typeOf[serializationKCES.KCESExportNameMap]()},
		{id: "kces.sad", root: typeOf[serializationKCES.SavedAttachFile]()},
		{id: "kces.system", root: typeOf[serializationKCES.KCESSystemData]()},
		{id: "kces.paths", root: typeOf[serializationKCES.KCESPathsFile]()},
		{id: "kces.maid_collider", root: typeOf[serializationKCES.MaidColliderFile]()},
		{id: "kces.menuassets", root: typeOf[serializationKCES.MenuAssets]()},
		{id: "kces.materialassets", root: typeOf[serializationKCES.MaterialAssets]()},
		{id: "kces.pmatassets", root: typeOf[serializationKCES.PriorityMaterialAssets]()},
		{id: "kces.model", root: typeOf[serializationKCES.Model]()},
		{id: "kces.hitcheck", root: typeOf[serializationKCES.HitCheck]()},
		{id: "kces.undressdat", root: typeOf[serializationKCES.KCESJSONText](), customize: markerCustomizer("extension", ".undressdat")},
		{id: "kces.undresspdat", root: typeOf[serializationKCES.KCESJSONText](), customize: markerCustomizer("extension", ".undresspdat")},
		{id: "kces.nson", root: typeOf[serializationKCES.KCESJSONText](), customize: markerCustomizer("extension", ".nson")},
		{id: "kces.bytes", root: typeOf[KCESService.RawUnityObjectEnvelope]()},
		{id: "kces.preset", root: typeOf[serializationKCES.KCESPreset]()},
		{id: "kces.ct", root: typeOf[KCESService.CtEnvelope]()},
		{id: "kces.virtualdirectory", root: typeOf[KCESService.CtEnvelope]()},
		{id: "kces.dbconf", root: typeOf[serializationKCES.KCESPayloadEnvelope](), customize: payloadEnvelopeCustomizer(".dbconf")},
		{id: "kces.dbcol", root: typeOf[serializationKCES.KCESPayloadEnvelope](), customize: payloadEnvelopeCustomizer(".dbcol")},
		{id: "kces.db2conf", root: typeOf[serializationKCES.KCESPayloadEnvelope](), customize: payloadEnvelopeCustomizer(".db2conf")},
		{id: "kces.dsbconf", root: typeOf[serializationKCES.KCESPayloadEnvelope](), customize: payloadEnvelopeCustomizer(".dsbconf")},
		{id: "kces.dsb2conf", root: typeOf[serializationKCES.KCESPayloadEnvelope](), customize: payloadEnvelopeCustomizer(".dsb2conf")},
		{id: "kces.dslconf", root: typeOf[serializationKCES.KCESPayloadEnvelope](), customize: payloadEnvelopeCustomizer(".dslconf")},
		{id: "kces.dsl2conf", root: typeOf[serializationKCES.KCESPayloadEnvelope](), customize: payloadEnvelopeCustomizer(".dsl2conf")},
		{id: "kces.dslcol", root: typeOf[serializationKCES.KCESPayloadEnvelope](), customize: payloadEnvelopeCustomizer(".dslcol")},
		{id: "kces.ikcol", root: typeOf[serializationKCES.KCESPayloadEnvelope](), customize: payloadEnvelopeCustomizer(".ikcol")},
		{id: "kces.ikcol.bytes", root: typeOf[serializationKCES.KCESPayloadEnvelope](), customize: payloadEnvelopeCustomizer(".ikcol.bytes")},
		{id: "kces.limbcol", root: typeOf[serializationKCES.KCESPayloadEnvelope](), customize: payloadEnvelopeCustomizer(".limbcol")},
	}
}

func payloadEnvelopeCustomizer(extension string) func(*jsonschema.Schema) error {
	return func(root *jsonschema.Schema) error {
		descriptor, ok := serializationKCES.DescribeKCESPayload(extension)
		if !ok {
			return fmt.Errorf("no KCES payload descriptor for %q", extension)
		}
		if err := markerCustomizer("extension", descriptor.Extension)(root); err != nil {
			return err
		}

		branches := []*jsonschema.Schema{
			payloadNativeBranch(descriptor, false),
			payloadNativeBranch(descriptor, true),
		}
		if descriptor.ExportCMKind != "" {
			branches = append(branches, payloadExportCMBranch(descriptor))
		}
		root.OneOf = branches
		return nil
	}
}

func payloadNativeBranch(descriptor serializationKCES.KCESPayloadDescriptor, rootNil bool) *jsonschema.Schema {
	branch := payloadTupleBranch(
		descriptor.Extension,
		serializationKCES.PayloadFormatKCESMessagePack,
		serializationKCES.PayloadStorageInt32LZ4MessagePack,
		descriptor.Kind,
		descriptor.LengthPrefixed,
	)
	branch.Properties["msgpackRootNil"] = &jsonschema.Schema{Type: "boolean", Const: anyPtr(rootNil)}
	allRoots := payloadRootFieldNames()
	if rootNil {
		branch.Required = append(branch.Required, "msgpackRootNil")
		branch.AllOf = forbidProperties(allRoots...)
		return branch
	}

	active := nativePayloadRootField(descriptor.Kind)
	if descriptor.Kind == serializationKCES.PayloadKindJSONString {
		branch.AnyOf = []*jsonschema.Schema{
			{Required: []string{"text"}},
			{Required: []string{"json"}},
		}
		active = ""
	} else {
		branch.Properties[active] = &jsonschema.Schema{Not: &jsonschema.Schema{Type: "null"}}
		branch.Required = append(branch.Required, active)
	}
	forbidden := make([]string, 0, len(allRoots)+2)
	for _, name := range allRoots {
		if name != active && !(descriptor.Kind == serializationKCES.PayloadKindJSONString && (name == "text" || name == "json")) {
			forbidden = append(forbidden, name)
		}
	}
	forbidden = append(forbidden, "msgpackBase64", "msgpackJsonPreview")
	branch.AllOf = forbidProperties(forbidden...)
	return branch
}

func payloadExportCMBranch(descriptor serializationKCES.KCESPayloadDescriptor) *jsonschema.Schema {
	branch := payloadTupleBranch(
		descriptor.Extension,
		serializationKCES.PayloadFormatKCESExportCM,
		descriptor.ExportCMStorageVariant,
		descriptor.ExportCMKind,
		false,
	)
	branch.Properties["msgpackRootNil"] = &jsonschema.Schema{Type: "boolean", Const: anyPtr(false)}
	branch.AnyOf = []*jsonschema.Schema{
		{Required: []string{"text"}},
		{Required: []string{"json"}},
	}
	forbidden := []string{
		"dynamicBoneStatus", "colliderPackage", "limbColliderPackage", "ikColliderPackage", "clothParams",
		"msgpackBase64", "msgpackJsonPreview", "msgpackTrailingData",
	}
	branch.AllOf = forbidProperties(forbidden...)
	return branch
}

func payloadTupleBranch(extension, format, storage, kind string, lengthPrefixed bool) *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"format":         {Type: "string", Const: anyPtr(format)},
			"extension":      {Type: "string", Const: anyPtr(extension)},
			"lengthPrefixed": {Type: "boolean", Const: anyPtr(lengthPrefixed)},
			"storageVariant": {Type: "string", Const: anyPtr(storage)},
			"kind":           {Type: "string", Const: anyPtr(kind)},
		},
		Required: []string{"format", "extension", "lengthPrefixed", "storageVariant", "kind"},
	}
}

func nativePayloadRootField(kind string) string {
	switch kind {
	case serializationKCES.PayloadKindDynamicBoneStatus:
		return "dynamicBoneStatus"
	case serializationKCES.PayloadKindColliderPackage:
		return "colliderPackage"
	case serializationKCES.PayloadKindLimbCollider:
		return "limbColliderPackage"
	case serializationKCES.PayloadKindIKCollider:
		return "ikColliderPackage"
	case serializationKCES.PayloadKindClothParams:
		return "clothParams"
	default:
		return ""
	}
}

func payloadRootFieldNames() []string {
	return []string{"dynamicBoneStatus", "colliderPackage", "limbColliderPackage", "ikColliderPackage", "clothParams", "text", "json"}
}

func forbidProperties(names ...string) []*jsonschema.Schema {
	result := make([]*jsonschema.Schema, 0, len(names))
	for _, name := range names {
		result = append(result, &jsonschema.Schema{Not: &jsonschema.Schema{Required: []string{name}}})
	}
	return result
}

func markerCustomizer(property, value string) func(*jsonschema.Schema) error {
	return func(root *jsonschema.Schema) error {
		if root.Properties == nil || root.Properties[property] == nil {
			return fmt.Errorf("required marker property %q is missing", property)
		}
		field := root.Properties[property]
		field.Type = "string"
		field.Types = nil
		field.Const = anyPtr(value)
		return nil
	}
}

func anyPtr(value any) *any { return &value }

func falseSchema() *jsonschema.Schema { return &jsonschema.Schema{Not: &jsonschema.Schema{}} }

func nativeSuffixes(id string) []string {
	values := map[string][]string{
		"com3d2.menu": {".menu"}, "com3d2.mate": {".mate", ".mat"}, "com3d2.pmat": {".pmat"}, "com3d2.col": {".col"},
		"com3d2.phy": {".phy"}, "com3d2.psk": {".psk"}, "com3d2.anm": {".anm"}, "com3d2.model": {".model"},
		"com3d2.preset": {".preset"}, "com3d2.timeline": {".bytes"}, "com3d2.object_data": {".bytes"},
		"kces.bridge_session": {".vd"}, "kces.brd": {".brd"}, "kces.enm": {".enm"}, "kces.sad": {".sad"},
		"kces.system": {"system.dat"}, "kces.paths": {"paths.dat"}, "kces.maid_collider": {".bytes"},
		"kces.menuassets": {".menuassets"}, "kces.materialassets": {".materialassets"}, "kces.pmatassets": {".pmatassets"}, "kces.model": {".model"},
		"kces.hitcheck": {".hitcheck"}, "kces.undressdat": {".undressdat"}, "kces.undresspdat": {".undresspdat"}, "kces.nson": {".nson"},
		"kces.bytes": {".bytes"}, "kces.preset": {".preset", ".perset"}, "kces.ct": {".ct"}, "kces.virtualdirectory": {".vd"},
		"kces.dbconf": {".dbconf"}, "kces.dbcol": {".dbcol"}, "kces.db2conf": {".db2conf"}, "kces.dsbconf": {".dsbconf"},
		"kces.dsb2conf": {".dsb2conf"}, "kces.dslconf": {".dslconf"}, "kces.dsl2conf": {".dsl2conf"}, "kces.dslcol": {".dslcol"},
		"kces.ikcol": {".ikcol"}, "kces.ikcol.bytes": {".ikcol.bytes"}, "kces.limbcol": {".limbcol"},
	}
	return append([]string(nil), values[id]...)
}
