package main

import (
	"bufio"
	"encoding/binary"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
)

const serializedFileProbeSize int64 = 64 << 10

// scanRow 表示 CSV 中的一个 ABA 或 SerializedFile 扫描结果 / scanRow represents one ABA or SerializedFile result in the CSV report
type scanRow struct {
	AbaPath                string   // ABA 绝对路径 / Absolute ABA path
	AbaSize                int64    // ABA 文件大小 / ABA file size
	Status                 string   // 扫描状态 / Scan status
	Error                  string   // 解析错误 / Parse error
	UnityFSVersion         uint32   // UnityFS 格式版本 / UnityFS format version
	GenerationVersion      string   // UnityFS generation 版本 / UnityFS generation version
	EngineVersion          string   // UnityFS 引擎版本 / UnityFS engine version
	EntryName              string   // SerializedFile 条目名称 / SerializedFile entry name
	EntrySize              int64    // SerializedFile 条目大小 / SerializedFile entry size
	SerializedFileVersion  uint32   // SerializedFile 格式版本 / SerializedFile format version
	SerializedUnityVersion string   // SerializedFile Unity 版本 / SerializedFile Unity version
	TargetPlatform         uint32   // SerializedFile 目标平台 / SerializedFile target platform
	TypeTreeEnabled        bool     // SerializedFile 是否内嵌 TypeTree / Whether the SerializedFile embeds a TypeTree
	DeclaredFileSize       int64    // SerializedFile 声明的文件大小 / File size declared by the SerializedFile
	MetadataSize           uint32   // SerializedFile metadata 大小 / SerializedFile metadata size
	DataOffset             int64    // SerializedFile 数据区偏移 / SerializedFile data-section offset
	Endianness             string   // SerializedFile metadata 字节序 / SerializedFile metadata byte order
	MatchedClassID         int32    // 可选对象扫描匹配的 Unity ClassID / Unity ClassID selected by the optional object scan
	MatchedObjectCount     int64    // 当前 SerializedFile 中匹配的对象数量 / Number of matching objects in this SerializedFile
	MatchedObjectNames     []string // 匹配对象的名称或 PathID 回退标识 / Names or PathID fallback identifiers of matching objects
	ClassScanEnabled       bool     // 是否对当前行执行了对象类型扫描 / Whether the object-type scan ran for this row
}

// scanOptions 保存需要在版本探测后执行的可选深度扫描 / scanOptions stores optional deep scans performed after version probing
type scanOptions struct {
	FindClassID *int32 // 需要查找的 Unity ClassID，nil 表示仅扫描版本 / Unity ClassID to find, with nil meaning version-only scanning
}

// serializedSummary 保存从 SerializedFile 头部和 metadata 前缀读取的版本信息 / serializedSummary stores version data read from a SerializedFile header and metadata prefix
type serializedSummary struct {
	Version         uint32 // SerializedFile 格式版本 / SerializedFile format version
	UnityVersion    string // Unity 版本 / Unity version
	TargetPlatform  uint32 // 目标平台 / Target platform
	TypeTreeEnabled bool   // 是否内嵌 TypeTree / Whether a TypeTree is embedded
	FileSize        int64  // 声明的文件大小 / Declared file size
	MetadataSize    uint32 // metadata 大小 / Metadata size
	DataOffset      int64  // 数据区偏移 / Data-section offset
	Endianness      string // metadata 字节序 / Metadata byte order
}

// main 执行只读的 KCES ABA 版本扫描并在失败时返回非零退出码
// main runs the read-only KCES ABA version scan and returns a non-zero exit code on fatal failure
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "scan failed:", err)
		os.Exit(1)
	}
}

// run 解析命令行参数、递归扫描 ABA 并写出 CSV 与版本汇总
// run parses command-line arguments, recursively scans ABA files, and writes the CSV report and version summary
func run() error {
	root := flag.String("root", "", "ABA file or directory to scan recursively")
	output := flag.String("output", "aba-version-report.csv", "CSV report path, or - for stdout")
	findClass := flag.String("find-class", "", "optional Unity ClassID to locate, for example 74 for AnimationClip")
	flag.Parse()
	if *root == "" && flag.NArg() == 1 {
		*root = flag.Arg(0)
	}
	if *root == "" {
		return fmt.Errorf("provide -root <ABA file or directory>")
	}
	findClassID, err := parseOptionalClassID(*findClass)
	if err != nil {
		return err
	}
	options := scanOptions{FindClassID: findClassID}

	paths, issueRows, err := collectAbaPaths(*root)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("no .aba files found below %q", *root)
	}

	rows := make([]scanRow, 0, len(paths)+len(issueRows))
	rows = append(rows, issueRows...)
	var processed int64
	for _, path := range paths {
		rows = append(rows, scanAbaWithOptions(path, options)...)
		processed++
		if processed%100 == 0 {
			fmt.Fprintf(os.Stderr, "scanned %d/%d ABA files\n", processed, len(paths))
		}
	}
	if err := writeCSVReport(*output, rows); err != nil {
		return err
	}

	summaryOut := io.Writer(os.Stdout)
	if *output == "-" {
		summaryOut = os.Stderr
	}
	printSummary(summaryOut, int64(len(paths)), rows, *output)
	if options.FindClassID != nil {
		printClassMatches(summaryOut, rows, *options.FindClassID)
	}
	return nil
}

// parseOptionalClassID 解析可选的 Unity ClassID 并确保其位于 Int32 范围
// parseOptionalClassID parses an optional Unity ClassID and ensures that it fits the Int32 range
func parseOptionalClassID(value string) (*int32, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid -find-class value %q: expected an Int32 Unity ClassID", value)
	}
	classID := int32(parsed)
	return &classID, nil
}

// collectAbaPaths 收集根路径下全部 ABA，并把无法访问的路径记录为报告行后继续扫描
// collectAbaPaths collects every ABA below the root and records inaccessible paths as report rows while continuing the scan
func collectAbaPaths(root string) ([]string, []scanRow, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve root %q: %w", root, err)
	}
	info, err := os.Stat(absoluteRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("stat root %q: %w", absoluteRoot, err)
	}
	if !info.IsDir() {
		if !strings.EqualFold(filepath.Ext(absoluteRoot), ".aba") {
			return nil, nil, fmt.Errorf("input file %q does not use the .aba extension", absoluteRoot)
		}
		return []string{absoluteRoot}, nil, nil
	}

	var paths []string
	var issues []scanRow
	err = filepath.WalkDir(absoluteRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			issues = append(issues, scanRow{AbaPath: path, Status: "walk_error", Error: walkErr.Error()})
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".aba") {
			return nil
		}
		absolutePath, err := filepath.Abs(path)
		if err != nil {
			issues = append(issues, scanRow{AbaPath: path, Status: "walk_error", Error: err.Error()})
			return nil
		}
		paths = append(paths, absolutePath)
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("walk root %q: %w", absoluteRoot, err)
	}
	sort.Strings(paths)
	return paths, issues, nil
}

// scanAba 读取一个 ABA 的 UnityFS 头和每个 SerializedFile 的版本前缀
// scanAba reads one ABA UnityFS header and the version prefix of every SerializedFile
func scanAba(path string) []scanRow {
	return scanAbaWithOptions(path, scanOptions{})
}

// scanAbaWithOptions 读取 ABA 版本信息并按需查找指定 ClassID 的对象
// scanAbaWithOptions reads ABA version information and optionally locates objects with a selected ClassID
func scanAbaWithOptions(path string, options scanOptions) []scanRow {
	base := scanRow{AbaPath: path}
	info, err := os.Stat(path)
	if err != nil {
		base.Status = "file_error"
		base.Error = err.Error()
		return []scanRow{base}
	}
	base.AbaSize = info.Size()

	file, err := os.Open(path)
	if err != nil {
		base.Status = "file_error"
		base.Error = err.Error()
		return []scanRow{base}
	}
	defer file.Close()

	bundle, err := aba.ReadAba(file)
	if err != nil {
		base.Status = classifyAbaError(err)
		base.Error = err.Error()
		return []scanRow{base}
	}
	base.UnityFSVersion = bundle.Header.Version
	base.GenerationVersion = bundle.Header.GenerationVersion
	base.EngineVersion = bundle.Header.EngineVersion

	var rows []scanRow
	for directoryIndex := range bundle.BlockInfo.DirectoryInfos {
		entry := bundle.BlockInfo.DirectoryInfos[directoryIndex]
		if !entry.IsSerialized() {
			continue
		}
		row := base
		row.EntryName = entry.Name
		row.EntrySize = entry.DecompressedSize
		summary, err := readSerializedSummary(bundle, int64(directoryIndex), entry)
		if err != nil {
			row.Status = "serialized_error"
			row.Error = err.Error()
			rows = append(rows, row)
			continue
		}
		row.Status = "ok"
		row.SerializedFileVersion = summary.Version
		row.SerializedUnityVersion = summary.UnityVersion
		row.TargetPlatform = summary.TargetPlatform
		row.TypeTreeEnabled = summary.TypeTreeEnabled
		row.DeclaredFileSize = summary.FileSize
		row.MetadataSize = summary.MetadataSize
		row.DataOffset = summary.DataOffset
		row.Endianness = summary.Endianness
		if options.FindClassID != nil {
			row.ClassScanEnabled = true
			row.MatchedClassID = *options.FindClassID
			count, names, err := scanSerializedClass(bundle, int64(directoryIndex), entry, *options.FindClassID)
			if err != nil {
				row.Status = "object_scan_error"
				row.Error = err.Error()
				rows = append(rows, row)
				continue
			}
			row.MatchedObjectCount = count
			row.MatchedObjectNames = names
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		base.Status = "no_serialized_file"
		return []scanRow{base}
	}
	return rows
}

// scanSerializedClass 完整读取一个 SerializedFile 并返回指定 ClassID 的对象名称
// scanSerializedClass reads one complete SerializedFile and returns object names for the selected ClassID
func scanSerializedClass(bundle *aba.Aba, directoryIndex int64, entry aba.DirectoryInfo, classID int32) (int64, []string, error) {
	data, err := bundle.GetFileData(directoryIndex)
	if err != nil {
		return 0, nil, fmt.Errorf("read SerializedFile %q: %w", entry.Name, err)
	}
	af, err := aba.ReadAssetsFile(data)
	if err != nil {
		return 0, nil, fmt.Errorf("parse SerializedFile %q: %w", entry.Name, err)
	}
	var count int64
	var names []string
	for _, asset := range af.GetAssetEntries() {
		if asset.TypeId != classID {
			continue
		}
		count++
		name := strings.TrimSpace(asset.Name)
		if name == "" {
			name = fmt.Sprintf("<unnamed PathID=%d>", asset.PathId)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return count, names, nil
}

// readSerializedSummary 只读取 SerializedFile 开头的有界范围并解析版本信息
// readSerializedSummary reads only a bounded prefix of a SerializedFile and parses its version information
func readSerializedSummary(bundle *aba.Aba, directoryIndex int64, entry aba.DirectoryInfo) (serializedSummary, error) {
	if entry.DecompressedSize <= 0 {
		return serializedSummary{}, fmt.Errorf("SerializedFile %q has invalid size %d", entry.Name, entry.DecompressedSize)
	}
	probeSize := entry.DecompressedSize
	if probeSize > serializedFileProbeSize {
		probeSize = serializedFileProbeSize
	}
	data, err := bundle.GetFileDataRange(directoryIndex, 0, probeSize)
	if err != nil {
		return serializedSummary{}, fmt.Errorf("read SerializedFile prefix: %w", err)
	}
	return parseSerializedPrefix(data)
}

// parseSerializedPrefix 解析 SerializedFile 固定头、v22 扩展头和 metadata 的版本前缀
// parseSerializedPrefix parses the SerializedFile fixed header, v22 extended header, and metadata version prefix
func parseSerializedPrefix(data []byte) (serializedSummary, error) {
	if int64(len(data)) < 20 {
		return serializedSummary{}, fmt.Errorf("SerializedFile prefix has only %d bytes", len(data))
	}
	summary := serializedSummary{
		MetadataSize: binary.BigEndian.Uint32(data[0:4]),
		FileSize:     int64(binary.BigEndian.Uint32(data[4:8])),
		Version:      binary.BigEndian.Uint32(data[8:12]),
		DataOffset:   int64(binary.BigEndian.Uint32(data[12:16])),
	}
	endianness := data[16]
	metadataStart := int64(20)
	if summary.Version >= 22 {
		if int64(len(data)) < 48 {
			return serializedSummary{}, fmt.Errorf("v%d SerializedFile prefix has only %d bytes", summary.Version, len(data))
		}
		summary.MetadataSize = binary.BigEndian.Uint32(data[20:24])
		summary.FileSize = int64(binary.BigEndian.Uint64(data[24:32]))
		summary.DataOffset = int64(binary.BigEndian.Uint64(data[32:40]))
		metadataStart = 48
	}
	if summary.FileSize < 0 || summary.DataOffset < 0 {
		return serializedSummary{}, fmt.Errorf("SerializedFile declares negative file size or data offset")
	}
	order, name, err := serializedByteOrder(endianness)
	if err != nil {
		return serializedSummary{}, err
	}
	summary.Endianness = name

	metadataEnd := metadataStart + int64(summary.MetadataSize)
	if metadataEnd < metadataStart {
		return serializedSummary{}, fmt.Errorf("SerializedFile metadata size overflows")
	}
	searchEnd := metadataEnd
	if searchEnd > int64(len(data)) {
		searchEnd = int64(len(data))
	}
	versionEnd := int64(-1)
	for cursor := metadataStart; cursor < searchEnd; cursor++ {
		if data[cursor] == 0 {
			versionEnd = cursor
			break
		}
	}
	if versionEnd < 0 {
		if metadataEnd > int64(len(data)) {
			return serializedSummary{}, fmt.Errorf("Unity version is not terminated within the %d-byte probe", len(data))
		}
		return serializedSummary{}, fmt.Errorf("Unity version is not NUL-terminated inside metadata")
	}
	if versionEnd+6 > searchEnd {
		return serializedSummary{}, fmt.Errorf("metadata ends before target platform and TypeTree flag")
	}
	summary.UnityVersion = string(data[metadataStart:versionEnd])
	summary.TargetPlatform = order.Uint32(data[versionEnd+1 : versionEnd+5])
	typeTreeFlag := data[versionEnd+5]
	if typeTreeFlag > 1 {
		return serializedSummary{}, fmt.Errorf("invalid TypeTreeEnabled value %d", typeTreeFlag)
	}
	summary.TypeTreeEnabled = typeTreeFlag == 1
	return summary, nil
}

// serializedByteOrder 将 SerializedFile 字节序标记转换为 binary.ByteOrder
// serializedByteOrder converts a SerializedFile endianness marker into binary.ByteOrder
func serializedByteOrder(value byte) (binary.ByteOrder, string, error) {
	switch value {
	case 0:
		return binary.LittleEndian, "little", nil
	case 1:
		return binary.BigEndian, "big", nil
	default:
		return nil, "", fmt.Errorf("invalid SerializedFile endianness value %d", value)
	}
}

// classifyAbaError 把加密文件和普通 UnityFS 解析错误分开标记
// classifyAbaError labels encrypted files separately from ordinary UnityFS parse errors
func classifyAbaError(err error) string {
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "encrypted") {
		return "encrypted"
	}
	return "aba_error"
}

// writeCSVReport 以带 UTF-8 BOM 的 CSV 写出全部扫描结果
// writeCSVReport writes all scan results as CSV with a UTF-8 BOM
func writeCSVReport(outputPath string, rows []scanRow) error {
	var output io.Writer
	var closeOutput func() error
	if outputPath == "-" {
		output = os.Stdout
		closeOutput = func() error { return nil }
	} else {
		file, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("create report %q: %w", outputPath, err)
		}
		output = file
		closeOutput = file.Close
	}

	buffered := bufio.NewWriter(output)
	if _, err := buffered.Write([]byte{0xef, 0xbb, 0xbf}); err != nil {
		_ = closeOutput()
		return fmt.Errorf("write report BOM: %w", err)
	}
	writer := csv.NewWriter(buffered)
	if err := writer.Write([]string{
		"aba_path", "aba_size", "status", "error", "unityfs_version", "generation_version", "engine_version",
		"entry_name", "entry_size", "serialized_file_version", "serialized_unity_version", "target_platform",
		"type_tree_enabled", "serialized_declared_size", "metadata_size", "data_offset", "endianness",
		"matched_class_id", "matched_object_count", "matched_object_names",
	}); err != nil {
		_ = closeOutput()
		return fmt.Errorf("write report header: %w", err)
	}
	for _, row := range rows {
		if err := writer.Write(scanRowRecord(row)); err != nil {
			_ = closeOutput()
			return fmt.Errorf("write report row for %q: %w", row.AbaPath, err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		_ = closeOutput()
		return fmt.Errorf("flush CSV report: %w", err)
	}
	if err := buffered.Flush(); err != nil {
		_ = closeOutput()
		return fmt.Errorf("flush report file: %w", err)
	}
	if err := closeOutput(); err != nil {
		return fmt.Errorf("close report file: %w", err)
	}
	return nil
}

// scanRowRecord 将扫描结果转换为固定列顺序的 CSV 记录
// scanRowRecord converts a scan result into a CSV record with a stable column order
func scanRowRecord(row scanRow) []string {
	matchedClassID := ""
	matchedObjectCount := ""
	if row.ClassScanEnabled {
		matchedClassID = strconv.FormatInt(int64(row.MatchedClassID), 10)
		matchedObjectCount = strconv.FormatInt(row.MatchedObjectCount, 10)
	}
	return []string{
		row.AbaPath,
		strconv.FormatInt(row.AbaSize, 10),
		row.Status,
		row.Error,
		strconv.FormatUint(uint64(row.UnityFSVersion), 10),
		row.GenerationVersion,
		row.EngineVersion,
		row.EntryName,
		strconv.FormatInt(row.EntrySize, 10),
		strconv.FormatUint(uint64(row.SerializedFileVersion), 10),
		row.SerializedUnityVersion,
		strconv.FormatUint(uint64(row.TargetPlatform), 10),
		strconv.FormatBool(row.TypeTreeEnabled),
		strconv.FormatInt(row.DeclaredFileSize, 10),
		strconv.FormatUint(uint64(row.MetadataSize), 10),
		strconv.FormatInt(row.DataOffset, 10),
		row.Endianness,
		matchedClassID,
		matchedObjectCount,
		strings.Join(row.MatchedObjectNames, "\n"),
	}
}

// printSummary 输出扫描状态计数和所有成功解析的版本组合
// printSummary prints scan-status counts and every successfully parsed version combination
func printSummary(output io.Writer, abaCount int64, rows []scanRow, reportPath string) {
	statusCounts := make(map[string]int64)
	versionCounts := make(map[string]int64)
	for _, row := range rows {
		statusCounts[row.Status]++
		if row.Status != "ok" {
			continue
		}
		key := fmt.Sprintf(
			"UnityFS=%d Engine=%s SerializedFile=%d Unity=%s Target=%d TypeTree=%t",
			row.UnityFSVersion,
			row.EngineVersion,
			row.SerializedFileVersion,
			row.SerializedUnityVersion,
			row.TargetPlatform,
			row.TypeTreeEnabled,
		)
		versionCounts[key]++
	}
	statusKeys := make([]string, 0, len(statusCounts))
	for key := range statusCounts {
		statusKeys = append(statusKeys, key)
	}
	sort.Strings(statusKeys)
	versionKeys := make([]string, 0, len(versionCounts))
	for key := range versionCounts {
		versionKeys = append(versionKeys, key)
	}
	sort.Strings(versionKeys)

	fmt.Fprintf(output, "\nABA files: %d\nCSV rows: %d\nReport: %s\n", abaCount, len(rows), reportPath)
	fmt.Fprintln(output, "Statuses:")
	for _, key := range statusKeys {
		fmt.Fprintf(output, "  %s: %d\n", key, statusCounts[key])
	}
	fmt.Fprintln(output, "Version combinations:")
	for _, key := range versionKeys {
		fmt.Fprintf(output, "  %d x %s\n", versionCounts[key], key)
	}
}

// printClassMatches 输出包含指定 ClassID 对象的 ABA、SerializedFile 和对象名称
// printClassMatches prints ABA files, SerializedFiles, and object names containing the selected ClassID
func printClassMatches(output io.Writer, rows []scanRow, classID int32) {
	fmt.Fprintf(output, "ClassID %d matches:\n", classID)
	var matchingFiles int64
	var matchingObjects int64
	for _, row := range rows {
		if row.Status != "ok" || row.MatchedObjectCount == 0 {
			continue
		}
		matchingFiles++
		matchingObjects += row.MatchedObjectCount
		fmt.Fprintf(output, "  %s | %s | %d object(s)\n", row.AbaPath, row.EntryName, row.MatchedObjectCount)
		for _, name := range row.MatchedObjectNames {
			fmt.Fprintf(output, "    %s\n", name)
		}
	}
	if matchingFiles == 0 {
		fmt.Fprintln(output, "  none")
	}
	fmt.Fprintf(output, "ClassID %d totals: %d SerializedFile(s), %d object(s)\n", classID, matchingFiles, matchingObjects)
}
