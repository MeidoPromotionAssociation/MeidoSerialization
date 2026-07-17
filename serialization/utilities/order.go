package utilities

import (
	"cmp"
	"fmt"
	"slices"
)

// MergeOrderedMapKeys treats order as optional round-trip metadata. Keys that
// are still present retain their recorded order, stale keys are ignored, and
// newly added keys are appended in their natural order. A duplicate live key
// remains an error because it cannot be emitted twice while retaining the
// map's declared count.
func MergeOrderedMapKeys[K cmp.Ordered, V any](values map[K]V, order []K, label string) ([]K, error) {
	keys := make([]K, 0, len(values))
	seen := make(map[K]struct{}, len(order))
	for index, key := range order {
		if _, exists := values[key]; !exists {
			continue
		}
		if _, duplicate := seen[key]; duplicate {
			return nil, fmt.Errorf("%s[%d] duplicates key %v", label, index, key)
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}

	missing := make([]K, 0, len(values)-len(keys))
	for key := range values {
		if _, exists := seen[key]; !exists {
			missing = append(missing, key)
		}
	}
	slices.Sort(missing)
	return append(keys, missing...), nil
}
