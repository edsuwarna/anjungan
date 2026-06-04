package compliance

import (
	"strconv"
	"strings"
)

// Service checks — CIS Benchmark Section 2
func ServiceChecks() []CheckDefinition {
	return []CheckDefinition{
		fail2banActive(),
		svcAuditdInstalled(),
		svcChronyNTP(),
		svcCronAllowed(),
		svcPackageUpdates(),
		svcUnnecessaryServices(),
	}
}

func fail2banActive() CheckDefinition {
	return CheckDefinition{
		ID:       "svc_fail2ban",
		Category: "services",
		Title:    "Fail2ban Active",
		Command:  `systemctl is-active fail2ban 2>/dev/null || echo 'inactive'`,
		Severity: "high",
		CISID:    "2.2.1",
		CISLevel: 1,
		Risk:     "Without fail2ban, brute-force attacks on SSH and other services have unlimited attempts.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Install and enable fail2ban: 'apt install fail2ban && systemctl enable --now fail2ban'.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check fail2ban status", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if trimmed != "active" {
				return CheckResult{
					Status:      "fail",
					Severity:    "high",
					Description: "Fail2ban is not active — brute-force protection is disabled",
					RawOutput:   output,
				}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "OK — fail2ban is active", RawOutput: output}
		},
	}
}

func svcAuditdInstalled() CheckDefinition {
	return CheckDefinition{
		ID:       "svc_auditd",
		Category: "services",
		Title:    "Auditd Installed and Running",
		Command:  `systemctl is-active auditd 2>/dev/null || echo 'inactive'; dpkg -l auditd 2>/dev/null | grep -c '^ii' || rpm -q audit 2>/dev/null | grep -v 'not installed' | wc -l`,
		Severity: "high",
		CISID:    "4.1.1",
		CISLevel: 1,
		Risk:     "Without auditd, security-relevant events (logins, privilege escalation, file changes) are not recorded, reducing incident response capability.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Install auditd: 'apt install auditd && systemctl enable --now auditd'.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check auditd status", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			lines := strings.SplitN(trimmed, "\n", 2)
			activeStatus := strings.TrimSpace(lines[0])
			if activeStatus != "active" {
				return CheckResult{
					Status:      "fail",
					Severity:    "high",
					Description: "Auditd is not installed or not running — security events are not being logged",
					RawOutput:   output,
				}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "OK — auditd is installed and running", RawOutput: output}
		},
	}
}

func svcChronyNTP() CheckDefinition {
	return CheckDefinition{
		ID:       "svc_chrony_ntp",
		Category: "services",
		Title:    "NTP/Chrony Time Synchronization",
		Command:  `timedatectl status 2>/dev/null | grep -i 'ntp service\|systemd-timesyncd' || (systemctl is-active chrony 2>/dev/null || systemctl is-active ntpd 2>/dev/null || echo 'inactive')`,
		Severity: "medium",
		CISID:    "2.2.1.1",
		CISLevel: 1,
		Risk:     "Inaccurate system time affects log timestamps, certificate validation, and time-based access controls.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Enable time synchronization: 'timedatectl set-ntp yes' or install chrony.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check NTP status", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			lower := strings.ToLower(trimmed)
			if strings.Contains(lower, "inactive") || strings.Contains(lower, "no") {
				return CheckResult{
					Status:      "fail",
					Severity:    "medium",
					Description: "NTP time synchronization is not active",
					RawOutput:   output,
				}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "OK — time synchronization is active", RawOutput: output}
		},
	}
}

func svcCronAllowed() CheckDefinition {
	return CheckDefinition{
		ID:       "svc_cron_allowed",
		Category: "services",
		Title:    "Cron Allow File",
		Command:  `test -f /etc/cron.allow && echo 'exists' || echo 'missing'; test -f /etc/cron.deny && echo 'exists' || echo 'missing'`,
		Severity: "medium",
		CISID:    "5.1.1",
		CISLevel: 2,
		Risk:     "Without cron.allow, any user can create cron jobs, potentially for persistence or resource abuse.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Create /etc/cron.allow with authorized users. Consider removing /etc/cron.deny.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check cron files", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if !strings.Contains(trimmed, "exists") {
				return CheckResult{
					Status:      "warn",
					Severity:    "medium",
					Description: "Neither /etc/cron.allow nor /etc/cron.deny restrict cron usage",
					RawOutput:   output,
				}
			}
			if strings.Contains(trimmed, "cron.allow") {
				return CheckResult{Status: "pass", Severity: "info", Description: "OK — /etc/cron.allow restricts cron access", RawOutput: output}
			}
			return CheckResult{Status: "warn", Severity: "low", Description: "Cron.deny exists but cron.allow is missing", RawOutput: output}
		},
	}
}

func svcPackageUpdates() CheckDefinition {
	return CheckDefinition{
		ID:       "svc_package_updates",
		Category: "services",
		Title:    "Pending Package Updates",
		Command:  `apt list --upgradable 2>/dev/null | grep -v 'Listing...' | wc -l || (yum check-update 2>/dev/null | wc -l) || echo 0`,
		Severity: "high",
		CISID:    "1.7",
		CISLevel: 1,
		Risk:     "Unpatched packages contain known vulnerabilities that can be exploited to compromise the system.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Apply updates: 'apt update && apt upgrade' or the appropriate package manager for your distribution.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil || output == "" {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check pending updates", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			count, _ := strconv.Atoi(trimmed)
			if count > 50 {
				return CheckResult{
					Status:      "fail",
					Severity:    "high",
					Description: trimmed + " pending package updates — high number of unpatched packages",
					RawOutput:   output,
				}
			}
			if count > 10 {
				return CheckResult{
					Status:      "fail",
					Severity:    "medium",
					Description: trimmed + " pending package updates",
					RawOutput:   output,
				}
			}
			if count > 0 {
				return CheckResult{
					Status:      "warn",
					Severity:    "low",
					Description: trimmed + " pending package updates",
					RawOutput:   output,
				}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "OK — packages are up to date", RawOutput: output}
		},
	}
}

func svcUnnecessaryServices() CheckDefinition {
	return CheckDefinition{
		ID:       "svc_unnecessary_services",
		Category: "services",
		Title:    "Unnecessary Services Disabled",
		Command:  `for svc in avahi-daemon cups bluetooth isc-dhcp-server slapd rpcbind; do systemctl is-active $svc 2>/dev/null | grep -q 'active' && echo "$svc:active"; done`,
		Severity: "medium",
		CISID:    "2.2.2",
		CISLevel: 2,
		Risk:     "Unnecessary services increase the attack surface. Services like avahi, cups, and bluetooth are rarely needed on servers.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Disable unnecessary services: 'systemctl stop <svc> && systemctl disable <svc>'.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil || output == "" {
				return CheckResult{Status: "pass", Severity: "info", Description: "OK — no unnecessary services detected", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			lines := strings.Split(trimmed, "\n")
			activeServices := []string{}
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" {
					parts := strings.SplitN(line, ":", 2)
					activeServices = append(activeServices, parts[0])
				}
			}
			if len(activeServices) > 0 {
				return CheckResult{
					Status:      "fail",
					Severity:    "medium",
					Description: "Unnecessary services are active: " + strings.Join(activeServices, ", "),
					RawOutput:   output,
				}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "OK — no unnecessary services active", RawOutput: output}
		},
	}
}
