package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	KCESService "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/service/KCES"
)

var listAbaCmd = &cobra.Command{
	Use:   "listAba [file/directory]",
	Short: "List assets inside a .aba file",
	Long: `List all assets inside a .aba (Unity AssetBundle) file.
When given a directory, processes all .aba files recursively.

Examples:
  MeidoSerialization listAba example.aba
  MeidoSerialization listAba ./aba_directory`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]
		if isDirectory(path) {
			return processDirectory(path, func(p string) error {
				fmt.Printf("=== %s ===\n", p)
				return listAbaFile(p)
			}, isAbaFile)
		}
		return listAbaFile(path)
	},
}

var unpackAbaCmd = &cobra.Command{
	Use:   "unpackAba [file/directory]",
	Short: "Unpack a .aba file to a directory",
	Long: `Unpack a .aba (Unity AssetBundle) file to a directory.
Assets are organized by type in subdirectories.
When given a directory, processes all .aba files recursively.

Examples:
  MeidoSerialization unpackAba example.aba
  MeidoSerialization unpackAba example.aba -o ./output_dir
  MeidoSerialization unpackAba ./aba_directory`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]
		if isDirectory(path) {
			return processDirectoryConcurrent(path, func(p string) error {
				service := &KCESService.AbaService{}
				outDir := p + "_unpacked"
				if err := service.UnpackAba(p, outDir); err != nil {
					return err
				}
				fmt.Printf("Unpacked %s to %s\n", p, outDir)
				return nil
			}, isAbaFile)
		}
		service := &KCESService.AbaService{}
		outDir := outputPathFlag
		if err := service.UnpackAba(path, outDir); err != nil {
			return err
		}
		if outDir == "" {
			outDir = path + "_unpacked"
		}
		fmt.Printf("Unpacked %s to %s\n", path, outDir)
		return nil
	},
}

var packAbaCmd = &cobra.Command{
	Use:   "packAba [directory]",
	Short: "Pack a directory into .aba and .ct files",
	Long: `Pack a directory into .aba (Unity AssetBundle) and .ct (VirtualDirectory) files.
Files are classified by extension:
  - Unity assets (.tex, .mesh, .png, etc.) → .aba
  - Game data (.menuassets, .materialassets, catalog, etc.) → .ct

KCES 1.34.5 reads parts containers only from files named exactly <aba name>.menuassets and
<aba name>.materialassets and compares those names case-sensitively, so the output name must be
all lowercase and must match the container names inside the package. A warning is printed for a
mismatch and for .materialassets entries whose fileName does not end in a lowercase .mate, since
the game can never look those materials up.

Examples:
  MeidoSerialization packAba ./my_folder
  MeidoSerialization packAba ./my_folder -o output_name`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		service := &KCESService.PackService{}
		if err := service.PackToAbaAndCt(args[0], outputPathFlag); err != nil {
			return err
		}
		fmt.Printf("Packed %s\n", args[0])
		return nil
	},
}

// listAbaFile 读取 ABA 资源条目并将其摘要打印到标准输出
// listAbaFile reads ABA asset entries and prints their summary to standard output
func listAbaFile(path string) error {
	service := &KCESService.AbaService{}
	entries, err := service.ListAba(path)
	if err != nil {
		return err
	}
	for _, e := range entries {
		fmt.Printf("PathId=%-22d Type=%-15s Size=%-10d Name=%q\n",
			e.PathId, e.TypeName, e.Size, e.Name)
	}
	fmt.Printf("\nTotal: %d assets\n", len(entries))
	return nil
}

// init 注册 ABA 解包目录和打包基础名称参数
// init registers the ABA unpack directory and pack base-name flags
func init() {
	unpackAbaCmd.Flags().StringVarP(&outputPathFlag, "output", "o", "", "Output directory path")
	packAbaCmd.Flags().StringVarP(&outputPathFlag, "output", "o", "", "Output base name")
}
