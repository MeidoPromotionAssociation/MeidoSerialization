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
	Entries   []HitCheckEntry `json:"entries"`   // hitcheck 条目列表 / Hitcheck entry list
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
		return nil, fmt.Errorf("invalid hitcheck entry count %d", count)
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

	for i := range value.Entries {
		if err := writeHitCheckEntry(bw, &value.Entries[i], i); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
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
