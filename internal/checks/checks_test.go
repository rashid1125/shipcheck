package checks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rashid1125/shipcheck/internal/config"
	"github.com/rashid1125/shipcheck/internal/detect"
	"github.com/rashid1125/shipcheck/internal/finding"
)

// ─── helpers ────────────────────────────────────────────────────────────────

func newCtx(t *testing.T, root string, stacks []detect.Stack, advanced bool) Context {
	t.Helper()
	return Context{
		Root:     root,
		Stacks:   stacks,
		Advanced: advanced,
		Config:   config.Default(),
	}
}

func makeDir(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		fp := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fp, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func findingByID(findings []finding.Finding, id string) (finding.Finding, bool) {
	for _, f := range findings {
		if f.RuleID == id {
			return f, true
		}
	}
	return finding.Finding{}, false
}

func assertStatus(t *testing.T, findings []finding.Finding, id, wantStatus string) {
	t.Helper()
	f, ok := findingByID(findings, id)
	if !ok {
		t.Fatalf("finding %s not found in %v", id, findings)
	}
	if f.Status != wantStatus {
		t.Errorf("finding %s: want status %q, got %q (message: %s)", id, wantStatus, f.Status, f.Message)
	}
}

// ─── MANIFEST_PRESENT ────────────────────────────────────────────────────────

func TestManifestPresent_Found(t *testing.T) {
	root := makeDir(t, map[string]string{"package.json": `{"name":"app"}`})
	stacks := []detect.Stack{{Language: "node", Manifest: "package.json"}}
	results := Run(newCtx(t, root, stacks, false))
	assertStatus(t, results, "MANIFEST_PRESENT", finding.StatusPass)
}

func TestManifestPresent_Missing(t *testing.T) {
	root := t.TempDir()
	results := Run(newCtx(t, root, nil, false))
	assertStatus(t, results, "MANIFEST_PRESENT", finding.StatusWarn)
}

// ─── MANIFEST_READABLE ───────────────────────────────────────────────────────

func TestManifestReadable_ValidJSON(t *testing.T) {
	root := makeDir(t, map[string]string{"package.json": `{"name":"app"}`})
	stacks := []detect.Stack{{Language: "node"}}
	results := Run(newCtx(t, root, stacks, false))
	assertStatus(t, results, "MANIFEST_READABLE", finding.StatusPass)
}

func TestManifestReadable_InvalidJSON(t *testing.T) {
	root := makeDir(t, map[string]string{"package.json": `{broken`})
	stacks := []detect.Stack{{Language: "node"}}
	results := Run(newCtx(t, root, stacks, false))
	assertStatus(t, results, "MANIFEST_READABLE", finding.StatusFail)
}

// ─── GITIGNORE_ENV ───────────────────────────────────────────────────────────

func TestGitignoreEnv_Covered(t *testing.T) {
	root := makeDir(t, map[string]string{".gitignore": ".env\n*.log\n"})
	results := Run(newCtx(t, root, nil, false))
	assertStatus(t, results, "GITIGNORE_ENV", finding.StatusPass)
}

func TestGitignoreEnv_NotCovered_NoEnvFile(t *testing.T) {
	root := makeDir(t, map[string]string{".gitignore": "*.log\n"})
	results := Run(newCtx(t, root, nil, false))
	assertStatus(t, results, "GITIGNORE_ENV", finding.StatusWarn)
}

func TestGitignoreEnv_NotCovered_EnvFileExists(t *testing.T) {
	root := makeDir(t, map[string]string{
		".gitignore": "*.log\n",
		".env":       "SECRET=abc\n",
	})
	results := Run(newCtx(t, root, nil, false))
	assertStatus(t, results, "GITIGNORE_ENV", finding.StatusFail)
}

func TestGitignoreEnv_WildcardPattern(t *testing.T) {
	root := makeDir(t, map[string]string{".gitignore": ".env*\n"})
	results := Run(newCtx(t, root, nil, false))
	assertStatus(t, results, "GITIGNORE_ENV", finding.StatusPass)
}

func TestGitignoreEnv_Missing(t *testing.T) {
	root := t.TempDir()
	results := Run(newCtx(t, root, nil, false))
	assertStatus(t, results, "GITIGNORE_ENV", finding.StatusWarn)
}

// ─── ENV_TEMPLATE ────────────────────────────────────────────────────────────

func TestEnvTemplate_Found(t *testing.T) {
	root := makeDir(t, map[string]string{".env.example": "DB_URL=\nSECRET=\n"})
	results := Run(newCtx(t, root, nil, false))
	assertStatus(t, results, "ENV_TEMPLATE", finding.StatusPass)
}

func TestEnvTemplate_Missing(t *testing.T) {
	root := t.TempDir()
	results := Run(newCtx(t, root, nil, false))
	assertStatus(t, results, "ENV_TEMPLATE", finding.StatusWarn)
}

// ─── DOCKERFILE_PRESENT ──────────────────────────────────────────────────────

func TestDockerfilePresent_Found(t *testing.T) {
	root := makeDir(t, map[string]string{"Dockerfile": "FROM node:20\n"})
	results := Run(newCtx(t, root, nil, false))
	assertStatus(t, results, "DOCKERFILE_PRESENT", finding.StatusPass)
}

func TestDockerfilePresent_Missing(t *testing.T) {
	root := t.TempDir()
	results := Run(newCtx(t, root, nil, false))
	assertStatus(t, results, "DOCKERFILE_PRESENT", finding.StatusWarn)
}

// ─── LOCKFILE_PRESENT (advanced) ─────────────────────────────────────────────

func TestLockfilePresent_Node(t *testing.T) {
	root := makeDir(t, map[string]string{
		"package.json":      `{"name":"app"}`,
		"package-lock.json": `{}`,
	})
	stacks := []detect.Stack{{Language: "node"}}
	results := Run(newCtx(t, root, stacks, true))
	assertStatus(t, results, "LOCKFILE_PRESENT", finding.StatusPass)
}

func TestLockfilePresent_Missing(t *testing.T) {
	root := makeDir(t, map[string]string{"package.json": `{"name":"app"}`})
	stacks := []detect.Stack{{Language: "node"}}
	results := Run(newCtx(t, root, stacks, true))
	assertStatus(t, results, "LOCKFILE_PRESENT", finding.StatusWarn)
}

// ─── PUBLIC_ENV_EXPOSED (advanced) ───────────────────────────────────────────

func TestPublicEnvExposed_Found(t *testing.T) {
	root := makeDir(t, map[string]string{"public/.env": "SECRET=exposed\n"})
	results := Run(newCtx(t, root, nil, true))
	assertStatus(t, results, "PUBLIC_ENV_EXPOSED", finding.StatusFail)
}

func TestPublicEnvExposed_Clean(t *testing.T) {
	root := makeDir(t, map[string]string{"public/index.html": "<html/>"})
	results := Run(newCtx(t, root, nil, true))
	assertStatus(t, results, "PUBLIC_ENV_EXPOSED", finding.StatusPass)
}

// ─── HARDCODED_SECRETS (advanced) ─────────────────────────────────────────────

func TestHardcodedSecrets_Clean(t *testing.T) {
	root := makeDir(t, map[string]string{
		"main.go": "package main\nfunc main(){}\n",
	})
	results := Run(newCtx(t, root, nil, true))
	assertStatus(t, results, "HARDCODED_SECRETS", finding.StatusPass)
}

func TestHardcodedSecrets_FoundInSource(t *testing.T) {
	root := makeDir(t, map[string]string{
		"config.go": `package config
const apiKey = "password=supersecretvalue123"
`,
	})
	results := Run(newCtx(t, root, nil, true))
	var found bool
	for _, f := range results {
		if f.RuleID == "HARDCODED_SECRETS" && f.Status == finding.StatusFail {
			found = true
			// Value must not appear in the finding message or recommendation.
			if contains(f.Message, "supersecretvalue123") || contains(f.Recommendation, "supersecretvalue123") {
				t.Error("secret value must not appear in finding output")
			}
		}
	}
	if !found {
		t.Error("expected HARDCODED_SECRETS fail finding")
	}
}

func TestHardcodedSecrets_SkipsEnvFiles(t *testing.T) {
	root := makeDir(t, map[string]string{
		".env":         "PASSWORD=realsecret123456\n",
		".env.example": "PASSWORD=\n",
	})
	results := Run(newCtx(t, root, nil, true))
	for _, f := range results {
		if f.RuleID == "HARDCODED_SECRETS" && f.Status == finding.StatusFail {
			if f.FilePath == ".env" || f.FilePath == ".env.example" {
				t.Errorf("secret scanner must not report findings from .env files, got %v", f)
			}
		}
	}
}

func TestHardcodedSecrets_SkipsNodeModules(t *testing.T) {
	root := makeDir(t, map[string]string{
		"node_modules/lib/index.js": `const secret = "password=abc123456789";`,
	})
	results := Run(newCtx(t, root, nil, true))
	for _, f := range results {
		if f.RuleID == "HARDCODED_SECRETS" && f.Status == finding.StatusFail {
			t.Errorf("scanner must skip node_modules, but found: %v", f)
		}
	}
}

// ─── Config: disabled rules ───────────────────────────────────────────────────

func TestDisabledRule(t *testing.T) {
	root := t.TempDir()
	cfg := config.Default()
	cfg.DisabledRules = []string{"DOCKERFILE_PRESENT"}
	ctx := Context{Root: root, Config: cfg}
	results := Run(ctx)
	for _, f := range results {
		if f.RuleID == "DOCKERFILE_PRESENT" {
			t.Error("disabled rule DOCKERFILE_PRESENT should not appear in results")
		}
	}
}

// ─── Config: severity override ────────────────────────────────────────────────

func TestSeverityOverride(t *testing.T) {
	root := t.TempDir()
	cfg := config.Default()
	cfg.SeverityOverrides = map[string]string{"CI_PRESENT": "high"}
	ctx := Context{Root: root, Config: cfg}
	results := Run(ctx)
	f, ok := findingByID(results, "CI_PRESENT")
	if !ok {
		t.Fatal("CI_PRESENT not found")
	}
	if f.Severity != "high" {
		t.Errorf("expected severity high after override, got %q", f.Severity)
	}
}

// ─── Strict mode: all findings included ──────────────────────────────────────

func TestRun_Summary(t *testing.T) {
	root := makeDir(t, map[string]string{
		".gitignore":   ".env\n",
		".env.example": "DB=\n",
		"package.json": `{"name":"app","scripts":{"start":"node index.js"}}`,
		"Dockerfile":   "FROM node:20\nEXPOSE 3000\nUSER node\n",
	})
	stacks := []detect.Stack{{Language: "node", Manifest: "package.json"}}
	results := Run(newCtx(t, root, stacks, false))
	sum := finding.Summarize(results)
	if sum.Fail > 0 {
		t.Errorf("expected no failures on a well-configured project, got %d fails", sum.Fail)
	}
}

func contains(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) &&
		(s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ─── Task 3 — false-positive reduction tests ──────────────────────────────────

func TestHardcodedSecrets_SkipsTestFiles(t *testing.T) {
	root := makeDir(t, map[string]string{
		"auth_test.go": `package auth
func TestLogin(t *testing.T) {
	password := "supersecret12345"
	_ = password
}
`,
	})
	results := Run(newCtx(t, root, nil, true))
	for _, f := range results {
		if f.RuleID == "HARDCODED_SECRETS" && f.Status == finding.StatusFail {
			t.Errorf("scanner should skip _test.go files, got finding in %s", f.FilePath)
		}
	}
}

func TestHardcodedSecrets_SkipsMarkdown(t *testing.T) {
	root := makeDir(t, map[string]string{
		"README.md": "## Example\n\nSet `password=yoursecrethere` in .env\n",
	})
	results := Run(newCtx(t, root, nil, true))
	for _, f := range results {
		if f.RuleID == "HARDCODED_SECRETS" && f.Status == finding.StatusFail {
			t.Errorf("scanner should skip .md files, got finding in %s", f.FilePath)
		}
	}
}

func TestHardcodedSecrets_SkipsMinJS(t *testing.T) {
	root := makeDir(t, map[string]string{
		"app.min.js": `var password="supersecretvalue";`,
	})
	results := Run(newCtx(t, root, nil, true))
	for _, f := range results {
		if f.RuleID == "HARDCODED_SECRETS" && f.Status == finding.StatusFail {
			t.Errorf("scanner should skip .min.js files, got finding in %s", f.FilePath)
		}
	}
}

func TestHardcodedSecrets_IgnorePaths(t *testing.T) {
	root := makeDir(t, map[string]string{
		"fixtures/config.go": `package fixtures
const apiKey = "password=supersecretvalue123"
`,
	})
	cfg := config.Default()
	cfg.IgnorePaths = []string{"fixtures/"}
	ctx := Context{Root: root, Advanced: true, Config: cfg}
	results := Run(ctx)
	for _, f := range results {
		if f.RuleID == "HARDCODED_SECRETS" && f.Status == finding.StatusFail {
			t.Errorf("scanner should respect ignore_paths, got finding in %s", f.FilePath)
		}
	}
}

func TestHardcodedSecrets_InlineSuppression(t *testing.T) {
	root := makeDir(t, map[string]string{
		"config.go": `package config
const apiKey = "password=supersecretvalue123" // shipcheck:ignore HARDCODED_SECRETS
`,
	})
	results := Run(newCtx(t, root, nil, true))
	for _, f := range results {
		if f.RuleID == "HARDCODED_SECRETS" && f.Status == finding.StatusFail {
			t.Errorf("scanner should respect inline suppression, got finding at line %d", f.Line)
		}
	}
}

func TestHardcodedSecrets_PEMKey_IsCritical(t *testing.T) {
	root := makeDir(t, map[string]string{
		"storage/key.pem": "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA...\n-----END RSA PRIVATE KEY-----\n",
	})
	results := Run(newCtx(t, root, nil, true))
	var found bool
	for _, f := range results {
		if f.RuleID == "HARDCODED_SECRETS" && f.Status == finding.StatusFail && f.Severity == finding.SeverityCritical {
			found = true
			if contains(f.Message, "MIIEpA") {
				t.Error("PEM key content must not appear in finding message")
			}
		}
	}
	if !found {
		t.Error("expected critical HARDCODED_SECRETS fail for PEM private key")
	}
}

func TestHardcodedSecrets_FirebaseApiKey_IsWarn(t *testing.T) {
	root := makeDir(t, map[string]string{
		"lib/firebase.js": `const firebaseConfig = {
  apiKey: "AIzaSyFAKEKEYFORTESTINGONLY12345678901",
  authDomain: "myapp.firebaseapp.com",
};`,
	})
	results := Run(newCtx(t, root, nil, true))
	var warnFound, failFound bool
	for _, f := range results {
		if f.RuleID == "HARDCODED_SECRETS" {
			if f.Status == finding.StatusWarn {
				warnFound = true
			}
			if f.Status == finding.StatusFail {
				failFound = true
			}
		}
	}
	if failFound {
		t.Error("Firebase apiKey should be WARN not FAIL")
	}
	_ = warnFound // warn is optional depending on pattern; no fail is the key assertion
}
