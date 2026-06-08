package executor

import (
	"context"
)

// ConnectionType constants
const (
	ConnectionTypeSSH          = "ssh"
	ConnectionTypeDockerSocket = "docker-socket"
)

// Metrics holds server resource usage data
type Metrics struct {
	CPULoad1     string  `json:"cpu_load_1"`
	CPULoad5     string  `json:"cpu_load_5"`
	CPULoad15    string  `json:"cpu_load_15"`
	MemoryTotal  uint64  `json:"memory_total"`
	MemoryUsed   uint64  `json:"memory_used"`
	MemoryFree   uint64  `json:"memory_free"`
	MemoryCached uint64  `json:"memory_cached"`
	DiskTotal    uint64  `json:"disk_total"`
	DiskUsed     uint64  `json:"disk_used"`
	DiskFree     uint64  `json:"disk_free"`
	DiskUsedPct  float64 `json:"disk_used_pct"`
	NetRX        int64   `json:"net_rx"`
	NetTX        int64   `json:"net_tx"`
	Uptime       string  `json:"uptime"`
}

// ServerInfo holds auto-detected server information
type ServerInfo struct {
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Kernel   string `json:"kernel"`
	Arch     string `json:"arch"`
	CPUCores int    `json:"cpu_cores"`
	CPUModel string `json:"cpu_model"`
}

// ServerExecutor abstracts command execution on a managed server.
// Implementations: SSH (remote), DockerSocket (local via Docker API).
type ServerExecutor interface {
	// RunCommand executes an arbitrary command and returns stdout.
	RunCommand(ctx context.Context, cmd string) (string, error)

	// RunDockerCommand executes a docker CLI command via the executor.
	RunDockerCommand(ctx context.Context, dockerArgs ...string) (string, error)

	// TestConnection checks if the server is reachable and returns hostname.
	TestConnection(ctx context.Context) (string, error)

	// GetServerInfo auto-detects OS, kernel, CPU info.
	GetServerInfo(ctx context.Context) (*ServerInfo, error)

	// GetMetrics collects live resource usage data.
	GetMetrics(ctx context.Context) (*Metrics, error)

	// Close releases any underlying connections.
	Close() error
}
