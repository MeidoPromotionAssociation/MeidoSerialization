package KCES

import (
	"fmt"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/msgpack"
	"github.com/ugorji/go/codec"
)

// KCES2GradationSaveFilePrefix 是 system.dat 中 KCES2 渐变保存虚拟文件的名称前缀
// KCES2GradationSaveFilePrefix is the filename prefix for KCES2 gradation-save virtual files in system.dat
const KCES2GradationSaveFilePrefix = "KCES2GradSv"

// KCES2GradPointsData 对应 KCES2.KCES2GradPointsData 的六槽渐变保存对象 / KCES2GradPointsData matches the six-slot gradation-save object KCES2.KCES2GradPointsData
type KCES2GradPointsData struct {
	_struct                     struct{}          `codec:",toarray"`                   // 强制按数组编码 / Forces array encoding
	GradPointParam              []map[int32]int32 `json:"gradPointParam"`              // 各控制点的九项颜色整数参数 / Nine integer color parameters for each control point
	ControlPointPosValue        []float32         `json:"controlPointPosValue"`        // 控制点中心位置值 / Control-point center position values
	GradationPointPositionRates []float32         `json:"gradationPointPositionRates"` // 渐变点位置比例 / Gradation-point position rates
	PointRangeAfterRates        []float32         `json:"pointRangeAfterRates"`        // 控制点后侧范围比例 / Control-point after-range rates
	PointRangeBeforeRates       []float32         `json:"pointRangeBeforeRates"`       // 控制点前侧范围比例 / Control-point before-range rates
	IsSave                      int32             `json:"isSave"`                      // 是否存在保存数据，0 表示否，1 表示是 / Whether saved data exists with zero for false and one for true
}

// DecodeKCES2GradPointsData 解码 EditData/KCES2GradSv{index} 中未压缩的 MessagePack 数据
// DecodeKCES2GradPointsData decodes uncompressed MessagePack data stored at EditData/KCES2GradSv{index}
func DecodeKCES2GradPointsData(data []byte) (*KCES2GradPointsData, error) {
	var value *KCES2GradPointsData
	if err := msgpack.DecodeMsgpack(data, &value); err != nil {
		return nil, fmt.Errorf("decode KCES2GradPointsData msgpack: %w", err)
	}
	return value, nil
}

// EncodeKCES2GradPointsData 编码 EditData/KCES2GradSv{index} 使用的未压缩 MessagePack 数据
// EncodeKCES2GradPointsData encodes uncompressed MessagePack data used by EditData/KCES2GradSv{index}
func EncodeKCES2GradPointsData(value *KCES2GradPointsData) ([]byte, error) {
	return encodeUncompressedIndexedMsgpack(value, "KCES2GradPointsData")
}

// NewKCES2GradPointsData 创建带空列表和明确保存状态的新 KCES2 渐变记录
// NewKCES2GradPointsData creates a new KCES2 gradation record with empty lists and an explicit saved state
func NewKCES2GradPointsData(isSave bool) *KCES2GradPointsData {
	value := &KCES2GradPointsData{
		GradPointParam:              make([]map[int32]int32, 0),
		ControlPointPosValue:        make([]float32, 0),
		GradationPointPositionRates: make([]float32, 0),
		PointRangeAfterRates:        make([]float32, 0),
		PointRangeBeforeRates:       make([]float32, 0),
	}
	if isSave {
		value.IsSave = 1
	}
	return value
}

// CodecEncodeSelf 按共享 indexed-object 规则编码 KCES2GradPointsData
// CodecEncodeSelf encodes KCES2GradPointsData using the shared indexed-object rules
func (v KCES2GradPointsData) CodecEncodeSelf(e *codec.Encoder) {
	msgpack.EncodeIndexedObjectSelf(e, &v)
}

// CodecDecodeSelf 按共享 indexed-object 规则解码 KCES2GradPointsData
// CodecDecodeSelf decodes KCES2GradPointsData using the shared indexed-object rules
func (v *KCES2GradPointsData) CodecDecodeSelf(d *codec.Decoder) {
	msgpack.DecodeIndexedObjectSelf(d, v)
}
