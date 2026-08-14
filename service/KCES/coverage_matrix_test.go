package KCES

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/internal/kcesfixtures"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
)

func TestKCESKnownExtensionRoutingMatrix(t *testing.T) {
	ctPaths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "KCES", "*.ct"))
	if err != nil {
		t.Fatalf("glob .ct route samples: %v", err)
	}
	if len(ctPaths) == 0 {
		t.Fatalf("no .ct route samples")
	}
	assertRoutePaths(t, ctPaths, ".ct", IsKCESCtFile)
	assertGeneratedCtJSONRouteSamples(t, ctPaths)

	partsPaths := []string{
		kcesfixtures.TextAssetPath(t, "cm3d2_megane002.aba", "cm3d2_megane002.model"),
		kcesfixtures.TextAssetPath(t, "cm3d2_eyes.aba", "cm3d2_eyes.menuassets"),
		kcesfixtures.TextAssetPath(t, "parts_personal002.aba", "parts_personal002.materialassets"),
		kcesfixtures.TextAssetPath(t, "partsmeta.aba", "partsmeta.pmatassets"),
	}
	assertRoutePaths(t, partsPaths, "parts", IsKCESPartsFile)

	payloadPaths := []string{
		kcesfixtures.TextAssetPath(t, "partsmeta.aba", "default_hairf.dbconf"),
		kcesfixtures.TextAssetPath(t, "partsmeta.aba", "default_hairf.db2conf"),
		kcesfixtures.TextAssetPath(t, "system.aba", "maidIKCollider.ikcol"),
	}
	assertRoutePaths(t, payloadPaths, "payload", IsKCESPayloadFile)

	miscPaths := []string{
		kcesfixtures.TextAssetPath(t, "system.aba", "IK.hitcheck"),
		kcesfixtures.TextAssetPath(t, "parts_bv001.aba", "crc2_Underwear038_pants.undressdat"),
	}
	assertRoutePaths(t, miscPaths, "misc", IsKCESMiscFile)

	pskPath := kcesfixtures.TextAssetPath(t, "partsmeta.aba", "default_skirt.psk")
	assertRoutePaths(t, []string{pskPath}, ".psk", IsKCESDataFile)

	rawTexturePath, _ := kcesfixtures.RawObjectPath(t, "cm3d2_megane002.aba", "cm3d2_megane002.tex", aba.ClassIDTexture2D, "cm3d2_megane002.tex.bytes")
	rawMonoScriptPath, _ := kcesfixtures.RawObjectPath(t, "system.aba", "DepthLUT", aba.ClassIDMonoScript, "DepthLUT.monoscript.bytes")
	rawPaths := []string{rawTexturePath, rawMonoScriptPath}
	assertRoutePaths(t, rawPaths, ".bytes", IsKCESRawUnityBytesFile)
	assertAssetDataUnsupportedSamples(t, rawPaths)
}

func assertGeneratedCtJSONRouteSamples(t *testing.T, paths []string) {
	t.Helper()
	if len(paths) == 0 {
		t.Fatalf("no .ct route samples for .ct.json")
	}
	service := &CtService{}
	t.Run(".ct.json", func(t *testing.T) {
		for _, path := range paths {
			path := path
			t.Run(filepath.Base(path)+".json", func(t *testing.T) {
				jsonPath := filepath.Join(t.TempDir(), filepath.Base(path)+".json")
				if err := service.ConvertCtToJson(TestConversionContext, path, jsonPath, TestConversionMaxOutput); err != nil {
					t.Fatalf("ConvertCtToJson: %v", err)
				}
				if !IsKCESCtJSONFile(jsonPath) {
					t.Fatalf(".ct JSON was not routed as supported: %s", jsonPath)
				}
			})
		}
	})
}

func assertRoutePaths(t *testing.T, paths []string, group string, ok func(string) bool) {
	t.Helper()
	if len(paths) == 0 {
		t.Fatalf("no route samples for %s", group)
	}
	t.Run(group, func(t *testing.T) {
		for _, path := range paths {
			path := path
			if strings.HasSuffix(strings.ToLower(path), ".meta.json") || strings.HasSuffix(strings.ToLower(path), ".typetree.json") {
				continue
			}
			t.Run(filepath.Base(path), func(t *testing.T) {
				if !ok(path) {
					t.Fatalf("%s was not routed as supported: %s", group, path)
				}
			})
		}
	})
}

func assertAssetDataUnsupportedSamples(t *testing.T, paths []string) {
	t.Helper()
	t.Run("assets-not-data", func(t *testing.T) {
		for _, path := range paths {
			path := path
			lower := strings.ToLower(path)
			if strings.HasSuffix(lower, ".meta.json") || strings.HasSuffix(lower, ".typetree.json") || strings.HasSuffix(lower, ".psk") {
				continue
			}
			t.Run(filepath.Base(path), func(t *testing.T) {
				if IsKCESDataFile(path) {
					t.Fatalf("non-.psk KCES asset was routed as shared data: %s", path)
				}
			})
		}
	})
}
