package KCES

// KCES files commonly carry collection sizes as untrusted Int32/UInt32
// values. Starting with a small capacity avoids allocating directly from a
// hostile declared count, while append still permits every collection that is
// representable on the wire and actually present in the input.
const kcesInitialCollectionCapacity = 1024

func makeKCESCountedSliceForAppend[T any](count uint64) []T {
	capacity := count
	if capacity > kcesInitialCollectionCapacity {
		capacity = kcesInitialCollectionCapacity
	}
	return make([]T, 0, int(capacity))
}

func makeKCESCountedMap[K comparable, V any](count uint64) map[K]V {
	capacity := count
	if capacity > kcesInitialCollectionCapacity {
		capacity = kcesInitialCollectionCapacity
	}
	return make(map[K]V, int(capacity))
}
