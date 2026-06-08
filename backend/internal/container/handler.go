package container

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/edsuwarna/anjungan/internal/auth"
	"github.com/edsuwarna/anjungan/internal/audit"
	"github.com/edsuwarna/anjungan/internal/common"
	"github.com/edsuwarna/anjungan/internal/common/db"
	"github.com/edsuwarna/anjungan/internal/common/model"
	"github.com/edsuwarna/anjungan/internal/executor"
	sshtool "github.com/edsuwarna/anjungan/internal/infra/ssh"
)

type Handler struct {
	repo            *db.Repository
	dockerSocketPath string
}

func NewHandler(repo *db.Repository, dockerSocketPath string) *Handler {
	return &Handler{repo: repo, dockerSocketPath: dockerSocketPath}
}

// ─── Models ─────────────────────────────────────────────────────────────────

type ContainerInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Image    string `json:"image"`
	Status   string `json:"status"`
	State    string `json:"state"`
	Ports    string `json:"ports"`
	Created  string `json:"created"`
	ServerID string `json:"server_id"`
	ServerName string `json:"server_name"`
	ServerHost string `json:"server_host"`
	// Security scan info (populated from latest Container Security scan)
	Security *ContainerSecurity `json:"security,omitempty"`
}

// ContainerSecurity holds per-container security data from the latest scan.
type ContainerSecurity struct {
	Score     int                   `json:"score"`
	Badges    []string              `json:"badges"`
	Findings  []model.ScanFinding   `json:"findings"`
	ScannedAt *time.Time            `json:"scanned_at"`
}

type ContainerStats struct {
	Total      int `json:"total"`
	Running    int `json:"running"`
	Exited     int `json:"exited"`
	Paused     int `json:"paused"`
	Other      int `json:"other"`
	ServersWithDocker int `json:"servers_with_docker"`
}

// ServerContainers groups containers and stats per server (for server-first view)
type ServerContainers struct {
	Server     ServerBrief     `json:"server"`
	Stats      ServerCtrStats  `json:"stats"`
	Containers []ContainerInfo `json:"containers"`
}

type ServerBrief struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
	CPUInfo string `json:"cpu_info"`
	OSInfo  string `json:"os_info"`
}

type ServerCtrStats struct {
	Total   int `json:"total"`
	Running int `json:"running"`
	Exited  int `json:"exited"`
	Paused  int `json:"paused"`
}

type ByServerResponse struct {
	Servers []ServerContainers `json:"servers"`
	Stats   ContainerStats     `json:"stats"`
}

type ContainerActionResponse struct {
	Message string `json:"message"`
	Output  string `json:"output"`
}

// ─── SSH helpers ────────────────────────────────────────────────────────────

func (h *Handler) resolveSSHKey(ctx context.Context, srv *model.Server) error {
	if srv.SSHKey == "" && srv.SSHKeyID != "" {
		savedKey, err := h.repo.GetSSHKeyByIDFull(ctx, srv.SSHKeyID)
		if err != nil {
			return fmt.Errorf("resolve ssh key %s: %w", srv.SSHKeyID, err)
		}
		srv.SSHKey = savedKey.PrivateKey
	}
	return nil
}

func (h *Handler) sshConfigForServer(ctx context.Context, srv *model.Server) (sshtool.Config, error) {
	if err := h.resolveSSHKey(ctx, srv); err != nil {
		return sshtool.Config{}, err
	}
	return sshtool.Config{
		Host:     srv.Host,
		Port:     srv.Port,
		User:     srv.SSHUser,
		AuthType: srv.SSHAuthType,
		Key:      srv.SSHKey,
		Password: srv.SSHPassword,
	}, nil
}

// ─── Executor helpers ───────────────────────────────────────────────────────

func (h *Handler) getExecutor(srv *model.Server) (executor.ServerExecutor, error) {
	if srv.ConnectionType == executor.ConnectionTypeDockerSocket {
		return executor.NewDockerExecutor(h.dockerSocketPath)
	}
	return nil, nil // nil means use SSH path
}

// runDockerCommand runs a docker command on a server, supporting both SSH and docker-socket.
func (h *Handler) runDockerCommand(ctx context.Context, srv *model.Server, cmd string) (string, error) {
	if srv.ConnectionType == executor.ConnectionTypeDockerSocket {
		exec, err := h.getExecutor(srv)
		if err != nil {
			return "", fmt.Errorf("executor init: %w", err)
		}
		defer exec.Close()

		// cmd is like "docker ps -a --format '...'" — strip "docker " prefix
		trimmed := strings.TrimPrefix(cmd, "docker ")
		// Parse into args (simple split preserves quoted strings as-is for docker CLI)
		args := strings.Fields(trimmed)
		return exec.RunDockerCommand(ctx, args...)
	}

	// SSH path
	return h.runDockerSSH(ctx, srv, cmd)
}

// runDockerSSH runs a docker command on a server via SSH (legacy path)
func (h *Handler) runDockerSSH(ctx context.Context, srv *model.Server, cmd string) (string, error) {
	cfg, err := h.sshConfigForServer(ctx, srv)
	if err != nil {
		return "", fmt.Errorf("ssh config: %w", err)
	}
	out, err := sshtool.RunCommand(ctx, cfg, cmd)
	if err != nil {
		return "", fmt.Errorf("docker command: %w", err)
	}
	return out, nil
}

// resolveAuth retrieves the caller's allowed groups for filtered access
func (h *Handler) allowedGroups(ctx context.Context) []string {
	if claims := auth.GetClaims(ctx); claims != nil {
		if claims.Role != model.RoleAdmin {
			groups, err := h.repo.GetUserServerGroups(ctx, claims.UserID)
			if err != nil {
				return []string{}
			}
			return groups
		}
	}
	return nil // nil means admin = no group restriction
}

// ─── Routes ─────────────────────────────────────────────────────────────────

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Get("/by-server", h.ByServer)
	r.Get("/{id}", h.Get)
	r.Get("/{id}/security", h.GetSecurity)
	r.Post("/{id}/start", h.Start)
	r.Post("/{id}/stop", h.Stop)
	r.Post("/{id}/restart", h.Restart)
	r.Get("/{id}/logs", h.Logs)
	r.Get("/{id}/stats", h.ContainerStats)
	r.Get("/stats", h.Stats)
	return r
}

// ─── List containers across all servers ─────────────────────────────────────

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	servers, err := h.repo.ListServersByGroups(r.Context(), h.allowedGroups(r.Context()))
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list servers")
		return
	}

	type result struct {
		server    *model.Server
		lines     []string
		err       error
	}

	results := make(chan result, len(servers))
	var wg sync.WaitGroup

	for i := range servers {
		wg.Add(1)
		go func(srv *model.Server) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
			defer cancel()

			out, err := h.runDockerCommand(ctx, srv,
				`docker ps -a --format '{"id":"{{.ID}}","name":"{{.Names}}","image":"{{.Image}}","status":"{{.Status}}","state":"{{.State}}","ports":"{{.Ports}}","created":"{{.CreatedAt}}"}'`,
			)
			if err != nil {
				results <- result{server: srv, err: err}
				return
			}
			lines := strings.Split(strings.TrimSpace(out), "\n")
			results <- result{server: srv, lines: lines}
		}(servers[i])
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	containers := make([]ContainerInfo, 0)
	for res := range results {
		if res.err != nil {
			continue
		}
		for _, line := range res.lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var c ContainerInfo
			if err := json.Unmarshal([]byte(line), &c); err != nil {
				continue
			}
			c.ServerID = res.server.ID
			c.ServerName = res.server.Name
			c.ServerHost = res.server.Host
			containers = append(containers, c)
		}
	}

	if containers == nil {
		containers = []ContainerInfo{}
	}

	common.JSON(w, http.StatusOK, containers)
}

// ─── ByServer — containers grouped by server ──────────────────────────────────

func (h *Handler) ByServer(w http.ResponseWriter, r *http.Request) {
	servers, err := h.repo.ListServersByGroups(r.Context(), h.allowedGroups(r.Context()))
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list servers")
		return
	}

	type serverResult struct {
		server     *model.Server
		containers []ContainerInfo
		hasDocker  bool
		err        error
	}

	results := make(chan serverResult, len(servers))
	var wg sync.WaitGroup

	for i := range servers {
		wg.Add(1)
		go func(srv *model.Server) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
			defer cancel()

			out, err := h.runDockerCommand(ctx, srv,
				`docker ps -a --format '{"id":"{{.ID}}","name":"{{.Names}}","image":"{{.Image}}","status":"{{.Status}}","state":"{{.State}}","ports":"{{.Ports}}","created":"{{.CreatedAt}}"}'`,
			)
			if err != nil {
				results <- serverResult{server: srv, err: err}
				return
			}

			lines := strings.Split(strings.TrimSpace(out), "\n")
			ctrs := make([]ContainerInfo, 0)
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				var c ContainerInfo
				if err := json.Unmarshal([]byte(line), &c); err != nil {
					continue
				}
				c.ServerID = srv.ID
				c.ServerName = srv.Name
				c.ServerHost = srv.Host

				// Attach security data if available
				c.Security = h.attachContainerSecurity(r.Context(), srv.ID, c.Name)

				ctrs = append(ctrs, c)
			}
			results <- serverResult{server: srv, containers: ctrs, hasDocker: true}
		}(servers[i])
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	type aggStats struct {
		total   int
		running int
		exited  int
		paused  int
	}

	byServer := make([]ServerContainers, 0)
	global := aggStats{}
	serverCount := 0

	for res := range results {
		if res.err != nil {
			continue
		}
		serverCount++
		st := ServerCtrStats{}
		for _, c := range res.containers {
			st.Total++
			switch c.State {
			case "running":
				st.Running++
			case "exited", "stopped":
				st.Exited++
			case "paused":
				st.Paused++
			}
		}
		global.total += st.Total
		global.running += st.Running
		global.exited += st.Exited
		global.paused += st.Paused

		byServer = append(byServer, ServerContainers{
			Server: ServerBrief{
				ID:      res.server.ID,
				Name:    res.server.Name,
				Host:    res.server.Host,
				Port:    res.server.Port,
				CPUInfo: res.server.CPUInfo,
				OSInfo:  res.server.OSInfo,
			},
			Stats:      st,
			Containers: res.containers,
		})
	}

	if byServer == nil {
		byServer = []ServerContainers{}
	}

	common.JSON(w, http.StatusOK, ByServerResponse{
		Servers: byServer,
		Stats: ContainerStats{
			Total:            global.total,
			Running:          global.running,
			Exited:           global.exited,
			Paused:           global.paused,
			ServersWithDocker: serverCount,
		},
	})
}

// attachContainerSecurity looks up the latest Container Security scan findings
// for a container on a given server by name, and returns a ContainerSecurity summary.
func (h *Handler) attachContainerSecurity(ctx context.Context, serverID, containerName string) *ContainerSecurity {
	// Normalize: strip leading / from container names
	name := strings.TrimPrefix(containerName, "/")

	secData, err := h.repo.GetContainerSecurityByServer(ctx, serverID)
	if err != nil || secData == nil {
		return nil
	}

	data, ok := secData[name]
	if !ok || data == nil {
		return nil
	}

	return &ContainerSecurity{
		Score:     data.Score,
		Badges:    data.Badges,
		Findings:  data.Findings,
		ScannedAt: data.ScannedAt,
	}
}

// ─── Single Container Stats ───────────────────────────────────────────────────

type ContainerUsageStats struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryUsage   int64   `json:"memory_usage"`
	MemoryLimit   int64   `json:"memory_limit"`
	MemoryPercent float64 `json:"memory_percent"`
	NetRX         int64   `json:"net_rx"`
	NetTX         int64   `json:"net_tx"`
	BlockRead     int64   `json:"block_read"`
	BlockWrite    int64   `json:"block_write"`
	PIDs          int     `json:"pids"`
}

func (h *Handler) ContainerStats(w http.ResponseWriter, r *http.Request) {
	containerID := chi.URLParam(r, "id")
	serverID := r.URL.Query().Get("server_id")

	srv, err := h.resolveServer(r.Context(), containerID, serverID)
	if err != nil {
		common.Error(w, http.StatusNotFound, "container not found")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	out, err := h.runDockerCommand(ctx, srv, fmt.Sprintf("docker stats --no-stream --format '{{json .}}' %s", containerID))
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to get container stats: "+err.Error())
		return
	}

	var raw struct {
		BlockIO    string `json:"BlockIO"`
		CPUPerc    string `json:"CPUPerc"`
		MemPerc    string `json:"MemPerc"`
		MemUsage   string `json:"MemUsage"`
		NetIO      string `json:"NetIO"`
		PIDs       string `json:"PIDs"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		// Container may be stopped — return empty stats
		common.JSON(w, http.StatusOK, ContainerUsageStats{})
		return
	}

	stats := ContainerUsageStats{}

	// Parse CPU percent: "0.05%" -> 0.05
	fmt.Sscanf(raw.CPUPerc, "%f", &stats.CPUPercent)

	// Parse memory percent: "0.05%"
	fmt.Sscanf(raw.MemPerc, "%f", &stats.MemoryPercent)

	// Parse memory usage: "6.5MiB / 31.3GiB"
	if parts := strings.Split(raw.MemUsage, "/"); len(parts) == 2 {
		stats.MemoryUsage = parseSizeStr(strings.TrimSpace(parts[0]))
		stats.MemoryLimit = parseSizeStr(strings.TrimSpace(parts[1]))
	}

	// Parse NetIO: "1.2kB / 0B"
	if parts := strings.Split(raw.NetIO, "/"); len(parts) == 2 {
		stats.NetRX = parseSizeStr(strings.TrimSpace(parts[0]))
		stats.NetTX = parseSizeStr(strings.TrimSpace(parts[1]))
	}

	// Parse BlockIO: "0B / 0B"
	if parts := strings.Split(raw.BlockIO, "/"); len(parts) == 2 {
		stats.BlockRead = parseSizeStr(strings.TrimSpace(parts[0]))
		stats.BlockWrite = parseSizeStr(strings.TrimSpace(parts[1]))
	}

	// Parse PIDs
	fmt.Sscanf(raw.PIDs, "%d", &stats.PIDs)

	common.JSON(w, http.StatusOK, stats)
}

// parseSizeStr converts size strings like "1.2kB", "6.5MiB", "31.3GiB" to bytes
func parseSizeStr(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "0B" {
		return 0
	}

	var val float64
	var unit string
	if n, _ := fmt.Sscanf(s, "%f%s", &val, &unit); n < 1 {
		return 0
	}

	// Normalize unit
	unit = strings.TrimSpace(unit)
	switch {
	case strings.HasPrefix(unit, "KiB") || unit == "KB" || unit == "kB":
		return int64(val * 1024)
	case strings.HasPrefix(unit, "MiB") || unit == "MB":
		return int64(val * 1024 * 1024)
	case strings.HasPrefix(unit, "GiB") || unit == "GB":
		return int64(val * 1024 * 1024 * 1024)
	case strings.HasPrefix(unit, "TiB") || unit == "TB":
		return int64(val * 1024 * 1024 * 1024 * 1024)
	case unit == "B" || unit == "":
		return int64(val)
	default:
		// Unknown unit, return as-is if it's just a number
		return int64(val)
	}
}

// ─── Container Stats (global) ───────────────────────────────────────────────

func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	servers, err := h.repo.ListServersByGroups(r.Context(), h.allowedGroups(r.Context()))
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list servers")
		return
	}

	type serverStats struct {
		total      int
		running    int
		exited     int
		paused     int
		other      int
		hasDocker  bool
	}

	mu := sync.Mutex{}
	aggregated := ContainerStats{}

	var wg sync.WaitGroup
	for i := range servers {
		wg.Add(1)
		go func(srv *model.Server) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
			defer cancel()

			out, err := h.runDockerCommand(ctx, srv,
				`docker ps -a --format '{{.State}}'`,
			)
			if err != nil {
				return
			}

			lines := strings.Split(strings.TrimSpace(out), "\n")
			var st serverStats
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				st.total++
				switch line {
				case "running":
					st.running++
				case "exited":
					st.exited++
				case "paused":
					st.paused++
				default:
					st.other++
				}
			}
			st.hasDocker = true

			mu.Lock()
			aggregated.Total += st.total
			aggregated.Running += st.running
			aggregated.Exited += st.exited
			aggregated.Paused += st.paused
			aggregated.Other += st.other
			if st.hasDocker {
				aggregated.ServersWithDocker++
			}
			mu.Unlock()
		}(servers[i])
	}
	wg.Wait()

	common.JSON(w, http.StatusOK, aggregated)
}

// ─── Container Action helpers ────────────────────────────────────────────────

// resolveServerFromQuery finds which server a container belongs to.
// If the server_id query param is provided, use it directly.
// Otherwise, iterate all accessible servers and find the container.
func (h *Handler) resolveServer(ctx context.Context, containerID, serverID string) (*model.Server, error) {
	if serverID != "" {
		return h.repo.GetServerByIDFull(ctx, serverID)
	}

	// Scan all servers to find the container
	servers, err := h.repo.ListServersByGroups(ctx, h.allowedGroups(ctx))
	if err != nil {
		return nil, fmt.Errorf("list servers: %w", err)
	}

	// Try all servers in parallel with short timeout
	type found struct {
		srv *model.Server
		err error
	}
	ch := make(chan *found, len(servers))
	ctx2, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	for i := range servers {
		go func(srv *model.Server) {
			out, err := h.runDockerSSH(ctx2, srv, fmt.Sprintf("docker ps -aq --filter id=%s", containerID))
			if err != nil {
				ch <- nil
				return
			}
			if strings.TrimSpace(out) != "" {
				ch <- &found{srv: srv}
			} else {
				ch <- nil
			}
		}(servers[i])
	}

	var foundSrv *model.Server
	for i := 0; i < len(servers); i++ {
		if f := <-ch; f != nil && f.srv != nil {
			foundSrv = f.srv
			cancel()
			// drain remaining
			go func() {
				for i := i + 1; i < len(servers); i++ {
					<-ch
				}
			}()
			break
		}
	}

	if foundSrv == nil {
		return nil, fmt.Errorf("container not found on any accessible server")
	}
	return foundSrv, nil
}

// ─── Get single container ────────────────────────────────────────────────────

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	containerID := chi.URLParam(r, "id")
	serverID := r.URL.Query().Get("server_id")

	srv, err := h.resolveServer(r.Context(), containerID, serverID)
	if err != nil {
		common.Error(w, http.StatusNotFound, "container not found")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	out, err := h.runDockerCommand(ctx, srv, fmt.Sprintf("docker inspect %s", containerID))
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to inspect container: "+err.Error())
		return
	}

	var parsed interface{}
	if json.Unmarshal([]byte(out), &parsed) == nil {
		common.JSON(w, http.StatusOK, parsed)
		return
	}
	common.JSON(w, http.StatusOK, map[string]interface{}{"raw": out})
}

// ─── Container Actions ───────────────────────────────────────────────────────

func (h *Handler) performAction(w http.ResponseWriter, r *http.Request, action string) {
	containerID := chi.URLParam(r, "id")
	serverID := r.URL.Query().Get("server_id")

	srv, err := h.resolveServer(r.Context(), containerID, serverID)
	if err != nil {
		common.Error(w, http.StatusNotFound, "container not found: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Get container name for audit log
	containerName := containerID
	nameCmd := fmt.Sprintf("docker ps --filter id=%s --format '{{.Names}}'", containerID)
	if nameOut, nameErr := h.runDockerCommand(ctx, srv, nameCmd); nameErr == nil {
		if n := strings.TrimSpace(nameOut); n != "" {
			containerName = n
		}
	}

	out, err := h.runDockerCommand(ctx, srv, fmt.Sprintf("docker %s %s", action, containerID))
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to "+action+" container: "+err.Error())
		return
	}

	// Audit log
	if claims := auth.GetClaims(r.Context()); claims != nil {
		meta, _ := json.Marshal(map[string]string{
			"container_id":   containerID,
			"container_name": containerName,
			"server_id":      srv.ID,
			"server_name":    srv.Name,
		})
		audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
			"container."+action, "container", containerID,
			fmt.Sprintf("%s container %s on %s", action, containerName, srv.Name),
			json.RawMessage(meta))
	}

	common.JSON(w, http.StatusOK, ContainerActionResponse{
		Message: fmt.Sprintf("Container %s %sed", containerID, action),
		Output:  out,
	})
}

func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	h.performAction(w, r, "start")
}

func (h *Handler) Stop(w http.ResponseWriter, r *http.Request) {
	h.performAction(w, r, "stop")
}

func (h *Handler) Restart(w http.ResponseWriter, r *http.Request) {
	h.performAction(w, r, "restart")
}

// ─── Container Logs ─────────────────────────────────────────────────────────

func (h *Handler) Logs(w http.ResponseWriter, r *http.Request) {
	containerID := chi.URLParam(r, "id")
	serverID := r.URL.Query().Get("server_id")

	srv, err := h.resolveServer(r.Context(), containerID, serverID)
	if err != nil {
		common.Error(w, http.StatusNotFound, "container not found")
		return
	}

	tail := r.URL.Query().Get("tail")
	if tail == "" {
		tail = "50"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	out, err := h.runDockerCommand(ctx, srv, fmt.Sprintf("docker logs --tail=%s %s", tail, containerID))
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to get logs: "+err.Error())
		return
	}

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"logs": out,
	})
}

// ─── Get Container Security Report ──────────────────────────────────────────

// GetSecurity returns container info + server info + security data for the security report page.
func (h *Handler) GetSecurity(w http.ResponseWriter, r *http.Request) {
	containerID := chi.URLParam(r, "id")
	serverID := r.URL.Query().Get("server_id")

	srv, err := h.resolveServer(r.Context(), containerID, serverID)
	if err != nil {
		common.Error(w, http.StatusNotFound, "container not found")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// Get container info via docker ps
	out, err := h.runDockerCommand(ctx, srv, fmt.Sprintf(
		`docker ps -a --filter id=%s --format '{"id":"{{.ID}}","name":"{{.Names}}","image":"{{.Image}}","status":"{{.Status}}","state":"{{.State}}","ports":"{{.Ports}}","created":"{{.CreatedAt}}"}'`,
		containerID,
	))
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to get container info: "+err.Error())
		return
	}

	line := strings.TrimSpace(out)
	if line == "" {
		common.Error(w, http.StatusNotFound, "container not found on server")
		return
	}

	var c struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Image   string `json:"image"`
		Status  string `json:"status"`
		State   string `json:"state"`
		Ports   string `json:"ports"`
		Created string `json:"created"`
	}
	if err := json.Unmarshal([]byte(line), &c); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to parse container info")
		return
	}

	// Attach security data
	sec := h.attachContainerSecurity(r.Context(), srv.ID, c.Name)

	resp := model.ContainerSecurityResponse{
		Container: model.ContainerSecurityContainer{
			ID:      c.ID,
			Name:    c.Name,
			Image:   c.Image,
			Status:  c.Status,
			State:   c.State,
			Ports:   c.Ports,
			Created: c.Created,
		},
		Server: model.ContainerSecurityServer{
			ID:   srv.ID,
			Name: srv.Name,
			Host: srv.Host,
			Port: srv.Port,
		},
	}

	if sec != nil {
		resp.Security = &model.SecuritySummary{
			Score:     sec.Score,
			Badges:    sec.Badges,
			Findings:  sec.Findings,
			ScannedAt: sec.ScannedAt,
		}
	}

	common.JSON(w, http.StatusOK, resp)
}
