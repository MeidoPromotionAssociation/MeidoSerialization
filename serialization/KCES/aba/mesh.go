package aba

import (
	"encoding/binary"
	"fmt"
	"math"
)

// MeshPrimitiveMode 表示可直接映射到 glTF 的网格图元拓扑 / MeshPrimitiveMode represents mesh primitive topology that maps directly to glTF
type MeshPrimitiveMode uint8

const (
	// MeshPrimitiveModePoints 表示独立点图元 / MeshPrimitiveModePoints represents independent point primitives
	MeshPrimitiveModePoints MeshPrimitiveMode = 0
	// MeshPrimitiveModeLines 表示独立线段图元 / MeshPrimitiveModeLines represents independent line primitives
	MeshPrimitiveModeLines MeshPrimitiveMode = 1
	// MeshPrimitiveModeLineStrip 表示连续线带图元 / MeshPrimitiveModeLineStrip represents a connected line-strip primitive
	MeshPrimitiveModeLineStrip MeshPrimitiveMode = 3
	// MeshPrimitiveModeTriangles 表示独立三角形图元 / MeshPrimitiveModeTriangles represents independent triangle primitives
	MeshPrimitiveModeTriangles MeshPrimitiveMode = 4
)

// MeshPrimitive 表示一个已展开为通用索引拓扑的 Unity SubMesh / MeshPrimitive represents one Unity SubMesh expanded to generic indexed topology
type MeshPrimitive struct {
	Mode    MeshPrimitiveMode // 图元拓扑 / Primitive topology
	Indices []uint32          // 已应用 baseVertex 的顶点索引 / Vertex indices with baseVertex applied
}

// MeshGeometry 表示从 Unity Mesh 中解码的通用几何数据 / MeshGeometry represents generic geometry decoded from a Unity Mesh
type MeshGeometry struct {
	Name       string          // Unity m_Name / Unity m_Name
	Positions  [][3]float32    // 顶点位置 / Vertex positions
	Normals    [][3]float32    // 可选顶点法线 / Optional vertex normals
	Tangents   [][4]float32    // 可选顶点切线 / Optional vertex tangents
	TexCoord0  [][2]float32    // 可选第一组纹理坐标 / Optional first texture-coordinate set
	Colors     [][4]float32    // 可选顶点颜色 / Optional vertex colors
	Primitives []MeshPrimitive // SubMesh 图元 / SubMesh primitives
}

// meshVertexChannel 描述 Unity VertexData 中一个通道的在线布局 / meshVertexChannel describes the wire layout of one Unity VertexData channel
type meshVertexChannel struct {
	Stream    uint8 // 顶点流索引 / Vertex stream index
	Offset    uint8 // 流内字节偏移 / Byte offset within the stream
	Format    uint8 // Unity VertexAttributeFormat / Unity VertexAttributeFormat
	Dimension uint8 // 分量数量 / Component count
}

// DecodeMeshGeometry 使用独立对象内嵌 TypeTree 解码 Unity 2019 及更高版本的 Mesh 顶点和 SubMesh 数据
// DecodeMeshGeometry decodes Unity 2019 and newer Mesh vertex and SubMesh data using the standalone object's embedded TypeTree
func (object *NativeUnityObject) DecodeMeshGeometry() (*MeshGeometry, error) {
	if object == nil || object.ClassID != ClassIDMesh {
		return nil, fmt.Errorf("native Unity object is not a Mesh")
	}
	root, err := object.DecodeValue()
	if err != nil {
		return nil, fmt.Errorf("decode Mesh TypeTree: %w", err)
	}
	vertexData := root.Field("m_VertexData")
	if vertexData == nil {
		return nil, fmt.Errorf("Mesh has no m_VertexData field")
	}
	vertexCountValue, ok := meshUnsignedField(vertexData, "m_VertexCount")
	if !ok || vertexCountValue == 0 || vertexCountValue > math.MaxUint32 {
		return nil, fmt.Errorf("Mesh has invalid m_VertexData.m_VertexCount %d", vertexCountValue)
	}
	vertexCount := uint32(vertexCountValue)
	channelsValue := vertexData.Field("m_Channels")
	if channelsValue == nil || len(channelsValue.Children) == 0 {
		return nil, fmt.Errorf("Mesh has no m_VertexData.m_Channels")
	}
	channels := make([]meshVertexChannel, len(channelsValue.Children))
	for channelIndex := range channelsValue.Children {
		channelValue := channelsValue.Children[channelIndex]
		stream, streamOK := meshUnsignedField(channelValue, "stream", "m_Stream")
		offset, offsetOK := meshUnsignedField(channelValue, "offset", "m_Offset")
		format, formatOK := meshUnsignedField(channelValue, "format", "m_Format")
		dimension, dimensionOK := meshUnsignedField(channelValue, "dimension", "m_Dimension")
		if !streamOK || !offsetOK || !formatOK || !dimensionOK || stream > math.MaxUint8 || offset > math.MaxUint8 || format > 11 || dimension > 4 {
			return nil, fmt.Errorf("Mesh vertex channel %d has an invalid layout", channelIndex)
		}
		channels[channelIndex] = meshVertexChannel{Stream: uint8(stream), Offset: uint8(offset), Format: uint8(format), Dimension: uint8(dimension) & 0x0f}
	}
	vertexBytes, ok := vertexData.Field("m_DataSize").Bytes()
	if !ok || len(vertexBytes) == 0 {
		return nil, fmt.Errorf("Mesh has no inline vertex data; compressed-only Mesh export is unsupported")
	}
	streamOffsets, streamStrides, err := meshVertexStreams(vertexCount, channels, uint64(len(vertexBytes)))
	if err != nil {
		return nil, err
	}

	geometry := &MeshGeometry{}
	geometry.Name, _ = root.Field("m_Name").String()
	if len(channels) == 0 || channels[0].Dimension != 3 {
		return nil, fmt.Errorf("Mesh position channel is absent or does not have three components")
	}
	positions, err := decodeMeshFloatChannel(vertexBytes, object.BigEndian, vertexCount, channels[0], streamOffsets, streamStrides)
	if err != nil {
		return nil, fmt.Errorf("decode Mesh positions: %w", err)
	}
	geometry.Positions = meshFloat3Values(positions)
	if uint64(len(geometry.Positions)) != uint64(vertexCount) {
		return nil, fmt.Errorf("Mesh position count %d does not match vertex count %d", len(geometry.Positions), vertexCount)
	}
	if len(channels) > 1 && channels[1].Dimension == 3 {
		values, err := decodeMeshFloatChannel(vertexBytes, object.BigEndian, vertexCount, channels[1], streamOffsets, streamStrides)
		if err != nil {
			return nil, fmt.Errorf("decode Mesh normals: %w", err)
		}
		geometry.Normals = meshFloat3Values(values)
	}
	if len(channels) > 2 && channels[2].Dimension == 4 {
		values, err := decodeMeshFloatChannel(vertexBytes, object.BigEndian, vertexCount, channels[2], streamOffsets, streamStrides)
		if err != nil {
			return nil, fmt.Errorf("decode Mesh tangents: %w", err)
		}
		geometry.Tangents = meshFloat4Values(values)
	}
	if len(channels) > 3 && channels[3].Dimension == 4 {
		values, err := decodeMeshFloatChannel(vertexBytes, object.BigEndian, vertexCount, channels[3], streamOffsets, streamStrides)
		if err != nil {
			return nil, fmt.Errorf("decode Mesh colors: %w", err)
		}
		geometry.Colors = meshFloat4Values(values)
	}
	if len(channels) > 4 && channels[4].Dimension >= 2 {
		values, err := decodeMeshFloatChannel(vertexBytes, object.BigEndian, vertexCount, channels[4], streamOffsets, streamStrides)
		if err != nil {
			return nil, fmt.Errorf("decode Mesh texture coordinates: %w", err)
		}
		geometry.TexCoord0 = meshFloat2Values(values, channels[4].Dimension)
	}

	geometry.Primitives, err = decodeMeshPrimitives(root, object.BigEndian, vertexCount)
	if err != nil {
		return nil, err
	}
	if len(geometry.Primitives) == 0 {
		return nil, fmt.Errorf("Mesh has no non-empty SubMesh primitives")
	}
	return geometry, nil
}

// meshVertexStreams 根据 Unity 5 及更高版本的隐式流规则计算每个顶点流的起始偏移和步长
// meshVertexStreams calculates vertex-stream offsets and strides using the implicit Unity 5 and newer stream rules
func meshVertexStreams(vertexCount uint32, channels []meshVertexChannel, dataSize uint64) ([]uint64, []uint64, error) {
	var streamCount uint16
	for _, channel := range channels {
		if channel.Dimension != 0 && uint16(channel.Stream)+1 > streamCount {
			streamCount = uint16(channel.Stream) + 1
		}
	}
	if streamCount == 0 {
		return nil, nil, fmt.Errorf("Mesh has no active vertex channels")
	}
	strides := make([]uint64, streamCount)
	for channelIndex, channel := range channels {
		if channel.Dimension == 0 {
			continue
		}
		componentSize, ok := meshVertexFormatSize(channel.Format)
		if !ok {
			return nil, nil, fmt.Errorf("Mesh vertex channel %d uses unsupported format %d", channelIndex, channel.Format)
		}
		end := uint64(channel.Offset) + uint64(channel.Dimension)*componentSize
		if end > strides[channel.Stream] {
			strides[channel.Stream] = end
		}
	}
	offsets := make([]uint64, streamCount)
	var offset uint64
	for streamIndex := uint16(0); streamIndex < streamCount; streamIndex++ {
		stride := strides[streamIndex]
		if stride == 0 {
			return nil, nil, fmt.Errorf("Mesh vertex stream %d is empty", streamIndex)
		}
		offsets[streamIndex] = offset
		streamSize := uint64(vertexCount) * stride
		if uint64(vertexCount) != 0 && streamSize/uint64(vertexCount) != stride {
			return nil, nil, fmt.Errorf("Mesh vertex stream %d size overflows UInt64", streamIndex)
		}
		if offset > math.MaxUint64-streamSize {
			return nil, nil, fmt.Errorf("Mesh vertex stream offsets overflow UInt64")
		}
		offset += streamSize
		if offset > math.MaxUint64-15 {
			return nil, nil, fmt.Errorf("Mesh vertex stream alignment overflows UInt64")
		}
		offset = (offset + 15) &^ uint64(15)
	}
	if offset > dataSize {
		return nil, nil, fmt.Errorf("Mesh vertex streams require %d bytes but m_DataSize contains %d", offset, dataSize)
	}
	return offsets, strides, nil
}

// decodeMeshFloatChannel 将任意 Unity VertexAttributeFormat 通道转换为 float32 分量
// decodeMeshFloatChannel converts a channel in any Unity VertexAttributeFormat to float32 components
func decodeMeshFloatChannel(data []byte, bigEndian bool, vertexCount uint32, channel meshVertexChannel, streamOffsets []uint64, streamStrides []uint64) ([]float32, error) {
	if channel.Dimension == 0 || uint64(channel.Stream) >= uint64(len(streamOffsets)) || uint64(channel.Stream) >= uint64(len(streamStrides)) {
		return nil, fmt.Errorf("invalid inactive vertex channel")
	}
	componentSize, ok := meshVertexFormatSize(channel.Format)
	if !ok {
		return nil, fmt.Errorf("unsupported vertex format %d", channel.Format)
	}
	componentCount := uint64(vertexCount) * uint64(channel.Dimension)
	if componentCount > uint64(len(data)) {
		return nil, fmt.Errorf("vertex channel component count %d exceeds the %d-byte source range", componentCount, len(data))
	}
	values := make([]float32, componentCount)
	streamOffset := streamOffsets[channel.Stream]
	stride := streamStrides[channel.Stream]
	for vertexIndex := uint32(0); vertexIndex < vertexCount; vertexIndex++ {
		for componentIndex := uint8(0); componentIndex < channel.Dimension; componentIndex++ {
			offset := streamOffset + uint64(vertexIndex)*stride + uint64(channel.Offset) + uint64(componentIndex)*componentSize
			if offset > uint64(len(data)) || componentSize > uint64(len(data))-offset {
				return nil, fmt.Errorf("component range [%d,%d) exceeds %d vertex bytes", offset, offset+componentSize, len(data))
			}
			value, err := meshVertexFloat(data[offset:offset+componentSize], bigEndian, channel.Format)
			if err != nil {
				return nil, err
			}
			values[uint64(vertexIndex)*uint64(channel.Dimension)+uint64(componentIndex)] = value
		}
	}
	return values, nil
}

// meshVertexFloat 解码一个 Unity VertexAttributeFormat 分量
// meshVertexFloat decodes one Unity VertexAttributeFormat component
func meshVertexFloat(data []byte, bigEndian bool, format uint8) (float32, error) {
	var order binary.ByteOrder = binary.LittleEndian
	if bigEndian {
		order = binary.BigEndian
	}
	switch format {
	case 0:
		return math.Float32frombits(order.Uint32(data)), nil
	case 1:
		return meshFloat16(order.Uint16(data)), nil
	case 2:
		return float32(data[0]) / 255, nil
	case 3:
		value := float32(int8(data[0])) / 127
		return max(value, -1), nil
	case 4:
		return float32(order.Uint16(data)) / 65535, nil
	case 5:
		value := float32(int16(order.Uint16(data))) / 32767
		return max(value, -1), nil
	case 6:
		return float32(data[0]), nil
	case 7:
		return float32(int8(data[0])), nil
	case 8:
		return float32(order.Uint16(data)), nil
	case 9:
		return float32(int16(order.Uint16(data))), nil
	case 10:
		return float32(order.Uint32(data)), nil
	case 11:
		return float32(int32(order.Uint32(data))), nil
	default:
		return 0, fmt.Errorf("unsupported vertex format %d", format)
	}
}

// meshFloat16 将 IEEE 754 binary16 位模式转换为 float32
// meshFloat16 converts an IEEE 754 binary16 bit pattern to float32
func meshFloat16(bits uint16) float32 {
	sign := uint32(bits>>15) << 31
	exponent := uint32(bits>>10) & 0x1f
	fraction := uint32(bits & 0x03ff)
	if exponent == 0 {
		if fraction == 0 {
			return math.Float32frombits(sign)
		}
		shift := uint32(0)
		for fraction&0x0400 == 0 {
			fraction <<= 1
			shift++
		}
		fraction &= 0x03ff
		exponent = 127 - 15 - shift
	} else if exponent == 0x1f {
		exponent = 0xff
	} else {
		exponent += 127 - 15
	}
	return math.Float32frombits(sign | exponent<<23 | fraction<<13)
}

// meshVertexFormatSize 返回 Unity VertexAttributeFormat 分量的字节数
// meshVertexFormatSize returns the byte size of one Unity VertexAttributeFormat component
func meshVertexFormatSize(format uint8) (uint64, bool) {
	switch format {
	case 0, 10, 11:
		return 4, true
	case 1, 4, 5, 8, 9:
		return 2, true
	case 2, 3, 6, 7:
		return 1, true
	default:
		return 0, false
	}
}

// decodeMeshPrimitives 解码 IndexBuffer 并把 Unity 三角带和四边形展开为三角形
// decodeMeshPrimitives decodes the IndexBuffer and expands Unity triangle strips and quads to triangles
func decodeMeshPrimitives(root *TypeTreeValue, bigEndian bool, vertexCount uint32) ([]MeshPrimitive, error) {
	indexData, ok := root.Field("m_IndexBuffer").Bytes()
	if !ok || len(indexData) == 0 {
		return nil, fmt.Errorf("Mesh has no m_IndexBuffer")
	}
	indexFormatValue, ok := meshUnsignedField(root, "m_IndexFormat")
	if !ok || indexFormatValue > 1 {
		return nil, fmt.Errorf("Mesh has invalid m_IndexFormat %d", indexFormatValue)
	}
	indexSize := uint64(2)
	if indexFormatValue == 1 {
		indexSize = 4
	}
	subMeshes := root.Field("m_SubMeshes")
	if subMeshes == nil {
		return nil, fmt.Errorf("Mesh has no m_SubMeshes")
	}
	primitives := make([]MeshPrimitive, 0, len(subMeshes.Children))
	for subMeshIndex, subMesh := range subMeshes.Children {
		firstByte, firstOK := meshUnsignedField(subMesh, "firstByte", "m_FirstByte")
		indexCount, countOK := meshUnsignedField(subMesh, "indexCount", "m_IndexCount")
		topology, topologyOK := meshUnsignedField(subMesh, "topology", "m_Topology")
		baseVertex, _ := meshUnsignedField(subMesh, "baseVertex", "m_BaseVertex")
		if !firstOK || !countOK || !topologyOK || topology > 5 || firstByte%indexSize != 0 {
			return nil, fmt.Errorf("Mesh SubMesh %d has an invalid index layout", subMeshIndex)
		}
		byteCount := indexCount * indexSize
		if firstByte > uint64(len(indexData)) || byteCount > uint64(len(indexData))-firstByte {
			return nil, fmt.Errorf("Mesh SubMesh %d index range [%d,%d) exceeds %d bytes", subMeshIndex, firstByte, firstByte+byteCount, len(indexData))
		}
		indices := make([]uint32, indexCount)
		for index := uint64(0); index < indexCount; index++ {
			offset := firstByte + index*indexSize
			var value uint64
			if indexSize == 2 {
				if bigEndian {
					value = uint64(binary.BigEndian.Uint16(indexData[offset : offset+2]))
				} else {
					value = uint64(binary.LittleEndian.Uint16(indexData[offset : offset+2]))
				}
			} else if bigEndian {
				value = uint64(binary.BigEndian.Uint32(indexData[offset : offset+4]))
			} else {
				value = uint64(binary.LittleEndian.Uint32(indexData[offset : offset+4]))
			}
			value += baseVertex
			if value >= uint64(vertexCount) || value > math.MaxUint32 {
				return nil, fmt.Errorf("Mesh SubMesh %d index %d targets vertex %d outside %d vertices", subMeshIndex, index, value, vertexCount)
			}
			indices[index] = uint32(value)
		}
		mode, indices, err := normalizeMeshTopology(uint8(topology), indices)
		if err != nil {
			return nil, fmt.Errorf("Mesh SubMesh %d: %w", subMeshIndex, err)
		}
		if len(indices) != 0 {
			primitives = append(primitives, MeshPrimitive{Mode: mode, Indices: indices})
		}
	}
	return primitives, nil
}

// normalizeMeshTopology 将 Unity 特有拓扑转换为 glTF 核心拓扑
// normalizeMeshTopology converts Unity-specific topology to glTF core topology
func normalizeMeshTopology(topology uint8, indices []uint32) (MeshPrimitiveMode, []uint32, error) {
	switch topology {
	case 0:
		if len(indices)%3 != 0 {
			return 0, nil, fmt.Errorf("triangle index count %d is not divisible by three", len(indices))
		}
		return MeshPrimitiveModeTriangles, indices, nil
	case 1:
		if len(indices) < 3 {
			return MeshPrimitiveModeTriangles, nil, nil
		}
		triangles := make([]uint32, 0, (len(indices)-2)*3)
		for index := int64(0); index+2 < int64(len(indices)); index++ {
			a, b, c := indices[index], indices[index+1], indices[index+2]
			if a == b || b == c || a == c {
				continue
			}
			if index&1 != 0 {
				a, b = b, a
			}
			triangles = append(triangles, a, b, c)
		}
		return MeshPrimitiveModeTriangles, triangles, nil
	case 2:
		if len(indices)%4 != 0 {
			return 0, nil, fmt.Errorf("quad index count %d is not divisible by four", len(indices))
		}
		triangles := make([]uint32, 0, len(indices)/4*6)
		for index := int64(0); index < int64(len(indices)); index += 4 {
			triangles = append(triangles, indices[index], indices[index+1], indices[index+2], indices[index], indices[index+2], indices[index+3])
		}
		return MeshPrimitiveModeTriangles, triangles, nil
	case 3:
		if len(indices)%2 != 0 {
			return 0, nil, fmt.Errorf("line index count %d is not divisible by two", len(indices))
		}
		return MeshPrimitiveModeLines, indices, nil
	case 4:
		return MeshPrimitiveModeLineStrip, indices, nil
	case 5:
		return MeshPrimitiveModePoints, indices, nil
	default:
		return 0, nil, fmt.Errorf("unsupported topology %d", topology)
	}
}

// meshUnsignedField 从多个候选字段名读取非负整数
// meshUnsignedField reads a non-negative integer from multiple candidate field names
func meshUnsignedField(parent *TypeTreeValue, names ...string) (uint64, bool) {
	if parent == nil {
		return 0, false
	}
	for _, name := range names {
		field := parent.Field(name)
		if field == nil {
			continue
		}
		if value, ok := field.UInt64(); ok {
			return value, true
		}
		if value, ok := field.Int64(); ok && value >= 0 {
			return uint64(value), true
		}
	}
	return 0, false
}

// meshFloat2Values 将扁平分量数组转换为二维向量并忽略额外分量
// meshFloat2Values converts flat components to two-dimensional vectors and ignores extra components
func meshFloat2Values(values []float32, dimension uint8) [][2]float32 {
	if dimension < 2 || uint64(len(values))%uint64(dimension) != 0 {
		return nil
	}
	result := make([][2]float32, uint64(len(values))/uint64(dimension))
	for index := range result {
		base := uint64(index) * uint64(dimension)
		result[index] = [2]float32{values[base], values[base+1]}
	}
	return result
}

// meshFloat3Values 将扁平分量数组转换为三维向量
// meshFloat3Values converts flat components to three-dimensional vectors
func meshFloat3Values(values []float32) [][3]float32 {
	if len(values)%3 != 0 {
		return nil
	}
	result := make([][3]float32, len(values)/3)
	for index := range result {
		result[index] = [3]float32{values[index*3], values[index*3+1], values[index*3+2]}
	}
	return result
}

// meshFloat4Values 将扁平分量数组转换为四维向量
// meshFloat4Values converts flat components to four-dimensional vectors
func meshFloat4Values(values []float32) [][4]float32 {
	if len(values)%4 != 0 {
		return nil
	}
	result := make([][4]float32, len(values)/4)
	for index := range result {
		result[index] = [4]float32{values[index*4], values[index*4+1], values[index*4+2], values[index*4+3]}
	}
	return result
}
