package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/internal/schemagen"
)

func main() {
	output := flag.String("out", "schemas/editing/v1", "output directory")
	flag.Parse()
	if err := run(*output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(output string) error {
	documents, err := schemagen.GenerateAll()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(output, 0755); err != nil {
		return fmt.Errorf("create schema directory: %w", err)
	}
	ids := make([]string, 0, len(documents))
	for id := range documents {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		path := filepath.Join(output, id+".schema.json")
		if err := os.WriteFile(path, documents[id].JSON, 0644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}
