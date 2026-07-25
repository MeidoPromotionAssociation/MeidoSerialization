package aba

import (
	"bytes"
	"fmt"
	"math"
)

// InlineMeshStreamData 将 Mesh 的外部顶点载荷写回 m_VertexData.m_DataSize 并清空 m_StreamData
// InlineMeshStreamData writes an external Mesh vertex payload back into m_VertexData.m_DataSize and clears m_StreamData
func (af *AssetsFile) InlineMeshStreamData(info *AssetInfo, resolver AbaFileRangeResolver) ([]byte, bool, error) {
	if af == nil {
		return nil, false, fmt.Errorf("nil assets file")
	}
	if info == nil {
		return nil, false, fmt.Errorf("nil asset info")
	}
	if info.TypeId != ClassIDMesh {
		return nil, false, fmt.Errorf("asset PathID=%d has class ID %d instead of Mesh", info.PathId, info.TypeId)
	}
	root, err := af.ReadAssetValue(info)
	if err != nil {
		return nil, false, err
	}
	stream := root.Field("m_StreamData")
	if stream == nil {
		return copyAssetData(af, info)
	}
	streamInfo, err := readStreamingInfo(stream)
	if err != nil {
		return nil, false, fmt.Errorf("read Mesh PathID=%d m_StreamData: %w", info.PathId, err)
	}
	changed := streamInfo.Offset != 0 || streamInfo.Size != 0 || streamInfo.Path != ""
	if !changed {
		return copyAssetData(af, info)
	}
	vertexData := root.FieldPath("m_VertexData", "m_DataSize")
	if vertexData == nil {
		return nil, false, fmt.Errorf("Mesh PathID=%d has no m_VertexData.m_DataSize field", info.PathId)
	}
	inlineData, ok := vertexData.Bytes()
	if !ok {
		return nil, false, fmt.Errorf("Mesh PathID=%d m_VertexData.m_DataSize is not a byte array", info.PathId)
	}
	if streamInfo.Size > 0 {
		if len(inlineData) != 0 {
			return nil, false, fmt.Errorf("Mesh PathID=%d has both inline vertex data and external stream data", info.PathId)
		}
		payload, err := resolveStreamingPayload("Mesh", info.PathId, streamInfo, resolver)
		if err != nil {
			return nil, false, err
		}
		vertexData.Value = payload
		vertexData.Children = nil
	}
	clearStreamingInfo(stream)
	data, err := af.EncodeAssetValue(info, root)
	if err != nil {
		return nil, false, fmt.Errorf("encode inline Mesh PathID=%d: %w", info.PathId, err)
	}
	return data, true, nil
}

// InlineAudioClipStreamData 将 AudioClip 的外部 m_Resource 载荷改为对象字段后的内联音频数据
// InlineAudioClipStreamData converts an external AudioClip m_Resource payload into audio bytes appended after the serialized object fields
func (af *AssetsFile) InlineAudioClipStreamData(info *AssetInfo, resolver AbaFileRangeResolver) ([]byte, bool, error) {
	if af == nil {
		return nil, false, fmt.Errorf("nil assets file")
	}
	if info == nil {
		return nil, false, fmt.Errorf("nil asset info")
	}
	if info.TypeId != ClassIDAudioClip {
		return nil, false, fmt.Errorf("asset PathID=%d has class ID %d instead of AudioClip", info.PathId, info.TypeId)
	}
	root, consumed, objectSize, err := af.readAssetValuePrefix(info)
	if err != nil {
		return nil, false, err
	}
	raw, err := af.GetAssetData(info)
	if err != nil {
		return nil, false, err
	}
	resource := root.Field("m_Resource")
	if resource == nil {
		if consumed != objectSize {
			return nil, false, fmt.Errorf("AudioClip PathID=%d has %d trailing bytes but no m_Resource field", info.PathId, objectSize-consumed)
		}
		return append([]byte(nil), raw...), false, nil
	}
	streamInfo, err := readStreamingInfo(resource)
	if err != nil {
		return nil, false, fmt.Errorf("read AudioClip PathID=%d m_Resource: %w", info.PathId, err)
	}
	trailing := raw[consumed:]
	if streamInfo.Path == "" {
		if err := validateInlineAudioPayload(info.PathId, trailing, streamInfo.Size); err != nil {
			return nil, false, err
		}
		return append([]byte(nil), raw...), false, nil
	}
	if len(trailing) != 0 {
		return nil, false, fmt.Errorf("AudioClip PathID=%d has both external m_Resource data and %d inline trailing bytes", info.PathId, len(trailing))
	}
	payload, err := resolveStreamingPayload("AudioClip", info.PathId, streamInfo, resolver)
	if err != nil {
		return nil, false, err
	}
	clearStreamingInfo(resource)
	if size := firstTypeTreeField(resource, "size", "m_Size"); size != nil {
		size.Value = uint64(len(payload))
	}
	encoded, err := af.EncodeAssetValue(info, root)
	if err != nil {
		return nil, false, fmt.Errorf("encode inline AudioClip PathID=%d: %w", info.PathId, err)
	}
	encoded = append(encoded, payload...)
	return encoded, true, nil
}

// resolveStreamingPayload 校验流范围并通过 resolver 读取完整载荷
// resolveStreamingPayload validates a stream range and reads the complete payload through the resolver
func resolveStreamingPayload(typeName string, pathID int64, streamInfo StreamingInfo, resolver AbaFileRangeResolver) ([]byte, error) {
	if streamInfo.Size == 0 {
		return nil, nil
	}
	if resolver == nil {
		return nil, fmt.Errorf("%s PathID=%d requires an external stream resolver", typeName, pathID)
	}
	streamName := normalizeStreamDataPath(streamInfo.Path)
	if streamName == "" {
		return nil, fmt.Errorf("%s PathID=%d has stream size %d with an empty stream path", typeName, pathID, streamInfo.Size)
	}
	if streamInfo.Size > math.MaxInt64 {
		return nil, fmt.Errorf("%s PathID=%d stream size %d exceeds Int64 resolver range", typeName, pathID, streamInfo.Size)
	}
	payload, err := resolver(streamName, streamInfo.Offset, int64(streamInfo.Size))
	if err != nil {
		return nil, fmt.Errorf("read %s PathID=%d stream data %q: %w", typeName, pathID, streamName, err)
	}
	if uint64(len(payload)) != streamInfo.Size {
		return nil, fmt.Errorf("%s PathID=%d stream resolver returned %d bytes instead of %d", typeName, pathID, len(payload), streamInfo.Size)
	}
	return append([]byte(nil), payload...), nil
}

// validateInlineAudioPayload 确认 AudioClip 尾随数据与 m_Size 一致，并只允许最多七字节的零对齐填充
// validateInlineAudioPayload verifies AudioClip trailing data against m_Size and permits at most seven zero alignment bytes
func validateInlineAudioPayload(pathID int64, trailing []byte, size uint64) error {
	if size > uint64(len(trailing)) {
		return fmt.Errorf("AudioClip PathID=%d declares %d inline bytes but only %d remain", pathID, size, len(trailing))
	}
	payloadEnd := int64(size)
	padding := trailing[payloadEnd:]
	if len(padding) > 7 || !bytes.Equal(padding, make([]byte, len(padding))) {
		return fmt.Errorf("AudioClip PathID=%d has %d unexpected bytes after its %d-byte inline payload", pathID, len(padding), size)
	}
	return nil
}

// copyAssetData 返回与源对象分离的原始字节副本并报告未发生更改
// copyAssetData returns a detached copy of the raw object bytes and reports that no change occurred
func copyAssetData(af *AssetsFile, info *AssetInfo) ([]byte, bool, error) {
	data, err := af.GetAssetData(info)
	if err != nil {
		return nil, false, err
	}
	return append([]byte(nil), data...), false, nil
}
