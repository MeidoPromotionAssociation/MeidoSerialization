package aba

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestSerializedFileWriter_TextAsset(t *testing.T) {
	w := NewSerializedFileWriter("2021.3.37f1")

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
	w := NewSerializedFileWriter("2021.3.37f1")
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
	if len(entries) < 1 {
		t.Fatal("no asset entries")
	}
	if entries[0].Name != name {
		t.Fatalf("entry name length = %d, want %d", len(entries[0].Name), len(name))
	}
}

func TestSerializedFileWriter_InAba(t *testing.T) {
	w := NewSerializedFileWriter("2021.3.37f1")
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
	w := NewSerializedFileWriter("2021.3.37f1")
	raw := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	w.AddRawObject(ClassIDMonoBehaviour, "sample_monobehaviour", raw)

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

	w := NewSerializedFileWriter("2021.3.37f1")
	w.AddRawObject(ClassIDMaterial, "material_new", oldData)
	w.AddRawObject(ClassIDTransform, "transform_new", oldData)

	var buf bytes.Buffer
	if err := w.Write(&buf); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	af, err := ReadAssetsFile(buf.Bytes())
	if err != nil {
		t.Fatalf("ReadAssetsFile failed: %v", err)
	}

	for i, e := range af.GetAssetEntries() {
		switch e.TypeId {
		case ClassIDMaterial:
			got, err := af.GetAssetData(&af.Metadata.AssetInfos[i])
			if err != nil {
				t.Fatalf("GetAssetData Material: %v", err)
			}
			if bytes.Equal(got, oldData) {
				t.Fatalf("Material raw data was not renamed")
			}
			name, ok := readLeadingAlignedNameForTest(got)
			if !ok || name != "material_new" {
				t.Fatalf("Material name got %q ok=%v", name, ok)
			}
		case ClassIDTransform:
			got, err := af.GetAssetData(&af.Metadata.AssetInfos[i])
			if err != nil {
				t.Fatalf("GetAssetData Transform: %v", err)
			}
			if !bytes.Equal(got, oldData) {
				t.Fatalf("Transform raw data should remain byte-identical")
			}
		}
	}
}

func TestSerializedFileWriter_PreservesOpaqueNamedObjectPayload(t *testing.T) {
	raw := []byte{1, 2, 3}
	w := NewSerializedFileWriter("2021.3.37f1")
	pathID := w.AddRawObjectWithLoadNameAndPathID(ClassIDTextAsset, "internal_name", "load_name", raw, 42)
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
	w := NewSerializedFileWriter("2021.3.37f1")
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

	w := NewSerializedFileWriter("2021.3.37f1")
	pathID := w.AddRawObject(ClassIDTransform, "load_name", raw)

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
	w := NewSerializedFileWriter("2021.3.37f1")
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
	w := NewSerializedFileWriter("2021.3.37f1")
	firstID := w.AddTextAsset("first.menuassets", []byte("first"))
	secondID := w.AddRawObject(ClassIDTransform, "second_transform", []byte{1, 2, 3, 4})

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
	w := NewSerializedFileWriter("2021.3.37f1")
	preferredID := int64(-1466831684398908746)
	gotID := w.AddRawObjectWithPathID(ClassIDMonoBehaviour, "sample_monobehaviour", []byte{1, 2, 3, 4}, preferredID)
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
	w := NewSerializedFileWriter("2021.3.37f1")
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
