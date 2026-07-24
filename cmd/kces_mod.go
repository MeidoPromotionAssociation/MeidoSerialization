package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	KCESService "github.com/MeidoPromotionAssociation/MeidoSerialization/service/KCES"
)

var inspectKcesCatalogCmd = &cobra.Command{
	Use:   "inspectKcesCatalog [file]",
	Short: "Inspect AssetBundleCatalog inside a .ct file",
	Long: `Inspect the AssetBundleCatalog and ExtensionNameList contents of a KCES .ct file.

Examples:
  MeidoSerialization inspectKcesCatalog example.ct`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return inspectKcesCatalog(args[0])
	},
}

var packKcesModCmd = &cobra.Command{
	Use:   "packKcesMod [manifest.json]",
	Short: "Pack a KCES MOD (.ct + .aba) from a manifest",
	Long: `Pack a KCES MOD into .ct (catalog) + .aba (Unity AssetBundle) files based on a manifest.json.

The manifest must include name, catalogType, packageType, priority, and an assets list.
Each asset references a TextAsset payload file (e.g. .menuassets, .materialassets, .pmatassets, .model).

Examples:
  MeidoSerialization packKcesMod manifest.json
  MeidoSerialization packKcesMod manifest.json -o ./out`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		service := &KCESService.ModPackService{}
		if err := service.PackMod(args[0], outputPathFlag); err != nil {
			return err
		}
		fmt.Printf("Packed KCES MOD from %s\n", args[0])
		return nil
	},
}

func inspectKcesCatalog(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open .ct: %w", err)
	}
	defer f.Close()

	table, err := ct.ReadContentTable(f)
	if err != nil {
		return fmt.Errorf("parse .ct: %w", err)
	}

	cat, err := ct.DecodeCatalogFromCt(table)
	if err != nil {
		return fmt.Errorf("decode catalog: %w", err)
	}

	fmt.Printf("=== Catalog ===\n")
	fmt.Printf("  Version:           %d\n", cat.Version)
	fmt.Printf("  Name:              %q\n", formatNullableStringForCLI(cat.Name))
	fmt.Printf("  SubName:           %q\n", formatNullableStringForCLI(cat.SubName))
	fmt.Printf("  CatalogType:       %d\n", cat.CatalogType)
	fmt.Printf("  PackageType:       %d\n", cat.PackageType)
	fmt.Printf("  Priority:          %d\n", cat.Priority)
	fmt.Printf("  Hash:              %d\n", cat.Hash)
	fmt.Printf("  CreateTime:        %d\n", cat.CreateTime)
	fmt.Printf("  IsEncrypted:       %v\n", cat.IsEncrypted)
	fmt.Printf("  ResourceFileNames: %v\n", cat.ResourceFileNames)
	fmt.Printf("  ExtensionList:     %v\n", cat.ExtensionList)

	fmt.Printf("\n=== Items (%d) ===\n", len(cat.Items))
	for i, item := range cat.Items {
		if item == nil {
			fmt.Printf("  [%d] null\n", i)
			continue
		}
		fmt.Printf("  [%d] resourceIndex=%d name=%q hash=%d\n", i, item.ResourceIndex, formatNullableStringForCLI(item.Name), item.Hash)
	}

	for index, extension := range cat.ExtensionList {
		if extension == nil {
			fmt.Printf("\n=== ExtensionNameList[%d] null ===\n", index)
			continue
		}
		ext := *extension
		enl, err := ct.DecodeExtensionNameListFromCt(table, ext)
		if err != nil {
			fmt.Printf("\n=== ExtensionNameList %q (decode failed: %v) ===\n", ext, err)
			continue
		}
		if enl == nil {
			fmt.Printf("\n=== ExtensionNameList %q null ===\n", ext)
			continue
		}
		fmt.Printf("\n=== ExtensionNameList %q (%d entries) ===\n", ext, len(enl.Data))
		for i, p := range enl.Data {
			if p == nil {
				fmt.Printf("  [%d] null\n", i)
				continue
			}
			fmt.Printf("  [%d] name=%q hash=%d\n", i, formatNullableStringForCLI(p.Name), p.Hash)
		}
	}

	return nil
}

// formatNullableStringForCLI 将可空字符串转换为 CLI 输出文本，nil 显示为 <nil>
// formatNullableStringForCLI converts a nullable string to CLI text and displays nil as <nil>
func formatNullableStringForCLI(value *string) string {
	if value == nil {
		return "<nil>"
	}
	return *value
}

func init() {
	packKcesModCmd.Flags().StringVarP(&outputPathFlag, "output", "o", "", "Output directory")
}
