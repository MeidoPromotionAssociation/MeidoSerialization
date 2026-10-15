package aba

import (
	"fmt"
	"math"
)

// ReadMMesh 读取 ClassID 43 Mesh 独立对象文件
// ReadMMesh reads a standalone ClassID 43 Mesh object file
func ReadMMesh(data []byte) (*NativeUnityObject, error) {
	return readNativeUnityObjectClass(data, ClassIDMesh, "Mesh")
}

// WriteMMesh 写入 ClassID 43 Mesh 独立对象文件
// WriteMMesh writes a standalone ClassID 43 Mesh object file
func WriteMMesh(object *NativeUnityObject) ([]byte, error) {
	return writeNativeUnityObjectClass(object, ClassIDMesh, "Mesh")
}

// ReadTexture2D 读取 ClassID 28 Texture2D 独立对象文件
// ReadTexture2D reads a standalone ClassID 28 Texture2D object file
func ReadTexture2D(data []byte) (*NativeUnityObject, error) {
	return readNativeUnityObjectClass(data, ClassIDTexture2D, "Texture2D")
}

// WriteTexture2D 写入 ClassID 28 Texture2D 独立对象文件
// WriteTexture2D writes a standalone ClassID 28 Texture2D object file
func WriteTexture2D(object *NativeUnityObject) ([]byte, error) {
	return writeNativeUnityObjectClass(object, ClassIDTexture2D, "Texture2D")
}

// ReadSpriteObject 读取 ClassID 213 Sprite 独立对象文件
// ReadSpriteObject reads a standalone ClassID 213 Sprite object file
func ReadSpriteObject(data []byte) (*NativeUnityObject, error) {
	return readNativeUnityObjectClass(data, ClassIDSprite, "Sprite")
}

// WriteSpriteObject 写入 ClassID 213 Sprite 独立对象文件
// WriteSpriteObject writes a standalone ClassID 213 Sprite object file
func WriteSpriteObject(object *NativeUnityObject) ([]byte, error) {
	return writeNativeUnityObjectClass(object, ClassIDSprite, "Sprite")
}

// ReadPartsAtlas 读取 ClassID 687078895 SpriteAtlas 独立对象文件
// ReadPartsAtlas reads a standalone ClassID 687078895 SpriteAtlas object file
func ReadPartsAtlas(data []byte) (*NativeUnityObject, error) {
	return readNativeUnityObjectClass(data, ClassIDSpriteAtlas, "SpriteAtlas")
}

// WritePartsAtlas 写入 ClassID 687078895 SpriteAtlas 独立对象文件
// WritePartsAtlas writes a standalone ClassID 687078895 SpriteAtlas object file
func WritePartsAtlas(object *NativeUnityObject) ([]byte, error) {
	return writeNativeUnityObjectClass(object, ClassIDSpriteAtlas, "SpriteAtlas")
}

// ReadAnimationClip 读取 ClassID 74 AnimationClip 独立对象文件
// ReadAnimationClip reads a standalone ClassID 74 AnimationClip object file
func ReadAnimationClip(data []byte) (*NativeUnityObject, error) {
	return readNativeUnityObjectClass(data, ClassIDAnimationClip, "AnimationClip")
}

// WriteAnimationClip 写入 ClassID 74 AnimationClip 独立对象文件
// WriteAnimationClip writes a standalone ClassID 74 AnimationClip object file
func WriteAnimationClip(object *NativeUnityObject) ([]byte, error) {
	return writeNativeUnityObjectClass(object, ClassIDAnimationClip, "AnimationClip")
}

// ReadMaterial 读取 ClassID 21 Material 独立对象文件
// ReadMaterial reads a standalone ClassID 21 Material object file
func ReadMaterial(data []byte) (*NativeUnityObject, error) {
	return readNativeUnityObjectClass(data, ClassIDMaterial, "Material")
}

// WriteMaterial 写入 ClassID 21 Material 独立对象文件
// WriteMaterial writes a standalone ClassID 21 Material object file
func WriteMaterial(object *NativeUnityObject) ([]byte, error) {
	return writeNativeUnityObjectClass(object, ClassIDMaterial, "Material")
}

// ReadAudioClip 读取 ClassID 83 AudioClip 独立对象文件
// ReadAudioClip reads a standalone ClassID 83 AudioClip object file
func ReadAudioClip(data []byte) (*NativeUnityObject, error) {
	return readNativeUnityObjectClass(data, ClassIDAudioClip, "AudioClip")
}

// WriteAudioClip 写入 ClassID 83 AudioClip 独立对象文件
// WriteAudioClip writes a standalone ClassID 83 AudioClip object file
func WriteAudioClip(object *NativeUnityObject) ([]byte, error) {
	return writeNativeUnityObjectClass(object, ClassIDAudioClip, "AudioClip")
}

// ReadMonoBehaviour 读取 ClassID 114 MonoBehaviour 独立对象文件
// ReadMonoBehaviour reads a standalone ClassID 114 MonoBehaviour object file
func ReadMonoBehaviour(data []byte) (*NativeUnityObject, error) {
	return readNativeUnityObjectClass(data, ClassIDMonoBehaviour, "MonoBehaviour")
}

// WriteMonoBehaviour 写入 ClassID 114 MonoBehaviour 独立对象文件
// WriteMonoBehaviour writes a standalone ClassID 114 MonoBehaviour object file
func WriteMonoBehaviour(object *NativeUnityObject) ([]byte, error) {
	return writeNativeUnityObjectClass(object, ClassIDMonoBehaviour, "MonoBehaviour")
}

// NewNativeTexture2DObject 从自上而下的 RGBA32 像素构建自带 Unity 2022.3 内置 TypeTree 的独立 Texture2D 对象，图像数据翻转为 Unity 自下而上行序后内联为单 mip RGBA32
// NewNativeTexture2DObject builds a standalone Texture2D object carrying the Unity 2022.3 built-in TypeTree from top-down RGBA32 pixels, inlining the image data as a single RGBA32 mip after flipping it into Unity's bottom-up row order
func NewNativeTexture2DObject(name string, width int64, height int64, rgba []byte) (*NativeUnityObject, error) {
	return NewNativeTexture2DObjectWithMips(name, width, height, [][]byte{rgba})
}

// NewNativeTexture2DObjectWithMips 从自上而下的 RGBA32 mip 链构建自带 Unity 2022.3 内置 TypeTree 的独立 Texture2D 对象，mips 第 0 级为全分辨率，逐级翻转后按 Unity 布局内联
// NewNativeTexture2DObjectWithMips builds a standalone Texture2D object carrying the Unity 2022.3 built-in TypeTree from a top-down RGBA32 mip chain, flipping every level of the chain whose level 0 is the full resolution and inlining them in the Unity layout
func NewNativeTexture2DObjectWithMips(name string, width int64, height int64, mips [][]byte) (*NativeUnityObject, error) {
	return NewNativeTexture2DObjectWithFormat(name, width, height, TextureFormatRGBA32, mips)
}

// NewNativeTexture2DObjectWithFormat 从自上而下的 mip 链构建自带 Unity 2022.3 内置 TypeTree 的独立 Texture2D 对象，format 可以是 RGBA32 或 DXT1、DXT5
// NewNativeTexture2DObjectWithFormat builds a standalone Texture2D object carrying the Unity 2022.3 built-in TypeTree from a top-down mip chain, where format can be RGBA32, DXT1, or DXT5
func NewNativeTexture2DObjectWithFormat(name string, width int64, height int64, format int32, mips [][]byte) (*NativeUnityObject, error) {
	flippedMips, err := FlipTextureMipChain(format, width, height, mips)
	if err != nil {
		return nil, err
	}
	data, err := encodeTexture2DData("2022.3.35f1", name, width, height, format, flippedMips)
	if err != nil {
		return nil, err
	}
	tree := unity2022Texture2DTypeTree()
	return &NativeUnityObject{ClassID: ClassIDTexture2D, BigEndian: false, TypeTree: tree, Data: data}, nil
}

// NativeTexture2DMipCount 读取独立 Texture2D 主文件声明的 mip 层数，字段缺失或非正值时按单层返回
// 该函数只解析元数据，不读取像素，因此对 m_StreamData 指向外部 sidecar 的贴图同样适用
// NativeTexture2DMipCount reads the mip level count declared by a standalone Texture2D primary file, falling back to a single level when the field is missing or not positive
// It parses metadata only and never reads pixels, so it also works for textures whose m_StreamData points into an external sidecar
func NativeTexture2DMipCount(data []byte) (int32, error) {
	object, err := ReadTexture2D(data)
	if err != nil {
		return 0, err
	}
	af, info, err := object.AssetsFileView()
	if err != nil {
		return 0, err
	}
	root, err := af.ReadAssetValue(info)
	if err != nil {
		return 0, err
	}
	mipCount, ok := root.Field("m_MipCount").Int64()
	if !ok || mipCount < 1 {
		return 1, nil
	}
	if mipCount > int64(math.MaxInt32) {
		return 0, fmt.Errorf("Texture2D m_MipCount %d exceeds Int32 range", mipCount)
	}
	return int32(mipCount), nil
}

// readNativeUnityObjectClass 读取独立对象并验证精确 ClassID
// readNativeUnityObjectClass reads a standalone object and validates its exact ClassID
func readNativeUnityObjectClass(data []byte, classID int32, typeName string) (*NativeUnityObject, error) {
	object, err := ReadNativeUnityObject(data)
	if err != nil {
		return nil, err
	}
	if object.ClassID != classID {
		return nil, fmt.Errorf("%s file contains ClassID %d instead of %d", typeName, object.ClassID, classID)
	}
	return object, nil
}

// writeNativeUnityObjectClass 验证精确 ClassID 后写入独立对象
// writeNativeUnityObjectClass validates the exact ClassID before writing a standalone object
func writeNativeUnityObjectClass(object *NativeUnityObject, classID int32, typeName string) ([]byte, error) {
	if object == nil {
		return nil, fmt.Errorf("nil %s object", typeName)
	}
	if object.ClassID != classID {
		return nil, fmt.Errorf("%s object has ClassID %d instead of %d", typeName, object.ClassID, classID)
	}
	return EncodeNativeUnityObject(object)
}
