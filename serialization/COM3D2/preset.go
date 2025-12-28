package COM3D2

import (
	"fmt"
	"io"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio"
)

// CM3D2_PRESET
// 角色预设文件
//
//有两种 PRESET 一种 CM3D2_PRESET 一种 CM3D2_PRESET_S， CM3D2_PRESET_S 已官方废弃（不支持、不解析）
//
// - 版本范围：
//	 1 ≤ version < 1560 某些官方预设文件（支持）
//   * CM3D2 的 1560 ≤ version < 20000（支持）
//   * COM3D2 使用 20000 ≤ version < 30000（支持）
//   * COM3D2.5 的 version ≥ 30000（支持）
//
//   COM3D2 和 COM3D2.5 的 Preset 无结构差异，但 COM3D2 会拒绝读取版本大于等于 30000 的文件
//	 COM3D2.5 则无读取版本校验
//
// 子块版本差异（与实际内容功能相关）
//
// 1) CM3D2_MPROP_LIST（属性列表版本）
// - version < 4：列表项前没有写入“键名”（MPN 字符串）
// - version ≥ 4：每个属性项前新增写入“键名”（MPN 字符串）
//
// 2) CM3D2_MPROP（单个属性版本）
// - version < 101：无 temp_value
// - version ≥ 101：新增 temp_value
// - version < 200：无子属性与附加数据（SubProps/皮肤位置/附着位置/材质属性）
// - version ≥ 200：新增
//   * SubProps（子属性数组）
//   * 皮肤位置（SkinPositions）
//   * 顶点附着位置（AttachPositions）
//   * 材质属性（MaterialProps）
// - version < 204：头部材质 ZTest 字段命名旧（读取时会被迁移为 _ZTest2，并调整值）
// - version ≥ 204：材质 ZTest 字段使用新命名/_ZTest2 规则
// - version < 211：SubProp 无 TexMulAlpha
// - version ≥ 211：SubProp 新增 TexMulAlpha
// - version < 213：无骨骼长度（BoneLengths）块
// - version ≥ 213：新增 BoneLengths 块
//
// 3) CM3D2_MULTI_COL（多颜色版本）
// - version ≤ 1200：旧格式（固定顺序的若干部件，历史上有 7 或 9 项的差异）
// - version > 1200：新格式（以部件名列举，直到读到 "MAX" 终止）
//
// 4) CM3D2_MAID_BODY（身体块版本）
// - 当前仅头+版本，无后续字段；现行写入为 24405
//
// 实务建议（本项目实现）
// - 仅支持 COM3D2 新格式：文件头 20000 ≤ version < 30000，各子块版本要求：
//   * MPROP_LIST ≥ 4
//   * MPROP ≥ 213
//   * MULTI_COL > 1200
//   * MAID_BODY 任意非零版本（当前 24405）
// - 不兼容 CM3D2_PRESET_S 与 CM3D2 老版本

// 预设类型常量
const (
	PresetTypeWear = 0 // 衣服
	PresetTypeBody = 1 // 身体
	PresetTypeAll  = 2 // 全部
)

// Preset 表示角色预设数据
type Preset struct {
	Signature          string              `json:"Signature"`          // "CM3D2_PRESET"
	Version            int32               `json:"Version"`            // 版本号  大于等于 30000 的是 COM3D2.5 格式，大于等于 1560 且小于 20000 的是 CM3D2 格式，版本号介于 20000 到 30000 之间的是 COM3D2 格式
	PresetType         int32               `json:"PresetType"`         // 预设类型：0=衣服, 1=身体, 2=全部
	ThumbLength        int32               `json:"ThumbLength"`        // 略缩图数据长度
	ThumbData          []byte              `json:"ThumbData"`          // 略缩图数据，PNG格式
	PresetPropertyList *PresetPropertyList `json:"PresetPropertyList"` // 预设属性列表
	MultiColor         *MultiColor         `json:"MultiColor"`         // 颜色设置
	BodyProperty       *BodyProperty       `json:"BodyProperty"`       // 身体属性
}

// PresetMetadata 表示仅包含略缩图的的角色预设数据，不包含实际数据
type PresetMetadata struct {
	Signature   string `json:"Signature"`   // "CM3D2_PRESET"
	Version     int32  `json:"Version"`     // 版本号
	PresetType  int32  `json:"PresetType"`  // 预设类型：0=衣服, 1=身体, 2=全部
	ThumbLength int32  `json:"ThumbLength"` // 略缩图数据长度
	ThumbData   []byte `json:"ThumbData"`   // 略缩图数据，PNG格式
}

// PresetPropertyList 表示预设属性列表
type PresetPropertyList struct {
	Signature        string                    `json:"Signature"`        // "CM3D2_MPROP_LIST"
	Version          int32                     `json:"Version"`          // 版本号
	PropertyCount    int32                     `json:"PropertyCount"`    // 属性数量
	PresetProperties map[string]PresetProperty `json:"PresetProperties"` // 属性映射表
}

// PresetProperty 表示单个属性
type PresetProperty struct {
	Signature       string                               `json:"Signature"`       // "CM3D2_MPROP"
	Version         int32                                `json:"Version"`         // 版本号
	Index           int32                                `json:"Index"`           // 索引
	Name            string                               `json:"Name"`            // 名称
	Type            int32                                `json:"Type"`            // 类型
	DefaultValue    int32                                `json:"DefaultValue"`    // 默认值
	Value           int32                                `json:"Value"`           // 当前值
	TempValue       int32                                `json:"TempValue"`       // 临时值
	LinkMaxValue    int32                                `json:"LinkMaxValue"`    // 链接最大值
	FileName        string                               `json:"FileName"`        // 文件名
	FileNameRID     int32                                `json:"FileNameRID"`     // 文件名哈希值  this.strFileName.ToLower().GetHashCode();
	IsDut           bool                                 `json:"IsDut"`           // 是否使用
	Max             int32                                `json:"Max"`             // 最大值
	Min             int32                                `json:"Min"`             // 最小值
	SubProps        []SubProp                            `json:"SubProps"`        // 子属性列表
	SkinPositions   map[int]BoneAttachPosEntry           `json:"SkinPositions"`   // 皮肤位置 slotID -> (RID, BoneAttachPos)
	AttachPositions map[int]map[string]VtxAttachPosEntry `json:"AttachPositions"` // 附件位置 slotID -> name -> (RID, VtxAttachPos)
	MaterialProps   map[int]MatPropSaveEntry             `json:"MaterialProps"`   // 材质属性 slotID -> (RID, MatPropSave)
	BoneLengths     map[int]BoneLengthEntry              `json:"BoneLengths"`     // 骨骼长度 slotID -> (RID, map[name]len)
}

type BoneAttachPosEntry struct {
	RID           int32         `json:"RID"`           // C# KeyValuePair<int, BoneAttachPos>.Key
	BoneAttachPos BoneAttachPos `json:"BoneAttachPos"` // C# ...Value
}

type VtxAttachPosEntry struct {
	RID          int32        `json:"RID"`
	VtxAttachPos VtxAttachPos `json:"VtxAttachPos"`
}

type MatPropSaveEntry struct {
	RID         int32       `json:"RID"`
	MatPropSave MatPropSave `json:"MatPropSave"`
}

type BoneLengthEntry struct {
	RID     int32              `json:"RID"`
	Lengths map[string]float32 `json:"Lengths"`
}

// SubProp 表示子属性
type SubProp struct {
	IsDut       bool    `json:"IsDut"`       // 是否使用
	FileName    string  `json:"FileName"`    // 文件名
	FileNameRID int32   `json:"FileNameRID"` // 文件名哈希值
	TexMulAlpha float32 `json:"TexMulAlpha"` // 纹理乘法透明度
}

// BoneAttachPos 表示骨骼附着位置
type BoneAttachPos struct {
	Enable      bool                  `json:"Enable"`                // 是否启用
	PosRotScale PositionRotationScale `json:"PositionRotationScale"` // 位置、旋转、缩放
}

// VtxAttachPos 表示顶点附着位置
type VtxAttachPos struct {
	Enable      bool                  `json:"Enable"`                // 是否启用
	VtxCount    int32                 `json:"VtxCount"`              // 顶点数量
	VtxIdx      int32                 `json:"VtxIdx"`                // 顶点索引
	PosRotScale PositionRotationScale `json:"PositionRotationScale"` // 位置、旋转、缩放
}

// MatPropSave 表示材质属性保存
type MatPropSave struct {
	MatId    int32  `json:"MatId"`    // 材质编号
	PropName string `json:"PropName"` // 属性名称
	TypeName string `json:"TypeName"` // 类型名称
	Value    string `json:"Value"`    // 属性值
}

// MultiColor 表示多颜色设置
type MultiColor struct {
	Signature   string       `json:"Signature"`   // "CM3D2_MULTI_COL"
	Version     int32        `json:"Version"`     // 版本号
	PartCount   int32        `json:"PartCount"`   // 颜色数量
	PartsColors []PartsColor `json:"PartsColors"` // 颜色列表
}

// PartsColor 表示部件颜色
type PartsColor struct {
	IsUse            bool  `json:"IsUse"`            // 是否使用
	MainHue          int32 `json:"MainHue"`          // 主色相
	MainChroma       int32 `json:"MainChroma"`       // 主色度
	MainBrightness   int32 `json:"MainBrightness"`   // 主亮度
	MainContrast     int32 `json:"MainContrast"`     // 主对比度
	ShadowRate       int32 `json:"ShadowRate"`       // 阴影比例
	ShadowHue        int32 `json:"ShadowHue"`        // 阴影色相
	ShadowChroma     int32 `json:"ShadowChroma"`     // 阴影色度
	ShadowBrightness int32 `json:"ShadowBrightness"` // 阴影亮度
	ShadowContrast   int32 `json:"ShadowContrast"`   // 阴影对比度
}

// BodyProperty 表示身体属性
type BodyProperty struct {
	Signature string `json:"Signature"` // "CM3D2_MAID_BODY"
	Version   int32  `json:"Version"`   // 版本号
	// 是的确实没有别的东西
}

// ReadPreset 从 r 中读取 Preset
func ReadPreset(r io.Reader) (*Preset, error) {
	p := &Preset{}

	// 1. signature
	sig, err := binaryio.ReadString(r)
	if err != nil {
		return nil, fmt.Errorf("read .Preset signature failed: %w", err)
	}
	//if sig != "CM3D2_PRESET" {
	//	return nil, fmt.Errorf("ReadPreset: signature error, expect CM3D2_PRESET, got %s", sig)
	//}
	p.Signature = sig

	// 2. version
	version, err := binaryio.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("read .Preset version failed: %w", err)
	}
	p.Version = version

	// 3. presetType
	presetType, err := binaryio.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("read .Preset presetType failed: %w", err)
	}
	//if presetType < int32(PresetTypeWear) || presetType > int32(PresetTypeAll) {
	//	return nil, fmt.Errorf("invalid .Preset presetType: %d", presetType)
	//}
	p.PresetType = presetType

	// 4. ThumbLength
	thumbLength, err := binaryio.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("read .Preset ThumbLength failed: %w", err)
	}
	p.ThumbLength = thumbLength

	// 5. ThumbData
	if p.ThumbLength > 0 {
		p.ThumbData = make([]byte, p.ThumbLength)
		_, err = io.ReadFull(r, p.ThumbData)
		if err != nil {
			return nil, fmt.Errorf("read .Preset ThumbData failed: %w", err)
		}
	}

	// 6. listMprop
	p.PresetPropertyList, err = readPresetPropertyList(r)
	if err != nil {
		return nil, fmt.Errorf("read .Preset PresetPropertyList failed: %w", err)
	}

	// 7. MultiColor
	if version >= 2 && (version < 2000 || 10000 <= version) {
		mc, err := readMultiColor(r)
		if err != nil {
			return nil, fmt.Errorf("read .Preset MultiColor failed: %w", err)
		}
		p.MultiColor = mc
	}

	// 8. Body
	if version >= 200 && (version < 2000 || 10000 <= version) {
		bp, err := readBodyProperty(r)
		if err != nil {
			return nil, fmt.Errorf("read .Preset Body failed: %w", err)
		}
		p.BodyProperty = bp
	}

	return p, nil
}

// ReadPresetMetadata 从 r 中读取 PresetMetadata
func ReadPresetMetadata(r io.Reader) (*PresetMetadata, error) {
	p := &PresetMetadata{}

	// 1. signature
	sig, err := binaryio.ReadString(r)
	if err != nil {
		return nil, fmt.Errorf("read .Preset signature failed: %w", err)
	}
	//if sig != "CM3D2_PRESET" {
	//	return nil, fmt.Errorf("ReadPreset: signature error, expect CM3D2_PRESET, got %s", sig)
	//}
	p.Signature = sig

	// 2. version
	version, err := binaryio.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("read .Preset version failed: %w", err)
	}
	p.Version = version

	// 3. presetType
	presetType, err := binaryio.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("read .Preset presetType failed: %w", err)
	}
	//if presetType < int32(PresetTypeWear) || presetType > int32(PresetTypeAll) {
	//	return nil, fmt.Errorf("invalid .Preset presetType: %d", presetType)
	//}
	p.PresetType = presetType

	// 4. ThumbLength
	thumbLength, err := binaryio.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("read .Preset ThumbLength failed: %w", err)
	}
	p.ThumbLength = thumbLength

	// 5. ThumbData
	if p.ThumbLength > 0 {
		p.ThumbData = make([]byte, p.ThumbLength)
		_, err = io.ReadFull(r, p.ThumbData)
		if err != nil {
			return nil, fmt.Errorf("read .Preset ThumbData failed: %w", err)
		}
	}

	return p, nil
}

// readPresetPropertyList 从 r 中读取 PresetPropertyList
func readPresetPropertyList(r io.Reader) (*PresetPropertyList, error) {
	ppl := &PresetPropertyList{PresetProperties: map[string]PresetProperty{}}

	// 1. signature
	sig, err := binaryio.ReadString(r)
	if err != nil {
		return nil, fmt.Errorf("read .Preset PresetPropertyList signature failed: %w", err)
	}
	//if sig != "CM3D2_MPROP_LIST" {
	//	return ppl, fmt.Errorf("read .Preset PresetPropertyList signature error, expect CM3D2_MPROP_LIST, got %s", sig)
	//}
	ppl.Signature = sig

	// 2. version
	version, err := binaryio.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("read .Preset PresetPropertyList version failed: %w", err)
	}
	ppl.Version = version

	//3. PropertyCount
	count, err := binaryio.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("read .Preset PresetPropertyList propertyCount failed: %w", err)
	}
	ppl.PropertyCount = count

	// 4. PresetProperties
	for i := 0; i < int(count); i++ {
		var key string
		if version >= 4 {
			// 新版：SerializeProp 会先写 key（MPN 名称字符串）
			k, err := binaryio.ReadString(r)
			if err != nil {
				return nil, fmt.Errorf("read Prop key[%d] failed: %w", i, err)
			}
			key = k
		}
		prop, err := readPresetProperty(r)
		if err != nil {
			return nil, fmt.Errorf("read Prop[%d] failed: %w", i, err)
		}
		if key == "" {
			// 旧版未写 key，用 prop.Name 作为 key
			key = prop.Name
		}
		ppl.PresetProperties[key] = *prop
	}

	return ppl, nil
}

// 读取单个属性：MaidProp.Deserialize（格式对齐 MaidProp.Serialize）
func readPresetProperty(r io.Reader) (*PresetProperty, error) {
	prop := &PresetProperty{}

	// 1. signature
	sig, err := binaryio.ReadString(r)
	if err != nil {
		return nil, fmt.Errorf("read .Preset PresetProperty signature failed: %w", err)
	}
	prop.Signature = sig

	// 2. version
	ver, err := binaryio.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("read .Preset PresetProperty version failed: %w", err)
	}
	prop.Version = ver

	// 3. index
	idx, err := binaryio.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("read prop.idx failed: %w", err)
	}
	prop.Index = idx

	// 4. name
	name, err := binaryio.ReadString(r)
	if err != nil {
		return nil, fmt.Errorf("read prop.name failed: %w", err)
	}
	prop.Name = name

	// 5. 基本数值
	if prop.Type, err = binaryio.ReadInt32(r); err != nil {
		return nil, fmt.Errorf("read prop.type failed: %w", err)
	}
	if prop.DefaultValue, err = binaryio.ReadInt32(r); err != nil {
		return nil, fmt.Errorf("read prop.default failed: %w", err)
	}
	if prop.Value, err = binaryio.ReadInt32(r); err != nil {
		return nil, fmt.Errorf("read prop.value failed: %w", err)
	}
	if ver >= 101 {
		if prop.TempValue, err = binaryio.ReadInt32(r); err != nil {
			return nil, fmt.Errorf("read prop.temp_value failed: %w", err)
		}
	}
	if prop.LinkMaxValue, err = binaryio.ReadInt32(r); err != nil {
		return nil, fmt.Errorf("read prop.linkMax failed: %w", err)
	}
	if prop.FileName, err = binaryio.ReadString(r); err != nil {
		return nil, fmt.Errorf("read prop.fileName failed: %w", err)
	}
	if prop.FileNameRID, err = binaryio.ReadInt32(r); err != nil {
		return nil, fmt.Errorf("read prop.fileNameRID failed: %w", err)
	}
	if prop.IsDut, err = binaryio.ReadBool(r); err != nil {
		return nil, fmt.Errorf("read prop.isDut failed: %w", err)
	}
	if prop.Max, err = binaryio.ReadInt32(r); err != nil {
		return nil, fmt.Errorf("read prop.max failed: %w", err)
	}
	if prop.Min, err = binaryio.ReadInt32(r); err != nil {
		return nil, fmt.Errorf("read prop.min failed: %w", err)
	}

	// 子属性（ver >= 200）
	if ver >= 200 {
		cnt, err := binaryio.ReadInt32(r)
		if err != nil {
			return nil, fmt.Errorf("read subProp count failed: %w", err)
		}
		if cnt > 0 {
			prop.SubProps = make([]SubProp, cnt)
		}
		for i := 0; i < int(cnt); i++ {
			exists, err := binaryio.ReadBool(r)
			if err != nil {
				return nil, fmt.Errorf("read subProp[%d] exists failed: %w", i, err)
			}
			if !exists {
				// 无法用 nil 表示，跳过但保留默认零值
				continue
			}
			var sp SubProp
			if sp.IsDut, err = binaryio.ReadBool(r); err != nil {
				return nil, fmt.Errorf("read subProp[%d].IsDut failed: %w", i, err)
			}
			if sp.FileName, err = binaryio.ReadString(r); err != nil {
				return nil, fmt.Errorf("read subProp[%d].FileName failed: %w", i, err)
			}
			if sp.FileNameRID, err = binaryio.ReadInt32(r); err != nil {
				return nil, fmt.Errorf("read subProp[%d].FileNameRID failed: %w", i, err)
			}
			if ver >= 211 {
				if sp.TexMulAlpha, err = binaryio.ReadFloat32(r); err != nil {
					return nil, fmt.Errorf("read subProp[%d].TexMulAlpha failed: %w", i, err)
				}
			}
			prop.SubProps[i] = sp
		}

		// 皮肤位置：slotID, RID, BoneAttachPos
		nSkin, err := binaryio.ReadInt32(r)
		if err != nil {
			return nil, fmt.Errorf("read skinPos count failed: %w", err)
		}
		if nSkin > 0 {
			prop.SkinPositions = make(map[int]BoneAttachPosEntry, nSkin)
		}
		for i := 0; i < int(nSkin); i++ {
			slotID, err := binaryio.ReadInt32(r)
			if err != nil {
				return nil, fmt.Errorf("read skinPos[%d].slotID failed: %w", i, err)
			}
			rid, err := binaryio.ReadInt32(r)
			if err != nil {
				return nil, fmt.Errorf("read skinPos[%d].rid failed: %w", i, err)
			}
			var b BoneAttachPos
			if b.Enable, err = binaryio.ReadBool(r); err != nil {
				return nil, fmt.Errorf("read skinPos[%d].enable failed: %w", i, err)
			}
			if b.PosRotScale.Position.X, err = binaryio.ReadFloat32(r); err != nil {
				return nil, fmt.Errorf("read skinPos[%d].pos.x failed: %w", i, err)
			}
			if b.PosRotScale.Position.Y, err = binaryio.ReadFloat32(r); err != nil {
				return nil, fmt.Errorf("read skinPos[%d].pos.y failed: %w", i, err)
			}
			if b.PosRotScale.Position.Z, err = binaryio.ReadFloat32(r); err != nil {
				return nil, fmt.Errorf("read skinPos[%d].pos.z failed: %w", i, err)
			}
			if b.PosRotScale.Rotation.X, err = binaryio.ReadFloat32(r); err != nil {
				return nil, fmt.Errorf("read skinPos[%d].rot.x failed: %w", i, err)
			}
			if b.PosRotScale.Rotation.Y, err = binaryio.ReadFloat32(r); err != nil {
				return nil, fmt.Errorf("read skinPos[%d].rot.y failed: %w", i, err)
			}
			if b.PosRotScale.Rotation.Z, err = binaryio.ReadFloat32(r); err != nil {
				return nil, fmt.Errorf("read skinPos[%d].rot.z failed: %w", i, err)
			}
			if b.PosRotScale.Rotation.W, err = binaryio.ReadFloat32(r); err != nil {
				return nil, fmt.Errorf("read skinPos[%d].rot.w failed: %w", i, err)
			}
			if b.PosRotScale.Scale.X, err = binaryio.ReadFloat32(r); err != nil {
				return nil, fmt.Errorf("read skinPos[%d].scale.x failed: %w", i, err)
			}
			if b.PosRotScale.Scale.Y, err = binaryio.ReadFloat32(r); err != nil {
				return nil, fmt.Errorf("read skinPos[%d].scale.y failed: %w", i, err)
			}
			if b.PosRotScale.Scale.Z, err = binaryio.ReadFloat32(r); err != nil {
				return nil, fmt.Errorf("read skinPos[%d].scale.z failed: %w", i, err)
			}

			prop.SkinPositions[int(slotID)] = BoneAttachPosEntry{RID: rid, BoneAttachPos: b}
		}

		// 附着位置：slotID, count, (name, RID, VtxAttachPos)*
		nAttach, err := binaryio.ReadInt32(r)
		if err != nil {
			return nil, fmt.Errorf("read attachPos count failed: %w", err)
		}
		if nAttach > 0 {
			prop.AttachPositions = make(map[int]map[string]VtxAttachPosEntry, nAttach)
		}
		for i := 0; i < int(nAttach); i++ {
			slotID, err := binaryio.ReadInt32(r)
			if err != nil {
				return nil, fmt.Errorf("read attachPos[%d].slotID failed: %w", i, err)
			}
			inner, err := binaryio.ReadInt32(r)
			if err != nil {
				return nil, fmt.Errorf("read attachPos[%d].innerCount failed: %w", i, err)
			}
			mp := make(map[string]VtxAttachPosEntry, inner)
			for j := 0; j < int(inner); j++ {
				key, err := binaryio.ReadString(r)
				if err != nil {
					return nil, fmt.Errorf("read attachPos[%d][%d].key failed: %w", i, j, err)
				}
				rid, err := binaryio.ReadInt32(r)
				if err != nil {
					return nil, fmt.Errorf("read attachPos[%d][%d].rid failed: %w", i, j, err)
				}
				var v VtxAttachPos
				if v.Enable, err = binaryio.ReadBool(r); err != nil {
					return nil, fmt.Errorf("read attachPos[%d][%d].enable failed: %w", i, j, err)
				}
				if v.VtxCount, err = binaryio.ReadInt32(r); err != nil {
					return nil, fmt.Errorf("read attachPos[%d][%d].vtxCount failed: %w", i, j, err)
				}
				if v.VtxIdx, err = binaryio.ReadInt32(r); err != nil {
					return nil, fmt.Errorf("read attachPos[%d][%d].vtxIdx failed: %w", i, j, err)
				}
				if v.PosRotScale.Position.X, err = binaryio.ReadFloat32(r); err != nil {
					return nil, fmt.Errorf("read attachPos[%d][%d].pos.x failed: %w", i, j, err)
				}
				if v.PosRotScale.Position.Y, err = binaryio.ReadFloat32(r); err != nil {
					return nil, fmt.Errorf("read attachPos[%d][%d].pos.y failed: %w", i, j, err)
				}
				if v.PosRotScale.Position.Z, err = binaryio.ReadFloat32(r); err != nil {
					return nil, fmt.Errorf("read attachPos[%d][%d].pos.z failed: %w", i, j, err)
				}
				if v.PosRotScale.Rotation.X, err = binaryio.ReadFloat32(r); err != nil {
					return nil, fmt.Errorf("read attachPos[%d][%d].rot.x failed: %w", i, j, err)
				}
				if v.PosRotScale.Rotation.Y, err = binaryio.ReadFloat32(r); err != nil {
					return nil, fmt.Errorf("read attachPos[%d][%d].rot.y failed: %w", i, j, err)
				}
				if v.PosRotScale.Rotation.Z, err = binaryio.ReadFloat32(r); err != nil {
					return nil, fmt.Errorf("read attachPos[%d][%d].rot.z failed: %w", i, j, err)
				}
				if v.PosRotScale.Rotation.W, err = binaryio.ReadFloat32(r); err != nil {
					return nil, fmt.Errorf("read attachPos[%d][%d].rot.w failed: %w", i, j, err)
				}
				if v.PosRotScale.Scale.X, err = binaryio.ReadFloat32(r); err != nil {
					return nil, fmt.Errorf("read attachPos[%d][%d].scale.x failed: %w", i, j, err)
				}
				if v.PosRotScale.Scale.Y, err = binaryio.ReadFloat32(r); err != nil {
					return nil, fmt.Errorf("read attachPos[%d][%d].scale.y failed: %w", i, j, err)
				}
				if v.PosRotScale.Scale.Z, err = binaryio.ReadFloat32(r); err != nil {
					return nil, fmt.Errorf("read attachPos[%d][%d].scale.z failed: %w", i, j, err)
				}

				mp[key] = VtxAttachPosEntry{RID: rid, VtxAttachPos: v}
			}
			prop.AttachPositions[int(slotID)] = mp
		}

		// 材质属性：slotID, RID, MatPropSave
		nMat, err := binaryio.ReadInt32(r)
		if err != nil {
			return nil, fmt.Errorf("read matProp count failed: %w", err)
		}
		if nMat > 0 {
			prop.MaterialProps = make(map[int]MatPropSaveEntry, nMat)
		}
		for i := 0; i < int(nMat); i++ {
			slotID, err := binaryio.ReadInt32(r)
			if err != nil {
				return nil, fmt.Errorf("read matProp[%d].slotID failed: %w", i, err)
			}
			rid, err := binaryio.ReadInt32(r)
			if err != nil {
				return nil, fmt.Errorf("read matProp[%d].rid failed: %w", i, err)
			}
			var m MatPropSave
			if m.MatId, err = binaryio.ReadInt32(r); err != nil {
				return nil, fmt.Errorf("read matProp[%d].matId failed: %w", i, err)
			}
			if m.PropName, err = binaryio.ReadString(r); err != nil {
				return nil, fmt.Errorf("read matProp[%d].propName failed: %w", i, err)
			}
			if m.TypeName, err = binaryio.ReadString(r); err != nil {
				return nil, fmt.Errorf("read matProp[%d].typeName failed: %w", i, err)
			}
			if m.Value, err = binaryio.ReadString(r); err != nil {
				return nil, fmt.Errorf("read matProp[%d].value failed: %w", i, err)
			}
			prop.MaterialProps[int(slotID)] = MatPropSaveEntry{RID: rid, MatPropSave: m}
		}

		// 骨骼长度（ver >= 213）：slotID, RID, count, (name,float)*
		if ver >= 213 {
			nBone, err := binaryio.ReadInt32(r)
			if err != nil {
				return nil, fmt.Errorf("read boneLen count failed: %w", err)
			}
			if nBone > 0 {
				prop.BoneLengths = make(map[int]BoneLengthEntry, nBone)
			}
			for i := 0; i < int(nBone); i++ {
				slotID, err := binaryio.ReadInt32(r)
				if err != nil {
					return nil, fmt.Errorf("read boneLen[%d].slotID failed: %w", i, err)
				}
				rid, err := binaryio.ReadInt32(r)
				if err != nil {
					return nil, fmt.Errorf("read boneLen[%d].rid failed: %w", i, err)
				}
				inner, err := binaryio.ReadInt32(r)
				if err != nil {
					return nil, fmt.Errorf("read boneLen[%d].inner failed: %w", i, err)
				}
				m := make(map[string]float32, inner)
				for j := 0; j < int(inner); j++ {
					k, err := binaryio.ReadString(r)
					if err != nil {
						return nil, fmt.Errorf("read boneLen[%d][%d].name failed: %w", i, j, err)
					}
					v, err := binaryio.ReadFloat32(r)
					if err != nil {
						return nil, fmt.Errorf("read boneLen[%d][%d].value failed: %w", i, j, err)
					}
					m[k] = v
				}
				prop.BoneLengths[int(slotID)] = BoneLengthEntry{RID: rid, Lengths: m}
			}
		}
	}

	return prop, nil
}

// 读取多颜色：MaidParts.DeserializePre（兼容旧/新）
func readMultiColor(r io.Reader) (*MultiColor, error) {
	mc := &MultiColor{}

	sig, err := binaryio.ReadString(r)
	if err != nil {
		return nil, fmt.Errorf("read MultiColor signature failed: %w", err)
	}
	mc.Signature = sig

	ver, err := binaryio.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("read MultiColor version failed: %w", err)
	}
	mc.Version = ver

	count, err := binaryio.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("read MultiColor count failed: %w", err)
	}
	mc.PartCount = count

	// 统一生成长度 13 的颜色数组（与新版本一致）
	colors := make([]PartsColor, 13)

	if ver <= 1200 {
		// 旧布局（前 9 项，固定顺序）
		// 数量为 7 或 9
		order := []string{"EYE_L", "EYE_R", "HAIR", "EYE_BROW", "UNDER_HAIR", "SKIN", "NIPPLE", "HAIR_OUTLINE", "SKIN_OUTLINE"}
		for j := 0; j < int(count); j++ {
			pc := PartsColor{}
			if pc.IsUse, err = binaryio.ReadBool(r); err != nil {
				return nil, fmt.Errorf("read MultiColor[%d].isUse failed: %w", j, err)
			}
			if pc.MainHue, err = binaryio.ReadInt32(r); err != nil {
				return nil, fmt.Errorf("read MultiColor[%d].mainHue failed: %w", j, err)
			}
			if pc.MainChroma, err = binaryio.ReadInt32(r); err != nil {
				return nil, fmt.Errorf("read MultiColor[%d].mainChroma failed: %w", j, err)
			}
			if pc.MainBrightness, err = binaryio.ReadInt32(r); err != nil {
				return nil, fmt.Errorf("read MultiColor[%d].mainBrightness failed: %w", j, err)
			}
			if pc.MainContrast, err = binaryio.ReadInt32(r); err != nil {
				return nil, fmt.Errorf("read MultiColor[%d].mainContrast failed: %w", j, err)
			}
			if pc.ShadowRate, err = binaryio.ReadInt32(r); err != nil {
				return nil, fmt.Errorf("read MultiColor[%d].shadowRate failed: %w", j, err)
			}
			if pc.ShadowHue, err = binaryio.ReadInt32(r); err != nil {
				return nil, fmt.Errorf("read MultiColor[%d].shadowHue failed: %w", j, err)
			}
			if pc.ShadowChroma, err = binaryio.ReadInt32(r); err != nil {
				return nil, fmt.Errorf("read MultiColor[%d].shadowChroma failed: %w", j, err)
			}
			if pc.ShadowBrightness, err = binaryio.ReadInt32(r); err != nil {
				return nil, fmt.Errorf("read MultiColor[%d].shadowBrightness failed: %w", j, err)
			}
			if pc.ShadowContrast, err = binaryio.ReadInt32(r); err != nil {
				return nil, fmt.Errorf("read MultiColor[%d].shadowContrast failed: %w", j, err)
			}
			// 仅旧格式前 9 项有固定顺序，多余项只消费不落位，避免越界
			if j >= len(order) {
				continue
			}
			idx := partsColorIndex(order[j])
			if idx >= 0 && idx < len(colors) {
				colors[idx] = pc
			}
		}
	} else {
		// 新布局：读字符串直到 "MAX"
		for {
			name, err := binaryio.ReadString(r)
			if err != nil {
				return nil, fmt.Errorf("read MultiColor entry name failed: %w", err)
			}
			if name == "MAX" {
				break
			}
			idx := partsColorIndex(name)
			if idx < 0 || idx >= len(colors) {
				// 跳过未知项但消费字段
				var dummy PartsColor
				if _, err = readPartsColor(r, &dummy); err != nil {
					return nil, fmt.Errorf("read MultiColor[%d] failed: %w", idx, err)
				}
				continue
			}
			if _, err = readPartsColor(r, &colors[idx]); err != nil {
				return nil, fmt.Errorf("read MultiColor[%d] failed: %w", idx, err)
			}
		}
	}

	mc.PartsColors = colors
	return mc, nil
}

func readPartsColor(r io.Reader, pc *PartsColor) (int, error) {
	var err error
	if pc.IsUse, err = binaryio.ReadBool(r); err != nil {
		return 0, fmt.Errorf("read isUse failed: %w", err)
	}
	if pc.MainHue, err = binaryio.ReadInt32(r); err != nil {
		return 0, fmt.Errorf("read mainHue failed: %w", err)
	}
	if pc.MainChroma, err = binaryio.ReadInt32(r); err != nil {
		return 0, fmt.Errorf("read mainChroma failed: %w", err)
	}
	if pc.MainBrightness, err = binaryio.ReadInt32(r); err != nil {
		return 0, fmt.Errorf("read mainBrightness failed: %w", err)
	}
	if pc.MainContrast, err = binaryio.ReadInt32(r); err != nil {
		return 0, fmt.Errorf("read mainContrast failed: %w", err)
	}
	if pc.ShadowRate, err = binaryio.ReadInt32(r); err != nil {
		return 0, fmt.Errorf("read shadowRate failed: %w", err)
	}
	if pc.ShadowHue, err = binaryio.ReadInt32(r); err != nil {
		return 0, fmt.Errorf("read shadowHue failed: %w", err)
	}
	if pc.ShadowChroma, err = binaryio.ReadInt32(r); err != nil {
		return 0, fmt.Errorf("read shadowChroma failed: %w", err)
	}
	if pc.ShadowBrightness, err = binaryio.ReadInt32(r); err != nil {
		return 0, fmt.Errorf("read shadowBrightness failed: %w", err)
	}
	if pc.ShadowContrast, err = binaryio.ReadInt32(r); err != nil {
		return 0, fmt.Errorf("read shadowContrast failed: %w", err)
	}
	return 10, nil
}

// 读取身体块：Maid.DeserializeBodyRead
func readBodyProperty(r io.Reader) (*BodyProperty, error) {
	bp := &BodyProperty{}
	sig, err := binaryio.ReadString(r)
	if err != nil {
		return nil, fmt.Errorf("read Body signature failed: %w", err)
	}
	bp.Signature = sig
	ver, err := binaryio.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("read Body version failed: %w", err)
	}
	bp.Version = ver
	// 该块目前没有更多字段
	return bp, nil
}

// 多颜色名到索引（与 MaidParts.PARTS_COLOR 对齐）
func partsColorIndex(name string) int {
	switch name {
	case "NONE":
		return -1
	case "EYE_L":
		return 0
	case "EYE_R":
		return 1
	case "HAIR":
		return 2
	case "EYE_BROW":
		return 3
	case "UNDER_HAIR":
		return 4
	case "SKIN":
		return 5
	case "NIPPLE":
		return 6
	case "HAIR_OUTLINE":
		return 7
	case "SKIN_OUTLINE":
		return 8
	case "EYE_WHITE":
		return 9
	case "MATSUGE_UP":
		return 10
	case "MATSUGE_LOW":
		return 11
	case "FUTAE":
		return 12
	default:
		return -1
	}
}

func (p *Preset) Dump(w io.Writer) error {
	// 1. signature
	if err := binaryio.WriteString(w, p.Signature); err != nil {
		return fmt.Errorf("write preset signature failed: %w", err)
	}

	// 2. version
	if err := binaryio.WriteInt32(w, p.Version); err != nil {
		return fmt.Errorf("write preset version failed: %w", err)
	}

	// 3. presetType
	if err := binaryio.WriteInt32(w, p.PresetType); err != nil {
		return fmt.Errorf("write preset type failed: %w", err)
	}

	// 4. thumb
	if len(p.ThumbData) > 0 {
		if err := binaryio.WriteInt32(w, int32(len(p.ThumbData))); err != nil {
			return fmt.Errorf("write thumb length failed: %w", err)
		}
		if _, err := w.Write(p.ThumbData); err != nil {
			return fmt.Errorf("write thumb data failed: %w", err)
		}
	} else {
		if err := binaryio.WriteInt32(w, 0); err != nil {
			return fmt.Errorf("write thumb length(0) failed: %w", err)
		}
	}

	// 5. mprop list
	if err := dumpPresetPropertyList(w, p.PresetPropertyList); err != nil {
		return fmt.Errorf("write PresetPropertyList failed: %w", err)
	}

	// 6. multicolor
	if p.Version >= 2 && (p.Version < 2000 || 10000 <= p.Version) {
		if err := dumpMultiColor(w, p.MultiColor); err != nil {
			return fmt.Errorf("write MultiColor failed: %w", err)
		}
	}

	// 7. body
	if p.Version >= 200 && (p.Version < 2000 || 10000 <= p.Version) {
		if err := dumpBodyProperty(w, p.BodyProperty); err != nil {
			return fmt.Errorf("write Body failed: %w", err)
		}
	}

	return nil
}

// 写入属性列表：Maid.SerializeProp
func dumpPresetPropertyList(w io.Writer, ppl *PresetPropertyList) error {
	if ppl == nil {
		return fmt.Errorf("write PresetPropertyList failed: PresetPropertyList is nil")
	}

	if err := binaryio.WriteString(w, ppl.Signature); err != nil {
		return fmt.Errorf("write preset property list signature failed: %w", err)
	}

	if err := binaryio.WriteInt32(w, ppl.Version); err != nil {
		return fmt.Errorf("write preset property list version failed: %w", err)
	}

	count := int32(len(ppl.PresetProperties))
	if err := binaryio.WriteInt32(w, count); err != nil {
		return fmt.Errorf("write preset property list count failed: %w", err)
	}

	for k, v := range ppl.PresetProperties {
		// 仅当列表版本 >= 4 时写 key（MPN 字符串）
		if ppl.Version >= 4 {
			if err := binaryio.WriteString(w, k); err != nil {
				return fmt.Errorf("write prop key failed: %w", err)
			}
		}
		prop := v // copy
		if err := writePresetProperty(w, &prop); err != nil {
			return fmt.Errorf("write prop '%s' failed: %w", k, err)
		}
	}

	return nil
}

// 写入单个属性：MaidProp.Serialize
func writePresetProperty(w io.Writer, pp *PresetProperty) error {
	if err := binaryio.WriteString(w, pp.Signature); err != nil {
		return fmt.Errorf("write prop signature failed: %w", err)
	}
	ver := pp.Version
	if err := binaryio.WriteInt32(w, ver); err != nil {
		return fmt.Errorf("write prop version failed: %w", err)
	}
	if err := binaryio.WriteInt32(w, pp.Index); err != nil {
		return fmt.Errorf("write prop index failed: %w", err)
	}
	if err := binaryio.WriteString(w, pp.Name); err != nil {
		return fmt.Errorf("write prop name failed: %w", err)
	}
	if err := binaryio.WriteInt32(w, pp.Type); err != nil {
		return fmt.Errorf("write prop type failed: %w", err)
	}
	if err := binaryio.WriteInt32(w, pp.DefaultValue); err != nil {
		return fmt.Errorf("write prop default value failed: %w", err)
	}
	if err := binaryio.WriteInt32(w, pp.Value); err != nil {
		return fmt.Errorf("write prop value failed: %w", err)
	}
	// ver >= 101 才写 TempValue
	if ver >= 101 {
		if err := binaryio.WriteInt32(w, pp.TempValue); err != nil {
			return fmt.Errorf("write prop temp value failed: %w", err)
		}
	}
	if err := binaryio.WriteInt32(w, pp.LinkMaxValue); err != nil {
		return fmt.Errorf("write prop link max value failed: %w", err)
	}
	if err := binaryio.WriteString(w, pp.FileName); err != nil {
		return fmt.Errorf("write prop file name failed: %w", err)
	}
	if err := binaryio.WriteInt32(w, pp.FileNameRID); err != nil {
		return fmt.Errorf("write prop file name rid failed: %w", err)
	}
	if err := binaryio.WriteBool(w, pp.IsDut); err != nil {
		return fmt.Errorf("write prop is dut failed: %w", err)
	}
	if err := binaryio.WriteInt32(w, pp.Max); err != nil {
		return fmt.Errorf("write prop max failed: %w", err)
	}
	if err := binaryio.WriteInt32(w, pp.Min); err != nil {
		return fmt.Errorf("write prop min failed: %w", err)
	}
	// 仅当 ver >= 200 时才写入子属性与附加块（与读取保持一致）
	if ver >= 200 {
		// 子属性
		nSub := int32(len(pp.SubProps))
		if err := binaryio.WriteInt32(w, nSub); err != nil {
			return fmt.Errorf("write prop sub count failed: %w", err)
		}
		for i := 0; i < int(nSub); i++ {
			sp := pp.SubProps[i]
			// 是否存在
			if err := binaryio.WriteBool(w, true); err != nil {
				return fmt.Errorf("write prop sub exists failed: %w", err)
			}
			if err := binaryio.WriteBool(w, sp.IsDut); err != nil {
				return fmt.Errorf("write prop sub is dut failed: %w", err)
			}
			if err := binaryio.WriteString(w, sp.FileName); err != nil {
				return fmt.Errorf("write prop sub file name failed: %w", err)
			}
			if err := binaryio.WriteInt32(w, sp.FileNameRID); err != nil {
				return fmt.Errorf("write prop sub file name rid failed: %w", err)
			}
			// ver >= 211 才写 TexMulAlpha
			if ver >= 211 {
				if err := binaryio.WriteFloat32(w, sp.TexMulAlpha); err != nil {
					return fmt.Errorf("write prop sub tex mul alpha failed: %w", err)
				}
			}
		}

		// 皮肤位置：PropertyCount -> [slotID, RID, data...]
		if len(pp.SkinPositions) == 0 {
			if err := binaryio.WriteInt32(w, 0); err != nil {
				return fmt.Errorf("write prop skin position count failed: %w", err)
			}
		} else {
			if err := binaryio.WriteInt32(w, int32(len(pp.SkinPositions))); err != nil {
				return fmt.Errorf("write prop skin position count failed: %w", err)
			}
			for slot, e := range pp.SkinPositions {
				if err := binaryio.WriteInt32(w, int32(slot)); err != nil {
					return fmt.Errorf("write prop skin position slot failed: %w", err)
				}
				if err := binaryio.WriteInt32(w, e.RID); err != nil {
					return fmt.Errorf("write prop skin position rid failed: %w", err)
				}
				b := e.BoneAttachPos
				if err := binaryio.WriteBool(w, b.Enable); err != nil {
					return fmt.Errorf("write prop skin position bone attach pos enable failed: %w", err)
				}
				if err := binaryio.WriteFloat32(w, b.PosRotScale.Position.X); err != nil {
					return fmt.Errorf("write prop skin position bone attach pos position x failed: %w", err)
				}
				if err := binaryio.WriteFloat32(w, b.PosRotScale.Position.Y); err != nil {
					return fmt.Errorf("write prop skin position bone attach pos position y failed: %w", err)
				}
				if err := binaryio.WriteFloat32(w, b.PosRotScale.Position.Z); err != nil {
					return fmt.Errorf("write prop skin position bone attach pos position z failed: %w", err)
				}
				if err := binaryio.WriteFloat32(w, b.PosRotScale.Rotation.X); err != nil {
					return fmt.Errorf("write prop skin position bone attach pos rotation x failed: %w", err)
				}
				if err := binaryio.WriteFloat32(w, b.PosRotScale.Rotation.Y); err != nil {
					return fmt.Errorf("write prop skin position bone attach pos rotation y failed: %w", err)
				}
				if err := binaryio.WriteFloat32(w, b.PosRotScale.Rotation.Z); err != nil {
					return fmt.Errorf("write prop skin position bone attach pos rotation z failed: %w", err)
				}
				if err := binaryio.WriteFloat32(w, b.PosRotScale.Rotation.W); err != nil {
					return fmt.Errorf("write prop skin position bone attach pos rotation w failed: %w", err)
				}
				if err := binaryio.WriteFloat32(w, b.PosRotScale.Scale.X); err != nil {
					return fmt.Errorf("write prop skin position bone attach pos scale x failed: %w", err)
				}
				if err := binaryio.WriteFloat32(w, b.PosRotScale.Scale.Y); err != nil {
					return fmt.Errorf("write prop skin position bone attach pos scale y failed: %w", err)
				}
				if err := binaryio.WriteFloat32(w, b.PosRotScale.Scale.Z); err != nil {
					return fmt.Errorf("write prop skin position bone attach pos scale z failed: %w", err)
				}
			}
		}

		// 附着位置：slotID -> map[name]Entry
		if len(pp.AttachPositions) == 0 {
			if err := binaryio.WriteInt32(w, 0); err != nil {
				return fmt.Errorf("write prop attach position count failed: %w", err)
			}
		} else {
			if err := binaryio.WriteInt32(w, int32(len(pp.AttachPositions))); err != nil {
				return fmt.Errorf("write prop attach position count failed: %w", err)
			}
			for slot, mp := range pp.AttachPositions {
				if err := binaryio.WriteInt32(w, int32(slot)); err != nil {
					return fmt.Errorf("write prop attach position slot failed: %w", err)
				}
				if err := binaryio.WriteInt32(w, int32(len(mp))); err != nil {
					return fmt.Errorf("write prop attach position name count failed: %w", err)
				}
				for name, e := range mp {
					if err := binaryio.WriteString(w, name); err != nil {
						return fmt.Errorf("write prop attach position name failed: %w", err)
					}
					if err := binaryio.WriteInt32(w, e.RID); err != nil {
						return fmt.Errorf("write prop attach position rid failed: %w", err)
					}
					v := e.VtxAttachPos
					if err := binaryio.WriteBool(w, v.Enable); err != nil {
						return fmt.Errorf("write prop attach position vtx attach pos enable failed: %w", err)
					}
					if err := binaryio.WriteInt32(w, v.VtxCount); err != nil {
						return fmt.Errorf("write prop attach position vtx attach pos vtx count failed: %w", err)
					}
					if err := binaryio.WriteInt32(w, v.VtxIdx); err != nil {
						return fmt.Errorf("write prop attach position vtx attach pos vtx idx failed: %w", err)
					}
					if err := binaryio.WriteFloat32(w, v.PosRotScale.Position.X); err != nil {
						return fmt.Errorf("write prop attach position vtx attach pos position x failed: %w", err)
					}
					if err := binaryio.WriteFloat32(w, v.PosRotScale.Position.Y); err != nil {
						return fmt.Errorf("write prop attach position vtx attach pos position y failed: %w", err)
					}
					if err := binaryio.WriteFloat32(w, v.PosRotScale.Position.Z); err != nil {
						return fmt.Errorf("write prop attach position vtx attach pos position z failed: %w", err)
					}
					if err := binaryio.WriteFloat32(w, v.PosRotScale.Rotation.X); err != nil {
						return fmt.Errorf("write prop attach position vtx attach pos rotation x failed: %w", err)
					}
					if err := binaryio.WriteFloat32(w, v.PosRotScale.Rotation.Y); err != nil {
						return fmt.Errorf("write prop attach position vtx attach pos rotation y failed: %w", err)
					}
					if err := binaryio.WriteFloat32(w, v.PosRotScale.Rotation.Z); err != nil {
						return fmt.Errorf("write prop attach position vtx attach pos rotation z failed: %w", err)
					}
					if err := binaryio.WriteFloat32(w, v.PosRotScale.Rotation.W); err != nil {
						return fmt.Errorf("write prop attach position vtx attach pos rotation w failed: %w", err)
					}
					if err := binaryio.WriteFloat32(w, v.PosRotScale.Scale.X); err != nil {
						return fmt.Errorf("write prop attach position vtx attach pos scale x failed: %w", err)
					}
					if err := binaryio.WriteFloat32(w, v.PosRotScale.Scale.Y); err != nil {
						return fmt.Errorf("write prop attach position vtx attach pos scale y failed: %w", err)
					}
					if err := binaryio.WriteFloat32(w, v.PosRotScale.Scale.Z); err != nil {
						return fmt.Errorf("write prop attach position vtx attach pos scale z failed: %w", err)
					}
				}
			}
		}

		// 材质属性：slotID -> Entry
		if len(pp.MaterialProps) == 0 {
			if err := binaryio.WriteInt32(w, 0); err != nil {
				return fmt.Errorf("write prop material prop count failed: %w", err)
			}
		} else {
			if err := binaryio.WriteInt32(w, int32(len(pp.MaterialProps))); err != nil {
				return fmt.Errorf("write prop material prop count failed: %w", err)
			}
			for slot, e := range pp.MaterialProps {
				if err := binaryio.WriteInt32(w, int32(slot)); err != nil {
					return fmt.Errorf("write prop material prop slot failed: %w", err)
				}
				if err := binaryio.WriteInt32(w, e.RID); err != nil {
					return fmt.Errorf("write prop material prop rid failed: %w", err)
				}
				m := e.MatPropSave
				if err := binaryio.WriteInt32(w, m.MatId); err != nil {
					return fmt.Errorf("write prop material prop mat id failed: %w", err)
				}
				if err := binaryio.WriteString(w, m.PropName); err != nil {
					return fmt.Errorf("write prop material prop prop name failed: %w", err)
				}
				if err := binaryio.WriteString(w, m.TypeName); err != nil {
					return fmt.Errorf("write prop material prop type name failed: %w", err)
				}
				if err := binaryio.WriteString(w, m.Value); err != nil {
					return fmt.Errorf("write prop material prop value failed: %w", err)
				}
			}
		}

		// 骨骼长度块仅在 ver >= 213 时写入
		if ver >= 213 {
			if len(pp.BoneLengths) == 0 {
				if err := binaryio.WriteInt32(w, 0); err != nil {
					return fmt.Errorf("write prop bone length count failed: %w", err)
				}
			} else {
				if err := binaryio.WriteInt32(w, int32(len(pp.BoneLengths))); err != nil {
					return fmt.Errorf("write prop bone length count failed: %w", err)
				}
				for slot, e := range pp.BoneLengths {
					if err := binaryio.WriteInt32(w, int32(slot)); err != nil {
						return fmt.Errorf("write prop bone length slot failed: %w", err)
					}
					if err := binaryio.WriteInt32(w, e.RID); err != nil {
						return fmt.Errorf("write prop bone length rid failed: %w", err)
					}
					if err := binaryio.WriteInt32(w, int32(len(e.Lengths))); err != nil {
						return fmt.Errorf("write prop bone length len count failed: %w", err)
					}
					for k, v := range e.Lengths {
						if err := binaryio.WriteString(w, k); err != nil {
							return fmt.Errorf("write prop bone length len name failed: %w", err)
						}
						if err := binaryio.WriteFloat32(w, v); err != nil {
							return fmt.Errorf("write prop bone length len value failed: %w", err)
						}
					}
				}
			}
		}
	}

	return nil
}

// 写入多颜色：MaidParts.Serialize（统一按新版本写）
func dumpMultiColor(w io.Writer, mc *MultiColor) error {
	if mc == nil {
		return fmt.Errorf("write MultiColor failed: MultiColor is nil")
	}

	if err := binaryio.WriteString(w, mc.Signature); err != nil {
		return fmt.Errorf("write prop multi color name failed: %w", err)
	}
	if err := binaryio.WriteInt32(w, mc.Version); err != nil {
		return fmt.Errorf("write prop multi color version failed: %w", err)
	}

	if mc.Version <= 1200 {
		// 旧格式：固定顺序、无名字串、无 "MAX"
		var order []int // 索引映射到你的 colors
		if mc.Version < 200 {
			// 7 项
			order = []int{0, 1, 2, 3, 4, 5, 6} // EYE_L, EYE_R, HAIR, EYE_BROW, UNDER_HAIR, SKIN, NIPPLE
		} else {
			// 9 项
			order = []int{0, 1, 2, 3, 4, 5, 6, 7, 8} // EYE_L, EYE_R, HAIR, EYE_BROW, UNDER_HAIR, SKIN, NIPPLE, HAIR_OUTLINE, SKIN_OUTLINE
		}
		if err := binaryio.WriteInt32(w, int32(len(order))); err != nil {
			return fmt.Errorf("write prop multi color len count failed: %w", err)
		}
		colors := mc.PartsColors
		need := len(order)
		if len(colors) < need {
			tmp := make([]PartsColor, need)
			copy(tmp, colors)
			colors = tmp // 用零值补足未提供的项
		}
		for _, idx := range order {
			pc := colors[idx]
			if err := binaryio.WriteBool(w, pc.IsUse); err != nil {
				return fmt.Errorf("write prop multi color is use failed: %w", err)
			}
			if err := binaryio.WriteInt32(w, pc.MainHue); err != nil {
				return fmt.Errorf("write prop multi color main hue failed: %w", err)
			}
			if err := binaryio.WriteInt32(w, pc.MainChroma); err != nil {
				return fmt.Errorf("write prop multi color main hue failed: %w", err)
			}
			if err := binaryio.WriteInt32(w, pc.MainBrightness); err != nil {
				return fmt.Errorf("write prop multi color main brightness failed: %w", err)
			}
			if err := binaryio.WriteInt32(w, pc.MainContrast); err != nil {
				return fmt.Errorf("write prop multi color main contrast failed: %w", err)
			}
			if err := binaryio.WriteInt32(w, pc.ShadowRate); err != nil {
				return fmt.Errorf("write prop multi color shadow rate failed: %w", err)
			}
			if err := binaryio.WriteInt32(w, pc.ShadowHue); err != nil {
				return fmt.Errorf("write prop multi color shadow hue failed: %w", err)
			}
			if err := binaryio.WriteInt32(w, pc.ShadowChroma); err != nil {
				return fmt.Errorf("write prop multi color shadow chroma failed: %w", err)
			}
			if err := binaryio.WriteInt32(w, pc.ShadowBrightness); err != nil {
				return fmt.Errorf("write prop multi color shadow contrast failed: %w", err)
			}
			if err := binaryio.WriteInt32(w, pc.ShadowContrast); err != nil {
				return fmt.Errorf("write prop multi color shadow chroma failed: %w", err)
			}
		}
		return nil
	}

	// 统一写 13, C# 中直接初始化为 m_aryPartsColor = new MaidParts.PartsColor[13]; 写入 m_aryPartsColor.Length
	names := []string{"EYE_L", "EYE_R", "HAIR", "EYE_BROW", "UNDER_HAIR", "SKIN", "NIPPLE", "HAIR_OUTLINE", "SKIN_OUTLINE", "EYE_WHITE", "MATSUGE_UP", "MATSUGE_LOW", "FUTAE"}
	if err := binaryio.WriteInt32(w, int32(len(names))); err != nil {
		return fmt.Errorf("write prop multi color len count failed: %w", err)
	}
	colors := mc.PartsColors
	if len(colors) < len(names) {
		tmp := make([]PartsColor, len(names))
		copy(tmp, colors)
		colors = tmp
	}
	for i, name := range names {
		if err := binaryio.WriteString(w, name); err != nil {
			return fmt.Errorf("write prop multi color name failed: %w", err)
		}
		pc := colors[i]
		if err := binaryio.WriteBool(w, pc.IsUse); err != nil {
			return fmt.Errorf("write prop multi color is use failed: %w", err)
		}
		if err := binaryio.WriteInt32(w, pc.MainHue); err != nil {
			return fmt.Errorf("write prop multi color main hue failed: %w", err)
		}
		if err := binaryio.WriteInt32(w, pc.MainChroma); err != nil {
			return fmt.Errorf("write prop multi color main chroma failed: %w", err)
		}
		if err := binaryio.WriteInt32(w, pc.MainBrightness); err != nil {
			return fmt.Errorf("write prop multi color main brightness failed: %w", err)
		}
		if err := binaryio.WriteInt32(w, pc.MainContrast); err != nil {
			return fmt.Errorf("write prop multi color main contrast failed: %w", err)
		}
		if err := binaryio.WriteInt32(w, pc.ShadowRate); err != nil {
			return fmt.Errorf("write prop multi color shadow rate failed: %w", err)
		}
		if err := binaryio.WriteInt32(w, pc.ShadowHue); err != nil {
			return fmt.Errorf("write prop multi color shadow hue failed: %w", err)
		}
		if err := binaryio.WriteInt32(w, pc.ShadowChroma); err != nil {
			return fmt.Errorf("write prop multi color shadow chroma failed: %w", err)
		}
		if err := binaryio.WriteInt32(w, pc.ShadowBrightness); err != nil {
			return fmt.Errorf("write prop multi color shadow brightness failed: %w", err)
		}
		if err := binaryio.WriteInt32(w, pc.ShadowContrast); err != nil {
			return fmt.Errorf("write prop multi color shadow contrast failed: %w", err)
		}
	}
	// 结尾 "MAX"
	if err := binaryio.WriteString(w, "MAX"); err != nil {
		return fmt.Errorf("write prop multi color max failed: %w", err)
	}
	return nil
}

// 写入身体块：Maid.SerializeBody（仅头+版本）
func dumpBodyProperty(w io.Writer, bp *BodyProperty) error {
	if bp == nil {
		return fmt.Errorf("write Body failed: BodyProperty is nil")
	}

	if err := binaryio.WriteString(w, bp.Signature); err != nil {
		return fmt.Errorf("write prop body property signature failed: %w", err)
	}
	if err := binaryio.WriteInt32(w, bp.Version); err != nil {
		return fmt.Errorf("write prop body property version failed: %w", err)
	}
	return nil
}
