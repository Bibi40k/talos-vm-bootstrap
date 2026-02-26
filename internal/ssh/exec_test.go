package ssh

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestShouldAutoRefreshKnownHost(t *testing.T) {
	cfg := ExecConfig{KnownHostsFile: "/tmp/known_hosts", Host: "1.2.3.4", KnownHostsMode: "auto-refresh"}
	msg := "Host key verification failed. Offending ECDSA key in /tmp/known_hosts"
	if !shouldAutoRefreshKnownHost(cfg, msg) {
		t.Fatalf("expected auto-refresh to be true")
	}
}

func TestShouldNotAutoRefreshKnownHostWithoutKnownHosts(t *testing.T) {
	cfg := ExecConfig{KnownHostsFile: ""}
	msg := "Host key verification failed"
	if shouldAutoRefreshKnownHost(cfg, msg) {
		t.Fatalf("expected auto-refresh to be false")
	}
}

func TestSummarizeSSHStderrFiltersNoise(t *testing.T) {
	in := `% Total    % Received
something useful
another useful line`
	got := summarizeSSHStderr(in)
	if strings.Contains(got, "% Total") {
		t.Fatalf("expected progress noise to be filtered: %q", got)
	}
	if got != "something useful | another useful line" {
		t.Fatalf("unexpected summarize output: %q", got)
	}
}

func TestBuildSSHArgsWithKnownHosts(t *testing.T) {
	cfg := ExecConfig{
		Host:           "10.0.0.1",
		Port:           22,
		User:           "dev",
		PrivateKeyPath: "/tmp/key",
		KnownHostsFile: "/tmp/known_hosts",
		ConnectTimeout: 5 * time.Second,
	}
	args := buildSSHArgs(cfg, "echo ok")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "StrictHostKeyChecking=yes") {
		t.Fatalf("expected strict host key checking")
	}
	if !strings.Contains(joined, "HostKeyAlgorithms=ssh-ed25519") {
		t.Fatalf("expected ssh-ed25519 host key algorithm pin")
	}
	if !strings.Contains(joined, "UserKnownHostsFile=/tmp/known_hosts") {
		t.Fatalf("expected known_hosts file in args")
	}
	if !strings.Contains(joined, "dev@10.0.0.1") {
		t.Fatalf("expected destination in args")
	}
}

func TestBuildSSHArgsAcceptNewWhenNoKnownHosts(t *testing.T) {
	cfg := ExecConfig{
		Host:           "10.0.0.1",
		Port:           22,
		User:           "dev",
		PrivateKeyPath: "/tmp/key",
		KnownHostsMode: "accept-new",
		ConnectTimeout: 5 * time.Second,
	}
	args := buildSSHArgs(cfg, "echo ok")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "StrictHostKeyChecking=accept-new") {
		t.Fatalf("expected accept-new when known_hosts missing")
	}
}

func TestKnownHostsTargetUsesPortNotation(t *testing.T) {
	cfg := ExecConfig{
		Host: "10.0.0.1",
		Port: 2222,
	}
	target, scanArgs := knownHostsTarget(cfg)
	if target != "[10.0.0.1]:2222" {
		t.Fatalf("unexpected target: %q", target)
	}
	joined := strings.Join(scanArgs, " ")
	if !strings.Contains(joined, "-p 2222") || !strings.Contains(joined, "10.0.0.1") {
		t.Fatalf("unexpected scan args: %q", joined)
	}
}

func TestFormatSSHRunErrorIncludesSummary(t *testing.T) {
	err := formatSSHRunError("ssh run command failed", errors.New("exit status 1"), "line1\nline2")
	msg := err.Error()
	if !strings.Contains(msg, "ssh run command failed") || !strings.Contains(msg, "line1") {
		t.Fatalf("unexpected error formatting: %q", msg)
	}
}

func TestRunCommandSuccessWithMockSSH(t *testing.T) {
	binDir := t.TempDir()
	sshScript := "#!/usr/bin/env bash\necho ok\n"
	writeExecutable(t, filepath.Join(binDir, "ssh"), sshScript)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	cfg := ExecConfig{
		Host:           "10.0.0.1",
		Port:           22,
		User:           "dev",
		PrivateKeyPath: "/tmp/key",
		KnownHostsMode: "accept-new",
		ConnectTimeout: 2 * time.Second,
	}
	out, _, err := RunCommand(context.Background(), cfg, "echo ok")
	if err != nil {
		t.Fatalf("RunCommand failed: %v", err)
	}
	if strings.TrimSpace(out) != "ok" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestRunCommandAutoRefreshKnownHost(t *testing.T) {
	binDir := t.TempDir()
	marker := filepath.Join(binDir, "first")
	sshScript := "#!/usr/bin/env bash\nif [ ! -f \"" + marker + "\" ]; then\n  echo \"Host key verification failed. Offending ECDSA key in /tmp/known_hosts\" 1>&2\n  touch \"" + marker + "\"\n  exit 255\nfi\necho ok\n"
	sshKeyscan := "#!/usr/bin/env bash\necho \"example.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIEd1Y3N0b21rZXk=\"\n"
	sshKeygen := "#!/usr/bin/env bash\nexit 0\n"
	writeExecutable(t, filepath.Join(binDir, "ssh"), sshScript)
	writeExecutable(t, filepath.Join(binDir, "ssh-keyscan"), sshKeyscan)
	writeExecutable(t, filepath.Join(binDir, "ssh-keygen"), sshKeygen)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	knownHosts := filepath.Join(t.TempDir(), "known_hosts")
	cfg := ExecConfig{
		Host:           "example.com",
		Port:           22,
		User:           "dev",
		PrivateKeyPath: "/tmp/key",
		KnownHostsFile: knownHosts,
		KnownHostsMode: "auto-refresh",
		ConnectTimeout: 2 * time.Second,
	}
	out, _, err := RunCommand(context.Background(), cfg, "echo ok")
	if err != nil {
		t.Fatalf("RunCommand failed: %v", err)
	}
	if strings.TrimSpace(out) != "ok" {
		t.Fatalf("unexpected output: %q", out)
	}
	data, err := os.ReadFile(knownHosts)
	if err != nil || len(data) == 0 {
		t.Fatalf("expected known_hosts to be written")
	}
}

func TestEnsureExpectedHostKey(t *testing.T) {
	keyDir := t.TempDir()
	priv := filepath.Join(keyDir, "id_ed25519")
	pub := priv + ".pub"
	if out, err := exec.Command("ssh-keygen", "-t", "ed25519", "-N", "", "-f", priv).CombinedOutput(); err != nil {
		t.Fatalf("ssh-keygen failed: %v (%s)", err, string(out))
	}
	pubData, err := os.ReadFile(pub)
	if err != nil {
		t.Fatalf("read pub: %v", err)
	}
	fields := strings.Fields(string(pubData))
	if len(fields) < 2 {
		t.Fatalf("invalid pub key")
	}
	keyLine := fields[0] + " " + fields[1]
	key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(keyLine))
	if err != nil {
		t.Fatalf("parse pub key: %v", err)
	}
	expected := ssh.FingerprintSHA256(key)

	binDir := t.TempDir()
	sshKeyscan := "#!/usr/bin/env bash\necho \"example.com " + fields[0] + " " + fields[1] + "\"\n"
	sshKeygen := "#!/usr/bin/env bash\nexit 0\n"
	writeExecutable(t, filepath.Join(binDir, "ssh-keyscan"), sshKeyscan)
	writeExecutable(t, filepath.Join(binDir, "ssh-keygen"), sshKeygen)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	knownHosts := filepath.Join(t.TempDir(), "known_hosts")
	cfg := ExecConfig{
		Host:                  "example.com",
		Port:                  22,
		User:                  "dev",
		PrivateKeyPath:        "/tmp/key",
		KnownHostsFile:        knownHosts,
		ExpectedHostKeySHA256: expected,
	}
	if err := ensureExpectedHostKey(context.Background(), cfg); err != nil {
		t.Fatalf("ensureExpectedHostKey failed: %v", err)
	}
	if data, err := os.ReadFile(knownHosts); err != nil || len(data) == 0 {
		t.Fatalf("expected known_hosts entry")
	}
}

func TestEnsureExpectedHostKeyMismatchStrictFails(t *testing.T) {
	keyType, keyData := generatePublicKeyFields(t)

	binDir := t.TempDir()
	sshKeyscan := "#!/usr/bin/env bash\necho \"example.com " + keyType + " " + keyData + "\"\n"
	sshKeygen := "#!/usr/bin/env bash\nexit 0\n"
	writeExecutable(t, filepath.Join(binDir, "ssh-keyscan"), sshKeyscan)
	writeExecutable(t, filepath.Join(binDir, "ssh-keygen"), sshKeygen)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	cfg := ExecConfig{
		Host:                  "example.com",
		Port:                  22,
		User:                  "dev",
		PrivateKeyPath:        "/tmp/key",
		KnownHostsFile:        filepath.Join(t.TempDir(), "known_hosts"),
		KnownHostsMode:        "strict",
		ExpectedHostKeySHA256: "SHA256:does-not-match",
	}
	err := ensureExpectedHostKey(context.Background(), cfg)
	if err == nil || !strings.Contains(err.Error(), "fingerprint mismatch") {
		t.Fatalf("expected strict mismatch error, got: %v", err)
	}
}

func TestEnsureExpectedHostKeyMismatchAutoRefreshAccepts(t *testing.T) {
	keyType, keyData := generatePublicKeyFields(t)

	binDir := t.TempDir()
	sshKeyscan := "#!/usr/bin/env bash\necho \"example.com " + keyType + " " + keyData + "\"\n"
	sshKeygen := "#!/usr/bin/env bash\nexit 0\n"
	writeExecutable(t, filepath.Join(binDir, "ssh-keyscan"), sshKeyscan)
	writeExecutable(t, filepath.Join(binDir, "ssh-keygen"), sshKeygen)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	knownHosts := filepath.Join(t.TempDir(), "known_hosts")
	cfg := ExecConfig{
		Host:                  "example.com",
		Port:                  22,
		User:                  "dev",
		PrivateKeyPath:        "/tmp/key",
		KnownHostsFile:        knownHosts,
		KnownHostsMode:        "auto-refresh",
		ExpectedHostKeySHA256: "SHA256:does-not-match",
	}
	if err := ensureExpectedHostKey(context.Background(), cfg); err != nil {
		t.Fatalf("expected auto-refresh mismatch to pass, got: %v", err)
	}
	if data, err := os.ReadFile(knownHosts); err != nil || len(data) == 0 {
		t.Fatalf("expected known_hosts to be updated")
	}
}

func TestEnsureExpectedHostKeyMismatchAutoRefreshFailsWhenKeyUnstable(t *testing.T) {
	keyTypeA, keyDataA := generatePublicKeyFields(t)
	keyTypeB, keyDataB := generatePublicKeyFields(t)

	binDir := t.TempDir()
	marker := filepath.Join(binDir, "scan_once")
	sshKeyscan := "#!/usr/bin/env bash\nif [ ! -f \"" + marker + "\" ]; then\n  touch \"" + marker + "\"\n  echo \"example.com " + keyTypeA + " " + keyDataA + "\"\n  exit 0\nfi\necho \"example.com " + keyTypeB + " " + keyDataB + "\"\n"
	sshKeygen := "#!/usr/bin/env bash\nexit 0\n"
	writeExecutable(t, filepath.Join(binDir, "ssh-keyscan"), sshKeyscan)
	writeExecutable(t, filepath.Join(binDir, "ssh-keygen"), sshKeygen)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	cfg := ExecConfig{
		Host:                  "example.com",
		Port:                  22,
		User:                  "dev",
		PrivateKeyPath:        "/tmp/key",
		KnownHostsFile:        filepath.Join(t.TempDir(), "known_hosts"),
		KnownHostsMode:        "auto-refresh",
		ExpectedHostKeySHA256: "SHA256:does-not-match",
	}
	err := ensureExpectedHostKey(context.Background(), cfg)
	if err == nil || !strings.Contains(err.Error(), "changed during verification") {
		t.Fatalf("expected unstable key verification failure, got: %v", err)
	}
}

func TestEnsureExpectedHostKeyStrictAcceptsExpectedFingerprintFromAnyKeyType(t *testing.T) {
	keyTypeA, keyDataA := generatePublicKeyFields(t)
	keyTypeB, keyDataB := generatePublicKeyFields(t)

	keyB, _, _, _, err := ssh.ParseAuthorizedKey([]byte(keyTypeB + " " + keyDataB))
	if err != nil {
		t.Fatalf("parse key B: %v", err)
	}
	expected := ssh.FingerprintSHA256(keyB)

	binDir := t.TempDir()
	sshKeyscan := "#!/usr/bin/env bash\n" +
		"echo \"example.com " + keyTypeA + " " + keyDataA + "\"\n" +
		"echo \"example.com " + keyTypeB + " " + keyDataB + "\"\n"
	sshKeygen := "#!/usr/bin/env bash\nexit 0\n"
	writeExecutable(t, filepath.Join(binDir, "ssh-keyscan"), sshKeyscan)
	writeExecutable(t, filepath.Join(binDir, "ssh-keygen"), sshKeygen)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	cfg := ExecConfig{
		Host:                  "example.com",
		Port:                  22,
		User:                  "dev",
		PrivateKeyPath:        "/tmp/key",
		KnownHostsFile:        filepath.Join(t.TempDir(), "known_hosts"),
		KnownHostsMode:        "strict",
		ExpectedHostKeySHA256: expected,
	}
	if err := ensureExpectedHostKey(context.Background(), cfg); err != nil {
		t.Fatalf("expected strict mode to accept matching fingerprint from key set, got: %v", err)
	}
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func generatePublicKeyFields(t *testing.T) (string, string) {
	t.Helper()
	keyDir := t.TempDir()
	priv := filepath.Join(keyDir, "id_ed25519")
	pub := priv + ".pub"
	if out, err := exec.Command("ssh-keygen", "-t", "ed25519", "-N", "", "-f", priv).CombinedOutput(); err != nil {
		t.Fatalf("ssh-keygen failed: %v (%s)", err, string(out))
	}
	pubData, err := os.ReadFile(pub)
	if err != nil {
		t.Fatalf("read pub: %v", err)
	}
	fields := strings.Fields(string(pubData))
	if len(fields) < 2 {
		t.Fatalf("invalid pub key")
	}
	return fields[0], fields[1]
}
