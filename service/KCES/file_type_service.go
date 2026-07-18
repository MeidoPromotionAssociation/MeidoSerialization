package KCES

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
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

// FileTypeService probes file contents for formats introduced by KCES.
//
// KCES and COM3D2 share a few extensions, most notably .model and .preset.
// CLI callers first run CommonService.TryFileTypeDetermine, which only accepts
// exact COM3D2 signatures, then call this probe before broader legacy extension
// and CSV heuristics. A matched result is always backed by the corresponding
// KCES decoder (or, for encrypted abap, its explicit wire signature);
// extensions alone are not treated as proof.
type FileTypeService struct{}

// TryFileTypeDetermine returns matched=false for files which are not proven to
// be KCES files. A non-nil error with matched=true means that the path is a
// KCES-only candidate (or has a KCES magic/marker) but its contents are
// malformed. This distinction prevents malformed binary data from falling
// through to the legacy CSV heuristic.
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
	// JSON detector, which only understands capitalized COM3D2 headers.
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

	// Standard KCES AssetBundles use UnityFS. ReadBundle validates the complete
	// header, file-size declaration, block metadata and supported version without
	// decompressing every asset payload.
	if bytes.HasPrefix(header, []byte(kcesUnityFSSignature+"\x00")) {
		if _, seekErr := file.Seek(0, 0); seekErr != nil {
			return info, true, seekErr
		}
		bundle, bundleErr := aba.ReadBundle(file)
		if bundleErr != nil {
			return info, true, fmt.Errorf("validate KCES UnityFS file %q: %w", path, bundleErr)
		}
		info.FileType = "aba"
		if ext == aba.AssetSceneExtension {
			info.FileType = "asset_scene"
		}
		info.StorageFormat = COM3D2Service.FormatBinary
		info.Game = COM3D2Service.GameKCES
		info.Signature = kcesUnityFSSignature
		info.Version = int32(bundle.Header.Version)
		return info, true, nil
	}
	// abap is an encrypted KCES AssetBundle. The parser intentionally cannot
	// decrypt it yet, but its four-byte signature is unambiguous and determine
	// should report that fact instead of calling it CSV.
	if bytes.HasPrefix(header, []byte(kcesEncryptedABASignature)) {
		info.FileType = "aba"
		info.StorageFormat = COM3D2Service.FormatBinary
		info.Game = COM3D2Service.GameKCES
		info.Signature = kcesEncryptedABASignature
		return info, true, nil
	}

	// .ct, current KCES presets and system.dat all use VirtualDirectory's
	// seven-byte signature. The virtual-file names distinguish their semantic
	// payloads even when the outer file was renamed.
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
		if versionErr := assignKCESVersion(&info, table.Version); versionErr != nil {
			return info, true, versionErr
		}
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
			// chance. The three *assets extensions are KCES-only.
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
			if versionErr := assignKCESVersion(&info, version); versionErr != nil {
				return info, true, versionErr
			}
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
		if envelope.Format == serializationKCES.PayloadFormatKCESExportCM {
			info.Signature = envelope.Format
		} else {
			info.Signature = kcesMessagePackSignature
		}
		if version := kcesPayloadVersion(envelope); version != 0 {
			if versionErr := assignKCESVersion(&info, version); versionErr != nil {
				return info, true, versionErr
			}
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
		// record count, not a version number.
		return info, true, nil
	}

	if IsKCESGP03BridgeFile(path) || serializationKCES.IsGP03BridgeData(header) {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return info, true, readErr
		}
		value, decodeErr := serializationKCES.DecodeGP03Bridge(data)
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
	// than allowing the legacy one-record CSV heuristic to misclassify the file.
	switch ext {
	case ".ct", ".aba", aba.AssetSceneExtension, serializationKCES.KCESPersetExtension:
		return info, true, fmt.Errorf("%q does not contain a valid KCES %s signature", path, strings.TrimPrefix(ext, "."))
	}

	return info, false, nil
}

func probeKCESJSON(path string, data []byte, info COM3D2Service.FileInfo) (COM3D2Service.FileInfo, bool, error) {
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	lowerPath := strings.ToLower(path)
	ext := strings.ToLower(filepath.Ext(path))

	// export_map.enm is itself a Unity JsonUtility document, while .enm.json
	// is our marker-based editing form. Their extensions are KCES-specific, so
	// malformed candidates remain matched and return their validation error
	// rather than falling through to the legacy JSON/CSV heuristics.
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
		if err := assignKCESVersion(&info, value.ContainerVersion); err != nil {
			return info, true, err
		}
		return info, true, nil
	}

	// Native .undressdat/.undresspdat/.nson files are themselves JSON. Their
	// extensions are KCES-specific, so malformed JSON remains a matched KCES
	// candidate with a useful validation error instead of falling through to
	// legacy CSV/JSON heuristics.
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
	// through to the legacy COM3D2 JSON or CSV probes.
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
		Version          int    `json:"version"`
		ContainerVersion int    `json:"containerVersion"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return info, false, nil
	}

	info.StorageFormat = COM3D2Service.FormatJSON
	info.Game = COM3D2Service.GameKCES
	if header.Version != 0 {
		if err := assignKCESVersion(&info, header.Version); err != nil {
			return info, true, err
		}
	}

	switch header.Format {
	case serializationKCES.KCESBridgeSessionFormat:
		value, err := decodeKCESBridgeSessionEditingJSON(data)
		if err != nil {
			return info, true, fmt.Errorf("validate KCES bridge session JSON %q: %w", path, err)
		}
		info.FileType = "bridge_session"
		info.Signature = serializationKCES.KCESBridgeSessionFormat
		if err := assignKCESVersion(&info, value.ContainerVersion); err != nil {
			return info, true, err
		}
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
		if err := assignKCESVersion(&info, value.Version); err != nil {
			return info, true, err
		}
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
		if err := assignKCESVersion(&info, envelope.Version); err != nil {
			return info, true, err
		}
		return info, true, nil
	case serializationKCES.KCESPresetFormat:
		var preset serializationKCES.KCESPreset
		if err := decodeStrictJSON(data, &preset, "KCES preset JSON"); err != nil {
			return info, true, fmt.Errorf("validate KCES preset JSON %q: %w", path, err)
		}
		if _, err := serializationKCES.EncodeKCESPreset(&preset); err != nil {
			return info, true, fmt.Errorf("validate KCES preset JSON %q: %w", path, err)
		}
		info.FileType = "preset"
		info.Signature = serializationKCES.KCESPresetFormat
		if err := assignKCESVersion(&info, preset.ContainerVersion); err != nil {
			return info, true, err
		}
		return info, true, nil
	case serializationKCES.PayloadFormatKCESMessagePack, serializationKCES.PayloadFormatKCESExportCM:
		var envelope serializationKCES.KCESPayloadEnvelope
		if err := decodeStrictJSON(data, &envelope, "KCES payload JSON"); err != nil {
			return info, true, fmt.Errorf("validate KCES payload JSON %q: %w", path, err)
		}
		if envelope.Extension == "" {
			envelope.Extension = serializationKCES.NormalizeKCESPayloadExtension(strings.TrimSuffix(path, filepath.Ext(path)))
		}
		if _, err := serializationKCES.EncodeKCESPayload(&envelope); err != nil {
			return info, true, fmt.Errorf("validate KCES payload JSON %q: %w", path, err)
		}
		payloadExt := serializationKCES.NormalizeKCESPayloadExtension(envelope.Extension)
		if payloadExt == "" {
			return info, true, fmt.Errorf("validate KCES payload JSON %q: unsupported extension %q", path, envelope.Extension)
		}
		info.FileType = strings.TrimPrefix(payloadExt, ".")
		info.Signature = envelope.Format
		if version := kcesPayloadVersion(&envelope); version != 0 {
			if err := assignKCESVersion(&info, version); err != nil {
				return info, true, err
			}
		}
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
				// capitalized legacy schema must remain available to the fallback.
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

// probeKCESOnlyEditingJSONByPath handles JSON schemas whose inner extension
// is exclusive to KCES. Shared .model.json and .preset/.perset.json remain in
// the marker/content-based path below so legacy COM3D2 JSON still gets a
// chance. A recognized path is a KCES candidate even when its JSON is
// truncated; this is what prevents fallback to the legacy CSV heuristic.
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
		if err := assignKCESVersion(&info, value.Version); err != nil {
			return info, true, err
		}
		return info, true, nil
	}

	if innerExt == ".menuassets" || innerExt == ".materialassets" || innerExt == ".pmatassets" {
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
		envelope, err := decodeKCESPayloadEditingJSON(data, payloadExt)
		if err != nil {
			return info, true, fmt.Errorf("validate KCES payload JSON %q: %w", path, err)
		}
		info.FileType = strings.TrimPrefix(payloadExt, ".")
		info.StorageFormat = COM3D2Service.FormatJSON
		info.Game = COM3D2Service.GameKCES
		info.Signature = envelope.Format
		if version := kcesPayloadVersion(envelope); version != 0 {
			if err := assignKCESVersion(&info, version); err != nil {
				return info, true, err
			}
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
		if err := assignKCESVersion(&info, envelope.Version); err != nil {
			return info, true, err
		}
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

func kcesPartsType(ext string) (string, bool) {
	switch strings.ToLower(ext) {
	case ".menuassets", ".materialassets", ".pmatassets", ".model":
		return strings.TrimPrefix(strings.ToLower(ext), "."), true
	default:
		return "", false
	}
}

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
	default:
		return nil, fmt.Errorf("unsupported KCES parts extension %q", ext)
	}
}

func kcesPartsVersion(value interface{}) int {
	switch value := value.(type) {
	case *serializationKCES.Model:
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

func kcesPayloadVersion(envelope *serializationKCES.KCESPayloadEnvelope) int {
	if envelope == nil {
		return 0
	}
	if envelope.DynamicBone != nil {
		return envelope.DynamicBone.Version
	}
	if envelope.ColliderPackage != nil {
		return envelope.ColliderPackage.Version
	}
	if envelope.LimbCollider != nil {
		return envelope.LimbCollider.Version
	}
	if envelope.IKCollider != nil {
		return envelope.IKCollider.Version
	}
	return 0
}

func assignKCESVersion(info *COM3D2Service.FileInfo, version int) error {
	if version < math.MinInt32 || version > math.MaxInt32 {
		return fmt.Errorf("KCES version %d is outside the CLR Int32 range", version)
	}
	info.Version = int32(version)
	return nil
}

func hasSavedAttachWireSignature(header []byte) bool {
	// BinaryWriter.Write(string) prefixes this ASCII signature with its UTF-8
	// byte length as a one-byte 7-bit encoded Int32.
	signature := []byte(serializationKCES.SavedAttachSignature)
	return len(signature) < 0x80 && len(header) >= len(signature)+1 &&
		header[0] == byte(len(signature)) && bytes.Equal(header[1:len(signature)+1], signature)
}
