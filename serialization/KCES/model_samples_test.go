package KCES

import "testing"

func TestModelSamples(t *testing.T) {
	assertPartsSamplesForSuffixRoundTrip(t, ".model", DecodeModel, EncodeModel)
}
