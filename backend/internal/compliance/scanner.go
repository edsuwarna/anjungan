package compliance

import (
	"context"
	"fmt"
	"time"

	sshtool "github.com/edsuwarna/anjungan/internal/infra/ssh"
)

// Scanner runs compliance checks against a remote server over SSH.
// Supports multiple scan profiles and individual check execution (Prowler-style).
type Scanner struct {
	registry *CheckRegistry
}

// ScanSummary holds the aggregated results of a compliance scan.
type ScanSummary struct {
	Score     int           `json:"score"`
	Total     int           `json:"total"`
	Passed    int           `json:"passed"`
	Warnings  int           `json:"warnings"`
	Criticals int           `json:"criticals"`
	High      int           `json:"high"`
	Medium    int           `json:"medium"`
	Low       int           `json:"low"`
	Info      int           `json:"info"`
	Findings  []CheckResult `json:"findings"`
}

// NewScanner creates a Scanner with the default check registry.
func NewScanner() *Scanner {
	return &Scanner{
		registry: NewCheckRegistry(),
	}
}

// NewScannerWithRegistry creates a Scanner with a pre-built registry.
func NewScannerWithRegistry(registry *CheckRegistry) *Scanner {
	return &Scanner{registry: registry}
}

// Registry returns the check registry for inspection.
func (s *Scanner) Registry() *CheckRegistry {
	return s.registry
}

// Run executes all checks matching the given profile against the remote server.
//   - ProfileAll: all checks
//   - ProfileCISLevel1: CIS Level 1 checks only
//   - ProfileCISLevel2: CIS Level 1 + Level 2 checks
func (s *Scanner) Run(ctx context.Context, sshCfg sshtool.Config, profile ScanProfile) (*ScanSummary, error) {
	checks := s.registry.GetByProfile(profile)
	if len(checks) == 0 {
		return nil, fmt.Errorf("no compliance checks available for profile %s", profile)
	}

	var findings []CheckResult

	for _, chk := range checks {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		output, err := sshtool.RunCommand(ctx, sshCfg, chk.Command)

		result := chk.Evaluate(output, err)
		result.CheckID = chk.ID
		result.Category = chk.Category
		result.Title = chk.Title
		result.CISID = chk.CISID
		result.CISLevel = chk.CISLevel
		result.Risk = chk.Risk
		if len(chk.References) > 0 {
			result.References = chk.References[0]
		}
		if result.Remediation == "" && chk.Remediation != "" {
			result.Remediation = chk.Remediation
		}
		findings = append(findings, result)
	}

	summary := calculateSummary(findings)
	return summary, nil
}

// RunSingle executes a single compliance check by ID.
func (s *Scanner) RunSingle(ctx context.Context, sshCfg sshtool.Config, checkID string) (*CheckResult, error) {
	chk, ok := s.registry.GetByID(checkID)
	if !ok {
		return nil, fmt.Errorf("check %q not found", checkID)
	}

	output, err := sshtool.RunCommand(ctx, sshCfg, chk.Command)

	result := chk.Evaluate(output, err)
	result.CheckID = chk.ID
	result.Category = chk.Category
	result.Title = chk.Title
	result.CISID = chk.CISID
	result.CISLevel = chk.CISLevel
	result.Risk = chk.Risk
	if len(chk.References) > 0 {
		result.References = chk.References[0]
	}
	if result.Remediation == "" && chk.Remediation != "" {
		result.Remediation = chk.Remediation
	}
	return &result, nil
}

// RunLynisButton runs the Lynis scanner helper (wraps the lynis.go functions).
func (s *Scanner) RunLynis(ctx context.Context, sshCfg sshtool.Config) (*LynisResult, error) {
	return RunLynis(ctx, sshCfg)
}

// RunCommand runs an arbitrary command on the remote server and returns output.
func RunCommand(ctx context.Context, sshCfg sshtool.Config, command string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	return sshtool.RunCommand(ctx, sshCfg, command)
}

// calculateSummary computes the aggregate statistics and security score from
// all check results.
//
// Scoring formula:
//   - Each critical failure deducts 15 points.
//   - Each high/medium failure deducts 5 points.
//   - Minimum score is 0.
func calculateSummary(findings []CheckResult) *ScanSummary {
	total := len(findings)
	passed := 0
	criticals := 0
	high := 0
	medium := 0
	low := 0
	info := 0
	warnings := 0

	for _, f := range findings {
		switch {
		case f.Status == "pass":
			passed++
		case f.Status == "info" || f.Status == "pass":
			info++
		case f.Severity == "critical" && (f.Status == "fail" || f.Status == "warn"):
			criticals++
		case f.Severity == "high" && (f.Status == "fail" || f.Status == "warn"):
			high++
			warnings++
		case f.Severity == "medium" && (f.Status == "fail" || f.Status == "warn"):
			medium++
			warnings++
		case f.Severity == "low" && (f.Status == "fail" || f.Status == "warn"):
			low++
		default:
			warnings++
		}
	}

	score := 100 - (criticals * 15) - ((high + medium) * 5)
	if score < 0 {
		score = 0
	}

	return &ScanSummary{
		Score:     score,
		Total:     total,
		Passed:    passed,
		Warnings:  warnings,
		Criticals: criticals,
		High:      high,
		Medium:    medium,
		Low:       low,
		Info:      info,
		Findings:  findings,
	}
}
