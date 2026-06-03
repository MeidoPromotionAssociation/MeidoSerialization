package aba

import (
	"fmt"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio"
)

// AssetBundleContainerEntry 表示 AssetBundle 对象中的一个 m_Container 记录 / AssetBundleContainerEntry is one m_Container record from an AssetBundle object
// Name 是 AssetBundle.LoadAsset 使用的 key，可能与目标对象内部 m_Name 不同，尤其是原生对象 / Name is the key used by AssetBundle.LoadAsset and may differ from the target object's internal m_Name, especially for raw/native objects
type AssetBundleContainerEntry struct {
	Name         string // m_Container 键名或加载名 / m_Container key or load name
	PreloadIndex int32  // m_PreloadTable 起始索引 / Start index in m_PreloadTable
	PreloadSize  int32  // m_PreloadTable 引用数量 / Number of referenced preload entries
	FileID       int32  // PPtr 文件 ID，0 表示当前文件 / PPtr file ID, zero means current file
	PathID       int64  // PPtr 路径 ID / PPtr path ID
}

// GetAssetBundleContainerMap 返回从所有 AssetBundle 对象收集的 PathID 到加载名映射 / GetAssetBundleContainerMap returns a PathID to load-name map collected from all AssetBundle objects in this AssetsFile
func (af *AssetsFile) GetAssetBundleContainerMap() (map[int64]string, error) {
	out := map[int64]string{}
	for i := range af.Metadata.AssetInfos {
		info := &af.Metadata.AssetInfos[i]
		if info.TypeId != ClassIDAssetBundle {
			continue
		}
		entries, err := af.GetAssetBundleContainerEntries(info)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.FileID == 0 && entry.PathID != 0 {
				out[entry.PathID] = entry.Name
			}
		}
	}
	return out, nil
}

// GetAssetBundleContainerEntries 解码 Unity AssetBundle 对象布局的稳定前缀：m_Name、m_PreloadTable 和 m_Container / GetAssetBundleContainerEntries decodes the stable prefix of Unity's AssetBundle object layout: m_Name, m_PreloadTable, and m_Container
func (af *AssetsFile) GetAssetBundleContainerEntries(info *AssetInfo) ([]AssetBundleContainerEntry, error) {
	if info == nil || info.TypeId != ClassIDAssetBundle {
		return nil, fmt.Errorf("asset is not AssetBundle")
	}
	data, err := af.GetAssetData(info)
	if err != nil {
		return nil, err
	}

	r := binaryio.NewEndianReader(data, af.byteOrder())
	if _, err := r.ReadAlignedString(); err != nil {
		return nil, fmt.Errorf("read AssetBundle m_Name: %w", err)
	}

	preloadCount, err := r.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read AssetBundle m_PreloadTable size: %w", err)
	}
	if preloadCount < 0 {
		return nil, fmt.Errorf("negative AssetBundle m_PreloadTable size %d", preloadCount)
	}
	for i := 0; i < int(preloadCount); i++ {
		if _, _, err := readSerializedPPtr(r, af.Header.Version); err != nil {
			return nil, fmt.Errorf("read AssetBundle m_PreloadTable[%d]: %w", i, err)
		}
	}

	containerCount, err := r.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read AssetBundle m_Container size: %w", err)
	}
	if containerCount < 0 {
		return nil, fmt.Errorf("negative AssetBundle m_Container size %d", containerCount)
	}

	entries := make([]AssetBundleContainerEntry, 0, containerCount)
	for i := 0; i < int(containerCount); i++ {
		name, err := r.ReadAlignedString()
		if err != nil {
			return nil, fmt.Errorf("read AssetBundle m_Container[%d] key: %w", i, err)
		}
		preloadIndex, err := r.ReadInt32()
		if err != nil {
			return nil, fmt.Errorf("read AssetBundle m_Container[%d].preloadIndex: %w", i, err)
		}
		preloadSize, err := r.ReadInt32()
		if err != nil {
			return nil, fmt.Errorf("read AssetBundle m_Container[%d].preloadSize: %w", i, err)
		}
		fileID, pathID, err := readSerializedPPtr(r, af.Header.Version)
		if err != nil {
			return nil, fmt.Errorf("read AssetBundle m_Container[%d].asset: %w", i, err)
		}
		entries = append(entries, AssetBundleContainerEntry{
			Name:         name,
			PreloadIndex: preloadIndex,
			PreloadSize:  preloadSize,
			FileID:       fileID,
			PathID:       pathID,
		})
	}
	return entries, nil
}

func readSerializedPPtr(r *binaryio.EndianReader, version uint32) (int32, int64, error) {
	fileID, err := r.ReadInt32()
	if err != nil {
		return 0, 0, err
	}
	if version >= 14 {
		pathID, err := r.ReadInt64()
		return fileID, pathID, err
	}
	pathID, err := r.ReadInt32()
	return fileID, int64(pathID), err
}
