package compliance

import (
	"strconv"
	"strings"
)

// Filesystem checks — CIS Benchmark Section 6
func FilesystemChecks() []CheckDefinition {
	return []CheckDefinition{
		fsSuidHomeFiles(),
		fsWorldReadableSSH(),
		fsSSHKeyPerms(),
		fsShadowPerms(),
		fsPasswdPerms(),
		fsStickyBitTmp(),
		fsUnownedFiles(),
		fsSeparatePartition(),
	}
}

func fsSuidHomeFiles() CheckDefinition {
	return CheckDefinition{
		ID:       "fs_suid_home",
		Category: "filesystem",
		Title:    "SUID Files in /home",
		Command:  `find /home -type f -perm -4000 2>/dev/null | wc -l`,
		Severity: "medium",
		CISID:    "6.1.1",
		CISLevel: 1,
		Risk:     "SUID files in user-writable directories allow privilege escalation if an attacker gains access to a non-root user.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Investigate and remove SUID bit from files in /home. Check with 'find /home -perm -4000 -ls'.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check SUID files", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			count, parseErr := strconv.Atoi(trimmed)
			if parseErr != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not parse SUID count", RawOutput: output}
			}
			if count > 0 {
				return CheckResult{
					Status:      "fail",
					Severity:    "medium",
					Description: "Found " + trimmed + " SUID file(s) in /home — privilege escalation risk",
					RawOutput:   output,
				}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "OK — no SUID files in /home", RawOutput: output}
		},
	}
}

func fsWorldReadableSSH() CheckDefinition {
	return CheckDefinition{
		ID:       "fs_ssh_world_readable",
		Category: "filesystem",
		Title:    "World-Readable SSH Files",
		Command:  `find /root/.ssh -type f -perm /o+r 2>/dev/null | wc -l`,
		Severity: "critical",
		CISID:    "6.2.1",
		CISLevel: 1,
		Risk:     "World-readable SSH private keys can be stolen by any user or process on the system, enabling unauthorized access.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Restrict permissions: 'chmod 600 /root/.ssh/*'. SSH private keys must not be world-readable.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check SSH file permissions", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			count, parseErr := strconv.Atoi(trimmed)
			if parseErr != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not parse file count", RawOutput: output}
			}
			if count > 0 {
				return CheckResult{
					Status:      "fail",
					Severity:    "critical",
					Description: "Found " + trimmed + " world-readable file(s) in /root/.ssh/ — keys may be exposed",
					RawOutput:   output,
				}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "OK — no world-readable SSH files", RawOutput: output}
		},
	}
}

func fsSSHKeyPerms() CheckDefinition {
	return CheckDefinition{
		ID:       "fs_authorized_keys_perms",
		Category: "filesystem",
		Title:    "SSH Authorized Keys Permissions",
		Command:  `stat -c %a ~/.ssh/authorized_keys 2>/dev/null || echo 'no-file'`,
		Severity: "medium",
		CISID:    "6.2.2",
		CISLevel: 1,
		Risk:     "Overly permissive authorized_keys allows other users to modify authentication keys.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Set permissions: 'chmod 600 ~/.ssh/authorized_keys'.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check authorized_keys perms", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if trimmed == "no-file" {
				return CheckResult{Status: "pass", Severity: "info", Description: "OK — authorized_keys does not exist", RawOutput: output}
			}
			if trimmed != "600" {
				return CheckResult{
					Status:      "fail",
					Severity:    "medium",
					Description: "authorized_keys has permissions " + trimmed + " (should be 600)",
					RawOutput:   output,
				}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "OK — permissions are correct", RawOutput: output}
		},
	}
}

func fsShadowPerms() CheckDefinition {
	return CheckDefinition{
		ID:       "fs_shadow_perms",
		Category: "filesystem",
		Title:    "/etc/shadow Permissions",
		Command:  `stat -c %a /etc/shadow 2>/dev/null || echo 'no-file'`,
		Severity: "critical",
		CISID:    "6.2.3",
		CISLevel: 1,
		Risk:     "The shadow file contains hashed passwords. If world-readable, attackers can crack password hashes offline.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Set permissions: 'chmod 640 /etc/shadow' or 'chmod 600 /etc/shadow'.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check /etc/shadow permissions", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if trimmed == "no-file" {
				return CheckResult{Status: "warn", Severity: "high", Description: "/etc/shadow not found — system may use alternative auth", RawOutput: output}
			}
			perms, parseErr := strconv.Atoi(trimmed)
			if parseErr != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not parse permissions: " + trimmed, RawOutput: output}
			}
			// Check world-readable
			if perms >= 644 || (perms%10) > 0 {
				return CheckResult{
					Status:      "fail",
					Severity:    "critical",
					Description: "/etc/shadow has permissions " + trimmed + " — should be 640 or stricter",
					RawOutput:   output,
				}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "OK — /etc/shadow permissions are restricted (0" + trimmed + ")", RawOutput: output}
		},
	}
}

func fsPasswdPerms() CheckDefinition {
	return CheckDefinition{
		ID:       "fs_passwd_perms",
		Category: "filesystem",
		Title:    "/etc/passwd Permissions",
		Command:  `stat -c %a /etc/passwd 2>/dev/null || echo 'no-file'`,
		Severity: "medium",
		CISID:    "6.2.4",
		CISLevel: 1,
		Risk:     "World-writable /etc/passwd allows any user to create accounts or modify existing user entries.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Set permissions: 'chmod 644 /etc/passwd'.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check /etc/passwd permissions", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if trimmed == "no-file" {
				return CheckResult{Status: "warn", Severity: "high", Description: "/etc/passwd not found", RawOutput: output}
			}
			if trimmed != "644" {
				return CheckResult{
					Status:      "fail",
					Severity:    "medium",
					Description: "/etc/passwd has permissions " + trimmed + " (should be 644)",
					RawOutput:   output,
				}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "OK — /etc/passwd permissions are correct (644)", RawOutput: output}
		},
	}
}

func fsStickyBitTmp() CheckDefinition {
	return CheckDefinition{
		ID:       "fs_sticky_bit_tmp",
		Category: "filesystem",
		Title:    "Sticky Bit on /tmp",
		Command:  `stat -c %a /tmp 2>/dev/null`,
		Severity: "high",
		CISID:    "6.1.2",
		CISLevel: 1,
		Risk:     "Without the sticky bit, any user can delete or rename files in /tmp owned by other users, enabling data tampering.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Set sticky bit: 'chmod +t /tmp'.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check /tmp sticky bit", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			// Sticky bit = 1xxx, check first digit for 1
			if len(trimmed) >= 4 && trimmed[0] != '1' {
				return CheckResult{
					Status:      "fail",
					Severity:    "high",
					Description: "Sticky bit is not set on /tmp (permissions " + trimmed + ")",
					RawOutput:   output,
				}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "OK — sticky bit is set on /tmp", RawOutput: output}
		},
	}
}

func fsUnownedFiles() CheckDefinition {
	return CheckDefinition{
		ID:       "fs_unowned_files",
		Category: "filesystem",
		Title:    "Unowned Files and Directories",
		Command:  `find / -nouser -o -nogroup 2>/dev/null | wc -l; find /home /tmp /var/tmp \( -nouser -o -nogroup \) 2>/dev/null | wc -l`,
		Severity: "medium",
		CISID:    "6.1.3",
		CISLevel: 2,
		Risk:     "Files without a valid owner/group may indicate a deleted user, or can be exploited for privilege escalation.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Investigate unowned files: 'find / -nouser -o -nogroup -ls'. Assign ownership or remove orphaned files.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check unowned files", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			lines := strings.SplitN(trimmed, "\n", 2)
			total := strings.TrimSpace(lines[0])
			critical := "0"
			if len(lines) > 1 {
				critical = strings.TrimSpace(lines[1])
			}
			totalCount, _ := strconv.Atoi(total)
			critCount, _ := strconv.Atoi(critical)

			if critCount > 0 {
				return CheckResult{
					Status:      "fail",
					Severity:    "medium",
					Description: "Found " + critical + " unowned files in user-writable directories, " + total + " total system-wide",
					RawOutput:   output,
				}
			}
			if totalCount > 0 {
				return CheckResult{
					Status:      "warn",
					Severity:    "low",
					Description: "Found " + total + " unowned files (none in user-writable dirs)",
					RawOutput:   output,
				}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "OK — no unowned files found", RawOutput: output}
		},
	}
}

func fsSeparatePartition() CheckDefinition {
	return CheckDefinition{
		ID:       "fs_separate_partition",
		Category: "filesystem",
		Title:    "Separate Partitions (/tmp, /var, /home)",
		Command:  `mount | grep -E ' /tmp | /var | /home ' | wc -l`,
		Severity: "medium",
		CISID:    "1.1.1",
		CISLevel: 2,
		Risk:     "Without separate partitions, /tmp can fill up root filesystem, and unconstrained growth of /var and /home can cause denial of service.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Create separate partitions for /tmp, /var, and /home with noexec,nosuid mount options.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check mount points", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			count, parseErr := strconv.Atoi(trimmed)
			if parseErr != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not parse mount count", RawOutput: output}
			}
			if count < 2 {
				return CheckResult{
					Status:      "warn",
					Severity:    "medium",
					Description: "Only " + trimmed + " of /tmp, /var, /home are on separate partitions",
					RawOutput:   output,
				}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "OK — " + trimmed + " directories are on separate partitions", RawOutput: output}
		},
	}
}
