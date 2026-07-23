package KCES

import (
	"bytes"
	"fmt"
	"math"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/utilities"
)

// .sad (SAVED_ATTACH_DATA)
// KCES/GP03 导出的部件附着信息文件，使用 BinaryWriter 编码外层列表与每条附着记录
// 外层版本为 2000，当前记录版本为 2001，并兼容无显式记录版本的旧版 2000 布局
// .sad (SAVED_ATTACH_DATA)
// KCES/GP03 exported part-attachment file encoded by BinaryWriter as an outer list of attachment records
// The outer version is 2000, while current records use version 2001 and the implicit legacy 2000 layout remains supported

const (
	// SavedAttachSignature 由 ExportCM.ExportAttachData 写在文件版本和记录数之前
	// SavedAttachSignature is written by ExportCM.ExportAttachData before the file-level version and record count
	SavedAttachSignature = "SAVED_ATTACH_DATA"
	// SavedAttachFileVersion 是 KCES 1.34.4 唯一写出的外层版本
	// SavedAttachFileVersion is the only outer version emitted by KCES 1.34.4
	SavedAttachFileVersion int32 = 2000
	// SavedAttachRecordVersion 写在每条当前 SavedAttachData 记录的空可空字符串哨兵之后
	// SavedAttachRecordVersion is emitted inside every current SavedAttachData record after an empty nullable-string sentinel
	SavedAttachRecordVersion int32 = 2001
	// KCESSavedAttachFormat 标识可编辑 JSON 表示
	// KCESSavedAttachFormat identifies the editable JSON representation
	KCESSavedAttachFormat = "kces-saved-attach"

	// 最短的可读隐式 v2000 记录占 36 字节，包含单字符部件名和空枚举字符串载荷
	// The shortest structurally readable implicit-v2000 record occupies 36 bytes with a one-character part name and empty enum-string payloads
	minSavedAttachItemBytes = 36
)

// SavedAttachFile 表示一个导出的 KCES/GP03 .sad 文件
// SavedAttachFile represents one exported KCES/GP03 .sad file
type SavedAttachFile struct {
	Format       string            `json:"format"`                 // JSON 表示格式标识 / JSON representation format identifier
	Signature    string            `json:"signature"`              // 文件签名 SAVED_ATTACH_DATA / File signature SAVED_ATTACH_DATA
	Version      int32             `json:"version"`                // 外层文件版本 / Outer file version
	Items        []SavedAttachData `json:"items"`                  // 部件附着记录 / Part-attachment records
	TrailingData []byte            `json:"trailingData,omitempty"` // 游戏读取声明记录后忽略的尾部字节 / Trailing bytes ignored by the game after reading the declared records
}

// SavedAttachData 对应游戏 SavedAttachData.Serialize 和 Deserialize 实现的 BinaryWriter 布局
// 指针字段保留 WriteNaS 与两个 PosRotScale 值使用的 null 和非 null 标志
// SavedAttachData mirrors the BinaryWriter layout implemented by the game's SavedAttachData.Serialize and Deserialize methods
// Pointer fields preserve the null and non-null flags used by WriteNaS and both PosRotScale values
type SavedAttachData struct {
	Version                int32                             `json:"version"`                      // 记录布局版本，隐式旧布局为 2000 / Record layout version, with 2000 for the implicit legacy layout
	ExplicitVersion        bool                              `json:"explicitVersion,omitempty"`    // 是否在线格式中写入空字符串哨兵和显式版本 / Whether the wire stores an empty-string sentinel and explicit version
	PartName               *string                           `json:"partName"`                     // 附着记录的部件标签名 / Part tag name for the attachment record
	Enabled                bool                              `json:"enabled"`                      // 该附着记录是否启用 / Whether this attachment record is enabled
	MyRID                  uint64                            `json:"myRid"`                        // 源部件菜单 RID，用于确认源菜单仍匹配 / Source-part menu RID used to verify that the source menu still matches
	MySlotID               string                            `json:"mySlotId"`                     // 源部件的 TBody.SlotID 名称 / TBody.SlotID name of the source part
	TargetRID              uint64                            `json:"targetRid"`                    // 目标部件菜单 RID，用于确认目标菜单仍匹配 / Target-part menu RID used to verify that the target menu still matches
	TargetSlotID           string                            `json:"targetSlotId"`                 // 目标部件的 TBody.SlotID 名称 / TBody.SlotID name of the target part
	TargetSlotNo           int32                             `json:"targetSlotNo"`                 // 目标槽位中的子部件编号，记录版本 2001 起存在 / Target slot sub-part number, present since record version 2001
	TargetAttachPointName  *string                           `json:"targetAttachPointName"`        // 目标新附着点名称 / Target new-attachment-point name
	TargetVertexCount      int32                             `json:"targetVertexCount"`            // 保存时的目标网格顶点数，用于拒绝已变化网格 / Target mesh vertex count when saved, used to reject a changed mesh
	TargetVertexIndex      int32                             `json:"targetVertexIndex"`            // 线格式保存的目标顶点索引，当前加载路径未直接消费 / Target vertex index stored on the wire and not directly consumed by the current loading path
	NewAttachVertexIndices []int32                           `json:"newAttachVertexIndices"`       // 重建新附着点使用的顶点索引 / Vertex indices used to rebuild the new attachment point
	PRS2                   *SavedAttachPosRotScale           `json:"prs2"`                         // center_tr2 的局部位置与旋转，游戏也保存缩放槽位 / Local position and rotation of center_tr2, with a scale slot also stored by the game
	PRS3                   *SavedAttachPosRotScale           `json:"prs3"`                         // center_tr3 的完整局部变换 / Complete local transform of center_tr3
	BoneAttachedHierarchy  map[string]SavedAttachPosRotScale `json:"boneAttachedHierarchy"`        // 按骨骼名保存的附着层级局部变换 / Attachment-hierarchy local transforms keyed by bone name
	BoneHierarchyOrder     []string                          `json:"boneHierarchyOrder,omitempty"` // 骨骼层级映射在线格式中的顺序 / Bone-hierarchy map order on the wire
	BoneAttachEdited       bool                              `json:"boneAttachEdited"`             // 骨骼附着层级是否已在编辑器中修改 / Whether the bone attachment hierarchy was edited
}

// SavedAttachPosRotScale 是 Scourt.Utility.UnityUtility.PosRotScale 的二进制形式，依次保存 Vector3 位置、Vector3 缩放和 Quaternion 旋转
// SavedAttachPosRotScale is the binary form of Scourt.Utility.UnityUtility.PosRotScale, storing Vector3 position, Vector3 scale, and Quaternion rotation in order
type SavedAttachPosRotScale struct {
	Position Vector3 `json:"position"` // 局部位置 / Local position
	Scale    Vector3 `json:"scale"`    // 局部缩放 / Local scale
	Rotation Vector4 `json:"rotation"` // 局部旋转四元数 / Local rotation quaternion
}

// DecodeSavedAttach 解码导出的 .sad 文件，槽位 ID 保持为不透明线格式字符串，枚举解释由消费游戏负责
// DecodeSavedAttach decodes an exported .sad file while keeping slot IDs as opaque wire strings whose enum interpretation belongs to the consuming game
func DecodeSavedAttach(data []byte) (*SavedAttachFile, error) {
	r := bytes.NewReader(data)
	br := stream.NewBinaryReader(r)

	signature, err := readSavedAttachString(br, "signature")
	if err != nil {
		return nil, err
	}
	if signature != SavedAttachSignature {
		return nil, fmt.Errorf("invalid saved-attach signature %q", signature)
	}

	version, err := br.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read saved-attach version: %w", err)
	}

	count, err := br.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read saved-attach item count: %w", err)
	}
	if count < 0 {
		return nil, fmt.Errorf("negative saved-attach item count %d", count)
	}
	if int64(count) > int64(r.Len()/minSavedAttachItemBytes) {
		return nil, fmt.Errorf("saved-attach item count %d cannot fit in %d remaining bytes", count, r.Len())
	}

	items := makeKCESCountedSliceForAppend[SavedAttachData](uint64(count))
	for i := int32(0); i < count; i++ {
		item, decodeErr := decodeSavedAttachItem(br, r, int64(i))
		if decodeErr != nil {
			return nil, decodeErr
		}
		items = append(items, item)
	}
	result := &SavedAttachFile{
		Format:    KCESSavedAttachFormat,
		Signature: signature,
		Version:   version,
		Items:     items,
	}
	if r.Len() != 0 {
		result.TrailingData = make([]byte, r.Len())
		if _, err := r.Read(result.TrailingData); err != nil {
			return nil, fmt.Errorf("read saved-attach trailing data: %w", err)
		}
	}
	return result, nil
}

// decodeSavedAttachItem 使用游戏槽位字符串校验读取一条附着记录
// decodeSavedAttachItem reads one attachment record using game slot-string validation
func decodeSavedAttachItem(br *stream.BinaryReader, remaining *bytes.Reader, index int64) (SavedAttachData, error) {
	return decodeSavedAttachItemWithSlotValidator(br, remaining, index, validateSavedAttachSlotID)
}

// decodeSavedAttachItemWithSlotValidator 使用调用方提供的槽位校验器读取一条附着记录
// decodeSavedAttachItemWithSlotValidator reads one attachment record using a caller-supplied slot validator
func decodeSavedAttachItemWithSlotValidator(br *stream.BinaryReader, remaining *bytes.Reader, index int64, validateSlot func(string, string) error) (SavedAttachData, error) {
	prefix := fmt.Sprintf("saved-attach item[%d]", index)
	firstName, err := readSavedAttachNullableString(br, prefix+" version sentinel/partName")
	if err != nil {
		return SavedAttachData{}, err
	}

	item := SavedAttachData{Version: SavedAttachFileVersion}
	if firstName == nil || *firstName == "" {
		item.ExplicitVersion = true
		item.Version, err = br.ReadInt32()
		if err != nil {
			return SavedAttachData{}, fmt.Errorf("read %s version: %w", prefix, err)
		}
		item.PartName, err = readSavedAttachNullableString(br, prefix+" partName")
		if err != nil {
			return SavedAttachData{}, err
		}
	} else {
		// 旧版 2000 布局直接以非空部件名开始且不包含 TargetSlotNo
		// The legacy 2000 layout starts directly with a nonempty part name and does not contain TargetSlotNo
		item.PartName = firstName
	}

	if item.Enabled, err = br.ReadBool(); err != nil {
		return SavedAttachData{}, fmt.Errorf("read %s enabled: %w", prefix, err)
	}
	if item.MyRID, err = br.ReadUInt64(); err != nil {
		return SavedAttachData{}, fmt.Errorf("read %s myRid: %w", prefix, err)
	}
	if item.MySlotID, err = readSavedAttachSlotIDWithValidator(br, prefix+" mySlotId", validateSlot); err != nil {
		return SavedAttachData{}, err
	}
	if item.TargetRID, err = br.ReadUInt64(); err != nil {
		return SavedAttachData{}, fmt.Errorf("read %s targetRid: %w", prefix, err)
	}
	if item.TargetSlotID, err = readSavedAttachSlotIDWithValidator(br, prefix+" targetSlotId", validateSlot); err != nil {
		return SavedAttachData{}, err
	}
	if item.Version >= SavedAttachRecordVersion {
		if item.TargetSlotNo, err = br.ReadInt32(); err != nil {
			return SavedAttachData{}, fmt.Errorf("read %s targetSlotNo: %w", prefix, err)
		}
	}
	if item.TargetAttachPointName, err = readSavedAttachNullableString(br, prefix+" targetAttachPointName"); err != nil {
		return SavedAttachData{}, err
	}
	if item.TargetVertexCount, err = br.ReadInt32(); err != nil {
		return SavedAttachData{}, fmt.Errorf("read %s targetVertexCount: %w", prefix, err)
	}
	if item.TargetVertexIndex, err = br.ReadInt32(); err != nil {
		return SavedAttachData{}, fmt.Errorf("read %s targetVertexIndex: %w", prefix, err)
	}
	if item.NewAttachVertexIndices, err = readSavedAttachInt32Slice(br, remaining, prefix+" newAttachVertexIndices"); err != nil {
		return SavedAttachData{}, err
	}
	if item.PRS2, err = readSavedAttachOptionalPRS(br, prefix+" prs2"); err != nil {
		return SavedAttachData{}, err
	}
	if item.PRS3, err = readSavedAttachOptionalPRS(br, prefix+" prs3"); err != nil {
		return SavedAttachData{}, err
	}
	if item.BoneAttachedHierarchy, item.BoneHierarchyOrder, err = readSavedAttachHierarchy(br, remaining, prefix+" boneAttachedHierarchy"); err != nil {
		return SavedAttachData{}, err
	}
	if item.BoneAttachEdited, err = br.ReadBool(); err != nil {
		return SavedAttachData{}, fmt.Errorf("read %s boneAttachEdited: %w", prefix, err)
	}
	return item, nil
}

// EncodeSavedAttach 按每个条目的 Version 写出当前或旧版布局
// 2000 记录通常使用直接旧布局，但部件名为 nil 或空时改用游戏接受的显式版本哨兵，因为直接形式会被误判为哨兵，外层与记录版本包括零值和未来值均原样写出
// EncodeSavedAttach emits the current or legacy layout selected by each item's Version
// A 2000 record normally uses the direct legacy layout but uses the explicit-version sentinel accepted by the game for a nil or empty part name because the direct form would be mistaken for a sentinel, while outer and record versions including zero and future values are written unchanged
func EncodeSavedAttach(value *SavedAttachFile) ([]byte, error) {
	if value == nil {
		return nil, fmt.Errorf("nil saved-attach file")
	}
	if value.Format != "" && value.Format != KCESSavedAttachFormat {
		return nil, fmt.Errorf("unsupported saved-attach JSON format %q", value.Format)
	}
	signature := value.Signature
	if signature != SavedAttachSignature {
		return nil, fmt.Errorf("invalid saved-attach signature %q", signature)
	}
	version := value.Version
	if len(value.Items) > math.MaxInt32 {
		return nil, fmt.Errorf("saved-attach item count %d exceeds Int32", len(value.Items))
	}
	count := int32(len(value.Items))

	var out bytes.Buffer
	bw := stream.NewBinaryWriter(&out)
	if err := bw.WriteString(signature); err != nil {
		return nil, fmt.Errorf("write saved-attach signature: %w", err)
	}
	if err := bw.WriteInt32(version); err != nil {
		return nil, fmt.Errorf("write saved-attach version: %w", err)
	}
	if err := bw.WriteInt32(count); err != nil {
		return nil, fmt.Errorf("write saved-attach item count: %w", err)
	}
	for i := range value.Items {
		if err := encodeSavedAttachItem(bw, &value.Items[i], int64(i)); err != nil {
			return nil, err
		}
	}
	if len(value.TrailingData) != 0 {
		if _, err := out.Write(value.TrailingData); err != nil {
			return nil, fmt.Errorf("write saved-attach trailing data: %w", err)
		}
	}
	return out.Bytes(), nil
}

// NewSavedAttachFile 显式创建当前外层文件头，记录版本仍由调用方控制且编码时不会升级
// NewSavedAttachFile explicitly creates the current outer header while record versions remain caller-controlled and are never upgraded during encoding
func NewSavedAttachFile() *SavedAttachFile {
	return &SavedAttachFile{
		Format:    KCESSavedAttachFormat,
		Signature: SavedAttachSignature,
		Version:   SavedAttachFileVersion,
	}
}

// encodeSavedAttachItem 使用游戏槽位字符串校验写入一条附着记录
// encodeSavedAttachItem writes one attachment record using game slot-string validation
func encodeSavedAttachItem(bw *stream.BinaryWriter, source *SavedAttachData, index int64) error {
	return encodeSavedAttachItemWithSlotValidator(bw, source, index, validateSavedAttachSlotID)
}

// encodeSavedAttachItemWithSlotValidator 使用调用方提供的槽位校验器写入一条附着记录
// encodeSavedAttachItemWithSlotValidator writes one attachment record using a caller-supplied slot validator
func encodeSavedAttachItemWithSlotValidator(bw *stream.BinaryWriter, source *SavedAttachData, index int64, validateSlot func(string, string) error) error {
	prefix := fmt.Sprintf("saved-attach item[%d]", index)
	version := source.Version
	if err := validateSavedAttachNullableString(source.PartName, prefix+" partName"); err != nil {
		return err
	}
	if err := validateSlot(source.MySlotID, prefix+" mySlotId"); err != nil {
		return err
	}
	if err := validateSlot(source.TargetSlotID, prefix+" targetSlotId"); err != nil {
		return err
	}
	if err := validateSavedAttachNullableString(source.TargetAttachPointName, prefix+" targetAttachPointName"); err != nil {
		return err
	}
	if len(source.NewAttachVertexIndices) > math.MaxInt32 {
		return fmt.Errorf("%s newAttachVertexIndices length %d exceeds Int32", prefix, len(source.NewAttachVertexIndices))
	}
	if len(source.BoneAttachedHierarchy) > math.MaxInt32 {
		return fmt.Errorf("%s boneAttachedHierarchy length %d exceeds Int32", prefix, len(source.BoneAttachedHierarchy))
	}
	if version < SavedAttachRecordVersion && source.TargetSlotNo != 0 {
		return fmt.Errorf("%s targetSlotNo is unavailable in record version %d", prefix, version)
	}

	writeExplicitVersion := source.ExplicitVersion || version != SavedAttachFileVersion ||
		(source.PartName == nil || *source.PartName == "")
	if writeExplicitVersion {
		empty := ""
		if err := writeSavedAttachNullableString(bw, &empty); err != nil {
			return fmt.Errorf("write %s version sentinel: %w", prefix, err)
		}
		if err := bw.WriteInt32(version); err != nil {
			return fmt.Errorf("write %s version: %w", prefix, err)
		}
	}
	if err := writeSavedAttachNullableString(bw, source.PartName); err != nil {
		return fmt.Errorf("write %s partName: %w", prefix, err)
	}
	if err := bw.WriteBool(source.Enabled); err != nil {
		return fmt.Errorf("write %s enabled: %w", prefix, err)
	}
	if err := bw.WriteUInt64(source.MyRID); err != nil {
		return fmt.Errorf("write %s myRid: %w", prefix, err)
	}
	if err := bw.WriteString(source.MySlotID); err != nil {
		return fmt.Errorf("write %s mySlotId: %w", prefix, err)
	}
	if err := bw.WriteUInt64(source.TargetRID); err != nil {
		return fmt.Errorf("write %s targetRid: %w", prefix, err)
	}
	if err := bw.WriteString(source.TargetSlotID); err != nil {
		return fmt.Errorf("write %s targetSlotId: %w", prefix, err)
	}
	if version >= SavedAttachRecordVersion {
		if err := bw.WriteInt32(source.TargetSlotNo); err != nil {
			return fmt.Errorf("write %s targetSlotNo: %w", prefix, err)
		}
	}
	if err := writeSavedAttachNullableString(bw, source.TargetAttachPointName); err != nil {
		return fmt.Errorf("write %s targetAttachPointName: %w", prefix, err)
	}
	if err := bw.WriteInt32(source.TargetVertexCount); err != nil {
		return fmt.Errorf("write %s targetVertexCount: %w", prefix, err)
	}
	if err := bw.WriteInt32(source.TargetVertexIndex); err != nil {
		return fmt.Errorf("write %s targetVertexIndex: %w", prefix, err)
	}
	if err := writeSavedAttachInt32Slice(bw, source.NewAttachVertexIndices); err != nil {
		return fmt.Errorf("write %s newAttachVertexIndices: %w", prefix, err)
	}
	if err := writeSavedAttachOptionalPRS(bw, source.PRS2); err != nil {
		return fmt.Errorf("write %s prs2: %w", prefix, err)
	}
	if err := writeSavedAttachOptionalPRS(bw, source.PRS3); err != nil {
		return fmt.Errorf("write %s prs3: %w", prefix, err)
	}
	if err := writeSavedAttachHierarchy(bw, source.BoneAttachedHierarchy, source.BoneHierarchyOrder, prefix); err != nil {
		return err
	}
	if err := bw.WriteBool(source.BoneAttachEdited); err != nil {
		return fmt.Errorf("write %s boneAttachEdited: %w", prefix, err)
	}
	return nil
}

// readSavedAttachString 读取一个 BinaryWriter 字符串并附加字段路径错误信息
// readSavedAttachString reads one BinaryWriter string and adds field-path error context
func readSavedAttachString(br *stream.BinaryReader, path string) (string, error) {
	value, err := br.ReadString()
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return value, nil
}

// readSavedAttachNullableString 读取 WriteNaS 使用的存在标志和可空字符串
// readSavedAttachNullableString reads the presence flag and nullable string used by WriteNaS
func readSavedAttachNullableString(br *stream.BinaryReader, path string) (*string, error) {
	present, err := br.ReadBool()
	if err != nil {
		return nil, fmt.Errorf("read %s presence: %w", path, err)
	}
	if !present {
		return nil, nil
	}
	value, err := readSavedAttachString(br, path)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

// writeSavedAttachNullableString 写入 WriteNaS 布局的存在标志和可空字符串
// writeSavedAttachNullableString writes a presence flag and nullable string using the WriteNaS layout
func writeSavedAttachNullableString(bw *stream.BinaryWriter, value *string) error {
	if err := bw.WriteBool(value != nil); err != nil {
		return err
	}
	if value != nil {
		return bw.WriteString(*value)
	}
	return nil
}

// validateSavedAttachNullableString 验证非 nil 可空字符串的线格式长度
// validateSavedAttachNullableString validates the wire length of a non-nil nullable string
func validateSavedAttachNullableString(value *string, path string) error {
	if value != nil {
		return validateSavedAttachString(*value, path)
	}
	return nil
}

// validateSavedAttachString 验证字符串字节数可由游戏长度字段表示
// validateSavedAttachString verifies that a string byte count is representable by the game's length field
func validateSavedAttachString(value, path string) error {
	if uint64(len(value)) > uint64(math.MaxInt32) {
		return fmt.Errorf("%s is %d bytes, exceeds Int32", path, len(value))
	}
	return nil
}

// readSavedAttachSlotID 使用游戏槽位字符串校验读取一个槽位 ID
// readSavedAttachSlotID reads one slot ID using game slot-string validation
func readSavedAttachSlotID(br *stream.BinaryReader, path string) (string, error) {
	return readSavedAttachSlotIDWithValidator(br, path, validateSavedAttachSlotID)
}

// readSavedAttachSlotIDWithValidator 使用调用方提供的校验器读取槽位 ID 字符串
// readSavedAttachSlotIDWithValidator reads a slot-ID string using a caller-supplied validator
func readSavedAttachSlotIDWithValidator(br *stream.BinaryReader, path string, validateSlot func(string, string) error) (string, error) {
	value, err := readSavedAttachString(br, path)
	if err != nil {
		return "", err
	}
	if err := validateSlot(value, path); err != nil {
		return "", err
	}
	return value, nil
}

// validateSavedAttachSlotID 验证槽位 ID 字符串可由线格式表示
// validateSavedAttachSlotID verifies that a slot-ID string is representable on the wire
func validateSavedAttachSlotID(value, path string) error {
	return validateSavedAttachString(value, path)
}

// readSavedAttachInt32Slice 读取带存在标志和 Int32 数量的可空 Int32 切片
// readSavedAttachInt32Slice reads a nullable Int32 slice with a presence flag and Int32 count
func readSavedAttachInt32Slice(br *stream.BinaryReader, remaining *bytes.Reader, path string) ([]int32, error) {
	present, err := br.ReadBool()
	if err != nil {
		return nil, fmt.Errorf("read %s presence: %w", path, err)
	}
	if !present {
		return nil, nil
	}
	count, err := br.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read %s count: %w", path, err)
	}
	if count < 0 {
		return nil, fmt.Errorf("negative %s count %d", path, count)
	}
	if int64(count) > int64(remaining.Len()/4) {
		return nil, fmt.Errorf("%s count %d cannot fit in %d remaining bytes", path, count, remaining.Len())
	}
	values := makeKCESCountedSliceForAppend[int32](uint64(count))
	for i := int32(0); i < count; i++ {
		value, readErr := br.ReadInt32()
		if readErr != nil {
			return nil, fmt.Errorf("read %s[%d]: %w", path, i, readErr)
		}
		values = append(values, value)
	}
	return values, nil
}

// writeSavedAttachInt32Slice 写入带存在标志和 Int32 数量的可空 Int32 切片
// writeSavedAttachInt32Slice writes a nullable Int32 slice with a presence flag and Int32 count
func writeSavedAttachInt32Slice(bw *stream.BinaryWriter, values []int32) error {
	if err := bw.WriteBool(values != nil); err != nil {
		return err
	}
	if values == nil {
		return nil
	}
	if len(values) > math.MaxInt32 {
		return fmt.Errorf("length %d exceeds Int32", len(values))
	}
	if err := bw.WriteInt32(int32(len(values))); err != nil {
		return err
	}
	for _, value := range values {
		if err := bw.WriteInt32(value); err != nil {
			return err
		}
	}
	return nil
}

// readSavedAttachOptionalPRS 读取存在标志及可选位置旋转缩放值
// readSavedAttachOptionalPRS reads a presence flag and optional position-rotation-scale value
func readSavedAttachOptionalPRS(br *stream.BinaryReader, path string) (*SavedAttachPosRotScale, error) {
	present, err := br.ReadBool()
	if err != nil {
		return nil, fmt.Errorf("read %s presence: %w", path, err)
	}
	if !present {
		return nil, nil
	}
	value, err := readSavedAttachPRS(br, path)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

// readSavedAttachPRS 按游戏顺序读取位置、缩放和旋转
// readSavedAttachPRS reads position, scale, and rotation in the game's order
func readSavedAttachPRS(br *stream.BinaryReader, path string) (SavedAttachPosRotScale, error) {
	position, err := br.ReadFloat3()
	if err != nil {
		return SavedAttachPosRotScale{}, fmt.Errorf("read %s position: %w", path, err)
	}
	scale, err := br.ReadFloat3()
	if err != nil {
		return SavedAttachPosRotScale{}, fmt.Errorf("read %s scale: %w", path, err)
	}
	rotation, err := br.ReadFloat4()
	if err != nil {
		return SavedAttachPosRotScale{}, fmt.Errorf("read %s rotation: %w", path, err)
	}
	return SavedAttachPosRotScale{
		Position: Vector3{X: position[0], Y: position[1], Z: position[2]},
		Scale:    Vector3{X: scale[0], Y: scale[1], Z: scale[2]},
		Rotation: Vector4{X: rotation[0], Y: rotation[1], Z: rotation[2], W: rotation[3]},
	}, nil
}

// writeSavedAttachOptionalPRS 写入存在标志及可选位置旋转缩放值
// writeSavedAttachOptionalPRS writes a presence flag and optional position-rotation-scale value
func writeSavedAttachOptionalPRS(bw *stream.BinaryWriter, value *SavedAttachPosRotScale) error {
	if err := bw.WriteBool(value != nil); err != nil {
		return err
	}
	if value == nil {
		return nil
	}
	return writeSavedAttachPRS(bw, value)
}

// writeSavedAttachPRS 按游戏顺序写入位置、缩放和旋转
// writeSavedAttachPRS writes position, scale, and rotation in the game's order
func writeSavedAttachPRS(bw *stream.BinaryWriter, value *SavedAttachPosRotScale) error {
	if err := bw.WriteFloat3([3]float32{value.Position.X, value.Position.Y, value.Position.Z}); err != nil {
		return err
	}
	if err := bw.WriteFloat3([3]float32{value.Scale.X, value.Scale.Y, value.Scale.Z}); err != nil {
		return err
	}
	return bw.WriteFloat4([4]float32{value.Rotation.X, value.Rotation.Y, value.Rotation.Z, value.Rotation.W})
}

// readSavedAttachHierarchy 读取可空的骨骼名到局部变换映射并保留条目顺序
// readSavedAttachHierarchy reads a nullable bone-name-to-local-transform map and preserves entry order
func readSavedAttachHierarchy(br *stream.BinaryReader, remaining *bytes.Reader, path string) (map[string]SavedAttachPosRotScale, []string, error) {
	present, err := br.ReadBool()
	if err != nil {
		return nil, nil, fmt.Errorf("read %s presence: %w", path, err)
	}
	if !present {
		return nil, nil, nil
	}
	count, err := br.ReadInt32()
	if err != nil {
		return nil, nil, fmt.Errorf("read %s count: %w", path, err)
	}
	if count < 0 {
		return nil, nil, fmt.Errorf("negative %s count %d", path, count)
	}
	// 每个条目至少包含一个单字节空字符串和十个 Float32 值
	// Each entry contains at least a one-byte empty string and ten Float32 values
	if int64(count) > int64(remaining.Len()/41) {
		return nil, nil, fmt.Errorf("%s count %d cannot fit in %d remaining bytes", path, count, remaining.Len())
	}
	values := makeKCESCountedMap[string, SavedAttachPosRotScale](uint64(count))
	order := makeKCESCountedSliceForAppend[string](uint64(count))
	for i := int32(0); i < count; i++ {
		name, readErr := readSavedAttachString(br, fmt.Sprintf("%s[%d] name", path, i))
		if readErr != nil {
			return nil, nil, readErr
		}
		if _, duplicate := values[name]; duplicate {
			return nil, nil, fmt.Errorf("%s contains duplicate bone name %q", path, name)
		}
		prs, readErr := readSavedAttachPRS(br, fmt.Sprintf("%s[%q]", path, name))
		if readErr != nil {
			return nil, nil, readErr
		}
		values[name] = prs
		order = append(order, name)
	}
	return values, order, nil
}

// writeSavedAttachHierarchy 按保存顺序写入可空的骨骼变换映射
// writeSavedAttachHierarchy writes a nullable bone-transform map in its preserved order
func writeSavedAttachHierarchy(bw *stream.BinaryWriter, values map[string]SavedAttachPosRotScale, order []string, itemPath string) error {
	if err := bw.WriteBool(values != nil); err != nil {
		return fmt.Errorf("write %s boneAttachedHierarchy presence: %w", itemPath, err)
	}
	if values == nil {
		return nil
	}
	if len(values) > math.MaxInt32 {
		return fmt.Errorf("%s boneAttachedHierarchy length %d exceeds Int32", itemPath, len(values))
	}
	count := int32(len(values))
	if err := bw.WriteInt32(count); err != nil {
		return fmt.Errorf("write %s boneAttachedHierarchy count: %w", itemPath, err)
	}
	for key := range values {
		if err := validateSavedAttachString(key, itemPath+" boneAttachedHierarchy key"); err != nil {
			return err
		}
	}
	keys, err := utilities.MergeOrderedMapKeys(values, order, itemPath+" boneHierarchyOrder")
	if err != nil {
		return err
	}
	for _, key := range keys {
		if err := bw.WriteString(key); err != nil {
			return fmt.Errorf("write %s boneAttachedHierarchy[%q] name: %w", itemPath, key, err)
		}
		value := values[key]
		if err := writeSavedAttachPRS(bw, &value); err != nil {
			return fmt.Errorf("write %s boneAttachedHierarchy[%q]: %w", itemPath, key, err)
		}
	}
	return nil
}
