![CI](https://github.com/rashid1125/shipcheck/actions/workflows/ci.yml/badge.svg)
![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)
![Go version](https://img.shields.io/badge/Go-1.22+-00ADD8.svg)

# ShipCheck

Offline deployment-readiness preflight for any project — checks env, gitignore, Docker, CI and credential patterns in one binary, no network required.

## Quick start

```bash
go install github.com/rashid1125/shipcheck@latest
shipcheck check --path ./myapp
shipcheck check --path ./myapp --advanced --format json
```

## Sample output

Running against the included demo monorepo frontend:

```
$ shipcheck check --path testdata/demo-monorepo/frontend

ShipCheck 0.2.0 — Deployment Readiness Check
Path: /project/testdata/demo-monorepo/frontend
Mode: basic

Detected stacks:
  • node (nextjs) [package.json]

Findings:
  ✓ PASS   [MANIFEST_PRESENT        ] Project manifest found: node
  ✓ PASS   [MANIFEST_READABLE       ] package.json is valid JSON.
  ✓ PASS   [START_COMMAND           ] Node.js start script found: next start
  ✓ PASS   [ENV_TEMPLATE            ] Environment template found: .env.example
  ✗ FAIL   [GITIGNORE_ENV           ] .env file exists but does not appear to be excluded by .gitignore.
             → Add `.env` or `.env*` to your .gitignore immediately to prevent committing secrets.
  ⚠ WARN   [DOCKERFILE_PRESENT      ] No Dockerfile found.
  ⚠ WARN   [CI_PRESENT              ] No CI configuration found.

Summary: 4 passed, 2 warnings, 1 failed
```

## Install

### Option 1 — `go install` (easiest for Go developers)

```bash
go install github.com/rashid1125/shipcheck@latest
shipcheck version
```

> Requires Go 1.22+. Binary lands in `$(go env GOPATH)/bin` (usually `~/go/bin`).

### Option 2 — Download pre-built binary (no Go required)

Go to [Releases](https://github.com/rashid1125/shipcheck/releases) and pick your platform:

| Platform | File |
|----------|------|
| Linux amd64 | `shipcheck-linux-amd64` |
| Linux arm64 | `shipcheck-linux-arm64` |
| macOS Intel | `shipcheck-darwin-amd64` |
| macOS Apple Silicon | `shipcheck-darwin-arm64` |
| Windows 64-bit | `shipcheck-windows-amd64.exe` |

```bash
# Linux / macOS
chmod +x shipcheck-linux-amd64
sudo mv shipcheck-linux-amd64 /usr/local/bin/shipcheck
shipcheck version
```

One-liner install (Linux / macOS):

```bash
curl -fsSL https://raw.githubusercontent.com/rashid1125/shipcheck/main/install.sh | bash
```

The script downloads the binary, verifies the SHA-256 checksum, and exits non-zero if verification fails.

### Option 3 — Docker (GitHub Packages)

```bash
docker pull ghcr.io/rashid1125/shipcheck:latest
docker run --rm -v "$(pwd):/project" ghcr.io/rashid1125/shipcheck:latest check --path /project --advanced
```

Multi-arch image: `linux/amd64` and `linux/arm64`.

### Option 4 — Build from source

```bash
git clone https://github.com/rashid1125/shipcheck.git
cd shipcheck
make build    # builds ./shipcheck
make install  # installs to ~/go/bin
make cross    # builds all platforms into ./dist/
```

## Releasing a new version

```bash
git tag v0.2.0
git push origin v0.2.0
```

GitHub Actions builds binaries for all five platforms, generates `checksums.txt`, creates a GitHub Release, and pushes a multi-arch Docker image to `ghcr.io`.

## Usage

```
shipcheck <command> [flags]

Commands:
  check     Run deployment-readiness checks on a project directory
  version   Print the shipcheck version
  help      Show this help message

Flags for 'check':
  --path DIR       Project directory to check (default: current directory)
  --depth N        Scan subdirectories up to N levels deep (0 = top-level only)
  --advanced       Enable advanced checks (lockfiles, secrets scan, framework config, etc.)
  --strict         Treat warnings as failures; return exit code 1
  --format FORMAT  Output format: text (default), json, sarif
  --config FILE    Load rules from a .shipcheck.json config file
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | All checks passed (or warnings only, without `--strict`) |
| 1 | One or more checks failed, or warnings with `--strict` |
| 2 | Usage error (invalid command, flag, format, or config) |

## Supported stacks

Detection is based on manifest files in the scanned directory. All detected stacks are reported in a deterministic, sorted order.

| Language | Detected by | Frameworks detected |
|----------|-------------|---------------------|
| Node.js | `package.json` | Next.js, Express, Fastify, Nuxt, NestJS, React, Vue, Svelte, Vite |
| PHP | `composer.json`, `artisan` | Laravel, Symfony |
| Python | `pyproject.toml`, `requirements.txt`, `Pipfile` | Django, Flask, FastAPI |
| Go | `go.mod` | — |
| Rust | `Cargo.toml` | — |

## Basic vs Advanced checks

### Basic mode (always on)

| Rule ID | What it checks | Default severity |
|---------|----------------|-----------------|
| `MANIFEST_PRESENT` | At least one manifest detected | medium (warn) |
| `MANIFEST_READABLE` | JSON manifests are valid JSON | high (fail) |
| `START_COMMAND` | A production entry point exists | low (warn) |
| `ENV_TEMPLATE` | An `.env.example` / `.env.sample` exists | low (warn) |
| `GITIGNORE_ENV` | `.env` files excluded in `.gitignore` | medium/high |
| `DOCKERFILE_PRESENT` | A Dockerfile exists (optional for many setups) | info (warn) |
| `CI_PRESENT` | CI configuration exists (optional) | info (warn) |

### Advanced mode (`--advanced`)

All basic checks plus:

| Rule ID | What it checks | Default severity |
|---------|----------------|-----------------|
| `LOCKFILE_PRESENT` | A dependency lockfile exists | medium (warn) |
| `DOCKER_COMPOSE_PRESENT` | A Docker Compose file exists | info (warn) |
| `GITHUB_ACTIONS_PRESENT` | `.github/workflows/*.yml` exist | info (warn) |
| `PUBLIC_ENV_EXPOSED` | `.env` files not in publicly-served directories | critical (fail) |
| `DOCKERFILE_BASIC` | Dockerfile has FROM, EXPOSE, non-root USER | medium (warn) |
| `FRAMEWORK_CONFIG` | Framework-specific checks (Laravel, Next.js, Django) | medium (warn) |
| `HARDCODED_SECRETS` | **Heuristic** scan for credential patterns | high/critical (fail) |

## Monorepo / subdirectory scanning

Use `--depth N` to scan sub-directories up to N levels deep:

```bash
shipcheck check --path ./my-monorepo --depth 3 --advanced
```

Each finding includes a `sub_project` field (relative path) in JSON and SARIF output, and is grouped visually in text output:

```
  ── sub-project: backend ──
  ✓ PASS   [MANIFEST_PRESENT        ] ...

  ── sub-project: frontend ──
  ✗ FAIL   [GITIGNORE_ENV           ] ...
```

Sub-directories named `node_modules`, `vendor`, `.git`, `dist`, `build`, `.next`, `target`, `__pycache__`, `venv`, `.venv` are always skipped.

## Configuration file (.shipcheck.json)

Create `.shipcheck.json` in your project root (or pass `--config`):

```json
{
  "disabled_rules": ["DOCKERFILE_PRESENT", "CI_PRESENT"],
  "severity_overrides": {
    "LOCKFILE_PRESENT": "high"
  },
  "ignore_paths": ["docs/", "scripts/fixtures/"]
}
```

- **`disabled_rules`** — exclude rules from findings and summary counts.
- **`severity_overrides`** — override a rule's severity. Valid values: `info`, `low`, `medium`, `high`, `critical`.
- **`ignore_paths`** — path prefixes (relative to project root) excluded from the `HARDCODED_SECRETS` scan.

`.launchsafe.json` is still accepted as a deprecated fallback (a notice is printed to stderr).

### Inline suppression

Add a comment on any source line to suppress secret-scan findings for that line:

```js
const vapidKey = "BM3..."; // shipcheck:ignore HARDCODED_SECRETS
```

## Output formats

### text (default)

Human-readable with icons (✓/⚠/✗), rule IDs, messages, and actionable recommendations.

### json

Stable JSON including `version`, `path`, `mode`, `scan_depth`, `stacks`, `summary`, and `findings`. Each finding may carry a `sub_project` field.

### sarif

Valid SARIF 2.1.0 with rule metadata and physical locations. Upload to GitHub Code Scanning or any SARIF-compatible SAST platform.

## How ShipCheck differs from gitleaks / trufflehog

ShipCheck's credential scan (`HARDCODED_SECRETS`) is a lightweight heuristic pass — it catches obvious patterns quickly as part of a broader deployment preflight that also checks gitignore hygiene, env templates, Docker, CI, and framework config. It is not a replacement for dedicated secret scanners.

For thorough secret scanning with git-history traversal, high-accuracy rules, and team-workflow features (e.g. pre-commit hooks, CI integration, allowlists), use [gitleaks](https://github.com/gitleaks/gitleaks) or [trufflehog](https://github.com/trufflesecurity/trufflehog) in addition to ShipCheck. They are complementary, not competing tools.

## Known limitations

- **Monorepo depth**: `--depth` values larger than the actual directory structure are safe but add no value.
- **Heuristic secret scan has false positives and negatives.** A clean result does not prove the absence of secrets; use gitleaks/trufflehog for thorough analysis.
- **YAML files are not parsed.** GitHub Actions workflow checks only verify file existence.
- **Dockerfile parsing is heuristic.** Multi-stage builds and complex ARG-based patterns may not be handled correctly.
- **No network checks.** ShipCheck cannot verify DNS, external services, or cloud configuration.

## Development

```bash
go test ./...      # run all tests
go vet ./...       # static analysis
gofmt -w .         # format code
make build         # build binary
make cross         # build all platforms into ./dist/
```

## License

MIT — see [LICENSE](LICENSE).
