package aba

import "fmt"

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

// NewNativeTexture2DObject 从 RGBA32 像素构建自带 Unity 2022.3 内置 TypeTree 的独立 Texture2D 对象，图像数据内联为单 mip RGBA32
// NewNativeTexture2DObject builds a standalone Texture2D object carrying the Unity 2022.3 built-in TypeTree from RGBA32 pixels with the image data inlined as a single RGBA32 mip
func NewNativeTexture2DObject(name string, width int64, height int64, rgba []byte) (*NativeUnityObject, error) {
	data, err := encodeTexture2DData("2022.3.35f1", name, width, height, rgba)
	if err != nil {
		return nil, err
	}
	tree := unity2022Texture2DTypeTree()
	return &NativeUnityObject{ClassID: ClassIDTexture2D, BigEndian: false, TypeTree: tree, Data: data}, nil
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
