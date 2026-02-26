package cli

import (
	"fmt"
	"os"

	vmtool "github.com/Bibi40k/talos-vm-bootstrap/internal/tooling/vmbootstrap"
)

func warnPinnedAssetDrift() {
	drift, err := vmtool.CheckPinnedAssets(".")
	if err != nil || len(drift) == 0 {
		return
	}
	_, _ = fmt.Fprintln(os.Stdout, "\033[33mvmbootstrap assets out of sync:\033[0m")
	for _, item := range drift {
		_, _ = fmt.Fprintf(os.Stdout, "  - %s\n", item)
	}
	_, _ = fmt.Fprintln(os.Stdout, "  Run: make vmbootstrap-sync-assets FORCE=1")
	_, _ = fmt.Fprintln(os.Stdout)
}
