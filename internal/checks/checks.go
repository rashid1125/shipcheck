// Package checks implements deployment-readiness checks.
// All checks are local and read-only; they never execute project code,
// install dependencies, or make network requests.
package checks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/rashid1125/launchsafe/internal/config"
	"github.com/rashid1125/launchsafe/internal/detect"
	"github.com/rashid1125/launchsafe/internal/finding"
)

// Context carries everything a check needs.
type Context struct {
	Root     string
	Stacks   []detect.Stack
	Advanced bool
	Config   *config.Config
}

// Run executes all applicable checks and returns the filtered findings.
// Disabled rules are excluded; severity overrides are applied.
func Run(ctx Context) []finding.Finding {
	var raw []finding.Finding

	// Basic checks always run.
	raw = append(raw, checkManifestPresent(ctx)...)
	raw = append(raw, checkManifestReadable(ctx)...)
	raw = append(raw, checkStartCommand(ctx)...)
	raw = append(raw, checkEnvTemplate(ctx)...)
	raw = append(raw, checkGitignoreEnv(ctx)...)
	raw = append(raw, checkDockerfilePresent(ctx)...)
	raw = append(raw, checkCIPresent(ctx)...)

	if ctx.Advanced {
		raw = append(raw, checkLockfilePresent(ctx)...)
		raw = append(raw, checkDockerComposePresent(ctx)...)
		raw = append(raw, checkGithubActionsPresent(ctx)...)
		raw = append(raw, checkPublicEnvExposed(ctx)...)
		raw = append(raw, checkDockerfileBasic(ctx)...)
		raw = append(raw, checkFrameworkConfig(ctx)...)
		raw = append(raw, checkHardcodedSecrets(ctx)...)
	}

	// Apply config: skip disabled rules, apply severity overrides.
	var out []finding.Finding
	for _, f := range raw {
		if ctx.Config.IsDisabled(f.RuleID) {
			continue
		}
		out = append(out, ctx.Config.Apply(f))
	}
	return out
}

// ─── helpers ────────────────────────────────────────────────────────────────

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func pass(ruleID, sev, msg, rec string) finding.Finding {
	return finding.Finding{RuleID: ruleID, Severity: sev, Status: finding.StatusPass, Message: msg, Recommendation: rec}
}

func warn(ruleID, sev, msg, rec string) finding.Finding {
	return finding.Finding{RuleID: ruleID, Severity: sev, Status: finding.StatusWarn, Message: msg, Recommendation: rec}
}

func fail(ruleID, sev, msg, rec string) finding.Finding {
	return finding.Finding{RuleID: ruleID, Severity: sev, Status: finding.StatusFail, Message: msg, Recommendation: rec}
}

func warnAt(ruleID, sev, msg, rec, fp string, line int) finding.Finding {
	f := warn(ruleID, sev, msg, rec)
	f.FilePath = fp
	f.Line = line
	return f
}

func failAt(ruleID, sev, msg, rec, fp string, line int) finding.Finding {
	f := fail(ruleID, sev, msg, rec)
	f.FilePath = fp
	f.Line = line
	return f
}

func stackByLang(stacks []detect.Stack, lang string) (detect.Stack, bool) {
	for _, s := range stacks {
		if s.Language == lang {
			return s, true
		}
	}
	return detect.Stack{}, false
}

// ─── basic checks ────────────────────────────────────────────────────────────

// MANIFEST_PRESENT checks that at least one project manifest was detected.
func checkManifestPresent(ctx Context) []finding.Finding {
	const id = "MANIFEST_PRESENT"
	if len(ctx.Stacks) > 0 {
		langs := make([]string, 0, len(ctx.Stacks))
		for _, s := range ctx.Stacks {
			langs = append(langs, s.Language)
		}
		return []finding.Finding{pass(id, finding.SeverityInfo,
			"Project manifest found: "+strings.Join(langs, ", "),
			"No action required.")}
	}
	return []finding.Finding{warn(id, finding.SeverityMedium,
		"No recognised project manifest found (package.json, go.mod, Cargo.toml, composer.json, requirements.txt, pyproject.toml, Pipfile).",
		"Ensure the manifest file is present in the project root. LaunchSafe checks the top-level directory only.")}
}

// MANIFEST_READABLE checks that detected manifests can be parsed.
// Only validates JSON manifests (package.json, composer.json).
func checkManifestReadable(ctx Context) []finding.Finding {
	const id = "MANIFEST_READABLE"
	var findings []finding.Finding

	jsonManifests := []string{"package.json", "composer.json"}
	for _, name := range jsonManifests {
		fp := filepath.Join(ctx.Root, name)
		if !exists(fp) {
			continue
		}
		data, err := os.ReadFile(fp)
		if err != nil {
			findings = append(findings, failAt(id, finding.SeverityHigh,
				name+" exists but cannot be read: "+err.Error(),
				"Ensure the file is readable.",
				name, 0))
			continue
		}
		var v interface{}
		if err := json.Unmarshal(data, &v); err != nil {
			findings = append(findings, failAt(id, finding.SeverityHigh,
				name+" is not valid JSON: "+err.Error(),
				"Fix the JSON syntax error in "+name+".",
				name, 0))
		} else {
			findings = append(findings, pass(id, finding.SeverityInfo,
				name+" is valid JSON.", "No action required."))
		}
	}
	if len(findings) == 0 {
		findings = append(findings, pass(id, finding.SeverityInfo,
			"Manifest readability check skipped (no JSON manifests present).",
			"No action required."))
	}
	return findings
}

// START_COMMAND checks for a likely production start command.
func checkStartCommand(ctx Context) []finding.Finding {
	const id = "START_COMMAND"

	// Check Procfile (generic, used by Heroku and similar platforms)
	if exists(filepath.Join(ctx.Root, "Procfile")) {
		return []finding.Finding{pass(id, finding.SeverityInfo,
			"Procfile found with production process definition.",
			"No action required.")}
	}

	// Node.js: check scripts.start
	if s, ok := stackByLang(ctx.Stacks, "node"); ok {
		fp := filepath.Join(ctx.Root, s.Manifest)
		data, err := os.ReadFile(fp)
		if err == nil {
			var pkg struct {
				Scripts map[string]string `json:"scripts"`
			}
			if json.Unmarshal(data, &pkg) == nil && pkg.Scripts["start"] != "" {
				return []finding.Finding{pass(id, finding.SeverityInfo,
					"Node.js start script found: "+pkg.Scripts["start"],
					"No action required.")}
			}
		}
		return []finding.Finding{warn(id, finding.SeverityLow,
			"No `scripts.start` found in package.json.",
			"Add a `start` script to package.json (e.g. `\"start\": \"node server.js\"`) or add a Procfile.")}
	}

	// Go: check for main package
	if _, ok := stackByLang(ctx.Stacks, "go"); ok {
		if exists(filepath.Join(ctx.Root, "main.go")) || exists(filepath.Join(ctx.Root, "cmd")) {
			return []finding.Finding{pass(id, finding.SeverityInfo,
				"Go project with main package or cmd/ directory found.",
				"No action required.")}
		}
		return []finding.Finding{warn(id, finding.SeverityLow,
			"Go project detected but no main.go or cmd/ directory found at root.",
			"Ensure your main package entry point is accessible.")}
	}

	// Rust: presence of [[bin]] or src/main.rs implies an executable
	if _, ok := stackByLang(ctx.Stacks, "rust"); ok {
		if exists(filepath.Join(ctx.Root, "src", "main.rs")) {
			return []finding.Finding{pass(id, finding.SeverityInfo,
				"Rust binary project (src/main.rs) found.",
				"No action required.")}
		}
		return []finding.Finding{warn(id, finding.SeverityLow,
			"Rust project detected but src/main.rs not found.",
			"Ensure your binary entry point is defined in Cargo.toml.")}
	}

	// PHP/Laravel
	if s, ok := stackByLang(ctx.Stacks, "php"); ok {
		if s.Framework == "laravel" && exists(filepath.Join(ctx.Root, "artisan")) {
			return []finding.Finding{pass(id, finding.SeverityInfo,
				"Laravel application with artisan found.",
				"Use `php artisan serve` for local dev; configure a proper web server (nginx/Apache) for production.")}
		}
		if exists(filepath.Join(ctx.Root, "public", "index.php")) || exists(filepath.Join(ctx.Root, "index.php")) {
			return []finding.Finding{pass(id, finding.SeverityInfo,
				"PHP entry point found.",
				"Configure your web server to point to the public directory.")}
		}
		return []finding.Finding{warn(id, finding.SeverityLow,
			"PHP project detected but no entry point (public/index.php or index.php) found.",
			"Add an entry point or a Procfile.")}
	}

	// Python
	if _, ok := stackByLang(ctx.Stacks, "python"); ok {
		for _, f := range []string{"manage.py", "app.py", "main.py", "wsgi.py", "asgi.py"} {
			if exists(filepath.Join(ctx.Root, f)) {
				return []finding.Finding{pass(id, finding.SeverityInfo,
					"Python entry point found: "+f,
					"Use a production WSGI/ASGI server (e.g. gunicorn, uvicorn) rather than the development server.")}
			}
		}
		return []finding.Finding{warn(id, finding.SeverityLow,
			"Python project detected but no common entry point found (manage.py, app.py, main.py).",
			"Add a Procfile or confirm the entry point path.")}
	}

	return []finding.Finding{warn(id, finding.SeverityLow,
		"Could not detect a production start command.",
		"Add a Procfile or verify your deployment platform's start command.")}
}

// ENV_TEMPLATE checks for an .env example/template file.
func checkEnvTemplate(ctx Context) []finding.Finding {
	const id = "ENV_TEMPLATE"
	templates := []string{".env.example", ".env.sample", ".env.template", ".env.dist"}
	for _, t := range templates {
		if exists(filepath.Join(ctx.Root, t)) {
			return []finding.Finding{pass(id, finding.SeverityInfo,
				"Environment template found: "+t,
				"No action required. Keep this file updated as new variables are added.")}
		}
	}
	return []finding.Finding{warn(id, finding.SeverityLow,
		"No .env.example (or .env.sample/.env.template/.env.dist) found.",
		"Add an .env.example file listing all required environment variables without their values, so teammates and deployment pipelines know what to configure.")}
}

// GITIGNORE_ENV checks that .env files are excluded from version control.
func checkGitignoreEnv(ctx Context) []finding.Finding {
	const id = "GITIGNORE_ENV"
	giPath := filepath.Join(ctx.Root, ".gitignore")
	if !exists(giPath) {
		return []finding.Finding{warn(id, finding.SeverityMedium,
			"No .gitignore file found.",
			"Create a .gitignore that includes `.env` to prevent accidental secret commits.")}
	}
	data, err := os.ReadFile(giPath)
	if err != nil {
		return []finding.Finding{warn(id, finding.SeverityMedium,
			".gitignore exists but could not be read.",
			"Ensure .gitignore is readable and contains `.env`.")}
	}

	// Check for .env coverage: patterns like .env, .env*, *.env, .env.local
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t == ".env" || t == ".env*" || t == "*.env" || t == ".env.*" ||
			t == ".env.local" || t == ".env.production" {
			return []finding.Finding{pass(id, finding.SeverityInfo,
				".env is covered by .gitignore.",
				"No action required.")}
		}
	}

	// Check if an actual .env exists — if so, this is a real risk.
	if exists(filepath.Join(ctx.Root, ".env")) {
		return []finding.Finding{fail(id, finding.SeverityHigh,
			".env file exists but does not appear to be excluded by .gitignore.",
			"Add `.env` or `.env*` to your .gitignore immediately to prevent committing secrets.")}
	}
	return []finding.Finding{warn(id, finding.SeverityMedium,
		".env does not appear to be excluded by .gitignore.",
		"Add `.env` or `.env*` to your .gitignore to prevent accidentally committing secrets if an .env file is created.")}
}

// DOCKERFILE_PRESENT warns (not fails) if no Dockerfile is found.
// A Dockerfile is optional for many deployment setups.
func checkDockerfilePresent(ctx Context) []finding.Finding {
	const id = "DOCKERFILE_PRESENT"
	for _, name := range []string{"Dockerfile", "dockerfile", "Dockerfile.prod", "Dockerfile.production"} {
		if exists(filepath.Join(ctx.Root, name)) {
			return []finding.Finding{pass(id, finding.SeverityInfo,
				"Dockerfile found: "+name,
				"No action required.")}
		}
	}
	return []finding.Finding{warn(id, finding.SeverityInfo,
		"No Dockerfile found.",
		"If you plan to containerise this application, add a Dockerfile. This is optional for non-container deployments.")}
}

// CI_PRESENT warns (not fails) if no CI configuration is found.
func checkCIPresent(ctx Context) []finding.Finding {
	const id = "CI_PRESENT"
	ciPaths := []string{
		".github/workflows",
		".gitlab-ci.yml",
		".travis.yml",
		".circleci/config.yml",
		"Jenkinsfile",
		"bitbucket-pipelines.yml",
		".drone.yml",
		"azure-pipelines.yml",
	}
	for _, p := range ciPaths {
		full := filepath.Join(ctx.Root, p)
		info, err := os.Stat(full)
		if err != nil {
			continue
		}
		// For .github/workflows, check that it's a directory with at least one file.
		if info.IsDir() {
			entries, err := os.ReadDir(full)
			if err == nil && len(entries) > 0 {
				return []finding.Finding{pass(id, finding.SeverityInfo,
					"CI configuration found: "+p,
					"No action required.")}
			}
			continue
		}
		return []finding.Finding{pass(id, finding.SeverityInfo,
			"CI configuration found: "+p,
			"No action required.")}
	}
	return []finding.Finding{warn(id, finding.SeverityInfo,
		"No CI configuration found.",
		"Consider adding CI (e.g. GitHub Actions) to automate tests before deployment. This is optional if you use another mechanism.")}
}
