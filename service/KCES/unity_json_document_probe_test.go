package KCES

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
	COM3D2Service "github.com/MeidoPromotionAssociation/MeidoSerialization/service/COM3D2"
)

// .undressdat 与 .undresspdat 的原生文件与编辑 JSON 是同一份 Unity JsonUtility 文档，
// 唯一的区分标记是 KCES 独有的双扩展名，因此两条探测路径必须应用同一个领域模型
// The native file and the editing JSON of .undressdat and .undresspdat are the same Unity JsonUtility
// document and the only distinguishing marker is their KCES-only double extension, so both probe paths
// must apply the same domain model

func TestFileTypeServiceRecognizesUndressDocuments(t *testing.T) {
	service := &FileTypeService{}
	documents := map[string]string{
		serializationKCES.KCESUndressDataExtension:      `{"format":"1.2.2","editVer":13,"dataGroup":[{"label":"Group_0000","layer":0,"indices":[7,11]}]}`,
		serializationKCES.KCESUndressPartsDataExtension: `{"editVer":13,"OneGroupLooker":{"Targets":[{"lyr":0,"lbl":"Group_0000"}]},"widthMeasurer":{"d":[]}}`,
	}
	for extension, document := range documents {
		for _, editingSuffix := range []string{"", ".json"} {
			name := "sample" + extension + editingSuffix
			t.Run(name, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), name)
				if err := os.WriteFile(path, []byte(document), 0644); err != nil {
					t.Fatal(err)
				}
				info, matched, err := service.TryFileTypeDetermine(path)
				if err != nil || !matched {
					t.Fatalf("matched=%v info=%+v err=%v", matched, info, err)
				}
				if info.FileType != strings.TrimPrefix(extension, ".") ||
					info.StorageFormat != COM3D2Service.FormatJSON || info.Game != COM3D2Service.GameKCES {
					t.Fatalf("info = %+v, want type %q and KCES JSON", info, strings.TrimPrefix(extension, "."))
				}
			})
		}
	}
}

func TestFileTypeServiceRejectsUnknownMembersInUndressDocuments(t *testing.T) {
	service := &FileTypeService{}
	// 领域结构建模后，未知成员不能再静默通过：JsonUtility.FromJson 会忽略它们，
	// 于是一个拼错的成员名在游戏里表现为该设置完全没有生效
	// Unknown members can no longer pass silently once the domain structure is modeled: JsonUtility.FromJson
	// ignores them, so a misspelled member name shows up in game as the setting simply having no effect
	for _, name := range []string{
		"bad.undressdat",
		"bad.undressdat.json",
		"bad.undresspdat",
		"bad.undresspdat.json",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), name)
			if err := os.WriteFile(path, []byte(`{"editVer":13,"future":1}`), 0644); err != nil {
				t.Fatal(err)
			}
			info, matched, err := service.TryFileTypeDetermine(path)
			if !matched || err == nil || !strings.Contains(err.Error(), "unknown field") {
				t.Fatalf("matched=%v info=%+v err=%v, want unknown-member rejection", matched, info, err)
			}
			if info.FileType != COM3D2Service.UnknownFileType {
				t.Fatalf("invalid undress document was assigned type %q", info.FileType)
			}
		})
	}
}

func TestFileTypeServiceRejectsWrongUndressRootForFileName(t *testing.T) {
	service := &FileTypeService{}
	// 目标格式完全由文件名决定，因此把 .undresspdat 的根写进 .undressdat 必须被拒绝
	// The destination format is decided entirely by the file name, so a .undresspdat root inside a .undressdat must be rejected
	path := filepath.Join(t.TempDir(), "swapped.undressdat")
	if err := os.WriteFile(path, []byte(`{"editVer":13,"widthMeasurer":{"d":[]}}`), 0644); err != nil {
		t.Fatal(err)
	}
	info, matched, err := service.TryFileTypeDetermine(path)
	if !matched || err == nil || !strings.Contains(err.Error(), "widthMeasurer") {
		t.Fatalf("matched=%v info=%+v err=%v, want rejection naming widthMeasurer", matched, info, err)
	}
}
