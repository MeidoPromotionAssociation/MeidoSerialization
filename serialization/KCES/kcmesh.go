package KCES

import (
	"fmt"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/msgpack"
	"github.com/ugorji/go/codec"
)

const (
	// KCMeshExtension 是 KCMesh 文件扩展名
	// KCMeshExtension is the KCMesh file extension
	KCMeshExtension        = ".kcmesh"
	kcMeshFixVersion int32 = 10001
)

// KCMesh 对应 ExportKCES.KCMesh 的三十四槽未压缩 MessagePack 网格 / KCMesh matches the 34-slot uncompressed MessagePack mesh in ExportKCES.KCMesh
type KCMesh struct {
	_struct               struct{}    `codec:",toarray"`             // 强制按数组编码 / Forces array encoding
	Version               int32       `json:"version"`               // 网格导出版本，当前 FixVersion 为 10001 / Mesh export version whose current FixVersion is 10001
	IndexFormat           int32       `json:"indexFormat"`           // UnityEngine.Rendering.IndexFormat 枚举值 / UnityEngine.Rendering.IndexFormat enum value
	VertexBufferCount     int32       `json:"vertexBufferCount"`     // Unity Mesh 顶点缓冲数量 / Unity Mesh vertex-buffer count
	VertexBufferTargetStr *string     `json:"vertexBufferTargetStr"` // GraphicsBuffer.Target 顶点缓冲标志字符串 / GraphicsBuffer.Target vertex-buffer flag string
	IndexBufferTargetStr  *string     `json:"indexBufferTargetStr"`  // GraphicsBuffer.Target 索引缓冲标志字符串 / GraphicsBuffer.Target index-buffer flag string
	BlendShapeCount       int32       `json:"blendShapeCount"`       // BlendShape 数量 / Blend-shape count
	BlendShapeNames       []*string   `json:"blendShapeNames"`       // 可空 BlendShape 名称数组 / Array of nullable blend-shape names
	BlendShapeFrameCount  []int32     `json:"blendShapeFrameCount"`  // 每个 BlendShape 的帧数量 / Frame counts for each blend shape
	DeltaVertices         [][]Vector3 `json:"deltaVertices"`         // 各 BlendShape 帧的顶点位移 / Vertex deltas for each blend-shape frame
	DeltaNormals          [][]Vector3 `json:"deltaNormals"`          // 各 BlendShape 帧的法线位移 / Normal deltas for each blend-shape frame
	DeltaTangents         [][]Vector3 `json:"deltaTangents"`         // 各 BlendShape 帧的切线位移 / Tangent deltas for each blend-shape frame
	BlendShapeFrameWeight []float32   `json:"blendShapeFrameWeight"` // 各 BlendShape 帧的权重 / Weights for each blend-shape frame
	BindPoses             []Matrix4x4 `json:"bindposes"`             // 骨骼绑定姿势矩阵 / Bone bind-pose matrices
	IsReadable            bool        `json:"isReadable"`            // Unity Mesh 是否可读 / Whether the Unity Mesh is readable
	VertexCount           int32       `json:"vertexCount"`           // 顶点数量 / Vertex count
	SubMeshCount          int32       `json:"subMeshCount"`          // 子网格数量 / Sub-mesh count
	Vertices              []Vector3   `json:"vertices"`              // 顶点位置 / Vertex positions
	Normals               []Vector3   `json:"normals"`               // 顶点法线 / Vertex normals
	Tangents              []Vector4   `json:"tangents"`              // 顶点切线 / Vertex tangents
	UV                    []Vector2   `json:"uv"`                    // 第一组 UV / First UV set
	UV2                   []Vector2   `json:"uv2"`                   // 第二组 UV / Second UV set
	UV3                   []Vector2   `json:"uv3"`                   // 第三组 UV / Third UV set
	UV4                   []Vector2   `json:"uv4"`                   // 第四组 UV / Fourth UV set
	UV5                   []Vector2   `json:"uv5"`                   // 第五组 UV / Fifth UV set
	UV6                   []Vector2   `json:"uv6"`                   // 第六组 UV / Sixth UV set
	UV7                   []Vector2   `json:"uv7"`                   // 第七组 UV / Seventh UV set
	UV8                   []Vector2   `json:"uv8"`                   // 第八组 UV / Eighth UV set
	Colors                []Color     `json:"colors"`                // 浮点顶点颜色 / Floating-point vertex colors
	Colors32              []Color32   `json:"colors32"`              // 八位顶点颜色 / Eight-bit vertex colors
	Triangles             []int32     `json:"triangles"`             // 合并后的三角形索引 / Combined triangle indices
	TrianglesInSubmesh    [][]int32   `json:"trianglesInSubmesh"`    // 按子网格分组的三角形索引 / Triangle indices grouped by sub-mesh
	BoneWeightWeight      []float32   `json:"boneWeightWeight"`      // 展平 BoneWeight1 权重 / Flattened BoneWeight1 weights
	BoneWeightIndex       []int32     `json:"boneWeightIndex"`       // 展平 BoneWeight1 骨骼索引 / Flattened BoneWeight1 bone indices
	BonePerVertex         []byte      `json:"bonePerVertex"`         // 每个顶点使用的骨骼权重数量 / Number of bone weights used by each vertex
}

// Matrix4x4 对应 MessagePack.Unity.Matrix4x4Formatter 的列优先十六槽布局 / Matrix4x4 matches the column-major 16-slot layout of MessagePack.Unity.Matrix4x4Formatter
type Matrix4x4 struct {
	_struct struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	M00     float32  `json:"m00"`       // 第零列第零行 / Column zero row zero
	M10     float32  `json:"m10"`       // 第零列第一行 / Column zero row one
	M20     float32  `json:"m20"`       // 第零列第二行 / Column zero row two
	M30     float32  `json:"m30"`       // 第零列第三行 / Column zero row three
	M01     float32  `json:"m01"`       // 第一列第零行 / Column one row zero
	M11     float32  `json:"m11"`       // 第一列第一行 / Column one row one
	M21     float32  `json:"m21"`       // 第一列第二行 / Column one row two
	M31     float32  `json:"m31"`       // 第一列第三行 / Column one row three
	M02     float32  `json:"m02"`       // 第二列第零行 / Column two row zero
	M12     float32  `json:"m12"`       // 第二列第一行 / Column two row one
	M22     float32  `json:"m22"`       // 第二列第二行 / Column two row two
	M32     float32  `json:"m32"`       // 第二列第三行 / Column two row three
	M03     float32  `json:"m03"`       // 第三列第零行 / Column three row zero
	M13     float32  `json:"m13"`       // 第三列第一行 / Column three row one
	M23     float32  `json:"m23"`       // 第三列第二行 / Column three row two
	M33     float32  `json:"m33"`       // 第三列第三行 / Column three row three
}

// Color 对应 MessagePack.Unity.ColorFormatter 的四槽浮点颜色 / Color matches the four-slot floating-point color used by MessagePack.Unity.ColorFormatter
type Color struct {
	_struct struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	R       float32  `json:"r"`         // 红色分量 / Red channel
	G       float32  `json:"g"`         // 绿色分量 / Green channel
	B       float32  `json:"b"`         // 蓝色分量 / Blue channel
	A       float32  `json:"a"`         // 透明度分量 / Alpha channel
}

// Color32 对应 MessagePack.Unity.Color32Formatter 的四槽八位颜色 / Color32 matches the four-slot eight-bit color used by MessagePack.Unity.Color32Formatter
type Color32 struct {
	_struct struct{} `codec:",toarray"` // 强制按数组编码 / Forces array encoding
	R       uint8    `json:"r"`         // 红色分量 / Red channel
	G       uint8    `json:"g"`         // 绿色分量 / Green channel
	B       uint8    `json:"b"`         // 蓝色分量 / Blue channel
	A       uint8    `json:"a"`         // 透明度分量 / Alpha channel
}

// DecodeKCMesh 解码未压缩的 .kcmesh MessagePack 数据
// DecodeKCMesh decodes uncompressed .kcmesh MessagePack data
func DecodeKCMesh(data []byte) (*KCMesh, error) {
	var value *KCMesh
	if err := msgpack.DecodeMsgpack(data, &value); err != nil {
		return nil, fmt.Errorf("decode KCMesh msgpack: %w", err)
	}
	return value, nil
}

// EncodeKCMesh 编码未压缩的 .kcmesh MessagePack 数据并保留调用方版本
// EncodeKCMesh encodes uncompressed .kcmesh MessagePack data while preserving the caller version
func EncodeKCMesh(value *KCMesh) ([]byte, error) {
	return encodeUncompressedIndexedMsgpack(value, "KCMesh")
}

// NewKCMesh 创建当前 10001 版本的新 KCMesh
// NewKCMesh creates a new KCMesh using the current version 10001
func NewKCMesh() *KCMesh {
	return &KCMesh{Version: kcMeshFixVersion}
}

// CodecEncodeSelf 按共享 indexed-object 规则编码 KCMesh
// CodecEncodeSelf encodes KCMesh using the shared indexed-object rules
func (v KCMesh) CodecEncodeSelf(e *codec.Encoder) { msgpack.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 KCMesh
// CodecDecodeSelf decodes KCMesh using the shared indexed-object rules
func (v *KCMesh) CodecDecodeSelf(d *codec.Decoder) { msgpack.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 Matrix4x4
// CodecEncodeSelf encodes Matrix4x4 using the shared indexed-object rules
func (v Matrix4x4) CodecEncodeSelf(e *codec.Encoder) { msgpack.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 Matrix4x4
// CodecDecodeSelf decodes Matrix4x4 using the shared indexed-object rules
func (v *Matrix4x4) CodecDecodeSelf(d *codec.Decoder) { msgpack.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 Color
// CodecEncodeSelf encodes Color using the shared indexed-object rules
func (v Color) CodecEncodeSelf(e *codec.Encoder) { msgpack.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 Color
// CodecDecodeSelf decodes Color using the shared indexed-object rules
func (v *Color) CodecDecodeSelf(d *codec.Decoder) { msgpack.DecodeIndexedObjectSelf(d, v) }

// CodecEncodeSelf 按共享 indexed-object 规则编码 Color32
// CodecEncodeSelf encodes Color32 using the shared indexed-object rules
func (v Color32) CodecEncodeSelf(e *codec.Encoder) { msgpack.EncodeIndexedObjectSelf(e, &v) }

// CodecDecodeSelf 按共享 indexed-object 规则解码 Color32
// CodecDecodeSelf decodes Color32 using the shared indexed-object rules
func (v *Color32) CodecDecodeSelf(d *codec.Decoder) { msgpack.DecodeIndexedObjectSelf(d, v) }
