package compliance

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/edsuwarna/anjungan/internal/auth"
	"github.com/edsuwarna/anjungan/internal/common"
	"github.com/edsuwarna/anjungan/internal/common/db"
	"github.com/edsuwarna/anjungan/internal/common/model"
	sshtool "github.com/edsuwarna/anjungan/internal/infra/ssh"
)

// Handler handles HTTP requests for compliance scanning.
type Handler struct {
	repo    *db.Repository
	scanner *Scanner
}

// NewHandler creates a new compliance Handler.
func NewHandler(repo *db.Repository) *Handler {
	return &Handler{repo: repo, scanner: NewScannerWithRegistry(NewCheckRegistry())}
}

// Routes returns the chi.Router for compliance endpoints.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/summary", h.Summary)
	r.Get("/checks", h.ListChecks)
	r.Get("/history", h.GlobalHistory)
	r.Get("/active", h.ActiveScans)
	r.Route("/{serverID}", func(r chi.Router) {
		r.Get("/latest", h.LatestScan)
		r.Get("/latest/categories", h.LatestCategories)
		r.Post("/scan", h.TriggerScan)
		r.Post("/scan/lynis", h.TriggerLynisScan)
		r.Post("/scan/docker", h.TriggerDockerScan)
		r.Post("/scan/containers", h.TriggerContainerScan)
		r.Post("/scan/check/{checkID}", h.TriggerSingleCheck)
		r.Get("/history/categories/{category}", h.CategoryHistory)
		r.Get("/history", h.ScanHistory)
		r.Get("/history/{scanID}", h.ScanDetail)
	})
	return r
}

// ─── Authorization helpers ────────────────────────────────────────────────

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
		Timeout:  60 * time.Second,
	}, nil
}

func (h *Handler) authorizeView(ctx context.Context, id string) (*model.Server, error) {
	claims := auth.GetClaims(ctx)
	if claims == nil {
		return nil, fmt.Errorf("unauthorized")
	}
	if claims.Role == model.RoleAdmin {
		return h.repo.GetServerByIDFull(ctx, id)
	}
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

// ─── GET /compliance/checks ───────────────────────────────────────────────

// ListChecks returns all available compliance checks with metadata.
func (h *Handler) ListChecks(w http.ResponseWriter, r *http.Request) {
	checks := h.scanner.Registry().ListChecks()
	categories := h.scanner.Registry().Categories()

	response := map[string]interface{}{
		"checks":     checks,
		"categories": categories,
		"total":      len(checks),
	}
	common.JSON(w, http.StatusOK, response)
}

// ─── GET /compliance/summary ──────────────────────────────────────────────

func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	summary, err := h.repo.GetComplianceSummary(r.Context())
	if err != nil {
		log.Err(err).Msg("failed to get compliance summary")
		common.Error(w, http.StatusInternalServerError, "failed to get compliance summary")
		return
	}
	common.JSON(w, http.StatusOK, summary)
}

// ─── GET /compliance/{serverID}/latest ────────────────────────────────────

func (h *Handler) LatestScan(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "serverID")

	_, err := h.authorizeView(r.Context(), serverID)
	if err != nil {
		common.Error(w, http.StatusNotFound, "server not found")
		return
	}

	// Support ?scan_type= filter
	scanType := r.URL.Query().Get("scan_type")
	var result *model.ScanResult
	if scanType != "" {
		result, err = h.repo.GetLatestScanResultByType(r.Context(), serverID, scanType)
	} else {
		result, err = h.repo.GetLatestScanResult(r.Context(), serverID)
	}
	if err != nil {
		common.Error(w, http.StatusNotFound, "no scan results found")
		return
	}

	// Support ?category= filter
	categoryFilter := r.URL.Query().Get("category")
	if categoryFilter != "" {
		findings, err := h.repo.GetFindingsByScanIDAndCategory(r.Context(), result.ID, categoryFilter)
		if err != nil {
			result.Findings = []model.ScanFinding{}
		} else {
			result.Findings = findings
			// Recalculate summary based on filtered findings
			var total, passed, warnings, criticals int
			for _, f := range findings {
				total++
				switch f.Status {
				case "pass":
					passed++
				case "warn":
					warnings++
				case "fail":
					criticals++
				}
			}
			result.TotalChecks = total
			result.Passed = passed
			result.Warnings = warnings
			result.Criticals = criticals
		}
	} else {
		findings, err := h.repo.GetFindingsByScanID(r.Context(), result.ID)
		if err != nil {
			result.Findings = []model.ScanFinding{}
		} else {
			result.Findings = findings
		}
	}

	common.JSON(w, http.StatusOK, result)
}

// ─── POST /compliance/{serverID}/scan ─────────────────────────────────────

func (h *Handler) TriggerScan(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "serverID")

	srv, err := h.authorizeView(r.Context(), serverID)
	if err != nil {
		common.Error(w, http.StatusNotFound, "server not found")
		return
	}

	// Parse profile query parameter
	profileStr := r.URL.Query().Get("profile")
	profile := ProfileAll
	switch profileStr {
	case "cis_level_1", "level1", "cis1":
		profile = ProfileCISLevel1
	case "cis_level_2", "level2", "cis2":
		profile = ProfileCISLevel2
	case "cis_docker", "docker":
		profile = ProfileDocker
	case "all", "":
		profile = ProfileAll
	default:
		common.Error(w, http.StatusBadRequest, "invalid profile: use 'cis_level_1', 'cis_level_2', 'cis_docker', or 'all'")
		return
	}

	now := time.Now()

	scanResult := &model.ScanResult{
		ID:        uuid.New().String(),
		ServerID:  serverID,
		ScanType:  profile.String(),
		Status:    "running",
		StartedAt: &now,
		CreatedAt: now,
	}

	if err := h.repo.CreateScanResult(r.Context(), scanResult); err != nil {
		log.Err(err).Msg("failed to create scan result")
		common.Error(w, http.StatusInternalServerError, "failed to create scan result")
		return
	}

	// Return immediately with scan ID — scan runs in background
	common.JSON(w, http.StatusAccepted, map[string]interface{}{
		"scan_id":   scanResult.ID,
		"status":    "running",
		"scan_type": scanResult.ScanType,
	})

	go func(sr *model.ScanResult, server *model.Server, prof ScanProfile) {
		ctx := context.Background()

		if err := h.resolveSSHKey(ctx, server); err != nil {
			completedAt := time.Now()
			zero := 0
			sr.Status = "failed"
			sr.ErrorMessage = "SSH key error: " + err.Error()
			sr.CompletedAt = &completedAt
			sr.Score = &zero
			_ = h.repo.UpdateScanResult(ctx, sr)
			log.Err(err).Str("scan_id", sr.ID).Msg("compliance scan failed: resolve SSH key")
			return
		}

		sshCfg, err := h.sshConfigForServer(ctx, server)
		if err != nil {
			completedAt := time.Now()
			zero := 0
			sr.Status = "failed"
			sr.ErrorMessage = "SSH config error: " + err.Error()
			sr.CompletedAt = &completedAt
			sr.Score = &zero
			_ = h.repo.UpdateScanResult(ctx, sr)
			log.Err(err).Str("scan_id", sr.ID).Msg("compliance scan failed: SSH config")
			return
		}

		summary, err := h.scanner.Run(ctx, sshCfg, prof)
		if err != nil {
			completedAt := time.Now()
			zero := 0
			sr.Status = "failed"
			sr.ErrorMessage = "Scan error: " + err.Error()
			sr.CompletedAt = &completedAt
			sr.Score = &zero
			_ = h.repo.UpdateScanResult(ctx, sr)
			log.Err(err).Str("scan_id", sr.ID).Msg("compliance scan failed")
			return
		}

		completedAt := time.Now()
		sr.Status = "completed"
		sr.Score = &summary.Score
		sr.TotalChecks = summary.Total
		sr.Passed = summary.Passed
		sr.Warnings = summary.Warnings
		sr.Criticals = summary.Criticals
		sr.CompletedAt = &completedAt

		if err := h.repo.UpdateScanResult(ctx, sr); err != nil {
			log.Err(err).Str("scan_id", sr.ID).Msg("failed to update scan result")
		}

		// Save findings
		var findings []model.ScanFinding
		now := time.Now()
		for _, f := range summary.Findings {
			finding := model.ScanFinding{
				ID:          uuid.New().String(),
				ScanID:      sr.ID,
				CheckID:     f.CheckID,
				Category:    f.Category,
				Severity:    f.Severity,
				Title:       f.Title,
				Description: f.Description,
				Remediation: f.Remediation,
				RawOutput:   f.RawOutput,
				Status:      f.Status,
				CreatedAt:   now,
			}
			findings = append(findings, finding)
		}

		if len(findings) > 0 {
			if err := h.repo.CreateScanFindings(ctx, findings); err != nil {
				log.Warn().Err(err).Str("scan_id", sr.ID).Msg("failed to save scan findings")
			}
		}
	}(scanResult, srv, profile)
}

// ─── POST /compliance/{serverID}/scan/lynis ───────────────────────────────

func (h *Handler) TriggerLynisScan(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "serverID")

	srv, err := h.authorizeView(r.Context(), serverID)
	if err != nil {
		common.Error(w, http.StatusNotFound, "server not found")
		return
	}

	now := time.Now()

	scanResult := &model.ScanResult{
		ID:        uuid.New().String(),
		ServerID:  serverID,
		ScanType:  "Lynis",
		Status:    "running",
		StartedAt: &now,
		CreatedAt: now,
	}

	if err := h.repo.CreateScanResult(r.Context(), scanResult); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to create scan result")
		return
	}

	// Return immediately with scan ID — scan runs in background
	common.JSON(w, http.StatusAccepted, map[string]interface{}{
		"scan_id":   scanResult.ID,
		"status":    "running",
		"scan_type": "Lynis",
	})

	go func(sr *model.ScanResult, server *model.Server) {
		ctx := context.Background()

		if err := h.resolveSSHKey(ctx, server); err != nil {
			completedAt := time.Now()
			zero := 0
			sr.Status = "failed"
			sr.ErrorMessage = "SSH key error: " + err.Error()
			sr.CompletedAt = &completedAt
			sr.Score = &zero
			_ = h.repo.UpdateScanResult(ctx, sr)
			log.Err(err).Str("scan_id", sr.ID).Msg("Lynis scan failed: resolve SSH key")
			return
		}

		sshCfg, err := h.sshConfigForServer(ctx, server)
		if err != nil {
			completedAt := time.Now()
			zero := 0
			sr.Status = "failed"
			sr.ErrorMessage = "SSH config error: " + err.Error()
			sr.CompletedAt = &completedAt
			sr.Score = &zero
			_ = h.repo.UpdateScanResult(ctx, sr)
			log.Err(err).Str("scan_id", sr.ID).Msg("Lynis scan failed: SSH config")
			return
		}

		lynisResult, err := h.scanner.RunLynis(ctx, sshCfg)
		if err != nil {
			completedAt := time.Now()
			zero := 0
			sr.Status = "failed"
			sr.ErrorMessage = "Lynis error: " + err.Error()
			sr.CompletedAt = &completedAt
			sr.Score = &zero
			_ = h.repo.UpdateScanResult(ctx, sr)
			log.Err(err).Str("scan_id", sr.ID).Msg("Lynis scan failed")
			return
		}

		completedAt := time.Now()
		score := lynisResult.HardeningScore
		sr.Status = "completed"
		sr.Score = &score
		sr.TotalChecks = lynisResult.Tests
		sr.Warnings = lynisResult.Warnings
		sr.CompletedAt = &completedAt

		if err := h.repo.UpdateScanResult(ctx, sr); err != nil {
			log.Err(err).Str("scan_id", sr.ID).Msg("failed to update Lynis scan result")
		}

		// Create findings from Lynis warnings
		var findings []model.ScanFinding
		now := time.Now()
		for _, w := range lynisResult.WarningsList {
			findings = append(findings, model.ScanFinding{
				ID:          uuid.New().String(),
				ScanID:      sr.ID,
				CheckID:     w.TestID,
				Category:    "lynis",
				Severity:    "high",
				Title:       "Lynis Warning: " + w.TestID,
				Description: w.Description,
				Status:      "fail",
				CreatedAt:   now,
			})
		}
		for _, s := range lynisResult.SuggestionsList {
			findings = append(findings, model.ScanFinding{
				ID:          uuid.New().String(),
				ScanID:      sr.ID,
				CheckID:     s.TestID,
				Category:    "lynis",
				Severity:    "medium",
				Title:       "Lynis Suggestion: " + s.TestID,
				Description: s.Description,
				Status:      "warn",
				CreatedAt:   now,
			})
		}
		if len(findings) > 0 {
			if err := h.repo.CreateScanFindings(ctx, findings); err != nil {
				log.Warn().Err(err).Str("scan_id", sr.ID).Msg("failed to save Lynis findings")
			}
		}
	}(scanResult, srv)
}

// ─── POST /compliance/{serverID}/scan/docker ────────────────────────────

func (h *Handler) TriggerDockerScan(w http.ResponseWriter, r *http.Request) {
	// Delegate to TriggerScan with docker profile
	r2 := r.Clone(r.Context())
	q := r2.URL.Query()
	q.Set("profile", "cis_docker")
	r2.URL.RawQuery = q.Encode()
	h.TriggerScan(w, r2)
}

// ─── POST /compliance/{serverID}/scan/containers ────────────────────────

func (h *Handler) TriggerContainerScan(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "serverID")

	srv, err := h.authorizeView(r.Context(), serverID)
	if err != nil {
		common.Error(w, http.StatusNotFound, "server not found")
		return
	}

	now := time.Now()
	scanResult := &model.ScanResult{
		ID:        uuid.New().String(),
		ServerID:  serverID,
		ScanType:  "Container Security",
		Status:    "running",
		StartedAt: &now,
		CreatedAt: now,
	}

	if err := h.repo.CreateScanResult(r.Context(), scanResult); err != nil {
		log.Err(err).Msg("failed to create container scan result")
		common.Error(w, http.StatusInternalServerError, "failed to create scan result")
		return
	}

	common.JSON(w, http.StatusAccepted, map[string]interface{}{
		"scan_id":   scanResult.ID,
		"status":    "running",
		"scan_type": "Container Security",
	})

	go func(sr *model.ScanResult, server *model.Server) {
		ctx := context.Background()

		if err := h.resolveSSHKey(ctx, server); err != nil {
			completedAt := time.Now()
			zero := 0
			sr.Status = "failed"
			sr.ErrorMessage = "SSH key error: " + err.Error()
			sr.CompletedAt = &completedAt
			sr.Score = &zero
			_ = h.repo.UpdateScanResult(ctx, sr)
			log.Err(err).Str("scan_id", sr.ID).Msg("container scan failed: SSH key")
			return
		}

		sshCfg, err := h.sshConfigForServer(ctx, server)
		if err != nil {
			completedAt := time.Now()
			zero := 0
			sr.Status = "failed"
			sr.ErrorMessage = "SSH config error: " + err.Error()
			sr.CompletedAt = &completedAt
			sr.Score = &zero
			_ = h.repo.UpdateScanResult(ctx, sr)
			log.Err(err).Str("scan_id", sr.ID).Msg("container scan failed: SSH config")
			return
		}

		summary, err := RunContainerScan(ctx, sshCfg)
		if err != nil {
			completedAt := time.Now()
			zero := 0
			sr.Status = "failed"
			sr.ErrorMessage = "Container scan error: " + err.Error()
			sr.CompletedAt = &completedAt
			sr.Score = &zero
			_ = h.repo.UpdateScanResult(ctx, sr)
			log.Err(err).Str("scan_id", sr.ID).Msg("container scan failed")
			return
		}

		completedAt := time.Now()
		score := summary.AverageScore
		sr.Status = "completed"
		sr.Score = &score
		sr.TotalChecks = summary.ScannedContainers
		sr.Passed = summary.TotalContainers - summary.ScannedContainers
		sr.Warnings = summary.TotalMisconfigs
		sr.Criticals = summary.TotalVulnerabilities
		sr.CompletedAt = &completedAt

		if err := h.repo.UpdateScanResult(ctx, sr); err != nil {
			log.Err(err).Str("scan_id", sr.ID).Msg("failed to update container scan result")
		}

		// Save findings per container
		var findings []model.ScanFinding
		now := time.Now()
		for _, c := range summary.Containers {
			for _, f := range c.Findings {
				findings = append(findings, model.ScanFinding{
					ID:          uuid.New().String(),
					ScanID:      sr.ID,
					CheckID:     f.CheckID,
					Category:    c.ContainerName,
					Severity:    f.Severity,
					Title:       c.ContainerName + ": " + f.Title,
					Description: f.Description,
					Remediation: f.Remediation,
					Status:      f.Status,
					CreatedAt:   now,
				})
			}
		}
		if len(findings) > 0 {
			if err := h.repo.CreateScanFindings(ctx, findings); err != nil {
				log.Warn().Err(err).Str("scan_id", sr.ID).Msg("failed to save container findings")
			}
		}
	}(scanResult, srv)
}

// ─── POST /compliance/{serverID}/scan/check/{checkID} ─────────────────────

func (h *Handler) TriggerSingleCheck(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "serverID")
	checkID := chi.URLParam(r, "checkID")

	srv, err := h.authorizeView(r.Context(), serverID)
	if err != nil {
		common.Error(w, http.StatusNotFound, "server not found")
		return
	}

	sshCfg, err := h.sshConfigForServer(r.Context(), srv)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to resolve SSH configuration")
		return
	}

	result, err := h.scanner.RunSingle(r.Context(), sshCfg, checkID)
	if err != nil {
		common.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"check_id": checkID,
		"result":   result,
	})
}

// ─── GET /compliance/history ──────────────────────────────────────────────

// GlobalHistory returns scan history across all servers with optional scan_type filter.
func (h *Handler) GlobalHistory(w http.ResponseWriter, r *http.Request) {
	page := common.ParseQueryInt(r, "page", 1)
	limit := common.ParseQueryInt(r, "limit", 10)
	scanType := r.URL.Query().Get("scan_type")

	result, err := h.repo.ListGlobalScanHistory(r.Context(), scanType, page, limit)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list global scan history")
		return
	}

	common.JSON(w, http.StatusOK, result)
}

// ─── GET /compliance/active ────────────────────────────────────────────

// ActiveScans returns running and recently completed scans for polling.
func (h *Handler) ActiveScans(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		common.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	isAdmin := claims.Role == model.RoleAdmin
	var allowedGroups []string
	if !isAdmin {
		var err error
		allowedGroups, err = h.repo.GetUserServerGroups(r.Context(), claims.UserID)
		if err != nil {
			allowedGroups = []string{}
		}
	}

	result, err := h.repo.ListActiveScans(r.Context(), allowedGroups, isAdmin)
	if err != nil {
		log.Err(err).Msg("failed to list active scans")
		common.Error(w, http.StatusInternalServerError, "failed to list active scans")
		return
	}

	common.JSON(w, http.StatusOK, result)
}

// ─── GET /compliance/{serverID}/history ───────────────────────────────────

func (h *Handler) ScanHistory(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "serverID")

	_, err := h.authorizeView(r.Context(), serverID)
	if err != nil {
		common.Error(w, http.StatusNotFound, "server not found")
		return
	}

	page := common.ParseQueryInt(r, "page", 1)
	limit := common.ParseQueryInt(r, "limit", 10)
	scanType := r.URL.Query().Get("scan_type")

	result, err := h.repo.ListScanResults(r.Context(), serverID, scanType, page, limit)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list scan results")
		return
	}

	common.JSON(w, http.StatusOK, result)
}

// ─── GET /compliance/{serverID}/history/{scanID} ──────────────────────────

func (h *Handler) ScanDetail(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "serverID")
	scanID := chi.URLParam(r, "scanID")

	_, err := h.authorizeView(r.Context(), serverID)
	if err != nil {
		common.Error(w, http.StatusNotFound, "server not found")
		return
	}

	result, err := h.repo.GetScanResultWithFindings(r.Context(), scanID)
	if err != nil {
		common.Error(w, http.StatusNotFound, "scan result not found")
		return
	}

	common.JSON(w, http.StatusOK, result)
}

// ─── GET /compliance/{serverID}/latest/categories ──────────────────────────

func (h *Handler) LatestCategories(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "serverID")

	_, err := h.authorizeView(r.Context(), serverID)
	if err != nil {
		common.Error(w, http.StatusNotFound, "server not found")
		return
	}

	// Support ?scan_type= filter
	scanType := r.URL.Query().Get("scan_type")
	var result *model.ScanResult
	if scanType != "" {
		result, err = h.repo.GetLatestScanResultByType(r.Context(), serverID, scanType)
	} else {
		result, err = h.repo.GetLatestScanResult(r.Context(), serverID)
	}
	if err != nil {
		common.Error(w, http.StatusNotFound, "no scan results found")
		return
	}

	breakdowns, err := h.repo.GetCategoryBreakdowns(r.Context(), result.ID)
	if err != nil {
		breakdowns = []model.CategoryBreakdown{}
	}

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"scan_id":      result.ID,
		"scan_type":    result.ScanType,
		"score":        result.Score,
		"categories":   breakdowns,
	})
}

// ─── GET /compliance/{serverID}/history/categories/{category} ────────────

func (h *Handler) CategoryHistory(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "serverID")
	category := chi.URLParam(r, "category")

	_, err := h.authorizeView(r.Context(), serverID)
	if err != nil {
		common.Error(w, http.StatusNotFound, "server not found")
		return
	}

	limit := common.ParseQueryInt(r, "limit", 10)

	result, err := h.repo.GetCategoryHistory(r.Context(), serverID, category, limit)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to get category history")
		return
	}

	common.JSON(w, http.StatusOK, result)
}

// ─── EOF ──────────────────────────────────────────────────────────────────
