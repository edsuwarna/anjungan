package authactivity

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	zlog "github.com/rs/zerolog/log"
	"github.com/edsuwarna/anjungan/internal/common"
	"github.com/edsuwarna/anjungan/internal/common/model"
)

// Repository interface for auth event storage
type Repository interface {
	CreateAuthEvent(ctx context.Context, e *model.AuthEvent) error
	ListAuthEvents(ctx context.Context, q model.AuthEventQuery) (*model.AuthEventListResponse, error)
	GetAuthEventSummary(ctx context.Context) (*model.AuthEventSummary, error)
	GetAuthEventTrend(ctx context.Context, days int) ([]model.AuthEventTrend, error)
	DetectBruteForce(ctx context.Context) ([]model.BruteForceAlert, error)
	ListMyAuthEvents(ctx context.Context, userID string, limit int) ([]*model.AuthEvent, error)
	GetTopIPs(ctx context.Context, days int) ([]model.TopIPEntry, error)
	GetTopUsers(ctx context.Context, days int) ([]model.TopUserEntry, error)
	GetHourlyHeatmap(ctx context.Context, days int) ([]model.HourlyHeatmapEntry, error)
	CreateSecurityEvent(ctx context.Context, e *model.SecurityEvent) error
	CreateBlockedIP(ctx context.Context, b *model.BlockedIP) error
	RemoveBlockedIP(ctx context.Context, ipAddress string) error
	ListBlockedIPs(ctx context.Context) ([]model.BlockedIP, error)
	PurgeAuthEvents(ctx context.Context, olderThan time.Duration) (int64, error)
	GetUserIDByEmail(ctx context.Context, email string) (string, error)
}

type Handler struct {
	repo Repository
	rdb  *redis.Client
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

// SetRedis sets the Redis client for IP blocking operations.
func (h *Handler) SetRedis(rdb *redis.Client) {
	h.rdb = rdb
}

// GET /summary — summary counts (today)
func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	s, err := h.repo.GetAuthEventSummary(r.Context())
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to get summary")
		return
	}
	common.JSON(w, http.StatusOK, s)
}

// GET /events — paginated event list
func (h *Handler) ListEvents(w http.ResponseWriter, r *http.Request) {
	q := model.AuthEventQuery{
		Page:      common.ParseQueryInt(r, "page", 1),
		Limit:     common.ParseQueryInt(r, "limit", 50),
		EventType: r.URL.Query().Get("event_type"),
		Status:    r.URL.Query().Get("status"),
		UserID:    r.URL.Query().Get("user_id"),
		Email:     r.URL.Query().Get("email"),
		IPAddress: r.URL.Query().Get("ip_address"),
		Search:    r.URL.Query().Get("search"),
		Sort:      r.URL.Query().Get("sort"),
		Order:     r.URL.Query().Get("order"),
	}

	if sd := r.URL.Query().Get("start_date"); sd != "" {
		q.StartDate = &sd
	}
	if ed := r.URL.Query().Get("end_date"); ed != "" {
		q.EndDate = &ed
	}

	resp, err := h.repo.ListAuthEvents(r.Context(), q)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list events")
		return
	}

	common.JSONWithMeta(w, http.StatusOK, resp.Events, &common.Meta{
		Page:       resp.Page,
		PerPage:    resp.Limit,
		Total:      resp.Total,
		TotalPages: resp.TotalPages,
	})
}

// GET /trend — daily trend data
func (h *Handler) Trend(w http.ResponseWriter, r *http.Request) {
	days := common.ParseQueryInt(r, "days", 7)
	if days < 1 || days > 90 {
		days = 7
	}

	trend, err := h.repo.GetAuthEventTrend(r.Context(), days)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to get trend")
		return
	}
	common.JSON(w, http.StatusOK, trend)
}

// GET /brute-force — brute force detection
func (h *Handler) BruteForce(w http.ResponseWriter, r *http.Request) {
	alerts, err := h.repo.DetectBruteForce(r.Context())
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to detect brute force")
		return
	}
	common.JSON(w, http.StatusOK, alerts)
}

// GET /events/export — CSV export
func (h *Handler) ExportCSV(w http.ResponseWriter, r *http.Request) {
	q := model.AuthEventQuery{
		Page:      1,
		Limit:     10000, // export up to 10k rows
		EventType: r.URL.Query().Get("event_type"),
		Status:    r.URL.Query().Get("status"),
		Email:     r.URL.Query().Get("email"),
		Search:    r.URL.Query().Get("search"),
		Sort:      "created_at",
		Order:     "desc",
	}

	resp, err := h.repo.ListAuthEvents(r.Context(), q)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to export events")
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=auth-events.csv")
	w.WriteHeader(http.StatusOK)

	// CSV header
	w.Write([]byte("ID,Email,Event Type,Status,Failure Reason,IP Address,User Agent,Country,ASN,ISP,Created At\n"))

	for _, e := range resp.Events {
		line := csvEscape(e.ID) + "," +
			csvEscape(e.Email) + "," +
			csvEscape(e.EventType) + "," +
			csvEscape(e.Status) + "," +
			csvEscape(e.FailureReason) + "," +
			csvEscape(e.IPAddress) + "," +
			csvEscape(e.UserAgent) + "," +
			csvEscape(e.Country) + "," +
			csvEscape(e.ASN) + "," +
			csvEscape(e.ISP) + "," +
			e.CreatedAt.Format(time.RFC3339) + "\n"
		w.Write([]byte(line))
	}
}

func csvEscape(s string) string {
	if s == "" {
		return ""
	}
	needsQuotes := false
	for _, c := range s {
		if c == ',' || c == '"' || c == '\n' || c == '\r' {
			needsQuotes = true
			break
		}
	}
	if !needsQuotes {
		return s
	}
	return `"` + escapeQuotes(s) + `"`
}

func escapeQuotes(s string) string {
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '"' {
			result = append(result, '"', '"')
		} else {
			result = append(result, s[i])
		}
	}
	return string(result)
}

// GET /events/mine — current user's login history (last 20)
func (h *Handler) MyEvents(w http.ResponseWriter, r *http.Request) {
	claims := common.GetUserClaims(r.Context())
	if claims == nil {
		common.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	events, err := h.repo.ListMyAuthEvents(r.Context(), claims.UserID, 20)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to get login history")
		return
	}
	if events == nil {
		events = []*model.AuthEvent{}
	}
	common.JSON(w, http.StatusOK, events)
}

// GET /top-ips — IPs with most failures
func (h *Handler) TopIPs(w http.ResponseWriter, r *http.Request) {
	days := common.ParseQueryInt(r, "days", 7)
	entries, err := h.repo.GetTopIPs(r.Context(), days)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to get top IPs")
		return
	}
	common.JSON(w, http.StatusOK, entries)
}

// GET /top-users — users with most failures
func (h *Handler) TopUsers(w http.ResponseWriter, r *http.Request) {
	days := common.ParseQueryInt(r, "days", 7)
	entries, err := h.repo.GetTopUsers(r.Context(), days)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to get top users")
		return
	}
	common.JSON(w, http.StatusOK, entries)
}

// GET /heatmap — hourly auth event distribution
func (h *Handler) HourlyHeatmap(w http.ResponseWriter, r *http.Request) {
	days := common.ParseQueryInt(r, "days", 7)
	entries, err := h.repo.GetHourlyHeatmap(r.Context(), days)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to get heatmap")
		return
	}
	common.JSON(w, http.StatusOK, entries)
}

// POST /block-ip — block an IP address
func (h *Handler) BlockIP(w http.ResponseWriter, r *http.Request) {
	if h.rdb == nil {
		common.Error(w, http.StatusServiceUnavailable, "Redis not configured")
		return
	}
	var req struct {
		IPAddress string `json:"ip_address"`
		Reason    string `json:"reason,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.IPAddress == "" {
		common.Error(w, http.StatusBadRequest, "ip_address is required")
		return
	}

	claims := common.GetUserClaims(r.Context())
	createdBy := ""
	if claims != nil {
		createdBy = claims.Email
	}

	now := time.Now()
	data, _ := json.Marshal(map[string]interface{}{
		"ip_address": req.IPAddress,
		"created_by": createdBy,
		"reason":     req.Reason,
		"created_at": now,
	})

	key := "blocked_ip:" + req.IPAddress
	if err := h.rdb.Set(r.Context(), key, string(data), 0).Err(); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to block IP")
		return
	}

	// Also add to blocked_ips set for listing
	h.rdb.SAdd(r.Context(), "blocked_ips", req.IPAddress)

	// Persist to DB for durability
	dbIP := &model.BlockedIP{
		ID:        uuid.New().String(),
		IPAddress: req.IPAddress,
		Reason:    req.Reason,
		CreatedBy: createdBy,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := h.repo.CreateBlockedIP(r.Context(), dbIP); err != nil {
		zlog.Warn().Err(err).Str("ip", req.IPAddress).Msg("failed to persist blocked IP to DB")
	}

	common.JSON(w, http.StatusOK, map[string]string{"status": "blocked", "ip": req.IPAddress})
}

// POST /unblock-ip — unblock an IP address
func (h *Handler) UnblockIP(w http.ResponseWriter, r *http.Request) {
	if h.rdb == nil {
		common.Error(w, http.StatusServiceUnavailable, "Redis not configured")
		return
	}
	var req struct {
		IPAddress string `json:"ip_address"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	key := "blocked_ip:" + req.IPAddress
	h.rdb.Del(r.Context(), key)
	h.rdb.SRem(r.Context(), "blocked_ips", req.IPAddress)

	// Remove from DB
	if err := h.repo.RemoveBlockedIP(r.Context(), req.IPAddress); err != nil {
		zlog.Warn().Err(err).Str("ip", req.IPAddress).Msg("failed to remove blocked IP from DB")
	}

	common.JSON(w, http.StatusOK, map[string]string{"status": "unblocked", "ip": req.IPAddress})
}

// GET /blocked-ips — list all blocked IPs (Redis + DB merge)
func (h *Handler) ListBlockedIPs(w http.ResponseWriter, r *http.Request) {
	byIP := make(map[string]model.BlockedIP)

	// Get from DB (authoritative for metadata)
	dbIPs, err := h.repo.ListBlockedIPs(r.Context())
	if err != nil {
		zlog.Warn().Err(err).Msg("failed to list blocked IPs from DB, falling back to Redis")
	} else {
		for _, b := range dbIPs {
			byIP[b.IPAddress] = b
		}
	}

	// Get from Redis (real-time listing)
	if h.rdb != nil {
		ips, err := h.rdb.SMembers(r.Context(), "blocked_ips").Result()
		if err == nil {
			for _, ip := range ips {
				if _, exists := byIP[ip]; !exists {
					key := "blocked_ip:" + ip
					data, err := h.rdb.Get(r.Context(), key).Result()
					if err == nil {
						var b model.BlockedIP
						if json.Unmarshal([]byte(data), &b) == nil {
							byIP[ip] = b
						} else {
							byIP[ip] = model.BlockedIP{IPAddress: ip}
						}
					} else {
						byIP[ip] = model.BlockedIP{IPAddress: ip}
					}
				}
			}
		}
	}

	var result []model.BlockedIP
	for _, b := range byIP {
		result = append(result, b)
	}
	if result == nil {
		result = []model.BlockedIP{}
	}

	common.JSON(w, http.StatusOK, result)
}

// GET /lockouts — list currently locked-out accounts with remaining time
func (h *Handler) ListLockouts(w http.ResponseWriter, r *http.Request) {
	if h.rdb == nil {
		common.JSON(w, http.StatusOK, []map[string]interface{}{})
		return
	}

	var lockouts []map[string]interface{}
	iter := h.rdb.Scan(r.Context(), 0, "lockout:*", 100).Iterator()
	for iter.Next(r.Context()) {
		key := iter.Val()
		email := strings.TrimPrefix(key, "lockout:")

		ttl, err := h.rdb.TTL(r.Context(), key).Result()
		if err != nil || ttl <= 0 {
			continue
		}

		// Look up user_id for the unlock action
		userID, _ := h.repo.GetUserIDByEmail(r.Context(), email)

		lockouts = append(lockouts, map[string]interface{}{
			"email":         email,
			"user_id":       userID,
			"remaining_sec": int(ttl.Seconds()),
			"locked_until":  time.Now().Add(ttl).Format(time.RFC3339),
		})
	}

	if lockouts == nil {
		lockouts = []map[string]interface{}{}
	}

	common.JSON(w, http.StatusOK, lockouts)
}

// Routes returns the chi router for auth activity endpoints
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/summary", h.Summary)
	r.Get("/events", h.ListEvents)
	r.Get("/events/export", h.ExportCSV)
	r.Get("/events/mine", h.MyEvents)
	r.Get("/trend", h.Trend)
	r.Get("/brute-force", h.BruteForce)
	r.Get("/top-ips", h.TopIPs)
	r.Get("/top-users", h.TopUsers)
	r.Get("/heatmap", h.HourlyHeatmap)
	r.Post("/block-ip", h.BlockIP)
	r.Post("/unblock-ip", h.UnblockIP)
	r.Get("/blocked-ips", h.ListBlockedIPs)
	r.Get("/lockouts", h.ListLockouts)
	return r
}

// ─── Recorder — convenience for recording auth events ───────────────────────

func RecordEvent(repo Repository, userID, email, eventType, status, failureReason, ipAddress, userAgent string) {
	event := &model.AuthEvent{
		ID:            uuid.New().String(),
		UserID:        userID,
		Email:         email,
		EventType:     eventType,
		Status:        status,
		FailureReason: failureReason,
		IPAddress:     ipAddress,
		UserAgent:     userAgent,
		CreatedAt:     time.Now(),
	}

	// Best-effort async with geolocation lookup
	go func() {
		// Populate country/ASN/ISP via geolocation (best-effort)
		country, asn, isp := lookupGeo(ipAddress)
		event.Country = country
		event.ASN = parseASN(asn)
		event.ISP = isp
		_ = repo.CreateAuthEvent(context.Background(), event)
	}()
}

func RecordJSON(repo Repository, userID, email, eventType, status, failureReason, ipAddress, userAgent string) {
	// Same as RecordEvent but returns nothing — used when you already have JSON context
	RecordEvent(repo, userID, email, eventType, status, failureReason, ipAddress, userAgent)
}

// CleanIP removes port from an IP address
func CleanIP(ip string) string {
	if len(ip) > 0 && ip[0] == '[' {
		// IPv6
		if idx := lastIndexByte(ip, ']'); idx > 0 {
			if idx+2 < len(ip) && ip[idx+1] == ':' {
				return ip[:idx+1]
			}
		}
		return ip
	}
	for i := len(ip) - 1; i >= 0; i-- {
		if ip[i] == ':' {
			return ip[:i]
		}
	}
	return ip
}

func lastIndexByte(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}
