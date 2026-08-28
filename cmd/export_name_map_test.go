package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES"
)

func TestKCESExportNameMapCommandRoutes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "export_map.enm")
	native, err := serializationKCES.EncodeKCESExportNameMap(&serializationKCES.KCESExportNameMap{
		Version: serializationKCES.KCESExportNameMapVersion,
		Entries: []serializationKCES.KCESExportNameMapEntry{{InternalName: "GP03_EXPORT.MENU", FileName: "0.MENU"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, native, 0644); err != nil {
		t.Fatal(err)
	}

	output, err := executeCommand(RootCmd, "determine", "--strict", path)
	if err != nil {
		t.Fatalf("determine .enm: %v\n%s", err, output)
	}
	for _, want := range []string{"Type: enm", "Format: json", "Game: KCES", "Version: 1000"} {
		if !strings.Contains(output, want) {
			t.Fatalf("determine output lacks %q:\n%s", want, output)
		}
	}

	if output, err = executeCommand(RootCmd, "convert2json", "--type", "enm", path); err != nil {
		t.Fatalf("convert2json .enm: %v\n%s", err, output)
	}
	jsonPath := path + ".json"
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if output, err = executeCommand(RootCmd, "convert2mod", "--strict", "--type", "enm.json", jsonPath); err != nil {
		t.Fatalf("convert2mod .enm.json: %v\n%s", err, output)
	}
	back, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := serializationKCES.DecodeKCESExportNameMap(back)
	if err != nil || len(decoded.Entries) != 1 || decoded.Entries[0].InternalName != "GP03_EXPORT.MENU" {
		t.Fatalf("command round trip: value=%+v err=%v", decoded, err)
	}
}

func TestKCESExportNameMapNonStrictFilter(t *testing.T) {
	oldStrict, oldType := strictMode, fileType
	t.Cleanup(func() {
		strictMode = oldStrict
		fileType = oldType
	})
	strictMode = false

	fileType = "enm"
	if !fileTypeFilter(`C:\\mods\\export_map.enm`) || fileTypeFilter(`C:\\mods\\export_map.enm.json`) {
		t.Fatal("non-strict enm filter did not separate native and editing paths")
	}
	fileType = "enm.json"
	if !fileTypeFilter(`C:\\mods\\export_map.enm.json`) || fileTypeFilter(`C:\\mods\\export_map.enm`) {
		t.Fatal("non-strict enm.json filter did not separate editing and native paths")
	}
}
