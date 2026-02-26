package workflow

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStabilizeHostFingerprintReturnsAfterTwoConsecutiveMatches(t *testing.T) {
	orig := scanHostKeyFingerprintFn
	t.Cleanup(func() { scanHostKeyFingerprintFn = orig })

	seq := []string{"A", "B", "B"}
	scanHostKeyFingerprintFn = func(ctx context.Context, host string, port int) (string, error) {
		if len(seq) == 0 {
			return "B", nil
		}
		fp := seq[0]
		seq = seq[1:]
		return fp, nil
	}

	fp, err := stabilizeHostFingerprint(context.Background(), "example.com", 22)
	if err != nil {
		t.Fatalf("stabilizeHostFingerprint error: %v", err)
	}
	if fp != "B" {
		t.Fatalf("expected stabilized fingerprint B, got %q", fp)
	}
}

func TestStabilizeHostFingerprintReturnsContextErrorWhenProbeFails(t *testing.T) {
	orig := scanHostKeyFingerprintFn
	t.Cleanup(func() { scanHostKeyFingerprintFn = orig })

	scanHostKeyFingerprintFn = func(ctx context.Context, host string, port int) (string, error) {
		return "", errors.New("scan failed")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := stabilizeHostFingerprint(ctx, "example.com", 22)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestRefreshBootstrapFingerprintGuards(t *testing.T) {
	if _, err := RefreshBootstrapFingerprint("x.yaml", nil); err == nil {
		t.Fatalf("expected nil guard error")
	}
	res := BootstrapResult{}
	changed, err := RefreshBootstrapFingerprint("", &res)
	if err != nil || changed {
		t.Fatalf("expected no-op for empty path, changed=%v err=%v", changed, err)
	}
	changed, err = RefreshBootstrapFingerprint("x.yaml", &res)
	if err != nil || changed {
		t.Fatalf("expected no-op for empty host, changed=%v err=%v", changed, err)
	}
}

func TestRefreshBootstrapFingerprintUpdatesFile(t *testing.T) {
	orig := scanHostKeyFingerprintFn
	t.Cleanup(func() { scanHostKeyFingerprintFn = orig })

	const newFP = "SHA256:abcdefghijklmnopqrstuvwxyzABCDEFGH0123456789+/"
	seq := []string{newFP, newFP}
	scanHostKeyFingerprintFn = func(ctx context.Context, host string, port int) (string, error) {
		if len(seq) == 0 {
			return "SHA256:newfp", nil
		}
		fp := seq[0]
		seq = seq[1:]
		return fp, nil
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "bootstrap-result.yaml")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatalf("write bootstrap result: %v", err)
	}

	res := BootstrapResult{
		VMName:             "devvm",
		IPAddress:          "192.168.1.10",
		SSHUser:            "dev",
		SSHPrivateKey:      "/tmp/id_ed25519",
		SSHPort:            22,
		SSHHostFingerprint: "SHA256:oldoldoldoldoldoldoldoldoldoldoldoldoldoldoldold",
	}
	changed, err := RefreshBootstrapFingerprint(path, &res)
	if err != nil {
		t.Fatalf("RefreshBootstrapFingerprint: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed=true")
	}
	if res.SSHHostFingerprint != newFP {
		t.Fatalf("unexpected refreshed fingerprint: %q", res.SSHHostFingerprint)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read updated result: %v", err)
	}
	if !strings.Contains(string(data), "ssh_host_fingerprint: "+newFP) {
		t.Fatalf("expected updated fingerprint in file, got: %s", string(data))
	}
}

func TestStabilizeHostFingerprintSkipsEmptyProbeThenConverges(t *testing.T) {
	orig := scanHostKeyFingerprintFn
	t.Cleanup(func() { scanHostKeyFingerprintFn = orig })

	seq := []string{"", "SHA256:x", "SHA256:x"}
	scanHostKeyFingerprintFn = func(ctx context.Context, host string, port int) (string, error) {
		fp := seq[0]
		seq = seq[1:]
		return fp, nil
	}

	fp, err := stabilizeHostFingerprint(context.Background(), "example.com", 22)
	if err != nil {
		t.Fatalf("stabilizeHostFingerprint error: %v", err)
	}
	if fp != "SHA256:x" {
		t.Fatalf("expected stabilized fingerprint SHA256:x, got %q", fp)
	}
}
