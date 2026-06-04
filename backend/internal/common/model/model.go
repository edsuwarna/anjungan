package model

import (
	"encoding/json"
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
	CreatedBy      string    `json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ServerResponse is the public-safe version (no credentials exposed)
type ServerResponse struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Host           string    `json:"host"`
	Port           int       `json:"port"`
	SSHUser        string    `json:"ssh_user"`
	SSHAuthType    string    `json:"ssh_auth_type"`
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
	CreatedBy      string    `json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
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
		CreatedBy:      s.CreatedBy,
		CreatedAt:      s.CreatedAt,
		UpdatedAt:      s.UpdatedAt,
	}
}

// ServerRequest is the input for create/update operations
type CreateServerRequest struct {
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

type UpdateServerRequest struct {
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

const (
    RoleAdmin     = "admin"
    RoleDeveloper = "developer"
    RoleMember    = "member"
    RoleViewer    = "viewer"
)
