package KCES

import (
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/msgpack"
)

// menuHistoricalWidths 列出游戏样本中实际出现过的 Menu indexed-array 宽度及其末尾 Key
// menuHistoricalWidths lists the Menu indexed-array widths actually observed in game samples together with their trailing keys
var menuHistoricalWidths = []struct {
	width    int
	tailKey  string
	populate func(*Menu)
	verify   func(*testing.T, *Menu)
}{
	{
		width:    21,
		tailKey:  "defineFirst",
		populate: func(menu *Menu) { menu.DefineFirst = 0x21 },
		verify: func(t *testing.T, menu *Menu) {
			if menu.DefineFirst != 0x21 {
				t.Fatalf("defineFirst = %d, want %d", menu.DefineFirst, 0x21)
			}
		},
	},
	{
		width:    22,
		tailKey:  "partsVer",
		populate: func(menu *Menu) { menu.PartsVer = &TupleStringInt{Item2: 100} },
		verify: func(t *testing.T, menu *Menu) {
			if menu.PartsVer == nil || menu.PartsVer.Item2 != 100 {
				t.Fatalf("partsVer = %+v, want item2 100", menu.PartsVer)
			}
		},
	},
	{
		width:    26,
		tailKey:  "attribute",
		populate: func(menu *Menu) { menu.Attribute = 0x26 },
		verify: func(t *testing.T, menu *Menu) {
			if menu.Attribute != 0x26 {
				t.Fatalf("attribute = %d, want %d", menu.Attribute, 0x26)
			}
		},
	},
	{
		width:    27,
		tailKey:  "hideInEdit",
		populate: func(menu *Menu) { menu.HideInEdit = true },
		verify: func(t *testing.T, menu *Menu) {
			if !menu.HideInEdit {
				t.Fatal("hideInEdit = false, want true")
			}
		},
	},
	{
		width:   28,
		tailKey: "toeLockSlotId",
		populate: func(menu *Menu) {
			slot := "toe"
			menu.ToeLockSlotId = &slot
		},
		verify: func(t *testing.T, menu *Menu) {
			if menu.ToeLockSlotId == nil || *menu.ToeLockSlotId != "toe" {
				t.Fatalf("toeLockSlotId = %v, want %q", menu.ToeLockSlotId, "toe")
			}
		},
	},
	{
		width:    30,
		tailKey:  "isHarayureAvailable",
		populate: func(menu *Menu) { menu.IsHarayureAvailable = 2 },
		verify: func(t *testing.T, menu *Menu) {
			if menu.IsHarayureAvailable != 2 {
				t.Fatalf("isHarayureAvailable = %d, want 2", menu.IsHarayureAvailable)
			}
		},
	},
	{
		width:    31,
		tailKey:  "skirt_phys",
		populate: func(menu *Menu) { menu.SkirtPhys = 3 },
		verify: func(t *testing.T, menu *Menu) {
			if menu.SkirtPhys != 3 {
				t.Fatalf("skirt_phys = %d, want 3", menu.SkirtPhys)
			}
		},
	},
	{
		width:    32,
		tailKey:  "hairMake",
		populate: func(menu *Menu) { menu.HairMake = NewHairMake() },
		verify: func(t *testing.T, menu *Menu) {
			if menu.HairMake == nil || menu.HairMake.Version != 1001 {
				t.Fatalf("hairMake = %+v, want version 1001", menu.HairMake)
			}
		},
	},
}

// TestMenuHistoricalWidthsRoundTrip 校验每个已知历史宽度都能原样写出、读回并保留该宽度末尾 Key 的值
// TestMenuHistoricalWidthsRoundTrip verifies that every known historical width encodes unchanged, decodes back, and preserves the value at that width's trailing key
func TestMenuHistoricalWidthsRoundTrip(t *testing.T) {
	for _, test := range menuHistoricalWidths {
		test := test
		t.Run(test.tailKey, func(t *testing.T) {
			menu := NewMenu()
			menu.SetMessagePackIndexedObjectWidth(int32(test.width))
			test.populate(menu)

			encoded, err := EncodeMenuAssets(&MenuAssets{Assets: []*Menu{menu}})
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
			test.verify(t, decoded.Assets[0])

			reencoded, err := EncodeMenuAssets(decoded)
			if err != nil {
				t.Fatalf("re-encode MenuAssets: %v", err)
			}
			if got := nestedCompressedArrayWidth(t, reencoded, 1, 0); got != test.width {
				t.Fatalf("re-encoded Menu width = %d, want %d", got, test.width)
			}
		})
	}
}

// TestMenuHistoricalWidthsMapGameWireSlots 校验游戏线格式中被截断的宽度按前缀映射到字段，且宽度之外的 Key 保持零值
// 槽位取值参考真实样本：width 26 来自 parts_dlc437hair_gp003，width 30 来自 parts_cas097_gp003
// TestMenuHistoricalWidthsMapGameWireSlots verifies that truncated widths in the game wire format map onto fields as a prefix and that keys beyond the width stay zero
// The slot values follow real samples, with width 26 from parts_dlc437hair_gp003 and width 30 from parts_cas097_gp003
func TestMenuHistoricalWidthsMapGameWireSlots(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		verify func(*testing.T, *Menu)
	}{
		{
			name:  "width 26 ends at attribute",
			width: 26,
			verify: func(t *testing.T, menu *Menu) {
				if menu.Attribute != 1 {
					t.Fatalf("attribute = %d, want 1", menu.Attribute)
				}
				if menu.HideInEdit || menu.ToeLockSlotId != nil || menu.SkirtPhys != 0 || menu.HairMake != nil {
					t.Fatalf("keys beyond width 26 are not zero: %+v", menu)
				}
			},
		},
		{
			name:  "width 30 ends at isHarayureAvailable",
			width: 30,
			verify: func(t *testing.T, menu *Menu) {
				if menu.IsHarayureAvailable != 1 {
					t.Fatalf("isHarayureAvailable = %d, want 1", menu.IsHarayureAvailable)
				}
				if menu.SkirtPhys != 0 || menu.HairMake != nil {
					t.Fatalf("keys beyond width 30 are not zero: %+v", menu)
				}
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			decoded, err := DecodeMenuAssets(compressedMenuAssetsTestWire(t, gameMenuTestSlots(test.width)))
			if err != nil {
				t.Fatalf("DecodeMenuAssets: %v", err)
			}
			if got := int(decoded.Assets[0].MessagePackIndexedObjectWidth()); got != test.width {
				t.Fatalf("decoded Menu width = %d, want %d", got, test.width)
			}
			if decoded.Assets[0].FileName == nil || *decoded.Assets[0].FileName != "historical.menu" {
				t.Fatalf("fileName = %v, want %q", decoded.Assets[0].FileName, "historical.menu")
			}
			test.verify(t, decoded.Assets[0])
		})
	}
}

// TestMenuRejectsUnobservedIndexedArrayWidth 校验样本中未出现过的宽度仍被拒绝，因为其槽位语义无法核实
// TestMenuRejectsUnobservedIndexedArrayWidth verifies that widths absent from the samples are still rejected because their slot semantics cannot be verified
func TestMenuRejectsUnobservedIndexedArrayWidth(t *testing.T) {
	for _, width := range []int{20, 23, 24, 25, 29, 33} {
		wire := compressedMenuAssetsTestWire(t, gameMenuTestSlots(width))
		_, err := DecodeMenuAssets(wire)
		if err == nil || !strings.Contains(err.Error(), "unsupported Menu indexed-array width") {
			t.Fatalf("DecodeMenuAssets(width %d) error = %v, want unsupported-width rejection", width, err)
		}
	}
}

// gameMenuTestSlots 按游戏线格式构造截断到指定宽度的 Menu 槽位数组
// gameMenuTestSlots builds a Menu slot array in the game wire format truncated to the requested width
func gameMenuTestSlots(width int) []interface{} {
	slots := []interface{}{
		int64(1003),                    // [0]  version
		uint64(0x1122334455667788),     // [1]  guid
		uint64(0x8877665544332211),     // [2]  id
		"historical.menu",              // [3]  fileName
		"Historical",                   // [4]  itemName
		"historical_i_",                // [5]  iconFileName
		"info",                         // [6]  infoText
		int64(3),                       // [7]  priority
		int64(0),                       // [8]  parentId
		false,                          // [9]  isMan
		false,                          // [10] isDiff
		false,                          // [11] isDelete
		[]interface{}{},                // [12] commandList
		"hairr",                        // [13] categoryText
		"haircolor",                    // [14] colorSetText
		int64(10),                      // [15] defineTagNames
		nil,                            // [16] preMulTexDatas
		nil,                            // [17] colvariFileNameExp
		nil,                            // [18] colvariInfo
		uint64(3208329972),             // [19] srcFileHashCRC32
		int64(2),                       // [20] defineFirst
		[]interface{}{nil, int64(100)}, // [21] partsVer
		false,                          // [22] isRecommendMan
		int64(0),                       // [23] targetBodyType
		nil,                            // [24] reserved
		int64(1),                       // [25] attribute
		false,                          // [26] hideInEdit
		nil,                            // [27] toeLockSlotId
		nil,                            // [28] exportModelFormTextureName
		int64(1),                       // [29] isHarayureAvailable
		int64(0),                       // [30] skirt_phys
		nil,                            // [31] hairMake
	}
	if width > len(slots) {
		return append(slots, make([]interface{}, width-len(slots))...)
	}
	return slots[:width]
}

// compressedMenuAssetsTestWire 将 Menu 槽位数组包装为 MenuAssets 容器并压缩为游戏线格式
// compressedMenuAssetsTestWire wraps a Menu slot array in a MenuAssets container and compresses it into the game wire format
func compressedMenuAssetsTestWire(t *testing.T, slots []interface{}) []byte {
	t.Helper()
	root := []interface{}{"historical.menuassets", []interface{}{slots}}
	encoded, err := msgpack.EncodeMsgpack(root)
	if err != nil {
		t.Fatalf("EncodeMsgpack: %v", err)
	}
	compressed, err := msgpack.CompressLz4BlockArray(encoded)
	if err != nil {
		t.Fatalf("CompressLz4BlockArray: %v", err)
	}
	return compressed
}
