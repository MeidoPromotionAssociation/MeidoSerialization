package COM3D2

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/COM3D2"
)

// MateService 专门处理 .mate 文件的读写
type MateService struct{}

// ReadMateFile 读取 .mate 或 .mate.json 文件并返回对应结构体
func (m *MateService) ReadMateFile(path string) (*COM3D2.Mate, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open .mate file: %w", err)
	}
	defer f.Close()

	if strings.HasSuffix(path, ".json") {
		decoder := json.NewDecoder(f)
		mateData := &COM3D2.Mate{}
		if err := decoder.Decode(mateData); err != nil {
			return nil, fmt.Errorf("failed to read .mate.json file: %w", err)
		}
		return mateData, nil
	}

	br := bufio.NewReader(f)             // 需要 Peek
	mateData, err := COM3D2.ReadMate(br) // 4KB 缓冲，74497 个样本中 90% 文件小于 1.81 KB，平均 1.62 KB，中位数 1.11 KB，最大值 16.01 KB
	if err != nil {
		return nil, fmt.Errorf("parsing the .mate file failed: %w", err)
	}

	return mateData, nil
}

// WriteMateFile 接收 Mate 数据并写入 .mate 或 .mate.json 文件
func (m *MateService) WriteMateFile(path string, mateData *COM3D2.Mate) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("unable to create .mate file: %w", err)
	}
	defer f.Close()

	if strings.HasSuffix(path, ".json") {
		marshal, err := json.Marshal(mateData)
		if err != nil {
			return err
		}
		_, err = f.Write(marshal)
		if err != nil {
			return fmt.Errorf("failed to write to .mate.json file: %w", err)
		}
		return nil
	}

	bw := bufio.NewWriter(f)
	if err := mateData.Dump(bw); err != nil {
		return fmt.Errorf("failed to write to .mate file: %w", err)
	}
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("an error occurred while flush bufio: %w", err)
	}
	return nil
}

// ConvertMateToJson 接收输入文件路径和输出文件路径，将输入文件转换为 .json 文件
func (m *MateService) ConvertMateToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if err := checkConversionContext(ctx); err != nil {
		return err
	}
	if strings.HasSuffix(outputPath, ".mate") {
		outputPath = strings.TrimSuffix(outputPath, ".mate") + ".mate.json"
	}

	mateData, err := m.ReadMateFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read mate file: %w", err)
	}

	if err := checkConversionContext(ctx); err != nil {
		return err
	}
	if err := writeConversionJSON(ctx, outputPath, mateData, maxOutputBytes); err != nil {
		return conversionOutputError("mate JSON", err)
	}
	return nil
}

// ConvertJsonToMate 接收输入文件路径和输出文件路径，将输入文件转换为 .mate 文件
func (m *MateService) ConvertJsonToMate(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if strings.HasSuffix(outputPath, ".json") {
		outputPath = strings.TrimSuffix(outputPath, ".json") + ".mate"
	}

	var mateData *COM3D2.Mate
	if err := readConversionJSON(ctx, inputPath, &mateData); err != nil {
		return fmt.Errorf("parsing the mate.json file failed: %w", err)
	}
	if err := writeConversionBinary(ctx, outputPath, maxOutputBytes, mateData.Dump); err != nil {
		return conversionOutputError("mate", err)
	}
	return nil
}
