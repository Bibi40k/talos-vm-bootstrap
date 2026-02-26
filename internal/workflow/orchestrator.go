package workflow

import (
	"context"
	"log/slog"

	"github.com/Bibi40k/talos-vm-bootstrap/internal/bootstrap"
	"github.com/Bibi40k/talos-vm-bootstrap/internal/config"
	vmconfig "github.com/Bibi40k/vmware-vm-bootstrap/pkg/config"
)

// ProvisionAndBootstrapOptions controls orchestrated execution behavior.
type ProvisionAndBootstrapOptions struct {
	DryRun        bool
	HumanProgress bool
}

var bootstrapRunFn = bootstrap.Run

// LoadBootstrapResult reads a BootstrapResult from YAML or JSON.
func LoadBootstrapResult(path string) (BootstrapResult, error) {
	return vmconfig.LoadBootstrapResult(path)
}

// ProvisionAndBootstrap executes the integrated workflow:
// Bootstrap contract -> merge into Talos config -> Talos bootstrap run.
func ProvisionAndBootstrap(
	ctx context.Context,
	logger *slog.Logger,
	stage2Base config.Config,
	bootstrapResult BootstrapResult,
	opts ProvisionAndBootstrapOptions,
) (bootstrap.Result, error) {
	merged, err := MergeBootstrapIntoStage2(stage2Base, bootstrapResult)
	if err != nil {
		return bootstrap.Result{}, err
	}

	logger.Info("workflow merged bootstrap result into stage2",
		"vm_name", bootstrapResult.VMName,
		"ip", bootstrapResult.IPAddress,
		"ssh_user", bootstrapResult.SSHUser,
	)

	return bootstrapRunFn(ctx, logger, merged, bootstrap.Options{
		DryRun:        opts.DryRun,
		HumanProgress: opts.HumanProgress,
	})
}
