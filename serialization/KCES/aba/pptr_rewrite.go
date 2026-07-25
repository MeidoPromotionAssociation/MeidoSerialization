package aba

import (
	"fmt"
	"math"
	"strings"
)

// PPtrRemapFunc 将一个 Unity PPtr 的文件 ID 和路径 ID 映射为新的目标
// PPtrRemapFunc maps one Unity PPtr file ID and path ID to a new target
type PPtrRemapFunc func(fileID int32, pathID int64) (newFileID int32, newPathID int64, err error)

// RewritePPtrReferences 遍历 TypeTree 值中的全部 PPtr 并原位重写非空引用
// RewritePPtrReferences walks every PPtr in a TypeTree value and rewrites non-null references in place
func RewritePPtrReferences(root *TypeTreeValue, remap PPtrRemapFunc) (int64, error) {
	if root == nil {
		return 0, fmt.Errorf("nil TypeTree root")
	}
	if remap == nil {
		return 0, fmt.Errorf("nil PPtr remapper")
	}
	return rewritePPtrValue(root, remap)
}

// rewritePPtrValue 递归处理当前值及其后代中的 PPtr
// rewritePPtrValue recursively processes PPtrs in the current value and its descendants
func rewritePPtrValue(value *TypeTreeValue, remap PPtrRemapFunc) (int64, error) {
	if value == nil {
		return 0, nil
	}
	if isPPtrTypeTreeValue(value) {
		fileValue := value.Field("m_FileID")
		pathValue := value.Field("m_PathID")
		if fileValue == nil || pathValue == nil {
			return 0, fmt.Errorf("PPtr %s is missing m_FileID or m_PathID", value.Name)
		}
		fileIDRaw, ok := fileValue.Int64()
		if !ok || fileIDRaw < math.MinInt32 || fileIDRaw > math.MaxInt32 {
			return 0, fmt.Errorf("PPtr %s m_FileID is outside Int32 range", value.Name)
		}
		pathID, ok := pathValue.Int64()
		if !ok {
			return 0, fmt.Errorf("PPtr %s m_PathID is outside Int64 range", value.Name)
		}
		if pathID == 0 {
			fileValue.Value = int64(0)
			return 0, nil
		}

		newFileID, newPathID, err := remap(int32(fileIDRaw), pathID)
		if err != nil {
			return 0, fmt.Errorf("remap PPtr %s fileID=%d pathID=%d: %w", value.Name, fileIDRaw, pathID, err)
		}
		if newPathID == 0 {
			return 0, fmt.Errorf("remap PPtr %s returned a null target for non-null pathID=%d", value.Name, pathID)
		}
		fileValue.Value = int64(newFileID)
		pathValue.Value = newPathID
		return 1, nil
	}

	var count int64
	for childIndex := int64(0); childIndex < int64(len(value.Children)); childIndex++ {
		childCount, err := rewritePPtrValue(value.Children[childIndex], remap)
		if err != nil {
			return count, err
		}
		count += childCount
	}
	return count, nil
}

// isPPtrTypeTreeValue 判断值类型名是否表示 Unity PPtr
// isPPtrTypeTreeValue reports whether a value type name denotes a Unity PPtr
func isPPtrTypeTreeValue(value *TypeTreeValue) bool {
	if value == nil {
		return false
	}
	typeName := strings.TrimSpace(value.TypeName)
	return strings.HasPrefix(typeName, "PPtr<") && strings.HasSuffix(typeName, ">")
}
