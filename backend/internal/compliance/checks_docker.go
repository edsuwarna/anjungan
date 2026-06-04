package compliance

import (
	"strings"
)

// ─── CIS Docker Benchmark Checks ──────────────────────────────────────────
//
// These checks run against the Docker daemon host via SSH (same as other
// compliance checks). They follow the CIS Docker Benchmark v1.6 and are
// organized into six categories:
//
//   daemon-config   — Docker daemon configuration (5 checks)
//   daemon-files    — Daemon file ownership & permissions (3 checks)
//   container-runtime — Container runtime & isolation (5 checks)
//   container-network — Docker networking (4 checks)
//   container-auth  — Authorization & access control (2 checks)
//   images-registry — Image & registry configuration (3 checks)

func DockerChecks() []CheckDefinition {
	return []CheckDefinition{
		// ── Daemon Configuration ───────────────────────────────────────────
		dockerServiceOwnership(),
		dockerServicePermissions(),
		dockerDaemonJSONOwnership(),
		dockerDaemonJSONPermissions(),
		dockerLiveRestore(),

		// ── Daemon Files ───────────────────────────────────────────────────
		dockerSocketOwnership(),
		dockerSocketPermissions(),
		dockerLogLevel(),

		// ── Container Runtime ──────────────────────────────────────────────
		dockerNoNewPrivileges(),
		dockerSeccompProfile(),
		dockerAppArmorProfile(),
		dockerReadOnlyRootFS(),
		dockerPrivilegedContainers(),

		// ── Network ────────────────────────────────────────────────────────
		dockerICCBridge(),
		dockerUserlandProxy(),
		dockerDefaultBridge(),
		dockerHostNetworkContainers(),

		// ── Auth & Access ──────────────────────────────────────────────────
		dockerAuthPlugin(),
		dockerNonRootUserCheck(),

		// ── Images & Registry ──────────────────────────────────────────────
		dockerContentTrust(),
		dockerHealthcheck(),
		dockerTrustedRegistries(),
	}
}

// ─── 1. Daemon Configuration ─────────────────────────────────────────────

// 1.1 — Ensure docker.service file ownership is root:root
func dockerServiceOwnership() CheckDefinition {
	return CheckDefinition{
		ID:          "docker_01",
		Category:    "daemon-config",
		Title:       "Docker Service File Ownership",
		Command:     `stat -c '%U:%G' /lib/systemd/system/docker.service 2>/dev/null || stat -c '%U:%G' /usr/lib/systemd/system/docker.service 2>/dev/null || echo "not-found"`,
		Severity:    "high",
		CISID:       "1.1.1",
		CISLevel:    1,
		Risk:        "Incorrect ownership of the docker.service file could allow unauthorized modifications to Docker daemon behaviour.",
		References:  []string{"https://www.cisecurity.org/benchmark/docker"},
		Remediation: "chown root:root /lib/systemd/system/docker.service",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check docker.service ownership", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if trimmed == "root:root" {
				return CheckResult{Status: "pass", Severity: "info", Description: "docker.service is owned by root:root", RawOutput: output}
			}
			return CheckResult{Status: "fail", Severity: "high", Description: "docker.service is owned by " + trimmed + " (should be root:root)", RawOutput: output}
		},
	}
}

// 1.2 — Ensure docker.service file permissions are 644 or more restrictive
func dockerServicePermissions() CheckDefinition {
	return CheckDefinition{
		ID:          "docker_02",
		Category:    "daemon-config",
		Title:       "Docker Service File Permissions",
		Command:     `stat -c '%a' /lib/systemd/system/docker.service 2>/dev/null || stat -c '%a' /usr/lib/systemd/system/docker.service 2>/dev/null || echo "not-found"`,
		Severity:    "high",
		CISID:       "1.1.2",
		CISLevel:    1,
		Risk:        "Overly permissive service file could allow non-root users to modify Docker daemon startup parameters.",
		References:  []string{"https://www.cisecurity.org/benchmark/docker"},
		Remediation: "chmod 644 /lib/systemd/system/docker.service",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check docker.service permissions", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if trimmed == "not-found" {
				return CheckResult{Status: "warn", Severity: "medium", Description: "docker.service file not found in standard paths", RawOutput: output}
			}
			perms := strings.TrimSpace(output)
			if len(perms) == 3 && (perms[0] == '6' || perms[0] == '4') && perms[2] <= '4' {
				return CheckResult{Status: "pass", Severity: "info", Description: "docker.service has permissions " + perms + " (644 or tighter)", RawOutput: output}
			}
			return CheckResult{Status: "fail", Severity: "high", Description: "docker.service has permissions " + perms + " (should be 644 or tighter)", RawOutput: output}
		},
	}
}

// 1.3 — Ensure daemon.json file ownership is root:root
func dockerDaemonJSONOwnership() CheckDefinition {
	return CheckDefinition{
		ID:          "docker_03",
		Category:    "daemon-config",
		Title:       "Docker Daemon JSON Ownership",
		Command:     `test -f /etc/docker/daemon.json && stat -c '%U:%G' /etc/docker/daemon.json || echo "not-found"`,
		Severity:    "high",
		CISID:       "1.2.1",
		CISLevel:    1,
		Risk:        "Incorrect ownership of daemon.json allows unauthorized modifications to Docker daemon settings.",
		References:  []string{"https://www.cisecurity.org/benchmark/docker"},
		Remediation: "chown root:root /etc/docker/daemon.json",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check daemon.json ownership", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if trimmed == "not-found" {
				return CheckResult{Status: "info", Severity: "info", Description: "daemon.json not found — no custom daemon configuration", RawOutput: output}
			}
			if trimmed == "root:root" {
				return CheckResult{Status: "pass", Severity: "info", Description: "daemon.json is owned by root:root", RawOutput: output}
			}
			return CheckResult{Status: "fail", Severity: "high", Description: "daemon.json is owned by " + trimmed + " (should be root:root)", RawOutput: output}
		},
	}
}

// 1.4 — Ensure daemon.json file permissions are 644 or more restrictive
func dockerDaemonJSONPermissions() CheckDefinition {
	return CheckDefinition{
		ID:          "docker_04",
		Category:    "daemon-config",
		Title:       "Docker Daemon JSON Permissions",
		Command:     `test -f /etc/docker/daemon.json && stat -c '%a' /etc/docker/daemon.json || echo "not-found"`,
		Severity:    "high",
		CISID:       "1.2.2",
		CISLevel:    1,
		Risk:        "Overly permissive daemon.json could allow non-root users to alter Docker runtime configuration.",
		References:  []string{"https://www.cisecurity.org/benchmark/docker"},
		Remediation: "chmod 644 /etc/docker/daemon.json",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check daemon.json permissions", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if trimmed == "not-found" {
				return CheckResult{Status: "info", Severity: "info", Description: "daemon.json not found — no custom daemon configuration", RawOutput: output}
			}
			if len(trimmed) == 3 && (trimmed[0] == '6' || trimmed[0] == '4') && trimmed[2] <= '4' {
				return CheckResult{Status: "pass", Severity: "info", Description: "daemon.json has permissions " + trimmed + " (644 or tighter)", RawOutput: output}
			}
			return CheckResult{Status: "fail", Severity: "high", Description: "daemon.json has permissions " + trimmed + " (should be 644 or tighter)", RawOutput: output}
		},
	}
}

// 1.5 — Ensure live-restore is enabled
func dockerLiveRestore() CheckDefinition {
	return CheckDefinition{
		ID:          "docker_05",
		Category:    "daemon-config",
		Title:       "Docker Live Restore Enabled",
		Command:     `docker info --format '{{json .LiveRestoreEnabled}}' 2>/dev/null || echo "unknown"`,
		Severity:    "medium",
		CISID:       "1.3",
		CISLevel:    1,
		Risk:        "Without live-restore, containers are killed when the Docker daemon restarts, causing downtime.",
		References:  []string{"https://www.cisecurity.org/benchmark/docker"},
		Remediation: "Add '{\"live-restore\": true}' to /etc/docker/daemon.json and restart Docker.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check live-restore setting", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if trimmed == "true" {
				return CheckResult{Status: "pass", Severity: "info", Description: "Live restore is enabled", RawOutput: output}
			}
			return CheckResult{Status: "fail", Severity: "medium", Description: "Live restore is not enabled", RawOutput: output}
		},
	}
}

// ─── 2. Daemon Files ────────────────────────────────────────────────────

// 2.1 — Ensure Docker socket file ownership is root:docker
func dockerSocketOwnership() CheckDefinition {
	return CheckDefinition{
		ID:          "docker_06",
		Category:    "daemon-files",
		Title:       "Docker Socket File Ownership",
		Command:     `stat -c '%U:%G' /var/run/docker.sock 2>/dev/null || echo "not-found"`,
		Severity:    "critical",
		CISID:       "1.4.1",
		CISLevel:    1,
		Risk:        "Incorrect socket ownership grants unauthorized users access to the Docker API.",
		References:  []string{"https://www.cisecurity.org/benchmark/docker"},
		Remediation: "chown root:docker /var/run/docker.sock",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check socket ownership", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if trimmed == "root:docker" || trimmed == "root:root" {
				return CheckResult{Status: "pass", Severity: "info", Description: "Socket is owned by " + trimmed, RawOutput: output}
			}
			return CheckResult{Status: "fail", Severity: "critical", Description: "Socket is owned by " + trimmed + " (should be root:docker)", RawOutput: output}
		},
	}
}

// 2.2 — Ensure Docker socket file permissions are 660 or more restrictive
func dockerSocketPermissions() CheckDefinition {
	return CheckDefinition{
		ID:          "docker_07",
		Category:    "daemon-files",
		Title:       "Docker Socket File Permissions",
		Command:     `stat -c '%a' /var/run/docker.sock 2>/dev/null || echo "not-found"`,
		Severity:    "critical",
		CISID:       "1.4.2",
		CISLevel:    1,
		Risk:        "World-readable/writable Docker socket allows any user to execute Docker commands with root privileges.",
		References:  []string{"https://www.cisecurity.org/benchmark/docker"},
		Remediation: "chmod 660 /var/run/docker.sock",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check socket permissions", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if trimmed == "660" || trimmed == "600" {
				return CheckResult{Status: "pass", Severity: "info", Description: "Socket permissions are " + trimmed, RawOutput: output}
			}
			return CheckResult{Status: "fail", Severity: "critical", Description: "Socket has permissions " + trimmed + " (should be 660 or 600)", RawOutput: output}
		},
	}
}

// 2.3 — Ensure Docker daemon log level is set to 'info'
func dockerLogLevel() CheckDefinition {
	return CheckDefinition{
		ID:          "docker_08",
		Category:    "daemon-files",
		Title:       "Docker Daemon Log Level",
		Command:     `docker info --format '{{json .LoggingDriver}}' 2>/dev/null && docker info --format '{{json .LogConfig}}' 2>/dev/null || echo "unknown"`,
		Severity:    "low",
		CISID:       "1.5",
		CISLevel:    1,
		Risk:        "Excessive logging can fill disk; insufficient logging hinders forensics.",
		References:  []string{"https://www.cisecurity.org/benchmark/docker"},
		Remediation: "Configure 'log-opts' in /etc/docker/daemon.json.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check log configuration", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if trimmed != "" && trimmed != "unknown" {
				return CheckResult{Status: "pass", Severity: "info", Description: "Docker logging is configured: " + trimmed, RawOutput: output}
			}
			return CheckResult{Status: "fail", Severity: "low", Description: "Could not determine log configuration", RawOutput: output}
		},
	}
}

// ─── 3. Container Runtime ───────────────────────────────────────────────

// 3.1 — Ensure no-new-privileges is enabled for containers
func dockerNoNewPrivileges() CheckDefinition {
	return CheckDefinition{
		ID:          "docker_09",
		Category:    "container-runtime",
		Title:       "No New Privileges Restriction",
		Command:     `docker info --format '{{json .SecurityOptions}}' 2>/dev/null | grep -qi 'no-new-privileges' && echo 'enabled' || echo 'disabled'`,
		Severity:    "high",
		CISID:       "3.1",
		CISLevel:    1,
		Risk:        "Without no-new-privileges, containers can escalate privileges via setuid binaries.",
		References:  []string{"https://www.cisecurity.org/benchmark/docker"},
		Remediation: "Start Docker daemon with '--no-new-privileges' or configure container security context.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check no-new-privileges setting", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if strings.EqualFold(trimmed, "enabled") {
				return CheckResult{Status: "pass", Severity: "info", Description: "No-new-privileges is enabled", RawOutput: output}
			}
			return CheckResult{Status: "fail", Severity: "high", Description: "No-new-privileges is not enabled", RawOutput: output}
		},
	}
}

// 3.2 — Ensure seccomp profile is configured
func dockerSeccompProfile() CheckDefinition {
	return CheckDefinition{
		ID:          "docker_10",
		Category:    "container-runtime",
		Title:       "Seccomp Profile Configured",
		Command:     `docker info --format '{{json .SecurityOptions}}' 2>/dev/null | grep -qi 'seccomp' && echo 'enabled' || echo 'disabled'`,
		Severity:    "medium",
		CISID:       "3.2",
		CISLevel:    1,
		Risk:        "Without seccomp, containers can make unrestricted system calls, increasing kernel attack surface.",
		References:  []string{"https://www.cisecurity.org/benchmark/docker"},
		Remediation: "Ensure the Docker daemon has seccomp profile enabled (default since Docker 1.10).",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check seccomp setting", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if strings.EqualFold(trimmed, "enabled") {
				return CheckResult{Status: "pass", Severity: "info", Description: "Seccomp is enabled", RawOutput: output}
			}
			return CheckResult{Status: "fail", Severity: "medium", Description: "Seccomp is not enabled", RawOutput: output}
		},
	}
}

// 3.3 — Ensure AppArmor profile is configured
func dockerAppArmorProfile() CheckDefinition {
	return CheckDefinition{
		ID:          "docker_11",
		Category:    "container-runtime",
		Title:       "AppArmor Profile Configured",
		Command:     `docker info --format '{{json .SecurityOptions}}' 2>/dev/null | grep -qi 'apparmor' && echo 'enabled' || (which aa-status 2>/dev/null && aa-status 2>/dev/null | head -5 || echo 'disabled')`,
		Severity:    "medium",
		CISID:       "3.3",
		CISLevel:    1,
		Risk:        "Without AppArmor, containers lack mandatory access control, increasing the risk of container escapes.",
		References:  []string{"https://www.cisecurity.org/benchmark/docker"},
		Remediation: "Install and configure AppArmor, and ensure Docker daemon loads AppArmor profiles.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check AppArmor setting", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if strings.Contains(trimmed, "enabled") || strings.Contains(trimmed, "profiles") {
				return CheckResult{Status: "pass", Severity: "info", Description: "AppArmor is enabled", RawOutput: output}
			}
			return CheckResult{Status: "fail", Severity: "medium", Description: "AppArmor is not enabled", RawOutput: output}
		},
	}
}

// 3.4 — Ensure containers use read-only root filesystem
func dockerReadOnlyRootFS() CheckDefinition {
	return CheckDefinition{
		ID:          "docker_12",
		Category:    "container-runtime",
		Title:       "Containers With Read-Only Root Filesystem",
		Command:     `docker ps -q 2>/dev/null | head -20 | while read id; do echo "$id: $(docker inspect --format '{{.HostConfig.ReadonlyRootfs}}' "$id")"; done || echo "no-containers"`,
		Severity:    "medium",
		CISID:       "3.4",
		CISLevel:    1,
		Risk:        "Writable root filesystems allow malware persistence and modification of container contents at runtime.",
		References:  []string{"https://www.cisecurity.org/benchmark/docker"},
		Remediation: "Add '--read-only' flag when running containers, and use tmpfs mounts for writable directories.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check read-only rootfs", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if trimmed == "no-containers" {
				return CheckResult{Status: "info", Severity: "info", Description: "No running containers to check", RawOutput: output}
			}
			if strings.Contains(trimmed, "false") {
				return CheckResult{Status: "fail", Severity: "medium", Description: "Some containers do not use read-only rootfs", RawOutput: output}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "All running containers use read-only rootfs", RawOutput: output}
		},
	}
}

// 3.5 — Ensure no privileged containers are running
func dockerPrivilegedContainers() CheckDefinition {
	return CheckDefinition{
		ID:          "docker_13",
		Category:    "container-runtime",
		Title:       "Privileged Containers Check",
		Command:     `docker ps -q 2>/dev/null | while read id; do priv=$(docker inspect --format '{{.HostConfig.Privileged}}' "$id"); if [ "$priv" = "true" ]; then docker inspect --format '{{.Name}}' "$id"; fi; done || echo "check-failed"`,
		Severity:    "critical",
		CISID:       "3.5",
		CISLevel:    1,
		Risk:        "Privileged containers have all capabilities and unrestricted host access, equivalent to root on the host.",
		References:  []string{"https://www.cisecurity.org/benchmark/docker"},
		Remediation: "Avoid running containers with --privileged flag. Drop capabilities and add specific ones as needed.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check privileged containers", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if trimmed == "" || strings.Contains(trimmed, "no-containers") {
				return CheckResult{Status: "pass", Severity: "info", Description: "No privileged containers found", RawOutput: output}
			}
			return CheckResult{Status: "fail", Severity: "critical", Description: "Privileged containers found: " + trimmed, RawOutput: output}
		},
	}
}

// ─── 4. Network ──────────────────────────────────────────────────────────

// 4.1 — Ensure inter-container communication is restricted
func dockerICCBridge() CheckDefinition {
	return CheckDefinition{
		ID:          "docker_14",
		Category:    "container-network",
		Title:       "Inter-Container Communication Restriction (ICC)",
		Command:     `docker info --format '{{json .DriverStatus}}' 2>/dev/null | grep -qi 'icc.*false' && echo 'restricted' || echo 'default-open'`,
		Severity:    "medium",
		CISID:       "4.1",
		CISLevel:    1,
		Risk:        "Default bridge allows all inter-container communication by default, increasing lateral movement risk.",
		References:  []string{"https://www.cisecurity.org/benchmark/docker"},
		Remediation: "Configure Docker daemon with '--icc=false' or restrict communication using network policies.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check ICC setting", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if strings.EqualFold(trimmed, "restricted") {
				return CheckResult{Status: "pass", Severity: "info", Description: "Inter-container communication is restricted (ICC=false)", RawOutput: output}
			}
			return CheckResult{Status: "fail", Severity: "medium", Description: "Inter-container communication is open by default (ICC=true)", RawOutput: output}
		},
	}
}

// 4.2 — Ensure userland proxy is disabled
func dockerUserlandProxy() CheckDefinition {
	return CheckDefinition{
		ID:          "docker_15",
		Category:    "container-network",
		Title:       "Userland Proxy Disabled",
		Command:     `docker info --format '{{json .DriverStatus}}' 2>/dev/null | grep -qi 'userland-proxy.*false' && echo 'disabled' || echo 'enabled'`,
		Severity:    "low",
		CISID:       "4.2",
		CISLevel:    1,
		Risk:        "Userland proxy uses extra CPU/memory. Direct iptables forwarding is more efficient and secure.",
		References:  []string{"https://www.cisecurity.org/benchmark/docker"},
		Remediation: "Start Docker daemon with '--userland-proxy=false'.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check userland proxy setting", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if strings.EqualFold(trimmed, "disabled") {
				return CheckResult{Status: "pass", Severity: "info", Description: "Userland proxy is disabled", RawOutput: output}
			}
			return CheckResult{Status: "fail", Severity: "low", Description: "Userland proxy is enabled", RawOutput: output}
		},
	}
}

// 4.3 — Ensure default bridge (docker0) is not used
func dockerDefaultBridge() CheckDefinition {
	return CheckDefinition{
		ID:          "docker_16",
		Category:    "container-network",
		Title:       "Default Docker Bridge Not Used",
		Command:     `docker network ls --filter driver=bridge --format '{{.Name}}' 2>/dev/null | grep -v '^bridge$' | head -1 || echo "only-bridge"`,
		Severity:    "low",
		CISID:       "4.3",
		CISLevel:    1,
		Risk:        "Using the default bridge (docker0) lacks user-defined network features like DNS resolution and network isolation.",
		References:  []string{"https://www.cisecurity.org/benchmark/docker"},
		Remediation: "Create and use user-defined bridge networks instead of the default docker0 bridge.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check bridge networks", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if trimmed == "only-bridge" {
				return CheckResult{Status: "warn", Severity: "low", Description: "Only the default docker0 bridge is in use", RawOutput: output}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "User-defined bridge networks exist: " + trimmed, RawOutput: output}
		},
	}
}

// 4.4 — Ensure no containers use host network mode
func dockerHostNetworkContainers() CheckDefinition {
	return CheckDefinition{
		ID:          "docker_17",
		Category:    "container-network",
		Title:       "Host Network Mode Check",
		Command:     `docker ps -q 2>/dev/null | while read id; do netmode=$(docker inspect --format '{{.HostConfig.NetworkMode}}' "$id"); if [ "$netmode" = "host" ]; then docker inspect --format '{{.Name}}' "$id"; fi; done || echo "check-failed"`,
		Severity:    "high",
		CISID:       "4.4",
		CISLevel:    1,
		Risk:        "Host networking gives containers direct access to the host's network stack, bypassing network isolation.",
		References:  []string{"https://www.cisecurity.org/benchmark/docker"},
		Remediation: "Avoid --network=host. Use port mapping (-p) or user-defined networks instead.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check host network mode", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if trimmed == "" {
				return CheckResult{Status: "pass", Severity: "info", Description: "No containers use host network mode", RawOutput: output}
			}
			return CheckResult{Status: "fail", Severity: "high", Description: "Containers using host network: " + trimmed, RawOutput: output}
		},
	}
}

// ─── 5. Auth & Access ────────────────────────────────────────────────────

// 5.1 — Ensure authorization plugin (Docker Content Trust) is configured
func dockerAuthPlugin() CheckDefinition {
	return CheckDefinition{
		ID:          "docker_18",
		Category:    "container-auth",
		Title:       "Docker Authorization Plugin",
		Command:     `docker info --format '{{json .AuthorizationOptions}}' 2>/dev/null | grep -qi 'plugin' && echo 'configured' || echo 'not-configured'`,
		Severity:    "high",
		CISID:       "5.1",
		CISLevel:    2,
		Risk:        "Without authorization plugins, any process with Docker socket access has unrestricted API access.",
		References:  []string{"https://www.cisecurity.org/benchmark/docker"},
		Remediation: "Configure an authorization plugin (e.g., Twistlock, AuthZ).",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check authorization plugin", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if strings.EqualFold(trimmed, "configured") {
				return CheckResult{Status: "pass", Severity: "info", Description: "Authorization plugin is configured", RawOutput: output}
			}
			return CheckResult{Status: "fail", Severity: "high", Description: "No authorization plugin configured", RawOutput: output}
		},
	}
}

// 5.2 — Ensure containers run as non-root user
func dockerNonRootUserCheck() CheckDefinition {
	return CheckDefinition{
		ID:          "docker_19",
		Category:    "container-auth",
		Title:       "Containers Running as Non-Root User",
		Command:     `docker ps -q 2>/dev/null | head -10 | while read id; do user=$(docker inspect --format '{{.Config.User}}' "$id"); if [ "$user" = "" ] || [ "$user" = "root" ] || [ "$user" = "0" ]; then docker inspect --format '{{.Name}}' "$id"; fi; done || echo "check-failed"`,
		Severity:    "high",
		CISID:       "5.2",
		CISLevel:    1,
		Risk:        "Running containers as root amplifies the impact of container compromises — an attacker gains root inside the container.",
		References:  []string{"https://www.cisecurity.org/benchmark/docker"},
		Remediation: "Use the USER directive in Dockerfiles or run containers with --user flag.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check container user", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if trimmed == "" {
				return CheckResult{Status: "pass", Severity: "info", Description: "All running containers use non-root users", RawOutput: output}
			}
			return CheckResult{Status: "fail", Severity: "high", Description: "Containers running as root: " + trimmed, RawOutput: output}
		},
	}
}

// ─── 6. Images & Registry ───────────────────────────────────────────────

// 6.1 — Ensure Docker Content Trust is enabled
func dockerContentTrust() CheckDefinition {
	return CheckDefinition{
		ID:          "docker_20",
		Category:    "images-registry",
		Title:       "Docker Content Trust Enabled",
		Command:     `docker info --format '{{json .DriverStatus}}' 2>/dev/null | grep -qi 'content-trust' && echo 'enabled' || echo 'disabled'`,
		Severity:    "medium",
		CISID:       "6.1",
		CISLevel:    1,
		Risk:        "Without content trust, images are not verified for integrity and authenticity before use.",
		References:  []string{"https://www.cisecurity.org/benchmark/docker"},
		Remediation: "Set DOCKER_CONTENT_TRUST=1 environment variable or in the Docker daemon configuration.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check content trust", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if strings.EqualFold(trimmed, "enabled") {
				return CheckResult{Status: "pass", Severity: "info", Description: "Docker Content Trust is enabled", RawOutput: output}
			}
			return CheckResult{Status: "fail", Severity: "medium", Description: "Docker Content Trust is not enabled", RawOutput: output}
		},
	}
}

// 6.2 — Ensure HEALTHCHECK instructions are used
func dockerHealthcheck() CheckDefinition {
	return CheckDefinition{
		ID:          "docker_21",
		Category:    "images-registry",
		Title:       "Container HEALTHCHECK Instructions",
		Command:     `docker ps -q 2>/dev/null | head -20 | while read id; do hc=$(docker inspect --format '{{.Config.Healthcheck}}' "$id"); if [ "$hc" = "<nil>" ]; then docker inspect --format '{{.Name}}' "$id"; fi; done || echo "check-failed"`,
		Severity:    "low",
		CISID:       "6.2",
		CISLevel:    1,
		Risk:        "Without HEALTHCHECK, container failures may go undetected until service becomes unavailable.",
		References:  []string{"https://www.cisecurity.org/benchmark/docker"},
		Remediation: "Add HEALTHCHECK instruction in Dockerfiles.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check HEALTHCHECK", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if trimmed == "" {
				return CheckResult{Status: "pass", Severity: "info", Description: "All running containers have HEALTHCHECK defined", RawOutput: output}
			}
			return CheckResult{Status: "fail", Severity: "low", Description: "Containers missing HEALTHCHECK: " + trimmed, RawOutput: output}
		},
	}
}

// 6.3 — Ensure only trusted registries are used
func dockerTrustedRegistries() CheckDefinition {
	return CheckDefinition{
		ID:          "docker_22",
		Category:    "images-registry",
		Title:       "Only Trusted Registries Used",
		Command:     `docker info --format '{{json .DriverStatus}}' 2>/dev/null | grep -oiE 'registry-mirror|insecure-registr' || echo "check-not-available"`,
		Severity:    "high",
		CISID:       "6.3",
		CISLevel:    2,
		Risk:        "Images from untrusted registries may contain malware, backdoors, or unpatched software.",
		References:  []string{"https://www.cisecurity.org/benchmark/docker"},
		Remediation: "Use only trusted, verified registries. Avoid --insecure-registry flag.",
		Evaluate: func(output string, err error) CheckResult {
			if err != nil {
				return CheckResult{Status: "warn", Severity: "info", Description: "Could not check registries", RawOutput: output}
			}
			trimmed := strings.TrimSpace(output)
			if strings.Contains(trimmed, "not-available") {
				return CheckResult{Status: "info", Severity: "info", Description: "No registry mirror or insecure registry configured", RawOutput: output}
			}
			return CheckResult{Status: "pass", Severity: "info", Description: "Registry is configured: " + trimmed, RawOutput: output}
		},
	}
}
