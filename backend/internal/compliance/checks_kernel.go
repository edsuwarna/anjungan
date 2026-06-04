package compliance

import (
	"strings"
)

// Kernel checks — CIS Benchmark Section 1 & 3
func KernelChecks() []CheckDefinition {
	return []CheckDefinition{
		kernelRebootRequired(),
		kernelIPForward(),
		kernelICMPRedirects(),
		kernelASLR(),
		kernelCoreDumps(),
		kernelSyncookies(),
		kernelICMPIgnore(),
		kernelMartianPackets(),
	}
}

func kernelRebootRequired() CheckDefinition {
	return CheckDefinition{
		ID:       "kernel_reboot_required",
		Category: "kernel",
		Title:    "Kernel Reboot Required",
		Command:  `test -f /var/run/reboot-required && echo 'yes' || echo 'no'`,
		Severity: "medium",
		CISID:    "1.1",
		CISLevel: 1,
		Risk:     "Pending kernel updates may contain critical security fixes. Running an outdated kernel exposes the system to known vulnerabilities.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Reboot the server to apply pending kernel updates: 'sudo reboot'.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check if reboot is required", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if strings.Contains(trimmed, "yes") {
				return CheckResult{
					Status:      "fail",
					Severity:    "medium",
					Description: "A system reboot is required to apply pending kernel updates",
					RawOutput:   output,
				}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "OK — no reboot required", RawOutput: output}
		},
	}
}

func kernelIPForward() CheckDefinition {
	return CheckDefinition{
		ID:       "kernel_ip_forward",
		Category: "kernel",
		Title:    "IP Forwarding Disabled",
		Command:  `sysctl -n net.ipv4.ip_forward 2>/dev/null || echo 'unknown'`,
		Severity: "high",
		CISID:    "3.1.1",
		CISLevel: 1,
		Risk:     "IP forwarding allows the system to act as a router. If not required, it increases the attack surface for network-based exploits.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Disable IP forwarding: 'sysctl -w net.ipv4.ip_forward=0' and add 'net.ipv4.ip_forward=0' to /etc/sysctl.conf",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check IP forwarding", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if trimmed == "1" {
				return CheckResult{
					Status:      "fail",
					Severity:    "high",
					Description: "IP forwarding is enabled — system can be used as a router",
					RawOutput:   output,
				}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "OK — IP forwarding is disabled", RawOutput: output}
		},
	}
}

func kernelICMPRedirects() CheckDefinition {
	return CheckDefinition{
		ID:       "kernel_icmp_redirects",
		Category: "kernel",
		Title:    "ICMP Redirect Acceptance Disabled",
		Command:  `sysctl -n net.ipv4.conf.all.accept_redirects 2>/dev/null || echo 'unknown'`,
		Severity: "high",
		CISID:    "3.2.1",
		CISLevel: 1,
		Risk:     "Accepting ICMP redirects allows attackers to alter routing tables, enabling man-in-the-middle attacks.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Disable ICMP redirect acceptance: 'sysctl -w net.ipv4.conf.all.accept_redirects=0' and persist in /etc/sysctl.conf",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check ICMP redirects", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if trimmed != "0" {
				return CheckResult{
					Status:      "fail",
					Severity:    "high",
					Description: "ICMP redirects are accepted — MITM attack risk",
					RawOutput:   output,
				}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "OK — ICMP redirects are disabled", RawOutput: output}
		},
	}
}

func kernelASLR() CheckDefinition {
	return CheckDefinition{
		ID:       "kernel_aslr",
		Category: "kernel",
		Title:    "ASLR Enabled",
		Command:  `sysctl -n kernel.randomize_va_space 2>/dev/null || echo 'unknown'`,
		Severity: "high",
		CISID:    "1.6.1",
		CISLevel: 1,
		Risk:     "ASLR randomizes memory addresses, making buffer overflow and ROP attacks significantly harder to execute.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Enable ASLR: 'sysctl -w kernel.randomize_va_space=2' and add 'kernel.randomize_va_space=2' to /etc/sysctl.conf",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check ASLR", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if trimmed != "2" {
				return CheckResult{
					Status:      "fail",
					Severity:    "high",
					Description: "ASLR is not fully enabled (value: " + trimmed + ", should be 2)",
					RawOutput:   output,
				}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "OK — ASLR is fully enabled", RawOutput: output}
		},
	}
}

func kernelCoreDumps() CheckDefinition {
	return CheckDefinition{
		ID:       "kernel_core_dumps",
		Category: "kernel",
		Title:    "Core Dumps Restricted",
		Command:  `sysctl -n fs.suid_dumpable 2>/dev/null || echo 'unknown'; ulimit -c 2>/dev/null`,
		Severity: "medium",
		CISID:    "1.5.1",
		CISLevel: 1,
		Risk:     "Core dumps can contain sensitive data like passwords, encryption keys, and memory contents.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Restrict core dumps: 'sysctl -w fs.suid_dumpable=0', set '* hard core 0' in /etc/security/limits.conf, and add 'ulimit -c 0' to profile.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check core dumps", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			parts := strings.SplitN(trimmed, "\n", 2)
			dumpable := strings.TrimSpace(parts[0])
			if dumpable != "0" {
				return CheckResult{
					Status:      "fail",
					Severity:    "medium",
					Description: "SUID core dumps are not restricted (fs.suid_dumpable=" + dumpable + ")",
					RawOutput:   output,
				}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "OK — core dumps are restricted", RawOutput: output}
		},
	}
}

func kernelSyncookies() CheckDefinition {
	return CheckDefinition{
		ID:       "kernel_syn_cookies",
		Category: "kernel",
		Title:    "TCP SYN Cookies Enabled",
		Command:  `sysctl -n net.ipv4.tcp_syncookies 2>/dev/null || echo 'unknown'`,
		Severity: "medium",
		CISID:    "3.3.1",
		CISLevel: 1,
		Risk:     "SYN cookies protect against SYN flood attacks that can exhaust server resources and cause denial of service.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Enable SYN cookies: 'sysctl -w net.ipv4.tcp_syncookies=1' and persist in /etc/sysctl.conf",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check SYN cookies", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if trimmed != "1" {
				return CheckResult{
					Status:      "fail",
					Severity:    "medium",
					Description: "SYN cookies are not enabled — vulnerable to SYN flood attacks",
					RawOutput:   output,
				}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "OK — SYN cookies are enabled", RawOutput: output}
		},
	}
}

func kernelICMPIgnore() CheckDefinition {
	return CheckDefinition{
		ID:       "kernel_icmp_ignore_broadcasts",
		Category: "kernel",
		Title:    "ICMP Broadcast Echo Ignored",
		Command:  `sysctl -n net.ipv4.icmp_echo_ignore_broadcasts 2>/dev/null || echo 'unknown'`,
		Severity: "medium",
		CISID:    "3.2.2",
		CISLevel: 1,
		Risk:     "Ignoring ICMP broadcast echoes prevents the server from being used in Smurf DDoS amplification attacks.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Ignore broadcast pings: 'sysctl -w net.ipv4.icmp_echo_ignore_broadcasts=1' and persist in /etc/sysctl.conf",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check ICMP broadcast setting", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if trimmed != "1" {
				return CheckResult{
					Status:      "fail",
					Severity:    "medium",
					Description: "ICMP broadcast echo is not ignored — server could be used in DDoS amplification",
					RawOutput:   output,
				}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "OK — ICMP broadcast echoes are ignored", RawOutput: output}
		},
	}
}

func kernelMartianPackets() CheckDefinition {
	return CheckDefinition{
		ID:       "kernel_martian_packets",
		Category: "kernel",
		Title:    "Reverse Path Filtering (Martian Packets)",
		Command:  `sysctl -n net.ipv4.conf.all.rp_filter 2>/dev/null || echo 'unknown'`,
		Severity: "medium",
		CISID:    "3.2.3",
		CISLevel: 1,
		Risk:     "Without reverse path filtering, the server may accept packets with spoofed source IPs, enabling IP spoofing attacks.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Enable reverse path filtering: 'sysctl -w net.ipv4.conf.all.rp_filter=1' and persist in /etc/sysctl.conf",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check reverse path filtering", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if trimmed != "1" {
				return CheckResult{
					Status:      "fail",
					Severity:    "medium",
					Description: "Reverse path filtering is not enabled — susceptible to IP spoofing",
					RawOutput:   output,
				}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "OK — reverse path filtering is enabled", RawOutput: output}
		},
	}
}
