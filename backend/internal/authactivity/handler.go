package authactivity

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
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
}

type Handler struct {
	repo Repository
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
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

// Routes returns the chi router for auth activity endpoints
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/summary", h.Summary)
	r.Get("/events", h.ListEvents)
	r.Get("/events/export", h.ExportCSV)
	r.Get("/trend", h.Trend)
	r.Get("/brute-force", h.BruteForce)
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

	// Best-effort async
	go func() {
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
