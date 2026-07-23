package ct

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/ugorji/go/codec"
)

func TestAssetBundleCatalogPreservesIndexedMetadataNullsAndTrailingData(t *testing.T) {
	itemFuture := codec.Raw{0xd4, 0x2a, 0x7f}
	rootFuture := codec.Raw{0x81, 0xa1, 'x', 0x92, 0xc0, 0xc3}
	items := []interface{}{
		nil,
		[]interface{}{int64(-7)},
		[]interface{}{int64(0), nil, uint64(99), itemFuture},
	}
	fields := []interface{}{
		int64(-100), int64(1 << 20), int64(99), int64(-8),
		nil, nil, uint64(123), int64(-456), false,
		[]interface{}{"bundle.aba", nil},
		[]interface{}{nil, ".future"},
		items,
		rootFuture,
	}
	root, err := encodeMsgpackAllowRaw(fields)
	if err != nil {
		t.Fatal(err)
	}
	tail := []byte{0xde, 0xad, 0xbe, 0xef, 0xc1}
	wire := append(append([]byte(nil), root...), tail...)

	value, err := DecodeCatalog(wire)
	if err != nil {
		t.Fatalf("DecodeCatalog: %v", err)
	}
	if value.Kind != CatalogKindAssetBundle || value.FieldCount == nil || *value.FieldCount != 13 ||
		!reflect.DeepEqual(value.FutureSlots, [][]byte{rootFuture}) || !bytes.Equal(value.TrailingData, tail) {
		t.Fatalf("root metadata changed: %+v", value)
	}
	if !value.NameIsNil || !value.SubNameIsNil ||
		!reflect.DeepEqual(value.ResourceFileNameNulls, []bool{false, true}) ||
		!reflect.DeepEqual(value.ExtensionListNulls, []bool{true}) {
		t.Fatalf("nullable strings changed: %+v", value)
	}
	if len(value.Items) != 3 || !reflect.DeepEqual(value.ItemNulls, []bool{true}) {
		t.Fatalf("nullable items changed: items=%+v nulls=%v", value.Items, value.ItemNulls)
	}
	if value.Items[1].FieldCount == nil || *value.Items[1].FieldCount != 1 || value.Items[1].ResourceIndex != -7 {
		t.Fatalf("short item changed: %+v", value.Items[1])
	}
	if value.Items[2].FieldCount == nil || *value.Items[2].FieldCount != 4 || !value.Items[2].NameIsNil ||
		!reflect.DeepEqual(value.Items[2].FutureSlots, [][]byte{itemFuture}) {
		t.Fatalf("future item changed: %+v", value.Items[2])
	}

	reencoded, err := EncodeCatalog(value)
	if err != nil {
		t.Fatalf("EncodeCatalog: %v", err)
	}
	if !bytes.Equal(reencoded, wire) {
		t.Fatalf("catalog wire changed:\n got  % x\n want % x", reencoded, wire)
	}
}

func TestCatalogShortCommonPrefixRequiresExplicitKind(t *testing.T) {
	common := []interface{}{int64(0), int64(0), int64(0), int64(0), nil, nil, uint64(0), int64(0)}
	wire, err := EncodeMsgpack(common)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeCatalog(wire); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("inferred ambiguous short catalog error = %v", err)
	}

	for _, kind := range []CatalogKind{CatalogKindAssetBundle, CatalogKindVirtualAsset} {
		t.Run(string(kind), func(t *testing.T) {
			value, err := DecodeCatalogWithKind(wire, kind)
			if err != nil {
				t.Fatalf("DecodeCatalogWithKind: %v", err)
			}
			if value.Kind != kind || value.FieldCount == nil || *value.FieldCount != int32(len(common)) {
				t.Fatalf("short catalog metadata = %+v", value)
			}
			reencoded, err := EncodeCatalog(value)
			if err != nil {
				t.Fatalf("EncodeCatalog: %v", err)
			}
			if !bytes.Equal(reencoded, wire) {
				t.Fatalf("short catalog changed: got %x want %x", reencoded, wire)
			}
		})
	}
}

func TestExtensionNameListPreservesIndexedMetadataNullsAndTrailingData(t *testing.T) {
	packFuture := codec.Raw{0xd6, 0x09, 0, 0, 0, 7}
	rootFuture := codec.Raw{0xcc, 0x80}
	fields := []interface{}{
		nil,
		[]interface{}{
			nil,
			[]interface{}{},
			[]interface{}{nil, uint64(42), packFuture},
		},
		rootFuture,
	}
	root, err := encodeMsgpackAllowRaw(fields)
	if err != nil {
		t.Fatal(err)
	}
	tail := []byte{0xc1, 0xff, 0x00}
	wire := append(append([]byte(nil), root...), tail...)

	value, err := DecodeExtensionNameList(wire)
	if err != nil {
		t.Fatalf("DecodeExtensionNameList: %v", err)
	}
	if !value.ExtensionIsNil || value.FieldCount == nil || *value.FieldCount != 3 ||
		!reflect.DeepEqual(value.FutureSlots, [][]byte{rootFuture}) || !bytes.Equal(value.TrailingData, tail) {
		t.Fatalf("ExtensionNameList root metadata changed: %+v", value)
	}
	if len(value.Data) != 3 || !reflect.DeepEqual(value.DataNulls, []bool{true}) {
		t.Fatalf("nullable packs changed: data=%+v nulls=%v", value.Data, value.DataNulls)
	}
	if value.Data[1].FieldCount == nil || *value.Data[1].FieldCount != 0 {
		t.Fatalf("zero-width pack changed: %+v", value.Data[1])
	}
	if value.Data[2].FieldCount == nil || *value.Data[2].FieldCount != 3 || !value.Data[2].NameIsNil ||
		!reflect.DeepEqual(value.Data[2].FutureSlots, [][]byte{packFuture}) {
		t.Fatalf("future pack changed: %+v", value.Data[2])
	}

	reencoded, err := EncodeExtensionNameList(value)
	if err != nil {
		t.Fatalf("EncodeExtensionNameList: %v", err)
	}
	if !bytes.Equal(reencoded, wire) {
		t.Fatalf("ExtensionNameList wire changed:\n got  % x\n want % x", reencoded, wire)
	}
}

func TestCatalogAndExtensionNilRootWithTrailingData(t *testing.T) {
	tail := []byte{1, 2, 3, 0xc1}
	wire := append([]byte{0xc0}, tail...)

	catalog, err := DecodeCatalog(wire)
	if err != nil || catalog == nil || !catalog.RootNil || !bytes.Equal(catalog.TrailingData, tail) {
		t.Fatalf("DecodeCatalog(nil+tail) = %+v, %v", catalog, err)
	}
	if encoded, err := EncodeCatalog(catalog); err != nil || !bytes.Equal(encoded, wire) {
		t.Fatalf("EncodeCatalog(nil+tail) = %x, %v", encoded, err)
	}

	list, err := DecodeExtensionNameList(wire)
	if err != nil || list == nil || !list.RootNil || !bytes.Equal(list.TrailingData, tail) {
		t.Fatalf("DecodeExtensionNameList(nil+tail) = %+v, %v", list, err)
	}
	if encoded, err := EncodeExtensionNameList(list); err != nil || !bytes.Equal(encoded, wire) {
		t.Fatalf("EncodeExtensionNameList(nil+tail) = %x, %v", encoded, err)
	}
}

func TestCatalogMetadataRejectsSilentFieldLoss(t *testing.T) {
	tests := []struct {
		name  string
		value *AssetBundleCatalog
	}{
		{
			name: "nil item with fields",
			value: &AssetBundleCatalog{
				Items:     []CatalogItem{{Hash: 1}},
				ItemNulls: []bool{true},
			},
		},
		{
			name: "short root drops name",
			value: &AssetBundleCatalog{
				IndexedObjectMetadata: IndexedObjectMetadata{FieldCount: intPointerForCatalogTest(4)},
				Name:                  "hidden",
			},
		},
		{
			name: "nil string with text",
			value: &AssetBundleCatalog{
				Name:      "hidden",
				NameIsNil: true,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := EncodeCatalog(tc.value); err == nil {
				t.Fatal("metadata conflict silently encoded")
			}
		})
	}

	badFuture := &ExtensionNameList{
		IndexedObjectMetadata: IndexedObjectMetadata{
			FieldCount:  intPointerForCatalogTest(3),
			FutureSlots: [][]byte{{0xc1}},
		},
	}
	if _, err := EncodeExtensionNameList(badFuture); err == nil {
		t.Fatal("reserved future marker silently encoded")
	}
}

func intPointerForCatalogTest(value int32) *int32 { return &value }
