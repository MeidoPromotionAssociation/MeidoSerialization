package COM3D2

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/COM3D2"
)

func TestModelService(t *testing.T) {
	files, err := filepath.Glob("../../testdata/*.model")
	if err != nil {
		t.Fatal(err)
	}

	s := &ModelService{}
	for _, inputPath := range files {
		t.Run(filepath.Base(inputPath), func(t *testing.T) {
			tempDir := t.TempDir()
			outputPath := filepath.Join(tempDir, "test.model")
			jsonPath := filepath.Join(tempDir, "test.model.json")
			backPath := filepath.Join(tempDir, "test_back.model")

			// 1. Test ReadModelFile
			model, err := s.ReadModelFile(inputPath)
			if err != nil {
				t.Fatalf("ReadModelFile failed: %v", err)
			}
			if model == nil {
				t.Fatal("ReadModelFile returned nil")
			}

			// 2. Test WriteModelFile
			err = s.WriteModelFile(outputPath, model)
			if err != nil {
				t.Fatalf("WriteModelFile failed: %v", err)
			}

			// 3. Test ConvertModelToJson
			err = s.ConvertModelToJson(TestConversionContext, inputPath, jsonPath, TestConversionMaxOutput)
			if err != nil {
				t.Fatalf("ConvertModelToJson failed: %v", err)
			}

			// 4. Test ConvertJsonToModel
			err = s.ConvertJsonToModel(TestConversionContext, jsonPath, backPath, TestConversionMaxOutput)
			if err != nil {
				t.Fatalf("ConvertJsonToModel failed: %v", err)
			}

			// Re-read and verify consistency
			modelBack, err := s.ReadModelFile(backPath)
			if err != nil {
				t.Fatalf("Read re-converted model failed: %v", err)
			}
			if !reflect.DeepEqual(model, modelBack) {
				t.Errorf("data mismatch after JSON conversion cycle")
			}

			// Also verify direct write consistency
			modelRepack, err := s.ReadModelFile(outputPath)
			if err != nil {
				t.Fatalf("Read re-written model failed: %v", err)
			}
			if !reflect.DeepEqual(model, modelRepack) {
				t.Errorf("data mismatch after direct write")
			}
		})
	}
}

func TestReadModelMetadata(t *testing.T) {
	files, err := filepath.Glob("../../testdata/*.model")
	if err != nil {
		t.Fatal(err)
	}

	s := &ModelService{}
	for _, inputPath := range files {
		t.Run(filepath.Base(inputPath), func(t *testing.T) {
			metadata, err := s.ReadModelMetadata(inputPath)
			if err != nil {
				t.Fatalf("ReadModelMetadata failed: %v", err)
			}
			if metadata == nil {
				t.Fatal("ReadModelMetadata returned nil")
			}

			model, err := s.ReadModelFile(inputPath)
			if err != nil {
				t.Fatalf("ReadModelFile failed: %v", err)
			}

			if metadata.Signature != model.Signature {
				t.Errorf("Signature mismatch: got %q, want %q", metadata.Signature, model.Signature)
			}
			if metadata.Version != model.Version {
				t.Errorf("Version mismatch: got %d, want %d", metadata.Version, model.Version)
			}
			if metadata.Name != model.Name {
				t.Errorf("Name mismatch: got %q, want %q", metadata.Name, model.Name)
			}
			if metadata.RootBoneName != model.RootBoneName {
				t.Errorf("RootBoneName mismatch: got %q, want %q", metadata.RootBoneName, model.RootBoneName)
			}
			if len(metadata.Materials) != len(model.Materials) {
				t.Errorf("Materials count mismatch: got %d, want %d", len(metadata.Materials), len(model.Materials))
			}
		})
	}
}

func TestWriteModelMetadata(t *testing.T) {
	files, err := filepath.Glob("../../testdata/*.model")
	if err != nil {
		t.Fatal(err)
	}

	s := &ModelService{}
	for _, inputPath := range files {
		t.Run(filepath.Base(inputPath), func(t *testing.T) {
			tempDir := t.TempDir()
			outputPath := filepath.Join(tempDir, "test_metadata_modified.model")

			originalMetadata, err := s.ReadModelMetadata(inputPath)
			if err != nil {
				t.Fatalf("ReadModelMetadata failed: %v", err)
			}

			modifiedMetadata := &COM3D2.ModelMetadata{
				Signature:         originalMetadata.Signature,
				Version:           originalMetadata.Version,
				Name:              "modified_name_test",
				RootBoneName:      "ModifiedRoot",
				ShadowCastingMode: originalMetadata.ShadowCastingMode,
				Materials:         originalMetadata.Materials,
			}

			err = s.WriteModelMetadata(inputPath, outputPath, modifiedMetadata)
			if err != nil {
				t.Fatalf("WriteModelMetadata failed: %v", err)
			}

			readBackMetadata, err := s.ReadModelMetadata(outputPath)
			if err != nil {
				t.Fatalf("Read modified metadata failed: %v", err)
			}

			if readBackMetadata.Name != "modified_name_test" {
				t.Errorf("Name not modified: got %q, want %q", readBackMetadata.Name, "modified_name_test")
			}
			if readBackMetadata.RootBoneName != "ModifiedRoot" {
				t.Errorf("RootBoneName not modified: got %q, want %q", readBackMetadata.RootBoneName, "ModifiedRoot")
			}
			if !reflect.DeepEqual(readBackMetadata.Materials, originalMetadata.Materials) {
				t.Errorf("Materials were unexpectedly modified")
			}

			model, err := s.ReadModelFile(outputPath)
			if err != nil {
				t.Fatalf("ReadModelFile on modified file failed: %v", err)
			}
			if model.Name != "modified_name_test" {
				t.Errorf("Full model Name mismatch: got %q, want %q", model.Name, "modified_name_test")
			}
		})
	}
}

func TestReadModelMaterial(t *testing.T) {
	files, err := filepath.Glob("../../testdata/*.model")
	if err != nil {
		t.Fatal(err)
	}

	s := &ModelService{}
	for _, inputPath := range files {
		t.Run(filepath.Base(inputPath), func(t *testing.T) {
			materials, err := s.ReadModelMaterial(inputPath)
			if err != nil {
				t.Fatalf("ReadModelMaterial failed: %v", err)
			}

			metadata, err := s.ReadModelMetadata(inputPath)
			if err != nil {
				t.Fatalf("ReadModelMetadata failed: %v", err)
			}

			if !reflect.DeepEqual(materials, metadata.Materials) {
				t.Errorf("ReadModelMaterial returned different materials than ReadModelMetadata")
			}
		})
	}
}

func TestWriteModelMaterial(t *testing.T) {
	files, err := filepath.Glob("../../testdata/*.model")
	if err != nil {
		t.Fatal(err)
	}

	s := &ModelService{}
	for _, inputPath := range files {
		t.Run(filepath.Base(inputPath), func(t *testing.T) {
			tempDir := t.TempDir()
			outputPath := filepath.Join(tempDir, "test_material_modified.model")

			originalMaterials, err := s.ReadModelMaterial(inputPath)
			if err != nil {
				t.Fatalf("ReadModelMaterial failed: %v", err)
			}

			if len(originalMaterials) == 0 {
				t.Skip("No materials to test modification")
			}

			modifiedMaterials := make([]*COM3D2.Material, len(originalMaterials))
			for i, mat := range originalMaterials {
				modifiedMaterials[i] = &COM3D2.Material{
					Name:           "modified_material_" + mat.Name,
					ShaderName:     mat.ShaderName,
					ShaderFilename: mat.ShaderFilename,
					Properties:     mat.Properties,
				}
			}

			err = s.WriteModelMaterial(inputPath, outputPath, modifiedMaterials)
			if err != nil {
				t.Fatalf("WriteModelMaterial failed: %v", err)
			}

			readBackMaterials, err := s.ReadModelMaterial(outputPath)
			if err != nil {
				t.Fatalf("Read modified materials failed: %v", err)
			}

			if len(readBackMaterials) != len(modifiedMaterials) {
				t.Fatalf("Material count mismatch: got %d, want %d", len(readBackMaterials), len(modifiedMaterials))
			}

			for i, mat := range readBackMaterials {
				expectedName := "modified_material_" + originalMaterials[i].Name
				if mat.Name != expectedName {
					t.Errorf("Material[%d].Name not modified: got %q, want %q", i, mat.Name, expectedName)
				}
			}

			model, err := s.ReadModelFile(outputPath)
			if err != nil {
				t.Fatalf("ReadModelFile on modified file failed: %v", err)
			}
			if !reflect.DeepEqual(model.Materials, readBackMaterials) {
				t.Errorf("Full model materials don't match ReadModelMaterial result")
			}
		})
	}
}
