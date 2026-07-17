package KCES

import "fmt"

// PriorityMaterial 表示优先级材质数据 / PriorityMaterial represents KCES Parts.PriorityMaterial data
// 对应 C# Parts.PriorityMaterial，继承自 AMessagePackSerializationVersionControlIntKey / Matches C# Parts.PriorityMaterial, derived from AMessagePackSerializationVersionControlIntKey
// MessagePack indexed array 布局 / MessagePack indexed-array layout:
//
//	[Key(0)] version      int     固定 1000 (FixVersion)
//	[Key(1)] id           uint64  材质 ID（文件名的 FNV hash）
//	[Key(2)] fileName     string  材质文件名（含 .pmat 扩展名）
//	[Key(3)] renderQueue  float32 渲染队列值
//	[Key(4)] targetId     uint64  目标材质 ID
type PriorityMaterial struct {
	_struct                struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`
	Version                int     `json:"version"`     // 版本号，固定为 1000 / Version value, fixed to 1000
	ID                     uint64  `json:"id"`          // 材质 ID，通常为 fileName 去扩展名后小写的 FNV hash / Material ID, usually lowercase extensionless fileName FNV hash
	FileName               string  `json:"fileName"`    // 材质文件名，如 xxx.pmat / Material file name such as xxx.pmat
	RenderQueue            float32 `json:"renderQueue"` // 渲染队列值，控制渲染顺序 / Render queue value controlling draw order
	TargetID               uint64  `json:"targetId"`    // 目标材质 ID，指向被覆盖的材质 / Target material ID pointing to the overridden material
}

// PriorityMaterialAssets 表示优先级材质资源容器 / PriorityMaterialAssets represents a priority-material asset container
// 对应 C# Parts.PriorityMaterialAssets，继承自 SerializPartsAssets<PriorityMaterial> / Matches C# Parts.PriorityMaterialAssets derived from SerializPartsAssets<PriorityMaterial>
// MessagePack indexed array 布局 / MessagePack indexed-array layout:
//
//	[Key(0)] fileName    string               容器文件名
//	[Key(1)] assetArray  PriorityMaterial[]    材质数组
//
// 存储在 .aba TextAsset 的 m_Script 中，使用 Lz4Block 压缩 / Stored in TextAsset m_Script inside .aba, compressed with Lz4Block
type PriorityMaterialAssets struct {
	_struct                struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`
	FileName               string             `json:"fileName"`                         // 容器文件名，如 xxx.pmatassets / Container file name such as xxx.pmatassets
	Assets                 []PriorityMaterial `json:"assetArray"`                       // 优先级材质数组 / Priority-material array
	RootNil                bool               `codec:"-" json:"rootNil,omitempty"`      // 根 MessagePack 值是否为 nil / Whether the root MessagePack value was nil
	TrailingData           []byte             `codec:"-" json:"trailingData,omitempty"` // 根 MessagePack 值之后游戏未读取的字节 / Bytes left unread after the root MessagePack value
}

func (a *PriorityMaterialAssets) getMessagePackTrailing() []byte     { return a.TrailingData }
func (a *PriorityMaterialAssets) setMessagePackTrailing(data []byte) { a.TrailingData = data }
func (a *PriorityMaterialAssets) getMessagePackRootNil() bool        { return a.RootNil }
func (a *PriorityMaterialAssets) setMessagePackRootNil(value bool)   { a.RootNil = value }

const priorityMaterialFixVersion = 1000

// DecodePriorityMaterial 从 MessagePack indexed array 解码单个 PriorityMaterial
func DecodePriorityMaterial(arr []interface{}) (*PriorityMaterial, error) {
	pm := &PriorityMaterial{}
	if err := decodeRawMsgpackArray(arr, pm, "PriorityMaterial"); err != nil {
		return nil, err
	}
	if err := validateGameInt32Fields(pm); err != nil {
		return nil, fmt.Errorf("decode PriorityMaterial integer field: %w", err)
	}
	return pm, nil
}

// EncodePriorityMaterial 将 PriorityMaterial 编码为 MessagePack indexed array
// This compatibility helper has no error return and therefore cannot report an
// out-of-Int32 Version. Use EncodePriorityMaterialAssets for checked game wire.
func EncodePriorityMaterial(pm *PriorityMaterial) []interface{} {
	if pm == nil {
		return nil
	}
	return []interface{}{
		int64(pm.Version),
		pm.ID,
		pm.FileName,
		float64(pm.RenderQueue),
		pm.TargetID,
	}
}

// DecodePriorityMaterialAssets 从 Lz4Block 压缩的 MessagePack 数据解码 PriorityMaterialAssets / DecodePriorityMaterialAssets decodes PriorityMaterialAssets from Lz4Block-compressed MessagePack data
// data 应为 TextAsset m_Script 的原始字节 / data should be raw TextAsset m_Script bytes
func DecodePriorityMaterialAssets(data []byte) (*PriorityMaterialAssets, error) {
	assets := &PriorityMaterialAssets{}
	if err := decodeCompressedMsgpack(data, assets, "PriorityMaterialAssets"); err != nil {
		return nil, err
	}
	if err := validateGameInt32Fields(assets); err != nil {
		return nil, fmt.Errorf("decode PriorityMaterialAssets integer field: %w", err)
	}
	return assets, nil
}

// EncodePriorityMaterialAssets 将 PriorityMaterialAssets 编码为 Lz4Block 压缩的 MessagePack 数据
func EncodePriorityMaterialAssets(assets *PriorityMaterialAssets) ([]byte, error) {
	if assets == nil {
		return nil, fmt.Errorf("PriorityMaterialAssets is nil")
	}
	normalized := *assets
	normalized.Assets = cloneSlicePreserveNil(assets.Assets)
	if err := validateGameInt32Fields(&normalized); err != nil {
		return nil, fmt.Errorf("encode PriorityMaterialAssets integer field: %w", err)
	}
	return encodeCompressedMsgpack(&normalized, "PriorityMaterialAssets")
}

func NewPriorityMaterial() *PriorityMaterial {
	return &PriorityMaterial{Version: priorityMaterialFixVersion}
}
