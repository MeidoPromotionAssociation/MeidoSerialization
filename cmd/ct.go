package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

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

var genCtCmd = &cobra.Command{
	Use:   "genCt [file/directory]",
	Short: "Generate a .ct catalog from a .aba file",
	Long: `Generate the companion .ct (catalog) file from a .aba (Unity AssetBundle) file.
Catalog entries are collected from the AssetBundle container of the .aba, and
metadata uses the same defaults as packAba (CatalogType Parts, PackageType Plugin,
priority 0). Use the convert command on the generated .ct for further metadata
editing through its .ct.json envelope.
When given a directory, processes all .aba files recursively.

Examples:
  MeidoSerialization genCt my_mod.aba
  MeidoSerialization genCt my_mod.aba -o custom.ct
  MeidoSerialization genCt ./aba_directory`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]
		if isDirectory(path) {
			return processDirectoryConcurrent(path, func(p string) error {
				return generateCtFromAbaFile(p, "")
			}, isAbaFile)
		}
		return generateCtFromAbaFile(path, outputPathFlag)
	},
}

// listCtFile 读取内容表中的文件路径并将其打印到标准输出
// listCtFile reads file paths from a content table and prints them to standard output
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

// generateCtFromAbaFile 为单个 .aba 生成配套 .ct 并打印实际输出路径
// generateCtFromAbaFile generates the companion .ct for one .aba file and prints the actual output path
func generateCtFromAbaFile(abaPath string, outPath string) error {
	service := &KCESService.CtService{}
	if err := service.GenerateCtFromAba(abaPath, outPath); err != nil {
		return err
	}
	if outPath == "" {
		base := strings.TrimSuffix(filepath.Base(abaPath), filepath.Ext(abaPath))
		outPath = filepath.Join(filepath.Dir(abaPath), base+".ct")
	}
	fmt.Printf("Generated %s from %s\n", outPath, abaPath)
	return nil
}

// init 注册 CT 生成命令的输出文件路径参数
// init registers the output file-path flag for the CT generation command
func init() {
	genCtCmd.Flags().StringVarP(&outputPathFlag, "output", "o", "", "Output file path")
}
