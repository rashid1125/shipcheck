// Package output formats ShipCheck results as text, JSON, or SARIF 2.1.0.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/rashid1125/shipcheck/internal/detect"
	"github.com/rashid1125/shipcheck/internal/finding"
)

// Report is the full data model shared by all formatters.
// ScanDepth > 0 indicates a monorepo scan with --depth N.
type Report struct {
	Version   string            `json:"version"`
	Path      string            `json:"path"`
	Mode      string            `json:"mode"`
	ScanDepth int               `json:"scan_depth,omitempty"`
	Stacks    []detect.Stack    `json:"stacks"`
	Summary   finding.Summary   `json:"summary"`
	Findings  []finding.Finding `json:"findings"`
}

// ─── Text ─────────────────────────────────────────────────────────────────────

// WriteText writes a human-readable text report.
func WriteText(w io.Writer, r Report) error {
	fmt.Fprintf(w, "ShipCheck %s — Deployment Readiness Check\n", r.Version)
	fmt.Fprintf(w, "Path: %s\n", r.Path)
	fmt.Fprintf(w, "Mode: %s\n", r.Mode)
	if r.ScanDepth > 0 {
		fmt.Fprintf(w, "Depth: %d\n", r.ScanDepth)
	}
	fmt.Fprintln(w)

	if len(r.Stacks) > 0 {
		fmt.Fprintln(w, "Detected stacks:")
		for _, s := range r.Stacks {
			label := s.Language
			if s.Framework != "" {
				label += " (" + s.Framework + ")"
			}
			fmt.Fprintf(w, "  • %s [%s]\n", label, s.Manifest)
		}
		fmt.Fprintln(w)
	} else {
		fmt.Fprintln(w, "Detected stacks: none")
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "Findings:")

	// Group by SubProject for monorepo output.
	currentSP := ""
	for _, f := range r.Findings {
		if f.SubProject != "" && f.SubProject != currentSP {
			currentSP = f.SubProject
			fmt.Fprintf(w, "\n  ── sub-project: %s ──\n", currentSP)
		}

		icon := statusIcon(f.Status)
		loc := ""
		if f.FilePath != "" {
			if f.Line > 0 {
				loc = fmt.Sprintf(" [%s:%d]", f.FilePath, f.Line)
			} else {
				loc = fmt.Sprintf(" [%s]", f.FilePath)
			}
		}
		ruleLabel := f.RuleID
		if f.SubProject != "" {
			ruleLabel = f.RuleID
		}
		fmt.Fprintf(w, "  %s %-6s [%-24s] %s%s\n",
			icon, strings.ToUpper(f.Status), ruleLabel, f.Message, loc)
		if f.Recommendation != "" && f.Status != finding.StatusPass {
			fmt.Fprintf(w, "             → %s\n", f.Recommendation)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "Summary: %d passed, %d warnings, %d failed\n",
		r.Summary.Pass, r.Summary.Warn, r.Summary.Fail)
	return nil
}

func statusIcon(status string) string {
	switch status {
	case finding.StatusPass:
		return "✓"
	case finding.StatusWarn:
		return "⚠"
	case finding.StatusFail:
		return "✗"
	default:
		return "?"
	}
}

// ─── JSON ─────────────────────────────────────────────────────────────────────

// WriteJSON writes a valid JSON report.
func WriteJSON(w io.Writer, r Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// ─── SARIF ────────────────────────────────────────────────────────────────────

type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	Version        string      `json:"version"`
	InformationURI string      `json:"informationUri"`
	Rules          []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID               string            `json:"id"`
	Name             string            `json:"name,omitempty"`
	ShortDescription sarifMessage      `json:"shortDescription"`
	Properties       map[string]string `json:"properties,omitempty"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations,omitempty"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysLoc `json:"physicalLocation"`
}

type sarifPhysLoc struct {
	ArtifactLocation sarifArtLoc  `json:"artifactLocation"`
	Region           *sarifRegion `json:"region,omitempty"`
}

type sarifArtLoc struct {
	URI       string `json:"uri"`
	URIBaseID string `json:"uriBaseId,omitempty"`
}

type sarifRegion struct {
	StartLine int `json:"startLine"`
}

// WriteSARIF writes a SARIF 2.1.0 report.
func WriteSARIF(w io.Writer, r Report) error {
	ruleSet := map[string]bool{}
	var rules []sarifRule
	for _, f := range r.Findings {
		if ruleSet[f.RuleID] {
			continue
		}
		ruleSet[f.RuleID] = true
		rules = append(rules, sarifRule{
			ID:               f.RuleID,
			ShortDescription: sarifMessage{Text: f.Message},
			Properties: map[string]string{
				"severity":       f.Severity,
				"recommendation": f.Recommendation,
			},
		})
	}

	var results []sarifResult
	for _, f := range r.Findings {
		sr := sarifResult{
			RuleID:  f.RuleID,
			Level:   sarifLevel(f),
			Message: sarifMessage{Text: f.Message},
		}
		if f.FilePath != "" {
			uri := f.FilePath
			if f.SubProject != "" && f.SubProject != "." {
				uri = filepath.ToSlash(filepath.Join(f.SubProject, f.FilePath))
			}
			loc := sarifLocation{
				PhysicalLocation: sarifPhysLoc{
					ArtifactLocation: sarifArtLoc{
						URI:       uri,
						URIBaseID: "%SRCROOT%",
					},
				},
			}
			if f.Line > 0 {
				loc.PhysicalLocation.Region = &sarifRegion{StartLine: f.Line}
			}
			sr.Locations = []sarifLocation{loc}
		}
		results = append(results, sr)
	}

	log := sarifLog{
		Schema:  "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json",
		Version: "2.1.0",
		Runs: []sarifRun{{
			Tool: sarifTool{
				Driver: sarifDriver{
					Name:           "ShipCheck",
					Version:        r.Version,
					InformationURI: "https://github.com/rashid1125/shipcheck",
					Rules:          rules,
				},
			},
			Results: results,
		}},
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(log)
}

func sarifLevel(f finding.Finding) string {
	switch f.Status {
	case finding.StatusFail:
		return "error"
	case finding.StatusWarn:
		return "warning"
	default:
		return "note"
	}
}
