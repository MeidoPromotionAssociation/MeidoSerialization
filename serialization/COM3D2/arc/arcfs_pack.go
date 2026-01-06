package arc

import (
	"fmt"
	"os"
	"path/filepath"
)

// Pack loads all files from dirPath into an Arc structure and dumps it to arcPath
func Pack(dirPath string, arcPath string) error {
	absDir, err := filepath.Abs(dirPath)
	if err != nil {
		return fmt.Errorf("failed to getting absolute path for %q: %w", dirPath, err)
	}

	// Create a new Arc
	// Use the directory name as the Arc name
	name := filepath.Base(absDir)
	fs := NewArc(name)

	err = filepath.Walk(absDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walking %q: %w", path, err)
		}
		if info.IsDir() {
			return nil
		}

		// Calculate relative path for use within the ARC
		rel, err := filepath.Rel(absDir, path)
		if err != nil {
			return fmt.Errorf("failed to calculating relative path for %q: %w", path, err)
		}

		// Read file data
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to reading file %q: %w", path, err)
		}

		// Add to Arc
		f := AddFileByPath(fs.Root, rel)
		f.arc = fs
		f.Ptr = NewMemoryPointer(data)

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk directory %q: %w", absDir, err)
	}

	// Dump to arcPath
	return fs.Dump(arcPath)
}
