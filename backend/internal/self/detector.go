package self

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/edsuwarna/anjungan/internal/common/db"
	"github.com/edsuwarna/anjungan/internal/common/model"
	"github.com/edsuwarna/anjungan/internal/config"
	"github.com/edsuwarna/anjungan/internal/executor"
)

// Detector handles auto-registration of the host server (where Anjungan runs).
type Detector struct {
	repo            *db.Repository
	cfg             *config.SelfServerConfig
	dockerSocketPath string
}

// NewDetector creates a self-server detector.
func NewDetector(repo *db.Repository, cfg *config.SelfServerConfig) *Detector {
	return &Detector{
		repo:             repo,
		cfg:              cfg,
		dockerSocketPath: cfg.DockerSocketPath,
	}
}

// DetectAndRegister checks for Docker socket availability and registers/updates
// the self-server in the database.
func (d *Detector) DetectAndRegister(ctx context.Context) {
	if !d.cfg.Enabled {
		log.Println("[self] self-server detection disabled (SELF_SERVER_ENABLED=false)")
		return
	}

	log.Printf("[self] detecting host via Docker socket (%s)...", d.dockerSocketPath)

	// Check if Docker socket is accessible
	if _, err := os.Stat(d.dockerSocketPath); err != nil {
		log.Printf("[self] Docker socket not accessible at %s: %v — skipping", d.dockerSocketPath, err)
		return
	}

	// Create executor
	exec, err := executor.NewDockerExecutor(d.dockerSocketPath)
	if err != nil {
		log.Printf("[self] failed to create Docker executor: %v — skipping", err)
		return
	}
	defer exec.Close()

	// Test connection
	detectCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	hostname, err := exec.TestConnection(detectCtx)
	if err != nil {
		log.Printf("[self] Docker socket test failed: %v — skipping", err)
		return
	}
	log.Printf("[self] Docker host detected: %s", hostname)

	// Get server info
	infoCtx, infoCancel := context.WithTimeout(ctx, 30*time.Second)
	defer infoCancel()

	info, err := exec.GetServerInfo(infoCtx)
	if err != nil {
		log.Printf("[self] failed to get server info: %v (continuing with partial)", err)
		info = &executor.ServerInfo{Hostname: hostname}
	}

	// Build OS info string
	osInfo := info.OS
	if info.Kernel != "" {
		osInfo = info.OS + " (" + info.Kernel + ")"
	}

	// Build CPU info string
	cpuInfo := info.CPUModel
	if info.CPUCores > 0 {
		cpuInfo = fmt.Sprintf("%s (%d cores)", cpuInfo, info.CPUCores)
	}

	// Determine host IP — try to get from environment or Docker
	hostIP := d.cfg.HostNetwork
	if hostIP == "" {
		hostIP = "127.0.0.1"
	}

	// Build self server model
	now := time.Now()
	selfServer := &model.Server{
		Name:           d.cfg.Name,
		Host:           hostIP,
		Port:           22, // default, not used for docker-socket
		Status:         "online",
		OSInfo:         osInfo,
		CPUInfo:        cpuInfo,
		ConnectionType: executor.ConnectionTypeDockerSocket,
		IsSelf:         true,
		SelfHostname:   hostname,
		Tags:           []string{"self", "anjungan-host"},
		ServerType:     "docker-host",
		Monitoring:     true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	// Find or create
	existing, isNew, err := d.repo.FindOrCreateSelfServer(ctx, selfServer)
	if err != nil {
		log.Printf("[self] failed to register self-server: %v", err)
		return
	}

	if isNew {
		log.Printf("[self] self-server registered: %s (%s)", existing.Name, existing.ID)
	} else {
		log.Printf("[self] self-server updated: %s (%s)", existing.Name, existing.ID)
	}
}
