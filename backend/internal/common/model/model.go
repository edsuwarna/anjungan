package model

import (
	"encoding/json"
	"strings"
	"time"
)

type User struct {
	ID                  string     `json:"id"`
	Email               string     `json:"email"`
	Name                string     `json:"name"`
	PasswordHash        string     `json:"-"`
	TOTPSecret          string     `json:"-"`
	TOTPEnabled         bool       `json:"totp_enabled"`
	Role                string     `json:"role"`
	LockedUntil         *time.Time `json:"locked_until,omitempty"`
	FailedLoginAttempts int        `json:"-"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type Server struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Host           string    `json:"host"`
	Port           int       `json:"port"`
	SSHUser        string    `json:"ssh_user"`
	SSHAuthType    string    `json:"ssh_auth_type"`
	SSHKeyID       string    `json:"ssh_key_id,omitempty"`
	SSHKey         string    `json:"-"`
	SSHPassword    string    `json:"-"`
	Status         string    `json:"status"`
	ContainerCount int       `json:"container_count"`
	Tags           []string  `json:"tags"`
	Labels         string    `json:"labels"`
	ServerGroup    string    `json:"server_group"`
	Region         string    `json:"region"`
	ServerType     string    `json:"server_type"`
	Description    string    `json:"description"`
	OSInfo         string    `json:"os_info"`
	CPUInfo        string    `json:"cpu_info"`
	LastSeenAt     *time.Time `json:"last_seen_at"`
	Monitoring     bool      `json:"monitoring"`
	ConnectionType string    `json:"connection_type"`
	IsSelf         bool      `json:"is_self"`
	SelfHostname   string    `json:"self_hostname"`
	CreatedBy      string    `json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ServerResponse is the public-safe version (no credentials exposed)
type ServerResponse struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Host           string     `json:"host"`
	Port           int        `json:"port"`
	SSHUser        string     `json:"ssh_user"`
	SSHAuthType    string     `json:"ssh_auth_type"`
	Status         string     `json:"status"`
	ContainerCount int        `json:"container_count"`
	Tags           []string   `json:"tags"`
	Labels         string     `json:"labels"`
	ServerGroup    string     `json:"server_group"`
	Region         string     `json:"region"`
	ServerType     string     `json:"server_type"`
	Description    string     `json:"description"`
	OSInfo         string     `json:"os_info"`
	CPUInfo        string     `json:"cpu_info"`
	LastSeenAt     *time.Time `json:"last_seen_at"`
	Monitoring     bool       `json:"monitoring"`
	ConnectionType string     `json:"connection_type"`
	IsSelf         bool       `json:"is_self"`
	SelfHostname   string     `json:"self_hostname"`
	CreatedBy      string     `json:"created_by"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	// Compliance fields — populated on list queries, may be nil/zero if unscanned
	Score     *int       `json:"score"`
	Criticals int        `json:"criticals"`
	Warnings  int        `json:"warnings"`
	Passed    int        `json:"passed"`
	LastScan  *time.Time `json:"last_scan"`
}

func (s *Server) ToResponse() ServerResponse {
	return ServerResponse{
		ID:             s.ID,
		Name:           s.Name,
		Host:           s.Host,
		Port:           s.Port,
		SSHUser:        s.SSHUser,
		SSHAuthType:    s.SSHAuthType,
		Status:         s.Status,
		ContainerCount: s.ContainerCount,
		Tags:           s.Tags,
		Labels:         s.Labels,
		ServerGroup:    s.ServerGroup,
		Region:         s.Region,
		ServerType:     s.ServerType,
		Description:    s.Description,
		OSInfo:         s.OSInfo,
		CPUInfo:        s.CPUInfo,
		LastSeenAt:     s.LastSeenAt,
		Monitoring:     s.Monitoring,
		ConnectionType: s.ConnectionType,
		IsSelf:         s.IsSelf,
		SelfHostname:   s.SelfHostname,
		CreatedBy:      s.CreatedBy,
		CreatedAt:      s.CreatedAt,
		UpdatedAt:      s.UpdatedAt,
	}
}

// ServerRequest is the input for create/update operations
type CreateServerRequest struct {
	Name           string   `json:"name"`
	Host           string   `json:"host"`
	Port           int      `json:"port"`
	SSHUser        string   `json:"ssh_user"`
	SSHAuthType    string   `json:"ssh_auth_type"`
	SSHKeyID       string   `json:"ssh_key_id,omitempty"`
	SSHKey         string   `json:"ssh_key,omitempty"`
	SSHPassword    string   `json:"ssh_password,omitempty"`
	Tags           []string `json:"tags,omitempty"`
	ServerGroup    string   `json:"server_group,omitempty"`
	Region         string   `json:"region,omitempty"`
	ServerType     string   `json:"server_type,omitempty"`
	Description    string   `json:"description,omitempty"`
	ConnectionType string   `json:"connection_type,omitempty"`
}

type UpdateServerRequest struct {
	Name           *string   `json:"name,omitempty"`
	Host           *string   `json:"host,omitempty"`
	Port           *int      `json:"port,omitempty"`
	SSHUser        *string   `json:"ssh_user,omitempty"`
	SSHAuthType    *string   `json:"ssh_auth_type,omitempty"`
	SSHKeyID       *string   `json:"ssh_key_id,omitempty"`
	SSHKey         *string   `json:"ssh_key,omitempty"`
	SSHPassword    *string   `json:"ssh_password,omitempty"`
	Tags           *[]string `json:"tags,omitempty"`
	ServerGroup    *string   `json:"server_group,omitempty"`
	Region         *string   `json:"region,omitempty"`
	ServerType     *string   `json:"server_type,omitempty"`
	Description    *string   `json:"description,omitempty"`
	ConnectionType *string   `json:"connection_type,omitempty"`
}

type TestConnectionRequest struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	SSHUser     string `json:"ssh_user"`
	SSHAuthType string `json:"ssh_auth_type"`
	SSHKeyID    string `json:"ssh_key_id,omitempty"`
	SSHKey      string `json:"ssh_key,omitempty"`
	SSHPassword string `json:"ssh_password,omitempty"`
}

// Pagination / List query
type ServerListQuery struct {
	Page       int    `json:"page"`
	Limit      int    `json:"limit"`
	Sort       string `json:"sort"`  // name, host, status, created_at, updated_at
	Order      string `json:"order"` // asc, desc
	Status     string `json:"status"`
	Search     string `json:"search"`
	ServerGroup string `json:"server_group"`
	Region     string `json:"region"`
	ServerType string `json:"server_type"`
	Tags       string `json:"tags"` // comma-separated
}

type ServerListResponse struct {
	Servers    []ServerResponse `json:"servers"`
	Total      int              `json:"total"`
	Page       int              `json:"page"`
	Limit      int              `json:"limit"`
	TotalPages int              `json:"total_pages"`
}

// ServerMetricsPoint is a single metrics snapshot
type ServerMetricsPoint struct {
	ID            int64     `json:"id"`
	ServerID      string    `json:"server_id"`
	CPULoad1      float64   `json:"cpu_load_1"`
	CPULoad5      float64   `json:"cpu_load_5"`
	CPULoad15     float64   `json:"cpu_load_15"`
	MemUsedBytes  int64     `json:"mem_used_bytes"`
	MemTotalBytes int64     `json:"mem_total_bytes"`
	DiskUsedBytes int64     `json:"disk_used_bytes"`
	DiskTotalBytes int64    `json:"disk_total_bytes"`
	DiskUsedPct   float64   `json:"disk_used_pct"`
	NetRXBytes    int64     `json:"net_rx_bytes"`
	NetTXBytes    int64     `json:"net_tx_bytes"`
	CollectedAt   time.Time `json:"collected_at"`
}

// SSHKey represents a saved SSH private key for reuse across servers
type SSHKey struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	KeyType     string    `json:"key_type"`
	PrivateKey  string    `json:"-"` // never exposed via API
	PublicKey   string    `json:"public_key,omitempty"`
	Fingerprint string    `json:"fingerprint,omitempty"`
	CreatedBy   string    `json:"created_by,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SSHKeyResponse is a safe response without private key
type SSHKeyResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	KeyType     string    `json:"key_type"`
	PublicKey   string    `json:"public_key,omitempty"`
	Fingerprint string    `json:"fingerprint,omitempty"`
	CreatedBy   string    `json:"created_by,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ServerCount int       `json:"server_count"`
}

func (k *SSHKey) ToResponse() SSHKeyResponse {
	return SSHKeyResponse{
		ID:          k.ID,
		Name:        k.Name,
		KeyType:     k.KeyType,
		PublicKey:   k.PublicKey,
		Fingerprint: k.Fingerprint,
		CreatedBy:   k.CreatedBy,
		CreatedAt:   k.CreatedAt,
		UpdatedAt:   k.UpdatedAt,
	}
}

type RegistryUser struct {
	ID             string    `json:"id"`
	Username       string    `json:"username"`
	PasswordHash   string    `json:"-"`
	Role           string    `json:"role"`
	AnjunganUserID string    `json:"anjungan_user_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// RegistryUserResponse is the public-safe version (no password hash exposed)
type RegistryUserResponse struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	Access    string    `json:"access"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (u *RegistryUser) ToResponse() RegistryUserResponse {
	access := ""
	switch u.Role {
	case "admin":
		access = "Read, write, delete — full access to all repositories"
	case "deploy":
		access = "Read and push — CI/CD pipelines and developer workstations"
	case "readonly":
		access = "Read-only — pull only"
	}
	return RegistryUserResponse{
		ID:        u.ID,
		Username:  u.Username,
		Role:      u.Role,
		Access:    access,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

type Alert struct {
	ID           string    `json:"id"`
	ServerID     string    `json:"server_id"`
	Type         string    `json:"type"`
	Severity     string    `json:"severity"`
	Message      string    `json:"message"`
	Value        string    `json:"value"`
	Threshold    string    `json:"threshold"`
	Acknowledged bool      `json:"acknowledged"`
	CreatedAt    time.Time `json:"created_at"`
}

// ─── Audit Log ──────────────────────────────────────────────────────────────

type AuditLogEntry struct {
	ID          string          `json:"id"`
	Action      string          `json:"action"`
	EntityType  string          `json:"entity_type"`
	EntityID    string          `json:"entity_id,omitempty"`
	Description string          `json:"description"`
	UserID      string          `json:"user_id,omitempty"`
	UserEmail   string          `json:"user_email,omitempty"`
	IPAddress   string          `json:"ip_address,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

type AuditLogQuery struct {
	Page       int     `json:"page"`
	Limit      int     `json:"limit"`
	Action     string  `json:"action"`
	EntityType string  `json:"entity_type"`
	UserID     string  `json:"user_id"`
	Search     string  `json:"search"`
	Sort       string  `json:"sort"`
	Order      string  `json:"order"`
	StartDate  *string `json:"start_date,omitempty"`
	EndDate    *string `json:"end_date,omitempty"`
}

type AuditLogListResponse struct {
	Entries    []*AuditLogEntry `json:"entries"`
	Total      int              `json:"total"`
	Page       int              `json:"page"`
	Limit      int              `json:"limit"`
	TotalPages int              `json:"total_pages"`
}

// ─── Compliance / Security Scan ───────────────────────────────────────────

type ScanResult struct {
	ID          string         `json:"id"`
	ServerID    string         `json:"server_id"`
	ScanType    string         `json:"scan_type"`
	Status      string         `json:"status"`       // pending, running, completed, failed
	Score       *int           `json:"score"`
	TotalChecks int            `json:"total_checks"`
	Passed      int            `json:"passed"`
	Warnings    int            `json:"warnings"`
	Criticals   int            `json:"criticals"`
	ErrorMessage string         `json:"error_message"`
	StartedAt    *time.Time     `json:"started_at"`
	CompletedAt  *time.Time     `json:"completed_at"`
	CreatedAt    time.Time      `json:"created_at"`
	Findings     []ScanFinding  `json:"findings,omitempty"`
}

type ScanFinding struct {
	ID          string    `json:"id"`
	ScanID      string    `json:"scan_id"`
	CheckID     string    `json:"check_id"`
	Category    string    `json:"category"`
	Severity    string    `json:"severity"` // critical, high, medium, low, info
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Remediation string    `json:"remediation"`
	RawOutput   string    `json:"-"`
	Status      string    `json:"status"` // pass, fail, warn, info
	CreatedAt   time.Time `json:"created_at"`
}

type ComplianceSummary struct {
	TotalServers   int                       `json:"total_servers"`
	ScannedServers int                       `json:"scanned_servers"`
	AverageScore   *int                      `json:"average_score"`
	ByStatus       map[string]int            `json:"by_status"`
	TopFindings    []ComplianceTopFinding    `json:"top_findings"`
	Servers        []ComplianceServerSummary `json:"servers"`
}

type ComplianceTopFinding struct {
	CheckID         string `json:"check_id"`
	Title           string `json:"title"`
	Severity        string `json:"severity"`
	ServersAffected int    `json:"servers_affected"`
}

type ComplianceServerSummary struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Host      string     `json:"host"`
	Score     *int       `json:"score"`
	Status    string     `json:"status"`
	Criticals int        `json:"criticals"`
	Warnings  int        `json:"warnings"`
	Passed    int        `json:"passed"`
	LastScan  *time.Time `json:"last_scan"`
}

// ScanResultsListResponse wraps paginated scan results
type ScanResultsListResponse struct {
	Results    []*ScanResult `json:"results"`
	Total      int           `json:"total"`
	Page       int           `json:"page"`
	Limit      int           `json:"limit"`
	TotalPages int           `json:"total_pages"`
}

// GlobalScanHistoryItem is a scan result with server info for global history view
type GlobalScanHistoryItem struct {
	ID            string     `json:"id"`
	ServerID      string     `json:"server_id"`
	ServerName    string     `json:"server_name"`
	ServerHost    string     `json:"server_host"`
	ScanType      string     `json:"scan_type"`
	Status        string     `json:"status"`
	Score         *int       `json:"score"`
	TotalChecks   int        `json:"total_checks"`
	Passed        int        `json:"passed"`
	Warnings      int        `json:"warnings"`
	Criticals     int        `json:"criticals"`
	ErrorMessage  string     `json:"error_message"`
	StartedAt     *time.Time `json:"started_at"`
	CompletedAt   *time.Time `json:"completed_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

// GlobalHistoryResponse wraps paginated global scan history
type GlobalHistoryResponse struct {
	Results    []GlobalScanHistoryItem `json:"results"`
	Total      int                     `json:"total"`
	Page       int                     `json:"page"`
	Limit      int                     `json:"limit"`
	TotalPages int                     `json:"total_pages"`
}

// ActiveScanItem is a scan result with server info for active scan polling
type ActiveScanItem struct {
	ID          string     `json:"id"`
	ServerID    string     `json:"server_id"`
	ServerName  string     `json:"server_name"`
	ServerHost  string     `json:"server_host"`
	ScanType    string     `json:"scan_type"`
	Status      string     `json:"status"`
	Score       *int       `json:"score"`
	TotalChecks int        `json:"total_checks"`
	Passed      int        `json:"passed"`
	Warnings    int        `json:"warnings"`
	Criticals   int        `json:"criticals"`
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// ActiveScansResponse wraps active scan list
type ActiveScansResponse struct {
	Running []ActiveScanItem `json:"running"`
	Recent  []ActiveScanItem `json:"recent"` // completed/failed within last 5 minutes
}

// CategoryBreakdown is the per-category summary within a scan.
type CategoryBreakdown struct {
	Category  string `json:"category"`
	Total     int    `json:"total"`
	Passed    int    `json:"passed"`
	Warnings  int    `json:"warnings"`
	Criticals int    `json:"criticals"`
}

// CategoryHistoryItem is a single data point in per-category history.
type CategoryHistoryItem struct {
	ScanID     string     `json:"scan_id"`
	ServerID   string     `json:"server_id"`
	ServerName string     `json:"server_name"`
	ScanType   string     `json:"scan_type"`
	Total      int        `json:"total"`
	Passed     int        `json:"passed"`
	Warnings   int        `json:"warnings"`
	Criticals  int        `json:"criticals"`
	Score      *int       `json:"score"`
	CreatedAt  time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

// CategoryHistoryResponse wraps paginated category history.
type CategoryHistoryResponse struct {
	Results    []CategoryHistoryItem `json:"results"`
	Total      int                   `json:"total"`
	Category   string                `json:"category"`
}

// ContainerScanHistoryItem is a single scan entry in per-container scan history.
type ContainerScanHistoryItem struct {
	ScanID     string     `json:"scan_id"`
	Score      int        `json:"score"`
	Total      int        `json:"total"`
	Failed     int        `json:"failed"`
	Passed     int        `json:"passed"`
	Criticals  int        `json:"criticals"`
	High       int        `json:"high"`
	Medium     int        `json:"medium"`
	Low        int        `json:"low"`
	CompletedAt *time.Time `json:"completed_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

// ContainerSecurityResponse wraps container info + server info + security data for the security report page.
type ContainerSecurityResponse struct {
	Container ContainerSecurityContainer `json:"container"`
	Server    ContainerSecurityServer    `json:"server"`
	Security  *SecuritySummary           `json:"security,omitempty"`
}

type ContainerSecurityContainer struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Image  string `json:"image"`
	Status string `json:"status"`
	State  string `json:"state"`
	Ports  string `json:"ports"`
	Created string `json:"created"`
}

type ContainerSecurityServer struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Host string `json:"host"`
	Port int    `json:"port"`
}

type SecuritySummary struct {
	Score     int           `json:"score"`
	Badges    []string      `json:"badges"`
	Findings  []ScanFinding `json:"findings"`
	ScannedAt *time.Time    `json:"scanned_at"`
}

const (
    RoleAdmin     = "admin"
    RoleDeveloper = "developer"
    RoleViewer    = "viewer"
)

// ─── Settings ────────────────────────────────────────────────────────────────

type Settings struct {
    Key         string    `json:"key"`
    Value       string    `json:"value"`
    Description string    `json:"description"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

type ComplianceThresholds struct {
    Compliant int `json:"compliant"` // Minimum score for green/compliant band
    Warning   int `json:"warning"`   // Minimum score for yellow/warning band (below this = red/critical)
}

// DefaultComplianceThresholds returns the hardcoded defaults used when no DB settings exist.
func DefaultComplianceThresholds() ComplianceThresholds {
    return ComplianceThresholds{Compliant: 90, Warning: 70}
}

// ─── Registry Webhooks ────────────────────────────────────────────────────────

type RegistryWebhook struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Platform  string    `json:"platform"`
	Events    string    `json:"events"` // JSON array
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RegistryWebhookEvent struct {
	ID          string    `json:"id"`
	WebhookID   string    `json:"webhook_id,omitempty"`
	EventType   string    `json:"event_type"`
	Repo        string    `json:"repo"`
	Tag         string    `json:"tag"`
	Digest      string    `json:"digest"`
	Actor       string    `json:"actor"`
	Description string    `json:"description"`
	Payload     *string   `json:"payload,omitempty"`
	Status      string    `json:"status"`
	StatusCode  int       `json:"status_code"`
	Response    string    `json:"response"`
	CreatedAt   time.Time `json:"created_at"`
	DeliveredAt *time.Time `json:"delivered_at,omitempty"`
}

func (w *RegistryWebhook) EventList() []string {
	var events []string
	if err := json.Unmarshal([]byte(w.Events), &events); err != nil {
		return []string{"push", "pull", "delete"}
	}
	return events
}

func (w *RegistryWebhook) PlatformIcon() string {
	switch w.Platform {
	case "telegram":
		return "solar:telegram-bold"
	case "discord":
		return "solar:discord-bold"
	case "slack":
		return "solar:slack-bold"
	default:
		return "solar:link-bold"
	}
}

type RegistryWebhookRequest struct {
	Name     string   `json:"name"`
	URL      string   `json:"url"`
	Platform string   `json:"platform"`
	Events   []string `json:"events"`
	Enabled  *bool    `json:"enabled,omitempty"`
}

func (r *RegistryWebhookRequest) Validate() string {
	if r.URL == "" {
		return "URL is required"
	}
	if r.Platform == "" {
		r.Platform = "generic"
	}
	validPlatforms := map[string]bool{"telegram": true, "discord": true, "slack": true, "generic": true}
	if !validPlatforms[r.Platform] {
		return "platform must be telegram, discord, slack, or generic"
	}
	if len(r.Events) == 0 {
		r.Events = []string{"push", "pull", "delete"}
	}
	for _, e := range r.Events {
		if e != "push" && e != "pull" && e != "delete" {
			return "event must be push, pull, or delete"
		}
	}
	return ""
}

// ─── Registry Tag Protection ──────────────────────────────────────────────────

type RegistryTagProtection struct {
	ID        string    `json:"id"`
	Repo      string    `json:"repo"`
	Tag       string    `json:"tag"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

type TagProtectionRequest struct {
	Repo string `json:"repo"`
	Tag  string `json:"tag"`
}

func (r *TagProtectionRequest) Validate() string {
	if r.Repo == "" {
		return "repo is required"
	}
	if r.Tag == "" {
		return "tag is required"
	}
	return ""
}

// ─── SSL Monitor ─────────────────────────────────────────────────────────────

type SSLMonitor struct {
	ID        string `json:"id"`
	Domain    string `json:"domain"`
	Port      int    `json:"port"`
	CreatedBy string `json:"created_by"`

	// Server association for auto-discovery
	ServerID      string `json:"server_id,omitempty"`
	SourceProvider string `json:"source_provider,omitempty"` // manual, traefik, nginx, caddy, letsencrypt, discovered

	// Core
	DisplayName   string `json:"display_name"`
	CheckInterval string `json:"check_interval"`
	NotifyBefore  string `json:"notify_before"`
	WebhookIDs    []string `json:"webhook_ids"`
	Enabled       bool   `json:"enabled"`

	// Last check results (TLS engine output)
	LastStatus    string     `json:"last_status"`    // pending, valid, expiring_soon, expired, error
	LastCheckAt   *time.Time `json:"last_check_at"`
	LastError     string     `json:"last_error,omitempty"`

	// Certificate info
	Issuer         string     `json:"issuer"`
	Subject        string     `json:"subject"`
	CertExpiresAt  *time.Time `json:"cert_expires_at"`
	DaysRemaining  int        `json:"days_remaining"`

	// Chain validation
	ChainValid *bool  `json:"chain_valid,omitempty"`
	ChainError string `json:"chain_error,omitempty"`

	// Cipher grade
	CipherGrade string `json:"cipher_grade"`
	CipherError string `json:"cipher_error,omitempty"`

	// OCSP revocation
	OCSPStatus string `json:"ocsp_status"`
	OCSPError  string `json:"ocsp_error,omitempty"`

	// SAN coverage
	SANNames    []string `json:"san_names,omitempty"`
	SANMismatch bool     `json:"san_mismatch"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SSLMonitorResponse is the public API shape (safe, no internal fields)
type SSLMonitorResponse struct {
	ID        string `json:"id"`
	Domain    string `json:"domain"`
	Port      int    `json:"port"`
	CreatedBy string `json:"created_by"`

	ServerID       string `json:"server_id,omitempty"`
	SourceProvider string `json:"source_provider,omitempty"`

	DisplayName   string   `json:"display_name"`
	CheckInterval string   `json:"check_interval"`
	NotifyBefore  string   `json:"notify_before"`
	WebhookIDs    []string `json:"webhook_ids"`
	Enabled       bool     `json:"enabled"`

	LastStatus    string     `json:"last_status"`
	LastCheckAt   *time.Time `json:"last_check_at"`
	LastError     string     `json:"last_error,omitempty"`

	Issuer         string     `json:"issuer"`
	Subject        string     `json:"subject"`
	CertExpiresAt  *time.Time `json:"cert_expires_at"`
	DaysRemaining  int        `json:"days_remaining"`

	ChainValid *bool  `json:"chain_valid,omitempty"`
	ChainError string `json:"chain_error,omitempty"`

	CipherGrade string `json:"cipher_grade"`
	CipherError string `json:"cipher_error,omitempty"`

	OCSPStatus string `json:"ocsp_status"`
	OCSPError  string `json:"ocsp_error,omitempty"`

	SANNames    []string `json:"san_names,omitempty"`
	SANMismatch bool     `json:"san_mismatch"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (m *SSLMonitor) ToResponse() SSLMonitorResponse {
	return SSLMonitorResponse{
		ID:           m.ID,
		Domain:       m.Domain,
		Port:         m.Port,
		CreatedBy:    m.CreatedBy,
		ServerID:      m.ServerID,
		SourceProvider: m.SourceProvider,
		DisplayName:  m.DisplayName,
		CheckInterval: m.CheckInterval,
		NotifyBefore: m.NotifyBefore,
		WebhookIDs:   m.WebhookIDs,
		Enabled:      m.Enabled,
		LastStatus:   m.LastStatus,
		LastCheckAt:  m.LastCheckAt,
		LastError:    m.LastError,
		Issuer:        m.Issuer,
		Subject:       m.Subject,
		CertExpiresAt: m.CertExpiresAt,
		DaysRemaining: m.DaysRemaining,
		ChainValid:    m.ChainValid,
		ChainError:    m.ChainError,
		CipherGrade:   m.CipherGrade,
		CipherError:   m.CipherError,
		OCSPStatus:    m.OCSPStatus,
		OCSPError:     m.OCSPError,
		SANNames:      m.SANNames,
		SANMismatch:   m.SANMismatch,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}
}

type CreateSSLMonitorRequest struct {
	Domain        string   `json:"domain"`
	Port          int      `json:"port"`
	DisplayName   string   `json:"display_name,omitempty"`
	CheckInterval string   `json:"check_interval,omitempty"`
	NotifyBefore  string   `json:"notify_before,omitempty"`
	WebhookIDs    []string `json:"webhook_ids,omitempty"`
	Enabled       *bool    `json:"enabled,omitempty"`
}

type UpdateSSLMonitorRequest struct {
	DisplayName   *string   `json:"display_name,omitempty"`
	Port          *int      `json:"port,omitempty"`
	CheckInterval *string   `json:"check_interval,omitempty"`
	NotifyBefore  *string   `json:"notify_before,omitempty"`
	WebhookIDs    *[]string `json:"webhook_ids,omitempty"`
	Enabled       *bool     `json:"enabled,omitempty"`
}

// ─── SSL Discovery ─────────────────────────────────────────────────────────

type SSLDiscoveryRequest struct {
	ServerID string `json:"server_id"`
	Provider string `json:"provider"` // "auto", "traefik", "nginx", "caddy", "letsencrypt", "filesystem"
}

type SSLDiscoveryResult struct {
	Domain         string   `json:"domain"`
	Port           int      `json:"port"`
	DisplayName    string   `json:"display_name"`
	CertExpiresAt  string   `json:"cert_expires_at"`
	Issuer         string   `json:"issuer"`
	SANNames       []string `json:"san_names"`
	CertPath       string   `json:"cert_path,omitempty"`
	SourceProvider string   `json:"source_provider"`
}

type SSLDiscoveryResponse struct {
	Domains []SSLDiscoveryResult `json:"domains"`
	Error   string               `json:"error,omitempty"`
}

type SSLDiscoveryImportRequest struct {
	Domains []SSLDiscoveryImportDomain `json:"domains"`
	Enabled *bool                      `json:"enabled,omitempty"`
}

type SSLDiscoveryImportDomain struct {
	Domain         string `json:"domain"`
	Port           int    `json:"port"`
	DisplayName    string `json:"display_name"`
	SourceProvider string `json:"source_provider"`
	ServerID       string `json:"server_id"`
}

// SSLSummary is the dashboard KPI card data
type SSLSummary struct {
	Total       int `json:"total"`
	Valid       int `json:"valid"`
	ExpiringSoon int `json:"expiring_soon"`
	Expired     int `json:"expired"`
	Error       int `json:"error"`
}

// SSLMonitorListResponse wraps paginated monitors
type SSLMonitorListResponse struct {
	Monitors   []SSLMonitorResponse `json:"monitors"`
	Total      int                  `json:"total"`
	Page       int                  `json:"page"`
	Limit      int                  `json:"limit"`
	TotalPages int                  `json:"total_pages"`
}

// ─── SSL Check History ─────────────────────────────────────────────────

type SSLCheckHistory struct {
	ID             string    `json:"id"`
	SSLMonitorID   string    `json:"ssl_monitor_id"`
	CheckedAt      time.Time `json:"checked_at"`
	Status         string    `json:"status"`
	DaysRemaining  int       `json:"days_remaining"`
	CipherGrade    string    `json:"cipher_grade"`
	TLSVersion     string    `json:"tls_version"`
	CipherSuite    string    `json:"cipher_suite"`
	ResponseTimeMs *int      `json:"response_time_ms"`
	Issuer         string    `json:"issuer"`
	Subject        string    `json:"subject"`
	ErrorMessage   string    `json:"error_message"`
}

type SSLCheckHistoryListResponse struct {
	Entries    []SSLCheckHistory `json:"entries"`
	Total      int               `json:"total"`
	Limit      int               `json:"limit"`
}

// ─── Uptime Monitoring ─────────────────────────────────────────────────────

type UptimeMonitor struct {
	ID                    string     `json:"id"`
	Name                  string     `json:"name"`
	URL                   string     `json:"url"`
	CheckType             string     `json:"check_type"`
	IntervalSeconds       int        `json:"interval_seconds"`
	TimeoutSeconds        int        `json:"timeout_seconds"`
	ExpectedStatusMin     int        `json:"expected_status_min"`
	ExpectedStatusMax     int        `json:"expected_status_max"`
	ExpectedBody          string     `json:"expected_body"`
	Enabled               bool       `json:"enabled"`
	NotificationTargetIDs []string   `json:"notification_target_ids"`
	Status                string     `json:"status"`
	LastStatus            string     `json:"last_status"`
	LastStatusCode        *int       `json:"last_status_code"`
	LastResponseTimeMs    *int       `json:"last_response_time_ms"`
	LastError             string     `json:"last_error"`
	LastCheckAt           *time.Time `json:"last_check_at"`
	CreatedBy             string     `json:"created_by"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

type UptimeMaintenanceWindow struct {
	ID          string    `json:"id"`
	MonitorID   string    `json:"monitor_id"`
	Description string    `json:"description"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// IsActive returns true if the maintenance window is currently active.
func (mw *UptimeMaintenanceWindow) IsActive() bool {
	now := time.Now()
	return now.After(mw.StartsAt) && now.Before(mw.EndsAt)
}

type UptimeCheckHistory struct {
	ID             string    `json:"id"`
	MonitorID      string    `json:"monitor_id"`
	CheckedAt      time.Time `json:"checked_at"`
	Status         string    `json:"status"`
	StatusCode     *int      `json:"status_code"`
	ResponseTimeMs *int      `json:"response_time_ms"`
	ErrorMessage   string    `json:"error_message"`
}

type UptimeDailySummary struct {
	MonitorID     string   `json:"monitor_id"`
	Date          string   `json:"date"`
	TotalChecks   int      `json:"total_checks"`
	UpCount       int      `json:"up_count"`
	DownCount     int      `json:"down_count"`
	AvgResponseMs *int     `json:"avg_response_ms"`
	MinResponseMs *int     `json:"min_response_ms"`
	MaxResponseMs *int     `json:"max_response_ms"`
	UptimePercent *float64 `json:"uptime_percent"`
}

type UptimeSummary struct {
	Total  int `json:"total"`
	Up     int `json:"up"`
	Down   int `json:"down"`
	Paused int `json:"paused"`
}

// ─── Notification Targets (shared — SSL + Uptime) ─────────────────────────

type NotificationTarget struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	URL           string    `json:"url"`
	Platform      string    `json:"platform"`
	WebhookSecret string    `json:"webhook_secret,omitempty"`
	BotToken      string    `json:"bot_token,omitempty"`
	ChatID        string    `json:"chat_id,omitempty"`
	Enabled       bool      `json:"enabled"`
	Scopes        []string  `json:"scopes"`
	CreatedBy     string    `json:"created_by"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type NotificationTargetRequest struct {
	Name          string   `json:"name"`
	URL           string   `json:"url"`
	Platform      string   `json:"platform"`
	WebhookSecret string   `json:"webhook_secret"`
	BotToken      string   `json:"bot_token,omitempty"`
	ChatID        string   `json:"chat_id,omitempty"`
	Enabled       *bool    `json:"enabled,omitempty"`
	Scopes        []string `json:"scopes"`
}

func (r *NotificationTargetRequest) Validate() string {
	if r.Name == "" {
		return "name is required"
	}
	if r.Platform == "" {
		r.Platform = "generic"
	}
	validPlatforms := map[string]bool{"telegram": true, "discord": true, "slack": true, "generic": true}
	if !validPlatforms[r.Platform] {
		return "platform must be telegram, discord, slack, or generic"
	}
	if r.Platform == "telegram" {
		if r.BotToken == "" {
			return "bot_token is required for telegram"
		}
		if r.ChatID == "" {
			return "chat_id is required for telegram"
		}
	} else if r.URL == "" {
		return "URL is required for this platform"
	}
	for _, s := range r.Scopes {
		validScopes := map[string]bool{"ssl": true, "uptime": true}
		if !validScopes[s] {
			return "scope must be ssl or uptime"
		}
	}
	return ""
}

// ─── Bookmarks ────────────────────────────────────────────────────────────────

type Bookmark struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	IconType  string    `json:"icon_type"`
	IconValue string    `json:"icon_value"`
	Category    string    `json:"category"`
	Description string    `json:"description"`
	Pinned      bool      `json:"pinned"`
	SortOrder   int       `json:"sort_order"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type BookmarkRequest struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	IconType    string `json:"icon_type,omitempty"`
	IconValue   string `json:"icon_value,omitempty"`
	Category    string `json:"category,omitempty"`
	Description string `json:"description,omitempty"`
	Pinned      *bool  `json:"pinned,omitempty"`
	SortOrder   *int   `json:"sort_order,omitempty"`
}

func (r *BookmarkRequest) Validate() string {
	if r.Title == "" {
		return "title is required"
	}
	if len(r.Title) > 100 {
		return "title must be 100 characters or less"
	}
	if r.URL == "" {
		return "url is required"
	}
	// Reject dangerous protocols
	lower := strings.ToLower(r.URL)
	if strings.HasPrefix(lower, "javascript:") || strings.HasPrefix(lower, "file:") || strings.HasPrefix(lower, "data:") {
		return "url uses an unsupported protocol"
	}
	// Auto-prepend https:// if no protocol
	if !strings.Contains(lower, "://") {
		r.URL = "https://" + r.URL
	}
	// No category validation — custom categories supported
	if r.IconType != "" && r.IconType != "auto" && r.IconType != "iconify" && r.IconType != "emoji" {
		return "icon_type must be auto, iconify, or emoji"
	}
	return ""
}

type BookmarkReorderItem struct {
	ID        string `json:"id"`
	SortOrder int    `json:"sort_order"`
}

// ─── Auth Events ────────────────────────────────────────────────────────────────

const (
	EventTypeLoginAttempt = "login_attempt"
	EventTypeLoginSuccess = "login_success"
	EventTypeLoginFailure = "login_failure"
	EventTypeLogout       = "logout"
	EventTypeLockout      = "lockout"
	EventTypePasswordChange = "password_change"
	EventTypeTOTPSetup    = "totp_setup"
	EventTypeTOTPDisable  = "totp_disable"
	EventTypeRegister     = "register"
	EventTypeRefreshToken = "refresh_token"
	EventTypeRateLimited  = "rate_limited"
	EventTypeIPBlocked    = "ip_blocked"

	EventStatusSuccess = "success"
	EventStatusFailure = "failure"
)

type AuthEvent struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id,omitempty"`
	Email         string    `json:"email"`
	EventType     string    `json:"event_type"`
	Status        string    `json:"status"`
	FailureReason string    `json:"failure_reason,omitempty"`
	IPAddress     string    `json:"ip_address"`
	UserAgent     string    `json:"user_agent,omitempty"`
	Country       string    `json:"country,omitempty"`
	ASN           string    `json:"asn,omitempty"`
	ISP           string    `json:"isp,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type AuthEventQuery struct {
	Page       int     `json:"page"`
	Limit      int     `json:"limit"`
	EventType  string  `json:"event_type"`
	Status     string  `json:"status"`
	UserID     string  `json:"user_id"`
	Email      string  `json:"email"`
	IPAddress  string  `json:"ip_address"`
	Search     string  `json:"search"`
	Sort       string  `json:"sort"`
	Order      string  `json:"order"`
	StartDate  *string `json:"start_date,omitempty"`
	EndDate    *string `json:"end_date,omitempty"`
}

type AuthEventListResponse struct {
	Events     []*AuthEvent `json:"events"`
	Total      int          `json:"total"`
	Page       int          `json:"page"`
	Limit      int          `json:"limit"`
	TotalPages int          `json:"total_pages"`
}

type AuthEventSummary struct {
	LoginsToday  int `json:"logins_today"`
	FailedToday  int `json:"failed_today"`
	LockedToday  int `json:"locked_today"`
	UniqueIPs    int `json:"unique_ips"`
	SuccessRate  int `json:"success_rate"` // percentage today
}

type AuthEventTrend struct {
	Date    string `json:"date"`
	Success int    `json:"success"`
	Failure int    `json:"failure"`
}

type BruteForceAlert struct {
	IPAddress     string `json:"ip_address"`
	Failures      int    `json:"failures"`
	WindowMinutes int    `json:"window_minutes"`
	FirstAttempt  string `json:"first_attempt"`
	LastAttempt   string `json:"last_attempt"`
	UserCount     int    `json:"user_count"`
}

type TopIPEntry struct {
	IPAddress string `json:"ip_address"`
	Failures  int    `json:"failures"`
	Users     int    `json:"users"`
	Country   string `json:"country,omitempty"`
}

type TopUserEntry struct {
	Email    string `json:"email"`
	Failures int    `json:"failures"`
	UserID   string `json:"user_id,omitempty"`
}

type HourlyHeatmapEntry struct {
	Hour    int `json:"hour"`
	Success int `json:"success"`
	Failure int `json:"failure"`
}

type BlockedIP struct {
	IPAddress string    `json:"ip_address"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy string    `json:"created_by,omitempty"`
	Reason    string    `json:"reason,omitempty"`
}
