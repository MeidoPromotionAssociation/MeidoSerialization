package COM3D2

import (
	"fmt"
	"io"
	"math"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio"
)

// CM3D2_MENU
// 物品菜单文件
//
// 无版本差异

// Menu 对应 .menu 文件的结构
type Menu struct {
	Signature   string    `json:"Signature"` // "CM3D2_MENU"
	Version     int32     `json:"Version"`
	SrcFileName string    `json:"SrcFileName"`
	ItemName    string    `json:"ItemName"`
	Category    string    `json:"Category"`
	InfoText    string    `json:"InfoText"`
	BodySize    int32     `json:"BodySize"`
	Commands    []Command `json:"Commands"`
}

// Command 对应 .menu 中的命令
type Command struct {
	Command string   `json:"Command"`
	Args    []string `json:"Args"`
}

// ReadMenu 从 r 中读取一个 .menu 文件结构
func ReadMenu(r io.Reader) (*Menu, error) {
	m := &Menu{}

	// 1. Signature
	sig, err := binaryio.ReadString(r)
	if err != nil {
		return nil, err
	}
	//if sig != MenuSignature {
	//	return nil, fmt.Errorf("invalid signature, got %q, want %s", sig, MenuSignature)
	//}
	m.Signature = sig

	// 2. Version (4 bytes little-endian)
	ver, err := binaryio.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("read version failed: %w", err)
	}
	m.Version = ver

	// 3. SrcFileName (string)
	src, err := binaryio.ReadString(r)
	if err != nil {
		return nil, fmt.Errorf("read srcFileName failed: %w", err)
	}
	m.SrcFileName = src

	// 4. ItemName
	itemName, err := binaryio.ReadString(r)
	if err != nil {
		return nil, fmt.Errorf("read itemName failed: %w", err)
	}
	m.ItemName = itemName

	// 5. Category
	cat, err := binaryio.ReadString(r)
	if err != nil {
		return nil, fmt.Errorf("read category failed: %w", err)
	}
	m.Category = cat

	// 6. InfoText
	infoText, err := binaryio.ReadString(r)
	if err != nil {
		return nil, fmt.Errorf("read infoText failed: %w", err)
	}
	m.InfoText = infoText

	// 7. BodySize
	bodySize, err := binaryio.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("read bodySize failed: %w", err)
	}
	m.BodySize = bodySize

	// 8. Commands, until we see a 0 byte
	for {
		peek, err := binaryio.PeekByte(r)
		if err != nil {
			return nil, fmt.Errorf("peek command argCount failed: %w", err)
		}
		if peek == 0 {
			// 说明后面是 endByte=0
			break
		}
		// read a new Command
		var cmd Command
		ac, err := binaryio.ReadByte(r)
		if err != nil {
			return nil, fmt.Errorf("read command.argCount failed: %w", err)
		}
		if ac == 0 {
			// 理论上不会出现，因为 0 在 PeekByte 时已作为终止判断，这里容错
			cmd.Command = ""
			cmd.Args = nil
		} else {
			// 第一个字符串为命令，其余为参数
			first, err := binaryio.ReadString(r)
			if err != nil {
				return nil, fmt.Errorf("read command failed: %w", err)
			}
			cmd.Command = first
			if ac > 1 {
				cmd.Args = make([]string, 0, int(ac)-1)
				for i := 1; i < int(ac); i++ {
					arg, err := binaryio.ReadString(r)
					if err != nil {
						return nil, fmt.Errorf("read command arg failed: %w", err)
					}
					cmd.Args = append(cmd.Args, arg)
				}
			}
		}
		m.Commands = append(m.Commands, cmd)
	}

	// 9. endByte = 0
	endB, err := binaryio.ReadByte(r)
	if err != nil {
		return nil, fmt.Errorf("read endByte failed: %w", err)
	}
	if endB != endByte {
		return nil, fmt.Errorf("expected endByte=0 but got %d", endB)
	}

	return m, nil
}

// Dump 把 Menu 写出到 w 中
func (m *Menu) Dump(w io.Writer) error {
	err := m.UpdateBodySize()
	if err != nil {
		return fmt.Errorf("update bodySize failed: %w", err)
	}

	// 1. Signature
	if err := binaryio.WriteString(w, m.Signature); err != nil {
		return fmt.Errorf("write signature failed: %w", err)
	}

	// 2. Version
	if err := binaryio.WriteInt32(w, m.Version); err != nil {
		return fmt.Errorf("write version failed: %w", err)
	}

	// 3. SrcFileName
	if err := binaryio.WriteString(w, m.SrcFileName); err != nil {
		return fmt.Errorf("write srcFileName failed: %w", err)
	}

	// 4. ItemName
	if err := binaryio.WriteString(w, m.ItemName); err != nil {
		return fmt.Errorf("write itemName failed: %w", err)
	}

	// 5. Category
	if err := binaryio.WriteString(w, m.Category); err != nil {
		return fmt.Errorf("write category failed: %w", err)
	}

	// 6. InfoText
	if err := binaryio.WriteString(w, m.InfoText); err != nil {
		return fmt.Errorf("write infoText failed: %w", err)
	}

	// 7. BodySize
	if err := binaryio.WriteInt32(w, m.BodySize); err != nil {
		return fmt.Errorf("write bodySize failed: %w", err)
	}

	// 8. 写入 Commands
	for _, cmd := range m.Commands {
		// 至少要有命令名；Args 可以为空
		if cmd.Command == "" {
			continue
		}

		// ArgCount = 1(命令名) + len(Args)，总数不能超过 255
		if len(cmd.Args)+1 > 255 {
			return fmt.Errorf("command %q has invalid arg count=%d, max count is 255", cmd.Command, len(cmd.Args)+1)
		}

		// 写 ArgCount
		if err := binaryio.WriteByte(w, byte(len(cmd.Args)+1)); err != nil {
			return fmt.Errorf("write command argCount failed: %w", err)
		}

		// 先写命令名
		if err := binaryio.WriteString(w, cmd.Command); err != nil {
			return fmt.Errorf("write command name failed: %w", err)
		}

		// 再写参数
		for _, arg := range cmd.Args {
			if err := binaryio.WriteString(w, arg); err != nil {
				return fmt.Errorf("write command arg failed: %w", err)
			}
		}
	}

	// 9. 写一个 0 byte 结束
	if err := binaryio.WriteByte(w, endByte); err != nil {
		return fmt.Errorf("write endByte=0 failed: %w", err)
	}
	return nil
}

// UpdateBodySize 根据当前的 Commands 列表计算 BodySize。
//   - 每个命令占用 1 字节记录 ArgCount。
//   - 对于每个字符串参数，先计算其 UTF-8 编码后字节数 encodedLength，然后
//     加上 7BitEncoded 编码 encodedLength 所需的字节数，再加上 encodedLength 本身。
//   - 最后再加上 1 个字节的结束标志。
func (m *Menu) UpdateBodySize() error {
	var sum int32 = 0

	for _, cmd := range m.Commands {
		// 1. 写入 ArgCount (1 字节)
		sum += 1

		// 2. 命令名
		{
			encodedLength := len(cmd.Command)
			if encodedLength > math.MaxInt32 {
				return fmt.Errorf("string parameter length (%d) exceeds the maximum value of int32", encodedLength)
			}
			lebSize := binaryio.Get7BitEncodedIntSize(int32(encodedLength))
			sum += int32(lebSize)
			sum += int32(encodedLength)
		}

		// 3. 遍历每个参数
		for _, arg := range cmd.Args {
			encodedLength := len(arg)
			if encodedLength > math.MaxInt32 {
				return fmt.Errorf("string parameter length (%d) exceeds the maximum value of int32", encodedLength)
			}
			lebSize := binaryio.Get7BitEncodedIntSize(int32(encodedLength))
			sum += int32(lebSize)
			sum += int32(encodedLength)
		}
	}

	// 3. 结束标志 0 字节
	sum += 1

	m.BodySize = sum

	return nil
}
