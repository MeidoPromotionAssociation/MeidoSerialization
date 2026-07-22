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

// MenuService 专门处理 .menu 文件的读写
type MenuService struct{}

// ReadMenuFile 读取 .menu 或 .menu.json 文件并返回对应结构体
func (s *MenuService) ReadMenuFile(path string) (*COM3D2.Menu, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open .menu file: %w", err)
	}
	defer f.Close()

	if strings.HasSuffix(path, ".json") {
		decoder := json.NewDecoder(f)
		menuData := &COM3D2.Menu{}
		if err := decoder.Decode(menuData); err != nil {
			return nil, fmt.Errorf("failed to read .menu.json file: %w", err)
		}
		return menuData, nil
	}

	br := bufio.NewReader(f)             // 需要 Peek
	menuData, err := COM3D2.ReadMenu(br) // 4KB 缓冲，134279 个样本中 90% 文件小于: 1.14 KB，平均 898.32 B，中位数 687.00 B，最大值 19.39 KB
	if err != nil {
		return nil, fmt.Errorf("parsing the .menu file failed: %w", err)
	}
	return menuData, nil
}

// WriteMenuFile 接收 Menu 数据并写入 .menu 或 .menu.json 文件
func (s *MenuService) WriteMenuFile(path string, menuData *COM3D2.Menu) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("unable to create .menu file: %w", err)
	}
	defer f.Close()

	if strings.HasSuffix(path, ".json") {
		marshal, err := json.Marshal(menuData)
		if err != nil {
			return err
		}
		_, err = f.Write(marshal)
		if err != nil {
			return fmt.Errorf("failed to write to .menu.json file: %w", err)
		}
		return nil
	}

	bw := bufio.NewWriter(f)
	if err := menuData.Dump(bw); err != nil {
		return fmt.Errorf("failed to write to .menu file: %w", err)
	}
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("an error occurred while flush bufio: %w", err)
	}
	return nil
}

// ConvertMenuToJson 接收输入文件路径和输出文件路径，将输入文件转换为 .json 文件
func (s *MenuService) ConvertMenuToJson(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if err := checkConversionContext(ctx); err != nil {
		return err
	}
	if strings.HasSuffix(outputPath, ".menu") {
		outputPath = strings.TrimSuffix(outputPath, ".menu") + ".menu.json"
	}

	menuData, err := s.ReadMenuFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read menu file: %w", err)
	}
	if err := checkConversionContext(ctx); err != nil {
		return err
	}
	if err := writeConversionJSON(ctx, outputPath, menuData, maxOutputBytes); err != nil {
		return conversionOutputError("menu JSON", err)
	}
	return nil
}

// ConvertJsonToMenu 接收输入文件路径和输出文件路径，将输入文件转换为 .menu 文件
func (s *MenuService) ConvertJsonToMenu(ctx context.Context, inputPath string, outputPath string, maxOutputBytes int64) error {
	if strings.HasSuffix(outputPath, ".json") {
		outputPath = strings.TrimSuffix(outputPath, ".json") + ".menu"
	}

	var menuData *COM3D2.Menu
	if err := readConversionJSON(ctx, inputPath, &menuData); err != nil {
		return fmt.Errorf("parsing the menu.json file failed: %w", err)
	}
	if err := writeConversionBinary(ctx, outputPath, maxOutputBytes, menuData.Dump); err != nil {
		return conversionOutputError("menu", err)
	}
	return nil
}
