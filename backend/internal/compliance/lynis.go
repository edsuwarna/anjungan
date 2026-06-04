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

	// Run Lynis audit
	lynisCmd := `sudo lynis audit system --report-journal --quiet 2>&1 | tail -200 || lynis audit system --report-journal --quiet 2>&1 | tail -200`
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

	// Regex patterns
	hardeningRe := regexp.MustCompile(`Hardening\s*index\s*[:\[]\s*(\d+)\s*[,\]]?\s*score\s*[:\[]\s*(\d+)`)
	hardeningSimpleRe := regexp.MustCompile(`\[Hardening Index\]\s*\[\s*(\d+)\s*\]`)
	testsRe := regexp.MustCompile(`(?:Performed|Tests)\s*:\s*(\d+)`)
	pluginsRe := regexp.MustCompile(`Plugins\s*:\s*(\d+)`)
	warningsRe := regexp.MustCompile(`(?:Security|Warnings)\s*(?:warnings|issues)?\s*:\s*(\d+)`)
	suggestionsRe := regexp.MustCompile(`Suggestions\s*:\s*(\d+)`)
	osRe := regexp.MustCompile(`(?i)(?:Detected\s*OS|Operating\s*system|OS)\s*:\s*(.+)`)
	hostnameRe := regexp.MustCompile(`(?i)Hostname\s*:\s*(.+)`)

	inWarnings := false
	inSuggestions := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Hardening index: multiple formats
		if m := hardeningRe.FindStringSubmatch(trimmed); len(m) >= 3 {
			result.HardeningScore, _ = strconv.Atoi(m[1])
		} else if m := hardeningSimpleRe.FindStringSubmatch(trimmed); len(m) >= 2 {
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

	// Lynis reports score out of ~100
	if result.HardeningScore == 0 {
		// Try to find any number in brackets near "Hardening"
		altRe := regexp.MustCompile(`(?i)hardening.*?(\d{2,3})`)
		if m := altRe.FindStringSubmatch(output); len(m) >= 2 {
			result.HardeningScore, _ = strconv.Atoi(m[1])
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
