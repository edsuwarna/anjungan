package compliance

// CheckResult holds the outcome of a single compliance check.
type CheckResult struct {
	CheckID     string `json:"check_id"`
	Category    string `json:"category"`
	Title       string `json:"title"`
	Severity    string `json:"severity"`    // critical, high, medium, low, info
	Description string `json:"description"`
	Remediation string `json:"remediation"`
	RawOutput   string `json:"raw_output"`
	Status      string `json:"status"`      // pass, fail, warn, info
	CISID       string `json:"cis_id,omitempty"`
	CISLevel    int    `json:"cis_level,omitempty"`
	Risk        string `json:"risk,omitempty"`
	References  string `json:"references,omitempty"` // comma-separated URLs
}

// CheckDefinition defines a compliance check to run on a remote server.
type CheckDefinition struct {
	ID          string
	Category    string
	Title       string
	Command     string
	Severity    string
	CISID       string   // e.g. "5.2.1"
	CISLevel    int      // 1 or 2
	Remediation string
	Risk        string   // explanation of the risk
	References  []string // URLs
	Evaluate    func(output string, err error) CheckResult
}

// CheckInfo is the public metadata for a check (safe for API exposure).
type CheckInfo struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	Title    string `json:"title"`
	Severity string `json:"severity"`
	CISID    string `json:"cis_id"`
	CISLevel int    `json:"cis_level"`
	Risk     string `json:"risk"`
}

// ScanProfile defines which set of checks to run.
type ScanProfile int

const (
	ProfileCISLevel1 ScanProfile = 1
	ProfileCISLevel2 ScanProfile = 2
	ProfileAll       ScanProfile = 0
)

// String returns the human-readable profile name.
func (p ScanProfile) String() string {
	switch p {
	case ProfileCISLevel1:
		return "CIS Level 1"
	case ProfileCISLevel2:
		return "CIS Level 2"
	case ProfileAll:
		return "All Checks"
	default:
		return "Unknown"
	}
}

// LynisResult holds parsed Lynis audit output.
type LynisResult struct {
	HardeningScore int               `json:"hardening_score"`
	Tests          int               `json:"tests"`
	Plugins        int               `json:"plugins"`
	Warnings       int               `json:"warnings"`
	Suggestions    int               `json:"suggestions"`
	OsVersion      string            `json:"os_version"`
	Hostname       string            `json:"hostname"`
	Categories     []LynisCategory   `json:"categories,omitempty"`
	RawLog         string            `json:"raw_log,omitempty"`
	SuggestionsList []LynisSuggestion `json:"suggestions_list,omitempty"`
	WarningsList   []LynisWarning    `json:"warnings_list,omitempty"`
}

type LynisCategory struct {
	Name       string `json:"name"`
	Tests      int    `json:"tests"`
	Passed     int    `json:"passed"`
	Warnings   int    `json:"warnings"`
	Suggestions int   `json:"suggestions"`
}

type LynisSuggestion struct {
	TestID      string `json:"test_id"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

type LynisWarning struct {
	TestID      string `json:"test_id"`
	Category    string `json:"category"`
	Description string `json:"description"`
}
