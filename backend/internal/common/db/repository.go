package db

import (
	"context"

	"github.com/edsuwarna/anjungan/internal/common/model"
)

// ─── User repository ──────────────────────────────────────────────────────

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	u := &model.User{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, email, name, password_hash, totp_secret, totp_enabled, role, created_at, updated_at
		 FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.TOTPSecret, &u.TOTPEnabled, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *Repository) GetUserByID(ctx context.Context, id string) (*model.User, error) {
	u := &model.User{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, email, name, password_hash, totp_secret, totp_enabled, role, created_at, updated_at
		 FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.TOTPSecret, &u.TOTPEnabled, &u.Role, &u.CreatedAt, &u.UpdatedAt)
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
		`INSERT INTO servers (id, name, host, port, status, created_by, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		s.ID, s.Name, s.Host, s.Port, s.Status, s.CreatedBy, s.CreatedAt, s.UpdatedAt,
	)
	return err
}

func (r *Repository) ListServers(ctx context.Context) ([]*model.Server, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, name, host, port, status, created_by, created_at, updated_at FROM servers ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []*model.Server
	for rows.Next() {
		s := &model.Server{}
		if err := rows.Scan(&s.ID, &s.Name, &s.Host, &s.Port, &s.Status, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		servers = append(servers, s)
	}
	return servers, nil
}

func (r *Repository) GetServerByID(ctx context.Context, id string) (*model.Server, error) {
	s := &model.Server{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, name, host, port, status, created_by, created_at, updated_at FROM servers WHERE id = $1`, id,
	).Scan(&s.ID, &s.Name, &s.Host, &s.Port, &s.Status, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (r *Repository) DeleteServer(ctx context.Context, id string) error {
	_, err := r.db.Pool.Exec(ctx, "DELETE FROM servers WHERE id = $1", id)
	return err
}
