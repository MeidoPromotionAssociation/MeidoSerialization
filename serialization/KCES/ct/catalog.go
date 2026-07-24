package ct

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/internal/strictjson"
	"github.com/ugorji/go/codec"
)

// CatalogKind 标识 KCES 使用的两种具体 catalog 布局
// CatalogKind identifies the two concrete catalog layouts used by KCES
type CatalogKind string

const (
	CatalogKindAssetBundle  CatalogKind = "assetBundle"
	CatalogKindVirtualAsset CatalogKind = "virtualAsset"
)

// AssetBundleCatalog 表示游戏的 AssetBundleCatalog 或 VirtualAssetCatalog
// Kind 明确选择固定十二槽 AssetBundle 布局或固定十槽 VirtualAsset 布局
// AssetBundleCatalog represents the game's AssetBundleCatalog or VirtualAssetCatalog
// Kind explicitly selects the fixed twelve-slot AssetBundle layout or fixed ten-slot VirtualAsset layout
type AssetBundleCatalog struct {
	Kind              CatalogKind           `json:"kind"`                        // 具体 catalog 类型 / Concrete catalog type
	Version           int32                 `json:"version"`                     // 序列化版本 / Serialization version
	CatalogType       CatalogType           `json:"catalogType"`                 // 资源分类标志位 / Resource-category flags
	PackageType       CatalogPackageType    `json:"packageType"`                 // catalog 包类型 / Catalog package type
	Priority          int32                 `json:"priority"`                    // catalog 排序优先级 / Catalog ordering priority
	Name              *string               `json:"name"`                        // catalog 名称 / Catalog name
	SubName           *string               `json:"subName"`                     // catalog 子名称 / Catalog sub-name
	Hash              uint64                `json:"hash"`                        // catalog 哈希 / Catalog hash
	CreateTime        int64                 `json:"createTime"`                  // 创建时间 / Creation time
	IsEncrypted       bool                  `json:"isEncrypted,omitempty"`       // AssetBundle 是否加密 / Whether AssetBundles are encrypted
	ResourceFileNames []*string             `json:"resourceFileNames,omitempty"` // AssetBundle 资源文件名 / AssetBundle resource file names
	ExtensionList     []*string             `json:"extensionList"`               // 扩展名虚拟文件列表 / Extension-name virtual-file list
	Items             []*CatalogItem        `json:"items,omitempty"`             // AssetBundle catalog 条目 / AssetBundle catalog items
	VirtualItems      []*VirtualCatalogItem `json:"virtualItems,omitempty"`      // VirtualAsset catalog 条目 / VirtualAsset catalog items
}

// assetBundleCatalogJSON 以原始 JSON 值区分 catalog union 分支字段缺失与显式 null / assetBundleCatalogJSON distinguishes missing catalog union fields from explicit null by retaining each branch-specific JSON value
type assetBundleCatalogJSON struct {
	Kind              CatalogKind        `json:"kind"`                        // 具体 catalog 类型 / Concrete catalog type
	Version           int32              `json:"version"`                     // 序列化版本 / Serialization version
	CatalogType       CatalogType        `json:"catalogType"`                 // 资源分类标志位 / Resource-category flags
	PackageType       CatalogPackageType `json:"packageType"`                 // catalog 包类型 / Catalog package type
	Priority          int32              `json:"priority"`                    // catalog 排序优先级 / Catalog ordering priority
	Name              *string            `json:"name"`                        // catalog 名称 / Catalog name
	SubName           *string            `json:"subName"`                     // catalog 子名称 / Catalog sub-name
	Hash              uint64             `json:"hash"`                        // catalog 哈希 / Catalog hash
	CreateTime        int64              `json:"createTime"`                  // 创建时间 / Creation time
	ExtensionList     []*string          `json:"extensionList"`               // 扩展名虚拟文件列表 / Extension-name virtual-file list
	IsEncrypted       json.RawMessage    `json:"isEncrypted,omitempty"`       // AssetBundle 加密分支字段 / AssetBundle encryption branch field
	ResourceFileNames json.RawMessage    `json:"resourceFileNames,omitempty"` // AssetBundle 资源文件名分支字段 / AssetBundle resource-name branch field
	Items             json.RawMessage    `json:"items,omitempty"`             // AssetBundle 条目分支字段 / AssetBundle item branch field
	VirtualItems      json.RawMessage    `json:"virtualItems,omitempty"`      // VirtualAsset 条目分支字段 / VirtualAsset item branch field
}

// MarshalJSON 仅写出 Kind 对应 catalog 布局的真实字段并让活动可空切片显式成为 JSON null
// MarshalJSON emits only the real fields of the catalog layout selected by Kind and represents active nullable slices as explicit JSON null
func (cat AssetBundleCatalog) MarshalJSON() ([]byte, error) {
	if err := ValidateCatalog(&cat); err != nil {
		return nil, err
	}
	raw := assetBundleCatalogJSON{
		Kind:          cat.Kind,
		Version:       cat.Version,
		CatalogType:   cat.CatalogType,
		PackageType:   cat.PackageType,
		Priority:      cat.Priority,
		Name:          cat.Name,
		SubName:       cat.SubName,
		Hash:          cat.Hash,
		CreateTime:    cat.CreateTime,
		ExtensionList: cat.ExtensionList,
	}
	var err error
	switch cat.Kind {
	case CatalogKindAssetBundle:
		raw.IsEncrypted, err = json.Marshal(cat.IsEncrypted)
		if err == nil {
			raw.ResourceFileNames, err = json.Marshal(cat.ResourceFileNames)
		}
		if err == nil {
			raw.Items, err = json.Marshal(cat.Items)
		}
	case CatalogKindVirtualAsset:
		raw.VirtualItems, err = json.Marshal(cat.VirtualItems)
	default:
		return nil, fmt.Errorf("unsupported catalog kind %q", cat.Kind)
	}
	if err != nil {
		return nil, fmt.Errorf("marshal %s catalog branch: %w", cat.Kind, err)
	}
	return json.Marshal(raw)
}

// UnmarshalJSON 严格解码 catalog union，要求当前布局的全部分支字段出现并拒绝另一布局的任何字段
// UnmarshalJSON strictly decodes the catalog union, requires every branch field of the selected layout, and rejects every field from the other layout
func (cat *AssetBundleCatalog) UnmarshalJSON(data []byte) error {
	var raw assetBundleCatalogJSON
	if err := decodeCatalogJSONStrict(data, &raw); err != nil {
		return err
	}
	if err := strictjson.RequireObjectFields(data, "catalog", "kind", "version", "catalogType", "packageType", "priority", "name", "subName", "hash", "createTime", "extensionList"); err != nil {
		return err
	}
	if err := validateCatalogJSONRootPresence(&raw); err != nil {
		return err
	}
	value := AssetBundleCatalog{
		Kind:          raw.Kind,
		Version:       raw.Version,
		CatalogType:   raw.CatalogType,
		PackageType:   raw.PackageType,
		Priority:      raw.Priority,
		Name:          raw.Name,
		SubName:       raw.SubName,
		Hash:          raw.Hash,
		CreateTime:    raw.CreateTime,
		ExtensionList: raw.ExtensionList,
	}
	var err error
	switch raw.Kind {
	case CatalogKindAssetBundle:
		if err = decodeCatalogJSONStrict(raw.IsEncrypted, &value.IsEncrypted); err == nil {
			err = decodeCatalogJSONStrict(raw.ResourceFileNames, &value.ResourceFileNames)
		}
		if err == nil {
			err = decodeCatalogJSONStrict(raw.Items, &value.Items)
		}
	case CatalogKindVirtualAsset:
		err = decodeCatalogJSONStrict(raw.VirtualItems, &value.VirtualItems)
	default:
		return fmt.Errorf("unsupported catalog kind %q", raw.Kind)
	}
	if err != nil {
		return fmt.Errorf("decode %s catalog branch: %w", raw.Kind, err)
	}
	if err := ValidateCatalog(&value); err != nil {
		return err
	}
	*cat = value
	return nil
}

// validateCatalogJSONRootPresence 检查活动 catalog 分支字段完整存在并拒绝非活动字段，包括显式 null 或 false
// validateCatalogJSONRootPresence requires every active catalog branch field and rejects inactive fields including explicit null or false
func validateCatalogJSONRootPresence(raw *assetBundleCatalogJSON) error {
	require := func(name string, data json.RawMessage) error {
		if len(data) == 0 {
			return fmt.Errorf("catalog kind %q requires field %s", raw.Kind, name)
		}
		return nil
	}
	reject := func(name string, data json.RawMessage) error {
		if len(data) != 0 {
			return fmt.Errorf("%s is inactive for catalog kind %q", name, raw.Kind)
		}
		return nil
	}
	switch raw.Kind {
	case CatalogKindAssetBundle:
		for _, field := range []struct {
			name string
			data json.RawMessage
		}{
			{name: "isEncrypted", data: raw.IsEncrypted},
			{name: "resourceFileNames", data: raw.ResourceFileNames},
			{name: "items", data: raw.Items},
		} {
			if err := require(field.name, field.data); err != nil {
				return err
			}
		}
		return reject("virtualItems", raw.VirtualItems)
	case CatalogKindVirtualAsset:
		if err := require("virtualItems", raw.VirtualItems); err != nil {
			return err
		}
		for _, field := range []struct {
			name string
			data json.RawMessage
		}{
			{name: "isEncrypted", data: raw.IsEncrypted},
			{name: "resourceFileNames", data: raw.ResourceFileNames},
			{name: "items", data: raw.Items},
		} {
			if err := reject(field.name, field.data); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported catalog kind %q", raw.Kind)
	}
}

// decodeCatalogJSONStrict 解码单个完整 JSON 值并递归拒绝结构体未知字段
// decodeCatalogJSONStrict decodes one complete JSON value and recursively rejects unknown struct fields
func decodeCatalogJSONStrict(data []byte, out any) error {
	return strictjson.Decode(data, out)
}

// CatalogItem 表示 C# AssetBundleCatalog.Item 的固定三槽资源索引条目
// CatalogItem represents the fixed three-slot resource index entry from C# AssetBundleCatalog.Item
type CatalogItem struct {
	_struct       struct{} `codec:",toarray"`     // 强制按数组编码 / Forces array encoding
	ResourceIndex int32    `json:"resourceIndex"` // ResourceFileNames 索引 / Index into ResourceFileNames
	Name          *string  `json:"name"`          // 资源名称 / Resource name
	Hash          uint64   `json:"hash"`          // 资源哈希 / Resource hash
}

// UnmarshalJSON 严格解码 AssetBundle catalog 条目并要求三个字段显式出现
// UnmarshalJSON strictly decodes an AssetBundle catalog item and requires all three fields to be explicitly present
func (value *CatalogItem) UnmarshalJSON(data []byte) error {
	if value == nil {
		return fmt.Errorf("nil catalog item JSON target")
	}
	type plainCatalogItem CatalogItem
	var decoded plainCatalogItem
	if err := decodeCatalogJSONStrict(data, &decoded); err != nil {
		return err
	}
	if err := strictjson.RequireObjectFields(data, "catalog item", "resourceIndex", "name", "hash"); err != nil {
		return err
	}
	*value = CatalogItem(decoded)
	return nil
}

// CodecEncodeSelf 按固定三槽布局编码 CatalogItem
// CodecEncodeSelf encodes CatalogItem using its fixed three-slot layout
func (v CatalogItem) CodecEncodeSelf(e *codec.Encoder) {
	EncodeIndexedObjectSelf(e, &v)
}

// CodecDecodeSelf 按固定三槽布局解码 CatalogItem
// CodecDecodeSelf decodes CatalogItem using its fixed three-slot layout
func (v *CatalogItem) CodecDecodeSelf(d *codec.Decoder) {
	DecodeIndexedObjectSelf(d, v)
}

// VirtualCatalogItem 表示 C# VirtualAssetCatalog.Item 的固定三槽本地资源条目
// VirtualCatalogItem represents the fixed three-slot local-resource entry from C# VirtualAssetCatalog.Item
type VirtualCatalogItem struct {
	_struct   struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	AssetPath *string  `json:"assetPath"` // Unity 工程资源路径 / Unity project asset path
	Name      *string  `json:"name"`      // 资源名称 / Resource name
	Hash      uint64   `json:"hash"`      // 资源哈希 / Resource hash
}

// UnmarshalJSON 严格解码 VirtualAsset catalog 条目并要求三个字段显式出现
// UnmarshalJSON strictly decodes a VirtualAsset catalog item and requires all three fields to be explicitly present
func (value *VirtualCatalogItem) UnmarshalJSON(data []byte) error {
	if value == nil {
		return fmt.Errorf("nil virtual catalog item JSON target")
	}
	type plainVirtualCatalogItem VirtualCatalogItem
	var decoded plainVirtualCatalogItem
	if err := decodeCatalogJSONStrict(data, &decoded); err != nil {
		return err
	}
	if err := strictjson.RequireObjectFields(data, "virtual catalog item", "assetPath", "name", "hash"); err != nil {
		return err
	}
	*value = VirtualCatalogItem(decoded)
	return nil
}

// CodecEncodeSelf 按固定三槽布局编码 VirtualCatalogItem
// CodecEncodeSelf encodes VirtualCatalogItem using its fixed three-slot layout
func (v VirtualCatalogItem) CodecEncodeSelf(e *codec.Encoder) {
	EncodeIndexedObjectSelf(e, &v)
}

// CodecDecodeSelf 按固定三槽布局解码 VirtualCatalogItem
// CodecDecodeSelf decodes VirtualCatalogItem using its fixed three-slot layout
func (v *VirtualCatalogItem) CodecDecodeSelf(d *codec.Decoder) {
	DecodeIndexedObjectSelf(d, v)
}

// ExtensionNameList 表示 .ct 中按扩展名分组的资源名称列表
// ExtensionNameList represents resource names grouped by extension in a .ct file
type ExtensionNameList struct {
	_struct   struct{}             `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	Extension *string              `json:"extention"` // 游戏字段名拼写为 extention，用途未知 / The game field is spelled extention and its purpose is unknown
	Data      []*ExtensionNamePack `json:"data"`      // 名称和哈希条目 / Name and hash entries
}

// UnmarshalJSON 严格解码 ExtensionNameList 并要求扩展名与数据字段显式出现
// UnmarshalJSON strictly decodes an ExtensionNameList and requires the extension and data fields to be explicitly present
func (value *ExtensionNameList) UnmarshalJSON(data []byte) error {
	if value == nil {
		return fmt.Errorf("nil ExtensionNameList JSON target")
	}
	type plainExtensionNameList ExtensionNameList
	var decoded plainExtensionNameList
	if err := decodeCatalogJSONStrict(data, &decoded); err != nil {
		return err
	}
	if err := strictjson.RequireObjectFields(data, "ExtensionNameList", "extention", "data"); err != nil {
		return err
	}
	*value = ExtensionNameList(decoded)
	return nil
}

// CodecEncodeSelf 按固定两槽布局编码 ExtensionNameList
// CodecEncodeSelf encodes ExtensionNameList using its fixed two-slot layout
func (v ExtensionNameList) CodecEncodeSelf(e *codec.Encoder) {
	EncodeIndexedObjectSelf(e, &v)
}

// CodecDecodeSelf 按固定两槽布局解码 ExtensionNameList
// CodecDecodeSelf decodes ExtensionNameList using its fixed two-slot layout
func (v *ExtensionNameList) CodecDecodeSelf(d *codec.Decoder) {
	DecodeIndexedObjectSelf(d, v)
}

// ExtensionNamePack 表示 ExtensionNameList 中固定两槽的名称和哈希条目
// ExtensionNamePack represents one fixed two-slot name and hash entry in ExtensionNameList
type ExtensionNamePack struct {
	_struct struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	Name    *string  `json:"name"`      // 资源名称 / Resource name
	Hash    uint64   `json:"hash"`      // 游戏字段用途未知 / The game field purpose is unknown
}

// UnmarshalJSON 严格解码 ExtensionNamePack 并要求名称与哈希字段显式出现
// UnmarshalJSON strictly decodes an ExtensionNamePack and requires the name and hash fields to be explicitly present
func (value *ExtensionNamePack) UnmarshalJSON(data []byte) error {
	if value == nil {
		return fmt.Errorf("nil ExtensionNamePack JSON target")
	}
	type plainExtensionNamePack ExtensionNamePack
	var decoded plainExtensionNamePack
	if err := decodeCatalogJSONStrict(data, &decoded); err != nil {
		return err
	}
	if err := strictjson.RequireObjectFields(data, "ExtensionNameList.data[]", "name", "hash"); err != nil {
		return err
	}
	*value = ExtensionNamePack(decoded)
	return nil
}

// CodecEncodeSelf 按固定两槽布局编码 ExtensionNamePack
// CodecEncodeSelf encodes ExtensionNamePack using its fixed two-slot layout
func (v ExtensionNamePack) CodecEncodeSelf(e *codec.Encoder) {
	EncodeIndexedObjectSelf(e, &v)
}

// CodecDecodeSelf 按固定两槽布局解码 ExtensionNamePack
// CodecDecodeSelf decodes ExtensionNamePack using its fixed two-slot layout
func (v *ExtensionNamePack) CodecDecodeSelf(d *codec.Decoder) {
	DecodeIndexedObjectSelf(d, v)
}

// CatalogType 是游戏资源分类标志位枚举
// CatalogType is the game's resource-category flag enumeration
type CatalogType int32

const (
	CatalogTypeUnknown   CatalogType = 1
	CatalogTypeLanguage  CatalogType = 2
	CatalogTypeProduct   CatalogType = 4
	CatalogTypeMovie     CatalogType = 8
	CatalogTypeScript    CatalogType = 16
	CatalogTypeSound     CatalogType = 32
	CatalogTypeVoice     CatalogType = 64
	CatalogTypeCsv       CatalogType = 128
	CatalogTypeSystem    CatalogType = 256
	CatalogTypeBg        CatalogType = 512
	CatalogTypeMotion    CatalogType = 1024
	CatalogTypePartsMeta CatalogType = 2048
	CatalogTypeParts     CatalogType = 4096
)

// CatalogPackageType 是游戏 catalog 包类型枚举
// CatalogPackageType is the game's catalog package-type enumeration
type CatalogPackageType int32

const (
	PackageTypeBase        CatalogPackageType = 0
	PackageTypePlugin      CatalogPackageType = 1
	PackageTypePluginPatch CatalogPackageType = 2
	PackageTypeBasePatch   CatalogPackageType = 3
	PackageTypeExtraBase   CatalogPackageType = 4
	PackageTypeExtraPatch  CatalogPackageType = 5
)

// assetBundleCatalogWire 表示游戏 AssetBundleCatalog 的固定十二槽线格式 / assetBundleCatalogWire represents the game's fixed twelve-slot AssetBundleCatalog wire layout
type assetBundleCatalogWire struct {
	_struct           struct{}           `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	Version           int32              // 序列化版本 / Serialization version
	CatalogType       CatalogType        // 资源分类标志位 / Resource-category flags
	PackageType       CatalogPackageType // catalog 包类型 / Catalog package type
	Priority          int32              // catalog 排序优先级 / Catalog ordering priority
	Name              *string            // catalog 名称 / Catalog name
	SubName           *string            // catalog 子名称 / Catalog sub-name
	Hash              uint64             // catalog 哈希 / Catalog hash
	CreateTime        int64              // 创建时间 / Creation time
	IsEncrypted       bool               // AssetBundle 是否加密 / Whether AssetBundles are encrypted
	ResourceFileNames []*string          // AssetBundle 资源文件名 / AssetBundle resource file names
	ExtensionList     []*string          // 扩展名虚拟文件列表 / Extension-name virtual-file list
	Items             []*CatalogItem     // AssetBundle catalog 条目 / AssetBundle catalog items
}

// CodecEncodeSelf 按固定十二槽布局编码 AssetBundleCatalog 线格式
// CodecEncodeSelf encodes the AssetBundleCatalog wire value using its fixed twelve-slot layout
func (v assetBundleCatalogWire) CodecEncodeSelf(e *codec.Encoder) {
	EncodeIndexedObjectSelf(e, &v)
}

// CodecDecodeSelf 按固定十二槽布局解码 AssetBundleCatalog 线格式
// CodecDecodeSelf decodes the AssetBundleCatalog wire value using its fixed twelve-slot layout
func (v *assetBundleCatalogWire) CodecDecodeSelf(d *codec.Decoder) {
	DecodeIndexedObjectSelf(d, v)
}

// virtualAssetCatalogWire 表示游戏 VirtualAssetCatalog 的固定十槽线格式 / virtualAssetCatalogWire represents the game's fixed ten-slot VirtualAssetCatalog wire layout
type virtualAssetCatalogWire struct {
	_struct       struct{}              `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	Version       int32                 // 序列化版本 / Serialization version
	CatalogType   CatalogType           // 资源分类标志位 / Resource-category flags
	PackageType   CatalogPackageType    // catalog 包类型 / Catalog package type
	Priority      int32                 // catalog 排序优先级 / Catalog ordering priority
	Name          *string               // catalog 名称 / Catalog name
	SubName       *string               // catalog 子名称 / Catalog sub-name
	Hash          uint64                // catalog 哈希 / Catalog hash
	CreateTime    int64                 // 创建时间 / Creation time
	ExtensionList []*string             // 扩展名虚拟文件列表 / Extension-name virtual-file list
	Items         []*VirtualCatalogItem // VirtualAsset catalog 条目 / VirtualAsset catalog items
}

// CodecEncodeSelf 按固定十槽布局编码 VirtualAssetCatalog 线格式
// CodecEncodeSelf encodes the VirtualAssetCatalog wire value using its fixed ten-slot layout
func (v virtualAssetCatalogWire) CodecEncodeSelf(e *codec.Encoder) {
	EncodeIndexedObjectSelf(e, &v)
}

// CodecDecodeSelf 按固定十槽布局解码 VirtualAssetCatalog 线格式
// CodecDecodeSelf decodes the VirtualAssetCatalog wire value using its fixed ten-slot layout
func (v *virtualAssetCatalogWire) CodecDecodeSelf(d *codec.Decoder) {
	DecodeIndexedObjectSelf(d, v)
}

// DecodeCatalog 解码唯一完整的 catalog 根值并按固定宽度识别具体类型
// DecodeCatalog decodes the sole complete catalog root and identifies its concrete type by fixed width
func DecodeCatalog(data []byte) (*AssetBundleCatalog, error) {
	return decodeCatalog(data, "")
}

// DecodeCatalogWithKind 按调用方指定的具体 catalog 类型解码并验证固定宽度
// DecodeCatalogWithKind decodes the caller-selected concrete catalog type and validates its fixed width
func DecodeCatalogWithKind(data []byte, kind CatalogKind) (*AssetBundleCatalog, error) {
	if err := validateCatalogKind(kind); err != nil {
		return nil, err
	}
	return decodeCatalog(data, kind)
}

// decodeCatalog 解码 catalog 根值并拒绝未知宽度和尾部数据
// decodeCatalog decodes a catalog root and rejects unknown widths and trailing data
func decodeCatalog(data []byte, forcedKind CatalogKind) (*AssetBundleCatalog, error) {
	if len(data) > 0 && data[0] == 0xc0 {
		if len(data) != 1 {
			return nil, fmt.Errorf("trailing data after catalog root: %d bytes", len(data)-1)
		}
		return nil, nil
	}
	fields, err := decodeRawMsgpackArray(data, "catalog")
	if err != nil {
		return nil, err
	}
	kind := forcedKind
	if kind == "" {
		switch len(fields) {
		case 12:
			kind = CatalogKindAssetBundle
		case 10:
			kind = CatalogKindVirtualAsset
		default:
			return nil, fmt.Errorf("unsupported catalog indexed-array width %d, expected 12 or 10", len(fields))
		}
	}
	switch kind {
	case CatalogKindAssetBundle:
		if len(fields) != 12 {
			return nil, fmt.Errorf("unsupported AssetBundleCatalog indexed-array width %d, expected 12", len(fields))
		}
		var wire assetBundleCatalogWire
		if err := DecodeMsgpack(data, &wire); err != nil {
			return nil, fmt.Errorf("decode AssetBundleCatalog: %w", err)
		}
		return &AssetBundleCatalog{
			Kind:              kind,
			Version:           wire.Version,
			CatalogType:       wire.CatalogType,
			PackageType:       wire.PackageType,
			Priority:          wire.Priority,
			Name:              wire.Name,
			SubName:           wire.SubName,
			Hash:              wire.Hash,
			CreateTime:        wire.CreateTime,
			IsEncrypted:       wire.IsEncrypted,
			ResourceFileNames: wire.ResourceFileNames,
			ExtensionList:     wire.ExtensionList,
			Items:             wire.Items,
		}, nil
	case CatalogKindVirtualAsset:
		if len(fields) != 10 {
			return nil, fmt.Errorf("unsupported VirtualAssetCatalog indexed-array width %d, expected 10", len(fields))
		}
		var wire virtualAssetCatalogWire
		if err := DecodeMsgpack(data, &wire); err != nil {
			return nil, fmt.Errorf("decode VirtualAssetCatalog: %w", err)
		}
		return &AssetBundleCatalog{
			Kind:          kind,
			Version:       wire.Version,
			CatalogType:   wire.CatalogType,
			PackageType:   wire.PackageType,
			Priority:      wire.Priority,
			Name:          wire.Name,
			SubName:       wire.SubName,
			Hash:          wire.Hash,
			CreateTime:    wire.CreateTime,
			ExtensionList: wire.ExtensionList,
			VirtualItems:  wire.Items,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported catalog kind %q", kind)
	}
}

// DecodeCatalogFromCt 从 .ct 的 catalog 虚拟文件解码具体 catalog
// DecodeCatalogFromCt decodes the concrete catalog from the catalog virtual file in a .ct container
func DecodeCatalogFromCt(table *ContentTable) (*AssetBundleCatalog, error) {
	data, err := decodeContentTableMessagePackFile(table, "catalog")
	if err != nil {
		return nil, err
	}
	return DecodeCatalog(data)
}

// DecodeCatalogFromCtWithKind 从 .ct 的 catalog 虚拟文件按指定类型解码
// DecodeCatalogFromCtWithKind decodes the caller-selected catalog type from the catalog virtual file in a .ct container
func DecodeCatalogFromCtWithKind(table *ContentTable, kind CatalogKind) (*AssetBundleCatalog, error) {
	data, err := decodeContentTableMessagePackFile(table, "catalog")
	if err != nil {
		return nil, err
	}
	return DecodeCatalogWithKind(data, kind)
}

// EncodeCatalog 按 Kind 选择固定布局并编码唯一完整的 catalog 根值
// EncodeCatalog selects the fixed layout by Kind and encodes the sole complete catalog root
func EncodeCatalog(cat *AssetBundleCatalog) ([]byte, error) {
	if cat == nil {
		return []byte{0xc0}, nil
	}
	if err := ValidateCatalog(cat); err != nil {
		return nil, err
	}
	switch cat.Kind {
	case CatalogKindAssetBundle:
		return EncodeIndexedMsgpack(&assetBundleCatalogWire{
			Version:           cat.Version,
			CatalogType:       cat.CatalogType,
			PackageType:       cat.PackageType,
			Priority:          cat.Priority,
			Name:              cat.Name,
			SubName:           cat.SubName,
			Hash:              cat.Hash,
			CreateTime:        cat.CreateTime,
			IsEncrypted:       cat.IsEncrypted,
			ResourceFileNames: cat.ResourceFileNames,
			ExtensionList:     cat.ExtensionList,
			Items:             cat.Items,
		})
	case CatalogKindVirtualAsset:
		return EncodeIndexedMsgpack(&virtualAssetCatalogWire{
			Version:       cat.Version,
			CatalogType:   cat.CatalogType,
			PackageType:   cat.PackageType,
			Priority:      cat.Priority,
			Name:          cat.Name,
			SubName:       cat.SubName,
			Hash:          cat.Hash,
			CreateTime:    cat.CreateTime,
			ExtensionList: cat.ExtensionList,
			Items:         cat.VirtualItems,
		})
	default:
		return nil, fmt.Errorf("unsupported catalog kind %q", cat.Kind)
	}
}

// DecodeExtensionNameList 解码唯一完整的固定两槽 ExtensionNameList 根值
// DecodeExtensionNameList decodes the sole complete fixed two-slot ExtensionNameList root
func DecodeExtensionNameList(data []byte) (*ExtensionNameList, error) {
	if len(data) > 0 && data[0] == 0xc0 {
		if len(data) != 1 {
			return nil, fmt.Errorf("trailing data after ExtensionNameList root: %d bytes", len(data)-1)
		}
		return nil, nil
	}
	var value ExtensionNameList
	if err := DecodeMsgpack(data, &value); err != nil {
		return nil, fmt.Errorf("decode ExtensionNameList: %w", err)
	}
	return &value, nil
}

// DecodeExtensionNameListFromCt 从 .ct 中按扩展名读取并解码 ExtensionNameList
// DecodeExtensionNameListFromCt reads and decodes an ExtensionNameList from a .ct container by extension
func DecodeExtensionNameListFromCt(table *ContentTable, extension string) (*ExtensionNameList, error) {
	data, err := decodeContentTableMessagePackFile(table, extension)
	if err != nil {
		return nil, err
	}
	return DecodeExtensionNameList(data)
}

// EncodeExtensionNameList 编码唯一完整的固定两槽 ExtensionNameList 根值
// EncodeExtensionNameList encodes the sole complete fixed two-slot ExtensionNameList root
func EncodeExtensionNameList(value *ExtensionNameList) ([]byte, error) {
	if value == nil {
		return []byte{0xc0}, nil
	}
	if err := ValidateExtensionNameList(value); err != nil {
		return nil, err
	}
	return EncodeIndexedMsgpack(value)
}

// decodeContentTableMessagePackFile 提取并解压 .ct 中一个 MessagePack 虚拟文件
// decodeContentTableMessagePackFile extracts and decompresses one MessagePack virtual file from a .ct container
func decodeContentTableMessagePackFile(table *ContentTable, name string) ([]byte, error) {
	if table == nil {
		return nil, fmt.Errorf("nil content table")
	}
	raw, err := table.GetFileData(name)
	if err != nil {
		return nil, err
	}
	data, err := DecompressLz4BlockArray(raw)
	if err != nil {
		return nil, fmt.Errorf("decompress content table file %q: %w", name, err)
	}
	return data, nil
}

// ValidateCatalog 校验 catalog 的具体类型、互斥字段和 UTF-8 字符串
// ValidateCatalog validates a catalog's concrete type, mutually exclusive fields, and UTF-8 strings
func ValidateCatalog(cat *AssetBundleCatalog) error {
	if cat == nil {
		return nil
	}
	if err := validateCatalogKind(cat.Kind); err != nil {
		return err
	}
	if err := validateOptionalUTF8String(cat.Name, "catalog.name"); err != nil {
		return err
	}
	if err := validateOptionalUTF8String(cat.SubName, "catalog.subName"); err != nil {
		return err
	}
	if err := validateOptionalUTF8Strings(cat.ExtensionList, "catalog.extensionList"); err != nil {
		return err
	}
	switch cat.Kind {
	case CatalogKindAssetBundle:
		if cat.VirtualItems != nil {
			return fmt.Errorf("assetBundle catalog cannot contain virtualItems")
		}
		if err := validateOptionalUTF8Strings(cat.ResourceFileNames, "catalog.resourceFileNames"); err != nil {
			return err
		}
		for index, item := range cat.Items {
			if item == nil {
				continue
			}
			if err := validateOptionalUTF8String(item.Name, fmt.Sprintf("catalog.items[%d].name", index)); err != nil {
				return err
			}
		}
	case CatalogKindVirtualAsset:
		if cat.IsEncrypted || cat.ResourceFileNames != nil || cat.Items != nil {
			return fmt.Errorf("virtualAsset catalog cannot contain AssetBundle-only fields")
		}
		for index, item := range cat.VirtualItems {
			if item == nil {
				continue
			}
			if err := validateOptionalUTF8String(item.AssetPath, fmt.Sprintf("catalog.virtualItems[%d].assetPath", index)); err != nil {
				return err
			}
			if err := validateOptionalUTF8String(item.Name, fmt.Sprintf("catalog.virtualItems[%d].name", index)); err != nil {
				return err
			}
		}
	}
	return nil
}

// ValidateExtensionNameList 校验 ExtensionNameList 的 UTF-8 字符串字段
// ValidateExtensionNameList validates the UTF-8 string fields of an ExtensionNameList
func ValidateExtensionNameList(value *ExtensionNameList) error {
	if value == nil {
		return nil
	}
	if err := validateOptionalUTF8String(value.Extension, "ExtensionNameList.extention"); err != nil {
		return err
	}
	for index, pack := range value.Data {
		if pack == nil {
			continue
		}
		if err := validateOptionalUTF8String(pack.Name, fmt.Sprintf("ExtensionNameList.data[%d].name", index)); err != nil {
			return err
		}
	}
	return nil
}

// validateCatalogKind 校验具体 catalog 类型
// validateCatalogKind validates a concrete catalog type
func validateCatalogKind(kind CatalogKind) error {
	switch kind {
	case CatalogKindAssetBundle, CatalogKindVirtualAsset:
		return nil
	default:
		return fmt.Errorf("unsupported catalog kind %q", kind)
	}
}

// validateOptionalUTF8String 校验可空字符串非空时是合法 UTF-8
// validateOptionalUTF8String validates that a nullable string is valid UTF-8 when non-null
func validateOptionalUTF8String(value *string, label string) error {
	if value != nil && !utf8.ValidString(*value) {
		return fmt.Errorf("%s is not valid UTF-8", label)
	}
	return nil
}

// validateOptionalUTF8Strings 校验可空字符串数组中的所有非空元素
// validateOptionalUTF8Strings validates every non-null element in a nullable string array
func validateOptionalUTF8Strings(values []*string, label string) error {
	for index, value := range values {
		if err := validateOptionalUTF8String(value, fmt.Sprintf("%s[%d]", label, index)); err != nil {
			return err
		}
	}
	return nil
}
