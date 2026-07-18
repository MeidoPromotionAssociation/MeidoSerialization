package KCES

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	"github.com/ugorji/go/codec"
)

// .menuassets、.materialassets、.model 与 .preset 共用的 Unity/Parts 数据类型及 Int32 校验。
//
// Unity/Parts data types and Int32 validation shared by .menuassets, .materialassets, .model, and .preset.

const (
	gameInt32Min = int64(-1 << 31)
	gameInt32Max = int64(1<<31 - 1)
)

// requireInt32 rejects Go int values that MessagePack-CSharp cannot assign to
// a C# System.Int32 field. On 64-bit hosts ugorji can otherwise emit an
// Int64/UInt32 value successfully, only for the game's checked ReadInt32 path
// to throw OverflowException while loading the generated asset.
func requireInt32(path string, value int) error {
	n := int64(value)
	if n < gameInt32Min || n > gameInt32Max {
		if path == "" {
			path = "value"
		}
		return fmt.Errorf("%s=%d is outside the Int32 range [%d,%d] required by the game's C# field", path, value, gameInt32Min, gameInt32Max)
	}
	return nil
}

type int32PointerVisit struct {
	typ reflect.Type
	ptr uintptr
}

// validateGameInt32Fields recursively validates every Go int in a typed KCES
// wire object. In the parts schemas all such fields map to C# int or an enum
// with an Int32 underlying type. Interface-valued compatibility carriers are
// deliberately opaque; GradaBytes.Value, for example, represents a byte[]
// rather than a C# integer. Sparse formatter holes use RawMessagePackSlot.
func validateGameInt32Fields(value interface{}) error {
	return validateGameInt32Value(reflect.ValueOf(value), "", make(map[int32PointerVisit]struct{}))
}

func validateGameInt32Value(value reflect.Value, path string, activePointers map[int32PointerVisit]struct{}) error {
	if !value.IsValid() {
		return nil
	}

	switch value.Kind() {
	case reflect.Interface:
		return nil
	case reflect.Ptr:
		if value.IsNil() {
			return nil
		}
		visit := int32PointerVisit{typ: value.Type(), ptr: value.Pointer()}
		if _, exists := activePointers[visit]; exists {
			if path == "" {
				path = "value"
			}
			return fmt.Errorf("%s contains a pointer cycle", path)
		}
		activePointers[visit] = struct{}{}
		err := validateGameInt32Value(value.Elem(), path, activePointers)
		delete(activePointers, visit)
		return err
	case reflect.Int:
		return requireInt32(path, int(value.Int()))
	case reflect.Struct:
		typ := value.Type()
		for i := 0; i < value.NumField(); i++ {
			fieldType := typ.Field(i)
			if fieldType.PkgPath != "" { // unexported marker fields such as _struct
				continue
			}
			name := fieldType.Name
			if tag, ok := fieldType.Tag.Lookup("json"); ok {
				name = strings.Split(tag, ",")[0]
				if name == "-" {
					continue
				}
				if name == "" {
					name = fieldType.Name
				}
			}
			if err := validateGameInt32Value(value.Field(i), joinInt32FieldPath(path, name), activePointers); err != nil {
				return err
			}
		}
		return nil
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			if err := validateGameInt32Value(value.Index(i), fmt.Sprintf("%s[%d]", path, i), activePointers); err != nil {
				return err
			}
		}
		return nil
	case reflect.Map:
		if value.IsNil() {
			return nil
		}
		keys := value.MapKeys()
		sort.Slice(keys, func(i, j int) bool {
			return int32MapKeyLabel(keys[i]) < int32MapKeyLabel(keys[j])
		})
		for _, key := range keys {
			entryPath := path + int32MapKeyLabel(key)
			if err := validateGameInt32Value(value.MapIndex(key), entryPath, activePointers); err != nil {
				return err
			}
		}
		return nil
	default:
		return nil
	}
}

func joinInt32FieldPath(parent, field string) string {
	if parent == "" {
		return field
	}
	if field == "" {
		return parent
	}
	return parent + "." + field
}

func int32MapKeyLabel(key reflect.Value) string {
	if key.Kind() == reflect.String {
		return fmt.Sprintf("[%q]", key.String())
	}
	return fmt.Sprintf("[%v]", key.Interface())
}

// Vector2 表示 UnityEngine.Vector2 的 MessagePack 数组布局 / Vector2 represents UnityEngine.Vector2 in MessagePack array layout
type Vector2 struct {
	_struct                struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`
	X                      float32 `json:"x"` // X 轴分量 / X-axis component
	Y                      float32 `json:"y"` // Y 轴分量 / Y-axis component
}

// Vector2Int 表示 UnityEngine.Vector2Int 的 MessagePack 数组布局 / Vector2Int represents UnityEngine.Vector2Int in MessagePack array layout
type Vector2Int struct {
	_struct                struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`
	X                      int `json:"x"` // X 轴整数分量 / Integer X-axis component
	Y                      int `json:"y"` // Y 轴整数分量 / Integer Y-axis component
}

// Vector3 表示 UnityEngine.Vector3 的 MessagePack 数组布局 / Vector3 represents UnityEngine.Vector3 in MessagePack array layout
type Vector3 struct {
	_struct                struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`
	X                      float32 `json:"x"` // X 轴分量 / X-axis component
	Y                      float32 `json:"y"` // Y 轴分量 / Y-axis component
	Z                      float32 `json:"z"` // Z 轴分量 / Z-axis component
}

// Vector4 表示 UnityEngine.Vector4 或 Quaternion 的 MessagePack 数组布局 / Vector4 represents UnityEngine.Vector4 or Quaternion in MessagePack array layout
type Vector4 struct {
	_struct                struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`
	X                      float32 `json:"x"` // X 轴分量或四元数 X / X-axis component or quaternion X
	Y                      float32 `json:"y"` // Y 轴分量或四元数 Y / Y-axis component or quaternion Y
	Z                      float32 `json:"z"` // Z 轴分量或四元数 Z / Z-axis component or quaternion Z
	W                      float32 `json:"w"` // W 分量或四元数 W / W component or quaternion W
}

// PartsColor 对应游戏源码 MaidInfinityColor.PartsColor。MessagePack 只保存
// m_gradaBytes；JSON 输出会额外提供由 DeserializeGrada 布局解出的 m_grada，
// 方便编辑，同时保留原始字节以保证未修改时的精确往返。
// PartsColor corresponds to the game's MaidInfinityColor.PartsColor. MessagePack
// stores only m_gradaBytes; JSON additionally exposes the m_grada view decoded
// with the game's DeserializeGrada layout while retaining the original bytes
// for exact unmodified round-trips.
type PartsColor struct {
	_struct                struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`
	MainHue                int        `json:"m_nMainHue"`          // 主色相，对应 m_nMainHue / Main hue, matching m_nMainHue
	MainChroma             int        `json:"m_nMainChroma"`       // 主色彩度，对应 m_nMainChroma / Main chroma, matching m_nMainChroma
	MainBrightness         int        `json:"m_nMainBrightness"`   // 主色亮度，对应 m_nMainBrightness / Main brightness, matching m_nMainBrightness
	MainContrast           int        `json:"m_nMainContrast"`     // 主色对比度，对应 m_nMainContrast / Main contrast, matching m_nMainContrast
	ShadowRate             int        `json:"m_nShadowRate"`       // 阴影混合比例，对应 m_nShadowRate / Shadow blend rate, matching m_nShadowRate
	ShadowHue              int        `json:"m_nShadowHue"`        // 阴影色相，对应 m_nShadowHue / Shadow hue, matching m_nShadowHue
	ShadowChroma           int        `json:"m_nShadowChroma"`     // 阴影彩度，对应 m_nShadowChroma / Shadow chroma, matching m_nShadowChroma
	ShadowBrightness       int        `json:"m_nShadowBrightness"` // 阴影亮度，对应 m_nShadowBrightness / Shadow brightness, matching m_nShadowBrightness
	ShadowContrast         int        `json:"m_nShadowContrast"`   // 阴影对比度，对应 m_nShadowContrast / Shadow contrast, matching m_nShadowContrast
	GradaBytes             GradaBytes `json:"m_gradaBytes"`        // SerializeGrada 生成的梯度色字节 / Gradient bytes produced by SerializeGrada
}

// PartsColorGrada is one m_grada element. SerializeGrada writes these nine
// signed Int32 fields in little-endian order and does not recursively serialize
// the element's own m_grada/m_gradaBytes fields.
type PartsColorGrada struct {
	MainHue          int32 `json:"m_nMainHue"`
	MainChroma       int32 `json:"m_nMainChroma"`
	MainBrightness   int32 `json:"m_nMainBrightness"`
	MainContrast     int32 `json:"m_nMainContrast"`
	ShadowRate       int32 `json:"m_nShadowRate"`
	ShadowHue        int32 `json:"m_nShadowHue"`
	ShadowChroma     int32 `json:"m_nShadowChroma"`
	ShadowBrightness int32 `json:"m_nShadowBrightness"`
	ShadowContrast   int32 `json:"m_nShadowContrast"`
}

const partsColorGradaRecordBytes = 9 * 4

// DecodePartsColorGrada implements MaidInfinityColor.PartsColor.DeserializeGrada.
// It returns bytes after the declared records separately: the game callback
// leaves such bytes unread, while a faithful editor must not silently discard
// them.
func DecodePartsColorGrada(data []byte) ([]PartsColorGrada, []byte, error) {
	if len(data) < 4 {
		return nil, nil, fmt.Errorf("gradient byte stream is %d bytes; need the Int32 count", len(data))
	}
	count := int64(int32(binary.LittleEndian.Uint32(data[:4])))
	if count < 0 {
		return nil, nil, fmt.Errorf("gradient color count is negative: %d", count)
	}
	remaining := int64(len(data) - 4)
	if count > remaining/partsColorGradaRecordBytes {
		return nil, nil, fmt.Errorf("gradient color count %d cannot fit in %d payload bytes", count, remaining)
	}

	values := make([]PartsColorGrada, int(count))
	offset := 4
	readInt32 := func() int32 {
		value := int32(binary.LittleEndian.Uint32(data[offset : offset+4]))
		offset += 4
		return value
	}
	for index := range values {
		value := &values[index]
		value.MainHue = readInt32()
		value.MainChroma = readInt32()
		value.MainBrightness = readInt32()
		value.MainContrast = readInt32()
		value.ShadowRate = readInt32()
		value.ShadowHue = readInt32()
		value.ShadowChroma = readInt32()
		value.ShadowBrightness = readInt32()
		value.ShadowContrast = readInt32()
	}
	if count == 0 {
		// DeserializeGrada assigns null rather than an empty array for count zero.
		values = nil
	}
	return values, append([]byte(nil), data[offset:]...), nil
}

// EncodePartsColorGrada implements the byte layout written by SerializeGrada.
func EncodePartsColorGrada(values []PartsColorGrada) ([]byte, error) {
	if int64(len(values)) > gameInt32Max {
		return nil, fmt.Errorf("gradient color count %d exceeds Int32", len(values))
	}
	size := 4 + len(values)*partsColorGradaRecordBytes
	out := make([]byte, size)
	binary.LittleEndian.PutUint32(out[:4], uint32(len(values)))
	offset := 4
	writeInt32 := func(value int32) {
		binary.LittleEndian.PutUint32(out[offset:offset+4], uint32(value))
		offset += 4
	}
	for index := range values {
		value := &values[index]
		writeInt32(value.MainHue)
		writeInt32(value.MainChroma)
		writeInt32(value.MainBrightness)
		writeInt32(value.MainContrast)
		writeInt32(value.ShadowRate)
		writeInt32(value.ShadowHue)
		writeInt32(value.ShadowChroma)
		writeInt32(value.ShadowBrightness)
		writeInt32(value.ShadowContrast)
	}
	return out, nil
}

// GradaBytes represents the game's byte[] m_gradaBytes field. Its decoder
// accepts the binary, legacy raw-string, nil, and byte-array carriers accepted
// by MessagePack-CSharp's ByteArrayFormatter, while rejecting values (such as
// a bool) that the game itself cannot deserialize. DecodeGrada and SetGrada
// expose the readable inner layout without running
// OnBeforeSerialize automatically.
type GradaBytes struct {
	Value interface{} `json:"-"` // nil 或 []byte；保留 interface 以避免破坏既有 Go API / nil or []byte; interface retained for Go API compatibility
}

func (g GradaBytes) DecodeGrada() ([]PartsColorGrada, []byte, error) {
	data, ok := g.Value.([]byte)
	if !ok {
		return nil, nil, fmt.Errorf("m_gradaBytes has MessagePack carrier %T, not byte[]", g.Value)
	}
	return DecodePartsColorGrada(data)
}

// SetGrada explicitly replaces the readable SerializeGrada prefix and appends
// any caller-supplied trailing bytes. Ordinary MessagePack encoding never calls
// this method implicitly.
func (g *GradaBytes) SetGrada(values []PartsColorGrada, trailing []byte) error {
	if g == nil {
		return fmt.Errorf("nil GradaBytes receiver")
	}
	encoded, err := EncodePartsColorGrada(values)
	if err != nil {
		return err
	}
	g.Value = append(encoded, trailing...)
	return nil
}

func (g GradaBytes) CodecEncodeSelf(e *codec.Encoder) {
	switch value := g.Value.(type) {
	case nil:
		e.MustEncode(nil)
	case []byte:
		e.MustEncode(value)
	default:
		panic(fmt.Errorf("m_gradaBytes has unsupported value %T; game ByteArrayFormatter accepts only byte[] or nil", g.Value))
	}
}

func (g *GradaBytes) CodecDecodeSelf(d *codec.Decoder) {
	var raw codec.Raw
	d.MustDecode(&raw)
	if len(raw) == 0 || (len(raw) == 1 && raw[0] == 0xc0) {
		g.Value = nil
		return
	}
	if !isGameByteArrayCarrier(raw[0]) {
		panic(fmt.Errorf("m_gradaBytes has unsupported MessagePack marker 0x%02x for the game's ByteArrayFormatter", raw[0]))
	}

	var value []byte
	if err := ct.DecodeMsgpack(raw, &value); err != nil {
		panic(fmt.Errorf("decode m_gradaBytes byte[]: %w", err))
	}
	g.Value = cloneSlicePreserveNil(value)
}

func isGameByteArrayCarrier(marker byte) bool {
	return marker == 0xc4 || marker == 0xc5 || marker == 0xc6 ||
		marker >= 0xa0 && marker <= 0xbf || marker == 0xda || marker == 0xdb ||
		marker >= 0x90 && marker <= 0x9f || marker == 0xdc || marker == 0xdd
}

func (g GradaBytes) MarshalJSON() ([]byte, error) {
	switch v := g.Value.(type) {
	case nil:
		return []byte("null"), nil
	case []byte:
		return json.Marshal(v)
	default:
		return nil, fmt.Errorf("m_gradaBytes has unsupported value %T; expected byte[] or nil", g.Value)
	}
}

func (g *GradaBytes) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) {
		g.Value = nil
		return nil
	}

	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		decoded, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return fmt.Errorf("m_gradaBytes string must be base64: %w", err)
		}
		g.Value = decoded
		return nil
	}
	return fmt.Errorf("m_gradaBytes must be a base64 JSON string or null")
}

// MarshalJSON exposes a readable m_grada companion for valid byte streams.
// m_gradaBytes remains present as base64, so malformed or extended inner data
// still has a lossless source of truth.
func (p PartsColor) MarshalJSON() ([]byte, error) {
	type partsColorJSON PartsColor
	base, err := json.Marshal(partsColorJSON(p))
	if err != nil {
		return nil, err
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(base, &object); err != nil {
		return nil, err
	}
	data, ok := p.GradaBytes.Value.([]byte)
	if !ok {
		return json.Marshal(object)
	}
	values, trailing, decodeErr := DecodePartsColorGrada(data)
	if decodeErr != nil {
		encodedError, err := json.Marshal(decodeErr.Error())
		if err != nil {
			return nil, err
		}
		object["m_gradaDecodeError"] = encodedError
		return json.Marshal(object)
	}
	encodedValues, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}
	object["m_grada"] = encodedValues
	if len(trailing) != 0 {
		encodedTrailing, err := json.Marshal(trailing)
		if err != nil {
			return nil, err
		}
		object["m_gradaTrailingBytes"] = encodedTrailing
	}
	return json.Marshal(object)
}

// UnmarshalJSON keeps m_gradaBytes byte-exact when the readable view was not
// changed. If m_grada or m_gradaTrailingBytes is edited, only then is the known
// SerializeGrada prefix rebuilt; existing trailing bytes are retained unless
// the JSON explicitly replaces them.
func (p *PartsColor) UnmarshalJSON(data []byte) error {
	type partsColorJSON PartsColor
	var decoded partsColorJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*p = PartsColor(decoded)

	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return err
	}
	gradationJSON, hasGradation := object["m_grada"]
	trailingJSON, hasTrailing := object["m_gradaTrailingBytes"]
	if !hasGradation && !hasTrailing {
		return nil
	}

	rawBytes, rawIsBytes := p.GradaBytes.Value.([]byte)
	var existingValues []PartsColorGrada
	var existingTrailing []byte
	var existingErr error
	if rawIsBytes {
		existingValues, existingTrailing, existingErr = DecodePartsColorGrada(rawBytes)
	} else {
		existingErr = fmt.Errorf("m_gradaBytes has MessagePack carrier %T, not byte[]", p.GradaBytes.Value)
	}

	targetValues := cloneSlicePreserveNil(existingValues)
	if hasGradation {
		if err := json.Unmarshal(gradationJSON, &targetValues); err != nil {
			return fmt.Errorf("decode m_grada: %w", err)
		}
	}
	targetTrailing := cloneSlicePreserveNil(existingTrailing)
	if hasTrailing {
		if err := json.Unmarshal(trailingJSON, &targetTrailing); err != nil {
			return fmt.Errorf("decode m_gradaTrailingBytes: %w", err)
		}
	}

	if existingErr == nil && reflect.DeepEqual(targetValues, existingValues) && bytes.Equal(targetTrailing, existingTrailing) {
		return nil
	}
	if !hasGradation && existingErr != nil {
		return fmt.Errorf("cannot edit m_gradaTrailingBytes without a readable m_gradaBytes prefix: %w", existingErr)
	}
	encoded, err := EncodePartsColorGrada(targetValues)
	if err != nil {
		return fmt.Errorf("encode m_grada: %w", err)
	}
	p.GradaBytes.Value = append(encoded, targetTrailing...)
	return nil
}

// PreMulTexDatas 对应 Parts.Menu.PreMulTexDatas 的贴图预合成记录 / PreMulTexDatas maps Parts.Menu.PreMulTexDatas texture pre-composition records
type PreMulTexDatas struct {
	_struct                struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`
	Version                int            `json:"version"`                 // 版本号，游戏 FixVersion 为 1001 / Version value; the game's FixVersion is 1001
	SlotID                 string         `json:"slotId"`                  // 目标槽位 ID / Target slot ID
	SaveTag                string         `json:"saveTag"`                 // 保存用图层标签 / Saved texture layer tag
	MatNo                  int            `json:"f_nMatNo"`                // 目标材质编号 / Target material index
	PropName               string         `json:"f_strPropName"`           // 目标材质属性名 / Target material property name
	LayerNo                int            `json:"f_nLayerNo"`              // 贴图层编号 / Texture layer number
	FileName               string         `json:"f_strFileName"`           // 合成源贴图文件名 / Source texture file name for composition
	BlendMode              string         `json:"f_eBlendMode"`            // 合成模式字符串 / Blend-mode string
	MaskParam              *MaskParam     `json:"maskParam"`               // 蒙版参数 / Mask parameters
	InfColParam            *InfColorParam `json:"infColParam"`             // 无限色参数 / Infinity-color parameters
	TexGroup               bool           `json:"f_bTexGroup"`             // 是否属于贴图组 / Whether this layer belongs to a texture group
	LayNoInGroup           int            `json:"f_nLayNoInGroup"`         // 组内层编号 / Layer index inside the group
	Alpha                  float32        `json:"f_fAlpha"`                // 合成透明度 / Composition alpha
	TargetBodyTexSize      int            `json:"f_nTargetBodyTexSize"`    // 目标身体贴图尺寸 / Target body texture size
	PosDefHokuroTatooSlot  string         `json:"posDefHokuroTatooSlotId"` // 默认痣/纹身位置槽位 / Default mole/tattoo position slot
	PreMaskData            []MaskData     `json:"preMaskData"`             // 预计算蒙版数据 / Precomputed mask data
	PreTransTexData        []TransTexData `json:"preTransTexData"`         // 预计算贴图变换数据 / Precomputed texture transform data
	PreInfColData          *InfColData    `json:"preInfColData"`           // 预计算无限色数据 / Precomputed infinity-color data
	PreTexCompoTypeStr     string         `json:"preTexCompoTypeStr"`      // 预合成系统材质模式字符串 / Pre-composition system material mode string
}

// NewPreMulTexDatas returns the defaults installed by the C# base constructor
// and field initializers before MessagePack/JSON member assignment.
func NewPreMulTexDatas() *PreMulTexDatas {
	return &PreMulTexDatas{
		Version:            preMulTexDatasFixVersion,
		LayNoInGroup:       -1,
		Alpha:              1,
		PreTexCompoTypeStr: "Alpha",
	}
}

func (p *PreMulTexDatas) UnmarshalJSON(data []byte) error {
	type preMulTexDatasJSON PreMulTexDatas
	var value preMulTexDatasJSON
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*p = PreMulTexDatas(value)
	return nil
}

// TransTexData 对应 TexLay.TransTexData，描述合成贴图的平移/缩放/旋转 / TransTexData maps TexLay.TransTexData and describes texture translation/scale/rotation
type TransTexData struct {
	_struct                struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`
	Pos                    Vector2       `json:"pos"`          // 贴图中心位置，通常为目标 RT 归一化坐标 / Texture center position, usually normalized in the target render texture
	Scale                  Vector2       `json:"scale"`        // 贴图缩放，负值表示翻转 / Texture scale; negative values indicate flipping
	RotDeg                 float32       `json:"rotDeg"`       // 以角度表示的旋转量 / Rotation in degrees
	AreaUV                 Vector4       `json:"areaUV"`       // 使用的源贴图 UV 区域 / Source texture UV area
	SrcTexPixcel           Vector2Int    `json:"srcTexPixcel"` // 源贴图像素尺寸，保留游戏字段原拼写 / Source texture pixel size, preserving the game's spelling
	DefTrans               *TransTexData `json:"defTrans"`     // 默认变换，用于 ResetTrans 恢复 / Default transform used by ResetTrans
}

func NewTransTexData() *TransTexData {
	return &TransTexData{
		Scale:  Vector2{X: 1, Y: 1},
		AreaUV: Vector4{Z: 1, W: 1},
	}
}

func (v *TransTexData) UnmarshalJSON(data []byte) error {
	type transTexDataJSON TransTexData
	var value transTexDataJSON
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*v = TransTexData(value)
	return nil
}

// InfColorParam 对应 TexLay.InfColorParam，描述无限色合成输入 / InfColorParam maps TexLay.InfColorParam and describes infinity-color composition input
type InfColorParam struct {
	_struct                  struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata   `codec:"-"`
	Tag                      string       `json:"tag"`                      // 无限色目标标签 / Infinity-color target tag
	InfColType               int          `json:"infColType"`               // 颜色类型枚举：NONE/INF_COLOR/PART_COLOR/GRADA_COLOR / Color type enum
	InfColorID               int          `json:"infColorId"`               // MaidInfinityColor.PARTS_COLOR 枚举值 / MaidInfinityColor.PARTS_COLOR enum value
	IsIndependenceMultiColor bool         `json:"isIndependenceMultiColor"` // 是否使用独立多色数据 / Whether independent multi-color data is used
	PC                       PartsColor   `json:"pc"`                       // 单色无限色参数 / Single infinity-color parameters
	IDTexName                []string     `json:"idTexName"`                // ID 贴图文件名列表 / ID texture file-name list
	PartCols                 []PartColDef `json:"partCols"`                 // 分部颜色定义列表 / Part-color definition list
	GradeCols                *GradaColDef `json:"gradeCols"`                // 渐变色定义，字段名沿用游戏 gradeCols 拼写 / Gradient color definition, keeping the game's spelling
	GradaLines               []Vector4    `json:"gradaLines"`               // 渐变线段数组 / Gradient line array
	IDTexIsRGB               bool         `json:"idTexIsRGB"`               // 是否把 ID 贴图按 RGB 通道分区解释 / Whether ID textures are interpreted by RGB channels
	GradaIsMugen             bool         `json:"gradaIsMugen"`             // 渐变是否使用无限色表 / Whether the gradient uses the infinity-color table
}

func NewInfColorParam() *InfColorParam {
	return &InfColorParam{InfColorID: -1}
}

func (v *InfColorParam) UnmarshalJSON(data []byte) error {
	type infColorParamJSON InfColorParam
	var value infColorParamJSON
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*v = InfColorParam(value)
	return nil
}

// MaskData 对应 TexLay.MaskData，记录单个蒙版开关 / MaskData maps TexLay.MaskData and stores one mask toggle
type MaskData struct {
	_struct                struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`
	Name                   string `json:"name"` // 蒙版名称 / Mask name
	Mask                   bool   `json:"mask"` // 是否启用该蒙版 / Whether this mask is enabled
}

// MaskParam 对应 TexLay.MaskParam，描述蒙版贴图和区域 / MaskParam maps TexLay.MaskParam and describes mask texture and ranges
type MaskParam struct {
	_struct                struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`
	MaskData               []MaskData `json:"maskData"`          // 每个蒙版槽位的启用状态 / Enabled state for each mask slot
	MaskTexName            string     `json:"maskTexName"`       // 蒙版贴图文件名 / Mask texture file name
	MaskRanges             []Vector4  `json:"maskRanges"`        // 蒙版 UV/范围数组 / Mask UV/range array
	LinkMaskName           string     `json:"linkMaskName"`      // 关联蒙版名称 / Linked mask name
	LinkMaskNo             int        `json:"linkMaskNo"`        // 关联蒙版编号 / Linked mask index
	ShareRtTargetPart      string     `json:"shareRtTargetPart"` // 共享 RenderTexture 的目标部件名 / Target part name for shared RenderTexture
}

// PartColDef 对应 InfinityColorTexMgr2.PartColDef，描述 ID 贴图某个部位颜色 / PartColDef maps InfinityColorTexMgr2.PartColDef and describes one ID-texture part color
type PartColDef struct {
	_struct                struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`
	PartName               string     `json:"part_name"`    // 部位名称，字段名对应 part_name / Part name, matching part_name
	MultiCol               PartsColor `json:"multi_col"`    // 部位颜色参数 / Part color parameters
	PatternScale           Vector2    `json:"patternScale"` // 纹样缩放 / Pattern texture scale
	PatternRot             float32    `json:"patternRot"`   // 纹样旋转角度 / Pattern texture rotation in degrees
}

func NewPartColDef() *PartColDef {
	return &PartColDef{PatternScale: Vector2{X: 1, Y: 1}}
}

func (v *PartColDef) UnmarshalJSON(data []byte) error {
	type partColDefJSON PartColDef
	var value partColDefJSON
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*v = PartColDef(value)
	return nil
}

// GradaColDef 对应 InfinityColorTexMgr2.GradaColDef，描述渐变色定义 / GradaColDef maps InfinityColorTexMgr2.GradaColDef and describes a gradient color definition
type GradaColDef struct {
	_struct                struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`
	NotUse                 string     `json:"notUse"`          // 游戏保留字段 notUse / Game-reserved notUse field
	GradaNum               int        `json:"gradaNum"`        // 渐变点数量 / Number of gradient points
	GradaRates             []float32  `json:"gradaRates"`      // 渐变点位置比例 / Gradient point rates
	GradaRateRanges        []Vector4  `json:"gradaRateRanges"` // 渐变点影响范围 / Gradient point influence ranges
	MultiCol               PartsColor `json:"multi_col"`       // 渐变用多色数据 / Multi-color data used by the gradient
}

// InfColData 对应 InfinityColorTexMgr2.InfColData，保存应用后的无限色数据 / InfColData maps InfinityColorTexMgr2.InfColData and stores applied infinity-color data
type InfColData struct {
	_struct                  struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata   `codec:"-"`
	IsIndependenceMultiColor bool         `json:"isIndependenceMultiColor"` // 是否使用独立多色表 / Whether independent multi-color table data is used
	InfColType               int          `json:"infColType"`               // 无限色类型枚举 / Infinity-color type enum
	PartsColorType           int          `json:"partsColorType"`           // MaidInfinityColor.PARTS_COLOR 枚举值 / MaidInfinityColor.PARTS_COLOR enum value
	ColData                  PartsColor   `json:"colData"`                  // 单色无限色数据 / Single infinity-color data
	PartColDefs              []PartColDef `json:"partColDefs"`              // 分部颜色数据 / Part-color data
	GradaColDef              *GradaColDef `json:"gradaColDef"`              // 渐变色数据 / Gradient color data
	GradaIsMugen             bool         `json:"gradaIsMugen"`             // 渐变是否按无限色处理 / Whether the gradient is treated as infinity color
}

func NewInfColData() *InfColData {
	return &InfColData{PartsColorType: -1}
}

func (v *InfColData) UnmarshalJSON(data []byte) error {
	type infColDataJSON InfColData
	var value infColDataJSON
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*v = InfColData(value)
	return nil
}

// Colvari 对应 Parts.Menu.Colvari，保存一个颜色变体菜单的入口信息 / Colvari maps Parts.Menu.Colvari and stores color-variant menu entry data
type Colvari struct {
	_struct                struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`
	Version                int           `json:"version"`      // 版本号，游戏 FixVersion 为 1000 / Version value; the game's FixVersion is 1000
	IconColor              PartsColor    `json:"iconColor"`    // 颜色变体图标色 / Color-variant icon color
	IconFileName           string        `json:"iconFileName"` // 颜色变体图标文件名 / Color-variant icon file name
	ReqDefine              string        `json:"reqDefine"`    // 启用该变体所需 DEFINE / DEFINE requirement for enabling this variant
	ColvariDatas           []ColvariData `json:"colvariDatas"` // 颜色变体数据列表 / Color-variant data list
}

// ColvariData 对应 Parts.Menu.Colvari.ColvariData，描述一条颜色变体规则 / ColvariData maps Parts.Menu.Colvari.ColvariData and describes one color-variant rule
type ColvariData struct {
	_struct                 struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata  `codec:"-"`
	Version                 int          `json:"version"`                 // 版本号，游戏 FixVersion 为 1000 / Version value; the game's FixVersion is 1000
	MPN                     string       `json:"mpn"`                     // 目标 MPN 名称，多个值用竖线分隔 / Target MPN names, pipe-separated when multiple
	LayerName               string       `json:"layerName"`               // 保存到 PropBase.savedTexDatas 的图层名 / Layer name saved into PropBase.savedTexDatas
	ColorType               int          `json:"colorType"`               // 主颜色类型枚举 / Primary color type enum
	MaskData                []MaskData   `json:"maskData"`                // 该变体的蒙版状态 / Mask state for this variant
	Alpha                   float32      `json:"alpha"`                   // 乘算透明度 / Multiplicative alpha
	ColData                 PartsColor   `json:"colData"`                 // 单色颜色数据 / Single color data
	PartColDefs             []PartColDef `json:"partColDefs"`             // 分部颜色定义 / Part-color definitions
	GradaColDef             *GradaColDef `json:"gradaColDef"`             // 渐变色定义 / Gradient color definition
	MamaFileName            string       `json:"mamaFileName"`            // 关联的 MAMA 文件名 / Related MAMA file name
	ColorTypeSub            int          `json:"colorTypeSub"`            // 渐变/复合颜色的子类型 / Subtype for gradient or compound color
	UseType                 uint8        `json:"useType"`                 // 使用标志，bit0=alpha，bit1=color / Use flags, bit0=alpha and bit1=color
	SaveInfColDataLinkLayer string       `json:"saveInfColDataLinkLayer"` // 共享无限色数据的源图层名 / Source layer name for shared infinity-color data
	ViewName                string       `json:"viewName"`                // 编辑界面显示名 / Display name in the edit UI
}

// BlendData 对应游戏 BlendData，保存模型 morph 顶点差分 / BlendData maps the game's BlendData and stores model morph vertex deltas
type BlendData struct {
	_struct                struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`
	Name                   string    `json:"name"`    // morph 名称 / Morph name
	VIndex                 []int     `json:"v_index"` // 受影响顶点索引 / Affected vertex indices
	Vert                   []Vector3 `json:"vert"`    // 顶点位置差分 / Vertex position deltas
	Norm                   []Vector3 `json:"norm"`    // 法线差分 / Normal deltas
	Tan                    []Vector4 `json:"tan"`     // 切线差分 / Tangent deltas
}

// SkinThickness 对应 Parts.Model.SkinThickness，保存皮肤厚度修正 / SkinThickness maps Parts.Model.SkinThickness and stores skin-thickness correction data
type SkinThickness struct {
	_struct                struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`
	Use                    bool                      `json:"use"`    // 是否启用皮肤厚度修正 / Whether skin-thickness correction is enabled
	Groups                 map[string]ThicknessGroup `json:"groups"` // 按组名索引的厚度修正组 / Thickness correction groups keyed by group name
}

// ThicknessGroup 对应 Parts.Model.SkinThickness.Group / ThicknessGroup maps Parts.Model.SkinThickness.Group
type ThicknessGroup struct {
	_struct                struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`
	GroupName              string           `json:"groupName"`      // 厚度组名称 / Thickness group name
	StartBoneName          string           `json:"startBoneName"`  // 线段起始骨骼名 / Segment start bone name
	EndBoneName            string           `json:"endBoneName"`    // 线段结束骨骼名 / Segment end bone name
	StepAngleDegree        int              `json:"stepAngleDgree"` // 角度采样步进，字段名保留游戏拼写 stepAngleDgree / Angle sampling step, preserving the game's stepAngleDgree spelling
	Points                 []ThicknessPoint `json:"points"`         // 厚度采样点列表 / Thickness sample points
}

// ThicknessPoint 对应 Parts.Model.SkinThickness.Group.Point / ThicknessPoint maps Parts.Model.SkinThickness.Group.Point
type ThicknessPoint struct {
	_struct                struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`
	TargetBoneName         string                 `json:"targetBoneName"`         // 采样点目标骨骼名 / Target bone name for this sample point
	RatioSegmentStartToEnd float32                `json:"ratioSegmentStartToEnd"` // 点位于起止骨骼线段上的比例 / Ratio along the start-to-end bone segment
	DistanceParAngle       []ThicknessDefPerAngle `json:"distanceParAngle"`       // 按角度记录的默认距离，字段名保留游戏拼写 Par / Default distances by angle, preserving the game's Par spelling
}

// ThicknessDefPerAngle 对应 Parts.Model.SkinThickness.Group.Point.DefPerAngle / ThicknessDefPerAngle maps Parts.Model.SkinThickness.Group.Point.DefPerAngle
type ThicknessDefPerAngle struct {
	_struct                struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`
	AngleDegree            int     `json:"angleDgree"`      // 角度，字段名保留游戏拼写 angleDgree / Angle in degrees, preserving the game's angleDgree spelling
	VertexIndex            int     `json:"vidx"`            // 顶点索引 vidx / Vertex index, vidx
	DefaultDistance        float32 `json:"defaultDistance"` // 默认厚度距离 / Default thickness distance
}

// TupleStringInt 对应 C# Tuple<string,int> 的 MessagePack 数组布局 / TupleStringInt maps the MessagePack array layout of C# Tuple<string,int>
type TupleStringInt struct {
	_struct                struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	*IndexedObjectMetadata `codec:"-"`
	Item1                  string `json:"item1"` // 第一个元组值 / First tuple value
	Item2                  int    `json:"item2"` // 第二个元组值 / Second tuple value
}
