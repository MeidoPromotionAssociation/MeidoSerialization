package CRC3D3

import (
	"os"
	"path/filepath"
	"testing"

	COM3D2Service "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/service/COM3D2"
)

// TestFileTypeServiceDetectsNeimp 验证 .neimp 与其 JSON 的类型判定
// TestFileTypeServiceDetectsNeimp verifies type detection for .neimp and its JSON
func TestFileTypeServiceDetectsNeimp(t *testing.T) {
	service := &FileTypeService{}
	directory := t.TempDir()

	nativePath := filepath.Join(directory, "cast_data.neimp")
	data, err := os.ReadFile(neimpSamplePath)
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}
	if err := os.WriteFile(nativePath, data, 0644); err != nil {
		t.Fatalf("write sample: %v", err)
	}
	jsonPath := nativePath + ".json"
	if err := (&NeimpService{}).ConvertNeimpToJSON(t.Context(), nativePath, jsonPath, 1<<20); err != nil {
		t.Fatalf("ConvertNeimpToJSON: %v", err)
	}

	tests := []struct {
		name          string
		path          string
		wantMatched   bool
		wantFormat    string
		wantSizeMatch bool
	}{
		{name: "native", path: nativePath, wantMatched: true, wantFormat: COM3D2Service.FormatBinary, wantSizeMatch: true},
		{name: "json", path: jsonPath, wantMatched: true, wantFormat: COM3D2Service.FormatJSON, wantSizeMatch: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			info, matched, err := service.TryFileTypeDetermine(test.path)
			if err != nil {
				t.Fatalf("TryFileTypeDetermine: %v", err)
			}
			if matched != test.wantMatched {
				t.Fatalf("matched = %v, want %v", matched, test.wantMatched)
			}
			if info.FileType != NeimpFileType {
				t.Fatalf("FileType = %q, want %q", info.FileType, NeimpFileType)
			}
			if info.Game != GameCRC3D3 {
				t.Fatalf("Game = %q, want %q", info.Game, GameCRC3D3)
			}
			if info.StorageFormat != test.wantFormat {
				t.Fatalf("StorageFormat = %q, want %q", info.StorageFormat, test.wantFormat)
			}
			stat, statErr := os.Stat(test.path)
			if statErr != nil {
				t.Fatalf("stat: %v", statErr)
			}
			if test.wantSizeMatch && info.Size != stat.Size() {
				t.Fatalf("Size = %d, want %d", info.Size, stat.Size())
			}
		})
	}
}

// TestFileTypeServiceRejectsOtherFiles 验证其它后缀与损坏的 .neimp 不会被认领
// TestFileTypeServiceRejectsOtherFiles verifies that other suffixes and a corrupt .neimp are not claimed
func TestFileTypeServiceRejectsOtherFiles(t *testing.T) {
	service := &FileTypeService{}
	directory := t.TempDir()

	corruptPath := filepath.Join(directory, "broken.neimp")
	if err := os.WriteFile(corruptPath, []byte{0x93, 0x01}, 0644); err != nil {
		t.Fatalf("write corrupt sample: %v", err)
	}
	otherPath := filepath.Join(directory, "table.nei")
	if err := os.WriteFile(otherPath, []byte{0x00}, 0644); err != nil {
		t.Fatalf("write other sample: %v", err)
	}

	for _, path := range []string{corruptPath, otherPath} {
		info, matched, err := service.TryFileTypeDetermine(path)
		if err != nil {
			t.Fatalf("TryFileTypeDetermine(%q): %v", path, err)
		}
		if matched {
			t.Fatalf("TryFileTypeDetermine(%q) matched as %+v", path, info)
		}
	}

	// 以 .neimp 结尾的目录必须报错，不能当成可读文件
	// A directory whose name ends with .neimp has to fail instead of being treated as a readable file
	directoryPath := filepath.Join(directory, "folder.neimp")
	if err := os.Mkdir(directoryPath, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if _, _, err := service.TryFileTypeDetermine(directoryPath); err == nil {
		t.Fatalf("TryFileTypeDetermine on a directory must fail")
	}
}
