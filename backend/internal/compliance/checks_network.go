package compliance

import (
	"strconv"
	"strings"
)

// Network checks — CIS Benchmark Section 3
func NetworkChecks() []CheckDefinition {
	return []CheckDefinition{
		firewallActive(),
		publicPorts(),
		netUfwDefaultDeny(),
		netIPTablesPolicy(),
		netUnusedInterfaces(),
	}
}

func firewallActive() CheckDefinition {
	return CheckDefinition{
		ID:       "net_firewall_active",
		Category: "network",
		Title:    "Firewall Active",
		Command:  `ufw status verbose 2>/dev/null || (iptables -L -n 2>/dev/null | head -5 | grep -q 'Chain' && echo 'iptables-active') || echo 'inactive'`,
		Severity: "critical",
		CISID:    "3.5.1",
		CISLevel: 1,
		Risk:     "Without an active firewall, all network services are exposed to the internet, increasing attack surface.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Enable ufw or configure iptables with appropriate rules to restrict inbound traffic.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check firewall status", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if strings.Contains(trimmed, "inactive") {
				return CheckResult{
					Status:      "fail",
					Severity:    "critical",
					Description: "No firewall is active — server is exposed to unrestricted network access",
					RawOutput:   output,
				}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "OK — firewall is active", RawOutput: output}
		},
	}
}

func publicPorts() CheckDefinition {
	return CheckDefinition{
		ID:       "net_public_ports",
		Category: "network",
		Title:    "Publicly Listening Ports",
		Command:  `ss -tlnp 2>/dev/null | awk '{print $4}' | grep -v '^127\.' | grep -v '^::1' | grep -v '127\.0\.0\.1' | grep -v ' 127\.' | grep -E '0\.0\.0\.0|\*'`,
		Severity: "medium",
		CISID:    "3.5.2",
		CISLevel: 1,
		Risk:     "Services bound to 0.0.0.0 are accessible from any network interface, including public ones. Bind to specific IPs when remote access isn't required.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Bind services to localhost (127.0.0.1) when remote access is not required. Use reverse proxy for public services.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check listening ports", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if trimmed == "" {
				return CheckResult{Status: "pass", Severity: "info", Description: "OK — no public ports detected", RawOutput: output}
			}
			lines := strings.Split(trimmed, "\n")
			var nonSSHPorts []string
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				parts := strings.Split(line, ":")
				port := parts[len(parts)-1]
				if port != "22" {
					nonSSHPorts = append(nonSSHPorts, line)
				}
			}
			if len(nonSSHPorts) > 0 {
				return CheckResult{
					Status:      "warn",
					Severity:    "medium",
					Description: "Publicly listening ports (excluding SSH): " + strings.Join(nonSSHPorts, ", "),
					RawOutput:   output,
				}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "OK — only SSH on public interface", RawOutput: output}
		},
	}
}

func netUfwDefaultDeny() CheckDefinition {
	return CheckDefinition{
		ID:       "net_ufw_default_deny",
		Category: "network",
		Title:    "UFW Default Deny Policy",
		Command:  `ufw status verbose 2>/dev/null | grep -i 'default' || echo 'unknown'`,
		Severity: "high",
		CISID:    "3.5.3",
		CISLevel: 1,
		Risk:     "Default allow policy permits all inbound traffic not explicitly denied, weakening firewall protection.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Set default deny: 'ufw default deny incoming' and 'ufw default allow outgoing'.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check UFW policy", RawOutput: output}
			}
			trimmed := strings.ToLower(strings.TrimSpace(output))
			if strings.Contains(trimmed, "deny") {
				return CheckResult{Status: "pass", Severity: "info", Description: "OK — UFW default policy is deny", RawOutput: output}
			}
			if trimmed == "unknown" {
				return CheckResult{Status: "warn", Severity: "info", Description: "UFW not in use — check iptables policy instead", RawOutput: output}
			}
			return CheckResult{
				Status:      "fail",
				Severity:    "high",
				Description: "UFW default policy is not deny — inbound traffic may be permitted by default",
				RawOutput:   output,
			}
		},
	}
}

func netIPTablesPolicy() CheckDefinition {
	return CheckDefinition{
		ID:       "net_iptables_policy",
		Category: "network",
		Title:    "IPTables INPUT Default Policy",
		Command:  `iptables -L INPUT -n 2>/dev/null | head -1 | awk '{print $4}' || echo 'unknown'`,
		Severity: "high",
		CISID:    "3.5.4",
		CISLevel: 2,
		Risk:     "Default ACCEPT policy on INPUT chain allows all traffic not explicitly matched by rules, reducing security effectiveness.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Set default policy to DROP: 'iptables -P INPUT DROP'. Add explicit ACCEPT rules for required services.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check iptables policy", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			lower := strings.ToLower(trimmed)
			if lower == "drop" || lower == "reject" {
				return CheckResult{Status: "pass", Severity: "info", Description: "OK — INPUT default policy is DROP/REJECT", RawOutput: output}
			}
			if lower == "accept" {
				return CheckResult{
					Status:      "fail",
					Severity:    "high",
					Description: "INPUT chain default policy is ACCEPT — all traffic allowed by default",
					RawOutput:   output,
				}
			}
			return CheckResult{Status: "warn", Severity: "info", Description: "Could not determine iptables policy", RawOutput: output}
		},
	}
}

func netUnusedInterfaces() CheckDefinition {
	return CheckDefinition{
		ID:       "net_unused_interfaces",
		Category: "network",
		Title:    "Unused Network Interfaces",
		Command:  `ip link show 2>/dev/null | grep -v 'LOOPBACK\|ether' | grep 'state DOWN' | wc -l`,
		Severity: "low",
		CISID:    "3.6",
		CISLevel: 2,
		Risk:     "Unused network interfaces increase the attack surface and may provide undetected access paths.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Disable unused interfaces: 'ip link set <interface> down' or remove the interface configuration.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check network interfaces", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			count, parseErr := strconv.Atoi(trimmed)
			if parseErr != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not parse interface count", RawOutput: output}
			}
			if count > 1 {
				return CheckResult{
					Status:      "warn",
					Severity:    "low",
					Description: "Found " + trimmed + " unused/down interface(s) — consider disabling",
					RawOutput:   output,
				}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "OK — no unused interfaces detected", RawOutput: output}
		},
	}
}
