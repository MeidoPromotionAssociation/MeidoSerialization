package KCES

import (
	"reflect"
	"testing"
)

func TestKCMetaDataRoundTripPreservesVersionAndWidth(t *testing.T) {
	model := "sample.kcmodel"
	texture := "sample.kctex"
	guid := "sample-guid"
	value := &KCMetaData{
		Version:                         9001,
		ModelFileName:                   &model,
		FormTextureFileName:             &texture,
		MaterialPerOriginalMenuFileName: []*string{nil, &model},
		BuildVersion:                    13500,
		GameVersion:                     349000,
		GUID:                            &guid,
		MaterialPerOriginalMenuVersion:  []int32{299, 300},
	}

	wire, err := EncodeKCMetaData(value)
	if err != nil {
		t.Fatalf("EncodeKCMetaData: %v", err)
	}
	if got := rawArrayWidth(t, wire); got != 9 {
		t.Fatalf("KCMetaData width = %d, want 9", got)
	}
	decoded, err := DecodeKCMetaData(wire)
	if err != nil {
		t.Fatalf("DecodeKCMetaData: %v", err)
	}
	if !reflect.DeepEqual(decoded, value) {
		t.Fatalf("KCMetaData round trip mismatch\n got: %#v\nwant: %#v", decoded, value)
	}
}
