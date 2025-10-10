package COM3D2

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"fmt"
	"io"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// COM3D2 .nei
// Nei 格式是一种 AES（CBC 模式，无填充，手动补齐）+ 自定义 IV 生成与嵌入机制 加密的 Shift-JIS 编码的 CSV 文件
// 通常情况下应该是固定密钥的，除非 KISS 做了什么奇怪的事情
// 本模块实现参考自 https://github.com/usagirei/CM3D2.Toolkit 和 https://github.com/JustAGuest4168/CM3D2.Toolkit
// 感谢 @usagirei 和 @JustAGuest4168 完整的实现了加解密与转换
// Under MIT License

type Nei struct {
	Rows uint32     `json:"Rows"` // CSV 的行数
	Cols uint32     `json:"Cols"` // CSV 的列数
	Data [][]string `json:"Data"` // CSV 的数据 [行][列]
}

var (
	NeiKey = []byte{
		0xAA, 0xC9, 0xD2, 0x35,
		0x22, 0x87, 0x20, 0xF2,
		0x40, 0xC5, 0x61, 0x7C,
		0x01, 0xDF, 0x66, 0x54,
	} // 加密密钥
)

// ReadNei 从 r 中读取一个 .nei 文件，并解析为 Nei 结构。
// neiKey 传入 nil 则使用默认密钥
func ReadNei(r io.Reader, neiKey []byte) (*Nei, error) {
	if neiKey == nil {
		neiKey = NeiKey
	}

	// 读取数据
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}

	// 解密数据
	decrypted, err := decryptData(data, neiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt data: %w", err)
	}

	buf := bytes.NewReader(decrypted)

	// 验证签名
	signature, err := binaryio.ReadBytes(buf, 4)
	if err != nil {
		return nil, fmt.Errorf("failed to read signature: %w", err)
	}
	if !bytes.Equal(signature, NeiSignature) {
		return nil, fmt.Errorf("invalid NEI signature, want %v, got %v", NeiSignature, signature)
	}

	// 读取列数和行数
	cols, err := binaryio.ReadUInt32(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read col: %w", err)
	}
	rows, err := binaryio.ReadUInt32(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read row: %w", err)
	}

	totalCells := int(cols * rows)

	// 读取每个单元格的偏移量和长度
	lengths := make([]int, totalCells)
	for i := 0; i < totalCells; i++ {

		// 该单元格内容在“字符串数据区”中的起始偏移（以 0 为该数据区起点）
		// 应该是用于随机访问使用的，对我们来说没有用
		_, err := binaryio.ReadUInt32(buf)
		if err != nil {
			return nil, fmt.Errorf("failed to read offset: %w", err)
		}

		length, err := binaryio.ReadUInt32(buf)
		if err != nil {
			return nil, fmt.Errorf("failed to read length: %w", err)
		}

		lengths[i] = int(length)
	}

	// 准备单元格二维数组
	data2D := make([][]string, int(rows))
	for i := range data2D {
		data2D[i] = make([]string, int(cols))
	}

	// 读取每个单元格的数据
	currentRow, currentCol := 0, 0
	for i := 0; i < totalCells; i++ {
		length := lengths[i]

		if length > 0 {
			cellData, err := binaryio.ReadBytes(buf, length)
			if err != nil {
				return nil, fmt.Errorf("failed to read cellData: %w", err)
			}

			cellValue, err := shiftJISToString(cellData)
			if err != nil {
				return nil, fmt.Errorf("failed to decode string: %w", err)
			}

			if currentRow < len(data2D) && currentCol < len(data2D[currentRow]) {
				data2D[currentRow][currentCol] = strings.TrimRight(cellValue, "\x00")
			}
		}

		currentCol++
		if currentCol >= int(cols) {
			currentCol = 0
			currentRow++
		}
	}

	return &Nei{
		Rows: rows,
		Cols: cols,
		Data: data2D,
	}, nil
}

// Dump 将 nei 写出到 w 中，格式与 .Nei 兼容。
func (nei *Nei) Dump(w io.Writer) error {
	totalCells := int(nei.Rows * nei.Cols)
	buf := new(bytes.Buffer)

	// 编码所有字符串
	encodedValues := make([][]byte, totalCells)
	for rowIndex, row := range nei.Data {
		for colIndex := 0; colIndex < int(nei.Cols); colIndex++ {
			cellIndex := colIndex + rowIndex*int(nei.Cols)

			if colIndex < len(row) && row[colIndex] != "" {
				encoded, err := stringToShiftJIS(row[colIndex])
				if err != nil {
					return fmt.Errorf("failed to encode string: %w", err)
				}
				encodedValues[cellIndex] = encoded
			} else {
				encodedValues[cellIndex] = nil
			}
		}
	}

	//重新计算行列数
	nei.Rows = uint32(len(nei.Data))
	nei.Cols = uint32(len(nei.Data[0]))

	// 写入文件头
	err := binaryio.WriteBytes(buf, NeiSignature)
	if err != nil {
		return fmt.Errorf("failed to write signature: %w", err)
	}

	// 写入列数
	err = binaryio.WriteUInt32(buf, nei.Cols)
	if err != nil {
		return fmt.Errorf("failed to write Cols: %w", err)
	}

	// 写入行数
	err = binaryio.WriteUInt32(buf, nei.Rows)
	if err != nil {
		return fmt.Errorf("failed to write Rows: %w", err)
	}

	// 写入索引表
	totalLength := 0
	for _, encoded := range encodedValues {
		length := 0
		if encoded != nil {
			length = len(encoded) + 1
		}

		offset := 0
		if length > 0 {
			offset = totalLength
		}

		// 索引偏移
		err = binaryio.WriteUInt32(buf, uint32(offset))
		if err != nil {
			return fmt.Errorf("failed to write offset: %w", err)
		}
		// 索引长度
		err = binaryio.WriteUInt32(buf, uint32(length))
		if err != nil {
			return fmt.Errorf("failed to write length: %w", err)
		}

		totalLength += length
	}

	// 写入字符串数据
	for _, encoded := range encodedValues {
		if encoded != nil {
			err = binaryio.WriteBytes(buf, encoded)
			if err != nil {
				return fmt.Errorf("failed to write string: %w", err)
			}
			// 每个非空单元都追加一个 null 终止符
			err = binaryio.WriteByte(buf, 0x00)
			if err != nil {
				return fmt.Errorf("failed to write null terminator: %w", err)
			}
		}
	}

	// 加密数据
	encryptedData, err := encryptData(buf.Bytes(), NeiKey, nil)
	if err != nil {
		return fmt.Errorf("failed to encrypt data: %w", err)
	}

	// 写入加密数据
	err = binaryio.WriteBytes(w, encryptedData)
	if err != nil {
		return fmt.Errorf("failed to write encrypted data: %w", err)
	}

	return nil
}

// stringToShiftJIS 将UTF-8字符串转换为Shift-JIS字节数组
func stringToShiftJIS(s string) ([]byte, error) {
	result, _, err := transform.Bytes(japanese.ShiftJIS.NewEncoder(), []byte(s))
	return result, err
}

// shiftJISToString 将Shift-JIS字节数组转换为UTF-8字符串
func shiftJISToString(data []byte) (string, error) {
	result, _, err := transform.Bytes(japanese.ShiftJIS.NewDecoder(), data)
	return string(result), err
}

// generateIV 生成初始化向量
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
	for i := 0; i < 4; i++ {
		n := seed[0] ^ (seed[0] << 11)
		seed[0] = seed[1]
		seed[1] = seed[2]
		seed[2] = seed[3]
		// 注：Golang 中位运算优先级与 C# 不同
		// C# 原式：seed[3] = n ^ seed[3] ^ (n ^ seed[3] >> 11) >> 8;
		// 解析为：seed[3] = n ^ seed[3] ^ ((n ^ (seed[3] >> 11)) >> 8)
		seed[3] = n ^ seed[3] ^ ((n ^ (seed[3] >> 11)) >> 8)
	}

	// 转换为字节数组
	iv := make([]byte, 16)
	for i, s := range seed {
		binary.LittleEndian.PutUint32(iv[i*4:], s)
	}

	return iv
}

// encryptData 加密数据
func encryptData(data []byte, key []byte, ivSeed []byte) ([]byte, error) {
	if ivSeed == nil {
		//// 生成随机IV种子
		//seed := rand.Uint32()
		//ivSeed = make([]byte, 4)
		//binary.LittleEndian.PutUint32(ivSeed, seed)
		ivSeed = []byte{0x09, 0x00, 0x01, 0x03} // We don't need any security, the easier it is to decode the better
	}

	iv := generateIV(ivSeed)

	// 创建AES加密器
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// 计算填充长度（16字节对齐）
	extraLen := 0
	if len(data)%16 != 0 {
		extraLen = 16 - (len(data) % 16)
	}

	// 准备加密数据
	plaintext := make([]byte, len(data)+extraLen)
	copy(plaintext, data)

	// 加密
	mode := cipher.NewCBCEncrypter(block, iv)
	ciphertext := make([]byte, len(plaintext))
	mode.CryptBlocks(ciphertext, plaintext)

	// 构建最终数据
	result := make([]byte, len(ciphertext)+5)
	copy(result, ciphertext)
	result[len(ciphertext)] = byte(extraLen) ^ ivSeed[0] // 额外长度标识
	copy(result[len(ciphertext)+1:], ivSeed)             // IV种子

	return result, nil
}

// decryptData 解密数据
func decryptData(encryptedData []byte, key []byte) ([]byte, error) {
	if len(encryptedData) < 5 {
		return nil, fmt.Errorf("invalid data length: %d", len(encryptedData))
	}

	// 提取控制信息
	dataLen := len(encryptedData) - 5
	if dataLen <= 0 {
		return nil, fmt.Errorf("invalid encrypted payload length: %d", dataLen)
	}
	// CBC 模式要求输入为分组大小的整数倍，否则 CryptBlocks 将 panic
	if dataLen%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext length (%d) is not a multiple of AES block size (%d)", dataLen, aes.BlockSize)
	}
	ivSeed := encryptedData[dataLen+1 : dataLen+5]
	extraLen := int(encryptedData[dataLen] ^ ivSeed[0])
	if len(ivSeed) != 4 {
		return nil, fmt.Errorf("invalid IV seed length: %d", len(ivSeed))
	}
	if extraLen < 0 || extraLen >= aes.BlockSize {
		return nil, fmt.Errorf("invalid padding length (extraLen): %d", extraLen)
	}

	// 生成IV
	iv := generateIV(ivSeed)

	// 创建AES解密器
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// 解密
	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, dataLen)
	mode.CryptBlocks(plaintext, encryptedData[:dataLen])

	// 移除填充
	actualLen := len(plaintext) - extraLen
	if actualLen < 0 {
		return nil, fmt.Errorf("invalid padding length: %d", extraLen)
	}

	return plaintext[:actualLen], nil
}
