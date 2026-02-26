package workflow

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type vmConfigFile struct {
	VM struct {
		Name               string `yaml:"name"`
		IPAddress          string `yaml:"ip_address"`
		Username           string `yaml:"username"`
		SSHKeyPath         string `yaml:"ssh_key_path"`
		SSHPort            int    `yaml:"ssh_port"`
		SSHHostFingerprint string `yaml:"ssh_host_fingerprint"`
	} `yaml:"vm"`
}

// LoadBootstrapResultFromVMConfig derives a BootstrapResult from a vmware-vm-bootstrap VM config.
func LoadBootstrapResultFromVMConfig(path string) (BootstrapResult, error) {
	content, err := readVMConfig(path)
	if err != nil {
		return BootstrapResult{}, err
	}

	var cfg vmConfigFile
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return BootstrapResult{}, fmt.Errorf("parse vm config %s: %w", path, err)
	}

	result := BootstrapResult{
		VMName:             strings.TrimSpace(cfg.VM.Name),
		IPAddress:          strings.TrimSpace(cfg.VM.IPAddress),
		SSHUser:            strings.TrimSpace(cfg.VM.Username),
		SSHPrivateKey:      resolveSSHPrivateKeyPath(cfg.VM.SSHKeyPath),
		SSHPort:            cfg.VM.SSHPort,
		SSHHostFingerprint: strings.TrimSpace(cfg.VM.SSHHostFingerprint),
	}
	if result.SSHPort == 0 {
		result.SSHPort = 22
	}
	if err := result.Validate(); err != nil {
		return BootstrapResult{}, err
	}
	return result, nil
}

func readVMConfig(path string) ([]byte, error) {
	if strings.Contains(strings.ToLower(path), ".sops.") {
		cmd := exec.Command("sops", "-d", path)
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("decrypt vm config %s: %w", path, err)
		}
		return bytes.TrimSpace(out), nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read vm config %s: %w", path, err)
	}
	return content, nil
}

func resolveSSHPrivateKeyPath(path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		return ""
	}
	if strings.HasSuffix(strings.ToLower(p), ".pub") {
		priv := strings.TrimSuffix(p, ".pub")
		if st, err := os.Stat(priv); err == nil && !st.IsDir() {
			return priv
		}
	}
	if strings.Contains(p, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			p = strings.Replace(p, "~", home, 1)
		}
	}
	if !filepath.IsAbs(p) {
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
	}
	return p
}
