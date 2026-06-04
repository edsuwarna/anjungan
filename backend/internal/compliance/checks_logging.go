package compliance

import (
	"strings"
)

// Logging checks — CIS Benchmark Section 4
func LoggingChecks() []CheckDefinition {
	return []CheckDefinition{
		logRsyslogConfigured(),
		logAuditdRules(),
		logRotateConfigured(),
	}
}

func logRsyslogConfigured() CheckDefinition {
	return CheckDefinition{
		ID:       "log_rsyslog",
		Category: "logging",
		Title:    "Rsyslog Configured and Running",
		Command:  `systemctl is-active rsyslog 2>/dev/null || echo 'inactive'; dpkg -l rsyslog 2>/dev/null | grep -c '^ii' || echo 0`,
		Severity: "high",
		CISID:    "4.2.1",
		CISLevel: 1,
		Risk:     "Without syslog, system and application logs are lost on reboot, hampering incident investigation.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Install and enable rsyslog: 'apt install rsyslog && systemctl enable --now rsyslog'.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check rsyslog status", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			lines := strings.SplitN(trimmed, "\n", 2)
			activeStatus := strings.TrimSpace(lines[0])
			if activeStatus != "active" {
				return CheckResult{
					Status:      "fail",
					Severity:    "high",
					Description: "Rsyslog is not running — system logs are not being collected",
					RawOutput:   output,
				}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "OK — rsyslog is running", RawOutput: output}
		},
	}
}

func logAuditdRules() CheckDefinition {
	return CheckDefinition{
		ID:       "log_auditd_rules",
		Category: "logging",
		Title:    "Auditd Rules Loaded",
		Command:  `auditctl -l 2>/dev/null | wc -l || (test -f /etc/audit/rules.d/audit.rules && wc -l < /etc/audit/rules.d/audit.rules || echo '0')`,
		Severity: "high",
		CISID:    "4.1.2",
		CISLevel: 2,
		Risk:     "Without audit rules, auditd captures no events, defeating the purpose of having it installed.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Add audit rules in /etc/audit/rules.d/audit.rules. CIS provides benchmark-specific rule sets.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check audit rules", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			count := 0
			if trimmed != "0" && trimmed != "" {
				count = 1 // non-zero means rules exist
			}
			if count == 0 {
				return CheckResult{
					Status:      "fail",
					Severity:    "high",
					Description: "No audit rules loaded — auditd is not monitoring events",
					RawOutput:   output,
				}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "OK — audit rules are loaded", RawOutput: output}
		},
	}
}

func logRotateConfigured() CheckDefinition {
	return CheckDefinition{
		ID:       "log_logrotate",
		Category: "logging",
		Title:    "Logrotate Configured",
		Command:  `test -f /etc/logrotate.conf && echo 'exists' || echo 'missing'`,
		Severity: "medium",
		CISID:    "4.3",
		CISLevel: 1,
		Risk:     "Without log rotation, log files can fill the disk, causing service disruption and loss of forensic data.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Configure logrotate: install logrotate package and configure /etc/logrotate.conf.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check logrotate", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if trimmed != "exists" {
				return CheckResult{
					Status:      "fail",
					Severity:    "medium",
					Description: "Logrotate is not configured — logs may fill disk",
					RawOutput:   output,
				}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "OK — logrotate is configured", RawOutput: output}
		},
	}
}
