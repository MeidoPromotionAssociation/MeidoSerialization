package COM3D2

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/COM3D2"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/binaryio/stream"
)

// DanceBytesType 表示 .bytes 文件子类型
type DanceBytesType string

const (
	DanceBytesTimeline   DanceBytesType = "timeline"    // timeline_data.bytes
	DanceBytesObjectData DanceBytesType = "object_data" // maid_data.bytes / item_data.bytes / event_data.bytes
	DanceBytesUnknown    DanceBytesType = "unknown"     // 无法识别
)

// DanceService 处理舞蹈相关的 .bytes 文件读写
type DanceService struct{}

// SniffDanceBytesType 通过读取文件头判断 .bytes 文件子类型
// "BaseData" 字符串开头则为 timeline；否则视为 object_data（其首字段为 int32 count）
func (s *DanceService) SniffDanceBytesType(path string) (DanceBytesType, error) {
	f, err := os.Open(path)
	if err != nil {
		return DanceBytesUnknown, fmt.Errorf("cannot open file: %w", err)
	}
	defer f.Close()

	reader := stream.NewBinaryReader(f)
	sig, err := reader.ReadString()
	if err == nil && sig == COM3D2.TimelineSignature {
		return DanceBytesTimeline, nil
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return DanceBytesUnknown, fmt.Errorf("seek failed: %w", err)
	}

	reader = stream.NewBinaryReader(f)
	count, err := reader.ReadInt32()
	if err != nil {
		return DanceBytesUnknown, fmt.Errorf("read int32 count failed: %w", err)
	}
	if count < 0 || count > 100000 {
		return DanceBytesUnknown, fmt.Errorf("invalid object count: %d", count)
	}
	return DanceBytesObjectData, nil
}

// ============================================================================
// DanceObjectData
// ============================================================================

// ReadDanceObjectDataFile 读取 maid_data/item_data/event_data .bytes 或 .json 文件
func (s *DanceService) ReadDanceObjectDataFile(path string) (*COM3D2.DanceObjectData, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open dance object data file: %w", err)
	}
	defer f.Close()

	if strings.HasSuffix(strings.ToLower(path), ".json") {
		decoder := json.NewDecoder(f)
		data := &COM3D2.DanceObjectData{}
		if err := decoder.Decode(data); err != nil {
			return nil, fmt.Errorf("failed to read dance object data json: %w", err)
		}
		return data, nil
	}

	br := bufio.NewReader(f)
	data, err := COM3D2.ReadDanceObjectData(br)
	if err != nil {
		return nil, fmt.Errorf("parsing dance object data failed: %w", err)
	}
	return data, nil
}

// WriteDanceObjectDataFile 写出 maid_data/item_data/event_data .bytes 或 .json 文件
func (s *DanceService) WriteDanceObjectDataFile(path string, data *COM3D2.DanceObjectData) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("unable to create dance object data file: %w", err)
	}
	defer f.Close()

	if strings.HasSuffix(strings.ToLower(path), ".json") {
		marshal, err := json.Marshal(data)
		if err != nil {
			return err
		}
		if _, err := f.Write(marshal); err != nil {
			return fmt.Errorf("failed to write dance object data json: %w", err)
		}
		return nil
	}

	bw := bufio.NewWriter(f)
	if err := data.Dump(bw); err != nil {
		return fmt.Errorf("failed to write dance object data: %w", err)
	}
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("flush bufio failed: %w", err)
	}
	return nil
}

// ConvertDanceObjectDataToJson 将 .bytes 转为 .json
func (s *DanceService) ConvertDanceObjectDataToJson(ctx context.Context, inputPath, outputPath string, maxOutputBytes int64) error {
	if err := checkConversionContext(ctx); err != nil {
		return err
	}
	if !strings.HasSuffix(strings.ToLower(outputPath), ".json") {
		outputPath = outputPath + ".json"
	}

	data, err := s.ReadDanceObjectDataFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read dance object data: %w", err)
	}

	if err := checkConversionContext(ctx); err != nil {
		return err
	}
	if err := writeConversionJSON(ctx, outputPath, data, maxOutputBytes); err != nil {
		return conversionOutputError("dance object JSON", err)
	}
	return nil
}

// ConvertJsonToDanceObjectData 将 .json 转为 .bytes
func (s *DanceService) ConvertJsonToDanceObjectData(ctx context.Context, inputPath, outputPath string, maxOutputBytes int64) error {
	if strings.HasSuffix(strings.ToLower(outputPath), ".json") {
		outputPath = strings.TrimSuffix(outputPath, ".json")
	}

	var data *COM3D2.DanceObjectData
	if err := readConversionJSON(ctx, inputPath, &data); err != nil {
		return fmt.Errorf("parsing json failed: %w", err)
	}
	if err := writeConversionBinary(ctx, outputPath, maxOutputBytes, data.Dump); err != nil {
		return conversionOutputError("dance object", err)
	}
	return nil
}

// ============================================================================
// TimelineData
// ============================================================================

// ReadTimelineDataFile 读取 timeline_data.bytes 或对应 .json 文件
func (s *DanceService) ReadTimelineDataFile(path string) (*COM3D2.TimelineData, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open timeline file: %w", err)
	}
	defer f.Close()

	if strings.HasSuffix(strings.ToLower(path), ".json") {
		decoder := json.NewDecoder(f)
		data := &COM3D2.TimelineData{}
		if err := decoder.Decode(data); err != nil {
			return nil, fmt.Errorf("failed to read timeline json: %w", err)
		}
		return data, nil
	}

	br := bufio.NewReader(f)
	data, err := COM3D2.ReadTimelineData(br)
	if err != nil {
		return nil, fmt.Errorf("parsing timeline data failed: %w", err)
	}
	return data, nil
}

// WriteTimelineDataFile 写出 timeline_data.bytes 或对应 .json 文件
func (s *DanceService) WriteTimelineDataFile(path string, data *COM3D2.TimelineData) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("unable to create timeline file: %w", err)
	}
	defer f.Close()

	if strings.HasSuffix(strings.ToLower(path), ".json") {
		marshal, err := json.Marshal(data)
		if err != nil {
			return err
		}
		if _, err := f.Write(marshal); err != nil {
			return fmt.Errorf("failed to write timeline json: %w", err)
		}
		return nil
	}

	bw := bufio.NewWriter(f)
	if err := data.Dump(bw); err != nil {
		return fmt.Errorf("failed to write timeline data: %w", err)
	}
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("flush bufio failed: %w", err)
	}
	return nil
}

// ConvertTimelineDataToJson 将 timeline_data.bytes 转为 .json
func (s *DanceService) ConvertTimelineDataToJson(ctx context.Context, inputPath, outputPath string, maxOutputBytes int64) error {
	if err := checkConversionContext(ctx); err != nil {
		return err
	}
	if !strings.HasSuffix(strings.ToLower(outputPath), ".json") {
		outputPath = outputPath + ".json"
	}

	data, err := s.ReadTimelineDataFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read timeline data: %w", err)
	}

	if err := checkConversionContext(ctx); err != nil {
		return err
	}
	if err := writeConversionJSON(ctx, outputPath, data, maxOutputBytes); err != nil {
		return conversionOutputError("timeline JSON", err)
	}
	return nil
}

// ConvertJsonToTimelineData 将 .json 转为 timeline_data.bytes
func (s *DanceService) ConvertJsonToTimelineData(ctx context.Context, inputPath, outputPath string, maxOutputBytes int64) error {
	if strings.HasSuffix(strings.ToLower(outputPath), ".json") {
		outputPath = strings.TrimSuffix(outputPath, ".json")
	}

	var data *COM3D2.TimelineData
	if err := readConversionJSON(ctx, inputPath, &data); err != nil {
		return fmt.Errorf("parsing json failed: %w", err)
	}
	if err := writeConversionBinary(ctx, outputPath, maxOutputBytes, data.Dump); err != nil {
		return conversionOutputError("timeline", err)
	}
	return nil
}
