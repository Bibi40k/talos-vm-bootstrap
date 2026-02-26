# talos-vm-bootstrap

![CI](https://github.com/Bibi40k/talos-vm-bootstrap/actions/workflows/ci.yml/badge.svg)
![Coverage](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/Bibi40k/talos-vm-bootstrap/master/docs/coverage/coverage.json)
[![Go Report Card](https://goreportcard.com/badge/github.com/Bibi40k/talos-vm-bootstrap)](https://goreportcard.com/report/github.com/Bibi40k/talos-vm-bootstrap)
![Go Version](https://img.shields.io/github/go-mod/go-version/Bibi40k/talos-vm-bootstrap)
![Release](https://img.shields.io/github/v/release/Bibi40k/talos-vm-bootstrap)

Enterprise-grade post-bootstrap for Ubuntu dev VMs.

## Scope

- Configure an existing Ubuntu VM for dev workflows
- Apply idempotent OS hardening baseline
- Install and verify Docker
- Install and verify talosctl
- Create Talos-in-Docker cluster idempotently (single-node controlplane by default)

## Status

Public alpha. First functional release baseline: `v0.1.0`.

## Quick Start

```bash
cp configs/talos-bootstrap.example.yaml configs/talos-bootstrap.yaml
# edit configs/talos-bootstrap.yaml (interactive wizard can auto-fill checksum)

make vm-deploy              # VM bootstrap via vmbootstrap
make talos-bootstrap-dry    # Talos bootstrap dry-run
make talos-bootstrap        # Talos bootstrap apply
```

Note: `talos.sha256_checksum` must match the VM architecture (`amd64` or `arm64`), since the installer downloads the corresponding `talosctl` binary.

## SSH Host Key Verification

By default, the bootstrap runs in **strict** mode and refuses to connect if the SSH host key changes.
You can control this behavior with:

- `vm.known_hosts_mode`: `strict` | `prompt` | `accept-new` | `auto-refresh`
- `vm.ssh_host_fingerprint`: optional `SHA256:...` fingerprint to pin the expected host key

Best practice for automation is to pass the fingerprint produced by `vmbootstrap` bootstrap output, so the host key is verified without interactive prompts.

## CLI

```bash
talos-vm-bootstrap vm-deploy
talos-vm-bootstrap bootstrap --config configs/talos-bootstrap.yaml [--dry-run] [--json]
talos-vm-bootstrap cluster-status --config configs/talos-bootstrap.yaml
talos-vm-bootstrap mount-check --config configs/talos-bootstrap.yaml
talos-vm-bootstrap kubeconfig-export --config configs/talos-bootstrap.yaml --out build/devvm/kubeconfig
talos-vm-bootstrap provision-and-bootstrap --config configs/talos-bootstrap.yaml --bootstrap-result bootstrap-result.yaml [--vm-config configs/vm.example.yaml]
```

## Quality Gates

- `go test ./...`
- `go vet ./...`
- `golangci-lint`
- `make test-cover` (core logic coverage report for `internal/bootstrap`, `internal/config`, `internal/workflow`)

Coverage badge reflects **full project coverage** (including CLI). Core coverage report helps track production logic trends over time.
