package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rashid1125/shipcheck/internal/detect"
	"github.com/rashid1125/shipcheck/internal/finding"
)

func sampleReport() Report {
	return Report{
		Version: "0.1.0",
		Path:    "/tmp/project",
		Mode:    "basic",
		Stacks: []detect.Stack{
			{Language: "node", Framework: "nextjs", Manifest: "package.json"},
		},
		Findings: []finding.Finding{
			{
				RuleID:         "MANIFEST_PRESENT",
				Severity:       finding.SeverityInfo,
				Status:         finding.StatusPass,
				Message:        "Project manifest found: node",
				Recommendation: "No action required.",
			},
			{
				RuleID:         "GITIGNORE_ENV",
				Severity:       finding.SeverityHigh,
				Status:         finding.StatusFail,
				Message:        ".env not in .gitignore",
				FilePath:       ".gitignore",
				Line:           1,
				Recommendation: "Add .env to .gitignore.",
			},
			{
				RuleID:         "DOCKERFILE_PRESENT",
				Severity:       finding.SeverityInfo,
				Status:         finding.StatusWarn,
				Message:        "No Dockerfile found",
				Recommendation: "Add a Dockerfile if needed.",
			},
		},
		Summary: finding.Summary{Pass: 1, Warn: 1, Fail: 1},
	}
}

// ─── Text ─────────────────────────────────────────────────────────────────────

func TestWriteText_Contains(t *testing.T) {
	var buf bytes.Buffer
	r := sampleReport()
	if err := WriteText(&buf, r); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	checks := []string{
		"ShipCheck",
		"/tmp/project",
		"basic",
		"node (nextjs)",
		"MANIFEST_PRESENT",
		"GITIGNORE_ENV",
		"DOCKERFILE_PRESENT",
		"1 passed",
		"1 warnings",
		"1 failed",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("text output missing %q", want)
		}
	}
}

func TestWriteText_NoError(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteText(&buf, Report{Version: "0.0.0", Findings: nil}); err != nil {
		t.Errorf("WriteText returned error: %v", err)
	}
}

// ─── JSON ─────────────────────────────────────────────────────────────────────

func TestWriteJSON_ValidJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteJSON(&buf, sampleReport()); err != nil {
		t.Fatal(err)
	}
	var got Report
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("JSON output is not valid: %v\n%s", err, buf.String())
	}
}

func TestWriteJSON_Structure(t *testing.T) {
	var buf bytes.Buffer
	r := sampleReport()
	_ = WriteJSON(&buf, r)

	var m map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"version", "path", "mode", "stacks", "summary", "findings"} {
		if _, ok := m[key]; !ok {
			t.Errorf("JSON output missing top-level key %q", key)
		}
	}
}

func TestWriteJSON_FindingsFields(t *testing.T) {
	var buf bytes.Buffer
	_ = WriteJSON(&buf, sampleReport())

	var got struct {
		Findings []struct {
			RuleID   string `json:"rule_id"`
			Severity string `json:"severity"`
			Status   string `json:"status"`
			Message  string `json:"message"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Findings) != 3 {
		t.Fatalf("expected 3 findings, got %d", len(got.Findings))
	}
	f := got.Findings[0]
	if f.RuleID == "" || f.Severity == "" || f.Status == "" || f.Message == "" {
		t.Errorf("finding missing required fields: %+v", f)
	}
}

func TestWriteJSON_Summary(t *testing.T) {
	var buf bytes.Buffer
	_ = WriteJSON(&buf, sampleReport())

	var got struct {
		Summary finding.Summary `json:"summary"`
	}
	json.Unmarshal(buf.Bytes(), &got)
	if got.Summary.Pass != 1 || got.Summary.Warn != 1 || got.Summary.Fail != 1 {
		t.Errorf("unexpected summary: %+v", got.Summary)
	}
}

// ─── SARIF ────────────────────────────────────────────────────────────────────

func TestWriteSARIF_ValidJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteSARIF(&buf, sampleReport()); err != nil {
		t.Fatal(err)
	}
	var m interface{}
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("SARIF output is not valid JSON: %v\n%s", err, buf.String())
	}
}

func TestWriteSARIF_Version(t *testing.T) {
	var buf bytes.Buffer
	_ = WriteSARIF(&buf, sampleReport())

	var got struct {
		Version string `json:"version"`
		Schema  string `json:"$schema"`
	}
	json.Unmarshal(buf.Bytes(), &got)
	if got.Version != "2.1.0" {
		t.Errorf("expected SARIF version 2.1.0, got %q", got.Version)
	}
	if !strings.Contains(got.Schema, "sarif") {
		t.Errorf("unexpected schema: %q", got.Schema)
	}
}

func TestWriteSARIF_Structure(t *testing.T) {
	var buf bytes.Buffer
	_ = WriteSARIF(&buf, sampleReport())

	var got struct {
		Runs []struct {
			Tool struct {
				Driver struct {
					Name  string      `json:"name"`
					Rules interface{} `json:"rules"`
				} `json:"driver"`
			} `json:"tool"`
			Results []struct {
				RuleID string `json:"ruleId"`
				Level  string `json:"level"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(got.Runs))
	}
	run := got.Runs[0]
	if run.Tool.Driver.Name != "ShipCheck" {
		t.Errorf("expected tool name LaunchSafe, got %q", run.Tool.Driver.Name)
	}
	if len(run.Results) != 3 {
		t.Errorf("expected 3 results, got %d", len(run.Results))
	}
}

func TestWriteSARIF_Levels(t *testing.T) {
	var buf bytes.Buffer
	_ = WriteSARIF(&buf, sampleReport())

	var got struct {
		Runs []struct {
			Results []struct {
				RuleID string `json:"ruleId"`
				Level  string `json:"level"`
			} `json:"results"`
		} `json:"runs"`
	}
	json.Unmarshal(buf.Bytes(), &got)
	levelByRule := map[string]string{}
	for _, res := range got.Runs[0].Results {
		levelByRule[res.RuleID] = res.Level
	}
	if levelByRule["MANIFEST_PRESENT"] != "note" {
		t.Errorf("pass finding should map to SARIF note, got %q", levelByRule["MANIFEST_PRESENT"])
	}
	if levelByRule["GITIGNORE_ENV"] != "error" {
		t.Errorf("fail finding should map to SARIF error, got %q", levelByRule["GITIGNORE_ENV"])
	}
	if levelByRule["DOCKERFILE_PRESENT"] != "warning" {
		t.Errorf("warn finding should map to SARIF warning, got %q", levelByRule["DOCKERFILE_PRESENT"])
	}
}

func TestWriteSARIF_Location(t *testing.T) {
	var buf bytes.Buffer
	_ = WriteSARIF(&buf, sampleReport())

	var got struct {
		Runs []struct {
			Results []struct {
				RuleID    string `json:"ruleId"`
				Locations []struct {
					PhysicalLocation struct {
						ArtifactLocation struct {
							URI string `json:"uri"`
						} `json:"artifactLocation"`
						Region *struct {
							StartLine int `json:"startLine"`
						} `json:"region"`
					} `json:"physicalLocation"`
				} `json:"locations"`
			} `json:"results"`
		} `json:"runs"`
	}
	json.Unmarshal(buf.Bytes(), &got)

	for _, res := range got.Runs[0].Results {
		if res.RuleID == "GITIGNORE_ENV" {
			if len(res.Locations) == 0 {
				t.Error("GITIGNORE_ENV should have a location")
				continue
			}
			loc := res.Locations[0].PhysicalLocation
			if loc.ArtifactLocation.URI != ".gitignore" {
				t.Errorf("expected URI .gitignore, got %q", loc.ArtifactLocation.URI)
			}
			if loc.Region == nil || loc.Region.StartLine != 1 {
				t.Errorf("expected startLine 1, got %v", loc.Region)
			}
		}
	}
}
