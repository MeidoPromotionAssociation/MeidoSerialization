package CRC3D3

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	serializationCRC3D3 "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/CRC3D3"
)

// neimpSamplePath 是一个真实的 .neimp 样本，位于 CRC3D3 的 csv.aba 解包结果中
// neimpSamplePath is a real .neimp sample taken from the unpacked CRC3D3 csv.aba
const neimpSamplePath = "../../testdata/CRC3D3/csv.aba_unpacked/TextAsset/cast_data.neimp"

// TestIsCRC3D3NeimpFile 验证 .neimp 原生文件与JSON 的路径判定
// TestIsCRC3D3NeimpFile verifies path detection for native .neimp files and their JSON
func TestIsCRC3D3NeimpFile(t *testing.T) {
	native := []string{"cast_data.neimp", "dir/CAST_DATA.NEIMP"}
	editingJSON := []string{"cast_data.neimp.json", "dir/Cast_Data.Neimp.JSON"}
	other := []string{"cast_data.nei", "cast_data.neimp.bak", "cast_data.csv", "cast_data.json"}

	for _, path := range native {
		if !IsCRC3D3NeimpFile(path) {
			t.Fatalf("IsCRC3D3NeimpFile(%q) = false", path)
		}
		if IsCRC3D3NeimpJSONFile(path) {
			t.Fatalf("IsCRC3D3NeimpJSONFile(%q) = true", path)
		}
	}
	for _, path := range editingJSON {
		if !IsCRC3D3NeimpJSONFile(path) {
			t.Fatalf("IsCRC3D3NeimpJSONFile(%q) = false", path)
		}
		if IsCRC3D3NeimpFile(path) {
			t.Fatalf("IsCRC3D3NeimpFile(%q) = true", path)
		}
	}
	for _, path := range other {
		if IsCRC3D3NeimpFile(path) || IsCRC3D3NeimpJSONFile(path) {
			t.Fatalf("%q was detected as a .neimp path", path)
		}
	}
}

// TestNeimpServiceRoundTrip 验证 service 层的文件级往返
// TestNeimpServiceRoundTrip verifies the file-level round trip of the service layer
func TestNeimpServiceRoundTrip(t *testing.T) {
	ctx := context.Background()
	service := &NeimpService{}
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
	if err := service.ConvertNeimpToJSON(ctx, nativePath, jsonPath, 1<<20); err != nil {
		t.Fatalf("ConvertNeimpToJSON: %v", err)
	}
	restoredPath := filepath.Join(directory, "restored.neimp")
	if err := service.ConvertJSONToNeimp(ctx, jsonPath, restoredPath, 1<<20); err != nil {
		t.Fatalf("ConvertJSONToNeimp: %v", err)
	}

	original, err := service.ReadNeimpFile(nativePath)
	if err != nil {
		t.Fatalf("ReadNeimpFile: %v", err)
	}
	restored, err := service.ReadNeimpFile(restoredPath)
	if err != nil {
		t.Fatalf("ReadNeimpFile restored: %v", err)
	}
	originalJSON, err := serializationCRC3D3.EncodeNeimpJSON(original)
	if err != nil {
		t.Fatalf("EncodeNeimpJSON original: %v", err)
	}
	restoredJSON, err := serializationCRC3D3.EncodeNeimpJSON(restored)
	if err != nil {
		t.Fatalf("EncodeNeimpJSON restored: %v", err)
	}
	if string(originalJSON) != string(restoredJSON) {
		t.Fatalf("restored .neimp does not match the original")
	}
}

// TestNeimpServiceRejectsOutputOverLimit 验证超出输出上限时返回约定的错误
// TestNeimpServiceRejectsOutputOverLimit verifies that exceeding the output limit returns the agreed error
func TestNeimpServiceRejectsOutputOverLimit(t *testing.T) {
	ctx := context.Background()
	service := &NeimpService{}
	directory := t.TempDir()
	outputPath := filepath.Join(directory, "cast_data.neimp.json")
	err := service.ConvertNeimpToJSON(ctx, neimpSamplePath, outputPath, 16)
	if !errors.Is(err, ErrConversionOutputLimitExceeded) {
		t.Fatalf("ConvertNeimpToJSON error = %v, want ErrConversionOutputLimitExceeded", err)
	}
	if _, statErr := os.Stat(outputPath); !os.IsNotExist(statErr) {
		t.Fatalf("output must not be written when the limit is exceeded")
	}
}
