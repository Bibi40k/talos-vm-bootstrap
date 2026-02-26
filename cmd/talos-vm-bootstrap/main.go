package main

import (
	"os"

	"github.com/Bibi40k/talos-vm-bootstrap/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
