# Release Notes

## Unreleased

Highlights:
- TBD

Notes:
- TBD

## v0.1.2 (2026-03-01)

Highlights:
- Bumped `cli-wizard-core` dependency from `v0.1.0` to `v0.1.1`.

Notes:
- Pulls latest shared wizard core release used by config draft/session flows.

## v0.1.1 (2026-03-01)

Highlights:
- Config manager draft lifecycle now uses `cli-wizard-core` session primitives for consistent behavior across repositories.
- Stage2 draft handling was simplified and aligned with shared wizard semantics.

Notes:
- Replaces custom ad-hoc draft flow in `internal/cli/config_manager.go` with `wizard.Session` integration.
- Improves reuse and keeps future wizard behavior changes centralized in `cli-wizard-core`.

## v0.1.0 (2026-02-26)

Highlights:

- Enterprise-grade CLI scaffold with `bootstrap`, `vm-deploy`, and `provision-and-bootstrap`.
- Idempotent Talos bootstrap flow: OS hardening, pinned Docker install, pinned talosctl install, single-node Talos-in-Docker create.
- Integration via `vmware-vm-bootstrap` module pin (config manager entrypoint + VM deploy + workflow integration).
- SSH host trust hardening: strict/prompt/accept-new/auto-refresh modes, fingerprint pinning, stability refresh for fresh VM workflows.
- Workflow UX polish: human progress steps, long-step heartbeat, better error messages and recovery hints.
- Release pipeline hardening: GitHub release publish with retry/backoff, deterministic assets sync from pinned `vmbootstrap`.

Notes:

- Talos-in-Docker create enforces single-node topology (`--workers 0`) by default.
- `kubeconfig-export` can regenerate remote kubeconfig when cluster state exists.
- Core coverage is tracked with `make test-cover`; current values should be read from the generated report.
