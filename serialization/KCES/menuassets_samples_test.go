package KCES

import "testing"

func TestMenuAssetsSamples(t *testing.T) {
	assertPartsSamplesForSuffixRoundTrip(t, ".menuassets", DecodeMenuAssets, EncodeMenuAssets)
}
