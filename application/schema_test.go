package application

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"

	serializationCOM3D2 "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/COM3D2"
	"github.com/google/jsonschema-go/jsonschema"
)

func TestDefaultRegistryPublishesSchemaForEveryConvertibleFormat(t *testing.T) {
	engine := NewEngine(EngineOptions{})
	convertible := 0
	for _, format := range engine.Formats() {
		if !format.Capability.Convert {
			continue
		}
		convertible++
		document, err := engine.GetFormatSchema(format.ID)
		if err != nil {
			t.Fatalf("%s: %v", format.ID, err)
		}
		if format.SchemaVersion != document.Version || format.SchemaID != document.ID || format.SchemaSHA256 != document.SHA256 {
			t.Fatalf("%s: capability metadata does not match schema", format.ID)
		}
		if document.Representation != RepresentationEditingJSON || document.MediaType != "application/schema+json" || !json.Valid(document.JSON) {
			t.Fatalf("%s: invalid schema metadata", format.ID)
		}
		digest := sha256.Sum256(document.JSON)
		if document.SHA256 != fmt.Sprintf("%x", digest[:]) {
			t.Fatalf("%s: digest mismatch", format.ID)
		}
	}
	if convertible < 30 {
		t.Fatalf("only %d convertible formats expose schemas", convertible)
	}
}

func TestMenuEditingJSONValidatesAgainstPublishedSchema(t *testing.T) {
	engine := NewEngine(EngineOptions{})
	document, err := engine.GetFormatSchema(" COM3D2.MENU ")
	if err != nil {
		t.Fatal(err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(document.JSON, &schema); err != nil {
		t.Fatal(err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	editingJSON, err := json.Marshal(&serializationCOM3D2.Menu{
		Signature: serializationCOM3D2.MenuSignature, Version: 1000,
		SrcFileName: "sample.menu", ItemName: "Schema", Category: "head", InfoText: "test",
		Commands: []serializationCOM3D2.Command{{Command: "name", Args: []string{"schema"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var instance any
	if err := json.Unmarshal(editingJSON, &instance); err != nil {
		t.Fatal(err)
	}
	if err := resolved.Validate(instance); err != nil {
		t.Fatalf("editing JSON does not match its schema: %v", err)
	}

	document.JSON[0] = 'x'
	again, err := engine.GetFormatSchema("com3d2.menu")
	if err != nil || again.JSON[0] != '{' {
		t.Fatalf("schema bytes were not copied: err=%v", err)
	}
}

func TestFormatSchemaErrorsAreTyped(t *testing.T) {
	engine := NewEngine(EngineOptions{})
	if _, err := engine.GetFormatSchema(""); CodeOf(err) != CodeInvalidArgument {
		t.Fatalf("empty format ID code = %s, err=%v", CodeOf(err), err)
	}
	if _, err := engine.GetFormatSchema("missing.format"); CodeOf(err) != CodeNotFound {
		t.Fatalf("missing format code = %s, err=%v", CodeOf(err), err)
	}
	if _, err := engine.GetFormatSchema("com3d2.arc"); CodeOf(err) != CodeUnsupported {
		t.Fatalf("native-only format code = %s, err=%v", CodeOf(err), err)
	}
}

func TestRegistryDoesNotTrustCallerSuppliedSchemaMetadata(t *testing.T) {
	registry, err := NewRegistry([]Format{{
		ID: "test.custom", NativeSuffixes: []string{".custom"},
		Capability:    Capability{Convert: true},
		SchemaVersion: "forged", SchemaID: "urn:forged", SchemaSHA256: "forged",
	}})
	if err != nil {
		t.Fatal(err)
	}
	format, ok := registry.Lookup("test.custom")
	if !ok {
		t.Fatal("custom format was not registered")
	}
	if format.SchemaVersion != "" || format.SchemaID != "" || format.SchemaSHA256 != "" {
		t.Fatalf("caller-supplied schema metadata leaked through Lookup: %+v", format)
	}
	format.NativeSuffixes[0] = ".changed"
	again, _ := registry.Lookup("test.custom")
	if again.NativeSuffixes[0] != ".custom" {
		t.Fatalf("Lookup exposed mutable suffix storage: %v", again.NativeSuffixes)
	}
	listed := registry.Formats()
	if len(listed) != 1 || listed[0].SchemaVersion != "" || listed[0].SchemaID != "" || listed[0].SchemaSHA256 != "" {
		t.Fatalf("forged metadata leaked through Formats: %+v", listed)
	}
}
