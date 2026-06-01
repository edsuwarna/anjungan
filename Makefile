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
MIGRATE_CMD = docker compose run --rm backend migrate -path /migrations -database "postgres://$$POSTGRES_USER:$$POSTGRES_PASSWORD@postgres:5432/$$POSTGRES_DB?sslmode=disable"

.PHONY: migrate-up migrate-down migrate-create

migrate-up:
	$(MIGRATE_CMD) up

migrate-down:
	$(MIGRATE_CMD) down 1

migrate-create:
	@read -p "Migration name: " name; \
	touch backend/migrations/$$(date +%s)_$$name.up.sql \
	      backend/migrations/$$(date +%s)_$$name.down.sql

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
