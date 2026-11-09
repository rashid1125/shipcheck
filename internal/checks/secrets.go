package checks

// HEURISTIC CREDENTIAL SCAN
//
// This check uses regular expressions to look for common patterns that suggest
// hard-coded credentials in source files.
//
// IMPORTANT LIMITATIONS:
//   - This is a heuristic scan; it will produce both false positives and false
//     negatives. A clean result does not mean the project is free of secrets.
//   - Matched text is NEVER included in findings — only file and line number.
//   - The following are always skipped: .env* files, lockfiles, binary files,
//     .git/, dependency/vendor directories, build outputs, files > 1 MB.

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rashid1125/launchsafe/internal/finding"
)

const maxSecretScanFileSize = 1 << 20 // 1 MB

// credentialPatterns lists regexes that suggest hard-coded credentials.
// They match the variable-name side, not the value — values are never captured.
var credentialPatterns = []*regexp.Regexp{
	// Generic key=value patterns for common secret variable names.
	regexp.MustCompile(`(?i)(password|passwd|pwd|secret|token|api_key|apikey|access_key|secret_key|private_key|auth_token)\s*[:=]\s*["']?[A-Za-z0-9_\-+/]{8,}`),
	// AWS Access Key ID format.
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	// Generic high-entropy hex or base64 tokens (≥32 chars) in assignments.
	regexp.MustCompile(`(?i)(key|token|secret)\s*[:=]\s*["']?[0-9a-f]{32,}["']?`),
	// GitHub personal access tokens.
	regexp.MustCompile(`ghp_[A-Za-z0-9]{36}`),
	// PEM private key header.
	regexp.MustCompile(`-----BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY`),
}

// skipDirs are directories that should never be scanned.
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
	"env":              true, // Python virtual env directory (not .env file)
	"target":           true, // Rust/Java build output
	".gradle":          true,
	"coverage":         true,
}

// skipFiles are exact filenames that should never be scanned.
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

// skipExtensions are file extensions that should never be scanned.
var skipExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".ico": true,
	".svg": true, ".woff": true, ".woff2": true, ".ttf": true, ".eot": true,
	".mp4": true, ".mp3": true, ".pdf": true, ".zip": true, ".tar": true,
	".gz": true, ".br": true, ".exe": true, ".dll": true, ".so": true,
	".dylib": true, ".bin": true, ".o": true, ".a": true, ".pyc": true,
}

// checkHardcodedSecrets scans source files for likely credential patterns.
func checkHardcodedSecrets(ctx Context) []finding.Finding {
	const id = "HARDCODED_SECRETS"

	findings, err := scanForSecrets(ctx.Root)
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
			Recommendation: "Review findings manually and use a dedicated secret-scanning tool (e.g. truffleHog, gitleaks) for thorough analysis.",
		}}
	}
	return findings
}

func scanForSecrets(root string) ([]finding.Finding, error) {
	const id = "HARDCODED_SECRETS"
	var findings []finding.Finding

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // skip unreadable entries silently
		}

		name := d.Name()

		// Skip hidden and known-skippable directories.
		if d.IsDir() {
			if name == "." {
				return nil
			}
			if skipDirs[name] || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip symlinks — they may point to directories or external mounts.
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}

		// Skip .env* files — they legitimately contain secrets.
		if strings.HasPrefix(name, ".env") {
			return nil
		}

		// Skip known lockfiles and binary extensions.
		if skipFiles[name] {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(name))
		if skipExtensions[ext] {
			return nil
		}

		// Skip large files.
		info, err := d.Info()
		if err != nil || info.Size() > maxSecretScanFileSize {
			return nil
		}

		// Open and scan line by line.
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		// Guard against symlinks-to-directories or other non-regular files.
		if stat, err := f.Stat(); err != nil || !stat.Mode().IsRegular() {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}

		lineNum := 0
		scanner := bufio.NewScanner(f)
		// Increase buffer to handle minified files with very long lines.
		scanner.Buffer(make([]byte, 256*1024), 256*1024)
		isBinaryChecked := false
		for scanner.Scan() {
			lineNum++
			line := scanner.Bytes()

			// Check for binary on first line.
			if !isBinaryChecked {
				isBinaryChecked = true
				if isBinaryLine(line) {
					return nil
				}
			}

			lineStr := string(line)
			for _, pat := range credentialPatterns {
				if pat.MatchString(lineStr) {
					findings = append(findings, finding.Finding{
						RuleID:   id,
						Severity: finding.SeverityHigh,
						Status:   finding.StatusFail,
						Message: "HEURISTIC: Possible hard-coded credential pattern detected. " +
							"File and line shown; the matched text is not displayed.",
						FilePath: rel,
						Line:     lineNum,
						Recommendation: "Review this line and move any secrets to environment variables. " +
							"If this is a false positive, consider adding the rule to disabled_rules in .launchsafe.json.",
					})
					break // one finding per line is enough
				}
			}
		}
		return scanner.Err()
	})

	return findings, err
}

func isBinaryLine(data []byte) bool {
	for _, b := range data {
		if b == 0 {
			return true
		}
	}
	return false
}
