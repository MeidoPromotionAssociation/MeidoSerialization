package ct

// AssetBundleCatalog 表示 .ct 文件中 "catalog" 虚拟文件的反序列化结构 / AssetBundleCatalog represents the deserialized "catalog" virtual file inside a .ct file
// 对应 C# WfSystem.Catalog.AssetBundleCatalog，使用 MessagePack indexed array 序列化 / Matches C# WfSystem.Catalog.AssetBundleCatalog serialized as a MessagePack indexed array
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
	Version           int                `json:"version"`           // 固定为 1000，对应 C# FixVersion / Fixed to 1000, matching the C# FixVersion
	CatalogType       CatalogType        `json:"catalogType"`       // 资源分类标志位 Flags 枚举，如 Parts=4096 / Resource category flag enum such as Parts=4096
	PackageType       CatalogPackageType `json:"packageType"`       // 包类型枚举，如 Base=0, Plugin=1 / Package type enum such as Base=0, Plugin=1
	Priority          int                `json:"priority"`          // 加载优先级，数值越大优先级越高 / Load priority, higher values load earlier
	Name              string             `json:"name"`              // catalog 名称，通常与 .ct 文件名一致且不含扩展名 / Catalog name, usually matching the .ct file name without extension
	SubName           string             `json:"subName"`           // 子名称，通常为空 / Sub name, usually empty
	Hash              uint64             `json:"hash"`              // catalog 自身的 FNV-1a hash / FNV-1a hash of the catalog itself
	CreateTime        int64              `json:"createTime"`        // 创建时间戳 / Creation timestamp
	IsEncrypted       bool               `json:"isEncrypted"`       // 是否为加密包 abap 格式 / Whether this is an encrypted abap package
	ResourceFileNames []string           `json:"resourceFileNames"` // 关联的资源文件名列表，如 name.aba / Related resource file-name list such as name.aba
	ExtensionList     []string           `json:"extensionList"`     // 扩展名列表，每个扩展名对应 .ct 中一个同名 ExtensionNameList 文件 / Extension list, each extension maps to a same-name ExtensionNameList virtual file
	Items             []CatalogItem      `json:"items"`             // 资源索引条目数组，按 hash 排序用于二分查找 / Resource index item array, sorted by hash for binary search
}

// CatalogItem 表示 catalog 中的单个资源索引条目 / CatalogItem represents one resource index item in the catalog
// 对应 C# AssetBundleCatalog.Item，MessagePack indexed array / Matches C# AssetBundleCatalog.Item as a MessagePack indexed array:
//
//	[0] resourceIndex  int     指向 resourceFileNames 的索引
//	[1] name           string  资源名称（如 "xxx.menuassets"）
//	[2] hash           uint64  资源名称的 FNV-1a ignore-case hash
type CatalogItem struct {
	ResourceIndex int    `json:"resourceIndex"` // 指向 AssetBundleCatalog.ResourceFileNames 的索引 / Index into AssetBundleCatalog.ResourceFileNames
	Name          string `json:"name"`          // 资源名称，游戏通过此名称加载资源 / Resource name used by the game to load the asset
	Hash          uint64 `json:"hash"`          // 资源名称的 FNV-1a 64-bit ignore-case hash，用于快速查找 / FNV-1a 64-bit ignore-case hash of the resource name for fast lookup
}

// ExtensionNameList 表示 .ct 中按扩展名分组的资源名称列表 / ExtensionNameList represents resource names grouped by extension inside a .ct file
// 对应 C# AssetBundleCatalog.ExtensionNameList，MessagePack indexed array / Matches C# AssetBundleCatalog.ExtensionNameList as a MessagePack indexed array:
//
//	[0] extention  string  扩展名（如 ".menuassets"）
//	[1] data       []Pack  名称+hash 列表
//
// 游戏通过 GetFileNameListFromExtension 获取某扩展名下的所有资源名 / The game uses GetFileNameListFromExtension to enumerate resource names for an extension
type ExtensionNameList struct {
	Extension string              `json:"extention"` // 扩展名，如 .menuassets、.tex、.model，字段名保留游戏 extention 拼写 / Extension such as .menuassets, .tex, or .model, keeping the game's extention spelling
	Data      []ExtensionNamePack `json:"data"`      // 该扩展名下的所有资源名称及其 hash / Resource names and hashes under this extension
}

// ExtensionNamePack 表示 ExtensionNameList 中的单个条目 / ExtensionNamePack represents one item in ExtensionNameList
// 对应 C# AssetBundleCatalog.ExtensionNameList.Pack，MessagePack indexed array / Matches C# AssetBundleCatalog.ExtensionNameList.Pack as a MessagePack indexed array:
//
//	[0] name  string  资源名称（不含扩展名）
//	[1] hash  uint64  名称的 FNV-1a ignore-case hash
type ExtensionNamePack struct {
	Name string `json:"name"` // 资源名称 / Resource name
	Hash uint64 `json:"hash"` // 名称的 FNV-1a 64-bit ignore-case hash / FNV-1a 64-bit ignore-case hash of the name
}

// CatalogType 资源分类标志位枚举（Flags）。
// 对应 C# WfSystem.Catalog.CatalogType
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

// CatalogPackageType 包类型枚举。
// 对应 C# WfSystem.Catalog.CatalogPackageType
type CatalogPackageType int

const (
	PackageTypeBase        CatalogPackageType = 0
	PackageTypePlugin      CatalogPackageType = 1
	PackageTypePluginPatch CatalogPackageType = 2
	PackageTypeBasePatch   CatalogPackageType = 3
	PackageTypeExtraBase   CatalogPackageType = 4
	PackageTypeExtraPatch  CatalogPackageType = 5
)

// DecodeCatalog 从 MessagePack indexed array 解码 AssetBundleCatalog。
// 输入应为 DecompressLz4BlockArray 解压后的原始 MessagePack 数据。
func DecodeCatalog(data []byte) (*AssetBundleCatalog, error) {
	var raw []interface{}
	if err := DecodeMsgpack(data, &raw); err != nil {
		return nil, err
	}
	return decodeCatalogFromArray(raw)
}

// DecodeCatalogFromCt 从 ContentTable 中读取 "catalog" 文件并解码。
// 自动处理 Lz4BlockArray 解压。
func DecodeCatalogFromCt(table *ContentTable) (*AssetBundleCatalog, error) {
	var raw []interface{}
	if err := table.DecodeMsgpackFile("catalog", &raw); err != nil {
		return nil, err
	}
	return decodeCatalogFromArray(raw)
}

func decodeCatalogFromArray(arr []interface{}) (*AssetBundleCatalog, error) {
	if len(arr) < 12 {
		arr = padArray(arr, 12)
	}

	cat := &AssetBundleCatalog{}
	if v, ok := toInt(arr[0]); ok {
		cat.Version = v
	}
	if v, ok := toInt(arr[1]); ok {
		cat.CatalogType = CatalogType(v)
	}
	if v, ok := toInt(arr[2]); ok {
		cat.PackageType = CatalogPackageType(v)
	}
	if v, ok := toInt(arr[3]); ok {
		cat.Priority = v
	}
	if v, ok := arr[4].(string); ok {
		cat.Name = v
	}
	if v, ok := arr[5].(string); ok {
		cat.SubName = v
	}
	if v, ok := toUint64(arr[6]); ok {
		cat.Hash = v
	}
	if v, ok := toInt64(arr[7]); ok {
		cat.CreateTime = v
	}
	if v, ok := arr[8].(bool); ok {
		cat.IsEncrypted = v
	}
	if v, ok := arr[9].([]interface{}); ok {
		cat.ResourceFileNames = toStringSlice(v)
	}
	if v, ok := arr[10].([]interface{}); ok {
		cat.ExtensionList = toStringSlice(v)
	}
	if v, ok := arr[11].([]interface{}); ok {
		cat.Items = decodeCatalogItems(v)
	}

	return cat, nil
}

// EncodeCatalog 将 AssetBundleCatalog 编码为 MessagePack indexed array 字节。
func EncodeCatalog(cat *AssetBundleCatalog) ([]byte, error) {
	arr := make([]interface{}, 12)
	arr[0] = int64(cat.Version)
	arr[1] = int64(cat.CatalogType)
	arr[2] = int64(cat.PackageType)
	arr[3] = int64(cat.Priority)
	arr[4] = cat.Name
	arr[5] = cat.SubName
	arr[6] = cat.Hash
	arr[7] = cat.CreateTime
	arr[8] = cat.IsEncrypted
	arr[9] = toInterfaceSlice(cat.ResourceFileNames)
	arr[10] = toInterfaceSlice(cat.ExtensionList)
	arr[11] = encodeCatalogItems(cat.Items)
	return EncodeMsgpack(arr)
}

// DecodeExtensionNameList 从 MessagePack indexed array 解码 ExtensionNameList。
func DecodeExtensionNameList(data []byte) (*ExtensionNameList, error) {
	var raw []interface{}
	if err := DecodeMsgpack(data, &raw); err != nil {
		return nil, err
	}
	return decodeExtNameListFromArray(raw), nil
}

// DecodeExtensionNameListFromCt 从 ContentTable 中读取指定扩展名文件并解码。
func DecodeExtensionNameListFromCt(table *ContentTable, extension string) (*ExtensionNameList, error) {
	var raw []interface{}
	if err := table.DecodeMsgpackFile(extension, &raw); err != nil {
		return nil, err
	}
	return decodeExtNameListFromArray(raw), nil
}

// EncodeExtensionNameList 将 ExtensionNameList 编码为 MessagePack indexed array 字节。
func EncodeExtensionNameList(enl *ExtensionNameList) ([]byte, error) {
	packs := make([]interface{}, len(enl.Data))
	for i, p := range enl.Data {
		packs[i] = []interface{}{p.Name, p.Hash}
	}
	arr := []interface{}{enl.Extension, packs}
	return EncodeMsgpack(arr)
}

func decodeExtNameListFromArray(arr []interface{}) *ExtensionNameList {
	enl := &ExtensionNameList{}
	if len(arr) >= 1 {
		if v, ok := arr[0].(string); ok {
			enl.Extension = v
		}
	}
	if len(arr) >= 2 {
		if v, ok := arr[1].([]interface{}); ok {
			enl.Data = make([]ExtensionNamePack, 0, len(v))
			for _, item := range v {
				if packArr, ok := item.([]interface{}); ok && len(packArr) >= 2 {
					pack := ExtensionNamePack{}
					if name, ok := packArr[0].(string); ok {
						pack.Name = name
					}
					if h, ok := toUint64(packArr[1]); ok {
						pack.Hash = h
					}
					enl.Data = append(enl.Data, pack)
				}
			}
		}
	}
	return enl
}

func decodeCatalogItems(arr []interface{}) []CatalogItem {
	items := make([]CatalogItem, 0, len(arr))
	for _, elem := range arr {
		itemArr, ok := elem.([]interface{})
		if !ok || len(itemArr) < 3 {
			continue
		}
		item := CatalogItem{}
		if v, ok := toInt(itemArr[0]); ok {
			item.ResourceIndex = v
		}
		if v, ok := itemArr[1].(string); ok {
			item.Name = v
		}
		if v, ok := toUint64(itemArr[2]); ok {
			item.Hash = v
		}
		items = append(items, item)
	}
	return items
}

func encodeCatalogItems(items []CatalogItem) []interface{} {
	arr := make([]interface{}, len(items))
	for i, item := range items {
		arr[i] = []interface{}{int64(item.ResourceIndex), item.Name, item.Hash}
	}
	return arr
}

func toUint64(v interface{}) (uint64, bool) {
	switch n := v.(type) {
	case uint64:
		return n, true
	case int64:
		return uint64(n), true
	case int:
		return uint64(n), true
	case uint:
		return uint64(n), true
	}
	return 0, false
}

func toStringSlice(arr []interface{}) []string {
	result := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

func toInterfaceSlice(ss []string) []interface{} {
	arr := make([]interface{}, len(ss))
	for i, s := range ss {
		arr[i] = s
	}
	return arr
}

func padArray(arr []interface{}, size int) []interface{} {
	if len(arr) >= size {
		return arr
	}
	padded := make([]interface{}, size)
	copy(padded, arr)
	return padded
}
