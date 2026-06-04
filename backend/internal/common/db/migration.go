package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// ─── Migration Runner ─────────────────────────────────────────────────────────
//
// Runs SQL migration files from a directory on startup.
// Tracks applied migrations in a `schema_migrations` table.
// Only .up.sql files are processed, sorted by filename (000001_*.up.sql).

type Migration struct {
	Version string
	Name    string
	SQL     string
}

// RunMigrations reads migration files from dir and applies any pending ones.
// Returns the number of migrations applied.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool, dir string) (int, error) {
	if err := ensureMigrationsTable(ctx, pool); err != nil {
		return 0, fmt.Errorf("ensure migrations table: %w", err)
	}

	migrations, err := loadMigrations(dir)
	if err != nil {
		return 0, fmt.Errorf("load migrations: %w", err)
	}

	if len(migrations) == 0 {
		log.Info().Str("dir", dir).Msg("no migration files found")
		return 0, nil
	}

	applied, err := getAppliedVersions(ctx, pool)
	if err != nil {
		return 0, fmt.Errorf("get applied versions: %w", err)
	}

	appliedSet := make(map[string]bool, len(applied))
	for _, v := range applied {
		appliedSet[v] = true
	}

	var pending []Migration
	for _, m := range migrations {
		if !appliedSet[m.Version] {
			pending = append(pending, m)
		}
	}

	if len(pending) == 0 {
		log.Info().Int("total", len(migrations)).Msg("all migrations already applied")
		return 0, nil
	}

	log.Info().Int("pending", len(pending)).Int("total", len(migrations)).Msg("running pending migrations")

	for _, m := range pending {
		if err := applyMigration(ctx, pool, m); err != nil {
			return 0, fmt.Errorf("migration %s: %w", m.Version, err)
		}
	}

	log.Info().Int("applied", len(pending)).Msg("migrations complete")
	return len(pending), nil
}

// ─── Internal ─────────────────────────────────────────────────────────────────

func ensureMigrationsTable(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    VARCHAR(255) PRIMARY KEY,
			name       VARCHAR(255) NOT NULL DEFAULT '',
			applied_at TIMESTAMPTZ   NOT NULL DEFAULT NOW()
		)
	`)
	return err
}

func loadMigrations(dir string) ([]Migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Warn().Str("dir", dir).Msg("migrations directory not found, skipping")
			return nil, nil
		}
		return nil, fmt.Errorf("read dir %s: %w", dir, err)
	}

	var migrations []Migration
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}

		parts := strings.SplitN(name, "_", 2)
		version := parts[0]

		path := filepath.Join(dir, name)
		sql, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}

		displayName := name
		if len(parts) > 1 {
			displayName = strings.TrimSuffix(parts[1], ".up.sql")
		}

		migrations = append(migrations, Migration{
			Version: version,
			Name:    displayName,
			SQL:     string(sql),
		})
	}

	slices.SortFunc(migrations, func(a, b Migration) int {
		if a.Version < b.Version {
			return -1
		}
		if a.Version > b.Version {
			return 1
		}
		return 0
	})

	return migrations, nil
}

func getAppliedVersions(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	rows, err := pool.Query(ctx, "SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, fmt.Errorf("query schema_migrations: %w", err)
	}
	defer rows.Close()

	var versions []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("scan version: %w", err)
		}
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

func applyMigration(ctx context.Context, pool *pgxpool.Pool, m Migration) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Run the migration SQL
	if _, err := tx.Exec(ctx, m.SQL); err != nil {
		return fmt.Errorf("exec migration: %w", err)
	}

	// Record it
	if _, err := tx.Exec(ctx,
		"INSERT INTO schema_migrations (version, name) VALUES ($1, $2) ON CONFLICT DO NOTHING",
		m.Version, m.Name,
	); err != nil {
		return fmt.Errorf("record migration: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	log.Info().Str("version", m.Version).Str("name", m.Name).Msg("migration applied")
	return nil
}
