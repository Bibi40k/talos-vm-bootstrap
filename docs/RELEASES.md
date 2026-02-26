# Release Notes

## Unreleased

Highlights:
- TBD

Notes:
- TBD

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
