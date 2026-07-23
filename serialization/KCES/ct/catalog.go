package ct

import (
	"fmt"
	"unicode/utf8"

	"github.com/ugorji/go/codec"
)

// CatalogKind identifies the two concrete catalog layouts used by KCES.
// The empty value is accepted only when reading legacy JSON and means the
// historical 12-slot AssetBundleCatalog layout.
type CatalogKind string

const (
	CatalogKindAssetBundle  CatalogKind = "assetBundle"
	CatalogKindVirtualAsset CatalogKind = "virtualAsset"
)

// AssetBundleCatalog is the backward-compatible Go catalog envelope. Kind
// selects either C# AssetBundleCatalog's 12-slot layout or VirtualAssetCatalog's
// 10-slot local-mode layout; the historical type name and assetBundle JSON fields
// are retained so existing callers and JSON remain readable.
//
// MessagePack 布局（[Key(N)] 对应数组下标）：
//
//	[0]  version           int          固定 1000
//	[1]  catalogType       CatalogType  资源分类标志位
//	[2]  packageType       CatalogPackageType  包类型
//	[3]  priority          int          加载优先级
//	[4]  name              string       catalog 名称
//	[5]  subName           string       子名称
//	[6]  hash              uint64       catalog 自身的 hash
//	[7]  createTime        int64        创建时间戳
//	[8]  isEncrypted       bool         是否加密（abap）
//	[9]  resourceFileNames []string     关联的资源文件名（如 "{name}.aba"）
//	[10] extensionList     []string     扩展名列表（如 ".menuassets", ".tex"）
//	[11] items             []Item       资源索引条目
type AssetBundleCatalog struct {
	MessagePackRootMetadata
	IndexedObjectMetadata
	Kind                  CatalogKind          `json:"kind,omitempty"`                  // catalog wire variant; empty is legacy JSON for assetBundle / catalog 线格式；空值表示旧 JSON 的 assetBundle
	Version               int32                `json:"version"`                         // 当前游戏通常写 1000；解码和重编码时原样保留 / The current game normally writes 1000; decoding and re-encoding preserve the stored value
	CatalogType           CatalogType          `json:"catalogType"`                     // 资源分类标志位 Flags 枚举，如 Parts=4096 / Resource category flag enum such as Parts=4096
	PackageType           CatalogPackageType   `json:"packageType"`                     // 包类型枚举，如 Base=0, Plugin=1 / Package type enum such as Base=0, Plugin=1
	Priority              int32                `json:"priority"`                        // 加载优先级，数值越大优先级越高 / Load priority, higher values load earlier
	Name                  string               `json:"name"`                            // catalog 名称，通常与 .ct 文件名一致且不含扩展名 / Catalog name, usually matching the .ct file name without extension
	NameIsNil             bool                 `json:"nameIsNil,omitempty"`             // Key(4) was MessagePack nil / Key(4) 为 MessagePack nil
	SubName               string               `json:"subName"`                         // 子名称，通常为空 / Sub name, usually empty
	SubNameIsNil          bool                 `json:"subNameIsNil,omitempty"`          // Key(5) was MessagePack nil / Key(5) 为 MessagePack nil
	Hash                  uint64               `json:"hash"`                            // catalog 稳定标识；.aba 通常使用 name.aba 的忽略大小写 hash / Stable catalog identifier; .aba catalogs commonly hash name.aba
	CreateTime            int64                `json:"createTime"`                      // 创建时间戳 / Creation timestamp
	IsEncrypted           bool                 `json:"isEncrypted"`                     // 是否为加密包 abap 格式 / Whether this is an encrypted abap package
	ResourceFileNames     []string             `json:"resourceFileNames"`               // 关联的资源文件名列表，如 name.aba / Related resource file-name list such as name.aba
	ResourceFileNameNulls []bool               `json:"resourceFileNameNulls,omitempty"` // nil string elements in resourceFileNames / resourceFileNames 中的 nil 字符串
	ExtensionList         []string             `json:"extensionList"`                   // 扩展名列表，每个扩展名对应 .ct 中一个同名 ExtensionNameList 文件 / Extension list, each extension maps to a same-name ExtensionNameList virtual file
	ExtensionListNulls    []bool               `json:"extensionListNulls,omitempty"`    // nil string elements in extensionList / extensionList 中的 nil 字符串
	Items                 []CatalogItem        `json:"items"`                           // 资源索引条目数组；游戏通常按 hash 排序，序列化器保留原顺序 / Resource index item array; the game normally sorts by hash, while this serializer preserves stored order
	ItemNulls             []bool               `json:"itemNulls,omitempty"`             // nil class elements in items / items 中的 nil 类元素
	VirtualItems          []VirtualCatalogItem `json:"virtualItems,omitempty"`          // local-mode 资源条目；仅 virtualAsset kind 可用 / Local-mode items, valid only for virtualAsset kind
	VirtualItemNulls      []bool               `json:"virtualItemNulls,omitempty"`      // nil class elements in virtualItems / virtualItems 中的 nil 类元素
}

// CatalogItem 表示 catalog 中的单个资源索引条目 / CatalogItem represents one resource index item in the catalog
// 对应 C# AssetBundleCatalog.Item，MessagePack indexed array / Matches C# AssetBundleCatalog.Item as a MessagePack indexed array:
//
//	[0] resourceIndex  int     指向 resourceFileNames 的索引
//	[1] name           string  资源名称（如 "xxx.menuassets"）
//	[2] hash           uint64  资源名称的 FNV-1a ignore-case hash
type CatalogItem struct {
	IndexedObjectMetadata
	ResourceIndex int32  `json:"resourceIndex"` // 指向 AssetBundleCatalog.ResourceFileNames 的索引 / Index into AssetBundleCatalog.ResourceFileNames
	Name          string `json:"name"`          // 资源名称，游戏通过此名称加载资源 / Resource name used by the game to load the asset
	NameIsNil     bool   `json:"nameIsNil,omitempty"`
	Hash          uint64 `json:"hash"` // 资源名称的 FNV-1a 64-bit ignore-case hash，用于快速查找 / FNV-1a 64-bit ignore-case hash of the resource name for fast lookup
}

// VirtualCatalogItem maps WfSystem.Catalog.VirtualAssetCatalog.Item.
// MessagePack indexed-array layout: [assetPath, name, hash].
type VirtualCatalogItem struct {
	IndexedObjectMetadata
	AssetPath      string `json:"assetPath"` // Unity project-local asset path / Unity 工程内资源路径
	AssetPathIsNil bool   `json:"assetPathIsNil,omitempty"`
	Name           string `json:"name"` // resource lookup name / 资源查找名称
	NameIsNil      bool   `json:"nameIsNil,omitempty"`
	Hash           uint64 `json:"hash"` // case-insensitive hash of Name / Name 的忽略大小写哈希
}

// ExtensionNameList 表示 .ct 中按扩展名分组的资源名称列表 / ExtensionNameList represents resource names grouped by extension inside a .ct file
// 对应 C# AssetBundleCatalog.ExtensionNameList，MessagePack indexed array / Matches C# AssetBundleCatalog.ExtensionNameList as a MessagePack indexed array:
//
//	[0] extention  string  扩展名（如 ".menuassets"）
//	[1] data       []Pack  名称+hash 列表
//
// 游戏通过 GetFileNameListFromExtension 获取某扩展名下的所有资源名 / The game uses GetFileNameListFromExtension to enumerate resource names for an extension
type ExtensionNameList struct {
	MessagePackRootMetadata
	IndexedObjectMetadata
	Extension      string              `json:"extention"` // 扩展名，如 .menuassets、.tex、.model，字段名保留游戏 extention 拼写 / Extension such as .menuassets, .tex, or .model, keeping the game's extention spelling
	ExtensionIsNil bool                `json:"extensionIsNil,omitempty"`
	Data           []ExtensionNamePack `json:"data"` // 该扩展名下的所有资源名称及其 hash / Resource names and hashes under this extension
	DataNulls      []bool              `json:"dataNulls,omitempty"`
}

// ExtensionNamePack 表示 ExtensionNameList 中的单个条目 / ExtensionNamePack represents one item in ExtensionNameList
// 对应 C# AssetBundleCatalog.ExtensionNameList.Pack，MessagePack indexed array / Matches C# AssetBundleCatalog.ExtensionNameList.Pack as a MessagePack indexed array:
//
//	[0] name  string  资源名称（不含扩展名）
//	[1] hash  uint64  名称的 FNV-1a ignore-case hash
type ExtensionNamePack struct {
	IndexedObjectMetadata
	Name      string `json:"name"` // 资源名称 / Resource name
	NameIsNil bool   `json:"nameIsNil,omitempty"`
	Hash      uint64 `json:"hash"` // 名称的 FNV-1a 64-bit ignore-case hash / FNV-1a 64-bit ignore-case hash of the name
}

// CatalogType 资源分类标志位枚举（Flags）。
// 对应 C# WfSystem.Catalog.CatalogType
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

// CatalogPackageType 包类型枚举。
// 对应 C# WfSystem.Catalog.CatalogPackageType
type CatalogPackageType int32

const (
	PackageTypeBase        CatalogPackageType = 0
	PackageTypePlugin      CatalogPackageType = 1
	PackageTypePluginPatch CatalogPackageType = 2
	PackageTypeBasePatch   CatalogPackageType = 3
	PackageTypeExtraBase   CatalogPackageType = 4
	PackageTypeExtraPatch  CatalogPackageType = 5
)

// DecodeCatalog decodes one uncompressed catalog root. The concrete class is
// inferred from Key(8), whose wire type is disjoint: bool for
// AssetBundleCatalog and string[]/nil for VirtualAssetCatalog. Arrays shorter
// than nine slots are genuinely ambiguous and require DecodeCatalogWithKind.
func DecodeCatalog(data []byte) (*AssetBundleCatalog, error) {
	return decodeCatalog(data, nil)
}

// DecodeCatalogWithKind decodes a catalog using the concrete C# class chosen
// by the caller. This is required for short indexed arrays and is also the
// lossless route for future arrays whose width alone cannot identify a type.
func DecodeCatalogWithKind(data []byte, kind CatalogKind) (*AssetBundleCatalog, error) {
	normalized, err := normalizedCatalogKind(kind)
	if err != nil {
		return nil, err
	}
	return decodeCatalog(data, &normalized)
}

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

func DecodeCatalogFromCt(table *ContentTable) (*AssetBundleCatalog, error) {
	data, err := decodeContentTableMessagePackFile(table, "catalog")
	if err != nil {
		return nil, err
	}
	return DecodeCatalog(data)
}

func DecodeCatalogFromCtWithKind(table *ContentTable, kind CatalogKind) (*AssetBundleCatalog, error) {
	data, err := decodeContentTableMessagePackFile(table, "catalog")
	if err != nil {
		return nil, err
	}
	return DecodeCatalogWithKind(data, kind)
}

func decodeCatalogFields(fields []codec.Raw, kind CatalogKind) (*AssetBundleCatalog, error) {
	known := 12
	if kind == CatalogKindVirtualAsset {
		known = 10
	}
	value := &AssetBundleCatalog{Kind: kind}
	if err := setIndexedObjectMetadata(&value.IndexedObjectMetadata, fields, int64(known)); err != nil {
		return nil, err
	}
	var err error
	if len(fields) >= 1 {
		value.Version, err = decodeRawInt32(fields[0], "catalog[0] version")
	}
	if err == nil && len(fields) >= 2 {
		var decoded int32
		decoded, err = decodeRawInt32(fields[1], "catalog[1] catalogType")
		value.CatalogType = CatalogType(decoded)
	}
	if err == nil && len(fields) >= 3 {
		var decoded int32
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

// EncodeCatalog preserves the selected indexed-object width, future slots,
// nullable class/string elements, and bytes after the root value. It never
// invokes OnBeforeSerialize or rewrites version.
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

func encodeCatalogFields(cat *AssetBundleCatalog) ([]interface{}, error) {
	common := []indexedKnownField{
		{name: "version", value: cat.Version, populated: cat.Version != 0},
		{name: "catalogType", value: cat.CatalogType, populated: cat.CatalogType != 0},
		{name: "packageType", value: cat.PackageType, populated: cat.PackageType != 0},
		{name: "priority", value: cat.Priority, populated: cat.Priority != 0},
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
	if err := setIndexedObjectMetadata(&value.IndexedObjectMetadata, fields, 2); err != nil {
		return nil, err
	}
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

func DecodeExtensionNameListFromCt(table *ContentTable, extension string) (*ExtensionNameList, error) {
	data, err := decodeContentTableMessagePackFile(table, extension)
	if err != nil {
		return nil, err
	}
	return DecodeExtensionNameList(data)
}

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
		if err := setIndexedObjectMetadata(&items[i].IndexedObjectMetadata, fields, 3); err != nil {
			return nil, nil, err
		}
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
		if err := setIndexedObjectMetadata(&items[i].IndexedObjectMetadata, fields, 3); err != nil {
			return nil, nil, err
		}
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
		if err := setIndexedObjectMetadata(&packs[i].IndexedObjectMetadata, fields, 2); err != nil {
			return nil, nil, err
		}
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

func encodeCatalogItemsValue(items []CatalogItem, nulls []bool) (interface{}, error) {
	if items == nil {
		if len(nulls) != 0 {
			return nil, fmt.Errorf("catalog itemNulls cannot describe a nil items array")
		}
		return nil, nil
	}
	if err := validateNullFlags(nulls, int64(len(items)), "catalog itemNulls"); err != nil {
		return nil, err
	}
	result := make([]interface{}, len(items))
	for i := range items {
		if nullFlagAt(nulls, int64(i)) {
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

func encodeVirtualCatalogItemsValue(items []VirtualCatalogItem, nulls []bool) (interface{}, error) {
	if items == nil {
		if len(nulls) != 0 {
			return nil, fmt.Errorf("virtualItemNulls cannot describe a nil virtualItems array")
		}
		return nil, nil
	}
	if err := validateNullFlags(nulls, int64(len(items)), "virtualItemNulls"); err != nil {
		return nil, err
	}
	result := make([]interface{}, len(items))
	for i := range items {
		if nullFlagAt(nulls, int64(i)) {
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

func encodeExtensionNamePacksValue(packs []ExtensionNamePack, nulls []bool) (interface{}, error) {
	if packs == nil {
		if len(nulls) != 0 {
			return nil, fmt.Errorf("ExtensionNameList dataNulls cannot describe a nil data array")
		}
		return nil, nil
	}
	if err := validateNullFlags(nulls, int64(len(packs)), "ExtensionNameList dataNulls"); err != nil {
		return nil, err
	}
	result := make([]interface{}, len(packs))
	for i := range packs {
		if nullFlagAt(nulls, int64(i)) {
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

func encodeCatalogItem(item *CatalogItem, label string) ([]interface{}, error) {
	if err := validateNullableString(item.Name, item.NameIsNil, label+" name"); err != nil {
		return nil, err
	}
	known := []indexedKnownField{
		{name: "resourceIndex", value: item.ResourceIndex, populated: item.ResourceIndex != 0},
		{name: "name", value: nullableStringValue(item.Name, item.NameIsNil), populated: item.Name != "" || item.NameIsNil},
		{name: "hash", value: item.Hash, populated: item.Hash != 0},
	}
	return buildIndexedObject(known, item.IndexedObjectMetadata, label)
}

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

type indexedKnownField struct {
	name      string
	value     interface{}
	populated bool
}

func buildIndexedObject(known []indexedKnownField, metadata IndexedObjectMetadata, label string) ([]interface{}, error) {
	count, err := resolveIndexedObjectFieldCount(metadata.FieldCount, int64(len(known)), metadata.FutureSlots, label)
	if err != nil {
		return nil, err
	}
	for i := count; i < int64(len(known)); i++ {
		if known[i].populated {
			return nil, fmt.Errorf("%s fieldCount %d would discard %s", label, count, known[i].name)
		}
	}
	result := make([]interface{}, 0, count)
	for i := int64(0); i < count && i < int64(len(known)); i++ {
		result = append(result, known[i].value)
	}
	for _, raw := range metadata.FutureSlots {
		result = append(result, codec.Raw(append([]byte(nil), raw...)))
	}
	return result, nil
}

// setIndexedObjectMetadata 记录旧目录/目录表解码器观察到的数组宽度，并在窄化前验证 C# Int32 范围。
// setIndexedObjectMetadata records the array width observed by legacy directory/catalog decoders and validates the C# Int32 range before narrowing.
func setIndexedObjectMetadata(metadata *IndexedObjectMetadata, fields []codec.Raw, known int64) error {
	if int64(len(fields)) != known {
		count, err := checkedInt32Count(int64(len(fields)), "indexed object field count")
		if err != nil {
			return err
		}
		metadata.FieldCount = &count
	}
	if int64(len(fields)) > known {
		metadata.FutureSlots = cloneCodecRawSlots(fields[known:])
	}
	return nil
}

func decodeRawArrayOrNil(raw codec.Raw, label string) ([]codec.Raw, bool, error) {
	if isRawMsgpackNil(raw) {
		return nil, true, nil
	}
	values, err := decodeRawMsgpackArray(raw, label)
	return values, false, err
}

func decodeRawInt32(raw codec.Raw, label string) (int32, error) {
	value, err := decodeRawInterface(raw, label)
	if err != nil {
		return 0, err
	}
	decoded, ok := toInt32(value)
	if !ok {
		return 0, fmt.Errorf("%s: expected Int32 integer, got %T", label, value)
	}
	return decoded, nil
}

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

func decodeRawInterface(raw codec.Raw, label string) (interface{}, error) {
	var value interface{}
	if err := decodeSingleRawMsgpackValue(raw, &value, label); err != nil {
		return nil, err
	}
	return value, nil
}

func encodeNullableStringSlice(values []string, nulls []bool, label string) (interface{}, error) {
	if values == nil {
		if len(nulls) != 0 {
			return nil, fmt.Errorf("%s null flags cannot describe a nil array", label)
		}
		return nil, nil
	}
	if err := validateNullFlags(nulls, int64(len(values)), label+" null flags"); err != nil {
		return nil, err
	}
	result := make([]interface{}, len(values))
	for i := range values {
		if nullFlagAt(nulls, int64(i)) {
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

func nullableStringValue(value string, isNil bool) interface{} {
	if isNil {
		return nil
	}
	return value
}

func validateNullFlags(flags []bool, count int64, label string) error {
	if int64(len(flags)) > count {
		return fmt.Errorf("%s has %d entries for %d values", label, len(flags), count)
	}
	return nil
}

func nullFlagAt(flags []bool, index int64) bool {
	return index >= 0 && index < int64(len(flags)) && flags[index]
}

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

func isRawMsgpackNil(raw []byte) bool {
	return len(raw) == 0 || (len(raw) == 1 && raw[0] == 0xc0)
}

func isMsgpackStringMarker(marker byte) bool {
	return marker >= 0xa0 && marker <= 0xbf || marker == 0xd9 || marker == 0xda || marker == 0xdb
}

func indexedObjectMetadataHasWirePayload(metadata IndexedObjectMetadata) bool {
	return metadata.FieldCount != nil || len(metadata.FutureSlots) != 0
}

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

func catalogItemHasWirePayload(item *CatalogItem) bool {
	return indexedObjectMetadataHasWirePayload(item.IndexedObjectMetadata) ||
		item.ResourceIndex != 0 || item.Name != "" || item.NameIsNil || item.Hash != 0
}

func virtualCatalogItemHasWirePayload(item *VirtualCatalogItem) bool {
	return indexedObjectMetadataHasWirePayload(item.IndexedObjectMetadata) ||
		item.AssetPath != "" || item.AssetPathIsNil || item.Name != "" || item.NameIsNil || item.Hash != 0
}

func extensionNameListHasWirePayload(value *ExtensionNameList) bool {
	return indexedObjectMetadataHasWirePayload(value.IndexedObjectMetadata) ||
		value.Extension != "" || value.ExtensionIsNil || value.Data != nil || len(value.DataNulls) != 0
}

func extensionNamePackHasWirePayload(value *ExtensionNamePack) bool {
	return indexedObjectMetadataHasWirePayload(value.IndexedObjectMetadata) ||
		value.Name != "" || value.NameIsNil || value.Hash != 0
}

func toUint64(v interface{}) (uint64, bool) {
	switch n := v.(type) {
	case uint64:
		return n, true
	case int64:
		if n < 0 {
			return 0, false
		}
		return uint64(n), true
	}
	return 0, false
}

// ValidateCatalog checks only wire representability and fields that would be
// silently discarded by the selected concrete layout. Runtime lookup rules,
// enum membership, hashes, sorting, and resource-index validity belong to the
// game and are deliberately not enforced here.
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
