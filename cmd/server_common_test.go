package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/application"
)

func TestTransportCommandsAreRegistered(t *testing.T) {
	for _, path := range [][]string{{"serve", "grpc"}, {"mcp"}} {
		command, _, err := RootCmd.Find(path)
		if err != nil || command == nil {
			t.Fatalf("RootCmd.Find(%v) = %v, %v", path, command, err)
		}
	}
}

func TestValidateGRPCListenAddress(t *testing.T) {
	for _, address := range []string{"127.0.0.1:50051", "[::1]:50051", "localhost:50051"} {
		if err := validateGRPCListenAddress(address, false); err != nil {
			t.Fatalf("loopback %q rejected: %v", address, err)
		}
	}
	for _, address := range []string{"0.0.0.0:50051", "192.0.2.1:50051", ":50051"} {
		if err := validateGRPCListenAddress(address, false); err == nil {
			t.Fatalf("remote address %q accepted without opt-in", address)
		}
		if err := validateGRPCListenAddress(address, true); err != nil {
			t.Fatalf("remote address %q rejected with opt-in: %v", address, err)
		}
	}
}

func TestConfiguredRoots(t *testing.T) {
	directory := t.TempDir()
	writeDirectory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "sample.menu"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	roots, err := configuredRoots([]string{"mods=" + directory})
	if err != nil {
		t.Fatal(err)
	}
	defer roots.Close()
	if _, err := roots.Resolve("mods", "sample.menu"); err != nil {
		t.Fatalf("configured root cannot resolve file: %v", err)
	}
	if _, _, err := roots.WriteFile(context.Background(), "mods", "out.bin", bytes.NewReader([]byte("x")), 16); application.CodeOf(err) != application.CodePermissionDenied {
		t.Fatalf("read-only root WriteFile error = %v", err)
	}
	writable, err := configuredRootsWithWrites(nil, []string{"mods=" + writeDirectory})
	if err != nil {
		t.Fatalf("configured writable root: %v", err)
	}
	defer writable.Close()
	if got := writable.WritableIDs(); len(got) != 1 || got[0] != "mods" {
		t.Fatalf("writable root IDs = %v", got)
	}
	if _, _, err := writable.WriteFile(context.Background(), "mods", "out.bin", bytes.NewReader([]byte("x")), 16); err != nil {
		t.Fatalf("writable root WriteFile: %v", err)
	}
	if _, err := configuredRoots([]string{"missing-separator"}); err == nil {
		t.Fatal("malformed root specification was accepted")
	}
	if duplicate, err := configuredRoots([]string{"mods=" + directory, "mods=" + directory}); err == nil {
		_ = duplicate.Close()
		t.Fatal("duplicate root ID was accepted")
	}
	if duplicate, err := configuredRootsWithWrites([]string{"mods=" + directory}, []string{"mods=" + writeDirectory}); err == nil {
		_ = duplicate.Close()
		t.Fatal("root ID shared by --root and --write-root was accepted")
	}
}

func TestMCPFilesystemModeSelection(t *testing.T) {
	tests := []struct {
		name          string
		restrictPaths bool
		readRoots     []string
		writeRoots    []string
		want          string
	}{
		{name: "default convenience mode", want: "unrestricted"},
		{name: "explicit restriction", restrictPaths: true, want: "restricted"},
		{name: "read root implies restriction", readRoots: []string{"mods=C:\\mods"}, want: "restricted"},
		{name: "write root implies restriction", writeRoots: []string{"work=C:\\work"}, want: "restricted"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := string(mcpFilesystemMode(test.restrictPaths, test.readRoots, test.writeRoots)); got != test.want {
				t.Fatalf("mcpFilesystemMode() = %q, want %q", got, test.want)
			}
		})
	}
	if flag := newMCPCmd().Flags().Lookup("restrict-paths"); flag == nil || flag.DefValue != "false" {
		t.Fatalf("--restrict-paths flag = %+v", flag)
	}
}
