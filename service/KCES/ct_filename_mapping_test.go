package KCES

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

func TestCtServiceUsesGameVirtualDirectoryWindowsNameMapping(t *testing.T) {
	dir := t.TempDir()
	ctPath := filepath.Join(dir, "system.dat")
	outDir := filepath.Join(dir, "unpacked")
	roundTripPath := filepath.Join(dir, "roundtrip.ct")

	table := &ct.ContentTable{
		Version: 1000,
		Files:   make(map[string]ct.VirtualFile),
		Raw:     make([]byte, ct.HeaderSize),
	}
	copy(table.Raw[:7], ct.FileSignature)
	table.Raw[7] = ct.SerializeTypeMsgPack
	wantData := map[string][]byte{
		"MoveablePanelManager::SceneEdit::savedata":    []byte("panel"),
		"PresetPanelNameSaveData::SceneEdit::savedata": []byte("preset"),
		"directory:with-colon/file:name":               []byte("nested"),
		`unsafe"<>|:*?chars`:                           []byte("mapped"),
	}
	for name, data := range wantData {
		table.AddFile(name, data)
	}
	var encoded bytes.Buffer
	if err := ct.WriteContentTable(&encoded, table); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ctPath, encoded.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	service := &CtService{}
	if err := service.UnpackCt(ctPath, outDir); err != nil {
		t.Fatalf("UnpackCt: %v", err)
	}
	wantDiskFiles := map[string][]byte{
		"MoveablePanelManager❺❺SceneEdit❺❺savedata":        []byte("panel"),
		"PresetPanelNameSaveData❺❺SceneEdit❺❺savedata":     []byte("preset"),
		filepath.Join("directory❺with-colon", "file❺name"): []byte("nested"),
		"unsafe❶❷❸❹❺❻❼chars":                               []byte("mapped"),
	}
	for name, want := range wantDiskFiles {
		got, err := os.ReadFile(filepath.Join(outDir, name))
		if err != nil {
			t.Fatalf("read mapped output %q: %v", name, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("mapped output %q = %q, want %q", name, got, want)
		}
	}

	if err := service.PackCt(outDir, roundTripPath); err != nil {
		t.Fatalf("PackCt: %v", err)
	}
	roundTrip, err := service.ReadCt(roundTripPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := roundTrip.GetFileNames(), table.GetFileNames(); !reflect.DeepEqual(got, want) {
		t.Fatalf("round-trip virtual names = %v, want %v", got, want)
	}
	for name, want := range wantData {
		got, err := roundTrip.GetFileData(name)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("round-trip data %q = %q, want %q", name, got, want)
		}
	}
}
