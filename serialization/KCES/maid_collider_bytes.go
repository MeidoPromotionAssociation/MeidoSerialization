package KCES

import (
	"bytes"
	"fmt"
	"math"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
)

// maid_collider.bytes 与 maid_collider_touch.bytes
// KCES 系统资源中的女仆胶囊碰撞体列表，Unity 导入后对应同名 TextAsset
// 载荷是无签名的 BinaryWriter 数据，依次保存 Int32 数量、骨骼路径与六个定长数值字段
//
// KCES system-resource lists of maid capsule colliders imported by Unity as same-named TextAssets
// The payload is signatureless BinaryWriter data storing an Int32 count followed by a bone path and six fixed-width numeric fields per entry

const MaidColliderFormat = "kces-maid-capsule-colliders"

const (
	// maidColliderFixedBytes 是中心 XYZ、方向、高度和半径的固定字节数
	// maidColliderFixedBytes is the fixed byte count for center XYZ, direction, height, and radius
	maidColliderFixedBytes = 6 * 4
)

// MaidColliderFile 表示 MaidColliderCollect 读取的自定义 BinaryReader 载荷
// 在 System AssetBundle 中，这些 TextAsset 名为 maid_collider 和 maid_collider_touch，源资源名另带 .bytes
// MaidColliderFile represents the custom BinaryReader payload consumed by MaidColliderCollect
// In the System AssetBundle these TextAssets are named maid_collider and maid_collider_touch, while source resource names additionally use .bytes
type MaidColliderFile struct {
	Format       string                `json:"format"`                 // JSON 表示格式标识 / JSON representation format identifier
	Colliders    []MaidCapsuleCollider `json:"colliders"`              // 胶囊碰撞体列表 / Capsule-collider list
	TrailingData []byte                `json:"trailingData,omitempty"` // 游戏读取声明条目后忽略的尾部字节 / Trailing bytes ignored by the game after reading the declared entries
}

// MaidCapsuleCollider 表示一个绑定到骨骼路径的 Unity 胶囊碰撞体
// MaidCapsuleCollider represents one Unity capsule collider bound to a bone path
type MaidCapsuleCollider struct {
	BonePath  string  `json:"bonePath"`  // 相对于角色根节点的骨骼路径 / Bone path relative to the character root
	Center    Vector3 `json:"center"`    // 胶囊碰撞体局部中心 / Local center of the capsule collider
	Direction int32   `json:"direction"` // Unity CapsuleCollider.direction 主轴枚举 / Unity CapsuleCollider.direction primary-axis enum
	Height    float32 `json:"height"`    // 胶囊高度 / Capsule height
	Radius    float32 `json:"radius"`    // 胶囊半径 / Capsule radius
}

// DecodeMaidCollider 解码无签名的女仆胶囊碰撞体列表并保留尾部字节
// DecodeMaidCollider decodes a signatureless maid capsule-collider list and preserves trailing bytes
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
	// 每个条目至少需要一个 7 位字符串长度字节和六个定长值，因此在按不可信数量分配前先检查容量
	// Every entry needs at least one 7-bit string-length byte plus six fixed-width values, so check capacity before allocating from the untrusted count
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

// EncodeMaidCollider 编码无签名的女仆胶囊碰撞体列表及保留尾部字节
// EncodeMaidCollider encodes a signatureless maid capsule-collider list and its preserved trailing bytes
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
