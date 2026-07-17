package KCES

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	serializationKCES "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES"
)

// IsKCESExportNameMapFile reports whether path names a native
// export_map.enm-style Unity JsonUtility document. The game uses the fixed
// basename export_map.enm, but accepting any .enm basename is useful when
// inspecting renamed/copy files; content probing still validates the payload.
func IsKCESExportNameMapFile(path string) bool {
	return !strings.HasSuffix(strings.ToLower(path), ".json") && strings.EqualFold(filepath.Ext(path), ".enm")
}

// IsKCESExportNameMapJSONFile reports whether path is a valid marker-based
// editing document. It deliberately validates the complete document instead
// of trusting the format field alone.
func IsKCESExportNameMapJSONFile(path string) bool {
	if !strings.HasSuffix(strings.ToLower(path), ".enm.json") {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	_, err = serializationKCES.DecodeKCESExportNameMapJSON(data)
	return err == nil
}

// ExportNameMapService converts between the game's nested JsonUtility layout
// and the deterministic, entry-based editing JSON representation.
type ExportNameMapService struct{}

func (s *ExportNameMapService) ConvertExportNameMapToJSON(inputPath, outputPath string) error {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read KCES export name map %q: %w", inputPath, err)
	}
	value, err := serializationKCES.DecodeKCESExportNameMap(data)
	if err != nil {
		return fmt.Errorf("decode KCES export name map %q: %w", inputPath, err)
	}
	encoded, err := serializationKCES.EncodeKCESExportNameMapJSON(value)
	if err != nil {
		return fmt.Errorf("encode KCES export name map editing JSON: %w", err)
	}
	if err := os.WriteFile(outputPath, encoded, 0644); err != nil {
		return fmt.Errorf("write KCES export name map JSON %q: %w", outputPath, err)
	}
	return nil
}

func (s *ExportNameMapService) ConvertJSONToExportNameMap(inputPath, outputPath string) error {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read KCES export name map JSON %q: %w", inputPath, err)
	}
	value, err := serializationKCES.DecodeKCESExportNameMapJSON(data)
	if err != nil {
		return fmt.Errorf("decode KCES export name map JSON %q: %w", inputPath, err)
	}
	encoded, err := serializationKCES.EncodeKCESExportNameMap(value)
	if err != nil {
		return fmt.Errorf("encode native KCES export name map: %w", err)
	}
	if err := os.WriteFile(outputPath, encoded, 0644); err != nil {
		return fmt.Errorf("write native KCES export name map %q: %w", outputPath, err)
	}
	return nil
}
