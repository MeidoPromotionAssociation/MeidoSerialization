package cmd

import (
	"fmt"
	"math"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/application"
)

// configuredRoots 根据只读根目录参数创建受限根目录集合
// configuredRoots creates a confined root set from read-only root specifications
func configuredRoots(specs []string) (*application.RootSet, error) {
	return configuredRootsWithWrites(specs, nil)
}

// configuredRootsWithWrites 校验只读和可写根目录参数并创建对应的受限根目录集合
// configuredRootsWithWrites validates read-only and writable root specifications and creates the corresponding confined root set
func configuredRootsWithWrites(readSpecs, writeSpecs []string) (*application.RootSet, error) {
	roots := application.NewRootSet()
	seen := make(map[string]string, len(readSpecs)+len(writeSpecs))
	for _, spec := range readSpecs {
		id, _, ok := parseRootSpec(spec, "--root")
		if !ok || strings.TrimSpace(id) == "" {
			_ = roots.Close()
			return nil, fmt.Errorf("invalid --root %q; expected id=directory", spec)
		}
		id = strings.TrimSpace(id)
		if previous, exists := seen[id]; exists {
			_ = roots.Close()
			return nil, fmt.Errorf("duplicate root ID %q in %s and --root", id, previous)
		}
		seen[id] = "--root"
	}
	for _, spec := range writeSpecs {
		id, directory, ok := parseRootSpec(spec, "--write-root")
		if !ok {
			_ = roots.Close()
			return nil, fmt.Errorf("invalid --write-root %q; expected id=directory", spec)
		}
		id = strings.TrimSpace(id)
		if previous, exists := seen[id]; exists {
			_ = roots.Close()
			return nil, fmt.Errorf("duplicate root ID %q in %s and --write-root", id, previous)
		}
		seen[id] = "--write-root"
		if err := roots.AddWritable(id, strings.TrimSpace(directory)); err != nil {
			_ = roots.Close()
			return nil, err
		}
	}
	for _, spec := range readSpecs {
		id, directory, ok := parseRootSpec(spec, "--root")
		if !ok || strings.TrimSpace(id) == "" || strings.TrimSpace(directory) == "" {
			_ = roots.Close()
			return nil, fmt.Errorf("invalid --root %q; expected id=directory", spec)
		}
		err := roots.Add(strings.TrimSpace(id), strings.TrimSpace(directory))
		if err != nil {
			_ = roots.Close()
			return nil, err
		}
	}
	return roots, nil
}

// parseRootSpec 将 id=directory 形式的根目录参数拆分并规范化
// parseRootSpec splits and normalizes a root specification in id=directory form
func parseRootSpec(spec, _ string) (id, directory string, ok bool) {
	id, directory, ok = strings.Cut(spec, "=")
	id = strings.TrimSpace(id)
	directory = strings.TrimSpace(directory)
	return id, directory, ok && id != "" && directory != ""
}

// mebibytes 将正数 MiB 配置安全转换为精确字节数
// mebibytes safely converts a positive MiB setting to an exact byte count
func mebibytes(value int64) (int64, error) {
	if value <= 0 || value > math.MaxInt64/(1<<20) {
		return 0, fmt.Errorf("MiB value %d is outside the supported range", value)
	}
	return value << 20, nil
}

// applicationVersion 返回构建时注入的应用版本号
// applicationVersion returns the application version injected at build time
func applicationVersion() string {
	if buildVersion == "" || buildVersion == "(devel)" {
		return "dev"
	}
	return buildVersion
}
