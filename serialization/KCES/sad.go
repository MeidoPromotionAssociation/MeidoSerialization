package KCES

import (
	"bytes"
	"fmt"
	"math"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/binaryio/stream"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/utilities"
)

// .sad (SAVED_ATTACH_DATA)
// KCES/GP03 导出的部件附着信息文件，使用 BinaryWriter 编码外层列表与每条附着记录。
// 外层版本为 2000；当前记录版本为 2001，并兼容无显式记录版本的旧版 2000 布局。
//
// .sad (SAVED_ATTACH_DATA)
// KCES/GP03 exported part-attachment file, encoded by BinaryWriter as an outer list of attachment records.
// The outer version is 2000; current records use version 2001 while the implicit legacy 2000 layout remains supported.

const (
	// SavedAttachSignature is written by ExportCM.ExportAttachData before the
	// file-level version and record count.
	SavedAttachSignature = "SAVED_ATTACH_DATA"
	// SavedAttachFileVersion is the only outer version emitted by KCES 1.34.4.
	SavedAttachFileVersion int32 = 2000
	// SavedAttachRecordVersion is emitted inside every current SavedAttachData
	// record after an empty nullable-string sentinel.
	SavedAttachRecordVersion int32 = 2001
	// KCESSavedAttachFormat identifies the editable JSON representation.
	KCESSavedAttachFormat = "kces-saved-attach"

	// The shortest structurally readable implicit-v2000 record occupies 36
	// bytes (one-character part name and empty enum-string payloads).
	minSavedAttachItemBytes = 36
)

// SavedAttachFile represents one exported KCES/GP03 .sad file.
type SavedAttachFile struct {
	Format       string            `json:"format"`
	Signature    string            `json:"signature"`
	Version      int32             `json:"version"`
	Items        []SavedAttachData `json:"items"`
	TrailingData []byte            `json:"trailingData,omitempty"`
}

// SavedAttachData mirrors the BinaryWriter layout implemented by the game's
// SavedAttachData.Serialize/Deserialize methods. Pointer fields preserve the
// null/non-null flags used by WriteNaS and by the two PosRotScale values.
type SavedAttachData struct {
	Version                int32                             `json:"version"`
	ExplicitVersion        bool                              `json:"explicitVersion,omitempty"`
	PartName               *string                           `json:"partName"`
	Enabled                bool                              `json:"enabled"`
	MyRID                  uint64                            `json:"myRid"`
	MySlotID               string                            `json:"mySlotId"`
	TargetRID              uint64                            `json:"targetRid"`
	TargetSlotID           string                            `json:"targetSlotId"`
	TargetSlotNo           int32                             `json:"targetSlotNo"`
	TargetAttachPointName  *string                           `json:"targetAttachPointName"`
	TargetVertexCount      int32                             `json:"targetVertexCount"`
	TargetVertexIndex      int32                             `json:"targetVertexIndex"`
	NewAttachVertexIndices []int32                           `json:"newAttachVertexIndices"`
	PRS2                   *SavedAttachPosRotScale           `json:"prs2"`
	PRS3                   *SavedAttachPosRotScale           `json:"prs3"`
	BoneAttachedHierarchy  map[string]SavedAttachPosRotScale `json:"boneAttachedHierarchy"`
	BoneHierarchyOrder     []string                          `json:"boneHierarchyOrder,omitempty"`
	BoneAttachEdited       bool                              `json:"boneAttachEdited"`
}

// SavedAttachPosRotScale is Scourt.Utility.UnityUtility.PosRotScale's binary
// form: Vector3 position, Vector3 scale, then Quaternion rotation.
type SavedAttachPosRotScale struct {
	Position Vector3 `json:"position"`
	Scale    Vector3 `json:"scale"`
	Rotation Vector4 `json:"rotation"`
}

// DecodeSavedAttach decodes an exported .sad file. Slot IDs remain opaque wire
// strings; enum interpretation belongs to the consuming game.
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

func decodeSavedAttachItem(br *stream.BinaryReader, remaining *bytes.Reader, index int64) (SavedAttachData, error) {
	return decodeSavedAttachItemWithSlotValidator(br, remaining, index, validateSavedAttachSlotID)
}

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
		// The legacy 2000 layout starts directly with a non-empty part name and
		// does not contain TargetSlotNo.
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

// EncodeSavedAttach emits the current or legacy layout selected by each item's
// Version. A 2000 record normally uses the direct legacy layout; when its part
// name is nil/empty it uses the explicit-version sentinel accepted by the game,
// because the direct form would be mistaken for a sentinel. Outer and record
// versions, including zero and future values, are written unchanged.
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

// NewSavedAttachFile explicitly creates the current outer header. Record
// versions remain caller-controlled and are never upgraded during encoding.
func NewSavedAttachFile() *SavedAttachFile {
	return &SavedAttachFile{
		Format:    KCESSavedAttachFormat,
		Signature: SavedAttachSignature,
		Version:   SavedAttachFileVersion,
	}
}

func encodeSavedAttachItem(bw *stream.BinaryWriter, source *SavedAttachData, index int64) error {
	return encodeSavedAttachItemWithSlotValidator(bw, source, index, validateSavedAttachSlotID)
}

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

func readSavedAttachString(br *stream.BinaryReader, path string) (string, error) {
	value, err := br.ReadString()
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return value, nil
}

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

func writeSavedAttachNullableString(bw *stream.BinaryWriter, value *string) error {
	if err := bw.WriteBool(value != nil); err != nil {
		return err
	}
	if value != nil {
		return bw.WriteString(*value)
	}
	return nil
}

func validateSavedAttachNullableString(value *string, path string) error {
	if value != nil {
		return validateSavedAttachString(*value, path)
	}
	return nil
}

func validateSavedAttachString(value, path string) error {
	if uint64(len(value)) > uint64(math.MaxInt32) {
		return fmt.Errorf("%s is %d bytes, exceeds Int32", path, len(value))
	}
	return nil
}

func readSavedAttachSlotID(br *stream.BinaryReader, path string) (string, error) {
	return readSavedAttachSlotIDWithValidator(br, path, validateSavedAttachSlotID)
}

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

func validateSavedAttachSlotID(value, path string) error {
	return validateSavedAttachString(value, path)
}

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

func writeSavedAttachOptionalPRS(bw *stream.BinaryWriter, value *SavedAttachPosRotScale) error {
	if err := bw.WriteBool(value != nil); err != nil {
		return err
	}
	if value == nil {
		return nil
	}
	return writeSavedAttachPRS(bw, value)
}

func writeSavedAttachPRS(bw *stream.BinaryWriter, value *SavedAttachPosRotScale) error {
	if err := bw.WriteFloat3([3]float32{value.Position.X, value.Position.Y, value.Position.Z}); err != nil {
		return err
	}
	if err := bw.WriteFloat3([3]float32{value.Scale.X, value.Scale.Y, value.Scale.Z}); err != nil {
		return err
	}
	return bw.WriteFloat4([4]float32{value.Rotation.X, value.Rotation.Y, value.Rotation.Z, value.Rotation.W})
}

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
	// Each entry has at least a one-byte empty string and ten float32 values.
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
