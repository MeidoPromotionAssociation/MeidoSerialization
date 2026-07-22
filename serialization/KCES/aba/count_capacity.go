package aba

// Unity 文件中的计数是不可受信任的 Int32 值，初始容量设为较小上限以避免伪造计数控制首次分配
// 后续 append 仍允许读取输入中实际存在且可由线格式表示的全部集合元素
// Counts in Unity files are untrusted Int32 values, so the initial capacity is capped to prevent a forged count from controlling the first allocation
// Subsequent append calls still permit every collection actually present in the input and representable on the wire
const abaInitialCollectionCapacity = 1024

// makeABACountedSliceForAppend 创建按线格式计数追加元素且限制初始容量的切片
// makeABACountedSliceForAppend creates a slice for appending a wire-counted collection with capped initial capacity
func makeABACountedSliceForAppend[T any](count int) []T {
	capacity := count
	if capacity > abaInitialCollectionCapacity {
		capacity = abaInitialCollectionCapacity
	}
	return make([]T, 0, capacity)
}

// makeABACountedMap 创建按线格式计数填充且限制初始容量的映射
// makeABACountedMap creates a map for a wire-counted collection with capped initial capacity
func makeABACountedMap[K comparable, V any](count int) map[K]V {
	capacity := count
	if capacity > abaInitialCollectionCapacity {
		capacity = abaInitialCollectionCapacity
	}
	return make(map[K]V, capacity)
}
