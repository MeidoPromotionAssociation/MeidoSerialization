package COM3D2

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
)

// ============================================================================
// DanceObjectData — maid_data.bytes / item_data.bytes / event_data.bytes 共用格式
// ============================================================================

// DanceObjectData 舞蹈对象数据，用于描述舞蹈场景中的物体引用关系
type DanceObjectData struct {
	Entries []DanceObjectEntry `json:"Entries"` // 对象条目列表
}

// DanceObjectEntry 单个舞蹈对象条目
type DanceObjectEntry struct {
	TargetMaidNo               int32   `json:"TargetMaidNo"`               // 目标女仆编号
	ObjectName                 string  `json:"ObjectName"`                 // 对象名称
	TopObjectName              string  `json:"TopObjectName"`              // 顶层对象名称
	ResourcesPath              string  `json:"ResourcesPath"`              // 资源路径
	TreePath                   string  `json:"TreePath"`                   // 对象树路径
	ObjectReferenceTrackIDList []int32 `json:"ObjectReferenceTrackIDList"` // 引用的轨道 ID 列表
}

// ReadDanceObjectData 从 r 中读取舞蹈对象数据
func ReadDanceObjectData(r io.Reader) (*DanceObjectData, error) {
	reader := stream.NewBinaryReader(r)

	count, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read DanceObjectData count failed: %w", err)
	}

	data := &DanceObjectData{
		Entries: make([]DanceObjectEntry, count),
	}

	for i := int32(0); i < count; i++ {
		entry := &data.Entries[i]

		entry.TargetMaidNo, err = reader.ReadInt32()
		if err != nil {
			return nil, fmt.Errorf("read entry[%d].TargetMaidNo failed: %w", i, err)
		}
		entry.ObjectName, err = reader.ReadString()
		if err != nil {
			return nil, fmt.Errorf("read entry[%d].ObjectName failed: %w", i, err)
		}

		entry.TopObjectName, err = reader.ReadString()
		if err != nil {
			return nil, fmt.Errorf("read entry[%d].TopObjectName failed: %w", i, err)
		}

		entry.ResourcesPath, err = reader.ReadString()
		if err != nil {
			return nil, fmt.Errorf("read entry[%d].ResourcesPath failed: %w", i, err)
		}

		entry.TreePath, err = reader.ReadString()
		if err != nil {
			return nil, fmt.Errorf("read entry[%d].TreePath failed: %w", i, err)
		}

		refCount, err := reader.ReadInt32()
		if err != nil {
			return nil, fmt.Errorf("read entry[%d].refCount failed: %w", i, err)
		}

		entry.ObjectReferenceTrackIDList = make([]int32, refCount)
		for j := int32(0); j < refCount; j++ {
			entry.ObjectReferenceTrackIDList[j], err = reader.ReadInt32()
			if err != nil {
				return nil, fmt.Errorf("read entry[%d].trackID[%d] failed: %w", i, j, err)
			}
		}
	}

	return data, nil
}

// Dump 将舞蹈对象数据写出到 w
func (d *DanceObjectData) Dump(w io.Writer) error {
	writer := stream.NewBinaryWriter(w)

	if err := writer.WriteInt32(int32(len(d.Entries))); err != nil {
		return fmt.Errorf("write DanceObjectData count failed: %w", err)
	}

	for i, entry := range d.Entries {
		if err := writer.WriteInt32(entry.TargetMaidNo); err != nil {
			return fmt.Errorf("write entry[%d].TargetMaidNo failed: %w", i, err)
		}
		if err := writer.WriteString(entry.ObjectName); err != nil {
			return fmt.Errorf("write entry[%d].ObjectName failed: %w", i, err)
		}
		if err := writer.WriteString(entry.TopObjectName); err != nil {
			return fmt.Errorf("write entry[%d].TopObjectName failed: %w", i, err)
		}
		if err := writer.WriteString(entry.ResourcesPath); err != nil {
			return fmt.Errorf("write entry[%d].ResourcesPath failed: %w", i, err)
		}
		if err := writer.WriteString(entry.TreePath); err != nil {
			return fmt.Errorf("write entry[%d].TreePath failed: %w", i, err)
		}
		if err := writer.WriteInt32(int32(len(entry.ObjectReferenceTrackIDList))); err != nil {
			return fmt.Errorf("write entry[%d].refCount failed: %w", i, err)
		}
		for j, trackID := range entry.ObjectReferenceTrackIDList {
			if err := writer.WriteInt32(trackID); err != nil {
				return fmt.Errorf("write entry[%d].trackID[%d] failed: %w", i, j, err)
			}
		}
	}

	return nil
}

// ============================================================================
// TimelineData — timeline_data.bytes 格式
// ============================================================================

const (
	TimelineSignature = "BaseData" // 时间线文件头标识
	TimelineFinish    = "Finish"   // 时间线结束标识
	TrackTranslation  = "Translation"
	TrackRotation     = "Rotation"
	TrackProperty     = "Property"
	TrackEvent        = "Event"
)

// TimelineData 舞蹈时间线数据
type TimelineData struct {
	TotalFrame int32           `json:"TotalFrame"` // 总帧数
	FrameRate  int32           `json:"FrameRate"`  // 帧率
	Tracks     []TimelineTrack `json:"Tracks"`     // 轨道列表
}

// TimelineTrack 时间线轨道接口
type TimelineTrack interface {
	GetTypeName() string
	read(reader *stream.BinaryReader) error
	write(writer *stream.BinaryWriter) error
}

// TranslationTrack 位移轨道
type TranslationTrack struct {
	TrackID        int32     `json:"TrackID"`        // 轨道 ID
	TotalFrame     int32     `json:"TotalFrame"`     // 总帧数
	ObjectTreePath string    `json:"ObjectTreePath"` // 对象树路径
	PosArray       []Vector3 `json:"PosArray"`       // 每帧位置数组
}

func (t *TranslationTrack) GetTypeName() string { return TrackTranslation }

func (t *TranslationTrack) read(reader *stream.BinaryReader) error {
	var err error
	t.TrackID, err = reader.ReadInt32()
	if err != nil {
		return fmt.Errorf("read TranslationTrack.TrackID failed: %w", err)
	}
	t.TotalFrame, err = reader.ReadInt32()
	if err != nil {
		return fmt.Errorf("read TranslationTrack.TotalFrame failed: %w", err)
	}
	t.ObjectTreePath, err = reader.ReadString()
	if err != nil {
		return fmt.Errorf("read TranslationTrack.ObjectTreePath failed: %w", err)
	}

	t.PosArray = make([]Vector3, t.TotalFrame)
	for i := int32(0); i < t.TotalFrame; i++ {
		t.PosArray[i].X, err = reader.ReadFloat32()
		if err != nil {
			return fmt.Errorf("read TranslationTrack.PosArray[%d].X failed: %w", i, err)
		}
		t.PosArray[i].Y, err = reader.ReadFloat32()
		if err != nil {
			return fmt.Errorf("read TranslationTrack.PosArray[%d].Y failed: %w", i, err)
		}
		t.PosArray[i].Z, err = reader.ReadFloat32()
		if err != nil {
			return fmt.Errorf("read TranslationTrack.PosArray[%d].Z failed: %w", i, err)
		}
	}
	return nil
}

func (t *TranslationTrack) write(writer *stream.BinaryWriter) error {
	if err := writer.WriteString(TrackTranslation); err != nil {
		return err
	}
	if err := writer.WriteInt32(t.TrackID); err != nil {
		return err
	}
	if err := writer.WriteInt32(t.TotalFrame); err != nil {
		return err
	}
	if err := writer.WriteString(t.ObjectTreePath); err != nil {
		return err
	}
	for i := range t.PosArray {
		if err := writer.WriteFloat32(t.PosArray[i].X); err != nil {
			return err
		}
		if err := writer.WriteFloat32(t.PosArray[i].Y); err != nil {
			return err
		}
		if err := writer.WriteFloat32(t.PosArray[i].Z); err != nil {
			return err
		}
	}
	return nil
}

// RotationTrack 旋转轨道
type RotationTrack struct {
	TrackID         int32        `json:"TrackID"`         // 轨道 ID
	TotalFrame      int32        `json:"TotalFrame"`      // 总帧数
	ObjectTreePath  string       `json:"ObjectTreePath"`  // 对象树路径
	QuaternionArray []Quaternion `json:"QuaternionArray"` // 每帧四元数数组
}

func (t *RotationTrack) GetTypeName() string { return TrackRotation }

func (t *RotationTrack) read(reader *stream.BinaryReader) error {
	var err error
	t.TrackID, err = reader.ReadInt32()
	if err != nil {
		return fmt.Errorf("read RotationTrack.TrackID failed: %w", err)
	}
	t.TotalFrame, err = reader.ReadInt32()
	if err != nil {
		return fmt.Errorf("read RotationTrack.TotalFrame failed: %w", err)
	}
	t.ObjectTreePath, err = reader.ReadString()
	if err != nil {
		return fmt.Errorf("read RotationTrack.ObjectTreePath failed: %w", err)
	}

	t.QuaternionArray = make([]Quaternion, t.TotalFrame)
	for i := int32(0); i < t.TotalFrame; i++ {
		t.QuaternionArray[i].X, err = reader.ReadFloat32()
		if err != nil {
			return err
		}
		t.QuaternionArray[i].Y, err = reader.ReadFloat32()
		if err != nil {
			return err
		}
		t.QuaternionArray[i].Z, err = reader.ReadFloat32()
		if err != nil {
			return err
		}
		t.QuaternionArray[i].W, err = reader.ReadFloat32()
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *RotationTrack) write(writer *stream.BinaryWriter) error {
	if err := writer.WriteString(TrackRotation); err != nil {
		return err
	}
	if err := writer.WriteInt32(t.TrackID); err != nil {
		return err
	}
	if err := writer.WriteInt32(t.TotalFrame); err != nil {
		return err
	}
	if err := writer.WriteString(t.ObjectTreePath); err != nil {
		return err
	}
	for i := range t.QuaternionArray {
		if err := writer.WriteFloat32(t.QuaternionArray[i].X); err != nil {
			return err
		}
		if err := writer.WriteFloat32(t.QuaternionArray[i].Y); err != nil {
			return err
		}
		if err := writer.WriteFloat32(t.QuaternionArray[i].Z); err != nil {
			return err
		}
		if err := writer.WriteFloat32(t.QuaternionArray[i].W); err != nil {
			return err
		}
	}
	return nil
}

// PropertyTrack 属性轨道（支持 Integer/Float/Vector3/Color 四种值类型）
type PropertyTrack struct {
	TrackID        int32             `json:"TrackID"`                 // 轨道 ID
	TotalFrame     int32             `json:"TotalFrame"`              // 总帧数
	ObjectTreePath string            `json:"ObjectTreePath"`          // 对象树路径
	ValueType      int32             `json:"ValueType"`               // 值类型枚举（AMPropertyTrack.ValueType）：0=Integer, 2=Float, 5=Vector3, 6=Color
	ComponentName  string            `json:"ComponentName"`           // 组件名称
	PropertyName   string            `json:"PropertyName"`            // 属性名称
	IntValArray    []int32           `json:"IntValArray,omitempty"`   // 整数值数组（ValueType=0）
	FloatValArray  []float32         `json:"FloatValArray,omitempty"` // 浮点值数组（ValueType=2）
	Vec3ValArray   []Vector3         `json:"Vec3ValArray,omitempty"`  // 三维向量数组（ValueType=5）
	ColorValArray  []Color           `json:"ColorValArray,omitempty"` // 颜色数组（ValueType=6）
	IndexArray     []KeyValuePairInt `json:"IndexArray"`              // 压缩索引数组
}

func (t *PropertyTrack) GetTypeName() string { return TrackProperty }

func (t *PropertyTrack) read(reader *stream.BinaryReader) error {
	var err error
	t.TrackID, err = reader.ReadInt32()
	if err != nil {
		return fmt.Errorf("read PropertyTrack.TrackID failed: %w", err)
	}
	t.TotalFrame, err = reader.ReadInt32()
	if err != nil {
		return fmt.Errorf("read PropertyTrack.TotalFrame failed: %w", err)
	}
	t.ObjectTreePath, err = reader.ReadString()
	if err != nil {
		return fmt.Errorf("read PropertyTrack.ObjectTreePath failed: %w", err)
	}
	t.ValueType, err = reader.ReadInt32()
	if err != nil {
		return fmt.Errorf("read PropertyTrack.ValueType failed: %w", err)
	}
	t.ComponentName, err = reader.ReadString()
	if err != nil {
		return fmt.Errorf("read PropertyTrack.ComponentName failed: %w", err)
	}
	t.PropertyName, err = reader.ReadString()
	if err != nil {
		return fmt.Errorf("read PropertyTrack.PropertyName failed: %w", err)
	}

	valCount, err := reader.ReadInt32()
	if err != nil {
		return fmt.Errorf("read PropertyTrack value count failed: %w", err)
	}

	switch t.ValueType {
	case 0: // Integer
		t.IntValArray = make([]int32, valCount)
		for i := int32(0); i < valCount; i++ {
			t.IntValArray[i], err = reader.ReadInt32()
			if err != nil {
				return err
			}
		}
	case 2: // Float
		t.FloatValArray = make([]float32, valCount)
		for i := int32(0); i < valCount; i++ {
			t.FloatValArray[i], err = reader.ReadFloat32()
			if err != nil {
				return err
			}
		}
	case 5: // Vector3
		t.Vec3ValArray = make([]Vector3, valCount)
		for i := int32(0); i < valCount; i++ {
			t.Vec3ValArray[i].X, err = reader.ReadFloat32()
			if err != nil {
				return err
			}
			t.Vec3ValArray[i].Y, err = reader.ReadFloat32()
			if err != nil {
				return err
			}
			t.Vec3ValArray[i].Z, err = reader.ReadFloat32()
			if err != nil {
				return err
			}
		}
	case 6: // Color
		t.ColorValArray = make([]Color, valCount)
		for i := int32(0); i < valCount; i++ {
			t.ColorValArray[i].A, err = reader.ReadFloat32()
			if err != nil {
				return err
			}
			t.ColorValArray[i].R, err = reader.ReadFloat32()
			if err != nil {
				return err
			}
			t.ColorValArray[i].G, err = reader.ReadFloat32()
			if err != nil {
				return err
			}
			t.ColorValArray[i].B, err = reader.ReadFloat32()
			if err != nil {
				return err
			}
		}
	}

	indexCount, err := reader.ReadInt32()
	if err != nil {
		return fmt.Errorf("read PropertyTrack index count failed: %w", err)
	}
	t.IndexArray = make([]KeyValuePairInt, indexCount)
	for i := int32(0); i < indexCount; i++ {
		t.IndexArray[i].Key, err = reader.ReadInt32()
		if err != nil {
			return err
		}
		t.IndexArray[i].Value, err = reader.ReadInt32()
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *PropertyTrack) write(writer *stream.BinaryWriter) error {
	if err := writer.WriteString(TrackProperty); err != nil {
		return err
	}
	if err := writer.WriteInt32(t.TrackID); err != nil {
		return err
	}
	if err := writer.WriteInt32(t.TotalFrame); err != nil {
		return err
	}
	if err := writer.WriteString(t.ObjectTreePath); err != nil {
		return err
	}
	if err := writer.WriteInt32(t.ValueType); err != nil {
		return err
	}
	if err := writer.WriteString(t.ComponentName); err != nil {
		return err
	}
	if err := writer.WriteString(t.PropertyName); err != nil {
		return err
	}

	switch t.ValueType {
	case 0:
		if err := writer.WriteInt32(int32(len(t.IntValArray))); err != nil {
			return err
		}
		for _, v := range t.IntValArray {
			if err := writer.WriteInt32(v); err != nil {
				return err
			}
		}
	case 2:
		if err := writer.WriteInt32(int32(len(t.FloatValArray))); err != nil {
			return err
		}
		for _, v := range t.FloatValArray {
			if err := writer.WriteFloat32(v); err != nil {
				return err
			}
		}
	case 5:
		if err := writer.WriteInt32(int32(len(t.Vec3ValArray))); err != nil {
			return err
		}
		for _, v := range t.Vec3ValArray {
			if err := writer.WriteFloat32(v.X); err != nil {
				return err
			}
			if err := writer.WriteFloat32(v.Y); err != nil {
				return err
			}
			if err := writer.WriteFloat32(v.Z); err != nil {
				return err
			}
		}
	case 6:
		if err := writer.WriteInt32(int32(len(t.ColorValArray))); err != nil {
			return err
		}
		for _, v := range t.ColorValArray {
			if err := writer.WriteFloat32(v.A); err != nil {
				return err
			}
			if err := writer.WriteFloat32(v.R); err != nil {
				return err
			}
			if err := writer.WriteFloat32(v.G); err != nil {
				return err
			}
			if err := writer.WriteFloat32(v.B); err != nil {
				return err
			}
		}
	}

	if err := writer.WriteInt32(int32(len(t.IndexArray))); err != nil {
		return err
	}
	for _, kv := range t.IndexArray {
		if err := writer.WriteInt32(kv.Key); err != nil {
			return err
		}
		if err := writer.WriteInt32(kv.Value); err != nil {
			return err
		}
	}
	return nil
}

// EventParameter 事件参数（递归结构，支持数组嵌套）
type EventParameter struct {
	ValueType int32            `json:"ValueType"`           // 值类型枚举
	ValBool   bool             `json:"ValBool,omitempty"`   // Boolean 值（ValueType=13）
	ValInt    int32            `json:"ValInt,omitempty"`    // Integer/Long 值（ValueType=0,1）
	ValFloat  float32          `json:"ValFloat,omitempty"`  // Float/Double 值（ValueType=2,3）
	ValVect2  *Vector2         `json:"ValVect2,omitempty"`  // Vector2 值（ValueType=4）
	ValVect3  *Vector3         `json:"ValVect3,omitempty"`  // Vector3 值（ValueType=5）
	ValVect4  *Vector4         `json:"ValVect4,omitempty"`  // Vector4 值（ValueType=6）
	ValColor  *Color           `json:"ValColor,omitempty"`  // Color 值（ValueType=7）
	ValRect   *Rect            `json:"ValRect,omitempty"`   // Rect 值（ValueType=8）
	ValString string           `json:"ValString,omitempty"` // String/Char/Object 值（ValueType=9,10,11）
	Array     []EventParameter `json:"Array,omitempty"`     // 数组值（ValueType=12）
}

func readEventParameter(reader *stream.BinaryReader) (EventParameter, error) {
	var p EventParameter
	var err error

	p.ValueType, err = reader.ReadInt32()
	if err != nil {
		return p, fmt.Errorf("read EventParameter.ValueType failed: %w", err)
	}

	if p.ValueType == 12 { // Array
		count, err := reader.ReadInt32()
		if err != nil {
			return p, err
		}
		p.Array = make([]EventParameter, count)
		for i := int32(0); i < count; i++ {
			p.Array[i], err = readEventParameter(reader)
			if err != nil {
				return p, err
			}
		}
		return p, nil
	}

	switch p.ValueType {
	case 0, 1: // Integer, Long
		p.ValInt, err = reader.ReadInt32()
	case 2, 3: // Float, Double
		p.ValFloat, err = reader.ReadFloat32()
	case 4: // Vector2
		v := &Vector2{}
		v.X, err = reader.ReadFloat32()
		if err == nil {
			v.Y, err = reader.ReadFloat32()
		}
		p.ValVect2 = v
	case 5: // Vector3
		v := &Vector3{}
		v.X, err = reader.ReadFloat32()
		if err == nil {
			v.Y, err = reader.ReadFloat32()
		}
		if err == nil {
			v.Z, err = reader.ReadFloat32()
		}
		p.ValVect3 = v
	case 6: // Vector4
		v := &Vector4{}
		v.X, err = reader.ReadFloat32()
		if err == nil {
			v.Y, err = reader.ReadFloat32()
		}
		if err == nil {
			v.Z, err = reader.ReadFloat32()
		}
		if err == nil {
			v.W, err = reader.ReadFloat32()
		}
		p.ValVect4 = v
	case 7: // Color
		c := &Color{}
		c.A, err = reader.ReadFloat32()
		if err == nil {
			c.R, err = reader.ReadFloat32()
		}
		if err == nil {
			c.G, err = reader.ReadFloat32()
		}
		if err == nil {
			c.B, err = reader.ReadFloat32()
		}
		p.ValColor = c
	case 8: // Rect
		r := &Rect{}
		r.XMin, err = reader.ReadFloat32()
		if err == nil {
			r.XMax, err = reader.ReadFloat32()
		}
		if err == nil {
			r.YMin, err = reader.ReadFloat32()
		}
		if err == nil {
			r.YMax, err = reader.ReadFloat32()
		}
		p.ValRect = r
	case 9, 10: // String, Char
		p.ValString, err = reader.ReadString()
	case 13: // Boolean
		p.ValBool, err = reader.ReadBool()
	case 11: // Object (stored as tree path string)
		p.ValString, err = reader.ReadString()
	}

	if err != nil {
		return p, fmt.Errorf("read EventParameter value (type=%d) failed: %w", p.ValueType, err)
	}
	return p, nil
}

func writeEventParameter(writer *stream.BinaryWriter, p EventParameter) error {
	if err := writer.WriteInt32(p.ValueType); err != nil {
		return err
	}

	if p.ValueType == 12 { // Array
		if err := writer.WriteInt32(int32(len(p.Array))); err != nil {
			return err
		}
		for _, item := range p.Array {
			if err := writeEventParameter(writer, item); err != nil {
				return err
			}
		}
		return nil
	}

	switch p.ValueType {
	case 0, 1:
		return writer.WriteInt32(p.ValInt)
	case 2, 3:
		return writer.WriteFloat32(p.ValFloat)
	case 4:
		if err := writer.WriteFloat32(p.ValVect2.X); err != nil {
			return err
		}
		return writer.WriteFloat32(p.ValVect2.Y)
	case 5:
		if err := writer.WriteFloat32(p.ValVect3.X); err != nil {
			return err
		}
		if err := writer.WriteFloat32(p.ValVect3.Y); err != nil {
			return err
		}
		return writer.WriteFloat32(p.ValVect3.Z)
	case 6:
		if err := writer.WriteFloat32(p.ValVect4.X); err != nil {
			return err
		}
		if err := writer.WriteFloat32(p.ValVect4.Y); err != nil {
			return err
		}
		if err := writer.WriteFloat32(p.ValVect4.Z); err != nil {
			return err
		}
		return writer.WriteFloat32(p.ValVect4.W)
	case 7:
		if err := writer.WriteFloat32(p.ValColor.A); err != nil {
			return err
		}
		if err := writer.WriteFloat32(p.ValColor.R); err != nil {
			return err
		}
		if err := writer.WriteFloat32(p.ValColor.G); err != nil {
			return err
		}
		return writer.WriteFloat32(p.ValColor.B)
	case 8:
		if err := writer.WriteFloat32(p.ValRect.XMin); err != nil {
			return err
		}
		if err := writer.WriteFloat32(p.ValRect.XMax); err != nil {
			return err
		}
		if err := writer.WriteFloat32(p.ValRect.YMin); err != nil {
			return err
		}
		return writer.WriteFloat32(p.ValRect.YMax)
	case 9, 10, 11:
		return writer.WriteString(p.ValString)
	case 13:
		return writer.WriteBool(p.ValBool)
	}
	return nil
}

// MethodData 事件方法数据
type MethodData struct {
	StartFrame    int32            `json:"StartFrame"`       // 触发帧
	ComponentName string           `json:"ComponentName"`    // 组件名称
	MethodName    string           `json:"MethodName"`       // 方法名称
	Params        []EventParameter `json:"Params,omitempty"` // 方法参数列表
}

// EventTrack 事件轨道
type EventTrack struct {
	TrackID         int32        `json:"TrackID"`         // 轨道 ID
	TotalFrame      int32        `json:"TotalFrame"`      // 总帧数
	ObjectTreePath  string       `json:"ObjectTreePath"`  // 对象树路径
	MethodDataArray []MethodData `json:"MethodDataArray"` // 方法数据数组
}

func (t *EventTrack) GetTypeName() string { return TrackEvent }

func (t *EventTrack) read(reader *stream.BinaryReader) error {
	var err error
	t.TrackID, err = reader.ReadInt32()
	if err != nil {
		return fmt.Errorf("read EventTrack.TrackID failed: %w", err)
	}
	t.TotalFrame, err = reader.ReadInt32()
	if err != nil {
		return fmt.Errorf("read EventTrack.TotalFrame failed: %w", err)
	}
	t.ObjectTreePath, err = reader.ReadString()
	if err != nil {
		return fmt.Errorf("read EventTrack.ObjectTreePath failed: %w", err)
	}

	methodCount, err := reader.ReadInt32()
	if err != nil {
		return fmt.Errorf("read EventTrack method count failed: %w", err)
	}

	t.MethodDataArray = make([]MethodData, methodCount)
	for i := int32(0); i < methodCount; i++ {
		md := &t.MethodDataArray[i]
		md.StartFrame, err = reader.ReadInt32()
		if err != nil {
			return err
		}
		md.ComponentName, err = reader.ReadString()
		if err != nil {
			return err
		}
		md.MethodName, err = reader.ReadString()
		if err != nil {
			return err
		}

		hasParams, err := reader.ReadBool()
		if err != nil {
			return err
		}
		if hasParams {
			paramCount, err := reader.ReadInt32()
			if err != nil {
				return err
			}
			md.Params = make([]EventParameter, paramCount)
			for j := int32(0); j < paramCount; j++ {
				md.Params[j], err = readEventParameter(reader)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (t *EventTrack) write(writer *stream.BinaryWriter) error {
	if err := writer.WriteString(TrackEvent); err != nil {
		return err
	}
	if err := writer.WriteInt32(t.TrackID); err != nil {
		return err
	}
	if err := writer.WriteInt32(t.TotalFrame); err != nil {
		return err
	}
	if err := writer.WriteString(t.ObjectTreePath); err != nil {
		return err
	}

	if err := writer.WriteInt32(int32(len(t.MethodDataArray))); err != nil {
		return err
	}
	for _, md := range t.MethodDataArray {
		if err := writer.WriteInt32(md.StartFrame); err != nil {
			return err
		}
		if err := writer.WriteString(md.ComponentName); err != nil {
			return err
		}
		if err := writer.WriteString(md.MethodName); err != nil {
			return err
		}

		hasParams := len(md.Params) > 0
		if err := writer.WriteBool(hasParams); err != nil {
			return err
		}
		if hasParams {
			if err := writer.WriteInt32(int32(len(md.Params))); err != nil {
				return err
			}
			for _, p := range md.Params {
				if err := writeEventParameter(writer, p); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// ============================================================================
// TimelineData 顶层 Read/Dump
// ============================================================================

// ReadTimelineData 从 r 中读取一个 timeline_data.bytes 文件
func ReadTimelineData(r io.Reader) (*TimelineData, error) {
	reader := stream.NewBinaryReader(r)

	sig, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read TimelineData signature failed: %w", err)
	}
	if sig != TimelineSignature {
		return nil, fmt.Errorf("invalid TimelineData signature: got %q, want %q", sig, TimelineSignature)
	}

	data := &TimelineData{}
	data.TotalFrame, err = reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read TimelineData.TotalFrame failed: %w", err)
	}
	data.FrameRate, err = reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read TimelineData.FrameRate failed: %w", err)
	}

	for {
		typeName, err := reader.ReadString()
		if err != nil {
			return nil, fmt.Errorf("read track type name failed: %w", err)
		}
		if typeName == TimelineFinish {
			break
		}

		var track TimelineTrack
		switch typeName {
		case TrackTranslation:
			track = &TranslationTrack{}
		case TrackRotation:
			track = &RotationTrack{}
		case TrackProperty:
			track = &PropertyTrack{}
		case TrackEvent:
			track = &EventTrack{}
		default:
			return nil, fmt.Errorf("unknown track type: %q", typeName)
		}

		if err := track.read(reader); err != nil {
			return nil, fmt.Errorf("read %s track failed: %w", typeName, err)
		}
		data.Tracks = append(data.Tracks, track)
	}

	return data, nil
}

// Dump 将 TimelineData 写出到 w
func (d *TimelineData) Dump(w io.Writer) error {
	writer := stream.NewBinaryWriter(w)

	if err := writer.WriteString(TimelineSignature); err != nil {
		return fmt.Errorf("write TimelineData signature failed: %w", err)
	}
	if err := writer.WriteInt32(d.TotalFrame); err != nil {
		return fmt.Errorf("write TimelineData.TotalFrame failed: %w", err)
	}
	if err := writer.WriteInt32(d.FrameRate); err != nil {
		return fmt.Errorf("write TimelineData.FrameRate failed: %w", err)
	}

	for i, track := range d.Tracks {
		if err := track.write(writer); err != nil {
			return fmt.Errorf("write track[%d] failed: %w", i, err)
		}
	}

	if err := writer.WriteString(TimelineFinish); err != nil {
		return fmt.Errorf("write TimelineData finish marker failed: %w", err)
	}
	return nil
}

// ============================================================================
// JSON 反序列化（多态轨道）
// ============================================================================

// timelineDataJSON 用于 JSON 反序列化的中间结构
type timelineDataJSON struct {
	TotalFrame int32             `json:"TotalFrame"`
	FrameRate  int32             `json:"FrameRate"`
	Tracks     []json.RawMessage `json:"Tracks"`
}

// trackTypeProbe 用于探测轨道类型
type trackTypeProbe struct {
	TypeName string `json:"TypeName"`
}

// MarshalJSON 序列化 TimelineData 时为每个轨道附加 TypeName 字段
func (d *TimelineData) MarshalJSON() ([]byte, error) {
	type trackOut struct {
		TypeName string      `json:"TypeName"`
		Track    interface{} `json:"-"`
	}

	out := struct {
		TotalFrame int32             `json:"TotalFrame"`
		FrameRate  int32             `json:"FrameRate"`
		Tracks     []json.RawMessage `json:"Tracks"`
	}{
		TotalFrame: d.TotalFrame,
		FrameRate:  d.FrameRate,
	}

	for _, track := range d.Tracks {
		raw, err := json.Marshal(track)
		if err != nil {
			return nil, err
		}

		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil {
			return nil, err
		}
		typeNameRaw, _ := json.Marshal(track.GetTypeName())
		obj["TypeName"] = typeNameRaw

		merged, err := json.Marshal(obj)
		if err != nil {
			return nil, err
		}
		out.Tracks = append(out.Tracks, merged)
	}
	_ = trackOut{}
	return json.Marshal(out)
}

// UnmarshalJSON 反序列化 TimelineData，根据 TypeName 字段创建对应的轨道实现
func (d *TimelineData) UnmarshalJSON(data []byte) error {
	var raw timelineDataJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	d.TotalFrame = raw.TotalFrame
	d.FrameRate = raw.FrameRate
	d.Tracks = make([]TimelineTrack, 0, len(raw.Tracks))

	for i, trackRaw := range raw.Tracks {
		var probe trackTypeProbe
		if err := json.Unmarshal(trackRaw, &probe); err != nil {
			return fmt.Errorf("probe track[%d] type failed: %w", i, err)
		}

		var track TimelineTrack
		switch probe.TypeName {
		case TrackTranslation:
			track = &TranslationTrack{}
		case TrackRotation:
			track = &RotationTrack{}
		case TrackProperty:
			track = &PropertyTrack{}
		case TrackEvent:
			track = &EventTrack{}
		default:
			return fmt.Errorf("unknown track TypeName: %q at index %d", probe.TypeName, i)
		}

		if err := json.Unmarshal(trackRaw, track); err != nil {
			return fmt.Errorf("unmarshal track[%d] (%s) failed: %w", i, probe.TypeName, err)
		}
		d.Tracks = append(d.Tracks, track)
	}

	return nil
}
