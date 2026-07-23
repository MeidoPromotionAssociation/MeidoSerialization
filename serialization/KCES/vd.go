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
// KCES 角色编辑桥接会话容器，使用 VirtualDirectory 保存 session_data 与 session_id。
// 外层容器当前版本为 1000；session_data 是未压缩的 MessagePack indexed object。
//
// .vd (bridge_session.vd)
// KCES character-edit bridge session container using VirtualDirectory to store session_data and session_id.
// The current outer-container version is 1000; session_data is an uncompressed MessagePack indexed object.

const (
	// KCESBridgeSessionFormat identifies the editable representation of
	// CRCEdit.EditBridgeSessionData's bridge_session.vd container.
	KCESBridgeSessionFormat = "kces-bridge-session"

	// Current versions used by the explicit constructor.
	KCESBridgeSessionContainerVersion = 1000
	KCESBridgeSessionDataVersion      = 0

	kcesBridgeSessionDataFile = "session_data"
	kcesBridgeSessionIDFile   = "session_id"
)

// KCESBridgeSession is the complete VirtualDirectory representation written
// to bridge_session.vd. The two files understood by EditBridgeSessionData are
// exposed through SessionData; every other virtual file is retained verbatim.
type KCESBridgeSession struct {
	Format                  string                                 `json:"format"`
	ContainerVersion        int32                                  `json:"containerVersion"`
	ContainerVersionless    bool                                   `json:"containerVersionless,omitempty"`
	ContainerFilesOnly      bool                                   `json:"containerFilesOnly,omitempty"`
	ContainerDirectoriesNil bool                                   `json:"containerDirectoriesNil,omitempty"`
	ContainerFilesNil       bool                                   `json:"containerFilesNil,omitempty"`
	ContainerFieldCount     *int32                                 `json:"containerFieldCount,omitempty"`
	ContainerFutureSlots    [][]byte                               `json:"containerFutureSlots,omitempty"`
	ContainerDirectories    map[string]ct.VirtualDirectoryMetadata `json:"containerDirectories,omitempty"`
	ContainerVirtualFiles   map[string]ct.VirtualFileMetadata      `json:"containerVirtualFiles,omitempty"`
	SessionData             *KCESBridgeSessionData                 `json:"sessionData"`
	SessionIDFileData       []byte                                 `json:"sessionIdFileData"`
	ExtraFiles              map[string][]byte                      `json:"extraFiles,omitempty"`
}

// KCESBridgeSessionData matches the bare Standard MessagePack indexed object
// stored in the session_data virtual file:
//
//	[Key(0) version, Key(1) sessionId, Key(2) HashSet<ulong>, ...future keys]
//
// HideMenuFileNames is an optional editing annotation for the C# IgnoreMember.
// It never exists on the wire and the encoder deliberately does not use it to
// rebuild or validate HideMenuFileIDs.
//
// FutureSlots retains each unknown indexed slot as one complete raw
// MessagePack value. This avoids rewriting or discarding fields added by a
// later KCES build while still validating their framing and nesting depth.
type KCESBridgeSessionData struct {
	MessagePackRootMetadata
	FieldCount        *int32    `json:"fieldCount,omitempty"`
	Version           int32     `json:"version"`
	SessionID         string    `json:"sessionId"`
	SessionIDIsNil    bool      `json:"sessionIdIsNil,omitempty"`
	HideMenuFileIDs   []uint64  `json:"hideMenuFileIds"`
	HideMenuFileNames *[]string `json:"hideMenuFileNames,omitempty"`
	FutureSlots       [][]byte  `json:"futureSlots,omitempty"`
}

// NewKCESBridgeSession creates a current-format object explicitly. Decoders do
// not call this constructor or inject any of these defaults.
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

// IsKCESBridgeSessionData reports whether data has VirtualDirectory's binary
// signature. It is intentionally only a cheap candidate check; callers that
// need to distinguish bridge_session.vd from another VirtualDirectory must
// call DecodeKCESBridgeSession and require its two reserved files/schema.
func IsKCESBridgeSessionData(data []byte) bool {
	return len(data) >= len(ct.FileSignature) && bytes.Equal(data[:len(ct.FileSignature)], ct.FileSignature)
}

// DecodeKCESBridgeSession parses and validates the complete
// bridge_session.vd VirtualDirectory.
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

// EncodeKCESBridgeSession writes the representation without invoking either
// version callback. Unknown files/future MessagePack slots are copied.
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

// KCESBridgeMenuFileID explicitly computes the value the game callback would
// produce for one non-nil name: Windows file-name extraction, extension replacement with
// ".menu", locale-independent Unicode lower-casing, then AssetManager's UTF-8
// FNV-1a. The C# source calls ToLower() and therefore technically depends on
// CurrentCulture, which is not stored in bridge_session.vd. Locale-sensitive
// edge cases (notably Turkish I) should use explicit wire IDs. The encoder does
// not call this helper or derive IDs from the IgnoreMember name annotation.
// Empty and nil C# strings have the same callback result (".menu"); Go's
// string value therefore fully expresses the resulting wire ID.
func KCESBridgeMenuFileID(path string) uint64 {
	baseStart := strings.LastIndexAny(path, `/\\`)
	base := path[baseStart+1:]
	if dot := strings.LastIndexByte(base, '.'); dot >= 0 {
		base = base[:dot]
	}
	canonical := strings.ToLower(base + ".menu")
	return kcesFNV1a64([]byte(canonical))
}

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

func readKCESBridgeSessionString(reader *simpleEditDataReader, path string) (string, error) {
	return reader.readString(path)
}

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
