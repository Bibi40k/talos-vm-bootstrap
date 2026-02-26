# Contributing

## Development

- Use Go version from `go.mod`.
- Run:

```bash
make fmt
make vet
make test
```

## Rules

- Keep changes idempotent and fail-fast.
- No hardcoded environment-specific values.
- Add tests for logic and validation paths.
