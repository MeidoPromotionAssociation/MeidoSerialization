package COM3D2

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
)

// CM3D2_MATERIAL
// 材质文件
//
// 版本差异不体现在版本号上，容器结构也没有变化
// COM3D2_5 新增了一些属性，具体类型见 propertyRegistry
// CM3D2_MATERIAL
// Material file
//
// Version differences are not represented by the version number and do not change the container structure
// COM3D2_5 adds several properties whose concrete types are listed in propertyRegistry

// Mate 对应 .mate 文件的整体结构
// Mate corresponds to the complete structure of a .mate file
type Mate struct {
	Signature string    `json:"Signature"` // 文件签名，固定为 CM3D2_MATERIAL / File signature fixed to CM3D2_MATERIAL
	Version   int32     `json:"Version"`   // 编译器写入但游戏材质读取逻辑未使用的版本值 / Version written by the compiler but unused by the game material reader
	Name      string    `json:"Name"`      // 编译时记录的容器名称，游戏 LoadMaterial 读取后未使用 / Container name recorded at compile time and read but unused by LoadMaterial
	Material  *Material `json:"Material"`  // 内嵌材质记录 / Embedded material record
}

// ReadMate 从 r 中读取一个 .mate 文件并返回 Mate 结构
// ReadMate reads one .mate file from r and returns its Mate structure
func ReadMate(r io.Reader) (*Mate, error) {
	rp, ok := r.(binaryio.Peeker)
	if !ok {
		return nil, fmt.Errorf("ReadMate: the reader is not peekable, wrap it with bufio.Reader first")
	}

	m := &Mate{}

	reader := stream.NewBinaryReader(rp)

	// 读取字符串签名
	// Read the string signature
	sig, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read .mate signature failed: %w", err)
	}
	if sig != MateSignature {
		return nil, fmt.Errorf("invalid .mate signature: got %q, want %q", sig, MateSignature)
	}
	m.Signature = sig

	// 读取 Int32 版本
	// Read the Int32 version
	ver, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read .mate version failed: %w", err)
	}
	m.Version = ver

	// 读取外层名称
	// Read the outer name
	nameStr, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read .mate name failed: %w", err)
	}
	m.Name = nameStr

	// 读取 Material
	// Read the Material
	mat, err := readMaterial(reader)
	if err != nil {
		return nil, fmt.Errorf("read .mate material failed: %w", err)
	}
	m.Material = mat

	if trailing, peekErr := rp.Peek(1); peekErr == nil {
		return nil, fmt.Errorf("read .mate: trailing data after material EndTag (next byte %#x)", trailing[0])
	} else if peekErr != io.EOF {
		return nil, fmt.Errorf("read .mate: check trailing data failed: %w", peekErr)
	}

	return m, nil
}

// Dump 将 Mate 写出到 w 中
// Dump writes Mate to w
func (m *Mate) Dump(w io.Writer) error {
	if m == nil {
		return fmt.Errorf("write .mate failed: mate is nil")
	}
	if m.Signature != MateSignature {
		return fmt.Errorf("write .mate signature failed: got %q, want %q", m.Signature, MateSignature)
	}
	if m.Material == nil {
		return fmt.Errorf("write .mate material failed: material is nil")
	}
	if err := validateMaterialForDump(".mate Material", m.Material); err != nil {
		return err
	}

	writer := stream.NewBinaryWriter(w)

	// 写出字符串签名
	// Write the string signature
	if err := writer.WriteString(m.Signature); err != nil {
		return fmt.Errorf("write .mate signature failed: %w", err)
	}

	// 写出 Int32 版本
	// Write the Int32 version
	if err := writer.WriteInt32(m.Version); err != nil {
		return fmt.Errorf("write .mate version failed: %w", err)
	}

	// 写出外层名称
	// Write the outer name
	if err := writer.WriteString(m.Name); err != nil {
		return fmt.Errorf("write .mate name failed: %w", err)
	}

	// 写出 Material
	// Write the Material
	if err := m.Material.Dump(writer); err != nil {
		return fmt.Errorf("write .mate material failed: %w", err)
	}

	return nil
}

// Material 及其属性解析
// Material and its property parsing
type Material struct {
	Name           string     `json:"Name"`           // 赋给 Unity Material.name 的材质名称 / Material name assigned to Unity Material.name
	ShaderName     string     `json:"ShaderName"`     // 用于查找或切换 Unity Shader 的名称 / Name used to find or switch the Unity Shader
	ShaderFilename string     `json:"ShaderFilename"` // 用于从 DefMaterial 加载模板材质的资源名 / Resource name used to load the template material from DefMaterial
	Properties     []Property `json:"Properties"`     // 由 end 字符串终止的材质属性 / Material properties terminated by the end string
}

// readMaterial 用于解析 Material 结构
// readMaterial parses a Material structure
func readMaterial(reader *stream.BinaryReader) (*Material, error) {
	_, ok := reader.R.(binaryio.Peeker)
	if !ok {
		return nil, fmt.Errorf("readMaterial: the reader is not peekable, wrap it with bufio.Reader first")
	}

	m := &Material{}

	// 读取材质名称
	// Read the material name
	nameStr, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read material.name failed: %w", err)
	}
	m.Name = nameStr

	// 读取着色器名称
	// Read the shader name
	shaderName, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read material.shaderName failed: %w", err)
	}
	m.ShaderName = shaderName

	// 读取默认材质资源名，原字段名为 shaderFilename
	// Read the default material resource name stored in the shaderFilename field
	shaderFile, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read material.shaderFilename failed: %w", err)
	}
	m.ShaderFilename = shaderFile

	// 循环读取属性直到遇到 EndTag
	// Read properties in a loop until EndTag is encountered
	props := make([]Property, 0)
	for {
		peek, err := reader.PeekString()
		if err != nil {
			return nil, fmt.Errorf("peek property type failed: %w", err)
		}

		if peek == EndTag {
			// 消费 EndTag
			// Consume EndTag
			_, _ = reader.ReadString()
			break
		}
		// 根据类型创建不同的 Property
		// Create a different Property according to its type
		prop, err := readProperty(reader)
		if err != nil {
			return nil, err
		}
		props = append(props, prop)
	}

	m.Properties = props

	return m, nil
}

// Dump 将 Material 写出到 w 中
// Dump writes Material to w
func (m *Material) Dump(writer *stream.BinaryWriter) error {
	if err := validateMaterialForDump("Material", m); err != nil {
		return err
	}
	// 写出材质名称
	// Write the material name
	if err := writer.WriteString(m.Name); err != nil {
		return fmt.Errorf("write material.name failed: %w", err)
	}

	// 写出着色器名称
	// Write the shader name
	if err := writer.WriteString(m.ShaderName); err != nil {
		return fmt.Errorf("write material.shaderName failed: %w", err)
	}

	// 写出默认材质资源名，原字段名为 shaderFilename
	// Write the default material resource name stored in the shaderFilename field
	if err := writer.WriteString(m.ShaderFilename); err != nil {
		return fmt.Errorf("write material.shaderFilename failed: %w", err)
	}

	// 写出属性列表
	// Write the property list
	for _, prop := range m.Properties {
		if err := dumpProperty(writer, prop); err != nil {
			return fmt.Errorf("write material property failed: %w", err)
		}
	}

	// 最后写出 EndTag 表示属性列表结束
	// Finally write EndTag to mark the end of the property list
	if err := writer.WriteString(EndTag); err != nil {
		return fmt.Errorf("write properties %s failed: %w", EndTag, err)
	}

	return nil
}

// validateMaterialForDump 检查材质属性具体类型、纹理子类型和集合数量能否完整写出
// validateMaterialForDump checks concrete material property types, texture subtypes, and collection counts before writing
func validateMaterialForDump(path string, material *Material) error {
	if material == nil {
		return fmt.Errorf("%s is nil", path)
	}
	for index, property := range material.Properties {
		propertyPath := fmt.Sprintf("%s.Properties[%d]", path, index)
		switch value := property.(type) {
		case *TexProperty:
			if value == nil {
				return fmt.Errorf("%s is nil", propertyPath)
			}
			switch value.SubTag {
			case "tex2d", "cube":
				if value.Tex2D == nil {
					return fmt.Errorf("%s has SubTag %q but Tex2D is nil", propertyPath, value.SubTag)
				}
			case "texRT":
				if value.TexRT == nil {
					return fmt.Errorf("%s has SubTag %q but TexRT is nil", propertyPath, value.SubTag)
				}
			case "null":
			default:
				return fmt.Errorf("%s has unknown TexProperty SubTag %q", propertyPath, value.SubTag)
			}
		case *ColProperty:
			if value == nil {
				return fmt.Errorf("%s is nil", propertyPath)
			}
		case *VecProperty:
			if value == nil {
				return fmt.Errorf("%s is nil", propertyPath)
			}
		case *FProperty:
			if value == nil {
				return fmt.Errorf("%s is nil", propertyPath)
			}
		case *RangeProperty:
			if value == nil {
				return fmt.Errorf("%s is nil", propertyPath)
			}
		case *TexOffsetProperty:
			if value == nil {
				return fmt.Errorf("%s is nil", propertyPath)
			}
		case *TexScaleProperty:
			if value == nil {
				return fmt.Errorf("%s is nil", propertyPath)
			}
		case *KeywordProperty:
			if value == nil {
				return fmt.Errorf("%s is nil", propertyPath)
			}
			if _, err := collectionCountInt32(propertyPath+" keyword count", len(value.Keywords)); err != nil {
				return err
			}
		case nil:
			return fmt.Errorf("%s is nil", propertyPath)
		default:
			return fmt.Errorf("%s has unsupported type %T", propertyPath, property)
		}
	}
	return nil
}

// Property 是对应 C# 抽象 Property 类的接口
// Go 使用接口和具体结构体表达这些类型
// 每个具体结构体故意保存 TypeName，否则前端难以推断接口值的具体类型
// Property is the interface corresponding to the abstract C# Property class
// Go expresses these types through an interface and concrete structs
// Each concrete struct deliberately stores TypeName because otherwise a frontend cannot easily infer the concrete interface value
type Property interface {
	// GetTypeName 返回线格式中的属性类型标识
	// GetTypeName returns the property type identifier used on the wire
	GetTypeName() string
	// Read 读取外层类型标识之后的属性载荷
	// Read reads the property payload after its outer type identifier
	Read(reader *stream.BinaryReader) error
	// Write 写入属性类型标识和载荷
	// Write writes the property type identifier and payload
	Write(writer *stream.BinaryWriter) error
}

// PropertyCreator 定义属性创建器类型
// PropertyCreator defines the property-creator type
type PropertyCreator func() Property

// propertyRegistry 是属性类型注册表
// propertyRegistry is the property-type registry
var propertyRegistry = map[string]PropertyCreator{
	"tex": func() Property { return &TexProperty{} },
	"col": func() Property { return &ColProperty{} },
	"vec": func() Property { return &VecProperty{} },
	"f":   func() Property { return &FProperty{} },
	// 下列属性仅由 COM3D2_5 及后续实现支持
	// The following properties are supported only by COM3D2_5 and later implementations
	"range":      func() Property { return &RangeProperty{} },
	"tex_offset": func() Property { return &TexOffsetProperty{} },
	"tex_scale":  func() Property { return &TexScaleProperty{} },
	"keyword":    func() Property { return &KeywordProperty{} },
}

// readProperty 根据下一段内容解析 Property 的具体子类型
// readProperty parses the concrete Property subtype from the next section
func readProperty(reader *stream.BinaryReader) (Property, error) {
	// 先读出 tex、col、vec、f 等属性类型标识
	// First read the property type identifier such as tex, col, vec, or f
	typeStr, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read property type failed: %w", err)
	}

	// 通过注册表创建属性实例
	// Create the property instance through the registry
	creator, ok := propertyRegistry[typeStr]
	if !ok {
		return nil, fmt.Errorf("unknown property type: %q", typeStr)
	}
	prop := creator()

	// 调用具体类型的反序列化方法
	// Call the deserializer of the concrete type
	if err := prop.Read(reader); err != nil {
		return nil, fmt.Errorf("read %s property failed: %w", typeStr, err)
	}
	return prop, nil
}

// dumpProperty 根据 Property 的子类型写出对应数据
// dumpProperty writes data for the concrete Property subtype
func dumpProperty(writer *stream.BinaryWriter, prop Property) error {
	return prop.Write(writer)
}

// -------------------------------------------------------------------
// 第一类属性是 TexProperty
// The first property type is TexProperty

// TexProperty 表示 tex 属性及其 tex2d、cube、texRT 或 null 子载荷
// TexProperty represents a tex property and its tex2d, cube, texRT, or null subpayload
type TexProperty struct {
	TypeName string            `json:"TypeName" default:"tex"` // JSON 类型判别标识 / JSON type discriminator
	PropName string            `json:"PropName"`               // Unity 材质纹理属性名 / Unity material texture-property name
	SubTag   string            `json:"SubTag"`                 // 纹理子类型标识 / Texture subtype identifier
	Tex2D    *Tex2DSubProperty `json:"Tex2D"`                  // tex2d 或 cube 子载荷 / tex2d or cube subpayload
	TexRT    *TexRTSubProperty `json:"TexRT"`                  // texRT 子载荷 / texRT subpayload
}

// Tex2DSubProperty 保存纹理资源名、未使用字符串、偏移和缩放
// Tex2DSubProperty stores the texture resource name, an unused string, offset, and scale
type Tex2DSubProperty struct {
	Name   string     `json:"Name"`   // 用于加载 <Name>.tex 的纹理资源名 / Texture resource name used to load <Name>.tex
	Path   string     `json:"Path"`   // 游戏读取后未使用的第二个字符串 / Second string read but unused by the game
	Offset [2]float32 `json:"Offset"` // 纹理 X、Y 偏移 / Texture X and Y offset
	Scale  [2]float32 `json:"Scale"`  // 纹理 X、Y 缩放 / Texture X and Y scale
}

// TexRTSubProperty 保存 texRT 在线格式中由游戏读取后丢弃的两个字符串
// TexRTSubProperty stores the two texRT wire strings that the game reads and discards
type TexRTSubProperty struct {
	DiscardedStr1 string `json:"DiscardedStr1"` // 游戏未使用的第一个字符串 / First string unused by the game
	DiscardedStr2 string `json:"DiscardedStr2"` // 游戏未使用的第二个字符串 / Second string unused by the game
}

// GetTypeName 返回纹理属性标识 tex
// GetTypeName returns the tex property identifier
func (t *TexProperty) GetTypeName() string { return "tex" }

// Read 读取纹理属性名、子类型及该子类型对应的载荷
// Read reads the texture property name, subtype, and subtype-specific payload
func (t *TexProperty) Read(reader *stream.BinaryReader) error {
	t.TypeName = t.GetTypeName()

	// 读取字符串属性名
	// Read the string property name
	name, err := reader.ReadString()
	if err != nil {
		return fmt.Errorf("read property name failed: %w", err)
	}
	t.PropName = name

	// 读取 tex2d、cube、texRT 或 null 子标签
	// Read the tex2d, cube, texRT, or null subtag
	subTag, err := reader.ReadString()

	if err != nil {
		return fmt.Errorf("read TexProperty subtag failed: %w", err)
	}
	t.SubTag = subTag

	switch subTag {
	case "tex2d", "cube":
		// 解析 Tex2DSubProperty
		// Parse Tex2DSubProperty
		var tex2d Tex2DSubProperty

		// 读取纹理名称
		// Read the texture name
		s1, err := reader.ReadString()
		if err != nil {
			return fmt.Errorf("read tex2d.name failed: %w", err)
		}
		tex2d.Name = s1

		// 读取路径字符串
		// Read the path string
		s2, err := reader.ReadString()
		if err != nil {
			return fmt.Errorf("read tex2d.path failed: %w", err)
		}
		tex2d.Path = s2

		// 读取 Float2 偏移
		// Read the Float2 offset
		offsetX, err := reader.ReadFloat32()
		if err != nil {
			return fmt.Errorf("read tex2d.offset.x failed: %w", err)
		}
		offsetY, err := reader.ReadFloat32()
		if err != nil {
			return fmt.Errorf("read tex2d.offset.y failed: %w", err)
		}
		tex2d.Offset[0] = offsetX
		tex2d.Offset[1] = offsetY

		// 读取 Float2 缩放
		// Read the Float2 scale
		scaleX, err := reader.ReadFloat32()
		if err != nil {
			return fmt.Errorf("read tex2d.scale.x failed: %w", err)
		}
		scaleY, err := reader.ReadFloat32()
		if err != nil {
			return fmt.Errorf("read tex2d.scale.y failed: %w", err)
		}
		tex2d.Scale[0] = scaleX
		tex2d.Scale[1] = scaleY

		t.Tex2D = &tex2d

	case "texRT":
		// 解析 TexRTSubProperty
		// Parse TexRTSubProperty
		var texRT TexRTSubProperty

		s1, err := reader.ReadString()
		if err != nil {
			return fmt.Errorf("read texRT.discardedStr1 failed: %w", err)
		}
		s2, err := reader.ReadString()
		if err != nil {
			return fmt.Errorf("read texRT.discardedStr2 failed: %w", err)
		}
		texRT.DiscardedStr1 = s1
		texRT.DiscardedStr2 = s2
		t.TexRT = &texRT

	case "null":
		// 将 null 当作空 tex2d
		// Treat null as an empty tex2d value
		var tex2d Tex2DSubProperty
		t.Tex2D = &tex2d

	default:
		return fmt.Errorf("unknown TexProperty subTag: %q", subTag)
	}

	return nil
}

// Write 写入 tex 类型标识、属性名、子类型及其对应载荷
// Write writes the tex type identifier, property name, subtype, and subtype-specific payload
func (t *TexProperty) Write(writer *stream.BinaryWriter) error {
	// 写出 tex 类型标识
	// Write the tex type identifier
	if err := writer.WriteString(t.GetTypeName()); err != nil {
		return fmt.Errorf("write TexProperty type failed: %w", err)
	}
	// 写出属性名
	// Write the property name
	if err := writer.WriteString(t.PropName); err != nil {
		return fmt.Errorf("write TexProperty name failed: %w", err)
	}
	// 写出 tex2d、cube、texRT 或 null 子标签
	// Write the tex2d, cube, texRT, or null subtag
	if err := writer.WriteString(t.SubTag); err != nil {
		return fmt.Errorf("write TexProperty subTag failed: %w", err)
	}
	// 根据子标签写出不同内容
	// Write different content according to the subtag
	switch t.SubTag {
	case "tex2d", "cube":
		if t.Tex2D == nil {
			return fmt.Errorf("TexProperty with subTag '%s' but Tex2D is nil", t.SubTag)
		}
		// 写出 Tex2DSubProperty
		// Write Tex2DSubProperty
		if err := writer.WriteString(t.Tex2D.Name); err != nil {
			return fmt.Errorf("write tex2d.name failed: %w", err)
		}
		if err := writer.WriteString(t.Tex2D.Path); err != nil {
			return fmt.Errorf("write tex2d.path failed: %w", err)
		}
		if err := writer.WriteFloat32(t.Tex2D.Offset[0]); err != nil {
			return fmt.Errorf("write tex2d.offset.x failed: %w", err)
		}
		if err := writer.WriteFloat32(t.Tex2D.Offset[1]); err != nil {
			return fmt.Errorf("write tex2d.offset.y failed: %w", err)
		}
		if err := writer.WriteFloat32(t.Tex2D.Scale[0]); err != nil {
			return fmt.Errorf("write tex2d.scale.x failed: %w", err)
		}
		if err := writer.WriteFloat32(t.Tex2D.Scale[1]); err != nil {
			return fmt.Errorf("write tex2d.scale.y failed: %w", err)
		}
	case "texRT":
		if t.TexRT == nil {
			return fmt.Errorf("TexProperty with subTag 'texRT' but TexRT is nil")
		}
		// 写出 TexRTSubProperty
		// Write TexRTSubProperty
		if err := writer.WriteString(t.TexRT.DiscardedStr1); err != nil {
			return fmt.Errorf("write texRT.discardedStr1 failed: %w", err)
		}
		if err := writer.WriteString(t.TexRT.DiscardedStr2); err != nil {
			return fmt.Errorf("write texRT.discardedStr2 failed: %w", err)
		}
	case "null":
		// null 子标签之后不再写入内容
		// No content follows a null subtag
	default:
		return fmt.Errorf("unknown TexProperty subTag: %q", t.SubTag)
	}
	return nil
}

// -------------------------------------------------------------------
// 第二类属性是标识为 col 的 ColProperty
// The second property type is ColProperty identified by col

// ColProperty 表示按 R、G、B、A 顺序保存的材质颜色属性
// ColProperty represents a material color property stored in R, G, B, A order
type ColProperty struct {
	TypeName string     `json:"TypeName" default:"col"` // JSON 类型判别标识 / JSON type discriminator
	PropName string     `json:"PropName"`               // Unity 材质颜色属性名 / Unity material color-property name
	Color    [4]float32 `json:"Color"`                  // R、G、B、A 分量 / R, G, B, A components
}

// GetTypeName 返回颜色属性标识 col
// GetTypeName returns the col property identifier
func (c *ColProperty) GetTypeName() string { return "col" }

// Read 读取颜色属性名和四个 RGBA 浮点分量
// Read reads the color property name and four RGBA float components
func (c *ColProperty) Read(reader *stream.BinaryReader) error {
	c.TypeName = c.GetTypeName()

	// 读取字符串属性名
	// Read the string property name
	name, err := reader.ReadString()
	if err != nil {
		return fmt.Errorf("read property name failed: %w", err)
	}
	c.PropName = name

	// 读取四个 float32 分量
	// Read four float32 components
	for i := 0; i < 4; i++ {
		c.Color[i], err = reader.ReadFloat32()
		if err != nil {
			return fmt.Errorf("read ColProperty color[%d] failed: %w", i, err)
		}
	}

	return nil
}

// Write 写入 col 类型标识、属性名和四个 RGBA 浮点分量
// Write writes the col type identifier, property name, and four RGBA float components
func (c *ColProperty) Write(writer *stream.BinaryWriter) error {
	// 写出 col 类型标识
	// Write the col type identifier
	if err := writer.WriteString(c.GetTypeName()); err != nil {
		return fmt.Errorf("write ColProperty type failed: %w", err)
	}
	// 写出属性名
	// Write the property name
	if err := writer.WriteString(c.PropName); err != nil {
		return fmt.Errorf("write ColProperty name failed: %w", err)
	}
	// 写出四个 RGBA float32 分量
	// Write four RGBA float32 components
	for i, c := range c.Color {
		if err := writer.WriteFloat32(c); err != nil {
			return fmt.Errorf("write ColProperty color[%d] failed: %w", i, err)
		}
	}
	return nil
}

// -------------------------------------------------------------------
// 第三类属性是标识为 vec 的 VecProperty
// The third property type is VecProperty identified by vec

// VecProperty 表示按 X、Y、Z、W 顺序保存的材质 Vector4 属性
// VecProperty represents a material Vector4 property stored in X, Y, Z, W order
type VecProperty struct {
	TypeName string     `json:"TypeName" default:"vec"` // JSON 类型判别标识 / JSON type discriminator
	PropName string     `json:"PropName"`               // Unity 材质向量属性名 / Unity material vector-property name
	Vector   [4]float32 `json:"Vector"`                 // X、Y、Z、W 分量 / X, Y, Z, W components
}

// GetTypeName 返回向量属性标识 vec
// GetTypeName returns the vec property identifier
func (v *VecProperty) GetTypeName() string { return "vec" }

// Read 读取向量属性名和四个 Vector4 浮点分量
// Read reads the vector property name and four Vector4 float components
func (v *VecProperty) Read(reader *stream.BinaryReader) error {
	v.TypeName = v.GetTypeName()

	// 读取字符串属性名
	// Read the string property name
	name, err := reader.ReadString()
	if err != nil {
		return fmt.Errorf("read property name failed: %w", err)
	}
	v.PropName = name

	// 读取四个 float32 分量
	// Read four float32 components
	for i := 0; i < 4; i++ {
		v.Vector[i], err = reader.ReadFloat32()
		if err != nil {
			return fmt.Errorf("read VecProperty vector[%d] failed: %w", i, err)
		}
	}

	return nil
}

// Write 写入 vec 类型标识、属性名和四个 Vector4 浮点分量
// Write writes the vec type identifier, property name, and four Vector4 float components
func (v *VecProperty) Write(writer *stream.BinaryWriter) error {
	// 写出 vec 类型标识
	// Write the vec type identifier
	if err := writer.WriteString(v.GetTypeName()); err != nil {
		return fmt.Errorf("write VecProperty type failed: %w", err)
	}
	// 写出属性名
	// Write the property name
	if err := writer.WriteString(v.PropName); err != nil {
		return fmt.Errorf("write VecProperty name failed: %w", err)
	}
	// 写出四个 float32 分量
	// Write four float32 components
	for i, v := range v.Vector {
		if err := writer.WriteFloat32(v); err != nil {
			return fmt.Errorf("write VecProperty vector[%d] failed: %w", i, err)
		}
	}
	return nil
}

// -------------------------------------------------------------------
// 第四类属性是标识为 f 的 FProperty
// The fourth property type is FProperty identified by f

// FProperty 表示经典 COM3D2 格式中的单浮点材质属性
// FProperty represents a single-float material property in the classic COM3D2 format
type FProperty struct {
	TypeName string  `json:"TypeName" default:"f"` // JSON 类型判别标识 / JSON type discriminator
	PropName string  `json:"PropName"`             // Unity 材质浮点属性名 / Unity material float-property name
	Number   float32 `json:"Number"`               // 传给 Material.SetFloat 的值 / Value passed to Material.SetFloat
}

// GetTypeName 返回浮点属性标识 f
// GetTypeName returns the f property identifier
func (f *FProperty) GetTypeName() string { return "f" }

// Read 读取浮点属性名和值
// Read reads the float property name and value
func (f *FProperty) Read(reader *stream.BinaryReader) error {
	f.TypeName = f.GetTypeName()
	// 读取字符串属性名
	// Read the string property name
	name, err := reader.ReadString()
	if err != nil {
		return fmt.Errorf("read property name failed: %w", err)
	}
	f.PropName = name

	// 读取一个 float32 值
	// Read one float32 value
	val, err := reader.ReadFloat32()
	if err != nil {
		return fmt.Errorf("read FProperty float failed: %w", err)
	}
	f.Number = val

	return nil
}

// Write 写入 f 类型标识、属性名和值
// Write writes the f type identifier, property name, and value
func (f *FProperty) Write(writer *stream.BinaryWriter) error {
	// 写出 f 类型标识
	// Write the f type identifier
	if err := writer.WriteString(f.GetTypeName()); err != nil {
		return fmt.Errorf("write FProperty type failed: %w", err)
	}
	// 写出属性名
	// Write the property name
	if err := writer.WriteString(f.PropName); err != nil {
		return fmt.Errorf("write FProperty name failed: %w", err)
	}
	// 写出一个 float32 值
	// Write one float32 value
	if err := writer.WriteFloat32(f.Number); err != nil {
		return fmt.Errorf("write FProperty float failed: %w", err)
	}
	return nil
}

// -------------------------------------------------------------------
// 第五类属性是标识为 range 的 RangeProperty，仅由 COM3D2_5 及后续实现支持
// The fifth property type is RangeProperty identified by range and supported only by COM3D2_5 and later implementations

// RangeProperty 表示 COM3D2_5 读取器支持的 range 单浮点材质属性
// RangeProperty represents the range single-float material property supported by the COM3D2_5 reader
type RangeProperty struct {
	TypeName string  `json:"TypeName" default:"range"` // JSON 类型判别标识 / JSON type discriminator
	PropName string  `json:"PropName"`                 // Unity 材质浮点属性名 / Unity material float-property name
	Number   float32 `json:"Number"`                   // 传给 Material.SetFloat 的值 / Value passed to Material.SetFloat
}

// GetTypeName 返回范围属性标识 range
// GetTypeName returns the range property identifier
func (ra *RangeProperty) GetTypeName() string { return "range" }

// Read 读取范围属性名和值
// Read reads the range property name and value
func (ra *RangeProperty) Read(reader *stream.BinaryReader) error {
	ra.TypeName = ra.GetTypeName()

	// 读取字符串属性名
	// Read the string property name
	name, err := reader.ReadString()
	if err != nil {
		return fmt.Errorf("read property name failed: %w", err)
	}
	ra.PropName = name

	// 读取一个 float32 值
	// Read one float32 value
	val, err := reader.ReadFloat32()
	if err != nil {
		return fmt.Errorf("read RangeProperty float failed: %w", err)
	}
	ra.Number = val

	return nil
}

// Write 写入 range 类型标识、属性名和值
// Write writes the range type identifier, property name, and value
func (ra *RangeProperty) Write(writer *stream.BinaryWriter) error {
	// 写出 range 类型标识
	// Write the range type identifier
	if err := writer.WriteString(ra.GetTypeName()); err != nil {
		return fmt.Errorf("write RangeProperty type failed: %w", err)
	}
	// 写出属性名
	// Write the property name
	if err := writer.WriteString(ra.PropName); err != nil {
		return fmt.Errorf("write RangeProperty name failed: %w", err)
	}
	// 写出一个 float32 值
	// Write one float32 value
	if err := writer.WriteFloat32(ra.Number); err != nil {
		return fmt.Errorf("write RangeProperty float failed: %w", err)
	}
	return nil
}

// -------------------------------------------------------------------
// 第六类属性是标识为 tex_offset 的 TexOffsetProperty，仅由 COM3D2_5 及后续实现支持
// The sixth property type is TexOffsetProperty identified by tex_offset and supported only by COM3D2_5 and later implementations

// TexOffsetProperty 表示 COM3D2_5 读取器支持的独立纹理偏移属性
// TexOffsetProperty represents the standalone texture-offset property supported by the COM3D2_5 reader
type TexOffsetProperty struct {
	TypeName string  `json:"TypeName" default:"tex_offset"` // JSON 类型判别标识 / JSON type discriminator
	PropName string  `json:"PropName"`                      // Unity 材质纹理属性名 / Unity material texture-property name
	OffsetX  float32 `json:"OffsetX"`                       // 纹理 X 偏移 / Texture X offset
	OffsetY  float32 `json:"OffsetY"`                       // 纹理 Y 偏移 / Texture Y offset
}

// GetTypeName 返回纹理偏移属性标识 tex_offset
// GetTypeName returns the tex_offset property identifier
func (to *TexOffsetProperty) GetTypeName() string { return "tex_offset" }

// Read 读取纹理属性名和 X、Y 偏移
// Read reads the texture property name and X and Y offsets
func (to *TexOffsetProperty) Read(reader *stream.BinaryReader) error {
	to.TypeName = to.GetTypeName()

	// 读取字符串属性名
	// Read the string property name
	name, err := reader.ReadString()
	if err != nil {
		return fmt.Errorf("read property name failed: %w", err)
	}
	to.PropName = name

	// 读取两个 float32 偏移分量
	// Read two float32 offset components
	x, err := reader.ReadFloat32()
	if err != nil {
		return fmt.Errorf("read RangeProperty float x failed: %w", err)
	}
	to.OffsetX = x

	y, err := reader.ReadFloat32()
	if err != nil {
		return fmt.Errorf("read RangeProperty float y failed: %w", err)
	}
	to.OffsetY = y

	return nil
}

// Write 写入 tex_offset 类型标识、属性名和 X、Y 偏移
// Write writes the tex_offset type identifier, property name, and X and Y offsets
func (to *TexOffsetProperty) Write(writer *stream.BinaryWriter) error {
	// 写出 tex_offset 类型标识
	// Write the tex_offset type identifier
	if err := writer.WriteString(to.GetTypeName()); err != nil {
		return fmt.Errorf("write TexOffsetProperty type failed: %w", err)
	}
	// 写出属性名
	// Write the property name
	if err := writer.WriteString(to.PropName); err != nil {
		return fmt.Errorf("write TexOffsetProperty name failed: %w", err)
	}
	// 写出两个 float32 偏移分量
	// Write two float32 offset components
	if err := writer.WriteFloat32(to.OffsetX); err != nil {
		return fmt.Errorf("write TexOffsetProperty float x failed: %w", err)
	}

	if err := writer.WriteFloat32(to.OffsetY); err != nil {
		return fmt.Errorf("write TexOffsetProperty float y failed: %w", err)
	}
	return nil
}

// -------------------------------------------------------------------
// 第七类属性是标识为 tex_scale 的 TexScaleProperty，仅由 COM3D2_5 及后续实现支持
// The seventh property type is TexScaleProperty identified by tex_scale and supported only by COM3D2_5 and later implementations

// TexScaleProperty 表示 COM3D2_5 读取器支持的独立纹理缩放属性
// TexScaleProperty represents the standalone texture-scale property supported by the COM3D2_5 reader
type TexScaleProperty struct {
	TypeName string  `json:"TypeName" default:"tex_scale"` // JSON 类型判别标识 / JSON type discriminator
	PropName string  `json:"PropName"`                     // Unity 材质纹理属性名 / Unity material texture-property name
	ScaleX   float32 `json:"ScaleX"`                       // 纹理 X 缩放 / Texture X scale
	ScaleY   float32 `json:"ScaleY"`                       // 纹理 Y 缩放 / Texture Y scale
}

// GetTypeName 返回纹理缩放属性标识 tex_scale
// GetTypeName returns the tex_scale property identifier
func (ts *TexScaleProperty) GetTypeName() string { return "tex_scale" }

// Read 读取纹理属性名和 X、Y 缩放
// Read reads the texture property name and X and Y scales
func (ts *TexScaleProperty) Read(reader *stream.BinaryReader) error {
	ts.TypeName = ts.GetTypeName()
	// 读取字符串属性名
	// Read the string property name
	name, err := reader.ReadString()
	if err != nil {
		return fmt.Errorf("read property name failed: %w", err)
	}
	ts.PropName = name

	// 读取两个 float32 缩放分量
	// Read two float32 scale components
	x, err := reader.ReadFloat32()
	if err != nil {
		return fmt.Errorf("read RangeProperty float x failed: %w", err)
	}
	ts.ScaleX = x

	y, err := reader.ReadFloat32()
	if err != nil {
		return fmt.Errorf("read RangeProperty float y failed: %w", err)
	}
	ts.ScaleY = y

	return nil
}

// Write 写入 tex_scale 类型标识、属性名和 X、Y 缩放
// Write writes the tex_scale type identifier, property name, and X and Y scales
func (ts *TexScaleProperty) Write(writer *stream.BinaryWriter) error {
	// 写出 tex_scale 类型标识
	// Write the tex_scale type identifier
	if err := writer.WriteString(ts.GetTypeName()); err != nil {
		return fmt.Errorf("write TexOffsetProperty type failed: %w", err)
	}
	// 写出属性名
	// Write the property name
	if err := writer.WriteString(ts.PropName); err != nil {
		return fmt.Errorf("write TexOffsetProperty name failed: %w", err)
	}
	// 写出两个 float32 缩放分量
	// Write two float32 scale components
	if err := writer.WriteFloat32(ts.ScaleX); err != nil {
		return fmt.Errorf("write TexOffsetProperty float x failed: %w", err)
	}

	if err := writer.WriteFloat32(ts.ScaleY); err != nil {
		return fmt.Errorf("write TexOffsetProperty float y failed: %w", err)
	}
	return nil
}

// -------------------------------------------------------------------
// 第八类属性是标识为 keyword 的 KeywordProperty，仅由 COM3D2_5 及后续实现支持
// The eighth property type is KeywordProperty identified by keyword and supported only by COM3D2_5 and later implementations

// KeywordProperty 表示 COM3D2_5 读取器用于启用或禁用着色器关键字的属性
// KeywordProperty represents the COM3D2_5 property that enables or disables shader keywords
type KeywordProperty struct {
	TypeName string    `json:"TypeName" default:"keyword"` // JSON 类型判别标识 / JSON type discriminator
	PropName string    `json:"PropName"`                   // 在线格式中保存但游戏关键字分支未使用的属性名 / Property name stored on wire but unused by the game keyword branch
	Count    int32     `json:"Count"`                      // 在线格式记录的关键字数量 / Keyword count recorded on the wire
	Keywords []Keyword `json:"Keywords"`                   // 按文件顺序保存的关键字状态 / Keyword states in file order
}

// Keyword 保存一个着色器关键字及其启用状态
// Keyword stores one shader keyword and its enabled state
type Keyword struct {
	Key   string `json:"Key"`   // 着色器关键字 / Shader keyword
	Value bool   `json:"Value"` // 是否调用 EnableKeyword / Whether EnableKeyword is called
}

// GetTypeName 返回关键字属性标识 keyword
// GetTypeName returns the keyword property identifier
func (f *KeywordProperty) GetTypeName() string { return "keyword" }

// Read 读取属性名、关键字数量和每个启用状态
// Read reads the property name, keyword count, and each enabled state
func (f *KeywordProperty) Read(reader *stream.BinaryReader) error {
	f.TypeName = f.GetTypeName()

	// 读取字符串属性名
	// Read the string property name
	name, err := reader.ReadString()
	if err != nil {
		return fmt.Errorf("read property name failed: %w", err)
	}
	f.PropName = name

	// 读取 Int32 关键字数量
	// Read the Int32 keyword count
	count, err := reader.ReadInt32()
	if err != nil {
		return fmt.Errorf("read Keyword count failed: %w", err)
	}
	if err := validateNonNegativeCount("Keyword count", count); err != nil {
		return err
	}
	f.Count = count

	// 循环读取 count 个关键字
	// Read count keywords in a loop
	f.Keywords = makeCountedSliceForAppend[Keyword](count)
	for i := int32(0); i < count; i++ {
		key, err := reader.ReadString()
		if err != nil {
			return fmt.Errorf("read Keyword key failed: %w", err)
		}
		value, err := reader.ReadBool()
		if err != nil {
			return fmt.Errorf("read Keyword value failed: %w", err)
		}
		f.Keywords = append(f.Keywords, Keyword{
			Key:   key,
			Value: value,
		})
	}
	return nil
}

// Write 根据 Keywords 更新 Count 并写入 keyword 属性
// Write updates Count from Keywords and writes the keyword property
func (f *KeywordProperty) Write(writer *stream.BinaryWriter) error {
	count, err := collectionCountInt32("KeywordProperty keyword count", len(f.Keywords))
	if err != nil {
		return err
	}
	f.Count = count
	// 写出 keyword 类型标识
	// Write the keyword type identifier
	if err := writer.WriteString(f.GetTypeName()); err != nil {
		return fmt.Errorf("write KeywordProperty type failed: %w", err)
	}
	// 写出属性名
	// Write the property name
	if err := writer.WriteString(f.PropName); err != nil {
		return fmt.Errorf("write KeywordProperty name failed: %w", err)
	}
	// 写出关键字数量
	// Write the keyword count
	if err := writer.WriteInt32(count); err != nil {
		return fmt.Errorf("write KeywordProperty count failed: %w", err)
	}
	// 循环写出全部关键字
	// Write all keywords in a loop
	for i, kv := range f.Keywords {
		if err := writer.WriteString(kv.Key); err != nil {
			return fmt.Errorf("write Keywords[%d] key failed: %w", i, err)
		}
		if err := writer.WriteBool(kv.Value); err != nil {
			return fmt.Errorf("write Keywords[%d] value failed: %w", i, err)
		}
	}
	return nil
}

// printMaterialDetails 打印 Material 的详细信息
// 此函数用于调试
// printMaterialDetails prints detailed Material information
// This function is used for debugging
func printMaterialDetails(m *Material) {
	fmt.Printf("Material Name: %s\n", m.Name)
	fmt.Printf("Shader Name: %s\n", m.ShaderName)
	fmt.Printf("Shader Filename: %s\n", m.ShaderFilename)

	fmt.Println("Properties:")
	for _, prop := range m.Properties {
		fmt.Printf("  - Type: %s\n", prop.GetTypeName())
		// 根据不同属性类型打印具体值
		// Print concrete values according to the property type
		switch p := prop.(type) {
		case *TexProperty:
			fmt.Printf("    PropName: %s\n", p.PropName)
			fmt.Printf("    SubTag: %s\n", p.SubTag)
			if p.Tex2D != nil {
				fmt.Printf("    Tex2D Name: %s\n", p.Tex2D.Name)
				fmt.Printf("    Tex2D Path: %s\n", p.Tex2D.Path)
				fmt.Printf("    Tex2D Offset: %v\n", p.Tex2D.Offset)
				fmt.Printf("    Tex2D Scale: %v\n", p.Tex2D.Scale)
			}
			if p.TexRT != nil {
				fmt.Printf("    TexRT DiscardedStr1: %s\n", p.TexRT.DiscardedStr1)
				fmt.Printf("    TexRT DiscardedStr2: %s\n", p.TexRT.DiscardedStr2)
			}
		case *ColProperty:
			fmt.Printf("    PropName: %s\n", p.PropName)
			fmt.Printf("    Color: %v\n", p.Color)
		case *VecProperty:
			fmt.Printf("    PropName: %s\n", p.PropName)
			fmt.Printf("    Vector: %v\n", p.Vector)
		case *FProperty:
			fmt.Printf("    PropName: %s\n", p.PropName)
			fmt.Printf("    Number: %v\n", p.Number)
		case *RangeProperty:
			fmt.Printf("    PropName: %s\n", p.PropName)
			fmt.Printf("    Number: %v\n", p.Number)
		case *TexOffsetProperty:
			fmt.Printf("    PropName: %s\n", p.PropName)
			fmt.Printf("    OffsetX: %v\n", p.OffsetX)
			fmt.Printf("    OffsetY: %v\n", p.OffsetY)
		case *TexScaleProperty:
			fmt.Printf("    PropName: %s\n", p.PropName)
			fmt.Printf("    ScaleX: %v\n", p.ScaleX)
			fmt.Printf("    ScaleY: %v\n", p.ScaleY)
		case *KeywordProperty:
			fmt.Printf("    PropName: %s\n", p.PropName)
			fmt.Printf("    Count: %v\n", p.Count)
			for i, kw := range p.Keywords {
				fmt.Printf("      - Key[%d]: %s, Value: %v\n", i, kw.Key, kw.Value)
			}
		}
	}
}

// UnmarshalJSON 为 Material 实现自定义 JSON 反序列化
// Material.Properties 是接口切片，需要根据 TypeName 字段决定具体类型
// UnmarshalJSON implements custom JSON deserialization for Material
// Material.Properties is an interface slice whose concrete types are selected by TypeName
func (m *Material) UnmarshalJSON(data []byte) error {
	// 使用中间结构接收 Properties 原始数据，其他字段由 Alias 直接接收
	// Use an intermediate structure for raw Properties while Alias receives the other fields directly
	type Alias Material
	aux := &struct {
		Properties []json.RawMessage `json:"Properties"` // 未分派的属性 JSON / Undispatched property JSON
		*Alias                       // Material 的其余字段 / Remaining fields of Material
	}{
		Alias: (*Alias)(m),
	}

	// 先解析 Material 的普通字段
	// Parse ordinary Material fields first
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	// 逐个根据 TypeName 解析 Properties
	// Decode Properties one by one according to TypeName
	var props []Property
	for _, raw := range aux.Properties {
		// 使用临时结构提取 TypeName 字段
		// Use a temporary structure to extract TypeName
		var typeHolder struct {
			TypeName string `json:"TypeName"` // JSON 类型判别标识 / JSON type discriminator
		}
		if err := json.Unmarshal(raw, &typeHolder); err != nil {
			return err
		}
		switch typeHolder.TypeName {
		case "tex":
			var tp TexProperty
			if err := json.Unmarshal(raw, &tp); err != nil {
				return err
			}
			props = append(props, &tp)
		case "col":
			var cp ColProperty
			if err := json.Unmarshal(raw, &cp); err != nil {
				return err
			}
			props = append(props, &cp)
		case "vec":
			var vp VecProperty
			if err := json.Unmarshal(raw, &vp); err != nil {
				return err
			}
			props = append(props, &vp)
		case "f":
			var fp FProperty
			if err := json.Unmarshal(raw, &fp); err != nil {
				return err
			}
			props = append(props, &fp)
		case "range":
			var r RangeProperty
			if err := json.Unmarshal(raw, &r); err != nil {
				return err
			}
			props = append(props, &r)
		case "tex_offset":
			var t TexOffsetProperty
			if err := json.Unmarshal(raw, &t); err != nil {
				return err
			}
			props = append(props, &t)
		case "tex_scale":
			var s TexScaleProperty
			if err := json.Unmarshal(raw, &s); err != nil {
				return err
			}
			props = append(props, &s)
		case "keyword":
			var k KeywordProperty
			if err := json.Unmarshal(raw, &k); err != nil {
				return err
			}
			props = append(props, &k)
		default:
			return fmt.Errorf("unknown property type: %s", typeHolder.TypeName)
		}
	}
	m.Properties = props
	return nil
}
