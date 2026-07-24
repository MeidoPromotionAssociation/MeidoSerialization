package KCES

import (
	"bytes"
	"encoding/binary"
	"math"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
)

func TestGP03BridgeGameLayoutRoundTrip(t *testing.T) {
	legacy := sourceConstructedLegacyBridgePreset(t)
	current := sourceConstructedCurrentBridgePreset(t)
	input := &GP03BridgeFile{
		Signature:     GP03BridgeSignature,
		Version:       GP03BridgeVersion,
		GUID:          "12345678-1234-1234-1234-123456789abc",
		LegacyPreset:  append([]byte(nil), legacy...),
		CurrentPreset: append([]byte(nil), current...),
	}
	before := *input
	before.LegacyPreset = append([]byte(nil), input.LegacyPreset...)
	before.CurrentPreset = append([]byte(nil), input.CurrentPreset...)

	encoded, err := EncodeGP03Bridge(input)
	if err != nil {
		t.Fatalf("EncodeGP03Bridge: %v", err)
	}
	if !reflect.DeepEqual(*input, before) {
		t.Fatalf("EncodeGP03Bridge mutated caller input:\ngot =%+v\nwant=%+v", *input, before)
	}
	if !IsGP03BridgeData(encoded) {
		t.Fatal("encoded bridge was not recognized by its wire signature")
	}

	// Audit the exact BinaryWriter field order emitted by ExportCM.cs rather
	// than merely checking that our decoder accepts its own output.
	r := bytes.NewReader(encoded)
	br := stream.NewBinaryReader(r)
	signature, err := br.ReadString()
	if err != nil || signature != GP03BridgeSignature {
		t.Fatalf("wire signature=%q err=%v", signature, err)
	}
	version, err := br.ReadInt32()
	if err != nil || version != GP03BridgeVersion {
		t.Fatalf("wire version=%d err=%v", version, err)
	}
	guid, err := br.ReadString()
	if err != nil || guid != input.GUID {
		t.Fatalf("wire guid=%q err=%v", guid, err)
	}
	legacyLength, err := br.ReadInt32()
	if err != nil || int(legacyLength) != len(legacy) {
		t.Fatalf("wire legacy length=%d want=%d err=%v", legacyLength, len(legacy), err)
	}
	legacyWire := make([]byte, legacyLength)
	if _, err := r.Read(legacyWire); err != nil || !bytes.Equal(legacyWire, legacy) {
		t.Fatalf("wire legacy payload mismatch err=%v", err)
	}
	currentLength, err := br.ReadInt32()
	if err != nil || int(currentLength) != len(current) {
		t.Fatalf("wire current length=%d want=%d err=%v", currentLength, len(current), err)
	}
	currentWire := make([]byte, currentLength)
	if _, err := r.Read(currentWire); err != nil || !bytes.Equal(currentWire, current) {
		t.Fatalf("wire current payload mismatch err=%v", err)
	}
	if r.Len() != 0 {
		t.Fatalf("wire has %d trailing bytes", r.Len())
	}

	decoded, err := DecodeGP03Bridge(encoded)
	if err != nil {
		t.Fatalf("DecodeGP03Bridge: %v", err)
	}
	want := &GP03BridgeFile{
		Signature:     GP03BridgeSignature,
		Version:       GP03BridgeVersion,
		GUID:          input.GUID,
		LegacyPreset:  legacy,
		CurrentPreset: current,
	}
	if !reflect.DeepEqual(decoded, want) {
		t.Fatalf("decoded bridge mismatch:\ngot =%+v\nwant=%+v", decoded, want)
	}
}

func TestGP03BridgeRejectsMalformedLengthsTruncationAndTrailingData(t *testing.T) {
	legacy := sourceConstructedLegacyBridgePreset(t)
	current := sourceConstructedCurrentBridgePreset(t)
	valid := sourceConstructedBridgeWire(t, GP03BridgeSignature, GP03BridgeVersion, "maid-guid", int32(len(legacy)), legacy, int32(len(current)), current, nil)

	legacyLengthOffset, currentLengthOffset := bridgeLengthOffsets(t, valid)
	mutateInt32 := func(offset int, value int32) []byte {
		out := append([]byte(nil), valid...)
		binary.LittleEndian.PutUint32(out[offset:offset+4], uint32(value))
		return out
	}
	tests := map[string][]byte{
		"truncated signature":         valid[:5],
		"negative legacy length":      mutateInt32(legacyLengthOffset, -1),
		"huge legacy length":          mutateInt32(legacyLengthOffset, math.MaxInt32),
		"short legacy payload":        valid[:currentLengthOffset-1],
		"negative current length":     mutateInt32(currentLengthOffset, -1),
		"huge current length":         mutateInt32(currentLengthOffset, math.MaxInt32),
		"truncated current payload":   valid[:len(valid)-1],
		"unsupported outer signature": sourceConstructedBridgeWire(t, "NOT_GP03", GP03BridgeVersion, "maid-guid", int32(len(legacy)), legacy, int32(len(current)), current, nil),
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeGP03Bridge(data); err == nil {
				t.Fatal("malformed bridge was accepted")
			}
		})
	}
	extended := append(append([]byte(nil), valid...), 0xde, 0xad)
	if _, err := DecodeGP03Bridge(extended); err == nil || !strings.Contains(err.Error(), "trailing data") {
		t.Fatalf("trailing-data error = %v", err)
	}
}

func TestGP03BridgePreservesOpaqueEmbeddedPresetBlobs(t *testing.T) {
	legacy := sourceConstructedLegacyBridgePreset(t)
	current := sourceConstructedCurrentBridgePreset(t)
	wrongLegacy := sourceConstructedLegacyBridgePresetWithSignature(t, "NOT_A_PRESET")
	signatureOnly := sourceConstructedLegacyFields(t, func(bw *stream.BinaryWriter) error {
		return bw.WriteString("CM3D2_PRESET")
	})
	truncatedHeader := append(append([]byte(nil), signatureOnly...), 0x01, 0x02)
	negativeThumbnail := sourceConstructedLegacyFields(t, func(bw *stream.BinaryWriter) error {
		if err := bw.WriteString("CM3D2_PRESET"); err != nil {
			return err
		}
		if err := bw.WriteInt32(2001); err != nil {
			return err
		}
		if err := bw.WriteInt32(2); err != nil {
			return err
		}
		return bw.WriteInt32(-1)
	})
	zeroCountMissingExtensions := sourceConstructedLegacyFields(t, func(bw *stream.BinaryWriter) error {
		for _, write := range []func() error{
			func() error { return bw.WriteString("CM3D2_PRESET") },
			func() error { return bw.WriteInt32(2001) },
			func() error { return bw.WriteInt32(2) },
			func() error { return bw.WriteInt32(0) },
			func() error { return bw.WriteString("CM3D2_MPROP_LIST") },
			func() error { return bw.WriteInt32(2001) },
			func() error { return bw.WriteInt32(0) },
		} {
			if err := write(); err != nil {
				return err
			}
		}
		return nil
	})
	wrongCurrent := append([]byte(nil), current...)
	wrongCurrent[0] ^= 0xff

	tests := []struct {
		name    string
		legacy  []byte
		current []byte
	}{
		{name: "empty legacy", current: current},
		{name: "wrong legacy signature", legacy: wrongLegacy, current: current},
		{name: "legacy signature only", legacy: signatureOnly, current: current},
		{name: "legacy truncated header", legacy: truncatedHeader, current: current},
		{name: "legacy negative thumbnail", legacy: negativeThumbnail, current: current},
		{name: "legacy zero count missing extensions", legacy: zeroCountMissingExtensions, current: current},
		{name: "empty current", legacy: legacy},
		{name: "wrong current signature", legacy: legacy, current: wrongCurrent},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wire := sourceConstructedBridgeWire(t, GP03BridgeSignature, GP03BridgeVersion, "maid-guid", int32(len(test.legacy)), test.legacy, int32(len(test.current)), test.current, nil)
			got, err := DecodeGP03Bridge(wire)
			if err != nil {
				t.Fatalf("DecodeGP03Bridge: %v", err)
			}
			if !bytes.Equal(got.LegacyPreset, test.legacy) || !bytes.Equal(got.CurrentPreset, test.current) {
				t.Fatalf("opaque blobs changed: got legacy=%x current=%x", got.LegacyPreset, got.CurrentPreset)
			}
		})
	}
}

func TestGP03BridgeEncodeValidationAndCallerOwnership(t *testing.T) {
	legacy := sourceConstructedLegacyBridgePreset(t)
	current := sourceConstructedCurrentBridgePreset(t)
	invalidUTF8 := string([]byte{0xff})
	tooLargeGUID := strings.Repeat("g", 10*1024*1024+1)
	tests := []struct {
		name  string
		value *GP03BridgeFile
	}{
		{name: "nil"},
		{name: "signature", value: &GP03BridgeFile{Signature: "future", GUID: "g", LegacyPreset: legacy, CurrentPreset: current}},
		{name: "missing signature", value: &GP03BridgeFile{Version: GP03BridgeVersion, GUID: "g", LegacyPreset: legacy, CurrentPreset: current}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := EncodeGP03Bridge(test.value); err == nil {
				t.Fatal("invalid bridge was accepted")
			}
		})
	}
	largeWire, err := EncodeGP03Bridge(&GP03BridgeFile{Signature: GP03BridgeSignature, Version: GP03BridgeVersion, GUID: tooLargeGUID})
	if err != nil {
		t.Fatalf("GUID above former implementation limit was rejected: %v", err)
	}
	largeDecoded, err := DecodeGP03Bridge(largeWire)
	if err != nil || largeDecoded.GUID != tooLargeGUID {
		t.Fatalf("large GUID round trip failed: decoded=%v err=%v", largeDecoded != nil, err)
	}

	input := &GP03BridgeFile{Signature: GP03BridgeSignature, Version: GP03BridgeVersion, GUID: "ownership", LegacyPreset: legacy, CurrentPreset: current}
	encoded, err := EncodeGP03Bridge(input)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeGP03Bridge(encoded)
	if err != nil {
		t.Fatal(err)
	}
	decoded.LegacyPreset[0] ^= 0xff
	decoded.CurrentPreset[0] ^= 0xff
	if !bytes.Equal(input.LegacyPreset, legacy) || !bytes.Equal(input.CurrentPreset, current) {
		t.Fatal("decoded payload aliases caller-owned input slices")
	}
	invalidInput := &GP03BridgeFile{Signature: GP03BridgeSignature, Version: GP03BridgeVersion, GUID: invalidUTF8, LegacyPreset: legacy, CurrentPreset: current}
	invalidWire, err := EncodeGP03Bridge(invalidInput)
	if err != nil {
		t.Fatalf("invalid UTF-8 BinaryWriter string was rejected: %v", err)
	}
	invalidDecoded, err := DecodeGP03Bridge(invalidWire)
	if err != nil || invalidDecoded.GUID != invalidUTF8 {
		t.Fatalf("invalid UTF-8 GUID changed: decoded=%+v err=%v", invalidDecoded, err)
	}
}

func TestGP03BridgeCOM3D2V2000RoundTripAndPayloadRules(t *testing.T) {
	current := sourceConstructedCurrentBridgePreset(t)
	for _, test := range []struct {
		name    string
		current []byte
	}{
		{name: "with retained KCES preset", current: current},
		{name: "empty current preset"},
	} {
		t.Run(test.name, func(t *testing.T) {
			wire := sourceConstructedBridgeWire(t, GP03BridgeSignature, GP03BridgeCOM3D2Version, "com3d2-reverse-guid", 0, nil, int32(len(test.current)), test.current, nil)
			decoded, err := DecodeGP03Bridge(wire)
			if err != nil {
				t.Fatalf("DecodeGP03Bridge(v2000): %v", err)
			}
			if decoded.Version != GP03BridgeCOM3D2Version || decoded.GUID != "com3d2-reverse-guid" || len(decoded.LegacyPreset) != 0 || !bytes.Equal(decoded.CurrentPreset, test.current) {
				t.Fatalf("unexpected v2000 round trip: %+v", decoded)
			}
			encoded, err := EncodeGP03Bridge(decoded)
			if err != nil {
				t.Fatalf("EncodeGP03Bridge(v2000): %v", err)
			}
			if !bytes.Equal(encoded, wire) {
				t.Fatal("v2000 source-constructed wire was not preserved")
			}
		})
	}

	legacy := sourceConstructedLegacyBridgePreset(t)
	wire := sourceConstructedBridgeWire(t, GP03BridgeSignature, GP03BridgeCOM3D2Version, "g", int32(len(legacy)), legacy, int32(len(current)), current, nil)
	if _, err := DecodeGP03Bridge(wire); err == nil || !strings.Contains(err.Error(), "legacy preset block must be empty") {
		t.Fatalf("v2000 non-empty legacy error = %v", err)
	}
}

func TestGP03BridgeLegacyOpaqueLengthFieldsAreBoundedAndConsumed(t *testing.T) {
	thumbnail := []byte{1, 2, 3}
	partsColor := []byte{4, 5}
	crcPreset := []byte{6, 7, 8, 9}
	legacy := sourceConstructedLegacyFields(t, func(bw *stream.BinaryWriter) error {
		for _, write := range []func() error{
			func() error { return bw.WriteString("CM3D2_PRESET") },
			func() error { return bw.WriteInt32(2001) },
			func() error { return bw.WriteInt32(2) },
			func() error { return bw.WriteInt32(int32(len(thumbnail))) },
			func() error { return bw.WriteBytes(thumbnail) },
			func() error { return bw.WriteString("CM3D2_MPROP_LIST") },
			func() error { return bw.WriteInt32(2001) },
			func() error { return bw.WriteInt32(0) },
			func() error { return bw.WriteInt32(0) },
			func() error { return bw.WriteInt32(int32(len(partsColor))) },
			func() error { return bw.WriteBytes(partsColor) },
			func() error { return bw.WriteInt32(int32(len(crcPreset))) },
			func() error { return bw.WriteBytes(crcPreset) },
		} {
			if err := write(); err != nil {
				return err
			}
		}
		return nil
	})
	current := sourceConstructedCurrentBridgePreset(t)
	encoded, err := EncodeGP03Bridge(&GP03BridgeFile{Signature: GP03BridgeSignature, Version: GP03BridgeVersion, GUID: "bounded", LegacyPreset: legacy, CurrentPreset: current})
	if err != nil {
		t.Fatalf("valid bounded opaque fields were rejected: %v", err)
	}
	decoded, err := DecodeGP03Bridge(encoded)
	if err != nil || !bytes.Equal(decoded.LegacyPreset, legacy) {
		t.Fatalf("bounded opaque fields round trip: decoded=%+v err=%v", decoded, err)
	}
}

// sourceConstructedLegacyBridgePreset mirrors ExportCM.GetPresetBinForCOM3D2
// with an empty property set. The final three zero Int32 values are the extra
// mprop/color/current-preset fields consumed by COM3D2_5's DeserializePropPre.
func sourceConstructedLegacyBridgePreset(t *testing.T) []byte {
	t.Helper()
	return sourceConstructedLegacyBridgePresetWithSignature(t, "CM3D2_PRESET")
}

func sourceConstructedLegacyBridgePresetWithSignature(t *testing.T, signature string) []byte {
	t.Helper()
	var out bytes.Buffer
	bw := stream.NewBinaryWriter(&out)
	for _, write := range []func() error{
		func() error { return bw.WriteString(signature) },
		func() error { return bw.WriteInt32(2001) },
		func() error { return bw.WriteInt32(2) },
		func() error { return bw.WriteInt32(0) },
		func() error { return bw.WriteString("CM3D2_MPROP_LIST") },
		func() error { return bw.WriteInt32(2001) },
		func() error { return bw.WriteInt32(1) },
		func() error { return bw.WriteString("body") },
		func() error { return bw.WriteString("CM3D2_MPROP") },
		func() error { return bw.WriteInt32(2001) },
		func() error { return bw.WriteInt32(0) },
		func() error { return bw.WriteString("body") },
		func() error { return bw.WriteInt32(3) },
		func() error { return bw.WriteInt32(0) },
		func() error { return bw.WriteInt32(100) },
		func() error { return bw.WriteInt32(100) },
		func() error { return bw.WriteInt32(0) },
		func() error { return bw.WriteString("crc_body.menu") },
		func() error { return bw.WriteInt32(0) },
		func() error { return bw.WriteBool(true) },
		func() error { return bw.WriteInt32(100) },
		func() error { return bw.WriteInt32(0) },
		func() error { return bw.WriteInt32(0) },
		func() error { return bw.WriteInt32(0) },
		func() error { return bw.WriteInt32(0) },
		func() error { return bw.WriteInt32(0) },
		func() error { return bw.WriteInt32(0) },
		func() error { return bw.WriteBool(true) },
		func() error { return bw.WriteInt32(0) },
		func() error { return bw.WriteInt32(0) },
		func() error { return bw.WriteInt32(0) },
	} {
		if err := write(); err != nil {
			t.Fatal(err)
		}
	}
	return out.Bytes()
}

func sourceConstructedCurrentBridgePreset(t *testing.T) []byte {
	t.Helper()
	data, err := EncodeKCESPreset(&KCESPreset{
		ContainerVersion: kcesPresetVersion,
		Thumbnail:        []byte{0x89, 'P', 'N', 'G'},
		MaidData:         validKCESPresetCoreForTest(t),
	})
	if err != nil {
		t.Fatalf("construct current preset: %v", err)
	}
	return data
}

func sourceConstructedLegacyFields(t *testing.T, write func(*stream.BinaryWriter) error) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := write(stream.NewBinaryWriter(&out)); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func sourceConstructedBridgeWire(t *testing.T, signature string, version int32, guid string, legacyLength int32, legacy []byte, currentLength int32, current, trailing []byte) []byte {
	t.Helper()
	if !utf8.ValidString(guid) {
		t.Fatal("test helper received invalid UTF-8")
	}
	var out bytes.Buffer
	bw := stream.NewBinaryWriter(&out)
	for _, write := range []func() error{
		func() error { return bw.WriteString(signature) },
		func() error { return bw.WriteInt32(version) },
		func() error { return bw.WriteString(guid) },
		func() error { return bw.WriteInt32(legacyLength) },
		func() error { return bw.WriteBytes(legacy) },
		func() error { return bw.WriteInt32(currentLength) },
		func() error { return bw.WriteBytes(current) },
		func() error { return bw.WriteBytes(trailing) },
	} {
		if err := write(); err != nil {
			t.Fatal(err)
		}
	}
	return out.Bytes()
}

func bridgeLengthOffsets(t *testing.T, data []byte) (legacy, current int) {
	t.Helper()
	r := bytes.NewReader(data)
	br := stream.NewBinaryReader(r)
	if _, err := br.ReadString(); err != nil {
		t.Fatal(err)
	}
	if _, err := br.ReadInt32(); err != nil {
		t.Fatal(err)
	}
	if _, err := br.ReadString(); err != nil {
		t.Fatal(err)
	}
	legacy = len(data) - r.Len()
	legacyLength, err := br.ReadInt32()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Seek(int64(legacyLength), 1); err != nil {
		t.Fatal(err)
	}
	current = len(data) - r.Len()
	return legacy, current
}
