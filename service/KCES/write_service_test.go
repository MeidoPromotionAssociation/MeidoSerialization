package KCES

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	serializationCOM3D2 "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/COM3D2"
	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

func TestKCESDirectWritersMatchNativeEncoders(t *testing.T) {
	mustEncode := func(data []byte, err error) []byte {
		t.Helper()
		if err != nil {
			t.Fatalf("encode expected direct-writer output: %v", err)
		}
		return data
	}

	preset, err := serializationKCES.NewKCESPreset()
	if err != nil {
		t.Fatalf("NewKCESPreset: %v", err)
	}
	expandedPreset, err := serializationKCES.ExpandKCESPreset(preset)
	if err != nil {
		t.Fatalf("ExpandKCESPreset: %v", err)
	}

	payloadWire := mustEncode(serializationKCES.EncodeDynamicBoneStatusFile(&serializationKCES.DynamicBoneStatus{
		Version: 1000,
		Gravity: serializationKCES.Vector3{Y: -0.05},
	}))
	payload, err := serializationKCES.DecodeKCESPayload(payloadWire, ".dbconf")
	if err != nil {
		t.Fatalf("DecodeKCESPayload: %v", err)
	}

	bridge := &GP03BridgeEditing{
		Format:    serializationKCES.KCESGP03BridgeFormat,
		Signature: serializationKCES.GP03BridgeSignature,
		Version:   serializationKCES.GP03BridgeVersion,
		GUID:      "direct-writer",
	}
	bridgeSession := serializationKCES.NewKCESBridgeSession("direct-writer")
	maidCollider := &serializationKCES.MaidColliderFile{
		Format:    serializationKCES.MaidColliderFormat,
		Colliders: []serializationKCES.MaidCapsuleCollider{},
	}
	exportNameMap := &serializationKCES.KCESExportNameMap{
		Format:  serializationKCES.KCESExportNameMapFormat,
		Version: serializationKCES.KCESExportNameMapVersion,
		Entries: []serializationKCES.KCESExportNameMapEntry{},
	}
	paths := serializationKCES.NewKCESPathsFile()
	paths.Paths = []string{"system", "parts"}
	savedAttach := serializationKCES.NewSavedAttachFile()
	savedAttach.Items = []serializationKCES.SavedAttachData{}
	systemData := serializationKCES.NewKCESSystemData()
	menuAssets := &serializationKCES.MenuAssets{Assets: []*serializationKCES.Menu{}}
	materialAssets := &serializationKCES.MaterialAssets{Assets: []*serializationKCES.Material{}}
	priorityMaterialAssets := &serializationKCES.PriorityMaterialAssets{Assets: []*serializationKCES.PriorityMaterial{}}
	model := &serializationKCES.Model{}
	hitCheck := serializationKCES.NewHitCheck()
	hitCheck.Entries = []serializationKCES.HitCheckEntry{}
	jsonText := &serializationKCES.KCESJSONText{
		Extension: ".nson",
		JSON:      []byte(`{"version":1000,"ids":[1,2]}`),
	}
	psk := &serializationCOM3D2.Psk{Signature: "CM3D21_PSK", Version: 217}
	var expectedPsk bytes.Buffer
	if err := psk.Dump(&expectedPsk); err != nil {
		t.Fatalf("encode expected psk: %v", err)
	}
	nei := &serializationCOM3D2.Nei{
		Rows: 1,
		Cols: 2,
		Data: [][]string{{"id", "name"}},
	}
	var expectedNei bytes.Buffer
	if err := nei.Dump(&expectedNei); err != nil {
		t.Fatalf("encode expected nei: %v", err)
	}

	table := &ct.ContentTable{
		Version: 1000,
		Files:   make(map[string]ct.VirtualFile),
		Raw:     make([]byte, ct.HeaderSize),
	}
	if err := table.AddFile("data/example", []byte{1, 2, 3}); err != nil {
		t.Fatalf("AddFile: %v", err)
	}
	var expectedCt bytes.Buffer
	if err := ct.WriteContentTable(&expectedCt, table); err != nil {
		t.Fatalf("encode expected content table: %v", err)
	}

	tests := []struct {
		name     string
		fileName string
		expected []byte
		write    func(string) error
	}{
		{
			name:     "bridge",
			fileName: "direct.brd",
			expected: mustEncode(encodeGP03BridgeEditing(bridge)),
			write: func(path string) error {
				return (&GP03BridgeService{}).WriteBridgeFile(path, bridge)
			},
		},
		{
			name:     "bridge session",
			fileName: "bridge_session.vd",
			expected: mustEncode(serializationKCES.EncodeKCESBridgeSession(bridgeSession)),
			write: func(path string) error {
				return (&BridgeSessionService{}).WriteBridgeSessionFile(path, bridgeSession)
			},
		},
		{
			name:     "maid collider",
			fileName: "maid_collider.bytes",
			expected: mustEncode(serializationKCES.EncodeMaidCollider(maidCollider)),
			write: func(path string) error {
				return (&MaidColliderService{}).WriteMaidColliderFile(path, maidCollider)
			},
		},
		{
			name:     "export name map",
			fileName: "export_map.enm",
			expected: mustEncode(serializationKCES.EncodeKCESExportNameMap(exportNameMap)),
			write: func(path string) error {
				return (&ExportNameMapService{}).WriteExportNameMapFile(path, exportNameMap)
			},
		},
		{
			name:     "paths",
			fileName: "paths.dat",
			expected: mustEncode(serializationKCES.EncodeKCESPaths(paths)),
			write: func(path string) error {
				return (&PathsService{}).WritePathsFile(path, paths)
			},
		},
		{
			name:     "payload",
			fileName: "direct.dbconf",
			expected: mustEncode(serializationKCES.EncodeKCESPayload(payload, ".dbconf")),
			write: func(path string) error {
				return (&PayloadService{}).WritePayloadFile(path, payload)
			},
		},
		{
			name:     "preset",
			fileName: "direct.preset",
			expected: mustEncode(serializationKCES.EncodeExpandedKCESPreset(expandedPreset)),
			write: func(path string) error {
				return (&PresetService{}).WritePresetFile(path, expandedPreset)
			},
		},
		{
			name:     "saved attach",
			fileName: "direct.sad",
			expected: mustEncode(serializationKCES.EncodeSavedAttach(savedAttach)),
			write: func(path string) error {
				return (&SavedAttachService{}).WriteSavedAttachFile(path, savedAttach)
			},
		},
		{
			name:     "system data",
			fileName: "system.dat",
			expected: mustEncode(serializationKCES.EncodeKCESSystemData(systemData)),
			write: func(path string) error {
				return (&SystemDataService{}).WriteSystemDataFile(path, systemData)
			},
		},
		{
			name:     "menuassets",
			fileName: "direct.menuassets",
			expected: mustEncode(serializationKCES.EncodeMenuAssets(menuAssets)),
			write: func(path string) error {
				return (&PartsService{}).WritePartsFile(path, menuAssets)
			},
		},
		{
			name:     "materialassets",
			fileName: "direct.materialassets",
			expected: mustEncode(serializationKCES.EncodeMaterialAssets(materialAssets)),
			write: func(path string) error {
				return (&PartsService{}).WritePartsFile(path, materialAssets)
			},
		},
		{
			name:     "pmatassets",
			fileName: "direct.pmatassets",
			expected: mustEncode(serializationKCES.EncodePriorityMaterialAssets(priorityMaterialAssets)),
			write: func(path string) error {
				return (&PartsService{}).WritePartsFile(path, priorityMaterialAssets)
			},
		},
		{
			name:     "model",
			fileName: "direct.model",
			expected: mustEncode(serializationKCES.EncodeModelWithOptions(model, &serializationKCES.LookupHashOptions{RecalculateHash: true, FileName: "direct.model"})),
			write: func(path string) error {
				return (&PartsService{}).WritePartsFile(path, model)
			},
		},
		{
			name:     "hitcheck",
			fileName: "direct.hitcheck",
			expected: mustEncode(serializationKCES.EncodeHitCheck(hitCheck)),
			write: func(path string) error {
				return (&MiscService{}).WriteMiscFile(path, hitCheck)
			},
		},
		{
			name:     "JSON text",
			fileName: "direct.nson",
			expected: mustEncode(serializationKCES.EncodeKCESJSONText(jsonText)),
			write: func(path string) error {
				return (&MiscService{}).WriteMiscFile(path, jsonText)
			},
		},
		{
			name:     "shared psk",
			fileName: "direct.psk",
			expected: expectedPsk.Bytes(),
			write: func(path string) error {
				return (&DataService{}).WriteDataFile(path, psk)
			},
		},
		{
			name:     "shared nei",
			fileName: "direct.nei",
			expected: expectedNei.Bytes(),
			write: func(path string) error {
				return (&DataService{}).WriteDataFile(path, nei)
			},
		},
		{
			name:     "content table",
			fileName: "direct.ct",
			expected: expectedCt.Bytes(),
			write: func(path string) error {
				return (&CtService{}).WriteCtFile(path, table)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), test.fileName)
			if err := test.write(path); err != nil {
				t.Fatalf("direct write: %v", err)
			}
			actual, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read direct output: %v", err)
			}
			if !bytes.Equal(actual, test.expected) {
				t.Fatalf("direct output differs from native encoder\n got  %x\n want %x", actual, test.expected)
			}
		})
	}
}

func TestWriteCtEnvelopeFileProducesReadableContentTable(t *testing.T) {
	catalogName := "direct"
	catalog := ct.AssetBundleCatalog{
		Kind:          ct.CatalogKindVirtualAsset,
		Version:       1000,
		CatalogType:   ct.CatalogTypeParts,
		PackageType:   ct.PackageTypePlugin,
		Name:          &catalogName,
		ExtensionList: []*string{},
		VirtualItems:  []*ct.VirtualCatalogItem{},
	}
	envelope := &CtEnvelope{
		Format:  CtEnvelopeFormat,
		Version: 1000,
		Catalog: catalog,
		Files: []CtEnvelopeFile{{
			Name:       "raw/data",
			DataBase64: base64.StdEncoding.EncodeToString([]byte{4, 5, 6}),
		}},
	}
	path := filepath.Join(t.TempDir(), "direct.ct")
	if err := (&CtService{}).WriteCtEnvelopeFile(path, envelope); err != nil {
		t.Fatalf("WriteCtEnvelopeFile: %v", err)
	}
	table, err := (&CtService{}).ReadCt(path)
	if err != nil {
		t.Fatalf("ReadCt: %v", err)
	}
	data, err := table.GetFileData("raw/data")
	if err != nil {
		t.Fatalf("GetFileData: %v", err)
	}
	if !bytes.Equal(data, []byte{4, 5, 6}) {
		t.Fatalf("raw/data = %v", data)
	}
}

func TestWriteRawUnityObjectFileWritesNativeDataAndSidecars(t *testing.T) {
	raw := []byte{3, 0, 0, 0, 'f', 'o', 'o', 0}
	value := &RawUnityObjectEnvelope{
		Format:                RawUnityObjectFormat,
		ClassID:               21,
		PathID:                42,
		LoadName:              "assets/direct",
		SerializedFileVersion: 22,
		DataBase64:            base64.StdEncoding.EncodeToString(raw),
		TypeTree: &RawUnityTypeTreeEnvelope{
			Format:  RawUnityTypeTreeFormat,
			ClassID: 21,
			PathID:  42,
		},
	}
	path := filepath.Join(t.TempDir(), "direct.material.bytes")
	if err := (&RawUnityObjectService{}).WriteRawUnityObjectFile(path, value); err != nil {
		t.Fatalf("WriteRawUnityObjectFile: %v", err)
	}
	if actual := mustReadFile(t, path); !bytes.Equal(actual, raw) {
		t.Fatalf("raw output = %x, want %x", actual, raw)
	}
	if _, err := os.Stat(assetMetaPath(path)); err != nil {
		t.Fatalf("metadata sidecar: %v", err)
	}
	if _, err := os.Stat(typeTreeSidecarPath(path)); err != nil {
		t.Fatalf("TypeTree sidecar: %v", err)
	}
}

func TestKCESDirectWritersValidateBeforeCreatingOutput(t *testing.T) {
	dir := t.TempDir()
	invalidPaths := serializationKCES.NewKCESPathsFile()
	invalidPaths.Format = "invalid"
	pathsOutput := filepath.Join(dir, "paths.dat")
	if err := (&PathsService{}).WritePathsFile(pathsOutput, invalidPaths); err == nil {
		t.Fatal("WritePathsFile accepted an invalid format")
	}
	if _, err := os.Stat(pathsOutput); !os.IsNotExist(err) {
		t.Fatalf("invalid paths output exists or stat failed unexpectedly: %v", err)
	}

	partsOutput := filepath.Join(dir, "mismatch.model")
	if err := (&PartsService{}).WritePartsFile(partsOutput, &serializationKCES.MenuAssets{}); err == nil {
		t.Fatal("WritePartsFile accepted a value that does not match the output extension")
	}
	if _, err := os.Stat(partsOutput); !os.IsNotExist(err) {
		t.Fatalf("mismatched parts output exists or stat failed unexpectedly: %v", err)
	}
}
