package KCES

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	COM3D2Service "github.com/MeidoPromotionAssociation/MeidoSerialization/service/COM3D2"
)

const (
	kcesVirtualDirectorySignature = "KCES_VIRTUAL_DIRECTORY"
	kcesMessagePackSignature      = "MessagePack-Lz4BlockArray"
	kcesUnityFSSignature          = "UnityFS"
	kcesEncryptedABASignature     = "abap"
)

// FileTypeService 探测 KCES 引入的文件格式，并通过真实解码或明确 wire 签名区分与 COM3D2 共用的扩展名 / FileTypeService probes formats introduced by KCES and distinguishes extensions shared with COM3D2 through real decoding or explicit wire signatures
type FileTypeService struct{}

// TryFileTypeDetermine 探测文件是否可证实为 KCES 格式，并用 matched 为 true 且 error 非空表示具有 KCES 特征但内容损坏的候选文件
// TryFileTypeDetermine probes whether a file is proven to use a KCES format and uses a true match with a non-nil error for malformed candidates carrying KCES characteristics
func (s *FileTypeService) TryFileTypeDetermine(path string) (info COM3D2Service.FileInfo, matched bool, err error) {
	info = COM3D2Service.FileInfo{
		Path:          path,
		FileType:      COM3D2Service.UnknownFileType,
		StorageFormat: COM3D2Service.FormatUnknown,
		Game:          COM3D2Service.UnknowGame,
		Signature:     COM3D2Service.UnknownSignature,
	}

	file, err := os.Open(path)
	if err != nil {
		return info, false, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return info, false, err
	}
	if stat.IsDir() {
		return info, false, fmt.Errorf("%q is a directory", path)
	}
	info.Size = stat.Size()

	header := make([]byte, 4096)
	n, readErr := file.Read(header)
	if readErr != nil && n == 0 && stat.Size() != 0 {
		return info, false, fmt.Errorf("read %q: %w", path, readErr)
	}
	header = header[:n]
	lowerPath := strings.ToLower(path)
	ext := strings.ToLower(filepath.Ext(path))

	// KCES has JSON-backed game files as well as JSON editing envelopes. Probe
	// these before binary signatures so .undressdat is not handed to the legacy
	// JSON detector, which only understands capitalized COM3D2 headers
	trimmedHeader := bytes.TrimSpace(bytes.TrimPrefix(header, []byte{0xef, 0xbb, 0xbf}))
	if strings.HasSuffix(lowerPath, ".json") || serializationKCES.IsKCESJSONTextExtension(ext) || IsKCESExportNameMapFile(path) ||
		bytes.HasPrefix(trimmedHeader, []byte{'{'}) || bytes.HasPrefix(trimmedHeader, []byte{'['}) {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return info, false, readErr
		}
		if jsonInfo, candidate, jsonErr := probeKCESJSON(path, data, info); candidate {
			return jsonInfo, true, jsonErr
		}
	}

	// Standard KCES AssetBundles use UnityFS. ReadAba validates the complete
	// header, file-size declaration, block metadata and supported version without
	// decompressing every asset payload
	if bytes.HasPrefix(header, []byte(kcesUnityFSSignature+"\x00")) {
		if _, seekErr := file.Seek(0, 0); seekErr != nil {
			return info, true, seekErr
		}
		abaFile, abaErr := aba.ReadAba(file)
		if abaErr != nil {
			return info, true, fmt.Errorf("validate KCES UnityFS file %q: %w", path, abaErr)
		}
		info.FileType = "aba"
		if ext == assetBGExtension {
			info.FileType = "asset_bg"
		} else if ext == aba.AssetSceneExtension {
			info.FileType = "asset_scene"
		}
		info.StorageFormat = COM3D2Service.FormatBinary
		info.Game = COM3D2Service.GameKCES
		info.Signature = kcesUnityFSSignature
		info.Version = int32(abaFile.Header.Version)
		return info, true, nil
	}
	// abap is an encrypted KCES AssetBundle. The parser intentionally cannot
	// decrypt it yet, but its four-byte signature is unambiguous and determine
	// should report that fact instead of calling it CSV
	if bytes.HasPrefix(header, []byte(kcesEncryptedABASignature)) {
		info.FileType = "aba"
		info.StorageFormat = COM3D2Service.FormatBinary
		info.Game = COM3D2Service.GameKCES
		info.Signature = kcesEncryptedABASignature
		return info, true, nil
	}

	// .ct, current KCES presets and system.dat all use VirtualDirectory's
	// seven-byte signature. The virtual-file names distinguish their semantic
	// payloads even when the outer file was renamed
	if bytes.HasPrefix(header, ct.FileSignature) {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return info, true, readErr
		}
		table, tableErr := ct.ReadContentTable(bytes.NewReader(data))
		if tableErr != nil {
			return info, true, fmt.Errorf("validate KCES VirtualDirectory %q: %w", path, tableErr)
		}
		info.StorageFormat = COM3D2Service.FormatBinary
		info.Game = COM3D2Service.GameKCES
		info.Signature = kcesVirtualDirectorySignature
		assignKCESVersion(&info, table.Version)
		_, hasBridgeSessionData := table.Files["session_data"]
		_, hasBridgeSessionID := table.Files["session_id"]
		if IsKCESBridgeSessionFile(path) || hasBridgeSessionData || hasBridgeSessionID {
			if _, sessionErr := serializationKCES.DecodeKCESBridgeSession(data); sessionErr != nil {
				return info, true, fmt.Errorf("validate KCES bridge_session.vd %q: %w", path, sessionErr)
			}
			info.FileType = "bridge_session"
			return info, true, nil
		}
		isSystemData := strings.EqualFold(filepath.Base(path), "system.dat")
		if !isSystemData {
			for name := range table.Files {
				if strings.HasPrefix(name, "EditData/") {
					isSystemData = true
					break
				}
			}
		}
		if isSystemData {
			if _, systemErr := serializationKCES.DecodeKCESSystemData(data); systemErr != nil {
				return info, true, fmt.Errorf("validate KCES system.dat %q: %w", path, systemErr)
			}
			info.FileType = "system"
			return info, true, nil
		}

		if _, ok := table.Files["catalog"]; ok {
			if _, catalogErr := ct.DecodeCatalogFromCt(table); catalogErr != nil {
				return info, true, fmt.Errorf("validate KCES catalog in %q: %w", path, catalogErr)
			}
			info.FileType = "ct"
			return info, true, nil
		}
		_, hasThumbnail := table.Files["thumbnail"]
		_, hasMaidData := table.Files["maiddata"]
		if hasThumbnail || hasMaidData {
			if !hasThumbnail || !hasMaidData {
				return info, true, fmt.Errorf("KCES preset %q is missing thumbnail or maiddata", path)
			}
			if _, presetErr := serializationKCES.DecodeKCESPreset(data); presetErr != nil {
				return info, true, fmt.Errorf("validate KCES preset %q: %w", path, presetErr)
			}
			info.FileType = "preset"
			return info, true, nil
		}
		info.FileType = "virtualdirectory"
		return info, true, nil
	}

	if IsKCESMaidColliderFile(path) {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return info, true, readErr
		}
		if _, decodeErr := serializationKCES.DecodeMaidCollider(data); decodeErr != nil {
			return info, true, fmt.Errorf("validate KCES maid collider %q: %w", path, decodeErr)
		}
		info.FileType = "maid_collider"
		info.StorageFormat = COM3D2Service.FormatBinary
		info.Game = COM3D2Service.GameKCES
		info.Signature = serializationKCES.MaidColliderFormat
		return info, true, nil
	}

	if partsType, isPartsCandidate := kcesPartsType(ext); isPartsCandidate {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return info, true, readErr
		}
		value, decodeErr := decodeKCESPartsForProbe(ext, data)
		if decodeErr != nil {
			// .model is shared with legacy COM3D2. A failed KCES decode is not
			// an error here; the caller must give the legacy signature parser a
			// chance, while the three *assets extensions are KCES-only
			if ext == ".model" {
				return info, false, nil
			}
			return info, true, fmt.Errorf("validate KCES %s file %q: %w", partsType, path, decodeErr)
		}
		info.FileType = partsType
		info.StorageFormat = COM3D2Service.FormatBinary
		info.Game = COM3D2Service.GameKCES
		info.Signature = kcesMessagePackSignature
		if version := kcesPartsVersion(value); version != 0 {
			assignKCESVersion(&info, version)
		}
		return info, true, nil
	}

	if payloadExt := serializationKCES.NormalizeKCESPayloadExtension(path); payloadExt != "" && !strings.HasSuffix(lowerPath, ".json") {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return info, true, readErr
		}
		envelope, decodeErr := serializationKCES.DecodeKCESPayload(data, path)
		if decodeErr != nil {
			return info, true, fmt.Errorf("validate KCES payload %q: %w", path, decodeErr)
		}
		info.FileType = strings.TrimPrefix(payloadExt, ".")
		info.StorageFormat = COM3D2Service.FormatBinary
		info.Game = COM3D2Service.GameKCES
		info.Signature = kcesMessagePackSignature
		if version := kcesPayloadVersion(envelope); version != 0 {
			assignKCESVersion(&info, version)
		}
		return info, true, nil
	}

	if ext == ".hitcheck" {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return info, true, readErr
		}
		if _, decodeErr := serializationKCES.DecodeHitCheck(data); decodeErr != nil {
			return info, true, fmt.Errorf("validate KCES HitCheck %q: %w", path, decodeErr)
		}
		info.FileType = "hitcheck"
		info.StorageFormat = COM3D2Service.FormatBinary
		info.Game = COM3D2Service.GameKCES
		info.Signature = serializationKCES.HitCheckSignature
		// HitCheck has no domain version; the int32 after its signature is a
		// record count, not a version number
		return info, true, nil
	}

	if IsKCESGP03BridgeFile(path) || serializationKCES.IsGP03BridgeData(header) {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return info, true, readErr
		}
		value, decodeErr := decodeGP03Bridge(data)
		if decodeErr != nil {
			return info, true, fmt.Errorf("validate KCES GP03 bridge file %q: %w", path, decodeErr)
		}
		info.FileType = "brd"
		info.StorageFormat = COM3D2Service.FormatBinary
		info.Game = COM3D2Service.GameKCES
		info.Signature = serializationKCES.GP03BridgeSignature
		info.Version = value.Version
		return info, true, nil
	}

	if IsKCESSavedAttachFile(path) || hasSavedAttachWireSignature(header) {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return info, true, readErr
		}
		value, decodeErr := serializationKCES.DecodeSavedAttach(data)
		if decodeErr != nil {
			return info, true, fmt.Errorf("validate KCES saved-attach file %q: %w", path, decodeErr)
		}
		info.FileType = "sad"
		info.StorageFormat = COM3D2Service.FormatBinary
		info.Game = COM3D2Service.GameKCES
		info.Signature = serializationKCES.SavedAttachSignature
		info.Version = value.Version
		return info, true, nil
	}

	if IsKCESPathsFile(path) {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return info, true, readErr
		}
		value, decodeErr := serializationKCES.DecodeKCESPaths(data)
		if decodeErr != nil {
			return info, true, fmt.Errorf("validate KCES paths.dat %q: %w", path, decodeErr)
		}
		info.FileType = "paths"
		info.StorageFormat = COM3D2Service.FormatBinary
		info.Game = COM3D2Service.GameKCES
		info.Signature = value.Signature
		info.Version = value.Version
		return info, true, nil
	}

	if IsKCESRawUnityBytesFile(path) {
		envelope, rawErr := (&RawUnityObjectService{}).ReadRawUnityObjectFile(path)
		if rawErr != nil {
			return info, true, fmt.Errorf("validate KCES raw Unity object %q: %w", path, rawErr)
		}
		info.FileType = "bytes"
		info.StorageFormat = COM3D2Service.FormatBinary
		info.Game = COM3D2Service.GameKCES
		info.Signature = envelope.TypeName
		if info.Signature == "" {
			info.Signature = RawUnityObjectFormat
		}
		return info, true, nil
	}
	if IsKCESBridgeSessionFile(path) {
		return info, true, fmt.Errorf("%q does not contain a valid KCES VirtualDirectory signature", path)
	}

	// These extensions are KCES-only. Reaching this point means the expected
	// content magic was absent, so return a deterministic validation error rather
	// than allowing the legacy one-record CSV heuristic to misclassify the file
	switch ext {
	case ".ct", ".aba", assetBGExtension, aba.AssetSceneExtension, serializationKCES.KCESPersetExtension:
		return info, true, fmt.Errorf("%q does not contain a valid KCES %s signature", path, strings.TrimPrefix(ext, "."))
	}

	return info, false, nil
}

// probeKCESJSON 探测原生 JSON 资源和编辑 JSON 封套，并返回是否属于 KCES 候选格式
// probeKCESJSON probes native JSON resources and editing JSON envelopes and reports whether they are KCES format candidates
func probeKCESJSON(path string, data []byte, info COM3D2Service.FileInfo) (COM3D2Service.FileInfo, bool, error) {
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	lowerPath := strings.ToLower(path)
	ext := strings.ToLower(filepath.Ext(path))

	// export_map.enm is itself a Unity JsonUtility document, while .enm.json
	// is our marker-based editing form. Their extensions are KCES-specific, so
	// malformed candidates remain matched and return their validation error
	// rather than falling through to the legacy JSON/CSV heuristics
	if IsKCESExportNameMapFile(path) {
		value, err := serializationKCES.DecodeKCESExportNameMap(data)
		if err != nil {
			return info, true, fmt.Errorf("validate KCES export name map %q: %w", path, err)
		}
		info.FileType = "enm"
		info.StorageFormat = COM3D2Service.FormatJSON
		info.Game = COM3D2Service.GameKCES
		info.Signature = serializationKCES.KCESExportNameMapSignature
		info.Version = value.Version
		return info, true, nil
	}
	if strings.HasSuffix(lowerPath, ".enm.json") {
		value, err := serializationKCES.DecodeKCESExportNameMapJSON(data)
		if err != nil {
			return info, true, fmt.Errorf("validate KCES export name map editing JSON %q: %w", path, err)
		}
		info.FileType = "enm"
		info.StorageFormat = COM3D2Service.FormatJSON
		info.Game = COM3D2Service.GameKCES
		info.Signature = serializationKCES.KCESExportNameMapFormat
		info.Version = value.Version
		return info, true, nil
	}
	if strings.HasSuffix(lowerPath, ".sad.json") {
		value, err := decodeSavedAttachEditingJSON(data)
		if err != nil {
			return info, true, fmt.Errorf("validate KCES saved-attach editing JSON %q: %w", path, err)
		}
		info.FileType = "sad"
		info.StorageFormat = COM3D2Service.FormatJSON
		info.Game = COM3D2Service.GameKCES
		info.Signature = serializationKCES.KCESSavedAttachFormat
		info.Version = value.Version
		return info, true, nil
	}
	if strings.HasSuffix(lowerPath, ".brd.json") {
		value, err := decodeGP03BridgeEditingJSON(data)
		if err != nil {
			return info, true, fmt.Errorf("validate KCES GP03 bridge editing JSON %q: %w", path, err)
		}
		info.FileType = "brd"
		info.StorageFormat = COM3D2Service.FormatJSON
		info.Game = COM3D2Service.GameKCES
		info.Signature = serializationKCES.KCESGP03BridgeFormat
		info.Version = value.Version
		return info, true, nil
	}
	if IsKCESBridgeSessionJSONFile(path) {
		value, err := decodeKCESBridgeSessionEditingJSON(data)
		if err != nil {
			return info, true, fmt.Errorf("validate KCES bridge session editing JSON %q: %w", path, err)
		}
		info.FileType = "bridge_session"
		info.StorageFormat = COM3D2Service.FormatJSON
		info.Game = COM3D2Service.GameKCES
		info.Signature = serializationKCES.KCESBridgeSessionFormat
		assignKCESVersion(&info, value.ContainerVersion)
		return info, true, nil
	}

	// Native .undressdat/.undresspdat/.nson files are themselves JSON. Their
	// extensions are KCES-specific, so malformed JSON remains a matched KCES
	// candidate with a useful validation error instead of falling through to
	// legacy CSV/JSON heuristics
	if serializationKCES.IsKCESJSONTextExtension(ext) && !strings.HasSuffix(lowerPath, ".json") {
		value, err := serializationKCES.DecodeKCESJSONText(data, ext)
		if err != nil {
			return info, true, err
		}
		info.StorageFormat = COM3D2Service.FormatJSON
		info.Game = COM3D2Service.GameKCES
		info.FileType = strings.TrimPrefix(value.Extension, ".")
		info.Signature = "KCES_JSON_TEXT"
		return info, true, nil
	}

	// Editing envelopes for the JSON-backed resources have no format marker;
	// the authoritative marker is their KCES-only double extension. Handle the
	// candidate before json.Valid so a truncated/invalid envelope cannot fall
	// through to the legacy COM3D2 JSON or CSV probes
	if strings.HasSuffix(lowerPath, ".json") {
		basePath := strings.TrimSuffix(path, filepath.Ext(path))
		jsonTextExt := serializationKCES.NormalizeKCESJSONTextExtension(basePath)
		if jsonTextExt != "" {
			value, err := decodeKCESJSONTextEditingJSON(data, jsonTextExt)
			if err != nil {
				return info, true, fmt.Errorf("validate KCES JSON-text envelope %q: %w", path, err)
			}
			info.StorageFormat = COM3D2Service.FormatJSON
			info.Game = COM3D2Service.GameKCES
			info.FileType = strings.TrimPrefix(value.Extension, ".")
			info.Signature = "KCES_JSON_TEXT"
			return info, true, nil
		}
	}
	if candidateInfo, candidate, candidateErr := probeKCESOnlyEditingJSONByPath(path, data, info); candidate {
		return candidateInfo, true, candidateErr
	}

	if !json.Valid(data) {
		return info, false, nil
	}

	var header struct {
		Format           string `json:"format"`
		Extension        string `json:"extension"`
		Version          int32  `json:"version"`
		ContainerVersion int32  `json:"containerVersion"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return info, false, nil
	}

	info.StorageFormat = COM3D2Service.FormatJSON
	info.Game = COM3D2Service.GameKCES
	if header.Version != 0 {
		assignKCESVersion(&info, header.Version)
	}

	switch header.Format {
	case serializationKCES.KCESBridgeSessionFormat:
		value, err := decodeKCESBridgeSessionEditingJSON(data)
		if err != nil {
			return info, true, fmt.Errorf("validate KCES bridge session JSON %q: %w", path, err)
		}
		info.FileType = "bridge_session"
		info.Signature = serializationKCES.KCESBridgeSessionFormat
		assignKCESVersion(&info, value.ContainerVersion)
		return info, true, nil
	case serializationKCES.KCESGP03BridgeFormat:
		value, err := decodeGP03BridgeEditingJSON(data)
		if err != nil {
			return info, true, fmt.Errorf("validate KCES GP03 bridge JSON %q: %w", path, err)
		}
		info.FileType = "brd"
		info.Signature = serializationKCES.KCESGP03BridgeFormat
		info.Version = value.Version
		return info, true, nil
	case serializationKCES.KCESExportNameMapFormat:
		value, err := serializationKCES.DecodeKCESExportNameMapJSON(data)
		if err != nil {
			return info, true, fmt.Errorf("validate KCES export name map JSON %q: %w", path, err)
		}
		info.FileType = "enm"
		info.Signature = serializationKCES.KCESExportNameMapFormat
		info.Version = value.Version
		return info, true, nil
	case serializationKCES.KCESSavedAttachFormat:
		value, err := decodeSavedAttachEditingJSON(data)
		if err != nil {
			return info, true, fmt.Errorf("validate KCES saved-attach JSON %q: %w", path, err)
		}
		info.FileType = "sad"
		info.Signature = serializationKCES.KCESSavedAttachFormat
		info.Version = value.Version
		return info, true, nil
	case serializationKCES.KCESPathsFormat:
		var value serializationKCES.KCESPathsFile
		if err := decodeStrictJSON(data, &value, "KCES paths.dat JSON"); err != nil {
			return info, true, fmt.Errorf("validate KCES paths.dat JSON %q: %w", path, err)
		}
		if _, err := serializationKCES.EncodeKCESPaths(&value); err != nil {
			return info, true, fmt.Errorf("validate KCES paths.dat JSON %q: %w", path, err)
		}
		info.FileType = "paths"
		info.Signature = value.Signature
		info.Version = value.Version
		return info, true, nil
	case serializationKCES.KCESSystemDataFormat:
		value, err := decodeKCESSystemDataEditingJSON(data)
		if err != nil {
			return info, true, fmt.Errorf("validate KCES system.dat JSON %q: %w", path, err)
		}
		info.FileType = "system"
		info.Signature = serializationKCES.KCESSystemDataFormat
		assignKCESVersion(&info, value.Version)
		return info, true, nil
	case CtEnvelopeFormat:
		var envelope CtEnvelope
		if err := decodeStrictJSON(data, &envelope, "KCES content-table JSON"); err != nil {
			return info, true, fmt.Errorf("validate KCES ct JSON %q: %w", path, err)
		}
		table, err := buildContentTableFromCtEnvelope(&envelope)
		if err != nil {
			return info, true, fmt.Errorf("validate KCES ct JSON %q: %w", path, err)
		}
		if err := ct.WriteContentTable(io.Discard, table); err != nil {
			return info, true, fmt.Errorf("validate KCES ct JSON %q: %w", path, err)
		}
		info.FileType = "ct"
		info.Signature = CtEnvelopeFormat
		assignKCESVersion(&info, envelope.Version)
		return info, true, nil
	case serializationKCES.KCESPresetFormat:
		var preset serializationKCES.ExpandedKCESPreset
		if err := decodeStrictJSON(data, &preset, "KCES preset JSON"); err != nil {
			return info, true, fmt.Errorf("validate KCES preset JSON %q: %w", path, err)
		}
		if _, err := serializationKCES.EncodeExpandedKCESPreset(&preset); err != nil {
			return info, true, fmt.Errorf("validate KCES preset JSON %q: %w", path, err)
		}
		info.FileType = "preset"
		info.Signature = serializationKCES.KCESPresetFormat
		assignKCESVersion(&info, preset.ContainerVersion)
		return info, true, nil
	case serializationKCES.MaidColliderFormat:
		var value serializationKCES.MaidColliderFile
		if err := decodeStrictJSON(data, &value, "KCES maid collider JSON"); err != nil {
			return info, true, fmt.Errorf("validate KCES maid collider JSON %q: %w", path, err)
		}
		if _, err := serializationKCES.EncodeMaidCollider(&value); err != nil {
			return info, true, fmt.Errorf("validate KCES maid collider JSON %q: %w", path, err)
		}
		info.FileType = "maid_collider"
		info.Signature = serializationKCES.MaidColliderFormat
		return info, true, nil
	case RawUnityObjectFormat:
		var envelope RawUnityObjectEnvelope
		if err := decodeStrictJSON(data, &envelope, "KCES raw Unity object JSON"); err != nil {
			return info, true, fmt.Errorf("validate KCES raw Unity JSON %q: %w", path, err)
		}
		raw, err := base64.StdEncoding.DecodeString(envelope.DataBase64)
		if err != nil {
			return info, true, fmt.Errorf("validate KCES raw Unity JSON %q dataBase64: %w", path, err)
		}
		if len(raw) == 0 {
			return info, true, fmt.Errorf("validate KCES raw Unity JSON %q: decoded dataBase64 is empty", path)
		}
		if envelope.TypeTree != nil && envelope.TypeTree.Format != "" && envelope.TypeTree.Format != RawUnityTypeTreeFormat {
			return info, true, fmt.Errorf("validate KCES raw Unity JSON %q: unsupported typeTree format %q", path, envelope.TypeTree.Format)
		}
		info.FileType = "bytes"
		info.Signature = RawUnityObjectFormat
		return info, true, nil
	}

	if partsExt := partsExtFromJSONPath(path); partsExt != "" {
		if _, isCandidate := kcesPartsType(partsExt); isCandidate {
			if _, err := encodePartsJSON(partsExt, data); err != nil {
				// Like binary .model, .model.json is shared with COM3D2. Its
				// capitalized legacy schema must remain available to the fallback
				if partsExt == ".model" {
					return info, false, nil
				}
				return info, true, fmt.Errorf("validate KCES parts JSON %q: %w", path, err)
			}
			info.FileType = strings.TrimPrefix(partsExt, ".")
			info.Signature = kcesMessagePackSignature
			return info, true, nil
		}
	}

	if strings.HasSuffix(lowerPath, ".hitcheck.json") {
		var value serializationKCES.HitCheck
		if err := decodeStrictJSON(data, &value, "KCES HitCheck JSON"); err != nil {
			return info, true, fmt.Errorf("validate KCES HitCheck JSON %q: %w", path, err)
		}
		if _, err := serializationKCES.EncodeHitCheck(&value); err != nil {
			return info, true, fmt.Errorf("validate KCES HitCheck JSON %q: %w", path, err)
		}
		info.FileType = "hitcheck"
		info.Signature = serializationKCES.HitCheckSignature
		return info, true, nil
	}

	return info, false, nil
}

// probeKCESOnlyEditingJSONByPath 根据 KCES 独占的内部扩展名探测编辑 JSON，并保留共用 .model 与预设格式的内容探测回退
// probeKCESOnlyEditingJSONByPath probes editing JSON through KCES-only inner extensions while retaining content-based fallback for shared .model and preset formats
func probeKCESOnlyEditingJSONByPath(path string, data []byte, info COM3D2Service.FileInfo) (COM3D2Service.FileInfo, bool, error) {
	if !strings.HasSuffix(strings.ToLower(path), ".json") {
		return info, false, nil
	}
	basePath := strings.TrimSuffix(path, filepath.Ext(path))
	innerExt := strings.ToLower(filepath.Ext(basePath))
	if strings.EqualFold(filepath.Base(basePath), "system.dat") {
		value, err := decodeKCESSystemDataEditingJSON(data)
		if err != nil {
			return info, true, fmt.Errorf("validate KCES system.dat JSON %q: %w", path, err)
		}
		info.FileType = "system"
		info.StorageFormat = COM3D2Service.FormatJSON
		info.Game = COM3D2Service.GameKCES
		info.Signature = serializationKCES.KCESSystemDataFormat
		assignKCESVersion(&info, value.Version)
		return info, true, nil
	}

	if innerExt == ".menuassets" || innerExt == ".materialassets" || innerExt == ".pmatassets" || innerExt == ".kcmenu" || innerExt == ".kcmat" || innerExt == ".kcmodel" {
		if _, err := encodePartsJSON(innerExt, data); err != nil {
			return info, true, fmt.Errorf("validate KCES parts JSON %q: %w", path, err)
		}
		info.FileType = strings.TrimPrefix(innerExt, ".")
		info.StorageFormat = COM3D2Service.FormatJSON
		info.Game = COM3D2Service.GameKCES
		info.Signature = kcesMessagePackSignature
		return info, true, nil
	}

	if payloadExt := serializationKCES.NormalizeKCESPayloadExtension(basePath); payloadExt != "" {
		value, err := decodeKCESPayloadEditingJSON(data, payloadExt)
		if err != nil {
			return info, true, fmt.Errorf("validate KCES payload JSON %q: %w", path, err)
		}
		info.FileType = strings.TrimPrefix(payloadExt, ".")
		info.StorageFormat = COM3D2Service.FormatJSON
		info.Game = COM3D2Service.GameKCES
		info.Signature = kcesMessagePackSignature
		if version := kcesPayloadVersion(value); version != 0 {
			assignKCESVersion(&info, version)
		}
		return info, true, nil
	}

	if innerExt == ".hitcheck" {
		var value serializationKCES.HitCheck
		if err := decodeStrictJSON(data, &value, "KCES HitCheck JSON"); err != nil {
			return info, true, fmt.Errorf("validate KCES HitCheck JSON %q: %w", path, err)
		}
		if value.Signature != serializationKCES.HitCheckSignature {
			return info, true, fmt.Errorf("validate KCES HitCheck JSON %q: invalid signature %q", path, value.Signature)
		}
		if _, err := serializationKCES.EncodeHitCheck(&value); err != nil {
			return info, true, fmt.Errorf("validate KCES HitCheck JSON %q: %w", path, err)
		}
		info.FileType = "hitcheck"
		info.StorageFormat = COM3D2Service.FormatJSON
		info.Game = COM3D2Service.GameKCES
		info.Signature = serializationKCES.HitCheckSignature
		return info, true, nil
	}

	if strings.EqualFold(filepath.Base(basePath), "paths.dat") {
		var value serializationKCES.KCESPathsFile
		if err := decodeStrictJSON(data, &value, "KCES paths.dat JSON"); err != nil {
			return info, true, fmt.Errorf("validate KCES paths.dat JSON %q: %w", path, err)
		}
		if value.Format != serializationKCES.KCESPathsFormat {
			return info, true, fmt.Errorf("validate KCES paths.dat JSON %q: unsupported format %q", path, value.Format)
		}
		if _, err := serializationKCES.EncodeKCESPaths(&value); err != nil {
			return info, true, fmt.Errorf("validate KCES paths.dat JSON %q: %w", path, err)
		}
		info.FileType = "paths"
		info.StorageFormat = COM3D2Service.FormatJSON
		info.Game = COM3D2Service.GameKCES
		info.Signature = value.Signature
		info.Version = value.Version
		return info, true, nil
	}

	if isMaidColliderBaseName(filepath.Base(basePath)) {
		var value serializationKCES.MaidColliderFile
		if err := decodeStrictJSON(data, &value, "KCES maid collider JSON"); err != nil {
			return info, true, fmt.Errorf("validate KCES maid collider JSON %q: %w", path, err)
		}
		if value.Format != serializationKCES.MaidColliderFormat {
			return info, true, fmt.Errorf("validate KCES maid collider JSON %q: unsupported format %q", path, value.Format)
		}
		if _, err := serializationKCES.EncodeMaidCollider(&value); err != nil {
			return info, true, fmt.Errorf("validate KCES maid collider JSON %q: %w", path, err)
		}
		info.FileType = "maid_collider"
		info.StorageFormat = COM3D2Service.FormatJSON
		info.Game = COM3D2Service.GameKCES
		info.Signature = serializationKCES.MaidColliderFormat
		return info, true, nil
	}

	if innerExt == ".ct" {
		var envelope CtEnvelope
		if err := decodeStrictJSON(data, &envelope, "KCES content-table JSON"); err != nil {
			return info, true, fmt.Errorf("validate KCES ct JSON %q: %w", path, err)
		}
		if envelope.Format != CtEnvelopeFormat {
			return info, true, fmt.Errorf("validate KCES ct JSON %q: unsupported format %q", path, envelope.Format)
		}
		table, err := buildContentTableFromCtEnvelope(&envelope)
		if err != nil {
			return info, true, fmt.Errorf("validate KCES ct JSON %q: %w", path, err)
		}
		if err := ct.WriteContentTable(io.Discard, table); err != nil {
			return info, true, fmt.Errorf("validate KCES ct JSON %q: %w", path, err)
		}
		info.FileType = "ct"
		info.StorageFormat = COM3D2Service.FormatJSON
		info.Game = COM3D2Service.GameKCES
		info.Signature = CtEnvelopeFormat
		assignKCESVersion(&info, envelope.Version)
		return info, true, nil
	}

	if _, _, ok := inferRawUnityObjectKind(basePath); ok {
		envelope, _, err := decodeRawUnityObjectEditingJSON(data)
		if err != nil {
			return info, true, fmt.Errorf("validate KCES raw Unity JSON %q: %w", path, err)
		}
		info.FileType = "bytes"
		info.StorageFormat = COM3D2Service.FormatJSON
		info.Game = COM3D2Service.GameKCES
		info.Signature = envelope.TypeName
		if info.Signature == "" {
			info.Signature = RawUnityObjectFormat
		}
		return info, true, nil
	}

	return info, false, nil
}

// kcesPartsType 将受支持的 KCES 部件扩展名映射为规范文件类型
// kcesPartsType maps a supported KCES parts extension to its canonical file type
func kcesPartsType(ext string) (string, bool) {
	switch strings.ToLower(ext) {
	case ".menuassets", ".materialassets", ".pmatassets", ".model", ".kcmenu", ".kcmat", ".kcmodel":
		return strings.TrimPrefix(strings.ToLower(ext), "."), true
	default:
		return "", false
	}
}

// decodeKCESPartsForProbe 使用扩展名专用 codec 解码 KCES 部件数据供格式探测
// decodeKCESPartsForProbe decodes KCES parts data with the extension-specific codec for format probing
func decodeKCESPartsForProbe(ext string, data []byte) (interface{}, error) {
	switch strings.ToLower(ext) {
	case ".menuassets":
		return serializationKCES.DecodeMenuAssets(data)
	case ".materialassets":
		return serializationKCES.DecodeMaterialAssets(data)
	case ".pmatassets":
		return serializationKCES.DecodePriorityMaterialAssets(data)
	case ".model":
		return serializationKCES.DecodeModel(data)
	case ".kcmenu":
		return serializationKCES.DecodeKCMenu(data)
	case ".kcmat":
		return serializationKCES.DecodeKCMat(data)
	case ".kcmodel":
		return serializationKCES.DecodeKCModel(data)
	default:
		return nil, fmt.Errorf("unsupported KCES parts extension %q", ext)
	}
}

// kcesPartsVersion 从已解码的 KCES 部件值中提取固定宽度版本号
// kcesPartsVersion extracts the fixed-width version from a decoded KCES parts value
func kcesPartsVersion(value interface{}) int32 {
	switch value := value.(type) {
	case *serializationKCES.Model:
		return value.Version
	case *serializationKCES.Menu:
		return value.Version
	case *serializationKCES.Material:
		return value.Version
	case *serializationKCES.MenuAssets:
		if len(value.Assets) > 0 {
			return value.Assets[0].Version
		}
	case *serializationKCES.MaterialAssets:
		if len(value.Assets) > 0 {
			return value.Assets[0].Version
		}
	case *serializationKCES.PriorityMaterialAssets:
		if len(value.Assets) > 0 {
			return value.Assets[0].Version
		}
	}
	return 0
}

// kcesPayloadVersion 从 KCES 载荷根对象中提取固定宽度版本号，未携带版本的载荷返回 0
// kcesPayloadVersion extracts the fixed-width version from a KCES payload root object and returns 0 for payloads that carry none
func kcesPayloadVersion(value any) int32 {
	switch typed := value.(type) {
	case *serializationKCES.DynamicBoneStatus:
		if typed != nil {
			return typed.Version
		}
	case *serializationKCES.ColliderPackage:
		if typed != nil {
			return typed.Version
		}
	case *serializationKCES.LimbColliderPackage:
		if typed != nil {
			return typed.Version
		}
	case *serializationKCES.IKColliderPackage:
		if typed != nil {
			return typed.Version
		}
	}
	return 0
}

// assignKCESVersion 将已类型化的 KCES Int32 版本写入通用文件信息
// assignKCESVersion assigns an already typed KCES Int32 version to common file information
func assignKCESVersion(info *COM3D2Service.FileInfo, version int32) {
	info.Version = version
}

// hasSavedAttachWireSignature 判断文件头是否包含 BinaryWriter 编码的 SAVED_ATTACH_DATA 签名
// hasSavedAttachWireSignature reports whether a header contains the BinaryWriter-encoded SAVED_ATTACH_DATA signature
func hasSavedAttachWireSignature(header []byte) bool {
	// BinaryWriter.Write(string) 使用单字节 7-bit Int32 长度作为该 ASCII 签名的前缀 / BinaryWriter.Write(string) prefixes this ASCII signature with a one-byte 7-bit Int32 length
	signature := []byte(serializationKCES.SavedAttachSignature)
	return len(signature) < 0x80 && len(header) >= len(signature)+1 &&
		header[0] == byte(len(signature)) && bytes.Equal(header[1:len(signature)+1], signature)
}
