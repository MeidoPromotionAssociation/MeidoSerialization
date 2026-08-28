package KCES

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/ct"
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

func TestEncodeMenuAssetsLookupFieldsRecalculateByDefaultAndCanBeDisabled(t *testing.T) {
	fileName := "MixedCase.menu"
	exportedGUID := "ABCDEF01-2345-6789-ABCD-EF0123456789"
	menu := NewKCES2Menu()
	menu.FileName = &fileName
	menu.ID = 1
	menu.GUID = 2
	menu.HairMake = NewHairMake()
	menu.HairMake.ExportedGUID = &exportedGUID
	assets := &MenuAssets{Assets: []*Menu{menu}}

	defaultWire, err := EncodeMenuAssets(assets)
	if err != nil {
		t.Fatalf("EncodeMenuAssets: %v", err)
	}
	defaultValue, err := DecodeMenuAssets(defaultWire)
	if err != nil {
		t.Fatalf("DecodeMenuAssets: %v", err)
	}
	if got, want := defaultValue.Assets[0].ID, ct.HashStringIgnoreCase(fileName); got != want {
		t.Fatalf("default ID = %d, want %d", got, want)
	}
	if got, want := defaultValue.Assets[0].GUID, ct.HashStringIgnoreCase(exportedGUID); got != want {
		t.Fatalf("default GUID = %d, want %d", got, want)
	}

	preservedWire, err := EncodeMenuAssetsWithOptions(assets, &LookupHashOptions{RecalculateHash: false})
	if err != nil {
		t.Fatalf("EncodeMenuAssetsWithOptions preserve: %v", err)
	}
	preserved, err := DecodeMenuAssets(preservedWire)
	if err != nil {
		t.Fatalf("DecodeMenuAssets preserve: %v", err)
	}
	if preserved.Assets[0].ID != 1 || preserved.Assets[0].GUID != 2 {
		t.Fatalf("disabled recalculation changed lookup fields: ID=%d GUID=%d", preserved.Assets[0].ID, preserved.Assets[0].GUID)
	}
	if menu.ID != 1 || menu.GUID != 2 {
		t.Fatalf("encoding mutated input lookup fields: ID=%d GUID=%d", menu.ID, menu.GUID)
	}
}

func TestEncodeMenuAssetsRegeneratesGUIDPerMenuWithoutExportedGUID(t *testing.T) {
	firstName := "first.menu"
	secondName := "second.menu"
	first := NewMenu()
	first.FileName = &firstName
	first.GUID = 7
	second := NewMenu()
	second.FileName = &secondName
	second.GUID = 7
	assets := &MenuAssets{Assets: []*Menu{first, second}}

	wire, err := EncodeMenuAssets(assets)
	if err != nil {
		t.Fatalf("EncodeMenuAssets: %v", err)
	}
	decoded, err := DecodeMenuAssets(wire)
	if err != nil {
		t.Fatalf("DecodeMenuAssets: %v", err)
	}
	for index, menu := range decoded.Assets {
		if menu.GUID == 7 || menu.GUID == 0 {
			t.Fatalf("menu[%d] GUID = %d, want a regenerated non-zero value", index, menu.GUID)
		}
	}
	if decoded.Assets[0].GUID == decoded.Assets[1].GUID {
		t.Fatalf("both menus share regenerated GUID %d", decoded.Assets[0].GUID)
	}
	if first.GUID != 7 || second.GUID != 7 {
		t.Fatalf("encoding mutated input GUIDs: %d %d", first.GUID, second.GUID)
	}
}

func TestEncodeMenuAssetsRejectsFileNameWithoutMenuExtension(t *testing.T) {
	invalid := []string{"testmenu", "", "testmenu.tex", "testmenu.menu.bak", "menu"}
	for _, name := range invalid {
		name := name
		menu := NewMenu()
		menu.FileName = &name
		assets := &MenuAssets{Assets: []*Menu{menu}}
		if _, err := EncodeMenuAssets(assets); err == nil || !strings.Contains(err.Error(), ".menu") {
			t.Fatalf("EncodeMenuAssets(%q) error = %v, want missing-extension error", name, err)
		}
		if _, err := EncodeMenuAssetsWithOptions(assets, &LookupHashOptions{RecalculateHash: false}); err == nil {
			t.Fatalf("EncodeMenuAssetsWithOptions(%q, preserve) accepted an extensionless filename", name)
		}
	}

	valid := []string{"testmenu.menu", "TestMenu.MENU", "hair.kcmenu", "Hair.KCMenu"}
	for _, name := range valid {
		name := name
		menu := NewKCES2Menu()
		menu.FileName = &name
		if _, err := EncodeMenuAssets(&MenuAssets{Assets: []*Menu{menu}}); err != nil {
			t.Fatalf("EncodeMenuAssets(%q): %v", name, err)
		}
	}

	if _, err := EncodeMenuAssets(&MenuAssets{Assets: []*Menu{{Version: menuFixVersion}}}); err != nil {
		t.Fatalf("EncodeMenuAssets(nil fileName): %v", err)
	}
}

func TestEncodeMenuAssetsGUIDMatchesGameHashOfUUIDSource(t *testing.T) {
	// The game derives GUID as GetHashIgnoreCase over a D-format UUID string,
	// so a regenerated GUID must be reachable from some canonical UUID text.
	fileName := "uuid_source.menu"
	exported := "3f2504e0-4f89-41d3-9a0c-0305e82c3301"
	menu := NewKCES2Menu()
	menu.FileName = &fileName
	menu.HairMake = NewHairMake()
	menu.HairMake.ExportedGUID = &exported

	wire, err := EncodeMenuAssets(&MenuAssets{Assets: []*Menu{menu}})
	if err != nil {
		t.Fatalf("EncodeMenuAssets: %v", err)
	}
	decoded, err := DecodeMenuAssets(wire)
	if err != nil {
		t.Fatalf("DecodeMenuAssets: %v", err)
	}
	if got, want := decoded.Assets[0].GUID, ct.HashStringIgnoreCase(exported); got != want {
		t.Fatalf("GUID = %d, want %d", got, want)
	}
	if got, want := decoded.Assets[0].GUID, ct.HashStringIgnoreCase(strings.ToUpper(exported)); got != want {
		t.Fatalf("GUID is case sensitive: %d vs %d", got, want)
	}
}
