// Package integration runs fixture-based tests against the bundled sample projects
// in ../testdata/. These tests exercise the full check pipeline end-to-end.
package integration_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/rashid1125/shipcheck/cmd"
	"github.com/rashid1125/shipcheck/internal/detect"
	"github.com/rashid1125/shipcheck/internal/output"
)

func fixtureDir(name string) string {
	_, file, _, _ := runtime.Caller(0)
	base := filepath.Dir(filepath.Dir(file)) // go up from integration/ to repo root
	return filepath.Join(base, "testdata", name)
}

func runCheck(t *testing.T, path string, extra ...string) output.Report {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	allArgs := append([]string{"check", "--path", path, "--format", "json"}, extra...)
	code := cmd.Run(allArgs, &outBuf, &errBuf)
	if code == cmd.ExitUsage {
		t.Fatalf("check returned usage error for %s: %s", path, errBuf.String())
	}
	var rep output.Report
	if err := json.Unmarshal(outBuf.Bytes(), &rep); err != nil {
		t.Fatalf("JSON parse error for %s: %v\n%s", path, err, outBuf.String())
	}
	return rep
}

func stackByLang(stacks []detect.Stack, lang string) (detect.Stack, bool) {
	for _, s := range stacks {
		if s.Language == lang {
			return s, true
		}
	}
	return detect.Stack{}, false
}

// ─── Node.js fixture ─────────────────────────────────────────────────────────

func TestFixture_NodeJS_Basic(t *testing.T) {
	rep := runCheck(t, fixtureDir("nodejs"))
	if _, ok := stackByLang(rep.Stacks, "node"); !ok {
		t.Fatalf("expected node stack, got: %v", rep.Stacks)
	}
	if rep.Summary.Fail > 0 {
		for _, f := range rep.Findings {
			if f.Status == "fail" {
				t.Logf("FAIL [%s]: %s", f.RuleID, f.Message)
			}
		}
		t.Errorf("well-configured Node.js fixture should have 0 failures, got %d", rep.Summary.Fail)
	}
}

func TestFixture_NodeJS_Advanced(t *testing.T) {
	rep := runCheck(t, fixtureDir("nodejs"), "--advanced")
	s, ok := stackByLang(rep.Stacks, "node")
	if !ok {
		t.Fatal("expected node stack")
	}
	if s.Framework != "express" {
		t.Errorf("expected express framework, got %q", s.Framework)
	}
	// package-lock.json is present → LOCKFILE_PRESENT should pass.
	var lockFound bool
	for _, f := range rep.Findings {
		if f.RuleID == "LOCKFILE_PRESENT" && f.Status == "pass" {
			lockFound = true
		}
	}
	if !lockFound {
		t.Error("expected LOCKFILE_PRESENT pass (package-lock.json exists)")
	}
}

// ─── PHP/Laravel fixture ─────────────────────────────────────────────────────

func TestFixture_PHP_Laravel(t *testing.T) {
	rep := runCheck(t, fixtureDir("php"))
	s, ok := stackByLang(rep.Stacks, "php")
	if !ok {
		t.Fatal("expected php stack")
	}
	if s.Framework != "laravel" {
		t.Errorf("expected laravel, got %q", s.Framework)
	}
}

func TestFixture_PHP_Laravel_Advanced(t *testing.T) {
	rep := runCheck(t, fixtureDir("php"), "--advanced")
	// .env.example with APP_KEY= should give FRAMEWORK_CONFIG pass.
	var fwPass bool
	for _, f := range rep.Findings {
		if f.RuleID == "FRAMEWORK_CONFIG" && f.Status == "pass" {
			fwPass = true
		}
	}
	if !fwPass {
		t.Error("expected FRAMEWORK_CONFIG pass for Laravel fixture with .env.example containing APP_KEY")
	}
}

// ─── Python/Django fixture ────────────────────────────────────────────────────

func TestFixture_Python_Django(t *testing.T) {
	rep := runCheck(t, fixtureDir("python"))
	s, ok := stackByLang(rep.Stacks, "python")
	if !ok {
		t.Fatal("expected python stack")
	}
	if s.Framework != "django" {
		t.Errorf("expected django, got %q", s.Framework)
	}
	// manage.py should satisfy START_COMMAND.
	var startPass bool
	for _, f := range rep.Findings {
		if f.RuleID == "START_COMMAND" && f.Status == "pass" {
			startPass = true
		}
	}
	if !startPass {
		t.Error("expected START_COMMAND pass (manage.py exists)")
	}
}

// ─── Go fixture ──────────────────────────────────────────────────────────────

func TestFixture_Go(t *testing.T) {
	rep := runCheck(t, fixtureDir("gomod"))
	if _, ok := stackByLang(rep.Stacks, "go"); !ok {
		t.Fatal("expected go stack")
	}
	var startPass bool
	for _, f := range rep.Findings {
		if f.RuleID == "START_COMMAND" && f.Status == "pass" {
			startPass = true
		}
	}
	if !startPass {
		t.Error("expected START_COMMAND pass (main.go exists)")
	}
}

// ─── Rust fixture ─────────────────────────────────────────────────────────────

func TestFixture_Rust(t *testing.T) {
	rep := runCheck(t, fixtureDir("rust"))
	if _, ok := stackByLang(rep.Stacks, "rust"); !ok {
		t.Fatal("expected rust stack")
	}
	var startPass bool
	for _, f := range rep.Findings {
		if f.RuleID == "START_COMMAND" && f.Status == "pass" {
			startPass = true
		}
	}
	if !startPass {
		t.Error("expected START_COMMAND pass (src/main.rs exists)")
	}
}

// ─── Stacks are sorted deterministically ──────────────────────────────────────

func TestFixture_Stacks_Deterministic(t *testing.T) {
	for _, dir := range []string{"nodejs", "php", "python", "gomod", "rust"} {
		var prev []string
		for i := 0; i < 5; i++ {
			rep := runCheck(t, fixtureDir(dir))
			var langs []string
			for _, s := range rep.Stacks {
				langs = append(langs, s.Language)
			}
			if prev != nil && len(langs) != len(prev) {
				t.Fatalf("%s: stack count changed between runs", dir)
			}
			for j := range langs {
				if prev != nil && langs[j] != prev[j] {
					t.Fatalf("%s: non-deterministic stack order at run %d", dir, i)
				}
			}
			prev = langs
		}
	}
}

// ─── Summary counts are consistent with findings ──────────────────────────────

func TestFixture_SummaryConsistency(t *testing.T) {
	for _, dir := range []string{"nodejs", "php", "python", "gomod", "rust"} {
		rep := runCheck(t, fixtureDir(dir))
		pass, warn, fail := 0, 0, 0
		for _, f := range rep.Findings {
			switch f.Status {
			case "pass":
				pass++
			case "warn":
				warn++
			case "fail":
				fail++
			}
		}
		if pass != rep.Summary.Pass || warn != rep.Summary.Warn || fail != rep.Summary.Fail {
			t.Errorf("%s: summary mismatch: reported {p:%d,w:%d,f:%d}, counted {p:%d,w:%d,f:%d}",
				dir, rep.Summary.Pass, rep.Summary.Warn, rep.Summary.Fail, pass, warn, fail)
		}
	}
}

// ─── Demo monorepo (Task 2) ───────────────────────────────────────────────────

func TestFixture_DemoMonorepo_Depth(t *testing.T) {
	rep := runCheck(t, fixtureDir("demo-monorepo"), "--depth", "2", "--advanced")

	// Both php and node stacks should appear somewhere across sub-projects.
	phpFound, nodeFound := false, false
	for _, f := range rep.Findings {
		for _, stack := range rep.Stacks {
			if stack.Language == "php" {
				phpFound = true
			}
			if stack.Language == "node" {
				nodeFound = true
			}
		}
		// At least one finding must have a sub_project set.
		_ = f
	}

	if !phpFound {
		t.Errorf("expected php stack across findings, stacks: %v", rep.Stacks)
	}
	if !nodeFound {
		t.Errorf("expected node stack across findings, stacks: %v", rep.Stacks)
	}

	// At least one finding must carry a sub_project field.
	var hasSubProject bool
	for _, f := range rep.Findings {
		if f.SubProject != "" {
			hasSubProject = true
			break
		}
	}
	if !hasSubProject {
		t.Error("expected at least one finding with sub_project set in monorepo mode")
	}

	// GITIGNORE_ENV fail must appear (frontend/.env exists but .gitignore doesn't cover it).
	var gitignoreEnvFail bool
	for _, f := range rep.Findings {
		if f.RuleID == "GITIGNORE_ENV" && f.Status == "fail" {
			gitignoreEnvFail = true
		}
	}
	if !gitignoreEnvFail {
		t.Error("expected GITIGNORE_ENV fail for frontend sub-project (.env not in .gitignore)")
	}
}
