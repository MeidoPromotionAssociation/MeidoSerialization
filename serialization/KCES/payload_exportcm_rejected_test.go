package KCES

import (
	"encoding/json"
	"strings"
	"testing"
)

// KCES ExportCM 会用 KCES 自己的扩展名写出给 COM3D2.5 读取的 JSON 中间产物。那是游戏自动生成的
// 导出物而不是 KCES 资源，本项目只支持 KCES 原生的 int32 长度前缀 LZ4 MessagePack 线格式
// KCES ExportCM writes JSON intermediates for COM3D2.5 using KCES's own extensions. Those are
// game-generated exports rather than KCES resources, so this project only supports the native KCES
// int32-length-prefixed LZ4 MessagePack wire format

func TestKCESPayloadRejectsExportCMSidecars(t *testing.T) {
	t.Parallel()

	dynamicBoneJSON := []byte(`{"version":1000,"damping":0.6,"DampingKeyFrames":[],"elasticity":0.1,"ElasticityKeyFrames":[],"stiffness":0.1,"StiffnessKeyFrames":[],"inert":0,"InertKeyFrames":[],"radius":0.02,"RadiusKeyFrames":[],"endLength":0,"endOffset":{"x":0,"y":0,"z":0},"gravity":{"x":0,"y":-0.05,"z":0},"force":{"x":0,"y":0,"z":0},"freezeAxis":0}`)
	colliderJSON := []byte(`{"version":1000,"StatusJsonStrList":[],"limbEnableList":[]}`)

	tests := []struct {
		name      string
		extension string
		wire      []byte
	}{
		{name: "dbconf direct Unity JSON", extension: ".dbconf", wire: dynamicBoneJSON},
		{name: "dbcol direct Unity JSON", extension: ".dbcol", wire: colliderJSON},
		{name: "dbconf Unity JSON with UTF-8 BOM", extension: ".dbconf", wire: append([]byte{0xef, 0xbb, 0xbf}, dynamicBoneJSON...)},
		{name: "dslcol BinaryWriter string Unity JSON", extension: ".dslcol", wire: appendDotNetStringForTest(nil, colliderJSON)},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeKCESPayload(test.wire, test.extension)
			if err == nil {
				t.Fatal("DecodeKCESPayload() accepted an ExportCM sidecar")
			}
			if !strings.Contains(err.Error(), "ExportCM") {
				t.Fatalf("DecodeKCESPayload() error = %v, want the ExportCM sidecar explanation", err)
			}
		})
	}
}

func TestKCESPayloadRootsRejectRemovedEditingEnvelope(t *testing.T) {
	t.Parallel()

	// 编辑封套连同 ExportCM 变体一起移除，编辑 JSON 的根现在就是载荷对象本身
	// The editing envelope was removed together with the ExportCM variants, so the editing JSON root is now the payload object itself
	envelopeJSON := []byte(`{
		"format":"kces-msgpack-lz4",
		"extension":".dbconf",
		"storageVariant":"int32-length-lz4-messagepack",
		"kind":"dynamic-bone-status",
		"dynamicBoneStatus":{"version":1000}
	}`)

	var status DynamicBoneStatus
	if err := json.Unmarshal(envelopeJSON, &status); err == nil {
		t.Fatal("the removed editing envelope was accepted as a DynamicBoneStatus root")
	}

	var pkg ColliderPackage
	if err := json.Unmarshal(envelopeJSON, &pkg); err == nil {
		t.Fatal("the removed editing envelope was accepted as a ColliderPackage root")
	}
}

func TestKCESPayloadDescriptorsDeclareOnlyNativeWireFormat(t *testing.T) {
	t.Parallel()

	for _, extension := range []string{
		".dbconf", ".dbcol", ".db2conf", ".dsbconf", ".dsb2conf",
		".dslconf", ".dsl2conf", ".dslcol", ".ikcol", ".ikcol.bytes", ".limbcol",
	} {
		descriptor, ok := DescribeKCESPayload(extension)
		if !ok {
			t.Fatalf("DescribeKCESPayload(%q) is not registered", extension)
		}
		if !descriptor.LengthPrefixed {
			t.Fatalf("%s does not declare the native int32 length prefix", extension)
		}
	}
}

func TestNativeMessagePackPayloadDecodesToItsDeclaredRoot(t *testing.T) {
	t.Parallel()
	wire, err := EncodeDynamicBoneStatusFile(NewDynamicBoneStatus())
	if err != nil {
		t.Fatalf("EncodeDynamicBoneStatusFile() error = %v", err)
	}
	decoded, err := DecodeKCESPayload(wire, ".dbconf")
	if err != nil {
		t.Fatalf("DecodeKCESPayload() error = %v", err)
	}
	if _, ok := decoded.(*DynamicBoneStatus); !ok {
		t.Fatalf("decoded .dbconf root type = %T, want *DynamicBoneStatus", decoded)
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
