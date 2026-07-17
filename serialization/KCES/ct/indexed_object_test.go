package ct

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/ugorji/go/codec"
)

type indexedObjectTestChild struct {
	_struct               struct{} `codec:",toarray"`
	IndexedObjectMetadata `codec:"-"`
	Number                int    `json:"number"`
	Text                  string `json:"text"`
}

func (v indexedObjectTestChild) CodecEncodeSelf(e *codec.Encoder) {
	EncodeIndexedObjectSelf(e, &v)
}

func (v *indexedObjectTestChild) CodecDecodeSelf(d *codec.Decoder) {
	DecodeIndexedObjectSelf(d, v)
}

type indexedObjectTestParent struct {
	_struct               struct{} `codec:",toarray"`
	IndexedObjectMetadata `codec:"-"`
	Children              []indexedObjectTestChild          `json:"children"`
	ByName                map[string]indexedObjectTestChild `json:"byName"`
}

type IndexedObjectTestInlineBase struct {
	Version int    `json:"version"`
	Name    string `json:"name"`
}

type indexedObjectTestInline struct {
	_struct                     struct{} `codec:",toarray"`
	IndexedObjectMetadata       `codec:"-"`
	IndexedObjectTestInlineBase `codec:",inline"`
	Enabled                     bool `json:"enabled"`
}

type indexedObjectTestSingles struct {
	_struct               struct{} `codec:",toarray"`
	IndexedObjectMetadata `codec:"-"`
	Value                 float32   `json:"value"`
	Values                []float32 `json:"values"`
}

func (v indexedObjectTestSingles) CodecEncodeSelf(e *codec.Encoder) {
	EncodeIndexedObjectSelf(e, &v)
}

func (v *indexedObjectTestSingles) CodecDecodeSelf(d *codec.Decoder) {
	DecodeIndexedObjectSelf(d, v)
}

func (v indexedObjectTestInline) CodecEncodeSelf(e *codec.Encoder) {
	EncodeIndexedObjectSelf(e, &v)
}

func (v *indexedObjectTestInline) CodecDecodeSelf(d *codec.Decoder) {
	DecodeIndexedObjectSelf(d, v)
}

func (v indexedObjectTestParent) CodecEncodeSelf(e *codec.Encoder) {
	EncodeIndexedObjectSelf(e, &v)
}

func (v *indexedObjectTestParent) CodecDecodeSelf(d *codec.Decoder) {
	DecodeIndexedObjectSelf(d, v)
}

func TestIndexedObjectSelferPreservesNestedWidthsAndFutureSlots(t *testing.T) {
	future := codec.Raw{0xd4, 0x2a, 0x7f}
	wire, err := encodeMsgpackAllowRaw([]interface{}{
		[]interface{}{
			[]interface{}{int64(7)},
			[]interface{}{int64(8), "full", future},
		},
		map[string]interface{}{
			"short": []interface{}{int64(9)},
		},
	})
	if err != nil {
		t.Fatalf("build test wire: %v", err)
	}

	var decoded indexedObjectTestParent
	if err := DecodeMsgpack(wire, &decoded); err != nil {
		t.Fatalf("DecodeMsgpack: %v", err)
	}
	if len(decoded.Children) != 2 {
		t.Fatalf("children length = %d, want 2", len(decoded.Children))
	}
	if decoded.Children[0].FieldCount == nil || *decoded.Children[0].FieldCount != 1 {
		t.Fatalf("short child fieldCount = %v, want 1", decoded.Children[0].FieldCount)
	}
	if decoded.Children[0].Text != "" {
		t.Fatalf("short child omitted text = %q, want wire zero", decoded.Children[0].Text)
	}
	if decoded.Children[1].FieldCount == nil || *decoded.Children[1].FieldCount != 3 {
		t.Fatalf("future child fieldCount = %v, want 3", decoded.Children[1].FieldCount)
	}
	if len(decoded.Children[1].FutureSlots) != 1 || !bytes.Equal(decoded.Children[1].FutureSlots[0], future) {
		t.Fatalf("future child slots = % x, want % x", decoded.Children[1].FutureSlots, future)
	}
	mapChild, ok := decoded.ByName["short"]
	if !ok || mapChild.FieldCount == nil || *mapChild.FieldCount != 1 {
		t.Fatalf("map child did not retain short width: %#v", mapChild)
	}

	reencoded, err := EncodeIndexedMsgpack(&decoded)
	if err != nil {
		t.Fatalf("EncodeIndexedMsgpack: %v", err)
	}
	assertIndexedObjectTestWire(t, reencoded, future)
}

func TestIndexedObjectSelferRejectsSilentShortFieldDiscard(t *testing.T) {
	count := 1
	value := indexedObjectTestChild{
		IndexedObjectMetadata: IndexedObjectMetadata{FieldCount: &count},
		Number:                1,
		Text:                  "edited missing slot",
	}
	if _, err := EncodeIndexedMsgpack(&value); err == nil || !strings.Contains(err.Error(), "would discard text") {
		t.Fatalf("short-width populated field error = %v", err)
	}

	count = 2
	if _, err := EncodeIndexedMsgpack(&value); err != nil {
		t.Fatalf("explicitly extending fieldCount should encode: %v", err)
	}
}

func TestIndexedObjectSelferValidatesFutureRawValue(t *testing.T) {
	count := 3
	value := indexedObjectTestChild{
		IndexedObjectMetadata: IndexedObjectMetadata{
			FieldCount:  &count,
			FutureSlots: [][]byte{{0xc1}},
		},
	}
	if _, err := EncodeIndexedMsgpack(&value); err == nil {
		t.Fatal("malformed future MessagePack slot was accepted")
	}
}

func TestIndexedObjectSelferFlattensExplicitInlineBase(t *testing.T) {
	wire, err := EncodeMsgpack([]interface{}{int64(7), "base", true})
	if err != nil {
		t.Fatal(err)
	}
	var decoded indexedObjectTestInline
	if err := DecodeMsgpack(wire, &decoded); err != nil {
		t.Fatalf("DecodeMsgpack: %v", err)
	}
	if decoded.Version != 7 || decoded.Name != "base" || !decoded.Enabled {
		t.Fatalf("decoded inline object = %+v", decoded)
	}
	reencoded, err := EncodeIndexedMsgpack(&decoded)
	if err != nil {
		t.Fatalf("EncodeIndexedMsgpack: %v", err)
	}
	if !bytes.Equal(reencoded, wire) {
		t.Fatalf("inline wire changed: got % x, want % x", reencoded, wire)
	}

	shortWire, err := EncodeMsgpack([]interface{}{int64(8)})
	if err != nil {
		t.Fatal(err)
	}
	if err := DecodeMsgpack(shortWire, &decoded); err != nil {
		t.Fatalf("DecodeMsgpack(short): %v", err)
	}
	if decoded.FieldCount == nil || *decoded.FieldCount != 1 || decoded.Name != "" || decoded.Enabled {
		t.Fatalf("decoded short inline object = %+v", decoded)
	}
	shortReencoded, err := EncodeIndexedMsgpack(&decoded)
	if err != nil {
		t.Fatalf("EncodeIndexedMsgpack(short): %v", err)
	}
	if !bytes.Equal(shortReencoded, shortWire) {
		t.Fatalf("short inline wire changed: got % x, want % x", shortReencoded, shortWire)
	}
}

func TestIndexedObjectSelferUsesMessagePackReadSingleConversions(t *testing.T) {
	wire, err := EncodeMsgpack([]interface{}{
		math.MaxFloat64,
		[]interface{}{math.NaN(), math.Inf(-1), int64(-7), uint64(9)},
	})
	if err != nil {
		t.Fatal(err)
	}
	var decoded indexedObjectTestSingles
	if err := DecodeMsgpack(wire, &decoded); err != nil {
		t.Fatalf("DecodeMsgpack: %v", err)
	}
	if !math.IsInf(float64(decoded.Value), 1) || len(decoded.Values) != 4 ||
		!math.IsNaN(float64(decoded.Values[0])) || !math.IsInf(float64(decoded.Values[1]), -1) ||
		decoded.Values[2] != -7 || decoded.Values[3] != 9 {
		t.Fatalf("ReadSingle conversions = %#v", decoded)
	}
}

func TestOrdinaryMessagePackEncoderCannotSilentlyEncodeIndexedRawSlots(t *testing.T) {
	count := 3
	value := indexedObjectTestChild{
		IndexedObjectMetadata: IndexedObjectMetadata{
			FieldCount:  &count,
			FutureSlots: [][]byte{{0xcc, 0x80}},
		},
	}
	if _, err := EncodeMsgpack(&value); err == nil {
		t.Fatal("ordinary EncodeMsgpack accepted an indexed raw slot without the validated Raw-mode boundary")
	}
}

func assertIndexedObjectTestWire(t *testing.T, wire []byte, future []byte) {
	t.Helper()
	root, err := decodeRawMsgpackArray(wire, "test parent")
	if err != nil {
		t.Fatalf("decode parent raw: %v", err)
	}
	if len(root) != 2 {
		t.Fatalf("parent slots = %d, want 2", len(root))
	}
	children, err := decodeRawMsgpackArray(root[0], "test children")
	if err != nil {
		t.Fatalf("decode children raw: %v", err)
	}
	short, err := decodeRawMsgpackArray(children[0], "test short child")
	if err != nil {
		t.Fatalf("decode short child raw: %v", err)
	}
	if len(short) != 1 {
		t.Fatalf("short child slots = %d, want 1", len(short))
	}
	withFuture, err := decodeRawMsgpackArray(children[1], "test future child")
	if err != nil {
		t.Fatalf("decode future child raw: %v", err)
	}
	if len(withFuture) != 3 || !bytes.Equal(withFuture[2], future) {
		t.Fatalf("future child wire = % x, want trailing slot % x", withFuture, future)
	}
}
