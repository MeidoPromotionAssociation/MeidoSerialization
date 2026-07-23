package aba

import (
	"fmt"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio"
)

// AssetBundleContainerEntry 表示 Unity AssetBundle 对象中的一个 m_Container 记录
// Name 是 AssetBundle.LoadAsset 使用的键，可能与目标对象内部 m_Name 不同，尤其是原生对象
// AssetBundleContainerEntry is one m_Container record from a Unity AssetBundle object
// Name is the key used by AssetBundle.LoadAsset and may differ from the target object's internal m_Name, especially for raw or native objects
type AssetBundleContainerEntry struct {
	Name         string // m_Container 键名或加载名 / m_Container key or load name
	PreloadIndex int32  // m_PreloadTable 起始索引 / Start index in m_PreloadTable
	PreloadSize  int32  // m_PreloadTable 引用数量 / Number of referenced preload entries
	FileID       int32  // PPtr 文件 ID，0 表示当前文件 / PPtr file ID, zero means current file
	PathID       int64  // PPtr 路径 ID / PPtr path ID
}

// GetAssetBundleContainerMap 返回从当前 AssetsFile 所有 Unity AssetBundle 对象收集的 PathID 到加载名映射
// GetAssetBundleContainerMap returns a PathID-to-load-name map collected from all Unity AssetBundle objects in this AssetsFile
func (af *AssetsFile) GetAssetBundleContainerMap() (map[int64]string, error) {
	if af == nil {
		return nil, fmt.Errorf("nil assets file")
	}
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

// GetAssetBundleContainerEntries 解码 Unity AssetBundle 对象布局的稳定前缀 m_Name、m_PreloadTable 和 m_Container
// GetAssetBundleContainerEntries decodes the stable prefix of Unity's AssetBundle object layout containing m_Name, m_PreloadTable, and m_Container
func (af *AssetsFile) GetAssetBundleContainerEntries(info *AssetInfo) ([]AssetBundleContainerEntry, error) {
	if af == nil {
		return nil, fmt.Errorf("nil assets file")
	}
	if info == nil {
		return nil, fmt.Errorf("nil asset info")
	}
	if info.TypeId != ClassIDAssetBundle {
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
	pptrSize := serializedPPtrSize(af.Header.Version)
	if int64(preloadCount) > r.Remaining()/pptrSize {
		return nil, fmt.Errorf("AssetBundle m_PreloadTable size %d requires at least %d bytes but only %d remain", preloadCount, int64(preloadCount)*pptrSize, r.Remaining())
	}
	for i := int64(0); i < int64(preloadCount); i++ {
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
	// 该下界按空对齐键、两个预加载字段和 PPtr 的最小宽度计算
	// This lower bound uses the minimum width of an empty aligned key, two preload fields, and a PPtr
	minimumContainerEntrySize := 4 + 8 + pptrSize
	if int64(containerCount) > r.Remaining()/minimumContainerEntrySize {
		return nil, fmt.Errorf("AssetBundle m_Container size %d requires at least %d bytes but only %d remain", containerCount, int64(containerCount)*minimumContainerEntrySize, r.Remaining())
	}

	entries := makeABACountedSliceForAppend[AssetBundleContainerEntry](int64(containerCount))
	for i := int64(0); i < int64(containerCount); i++ {
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
		if preloadIndex < 0 || preloadSize < 0 || int64(preloadIndex)+int64(preloadSize) > int64(preloadCount) {
			return nil, fmt.Errorf("AssetBundle m_Container[%d] preload range [%d,%d) is outside m_PreloadTable size %d", i, preloadIndex, int64(preloadIndex)+int64(preloadSize), preloadCount)
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

// readSerializedPPtr 按 SerializedFile 格式版本读取 PPtr 的文件 ID 和路径 ID
// readSerializedPPtr reads a PPtr file ID and path ID using the SerializedFile format version
func readSerializedPPtr(r *binaryio.EndianReader, version uint32) (int32, int64, error) {
	if r == nil {
		return 0, 0, fmt.Errorf("nil PPtr reader")
	}
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

// serializedPPtrSize 返回指定 SerializedFile 格式版本中 PPtr 的字节宽度
// serializedPPtrSize returns the byte width of a PPtr in the selected SerializedFile format version
func serializedPPtrSize(version uint32) int64 {
	if version >= 14 {
		return 12
	}
	return 8
}
