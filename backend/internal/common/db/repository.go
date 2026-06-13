package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/edsuwarna/anjungan/internal/common/model"
)

// ─── User repository ──────────────────────────────────────────────────────

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	u := &model.User{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, email, name, password_hash, totp_secret, totp_enabled, role, locked_until, failed_login_attempts, created_at, updated_at
		 FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.TOTPSecret, &u.TOTPEnabled, &u.Role, &u.LockedUntil, &u.FailedLoginAttempts, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *Repository) GetUserByID(ctx context.Context, id string) (*model.User, error) {
	u := &model.User{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, email, name, password_hash, totp_secret, totp_enabled, role, locked_until, failed_login_attempts, created_at, updated_at
		 FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.TOTPSecret, &u.TOTPEnabled, &u.Role, &u.LockedUntil, &u.FailedLoginAttempts, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *Repository) CreateUser(ctx context.Context, u *model.User) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO users (id, email, name, password_hash, totp_secret, totp_enabled, role, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		u.ID, u.Email, u.Name, u.PasswordHash, u.TOTPSecret, u.TOTPEnabled, u.Role, u.CreatedAt, u.UpdatedAt,
	)
	return err
}

func (r *Repository) UpdateUserLockout(ctx context.Context, userID string, lockedUntil *time.Time, failedAttempts int) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE users SET locked_until = $1, failed_login_attempts = $2, updated_at = NOW() WHERE id = $3`,
		lockedUntil, failedAttempts, userID,
	)
	return err
}

func (r *Repository) ResetUserLockout(ctx context.Context, userID string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE users SET locked_until = NULL, failed_login_attempts = 0, updated_at = NOW() WHERE id = $1`,
		userID,
	)
	return err
}

func (r *Repository) CountRecentFailedLogins(ctx context.Context, since time.Time) (int, error) {
	var count int
	err := r.db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM users WHERE failed_login_attempts > 0 AND updated_at >= $1`, since,
	).Scan(&count)
	return count, err
}

func (r *Repository) ListUsers(ctx context.Context) ([]*model.User, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, email, name, role, totp_enabled, locked_until, failed_login_attempts, created_at, updated_at FROM users ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*model.User
	for rows.Next() {
		u := &model.User{}
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.TOTPEnabled, &u.LockedUntil, &u.FailedLoginAttempts, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

// ─── Server repository ────────────────────────────────────────────────────

func (r *Repository) CreateServer(ctx context.Context, s *model.Server) error {
	// Handle nullable created_by (UUID column in PostgreSQL)
	// Using NULLIF to convert empty string to NULL before ::uuid cast
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO servers (id, name, host, port, ssh_user, ssh_auth_type, ssh_key, ssh_key_id, ssh_password,
		 status, tags, server_group, region, server_type, description, monitoring,
		 connection_type, is_self, self_hostname, created_by, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8, '')::uuid,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,
		 NULLIF($20, '')::uuid,
		 $21,$22)`,
		s.ID, s.Name, s.Host, s.Port, s.SSHUser, s.SSHAuthType, s.SSHKey, s.SSHKeyID, s.SSHPassword,
		s.Status, s.Tags, s.ServerGroup, s.Region, s.ServerType, s.Description,
		s.Monitoring, s.ConnectionType, s.IsSelf, s.SelfHostname, s.CreatedBy, s.CreatedAt, s.UpdatedAt,
	)
	return err
}

const serverColumns = `id, name, host, port, ssh_user, ssh_auth_type, status, container_count,
	COALESCE(tags, '{}'), COALESCE(labels, '{}')::text, COALESCE(server_group, ''),
	COALESCE(region, ''), COALESCE(server_type, ''), COALESCE(description, ''),
	COALESCE(os_info, ''), COALESCE(cpu_info, ''), last_seen_at, COALESCE(monitoring, false),
	COALESCE(connection_type, 'ssh'), COALESCE(is_self, false), COALESCE(self_hostname, ''),
	COALESCE(created_by::text, ''), created_at, updated_at, COALESCE(ssh_key_id::text, '')`

func scanServer(scanner interface {
	Scan(dest ...interface{}) error
}) (*model.Server, error) {
	s := &model.Server{}
	err := scanner.Scan(
		&s.ID, &s.Name, &s.Host, &s.Port, &s.SSHUser, &s.SSHAuthType,
		&s.Status, &s.ContainerCount, &s.Tags, &s.Labels,
		&s.ServerGroup, &s.Region, &s.ServerType, &s.Description,
		&s.OSInfo, &s.CPUInfo, &s.LastSeenAt, &s.Monitoring,
		&s.ConnectionType, &s.IsSelf, &s.SelfHostname,
		&s.CreatedBy, &s.CreatedAt, &s.UpdatedAt, &s.SSHKeyID,
	)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func scanServerFull(scanner interface {
	Scan(dest ...interface{}) error
}) (*model.Server, error) {
	s := &model.Server{}
	err := scanner.Scan(
		&s.ID, &s.Name, &s.Host, &s.Port, &s.SSHUser, &s.SSHAuthType,
		&s.SSHKey, &s.SSHPassword, &s.SSHKeyID, &s.Status, &s.ContainerCount,
		&s.Tags, &s.Labels, &s.ServerGroup, &s.Region, &s.ServerType,
		&s.Description, &s.OSInfo, &s.CPUInfo, &s.LastSeenAt, &s.Monitoring,
		&s.ConnectionType, &s.IsSelf, &s.SelfHostname,
		&s.CreatedBy, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// serverColumnsWithCompliance extends serverColumns with compliance data from the latest scan
const serverColumnsWithCompliance = `s.id, s.name, s.host, s.port, s.ssh_user, s.ssh_auth_type, s.status, s.container_count,
	COALESCE(s.tags, '{}'), COALESCE(s.labels, '{}')::text, COALESCE(s.server_group, ''),
	COALESCE(s.region, ''), COALESCE(s.server_type, ''), COALESCE(s.description, ''),
	COALESCE(s.os_info, ''), COALESCE(s.cpu_info, ''), s.last_seen_at, COALESCE(s.monitoring, false),
	COALESCE(s.connection_type, 'ssh'), COALESCE(s.is_self, false), COALESCE(s.self_hostname, ''),
	COALESCE(s.created_by::text, ''), s.created_at, s.updated_at, COALESCE(s.ssh_key_id::text, ''),
	sr.score, COALESCE(sr.criticals, 0), COALESCE(sr.warnings, 0), COALESCE(sr.passed, 0), sr.completed_at`

// scanServerResponseWithCompliance scans rows from a query that joins servers with compliance data
func scanServerResponseWithCompliance(scanner interface {
	Scan(dest ...interface{}) error
}) (*model.ServerResponse, error) {
	s := &model.ServerResponse{}
	var score *int
	var lastScan *time.Time
	var sshKeyID string
	err := scanner.Scan(
		&s.ID, &s.Name, &s.Host, &s.Port, &s.SSHUser, &s.SSHAuthType,
		&s.Status, &s.ContainerCount, &s.Tags, &s.Labels,
		&s.ServerGroup, &s.Region, &s.ServerType, &s.Description,
		&s.OSInfo, &s.CPUInfo, &s.LastSeenAt, &s.Monitoring,
		&s.ConnectionType, &s.IsSelf, &s.SelfHostname,
		&s.CreatedBy, &s.CreatedAt, &s.UpdatedAt, &sshKeyID,
		&score, &s.Criticals, &s.Warnings, &s.Passed, &lastScan,
	)
	if err != nil {
		return nil, err
	}
	s.Score = score
	s.LastScan = lastScan
	return s, nil
}

func (r *Repository) ListServers(ctx context.Context) ([]*model.Server, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT `+serverColumns+` FROM servers ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []*model.Server
	for rows.Next() {
		s, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		servers = append(servers, s)
	}
	return servers, nil
}

func (r *Repository) ListServersPaginated(ctx context.Context, q model.ServerListQuery, allowedGroups []string) (*model.ServerListResponse, error) {
	// nil allowedGroups = admin/unrestricted → no group filter
	// non-nil empty = restricted user with no groups → return empty
	// non-nil non-empty = filter by groups
	if allowedGroups != nil && len(allowedGroups) == 0 {
		page := q.Page
		if page < 1 {
			page = 1
		}
		limit := q.Limit
		if limit < 1 {
			limit = 50
		}
		if limit > 200 {
			limit = 200
		}
		totalPages := 1
		return &model.ServerListResponse{
			Servers:    []model.ServerResponse{},
			Total:      0,
			Page:       page,
			Limit:      limit,
			TotalPages: totalPages,
		}, nil
	}

	// Build WHERE clauses
	var conditions []string
	var args []interface{}
	argIdx := 1

	if q.Status != "" {
		conditions = append(conditions, fmt.Sprintf("s.status = $%d", argIdx))
		args = append(args, q.Status)
		argIdx++
	}
	if q.Search != "" {
		conditions = append(conditions, fmt.Sprintf("(LOWER(s.name) LIKE $%d OR LOWER(s.host) LIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+strings.ToLower(q.Search)+"%")
		argIdx++
	}
	if q.ServerGroup != "" {
		conditions = append(conditions, fmt.Sprintf("s.server_group = $%d", argIdx))
		args = append(args, q.ServerGroup)
		argIdx++
	}
	if q.Region != "" {
		conditions = append(conditions, fmt.Sprintf("s.region = $%d", argIdx))
		args = append(args, q.Region)
		argIdx++
	}
	if q.ServerType != "" {
		conditions = append(conditions, fmt.Sprintf("s.server_type = $%d", argIdx))
		args = append(args, q.ServerType)
		argIdx++
	}
	// Filter by allowed groups if set (non-admin users)
	if len(allowedGroups) > 0 {
		placeholders := make([]string, len(allowedGroups))
		for i, g := range allowedGroups {
			placeholders[i] = fmt.Sprintf("$%d", argIdx)
			args = append(args, g)
			argIdx++
		}
		conditions = append(conditions, fmt.Sprintf("s.server_group IN (%s)", strings.Join(placeholders, ",")))
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count
	countQuery := "SELECT COUNT(*) FROM servers s " + whereClause
	var total int
	if err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, err
	}

	// Sort
	allowedSorts := map[string]string{
		"name": "s.name", "host": "s.host", "status": "s.status",
		"created_at": "s.created_at", "updated_at": "s.updated_at",
		"server_group": "s.server_group", "region": "s.region", "server_type": "s.server_type",
	}
	sortCol, ok := allowedSorts[q.Sort]
	if !ok {
		sortCol = "s.name"
	}
	order := "ASC"
	if strings.EqualFold(q.Order, "desc") {
		order = "DESC"
	}

	// Pagination
	page := q.Page
	if page < 1 {
		page = 1
	}
	limit := q.Limit
	if limit < 1 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset := (page - 1) * limit

	dataQuery := fmt.Sprintf(
		`SELECT `+serverColumnsWithCompliance+` FROM servers s
		LEFT JOIN LATERAL (
		    SELECT score, criticals, warnings, passed, completed_at
		    FROM scan_results
		    WHERE server_id = s.id AND status = 'completed'
		    ORDER BY created_at DESC LIMIT 1
		) sr ON true
		%s ORDER BY %s %s LIMIT $%d OFFSET $%d`,
		whereClause, sortCol, order, argIdx, argIdx+1,
	)
	args = append(args, limit, offset)

	rows, err := r.db.Pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []model.ServerResponse
	for rows.Next() {
		s, err := scanServerResponseWithCompliance(rows)
		if err != nil {
			return nil, err
		}
		servers = append(servers, *s)
	}
	if servers == nil {
		servers = []model.ServerResponse{}
	}

	totalPages := (total + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	return &model.ServerListResponse{
		Servers:    servers,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

func (r *Repository) ListServersByGroups(ctx context.Context, allowedGroups []string) ([]*model.Server, error) {
	// nil allowedGroups = admin/unrestricted → return all servers
	// non-nil empty = restricted user with no groups → return empty
	// non-nil non-empty = filter by groups
	if allowedGroups != nil && len(allowedGroups) == 0 {
		return []*model.Server{}, nil
	}

	var query string
	var args []interface{}
	if len(allowedGroups) > 0 {
		placeholders := make([]string, len(allowedGroups))
		for i, g := range allowedGroups {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			args = append(args, g)
		}
		query = fmt.Sprintf(`SELECT `+serverColumns+` FROM servers WHERE server_group IN (%s) ORDER BY name`, strings.Join(placeholders, ","))
	} else {
		query = `SELECT `+serverColumns+` FROM servers ORDER BY name`
	}
	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []*model.Server
	for rows.Next() {
		s, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		servers = append(servers, s)
	}
	return servers, nil
}

func (r *Repository) GetServerByID(ctx context.Context, id string) (*model.Server, error) {
	row := r.db.Pool.QueryRow(ctx,
		`SELECT `+serverColumns+` FROM servers WHERE id = $1`, id,
	)
	return scanServer(row)
}

func (r *Repository) GetServerByIDFull(ctx context.Context, id string) (*model.Server, error) {
	row := r.db.Pool.QueryRow(ctx,
		`SELECT id, name, host, port, ssh_user, ssh_auth_type, ssh_key, ssh_password, COALESCE(ssh_key_id::text, ''), status, container_count,
		 COALESCE(tags, '{}'), COALESCE(labels, '{}')::text, COALESCE(server_group, ''),
		 COALESCE(region, ''), COALESCE(server_type, ''), COALESCE(description, ''),
		 COALESCE(os_info, ''), COALESCE(cpu_info, ''), last_seen_at, COALESCE(monitoring, false),
		COALESCE(connection_type, 'ssh'), COALESCE(is_self, false), COALESCE(self_hostname, ''), COALESCE(created_by::text, ''), created_at, updated_at
		 FROM servers WHERE id = $1`, id,
	)
	return scanServerFull(row)
}

// GetSelfServer returns the server marked as self (the host Anjungan runs on).
// Returns nil, nil if no self server is registered.
func (r *Repository) GetSelfServer(ctx context.Context) (*model.Server, error) {
	row := r.db.Pool.QueryRow(ctx,
		`SELECT id, name, host, port, ssh_user, ssh_auth_type, ssh_key, ssh_password, COALESCE(ssh_key_id::text, ''), status, container_count,
		 COALESCE(tags, '{}'), COALESCE(labels, '{}')::text, COALESCE(server_group, ''),
		 COALESCE(region, ''), COALESCE(server_type, ''), COALESCE(description, ''),
		 COALESCE(os_info, ''), COALESCE(cpu_info, ''), last_seen_at, COALESCE(monitoring, false),
		 COALESCE(connection_type, 'ssh'), COALESCE(is_self, false), COALESCE(self_hostname, ''), COALESCE(created_by::text, ''), created_at, updated_at
		 FROM servers WHERE is_self = true LIMIT 1`,
	)
	s, err := scanServerFull(row)
	if err != nil {
		// pgx.ErrNoRows — no self server registered yet
		return nil, nil
	}
	return s, nil
}

// FindOrCreateSelfServer finds the existing self server or creates a new one.
// Returns the server and a boolean indicating if it was newly created.
func (r *Repository) FindOrCreateSelfServer(ctx context.Context, s *model.Server) (*model.Server, bool, error) {
	existing, err := r.GetSelfServer(ctx)
	if err != nil {
		return nil, false, err
	}
	if existing != nil {
		// Update OS info, hostname, etc. but keep existing
		existing.SelfHostname = s.SelfHostname
		existing.OSInfo = s.OSInfo
		existing.CPUInfo = s.CPUInfo
		existing.Host = s.Host
		existing.Status = "online"
		existing.ConnectionType = s.ConnectionType
		if err := r.UpdateServer(ctx, existing); err != nil {
			log.Printf("[self] failed to update self-server: %v", err)
			return existing, false, nil
		}
		return existing, false, nil
	}
	// Create new self server
	if err := r.CreateServer(ctx, s); err != nil {
		return nil, false, err
	}
	return s, true, nil
}

func (r *Repository) UpdateServer(ctx context.Context, s *model.Server) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE servers SET name=$1, host=$2, port=$3, ssh_user=$4, ssh_auth_type=$5, ssh_key=$6,
		 ssh_password=$7, ssh_key_id=NULLIF($8, '')::uuid, status=$9, container_count=$10, tags=$11, server_group=$12, region=$13,
		 server_type=$14, description=$15, os_info=$16, cpu_info=$17, monitoring=$18,
		 connection_type=$19, is_self=$20, self_hostname=$21, updated_at=NOW()
		 WHERE id=$22`,
		s.Name, s.Host, s.Port, s.SSHUser, s.SSHAuthType, s.SSHKey, s.SSHPassword,
		s.SSHKeyID, s.Status, s.ContainerCount, s.Tags, s.ServerGroup, s.Region,
		s.ServerType, s.Description, s.OSInfo, s.CPUInfo, s.Monitoring,
		s.ConnectionType, s.IsSelf, s.SelfHostname, s.ID,
	)
	return err
}

func (r *Repository) UpdateServerStatus(ctx context.Context, id string, status string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE servers SET status=$1, last_seen_at=NOW(), updated_at=NOW() WHERE id=$2`, status, id,
	)
	return err
}

func (r *Repository) UpdateServerContainerCount(ctx context.Context, id string, count int) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE servers SET container_count=$1, updated_at=NOW() WHERE id=$2`, count, id,
	)
	return err
}

func (r *Repository) UpdateServerInfo(ctx context.Context, id, osInfo, cpuInfo string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE servers SET os_info=$1, cpu_info=$2, updated_at=NOW() WHERE id=$3`, osInfo, cpuInfo, id,
	)
	return err
}

func (r *Repository) DeleteServer(ctx context.Context, id string) error {
	_, err := r.db.Pool.Exec(ctx, "DELETE FROM servers WHERE id = $1", id)
	return err
}

func (r *Repository) BulkDeleteServers(ctx context.Context, ids []string) error {
	_, err := r.db.Pool.Exec(ctx, "DELETE FROM servers WHERE id = ANY($1)", ids)
	return err
}

func (r *Repository) CountServers(ctx context.Context) (int, error) {
	var count int
	err := r.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM servers").Scan(&count)
	return count, err
}

func (r *Repository) ListRecentServers(ctx context.Context, limit int) ([]*model.Server, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT `+serverColumns+` FROM servers ORDER BY created_at DESC LIMIT $1`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []*model.Server
	for rows.Next() {
		s, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		servers = append(servers, s)
	}
	return servers, nil
}

func (r *Repository) CountServersByStatus(ctx context.Context) (map[string]int, error) {
	rows, err := r.db.Pool.Query(ctx, "SELECT status, COUNT(*) FROM servers GROUP BY status")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		result[status] = count
	}
	return result, nil
}

func (r *Repository) CountServersByGroups(ctx context.Context, allowedGroups []string) (int, error) {
	if allowedGroups == nil {
		return r.CountServers(ctx)
	}
	if len(allowedGroups) == 0 {
		return 0, nil
	}
	var count int
	err := r.db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM servers WHERE server_group = ANY($1)`, allowedGroups,
	).Scan(&count)
	return count, err
}

func (r *Repository) CountServersByStatusByGroups(ctx context.Context, allowedGroups []string) (map[string]int, error) {
	if allowedGroups == nil {
		return r.CountServersByStatus(ctx)
	}
	if len(allowedGroups) == 0 {
		return map[string]int{}, nil
	}
	rows, err := r.db.Pool.Query(ctx,
		"SELECT status, COUNT(*) FROM servers WHERE server_group = ANY($1) GROUP BY status", allowedGroups,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		result[status] = count
	}
	return result, nil
}

func (r *Repository) SumContainerCountByGroups(ctx context.Context, allowedGroups []string) (int, error) {
	if allowedGroups == nil {
		return r.SumContainerCount(ctx)
	}
	if len(allowedGroups) == 0 {
		return 0, nil
	}
	var count int
	err := r.db.Pool.QueryRow(ctx,
		"SELECT COALESCE(SUM(container_count), 0) FROM servers WHERE server_group = ANY($1)", allowedGroups,
	).Scan(&count)
	return count, err
}

func (r *Repository) CountUsers(ctx context.Context) (int, error) {
	var count int
	err := r.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

func (r *Repository) SumContainerCount(ctx context.Context) (int, error) {
	var count int
	err := r.db.Pool.QueryRow(ctx, "SELECT COALESCE(SUM(container_count), 0) FROM servers").Scan(&count)
	return count, err
}

// ─── Server Metrics Repository ─────────────────────────────────────────────

func (r *Repository) SaveMetrics(ctx context.Context, m *model.ServerMetricsPoint) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO server_metrics (server_id, cpu_load_1, cpu_load_5, cpu_load_15,
		 mem_used_bytes, mem_total_bytes, disk_used_bytes, disk_total_bytes, disk_used_pct,
		 net_rx_bytes, net_tx_bytes, collected_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		m.ServerID, m.CPULoad1, m.CPULoad5, m.CPULoad15,
		m.MemUsedBytes, m.MemTotalBytes, m.DiskUsedBytes, m.DiskTotalBytes, m.DiskUsedPct,
		m.NetRXBytes, m.NetTXBytes, m.CollectedAt,
	)
	return err
}

func (r *Repository) GetHistoricalMetrics(ctx context.Context, serverID string, since time.Time, limit int) ([]*model.ServerMetricsPoint, error) {
	if limit < 1 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, server_id, cpu_load_1, cpu_load_5, cpu_load_15,
		 mem_used_bytes, mem_total_bytes, disk_used_bytes, disk_total_bytes, disk_used_pct,
		 net_rx_bytes, net_tx_bytes, collected_at
		 FROM server_metrics
		 WHERE server_id = $1 AND collected_at >= $2
		 ORDER BY collected_at DESC LIMIT $3`,
		serverID, since, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []*model.ServerMetricsPoint
	for rows.Next() {
		p := &model.ServerMetricsPoint{}
		if err := rows.Scan(&p.ID, &p.ServerID, &p.CPULoad1, &p.CPULoad5, &p.CPULoad15,
			&p.MemUsedBytes, &p.MemTotalBytes, &p.DiskUsedBytes, &p.DiskTotalBytes, &p.DiskUsedPct,
			&p.NetRXBytes, &p.NetTXBytes, &p.CollectedAt); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, nil
}

// ─── Alerts Repository ─────────────────────────────────────────────────────

func (r *Repository) CreateAlert(ctx context.Context, a *model.Alert) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO alerts (id, server_id, type, severity, message, value, threshold, acknowledged, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		a.ID, a.ServerID, a.Type, a.Severity, a.Message, a.Value, a.Threshold, a.Acknowledged, a.CreatedAt,
	)
	return err
}

func (r *Repository) ListAlerts(ctx context.Context, limit int, unreadOnly bool) ([]*model.Alert, error) {
	if limit < 1 {
		limit = 50
	}

	query := `SELECT id, server_id, type, severity, message, value, threshold, acknowledged, created_at
		FROM alerts`
	if unreadOnly {
		query += " WHERE NOT acknowledged"
	}
	query += " ORDER BY created_at DESC LIMIT $1"

	if unreadOnly {
		var count int
		if err := r.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM alerts WHERE NOT acknowledged").Scan(&count); err != nil {
			return nil, err
		}
	}

	rows, err := r.db.Pool.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []*model.Alert
	for rows.Next() {
		a := &model.Alert{}
		if err := rows.Scan(&a.ID, &a.ServerID, &a.Type, &a.Severity, &a.Message, &a.Value, &a.Threshold, &a.Acknowledged, &a.CreatedAt); err != nil {
			return nil, err
		}
		alerts = append(alerts, a)
	}
	return alerts, nil
}

func (r *Repository) CountUnacknowledgedAlerts(ctx context.Context) (int, error) {
	var count int
	err := r.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM alerts WHERE NOT acknowledged").Scan(&count)
	return count, err
}

func (r *Repository) AcknowledgeAlert(ctx context.Context, id string) error {
	_, err := r.db.Pool.Exec(ctx, "UPDATE alerts SET acknowledged=TRUE WHERE id=$1", id)
	return err
}

func (r *Repository) CountAlertsBySeverity(ctx context.Context) (map[string]int, error) {
	rows, err := r.db.Pool.Query(ctx, "SELECT severity, COUNT(*) FROM alerts WHERE NOT acknowledged GROUP BY severity")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var sev string
		var count int
		if err := rows.Scan(&sev, &count); err != nil {
			return nil, err
		}
		result[sev] = count
	}
	return result, nil
}

// ─── Distinct values for filter dropdowns ──────────────────────────────────

func (r *Repository) ListServerGroups(ctx context.Context) ([]string, error) {
	rows, err := r.db.Pool.Query(ctx,
		"SELECT DISTINCT server_group FROM servers WHERE server_group != '' ORDER BY server_group")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []string
	for rows.Next() {
		var g string
		if err := rows.Scan(&g); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, nil
}

func (r *Repository) ListRegions(ctx context.Context) ([]string, error) {
	rows, err := r.db.Pool.Query(ctx,
		"SELECT DISTINCT region FROM servers WHERE region != '' ORDER BY region")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var regions []string
	for rows.Next() {
		var reg string
		if err := rows.Scan(&reg); err != nil {
			return nil, err
		}
		regions = append(regions, reg)
	}
	return regions, nil
}

func (r *Repository) ListServerTypes(ctx context.Context) ([]string, error) {
	rows, err := r.db.Pool.Query(ctx,
		"SELECT DISTINCT server_type FROM servers WHERE server_type != '' ORDER BY server_type")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var types []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		types = append(types, t)
	}
	return types, nil
}

// ─── Activity / Recent Events ──────────────────────────────────────────────

func (r *Repository) SaveActivity(ctx context.Context, activityType, message, userID string) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO activity_log (type, message, user_id, created_at)
		 VALUES ($1,$2,$3,NOW())`,
		activityType, message, userID,
	)
	return err
}

func (r *Repository) ListRecentActivity(ctx context.Context, limit int) ([]struct {
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}, error) {
	if limit < 1 {
		limit = 20
	}

	rows, err := r.db.Pool.Query(ctx,
		`SELECT type, message, created_at FROM activity_log ORDER BY created_at DESC LIMIT $1`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var activities []struct {
		Type      string    `json:"type"`
		Message   string    `json:"message"`
		Timestamp time.Time `json:"timestamp"`
	}
	for rows.Next() {
		var a struct {
			Type      string    `json:"type"`
			Message   string    `json:"message"`
			Timestamp time.Time `json:"timestamp"`
		}
		if err := rows.Scan(&a.Type, &a.Message, &a.Timestamp); err != nil {
			return nil, err
		}
		activities = append(activities, a)
	}
	return activities, nil
}

// ─── SSH Keys Repository ──────────────────────────────────────────────────

func (r *Repository) CreateSSHKey(ctx context.Context, k *model.SSHKey) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO ssh_keys (id, name, key_type, private_key, public_key, fingerprint, created_by, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		k.ID, k.Name, k.KeyType, k.PrivateKey, k.PublicKey, k.Fingerprint, k.CreatedBy, k.CreatedAt, k.UpdatedAt,
	)
	return err
}

const sshKeyColumns = `id, name, key_type, COALESCE(public_key, ''), COALESCE(fingerprint, ''), created_by, created_at, updated_at`

func scanSSHKey(scanner interface {
	Scan(dest ...interface{}) error
}) (*model.SSHKey, error) {
	k := &model.SSHKey{}
	err := scanner.Scan(&k.ID, &k.Name, &k.KeyType, &k.PublicKey, &k.Fingerprint, &k.CreatedBy, &k.CreatedAt, &k.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return k, nil
}

func scanSSHKeyFull(scanner interface {
	Scan(dest ...interface{}) error
}) (*model.SSHKey, error) {
	k := &model.SSHKey{}
	err := scanner.Scan(&k.ID, &k.Name, &k.KeyType, &k.PrivateKey, &k.PublicKey, &k.Fingerprint, &k.CreatedBy, &k.CreatedAt, &k.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return k, nil
}

func (r *Repository) ListSSHKeys(ctx context.Context) ([]*model.SSHKey, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT `+sshKeyColumns+` FROM ssh_keys ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*model.SSHKey
	for rows.Next() {
		k, err := scanSSHKey(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, nil
}

func (r *Repository) GetSSHKeyByID(ctx context.Context, id string) (*model.SSHKey, error) {
	row := r.db.Pool.QueryRow(ctx,
		`SELECT `+sshKeyColumns+` FROM ssh_keys WHERE id = $1`, id,
	)
	return scanSSHKey(row)
}

func (r *Repository) GetSSHKeyByIDFull(ctx context.Context, id string) (*model.SSHKey, error) {
	row := r.db.Pool.QueryRow(ctx,
		`SELECT id, name, key_type, private_key, COALESCE(public_key, ''), COALESCE(fingerprint, ''), created_by, created_at, updated_at FROM ssh_keys WHERE id = $1`, id,
	)
	return scanSSHKeyFull(row)
}

func (r *Repository) UpdateSSHKey(ctx context.Context, k *model.SSHKey) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE ssh_keys SET name=$1, key_type=$2, private_key=$3, public_key=$4, fingerprint=$5, updated_at=NOW() WHERE id=$6`,
		k.Name, k.KeyType, k.PrivateKey, k.PublicKey, k.Fingerprint, k.ID,
	)
	return err
}

func (r *Repository) DeleteSSHKey(ctx context.Context, id string) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM ssh_keys WHERE id = $1`, id)
	return err
}

func (r *Repository) CountServersUsingSSHKey(ctx context.Context, keyID string) (int, error) {
	var count int
	err := r.db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM servers WHERE ssh_key_id = $1`, keyID,
	).Scan(&count)
	return count, err
}

// ─── User Server Groups ──────────────────────────────────────────────────

func (r *Repository) SetUserServerGroups(ctx context.Context, userID string, groups []string) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Delete existing
	if _, err := tx.Exec(ctx, `DELETE FROM user_server_groups WHERE user_id = $1`, userID); err != nil {
		return err
	}

	// Insert new (trimmed)
	for _, g := range groups {
		g = strings.TrimSpace(g)
		if g == "" {
			continue
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO user_server_groups (user_id, server_group) VALUES ($1, $2)`,
			userID, g,
		); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *Repository) GetUserServerGroups(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT server_group FROM user_server_groups WHERE user_id = $1 ORDER BY server_group`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []string
	for rows.Next() {
		var g string
		if err := rows.Scan(&g); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	if groups == nil {
		groups = []string{}
	}
	return groups, nil
}

func (r *Repository) DeleteUserServerGroups(ctx context.Context, userID string) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM user_server_groups WHERE user_id = $1`, userID)
	return err
}

// ─── User Management (Admin) ──────────────────────────────────────────────

func (r *Repository) UpdateUser(ctx context.Context, u *model.User) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE users SET name=$1, email=$2, role=$3, updated_at=NOW() WHERE id=$4`,
		u.Name, u.Email, u.Role, u.ID,
	)
	return err
}

func (r *Repository) UpdateUserPassword(ctx context.Context, id, passwordHash string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE users SET password_hash=$1, updated_at=NOW() WHERE id=$2`,
		passwordHash, id,
	)
	return err
}

func (r *Repository) UpdateUserTOTPSecret(ctx context.Context, id, secret string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE users SET totp_secret=$1, updated_at=NOW() WHERE id=$2`,
		secret, id,
	)
	return err
}

func (r *Repository) UpdateUserTOTPEnabled(ctx context.Context, id string, enabled bool) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE users SET totp_enabled=$1, updated_at=NOW() WHERE id=$2`,
		enabled, id,
	)
	return err
}

func (r *Repository) DeleteUser(ctx context.Context, id string) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	return err
}

func (r *Repository) CountAdminUsers(ctx context.Context) (int, error) {
	var count int
	err := r.db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM users WHERE role = 'admin'`,
	).Scan(&count)
	return count, err
}

// ─── Audit Log Repository ─────────────────────────────────────────────────

func (r *Repository) CreateAuditLog(ctx context.Context, e *model.AuditLogEntry) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO audit_logs (id, action, entity_type, entity_id, description, user_id, user_email, ip_address, metadata, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		e.ID, e.Action, e.EntityType, e.EntityID, e.Description,
		e.UserID, e.UserEmail, e.IPAddress, e.Metadata, e.CreatedAt,
	)
	return err
}

func (r *Repository) ListAuditLogs(ctx context.Context, q model.AuditLogQuery) (*model.AuditLogListResponse, error) {
	var conditions []string
	var args []interface{}
	argIdx := 1

	if q.Action != "" {
		conditions = append(conditions, fmt.Sprintf("a.action = $%d", argIdx))
		args = append(args, q.Action)
		argIdx++
	}
	if q.EntityType != "" {
		conditions = append(conditions, fmt.Sprintf("a.entity_type = $%d", argIdx))
		args = append(args, q.EntityType)
		argIdx++
	}
	if q.UserID != "" {
		conditions = append(conditions, fmt.Sprintf("a.user_id = $%d", argIdx))
		args = append(args, q.UserID)
		argIdx++
	}
	if q.Search != "" {
		conditions = append(conditions, fmt.Sprintf(
			"(LOWER(a.description) LIKE $%d OR LOWER(a.user_email) LIKE $%d OR LOWER(a.entity_id) LIKE $%d)",
			argIdx, argIdx, argIdx,
		))
		args = append(args, "%"+strings.ToLower(q.Search)+"%")
		argIdx++
	}
	if q.StartDate != nil && *q.StartDate != "" {
		conditions = append(conditions, fmt.Sprintf("a.created_at >= $%d::timestamptz", argIdx))
		args = append(args, *q.StartDate)
		argIdx++
	}
	if q.EndDate != nil && *q.EndDate != "" {
		conditions = append(conditions, fmt.Sprintf("a.created_at <= $%d::timestamptz", argIdx))
		args = append(args, *q.EndDate)
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count
	countQuery := "SELECT COUNT(*) FROM audit_logs a " + whereClause
	var total int
	if err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, err
	}

	// Pagination
	page := q.Page
	if page < 1 {
		page = 1
	}
	limit := q.Limit
	if limit < 1 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset := (page - 1) * limit

	// Dynamic sort/order
	sortCol := "a.created_at"
	if q.Sort == "action" {
		sortCol = "LOWER(a.action)"
	} else if q.Sort == "entity_type" {
		sortCol = "LOWER(a.entity_type)"
	} else if q.Sort == "user_email" {
		sortCol = "LOWER(a.user_email)"
	} else if q.Sort == "description" {
		sortCol = "LOWER(a.description)"
	} else if q.Sort == "ip_address" {
		sortCol = "LOWER(a.ip_address)"
	} else if q.Sort == "created_at" {
		sortCol = "a.created_at"
	}
	orderDir := "DESC"
	if q.Order == "asc" {
		orderDir = "ASC"
	}

	dataQuery := fmt.Sprintf(
		`SELECT a.id, a.action, a.entity_type, COALESCE(a.entity_id, ''), a.description,
		 COALESCE(a.user_id::text, ''), COALESCE(a.user_email, ''), COALESCE(a.ip_address, ''),
		 COALESCE(a.metadata, '{}'), a.created_at
		 FROM audit_logs a %s ORDER BY %s %s LIMIT $%d OFFSET $%d`,
		whereClause, sortCol, orderDir, argIdx, argIdx+1,
	)
	args = append(args, limit, offset)

	rows, err := r.db.Pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*model.AuditLogEntry
	for rows.Next() {
		e := &model.AuditLogEntry{}
		if err := rows.Scan(&e.ID, &e.Action, &e.EntityType, &e.EntityID,
			&e.Description, &e.UserID, &e.UserEmail, &e.IPAddress,
			&e.Metadata, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if entries == nil {
		entries = []*model.AuditLogEntry{}
	}

	totalPages := (total + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	return &model.AuditLogListResponse{
		Entries:    entries,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

func (r *Repository) ListAuditActions(ctx context.Context) ([]string, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT DISTINCT action FROM audit_logs ORDER BY action`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var actions []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			return nil, err
		}
		actions = append(actions, a)
	}
	return actions, nil
}

func (r *Repository) ListAuditEntityTypes(ctx context.Context) ([]string, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT DISTINCT entity_type FROM audit_logs ORDER BY entity_type`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var types []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		types = append(types, t)
	}
	return types, nil
}

// ─── Lockout Events ───────────────────────────────────────────────────

type LockoutEvent struct {
	IP             string    `json:"ip"`
	FailedAttempts int       `json:"failed_attempts"`
	LastAttempt    time.Time `json:"last_attempt"`
	Status         string    `json:"status"` // "locked" or "unlocked"
}

func (r *Repository) ListRecentLockoutEvents(ctx context.Context, limit int) ([]LockoutEvent, error) {
	// This is a simplified view - for real implementation we'd query audit_logs for auth.login failures
	// For now, return users with failed_login_attempts > 0
	rows, err := r.db.Pool.Query(ctx,
		`SELECT email, failed_login_attempts, updated_at,
			CASE WHEN locked_until IS NOT NULL AND locked_until > NOW() THEN 'locked' ELSE 'unlocked' END as status
		 FROM users WHERE failed_login_attempts > 0
		 ORDER BY updated_at DESC LIMIT $1`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []LockoutEvent
	for rows.Next() {
		var e LockoutEvent
		if err := rows.Scan(&e.IP, &e.FailedAttempts, &e.LastAttempt, &e.Status); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

// ─── Compliance Scan ───────────────────────────────────────────────────

func (r *Repository) CreateScanResult(ctx context.Context, s *model.ScanResult) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO scan_results (id, server_id, scan_type, status, score, total_checks, passed, warnings, criticals, started_at, completed_at, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		s.ID, s.ServerID, s.ScanType, s.Status, s.Score, s.TotalChecks,
		s.Passed, s.Warnings, s.Criticals, s.StartedAt, s.CompletedAt, s.CreatedAt,
	)
	return err
}

func (r *Repository) UpdateScanResult(ctx context.Context, s *model.ScanResult) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE scan_results SET status=$1, score=$2, total_checks=$3, passed=$4, warnings=$5, criticals=$6, completed_at=$7, error_message=$8
		 WHERE id=$9`,
		s.Status, s.Score, s.TotalChecks, s.Passed, s.Warnings, s.Criticals, s.CompletedAt, s.ErrorMessage, s.ID,
	)
	return err
}

func (r *Repository) GetLatestScanResult(ctx context.Context, serverID string) (*model.ScanResult, error) {
	s := &model.ScanResult{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, server_id, scan_type, status, score, total_checks, passed, warnings, criticals, error_message, started_at, completed_at, created_at
		 FROM scan_results WHERE server_id = $1 ORDER BY created_at DESC LIMIT 1`, serverID,
	).Scan(&s.ID, &s.ServerID, &s.ScanType, &s.Status, &s.Score, &s.TotalChecks,
		&s.Passed, &s.Warnings, &s.Criticals, &s.ErrorMessage, &s.StartedAt, &s.CompletedAt, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (r *Repository) GetLatestScanResultByType(ctx context.Context, serverID, scanType string) (*model.ScanResult, error) {
	s := &model.ScanResult{}
	var err error
	if scanType == "CIS Docker" {
		err = r.db.Pool.QueryRow(ctx,
			`SELECT id, server_id, scan_type, status, score, total_checks, passed, warnings, criticals, error_message, started_at, completed_at, created_at
			 FROM scan_results WHERE server_id = $1 AND scan_type IN ($2, $3) AND status = 'completed' ORDER BY created_at DESC LIMIT 1`, serverID, "CIS Docker", "Container Security",
		).Scan(&s.ID, &s.ServerID, &s.ScanType, &s.Status, &s.Score, &s.TotalChecks,
			&s.Passed, &s.Warnings, &s.Criticals, &s.ErrorMessage, &s.StartedAt, &s.CompletedAt, &s.CreatedAt)
	} else {
		err = r.db.Pool.QueryRow(ctx,
			`SELECT id, server_id, scan_type, status, score, total_checks, passed, warnings, criticals, error_message, started_at, completed_at, created_at
			 FROM scan_results WHERE server_id = $1 AND scan_type = $2 ORDER BY created_at DESC LIMIT 1`, serverID, scanType,
		).Scan(&s.ID, &s.ServerID, &s.ScanType, &s.Status, &s.Score, &s.TotalChecks,
			&s.Passed, &s.Warnings, &s.Criticals, &s.ErrorMessage, &s.StartedAt, &s.CompletedAt, &s.CreatedAt)
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

// GetLatestCompletedScanByType returns the latest COMPLETED scan result for a server by scan type.
func (r *Repository) GetLatestCompletedScanByType(ctx context.Context, serverID, scanType string) (*model.ScanResult, error) {
	s := &model.ScanResult{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, server_id, scan_type, status, score, total_checks, passed, warnings, criticals, error_message, started_at, completed_at, created_at
		 FROM scan_results WHERE server_id = $1 AND scan_type = $2 AND status = 'completed' ORDER BY created_at DESC LIMIT 1`, serverID, scanType,
	).Scan(&s.ID, &s.ServerID, &s.ScanType, &s.Status, &s.Score, &s.TotalChecks,
		&s.Passed, &s.Warnings, &s.Criticals, &s.ErrorMessage, &s.StartedAt, &s.CompletedAt, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (r *Repository) GetScanResultByID(ctx context.Context, id string) (*model.ScanResult, error) {
	s := &model.ScanResult{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, server_id, scan_type, status, score, total_checks, passed, warnings, criticals, error_message, started_at, completed_at, created_at
		 FROM scan_results WHERE id = $1`, id,
	).Scan(&s.ID, &s.ServerID, &s.ScanType, &s.Status, &s.Score, &s.TotalChecks,
		&s.Passed, &s.Warnings, &s.Criticals, &s.ErrorMessage, &s.StartedAt, &s.CompletedAt, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// GetScanResultWithFindings returns a scan result with its findings loaded.
func (r *Repository) GetScanResultWithFindings(ctx context.Context, id string) (*model.ScanResult, error) {
	s, err := r.GetScanResultByID(ctx, id)
	if err != nil {
		return nil, err
	}
	findings, err := r.GetFindingsByScanID(ctx, s.ID)
	if err != nil {
		s.Findings = []model.ScanFinding{}
	} else {
		s.Findings = findings
	}
	return s, nil
}

func (r *Repository) ListScanResults(ctx context.Context, serverID string, scanType string, page, limit int) (*model.ScanResultsListResponse, error) {
	// Count
	var total int
	var countQuery string
	var countArgs []interface{}
	countArgs = append(countArgs, serverID)
	if scanType != "" && scanType != "all" {
		countQuery = `SELECT COUNT(*) FROM scan_results WHERE server_id = $1 AND scan_type = $2`
		countArgs = append(countArgs, scanType)
	} else {
		countQuery = `SELECT COUNT(*) FROM scan_results WHERE server_id = $1`
	}
	err := r.db.Pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, err
	}

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	offset := (page - 1) * limit

	var query string
	var queryArgs []interface{}
	queryArgs = append(queryArgs, serverID)
	if scanType != "" && scanType != "all" {
		query = `SELECT id, server_id, scan_type, status, score, total_checks, passed, warnings, criticals, error_message, started_at, completed_at, created_at
			 FROM scan_results WHERE server_id = $1 AND scan_type = $2 ORDER BY created_at DESC LIMIT $3 OFFSET $4`
		queryArgs = append(queryArgs, scanType, limit, offset)
	} else {
		query = `SELECT id, server_id, scan_type, status, score, total_checks, passed, warnings, criticals, error_message, started_at, completed_at, created_at
			 FROM scan_results WHERE server_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
		queryArgs = append(queryArgs, limit, offset)
	}

	rows, err := r.db.Pool.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*model.ScanResult
	for rows.Next() {
		s := &model.ScanResult{}
		if err := rows.Scan(&s.ID, &s.ServerID, &s.ScanType, &s.Status, &s.Score, &s.TotalChecks,
			&s.Passed, &s.Warnings, &s.Criticals, &s.ErrorMessage, &s.StartedAt, &s.CompletedAt, &s.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, s)
	}

	totalPages := (total + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	return &model.ScanResultsListResponse{
		Results:    results,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

func (r *Repository) CreateScanFindings(ctx context.Context, findings []model.ScanFinding) error {
	if len(findings) == 0 {
		return nil
	}
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, f := range findings {
		_, err := tx.Exec(ctx,
			`INSERT INTO scan_findings (id, scan_id, check_id, category, severity, title, description, remediation, raw_output, status, created_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
			f.ID, f.ScanID, f.CheckID, f.Category, f.Severity, f.Title,
			f.Description, f.Remediation, f.RawOutput, f.Status, f.CreatedAt,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *Repository) GetFindingsByScanID(ctx context.Context, scanID string) ([]model.ScanFinding, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, scan_id, check_id, category, severity, title, COALESCE(description,''), COALESCE(remediation,''), status, created_at
		 FROM scan_findings WHERE scan_id = $1 ORDER BY
		    CASE severity
		        WHEN 'critical' THEN 1
		        WHEN 'high' THEN 2
		        WHEN 'medium' THEN 3
		        WHEN 'low' THEN 4
		        ELSE 5
		    END, created_at`, scanID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var findings []model.ScanFinding
	for rows.Next() {
		f := model.ScanFinding{}
		if err := rows.Scan(&f.ID, &f.ScanID, &f.CheckID, &f.Category, &f.Severity,
			&f.Title, &f.Description, &f.Remediation, &f.Status, &f.CreatedAt); err != nil {
			return nil, err
		}
		findings = append(findings, f)
	}
	return findings, nil
}

// GetFindingsByScanIDAndCategory returns findings for a scan filtered by category.
func (r *Repository) GetFindingsByScanIDAndCategory(ctx context.Context, scanID, category string) ([]model.ScanFinding, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, scan_id, check_id, category, severity, title, COALESCE(description,''), COALESCE(remediation,''), status, created_at
		 FROM scan_findings WHERE scan_id = $1 AND category = $2 ORDER BY
		    CASE severity
		        WHEN 'critical' THEN 1
		        WHEN 'high' THEN 2
		        WHEN 'medium' THEN 3
		        WHEN 'low' THEN 4
		        ELSE 5
		    END, created_at`, scanID, category,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var findings []model.ScanFinding
	for rows.Next() {
		f := model.ScanFinding{}
		if err := rows.Scan(&f.ID, &f.ScanID, &f.CheckID, &f.Category, &f.Severity,
			&f.Title, &f.Description, &f.Remediation, &f.Status, &f.CreatedAt); err != nil {
			return nil, err
		}
		findings = append(findings, f)
	}
	return findings, nil
}

// GetCategoryBreakdowns returns per-category summaries for a given scan.
func (r *Repository) GetCategoryBreakdowns(ctx context.Context, scanID string) ([]model.CategoryBreakdown, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT category,
		        COUNT(*) AS total,
		        COUNT(*) FILTER (WHERE status = 'pass') AS passed,
		        COUNT(*) FILTER (WHERE status = 'warn') AS warnings,
		        COUNT(*) FILTER (WHERE status = 'fail') AS criticals
		 FROM scan_findings WHERE scan_id = $1
		 GROUP BY category ORDER BY category`, scanID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var breakdowns []model.CategoryBreakdown
	for rows.Next() {
		b := model.CategoryBreakdown{}
		if err := rows.Scan(&b.Category, &b.Total, &b.Passed, &b.Warnings, &b.Criticals); err != nil {
			return nil, err
		}
		breakdowns = append(breakdowns, b)
	}
	return breakdowns, nil
}

// ─── Container Security Repository ─────────────────────────────────────────

// ContainerSecurityData holds per-container security results from scan findings.
type ContainerSecurityData struct {
	ContainerName string           `json:"container_name"`
	Score         int              `json:"score"`
	Findings      []model.ScanFinding `json:"findings"`
	Badges        []string         `json:"badges"`
	ScannedAt     *time.Time       `json:"scanned_at"`
}

// GetContainerSecurityByServer returns the latest Container Security scan findings
// grouped by container name for a given server.
func (r *Repository) GetContainerSecurityByServer(ctx context.Context, serverID string) (map[string]*ContainerSecurityData, error) {
	result := make(map[string]*ContainerSecurityData)

	// Get latest COMPLETED Container Security scan
	scan, err := r.GetLatestCompletedScanByType(ctx, serverID, "Container Security")
	if err != nil {
		// No scan found — return empty map, not an error
		return result, nil
	}
	if scan == nil || scan.Status != "completed" {
		return result, nil
	}

	// Get all findings for this scan
	findings, err := r.GetFindingsByScanID(ctx, scan.ID)
	if err != nil {
		return result, nil
	}

	// Group findings by category (container name)
	containerFindings := make(map[string][]model.ScanFinding)
	for _, f := range findings {
		name := f.Category
		if name == "" {
			continue
		}
		containerFindings[name] = append(containerFindings[name], f)
	}

	// Compute scores per container
	for name, f := range containerFindings {
		score := 100
		var badges []string
		for _, ff := range f {
			if ff.Status == "fail" {
				switch ff.Severity {
				case "critical":
					score -= 25
				case "high":
					score -= 15
				case "medium":
					score -= 10
				case "low":
					score -= 5
				}
			}
		}
		if score < 0 {
			score = 0
		}

		// Generate badges from findings
		for _, ff := range f {
			if ff.Status == "fail" {
				switch ff.CheckID {
				case "ctr_01":
					badges = append(badges, "🔓 privileged")
				case "ctr_02":
					badges = append(badges, "🔓 root user")
				case "ctr_03":
					badges = append(badges, "🌐 host net")
				case "ctr_04":
					badges = append(badges, "🔓 ports")
				case "ctr_05":
					badges = append(badges, "🛡️ no seccomp")
				case "ctr_06":
					badges = append(badges, "📁 writable")
				case "ctr_07":
					if ff.Severity == "high" || ff.Severity == "critical" {
						badges = append(badges, "🔓 caps")
					}
				case "ctr_08":
					badges = append(badges, "🏥 no healthcheck")
				case "ctr_09":
					badges = append(badges, "📁 bind mounts")
				case "ctr_10":
					badges = append(badges, "💾 no limits")
				}
			} else if ff.Status == "pass" {
				switch ff.CheckID {
				case "ctr_01":
					badges = append(badges, "🔒 unprivileged")
				case "ctr_02":
					badges = append(badges, "📦 non-root")
				case "ctr_05":
					badges = append(badges, "🛡️ seccomp")
				case "ctr_06":
					badges = append(badges, "📁 read-only")
				case "ctr_07":
					badges = append(badges, "🔒 caps dropped")
				case "ctr_10":
					badges = append(badges, "💾 limits")
				}
			}
		}

		// Deduplicate badges
		seen := make(map[string]bool)
		var uniqueBadges []string
		for _, b := range badges {
			if !seen[b] {
				seen[b] = true
				uniqueBadges = append(uniqueBadges, b)
			}
		}

		result[name] = &ContainerSecurityData{
			ContainerName: name,
			Score:         score,
			Findings:      f,
			Badges:        uniqueBadges,
			ScannedAt:     scan.CompletedAt,
		}
	}

	return result, nil
}

// GetContainerScanHistory returns scan history entries for a specific container
// by finding all Container Security scans that have findings with the given category (container name).
func (r *Repository) GetContainerScanHistory(ctx context.Context, serverID, containerName string) ([]model.ContainerScanHistoryItem, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT sr.id, sr.score,
			COUNT(sf.id) AS total,
			COUNT(sf.id) FILTER (WHERE sf.status = 'fail') AS failed,
			COUNT(sf.id) FILTER (WHERE sf.status = 'pass') AS passed,
			COUNT(sf.id) FILTER (WHERE sf.severity = 'critical') AS criticals,
			COUNT(sf.id) FILTER (WHERE sf.severity = 'high') AS high,
			COUNT(sf.id) FILTER (WHERE sf.severity = 'medium') AS medium,
			COUNT(sf.id) FILTER (WHERE sf.severity = 'low') AS low,
			sr.completed_at, sr.created_at
		 FROM scan_results sr
		 INNER JOIN scan_findings sf ON sf.scan_id = sr.id AND sf.category = $2
		 WHERE sr.server_id = $1 AND sr.scan_type = 'Container Security'
		 GROUP BY sr.id
		 ORDER BY sr.created_at DESC
		 LIMIT 20`,
		serverID, containerName,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.ContainerScanHistoryItem
	for rows.Next() {
		var item model.ContainerScanHistoryItem
		if err := rows.Scan(&item.ScanID, &item.Score,
			&item.Total, &item.Failed, &item.Passed,
			&item.Criticals, &item.High, &item.Medium, &item.Low,
			&item.CompletedAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if items == nil {
		items = []model.ContainerScanHistoryItem{}
	}
	return items, nil
}

// GetCategoryHistory returns per-category history across scans for a server.
func (r *Repository) GetCategoryHistory(ctx context.Context, serverID, category string, limit int) (*model.CategoryHistoryResponse, error) {
	if limit < 1 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	// First get scan IDs for this server (most recent first)
	scanRows, err := r.db.Pool.Query(ctx,
		`SELECT id, scan_type, score, completed_at, created_at
		 FROM scan_results WHERE server_id = $1 AND status = 'completed'
		 ORDER BY created_at DESC LIMIT $2`, serverID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer scanRows.Close()

	type scanMeta struct {
		ID          string
		ScanType    string
		Score       *int
		CompletedAt *time.Time
		CreatedAt   time.Time
	}
	var scans []scanMeta
	for scanRows.Next() {
		s := scanMeta{}
		if err := scanRows.Scan(&s.ID, &s.ScanType, &s.Score, &s.CompletedAt, &s.CreatedAt); err != nil {
			return nil, err
		}
		scans = append(scans, s)
	}

	// Aggregate findings by category for each scan
	var items []model.CategoryHistoryItem
	for _, s := range scans {
		var total, passed, warnings, criticals int
		err := r.db.Pool.QueryRow(ctx,
			`SELECT
				COUNT(*),
				COUNT(*) FILTER (WHERE status = 'pass'),
				COUNT(*) FILTER (WHERE status = 'warn'),
				COUNT(*) FILTER (WHERE status = 'fail')
			 FROM scan_findings WHERE scan_id = $1 AND category = $2`,
			s.ID, category,
		).Scan(&total, &passed, &warnings, &criticals)
		if err != nil {
			continue
		}
		if total == 0 {
			continue
		}
		items = append(items, model.CategoryHistoryItem{
			ScanID:      s.ID,
			ServerID:    serverID,
			ScanType:    s.ScanType,
			Total:       total,
			Passed:      passed,
			Warnings:    warnings,
			Criticals:   criticals,
			Score:       s.Score,
			CreatedAt:   s.CreatedAt,
			CompletedAt: s.CompletedAt,
		})
	}

	// Also get server name
	var serverName string
	_ = r.db.Pool.QueryRow(ctx, `SELECT name FROM servers WHERE id = $1`, serverID).Scan(&serverName)
	for i := range items {
		items[i].ServerName = serverName
	}

	return &model.CategoryHistoryResponse{
		Results:  items,
		Total:    len(items),
		Category: category,
	}, nil
}

func (r *Repository) ListGlobalScanHistory(ctx context.Context, scanType string, page, limit int) (*model.GlobalHistoryResponse, error) {
	var total int
	var countQuery string
	var countArgs []interface{}
	if scanType != "" && scanType != "all" {
		if scanType == "CIS Docker" {
			countQuery = `SELECT COUNT(*) FROM scan_results WHERE scan_type IN ($1, $2)`
			countArgs = append(countArgs, "CIS Docker", "Container Security")
		} else {
			countQuery = `SELECT COUNT(*) FROM scan_results WHERE scan_type = $1`
			countArgs = append(countArgs, scanType)
		}
	} else {
		countQuery = `SELECT COUNT(*) FROM scan_results`
	}
	err := r.db.Pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, err
	}

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	offset := (page - 1) * limit

	var query string
	var args []interface{}
	if scanType != "" && scanType != "all" {
		if scanType == "CIS Docker" {
	query = `SELECT sr.id, sr.server_id, COALESCE(s.name,''), COALESCE(s.host,''),
		       sr.scan_type, sr.status, sr.score, sr.total_checks, sr.passed, sr.warnings, sr.criticals,
		       COALESCE(sr.error_message,''), sr.started_at, sr.completed_at, sr.created_at
		 FROM scan_results sr
		 LEFT JOIN servers s ON sr.server_id = s.id
		 WHERE sr.scan_type IN ($1, $2)
		 ORDER BY sr.created_at DESC LIMIT $3 OFFSET $4`
			args = append(args, "CIS Docker", "Container Security", limit, offset)
		} else {
			query = `SELECT sr.id, sr.server_id, COALESCE(s.name,''), COALESCE(s.host,''),
			       sr.scan_type, sr.status, sr.score, sr.total_checks, sr.passed, sr.warnings, sr.criticals,
			       COALESCE(sr.error_message,''), sr.started_at, sr.completed_at, sr.created_at
			 FROM scan_results sr
			 LEFT JOIN servers s ON sr.server_id = s.id
			 WHERE sr.scan_type = $1
			 ORDER BY sr.created_at DESC LIMIT $2 OFFSET $3`
			args = append(args, scanType, limit, offset)
		}
	} else {
		query = `SELECT sr.id, sr.server_id, COALESCE(s.name,''), COALESCE(s.host,''),
		       sr.scan_type, sr.status, sr.score, sr.total_checks, sr.passed, sr.warnings, sr.criticals,
		       COALESCE(sr.error_message,''), sr.started_at, sr.completed_at, sr.created_at
		 FROM scan_results sr
		 LEFT JOIN servers s ON sr.server_id = s.id
		 ORDER BY sr.created_at DESC LIMIT $1 OFFSET $2`
		args = append(args, limit, offset)
	}

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.GlobalScanHistoryItem
	for rows.Next() {
		item := model.GlobalScanHistoryItem{}
		if err := rows.Scan(&item.ID, &item.ServerID, &item.ServerName, &item.ServerHost,
			&item.ScanType, &item.Status, &item.Score, &item.TotalChecks,
			&item.Passed, &item.Warnings, &item.Criticals,
			&item.ErrorMessage, &item.StartedAt, &item.CompletedAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, item)
	}

	totalPages := (total + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	return &model.GlobalHistoryResponse{
		Results:    results,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

func (r *Repository) GetComplianceSummary(ctx context.Context, allowedGroups []string) (*model.ComplianceSummary, error) {
	summary := &model.ComplianceSummary{
		ByStatus: make(map[string]int),
	}

	// Build group filter clause
	var groupFilter string
	var args []interface{}
	argIdx := 1

	if allowedGroups != nil && len(allowedGroups) == 0 {
		// Non-admin with no groups → return empty summary
		summary.TotalServers = 0
		summary.Servers = []model.ComplianceServerSummary{}
		summary.TopFindings = []model.ComplianceTopFinding{}
		return summary, nil
	}
	if len(allowedGroups) > 0 {
		placeholders := make([]string, len(allowedGroups))
		for i, g := range allowedGroups {
			placeholders[i] = fmt.Sprintf("$%d", argIdx)
			args = append(args, g)
			argIdx++
		}
		groupFilter = "WHERE s.server_group IN (" + strings.Join(placeholders, ",") + ") "
	}

	// Total servers
	countQuery := "SELECT COUNT(*) FROM servers s " + groupFilter
	var countArgs []interface{}
	countArgs = append(countArgs, args...)
	err := r.db.Pool.QueryRow(ctx, countQuery, countArgs...).Scan(&summary.TotalServers)
	if err != nil {
		return nil, err
	}
	if summary.TotalServers == 0 {
		summary.Servers = []model.ComplianceServerSummary{}
		summary.TopFindings = []model.ComplianceTopFinding{}
		return summary, nil
	}

	// Latest scan per server
	query := `
		SELECT s.id, s.name, s.host,
		       sr.score, COALESCE(sr.criticals,0), COALESCE(sr.warnings,0), COALESCE(sr.passed,0), sr.completed_at
		FROM servers s
		LEFT JOIN LATERAL (
		    SELECT score, criticals, warnings, passed, completed_at
		    FROM scan_results
		    WHERE server_id = s.id AND status = 'completed'
		    ORDER BY created_at DESC LIMIT 1
		) sr ON true
		` + groupFilter + `
		ORDER BY s.name
	`
	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var totalScore int
	var scoreCount int

	for rows.Next() {
		cs := model.ComplianceServerSummary{}
		if err := rows.Scan(&cs.ID, &cs.Name, &cs.Host, &cs.Score, &cs.Criticals, &cs.Warnings, &cs.Passed, &cs.LastScan); err != nil {
			return nil, err
		}
		summary.Servers = append(summary.Servers, cs)

		if cs.Score != nil {
			scoreCount++
			totalScore += *cs.Score
			if *cs.Score >= 80 {
				summary.ByStatus["good"]++
			} else if *cs.Score >= 60 {
				summary.ByStatus["warning"]++
			} else {
				summary.ByStatus["critical"]++
			}
		} else {
			summary.ByStatus["unscanned"]++
		}
	}

	if scoreCount > 0 {
		avg := totalScore / scoreCount
		summary.AverageScore = &avg
	}
	summary.ScannedServers = scoreCount

	// Top findings across all servers (filtered by group if applicable)
	var topFindingsQuery string
	var topFindingsArgs []interface{}

	if len(allowedGroups) > 0 {
		// Build group placeholders for the subquery
		gp := make([]string, len(allowedGroups))
		gi := 1
		for i, g := range allowedGroups {
			gp[i] = fmt.Sprintf("$%d", gi)
			topFindingsArgs = append(topFindingsArgs, g)
			gi++
		}
		topFindingsQuery = fmt.Sprintf(`
			SELECT f.check_id, f.title, f.severity, COUNT(DISTINCT sr.server_id) as affected
			FROM scan_findings f
			JOIN scan_results sr ON f.scan_id = sr.id
			JOIN servers s ON sr.server_id = s.id
			WHERE f.status = 'fail' AND s.server_group IN (%s) AND sr.id IN (
			    SELECT id FROM (
			        SELECT id, ROW_NUMBER() OVER (PARTITION BY server_id ORDER BY created_at DESC) as rn
			        FROM scan_results WHERE status = 'completed'
			    ) sub WHERE rn = 1
			)
			GROUP BY f.check_id, f.title, f.severity
			ORDER BY affected DESC, CASE f.severity
			    WHEN 'critical' THEN 1 WHEN 'high' THEN 2 WHEN 'medium' THEN 3 ELSE 4 END
			LIMIT 10
		`, strings.Join(gp, ","))
	} else {
		topFindingsQuery = `
			SELECT f.check_id, f.title, f.severity, COUNT(DISTINCT sr.server_id) as affected
			FROM scan_findings f
			JOIN scan_results sr ON f.scan_id = sr.id
			WHERE f.status = 'fail' AND sr.id IN (
			    SELECT id FROM (
			        SELECT id, ROW_NUMBER() OVER (PARTITION BY server_id ORDER BY created_at DESC) as rn
			        FROM scan_results WHERE status = 'completed'
			    ) sub WHERE rn = 1
			)
			GROUP BY f.check_id, f.title, f.severity
			ORDER BY affected DESC, CASE f.severity
			    WHEN 'critical' THEN 1 WHEN 'high' THEN 2 WHEN 'medium' THEN 3 ELSE 4 END
			LIMIT 10
		`
	}
	topRows, err := r.db.Pool.Query(ctx, topFindingsQuery, topFindingsArgs...)
	if err != nil {
		return nil, err
	}
	defer topRows.Close()

	for topRows.Next() {
		tf := model.ComplianceTopFinding{}
		if err := topRows.Scan(&tf.CheckID, &tf.Title, &tf.Severity, &tf.ServersAffected); err != nil {
			return nil, err
		}
		summary.TopFindings = append(summary.TopFindings, tf)
	}
	if summary.TopFindings == nil {
		summary.TopFindings = []model.ComplianceTopFinding{}
	}

	return summary, nil
}

// ListActiveScans returns all running scans and recently completed scans
// (within the last 5 minutes) that the user has access to.
func (r *Repository) ListActiveScans(ctx context.Context, allowedGroups []string, isAdmin bool) (*model.ActiveScansResponse, error) {
	resp := &model.ActiveScansResponse{
		Running: []model.ActiveScanItem{},
		Recent:  []model.ActiveScanItem{},
	}

	// Build the server filter clause
	var serverFilter string
	var args []interface{}
	argIdx := 1

	if !isAdmin {
		serverFilter = fmt.Sprintf("s.server_group = ANY($%d::text[])", argIdx)
		args = append(args, allowedGroups)
		argIdx++
	}

	// Running scans
	runningQuery := fmt.Sprintf(`
		SELECT sr.id, sr.server_id, COALESCE(s.name,''), COALESCE(s.host,''),
		       sr.scan_type, sr.status, sr.score, sr.total_checks, sr.passed, sr.warnings, sr.criticals,
		       sr.started_at, sr.completed_at, sr.created_at
		FROM scan_results sr
		JOIN servers s ON sr.server_id = s.id
		WHERE sr.status = 'running'
		%s
		ORDER BY sr.created_at DESC
	`, func() string {
		if serverFilter != "" {
			return "AND " + serverFilter
		}
		return ""
	}())

	rows, err := r.db.Pool.Query(ctx, runningQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		item := model.ActiveScanItem{}
		if err := rows.Scan(&item.ID, &item.ServerID, &item.ServerName, &item.ServerHost,
			&item.ScanType, &item.Status, &item.Score, &item.TotalChecks,
			&item.Passed, &item.Warnings, &item.Criticals,
			&item.StartedAt, &item.CompletedAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		resp.Running = append(resp.Running, item)
	}

	// Recent scans (completed/failed within last 5 minutes)
	sinceFilter := fmt.Sprintf(" AND sr.completed_at >= NOW() - interval '5 minutes'")
	recentQuery := fmt.Sprintf(`
		SELECT sr.id, sr.server_id, COALESCE(s.name,''), COALESCE(s.host,''),
		       sr.scan_type, sr.status, sr.score, sr.total_checks, sr.passed, sr.warnings, sr.criticals,
		       sr.started_at, sr.completed_at, sr.created_at
		FROM scan_results sr
		JOIN servers s ON sr.server_id = s.id
		WHERE sr.status IN ('completed', 'failed')
		%s
		%s
		ORDER BY sr.completed_at DESC
		LIMIT 20
	`, func() string {
		if serverFilter != "" {
			return "AND " + serverFilter
		}
		return ""
	}(), sinceFilter)

	recentRows, err := r.db.Pool.Query(ctx, recentQuery, args...)
	if err != nil {
		return nil, err
	}
	defer recentRows.Close()
	for recentRows.Next() {
		item := model.ActiveScanItem{}
		if err := recentRows.Scan(&item.ID, &item.ServerID, &item.ServerName, &item.ServerHost,
			&item.ScanType, &item.Status, &item.Score, &item.TotalChecks,
			&item.Passed, &item.Warnings, &item.Criticals,
			&item.StartedAt, &item.CompletedAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		resp.Recent = append(resp.Recent, item)
	}

	return resp, nil
}

// ─── Audit Log Export ──────────────────────────────────────────────────

func (r *Repository) ListAuditLogsAll(ctx context.Context, q model.AuditLogQuery) ([]*model.AuditLogEntry, error) {
	// Same query as ListAuditLogs but without pagination limit/offset
	var conditions []string
	var args []interface{}
	argIdx := 1

	if q.Action != "" {
		conditions = append(conditions, fmt.Sprintf("a.action = $%d", argIdx))
		args = append(args, q.Action)
		argIdx++
	}
	if q.EntityType != "" {
		conditions = append(conditions, fmt.Sprintf("a.entity_type = $%d", argIdx))
		args = append(args, q.EntityType)
		argIdx++
	}
	if q.UserID != "" {
		conditions = append(conditions, fmt.Sprintf("a.user_id::text = $%d", argIdx))
		args = append(args, q.UserID)
		argIdx++
	}
	if q.Search != "" {
		conditions = append(conditions, fmt.Sprintf("(a.description ILIKE $%d OR a.user_email ILIKE $%d OR a.action ILIKE $%d)", argIdx, argIdx, argIdx))
		args = append(args, "%"+q.Search+"%")
		argIdx++
	}
	if q.StartDate != nil && *q.StartDate != "" {
		conditions = append(conditions, fmt.Sprintf("a.created_at >= $%d::timestamptz", argIdx))
		args = append(args, *q.StartDate)
		argIdx++
	}
	if q.EndDate != nil && *q.EndDate != "" {
		conditions = append(conditions, fmt.Sprintf("a.created_at <= $%d::timestamptz + interval '1 day'", argIdx))
		args = append(args, *q.EndDate)
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	dataQuery := fmt.Sprintf(
		`SELECT a.id, a.action, a.entity_type, COALESCE(a.entity_id, ''), a.description,
		 COALESCE(a.user_id::text, ''), COALESCE(a.user_email, ''), COALESCE(a.ip_address, ''),
		 COALESCE(a.metadata, '{}'), a.created_at
		 FROM audit_logs a %s ORDER BY a.created_at DESC`,
		whereClause,
	)

	rows, err := r.db.Pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*model.AuditLogEntry
	for rows.Next() {
		e := &model.AuditLogEntry{}
		if err := rows.Scan(&e.ID, &e.Action, &e.EntityType, &e.EntityID,
			&e.Description, &e.UserID, &e.UserEmail, &e.IPAddress,
			&e.Metadata, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if entries == nil {
		entries = []*model.AuditLogEntry{}
	}
	return entries, nil
}

// ─── Registry User Management ─────────────────────────────────────────────

func (r *Repository) GetRegistryUserByAnjunganUserID(ctx context.Context, anjunganUserID string) (*model.RegistryUser, error) {
	u := &model.RegistryUser{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, username, password_hash, role, COALESCE(anjungan_user_id, ''), created_at, updated_at FROM registry_users WHERE anjungan_user_id = $1`,
		anjunganUserID,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.AnjunganUserID, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *Repository) ListRegistryUsers(ctx context.Context) ([]*model.RegistryUser, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, username, password_hash, role, COALESCE(anjungan_user_id, ''), created_at, updated_at FROM registry_users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*model.RegistryUser
	for rows.Next() {
		u := &model.RegistryUser{}
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.AnjunganUserID, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	if users == nil {
		users = []*model.RegistryUser{}
	}
	return users, nil
}

func (r *Repository) GetRegistryUserByUsername(ctx context.Context, username string) (*model.RegistryUser, error) {
	u := &model.RegistryUser{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, username, password_hash, role, COALESCE(anjungan_user_id, ''), created_at, updated_at FROM registry_users WHERE username = $1`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.AnjunganUserID, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *Repository) GetRegistryUserByID(ctx context.Context, id string) (*model.RegistryUser, error) {
	u := &model.RegistryUser{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, username, password_hash, role, COALESCE(anjungan_user_id, ''), created_at, updated_at FROM registry_users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.AnjunganUserID, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *Repository) CreateRegistryUser(ctx context.Context, u *model.RegistryUser) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO registry_users (id, username, password_hash, role, anjungan_user_id, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		u.ID, u.Username, u.PasswordHash, u.Role, u.AnjunganUserID, u.CreatedAt, u.UpdatedAt,
	)
	return err
}

func (r *Repository) UpdateRegistryUser(ctx context.Context, u *model.RegistryUser) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE registry_users SET username=$1, role=$2, updated_at=NOW() WHERE id=$3`,
		u.Username, u.Role, u.ID,
	)
	return err
}

func (r *Repository) UpdateRegistryUserPassword(ctx context.Context, id, passwordHash string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE registry_users SET password_hash=$1, updated_at=NOW() WHERE id=$2`,
		passwordHash, id,
	)
	return err
}

func (r *Repository) DeleteRegistryUser(ctx context.Context, id string) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM registry_users WHERE id = $1`, id)
	return err
}

// ─── Settings ─────────────────────────────────────────────────────────────────

func (r *Repository) GetSetting(ctx context.Context, key string) (*model.Settings, error) {
	s := &model.Settings{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT key, value, description, created_at, updated_at FROM settings WHERE key = $1`, key).
		Scan(&s.Key, &s.Value, &s.Description, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (r *Repository) GetComplianceThresholds(ctx context.Context) (model.ComplianceThresholds, error) {
	def := model.DefaultComplianceThresholds()
	s, err := r.GetSetting(ctx, "compliance_thresholds")
	if err != nil {
		return def, nil
	}
	var t model.ComplianceThresholds
	if err := json.Unmarshal([]byte(s.Value), &t); err != nil {
		return def, nil
	}
	if t.Compliant <= 0 || t.Warning <= 0 || t.Compliant <= t.Warning {
		return def, nil
	}
	return t, nil
}

func (r *Repository) UpsertSetting(ctx context.Context, key, value, description string) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO settings (key, value, description) VALUES ($1, $2, $3)
		 ON CONFLICT (key) DO UPDATE SET value = $2, description = $3, updated_at = NOW()`,
		key, value, description)
	return err
}

// ─── Registry Webhook CRUD ──────────────────────────────────────────────────

func (r *Repository) ListRegistryWebhooks(ctx context.Context) ([]*model.RegistryWebhook, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, name, url, platform, events, enabled, created_at, updated_at
		 FROM registry_webhooks ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hooks []*model.RegistryWebhook
	for rows.Next() {
		h := &model.RegistryWebhook{}
		if err := rows.Scan(&h.ID, &h.Name, &h.URL, &h.Platform, &h.Events, &h.Enabled, &h.CreatedAt, &h.UpdatedAt); err != nil {
			return nil, err
		}
		hooks = append(hooks, h)
	}
	return hooks, nil
}

func (r *Repository) GetRegistryWebhook(ctx context.Context, id string) (*model.RegistryWebhook, error) {
	h := &model.RegistryWebhook{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, name, url, platform, events, enabled, created_at, updated_at
		 FROM registry_webhooks WHERE id = $1`, id).
		Scan(&h.ID, &h.Name, &h.URL, &h.Platform, &h.Events, &h.Enabled, &h.CreatedAt, &h.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return h, nil
}

func (r *Repository) CreateRegistryWebhook(ctx context.Context, h *model.RegistryWebhook) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO registry_webhooks (id, name, url, platform, events, enabled, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		h.ID, h.Name, h.URL, h.Platform, h.Events, h.Enabled, h.CreatedAt, h.UpdatedAt)
	return err
}

func (r *Repository) UpdateRegistryWebhook(ctx context.Context, h *model.RegistryWebhook) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE registry_webhooks SET name=$1, url=$2, platform=$3, events=$4, enabled=$5, updated_at=NOW()
		 WHERE id=$6`,
		h.Name, h.URL, h.Platform, h.Events, h.Enabled, h.ID)
	return err
}

func (r *Repository) DeleteRegistryWebhook(ctx context.Context, id string) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM registry_webhooks WHERE id = $1`, id)
	return err
}

// ─── Registry Webhook Events ────────────────────────────────────────────────

func (r *Repository) CreateRegistryWebhookEvent(ctx context.Context, e *model.RegistryWebhookEvent) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO registry_webhook_events (id, webhook_id, event_type, repo, tag, digest, actor, description, payload, status, status_code, response, created_at, delivered_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		e.ID, e.WebhookID, e.EventType, e.Repo, e.Tag, e.Digest, e.Actor, e.Description, e.Payload, e.Status, e.StatusCode, e.Response, e.CreatedAt, e.DeliveredAt)
	return err
}

func (r *Repository) ListRegistryWebhookEvents(ctx context.Context, limit, offset int) ([]*model.RegistryWebhookEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, COALESCE(webhook_id, ''), event_type, repo, tag, digest, actor, description, payload, status, status_code, response, created_at, delivered_at
		 FROM registry_webhook_events ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*model.RegistryWebhookEvent
	for rows.Next() {
		e := &model.RegistryWebhookEvent{}
		if err := rows.Scan(&e.ID, &e.WebhookID, &e.EventType, &e.Repo, &e.Tag, &e.Digest, &e.Actor, &e.Description, &e.Payload, &e.Status, &e.StatusCode, &e.Response, &e.CreatedAt, &e.DeliveredAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

func (r *Repository) CountRegistryWebhookEvents(ctx context.Context) (int, error) {
	var count int
	err := r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM registry_webhook_events`).Scan(&count)
	return count, err
}

func (r *Repository) UpdateRegistryWebhookEventDelivery(ctx context.Context, id, status string, statusCode int, response string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE registry_webhook_events SET status=$1, status_code=$2, response=$3, delivered_at=NOW()
		 WHERE id=$4`, status, statusCode, response, id)
	return err
}

// ─── Enabled webhooks for dispatch ──────────────────────────────────────────
func (r *Repository) ListEnabledRegistryWebhooks(ctx context.Context) ([]*model.RegistryWebhook, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, name, url, platform, events, enabled, created_at, updated_at
		 FROM registry_webhooks WHERE enabled = TRUE ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hooks []*model.RegistryWebhook
	for rows.Next() {
		h := &model.RegistryWebhook{}
		if err := rows.Scan(&h.ID, &h.Name, &h.URL, &h.Platform, &h.Events, &h.Enabled, &h.CreatedAt, &h.UpdatedAt); err != nil {
			return nil, err
		}
		hooks = append(hooks, h)
	}
	return hooks, nil
}

func (r *Repository) ListRegistryWebhooksByIDs(ctx context.Context, ids []string) ([]*model.RegistryWebhook, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, name, url, platform, events, enabled, created_at, updated_at
		 FROM registry_webhooks WHERE id IN (`+strings.Join(placeholders, ",")+`) AND enabled = TRUE`,
		args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hooks []*model.RegistryWebhook
	for rows.Next() {
		h := &model.RegistryWebhook{}
		if err := rows.Scan(&h.ID, &h.Name, &h.URL, &h.Platform, &h.Events, &h.Enabled, &h.CreatedAt, &h.UpdatedAt); err != nil {
			return nil, err
		}
		hooks = append(hooks, h)
	}
	return hooks, nil
}

// ─── Registry Tag Protection ────────────────────────────────────────────────

func (r *Repository) ListTagProtections(ctx context.Context, repo string) ([]*model.RegistryTagProtection, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, repo, tag, created_by, created_at
		 FROM registry_tag_protections WHERE repo = $1 ORDER BY tag`, repo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var protections []*model.RegistryTagProtection
	for rows.Next() {
		p := &model.RegistryTagProtection{}
		if err := rows.Scan(&p.ID, &p.Repo, &p.Tag, &p.CreatedBy, &p.CreatedAt); err != nil {
			return nil, err
		}
		protections = append(protections, p)
	}
	return protections, nil
}

func (r *Repository) ListAllTagProtections(ctx context.Context) ([]*model.RegistryTagProtection, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, repo, tag, created_by, created_at
		 FROM registry_tag_protections ORDER BY repo, tag`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var protections []*model.RegistryTagProtection
	for rows.Next() {
		p := &model.RegistryTagProtection{}
		if err := rows.Scan(&p.ID, &p.Repo, &p.Tag, &p.CreatedBy, &p.CreatedAt); err != nil {
			return nil, err
		}
		protections = append(protections, p)
	}
	return protections, nil
}

func (r *Repository) IsTagProtected(ctx context.Context, repo, tag string) (bool, error) {
	var count int
	err := r.db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM registry_tag_protections WHERE repo = $1 AND tag = $2`,
		repo, tag).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repository) CreateTagProtection(ctx context.Context, p *model.RegistryTagProtection) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO registry_tag_protections (id, repo, tag, created_by, created_at)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (repo, tag) DO NOTHING`,
		p.ID, p.Repo, p.Tag, p.CreatedBy, p.CreatedAt)
	return err
}

func (r *Repository) DeleteTagProtection(ctx context.Context, id string) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM registry_tag_protections WHERE id = $1`, id)
	return err
}

func (r *Repository) DeleteTagProtectionByRepoTag(ctx context.Context, repo, tag string) error {
	_, err := r.db.Pool.Exec(ctx,
		`DELETE FROM registry_tag_protections WHERE repo = $1 AND tag = $2`, repo, tag)
	return err
}

// ─── SSL Monitor Repository ───────────────────────────────────────────────────

const sslMonitorColumns = `id, domain, port, COALESCE(display_name, ''), COALESCE(check_interval, '1h'),
	COALESCE(notify_before, '14d'), COALESCE(webhook_ids, '{}'),
	COALESCE(last_status, 'pending'), last_check_at, COALESCE(last_error, ''),
	COALESCE(issuer, ''), COALESCE(subject, ''), cert_expires_at,
	COALESCE(days_remaining, 0), chain_valid, COALESCE(chain_error, ''),
	COALESCE(cipher_grade, ''), COALESCE(cipher_error, ''),
	COALESCE(ocsp_status, ''), COALESCE(ocsp_error, ''),
	COALESCE(san_names, '{}'), COALESCE(san_mismatch, false),
	COALESCE(created_by, ''), COALESCE(enabled, true),
	COALESCE(server_id::text, ''), COALESCE(source_provider, 'manual'),
	created_at, updated_at`

func scanSSLMonitor(scanner interface {
	Scan(dest ...interface{}) error
}) (*model.SSLMonitor, error) {
	m := &model.SSLMonitor{}
	err := scanner.Scan(
		&m.ID, &m.Domain, &m.Port, &m.DisplayName, &m.CheckInterval,
		&m.NotifyBefore, &m.WebhookIDs,
		&m.LastStatus, &m.LastCheckAt, &m.LastError,
		&m.Issuer, &m.Subject, &m.CertExpiresAt,
		&m.DaysRemaining, &m.ChainValid, &m.ChainError,
		&m.CipherGrade, &m.CipherError,
		&m.OCSPStatus, &m.OCSPError,
		&m.SANNames, &m.SANMismatch,
		&m.CreatedBy, &m.Enabled,
		&m.ServerID, &m.SourceProvider,
		&m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (r *Repository) CreateSSLMonitor(ctx context.Context, m *model.SSLMonitor) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO ssl_monitors (id, domain, port, display_name, check_interval, notify_before,
		 webhook_ids, last_status, created_by, enabled, server_id, source_provider, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		m.ID, m.Domain, m.Port, m.DisplayName, m.CheckInterval, m.NotifyBefore,
		m.WebhookIDs, "pending", m.CreatedBy, m.Enabled, m.ServerID, m.SourceProvider, m.CreatedAt, m.UpdatedAt)
	return err
}

func (r *Repository) ListSSLMonitors(ctx context.Context, search string, status string, enabledOnly bool) ([]*model.SSLMonitor, error) {
	var conditions []string
	var args []interface{}
	argIdx := 1

	if search != "" {
		conditions = append(conditions, fmt.Sprintf("(LOWER(domain) LIKE $%d OR LOWER(COALESCE(display_name, '')) LIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+strings.ToLower(search)+"%")
		argIdx++
	}
	if status != "" {
		conditions = append(conditions, fmt.Sprintf("last_status = $%d", argIdx))
		args = append(args, status)
		argIdx++
	}
	if enabledOnly {
		conditions = append(conditions, "enabled = TRUE")
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	rows, err := r.db.Pool.Query(ctx,
		`SELECT `+sslMonitorColumns+` FROM ssl_monitors `+whereClause+` ORDER BY domain`,
		args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var monitors []*model.SSLMonitor
	for rows.Next() {
		m, err := scanSSLMonitor(rows)
		if err != nil {
			return nil, err
		}
		monitors = append(monitors, m)
	}
	return monitors, nil
}

func (r *Repository) ListSSLMonitorsPaginated(ctx context.Context, page, limit int, search, status, sort, order string, enabledOnly bool) (*model.SSLMonitorListResponse, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	var conditions []string
	var args []interface{}
	argIdx := 1

	if search != "" {
		conditions = append(conditions, fmt.Sprintf("(LOWER(domain) LIKE $%d OR LOWER(COALESCE(display_name, '')) LIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+strings.ToLower(search)+"%")
		argIdx++
	}
	if status != "" {
		conditions = append(conditions, fmt.Sprintf("last_status = $%d", argIdx))
		args = append(args, status)
		argIdx++
	}
	if enabledOnly {
		conditions = append(conditions, "enabled = TRUE")
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count
	var total int
	countQuery := "SELECT COUNT(*) FROM ssl_monitors " + whereClause
	if err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, err
	}

	// Sort
	allowedSorts := map[string]string{
		"domain":        "domain",
		"last_status":   "last_status",
		"days_remaining": "days_remaining",
		"created_at":    "created_at",
	}
	sortCol, ok := allowedSorts[sort]
	if !ok {
		sortCol = "domain"
	}
	orderDir := "ASC"
	if strings.EqualFold(order, "desc") {
		orderDir = "DESC"
	}

	offset := (page - 1) * limit

	dataQuery := fmt.Sprintf(
		`SELECT `+sslMonitorColumns+` FROM ssl_monitors %s ORDER BY %s %s LIMIT $%d OFFSET $%d`,
		whereClause, sortCol, orderDir, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.db.Pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var monitors []model.SSLMonitorResponse
	for rows.Next() {
		m, err := scanSSLMonitor(rows)
		if err != nil {
			return nil, err
		}
		monitors = append(monitors, m.ToResponse())
	}
	if monitors == nil {
		monitors = []model.SSLMonitorResponse{}
	}

	totalPages := (total + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	return &model.SSLMonitorListResponse{
		Monitors:   monitors,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

func (r *Repository) GetSSLMonitor(ctx context.Context, id string) (*model.SSLMonitor, error) {
	row := r.db.Pool.QueryRow(ctx,
		`SELECT `+sslMonitorColumns+` FROM ssl_monitors WHERE id = $1`, id)
	return scanSSLMonitor(row)
}

func (r *Repository) UpdateSSLMonitor(ctx context.Context, m *model.SSLMonitor) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE ssl_monitors SET domain=$1, port=$2, display_name=$3, check_interval=$4,
		 notify_before=$5, webhook_ids=$6, enabled=$7, server_id=NULLIF($8,'')::uuid, source_provider=$9, updated_at=NOW() WHERE id=$10`,
		m.Domain, m.Port, m.DisplayName, m.CheckInterval, m.NotifyBefore,
		m.WebhookIDs, m.Enabled, m.ServerID, m.SourceProvider, m.ID)
	return err
}

// UpdateSSLMonitorCheckResult updates only the TLS check result fields (no updated_at change —
// the check engine sets its own timestamp via last_check_at)
func (r *Repository) UpdateSSLMonitorCheckResult(ctx context.Context, m *model.SSLMonitor) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE ssl_monitors SET last_status=$1, last_check_at=$2, last_error=$3,
		 issuer=$4, subject=$5, cert_expires_at=$6, days_remaining=$7,
		 chain_valid=$8, chain_error=$9, cipher_grade=$10, cipher_error=$11,
		 ocsp_status=$12, ocsp_error=$13, san_names=$14, san_mismatch=$15,
		 updated_at=NOW()
		 WHERE id=$16`,
		m.LastStatus, m.LastCheckAt, m.LastError,
		m.Issuer, m.Subject, m.CertExpiresAt, m.DaysRemaining,
		m.ChainValid, m.ChainError, m.CipherGrade, m.CipherError,
		m.OCSPStatus, m.OCSPError, m.SANNames, m.SANMismatch,
		m.ID)
	return err
}

func (r *Repository) DeleteSSLMonitor(ctx context.Context, id string) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM ssl_monitors WHERE id = $1`, id)
	return err
}

func (r *Repository) CountSSLMonitorsByStatus(ctx context.Context) (map[string]int, error) {
	rows, err := r.db.Pool.Query(ctx,
		"SELECT last_status, COUNT(*) FROM ssl_monitors GROUP BY last_status")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		result[status] = count
	}
	return result, nil
}

func (r *Repository) ListEnabledSSLMonitors(ctx context.Context) ([]*model.SSLMonitor, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT `+sslMonitorColumns+` FROM ssl_monitors WHERE enabled = TRUE ORDER BY domain`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var monitors []*model.SSLMonitor
	for rows.Next() {
		m, err := scanSSLMonitor(rows)
		if err != nil {
			return nil, err
		}
		monitors = append(monitors, m)
	}
	return monitors, nil
}

func (r *Repository) GetSSLMonitorByDomainPort(ctx context.Context, domain string, port int) (*model.SSLMonitor, error) {
	row := r.db.Pool.QueryRow(ctx,
		`SELECT `+sslMonitorColumns+` FROM ssl_monitors WHERE domain = $1 AND port = $2`,
		domain, port)
	return scanSSLMonitor(row)
}

// ─── SSL Check History Repository ──────────────────────────────────────────

const sslCheckHistoryColumns = `id, ssl_monitor_id, checked_at, status, days_remaining,
	COALESCE(cipher_grade, ''), COALESCE(tls_version, ''), COALESCE(cipher_suite, ''),
	response_time_ms, COALESCE(issuer, ''), COALESCE(subject, ''), COALESCE(error_message, '')`

func scanSSLCheckHistory(scanner interface {
	Scan(dest ...interface{}) error
}) (*model.SSLCheckHistory, error) {
	h := &model.SSLCheckHistory{}
	err := scanner.Scan(
		&h.ID, &h.SSLMonitorID, &h.CheckedAt, &h.Status, &h.DaysRemaining,
		&h.CipherGrade, &h.TLSVersion, &h.CipherSuite,
		&h.ResponseTimeMs, &h.Issuer, &h.Subject, &h.ErrorMessage,
	)
	if err != nil {
		return nil, err
	}
	return h, nil
}

func (r *Repository) CreateSSLCheckHistory(ctx context.Context, h *model.SSLCheckHistory) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO ssl_check_history (id, ssl_monitor_id, checked_at, status, days_remaining,
		 cipher_grade, tls_version, cipher_suite, response_time_ms, issuer, subject, error_message)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		h.ID, h.SSLMonitorID, h.CheckedAt, h.Status, h.DaysRemaining,
		h.CipherGrade, h.TLSVersion, h.CipherSuite, h.ResponseTimeMs,
		h.Issuer, h.Subject, h.ErrorMessage,
	)
	return err
}

func (r *Repository) ListSSLCheckHistory(ctx context.Context, sslMonitorID string, limit, offset int) (*model.SSLCheckHistoryListResponse, error) {
	var total int
	err := r.db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM ssl_check_history WHERE ssl_monitor_id = $1`, sslMonitorID,
	).Scan(&total)
	if err != nil {
		return nil, err
	}

	if limit < 1 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	rows, err := r.db.Pool.Query(ctx,
		`SELECT `+sslCheckHistoryColumns+` FROM ssl_check_history
		 WHERE ssl_monitor_id = $1 ORDER BY checked_at DESC LIMIT $2 OFFSET $3`,
		sslMonitorID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []model.SSLCheckHistory
	for rows.Next() {
		h, err := scanSSLCheckHistory(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, *h)
	}
	if entries == nil {
		entries = []model.SSLCheckHistory{}
	}

	return &model.SSLCheckHistoryListResponse{
		Entries: entries,
		Total:   total,
		Limit:   limit,
	}, nil
}

// GetSSLMonitorTrend returns time-series history entries for trend chart (chronological order).
func (r *Repository) GetSSLMonitorTrend(ctx context.Context, monitorID string, limit int) ([]model.SSLCheckHistory, error) {
	if limit < 1 || limit > 365 {
		limit = 90
	}
	rows, err := r.db.Pool.Query(ctx,
		`SELECT `+sslCheckHistoryColumns+` 
		 FROM ssl_check_history 
		 WHERE ssl_monitor_id = $1 
		 ORDER BY checked_at ASC LIMIT $2`,
		monitorID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []model.SSLCheckHistory
	for rows.Next() {
		e, err := scanSSLCheckHistory(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, *e)
	}
	if entries == nil {
		entries = []model.SSLCheckHistory{}
	}
	return entries, nil
}

// PurgeSSLCheckHistory deletes entries older than the given age (e.g. 90 days).
func (r *Repository) PurgeSSLCheckHistory(ctx context.Context, olderThan time.Duration) (int, error) {
	res, err := r.db.Pool.Exec(ctx,
		`DELETE FROM ssl_check_history WHERE checked_at < NOW() - $1::interval`,
		fmt.Sprintf("%.0f seconds", olderThan.Seconds()),
	)
	if err != nil {
		return 0, err
	}
	return int(res.RowsAffected()), nil
}

// ListDueSSLMonitors returns monitors where the next check is due.
func (r *Repository) ListDueSSLMonitors(ctx context.Context) ([]*model.SSLMonitor, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT `+sslMonitorColumns+` FROM ssl_monitors
		 WHERE enabled = TRUE
		 AND (
		   last_check_at IS NULL
		   OR last_check_at <= NOW() - (
		     CASE
		       WHEN check_interval = '30m' THEN INTERVAL '30 minutes'
		       WHEN check_interval = '1h'  THEN INTERVAL '1 hour'
		       WHEN check_interval = '6h'  THEN INTERVAL '6 hours'
		       WHEN check_interval = '12h' THEN INTERVAL '12 hours'
		       WHEN check_interval = '24h' THEN INTERVAL '24 hours'
		       ELSE INTERVAL '1 hour'
		     END
		   )
		 )
		 ORDER BY last_check_at ASC NULLS FIRST`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var monitors []*model.SSLMonitor
	for rows.Next() {
		m, err := scanSSLMonitor(rows)
		if err != nil {
			return nil, err
		}
		monitors = append(monitors, m)
	}
	return monitors, nil
}

// ─── Uptime Monitors CRUD ───────────────────────────────────────────────────

// ListUptimeMonitors returns paginated uptime monitors with optional filters.
func (r *Repository) ListUptimeMonitors(ctx context.Context, page, limit int, status, search, sort, order string) ([]model.UptimeMonitor, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	var conditions []string
	var args []interface{}
	argIdx := 1

	if search != "" {
		conditions = append(conditions, fmt.Sprintf("(LOWER(name) LIKE $%d OR LOWER(url) LIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+strings.ToLower(search)+"%")
		argIdx++
	}
	if status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, status)
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count
	var total int
	countQuery := "SELECT COUNT(*) FROM uptime_monitors " + whereClause
	if err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Sort
	allowedSorts := map[string]string{
		"name":                  "name",
		"status":                "status",
		"last_check_at":         "last_check_at",
		"last_response_time_ms": "last_response_time_ms",
	}
	sortCol, ok := allowedSorts[sort]
	if !ok {
		sortCol = "created_at"
	}
	orderDir := "DESC"
	if strings.EqualFold(order, "asc") {
		orderDir = "ASC"
	}

	offset := (page - 1) * limit

	dataQuery := fmt.Sprintf(
		`SELECT id, name, url, check_type, interval_seconds, timeout_seconds,
		 expected_status_min, expected_status_max, expected_body, enabled,
		 COALESCE(notification_target_ids, '{}'), status, last_status,
		 last_status_code, last_response_time_ms, COALESCE(last_error, ''),
		 last_check_at, COALESCE(created_by, ''), created_at, updated_at
		 FROM uptime_monitors %s ORDER BY %s %s LIMIT $%d OFFSET $%d`,
		whereClause, sortCol, orderDir, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.db.Pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []model.UptimeMonitor
	for rows.Next() {
		var m model.UptimeMonitor
		err := rows.Scan(
			&m.ID, &m.Name, &m.URL, &m.CheckType, &m.IntervalSeconds, &m.TimeoutSeconds,
			&m.ExpectedStatusMin, &m.ExpectedStatusMax, &m.ExpectedBody, &m.Enabled,
			&m.NotificationTargetIDs, &m.Status, &m.LastStatus,
			&m.LastStatusCode, &m.LastResponseTimeMs, &m.LastError,
			&m.LastCheckAt, &m.CreatedBy, &m.CreatedAt, &m.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, m)
	}
	if items == nil {
		items = []model.UptimeMonitor{}
	}

	return items, total, nil
}

// GetUptimeMonitor returns a single uptime monitor by ID.
func (r *Repository) GetUptimeMonitor(ctx context.Context, id string) (*model.UptimeMonitor, error) {
	m := &model.UptimeMonitor{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, name, url, check_type, interval_seconds, timeout_seconds,
		 expected_status_min, expected_status_max, expected_body, enabled,
		 COALESCE(notification_target_ids, '{}'), status, last_status,
		 last_status_code, last_response_time_ms, COALESCE(last_error, ''),
		 last_check_at, COALESCE(created_by, ''), created_at, updated_at
		 FROM uptime_monitors WHERE id = $1`, id).
		Scan(&m.ID, &m.Name, &m.URL, &m.CheckType, &m.IntervalSeconds, &m.TimeoutSeconds,
			&m.ExpectedStatusMin, &m.ExpectedStatusMax, &m.ExpectedBody, &m.Enabled,
			&m.NotificationTargetIDs, &m.Status, &m.LastStatus,
			&m.LastStatusCode, &m.LastResponseTimeMs, &m.LastError,
			&m.LastCheckAt, &m.CreatedBy, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return m, nil
}

// CreateUptimeMonitor inserts a new uptime monitor.
func (r *Repository) CreateUptimeMonitor(ctx context.Context, m *model.UptimeMonitor) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO uptime_monitors (id, name, url, check_type, interval_seconds, timeout_seconds,
		 expected_status_min, expected_status_max, expected_body, enabled,
		 notification_target_ids, status, last_status, created_by, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		m.ID, m.Name, m.URL, m.CheckType, m.IntervalSeconds, m.TimeoutSeconds,
		m.ExpectedStatusMin, m.ExpectedStatusMax, m.ExpectedBody, m.Enabled,
		m.NotificationTargetIDs, m.Status, m.LastStatus, m.CreatedBy, m.CreatedAt, m.UpdatedAt)
	return err
}

// UpdateUptimeMonitor updates an existing uptime monitor.
func (r *Repository) UpdateUptimeMonitor(ctx context.Context, m *model.UptimeMonitor) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE uptime_monitors SET name=$1, url=$2, check_type=$3, interval_seconds=$4,
		 timeout_seconds=$5, expected_status_min=$6, expected_status_max=$7,
		 expected_body=$8, enabled=$9, notification_target_ids=$10, updated_at=NOW()
		 WHERE id=$11`,
		m.Name, m.URL, m.CheckType, m.IntervalSeconds, m.TimeoutSeconds,
		m.ExpectedStatusMin, m.ExpectedStatusMax, m.ExpectedBody, m.Enabled,
		m.NotificationTargetIDs, m.ID)
	return err
}

// DeleteUptimeMonitor deletes an uptime monitor by ID.
func (r *Repository) DeleteUptimeMonitor(ctx context.Context, id string) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM uptime_monitors WHERE id = $1`, id)
	return err
}

// ListEnabledUptimeMonitors returns all enabled uptime monitors (for scheduler use).
func (r *Repository) ListEnabledUptimeMonitors(ctx context.Context) ([]model.UptimeMonitor, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, name, url, check_type, interval_seconds, timeout_seconds,
		 expected_status_min, expected_status_max, expected_body, enabled,
		 COALESCE(notification_target_ids, '{}'), status, last_status,
		 last_status_code, last_response_time_ms, COALESCE(last_error, ''),
		 last_check_at, COALESCE(created_by, ''), created_at, updated_at
		 FROM uptime_monitors WHERE enabled = true`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.UptimeMonitor
	for rows.Next() {
		var m model.UptimeMonitor
		err := rows.Scan(
			&m.ID, &m.Name, &m.URL, &m.CheckType, &m.IntervalSeconds, &m.TimeoutSeconds,
			&m.ExpectedStatusMin, &m.ExpectedStatusMax, &m.ExpectedBody, &m.Enabled,
			&m.NotificationTargetIDs, &m.Status, &m.LastStatus,
			&m.LastStatusCode, &m.LastResponseTimeMs, &m.LastError,
			&m.LastCheckAt, &m.CreatedBy, &m.CreatedAt, &m.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	return items, nil
}

// GetUptimeSummary returns aggregated counts of uptime monitors grouped by status.
func (r *Repository) GetUptimeSummary(ctx context.Context) (*model.UptimeSummary, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT status, COUNT(*) FROM uptime_monitors GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summary := &model.UptimeSummary{}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		switch status {
		case "up":
			summary.Up = count
		case "down":
			summary.Down = count
		case "paused":
			summary.Paused = count
		}
		summary.Total += count
	}
	return summary, nil
}

// UpdateUptimeMonitorStatus updates the latest check result fields for a monitor.
func (r *Repository) UpdateUptimeMonitorStatus(ctx context.Context, id, status string, statusCode *int, responseTimeMs *int, errMsg string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE uptime_monitors SET status=$1, last_status=status, last_status_code=$2,
		 last_response_time_ms=$3, last_error=$4, last_check_at=NOW()
		 WHERE id=$5`,
		status, statusCode, responseTimeMs, errMsg, id)
	return err
}

// PauseUptimeMonitor disables and pauses an uptime monitor.
func (r *Repository) PauseUptimeMonitor(ctx context.Context, id string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE uptime_monitors SET enabled=false, status='paused' WHERE id=$1`, id)
	return err
}

// ResumeUptimeMonitor enables and sets status to pending for an uptime monitor.
func (r *Repository) ResumeUptimeMonitor(ctx context.Context, id string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE uptime_monitors SET enabled=true, status='pending' WHERE id=$1`, id)
	return err
}

// ─── Uptime Check History ───────────────────────────────────────────────────

// CreateUptimeCheckHistory inserts a new check history record.
func (r *Repository) CreateUptimeCheckHistory(ctx context.Context, h *model.UptimeCheckHistory) error {
	if h.ID == "" {
		h.ID = uuid.New().String()
	}
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO uptime_check_history (id, monitor_id, checked_at, status, status_code,
		 response_time_ms, error_message)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		h.ID, h.MonitorID, h.CheckedAt, h.Status, h.StatusCode, h.ResponseTimeMs, h.ErrorMessage)
	return err
}

// ListUptimeCheckHistory returns paginated check history for a specific monitor.
func (r *Repository) ListUptimeCheckHistory(ctx context.Context, monitorID string, limit, offset int) ([]model.UptimeCheckHistory, error) {
	if limit < 1 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, monitor_id, checked_at, status, status_code, response_time_ms, COALESCE(error_message, '')
		 FROM uptime_check_history
		 WHERE monitor_id = $1 ORDER BY checked_at DESC LIMIT $2 OFFSET $3`,
		monitorID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.UptimeCheckHistory
	for rows.Next() {
		var h model.UptimeCheckHistory
		if err := rows.Scan(&h.ID, &h.MonitorID, &h.CheckedAt, &h.Status, &h.StatusCode, &h.ResponseTimeMs, &h.ErrorMessage); err != nil {
			return nil, err
		}
		items = append(items, h)
	}
	if items == nil {
		items = []model.UptimeCheckHistory{}
	}
	return items, nil
}

// GetUptimeTrend returns time-series check history for trend charts.
func (r *Repository) GetUptimeTrend(ctx context.Context, monitorID, period string) ([]model.UptimeCheckHistory, error) {
	var rows interface{ Next() bool; Scan(dest ...interface{}) error; Close() }

	switch period {
	case "24h":
		r, err := r.db.Pool.Query(ctx,
			`SELECT id, monitor_id, checked_at, status, status_code, response_time_ms, COALESCE(error_message, '')
			 FROM uptime_check_history
			 WHERE monitor_id = $1 AND checked_at > NOW() - INTERVAL '24 hours'
			 ORDER BY checked_at ASC`,
			monitorID)
		if err != nil {
			return nil, err
		}
		rows = r
	case "7d":
		r, err := r.db.Pool.Query(ctx,
			`SELECT monitor_id, date, total_checks, up_count, down_count,
			 COALESCE(avg_response_ms, 0), COALESCE(min_response_ms, 0), COALESCE(max_response_ms, 0),
			 COALESCE(uptime_percent, 0)
			 FROM uptime_daily_summary
			 WHERE monitor_id = $1 AND date > NOW() - INTERVAL '7 days'
			 ORDER BY date ASC`,
			monitorID)
		if err != nil {
			return nil, err
		}
		rows = r
	case "3d":
		r, err := r.db.Pool.Query(ctx,
			`SELECT monitor_id, date, total_checks, up_count, down_count,
			 COALESCE(avg_response_ms, 0), COALESCE(min_response_ms, 0), COALESCE(max_response_ms, 0),
			 COALESCE(uptime_percent, 0)
			 FROM uptime_daily_summary
			 WHERE monitor_id = $1 AND date > NOW() - INTERVAL '3 days'
			 ORDER BY date ASC`,
			monitorID)
		if err != nil {
			return nil, err
		}
		rows = r
	case "30d":
		r, err := r.db.Pool.Query(ctx,
			`SELECT monitor_id, date, total_checks, up_count, down_count,
			 COALESCE(avg_response_ms, 0), COALESCE(min_response_ms, 0), COALESCE(max_response_ms, 0),
			 COALESCE(uptime_percent, 0)
			 FROM uptime_daily_summary
			 WHERE monitor_id = $1 AND date > NOW() - INTERVAL '30 days'
			 ORDER BY date ASC`,
			monitorID)
		if err != nil {
			return nil, err
		}
		rows = r
	default:
		r, err := r.db.Pool.Query(ctx,
			`SELECT id, monitor_id, checked_at, status, status_code, response_time_ms, COALESCE(error_message, '')
			 FROM uptime_check_history
			 WHERE monitor_id = $1 AND checked_at > NOW() - INTERVAL '24 hours'
			 ORDER BY checked_at ASC`,
			monitorID)
		if err != nil {
			return nil, err
		}
		rows = r
	}
	defer rows.Close()

	var items []model.UptimeCheckHistory
	for rows.Next() {
		var h model.UptimeCheckHistory
		// For daily summary queries, the columns differ from check_history;
		// we map the summary fields into the history struct.
		if period == "3d" || period == "7d" || period == "30d" {
			var totalChecks, upCount, downCount, avgMs, minMs, maxMs int
			var uptimePct float64
			if err := rows.Scan(&h.MonitorID, &h.CheckedAt, &totalChecks, &upCount, &downCount, &avgMs, &minMs, &maxMs, &uptimePct); err != nil {
				return nil, err
			}
			// Map daily summary data into check history fields for the trend
			h.ID = fmt.Sprintf("summary-%s-%s", h.MonitorID, h.CheckedAt.Format("2006-01-02"))
			h.Status = "summary"
			statusCode := uptimePct
			h.ResponseTimeMs = &avgMs
			h.ErrorMessage = fmt.Sprintf("up=%d down=%d total=%d uptime=%.1f%%", upCount, downCount, totalChecks, uptimePct*100)
			_ = statusCode
		} else {
			if err := rows.Scan(&h.ID, &h.MonitorID, &h.CheckedAt, &h.Status, &h.StatusCode, &h.ResponseTimeMs, &h.ErrorMessage); err != nil {
				return nil, err
			}
		}
		items = append(items, h)
	}
	if items == nil {
		items = []model.UptimeCheckHistory{}
	}
	return items, nil
}

// GetUptimeTrendCustom returns time-series check history for a custom date range.
func (r *Repository) GetUptimeTrendCustom(ctx context.Context, monitorID, from, to string) ([]model.UptimeCheckHistory, error) {
	fromTime, err := time.Parse(time.RFC3339, from)
	if err != nil {
		fromTime, err = time.Parse("2006-01-02", from)
		if err != nil {
			return nil, fmt.Errorf("invalid from date: %w", err)
		}
	}

	toTime := time.Now()
	if to != "" {
		toTime, err = time.Parse(time.RFC3339, to)
		if err != nil {
			toTime, err = time.Parse("2006-01-02", to)
			if err != nil {
				return nil, fmt.Errorf("invalid to date: %w", err)
			}
			toTime = toTime.Add(24 * time.Hour) // include full day
		}
	}

	// Use daily_summary for custom ranges
	rows, err := r.db.Pool.Query(ctx,
		`SELECT monitor_id, date, total_checks, up_count, down_count,
		 COALESCE(avg_response_ms, 0), COALESCE(min_response_ms, 0), COALESCE(max_response_ms, 0),
		 COALESCE(uptime_percent, 0)
		 FROM uptime_daily_summary
		 WHERE monitor_id = $1 AND date >= $2 AND date <= $3
		 ORDER BY date ASC`,
		monitorID, fromTime, toTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.UptimeCheckHistory
	for rows.Next() {
		var h model.UptimeCheckHistory
		var totalChecks, upCount, downCount, avgMs, minMs, maxMs int
		var uptimePct float64
		if err := rows.Scan(&h.MonitorID, &h.CheckedAt, &totalChecks, &upCount, &downCount, &avgMs, &minMs, &maxMs, &uptimePct); err != nil {
			return nil, err
		}
		h.ID = fmt.Sprintf("custom-%s-%s", h.MonitorID, h.CheckedAt.Format("2006-01-02"))
		h.Status = "summary"
		h.ResponseTimeMs = &avgMs
		h.ErrorMessage = fmt.Sprintf("up=%d down=%d total=%d uptime=%.1f%%", upCount, downCount, totalChecks, uptimePct*100)
		items = append(items, h)
	}
	if items == nil {
		items = []model.UptimeCheckHistory{}
	}
	return items, nil
}

// PurgeOldUptimeHistory deletes check history entries older than the given retention period.
func (r *Repository) PurgeOldUptimeHistory(ctx context.Context, retentionDays int) error {
	_, err := r.db.Pool.Exec(ctx,
		`DELETE FROM uptime_check_history WHERE checked_at < NOW() - $1::interval`,
		fmt.Sprintf("%d days", retentionDays))
	return err
}

// UpsertUptimeDailySummary inserts or updates a daily summary record.
func (r *Repository) UpsertUptimeDailySummary(ctx context.Context, s *model.UptimeDailySummary) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO uptime_daily_summary (monitor_id, date, total_checks, up_count, down_count,
		 avg_response_ms, min_response_ms, max_response_ms, uptime_percent)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 ON CONFLICT (monitor_id, date) DO UPDATE SET
		 total_checks=$3, up_count=$4, down_count=$5, avg_response_ms=$6,
		 min_response_ms=$7, max_response_ms=$8, uptime_percent=$9`,
		s.MonitorID, s.Date, s.TotalChecks, s.UpCount, s.DownCount,
		s.AvgResponseMs, s.MinResponseMs, s.MaxResponseMs, s.UptimePercent)
	return err
}

// ─── Uptime Maintenance Windows ────────────────────────────────────────────

// CreateUptimeMaintenance inserts a new maintenance window.
func (r *Repository) CreateUptimeMaintenance(ctx context.Context, mw *model.UptimeMaintenanceWindow) error {
	if mw.ID == "" {
		mw.ID = uuid.New().String()
	}
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO uptime_maintenance_windows (id, monitor_id, description, starts_at, ends_at, created_by, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		mw.ID, mw.MonitorID, mw.Description, mw.StartsAt, mw.EndsAt, mw.CreatedBy, mw.CreatedAt, mw.UpdatedAt)
	return err
}

// GetUptimeMaintenance returns a single maintenance window by ID.
func (r *Repository) GetUptimeMaintenance(ctx context.Context, id string) (*model.UptimeMaintenanceWindow, error) {
	mw := &model.UptimeMaintenanceWindow{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, monitor_id, description, starts_at, ends_at, COALESCE(created_by, ''), created_at, updated_at
		 FROM uptime_maintenance_windows WHERE id = $1`, id).
		Scan(&mw.ID, &mw.MonitorID, &mw.Description, &mw.StartsAt, &mw.EndsAt, &mw.CreatedBy, &mw.CreatedAt, &mw.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return mw, nil
}

// DeleteUptimeMaintenance deletes a maintenance window by ID.
func (r *Repository) DeleteUptimeMaintenance(ctx context.Context, id string) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM uptime_maintenance_windows WHERE id = $1`, id)
	return err
}

// ListUptimeMaintenanceWindows returns all maintenance windows for a monitor, ordered by starts_at DESC.
func (r *Repository) ListUptimeMaintenanceWindows(ctx context.Context, monitorID string) ([]model.UptimeMaintenanceWindow, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, monitor_id, description, starts_at, ends_at, COALESCE(created_by, ''), created_at, updated_at
		 FROM uptime_maintenance_windows
		 WHERE monitor_id = $1
		 ORDER BY starts_at DESC`, monitorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.UptimeMaintenanceWindow
	for rows.Next() {
		var mw model.UptimeMaintenanceWindow
		if err := rows.Scan(&mw.ID, &mw.MonitorID, &mw.Description, &mw.StartsAt, &mw.EndsAt, &mw.CreatedBy, &mw.CreatedAt, &mw.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, mw)
	}
	if items == nil {
		items = []model.UptimeMaintenanceWindow{}
	}
	return items, nil
}

// ListActiveMaintenanceWindows returns maintenance windows for a monitor that are currently active (starts_at <= now AND ends_at >= now).
func (r *Repository) ListActiveMaintenanceWindows(ctx context.Context, monitorID string) ([]model.UptimeMaintenanceWindow, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, monitor_id, description, starts_at, ends_at, COALESCE(created_by, ''), created_at, updated_at
		 FROM uptime_maintenance_windows
		 WHERE monitor_id = $1 AND starts_at <= NOW() AND ends_at >= NOW()
		 ORDER BY starts_at DESC`, monitorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.UptimeMaintenanceWindow
	for rows.Next() {
		var mw model.UptimeMaintenanceWindow
		if err := rows.Scan(&mw.ID, &mw.MonitorID, &mw.Description, &mw.StartsAt, &mw.EndsAt, &mw.CreatedBy, &mw.CreatedAt, &mw.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, mw)
	}
	if items == nil {
		items = []model.UptimeMaintenanceWindow{}
	}
	return items, nil
}

// ─── Notification Targets (shared — SSL + Uptime) ──────────────────────────

// ListNotificationTargets returns enabled notification targets.
func (r *Repository) ListNotificationTargets(ctx context.Context) ([]model.NotificationTarget, error) {
	query := `SELECT id, name, url, platform, COALESCE(webhook_secret, ''), COALESCE(bot_token, ''), COALESCE(chat_id, ''), enabled,
		 COALESCE(created_by, ''), created_at, updated_at
		 FROM notification_targets WHERE enabled=true ORDER BY name`

	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.NotificationTarget
	for rows.Next() {
		var t model.NotificationTarget
		if err := rows.Scan(&t.ID, &t.Name, &t.URL, &t.Platform, &t.WebhookSecret, &t.BotToken, &t.ChatID, &t.Enabled, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	if items == nil {
		items = []model.NotificationTarget{}
	}
	return items, nil
}

// GetNotificationTarget returns a notification target by ID.
func (r *Repository) GetNotificationTarget(ctx context.Context, id string) (*model.NotificationTarget, error) {
	t := &model.NotificationTarget{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, name, url, platform, COALESCE(webhook_secret, ''), COALESCE(bot_token, ''), COALESCE(chat_id, ''), enabled,
		 COALESCE(created_by, ''), created_at, updated_at
		 FROM notification_targets WHERE id = $1`, id).
		Scan(&t.ID, &t.Name, &t.URL, &t.Platform, &t.WebhookSecret, &t.BotToken, &t.ChatID, &t.Enabled, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return t, nil
}

// CreateNotificationTarget inserts a new notification target.
func (r *Repository) CreateNotificationTarget(ctx context.Context, t *model.NotificationTarget) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO notification_targets (id, name, url, platform, webhook_secret, bot_token, chat_id, enabled, created_by, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		t.ID, t.Name, t.URL, t.Platform, t.WebhookSecret, t.BotToken, t.ChatID, t.Enabled, t.CreatedBy, t.CreatedAt, t.UpdatedAt)
	return err
}

// UpdateNotificationTarget updates an existing notification target.
func (r *Repository) UpdateNotificationTarget(ctx context.Context, t *model.NotificationTarget) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE notification_targets SET name=$1, url=$2, platform=$3, webhook_secret=$4,
		 bot_token=$5, chat_id=$6, enabled=$7, updated_at=NOW() WHERE id=$8`,
		t.Name, t.URL, t.Platform, t.WebhookSecret, t.BotToken, t.ChatID, t.Enabled, t.ID)
	return err
}

// DeleteNotificationTarget deletes a notification target by ID.
func (r *Repository) DeleteNotificationTarget(ctx context.Context, id string) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM notification_targets WHERE id = $1`, id)
	return err
}

// UptimeStats contains computed uptime statistics for a monitor.
type UptimeStats struct {
	UptimeOverall *float64 `json:"uptime_overall"`
	Uptime24h     *float64 `json:"uptime_24h"`
	Uptime3d      *float64 `json:"uptime_3d"`
	Uptime7d      *float64 `json:"uptime_7d"`
	Uptime30d     *float64 `json:"uptime_30d"`
	TotalChecks  int      `json:"total_checks"`
	UpChecks     int      `json:"up_checks"`
	DownChecks   int      `json:"down_checks"`
}

// ResponseTimeStats holds computed response time statistics for a period.
type ResponseTimeStats struct {
	MinResponseMs *float64 `json:"min_response_ms"`
	MaxResponseMs *float64 `json:"max_response_ms"`
	AvgResponseMs *float64 `json:"avg_response_ms"`
	P95ResponseMs *float64 `json:"p95_response_ms"`
}

// PerPeriodResponseTimeStats holds response time stats per time period.
type PerPeriodResponseTimeStats struct {
	Period24h *ResponseTimeStats `json:"period_24h"`
	Period7d  *ResponseTimeStats `json:"period_7d"`
	Period30d *ResponseTimeStats `json:"period_30d"`
}

// GetUptimeStats computes uptime statistics for a monitor over 24h, 7d, and 30d periods.
func (r *Repository) GetUptimeStats(ctx context.Context, monitorID string) (*UptimeStats, error) {
	stats := &UptimeStats{}

	// 24h — from check_history raw data
	var total24, up24, down24 int
	err := r.db.Pool.QueryRow(ctx,
		`SELECT COUNT(*),
		        COALESCE(SUM(CASE WHEN status = 'up' THEN 1 ELSE 0 END), 0),
		        COALESCE(SUM(CASE WHEN status = 'down' THEN 1 ELSE 0 END), 0)
		 FROM uptime_check_history
		 WHERE monitor_id = $1 AND checked_at > NOW() - INTERVAL '24 hours'`,
		monitorID).Scan(&total24, &up24, &down24)
	if err != nil {
		return nil, err
	}
	if total24 > 0 {
		pct := float64(up24) / float64(total24) * 100
		// Round to 1 decimal
		pct = float64(int(pct*10)) / 10
		stats.Uptime24h = &pct
	}
	stats.TotalChecks += total24
	stats.UpChecks += up24
	stats.DownChecks += down24

	// 7d — from daily_summary
	var total7, up7, down7 int
	err = r.db.Pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(total_checks), 0),
		        COALESCE(SUM(up_count), 0),
		        COALESCE(SUM(down_count), 0)
		 FROM uptime_daily_summary
		 WHERE monitor_id = $1 AND date > NOW() - INTERVAL '7 days'`,
		monitorID).Scan(&total7, &up7, &down7)
	if err == nil && total7 > 0 {
		pct := float64(up7) / float64(total7) * 100
		pct = float64(int(pct*10)) / 10
		stats.Uptime7d = &pct
	}

	// 3d — from daily_summary
	var total3, up3, down3 int
	err = r.db.Pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(total_checks), 0),
		        COALESCE(SUM(up_count), 0),
		        COALESCE(SUM(down_count), 0)
		 FROM uptime_daily_summary
		 WHERE monitor_id = $1 AND date > NOW() - INTERVAL '3 days'`,
		monitorID).Scan(&total3, &up3, &down3)
	if err == nil && total3 > 0 {
		pct := float64(up3) / float64(total3) * 100
		pct = float64(int(pct*10)) / 10
		stats.Uptime3d = &pct
	}

	// 30d — from daily_summary
	var total30, up30, down30 int
	err = r.db.Pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(total_checks), 0),
		        COALESCE(SUM(up_count), 0),
		        COALESCE(SUM(down_count), 0)
		 FROM uptime_daily_summary
		 WHERE monitor_id = $1 AND date > NOW() - INTERVAL '30 days'`,
		monitorID).Scan(&total30, &up30, &down30)
	if err == nil && total30 > 0 {
		pct := float64(up30) / float64(total30) * 100
		pct = float64(int(pct*10)) / 10
		stats.Uptime30d = &pct
	}

	// Overall — all time from daily_summary
	var totalAll, upAll, downAll int
	err = r.db.Pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(total_checks), 0),
		        COALESCE(SUM(up_count), 0),
		        COALESCE(SUM(down_count), 0)
		 FROM uptime_daily_summary
		 WHERE monitor_id = $1`, monitorID).Scan(&totalAll, &upAll, &downAll)
	// Fallback to raw check_history if daily_summary has no data
	if err != nil || totalAll == 0 {
		err = r.db.Pool.QueryRow(ctx,
			`SELECT COUNT(*),
			        COALESCE(SUM(CASE WHEN status = 'up' THEN 1 ELSE 0 END), 0),
			        COALESCE(SUM(CASE WHEN status = 'down' THEN 1 ELSE 0 END), 0)
			 FROM uptime_check_history
			 WHERE monitor_id = $1`, monitorID).Scan(&totalAll, &upAll, &downAll)
	}
	if err == nil && totalAll > 0 {
		pct := float64(upAll) / float64(totalAll) * 100
		pct = float64(int(pct*10)) / 10
		stats.UptimeOverall = &pct
		stats.TotalChecks = totalAll
		stats.UpChecks = upAll
		stats.DownChecks = downAll
	}

	return stats, nil
}

// GetUptimeResponseTimeStats computes response time statistics (min, max, avg, p95) per period.
func (r *Repository) GetUptimeResponseTimeStats(ctx context.Context, monitorID string) (*PerPeriodResponseTimeStats, error) {
	result := &PerPeriodResponseTimeStats{}

	// 24h — from raw check_history
	var min24, max24, avg24, p9524 float64
	err := r.db.Pool.QueryRow(ctx,
		`SELECT
			COALESCE(MIN(response_time_ms), 0)::float8,
			COALESCE(MAX(response_time_ms), 0)::float8,
			COALESCE(AVG(response_time_ms), 0)::float8,
			COALESCE(
				(SELECT PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY response_time_ms)
				 FROM uptime_check_history
				 WHERE monitor_id = $1 AND checked_at > NOW() - INTERVAL '24 hours' AND response_time_ms IS NOT NULL),
				0)::float8
		 FROM uptime_check_history
		 WHERE monitor_id = $1 AND checked_at > NOW() - INTERVAL '24 hours' AND response_time_ms IS NOT NULL`,
		monitorID).Scan(&min24, &max24, &avg24, &p9524)
	if err == nil && min24 > 0 {
		result.Period24h = &ResponseTimeStats{
			MinResponseMs: &min24,
			MaxResponseMs: &max24,
			AvgResponseMs: &avg24,
			P95ResponseMs: &p9524,
		}
	}

	// 7d — from daily_summary (min of mins, max of maxes, weighted avg)
	var min7, max7, avg7 float64
	var totalChecks7 int
	err = r.db.Pool.QueryRow(ctx,
		`SELECT
			COALESCE(MIN(min_response_ms), 0)::float8,
			COALESCE(MAX(max_response_ms), 0)::float8,
			CASE WHEN SUM(total_checks) > 0 THEN SUM(avg_response_ms * total_checks)::float8 / SUM(total_checks)::float8 ELSE 0 END,
			COALESCE(SUM(total_checks), 0)
		 FROM uptime_daily_summary
		 WHERE monitor_id = $1 AND date > NOW() - INTERVAL '7 days'`,
		monitorID).Scan(&min7, &max7, &avg7, &totalChecks7)
	if err == nil && totalChecks7 > 0 {
		result.Period7d = &ResponseTimeStats{
			MinResponseMs: &min7,
			MaxResponseMs: &max7,
			AvgResponseMs: &avg7,
		}
	}

	// 30d — from daily_summary
	var min30, max30, avg30 float64
	var totalChecks30 int
	err = r.db.Pool.QueryRow(ctx,
		`SELECT
			COALESCE(MIN(min_response_ms), 0)::float8,
			COALESCE(MAX(max_response_ms), 0)::float8,
			CASE WHEN SUM(total_checks) > 0 THEN SUM(avg_response_ms * total_checks)::float8 / SUM(total_checks)::float8 ELSE 0 END,
			COALESCE(SUM(total_checks), 0)
		 FROM uptime_daily_summary
		 WHERE monitor_id = $1 AND date > NOW() - INTERVAL '30 days'`,
		monitorID).Scan(&min30, &max30, &avg30, &totalChecks30)
	if err == nil && totalChecks30 > 0 {
		result.Period30d = &ResponseTimeStats{
			MinResponseMs: &min30,
			MaxResponseMs: &max30,
			AvgResponseMs: &avg30,
		}
	}

	return result, nil
}

// GetUptimeIncidents groups consecutive down/error checks into incidents.
func (r *Repository) GetUptimeIncidents(ctx context.Context, monitorID string, limit, offset int) ([]UptimeIncident, int, error) {
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	// Fetch all checks ordered by time (ascending) to group incidents
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, checked_at, status, status_code, COALESCE(error_message, ''), response_time_ms
		 FROM uptime_check_history
		 WHERE monitor_id = $1
		 ORDER BY checked_at ASC`,
		monitorID)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	type checkRow struct {
		id           string
		checkedAt    time.Time
		status       string
		statusCode   *int
		errorMessage string
		responseMs   *int
	}

	var allChecks []checkRow
	for rows.Next() {
		var c checkRow
		if err := rows.Scan(&c.id, &c.checkedAt, &c.status, &c.statusCode, &c.errorMessage, &c.responseMs); err != nil {
			return nil, 0, err
		}
		allChecks = append(allChecks, c)
	}

	// Group consecutive failures into incidents
	type incidentBlock struct {
		status      string
		startAt     time.Time
		endAt       time.Time
		errorMsg    string
		count       int
	}

	var blocks []incidentBlock
	var current *incidentBlock

	isFailure := func(s string) bool {
		return s == "down" || s == "error"
	}

	for _, c := range allChecks {
		if !isFailure(c.status) {
			current = nil
			continue
		}

		if current == nil {
			current = &incidentBlock{
				status:   c.status,
				startAt:  c.checkedAt,
				endAt:    c.checkedAt,
				errorMsg: c.errorMessage,
				count:    1,
			}
		} else {
			current.endAt = c.checkedAt
			current.count++
			if c.errorMessage != "" && current.errorMsg == "" {
				current.errorMsg = c.errorMessage
			}
		}
	}

	// Close last block if still open
	if current != nil {
		blocks = append(blocks, *current)
	}

	// Reverse to show newest first
	for i, j := 0, len(blocks)-1; i < j; i, j = i+1, j-1 {
		blocks[i], blocks[j] = blocks[j], blocks[i]
	}

	total := len(blocks)

	// Paginate
	if offset >= len(blocks) {
		return []UptimeIncident{}, total, nil
	}
	end := offset + limit
	if end > len(blocks) {
		end = len(blocks)
	}
	paged := blocks[offset:end]

	incidents := make([]UptimeIncident, len(paged))
	for i, b := range paged {
		durationSec := int(b.endAt.Sub(b.startAt).Seconds())
		incidents[i] = UptimeIncident{
			ID:            fmt.Sprintf("inc-%s-%d", monitorID, b.startAt.UnixMilli()),
			MonitorID:     monitorID,
			Status:        b.status,
			StartedAt:     b.startAt,
			EndedAt:       b.endAt,
			DurationSec:   durationSec,
			FailureCount:  b.count,
			ErrorMessage:  b.errorMsg,
		}
	}

	return incidents, total, nil
}

// UptimeIncident represents a grouped incident from consecutive check failures.
type UptimeIncident struct {
	ID           string    `json:"id"`
	MonitorID    string    `json:"monitor_id"`
	Status       string    `json:"status"`
	StartedAt    time.Time `json:"started_at"`
	EndedAt      time.Time `json:"ended_at"`
	DurationSec  int       `json:"duration_sec"`
	FailureCount int       `json:"failure_count"`
	ErrorMessage string    `json:"error_message"`
}

// ─── Bookmarks ────────────────────────────────────────────────────────────────

func (r *Repository) ListBookmarks(ctx context.Context) ([]model.Bookmark, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, user_id, title, url, icon_type, icon_value, category, description, pinned, sort_order, created_at, updated_at
		 FROM bookmarks ORDER BY pinned DESC, sort_order ASC, created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookmarks []model.Bookmark
	for rows.Next() {
		var b model.Bookmark
		if err := rows.Scan(&b.ID, &b.UserID, &b.Title, &b.URL, &b.IconType, &b.IconValue, &b.Category, &b.Description, &b.Pinned, &b.SortOrder, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		bookmarks = append(bookmarks, b)
	}
	if bookmarks == nil {
		bookmarks = []model.Bookmark{}
	}
	return bookmarks, rows.Err()
}

func (r *Repository) GetBookmark(ctx context.Context, id string) (*model.Bookmark, error) {
	b := &model.Bookmark{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, user_id, title, url, icon_type, icon_value, category, description, pinned, sort_order, created_at, updated_at
		 FROM bookmarks WHERE id = $1`, id,
	).Scan(&b.ID, &b.UserID, &b.Title, &b.URL, &b.IconType, &b.IconValue, &b.Category, &b.Description, &b.Pinned, &b.SortOrder, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (r *Repository) CreateBookmark(ctx context.Context, b *model.Bookmark) error {
	b.ID = uuid.New().String()
	now := time.Now()
	b.CreatedAt = now
	b.UpdatedAt = now
	if b.IconType == "" {
		b.IconType = "auto"
	}
	if b.Category == "" {
		b.Category = "Other"
	}
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO bookmarks (id, user_id, title, url, icon_type, icon_value, category, description, pinned, sort_order, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		b.ID, b.UserID, b.Title, b.URL, b.IconType, b.IconValue, b.Category, b.Description, b.Pinned, b.SortOrder, b.CreatedAt, b.UpdatedAt,
	)
	return err
}

func (r *Repository) UpdateBookmark(ctx context.Context, b *model.Bookmark) error {
	b.UpdatedAt = time.Now()
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE bookmarks SET title=$1, url=$2, icon_type=$3, icon_value=$4, category=$5, description=$6, pinned=$7, sort_order=$8, updated_at=$9
		 WHERE id=$10`,
		b.Title, b.URL, b.IconType, b.IconValue, b.Category, b.Description, b.Pinned, b.SortOrder, b.UpdatedAt, b.ID,
	)
	return err
}

func (r *Repository) DeleteBookmark(ctx context.Context, id string) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM bookmarks WHERE id = $1`, id)
	return err
}

func (r *Repository) ReorderBookmarks(ctx context.Context, items []model.BookmarkReorderItem) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, item := range items {
		if _, err := tx.Exec(ctx, `UPDATE bookmarks SET sort_order=$1, updated_at=NOW() WHERE id=$2`, item.SortOrder, item.ID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// ─── Auth Events ──────────────────────────────────────────────────────────────

func (r *Repository) CreateAuthEvent(ctx context.Context, e *model.AuthEvent) error {
	var userID *string
	if e.UserID != "" {
		userID = &e.UserID
	}
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO auth_events (id, user_id, email, event_type, status, failure_reason, ip_address, user_agent, country, asn, isp, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		e.ID, userID, e.Email, e.EventType, e.Status, e.FailureReason,
		e.IPAddress, e.UserAgent, e.Country, e.ASN, e.ISP, e.CreatedAt,
	)
	return err
}

func (r *Repository) ListAuthEvents(ctx context.Context, q model.AuthEventQuery) (*model.AuthEventListResponse, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	argIdx := 1

	if q.EventType != "" {
		where = append(where, fmt.Sprintf("event_type = $%d", argIdx))
		args = append(args, q.EventType)
		argIdx++
	}
	if q.Status != "" {
		where = append(where, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, q.Status)
		argIdx++
	}
	if q.UserID != "" {
		where = append(where, fmt.Sprintf("user_id = $%d", argIdx))
		args = append(args, q.UserID)
		argIdx++
	}
	if q.Email != "" {
		where = append(where, fmt.Sprintf("email ILIKE $%d", argIdx))
		args = append(args, "%"+q.Email+"%")
		argIdx++
	}
	if q.IPAddress != "" {
		where = append(where, fmt.Sprintf("ip_address = $%d", argIdx))
		args = append(args, q.IPAddress)
		argIdx++
	}
	if q.Search != "" {
		where = append(where, fmt.Sprintf("(email ILIKE $%d OR ip_address ILIKE $%d OR failure_reason ILIKE $%d)", argIdx, argIdx+1, argIdx+2))
		search := "%" + q.Search + "%"
		args = append(args, search, search, search)
		argIdx += 3
	}
	if q.StartDate != nil && *q.StartDate != "" {
		where = append(where, fmt.Sprintf("created_at >= $%d", argIdx))
		args = append(args, *q.StartDate)
		argIdx++
	}
	if q.EndDate != nil && *q.EndDate != "" {
		where = append(where, fmt.Sprintf("created_at <= $%d", argIdx))
		args = append(args, *q.EndDate)
		argIdx++
	}

	whereClause := strings.Join(where, " AND ")

	// Count total
	var total int
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM auth_events WHERE %s`, whereClause)
	if err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, err
	}

	// Sort & order
	sort := "created_at"
	order := "DESC"
	if q.Sort != "" {
		validSorts := map[string]bool{"created_at": true, "email": true, "event_type": true, "status": true, "ip_address": true}
		if validSorts[q.Sort] {
			sort = q.Sort
		}
	}
	if q.Order == "asc" {
		order = "ASC"
	}

	// Pagination
	limit := 50
	if q.Limit > 0 && q.Limit <= 200 {
		limit = q.Limit
	}
	page := 1
	if q.Page > 0 {
		page = q.Page
	}
	offset := (page - 1) * limit

	query := fmt.Sprintf(
		`SELECT id, COALESCE(user_id::TEXT,''), email, event_type, status, COALESCE(failure_reason,''), ip_address, user_agent, country, asn, isp, created_at
		 FROM auth_events WHERE %s ORDER BY %s %s LIMIT $%d OFFSET $%d`,
		whereClause, sort, order, argIdx, argIdx+1,
	)
	fullArgs := append(args, limit, offset)

	rows, err := r.db.Pool.Query(ctx, query, fullArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*model.AuthEvent
	for rows.Next() {
		e := &model.AuthEvent{}
		if err := rows.Scan(&e.ID, &e.UserID, &e.Email, &e.EventType, &e.Status, &e.FailureReason,
			&e.IPAddress, &e.UserAgent, &e.Country, &e.ASN, &e.ISP, &e.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}

	totalPages := (total + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	return &model.AuthEventListResponse{
		Events:     events,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

func (r *Repository) GetAuthEventSummary(ctx context.Context) (*model.AuthEventSummary, error) {
	s := &model.AuthEventSummary{}

	r.db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM auth_events WHERE event_type = 'login_success' AND created_at >= CURRENT_DATE`,
	).Scan(&s.LoginsToday)

	r.db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM auth_events WHERE status = 'failure' AND created_at >= CURRENT_DATE`,
	).Scan(&s.FailedToday)

	r.db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM auth_events WHERE event_type = 'lockout' AND created_at >= CURRENT_DATE`,
	).Scan(&s.LockedToday)

	r.db.Pool.QueryRow(ctx,
		`SELECT COUNT(DISTINCT ip_address) FROM auth_events WHERE ip_address != '' AND created_at >= CURRENT_DATE`,
	).Scan(&s.UniqueIPs)

	totalToday := s.LoginsToday + s.FailedToday
	if totalToday > 0 {
		s.SuccessRate = int(float64(s.LoginsToday) / float64(totalToday) * 100)
	}

	return s, nil
}

func (r *Repository) GetAuthEventTrend(ctx context.Context, days int) ([]model.AuthEventTrend, error) {
	if days < 1 || days > 90 {
		days = 7
	}

	since := time.Now().AddDate(0, 0, -days).Truncate(24 * time.Hour)
	rows, err := r.db.Pool.Query(ctx,
		`SELECT DATE(created_at)::TEXT AS d,
		        COUNT(*) FILTER (WHERE event_type = 'login_success') AS success,
		        COUNT(*) FILTER (WHERE status = 'failure') AS failure
		 FROM auth_events
		 WHERE created_at >= $1
		 GROUP BY DATE(created_at)
		 ORDER BY d ASC`,
		since,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trends []model.AuthEventTrend
	for rows.Next() {
		t := model.AuthEventTrend{}
		if err := rows.Scan(&t.Date, &t.Success, &t.Failure); err != nil {
			return nil, err
		}
		trends = append(trends, t)
	}
	return trends, nil
}

func (r *Repository) DetectBruteForce(ctx context.Context) ([]model.BruteForceAlert, error) {
	threshold := 20
	windowMinutes := 5

	rows, err := r.db.Pool.Query(ctx,
		`SELECT ip_address, COUNT(*) AS failures,
		        MIN(created_at) AS first_attempt, MAX(created_at) AS last_attempt,
		        COUNT(DISTINCT user_id) AS user_count
		 FROM auth_events
		 WHERE status = 'failure' AND created_at >= NOW() - ($1 || ' minutes')::INTERVAL
		 GROUP BY ip_address
		 HAVING COUNT(*) >= $2
		 ORDER BY failures DESC`,
		fmt.Sprintf("%d", windowMinutes), threshold,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []model.BruteForceAlert
	for rows.Next() {
		a := model.BruteForceAlert{
			WindowMinutes: windowMinutes,
		}
		var first, last time.Time
		if err := rows.Scan(&a.IPAddress, &a.Failures, &first, &last, &a.UserCount); err != nil {
			return nil, err
		}
		a.FirstAttempt = first.Format(time.RFC3339)
		a.LastAttempt = last.Format(time.RFC3339)
		alerts = append(alerts, a)
	}
	return alerts, nil
}

// ListMyAuthEvents returns recent auth events for a specific user.
func (r *Repository) ListMyAuthEvents(ctx context.Context, userID string, limit int) ([]*model.AuthEvent, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, COALESCE(user_id::TEXT,''), email, event_type, status, COALESCE(failure_reason,''),
		        ip_address, user_agent, country, asn, isp, created_at
		 FROM auth_events WHERE user_id = $1
		 ORDER BY created_at DESC LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*model.AuthEvent
	for rows.Next() {
		e := &model.AuthEvent{}
		if err := rows.Scan(&e.ID, &e.UserID, &e.Email, &e.EventType, &e.Status, &e.FailureReason,
			&e.IPAddress, &e.UserAgent, &e.Country, &e.ASN, &e.ISP, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.IPAddress = maskIP(e.IPAddress)
		events = append(events, e)
	}
	return events, nil
}

// GetTopIPs returns IPs with the most failed auth events in the given period.
func (r *Repository) GetTopIPs(ctx context.Context, days int) ([]model.TopIPEntry, error) {
	if days < 1 || days > 90 {
		days = 7
	}
	rows, err := r.db.Pool.Query(ctx,
		`SELECT ip_address, COUNT(*) AS failures, COUNT(DISTINCT user_id) AS users,
		        COALESCE(MIN(country), '') AS country
		 FROM auth_events
		 WHERE status = 'failure' AND created_at >= NOW() - ($1 || ' days')::INTERVAL AND ip_address != ''
		 GROUP BY ip_address
		 ORDER BY failures DESC LIMIT 10`,
		fmt.Sprintf("%d", days),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []model.TopIPEntry
	for rows.Next() {
		var e model.TopIPEntry
		if err := rows.Scan(&e.IPAddress, &e.Failures, &e.Users, &e.Country); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// GetTopUsers returns users with the most failed auth events in the given period.
func (r *Repository) GetTopUsers(ctx context.Context, days int) ([]model.TopUserEntry, error) {
	if days < 1 || days > 90 {
		days = 7
	}
	rows, err := r.db.Pool.Query(ctx,
		`SELECT email, COUNT(*) AS failures, COALESCE(user_id::TEXT, '') AS user_id
		 FROM auth_events
		 WHERE status = 'failure' AND created_at >= NOW() - ($1 || ' days')::INTERVAL
		  AND email != ''
		 GROUP BY email, user_id
		 ORDER BY failures DESC LIMIT 10`,
		fmt.Sprintf("%d", days),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []model.TopUserEntry
	for rows.Next() {
		var e model.TopUserEntry
		if err := rows.Scan(&e.Email, &e.Failures, &e.UserID); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// GetHourlyHeatmap returns hourly aggregated auth events for the given period.
func (r *Repository) GetHourlyHeatmap(ctx context.Context, days int) ([]model.HourlyHeatmapEntry, error) {
	if days < 1 || days > 90 {
		days = 7
	}
	rows, err := r.db.Pool.Query(ctx,
		`SELECT EXTRACT(HOUR FROM created_at)::int AS hour,
		        COUNT(*) FILTER (WHERE status = 'success') AS success,
		        COUNT(*) FILTER (WHERE status = 'failure') AS failure
		 FROM auth_events
		 WHERE created_at >= NOW() - ($1 || ' days')::INTERVAL
		 GROUP BY hour
		 ORDER BY hour`,
		fmt.Sprintf("%d", days),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []model.HourlyHeatmapEntry
	for rows.Next() {
		var e model.HourlyHeatmapEntry
		if err := rows.Scan(&e.Hour, &e.Success, &e.Failure); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// maskIP obscures the last octet(s) of an IP for user-facing views.
func maskIP(ip string) string {
	if ip == "" {
		return ip
	}
	// Simple IPv4 masking
	for i := len(ip) - 1; i >= 0; i-- {
		if ip[i] == '.' {
			return ip[:i+1] + "***"
		}
	}
	return ip
}

// ─── Security Events ──────────────────────────────────────────────────────────

func (r *Repository) CreateSecurityEvent(ctx context.Context, e *model.SecurityEvent) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO security_events (id, event_type, ip_address, details, severity, detected_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		e.ID, e.EventType, e.IPAddress, e.Details, e.Severity, e.DetectedAt, e.CreatedAt,
	)
	return err
}

// ─── Blocked IPs (DB-backed persistence) ─────────────────────────────────────

func (r *Repository) CreateBlockedIP(ctx context.Context, b *model.BlockedIP) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO blocked_ips (id, ip_address, reason, created_by, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (ip_address) DO UPDATE SET
		   reason = EXCLUDED.reason,
		   created_by = EXCLUDED.created_by,
		   updated_at = NOW()`,
		b.ID, b.IPAddress, b.Reason, b.CreatedBy, b.CreatedAt, b.CreatedAt,
	)
	return err
}

func (r *Repository) RemoveBlockedIP(ctx context.Context, ipAddress string) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM blocked_ips WHERE ip_address = $1`, ipAddress)
	return err
}

func (r *Repository) ListBlockedIPs(ctx context.Context) ([]model.BlockedIP, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, ip_address, COALESCE(reason,''), COALESCE(created_by,''), created_at, updated_at
		 FROM blocked_ips ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ips []model.BlockedIP
	for rows.Next() {
		var b model.BlockedIP
		if err := rows.Scan(&b.ID, &b.IPAddress, &b.Reason, &b.CreatedBy, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		ips = append(ips, b)
	}
	if ips == nil {
		ips = []model.BlockedIP{}
	}
	return ips, nil
}

// GetUserIDByEmail returns a user's ID by their email address.
func (r *Repository) GetUserIDByEmail(ctx context.Context, email string) (string, error) {
	var userID string
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id::TEXT FROM users WHERE email = $1`, email,
	).Scan(&userID)
	if err != nil {
		return "", err
	}
	return userID, nil
}

// ─── Auth Events Purge ─────────────────────────────────────────────────────────

func (r *Repository) PurgeAuthEvents(ctx context.Context, olderThan time.Duration) (int64, error) {
	result, err := r.db.Pool.Exec(ctx,
		`DELETE FROM auth_events WHERE created_at < NOW() - ($1 || ' days')::INTERVAL`,
		fmt.Sprintf("%d", int(olderThan.Hours()/24)),
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

// ─── Brute Force Config ─────────────────────────────────────────────────────

// GetBruteForceConfig returns the brute force alert config.
func (r *Repository) GetBruteForceConfig(ctx context.Context) (*model.BruteForceConfig, error) {
	cfg := &model.BruteForceConfig{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, COALESCE(notification_target_ids, '{}'), created_at, updated_at
		 FROM brute_force_config WHERE id = 'default'`,
	).Scan(&cfg.ID, &cfg.NotificationTargetIDs, &cfg.CreatedAt, &cfg.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &model.BruteForceConfig{ID: "default", NotificationTargetIDs: []string{}}, nil
		}
		return nil, err
	}
	if cfg.NotificationTargetIDs == nil {
		cfg.NotificationTargetIDs = []string{}
	}
	return cfg, nil
}

// UpsertBruteForceConfig updates the brute force alert config.
func (r *Repository) UpsertBruteForceConfig(ctx context.Context, targetIDs []string) (*model.BruteForceConfig, error) {
	if targetIDs == nil {
		targetIDs = []string{}
	}
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO brute_force_config (id, notification_target_ids, created_at, updated_at)
		 VALUES ('default', $1, NOW(), NOW())
		 ON CONFLICT (id) DO UPDATE SET
		   notification_target_ids = $1,
		   updated_at = NOW()`,
		targetIDs,
	)
	if err != nil {
		return nil, err
	}
	return r.GetBruteForceConfig(ctx)
}