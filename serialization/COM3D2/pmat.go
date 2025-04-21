package COM3D2

import (
	"fmt"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/utilities"
	"io"
)

// CM3D2_PMATERIAL
// 无版本差异

// PMat 对应 .PMat 文件结构
type PMat struct {
	Signature    string  `json:"Signature"` // "CM3D2_PMATERIAL"
	Version      int32   `json:"Version"`   // 1000
	Hash         int32   `json:"Hash"`
	MaterialName string  `json:"MaterialName"`
	RenderQueue  float32 `json:"RenderQueue"`
	Shader       string  `json:"Shader"`
}

// ReadPMat 从 r 中读取一个 .PMat 文件，并解析为 PMat 结构。
func ReadPMat(r io.Reader) (*PMat, error) {
	p := &PMat{}

	// 1. signature
	sig, err := utilities.ReadString(r)
	if err != nil {
		return nil, fmt.Errorf("read .PMat signature failed: %w", err)
	}
	//if sig != PMatSignature {
	//	return nil, fmt.Errorf("invalid .PMat signature: got %q, want \"CM3D2_PMATERIAL\"", sig)
	//}
	p.Signature = sig

	// 2. version (int32)
	ver, err := utilities.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("read .PMat version failed: %w", err)
	}
	p.Version = ver

	// 3. hash (int32)
	h, err := utilities.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("read .PMat hash failed: %w", err)
	}
	p.Hash = h

	// 4. materialName (string)
	matName, err := utilities.ReadString(r)
	if err != nil {
		return nil, fmt.Errorf("read .PMat materialName failed: %w", err)
	}
	p.MaterialName = matName

	// 5. renderQueue (float32)
	rq, err := utilities.ReadFloat32(r)
	if err != nil {
		return nil, fmt.Errorf("read .PMat renderQueue failed: %w", err)
	}
	p.RenderQueue = rq

	// 6. shader (string)
	shaderStr, err := utilities.ReadString(r)
	if err != nil {
		return nil, fmt.Errorf("read .PMat shader failed: %w", err)
	}
	p.Shader = shaderStr

	return p, nil
}

// Dump 将 p 写出到 w 中，格式与 .PMat 兼容。
func (p *PMat) Dump(w io.Writer, calculateHash bool) error {
	// 1. signature
	if err := utilities.WriteString(w, p.Signature); err != nil {
		return fmt.Errorf("write .PMat signature failed: %w", err)
	}

	// 2. version
	if err := utilities.WriteInt32(w, p.Version); err != nil {
		return fmt.Errorf("write .PMat version failed: %w", err)
	}

	// 3. hash
	//  Unfortunately, we can't match C#'s HashCode implementation
	//  Therefore, files edited by this editor cannot share cache with files edited by other editors
	//  Because the game uses this Hash value to determine the cache key
	//  Even if their values are the same, the calculated hashes are different
	if calculateHash {
		materialNameHash := utilities.GetStringHashInt32(p.MaterialName + p.Shader)
		if err := utilities.WriteInt32(w, materialNameHash); err != nil {
			return fmt.Errorf("write .PMat hash failed: %w", err)
		}
	} else {
		if err := utilities.WriteInt32(w, p.Hash); err != nil {
			return fmt.Errorf("write .PMat hash failed: %w", err)
		}
	}

	// 4. materialName
	if err := utilities.WriteString(w, p.MaterialName); err != nil {
		return fmt.Errorf("write .PMat materialName failed: %w", err)
	}
	// 5. renderQueue
	if err := utilities.WriteFloat32(w, p.RenderQueue); err != nil {
		return fmt.Errorf("write .PMat renderQueue failed: %w", err)
	}
	// 6. shader
	if err := utilities.WriteString(w, p.Shader); err != nil {
		return fmt.Errorf("write .PMat shader failed: %w", err)
	}

	return nil
}
