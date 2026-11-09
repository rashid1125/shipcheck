package checks

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/rashid1125/launchsafe/internal/finding"
)

// LOCKFILE_PRESENT checks for a dependency lockfile (advanced mode).
func checkLockfilePresent(ctx Context) []finding.Finding {
	const id = "LOCKFILE_PRESENT"

	type lockSpec struct {
		lang string
		file string
	}
	specs := []lockSpec{
		{"node", "package-lock.json"},
		{"node", "yarn.lock"},
		{"node", "pnpm-lock.yaml"},
		{"php", "composer.lock"},
		{"python", "poetry.lock"},
		{"rust", "Cargo.lock"},
		// Go uses go.sum, not strictly a lockfile but provides integrity.
		{"go", "go.sum"},
	}

	for _, spec := range specs {
		if _, ok := stackByLang(ctx.Stacks, spec.lang); ok {
			if exists(filepath.Join(ctx.Root, spec.file)) {
				return []finding.Finding{pass(id, finding.SeverityInfo,
					"Dependency lockfile found: "+spec.file,
					"Keep the lockfile committed to ensure reproducible builds.")}
			}
		}
	}

	if len(ctx.Stacks) == 0 {
		return []finding.Finding{pass(id, finding.SeverityInfo,
			"No stacks detected; lockfile check skipped.",
			"No action required.")}
	}

	return []finding.Finding{warn(id, finding.SeverityMedium,
		"No dependency lockfile found.",
		"Generate and commit a lockfile (package-lock.json, yarn.lock, composer.lock, poetry.lock, etc.) to ensure reproducible deployments.")}
}

// DOCKER_COMPOSE_PRESENT checks for a Docker Compose file (advanced mode).
func checkDockerComposePresent(ctx Context) []finding.Finding {
	const id = "DOCKER_COMPOSE_PRESENT"
	for _, name := range []string{"docker-compose.yml", "docker-compose.yaml",
		"docker-compose.prod.yml", "docker-compose.production.yml", "compose.yml", "compose.yaml"} {
		if exists(filepath.Join(ctx.Root, name)) {
			return []finding.Finding{pass(id, finding.SeverityInfo,
				"Docker Compose file found: "+name,
				"No action required.")}
		}
	}
	return []finding.Finding{warn(id, finding.SeverityInfo,
		"No Docker Compose file found.",
		"If your deployment uses Docker Compose, add a compose file. This is optional for other deployment setups.")}
}

// GITHUB_ACTIONS_PRESENT checks for GitHub Actions workflow YAML files (advanced mode).
func checkGithubActionsPresent(ctx Context) []finding.Finding {
	const id = "GITHUB_ACTIONS_PRESENT"
	workflowDir := filepath.Join(ctx.Root, ".github", "workflows")
	entries, err := os.ReadDir(workflowDir)
	if err != nil {
		return []finding.Finding{warn(id, finding.SeverityInfo,
			"No GitHub Actions workflows directory found (.github/workflows/).",
			"If you host on GitHub, consider adding workflow files for CI/CD.")}
	}

	var ymlFiles []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasSuffix(n, ".yml") || strings.HasSuffix(n, ".yaml") {
			ymlFiles = append(ymlFiles, n)
		}
	}

	if len(ymlFiles) == 0 {
		return []finding.Finding{warn(id, finding.SeverityInfo,
			".github/workflows/ exists but contains no YAML files.",
			"Add at least one workflow YAML file (e.g. ci.yml) to enable GitHub Actions.")}
	}
	return []finding.Finding{pass(id, finding.SeverityInfo,
		"GitHub Actions workflow(s) found: "+strings.Join(ymlFiles, ", "),
		"No action required.")}
}

// PUBLIC_ENV_EXPOSED checks for environment files inside publicly-served directories (advanced mode).
func checkPublicEnvExposed(ctx Context) []finding.Finding {
	const id = "PUBLIC_ENV_EXPOSED"

	// Directories that are commonly web-served and should never contain .env files.
	publicDirs := []string{"public", "web", "www", "static", "dist", "build", "out", "htdocs"}
	var findings []finding.Finding

	for _, dir := range publicDirs {
		dirPath := filepath.Join(ctx.Root, dir)
		if !exists(dirPath) {
			continue
		}
		for _, envName := range []string{".env", ".env.local", ".env.production", ".env.staging"} {
			fp := filepath.Join(dirPath, envName)
			if exists(fp) {
				rel := filepath.Join(dir, envName)
				findings = append(findings, fail(id, finding.SeverityCritical,
					"Potential environment file found in a publicly-served directory: "+rel,
					"Remove "+rel+" immediately. It may expose secrets to anyone who knows the URL. "+
						"Move environment configuration outside the web root."))
			}
		}
	}

	if len(findings) == 0 {
		findings = append(findings, pass(id, finding.SeverityInfo,
			"No .env files found in common public/web directories.",
			"No action required."))
	}
	return findings
}

// DOCKERFILE_BASIC performs basic Dockerfile checks without executing Docker (advanced mode).
// Limitations: does not parse Dockerfile syntax fully; checks are heuristic.
func checkDockerfileBasic(ctx Context) []finding.Finding {
	const id = "DOCKERFILE_BASIC"

	var dfPath string
	for _, name := range []string{"Dockerfile", "dockerfile", "Dockerfile.prod", "Dockerfile.production"} {
		fp := filepath.Join(ctx.Root, name)
		if exists(fp) {
			dfPath = fp
			break
		}
	}
	if dfPath == "" {
		return nil // DOCKERFILE_PRESENT already warns; no need to double-report.
	}

	data, err := os.ReadFile(dfPath)
	if err != nil {
		return []finding.Finding{warn(id, finding.SeverityLow,
			"Dockerfile exists but could not be read.",
			"Ensure the Dockerfile is readable.")}
	}

	content := string(data)
	var findings []finding.Finding
	relPath := filepath.Base(dfPath)

	// Check for FROM instruction
	if !strings.Contains(strings.ToUpper(content), "FROM ") {
		findings = append(findings, fail(id, finding.SeverityHigh,
			"Dockerfile does not contain a FROM instruction.",
			"Add a FROM instruction specifying the base image."))
	}

	// Warn about FROM latest (unpinned base image)
	upperContent := strings.ToUpper(content)
	if strings.Contains(upperContent, ":LATEST") || containsUnpinnedFrom(content) {
		findings = append(findings, warnAt(id, finding.SeverityMedium,
			"Dockerfile may use an unpinned (`:latest`) base image tag.",
			"Pin your base image to a specific version (e.g. `node:20.11-alpine`) for reproducible builds.",
			relPath, 0))
	}

	// Check for EXPOSE instruction
	if !strings.Contains(upperContent, "EXPOSE ") {
		findings = append(findings, warn(id, finding.SeverityInfo,
			"Dockerfile does not contain an EXPOSE instruction.",
			"Add an EXPOSE instruction to document the port your container listens on."))
	}

	// Check for running as root (no USER instruction)
	if !strings.Contains(upperContent, "\nUSER ") && !strings.HasPrefix(upperContent, "USER ") {
		findings = append(findings, warn(id, finding.SeverityMedium,
			"Dockerfile does not set a non-root USER.",
			"Add a USER instruction to run the container as a non-root user for better security."))
	}

	if len(findings) == 0 {
		findings = append(findings, pass(id, finding.SeverityInfo,
			"Dockerfile passes basic checks.",
			"No action required."))
	}
	return findings
}

func containsUnpinnedFrom(content string) bool {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		upper := strings.ToUpper(line)
		if strings.HasPrefix(upper, "FROM ") {
			// FROM image:latest or FROM image (no tag = implicitly latest)
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				image := parts[1]
				// ignore ARG references like $BASE_IMAGE
				if strings.HasPrefix(image, "$") {
					continue
				}
				if strings.HasSuffix(image, ":latest") || !strings.Contains(image, ":") {
					// scratch and ubuntu without tag are common; only warn for :latest
					if strings.HasSuffix(image, ":latest") {
						return true
					}
				}
			}
		}
	}
	return false
}

// FRAMEWORK_CONFIG runs framework-specific configuration checks (advanced mode).
func checkFrameworkConfig(ctx Context) []finding.Finding {
	const id = "FRAMEWORK_CONFIG"
	var findings []finding.Finding

	// Laravel: check APP_KEY placeholder and .env setup
	if s, ok := stackByLang(ctx.Stacks, "php"); ok && s.Framework == "laravel" {
		findings = append(findings, checkLaravelConfig(ctx)...)
	}

	// Next.js: check for next.config.js/ts
	if s, ok := stackByLang(ctx.Stacks, "node"); ok && s.Framework == "nextjs" {
		hasConfig := exists(filepath.Join(ctx.Root, "next.config.js")) ||
			exists(filepath.Join(ctx.Root, "next.config.ts")) ||
			exists(filepath.Join(ctx.Root, "next.config.mjs"))
		if hasConfig {
			findings = append(findings, pass(id, finding.SeverityInfo,
				"Next.js configuration file found.",
				"No action required."))
		} else {
			findings = append(findings, warn(id, finding.SeverityInfo,
				"Next.js project has no next.config.js/ts file.",
				"Consider adding next.config.js to configure your production settings (image domains, redirects, etc.)."))
		}
	}

	// Django: check for settings module
	if s, ok := stackByLang(ctx.Stacks, "python"); ok && s.Framework == "django" {
		hasManagy := exists(filepath.Join(ctx.Root, "manage.py"))
		if hasManagy {
			findings = append(findings, pass(id, finding.SeverityInfo,
				"Django manage.py found.",
				"Ensure DEBUG=False and ALLOWED_HOSTS is set in production settings."))
		}
	}

	if len(findings) == 0 {
		findings = append(findings, pass(id, finding.SeverityInfo,
			"No framework-specific configuration checks applicable.",
			"No action required."))
	}
	return findings
}

func checkLaravelConfig(ctx Context) []finding.Finding {
	const id = "FRAMEWORK_CONFIG"
	// Check that .env.example exists (already done in basic, but here we verify APP_KEY)
	examplePath := filepath.Join(ctx.Root, ".env.example")
	if !exists(examplePath) {
		return []finding.Finding{warn(id, finding.SeverityMedium,
			"Laravel project has no .env.example file.",
			"Generate .env.example from your .env file (removing actual values) so teammates know required variables.")}
	}
	data, err := os.ReadFile(examplePath)
	if err != nil {
		return nil
	}
	if strings.Contains(string(data), "APP_KEY=") {
		return []finding.Finding{pass(id, finding.SeverityInfo,
			"Laravel .env.example contains APP_KEY entry.",
			"Ensure APP_KEY is set to a fresh generated value in each environment via `php artisan key:generate`.")}
	}
	return []finding.Finding{warn(id, finding.SeverityMedium,
		"Laravel .env.example does not contain APP_KEY.",
		"Add APP_KEY= to .env.example so deployment pipelines know to generate it.")}
}
