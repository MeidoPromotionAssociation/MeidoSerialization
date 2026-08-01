package KCES

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

func TestKCESSystemDataRoundTripAllKnownEditDataAndOpaqueFiles(t *testing.T) {
	box1 := "BOX1"
	boxJP := "日本語"
	presetID := "preset-guid-1"
	hairLengthPanel := "HairLengthPanel"
	colorPreset, err := NewColorPreset("12345678-1234-1234-1234-123456789abc")
	if err != nil {
		t.Fatal(err)
	}
	colors := make(map[int32]int32, 10)
	for key := int32(0); key <= 8; key++ {
		colors[key] = key * 100
	}
	colors[99] = -99
	gradColors := make(map[int32]int32, 9)
	for key := int32(0); key <= 8; key++ {
		gradColors[key] = key
	}
	value := &KCESSystemData{
		Format:           KCESSystemDataFormat,
		Version:          1000,
		ContainerFraming: ct.VirtualDirectoryFramingExtended,
		Directories: map[string]ct.VirtualDirectoryMetadata{
			"EditData":                    {Version: 1000},
			"EditData/color_preset":       {Version: 1000},
			"EditData/color_preset/hairf": {Version: 1000},
			"Other":                       {Version: 1000},
		},
		EditData: []KCESEditDataFile{
			{
				Path: "EditData/GradSv2",
				Kind: KCESEditDataGradPoints,
				GradPoints: &GradPointsData{
					GradPointParam:              []map[int32]int32{gradColors},
					ControlPointPosValue:        []float32{0.25},
					GradationPointPositionRates: []float32{0.75},
					EditMPN:                     7,
					PointRangeAfterRates:        []float32{0.1},
					PointRangeBeforeRates:       []float32{0.2},
					IsSave:                      1,
				},
			},
			{
				Path: "EditData/MoveablePanelManager::SceneEdit::savedata",
				Kind: KCESEditDataMoveablePanel,
				MoveablePanel: &MoveablePanelSaveData{
					MoveablePanelPosition: []MoveablePanelPositionEntry{{
						PanelName: &hairLengthPanel,
						Position:  Vector3{X: 10, Y: -20, Z: 3},
					}},
					// An empty active list has defined game semantics: use each
					// scene panel's default open state.
					MoveablePanelActiveState: []MoveablePanelActiveStateEntry{},
				},
			},
			{
				Path:         "EditData/PaletteColorSave3",
				Kind:         KCESEditDataPaletteColor,
				PaletteColor: &PaletteColorSaveData{Color: colors, Index: 3, IsSave: 1},
			},
			{
				Path:             "EditData/PresetPanelNameSaveData::SceneEdit::savedata",
				Kind:             KCESEditDataPresetPanelNames,
				PresetPanelNames: &PresetPanelNameSaveData{BoxNameList: []*string{&box1, nil, &boxJP}},
			},
			{
				Path:            "EditData/color_preset/hairf/preset_orderlist",
				Kind:            KCESEditDataPresetOrderList,
				PresetOrderList: &ColorPresetOrderList{Version: ColorPresetOrderListVersion, IDOrderList: []*string{&presetID, nil}},
			},
			{
				Path:        "EditData/color_preset/hairf/user-preset-1",
				Kind:        KCESEditDataColorPreset,
				ColorPreset: colorPreset,
			},
		},
		ExtraFiles: map[string][]byte{
			"EditData/future_payload": {0x91, 0x2a},
			"Other/value":             {1, 2, 3, 4},
		},
	}
	before := cloneSystemDataForTest(value)

	encoded, err := EncodeKCESSystemData(value)
	if err != nil {
		t.Fatalf("EncodeKCESSystemData: %v", err)
	}
	if !reflect.DeepEqual(value, before) {
		t.Fatalf("encoding mutated caller:\n got  %#v\n want %#v", value, before)
	}

	table, err := ct.ReadContentTable(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("ReadContentTable: %v", err)
	}
	if table.Framing != ct.VirtualDirectoryFramingExtended {
		t.Fatalf("container framing = %d, want extended", table.Framing)
	}
	if got, want := table.GetFileNames(), []string{
		"EditData/GradSv2",
		"EditData/MoveablePanelManager::SceneEdit::savedata",
		"EditData/PaletteColorSave3",
		"EditData/PresetPanelNameSaveData::SceneEdit::savedata",
		"EditData/color_preset/hairf/preset_orderlist",
		"EditData/color_preset/hairf/user-preset-1",
		"EditData/future_payload",
		"Other/value",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("virtual paths = %#v, want %#v", got, want)
	}

	decoded, err := DecodeKCESSystemData(encoded)
	if err != nil {
		t.Fatalf("DecodeKCESSystemData: %v", err)
	}
	if !reflect.DeepEqual(decoded, value) {
		t.Fatalf("round trip changed system.dat:\n got  %#v\n want %#v", decoded, value)
	}

	reencoded, err := EncodeKCESSystemData(decoded)
	if err != nil {
		t.Fatalf("re-encode: %v", err)
	}
	decodedAgain, err := DecodeKCESSystemData(reencoded)
	if err != nil {
		t.Fatalf("decode re-encoded system.dat: %v", err)
	}
	if !reflect.DeepEqual(decodedAgain, decoded) {
		t.Fatal("semantic second round trip changed system.dat")
	}
}

func TestKCESEditDataKindForPath(t *testing.T) {
	tests := map[string]KCESEditDataKind{
		"EditData/GradSv0":                                             KCESEditDataGradPoints,
		"EditData/GradSv00012":                                         KCESEditDataGradPoints,
		"EditData/PaletteColorSave9":                                   KCESEditDataPaletteColor,
		"EditData/MoveablePanelManager::SceneEdit::savedata":           KCESEditDataMoveablePanel,
		"EditData/PresetPanelNameSaveData::SceneEdit::savedata":        KCESEditDataPresetPanelNames,
		"EditData/color_preset/hairf/preset_orderlist":                 KCESEditDataPresetOrderList,
		"EditData/color_preset/category/menu/preset_orderlist":         KCESEditDataPresetOrderList,
		"EditData/color_preset/preset_orderlist":                       KCESEditDataPresetOrderList,
		"EditData/color_preset/hairf/user-preset-id":                   KCESEditDataColorPreset,
		"EditData/color_preset/category/menu/user-preset-id":           KCESEditDataColorPreset,
		"EditData/color_preset/user-preset-id":                         "",
		"EditData/color_preset/hairf/":                                 "",
		"EditData/GradSv":                                              "",
		"EditData/GradSv-1":                                            "",
		"EditData/GradSv1/child":                                       "",
		"EditData/paletteColorSave1":                                   "",
		"editdata/PaletteColorSave1":                                   "",
		"PaletteColorSave1":                                            "",
		"EditData/MoveablePanelManager❺❺SceneEdit❺❺savedata":           "",
		"EditData/PresetPanelNameSaveData::SceneEdit::savedata/future": "",
		"EditData/color_preset/hairf/PRESET_ORDERLIST":                 KCESEditDataColorPreset,
		"EditData/color_preset/hairf/not_preset_orderlist":             KCESEditDataColorPreset,
	}
	for path, want := range tests {
		if got := KCESEditDataKindForPath(path); got != want {
			t.Errorf("KCESEditDataKindForPath(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestDecodeKCESSystemDataRejectsMalformedKnownPayload(t *testing.T) {
	table := &ct.ContentTable{Version: 1000, Raw: make([]byte, ct.HeaderSize)}
	table.AddFile("EditData/PaletteColorSave0", []byte{0x93, 0x80})
	table.AddFile("EditData/future_payload", []byte{0xff})
	var binary bytes.Buffer
	if err := ct.WriteContentTable(&binary, table); err != nil {
		t.Fatal(err)
	}
	_, err := DecodeKCESSystemData(binary.Bytes())
	if err == nil || !strings.Contains(err.Error(), "PaletteColorSave0") {
		t.Fatalf("malformed known payload error = %v", err)
	}
}

func TestKCESSystemDataPreservesRootNilReferencePayloads(t *testing.T) {
	value := &KCESSystemData{
		Format:  KCESSystemDataFormat,
		Version: 1000,
		EditData: []KCESEditDataFile{
			{Path: "EditData/GradSv1", Kind: KCESEditDataGradPoints},
			{Path: kcesPresetPanelNameSaveDataPath, Kind: KCESEditDataPresetPanelNames},
			{Path: "EditData/PaletteColorSave1", Kind: KCESEditDataPaletteColor},
			{Path: kcesMoveablePanelSaveDataPath, Kind: KCESEditDataMoveablePanel},
			{Path: "EditData/color_preset/hairf/preset_orderlist", Kind: KCESEditDataPresetOrderList},
			{Path: "EditData/color_preset/hairf/nil-preset", Kind: KCESEditDataColorPreset},
		},
	}
	encoded, err := EncodeKCESSystemData(value)
	if err != nil {
		t.Fatalf("EncodeKCESSystemData: %v", err)
	}
	decoded, err := DecodeKCESSystemData(encoded)
	if err != nil {
		t.Fatalf("DecodeKCESSystemData: %v", err)
	}
	if len(decoded.EditData) != 6 {
		t.Fatalf("root nil reference count = %d, want 6", len(decoded.EditData))
	}
	for _, entry := range decoded.EditData {
		if entry.PresetPanelNames != nil || entry.PaletteColor != nil || entry.GradPoints != nil || entry.MoveablePanel != nil || entry.PresetOrderList != nil || entry.ColorPreset != nil {
			t.Fatalf("root nil reference payload changed: %#v", entry)
		}
	}
}

func TestKCESSystemDataPreservesStoredVersion(t *testing.T) {
	for _, version := range []int32{-1, 0, 999, 1000, 1001} {
		value := &KCESSystemData{Version: version}
		encoded, err := EncodeKCESSystemData(value)
		if err != nil {
			t.Fatalf("EncodeKCESSystemData(version=%d): %v", version, err)
		}
		decoded, err := DecodeKCESSystemData(encoded)
		if err != nil {
			t.Fatalf("DecodeKCESSystemData(version=%d): %v", version, err)
		}
		if decoded.Version != version {
			t.Fatalf("version round trip = %d, want %d", decoded.Version, version)
		}
	}
}

func TestKCESSystemDataPreservesVirtualDirectoryVersionFields(t *testing.T) {
	value := &KCESSystemData{
		Format:  KCESSystemDataFormat,
		Version: -17,
		Directories: map[string]ct.VirtualDirectoryMetadata{
			"future": {Version: 77},
			"empty":  {Version: -9},
		},
		ExtraFiles: map[string][]byte{"future/state": {0x11, 0x22}},
	}
	encoded, err := EncodeKCESSystemData(value)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeKCESSystemData(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Version != value.Version || !bytes.Equal(decoded.ExtraFiles["future/state"], []byte{0x11, 0x22}) || !reflect.DeepEqual(decoded.Directories, value.Directories) {
		t.Fatalf("system.dat directory fields changed: %+v", decoded)
	}
	reencoded, err := EncodeKCESSystemData(decoded)
	if err != nil {
		t.Fatal(err)
	}
	table, err := ct.ReadContentTable(bytes.NewReader(reencoded))
	if err != nil {
		t.Fatal(err)
	}
	if table.Version != value.Version {
		t.Fatalf("VirtualDirectory version changed: %+v", table)
	}
	if !reflect.DeepEqual(table.GetVirtualDirectoryMetadata(), value.Directories) {
		t.Fatalf("system.dat VirtualDirectory fields changed: %+v", table)
	}
}

func TestEncodeKCESSystemDataValidation(t *testing.T) {
	validColors := make(map[int32]int32, 9)
	for i := int32(0); i <= 8; i++ {
		validColors[i] = i
	}
	validPalette := &PaletteColorSaveData{Color: validColors}
	tests := map[string]*KCESSystemData{
		"nil": nil,
		"wrong format": {
			Format: "future-system-format",
		},
		"unknown typed path": {
			EditData: []KCESEditDataFile{{Path: "EditData/future", Kind: KCESEditDataPaletteColor, PaletteColor: validPalette}},
		},
		"path kind mismatch": {
			EditData: []KCESEditDataFile{{Path: "EditData/PaletteColorSave0", Kind: KCESEditDataGradPoints}},
		},
		"wrong union member": {
			EditData: []KCESEditDataFile{{Path: "EditData/GradSv0", Kind: KCESEditDataGradPoints, PaletteColor: validPalette}},
		},
		"duplicate typed path": {
			EditData: []KCESEditDataFile{
				{Path: "EditData/PaletteColorSave0", Kind: KCESEditDataPaletteColor, PaletteColor: validPalette},
				{Path: "EditData/PaletteColorSave0", Kind: KCESEditDataPaletteColor, PaletteColor: validPalette},
			},
		},
		"recognized path in extras": {
			ExtraFiles: map[string][]byte{"EditData/GradSv0": {0xc0}},
		},
		"typed extra collision": {
			EditData:   []KCESEditDataFile{{Path: "EditData/PaletteColorSave0", Kind: KCESEditDataPaletteColor, PaletteColor: validPalette}},
			ExtraFiles: map[string][]byte{"EditData/PaletteColorSave0": {1}},
		},
		"unsafe extra path": {
			ExtraFiles: map[string][]byte{"EditData/../escape": {1}},
		},
	}
	for name, value := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := EncodeKCESSystemData(value); err == nil {
				t.Fatalf("EncodeKCESSystemData(%#v) unexpectedly succeeded", value)
			}
		})
	}
}

func cloneSystemDataForTest(value *KCESSystemData) *KCESSystemData {
	if value == nil {
		return nil
	}
	clone := *value
	clone.EditData = append([]KCESEditDataFile(nil), value.EditData...)
	clone.ExtraFiles = make(map[string][]byte, len(value.ExtraFiles))
	for path, data := range value.ExtraFiles {
		clone.ExtraFiles[path] = append([]byte(nil), data...)
	}
	return &clone
}
