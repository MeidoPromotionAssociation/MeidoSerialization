package KCES

import "testing"

func TestMenuAssetsSamples(t *testing.T) {
	// Recalculation is disabled so the layout comparison stays deterministic:
	// a Menu without HairMake.ExportedGUID gets a fresh random GUID on every encode.
	encode := func(assets *MenuAssets) ([]byte, error) {
		return EncodeMenuAssetsWithOptions(assets, &LookupHashOptions{RecalculateHash: false})
	}
	assertPartsSamplesForSuffixRoundTrip(t, ".menuassets", DecodeMenuAssets, encode)
}
