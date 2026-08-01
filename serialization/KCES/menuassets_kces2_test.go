package KCES

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMenuLayoutRoundTripPreservesKCESGeneration(t *testing.T) {
	tests := []struct {
		name    string
		menu    *Menu
		width   int
		version int32
	}{
		{name: "KCES", menu: NewMenu(), width: menuLegacyWidth, version: 901},
		{name: "KCES2", menu: NewKCES2Menu(), width: menuKCES2Width, version: 902},
	}
	for index := range tests {
		tests[index].menu.Version = tests[index].version
	}
	tests[1].menu.HairMake = NewHairMake()
	tests[1].menu.HairMake.Version = 903
	tests[1].menu.HairMake.ExportedHairBuildVersion = 904
	tests[1].menu.HairMake.ExportedHairGameVersion = 905

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := EncodeMenuAssets(&MenuAssets{Assets: []*Menu{test.menu}})
			if err != nil {
				t.Fatalf("EncodeMenuAssets: %v", err)
			}
			if got := nestedCompressedArrayWidth(t, encoded, 1, 0); got != test.width {
				t.Fatalf("encoded Menu width = %d, want %d", got, test.width)
			}

			decoded, err := DecodeMenuAssets(encoded)
			if err != nil {
				t.Fatalf("DecodeMenuAssets: %v", err)
			}
			if got := int(decoded.Assets[0].MessagePackIndexedObjectWidth()); got != test.width {
				t.Fatalf("decoded Menu width = %d, want %d", got, test.width)
			}
			if decoded.Assets[0].Version != test.version {
				t.Fatalf("decoded Menu version = %d, want %d", decoded.Assets[0].Version, test.version)
			}
			if test.name == "KCES2" && decoded.Assets[0].HairMake.Version != 903 {
				t.Fatalf("decoded HairMake version = %d, want 903", decoded.Assets[0].HairMake.Version)
			}

			jsonData, err := json.Marshal(decoded)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if test.name == "KCES2" {
				text := string(jsonData)
				if !strings.Contains(text, `"exportedHairBuildVersion":904`) || !strings.Contains(text, `"exportedHairGameVersion":905`) {
					t.Fatalf("corrected HairMake field names are missing: %s", text)
				}
				if strings.Contains(text, "exporedHair") {
					t.Fatalf("misspelled HairMake field name leaked into editing JSON: %s", text)
				}
			}
			var edited MenuAssets
			if err := json.Unmarshal(jsonData, &edited); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			reencoded, err := EncodeMenuAssets(&edited)
			if err != nil {
				t.Fatalf("re-encode MenuAssets: %v", err)
			}
			if got := nestedCompressedArrayWidth(t, reencoded, 1, 0); got != test.width {
				t.Fatalf("re-encoded Menu width = %d, want %d", got, test.width)
			}
		})
	}
}

func TestHistoricalMenuLayoutRejectsKCES2TailField(t *testing.T) {
	menu := NewMenu()
	menu.HairMake = NewHairMake()
	if _, err := EncodeMenuAssets(&MenuAssets{Assets: []*Menu{menu}}); err == nil || !strings.Contains(err.Error(), "not representable") {
		t.Fatalf("EncodeMenuAssets error = %v, want unrepresentable-tail error", err)
	}
}
