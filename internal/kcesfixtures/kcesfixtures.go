package kcesfixtures

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
)

// TB 表示夹具辅助函数使用的 testing.T 子集 / TB is the subset of testing.T used by fixture helpers
type TB interface {
	// Helper 将当前调用函数标记为测试辅助函数
	// Helper marks the calling test function as a helper
	Helper()
	// TempDir 返回当前测试使用的临时目录
	// TempDir returns the temporary directory used by the current test
	TempDir() string
	// Fatalf 报告致命错误并停止当前测试
	// Fatalf reports a fatal error and stops the current test
	Fatalf(format string, args ...any)
	// Skipf 跳过当前测试并给出原因
	// Skipf skips the current test with a reason
	Skipf(format string, args ...any)
}

// AbaFilePath 返回 testdata/KCES 下样本 ABA 文件的路径
// AbaFilePath returns the path to a sample ABA file under testdata/KCES
func AbaFilePath(name string) string {
	return filepath.Join(repoRoot(), "testdata", "KCES", name)
}

// TextAssetBytes 从 abaName 中提取名为 assetName 的 TextAsset
// TextAssetBytes extracts a TextAsset named assetName from abaName
func TextAssetBytes(t TB, abaName string, assetName string) []byte {
	t.Helper()
	data, err := readTextAsset(AbaFilePath(abaName), assetName)
	if err != nil {
		t.Skipf("KCES TextAsset sample %s in %s is not available: %v", assetName, abaName, err)
	}
	return data
}

// TextAssetPath 将 TextAsset 样本写入临时文件并返回路径
// TextAssetPath writes a TextAsset sample to a temporary file and returns its path
func TextAssetPath(t TB, abaName string, assetName string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), filepath.Base(assetName))
	if err := os.WriteFile(path, TextAssetBytes(t, abaName, assetName), 0644); err != nil {
		t.Fatalf("write KCES TextAsset sample %s: %v", assetName, err)
	}
	return path
}

// TextAssetPathsBySuffix 从 abaName 中提取所有具有 suffix 后缀的 TextAsset
// TextAssetPathsBySuffix extracts every TextAsset with suffix from abaName
func TextAssetPathsBySuffix(t TB, abaName string, suffix string) []string {
	t.Helper()
	paths, err := readTextAssetPathsBySuffix(AbaFilePath(abaName), suffix)
	if err != nil {
		t.Fatalf("extract KCES TextAsset samples with suffix %s from %s: %v", suffix, abaName, err)
	}
	if len(paths) == 0 {
		t.Skipf("no KCES TextAsset samples with suffix %s in %s", suffix, abaName)
	}
	dir := t.TempDir()
	out := make([]string, 0, len(paths))
	for _, sample := range paths {
		path := filepath.Join(dir, sample.name)
		if err := os.WriteFile(path, sample.data, 0644); err != nil {
			t.Fatalf("write KCES TextAsset sample %s: %v", sample.name, err)
		}
		out = append(out, path)
	}
	return out
}

// RawObjectPath 从 abaName 中提取原始 Unity 对象并写入 .bytes 文件及其 .meta.json 旁车
// RawObjectPath extracts a raw Unity object from abaName and writes both the .bytes file and its .meta.json sidecar
func RawObjectPath(t TB, abaName string, assetName string, classID int32, outputName string) (string, string) {
	t.Helper()
	raw, meta, err := readRawObject(AbaFilePath(abaName), assetName, classID)
	if err != nil {
		t.Skipf("KCES raw object sample %s in %s is not available: %v", assetName, abaName, err)
	}

	dir := t.TempDir()
	rawPath := filepath.Join(dir, outputName)
	if err := os.WriteFile(rawPath, raw, 0644); err != nil {
		t.Fatalf("write KCES raw object sample %s: %v", outputName, err)
	}
	metaPath := rawPath + ".meta.json"
	metaData, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		t.Fatalf("marshal KCES raw object metadata for %s: %v", outputName, err)
	}
	metaData = append(metaData, '\n')
	if err := os.WriteFile(metaPath, metaData, 0644); err != nil {
		t.Fatalf("write KCES raw object metadata for %s: %v", outputName, err)
	}
	return rawPath, metaPath
}

// textAssetSpec 表示一个 TextAsset 样本来源 / textAssetSpec represents one TextAsset sample source
type textAssetSpec struct {
	abaName   string
	assetName string
}

// PartsSamplePaths 从已知部件 ABA 中提取 menuassets、materialassets、model 和 pmatassets 样本
// PartsSamplePaths extracts menuassets, materialassets, model, and pmatassets samples from known parts ABAs
func PartsSamplePaths(t TB) []string {
	t.Helper()
	return textAssetPathsForSpecs(t, "parts", []textAssetSpec{
		{"cm3d2_megane002.aba", "cm3d2_megane002.menuassets"},
		{"cm3d2_eyes.aba", "cm3d2_eyes.menuassets"},
		{"parts_personal002.aba", "parts_personal002.menuassets"},
		{"cm3d2_megane002.aba", "cm3d2_megane002.materialassets"},
		{"parts_personal002.aba", "parts_personal002.materialassets"},
		{"crc_nt008_accha001.aba", "crc_nt008_accha009.materialassets"},
		{"cm3d2_megane002.aba", "cm3d2_megane002.model"},
		{"parts_personal002.aba", "hair_twin019.model"},
		{"partsmeta.aba", "partsmeta.pmatassets"},
	})
}

// PayloadSamplePaths 从已知数据 ABA 中提取常用 KCES payload 样本
// PayloadSamplePaths extracts common KCES payload samples from known data ABAs
func PayloadSamplePaths(t TB) []string {
	t.Helper()
	return textAssetPathsForSpecs(t, "payload", []textAssetSpec{
		{"partsmeta.aba", "default_hairf.dbconf"},
		{"partsmeta.aba", "default_bra_col.dbcol"},
		{"partsmeta.aba", "default_hairf.db2conf"},
		{"partsmeta.aba", "default_skirt.dsbconf"},
		{"partsmeta.aba", "default_skirt.dsb2conf"},
		{"partsmeta.aba", "default_sleeve.dsl2conf"},
		{"partsmeta.aba", "default_sleeve_col.dslcol"},
		{"system.aba", "maidIKCollider.ikcol"},
		{"system.aba", "limbconf.limbcol"},
	})
}

// MiscSamplePaths 从已知数据 ABA 中提取 hitcheck、undressdat 和 undresspdat 样本
// MiscSamplePaths extracts hitcheck, undressdat, and undresspdat samples from known data ABAs
func MiscSamplePaths(t TB) []string {
	t.Helper()
	return textAssetPathsForSpecs(t, "misc", []textAssetSpec{
		{"system.aba", "IK.hitcheck"},
		{"parts_bv001.aba", "crc2_Underwear038_pants.undressdat"},
		{"parts_bv001.aba", "crc2_Underwear038_pants.undresspdat"},
	})
}

// AssetSamplePaths 返回 KCES 可编辑资产样本路径，包括 NEI、PSK、PNG 和原始 Unity 对象
// AssetSamplePaths returns KCES editable asset sample paths including NEI, PSK, PNG, and raw Unity objects
func AssetSamplePaths(t TB) []string {
	t.Helper()
	paths := []string{
		TextAssetPath(t, "csv.aba", "edit_pose_enabled_list.nei"),
		TextAssetPath(t, "partsmeta.aba", "default_skirt.psk"),
		TexturePNGPath(t, "cm3d2_megane002.aba", "crc_nt008_line.tex", "crc_nt008_line.tex.png"),
	}
	rawTexturePath, _ := RawObjectPath(t, "cm3d2_megane002.aba", "cm3d2_megane002.tex", aba.ClassIDTexture2D, "cm3d2_megane002.tex.bytes")
	rawMonoScriptPath, _ := RawObjectPath(t, "system.aba", "DepthLUT", aba.ClassIDMonoScript, "DepthLUT.monoscript.bytes")
	paths = append(paths, rawTexturePath, rawMonoScriptPath)
	return paths
}

// RawObjectSamplePaths 返回服务层原始 Unity 对象测试使用的 .bytes 样本路径
// RawObjectSamplePaths returns .bytes sample paths used by the service-layer raw Unity object tests
func RawObjectSamplePaths(t TB) []string {
	t.Helper()
	rawTexturePath, _ := RawObjectPath(t, "cm3d2_megane002.aba", "cm3d2_megane002.tex", aba.ClassIDTexture2D, "cm3d2_megane002.tex.bytes")
	rawMonoScriptPath, _ := RawObjectPath(t, "system.aba", "DepthLUT", aba.ClassIDMonoScript, "DepthLUT.monoscript.bytes")
	return []string{rawTexturePath, rawMonoScriptPath}
}

// TexturePNGPath 从 ABA 中提取指定 Texture2D 并写出 PNG 文件
// TexturePNGPath extracts the selected Texture2D from an ABA and writes it as a PNG file
func TexturePNGPath(t TB, abaName string, textureName string, outputName string) string {
	t.Helper()
	texture, err := readTexture2D(AbaFilePath(abaName), textureName)
	if err != nil {
		t.Skipf("KCES Texture2D sample %s in %s is not available: %v", textureName, abaName, err)
	}
	data, err := aba.TexturePNGBytes(texture)
	if err != nil {
		t.Skipf("KCES Texture2D sample %s in %s cannot be encoded as PNG: %v", textureName, abaName, err)
	}
	path := filepath.Join(t.TempDir(), outputName)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write KCES Texture2D PNG sample %s: %v", outputName, err)
	}
	return path
}

// textAssetPathsForSpecs 从一组 ABA 来源中提取可用 TextAsset，并在无样本可用时跳过测试
// textAssetPathsForSpecs extracts available TextAssets from a set of ABA sources and skips the test when no samples are available
func textAssetPathsForSpecs(t TB, group string, specs []textAssetSpec) []string {
	t.Helper()
	dir := t.TempDir()
	paths := make([]string, 0, len(specs))
	for _, spec := range specs {
		data, err := readTextAsset(AbaFilePath(spec.abaName), spec.assetName)
		if err != nil {
			continue
		}
		path := filepath.Join(dir, filepath.Base(spec.assetName))
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatalf("write %s TextAsset sample %s: %v", group, spec.assetName, err)
		}
		paths = append(paths, path)
	}
	if len(paths) == 0 {
		t.Skipf("no %s TextAsset samples available", group)
	}
	return paths
}

// textAssetSample 保存一个待写入临时目录的 TextAsset 名称和字节内容 / textAssetSample stores one TextAsset name and byte content to be written into a temporary directory
type textAssetSample struct {
	name string // name 是 ABA 中 TextAsset 的内部名称 / name is the internal TextAsset name inside the ABA
	data []byte // data 是 TextAsset 的字节内容 / data is the TextAsset byte content
}

// rawAssetMeta 保存原始 Unity 对象旁车所需的最小元数据 / rawAssetMeta stores the minimal metadata needed by a raw Unity object sidecar
type rawAssetMeta struct {
	// PathID 是 Unity PathID / PathID is the Unity PathID
	PathID int64 `json:"pathId"`
	// LoadName 是 AssetBundle m_Container 加载名 / LoadName is the AssetBundle m_Container load name
	LoadName string `json:"loadName,omitempty"`
}

// repoRoot 从当前工作目录向上查找包含 go.mod 的仓库根目录
// repoRoot walks upward from the working directory to find the repository root containing go.mod
func repoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("find repository root: %v", err))
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("repository root not found from working directory")
		}
		dir = parent
	}
}

// readTextAsset 从单个 ABA 中按名称提取一个 TextAsset
// readTextAsset extracts one TextAsset by name from a single ABA
func readTextAsset(path string, assetName string) ([]byte, error) {
	found := false
	var result []byte
	err := visitTextAssets(path, func(name string, data []byte) error {
		if name != assetName {
			return nil
		}
		found = true
		result = append([]byte(nil), data...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("TextAsset %q not found", assetName)
	}
	return result, nil
}

// readTextAssetPathsBySuffix 从单个 ABA 中提取所有具有指定后缀的 TextAsset
// readTextAssetPathsBySuffix extracts every TextAsset with the selected suffix from a single ABA
func readTextAssetPathsBySuffix(path string, suffix string) ([]textAssetSample, error) {
	var samples []textAssetSample
	err := visitTextAssets(path, func(name string, data []byte) error {
		if strings.EqualFold(filepath.Ext(name), suffix) {
			samples = append(samples, textAssetSample{name: name, data: append([]byte(nil), data...)})
		}
		return nil
	})
	return samples, err
}

// visitTextAssets 遍历单个 ABA 中的全部 TextAsset 并交给 visit 回调
// visitTextAssets walks every TextAsset in a single ABA and hands each one to the visit callback
func visitTextAssets(path string, visit func(string, []byte) error) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	bundle, err := aba.ReadAba(file)
	if err != nil {
		return err
	}
	for directoryIndex := range bundle.BlockInfo.DirectoryInfos {
		entry := &bundle.BlockInfo.DirectoryInfos[directoryIndex]
		if !entry.IsSerialized() {
			continue
		}
		assetsFile, err := aba.ReadAssetsFileRange(entry.DecompressedSize, func(offset int64, size int64) ([]byte, error) {
			return bundle.GetFileDataRange(int64(directoryIndex), offset, size)
		})
		if err != nil {
			return err
		}
		for assetIndex := range assetsFile.Metadata.AssetInfos {
			info := &assetsFile.Metadata.AssetInfos[assetIndex]
			if info.TypeId != aba.ClassIDTextAsset {
				continue
			}
			name, data, err := assetsFile.GetTextAssetData(info)
			if err != nil {
				return fmt.Errorf("read TextAsset %q: %w", name, err)
			}
			if err := visit(name, data); err != nil {
				return err
			}
		}
	}
	return nil
}

// readRawObject 从单个 ABA 中提取指定名称和 ClassID 的原始 Unity 对象及其元数据
// readRawObject extracts the raw Unity object with the selected name and ClassID plus its metadata from a single ABA
func readRawObject(path string, assetName string, classID int32) ([]byte, rawAssetMeta, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, rawAssetMeta{}, err
	}
	defer file.Close()

	bundle, err := aba.ReadAba(file)
	if err != nil {
		return nil, rawAssetMeta{}, err
	}
	for directoryIndex := range bundle.BlockInfo.DirectoryInfos {
		entry := &bundle.BlockInfo.DirectoryInfos[directoryIndex]
		if !entry.IsSerialized() {
			continue
		}
		assetsFile, err := aba.ReadAssetsFileRange(entry.DecompressedSize, func(offset int64, size int64) ([]byte, error) {
			return bundle.GetFileDataRange(int64(directoryIndex), offset, size)
		})
		if err != nil {
			return nil, rawAssetMeta{}, err
		}
		for _, candidate := range assetsFile.GetAssetEntries() {
			if candidate.Name != assetName || candidate.TypeId != classID {
				continue
			}
			info := assetsFile.GetAssetInfoByPathID(candidate.PathId)
			if info == nil {
				return nil, rawAssetMeta{}, fmt.Errorf("raw object %q PathID %d has no AssetInfo", assetName, candidate.PathId)
			}
			data, err := assetsFile.GetAssetData(info)
			if err != nil {
				return nil, rawAssetMeta{}, fmt.Errorf("read raw object %q: %w", assetName, err)
			}
			meta := rawAssetMeta{PathID: info.PathId}
			if containers, err := assetsFile.GetAssetBundleContainerMap(); err == nil {
				meta.LoadName = containers[info.PathId]
			}
			return append([]byte(nil), data...), meta, nil
		}
	}
	return nil, rawAssetMeta{}, fmt.Errorf("raw object %q class %d not found", assetName, classID)
}

// readTexture2D 从单个 ABA 中提取指定名称的 Texture2D 数据
// readTexture2D extracts Texture2D data with the selected name from a single ABA
func readTexture2D(path string, textureName string) (*aba.Texture2DData, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	bundle, err := aba.ReadAba(file)
	if err != nil {
		return nil, err
	}
	for directoryIndex := range bundle.BlockInfo.DirectoryInfos {
		entry := &bundle.BlockInfo.DirectoryInfos[directoryIndex]
		if !entry.IsSerialized() {
			continue
		}
		assetsFile, err := aba.ReadAssetsFileRange(entry.DecompressedSize, func(offset int64, size int64) ([]byte, error) {
			return bundle.GetFileDataRange(int64(directoryIndex), offset, size)
		})
		if err != nil {
			return nil, err
		}
		for _, candidate := range assetsFile.GetAssetEntries() {
			if candidate.Name != textureName || candidate.TypeId != aba.ClassIDTexture2D {
				continue
			}
			info := assetsFile.GetAssetInfoByPathID(candidate.PathId)
			if info == nil {
				return nil, fmt.Errorf("Texture2D %q PathID %d has no AssetInfo", textureName, candidate.PathId)
			}
			return assetsFile.GetTexture2DDataRange(info, bundle.GetFileDataRangeByName)
		}
	}
	return nil, fmt.Errorf("Texture2D %q not found", textureName)
}
