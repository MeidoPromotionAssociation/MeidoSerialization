package KCES

import (
	"bytes"
	"context"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/tools"
	"github.com/qmuntal/gltf"
)

func TestEncodeMeshGLB(t *testing.T) {
	geometry := &aba.MeshGeometry{
		Name:      "triangle",
		Positions: [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
		Normals:   [][3]float32{{0, 0, 1}, {0, 0, 1}, {0, 0, 1}},
		Primitives: []aba.MeshPrimitive{{
			Mode:    aba.MeshPrimitiveModeTriangles,
			Indices: []uint32{0, 1, 2},
		}},
	}
	geometry.TexCoords[0] = [][2]float32{{0, 0}, {1, 0}, {0, 1}}
	data, err := encodeMeshGLB(geometry)
	if err != nil {
		t.Fatal(err)
	}
	var document gltf.Document
	if err := gltf.NewDecoder(bytes.NewReader(data)).Decode(&document); err != nil {
		t.Fatal(err)
	}
	if len(document.Meshes) != 1 || len(document.Meshes[0].Primitives) != 1 || document.Meshes[0].Primitives[0].Mode != gltf.PrimitiveTriangles {
		t.Fatalf("unexpected GLB mesh: %+v", document.Meshes)
	}
}

func TestNativeUnityMediaServiceExportsSampleMeshAndSprite(t *testing.T) {
	sample := filepath.Join("..", "..", "testdata", "aba", "cm3d2_megane002.aba")
	if _, err := os.Stat(sample); err != nil {
		t.Skipf("sample not available: %v", err)
	}
	root := filepath.Join(t.TempDir(), "unpacked")
	if err := (&AbaService{}).UnpackAba(sample, root); err != nil {
		t.Fatal(err)
	}
	meshFiles, err := filepath.Glob(filepath.Join(root, "Mesh", "*.mmesh"))
	if err != nil || len(meshFiles) == 0 {
		t.Fatalf("find sample Mesh: %v", err)
	}
	meshOutput := filepath.Join(t.TempDir(), "sample.glb")
	service := &NativeUnityMediaService{}
	if err := service.ConvertMeshToGLB(context.Background(), meshFiles[0], meshOutput, TestConversionMaxOutput); err != nil {
		t.Fatal(err)
	}
	document, err := gltf.Open(meshOutput)
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Meshes) != 1 || len(document.Meshes[0].Primitives) == 0 || len(document.Accessors) < 2 {
		t.Fatalf("sample GLB is missing geometry: meshes=%d accessors=%d", len(document.Meshes), len(document.Accessors))
	}

	t.Run("Sprite", func(t *testing.T) {
		if err := tools.CheckMagick(); err != nil {
			t.Skipf("ImageMagick unavailable: %v", err)
		}
		spriteFiles, err := filepath.Glob(filepath.Join(root, "Sprite", "*.sprite"))
		if err != nil || len(spriteFiles) == 0 {
			t.Fatalf("find sample Sprite: %v", err)
		}
		spriteOutput := filepath.Join(t.TempDir(), "sample.png")
		if err := service.ConvertSpriteToPNG(context.Background(), spriteFiles[0], spriteOutput, TestConversionMaxOutput); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(spriteOutput)
		if err != nil {
			t.Fatal(err)
		}
		config, err := png.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if config.Width <= 0 || config.Height <= 0 {
			t.Fatalf("invalid Sprite PNG dimensions %dx%d", config.Width, config.Height)
		}
	})
}

func TestNativeUnityMediaServiceExportsSpriteViaSiblingAtlas(t *testing.T) {
	if err := tools.CheckMagick(); err != nil {
		t.Skipf("ImageMagick unavailable: %v", err)
	}
	sample := filepath.Join("..", "..", "testdata", "aba", "parts_personal_om015_gp003.aba")
	if _, err := os.Stat(sample); err != nil {
		t.Skipf("sample not available: %v", err)
	}
	root := filepath.Join(t.TempDir(), "unpacked")
	if err := (&AbaService{}).UnpackAba(sample, root); err != nil {
		t.Fatal(err)
	}
	spritePath := filepath.Join(root, "Sprite", "crc2_dress296_acchead_i_.tex.sprite")
	data, err := os.ReadFile(spritePath)
	if err != nil {
		t.Fatal(err)
	}
	object, err := aba.ReadSpriteObject(data)
	if err != nil {
		t.Fatal(err)
	}
	value, err := object.DecodeValue()
	if err != nil {
		t.Fatal(err)
	}
	atlas := value.Field("m_SpriteAtlas")
	if atlas == nil {
		t.Fatal("fixture has no m_SpriteAtlas field")
	}
	atlasPathID, ok := atlas.Field("m_PathID").Int64()
	if !ok || atlasPathID != 0 {
		t.Fatalf("fixture m_SpriteAtlas PathID=%d ok=%t, want a null direct reference", atlasPathID, ok)
	}

	output := filepath.Join(t.TempDir(), "sprite.png")
	if err := (&NativeUnityMediaService{}).ConvertSpriteToPNG(context.Background(), spritePath, output, TestConversionMaxOutput); err != nil {
		t.Fatal(err)
	}
	imageData, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	config, err := png.DecodeConfig(bytes.NewReader(imageData))
	if err != nil {
		t.Fatal(err)
	}
	if config.Width <= 0 || config.Height <= 0 {
		t.Fatalf("invalid Sprite PNG dimensions %dx%d", config.Width, config.Height)
	}
}

func TestNativeUnityMediaServiceExportsSampleAnimation(t *testing.T) {
	sample := filepath.Join("..", "..", "testdata", "aba", "motion.aba")
	if _, err := os.Stat(sample); err != nil {
		t.Skipf("sample not available: %v", err)
	}
	root := filepath.Join(t.TempDir(), "unpacked")
	if err := (&AbaService{}).UnpackAba(sample, root); err != nil {
		t.Fatal(err)
	}
	animationFiles, err := filepath.Glob(filepath.Join(root, "AnimationClip", "*.anm"))
	if err != nil || len(animationFiles) == 0 {
		t.Fatalf("find sample AnimationClip: %v", err)
	}
	output := filepath.Join(t.TempDir(), "sample.glb")
	if err := (&NativeUnityMediaService{}).ConvertAnimationClipToGLTF(context.Background(), animationFiles[0], output, "glb", TestConversionMaxOutput); err != nil {
		t.Fatal(err)
	}
	document, err := gltf.Open(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Animations) != 1 || len(document.Animations[0].Channels) == 0 || len(document.Nodes) == 0 {
		t.Fatalf("sample GLB is missing animation data: animations=%d nodes=%d", len(document.Animations), len(document.Nodes))
	}
}

func TestNativeUnityMediaServiceExtractsSampleAudio(t *testing.T) {
	sample := filepath.Join("..", "..", "testdata", "aba", "sound.aba")
	if _, err := os.Stat(sample); err != nil {
		t.Skipf("sample not available: %v", err)
	}
	root := filepath.Join(t.TempDir(), "unpacked")
	if err := (&AbaService{}).UnpackAba(sample, root); err != nil {
		t.Fatal(err)
	}
	audioFiles, err := filepath.Glob(filepath.Join(root, "AudioClip", "*.audioclip"))
	if err != nil || len(audioFiles) == 0 {
		t.Fatalf("find sample AudioClip: %v", err)
	}
	service := &NativeUnityMediaService{}
	extension, err := service.DetectAudioClipExtension(context.Background(), audioFiles[0])
	if err != nil {
		t.Fatal(err)
	}
	if extension != ".ogg" && extension != ".wav" && extension != ".fsb" {
		t.Fatalf("unexpected sample AudioClip extension %q", extension)
	}
	output := filepath.Join(t.TempDir(), "sample"+extension)
	if err := service.ExtractAudioClip(context.Background(), audioFiles[0], output, TestConversionMaxOutput); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 || NativeAudioClipExtension(data) != extension {
		t.Fatalf("extracted AudioClip has invalid payload for %s", extension)
	}
}
