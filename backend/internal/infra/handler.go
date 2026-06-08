package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/creack/pty"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/edsuwarna/anjungan/internal/audit"
	"github.com/edsuwarna/anjungan/internal/auth"
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

// resolveSSHKey ensures a server's SSHKey is populated.
// If the server has an ssh_key_id but empty ssh_key, it loads the key from the ssh_keys table.
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

// sshConfigForServer builds an ssh tool config from a server, resolving saved keys if needed.
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

// getExecutor creates the appropriate executor for a server.
// For SSH connections, returns nil (use existing sshConfigForServer path).
// For docker-socket connections, returns a DockerSocketExecutor.
func (h *Handler) getExecutor(srv *model.Server) (executor.ServerExecutor, error) {
	if srv.ConnectionType == executor.ConnectionTypeDockerSocket {
		return executor.NewDockerExecutor(h.dockerSocketPath)
	}
	return nil, nil // nil means use SSH path
}

// authorizeView checks if the user can VIEW a server (admin always allowed,
// developer/viewer only if server's group is in their allowed groups).
func (h *Handler) authorizeView(ctx context.Context, id string) (*model.Server, error) {
	claims := auth.GetClaims(ctx)
	if claims == nil {
		return nil, fmt.Errorf("unauthorized")
	}
	if claims.Role == model.RoleAdmin {
		return h.repo.GetServerByIDFull(ctx, id)
	}
	// Developer or Viewer: check group membership
	allowedGroups, err := h.repo.GetUserServerGroups(ctx, claims.UserID)
	if err != nil || len(allowedGroups) == 0 {
		return nil, fmt.Errorf("forbidden")
	}
	srv, err := h.repo.GetServerByIDFull(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("server not found")
	}
	if srv.ServerGroup == "" {
		return nil, fmt.Errorf("forbidden")
	}
	for _, g := range allowedGroups {
		if g == srv.ServerGroup {
			return srv, nil
		}
	}
	return nil, fmt.Errorf("forbidden")
}

// authorizeWrite returns the server if the user can WRITE/INTERACT with it
// (admin unrestricted, developer by group, viewer always denied).
func (h *Handler) authorizeWrite(ctx context.Context, id string) (*model.Server, error) {
	srv, err := h.authorizeView(ctx, id)
	if err != nil {
		return nil, err
	}
	claims := auth.GetClaims(ctx)
	if claims == nil {
		return nil, fmt.Errorf("unauthorized")
	}
	if claims.Role == model.RoleViewer {
		return nil, fmt.Errorf("forbidden")
	}
	return srv, nil
}

// requireAdmin checks the caller is an admin.
func (h *Handler) requireAdmin(ctx context.Context) error {
	claims := auth.GetClaims(ctx)
	if claims == nil {
		return fmt.Errorf("unauthorized")
	}
	if claims.Role != model.RoleAdmin {
		return fmt.Errorf("forbidden")
	}
	return nil
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Get("/groups", h.ListGroups)
	r.Get("/regions", h.ListRegions)
	r.Get("/types", h.ListTypes)
	r.Post("/bulk-delete", h.BulkDelete)
	r.Get("/{id}", h.Get)
	r.Put("/{id}", h.Update)
	r.Delete("/{id}", h.Delete)
	r.Post("/test", h.TestConnectionGlobal)
	r.Post("/{id}/test", h.TestConnection)
	r.Get("/{id}/metrics", h.Metrics)
	r.Post("/{id}/detect", h.DetectInfo)
	r.Get("/{id}/containers", h.Containers)
	r.Post("/{id}/containers/{container}/start", h.ContainerStart)
	r.Post("/{id}/containers/{container}/stop", h.ContainerStop)
	r.Post("/{id}/containers/{container}/restart", h.ContainerRestart)
	r.Get("/{id}/containers/{container}/logs", h.ContainerLogs)
	r.Get("/{id}/containers/{container}/inspect", h.ContainerInspect)
	r.Post("/{id}/containers/{container}/exec", h.ContainerExec)
	r.Get("/{id}/containers/{container}/exec-ws", h.ContainerExecWS)
	r.Get("/{id}/terminal", h.TerminalWS)
	return r
}

// ─── List with pagination, sort, filter ────────────────────────────────────

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// Determine user's allowed groups for access filtering
	var allowedGroups []string
	if claims := auth.GetClaims(r.Context()); claims != nil {
		if claims.Role != model.RoleAdmin {
			var err error
			allowedGroups, err = h.repo.GetUserServerGroups(r.Context(), claims.UserID)
			if err != nil {
				allowedGroups = []string{}
			}
		}
	}

	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))

	if q.Get("all") == "true" {
		// Legacy: return all servers (for dropdowns / overview)
		servers, err := h.repo.ListServersByGroups(r.Context(), allowedGroups)
		if err != nil {
			common.Error(w, http.StatusInternalServerError, "failed to list servers")
			return
		}
		resp := make([]model.ServerResponse, len(servers))
		for i, s := range servers {
			resp[i] = s.ToResponse()
		}
		common.JSON(w, http.StatusOK, resp)
		return
	}

	query := model.ServerListQuery{
		Page:        page,
		Limit:       limit,
		Sort:        q.Get("sort"),
		Order:       q.Get("order"),
		Status:      q.Get("status"),
		Search:      q.Get("search"),
		ServerGroup: q.Get("server_group"),
		Region:      q.Get("region"),
		ServerType:  q.Get("server_type"),
	}

	result, err := h.repo.ListServersPaginated(r.Context(), query, allowedGroups)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list servers")
		return
	}

	common.JSON(w, http.StatusOK, result)
}

// ─── Create server ─────────────────────────────────────────────────────────

type createServerInput struct {
	Name        string   `json:"name"`
	Host        string   `json:"host"`
	Port        int      `json:"port"`
	SSHUser     string   `json:"ssh_user"`
	SSHAuthType string   `json:"ssh_auth_type"`
	SSHKeyID    string   `json:"ssh_key_id,omitempty"`
	SSHKey      string   `json:"ssh_key,omitempty"`
	SSHPassword string   `json:"ssh_password,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	ServerGroup string   `json:"server_group,omitempty"`
	Region      string   `json:"region,omitempty"`
	ServerType  string   `json:"server_type,omitempty"`
	Description string   `json:"description,omitempty"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	if err := h.requireAdmin(r.Context()); err != nil {
		common.Error(w, http.StatusForbidden, "admin access required")
		return
	}
	var input createServerInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if input.Name == "" {
		common.Error(w, http.StatusBadRequest, "name is required")
		return
	}
	if input.Host == "" {
		common.Error(w, http.StatusBadRequest, "host is required")
		return
	}
	if input.Port == 0 {
		input.Port = 22
	}
	if input.SSHUser == "" {
		input.SSHUser = "root"
	}
	if input.SSHAuthType == "" {
		input.SSHAuthType = "key"
	}
	if input.Tags == nil {
		input.Tags = []string{}
	}

	// Resolve SSH key from ssh_key_id if provided
	sshKeyContent := input.SSHKey
	sshKeyID := input.SSHKeyID
	if sshKeyID != "" && sshKeyContent == "" {
		savedKey, err := h.repo.GetSSHKeyByIDFull(r.Context(), sshKeyID)
		if err == nil {
			sshKeyContent = savedKey.PrivateKey
		}
	}

	userID := ""
	if claims := auth.GetClaims(r.Context()); claims != nil {
		userID = claims.UserID
	}

	if input.Tags == nil {
		input.Tags = []string{}
	}

	now := time.Now()
	server := &model.Server{
		ID:          uuid.New().String(),
		Name:        input.Name,
		Host:        input.Host,
		Port:        input.Port,
		SSHUser:     input.SSHUser,
		SSHAuthType: input.SSHAuthType,
		SSHKey:      sshKeyContent,
		SSHKeyID:    sshKeyID,
		SSHPassword: input.SSHPassword,
		Status:      "unknown",
		Tags:        input.Tags,
		ServerGroup: input.ServerGroup,
		Region:      input.Region,
		ServerType:  input.ServerType,
		Description: input.Description,
		CreatedBy:   userID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := h.repo.CreateServer(r.Context(), server); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to create server")
		return
	}

	// Log activity
	_ = h.repo.SaveActivity(r.Context(), "server_added", fmt.Sprintf("Added server %s (%s)", server.Name, server.Host), userID)

	// Audit log
	claims := auth.GetClaims(r.Context())
	claimsUserID := ""
	claimsEmail := ""
	if claims != nil {
		claimsUserID = claims.UserID
		claimsEmail = claims.Email
	}
	meta, _ := json.Marshal(map[string]interface{}{
		"server_name":  server.Name,
		"server_host":  server.Host,
		"server_port":  server.Port,
		"server_group": server.ServerGroup,
		"server_type":  server.ServerType,
		"region":       server.Region,
	})
	audit.Log(h.repo, claimsUserID, claimsEmail, r.RemoteAddr,
		"server.create", "server", server.ID,
		fmt.Sprintf("Created server %s (%s) [%s]", server.Name, server.Host, server.ServerGroup),
		json.RawMessage(meta))

	// Auto-test connection after create to update status immediately
	sshCfg, cfgErr := h.sshConfigForServer(context.Background(), server)
	if cfgErr == nil {
		_, testErr := sshtool.TestConnection(context.Background(), sshCfg)
		if testErr == nil {
			server.Status = "online"
			_ = h.repo.UpdateServerInfo(context.Background(), server.ID, "", "")
			_ = h.repo.UpdateServerStatus(context.Background(), server.ID, "online")
		} else {
			server.Status = "offline"
			_ = h.repo.UpdateServerStatus(context.Background(), server.ID, "offline")
		}
	}

	common.JSON(w, http.StatusCreated, server.ToResponse())
}

// ─── Filter option endpoints ───────────────────────────────────────────────

func (h *Handler) ListGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := h.repo.ListServerGroups(r.Context())
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list groups")
		return
	}
	if groups == nil {
		groups = []string{}
	}
	common.JSON(w, http.StatusOK, groups)
}

func (h *Handler) ListRegions(w http.ResponseWriter, r *http.Request) {
	regions, err := h.repo.ListRegions(r.Context())
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list regions")
		return
	}
	common.JSON(w, http.StatusOK, regions)
}

func (h *Handler) ListTypes(w http.ResponseWriter, r *http.Request) {
	types, err := h.repo.ListServerTypes(r.Context())
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list types")
		return
	}
	common.JSON(w, http.StatusOK, types)
}

// ─── Bulk delete ───────────────────────────────────────────────────────────

func (h *Handler) BulkDelete(w http.ResponseWriter, r *http.Request) {
	if err := h.requireAdmin(r.Context()); err != nil {
		common.Error(w, http.StatusForbidden, "admin access required")
		return
	}
	var input struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(input.IDs) == 0 {
		common.Error(w, http.StatusBadRequest, "no server IDs provided")
		return
	}
	if err := h.repo.BulkDeleteServers(r.Context(), input.IDs); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to delete servers")
		return
	}

	// Audit log
	if claims := auth.GetClaims(r.Context()); claims != nil {
		meta, _ := json.Marshal(map[string]interface{}{
			"count":      len(input.IDs),
			"server_ids": input.IDs,
		})
		audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
			"server.bulk-delete", "server", "",
			fmt.Sprintf("Bulk deleted %d servers", len(input.IDs)),
			json.RawMessage(meta))
	}

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"message": fmt.Sprintf("Deleted %d servers", len(input.IDs)),
		"count":   len(input.IDs),
	})
}

// ─── Get single server ─────────────────────────────────────────────────────

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	srv, err := h.authorizeView(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "server not found")
		return
	}
	common.JSON(w, http.StatusOK, srv.ToResponse())
}

// ─── Update server ─────────────────────────────────────────────────────────

type updateServerInput struct {
	Name        *string   `json:"name,omitempty"`
	Host        *string   `json:"host,omitempty"`
	Port        *int      `json:"port,omitempty"`
	SSHUser     *string   `json:"ssh_user,omitempty"`
	SSHAuthType *string   `json:"ssh_auth_type,omitempty"`
	SSHKeyID    *string   `json:"ssh_key_id,omitempty"`
	SSHKey      *string   `json:"ssh_key,omitempty"`
	SSHPassword *string   `json:"ssh_password,omitempty"`
	Tags        *[]string `json:"tags,omitempty"`
	ServerGroup *string   `json:"server_group,omitempty"`
	Region      *string   `json:"region,omitempty"`
	ServerType  *string   `json:"server_type,omitempty"`
	Description *string   `json:"description,omitempty"`
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	if err := h.requireAdmin(r.Context()); err != nil {
		common.Error(w, http.StatusForbidden, "admin access required")
		return
	}
	id := chi.URLParam(r, "id")

	srv, err := h.repo.GetServerByIDFull(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "server not found")
		return
	}

	var input updateServerInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if input.Name != nil {
		srv.Name = *input.Name
	}
	if input.Host != nil {
		srv.Host = *input.Host
	}
	if input.Port != nil {
		srv.Port = *input.Port
	}
	if input.SSHUser != nil {
		srv.SSHUser = *input.SSHUser
	}
	if input.SSHAuthType != nil {
		srv.SSHAuthType = *input.SSHAuthType
	}
	if input.SSHKeyID != nil {
		srv.SSHKeyID = *input.SSHKeyID
		// If switching to a saved key, resolve its content
		if *input.SSHKeyID != "" {
			savedKey, err := h.repo.GetSSHKeyByIDFull(r.Context(), *input.SSHKeyID)
			if err == nil {
				srv.SSHKey = savedKey.PrivateKey
			}
		}
	}
	// Only overwrite SSH key/password when explicitly provided (non-empty).
	// Prevents empty strings from saved-key/password workflows wiping the stored value.
	if input.SSHKey != nil && *input.SSHKey != "" {
		srv.SSHKey = *input.SSHKey
	}
	if input.SSHPassword != nil && *input.SSHPassword != "" {
		srv.SSHPassword = *input.SSHPassword
	}
	if input.Tags != nil {
		srv.Tags = *input.Tags
	}
	if input.ServerGroup != nil {
		srv.ServerGroup = *input.ServerGroup
	}
	if input.Region != nil {
		srv.Region = *input.Region
	}
	if input.ServerType != nil {
		srv.ServerType = *input.ServerType
	}
	if input.Description != nil {
		srv.Description = *input.Description
	}

	if err := h.repo.UpdateServer(r.Context(), srv); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to update server")
		return
	}

	// Audit log
	if claims := auth.GetClaims(r.Context()); claims != nil {
		changedFields := []string{}
		if input.Name != nil && *input.Name != srv.Name {
			changedFields = append(changedFields, "name")
		}
		if input.Host != nil && *input.Host != srv.Host {
			changedFields = append(changedFields, "host")
		}
		if input.ServerGroup != nil {
			changedFields = append(changedFields, "group")
		}
		if input.ServerType != nil {
			changedFields = append(changedFields, "type")
		}
		if input.Region != nil {
			changedFields = append(changedFields, "region")
		}
		if input.Tags != nil {
			changedFields = append(changedFields, "tags")
		}
		meta, _ := json.Marshal(map[string]interface{}{
			"server_name":    srv.Name,
			"server_host":    srv.Host,
			"changed_fields": changedFields,
		})
		audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
			"server.update", "server", srv.ID,
			fmt.Sprintf("Updated server %s (%s)", srv.Name, srv.Host),
			json.RawMessage(meta))
	}

	common.JSON(w, http.StatusOK, srv.ToResponse())
}

// ─── Delete server ─────────────────────────────────────────────────────────

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.requireAdmin(r.Context()); err != nil {
		common.Error(w, http.StatusForbidden, "admin access required")
		return
	}
	id := chi.URLParam(r, "id")

	// Get server name before deleting (for audit)
	srv, _ := h.repo.GetServerByID(r.Context(), id)
	srvName := id
	if srv != nil {
		srvName = srv.Name
	}

	if err := h.repo.DeleteServer(r.Context(), id); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to delete server")
		return
	}

	// Audit log
	if claims := auth.GetClaims(r.Context()); claims != nil {
		meta, _ := json.Marshal(map[string]string{
			"server_name": srvName,
			"server_id":   id,
		})
		audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
			"server.delete", "server", id,
			fmt.Sprintf("Deleted server %s", srvName),
			json.RawMessage(meta))
	}

	common.JSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}

// ─── Detect server info via SSH ────────────────────────────────────────────

func (h *Handler) DetectInfo(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	srv, err := h.authorizeWrite(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "server not found")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Docker socket path
	if srv.ConnectionType == executor.ConnectionTypeDockerSocket {
		exec, err := h.getExecutor(srv)
		if err != nil {
			common.Error(w, http.StatusInternalServerError, "executor init: "+err.Error())
			return
		}
		defer exec.Close()

		info, err := exec.GetServerInfo(ctx)
		if err != nil {
			common.Error(w, http.StatusInternalServerError, "failed to detect server info: "+err.Error())
			return
		}

		osInfo := info.OS
		if info.Kernel != "" {
			osInfo = info.OS + " (" + info.Kernel + ")"
		}
		cpuInfo := info.CPUModel
		if info.CPUCores > 0 {
			cpuInfo = fmt.Sprintf("%s (%d cores)", cpuInfo, info.CPUCores)
		}

		_ = h.repo.UpdateServerInfo(ctx, id, osInfo, cpuInfo)
		_ = h.repo.UpdateServerStatus(ctx, id, "online")

		common.JSON(w, http.StatusOK, info)
		return
	}

	// SSH path (existing)
	sshCfg, err := h.sshConfigForServer(ctx, srv)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "SSH key resolution: "+err.Error())
		return
	}

	info, err := sshtool.GetServerInfo(ctx, sshCfg)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to detect server info: "+err.Error())
		return
	}

	// Save to DB
	osInfo := info.OS
	if info.Kernel != "" {
		osInfo = info.OS + " (" + info.Kernel + ")"
	}
	cpuInfo := info.CPUModel
	if info.CPUCores > 0 {
		cpuInfo = fmt.Sprintf("%s (%d cores)", cpuInfo, info.CPUCores)
	}

	_ = h.repo.UpdateServerInfo(ctx, id, osInfo, cpuInfo)
	_ = h.repo.UpdateServerStatus(ctx, id, "online")

	common.JSON(w, http.StatusOK, info)
}

// ─── Test connection endpoints ─────────────────────────────────────────────

func (h *Handler) TestConnectionGlobal(w http.ResponseWriter, r *http.Request) {
	var input model.TestConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Resolve saved SSH key if ssh_key_id is provided
	keyContent := input.SSHKey
	if input.SSHKeyID != "" && keyContent == "" {
		savedKey, err := h.repo.GetSSHKeyByIDFull(r.Context(), input.SSHKeyID)
		if err == nil {
			keyContent = savedKey.PrivateKey
		} else {
			log.Printf("[debug] GetSSHKeyByIDFull(%q) error: %v", input.SSHKeyID, err)
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	hostname, err := sshtool.TestConnection(ctx, sshtool.Config{
		Host:     input.Host,
		Port:     input.Port,
		User:     input.SSHUser,
		AuthType: input.SSHAuthType,
		Key:      keyContent,
		Password: input.SSHPassword,
	})
	if err != nil {
		common.JSON(w, http.StatusOK, map[string]interface{}{
			"reachable": false,
			"hostname":  "",
			"error":     err.Error(),
		})
		return
	}

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"reachable": true,
		"hostname":  hostname,
		"error":     nil,
	})
}

func (h *Handler) TestConnection(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	srv, err := h.authorizeWrite(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "server not found")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// Docker socket path
	if srv.ConnectionType == executor.ConnectionTypeDockerSocket {
		exec, err := h.getExecutor(srv)
		if err != nil {
			_ = h.repo.UpdateServerStatus(r.Context(), id, "offline")
			common.JSON(w, http.StatusOK, map[string]interface{}{
				"reachable": false,
				"hostname":  "",
				"error":     "executor init: " + err.Error(),
			})
			return
		}
		defer exec.Close()

		hostname, err := exec.TestConnection(ctx)
		if err != nil {
			_ = h.repo.UpdateServerStatus(r.Context(), id, "offline")
			common.JSON(w, http.StatusOK, map[string]interface{}{
				"reachable": false,
				"hostname":  "",
				"error":     err.Error(),
			})
			return
		}
		_ = h.repo.UpdateServerStatus(r.Context(), id, "online")
		common.JSON(w, http.StatusOK, map[string]interface{}{
			"reachable": true,
			"hostname":  hostname,
			"error":     nil,
		})
		return
	}

	// SSH path (existing)
	sshCfg, err := h.sshConfigForServer(ctx, srv)
	if err != nil {
		_ = h.repo.UpdateServerStatus(r.Context(), id, "offline")
		common.JSON(w, http.StatusOK, map[string]interface{}{
			"reachable": false,
			"hostname":  "",
			"error":     "SSH key resolution: " + err.Error(),
		})
		return
	}

	hostname, err := sshtool.TestConnection(ctx, sshCfg)
	if err != nil {
		_ = h.repo.UpdateServerStatus(r.Context(), id, "offline")
		common.JSON(w, http.StatusOK, map[string]interface{}{
			"reachable": false,
			"hostname":  "",
			"error":     err.Error(),
		})
		return
	}

	_ = h.repo.UpdateServerStatus(r.Context(), id, "online")
	common.JSON(w, http.StatusOK, map[string]interface{}{
		"reachable": true,
		"hostname":  hostname,
		"error":     nil,
	})
}

// ─── Server Metrics (live snapshot) ────────────────────────────────────────

type ServerMetrics struct {
	CPU struct {
		Load1  float64 `json:"load_1"`
		Load5  float64 `json:"load_5"`
		Load15 float64 `json:"load_15"`
	} `json:"cpu"`
	Memory struct {
		Total int64 `json:"total_bytes"`
		Used  int64 `json:"used_bytes"`
		Free  int64 `json:"free_bytes"`
	} `json:"memory"`
	Disk struct {
		Total     int64   `json:"total_bytes"`
		Used      int64   `json:"used_bytes"`
		Available int64   `json:"available_bytes"`
		UsedPct   float64 `json:"used_percent"`
	} `json:"disk"`
	Network struct {
		RXBytes int64 `json:"rx_bytes"`
		TXBytes int64 `json:"tx_bytes"`
	} `json:"network"`
	Uptime string `json:"uptime"`
}

func (h *Handler) Metrics(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	srv, err := h.authorizeView(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "server not found")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Docker socket path
	if srv.ConnectionType == executor.ConnectionTypeDockerSocket {
		exec, err := h.getExecutor(srv)
		if err != nil {
			common.Error(w, http.StatusInternalServerError, "executor init: "+err.Error())
			return
		}
		defer exec.Close()

		em, err := exec.GetMetrics(ctx)
		if err != nil {
			common.Error(w, http.StatusInternalServerError, "metrics: "+err.Error())
			return
		}

		var load1, load5, load15 float64
		fmt.Sscanf(em.CPULoad1, "%f", &load1)
		fmt.Sscanf(em.CPULoad5, "%f", &load5)
		fmt.Sscanf(em.CPULoad15, "%f", &load15)

		metrics := ServerMetrics{
			Uptime: em.Uptime,
		}
		metrics.CPU.Load1 = load1
		metrics.CPU.Load5 = load5
		metrics.CPU.Load15 = load15
		metrics.Memory.Total = int64(em.MemoryTotal)
		metrics.Memory.Used = int64(em.MemoryUsed)
		metrics.Memory.Free = int64(em.MemoryFree)
		metrics.Disk.Total = int64(em.DiskTotal)
		metrics.Disk.Used = int64(em.DiskUsed)
		metrics.Disk.Available = int64(em.DiskFree)
		metrics.Disk.UsedPct = em.DiskUsedPct
		metrics.Network.RXBytes = em.NetRX
		metrics.Network.TXBytes = em.NetTX

		if load1 != 0 || load5 != 0 || load15 != 0 {
			point := &model.ServerMetricsPoint{
				ServerID:       id,
				CPULoad1:       load1,
				CPULoad5:       load5,
				CPULoad15:      load15,
				MemUsedBytes:   int64(em.MemoryUsed),
				MemTotalBytes:  int64(em.MemoryTotal),
				DiskUsedBytes:  int64(em.DiskUsed),
				DiskTotalBytes: int64(em.DiskTotal),
				DiskUsedPct:    em.DiskUsedPct,
				NetRXBytes:     em.NetRX,
				NetTXBytes:     em.NetTX,
				CollectedAt:    time.Now(),
			}
			_ = h.repo.SaveMetrics(ctx, point)
			h.checkThresholds(ctx, id, point)
		}

		common.JSON(w, http.StatusOK, metrics)
		return
	}

	// SSH path (existing)
	sshCfg, err := h.sshConfigForServer(ctx, srv)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "SSH key resolution: "+err.Error())
		return
	}

	var metrics ServerMetrics
	var hasError bool

	// CPU
	cpuOut, err := sshtool.RunCommand(ctx, sshCfg, "cat /proc/loadavg")
	if err != nil {
		hasError = true
	} else {
		fmt.Sscanf(cpuOut, "%f %f %f", &metrics.CPU.Load1, &metrics.CPU.Load5, &metrics.CPU.Load15)
	}

	// Memory
	memOut, err := sshtool.RunCommand(ctx, sshCfg, "free -b | awk 'NR==2{print $2,$3,$4,$7}'")
	if err != nil {
		hasError = true
	} else {
		fmt.Sscanf(memOut, "%d %d %d %d",
			&metrics.Memory.Total, &metrics.Memory.Used, &metrics.Memory.Free, new(int64))
	}

	// Disk
	diskOut, err := sshtool.RunCommand(ctx, sshCfg, `df -B1 / | awk 'NR==2{print $2,$3,$4,$5}'`)
	if err != nil {
		hasError = true
	} else {
		var usePctStr string
		if _, err := fmt.Sscanf(diskOut, "%d %d %d %s",
			&metrics.Disk.Total, &metrics.Disk.Used, &metrics.Disk.Available, &usePctStr); err == nil {
			fmt.Sscanf(usePctStr, "%f%%", &metrics.Disk.UsedPct)
		}
	}

	// Network
	rx, tx, err := sshtool.GetNetTraffic(ctx, sshCfg)
	if err == nil {
		metrics.Network.RXBytes = rx
		metrics.Network.TXBytes = tx
	}

	// Uptime
	uptimeOut, err := sshtool.RunCommand(ctx, sshCfg, "uptime -p")
	if err == nil {
		metrics.Uptime = uptimeOut
	}

	// Save to historical metrics
	if !hasError {
		point := &model.ServerMetricsPoint{
			ServerID:       id,
			CPULoad1:       metrics.CPU.Load1,
			CPULoad5:       metrics.CPU.Load5,
			CPULoad15:      metrics.CPU.Load15,
			MemUsedBytes:   metrics.Memory.Used,
			MemTotalBytes:  metrics.Memory.Total,
			DiskUsedBytes:  metrics.Disk.Used,
			DiskTotalBytes: metrics.Disk.Total,
			DiskUsedPct:    metrics.Disk.UsedPct,
			NetRXBytes:     metrics.Network.RXBytes,
			NetTXBytes:     metrics.Network.TXBytes,
			CollectedAt:    time.Now(),
		}
		_ = h.repo.SaveMetrics(ctx, point)

		// Check thresholds and create alerts
		h.checkThresholds(ctx, id, point)
	}

	common.JSON(w, http.StatusOK, metrics)
}

// ─── Threshold alerts ──────────────────────────────────────────────────────

func (h *Handler) checkThresholds(ctx context.Context, serverID string, m *model.ServerMetricsPoint) {
	alerts := []struct {
		alertType string
		severity  string
		message   string
		value     string
		threshold string
		condition bool
	}{
		{"disk", "critical", "Disk usage critical", fmt.Sprintf("%.1f%%", m.DiskUsedPct), "> 90%", m.DiskUsedPct > 90},
		{"disk", "warning", "Disk usage warning", fmt.Sprintf("%.1f%%", m.DiskUsedPct), "> 80%", m.DiskUsedPct > 80 && m.DiskUsedPct <= 90},
		{"memory", "critical", "Memory usage critical", fmt.Sprintf("%d MB", m.MemUsedBytes/1024/1024), "> 90%", m.MemTotalBytes > 0 && float64(m.MemUsedBytes)/float64(m.MemTotalBytes)*100 > 90},
		{"memory", "warning", "Memory usage warning", fmt.Sprintf("%d MB", m.MemUsedBytes/1024/1024), "> 80%", m.MemTotalBytes > 0 && float64(m.MemUsedBytes)/float64(m.MemTotalBytes)*100 > 80},
		{"cpu", "warning", "CPU load high (15min)", fmt.Sprintf("%.2f", m.CPULoad15), "> 2.0", m.CPULoad15 > 2.0},
	}

	for _, a := range alerts {
		if a.condition {
			alert := &model.Alert{
				ID:        uuid.New().String(),
				ServerID:  serverID,
				Type:      a.alertType + "_usage",
				Severity:  a.severity,
				Message:   a.message,
				Value:     a.value,
				Threshold: a.threshold,
			}
			alert.CreatedAt = time.Now()
			_ = h.repo.CreateAlert(ctx, alert)
		}
	}
}

// ─── Containers ────────────────────────────────────────────────────────────

type DockerContainer struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Image   string `json:"image"`
	Status  string `json:"status"`
	State   string `json:"state"`
	Ports   string `json:"ports"`
	Created string `json:"created"`
	Uptime  string `json:"uptime"`
}

// runDockerOnServer executes a docker command either via SSH or local Docker socket.
// For SSH connections, it runs "docker <cmd>" on the remote server.
// For docker-socket connections, it runs docker CLI locally via the mounted socket.
func (h *Handler) runDockerOnServer(ctx context.Context, srv *model.Server, dockerArgs ...string) (string, error) {
	if srv.ConnectionType == executor.ConnectionTypeDockerSocket {
		exec, err := h.getExecutor(srv)
		if err != nil {
			return "", fmt.Errorf("executor init: %w", err)
		}
		defer exec.Close()
		return exec.RunDockerCommand(ctx, dockerArgs...)
	}

	// SSH path
	cfg, err := h.sshConfigForServer(ctx, srv)
	if err != nil {
		return "", fmt.Errorf("ssh config: %w", err)
	}
	return sshtool.RunCommand(ctx, cfg, "docker "+strings.Join(dockerArgs, " "))
}

func (h *Handler) Containers(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	srv, err := h.authorizeView(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "server not found")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	out, err := h.runDockerOnServer(ctx, srv,
		"ps", "-a", "--format", `{"id":"{{.ID}}","name":"{{.Names}}","image":"{{.Image}}","status":"{{.Status}}","state":"{{.State}}","ports":"{{.Ports}}","created":"{{.CreatedAt}}"}`,
	)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list containers: "+err.Error())
		return
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	containers := make([]DockerContainer, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var c DockerContainer
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			continue
		}
		containers = append(containers, c)
	}

	_ = h.repo.UpdateServerContainerCount(r.Context(), id, len(containers))

	common.JSON(w, http.StatusOK, containers)
}

// ─── Container Actions ─────────────────────────────────────────────────

// containerSSHExec runs a docker command on the server via SSH
func (h *Handler) containerSSHExec(ctx context.Context, serverID, containerID, dockerCmd string) (string, error) {
	srv, err := h.repo.GetServerByIDFull(ctx, serverID)
	if err != nil {
		return "", fmt.Errorf("server not found: %w", err)
	}
	return h.runDockerOnServer(ctx, srv, dockerCmd, containerID)
}

func (h *Handler) ContainerStart(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	container := chi.URLParam(r, "container")
	if _, err := h.authorizeWrite(r.Context(), id); err != nil {
		common.Error(w, http.StatusForbidden, "write access required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	out, err := h.containerSSHExec(ctx, id, container, "start")
	if err != nil {
		common.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Audit log
	claims := auth.GetClaims(r.Context())
	if claims != nil {
		meta, _ := json.Marshal(map[string]string{"container_name": container})
		audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
			"container.start", "container", container,
			fmt.Sprintf("Started container %s on %s", container, id),
			json.RawMessage(meta))
	}
	common.JSON(w, http.StatusOK, map[string]interface{}{
		"message": fmt.Sprintf("Container %s started", container),
		"output":  out,
	})
}

func (h *Handler) ContainerStop(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	container := chi.URLParam(r, "container")
	if _, err := h.authorizeWrite(r.Context(), id); err != nil {
		common.Error(w, http.StatusForbidden, "write access required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	out, err := h.containerSSHExec(ctx, id, container, "stop")
	if err != nil {
		common.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	claims := auth.GetClaims(r.Context())
	if claims != nil {
		meta, _ := json.Marshal(map[string]string{"container_name": container})
		audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
			"container.stop", "container", container,
			fmt.Sprintf("Stopped container %s on %s", container, id),
			json.RawMessage(meta))
	}
	common.JSON(w, http.StatusOK, map[string]interface{}{
		"message": fmt.Sprintf("Container %s stopped", container),
		"output":  out,
	})
}

func (h *Handler) ContainerRestart(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	container := chi.URLParam(r, "container")
	if _, err := h.authorizeWrite(r.Context(), id); err != nil {
		common.Error(w, http.StatusForbidden, "write access required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	out, err := h.containerSSHExec(ctx, id, container, "restart")
	if err != nil {
		common.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	claims := auth.GetClaims(r.Context())
	if claims != nil {
		meta, _ := json.Marshal(map[string]string{"container_name": container})
		audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
			"container.restart", "container", container,
			fmt.Sprintf("Restarted container %s on %s", container, id),
			json.RawMessage(meta))
	}
	common.JSON(w, http.StatusOK, map[string]interface{}{
		"message": fmt.Sprintf("Container %s restarted", container),
		"output":  out,
	})
}

func (h *Handler) ContainerLogs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	container := chi.URLParam(r, "container")
	if _, err := h.authorizeView(r.Context(), id); err != nil {
		common.Error(w, http.StatusNotFound, "server not found")
		return
	}
	tail := r.URL.Query().Get("tail")
	if tail == "" {
		tail = "50"
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	out, err := h.containerSSHExec(ctx, id, container, fmt.Sprintf("logs --tail=%s", tail))
	if err != nil {
		common.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	common.JSON(w, http.StatusOK, map[string]interface{}{
		"logs": out,
	})
}

func (h *Handler) ContainerInspect(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	container := chi.URLParam(r, "container")
	if _, err := h.authorizeView(r.Context(), id); err != nil {
		common.Error(w, http.StatusNotFound, "server not found")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	out, err := h.containerSSHExec(ctx, id, container, "inspect")
	if err != nil {
		common.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Try to parse JSON output
	var parsed interface{}
	if json.Unmarshal([]byte(out), &parsed) == nil {
		common.JSON(w, http.StatusOK, parsed)
		return
	}
	common.JSON(w, http.StatusOK, map[string]interface{}{
		"raw": out,
	})
}

func (h *Handler) ContainerExec(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	container := chi.URLParam(r, "container")
	if _, err := h.authorizeWrite(r.Context(), id); err != nil {
		common.Error(w, http.StatusForbidden, "write access required")
		return
	}
	var req struct {
		Command string `json:"command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Command == "" {
		common.Error(w, http.StatusBadRequest, "command is required")
		return
	}

	srv, err := h.repo.GetServerByIDFull(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "server not found")
		return
	}

	// Resolve container name for audit log (SSH path only, Docker socket skips)
	containerName := container
	if srv.ConnectionType != executor.ConnectionTypeDockerSocket {
		sshCfg, sshErr := h.sshConfigForServer(r.Context(), srv)
		if sshErr == nil {
			nameCmd := fmt.Sprintf("docker ps --filter id=%s --format '{{.Names}}'", container)
			if nameOut, nameErr := sshtool.RunCommand(r.Context(), sshCfg, nameCmd); nameErr == nil {
				if n := strings.TrimSpace(nameOut); n != "" {
					containerName = n
				}
			}
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	out, err := h.runDockerOnServer(ctx, srv, "exec", container, req.Command)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, fmt.Sprintf("exec error: %v", err))
		return
	}

	// Audit log
	if claims := auth.GetClaims(r.Context()); claims != nil {
		meta, _ := json.Marshal(map[string]string{
			"container_id":   container,
			"container_name": containerName,
			"server_id":      id,
			"command":        req.Command,
		})
		audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
			"container.exec", "container", containerName,
			fmt.Sprintf("Executed command in container %s on %s", containerName, srv.Name),
			json.RawMessage(meta))
	}

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"output": out,
	})
}

// ─── WebSocket SSH Terminal ────────────────────────────────────────────────

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (h *Handler) TerminalWS(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	srv, err := h.authorizeWrite(r.Context(), id)
	if err != nil {
		// WebSocket upgrade not yet done — send regular HTTP error
		common.Error(w, http.StatusNotFound, "server not found")
		return
	}

	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	if srv.ConnectionType == executor.ConnectionTypeDockerSocket {
		// Docker-socket: spawn a host shell via nsenter
		h.dockerTerminal(ctx, conn, cancel)
		return
	}

	// SSH path
	cfg, err := h.sshConfigForServer(ctx, srv)
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\n\x1b[31mSSH key resolution failed: %v\x1b[0m\r\n", err)))
		return
	}
	cfg.Timeout = 10 * time.Second

	_, session, combined, stdin, err := sshtool.NewTerminalSession(ctx, cfg, "xterm-256color", 40, 120)
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\n\x1b[31mConnection failed: %v\x1b[0m\r\n", err)))
		return
	}
	defer session.Close()

	_ = h.repo.UpdateServerStatus(ctx, id, "online")

	// SSH output → WebSocket
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := combined.Read(buf)
			if n > 0 {
				conn.WriteMessage(websocket.TextMessage, buf[:n])
			}
			if err != nil {
				conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\n\x1b[33m[Connection closed]\x1b[0m\r\n")))
				cancel()
				return
			}
		}
	}()

	// WebSocket → SSH stdin
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}

		msgStr := strings.TrimSpace(string(msg))
		if strings.HasPrefix(msgStr, "{") && strings.Contains(msgStr, "resize") {
			var resizeMsg struct {
				Resize bool `json:"resize"`
				Cols   int  `json:"cols"`
				Rows   int  `json:"rows"`
			}
			if err := json.Unmarshal(msg, &resizeMsg); err == nil && resizeMsg.Resize && resizeMsg.Cols > 0 && resizeMsg.Rows > 0 {
				sshtool.ResizeTerminal(session, resizeMsg.Rows, resizeMsg.Cols)
			}
			continue
		}

		if _, err := io.WriteString(stdin, string(msg)); err != nil {
			break
		}
	}
}

// dockerTerminal runs an interactive host shell via Docker nsenter with PTY, connected to the WebSocket.
func (h *Handler) dockerTerminal(ctx context.Context, conn *websocket.Conn, cancel context.CancelFunc) {
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm", "-it", "--pid=host", "--privileged",
		"--net=host", "-v", "/:/host:ro",
		"alpine:latest",
		"nsenter", "-t", "1", "-m", "-u", "-i", "-n",
		"sh", "-c", "exec bash 2>/dev/null || exec sh")

	cmd.Env = append(cmd.Env,
		"DOCKER_HOST=unix:///var/run/docker.sock",
		"TERM=xterm-256color")

	// Start with a PTY so docker allocates a real TTY (needed for vim, htop, nano, etc.)
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 40, Cols: 120})
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\n\x1b[31mFailed to start shell: %v\x1b[0m\r\n", err)))
		return
	}
	defer ptmx.Close()

	// PTY output → WebSocket (binary to handle raw terminal sequences safely)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				conn.WriteMessage(websocket.BinaryMessage, buf[:n])
			}
			if err != nil {
				conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\n\x1b[33m[Connection closed]\x1b[0m\r\n")))
				cancel()
				return
			}
		}
	}()

	// WebSocket → PTY input, with resize handling
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}

		msgStr := strings.TrimSpace(string(msg))
		if strings.HasPrefix(msgStr, "{") && strings.Contains(msgStr, "resize") {
			var resizeReq struct {
				Resize bool `json:"resize"`
				Cols   int  `json:"cols"`
				Rows   int  `json:"rows"`
			}
			if err := json.Unmarshal(msg, &resizeReq); err == nil && resizeReq.Resize && resizeReq.Cols > 0 && resizeReq.Rows > 0 {
				pty.Setsize(ptmx, &pty.Winsize{
					Rows: uint16(resizeReq.Rows),
					Cols: uint16(resizeReq.Cols),
				})
			}
			continue
		}

		if _, err := ptmx.Write(msg); err != nil {
			break
		}
	}

	cmd.Process.Kill()
	cmd.Wait()
}

// ─── WebSocket Container Exec Terminal ────────────────────────────────────

func (h *Handler) ContainerExecWS(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	container := chi.URLParam(r, "container")

	srv, err := h.authorizeWrite(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "server not found")
		return
	}

	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Audit log — call before entering interactive session
	containerName := container
	if claims := auth.GetClaims(r.Context()); claims != nil {
		meta, _ := json.Marshal(map[string]string{
			"container_id":   container,
			"container_name": containerName,
			"server_id":      id,
			"server_name":    srv.Name,
		})
		audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
			"container.exec", "container", containerName,
			fmt.Sprintf("Opened interactive terminal in container %s on %s", containerName, srv.Name),
			json.RawMessage(meta))
	}

	if srv.ConnectionType == executor.ConnectionTypeDockerSocket {
		// Docker-socket: exec directly into the container with PTY
		// Detect shell
		detectCmd := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.Config.Cmd}}", container)
		detectCmd.Env = append(detectCmd.Env, "DOCKER_HOST=unix:///var/run/docker.sock")
		shellOut, _ := detectCmd.Output()
		shell := "sh"
		if strings.Contains(string(shellOut), "bash") {
			shell = "bash"
		}

		cmd := exec.CommandContext(ctx, "docker", "exec", "-it", container, shell)
		cmd.Env = append(cmd.Env,
			"DOCKER_HOST=unix:///var/run/docker.sock",
			"TERM=xterm-256color")

		// Start with PTY for full TTY support (vim, nano, htop, etc.)
		ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 40, Cols: 120})
		if err != nil {
			conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\n\x1b[31mContainer exec failed: %v\x1b[0m\r\n", err)))
			return
		}
		defer ptmx.Close()

		// PTY output → WebSocket
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := ptmx.Read(buf)
				if n > 0 {
					conn.WriteMessage(websocket.BinaryMessage, buf[:n])
				}
				if err != nil {
					conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\n\x1b[33m[Connection closed]\x1b[0m\r\n")))
					cancel()
					return
				}
			}
		}()

		// WebSocket → PTY input with resize
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				break
			}

			msgStr := strings.TrimSpace(string(msg))
			if strings.HasPrefix(msgStr, "{") && strings.Contains(msgStr, "resize") {
				var resizeReq struct {
					Resize bool `json:"resize"`
					Cols   int  `json:"cols"`
					Rows   int  `json:"rows"`
				}
				if err := json.Unmarshal(msg, &resizeReq); err == nil && resizeReq.Resize && resizeReq.Cols > 0 && resizeReq.Rows > 0 {
					pty.Setsize(ptmx, &pty.Winsize{
						Rows: uint16(resizeReq.Rows),
						Cols: uint16(resizeReq.Cols),
					})
				}
				continue
			}

			if _, err := ptmx.Write(msg); err != nil {
				break
			}
		}

		cmd.Process.Kill()
		cmd.Wait()
		return
	}

	// SSH path
	cfg, err := h.sshConfigForServer(ctx, srv)
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\n\x1b[31mSSH key resolution failed: %v\x1b[0m\r\n", err)))
		return
	}
	cfg.Timeout = 10 * time.Second

	// Resolve container name for audit log
	if nameOut, nameErr := sshtool.RunCommand(ctx, cfg, fmt.Sprintf("docker ps --filter id=%s --format '{{.Names}}'", container)); nameErr == nil {
		if n := strings.TrimSpace(nameOut); n != "" {
			containerName = n
		}
	}

	// Detect available shell inside container
	shell := sshtool.DetectContainerShell(ctx, cfg, container)
	dockerCmd := fmt.Sprintf("docker exec -it %s %s", container, shell)
	_, session, combined, stdin, err := sshtool.NewContainerExecSession(ctx, cfg, dockerCmd, "xterm-256color", 40, 120)
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\n\x1b[31mContainer exec failed: %v\x1b[0m\r\n", err)))
		return
	}
	defer session.Close()

	// SSH output → WebSocket
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := combined.Read(buf)
			if n > 0 {
				conn.WriteMessage(websocket.TextMessage, buf[:n])
			}
			if err != nil {
				conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\n\x1b[33m[Connection closed]\x1b[0m\r\n")))
				cancel()
				return
			}
		}
	}()

	// WebSocket → SSH stdin
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}

		msgStr := strings.TrimSpace(string(msg))

		if strings.HasPrefix(msgStr, "{") && strings.Contains(msgStr, "resize") {
			var resizeMsg struct {
				Resize bool `json:"resize"`
				Cols   int  `json:"cols"`
				Rows   int  `json:"rows"`
			}
			if err := json.Unmarshal(msg, &resizeMsg); err == nil && resizeMsg.Resize && resizeMsg.Cols > 0 && resizeMsg.Rows > 0 {
				sshtool.ResizeTerminal(session, resizeMsg.Rows, resizeMsg.Cols)
			}
			continue
		}

		if _, err := io.WriteString(stdin, string(msg)); err != nil {
			break
		}
	}
}
