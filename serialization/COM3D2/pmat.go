package COM3D2

import (
	"fmt"
	"io"
	"strconv"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/utilities"
)

// CM3D2_PMATERIAL
// 渲染顺序文件
//
// 无版本差异

// PMat 对应 .PMat 文件结构
type PMat struct {
	Signature    string  `json:"Signature"`    // "CM3D2_PMATERIAL"
	Version      int32   `json:"Version"`      // 1000
	Hash         int32   `json:"Hash"`         // 哈希值，用于缓存控制
	MaterialName string  `json:"MaterialName"` // 材质名称
	RenderQueue  float32 `json:"RenderQueue"`  // 渲染顺序
	Shader       string  `json:"Shader"`       // 着色器名称
}

// ReadPMat 从 r 中读取一个 .PMat 文件，并解析为 PMat 结构。
func ReadPMat(r io.Reader) (*PMat, error) {
	p := &PMat{}

	// 1. signature
	sig, err := binaryio.ReadString(r)
	if err != nil {
		return nil, fmt.Errorf("read .PMat signature failed: %w", err)
	}
	//if sig != PMatSignature {
	//	return nil, fmt.Errorf("invalid .PMat signature: got %q, want \"CM3D2_PMATERIAL\"", sig)
	//}
	p.Signature = sig

	// 2. version (int32)
	ver, err := binaryio.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("read .PMat version failed: %w", err)
	}
	p.Version = ver

	// 3. hash (int32)
	h, err := binaryio.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("read .PMat hash failed: %w", err)
	}
	p.Hash = h

	// 4. materialName (string)
	matName, err := binaryio.ReadString(r)
	if err != nil {
		return nil, fmt.Errorf("read .PMat materialName failed: %w", err)
	}
	p.MaterialName = matName

	// 5. renderQueue (float32)
	rq, err := binaryio.ReadFloat32(r)
	if err != nil {
		return nil, fmt.Errorf("read .PMat renderQueue failed: %w", err)
	}
	p.RenderQueue = rq

	// 6. shader (string)
	// This field exists in the official files, but it's never read in the code.
	// Considering that some programs might not write to this field, so no error.
	shaderStr, err := binaryio.ReadString(r)
	if err != nil {
		shaderStr = ""
	}
	p.Shader = shaderStr

	return p, nil
}

// Dump 将 p 写出到 w 中，格式与 .PMat 兼容。
func (p *PMat) Dump(w io.Writer, calculateHash bool) error {
	// 1. signature
	if err := binaryio.WriteString(w, p.Signature); err != nil {
		return fmt.Errorf("write .PMat signature failed: %w", err)
	}

	// 2. version
	if err := binaryio.WriteInt32(w, p.Version); err != nil {
		return fmt.Errorf("write .PMat version failed: %w", err)
	}

	// 3. hash
	//  In short, this is a bad design; different versions of C# runtime may produce different hash codes.
	//	The game developers use `materialName.GetHashCode()` to query the cache, but the hashCode within the file is pre-written.
	//  We can't match C#'s String.GetHashCode() implementation, Furthermore, C# does not guarantee a stable hashcode.
	//	Which means that the cache may never be hit, especially when the game engine version changes (2.0 and 2.5).
	//  Furthermore, it's only 32 bits, which means a higher probability of collisions.
	//  Therefore, we use a standard algorithm to replace it.
	//  Even so, it's impossible for it to hit the cache.
	if calculateHash {
		materialNameHash, err := utilities.GetStringHashFNV1a(p.MaterialName + p.Shader + strconv.FormatFloat(float64(p.RenderQueue), 'f', -1, 32))
		if err != nil {
			return fmt.Errorf("write .PMat hash failed: %w", err)
		}
		if err := binaryio.WriteInt32(w, materialNameHash); err != nil {
			return fmt.Errorf("write .PMat hash failed: %w", err)
		}
	} else {
		if err := binaryio.WriteInt32(w, p.Hash); err != nil {
			return fmt.Errorf("write .PMat hash failed: %w", err)
		}
	}

	// 4. materialName
	if err := binaryio.WriteString(w, p.MaterialName); err != nil {
		return fmt.Errorf("write .PMat materialName failed: %w", err)
	}

	// 5. renderQueue
	if err := binaryio.WriteFloat32(w, p.RenderQueue); err != nil {
		return fmt.Errorf("write .PMat renderQueue failed: %w", err)
	}

	// 6. shader
	// The official file contains this field, but I didn't see it been read.
	// Since it's in the official file, we'll write it as well.
	if err := binaryio.WriteString(w, p.Shader); err != nil {
		return fmt.Errorf("write .PMat shader failed: %w", err)
	}

	return nil
}
