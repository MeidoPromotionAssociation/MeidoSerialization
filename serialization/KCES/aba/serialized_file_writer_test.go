package aba

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// minimalTestTypeTree builds the smallest valid TypeTree for a class so tests can
// exercise writer mechanics through the tree-carrying native-object entry points.
func minimalTestTypeTree(classID int32, typeName string) TypeTreeType {
	tree := TypeTreeType{TypeId: classID, ScriptTypeIndex: -1}
	tree.StringBuffer = append(tree.StringBuffer, typeName...)
	tree.StringBuffer = append(tree.StringBuffer, 0)
	nameOff := uint32(len(tree.StringBuffer))
	tree.StringBuffer = append(tree.StringBuffer, "Base"...)
	tree.StringBuffer = append(tree.StringBuffer, 0)
	tree.Nodes = []TypeTreeNode{{Version: 1, NameStrOff: nameOff, ByteSize: -1}}
	return tree
}

// addTestObjectWithTree adds an opaque payload through the native-object entry point
// with a minimal tree, replacing the treeless raw-object calls used before every
// SerializedFile type was required to carry a complete TypeTree.
func addTestObjectWithTree(w *SerializedFileWriter, classID int32, typeName string, name string, loadName string, data []byte, pathID int64) int64 {
	payload := append([]byte(nil), data...)
	return w.AddNativeUnityObjectSourceWithLoadNameAndPathID(classID, minimalTestTypeTree(classID, typeName), name, loadName, SerializedObjectDataSource{
		Size:   uint32(len(payload)),
		Prefix: payload,
		WriteTo: func(out io.Writer) error {
			_, err := out.Write(payload)
			return err
		},
	}, pathID)
}

func TestSerializedFileWriter_TextAsset(t *testing.T) {
	w := NewSerializedFileWriter("2022.3.35f")

	script := []byte("hello world test data")
	w.AddTextAsset("test.menuassets", script)
	w.AddTextAsset("another.materialassets", []byte{1, 2, 3, 4})

	var buf bytes.Buffer
	if err := w.Write(&buf); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// 用现有 reader 验证
	af, err := ReadAssetsFile(buf.Bytes())
	if err != nil {
		t.Fatalf("ReadAssetsFile failed: %v", err)
	}

	entries := af.GetAssetEntries()
	t.Logf("Generated %d entries", len(entries))

	foundTextAsset := 0
	foundBundle := 0
	for _, e := range entries {
		t.Logf("  PathId=%d Type=%s Name=%q Size=%d", e.PathId, e.TypeName, e.Name, e.Size)
		if e.TypeId == ClassIDTextAsset {
			foundTextAsset++
		}
		if e.TypeId == classIDAssetBundle {
			foundBundle++
		}
	}

	if foundTextAsset != 2 {
		t.Errorf("expected 2 TextAssets, got %d", foundTextAsset)
	}
	if foundBundle != 1 {
		t.Errorf("expected 1 AssetBundle, got %d", foundBundle)
	}

	// 验证 TextAsset 数据可读取
	for _, e := range entries {
		if e.TypeId == ClassIDTextAsset && e.Name == "test.menuassets" {
			_, data, err := af.GetTextAssetData(&AssetInfo{
				PathId:     e.PathId,
				ByteOffset: e.Offset,
				ByteSize:   e.Size,
				TypeId:     e.TypeId,
			})
			if err != nil {
				t.Errorf("GetTextAssetData failed: %v", err)
			} else if !bytes.Equal(data, script) {
				t.Errorf("TextAsset data mismatch: got %d bytes, want %d", len(data), len(script))
			}
		}
	}
}

func TestSerializedFileWriterPreservesLongAssetNameInEntries(t *testing.T) {
	name := strings.Repeat("n", 5000)
	w := NewSerializedFileWriter("2022.3.35f")
	w.AddTextAsset(name, []byte("payload"))
	var out bytes.Buffer
	if err := w.Write(&out); err != nil {
		t.Fatalf("Write: %v", err)
	}
	af, err := ReadAssetsFile(out.Bytes())
	if err != nil {
		t.Fatalf("ReadAssetsFile: %v", err)
	}
	entries := af.GetAssetEntries()
	found := false
	for _, entry := range entries {
		if entry.TypeId != ClassIDTextAsset {
			continue
		}
		found = true
		if entry.Name != name {
			t.Fatalf("entry name length = %d, want %d", len(entry.Name), len(name))
		}
	}
	if !found {
		t.Fatal("TextAsset entry not found")
	}
}

func TestSerializedFileWriter_InAba(t *testing.T) {
	w := NewSerializedFileWriter("2022.3.35f")
	w.AddTextAsset("parts.menuassets", []byte("menu data"))

	var sfBuf bytes.Buffer
	if err := w.Write(&sfBuf); err != nil {
		t.Fatalf("Write SerializedFile failed: %v", err)
	}

	// 包装为 UnityFS .aba 文件
	entries := []AbaFileEntry{
		{Name: "CAB-test", Data: sfBuf.Bytes(), IsSerialized: true},
	}
	var abaBuffer bytes.Buffer
	if err := WriteAba(&abaBuffer, entries, &AbaWriteOptions{Compress: false}); err != nil {
		t.Fatalf("WriteAba failed: %v", err)
	}

	// 用 ReadAba 验证
	abaFile, err := ReadAba(bytes.NewReader(abaBuffer.Bytes()))
	if err != nil {
		t.Fatalf("ReadAba failed: %v", err)
	}

	fileData, err := abaFile.GetFileDataByName("CAB-test")
	if err != nil {
		t.Fatalf("GetFileDataByName failed: %v", err)
	}

	af, err := ReadAssetsFile(fileData)
	if err != nil {
		t.Fatalf("ReadAssetsFile from .aba failed: %v", err)
	}

	assetEntries := af.GetAssetEntries()
	t.Logf("Aba contains %d assets", len(assetEntries))
	for _, e := range assetEntries {
		t.Logf("  %s: %q", e.TypeName, e.Name)
	}
}

func TestSerializedFileWriter_MonoBehaviourTypeMetadata(t *testing.T) {
	w := NewSerializedFileWriter("2022.3.35f")
	raw := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	addTestObjectWithTree(w, ClassIDMonoBehaviour, "MonoBehaviour", "sample_monobehaviour", "sample_monobehaviour", raw, 0)

	var buf bytes.Buffer
	if err := w.Write(&buf); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	af, err := ReadAssetsFile(buf.Bytes())
	if err != nil {
		t.Fatalf("ReadAssetsFile failed: %v", err)
	}
	if len(af.Metadata.AssetInfos) != 2 {
		t.Fatalf("asset count got %d, want 2", len(af.Metadata.AssetInfos))
	}

	found := false
	for i, e := range af.GetAssetEntries() {
		if e.TypeId == ClassIDMonoBehaviour {
			found = true
			got, err := af.GetAssetData(&af.Metadata.AssetInfos[i])
			if err != nil {
				t.Fatalf("GetAssetData MonoBehaviour: %v", err)
			}
			if !bytes.Equal(got, raw) {
				t.Fatalf("MonoBehaviour raw data was rewritten: got %v want %v", got, raw)
			}
			break
		}
	}
	if !found {
		t.Fatalf("MonoBehaviour asset not found")
	}
}

func TestSerializedFileWriter_AssociatesDistinctMonoBehaviourScriptTypes(t *testing.T) {
	const scriptAPathID int64 = 101
	const scriptBPathID int64 = 202
	w := NewSerializedFileWriter("2022.3.35f")
	addTestObjectWithTree(w, ClassIDMonoScript, "MonoScript", "ScriptA", "ScriptA", []byte{1}, scriptAPathID)
	addTestObjectWithTree(w, ClassIDMonoScript, "MonoScript", "ScriptB", "ScriptB", []byte{2}, scriptBPathID)

	monoA := make([]byte, 28)
	monoA[12] = 1
	binary.LittleEndian.PutUint64(monoA[20:28], uint64(scriptAPathID))
	monoB := make([]byte, 28)
	monoB[12] = 1
	binary.LittleEndian.PutUint64(monoB[20:28], uint64(scriptBPathID))
	addTestObjectWithTree(w, ClassIDMonoBehaviour, "MonoBehaviour", "BehaviourA", "BehaviourA", monoA, 0)
	addTestObjectWithTree(w, ClassIDMonoBehaviour, "MonoBehaviour", "BehaviourB", "BehaviourB", monoB, 0)

	var out bytes.Buffer
	if err := w.Write(&out); err != nil {
		t.Fatalf("Write: %v", err)
	}
	af, err := ReadAssetsFile(out.Bytes())
	if err != nil {
		t.Fatalf("ReadAssetsFile: %v", err)
	}
	if len(af.Metadata.ScriptTypes) != 2 {
		t.Fatalf("ScriptTypes count = %d, want 2", len(af.Metadata.ScriptTypes))
	}
	if af.Metadata.ScriptTypes[0].LocalIdentifierInFile != scriptAPathID || af.Metadata.ScriptTypes[1].LocalIdentifierInFile != scriptBPathID {
		t.Fatalf("ScriptTypes = %+v", af.Metadata.ScriptTypes)
	}
	var monoTypes []TypeTreeType
	for _, serializedType := range af.Metadata.TypeTreeTypes {
		if serializedType.TypeId == ClassIDMonoBehaviour {
			monoTypes = append(monoTypes, serializedType)
		}
	}
	if len(monoTypes) != 2 || monoTypes[0].ScriptTypeIndex != 0 || monoTypes[1].ScriptTypeIndex != 1 {
		t.Fatalf("MonoBehaviour SerializedTypes = %+v", monoTypes)
	}
}

func TestSerializedFileWriter_RawObjectNameRewriteOnlyForNamedClasses(t *testing.T) {
	oldData, err := encodeTextAssetData("old_name", []byte("payload"))
	if err != nil {
		t.Fatalf("encodeTextAssetData: %v", err)
	}
	rewritten, err := rewriteLeadingAlignedName(oldData, "new_name")
	if err != nil {
		t.Fatalf("rewriteLeadingAlignedName: %v", err)
	}
	if bytes.Equal(rewritten, oldData) {
		t.Fatalf("expected leading name rewrite for valid named object data")
	}

	w := NewSerializedFileWriter("2022.3.35f")
	w.AddRawObject(ClassIDMaterial, "material_new", oldData)
	w.AddRawObject(ClassIDTransform, "transform_new", oldData)

	// The rewrite happens when the object is added, so the queued object bytes are
	// checked directly; treeless raw objects can no longer be written to a file.
	if len(w.objects) != 2 {
		t.Fatalf("queued object count = %d, want 2", len(w.objects))
	}
	material := w.objects[0].data
	if bytes.Equal(material, oldData) {
		t.Fatalf("Material raw data was not renamed")
	}
	if name, ok := readLeadingAlignedNameForTest(material); !ok || name != "material_new" {
		t.Fatalf("Material name got %q ok=%v", name, ok)
	}
	transform := w.objects[1].data
	if !bytes.Equal(transform, oldData) {
		t.Fatalf("Transform raw data should remain byte-identical")
	}
}

func TestSerializedFileWriter_PreservesOpaqueNamedObjectPayload(t *testing.T) {
	raw := []byte{1, 2, 3}
	w := NewSerializedFileWriter("2022.3.35f")
	pathID := addTestObjectWithTree(w, ClassIDTextAsset, "TextAsset", "internal_name", "load_name", raw, 42)
	if pathID != 42 {
		t.Fatalf("PathID got %d, want 42", pathID)
	}

	var buf bytes.Buffer
	if err := w.Write(&buf); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	af, err := ReadAssetsFile(buf.Bytes())
	if err != nil {
		t.Fatalf("ReadAssetsFile failed: %v", err)
	}
	info := af.GetAssetInfoByPathID(pathID)
	if info == nil {
		t.Fatalf("raw object info PathID=%d not found", pathID)
	}
	got, err := af.GetAssetData(info)
	if err != nil {
		t.Fatalf("GetAssetData: %v", err)
	}
	if !bytes.Equal(got, raw) {
		t.Fatalf("opaque raw object data got %v, want %v", got, raw)
	}
	containerNames, err := af.GetAssetBundleContainerMap()
	if err != nil {
		t.Fatalf("GetAssetBundleContainerMap: %v", err)
	}
	if containerNames[pathID] != "load_name" {
		t.Fatalf("container name got %q, want load_name", containerNames[pathID])
	}
}

func TestSerializedFileWriter_PreservingRawObjectDoesNotRewriteLeadingName(t *testing.T) {
	raw, err := encodeTextAssetData("original_name", []byte("payload"))
	if err != nil {
		t.Fatalf("encodeTextAssetData: %v", err)
	}
	w := NewSerializedFileWriter("2022.3.35f1")
	pathID := addTestObjectWithTree(w, ClassIDTextAsset, "TextAsset", "directory_name", "load_name", raw, 42)

	var buf bytes.Buffer
	if err := w.Write(&buf); err != nil {
		t.Fatalf("Write: %v", err)
	}
	af, err := ReadAssetsFile(buf.Bytes())
	if err != nil {
		t.Fatalf("ReadAssetsFile: %v", err)
	}
	info := af.GetAssetInfoByPathID(pathID)
	if info == nil {
		t.Fatalf("raw object info PathID=%d not found", pathID)
	}
	got, err := af.GetAssetData(info)
	if err != nil {
		t.Fatalf("GetAssetData: %v", err)
	}
	if !bytes.Equal(got, raw) {
		t.Fatalf("preserved raw data differs: got % x, want % x", got, raw)
	}
	if name, ok := readLeadingAlignedNameForTest(got); !ok || name != "original_name" {
		t.Fatalf("leading name = %q, ok=%v; want original_name", name, ok)
	}
}

func TestSerializedFileWriterStreamsRawObjectData(t *testing.T) {
	raw := []byte("streamed-object-payload")
	w := NewSerializedFileWriter("2022.3.35f1")
	var calls int64
	pathID := w.AddNativeUnityObjectSourceWithLoadNameAndPathID(
		ClassIDTransform,
		minimalTestTypeTree(ClassIDTransform, "Transform"),
		"streamed",
		"streamed_load",
		SerializedObjectDataSource{
			Size:   uint32(len(raw)),
			Prefix: append([]byte(nil), raw[:8]...),
			WriteTo: func(out io.Writer) error {
				calls++
				if _, err := out.Write(raw[:7]); err != nil {
					return err
				}
				_, err := out.Write(raw[7:])
				return err
			},
		},
		42,
	)
	var buf bytes.Buffer
	if err := w.Write(&buf); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if calls != 1 {
		t.Fatalf("source calls = %d, want 1", calls)
	}
	af, err := ReadAssetsFile(buf.Bytes())
	if err != nil {
		t.Fatalf("ReadAssetsFile: %v", err)
	}
	got, err := af.GetAssetData(af.GetAssetInfoByPathID(pathID))
	if err != nil {
		t.Fatalf("GetAssetData: %v", err)
	}
	if !bytes.Equal(got, raw) {
		t.Fatalf("streamed object = %q, want %q", got, raw)
	}
}

func TestSerializedFileWriterSizeDoesNotReadStreamedObjects(t *testing.T) {
	payload := []byte("sized payload")
	w := NewSerializedFileWriter("2022.3.35f1")
	var calls int64
	w.AddNativeUnityObjectSourceWithLoadNameAndPathID(
		ClassIDTransform,
		minimalTestTypeTree(ClassIDTransform, "Transform"),
		"sized",
		"sized",
		SerializedObjectDataSource{
			Size: uint32(len(payload)),
			WriteTo: func(out io.Writer) error {
				calls++
				_, err := out.Write(payload)
				return err
			},
		},
		1,
	)
	size, err := w.Size()
	if err != nil {
		t.Fatalf("Size: %v", err)
	}
	if calls != 0 {
		t.Fatalf("Size invoked streamed source %d times", calls)
	}
	var out bytes.Buffer
	if err := w.Write(&out); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if int64(out.Len()) != size {
		t.Fatalf("Write size = %d, Size = %d", out.Len(), size)
	}
}

func TestSerializedFileWriterRejectsIncorrectStreamedObjectSize(t *testing.T) {
	tests := []struct {
		name    string
		size    uint32
		payload []byte
	}{
		{name: "short", size: 4, payload: []byte("abc")},
		{name: "long", size: 3, payload: []byte("abcd")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := NewSerializedFileWriter("2022.3.35f1")
			w.AddNativeUnityObjectSourceWithLoadNameAndPathID(
				ClassIDTransform,
				minimalTestTypeTree(ClassIDTransform, "Transform"),
				"invalid",
				"invalid",
				SerializedObjectDataSource{
					Size: tt.size,
					WriteTo: func(out io.Writer) error {
						_, err := out.Write(tt.payload)
						return err
					},
				},
				1,
			)
			if err := w.Write(io.Discard); err == nil {
				t.Fatal("Write accepted an incorrect streamed object size")
			}
		})
	}
}

func TestSerializedFileWriterWritesCopiedExternalFileTable(t *testing.T) {
	externalFiles := []ExternalFile{{AssetPath: "virtual/path", Guid: [16]byte{8: 0x0f}, Type: 3, PathName: "Resources/unity_builtin_extra"}}
	w := NewSerializedFileWriter("2022.3.35f1")
	w.SetExternalFiles(externalFiles)
	externalFiles[0].PathName = "mutated"
	w.AddTextAsset("sample.txt", []byte("payload"))

	var buf bytes.Buffer
	if err := w.Write(&buf); err != nil {
		t.Fatalf("Write: %v", err)
	}
	af, err := ReadAssetsFile(buf.Bytes())
	if err != nil {
		t.Fatalf("ReadAssetsFile: %v", err)
	}
	want := ExternalFile{AssetPath: "virtual/path", Guid: [16]byte{8: 0x0f}, Type: 3, PathName: "Resources/unity_builtin_extra"}
	if len(af.Metadata.ExternalFiles) != 1 || af.Metadata.ExternalFiles[0] != want {
		t.Fatalf("ExternalFiles = %+v, want %+v", af.Metadata.ExternalFiles, want)
	}
}

func TestSerializedFileWriterRejectsNULInExternalFilePath(t *testing.T) {
	w := NewSerializedFileWriter("2022.3.35f1")
	w.SetExternalFiles([]ExternalFile{{PathName: "bad\x00path"}})
	var buf bytes.Buffer
	if err := w.Write(&buf); err == nil || !strings.Contains(err.Error(), "contains NUL") {
		t.Fatalf("Write error = %v, want NUL validation error", err)
	}
}

func TestRewriteLeadingAlignedNameSupportsLongAndEmptyNames(t *testing.T) {
	oldData, err := encodeTextAssetData(strings.Repeat("o", 5000), []byte("payload"))
	if err != nil {
		t.Fatalf("encodeTextAssetData: %v", err)
	}
	longName := strings.Repeat("n", 6000)
	rewritten, err := rewriteLeadingAlignedName(oldData, longName)
	if err != nil {
		t.Fatalf("rewrite long leading name: %v", err)
	}
	if got, ok := readLeadingAlignedNameForTest(rewritten); !ok || got != longName {
		t.Fatalf("long rewritten name length = %d, ok=%v; want %d", len(got), ok, len(longName))
	}

	rewritten, err = rewriteLeadingAlignedName(oldData, "")
	if err != nil {
		t.Fatalf("rewrite empty leading name: %v", err)
	}
	if got, ok := readLeadingAlignedNameForTest(rewritten); !ok || got != "" {
		t.Fatalf("empty rewritten name = %q, ok=%v", got, ok)
	}
}

func TestSerializedFileWriterRejectsZeroProgressWriter(t *testing.T) {
	w := NewSerializedFileWriter("2022.3.35f")
	w.AddTextAsset("test.menuassets", []byte("payload"))
	err := w.Write(zeroProgressSerializedWriter{})
	if err == nil || !strings.Contains(err.Error(), io.ErrShortWrite.Error()) {
		t.Fatalf("Write error = %v, want io.ErrShortWrite", err)
	}
}

type zeroProgressSerializedWriter struct{}

func (zeroProgressSerializedWriter) Write([]byte) (int, error) { return 0, nil }

func TestSerializedFileWriter_AssetBundleContainerKeepsLoadNames(t *testing.T) {
	raw, err := encodeTextAssetData("internal_name", []byte("payload"))
	if err != nil {
		t.Fatalf("encodeTextAssetData: %v", err)
	}

	w := NewSerializedFileWriter("2022.3.35f")
	pathID := addTestObjectWithTree(w, ClassIDTransform, "Transform", "load_name", "load_name", raw, 0)

	var buf bytes.Buffer
	if err := w.Write(&buf); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	af, err := ReadAssetsFile(buf.Bytes())
	if err != nil {
		t.Fatalf("ReadAssetsFile failed: %v", err)
	}
	containerNames, err := af.GetAssetBundleContainerMap()
	if err != nil {
		t.Fatalf("GetAssetBundleContainerMap: %v", err)
	}
	if containerNames[pathID] != "load_name" {
		t.Fatalf("container name got %q, want load_name", containerNames[pathID])
	}

	info := af.GetAssetInfoByPathID(pathID)
	if info == nil {
		t.Fatalf("raw object info PathID=%d not found", pathID)
	}
	got, err := af.GetAssetData(info)
	if err != nil {
		t.Fatalf("GetAssetData: %v", err)
	}
	if !bytes.Equal(got, raw) {
		t.Fatalf("raw object data changed")
	}
}

func TestSerializedFileWriter_SeparatesInternalNameAndLoadName(t *testing.T) {
	w := NewSerializedFileWriter("2022.3.35f")
	pathID := w.AddTextAssetWithLoadName("parts_personal002.menuassets", "assets/gamedata/parts/parts_personal002/parts_personal002.menuassets.bytes", []byte("payload"))

	var buf bytes.Buffer
	if err := w.Write(&buf); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	af, err := ReadAssetsFile(buf.Bytes())
	if err != nil {
		t.Fatalf("ReadAssetsFile failed: %v", err)
	}

	info := af.GetAssetInfoByPathID(pathID)
	if info == nil {
		t.Fatalf("TextAsset info PathID=%d not found", pathID)
	}
	name, _, err := af.GetTextAssetData(info)
	if err != nil {
		t.Fatalf("GetTextAssetData: %v", err)
	}
	if name != "parts_personal002.menuassets" {
		t.Fatalf("internal name got %q", name)
	}

	containerNames, err := af.GetAssetBundleContainerMap()
	if err != nil {
		t.Fatalf("GetAssetBundleContainerMap: %v", err)
	}
	if containerNames[pathID] != "assets/gamedata/parts/parts_personal002/parts_personal002.menuassets.bytes" {
		t.Fatalf("container name got %q", containerNames[pathID])
	}
}

func TestSerializedFileWriter_ReadAssetBundleContainerEntries(t *testing.T) {
	w := NewSerializedFileWriter("2022.3.35f")
	firstID := w.AddTextAsset("first.menuassets", []byte("first"))
	secondID := addTestObjectWithTree(w, ClassIDTransform, "Transform", "second_transform", "second_transform", []byte{1, 2, 3, 4}, 0)

	var buf bytes.Buffer
	if err := w.Write(&buf); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	af, err := ReadAssetsFile(buf.Bytes())
	if err != nil {
		t.Fatalf("ReadAssetsFile failed: %v", err)
	}

	var bundleInfo *AssetInfo
	for i := range af.Metadata.AssetInfos {
		if af.Metadata.AssetInfos[i].TypeId == ClassIDAssetBundle {
			bundleInfo = &af.Metadata.AssetInfos[i]
			break
		}
	}
	if bundleInfo == nil {
		t.Fatalf("AssetBundle info not found")
	}
	entries, err := af.GetAssetBundleContainerEntries(bundleInfo)
	if err != nil {
		t.Fatalf("GetAssetBundleContainerEntries: %v", err)
	}
	got := map[int64]string{}
	for _, entry := range entries {
		if entry.FileID != 0 {
			t.Fatalf("entry %q FileID got %d", entry.Name, entry.FileID)
		}
		got[entry.PathID] = entry.Name
	}
	if got[firstID] != "first.menuassets" {
		t.Fatalf("first entry got %q", got[firstID])
	}
	if got[secondID] != "second_transform" {
		t.Fatalf("second entry got %q", got[secondID])
	}
}

func TestSerializedFileWriter_PreservesPreferredRawObjectPathID(t *testing.T) {
	w := NewSerializedFileWriter("2022.3.35f")
	preferredID := int64(-1466831684398908746)
	gotID := addTestObjectWithTree(w, ClassIDMonoBehaviour, "MonoBehaviour", "sample_monobehaviour", "sample_monobehaviour", []byte{1, 2, 3, 4}, preferredID)
	if gotID != preferredID {
		t.Fatalf("AddRawObjectWithPathID got %d, want %d", gotID, preferredID)
	}

	var buf bytes.Buffer
	if err := w.Write(&buf); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	af, err := ReadAssetsFile(buf.Bytes())
	if err != nil {
		t.Fatalf("ReadAssetsFile failed: %v", err)
	}
	info := af.GetAssetInfoByPathID(preferredID)
	if info == nil {
		t.Fatalf("PathID %d not found after write", preferredID)
	}
	if info.TypeId != ClassIDMonoBehaviour {
		t.Fatalf("PathID %d type got %d, want MonoBehaviour", preferredID, info.TypeId)
	}
	containerNames, err := af.GetAssetBundleContainerMap()
	if err != nil {
		t.Fatalf("GetAssetBundleContainerMap: %v", err)
	}
	if containerNames[preferredID] != "sample_monobehaviour" {
		t.Fatalf("container name got %q", containerNames[preferredID])
	}
}

func TestSerializedFileWriter_DuplicatePreferredPathIDFallsBack(t *testing.T) {
	w := NewSerializedFileWriter("2022.3.35f")
	firstID := w.AddRawObjectWithPathID(ClassIDTransform, "first", []byte{1, 2, 3, 4}, 42)
	secondID := w.AddRawObjectWithPathID(ClassIDTransform, "second", []byte{5, 6, 7, 8}, 42)
	if firstID != 42 {
		t.Fatalf("firstID got %d, want 42", firstID)
	}
	if secondID == 42 || secondID == 0 {
		t.Fatalf("secondID got invalid fallback %d", secondID)
	}
}

func readLeadingAlignedNameForTest(data []byte) (string, bool) {
	if len(data) < 4 {
		return "", false
	}
	n := int(bytesToLittleUint32(data[:4]))
	if n < 0 || 4+n > len(data) {
		return "", false
	}
	return string(data[4 : 4+n]), true
}

func bytesToLittleUint32(data []byte) uint32 {
	return uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24
}

func legacyHandBuiltTexture2DTestTree() TypeTreeType {
	tree := TypeTreeType{TypeId: ClassIDTexture2D, ScriptTypeIndex: -1}
	stringOffset := func(value string) uint32 {
		offset := uint32(len(tree.StringBuffer))
		tree.StringBuffer = append(tree.StringBuffer, value...)
		tree.StringBuffer = append(tree.StringBuffer, 0)
		return offset
	}
	for _, want := range legacyTexture2DTreeSignature {
		tree.Nodes = append(tree.Nodes, TypeTreeNode{
			Version:    1,
			Level:      want.level,
			TypeStrOff: stringOffset(want.typeName),
			NameStrOff: stringOffset(want.fieldName),
			ByteSize:   want.byteSize,
			Index:      int32(len(tree.Nodes)),
			MetaFlags:  want.metaFlags,
		})
	}
	return tree
}

func TestSerializedFileWriter_UpgradesLegacyZeroHashTexture2DTree(t *testing.T) {
	object, err := NewNativeTexture2DObject("legacy.tex", 2, 2, make([]byte, 16))
	if err != nil {
		t.Fatal(err)
	}
	object.TypeTree = legacyHandBuiltTexture2DTestTree()

	w := NewSerializedFileWriter("")
	w.AddNativeUnityObjectWithLoadNameAndPathID(object, "legacy.tex", "legacy.tex", 0)
	var buf bytes.Buffer
	if err := w.Write(&buf); err != nil {
		t.Fatalf("Write: %v", err)
	}
	af, err := ReadAssetsFileRange(int64(buf.Len()), func(off, size int64) ([]byte, error) {
		return buf.Bytes()[off : off+size], nil
	})
	if err != nil {
		t.Fatal(err)
	}
	official := unity2022Texture2DTypeTree()
	found := false
	for _, tt := range af.Metadata.TypeTreeTypes {
		if tt.TypeId != ClassIDTexture2D {
			continue
		}
		found = true
		if len(tt.Nodes) != len(official.Nodes) {
			t.Fatalf("emitted Texture2D tree has %d nodes, want official %d", len(tt.Nodes), len(official.Nodes))
		}
		if tt.TypeHash != official.TypeHash {
			t.Fatalf("emitted Texture2D TypeHash = %x, want official %x", tt.TypeHash, official.TypeHash)
		}
	}
	if !found {
		t.Fatal("no Texture2D type emitted")
	}
}

func TestSerializedFileWriter_PreservesUnknownZeroHashTree(t *testing.T) {
	object, err := NewNativeTexture2DObject("unknown.tex", 2, 2, make([]byte, 16))
	if err != nil {
		t.Fatal(err)
	}
	tree := legacyHandBuiltTexture2DTestTree()
	tree.Nodes[5].ByteSize = 8
	object.TypeTree = tree

	w := NewSerializedFileWriter("")
	w.AddNativeUnityObjectWithLoadNameAndPathID(object, "unknown.tex", "unknown.tex", 0)
	var buf bytes.Buffer
	if err := w.Write(&buf); err != nil {
		t.Fatalf("Write: %v", err)
	}
	af, err := ReadAssetsFileRange(int64(buf.Len()), func(off, size int64) ([]byte, error) {
		return buf.Bytes()[off : off+size], nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range af.Metadata.TypeTreeTypes {
		if tt.TypeId != ClassIDTexture2D {
			continue
		}
		if len(tt.Nodes) != len(tree.Nodes) || tt.TypeHash != ([16]byte{}) {
			t.Fatalf("unknown zero-hash tree was rewritten: nodes=%d hash=%x", len(tt.Nodes), tt.TypeHash)
		}
		return
	}
	t.Fatal("no Texture2D type emitted")
}

func TestSerializedFileWriter_HealsLegacyErrorSampleTexture(t *testing.T) {
	f, err := os.Open(filepath.Join("..", "..", "..", "testdata", "error", "test.aba"))
	if os.IsNotExist(err) {
		t.Skip("no legacy error sample")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	bundle, err := ReadAba(f)
	if err != nil {
		t.Fatal(err)
	}
	for directoryIndex, entry := range bundle.BlockInfo.DirectoryInfos {
		if !entry.IsSerialized() {
			continue
		}
		directoryIndex := directoryIndex
		af, err := ReadAssetsFileRange(entry.DecompressedSize, func(offset, size int64) ([]byte, error) {
			return bundle.GetFileDataRange(int64(directoryIndex), offset, size)
		})
		if err != nil {
			t.Fatal(err)
		}
		for infoIndex := range af.Metadata.AssetInfos {
			info := &af.Metadata.AssetInfos[infoIndex]
			if info.TypeId != ClassIDTexture2D {
				continue
			}
			tree, err := af.AssetTypeTree(info)
			if err != nil {
				t.Fatal(err)
			}
			if !isLegacyHandBuiltTexture2DTree(&tree) {
				t.Fatalf("error-sample texture tree no longer matches the legacy signature: nodes=%d hash=%x", len(tree.Nodes), tree.TypeHash)
			}
			upgraded := upgradeZeroHashNativeTypeTree(&tree)
			if upgraded.TypeHash == ([16]byte{}) || len(upgraded.Nodes) != len(unity2022Texture2DTypeTree().Nodes) {
				t.Fatalf("legacy sample tree was not upgraded: nodes=%d hash=%x", len(upgraded.Nodes), upgraded.TypeHash)
			}
			return
		}
	}
	t.Skip("error sample carries no Texture2D")
}
