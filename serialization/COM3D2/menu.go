package COM3D2

import (
	"fmt"
	"io"
	"math"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
)

// CM3D2_MENU
// 物品菜单文件
//
// 无版本差异
// ModCompile 依次写入小写源文件路径、物品名称、类别和说明文本
// COM3D2.5 的 ScriptManager 会使用源文件路径判断删除菜单，并使用物品名称执行 HasItemNameFromMaidMPN 匹配
// MenuHeader 和原生 MenuDataBase 还会读取或暴露类别与说明文本，但在所提供的托管源码中未找到这两个字段的直接消费点（没有用）
// CM3D2_MENU
// Item menu file
//
// There are no version differences
// ModCompile writes the lowercase source path, item name, category, and description text in that order
// COM3D2.5 ScriptManager uses the source path to identify deletion menus and the item name for HasItemNameFromMaidMPN matching
// MenuHeader and the native MenuDataBase also read or expose the category and description, but no direct consumers of those two fields were found in the provided managed source (useless)

// Menu 对应 .menu 文件的结构
// Menu corresponds to the structure of a .menu file
type Menu struct {
	Signature   string    `json:"Signature"`   // CM3D2_MENU 文件签名 / CM3D2_MENU file signature
	Version     int32     `json:"Version"`     // 菜单格式版本 / Menu format version
	SrcFileName string    `json:"SrcFileName"` // 编译菜单时记录的小写源文件路径 / Lowercase source path recorded when compiling the menu
	ItemName    string    `json:"ItemName"`    // 编译菜单时记录的物品名称 / Item name recorded when compiling the menu
	Category    string    `json:"Category"`    // 编译菜单时记录的类别名称 / Category name recorded when compiling the menu
	InfoText    string    `json:"InfoText"`    // 编译菜单时记录的说明文本 / Description text recorded when compiling the menu
	BodySize    int32     `json:"BodySize"`    // 命令块含终止字节的总字节数 / Total command-block size including the terminator byte
	Commands    []Command `json:"Commands"`    // 菜单命令 / Menu commands
}

// Command 对应 .menu 中的命令
// Command corresponds to a command in a .menu file
type Command struct {
	Command string   `json:"Command"` // 命令名称，即记录中的第一个字符串 / Command name, which is the first string in the record
	Args    []string `json:"Args"`    // 命令名称之后的参数字符串 / Argument strings following the command name
}

// ReadMenu 从 r 中读取一个 .menu 文件结构
// ReadMenu reads a .menu file structure from r
func ReadMenu(r io.Reader) (*Menu, error) {
	rp, ok := r.(stream.Peeker)
	if !ok {
		return nil, fmt.Errorf("ReadMenu: the reader is not peekable, wrap it with bufio.Reader first")
	}

	m := &Menu{}

	reader := stream.NewBinaryReader(rp)

	// 1. 签名
	// 1. Signature
	sig, err := reader.ReadString()
	if err != nil {
		return nil, err
	}
	// if sig != MenuSignature {
	// 	return nil, fmt.Errorf("invalid signature, got %q, want %s", sig, MenuSignature)
	// }
	m.Signature = sig

	// 2. 版本（4 字节小端序）
	// 2. Version (4 bytes little-endian)
	ver, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read version failed: %w", err)
	}
	m.Version = ver

	// 3. 源文件名（字符串）
	// 3. SrcFileName (string)
	src, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read srcFileName failed: %w", err)
	}
	m.SrcFileName = src

	// 4. 物品名称
	// 4. ItemName
	itemName, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read itemName failed: %w", err)
	}
	m.ItemName = itemName

	// 5. 类别
	// 5. Category
	cat, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read category failed: %w", err)
	}
	m.Category = cat

	// 6. 说明文本
	// 6. InfoText
	infoText, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read infoText failed: %w", err)
	}
	m.InfoText = infoText

	// 7. 命令块大小
	// 7. BodySize
	bodySize, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read bodySize failed: %w", err)
	}
	if err := validateNonNegativeCount("menu bodySize", bodySize); err != nil {
		return nil, err
	}
	m.BodySize = bodySize

	// 8. 读取命令，直到遇到 0 字节
	// 8. Read commands until a 0 byte is encountered
	for {
		peek, err := reader.PeekByte()
		if err != nil {
			return nil, fmt.Errorf("peek command argCount failed: %w", err)
		}
		if peek == 0 {
			// 说明后面是 endByte=0
			// The following byte is endByte=0
			break
		}
		// 读取一个新 Command
		// Read a new Command
		var cmd Command
		ac, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("read command.argCount failed: %w", err)
		}
		if ac == 0 {
			// 理论上不会出现，因为 0 在 PeekByte 时已作为终止判断，这里容错
			// This should not occur because PeekByte already treats 0 as the terminator, but retain it for tolerance
			cmd.Command = ""
			cmd.Args = nil
		} else {
			// 第一个字符串为命令，其余为参数
			// The first string is the command and the rest are arguments
			first, err := reader.ReadString()
			if err != nil {
				return nil, fmt.Errorf("read command failed: %w", err)
			}
			if first == "" {
				return nil, fmt.Errorf("menu command name is empty")
			}
			cmd.Command = first
			if ac > 1 {
				cmd.Args = make([]string, 0, ac-1)
				for i := uint8(1); i < ac; i++ {
					arg, err := reader.ReadString()
					if err != nil {
						return nil, fmt.Errorf("read command arg failed: %w", err)
					}
					cmd.Args = append(cmd.Args, arg)
				}
			}
		}
		m.Commands = append(m.Commands, cmd)
	}

	// 9. 结束字节 = 0
	// 9. endByte = 0
	endB, err := reader.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("read endByte failed: %w", err)
	}
	if endB != endByte {
		return nil, fmt.Errorf("expected endByte=0 but got %d", endB)
	}

	return m, nil
}

// Dump 把 Menu 写出到 w 中
// Dump writes Menu to w
func (m *Menu) Dump(w io.Writer) error {
	if m == nil {
		return fmt.Errorf("nil menu")
	}
	if err := m.UpdateBodySize(); err != nil {
		return fmt.Errorf("update bodySize failed: %w", err)
	}
	writer := stream.NewBinaryWriter(w)

	// 1. 签名
	// 1. Signature
	if err := writer.WriteString(m.Signature); err != nil {
		return fmt.Errorf("write signature failed: %w", err)
	}

	// 2. 版本
	// 2. Version
	if err := writer.WriteInt32(m.Version); err != nil {
		return fmt.Errorf("write version failed: %w", err)
	}

	// 3. 源文件名
	// 3. SrcFileName
	if err := writer.WriteString(m.SrcFileName); err != nil {
		return fmt.Errorf("write srcFileName failed: %w", err)
	}

	// 4. 物品名称
	// 4. ItemName
	if err := writer.WriteString(m.ItemName); err != nil {
		return fmt.Errorf("write itemName failed: %w", err)
	}

	// 5. 类别
	// 5. Category
	if err := writer.WriteString(m.Category); err != nil {
		return fmt.Errorf("write category failed: %w", err)
	}

	// 6. 说明文本
	// 6. InfoText
	if err := writer.WriteString(m.InfoText); err != nil {
		return fmt.Errorf("write infoText failed: %w", err)
	}

	// 7. 命令块大小
	// 7. BodySize
	if err := writer.WriteInt32(m.BodySize); err != nil {
		return fmt.Errorf("write bodySize failed: %w", err)
	}

	// 8. 写入 Commands
	// 8. Write Commands
	for _, cmd := range m.Commands {
		// ArgCount = 1(命令名) + len(Args)，总数不能超过 255
		// 写 ArgCount
		// ArgCount = 1 (command name) + len(Args), and the total cannot exceed 255
		// Write ArgCount
		if err := writer.WriteByte(byte(len(cmd.Args) + 1)); err != nil {
			return fmt.Errorf("write command argCount failed: %w", err)
		}

		// 先写命令名
		// Write the command name first
		if err := writer.WriteString(cmd.Command); err != nil {
			return fmt.Errorf("write command name failed: %w", err)
		}

		// 再写参数
		// Then write the arguments
		for _, arg := range cmd.Args {
			if err := writer.WriteString(arg); err != nil {
				return fmt.Errorf("write command arg failed: %w", err)
			}
		}
	}

	// 9. 写一个 0 byte 结束
	// 9. Write one 0 byte to finish
	if err := writer.WriteByte(endByte); err != nil {
		return fmt.Errorf("write endByte=0 failed: %w", err)
	}
	return nil
}

// UpdateBodySize 根据当前的 Commands 列表计算 BodySize
//   - 每个命令占用 1 字节记录 ArgCount
//   - 对于每个字符串参数，先计算其 UTF-8 编码后字节数 encodedLength，然后加上 7BitEncoded 编码 encodedLength 所需的字节数，再加上 encodedLength 本身
//   - 最后再加上 1 个字节的结束标志
//
// UpdateBodySize calculates BodySize from the current Commands list
//   - Each command uses 1 byte for ArgCount
//   - For each string, add its UTF-8 encodedLength, the bytes required to encode encodedLength as 7BitEncoded, and the string bytes themselves
//   - Finally add the 1-byte terminator
func (m *Menu) UpdateBodySize() error {
	sum, err := m.CalculateBodySize()
	if err != nil {
		return err
	}
	m.BodySize = sum
	return nil
}

// CalculateBodySize 返回 Dump 和官方编译器写入的命令块大小
// CalculateBodySize returns the command-block size written by Dump and the official compiler
func (m *Menu) CalculateBodySize() (int32, error) {
	if m == nil {
		return 0, fmt.Errorf("nil menu")
	}
	var sum int64

	for _, cmd := range m.Commands {
		if cmd.Command == "" {
			return 0, fmt.Errorf("menu command name is empty")
		}
		argCount := int64(len(cmd.Args)) + 1
		if argCount > 255 {
			return 0, fmt.Errorf("command %q has invalid arg count=%d, max count is 255", cmd.Command, argCount)
		}
		// 1. 写入 ArgCount (1 字节)
		// 1. Write ArgCount (1 byte)
		sum += 1

		// 2. 命令名
		// 2. Command name
		{
			encodedLength := int64(len(cmd.Command))
			if encodedLength > math.MaxInt32 {
				return 0, fmt.Errorf("string parameter length (%d) exceeds the maximum value of int32", encodedLength)
			}
			lebSize := stream.Get7BitEncodedIntSize(int32(encodedLength))
			sum += lebSize + encodedLength
		}

		// 3. 遍历每个参数
		// 3. Traverse each argument
		for _, arg := range cmd.Args {
			encodedLength := int64(len(arg))
			if encodedLength > math.MaxInt32 {
				return 0, fmt.Errorf("string parameter length (%d) exceeds the maximum value of int32", encodedLength)
			}
			lebSize := stream.Get7BitEncodedIntSize(int32(encodedLength))
			sum += lebSize + encodedLength
		}
		if sum > math.MaxInt32 {
			return 0, fmt.Errorf("menu command body size %d exceeds the maximum value of int32", sum)
		}
	}

	// 3. 结束标志 0 字节
	// 3. Ending marker 0 byte
	sum += 1
	if sum > math.MaxInt32 {
		return 0, fmt.Errorf("menu command body size %d exceeds the maximum value of int32", sum)
	}
	return int32(sum), nil
}
