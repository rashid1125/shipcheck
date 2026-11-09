# LaunchSafe

A local, read-only deployment-readiness CLI for developers. Run it before you deploy to catch common configuration issues before they cause production problems.

```
$ launchsafe check --path ./myapp --advanced
LaunchSafe 0.1.0 — Deployment Readiness Check
Path: /home/user/myapp
Mode: advanced

Detected stacks:
  • node (nextjs) [package.json]

Findings:
  ✓ PASS   [MANIFEST_PRESENT        ] Project manifest found: node
  ✓ PASS   [START_COMMAND           ] Node.js start script found: next start
  ✗ FAIL   [GITIGNORE_ENV           ] .env file exists but is not excluded by .gitignore.
             → Add `.env` or `.env*` to your .gitignore immediately to prevent committing secrets.
  ⚠ WARN   [DOCKERFILE_PRESENT      ] No Dockerfile found.
  ...

Summary: 8 passed, 3 warnings, 1 failed
```

## Safety model

- **Local and read-only by default.** LaunchSafe only reads files; it never writes, executes, or modifies your project.
- **No network requests.** All checks are offline.
- **Secrets are never displayed.** The heuristic credential scan reports a file and line number only; the matched text is never shown.
- **Heuristics are labelled.** Any check that uses pattern matching is clearly labelled as heuristic, with its limitations documented.
- **Missing files are warnings, not failures.** A missing Dockerfile or CI config is a warning because these are optional for many deployment setups.

## Install

### Option 1 — `go install` (easiest for Go developers)

```bash
go install github.com/rashid1125/launchsafe@latest
launchsafe version
```

> Requires Go 1.22+. The binary lands in `$(go env GOPATH)/bin` (usually `~/go/bin`).

---

### Option 2 — Download pre-built binary (no Go needed)

Go to the [Releases page](https://github.com/rashid1125/launchsafe/releases) and download
the binary for your platform:

| Platform | File |
|----------|------|
| Linux (Intel/AMD 64-bit) | `launchsafe-linux-amd64` |
| Linux (ARM64 / Graviton) | `launchsafe-linux-arm64` |
| macOS (Intel) | `launchsafe-darwin-amd64` |
| macOS (Apple Silicon M1/M2) | `launchsafe-darwin-arm64` |
| Windows (64-bit) | `launchsafe-windows-amd64.exe` |

```bash
# Linux / macOS
chmod +x launchsafe-linux-amd64
sudo mv launchsafe-linux-amd64 /usr/local/bin/launchsafe
launchsafe version

# Windows — rename to launchsafe.exe and add to PATH
```

**One-liner install script (Linux / macOS):**

```bash
curl -fsSL https://raw.githubusercontent.com/rashid1125/launchsafe/main/install.sh | bash
```

**Verify checksum** (recommended):

```bash
sha256sum -c checksums.txt
```

---

### Option 3 — Docker

```bash
# Run against the current directory
docker run --rm -v "$(pwd):/project" ghcr.io/rashid1125/launchsafe check --path /project

# Advanced mode
docker run --rm -v "$(pwd):/project" ghcr.io/rashid1125/launchsafe check --path /project --advanced
```

---

### Option 4 — Build from source

```bash
git clone https://github.com/rashid1125/launchsafe.git
cd launchsafe
make build          # builds ./launchsafe
make install        # installs to ~/go/bin
make cross          # builds all platforms into ./dist/
```

Or without Make:

```bash
go build -ldflags="-X github.com/rashid1125/launchsafe/cmd.Version=0.1.0" -o launchsafe .
```

---

## Releasing a new version

1. Push all changes to `main`
2. Create and push a version tag:
   ```bash
   git tag v0.2.0
   git push origin v0.2.0
   ```
3. GitHub Actions automatically builds binaries for all platforms and publishes a GitHub Release.

## Usage

```
launchsafe <command> [flags]

Commands:
  check     Run deployment-readiness checks on a project directory
  version   Print the launchsafe version
  help      Show this help message

Flags for 'check':
  --path DIR       Project directory to check (default: current directory)
  --advanced       Enable advanced checks
  --strict         Treat warnings as failures (exit code 1)
  --format FORMAT  Output format: text (default), json, sarif
  --config FILE    Load custom rules from a .launchsafe.json file
```

### Examples

```bash
# Check the current directory (basic mode)
launchsafe check

# Check a specific directory with advanced checks
launchsafe check --path ./myapp --advanced

# Machine-readable JSON output for CI integration
launchsafe check --advanced --format json > report.json

# SARIF output for GitHub Code Scanning or other SAST tools
launchsafe check --advanced --format sarif > results.sarif

# Strict mode: exit 1 if any warnings are found
launchsafe check --strict

# Use a custom config file
launchsafe check --config .launchsafe.json

# Check a project and fail the CI step on warnings or failures
launchsafe check --path . --advanced --strict --format sarif > launchsafe.sarif
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0    | All checks passed (or only warnings, without `--strict`) |
| 1    | One or more checks failed, OR warnings with `--strict` |
| 2    | Usage error (invalid command, flag, format, or config) |

## Supported stacks

Detection is based on the presence and content of manifest files. Multiple stacks in one repository are all detected and reported.

| Language | Detected by | Frameworks detected |
|----------|-------------|---------------------|
| Node.js  | `package.json` | Next.js, Express, Fastify, Nuxt, NestJS, React, Vue, Svelte, Vite |
| PHP      | `composer.json`, `artisan` | Laravel, Symfony |
| Python   | `pyproject.toml`, `requirements.txt`, `Pipfile` | Django, Flask, FastAPI |
| Go       | `go.mod` | — |
| Rust     | `Cargo.toml` | — |

## Basic vs Advanced checks

### Basic mode (always on)

| Rule ID | What it checks | Default severity |
|---------|----------------|-----------------|
| `MANIFEST_PRESENT` | At least one manifest file detected | medium (warn) |
| `MANIFEST_READABLE` | JSON manifests are valid JSON | high (fail) |
| `START_COMMAND` | A production entry point or start command exists | low (warn) |
| `ENV_TEMPLATE` | An `.env.example` / `.env.sample` template exists | low (warn) |
| `GITIGNORE_ENV` | `.env` files are excluded in `.gitignore` | medium/high |
| `DOCKERFILE_PRESENT` | A Dockerfile exists (optional for many setups) | info (warn) |
| `CI_PRESENT` | CI configuration exists (optional) | info (warn) |

### Advanced mode (`--advanced`)

All basic checks plus:

| Rule ID | What it checks | Default severity |
|---------|----------------|-----------------|
| `LOCKFILE_PRESENT` | A dependency lockfile exists | medium (warn) |
| `DOCKER_COMPOSE_PRESENT` | A Docker Compose file exists (optional) | info (warn) |
| `GITHUB_ACTIONS_PRESENT` | `.github/workflows/*.yml` files exist | info (warn) |
| `PUBLIC_ENV_EXPOSED` | `.env` files are not in publicly-served directories | critical (fail) |
| `DOCKERFILE_BASIC` | Dockerfile has FROM, EXPOSE, non-root USER | medium (warn) |
| `FRAMEWORK_CONFIG` | Framework-specific config checks (Laravel, Next.js, Django) | medium (warn) |
| `HARDCODED_SECRETS` | **Heuristic** scan for credential patterns in source files | high (fail) |

## Configuration file

Create `.launchsafe.json` in your project root (or specify with `--config`):

```json
{
  "disabled_rules": ["DOCKERFILE_PRESENT", "CI_PRESENT"],
  "severity_overrides": {
    "LOCKFILE_PRESENT": "high",
    "GITHUB_ACTIONS_PRESENT": "medium"
  }
}
```

**`disabled_rules`** — rule IDs to exclude from findings and summary counts.

**`severity_overrides`** — override the severity of a rule. Valid values: `info`, `low`, `medium`, `high`, `critical`.

LaunchSafe automatically looks for `.launchsafe.json` in the project directory. Use `--config` to point to a file elsewhere.

## Output formats

### text (default)

Human-readable, with icons (✓/⚠/✗), rule IDs, messages, and actionable recommendations.

### json

Stable JSON object with `version`, `path`, `mode`, `stacks`, `summary`, and `findings` arrays. Suitable for parsing in CI pipelines.

```json
{
  "version": "0.1.0",
  "path": "/path/to/project",
  "mode": "basic",
  "stacks": [{"language": "node", "framework": "nextjs", "manifest": "package.json"}],
  "summary": {"pass": 5, "warn": 2, "fail": 1},
  "findings": [
    {
      "rule_id": "GITIGNORE_ENV",
      "severity": "high",
      "status": "fail",
      "message": ".env file exists but is not excluded by .gitignore.",
      "file_path": ".gitignore",
      "recommendation": "Add `.env` or `.env*` to your .gitignore immediately."
    }
  ]
}
```

### sarif

Valid SARIF 2.1.0 output with rule metadata and physical locations. Upload to GitHub Code Scanning or any compatible SAST platform.

## Known limitations

- **Detection is top-level only.** Manifests in subdirectories are not scanned.
- **Heuristic secret scan has false positives and negatives.** It uses regex patterns, not entropy analysis or a full AST. A clean result does not prove the absence of secrets. Use a dedicated tool (truffleHog, gitleaks, git-secrets) for thorough analysis.
- **YAML files are not parsed.** GitHub Actions workflow checks only verify file existence, not content validity.
- **Dockerfile parsing is heuristic.** Multi-stage builds, ARG-based FROM, and complex Dockerfile syntax may not be handled correctly.
- **No Windows path testing.** The CI matrix includes Windows, but cross-platform edge cases in path handling may exist.
- **No network checks.** LaunchSafe does not verify that environment variables, external services, or DNS entries are correctly configured for production.

## Real-world example — TradingERP (Laravel + Vite + Next.js frontend)

LaunchSafe was tested against a real multi-stack project containing a Laravel backend
(`TradingERP/`) and a Next.js frontend (`frontend/`).

### Backend — Laravel + Vite (basic mode)

```
$ launchsafe check --path ./TradingERP

LaunchSafe 0.1.0 — Deployment Readiness Check
Path: /var/www/PHP-Project/TradingERP
Mode: basic

Detected stacks:
  • node (vite) [package.json]
  • php (laravel) [composer.json]

Findings:
  ✓ PASS   [MANIFEST_PRESENT        ] Project manifest found: node, php
  ✓ PASS   [MANIFEST_READABLE       ] package.json is valid JSON.
  ✓ PASS   [MANIFEST_READABLE       ] composer.json is valid JSON.
  ⚠ WARN   [START_COMMAND           ] No `scripts.start` found in package.json.
             → Add a `start` script to package.json or add a Procfile.
  ✓ PASS   [ENV_TEMPLATE            ] Environment template found: .env.example
  ✓ PASS   [GITIGNORE_ENV           ] .env is covered by .gitignore.
  ✓ PASS   [DOCKERFILE_PRESENT      ] Dockerfile found: Dockerfile
  ✓ PASS   [CI_PRESENT              ] CI configuration found: azure-pipelines.yml

Summary: 7 passed, 1 warnings, 0 failed
```

### Backend — Laravel + Vite (advanced mode — notable findings)

```
$ launchsafe check --path ./TradingERP --advanced

  ⚠ WARN   [DOCKERFILE_BASIC        ] Dockerfile does not contain an EXPOSE instruction.
             → Add an EXPOSE instruction to document the port your container listens on.
  ⚠ WARN   [DOCKERFILE_BASIC        ] Dockerfile does not set a non-root USER.
             → Add a USER instruction to run the container as a non-root user.
  ✓ PASS   [FRAMEWORK_CONFIG        ] Laravel .env.example contains APP_KEY entry.
  ✗ FAIL   [HARDCODED_SECRETS       ] HEURISTIC: Possible hard-coded credential pattern detected.
                                       [storage/oauth-private.key:1]
  ✗ FAIL   [HARDCODED_SECRETS       ] HEURISTIC: Possible hard-coded credential pattern detected.
                                       [resources/js/firebase-init.js:10]
  ✗ FAIL   [HARDCODED_SECRETS       ] HEURISTIC: Possible hard-coded credential pattern detected.
                                       [app/Http/Controllers/Developer/AuthController.php:96]
  ...

Summary: 11 passed, 4 warnings, 34 failed
```

> **Note:** `storage/oauth-private.key` is a real OAuth private key committed to the repo —
> this should be removed and regenerated. `firebase-init.js` may contain a Firebase API key
> that should be moved to an environment variable. Test files flagged are likely false positives
> using dummy credentials; add `HARDCODED_SECRETS` to `disabled_rules` or fix per file.

### Frontend — Next.js (basic mode — clean pass)

```
$ launchsafe check --path ./frontend

LaunchSafe 0.1.0 — Deployment Readiness Check
Path: /var/www/PHP-Project/frontend
Mode: basic

Detected stacks:
  • node (nextjs) [package.json]

Findings:
  ✓ PASS   [MANIFEST_PRESENT        ] Project manifest found: node
  ✓ PASS   [MANIFEST_READABLE       ] package.json is valid JSON.
  ✓ PASS   [START_COMMAND           ] Node.js start script found: next start
  ✓ PASS   [ENV_TEMPLATE            ] Environment template found: .env.example
  ✓ PASS   [GITIGNORE_ENV           ] .env is covered by .gitignore.
  ✓ PASS   [DOCKERFILE_PRESENT      ] Dockerfile found: Dockerfile
  ✓ PASS   [CI_PRESENT              ] CI configuration found: azure-pipelines.yml

Summary: 7 passed, 0 warnings, 0 failed
```

### Frontend — Next.js (advanced mode — secrets scan flagged issues)

```
$ launchsafe check --path ./frontend --advanced

  ✓ PASS   [LOCKFILE_PRESENT        ] Dependency lockfile found: package-lock.json
  ✓ PASS   [DOCKER_COMPOSE_PRESENT  ] Docker Compose file found: docker-compose.yml
  ✓ PASS   [DOCKERFILE_BASIC        ] Dockerfile passes basic checks.
  ✓ PASS   [FRAMEWORK_CONFIG        ] Next.js configuration file found.
  ✗ FAIL   [HARDCODED_SECRETS       ] HEURISTIC: Possible hard-coded credential pattern detected.
                                       [lib/config.js:28]
  ✗ FAIL   [HARDCODED_SECRETS       ] HEURISTIC: Possible hard-coded credential pattern detected.
                                       [lib/reverb.js:21]
  ✗ FAIL   [HARDCODED_SECRETS       ] HEURISTIC: Possible hard-coded credential pattern detected.
                                       [hooks/useBrowserPush.js:78]
  ...

Summary: 12 passed, 1 warnings, 24 failed
```

> **Note:** `lib/config.js` and `lib/reverb.js` contain token-like assignments worth reviewing.
> `hooks/useBrowserPush.js` may contain a Web Push public key (VAPID key) — these are
> intentionally public-facing but still flagged by the heuristic scanner. Review each flagged
> file; use `.launchsafe.json` to suppress confirmed false positives.

### Multi-stack detection in action

Both projects demonstrate that LaunchSafe correctly handles repositories with more than one
manifest (`node` + `php` in TradingERP) and reports all detected stacks in a deterministic,
sorted order.

## Development

```bash
go test ./...          # run all tests
go vet ./...           # static analysis
gofmt -w .             # format code
go build -o launchsafe . # build binary
```

## License

MIT — see [LICENSE](LICENSE).
