package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rashid1125/launchsafe/internal/output"
)

func run(args ...string) (stdout, stderr string, code int) {
	var outBuf, errBuf bytes.Buffer
	code = Run(args, &outBuf, &errBuf)
	return outBuf.String(), errBuf.String(), code
}

func makeProject(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		fp := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fp, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// ─── version ─────────────────────────────────────────────────────────────────

func TestVersion(t *testing.T) {
	out, _, code := run("version")
	if code != ExitSuccess {
		t.Errorf("version: exit %d", code)
	}
	if !strings.Contains(out, "launchsafe version") {
		t.Errorf("version output missing expected string: %q", out)
	}
}

// ─── help ─────────────────────────────────────────────────────────────────────

func TestHelp(t *testing.T) {
	out, _, code := run("help")
	if code != ExitSuccess {
		t.Errorf("help: exit %d", code)
	}
	if !strings.Contains(out, "check") {
		t.Errorf("help output should mention 'check': %q", out)
	}
}

// ─── unknown command ──────────────────────────────────────────────────────────

func TestUnknownCommand(t *testing.T) {
	_, errOut, code := run("deploy")
	if code != ExitUsage {
		t.Errorf("unknown command should return ExitUsage, got %d", code)
	}
	if !strings.Contains(errOut, "unknown command") {
		t.Errorf("expected 'unknown command' in stderr: %q", errOut)
	}
}

// ─── invalid --format ────────────────────────────────────────────────────────

func TestInvalidFormat(t *testing.T) {
	dir := t.TempDir()
	_, errOut, code := run("check", "--path", dir, "--format", "xml")
	if code != ExitUsage {
		t.Errorf("invalid format should return ExitUsage, got %d", code)
	}
	if !strings.Contains(errOut, "invalid --format") {
		t.Errorf("expected error about invalid format, got %q", errOut)
	}
}

// ─── nonexistent path ────────────────────────────────────────────────────────

func TestNonexistentPath(t *testing.T) {
	_, _, code := run("check", "--path", "/definitely/does/not/exist/xyz123")
	if code != ExitUsage {
		t.Errorf("nonexistent path should return ExitUsage, got %d", code)
	}
}

// ─── JSON output ─────────────────────────────────────────────────────────────

func TestJSONOutput_Valid(t *testing.T) {
	dir := makeProject(t, map[string]string{
		"package.json": `{"name":"app","scripts":{"start":"node index.js"}}`,
	})
	out, _, _ := run("check", "--path", dir, "--format", "json")
	var rep output.Report
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("JSON output not valid: %v\n%s", err, out)
	}
	if rep.Version == "" {
		t.Error("JSON output missing version")
	}
	if rep.Mode != "basic" {
		t.Errorf("expected mode basic, got %q", rep.Mode)
	}
}

func TestJSONOutput_Advanced(t *testing.T) {
	dir := makeProject(t, map[string]string{
		"package.json": `{"name":"app"}`,
	})
	out, _, _ := run("check", "--path", dir, "--format", "json", "--advanced")
	var rep output.Report
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("JSON output not valid: %v\n%s", err, out)
	}
	if rep.Mode != "advanced" {
		t.Errorf("expected mode advanced, got %q", rep.Mode)
	}
}

// ─── SARIF output ─────────────────────────────────────────────────────────────

func TestSARIFOutput_Valid(t *testing.T) {
	dir := makeProject(t, map[string]string{
		"package.json": `{"name":"app"}`,
	})
	out, _, _ := run("check", "--path", dir, "--format", "sarif")
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("SARIF output not valid JSON: %v\n%s", err, out)
	}
	if m["version"] != "2.1.0" {
		t.Errorf("expected SARIF version 2.1.0, got %v", m["version"])
	}
}

// ─── exit codes ─────────────────────────────────────────────────────────────

func TestExitCode_FailOnFail(t *testing.T) {
	// A project with .env that is not in .gitignore should produce a fail.
	dir := makeProject(t, map[string]string{
		".gitignore": "*.log\n",
		".env":       "SECRET=abc\n",
	})
	_, _, code := run("check", "--path", dir)
	if code != ExitFailure {
		t.Errorf("expected exit 1 when checks fail, got %d", code)
	}
}

func TestExitCode_StrictWarning(t *testing.T) {
	// A project with no Dockerfile will get a WARN; --strict should turn that into exit 1.
	dir := makeProject(t, map[string]string{
		".gitignore": ".env\n",
	})
	_, _, code := run("check", "--path", dir, "--strict")
	if code != ExitFailure {
		t.Errorf("expected exit 1 in strict mode with warnings, got %d", code)
	}
}

func TestExitCode_NoArgs(t *testing.T) {
	_, _, code := run()
	if code != ExitUsage {
		t.Errorf("no args should return ExitUsage, got %d", code)
	}
}

// ─── config loading ──────────────────────────────────────────────────────────

func TestConfigFile_DisableRule(t *testing.T) {
	dir := makeProject(t, map[string]string{
		"package.json":     `{"name":"app"}`,
		".launchsafe.json": `{"disabled_rules":["DOCKERFILE_PRESENT"]}`,
	})
	out, _, _ := run("check", "--path", dir, "--format", "json")
	var rep output.Report
	json.Unmarshal([]byte(out), &rep)
	for _, f := range rep.Findings {
		if f.RuleID == "DOCKERFILE_PRESENT" {
			t.Error("disabled rule DOCKERFILE_PRESENT should not appear in output")
		}
	}
}

func TestConfigFile_InvalidSeverity(t *testing.T) {
	dir := makeProject(t, map[string]string{
		".launchsafe.json": `{"severity_overrides":{"RULE":"notaseverity"}}`,
	})
	_, errOut, code := run("check", "--path", dir)
	if code != ExitUsage {
		t.Errorf("invalid config should return ExitUsage, got %d", code)
	}
	if !strings.Contains(errOut, "config error") {
		t.Errorf("expected config error in stderr, got: %q", errOut)
	}
}

func TestExplicitConfigFile_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, errOut, code := run("check", "--path", dir, "--config", "/no/such/file.json")
	if code != ExitUsage {
		t.Errorf("missing config file should return ExitUsage, got %d", code)
	}
	_ = errOut
}
