package KCES

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

func TestUndressPairExtension(t *testing.T) {
	tests := map[string]string{
		"crc2_Underwear038_pants.undressdat":      serializationKCES.KCESUndressPartsDataExtension,
		"crc2_Underwear038_pants.UNDRESSPDAT":     serializationKCES.KCESUndressDataExtension,
		"crc2_Underwear038_pants.undressdat.json": "",
		"dance_enabled_list.nson":                 "",
		"default_hairf.dbconf":                    "",
	}
	for input, want := range tests {
		if got := UndressPairExtension(input); got != want {
			t.Fatalf("UndressPairExtension(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestMissingUndressPairWarning(t *testing.T) {
	// 游戏两个文件都读，任一缺失就中止整套脱衣设置，因此缺配对是提示而不是错误
	// The game reads both files and aborts the whole undress setup when either is missing, so a missing pair is a hint rather than an error
	dir := t.TempDir()
	lonely := filepath.Join(dir, "crc2_Underwear038_pants.undressdat")
	if err := os.WriteFile(lonely, []byte(`{"format":"1.2.2"}`), 0644); err != nil {
		t.Fatal(err)
	}
	warning := MissingUndressPairWarning(lonely)
	if !strings.Contains(warning, "crc2_Underwear038_pants.undresspdat") || !strings.Contains(warning, "without any peel behavior") {
		t.Fatalf("warning for a lonely .undressdat = %q", warning)
	}

	// 配对文件只有编辑 JSON 形式时也算存在，否则分两步转换一对文件会误报
	// The paired file counts as present in editing JSON form too, otherwise converting a pair in two steps reports a false positive
	editingPair := filepath.Join(dir, "crc2_Underwear038_pants.undresspdat.json")
	if err := os.WriteFile(editingPair, []byte(`{"editVer":13}`), 0644); err != nil {
		t.Fatal(err)
	}
	if warning := MissingUndressPairWarning(lonely); warning != "" {
		t.Fatalf("warning while the paired editing JSON exists = %q", warning)
	}

	if err := os.Rename(editingPair, filepath.Join(dir, "crc2_Underwear038_pants.undresspdat")); err != nil {
		t.Fatal(err)
	}
	if warning := MissingUndressPairWarning(lonely); warning != "" {
		t.Fatalf("warning while the paired native file exists = %q", warning)
	}

	// 其他格式不参与配对检查
	// Other formats take no part in the pair check
	other := filepath.Join(dir, "dance_enabled_list.nson")
	if err := os.WriteFile(other, []byte(`{"version":1000}`), 0644); err != nil {
		t.Fatal(err)
	}
	if warning := MissingUndressPairWarning(other); warning != "" {
		t.Fatalf("warning for an unrelated format = %q", warning)
	}
}

func TestUndressPairWarningsReportHalfPackedPairs(t *testing.T) {
	manifest := ModManifest{Assets: []ModAsset{
		{Name: "crc2_Underwear038_pants.undressdat"},
		{Name: "crc2_Underwear038_pants.undresspdat"},
		{Name: "crc2_Underwear001_pants.undressdat"},
		{Name: "crc2_Underwear002_pants.undresspdat"},
		{Name: "parts.menuassets"},
	}}
	warnings := undressPairWarnings(manifest)
	if len(warnings) != 2 {
		t.Fatalf("undressPairWarnings returned %d warnings: %v", len(warnings), warnings)
	}
	joined := strings.Join(warnings, "\n")
	for _, want := range []string{
		"crc2_Underwear001_pants.undresspdat",
		"crc2_Underwear002_pants.undressdat",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("warnings do not name the missing %s: %v", want, warnings)
		}
	}
	if strings.Contains(joined, "crc2_Underwear038") {
		t.Fatalf("a complete pair was reported: %v", warnings)
	}
}
