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

func TestKCESJSONTextEditingEnvelopeStrictValidation(t *testing.T) {
	valid := []byte(`{"extension":".nson","json":{"version":1000}}`)
	value, err := decodeKCESJSONTextEditingJSON(valid, ".nson")
	if err != nil {
		t.Fatalf("valid envelope: %v", err)
	}
	if value.Extension != ".nson" || string(value.JSON) != `{"version":1000}` {
		t.Fatalf("unexpected decoded envelope: %+v", value)
	}

	for name, data := range map[string][]byte{
		"invalid UTF-8":        append([]byte(`{"extension":".nson","json":"`), 0xff, '"', '}'),
		"unknown field":        []byte(`{"extension":".nson","json":{},"future":1}`),
		"second JSON value":    []byte(`{"extension":".nson","json":{}} {}`),
		"missing payload":      []byte(`{"extension":".nson"}`),
		"mismatched extension": []byte(`{"extension":".undressdat","json":{}}`),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeKCESJSONTextEditingJSON(data, ".nson"); err == nil {
				t.Fatalf("accepted invalid envelope %q", data)
			}
		})
	}

	for name, data := range map[string][]byte{
		"missing extension defaults": []byte(`{"json":null}`),
		"empty extension defaults":   []byte(`{"extension":"","json":[]}`),
		"BOM":                        append([]byte{0xef, 0xbb, 0xbf}, valid...),
	} {
		t.Run(name, func(t *testing.T) {
			value, err := decodeKCESJSONTextEditingJSON(data, ".nson")
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if value.Extension != ".nson" {
				t.Fatalf("extension = %q", value.Extension)
			}
		})
	}
}

func TestFileTypeServiceStrictlyValidatesJSONTextEditingEnvelope(t *testing.T) {
	service := &FileTypeService{}
	for name, data := range map[string][]byte{
		"unknown":  []byte(`{"extension":".nson","json":{},"future":1}`),
		"mismatch": []byte(`{"extension":".undressdat","json":{}}`),
		"trailing": []byte(`{"extension":".nson","json":{}} []`),
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

	path := filepath.Join(t.TempDir(), "probe.nson.JSON")
	valid := append([]byte{0xef, 0xbb, 0xbf}, []byte(`{"extension":".NSON","json":{"ok":true}}`)...)
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

	bad := bytes.Replace(valid, []byte(`.NSON`), []byte(`.undressdat`), 1)
	if err := os.WriteFile(path, bad, 0644); err != nil {
		t.Fatal(err)
	}
	if err := (&MiscService{}).ConvertJsonToMisc(TestConversionContext, path, out, TestConversionMaxOutput); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("service accepted mismatched extension: %v", err)
	}
}

func TestFileTypeServiceRejectsUnknownFieldsInKCESOnlyEditingJSON(t *testing.T) {
	tests := map[string]string{
		"bad.menuassets.json":      `{"assetArray":[],"future":1}`,
		"bad.db2conf.json":         `{"format":"kces-msgpack-lz4","extension":".db2conf","storageVariant":"int32-length-lz4-messagepack","kind":"msgpack-json-string","json":{},"future":1}`,
		"bad.hitcheck.json":        `{"signature":"HitCheck","entries":[],"future":1}`,
		"paths.dat.json":           `{"format":"kces-auto-paths","signature":"CM3D2_PATHS","version":1000,"paths":["system"],"future":1}`,
		"system.dat.json":          `{"format":"kces-system-data","version":1000,"future":1}`,
		"maid_collider.bytes.json": `{"format":"kces-maid-capsule-colliders","colliders":[],"future":1}`,
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
