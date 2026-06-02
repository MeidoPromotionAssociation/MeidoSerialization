package KCES

import (
	"bytes"
	"fmt"
	"io"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
)

const HitCheckSignature = "HitCheck"

// HitCheck 表示 KCES hitcheck 二进制文件 / HitCheck represents a KCES hitcheck binary file
type HitCheck struct {
	Signature string          `json:"signature"` // 文件签名，通常为 HitCheck / File signature, usually HitCheck
	Header    int32           `json:"header"`    // 条目计数之后的头部整数 / Header integer stored after the entry count
	Entries   []HitCheckEntry `json:"entries"`   // hitcheck 条目列表 / Hitcheck entry list
}

// HitCheckEntry 表示一个 hitcheck 球形检测条目 / HitCheckEntry represents one spherical hitcheck entry
type HitCheckEntry struct {
	Radius     float32 `json:"radius"`         // 半径 / Radius
	RadiusSqr  float32 `json:"radiusSqr"`      // 半径平方 / Squared radius
	ShapeName  string  `json:"shapeName"`      // 形状名称 / Shape name
	BoneName   string  `json:"boneName"`       // 绑定骨骼名称 / Bound bone name
	Position   Vector3 `json:"position"`       // 相对位置 / Relative position
	TargetType int32   `json:"targetType"`     // 目标类型枚举 / Target type enum
	Side       int32   `json:"side"`           // 左右侧枚举 / Side enum
	Tail       *int32  `json:"tail,omitempty"` // 可选尾部整数，用于兼容带额外字段的样本 / Optional tail integer for samples with an extra field
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
		return nil, fmt.Errorf("invalid hitcheck entry count %d", count)
	}

	header, err := br.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read hitcheck header: %w", err)
	}

	out := &HitCheck{
		Signature: signature,
		Header:    header,
		Entries:   make([]HitCheckEntry, 0, count),
	}
	for i := 0; i < int(count); i++ {
		entry, err := readHitCheckEntry(br, reader, i)
		if err != nil {
			return nil, err
		}
		out.Entries = append(out.Entries, entry)
	}

	pos, err := reader.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, fmt.Errorf("inspect hitcheck tail: %w", err)
	}
	if pos != int64(len(data)) {
		return nil, fmt.Errorf("hitcheck has %d unread bytes", len(data)-int(pos))
	}

	return out, nil
}

func EncodeHitCheck(value *HitCheck) ([]byte, error) {
	if value == nil {
		return nil, fmt.Errorf("nil hitcheck")
	}

	var buf bytes.Buffer
	bw := stream.NewBinaryWriter(&buf)

	signature := value.Signature
	if signature == "" {
		signature = HitCheckSignature
	}
	if err := bw.WriteString(signature); err != nil {
		return nil, fmt.Errorf("write hitcheck signature: %w", err)
	}
	if err := bw.WriteInt32(int32(len(value.Entries))); err != nil {
		return nil, fmt.Errorf("write hitcheck entry count: %w", err)
	}
	if err := bw.WriteInt32(value.Header); err != nil {
		return nil, fmt.Errorf("write hitcheck header: %w", err)
	}

	for i := range value.Entries {
		if err := writeHitCheckEntry(bw, &value.Entries[i], i); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

func readHitCheckEntry(br *stream.BinaryReader, reader *bytes.Reader, index int) (HitCheckEntry, error) {
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
		return HitCheckEntry{}, fmt.Errorf("read hitcheck[%d].shapeName: %w", index, err)
	}
	boneName, err := br.ReadString()
	if err != nil {
		return HitCheckEntry{}, fmt.Errorf("read hitcheck[%d].boneName: %w", index, err)
	}
	pos, err := br.ReadFloat3()
	if err != nil {
		return HitCheckEntry{}, fmt.Errorf("read hitcheck[%d].position: %w", index, err)
	}
	targetType, err := br.ReadInt32()
	if err != nil {
		return HitCheckEntry{}, fmt.Errorf("read hitcheck[%d].targetType: %w", index, err)
	}
	side, err := br.ReadInt32()
	if err != nil {
		return HitCheckEntry{}, fmt.Errorf("read hitcheck[%d].side: %w", index, err)
	}

	entry := HitCheckEntry{
		Radius:     radius,
		RadiusSqr:  radiusSqr,
		ShapeName:  shapeName,
		BoneName:   boneName,
		Position:   Vector3{X: pos[0], Y: pos[1], Z: pos[2]},
		TargetType: targetType,
		Side:       side,
	}

	if remaining := reader.Len(); remaining >= 4 {
		tail, err := br.ReadInt32()
		if err != nil {
			return HitCheckEntry{}, fmt.Errorf("read hitcheck[%d].tail: %w", index, err)
		}
		entry.Tail = &tail
	} else if remaining != 0 {
		return HitCheckEntry{}, fmt.Errorf("hitcheck[%d] has truncated tail: %d bytes", index, remaining)
	}

	return entry, nil
}

func writeHitCheckEntry(bw *stream.BinaryWriter, entry *HitCheckEntry, index int) error {
	if err := bw.WriteFloat32(entry.Radius); err != nil {
		return fmt.Errorf("write hitcheck[%d].radius: %w", index, err)
	}
	if err := bw.WriteFloat32(entry.RadiusSqr); err != nil {
		return fmt.Errorf("write hitcheck[%d].radiusSqr: %w", index, err)
	}
	if err := bw.WriteString(entry.ShapeName); err != nil {
		return fmt.Errorf("write hitcheck[%d].shapeName: %w", index, err)
	}
	if err := bw.WriteString(entry.BoneName); err != nil {
		return fmt.Errorf("write hitcheck[%d].boneName: %w", index, err)
	}
	if err := bw.WriteFloat3([3]float32{entry.Position.X, entry.Position.Y, entry.Position.Z}); err != nil {
		return fmt.Errorf("write hitcheck[%d].position: %w", index, err)
	}
	if err := bw.WriteInt32(entry.TargetType); err != nil {
		return fmt.Errorf("write hitcheck[%d].targetType: %w", index, err)
	}
	if err := bw.WriteInt32(entry.Side); err != nil {
		return fmt.Errorf("write hitcheck[%d].side: %w", index, err)
	}
	if entry.Tail != nil {
		if err := bw.WriteInt32(*entry.Tail); err != nil {
			return fmt.Errorf("write hitcheck[%d].tail: %w", index, err)
		}
	}
	return nil
}
