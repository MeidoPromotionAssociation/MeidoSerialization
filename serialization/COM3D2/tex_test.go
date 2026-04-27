package COM3D2

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/tools"
)

func TestTex(t *testing.T) {
	files, err := filepath.Glob("../../testdata/*.tex")
	if err != nil {
		t.Fatal(err)
	}

	for _, filePath := range files {
		t.Run(filepath.Base(filePath), func(t *testing.T) {
			f, err := os.Open(filePath)
			if err != nil {
				t.Fatalf("failed to open test file: %v", err)
			}
			defer f.Close()

			tex, err := ReadTex(f)
			if err != nil {
				t.Fatalf("failed to read tex: %v", err)
			}

			// Test Dump
			var buf bytes.Buffer
			err = tex.Dump(&buf)
			if err != nil {
				t.Fatalf("failed to dump tex: %v", err)
			}

			// Re-read from dumped buffer
			tex2, err := ReadTex(&buf)
			if err != nil {
				t.Fatalf("failed to re-read dumped tex: %v", err)
			}

			// Compare complete structure
			if !reflect.DeepEqual(tex, tex2) {
				t.Errorf("data mismatch after dump and re-read")
			}
		})
	}
}

func TestFlipBlockCompressedTextureVerticallyDXT1(t *testing.T) {
	makeBlock := func(base byte) []byte {
		return []byte{
			base + 0, base + 1, base + 2, base + 3,
			base + 4, base + 5, base + 6, base + 7,
		}
	}
	flipBlockRows := func(block []byte) []byte {
		return []byte{
			block[0], block[1], block[2], block[3],
			block[7], block[6], block[5], block[4],
		}
	}

	blockA := makeBlock(0x10)
	blockB := makeBlock(0x20)
	blockC := makeBlock(0x30)
	blockD := makeBlock(0x40)

	data := append(append(append([]byte{}, blockA...), blockB...), append(blockC, blockD...)...)
	got, err := flipBlockCompressedTextureVertically(data, 8, 8, DXT1)
	if err != nil {
		t.Fatalf("flipBlockCompressedTextureVertically returned error: %v", err)
	}

	want := append(
		append([]byte{}, flipBlockRows(blockC)...),
		append(flipBlockRows(blockD), append(flipBlockRows(blockA), flipBlockRows(blockB)...)...)...,
	)
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected flipped DXT1 data:\n got: %v\nwant: %v", got, want)
	}

	roundTrip, err := flipBlockCompressedTextureVertically(got, 8, 8, DXT1)
	if err != nil {
		t.Fatalf("round-trip flip returned error: %v", err)
	}
	if !bytes.Equal(roundTrip, data) {
		t.Fatalf("double flip should restore original data")
	}
}

func TestFlipBlockCompressedTextureVerticallyDXT5(t *testing.T) {
	packAlphaRows := func(rows [4]uint16) []byte {
		bits := uint64(rows[0]) |
			(uint64(rows[1]) << 12) |
			(uint64(rows[2]) << 24) |
			(uint64(rows[3]) << 36)
		out := make([]byte, 6)
		for i := 0; i < 6; i++ {
			out[i] = byte(bits >> (8 * i))
		}
		return out
	}
	makeBlock := func(alphaRows [4]uint16, colorBase byte) []byte {
		block := []byte{
			colorBase + 0,
			colorBase + 1,
		}
		block = append(block, packAlphaRows(alphaRows)...)
		block = append(block,
			colorBase+8, colorBase+9, colorBase+10, colorBase+11,
			colorBase+12, colorBase+13, colorBase+14, colorBase+15,
		)
		return block
	}
	flipBlockRows := func(block []byte) []byte {
		out := make([]byte, len(block))
		copy(out[:2], block[:2])

		var alphaBits uint64
		for i := 0; i < 6; i++ {
			alphaBits |= uint64(block[2+i]) << (8 * i)
		}
		rows := [4]uint16{
			uint16(alphaBits & 0x0FFF),
			uint16((alphaBits >> 12) & 0x0FFF),
			uint16((alphaBits >> 24) & 0x0FFF),
			uint16((alphaBits >> 36) & 0x0FFF),
		}
		copy(out[2:8], packAlphaRows([4]uint16{rows[3], rows[2], rows[1], rows[0]}))

		copy(out[8:12], block[8:12])
		out[12] = block[15]
		out[13] = block[14]
		out[14] = block[13]
		out[15] = block[12]
		return out
	}

	blockA := makeBlock([4]uint16{0x001, 0x123, 0x456, 0x789}, 0x10)
	blockB := makeBlock([4]uint16{0x00F, 0x0AA, 0x155, 0x2A8}, 0x30)
	blockC := makeBlock([4]uint16{0x321, 0x654, 0x987, 0xABC}, 0x50)
	blockD := makeBlock([4]uint16{0x111, 0x222, 0x333, 0x444}, 0x70)

	data := append(append(append([]byte{}, blockA...), blockB...), append(blockC, blockD...)...)
	got, err := flipBlockCompressedTextureVertically(data, 8, 8, DXT5)
	if err != nil {
		t.Fatalf("flipBlockCompressedTextureVertically returned error: %v", err)
	}

	want := append(
		append([]byte{}, flipBlockRows(blockC)...),
		append(flipBlockRows(blockD), append(flipBlockRows(blockA), flipBlockRows(blockB)...)...)...,
	)
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected flipped DXT5 data:\n got: %v\nwant: %v", got, want)
	}

	roundTrip, err := flipBlockCompressedTextureVertically(got, 8, 8, DXT5)
	if err != nil {
		t.Fatalf("round-trip flip returned error: %v", err)
	}
	if !bytes.Equal(roundTrip, data) {
		t.Fatalf("double flip should restore original data")
	}
}

func TestConvertImageToTexCompressedStoresFlippedDXTPayload(t *testing.T) {
	if err := tools.CheckMagick(); err != nil {
		t.Skipf("skipping test that requires ImageMagick: %v", err)
	}

	tests := []struct {
		name         string
		ext          string
		format       string
		textureFmt   int32
		withAlpha    bool
		ddsCompress  string
		writeImageFn func(string) error
	}{
		{
			name:        "dxt1",
			ext:         ".jpg",
			format:      "JPEG",
			textureFmt:  DXT1,
			withAlpha:   false,
			ddsCompress: "dxt1",
			writeImageFn: func(path string) error {
				return writeDirectionalJPEG(path, 80, 80)
			},
		},
		{
			name:        "dxt5",
			ext:         ".png",
			format:      "PNG",
			textureFmt:  DXT5,
			withAlpha:   true,
			ddsCompress: "dxt5",
			writeImageFn: func(path string) error {
				return writeDirectionalPNG(path, 80, 80)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			inputPath := filepath.Join(tempDir, tc.name+tc.ext)
			if err := tc.writeImageFn(inputPath); err != nil {
				t.Fatalf("failed to write %s fixture: %v", tc.format, err)
			}

			tex, err := ConvertImageToTex(inputPath, tc.name, true, false)
			if err != nil {
				t.Fatalf("ConvertImageToTex failed: %v", err)
			}

			if tex.Width != 80 || tex.Height != 80 {
				t.Fatalf("unexpected size: got %dx%d, want 80x80", tex.Width, tex.Height)
			}
			if tex.TextureFormat != tc.textureFmt {
				t.Fatalf("unexpected texture format: got %d, want %d", tex.TextureFormat, tc.textureFmt)
			}

			ddsData, err := convertImageToDDSBytes(inputPath, tc.ddsCompress)
			if err != nil {
				t.Fatalf("failed to create reference DDS: %v", err)
			}
			if len(ddsData) < 128 || string(ddsData[:4]) != "DDS " {
				t.Fatalf("ImageMagick returned invalid DDS header")
			}

			rawPayload := append([]byte(nil), ddsData[128:]...)
			expected, err := flipBlockCompressedTextureVertically(rawPayload, tex.Width, tex.Height, tc.textureFmt)
			if err != nil {
				t.Fatalf("failed to flip reference payload: %v", err)
			}

			if bytes.Equal(rawPayload, expected) {
				t.Fatalf("fixture produced vertically symmetric DXT payload, test cannot prove flipping behavior")
			}
			if !bytes.Equal(tex.Data, expected) {
				t.Fatalf("compressed tex payload does not match flipped DDS payload")
			}
		})
	}
}

func writeDirectionalJPEG(path string, width, height int) error {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if y < height/2 {
				img.Set(x, y, color.RGBA{
					R: uint8(240 - (x % 17)),
					G: uint8((x*5 + y) % 251),
					B: uint8((x + y) % 43),
					A: 255,
				})
				continue
			}
			img.Set(x, y, color.RGBA{
				R: uint8((x*3 + y) % 61),
				G: uint8((x + y*2) % 89),
				B: uint8(200 + (x % 37)),
				A: 255,
			})
		}
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	return jpeg.Encode(file, img, &jpeg.Options{Quality: 95})
}

func writeDirectionalPNG(path string, width, height int) error {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if y < height/2 {
				img.Set(x, y, color.NRGBA{
					R: uint8(250 - (x % 31)),
					G: uint8((x*3 + y) % 97),
					B: uint8((x + y) % 53),
					A: uint8(32 + (x+y)%96),
				})
				continue
			}
			img.Set(x, y, color.NRGBA{
				R: uint8((x*2 + y) % 71),
				G: uint8((x + y*5) % 113),
				B: uint8(180 + (x % 61)),
				A: uint8(160 + (x+y)%95),
			})
		}
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	return png.Encode(file, img)
}

func convertImageToDDSBytes(inputPath string, compression string) ([]byte, error) {
	cmd := exec.Command(
		"magick",
		inputPath,
		"-define", "dds:compression="+compression,
		"dds:-",
	)
	tools.SetHideWindow(cmd)

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return output, nil
}
