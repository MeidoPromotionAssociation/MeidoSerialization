package KCES

import "fmt"

// .pmatassets
// KCES 优先级材质资源容器，在 .aba 的 TextAsset 中保存 Parts.PriorityMaterial 数组
// 载荷使用 LZ4 Block Array 压缩的 MessagePack indexed-array，当前 PriorityMaterial 固定版本为 1000
// .pmatassets
// KCES priority-material resource container storing Parts.PriorityMaterial entries in a TextAsset inside an .aba file
// The payload is an LZ4 Block Array-compressed MessagePack indexed array, with current PriorityMaterial fixed version 1000

// PriorityMaterial 表示优先级材质数据
// 对应 C# Parts.PriorityMaterial，继承自 AMessagePackSerializationVersionControlIntKey
// MessagePack indexed-array 布局如下
//
//	[Key(0)] version      int     固定 1000 (FixVersion)
//	[Key(1)] id           uint64  材质 ID（文件名的 FNV hash）
//	[Key(2)] fileName     string  材质文件名（含 .pmat 扩展名）
//	[Key(3)] renderQueue  float32 渲染队列值
//	[Key(4)] targetId     uint64  目标材质 ID
//
// PriorityMaterial represents KCES Parts.PriorityMaterial data
// It matches C# Parts.PriorityMaterial derived from AMessagePackSerializationVersionControlIntKey
// Its MessagePack indexed-array layout is as follows
//
//	[Key(0)] version      int     fixed at 1000 by FixVersion
//	[Key(1)] id           uint64  material ID using the filename FNV hash
//	[Key(2)] fileName     string  material filename including the .pmat extension
//	[Key(3)] renderQueue  float32 render queue value
//	[Key(4)] targetId     uint64  target material ID
type PriorityMaterial struct {
	_struct                struct{}    `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"` // 索引对象的线格式元数据 / Indexed-object wire metadata
	Version                int         `json:"version"`     // 版本号，固定为 1000 / Version value, fixed to 1000
	ID                     uint64      `json:"id"`          // 材质 ID，通常为 fileName 去扩展名后小写的 FNV hash / Material ID, usually lowercase extensionless fileName FNV hash
	FileName               string      `json:"fileName"`    // 材质文件名，如 xxx.pmat / Material file name such as xxx.pmat
	RenderQueue            float32     `json:"renderQueue"` // 渲染队列值，控制渲染顺序 / Render queue value controlling draw order
	TargetID               uint64      `json:"targetId"`    // 目标材质 ID，指向被覆盖的材质 / Target material ID pointing to the overridden material
}

// PriorityMaterialAssets 表示优先级材质资源容器
// 对应 C# Parts.PriorityMaterialAssets，继承自 SerializPartsAssets<PriorityMaterial>
// MessagePack indexed-array 依次保存容器文件名和 PriorityMaterial 数组
// 数据存储在 .aba TextAsset 的 m_Script 中并使用 Lz4BlockArray 压缩
// PriorityMaterialAssets represents a priority-material asset container
// It matches C# Parts.PriorityMaterialAssets derived from SerializPartsAssets<PriorityMaterial>
// Its MessagePack indexed array stores the container filename followed by the PriorityMaterial array
// The data is stored in TextAsset m_Script inside .aba and compressed with Lz4BlockArray
type PriorityMaterialAssets struct {
	_struct                struct{}           `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`        // 索引对象的线格式元数据 / Indexed-object wire metadata
	FileName               string             `json:"fileName"`                         // 容器文件名，如 xxx.pmatassets / Container file name such as xxx.pmatassets
	Assets                 []PriorityMaterial `json:"assetArray"`                       // 优先级材质数组 / Priority-material array
	RootNil                bool               `codec:"-" json:"rootNil,omitempty"`      // 根 MessagePack 值是否为 nil / Whether the root MessagePack value was nil
	TrailingData           []byte             `codec:"-" json:"trailingData,omitempty"` // 根 MessagePack 值之后游戏未读取的字节 / Bytes left unread after the root MessagePack value
}

// getMessagePackTrailing 返回根 MessagePack 值后的保留字节
// getMessagePackTrailing returns the preserved bytes after the root MessagePack value
func (a *PriorityMaterialAssets) getMessagePackTrailing() []byte { return a.TrailingData }

// setMessagePackTrailing 设置根 MessagePack 值后的保留字节
// setMessagePackTrailing sets the preserved bytes after the root MessagePack value
func (a *PriorityMaterialAssets) setMessagePackTrailing(data []byte) { a.TrailingData = data }

// getMessagePackRootNil 返回根 MessagePack 值是否为 nil
// getMessagePackRootNil reports whether the root MessagePack value was nil
func (a *PriorityMaterialAssets) getMessagePackRootNil() bool { return a.RootNil }

// setMessagePackRootNil 设置根 MessagePack 值的 nil 标记
// setMessagePackRootNil sets the nil marker for the root MessagePack value
func (a *PriorityMaterialAssets) setMessagePackRootNil(value bool) { a.RootNil = value }

const priorityMaterialFixVersion = 1000

// DecodePriorityMaterial 从 MessagePack indexed array 解码单个 PriorityMaterial
// DecodePriorityMaterial decodes one PriorityMaterial from a MessagePack indexed array
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
// 此兼容辅助函数没有 error 返回值，因此无法报告超出 Int32 的 Version，需检查游戏线格式时应使用 EncodePriorityMaterialAssets
// EncodePriorityMaterial encodes PriorityMaterial as a MessagePack indexed array
// This compatibility helper has no error return and cannot report an out-of-Int32 Version, so use EncodePriorityMaterialAssets for checked game wire
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

// DecodePriorityMaterialAssets 从 Lz4BlockArray 压缩的 MessagePack 数据解码 PriorityMaterialAssets
// data 应为 TextAsset m_Script 的原始字节
// DecodePriorityMaterialAssets decodes PriorityMaterialAssets from Lz4BlockArray-compressed MessagePack data
// data should contain the raw TextAsset m_Script bytes
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

// EncodePriorityMaterialAssets 将 PriorityMaterialAssets 编码为 Lz4BlockArray 压缩的 MessagePack 数据
// EncodePriorityMaterialAssets encodes PriorityMaterialAssets as Lz4BlockArray-compressed MessagePack data
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

// NewPriorityMaterial 创建使用当前固定版本的新优先级材质
// NewPriorityMaterial creates a new priority material with the current fixed version
func NewPriorityMaterial() *PriorityMaterial {
	return &PriorityMaterial{Version: priorityMaterialFixVersion}
}
