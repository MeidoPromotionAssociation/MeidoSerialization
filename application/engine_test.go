package application

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	serializationCOM3D2 "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/COM3D2"
	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
	KCESService "github.com/MeidoPromotionAssociation/MeidoSerialization/service/KCES"
)

func TestEngineDetectConvertValidateMenu(t *testing.T) {
	ctx := context.Background()
	engine := NewEngine(EngineOptions{})
	native := syntheticMenuBytes(t)

	detection, err := engine.Detect(ctx, NewBytesSource("sample.menu", native))
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if detection.FormatID != "com3d2.menu" || detection.Representation != RepresentationNative {
		t.Fatalf("detection = %+v", detection)
	}

	artifact, editingJSON, err := engine.ConvertBytes(ctx, ConvertRequest{
		Source: NewBytesSource("sample.menu", native),
		To:     RepresentationEditingJSON,
	})
	if err != nil {
		t.Fatalf("Convert to editing JSON: %v", err)
	}
	if artifact.Name != "sample.menu.json" || artifact.FormatID != "com3d2.menu" || !json.Valid(editingJSON) {
		t.Fatalf("editing artifact = %+v, JSON valid=%v", artifact, json.Valid(editingJSON))
	}
	if _, err := engine.Validate(ctx, NewBytesSource(artifact.Name, editingJSON), ""); err != nil {
		t.Fatalf("Validate editing JSON: %v", err)
	}

	backArtifact, back, err := engine.ConvertBytes(ctx, ConvertRequest{
		Source: NewBytesSource(artifact.Name, editingJSON),
		To:     RepresentationNative,
	})
	if err != nil {
		t.Fatalf("Convert to native: %v", err)
	}
	if backArtifact.Name != "sample.menu" {
		t.Fatalf("native output name = %q", backArtifact.Name)
	}
	decoded, err := serializationCOM3D2.ReadMenu(bufio.NewReader(bytes.NewReader(back)))
	if err != nil {
		t.Fatalf("read converted menu: %v", err)
	}
	if decoded.Signature != serializationCOM3D2.MenuSignature || decoded.ItemName != "Synthetic Item" || len(decoded.Commands) != 1 {
		t.Fatalf("converted menu = %+v", decoded)
	}
}

func TestDefaultRegistryIncludesDanceAndPersetFormats(t *testing.T) {
	registry := DefaultRegistry()
	for _, id := range []string{"com3d2.timeline", "com3d2.object_data", "kces.preset"} {
		format, ok := registry.Lookup(id)
		if !ok || !format.Capability.Convert {
			t.Fatalf("registry lookup %q = %+v, ok=%v", id, format, ok)
		}
	}
	format, _ := registry.Lookup("kces.preset")
	if len(format.NativeSuffixes) != 2 || format.NativeSuffixes[1] != ".perset" {
		t.Fatalf("KCES preset suffixes = %v", format.NativeSuffixes)
	}
}

func TestFormatNamesPreserveJSONBasenameAndCase(t *testing.T) {
	format, ok := DefaultRegistry().Lookup("com3d2.menu")
	if !ok {
		t.Fatal("menu format is not registered")
	}
	input := formatInputName(format, "SAMPLE.JSON", RepresentationNative)
	if input != "SAMPLE.JSON" {
		t.Fatalf("native input name = %q", input)
	}
	if got := formatOutputName(format, input, RepresentationNative); got != "SAMPLE" {
		t.Fatalf("native output name = %q", got)
	}
	if got := formatOutputName(format, "sample.menu", RepresentationEditingJSON); got != "sample.menu.json" {
		t.Fatalf("editing output name = %q", got)
	}
	preset, ok := DefaultRegistry().Lookup("kces.preset")
	if !ok {
		t.Fatal("KCES preset format is not registered")
	}
	if input := formatInputName(preset, "hero.perset", RepresentationEditingJSON); input != "hero.perset" {
		t.Fatalf("perset input name = %q", input)
	}
	if got := formatOutputName(preset, "hero.perset", RepresentationEditingJSON); got != "hero.perset.json" {
		t.Fatalf("perset JSON output name = %q", got)
	}
	if got := formatOutputName(preset, "hero.perset.json", RepresentationNative); got != "hero.perset" {
		t.Fatalf("perset native output name = %q", got)
	}
}

func TestEngineDetectsAndConvertsDanceFormats(t *testing.T) {
	var timeline bytes.Buffer
	if err := (&serializationCOM3D2.TimelineData{TotalFrame: 0, FrameRate: 60}).Dump(&timeline); err != nil {
		t.Fatal(err)
	}
	var objectData bytes.Buffer
	if err := (&serializationCOM3D2.DanceObjectData{}).Dump(&objectData); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(EngineOptions{})
	ctx := context.Background()
	for _, test := range []struct {
		name   string
		data   []byte
		format string
	}{
		{name: "timeline_data.bytes", data: timeline.Bytes(), format: "com3d2.timeline"},
		{name: "maid_data.bytes", data: objectData.Bytes(), format: "com3d2.object_data"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := NewBytesSource(test.name, test.data)
			detection, err := engine.Detect(ctx, source)
			if err != nil || detection.FormatID != test.format || detection.Representation != RepresentationNative {
				t.Fatalf("native detection = %+v, err=%v", detection, err)
			}
			artifact, editing, err := engine.ConvertBytes(ctx, ConvertRequest{Source: source, To: RepresentationEditingJSON})
			if err != nil || artifact.FormatID != test.format || !json.Valid(editing) {
				t.Fatalf("native conversion = %+v, JSON=%v, err=%v", artifact, json.Valid(editing), err)
			}
			jsonDetection, err := engine.Detect(ctx, NewBytesSource(artifact.Name, editing))
			if err != nil || jsonDetection.FormatID != test.format || jsonDetection.Representation != RepresentationEditingJSON {
				t.Fatalf("JSON detection = %+v, err=%v", jsonDetection, err)
			}
			back, _, err := engine.ConvertBytes(ctx, ConvertRequest{Source: NewBytesSource(artifact.Name, editing), To: RepresentationNative})
			if err != nil || back.Name != test.name || back.FormatID != test.format {
				t.Fatalf("native round trip artifact = %+v, err=%v", back, err)
			}
		})
	}
	_, timelineJSON, err := engine.ConvertBytes(ctx, ConvertRequest{
		Source: NewBytesSource("timeline_data.bytes", timeline.Bytes()), To: RepresentationEditingJSON,
	})
	if err != nil {
		t.Fatal(err)
	}
	renamed, err := engine.Detect(ctx, NewBytesSource("renamed.bytes.json", timelineJSON))
	if err != nil || renamed.FormatID != "com3d2.timeline" {
		t.Fatalf("renamed timeline JSON detection = %+v, err=%v", renamed, err)
	}
	if _, err := engine.Detect(ctx, NewBytesSource("fake.bytes.json", []byte(`{"ok":true}`))); CodeOf(err) != CodeUnsupported {
		t.Fatalf("unmarked dance JSON error = %v", err)
	}
}

func TestEngineDoesNotExposeOversizedPathConverterOutput(t *testing.T) {
	ctx := context.WithValue(context.Background(), conversionContextKey{}, "original")
	var receivedContext context.Context
	var receivedLimit int64
	registry, err := NewRegistry([]Format{
		format("TEST", "large", "input.large", []string{".large"}, pathConverter{
			toEditing: func(callbackContext context.Context, _, output string, maxOutputBytes int64) error {
				receivedContext = callbackContext
				receivedLimit = maxOutputBytes
				return os.WriteFile(output, bytes.Repeat([]byte{'x'}, 65), 0644)
			},
			toNative: func(_ context.Context, _, output string, _ int64) error {
				return os.WriteFile(output, []byte("native"), 0644)
			},
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(EngineOptions{Registry: registry, MaxInputBytes: 128, MaxOutputBytes: 64})
	artifact, output, err := engine.ConvertBytes(ctx, ConvertRequest{
		Source: NewBytesSource("sample.large", []byte("input")), FormatID: "test.large", To: RepresentationEditingJSON,
	})
	if CodeOf(err) != CodeResourceExhausted {
		t.Fatalf("oversized converter error = %v", err)
	}
	if artifact != (Artifact{}) || len(output) != 0 {
		t.Fatalf("oversized converter leaked artifact=%+v output=%d", artifact, len(output))
	}
	if receivedContext != ctx || receivedContext.Value(conversionContextKey{}) != "original" {
		t.Fatal("converter did not receive the original context")
	}
	if receivedLimit != 64 {
		t.Fatalf("converter output limit = %d, want 64", receivedLimit)
	}
}

type conversionContextKey struct{}

func TestEngineRejectsConverterOutputWhenContextCanceledAfterCallback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	registry, err := NewRegistry([]Format{
		format("TEST", "cancel", "input.cancel", []string{".cancel"}, pathConverter{
			toEditing: func(callbackContext context.Context, _, output string, maxOutputBytes int64) error {
				if callbackContext != ctx || maxOutputBytes != 64 {
					t.Fatalf("callback context or limit changed")
				}
				if err := os.WriteFile(output, []byte("editing"), 0644); err != nil {
					return err
				}
				cancel()
				return nil
			},
			toNative: func(context.Context, string, string, int64) error { return nil },
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(EngineOptions{Registry: registry, MaxInputBytes: 128, MaxOutputBytes: 64})
	artifact, output, err := engine.ConvertBytes(ctx, ConvertRequest{
		Source: NewBytesSource("sample.cancel", []byte("input")), FormatID: "test.cancel", To: RepresentationEditingJSON,
	})
	if CodeOf(err) != CodeCanceled {
		t.Fatalf("canceled converter error = %v", err)
	}
	if artifact != (Artifact{}) || len(output) != 0 {
		t.Fatalf("canceled converter leaked artifact=%+v output=%d", artifact, len(output))
	}
}

func TestEngineRejectsPrimaryAndSidecarAggregateOverLimit(t *testing.T) {
	registry, err := NewRegistry([]Format{
		format("TEST", "sidecar", "input.sidecar", []string{".sidecar"}, pathConverter{
			toEditing: func(_ context.Context, _, output string, _ int64) error {
				if err := os.WriteFile(output, bytes.Repeat([]byte{'p'}, 40), 0644); err != nil {
					return err
				}
				return os.WriteFile(output+".meta.json", bytes.Repeat([]byte{'s'}, 25), 0644)
			},
			toNative: func(context.Context, string, string, int64) error { return nil },
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(EngineOptions{Registry: registry, MaxInputBytes: 128, MaxOutputBytes: 64})
	artifact, output, err := engine.ConvertBytes(context.Background(), ConvertRequest{
		Source: NewBytesSource("sample.sidecar", []byte("input")), FormatID: "test.sidecar", To: RepresentationEditingJSON,
	})
	if CodeOf(err) != CodeResourceExhausted {
		t.Fatalf("aggregate output error = %v", err)
	}
	if artifact != (Artifact{}) || len(output) != 0 {
		t.Fatalf("aggregate overflow leaked artifact=%+v output=%d", artifact, len(output))
	}
}

func TestEngineRejectsEditingJSONOutsidePublishedSchema(t *testing.T) {
	engine := NewEngine(EngineOptions{})
	_, valid, err := engine.ConvertBytes(context.Background(), ConvertRequest{
		Source: NewBytesSource("sample.menu", syntheticMenuBytes(t)), To: RepresentationEditingJSON,
	})
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(valid, &object); err != nil {
		t.Fatal(err)
	}
	object["schemaForbiddenField"] = true
	unknown, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string][]byte{
		"unknown field": unknown,
		"second value":  append(append([]byte(nil), valid...), []byte("\n{}")...),
	} {
		t.Run(name, func(t *testing.T) {
			source := NewBytesSource("sample.menu.json", body)
			if _, err := engine.Validate(context.Background(), source, "com3d2.menu"); CodeOf(err) != CodeInvalidArgument {
				t.Fatalf("Validate error = %v", err)
			}
			if _, _, err := engine.ConvertBytes(context.Background(), ConvertRequest{
				Source: source, FormatID: "com3d2.menu", To: RepresentationNative,
			}); CodeOf(err) != CodeInvalidArgument {
				t.Fatalf("Convert error = %v", err)
			}
		})
	}
}

func TestEngineRejectsColliderObjectThatDoesNotMatchDiscriminator(t *testing.T) {
	valid, err := json.Marshal(&serializationKCES.KCESPayloadEnvelope{
		Format:         serializationKCES.PayloadFormatKCESMessagePack,
		Extension:      ".dbcol",
		StorageVariant: serializationKCES.PayloadStorageInt32LZ4MessagePack,
		Kind:           serializationKCES.PayloadKindColliderPackage,
		ColliderPackage: &serializationKCES.ColliderPackage{
			Version: 1000,
			Colliders: []*serializationKCES.ColliderRef{{
				Type:     serializationKCES.ColliderTypeCapsule,
				Collider: serializationKCES.NewColliderCapsule(),
			}},
			LimbEnableList: []*serializationKCES.ColliderState{{Version: 1000, LimbType: 0, IsEnable: true}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(EngineOptions{})
	if _, err := engine.Validate(context.Background(), NewBytesSource("sample.dbcol.json", valid), "kces.dbcol"); err != nil {
		t.Fatalf("valid capsule editing JSON rejected: %v", err)
	}

	var envelope map[string]any
	if err := json.Unmarshal(valid, &envelope); err != nil {
		t.Fatal(err)
	}
	colliderPackage := envelope["colliderPackage"].(map[string]any)
	colliders := colliderPackage["colliders"].([]any)
	collider := colliders[0].(map[string]any)
	collider["type"] = float64(serializationKCES.ColliderTypePlane)
	mismatched, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}

	source := NewBytesSource("sample.dbcol.json", mismatched)
	if _, err := engine.Validate(context.Background(), source, "kces.dbcol"); CodeOf(err) != CodeInvalidArgument || !strings.Contains(err.Error(), "published schema") {
		t.Fatalf("Validate mismatch error = %v", err)
	}
	if _, _, err := engine.ConvertBytes(context.Background(), ConvertRequest{
		Source: source, FormatID: "kces.dbcol", To: RepresentationNative,
	}); CodeOf(err) != CodeInvalidArgument || !strings.Contains(err.Error(), "published schema") {
		t.Fatalf("Convert mismatch error = %v", err)
	}
}

func TestEngineAcceptsUTF8BOMEditingJSON(t *testing.T) {
	engine := NewEngine(EngineOptions{})
	_, editingJSON, err := engine.ConvertBytes(context.Background(), ConvertRequest{
		Source: NewBytesSource("sample.menu", syntheticMenuBytes(t)), To: RepresentationEditingJSON,
	})
	if err != nil {
		t.Fatal(err)
	}
	withBOM := append([]byte{0xef, 0xbb, 0xbf}, editingJSON...)
	source := NewBytesSource("sample.menu.json", withBOM)
	if _, err := engine.Validate(context.Background(), source, "com3d2.menu"); err != nil {
		t.Fatalf("Validate BOM-prefixed editing JSON: %v", err)
	}
	artifact, native, err := engine.ConvertBytes(context.Background(), ConvertRequest{
		Source: source, FormatID: "com3d2.menu", To: RepresentationNative,
	})
	if err != nil {
		t.Fatalf("Convert BOM-prefixed editing JSON: %v", err)
	}
	if artifact.Name != "sample.menu" || len(native) == 0 {
		t.Fatalf("native artifact = %+v, bytes=%d", artifact, len(native))
	}
}

func TestEnginePreservesRawUnityInputAndOutputAttachments(t *testing.T) {
	raw := []byte{4, 0, 0, 0, 'h', 'a', 'i', 'r', 1, 2, 3, 4}
	meta := []byte(`{"pathId":42,"loadName":"assets/hair.mmesh"}`)
	typeTree := []byte(`{"format":"kces-unity-typetree","classId":43,"typeName":"Mesh","pathId":42,"value":{"typeName":"Mesh","name":"Base"}}`)
	source, err := NewBundleSource(NewBytesSource("hair.mmesh.bytes", raw), []SourceAttachment{
		{Suffix: ".meta.json", Source: NewBytesSource("ignored", meta)},
		{Suffix: ".typetree.json", Source: NewBytesSource("ignored", typeTree)},
	})
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(EngineOptions{})
	jsonArtifact, editingJSON, err := engine.ConvertBytes(context.Background(), ConvertRequest{
		Source: source, FormatID: "kces.bytes", To: RepresentationEditingJSON,
	})
	if err != nil {
		t.Fatalf("raw to editing JSON: %v", err)
	}
	var envelope KCESService.RawUnityObjectEnvelope
	if err := json.Unmarshal(editingJSON, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.PathID != 42 || envelope.LoadName != "assets/hair.mmesh" || envelope.TypeTree == nil {
		t.Fatalf("editing envelope lost sidecars: %+v", envelope)
	}

	nativeArtifact, roundTrip, err := engine.ConvertBytes(context.Background(), ConvertRequest{
		Source: NewBytesSource(jsonArtifact.Name, editingJSON), FormatID: "kces.bytes", To: RepresentationNative,
	})
	if err != nil {
		t.Fatalf("editing JSON to raw: %v", err)
	}
	if !bytes.Equal(roundTrip, raw) {
		t.Fatalf("raw payload changed: got %x want %x", roundTrip, raw)
	}
	attachments := nativeArtifact.AttachmentFiles()
	if len(attachments) != 2 || attachments[0].Suffix != ".meta.json" || attachments[1].Suffix != ".typetree.json" {
		t.Fatalf("native attachments = %+v", attachments)
	}
	if !json.Valid(attachments[0].Data) || !json.Valid(attachments[1].Data) {
		t.Fatalf("native sidecars are not JSON: %+v", attachments)
	}
}

func syntheticMenuBytes(t *testing.T) []byte {
	t.Helper()
	menu := &serializationCOM3D2.Menu{
		Signature: serializationCOM3D2.MenuSignature,
		Version:   1000, SrcFileName: "sample.menu", ItemName: "Synthetic Item",
		Category: "head", InfoText: "test", Commands: []serializationCOM3D2.Command{{Command: "name", Args: []string{"synthetic"}}},
	}
	var output bytes.Buffer
	if err := menu.Dump(&output); err != nil {
		t.Fatalf("dump synthetic menu: %v", err)
	}
	return output.Bytes()
}
