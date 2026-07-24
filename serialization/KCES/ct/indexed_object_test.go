package ct

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/ugorji/go/codec"
)

// indexedObjectTestChild is a fixed two-slot test object
type indexedObjectTestChild struct {
	_struct struct{} `codec:",toarray"`
	Number  int32    `json:"number"`
	Text    string   `json:"text"`
}

func (v indexedObjectTestChild) CodecEncodeSelf(e *codec.Encoder) {
	EncodeIndexedObjectSelf(e, &v)
}

func (v *indexedObjectTestChild) CodecDecodeSelf(d *codec.Decoder) {
	DecodeIndexedObjectSelf(d, v)
}

// indexedObjectTestParent contains value-typed collections
type indexedObjectTestParent struct {
	_struct  struct{}                          `codec:",toarray"`
	Children []indexedObjectTestChild          `json:"children"`
	ByName   map[string]indexedObjectTestChild `json:"byName"`
}

func (v indexedObjectTestParent) CodecEncodeSelf(e *codec.Encoder) {
	EncodeIndexedObjectSelf(e, &v)
}

func (v *indexedObjectTestParent) CodecDecodeSelf(d *codec.Decoder) {
	DecodeIndexedObjectSelf(d, v)
}

// IndexedObjectTestInlineBase is an explicitly inlined base layout
type IndexedObjectTestInlineBase struct {
	Version int32  `json:"version"`
	Name    string `json:"name"`
}

// indexedObjectTestInline has a fixed three-slot inherited layout
type indexedObjectTestInline struct {
	_struct                     struct{} `codec:",toarray"`
	IndexedObjectTestInlineBase `codec:",inline"`
	Enabled                     bool `json:"enabled"`
}

func (v indexedObjectTestInline) CodecEncodeSelf(e *codec.Encoder) {
	EncodeIndexedObjectSelf(e, &v)
}

func (v *indexedObjectTestInline) CodecDecodeSelf(d *codec.Decoder) {
	DecodeIndexedObjectSelf(d, v)
}

// indexedObjectTestSingles exercises MessagePack-CSharp ReadSingle conversions
type indexedObjectTestSingles struct {
	_struct struct{}  `codec:",toarray"`
	Value   float32   `json:"value"`
	Values  []float32 `json:"values"`
}

func (v indexedObjectTestSingles) CodecEncodeSelf(e *codec.Encoder) {
	EncodeIndexedObjectSelf(e, &v)
}

func (v *indexedObjectTestSingles) CodecDecodeSelf(d *codec.Decoder) {
	DecodeIndexedObjectSelf(d, v)
}

// indexedObjectTestSparse declares one absent C# key
type indexedObjectTestSparse struct {
	_struct struct{} `codec:",toarray" kces:"nil=1"`
	Number  int32    `json:"number"`
	Text    string   `json:"text"`
}

func (v indexedObjectTestSparse) CodecEncodeSelf(e *codec.Encoder) {
	EncodeIndexedObjectSelf(e, &v)
}

func (v *indexedObjectTestSparse) CodecDecodeSelf(d *codec.Decoder) {
	DecodeIndexedObjectSelf(d, v)
}

func TestIndexedObjectSelferRoundTripsCurrentNestedLayout(t *testing.T) {
	wire, err := EncodeMsgpack([]interface{}{
		[]interface{}{
			[]interface{}{int64(7), "first"},
			[]interface{}{int64(8), "second"},
		},
		map[string]interface{}{
			"full": []interface{}{int64(9), "map"},
		},
	})
	if err != nil {
		t.Fatalf("build test wire: %v", err)
	}

	var decoded indexedObjectTestParent
	if err := DecodeMsgpack(wire, &decoded); err != nil {
		t.Fatalf("DecodeMsgpack: %v", err)
	}
	if len(decoded.Children) != 2 || decoded.Children[0].Number != 7 || decoded.Children[0].Text != "first" ||
		decoded.Children[1].Number != 8 || decoded.Children[1].Text != "second" ||
		decoded.ByName["full"].Number != 9 || decoded.ByName["full"].Text != "map" {
		t.Fatalf("decoded nested object = %#v", decoded)
	}

	reencoded, err := EncodeIndexedMsgpack(&decoded)
	if err != nil {
		t.Fatalf("EncodeIndexedMsgpack: %v", err)
	}
	if !bytes.Equal(reencoded, wire) {
		t.Fatalf("nested wire changed: got % x, want % x", reencoded, wire)
	}
}

func TestIndexedObjectSelferRejectsWrongWidths(t *testing.T) {
	for _, fields := range [][]interface{}{
		{int64(7)},
		{int64(7), "text", true},
	} {
		wire, err := EncodeMsgpack(fields)
		if err != nil {
			t.Fatal(err)
		}
		var decoded indexedObjectTestChild
		if err := DecodeMsgpack(wire, &decoded); err == nil || !strings.Contains(err.Error(), "indexed-array width") {
			t.Fatalf("wrong-width decode error = %v", err)
		}
	}
}

func TestIndexedObjectSelferRejectsNonNilSparseSlot(t *testing.T) {
	wire, err := EncodeMsgpack([]interface{}{int64(7), true, "text"})
	if err != nil {
		t.Fatal(err)
	}
	var decoded indexedObjectTestSparse
	if err := DecodeMsgpack(wire, &decoded); err == nil || !strings.Contains(err.Error(), "sparse slot 1 must be nil") {
		t.Fatalf("non-nil sparse-slot error = %v", err)
	}
}

func TestIndexedObjectSelferRejectsNilInValuePositions(t *testing.T) {
	for name, wireValue := range map[string]interface{}{
		"scalar": []interface{}{nil, "text"},
		"slice":  []interface{}{[]interface{}{nil}, map[string]interface{}{}},
		"map":    []interface{}{[]interface{}{}, map[string]interface{}{"null": nil}},
	} {
		t.Run(name, func(t *testing.T) {
			wire, err := EncodeMsgpack(wireValue)
			if err != nil {
				t.Fatal(err)
			}
			if name == "scalar" {
				var decoded indexedObjectTestChild
				if err := DecodeMsgpack(wire, &decoded); err == nil || !strings.Contains(err.Error(), "nil is not valid") {
					t.Fatalf("nil scalar error = %v", err)
				}
				return
			}
			var decoded indexedObjectTestParent
			if err := DecodeMsgpack(wire, &decoded); err == nil || !strings.Contains(err.Error(), "nil is not valid") {
				t.Fatalf("nil collection value error = %v", err)
			}
		})
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
	if err := DecodeMsgpack(shortWire, &decoded); err == nil || !strings.Contains(err.Error(), "indexed-array width") {
		t.Fatalf("short inline error = %v", err)
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
