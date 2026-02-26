package ssh

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

const hostKeyAlgorithm = "ssh-ed25519"

type ExecConfig struct {
	Host                  string
	Port                  int
	User                  string
	PrivateKeyPath        string
	KnownHostsFile        string
	KnownHostsMode        string
	ExpectedHostKeySHA256 string
	Prompt                func(message string) (bool, error)
	ConnectTimeout        time.Duration
}

func RunScript(ctx context.Context, cfg ExecConfig, script string) (string, string, error) {
	return RunScriptWithCommand(ctx, cfg, "sudo -n bash -s", script)
}

func RunScriptWithCommand(ctx context.Context, cfg ExecConfig, remoteCommand string, script string) (string, string, error) {
	args := buildSSHArgs(cfg, remoteCommand)
	return runSSHCommand(ctx, cfg, args, script, "ssh run script failed")
}

func RunCommand(ctx context.Context, cfg ExecConfig, remoteCommand string) (string, string, error) {
	args := buildSSHArgs(cfg, remoteCommand)
	return runSSHCommand(ctx, cfg, args, "", "ssh run command failed")
}

func runSSHCommand(ctx context.Context, cfg ExecConfig, args []string, stdinScript string, prefix string) (string, string, error) {
	if err := ensureExpectedHostKey(ctx, cfg); err != nil {
		return "", "", err
	}

	cmd := exec.CommandContext(ctx, "ssh", args...)
	if stdinScript != "" {
		cmd.Stdin = strings.NewReader(stdinScript)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if shouldAutoRefreshKnownHost(cfg, stderr.String()) {
			allow := true
			if normalizeKnownHostsMode(cfg.KnownHostsMode) == "prompt" {
				if cfg.Prompt == nil {
					return stdout.String(), stderr.String(), formatSSHRunError(prefix, err, stderr.String())
				}
				ok, pErr := cfg.Prompt("SSH host key changed. Accept new host key?")
				if pErr != nil {
					return stdout.String(), stderr.String(), fmt.Errorf("known_hosts prompt failed: %w", pErr)
				}
				allow = ok
			}
			if allow {
				if recErr := autoRefreshKnownHost(ctx, cfg); recErr == nil {
					retry := exec.CommandContext(ctx, "ssh", args...)
					if stdinScript != "" {
						retry.Stdin = strings.NewReader(stdinScript)
					}
					stdout.Reset()
					stderr.Reset()
					retry.Stdout = &stdout
					retry.Stderr = &stderr
					if retryErr := retry.Run(); retryErr == nil {
						return stdout.String(), stderr.String(), nil
					} else {
						return stdout.String(), stderr.String(), formatSSHRunError(prefix, retryErr, stderr.String())
					}
				}
			}
		}
		return stdout.String(), stderr.String(), formatSSHRunError(prefix, err, stderr.String())
	}

	return stdout.String(), stderr.String(), nil
}

func shouldAutoRefreshKnownHost(cfg ExecConfig, stderr string) bool {
	if strings.TrimSpace(cfg.KnownHostsFile) == "" {
		return false
	}
	mode := normalizeKnownHostsMode(cfg.KnownHostsMode)
	if mode != "auto-refresh" && mode != "prompt" {
		return false
	}
	msg := strings.ToLower(stderr)
	return strings.Contains(msg, "host key verification failed") &&
		(strings.Contains(msg, "host identification has changed") || strings.Contains(msg, "offending"))
}

func autoRefreshKnownHost(ctx context.Context, cfg ExecConfig) error {
	host := strings.TrimSpace(cfg.Host)
	knownHosts := strings.TrimSpace(cfg.KnownHostsFile)
	if host == "" || knownHosts == "" {
		return fmt.Errorf("known_hosts refresh requires host and known_hosts path")
	}

	if err := os.MkdirAll(filepathDirSafe(knownHosts), 0o700); err != nil {
		return fmt.Errorf("create known_hosts dir: %w", err)
	}

	hostTarget, scanArgs := knownHostsTarget(cfg)
	remove := exec.CommandContext(ctx, "ssh-keygen", "-f", knownHosts, "-R", hostTarget)
	_ = remove.Run() // best-effort: entry may not exist

	scan := exec.CommandContext(ctx, "ssh-keyscan", append([]string{"-H", "-t", "ed25519"}, scanArgs...)...)
	out, err := scan.Output()
	if err != nil {
		return fmt.Errorf("ssh-keyscan %s: %w", hostTarget, err)
	}
	f, err := os.OpenFile(knownHosts, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open known_hosts %s: %w", knownHosts, err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Write(out); err != nil {
		return fmt.Errorf("append known_hosts %s: %w", knownHosts, err)
	}
	return nil
}

func ensureExpectedHostKey(ctx context.Context, cfg ExecConfig) error {
	expected := strings.TrimSpace(cfg.ExpectedHostKeySHA256)
	if expected == "" {
		return nil
	}
	if strings.TrimSpace(cfg.KnownHostsFile) == "" {
		return fmt.Errorf("known_hosts_file is required when expected host fingerprint is set")
	}

	fp, entry, allFPs, err := scanHostKeyFingerprintSet(ctx, cfg)
	if err != nil {
		return err
	}
	if _, ok := allFPs[expected]; !ok {
		mode := normalizeKnownHostsMode(cfg.KnownHostsMode)
		if mode == "auto-refresh" || mode == "prompt" {
			allow := mode == "auto-refresh"
			if mode == "prompt" {
				if cfg.Prompt == nil {
					return fmt.Errorf("ssh host fingerprint mismatch (expected %s, got %s)", expected, fp)
				}
				ok, pErr := cfg.Prompt(fmt.Sprintf("SSH host key changed (expected %s, got %s). Accept new host key?", expected, fp))
				if pErr != nil {
					return fmt.Errorf("known_hosts prompt failed: %w", pErr)
				}
				allow = ok
			}
			if allow {
				stableFP, stableEntry, err := ensureStableScannedHostKey(ctx, cfg, fp, entry)
				if err != nil {
					return err
				}
				_ = stableFP // explicit for readability: stability is validated before trust update
				if err := writeKnownHostsEntry(cfg, stableEntry); err != nil {
					return err
				}
				return nil
			}
		}
		return fmt.Errorf("ssh host fingerprint mismatch (expected %s, got %s)", expected, fp)
	}
	if err := writeKnownHostsEntry(cfg, entry); err != nil {
		return err
	}
	return nil
}

func ensureStableScannedHostKey(ctx context.Context, cfg ExecConfig, firstFP, firstEntry string) (string, string, error) {
	// Avoid trusting a host key that is still rotating during early boot.
	timer := time.NewTimer(800 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return "", "", ctx.Err()
	case <-timer.C:
	}

	secondFP, secondEntry, _, err := scanHostKeyFingerprintSet(ctx, cfg)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(firstFP) == "" || strings.TrimSpace(secondFP) == "" {
		return "", "", fmt.Errorf("ssh host fingerprint scan returned empty value")
	}
	if firstFP != secondFP {
		return "", "", fmt.Errorf("ssh host fingerprint changed during verification (%s -> %s); retry after VM stabilizes", firstFP, secondFP)
	}
	if strings.TrimSpace(secondEntry) == "" {
		return "", "", fmt.Errorf("ssh host key entry is empty after verification")
	}
	return secondFP, secondEntry, nil
}

func scanHostKeyFingerprint(ctx context.Context, cfg ExecConfig) (string, string, error) {
	fp, entry, _, err := scanHostKeyFingerprintSet(ctx, cfg)
	return fp, entry, err
}

func scanHostKeyFingerprintSet(ctx context.Context, cfg ExecConfig) (string, string, map[string]string, error) {
	hostTarget, scanArgs := knownHostsTarget(cfg)
	args := append([]string{"-H", "-t", "ed25519"}, scanArgs...)
	out, err := exec.CommandContext(ctx, "ssh-keyscan", args...).Output()
	if err != nil {
		return "", "", nil, fmt.Errorf("ssh-keyscan %s: %w", hostTarget, err)
	}
	lines := strings.Split(string(out), "\n")
	allFPs := make(map[string]string)
	preferredFP := ""
	preferredEntry := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		keyLine := fields[1] + " " + fields[2]
		key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(keyLine))
		if err != nil {
			continue
		}
		fp := ssh.FingerprintSHA256(key)
		entry := line + "\n"
		allFPs[fp] = entry
		if preferredFP == "" {
			preferredFP = fp
			preferredEntry = entry
		}
	}
	if preferredFP != "" {
		return preferredFP, preferredEntry, allFPs, nil
	}
	return "", "", nil, fmt.Errorf("no valid host key found for %s", hostTarget)
}

// ScanHostKeyFingerprint returns the SHA256 fingerprint for host:port.
func ScanHostKeyFingerprint(ctx context.Context, host string, port int) (string, error) {
	if strings.TrimSpace(host) == "" {
		return "", fmt.Errorf("host is empty")
	}
	cfg := ExecConfig{Host: host, Port: port}
	fp, _, err := scanHostKeyFingerprint(ctx, cfg)
	return fp, err
}

func writeKnownHostsEntry(cfg ExecConfig, entry string) error {
	hostTarget, _ := knownHostsTarget(cfg)
	knownHosts := strings.TrimSpace(cfg.KnownHostsFile)
	if knownHosts == "" {
		return fmt.Errorf("known_hosts path is empty")
	}
	if err := os.MkdirAll(filepathDirSafe(knownHosts), 0o700); err != nil {
		return fmt.Errorf("create known_hosts dir: %w", err)
	}
	remove := exec.CommandContext(context.Background(), "ssh-keygen", "-f", knownHosts, "-R", hostTarget)
	_ = remove.Run()
	f, err := os.OpenFile(knownHosts, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open known_hosts %s: %w", knownHosts, err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(entry); err != nil {
		return fmt.Errorf("append known_hosts %s: %w", knownHosts, err)
	}
	return nil
}

func knownHostsTarget(cfg ExecConfig) (string, []string) {
	host := strings.TrimSpace(cfg.Host)
	port := cfg.Port
	if port <= 0 || port == 22 {
		return host, []string{host}
	}
	return fmt.Sprintf("[%s]:%d", host, port), []string{"-p", strconv.Itoa(port), host}
}

func normalizeKnownHostsMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "strict":
		return "strict"
	case "prompt":
		return "prompt"
	case "accept-new", "accept_new":
		return "accept-new"
	case "auto-refresh", "auto_refresh":
		return "auto-refresh"
	default:
		return "strict"
	}
}

func filepathDirSafe(path string) string {
	dir := filepath.Dir(path)
	if dir == "" {
		return "."
	}
	return dir
}

func formatSSHRunError(prefix string, runErr error, stderr string) error {
	errText := summarizeSSHStderr(stderr)
	if errText == "" {
		return fmt.Errorf("%s: %w", prefix, runErr)
	}
	const max = 360
	if len(errText) > max {
		errText = errText[:max] + "..."
	}
	return fmt.Errorf("%s: %w (%s)", prefix, runErr, errText)
}

func summarizeSSHStderr(stderr string) string {
	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	filtered := make([]string, 0, len(lines))
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if t == "" {
			continue
		}
		// Drop noisy curl progress rows.
		if strings.Contains(t, "% Total") || strings.Contains(t, "Dload") || strings.Contains(t, "--:--:--") {
			continue
		}
		filtered = append(filtered, t)
	}
	if len(filtered) == 0 {
		return strings.TrimSpace(stderr)
	}
	if len(filtered) == 1 {
		return filtered[0]
	}
	// Keep the last 2 lines; they usually contain the actionable error.
	start := len(filtered) - 2
	if start < 0 {
		start = 0
	}
	return strings.Join(filtered[start:], " | ")
}

func buildSSHArgs(cfg ExecConfig, remoteCommand string) []string {
	connectSeconds := int(cfg.ConnectTimeout / time.Second)
	if connectSeconds <= 0 {
		connectSeconds = 5
	}

	args := []string{
		"-o", "BatchMode=yes",
		"-o", "IdentitiesOnly=yes",
		"-o", "HostKeyAlgorithms=" + hostKeyAlgorithm,
		"-o", "ConnectTimeout=" + strconv.Itoa(connectSeconds),
		"-p", strconv.Itoa(cfg.Port),
		"-i", cfg.PrivateKeyPath,
	}

	mode := normalizeKnownHostsMode(cfg.KnownHostsMode)
	strict := "yes"
	if mode == "accept-new" {
		strict = "accept-new"
	}

	if cfg.KnownHostsFile != "" {
		args = append(args,
			"-o", "StrictHostKeyChecking="+strict,
			"-o", "UserKnownHostsFile="+cfg.KnownHostsFile,
		)
	} else {
		args = append(args, "-o", "StrictHostKeyChecking="+strict)
	}

	destination := cfg.User + "@" + cfg.Host
	args = append(args, destination, remoteCommand)
	return args
}
