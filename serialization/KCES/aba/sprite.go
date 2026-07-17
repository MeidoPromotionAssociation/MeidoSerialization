package aba

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/tools"
)

// AssetResolver 解析 Unity PPtr 引用 / AssetResolver resolves Unity PPtr references
// fileID 0 表示当前 AssetsFile，正数 fileID 是从 1 开始的外部依赖索引 / fileID 0 is the current AssetsFile, while positive file IDs are one-based external dependency indexes
type AssetResolver func(relativeTo *AssetsFile, fileID int, pathID int64) (*AssetsFile, *AssetInfo, error)

// SpriteExport 描述从 Texture2D 或 SpriteAtlas 裁剪出的 Sprite / SpriteExport describes a Sprite cropped from a Texture2D or SpriteAtlas
type SpriteExport struct {
	Name        string         // Sprite 名称 / Sprite name
	Texture     *Texture2DData // 源 Texture2D 数据 / Source Texture2D data
	Rect        SpriteRect     // 裁剪矩形 / Crop rectangle
	SettingsRaw uint32         // Unity SpriteSettings 原始位标志 / Raw Unity SpriteSettings flags
}

// SpriteRect 使用 Unity 贴图坐标，原点在左下角 / SpriteRect uses Unity texture coordinates with the origin at the lower-left corner
type SpriteRect struct {
	X      float32 // 左下角 X 坐标 / Lower-left X coordinate
	Y      float32 // 左下角 Y 坐标 / Lower-left Y coordinate
	Width  float32 // 宽度 / Width
	Height float32 // 高度 / Height
}

// unityPPtr 表示 Unity 序列化对象引用 / unityPPtr represents a Unity serialized object reference
type unityPPtr struct {
	FileID int   // 文件 ID，0 表示当前文件 / File ID, zero means current file
	PathID int64 // 路径 ID / Path ID
}

// spriteRenderDataKey 表示 SpriteAtlas m_RenderDataMap 的键 / spriteRenderDataKey represents a SpriteAtlas m_RenderDataMap key
type spriteRenderDataKey struct {
	GUID   [4]uint32 // Sprite render data GUID / Sprite render data GUID
	Second int64     // 复合 key 的 second 部分 / Second part of the composite key
}

// DefaultAssetResolver 只解析同一 AssetsFile 内部引用 / DefaultAssetResolver resolves references within the same AssetsFile only
func DefaultAssetResolver(relativeTo *AssetsFile, fileID int, pathID int64) (*AssetsFile, *AssetInfo, error) {
	if pathID == 0 {
		return nil, nil, fmt.Errorf("null PPtr")
	}
	if fileID != 0 {
		return nil, nil, fmt.Errorf("external asset fileID=%d is not available", fileID)
	}
	if relativeTo == nil {
		return nil, nil, fmt.Errorf("nil relative AssetsFile")
	}
	info := relativeTo.GetAssetInfoByPathID(pathID)
	if info == nil {
		return nil, nil, fmt.Errorf("asset PathID=%d not found", pathID)
	}
	return relativeTo, info, nil
}

// BundleAssetResolver 返回可解析 bundle 内全部序列化文件的 resolver / BundleAssetResolver returns a resolver for all serialized files in a bundle
// map key 应包含 bundle 目录名或文件基名 / Map keys should include bundle directory names or file basenames
func BundleAssetResolver(files map[string]*AssetsFile) AssetResolver {
	return func(relativeTo *AssetsFile, fileID int, pathID int64) (*AssetsFile, *AssetInfo, error) {
		if pathID == 0 {
			return nil, nil, fmt.Errorf("null PPtr")
		}
		if fileID == 0 {
			if relativeTo == nil {
				return nil, nil, fmt.Errorf("nil relative AssetsFile")
			}
			info := relativeTo.GetAssetInfoByPathID(pathID)
			if info == nil {
				return nil, nil, fmt.Errorf("asset PathID=%d not found in current AssetsFile", pathID)
			}
			return relativeTo, info, nil
		}
		if relativeTo != nil {
			depIdx := fileID - 1
			if depIdx >= 0 && depIdx < len(relativeTo.Metadata.ExternalFiles) {
				depName := normalizeBundleAssetPath(relativeTo.Metadata.ExternalFiles[depIdx].PathName)
				if depName != "" {
					af, matched, err := lookupBundleAssetFile(files, depName)
					if err != nil {
						return nil, nil, fmt.Errorf("resolve external asset %q: %w", depName, err)
					}
					if matched {
						if info := af.GetAssetInfoByPathID(pathID); info != nil {
							return af, info, nil
						}
						return nil, nil, fmt.Errorf("asset PathID=%d not found in external file %q", pathID, depName)
					}
				}
			}
		}
		seen := make(map[*AssetsFile]struct{}, len(files))
		var matches []*AssetsFile
		for _, af := range files {
			if af == nil {
				continue
			}
			if _, ok := seen[af]; ok {
				continue
			}
			seen[af] = struct{}{}
			if info := af.GetAssetInfoByPathID(pathID); info != nil {
				matches = append(matches, af)
			}
		}
		if len(matches) > 1 {
			return nil, nil, fmt.Errorf("asset PathID=%d is ambiguous across %d AssetsFiles", pathID, len(matches))
		}
		if len(matches) == 1 {
			info := matches[0].GetAssetInfoByPathID(pathID)
			return matches[0], info, nil
		}
		return nil, nil, fmt.Errorf("asset fileID=%d PathID=%d not found", fileID, pathID)
	}
}

// lookupBundleAssetFile resolves an external-file name without depending on
// map iteration order. Case-sensitive normalized matches take precedence over
// case-insensitive matches; multiple distinct AssetsFiles are always rejected
// as ambiguous. Multiple aliases that point to the same AssetsFile are safe.
func lookupBundleAssetFile(files map[string]*AssetsFile, name string) (*AssetsFile, bool, error) {
	fullName := normalizeBundleAssetPath(name)
	if fullName == "" {
		return nil, false, nil
	}
	find := func(target string, equalFold bool, baseOnly bool) map[*AssetsFile]struct{} {
		matches := make(map[*AssetsFile]struct{})
		for key, af := range files {
			if af == nil {
				continue
			}
			normalized := normalizeBundleAssetPath(key)
			if normalized == "" {
				continue
			}
			candidate := normalized
			if baseOnly {
				candidate = path.Base(normalized)
			}
			if (!equalFold && candidate == target) || (equalFold && strings.EqualFold(candidate, target)) {
				matches[af] = struct{}{}
			}
		}
		return matches
	}

	// Prefer the complete normalized path. This preserves Unity's external
	// file identity when two directories contain same-named assets files.
	matches := find(fullName, false, false)
	if len(matches) == 0 {
		matches = find(fullName, true, false)
	}
	// Older callers often index bundle files by basename only. Retain that
	// compatibility as a final fallback after complete-path matching.
	if len(matches) == 0 {
		baseName := path.Base(fullName)
		matches = find(baseName, false, true)
		if len(matches) == 0 {
			matches = find(baseName, true, true)
		}
	}
	if len(matches) > 1 {
		return nil, true, fmt.Errorf("external asset name %q matches %d AssetsFiles", name, len(matches))
	}
	for af := range matches {
		return af, true, nil
	}
	return nil, false, nil
}

func normalizeBundleAssetPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" || p == "." {
		return ""
	}
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.TrimPrefix(p, "archive://")
	p = strings.TrimPrefix(p, "archive:/")
	p = strings.TrimLeft(p, "/")
	if p == "" || p == "." {
		return ""
	}
	p = path.Clean(p)
	if p == "." || p == ".." || strings.HasPrefix(p, "../") {
		return ""
	}
	return p
}

// GetSpriteExport 将 Unity Sprite 解析为 Texture2D、裁剪矩形和 SpriteSettings / GetSpriteExport resolves a Unity Sprite to a Texture2D, crop rectangle, and SpriteSettings
// 支持直接 m_RD.texture 引用和通过 m_RenderDataMap 的 SpriteAtlas 间接引用 / It supports direct m_RD.texture references and SpriteAtlas indirection through m_RenderDataMap
func (af *AssetsFile) GetSpriteExport(info *AssetInfo, resolver AssetResolver, streamResolver BundleFileResolver) (*SpriteExport, error) {
	var rangeResolver BundleFileRangeResolver
	if streamResolver != nil {
		rangeResolver = bundleRangeResolverAdapter{whole: streamResolver}.ResolveBundleFileRange
	}
	return af.GetSpriteExportRange(info, resolver, rangeResolver)
}

// GetSpriteExportRange 是 GetSpriteExport 的范围读取版本 / GetSpriteExportRange is the range-read variant of GetSpriteExport
func (af *AssetsFile) GetSpriteExportRange(info *AssetInfo, resolver AssetResolver, streamResolver BundleFileRangeResolver) (*SpriteExport, error) {
	if resolver == nil {
		resolver = DefaultAssetResolver
	}
	root, err := af.ReadAssetValue(info)
	if err != nil {
		return nil, err
	}
	name, _ := root.Field("m_Name").String()
	rd := root.Field("m_RD")
	if rd == nil {
		return nil, fmt.Errorf("m_RD field missing")
	}

	rect, hasRect := readSpriteRect(rd.Field("textureRect"))
	settingsRaw := readUint32Field(rd.Field("settingsRaw"))
	if pptr, ok := readPPtr(rd.Field("texture")); ok && pptr.PathID != 0 {
		tex, err := resolveTexture2D(af, pptr, resolver, streamResolver)
		if err != nil {
			return nil, err
		}
		if !hasRect {
			rect = fullSpriteRect(tex)
		}
		return &SpriteExport{Name: name, Texture: tex, Rect: rect, SettingsRaw: settingsRaw}, nil
	}

	tex, atlasRect, atlasSettings, err := af.resolveSpriteViaAtlas(root, resolver, streamResolver)
	if err != nil {
		return nil, err
	}
	return &SpriteExport{Name: name, Texture: tex, Rect: atlasRect, SettingsRaw: atlasSettings}, nil
}

func (af *AssetsFile) resolveSpriteViaAtlas(root *TypeTreeValue, resolver AssetResolver, streamResolver BundleFileRangeResolver) (*Texture2DData, SpriteRect, uint32, error) {
	spriteKey, ok := readRenderDataKey(root.Field("m_RenderDataKey"))
	if !ok {
		return nil, SpriteRect{}, 0, fmt.Errorf("m_RenderDataKey field missing")
	}
	atlasPtr, ok := readPPtr(root.Field("m_SpriteAtlas"))
	var atlasErr error
	if ok && atlasPtr.PathID != 0 {
		atlasAF, atlasInfo, err := resolver(af, atlasPtr.FileID, atlasPtr.PathID)
		if err != nil {
			atlasErr = fmt.Errorf("resolve SpriteAtlas: %w", err)
		} else {
			tex, rect, settings, err := atlasAF.resolveSpriteInAtlas(atlasInfo, spriteKey, resolver, streamResolver)
			if err == nil {
				return tex, rect, settings, nil
			}
			atlasErr = err
		}
	}

	for _, atlasInfo := range af.GetAssetsByType(ClassIDSpriteAtlas) {
		tex, rect, settings, err := af.resolveSpriteInAtlas(&atlasInfo, spriteKey, resolver, streamResolver)
		if err == nil {
			return tex, rect, settings, nil
		}
	}
	if atlasErr != nil {
		return nil, SpriteRect{}, 0, fmt.Errorf("SpriteAtlas render data not found: %w", atlasErr)
	}
	return nil, SpriteRect{}, 0, fmt.Errorf("SpriteAtlas render data not found")
}

func (af *AssetsFile) resolveSpriteInAtlas(atlasInfo *AssetInfo, key spriteRenderDataKey, resolver AssetResolver, streamResolver BundleFileRangeResolver) (*Texture2DData, SpriteRect, uint32, error) {
	root, err := af.ReadAssetValue(atlasInfo)
	if err != nil {
		return nil, SpriteRect{}, 0, err
	}
	for _, entry := range collectAtlasMapEntries(root.Field("m_RenderDataMap")) {
		first := entry.Field("first")
		entryKey, ok := readAtlasMapKey(first)
		if !ok || entryKey != key {
			continue
		}
		data := entry.Field("second")
		if data == nil {
			continue
		}
		pptr, ok := readPPtr(data.Field("texture"))
		if !ok || pptr.PathID == 0 {
			return nil, SpriteRect{}, 0, fmt.Errorf("SpriteAtlasData.texture is null")
		}
		tex, err := resolveTexture2D(af, pptr, resolver, streamResolver)
		if err != nil {
			return nil, SpriteRect{}, 0, err
		}
		rect, ok := readSpriteRect(data.Field("textureRect"))
		if !ok {
			rect = fullSpriteRect(tex)
		}
		return tex, rect, readUint32Field(data.Field("settingsRaw")), nil
	}
	return nil, SpriteRect{}, 0, fmt.Errorf("m_RenderDataMap has no matching key")
}

func resolveTexture2D(relativeTo *AssetsFile, pptr unityPPtr, resolver AssetResolver, streamResolver BundleFileRangeResolver) (*Texture2DData, error) {
	texAF, texInfo, err := resolver(relativeTo, pptr.FileID, pptr.PathID)
	if err != nil {
		return nil, err
	}
	if texInfo.TypeId != ClassIDTexture2D {
		return nil, fmt.Errorf("PathID=%d is %s, not Texture2D", pptr.PathID, classIdToName(texInfo.TypeId))
	}
	return texAF.GetTexture2DDataRange(texInfo, streamResolver)
}

// WriteSpritePNG 将 Sprite 裁剪结果导出为 PNG / WriteSpritePNG exports a Sprite crop as PNG
// 贴图解压和最终裁剪、翻转、旋转交给 ImageMagick 处理 / Texture decompression and final crop, flip, and rotate are delegated to ImageMagick
func WriteSpritePNG(sprite *SpriteExport, outPath string) error {
	if sprite == nil {
		return fmt.Errorf("nil sprite")
	}
	if sprite.Texture == nil {
		return fmt.Errorf("sprite has no texture")
	}
	if err := tools.CheckMagick(); err != nil {
		return err
	}
	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create sprite PNG %q: %w", outPath, err)
	}
	writeErr := WriteSpritePNGTo(sprite, f)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return fmt.Errorf("close sprite PNG %q: %w", outPath, closeErr)
	}
	return nil
}

// WriteSpritePNGTo writes a cropped Sprite PNG to an already-open
// destination. ImageMagick emits to stdout so callers can retain filesystem
// confinement with an os.Root-backed file handle.
func WriteSpritePNGTo(sprite *SpriteExport, out io.Writer) error {
	if sprite == nil {
		return fmt.Errorf("nil sprite")
	}
	if sprite.Texture == nil {
		return fmt.Errorf("sprite has no texture")
	}
	if out == nil {
		return fmt.Errorf("nil sprite PNG writer")
	}
	if err := tools.CheckMagick(); err != nil {
		return err
	}
	inputFormat, inputData, err := textureInputForMagick(sprite.Texture)
	if err != nil {
		return err
	}

	crop := spriteCropGeometry(sprite.Texture, sprite.Rect)
	args := []string{}
	if isRawMagickInputFormat(inputFormat) {
		args = append(args, "-size", fmt.Sprintf("%dx%d", sprite.Texture.Width, sprite.Texture.Height), "-depth", "8")
	}
	args = append(args, inputFormat+":-")
	if crop != "" {
		args = append(args, "-crop", crop, "+repage")
	}
	args = append(args, spriteMagickOrientationArgs(sprite.SettingsRaw)...)
	args = append(args, "png32:-")

	cmd := exec.Command("magick", args...)
	tools.SetHideWindow(cmd)
	cmd.Stdin = bytes.NewReader(inputData)
	cmd.Stdout = out
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("magick export sprite %q failed: %w, stderr: %s", sprite.Name, err, stderr.String())
	}
	return nil
}

func spriteCropGeometry(tex *Texture2DData, rect SpriteRect) string {
	x := int(math.Round(float64(rect.X)))
	y := tex.Height - int(math.Round(float64(rect.Y+rect.Height)))
	w := int(rect.Width)
	h := int(rect.Height)
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	if tex.Width <= 0 || tex.Height <= 0 {
		return ""
	}
	x = clampInt(x, 0, tex.Width-1)
	y = clampInt(y, 0, tex.Height-1)
	w = clampInt(w, 1, tex.Width-x)
	h = clampInt(h, 1, tex.Height-y)
	if x == 0 && y == 0 && w == tex.Width && h == tex.Height {
		return ""
	}
	return fmt.Sprintf("%dx%d+%d+%d", w, h, x, y)
}

func spriteMagickOrientationArgs(settingsRaw uint32) []string {
	if settingsRaw&1 == 0 {
		return nil
	}
	switch (settingsRaw >> 2) & 15 {
	case 1:
		return []string{"-flop"}
	case 2:
		return []string{"-rotate", "180", "-flop"}
	case 3:
		return []string{"-rotate", "180"}
	case 4:
		return []string{"-rotate", "270"}
	default:
		return nil
	}
}

func readPPtr(v *TypeTreeValue) (unityPPtr, bool) {
	if v == nil {
		return unityPPtr{}, false
	}
	fileID, okFile := v.Field("m_FileID").Int64()
	pathID, okPath := v.Field("m_PathID").Int64()
	if !okFile || !okPath {
		return unityPPtr{}, false
	}
	return unityPPtr{FileID: int(fileID), PathID: pathID}, true
}

func readRenderDataKey(v *TypeTreeValue) (spriteRenderDataKey, bool) {
	if v == nil {
		return spriteRenderDataKey{}, false
	}
	guid, ok := readGUID(v.Field("first"))
	if !ok {
		return spriteRenderDataKey{}, false
	}
	second, ok := v.Field("second").Int64()
	if !ok {
		return spriteRenderDataKey{}, false
	}
	return spriteRenderDataKey{GUID: guid, Second: second}, true
}

func readAtlasMapKey(v *TypeTreeValue) (spriteRenderDataKey, bool) {
	if v == nil {
		return spriteRenderDataKey{}, false
	}
	guid, ok := readGUID(v.Field("first"))
	if !ok {
		return spriteRenderDataKey{}, false
	}
	second, ok := v.Field("second").Int64()
	if !ok {
		return spriteRenderDataKey{}, false
	}
	return spriteRenderDataKey{GUID: guid, Second: second}, true
}

func readGUID(v *TypeTreeValue) ([4]uint32, bool) {
	var out [4]uint32
	if v == nil || len(v.Children) < 4 {
		return out, false
	}
	for i := 0; i < 4; i++ {
		n, ok := v.Children[i].Int64()
		if !ok {
			return out, false
		}
		out[i] = uint32(n)
	}
	return out, true
}

func readSpriteRect(v *TypeTreeValue) (SpriteRect, bool) {
	if v == nil {
		return SpriteRect{}, false
	}
	x, okX := v.Field("x").Float32()
	y, okY := v.Field("y").Float32()
	w, okW := v.Field("width").Float32()
	h, okH := v.Field("height").Float32()
	if !okX || !okY || !okW || !okH || w <= 0 || h <= 0 {
		return SpriteRect{}, false
	}
	return SpriteRect{X: x, Y: y, Width: w, Height: h}, true
}

func fullSpriteRect(tex *Texture2DData) SpriteRect {
	if tex == nil {
		return SpriteRect{}
	}
	return SpriteRect{Width: float32(tex.Width), Height: float32(tex.Height)}
}

func readUint32Field(v *TypeTreeValue) uint32 {
	if v == nil {
		return 0
	}
	n, ok := v.Int64()
	if !ok {
		return 0
	}
	return uint32(n)
}

func collectAtlasMapEntries(v *TypeTreeValue) []*TypeTreeValue {
	var out []*TypeTreeValue
	var walk func(*TypeTreeValue)
	walk = func(cur *TypeTreeValue) {
		if cur == nil {
			return
		}
		if cur.TypeName == "pair" && cur.Field("first") != nil && cur.Field("second") != nil {
			if first := cur.Field("first"); first != nil && first.Field("first") != nil && first.Field("second") != nil {
				out = append(out, cur)
				return
			}
		}
		for _, child := range cur.Children {
			walk(child)
		}
	}
	walk(v)
	return out
}

func clampInt(v, min, max int) int {
	if max < min {
		return min
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
