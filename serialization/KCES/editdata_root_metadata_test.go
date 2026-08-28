package KCES

import (
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/msgpack"
)

func TestEditDataDecodersRejectTrailingRootData(t *testing.T) {
	colorPreset, err := NewColorPreset(colorPresetTestGUID)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		encode func() ([]byte, error)
		decode func([]byte) error
		raw    bool
	}{
		{name: "PresetPanelNameSaveData", encode: func() ([]byte, error) { return EncodePresetPanelNameSaveData(&PresetPanelNameSaveData{}) }, decode: func(data []byte) error { _, err := DecodePresetPanelNameSaveData(data); return err }},
		{name: "PaletteColorSaveData", encode: func() ([]byte, error) { return EncodePaletteColorSaveData(&PaletteColorSaveData{}) }, decode: func(data []byte) error { _, err := DecodePaletteColorSaveData(data); return err }},
		{name: "GradPointsData", encode: func() ([]byte, error) { return EncodeGradPointsData(&GradPointsData{}) }, decode: func(data []byte) error { _, err := DecodeGradPointsData(data); return err }},
		{name: "MoveablePanelSaveData", encode: func() ([]byte, error) { return EncodeMoveablePanelSaveData(&MoveablePanelSaveData{}) }, decode: func(data []byte) error { _, err := DecodeMoveablePanelSaveData(data); return err }},
		{name: "ColorPresetOrderList", encode: func() ([]byte, error) { return EncodeColorPresetOrderList(&ColorPresetOrderList{}) }, decode: func(data []byte) error { _, err := DecodeColorPresetOrderList(data); return err }, raw: true},
		{name: "ColorPreset", encode: func() ([]byte, error) { return EncodeColorPreset(colorPreset) }, decode: func(data []byte) error { _, err := DecodeColorPreset(data); return err }, raw: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wire, err := test.encode()
			if err != nil {
				t.Fatal(err)
			}
			if test.raw {
				wire, err = msgpack.DecompressLz4BlockArray(wire)
				if err != nil {
					t.Fatal(err)
				}
			}
			wire = append(wire, 0xc0)
			if err := test.decode(wire); err == nil || !strings.Contains(strings.ToLower(err.Error()), "trailing") {
				t.Fatalf("decode error = %v, want trailing-data rejection", err)
			}
		})
	}
}

func TestNullableEditDataRootsRejectTrailingData(t *testing.T) {
	for name, decode := range map[string]func([]byte) error{
		"PresetPanelNameSaveData": func(data []byte) error { _, err := DecodePresetPanelNameSaveData(data); return err },
		"PaletteColorSaveData":    func(data []byte) error { _, err := DecodePaletteColorSaveData(data); return err },
		"GradPointsData":          func(data []byte) error { _, err := DecodeGradPointsData(data); return err },
		"MoveablePanelSaveData":   func(data []byte) error { _, err := DecodeMoveablePanelSaveData(data); return err },
		"ColorPresetOrderList":    func(data []byte) error { _, err := DecodeColorPresetOrderList(data); return err },
		"ColorPreset":             func(data []byte) error { _, err := DecodeColorPreset(data); return err },
	} {
		t.Run(name, func(t *testing.T) {
			if err := decode([]byte{0xc0, 0xc0}); err == nil {
				t.Fatal("nil root with trailing data unexpectedly decoded")
			}
		})
	}
}

func TestBridgeSessionDataRejectsTrailingRootData(t *testing.T) {
	data := []byte{0x93, 0x00, 0xa1, 'x', 0x90, 0xc0}
	wire := makeBridgeSessionVirtualDirectory(t, 1000, data, []byte("x"), nil)
	if _, err := DecodeKCESBridgeSession(wire); err == nil || !strings.Contains(strings.ToLower(err.Error()), "trailing") {
		t.Fatalf("DecodeKCESBridgeSession() error = %v, want trailing data", err)
	}
}
