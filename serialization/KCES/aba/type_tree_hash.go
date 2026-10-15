package aba

import (
	"encoding/binary"
	"hash"

	"golang.org/x/crypto/md4"
)

// typeHashAlignFlag 是参与 TypeHash 计算的唯一 MetaFlags 位，即 AlignBytes
// typeHashAlignFlag is the only MetaFlags bit participating in TypeHash computation, namely AlignBytes
const typeHashAlignFlag = 0x4000

// ComputeTypeHash 按 Unity 与 AssetsTools.NET 共用的算法从节点表推导 TypeHash
//
// 算法按节点顺序对每个节点依次喂入下列字节，最后取 MD4 摘要：
//
//	UTF8(类型名) ‖ UTF8(字段名) ‖ LE-Int32(ByteSize) ‖ LE-Int32(TypeFlags)
//	                                  ‖ LE-Int32(Version) ‖ LE-Int32(MetaFlags & 0x4000)
//
// 与 AssetsTools.NET 的 ClassDatabaseToTypeTree.ComputeHash 和 TemplateFieldToTypeTree.ComputeHash 一致，
// 因此内置类型的 TypeHash 可以实时推导，不必再从样本中抄录常量，节点表被改动后也不会留下过期哈希
// ComputeTypeHash derives a TypeTree's TypeHash from its node table with the algorithm shared by Unity and AssetsTools.NET
// It matches ClassDatabaseToTypeTree.ComputeHash and TemplateFieldToTypeTree.ComputeHash, so built-in TypeHashes can be derived
// instead of copied from samples, and an edited node table can no longer leave a stale hash behind
func (tt *TypeTreeType) ComputeTypeHash() [16]byte {
	if tt == nil {
		return [16]byte{}
	}
	digest := md4.New()
	for nodeIndex := range tt.Nodes {
		node := &tt.Nodes[nodeIndex]
		_, _ = digest.Write([]byte(tt.GetTypeTreeString(node, true)))
		_, _ = digest.Write([]byte(tt.GetTypeTreeString(node, false)))
		writeTypeHashInt32(digest, node.ByteSize)
		writeTypeHashInt32(digest, int32(node.TypeFlags))
		writeTypeHashInt32(digest, int32(node.Version))
		writeTypeHashInt32(digest, int32(node.MetaFlags&typeHashAlignFlag))
	}
	var sum [16]byte
	copy(sum[:], digest.Sum(nil))
	return sum
}

// writeTypeHashInt32 按小端补码写入一个 Int32，与 .NET BitConverter.GetBytes 的字节序一致
// writeTypeHashInt32 writes one Int32 little-endian in two's-complement form, matching the byte order of .NET BitConverter.GetBytes
func writeTypeHashInt32(digest hash.Hash, value int32) {
	var encoded [4]byte
	binary.LittleEndian.PutUint32(encoded[:], uint32(value))
	_, _ = digest.Write(encoded[:])
}
