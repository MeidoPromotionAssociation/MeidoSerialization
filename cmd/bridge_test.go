package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/binaryio/stream"
)

func TestKCESGP03BridgeCommandRoutes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gp03_export_maid.brd")
	native, err := serializationKCES.EncodeGP03Bridge(sourceConstructedCommandBridge(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, native, 0644); err != nil {
		t.Fatal(err)
	}

	output, err := executeCommand(RootCmd, "determine", "--strict", path)
	if err != nil {
		t.Fatalf("determine .brd: %v\n%s", err, output)
	}
	for _, want := range []string{"Type: brd", "Format: binary", "Game: KCES", "Signature: GP03_BRIDGE", "Version: 2001"} {
		if !strings.Contains(output, want) {
			t.Fatalf("determine output lacks %q:\n%s", want, output)
		}
	}

	if output, err = executeCommand(RootCmd, "convert2json", "--strict", "--type", "brd", path); err != nil {
		t.Fatalf("convert2json .brd: %v\n%s", err, output)
	}
	jsonPath := path + ".json"
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if output, err = executeCommand(RootCmd, "convert2mod", "--strict", "--type", "brd.json", jsonPath); err != nil {
		t.Fatalf("convert2mod .brd.json: %v\n%s", err, output)
	}
	back, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(back, native) {
		t.Fatal("command bridge round-trip changed deterministic wire bytes")
	}
	decoded, err := serializationKCES.DecodeGP03Bridge(back)
	if err != nil || decoded.GUID != "command-source-guid" {
		t.Fatalf("command bridge round-trip value=%+v err=%v", decoded, err)
	}
}

func TestKCESGP03BridgeNonStrictFilter(t *testing.T) {
	oldStrict, oldType := strictMode, fileType
	t.Cleanup(func() {
		strictMode = oldStrict
		fileType = oldType
	})
	strictMode = false

	fileType = "brd"
	if !fileTypeFilter(`C:\mods\gp03_export_maid.brd`) || fileTypeFilter(`C:\mods\gp03_export_maid.brd.json`) {
		t.Fatal("non-strict brd filter did not separate native and editing paths")
	}
	fileType = "brd.json"
	if !fileTypeFilter(`C:\mods\gp03_export_maid.brd.json`) || fileTypeFilter(`C:\mods\gp03_export_maid.brd`) {
		t.Fatal("non-strict brd.json filter did not separate editing and native paths")
	}
}

func sourceConstructedCommandBridge(t *testing.T) *serializationKCES.GP03BridgeFile {
	t.Helper()
	var legacy bytes.Buffer
	bw := stream.NewBinaryWriter(&legacy)
	for _, write := range []func() error{
		func() error { return bw.WriteString("CM3D2_PRESET") },
		func() error { return bw.WriteInt32(2001) },
		func() error { return bw.WriteInt32(2) },
		func() error { return bw.WriteInt32(0) },
		func() error { return bw.WriteString("CM3D2_MPROP_LIST") },
		func() error { return bw.WriteInt32(2001) },
		func() error { return bw.WriteInt32(0) },
		func() error { return bw.WriteInt32(0) },
		func() error { return bw.WriteInt32(0) },
		func() error { return bw.WriteInt32(0) },
	} {
		if err := write(); err != nil {
			t.Fatal(err)
		}
	}
	core, err := serializationKCES.NewKCESPresetCore()
	if err != nil {
		t.Fatal(err)
	}
	current, err := serializationKCES.EncodeKCESPreset(&serializationKCES.KCESPreset{
		ContainerVersion: 1000,
		Thumbnail:        []byte{1},
		MaidData:         core,
	})
	if err != nil {
		t.Fatal(err)
	}
	return &serializationKCES.GP03BridgeFile{
		Signature:     serializationKCES.GP03BridgeSignature,
		Version:       serializationKCES.GP03BridgeVersion,
		GUID:          "command-source-guid",
		LegacyPreset:  legacy.Bytes(),
		CurrentPreset: current,
	}
}
