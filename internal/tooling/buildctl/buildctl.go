package buildctl

import (
	"fmt"
	"os"
	"os/exec"
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
