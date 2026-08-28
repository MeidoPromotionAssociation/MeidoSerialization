package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/application"
	KCESService "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/service/KCES"
	"github.com/spf13/cobra"
)

var convert2audioCmd = &cobra.Command{
	Use:   "convert2audio [file/directory]",
	Short: "Extract encoded audio from native KCES AudioClip files",
	Long: `Extract the inline encoded payload from standalone KCES AudioClip primary files.
OGG, WAV, and FSB5 signatures receive their corresponding extension without transcoding.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]
		if isDirectory(path) {
			fmt.Printf("Processing directory: %s\n", path)
			return processDirectoryConcurrent(path, convertNativeAudioClip, func(candidate string) bool {
				return fileTypeFilter(candidate) && KCESService.IsKCESNativeAudioClipFile(candidate)
			})
		}
		return processFile(path, convertNativeAudioClip)
	},
}

// convertNativeAudioClip 无损提取 AudioClip 内联编码载荷
// convertNativeAudioClip losslessly extracts an AudioClip's inline encoded payload
func convertNativeAudioClip(path string) error {
	service := &KCESService.NativeUnityMediaService{}
	extension, err := service.DetectAudioClipExtension(context.Background(), path)
	if err != nil {
		return err
	}
	outputPath := strings.TrimSuffix(path, filepath.Ext(path)) + extension
	if err := service.ExtractAudioClip(context.Background(), path, outputPath, application.DefaultMaxOutputBytes); err != nil {
		return err
	}
	fmt.Printf("Converted %s to %s\n", path, outputPath)
	return nil
}
