package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	vmtool "github.com/Bibi40k/talos-vm-bootstrap/internal/tooling/vmbootstrap"
	"github.com/spf13/cobra"
)

func newVMDeployCmd() *cobra.Command {
	var (
		vmbootstrapBin    string
		vmbootstrapRepo   string
		vmbootstrapBuild  bool
		vmbootstrapNotify bool
		bootstrapResult   string
	)

	cmd := &cobra.Command{
		Use:   "vm-deploy",
		Short: "Create/bootstrap Ubuntu VM via vmbootstrap",
		RunE: func(_ *cobra.Command, _ []string) error {
			if vmbootstrapNotify {
				current, latest, hasUpdate, err := vmtool.IsUpdateAvailable()
				if err == nil && hasUpdate {
					fmt.Printf("\n\033[1;33m⚠ VMBOOTSTRAP UPDATE AVAILABLE: %s -> %s\033[0m\n", current, latest)
					fmt.Println("  Run: make update-vmbootstrap-pin install-vmbootstrap")
					fmt.Println()
				}
			}

			resolvedBin, err := vmtool.ResolveBinary(vmtool.ResolveOptions{
				Bin:       vmbootstrapBin,
				Repo:      vmbootstrapRepo,
				AutoBuild: vmbootstrapBuild,
			})
			if err != nil {
				return fmt.Errorf("resolve vmbootstrap binary: %w", err)
			}

			args := []string{"run"}
			if bootstrapResult != "" {
				args = append(args, "--bootstrap-result", bootstrapResult)
			}
			run := exec.Command(resolvedBin, args...)
			if workdir := deriveVMBootstrapWorkDir(resolvedBin); workdir != "" {
				run.Dir = workdir
			}
			run.Stdin = os.Stdin
			run.Stdout = os.Stdout
			var stderr bytes.Buffer
			run.Stderr = io.MultiWriter(os.Stderr, &stderr)
			if err := run.Run(); err != nil {
				if strings.Contains(stderr.String(), "unknown flag: --bootstrap-result") {
					return fmt.Errorf("vmbootstrap is too old to support --bootstrap-result (run: make install-vmbootstrap or set VMBOOTSTRAP_AUTO_BUILD=1)")
				}
				return fmt.Errorf("run vmbootstrap deployment: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&vmbootstrapBin, "vmbootstrap-bin", "bin/vmbootstrap", "vmware-vm-bootstrap CLI binary")
	cmd.Flags().StringVar(&vmbootstrapRepo, "vmbootstrap-repo", "../vmware-vm-bootstrap", "Path to vmware-vm-bootstrap repository (used only with --vmbootstrap-auto-build)")
	cmd.Flags().BoolVar(&vmbootstrapBuild, "vmbootstrap-auto-build", false, "Auto-build vmbootstrap from --vmbootstrap-repo when binary is missing")
	cmd.Flags().BoolVar(&vmbootstrapNotify, "vmbootstrap-update-notify", true, "Show update notice when a newer vmbootstrap module version is available")
	cmd.Flags().StringVar(&bootstrapResult, "bootstrap-result", "", "Write vmbootstrap result to YAML/JSON file")
	return cmd
}
