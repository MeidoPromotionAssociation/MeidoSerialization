package KCES

import (
	"bufio"
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/tools"
)

const neiExtension = ".nei"

// NeiService 专门处理 KCES 使用的 .nei 共用格式 / NeiService handles the shared .nei format used by KCES
type NeiService struct{}

// IsKCESNeiFile 判断路径是否为 KCES 使用的 .nei 文件
// IsKCESNeiFile reports whether a path names a .nei file used by KCES
func IsKCESNeiFile(path string) bool {
	return !strings.HasSuffix(strings.ToLower(path), ".json") && strings.EqualFold(filepath.Ext(path), neiExtension)
}

// ReadNeiFile 读取并解码 .nei 文件
// ReadNeiFile reads and decodes a .nei file
func (s *NeiService) ReadNeiFile(path string) (*serializationKCES.Nei, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open .nei file %q: %w", path, err)
	}
	defer file.Close()
	value, err := serializationKCES.ReadNei(bufio.NewReader(file), nil)
	if err != nil {
		return nil, fmt.Errorf("decode .nei file %q: %w", path, err)
	}
	return value, nil
}

// WriteNeiFile 编码并写入 .nei 文件
// WriteNeiFile encodes and writes a .nei file
func (s *NeiService) WriteNeiFile(path string, value *serializationKCES.Nei) error {
	return writeNeiFile(path, value)
}

// ConvertNeiToCSV 将 .nei 文件转换为 CSV
// ConvertNeiToCSV converts a .nei file to CSV
func (s *NeiService) ConvertNeiToCSV(inputPath string, outputPath string) error {
	value, err := s.ReadNeiFile(inputPath)
	if err != nil {
		return err
	}
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create CSV file %q: %w", outputPath, err)
	}
	writeErr := tools.WriteCSVWithUTF8BOM(file, value.Data)
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("write CSV file %q: %w", outputPath, writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close CSV file %q: %w", outputPath, closeErr)
	}
	return nil
}

// ConvertCSVToNei 将 CSV 转换为 .nei 文件
// ConvertCSVToNei converts CSV to a .nei file
func (s *NeiService) ConvertCSVToNei(inputPath string, outputPath string) error {
	file, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("open CSV file %q: %w", inputPath, err)
	}
	records, readErr := tools.NewCSVReaderSkipUTF8BOM(file, 0).ReadAll()
	closeErr := file.Close()
	if readErr != nil {
		return fmt.Errorf("read CSV file %q: %w", inputPath, readErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close CSV file %q: %w", inputPath, closeErr)
	}
	if uint64(len(records)) > math.MaxUint32 {
		return fmt.Errorf("CSV row count %d exceeds Uint32", uint64(len(records)))
	}
	var maxCols uint32
	for _, record := range records {
		if uint64(len(record)) > math.MaxUint32 {
			return fmt.Errorf("CSV column count %d exceeds Uint32", uint64(len(record)))
		}
		cols := uint32(len(record))
		if cols > maxCols {
			maxCols = cols
		}
	}
	data := make([][]string, len(records))
	for index, record := range records {
		row := make([]string, maxCols)
		copy(row, record)
		data[index] = row
	}
	// NewNei 固定使用 UTF-8，KCES 的 crc.dll 按 UTF-8 解码单元格，写出 Shift-JIS 会让游戏读到乱码
	// NewNei always selects UTF-8 because KCES's crc.dll decodes cells as UTF-8 and writing Shift-JIS would make the game read garbage
	return s.WriteNeiFile(outputPath, serializationKCES.NewNei(uint32(len(records)), maxCols, data))
}

// WriteNeiFile 为聚合 service 保留 .nei 直接写入 API
// WriteNeiFile preserves the direct .nei writer on the aggregate service
func (s *DataService) WriteNeiFile(path string, value *serializationKCES.Nei) error {
	return writeNeiFile(path, value)
}

// writeNeiFile 编码并直接写入原生 .nei 数据
// writeNeiFile encodes and directly writes native .nei data
func writeNeiFile(path string, value *serializationKCES.Nei) error {
	if value == nil {
		return fmt.Errorf("encode KCES shared nei: nil nei")
	}
	var encoded bytes.Buffer
	if err := value.Dump(&encoded); err != nil {
		return fmt.Errorf("encode KCES shared nei: %w", err)
	}
	if err := os.WriteFile(path, encoded.Bytes(), 0644); err != nil {
		return fmt.Errorf("write .nei file %q: %w", path, err)
	}
	return nil
}
