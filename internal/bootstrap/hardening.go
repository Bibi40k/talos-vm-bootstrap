package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Bibi40k/talos-vm-bootstrap/internal/config"
)

func runOSHardening(ctx context.Context, logger *slog.Logger, cfg config.Config) error {
	if !cfg.Hardening.Enabled {
		logger.Info("os_hardening disabled by config")
		return nil
	}

	passwordAuth := "no"
	if cfg.Hardening.AllowPasswordSSH {
		passwordAuth = "yes"
	}

	ports := make([]string, 0, len(cfg.Hardening.AllowTCPPorts))
	for _, p := range cfg.Hardening.AllowTCPPorts {
		ports = append(ports, fmt.Sprintf("%d", p))
	}
	allowedPorts := strings.Join(ports, " ")
	enableUFW := "false"
	if cfg.Hardening.EnableUFW {
		enableUFW = "true"
	}

	script := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

MISSING=()
for PKG in openssh-server unattended-upgrades ufw ca-certificates curl; do
  if ! dpkg -s "$PKG" >/dev/null 2>&1; then
    MISSING+=("$PKG")
  fi
done

if [ "${#MISSING[@]}" -gt 0 ]; then
  apt-get update -y
  apt-get install -y --no-install-recommends "${MISSING[@]}"
fi

install -d -m 0755 /etc/ssh/sshd_config.d
SSH_DROPIN=/etc/ssh/sshd_config.d/99-talos-vm-bootstrap.conf
TMP_SSH="$(mktemp)"
cat > "$TMP_SSH" <<'SSHCFG'
PermitRootLogin no
PasswordAuthentication %s
KbdInteractiveAuthentication no
ChallengeResponseAuthentication no
PubkeyAuthentication yes
SSHCFG
if [ ! -f "$SSH_DROPIN" ] || ! cmp -s "$TMP_SSH" "$SSH_DROPIN"; then
  install -m 0644 "$TMP_SSH" "$SSH_DROPIN"
  systemctl reload ssh || systemctl reload sshd
fi
rm -f "$TMP_SSH"

SYSCTL_FILE=/etc/sysctl.d/99-talos-vm-bootstrap.conf
TMP_SYSCTL="$(mktemp)"
cat > "$TMP_SYSCTL" <<'SYSCTL'
net.ipv4.conf.all.rp_filter=1
net.ipv4.conf.default.rp_filter=1
net.ipv4.tcp_syncookies=1
kernel.kptr_restrict=2
fs.protected_hardlinks=1
fs.protected_symlinks=1
SYSCTL
if [ ! -f "$SYSCTL_FILE" ] || ! cmp -s "$TMP_SYSCTL" "$SYSCTL_FILE"; then
  install -m 0644 "$TMP_SYSCTL" "$SYSCTL_FILE"
  sysctl -p "$SYSCTL_FILE" >/dev/null
fi
rm -f "$TMP_SYSCTL"

systemctl enable --now unattended-upgrades

if [ "%s" = "true" ]; then
  ufw --force default deny incoming >/dev/null
  ufw --force default allow outgoing >/dev/null
  for PORT in %s; do
    if ! ufw status | grep -Eq "^${PORT}/tcp[[:space:]]+ALLOW"; then
      ufw allow "${PORT}/tcp" >/dev/null
    fi
  done
  ufw --force enable >/dev/null
fi
`, passwordAuth, enableUFW, allowedPorts)

	return runRemoteScript(ctx, logger, cfg, "os_hardening", script)
}
