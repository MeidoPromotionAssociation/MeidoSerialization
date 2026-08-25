package KCES

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	COM3D2Service "github.com/MeidoPromotionAssociation/MeidoSerialization/service/COM3D2"
)

func TestFileTypeServiceKeepsMalformedNativeJSONTextAsKCESCandidate(t *testing.T) {
	for _, extension := range []string{".undressdat", ".undresspdat", ".nson"} {
		for _, editingSuffix := range []string{"", ".json"} {
			name := extension + editingSuffix
			t.Run(name, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "malformed"+name)
				if err := os.WriteFile(path, []byte(`{"truncated":`), 0644); err != nil {
					t.Fatal(err)
				}
				info, matched, err := (&FileTypeService{}).TryFileTypeDetermine(path)
				if !matched || err == nil {
					t.Fatalf("malformed JSON text candidate fell through: matched=%v info=%+v err=%v", matched, info, err)
				}
				if info.FileType != COM3D2Service.UnknownFileType {
					t.Fatalf("malformed JSON text candidate was assigned type %q", info.FileType)
				}
			})
		}
	}
}

func TestFileTypeServiceKeepsMalformedKCESOnlyEditingJSONAsCandidate(t *testing.T) {
	for _, name := range []string{
		"bad.menuassets.json",
		"bad.materialassets.json",
		"bad.pmatassets.json",
		"bad.dbconf.json",
		"bad.db2conf.json",
		"bad.hitcheck.json",
		"paths.dat.json",
		"system.dat.json",
		"maid_collider.bytes.json",
		"bad.ct.json",
		"bad.texture2d.bytes.json",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), name)
			if err := os.WriteFile(path, []byte(`{"truncated":`), 0644); err != nil {
				t.Fatal(err)
			}
			info, matched, err := (&FileTypeService{}).TryFileTypeDetermine(path)
			if !matched || err == nil {
				t.Fatalf("malformed KCES-only editing JSON fell through: matched=%v info=%+v err=%v", matched, info, err)
			}
			if info.FileType != COM3D2Service.UnknownFileType {
				t.Fatalf("malformed KCES-only editing JSON was assigned type %q", info.FileType)
			}
		})
	}
}

func TestKCESJSONTextEditingDocumentStrictValidation(t *testing.T) {
	valid := []byte(`{"version":1000}`)
	value, err := decodeKCESJSONTextEditingJSON(valid, ".nson")
	if err != nil {
		t.Fatalf("valid document: %v", err)
	}
	if string(value) != `{"version":1000}` {
		t.Fatalf("unexpected decoded document: %s", value)
	}

	for name, data := range map[string][]byte{
		"invalid UTF-8":     append([]byte(`{"version":"`), 0xff, '"', '}'),
		"second JSON value": []byte(`{"version":1000} {}`),
		"empty document":    []byte("  "),
		"trailing garbage":  []byte(`{"version":1000} oops`),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeKCESJSONTextEditingJSON(data, ".nson"); err == nil {
				t.Fatalf("accepted invalid document %q", data)
			}
		})
	}

	// 编辑封套已移除，根就是资源自身的 JSON 文档，所以任何合法 JSON 值都可以是根
	// The editing envelope was removed and the root is the resource's own JSON document, so any valid JSON value can be the root
	for name, data := range map[string][]byte{
		"object": []byte(`{"version":1000}`),
		"array":  []byte(`[1,2,3]`),
		"null":   []byte(`null`),
		"BOM":    append([]byte{0xef, 0xbb, 0xbf}, valid...),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeKCESJSONTextEditingJSON(trimJSONUTF8BOM(data), ".nson"); err != nil {
				t.Fatalf("decode: %v", err)
			}
		})
	}
}

func TestFileTypeServiceStrictlyValidatesJSONTextEditingDocument(t *testing.T) {
	service := &FileTypeService{}
	for name, data := range map[string][]byte{
		"trailing value":   []byte(`{"ok":true} []`),
		"truncated":        []byte(`{"ok":`),
		"empty":            []byte("   "),
		"trailing garbage": []byte(`{"ok":true} oops`),
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "probe.nson.JSON")
			if err := os.WriteFile(path, data, 0644); err != nil {
				t.Fatal(err)
			}
			info, matched, err := service.TryFileTypeDetermine(path)
			if !matched || err == nil || info.FileType != COM3D2Service.UnknownFileType {
				t.Fatalf("matched=%v info=%+v err=%v", matched, info, err)
			}
		})
	}

	// 目标扩展名完全由文件名决定，编辑 JSON 的根就是资源文档本身
	// The destination extension is decided entirely by the file name and the editing JSON root is the resource document itself
	path := filepath.Join(t.TempDir(), "probe.nson.JSON")
	valid := append([]byte{0xef, 0xbb, 0xbf}, []byte(`{"ok":true}`)...)
	if err := os.WriteFile(path, valid, 0644); err != nil {
		t.Fatal(err)
	}
	info, matched, err := service.TryFileTypeDetermine(path)
	if err != nil || !matched || info.FileType != "nson" || info.StorageFormat != COM3D2Service.FormatJSON {
		t.Fatalf("matched=%v info=%+v err=%v", matched, info, err)
	}

	out := filepath.Join(t.TempDir(), "out.nson")
	if err := (&MiscService{}).ConvertJsonToMisc(TestConversionContext, path, out, TestConversionMaxOutput); err != nil {
		t.Fatalf("ConvertJsonToMisc: %v", err)
	}
	if got := bytes.TrimSpace(mustReadTestFile(t, out)); !bytes.Equal(got, []byte("{\n  \"ok\": true\n}")) {
		t.Fatalf("unexpected native output: %q", got)
	}
}

func TestFileTypeServiceRejectsUnknownFieldsInKCESOnlyEditingJSON(t *testing.T) {
	tests := map[string]string{
		"bad.menuassets.json":      `{"assetArray":[],"future":1}`,
		"bad.db2conf.json":         `{"format":"kces-msgpack-lz4","extension":".db2conf","storageVariant":"int32-length-lz4-messagepack","kind":"msgpack-json-string","json":{},"future":1}`,
		"bad.hitcheck.json":        `{"signature":"HitCheck","entries":[],"future":1}`,
		"paths.dat.json":           `{"signature":"CM3D2_PATHS","version":1000,"paths":["system"],"future":1}`,
		"system.dat.json":          `{"version":1000,"future":1}`,
		"maid_collider.bytes.json": `{"colliders":[],"future":1}`,
		"bad.ct.json":              `{"format":"kces-content-table","future":1}`,
		"bad.texture2d.bytes.json": `{"format":"kces-unity-raw-object","dataBase64":"AQ==","future":1}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), name)
			if err := os.WriteFile(path, []byte(body), 0644); err != nil {
				t.Fatal(err)
			}
			info, matched, err := (&FileTypeService{}).TryFileTypeDetermine(path)
			if !matched || err == nil || !strings.Contains(err.Error(), "unknown field") {
				t.Fatalf("matched=%v info=%+v err=%v, want unknown-field rejection", matched, info, err)
			}
			if info.FileType != COM3D2Service.UnknownFileType {
				t.Fatalf("invalid editing JSON was assigned type %q", info.FileType)
			}
		})
	}
}

func TestKCESPayloadEditingJSONBindsRootTypeToFileName(t *testing.T) {
	// 编辑封套已移除，目标格式完全由文件名决定，因此载荷根必须与文件名声明的载荷类型一致
	// The editing envelope was removed and the destination format is decided entirely by the file name,
	// so the payload root must match the payload type that file name declares
	magica := []byte(`{"clothType":1}`)
	if _, err := decodeKCESPayloadEditingJSON(magica, ".db2conf"); err != nil {
		t.Fatalf("MagicaCloth2 root on .db2conf: %v", err)
	}
	if _, err := decodeKCESPayloadEditingJSON(magica, ".dbconf"); err == nil ||
		!strings.Contains(err.Error(), "clothType") {
		t.Fatalf("MagicaCloth2 root on .dbconf error = %v, want unknown-field rejection", err)
	}

	envelope := []byte(`{"format":"kces-msgpack-lz4","extension":".db2conf","storageVariant":"int32-length-lz4-messagepack","kind":"msgpack-json-string","json":{}}`)
	if _, err := decodeKCESPayloadEditingJSON(envelope, ".db2conf"); err == nil ||
		!strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("removed editing envelope error = %v, want unknown-field rejection", err)
	}
}
