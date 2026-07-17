package aba

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"math/bits"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
)

// PackedMesh 对应 AbaExtractor 的 CR_MOD_MESH MessagePack 布局 / PackedMesh matches AbaExtractor's MessagePack layout for CR_MOD_MESH
type PackedMesh struct {
	_struct     struct{}             `codec:",toarray"`     // 强制按数组编码 / Forces array encoding
	VertexCount int                  `json:"m_VertexCount"` // 顶点数量 / Vertex count
	Vertices    [][]float32          // 顶点位置数组，每项为 xyz / Vertex position array, each item is xyz
	Normals     [][]float32          // 法线数组，每项为 xyz / Normal array, each item is xyz
	Tangents    [][]float32          // 切线数组，每项为 xyzw / Tangent array, each item is xyzw
	UV          [][]float32          // UV0 数组，每项为 uv / UV0 array, each item is uv
	BindPose    [][]float32          // 绑定姿态矩阵数组，每项为 4x4 展平矩阵 / Bind-pose matrix array, each item is a flattened 4x4 matrix
	Skin        []PackedBoneWeights4 // 顶点蒙皮权重数组 / Vertex skinning weight array
	SubMeshes   [][]uint32           // 子网格索引数组 / Submesh index arrays
}

// PackedBoneWeights4 对应 AbaExtractor 的骨骼权重四元组布局 / PackedBoneWeights4 matches AbaExtractor's four-bone-weight layout
type PackedBoneWeights4 struct {
	_struct    struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	BoneIndex0 int      // 第 0 个骨骼索引 / Bone index 0
	BoneIndex1 int      // 第 1 个骨骼索引 / Bone index 1
	BoneIndex2 int      // 第 2 个骨骼索引 / Bone index 2
	BoneIndex3 int      // 第 3 个骨骼索引 / Bone index 3
	Weight0    float32  // 第 0 个骨骼权重 / Bone weight 0
	Weight1    float32  // 第 1 个骨骼权重 / Bone weight 1
	Weight2    float32  // 第 2 个骨骼权重 / Bone weight 2
	Weight3    float32  // 第 3 个骨骼权重 / Bone weight 3
}

// meshChannel 表示 Unity Mesh 顶点通道描述 / meshChannel represents one Unity Mesh vertex channel descriptor
type meshChannel struct {
	Stream    int  // 顶点流索引 / Vertex stream index
	Offset    int  // 通道在流内的字节偏移 / Byte offset inside the stream
	Format    byte // Unity 通道格式枚举 / Unity channel format enum
	Dimension int  // 通道维度 / Channel dimension
}

// TryConvertMeshToCRMesh 将 Unity Mesh 资源转换为 KCES CR_MOD_MESH 字节 / TryConvertMeshToCRMesh converts a Unity Mesh asset to KCES CR_MOD_MESH bytes
func (af *AssetsFile) TryConvertMeshToCRMesh(info *AssetInfo, log func(string)) ([]byte, error) {
	root, err := af.ReadAssetValue(info)
	if err != nil {
		return nil, err
	}

	meshCompression, _ := root.Field("m_MeshCompression").Int64()
	if meshCompression > 0 {
		return nil, fmt.Errorf("CompressedMesh (level=%d) is not supported", meshCompression)
	}

	vertexData := root.Field("m_VertexData")
	if vertexData == nil {
		return nil, fmt.Errorf("m_VertexData field missing")
	}

	vertCount, ok := vertexData.Field("m_VertexCount").Int64()
	if !ok || vertCount <= 0 {
		compressed := root.Field("m_CompressedMesh")
		extra := ""
		if compressed != nil && len(compressed.Children) > 0 {
			extra = " (m_CompressedMesh present)"
		}
		return nil, fmt.Errorf("invalid m_VertexCount=%d%s", vertCount, extra)
	}
	// AbaExtractor reads this value through AssetTypeValueField.AsInt and stores
	// it in PackedMesh.m_VertexCount (System.Int32). Keeping the bound explicit
	// also prevents a corrupt UInt32 value from overflowing Go allocation math.
	if vertCount > int64(1<<31-1) {
		return nil, fmt.Errorf("m_VertexCount=%d exceeds the Int32 range used by PackedMesh", vertCount)
	}
	vertexCount := int(vertCount)

	dataBytes, ok := vertexData.Field("m_DataSize").Bytes()
	if !ok || len(dataBytes) == 0 {
		return nil, fmt.Errorf("m_DataSize is empty")
	}

	channelsField := vertexData.Field("m_Channels")
	if channelsField == nil {
		return nil, fmt.Errorf("m_Channels field missing")
	}
	channels := collectMeshChannels(arrayItems(channelsField))
	if len(channels) == 0 {
		return nil, fmt.Errorf("m_Channels: no valid channel entries")
	}

	streamCount := 0
	for _, ch := range channels {
		if ch.Stream+1 > streamCount {
			streamCount = ch.Stream + 1
		}
	}
	streamOffset := make([]uint64, streamCount)
	streamStride := make([]uint64, streamCount)
	var cursor uint64
	for streamIdx := 0; streamIdx < streamCount; streamIdx++ {
		var stride uint64
		for _, ch := range channels {
			if ch.Stream == streamIdx && ch.Dimension > 0 {
				stride += uint64(ch.Dimension) * uint64(meshFormatSize(ch.Format))
			}
		}
		streamOffset[streamIdx] = cursor
		streamStride[streamIdx] = stride
		high, streamBytes := bits.Mul64(uint64(vertexCount), stride)
		streamEnd, carry := bits.Add64(cursor, streamBytes, 0)
		alignedEnd, alignCarry := bits.Add64(streamEnd, 15, 0)
		if high != 0 || carry != 0 || alignCarry != 0 {
			return nil, fmt.Errorf("vertex stream %d layout overflows UInt64", streamIdx)
		}
		cursor = alignedEnd &^ 15
	}

	for _, channelIndex := range []int{0, 1, 2, 4, 12, 13} {
		if err := validateMeshChannelBounds(channelIndex, channels, streamOffset, streamStride, vertexCount, len(dataBytes)); err != nil {
			return nil, err
		}
	}

	readFloats := func(chIdx int) []float32 {
		if chIdx < 0 || chIdx >= len(channels) || channels[chIdx].Dimension == 0 {
			return nil
		}
		ch := channels[chIdx]
		stride := streamStride[ch.Stream]
		offset := streamOffset[ch.Stream]
		elemSize := meshFormatSize(ch.Format)
		out := make([]float32, vertexCount*ch.Dimension)
		for i := 0; i < vertexCount; i++ {
			base := int(offset + uint64(i)*stride + uint64(ch.Offset))
			for j := 0; j < ch.Dimension; j++ {
				out[i*ch.Dimension+j] = meshReadFloat(dataBytes, base+j*int(elemSize), ch.Format)
			}
		}
		return out
	}

	readInts := func(chIdx int) []int {
		if chIdx < 0 || chIdx >= len(channels) || channels[chIdx].Dimension == 0 {
			return nil
		}
		ch := channels[chIdx]
		stride := streamStride[ch.Stream]
		offset := streamOffset[ch.Stream]
		elemSize := meshFormatSize(ch.Format)
		out := make([]int, vertexCount*ch.Dimension)
		for i := 0; i < vertexCount; i++ {
			base := int(offset + uint64(i)*stride + uint64(ch.Offset))
			for j := 0; j < ch.Dimension; j++ {
				out[i*ch.Dimension+j] = meshReadInt(dataBytes, base+j*int(elemSize), ch.Format)
			}
		}
		return out
	}

	pos := readFloats(0)
	if pos == nil {
		return nil, fmt.Errorf("position channel is missing")
	}
	normals := readFloats(1)
	tangents := readFloats(2)
	uv0 := readFloats(4)
	skinWeightDim := 0
	skinIndexDim := 0
	if len(channels) > 12 {
		skinWeightDim = channels[12].Dimension
	}
	if len(channels) > 13 {
		skinIndexDim = channels[13].Dimension
	}
	skinWeights := []float32(nil)
	skinIndices := []int(nil)
	if skinWeightDim > 0 {
		skinWeights = readFloats(12)
	}
	if skinIndexDim > 0 {
		skinIndices = readInts(13)
	}

	bindPose, bindDiag := readBindPose(root)
	subMeshes, subDiag := buildSubMeshes(root, log)
	skin := buildSkin(vertexCount, skinWeights, skinIndices, skinWeightDim, skinIndexDim)

	packed := &PackedMesh{
		VertexCount: vertexCount,
		Vertices:    toVec3List(pos),
		Normals:     toVec3ListOrEmpty(normals),
		Tangents:    toVec4ListOrEmpty(tangents),
		UV:          toVec2ListOrEmpty(uv0),
		BindPose:    bindPose,
		Skin:        skin,
		SubMeshes:   subMeshes,
	}

	if log != nil {
		name, _ := root.Field("m_Name").String()
		log(fmt.Sprintf("[crmesh] %s: verts=%d norm=%d tang=%d uv=%d %s %s submesh=%d(idx=%d)",
			name,
			packed.VertexCount,
			len(packed.Normals),
			len(packed.Tangents),
			len(packed.UV),
			bindDiag,
			skinDiag(skinWeightDim, skinIndexDim, skinWeights, skinIndices),
			len(packed.SubMeshes),
			sumSubMeshIndices(packed.SubMeshes),
		))
		if subDiag != "" {
			log("  [crmesh] " + subDiag)
		}
	}

	payload, err := ct.EncodeMsgpack(packed)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	bw := stream.NewBinaryWriter(&buf)
	if err := bw.WriteString("CR_MOD_MESH"); err != nil {
		return nil, err
	}
	if err := bw.WriteBytes(payload); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func collectMeshChannels(src []*TypeTreeValue) []meshChannel {
	var out []meshChannel
	for _, item := range src {
		if item == nil {
			continue
		}
		stream, ok1 := item.Field("stream").Int64()
		offset, ok2 := item.Field("offset").Int64()
		format, ok3 := item.Field("format").Int64()
		dim, ok4 := item.Field("dimension").Int64()
		if !ok1 || !ok2 || !ok3 || !ok4 {
			continue
		}
		out = append(out, meshChannel{
			// AbaExtractor explicitly casts all four source integers to byte.
			// Preserve the same unchecked low-byte behavior before applying the
			// dimension nibble mask.
			Stream:    int(byte(stream)),
			Offset:    int(byte(offset)),
			Format:    byte(format),
			Dimension: int(byte(dim) & 15),
		})
	}
	return out
}

func validateMeshChannelBounds(channelIndex int, channels []meshChannel, streamOffset, streamStride []uint64, vertexCount, dataLength int) error {
	if channelIndex < 0 || channelIndex >= len(channels) || channels[channelIndex].Dimension == 0 {
		return nil
	}
	channel := channels[channelIndex]
	if channel.Stream < 0 || channel.Stream >= len(streamOffset) || channel.Stream >= len(streamStride) {
		return fmt.Errorf("mesh channel %d references missing stream %d", channelIndex, channel.Stream)
	}
	if vertexCount <= 0 {
		return fmt.Errorf("mesh channel %d has invalid vertex count %d", channelIndex, vertexCount)
	}
	componentBytes := uint64(meshFormatSize(channel.Format))
	high, lastVertexOffset := bits.Mul64(uint64(vertexCount-1), streamStride[channel.Stream])
	if high != 0 {
		return fmt.Errorf("mesh channel %d layout overflows UInt64", channelIndex)
	}
	lastEnd, ok := checkedMeshOffset(
		streamOffset[channel.Stream],
		lastVertexOffset,
		uint64(channel.Offset),
		uint64(channel.Dimension)*componentBytes,
	)
	if !ok {
		return fmt.Errorf("mesh channel %d layout overflows UInt64", channelIndex)
	}
	if lastEnd > uint64(dataLength) {
		return fmt.Errorf("mesh channel %d requires vertex data through byte %d, only %d bytes are available", channelIndex, lastEnd, dataLength)
	}
	return nil
}

func checkedMeshOffset(parts ...uint64) (uint64, bool) {
	var total uint64
	for _, part := range parts {
		var carry uint64
		total, carry = bits.Add64(total, part, 0)
		if carry != 0 {
			return 0, false
		}
	}
	return total, true
}

func meshFormatSize(format byte) uint32 {
	switch format {
	case 0:
		return 4
	case 1:
		return 2
	case 2, 3:
		return 1
	case 4, 5:
		return 2
	case 6, 7:
		return 1
	case 8, 9:
		return 2
	case 10, 11:
		return 4
	default:
		return 4
	}
}

func meshReadFloat(data []byte, off int, format byte) float32 {
	if off < 0 || off >= len(data) {
		return 0
	}
	switch format {
	case 0:
		if off+4 > len(data) {
			return 0
		}
		return math.Float32frombits(binary.LittleEndian.Uint32(data[off:]))
	case 1:
		if off+2 > len(data) {
			return 0
		}
		return halfToFloat(binary.LittleEndian.Uint16(data[off:]))
	case 2:
		return float32(data[off]) / 255.0
	case 3:
		return maxFloat32(float32(int8(data[off]))/127.0, -1.0)
	case 4:
		if off+2 > len(data) {
			return 0
		}
		return float32(binary.LittleEndian.Uint16(data[off:])) / 65535.0
	case 5:
		if off+2 > len(data) {
			return 0
		}
		return maxFloat32(float32(int16(binary.LittleEndian.Uint16(data[off:])))/32767.0, -1.0)
	default:
		return 0
	}
}

func meshReadInt(data []byte, off int, format byte) int {
	if off < 0 || off >= len(data) {
		return 0
	}
	switch format {
	case 6:
		return int(data[off])
	case 7:
		return int(int8(data[off]))
	case 8:
		if off+2 > len(data) {
			return 0
		}
		return int(binary.LittleEndian.Uint16(data[off:]))
	case 9:
		if off+2 > len(data) {
			return 0
		}
		return int(int16(binary.LittleEndian.Uint16(data[off:])))
	case 10:
		if off+4 > len(data) {
			return 0
		}
		// C# performs an unchecked (int)UInt32 cast here.
		return int(int32(binary.LittleEndian.Uint32(data[off:])))
	case 11:
		if off+4 > len(data) {
			return 0
		}
		return int(int32(binary.LittleEndian.Uint32(data[off:])))
	default:
		return 0
	}
}

func halfToFloat(h uint16) float32 {
	sign := uint32(h>>15) << 31
	exp := uint32((h >> 10) & 0x1f)
	mant := uint32(h & 0x3ff)
	if exp == 0 {
		// Match AbaExtractor.HalfToFloat exactly. Its implementation copies a
		// half subnormal's mantissa into a Single subnormal instead of
		// normalizing it as a general IEEE-754 half converter would.
		return math.Float32frombits(sign | (mant << 13))
	}
	if exp == 31 {
		return math.Float32frombits(sign | 0x7f800000 | (mant << 13))
	}
	return math.Float32frombits(sign | ((exp + 112) << 23) | (mant << 13))
}

func readBindPose(root *TypeTreeValue) ([][]float32, string) {
	field := root.Field("m_BindPose")
	if field == nil {
		// AbaExtractor initializes a List<float[]> before probing the field, so
		// a missing bind-pose field serializes as an empty array rather than nil.
		return [][]float32{}, "bp=dummy"
	}
	items := arrayItems(field)
	out := make([][]float32, 0, len(items))
	diag := fmt.Sprintf("bp.Children=%d", len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		mat := readMatrix4x4(item)
		if mat == nil {
			diag += fmt.Sprintf(",skip(cnt=%d)", len(item.Children))
			continue
		}
		out = append(out, transposeMatrix4x4(mat))
	}
	return out, fmt.Sprintf("bp=%d[%s]", len(out), diag)
}

func readMatrix4x4(mat *TypeTreeValue) []float32 {
	if mat == nil {
		return nil
	}
	if len(mat.Children) == 16 {
		out := make([]float32, 16)
		for i, child := range mat.Children {
			if child == nil {
				continue
			}
			if f, ok := child.Float32(); ok {
				out[i] = f
				continue
			}
			if n, ok := child.Int64(); ok {
				out[i] = float32(n)
			}
		}
		return out
	}
	if len(mat.Children) == 4 {
		if mat.Children[0] != nil && len(mat.Children[0].Children) >= 4 {
			out := make([]float32, 0, 16)
			for _, row := range mat.Children {
				if row == nil {
					continue
				}
				for i := 0; i < 4 && i < len(row.Children); i++ {
					if row.Children[i] == nil {
						out = append(out, 0)
						continue
					}
					if f, ok := row.Children[i].Float32(); ok {
						out = append(out, f)
					} else if n, ok := row.Children[i].Int64(); ok {
						out = append(out, float32(n))
					} else {
						out = append(out, 0)
					}
				}
			}
			if len(out) == 16 {
				return out
			}
		}
	}
	return nil
}

func transposeMatrix4x4(m []float32) []float32 {
	if len(m) != 16 {
		return nil
	}
	return []float32{
		m[0], m[4], m[8], m[12],
		m[1], m[5], m[9], m[13],
		m[2], m[6], m[10], m[14],
		m[3], m[7], m[11], m[15],
	}
}

func buildSkin(vertCount int, skinW []float32, skinIdx []int, wDim, iDim int) []PackedBoneWeights4 {
	if len(skinIdx) == 0 || iDim == 0 {
		return []PackedBoneWeights4{}
	}
	out := make([]PackedBoneWeights4, 0, vertCount)
	for v := 0; v < vertCount; v++ {
		baseW := v * wDim
		baseI := v * iDim
		item := PackedBoneWeights4{}
		item.BoneIndex0 = meshSkinIndex(skinIdx, baseI, 0, iDim)
		item.BoneIndex1 = meshSkinIndex(skinIdx, baseI, 1, iDim)
		item.BoneIndex2 = meshSkinIndex(skinIdx, baseI, 2, iDim)
		item.BoneIndex3 = meshSkinIndex(skinIdx, baseI, 3, iDim)
		item.Weight0 = meshSkinWeight(skinW, baseW, 0, wDim)
		item.Weight1 = meshSkinWeight(skinW, baseW, 1, wDim)
		item.Weight2 = meshSkinWeight(skinW, baseW, 2, wDim)
		item.Weight3 = meshSkinWeight(skinW, baseW, 3, wDim)
		out = append(out, item)
	}
	return out
}

func meshSkinWeight(skinW []float32, base, d, wDim int) float32 {
	if len(skinW) == 0 || d >= wDim {
		return 0
	}
	idx := base + d
	if idx < 0 || idx >= len(skinW) {
		return 0
	}
	return skinW[idx]
}

func meshSkinIndex(skinIdx []int, base, d, iDim int) int {
	if d >= iDim {
		return 0
	}
	idx := base + d
	if idx < 0 || idx >= len(skinIdx) {
		return 0
	}
	return skinIdx[idx]
}

func buildSubMeshes(root *TypeTreeValue, log func(string)) ([][]uint32, string) {
	idxBytes, ok := root.Field("m_IndexBuffer").Bytes()
	if !ok || len(idxBytes) == 0 {
		if log != nil {
			log("  [crmesh] m_IndexBuffer empty/null")
		}
		return [][]uint32{}, ""
	}

	indexFormat, _ := root.Field("m_IndexFormat").Int64()
	is16 := indexFormat == 0
	indexSize := 4
	if is16 {
		indexSize = 2
	}

	subMeshesField := root.Field("m_SubMeshes")
	if subMeshesField == nil {
		if log != nil {
			log("  [crmesh] m_SubMeshes=dummy/null")
		}
		return [][]uint32{}, ""
	}

	items := arrayItems(subMeshesField)
	if len(items) == 0 {
		if log != nil {
			log("  [crmesh] m_SubMeshes Array node missing")
		}
		return [][]uint32{}, ""
	}

	out := make([][]uint32, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		firstField := item.Field("firstByte")
		if firstField == nil {
			firstField = item.Field("first")
		}
		if firstField == nil {
			continue
		}
		first, _ := firstField.Int64()
		indexCount, _ := item.Field("indexCount").Int64()
		topology, _ := item.Field("topology").Int64()
		if first < 0 || indexCount <= 0 || first >= int64(len(idxBytes)) {
			out = append(out, []uint32{})
			continue
		}
		availableCount := int64(len(idxBytes)-int(first)) / int64(indexSize)
		readCount := indexCount
		if readCount > availableCount {
			readCount = availableCount
		}
		capacity := readCount
		if topology == 1 && readCount >= 3 {
			capacity = (readCount - 2) * 3
		}
		idxs := make([]uint32, 0, int(capacity))
		if topology == 0 {
			for n := int64(0); n < readCount; n++ {
				off := int(first) + int(n)*indexSize
				if is16 {
					idxs = append(idxs, uint32(binary.LittleEndian.Uint16(idxBytes[off:])))
				} else {
					idxs = append(idxs, binary.LittleEndian.Uint32(idxBytes[off:]))
				}
			}
		} else if topology == 1 && readCount >= 3 {
			for n := int64(0); n < readCount-2; n++ {
				off := int(first) + int(n)*indexSize
				var a, b, c uint32
				if is16 {
					a = uint32(binary.LittleEndian.Uint16(idxBytes[off:]))
					b = uint32(binary.LittleEndian.Uint16(idxBytes[off+indexSize:]))
					c = uint32(binary.LittleEndian.Uint16(idxBytes[off+indexSize*2:]))
				} else {
					a = binary.LittleEndian.Uint32(idxBytes[off:])
					b = binary.LittleEndian.Uint32(idxBytes[off+indexSize:])
					c = binary.LittleEndian.Uint32(idxBytes[off+indexSize*2:])
				}
				if a != b && a != c && b != c {
					if n&1 == 1 {
						idxs = append(idxs, b, a)
					} else {
						idxs = append(idxs, a, b)
					}
					idxs = append(idxs, c)
				}
			}
		}
		out = append(out, idxs)
	}

	diag := fmt.Sprintf("submesh=%d(idx=%d)", len(out), sumSubMeshIndices(out))
	return out, diag
}

func sumSubMeshIndices(src [][]uint32) int {
	total := 0
	for _, s := range src {
		total += len(s)
	}
	return total
}

func skinDiag(wDim, iDim int, skinW []float32, skinIdx []int) string {
	return fmt.Sprintf("skin: wDim=%d, iDim=%d, skinW=%d, skinIdx=%d",
		wDim, iDim,
		len(skinW),
		len(skinIdx),
	)
}

func toVec3List(arr []float32) [][]float32 {
	if len(arr) == 0 {
		return [][]float32{}
	}
	out := make([][]float32, 0, len(arr)/3)
	for i := 0; i+2 < len(arr); i += 3 {
		out = append(out, []float32{arr[i], arr[i+1], arr[i+2]})
	}
	return out
}

func toVec3ListOrEmpty(arr []float32) [][]float32 {
	if len(arr) == 0 {
		return [][]float32{}
	}
	return toVec3List(arr)
}

func toVec4ListOrEmpty(arr []float32) [][]float32 {
	if len(arr) == 0 {
		return [][]float32{}
	}
	out := make([][]float32, 0, len(arr)/4)
	for i := 0; i+3 < len(arr); i += 4 {
		out = append(out, []float32{arr[i], arr[i+1], arr[i+2], arr[i+3]})
	}
	return out
}

func toVec2ListOrEmpty(arr []float32) [][]float32 {
	if len(arr) == 0 {
		return [][]float32{}
	}
	out := make([][]float32, 0, len(arr)/2)
	for i := 0; i+1 < len(arr); i += 2 {
		out = append(out, []float32{arr[i], arr[i+1]})
	}
	return out
}

func arrayItems(v *TypeTreeValue) []*TypeTreeValue {
	if v == nil {
		return nil
	}
	if len(v.Children) == 1 && v.Children[0] != nil && strings.EqualFold(v.Children[0].Name, "Array") {
		return v.Children[0].Children
	}
	return v.Children
}

func maxFloat32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
