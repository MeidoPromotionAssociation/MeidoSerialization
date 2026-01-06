package arc

import (
	"fmt"
	"os"
	"path/filepath"
)

// Unpack extracts the entire Arc file system to the specified directory.
func (arc *Arc) Unpack(outDir string) error {
	for _, f := range AllFiles(arc) {
		relPath := f.RelativePath()
		targetPath := filepath.Join(outDir, relPath)
		if err := f.Extract(targetPath); err != nil {
			return fmt.Errorf("failed to extract %s: %w", relPath, err)
		}
	}
	return nil
}

// Extract saves the file to the specified path.
func (f *File) Extract(outPath string) error {
	data, err := f.Ptr.Data()
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", f.RelativePath(), err)
	}

	if f.Ptr.Compressed() {
		data, err = deflateDecompress(data)
		if err != nil {
			return fmt.Errorf("failed to decompress %s: %w", f.RelativePath(), err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", outPath, err)
	}

	return os.WriteFile(outPath, data, 0644)
}
