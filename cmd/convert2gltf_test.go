package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
	KCESService "github.com/MeidoPromotionAssociation/MeidoSerialization/service/KCES"
)

// writeTestMMesh 在 dir 中写出一个只有几何体的最小 .mmesh
// writeTestMMesh writes a minimal geometry-only .mmesh into dir
func writeTestMMesh(t *testing.T, dir string, name string) string {
	t.Helper()
	geometry := &aba.MeshGeometry{
		Name:      name,
		Positions: [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
		Primitives: []aba.MeshPrimitive{{
			Mode:    aba.MeshPrimitiveModeTriangles,
			Indices: []uint32{0, 1, 2},
		}},
	}
	object, err := aba.NewNativeMeshObject(name, geometry)
	if err != nil {
		t.Fatalf("build native Mesh %q: %v", name, err)
	}
	data, err := aba.WriteMMesh(object)
	if err != nil {
		t.Fatalf("write native Mesh %q: %v", name, err)
	}
	path := filepath.Join(dir, name+".mmesh")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
	return path
}

// TestConvert2GLTFNotesStandaloneMeshExport 校验直接转换 .mmesh 时会提示改用 .model，且提示每次调用只打印一次
// TestConvert2GLTFNotesStandaloneMeshExport verifies that converting a .mmesh directly points the user at the .model and that the notice is printed once per invocation
func TestConvert2GLTFNotesStandaloneMeshExport(t *testing.T) {
	t.Run("single file", func(t *testing.T) {
		dir := t.TempDir()
		meshPath := writeTestMMesh(t, dir, "notice_single")
		output, err := executeCommand(RootCmd, "convert2gltf", meshPath)
		if err != nil {
			t.Fatalf("convert2gltf failed: %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, "notice_single.glb")); err != nil {
			t.Fatalf("expected exported glb: %v", err)
		}
		if !strings.Contains(output, "exported 1 standalone Mesh file(s) as geometry only") {
			t.Fatalf("standalone Mesh notice is missing: %s", output)
		}
		if !strings.Contains(output, "you should use `convert2gltf` on a .model file") {
			t.Fatalf("notice does not point at the .model input: %s", output)
		}
		if !strings.Contains(output, "你应该对 .model 使用 convert2gltf") {
			t.Fatalf("notice is missing its Chinese wording: %s", output)
		}
	})

	t.Run("directory counts every mesh once", func(t *testing.T) {
		dir := t.TempDir()
		writeTestMMesh(t, dir, "notice_first")
		writeTestMMesh(t, dir, "notice_second")
		output, err := executeCommand(RootCmd, "convert2gltf", dir)
		if err != nil {
			t.Fatalf("convert2gltf failed: %v", err)
		}
		if !strings.Contains(output, "exported 2 standalone Mesh file(s) as geometry only") {
			t.Fatalf("standalone Mesh notice is missing or miscounted: %s", output)
		}
		if count := strings.Count(output, "Note: exported"); count != 1 {
			t.Fatalf("notice printed %d times, want once: %s", count, output)
		}
	})

	t.Run("model input stays silent", func(t *testing.T) {
		sample := filepath.Join("..", "testdata", "KCES", "cm3d2_megane002.aba")
		if _, err := os.Stat(sample); err != nil {
			t.Skipf("sample not available: %v", err)
		}
		root := filepath.Join(t.TempDir(), "unpacked")
		if err := (&KCESService.AbaService{}).UnpackAba(sample, root); err != nil {
			t.Fatalf("unpackAba: %v", err)
		}
		modelFiles, err := filepath.Glob(filepath.Join(root, "TextAsset", "*.model"))
		if err != nil || len(modelFiles) == 0 {
			t.Skipf("no unpacked .model sample: %v", err)
		}
		output, err := executeCommand(RootCmd, "convert2gltf", modelFiles[0])
		if err != nil {
			t.Fatalf("convert2gltf failed: %v", err)
		}
		if strings.Contains(output, "Note: exported") {
			t.Fatalf("a .model export must not print the standalone Mesh notice: %s", output)
		}
	})
}
