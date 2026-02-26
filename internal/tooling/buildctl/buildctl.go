package buildctl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

func EnsureGoToolchain(required string) error {
	required = strings.TrimSpace(required)
	if required == "" {
		return fmt.Errorf("required go version is empty")
	}
	local, err := localGoVersion()
	if err != nil {
		return fmt.Errorf("go not found in PATH (local toolchain)\n  Run: make install-requirements")
	}
	if compareGoVersions(local, required) < 0 {
		return fmt.Errorf("local Go toolchain too old (local: %s, required: %s)\n  Run: make install-requirements", local, required)
	}
	return nil
}

func RequireConfig(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("config path is empty")
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("missing Talos bootstrap config: %s\n  Run: make config", path)
	}
	return nil
}

func RequireOut(out string) error {
	if strings.TrimSpace(out) == "" {
		return fmt.Errorf("missing OUT for kubeconfig export\n  Example: make kubeconfig-export OUT=build/devvm/kubeconfig")
	}
	return nil
}

func GenerateCoverageBadges(repoRoot string) error {
	root := strings.TrimSpace(repoRoot)
	if root == "" {
		root = "."
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve repo root %s: %w", root, err)
	}

	pkgsOut, err := runOutput(rootAbs, "go", "list", "./...")
	if err != nil {
		return fmt.Errorf("list go packages: %w", err)
	}
	allPkgs := filterPackages(strings.Fields(string(pkgsOut)))
	if len(allPkgs) == 0 {
		return fmt.Errorf("no packages found for coverage")
	}

	if err := os.MkdirAll(filepath.Join(rootAbs, "tmp"), 0o755); err != nil {
		return fmt.Errorf("create tmp dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(rootAbs, "docs", "coverage"), 0o755); err != nil {
		return fmt.Errorf("create docs/coverage dir: %w", err)
	}

	allCover := filepath.Join(rootAbs, "tmp", "coverage.all.out")
	if _, err := runOutput(rootAbs, "go", append([]string{"test", "-coverprofile=" + allCover}, allPkgs...)...); err != nil {
		return fmt.Errorf("run full coverage: %w", err)
	}
	allTotal, err := readCoverageTotal(rootAbs, allCover)
	if err != nil {
		return fmt.Errorf("parse full coverage total: %w", err)
	}

	coreCover := filepath.Join(rootAbs, "tmp", "coverage.core.out")
	corePkgs := []string{"./internal/bootstrap", "./internal/config", "./internal/workflow"}
	if _, err := runOutput(rootAbs, "go", append([]string{"test", "-coverprofile=" + coreCover}, corePkgs...)...); err != nil {
		return fmt.Errorf("run core coverage: %w", err)
	}
	coreTotal, err := readCoverageTotal(rootAbs, coreCover)
	if err != nil {
		return fmt.Errorf("parse core coverage total: %w", err)
	}

	if err := writeBadgeFile(filepath.Join(rootAbs, "docs", "coverage", "coverage-all.json"), "coverage-all", allTotal); err != nil {
		return err
	}
	if err := writeBadgeFile(filepath.Join(rootAbs, "docs", "coverage", "coverage-core.json"), "coverage-core", coreTotal); err != nil {
		return err
	}
	if err := writeBadgeFile(filepath.Join(rootAbs, "docs", "coverage", "coverage.json"), "coverage", allTotal); err != nil {
		return err
	}
	return nil
}

type badgePayload struct {
	SchemaVersion int    `json:"schemaVersion"`
	Label         string `json:"label"`
	Message       string `json:"message"`
	Color         string `json:"color"`
}

func writeBadgeFile(path, label, total string) error {
	color, err := coverageColor(total)
	if err != nil {
		return fmt.Errorf("derive badge color for %s: %w", label, err)
	}
	payload := badgePayload{
		SchemaVersion: 1,
		Label:         label,
		Message:       total,
		Color:         color,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal badge payload: %w", err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		return fmt.Errorf("write badge %s: %w", path, err)
	}
	return nil
}

func coverageColor(total string) (string, error) {
	pct, err := parsePercent(total)
	if err != nil {
		return "", err
	}
	switch {
	case pct >= 80:
		return "green", nil
	case pct >= 60:
		return "yellow", nil
	default:
		return "red", nil
	}
}

func readCoverageTotal(dir, profile string) (string, error) {
	out, err := runOutput(dir, "go", "tool", "cover", "-func="+profile)
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 {
		return "", fmt.Errorf("empty go tool cover output")
	}
	last := strings.TrimSpace(lines[len(lines)-1])
	fields := strings.Fields(last)
	if len(fields) == 0 {
		return "", fmt.Errorf("invalid coverage summary line: %q", last)
	}
	total := fields[len(fields)-1]
	if _, err := parsePercent(total); err != nil {
		return "", fmt.Errorf("invalid coverage percent %q: %w", total, err)
	}
	return total, nil
}

func parsePercent(s string) (float64, error) {
	re := regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)?)%$`)
	m := re.FindStringSubmatch(strings.TrimSpace(s))
	if len(m) != 2 {
		return 0, fmt.Errorf("invalid percent format: %s", s)
	}
	return strconv.ParseFloat(m[1], 64)
}

func filterPackages(pkgs []string) []string {
	out := make([]string, 0, len(pkgs))
	for _, p := range pkgs {
		if strings.Contains(p, "/tools") {
			continue
		}
		out = append(out, p)
	}
	return out
}

func runOutput(dir string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("%s %s failed: %s", name, strings.Join(args, " "), msg)
	}
	return stdout.Bytes(), nil
}

func localGoVersion() (string, error) {
	cmd := exec.Command("go", "env", "GOVERSION")
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	v := strings.TrimSpace(string(out))
	return strings.TrimPrefix(v, "go"), nil
}

func compareGoVersions(a, b string) int {
	pa := parseVersion(a)
	pb := parseVersion(b)
	for i := 0; i < 3; i++ {
		if pa[i] < pb[i] {
			return -1
		}
		if pa[i] > pb[i] {
			return 1
		}
	}
	return 0
}

func parseVersion(v string) [3]int {
	v = strings.TrimSpace(strings.TrimPrefix(v, "go"))
	base := strings.SplitN(v, "-", 2)[0]
	parts := strings.Split(base, ".")
	var out [3]int
	for i := 0; i < len(out) && i < len(parts); i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			continue
		}
		out[i] = n
	}
	return out
}
