package aba

import (
	"encoding/hex"

	"golang.org/x/crypto/md4"
)

// SerializedFileName 返回 AssetBundle 基名对应的 UnityFS 目录条目名
// Unity 把包内唯一的 SerializedFile 命名为 CAB- 加上基名的 MD4 十六进制摘要，官方 KCES 样本与 Unity 官方测试包都严格符合该规则
// SerializedFileName returns the UnityFS directory-entry name for an AssetBundle base name
// Unity names the single SerializedFile inside a bundle CAB- followed by the hexadecimal MD4 digest of the base name, and both official KCES samples and Unity's own test bundles follow that rule exactly
func SerializedFileName(assetBundleName string) string {
	digest := md4.New()
	_, _ = digest.Write([]byte(assetBundleName))
	return "CAB-" + hex.EncodeToString(digest.Sum(nil))
}
