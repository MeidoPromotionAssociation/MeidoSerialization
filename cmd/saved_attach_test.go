package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

func TestKCESSavedAttachCommandRoutes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "accessory.sad")
	partName := "hat-point"
	native, err := serializationKCES.EncodeSavedAttach(&serializationKCES.SavedAttachFile{
		Signature: serializationKCES.SavedAttachSignature,
		Version:   serializationKCES.SavedAttachFileVersion,
		Items: []serializationKCES.SavedAttachData{{
			Version:      serializationKCES.SavedAttachRecordVersion,
			PartName:     &partName,
			Enabled:      true,
			MySlotID:     "accHat",
			TargetSlotID: "body",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, native, 0644); err != nil {
		t.Fatal(err)
	}

	output, err := executeCommand(RootCmd, "determine", "--strict", path)
	if err != nil {
		t.Fatalf("determine .sad: %v\n%s", err, output)
	}
	for _, want := range []string{"Type: sad", "Format: binary", "Game: KCES", "Version: 2000"} {
		if !strings.Contains(output, want) {
			t.Fatalf("determine output lacks %q:\n%s", want, output)
		}
	}

	if output, err = executeCommand(RootCmd, "convert2json", "--strict", "--type", "sad", path); err != nil {
		t.Fatalf("convert2json .sad: %v\n%s", err, output)
	}
	jsonPath := path + ".json"
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if output, err = executeCommand(RootCmd, "convert2mod", "--strict", "--type", "sad.json", jsonPath); err != nil {
		t.Fatalf("convert2mod .sad.json: %v\n%s", err, output)
	}
	back, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := serializationKCES.DecodeSavedAttach(back)
	if err != nil || len(decoded.Items) != 1 || decoded.Items[0].PartName == nil || *decoded.Items[0].PartName != partName {
		t.Fatalf("command round-trip: value=%+v err=%v", decoded, err)
	}
}

func TestKCESSavedAttachNonStrictFilter(t *testing.T) {
	oldStrict, oldType := strictMode, fileType
	t.Cleanup(func() {
		strictMode = oldStrict
		fileType = oldType
	})
	strictMode = false

	fileType = "sad"
	if !fileTypeFilter(`C:\mods\accessory.sad`) || fileTypeFilter(`C:\mods\accessory.sad.json`) {
		t.Fatal("non-strict sad filter did not separate native and editing paths")
	}
	fileType = "sad.json"
	if !fileTypeFilter(`C:\mods\accessory.sad.json`) || !fileTypeFilter(`C:\mods\accessory.SAD.JSON`) || fileTypeFilter(`C:\mods\accessory.sad`) {
		t.Fatal("non-strict sad.json filter did not separate editing and native paths")
	}
}

func TestConvertToModUppercaseJSONDoesNotOverwriteInput(t *testing.T) {
	dir := t.TempDir()
	partName := "case-test"
	native, err := serializationKCES.EncodeSavedAttach(&serializationKCES.SavedAttachFile{
		Signature: serializationKCES.SavedAttachSignature,
		Version:   serializationKCES.SavedAttachFileVersion,
		Items: []serializationKCES.SavedAttachData{{
			Version:      serializationKCES.SavedAttachRecordVersion,
			PartName:     &partName,
			MySlotID:     "body",
			TargetSlotID: "body",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := serializationKCES.DecodeSavedAttach(native)
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := json.MarshalIndent(decoded, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	jsonPath := filepath.Join(dir, "accessory.SAD.JSON")
	outputPath := filepath.Join(dir, "accessory.SAD")
	if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
		t.Fatal(err)
	}
	before := append([]byte(nil), jsonData...)

	if output, err := executeCommand(RootCmd, "convert2mod", jsonPath); err != nil {
		t.Fatalf("convert2mod uppercase JSON: %v\n%s", err, output)
	}
	if got, err := os.ReadFile(jsonPath); err != nil || !bytes.Equal(got, before) {
		t.Fatalf("input JSON was changed: err=%v\ngot=%s\nwant=%s", err, got, before)
	}
	if got, err := os.ReadFile(outputPath); err != nil {
		t.Fatalf("uppercase JSON output missing: %v", err)
	} else if _, err := serializationKCES.DecodeSavedAttach(got); err != nil {
		t.Fatalf("uppercase JSON output is invalid: %v", err)
	}
}
