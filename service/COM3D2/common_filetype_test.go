package COM3D2

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStrictFileTypeCSVDetectionRequiresValidTextAndStructure(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{name: "valid.csv", data: []byte("\xef\xbb\xbfname,value\nfoo,1\n"), want: "csv"},
		{name: "quoted.csv", data: []byte("name,value\nfoo,\"line 1\nline 2\"\n"), want: "csv"},
		{name: "plain.txt", data: []byte("ordinary text without a delimiter\n"), want: UnknownFileType},
		{name: "binary.bin", data: []byte{0x92, 0xc7, 0x03, 0x62, 0x00, ',', 0xff}, want: UnknownFileType},
		{name: "inconsistent.csv", data: []byte("a,b\nonly-one-field\n"), want: UnknownFileType},
	}

	service := &CommonService{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(dir, test.name)
			if err := os.WriteFile(path, test.data, 0644); err != nil {
				t.Fatal(err)
			}
			info, err := service.FileTypeDetermine(path, true)
			if err != nil {
				t.Fatalf("FileTypeDetermine: %v", err)
			}
			if info.FileType != test.want {
				t.Fatalf("FileType = %q, want %q (info=%+v)", info.FileType, test.want, info)
			}
		})
	}
}

func TestLegacyStrictDetectorDoesNotCallKCESBinaryCSV(t *testing.T) {
	service := &CommonService{}
	for _, path := range []string{
		filepath.Join("..", "..", "testdata", "aba", "cm3d2_eyes.aba"),
		filepath.Join("..", "..", "testdata", "kces_parts", "cm3d2_eyes.menuassets"),
		filepath.Join("..", "..", "testdata", "kces_payload", "default_hairf.dbconf"),
		filepath.Join("..", "..", "testdata", "kces_misc", "IK.hitcheck"),
	} {
		info, err := service.FileTypeDetermine(path, true)
		if err != nil {
			t.Fatalf("FileTypeDetermine(%q): %v", path, err)
		}
		if info.FileType == "csv" {
			t.Fatalf("binary KCES file %q was misidentified as CSV: %+v", path, info)
		}
	}
}

func TestTryFileTypeDetermineMatchesCOM3D2BeforeKCESFallback(t *testing.T) {
	service := &CommonService{}
	for _, path := range []string{
		filepath.Join("..", "..", "testdata", "test.model"),
		filepath.Join("..", "..", "testdata", "test.preset"),
	} {
		info, matched, err := service.TryFileTypeDetermine(path)
		if err != nil {
			t.Fatalf("TryFileTypeDetermine(%q): %v", path, err)
		}
		if !matched || info.Game != GameCOM3D2 || info.FileType == UnknownFileType {
			t.Fatalf("COM3D2 file %q was not matched by fast probe: matched=%v info=%+v", path, matched, info)
		}
	}

	for _, path := range []string{
		filepath.Join("..", "..", "testdata", "kces_parts", "cm3d2_megane002.model"),
		filepath.Join("..", "..", "testdata", "aba", "cm3d2_eyes.aba"),
	} {
		info, matched, err := service.TryFileTypeDetermine(path)
		if err != nil {
			t.Fatalf("TryFileTypeDetermine(%q): %v", path, err)
		}
		if matched {
			t.Fatalf("KCES file %q was consumed by COM3D2 fast probe: %+v", path, info)
		}
	}
}
