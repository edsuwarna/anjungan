package executor

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// DockerSocketExecutor executes commands via the local Docker socket.
// All Docker operations run directly; host commands use docker run --pid=host.
type DockerSocketExecutor struct {
	socketPath string
	hostname   string
}

// NewDockerSocketExecutor creates an executor that uses the local Docker socket.
func NewDockerSocketExecutor(socketPath string) (*DockerSocketExecutor, error) {
	// Verify socket is accessible
	if _, err := os.Stat(socketPath); err != nil {
		return nil, fmt.Errorf("docker socket not accessible at %s: %w", socketPath, err)
	}

	e := &DockerSocketExecutor{
		socketPath: socketPath,
	}

	// Detect hostname on init
	if hn, err := e.dockerInfo("Name"); err == nil {
		e.hostname = strings.TrimSpace(hn)
	} else if hn, err := os.Hostname(); err == nil {
		e.hostname = hn
	}

	return e, nil
}

func (e *DockerSocketExecutor) env() []string {
	return append(os.Environ(), "DOCKER_HOST=unix://"+e.socketPath)
}

func (e *DockerSocketExecutor) dockerInfo(format string) (string, error) {
	cmd := exec.Command("docker", "info", "--format", fmt.Sprintf("{{.%s}}", format))
	cmd.Env = e.env()
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("docker info: %w", err)
	}
	return string(out), nil
}

// RunCommand executes a command on the host via nsenter.
// Uses docker run --pid=host --rm to access host process namespace.
func (e *DockerSocketExecutor) RunCommand(ctx context.Context, cmd string) (string, error) {
	// Use nsenter to run command in host namespace
	fullCmd := exec.CommandContext(ctx, "docker", "run", "--rm", "--pid=host",
		"--privileged", "--net=host",
		"-v", "/:/host:ro",
		"alpine:latest",
		"nsenter", "-t", "1", "-m", "-u", "-i", "-n",
		"sh", "-c", cmd)
	fullCmd.Env = e.env()

	var stdout, stderr bytes.Buffer
	fullCmd.Stdout = &stdout
	fullCmd.Stderr = &stderr

	if err := fullCmd.Run(); err != nil {
		return "", fmt.Errorf("host command: %w\nstderr: %s", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// RunDockerCommand runs a docker CLI command via the local socket.
func (e *DockerSocketExecutor) RunDockerCommand(ctx context.Context, dockerArgs ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", dockerArgs...)
	cmd.Env = e.env()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker command: %w\nstderr: %s", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// TestConnection checks Docker socket accessibility and returns hostname.
func (e *DockerSocketExecutor) TestConnection(ctx context.Context) (string, error) {
	hn, err := e.dockerInfo("Name")
	if err != nil {
		return "", fmt.Errorf("docker socket test failed: %w", err)
	}
	return strings.TrimSpace(hn), nil
}

// GetServerInfo collects host info via Docker info commands.
func (e *DockerSocketExecutor) GetServerInfo(ctx context.Context) (*ServerInfo, error) {
	info := &ServerInfo{
		Hostname: e.hostname,
	}

	// OS from docker info
	if osStr, err := e.dockerInfo("OperatingSystem"); err == nil {
		info.OS = strings.TrimSpace(osStr)
	}

	// Kernel from docker info
	if kernel, err := e.dockerInfo("KernelVersion"); err == nil {
		info.Kernel = strings.TrimSpace(kernel)
	}

	// Architecture
	if arch, err := e.dockerInfo("Architecture"); err == nil {
		info.Arch = strings.TrimSpace(arch)
	}

	// CPU cores
	if cpuStr, err := e.dockerInfo("NCPU"); err == nil {
		if cores, err := strconv.Atoi(strings.TrimSpace(cpuStr)); err == nil {
			info.CPUCores = cores
		}
	}

	// CPU model — try /proc/cpuinfo via nsenter
	if cpuModel, err := e.RunCommand(ctx, `cat /proc/cpuinfo 2>/dev/null | grep -m1 "model name" | cut -d: -f2 | sed 's/^ *//'`); err == nil {
		info.CPUModel = cpuModel
	}

	return info, nil
}

// GetMetrics collects resource usage from the host via nsenter.
func (e *DockerSocketExecutor) GetMetrics(ctx context.Context) (*Metrics, error) {
	m := &Metrics{}

	// CPU load
	if out, err := e.RunCommand(ctx, "cat /proc/loadavg"); err == nil {
		parts := strings.Fields(out)
		if len(parts) >= 3 {
			m.CPULoad1 = parts[0]
			m.CPULoad5 = parts[1]
			m.CPULoad15 = parts[2]
		}
	}

	// Memory
	if out, err := e.RunCommand(ctx, "free -b | awk 'NR==2{print $2,$3,$4,$7}'"); err == nil {
		parts := strings.Fields(out)
		if len(parts) >= 4 {
			m.MemoryTotal, _ = strconv.ParseUint(parts[0], 10, 64)
			m.MemoryUsed, _ = strconv.ParseUint(parts[1], 10, 64)
			m.MemoryFree, _ = strconv.ParseUint(parts[2], 10, 64)
			m.MemoryCached, _ = strconv.ParseUint(parts[3], 10, 64)
		}
	}

	// Disk
	if out, err := e.RunCommand(ctx, "df -B1 / | awk 'NR==2{print $2,$3,$4,$5}'"); err == nil {
		parts := strings.Fields(out)
		if len(parts) >= 4 {
			m.DiskTotal, _ = strconv.ParseUint(parts[0], 10, 64)
			m.DiskUsed, _ = strconv.ParseUint(parts[1], 10, 64)
			m.DiskFree, _ = strconv.ParseUint(parts[2], 10, 64)
			pctStr := strings.TrimRight(parts[3], "%")
			m.DiskUsedPct, _ = strconv.ParseFloat(pctStr, 64)
		}
	}

	// Network traffic
	if out, err := e.RunCommand(ctx, "cat /proc/net/dev | awk 'NR>2 {rx+=$2; tx+=$10} END {print rx, tx}'"); err == nil {
		parts := strings.Fields(out)
		if len(parts) >= 2 {
			m.NetRX, _ = strconv.ParseInt(parts[0], 10, 64)
			m.NetTX, _ = strconv.ParseInt(parts[1], 10, 64)
		}
	}

	// Uptime
	if out, err := e.RunCommand(ctx, "uptime -p"); err == nil {
		m.Uptime = strings.TrimSpace(out)
	}

	return m, nil
}

// Close is a no-op for Docker socket executor.
func (e *DockerSocketExecutor) Close() error {
	return nil
}
