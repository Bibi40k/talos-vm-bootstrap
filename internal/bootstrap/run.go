package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/Bibi40k/talos-vm-bootstrap/internal/config"
	"github.com/Bibi40k/talos-vm-bootstrap/internal/ssh"
	"github.com/Bibi40k/talos-vm-bootstrap/pkg/model"
)

type Options struct {
	DryRun        bool
	HumanProgress bool
}

type Result = model.BootstrapResult

type Step = model.StepResult

var (
	waitForTCPPortWithStatsFn = ssh.WaitForTCPPortWithStats
	runOSHardeningFn          = runOSHardening
	runDockerInstallFn        = runDockerInstall
	runTalosctlInstallFn      = runTalosctlInstall
	runClusterCreateFn        = runClusterCreate
	knownHostsPromptFn        func(message string) (bool, error)
)

// SetKnownHostsPrompt sets a prompt handler for known_hosts mismatch confirmations.
// Returns a restore func to reset the previous handler.
func SetKnownHostsPrompt(fn func(message string) (bool, error)) func() {
	prev := knownHostsPromptFn
	knownHostsPromptFn = fn
	return func() { knownHostsPromptFn = prev }
}

func Run(ctx context.Context, logger *slog.Logger, cfg config.Config, opts Options) (Result, error) {
	res := Result{
		Status:         "running",
		StartedAt:      time.Now().UTC(),
		VMHost:         cfg.VM.Host,
		VMUser:         cfg.VM.User,
		Cluster:        cfg.Cluster.Name,
		KubeconfigPath: filepath.Join(cfg.Cluster.StateDir, "kubeconfig"),
		DryRun:         opts.DryRun,
	}

	if opts.DryRun {
		res.Steps = []Step{
			{Name: "ssh_connectivity", Status: model.StepStatusPlanned, Message: "Check SSH TCP reachability"},
			{Name: "os_hardening", Status: model.StepStatusPlanned, Message: "Apply idempotent OS hardening baseline"},
			{Name: "docker_install", Status: model.StepStatusPlanned, Message: "Install pinned Docker version"},
			{Name: "talosctl_install", Status: model.StepStatusPlanned, Message: "Install pinned talosctl and verify checksum"},
			{Name: "cluster_create", Status: model.StepStatusPlanned, Message: "Create Talos-in-Docker cluster if missing"},
		}
		res.Status = "planned"
		res.EndedAt = time.Now().UTC()
		return res, nil
	}

	steps := []struct {
		name string
		desc string
		run  func(context.Context) error
	}{
		{
			name: "ssh_connectivity",
			desc: "Check SSH TCP reachability",
			run: func(ctx context.Context) error {
				if opts.HumanProgress {
					logger.Debug("ssh connectivity probe",
						"host", cfg.VM.Host,
						"port", cfg.VM.Port,
						"connect_timeout", cfg.Timeouts.SSHConnectDuration().String(),
						"retries", cfg.Timeouts.SSHRetries,
						"retry_delay", cfg.Timeouts.SSHRetryDelayDuration().String(),
					)
				} else {
					logger.Info("ssh connectivity probe",
						"host", cfg.VM.Host,
						"port", cfg.VM.Port,
						"connect_timeout", cfg.Timeouts.SSHConnectDuration().String(),
						"retries", cfg.Timeouts.SSHRetries,
						"retry_delay", cfg.Timeouts.SSHRetryDelayDuration().String(),
					)
				}
				stats, err := waitForTCPPortWithStatsFn(
					ctx,
					cfg.VM.Host,
					cfg.VM.Port,
					cfg.Timeouts.SSHRetries,
					cfg.Timeouts.SSHConnectDuration(),
					cfg.Timeouts.SSHRetryDelayDuration(),
				)
				if err != nil {
					return err
				}
				if opts.HumanProgress {
					logger.Debug("ssh connectivity ready",
						"attempts_used", stats.Attempts,
						"elapsed", stats.Elapsed.Truncate(time.Millisecond).String(),
					)
				} else {
					logger.Info("ssh connectivity ready",
						"attempts_used", stats.Attempts,
						"elapsed", stats.Elapsed.Truncate(time.Millisecond).String(),
					)
				}
				return nil
			},
		},
		{
			name: "os_hardening",
			desc: "Apply idempotent OS hardening baseline",
			run: func(ctx context.Context) error {
				return runOSHardeningFn(ctx, logger, cfg)
			},
		},
		{
			name: "docker_install",
			desc: "Install pinned Docker version",
			run: func(ctx context.Context) error {
				return runDockerInstallFn(ctx, logger, cfg)
			},
		},
		{
			name: "talosctl_install",
			desc: "Install pinned talosctl and verify checksum",
			run: func(ctx context.Context) error {
				return runTalosctlInstallFn(ctx, logger, cfg)
			},
		},
		{
			name: "cluster_create",
			desc: "Create Talos-in-Docker cluster if missing",
			run: func(ctx context.Context) error {
				return runClusterCreateFn(ctx, logger, cfg)
			},
		},
	}

	total := len(steps)
	for i, s := range steps {
		current := i + 1
		pct := (current - 1) * 100 / total
		if opts.HumanProgress {
			fmt.Printf("\033[36m[%d/%d]\033[0m \033[1m%s\033[0m \033[90m(%d%%)\033[0m\n", current, total, humanStepLabel(s.name), pct)
			fmt.Printf("  \033[90m%s\033[0m\n", s.desc)
		} else {
			logger.Info("progress",
				"step", fmt.Sprintf("[%d/%d]", current, total),
				"percent", fmt.Sprintf("%d%%", pct),
				"current", s.name,
				"description", s.desc,
			)
		}
		started := time.Now()
		stopHeartbeat := func() {}
		if opts.HumanProgress {
			stopHeartbeat = startStepHeartbeat(s.name)
		}
		if !opts.HumanProgress {
			logger.Info("step start", "step", s.name, "description", s.desc)
		}
		err := s.run(ctx)
		stopHeartbeat()
		d := time.Since(started)
		if err != nil {
			if opts.HumanProgress {
				fmt.Printf("  \033[31m✗ failed\033[0m in %s\n", d.Truncate(time.Millisecond))
			}
			res.Steps = append(res.Steps, Step{Name: s.name, Status: model.StepStatusFailed, Duration: d, Message: err.Error()})
			res.Status = "failed"
			res.Error = fmt.Sprintf("step %s failed: %v", s.name, err)
			res.EndedAt = time.Now().UTC()
			return res, errors.New(res.Error)
		}
		res.Steps = append(res.Steps, Step{Name: s.name, Status: model.StepStatusSuccess, Duration: d})
		donePct := current * 100 / total
		if opts.HumanProgress {
			fmt.Printf("  \033[32m✓ done\033[0m in %s \033[90m[%d/%d %d%%]\033[0m\n", d.Truncate(time.Millisecond), current, total, donePct)
		} else {
			logger.Info("step success",
				"step", s.name,
				"duration", d.String(),
				"progress", fmt.Sprintf("[%d/%d] %d%%", current, total, donePct),
			)
		}
	}

	res.Status = "success"
	res.EndedAt = time.Now().UTC()
	return res, nil
}

func humanStepLabel(step string) string {
	return strings.ReplaceAll(step, "_", "-")
}

func startStepHeartbeat(step string) func() {
	done := make(chan struct{})
	started := time.Now()
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				fmt.Printf("  \033[90m... %s running (%s)\033[0m\n", humanStepLabel(step), time.Since(started).Truncate(time.Second))
			case <-done:
				return
			}
		}
	}()
	return func() { close(done) }
}
