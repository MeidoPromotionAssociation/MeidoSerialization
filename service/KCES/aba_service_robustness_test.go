package KCES

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
)

// minimalAbaServiceTypeTree builds the smallest valid TypeTree so fixtures can
// carry deliberately malformed payloads through the tree-requiring writer.
func minimalAbaServiceTypeTree(classID int32) aba.TypeTreeType {
	return aba.TypeTreeType{
		TypeId:          classID,
		ScriptTypeIndex: -1,
		StringBuffer:    []byte("Base\x00"),
		Nodes:           []aba.TypeTreeNode{{Version: 1, ByteSize: -1}},
	}
}

// addMalformedAbaServiceObject queues an arbitrary payload for a class through the
// native-object entry point, bypassing the payload encoding used by real assets.
func addMalformedAbaServiceObject(w *aba.SerializedFileWriter, classID int32, name string, payload []byte) {
	data := append([]byte(nil), payload...)
	w.AddNativeUnityObjectSourceWithLoadNameAndPathID(classID, minimalAbaServiceTypeTree(classID), name, name, aba.SerializedObjectDataSource{
		Size:   uint32(len(data)),
		Prefix: data,
		WriteTo: func(out io.Writer) error {
			_, err := out.Write(data)
			return err
		},
	}, 0)
}

func buildAbaServiceSerializedFile(t *testing.T, add func(*aba.SerializedFileWriter)) []byte {
	t.Helper()
	w := aba.NewSerializedFileWriter(defaultKCESUnityVersion)
	if add != nil {
		add(w)
	}
	var out bytes.Buffer
	if err := w.Write(&out); err != nil {
		t.Fatalf("write SerializedFile fixture: %v", err)
	}
	return out.Bytes()
}

func writeAbaServiceFile(t *testing.T, entries []aba.AbaFileEntry) string {
	t.Helper()
	var out bytes.Buffer
	if err := aba.WriteAba(&out, entries, &aba.AbaWriteOptions{
		EngineVersion:     defaultKCESUnityVersion,
		GenerationVersion: defaultKCESGenerationVersion,
		Version:           defaultKCESAbaVersion,
		Compress:          false,
	}); err != nil {
		t.Fatalf("write ABA fixture: %v", err)
	}
	path := filepath.Join(t.TempDir(), "fixture.aba")
	if err := os.WriteFile(path, out.Bytes(), 0644); err != nil {
		t.Fatalf("save ABA fixture: %v", err)
	}
	return path
}

func TestAbaServiceListAbaRejectsPartialResultOnMalformedSerializedFile(t *testing.T) {
	valid := buildAbaServiceSerializedFile(t, func(w *aba.SerializedFileWriter) {
		w.AddTextAsset("valid.txt", []byte("valid"))
	})
	path := writeAbaServiceFile(t, []aba.AbaFileEntry{
		{Name: "CAB-valid", Data: valid, IsSerialized: true},
		{Name: "CAB-broken", Data: []byte("not a SerializedFile"), IsSerialized: true},
	})

	entries, err := (&AbaService{}).ListAba(path)
	if err == nil {
		t.Fatalf("ListAba returned partial success: %+v", entries)
	}
	if entries != nil {
		t.Fatalf("ListAba returned %d partial entries together with error", len(entries))
	}
	if !strings.Contains(err.Error(), "CAB-broken") || !strings.Contains(err.Error(), "parse serialized .aba entry") {
		t.Fatalf("ListAba error lacks SerializedFile context: %v", err)
	}
}

func writeCorruptLZ4SerializedAba(t *testing.T) string {
	t.Helper()
	var info bytes.Buffer
	info.Write(make([]byte, 16))
	mustWriteBinary := func(value interface{}) {
		t.Helper()
		if err := binary.Write(&info, binary.BigEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	mustWriteBinary(int32(1))
	mustWriteBinary(uint32(32))
	mustWriteBinary(uint32(4))
	mustWriteBinary(uint16(0x40 | aba.CompressionLZ4))
	mustWriteBinary(int32(1))
	mustWriteBinary(int64(0))
	mustWriteBinary(int64(32))
	mustWriteBinary(uint32(aba.DirFlagSerializedFile))
	info.WriteString("CAB-corrupt-lz4\x00")

	var file bytes.Buffer
	file.WriteString("UnityFS\x00")
	if err := binary.Write(&file, binary.BigEndian, uint32(7)); err != nil {
		t.Fatal(err)
	}
	file.WriteString("5.x.x\x00")
	file.WriteString("2021.3.3f1\x00")
	fsHeaderOffset := file.Len()
	for _, value := range []interface{}{
		int64(0),
		uint32(info.Len()),
		uint32(info.Len()),
		uint32(aba.FlagHasDirectoryInfo),
	} {
		if err := binary.Write(&file, binary.BigEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	for file.Len()%16 != 0 {
		file.WriteByte(0)
	}
	file.Write(info.Bytes())
	file.Write([]byte{0xff, 0xff, 0xff, 0xff})
	binary.BigEndian.PutUint64(file.Bytes()[fsHeaderOffset:], uint64(file.Len()))

	path := filepath.Join(t.TempDir(), "corrupt-data.aba")
	if err := os.WriteFile(path, file.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAbaServiceListAbaReturnsAbaDataError(t *testing.T) {
	path := writeCorruptLZ4SerializedAba(t)
	entries, err := (&AbaService{}).ListAba(path)
	if err == nil {
		t.Fatalf("ListAba silently skipped corrupt LZ4 data: %+v", entries)
	}
	if entries != nil {
		t.Fatalf("ListAba returned partial entries with error: %+v", entries)
	}
	if !strings.Contains(err.Error(), "CAB-corrupt-lz4") || !strings.Contains(err.Error(), "read serialized .aba entry") {
		t.Fatalf("ListAba error lacks .aba directory context: %v", err)
	}
}

func TestAbaServiceUnpackRejectsMalformedTextAsset(t *testing.T) {
	serialized := buildAbaServiceSerializedFile(t, func(w *aba.SerializedFileWriter) {
		addMalformedAbaServiceObject(w, aba.ClassIDTextAsset, "broken.txt", []byte{1, 2, 3})
	})
	path := writeAbaServiceFile(t, []aba.AbaFileEntry{{
		Name: "CAB-broken-text", Data: serialized, IsSerialized: true,
	}})

	err := (&AbaService{}).UnpackAba(path, filepath.Join(t.TempDir(), "out"))
	if err == nil {
		t.Fatal("UnpackAba silently dropped malformed TextAsset")
	}
	if !strings.Contains(err.Error(), "read TextAsset") || !strings.Contains(err.Error(), "CAB-broken-text") || !strings.Contains(err.Error(), "PathID") {
		t.Fatalf("TextAsset error lacks asset context: %v", err)
	}
}

func TestAbaServiceUnpackRejectsMalformedAssetBundleContainer(t *testing.T) {
	serialized := buildAbaServiceSerializedFile(t, func(w *aba.SerializedFileWriter) {
		// SerializedFileWriter will append its valid structural AssetBundle too;
		// this deliberately malformed earlier one verifies that map failures are
		// not discarded.
		addMalformedAbaServiceObject(w, aba.ClassIDAssetBundle, "broken-container", []byte{1, 2, 3})
	})
	path := writeAbaServiceFile(t, []aba.AbaFileEntry{{
		Name: "CAB-broken-container", Data: serialized, IsSerialized: true,
	}})
	outDir := filepath.Join(t.TempDir(), "out")

	err := (&AbaService{}).UnpackAba(path, outDir)
	if err == nil {
		t.Fatal("UnpackAba silently ignored malformed AssetBundle container")
	}
	if !strings.Contains(err.Error(), "AssetBundle container map") || !strings.Contains(err.Error(), "CAB-broken-container") {
		t.Fatalf("container-map error lacks source context: %v", err)
	}
	if _, statErr := os.Stat(outDir); !os.IsNotExist(statErr) {
		t.Fatalf("output directory was created before structural validation: %v", statErr)
	}
}

func TestAbaServiceUnpackPreservesEmptyTextAsset(t *testing.T) {
	serialized := buildAbaServiceSerializedFile(t, func(w *aba.SerializedFileWriter) {
		w.AddTextAssetWithLoadName("empty.txt", "assets/empty.txt", nil)
	})
	path := writeAbaServiceFile(t, []aba.AbaFileEntry{{
		Name: "CAB-empty-text", Data: serialized, IsSerialized: true,
	}})
	outDir := filepath.Join(t.TempDir(), "out")

	if err := (&AbaService{}).UnpackAba(path, outDir); err != nil {
		t.Fatalf("UnpackAba: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(outDir, "TextAsset", "empty.txt"))
	if err != nil {
		t.Fatalf("empty TextAsset was not extracted: %v", err)
	}
	if len(data) != 0 {
		t.Fatalf("empty TextAsset contains %d bytes", len(data))
	}
	if _, err := os.Stat(filepath.Join(outDir, "TextAsset", "empty.txt.meta.json")); !os.IsNotExist(err) {
		t.Fatalf("pure directory unexpectedly contains TextAsset metadata: %v", err)
	}
}

func TestAbaServiceUnpackRejectsTextureWithoutTypeTree(t *testing.T) {
	// The writer can no longer produce files without type trees, so a minimal v22
	// SerializedFile with a treeless Texture2D type is written by hand the way a
	// third-party tool might emit it.
	rawTexture := []byte{3, 0, 0, 0, 'b', 'a', 'd', 0}
	serialized := writeTypeTreelessTexture2DSerializedFile(t, rawTexture)
	path := writeAbaServiceFile(t, []aba.AbaFileEntry{{
		Name: "CAB-bad-preview", Data: serialized, IsSerialized: true,
	}})
	outDir := filepath.Join(t.TempDir(), "out")

	err := (&AbaService{}).UnpackAba(path, outDir)
	if err == nil || !strings.Contains(err.Error(), "has no TypeTree") {
		t.Fatalf("Texture2D without a TypeTree error = %v", err)
	}
	if _, statErr := os.Stat(outDir); !os.IsNotExist(statErr) {
		t.Fatalf("output directory was created for an object without a TypeTree: %v", statErr)
	}
}

func TestAbaServiceUnpackDisambiguatesCollidingRequiredAssetPaths(t *testing.T) {
	serialized := buildAbaServiceSerializedFile(t, func(w *aba.SerializedFileWriter) {
		w.AddTextAsset("same.menuassets", []byte("first"))
		w.AddTextAsset("same.menuassets", []byte("second"))
	})
	path := writeAbaServiceFile(t, []aba.AbaFileEntry{{
		Name: "CAB-collision", Data: serialized, IsSerialized: true,
	}})
	outDir := filepath.Join(t.TempDir(), "out")

	if err := (&AbaService{}).UnpackAba(path, outDir); err != nil {
		t.Fatalf("UnpackAba: %v", err)
	}
	for name, want := range map[string]string{
		"same.menuassets":      "first",
		"same_0002.menuassets": "second",
	} {
		got, err := os.ReadFile(filepath.Join(outDir, "TextAsset", name))
		if err != nil {
			t.Fatalf("read disambiguated asset %q: %v", name, err)
		}
		if string(got) != want {
			t.Fatalf("disambiguated asset %q = %q, want %q", name, got, want)
		}
	}
}

// writeTypeTreelessTexture2DSerializedFile hand-writes a minimal v22 SerializedFile
// containing one Texture2D object whose serialized type carries no TypeTree.
func writeTypeTreelessTexture2DSerializedFile(t *testing.T, payload []byte) []byte {
	t.Helper()
	var metadata bytes.Buffer
	metadata.WriteString(defaultKCESUnityVersion)
	metadata.WriteByte(0)
	mustWriteLittle := func(value interface{}) {
		t.Helper()
		if err := binary.Write(&metadata, binary.LittleEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	mustWriteLittle(uint32(5)) // TargetPlatform: Windows Standalone
	metadata.WriteByte(0)      // TypeTreeEnabled = false
	mustWriteLittle(int32(1))  // type count
	mustWriteLittle(aba.ClassIDTexture2D)
	metadata.WriteByte(0)            // IsStrippedType
	mustWriteLittle(int16(-1))       // ScriptTypeIndex
	metadata.Write(make([]byte, 16)) // TypeHash
	mustWriteLittle(int32(1))        // object count
	for metadata.Len()%4 != 0 {
		metadata.WriteByte(0)
	}
	mustWriteLittle(int64(1))             // PathID
	mustWriteLittle(int64(0))             // ByteOffset
	mustWriteLittle(uint32(len(payload))) // ByteSize
	mustWriteLittle(int32(0))             // TypeIndex
	mustWriteLittle(int32(0))             // ScriptTypes
	mustWriteLittle(int32(0))             // ExternalFiles
	mustWriteLittle(uint32(0))            // RefTypes
	metadata.WriteByte(0)                 // UserInformation

	const headerSize = 48
	dataOffset := int64(headerSize + metadata.Len())
	for dataOffset%16 != 0 {
		dataOffset++
	}
	fileSize := dataOffset + int64(len(payload))

	var file bytes.Buffer
	mustWriteBig := func(value interface{}) {
		t.Helper()
		if err := binary.Write(&file, binary.BigEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	mustWriteBig(uint32(0))  // legacy MetadataSize
	mustWriteBig(uint32(0))  // legacy FileSize
	mustWriteBig(uint32(22)) // Version
	mustWriteBig(uint32(0))  // legacy DataOffset
	file.WriteByte(0)        // Endianness = Little
	file.Write(make([]byte, 3))
	mustWriteBig(uint32(metadata.Len()))
	mustWriteBig(fileSize)
	mustWriteBig(dataOffset)
	mustWriteBig(int64(0))
	file.Write(metadata.Bytes())
	for int64(file.Len()) < dataOffset {
		file.WriteByte(0)
	}
	file.Write(payload)
	return file.Bytes()
}
