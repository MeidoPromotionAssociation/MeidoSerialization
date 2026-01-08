package main

import (
	"fmt"
	"os"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		_, err := fmt.Fprintln(os.Stderr, err)
		if err != nil {
			return
		}
		os.Exit(1)
	}
}
