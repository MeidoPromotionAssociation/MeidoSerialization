package COM3D2

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/binaryio/stream"
)

func TestMenu(t *testing.T) {
	files, err := filepath.Glob("../../testdata/*.menu")
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

			br := bufio.NewReader(f)
			menu, err := ReadMenu(br)
			if err != nil {
				t.Fatalf("failed to read menu: %v", err)
			}

			// Test Dump
			var buf bytes.Buffer
			err = menu.Dump(&buf)
			if err != nil {
				t.Fatalf("failed to dump menu: %v", err)
			}

			// Re-read from dumped buffer
			br2 := bufio.NewReader(&buf)
			menu2, err := ReadMenu(br2)
			if err != nil {
				t.Fatalf("failed to re-read dumped menu: %v", err)
			}

			// Compare complete structure
			if !reflect.DeepEqual(menu, menu2) {
				t.Errorf("data mismatch after dump and re-read")
			}
		})
	}
}

func TestMenuDumpRecalculatesBodySizeAndRejectsEmptyCommand(t *testing.T) {
	menu := &Menu{
		Signature:   MenuSignature,
		Version:     777,
		SrcFileName: "source.menu",
		ItemName:    "item",
		Category:    "category",
		InfoText:    "info",
		BodySize:    123,
		Commands:    []Command{{Command: "command", Args: []string{"x"}}},
	}

	var wire bytes.Buffer
	if err := menu.Dump(&wire); err != nil {
		t.Fatalf("Dump: %v", err)
	}
	wantBodySize, err := menu.CalculateBodySize()
	if err != nil {
		t.Fatal(err)
	}
	if menu.BodySize != wantBodySize || menu.BodySize == 123 {
		t.Fatalf("Dump BodySize = %d, want recalculated %d", menu.BodySize, wantBodySize)
	}
	decoded, err := ReadMenu(bufio.NewReader(bytes.NewReader(wire.Bytes())))
	if err != nil {
		t.Fatalf("ReadMenu: %v", err)
	}
	if decoded.BodySize != wantBodySize || len(decoded.Commands) != 1 || decoded.Commands[0].Command != "command" || !reflect.DeepEqual(decoded.Commands[0].Args, []string{"x"}) {
		t.Fatalf("stored menu fields changed: %#v", decoded)
	}

	var reencoded bytes.Buffer
	if err := decoded.Dump(&reencoded); err != nil {
		t.Fatalf("re-Dump: %v", err)
	}
	if !bytes.Equal(reencoded.Bytes(), wire.Bytes()) {
		t.Fatal("menu wire changed after round-trip")
	}

	invalid := *menu
	invalid.Commands = []Command{{Command: "", Args: []string{"x"}}}
	wire.Reset()
	if err := invalid.Dump(&wire); err == nil {
		t.Fatal("Dump accepted an empty command name")
	}
	if wire.Len() != 0 {
		t.Fatalf("rejected empty command wrote %d bytes", wire.Len())
	}
}

func TestMenuDumpReplacesStaleNegativeBodySize(t *testing.T) {
	menu := &Menu{Signature: MenuSignature, BodySize: -1}
	var wire bytes.Buffer
	if err := menu.Dump(&wire); err != nil {
		t.Fatalf("Dump should recalculate BodySize: %v", err)
	}
	if menu.BodySize != 1 {
		t.Fatalf("recalculated BodySize = %d, want 1", menu.BodySize)
	}
}

// buildRawMenu 手工拼出一个 .menu 字节流。records 中每个元素是一条命令的全部字符串
// （第一个是命令名，其余是参数），因此可以构造出 ReadMenu 需要跳过的空命令名记录。
// 字符串与整数均复用库内的 stream.BinaryWriter，与生产代码保持同一套编码。
// buildRawMenu hand-assembles a .menu byte stream. Each element of records is the full string
// list of one command (the first is the command name, the rest are arguments), so callers can
// build the empty-command-name records that ReadMenu must skip. Strings and integers reuse the
// library's stream.BinaryWriter so the encoding matches production code.
func buildRawMenu(t *testing.T, records ...[]string) []byte {
	t.Helper()

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("build raw menu: %v", err)
		}
	}

	var body bytes.Buffer
	bodyWriter := stream.NewBinaryWriter(&body)
	for _, rec := range records {
		must(bodyWriter.WriteByte(byte(len(rec))))
		for _, s := range rec {
			must(bodyWriter.WriteString(s))
		}
	}
	must(bodyWriter.WriteByte(endByte))

	var out bytes.Buffer
	writer := stream.NewBinaryWriter(&out)
	must(writer.WriteString(MenuSignature))
	must(writer.WriteInt32(1000))
	must(writer.WriteString("source.menu"))
	must(writer.WriteString("item"))
	must(writer.WriteString("category"))
	must(writer.WriteString("info"))
	must(writer.WriteInt32(int32(body.Len())))
	must(writer.WriteBytes(body.Bytes()))
	return out.Bytes()
}

func TestReadMenuSkipsEmptyCommandRecords(t *testing.T) {
	// 官方编译器对源码中的空行（例如 `""`）会写出 ArgCount=1、命令名为空的记录，
	// 官方 Menu.ProcScriptBin 会忽略这类记录。读取时必须跳过整条记录而不是报错，
	// 且即使空命令名后面还跟着参数，也不能打乱后续命令的解析位置。
	// The official compiler emits ArgCount=1 records with an empty command name for blank
	// source lines (for example `""`), and the official Menu.ProcScriptBin ignores them.
	// Reading must skip the whole record instead of failing, and must keep the stream position
	// correct even when an empty command name is followed by arguments.
	wire := buildRawMenu(t,
		[]string{"name", "x"},
		[]string{""},             // ArgCount=1 + 空字符串 / empty string
		[]string{"", "leftover"}, // 空命令名 + 参数，整条跳过 / empty name with args, skip whole record
		[]string{"priority", "1"},
	)

	menu, err := ReadMenu(bufio.NewReader(bytes.NewReader(wire)))
	if err != nil {
		t.Fatalf("ReadMenu should skip empty command records, got: %v", err)
	}
	want := []Command{
		{Command: "name", Args: []string{"x"}},
		{Command: "priority", Args: []string{"1"}},
	}
	if !reflect.DeepEqual(menu.Commands, want) {
		t.Fatalf("Commands = %#v, want %#v", menu.Commands, want)
	}
}
