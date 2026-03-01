package workflow

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/Bibi40k/talos-docker-bootstrap/internal/bootstrap"
	"github.com/Bibi40k/talos-docker-bootstrap/internal/config"
)

func TestLoadBootstrapResultYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bootstrap.yaml")
	content := []byte(`
vm_name: devvm-01
ip: 192.168.1.50
ssh_user: dev
ssh_key_path: /home/dev/.ssh/id_ed25519
ssh_port: 22
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	got, err := LoadBootstrapResult(path)
	if err != nil {
		t.Fatalf("load bootstrap result: %v", err)
	}
	if got.VMName != "devvm-01" {
		t.Fatalf("unexpected vm_name: %s", got.VMName)
	}
	if got.IPAddress != "192.168.1.50" {
		t.Fatalf("unexpected ip: %s", got.IPAddress)
	}
}

func TestLoadBootstrapResultInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bootstrap.yaml")
	content := []byte(`vm_name: only-name`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	if _, err := LoadBootstrapResult(path); err == nil {
		t.Fatalf("expected error for invalid bootstrap result")
	}
}

func TestProvisionAndBootstrapPassesMergedConfig(t *testing.T) {
	base := mustValidStage2Config(t)
	bootstrapResult := BootstrapResult{
		VMName:        "vm-x",
		IPAddress:     "10.0.0.2",
		SSHUser:       "alice",
		SSHPrivateKey: "/tmp/key",
		SSHPort:       2222,
	}

	orig := bootstrapRunFn
	t.Cleanup(func() { bootstrapRunFn = orig })

	bootstrapRunFn = func(_ context.Context, _ *slog.Logger, cfg config.Config, opts bootstrap.Options) (bootstrap.Result, error) {
		if cfg.VM.Host != "10.0.0.2" || cfg.VM.User != "alice" || cfg.VM.Port != 2222 {
			t.Fatalf("merged config not propagated: %#v", cfg.VM)
		}
		if !opts.DryRun {
			t.Fatalf("expected dry-run passthrough")
		}
		return bootstrap.Result{Status: "planned"}, nil
	}

	res, err := ProvisionAndBootstrap(context.Background(), slog.Default(), base, bootstrapResult, ProvisionAndBootstrapOptions{DryRun: true})
	if err != nil {
		t.Fatalf("ProvisionAndBootstrap failed: %v", err)
	}
	if res.Status != "planned" {
		t.Fatalf("unexpected result: %#v", res)
	}
}

func TestProvisionAndBootstrapPropagatesBootstrapError(t *testing.T) {
	base := mustValidStage2Config(t)
	bootstrapResult := BootstrapResult{
		VMName:        "vm-x",
		IPAddress:     "10.0.0.2",
		SSHUser:       "alice",
		SSHPrivateKey: "/tmp/key",
		SSHPort:       2222,
	}

	orig := bootstrapRunFn
	t.Cleanup(func() { bootstrapRunFn = orig })
	bootstrapRunFn = func(_ context.Context, _ *slog.Logger, _ config.Config, _ bootstrap.Options) (bootstrap.Result, error) {
		return bootstrap.Result{}, errors.New("boom")
	}

	if _, err := ProvisionAndBootstrap(context.Background(), slog.Default(), base, bootstrapResult, ProvisionAndBootstrapOptions{}); err == nil {
		t.Fatalf("expected error")
	}
}
