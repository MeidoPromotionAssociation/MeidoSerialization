package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/application"
	KCESService "github.com/MeidoPromotionAssociation/MeidoSerialization/service/KCES"
	"github.com/spf13/cobra"
)

var gltfOutputFormat string

var convert2gltfCmd = &cobra.Command{
	Use:   "convert2gltf [file/directory]",
	Short: "Export native KCES Mesh and AnimationClip files to glTF",
	Long: `Export standalone KCES Mesh or AnimationClip primary files to glTF 2.0.
The default output is binary .glb; use --format gltf for JSON glTF with an embedded data URI.
Animation export supports explicit Rotation, Position, Scale, and Euler curves with Transform paths.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]
		processor := func(filePath string) error {
			return convertNativeUnityToGLTF(filePath, gltfOutputFormat)
		}
		if isDirectory(path) {
			fmt.Printf("Processing directory: %s\n", path)
			return processDirectoryConcurrent(path, processor, func(candidate string) bool {
				return fileTypeFilter(candidate) && (KCESService.IsKCESNativeMeshFile(candidate) || KCESService.IsKCESNativeAnimationClipFile(candidate))
			})
		}
		return processFile(path, processor)
	},
}

// convertNativeUnityToGLTF 将 Mesh 或 AnimationClip 主文件导出为 glTF
// convertNativeUnityToGLTF exports a Mesh or AnimationClip primary file to glTF
func convertNativeUnityToGLTF(path string, format string) error {
	format = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(format), "."))
	if format != "glb" && format != "gltf" {
		return fmt.Errorf("unsupported glTF output format %q; use glb or gltf", format)
	}
	outputPath := strings.TrimSuffix(path, filepath.Ext(path)) + "." + format
	service := &KCESService.NativeUnityMediaService{}
	var err error
	switch {
	case KCESService.IsKCESNativeMeshFile(path):
		err = service.ConvertMeshToGLTF(context.Background(), path, outputPath, format, application.DefaultMaxOutputBytes)
	case KCESService.IsKCESNativeAnimationClipFile(path):
		err = service.ConvertAnimationClipToGLTF(context.Background(), path, outputPath, format, application.DefaultMaxOutputBytes)
	default:
		return fmt.Errorf("not a native KCES Mesh or AnimationClip file: %s", path)
	}
	if err != nil {
		return err
	}
	fmt.Printf("Converted %s to %s\n", path, outputPath)
	return nil
}

// init 注册 glTF 输出格式参数
// init registers the glTF output format flag
func init() {
	convert2gltfCmd.Flags().StringVarP(&gltfOutputFormat, "format", "f", "glb", "Output format: glb or gltf")
}
