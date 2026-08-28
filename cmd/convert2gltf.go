package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/application"
	KCESService "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/service/KCES"
	"github.com/spf13/cobra"
)

var gltfOutputFormat string

// meshOnlyGLTFNoticeLines 说明直接导出 .mmesh 只得到几何体，完整模型应转换引用它的 .model
// meshOnlyGLTFNoticeLines explains that exporting a .mmesh directly yields geometry alone and that a complete model comes from the .model referencing it
var meshOnlyGLTFNoticeLines = []string{
	"\nDirectly converting a .mmesh file will result in missing information such as the armature. To obtain complete data, you should use `convert2gltf` on a .model file. The program will automatically search for the corresponding .mmesh file in the same folder and the Mesh folder within its parent folder",
	"\n直接对 .mmesh 使用转换会缺少骨骼等信息，若要得到完整信息，你应该对 .model 使用 convert2gltf，程序会自动在同文件夹以及上级文件夹中的 Mesh 文件夹中寻找对应的 .mmesh\n",
}

var convert2gltfCmd = &cobra.Command{
	Use:   "convert2gltf [file/directory]",
	Short: "Export KCES Model and native Mesh or AnimationClip files to glTF",
	Long: `Export KCES .model files or standalone Mesh and AnimationClip primary files to glTF 2.0.
A .model input also loads the .mmesh referenced by meshFileName and produces a complete skinned
glTF with the skeleton, bone weights, morph targets, material names, and KCES extras for gltf2model.
Convert the .model rather than the .mmesh it references: a .mmesh stores geometry alone, so exporting
one directly gives a glTF without the skeleton, bone weights, morph targets, material names, or
UV1-UV7, and gltf2model can only reimport that file with a synthesized single-bone skin.
The default output is binary .glb; use --format gltf for JSON glTF with an embedded data URI.
Animation export supports explicit Rotation, Position, Scale, and Euler curves with Transform paths.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]
		var meshOnlyCount atomic.Int64
		processor := func(filePath string) error {
			meshOnly, err := convertNativeUnityToGLTF(filePath, gltfOutputFormat)
			if meshOnly {
				meshOnlyCount.Add(1)
			}
			return err
		}
		var err error
		if isDirectory(path) {
			fmt.Printf("Processing directory: %s\n", path)
			err = processDirectoryConcurrent(path, processor, func(candidate string) bool {
				return fileTypeFilter(candidate) && (KCESService.IsKCESModelFile(candidate) || KCESService.IsKCESNativeMeshFile(candidate) || KCESService.IsKCESNativeAnimationClipFile(candidate))
			})
		} else {
			err = processFile(path, processor)
		}
		printMeshOnlyGLTFNotice(meshOnlyCount.Load())
		return err
	},
}

// convertNativeUnityToGLTF 将 Model、Mesh 或 AnimationClip 主文件导出为 glTF，并报告本次是否导出了缺少骨架的独立 Mesh
// convertNativeUnityToGLTF exports a Model, Mesh, or AnimationClip primary file to glTF and reports whether it exported a standalone Mesh that lacks a skeleton
func convertNativeUnityToGLTF(path string, format string) (bool, error) {
	format = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(format), "."))
	if format != "glb" && format != "gltf" {
		return false, fmt.Errorf("unsupported glTF output format %q; use glb or gltf", format)
	}
	outputPath := strings.TrimSuffix(path, filepath.Ext(path)) + "." + format
	service := &KCESService.NativeUnityMediaService{}
	var err error
	meshOnly := false
	switch {
	case KCESService.IsKCESModelFile(path):
		err = (&KCESService.ModelService{}).ConvertModelToGLTF(context.Background(), path, outputPath, format, application.DefaultMaxOutputBytes)
	case KCESService.IsKCESNativeMeshFile(path):
		meshOnly = true
		err = service.ConvertMeshToGLTF(context.Background(), path, outputPath, format, application.DefaultMaxOutputBytes)
	case KCESService.IsKCESNativeAnimationClipFile(path):
		err = service.ConvertAnimationClipToGLTF(context.Background(), path, outputPath, format, application.DefaultMaxOutputBytes)
	default:
		return false, fmt.Errorf("not a KCES Model or native Mesh or AnimationClip file: %s", path)
	}
	if err != nil {
		return false, err
	}
	fmt.Printf("Converted %s to %s\n", path, outputPath)
	return meshOnly, nil
}

// printMeshOnlyGLTFNotice 在导出过独立 Mesh 后打印一次改用 .model 的提示，count 为零时不打印
// printMeshOnlyGLTFNotice prints the guidance to use the .model input once after standalone Mesh exports and stays silent when count is zero
func printMeshOnlyGLTFNotice(count int64) {
	if count <= 0 {
		return
	}
	fmt.Printf("Note: exported %d standalone Mesh file(s) as geometry only.\n", count)
	for _, line := range meshOnlyGLTFNoticeLines {
		fmt.Println(line)
	}
}

// init 注册 glTF 输出格式参数
// init registers the glTF output format flag
func init() {
	convert2gltfCmd.Flags().StringVarP(&gltfOutputFormat, "format", "f", "glb", "Output format: glb or gltf")
}
