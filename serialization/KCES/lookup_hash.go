package KCES

// LookupHashOptions 控制 KCES 部件写出时是否重算由名称派生的哈希字段，未提供选项时默认重算 / LookupHashOptions controls whether KCES part writers recalculate hash fields derived from names, with recalculation enabled by default when options are omitted
type LookupHashOptions struct {
	RecalculateHash bool   `json:"-"` // 是否在写出时重算 ID 与 GUID，显式提供 false 可保留原值；Menu.GUID 缺少 HairMake.ExportedGUID 来源时按新 UUID v4 重算，因此写出结果不可复现 / Whether to recalculate IDs and GUIDs during encoding, with an explicitly supplied false preserving stored values; Menu.GUID is recalculated from a fresh UUID v4 when the HairMake.ExportedGUID source is absent, so those writes are not reproducible
	FileName        string `json:"-"` // 单对象重算时写入副本的规范 wire 文件名，服务层应由目标路径推导，空值表示使用结构中的 FileName / Canonical wire filename written into the output copy during single-object recalculation, derived by the service from the destination path, with an empty value using the structure's FileName
}

// ShouldRecalculateLookupHashes 判断派生哈希字段是否应按默认值或显式选项重算
// ShouldRecalculateLookupHashes reports whether derived hash fields should be recalculated under the default or explicitly selected behavior
func ShouldRecalculateLookupHashes(options *LookupHashOptions) bool {
	return options == nil || options.RecalculateHash
}
