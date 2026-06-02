package KCES

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/tools"
)

func TestAbaService_UnpackAba_KCESSampleExportsUsableFiles(t *testing.T) {
	if err := tools.CheckMagick(); err != nil {
		t.Skipf("ImageMagick not available: %v", err)
	}

	sample := filepath.Join("..", "..", "testdata", "aba", "parts_personal002.aba")
	if _, err := os.Stat(sample); err != nil {
		t.Skipf("sample not found: %v", err)
	}

	outDir := filepath.Join(t.TempDir(), "parts_personal002")
	service := &AbaService{}
	if err := service.UnpackAba(sample, outDir); err != nil {
		t.Fatalf("UnpackAba failed: %v", err)
	}

	assertExists := func(rel string) {
		t.Helper()
		if _, err := os.Stat(filepath.Join(outDir, rel)); err != nil {
			t.Fatalf("expected %s: %v", rel, err)
		}
	}
	assertNotExists := func(rel string) {
		t.Helper()
		if _, err := os.Stat(filepath.Join(outDir, rel)); !os.IsNotExist(err) {
			t.Fatalf("did not expect %s, stat err=%v", rel, err)
		}
	}

	assertExists(filepath.Join("TextAsset", "parts_personal002.menuassets"))
	assertExists(filepath.Join("TextAsset", "parts_personal002.menuassets.meta.json"))
	assertExists(filepath.Join("Texture2D", "hair_twin019_pink.tex.png"))
	assertExists(filepath.Join("Texture2D", "hair_twin019_pink.tex.bytes"))
	assertExists(filepath.Join("Texture2D", "hair_twin019_pink.tex.bytes.meta.json"))
	assertExists(filepath.Join("Texture2D", "hair_twin019_pink.tex.bytes.typetree.json"))
	assertExists(filepath.Join("Sprite", "hair_twin019_i_.tex.png"))
	assertExists(filepath.Join("Sprite", "hair_twin019_i_.tex.sprite.bytes"))
	assertExists(filepath.Join("Sprite", "hair_twin019_i_.tex.sprite.bytes.meta.json"))
	assertExists(filepath.Join("Sprite", "hair_twin019_i_.tex.sprite.bytes.typetree.json"))
	assertExists(filepath.Join("Mesh", "hair_twin019.mmesh.bytes"))
	assertExists(filepath.Join("Mesh", "hair_twin019.mmesh.bytes.meta.json"))
	assertExists(filepath.Join("Mesh", "hair_twin019.mmesh.bytes.typetree.json"))
	assertExists(filepath.Join("Mesh", "hair_twin019.mmesh.crmesh"))
	assertExists(filepath.Join("SpriteAtlas", "parts_personal002.partsatlas.bytes"))
	assertExists(filepath.Join("SpriteAtlas", "parts_personal002.partsatlas.bytes.meta.json"))
	assertExists(filepath.Join("SpriteAtlas", "parts_personal002.partsatlas.bytes.typetree.json"))
	assertNotExists(filepath.Join("AssetBundle", "parts_personal002.aba.bytes"))
	assertNotExists(filepath.Join("Type_142", "parts_personal002.aba.bytes"))
	assertValidRawMeta := func(rel string) rawAssetMeta {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(outDir, rel))
		if err != nil {
			t.Fatalf("read raw meta %s: %v", rel, err)
		}
		var meta rawAssetMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			t.Fatalf("decode raw meta %s: %v", rel, err)
		}
		if meta.PathID == 0 {
			t.Fatalf("raw meta %s has zero PathID", rel)
		}
		if meta.LoadName == "" {
			t.Fatalf("raw meta %s has empty loadName", rel)
		}
		return meta
	}
	textMeta := assertValidRawMeta(filepath.Join("TextAsset", "parts_personal002.menuassets.meta.json"))
	if textMeta.LoadName != "assets/gamedata/parts/parts_personal002/parts_personal002.menuassets.bytes" {
		t.Fatalf("TextAsset loadName got %q", textMeta.LoadName)
	}
	texMeta := assertValidRawMeta(filepath.Join("Texture2D", "hair_twin019_pink.tex.bytes.meta.json"))
	if texMeta.LoadName != "assets/gamedata/parts/parts_personal002/parts/face/hair/hairt/hair_twin019/texture/hair_twin019_pink.tex.png" {
		t.Fatalf("Texture2D loadName got %q", texMeta.LoadName)
	}
	assertValidTypeTree := func(rel string, wantClassID int32) RawUnityTypeTreeEnvelope {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(outDir, rel))
		if err != nil {
			t.Fatalf("read TypeTree sidecar %s: %v", rel, err)
		}
		var envelope RawUnityTypeTreeEnvelope
		if err := json.Unmarshal(data, &envelope); err != nil {
			t.Fatalf("decode TypeTree sidecar %s: %v", rel, err)
		}
		if envelope.Format != RawUnityTypeTreeFormat {
			t.Fatalf("TypeTree sidecar %s format got %q", rel, envelope.Format)
		}
		if envelope.ClassID != wantClassID || envelope.TypeName == "" || envelope.PathID == 0 || envelope.Value == nil {
			t.Fatalf("incomplete TypeTree sidecar %s: %+v", rel, envelope)
		}
		if envelope.Value.Name == "" && len(envelope.Value.Children) == 0 {
			t.Fatalf("empty TypeTree value in %s: %+v", rel, envelope.Value)
		}
		return envelope
	}
	texTree := assertValidTypeTree(filepath.Join("Texture2D", "hair_twin019_pink.tex.bytes.typetree.json"), aba.ClassIDTexture2D)
	if texTree.Name != "hair_twin019_pink.tex" {
		t.Fatalf("Texture2D TypeTree name got %q", texTree.Name)
	}
	assertValidRawMeta(filepath.Join("Sprite", "hair_twin019_i_.tex.sprite.bytes.meta.json"))
	assertValidRawMeta(filepath.Join("Mesh", "hair_twin019.mmesh.bytes.meta.json"))

	crmesh, err := os.ReadFile(filepath.Join(outDir, "Mesh", "hair_twin019.mmesh.crmesh"))
	if err != nil {
		t.Fatalf("read crmesh: %v", err)
	}
	if len(crmesh) < 12 || crmesh[0] != 11 || string(crmesh[1:12]) != "CR_MOD_MESH" {
		t.Fatalf("invalid crmesh prefix: % x", crmesh[:min(len(crmesh), 12)])
	}

	spriteFile, err := os.Open(filepath.Join(outDir, "Sprite", "hair_twin019_i_.tex.png"))
	if err != nil {
		t.Fatalf("open sprite png: %v", err)
	}
	defer spriteFile.Close()
	cfg, err := png.DecodeConfig(spriteFile)
	if err != nil {
		t.Fatalf("decode sprite png config: %v", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 256 || cfg.Height > 256 {
		t.Fatalf("unexpected sprite png size: %dx%d", cfg.Width, cfg.Height)
	}
}
