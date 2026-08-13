package KCES

import (
	"io"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/common/nei"
)

// KCES .nei
// KCES 通过 crc.dll 的 DLL_CSV_GetCellAsWideString 解码单元格，实际写入的是 UTF-8，与 COM3D2 的 Shift-JIS 不同
// 容器结构与 COM3D2 完全一致，实现位于共用的 serialization/nei 包，这里只提供 KCES 语义的封装
// KCES .nei
// KCES decodes cells through crc.dll's DLL_CSV_GetCellAsWideString and actually writes UTF-8, unlike COM3D2's Shift-JIS
// The container structure is identical to the COM3D2 one and the implementation lives in the shared serialization/nei package, so this file only provides the KCES semantics

// Nei 表示解密后的 .nei CSV 表格数据 / Nei represents decrypted .nei CSV table data
type Nei = nei.Table

// NeiTextEncoding 表示 .nei 单元格文本使用的字符编码 / NeiTextEncoding represents the character encoding used by .nei cell text
type NeiTextEncoding = nei.TextEncoding

const (
	// NeiTextEncodingUTF8 是 KCES 读取 .nei 时使用的 UTF-8 编码
	// NeiTextEncodingUTF8 is the UTF-8 encoding KCES uses when reading .nei
	NeiTextEncodingUTF8 = nei.TextEncodingUTF8
	// NeiTextEncodingShiftJIS 是 COM3D2 读取 .nei 时使用的 Shift-JIS（CP932）编码，KCES 表格不应写出该编码
	// NeiTextEncodingShiftJIS is the Shift-JIS (CP932) encoding COM3D2 uses when reading .nei, which KCES tables should not emit
	NeiTextEncodingShiftJIS = nei.TextEncodingShiftJIS
)

// NeiKey 是默认加密密钥 / NeiKey is the default encryption key
var NeiKey = nei.Key

// ReadNei 从 r 中读取一个 .nei 文件，并解析为 Nei 结构
// 单元格编码按内容探测，KCES 自带的表格会得到 UTF-8，纯 ASCII 表格无法区分编码并按 Shift-JIS 报告
// neiKey 传入 nil 则使用默认密钥
// ReadNei reads a .nei file from r and parses it into a Nei structure
// The cell encoding is detected from content and KCES's own tables resolve to UTF-8, while ASCII-only tables carry no encoding evidence and are reported as Shift-JIS
// Passing nil as neiKey selects the default key
func ReadNei(r io.Reader, neiKey []byte) (*Nei, error) {
	return nei.Read(r, neiKey)
}

// NewNei 创建使用 KCES 单元格编码的空表格
// 写出 KCES 表格时必须设置 UTF-8，否则游戏会把日文读成乱码
// NewNei creates an empty table using the KCES cell encoding
// KCES tables must be written as UTF-8 or the game reads Japanese text as garbage
func NewNei(rows uint32, cols uint32, data [][]string) *Nei {
	return &Nei{
		Rows:         rows,
		Cols:         cols,
		Data:         data,
		TextEncoding: NeiTextEncodingUTF8,
	}
}
