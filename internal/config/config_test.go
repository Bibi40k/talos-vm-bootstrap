package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateSuccess(t *testing.T) {
	cfg := defaultConfig()
	cfg.VM.Host = "192.168.1.100"
	cfg.VM.User = "dev"
	cfg.VM.SSHPrivateKey = "~/.ssh/id_ed25519"
	cfg.Docker.Version = "28.0.2"
	cfg.Talos.Version = "1.12.3"
	cfg.Talos.SHA256Checksum = "2baf4747e5f6b7f3655f47c665b45dec0c4b6935f0be9614dfe2262c3079eb93"
	cfg.Cluster.Name = "devvm"
	cfg.Cluster.StateDir = "/home/dev/.talos/clusters/devvm"
	cfg.Cluster.MountSrc = "/home/dev/work"
	cfg.Cluster.MountDst = "/var/mnt/work"
	cfg.Hardening.AllowTCPPorts = []int{22, 6443}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}
}

func TestValidateMissingRequired(t *testing.T) {
	cfg := defaultConfig()
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected validation error for missing required fields")
	}
}

func TestValidateInvalidHardeningPort(t *testing.T) {
	cfg := defaultConfig()
	cfg.VM.Host = "192.168.1.100"
	cfg.VM.User = "dev"
	cfg.VM.SSHPrivateKey = "~/.ssh/id_ed25519"
	cfg.Docker.Version = "28.0.2"
	cfg.Talos.Version = "1.12.3"
	cfg.Talos.SHA256Checksum = "2baf4747e5f6b7f3655f47c665b45dec0c4b6935f0be9614dfe2262c3079eb93"
	cfg.Cluster.Name = "devvm"
	cfg.Cluster.StateDir = "/home/dev/.talos/clusters/devvm"
	cfg.Cluster.MountSrc = "/home/dev/work"
	cfg.Cluster.MountDst = "/var/mnt/work"
	cfg.Hardening.AllowTCPPorts = []int{0}

	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected validation error for invalid hardening port")
	}
}

func TestValidateInvalidTalosChecksum(t *testing.T) {
	cfg := defaultConfig()
	cfg.VM.Host = "192.168.1.100"
	cfg.VM.User = "dev"
	cfg.VM.SSHPrivateKey = "~/.ssh/id_ed25519"
	cfg.Docker.Version = "28.0.2"
	cfg.Talos.Version = "1.12.3"
	cfg.Talos.SHA256Checksum = "deadbeef"
	cfg.Cluster.Name = "devvm"
	cfg.Cluster.StateDir = "/home/dev/.talos/clusters/devvm"
	cfg.Cluster.MountSrc = "/home/dev/work"
	cfg.Cluster.MountDst = "/var/mnt/work"

	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected validation error for invalid talos checksum")
	}
}

func TestLoadAppliesEnvOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	content := []byte(`
vm:
  host: 10.0.0.1
  port: 22
  user: dev
  ssh_private_key: /tmp/key
docker:
  version: "28.5.2"
talos:
  version: "1.12.4"
  sha256_checksum: "6b85f633721e02d31c8a28a633c9cd8ebfb7e41677ff29e94236a082d4cd6cd9"
cluster:
  name: devvm
  state_dir: /tmp/state
  mount_src: /tmp/src
  mount_dst: /var/mnt/work
timeouts:
  ssh_connect_seconds: 5
  ssh_retries: 2
  ssh_retry_delay_seconds: 1
  total_minutes: 1
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write cfg: %v", err)
	}

	t.Setenv("TDB_VM_HOST", "192.168.0.10")
	t.Setenv("TDB_VM_USER", "bibi40k")
	t.Setenv("TDB_VM_SSH_PRIVATE_KEY", "/home/bibi40k/.ssh/DA")
	t.Setenv("TDB_CLUSTER_STATE_DIR", "/home/bibi40k/.talos/clusters/devvm")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.VM.Host != "192.168.0.10" || cfg.VM.User != "bibi40k" {
		t.Fatalf("env overrides not applied: %#v", cfg.VM)
	}
	if cfg.VM.SSHPrivateKey != "/home/bibi40k/.ssh/DA" {
		t.Fatalf("ssh key override missing: %q", cfg.VM.SSHPrivateKey)
	}
	if cfg.Cluster.StateDir != "/home/bibi40k/.talos/clusters/devvm" {
		t.Fatalf("state dir override missing: %q", cfg.Cluster.StateDir)
	}
}

func TestLoadExpandsHomePaths(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	t.Setenv("HOME", home)

	path := filepath.Join(dir, "cfg.yaml")
	content := []byte(`
vm:
  host: 10.0.0.1
  port: 22
  user: dev
  ssh_private_key: ~/.ssh/id_ed25519
  known_hosts_file: ~/.ssh/known_hosts
docker:
  version: "28.5.2"
talos:
  version: "1.12.4"
  sha256_checksum: "6b85f633721e02d31c8a28a633c9cd8ebfb7e41677ff29e94236a082d4cd6cd9"
cluster:
  name: devvm
  state_dir: ~/.talos/clusters/devvm
  mount_src: ~/work
  mount_dst: /var/mnt/work
timeouts:
  ssh_connect_seconds: 5
  ssh_retries: 2
  ssh_retry_delay_seconds: 1
  total_minutes: 1
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write cfg: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if !strings.HasPrefix(cfg.VM.SSHPrivateKey, home) {
		t.Fatalf("ssh_private_key not expanded: %q", cfg.VM.SSHPrivateKey)
	}
	if !strings.HasPrefix(cfg.VM.KnownHostsFile, home) {
		t.Fatalf("known_hosts_file not expanded: %q", cfg.VM.KnownHostsFile)
	}
	if !strings.HasPrefix(cfg.Cluster.StateDir, home) {
		t.Fatalf("state_dir not expanded: %q", cfg.Cluster.StateDir)
	}
	if !strings.HasPrefix(cfg.Cluster.MountSrc, home) {
		t.Fatalf("mount_src not expanded: %q", cfg.Cluster.MountSrc)
	}
}

func TestTimeoutDurations(t *testing.T) {
	tm := TimeoutsConfig{
		SSHConnectSeconds: 3,
		SSHRetryDelaySec:  7,
		TotalMinutes:      2,
	}
	if tm.SSHConnectDuration() != 3*time.Second {
		t.Fatalf("unexpected connect duration")
	}
	if tm.SSHRetryDelayDuration() != 7*time.Second {
		t.Fatalf("unexpected retry delay")
	}
	if tm.TotalDuration() != 2*time.Minute {
		t.Fatalf("unexpected total duration")
	}
}

func TestValidateRejectsPubKeyPath(t *testing.T) {
	cfg := defaultConfig()
	cfg.VM.Host = "192.168.1.100"
	cfg.VM.User = "dev"
	cfg.VM.SSHPrivateKey = "/tmp/id_ed25519.pub"
	cfg.Docker.Version = "28.0.2"
	cfg.Talos.Version = "1.12.3"
	cfg.Talos.SHA256Checksum = "2baf4747e5f6b7f3655f47c665b45dec0c4b6935f0be9614dfe2262c3079eb93"
	cfg.Cluster.Name = "devvm"
	cfg.Cluster.StateDir = "/home/dev/.talos/clusters/devvm"
	cfg.Cluster.MountSrc = "/home/dev/work"
	cfg.Cluster.MountDst = "/var/mnt/work"
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected .pub validation error")
	}
}

func TestValidateRejectsUnsafeVersions(t *testing.T) {
	cfg := defaultConfig()
	cfg.VM.Host = "192.168.1.100"
	cfg.VM.User = "dev"
	cfg.VM.SSHPrivateKey = "~/.ssh/id_ed25519"
	cfg.Docker.Version = "28.0.2;rm -rf /"
	cfg.Talos.Version = "1.12.3"
	cfg.Talos.SHA256Checksum = "2baf4747e5f6b7f3655f47c665b45dec0c4b6935f0be9614dfe2262c3079eb93"
	cfg.Cluster.Name = "devvm"
	cfg.Cluster.StateDir = "/home/dev/.talos/clusters/devvm"
	cfg.Cluster.MountSrc = "/home/dev/work"
	cfg.Cluster.MountDst = "/var/mnt/work"
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected unsafe docker.version validation error")
	}
}

func TestValidateMoreErrorBranches(t *testing.T) {
	base := defaultConfig()
	base.VM.Host = "192.168.1.100"
	base.VM.User = "dev"
	base.VM.SSHPrivateKey = "~/.ssh/id_ed25519"
	base.Docker.Version = "28.0.2"
	base.Talos.Version = "1.12.3"
	base.Talos.SHA256Checksum = "2baf4747e5f6b7f3655f47c665b45dec0c4b6935f0be9614dfe2262c3079eb93"
	base.Cluster.Name = "devvm"
	base.Cluster.StateDir = "/home/dev/.talos/clusters/devvm"
	base.Cluster.MountSrc = "/home/dev/work"
	base.Cluster.MountDst = "/var/mnt/work"

	tests := []struct {
		name string
		mut  func(*Config)
	}{
		{name: "invalid vm port", mut: func(c *Config) { c.VM.Port = 70000 }},
		{name: "invalid known hosts mode", mut: func(c *Config) { c.VM.KnownHostsMode = "weird" }},
		{name: "fingerprint requires known_hosts_file", mut: func(c *Config) { c.VM.SSHHostFingerprint = "SHA256:abc123"; c.VM.KnownHostsFile = "" }},
		{name: "invalid talos version token", mut: func(c *Config) { c.Talos.Version = "1.12.3;bad" }},
		{name: "missing cluster state dir", mut: func(c *Config) { c.Cluster.StateDir = "" }},
		{name: "missing mount src", mut: func(c *Config) { c.Cluster.MountSrc = "" }},
		{name: "missing mount dst", mut: func(c *Config) { c.Cluster.MountDst = "" }},
		{name: "invalid connect timeout", mut: func(c *Config) { c.Timeouts.SSHConnectSeconds = 0 }},
		{name: "invalid retries", mut: func(c *Config) { c.Timeouts.SSHRetries = 0 }},
		{name: "invalid retry delay", mut: func(c *Config) { c.Timeouts.SSHRetryDelaySec = 0 }},
		{name: "invalid total minutes", mut: func(c *Config) { c.Timeouts.TotalMinutes = 0 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := base
			tt.mut(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatalf("expected validation error")
			}
		})
	}
}
