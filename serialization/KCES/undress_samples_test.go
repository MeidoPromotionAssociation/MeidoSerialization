package KCES

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/internal/kcesfixtures"
)

// assertUndressDataSampleFields 校验已知 .undressdat 样本的实测成员值
// assertUndressDataSampleFields checks measured member values of a known .undressdat sample
func assertUndressDataSampleFields(t *testing.T, name string, value *UndressArchiveTarget) {
	t.Helper()
	if value.Format == nil || *value.Format == "" {
		t.Fatalf("%s has no format member, which the game needs to run its migrations", name)
	}
	if value.DataGroup == nil || len(*value.DataGroup) == 0 {
		t.Fatalf("%s has no vertex groups", name)
	}
	if value.Layers == nil || len(*value.Layers) == 0 {
		t.Fatalf("%s has no peel channels", name)
	}
	// (layer, label) 是 OneGroupLooker 字典的键，因此必须成对存在且在全文件内唯一
	// 标签本身会在不同 layer 之间重复，子网格层的标签甚至是空串，所以单独的 label 不是标识
	// (layer, label) is the key of the OneGroupLooker dictionary, so it must be present as a pair and unique across the file
	// Labels themselves repeat across layers and the sub-mesh layer even uses an empty one, so a label alone is not an identity
	seen := make(map[string]int, len(*value.DataGroup))
	for index, group := range *value.DataGroup {
		if group.Label == nil || group.Layer == nil {
			t.Fatalf("%s dataGroup[%d] has an incomplete (layer, label) key: %+v", name, index, group)
		}
		key := strconv.FormatInt(int64(*group.Layer), 10) + "/" + *group.Label
		if previous, duplicate := seen[key]; duplicate {
			t.Fatalf("%s dataGroup[%d] repeats the (layer, label) key of dataGroup[%d]: %s", name, index, previous, key)
		}
		seen[key] = index
	}
	if name != "crc2_Underwear038_pants.undressdat" {
		return
	}
	if *value.Format != "1.2.2" || value.EditVer == nil || *value.EditVer != 13 {
		t.Fatalf("%s version members = format %q editVer %v, want 1.2.2/13", name, *value.Format, value.EditVer)
	}
	if value.SetupDataType == nil || *value.SetupDataType != 2 || value.PeelCategory == nil || *value.PeelCategory != 1 {
		t.Fatalf("%s classification = setupDataType %v peelCategory %v, want 2 (Pants)/1 (Shorts)", name, value.SetupDataType, value.PeelCategory)
	}
	if value.MeshRelPath == nil || *value.MeshRelPath != "crc2_Underwear038" || value.FbxName == nil || *value.FbxName != "crc2_Underwear038_pants" {
		t.Fatalf("%s resource references = %v/%v", name, value.MeshRelPath, value.FbxName)
	}
	if len(*value.Layers) != 15 || len(*value.DataGroup) != 263 {
		t.Fatalf("%s collection sizes = %d layers, %d groups, want 15/263", name, len(*value.Layers), len(*value.DataGroup))
	}
	// 样本覆盖了纵、纵逆、股间、前横、后横以及已废弃的 50，未使用的通道则是 0
	// The sample covers vertical, vertical reverse, crotch, front sideways, back sideways, and the obsolete 50, while unused channels are 0
	wantUseModes := []int32{10, 0, 14, 0, 30, 20, 50, 40, 21, 31, 41, 0, 0, 0, 0}
	for index, layer := range *value.Layers {
		if layer.UseMode == nil || *layer.UseMode != wantUseModes[index] {
			t.Fatalf("%s layers[%d].useMode = %v, want %d", name, index, layer.UseMode, wantUseModes[index])
		}
	}
	if value.HPeelLimits == nil || value.HPeelLimits.FormatVersion == nil || *value.HPeelLimits.FormatVersion != 2 {
		t.Fatalf("%s hPeelLimits.format_version = %v, want the migrated value 2", name, value.HPeelLimits)
	}
	if value.HPeelLimits.Tails == nil || len(*value.HPeelLimits.Tails) != 20 ||
		value.HPeelLimits.Heads == nil || len(*value.HPeelLimits.Heads) != 3 {
		t.Fatalf("%s hPeelLimits bound array lengths = heads %v tails %v, want 3/20", name, value.HPeelLimits.Heads, value.HPeelLimits.Tails)
	}
	if value.VPeelExInfo == nil || value.VPeelExInfo.VPeelFoldingCorrectWidth == nil || *value.VPeelExInfo.VPeelFoldingCorrectWidth != -2.5 {
		t.Fatalf("%s vPeelExInfo.vPeelFoldingCorrectWidth = %v, want -2.5", name, value.VPeelExInfo)
	}
	if value.CommonPeelInfo == nil || value.CommonPeelInfo.FixedPullLength == nil || *value.CommonPeelInfo.FixedPullLength != 1 {
		t.Fatalf("%s commonPeelInfo.fixedPullLength = %v, want 1", name, value.CommonPeelInfo)
	}
	// 空数组与缺失成员是两种不同的状态，编码必须分别写回 [] 和完全省略
	// An empty array and a missing member are two different states, and encoding must write back [] and total omission respectively
	first := (*value.DataGroup)[0]
	if first.Weights == nil || len(*first.Weights) != 0 || first.Vertices == nil || len(*first.Vertices) != 0 || first.ExFixeds == nil || len(*first.ExFixeds) != 0 {
		t.Fatalf("%s dataGroup[0] optional vertex data = weights %v vertices %v exFixeds %v, want three present empty arrays", name, first.Weights, first.Vertices, first.ExFixeds)
	}
}

// assertUndressPartsDataSampleFields 校验已知 .undresspdat 样本的实测成员值
// assertUndressPartsDataSampleFields checks measured member values of a known .undresspdat sample
func assertUndressPartsDataSampleFields(t *testing.T, name string, value *UndressPrecomputeTarget) {
	t.Helper()
	if value.OneGroupLooker == nil || value.OneGroupLooker.Targets == nil || len(*value.OneGroupLooker.Targets) == 0 {
		t.Fatalf("%s has no baked group keys", name)
	}
	targets := *value.OneGroupLooker.Targets
	for index, key := range targets {
		if key.Lbl == nil || *key.Lbl == "" || key.Lyr == nil {
			t.Fatalf("%s OneGroupLooker.Targets[%d] = %+v, want a complete (lyr, lbl) key", name, index, key)
		}
	}
	// 两张缓存表都只能引用 Targets 范围内的序号，越界时游戏会丢弃该组的缓存
	// Both cache tables may only reference an index within Targets, and the game drops a group's cache when the index is out of range
	for label, table := range map[string]*UndressMeshReductionTable{"meshReduction": value.MeshReduction} {
		if table == nil || table.D == nil {
			t.Fatalf("%s has no %s entries", name, label)
		}
		for index, entry := range *table.D {
			if entry.V == nil || entry.V.Index == nil || int(*entry.V.Index) >= len(targets) {
				t.Fatalf("%s %s[%d] references group %v, outside the %d baked keys", name, label, index, entry.V, len(targets))
			}
		}
	}
	if value.WidthMeasurer == nil || value.WidthMeasurer.D == nil {
		t.Fatalf("%s has no widthMeasurer entries", name)
	}
	for index, entry := range *value.WidthMeasurer.D {
		if entry.V == nil || entry.V.Index == nil || int(*entry.V.Index) >= len(targets) {
			t.Fatalf("%s widthMeasurer[%d] references group %v, outside the %d baked keys", name, index, entry.V, len(targets))
		}
	}
	if name != "crc2_Underwear038_pants.undresspdat" {
		return
	}
	if value.EditVer == nil || *value.EditVer != 13 {
		t.Fatalf("%s editVer = %v, want 13 to match the paired .undressdat", name, value.EditVer)
	}
	if value.WidthMeasurerValidPixelThreshold == nil || *value.WidthMeasurerValidPixelThreshold != 0.99 {
		t.Fatalf("%s WidthMeasurerValidPixelThreshold = %v, want 0.99", name, value.WidthMeasurerValidPixelThreshold)
	}
	if len(targets) != 84 || len(*value.MeshReduction.D) != 60 || len(*value.WidthMeasurer.D) != 24 {
		t.Fatalf("%s table sizes = %d keys, %d meshReduction, %d widthMeasurer, want 84/60/24", name, len(targets), len(*value.MeshReduction.D), len(*value.WidthMeasurer.D))
	}
	if *targets[0].Lyr != 0 || *targets[0].Lbl != "Group_0000" {
		t.Fatalf("%s OneGroupLooker.Targets[0] = %+v, want lyr 0 / Group_0000", name, targets[0])
	}
	// 前段退化目标为空数组、后段非空，两者必须分别原样保留
	// The preceding degeneracy target is an empty array while the succeeding one is not, and both must be preserved as stored
	first := (*value.MeshReduction.D)[0]
	if first.Dat == nil || first.Dat.P == nil || first.Dat.P.Idcs == nil || len(*first.Dat.P.Idcs) != 0 ||
		first.Dat.S == nil || first.Dat.S.Idcs == nil || len(*first.Dat.S.Idcs) != 1 || (*first.Dat.S.Idcs)[0] != 148 {
		t.Fatalf("%s meshReduction[0].dat = %+v, want an empty p.idcs and s.idcs [148]", name, first.Dat)
	}
}

func TestKCESUndressDocumentsPreserveLayoutAndFloat32Values(t *testing.T) {
	for _, path := range kcesfixtures.MiscSamplePaths(t) {
		extension := strings.ToLower(filepath.Ext(path))
		if extension != KCESUndressDataExtension && extension != KCESUndressPartsDataExtension {
			continue
		}
		t.Run(filepath.Base(path), func(t *testing.T) {
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			var encoded []byte
			if extension == KCESUndressDataExtension {
				value, decodeErr := DecodeKCESUndressData(original)
				if decodeErr != nil {
					t.Fatalf("DecodeKCESUndressData: %v", decodeErr)
				}
				if encoded, err = EncodeKCESUndressData(value); err != nil {
					t.Fatalf("EncodeKCESUndressData: %v", err)
				}
			} else {
				value, decodeErr := DecodeKCESUndressPartsData(original)
				if decodeErr != nil {
					t.Fatalf("DecodeKCESUndressPartsData: %v", decodeErr)
				}
				if encoded, err = EncodeKCESUndressPartsData(value); err != nil {
					t.Fatalf("EncodeKCESUndressPartsData: %v", err)
				}
			}
			// 成员集合、成员顺序与缩进逐字节保留，只有数字字面量会改写为等值的最短 float32 文本
			// The member set, member order, and indentation survive byte for byte, and only numeric literals are rewritten as the shortest equivalent float32 text
			assertSameJSONTokensAsFloat32(t, original, encoded)
			if bytes.Equal(encoded, original) {
				return
			}
			if !bytes.Equal(stripJSONNumberLiterals(t, original), stripJSONNumberLiterals(t, encoded)) {
				t.Fatalf("%s differs outside numeric literals", filepath.Base(path))
			}
		})
	}
}

// assertSameJSONTokensAsFloat32 逐个 token 比较两份 JSON，数字按 float32 取值比较
// assertSameJSONTokensAsFloat32 compares two JSON documents token by token and compares numbers by their float32 value
func assertSameJSONTokensAsFloat32(t *testing.T, want []byte, got []byte) {
	t.Helper()
	wantDecoder := json.NewDecoder(bytes.NewReader(want))
	wantDecoder.UseNumber()
	gotDecoder := json.NewDecoder(bytes.NewReader(got))
	gotDecoder.UseNumber()
	for index := 0; ; index++ {
		wantToken, wantErr := wantDecoder.Token()
		gotToken, gotErr := gotDecoder.Token()
		if wantErr == io.EOF && gotErr == io.EOF {
			return
		}
		if wantErr != nil || gotErr != nil {
			t.Fatalf("token %d read error: want %v, got %v", index, wantErr, gotErr)
		}
		wantNumber, wantIsNumber := wantToken.(json.Number)
		gotNumber, gotIsNumber := gotToken.(json.Number)
		if wantIsNumber != gotIsNumber {
			t.Fatalf("token %d kind changed: %v -> %v", index, wantToken, gotToken)
		}
		if !wantIsNumber {
			if wantToken != gotToken {
				t.Fatalf("token %d changed: %v -> %v", index, wantToken, gotToken)
			}
			continue
		}
		if !sameFloat32Number(wantNumber, gotNumber) {
			t.Fatalf("token %d number changed value: %s -> %s", index, wantNumber, gotNumber)
		}
	}
}

// sameFloat32Number 判断两个 JSON 数字字面量是否表示同一个 float32 取值
// sameFloat32Number reports whether two JSON number literals denote the same float32 value
func sameFloat32Number(want json.Number, got json.Number) bool {
	wantInt, wantIsInt := parseJSONInteger(want)
	gotInt, gotIsInt := parseJSONInteger(got)
	if wantIsInt && gotIsInt {
		return wantInt == gotInt
	}
	wantFloat, wantErr := strconv.ParseFloat(want.String(), 32)
	gotFloat, gotErr := strconv.ParseFloat(got.String(), 32)
	if wantErr != nil || gotErr != nil {
		return want.String() == got.String()
	}
	return math.Float32bits(float32(wantFloat)) == math.Float32bits(float32(gotFloat))
}

// parseJSONInteger 在字面量是精确整数时返回其值
// parseJSONInteger returns the value of a literal when it is an exact integer
func parseJSONInteger(number json.Number) (int64, bool) {
	value, err := strconv.ParseInt(number.String(), 10, 64)
	return value, err == nil
}

// stripJSONNumberLiterals 把 JSON 中的每个数字字面量整体替换为单个占位符，只保留成员集合、顺序与排版
// JSON 的数字字面量只能以负号或数字开头，因此 true 与 false 里的 e 不会被误判为数字的一部分
// stripJSONNumberLiterals replaces every numeric literal in JSON with a single placeholder so only the member set, order, and layout remain
// A JSON number literal can only start with a minus sign or a digit, so the e inside true and false is never mistaken for part of a number
func stripJSONNumberLiterals(t *testing.T, data []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	inString := false
	escaped := false
	for index := 0; index < len(data); index++ {
		character := data[index]
		if inString {
			out.WriteByte(character)
			switch {
			case escaped:
				escaped = false
			case character == '\\':
				escaped = true
			case character == '"':
				inString = false
			}
			continue
		}
		if character == '"' {
			inString = true
			out.WriteByte(character)
			continue
		}
		if character != '-' && (character < '0' || character > '9') {
			out.WriteByte(character)
			continue
		}
		out.WriteByte('#')
		for index+1 < len(data) && isJSONNumberByte(data[index+1]) {
			index++
		}
	}
	return out.Bytes()
}

// isJSONNumberByte 判断字节是否可以出现在 JSON 数字字面量的非首位
// isJSONNumberByte reports whether a byte may appear at a non-leading position of a JSON number literal
func isJSONNumberByte(character byte) bool {
	switch character {
	case '+', '-', '.', 'e', 'E':
		return true
	default:
		return character >= '0' && character <= '9'
	}
}
