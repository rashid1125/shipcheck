package finding

// Severity levels.
const (
	SeverityInfo     = "info"
	SeverityLow      = "low"
	SeverityMedium   = "medium"
	SeverityHigh     = "high"
	SeverityCritical = "critical"
)

// Status values.
const (
	StatusPass = "pass"
	StatusWarn = "warn"
	StatusFail = "fail"
)

// ValidSeverities is the set of accepted severity values.
var ValidSeverities = map[string]bool{
	SeverityInfo:     true,
	SeverityLow:      true,
	SeverityMedium:   true,
	SeverityHigh:     true,
	SeverityCritical: true,
}

// Finding represents a single check result.
type Finding struct {
	RuleID         string `json:"rule_id"`
	Severity       string `json:"severity"`
	Status         string `json:"status"`
	Message        string `json:"message"`
	FilePath       string `json:"file_path,omitempty"`
	Line           int    `json:"line,omitempty"`
	Recommendation string `json:"recommendation"`
}

// Summary holds aggregated counts.
type Summary struct {
	Pass int `json:"pass"`
	Warn int `json:"warn"`
	Fail int `json:"fail"`
}

// Summarize counts finding statuses.
func Summarize(findings []Finding) Summary {
	s := Summary{}
	for _, f := range findings {
		switch f.Status {
		case StatusPass:
			s.Pass++
		case StatusWarn:
			s.Warn++
		case StatusFail:
			s.Fail++
		}
	}
	return s
}
