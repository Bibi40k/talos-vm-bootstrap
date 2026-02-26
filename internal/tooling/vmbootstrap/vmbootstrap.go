package vmbootstrap

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ResolveOptions controls vmbootstrap binary resolution behavior.
type ResolveOptions struct {
	Bin       string
	Repo      string
	AutoBuild bool
}

func ResolveBinary(opts ResolveOptions) (string, error) {
	bin := strings.TrimSpace(opts.Bin)
	if bin == "" {
		return "", errors.New("vmbootstrap binary is empty")
	}

	if isRunnable(bin) {
		return bin, nil
	}
	if !strings.Contains(bin, "/") {
		if path, err := exec.LookPath(bin); err == nil {
			return path, nil
		}
	}

	if opts.AutoBuild {
		if opts.Repo == "" {
			return "", errors.New("vmbootstrap auto-build enabled but repo path is empty")
		}
		info, err := os.Stat(opts.Repo)
		if err != nil || !info.IsDir() {
			return "", fmt.Errorf("vmbootstrap repo missing: %s", opts.Repo)
		}
		cmd := exec.Command("make", "-C", opts.Repo, "build-cli")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("build vmbootstrap in %s: %w", opts.Repo, err)
		}
		candidate := filepath.Join(opts.Repo, "bin", "vmbootstrap")
		if isRunnable(candidate) {
			return candidate, nil
		}
		return "", fmt.Errorf("vmbootstrap build finished but binary missing: %s", candidate)
	}

	return "", fmt.Errorf("vmbootstrap not found: %s", bin)
}

func IsUpdateAvailable() (current string, latest string, hasUpdate bool, err error) {
	current, err = CurrentPinnedVersion()
	if err != nil {
		return "", "", false, err
	}

	updCmd := exec.Command("go", "list", "-m", "-u", "-f", "{{if .Update}}{{.Update.Version}}{{end}}", "github.com/Bibi40k/vmware-vm-bootstrap")
	updOut, updErr := updCmd.Output()
	if updErr != nil {
		updCmdDirect := exec.Command("go", "list", "-m", "-u", "-f", "{{if .Update}}{{.Update.Version}}{{end}}", "github.com/Bibi40k/vmware-vm-bootstrap")
		updCmdDirect.Env = append(os.Environ(), "GOPROXY=direct")
		updOut, updErr = updCmdDirect.Output()
		if updErr != nil {
			return current, "", false, fmt.Errorf("check vmbootstrap module updates: %w", updErr)
		}
	}
	latest = strings.TrimSpace(string(updOut))
	hasUpdate = latest != ""
	return current, latest, hasUpdate, nil
}

func CurrentPinnedVersion() (string, error) {
	curCmd := exec.Command("go", "list", "-m", "-f", "{{.Version}}", "github.com/Bibi40k/vmware-vm-bootstrap")
	curOut, curErr := curCmd.Output()
	if curErr != nil {
		return "", fmt.Errorf("resolve current vmbootstrap module version: %w", curErr)
	}
	return strings.TrimSpace(string(curOut)), nil
}

func UpdatePinToLatest() (string, error) {
	getCmd := exec.Command("go", "get", "github.com/Bibi40k/vmware-vm-bootstrap@latest")
	getCmd.Stdout = os.Stdout
	getCmd.Stderr = os.Stderr
	if err := getCmd.Run(); err != nil {
		return "", fmt.Errorf("update vmbootstrap module pin: %w", err)
	}

	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Stdout = os.Stdout
	tidyCmd.Stderr = os.Stderr
	if err := tidyCmd.Run(); err != nil {
		return "", fmt.Errorf("go mod tidy after vmbootstrap pin update: %w", err)
	}

	curCmd := exec.Command("go", "list", "-m", "-f", "{{.Version}}", "github.com/Bibi40k/vmware-vm-bootstrap")
	curOut, err := curCmd.Output()
	if err != nil {
		return "", fmt.Errorf("read updated vmbootstrap module version: %w", err)
	}
	return strings.TrimSpace(string(curOut)), nil
}

func InstallPinnedToDir(dir string) (version string, installedPath string, err error) {
	version, err = CurrentPinnedVersion()
	if err != nil {
		return "", "", err
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", "", fmt.Errorf("resolve install dir %s: %w", dir, err)
	}
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create install dir %s: %w", absDir, err)
	}

	cmd := exec.Command("go", "install", "github.com/Bibi40k/vmware-vm-bootstrap/cmd/vmbootstrap@"+version)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "GOBIN="+absDir)
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("install pinned vmbootstrap (%s): %w", version, err)
	}
	return version, filepath.Join(absDir, "vmbootstrap"), nil
}

var syncedAssets = []string{
	"configs/defaults.yaml",
	"configs/vcenter.example.yaml",
	"configs/vm.example.yaml",
	".sops.yaml.example",
	".sopsrc.example",
}

func CheckPinnedAssets(repoRoot string) ([]string, error) {
	moduleDir, err := moduleSourceDir()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(repoRoot) == "" {
		repoRoot = "."
	}
	rootAbs, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve repo root %s: %w", repoRoot, err)
	}

	var drift []string
	for _, rel := range syncedAssets {
		src := filepath.Join(moduleDir, rel)
		dst := filepath.Join(rootAbs, rel)
		status, err := assetDiffStatus(src, dst)
		if err != nil {
			return nil, err
		}
		if status != "" {
			drift = append(drift, fmt.Sprintf("%s (%s)", rel, status))
		}
	}
	return drift, nil
}

func assetDiffStatus(src, dst string) (string, error) {
	srcData, err := os.ReadFile(src)
	if err != nil {
		return "", fmt.Errorf("read source asset %s: %w", src, err)
	}
	dstData, err := os.ReadFile(dst)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "missing", nil
		}
		return "", fmt.Errorf("read destination asset %s: %w", dst, err)
	}
	if bytes.Equal(dstData, srcData) {
		return "", nil
	}
	return "differs", nil
}

func SyncPinnedAssets(repoRoot string, force bool) ([]string, error) {
	moduleDir, err := moduleSourceDir()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(repoRoot) == "" {
		repoRoot = "."
	}
	rootAbs, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve repo root %s: %w", repoRoot, err)
	}

	results := make([]string, 0, len(syncedAssets))
	for _, rel := range syncedAssets {
		src := filepath.Join(moduleDir, rel)
		dst := filepath.Join(rootAbs, rel)
		if err := syncOneFile(src, dst, force); err != nil {
			return nil, err
		}
		results = append(results, rel)
	}
	return results, nil
}

func moduleSourceDir() (string, error) {
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/Bibi40k/vmware-vm-bootstrap")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("resolve vmbootstrap module source dir: %w", err)
	}
	dir := strings.TrimSpace(string(out))
	if dir == "" {
		return "", errors.New("empty vmbootstrap module source dir")
	}
	info, statErr := os.Stat(dir)
	if statErr != nil || !info.IsDir() {
		return "", fmt.Errorf("vmbootstrap module source dir invalid: %s", dir)
	}
	return dir, nil
}

func syncOneFile(src, dst string, force bool) error {
	srcData, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read source asset %s: %w", src, err)
	}

	if dstData, readErr := os.ReadFile(dst); readErr == nil {
		if bytes.Equal(dstData, srcData) {
			return nil
		}
		if !force {
			return fmt.Errorf("local asset differs: %s (run with FORCE=1 to overwrite)", dst)
		}
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return fmt.Errorf("read destination asset %s: %w", dst, readErr)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create dir for %s: %w", dst, err)
	}
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat source asset %s: %w", src, err)
	}
	mode := info.Mode().Perm()
	if mode == 0 {
		mode = 0o644
	}
	if err := os.WriteFile(dst, srcData, mode); err != nil {
		return fmt.Errorf("write destination asset %s: %w", dst, err)
	}
	return nil
}

func isRunnable(bin string) bool {
	if strings.Contains(bin, "/") {
		info, err := os.Stat(bin)
		return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
	}
	_, err := exec.LookPath(bin)
	return err == nil
}
