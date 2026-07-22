package cmd

import (
	"fmt"
	"math"
	"runtime/debug"
	"strings"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/application"
)

func configuredRoots(specs []string) (*application.RootSet, error) {
	return configuredRootsWithWrites(specs, nil)
}

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

func parseRootSpec(spec, _ string) (id, directory string, ok bool) {
	id, directory, ok = strings.Cut(spec, "=")
	id = strings.TrimSpace(id)
	directory = strings.TrimSpace(directory)
	return id, directory, ok && id != "" && directory != ""
}

func mebibytes(value int64) (int64, error) {
	if value <= 0 || value > math.MaxInt64/(1<<20) {
		return 0, fmt.Errorf("MiB value %d is outside the supported range", value)
	}
	return value << 20, nil
}

func applicationVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return "dev"
	}
	return info.Main.Version
}
