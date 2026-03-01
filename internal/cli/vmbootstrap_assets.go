package cli

import (
	"fmt"
	"os"

	vmtool "github.com/Bibi40k/talos-docker-bootstrap/internal/tooling/vmbootstrap"
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
	_, _ = fmt.Fprintln(os.Stdout, "  Run: make vmbootstrap-sync-assets")
	_, _ = fmt.Fprintln(os.Stdout, "  Use FORCE=1 only when you intentionally want full overwrite.")
	_, _ = fmt.Fprintln(os.Stdout)
}
