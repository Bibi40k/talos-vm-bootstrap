package workflow

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Bibi40k/talos-vm-bootstrap/internal/ssh"
	vmconfig "github.com/Bibi40k/vmware-vm-bootstrap/pkg/config"
)

var scanHostKeyFingerprintFn = ssh.ScanHostKeyFingerprint

// RefreshBootstrapFingerprint re-checks the host fingerprint until it stabilizes
// and updates the result file if needed.
func RefreshBootstrapFingerprint(path string, result *BootstrapResult) (bool, error) {
	if result == nil {
		return false, fmt.Errorf("bootstrap result is nil")
	}
	if strings.TrimSpace(path) == "" {
		return false, nil
	}
	host := strings.TrimSpace(result.IPAddress)
	if host == "" {
		return false, nil
	}
	port := result.SSHPort
	if port <= 0 {
		port = 22
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	fp, err := stabilizeHostFingerprint(ctx, host, port)
	if err != nil {
		return false, err
	}
	if fp == "" || fp == result.SSHHostFingerprint {
		return false, nil
	}
	result.SSHHostFingerprint = fp
	if err := vmconfig.SaveBootstrapResult(path, vmconfig.BootstrapResult{
		VMName:             result.VMName,
		IPAddress:          result.IPAddress,
		SSHUser:            result.SSHUser,
		SSHPrivateKey:      result.SSHPrivateKey,
		SSHPort:            result.SSHPort,
		SSHHostFingerprint: result.SSHHostFingerprint,
	}); err != nil {
		return false, fmt.Errorf("update bootstrap result: %w", err)
	}
	return true, nil
}

func stabilizeHostFingerprint(ctx context.Context, host string, port int) (string, error) {
	const (
		requiredConsecutive = 2
		probeInterval       = 900 * time.Millisecond
	)

	prev := ""
	consecutive := 0
	var lastErr error

	for {
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return "", fmt.Errorf("stabilize ssh host fingerprint: %w (last probe error: %v)", ctx.Err(), lastErr)
			}
			return "", fmt.Errorf("stabilize ssh host fingerprint: %w", ctx.Err())
		default:
		}

		fp, err := scanHostKeyFingerprintFn(ctx, host, port)
		if err != nil {
			lastErr = err
			time.Sleep(probeInterval)
			continue
		}
		if strings.TrimSpace(fp) == "" {
			time.Sleep(probeInterval)
			continue
		}

		if fp == prev {
			consecutive++
		} else {
			prev = fp
			consecutive = 1
		}

		if consecutive >= requiredConsecutive {
			return fp, nil
		}
		time.Sleep(probeInterval)
	}
}
