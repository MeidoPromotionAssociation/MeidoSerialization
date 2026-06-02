package KCES

import "testing"

func TestPriorityMaterialAssetsSamples(t *testing.T) {
	assertPartsSamplesForSuffixRoundTrip(t, ".pmatassets", DecodePriorityMaterialAssets, EncodePriorityMaterialAssets)
}
