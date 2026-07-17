package aba

// Counts in Unity files are untrusted Int32 values. Start small so a forged
// count does not control the first allocation; append still permits every
// collection that is representable on the wire and present in the input.
const abaInitialCollectionCapacity = 1024

func makeABACountedSliceForAppend[T any](count int) []T {
	capacity := count
	if capacity > abaInitialCollectionCapacity {
		capacity = abaInitialCollectionCapacity
	}
	return make([]T, 0, capacity)
}

func makeABACountedMap[K comparable, V any](count int) map[K]V {
	capacity := count
	if capacity > abaInitialCollectionCapacity {
		capacity = abaInitialCollectionCapacity
	}
	return make(map[K]V, capacity)
}
