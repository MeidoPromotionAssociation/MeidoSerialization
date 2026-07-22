package ct

import (
	"fmt"
	"unicode/utf8"

	"github.com/ugorji/go/codec"
)

// CatalogKind 标识 KCES 使用的两种具体 catalog 布局
// 空值仅用于兼容旧 JSON，并表示历史默认的 12 槽 AssetBundleCatalog 布局
// CatalogKind identifies the two concrete catalog layouts used by KCES
// The empty value is accepted only for legacy JSON and denotes the historical default 12-slot AssetBundleCatalog layout
type CatalogKind string

const (
	CatalogKindAssetBundle  CatalogKind = "assetBundle"
	CatalogKindVirtualAsset CatalogKind = "virtualAsset"
)

// AssetBundleCatalog 是兼容既有 API 和 JSON 的 Go catalog 容器
// Kind 选择 C# AssetBundleCatalog 的 12 槽布局或 VirtualAssetCatalog 的 10 槽本地资源布局，类型名及 AssetBundle 专用字段因兼容性而保留
// 两种布局的 Key(0) 至 Key(7) 依次为 version、catalogType、packageType、priority、name、subName、hash、createTime
// AssetBundleCatalog 的 Key(8) 至 Key(11) 依次为 isEncrypted、resourceFileNames、extensionList、items
// VirtualAssetCatalog 的 Key(8) 和 Key(9) 依次为 extensionList、items
// AssetBundleCatalog is a Go catalog envelope kept compatible with existing APIs and JSON
// Kind selects either the 12-slot C# AssetBundleCatalog layout or the 10-slot local-resource VirtualAssetCatalog layout; the type name and AssetBundle-specific fields remain for compatibility
// Keys 0 through 7 in both layouts are version, catalogType, packageType, priority, name, subName, hash, and createTime
// Keys 8 through 11 in AssetBundleCatalog are isEncrypted, resourceFileNames, extensionList, and items
// Keys 8 and 9 in VirtualAssetCatalog are extensionList and items
type AssetBundleCatalog struct {
	MessagePackRootMetadata                      // 根 nil 与尾随字节元数据 / Root nil and trailing-byte metadata
	IndexedObjectMetadata                        // indexed object 宽度与未来槽位元数据 / Indexed-object width and future-slot metadata
	Kind                    CatalogKind          `json:"kind,omitempty"`                  // 具体 catalog 线格式，空值表示旧 JSON 的 AssetBundle 布局 / Concrete catalog wire layout; empty denotes the AssetBundle layout for legacy JSON
	Version                 int                  `json:"version"`                         // 当前游戏通常写 1000；解码和重编码时原样保留 / The current game normally writes 1000; decoding and re-encoding preserve the stored value
	CatalogType             CatalogType          `json:"catalogType"`                     // HasCatalogType 使用的资源分类标志位 / Resource-category flags used by HasCatalogType
	PackageType             CatalogPackageType   `json:"packageType"`                     // 用于 catalog 排序及补丁类型判断的包类型 / Package type used for catalog ordering and patch classification
	Priority                int                  `json:"priority"`                        // CatalogUtility.Compare 在类型和包类型之后比较的排序值 / Ordering value compared by CatalogUtility.Compare after catalog and package types
	Name                    string               `json:"name"`                            // ICatalog 名称，也是排序相同时的最终比较键 / ICatalog name and final comparison key when earlier ordering fields match
	NameIsNil               bool                 `json:"nameIsNil,omitempty"`             // Key(4) 在线格式中为 MessagePack nil / Key(4) was MessagePack nil on the wire
	SubName                 string               `json:"subName"`                         // PluginPatch 计算依赖 catalog 名称时使用的子名称 / Sub-name used when PluginPatch computes its prerequisite catalog name
	SubNameIsNil            bool                 `json:"subNameIsNil,omitempty"`          // Key(5) 在线格式中为 MessagePack nil / Key(5) was MessagePack nil on the wire
	Hash                    uint64               `json:"hash"`                            // ICatalog 哈希，游戏在补丁依赖筛选中使用 / ICatalog hash used by the game for patch-dependency filtering
	CreateTime              int64                `json:"createTime"`                      // 通过 ICatalog 暴露的 Int64 创建时间值 / Int64 creation-time value exposed through ICatalog
	IsEncrypted             bool                 `json:"isEncrypted"`                     // 是否在初始化资源包时进入补丁解密流程 / Whether asset bundles enter the patch-decryption path during initialization
	ResourceFileNames       []string             `json:"resourceFileNames"`               // 按 ResourceIndex 引用的资源包文件名 / Resource-bundle file names referenced by ResourceIndex
	ResourceFileNameNulls   []bool               `json:"resourceFileNameNulls,omitempty"` // resourceFileNames 中在线格式为 nil 的字符串元素 / String elements in resourceFileNames that were nil on the wire
	ExtensionList           []string             `json:"extensionList"`                   // 对应 .ct 中同名 ExtensionNameList 虚拟文件的扩展名列表 / Extension list naming corresponding ExtensionNameList virtual files in the .ct container
	ExtensionListNulls      []bool               `json:"extensionListNulls,omitempty"`    // extensionList 中在线格式为 nil 的字符串元素 / String elements in extensionList that were nil on the wire
	Items                   []CatalogItem        `json:"items"`                           // AssetBundle 条目，游戏按哈希二分查找，序列化器保留原顺序 / AssetBundle items searched by hash in the game; the serializer preserves stored order
	ItemNulls               []bool               `json:"itemNulls,omitempty"`             // items 中在线格式为 nil 的类元素 / Class elements in items that were nil on the wire
	VirtualItems            []VirtualCatalogItem `json:"virtualItems,omitempty"`          // 仅用于 VirtualAsset 布局的本地资源条目 / Local-resource items used only by the VirtualAsset layout
	VirtualItemNulls        []bool               `json:"virtualItemNulls,omitempty"`      // virtualItems 中在线格式为 nil 的类元素 / Class elements in virtualItems that were nil on the wire
}

// CatalogItem 表示 C# AssetBundleCatalog.Item 的单个资源索引条目
// MessagePack indexed array 依次保存 resourceIndex、name 和 hash
// CatalogItem represents one resource index entry from C# AssetBundleCatalog.Item
// Its MessagePack indexed array stores resourceIndex, name, and hash in that order
type CatalogItem struct {
	IndexedObjectMetadata        // indexed object 宽度与未来槽位元数据 / Indexed-object width and future-slot metadata
	ResourceIndex         int    `json:"resourceIndex"`       // 指向 AssetBundleCatalog.ResourceFileNames 的索引 / Index into AssetBundleCatalog.ResourceFileNames
	Name                  string `json:"name"`                // ResourceLocation 的主键和内部标识 / Primary key and internal identifier of the ResourceLocation
	NameIsNil             bool   `json:"nameIsNil,omitempty"` // name 在线格式中为 MessagePack nil / name was MessagePack nil on the wire
	Hash                  uint64 `json:"hash"`                // 游戏用于二分查找和统一资源索引的条目哈希 / Item hash used by the game for binary search and the unified resource index
}

// VirtualCatalogItem 对应 WfSystem.Catalog.VirtualAssetCatalog.Item
// MessagePack indexed array 依次保存 assetPath、name 和 hash
// VirtualCatalogItem maps WfSystem.Catalog.VirtualAssetCatalog.Item
// Its MessagePack indexed array stores assetPath, name, and hash in that order
type VirtualCatalogItem struct {
	IndexedObjectMetadata        // indexed object 宽度与未来槽位元数据 / Indexed-object width and future-slot metadata
	AssetPath             string `json:"assetPath"`                // 本地 ResourceLocation 使用的 Unity 工程资源路径 / Unity project asset path used as the local ResourceLocation internal ID
	AssetPathIsNil        bool   `json:"assetPathIsNil,omitempty"` // assetPath 在线格式中为 MessagePack nil / assetPath was MessagePack nil on the wire
	Name                  string `json:"name"`                     // 本地 ResourceLocation 的主键 / Primary key of the local ResourceLocation
	NameIsNil             bool   `json:"nameIsNil,omitempty"`      // name 在线格式中为 MessagePack nil / name was MessagePack nil on the wire
	Hash                  uint64 `json:"hash"`                     // 游戏用于二分查找和统一资源索引的条目哈希 / Item hash used by the game for binary search and the unified resource index
}

// ExtensionNameList 表示 .ct 中按扩展名分组的资源名称列表，对应 C# AssetBundleCatalog.ExtensionNameList
// MessagePack indexed array 依次保存 extention 和 data，游戏通过 GetFileNameListFromExtension 读取相应虚拟文件
// ExtensionNameList represents resource names grouped by extension in a .ct file and maps C# AssetBundleCatalog.ExtensionNameList
// Its MessagePack indexed array stores extention and data in that order, and the game reads the corresponding virtual file through GetFileNameListFromExtension
type ExtensionNameList struct {
	MessagePackRootMetadata                     // 根 nil 与尾随字节元数据 / Root nil and trailing-byte metadata
	IndexedObjectMetadata                       // indexed object 宽度与未来槽位元数据 / Indexed-object width and future-slot metadata
	Extension               string              `json:"extention"`                // 线格式中的 extention 值，当前游戏枚举路径未读取此字段 / Wire extention value, not read by the current game's enumeration path
	ExtensionIsNil          bool                `json:"extensionIsNil,omitempty"` // extention 在线格式中为 MessagePack nil / extention was MessagePack nil on the wire
	Data                    []ExtensionNamePack `json:"data"`                     // 游戏枚举并返回名称的 Pack 数组 / Pack array whose names are enumerated and returned by the game
	DataNulls               []bool              `json:"dataNulls,omitempty"`      // data 中在线格式为 nil 的 Pack 元素 / Pack elements in data that were nil on the wire
}

// ExtensionNamePack 表示 ExtensionNameList 中的单个条目，对应 C# AssetBundleCatalog.ExtensionNameList.Pack
// MessagePack indexed array 依次保存 name 和 hash
// ExtensionNamePack represents one ExtensionNameList entry and maps C# AssetBundleCatalog.ExtensionNameList.Pack
// Its MessagePack indexed array stores name and hash in that order
type ExtensionNamePack struct {
	IndexedObjectMetadata        // indexed object 宽度与未来槽位元数据 / Indexed-object width and future-slot metadata
	Name                  string `json:"name"`                // GameResource 枚举扩展名文件列表时返回的资源名称 / Resource name returned when GameResource enumerates an extension file list
	NameIsNil             bool   `json:"nameIsNil,omitempty"` // name 在线格式中为 MessagePack nil / name was MessagePack nil on the wire
	Hash                  uint64 `json:"hash"`                // 线格式中保存的哈希，当前游戏枚举路径未读取此字段 / Hash stored on the wire, not read by the current game's enumeration path
}

// CatalogType 是资源分类标志位枚举（Flags）
// 对应 C# WfSystem.Catalog.CatalogType
// CatalogType is the resource-category flag enumeration
// It maps C# WfSystem.Catalog.CatalogType
type CatalogType int

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

// CatalogPackageType 是包类型枚举
// 对应 C# WfSystem.Catalog.CatalogPackageType
// CatalogPackageType is the package-type enumeration
// It maps C# WfSystem.Catalog.CatalogPackageType
type CatalogPackageType int

const (
	PackageTypeBase        CatalogPackageType = 0
	PackageTypePlugin      CatalogPackageType = 1
	PackageTypePluginPatch CatalogPackageType = 2
	PackageTypeBasePatch   CatalogPackageType = 3
	PackageTypeExtraBase   CatalogPackageType = 4
	PackageTypeExtraPatch  CatalogPackageType = 5
)

// DecodeCatalog 解码一个未压缩的 catalog 根值
// 当数组至少包含 Key(8) 时，根据该槽位的线格式类型区分 AssetBundleCatalog 和 VirtualAssetCatalog；少于九槽的数组无法自动区分
// DecodeCatalog decodes one uncompressed catalog root
// When Key(8) exists its wire type distinguishes AssetBundleCatalog from VirtualAssetCatalog; arrays shorter than nine slots are ambiguous and require DecodeCatalogWithKind
func DecodeCatalog(data []byte) (*AssetBundleCatalog, error) {
	return decodeCatalog(data, nil)
}

// DecodeCatalogWithKind 按调用方指定的 C# 具体类布局解码 catalog
// 短 indexed array 必须使用此函数，未来数组的宽度无法单独识别类型时也应使用此函数
// DecodeCatalogWithKind decodes a catalog using the concrete C# class layout selected by the caller
// It is required for short indexed arrays and for future arrays whose width alone cannot identify a type
func DecodeCatalogWithKind(data []byte, kind CatalogKind) (*AssetBundleCatalog, error) {
	normalized, err := normalizedCatalogKind(kind)
	if err != nil {
		return nil, err
	}
	return decodeCatalog(data, &normalized)
}

// decodeCatalog 解码 catalog 根值并保留根值之后的尾随字节
// decodeCatalog decodes a catalog root and preserves bytes following the root value
func decodeCatalog(data []byte, forcedKind *CatalogKind) (*AssetBundleCatalog, error) {
	root, trailing, err := SplitFirstMsgpackValue(data)
	if err != nil {
		return nil, fmt.Errorf("catalog: %w", err)
	}
	if isRawMsgpackNil(root) {
		if len(trailing) == 0 {
			return nil, nil
		}
		value := &AssetBundleCatalog{MessagePackRootMetadata: MessagePackRootMetadata{RootNil: true, TrailingData: trailing}}
		if forcedKind != nil {
			value.Kind = *forcedKind
		}
		return value, nil
	}
	fields, err := decodeRawMsgpackArray(root, "catalog")
	if err != nil {
		return nil, err
	}
	kind := CatalogKind("")
	if forcedKind != nil {
		kind = *forcedKind
	} else {
		kind, err = inferCatalogKind(fields)
		if err != nil {
			return nil, err
		}
	}
	value, err := decodeCatalogFields(fields, kind)
	if err != nil {
		return nil, err
	}
	value.TrailingData = trailing
	return value, nil
}

// inferCatalogKind 根据 Key(8) 的原始 MessagePack 标记推断 catalog 布局
// inferCatalogKind infers the catalog layout from the raw MessagePack marker at Key(8)
func inferCatalogKind(fields []codec.Raw) (CatalogKind, error) {
	if len(fields) < 9 {
		return "", fmt.Errorf("catalog array(%d) does not contain Key(8); concrete kind is ambiguous, use DecodeCatalogWithKind", len(fields))
	}
	value := fields[8]
	if isRawMsgpackNil(value) || (len(value) != 0 && isArrayMarker(value[0])) {
		return CatalogKindVirtualAsset, nil
	}
	if len(value) == 1 && (value[0] == 0xc2 || value[0] == 0xc3) {
		return CatalogKindAssetBundle, nil
	}
	return "", fmt.Errorf("catalog Key(8) marker cannot identify a concrete kind")
}

// DecodeCatalogFromCt 从 ContentTable 的 catalog 虚拟文件解码 catalog
// DecodeCatalogFromCt decodes the catalog virtual file from ContentTable
func DecodeCatalogFromCt(table *ContentTable) (*AssetBundleCatalog, error) {
	data, err := decodeContentTableMessagePackFile(table, "catalog")
	if err != nil {
		return nil, err
	}
	return DecodeCatalog(data)
}

// DecodeCatalogFromCtWithKind 从 ContentTable 的 catalog 虚拟文件按指定布局解码 catalog
// DecodeCatalogFromCtWithKind decodes the catalog virtual file from ContentTable using the selected layout
func DecodeCatalogFromCtWithKind(table *ContentTable, kind CatalogKind) (*AssetBundleCatalog, error) {
	data, err := decodeContentTableMessagePackFile(table, "catalog")
	if err != nil {
		return nil, err
	}
	return DecodeCatalogWithKind(data, kind)
}

// decodeCatalogFields 将 catalog indexed array 的已知槽位转换为 Go 模型并保留未知槽位
// decodeCatalogFields converts known slots of a catalog indexed array to the Go model while preserving unknown slots
func decodeCatalogFields(fields []codec.Raw, kind CatalogKind) (*AssetBundleCatalog, error) {
	known := 12
	if kind == CatalogKindVirtualAsset {
		known = 10
	}
	value := &AssetBundleCatalog{Kind: kind}
	setIndexedObjectMetadata(&value.IndexedObjectMetadata, fields, known)
	var err error
	if len(fields) >= 1 {
		value.Version, err = decodeRawInt32(fields[0], "catalog[0] version")
	}
	if err == nil && len(fields) >= 2 {
		var decoded int
		decoded, err = decodeRawInt32(fields[1], "catalog[1] catalogType")
		value.CatalogType = CatalogType(decoded)
	}
	if err == nil && len(fields) >= 3 {
		var decoded int
		decoded, err = decodeRawInt32(fields[2], "catalog[2] packageType")
		value.PackageType = CatalogPackageType(decoded)
	}
	if err == nil && len(fields) >= 4 {
		value.Priority, err = decodeRawInt32(fields[3], "catalog[3] priority")
	}
	if err == nil && len(fields) >= 5 {
		value.Name, value.NameIsNil, err = decodeRawNullableString(fields[4], "catalog[4] name")
	}
	if err == nil && len(fields) >= 6 {
		value.SubName, value.SubNameIsNil, err = decodeRawNullableString(fields[5], "catalog[5] subName")
	}
	if err == nil && len(fields) >= 7 {
		value.Hash, err = decodeRawUint64(fields[6], "catalog[6] hash")
	}
	if err == nil && len(fields) >= 8 {
		value.CreateTime, err = decodeRawInt64(fields[7], "catalog[7] createTime")
	}
	if err != nil {
		return nil, err
	}

	if kind == CatalogKindVirtualAsset {
		if len(fields) >= 9 {
			value.ExtensionList, value.ExtensionListNulls, err = decodeRawNullableStringSlice(fields[8], "virtual catalog[8] extensionList")
		}
		if err == nil && len(fields) >= 10 {
			value.VirtualItems, value.VirtualItemNulls, err = decodeVirtualCatalogItemsRaw(fields[9])
		}
	} else {
		if len(fields) >= 9 {
			value.IsEncrypted, err = decodeRawBool(fields[8], "catalog[8] isEncrypted")
		}
		if err == nil && len(fields) >= 10 {
			value.ResourceFileNames, value.ResourceFileNameNulls, err = decodeRawNullableStringSlice(fields[9], "catalog[9] resourceFileNames")
		}
		if err == nil && len(fields) >= 11 {
			value.ExtensionList, value.ExtensionListNulls, err = decodeRawNullableStringSlice(fields[10], "catalog[10] extensionList")
		}
		if err == nil && len(fields) >= 12 {
			value.Items, value.ItemNulls, err = decodeCatalogItemsRaw(fields[11])
		}
	}
	if err != nil {
		return nil, err
	}
	if err := ValidateCatalog(value); err != nil {
		return nil, err
	}
	return value, nil
}

// EncodeCatalog 保留选定的 indexed object 宽度、未来槽位、可空类或字符串元素以及根值后的字节
// 函数不会调用 OnBeforeSerialize，也不会重写 Version
// EncodeCatalog preserves the selected indexed-object width, future slots, nullable class or string elements, and bytes after the root value
// It never invokes OnBeforeSerialize or rewrites Version
func EncodeCatalog(cat *AssetBundleCatalog) ([]byte, error) {
	if cat == nil {
		return []byte{0xc0}, nil
	}
	if cat.RootNil {
		if catalogHasWirePayload(cat) {
			return nil, fmt.Errorf("catalog rootNil would discard populated wire fields")
		}
		return append([]byte{0xc0}, cat.TrailingData...), nil
	}
	kind, err := normalizedCatalogKind(cat.Kind)
	if err != nil {
		return nil, err
	}
	canonical := *cat
	canonical.Kind = kind
	if err := ValidateCatalog(&canonical); err != nil {
		return nil, err
	}
	fields, err := encodeCatalogFields(&canonical)
	if err != nil {
		return nil, err
	}
	encoded, err := encodeMsgpackAllowRaw(fields)
	if err != nil {
		return nil, fmt.Errorf("encode catalog MessagePack: %w", err)
	}
	return append(encoded, canonical.TrailingData...), nil
}

// encodeCatalogFields 按 AssetBundleCatalog 或 VirtualAssetCatalog 的布局构造已知槽位
// encodeCatalogFields builds known slots using the AssetBundleCatalog or VirtualAssetCatalog layout
func encodeCatalogFields(cat *AssetBundleCatalog) ([]interface{}, error) {
	common := []indexedKnownField{
		{name: "version", value: int64(cat.Version), populated: cat.Version != 0},
		{name: "catalogType", value: int64(cat.CatalogType), populated: cat.CatalogType != 0},
		{name: "packageType", value: int64(cat.PackageType), populated: cat.PackageType != 0},
		{name: "priority", value: int64(cat.Priority), populated: cat.Priority != 0},
		{name: "name", value: nullableStringValue(cat.Name, cat.NameIsNil), populated: cat.Name != "" || cat.NameIsNil},
		{name: "subName", value: nullableStringValue(cat.SubName, cat.SubNameIsNil), populated: cat.SubName != "" || cat.SubNameIsNil},
		{name: "hash", value: cat.Hash, populated: cat.Hash != 0},
		{name: "createTime", value: cat.CreateTime, populated: cat.CreateTime != 0},
	}
	if cat.Kind == CatalogKindVirtualAsset {
		extensions, err := encodeNullableStringSlice(cat.ExtensionList, cat.ExtensionListNulls, "virtual catalog extensionList")
		if err != nil {
			return nil, err
		}
		items, err := encodeVirtualCatalogItemsValue(cat.VirtualItems, cat.VirtualItemNulls)
		if err != nil {
			return nil, err
		}
		known := append(common,
			indexedKnownField{name: "extensionList", value: extensions, populated: cat.ExtensionList != nil || len(cat.ExtensionListNulls) != 0},
			indexedKnownField{name: "virtualItems", value: items, populated: cat.VirtualItems != nil || len(cat.VirtualItemNulls) != 0},
		)
		return buildIndexedObject(known, cat.IndexedObjectMetadata, "VirtualAssetCatalog")
	}
	resources, err := encodeNullableStringSlice(cat.ResourceFileNames, cat.ResourceFileNameNulls, "catalog resourceFileNames")
	if err != nil {
		return nil, err
	}
	extensions, err := encodeNullableStringSlice(cat.ExtensionList, cat.ExtensionListNulls, "catalog extensionList")
	if err != nil {
		return nil, err
	}
	items, err := encodeCatalogItemsValue(cat.Items, cat.ItemNulls)
	if err != nil {
		return nil, err
	}
	known := append(common,
		indexedKnownField{name: "isEncrypted", value: cat.IsEncrypted, populated: cat.IsEncrypted},
		indexedKnownField{name: "resourceFileNames", value: resources, populated: cat.ResourceFileNames != nil || len(cat.ResourceFileNameNulls) != 0},
		indexedKnownField{name: "extensionList", value: extensions, populated: cat.ExtensionList != nil || len(cat.ExtensionListNulls) != 0},
		indexedKnownField{name: "items", value: items, populated: cat.Items != nil || len(cat.ItemNulls) != 0},
	)
	return buildIndexedObject(known, cat.IndexedObjectMetadata, "AssetBundleCatalog")
}

// DecodeExtensionNameList 解码一个 ExtensionNameList 根值并保留尾随字节
// DecodeExtensionNameList decodes one ExtensionNameList root and preserves trailing bytes
func DecodeExtensionNameList(data []byte) (*ExtensionNameList, error) {
	root, trailing, err := SplitFirstMsgpackValue(data)
	if err != nil {
		return nil, fmt.Errorf("ExtensionNameList: %w", err)
	}
	if isRawMsgpackNil(root) {
		if len(trailing) == 0 {
			return nil, nil
		}
		return &ExtensionNameList{MessagePackRootMetadata: MessagePackRootMetadata{RootNil: true, TrailingData: trailing}}, nil
	}
	fields, err := decodeRawMsgpackArray(root, "ExtensionNameList")
	if err != nil {
		return nil, err
	}
	value := &ExtensionNameList{}
	setIndexedObjectMetadata(&value.IndexedObjectMetadata, fields, 2)
	if len(fields) >= 1 {
		value.Extension, value.ExtensionIsNil, err = decodeRawNullableString(fields[0], "ExtensionNameList[0] extention")
	}
	if err == nil && len(fields) >= 2 {
		value.Data, value.DataNulls, err = decodeExtensionNamePacksRaw(fields[1])
	}
	if err != nil {
		return nil, err
	}
	value.TrailingData = trailing
	if err := ValidateExtensionNameList(value); err != nil {
		return nil, err
	}
	return value, nil
}

// DecodeExtensionNameListFromCt 从 ContentTable 中按扩展名读取并解码 ExtensionNameList
// DecodeExtensionNameListFromCt reads and decodes an ExtensionNameList from ContentTable by extension name
func DecodeExtensionNameListFromCt(table *ContentTable, extension string) (*ExtensionNameList, error) {
	data, err := decodeContentTableMessagePackFile(table, extension)
	if err != nil {
		return nil, err
	}
	return DecodeExtensionNameList(data)
}

// EncodeExtensionNameList 将 ExtensionNameList 编码为 MessagePack 并保留根值后的字节
// EncodeExtensionNameList encodes ExtensionNameList as MessagePack while preserving bytes after the root value
func EncodeExtensionNameList(enl *ExtensionNameList) ([]byte, error) {
	if enl == nil {
		return []byte{0xc0}, nil
	}
	if enl.RootNil {
		if extensionNameListHasWirePayload(enl) {
			return nil, fmt.Errorf("ExtensionNameList rootNil would discard populated wire fields")
		}
		return append([]byte{0xc0}, enl.TrailingData...), nil
	}
	if err := ValidateExtensionNameList(enl); err != nil {
		return nil, err
	}
	data, err := encodeExtensionNamePacksValue(enl.Data, enl.DataNulls)
	if err != nil {
		return nil, err
	}
	known := []indexedKnownField{
		{name: "extention", value: nullableStringValue(enl.Extension, enl.ExtensionIsNil), populated: enl.Extension != "" || enl.ExtensionIsNil},
		{name: "data", value: data, populated: enl.Data != nil || len(enl.DataNulls) != 0},
	}
	fields, err := buildIndexedObject(known, enl.IndexedObjectMetadata, "ExtensionNameList")
	if err != nil {
		return nil, err
	}
	encoded, err := encodeMsgpackAllowRaw(fields)
	if err != nil {
		return nil, fmt.Errorf("encode ExtensionNameList MessagePack: %w", err)
	}
	return append(encoded, enl.TrailingData...), nil
}

// decodeContentTableMessagePackFile 提取并解压 ContentTable 中的 MessagePack 虚拟文件
// decodeContentTableMessagePackFile extracts and decompresses a MessagePack virtual file from ContentTable
func decodeContentTableMessagePackFile(table *ContentTable, name string) ([]byte, error) {
	if table == nil {
		return nil, fmt.Errorf("nil ContentTable")
	}
	raw, err := table.GetFileData(name)
	if err != nil {
		return nil, err
	}
	decoded, err := DecompressLz4BlockArray(raw)
	if err != nil {
		return nil, fmt.Errorf("decompress content table file %q: %w", name, err)
	}
	return decoded, nil
}

// decodeCatalogItemsRaw 解码 AssetBundle 条目数组并记录 nil 元素
// decodeCatalogItemsRaw decodes the AssetBundle item array and records nil elements
func decodeCatalogItemsRaw(raw codec.Raw) ([]CatalogItem, []bool, error) {
	values, isNil, err := decodeRawArrayOrNil(raw, "catalog items")
	if err != nil || isNil {
		return nil, nil, err
	}
	items := make([]CatalogItem, len(values))
	nulls := make([]bool, len(values))
	for i, value := range values {
		if isRawMsgpackNil(value) {
			nulls[i] = true
			continue
		}
		fields, err := decodeRawMsgpackArray(value, fmt.Sprintf("catalog items[%d]", i))
		if err != nil {
			return nil, nil, err
		}
		setIndexedObjectMetadata(&items[i].IndexedObjectMetadata, fields, 3)
		if len(fields) >= 1 {
			items[i].ResourceIndex, err = decodeRawInt32(fields[0], fmt.Sprintf("catalog items[%d][0] resourceIndex", i))
		}
		if err == nil && len(fields) >= 2 {
			items[i].Name, items[i].NameIsNil, err = decodeRawNullableString(fields[1], fmt.Sprintf("catalog items[%d][1] name", i))
		}
		if err == nil && len(fields) >= 3 {
			items[i].Hash, err = decodeRawUint64(fields[2], fmt.Sprintf("catalog items[%d][2] hash", i))
		}
		if err != nil {
			return nil, nil, err
		}
	}
	return items, trimFalseNullFlags(nulls), nil
}

// decodeVirtualCatalogItemsRaw 解码 VirtualAsset 条目数组并记录 nil 元素
// decodeVirtualCatalogItemsRaw decodes the VirtualAsset item array and records nil elements
func decodeVirtualCatalogItemsRaw(raw codec.Raw) ([]VirtualCatalogItem, []bool, error) {
	values, isNil, err := decodeRawArrayOrNil(raw, "virtual catalog items")
	if err != nil || isNil {
		return nil, nil, err
	}
	items := make([]VirtualCatalogItem, len(values))
	nulls := make([]bool, len(values))
	for i, value := range values {
		if isRawMsgpackNil(value) {
			nulls[i] = true
			continue
		}
		fields, err := decodeRawMsgpackArray(value, fmt.Sprintf("virtual catalog items[%d]", i))
		if err != nil {
			return nil, nil, err
		}
		setIndexedObjectMetadata(&items[i].IndexedObjectMetadata, fields, 3)
		if len(fields) >= 1 {
			items[i].AssetPath, items[i].AssetPathIsNil, err = decodeRawNullableString(fields[0], fmt.Sprintf("virtual catalog items[%d][0] assetPath", i))
		}
		if err == nil && len(fields) >= 2 {
			items[i].Name, items[i].NameIsNil, err = decodeRawNullableString(fields[1], fmt.Sprintf("virtual catalog items[%d][1] name", i))
		}
		if err == nil && len(fields) >= 3 {
			items[i].Hash, err = decodeRawUint64(fields[2], fmt.Sprintf("virtual catalog items[%d][2] hash", i))
		}
		if err != nil {
			return nil, nil, err
		}
	}
	return items, trimFalseNullFlags(nulls), nil
}

// decodeExtensionNamePacksRaw 解码 ExtensionNameList 的 Pack 数组并记录 nil 元素
// decodeExtensionNamePacksRaw decodes an ExtensionNameList Pack array and records nil elements
func decodeExtensionNamePacksRaw(raw codec.Raw) ([]ExtensionNamePack, []bool, error) {
	values, isNil, err := decodeRawArrayOrNil(raw, "ExtensionNameList data")
	if err != nil || isNil {
		return nil, nil, err
	}
	packs := make([]ExtensionNamePack, len(values))
	nulls := make([]bool, len(values))
	for i, value := range values {
		if isRawMsgpackNil(value) {
			nulls[i] = true
			continue
		}
		fields, err := decodeRawMsgpackArray(value, fmt.Sprintf("ExtensionNameList data[%d]", i))
		if err != nil {
			return nil, nil, err
		}
		setIndexedObjectMetadata(&packs[i].IndexedObjectMetadata, fields, 2)
		if len(fields) >= 1 {
			packs[i].Name, packs[i].NameIsNil, err = decodeRawNullableString(fields[0], fmt.Sprintf("ExtensionNameList data[%d][0] name", i))
		}
		if err == nil && len(fields) >= 2 {
			packs[i].Hash, err = decodeRawUint64(fields[1], fmt.Sprintf("ExtensionNameList data[%d][1] hash", i))
		}
		if err != nil {
			return nil, nil, err
		}
	}
	return packs, trimFalseNullFlags(nulls), nil
}

// encodeCatalogItemsValue 编码 AssetBundle 条目数组并恢复 nil 元素
// encodeCatalogItemsValue encodes an AssetBundle item array and restores nil elements
func encodeCatalogItemsValue(items []CatalogItem, nulls []bool) (interface{}, error) {
	if items == nil {
		if len(nulls) != 0 {
			return nil, fmt.Errorf("catalog itemNulls cannot describe a nil items array")
		}
		return nil, nil
	}
	if err := validateNullFlags(nulls, len(items), "catalog itemNulls"); err != nil {
		return nil, err
	}
	result := make([]interface{}, len(items))
	for i := range items {
		if nullFlagAt(nulls, i) {
			if catalogItemHasWirePayload(&items[i]) {
				return nil, fmt.Errorf("catalog items[%d] is marked nil but has populated fields", i)
			}
			continue
		}
		fields, err := encodeCatalogItem(&items[i], fmt.Sprintf("catalog items[%d]", i))
		if err != nil {
			return nil, err
		}
		result[i] = fields
	}
	return result, nil
}

// encodeVirtualCatalogItemsValue 编码 VirtualAsset 条目数组并恢复 nil 元素
// encodeVirtualCatalogItemsValue encodes a VirtualAsset item array and restores nil elements
func encodeVirtualCatalogItemsValue(items []VirtualCatalogItem, nulls []bool) (interface{}, error) {
	if items == nil {
		if len(nulls) != 0 {
			return nil, fmt.Errorf("virtualItemNulls cannot describe a nil virtualItems array")
		}
		return nil, nil
	}
	if err := validateNullFlags(nulls, len(items), "virtualItemNulls"); err != nil {
		return nil, err
	}
	result := make([]interface{}, len(items))
	for i := range items {
		if nullFlagAt(nulls, i) {
			if virtualCatalogItemHasWirePayload(&items[i]) {
				return nil, fmt.Errorf("virtualItems[%d] is marked nil but has populated fields", i)
			}
			continue
		}
		fields, err := encodeVirtualCatalogItem(&items[i], fmt.Sprintf("virtualItems[%d]", i))
		if err != nil {
			return nil, err
		}
		result[i] = fields
	}
	return result, nil
}

// encodeExtensionNamePacksValue 编码 ExtensionNameList 的 Pack 数组并恢复 nil 元素
// encodeExtensionNamePacksValue encodes an ExtensionNameList Pack array and restores nil elements
func encodeExtensionNamePacksValue(packs []ExtensionNamePack, nulls []bool) (interface{}, error) {
	if packs == nil {
		if len(nulls) != 0 {
			return nil, fmt.Errorf("ExtensionNameList dataNulls cannot describe a nil data array")
		}
		return nil, nil
	}
	if err := validateNullFlags(nulls, len(packs), "ExtensionNameList dataNulls"); err != nil {
		return nil, err
	}
	result := make([]interface{}, len(packs))
	for i := range packs {
		if nullFlagAt(nulls, i) {
			if extensionNamePackHasWirePayload(&packs[i]) {
				return nil, fmt.Errorf("ExtensionNameList data[%d] is marked nil but has populated fields", i)
			}
			continue
		}
		fields, err := encodeExtensionNamePack(&packs[i], fmt.Sprintf("ExtensionNameList data[%d]", i))
		if err != nil {
			return nil, err
		}
		result[i] = fields
	}
	return result, nil
}

// encodeCatalogItem 编码一个 AssetBundle catalog 条目并校验其线格式字段
// encodeCatalogItem encodes one AssetBundle catalog item and validates its wire fields
func encodeCatalogItem(item *CatalogItem, label string) ([]interface{}, error) {
	if err := validateInt32Field(item.ResourceIndex, label+" resourceIndex"); err != nil {
		return nil, err
	}
	if err := validateNullableString(item.Name, item.NameIsNil, label+" name"); err != nil {
		return nil, err
	}
	known := []indexedKnownField{
		{name: "resourceIndex", value: int64(item.ResourceIndex), populated: item.ResourceIndex != 0},
		{name: "name", value: nullableStringValue(item.Name, item.NameIsNil), populated: item.Name != "" || item.NameIsNil},
		{name: "hash", value: item.Hash, populated: item.Hash != 0},
	}
	return buildIndexedObject(known, item.IndexedObjectMetadata, label)
}

// encodeVirtualCatalogItem 编码一个 VirtualAsset catalog 条目并校验其线格式字段
// encodeVirtualCatalogItem encodes one VirtualAsset catalog item and validates its wire fields
func encodeVirtualCatalogItem(item *VirtualCatalogItem, label string) ([]interface{}, error) {
	if err := validateNullableString(item.AssetPath, item.AssetPathIsNil, label+" assetPath"); err != nil {
		return nil, err
	}
	if err := validateNullableString(item.Name, item.NameIsNil, label+" name"); err != nil {
		return nil, err
	}
	known := []indexedKnownField{
		{name: "assetPath", value: nullableStringValue(item.AssetPath, item.AssetPathIsNil), populated: item.AssetPath != "" || item.AssetPathIsNil},
		{name: "name", value: nullableStringValue(item.Name, item.NameIsNil), populated: item.Name != "" || item.NameIsNil},
		{name: "hash", value: item.Hash, populated: item.Hash != 0},
	}
	return buildIndexedObject(known, item.IndexedObjectMetadata, label)
}

// encodeExtensionNamePack 编码一个 ExtensionNameList Pack 条目并校验其线格式字段
// encodeExtensionNamePack encodes one ExtensionNameList Pack item and validates its wire fields
func encodeExtensionNamePack(pack *ExtensionNamePack, label string) ([]interface{}, error) {
	if err := validateNullableString(pack.Name, pack.NameIsNil, label+" name"); err != nil {
		return nil, err
	}
	known := []indexedKnownField{
		{name: "name", value: nullableStringValue(pack.Name, pack.NameIsNil), populated: pack.Name != "" || pack.NameIsNil},
		{name: "hash", value: pack.Hash, populated: pack.Hash != 0},
	}
	return buildIndexedObject(known, pack.IndexedObjectMetadata, label)
}

// indexedKnownField 描述一个 indexed object 的已知槽位及其丢弃检查状态
// indexedKnownField describes one known indexed-object slot and its discard-check state
type indexedKnownField struct {
	name      string      // 校验错误中使用的字段名 / Field name used in validation errors
	value     interface{} // 写入该槽位的 MessagePack 值 / MessagePack value written to the slot
	populated bool        // 缩短数组时该槽位是否包含不可丢弃的数据 / Whether the slot contains data that cannot be discarded when shortening the array
}

// buildIndexedObject 按保留的数组宽度组合已知槽位与未来槽位
// buildIndexedObject combines known and future slots using the retained array width
func buildIndexedObject(known []indexedKnownField, metadata IndexedObjectMetadata, label string) ([]interface{}, error) {
	count, err := resolveIndexedObjectFieldCount(metadata.FieldCount, len(known), metadata.FutureSlots, label)
	if err != nil {
		return nil, err
	}
	for i := count; i < len(known); i++ {
		if known[i].populated {
			return nil, fmt.Errorf("%s fieldCount %d would discard %s", label, count, known[i].name)
		}
	}
	result := make([]interface{}, 0, count)
	for i := 0; i < count && i < len(known); i++ {
		result = append(result, known[i].value)
	}
	for _, raw := range metadata.FutureSlots {
		result = append(result, codec.Raw(append([]byte(nil), raw...)))
	}
	return result, nil
}

// setIndexedObjectMetadata 根据解码数组记录非标准宽度与未来槽位
// setIndexedObjectMetadata records non-canonical width and future slots from a decoded array
func setIndexedObjectMetadata(metadata *IndexedObjectMetadata, fields []codec.Raw, known int) {
	if len(fields) != known {
		count := len(fields)
		metadata.FieldCount = &count
	}
	if len(fields) > known {
		metadata.FutureSlots = cloneCodecRawSlots(fields[known:])
	}
}

// decodeRawArrayOrNil 解码原始 MessagePack 数组并单独报告 nil
// decodeRawArrayOrNil decodes a raw MessagePack array and reports nil separately
func decodeRawArrayOrNil(raw codec.Raw, label string) ([]codec.Raw, bool, error) {
	if isRawMsgpackNil(raw) {
		return nil, true, nil
	}
	values, err := decodeRawMsgpackArray(raw, label)
	return values, false, err
}

// decodeRawInt32 将原始 MessagePack 整数解码并限制到 Int32 范围
// decodeRawInt32 decodes a raw MessagePack integer and constrains it to the Int32 range
func decodeRawInt32(raw codec.Raw, label string) (int, error) {
	value, err := decodeRawInterface(raw, label)
	if err != nil {
		return 0, err
	}
	decoded, ok := toInt(value)
	if !ok {
		return 0, fmt.Errorf("%s: expected Int32 integer, got %T", label, value)
	}
	return decoded, nil
}

// decodeRawInt64 将原始 MessagePack 整数解码为有符号 Int64
// decodeRawInt64 decodes a raw MessagePack integer as signed Int64
func decodeRawInt64(raw codec.Raw, label string) (int64, error) {
	value, err := decodeRawInterface(raw, label)
	if err != nil {
		return 0, err
	}
	decoded, ok := toInt64(value)
	if !ok {
		return 0, fmt.Errorf("%s: expected Int64 integer, got %T", label, value)
	}
	return decoded, nil
}

// decodeRawUint64 将原始 MessagePack 整数解码为 UInt64
// decodeRawUint64 decodes a raw MessagePack integer as UInt64
func decodeRawUint64(raw codec.Raw, label string) (uint64, error) {
	value, err := decodeRawInterface(raw, label)
	if err != nil {
		return 0, err
	}
	decoded, ok := toUint64(value)
	if !ok {
		return 0, fmt.Errorf("%s: expected UInt64 integer, got %T", label, value)
	}
	return decoded, nil
}

// decodeRawBool 解码一个原始 MessagePack 布尔值
// decodeRawBool decodes one raw MessagePack boolean value
func decodeRawBool(raw codec.Raw, label string) (bool, error) {
	value, err := decodeRawInterface(raw, label)
	if err != nil {
		return false, err
	}
	decoded, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("%s: expected bool, got %T", label, value)
	}
	return decoded, nil
}

// decodeRawNullableString 解码字符串，并以独立标志保留 MessagePack nil
// decodeRawNullableString decodes a string and preserves MessagePack nil in a separate flag
func decodeRawNullableString(raw codec.Raw, label string) (string, bool, error) {
	if isRawMsgpackNil(raw) {
		return "", true, nil
	}
	if len(raw) == 0 || !isMsgpackStringMarker(raw[0]) {
		return "", false, fmt.Errorf("%s: expected string or nil", label)
	}
	var value string
	if err := decodeSingleRawMsgpackValue(raw, &value, label); err != nil {
		return "", false, err
	}
	return value, false, nil
}

// decodeRawNullableStringSlice 解码可为 nil 的字符串数组及其 nil 元素标志
// decodeRawNullableStringSlice decodes a nullable string array and flags its nil elements
func decodeRawNullableStringSlice(raw codec.Raw, label string) ([]string, []bool, error) {
	values, isNil, err := decodeRawArrayOrNil(raw, label)
	if err != nil || isNil {
		return nil, nil, err
	}
	strings := make([]string, len(values))
	nulls := make([]bool, len(values))
	for i := range values {
		strings[i], nulls[i], err = decodeRawNullableString(values[i], fmt.Sprintf("%s[%d]", label, i))
		if err != nil {
			return nil, nil, err
		}
	}
	return strings, trimFalseNullFlags(nulls), nil
}

// decodeRawInterface 将恰好一个原始 MessagePack 值解码为通用 Go 值
// decodeRawInterface decodes exactly one raw MessagePack value into a generic Go value
func decodeRawInterface(raw codec.Raw, label string) (interface{}, error) {
	var value interface{}
	if err := decodeSingleRawMsgpackValue(raw, &value, label); err != nil {
		return nil, err
	}
	return value, nil
}

// encodeNullableStringSlice 编码字符串数组并按标志恢复 nil 元素
// encodeNullableStringSlice encodes a string array and restores nil elements from flags
func encodeNullableStringSlice(values []string, nulls []bool, label string) (interface{}, error) {
	if values == nil {
		if len(nulls) != 0 {
			return nil, fmt.Errorf("%s null flags cannot describe a nil array", label)
		}
		return nil, nil
	}
	if err := validateNullFlags(nulls, len(values), label+" null flags"); err != nil {
		return nil, err
	}
	result := make([]interface{}, len(values))
	for i := range values {
		if nullFlagAt(nulls, i) {
			if values[i] != "" {
				return nil, fmt.Errorf("%s[%d] is marked nil but has non-empty text", label, i)
			}
			continue
		}
		if !utf8.ValidString(values[i]) {
			return nil, fmt.Errorf("%s[%d] is not valid UTF-8", label, i)
		}
		result[i] = values[i]
	}
	return result, nil
}

// validateNullableString 校验字符串与其 nil 标志一致并包含有效 UTF-8
// validateNullableString validates that a string agrees with its nil flag and contains valid UTF-8
func validateNullableString(value string, isNil bool, label string) error {
	if isNil {
		if value != "" {
			return fmt.Errorf("%s is marked nil but has non-empty text", label)
		}
		return nil
	}
	if !utf8.ValidString(value) {
		return fmt.Errorf("%s is not valid UTF-8", label)
	}
	return nil
}

// nullableStringValue 按 nil 标志返回字符串或 nil 的可编码值
// nullableStringValue returns an encodable string or nil according to the nil flag
func nullableStringValue(value string, isNil bool) interface{} {
	if isNil {
		return nil
	}
	return value
}

// validateNullFlags 校验 nil 标志数量不超过对应数组长度
// validateNullFlags validates that nil flags do not exceed the corresponding array length
func validateNullFlags(flags []bool, count int, label string) error {
	if len(flags) > count {
		return fmt.Errorf("%s has %d entries for %d values", label, len(flags), count)
	}
	return nil
}

// nullFlagAt 返回给定索引是否被标记为线格式 nil
// nullFlagAt reports whether an index is marked as wire nil
func nullFlagAt(flags []bool, index int) bool {
	return index < len(flags) && flags[index]
}

// trimFalseNullFlags 删除最后一个 true 之后没有信息量的 false 标志
// trimFalseNullFlags removes uninformative false flags after the final true entry
func trimFalseNullFlags(flags []bool) []bool {
	last := -1
	for i, value := range flags {
		if value {
			last = i
		}
	}
	if last < 0 {
		return nil
	}
	return flags[:last+1]
}

// isRawMsgpackNil 识别显式 0xc0 以及 ugorji 使用的空切片 nil 表示
// isRawMsgpackNil recognizes explicit 0xc0 and ugorji's empty-slice nil representation
func isRawMsgpackNil(raw []byte) bool {
	return len(raw) == 0 || (len(raw) == 1 && raw[0] == 0xc0)
}

// isMsgpackStringMarker 判断标记是否开始一个 MessagePack 字符串
// isMsgpackStringMarker reports whether a marker begins a MessagePack string
func isMsgpackStringMarker(marker byte) bool {
	return marker >= 0xa0 && marker <= 0xbf || marker == 0xd9 || marker == 0xda || marker == 0xdb
}

// indexedObjectMetadataHasWirePayload 判断元数据是否包含必须保留的线格式形状
// indexedObjectMetadataHasWirePayload reports whether metadata contains wire shape that must be preserved
func indexedObjectMetadataHasWirePayload(metadata IndexedObjectMetadata) bool {
	return metadata.FieldCount != nil || len(metadata.FutureSlots) != 0
}

// catalogHasWirePayload 判断 catalog 是否包含会被根 nil 丢弃的线格式数据
// catalogHasWirePayload reports whether a catalog contains wire data that root nil would discard
func catalogHasWirePayload(cat *AssetBundleCatalog) bool {
	return indexedObjectMetadataHasWirePayload(cat.IndexedObjectMetadata) ||
		cat.Version != 0 || cat.CatalogType != 0 || cat.PackageType != 0 || cat.Priority != 0 ||
		cat.Name != "" || cat.NameIsNil || cat.SubName != "" || cat.SubNameIsNil ||
		cat.Hash != 0 || cat.CreateTime != 0 || cat.IsEncrypted ||
		cat.ResourceFileNames != nil || len(cat.ResourceFileNameNulls) != 0 ||
		cat.ExtensionList != nil || len(cat.ExtensionListNulls) != 0 ||
		cat.Items != nil || len(cat.ItemNulls) != 0 ||
		cat.VirtualItems != nil || len(cat.VirtualItemNulls) != 0
}

// catalogItemHasWirePayload 判断 AssetBundle 条目是否包含不可由 nil 元素表示的数据
// catalogItemHasWirePayload reports whether an AssetBundle item contains data that a nil element cannot represent
func catalogItemHasWirePayload(item *CatalogItem) bool {
	return indexedObjectMetadataHasWirePayload(item.IndexedObjectMetadata) ||
		item.ResourceIndex != 0 || item.Name != "" || item.NameIsNil || item.Hash != 0
}

// virtualCatalogItemHasWirePayload 判断 VirtualAsset 条目是否包含不可由 nil 元素表示的数据
// virtualCatalogItemHasWirePayload reports whether a VirtualAsset item contains data that a nil element cannot represent
func virtualCatalogItemHasWirePayload(item *VirtualCatalogItem) bool {
	return indexedObjectMetadataHasWirePayload(item.IndexedObjectMetadata) ||
		item.AssetPath != "" || item.AssetPathIsNil || item.Name != "" || item.NameIsNil || item.Hash != 0
}

// extensionNameListHasWirePayload 判断 ExtensionNameList 是否包含会被根 nil 丢弃的数据
// extensionNameListHasWirePayload reports whether ExtensionNameList contains data that root nil would discard
func extensionNameListHasWirePayload(value *ExtensionNameList) bool {
	return indexedObjectMetadataHasWirePayload(value.IndexedObjectMetadata) ||
		value.Extension != "" || value.ExtensionIsNil || value.Data != nil || len(value.DataNulls) != 0
}

// extensionNamePackHasWirePayload 判断 Pack 是否包含不可由 nil 元素表示的数据
// extensionNamePackHasWirePayload reports whether a Pack contains data that a nil element cannot represent
func extensionNamePackHasWirePayload(value *ExtensionNamePack) bool {
	return indexedObjectMetadataHasWirePayload(value.IndexedObjectMetadata) ||
		value.Name != "" || value.NameIsNil || value.Hash != 0
}

// toUint64 将支持的非负 MessagePack 整数转换为 UInt64
// toUint64 converts a supported non-negative MessagePack integer to UInt64
func toUint64(v interface{}) (uint64, bool) {
	switch n := v.(type) {
	case uint64:
		return n, true
	case int64:
		if n < 0 {
			return 0, false
		}
		return uint64(n), true
	case int:
		if n < 0 {
			return 0, false
		}
		return uint64(n), true
	case uint:
		return uint64(n), true
	}
	return 0, false
}

// ValidateCatalog 只校验线格式可表示性以及所选具体布局会静默丢弃的字段
// 运行时查找规则、枚举成员、哈希、排序和资源索引有效性属于游戏逻辑，本函数不会强制检查
// ValidateCatalog checks only wire representability and fields that the selected concrete layout would silently discard
// Runtime lookup rules, enum membership, hashes, sorting, and resource-index validity belong to the game and are deliberately not enforced here
func ValidateCatalog(cat *AssetBundleCatalog) error {
	if cat == nil {
		return nil
	}
	if cat.RootNil {
		if catalogHasWirePayload(cat) {
			return fmt.Errorf("catalog rootNil would discard populated wire fields")
		}
		return nil
	}
	kind, err := normalizedCatalogKind(cat.Kind)
	if err != nil {
		return err
	}
	if err := validateInt32Field(cat.Version, "catalog version"); err != nil {
		return err
	}
	if err := validateInt32Field(cat.Priority, "catalog priority"); err != nil {
		return err
	}
	if err := validateInt32Field(int(cat.CatalogType), "catalog type"); err != nil {
		return err
	}
	if err := validateInt32Field(int(cat.PackageType), "catalog package type"); err != nil {
		return err
	}
	if err := validateNullableString(cat.Name, cat.NameIsNil, "catalog name"); err != nil {
		return err
	}
	if err := validateNullableString(cat.SubName, cat.SubNameIsNil, "catalog subName"); err != nil {
		return err
	}

	switch kind {
	case CatalogKindAssetBundle:
		if cat.VirtualItems != nil || len(cat.VirtualItemNulls) != 0 {
			return fmt.Errorf("assetBundle catalog cannot contain virtualItems")
		}
	case CatalogKindVirtualAsset:
		if cat.IsEncrypted {
			return fmt.Errorf("virtualAsset catalog cannot set isEncrypted")
		}
		if cat.ResourceFileNames != nil || len(cat.ResourceFileNameNulls) != 0 {
			return fmt.Errorf("virtualAsset catalog cannot contain resourceFileNames")
		}
		if cat.Items != nil || len(cat.ItemNulls) != 0 {
			return fmt.Errorf("virtualAsset catalog cannot contain assetBundle items")
		}
	default:
		return fmt.Errorf("unsupported catalog kind %q", kind)
	}
	canonical := *cat
	canonical.Kind = kind
	_, err = encodeCatalogFields(&canonical)
	return err
}

// normalizedCatalogKind 将兼容旧 JSON 的空 Kind 解析为 AssetBundle 布局
// normalizedCatalogKind resolves an empty Kind from legacy JSON to the AssetBundle layout
func normalizedCatalogKind(kind CatalogKind) (CatalogKind, error) {
	switch kind {
	case "", CatalogKindAssetBundle:
		return CatalogKindAssetBundle, nil
	case CatalogKindVirtualAsset:
		return CatalogKindVirtualAsset, nil
	default:
		return "", fmt.Errorf("unsupported catalog kind %q", kind)
	}
}

// ValidateExtensionNameList 校验 ExtensionNameList 能按保留的 indexed object 形状编码
// ValidateExtensionNameList validates that ExtensionNameList can be encoded with its retained indexed-object shape
func ValidateExtensionNameList(enl *ExtensionNameList) error {
	if enl == nil {
		return nil
	}
	if enl.RootNil {
		if extensionNameListHasWirePayload(enl) {
			return fmt.Errorf("ExtensionNameList rootNil would discard populated wire fields")
		}
		return nil
	}
	if err := validateNullableString(enl.Extension, enl.ExtensionIsNil, "ExtensionNameList extention"); err != nil {
		return err
	}
	data, err := encodeExtensionNamePacksValue(enl.Data, enl.DataNulls)
	if err != nil {
		return err
	}
	known := []indexedKnownField{
		{name: "extention", value: nullableStringValue(enl.Extension, enl.ExtensionIsNil), populated: enl.Extension != "" || enl.ExtensionIsNil},
		{name: "data", value: data, populated: enl.Data != nil || len(enl.DataNulls) != 0},
	}
	_, err = buildIndexedObject(known, enl.IndexedObjectMetadata, "ExtensionNameList")
	return err
}
