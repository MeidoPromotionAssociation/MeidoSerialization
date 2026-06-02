package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	KCESService "github.com/MeidoPromotionAssociation/MeidoSerialization/service/KCES"
)

var listCtCmd = &cobra.Command{
	Use:   "listCt [file/directory]",
	Short: "List files inside a .ct archive",
	Long: `List all files inside a .ct (VirtualDirectory) archive.
When given a directory, processes all .ct files recursively.

Examples:
  MeidoSerialization listCt example.ct
  MeidoSerialization listCt ./ct_directory`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]
		if isDirectory(path) {
			return processDirectory(path, func(p string) error {
				fmt.Printf("=== %s ===\n", p)
				return listCtFile(p)
			}, isCtFile)
		}
		return listCtFile(path)
	},
}

var unpackCtCmd = &cobra.Command{
	Use:   "unpackCt [file/directory]",
	Short: "Unpack a .ct archive to a directory",
	Long: `Unpack a .ct (VirtualDirectory) archive to a directory.
When given a directory, processes all .ct files recursively.

Examples:
  MeidoSerialization unpackCt example.ct
  MeidoSerialization unpackCt example.ct -o ./output_dir
  MeidoSerialization unpackCt ./ct_directory`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]
		if isDirectory(path) {
			return processDirectoryConcurrent(path, func(p string) error {
				service := &KCESService.CtService{}
				outDir := p + "_unpacked"
				if err := service.UnpackCt(p, outDir); err != nil {
					return err
				}
				fmt.Printf("Unpacked %s to %s\n", p, outDir)
				return nil
			}, isCtFile)
		}
		service := &KCESService.CtService{}
		outDir := outputPathFlag
		if err := service.UnpackCt(path, outDir); err != nil {
			return err
		}
		if outDir == "" {
			outDir = path + "_unpacked"
		}
		fmt.Printf("Unpacked %s to %s\n", path, outDir)
		return nil
	},
}

var packCtCmd = &cobra.Command{
	Use:   "packCt [directory]",
	Short: "Pack a directory into a .ct archive",
	Long: `Pack a directory into a .ct (VirtualDirectory) archive.

Examples:
  MeidoSerialization packCt ./my_folder
  MeidoSerialization packCt ./my_folder -o custom.ct`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		service := &KCESService.CtService{}
		outPath := outputPathFlag
		if err := service.PackCt(args[0], outPath); err != nil {
			return err
		}
		if outPath == "" {
			outPath = args[0] + ".ct"
		}
		fmt.Printf("Packed %s to %s\n", args[0], outPath)
		return nil
	},
}

func listCtFile(path string) error {
	service := &KCESService.CtService{}
	files, err := service.ListCt(path)
	if err != nil {
		return err
	}
	for _, f := range files {
		fmt.Println(f)
	}
	fmt.Printf("\nTotal: %d files\n", len(files))
	return nil
}

func init() {
	unpackCtCmd.Flags().StringVarP(&outputPathFlag, "output", "o", "", "Output directory path")
	packCtCmd.Flags().StringVarP(&outputPathFlag, "output", "o", "", "Output file path")
}
