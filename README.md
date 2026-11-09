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

## Install / build

**Requirements:** Go 1.22+

```bash
git clone https://github.com/rashid1125/launchsafe.git
cd launchsafe
go build -o launchsafe .
./launchsafe version
```

To embed the version at build time:

```bash
go build -ldflags="-X github.com/rashid1125/launchsafe/cmd.Version=0.1.0" -o launchsafe .
```

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

## Development

```bash
go test ./...          # run all tests
go vet ./...           # static analysis
gofmt -w .             # format code
go build -o launchsafe . # build binary
```

## License

MIT — see [LICENSE](LICENSE).
