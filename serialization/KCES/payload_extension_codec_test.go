package KCES

import (
	"bytes"
	"reflect"
	"testing"
)

// 每个扩展名的编解码函数只接受该扩展名声明的载荷根类型，因此扩展名与根类型的约定由类型系统保证，
// 这里只需验证按扩展名调度的 DecodeKCESPayload/EncodeKCESPayload 与直接调用完全一致
// The codec functions of each extension only accept the payload root type that extension declares, so the
// extension-to-root-type contract is enforced by the type system, and this only verifies that the
// extension-dispatched DecodeKCESPayload/EncodeKCESPayload agree exactly with the direct calls

func TestExtensionPayloadCodecsMatchCompatibilityDispatch(t *testing.T) {
	tests := []struct {
		name      string
		extension string
		kind      string
		encode    func(any) ([]byte, error)
		decode    func([]byte) (any, error)
	}{
		{
			name: "dbconf", extension: KCESDBConfExtension, kind: PayloadKindDynamicBoneStatus,
			encode: func(value any) ([]byte, error) { return EncodeDBConf(value.(*DynamicBoneStatus)) },
			decode: func(data []byte) (any, error) { return DecodeDBConf(data) },
		},
		{
			name: "dbcol", extension: KCESDBColExtension, kind: PayloadKindColliderPackage,
			encode: func(value any) ([]byte, error) { return EncodeDBCol(value.(*ColliderPackage)) },
			decode: func(data []byte) (any, error) { return DecodeDBCol(data) },
		},
		{
			name: "db2conf", extension: KCESDB2ConfExtension, kind: PayloadKindJSONString,
			encode: func(value any) ([]byte, error) { return EncodeDB2Conf(value.(*MagicaClothSerializeData)) },
			decode: func(data []byte) (any, error) { return DecodeDB2Conf(data) },
		},
		{
			name: "dsbconf", extension: KCESDSBConfExtension, kind: PayloadKindClothParams,
			encode: func(value any) ([]byte, error) { return EncodeDSBConf(value.(*ClothParams)) },
			decode: func(data []byte) (any, error) { return DecodeDSBConf(data) },
		},
		{
			name: "dsb2conf", extension: KCESDSB2ConfExtension, kind: PayloadKindJSONString,
			encode: func(value any) ([]byte, error) { return EncodeDSB2Conf(value.(*MagicaClothSerializeData)) },
			decode: func(data []byte) (any, error) { return DecodeDSB2Conf(data) },
		},
		{
			name: "dslconf", extension: KCESDSLConfExtension, kind: PayloadKindClothParams,
			encode: func(value any) ([]byte, error) { return EncodeDSLConf(value.(*ClothParams)) },
			decode: func(data []byte) (any, error) { return DecodeDSLConf(data) },
		},
		{
			name: "dsl2conf", extension: KCESDSL2ConfExtension, kind: PayloadKindJSONString,
			encode: func(value any) ([]byte, error) { return EncodeDSL2Conf(value.(*MagicaClothSerializeData)) },
			decode: func(data []byte) (any, error) { return DecodeDSL2Conf(data) },
		},
		{
			name: "dslcol", extension: KCESDSLColExtension, kind: PayloadKindColliderPackage,
			encode: func(value any) ([]byte, error) { return EncodeDSLCol(value.(*ColliderPackage)) },
			decode: func(data []byte) (any, error) { return DecodeDSLCol(data) },
		},
		{
			name: "ikcol", extension: KCESIKColExtension, kind: PayloadKindIKCollider,
			encode: func(value any) ([]byte, error) { return EncodeIKCol(value.(*IKColliderPackage)) },
			decode: func(data []byte) (any, error) { return DecodeIKCol(data) },
		},
		{
			name: "ikcol.bytes", extension: KCESIKColBytesExtension, kind: PayloadKindIKCollider,
			encode: func(value any) ([]byte, error) { return EncodeIKColBytes(value.(*IKColliderPackage)) },
			decode: func(data []byte) (any, error) { return DecodeIKColBytes(data) },
		},
		{
			name: "limbcol", extension: KCESLimbColExtension, kind: PayloadKindLimbCollider,
			encode: func(value any) ([]byte, error) { return EncodeLimbCol(value.(*LimbColliderPackage)) },
			decode: func(data []byte) (any, error) { return DecodeLimbCol(data) },
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			root := newPayloadRootForKind(test.kind)
			directWire, err := test.encode(root)
			if err != nil {
				t.Fatalf("extension encoder: %v", err)
			}
			directValue, err := test.decode(directWire)
			if err != nil {
				t.Fatalf("extension decoder: %v", err)
			}
			if !reflect.DeepEqual(directValue, root) {
				t.Fatalf("extension round trip = %#v, want %#v", directValue, root)
			}

			compatibilityWire, err := EncodeKCESPayload(root, test.extension)
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
