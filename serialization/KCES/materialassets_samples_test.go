package KCES

import "testing"

func TestMaterialAssetsSamples(t *testing.T) {
	assertPartsSamplesForSuffixRoundTrip(t, ".materialassets", DecodeMaterialAssets, EncodeMaterialAssets)
}
