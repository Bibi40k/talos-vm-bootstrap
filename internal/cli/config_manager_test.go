package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDeriveVMBootstrapWorkDir(t *testing.T) {
	tests := []struct {
		name string
		bin  string
		want string
	}{
		{
			name: "path from sibling vmware repo bin",
			bin:  "../vmware-vm-bootstrap/bin/vmbootstrap",
			want: "../vmware-vm-bootstrap",
		},
		{
			name: "absolute path from bin dir",
			bin:  "/opt/tools/vmware-vm-bootstrap/bin/vmbootstrap",
			want: "/opt/tools/vmware-vm-bootstrap",
		},
		{
			name: "binary in PATH has no fixed workdir",
			bin:  "vmbootstrap",
			want: "",
		},
		{
			name: "non-bin path has no fixed workdir",
			bin:  "/tmp/vmbootstrap",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := deriveVMBootstrapWorkDir(tt.bin); got != tt.want {
				t.Fatalf("deriveVMBootstrapWorkDir(%q)=%q, want %q", tt.bin, got, tt.want)
			}
		})
	}
}

func TestVersionPrompt(t *testing.T) {
	got := versionPrompt("Talosctl version", "1.12.4", "2026-02-13")
	want := "Talosctl version (latest known: 1.12.4, released 2026-02-13)"
	if got != want {
		t.Fatalf("unexpected prompt: %q", got)
	}
}

func TestTalosChecksumForVersion(t *testing.T) {
	meta := toolVersionMetadata{}
	meta.Talosctl.ChecksumsLinuxAMD64 = map[string]string{
		"1.12.4": "abc",
	}
	if got := talosChecksumForVersion(meta, "v1.12.4"); got != "abc" {
		t.Fatalf("unexpected checksum: %q", got)
	}
}

func TestResolveTalosChecksumFromMetadata(t *testing.T) {
	meta := toolVersionMetadata{}
	meta.Talosctl.ChecksumsLinuxAMD64 = map[string]string{
		"1.12.4": "abc",
	}
	got, err := resolveTalosChecksum(meta, "1.12.4")
	if err != nil {
		t.Fatalf("resolveTalosChecksum failed: %v", err)
	}
	if got != "abc" {
		t.Fatalf("unexpected checksum: %q", got)
	}
}

func TestFetchTalosChecksumFromRelease(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  talosctl-linux-amd64\n"))
	}))
	defer srv.Close()

	origFmt := talosReleaseChecksumsURLFmt
	talosReleaseChecksumsURLFmt = srv.URL + "/v%s/sha256sum.txt"
	t.Cleanup(func() { talosReleaseChecksumsURLFmt = origFmt })

	got, err := fetchTalosChecksumFromRelease("1.12.4")
	if err != nil {
		t.Fatalf("fetchTalosChecksumFromRelease failed: %v", err)
	}
	if got != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("unexpected checksum: %q", got)
	}
}

func TestNormalizeClusterName(t *testing.T) {
	if got := normalizeClusterName("Dan Serban_WORK"); got != "dan-serban-work" {
		t.Fatalf("unexpected normalized name: %q", got)
	}
}

func TestAdjustStateDirForClusterName(t *testing.T) {
	got := adjustStateDirForClusterName("/home/dev/.talos/clusters/devvm", "devvm", "danserban-work")
	if got != "/home/dev/.talos/clusters/danserban-work" {
		t.Fatalf("unexpected adjusted state dir: %q", got)
	}
}

func TestSuggestSSHPrivateKeyPath(t *testing.T) {
	dir := t.TempDir()
	priv := filepath.Join(dir, "id_ed25519")
	pub := priv + ".pub"
	if err := os.WriteFile(priv, []byte("k"), 0o600); err != nil {
		t.Fatalf("write private key: %v", err)
	}
	if err := os.WriteFile(pub, []byte("k"), 0o600); err != nil {
		t.Fatalf("write public key: %v", err)
	}
	if got := suggestSSHPrivateKeyPath(pub); got != priv {
		t.Fatalf("unexpected private key suggestion: %q", got)
	}
}

func TestDraftHelpers(t *testing.T) {
	base := filepath.Join(t.TempDir(), "configs", "talos-bootstrap.yaml")
	draft := stage2DraftPath(base)
	if err := os.MkdirAll(filepath.Dir(base), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	gotDraft, err := writeStage2Draft(base, "", []byte("vm: {}\n"))
	if err != nil {
		t.Fatalf("writeStage2Draft failed: %v", err)
	}
	if gotDraft != draft {
		t.Fatalf("unexpected draft path: got %q want %q", gotDraft, draft)
	}
	if draft == base {
		t.Fatalf("expected distinct draft path")
	}
	list := listStage2Drafts(base)
	if len(list) != 1 || list[0] != draft {
		t.Fatalf("unexpected drafts: %#v", list)
	}
	if err := cleanupStage2Draft(base); err != nil {
		t.Fatalf("cleanupStage2Draft failed: %v", err)
	}
	if fileExists(draft) {
		t.Fatalf("expected draft cleanup")
	}
}

func TestSanitizeSuggestion(t *testing.T) {
	t.Setenv("HOME", "/home/test")
	if got := sanitizeSuggestion("${HOME}/x"); got != "/home/test/x" {
		t.Fatalf("expected expanded placeholder, got %q", got)
	}
	if got := sanitizeSuggestion(" ${HOME} "); got != "/home/test" {
		t.Fatalf("expected env expansion, got %q", got)
	}
}

func TestApplyClusterNameFallback(t *testing.T) {
	cfg := stage2File{}
	cfg.VM.Host = "Dan-Serban_Work"
	applyClusterNameFallback(&cfg)
	if cfg.Cluster.Name != "dan-serban-work" {
		t.Fatalf("unexpected fallback cluster name: %q", cfg.Cluster.Name)
	}
}

func TestApplyClusterNameSuggestionFromBootstrap(t *testing.T) {
	dir := t.TempDir()
	fakeSops := installFakeSops(t, dir)
	t.Setenv("PATH", fakeSops+string(os.PathListSeparator)+os.Getenv("PATH"))
	oldwd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	if err := os.MkdirAll("configs", 0o755); err != nil {
		t.Fatalf("mkdir configs: %v", err)
	}
	vmCfg := []byte("vm:\n  name: DanSerban-Work\n  ip_address: 1.2.3.4\n  username: dev\n")
	if err := os.WriteFile("configs/vm.danserban.sops.yaml", vmCfg, 0o600); err != nil {
		t.Fatalf("write vm config: %v", err)
	}

	cfg := stage2File{}
	cfg.Cluster.Name = "devvm"
	applyClusterNameSuggestionFromBootstrap(&cfg)
	if cfg.Cluster.Name != "danserban-work" {
		t.Fatalf("unexpected suggested name: %q", cfg.Cluster.Name)
	}
}

func TestLatestVMConfigPathAndLoad(t *testing.T) {
	dir := t.TempDir()
	fakeSops := installFakeSops(t, dir)
	t.Setenv("PATH", fakeSops+string(os.PathListSeparator)+os.Getenv("PATH"))
	oldwd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	if err := os.MkdirAll("configs", 0o755); err != nil {
		t.Fatalf("mkdir configs: %v", err)
	}
	a := "configs/vm.a.sops.yaml"
	b := "configs/vm.b.sops.yaml"
	if err := os.WriteFile(a, []byte("vm:\n  name: a\n"), 0o600); err != nil {
		t.Fatalf("write a: %v", err)
	}
	// Ensure deterministic mtime ordering.
	now := time.Now()
	if err := os.Chtimes(a, now.Add(-2*time.Minute), now.Add(-2*time.Minute)); err != nil {
		t.Fatalf("chtimes a: %v", err)
	}
	timeLater := []byte("vm:\n  name: b\n  ip_address: 10.0.0.2\n  username: dev\n  ssh_key_path: /tmp/key\n  ssh_port: 22\n")
	if err := os.WriteFile(b, timeLater, 0o600); err != nil {
		t.Fatalf("write b: %v", err)
	}

	got, err := latestVMConfigPath()
	if err != nil {
		t.Fatalf("latestVMConfigPath failed: %v", err)
	}
	if !strings.HasSuffix(got, "vm.b.sops.yaml") {
		t.Fatalf("unexpected latest path: %q", got)
	}
	bootstrapVM, err := loadVMBootstrapConfig(got)
	if err != nil {
		t.Fatalf("loadVMBootstrapConfig failed: %v", err)
	}
	if bootstrapVM.VM.Name != "b" {
		t.Fatalf("unexpected bootstrap vm name: %q", bootstrapVM.VM.Name)
	}
}

func TestLoadYAMLAndSaveYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.yaml")
	cfg := stage2File{}
	cfg.VM.Host = "1.2.3.4"
	if err := saveYAML(path, cfg); err != nil {
		t.Fatalf("saveYAML failed: %v", err)
	}
	var got stage2File
	if err := loadYAML(path, &got); err != nil {
		t.Fatalf("loadYAML failed: %v", err)
	}
	if got.VM.Host != "1.2.3.4" {
		t.Fatalf("unexpected loaded host: %q", got.VM.Host)
	}
}

func TestResolveTalosChecksumErrorsForEmptyVersion(t *testing.T) {
	_, err := resolveTalosChecksum(toolVersionMetadata{}, "")
	if err == nil || !strings.Contains(err.Error(), "empty talos version") {
		t.Fatalf("expected empty version error, got %v", err)
	}
}

func installFakeSops(t *testing.T, root string) string {
	t.Helper()
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir fake sops dir: %v", err)
	}
	path := filepath.Join(binDir, "sops")
	script := "#!/usr/bin/env bash\nset -euo pipefail\ncat \"$4\"\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake sops: %v", err)
	}
	return binDir
}
