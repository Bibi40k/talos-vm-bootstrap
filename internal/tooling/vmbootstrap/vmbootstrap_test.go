package vmbootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestResolveBinaryFromAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "vmbootstrap")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	got, err := ResolveBinary(ResolveOptions{Bin: bin})
	if err != nil {
		t.Fatalf("resolve binary: %v", err)
	}
	if got != bin {
		t.Fatalf("unexpected binary path: got %q want %q", got, bin)
	}
}

func TestResolveBinaryFromPATH(t *testing.T) {
	got, err := ResolveBinary(ResolveOptions{Bin: "sh"})
	if err != nil {
		t.Fatalf("resolve binary from PATH: %v", err)
	}
	if got == "" {
		t.Fatalf("expected non-empty resolved path")
	}
}

func TestResolveBinaryMissing(t *testing.T) {
	_, err := ResolveBinary(ResolveOptions{Bin: "/does/not/exist/vmbootstrap"})
	if err == nil {
		t.Fatalf("expected error for missing binary")
	}
}

func TestSyncOneFileCreateAndForce(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")

	if err := os.WriteFile(src, []byte("v1"), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	if err := syncOneFile(src, dst, false); err != nil {
		t.Fatalf("syncOneFile create failed: %v", err)
	}

	if err := os.WriteFile(src, []byte("v2"), 0o644); err != nil {
		t.Fatalf("rewrite src: %v", err)
	}
	if err := syncOneFile(src, dst, false); err == nil {
		t.Fatalf("expected conflict without force")
	}
	if err := syncOneFile(src, dst, true); err != nil {
		t.Fatalf("syncOneFile force failed: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(got) != "v2" {
		t.Fatalf("unexpected dst content: %q", string(got))
	}
}

func TestSyncOneFileMissingSource(t *testing.T) {
	dir := t.TempDir()
	err := syncOneFile(filepath.Join(dir, "missing"), filepath.Join(dir, "dst"), false)
	if err == nil || !strings.Contains(err.Error(), "read source asset") {
		t.Fatalf("expected source read error, got %v", err)
	}
}

func TestSyncDefaultsFileMergesMissingKeysWithoutForce(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src-defaults.yaml")
	dst := filepath.Join(dir, "dst-defaults.yaml")

	srcYAML := "" +
		"network:\n" +
		"  interface: ens192\n" +
		"talos:\n" +
		"  default_version: v1.12.4\n" +
		"  plan_network:\n" +
		"    cidr: 192.168.110.0/24\n" +
		"    gateway: 192.168.110.1\n"
	dstYAML := "" +
		"network:\n" +
		"  interface: eth0\n" +
		"talos:\n" +
		"  default_version: v1.11.0\n" +
		"  plan_network:\n" +
		"    cidr: 192.168.115.0/24\n"

	if err := os.WriteFile(src, []byte(srcYAML), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	if err := os.WriteFile(dst, []byte(dstYAML), 0o644); err != nil {
		t.Fatalf("write dst: %v", err)
	}

	if err := syncDefaultsFile(src, dst, false); err != nil {
		t.Fatalf("syncDefaultsFile failed: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}

	var merged map[string]any
	if err := yaml.Unmarshal(got, &merged); err != nil {
		t.Fatalf("unmarshal merged yaml: %v", err)
	}

	if network := merged["network"].(map[string]any); network["interface"] != "eth0" {
		t.Fatalf("expected local network.interface preserved, got %v", network["interface"])
	}
	talos := merged["talos"].(map[string]any)
	if talos["default_version"] != "v1.11.0" {
		t.Fatalf("expected local talos.default_version preserved, got %v", talos["default_version"])
	}
	planNet := talos["plan_network"].(map[string]any)
	if planNet["cidr"] != "192.168.115.0/24" {
		t.Fatalf("expected local talos.plan_network.cidr preserved, got %v", planNet["cidr"])
	}
	if planNet["gateway"] != "192.168.110.1" {
		t.Fatalf("expected missing key talos.plan_network.gateway added, got %v", planNet["gateway"])
	}
}

func TestSyncDefaultsFileNoMissingKeysDoesNotOverwriteWithoutForce(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src-defaults.yaml")
	dst := filepath.Join(dir, "dst-defaults.yaml")

	srcYAML := "" +
		"network:\n" +
		"  interface: ens192\n"
	dstYAML := "" +
		"network:\n" +
		"  interface: eth0\n"

	if err := os.WriteFile(src, []byte(srcYAML), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	if err := os.WriteFile(dst, []byte(dstYAML), 0o644); err != nil {
		t.Fatalf("write dst: %v", err)
	}

	if err := syncDefaultsFile(src, dst, false); err != nil {
		t.Fatalf("syncDefaultsFile failed: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(got) != dstYAML {
		t.Fatalf("expected dst to remain unchanged without force")
	}
}

func TestResolveBinaryAutoBuildMissingRepo(t *testing.T) {
	_, err := ResolveBinary(ResolveOptions{
		Bin:       "/missing/vmbootstrap",
		Repo:      "/missing/repo",
		AutoBuild: true,
	})
	if err == nil {
		t.Fatalf("expected auto-build repo error")
	}
}

func TestModuleSourceDir(t *testing.T) {
	dir, err := moduleSourceDir()
	if err != nil {
		t.Fatalf("moduleSourceDir failed: %v", err)
	}
	if dir == "" {
		t.Fatalf("expected non-empty module dir")
	}
}

func TestCurrentPinnedVersionWithMockGo(t *testing.T) {
	binDir := t.TempDir()
	writeExecutable(t, filepath.Join(binDir, "go"), mockGoScript())
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GO_MOCK_VERSION", "v0.1.4")

	got, err := CurrentPinnedVersion()
	if err != nil {
		t.Fatalf("CurrentPinnedVersion failed: %v", err)
	}
	if got != "v0.1.4" {
		t.Fatalf("unexpected version: %q", got)
	}
}

func TestIsUpdateAvailableWithMockGo(t *testing.T) {
	binDir := t.TempDir()
	writeExecutable(t, filepath.Join(binDir, "go"), mockGoScript())
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GO_MOCK_VERSION", "v0.1.4")
	t.Setenv("GO_MOCK_UPDATE", "v0.2.0")

	current, latest, hasUpdate, err := IsUpdateAvailable()
	if err != nil {
		t.Fatalf("IsUpdateAvailable failed: %v", err)
	}
	if current != "v0.1.4" || latest != "v0.2.0" || !hasUpdate {
		t.Fatalf("unexpected update info: %s %s %v", current, latest, hasUpdate)
	}
}

func TestUpdatePinToLatestWithMockGo(t *testing.T) {
	binDir := t.TempDir()
	writeExecutable(t, filepath.Join(binDir, "go"), mockGoScript())
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GO_MOCK_VERSION", "v0.2.0")

	got, err := UpdatePinToLatest()
	if err != nil {
		t.Fatalf("UpdatePinToLatest failed: %v", err)
	}
	if got != "v0.2.0" {
		t.Fatalf("unexpected version: %q", got)
	}
}

func TestInstallPinnedToDirWithMockGo(t *testing.T) {
	binDir := t.TempDir()
	writeExecutable(t, filepath.Join(binDir, "go"), mockGoScript())
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GO_MOCK_VERSION", "v0.1.4")

	installDir := t.TempDir()
	version, bin, err := InstallPinnedToDir(installDir)
	if err != nil {
		t.Fatalf("InstallPinnedToDir failed: %v", err)
	}
	if version != "v0.1.4" {
		t.Fatalf("unexpected version: %q", version)
	}
	if _, err := os.Stat(bin); err != nil {
		t.Fatalf("expected installed binary: %v", err)
	}
}

func TestSyncPinnedAssetsWithMockGo(t *testing.T) {
	modDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(modDir, "configs"), 0o755); err != nil {
		t.Fatalf("mkdir configs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "configs", "defaults.yaml"), []byte("defaults"), 0o644); err != nil {
		t.Fatalf("write defaults: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "configs", "vcenter.example.yaml"), []byte("vcenter"), 0o644); err != nil {
		t.Fatalf("write vcenter example: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "configs", "vm.example.yaml"), []byte("vm"), 0o644); err != nil {
		t.Fatalf("write vm example: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, ".sops.yaml.example"), []byte("sops"), 0o644); err != nil {
		t.Fatalf("write sops: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, ".sopsrc.example"), []byte("sopsrc"), 0o644); err != nil {
		t.Fatalf("write sopsrc: %v", err)
	}

	binDir := t.TempDir()
	writeExecutable(t, filepath.Join(binDir, "go"), mockGoScript())
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GO_MOCK_DIR", modDir)

	repo := t.TempDir()
	files, err := SyncPinnedAssets(repo, true)
	if err != nil {
		t.Fatalf("SyncPinnedAssets failed: %v", err)
	}
	if len(files) != 5 {
		t.Fatalf("expected 5 files synced, got %d", len(files))
	}
	drift, err := CheckPinnedAssets(repo)
	if err != nil {
		t.Fatalf("CheckPinnedAssets failed: %v", err)
	}
	if len(drift) != 0 {
		t.Fatalf("expected no drift, got %v", drift)
	}
}

func mockGoScript() string {
	return `#!/usr/bin/env bash
set -euo pipefail

if [[ "$1" == "list" && "$2" == "-m" && "$3" == "-u" ]]; then
  echo "${GO_MOCK_UPDATE:-}"
  exit 0
fi

if [[ "$1" == "list" && "$2" == "-m" && "$3" == "-f" ]]; then
  if [[ "$4" == "{{.Version}}" ]]; then
    echo "${GO_MOCK_VERSION:-v0.1.4}"
    exit 0
  fi
  if [[ "$4" == "{{.Dir}}" ]]; then
    echo "${GO_MOCK_DIR:-}"
    exit 0
  fi
fi

if [[ "$1" == "get" ]]; then
  exit 0
fi

if [[ "$1" == "mod" && "$2" == "tidy" ]]; then
  exit 0
fi

if [[ "$1" == "install" ]]; then
  bin="${GOBIN}/vmbootstrap"
  mkdir -p "$(dirname "${bin}")"
  echo "#!/usr/bin/env bash" > "${bin}"
  chmod +x "${bin}"
  exit 0
fi

echo "unsupported go command" >&2
exit 1
`
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
