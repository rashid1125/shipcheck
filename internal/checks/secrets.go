package checks

// HEURISTIC CREDENTIAL SCAN
//
// Limitations:
//   - Pattern-based only; will have false positives and false negatives.
//   - A clean result does NOT mean the project is free of secrets.
//   - Matched text is NEVER included in findings — only file and line number.
//   - Skipped: .env* files, lockfiles, binary files, .git/, dependency/vendor
//     dirs, build outputs, test files, markdown, minified JS, files > 1 MB.

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rashid1125/shipcheck/internal/config"
	"github.com/rashid1125/shipcheck/internal/finding"
)

const maxSecretScanFileSize = 1 << 20 // 1 MB

// pemPattern matches PEM private key headers — treated as critical.
var pemPattern = regexp.MustCompile(`-----BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY`)

// credentialPatterns match likely hard-coded secrets (produce FAIL/high).
var credentialPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(password|passwd|pwd|secret|token|api_key|apikey|access_key|secret_key|private_key|auth_token)\s*[:=]\s*["']?[A-Za-z0-9_\-+/]{8,}`),
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	regexp.MustCompile(`(?i)(key|token|secret)\s*[:=]\s*["']?[0-9a-f]{32,}["']?`),
	regexp.MustCompile(`ghp_[A-Za-z0-9]{36}`),
}

// publicKeyPatterns match patterns that look like secrets but are typically
// intentionally public (produce WARN/low with an explanatory message).
var publicKeyPatterns = []*regexp.Regexp{
	// Firebase web apiKey (always bundled in frontend code)
	regexp.MustCompile(`(?i)apiKey\s*:\s*["'][A-Za-z0-9_\-]{20,}["']`),
	// VAPID public key reference
	regexp.MustCompile(`(?i)vapid[_\-]?(public[_\-]?)?key\s*[:=]\s*["']?[A-Za-z0-9_\-+/=]{40,}`),
}

// skipDirs lists directories that are never scanned.
var skipDirs = map[string]bool{
	".git":             true,
	"node_modules":     true,
	"vendor":           true,
	".vendor":          true,
	"bower_components": true,
	"dist":             true,
	"build":            true,
	"out":              true,
	".next":            true,
	".nuxt":            true,
	"__pycache__":      true,
	".venv":            true,
	"venv":             true,
	"env":              true,
	"target":           true,
	".gradle":          true,
	"coverage":         true,
	"__tests__":        true,
}

// skipPathComponents are path segment names that cause the file to be skipped.
var skipPathComponents = map[string]bool{
	"testdata":  true,
	"__tests__": true,
}

// skipFiles lists exact filenames that are never scanned.
var skipFiles = map[string]bool{
	"package-lock.json": true,
	"yarn.lock":         true,
	"pnpm-lock.yaml":    true,
	"composer.lock":     true,
	"poetry.lock":       true,
	"Cargo.lock":        true,
	"go.sum":            true,
	"Gemfile.lock":      true,
	"Pipfile.lock":      true,
}

// skipExtensions lists file extensions that are never scanned.
var skipExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".ico": true,
	".svg": true, ".woff": true, ".woff2": true, ".ttf": true, ".eot": true,
	".mp4": true, ".mp3": true, ".pdf": true, ".zip": true, ".tar": true,
	".gz": true, ".br": true, ".exe": true, ".dll": true, ".so": true,
	".dylib": true, ".bin": true, ".o": true, ".a": true, ".pyc": true,
	".md": true,
}

// checkHardcodedSecrets scans source files for likely credential patterns.
func checkHardcodedSecrets(ctx Context) []finding.Finding {
	const id = "HARDCODED_SECRETS"

	findings, err := scanForSecrets(ctx.Root, ctx.Config)
	if err != nil {
		return []finding.Finding{warn(id, finding.SeverityMedium,
			"Secret scan encountered an error: "+err.Error(),
			"Check file permissions in the project directory.")}
	}

	if len(findings) == 0 {
		return []finding.Finding{{
			RuleID:   id,
			Severity: finding.SeverityInfo,
			Status:   finding.StatusPass,
			Message: "HEURISTIC: No obvious credential patterns found in source files. " +
				"This does not guarantee the absence of secrets.",
			Recommendation: "Use a dedicated secret-scanning tool (gitleaks, trufflehog) for thorough analysis.",
		}}
	}
	return findings
}

func scanForSecrets(root string, cfg *config.Config) ([]finding.Finding, error) {
	const id = "HARDCODED_SECRETS"
	var findings []finding.Finding

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}

		name := d.Name()

		if d.IsDir() {
			if name == "." {
				return nil
			}
			if skipDirs[name] || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip symlinks.
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}

		// Skip .env* files.
		if strings.HasPrefix(name, ".env") {
			return nil
		}

		// Skip test / spec files.
		if isTestFile(name) {
			return nil
		}

		// Skip lockfiles and binary/doc extensions.
		if skipFiles[name] {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(name))
		if skipExtensions[ext] {
			return nil
		}
		// Skip *.min.js
		if strings.HasSuffix(strings.ToLower(name), ".min.js") {
			return nil
		}

		// Compute relative path; skip files under skipped path components.
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		if hasSkippedComponent(rel) {
			return nil
		}

		// Check config ignore_paths.
		if cfg != nil && cfg.IsIgnoredPath(rel) {
			return nil
		}

		// Skip large files.
		info, err := d.Info()
		if err != nil || info.Size() > maxSecretScanFileSize {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		if stat, err := f.Stat(); err != nil || !stat.Mode().IsRegular() {
			return nil
		}

		lineNum := 0
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 256*1024), 256*1024)
		isBinaryChecked := false

		for scanner.Scan() {
			lineNum++
			line := scanner.Bytes()

			if !isBinaryChecked {
				isBinaryChecked = true
				if isBinaryLine(line) {
					return nil
				}
			}

			lineStr := string(line)

			// Inline suppression: either tool name accepted.
			if strings.Contains(lineStr, "shipcheck:ignore HARDCODED_SECRETS") ||
				strings.Contains(lineStr, "launchsafe:ignore HARDCODED_SECRETS") {
				continue
			}

			// PEM private key — critical, report file only (no line content).
			if pemPattern.MatchString(lineStr) {
				findings = append(findings, finding.Finding{
					RuleID:   id,
					Severity: finding.SeverityCritical,
					Status:   finding.StatusFail,
					Message:  "Private key file detected — remove from repository and revoke immediately.",
					FilePath: rel,
					Recommendation: "Delete this file from the repository, rotate the key, and add the pattern to .gitignore. " +
						"Check git history to ensure it was not previously committed.",
				})
				return nil // one finding per file for PEM keys
			}

			// Public-key patterns take priority — warn, not fail.
			// If matched, skip credential-pattern check for this line.
			var publicMatched bool
			for _, pat := range publicKeyPatterns {
				if pat.MatchString(lineStr) {
					publicMatched = true
					findings = append(findings, finding.Finding{
						RuleID:   id,
						Severity: finding.SeverityLow,
						Status:   finding.StatusWarn,
						Message:  "HEURISTIC (likely public by design — verify): credential-like pattern detected. File and line shown; matched text not displayed.",
						FilePath: rel,
						Line:     lineNum,
						Recommendation: "Confirm this value is intentionally public (e.g. Firebase web apiKey, VAPID public key). " +
							"If it is a private secret, move it to an environment variable.",
					})
					break
				}
			}

			if !publicMatched {
				// Credential patterns — fail.
				for _, pat := range credentialPatterns {
					if pat.MatchString(lineStr) {
						findings = append(findings, finding.Finding{
							RuleID:   id,
							Severity: finding.SeverityHigh,
							Status:   finding.StatusFail,
							Message:  "HEURISTIC: Possible hard-coded credential pattern detected. File and line shown; matched text not displayed.",
							FilePath: rel,
							Line:     lineNum,
							Recommendation: "Review this line and move any secrets to environment variables. " +
								"If this is a false positive, add a `shipcheck:ignore HARDCODED_SECRETS` comment or list the path in ignore_paths in .shipcheck.json.",
						})
						break
					}
				}
			}
		}
		return scanner.Err()
	})

	return findings, err
}

// isTestFile reports whether a filename is a test/spec file.
func isTestFile(name string) bool {
	lower := strings.ToLower(name)
	if strings.HasSuffix(name, "_test.go") {
		return true
	}
	for _, pat := range []string{".test.js", ".test.ts", ".test.jsx", ".test.tsx",
		".spec.js", ".spec.ts", ".spec.jsx", ".spec.tsx"} {
		if strings.HasSuffix(lower, pat) {
			return true
		}
	}
	return false
}

// hasSkippedComponent reports whether any path segment is a skippable component.
func hasSkippedComponent(relPath string) bool {
	for _, part := range strings.Split(filepath.ToSlash(relPath), "/") {
		if skipPathComponents[part] {
			return true
		}
	}
	return false
}

func isBinaryLine(data []byte) bool {
	for _, b := range data {
		if b == 0 {
			return true
		}
	}
	return false
}
