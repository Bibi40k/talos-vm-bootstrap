package workflow

import (
	"fmt"
	"strings"

	"github.com/Bibi40k/talos-docker-bootstrap/internal/config"
	vmconfig "github.com/Bibi40k/vmware-vm-bootstrap/pkg/config"
)

// BootstrapResult reuses the canonical bootstrap contract from vmware-vm-bootstrap.
type BootstrapResult = vmconfig.BootstrapResult

// MergeBootstrapIntoStage2 applies the bootstrap contract values into a Talos config.
// Bootstrap values are authoritative for VM connection fields.
func MergeBootstrapIntoStage2(base config.Config, bootstrap BootstrapResult) (config.Config, error) {
	if err := bootstrap.Validate(); err != nil {
		return config.Config{}, err
	}

	merged := base
	merged.VM.Host = bootstrap.IPAddress
	merged.VM.User = bootstrap.SSHUser
	merged.VM.SSHPrivateKey = bootstrap.SSHPrivateKey
	if bootstrap.SSHPort > 0 {
		merged.VM.Port = bootstrap.SSHPort
	}
	if strings.TrimSpace(bootstrap.SSHHostFingerprint) != "" {
		merged.VM.SSHHostFingerprint = bootstrap.SSHHostFingerprint
	}

	if err := merged.Validate(); err != nil {
		return config.Config{}, fmt.Errorf("merged talos config invalid: %w", err)
	}
	return merged, nil
}
