package application

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/aba"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/ct"
)

func TestEngineContentTableArchiveAndEditingJSON(t *testing.T) {
	table := &ct.ContentTable{Version: 1000, Raw: make([]byte, ct.HeaderSize), Files: map[string]ct.VirtualFile{}}
	catalogName := "synthetic"
	catalog, err := ct.EncodeCatalog(&ct.AssetBundleCatalog{
		Kind: ct.CatalogKindAssetBundle, Version: 1000, Name: &catalogName,
		ResourceFileNames: []*string{}, ExtensionList: []*string{}, Items: []*ct.CatalogItem{},
	})
	if err != nil {
		t.Fatal(err)
	}
	table.AddFile("catalog", catalog)
	table.AddFile("folder/item.bin", []byte("payload"))
	var native bytes.Buffer
	if err := ct.WriteContentTable(&native, table); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(EngineOptions{})
	ctx := context.Background()

	entries, err := engine.ListArchive(ctx, NewBytesSource("sample.ct", native.Bytes()), "kces.ct")
	if err != nil {
		t.Fatalf("ListArchive: %v", err)
	}
	if len(entries) != 2 || entries[1].Name != "folder/item.bin" || entries[1].Size != 7 {
		t.Fatalf("entries = %+v", entries)
	}
	var extracted bytes.Buffer
	artifact, err := engine.ExtractArchiveEntry(ctx, NewBytesSource("sample.ct", native.Bytes()), "kces.ct", "folder/item.bin", &extracted)
	if err != nil || extracted.String() != "payload" || artifact.Name != "item.bin" {
		t.Fatalf("ExtractArchiveEntry: artifact=%+v data=%q err=%v", artifact, extracted.String(), err)
	}

	jsonArtifact, editingJSON, err := engine.ConvertBytes(ctx, ConvertRequest{
		Source: NewBytesSource("sample.ct", native.Bytes()), FormatID: "kces.ct", To: RepresentationEditingJSON,
	})
	if err != nil {
		t.Fatalf("Convert CT to JSON: %v", err)
	}
	if jsonArtifact.Name != "sample.ct.json" {
		t.Fatalf("JSON artifact = %+v", jsonArtifact)
	}
	if _, err := engine.Validate(ctx, NewBytesSource(jsonArtifact.Name, editingJSON), "kces.ct"); err != nil {
		t.Fatalf("Validate CT JSON: %v", err)
	}
	backArtifact, back, err := engine.ConvertBytes(ctx, ConvertRequest{
		Source: NewBytesSource(jsonArtifact.Name, editingJSON), FormatID: "kces.ct", To: RepresentationNative,
	})
	if err != nil || backArtifact.Name != "sample.ct" {
		t.Fatalf("Convert CT to native: artifact=%+v err=%v", backArtifact, err)
	}
	upperArtifact, _, err := engine.ConvertBytes(ctx, ConvertRequest{
		Source: NewBytesSource("SAMPLE.JSON", editingJSON), FormatID: "kces.ct", To: RepresentationNative,
	})
	if err != nil || upperArtifact.Name != "SAMPLE" {
		t.Fatalf("uppercase JSON native artifact=%+v err=%v", upperArtifact, err)
	}
	decoded, err := ct.ReadContentTable(bytes.NewReader(back))
	if err != nil {
		t.Fatal(err)
	}
	data, err := decoded.GetFileData("folder/item.bin")
	if err != nil || string(data) != "payload" {
		t.Fatalf("round-trip entry = %q, err=%v", data, err)
	}
}

func TestEngineStreamsAbaEntryAcrossRanges(t *testing.T) {
	payload := make([]byte, (8<<20)+17)
	for i := range payload {
		payload[i] = byte(i % 251)
	}
	var bundle bytes.Buffer
	if err := aba.WriteAba(&bundle, []aba.AbaFileEntry{{Name: "archive/data.resS", Data: payload}}, &aba.AbaWriteOptions{Compress: true}); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(EngineOptions{MaxInputBytes: 32 << 20, MaxOutputBytes: 16 << 20})
	var extracted bytes.Buffer
	artifact, err := engine.ExtractArchiveEntry(
		context.Background(), NewBytesSource("sample.aba", bundle.Bytes()), "kces.aba", "archive/data.resS", &extracted,
	)
	if err != nil {
		t.Fatalf("ExtractArchiveEntry: %v", err)
	}
	if artifact.Size != int64(len(payload)) || !bytes.Equal(extracted.Bytes(), payload) {
		t.Fatalf("artifact=%+v extracted=%d want=%d", artifact, extracted.Len(), len(payload))
	}
}

func TestEngineRejectsOversizedContentTableEntryBeforeWriting(t *testing.T) {
	table := &ct.ContentTable{Version: 1000, Raw: make([]byte, ct.HeaderSize), Files: map[string]ct.VirtualFile{}}
	table.AddFile("large.bin", bytes.Repeat([]byte{'x'}, 65))
	var native bytes.Buffer
	if err := ct.WriteContentTable(&native, table); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(EngineOptions{MaxInputBytes: 1 << 20, MaxOutputBytes: 64})
	var output bytes.Buffer
	_, err := engine.ExtractArchiveEntry(
		context.Background(), NewBytesSource("sample.ct", native.Bytes()), "kces.ct", "large.bin", &output,
	)
	if CodeOf(err) != CodeResourceExhausted {
		t.Fatalf("oversized CT entry error = %v", err)
	}
	if output.Len() != 0 {
		t.Fatalf("oversized CT entry wrote %d bytes", output.Len())
	}
}

func TestEngineRejectsEditingJSONForNativeOnlyArchive(t *testing.T) {
	engine := NewEngine(EngineOptions{})
	_, err := engine.Validate(context.Background(), NewBytesSource("sample.arc.json", []byte(`{"not":"an archive"}`)), "com3d2.arc")
	if CodeOf(err) != CodeUnsupported {
		t.Fatalf("native-only archive JSON validation error = %v", err)
	}
}

func TestArchivePagerBindsTokenToServerFormatAndExactArchive(t *testing.T) {
	engine := NewEngine(EngineOptions{})
	listingA, err := engine.ListArchiveListing(context.Background(), NewBytesSource(
		"a.ct", contentTableArchive(t, map[string][]byte{"a.bin": []byte("one"), "b.bin": []byte("two")}),
	), "kces.ct")
	if err != nil {
		t.Fatal(err)
	}
	listingB, err := engine.ListArchiveListing(context.Background(), NewBytesSource(
		"b.ct", contentTableArchive(t, map[string][]byte{"a.bin": []byte("ONE"), "b.bin": []byte("TWO")}),
	), "kces.ct")
	if err != nil {
		t.Fatal(err)
	}
	pager, err := NewArchivePager()
	if err != nil {
		t.Fatal(err)
	}
	token, err := pager.Encode(listingA, 1)
	if err != nil {
		t.Fatal(err)
	}
	if token == "1" || token == "" {
		t.Fatalf("page token is not opaque: %q", token)
	}
	if offset, err := pager.Decode(listingA, token); err != nil || offset != 1 {
		t.Fatalf("decode matching token = %d, %v", offset, err)
	}

	nonCanonicalLastByte, ok := map[byte]byte{'A': 'B', 'Q': 'R', 'g': 'h', 'w': 'x'}[token[len(token)-1]]
	if !ok {
		t.Fatalf("unexpected final base64 character in token %q", token)
	}
	tampered := token[:len(token)-1] + string(nonCanonicalLastByte)
	wrongFormat := listingA
	wrongFormat.FormatID = "kces.virtualdirectory"
	otherPager, err := NewArchivePager()
	if err != nil {
		t.Fatal(err)
	}
	for name, test := range map[string]struct {
		pager   *ArchivePager
		listing ArchiveListing
		token   string
	}{
		"tampered":        {pager: pager, listing: listingA, token: tampered},
		"another archive": {pager: pager, listing: listingB, token: token},
		"another format":  {pager: pager, listing: wrongFormat, token: token},
		"another server":  {pager: otherPager, listing: listingA, token: token},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := test.pager.Decode(test.listing, test.token); CodeOf(err) != CodeInvalidArgument {
				t.Fatalf("Decode error = %v", err)
			}
		})
	}
}

func TestArchiveListingUsesIndependentInputAndEntryBudgets(t *testing.T) {
	native := contentTableArchive(t, map[string][]byte{
		"a.bin": []byte("a"), "b.bin": []byte("b"), "c.bin": []byte("c"),
	})
	for name, engine := range map[string]*Engine{
		"input bytes": NewEngine(EngineOptions{
			MaxInputBytes: 1 << 20, MaxArchiveListingBytes: int64(len(native) - 1), MaxArchiveEntries: 10,
		}),
		"entry count": NewEngine(EngineOptions{
			MaxInputBytes: 1 << 20, MaxArchiveListingBytes: 1 << 20, MaxArchiveEntries: 2,
		}),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := engine.ListArchive(context.Background(), NewBytesSource("sample.ct", native), "kces.ct")
			if CodeOf(err) != CodeResourceExhausted {
				t.Fatalf("ListArchive error = %v", err)
			}
		})
	}
}

func TestArchiveTraversalChecksContext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.ct")
	if err := os.WriteFile(path, contentTableArchive(t, map[string][]byte{"a.bin": []byte("a")}), 0644); err != nil {
		t.Fatal(err)
	}
	ctx := &cancelAfterContext{Context: context.Background(), remaining: 1}
	_, err := NewEngine(EngineOptions{}).listArchivePath(ctx, "kces.ct", path)
	if CodeOf(err) != CodeCanceled {
		t.Fatalf("listArchivePath error = %v", err)
	}
}

type cancelAfterContext struct {
	context.Context
	remaining int
}

func (c *cancelAfterContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *cancelAfterContext) Done() <-chan struct{}       { return nil }
func (c *cancelAfterContext) Value(key any) any           { return c.Context.Value(key) }
func (c *cancelAfterContext) Err() error {
	if c.remaining == 0 {
		return context.Canceled
	}
	c.remaining--
	return nil
}

func contentTableArchive(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	table := &ct.ContentTable{Version: 1000, Raw: make([]byte, ct.HeaderSize), Files: map[string]ct.VirtualFile{}}
	for name, data := range files {
		table.AddFile(name, data)
	}
	var native bytes.Buffer
	if err := ct.WriteContentTable(&native, table); err != nil {
		t.Fatal(err)
	}
	return native.Bytes()
}
