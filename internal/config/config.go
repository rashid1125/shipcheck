package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/rashid1125/launchsafe/internal/finding"
)

// DefaultConfigFile is the filename looked up automatically in the project root.
const DefaultConfigFile = ".launchsafe.json"

// Config holds user-configurable rule settings.
type Config struct {
	DisabledRules     []string          `json:"disabled_rules"`
	SeverityOverrides map[string]string `json:"severity_overrides"`
}

// Default returns a configuration with no overrides.
func Default() *Config {
	return &Config{
		DisabledRules:     nil,
		SeverityOverrides: make(map[string]string),
	}
}

// Load reads and validates a config file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: invalid JSON: %w", path, err)
	}
	if cfg.SeverityOverrides == nil {
		cfg.SeverityOverrides = make(map[string]string)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Validate checks that all override severities are valid.
func (c *Config) Validate() error {
	for ruleID, sev := range c.SeverityOverrides {
		if !finding.ValidSeverities[sev] {
			return fmt.Errorf("invalid severity %q for rule %q in severity_overrides", sev, ruleID)
		}
	}
	return nil
}

// IsDisabled reports whether the given rule ID is in the disabled list.
func (c *Config) IsDisabled(ruleID string) bool {
	for _, id := range c.DisabledRules {
		if id == ruleID {
			return true
		}
	}
	return false
}

// Apply applies severity overrides to a finding.
func (c *Config) Apply(f finding.Finding) finding.Finding {
	if sev, ok := c.SeverityOverrides[f.RuleID]; ok {
		f.Severity = sev
	}
	return f
}
