package COM3D2

import (
	"io"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/common/nei"
)

// COM3D2 .nei
// COM3D2 的 CsvParser 通过 MultiByteToWideChar(932) 解码单元格，即 Shift-JIS
// 容器结构与 KCES 完全一致，实现位于共用的 serialization/nei 包，这里只提供 COM3D2 语义的封装
// COM3D2 .nei
// COM3D2's CsvParser decodes cells through MultiByteToWideChar(932), which is Shift-JIS
// The container structure is identical to the KCES one and the implementation lives in the shared serialization/nei package, so this file only provides the COM3D2 semantics

// Nei 表示解密后的 .nei CSV 表格数据 / Nei represents decrypted .nei CSV table data
type Nei = nei.Table

// NeiTextEncoding 表示 .nei 单元格文本使用的字符编码 / NeiTextEncoding represents the character encoding used by .nei cell text
type NeiTextEncoding = nei.TextEncoding

const (
	// NeiTextEncodingShiftJIS 是 COM3D2 读取 .nei 时使用的 Shift-JIS（CP932）编码
	// NeiTextEncodingShiftJIS is the Shift-JIS (CP932) encoding COM3D2 uses when reading .nei
	NeiTextEncodingShiftJIS = nei.TextEncodingShiftJIS
	// NeiTextEncodingUTF8 是 KCES 读取 .nei 时使用的 UTF-8 编码，COM3D2 表格不应写出该编码
	// NeiTextEncodingUTF8 is the UTF-8 encoding KCES uses when reading .nei, which COM3D2 tables should not emit
	NeiTextEncodingUTF8 = nei.TextEncodingUTF8
)

// NeiKey 是默认加密密钥 / NeiKey is the default encryption key
var NeiKey = nei.Key

// ReadNei 从 r 中读取一个 .nei 文件，并解析为 Nei 结构
// 单元格编码按内容探测，COM3D2 自带的表格会得到 Shift-JIS
// neiKey 传入 nil 则使用默认密钥
// ReadNei reads a .nei file from r and parses it into a Nei structure
// The cell encoding is detected from content and COM3D2's own tables resolve to Shift-JIS
// Passing nil as neiKey selects the default key
func ReadNei(r io.Reader, neiKey []byte) (*Nei, error) {
	return nei.Read(r, neiKey)
}

// NewNei 创建使用 COM3D2 单元格编码的空表格
// NewNei creates an empty table using the COM3D2 cell encoding
func NewNei(rows uint32, cols uint32, data [][]string) *Nei {
	return &Nei{
		Rows:         rows,
		Cols:         cols,
		Data:         data,
		TextEncoding: NeiTextEncodingShiftJIS,
	}
}
