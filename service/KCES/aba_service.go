package KCES

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
)

// AbaService 提供 .aba 文件 Unity AssetBundle 的读取、列出和提取操作 / AbaService provides read, list, and extraction operations for .aba Unity AssetBundle files
type AbaService struct{}

// rawAssetMeta 表示原始 Unity 对象 sidecar 元数据 / rawAssetMeta represents sidecar metadata for a raw Unity object
type rawAssetMeta struct {
	PathID   int64  `json:"pathId"`             // Unity PathID / Unity PathID
	LoadName string `json:"loadName,omitempty"` // AssetBundle m_Container 加载名 / AssetBundle m_Container load name
}

// ReadAba 读取 .aba 文件并返回 Bundle
func (s *AbaService) ReadAba(path string) (*aba.Bundle, *os.File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open .aba file failed: %w", err)
	}

	bundle, err := aba.ReadBundle(f)
	if err != nil {
		f.Close()
		return nil, nil, fmt.Errorf("parse .aba file failed: %w", err)
	}
	return bundle, f, nil
}

// ListAba 列出 .aba 文件中的所有资源
func (s *AbaService) ListAba(path string) ([]aba.AssetEntry, error) {
	bundle, f, err := s.ReadAba(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var allEntries []aba.AssetEntry
	for i, dir := range bundle.BlockInfo.DirectoryInfos {
		if !dir.IsSerialized() {
			continue
		}
		data, err := bundle.GetFileData(i)
		if err != nil {
			continue
		}
		af, err := aba.ReadAssetsFile(data)
		if err != nil {
			continue
		}
		allEntries = append(allEntries, af.GetAssetEntries()...)
	}
	return allEntries, nil
}

// UnpackAba 将 .aba 文件中的所有资源提取到指定目录
func (s *AbaService) UnpackAba(abaPath string, outDir string) error {
	bundle, f, err := s.ReadAba(abaPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if outDir == "" {
		outDir = abaPath + "_unpacked"
	}

	serialized := make(map[int]*aba.AssetsFile)
	serializedByName := make(map[string]*aba.AssetsFile)
	for i, dir := range bundle.BlockInfo.DirectoryInfos {
		if !dir.IsSerialized() {
			continue
		}
		data, err := bundle.GetFileData(i)
		if err != nil {
			return fmt.Errorf("read file %q from bundle failed: %w", dir.Name, err)
		}
		af, err := aba.ReadAssetsFile(data)
		if err != nil {
			return fmt.Errorf("parse AssetsFile %q failed: %w", dir.Name, err)
		}
		serialized[i] = af
		serializedByName[dir.Name] = af
		serializedByName[filepath.Base(dir.Name)] = af
	}
	resolver := aba.BundleAssetResolver(serializedByName)
	streamResolver := bundle.GetFileDataRangeByName

	for i, dir := range bundle.BlockInfo.DirectoryInfos {
		if !dir.IsSerialized() {
			// 非序列化文件（如 .resS）直接提取原始数据
			data, err := bundle.GetFileData(i)
			if err != nil {
				continue
			}
			outPath := filepath.Join(outDir, dir.Name)
			if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
				continue
			}
			os.WriteFile(outPath, data, 0644)
			continue
		}

		af := serialized[i]
		if af == nil {
			continue
		}
		containerNames, _ := af.GetAssetBundleContainerMap()

		entries := af.GetAssetEntries()
		for _, entry := range entries {
			info := findInfo(af, entry.PathId)
			if info == nil {
				continue
			}

			if entry.TypeId == aba.ClassIDAssetBundle {
				continue
			}

			// TextAsset: 提取 m_Script 内容
			if entry.TypeId == aba.ClassIDTextAsset {
				name, script, err := af.GetTextAssetData(info)
				if err != nil || len(script) == 0 {
					continue
				}
				outPath := filepath.Join(outDir, "TextAsset", sanitizeName(name))
				if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
					continue
				}
				os.WriteFile(outPath, script, 0644)
				_ = writeAssetMeta(outPath, entry.PathId, containerNames[entry.PathId])
				continue
			}

			assetBaseName := entry.Name
			if assetBaseName == "" {
				assetBaseName = fmt.Sprintf("asset_%d", entry.PathId)
			}

			// Texture2D: 保留原始 Unity 对象数据用于重打包，同时额外导出 PNG 便于检查。
			if entry.TypeId == aba.ClassIDTexture2D {
				textureDir := filepath.Join(outDir, "Texture2D")
				if err := os.MkdirAll(textureDir, 0755); err == nil {
					if assetData, err := af.GetAssetData(info); err == nil {
						rawPath := filepath.Join(textureDir, textureRawFileName(assetBaseName))
						_ = os.WriteFile(rawPath, assetData, 0644)
						_ = writeAssetMeta(rawPath, entry.PathId, containerNames[entry.PathId])
						_ = writeRawUnityTypeTreeSidecar(rawPath, af, info, entry, containerNames[entry.PathId])
					}
				}
				tex, err := af.GetTexture2DDataRange(info, streamResolver)
				if err == nil {
					outPath := filepath.Join(textureDir, sanitizeName(assetBaseName)+".png")
					if err := os.MkdirAll(filepath.Dir(outPath), 0755); err == nil {
						if err := aba.WriteTexturePNG(tex, outPath); err == nil {
							continue
						}
					}
				}
			}

			// Sprite: 保留原始 Unity 对象数据用于重打包，同时额外导出 PNG 便于检查。
			if entry.TypeId == aba.ClassIDSprite {
				spriteDir := filepath.Join(outDir, "Sprite")
				if err := os.MkdirAll(spriteDir, 0755); err == nil {
					if assetData, err := af.GetAssetData(info); err == nil {
						rawPath := filepath.Join(spriteDir, sanitizeName(assetBaseName)+".sprite.bytes")
						_ = os.WriteFile(rawPath, assetData, 0644)
						_ = writeAssetMeta(rawPath, entry.PathId, containerNames[entry.PathId])
						_ = writeRawUnityTypeTreeSidecar(rawPath, af, info, entry, containerNames[entry.PathId])
					}
				}
				sprite, err := af.GetSpriteExportRange(info, resolver, streamResolver)
				if err == nil {
					outPath := filepath.Join(spriteDir, sanitizeName(assetBaseName)+".png")
					if err := os.MkdirAll(filepath.Dir(outPath), 0755); err == nil {
						if err := aba.WriteSpritePNG(sprite, outPath); err == nil {
							continue
						}
					}
				}
			}

			// Mesh: 保留原始 Unity 对象数据用于重打包，同时额外导出 .crmesh 便于检查。
			if entry.TypeId == aba.ClassIDMesh {
				meshDir := filepath.Join(outDir, "Mesh")
				if err := os.MkdirAll(meshDir, 0755); err == nil {
					if assetData, err := af.GetAssetData(info); err == nil {
						rawPath := filepath.Join(meshDir, sanitizeName(assetBaseName)+".bytes")
						_ = os.WriteFile(rawPath, assetData, 0644)
						_ = writeAssetMeta(rawPath, entry.PathId, containerNames[entry.PathId])
						_ = writeRawUnityTypeTreeSidecar(rawPath, af, info, entry, containerNames[entry.PathId])
					}
					if crmesh, err := af.TryConvertMeshToCRMesh(info, nil); err == nil && len(crmesh) > 0 {
						_ = os.WriteFile(filepath.Join(meshDir, sanitizeName(assetBaseName)+".crmesh"), crmesh, 0644)
						continue
					}
				}
			}

			// 其他类型：提取原始序列化数据
			assetData, err := af.GetAssetData(info)
			if err != nil {
				continue
			}
			typeName := entry.TypeName
			outPath := filepath.Join(outDir, typeName, sanitizeName(assetBaseName)+".bytes")
			if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
				continue
			}
			os.WriteFile(outPath, assetData, 0644)
			_ = writeAssetMeta(outPath, entry.PathId, containerNames[entry.PathId])
			_ = writeRawUnityTypeTreeSidecar(outPath, af, info, entry, containerNames[entry.PathId])
		}
	}
	return nil
}

func writeAssetMeta(assetPath string, pathID int64, loadName string) error {
	data, err := json.MarshalIndent(rawAssetMeta{PathID: pathID, LoadName: loadName}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(assetMetaPath(assetPath), data, 0644)
}

func assetMetaPath(assetPath string) string {
	return assetPath + ".meta.json"
}

func textureRawFileName(name string) string {
	safeName := sanitizeName(name)
	if filepath.Ext(safeName) == "" {
		return safeName + ".texture2d.bytes"
	}
	return safeName + ".bytes"
}

func findInfo(af *aba.AssetsFile, pathId int64) *aba.AssetInfo {
	for i, info := range af.Metadata.AssetInfos {
		if info.PathId == pathId {
			return &af.Metadata.AssetInfos[i]
		}
	}
	return nil
}

func sanitizeName(name string) string {
	// 替换文件系统不允许的字符
	result := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch c {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			result = append(result, '_')
		default:
			result = append(result, c)
		}
	}
	if len(result) == 0 {
		return "unnamed"
	}
	return string(result)
}
