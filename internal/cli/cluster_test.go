package cli

import (
	"errors"
	"testing"

	"github.com/Bibi40k/talos-vm-bootstrap/internal/config"
)

func TestExplainClusterOpError(t *testing.T) {
	cfg := config.Config{
		VM: config.VMConfig{Host: "1.2.3.4", Port: 22, User: "dev"},
	}

	err := explainClusterOpError(errors.New("No Talos-in-Docker cluster found on remote VM."), cfg)
	ue, ok := err.(*userError)
	if !ok || ue.Hint() == "" {
		t.Fatalf("expected userError with hint for missing cluster")
	}

	err = explainClusterOpError(errors.New("ssh run command failed: exit status 255"), cfg)
	ue, ok = err.(*userError)
	if !ok || ue.Hint() == "" {
		t.Fatalf("expected userError with hint for ssh failure")
	}
}
