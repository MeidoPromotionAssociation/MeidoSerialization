package KCES

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/aba"
)

func TestKCES2AbaTextAssetCodecsPreserveRealLayouts(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "KCES2", "*_kces2*.aba"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Skip("no KCES2 ABA samples")
	}
	menuCount := 0
	materialCount := 0
	modelCount := 0
	menuKCES2Count := 0
	materialKCES2Count := 0
	for _, path := range paths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			visitKCES2TextAssets(t, path, func(name string, data []byte) error {
				switch strings.ToLower(filepath.Ext(name)) {
				case ".menuassets":
					value, err := DecodeMenuAssets(data)
					if err != nil {
						return err
					}
					reencoded, err := EncodeMenuAssets(value)
					if err != nil {
						return err
					}
					roundTrip, err := DecodeMenuAssets(reencoded)
					if err != nil {
						return err
					}
					if !reflect.DeepEqual(roundTrip, value) {
						return &kces2SampleMismatchError{format: name, message: "MenuAssets changed after re-encoding"}
					}
					menuCount++
					for _, menu := range value.Assets {
						if menu != nil && menu.MessagePackIndexedObjectWidth() == menuKCES2Width {
							menuKCES2Count++
						}
					}
				case ".materialassets":
					value, err := DecodeMaterialAssets(data)
					if err != nil {
						return err
					}
					reencoded, err := EncodeMaterialAssets(value)
					if err != nil {
						return err
					}
					roundTrip, err := DecodeMaterialAssets(reencoded)
					if err != nil {
						return err
					}
					if !reflect.DeepEqual(roundTrip, value) {
						return &kces2SampleMismatchError{format: name, message: "MaterialAssets changed after re-encoding"}
					}
					materialCount++
					for _, material := range value.Assets {
						if material != nil && material.MessagePackIndexedObjectWidth() == materialKCES2Width {
							materialKCES2Count++
						}
					}
				case ".model":
					value, err := DecodeModel(data)
					if err != nil {
						return err
					}
					reencoded, err := EncodeModel(value)
					if err != nil {
						return err
					}
					roundTrip, err := DecodeModel(reencoded)
					if err != nil {
						return err
					}
					if !reflect.DeepEqual(roundTrip, value) {
						return &kces2SampleMismatchError{format: name, message: "Model changed after re-encoding"}
					}
					modelCount++
				case ".presetcolor":
					value, err := DecodePresetColor(data)
					if err != nil {
						return err
					}
					reencoded, err := EncodePresetColor(value)
					if err != nil {
						return err
					}
					roundTrip, err := DecodePresetColor(reencoded)
					if err != nil {
						return err
					}
					if !reflect.DeepEqual(roundTrip, value) {
						return &kces2SampleMismatchError{format: name, message: "PresetColor changed after re-encoding"}
					}
				case ".system":
					value, err := DecodeKCESSystemData(data)
					if err != nil {
						return err
					}
					reencoded, err := EncodeKCESSystemData(value)
					if err != nil {
						return err
					}
					if _, err := DecodeKCESSystemData(reencoded); err != nil {
						return err
					}
				}
				return nil
			})
		})
	}
	if menuCount == 0 || materialCount == 0 || modelCount == 0 {
		t.Fatalf("real KCES2 TextAssets found menuassets=%d materialassets=%d model=%d", menuCount, materialCount, modelCount)
	}
	if menuKCES2Count == 0 || materialKCES2Count == 0 {
		t.Fatalf("real KCES2 layouts not observed: Menu=%d Material=%d", menuKCES2Count, materialKCES2Count)
	}
}

func visitKCES2TextAssets(t *testing.T, path string, visit func(string, []byte) error) {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open ABA: %v", err)
	}
	defer file.Close()
	bundle, err := aba.ReadAba(file)
	if err != nil {
		t.Fatalf("read ABA: %v", err)
	}
	for directoryIndex, entry := range bundle.BlockInfo.DirectoryInfos {
		if !entry.IsSerialized() {
			continue
		}
		assetsFile, err := aba.ReadAssetsFileRange(entry.DecompressedSize, func(offset int64, size int64) ([]byte, error) {
			return bundle.GetFileDataRange(int64(directoryIndex), offset, size)
		})
		if err != nil {
			t.Fatalf("read SerializedFile %q: %v", entry.Name, err)
		}
		for assetIndex := range assetsFile.Metadata.AssetInfos {
			info := &assetsFile.Metadata.AssetInfos[assetIndex]
			if info.TypeId != aba.ClassIDTextAsset {
				continue
			}
			name, script, err := assetsFile.GetTextAssetData(info)
			if err != nil {
				t.Fatalf("read TextAsset %q: %v", name, err)
			}
			if err := visit(name, script); err != nil {
				t.Fatalf("decode TextAsset %q: %v", name, err)
			}
		}
	}
}

type kces2SampleMismatchError struct {
	format  string
	message string
}

func (err *kces2SampleMismatchError) Error() string {
	return err.format + ": " + err.message
}

func TestKCES2SampleSystemDataPreservesUnknownBytes(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "KCES2", "system.dat")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t.Skip("no KCES2 user system.dat sample")
	}
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeKCESSystemData(data)
	if err != nil {
		t.Fatal(err)
	}
	reencoded, err := EncodeKCESSystemData(decoded)
	if err != nil {
		t.Fatal(err)
	}
	roundTrip, err := DecodeKCESSystemData(reencoded)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(roundTrip, decoded) {
		t.Fatal("system.dat semantic structure changed after re-encoding")
	}
	for virtualPath, original := range decoded.ExtraFiles {
		if !bytes.Equal(roundTrip.ExtraFiles[virtualPath], original) {
			t.Fatalf("unknown virtual-file bytes changed after re-encoding: %s", virtualPath)
		}
	}
}
