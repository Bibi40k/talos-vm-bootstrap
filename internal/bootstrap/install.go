package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Bibi40k/talos-docker-bootstrap/internal/config"
	"github.com/Bibi40k/talos-docker-bootstrap/internal/ssh"
)

var sshRunScriptFn = ssh.RunScript
var sshRunCommandFn = ssh.RunCommand

func runDockerInstall(ctx context.Context, logger *slog.Logger, cfg config.Config) error {
	script := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

TARGET_DOCKER_VERSION="%s"
TARGET_USER="%s"

if command -v docker >/dev/null 2>&1; then
  CURRENT="$(docker --version | sed -n 's/^Docker version \([^,]*\),.*/\1/p')"
  if [ "${CURRENT}" = "${TARGET_DOCKER_VERSION}" ]; then
    systemctl enable --now docker >/dev/null
    if ! id -nG "${TARGET_USER}" | tr ' ' '\n' | grep -qx docker; then
      usermod -aG docker "${TARGET_USER}"
      echo "Added ${TARGET_USER} to docker group."
    fi
    echo "Docker already at target version: ${CURRENT}"
    exit 0
  fi
fi

apt-get update -y
apt-get install -y --no-install-recommends ca-certificates curl gnupg lsb-release

install -d -m 0755 /etc/apt/keyrings
if [ ! -f /etc/apt/keyrings/docker.gpg ]; then
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
fi
chmod a+r /etc/apt/keyrings/docker.gpg

. /etc/os-release
ARCH="$(dpkg --print-architecture)"
REPO_LINE="deb [arch=${ARCH} signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu ${VERSION_CODENAME} stable"
if [ ! -f /etc/apt/sources.list.d/docker.list ] || ! grep -Fqx "${REPO_LINE}" /etc/apt/sources.list.d/docker.list; then
  printf '%%s\n' "${REPO_LINE}" > /etc/apt/sources.list.d/docker.list
fi

apt-get update -y
DOCKER_PKG_VER="$(apt-cache madison docker-ce | awk '{print $3}' | grep "^5:${TARGET_DOCKER_VERSION}" | head -n1 || true)"
if [ -z "${DOCKER_PKG_VER}" ]; then
  echo "Requested Docker version not found in Docker APT repo: ${TARGET_DOCKER_VERSION}" >&2
  exit 1
fi

apt-get install -y --no-install-recommends \
  docker-ce="${DOCKER_PKG_VER}" \
  docker-ce-cli="${DOCKER_PKG_VER}" \
  containerd.io \
  docker-buildx-plugin \
  docker-compose-plugin

apt-mark hold docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin >/dev/null || true
systemctl enable --now docker >/dev/null

if ! id -nG "${TARGET_USER}" | tr ' ' '\n' | grep -qx docker; then
  usermod -aG docker "${TARGET_USER}"
  echo "Added ${TARGET_USER} to docker group."
fi

INSTALLED="$(docker --version | sed -n 's/^Docker version \([^,]*\),.*/\1/p')"
if [ "${INSTALLED}" != "${TARGET_DOCKER_VERSION}" ]; then
  echo "Docker version mismatch after install (got ${INSTALLED}, expected ${TARGET_DOCKER_VERSION})" >&2
  exit 1
fi
`, cfg.Docker.Version, cfg.VM.User)

	return runRemoteScript(ctx, logger, cfg, "docker_install", script)
}

func runTalosctlInstall(ctx context.Context, logger *slog.Logger, cfg config.Config) error {
	script := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail

TARGET_TALOS_VERSION="%s"
TARGET_SHA256="%s"
ARCH="$(dpkg --print-architecture)"
case "${ARCH}" in
  amd64) BIN="talosctl-linux-amd64" ;;
  arm64) BIN="talosctl-linux-arm64" ;;
  *)
    echo "Unsupported architecture for talosctl: ${ARCH}" >&2
    exit 1
    ;;
esac
URL="https://github.com/siderolabs/talos/releases/download/v${TARGET_TALOS_VERSION}/${BIN}"

current_talos_version() {
  # Extract first semantic version token from talosctl output, regardless of line format.
  talosctl version --client 2>/dev/null | grep -Eo 'v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z]+)?' | head -n1 | sed 's/^v//' || true
}

if command -v talosctl >/dev/null 2>&1; then
  CURRENT="$(current_talos_version)"
  if [ "${CURRENT}" = "${TARGET_TALOS_VERSION}" ]; then
    echo "talosctl already at target version: ${CURRENT}"
    exit 0
  fi
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT
BIN_PATH="${TMP_DIR}/talosctl"

if ! curl -fsSL "${URL}" -o "${BIN_PATH}"; then
  echo "Failed to download talosctl binary: ${URL}" >&2
  exit 1
fi
if ! echo "${TARGET_SHA256}  ${BIN_PATH}" | sha256sum -c - >/dev/null; then
  echo "talosctl checksum verification failed for version ${TARGET_TALOS_VERSION} (${ARCH})" >&2
  exit 1
fi
install -m 0755 "${BIN_PATH}" /usr/local/bin/talosctl

INSTALLED="$(current_talos_version)"
if [ "${INSTALLED}" != "${TARGET_TALOS_VERSION}" ]; then
  echo "talosctl version mismatch after install (got ${INSTALLED}, expected ${TARGET_TALOS_VERSION})" >&2
  exit 1
fi
`, cfg.Talos.Version, strings.ToLower(cfg.Talos.SHA256Checksum))

	return runRemoteScript(ctx, logger, cfg, "talosctl_install", script)
}

func runRemoteScript(ctx context.Context, logger *slog.Logger, cfg config.Config, stepName, script string) error {
	sshCfg := ssh.ExecConfig{
		Host:                  cfg.VM.Host,
		Port:                  cfg.VM.Port,
		User:                  cfg.VM.User,
		PrivateKeyPath:        cfg.VM.SSHPrivateKey,
		KnownHostsFile:        cfg.VM.KnownHostsFile,
		KnownHostsMode:        cfg.VM.KnownHostsMode,
		ExpectedHostKeySHA256: cfg.VM.SSHHostFingerprint,
		Prompt:                knownHostsPromptFn,
		ConnectTimeout:        cfg.Timeouts.SSHConnectDuration(),
	}

	stdout, stderr, err := sshRunScriptFn(ctx, sshCfg, script)
	if stdout != "" {
		logger.Debug(stepName+" stdout", "output", strings.TrimSpace(stdout))
	}
	if stderr != "" {
		logger.Debug(stepName+" stderr", "output", strings.TrimSpace(stderr))
	}
	if err != nil {
		return err
	}
	return nil
}
