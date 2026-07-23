package KCES

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

// .vd (bridge_session.vd)
// KCES 角色编辑桥接会话容器，使用 VirtualDirectory 保存 session_data 与 session_id
// 外层容器当前版本为 1000，session_data 是未压缩的 MessagePack indexed object
// .vd (bridge_session.vd)
// KCES character-edit bridge session container using VirtualDirectory to store session_data and session_id
// The current outer-container version is 1000 and session_data is an uncompressed MessagePack indexed object

const (
	// KCESBridgeSessionFormat 标识 CRCEdit.EditBridgeSessionData 的 bridge_session.vd 容器所用的库内可编辑表示
	// KCESBridgeSessionFormat identifies the library editing representation of the CRCEdit.EditBridgeSessionData bridge_session.vd container
	KCESBridgeSessionFormat = "kces-bridge-session"

	// 以下是显式构造函数使用的当前版本，解码器不会注入这些值
	// These are the current versions used by the explicit constructor and are not injected by decoders
	KCESBridgeSessionContainerVersion = 1000
	KCESBridgeSessionDataVersion      = 0

	kcesBridgeSessionDataFile = "session_data"
	kcesBridgeSessionIDFile   = "session_id"
)

// KCESBridgeSession 表示写入 bridge_session.vd 的完整 VirtualDirectory
// EditBridgeSessionData 识别的两个文件公开为专用字段，其余虚拟文件原样保留
// KCESBridgeSession represents the complete VirtualDirectory written to bridge_session.vd
// The two files understood by EditBridgeSessionData are exposed as dedicated fields while every other virtual file is retained verbatim
type KCESBridgeSession struct {
	Format                  string                                 `json:"format"`                            // 库的可编辑表示标识，不写入游戏文件 / Library editing-representation identifier, not written to the game file
	ContainerVersion        int32                                  `json:"containerVersion"`                  // 外层 VirtualDirectory 对象版本 / Outer VirtualDirectory object version
	ContainerVersionless    bool                                   `json:"containerVersionless,omitempty"`    // 原始外层对象是否缺少版本槽位 / Whether the original outer object omitted its version slot
	ContainerFilesOnly      bool                                   `json:"containerFilesOnly,omitempty"`      // 原始外层对象是否采用仅文件兼容布局 / Whether the original outer object used the files-only compatibility layout
	ContainerDirectoriesNil bool                                   `json:"containerDirectoriesNil,omitempty"` // 原始目录集合是否为 MessagePack nil / Whether the original directory collection was MessagePack nil
	ContainerFilesNil       bool                                   `json:"containerFilesNil,omitempty"`       // 原始文件集合是否为 MessagePack nil / Whether the original file collection was MessagePack nil
	ContainerFieldCount     *int32                                 `json:"containerFieldCount,omitempty"`     // 原始 VirtualDirectory indexed object 的槽位数 / Slot count of the original VirtualDirectory indexed object
	ContainerFutureSlots    [][]byte                               `json:"containerFutureSlots,omitempty"`    // 当前模型未知的后续 VirtualDirectory 槽位原始值 / Raw later VirtualDirectory slot values unknown to the current model
	ContainerDirectories    map[string]ct.VirtualDirectoryMetadata `json:"containerDirectories,omitempty"`    // 各虚拟目录的线格式元数据 / Wire metadata for each virtual directory
	ContainerVirtualFiles   map[string]ct.VirtualFileMetadata      `json:"containerVirtualFiles,omitempty"`   // 各虚拟文件的线格式元数据 / Wire metadata for each virtual file
	SessionData             *KCESBridgeSessionData                 `json:"sessionData"`                       // session_data 文件中的 EditBridgeSessionData 根值 / EditBridgeSessionData root value in the session_data file
	SessionIDFileData       []byte                                 `json:"sessionIdFileData"`                 // session_id 文件的原始 UTF-8 字节，独立于 MessagePack 中的 sessionId / Raw UTF-8 bytes of the session_id file, independent of the sessionId inside MessagePack
	ExtraFiles              map[string][]byte                      `json:"extraFiles,omitempty"`              // 两个保留名称之外的虚拟文件原始载荷 / Raw payloads of virtual files other than the two reserved names
}

// KCESBridgeSessionData 对应 session_data 虚拟文件中的裸 Standard MessagePack indexed object
// 其已知槽位依次为 Key 0 的版本、Key 1 的 sessionId 和 Key 2 的 HashSet<ulong>
// HideMenuFileNames 是对应 C# IgnoreMember 的可选编辑标注，不在线格式中存在，编码器也不会用它重建或校验 HideMenuFileIDs
// FutureSlots 将每个未知索引槽位保存为一个完整 MessagePack 原始值，以在校验边界和嵌套深度的同时保留未来 KCES 版本字段
// KCESBridgeSessionData corresponds to the bare Standard MessagePack indexed object in the session_data virtual file
// Its known slots are the version at Key 0, sessionId at Key 1, and HashSet<ulong> at Key 2
// HideMenuFileNames is an optional editing annotation for the C# IgnoreMember, never exists on the wire, and is not used by the encoder to rebuild or validate HideMenuFileIDs
// FutureSlots retains each unknown indexed slot as one complete raw MessagePack value so fields from later KCES builds survive while framing and nesting depth are still validated
type KCESBridgeSessionData struct {
	MessagePackRootMetadata           // 根值 nil 与尾部字节元数据 / Root nil and trailing-byte metadata
	FieldCount              *int32    `json:"fieldCount,omitempty"`        // 原始 EditBridgeSessionData indexed object 的槽位数 / Slot count of the original EditBridgeSessionData indexed object
	Version                 int32     `json:"version"`                     // Key 0 的版本，当前 FixVersion 为 0 / Version at Key 0, with a current FixVersion of 0
	SessionID               string    `json:"sessionId"`                   // Key 1 的桥接会话标识 / Bridge session identifier at Key 1
	SessionIDIsNil          bool      `json:"sessionIdIsNil,omitempty"`    // Key 1 是否显式为 MessagePack nil / Whether Key 1 was explicitly MessagePack nil
	HideMenuFileIDs         []uint64  `json:"hideMenuFileIds"`             // Key 2 中需要隐藏的菜单文件 FNV-1a RID 集合 / Set of menu-file FNV-1a RIDs to hide at Key 2
	HideMenuFileNames       *[]string `json:"hideMenuFileNames,omitempty"` // 对应 C# IgnoreMember 的库内编辑标注，不写入线格式 / Library editing annotation corresponding to the C# IgnoreMember, not written to the wire
	FutureSlots             [][]byte  `json:"futureSlots,omitempty"`       // Key 3 起未知槽位的完整 MessagePack 原始值 / Complete raw MessagePack values of unknown slots starting at Key 3
}

// NewKCESBridgeSession 显式创建当前格式对象，解码器不会调用它或注入这些默认值
// NewKCESBridgeSession explicitly creates a current-format object while decoders neither call it nor inject these defaults
func NewKCESBridgeSession(sessionID string) *KCESBridgeSession {
	fieldCount := int32(3)
	return &KCESBridgeSession{
		Format:            KCESBridgeSessionFormat,
		ContainerVersion:  KCESBridgeSessionContainerVersion,
		SessionIDFileData: []byte(sessionID),
		SessionData: &KCESBridgeSessionData{
			FieldCount:      &fieldCount,
			Version:         KCESBridgeSessionDataVersion,
			SessionID:       sessionID,
			HideMenuFileIDs: []uint64{},
		},
	}
}

// IsKCESBridgeSessionData 判断数据是否具有 VirtualDirectory 二进制签名
// 这只是低成本候选检查，需要与其他 VirtualDirectory 区分时仍须调用 DecodeKCESBridgeSession 并验证两个保留文件及其模式
// IsKCESBridgeSessionData reports whether data has the VirtualDirectory binary signature
// This is only a cheap candidate check, so callers distinguishing bridge_session.vd from another VirtualDirectory must still call DecodeKCESBridgeSession and validate its two reserved files and schema
func IsKCESBridgeSessionData(data []byte) bool {
	return len(data) >= len(ct.FileSignature) && bytes.Equal(data[:len(ct.FileSignature)], ct.FileSignature)
}

// DecodeKCESBridgeSession 解析并验证完整 bridge_session.vd VirtualDirectory
// DecodeKCESBridgeSession parses and validates the complete bridge_session.vd VirtualDirectory
func DecodeKCESBridgeSession(data []byte) (*KCESBridgeSession, error) {
	if !IsKCESBridgeSessionData(data) {
		return nil, fmt.Errorf("not a KCES VirtualDirectory")
	}
	table, err := ct.ReadContentTable(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode KCES bridge session VirtualDirectory: %w", err)
	}

	rawSessionData, err := table.GetFileData(kcesBridgeSessionDataFile)
	if err != nil {
		return nil, fmt.Errorf("required virtual file %q: %w", kcesBridgeSessionDataFile, err)
	}
	sessionData, err := decodeKCESBridgeSessionData(rawSessionData)
	if err != nil {
		return nil, fmt.Errorf("decode virtual file %q: %w", kcesBridgeSessionDataFile, err)
	}

	rawSessionID, err := table.GetFileData(kcesBridgeSessionIDFile)
	if err != nil {
		return nil, fmt.Errorf("required virtual file %q: %w", kcesBridgeSessionIDFile, err)
	}
	result := &KCESBridgeSession{
		Format:                  KCESBridgeSessionFormat,
		ContainerVersion:        table.Version,
		ContainerVersionless:    table.Versionless,
		ContainerFilesOnly:      table.FilesOnly,
		ContainerDirectoriesNil: table.DirectoriesNil,
		ContainerFilesNil:       table.FilesNil,
		ContainerFieldCount:     table.FieldCount,
		ContainerFutureSlots:    table.FutureSlots,
		ContainerDirectories:    table.GetVirtualDirectoryMetadata(),
		ContainerVirtualFiles:   table.GetVirtualFileMetadata(),
		SessionData:             sessionData,
		SessionIDFileData:       append(make([]byte, 0, len(rawSessionID)), rawSessionID...),
	}
	for _, name := range table.GetFileNames() {
		if name == kcesBridgeSessionDataFile || name == kcesBridgeSessionIDFile {
			continue
		}
		payload, err := table.GetFileData(name)
		if err != nil {
			return nil, fmt.Errorf("read extra virtual file %q: %w", name, err)
		}
		if result.ExtraFiles == nil {
			result.ExtraFiles = make(map[string][]byte)
		}
		result.ExtraFiles[name] = append([]byte(nil), payload...)
	}
	return result, nil
}

// EncodeKCESBridgeSession 写入表示但不调用内外层版本回调，并复制未知文件与未来 MessagePack 槽位
// EncodeKCESBridgeSession writes the representation without invoking either version callback and copies unknown files and future MessagePack slots
func EncodeKCESBridgeSession(value *KCESBridgeSession) ([]byte, error) {
	if value == nil {
		return nil, fmt.Errorf("nil KCES bridge session")
	}
	if value.Format != "" && value.Format != KCESBridgeSessionFormat {
		return nil, fmt.Errorf("unsupported KCES bridge session format %q", value.Format)
	}
	var rawSessionData []byte
	var err error
	if value.SessionData == nil {
		rawSessionData = []byte{0xc0}
	} else {
		rawSessionData, err = encodeKCESBridgeSessionData(value.SessionData)
		if err != nil {
			return nil, fmt.Errorf("encode virtual file %q: %w", kcesBridgeSessionDataFile, err)
		}
	}

	table := &ct.ContentTable{
		Version:        value.ContainerVersion,
		Versionless:    value.ContainerVersionless,
		FilesOnly:      value.ContainerFilesOnly,
		DirectoriesNil: value.ContainerDirectoriesNil,
		FilesNil:       value.ContainerFilesNil,
		FieldCount:     value.ContainerFieldCount,
		FutureSlots:    value.ContainerFutureSlots,
		Directories:    value.ContainerDirectories,
		Raw:            make([]byte, ct.HeaderSize),
		Files:          make(map[string]ct.VirtualFile),
	}
	if err := table.AddFile(kcesBridgeSessionDataFile, rawSessionData); err != nil {
		return nil, err
	}
	rawSessionID := value.SessionIDFileData
	if rawSessionID == nil && value.SessionData != nil && !value.SessionData.SessionIDIsNil {
		rawSessionID = []byte(value.SessionData.SessionID)
	}
	if err := table.AddFile(kcesBridgeSessionIDFile, append([]byte(nil), rawSessionID...)); err != nil {
		return nil, err
	}
	for name, payload := range value.ExtraFiles {
		if name == kcesBridgeSessionDataFile || name == kcesBridgeSessionIDFile {
			return nil, fmt.Errorf("extraFiles contains reserved virtual file %q", name)
		}
		if err := table.AddFile(name, append([]byte(nil), payload...)); err != nil {
			return nil, err
		}
	}
	if err := table.ApplyVirtualFileMetadata(value.ContainerVirtualFiles); err != nil {
		return nil, fmt.Errorf("KCES bridge session: %w", err)
	}

	var out bytes.Buffer
	if err := ct.WriteContentTable(&out, table); err != nil {
		return nil, fmt.Errorf("encode KCES bridge session VirtualDirectory: %w", err)
	}
	return out.Bytes(), nil
}

// decodeKCESBridgeSessionData 解码 session_data 中的 Standard MessagePack indexed object 并保留未来槽位与根尾部
// decodeKCESBridgeSessionData decodes the Standard MessagePack indexed object in session_data and preserves future slots and root trailing bytes
func decodeKCESBridgeSessionData(data []byte) (*KCESBridgeSessionData, error) {
	reader := simpleEditDataReader{data: data}
	if reader.tryReadNil() {
		trailing, err := messagePackRootTrailingAfterParsed(data, reader.pos, "EditBridgeSessionData")
		if err != nil {
			return nil, err
		}
		if len(trailing) != 0 {
			return &KCESBridgeSessionData{MessagePackRootMetadata: MessagePackRootMetadata{RootNil: true, TrailingData: trailing}}, nil
		}
		return nil, nil
	}
	fieldCount, err := reader.readArrayLength("EditBridgeSessionData")
	if err != nil {
		return nil, err
	}
	if err := reader.requirePossibleValues(fieldCount, "EditBridgeSessionData fields"); err != nil {
		return nil, err
	}

	storedFieldCount := int32(fieldCount)
	result := &KCESBridgeSessionData{FieldCount: &storedFieldCount}
	if fieldCount >= 1 {
		result.Version, err = reader.readInt32("EditBridgeSessionData.version")
		if err != nil {
			return nil, err
		}
	}
	if fieldCount >= 2 {
		if reader.tryReadNil() {
			result.SessionIDIsNil = true
		} else {
			result.SessionID, err = readKCESBridgeSessionString(&reader, "EditBridgeSessionData.sessionId")
			if err != nil {
				return nil, err
			}
		}
	}
	if fieldCount >= 3 {
		if !reader.tryReadNil() {
			setCount, readErr := reader.readArrayLength("EditBridgeSessionData.hideMenuFileIds")
			if readErr != nil {
				return nil, readErr
			}
			if err := reader.requirePossibleValues(setCount, "EditBridgeSessionData.hideMenuFileIds items"); err != nil {
				return nil, err
			}
			result.HideMenuFileIDs = makeKCESCountedSliceForAppend[uint64](uint64(setCount))
			for i := int64(0); i < setCount; i++ {
				id, readErr := readKCESBridgeSessionUInt64(&reader, fmt.Sprintf("EditBridgeSessionData.hideMenuFileIds[%d]", i))
				if readErr != nil {
					return nil, readErr
				}
				result.HideMenuFileIDs = append(result.HideMenuFileIDs, id)
			}
		}
	}
	if fieldCount > 3 {
		result.FutureSlots = makeKCESCountedSliceForAppend[[]byte](uint64(fieldCount - 3))
	}
	for key := int64(3); key < fieldCount; key++ {
		start := reader.pos
		if err := reader.skipValue(0); err != nil {
			return nil, fmt.Errorf("EditBridgeSessionData future Key(%d): %w", key, err)
		}
		result.FutureSlots = append(result.FutureSlots, append([]byte(nil), reader.data[start:reader.pos]...))
	}
	trailing, err := messagePackRootTrailingAfterParsed(data, reader.pos, "EditBridgeSessionData")
	if err != nil {
		return nil, err
	}
	result.TrailingData = trailing
	return result, nil
}

// encodeKCESBridgeSessionData 编码 session_data 根值并阻止 FieldCount 静默丢弃已设置字段
// encodeKCESBridgeSessionData encodes the session_data root while preventing FieldCount from silently discarding populated fields
func encodeKCESBridgeSessionData(value *KCESBridgeSessionData) ([]byte, error) {
	if value == nil {
		return []byte{0xc0}, nil
	}
	if out, handled, err := encodeNilMessagePackRootIfRequested(
		value.MessagePackRootMetadata,
		value.FieldCount != nil || value.Version != 0 || value.SessionID != "" || value.SessionIDIsNil || value.HideMenuFileIDs != nil || len(value.FutureSlots) != 0,
		"EditBridgeSessionData",
	); handled {
		return out, err
	}
	fieldCount, err := resolveIndexedFieldCount(value.FieldCount, 3, value.FutureSlots, "EditBridgeSessionData")
	if err != nil {
		return nil, err
	}
	if fieldCount < 1 && value.Version != 0 {
		return nil, fmt.Errorf("fieldCount %d would discard version=%d", fieldCount, value.Version)
	}
	if fieldCount < 2 && (value.SessionID != "" || value.SessionIDIsNil) {
		return nil, fmt.Errorf("fieldCount %d would discard sessionId", fieldCount)
	}
	if fieldCount < 3 && value.HideMenuFileIDs != nil {
		return nil, fmt.Errorf("fieldCount %d would discard hideMenuFileIds", fieldCount)
	}
	if fieldCount >= 2 && !value.SessionIDIsNil {
		if !utf8.ValidString(value.SessionID) {
			return nil, fmt.Errorf("sessionId is not valid UTF-8")
		}
		if uint64(len(value.SessionID)) > math.MaxUint32 {
			return nil, fmt.Errorf("sessionId has %d UTF-8 bytes, exceeding the MessagePack str32 limit", len(value.SessionID))
		}
	}
	if fieldCount >= 2 && value.SessionIDIsNil && value.SessionID != "" {
		return nil, fmt.Errorf("sessionIdIsNil would discard non-empty sessionId")
	}
	if int64(len(value.HideMenuFileIDs)) > math.MaxInt32 {
		return nil, fmt.Errorf("hideMenuFileIds has %d items, exceeding the C# Int32 array-header limit", len(value.HideMenuFileIDs))
	}

	out := simpleEditDataAppendArrayHeader(nil, fieldCount)
	if fieldCount >= 1 {
		out = simpleEditDataAppendInt32(out, value.Version)
	}
	if fieldCount >= 2 {
		if value.SessionIDIsNil {
			out = append(out, 0xc0)
		} else {
			out = simpleEditDataAppendString(out, value.SessionID)
		}
	}
	if fieldCount >= 3 {
		if value.HideMenuFileIDs == nil {
			out = append(out, 0xc0)
		} else {
			out = simpleEditDataAppendArrayHeader(out, int64(len(value.HideMenuFileIDs)))
			for _, id := range value.HideMenuFileIDs {
				out = appendKCESBridgeSessionUInt64(out, id)
			}
		}
	}
	for _, slot := range value.FutureSlots {
		out = append(out, slot...)
	}
	return appendMessagePackRootTrailing(out, value.MessagePackRootMetadata), nil
}

// KCESBridgeMenuFileID 显式计算游戏回调对一个非 nil 名称生成的值
// 计算依次提取 Windows 文件名、将扩展名替换为 .menu、执行与区域无关的 Unicode 小写转换，再计算 AssetManager 的 UTF-8 FNV-1a
// C# 源码调用 ToLower，严格来说依赖 bridge_session.vd 未保存的 CurrentCulture，因此土耳其语 I 等区域敏感情况应直接使用线格式 ID
// 编码器不会调用此辅助函数，也不会从 IgnoreMember 名称标注推导 ID，C# 的空字符串与 nil 字符串都会生成 .menu 的结果
// KCESBridgeMenuFileID explicitly computes the value produced by the game callback for one non-nil name
// The calculation extracts a Windows filename, replaces its extension with .menu, applies locale-independent Unicode lower-casing, and then computes AssetManager UTF-8 FNV-1a
// The C# source calls ToLower and therefore technically depends on CurrentCulture which bridge_session.vd does not store, so locale-sensitive cases such as Turkish I should use explicit wire IDs
// The encoder does not call this helper or derive IDs from the IgnoreMember name annotation, and empty and nil C# strings both produce the result for .menu
func KCESBridgeMenuFileID(path string) uint64 {
	baseStart := int64(strings.LastIndexAny(path, `/\\`))
	base := path[baseStart+1:]
	if dot := int64(strings.LastIndexByte(base, '.')); dot >= 0 {
		base = base[:dot]
	}
	canonical := strings.ToLower(base + ".menu")
	return kcesFNV1a64([]byte(canonical))
}

// kcesFNV1a64 按游戏 AssetManager 的约定计算字节串 FNV-1a 64 位哈希，空输入返回零
// kcesFNV1a64 computes the FNV-1a 64-bit hash using the game AssetManager convention and returns zero for empty input
func kcesFNV1a64(data []byte) uint64 {
	if len(data) == 0 {
		return 0
	}
	hash := uint64(14695981039346656037)
	for _, b := range data {
		hash ^= uint64(b)
		hash *= 1099511628211
	}
	return hash
}

// readKCESBridgeSessionString 使用共享 MessagePack 读取器解码并验证会话字符串
// readKCESBridgeSessionString decodes and validates a session string with the shared MessagePack reader
func readKCESBridgeSessionString(reader *simpleEditDataReader, path string) (string, error) {
	return reader.readString(path)
}

// readKCESBridgeSessionUInt64 接受 MessagePack 的无符号编码及非负有符号整数编码并返回 UInt64
// readKCESBridgeSessionUInt64 accepts unsigned MessagePack encodings and non-negative signed integer encodings and returns a UInt64
func readKCESBridgeSessionUInt64(reader *simpleEditDataReader, path string) (uint64, error) {
	marker, err := reader.readByte(path)
	if err != nil {
		return 0, err
	}
	if marker <= 0x7f {
		return uint64(marker), nil
	}
	if marker >= 0xe0 {
		return 0, fmt.Errorf("%s is negative and cannot be read as UInt64", path)
	}
	switch marker {
	case 0xcc:
		value, err := reader.readByte(path + " uint8")
		return uint64(value), err
	case 0xcd:
		value, err := reader.readBytes(2, path+" uint16")
		if err != nil {
			return 0, err
		}
		return uint64(binary.BigEndian.Uint16(value)), nil
	case 0xce:
		value, err := reader.readBytes(4, path+" uint32")
		if err != nil {
			return 0, err
		}
		return uint64(binary.BigEndian.Uint32(value)), nil
	case 0xcf:
		value, err := reader.readBytes(8, path+" uint64")
		if err != nil {
			return 0, err
		}
		return binary.BigEndian.Uint64(value), nil
	case 0xd0:
		value, err := reader.readByte(path + " int8")
		if err != nil {
			return 0, err
		}
		signed := int8(value)
		if signed < 0 {
			return 0, fmt.Errorf("%s is negative and cannot be read as UInt64", path)
		}
		return uint64(signed), nil
	case 0xd1:
		value, err := reader.readBytes(2, path+" int16")
		if err != nil {
			return 0, err
		}
		signed := int16(binary.BigEndian.Uint16(value))
		if signed < 0 {
			return 0, fmt.Errorf("%s is negative and cannot be read as UInt64", path)
		}
		return uint64(signed), nil
	case 0xd2:
		value, err := reader.readBytes(4, path+" int32")
		if err != nil {
			return 0, err
		}
		signed := int32(binary.BigEndian.Uint32(value))
		if signed < 0 {
			return 0, fmt.Errorf("%s is negative and cannot be read as UInt64", path)
		}
		return uint64(signed), nil
	case 0xd3:
		value, err := reader.readBytes(8, path+" int64")
		if err != nil {
			return 0, err
		}
		signed := int64(binary.BigEndian.Uint64(value))
		if signed < 0 {
			return 0, fmt.Errorf("%s is negative and cannot be read as UInt64", path)
		}
		return uint64(signed), nil
	default:
		return 0, fmt.Errorf("%s must be a MessagePack UInt64-compatible integer, got marker 0x%02x", path, marker)
	}
}

// appendKCESBridgeSessionUInt64 以可表示给定值的最短 MessagePack 无符号整数形式追加 UInt64
// appendKCESBridgeSessionUInt64 appends a UInt64 using the shortest MessagePack unsigned integer form that can represent it
func appendKCESBridgeSessionUInt64(dst []byte, value uint64) []byte {
	switch {
	case value <= 0x7f:
		return append(dst, byte(value))
	case value <= math.MaxUint8:
		return append(dst, 0xcc, byte(value))
	case value <= math.MaxUint16:
		return append(dst, 0xcd, byte(value>>8), byte(value))
	case value <= math.MaxUint32:
		dst = append(dst, 0xce, 0, 0, 0, 0)
		binary.BigEndian.PutUint32(dst[len(dst)-4:], uint32(value))
		return dst
	default:
		dst = append(dst, 0xcf, 0, 0, 0, 0, 0, 0, 0, 0)
		binary.BigEndian.PutUint64(dst[len(dst)-8:], value)
		return dst
	}
}
