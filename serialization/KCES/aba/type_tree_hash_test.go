package aba

import "testing"

// officialBuiltInTypeHashes 保存从官方 KCES 样本提取的 TypeHash 期望值
// 这些常量只用于校验 ComputeTypeHash 的推导结果，不再作为写出时的数据来源
// officialBuiltInTypeHashes holds the TypeHash expectations extracted from official KCES samples
// These constants only verify the output of ComputeTypeHash and are no longer the source of what gets written
var officialBuiltInTypeHashes = []struct {
	name   string
	build  func() (TypeTreeType, error)
	expect [16]byte
}{
	{
		name:   "Texture2D",
		build:  func() (TypeTreeType, error) { return unity2022Texture2DTypeTree(), nil },
		expect: [16]byte{0x31, 0xF6, 0x5A, 0xE3, 0x43, 0xF6, 0x71, 0xBF, 0x46, 0x2F, 0x15, 0xC1, 0xB4, 0x1E, 0x97, 0x68},
	},
	{
		name:   "TextAsset",
		build:  func() (TypeTreeType, error) { return unity2022TextAssetTypeTree(), nil },
		expect: [16]byte{0x48, 0x6B, 0xA4, 0xE1, 0x5D, 0xBD, 0x6A, 0xEA, 0x8A, 0xC1, 0xA0, 0x64, 0x30, 0x58, 0x89, 0xC8},
	},
	{
		name:   "AssetBundle",
		build:  func() (TypeTreeType, error) { return unity2022AssetBundleTypeTree(), nil },
		expect: [16]byte{0x97, 0xDA, 0x5F, 0x46, 0x88, 0xE4, 0x5A, 0x57, 0xC8, 0xB4, 0x2D, 0x4F, 0x42, 0x49, 0x72, 0x97},
	},
	{
		name:   "Mesh",
		build:  unity2022MeshTypeTree,
		expect: [16]byte{0xC1, 0x8F, 0xD5, 0x29, 0xF7, 0x1D, 0xBF, 0xC0, 0xF9, 0xB3, 0xCD, 0xBF, 0xB9, 0x8E, 0xA7, 0xCD},
	},
}

// TestComputeTypeHashMatchesOfficialSamples 校验内置类型树的 TypeHash 推导结果与官方样本逐字节一致
// 节点表一旦被改动，本测试会立即失败，因此硬编码的哈希常量可以退化为纯校验数据
// TestComputeTypeHashMatchesOfficialSamples verifies that derived TypeHashes match the official samples byte for byte
// Any edit to a node table fails this test immediately, which lets the hardcoded hashes degrade into pure verification data
func TestComputeTypeHashMatchesOfficialSamples(t *testing.T) {
	for _, testCase := range officialBuiltInTypeHashes {
		t.Run(testCase.name, func(t *testing.T) {
			tree, err := testCase.build()
			if err != nil {
				t.Fatalf("build %s tree: %v", testCase.name, err)
			}
			derived := tree.ComputeTypeHash()
			if derived != testCase.expect {
				t.Fatalf("derived TypeHash = %x, want official %x (nodes=%d)", derived, testCase.expect, len(tree.Nodes))
			}
			if tree.TypeHash != testCase.expect {
				t.Fatalf("tree carries TypeHash %x, want derived %x", tree.TypeHash, testCase.expect)
			}
		})
	}
}

// TestComputeTypeHashTracksNodeChanges 校验 TypeHash 确实由节点内容推导，而不是常量
// TestComputeTypeHashTracksNodeChanges verifies that the TypeHash is really derived from node content rather than being a constant
func TestComputeTypeHashTracksNodeChanges(t *testing.T) {
	baseline := unity2022TextAssetTypeTree()
	original := baseline.ComputeTypeHash()

	edited := cloneTypeTreeType(&baseline)
	edited.Nodes[3].MetaFlags ^= typeHashAlignFlag
	if edited.ComputeTypeHash() == original {
		t.Fatal("changing MetaFlags AlignBytes did not change the derived TypeHash")
	}

	edited = cloneTypeTreeType(&baseline)
	edited.Nodes[1].ByteSize++
	if edited.ComputeTypeHash() == original {
		t.Fatal("changing ByteSize did not change the derived TypeHash")
	}

	edited = cloneTypeTreeType(&baseline)
	edited.Nodes[0].Version++
	if edited.ComputeTypeHash() == original {
		t.Fatal("changing node Version did not change the derived TypeHash")
	}
}

// TestComputeTypeHashHandlesNilTree 校验空树返回全零哈希而不 panic
// TestComputeTypeHashHandlesNilTree verifies that a nil tree yields a zero hash instead of panicking
func TestComputeTypeHashHandlesNilTree(t *testing.T) {
	var tree *TypeTreeType
	if tree.ComputeTypeHash() != ([16]byte{}) {
		t.Fatal("nil tree produced a non-zero TypeHash")
	}
}
