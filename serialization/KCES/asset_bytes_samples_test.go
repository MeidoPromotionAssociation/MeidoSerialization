package KCES

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// rawKCESAssetSampleMeta 表示 raw Unity 样本的 sidecar 元数据 / rawKCESAssetSampleMeta represents sidecar metadata for raw Unity samples
type rawKCESAssetSampleMeta struct {
	PathID   int64  `json:"pathId"`             // Unity PathID / Unity PathID
	LoadName string `json:"loadName,omitempty"` // AssetBundle m_Container 加载名 / AssetBundle m_Container load name
}

func TestKCESAssetBytesSamples(t *testing.T) {
	for _, path := range assetSamplePathsBySuffix(t, ".bytes") {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			data := readAssetSampleFile(t, path)
			if len(data) < 4 {
				t.Fatalf("raw Unity object %s too short: %d bytes", filepath.Base(path), len(data))
			}

			metaPath := path + ".meta.json"
			metaData, err := os.ReadFile(metaPath)
			if err != nil {
				t.Fatalf("missing raw Unity meta sidecar %s: %v", metaPath, err)
			}
			var meta rawKCESAssetSampleMeta
			if err := json.Unmarshal(metaData, &meta); err != nil {
				t.Fatalf("decode raw Unity meta sidecar %s: %v", metaPath, err)
			}
			if meta.PathID == 0 {
				t.Fatalf("raw Unity meta sidecar has zero pathId: %+v", meta)
			}
		})
	}
}
