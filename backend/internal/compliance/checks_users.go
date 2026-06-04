package compliance

import (
	"strconv"
	"strings"
)

// User checks — CIS Benchmark Section 5 & 6
func UserChecks() []CheckDefinition {
	return []CheckDefinition{
		passwdEmpty(),
		userUIDZero(),
		userPasswordAging(),
		userPasswordLength(),
		userDuplicateUIDs(),
		userHomePerms(),
		userSudoersConfig(),
	}
}

func passwdEmpty() CheckDefinition {
	return CheckDefinition{
		ID:       "users_empty_passwords",
		Category: "users",
		Title:    "Users with Empty/Disabled Passwords",
		Command:  `awk -F: '($2 == "" || $2 == "!") {print $1}' /etc/shadow 2>/dev/null | wc -l`,
		Severity: "high",
		CISID:    "5.5.1",
		CISLevel: 1,
		Risk:     "Accounts with empty passwords can be accessed without authentication, granting immediate shell access.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Set strong passwords: 'passwd <username>'. Lock unused accounts: 'usermod -L <username>'.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check empty passwords", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			count, parseErr := strconv.Atoi(trimmed)
			if parseErr != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not parse count", RawOutput: output}
			}
			if count > 0 {
				return CheckResult{
					Status:      "fail",
					Severity:    "high",
					Description: "Found " + trimmed + " user(s) with empty or disabled passwords",
					RawOutput:   output,
				}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "OK — no users with empty passwords", RawOutput: output}
		},
	}
}

func userUIDZero() CheckDefinition {
	return CheckDefinition{
		ID:       "users_uid_zero",
		Category: "users",
		Title:    "Accounts with UID 0",
		Command:  `awk -F: '($3 == 0) {print $1}' /etc/passwd 2>/dev/null | grep -v '^root$' | wc -l`,
		Severity: "critical",
		CISID:    "5.4.1",
		CISLevel: 1,
		Risk:     "Any account with UID 0 has root privileges. Extra UID 0 accounts bypass audit controls.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Remove any non-root accounts with UID 0: 'usermod -u <new_uid> <username>' or delete the account.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check UID 0 accounts", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			count, parseErr := strconv.Atoi(trimmed)
			if parseErr != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not parse count", RawOutput: output}
			}
			if count > 0 {
				return CheckResult{
					Status:      "fail",
					Severity:    "critical",
					Description: "Found " + trimmed + " non-root account(s) with UID 0 (root privileges)",
					RawOutput:   output,
				}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "OK — only root has UID 0", RawOutput: output}
		},
	}
}

func userPasswordAging() CheckDefinition {
	return CheckDefinition{
		ID:       "users_password_aging",
		Category: "users",
		Title:    "Password Aging (PASS_MAX_DAYS)",
		Command:  `grep -i "^PASS_MAX_DAYS" /etc/login.defs 2>/dev/null || echo "not-set"`,
		Severity: "medium",
		CISID:    "5.4.1.1",
		CISLevel: 1,
		Risk:     "Without password aging, users may keep the same password indefinitely, increasing the risk of credential compromise.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Set 'PASS_MAX_DAYS 90' in /etc/login.defs and run 'chage --maxdays 90 <user>' for existing users.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil || strings.TrimSpace(output) == "not-set" {
				return CheckResult{Status: "warn", Severity: "low", Description: "PASS_MAX_DAYS not configured — use default system setting", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				val, parseErr := strconv.Atoi(parts[1])
				if parseErr == nil && val > 90 {
					return CheckResult{
						Status:      "fail",
						Severity:    "medium",
						Description: "Password maximum age is " + strconv.Itoa(val) + " days (should be ≤ 90)",
						RawOutput:   output,
					}
				}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "OK — password aging is configured", RawOutput: output}
		},
	}
}

func userPasswordLength() CheckDefinition {
	return CheckDefinition{
		ID:       "users_password_length",
		Category: "users",
		Title:    "Password Minimum Length",
		Command:  `grep -i "^PASS_MIN_LEN" /etc/login.defs 2>/dev/null || echo "not-set"`,
		Severity: "medium",
		CISID:    "5.4.1.2",
		CISLevel: 1,
		Risk:     "Short passwords are vulnerable to brute-force attacks. Modern guidelines recommend at least 14 characters.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Set 'PASS_MIN_LEN 14' in /etc/login.defs.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil || strings.TrimSpace(output) == "not-set" {
				return CheckResult{Status: "warn", Severity: "low", Description: "PASS_MIN_LEN not explicitly configured", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				val, parseErr := strconv.Atoi(parts[1])
				if parseErr == nil && val < 14 {
					return CheckResult{
						Status:      "fail",
						Severity:    "medium",
						Description: "Minimum password length is " + strconv.Itoa(val) + " (should be ≥ 14)",
						RawOutput:   output,
					}
				}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "OK — minimum password length ≥ 14", RawOutput: output}
		},
	}
}

func userDuplicateUIDs() CheckDefinition {
	return CheckDefinition{
		ID:       "users_duplicate_uids",
		Category: "users",
		Title:    "Duplicate UIDs",
		Command:  `cut -d: -f3 /etc/passwd 2>/dev/null | sort | uniq -d | wc -l`,
		Severity: "medium",
		CISID:    "6.2.5",
		CISLevel: 1,
		Risk:     "Duplicate UIDs cause access control ambiguity — multiple users share the same privileges and file ownership.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Assign unique UIDs for each user with 'usermod -u <new_uid> <username>'.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check duplicate UIDs", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			count, parseErr := strconv.Atoi(trimmed)
			if parseErr != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not parse UID count", RawOutput: output}
			}
			if count > 0 {
				return CheckResult{
					Status:      "fail",
					Severity:    "medium",
					Description: "Found " + trimmed + " duplicate UID(s) in /etc/passwd",
					RawOutput:   output,
				}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "OK — no duplicate UIDs", RawOutput: output}
		},
	}
}

func userHomePerms() CheckDefinition {
	return CheckDefinition{
		ID:       "users_home_perms",
		Category: "users",
		Title:    "Home Directory Permissions",
		Command:  `find /home -maxdepth 1 -type d -perm /o+w 2>/dev/null | wc -l`,
		Severity: "medium",
		CISID:    "6.2.6",
		CISLevel: 2,
		Risk:     "World-writable home directories allow other users to view, modify, or delete personal files.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Restrict home directory permissions: 'chmod o-w /home/<user>' for each affected user.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check home permissions", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			count, parseErr := strconv.Atoi(trimmed)
			if parseErr != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not parse count", RawOutput: output}
			}
			if count > 0 {
				return CheckResult{
					Status:      "fail",
					Severity:    "medium",
					Description: "Found " + trimmed + " world-writable home director(ies)",
					RawOutput:   output,
				}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "OK — no world-writable home dirs", RawOutput: output}
		},
	}
}

func userSudoersConfig() CheckDefinition {
	return CheckDefinition{
		ID:       "users_sudoers_config",
		Category: "users",
		Title:    "Sudoers File Permissions",
		Command:  `stat -c %a /etc/sudoers 2>/dev/null; stat -c %a /etc/sudoers.d 2>/dev/null`,
		Severity: "high",
		CISID:    "5.3.1",
		CISLevel: 1,
		Risk:     "World-writable sudoers configuration allows any user to grant themselves root privileges.",
		References: []string{
			"https://www.cisecurity.org/benchmark/distribution_independent_linux",
		},
		Remediation: "Fix permissions: 'chmod 440 /etc/sudoers', 'chmod 440 /etc/sudoers.d'.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check sudoers permissions", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			lines := strings.Split(trimmed, "\n")
			for i, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				perms, parseErr := strconv.Atoi(line)
				if parseErr != nil {
					continue
				}
				// Check world-writable or group-writable
				if perms >= 644 {
					name := "/etc/sudoers"
					if i == 1 {
						name = "/etc/sudoers.d"
					}
					return CheckResult{
						Status:      "fail",
						Severity:    "high",
						Description: name + " has permissions " + line + " (should be 440)",
						RawOutput:   output,
					}
				}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "OK — sudoers permissions are correct", RawOutput: output}
		},
	}
}
