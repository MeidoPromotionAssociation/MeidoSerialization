package KCES

import (
	"bytes"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

func TestEditDataRootMetadataPreservesTrailingBytesAndNilRoots(t *testing.T) {
	tail := []byte{0xde, 0xad, 0xbe, 0xef, 0xc1}
	type decodeEncodeFunc func([]byte) (MessagePackRootMetadata, []byte, error)
	tests := []struct {
		name         string
		baseRoot     func() ([]byte, error)
		decodeEncode decodeEncodeFunc
	}{
		{
			name: "PresetPanelNameSaveData",
			baseRoot: func() ([]byte, error) {
				return EncodePresetPanelNameSaveData(&PresetPanelNameSaveData{})
			},
			decodeEncode: func(data []byte) (MessagePackRootMetadata, []byte, error) {
				value, err := DecodePresetPanelNameSaveData(data)
				if err != nil {
					return MessagePackRootMetadata{}, nil, err
				}
				encoded, err := EncodePresetPanelNameSaveData(value)
				return value.MessagePackRootMetadata, encoded, err
			},
		},
		{
			name: "PaletteColorSaveData",
			baseRoot: func() ([]byte, error) {
				return EncodePaletteColorSaveData(&PaletteColorSaveData{})
			},
			decodeEncode: func(data []byte) (MessagePackRootMetadata, []byte, error) {
				value, err := DecodePaletteColorSaveData(data)
				if err != nil {
					return MessagePackRootMetadata{}, nil, err
				}
				encoded, err := EncodePaletteColorSaveData(value)
				return value.MessagePackRootMetadata, encoded, err
			},
		},
		{
			name: "GradPointsData",
			baseRoot: func() ([]byte, error) {
				return EncodeGradPointsData(&GradPointsData{})
			},
			decodeEncode: func(data []byte) (MessagePackRootMetadata, []byte, error) {
				value, err := DecodeGradPointsData(data)
				if err != nil {
					return MessagePackRootMetadata{}, nil, err
				}
				encoded, err := EncodeGradPointsData(value)
				return value.MessagePackRootMetadata, encoded, err
			},
		},
		{
			name: "MoveablePanelSaveData",
			baseRoot: func() ([]byte, error) {
				return EncodeMoveablePanelSaveData(&MoveablePanelSaveData{})
			},
			decodeEncode: func(data []byte) (MessagePackRootMetadata, []byte, error) {
				value, err := DecodeMoveablePanelSaveData(data)
				if err != nil {
					return MessagePackRootMetadata{}, nil, err
				}
				encoded, err := EncodeMoveablePanelSaveData(value)
				return value.MessagePackRootMetadata, encoded, err
			},
		},
		{
			name: "ColorPresetOrderList",
			baseRoot: func() ([]byte, error) {
				encoded, err := EncodeColorPresetOrderList(&ColorPresetOrderList{})
				return decompressEditDataTestRoot(encoded, err)
			},
			decodeEncode: func(data []byte) (MessagePackRootMetadata, []byte, error) {
				value, err := DecodeColorPresetOrderList(data)
				if err != nil {
					return MessagePackRootMetadata{}, nil, err
				}
				encoded, err := EncodeColorPresetOrderList(value)
				decoded, err := decompressEditDataTestRoot(encoded, err)
				return value.MessagePackRootMetadata, decoded, err
			},
		},
		{
			name: "ColorPreset",
			baseRoot: func() ([]byte, error) {
				value, err := NewColorPreset(colorPresetTestGUID)
				if err != nil {
					return nil, err
				}
				encoded, err := EncodeColorPreset(value)
				return decompressEditDataTestRoot(encoded, err)
			},
			decodeEncode: func(data []byte) (MessagePackRootMetadata, []byte, error) {
				value, err := DecodeColorPreset(data)
				if err != nil {
					return MessagePackRootMetadata{}, nil, err
				}
				encoded, err := EncodeColorPreset(value)
				decoded, err := decompressEditDataTestRoot(encoded, err)
				return value.MessagePackRootMetadata, decoded, err
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name+"/non_nil_root", func(t *testing.T) {
			base, err := tc.baseRoot()
			if err != nil {
				t.Fatalf("build base root: %v", err)
			}
			wire := append(append([]byte(nil), base...), tail...)
			metadata, reencoded, err := tc.decodeEncode(wire)
			if err != nil {
				t.Fatalf("decode/re-encode: %v", err)
			}
			if metadata.RootNil || !bytes.Equal(metadata.TrailingData, tail) {
				t.Fatalf("metadata = %+v, want non-nil root and tail % x", metadata, tail)
			}
			if !bytes.Equal(reencoded, wire) {
				t.Fatalf("root stream changed:\n got  % x\n want % x", reencoded, wire)
			}
		})

		t.Run(tc.name+"/nil_root", func(t *testing.T) {
			wire := append([]byte{0xc0}, tail...)
			metadata, reencoded, err := tc.decodeEncode(wire)
			if err != nil {
				t.Fatalf("decode/re-encode: %v", err)
			}
			if !metadata.RootNil || !bytes.Equal(metadata.TrailingData, tail) {
				t.Fatalf("metadata = %+v, want nil root and tail % x", metadata, tail)
			}
			if !bytes.Equal(reencoded, wire) {
				t.Fatalf("nil root stream changed:\n got  % x\n want % x", reencoded, wire)
			}
		})
	}
}

func TestBridgeSessionDataRootMetadataRoundTrip(t *testing.T) {
	tail := []byte{0xde, 0xad, 0xbe, 0xef, 0xc1}
	for name, root := range map[string][]byte{
		"non_nil_root": {0x93, 0x00, 0xa1, 'x', 0x90},
		"nil_root":     {0xc0},
	} {
		t.Run(name, func(t *testing.T) {
			sessionData := append(append([]byte(nil), root...), tail...)
			wire := makeBridgeSessionVirtualDirectory(t, 1000, sessionData, []byte("x"), nil)
			value, err := DecodeKCESBridgeSession(wire)
			if err != nil {
				t.Fatalf("DecodeKCESBridgeSession: %v", err)
			}
			if value.SessionData == nil || value.SessionData.RootNil != (name == "nil_root") || !bytes.Equal(value.SessionData.TrailingData, tail) {
				t.Fatalf("session root metadata = %+v", value.SessionData)
			}
			reencoded, err := EncodeKCESBridgeSession(value)
			if err != nil {
				t.Fatalf("EncodeKCESBridgeSession: %v", err)
			}
			table, err := ct.ReadContentTable(bytes.NewReader(reencoded))
			if err != nil {
				t.Fatalf("ReadContentTable: %v", err)
			}
			got, err := table.GetFileData(kcesBridgeSessionDataFile)
			if err != nil {
				t.Fatalf("GetFileData: %v", err)
			}
			if !bytes.Equal(got, sessionData) {
				t.Fatalf("session_data changed:\n got  % x\n want % x", got, sessionData)
			}
		})
	}
}

func TestRootNilMetadataRejectsPopulatedWireFields(t *testing.T) {
	_, err := EncodeGradPointsData(&GradPointsData{
		MessagePackRootMetadata: MessagePackRootMetadata{RootNil: true},
		IsSave:                  1,
	})
	if err == nil {
		t.Fatal("rootNil silently discarded a populated known field")
	}
}

func decompressEditDataTestRoot(data []byte, err error) ([]byte, error) {
	if err != nil {
		return nil, err
	}
	return ct.DecompressLz4BlockArray(data)
}
