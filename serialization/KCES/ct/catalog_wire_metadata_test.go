package ct

import (
	"bytes"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/msgpack"
)

func TestCatalogRejectsUnsupportedWidths(t *testing.T) {
	for _, fields := range [][]interface{}{
		{int64(0), int64(0), int64(0), int64(0), nil, nil, uint64(0), int64(0)},
		{int64(0), int64(0), int64(0), int64(0), nil, nil, uint64(0), int64(0), false, []interface{}{}, []interface{}{}, []interface{}{}, nil},
	} {
		wire, err := msgpack.EncodeMsgpack(fields)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeCatalog(wire); err == nil || !strings.Contains(err.Error(), "indexed-array width") {
			t.Fatalf("unsupported catalog width error = %v", err)
		}
	}
}

func TestCatalogUsesTypedNullability(t *testing.T) {
	wire, err := msgpack.EncodeMsgpack([]interface{}{
		int64(1000), int64(CatalogTypeParts), int64(PackageTypePlugin), int64(0),
		nil, nil, uint64(0), int64(0), false,
		[]interface{}{"bundle.aba", nil},
		[]interface{}{nil, ".menuassets"},
		[]interface{}{nil, []interface{}{int64(0), nil, uint64(9)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	value, err := DecodeCatalog(wire)
	if err != nil {
		t.Fatalf("DecodeCatalog: %v", err)
	}
	if value.Name != nil || value.SubName != nil || value.ResourceFileNames[1] != nil || value.ExtensionList[0] != nil ||
		value.Items[0] != nil || value.Items[1] == nil || value.Items[1].Name != nil {
		t.Fatalf("typed nullability changed: %+v", value)
	}
	reencoded, err := EncodeCatalog(value)
	if err != nil {
		t.Fatalf("EncodeCatalog: %v", err)
	}
	if !bytes.Equal(reencoded, wire) {
		t.Fatalf("catalog wire changed: got %x want %x", reencoded, wire)
	}
}

func TestCatalogAndExtensionRejectRootTrailingData(t *testing.T) {
	wire := []byte{0xc0, 1, 2, 3}
	if _, err := DecodeCatalog(wire); err == nil || !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("catalog trailing-data error = %v", err)
	}
	if _, err := DecodeExtensionNameList(wire); err == nil || !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("extension trailing-data error = %v", err)
	}
}

func TestExtensionNameListRejectsUnsupportedWidths(t *testing.T) {
	for _, fields := range [][]interface{}{
		{nil},
		{nil, []interface{}{}, nil},
	} {
		wire, err := msgpack.EncodeMsgpack(fields)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeExtensionNameList(wire); err == nil || !strings.Contains(err.Error(), "indexed-array width") {
			t.Fatalf("unsupported ExtensionNameList width error = %v", err)
		}
	}
}
