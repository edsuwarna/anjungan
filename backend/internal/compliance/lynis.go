package compliance

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	sshtool "github.com/edsuwarna/anjungan/internal/infra/ssh"
)

// ─── Lynis Integration ─────────────────────────────────────────────────────
//
// Runs `lynis audit system --report-journal` over SSH and parses the output.
// Lynis provides a hardening score, test counts, warnings, and suggestions.

// RunLynis executes a Lynis system audit on the remote server and parses results.
func RunLynis(ctx context.Context, sshCfg sshtool.Config) (*LynisResult, error) {
	// First check if Lynis is installed
	checkCmd := `which lynis 2>/dev/null || dpkg -l lynis 2>/dev/null | grep -q '^ii' && echo "installed" || echo "not-installed"`
	checkOut, err := sshtool.RunCommand(ctx, sshCfg, checkCmd)
	if err != nil || strings.TrimSpace(checkOut) == "not-installed" {
		return nil, fmt.Errorf("lynis is not installed on this server")
	}

	// Run Lynis audit — no --quiet to ensure summary lines in output
	lynisCmd := `sudo lynis audit system --no-colors 2>&1 | tail -250 || lynis audit system --no-colors 2>&1 | tail -250`
	output, err := sshtool.RunCommand(ctx, sshCfg, lynisCmd)
	if err != nil && output == "" {
		return nil, fmt.Errorf("lynis audit failed: %w", err)
	}

	result := parseLynisOutput(output)
	result.RawLog = output
	return result, nil
}

// parseLynisOutput extracts structured data from Lynis console output.
func parseLynisOutput(output string) *LynisResult {
	result := &LynisResult{}

	lines := strings.Split(output, "\n")

	// Regex patterns — Lynis 3.x output format
	//   "Hardening index : 57 [###########         ]"
	hardeningRe := regexp.MustCompile(`Hardening\s+index\s*:\s*(\d+)`)
	//   "Tests performed : 266"
	testsRe := regexp.MustCompile(`Tests\s+performed\s*:\s*(\d+)`)
	//   "Plugins enabled : 1"
	pluginsRe := regexp.MustCompile(`Plugins\s+enabled\s*:\s*(\d+)`)
	//   "Warnings (1):"
	warningsRe := regexp.MustCompile(`(?i)(?:Security|Warnings)\s*\((\d+)\)\s*:`)
	//   "Suggestions (51):"
	suggestionsRe := regexp.MustCompile(`(?i)Suggestions\s*\((\d+)\)\s*:`)
	osRe := regexp.MustCompile(`(?i)(?:Detected\s*OS|Operating\s*system|OS)\s*:\s*(.+)`)
	hostnameRe := regexp.MustCompile(`(?i)Hostname\s*:\s*(.+)`)

	inWarnings := false
	inSuggestions := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Hardening index
		if m := hardeningRe.FindStringSubmatch(trimmed); len(m) >= 2 {
			result.HardeningScore, _ = strconv.Atoi(m[1])
		}

		// Test counts
		if m := testsRe.FindStringSubmatch(trimmed); len(m) >= 2 {
			result.Tests, _ = strconv.Atoi(m[1])
		}
		if m := pluginsRe.FindStringSubmatch(trimmed); len(m) >= 2 {
			result.Plugins, _ = strconv.Atoi(m[1])
		}
		if m := warningsRe.FindStringSubmatch(trimmed); len(m) >= 2 {
			result.Warnings, _ = strconv.Atoi(m[1])
		}
		if m := suggestionsRe.FindStringSubmatch(trimmed); len(m) >= 2 {
			result.Suggestions, _ = strconv.Atoi(m[1])
		}

		// OS and hostname
		if m := osRe.FindStringSubmatch(trimmed); len(m) >= 2 {
			result.OsVersion = strings.TrimSpace(m[1])
		}
		if m := hostnameRe.FindStringSubmatch(trimmed); len(m) >= 2 {
			result.Hostname = strings.TrimSpace(m[1])
		}

		// Parse warning/suggestion sections
		if strings.Contains(trimmed, "[WARNING]") || strings.Contains(trimmed, "Warning:") {
			inWarnings = true
			inSuggestions = false
		}
		if strings.Contains(trimmed, "[SUGGESTION]") || strings.Contains(trimmed, "Suggestion:") {
			inSuggestions = true
			inWarnings = false
		}

		// Collect individual warnings
		if inWarnings && strings.TrimSpace(line) != "" {
			if testMatch := extractTestID(trimmed); testMatch != "" {
				result.WarningsList = append(result.WarningsList, LynisWarning{
					TestID:      testMatch,
					Description: trimmed,
				})
			}
		}

		// Collect individual suggestions
		if inSuggestions && strings.TrimSpace(line) != "" {
			if testMatch := extractTestID(trimmed); testMatch != "" {
				result.SuggestionsList = append(result.SuggestionsList, LynisSuggestion{
					TestID:      testMatch,
					Description: trimmed,
				})
			}
		}
	}

	return result
}

// extractTestID pulls a test identifier like "TEST-1234" or "LYNIS:TEST-1234"
func extractTestID(s string) string {
	re := regexp.MustCompile(`(?:LYNIS:)?(?:TEST-|test-|test_)(\w+)`)
	if m := re.FindStringSubmatch(s); len(m) >= 2 {
		return "TEST-" + m[1]
	}
	return ""
}
