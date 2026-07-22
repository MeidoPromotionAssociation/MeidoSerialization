package KCES

import (
	"bytes"
	"fmt"
	"math"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
)

// .hitcheck (HitCheck)
// KCES/COM3D2 身体与头发使用的球形碰撞检测列表。文件以 BinaryWriter 字符串签名和 Int32 条目数开头，
// 随后逐项保存响应类型、半径、对象与父骨骼名称、局部位置和用途标记；该格式没有版本字段。
//
// .hitcheck (HitCheck)
// Spherical collision-check list used by KCES/COM3D2 bodies and hair. A BinaryWriter string signature and Int32 entry count
// are followed by response type, radius, object and parent-bone names, local position, and usage flags; the format has no version field.

const HitCheckSignature = "HitCheck"

// HitCheck 表示 KCES hitcheck 二进制文件 / HitCheck represents a KCES hitcheck binary file
type HitCheck struct {
	Signature    string          `json:"signature"`              // 文件签名，通常为 HitCheck / File signature, usually HitCheck
	Entries      []HitCheckEntry `json:"entries"`                // hitcheck 条目列表 / Hitcheck entry list
	TrailingData []byte          `json:"trailingData,omitempty"` // Bytes ignored by the game after count entries / 游戏读取 count 项后忽略的字节
}

// HitCheckEntry 表示一个 hitcheck 球形检测条目 / HitCheckEntry represents one spherical hitcheck entry
type HitCheckEntry struct {
	Type      int32   `json:"type"`      // 碰撞响应类型：0=通常球，1=头部等特殊球 / Collision response type: 0=normal sphere, 1=head/special sphere
	Radius    float32 `json:"radius"`    // 半径，对应游戏 THitSphere.len / Radius, matching game THitSphere.len
	RadiusSqr float32 `json:"radiusSqr"` // 半径平方，对应游戏 THitSphere.lenxlen / Squared radius, matching game THitSphere.lenxlen
	Name      string  `json:"name"`      // 碰撞球对象名，对应游戏 THitSphere.name / Hit sphere object name, matching game THitSphere.name
	Parent    string  `json:"parent"`    // 父骨骼名，对应游戏 THitSphere.pname / Parent bone name, matching game THitSphere.pname
	Position  Vector3 `json:"position"`  // 本地位置，对应游戏 THitSphere.vs / Local position, matching game THitSphere.vs
	SKRT      int32   `json:"skrt"`      // 裙/胸等用途标记，对应游戏 THitSphere.SKRT / Skirt/bust usage marker, matching game THitSphere.SKRT
	RL        int32   `json:"rl"`        // 左右标记，对应游戏 THitSphere.RL / Left/right marker, matching game THitSphere.RL
}

func DecodeHitCheck(data []byte) (*HitCheck, error) {
	reader := bytes.NewReader(data)
	br := stream.NewBinaryReader(reader)

	signature, err := br.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read hitcheck signature: %w", err)
	}
	if signature != HitCheckSignature {
		return nil, fmt.Errorf("invalid hitcheck signature %q", signature)
	}

	count, err := br.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read hitcheck entry count: %w", err)
	}
	if count < 0 {
		return nil, fmt.Errorf("negative hitcheck entry count %d", count)
	}
	// Even with two empty strings, one entry needs 34 bytes. Bound the
	// attacker-controlled count before allocating the output slice.
	const minimumHitCheckEntrySize = 34
	if int64(count) > int64(reader.Len()/minimumHitCheckEntrySize) {
		return nil, fmt.Errorf("hitcheck entry count %d exceeds remaining data capacity %d", count, reader.Len()/minimumHitCheckEntrySize)
	}

	out := &HitCheck{
		Signature: signature,
		Entries:   make([]HitCheckEntry, 0, count),
	}
	for i := 0; i < int(count); i++ {
		entry, err := readHitCheckEntry(br, i)
		if err != nil {
			return nil, err
		}
		out.Entries = append(out.Entries, entry)
	}

	if reader.Len() != 0 {
		out.TrailingData = make([]byte, reader.Len())
		if _, err := reader.Read(out.TrailingData); err != nil {
			return nil, fmt.Errorf("read hitcheck trailing data: %w", err)
		}
	}

	return out, nil
}

func EncodeHitCheck(value *HitCheck) ([]byte, error) {
	if value == nil {
		return nil, fmt.Errorf("nil hitcheck")
	}
	if uint64(len(value.Entries)) > uint64(math.MaxInt32) {
		return nil, fmt.Errorf("hitcheck entry count %d exceeds int32 capacity", len(value.Entries))
	}
	count := int32(len(value.Entries))

	var buf bytes.Buffer
	bw := stream.NewBinaryWriter(&buf)

	signature := value.Signature
	if signature != HitCheckSignature {
		return nil, fmt.Errorf("invalid hitcheck signature %q", signature)
	}
	if err := bw.WriteString(signature); err != nil {
		return nil, fmt.Errorf("write hitcheck signature: %w", err)
	}
	if err := bw.WriteInt32(count); err != nil {
		return nil, fmt.Errorf("write hitcheck entry count: %w", err)
	}

	for i := range value.Entries {
		if err := writeHitCheckEntry(bw, &value.Entries[i], i); err != nil {
			return nil, err
		}
	}
	if len(value.TrailingData) != 0 {
		if _, err := buf.Write(value.TrailingData); err != nil {
			return nil, fmt.Errorf("write hitcheck trailing data: %w", err)
		}
	}

	return buf.Bytes(), nil
}

// NewHitCheck explicitly creates the current recognized file header. Existing
// decoded values are never defaulted or upgraded during encoding.
func NewHitCheck() *HitCheck {
	return &HitCheck{Signature: HitCheckSignature}
}

func readHitCheckEntry(br *stream.BinaryReader, index int) (HitCheckEntry, error) {
	typ, err := br.ReadInt32()
	if err != nil {
		return HitCheckEntry{}, fmt.Errorf("read hitcheck[%d].type: %w", index, err)
	}
	radius, err := br.ReadFloat32()
	if err != nil {
		return HitCheckEntry{}, fmt.Errorf("read hitcheck[%d].radius: %w", index, err)
	}
	radiusSqr, err := br.ReadFloat32()
	if err != nil {
		return HitCheckEntry{}, fmt.Errorf("read hitcheck[%d].radiusSqr: %w", index, err)
	}
	shapeName, err := br.ReadString()
	if err != nil {
		return HitCheckEntry{}, fmt.Errorf("read hitcheck[%d].name: %w", index, err)
	}
	parent, err := br.ReadString()
	if err != nil {
		return HitCheckEntry{}, fmt.Errorf("read hitcheck[%d].parent: %w", index, err)
	}
	pos, err := br.ReadFloat3()
	if err != nil {
		return HitCheckEntry{}, fmt.Errorf("read hitcheck[%d].position: %w", index, err)
	}
	skrt, err := br.ReadInt32()
	if err != nil {
		return HitCheckEntry{}, fmt.Errorf("read hitcheck[%d].skrt: %w", index, err)
	}
	rl, err := br.ReadInt32()
	if err != nil {
		return HitCheckEntry{}, fmt.Errorf("read hitcheck[%d].rl: %w", index, err)
	}

	return HitCheckEntry{
		Type:      typ,
		Radius:    radius,
		RadiusSqr: radiusSqr,
		Name:      shapeName,
		Parent:    parent,
		Position:  Vector3{X: pos[0], Y: pos[1], Z: pos[2]},
		SKRT:      skrt,
		RL:        rl,
	}, nil
}

func writeHitCheckEntry(bw *stream.BinaryWriter, entry *HitCheckEntry, index int) error {
	if err := bw.WriteInt32(entry.Type); err != nil {
		return fmt.Errorf("write hitcheck[%d].type: %w", index, err)
	}
	if err := bw.WriteFloat32(entry.Radius); err != nil {
		return fmt.Errorf("write hitcheck[%d].radius: %w", index, err)
	}
	if err := bw.WriteFloat32(entry.RadiusSqr); err != nil {
		return fmt.Errorf("write hitcheck[%d].radiusSqr: %w", index, err)
	}
	if err := bw.WriteString(entry.Name); err != nil {
		return fmt.Errorf("write hitcheck[%d].name: %w", index, err)
	}
	if err := bw.WriteString(entry.Parent); err != nil {
		return fmt.Errorf("write hitcheck[%d].parent: %w", index, err)
	}
	if err := bw.WriteFloat3([3]float32{entry.Position.X, entry.Position.Y, entry.Position.Z}); err != nil {
		return fmt.Errorf("write hitcheck[%d].position: %w", index, err)
	}
	if err := bw.WriteInt32(entry.SKRT); err != nil {
		return fmt.Errorf("write hitcheck[%d].skrt: %w", index, err)
	}
	if err := bw.WriteInt32(entry.RL); err != nil {
		return fmt.Errorf("write hitcheck[%d].rl: %w", index, err)
	}
	return nil
}
