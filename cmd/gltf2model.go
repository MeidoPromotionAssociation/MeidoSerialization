package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/application"
	KCESService "github.com/MeidoPromotionAssociation/MeidoSerialization/service/KCES"
	"github.com/spf13/cobra"
)

var gltf2modelOutputDir string

var gltf2modelCmd = &cobra.Command{
	Use:   "gltf2model [file/directory]",
	Short: "Convert glTF or GLB files to KCES .model and .mmesh",
	Long: `Convert glTF 2.0 or GLB files into KCES .model and .mmesh files.
The scene needs exactly one triangle mesh node; its skeleton, skin, morph targets, and material
names become the Model data, and the geometry becomes an official Unity 2022.3 native Mesh.
An unskinned scene is bound rigidly to the mesh node with a synthesized single-bone skin.
Material appearance is not converted: glTF PBR parameters and textures are ignored, and each
material's name must match the KCES material the game should load for that sub-mesh.
Material names from the kcesModel extras take precedence when present; a file re-exported by
Blender normally drops those extras, so the Blender material slot names take effect instead.
UV layers map by order: TEXCOORD_0 through TEXCOORD_7 become Unity UV0 through UV7, and the
main texture always samples UV0.
KCES fields saved by convert2gltf in the kcesModel extras are restored losslessly.
Output files are written next to the input by default; use --output to select a directory.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]
		processor := func(filePath string) error {
			return convertGLTFToKCESModel(filePath, gltf2modelOutputDir)
		}
		if isDirectory(path) {
			fmt.Printf("Processing directory: %s\n", path)
			return processDirectoryConcurrent(path, processor, func(candidate string) bool {
				return fileTypeFilter(candidate) && KCESService.IsKCESGLTFFile(candidate)
			})
		}
		return processFile(path, processor)
	},
}

// convertGLTFToKCESModel 将一个 glTF 或 GLB 文件转换为 .model 与 .mmesh
// convertGLTFToKCESModel converts one glTF or GLB file into .model and .mmesh files
func convertGLTFToKCESModel(path string, outputDir string) error {
	if !KCESService.IsKCESGLTFFile(path) {
		return fmt.Errorf("not a glTF or GLB file: %s", path)
	}
	if outputDir == "" {
		outputDir = filepath.Dir(path)
	}
	if err := (&KCESService.ModelService{}).ConvertGLTFToModel(context.Background(), path, outputDir, application.DefaultMaxOutputBytes); err != nil {
		return err
	}
	fmt.Printf("Converted %s to .model and .mmesh in %s\n", path, outputDir)
	return nil
}

// init 注册 glTF 转模型的输出目录参数
// init registers the output directory flag for glTF-to-model conversion
func init() {
	gltf2modelCmd.Flags().StringVarP(&gltf2modelOutputDir, "output", "o", "", "Output directory (defaults to the input directory)")
}
