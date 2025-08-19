package tools

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// NewCSVReaderSkipUTF8BOM 创建一个 CSV Reader，并在存在时跳过 UTF-8 BOM。
func NewCSVReaderSkipUTF8BOM(r io.Reader) *csv.Reader {
	br := bufio.NewReader(r)
	if b, err := br.Peek(3); err == nil && len(b) == 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		_, _ = br.Discard(3)
	}
	return csv.NewReader(br)
}

// WriteCSVWithUTF8BOM 将 records 写入 w，并在文件开头写入 UTF-8 BOM，便于 Microsoft Excel 识别。
func WriteCSVWithUTF8BOM(w io.Writer, records [][]string) error {
	bw := bufio.NewWriter(w)
	if _, err := bw.Write(utf8BOM); err != nil {
		return fmt.Errorf("write UTF-8 BOM: %w", err)
	}

	writer := csv.NewWriter(bw)
	err := writer.WriteAll(records)
	if err != nil {
		return fmt.Errorf("write CSV failed: %w", err)
	}
	if err = writer.Error(); err != nil {
		return fmt.Errorf("write CSV failed: %w", err)
	}

	if err = bw.Flush(); err != nil {
		return fmt.Errorf("failed to flush buffered writer: %w", err)
	}
	return nil
}
