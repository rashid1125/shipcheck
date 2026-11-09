package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rashid1125/launchsafe/internal/finding"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "cfg.json")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDefault(t *testing.T) {
	c := Default()
	if c == nil {
		t.Fatal("Default returned nil")
	}
	if c.IsDisabled("ANY") {
		t.Error("default config should not disable anything")
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	p := writeTemp(t, `{
		"disabled_rules": ["RULE_A"],
		"severity_overrides": {"RULE_B": "high"}
	}`)
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.IsDisabled("RULE_A") {
		t.Error("RULE_A should be disabled")
	}
	if cfg.IsDisabled("RULE_B") {
		t.Error("RULE_B should not be disabled")
	}
	f := finding.Finding{RuleID: "RULE_B", Severity: "low"}
	f = cfg.Apply(f)
	if f.Severity != "high" {
		t.Errorf("expected severity high, got %s", f.Severity)
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	p := writeTemp(t, `{invalid json}`)
	_, err := Load(p)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoad_InvalidSeverity(t *testing.T) {
	p := writeTemp(t, `{"severity_overrides": {"RULE_X": "catastrophic"}}`)
	_, err := Load(p)
	if err == nil {
		t.Fatal("expected error for invalid severity")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path/.launchsafe.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestValidate_AllSeverities(t *testing.T) {
	for _, sev := range []string{"info", "low", "medium", "high", "critical"} {
		cfg := &Config{
			SeverityOverrides: map[string]string{"RULE": sev},
		}
		if err := cfg.Validate(); err != nil {
			t.Errorf("severity %q should be valid: %v", sev, err)
		}
	}
}

func TestApply_NoOverride(t *testing.T) {
	cfg := Default()
	f := finding.Finding{RuleID: "RULE", Severity: "medium"}
	out := cfg.Apply(f)
	if out.Severity != "medium" {
		t.Errorf("severity should be unchanged, got %s", out.Severity)
	}
}

func TestIsDisabled_EmptyList(t *testing.T) {
	cfg := Default()
	if cfg.IsDisabled("ANYTHING") {
		t.Error("empty disabled list should not disable anything")
	}
}
