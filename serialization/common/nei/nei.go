// Package nei 实现 KISS 游戏共用的 .nei 加密 CSV 表格格式
// COM3D2 与 KCES 使用完全相同的容器结构，仅单元格文本的字符编码不同，因此实现放在共用包中，由各游戏包提供自身语义的封装
// Package nei implements the encrypted .nei CSV table format shared by KISS games
// COM3D2 and KCES use an identical container structure and differ only in the character encoding of cell text, so the implementation lives in a shared package and each game package wraps it with its own semantics
package nei

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// .nei
// Nei 格式是一种 AES（CBC 模式，无填充，手动补齐）+ 自定义 IV 生成与嵌入机制 加密的 CSV 文件
// 单元格文本的字符编码取决于读取该表的游戏：COM3D2 的 CsvParser 用 MultiByteToWideChar(932) 解码，即 Shift-JIS，而 KCES 改由 crc.dll 的 DLL_CSV_GetCellAsWideString 解码，实际写入的是 UTF-8
// 两种编码共用同一签名与结构，无法靠文件头区分，因此读取时按单元格字节内容探测
// 通常情况下应该是固定密钥的，除非 KISS 做了什么奇怪的事情
// 本模块实现参考自 https://github.com/usagirei/CM3D2.Toolkit 和 https://github.com/JustAGuest4168/CM3D2.Toolkit
// 感谢 @usagirei 和 @JustAGuest4168 完整的实现了加解密与转换
// Under MIT License
// .nei
// The Nei format is a CSV file encrypted with AES in CBC mode, manual alignment without standard padding, and custom IV generation and embedding
// The character encoding of cell text depends on the game reading the table: COM3D2's CsvParser decodes through MultiByteToWideChar(932), which is Shift-JIS, while KCES decodes through crc.dll's DLL_CSV_GetCellAsWideString and actually writes UTF-8
// Both encodings share the same signature and structure and cannot be told apart from the header, so reading detects the encoding from the cell bytes
// Normally it should use a fixed key unless KISS did something unusual
// This module's implementation is based on https://github.com/usagirei/CM3D2.Toolkit and https://github.com/JustAGuest4168/CM3D2.Toolkit
// Thanks to @usagirei and @JustAGuest4168 for fully implementing encryption, decryption, and conversion
// Under MIT License

// TextEncoding 表示 .nei 单元格文本使用的字符编码 / TextEncoding represents the character encoding used by .nei cell text
type TextEncoding string

const (
	// TextEncodingShiftJIS 是 COM3D2 读取 .nei 时使用的 Shift-JIS（CP932）编码
	// TextEncodingShiftJIS is the Shift-JIS (CP932) encoding COM3D2 uses when reading .nei
	TextEncodingShiftJIS TextEncoding = "Shift-JIS"
	// TextEncodingUTF8 是 KCES 读取 .nei 时使用的 UTF-8 编码
	// TextEncodingUTF8 is the UTF-8 encoding KCES uses when reading .nei
	TextEncodingUTF8 TextEncoding = "UTF-8"
)

// Table 表示解密后的 .nei CSV 表格数据
// Table represents decrypted .nei CSV table data
type Table struct {
	Rows uint32     `json:"Rows"` // CSV 的行数 / Number of CSV rows
	Cols uint32     `json:"Cols"` // CSV 的列数 / Number of CSV columns
	Data [][]string `json:"Data"` // CSV 的数据 [行][列] / CSV cell data indexed as [row][column]
	// 单元格文本的字符编码，读取时按内容探测，写出时为空等同于 Shift-JIS
	// Character encoding of cell text, detected from content while reading, and an empty value writes as Shift-JIS
	TextEncoding TextEncoding `json:"TextEncoding,omitempty"`
}

var (
	// Signature 是 .nei 解密后数据的固定签名
	// Signature is the fixed signature of decrypted .nei data
	Signature = []byte{0x77, 0x73, 0x76, 0xFF}

	// Key 是默认加密密钥
	// Key is the default encryption key
	Key = []byte{
		0xAA, 0xC9, 0xD2, 0x35,
		0x22, 0x87, 0x20, 0xF2,
		0x40, 0xC5, 0x61, 0x7C,
		0x01, 0xDF, 0x66, 0x54,
	}
)

// Read 从 r 中读取一个 .nei 文件，并解析为 Table 结构，单元格编码按内容探测
// key 传入 nil 则使用默认密钥
// Read reads a .nei file from r and parses it into a Table structure, detecting the cell encoding from content
// Passing nil as key selects the default key
func Read(r io.Reader, key []byte) (*Table, error) {
	if key == nil {
		key = Key
	}

	// 要解密，所以必须全读取
	// Decryption requires reading the complete input
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}

	// 解密数据
	// Decrypt the data
	decrypted, err := decryptData(data, key)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt data: %w", err)
	}

	buf := bytes.NewReader(decrypted)
	reader := stream.NewBinaryReader(buf)

	// 验证签名
	// Validate the signature
	signature, err := reader.ReadBytes(4)
	if err != nil {
		return nil, fmt.Errorf("failed to read signature: %w", err)
	}
	if !bytes.Equal(signature, Signature) {
		return nil, fmt.Errorf("invalid NEI signature, want %v, got %v", Signature, signature)
	}

	// 读取列数和行数
	// Read the column and row counts
	cols, err := reader.ReadUInt32()
	if err != nil {
		return nil, fmt.Errorf("failed to read col: %w", err)
	}
	rows, err := reader.ReadUInt32()
	if err != nil {
		return nil, fmt.Errorf("failed to read row: %w", err)
	}

	totalCells, err := checkedCellCount(rows, cols)
	if err != nil {
		return nil, err
	}
	// 每个单元格都有一个偏移和长度对
	// 在使用外部输入的维度分配数组前检查物理下界
	// Every cell has an offset/length pair
	// Check the physical lower bound before allocating from attacker-controlled dimensions
	if totalCells > int64(buf.Len())/8 {
		return nil, fmt.Errorf("NEI dimensions %d x %d require %d index entries, but only %d bytes remain", rows, cols, totalCells, buf.Len())
	}

	// 读取每个单元格的偏移量和长度
	// Read the offset and length of every cell
	offsets := make([]uint32, totalCells)
	lengths := make([]uint32, totalCells)
	for i := int64(0); i < totalCells; i++ {

		// 该单元格内容在字符串数据区中的起始偏移，以 0 为该数据区起点
		// 该值用于随机访问，解析器稍后按它定位单元格
		// This is the cell's starting offset relative to the beginning of the string-data area
		// It supports random access and is used below to locate the cell
		offset, err := reader.ReadUInt32()
		if err != nil {
			return nil, fmt.Errorf("failed to read offset: %w", err)
		}
		offsets[i] = offset

		length, err := reader.ReadUInt32()
		if err != nil {
			return nil, fmt.Errorf("failed to read length: %w", err)
		}

		lengths[i] = length
	}
	stringData, err := io.ReadAll(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read NEI string data: %w", err)
	}

	// 准备单元格二维数组
	// Prepare the two-dimensional cell array
	var data2D [][]string
	if cols != 0 {
		data2D = make([][]string, int64(rows))
		for i := int64(0); i < int64(rows); i++ {
			data2D[i] = make([]string, int64(cols))
		}
	}

	// 按索引表定位每个单元格，偏移是相对字符串数据区的
	// 不能假设单元格按索引顺序连续存放
	// 编码探测需要整表信息，因此先收集全部原始字节再统一解码
	// Locate every cell through the index table, with offsets relative to the string-data area
	// Cells cannot be assumed to be stored contiguously in index order
	// Encoding detection needs whole-table information, so all raw bytes are collected before decoding
	rawCells := make([][]byte, totalCells)
	for i := int64(0); i < totalCells; i++ {
		offset := uint64(offsets[i])
		length := uint64(lengths[i])

		if length > 0 {
			end := offset + length
			if end < offset || end > uint64(len(stringData)) {
				return nil, fmt.Errorf("NEI cell[%d] byte range [%d,%d) exceeds string data length %d", i, offset, end, len(stringData))
			}
			rawCells[i] = stringData[offset:end]
		}
	}

	encoding := DetectTextEncoding(rawCells)

	for i := int64(0); i < totalCells; i++ {
		if rawCells[i] == nil {
			continue
		}
		cellValue, err := decodeCell(rawCells[i], encoding)
		if err != nil {
			return nil, fmt.Errorf("failed to decode string: %w", err)
		}
		row := i / int64(cols)
		column := i % int64(cols)
		data2D[row][column] = strings.TrimRight(cellValue, "\x00")
	}

	return &Table{
		Rows:         rows,
		Cols:         cols,
		Data:         data2D,
		TextEncoding: encoding,
	}, nil
}

// Dump 将 table 写出到 w 中，格式与 .nei 兼容，单元格按 TextEncoding 编码
// Dump writes table to w in a format compatible with .nei, encoding cells according to TextEncoding
func (table *Table) Dump(w io.Writer) error {
	totalCells, err := validateShape(table)
	if err != nil {
		return err
	}
	buf := new(bytes.Buffer)
	writer := stream.NewBinaryWriter(buf)

	// 编码所有字符串
	// Encode all strings
	encodedValues := make([][]byte, totalCells)
	for rowIndex := int64(0); rowIndex < int64(len(table.Data)); rowIndex++ {
		row := table.Data[rowIndex]
		for colIndex := int64(0); colIndex < int64(table.Cols); colIndex++ {
			cellIndex := colIndex + rowIndex*int64(table.Cols)

			if row[colIndex] != "" {
				encoded, err := encodeCell(row[colIndex], table.TextEncoding)
				if err != nil {
					return fmt.Errorf("failed to encode string at row=%d col=%d value=%q: %w", rowIndex, colIndex, row[colIndex], err)
				}
				encodedValues[cellIndex] = encoded
			} else {
				encodedValues[cellIndex] = nil
			}
		}
	}

	// 写入文件头
	// Write the file header
	err = writer.WriteBytes(Signature)
	if err != nil {
		return fmt.Errorf("failed to write signature: %w", err)
	}

	// 写入列数
	// Write the column count
	err = writer.WriteUInt32(table.Cols)
	if err != nil {
		return fmt.Errorf("failed to write Cols: %w", err)
	}

	// 写入行数
	// Write the row count
	err = writer.WriteUInt32(table.Rows)
	if err != nil {
		return fmt.Errorf("failed to write Rows: %w", err)
	}

	// 写入索引表
	// Write the index table
	var totalLength uint64
	const maxUint32 = uint64(^uint32(0))
	for _, encoded := range encodedValues {
		var length uint64
		if encoded != nil {
			length = uint64(len(encoded)) + 1
			if length > maxUint32 {
				return fmt.Errorf("NEI encoded cell length %d exceeds UInt32", length)
			}
		}

		var offset uint64
		if length > 0 {
			offset = totalLength
			if offset > maxUint32 || totalLength+length > maxUint32 {
				return fmt.Errorf("NEI string data exceeds the UInt32 offset range")
			}
		}

		// 索引偏移
		// Index offset
		err = writer.WriteUInt32(uint32(offset))
		if err != nil {
			return fmt.Errorf("failed to write offset: %w", err)
		}
		// 索引长度
		// Index length
		err = writer.WriteUInt32(uint32(length))
		if err != nil {
			return fmt.Errorf("failed to write length: %w", err)
		}

		totalLength += length
	}

	// 写入字符串数据
	// Write the string data
	for _, encoded := range encodedValues {
		if encoded != nil {
			err = writer.WriteBytes(encoded)
			if err != nil {
				return fmt.Errorf("failed to write string: %w", err)
			}
			// 每个非空单元都追加一个 null 终止符
			// Append a null terminator to every non-empty cell
			err = writer.WriteByte(0x00)
			if err != nil {
				return fmt.Errorf("failed to write null terminator: %w", err)
			}
		}
	}

	// 加密数据
	// Encrypt the data
	encryptedData, err := encryptData(buf.Bytes(), Key, nil)
	if err != nil {
		return fmt.Errorf("failed to encrypt data: %w", err)
	}

	// 向 w 写入加密数据
	// Write the encrypted data to w
	err = binaryio.WriteBytes(w, encryptedData)
	if err != nil {
		return fmt.Errorf("failed to write encrypted data: %w", err)
	}

	return nil
}

// checkedCellCount 检查 UInt32 行列乘积是否能安全转换为当前平台的切片长度
// checkedCellCount verifies that the UInt32 row-by-column product can be safely converted to a slice length on the current platform
func checkedCellCount(rows, cols uint32) (int64, error) {
	cells := uint64(rows) * uint64(cols)
	if cells > uint64(math.MaxInt64) {
		return 0, fmt.Errorf("NEI cell count %d x %d exceeds Int64", rows, cols)
	}
	total := int64(cells)
	if total > int64(^uint(0)>>1) {
		return 0, fmt.Errorf("NEI cell count %d x %d exceeds this platform's slice limits", rows, cols)
	}
	return total, nil
}

// validateShape 验证 Table 的二维切片形状与线格式 Rows、Cols 一致
// validateShape verifies that the Table two-dimensional slice shape matches the wire Rows and Cols values
func validateShape(table *Table) (int64, error) {
	if table == nil {
		return 0, fmt.Errorf("nil NEI table")
	}
	totalCells, err := checkedCellCount(table.Rows, table.Cols)
	if err != nil {
		return 0, err
	}
	if table.Cols == 0 && len(table.Data) == 0 {
		// 线格式仅需 Rows 即可表示零列表格
		// 当行数可达到 UInt32 范围时，避免要求每行都对应一个空 Go 切片
		// Rows is sufficient to represent a zero-width table on the wire
		// Avoid requiring one empty Go slice per row when the count can span UInt32
		return totalCells, nil
	}
	if int64(len(table.Data)) != int64(table.Rows) {
		return 0, fmt.Errorf("NEI Rows=%d does not match Data row count %d", table.Rows, len(table.Data))
	}
	for rowIndex := int64(0); rowIndex < int64(len(table.Data)); rowIndex++ {
		row := table.Data[rowIndex]
		if int64(len(row)) != int64(table.Cols) {
			return 0, fmt.Errorf("NEI Cols=%d does not match Data[%d] column count %d", table.Cols, rowIndex, len(row))
		}
	}
	return totalCells, nil
}

// DetectTextEncoding 按单元格字节内容探测 .nei 表格使用的文本编码
// COM3D2 与 KCES 的 .nei 共用签名与结构，只能靠内容区分：KCES 写出的日文是合法 UTF-8，而 Shift-JIS 的假名与大多数汉字首字节落在 UTF-8 的非法首字节区
// 全部单元格都是 ASCII 时两种编码等价，返回 Shift-JIS 以保持既有默认
// DetectTextEncoding detects the text encoding of a .nei table from the cell bytes
// COM3D2 and KCES .nei files share a signature and structure and can only be told apart by content: the Japanese text KCES writes is valid UTF-8, while Shift-JIS kana and most kanji lead bytes fall in UTF-8's invalid lead-byte range
// The two encodings are equivalent when every cell is ASCII, so Shift-JIS is returned to keep the existing default
func DetectTextEncoding(cells [][]byte) TextEncoding {
	hasNonASCII := false
	for _, cell := range cells {
		for _, b := range cell {
			if b >= utf8.RuneSelf {
				hasNonASCII = true
				break
			}
		}
		if hasNonASCII {
			break
		}
	}
	if !hasNonASCII {
		return TextEncodingShiftJIS
	}

	// 任何一个单元格不是合法 UTF-8 就说明整表是 Shift-JIS
	// A single cell that is not valid UTF-8 proves the whole table is Shift-JIS
	for _, cell := range cells {
		if !utf8.Valid(cell) {
			return TextEncodingShiftJIS
		}
	}
	return TextEncodingUTF8
}

// decodeCell 按指定编码将单元格字节解码为 UTF-8 字符串
// decodeCell decodes cell bytes into a UTF-8 string using the selected encoding
func decodeCell(data []byte, encoding TextEncoding) (string, error) {
	if encoding == TextEncodingUTF8 {
		return string(data), nil
	}
	return shiftJISToString(data)
}

// encodeCell 按指定编码将 UTF-8 字符串编码为单元格字节，编码为空时使用 Shift-JIS
// encodeCell encodes a UTF-8 string into cell bytes using the selected encoding, falling back to Shift-JIS when the encoding is empty
func encodeCell(value string, encoding TextEncoding) ([]byte, error) {
	switch encoding {
	case TextEncodingUTF8:
		// 游戏按 UTF-8 解码，非法序列会读成乱码，因此拒绝写出
		// The game decodes as UTF-8 and would read invalid sequences as garbage, so writing them is rejected
		if !utf8.ValidString(value) {
			return nil, fmt.Errorf("value is not valid UTF-8")
		}
		return []byte(value), nil
	case TextEncodingShiftJIS, "":
		return stringToShiftJIS(value)
	default:
		return nil, fmt.Errorf("unknown NEI text encoding %q", encoding)
	}
}

// stringToShiftJIS 将UTF-8字符串转换为Shift-JIS字节数组
// stringToShiftJIS converts a UTF-8 string to a Shift-JIS byte array
func stringToShiftJIS(s string) ([]byte, error) {
	var out []byte
	for _, r := range s {
		if r == '�' {
			out = append(out, 0xfd)
			continue
		}
		if 0x80 <= r && r <= 0x9f {
			out = append(out, byte(r))
			continue
		}
		encoded, _, err := transform.Bytes(japanese.ShiftJIS.NewEncoder(), []byte(string(r)))
		if err != nil {
			return nil, err
		}
		out = append(out, encoded...)
	}
	return out, nil
}

// shiftJISToString 将Shift-JIS字节数组转换为UTF-8字符串
// shiftJISToString converts a Shift-JIS byte array to a UTF-8 string
func shiftJISToString(data []byte) (string, error) {
	result, _, err := transform.Bytes(japanese.ShiftJIS.NewDecoder(), data)
	return string(result), err
}

// generateIV 生成初始化向量
// generateIV generates the initialization vector
func generateIV(ivSeed []byte) []byte {
	if len(ivSeed) != 4 {
		panic("IV seed must be 4 bytes")
	}

	seed := []uint32{
		0x075BCD15,
		0x159A55E5,
		0x1F123BB5,
		binary.LittleEndian.Uint32(ivSeed) ^ 0xBFBFBFBF,
	}

	// 线性反馈移位寄存器算法
	// Linear-feedback shift-register algorithm
	for i := int64(0); i < 4; i++ {
		n := seed[0] ^ (seed[0] << 11)
		seed[0] = seed[1]
		seed[1] = seed[2]
		seed[2] = seed[3]
		// 注：Golang 中位运算优先级与 C# 不同
		// C# 原式：seed[3] = n ^ seed[3] ^ (n ^ seed[3] >> 11) >> 8;
		// 解析为：seed[3] = n ^ seed[3] ^ ((n ^ (seed[3] >> 11)) >> 8)
		// Note: bitwise operators have different precedence in Golang and C#
		// The original C# expression is seed[3] = n ^ seed[3] ^ (n ^ seed[3] >> 11) >> 8;
		// It parses as seed[3] = n ^ seed[3] ^ ((n ^ (seed[3] >> 11)) >> 8)
		seed[3] = n ^ seed[3] ^ ((n ^ (seed[3] >> 11)) >> 8)
	}

	// 转换为字节数组
	// Convert to a byte array
	iv := make([]byte, 16)
	for i := int64(0); i < int64(len(seed)); i++ {
		binary.LittleEndian.PutUint32(iv[i*4:], seed[i])
	}

	return iv
}

// encryptData 加密数据
// encryptData encrypts data
func encryptData(data []byte, key []byte, ivSeed []byte) ([]byte, error) {
	if ivSeed == nil {
		// 以下随机 IV 种子实现按原代码保留为注释
		// The random IV seed implementation below remains commented out as in the original code
		// 生成随机 IV 种子
		// Generate a random IV seed
		// seed := rand.Uint32()
		// ivSeed = make([]byte, 4)
		// binary.LittleEndian.PutUint32(ivSeed, seed)
		// 不需要任何安全性，越容易解码越好
		// We don't need any security, the easier it is to decode the better
		ivSeed = []byte{0x09, 0x00, 0x01, 0x03}
	}

	iv := generateIV(ivSeed)

	// 创建 AES 加密器
	// Create the AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// 计算填充长度（16字节对齐）
	dataLen := int64(len(data))
	// 计算填充长度（16字节对齐）
	// Calculate the padding length for 16-byte alignment
	extraLen := int64(0)
	if dataLen%16 != 0 {
		extraLen = 16 - (dataLen % 16)
	}

	// 准备加密数据
	// Prepare the data for encryption
	plaintext := make([]byte, dataLen+extraLen)
	copy(plaintext, data)

	// 加密
	// Encrypt
	mode := cipher.NewCBCEncrypter(block, iv)
	ciphertext := make([]byte, len(plaintext))
	mode.CryptBlocks(ciphertext, plaintext)

	// 构建最终数据
	// Build the final data
	result := make([]byte, int64(len(ciphertext))+5)
	copy(result, ciphertext)
	// 写入额外长度标识
	// Write the extra-length marker
	result[int64(len(ciphertext))] = byte(extraLen) ^ ivSeed[0]
	// 写入 IV 种子
	// Write the IV seed
	copy(result[int64(len(ciphertext))+1:], ivSeed)

	return result, nil
}

// decryptData 解密数据
// decryptData decrypts data
func decryptData(encryptedData []byte, key []byte) ([]byte, error) {
	if int64(len(encryptedData)) < 5 {
		return nil, fmt.Errorf("invalid data length: %d", len(encryptedData))
	}

	// 提取控制信息
	// Extract the control data
	dataLen := int64(len(encryptedData)) - 5
	if dataLen <= 0 {
		return nil, fmt.Errorf("invalid encrypted payload length: %d", dataLen)
	}
	// CBC 模式要求输入为分组大小的整数倍，否则 CryptBlocks 将 panic
	// CBC mode requires input to be a multiple of the block size or CryptBlocks will panic
	if dataLen%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext length (%d) is not a multiple of AES block size (%d)", dataLen, aes.BlockSize)
	}
	ivSeed := encryptedData[dataLen+1 : dataLen+5]
	extraLen := int64(encryptedData[dataLen] ^ ivSeed[0])
	if len(ivSeed) != 4 {
		return nil, fmt.Errorf("invalid IV seed length: %d", len(ivSeed))
	}
	if extraLen >= aes.BlockSize {
		return nil, fmt.Errorf("invalid padding length (extraLen): %d", extraLen)
	}

	// 生成 IV
	// Generate the IV
	iv := generateIV(ivSeed)

	// 创建 AES 解密器
	// Create the AES decryptor
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// 解密
	// Decrypt
	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, dataLen)
	mode.CryptBlocks(plaintext, encryptedData[:dataLen])

	// 移除填充
	// Remove the padding
	actualLen := int64(len(plaintext)) - extraLen
	if actualLen < 0 {
		return nil, fmt.Errorf("invalid padding length: %d", extraLen)
	}

	return plaintext[:actualLen], nil
}
