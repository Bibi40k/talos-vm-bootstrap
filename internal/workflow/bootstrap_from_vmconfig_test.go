package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadBootstrapResultFromVMConfig(t *testing.T) {
	dir := t.TempDir()
	privateKey := filepath.Join(dir, "id_ed25519")
	publicKey := privateKey + ".pub"
	if err := os.WriteFile(privateKey, []byte("priv"), 0o600); err != nil {
		t.Fatalf("write private key: %v", err)
	}
	if err := os.WriteFile(publicKey, []byte("pub"), 0o644); err != nil {
		t.Fatalf("write public key: %v", err)
	}

	cfgPath := filepath.Join(dir, "vm.yaml")
	content := []byte(`
vm:
  name: "devvm-1"
  ip_address: "192.168.1.10"
  username: "dev"
  ssh_key_path: "` + publicKey + `"
  ssh_port: 2222
`)
	if err := os.WriteFile(cfgPath, content, 0o644); err != nil {
		t.Fatalf("write vm config: %v", err)
	}

	res, err := LoadBootstrapResultFromVMConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadBootstrapResultFromVMConfig: %v", err)
	}
	if res.VMName != "devvm-1" {
		t.Fatalf("unexpected vm name: %q", res.VMName)
	}
	if res.IPAddress != "192.168.1.10" {
		t.Fatalf("unexpected ip: %q", res.IPAddress)
	}
	if res.SSHUser != "dev" {
		t.Fatalf("unexpected ssh user: %q", res.SSHUser)
	}
	if res.SSHPrivateKey != privateKey {
		t.Fatalf("unexpected ssh key: %q", res.SSHPrivateKey)
	}
	if res.SSHPort != 2222 {
		t.Fatalf("unexpected ssh port: %d", res.SSHPort)
	}
}

func TestLoadBootstrapResultFromVMConfigDefaultsAndTildeKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	keyDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(keyDir, 0o755); err != nil {
		t.Fatalf("mkdir .ssh: %v", err)
	}
	priv := filepath.Join(keyDir, "id_ed25519")
	if err := os.WriteFile(priv, []byte("priv"), 0o600); err != nil {
		t.Fatalf("write private key: %v", err)
	}

	cfgPath := filepath.Join(t.TempDir(), "vm.yaml")
	content := []byte(`
vm:
  name: "devvm-2"
  ip_address: "10.0.0.2"
  username: "dev"
  ssh_key_path: "~/.ssh/id_ed25519"
`)
	if err := os.WriteFile(cfgPath, content, 0o644); err != nil {
		t.Fatalf("write vm config: %v", err)
	}
	res, err := LoadBootstrapResultFromVMConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadBootstrapResultFromVMConfig: %v", err)
	}
	if res.SSHPort != 22 {
		t.Fatalf("expected default ssh port 22, got %d", res.SSHPort)
	}
	if res.SSHPrivateKey != priv {
		t.Fatalf("expected resolved private key %q, got %q", priv, res.SSHPrivateKey)
	}
}

func TestReadVMConfigSOPSUsesDecryptCommand(t *testing.T) {
	binDir := t.TempDir()
	sopsPath := filepath.Join(binDir, "sops")
	script := "#!/usr/bin/env bash\necho \"vm:\n  name: devvm-sops\n  ip_address: 10.0.0.3\n  username: dev\n  ssh_key_path: /tmp/id_ed25519\"\n"
	if err := os.WriteFile(sopsPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake sops: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	content, err := readVMConfig("configs/vm.dev.sops.yaml")
	if err != nil {
		t.Fatalf("readVMConfig sops: %v", err)
	}
	if !strings.Contains(string(content), "devvm-sops") {
		t.Fatalf("expected decrypted content, got: %s", string(content))
	}
}

func TestReadVMConfigSOPSError(t *testing.T) {
	binDir := t.TempDir()
	sopsPath := filepath.Join(binDir, "sops")
	script := "#!/usr/bin/env bash\nexit 1\n"
	if err := os.WriteFile(sopsPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake failing sops: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, err := readVMConfig("configs/vm.dev.sops.yaml")
	if err == nil || !strings.Contains(err.Error(), "decrypt vm config") {
		t.Fatalf("expected decrypt vm config error, got: %v", err)
	}
}

func TestLoadBootstrapResultFromVMConfigInvalidYAML(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "vm.yaml")
	if err := os.WriteFile(cfgPath, []byte(":::invalid"), 0o644); err != nil {
		t.Fatalf("write vm config: %v", err)
	}
	_, err := LoadBootstrapResultFromVMConfig(cfgPath)
	if err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestResolveSSHPrivateKeyPathKeepsPubWhenPrivateMissing(t *testing.T) {
	dir := t.TempDir()
	pub := filepath.Join(dir, "id_ed25519.pub")
	if err := os.WriteFile(pub, []byte("pub"), 0o644); err != nil {
		t.Fatalf("write pub key: %v", err)
	}
	got := resolveSSHPrivateKeyPath(pub)
	if got == strings.TrimSuffix(pub, ".pub") {
		t.Fatalf("expected .pub path to remain when private key is missing, got %q", got)
	}
}
