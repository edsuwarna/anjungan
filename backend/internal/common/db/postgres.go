package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/edsuwarna/anjungan/internal/config"
)

// DB wraps pgxpool
type DB struct {
	Pool *pgxpool.Pool
}

func Connect(cfg config.PostgresConfig) (*DB, error) {
	pool, err := pgxpool.New(context.Background(), cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("pgxpool new: %w", err)
	}
	return &DB{Pool: pool}, nil
}

func (d *DB) Ping() error {
	return d.Pool.Ping(context.Background())
}

func (d *DB) Close() {
	d.Pool.Close()
}

func NewRedis(cfg config.RedisConfig) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr(),
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	return rdb
}

// Repository groups all DB query methods
type Repository struct {
	db *DB
}

func NewRepository(db *DB) *Repository {
	return &Repository{db: db}
}
