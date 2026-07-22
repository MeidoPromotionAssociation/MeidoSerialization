package COM3D2

import (
	"fmt"
	"io"
	"strconv"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/utilities"
)

// CM3D2_PMATERIAL
// 渲染顺序文件
//
// 无版本差异
// 游戏的 ImportCM.TryGetPriorityMaterial 只读取到 RenderQueue，官方样本中的 Shader 尾随字符串不会被该游戏读取器消费
// 然而官方文件中有此字段，因此原样保留，猜测此字段用于计算哈希
// CM3D2_PMATERIAL
// Render-order file
//
// There are no version differences
// The game's ImportCM.TryGetPriorityMaterial reads only through RenderQueue and does not consume the trailing Shader string found in official samples
// However, this field exists in the official documentation, so it is retained as is, presumably for calculating the hash

// PMat 对应 .PMat 文件结构
// PMat corresponds to the structure of a .PMat file
type PMat struct {
	Signature    string  `json:"Signature"`    // CM3D2_PMATERIAL 文件签名 / CM3D2_PMATERIAL file signature
	Version      int32   `json:"Version"`      // 版本 1000 / Version 1000
	Hash         int32   `json:"Hash"`         // 哈希值，用于缓存控制 / Hash value used for cache control
	MaterialName string  `json:"MaterialName"` // 材质名称 / Material name
	RenderQueue  float32 `json:"RenderQueue"`  // 渲染顺序 / Render order
	Shader       string  `json:"Shader"`       // 官方样本中的着色器名称，游戏读取器不消费此字段 / Shader name found in official samples and not consumed by the game reader
}

// ReadPMat 从 r 中读取一个 .PMat 文件，并解析为 PMat 结构
// ReadPMat reads a .PMat file from r and parses it into a PMat structure
func ReadPMat(r io.Reader) (*PMat, error) {
	p := &PMat{}

	reader := stream.NewBinaryReader(r)

	// 1. 签名
	// 1. Signature
	sig, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read .PMat signature failed: %w", err)
	}
	// if sig != PMatSignature {
	// 	return nil, fmt.Errorf("invalid .PMat signature: got %q, want \"CM3D2_PMATERIAL\"", sig)
	// }
	p.Signature = sig

	// 2. 版本（int32）
	// 2. Version (int32)
	ver, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read .PMat version failed: %w", err)
	}
	p.Version = ver

	// 3. 哈希值（int32）
	// 3. Hash (int32)
	h, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read .PMat hash failed: %w", err)
	}
	p.Hash = h

	// 4. 材质名称（字符串）
	// 4. MaterialName (string)
	matName, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read .PMat materialName failed: %w", err)
	}
	p.MaterialName = matName

	// 5. 渲染顺序（float32）
	// 5. RenderQueue (float32)
	rq, err := reader.ReadFloat32()
	if err != nil {
		return nil, fmt.Errorf("read .PMat renderQueue failed: %w", err)
	}
	p.RenderQueue = rq

	// 6. shader (string)
	// 这是官方字段，但读取器允许旧文件或不完整文件在该字段之前结束
	// 下方写入器始终输出此字段
	// 6. shader (string)
	// This is an official field, but readers tolerate old or incomplete files that end before it
	// Writers always emit it below
	shaderStr, err := reader.ReadString()
	if err != nil {
		shaderStr = ""
	}
	p.Shader = shaderStr

	return p, nil
}

// Dump 将 p 写出到 w 中，格式与 .PMat 兼容
// Dump writes p to w in a format compatible with .PMat
func (p *PMat) Dump(w io.Writer, calculateHash bool) error {
	if p == nil {
		return fmt.Errorf("nil .PMat")
	}
	writer := stream.NewBinaryWriter(w)

	// 1. 签名
	// 1. Signature
	if err := writer.WriteString(p.Signature); err != nil {
		return fmt.Errorf("write .PMat signature failed: %w", err)
	}

	// 2. 版本
	// 2. Version
	if err := writer.WriteInt32(p.Version); err != nil {
		return fmt.Errorf("write .PMat version failed: %w", err)
	}

	// 3. hash
	// 简而言之，这是一个糟糕的设计，不同版本的 C# 运行时可能产生不同的哈希值
	// 游戏开发者使用 `materialName.GetHashCode()` 查询缓存，但文件中的 hashCode 是预先写入的
	// 我们无法匹配 C# 的 String.GetHashCode() 实现，而且 C# 不保证哈希值稳定
	// 这意味着缓存可能永远不会命中，尤其是在游戏引擎版本变化时，例如 2.0 和 2.5
	// 此外它只有 32 位，发生冲突的概率更高
	// 因此这里使用标准算法替代它
	// 即便如此，它仍不可能命中游戏缓存
	// 3. hash
	// In short, this is a bad design; different versions of C# runtime may produce different hash codes
	// The game developers use `materialName.GetHashCode()` to query the cache, but the hashCode within the file is pre-written
	// We can't match C#'s String.GetHashCode() implementation, Furthermore, C# does not guarantee a stable hashcode
	// Which means that the cache may never be hit, especially when the game engine version changes (2.0 and 2.5)
	// Furthermore, it's only 32 bits, which means a higher probability of collisions
	// Therefore, we use a standard algorithm to replace it
	// Even so, it's impossible for it to hit the cache
	hash := p.Hash
	if calculateHash {
		var err error
		hash, err = utilities.GetStringHashFNV1a(p.MaterialName + p.Shader + strconv.FormatFloat(float64(p.RenderQueue), 'f', -1, 32))
		if err != nil {
			return fmt.Errorf("calculate .PMat hash failed: %w", err)
		}
		p.Hash = hash
	}

	if err := writer.WriteInt32(hash); err != nil {
		return fmt.Errorf("write .PMat hash failed: %w", err)
	}

	// 4. 材质名称
	// 4. MaterialName
	if err := writer.WriteString(p.MaterialName); err != nil {
		return fmt.Errorf("write .PMat materialName failed: %w", err)
	}

	// 5. 渲染顺序
	// 5. RenderQueue
	if err := writer.WriteFloat32(p.RenderQueue); err != nil {
		return fmt.Errorf("write .PMat renderQueue failed: %w", err)
	}

	// 6. shader
	// 即使兼容读取器接受了较短的输入，输出也始终采用完整的官方布局
	// 6. shader
	// Even when a compatible reader accepted a short input, output is always the complete official layout
	if err := writer.WriteString(p.Shader); err != nil {
		return fmt.Errorf("write .PMat shader failed: %w", err)
	}

	return nil
}
