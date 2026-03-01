package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/Bibi40k/talos-docker-bootstrap/internal/bootstrap"
	"github.com/Bibi40k/talos-docker-bootstrap/internal/config"
)

func maybeSetKnownHostsPrompt(cfg config.Config, allowPrompt bool) func() {
	if !allowPrompt {
		return func() {}
	}
	mode := strings.ToLower(strings.TrimSpace(cfg.VM.KnownHostsMode))
	if mode != "prompt" {
		return func() {}
	}
	return bootstrap.SetKnownHostsPrompt(promptKnownHosts)
}

func promptKnownHosts(message string) (bool, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("\n\033[33m⚠ %s\033[0m [y/N]: ", message)
	raw, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	s := strings.ToLower(strings.TrimSpace(raw))
	return s == "y" || s == "yes", nil
}
