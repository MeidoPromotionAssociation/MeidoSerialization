package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMalformedKCESOnlyEditingJSONRoutesToItsValidator(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantError string
	}{
		{
			name:      "paths.dat.JSON",
			body:      `{"format":"wrong","signature":"CM3D2_PATHS","version":1000,"paths":[]}`,
			wantError: "paths.dat",
		},
		{
			name:      "maid_collider.bytes.JSON",
			body:      `{"format":"wrong","colliders":[]}`,
			wantError: "maid collider",
		},
		{
			name:      "broken.ct.JSON",
			body:      `{"format":"kces-content-table","future":1}`,
			wantError: "KCES ct",
		},
		{
			name:      "broken.texture2d.bytes.JSON",
			body:      `{"format":"kces-unity-raw-object","dataBase64":"AQ==","future":1}`,
			wantError: "raw Unity",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), test.name)
			if err := os.WriteFile(path, []byte(test.body), 0644); err != nil {
				t.Fatal(err)
			}
			err := convertToMod(path)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("convertToMod error = %v, want %q validator", err, test.wantError)
			}
		})
	}
}
