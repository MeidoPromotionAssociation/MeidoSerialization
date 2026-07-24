package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

func TestKCESBridgeSessionCommandRoutesStrictAndNonStrict(t *testing.T) {
	want := serializationKCES.NewKCESBridgeSession("cli-session")
	want.SessionData.HideMenuFileIDs = []uint64{1, ^uint64(0)}
	want.ContainerDirectories = map[string]ct.VirtualDirectoryMetadata{
		"future": {Version: 1000},
	}
	want.ExtraFiles = map[string][]byte{"future/data": {1, 2, 3}}
	native, err := serializationKCES.EncodeKCESBridgeSession(want)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "bridge_session.vd")
	jsonPath := path + ".json"
	if err := os.WriteFile(path, native, 0644); err != nil {
		t.Fatal(err)
	}

	output, err := executeCommand(RootCmd, "determine", "--strict", path)
	if err != nil {
		t.Fatalf("determine bridge_session.vd: %v\n%s", err, output)
	}
	for _, text := range []string{"Type: bridge_session", "Format: binary", "Game: KCES", "Signature: KCES_VIRTUAL_DIRECTORY", "Version: 1000"} {
		if !strings.Contains(output, text) {
			t.Fatalf("determine output lacks %q:\n%s", text, output)
		}
	}

	if output, err = executeCommand(RootCmd, "convert2json", "--strict", "--type", "bridge_session", path); err != nil {
		t.Fatalf("convert2json bridge_session.vd: %v\n%s", err, output)
	}
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if output, err = executeCommand(RootCmd, "convert2mod", "--strict", "--type", "bridge_session.json", jsonPath); err != nil {
		t.Fatalf("convert2mod bridge_session.vd.json: %v\n%s", err, output)
	}
	got, err := serializationKCES.DecodeKCESBridgeSession(mustReadFile(t, path))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CLI round trip changed bridge session:\n got  %#v\n want %#v", got, want)
	}

	oldStrict, oldType := strictMode, fileType
	t.Cleanup(func() {
		strictMode = oldStrict
		fileType = oldType
	})
	strictMode = false
	fileType = "bridge_session"
	if !fileTypeFilter(`C:\work\bridge_session.vd`) || fileTypeFilter(`C:\work\other.vd`) || fileTypeFilter(`C:\work\bridge_session.vd.json`) {
		t.Fatal("non-strict bridge_session filter mismatch")
	}
	fileType = "bridge_session.json"
	if !fileTypeFilter(`C:\work\bridge_session.vd.json`) || !fileTypeFilter(`C:\work\BRIDGE_SESSION.VD.JSON`) || fileTypeFilter(`C:\work\bridge_session.vd`) {
		t.Fatal("non-strict bridge_session.json filter mismatch")
	}
}
