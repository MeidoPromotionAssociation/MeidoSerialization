package COM3D2

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
)

// ============================================================================
// DanceObjectData 是 maid_data.bytes、item_data.bytes、event_data.bytes 的共用格式
// DanceObjectData is the shared format for maid_data.bytes, item_data.bytes, and event_data.bytes
// ============================================================================

// DanceObjectData 是舞蹈对象数据，用于描述舞蹈场景中的物体引用关系
// DanceObjectData is dance-object data describing object references in a dance scene
type DanceObjectData struct {
	Entries []DanceObjectEntry `json:"Entries"` // 对象条目列表 / Object entry list
}

// DanceObjectEntry 表示单个舞蹈对象条目
// DanceObjectEntry represents one dance-object entry
type DanceObjectEntry struct {
	TargetMaidNo               int32   `json:"TargetMaidNo"`               // 目标女仆编号 / Target maid number
	ObjectName                 string  `json:"ObjectName"`                 // 对象名称 / Object name
	TopObjectName              string  `json:"TopObjectName"`              // 顶层对象名称 / Top-level object name
	ResourcesPath              string  `json:"ResourcesPath"`              // 资源路径 / Resource path
	TreePath                   string  `json:"TreePath"`                   // 对象树路径 / Object-tree path
	ObjectReferenceTrackIDList []int32 `json:"ObjectReferenceTrackIDList"` // 引用的轨道 ID 列表 / Referenced track ID list
}

// ReadDanceObjectData 从 r 中读取舞蹈对象数据
// ReadDanceObjectData reads dance-object data from r
func ReadDanceObjectData(r io.Reader) (*DanceObjectData, error) {
	reader := stream.NewBinaryReader(r)

	count, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read DanceObjectData count failed: %w", err)
	}
	if err := validateNonNegativeCount("DanceObjectData count", count); err != nil {
		return nil, err
	}

	data := &DanceObjectData{
		Entries: makeCountedSliceForAppend[DanceObjectEntry](count),
	}

	for i := int32(0); i < count; i++ {
		var entry DanceObjectEntry

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
		if err := validateNonNegativeCount(fmt.Sprintf("entry[%d].refCount", i), refCount); err != nil {
			return nil, err
		}

		entry.ObjectReferenceTrackIDList = makeCountedSliceForAppend[int32](refCount)
		for j := int32(0); j < refCount; j++ {
			trackID, err := reader.ReadInt32()
			if err != nil {
				return nil, fmt.Errorf("read entry[%d].trackID[%d] failed: %w", i, j, err)
			}
			entry.ObjectReferenceTrackIDList = append(entry.ObjectReferenceTrackIDList, trackID)
		}
		data.Entries = append(data.Entries, entry)
	}

	return data, nil
}

// Dump 将舞蹈对象数据写出到 w
// Dump writes dance-object data to w
func (d *DanceObjectData) Dump(w io.Writer) error {
	if d == nil {
		return fmt.Errorf("nil DanceObjectData")
	}
	entryCount, err := collectionCountInt32("DanceObjectData entry count", int64(len(d.Entries)))
	if err != nil {
		return err
	}
	refCounts := make([]int32, len(d.Entries))
	for i := range d.Entries {
		refCounts[i], err = collectionCountInt32(fmt.Sprintf("DanceObjectData entry[%d] reference count", i), int64(len(d.Entries[i].ObjectReferenceTrackIDList)))
		if err != nil {
			return err
		}
	}
	writer := stream.NewBinaryWriter(w)

	if err := writer.WriteInt32(entryCount); err != nil {
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
		if err := writer.WriteInt32(refCounts[i]); err != nil {
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
// TimelineData 是 timeline_data.bytes 格式
// TimelineData is the timeline_data.bytes format
// ============================================================================

const (
	// TimelineSignature 是时间线文件头标识
	// TimelineSignature is the timeline file header marker
	TimelineSignature = "BaseData"
	// TimelineFinish 是时间线结束标识
	// TimelineFinish is the timeline end marker
	TimelineFinish   = "Finish"
	TrackTranslation = "Translation"
	TrackRotation    = "Rotation"
	TrackProperty    = "Property"
	TrackEvent       = "Event"
)

// TimelineData 表示舞蹈时间线数据
// TimelineData represents dance timeline data
type TimelineData struct {
	TotalFrame int32           `json:"TotalFrame"` // 总帧数 / Total frame count
	FrameRate  int32           `json:"FrameRate"`  // 帧率 / Frame rate
	Tracks     []TimelineTrack `json:"Tracks"`     // 轨道列表 / Track list
}

// TimelineTrack 表示时间线轨道接口
// TimelineTrack represents the timeline-track interface
type TimelineTrack interface {
	// GetTypeName 返回游戏轨道类型字符串
	// GetTypeName returns the game track-type string
	GetTypeName() string
	// read 从当前流读取轨道载荷，不读取外层类型字符串
	// read reads the track payload from the current stream position without the outer type string
	read(reader *stream.BinaryReader) error
	// write 写入外层类型字符串和轨道载荷
	// write writes the outer type string and track payload
	write(writer *stream.BinaryWriter) error
}

// TranslationTrack 表示位移轨道
// TranslationTrack represents a translation track
type TranslationTrack struct {
	TrackID        int32     `json:"TrackID"`        // 轨道 ID / Track ID
	TotalFrame     int32     `json:"TotalFrame"`     // 总帧数 / Total frame count
	ObjectTreePath string    `json:"ObjectTreePath"` // 对象树路径 / Object-tree path
	PosArray       []Vector3 `json:"PosArray"`       // 每帧位置数组 / Per-frame position array
}

// GetTypeName 返回位移轨道标识 Translation
// GetTypeName returns the Translation track identifier
func (t *TranslationTrack) GetTypeName() string { return TrackTranslation }

// read 读取轨道 ID、帧数、对象路径和每帧 Vector3 位移
// read reads the track ID, frame count, object path, and per-frame Vector3 translations
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
	if err := validateNonNegativeCount("TranslationTrack.TotalFrame", t.TotalFrame); err != nil {
		return err
	}
	t.ObjectTreePath, err = reader.ReadString()
	if err != nil {
		return fmt.Errorf("read TranslationTrack.ObjectTreePath failed: %w", err)
	}

	t.PosArray = makeCountedSliceForAppend[Vector3](t.TotalFrame)
	for i := int32(0); i < t.TotalFrame; i++ {
		var position Vector3
		position.X, err = reader.ReadFloat32()
		if err != nil {
			return fmt.Errorf("read TranslationTrack.PosArray[%d].X failed: %w", i, err)
		}
		position.Y, err = reader.ReadFloat32()
		if err != nil {
			return fmt.Errorf("read TranslationTrack.PosArray[%d].Y failed: %w", i, err)
		}
		position.Z, err = reader.ReadFloat32()
		if err != nil {
			return fmt.Errorf("read TranslationTrack.PosArray[%d].Z failed: %w", i, err)
		}
		t.PosArray = append(t.PosArray, position)
	}
	return nil
}

// write 写入位移轨道类型标识及其所有帧数据
// write writes the translation track identifier and all frame data
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

// RotationTrack 表示旋转轨道
// RotationTrack represents a rotation track
type RotationTrack struct {
	TrackID         int32        `json:"TrackID"`         // 轨道 ID / Track ID
	TotalFrame      int32        `json:"TotalFrame"`      // 总帧数 / Total frame count
	ObjectTreePath  string       `json:"ObjectTreePath"`  // 对象树路径 / Object-tree path
	QuaternionArray []Quaternion `json:"QuaternionArray"` // 每帧四元数数组 / Per-frame quaternion array
}

// GetTypeName 返回旋转轨道标识 Rotation
// GetTypeName returns the Rotation track identifier
func (t *RotationTrack) GetTypeName() string { return TrackRotation }

// read 读取轨道 ID、帧数、对象路径和每帧 Quaternion 旋转
// read reads the track ID, frame count, object path, and per-frame Quaternion rotations
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
	if err := validateNonNegativeCount("RotationTrack.TotalFrame", t.TotalFrame); err != nil {
		return err
	}
	t.ObjectTreePath, err = reader.ReadString()
	if err != nil {
		return fmt.Errorf("read RotationTrack.ObjectTreePath failed: %w", err)
	}

	t.QuaternionArray = makeCountedSliceForAppend[Quaternion](t.TotalFrame)
	for i := int32(0); i < t.TotalFrame; i++ {
		var rotation Quaternion
		rotation.X, err = reader.ReadFloat32()
		if err != nil {
			return err
		}
		rotation.Y, err = reader.ReadFloat32()
		if err != nil {
			return err
		}
		rotation.Z, err = reader.ReadFloat32()
		if err != nil {
			return err
		}
		rotation.W, err = reader.ReadFloat32()
		if err != nil {
			return err
		}
		t.QuaternionArray = append(t.QuaternionArray, rotation)
	}
	return nil
}

// write 写入旋转轨道类型标识及其所有帧数据
// write writes the rotation track identifier and all frame data
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

// PropertyTrack 表示支持 Integer、Float、Vector3、Color 四种值类型的属性轨道
// PropertyTrack represents a property track supporting Integer, Float, Vector3, and Color values
type PropertyTrack struct {
	TrackID        int32             `json:"TrackID"`                 // 轨道 ID / Track ID
	TotalFrame     int32             `json:"TotalFrame"`              // 总帧数 / Total frame count
	ObjectTreePath string            `json:"ObjectTreePath"`          // 对象树路径 / Object-tree path
	ValueType      int32             `json:"ValueType"`               // AMPropertyTrack.ValueType 值类型枚举，0=Integer、2=Float、5=Vector3、6=Color / AMPropertyTrack.ValueType enum, 0=Integer, 2=Float, 5=Vector3, 6=Color
	ComponentName  string            `json:"ComponentName"`           // 组件名称 / Component name
	PropertyName   string            `json:"PropertyName"`            // 属性名称 / Property name
	IntValArray    []int32           `json:"IntValArray,omitempty"`   // ValueType=0 的整数值数组 / Integer value array for ValueType=0
	FloatValArray  []float32         `json:"FloatValArray,omitempty"` // ValueType=2 的浮点值数组 / Float value array for ValueType=2
	Vec3ValArray   []Vector3         `json:"Vec3ValArray,omitempty"`  // ValueType=5 的三维向量数组 / Vector3 value array for ValueType=5
	ColorValArray  []Color           `json:"ColorValArray,omitempty"` // ValueType=6 的颜色数组 / Color value array for ValueType=6
	IndexArray     []KeyValuePairInt `json:"IndexArray"`              // 压缩索引数组 / Compression index array
}

// GetTypeName 返回属性轨道标识 Property
// GetTypeName returns the Property track identifier
func (t *PropertyTrack) GetTypeName() string { return TrackProperty }

// read 读取属性轨道公共字段、按值类型选择的值数组和索引数组
// read reads common property-track fields, the value array selected by type, and the index array
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

	switch t.ValueType {
	case 0:
		// 整数值类型
		// Integer value
		valCount, err := readPropertyValueCount(reader)
		if err != nil {
			return err
		}
		t.IntValArray = makeCountedSliceForAppend[int32](valCount)
		for i := int32(0); i < valCount; i++ {
			value, err := reader.ReadInt32()
			if err != nil {
				return err
			}
			t.IntValArray = append(t.IntValArray, value)
		}
	case 2:
		// 浮点值类型
		// Float value
		valCount, err := readPropertyValueCount(reader)
		if err != nil {
			return err
		}
		t.FloatValArray = makeCountedSliceForAppend[float32](valCount)
		for i := int32(0); i < valCount; i++ {
			value, err := reader.ReadFloat32()
			if err != nil {
				return err
			}
			t.FloatValArray = append(t.FloatValArray, value)
		}
	case 5:
		// 三维向量类型
		// Vector3 value
		valCount, err := readPropertyValueCount(reader)
		if err != nil {
			return err
		}
		t.Vec3ValArray = makeCountedSliceForAppend[Vector3](valCount)
		for i := int32(0); i < valCount; i++ {
			var value Vector3
			value.X, err = reader.ReadFloat32()
			if err != nil {
				return err
			}
			value.Y, err = reader.ReadFloat32()
			if err != nil {
				return err
			}
			value.Z, err = reader.ReadFloat32()
			if err != nil {
				return err
			}
			t.Vec3ValArray = append(t.Vec3ValArray, value)
		}
	case 6:
		// 颜色类型
		// Color value
		valCount, err := readPropertyValueCount(reader)
		if err != nil {
			return err
		}
		t.ColorValArray = makeCountedSliceForAppend[Color](valCount)
		for i := int32(0); i < valCount; i++ {
			var value Color
			value.A, err = reader.ReadFloat32()
			if err != nil {
				return err
			}
			value.R, err = reader.ReadFloat32()
			if err != nil {
				return err
			}
			value.G, err = reader.ReadFloat32()
			if err != nil {
				return err
			}
			value.B, err = reader.ReadFloat32()
			if err != nil {
				return err
			}
			t.ColorValArray = append(t.ColorValArray, value)
		}
	}

	indexCount, err := reader.ReadInt32()
	if err != nil {
		return fmt.Errorf("read PropertyTrack index count failed: %w", err)
	}
	if err := validateNonNegativeCount("PropertyTrack index count", indexCount); err != nil {
		return err
	}
	t.IndexArray = makeCountedSliceForAppend[KeyValuePairInt](indexCount)
	for i := int32(0); i < indexCount; i++ {
		var pair KeyValuePairInt
		pair.Key, err = reader.ReadInt32()
		if err != nil {
			return err
		}
		pair.Value, err = reader.ReadInt32()
		if err != nil {
			return err
		}
		t.IndexArray = append(t.IndexArray, pair)
	}
	return nil
}

// readPropertyValueCount 读取并验证属性轨道值数组数量
// readPropertyValueCount reads and validates the property-track value-array count
func readPropertyValueCount(reader *stream.BinaryReader) (int32, error) {
	count, err := reader.ReadInt32()
	if err != nil {
		return 0, fmt.Errorf("read PropertyTrack value count failed: %w", err)
	}
	if err := validateNonNegativeCount("PropertyTrack value count", count); err != nil {
		return 0, err
	}
	return count, nil
}

// write 写入属性轨道公共字段、值数组和压缩索引数组
// write writes the property-track common fields, value array, and compressed index array
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

// EventParameter 表示支持数组嵌套的递归事件参数
// EventParameter represents a recursive event parameter supporting nested arrays
type EventParameter struct {
	ValueType int32            `json:"ValueType"`           // 值类型枚举 / Value-type enum
	ValBool   bool             `json:"ValBool,omitempty"`   // ValueType=13 的 Boolean 值 / Boolean value for ValueType=13
	ValInt    int32            `json:"ValInt,omitempty"`    // ValueType=0 或 1 的 Integer 或 Long 值 / Integer or Long value for ValueType=0 or 1
	ValFloat  float32          `json:"ValFloat,omitempty"`  // ValueType=2 或 3 的 Float 或 Double 值 / Float or Double value for ValueType=2 or 3
	ValVect2  *Vector2         `json:"ValVect2,omitempty"`  // ValueType=4 的 Vector2 值 / Vector2 value for ValueType=4
	ValVect3  *Vector3         `json:"ValVect3,omitempty"`  // ValueType=5 的 Vector3 值 / Vector3 value for ValueType=5
	ValVect4  *Vector4         `json:"ValVect4,omitempty"`  // ValueType=6 的 Vector4 值 / Vector4 value for ValueType=6
	ValColor  *Color           `json:"ValColor,omitempty"`  // ValueType=7 的 Color 值 / Color value for ValueType=7
	ValRect   *Rect            `json:"ValRect,omitempty"`   // ValueType=8 的 Rect 值 / Rect value for ValueType=8
	ValString string           `json:"ValString,omitempty"` // ValueType=9、10、11 的 String、Char 或 Object 值 / String, Char, or Object value for ValueType=9, 10, or 11
	Array     []EventParameter `json:"Array,omitempty"`     // ValueType=12 的数组值 / Array value for ValueType=12
}

const maxEventParameterDepth int64 = 256

// readEventParameter 读取一个事件参数并限制递归深度
// readEventParameter reads one event parameter with a recursion-depth limit
func readEventParameter(reader *stream.BinaryReader) (EventParameter, error) {
	return readEventParameterDepth(reader, 0)
}

// readEventParameterDepth 按 ValueType 读取标量或递归数组参数
// readEventParameterDepth reads a scalar or recursively nested array according to ValueType
func readEventParameterDepth(reader *stream.BinaryReader, depth int64) (EventParameter, error) {
	var p EventParameter
	var err error
	if depth >= maxEventParameterDepth {
		return p, fmt.Errorf("EventParameter nesting exceeds %d", maxEventParameterDepth)
	}

	p.ValueType, err = reader.ReadInt32()
	if err != nil {
		return p, fmt.Errorf("read EventParameter.ValueType failed: %w", err)
	}

	// 数组类型
	// Array value
	if p.ValueType == 12 {
		count, err := reader.ReadInt32()
		if err != nil {
			return p, err
		}
		if err := validateNonNegativeCount("EventParameter array count", count); err != nil {
			return p, err
		}
		p.Array = makeCountedSliceForAppend[EventParameter](count)
		for i := int32(0); i < count; i++ {
			item, err := readEventParameterDepth(reader, depth+1)
			if err != nil {
				return p, err
			}
			p.Array = append(p.Array, item)
		}
		return p, nil
	}

	switch p.ValueType {
	case 0, 1:
		// Integer 或 Long
		// Integer or Long
		p.ValInt, err = reader.ReadInt32()
	case 2, 3:
		// Float 或 Double
		// Float or Double
		p.ValFloat, err = reader.ReadFloat32()
	case 4:
		// 二维向量类型
		// Vector2 value
		v := &Vector2{}
		v.X, err = reader.ReadFloat32()
		if err == nil {
			v.Y, err = reader.ReadFloat32()
		}
		p.ValVect2 = v
	case 5:
		// 三维向量类型
		// Vector3 value
		v := &Vector3{}
		v.X, err = reader.ReadFloat32()
		if err == nil {
			v.Y, err = reader.ReadFloat32()
		}
		if err == nil {
			v.Z, err = reader.ReadFloat32()
		}
		p.ValVect3 = v
	case 6:
		// 四维向量类型
		// Vector4 value
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
	case 7:
		// 颜色类型
		// Color value
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
	case 8:
		// 矩形类型
		// Rect value
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
	case 9, 10:
		// String 或 Char
		// String or Char
		p.ValString, err = reader.ReadString()
	case 13:
		// 布尔类型
		// Boolean value
		p.ValBool, err = reader.ReadBool()
	case 11:
		// Object 以对象树路径字符串保存
		// Object stored as an object-tree path string
		p.ValString, err = reader.ReadString()
	}

	if err != nil {
		return p, fmt.Errorf("read EventParameter value (type=%d) failed: %w", p.ValueType, err)
	}
	return p, nil
}

// writeEventParameter 写入一个事件参数并限制递归深度
// writeEventParameter writes one event parameter with a recursion-depth limit
func writeEventParameter(writer *stream.BinaryWriter, p EventParameter) error {
	return writeEventParameterDepth(writer, p, 0)
}

// writeEventParameterDepth 按 ValueType 写入标量或递归数组参数
// writeEventParameterDepth writes a scalar or recursively nested array according to ValueType
func writeEventParameterDepth(writer *stream.BinaryWriter, p EventParameter, depth int64) error {
	if depth >= maxEventParameterDepth {
		return fmt.Errorf("EventParameter nesting exceeds %d", maxEventParameterDepth)
	}
	if err := writer.WriteInt32(p.ValueType); err != nil {
		return err
	}

	// 数组类型
	// Array value
	if p.ValueType == 12 {
		count, err := collectionCountInt32("EventParameter array count", int64(len(p.Array)))
		if err != nil {
			return err
		}
		if err := writer.WriteInt32(count); err != nil {
			return err
		}
		for _, item := range p.Array {
			if err := writeEventParameterDepth(writer, item, depth+1); err != nil {
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

// MethodData 表示事件方法数据
// MethodData represents event method data
type MethodData struct {
	StartFrame    int32            `json:"StartFrame"`       // 触发帧 / Trigger frame
	ComponentName string           `json:"ComponentName"`    // 组件名称 / Component name
	MethodName    string           `json:"MethodName"`       // 方法名称 / Method name
	Params        []EventParameter `json:"Params,omitempty"` // 方法参数列表 / Method parameter list
}

// EventTrack 表示事件轨道
// EventTrack represents an event track
type EventTrack struct {
	TrackID         int32        `json:"TrackID"`         // 轨道 ID / Track ID
	TotalFrame      int32        `json:"TotalFrame"`      // 总帧数 / Total frame count
	ObjectTreePath  string       `json:"ObjectTreePath"`  // 对象树路径 / Object-tree path
	MethodDataArray []MethodData `json:"MethodDataArray"` // 方法数据数组 / Method data array
}

// GetTypeName 返回事件轨道标识 Event
// GetTypeName returns the Event track identifier
func (t *EventTrack) GetTypeName() string { return TrackEvent }

// read 读取事件轨道公共字段和方法数据数组
// read reads the event-track common fields and method-data array
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
	if err := validateNonNegativeCount("EventTrack method count", methodCount); err != nil {
		return err
	}

	t.MethodDataArray = makeCountedSliceForAppend[MethodData](methodCount)
	for i := int32(0); i < methodCount; i++ {
		var md MethodData
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
			if err := validateNonNegativeCount(fmt.Sprintf("EventTrack method[%d] parameter count", i), paramCount); err != nil {
				return err
			}
			md.Params = makeCountedSliceForAppend[EventParameter](paramCount)
			for j := int32(0); j < paramCount; j++ {
				parameter, err := readEventParameter(reader)
				if err != nil {
					return err
				}
				md.Params = append(md.Params, parameter)
			}
		}
		t.MethodDataArray = append(t.MethodDataArray, md)
	}
	return nil
}

// write 写入事件轨道类型标识、公共字段和方法数据数组
// write writes the event-track identifier, common fields, and method-data array
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
// TimelineData 顶层 Read 和 Dump
// Top-level TimelineData Read and Dump
// ============================================================================

// ReadTimelineData 从 r 中读取一个 timeline_data.bytes 文件
// ReadTimelineData reads one timeline_data.bytes file from r
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
// Dump writes TimelineData to w
func (d *TimelineData) Dump(w io.Writer) error {
	if err := validateTimelineDataForDump(d); err != nil {
		return err
	}
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

// validateTimelineDataForDump 检查时间线帧数、轨道类型、数组长度和事件参数是否可写入游戏格式
// validateTimelineDataForDump checks timeline counts, track types, array lengths, and event parameters before writing the game format
func validateTimelineDataForDump(data *TimelineData) error {
	if data == nil {
		return fmt.Errorf("nil TimelineData")
	}
	if err := validateNonNegativeCount("TimelineData.TotalFrame", data.TotalFrame); err != nil {
		return err
	}
	for index, track := range data.Tracks {
		path := fmt.Sprintf("TimelineData.Tracks[%d]", index)
		switch value := track.(type) {
		case *TranslationTrack:
			if value == nil {
				return fmt.Errorf("%s is nil", path)
			}
			if err := validateTrackFrameArray(path, value.TotalFrame, int64(len(value.PosArray))); err != nil {
				return err
			}
		case *RotationTrack:
			if value == nil {
				return fmt.Errorf("%s is nil", path)
			}
			if err := validateTrackFrameArray(path, value.TotalFrame, int64(len(value.QuaternionArray))); err != nil {
				return err
			}
		case *PropertyTrack:
			if value == nil {
				return fmt.Errorf("%s is nil", path)
			}
			if err := validateNonNegativeCount(path+".TotalFrame", value.TotalFrame); err != nil {
				return err
			}
			if err := validatePropertyTrackForDump(path, value); err != nil {
				return err
			}
		case *EventTrack:
			if value == nil {
				return fmt.Errorf("%s is nil", path)
			}
			if err := validateNonNegativeCount(path+".TotalFrame", value.TotalFrame); err != nil {
				return err
			}
			if _, err := collectionCountInt32(path+".MethodDataArray count", int64(len(value.MethodDataArray))); err != nil {
				return err
			}
			for methodIndex := range value.MethodDataArray {
				methodPath := fmt.Sprintf("%s.MethodDataArray[%d]", path, methodIndex)
				method := &value.MethodDataArray[methodIndex]
				if _, err := collectionCountInt32(methodPath+".Params count", int64(len(method.Params))); err != nil {
					return err
				}
				for parameterIndex := range method.Params {
					if err := validateEventParameterForDump(fmt.Sprintf("%s.Params[%d]", methodPath, parameterIndex), method.Params[parameterIndex], 0); err != nil {
						return err
					}
				}
			}
		default:
			return fmt.Errorf("%s has unsupported type %T", path, track)
		}
	}
	return nil
}

// validateTrackFrameArray 验证轨道声明帧数与实际帧数组长度一致
// validateTrackFrameArray verifies that the declared track frame count matches the frame-array length
func validateTrackFrameArray(path string, totalFrame int32, arrayLength int64) error {
	if err := validateNonNegativeCount(path+".TotalFrame", totalFrame); err != nil {
		return err
	}
	if int64(totalFrame) != arrayLength {
		return fmt.Errorf("%s TotalFrame=%d but frame array length=%d", path, totalFrame, arrayLength)
	}
	return nil
}

// validatePropertyTrackForDump 验证属性值类型对应的数组和压缩索引数量
// validatePropertyTrackForDump validates the value array selected by the property type and the compressed index count
func validatePropertyTrackForDump(path string, track *PropertyTrack) error {
	lengths := map[int32]int64{
		0: int64(len(track.IntValArray)),
		2: int64(len(track.FloatValArray)),
		5: int64(len(track.Vec3ValArray)),
		6: int64(len(track.ColorValArray)),
	}
	selectedLength, known := lengths[track.ValueType]
	if known {
		if _, err := collectionCountInt32(path+" value count", selectedLength); err != nil {
			return err
		}
	}
	for valueType, length := range lengths {
		if length != 0 && valueType != track.ValueType {
			return fmt.Errorf("%s ValueType=%d would discard the ValueType=%d array with %d values", path, track.ValueType, valueType, length)
		}
	}
	if _, err := collectionCountInt32(path+".IndexArray count", int64(len(track.IndexArray))); err != nil {
		return err
	}
	return nil
}

// validateEventParameterForDump 递归验证事件参数的数组深度、值类型和复合值指针
// validateEventParameterForDump recursively validates event-array depth, value types, and composite-value pointers
func validateEventParameterForDump(path string, parameter EventParameter, depth int64) error {
	if depth >= maxEventParameterDepth {
		return fmt.Errorf("%s nesting exceeds %d", path, maxEventParameterDepth)
	}
	if parameter.ValueType == 12 {
		if _, err := collectionCountInt32(path+" array count", int64(len(parameter.Array))); err != nil {
			return err
		}
		for index := range parameter.Array {
			if err := validateEventParameterForDump(fmt.Sprintf("%s.Array[%d]", path, index), parameter.Array[index], depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if len(parameter.Array) != 0 {
		return fmt.Errorf("%s ValueType=%d would discard %d array elements", path, parameter.ValueType, len(parameter.Array))
	}
	switch parameter.ValueType {
	case 4:
		if parameter.ValVect2 == nil {
			return fmt.Errorf("%s Vector2 value is nil", path)
		}
	case 5:
		if parameter.ValVect3 == nil {
			return fmt.Errorf("%s Vector3 value is nil", path)
		}
	case 6:
		if parameter.ValVect4 == nil {
			return fmt.Errorf("%s Vector4 value is nil", path)
		}
	case 7:
		if parameter.ValColor == nil {
			return fmt.Errorf("%s Color value is nil", path)
		}
	case 8:
		if parameter.ValRect == nil {
			return fmt.Errorf("%s Rect value is nil", path)
		}
	}
	return nil
}

// ============================================================================
// JSON 反序列化使用多态轨道
// JSON deserialization uses polymorphic tracks
// ============================================================================

// timelineDataJSON 是用于 JSON 反序列化的中间结构
// timelineDataJSON is the intermediate structure used for JSON deserialization
type timelineDataJSON struct {
	TotalFrame int32             `json:"TotalFrame"` // 时间线总帧数 / Total timeline frames
	FrameRate  int32             `json:"FrameRate"`  // 每秒帧数 / Frames per second
	Tracks     []json.RawMessage `json:"Tracks"`     // 原始轨道对象 / Raw track objects
}

// trackTypeProbe 用于探测轨道类型
// trackTypeProbe is used to probe a track type
type trackTypeProbe struct {
	TypeName string `json:"TypeName"` // 轨道类型标识 / Track type identifier
}

// MarshalJSON 序列化 TimelineData 时为每个轨道附加 TypeName 字段
// MarshalJSON adds a TypeName field to each track while serializing TimelineData
func (d *TimelineData) MarshalJSON() ([]byte, error) {
	if d == nil {
		return []byte("null"), nil
	}
	out := struct {
		TotalFrame int32             `json:"TotalFrame"` // 时间线总帧数 / Total timeline frames
		FrameRate  int32             `json:"FrameRate"`  // 每秒帧数 / Frames per second
		Tracks     []json.RawMessage `json:"Tracks"`     // 编码后的轨道对象 / Encoded track objects
	}{
		TotalFrame: d.TotalFrame,
		FrameRate:  d.FrameRate,
	}

	for index, track := range d.Tracks {
		typeName, err := timelineTrackTypeName(track)
		if err != nil {
			return nil, fmt.Errorf("marshal track[%d]: %w", index, err)
		}
		raw, err := json.Marshal(track)
		if err != nil {
			return nil, err
		}

		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil {
			return nil, err
		}
		if obj == nil {
			return nil, fmt.Errorf("marshal track[%d]: track encoded as null", index)
		}
		typeNameRaw, err := json.Marshal(typeName)
		if err != nil {
			return nil, err
		}
		obj["TypeName"] = typeNameRaw

		merged, err := json.Marshal(obj)
		if err != nil {
			return nil, err
		}
		out.Tracks = append(out.Tracks, merged)
	}
	return json.Marshal(out)
}

// timelineTrackTypeName 返回轨道具体实现对应的 JSON 类型标识
// timelineTrackTypeName returns the JSON type identifier for a concrete track implementation
func timelineTrackTypeName(track TimelineTrack) (string, error) {
	switch value := track.(type) {
	case *TranslationTrack:
		if value == nil {
			return "", fmt.Errorf("nil TranslationTrack")
		}
		return TrackTranslation, nil
	case *RotationTrack:
		if value == nil {
			return "", fmt.Errorf("nil RotationTrack")
		}
		return TrackRotation, nil
	case *PropertyTrack:
		if value == nil {
			return "", fmt.Errorf("nil PropertyTrack")
		}
		return TrackProperty, nil
	case *EventTrack:
		if value == nil {
			return "", fmt.Errorf("nil EventTrack")
		}
		return TrackEvent, nil
	default:
		return "", fmt.Errorf("unsupported track type %T", track)
	}
}

// UnmarshalJSON 反序列化 TimelineData，并根据 TypeName 字段创建对应的轨道实现
// UnmarshalJSON deserializes TimelineData and creates the corresponding track implementation from TypeName
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
