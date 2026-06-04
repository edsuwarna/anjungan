package compliance

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	sshtool "github.com/edsuwarna/anjungan/internal/infra/ssh"
)

// ─── Container Security Scanner ──────────────────────────────────────────
//
// Phase 2: Runs per-container security checks against the target server via
// SSH Docker commands. Each container gets a security score plus findings
// for vulnerabilities, runtime misconfigurations, and privilege issues.

// ContainerSecurityResult holds results for a single container.
type ContainerSecurityResult struct {
	ContainerID   string                   `json:"container_id"`
	ContainerName string                   `json:"container_name"`
	Image         string                   `json:"image"`
	Status        string                   `json:"status"`
	Score         int                      `json:"score"`
	Findings      []ContainerFinding       `json:"findings"`
	Badges        []string                 `json:"badges"`
}

// ContainerFinding is a single security finding for a container.
type ContainerFinding struct {
	CheckID     string `json:"check_id"`
	Title       string `json:"title"`
	Severity    string `json:"severity"` // critical, high, medium, low, info
	Status      string `json:"status"`  // pass, fail, warn
	Description string `json:"description"`
	Remediation string `json:"remediation"`
}

// ContainerScanSummary aggregates results across all containers on a server.
type ContainerScanSummary struct {
	ServerID             string                    `json:"server_id"`
	TotalContainers      int                       `json:"total_containers"`
	ScannedContainers    int                       `json:"scanned_containers"`
	AverageScore         int                       `json:"average_score"`
	TotalVulnerabilities int                       `json:"total_vulnerabilities"`
	TotalMisconfigs      int                       `json:"total_misconfigs"`
	Containers           []ContainerSecurityResult `json:"containers"`
}

// ContainerScanner performs security scans on Docker containers via SSH.
type ContainerScanner struct{}

// NewContainerScanner creates a new ContainerScanner.
func NewContainerScanner() *ContainerScanner {
	return &ContainerScanner{}
}

// Scan runs container security checks on the target server via SSH.
func (cs *ContainerScanner) Scan(ctx context.Context, sshCfg sshtool.Config) (*ContainerScanSummary, error) {
	// Step 1: List all containers on the server
	listCmd := `docker ps -a --format '{{.ID}}|{{.Names}}|{{.Image}}|{{.Status}}' 2>/dev/null || echo "docker-not-found"`
	raw, err := sshtool.RunCommand(ctx, sshCfg, listCmd)
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	trimmed := strings.TrimSpace(raw)
	if trimmed == "docker-not-found" || trimmed == "" {
		return &ContainerScanSummary{
			TotalContainers: 0, ScannedContainers: 0, AverageScore: 100,
		}, nil
	}

	lines := strings.Split(trimmed, "\n")
	var containers []ContainerSecurityResult
	totalScore := 0

	for _, line := range lines {
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}
		containerID := strings.TrimSpace(parts[0])
		containerName := strings.TrimSpace(parts[1])
		image := strings.TrimSpace(parts[2])
		status := strings.TrimSpace(parts[3])

		result, err := cs.scanContainer(ctx, sshCfg, containerID, containerName, image, status)
		if err != nil {
			// Skip failed container scans but continue with others
			continue
		}
		containers = append(containers, result)
		totalScore += result.Score
	}

	summary := &ContainerScanSummary{
		TotalContainers:   len(lines),
		ScannedContainers: len(containers),
		AverageScore:      100,
	}

	if len(containers) > 0 {
		summary.AverageScore = totalScore / len(containers)
	}

	// Calculate totals across all containers
	vulnCount := 0
	misconfigCount := 0
	summary.Containers = containers
	for _, c := range containers {
		for _, f := range c.Findings {
			if f.Status == "fail" {
				if f.Severity == "critical" || f.Severity == "high" {
					vulnCount++
				} else {
					misconfigCount++
				}
			}
		}
	}
	summary.TotalVulnerabilities = vulnCount
	summary.TotalMisconfigs = misconfigCount

	return summary, nil
}

func (cs *ContainerScanner) scanContainer(ctx context.Context, sshCfg sshtool.Config, id, name, image, status string) (ContainerSecurityResult, error) {
	// Get container inspect JSON
	inspectCmd := fmt.Sprintf(`docker inspect '%s' 2>/dev/null || echo '{"error":"inspect-failed"}'`, id)
	raw, err := sshtool.RunCommand(ctx, sshCfg, inspectCmd)
	if err != nil {
		return ContainerSecurityResult{}, fmt.Errorf("inspect container %s: %w", id, err)
	}

	// Parse inspect JSON (it returns an array)
	var inspectList []map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &inspectList); err != nil || len(inspectList) == 0 {
		return ContainerSecurityResult{}, fmt.Errorf("parse inspect for %s: %w", id, err)
	}
	inspect := inspectList[0]

	// Extract config and host config
	config, _ := inspect["Config"].(map[string]interface{})
	hostConfig, _ := inspect["HostConfig"].(map[string]interface{})
	networkSettings, _ := inspect["NetworkSettings"].(map[string]interface{})
	state, _ := inspect["State"].(map[string]interface{})

	var findings []ContainerFinding
	var badges []string

	// ── Check 1: Privileged mode ───────────────────────────────────────
	privCheck := checkPrivileged(hostConfig)
	findings = append(findings, privCheck)
	if privCheck.Status == "fail" {
		badges = append(badges, "🔓 privileged mode")
	} else if privCheck.Status == "pass" {
		badges = append(badges, "🔒 unprivileged ✓")
	}

	// ── Check 2: Root user ─────────────────────────────────────────────
	rootCheck := checkRootUser(config)
	findings = append(findings, rootCheck)
	if rootCheck.Status == "fail" {
		badges = append(badges, "🔓 runs as root")
	} else if rootCheck.Status == "pass" {
		badges = append(badges, "📦 non-root ✓")
	}

	// ── Check 3: Host network mode ─────────────────────────────────────
	netCheck := checkHostNetwork(hostConfig)
	findings = append(findings, netCheck)
	if netCheck.Status == "fail" {
		badges = append(badges, "🌐 host networking")
	} else {
		// Check port exposure
		portCheck := checkPortExposure(networkSettings)
		findings = append(findings, portCheck)
		if portCheck.Status == "fail" {
			badges = append(badges, "🔓 host ports")
		}
	}

	// ── Check 4: Seccomp profile ──────────────────────────────────────
	secCheck := checkSeccomp(hostConfig)
	findings = append(findings, secCheck)
	if secCheck.Status == "pass" {
		badges = append(badges, "🛡️ seccomp ✓")
	} else if secCheck.Status == "fail" {
		badges = append(badges, "🛡️ no seccomp ⚠")
	}

	// ── Check 5: Read-only root filesystem ─────────────────────────────
	roCheck := checkReadOnlyRootFS(hostConfig)
	findings = append(findings, roCheck)
	if roCheck.Status == "fail" {
		badges = append(badges, "📁 writable rootfs")
	} else if roCheck.Status == "pass" {
		badges = append(badges, "📁 read-only ✓")
	}

	// ── Check 6: Added capabilities ──────────────────────────────────
	capCheck := checkCapabilities(hostConfig)
	findings = append(findings, capCheck)
	if capCheck.Status == "pass" {
		badges = append(badges, "🔒 caps dropped ✓")
	}

	// ── Check 7: HEALTHCHECK ──────────────────────────────────────────
	healthCheck := checkHealthcheck(config)
	findings = append(findings, healthCheck)

	// ── Check 8: Bind mounts ──────────────────────────────────────────
	mountCheck := checkBindMounts(inspect)
	findings = append(findings, mountCheck)
	if mountCheck.Status == "fail" {
		badges = append(badges, "📁 bind mounts")
	}

	// ── Check 9: Resource limits ──────────────────────────────────────
	resCheck := checkResourceLimits(hostConfig)
	findings = append(findings, resCheck)
	if resCheck.Status == "pass" {
		badges = append(badges, "💾 limits set ✓")
	}

	// ── Check 10: Container running state ─────────────────────────────
	if state != nil {
		running, _ := state["Running"].(bool)
		if running {
			uptimeCheck := checkUptime(state)
			findings = append(findings, uptimeCheck)
		}
	}

	// Calculate score
	score := calculateContainerScore(findings)

	// Truncate container ID to short form
	shortID := id
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}

	// Clean container name
	cleanName := strings.TrimPrefix(name, "/")

	return ContainerSecurityResult{
		ContainerID:   shortID,
		ContainerName: cleanName,
		Image:         image,
		Status:        extractShortStatus(status),
		Score:         score,
		Findings:      findings,
		Badges:        badges,
	}, nil
}

// ScanSingleContainer runs container security checks on a single container
// identified by containerID on the target server via SSH.
func (cs *ContainerScanner) ScanSingleContainer(ctx context.Context, sshCfg sshtool.Config, containerID string) (ContainerSecurityResult, error) {
	// Get container info
	infoCmd := fmt.Sprintf(`docker inspect '%s' --format '{{.Name}}|{{.Config.Image}}|{{.Status}}|{{.State.Status}}' 2>/dev/null || echo "not-found"`, containerID)
	raw, err := sshtool.RunCommand(ctx, sshCfg, infoCmd)
	if err != nil {
		return ContainerSecurityResult{}, fmt.Errorf("get container info: %w", err)
	}

	trimmed := strings.TrimSpace(raw)
	if trimmed == "not-found" || trimmed == "" {
		return ContainerSecurityResult{}, fmt.Errorf("container %s not found on server", containerID)
	}

	parts := strings.SplitN(trimmed, "|", 4)
	containerName := strings.TrimPrefix(strings.TrimSpace(parts[0]), "/")
	image := ""
	status := ""
	if len(parts) > 1 {
		image = strings.TrimSpace(parts[1])
	}
	if len(parts) > 2 {
		status = strings.TrimSpace(parts[2])
	}

	return cs.scanContainer(ctx, sshCfg, containerID, containerName, image, status)
}

// ─── Individual check implementations ────────────────────────────────────

func checkPrivileged(hostConfig map[string]interface{}) ContainerFinding {
	priv, _ := hostConfig["Privileged"].(bool)
	if priv {
		return ContainerFinding{
			CheckID: "ctr_01", Title: "Privileged Mode",
			Severity: "critical", Status: "fail",
			Description: "Container runs in privileged mode with unrestricted host access",
			Remediation: "Remove --privileged flag; add specific capabilities instead",
		}
	}
	return ContainerFinding{
		CheckID: "ctr_01", Title: "Privileged Mode",
		Severity: "info", Status: "pass",
		Description: "Container is not privileged",
	}
}

func checkRootUser(config map[string]interface{}) ContainerFinding {
	user, _ := config["User"].(string)
	if user == "" || user == "root" || user == "0" {
		return ContainerFinding{
			CheckID: "ctr_02", Title: "Root User",
			Severity: "high", Status: "fail",
			Description: "Container runs as root user (User: '" + user + "')",
			Remediation: "Use USER directive in Dockerfile or --user flag",
		}
	}
	return ContainerFinding{
		CheckID: "ctr_02", Title: "Root User",
		Severity: "info", Status: "pass",
		Description: "Container runs as non-root user: " + user,
	}
}

func checkHostNetwork(hostConfig map[string]interface{}) ContainerFinding {
	netMode, _ := hostConfig["NetworkMode"].(string)
	if netMode == "host" {
		return ContainerFinding{
			CheckID: "ctr_03", Title: "Host Network Mode",
			Severity: "high", Status: "fail",
			Description: "Container uses host network mode, bypassing Docker network isolation",
			Remediation: "Use port mapping (-p) or user-defined networks instead of --network=host",
		}
	}
	return ContainerFinding{
		CheckID: "ctr_03", Title: "Host Network Mode",
		Severity: "info", Status: "pass",
		Description: "Container uses isolated network mode: " + netMode,
	}
}

func checkPortExposure(networkSettings map[string]interface{}) ContainerFinding {
	if networkSettings == nil {
		return ContainerFinding{CheckID: "ctr_04", Title: "Port Exposure", Severity: "info", Status: "info", Description: "Cannot determine port exposure"}
	}
	ports, _ := networkSettings["Ports"].(map[string]interface{})
	if ports != nil && len(ports) > 0 {
		exposedPorts := []string{}
		for p := range ports {
			exposedPorts = append(exposedPorts, p)
		}
		if len(exposedPorts) > 0 {
			return ContainerFinding{
				CheckID: "ctr_04", Title: "Port Exposure",
				Severity: "medium", Status: "fail",
				Description: "Container exposes ports to host: " + strings.Join(exposedPorts, ", "),
				Remediation: "Use internal networks and only expose necessary ports",
			}
		}
	}
	return ContainerFinding{
		CheckID: "ctr_04", Title: "Port Exposure",
		Severity: "info", Status: "pass",
		Description: "No ports exposed to host",
	}
}

func checkSeccomp(hostConfig map[string]interface{}) ContainerFinding {
	secOpts, _ := hostConfig["SecurityOpt"].([]interface{})
	if secOpts == nil {
		return ContainerFinding{
			CheckID: "ctr_05", Title: "Seccomp Profile",
			Severity: "medium", Status: "fail",
			Description: "No seccomp security options configured",
			Remediation: "Use default seccomp profile or specify --security-opt seccomp=...",
		}
	}
	for _, opt := range secOpts {
		optStr, ok := opt.(string)
		if ok && strings.Contains(optStr, "seccomp") {
			return ContainerFinding{
				CheckID: "ctr_05", Title: "Seccomp Profile",
				Severity: "info", Status: "pass",
				Description: "Seccomp is configured: " + optStr,
			}
		}
	}
	return ContainerFinding{
		CheckID: "ctr_05", Title: "Seccomp Profile",
		Severity: "medium", Status: "fail",
		Description: "Seccomp not found in security options",
		Remediation: "Configure seccomp profile for the container",
	}
}

func checkReadOnlyRootFS(hostConfig map[string]interface{}) ContainerFinding {
	readonly, _ := hostConfig["ReadonlyRootfs"].(bool)
	if !readonly {
		return ContainerFinding{
			CheckID: "ctr_06", Title: "Read-Only Root Filesystem",
			Severity: "medium", Status: "fail",
			Description: "Container root filesystem is writable",
			Remediation: "Use --read-only flag and tmpfs for writable directories",
		}
	}
	return ContainerFinding{
		CheckID: "ctr_06", Title: "Read-Only Root Filesystem",
		Severity: "info", Status: "pass",
		Description: "Container root filesystem is read-only",
	}
}

func checkCapabilities(hostConfig map[string]interface{}) ContainerFinding {
	capAdd, _ := hostConfig["CapAdd"].([]interface{})
	if capAdd != nil && len(capAdd) > 0 {
		var added []string
		for _, c := range capAdd {
			added = append(added, fmt.Sprintf("%v", c))
		}
		hasDangerous := false
		for _, c := range added {
			upper := strings.ToUpper(c)
			if upper == "ALL" || upper == "SYS_ADMIN" || upper == "NET_ADMIN" || upper == "SYS_MODULE" {
				hasDangerous = true
			}
		}
		if hasDangerous {
			return ContainerFinding{
				CheckID: "ctr_07", Title: "Container Capabilities",
				Severity: "high", Status: "fail",
				Description: "Dangerous capabilities added: " + strings.Join(added, ", "),
				Remediation: "Drop all capabilities and add only required ones",
			}
		}
		return ContainerFinding{
			CheckID: "ctr_07", Title: "Container Capabilities",
			Severity: "medium", Status: "warn",
			Description: "Extra capabilities added: " + strings.Join(added, ", "),
			Remediation: "Review if these capabilities are necessary",
		}
	}
	return ContainerFinding{
		CheckID: "ctr_07", Title: "Container Capabilities",
		Severity: "info", Status: "pass",
		Description: "No extra capabilities added (all dropped by default)",
	}
}

func checkHealthcheck(config map[string]interface{}) ContainerFinding {
	healthcheck, _ := config["Healthcheck"].(map[string]interface{})
	if healthcheck == nil || len(healthcheck) == 0 {
		return ContainerFinding{
			CheckID: "ctr_08", Title: "HEALTHCHECK Instruction",
			Severity: "low", Status: "fail",
			Description: "Container does not have HEALTHCHECK configured",
			Remediation: "Add HEALTHCHECK instruction in Dockerfile",
		}
	}
	return ContainerFinding{
		CheckID: "ctr_08", Title: "HEALTHCHECK Instruction",
		Severity: "info", Status: "pass",
		Description: "HEALTHCHECK is configured",
	}
}

func checkBindMounts(inspect map[string]interface{}) ContainerFinding {
	mounts, _ := inspect["Mounts"].([]interface{})
	if mounts == nil {
		return ContainerFinding{CheckID: "ctr_09", Title: "Bind Mounts", Severity: "info", Status: "info", Description: "Cannot determine mounts"}
	}
	bindCount := 0
	var bindPaths []string
	for _, m := range mounts {
		mnt, ok := m.(map[string]interface{})
		if ok {
			mtype, _ := mnt["Type"].(string)
			if mtype == "bind" {
				bindCount++
				src, _ := mnt["Source"].(string)
				bindPaths = append(bindPaths, src)
			}
		}
	}
	if bindCount > 0 {
		return ContainerFinding{
			CheckID: "ctr_09", Title: "Bind Mounts",
			Severity: "medium", Status: "fail",
			Description: fmt.Sprintf("Container has %d bind mount(s): %s", bindCount, strings.Join(bindPaths, ", ")),
			Remediation: "Use named volumes instead of bind mounts where possible; restrict host paths",
		}
	}
	return ContainerFinding{
		CheckID: "ctr_09", Title: "Bind Mounts",
		Severity: "info", Status: "pass",
		Description: "No bind mounts detected",
	}
}

func checkResourceLimits(hostConfig map[string]interface{}) ContainerFinding {
	memory, _ := hostConfig["Memory"].(int64)
	maxCPU, _ := hostConfig["NanoCpus"].(int64)

	if memory == 0 && maxCPU == 0 {
		return ContainerFinding{
			CheckID: "ctr_10", Title: "Resource Limits",
			Severity: "low", Status: "fail",
			Description: "Container has no memory or CPU limits",
			Remediation: "Use --memory and --cpus flags to limit container resources",
		}
	}
	return ContainerFinding{
		CheckID: "ctr_10", Title: "Resource Limits",
		Severity: "info", Status: "pass",
		Description: fmt.Sprintf("Resource limits: memory=%d, cpu=%d", memory, maxCPU),
	}
}

func checkUptime(state map[string]interface{}) ContainerFinding {
	startedAt, _ := state["StartedAt"].(string)
	if startedAt != "" {
		t, err := time.Parse(time.RFC3339Nano, startedAt)
		if err == nil {
			uptime := time.Since(t)
			if uptime > 30*24*time.Hour {
				return ContainerFinding{
					CheckID: "ctr_11", Title: "Container Uptime",
					Severity: "low", Status: "warn",
					Description: fmt.Sprintf("Container has been running for %d days", int(uptime.Hours()/24)),
					Remediation: "Consider restarting to pick up security updates and kernel patches",
				}
			}
			return ContainerFinding{
				CheckID: "ctr_11", Title: "Container Uptime",
				Severity: "info", Status: "pass",
				Description: fmt.Sprintf("Container running for %d days", int(uptime.Hours()/24)),
			}
		}
	}
	return ContainerFinding{
		CheckID: "ctr_11", Title: "Container Uptime",
		Severity: "info", Status: "info", Description: "Could not determine uptime",
	}
}

// ─── Helpers ────────────────────────────────────────────────────────────

func calculateContainerScore(findings []ContainerFinding) int {
	score := 100
	for _, f := range findings {
		if f.Status == "fail" {
			switch f.Severity {
			case "critical":
				score -= 25
			case "high":
				score -= 15
			case "medium":
				score -= 10
			case "low":
				score -= 5
			}
		} else if f.Status == "warn" {
			score -= 5
		}
	}
	if score < 0 {
		score = 0
	}
	return score
}

func extractShortStatus(status string) string {
	if strings.Contains(status, "Up ") {
		re := regexp.MustCompile(`Up\s+([\w\s]+)`)
		matches := re.FindStringSubmatch(status)
		if len(matches) > 1 {
			return "running (" + strings.TrimSpace(matches[1]) + ")"
		}
		return "running"
	}
	if strings.Contains(status, "Exited ") {
		return "stopped"
	}
	if strings.Contains(status, "Paused") {
		return "paused"
	}
	return status
}

// RunContainerScan runs a full container security scan on the target server,
// saving results to the scan result database.
func RunContainerScan(ctx context.Context, sshCfg sshtool.Config) (*ContainerScanSummary, error) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	scanner := NewContainerScanner()
	return scanner.Scan(ctx, sshCfg)
}
