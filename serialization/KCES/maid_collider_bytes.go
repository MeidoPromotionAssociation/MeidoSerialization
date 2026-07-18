package KCES

import (
	"bytes"
	"fmt"
	"math"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
)

// maid_collider.bytes / maid_collider_touch.bytes
// KCES 系统资源中的女仆胶囊碰撞体列表；Unity 导入后对应同名 TextAsset。
// 载荷是无签名的 BinaryWriter 数据：Int32 数量，随后为骨骼路径与六个定长数值字段。
//
// maid_collider.bytes / maid_collider_touch.bytes
// KCES system-resource lists of maid capsule colliders, imported by Unity as same-named TextAssets.
// The payload is signatureless BinaryWriter data: an Int32 count followed by a bone path and six fixed-width numeric fields per entry.

const MaidColliderFormat = "kces-maid-capsule-colliders"

const (
	maidColliderFixedBytes = 6 * 4 // center xyz, direction, height, radius
)

// MaidColliderFile represents the custom BinaryReader payload consumed by
// MaidColliderCollect. In the System AssetBundle these TextAssets are named
// maid_collider and maid_collider_touch (the source resource names add .bytes).
type MaidColliderFile struct {
	Format       string                `json:"format"`
	Colliders    []MaidCapsuleCollider `json:"colliders"`
	TrailingData []byte                `json:"trailingData,omitempty"`
}

type MaidCapsuleCollider struct {
	BonePath  string  `json:"bonePath"`
	Center    Vector3 `json:"center"`
	Direction int32   `json:"direction"`
	Height    float32 `json:"height"`
	Radius    float32 `json:"radius"`
}

func DecodeMaidCollider(data []byte) (*MaidColliderFile, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("maid collider payload is too short: %d", len(data))
	}
	r := bytes.NewReader(data)
	reader := stream.NewBinaryReader(r)
	count, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read maid collider count: %w", err)
	}
	if count < 0 {
		return nil, fmt.Errorf("negative maid collider count %d", count)
	}
	// Every entry needs at least one 7-bit string-length byte plus its six
	// fixed-width values. Check before allocating from the attacker-controlled
	// count.
	if int64(count) > int64(r.Len())/(1+maidColliderFixedBytes) {
		return nil, fmt.Errorf("maid collider count %d cannot fit in %d remaining bytes", count, r.Len())
	}

	result := &MaidColliderFile{
		Format:    MaidColliderFormat,
		Colliders: makeKCESCountedSliceForAppend[MaidCapsuleCollider](uint64(count)),
	}
	for i := int32(0); i < count; i++ {
		bonePath, err := reader.ReadString()
		if err != nil {
			return nil, fmt.Errorf("read maid collider[%d].bonePath: %w", i, err)
		}
		entry := MaidCapsuleCollider{BonePath: bonePath}
		if entry.Center.X, err = reader.ReadFloat32(); err != nil {
			return nil, fmt.Errorf("read maid collider[%d].center.x: %w", i, err)
		}
		if entry.Center.Y, err = reader.ReadFloat32(); err != nil {
			return nil, fmt.Errorf("read maid collider[%d].center.y: %w", i, err)
		}
		if entry.Center.Z, err = reader.ReadFloat32(); err != nil {
			return nil, fmt.Errorf("read maid collider[%d].center.z: %w", i, err)
		}
		if entry.Direction, err = reader.ReadInt32(); err != nil {
			return nil, fmt.Errorf("read maid collider[%d].direction: %w", i, err)
		}
		if entry.Height, err = reader.ReadFloat32(); err != nil {
			return nil, fmt.Errorf("read maid collider[%d].height: %w", i, err)
		}
		if entry.Radius, err = reader.ReadFloat32(); err != nil {
			return nil, fmt.Errorf("read maid collider[%d].radius: %w", i, err)
		}
		result.Colliders = append(result.Colliders, entry)
	}
	if r.Len() != 0 {
		result.TrailingData = make([]byte, r.Len())
		if _, err := r.Read(result.TrailingData); err != nil {
			return nil, fmt.Errorf("read maid collider trailing data: %w", err)
		}
	}
	return result, nil
}

func EncodeMaidCollider(value *MaidColliderFile) ([]byte, error) {
	if value == nil {
		return nil, fmt.Errorf("nil maid collider payload")
	}
	if value.Format != "" && value.Format != MaidColliderFormat {
		return nil, fmt.Errorf("unsupported maid collider format %q", value.Format)
	}
	if uint64(len(value.Colliders)) > uint64(math.MaxInt32) {
		return nil, fmt.Errorf("maid collider count %d exceeds Int32", len(value.Colliders))
	}
	count := int32(len(value.Colliders))

	var out bytes.Buffer
	writer := stream.NewBinaryWriter(&out)
	if err := writer.WriteInt32(count); err != nil {
		return nil, fmt.Errorf("write maid collider count: %w", err)
	}
	for i, entry := range value.Colliders {
		if err := writer.WriteString(entry.BonePath); err != nil {
			return nil, fmt.Errorf("write maid collider[%d].bonePath: %w", i, err)
		}
		if err := writer.WriteFloat32(entry.Center.X); err != nil {
			return nil, err
		}
		if err := writer.WriteFloat32(entry.Center.Y); err != nil {
			return nil, err
		}
		if err := writer.WriteFloat32(entry.Center.Z); err != nil {
			return nil, err
		}
		if err := writer.WriteInt32(entry.Direction); err != nil {
			return nil, err
		}
		if err := writer.WriteFloat32(entry.Height); err != nil {
			return nil, err
		}
		if err := writer.WriteFloat32(entry.Radius); err != nil {
			return nil, err
		}
	}
	if len(value.TrailingData) != 0 {
		if _, err := out.Write(value.TrailingData); err != nil {
			return nil, fmt.Errorf("write maid collider trailing data: %w", err)
		}
	}
	return out.Bytes(), nil
}
