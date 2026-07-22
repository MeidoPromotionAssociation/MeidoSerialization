package COM3D2

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/COM3D2"
)

// PskService 专门处理 .psk 文件的读写
type PskService struct{}

// ReadPskFile 读取 .psk 或 .psk.json 文件并返回对应结构体
func (m *PskService) ReadPskFile(path string) (*COM3D2.Psk, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open .psk file: %w", err)
	}
	defer f.Close()

	if strings.HasSuffix(path, ".json") {
		decoder := json.NewDecoder(f)
		pskData := &COM3D2.Psk{}
		if err := decoder.Decode(pskData); err != nil {
			return nil, fmt.Errorf("failed to read .psk.json file: %w", err)
		}
		return pskData, nil
	}

	pskData, err := COM3D2.ReadPsk(f) // 无需缓冲区，380 个样本中 90% 文件小于: 167.00 B，平均 171.28 B，中位数 167.00 B，最大值 477.00 B
	if err != nil {
		return nil, fmt.Errorf("parsing the .psk file failed: %w", err)
	}

	return pskData, nil
}

// WritePskFile 接收 Psk 数据并写入 .psk 或 .psk.json 文件
func (m *PskService) WritePskFile(path string, pskData *COM3D2.Psk) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("unable to create .psk file: %w", err)
	}
	defer f.Close()

	if strings.HasSuffix(path, ".json") {
		marshal, err := json.Marshal(pskData)
		if err != nil {
			return err
		}
		_, err = f.Write(marshal)
		if err != nil {
			return fmt.Errorf("failed to write to .psk.json file: %w", err)
		}
		return nil
	}

	bw := bufio.NewWriter(f)
	if err := pskData.Dump(bw); err != nil {
		return fmt.Errorf("failed to write to .psk file: %w", err)
	}
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("an error occurred while flush bufio: %w", err)
	}
	return nil
}

// ConvertPskToJson 接收输入文件路径和输出文件路径，将输入文件转换为 .json 文件
func (m *PskService) ConvertPskToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if err := checkConversionContext(ctx); err != nil {
		return err
	}
	if strings.HasSuffix(outputPath, ".psk") {
		outputPath = strings.TrimSuffix(outputPath, ".psk") + ".psk.json"
	}

	pskData, err := m.ReadPskFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read psk file: %w", err)
	}

	if err := checkConversionContext(ctx); err != nil {
		return err
	}
	if err := writeConversionJSON(ctx, outputPath, pskData, maxOutputBytes); err != nil {
		return conversionOutputError("psk JSON", err)
	}
	return nil
}

// ConvertJsonToPsk 接收输入文件路径和输出文件路径，将输入文件转换为 .psk 文件
func (m *PskService) ConvertJsonToPsk(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if strings.HasSuffix(outputPath, ".json") {
		outputPath = strings.TrimSuffix(outputPath, ".json") + ".psk"
	}

	var pskData *COM3D2.Psk
	if err := readConversionJSON(ctx, inputPath, &pskData); err != nil {
		return fmt.Errorf("parsing the psk.json file failed: %w", err)
	}
	if err := writeConversionBinary(ctx, outputPath, maxOutputBytes, pskData.Dump); err != nil {
		return conversionOutputError("psk", err)
	}
	return nil
}
