package KCES

// KCES 各二进制格式共用的不可信数量安全分配策略
// Safe allocation strategy for untrusted collection counts shared by KCES binary formats

// KCES 文件通常以不可信的 Int32 或 UInt32 保存集合大小，较小的初始容量可避免按恶意声明值直接分配内存，同时 append 仍可容纳线格式能够表示且输入中实际存在的全部元素
// KCES files commonly carry collection sizes as untrusted Int32 or UInt32 values, and a small initial capacity avoids allocating directly from a hostile declared count while append still permits every collection representable on the wire and actually present in the input
const kcesInitialCollectionCapacity = 1024

// makeKCESCountedSliceForAppend 创建容量受限但可继续追加的切片
// makeKCESCountedSliceForAppend creates an appendable slice with a capped initial capacity
func makeKCESCountedSliceForAppend[T any](count uint64) []T {
	capacity := count
	if capacity > kcesInitialCollectionCapacity {
		capacity = kcesInitialCollectionCapacity
	}
	return make([]T, 0, int(capacity))
}

// makeKCESCountedMap 创建容量受限的映射表
// makeKCESCountedMap creates a map with a capped initial capacity
func makeKCESCountedMap[K comparable, V any](count uint64) map[K]V {
	capacity := count
	if capacity > kcesInitialCollectionCapacity {
		capacity = kcesInitialCollectionCapacity
	}
	return make(map[K]V, int(capacity))
}
