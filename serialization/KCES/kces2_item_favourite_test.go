package KCES

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/msgpack"
	"github.com/ugorji/go/codec"
)

func TestKCES2ItemFavouriteStateSettingRoundTripPreservesRawStates(t *testing.T) {
	fileName := "hair_front.menu"
	unknownState := "FutureFavouriteState"
	value := KCES2ItemFavouriteStateSetting{
		{ItemFileName: &fileName, ItemFavouriteStateString: &unknownState},
		nil,
		{},
	}

	wire, err := EncodeKCES2ItemFavouriteStateSetting(value)
	if err != nil {
		t.Fatalf("EncodeKCES2ItemFavouriteStateSetting: %v", err)
	}
	var entries []codec.Raw
	if err := msgpack.DecodeMsgpack(wire, &entries); err != nil {
		t.Fatalf("decode favourite-state root: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("favourite-state entry count = %d, want 3", len(entries))
	}
	if got := rawArrayWidth(t, entries[0]); got != 2 {
		t.Fatalf("favourite-state entry width = %d, want 2", got)
	}
	if len(entries[1]) != 0 && (len(entries[1]) != 1 || entries[1][0] != 0xc0) {
		t.Fatalf("nil favourite-state entry changed: %x", entries[1])
	}

	decoded, err := DecodeKCES2ItemFavouriteStateSetting(wire)
	if err != nil {
		t.Fatalf("DecodeKCES2ItemFavouriteStateSetting: %v", err)
	}
	if !reflect.DeepEqual(decoded, value) {
		t.Fatalf("favourite-state round trip mismatch\n got: %#v\nwant: %#v", decoded, value)
	}
	reencoded, err := EncodeKCES2ItemFavouriteStateSetting(decoded)
	if err != nil {
		t.Fatalf("re-encode favourite-state setting: %v", err)
	}
	if !bytes.Equal(reencoded, wire) {
		t.Fatalf("favourite-state bytes changed\n got: %x\nwant: %x", reencoded, wire)
	}
}

func TestKCES2ItemFavouriteStateSettingRejectsUnknownEntryWidth(t *testing.T) {
	wire, err := msgpack.EncodeMsgpack([]interface{}{[]interface{}{"hair_front.menu"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeKCES2ItemFavouriteStateSetting(wire); err == nil || !strings.Contains(err.Error(), "indexed-array width 1") {
		t.Fatalf("DecodeKCES2ItemFavouriteStateSetting error = %v, want width error", err)
	}
}

func TestKCES2ItemFavouriteStateSettingPreservesNilRoot(t *testing.T) {
	var value KCES2ItemFavouriteStateSetting
	wire, err := EncodeKCES2ItemFavouriteStateSetting(value)
	if err != nil {
		t.Fatalf("EncodeKCES2ItemFavouriteStateSetting: %v", err)
	}
	decoded, err := DecodeKCES2ItemFavouriteStateSetting(wire)
	if err != nil {
		t.Fatalf("DecodeKCES2ItemFavouriteStateSetting: %v", err)
	}
	if decoded != nil {
		t.Fatalf("decoded nil root = %#v, want nil", decoded)
	}
}

func TestKCES2ItemFavouriteStateSettingSystemDataRoutingPreservesPresentNilRoot(t *testing.T) {
	if got := KCESEditDataKindForPath(KCES2ItemFavouriteStateSettingPath); got != KCESEditDataItemFavouriteStateSetting {
		t.Fatalf("KCESEditDataKindForPath(%q) = %q, want %q", KCES2ItemFavouriteStateSettingPath, got, KCESEditDataItemFavouriteStateSetting)
	}
	systemData := NewKCESSystemData()
	systemData.EditData = []KCESEditDataFile{{
		Path: KCES2ItemFavouriteStateSettingPath,
		Kind: KCESEditDataItemFavouriteStateSetting,
	}}
	wire, err := EncodeKCESSystemData(systemData)
	if err != nil {
		t.Fatalf("EncodeKCESSystemData: %v", err)
	}
	decoded, err := DecodeKCESSystemData(wire)
	if err != nil {
		t.Fatalf("DecodeKCESSystemData: %v", err)
	}
	if len(decoded.EditData) != 1 || decoded.EditData[0].Kind != KCESEditDataItemFavouriteStateSetting || decoded.EditData[0].ItemFavouriteStateSetting != nil {
		t.Fatalf("decoded item favourite state file = %#v, want present nil root", decoded.EditData)
	}

	jsonData, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("Marshal system.dat JSON: %v", err)
	}
	var edited KCESSystemData
	if err := json.Unmarshal(jsonData, &edited); err != nil {
		t.Fatalf("Unmarshal system.dat JSON: %v", err)
	}
	if len(edited.EditData) != 1 || edited.EditData[0].Kind != KCESEditDataItemFavouriteStateSetting || edited.EditData[0].ItemFavouriteStateSetting != nil {
		t.Fatalf("JSON item favourite state file = %#v, want present nil root", edited.EditData)
	}
	if _, err := EncodeKCESSystemData(&edited); err != nil {
		t.Fatalf("re-encode KCESSystemData: %v", err)
	}
}

func TestKCES2ItemFavouriteStateSettingRejectsOpaqueSystemDataCollision(t *testing.T) {
	systemData := NewKCESSystemData()
	systemData.ExtraFiles = map[string][]byte{KCES2ItemFavouriteStateSettingPath: {0xc0}}
	if _, err := EncodeKCESSystemData(systemData); err == nil || !strings.Contains(err.Error(), "use editData") {
		t.Fatalf("EncodeKCESSystemData error = %v, want typed-path error", err)
	}
}
