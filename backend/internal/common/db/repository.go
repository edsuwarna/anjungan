package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
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
		`SELECT id, email, name, role, totp_enabled, created_at, updated_at FROM users ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*model.User
	for rows.Next() {
		u := &model.User{}
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.TOTPEnabled, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

// ─── Server repository ────────────────────────────────────────────────────

func (r *Repository) CreateServer(ctx context.Context, s *model.Server) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO servers (id, name, host, port, ssh_user, ssh_auth_type, ssh_key, ssh_key_id, ssh_password,
		 status, tags, server_group, region, server_type, description, monitoring, created_by, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)`,
		s.ID, s.Name, s.Host, s.Port, s.SSHUser, s.SSHAuthType, s.SSHKey, s.SSHKeyID, s.SSHPassword,
		s.Status, s.Tags, s.ServerGroup, s.Region, s.ServerType, s.Description,
		s.Monitoring, s.CreatedBy, s.CreatedAt, s.UpdatedAt,
	)
	return err
}

const serverColumns = `id, name, host, port, ssh_user, ssh_auth_type, status, container_count,
	COALESCE(tags, '{}'), COALESCE(labels, '{}')::text, COALESCE(server_group, ''),
	COALESCE(region, ''), COALESCE(server_type, ''), COALESCE(description, ''),
	COALESCE(os_info, ''), COALESCE(cpu_info, ''), last_seen_at, COALESCE(monitoring, false),
	created_by, created_at, updated_at,	COALESCE(ssh_key_id::text, '')`

func scanServer(scanner interface {
	Scan(dest ...interface{}) error
}) (*model.Server, error) {
	s := &model.Server{}
	err := scanner.Scan(
		&s.ID, &s.Name, &s.Host, &s.Port, &s.SSHUser, &s.SSHAuthType,
		&s.Status, &s.ContainerCount, &s.Tags, &s.Labels,
		&s.ServerGroup, &s.Region, &s.ServerType, &s.Description,
		&s.OSInfo, &s.CPUInfo, &s.LastSeenAt, &s.Monitoring,
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
		&s.CreatedBy, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
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
		`SELECT `+serverColumns+` FROM servers s %s ORDER BY %s %s LIMIT $%d OFFSET $%d`,
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
		s, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		servers = append(servers, s.ToResponse())
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
		 COALESCE(os_info, ''), COALESCE(cpu_info, ''), last_seen_at, COALESCE(monitoring, false), created_by, created_at, updated_at
		 FROM servers WHERE id = $1`, id,
	)
	return scanServerFull(row)
}

func (r *Repository) UpdateServer(ctx context.Context, s *model.Server) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE servers SET name=$1, host=$2, port=$3, ssh_user=$4, ssh_auth_type=$5, ssh_key=$6,
		 ssh_password=$7, ssh_key_id=$8, status=$9, container_count=$10, tags=$11, server_group=$12, region=$13,
		 server_type=$14, description=$15, os_info=$16, cpu_info=$17, monitoring=$18, updated_at=NOW()
		 WHERE id=$19`,
		s.Name, s.Host, s.Port, s.SSHUser, s.SSHAuthType, s.SSHKey, s.SSHPassword,
		s.SSHKeyID, s.Status, s.ContainerCount, s.Tags, s.ServerGroup, s.Region,
		s.ServerType, s.Description, s.OSInfo, s.CPUInfo, s.Monitoring, s.ID,
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

func (r *Repository) CountDeployments(ctx context.Context) (int, error) {
	var count int
	err := r.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM deployments").Scan(&count)
	return count, err
}

func (r *Repository) CountDeploymentsByStatus(ctx context.Context) (map[string]int, error) {
	rows, err := r.db.Pool.Query(ctx, `SELECT status, COUNT(*) FROM deployments GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]int{}
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

func (r *Repository) ListRecentDeployments(ctx context.Context, limit int) ([]*model.Deployment, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT d.id, d.name, d.environment_id, d.repo_provider, d.repo_owner, d.repo_name,
			d.branch, d.commit_sha, d.server_id, d.service_name, d.image, d.status,
			d.deployed_by, d.deployed_at, d.updated_at, d.rollback_from,
			COALESCE(e.name,''), COALESCE(e.color,'#10b981'), COALESCE(s.name,'')
			FROM deployments d
			LEFT JOIN environments e ON e.id = d.environment_id
			LEFT JOIN servers s ON s.id = d.server_id
			ORDER BY d.deployed_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var deps []*model.Deployment
	for rows.Next() {
		d := &model.Deployment{}
		if err := rows.Scan(&d.ID, &d.Name, &d.EnvironmentID, &d.RepoProvider, &d.RepoOwner, &d.RepoName,
			&d.Branch, &d.CommitSHA, &d.ServerID, &d.ServiceName, &d.Image, &d.Status,
			&d.DeployedBy, &d.DeployedAt, &d.UpdatedAt, &d.RollbackFrom,
			&d.EnvironmentName, &d.EnvironmentColor, &d.ServerName); err != nil {
			return nil, err
		}
		deps = append(deps, d)
	}
	return deps, nil
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

	// Insert new
	for _, g := range groups {
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
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, server_id, scan_type, status, score, total_checks, passed, warnings, criticals, error_message, started_at, completed_at, created_at
		 FROM scan_results WHERE server_id = $1 AND scan_type = $2 ORDER BY created_at DESC LIMIT 1`, serverID, scanType,
	).Scan(&s.ID, &s.ServerID, &s.ScanType, &s.Status, &s.Score, &s.TotalChecks,
		&s.Passed, &s.Warnings, &s.Criticals, &s.ErrorMessage, &s.StartedAt, &s.CompletedAt, &s.CreatedAt)
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
		countQuery = `SELECT COUNT(*) FROM scan_results WHERE scan_type = $1`
		countArgs = append(countArgs, scanType)
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
		query = `SELECT sr.id, sr.server_id, COALESCE(s.name,''), COALESCE(s.host,''),
		       sr.scan_type, sr.status, sr.score, sr.total_checks, sr.passed, sr.warnings, sr.criticals,
		       sr.started_at, sr.completed_at, sr.created_at
		 FROM scan_results sr
		 LEFT JOIN servers s ON sr.server_id = s.id
		 WHERE sr.scan_type = $1
		 ORDER BY sr.created_at DESC LIMIT $2 OFFSET $3`
		args = append(args, scanType, limit, offset)
	} else {
		query = `SELECT sr.id, sr.server_id, COALESCE(s.name,''), COALESCE(s.host,''),
		       sr.scan_type, sr.status, sr.score, sr.total_checks, sr.passed, sr.warnings, sr.criticals,
		       sr.started_at, sr.completed_at, sr.created_at
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
			&item.StartedAt, &item.CompletedAt, &item.CreatedAt); err != nil {
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

func (r *Repository) GetComplianceSummary(ctx context.Context) (*model.ComplianceSummary, error) {
	summary := &model.ComplianceSummary{
		ByStatus: make(map[string]int),
	}

	// Total servers
	err := r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM servers`).Scan(&summary.TotalServers)
	if err != nil {
		return nil, err
	}

	// Latest scan per server
	rows, err := r.db.Pool.Query(ctx, `
		SELECT s.id, s.name, s.host,
		       sr.score, COALESCE(sr.criticals,0), COALESCE(sr.warnings,0), COALESCE(sr.passed,0), sr.completed_at
		FROM servers s
		LEFT JOIN LATERAL (
		    SELECT score, criticals, warnings, passed, completed_at
		    FROM scan_results
		    WHERE server_id = s.id AND status = 'completed'
		    ORDER BY created_at DESC LIMIT 1
		) sr ON true
		ORDER BY s.name
	`)
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

	// Top findings across all servers
	topRows, err := r.db.Pool.Query(ctx, `
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
	`)
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

// ─── Environment Repository ─────────────────────────────────────────────────

func (r *Repository) ListEnvironments(ctx context.Context) ([]*model.Environment, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, name, color, COALESCE(description,''), is_protected, created_at, updated_at
		 FROM environments ORDER BY is_protected DESC, name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var envs []*model.Environment
	for rows.Next() {
		e := &model.Environment{}
		if err := rows.Scan(&e.ID, &e.Name, &e.Color, &e.Description, &e.IsProtected, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		envs = append(envs, e)
	}
	return envs, nil
}

func (r *Repository) GetEnvironment(ctx context.Context, id string) (*model.Environment, error) {
	e := &model.Environment{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, name, color, COALESCE(description,''), is_protected, created_at, updated_at
		 FROM environments WHERE id = $1`, id,
	).Scan(&e.ID, &e.Name, &e.Color, &e.Description, &e.IsProtected, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return e, nil
}

func (r *Repository) CreateEnvironment(ctx context.Context, e *model.Environment) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO environments (id, name, color, description, is_protected, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		e.ID, e.Name, e.Color, e.Description, e.IsProtected, e.CreatedAt, e.UpdatedAt,
	)
	return err
}

func (r *Repository) UpdateEnvironment(ctx context.Context, e *model.Environment) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE environments SET name=$1, color=$2, description=$3, updated_at=NOW() WHERE id=$4`,
		e.Name, e.Color, e.Description, e.ID,
	)
	return err
}

func (r *Repository) DeleteEnvironment(ctx context.Context, id string) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM environments WHERE id = $1`, id)
	return err
}

// ─── Repo Connection Repository ─────────────────────────────────────────────

func (r *Repository) ListRepoConnections(ctx context.Context, userID string) ([]*model.RepoConnection, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, user_id, provider, COALESCE(label,''), COALESCE(base_url,''), token_encrypted, is_active, COALESCE(affiliations,'owner,collaborator,organization_member'), created_at, updated_at
		 FROM repo_connections WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var conns []*model.RepoConnection
	for rows.Next() {
		c := &model.RepoConnection{}
		if err := rows.Scan(&c.ID, &c.UserID, &c.Provider, &c.Label, &c.BaseURL, &c.TokenEncrypted, &c.IsActive, &c.Affiliations, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		conns = append(conns, c)
	}
	return conns, nil
}

func (r *Repository) CreateRepoConnection(ctx context.Context, c *model.RepoConnection) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO repo_connections (id, user_id, provider, label, base_url, token_encrypted, is_active, affiliations, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		c.ID, c.UserID, c.Provider, c.Label, c.BaseURL, c.TokenEncrypted, c.IsActive, c.Affiliations, c.CreatedAt, c.UpdatedAt,
	)
	return err
}

func (r *Repository) DeleteRepoConnection(ctx context.Context, id, userID string) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM repo_connections WHERE id = $1 AND user_id = $2`, id, userID)
	return err
}

// ─── Repo Selection Repository ──────────────────────────────────────────────

func (r *Repository) GetRepoSelections(ctx context.Context, userID string) ([]*model.RepoSelection, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, user_id, provider, owner, repo_name, selected, created_at, updated_at
		 FROM repo_selections WHERE user_id = $1`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var selections []*model.RepoSelection
	for rows.Next() {
		s := &model.RepoSelection{}
		if err := rows.Scan(&s.ID, &s.UserID, &s.Provider, &s.Owner, &s.RepoName, &s.Selected, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		selections = append(selections, s)
	}
	return selections, nil
}

func (r *Repository) BulkSetRepoSelections(ctx context.Context, userID string, items []model.RepoSelectionItem) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, item := range items {
		_, err := tx.Exec(ctx,
			`INSERT INTO repo_selections (id, user_id, provider, owner, repo_name, selected, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
			 ON CONFLICT (user_id, provider, owner, repo_name)
			 DO UPDATE SET selected = $6, updated_at = NOW()`,
			uuid.New().String(), userID, item.Provider, item.Owner, item.RepoName, item.Selected,
		)
		if err != nil {
			return fmt.Errorf("upsert selection %s/%s/%s: %w", item.Provider, item.Owner, item.RepoName, err)
		}
	}

	return tx.Commit(ctx)
}

// ─── Deployment Repository ──────────────────────────────────────────────────

func (r *Repository) ListDeployments(ctx context.Context, environmentID string) ([]*model.Deployment, error) {
	query := `SELECT d.id, d.name, d.environment_id, d.repo_provider, d.repo_owner, d.repo_name,
		d.branch, d.commit_sha, d.server_id, d.service_name, d.image, d.status,
		d.deployed_by, d.deployed_at, d.updated_at, d.rollback_from,
		COALESCE(e.name,''), COALESCE(e.color,'#10b981'), COALESCE(s.name,'')
		FROM deployments d
		LEFT JOIN environments e ON e.id = d.environment_id
		LEFT JOIN servers s ON s.id = d.server_id`
	var args []interface{}
	if environmentID != "" {
		query += ` WHERE d.environment_id = $1`
		args = append(args, environmentID)
	}
	query += ` ORDER BY d.deployed_at DESC`

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var deps []*model.Deployment
	for rows.Next() {
		d := &model.Deployment{}
		if err := rows.Scan(&d.ID, &d.Name, &d.EnvironmentID, &d.RepoProvider, &d.RepoOwner, &d.RepoName,
			&d.Branch, &d.CommitSHA, &d.ServerID, &d.ServiceName, &d.Image, &d.Status,
			&d.DeployedBy, &d.DeployedAt, &d.UpdatedAt, &d.RollbackFrom,
			&d.EnvironmentName, &d.EnvironmentColor, &d.ServerName); err != nil {
			return nil, err
		}
		deps = append(deps, d)
	}
	return deps, nil
}

func (r *Repository) GetDeployment(ctx context.Context, id string) (*model.Deployment, error) {
	d := &model.Deployment{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT d.id, d.name, d.environment_id, d.repo_provider, d.repo_owner, d.repo_name,
			d.branch, d.commit_sha, d.server_id, d.service_name, d.image, d.status,
			d.deployed_by, d.deployed_at, d.updated_at, d.rollback_from,
			COALESCE(e.name,''), COALESCE(e.color,'#10b981'), COALESCE(s.name,'')
			FROM deployments d
			LEFT JOIN environments e ON e.id = d.environment_id
			LEFT JOIN servers s ON s.id = d.server_id
			WHERE d.id = $1`, id,
	).Scan(&d.ID, &d.Name, &d.EnvironmentID, &d.RepoProvider, &d.RepoOwner, &d.RepoName,
		&d.Branch, &d.CommitSHA, &d.ServerID, &d.ServiceName, &d.Image, &d.Status,
		&d.DeployedBy, &d.DeployedAt, &d.UpdatedAt, &d.RollbackFrom,
		&d.EnvironmentName, &d.EnvironmentColor, &d.ServerName)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (r *Repository) CreateDeployment(ctx context.Context, d *model.Deployment) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO deployments (id, name, environment_id, repo_provider, repo_owner, repo_name,
			branch, commit_sha, server_id, service_name, image, status, deployed_by, deployed_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		d.ID, d.Name, d.EnvironmentID, d.RepoProvider, d.RepoOwner, d.RepoName,
		d.Branch, d.CommitSHA, d.ServerID, d.ServiceName, d.Image, d.Status,
		d.DeployedBy, d.DeployedAt, d.UpdatedAt,
	)
	return err
}

func (r *Repository) UpdateDeploymentStatus(ctx context.Context, id, status, message string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE deployments SET status=$1, updated_at=NOW() WHERE id=$2`,
		status, id,
	)
	if err != nil {
		return err
	}
	// Record history entry
	_, err = r.db.Pool.Exec(ctx,
		`INSERT INTO deployment_history (id, deployment_id, status, message, created_at)
		 VALUES (gen_random_uuid(), $1, $2, $3, NOW())`,
		id, status, message,
	)
	return err
}

func (r *Repository) RollbackDeployment(ctx context.Context, id, rollbackFromID string) error {
	// Copy the old deployment data into current
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE deployments SET rollback_from=$1, updated_at=NOW() WHERE id=$2`,
		rollbackFromID, id,
	)
	if err != nil {
		return err
	}
	_, err = r.db.Pool.Exec(ctx,
		`INSERT INTO deployment_history (id, deployment_id, status, message, created_at)
		 VALUES (gen_random_uuid(), $1, 'rolled_back', 'Rolled back', NOW())`,
		id,
	)
	return err
}

func (r *Repository) ListDeploymentHistory(ctx context.Context, deploymentID string) ([]*model.DeploymentHistory, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, deployment_id, status, COALESCE(message,''), created_at
		 FROM deployment_history WHERE deployment_id = $1 ORDER BY created_at DESC`, deploymentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hist []*model.DeploymentHistory
	for rows.Next() {
		h := &model.DeploymentHistory{}
		if err := rows.Scan(&h.ID, &h.DeploymentID, &h.Status, &h.Message, &h.CreatedAt); err != nil {
			return nil, err
		}
		hist = append(hist, h)
	}
	return hist, nil
}

func (r *Repository) ListDeploymentsByRepo(ctx context.Context, provider, owner, name string) ([]*model.Deployment, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT d.id, d.name, d.environment_id, d.repo_provider, d.repo_owner, d.repo_name,
			d.branch, d.commit_sha, d.server_id, d.service_name, d.image, d.status,
			d.deployed_by, d.deployed_at, d.updated_at, d.rollback_from,
			COALESCE(e.name,''), COALESCE(e.color,'#10b981'), COALESCE(s.name,'')
			FROM deployments d
			LEFT JOIN environments e ON e.id = d.environment_id
			LEFT JOIN servers s ON s.id = d.server_id
			WHERE d.repo_provider = $1 AND d.repo_owner = $2 AND d.repo_name = $3
			ORDER BY d.deployed_at DESC`,
		provider, owner, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var deps []*model.Deployment
	for rows.Next() {
		d := &model.Deployment{}
		if err := rows.Scan(&d.ID, &d.Name, &d.EnvironmentID, &d.RepoProvider, &d.RepoOwner, &d.RepoName,
			&d.Branch, &d.CommitSHA, &d.ServerID, &d.ServiceName, &d.Image, &d.Status,
			&d.DeployedBy, &d.DeployedAt, &d.UpdatedAt, &d.RollbackFrom,
			&d.EnvironmentName, &d.EnvironmentColor, &d.ServerName); err != nil {
			return nil, err
		}
		deps = append(deps, d)
	}
	return deps, nil
}
