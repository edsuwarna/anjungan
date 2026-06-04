# ─── Development ─────────────────────────────────────────────────────────────
.PHONY: dev dev-backend dev-frontend dev-db

## Run all services (Docker compose)
dev:
	docker compose up -d

## Run backend with hot-reload (requires Go air)
dev-backend:
	cd backend && air -- --config ../.air.toml

## Run frontend dev server
dev-frontend:
	cd frontend && npm run dev

## Start only database services
dev-db:
	docker compose up -d postgres redis

## Connect to PostgreSQL
db:
	docker compose exec postgres psql -U $$POSTGRES_USER -d $$POSTGRES_DB

# ─── Database Migrations ─────────────────────────────────────────────────────
# Migrations run automatically on backend startup. Use these for management.

.PHONY: migrate-up migrate-create

## Restart backend to run pending migrations
migrate-up:
	docker compose restart backend
	@echo "Backend restarted — migrations auto-run on startup"

## Create new migration files (sequential numbering)
migrate-create:
	@read -p "Migration name (e.g. add_user_groups): " name; \
	last=$$(ls backend/migrations/*.up.sql 2>/dev/null | tail -1 | grep -oP '\d+' | head -1); \
	next=$$(printf "%06d" $$((10#$${last:-0} + 1))); \
	echo "Creating $${next}_$${name}.up.sql and .down.sql"; \
	printf -- "-- Migration: %s_%s\n-- Up: apply the changes\n" "$${next}" "$$name" > "backend/migrations/$${next}_$${name}.up.sql"; \
	printf -- "-- Migration: %s_%s\n-- Down: revert the changes\n" "$${next}" "$$name" > "backend/migrations/$${next}_$${name}.down.sql"; \
	echo "Created: $${next}_$${name}.{up,down}.sql"

# ─── Docker ──────────────────────────────────────────────────────────────────
.PHONY: build up down logs restart

build:
	docker compose build

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f

restart:
	docker compose restart

# ─── Build & Deploy ──────────────────────────────────────────────────────────
.PHONY: build-backend build-frontend

build-backend:
	cd backend && go build -ldflags="-s -w" -o /tmp/anjungan-server ./cmd/server

build-frontend:
	cd frontend && npm run build

# ─── Go ──────────────────────────────────────────────────────────────────────
.PHONY: go-tidy go-test go-lint

go-tidy:
	cd backend && go mod tidy

go-test:
	cd backend && go test ./... -v -count=1

go-lint:
	cd backend && go vet ./...

# ─── Help ────────────────────────────────────────────────────────────────────
help:
	@echo "Anjungan — Internal Developer Platform"
	@echo ""
	@echo "Commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Development:"
	@echo "  make dev              Run all services"
	@echo "  make dev-backend      Hot-reload backend"
	@echo "  make dev-frontend     Hot-reload frontend"
	@echo ""
	@echo "Database:"
	@echo "  make migrate-up       Apply all pending migrations"
	@echo "  make migrate-down     Rollback last migration"
	@echo ""
	@echo "Docker:"
	@echo "  make build            Build all images"
	@echo "  make up               Start services"
	@echo "  make down             Stop services"
	@echo "  make logs             Tail logs"
