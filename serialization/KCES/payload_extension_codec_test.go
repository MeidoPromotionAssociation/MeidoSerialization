package KCES

import (
	"bytes"
	"reflect"
	"testing"
)

func TestExtensionPayloadCodecsMatchCompatibilityDispatch(t *testing.T) {
	tests := []struct {
		name      string
		extension string
		kind      string
		encode    func(*KCESPayloadEnvelope) ([]byte, error)
		decode    func([]byte) (*KCESPayloadEnvelope, error)
	}{
		{name: "dbconf", extension: KCESDBConfExtension, kind: PayloadKindDynamicBoneStatus, encode: EncodeDBConf, decode: DecodeDBConf},
		{name: "dbcol", extension: KCESDBColExtension, kind: PayloadKindColliderPackage, encode: EncodeDBCol, decode: DecodeDBCol},
		{name: "db2conf", extension: KCESDB2ConfExtension, kind: PayloadKindJSONString, encode: EncodeDB2Conf, decode: DecodeDB2Conf},
		{name: "dsbconf", extension: KCESDSBConfExtension, kind: PayloadKindClothParams, encode: EncodeDSBConf, decode: DecodeDSBConf},
		{name: "dsb2conf", extension: KCESDSB2ConfExtension, kind: PayloadKindJSONString, encode: EncodeDSB2Conf, decode: DecodeDSB2Conf},
		{name: "dslconf", extension: KCESDSLConfExtension, kind: PayloadKindClothParams, encode: EncodeDSLConf, decode: DecodeDSLConf},
		{name: "dsl2conf", extension: KCESDSL2ConfExtension, kind: PayloadKindJSONString, encode: EncodeDSL2Conf, decode: DecodeDSL2Conf},
		{name: "dslcol", extension: KCESDSLColExtension, kind: PayloadKindColliderPackage, encode: EncodeDSLCol, decode: DecodeDSLCol},
		{name: "ikcol", extension: KCESIKColExtension, kind: PayloadKindIKCollider, encode: EncodeIKCol, decode: DecodeIKCol},
		{name: "ikcol.bytes", extension: KCESIKColBytesExtension, kind: PayloadKindIKCollider, encode: EncodeIKColBytes, decode: DecodeIKColBytes},
		{name: "limbcol", extension: KCESLimbColExtension, kind: PayloadKindLimbCollider, encode: EncodeLimbCol, decode: DecodeLimbCol},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			envelope := &KCESPayloadEnvelope{
				Format:         PayloadFormatKCESMessagePack,
				Extension:      test.extension,
				StorageVariant: PayloadStorageInt32LZ4MessagePack,
				Kind:           test.kind,
			}
			directWire, err := test.encode(envelope)
			if err != nil {
				t.Fatalf("extension encoder: %v", err)
			}
			directValue, err := test.decode(directWire)
			if err != nil {
				t.Fatalf("extension decoder: %v", err)
			}
			if !reflect.DeepEqual(directValue, envelope) {
				t.Fatalf("extension round trip = %#v, want %#v", directValue, envelope)
			}

			compatibilityWire, err := EncodeKCESPayload(envelope)
			if err != nil {
				t.Fatalf("compatibility encoder: %v", err)
			}
			if !bytes.Equal(compatibilityWire, directWire) {
				t.Fatal("compatibility encoder produced different wire bytes")
			}
			compatibilityValue, err := DecodeKCESPayload(directWire, test.extension)
			if err != nil {
				t.Fatalf("compatibility decoder: %v", err)
			}
			if !reflect.DeepEqual(compatibilityValue, directValue) {
				t.Fatalf("compatibility decoder = %#v, want %#v", compatibilityValue, directValue)
			}
		})
	}
}

func TestExtensionPayloadEncodersRejectOtherExtensions(t *testing.T) {
	envelope := &KCESPayloadEnvelope{
		Format:         PayloadFormatKCESMessagePack,
		Extension:      KCESDBColExtension,
		StorageVariant: PayloadStorageInt32LZ4MessagePack,
		Kind:           PayloadKindColliderPackage,
	}
	if _, err := EncodeDBConf(envelope); err == nil {
		t.Fatal("EncodeDBConf accepted a .dbcol envelope")
	}
}
