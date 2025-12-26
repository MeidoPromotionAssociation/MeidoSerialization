package arc

import (
	"os"
	"path/filepath"
)

// Pack loads all files from dirPath into an Arc structure and dumps it to arcPath
func Pack(dirPath string, arcPath string) error {
	absDir, err := filepath.Abs(dirPath)
	if err != nil {
		return err
	}

	// Create a new Arc
	// Use the directory name as the Arc name
	name := filepath.Base(absDir)
	fs := New(name)

	err = filepath.Walk(absDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Calculate relative path for use within the ARC
		rel, err := filepath.Rel(absDir, path)
		if err != nil {
			return err
		}

		// Read file data
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		// Add to Arc
		f := AddFileByPath(fs.Root, rel)
		f.fs = fs
		f.Ptr = NewMemoryPointer(data)

		return nil
	})

	if err != nil {
		return err
	}

	// Dump to arcPath
	return fs.Dump(arcPath)
}
