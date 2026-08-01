package KCES

const (
	// KCES2ColorPresetVersion 是 KCES2 中 CustomColorPresetBase<T>.FixVersion 的值
	// KCES2ColorPresetVersion is the value of CustomColorPresetBase<T>.FixVersion in KCES2
	KCES2ColorPresetVersion = 1005
	// PresetColorExtension 是 KCES2 独立颜色预设文件扩展名
	// PresetColorExtension is the standalone KCES2 color-preset file extension
	PresetColorExtension = ".presetcolor"
)

// NewKCES2ColorPreset 创建使用 1005 版本和十一槽布局的新 KCES2 颜色预设
// NewKCES2ColorPreset creates a new KCES2 color preset using version 1005 and the 11-slot layout
func NewKCES2ColorPreset(instanceGUID string) (*ColorPreset, error) {
	value, err := NewColorPreset(instanceGUID)
	if err != nil {
		return nil, err
	}
	value.Version = KCES2ColorPresetVersion
	value.IndexedArrayWidth = 11
	value.MetaTexts = make(map[string]*string)
	return value, nil
}

// DecodePresetColor 解码独立 .presetcolor 文件并保留实际数组宽度
// DecodePresetColor decodes a standalone .presetcolor file while preserving its actual array width
func DecodePresetColor(data []byte) (*ColorPreset, error) {
	return DecodeColorPreset(data)
}

// EncodePresetColor 编码独立 .presetcolor 文件并保留调用方选择的数组宽度
// EncodePresetColor encodes a standalone .presetcolor file while preserving the caller-selected array width
func EncodePresetColor(value *ColorPreset) ([]byte, error) {
	return EncodeColorPreset(value)
}
