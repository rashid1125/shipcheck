# Security Policy

## Supported versions

| Version | Supported |
|---------|-----------|
| latest  | Yes       |
| older   | No        |

Please upgrade to the latest release before reporting an issue.

## Reporting a vulnerability

Email **rashidrasheed1125@gmail.com** with:

- A description of the vulnerability
- Steps to reproduce
- Potential impact

Please do not open a public GitHub issue for security vulnerabilities. You should receive a response within 72 hours.

## What ShipCheck does NOT do

- **No network requests.** ShipCheck never contacts any external server. It reads local files only.
- **No code execution.** ShipCheck never runs project scripts, installs dependencies, or spawns child processes.
- **No file writes.** ShipCheck is strictly read-only; it never modifies your project.
- **No secret display.** The `HARDCODED_SECRETS` check reports file paths and line numbers only; it never includes matched credential text in output.

## Limitations of the credential scan

`HARDCODED_SECRETS` is a heuristic, pattern-based scan. It will produce false positives and false negatives. A clean result does not mean your project is free of hard-coded secrets. For thorough secret scanning, use [gitleaks](https://github.com/gitleaks/gitleaks) or [trufflehog](https://github.com/trufflesecurity/trufflehog).
