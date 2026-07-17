package KCES

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestExportCMPayloadVariantsRoundTrip(t *testing.T) {
	t.Parallel()

	dbconfJSON := []byte(`{"version":1000,"damping":0.6,"DampingKeyFrames":[],"elasticity":0.1,"ElasticityKeyFrames":[],"stiffness":0.1,"StiffnessKeyFrames":[],"inert":0,"InertKeyFrames":[],"radius":0.02,"RadiusKeyFrames":[],"endLength":0,"endOffset":{"x":0,"y":0,"z":0},"gravity":{"x":0,"y":-0.05,"z":0},"force":{"x":0,"y":0,"z":0},"freezeAxis":0}`)
	dbcolJSON := []byte(`{"version":1000,"StatusJsonStrList":["{\"version\":1000,\"colliderType\":2,\"parentName\":\"Bip01 Head\",\"name\":\"Sphere\",\"radius\":0.5}"],"limbEnableList":[{"version":1000,"limbType":0,"isEnable":true}]}`)

	tests := []struct {
		name        string
		extension   string
		wire        []byte
		wantStorage string
		wantKind    string
		wantJSON    []byte
	}{
		{
			name:        "dbconf raw Unity JSON",
			extension:   ".dbconf",
			wire:        dbconfJSON,
			wantStorage: PayloadStorageExportCMUnityJSON,
			wantKind:    PayloadKindExportCMDynamicBoneJSON,
			wantJSON:    dbconfJSON,
		},
		{
			name:        "dbcol raw Unity JSON",
			extension:   ".dbcol",
			wire:        dbcolJSON,
			wantStorage: PayloadStorageExportCMUnityJSON,
			wantKind:    PayloadKindExportCMColliderJSON,
			wantJSON:    dbcolJSON,
		},
		{
			name:        "dslcol BinaryWriter string Unity JSON",
			extension:   ".dslcol",
			wire:        appendDotNetStringForTest(nil, dbcolJSON),
			wantStorage: PayloadStorageExportCMDotNetStringJSON,
			wantKind:    PayloadKindExportCMColliderJSON,
			wantJSON:    dbcolJSON,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			envelope, err := DecodeKCESPayload(test.wire, test.extension)
			if err != nil {
				t.Fatalf("DecodeKCESPayload() error = %v", err)
			}
			if envelope.Format != PayloadFormatKCESExportCM {
				t.Fatalf("Format = %q, want %q", envelope.Format, PayloadFormatKCESExportCM)
			}
			if envelope.StorageVariant != test.wantStorage {
				t.Fatalf("StorageVariant = %q, want %q", envelope.StorageVariant, test.wantStorage)
			}
			if envelope.Kind != test.wantKind {
				t.Fatalf("Kind = %q, want %q", envelope.Kind, test.wantKind)
			}
			if envelope.LengthPrefixed {
				t.Fatal("LengthPrefixed describes the int32+LZ4 wire and must be false for ExportCM JSON")
			}
			if !json.Valid(envelope.JSON) || !bytes.Equal(envelope.JSON, test.wantJSON) {
				t.Fatalf("JSON = %s, want %s", envelope.JSON, test.wantJSON)
			}

			encoded, err := EncodeKCESPayload(envelope)
			if err != nil {
				t.Fatalf("EncodeKCESPayload() error = %v", err)
			}
			if !bytes.Equal(encoded, test.wire) {
				t.Fatalf("round trip changed wire:\n got  %x\n want %x", encoded, test.wire)
			}
			redecoded, err := DecodeKCESPayload(encoded, test.extension)
			if err != nil {
				t.Fatalf("DecodeKCESPayload(round trip) error = %v", err)
			}
			if redecoded.StorageVariant != test.wantStorage || redecoded.Kind != test.wantKind {
				t.Fatalf("redecoded envelope = %+v", redecoded)
			}
		})
	}
}

func TestMessagePackPayloadReportsStorageVariant(t *testing.T) {
	t.Parallel()
	wire, err := EncodeDynamicBoneStatusFile(NewDynamicBoneStatus())
	if err != nil {
		t.Fatalf("EncodeDynamicBoneStatusFile() error = %v", err)
	}
	envelope, err := DecodeKCESPayload(wire, ".dbconf")
	if err != nil {
		t.Fatalf("DecodeKCESPayload() error = %v", err)
	}
	if envelope.Format != PayloadFormatKCESMessagePack || envelope.StorageVariant != PayloadStorageInt32LZ4MessagePack {
		t.Fatalf("messagepack envelope format/storage = %q/%q", envelope.Format, envelope.StorageVariant)
	}
}

func TestExportCMPayloadRejectsMalformedWire(t *testing.T) {
	t.Parallel()

	validCollider := []byte(`{"version":1000,"StatusJsonStrList":[],"limbEnableList":[]}`)
	tests := []struct {
		name      string
		extension string
		wire      []byte
		contains  string
	}{
		{name: "dbconf invalid UTF-8", extension: ".dbconf", wire: append([]byte(`{"version":1000,"damping":"`), 0xff, '"', '}'), contains: "UTF-8"},
		{name: "dbconf trailing value", extension: ".dbconf", wire: []byte(`{"version":1000} {}`), contains: "JSON"},
		{name: "dbconf malformed JSON", extension: ".dbconf", wire: []byte(`{"version":`), contains: "JSON"},
		{name: "dbcol BinaryWriter string is wrong variant", extension: ".dbcol", wire: appendDotNetStringForTest(nil, validCollider), contains: "JSON"},
		{name: "dslcol raw JSON is wrong variant", extension: ".dslcol", wire: validCollider, contains: "BinaryWriter string"},
		{name: "dslcol truncated prefix", extension: ".dslcol", wire: []byte{0x80}, contains: "EOF"},
		{name: "dslcol truncated string", extension: ".dslcol", wire: append([]byte{10}, []byte(`{}`)...), contains: "EOF"},
		{name: "dslcol trailing bytes", extension: ".dslcol", wire: append(appendDotNetStringForTest(nil, validCollider), 0), contains: "trailing"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeKCESPayload(test.wire, test.extension)
			if err == nil || !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("DecodeKCESPayload() error = %v, want text %q", err, test.contains)
			}
		})
	}
}

func TestExportCMPayloadPreservesConsumerSpecificJSONVerbatim(t *testing.T) {
	t.Parallel()

	// ExportCM only stores JsonUtility output (or a BinaryWriter string holding
	// it). Missing fields, null lists, unknown versions, and inner collider
	// strings are interpreted by the game after this wire layer has finished.
	tests := []struct {
		name      string
		extension string
		jsonText  []byte
	}{
		{name: "dbconf array root", extension: ".dbconf", jsonText: []byte("  [1, {\n  \"future\": true\n}]  \r\n")},
		{name: "dbconf partial future object", extension: ".dbconf", jsonText: []byte("{ \"version\" : -7, \"future\" : 1 }")},
		{name: "dbcol missing lists", extension: ".dbcol", jsonText: []byte(`{"version":1000}`)},
		{name: "dbcol null lists", extension: ".dbcol", jsonText: []byte(`{"version":0,"StatusJsonStrList":null,"limbEnableList":null}`)},
		{name: "dbcol opaque invalid inner status", extension: ".dbcol", jsonText: []byte(`{"StatusJsonStrList":["{"],"limbEnableList":[null],"future":true}`)},
		{name: "dslcol scalar root", extension: ".dslcol", jsonText: []byte(`42`)},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			wire := append([]byte(nil), test.jsonText...)
			if test.extension == ".dslcol" {
				wire = appendDotNetStringForTest(nil, test.jsonText)
			}
			envelope, err := DecodeKCESPayload(wire, test.extension)
			if err != nil {
				t.Fatalf("DecodeKCESPayload() error = %v", err)
			}
			encoded, err := EncodeKCESPayload(envelope)
			if err != nil {
				t.Fatalf("EncodeKCESPayload() error = %v", err)
			}
			if !bytes.Equal(encoded, wire) {
				t.Fatalf("consumer-specific JSON changed:\n got  %x\n want %x", encoded, wire)
			}
		})
	}
}

func TestExportCMPayloadPreservesOriginalJSONTextUntilEdited(t *testing.T) {
	t.Parallel()

	originalJSON := append([]byte{0xef, 0xbb, 0xbf}, []byte("{\r\n  \"version\" : 0,\r\n  \"future\" : [ 1, 2 ]\r\n}\r\n")...)
	for _, extension := range []string{".dbconf", ".dbcol", ".dslcol"} {
		extension := extension
		t.Run(extension, func(t *testing.T) {
			t.Parallel()
			wire := append([]byte(nil), originalJSON...)
			if extension == ".dslcol" {
				wire = appendDotNetStringForTest(nil, originalJSON)
			}
			envelope, err := DecodeKCESPayload(wire, extension)
			if err != nil {
				t.Fatalf("DecodeKCESPayload() error = %v", err)
			}
			if envelope.Text != string(originalJSON) {
				t.Fatalf("Text did not retain the original BOM/whitespace: %x", []byte(envelope.Text))
			}

			unchanged, err := EncodeKCESPayload(envelope)
			if err != nil {
				t.Fatalf("EncodeKCESPayload(unchanged) error = %v", err)
			}
			if !bytes.Equal(unchanged, wire) {
				t.Fatalf("unchanged JSON text was rebuilt:\n got  %x\n want %x", unchanged, wire)
			}

			envelope.JSON = json.RawMessage(` { "edited" : true } `)
			edited, err := EncodeKCESPayload(envelope)
			if err != nil {
				t.Fatalf("EncodeKCESPayload(edited) error = %v", err)
			}
			wantJSON := []byte(`{"edited":true}`)
			want := wantJSON
			if extension == ".dslcol" {
				want = appendDotNetStringForTest(nil, wantJSON)
			}
			if !bytes.Equal(edited, want) {
				t.Fatalf("edited JSON wire = %x, want %x", edited, want)
			}
		})
	}
}

func TestExportCMPayloadEnvelopeRequiresUnambiguousVariant(t *testing.T) {
	t.Parallel()

	validDynamic := json.RawMessage(`{"version":1000,"damping":0.6,"DampingKeyFrames":[],"elasticity":0.1,"ElasticityKeyFrames":[],"stiffness":0.1,"StiffnessKeyFrames":[],"inert":0,"InertKeyFrames":[],"radius":0,"RadiusKeyFrames":[],"endLength":0,"endOffset":{"x":0,"y":0,"z":0},"gravity":{"x":0,"y":0,"z":0},"force":{"x":0,"y":0,"z":0},"freezeAxis":0}`)
	tests := []struct {
		name string
		env  *KCESPayloadEnvelope
	}{
		{
			name: "ExportCM format without storage variant",
			env:  &KCESPayloadEnvelope{Format: PayloadFormatKCESExportCM, Extension: ".dbconf", Kind: PayloadKindExportCMDynamicBoneJSON, JSON: validDynamic},
		},
		{
			name: "raw JSON variant on dslcol",
			env:  &KCESPayloadEnvelope{Format: PayloadFormatKCESExportCM, Extension: ".dslcol", StorageVariant: PayloadStorageExportCMUnityJSON, Kind: PayloadKindExportCMColliderJSON, JSON: json.RawMessage(`{"version":1000,"StatusJsonStrList":[],"limbEnableList":[]}`)},
		},
		{
			name: "dotnet string variant on dbcol",
			env:  &KCESPayloadEnvelope{Format: PayloadFormatKCESExportCM, Extension: ".dbcol", StorageVariant: PayloadStorageExportCMDotNetStringJSON, Kind: PayloadKindExportCMColliderJSON, JSON: json.RawMessage(`{"version":1000,"StatusJsonStrList":[],"limbEnableList":[]}`)},
		},
		{
			name: "MessagePack format with ExportCM storage",
			env:  &KCESPayloadEnvelope{Format: PayloadFormatKCESMessagePack, Extension: ".dbconf", StorageVariant: PayloadStorageExportCMUnityJSON, Kind: PayloadKindExportCMDynamicBoneJSON, JSON: validDynamic},
		},
		{
			name: "ExportCM format with MessagePack storage",
			env:  &KCESPayloadEnvelope{Format: PayloadFormatKCESExportCM, Extension: ".dbconf", StorageVariant: PayloadStorageInt32LZ4MessagePack, Kind: PayloadKindDynamicBoneStatus, DynamicBone: NewDynamicBoneStatus()},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := EncodeKCESPayload(test.env); err == nil {
				t.Fatal("EncodeKCESPayload() unexpectedly accepted ambiguous/inconsistent envelope")
			}
		})
	}
}

func appendDotNetStringForTest(dst, value []byte) []byte {
	length := len(value)
	for length >= 0x80 {
		dst = append(dst, byte(length)|0x80)
		length >>= 7
	}
	dst = append(dst, byte(length))
	return append(dst, value...)
}
