# Contributing to ShipCheck

## Build and test locally

```bash
git clone https://github.com/rashid1125/shipcheck.git
cd shipcheck
go build -o shipcheck .
go test ./...
go vet ./...
gofmt -w .
```

Requires Go 1.22+. No other tools or external dependencies.

## PR guidelines

- Every change must include tests. PRs without tests for new behaviour will not be merged.
- ShipCheck is **offline and read-only**. PRs that add network calls, file writes, or process execution will be rejected without exception.
- All new checks must be labelled as heuristic in their message text if they use pattern matching.
- Secret values must never appear in finding output — only file paths and line numbers.
- Run `gofmt -w .` before pushing; the CI gofmt check will fail otherwise.
- Keep external dependencies at zero. Use the Go standard library only.

## Adding a new check

1. Pick a stable, upper-snake-case rule ID (e.g. `MY_NEW_RULE`).
2. Implement a function `checkMyNewRule(ctx Context) []finding.Finding` in the appropriate file under `internal/checks/`.
3. Register it in `Run()` in `internal/checks/checks.go` (basic or advanced section).
4. Add at least: a pass case test, a fail/warn case test, and a disabled-rule test.
5. Document the rule ID, what it checks, and its default severity in `README.md` under "Basic vs Advanced checks".

## Code style

- `gofmt` — no exceptions.
- No external dependencies; standard library only.
- No comments explaining *what* the code does — only *why* when the reason is non-obvious.
- Keep packages small and focused: `detect` detects, `checks` checks, `output` formats.
