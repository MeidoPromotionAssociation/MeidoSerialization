package KCES

import (
	"bytes"
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
)

func savedAttachString(value string) *string { return &value }

func TestSavedAttachHierarchyOrderToleratesMapEdits(t *testing.T) {
	values := map[string]SavedAttachPosRotScale{
		"aBone": {},
		"bBone": {},
		"cBone": {},
	}
	var wire bytes.Buffer
	if err := writeSavedAttachHierarchy(stream.NewBinaryWriter(&wire), values, []string{"bBone", "removed"}, "item"); err != nil {
		t.Fatalf("writeSavedAttachHierarchy: %v", err)
	}
	remaining := bytes.NewReader(wire.Bytes())
	decoded, order, err := readSavedAttachHierarchy(stream.NewBinaryReader(remaining), remaining, "item")
	if err != nil {
		t.Fatalf("readSavedAttachHierarchy: %v", err)
	}
	if !reflect.DeepEqual(order, []string{"bBone", "aBone", "cBone"}) {
		t.Fatalf("hierarchy order = %#v, want preserved live keys followed by sorted additions", order)
	}
	if !reflect.DeepEqual(decoded, values) {
		t.Fatalf("hierarchy values changed: got %#v want %#v", decoded, values)
	}
}

func TestSavedAttachCurrentRoundTripPreservesVersionsAndDoesNotMutate(t *testing.T) {
	value := &SavedAttachFile{
		Signature: SavedAttachSignature,
		Version:   SavedAttachFileVersion,
		Items: []SavedAttachData{
			{
				Version:                SavedAttachRecordVersion,
				ExplicitVersion:        true,
				PartName:               savedAttachString("HatPoint"),
				Enabled:                true,
				MyRID:                  math.MaxUint64,
				MySlotID:               "accHat",
				TargetRID:              42,
				TargetSlotID:           "body",
				TargetSlotNo:           math.MaxInt32,
				TargetAttachPointName:  savedAttachString("Bip01 Head"),
				TargetVertexCount:      math.MaxInt32,
				TargetVertexIndex:      math.MinInt32,
				NewAttachVertexIndices: []int32{math.MinInt32, 0, math.MaxInt32},
				PRS2: &SavedAttachPosRotScale{
					Position: Vector3{X: 1, Y: 2, Z: 3},
					Scale:    Vector3{X: 4, Y: 5, Z: 6},
					Rotation: Vector4{X: 7, Y: 8, Z: 9, W: 10},
				},
				PRS3: &SavedAttachPosRotScale{
					Position: Vector3{X: -1, Y: -2, Z: -3},
					Scale:    Vector3{X: 1, Y: 1, Z: 1},
					Rotation: Vector4{W: 1},
				},
				BoneAttachedHierarchy: map[string]SavedAttachPosRotScale{
					"zBone": {Position: Vector3{Z: 3}},
					"aBone": {Rotation: Vector4{W: 1}},
				},
				BoneAttachEdited: true,
			},
			{
				Version:                SavedAttachRecordVersion,
				ExplicitVersion:        true,
				PartName:               nil,
				MySlotID:               "-1",
				TargetSlotID:           "end",
				NewAttachVertexIndices: []int32{},
				BoneAttachedHierarchy:  map[string]SavedAttachPosRotScale{},
			},
		},
	}
	before, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := EncodeSavedAttach(value)
	if err != nil {
		t.Fatalf("EncodeSavedAttach: %v", err)
	}
	after, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatalf("EncodeSavedAttach mutated caller:\nbefore=%s\nafter =%s", before, after)
	}
	encodedAgain, err := EncodeSavedAttach(value)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encodedAgain, encoded) {
		t.Fatal("EncodeSavedAttach is not deterministic")
	}

	decoded, err := DecodeSavedAttach(encoded)
	if err != nil {
		t.Fatalf("DecodeSavedAttach: %v", err)
	}
	if decoded.Format != KCESSavedAttachFormat || decoded.Signature != SavedAttachSignature || decoded.Version != SavedAttachFileVersion {
		t.Fatalf("unexpected envelope: %+v", decoded)
	}
	if len(decoded.Items) != 2 || decoded.Items[0].Version != SavedAttachRecordVersion || decoded.Items[1].Version != SavedAttachRecordVersion {
		t.Fatalf("unexpected decoded versions/items: %+v", decoded.Items)
	}
	if decoded.Items[0].PartName == nil || *decoded.Items[0].PartName != "HatPoint" || decoded.Items[0].MyRID != math.MaxUint64 {
		t.Fatalf("unexpected first item: %+v", decoded.Items[0])
	}
	if decoded.Items[1].PartName != nil || decoded.Items[1].NewAttachVertexIndices == nil || decoded.Items[1].BoneAttachedHierarchy == nil {
		t.Fatalf("nullable/empty carriers were not preserved: %+v", decoded.Items[1])
	}
	if !reflect.DeepEqual(decoded.Items[0].PRS2, value.Items[0].PRS2) || !reflect.DeepEqual(decoded.Items[0].BoneAttachedHierarchy, value.Items[0].BoneAttachedHierarchy) {
		t.Fatalf("pose hierarchy changed: got=%+v want=%+v", decoded.Items[0], value.Items[0])
	}

	reencoded, err := EncodeSavedAttach(decoded)
	if err != nil {
		t.Fatalf("re-encode decoded: %v", err)
	}
	if !bytes.Equal(reencoded, encoded) {
		t.Fatal("decode/re-encode changed the current saved-attach wire")
	}
}

func TestSavedAttachLegacy2000RecordLayout(t *testing.T) {
	legacy := &SavedAttachFile{
		Signature: SavedAttachSignature,
		Version:   SavedAttachFileVersion,
		Items: []SavedAttachData{{
			Version:           SavedAttachFileVersion,
			PartName:          savedAttachString("legacy-part"),
			Enabled:           true,
			MySlotID:          "accHat",
			TargetSlotID:      "body",
			TargetSlotNo:      0, // absent from the legacy wire
			TargetVertexCount: 10,
			TargetVertexIndex: 3,
			BoneAttachEdited:  true,
		}},
	}
	encoded, err := EncodeSavedAttach(legacy)
	if err != nil {
		t.Fatal(err)
	}

	// Independently inspect the start of the first record: the 2000 layout has
	// no empty-string/version sentinel and starts directly with partName.
	r := bytes.NewReader(encoded)
	br := stream.NewBinaryReader(r)
	if signature, _ := br.ReadString(); signature != SavedAttachSignature {
		t.Fatalf("signature=%q", signature)
	}
	_, _ = br.ReadInt32()
	_, _ = br.ReadInt32()
	present, err := br.ReadBool()
	if err != nil || !present {
		t.Fatalf("legacy partName presence=%v err=%v", present, err)
	}
	if partName, err := br.ReadString(); err != nil || partName != "legacy-part" {
		t.Fatalf("legacy first string=%q err=%v", partName, err)
	}

	decoded, err := DecodeSavedAttach(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Items[0].Version != SavedAttachFileVersion || decoded.Items[0].TargetSlotNo != 0 {
		t.Fatalf("unexpected legacy decode: %+v", decoded.Items[0])
	}
	reencoded, err := EncodeSavedAttach(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(reencoded, encoded) {
		t.Fatal("legacy saved-attach wire changed after round-trip")
	}

	// Deserialize treats both null/empty first strings as an explicit-version
	// sentinel. Although the game's current writer does not emit this variant,
	// an explicit 2000 record follows the same legacy field layout and is valid
	// for the game reader.
	versionedLegacy := manualCurrentSavedAttach(t, "accHat", "body", SavedAttachFileVersion, nil, false)
	decodedVersioned, err := DecodeSavedAttach(versionedLegacy)
	if err != nil {
		t.Fatalf("decode explicitly versioned legacy record: %v", err)
	}
	if decodedVersioned.Items[0].Version != SavedAttachFileVersion || decodedVersioned.Items[0].TargetSlotNo != 0 {
		t.Fatalf("unexpected explicitly versioned legacy record: %+v", decodedVersioned.Items[0])
	}
}

func TestSavedAttachRejectsMalformedAndGameIncompatibleData(t *testing.T) {
	valid, err := EncodeSavedAttach(&SavedAttachFile{Signature: SavedAttachSignature, Version: SavedAttachFileVersion, Items: []SavedAttachData{{
		Version:         SavedAttachRecordVersion,
		ExplicitVersion: true,
		PartName:        savedAttachString("part"),
		MySlotID:        "accHat",
		TargetSlotID:    "body",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	for cut := 0; cut < len(valid); cut++ {
		if _, err := DecodeSavedAttach(valid[:cut]); err == nil {
			t.Fatalf("truncated wire at %d/%d bytes was accepted", cut, len(valid))
		}
	}
	extended := append(append([]byte(nil), valid...), 0xde, 0xad)
	decodedTrailing, err := DecodeSavedAttach(extended)
	if err != nil || !bytes.Equal(decodedTrailing.TrailingData, []byte{0xde, 0xad}) {
		t.Fatalf("trailing bytes were not preserved: decoded=%+v err=%v", decodedTrailing, err)
	}
	reencodedTrailing, err := EncodeSavedAttach(decodedTrailing)
	if err != nil || !bytes.Equal(reencodedTrailing, extended) {
		t.Fatalf("trailing-byte round trip=%x err=%v want=%x", reencodedTrailing, err, extended)
	}
	negativeCountWire := append(manualSavedAttachHeader(t, SavedAttachSignature, SavedAttachFileVersion, -1), 1, 2, 3)
	if _, err := DecodeSavedAttach(negativeCountWire); err == nil || !strings.Contains(err.Error(), "negative") {
		t.Fatalf("negative count error=%v", err)
	}

	for _, test := range []struct {
		name string
		wire []byte
		want string
	}{
		{name: "bad signature", wire: manualSavedAttachHeader(t, "WRONG", SavedAttachFileVersion, 0), want: "signature"},
		{name: "impossible count", wire: manualSavedAttachHeader(t, SavedAttachSignature, SavedAttachFileVersion, math.MaxInt32), want: "cannot fit"},
		{name: "count below minimum record bytes", wire: append(manualSavedAttachHeader(t, SavedAttachSignature, SavedAttachFileVersion, 2), make([]byte, 2)...), want: "cannot fit"},
		{name: "impossible index count", wire: manualCurrentSavedAttach(t, "accHat", "body", SavedAttachRecordVersion, func(bw *stream.BinaryWriter) {
			_ = bw.WriteBool(true)
			_ = bw.WriteInt32(math.MaxInt32)
		}, false), want: "cannot fit"},
		{name: "duplicate hierarchy", wire: manualCurrentSavedAttach(t, "accHat", "body", SavedAttachRecordVersion, nil, true), want: "duplicate"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := DecodeSavedAttach(test.wire)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("DecodeSavedAttach error=%v, want %q", err, test.want)
			}
		})
	}

	// Both version integers are data fields ignored/gated by the game reader;
	// the serializer must retain them instead of silently upgrading to 2000 or
	// 2001. A record >=2001 uses the same targetSlotNo-bearing layout.
	outerLegacy := manualSavedAttachHeader(t, SavedAttachSignature, 1999, 0)
	decodedOuterLegacy, err := DecodeSavedAttach(outerLegacy)
	if err != nil || decodedOuterLegacy.Version != 1999 {
		t.Fatalf("outer version was not preserved: decoded=%+v err=%v", decodedOuterLegacy, err)
	}
	futureRecord := manualCurrentSavedAttach(t, "accHat", "body", 2002, nil, false)
	decodedFuture, err := DecodeSavedAttach(futureRecord)
	if err != nil || decodedFuture.Items[0].Version != 2002 || !decodedFuture.Items[0].ExplicitVersion {
		t.Fatalf("record version was not preserved: decoded=%+v err=%v", decodedFuture, err)
	}
	reencodedFuture, err := EncodeSavedAttach(decodedFuture)
	if err != nil || !bytes.Equal(reencodedFuture, futureRecord) {
		t.Fatalf("future-layout record wire changed: equal=%v err=%v", bytes.Equal(reencodedFuture, futureRecord), err)
	}

	badSlot := &SavedAttachFile{Signature: SavedAttachSignature, Items: []SavedAttachData{{PartName: savedAttachString("part"), MySlotID: "acchat", TargetSlotID: "body"}}}
	wire, err := EncodeSavedAttach(badSlot)
	if err != nil {
		t.Fatalf("opaque SlotID encode error=%v", err)
	}
	decodedSlot, err := DecodeSavedAttach(wire)
	if err != nil || decodedSlot.Items[0].MySlotID != "acchat" {
		t.Fatalf("opaque SlotID round trip=%+v err=%v", decodedSlot, err)
	}
	for _, partName := range []*string{nil, savedAttachString("")} {
		versionedLegacy := &SavedAttachFile{Signature: SavedAttachSignature, Items: []SavedAttachData{{Version: 2000, PartName: partName, MySlotID: "body", TargetSlotID: "body"}}}
		wire, err := EncodeSavedAttach(versionedLegacy)
		if err != nil {
			t.Fatalf("encode explicitly versioned legacy partName=%v: %v", partName, err)
		}
		decoded, err := DecodeSavedAttach(wire)
		if err != nil || decoded.Items[0].Version != 2000 || !reflect.DeepEqual(decoded.Items[0].PartName, partName) {
			t.Fatalf("explicitly versioned legacy round-trip: decoded=%+v err=%v", decoded, err)
		}
	}
	wrongFormat := &SavedAttachFile{Format: "future", Items: []SavedAttachData{}}
	if _, err := EncodeSavedAttach(wrongFormat); err == nil || !strings.Contains(err.Error(), "format") {
		t.Fatalf("wrong format error=%v", err)
	}
	invalidUTF8 := string([]byte{0xff})
	badText := &SavedAttachFile{Signature: SavedAttachSignature, Items: []SavedAttachData{{PartName: &invalidUTF8, MySlotID: "body", TargetSlotID: "body"}}}
	invalidWire, err := EncodeSavedAttach(badText)
	if err != nil {
		t.Fatalf("invalid UTF-8 wire string was rejected: %v", err)
	}
	invalidDecoded, err := DecodeSavedAttach(invalidWire)
	if err != nil || invalidDecoded.Items[0].PartName == nil || *invalidDecoded.Items[0].PartName != invalidUTF8 {
		t.Fatalf("invalid UTF-8 wire string changed: decoded=%+v err=%v", invalidDecoded, err)
	}
	longString := strings.Repeat("x", (10<<20)+1)
	longText := &SavedAttachFile{Signature: SavedAttachSignature, Items: []SavedAttachData{{PartName: &longString, MySlotID: "body", TargetSlotID: "body"}}}
	longWire, err := EncodeSavedAttach(longText)
	if err != nil {
		t.Fatalf("valid string above the former implementation limit was rejected: %v", err)
	}
	longDecoded, err := DecodeSavedAttach(longWire)
	if err != nil || longDecoded.Items[0].PartName == nil || *longDecoded.Items[0].PartName != longString {
		t.Fatalf("long string round trip failed: decoded=%v err=%v", longDecoded != nil, err)
	}

	unrepresentable := &SavedAttachFile{Signature: SavedAttachSignature, Items: []SavedAttachData{{
		Version: 2000, PartName: savedAttachString("part"), MySlotID: "body", TargetSlotID: "body", TargetSlotNo: 1,
	}}}
	if _, err := EncodeSavedAttach(unrepresentable); err == nil || !strings.Contains(err.Error(), "targetSlotNo") {
		t.Fatalf("legacy record silently discarded targetSlotNo: %v", err)
	}
}

func TestSavedAttachRejectsNegativeBoneHierarchyCount(t *testing.T) {
	value := NewSavedAttachFile()
	value.Items = []SavedAttachData{{
		Version:               SavedAttachRecordVersion,
		ExplicitVersion:       true,
		MySlotID:              "body",
		TargetSlotID:          "body",
		BoneAttachedHierarchy: map[string]SavedAttachPosRotScale{},
		BoneAttachEdited:      true,
	}}

	wire, err := EncodeSavedAttach(value)
	if err != nil {
		t.Fatalf("EncodeSavedAttach: %v", err)
	}
	// The final fields are dictionary-present, int32 count, then
	// boneAttachEdited. Replace the empty dictionary count with -1.
	copy(wire[len(wire)-5:len(wire)-1], []byte{0xff, 0xff, 0xff, 0xff})
	if _, err := DecodeSavedAttach(wire); err == nil || !strings.Contains(err.Error(), "negative") {
		t.Fatalf("negative hierarchy count error = %v", err)
	}
}

func TestSavedAttachSlotIDStringsAreOpaque(t *testing.T) {
	for _, value := range []string{"", "body", "accHat", "none", "accAcc73", "Body", "acchat", "2147483648", "body,1", "future-slot"} {
		input := &SavedAttachFile{Signature: SavedAttachSignature, Items: []SavedAttachData{{PartName: savedAttachString("part"), MySlotID: value, TargetSlotID: value}}}
		wire, err := EncodeSavedAttach(input)
		if err != nil {
			t.Fatalf("EncodeSavedAttach(%q): %v", value, err)
		}
		got, err := DecodeSavedAttach(wire)
		if err != nil || got.Items[0].MySlotID != value || got.Items[0].TargetSlotID != value {
			t.Fatalf("SlotID %q round trip=%+v err=%v", value, got, err)
		}
	}
}

func manualSavedAttachHeader(t *testing.T, signature string, version, count int32) []byte {
	t.Helper()
	var out bytes.Buffer
	bw := stream.NewBinaryWriter(&out)
	if err := bw.WriteString(signature); err != nil {
		t.Fatal(err)
	}
	if err := bw.WriteInt32(version); err != nil {
		t.Fatal(err)
	}
	if err := bw.WriteInt32(count); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// manualCurrentSavedAttach constructs a game-shaped record independently of
// EncodeSavedAttach. writeArrayOverride, when non-nil, writes the array field
// and intentionally stops the record so impossible-count handling can be
// tested before any allocation.
func manualCurrentSavedAttach(t *testing.T, mySlot, targetSlot string, recordVersion int32, writeArrayOverride func(*stream.BinaryWriter), duplicateHierarchy bool) []byte {
	t.Helper()
	out := bytes.NewBuffer(manualSavedAttachHeader(t, SavedAttachSignature, SavedAttachFileVersion, 1))
	bw := stream.NewBinaryWriter(out)
	mustWrite := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(bw.WriteBool(true))
	mustWrite(bw.WriteString(""))
	mustWrite(bw.WriteInt32(recordVersion))
	mustWrite(bw.WriteBool(true))
	mustWrite(bw.WriteString("part"))
	mustWrite(bw.WriteBool(true))
	mustWrite(bw.WriteUInt64(1))
	mustWrite(bw.WriteString(mySlot))
	mustWrite(bw.WriteUInt64(2))
	mustWrite(bw.WriteString(targetSlot))
	if recordVersion >= SavedAttachRecordVersion {
		mustWrite(bw.WriteInt32(0))
	}
	mustWrite(bw.WriteBool(false)) // nullable target attach-point name
	mustWrite(bw.WriteInt32(10))
	mustWrite(bw.WriteInt32(2))
	if writeArrayOverride != nil {
		writeArrayOverride(bw)
		return out.Bytes()
	}
	mustWrite(bw.WriteBool(false)) // nil newAttachVertexIndices
	mustWrite(bw.WriteBool(false)) // nil prs2
	mustWrite(bw.WriteBool(false)) // nil prs3
	if duplicateHierarchy {
		mustWrite(bw.WriteBool(true))
		mustWrite(bw.WriteInt32(2))
		for i := 0; i < 2; i++ {
			mustWrite(bw.WriteString("same"))
			mustWrite(bw.WriteFloat3([3]float32{}))
			mustWrite(bw.WriteFloat3([3]float32{}))
			mustWrite(bw.WriteFloat4([4]float32{}))
		}
	} else {
		mustWrite(bw.WriteBool(false))
	}
	mustWrite(bw.WriteBool(false))
	return out.Bytes()
}
