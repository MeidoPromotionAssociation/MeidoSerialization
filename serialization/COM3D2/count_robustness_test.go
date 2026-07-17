package COM3D2

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
)

func TestCOM3D2ReadersRejectNegativeCollectionCountsWithoutPanicking(t *testing.T) {
	modelHeader := func() []byte {
		return com3d2CountFixture(t, func(w *stream.BinaryWriter) {
			countMustWrite(t, w.WriteString(ModelSignature))
			countMustWrite(t, w.WriteInt32(2001))
			countMustWrite(t, w.WriteString("model"))
			countMustWrite(t, w.WriteString("root"))
			countMustWrite(t, w.WriteInt32(-1))
		})
	}

	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "animation curve keyframes",
			run: func() error {
				_, err := ReadAnimationCurve(stream.NewBinaryReader(bytes.NewReader([]byte{0xff, 0xff, 0xff, 0xff})))
				return err
			},
		},
		{
			name: "dance objects",
			run: func() error {
				_, err := ReadDanceObjectData(bytes.NewReader([]byte{0xff, 0xff, 0xff, 0xff}))
				return err
			},
		},
		{
			name: "colliders",
			run: func() error {
				wire := com3d2CountFixture(t, func(w *stream.BinaryWriter) {
					countMustWrite(t, w.WriteString(ColSignature))
					countMustWrite(t, w.WriteInt32(ColVersion))
					countMustWrite(t, w.WriteInt32(-1))
				})
				_, err := ReadCol(bytes.NewReader(wire))
				return err
			},
		},
		{
			name: "animation property keyframes",
			run: func() error {
				wire := com3d2CountFixture(t, func(w *stream.BinaryWriter) {
					countMustWrite(t, w.WriteString(AnmSignature))
					countMustWrite(t, w.WriteInt32(AnmVersion))
					countMustWrite(t, w.WriteByte(1))
					countMustWrite(t, w.WriteString("bone"))
					countMustWrite(t, w.WriteByte(100))
					countMustWrite(t, w.WriteInt32(-1))
				})
				_, err := ReadAnm(bytes.NewReader(wire))
				return err
			},
		},
		{
			name: "material keywords",
			run: func() error {
				wire := com3d2CountFixture(t, func(w *stream.BinaryWriter) {
					countMustWrite(t, w.WriteString("_KEYWORDS"))
					countMustWrite(t, w.WriteInt32(-1))
				})
				return (&KeywordProperty{}).Read(stream.NewBinaryReader(bytes.NewReader(wire)))
			},
		},
		{
			name: "translation frames",
			run: func() error {
				wire := com3d2CountFixture(t, func(w *stream.BinaryWriter) {
					countMustWrite(t, w.WriteInt32(1))
					countMustWrite(t, w.WriteInt32(-1))
				})
				return (&TranslationTrack{}).read(stream.NewBinaryReader(bytes.NewReader(wire)))
			},
		},
		{
			name: "partial bone values",
			run: func() error {
				wire := com3d2CountFixture(t, func(w *stream.BinaryWriter) {
					countMustWrite(t, w.WriteInt32(PartialModePartial))
					countMustWrite(t, w.WriteInt32(-1))
				})
				_, _, err := readPartial(stream.NewBinaryReader(bytes.NewReader(wire)))
				return err
			},
		},
		{
			name: "panier groups",
			run: func() error {
				wire := com3d2CountFixture(t, func(w *stream.BinaryWriter) {
					countMustWrite(t, w.WriteString(PskSignature))
					countMustWrite(t, w.WriteInt32(217))
					countMustWrite(t, w.WriteFloat32(0))
					countMustWrite(t, w.WriteInt32(0))
					countMustWrite(t, w.WriteInt32(-1))
				})
				_, err := ReadPsk(bytes.NewReader(wire))
				return err
			},
		},
		{
			name: "model bones",
			run: func() error {
				_, err := ReadModel(bufio.NewReader(bytes.NewReader(modelHeader())))
				return err
			},
		},
		{
			name: "model metadata bones",
			run: func() error {
				_, err := ReadModelMetadata(bufio.NewReader(bytes.NewReader(modelHeader())))
				return err
			},
		},
		{
			name: "morph vertices",
			run: func() error {
				wire := com3d2CountFixture(t, func(w *stream.BinaryWriter) {
					countMustWrite(t, w.WriteString("morph"))
					countMustWrite(t, w.WriteInt32(-1))
				})
				_, err := ReadMorphData(stream.NewBinaryReader(bytes.NewReader(wire)), 2102)
				return err
			},
		},
		{
			name: "skin thickness points",
			run: func() error {
				wire := com3d2CountFixture(t, func(w *stream.BinaryWriter) {
					countMustWrite(t, w.WriteString("group"))
					countMustWrite(t, w.WriteString("start"))
					countMustWrite(t, w.WriteString("end"))
					countMustWrite(t, w.WriteInt32(15))
					countMustWrite(t, w.WriteInt32(-1))
				})
				return readThickGroup(stream.NewBinaryReader(bytes.NewReader(wire)), &ThickGroup{})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("reader panicked: %v", recovered)
				}
			}()
			err := test.run()
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), "negative") {
				t.Fatalf("error = %v, want a negative-count rejection", err)
			}
		})
	}
}

func TestCOM3D2ReadersDoNotPreallocateHugePositiveCountsOnTruncatedStreams(t *testing.T) {
	const hugeCount int32 = 1<<31 - 1

	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "animation curve keyframes",
			run: func() error {
				_, err := ReadAnimationCurve(stream.NewBinaryReader(bytes.NewReader(com3d2CountFixture(t, func(w *stream.BinaryWriter) {
					countMustWrite(t, w.WriteInt32(hugeCount))
				}))))
				return err
			},
		},
		{
			name: "dance objects",
			run: func() error {
				_, err := ReadDanceObjectData(bytes.NewReader(com3d2CountFixture(t, func(w *stream.BinaryWriter) {
					countMustWrite(t, w.WriteInt32(hugeCount))
				})))
				return err
			},
		},
		{
			name: "colliders",
			run: func() error {
				wire := com3d2CountFixture(t, func(w *stream.BinaryWriter) {
					countMustWrite(t, w.WriteString(ColSignature))
					countMustWrite(t, w.WriteInt32(ColVersion))
					countMustWrite(t, w.WriteInt32(hugeCount))
				})
				_, err := ReadCol(bytes.NewReader(wire))
				return err
			},
		},
		{
			name: "animation property keyframes",
			run: func() error {
				wire := com3d2CountFixture(t, func(w *stream.BinaryWriter) {
					countMustWrite(t, w.WriteString(AnmSignature))
					countMustWrite(t, w.WriteInt32(AnmVersion))
					countMustWrite(t, w.WriteByte(1))
					countMustWrite(t, w.WriteString("bone"))
					countMustWrite(t, w.WriteByte(100))
					countMustWrite(t, w.WriteInt32(hugeCount))
				})
				_, err := ReadAnm(bytes.NewReader(wire))
				return err
			},
		},
		{
			name: "material keywords",
			run: func() error {
				wire := com3d2CountFixture(t, func(w *stream.BinaryWriter) {
					countMustWrite(t, w.WriteString("_KEYWORDS"))
					countMustWrite(t, w.WriteInt32(hugeCount))
				})
				return (&KeywordProperty{}).Read(stream.NewBinaryReader(bytes.NewReader(wire)))
			},
		},
		{
			name: "translation frames",
			run: func() error {
				wire := com3d2CountFixture(t, func(w *stream.BinaryWriter) {
					countMustWrite(t, w.WriteInt32(1))
					countMustWrite(t, w.WriteInt32(hugeCount))
					countMustWrite(t, w.WriteString(""))
				})
				return (&TranslationTrack{}).read(stream.NewBinaryReader(bytes.NewReader(wire)))
			},
		},
		{
			name: "property values",
			run: func() error {
				wire := com3d2CountFixture(t, func(w *stream.BinaryWriter) {
					countMustWrite(t, w.WriteInt32(1))
					countMustWrite(t, w.WriteInt32(0))
					countMustWrite(t, w.WriteString(""))
					countMustWrite(t, w.WriteInt32(0))
					countMustWrite(t, w.WriteString(""))
					countMustWrite(t, w.WriteString(""))
					countMustWrite(t, w.WriteInt32(hugeCount))
				})
				return (&PropertyTrack{}).read(stream.NewBinaryReader(bytes.NewReader(wire)))
			},
		},
		{
			name: "event parameter array",
			run: func() error {
				wire := com3d2CountFixture(t, func(w *stream.BinaryWriter) {
					countMustWrite(t, w.WriteInt32(12))
					countMustWrite(t, w.WriteInt32(hugeCount))
				})
				_, err := readEventParameter(stream.NewBinaryReader(bytes.NewReader(wire)))
				return err
			},
		},
		{
			name: "partial bone values",
			run: func() error {
				wire := com3d2CountFixture(t, func(w *stream.BinaryWriter) {
					countMustWrite(t, w.WriteInt32(PartialModePartial))
					countMustWrite(t, w.WriteInt32(hugeCount))
				})
				_, _, err := readPartial(stream.NewBinaryReader(bytes.NewReader(wire)))
				return err
			},
		},
		{
			name: "panier groups",
			run: func() error {
				wire := com3d2CountFixture(t, func(w *stream.BinaryWriter) {
					countMustWrite(t, w.WriteString(PskSignature))
					countMustWrite(t, w.WriteInt32(217))
					countMustWrite(t, w.WriteFloat32(0))
					countMustWrite(t, w.WriteInt32(0))
					countMustWrite(t, w.WriteInt32(hugeCount))
				})
				_, err := ReadPsk(bytes.NewReader(wire))
				return err
			},
		},
		{
			name: "model bones",
			run: func() error {
				wire := com3d2CountFixture(t, func(w *stream.BinaryWriter) {
					countMustWrite(t, w.WriteString(ModelSignature))
					countMustWrite(t, w.WriteInt32(2001))
					countMustWrite(t, w.WriteString("model"))
					countMustWrite(t, w.WriteString("root"))
					countMustWrite(t, w.WriteInt32(hugeCount))
				})
				_, err := ReadModel(bufio.NewReader(bytes.NewReader(wire)))
				return err
			},
		},
		{
			name: "morph vertices",
			run: func() error {
				wire := com3d2CountFixture(t, func(w *stream.BinaryWriter) {
					countMustWrite(t, w.WriteString("morph"))
					countMustWrite(t, w.WriteInt32(hugeCount))
					countMustWrite(t, w.WriteBool(false))
				})
				_, err := ReadMorphData(stream.NewBinaryReader(bytes.NewReader(wire)), 2102)
				return err
			},
		},
		{
			name: "skin thickness points",
			run: func() error {
				wire := com3d2CountFixture(t, func(w *stream.BinaryWriter) {
					countMustWrite(t, w.WriteString("group"))
					countMustWrite(t, w.WriteString("start"))
					countMustWrite(t, w.WriteString("end"))
					countMustWrite(t, w.WriteInt32(15))
					countMustWrite(t, w.WriteInt32(hugeCount))
				})
				return readThickGroup(stream.NewBinaryReader(bytes.NewReader(wire)), &ThickGroup{})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("reader panicked: %v", recovered)
				}
			}()
			if err := test.run(); err == nil {
				t.Fatal("truncated stream unexpectedly succeeded")
			}
		})
	}
}

func com3d2CountFixture(t *testing.T, write func(*stream.BinaryWriter)) []byte {
	t.Helper()
	var out bytes.Buffer
	write(stream.NewBinaryWriter(&out))
	return out.Bytes()
}

func countMustWrite(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("build fixture: %v", err)
	}
}
