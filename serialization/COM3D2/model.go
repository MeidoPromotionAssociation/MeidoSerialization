package COM3D2

import (
	"fmt"
	"io"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/utilities"
)

// CM3D2_MESH
// 模型文件
//
// CM3D2 支持 1000 - 2000 版本
// COM3D2 支持 1000 到 2001 版本，2100 版本的额外数据追加在文件末尾，不影响解析，应当可以正常读取，但无实际功能
// COM3D2_5 支持 1000 到 2200 以下版本
//
// 1000 - 2000 版本
// 基础版本
// 支持基本的骨骼、网格、UV、法线、切线数据
// 支持材质和基本的形态数据
//
// 2001 版本
// 新增 localScale 支持
//
// 2100 版本
// 新增 SkinThickness 支持
//
// 版本 2101
// 新增更多 UV 通道支持 (UV2, UV3, UV4)
// 新增多个未知标志位读取
//
// 版本 2102
// 新增 Morph Tangents 支持
//
// 版本 2104 及以上但低于 2200 版本
// 新增 ShadowCastingMode 支持
//
// 版本 2100 及以上但低于 2200 版本
// 验证文件名，必须以 crc_ 或 crx_ 或 gp03_ 开头
// 对于这些特殊前缀的文件，跳过了 "Bip01" 骨骼的无权重移除
//
// 版本 2200
// 未知
// CM3D2_MESH
// Model file
//
// CM3D2 supports versions 1000 through 2000
// COM3D2 supports versions 1000 through 2001; the extra version 2100 data is appended at the end of the file and does not affect parsing, so it can be read but has no practical function there
// COM3D2.5 supports versions from 1000 up to but not including 2200
//
// Versions 1000 through 2000
// Base versions
// Support basic bones, meshes, UVs, normals, and tangents
// Support materials and basic morph data
//
// Version 2001
// Adds localScale support
//
// Version 2100
// Adds SkinThickness support
//
// Version 2101
// Adds more UV channels (UV2, UV3, UV4)
// Adds several unknown channel flags
//
// Version 2102
// Adds morph tangent support
//
// Versions 2104 and later but below 2200
// Add ShadowCastingMode support
//
// Versions 2100 and later but below 2200
// Validate that filenames begin with crc_, crx_, or gp03_
// Files with these special prefixes skip removal of the unweighted "Bip01" bone
//
// Version 2200
// Unknown

// Model 对应 .model 文件
// 也称作 SkinMesh 皮肤网格
// Model corresponds to a .model file
// It is also known as a SkinMesh skinned mesh
type Model struct {
	Signature         string         `json:"Signature"`                   // "CM3D2_MESH" 文件签名 / "CM3D2_MESH" file signature
	Version           int32          `json:"Version"`                     // 模型格式版本 / Model format version
	Name              string         `json:"Name"`                        // 模型名称 / Model name
	RootBoneName      string         `json:"RootBoneName"`                // 根骨骼名称 / Root bone name
	ShadowCastingMode *string        `json:"ShadowCastingMode,omitempty"` // Unity ShadowCastingMode 的字符串表示，仅版本 2104 及以上且低于 2200 时存在 / String representation of Unity ShadowCastingMode, present only from version 2104 through versions below 2200
	Bones             []*Bone        `json:"Bones"`                       // 完整骨骼层级数据 / Complete bone hierarchy data
	VertCount         int32          `json:"VertCount"`                   // 线格式顶点数量，写出时由 Vertices 推导 / Wire vertex count, derived from Vertices when writing
	SubMeshCount      int32          `json:"SubMeshCount"`                // 线格式子网格数量，写出时由 SubMeshes 推导 / Wire submesh count, derived from SubMeshes when writing
	BoneCount         int32          `json:"BoneCount"`                   // 网格引用的骨骼数量，写出时由 BoneNames 推导 / Number of bones referenced by the mesh, derived from BoneNames when writing
	BoneNames         []string       `json:"BoneNames"`                   // 网格引用的骨骼名称列表 / Names of bones referenced by the mesh
	BindPoses         []Matrix4x4    `json:"BindPoses"`                   // 与 BoneNames 对应的绑定姿势矩阵 / Bind-pose matrices corresponding to BoneNames
	Vertices          []Vertex       `json:"Vertices"`                    // 顶点数据 / Vertex data
	Tangents          []Quaternion   `json:"Tangents,omitempty"`          // Unity Vector4 顺序的顶点切线数据 / Vertex tangent data in Unity Vector4 order
	BoneWeights       []BoneWeight   `json:"BoneWeights"`                 // 每个顶点的骨骼权重数据 / Bone-weight data for each vertex
	SubMeshes         [][]int32      `json:"SubMeshes"`                   // 各子网格的 UInt16 顶点索引列表 / UInt16 vertex-index lists for each submesh
	Materials         []*Material    `json:"Materials"`                   // 材质数据 / Material data
	MorphData         []*MorphData   `json:"MorphData,omitempty"`         // 以 morph 记录保存的形态数据 / Morph data stored in morph records
	SkinThickness     *SkinThickness `json:"SkinThickness,omitempty"`     // 版本 2100 及以上在 end 标记后保存的皮肤厚度数据 / Skin-thickness data stored after the end marker from version 2100 onward
}

// Bone 表示骨骼数据
// Bone represents bone data
type Bone struct {
	Name        string     `json:"Name"`            // 骨骼名称 / Bone name
	HasScale    bool       `json:"HasScale"`        // 是否创建名称带 _SCL_ 的缩放辅助节点 / Whether to create a scaling helper node whose name contains _SCL_
	ParentIndex int32      `json:"ParentIndex"`     // 父骨骼索引，负值表示挂在模型根节点 / Parent bone index, with a negative value attaching the bone to the model root
	Position    Vector3    `json:"Position"`        // 骨骼局部位置 / Bone local position
	Rotation    Quaternion `json:"Rotation"`        // 骨骼局部旋转 / Bone local rotation
	Scale       *Vector3   `json:"Scale,omitempty"` // 版本 2001 及以上可选的骨骼局部缩放 / Optional bone local scale for version 2001 and later
}

// Vertex 表示顶点数据
// Vertex represents vertex data
type Vertex struct {
	Position Vector3  `json:"Position"`           // 顶点位置 / Vertex position
	Normal   Vector3  `json:"Normal"`             // 顶点法线 / Vertex normal
	UV       Vector2  `json:"UV"`                 // 基础顶点 UV 坐标 / Base vertex UV coordinates
	UV2      *Vector2 `json:"UV2,omitempty"`      // 版本 2101 及以上由标志控制的顶点 UV2 坐标 / Vertex UV2 coordinates controlled by a flag from version 2101 onward
	UV3      *Vector2 `json:"UV3,omitempty"`      // 版本 2101 及以上由标志控制的顶点 UV3 坐标 / Vertex UV3 coordinates controlled by a flag from version 2101 onward
	UV4      *Vector2 `json:"UV4,omitempty"`      // 版本 2101 及以上由标志控制的顶点 UV4 坐标 / Vertex UV4 coordinates controlled by a flag from version 2101 onward
	Unknown1 *Vector2 `json:"Unknown1,omitempty"` // 版本 2101 及以上的未知可选 Vector2 通道 1，游戏读取后未使用 / Unknown optional Vector2 channel 1 from version 2101 onward, read but unused by the game
	Unknown2 *Vector2 `json:"Unknown2,omitempty"` // 版本 2101 及以上的未知可选 Vector2 通道 2，游戏读取后未使用 / Unknown optional Vector2 channel 2 from version 2101 onward, read but unused by the game
	Unknown3 *Vector2 `json:"Unknown3,omitempty"` // 版本 2101 及以上的未知可选 Vector2 通道 3，游戏读取后未使用 / Unknown optional Vector2 channel 3 from version 2101 onward, read but unused by the game
	Unknown4 *Vector2 `json:"Unknown4,omitempty"` // 版本 2101 及以上的未知可选 Vector2 通道 4，游戏读取后未使用 / Unknown optional Vector2 channel 4 from version 2101 onward, read but unused by the game
}

// BoneWeight 表示骨骼权重
// BoneWeight represents bone weights
type BoneWeight struct {
	BoneIndex0 uint16  `json:"BoneIndex0"` // 第一个骨骼索引 / First bone index
	BoneIndex1 uint16  `json:"BoneIndex1"` // 第二个骨骼索引 / Second bone index
	BoneIndex2 uint16  `json:"BoneIndex2"` // 第三个骨骼索引 / Third bone index
	BoneIndex3 uint16  `json:"BoneIndex3"` // 第四个骨骼索引 / Fourth bone index
	Weight0    float32 `json:"Weight0"`    // 第一个骨骼权重 / First bone weight
	Weight1    float32 `json:"Weight1"`    // 第二个骨骼权重 / Second bone weight
	Weight2    float32 `json:"Weight2"`    // 第三个骨骼权重 / Third bone weight
	Weight3    float32 `json:"Weight3"`    // 第四个骨骼权重 / Fourth bone weight
}

// MorphData 表示形态数据
// MorphData represents morph data
type MorphData struct {
	Name     string       `json:"Name"`               // 形态名称 / Morph name
	Indices  []int        `json:"Indices"`            // 受此形态影响的 UInt16 顶点索引 / UInt16 vertex indices affected by this morph
	Vertex   []Vector3    `json:"Vertex"`             // 顶点位置位移 / Vertex-position deltas
	Normals  []Vector3    `json:"Normals"`            // 顶点法线位移 / Vertex-normal deltas
	Tangents []Quaternion `json:"Tangents,omitempty"` // 版本 2102 及以上可选的切线数据 / Optional tangent data for version 2102 and later
}

// SkinThickness 表示皮肤厚度数据
// SkinThickness represents skin-thickness data
type SkinThickness struct {
	Signature  string                 `json:"Signature"`            // "SkinThickness" 文件签名 / "SkinThickness" file signature
	Version    int32                  `json:"Version"`              // 游戏当前写入 100，读取后不作版本分支 / Currently written as 100 by the game and read without version branches
	Use        bool                   `json:"Use"`                  // 是否使用皮肤厚度 / Whether skin thickness is enabled
	Groups     map[string]*ThickGroup `json:"Groups"`               // 按外层键索引的皮肤厚度组 / Skin-thickness groups indexed by their outer keys
	GroupOrder []string               `json:"GroupOrder,omitempty"` // 线格式中的组顺序 / Group order on the wire
}

// ThickGroup 表示皮肤厚度组
// ThickGroup represents a skin-thickness group
type ThickGroup struct {
	GroupName       string        `json:"GroupName"`       // 组名称 / Group name
	StartBoneName   string        `json:"StartBoneName"`   // 线段起始骨骼名称 / Segment start-bone name
	EndBoneName     string        `json:"EndBoneName"`     // 线段结束骨骼名称 / Segment end-bone name
	StepAngleDegree int32         `json:"StepAngleDegree"` // 角度步长 / Angle step in degrees
	Points          []*ThickPoint `json:"Points"`          // 皮肤厚度采样点 / Skin-thickness sample points
}

// ThickPoint 表示皮肤厚度点
// ThickPoint represents a skin-thickness point
type ThickPoint struct {
	TargetBoneName         string              `json:"TargetBoneName"`         // 目标骨骼名称 / Target bone name
	RatioSegmentStartToEnd float32             `json:"RatioSegmentStartToEnd"` // 点在线段起点到终点之间的比例 / Point ratio along the segment from start to end
	DistanceParAngle       []*ThickDefPerAngle `json:"DistanceParAngle"`       // 按角度保存的距离定义 / Distance definitions stored per angle
}

// ModelMetadata 表示模型的元数据
// 不包含模型的 3D 信息，只包含模型的文本信息
// 例如模型名称、根骨骼名称、材质名称等
// 用于编辑一些模型的文本属性
// 修改后需要与原模型文件合并
// ModelMetadata represents model metadata
// It excludes the model's 3D information and contains only textual information
// Examples include the model name, root bone name, and material names
// It is used to edit textual model properties
// After modification it must be merged with the original model file
type ModelMetadata struct {
	Signature         string      `json:"Signature"`                   // "CM3D2_MESH" 文件签名 / "CM3D2_MESH" file signature
	Version           int32       `json:"Version"`                     // 模型格式版本 / Model format version
	Name              string      `json:"Name"`                        // 模型名称 / Model name
	RootBoneName      string      `json:"RootBoneName"`                // 根骨骼名称 / Root bone name
	ShadowCastingMode *string     `json:"ShadowCastingMode,omitempty"` // Unity ShadowCastingMode 的字符串表示，仅版本 2104 及以上且低于 2200 时存在 / String representation of Unity ShadowCastingMode, present only from version 2104 through versions below 2200
	Materials         []*Material `json:"Materials"`                   // 材质数据 / Material data
}

// ThickDefPerAngle 表示每个角度的皮肤厚度定义
// ThickDefPerAngle represents a skin-thickness definition for one angle
type ThickDefPerAngle struct {
	AngleDegree     int32   `json:"AngleDegree"`     // 角度 / Angle in degrees
	VertexIndex     int32   `json:"VertexIndex"`     // 顶点索引 / Vertex index
	DefaultDistance float32 `json:"DefaultDistance"` // 默认距离 / Default distance
}

// 阴影投射方式，对应 Unity 的 ShadowCastingMode
// Shadow-casting modes corresponding to Unity ShadowCastingMode
const (
	// 不投射阴影
	// Do not cast shadows
	ShadowCastingModeOff = "Off"
	// 投射阴影
	// Cast shadows
	ShadowCastingModeOn = "On"
	// 双面投射阴影
	// Cast two-sided shadows
	ShadowCastingModeTwoSided = "TwoSided"
	// 只投射阴影
	// Cast shadows only
	ShadowCastingModeShadowsOnly = "ShadowsOnly"
)

// ReadModel 从 r 中读取皮肤网格数据
// ReadModel reads skinned-mesh data from r
func ReadModel(r io.Reader) (*Model, error) {
	rp, ok := r.(stream.Peeker)
	if !ok {
		return nil, fmt.Errorf("ReadModel: the reader is not peekable, wrap it with bufio.Reader first")
	}

	model := &Model{}

	reader := stream.NewBinaryReader(rp)

	// 读取文件头
	// Read the file header
	var err error
	// 读取签名
	// Read the signature
	model.Signature, err = reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("failed to read signature: %w", err)
	}
	// if model.Signature != "CM3D2_MESH" {
	// 	return nil, fmt.Errorf("invalid .model signature: got %q, want %s", sig, MateSignature)
	// }

	// 读取版本号
	// Read the version
	model.Version, err = reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("failed to read version: %w", err)
	}

	// 读取模型名称
	// Read the model name
	model.Name, err = reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("failed to read name: %w", err)
	}

	// 读取根骨骼名称
	// Read the root bone name
	model.RootBoneName, err = reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("failed to read root bone name: %w", err)
	}

	// 读取骨骼数量
	// Read the bone count
	boneCount, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("failed to read bone count: %w", err)
	}
	if err := validateNonNegativeCount("model bone count", boneCount); err != nil {
		return nil, err
	}

	// 读取阴影投射方式
	// Read the shadow-casting mode
	if model.Version >= 2104 && model.Version < 2200 {
		shadowCastingMode, err := reader.ReadString()
		if err != nil {
			return nil, fmt.Errorf("failed to read shadow casting mode: %w", err)
		}
		model.ShadowCastingMode = &shadowCastingMode
	}

	model.Bones = makeCountedSliceForAppend[*Bone](boneCount)
	for i := int32(0); i < boneCount; i++ {
		bone := &Bone{}

		bone.Name, err = reader.ReadString()
		if err != nil {
			return nil, fmt.Errorf("failed to read bone name: %w", err)
		}

		hasScale, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read bone scaling flags: %w", err)
		}
		bone.HasScale = hasScale != 0

		model.Bones = append(model.Bones, bone)
	}

	// 读取骨骼父子关系
	// Read the bone hierarchy
	for i := int32(0); i < boneCount; i++ {
		parentIndex, err := reader.ReadInt32()
		if err != nil {
			return nil, fmt.Errorf("failed to read bone parent index: %w", err)
		}
		model.Bones[i].ParentIndex = parentIndex
	}

	// 读取骨骼变换信息
	// Read bone transforms
	for i := int32(0); i < boneCount; i++ {
		bone := model.Bones[i]

		// 位置
		// Position
		x, err := reader.ReadFloat32()
		if err != nil {
			return nil, fmt.Errorf("failed to read bone position X: %w", err)
		}
		y, err := reader.ReadFloat32()
		if err != nil {
			return nil, fmt.Errorf("failed to read bone position Y: %w", err)
		}
		z, err := reader.ReadFloat32()
		if err != nil {
			return nil, fmt.Errorf("failed to read bone position Z: %w", err)
		}
		bone.Position = Vector3{X: x, Y: y, Z: z}

		// 旋转
		// Rotation
		x, err = reader.ReadFloat32()
		if err != nil {
			return nil, fmt.Errorf("failed to read bone rotation X: %w", err)
		}
		y, err = reader.ReadFloat32()
		if err != nil {
			return nil, fmt.Errorf("failed to read bone rotation Y: %w", err)
		}
		z, err = reader.ReadFloat32()
		if err != nil {
			return nil, fmt.Errorf("failed to read bone rotation Z: %w", err)
		}
		w, err := reader.ReadFloat32()
		if err != nil {
			return nil, fmt.Errorf("failed to read bone rotation W: %w", err)
		}
		bone.Rotation = Quaternion{X: x, Y: y, Z: z, W: w}

		// 如果版本大于等于2001且有缩放
		// Read an optional scale when the version is at least 2001
		if model.Version >= 2001 {
			hasScale, err := reader.ReadBool()
			if err != nil {
				return nil, fmt.Errorf("failed to read bone scaling flags: %w", err)
			}

			if hasScale {
				// 读取缩放 X
				// Read scale X
				x, err := reader.ReadFloat32()
				if err != nil {
					return nil, fmt.Errorf("failed to read bone scale X: %w", err)
				}
				y, err := reader.ReadFloat32()
				if err != nil {
					return nil, fmt.Errorf("failed to read bone scale Y: %w", err)
				}
				z, err := reader.ReadFloat32()
				if err != nil {
					return nil, fmt.Errorf("failed to read bone scale Z: %w", err)
				}
				bone.Scale = &Vector3{X: x, Y: y, Z: z}
			}
		}
	}

	// 读取网格基本信息
	// Read basic mesh information
	model.VertCount, err = reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("failed to read the number of vertices: %w", err)
	}
	if err := validateNonNegativeCount("model vertex count", model.VertCount); err != nil {
		return nil, err
	}

	model.SubMeshCount, err = reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("failed to read the number of subgrids: %w", err)
	}
	if err := validateNonNegativeCount("model submesh count", model.SubMeshCount); err != nil {
		return nil, err
	}

	model.BoneCount, err = reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("failed to read the number of bones: %w", err)
	}
	if err := validateNonNegativeCount("model mesh bone count", model.BoneCount); err != nil {
		return nil, err
	}

	// 读取骨骼名称
	// Read the mesh bone names
	boneNames := makeCountedSliceForAppend[string](model.BoneCount)
	for i := int32(0); i < model.BoneCount; i++ {
		boneName, err := reader.ReadString()
		if err != nil {
			return nil, fmt.Errorf("failed to read bone name (at bone index): %w", err)
		}
		boneNames = append(boneNames, boneName)
	}
	model.BoneNames = boneNames

	// 读取骨骼绑定姿势
	// Read bone bind poses
	bindPoses := makeCountedSliceForAppend[Matrix4x4](model.BoneCount)
	for i := int32(0); i < model.BoneCount; i++ {
		matrix, err := reader.ReadFloat4x4()
		if err != nil {
			return nil, fmt.Errorf("failed to read the armature binding pose: %w", err)
		}
		bindPoses = append(bindPoses, matrix)
	}
	model.BindPoses = bindPoses

	// 如果版本为 2101 或更高，读取额外标志位
	// Read the additional channel flags when the version is 2101 or later
	hasUV2 := false
	hasUV3 := false
	hasUV4 := false
	hasUnknownFlag1 := false
	hasUnknownFlag2 := false
	hasUnknownFlag3 := false
	hasUnknownFlag4 := false

	if model.Version >= 2101 {
		hasUV2, err = reader.ReadBool()
		if err != nil {
			return nil, fmt.Errorf("failed to read UV2 flag: %w", err)
		}

		hasUV3, err = reader.ReadBool()
		if err != nil {
			return nil, fmt.Errorf("failed to read UV3 flag: %w", err)
		}

		hasUV4, err = reader.ReadBool()
		if err != nil {
			return nil, fmt.Errorf("failed to read UV4 flag: %w", err)
		}

		hasUnknownFlag1, err = reader.ReadBool()
		if err != nil {
			return nil, fmt.Errorf("failed to read unknown flag 1: %w", err)
		}

		hasUnknownFlag2, err = reader.ReadBool()
		if err != nil {
			return nil, fmt.Errorf("failed to read unknown flag 2: %w", err)
		}

		hasUnknownFlag3, err = reader.ReadBool()
		if err != nil {
			return nil, fmt.Errorf("failed to read unknown flag 3: %w", err)
		}

		hasUnknownFlag4, err = reader.ReadBool()
		if err != nil {
			return nil, fmt.Errorf("failed to read unknown flag 4: %w", err)
		}
	}

	// 读取顶点数据
	// Read vertex data
	model.Vertices = makeCountedSliceForAppend[Vertex](model.VertCount)
	for i := int32(0); i < model.VertCount; i++ {
		var vertex Vertex
		// 顶点位置
		// Vertex position
		x, err := reader.ReadFloat32()
		if err != nil {
			return nil, fmt.Errorf("failed to read vertex position X: %w", err)
		}
		y, err := reader.ReadFloat32()
		if err != nil {
			return nil, fmt.Errorf("failed to read vertex position Y: %w", err)
		}
		z, err := reader.ReadFloat32()
		if err != nil {
			return nil, fmt.Errorf("failed to read vertex position Z: %w", err)
		}
		vertex.Position = Vector3{X: x, Y: y, Z: z}

		// 法线
		// Normal
		x, err = reader.ReadFloat32()
		if err != nil {
			return nil, fmt.Errorf("failed to read vertex normal X: %w", err)
		}
		y, err = reader.ReadFloat32()
		if err != nil {
			return nil, fmt.Errorf("failed to read vertex normal Y: %w", err)
		}
		z, err = reader.ReadFloat32()
		if err != nil {
			return nil, fmt.Errorf("failed to read vertex normal Z: %w", err)
		}
		vertex.Normal = Vector3{X: x, Y: y, Z: z}

		// UV 坐标
		// UV coordinates
		uvX, err := reader.ReadFloat32()
		if err != nil {
			return nil, fmt.Errorf("failed to read vertex UV coordinate X: %w", err)
		}
		uvY, err := reader.ReadFloat32()
		if err != nil {
			return nil, fmt.Errorf("failed to read vertex UV coordinate Y: %w", err)
		}
		vertex.UV = Vector2{X: uvX, Y: uvY}

		if hasUV2 {
			uv2X, err := reader.ReadFloat32()
			if err != nil {
				return nil, fmt.Errorf("failed to read vertex UV2 coordinate X: %w", err)
			}
			uv2Y, err := reader.ReadFloat32()
			if err != nil {
				return nil, fmt.Errorf("failed to read vertex UV2 coordinate Y: %w", err)
			}
			vertex.UV2 = &Vector2{X: uv2X, Y: uv2Y}
		}

		if hasUV3 {
			uv3X, err := reader.ReadFloat32()
			if err != nil {
				return nil, fmt.Errorf("failed to read vertex UV3 coordinate X: %w", err)
			}
			uv3Y, err := reader.ReadFloat32()
			if err != nil {
				return nil, fmt.Errorf("failed to read vertex UV3 coordinate Y: %w", err)
			}
			vertex.UV3 = &Vector2{X: uv3X, Y: uv3Y}
		}

		if hasUV4 {
			uv4X, err := reader.ReadFloat32()
			if err != nil {
				return nil, fmt.Errorf("failed to read vertex UV4 coordinate X: %w", err)
			}
			uv4Y, err := reader.ReadFloat32()
			if err != nil {
				return nil, fmt.Errorf("failed to read vertex UV4 coordinate Y: %w", err)
			}
			vertex.UV4 = &Vector2{X: uv4X, Y: uv4Y}
		}

		// 读取未知标志位对应的数据
		// Read data controlled by the unknown flags
		if hasUnknownFlag1 {
			unknownX1, err := reader.ReadFloat32()
			if err != nil {
				return nil, fmt.Errorf("failed to read unknown flag 1 data X: %w", err)
			}
			unknownY1, err := reader.ReadFloat32()
			if err != nil {
				return nil, fmt.Errorf("failed to read unknown flag 1 data Y: %w", err)
			}
			vertex.Unknown1 = &Vector2{X: unknownX1, Y: unknownY1}
		}

		if hasUnknownFlag2 {
			unknownX2, err := reader.ReadFloat32()
			if err != nil {
				return nil, fmt.Errorf("failed to read unknown flag 2 data X: %w", err)
			}
			unknownY2, err := reader.ReadFloat32()
			if err != nil {
				return nil, fmt.Errorf("failed to read unknown flag 2 data Y: %w", err)
			}
			vertex.Unknown2 = &Vector2{X: unknownX2, Y: unknownY2}
		}

		if hasUnknownFlag3 {
			unknownX3, err := reader.ReadFloat32()
			if err != nil {
				return nil, fmt.Errorf("failed to read unknown flag 3 data X: %w", err)
			}
			unknownY3, err := reader.ReadFloat32()
			if err != nil {
				return nil, fmt.Errorf("failed to read unknown flag 3 data Y: %w", err)
			}
			vertex.Unknown3 = &Vector2{X: unknownX3, Y: unknownY3}
		}

		if hasUnknownFlag4 {
			unknownX4, err := reader.ReadFloat32()
			if err != nil {
				return nil, fmt.Errorf("failed to read unknown flag 4 data X: %w", err)
			}
			unknownY4, err := reader.ReadFloat32()
			if err != nil {
				return nil, fmt.Errorf("failed to read unknown flag 4 data Y: %w", err)
			}
			vertex.Unknown4 = &Vector2{X: unknownX4, Y: unknownY4}
		}
		model.Vertices = append(model.Vertices, vertex)
	}

	// 读取切线数据
	// Read tangent data
	tangentCount, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("failed to read the number of tangents: %w", err)
	}
	if err := validateNonNegativeCount("model tangent count", tangentCount); err != nil {
		return nil, err
	}

	if tangentCount > 0 {
		model.Tangents = makeCountedSliceForAppend[Quaternion](tangentCount)
		for i := int32(0); i < tangentCount; i++ {
			x, err := reader.ReadFloat32()
			if err != nil {
				return nil, fmt.Errorf("failed to read tangent X: %w", err)
			}
			y, err := reader.ReadFloat32()
			if err != nil {
				return nil, fmt.Errorf("failed to read tangent Y: %w", err)
			}
			z, err := reader.ReadFloat32()
			if err != nil {
				return nil, fmt.Errorf("failed to read tangent Z: %w", err)
			}
			w, err := reader.ReadFloat32()
			if err != nil {
				return nil, fmt.Errorf("failed to read tangent W: %w", err)
			}
			model.Tangents = append(model.Tangents, Quaternion{X: x, Y: y, Z: z, W: w})
		}
	}

	// 读取骨骼权重
	// Read bone weights
	model.BoneWeights = makeCountedSliceForAppend[BoneWeight](model.VertCount)
	for i := int32(0); i < model.VertCount; i++ {
		var bw BoneWeight

		bw.BoneIndex0, err = reader.ReadUInt16()
		if err != nil {
			return nil, fmt.Errorf("failed to read bone weight index 0: %w", err)
		}

		bw.BoneIndex1, err = reader.ReadUInt16()
		if err != nil {
			return nil, fmt.Errorf("failed to read bone weight index 1: %w", err)
		}

		bw.BoneIndex2, err = reader.ReadUInt16()
		if err != nil {
			return nil, fmt.Errorf("failed to read bone weight index 2: %w", err)
		}

		bw.BoneIndex3, err = reader.ReadUInt16()
		if err != nil {
			return nil, fmt.Errorf("failed to read bone weight index 3: %w", err)
		}

		bw.Weight0, err = reader.ReadFloat32()
		if err != nil {
			return nil, fmt.Errorf("failed to read bone weight 0: %w", err)
		}

		bw.Weight1, err = reader.ReadFloat32()
		if err != nil {
			return nil, fmt.Errorf("failed to read bone weight 1: %w", err)
		}

		bw.Weight2, err = reader.ReadFloat32()
		if err != nil {
			return nil, fmt.Errorf("failed to read bone weight 2: %w", err)
		}

		bw.Weight3, err = reader.ReadFloat32()
		if err != nil {
			return nil, fmt.Errorf("failed to read bone weight 3: %w", err)
		}
		model.BoneWeights = append(model.BoneWeights, bw)
	}

	// 读取子网格数据
	// Read submesh data
	model.SubMeshes = makeCountedSliceForAppend[[]int32](model.SubMeshCount)
	for i := int32(0); i < model.SubMeshCount; i++ {
		triCount, err := reader.ReadInt32()
		if err != nil {
			return nil, fmt.Errorf("failed to read submesh triangle count: %w", err)
		}
		if err := validateNonNegativeCount(fmt.Sprintf("model submesh[%d] triangle count", i), triCount); err != nil {
			return nil, err
		}

		triangles := makeCountedSliceForAppend[int32](triCount)
		for j := int32(0); j < triCount; j++ {
			index, err := reader.ReadUInt16()
			if err != nil {
				return nil, fmt.Errorf("failed to read submesh triangle index: %w", err)
			}
			triangles = append(triangles, int32(index))
		}
		model.SubMeshes = append(model.SubMeshes, triangles)
	}

	// 读取材质数据
	// Read material data
	materialCount, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("failed to read the number of materials: %w", err)
	}
	if err := validateNonNegativeCount("model material count", materialCount); err != nil {
		return nil, err
	}
	model.Materials = makeCountedSliceForAppend[*Material](materialCount)
	for i := int32(0); i < materialCount; i++ {
		material, err := readMaterial(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to read material: %w", err)
		}
		model.Materials = append(model.Materials, material)
	}

	// 读取形态数据
	// Read morph data
	for {
		tag, err := reader.ReadString()
		if err != nil {
			return nil, fmt.Errorf("failed to read tag: %w", err)
		}

		if tag == EndTag {
			break
		}

		if tag == "morph" {
			morphData, err := ReadMorphData(reader, model.Version)
			if err != nil {
				return nil, fmt.Errorf("failed to read morph data: %w", err)
			}
			model.MorphData = append(model.MorphData, morphData)
		}
	}

	// 检查版本号，读取SkinThickness
	// Check the version and read SkinThickness
	if model.Version >= 2100 {
		hasSkinThickness, err := reader.ReadInt32()
		if err != nil {
			// 这可能是文件结束，不返回错误
			// This can be the end of the file, so do not return an error for EOF
			if err == io.EOF {
				return model, nil
			}
			return nil, fmt.Errorf("failed to read skin thickness flag: %w", err)
		}

		if hasSkinThickness != 0 {
			model.SkinThickness, err = ReadSkinThickness(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to read skin thickness: %w", err)
			}
		}
	}

	return model, nil
}

// ReadMorphData 从 r 中读取形态数据
// ReadMorphData reads morph data from r
func ReadMorphData(reader *stream.BinaryReader, version int32) (*MorphData, error) {
	md := &MorphData{}
	var err error

	md.Name, err = reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("failed to read the morph name: %w", err)
	}

	vertCount, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("failed to read the number of morph vertices: %w", err)
	}
	if err := validateNonNegativeCount("morph vertex count", vertCount); err != nil {
		return nil, err
	}

	md.Indices = makeCountedSliceForAppend[int](vertCount)
	md.Vertex = makeCountedSliceForAppend[Vector3](vertCount)
	md.Normals = makeCountedSliceForAppend[Vector3](vertCount)

	// 2102 版本支持
	// Supported from version 2102
	hasTangents := false
	if version >= 2102 {
		hasTangents, err = reader.ReadBool()
		if err != nil {
			return nil, fmt.Errorf("failed to read has tangents flag: %w", err)
		}

		if hasTangents {
			md.Tangents = makeCountedSliceForAppend[Quaternion](vertCount)
		}
	}

	for i := int32(0); i < vertCount; i++ {
		index, err := reader.ReadUInt16()
		if err != nil {
			return nil, fmt.Errorf("failed to read the morph vertex index.: %w", err)
		}
		md.Indices = append(md.Indices, int(index))

		// 读取顶点位移
		// Read the vertex displacement
		x, err := reader.ReadFloat32()
		if err != nil {
			return nil, fmt.Errorf("failed to read morph vertex displacement X: %w", err)
		}
		y, err := reader.ReadFloat32()
		if err != nil {
			return nil, fmt.Errorf("failed to read morph vertex displacement Y: %w", err)
		}
		z, err := reader.ReadFloat32()
		if err != nil {
			return nil, fmt.Errorf("failed to read morph vertex displacement Z: %w", err)
		}
		md.Vertex = append(md.Vertex, Vector3{X: x, Y: y, Z: z})

		// 读取法线位移
		// Read the normal displacement
		x, err = reader.ReadFloat32()
		if err != nil {
			return nil, fmt.Errorf("failed to read the morph normal displacement X: %w", err)
		}
		y, err = reader.ReadFloat32()
		if err != nil {
			return nil, fmt.Errorf("failed to read the morph normal displacement Y: %w", err)
		}
		z, err = reader.ReadFloat32()
		if err != nil {
			return nil, fmt.Errorf("failed to read the morph normal displacement Z: %w", err)
		}
		md.Normals = append(md.Normals, Vector3{X: x, Y: y, Z: z})

		// 如果有切线数据，读取切线
		// Read tangents when tangent data is present
		if hasTangents {
			x, err := reader.ReadFloat32()
			if err != nil {
				return nil, fmt.Errorf("failed to read morph tangent X: %w", err)
			}
			y, err := reader.ReadFloat32()
			if err != nil {
				return nil, fmt.Errorf("failed to read morph tangent Y: %w", err)
			}
			z, err := reader.ReadFloat32()
			if err != nil {
				return nil, fmt.Errorf("failed to read morph tangent Z: %w", err)
			}
			w, err := reader.ReadFloat32()
			if err != nil {
				return nil, fmt.Errorf("failed to read morph tangent W: %w", err)
			}
			md.Tangents = append(md.Tangents, Quaternion{X: x, Y: y, Z: z, W: w})
		}
	}

	return md, nil
}

// ReadSkinThickness 从 r 中读取皮肤厚度数据
// ReadSkinThickness reads skin-thickness data from r
func ReadSkinThickness(reader *stream.BinaryReader) (*SkinThickness, error) {
	skinThickness := &SkinThickness{
		Groups: make(map[string]*ThickGroup),
	}

	var err error

	// 读取签名
	// Read the signature
	skinThickness.Signature, err = reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("failed to read skin thickness signature: %w", err)
	}
	// if signature != SkinThicknessSignature {
	// 	return nil, fmt.Errorf("invalid skin thickness signature: got %q, want %s", signature, SkinThicknessSignature)
	// }

	// 读取版本号
	// Read the version
	skinThickness.Version, err = reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("failed to read skin thickness version: %w", err)
	}

	// 读取使用标志
	// Read the use flag
	skinThickness.Use, err = reader.ReadBool()
	if err != nil {
		return nil, fmt.Errorf("failed to read skin thickness use flag: %w", err)
	}

	// 读取组数量
	// Read the group count
	groupCount, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("failed to read skin thickness group count: %w", err)
	}
	if err := validateNonNegativeCount("skin thickness group count", groupCount); err != nil {
		return nil, err
	}

	// 读取每个组
	// Read each group
	for i := int32(0); i < groupCount; i++ {
		key, err := reader.ReadString()
		if err != nil {
			return nil, fmt.Errorf("failed to read skin thickness group key: %w", err)
		}

		group := &ThickGroup{}
		err = readThickGroup(reader, group)
		if err != nil {
			return nil, fmt.Errorf("failed to read skin thickness group: %w", err)
		}

		if _, exists := skinThickness.Groups[key]; exists {
			return nil, fmt.Errorf("duplicate skin thickness group key %q", key)
		}
		skinThickness.Groups[key] = group
		skinThickness.GroupOrder = append(skinThickness.GroupOrder, key)
	}

	return skinThickness, nil
}

// readThickGroup 从 r 中读取皮肤厚度组数据
// readThickGroup reads skin-thickness group data from r
func readThickGroup(reader *stream.BinaryReader, group *ThickGroup) error {
	var err error

	// 读取组名
	// Read the group name
	group.GroupName, err = reader.ReadString()
	if err != nil {
		return fmt.Errorf("failed to read group name: %w", err)
	}

	// 读取起始骨骼名
	// Read the start bone name
	group.StartBoneName, err = reader.ReadString()
	if err != nil {
		return fmt.Errorf("failed to read start bone name: %w", err)
	}

	// 读取结束骨骼名
	// Read the end bone name
	group.EndBoneName, err = reader.ReadString()
	if err != nil {
		return fmt.Errorf("failed to read end bone name: %w", err)
	}

	// 读取角度步长
	// Read the angle step
	group.StepAngleDegree, err = reader.ReadInt32()
	if err != nil {
		return fmt.Errorf("failed to read step angle degree: %w", err)
	}

	// 读取点数量
	// Read the point count
	pointCount, err := reader.ReadInt32()
	if err != nil {
		return fmt.Errorf("failed to read point count: %w", err)
	}
	if err := validateNonNegativeCount("skin thickness point count", pointCount); err != nil {
		return err
	}

	// 读取每个点
	// Read each point
	group.Points = makeCountedSliceForAppend[*ThickPoint](pointCount)
	for i := int32(0); i < pointCount; i++ {
		point := &ThickPoint{}
		err = readThickPoint(reader, point)
		if err != nil {
			return fmt.Errorf("failed to read point: %w", err)
		}
		group.Points = append(group.Points, point)
	}

	return nil
}

// readThickPoint 从 r 中读取皮肤厚度点数据
// readThickPoint reads skin-thickness point data from r
func readThickPoint(reader *stream.BinaryReader, point *ThickPoint) error {
	var err error

	// 读取目标骨骼名
	// Read the target bone name
	point.TargetBoneName, err = reader.ReadString()
	if err != nil {
		return fmt.Errorf("failed to read target bone name: %w", err)
	}

	// 读取起始到结束的比例
	// Read the ratio from start to end
	point.RatioSegmentStartToEnd, err = reader.ReadFloat32()
	if err != nil {
		return fmt.Errorf("failed to read ratio segment start to end: %w", err)
	}

	// 读取角度定义数量
	// Read the angle-definition count
	angleDefCount, err := reader.ReadInt32()
	if err != nil {
		return fmt.Errorf("failed to read angle definition count: %w", err)
	}
	if err := validateNonNegativeCount("skin thickness angle definition count", angleDefCount); err != nil {
		return err
	}

	// 读取每个角度定义
	// Read each angle definition
	point.DistanceParAngle = makeCountedSliceForAppend[*ThickDefPerAngle](angleDefCount)
	for i := int32(0); i < angleDefCount; i++ {
		angleDef := &ThickDefPerAngle{}
		err = readThickDefPerAngle(reader, angleDef)
		if err != nil {
			return fmt.Errorf("failed to read angle definition: %w", err)
		}
		point.DistanceParAngle = append(point.DistanceParAngle, angleDef)
	}

	return nil
}

// readThickDefPerAngle 从 r 中读取每个角度的皮肤厚度定义
// readThickDefPerAngle reads a skin-thickness definition for one angle from r
func readThickDefPerAngle(reader *stream.BinaryReader, angleDef *ThickDefPerAngle) error {
	var err error

	// 读取角度
	// Read the angle
	angleDef.AngleDegree, err = reader.ReadInt32()
	if err != nil {
		return fmt.Errorf("failed to read angle degree: %w", err)
	}

	// 读取顶点索引
	// Read the vertex index
	angleDef.VertexIndex, err = reader.ReadInt32()
	if err != nil {
		return fmt.Errorf("failed to read vertex index: %w", err)
	}

	// 读取默认距离
	// Read the default distance
	angleDef.DefaultDistance, err = reader.ReadFloat32()
	if err != nil {
		return fmt.Errorf("failed to read default distance: %w", err)
	}

	return nil
}

// ReadModelMetadata 仅读取 Model 元数据，返回精简的 ModelMetadata
// ReadModelMetadata reads only Model metadata and returns a reduced ModelMetadata
func ReadModelMetadata(r io.Reader) (*ModelMetadata, error) {
	rp, ok := r.(stream.Peeker)
	if !ok {
		return nil, fmt.Errorf("ReadModelMetadata: the reader is not peekable, wrap it with bufio.Reader first")
	}

	metadata := &ModelMetadata{}
	reader := stream.NewBinaryReader(rp)

	// 读取签名
	// Read the signature
	var err error
	metadata.Signature, err = reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("failed to read signature: %w", err)
	}

	// 读取版本号
	// Read the version
	metadata.Version, err = reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("failed to read version: %w", err)
	}

	// 读取模型名称
	// Read the model name
	metadata.Name, err = reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("failed to read name: %w", err)
	}

	// 读取根骨骼名称
	// Read the root bone name
	metadata.RootBoneName, err = reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("failed to read root bone name: %w", err)
	}

	// 读取骨骼数量 (用于后续循环)
	// Read the bone count for the loops that follow
	boneCount, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("failed to read bone count: %w", err)
	}
	if err := validateNonNegativeCount("model metadata bone count", boneCount); err != nil {
		return nil, err
	}

	// 读取阴影投射方式
	// Read the shadow-casting mode
	if metadata.Version >= 2104 && metadata.Version < 2200 {
		shadowCastingMode, err := reader.ReadString()
		if err != nil {
			return nil, fmt.Errorf("failed to read shadow casting mode: %w", err)
		}
		metadata.ShadowCastingMode = &shadowCastingMode
	}

	// 骨骼信息 (不得不读，因为有变长的 Name)
	// Bone information must be read because Name has variable length
	for i := int32(0); i < boneCount; i++ {
		// 骨骼名称
		// Bone Name
		_, err = reader.ReadString()
		if err != nil {
			return nil, fmt.Errorf("failed to read bone name: %w", err)
		}

		// 缩放辅助节点标志
		// HasScale
		_, err = reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read bone scaling flags: %w", err)
		}
	}

	// 跳过骨骼父子关系 (boneCount * 4 字节)
	// Skip the bone hierarchy (boneCount * 4 bytes)
	if _, err := io.CopyN(io.Discard, r, int64(boneCount)*4); err != nil {
		return nil, fmt.Errorf("failed to skip bone parent indices: %w", err)
	}

	// 跳过骨骼变换信息
	// Skip bone transforms
	for i := int32(0); i < boneCount; i++ {
		// 位置(12) + 旋转(16) = 28
		// Position(12) + Rotation(16) = 28
		skipLen := int64(28)
		if _, err := io.CopyN(io.Discard, r, skipLen); err != nil {
			return nil, fmt.Errorf("failed to skip bone transform: %w", err)
		}

		if metadata.Version >= 2001 {
			hasScale, err := reader.ReadBool()
			if err != nil {
				return nil, fmt.Errorf("failed to read bone scaling flag: %w", err)
			}
			if hasScale {
				// 缩放(12)
				// Scale(12)
				if _, err := io.CopyN(io.Discard, r, 12); err != nil {
					return nil, fmt.Errorf("failed to skip bone scale: %w", err)
				}
			}
		}
	}

	// 读取网格基本信息
	// Read basic mesh information
	vertCount, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("failed to read the number of vertices: %w", err)
	}
	if err := validateNonNegativeCount("model metadata vertex count", vertCount); err != nil {
		return nil, err
	}

	subMeshCount, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("failed to read the number of subgrids: %w", err)
	}
	if err := validateNonNegativeCount("model metadata submesh count", subMeshCount); err != nil {
		return nil, err
	}

	meshBoneCount, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("failed to read the number of bones: %w", err)
	}
	if err := validateNonNegativeCount("model metadata mesh bone count", meshBoneCount); err != nil {
		return nil, err
	}

	// 读取网格关联的骨骼名称 (不得不读)
	// Read the mesh bone names because they have variable length
	for i := int32(0); i < meshBoneCount; i++ {
		_, err := reader.ReadString()
		if err != nil {
			return nil, fmt.Errorf("failed to read bone name (at bone index): %w", err)
		}
	}

	// 跳过骨骼绑定姿势 (meshBoneCount * 16 * 4)
	// Skip bone bind poses (meshBoneCount * 16 * 4)
	if _, err := io.CopyN(io.Discard, r, int64(meshBoneCount)*64); err != nil {
		return nil, fmt.Errorf("failed to skip bind poses: %w", err)
	}

	// UV 标志
	// UV flags
	hasUV2 := false
	hasUV3 := false
	hasUV4 := false
	hasUnknownFlag1 := false
	hasUnknownFlag2 := false
	hasUnknownFlag3 := false
	hasUnknownFlag4 := false

	if metadata.Version >= 2101 {
		if hasUV2, err = reader.ReadBool(); err != nil {
			return nil, err
		}
		if hasUV3, err = reader.ReadBool(); err != nil {
			return nil, err
		}
		if hasUV4, err = reader.ReadBool(); err != nil {
			return nil, err
		}
		if hasUnknownFlag1, err = reader.ReadBool(); err != nil {
			return nil, err
		}
		if hasUnknownFlag2, err = reader.ReadBool(); err != nil {
			return nil, err
		}
		if hasUnknownFlag3, err = reader.ReadBool(); err != nil {
			return nil, err
		}
		if hasUnknownFlag4, err = reader.ReadBool(); err != nil {
			return nil, err
		}
	}

	// 计算单个顶点的长度
	// Calculate the size of one vertex
	// 位置(12) + 法线(12) + UV(8)
	// Pos(12) + Norm(12) + UV(8)
	vertLen := int64(12 + 12 + 8)
	if hasUV2 {
		vertLen += 8
	}
	if hasUV3 {
		vertLen += 8
	}
	if hasUV4 {
		vertLen += 8
	}
	if hasUnknownFlag1 {
		vertLen += 8
	}
	if hasUnknownFlag2 {
		vertLen += 8
	}
	if hasUnknownFlag3 {
		vertLen += 8
	}
	if hasUnknownFlag4 {
		vertLen += 8
	}

	// 跳过所有顶点
	// Skip all vertices
	if _, err := io.CopyN(io.Discard, r, int64(vertCount)*vertLen); err != nil {
		return nil, fmt.Errorf("failed to skip vertices: %w", err)
	}

	// 跳过切线数据
	// Skip tangent data
	tangentCount, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("failed to read tangent count: %w", err)
	}
	if err := validateNonNegativeCount("model metadata tangent count", tangentCount); err != nil {
		return nil, err
	}
	if _, err := io.CopyN(io.Discard, r, int64(tangentCount)*16); err != nil {
		return nil, fmt.Errorf("failed to skip tangents: %w", err)
	}

	// 跳过骨骼权重 (每个顶点 24 字节: 4*uint16 + 4*float32 = 8 + 16 = 24)
	// Skip bone weights (24 bytes per vertex: 4*uint16 + 4*float32 = 8 + 16 = 24)
	if _, err := io.CopyN(io.Discard, r, int64(vertCount)*24); err != nil {
		return nil, fmt.Errorf("failed to skip bone weights: %w", err)
	}

	// 跳过子网格数据
	// Skip submesh data
	for i := int32(0); i < subMeshCount; i++ {
		triCount, err := reader.ReadInt32()
		if err != nil {
			return nil, err
		}
		if err := validateNonNegativeCount(fmt.Sprintf("model metadata submesh[%d] triangle count", i), triCount); err != nil {
			return nil, err
		}
		// UInt16 索引
		// uint16 indices
		if _, err := io.CopyN(io.Discard, r, int64(triCount)*2); err != nil {
			return nil, fmt.Errorf("failed to skip submesh triangles: %w", err)
		}
	}

	// 读取材质
	// Read materials
	materialCount, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("failed to read material count: %w", err)
	}
	if err := validateNonNegativeCount("model metadata material count", materialCount); err != nil {
		return nil, err
	}

	metadata.Materials = makeCountedSliceForAppend[*Material](materialCount)
	for i := int32(0); i < materialCount; i++ {
		material, err := readMaterial(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to read material %d: %w", i, err)
		}
		metadata.Materials = append(metadata.Materials, material)
	}

	return metadata, nil
}

// Dump 将 Model 写到 w 中，生成 CM3D2_MESH 二进制数据
// Dump writes Model to w as CM3D2_MESH binary data
func (m *Model) Dump(w io.Writer) error {
	if err := validateModelForDump(m); err != nil {
		return err
	}
	// 以下三个字段是从对应集合推导的线格式计数
	// 像游戏写入器一样重新计算它们，避免过期的编辑元数据使原本有效的模型无法读取
	// These three fields are wire counts derived from the collections below
	// Recompute them just like the game's writer so stale editing metadata
	// cannot make an otherwise valid model unreadable
	m.VertCount = int32(len(m.Vertices))
	m.SubMeshCount = int32(len(m.SubMeshes))
	m.BoneCount = int32(len(m.BoneNames))
	writer := stream.NewBinaryWriter(w)

	// 写入签名
	// Write the signature
	if err := writer.WriteString(m.Signature); err != nil {
		return fmt.Errorf("failed to write signature: %w", err)
	}

	// 写入版本号
	// Write the version
	if err := writer.WriteInt32(m.Version); err != nil {
		return fmt.Errorf("failed to write version: %w", err)
	}

	// 写入模型名称
	// Write the model name
	if err := writer.WriteString(m.Name); err != nil {
		return fmt.Errorf("failed to write name: %w", err)
	}

	// 写入根骨骼名称
	// Write the root bone name
	if err := writer.WriteString(m.RootBoneName); err != nil {
		return fmt.Errorf("failed to write root bone name: %w", err)
	}

	// 写入骨骼数量
	// Write the bone count
	if err := writer.WriteInt32(int32(len(m.Bones))); err != nil {
		return fmt.Errorf("failed to write bone count: %w", err)
	}

	// 写入阴影投射方式（如果版本支持）
	// Write the shadow-casting mode when supported by the version
	if m.Version >= 2104 && m.Version < 2200 {
		if m.ShadowCastingMode == nil {
			return fmt.Errorf("ShadowCastingMode is nil. ShadowCastingMode is required, when version >= 2104 and < 2200")
		}
		if err := writer.WriteString(*m.ShadowCastingMode); err != nil {
			return fmt.Errorf("failed to write shadow casting mode: %w", err)
		}
	}

	// 写入骨骼数据
	// Write bone data
	for _, bone := range m.Bones {
		// 写入骨骼名称
		// Write the bone name
		if err := writer.WriteString(bone.Name); err != nil {
			return fmt.Errorf("failed to write bone name: %w", err)
		}

		// 写入骨骼缩放标志
		// Write the bone scaling-helper flag
		if err := writer.WriteByte(utilities.BoolToByte(bone.HasScale)); err != nil {
			return fmt.Errorf("failed to write bone scaling flags: %w", err)
		}
	}

	// 写入骨骼父子关系
	// Write the bone hierarchy
	for _, bone := range m.Bones {
		// 写入父骨骼索引
		// Write the parent bone index
		if err := writer.WriteInt32(bone.ParentIndex); err != nil {
			return fmt.Errorf("failed to write bone parent index: %w", err)
		}
	}

	// 写入骨骼变换信息
	// Write bone transforms
	for _, bone := range m.Bones {
		// 写入位置
		// Write the position
		if err := writer.WriteFloat32(bone.Position.X); err != nil {
			return fmt.Errorf("failed to write bone position X: %w", err)
		}
		if err := writer.WriteFloat32(bone.Position.Y); err != nil {
			return fmt.Errorf("failed to write bone position Y: %w", err)
		}
		if err := writer.WriteFloat32(bone.Position.Z); err != nil {
			return fmt.Errorf("failed to write bone position Z: %w", err)
		}

		// 写入旋转
		// Write the rotation
		if err := writer.WriteFloat32(bone.Rotation.X); err != nil {
			return fmt.Errorf("failed to write bone rotation X: %w", err)
		}
		if err := writer.WriteFloat32(bone.Rotation.Y); err != nil {
			return fmt.Errorf("failed to write bone rotation Y: %w", err)
		}
		if err := writer.WriteFloat32(bone.Rotation.Z); err != nil {
			return fmt.Errorf("failed to write bone rotation Z: %w", err)
		}
		if err := writer.WriteFloat32(bone.Rotation.W); err != nil {
			return fmt.Errorf("failed to write bone rotation W: %w", err)
		}

		// 如果版本大于等于2001，处理骨骼缩放
		// Handle bone scale when the version is at least 2001
		if m.Version >= 2001 {
			hasScale := bone.Scale != nil
			if err := writer.WriteBool(hasScale); err != nil {
				return fmt.Errorf("failed to write bone scaling flag: %w", err)
			}

			if hasScale {
				if err := writer.WriteFloat32(bone.Scale.X); err != nil {
					return fmt.Errorf("failed to write bone scale X: %w", err)
				}
				if err := writer.WriteFloat32(bone.Scale.Y); err != nil {
					return fmt.Errorf("failed to write bone scale Y: %w", err)
				}
				if err := writer.WriteFloat32(bone.Scale.Z); err != nil {
					return fmt.Errorf("failed to write bone scale Z: %w", err)
				}
			}
		}
	}

	// 写入网格基本信息
	// Write basic mesh information
	if err := writer.WriteInt32(m.VertCount); err != nil {
		return fmt.Errorf("failed to write the number of vertices: %w", err)
	}

	if err := writer.WriteInt32(m.SubMeshCount); err != nil {
		return fmt.Errorf("failed to write the number of subgrids: %w", err)
	}

	if err := writer.WriteInt32(m.BoneCount); err != nil {
		return fmt.Errorf("failed to write the number of bones: %w", err)
	}

	// 写入骨骼名称
	// Write mesh bone names
	for _, boneName := range m.BoneNames {
		if err := writer.WriteString(boneName); err != nil {
			return fmt.Errorf("failed to write bone name (at bone index): %w", err)
		}
	}

	// 写入骨骼绑定姿势
	// Write bone bind poses
	for _, bindPose := range m.BindPoses {
		if err := writer.WriteFloat4x4(bindPose); err != nil {
			return fmt.Errorf("failed to write the armature binding pose: %w", err)
		}
	}

	// 如果版本为 2101 或更高，写入额外标志位
	// Write the additional channel flags when the version is 2101 or later
	if m.Version >= 2101 {
		// 确定是否有 UV2、UV3、UV4 和未知标志位
		// Determine whether UV2, UV3, UV4, and the unknown channels are present
		hasUV2 := false
		hasUV3 := false
		hasUV4 := false
		hasUnknownFlag1 := false
		hasUnknownFlag2 := false
		hasUnknownFlag3 := false
		hasUnknownFlag4 := false

		// 检查第一个顶点确定是否存在这些标志位
		// Inspect the first vertex to determine channel presence
		if len(m.Vertices) > 0 {
			hasUV2 = m.Vertices[0].UV2 != nil
			hasUV3 = m.Vertices[0].UV3 != nil
			hasUV4 = m.Vertices[0].UV4 != nil
			hasUnknownFlag1 = m.Vertices[0].Unknown1 != nil
			hasUnknownFlag2 = m.Vertices[0].Unknown2 != nil
			hasUnknownFlag3 = m.Vertices[0].Unknown3 != nil
			hasUnknownFlag4 = m.Vertices[0].Unknown4 != nil
		}

		// 写入UV标志位
		// Write the UV flags
		if err := writer.WriteBool(hasUV2); err != nil {
			return fmt.Errorf("failed to write UV2 flag: %w", err)
		}
		if err := writer.WriteBool(hasUV3); err != nil {
			return fmt.Errorf("failed to write UV3 flag: %w", err)
		}
		if err := writer.WriteBool(hasUV4); err != nil {
			return fmt.Errorf("failed to write UV4 flag: %w", err)
		}

		// 写入未知标志位
		// Write the unknown flags
		if err := writer.WriteBool(hasUnknownFlag1); err != nil {
			return fmt.Errorf("failed to write unknown flag 1: %w", err)
		}
		if err := writer.WriteBool(hasUnknownFlag2); err != nil {
			return fmt.Errorf("failed to write unknown flag 2: %w", err)
		}
		if err := writer.WriteBool(hasUnknownFlag3); err != nil {
			return fmt.Errorf("failed to write unknown flag 3: %w", err)
		}
		if err := writer.WriteBool(hasUnknownFlag4); err != nil {
			return fmt.Errorf("failed to write unknown flag 4: %w", err)
		}
	}

	// 写入顶点数据
	// Write vertex data
	for _, vertex := range m.Vertices {
		// 写入顶点位置
		// Write the vertex position
		if err := writer.WriteFloat32(vertex.Position.X); err != nil {
			return fmt.Errorf("failed to write vertex position X: %w", err)
		}
		if err := writer.WriteFloat32(vertex.Position.Y); err != nil {
			return fmt.Errorf("failed to write vertex position Y: %w", err)
		}
		if err := writer.WriteFloat32(vertex.Position.Z); err != nil {
			return fmt.Errorf("failed to write vertex position Z: %w", err)
		}

		// 写入法线
		// Write the normal
		if err := writer.WriteFloat32(vertex.Normal.X); err != nil {
			return fmt.Errorf("failed to write vertex normal X: %w", err)
		}
		if err := writer.WriteFloat32(vertex.Normal.Y); err != nil {
			return fmt.Errorf("failed to write vertex normal Y: %w", err)
		}
		if err := writer.WriteFloat32(vertex.Normal.Z); err != nil {
			return fmt.Errorf("failed to write vertex normal Z: %w", err)
		}

		// 写入UV坐标
		// Write UV coordinates
		if err := writer.WriteFloat32(vertex.UV.X); err != nil {
			return fmt.Errorf("failed to write vertex UV coordinate X: %w", err)
		}
		if err := writer.WriteFloat32(vertex.UV.Y); err != nil {
			return fmt.Errorf("failed to write vertex UV coordinate Y: %w", err)
		}

		// 写入UV2坐标（如果存在）
		// Write UV2 coordinates when present
		if m.Version >= 2101 && vertex.UV2 != nil {
			if err := writer.WriteFloat32(vertex.UV2.X); err != nil {
				return fmt.Errorf("failed to write vertex UV2 coordinate X: %w", err)
			}
			if err := writer.WriteFloat32(vertex.UV2.Y); err != nil {
				return fmt.Errorf("failed to write vertex UV2 coordinate Y: %w", err)
			}
		}

		// 写入UV3坐标（如果存在）
		// Write UV3 coordinates when present
		if m.Version >= 2101 && vertex.UV3 != nil {
			if err := writer.WriteFloat32(vertex.UV3.X); err != nil {
				return fmt.Errorf("failed to write vertex UV3 coordinate X: %w", err)
			}
			if err := writer.WriteFloat32(vertex.UV3.Y); err != nil {
				return fmt.Errorf("failed to write vertex UV3 coordinate Y: %w", err)
			}
		}

		// 写入UV4坐标（如果存在）
		// Write UV4 coordinates when present
		if m.Version >= 2101 && vertex.UV4 != nil {
			if err := writer.WriteFloat32(vertex.UV4.X); err != nil {
				return fmt.Errorf("failed to write vertex UV4 coordinate X: %w", err)
			}
			if err := writer.WriteFloat32(vertex.UV4.Y); err != nil {
				return fmt.Errorf("failed to write vertex UV4 coordinate Y: %w", err)
			}
		}

		// 写入未知标志位对应的数据（如果存在）
		// Write data for the unknown flags when present
		if m.Version >= 2101 && vertex.Unknown1 != nil {
			if err := writer.WriteFloat32(vertex.Unknown1.X); err != nil {
				return fmt.Errorf("failed to write unknown flag 1 data X: %w", err)
			}
			if err := writer.WriteFloat32(vertex.Unknown1.Y); err != nil {
				return fmt.Errorf("failed to write unknown flag 1 data Y: %w", err)
			}
		}

		if m.Version >= 2101 && vertex.Unknown2 != nil {
			if err := writer.WriteFloat32(vertex.Unknown2.X); err != nil {
				return fmt.Errorf("failed to write unknown flag 2 data X: %w", err)
			}
			if err := writer.WriteFloat32(vertex.Unknown2.Y); err != nil {
				return fmt.Errorf("failed to write unknown flag 2 data Y: %w", err)
			}
		}

		if m.Version >= 2101 && vertex.Unknown3 != nil {
			if err := writer.WriteFloat32(vertex.Unknown3.X); err != nil {
				return fmt.Errorf("failed to write unknown flag 3 data X: %w", err)
			}
			if err := writer.WriteFloat32(vertex.Unknown3.Y); err != nil {
				return fmt.Errorf("failed to write unknown flag 3 data Y: %w", err)
			}
		}

		if m.Version >= 2101 && vertex.Unknown4 != nil {
			if err := writer.WriteFloat32(vertex.Unknown4.X); err != nil {
				return fmt.Errorf("failed to write unknown flag 4 data X: %w", err)
			}
			if err := writer.WriteFloat32(vertex.Unknown4.Y); err != nil {
				return fmt.Errorf("failed to write unknown flag 4 data Y: %w", err)
			}
		}
	}

	// 写入切线数据
	// Write tangent data
	if m.Tangents != nil {
		if err := writer.WriteInt32(int32(len(m.Tangents))); err != nil {
			return fmt.Errorf("failed to write the number of tangents: %w", err)
		}

		for _, tangent := range m.Tangents {
			if err := writer.WriteFloat32(tangent.X); err != nil {
				return fmt.Errorf("failed to write tangent X: %w", err)
			}
			if err := writer.WriteFloat32(tangent.Y); err != nil {
				return fmt.Errorf("failed to write tangent Y: %w", err)
			}
			if err := writer.WriteFloat32(tangent.Z); err != nil {
				return fmt.Errorf("failed to write tangent Z: %w", err)
			}
			if err := writer.WriteFloat32(tangent.W); err != nil {
				return fmt.Errorf("failed to write tangent W: %w", err)
			}
		}
	} else {
		// 如果没有切线数据，写入0
		// Write 0 when tangent data is absent
		if err := writer.WriteInt32(0); err != nil {
			return fmt.Errorf("failed to write the number of tangents: %w", err)
		}
	}

	// 写入骨骼权重
	// Write bone weights
	for _, bw := range m.BoneWeights {
		if err := writer.WriteUInt16(bw.BoneIndex0); err != nil {
			return fmt.Errorf("failed to write bone weight index 0: %w", err)
		}
		if err := writer.WriteUInt16(bw.BoneIndex1); err != nil {
			return fmt.Errorf("failed to write bone weight index 1: %w", err)
		}
		if err := writer.WriteUInt16(bw.BoneIndex2); err != nil {
			return fmt.Errorf("failed to write bone weight index 2: %w", err)
		}
		if err := writer.WriteUInt16(bw.BoneIndex3); err != nil {
			return fmt.Errorf("failed to write bone weight index 3: %w", err)
		}

		if err := writer.WriteFloat32(bw.Weight0); err != nil {
			return fmt.Errorf("failed to write bone weight 0: %w", err)
		}
		if err := writer.WriteFloat32(bw.Weight1); err != nil {
			return fmt.Errorf("failed to write bone weight 1: %w", err)
		}
		if err := writer.WriteFloat32(bw.Weight2); err != nil {
			return fmt.Errorf("failed to write bone weight 2: %w", err)
		}
		if err := writer.WriteFloat32(bw.Weight3); err != nil {
			return fmt.Errorf("failed to write bone weight 3: %w", err)
		}
	}

	// 写入子网格数据
	// Write submesh data
	for _, subMesh := range m.SubMeshes {
		if err := writer.WriteInt32(int32(len(subMesh))); err != nil {
			return fmt.Errorf("failed to write submesh triangle count: %w", err)
		}

		for _, index := range subMesh {
			if err := writer.WriteUInt16(uint16(index)); err != nil {
				return fmt.Errorf("failed to write submesh triangle index: %w", err)
			}
		}
	}

	// 写入材质数据
	// Write material data
	if err := writer.WriteInt32(int32(len(m.Materials))); err != nil {
		return fmt.Errorf("failed to write the number of materials: %w", err)
	}
	for _, material := range m.Materials {
		if err := material.Dump(writer); err != nil {
			return fmt.Errorf("failed to write material: %w", err)
		}
	}

	// 写入形态数据
	// Write morph data
	for _, morph := range m.MorphData {
		if err := writer.WriteString("morph"); err != nil {
			return fmt.Errorf("failed to write morph tag: %w", err)
		}

		if err := writeMorphData(writer, morph, m.Version); err != nil {
			return fmt.Errorf("failed to write morph data: %w", err)
		}
	}

	// 写入结束标记
	// Write the end marker
	if err := writer.WriteString(EndTag); err != nil {
		return fmt.Errorf("failed to write end tag: %w", err)
	}

	// 如果版本号大于等于2100，写入SkinThickness
	// Write SkinThickness when the version is at least 2100
	if m.Version >= 2100 {
		if m.SkinThickness != nil {
			if err := writer.WriteInt32(1); err != nil {
				return fmt.Errorf("failed to write skin thickness flag: %w", err)
			}
			if err := writeSkinThickness(writer, m.SkinThickness); err != nil {
				return fmt.Errorf("failed to write skin thickness: %w", err)
			}
		} else {
			if err := writer.WriteInt32(0); err != nil {
				return fmt.Errorf("failed to write skin thickness flag: %w", err)
			}
		}
	}

	return nil
}

// modelVertexChannels 记录一个顶点是否包含各扩展 Vector2 通道
// modelVertexChannels records whether a vertex contains each extended Vector2 channel
type modelVertexChannels struct {
	uv2      bool // 是否包含 UV2 / Whether UV2 is present
	uv3      bool // 是否包含 UV3 / Whether UV3 is present
	uv4      bool // 是否包含 UV4 / Whether UV4 is present
	unknown1 bool // 是否包含未知 1 / Whether channel 1 is present
	unknown2 bool // 是否包含未知 2 / Whether channel 2 is present
	unknown3 bool // 是否包含未知 3 / Whether channel 3 is present
	unknown4 bool // 是否包含未知 4 / Whether channel 4 is present
}

// validateModelForDump 验证 Model 的版本门槛、集合长度和跨字段关系可被线格式完整表示
// validateModelForDump validates that Model version gates, collection lengths, and cross-field relationships are fully representable on the wire
func validateModelForDump(model *Model) error {
	if model == nil {
		return fmt.Errorf("nil model")
	}
	if _, err := collectionCountInt32("model bone count", len(model.Bones)); err != nil {
		return err
	}
	if _, err := collectionCountInt32("model vertex count", len(model.Vertices)); err != nil {
		return err
	}
	if _, err := collectionCountInt32("model submesh count", len(model.SubMeshes)); err != nil {
		return err
	}
	if _, err := collectionCountInt32("model mesh bone count", len(model.BoneNames)); err != nil {
		return err
	}
	if len(model.BoneNames) != len(model.BindPoses) {
		return fmt.Errorf("model has BoneNames=%d and BindPoses=%d", len(model.BoneNames), len(model.BindPoses))
	}
	if _, err := collectionCountInt32("model material count", len(model.Materials)); err != nil {
		return err
	}
	if _, err := collectionCountInt32("model tangent count", len(model.Tangents)); err != nil {
		return err
	}

	shadowSupported := model.Version >= 2104 && model.Version < 2200
	if shadowSupported && model.ShadowCastingMode == nil {
		return fmt.Errorf("ShadowCastingMode is required for model version %d", model.Version)
	}
	if !shadowSupported && model.ShadowCastingMode != nil {
		return fmt.Errorf("model version %d cannot encode ShadowCastingMode", model.Version)
	}

	for index, bone := range model.Bones {
		if bone == nil {
			return fmt.Errorf("model Bones[%d] is nil", index)
		}
		if model.Version < 2001 && bone.Scale != nil {
			return fmt.Errorf("model version %d cannot encode Bones[%d].Scale", model.Version, index)
		}
	}

	if len(model.BoneWeights) != len(model.Vertices) {
		return fmt.Errorf("model has Vertices=%d but BoneWeights=%d", len(model.Vertices), len(model.BoneWeights))
	}
	if err := validateModelVertexChannels(model); err != nil {
		return err
	}
	for submeshIndex, submesh := range model.SubMeshes {
		if _, err := collectionCountInt32(fmt.Sprintf("model SubMeshes[%d] triangle count", submeshIndex), len(submesh)); err != nil {
			return err
		}
		for indexIndex, index := range submesh {
			if index < 0 || index > 1<<16-1 {
				return fmt.Errorf("model SubMeshes[%d][%d]=%d is outside UInt16", submeshIndex, indexIndex, index)
			}
		}
	}

	for index, material := range model.Materials {
		if err := validateMaterialForDump(fmt.Sprintf("model Materials[%d]", index), material); err != nil {
			return err
		}
	}
	for index, morph := range model.MorphData {
		if err := validateMorphDataForDump(fmt.Sprintf("model MorphData[%d]", index), morph, model.Version); err != nil {
			return err
		}
	}
	if model.Version < 2100 && model.SkinThickness != nil {
		return fmt.Errorf("model version %d cannot encode SkinThickness", model.Version)
	}
	if model.SkinThickness != nil {
		if err := validateSkinThicknessForDump(model.SkinThickness); err != nil {
			return err
		}
	}
	return nil
}

// modelChannelsForVertex 返回顶点扩展通道的存在状态
// modelChannelsForVertex returns the presence state of a vertex's extended channels
func modelChannelsForVertex(vertex Vertex) modelVertexChannels {
	return modelVertexChannels{
		uv2:      vertex.UV2 != nil,
		uv3:      vertex.UV3 != nil,
		uv4:      vertex.UV4 != nil,
		unknown1: vertex.Unknown1 != nil,
		unknown2: vertex.Unknown2 != nil,
		unknown3: vertex.Unknown3 != nil,
		unknown4: vertex.Unknown4 != nil,
	}
}

// validateModelVertexChannels 验证所有顶点具有相同通道布局且版本支持这些通道
// validateModelVertexChannels verifies that all vertices share one channel layout and that the version supports it
func validateModelVertexChannels(model *Model) error {
	if len(model.Vertices) == 0 {
		return nil
	}
	want := modelChannelsForVertex(model.Vertices[0])
	if model.Version < 2101 && want != (modelVertexChannels{}) {
		return fmt.Errorf("model version %d cannot encode extended vertex channels", model.Version)
	}
	for index := 1; index < len(model.Vertices); index++ {
		if got := modelChannelsForVertex(model.Vertices[index]); got != want {
			return fmt.Errorf("model Vertices[%d] channel presence differs from Vertices[0]", index)
		}
	}
	return nil
}

// validateMorphDataForDump 验证形态数组长度、UInt16 索引和切线版本门槛
// validateMorphDataForDump validates morph array lengths, UInt16 indices, and the tangent version gate
func validateMorphDataForDump(path string, morph *MorphData, version int32) error {
	if morph == nil {
		return fmt.Errorf("%s is nil", path)
	}
	count, err := collectionCountInt32(path+" vertex count", len(morph.Indices))
	if err != nil {
		return err
	}
	if int64(count) != int64(len(morph.Vertex)) || int64(count) != int64(len(morph.Normals)) {
		return fmt.Errorf("%s has Indices=%d, Vertex=%d, Normals=%d", path, len(morph.Indices), len(morph.Vertex), len(morph.Normals))
	}
	for index, vertexIndex := range morph.Indices {
		if vertexIndex < 0 || vertexIndex > 1<<16-1 {
			return fmt.Errorf("%s.Indices[%d]=%d is outside UInt16", path, index, vertexIndex)
		}
	}
	if version < 2102 {
		if morph.Tangents != nil {
			return fmt.Errorf("model version %d cannot encode %s.Tangents", version, path)
		}
		return nil
	}
	if morph.Tangents != nil && len(morph.Tangents) != len(morph.Indices) {
		return fmt.Errorf("%s has Indices=%d but Tangents=%d", path, len(morph.Indices), len(morph.Tangents))
	}
	return nil
}

// validateSkinThicknessForDump 验证皮肤厚度组、点和角度定义的数量与非 nil 约束
// validateSkinThicknessForDump validates counts and non-nil constraints for skin-thickness groups, points, and angle definitions
func validateSkinThicknessForDump(thickness *SkinThickness) error {
	keys, err := orderedSkinThicknessGroupKeys(thickness)
	if err != nil {
		return err
	}
	if _, err := collectionCountInt32("skin thickness group count", len(thickness.Groups)); err != nil {
		return err
	}
	for _, key := range keys {
		group := thickness.Groups[key]
		if _, err := collectionCountInt32(fmt.Sprintf("skin thickness group %q point count", key), len(group.Points)); err != nil {
			return err
		}
		for pointIndex, point := range group.Points {
			if point == nil {
				return fmt.Errorf("skin thickness group %q Points[%d] is nil", key, pointIndex)
			}
			if _, err := collectionCountInt32(fmt.Sprintf("skin thickness group %q Points[%d] angle count", key, pointIndex), len(point.DistanceParAngle)); err != nil {
				return err
			}
			for angleIndex, angle := range point.DistanceParAngle {
				if angle == nil {
					return fmt.Errorf("skin thickness group %q Points[%d].DistanceParAngle[%d] is nil", key, pointIndex, angleIndex)
				}
			}
		}
	}
	return nil
}

// writeMorphData 将形态数据写入 w
// writeMorphData writes morph data to w
func writeMorphData(writer *stream.BinaryWriter, md *MorphData, version int32) error {
	// 写入形态名称
	// Write the morph name
	if err := writer.WriteString(md.Name); err != nil {
		return fmt.Errorf("failed to write the morph name: %w", err)
	}

	// 写入顶点数量
	// Write the vertex count
	if err := writer.WriteInt32(int32(len(md.Indices))); err != nil {
		return fmt.Errorf("failed to write the number of morph vertices: %w", err)
	}

	// 2102 版本支持
	// Version 2102 support
	hasTangents := md.Tangents != nil && version >= 2102
	if version >= 2102 {
		if err := writer.WriteBool(hasTangents); err != nil {
			return fmt.Errorf("failed to write has tangents flag: %w", err)
		}
	}

	for i, index := range md.Indices {
		if err := writer.WriteUInt16(uint16(index)); err != nil {
			return fmt.Errorf("failed to write the morph vertex index: %w", err)
		}

		// 写入顶点位移
		// Write the vertex displacement
		if err := writer.WriteFloat32(md.Vertex[i].X); err != nil {
			return fmt.Errorf("failed to write morph vertex displacement X: %w", err)
		}
		if err := writer.WriteFloat32(md.Vertex[i].Y); err != nil {
			return fmt.Errorf("failed to write morph vertex displacement Y: %w", err)
		}
		if err := writer.WriteFloat32(md.Vertex[i].Z); err != nil {
			return fmt.Errorf("failed to write morph vertex displacement Z: %w", err)
		}

		// 写入法线位移
		// Write the normal displacement
		if err := writer.WriteFloat32(md.Normals[i].X); err != nil {
			return fmt.Errorf("failed to write the morph normal displacement X: %w", err)
		}
		if err := writer.WriteFloat32(md.Normals[i].Y); err != nil {
			return fmt.Errorf("failed to write the morph normal displacement Y: %w", err)
		}
		if err := writer.WriteFloat32(md.Normals[i].Z); err != nil {
			return fmt.Errorf("failed to write the morph normal displacement Z: %w", err)
		}

		// 如果有切线数据，写入切线
		// Write the tangent when tangent data is present
		if hasTangents {
			if err := writer.WriteFloat32(md.Tangents[i].X); err != nil {
				return fmt.Errorf("failed to write morph tangent X: %w", err)
			}
			if err := writer.WriteFloat32(md.Tangents[i].Y); err != nil {
				return fmt.Errorf("failed to write morph tangent Y: %w", err)
			}
			if err := writer.WriteFloat32(md.Tangents[i].Z); err != nil {
				return fmt.Errorf("failed to write morph tangent Z: %w", err)
			}
			if err := writer.WriteFloat32(md.Tangents[i].W); err != nil {
				return fmt.Errorf("failed to write morph tangent W: %w", err)
			}
		}
	}

	return nil
}

// writeSkinThickness 将皮肤厚度数据写入 w
// writeSkinThickness writes skin-thickness data to w
func writeSkinThickness(writer *stream.BinaryWriter, st *SkinThickness) error {
	keys, err := orderedSkinThicknessGroupKeys(st)
	if err != nil {
		return err
	}

	// 写入签名
	// Write the signature
	if err := writer.WriteString(st.Signature); err != nil {
		return fmt.Errorf("failed to write skin thickness signature: %w", err)
	}

	// 写入版本号
	// Write the version
	if err := writer.WriteInt32(st.Version); err != nil {
		return fmt.Errorf("failed to write skin thickness version: %w", err)
	}

	// 写入使用标志
	// Write the enable flag
	if err := writer.WriteBool(st.Use); err != nil {
		return fmt.Errorf("failed to write skin thickness use flag: %w", err)
	}

	// 写入组数量
	// Write the group count
	if err := writer.WriteInt32(int32(len(st.Groups))); err != nil {
		return fmt.Errorf("failed to write skin thickness group count: %w", err)
	}

	// 写入每个组
	// Write each group
	for _, key := range keys {
		group := st.Groups[key]
		if err := writer.WriteString(key); err != nil {
			return fmt.Errorf("failed to write skin thickness group key: %w", err)
		}

		if err := writeThickGroup(writer, group); err != nil {
			return fmt.Errorf("failed to write skin thickness group: %w", err)
		}
	}

	return nil
}

// orderedSkinThicknessGroupKeys 按 GroupOrder 合并并验证皮肤厚度组的写出顺序
// orderedSkinThicknessGroupKeys merges and validates the skin-thickness group output order according to GroupOrder
func orderedSkinThicknessGroupKeys(st *SkinThickness) ([]string, error) {
	if st == nil {
		return nil, fmt.Errorf("nil skin thickness")
	}
	keys, err := utilities.MergeOrderedMapKeys(st.Groups, st.GroupOrder, "skin thickness GroupOrder")
	if err != nil {
		return nil, err
	}
	for _, key := range keys {
		group := st.Groups[key]
		if group == nil {
			return nil, fmt.Errorf("skin thickness group %q is nil", key)
		}
	}
	return keys, nil
}

// writeThickGroup 将皮肤厚度组数据写入 w
// writeThickGroup writes skin-thickness group data to w
func writeThickGroup(writer *stream.BinaryWriter, group *ThickGroup) error {
	// 写入组名
	// Write the group name
	if err := writer.WriteString(group.GroupName); err != nil {
		return fmt.Errorf("failed to write group name: %w", err)
	}

	// 写入起始骨骼名
	// Write the start bone name
	if err := writer.WriteString(group.StartBoneName); err != nil {
		return fmt.Errorf("failed to write start bone name: %w", err)
	}

	// 写入结束骨骼名
	// Write the end bone name
	if err := writer.WriteString(group.EndBoneName); err != nil {
		return fmt.Errorf("failed to write end bone name: %w", err)
	}

	// 写入角度步长
	// Write the angle step
	if err := writer.WriteInt32(group.StepAngleDegree); err != nil {
		return fmt.Errorf("failed to write step angle degree: %w", err)
	}

	// 写入点数量
	// Write the point count
	if err := writer.WriteInt32(int32(len(group.Points))); err != nil {
		return fmt.Errorf("failed to write point count: %w", err)
	}

	// 写入每个点
	// Write each point
	for _, point := range group.Points {
		if err := writeThickPoint(writer, point); err != nil {
			return fmt.Errorf("failed to write point: %w", err)
		}
	}

	return nil
}

// writeThickPoint 将皮肤厚度点数据写入 w
// writeThickPoint writes skin-thickness point data to w
func writeThickPoint(writer *stream.BinaryWriter, point *ThickPoint) error {
	// 写入目标骨骼名
	// Write the target bone name
	if err := writer.WriteString(point.TargetBoneName); err != nil {
		return fmt.Errorf("failed to write target bone name: %w", err)
	}

	// 写入起始到结束的比例
	// Write the ratio from the start to the end
	if err := writer.WriteFloat32(point.RatioSegmentStartToEnd); err != nil {
		return fmt.Errorf("failed to write ratio segment start to end: %w", err)
	}

	// 写入角度定义数量
	// Write the angle-definition count
	if err := writer.WriteInt32(int32(len(point.DistanceParAngle))); err != nil {
		return fmt.Errorf("failed to write angle definition count: %w", err)
	}

	// 写入每个角度定义
	// Write each angle definition
	for _, angleDef := range point.DistanceParAngle {
		if err := writeThickDefPerAngle(writer, angleDef); err != nil {
			return fmt.Errorf("failed to write angle definition: %w", err)
		}
	}

	return nil
}

// writeThickDefPerAngle 将每个角度的皮肤厚度定义写入 w
// writeThickDefPerAngle writes the skin-thickness definition for one angle to w
func writeThickDefPerAngle(writer *stream.BinaryWriter, angleDef *ThickDefPerAngle) error {
	// 写入角度
	// Write the angle
	if err := writer.WriteInt32(angleDef.AngleDegree); err != nil {
		return fmt.Errorf("failed to write angle degree: %w", err)
	}

	// 写入顶点索引
	// Write the vertex index
	if err := writer.WriteInt32(angleDef.VertexIndex); err != nil {
		return fmt.Errorf("failed to write vertex index: %w", err)
	}

	// 写入默认距离
	// Write the default distance
	if err := writer.WriteFloat32(angleDef.DefaultDistance); err != nil {
		return fmt.Errorf("failed to write default distance: %w", err)
	}

	return nil
}
