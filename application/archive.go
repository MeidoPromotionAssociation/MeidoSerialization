package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/KCES/aba"
	COM3D2Service "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/service/COM3D2"
	KCESService "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/service/KCES"
)

// ArchiveEntry 描述归档中可列出和提取的单个条目 / ArchiveEntry describes one listable and extractable entry in an archive
type ArchiveEntry struct {
	// Name 是归档内部使用正斜杠表示的条目路径或名称 / Name is the entry path or name inside the archive using forward slashes where applicable
	Name string
	// Size 是条目解压后的字节数 / Size is the decompressed size of the entry in bytes
	Size int64
	// Kind 是区分普通文件、虚拟文件和序列化文件的条目类别 / Kind classifies the entry as a regular, virtual, or serialized file
	Kind string
}

// ListArchive 返回归档中经过排序和资源限制检查的条目副本
// ListArchive returns a copy of the sorted and resource-checked entries in an archive
func (e *Engine) ListArchive(ctx context.Context, source Source, formatID string) ([]ArchiveEntry, error) {
	listing, err := e.ListArchiveListing(ctx, source, formatID)
	if err != nil {
		return nil, err
	}
	return append([]ArchiveEntry(nil), listing.Entries...), nil
}

// ListArchiveListing 返回带有源文件指纹的归档列表以供安全分页
// ListArchiveListing returns an archive listing with a source fingerprint for safe pagination
func (e *Engine) ListArchiveListing(ctx context.Context, source Source, formatID string) (ArchiveListing, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if source == nil {
		return ArchiveListing{}, opError("list archive", CodeInvalidArgument, fmt.Errorf("source is required"))
	}
	workspace, path, err := e.materializeWithLimit(ctx, source, source.Name(), e.maxArchiveListingBytes)
	if err != nil {
		return ArchiveListing{}, err
	}
	defer os.RemoveAll(workspace)
	if strings.TrimSpace(formatID) == "" {
		detection, detectErr := e.detectPath(ctx, path)
		if detectErr != nil {
			return ArchiveListing{}, detectErr
		}
		formatID = detection.FormatID
	}
	formatID = normalizeArchiveFormatID(formatID)
	entries, err := e.listArchivePath(ctx, formatID, path)
	if err != nil {
		return ArchiveListing{}, err
	}
	sourceDigest, err := hashArchiveListingSource(ctx, path)
	if err != nil {
		return ArchiveListing{}, err
	}
	return ArchiveListing{
		FormatID:    formatID,
		Entries:     entries,
		fingerprint: archiveListingFingerprint(formatID, sourceDigest, entries),
	}, nil
}

// listArchivePath 使用已物化的本地文件列出指定格式的归档条目
// listArchivePath lists archive entries for a format using an already materialized local file
func (e *Engine) listArchivePath(ctx context.Context, formatID, path string) ([]ArchiveEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, opError("list archive", CodeCanceled, err)
	}
	format, ok := e.registry.Lookup(formatID)
	if !ok || !format.Capability.Archive {
		return nil, opError("list archive", CodeUnsupported, fmt.Errorf("format %q is not an archive", formatID))
	}
	var entries []ArchiveEntry
	switch format.ID {
	case "com3d2.arc":
		arcFile, closer, err := (&COM3D2Service.ArcService{}).ReadArcLazy(path)
		if err != nil {
			return nil, opError("list ARC", CodeInvalidArgument, err)
		}
		defer closer.Close()
		names := arcFile.GetFileList()
		if err := e.validateArchiveEntryCount(len(names)); err != nil {
			return nil, err
		}
		for _, name := range names {
			if err := ctx.Err(); err != nil {
				return nil, opError("list ARC", CodeCanceled, err)
			}
			file := arcFile.GetFile(name)
			if file == nil {
				continue
			}
			entries = append(entries, ArchiveEntry{Name: filepath.ToSlash(name), Size: int64(file.Ptr.RawSize()), Kind: "file"})
		}
	case "kces.ct", "kces.virtualdirectory":
		table, err := (&KCESService.CtService{}).ReadCt(path)
		if err != nil {
			return nil, opError("list content table", CodeInvalidArgument, err)
		}
		if err := e.validateArchiveEntryCount(len(table.Files)); err != nil {
			return nil, err
		}
		for name, file := range table.Files {
			if err := ctx.Err(); err != nil {
				return nil, opError("list content table", CodeCanceled, err)
			}
			if file.Size < 0 {
				return nil, opError("list content table", CodeInvalidArgument, fmt.Errorf("entry %q has negative size %d", name, file.Size))
			}
			entries = append(entries, ArchiveEntry{Name: name, Size: int64(file.Size), Kind: "virtual_file"})
		}
	case "kces.aba", "kces.asset_bg", "kces.asset_scene":
		bundle, closer, err := readKCESUnityFSArchive(format.ID, path)
		if err != nil {
			return nil, opError("list ABA", CodeInvalidArgument, err)
		}
		defer closer.Close()
		if err := e.validateArchiveEntryCount(len(bundle.BlockInfo.DirectoryInfos)); err != nil {
			return nil, err
		}
		for _, entry := range bundle.BlockInfo.DirectoryInfos {
			if err := ctx.Err(); err != nil {
				return nil, opError("list ABA", CodeCanceled, err)
			}
			if entry.DecompressedSize < 0 {
				return nil, opError("list ABA", CodeInvalidArgument, fmt.Errorf("entry %q has negative size %d", entry.Name, entry.DecompressedSize))
			}
			kind := "file"
			if entry.IsSerialized() {
				kind = "serialized_file"
			}
			entries = append(entries, ArchiveEntry{Name: entry.Name, Size: entry.DecompressedSize, Kind: kind})
		}
	default:
		return nil, opError("list archive", CodeUnsupported, fmt.Errorf("format %q has no archive adapter", format.ID))
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	if err := ctx.Err(); err != nil {
		return nil, opError("list archive", CodeCanceled, err)
	}
	return entries, nil
}

// validateArchiveEntryCount 检查归档条目数量是否超过引擎限制
// validateArchiveEntryCount checks whether an archive entry count exceeds the engine limit
func (e *Engine) validateArchiveEntryCount(count int) error {
	if count > e.maxArchiveEntries {
		return opError("list archive", CodeResourceExhausted, fmt.Errorf("archive entry count %d exceeds limit %d", count, e.maxArchiveEntries))
	}
	return nil
}

// hashArchiveListingSource 以支持取消的方式计算归档源文件的 SHA-256 摘要
// hashArchiveListingSource computes the SHA-256 digest of an archive source with cancellation support
func hashArchiveListingSource(ctx context.Context, path string) ([sha256.Size]byte, error) {
	var result [sha256.Size]byte
	file, err := os.Open(path)
	if err != nil {
		return result, opError("fingerprint archive", CodeInternal, err)
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, &contextReader{ctx: ctx, reader: file})
	closeErr := file.Close()
	if copyErr != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return result, opError("fingerprint archive", CodeCanceled, ctxErr)
		}
		return result, opError("fingerprint archive", CodeInternal, copyErr)
	}
	if closeErr != nil {
		return result, opError("fingerprint archive", CodeInternal, closeErr)
	}
	copy(result[:], hash.Sum(nil))
	return result, nil
}

// ExtractArchiveEntry 从受支持的归档中提取指定条目并流式写入输出
// ExtractArchiveEntry extracts a named entry from a supported archive and streams it to the output
func (e *Engine) ExtractArchiveEntry(ctx context.Context, source Source, formatID, entryName string, output io.Writer) (Artifact, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if source == nil || output == nil || strings.TrimSpace(entryName) == "" || strings.IndexByte(entryName, 0) >= 0 {
		return Artifact{}, opError("extract archive entry", CodeInvalidArgument, fmt.Errorf("source, entry name, and output are required"))
	}
	workspace, path, err := e.materialize(ctx, source, source.Name())
	if err != nil {
		return Artifact{}, err
	}
	defer os.RemoveAll(workspace)
	if strings.TrimSpace(formatID) == "" {
		detection, detectErr := e.detectPath(ctx, path)
		if detectErr != nil {
			return Artifact{}, detectErr
		}
		formatID = detection.FormatID
	}
	formatID = strings.ToLower(strings.TrimSpace(formatID))
	outputName := cleanSourceName(entryName)
	switch formatID {
	case "com3d2.arc":
		arcFile, closer, readErr := (&COM3D2Service.ArcService{}).ReadArcLazy(path)
		if readErr != nil {
			return Artifact{}, opError("extract ARC entry", CodeInvalidArgument, readErr)
		}
		defer closer.Close()
		file := arcFile.GetFile(filepath.FromSlash(strings.ReplaceAll(entryName, `\`, "/")))
		if file == nil {
			return Artifact{}, opError("extract ARC entry", CodeNotFound, fmt.Errorf("entry %q was not found", entryName))
		}
		if int64(file.Ptr.RawSize()) > e.maxOutputBytes {
			return Artifact{}, opError("extract ARC entry", CodeResourceExhausted, fmt.Errorf("entry size %d exceeds limit %d", file.Ptr.RawSize(), e.maxOutputBytes))
		}
		outPath := filepath.Join(workspace, "archive-output.bin")
		if err := file.Extract(outPath); err != nil {
			return Artifact{}, opError("extract ARC entry", CodeInvalidArgument, err)
		}
		return e.copyFileArtifact(ctx, outPath, outputName, formatID, RepresentationNative, output)
	case "kces.ct", "kces.virtualdirectory":
		table, readErr := (&KCESService.CtService{}).ReadCt(path)
		if readErr != nil {
			return Artifact{}, opError("extract content-table entry", CodeInvalidArgument, readErr)
		}
		entry, ok := table.Files[entryName]
		if !ok {
			return Artifact{}, opError("extract content-table entry", CodeNotFound, fmt.Errorf("entry %q was not found", entryName))
		}
		if entry.Size < 0 {
			return Artifact{}, opError("extract content-table entry", CodeInvalidArgument, fmt.Errorf("entry %q has negative size %d", entryName, entry.Size))
		}
		if int64(entry.Size) > e.maxOutputBytes {
			return Artifact{}, opError("extract content-table entry", CodeResourceExhausted, fmt.Errorf("entry size %d exceeds limit %d", entry.Size, e.maxOutputBytes))
		}
		data, readErr := table.GetFileData(entryName)
		if readErr != nil {
			return Artifact{}, opError("extract content-table entry", CodeInvalidArgument, readErr)
		}
		return e.copyBytesArtifact(ctx, data, outputName, formatID, output)
	case "kces.aba", "kces.asset_bg", "kces.asset_scene":
		bundle, closer, readErr := readKCESUnityFSArchive(formatID, path)
		if readErr != nil {
			return Artifact{}, opError("extract ABA entry", CodeInvalidArgument, readErr)
		}
		defer closer.Close()
		var entrySize int64
		found := false
		for _, entry := range bundle.BlockInfo.DirectoryInfos {
			if entry.Name == entryName {
				entrySize = entry.DecompressedSize
				found = true
				break
			}
		}
		if !found {
			return Artifact{}, opError("extract ABA entry", CodeNotFound, fmt.Errorf("entry %q was not found", entryName))
		}
		if entrySize < 0 {
			return Artifact{}, opError("extract ABA entry", CodeInvalidArgument, fmt.Errorf("entry %q has negative size %d", entryName, entrySize))
		}
		if entrySize > e.maxOutputBytes {
			return Artifact{}, opError("extract ABA entry", CodeResourceExhausted, fmt.Errorf("entry size %d exceeds limit %d", entrySize, e.maxOutputBytes))
		}
		hash := sha256.New()
		writer := io.MultiWriter(output, hash)
		const rangeSize int64 = 8 << 20
		var written int64
		for written < entrySize {
			if err := ctx.Err(); err != nil {
				return Artifact{}, opError("extract ABA entry", CodeCanceled, err)
			}
			size := min(rangeSize, entrySize-written)
			data, err := bundle.GetFileDataRangeByName(entryName, written, size)
			if err != nil {
				return Artifact{}, opError("extract ABA entry", CodeInvalidArgument, err)
			}
			n, err := writer.Write(data)
			if err != nil {
				return Artifact{}, opError("extract ABA entry", CodeInternal, err)
			}
			if n != len(data) || int64(n) != size {
				return Artifact{}, opError("extract ABA entry", CodeInternal, fmt.Errorf("short range write: wrote %d of %d bytes", n, size))
			}
			written += int64(n)
		}
		return Artifact{Name: outputName, FormatID: formatID, Representation: RepresentationNative, Size: written, SHA256: hex.EncodeToString(hash.Sum(nil))}, nil
	default:
		return Artifact{}, opError("extract archive entry", CodeUnsupported, fmt.Errorf("format %q is not a supported archive", formatID))
	}
}

// readKCESUnityFSArchive 按独立扩展名格式选择对应的 UnityFS service 并返回已解析资源包
// readKCESUnityFSArchive selects the extension-specific UnityFS service and returns the parsed bundle
func readKCESUnityFSArchive(formatID string, path string) (*aba.Aba, *os.File, error) {
	switch formatID {
	case "kces.aba":
		return (&KCESService.AbaService{}).ReadAba(path)
	case "kces.asset_bg":
		return (&KCESService.AssetBGService{}).ReadAssetBG(path)
	case "kces.asset_scene":
		return (&KCESService.AssetSceneService{}).ReadAssetScene(path)
	default:
		return nil, nil, fmt.Errorf("unsupported KCES UnityFS format %q", formatID)
	}
}

// copyBytesArtifact 将内存中的归档条目写入临时文件并复用统一制品流式输出逻辑
// copyBytesArtifact writes an in-memory archive entry to a temporary file and reuses the common artifact streaming logic
func (e *Engine) copyBytesArtifact(ctx context.Context, data []byte, name, formatID string, output io.Writer) (Artifact, error) {
	if int64(len(data)) > e.maxOutputBytes {
		return Artifact{}, opError("extract archive entry", CodeResourceExhausted, fmt.Errorf("entry size %d exceeds limit %d", len(data), e.maxOutputBytes))
	}
	temp, err := os.CreateTemp("", "meido-entry-")
	if err != nil {
		return Artifact{}, opError("extract archive entry", CodeInternal, err)
	}
	path := temp.Name()
	defer os.Remove(path)
	if _, err := io.Copy(temp, bytes.NewReader(data)); err != nil {
		_ = temp.Close()
		return Artifact{}, opError("extract archive entry", CodeInternal, err)
	}
	if err := temp.Close(); err != nil {
		return Artifact{}, opError("extract archive entry", CodeInternal, err)
	}
	return e.copyFileArtifact(ctx, path, name, formatID, RepresentationNative, output)
}
