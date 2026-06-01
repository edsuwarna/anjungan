# Anjungan — Internal Developer Platform

> **Anjungan** (Indonesian: *platform*) — A modular internal developer platform for managing servers, containers, deployments, and infrastructure through a unified dashboard.

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                    Frontend                          │
│               SvelteKit + Tailwind                   │
│          Emerald theme · Dark/Light mode             │
└──────────────┬──────────────────────────────────────┘
               │ HTTP
┌──────────────▼──────────────────────────────────────┐
│                  Backend API                         │
│         Go · Chi · PostgreSQL · Redis                │
│     Modular Monolith (domain packages)               │
│                                                      │
│  ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐               │
│  │ Auth │ │Infra │ │Cont. │ │Deploy│               │
│  └──────┘ └──────┘ └──────┘ └──────┘               │
│  ┌──────┐ ┌──────┐ ┌──────┐                         │
│  │ Reg. │ │ Repo │ │Admin │                         │
│  └──────┘ └──────┘ └──────┘                         │
└──────────────┬──────────────────────────────────────┘
               │
┌──────────────┴──────────────────────────────────────┐
│              Infrastructure Layer                    │
│  Docker · SSH · Zot · VictoriaMetrics · Grafana     │
└─────────────────────────────────────────────────────┘
```

## Quick Start

```bash
# Clone the repo
git clone git@github.com:edsuwarna/anjungan.git
cd anjungan

# Start all services
make dev

# Apply database migrations
make migrate-up

# Access the dashboard
open http://localhost
```

## Development

```bash
# Start databases only (PostgreSQL + Redis)
make dev-db

# Backend with hot-reload (requires air)
make dev-backend

# Frontend dev server
make dev-frontend
```

## Stack

| Layer | Tech |
|-------|------|
| Frontend | SvelteKit, Tailwind CSS, Iconify |
| Backend | Go 1.25, Chi router, pgx |
| Database | PostgreSQL 17, Redis 7 |
| Container Registry | Zot (optional) |
| Monitoring | VictoriaMetrics, Grafana (optional) |
| Deployment | Docker Compose, Dockerfiles |

## Project Structure

```
anjungan/
├── backend/               # Go modular monolith
│   ├── cmd/server/        # Entry point
│   ├── internal/
│   │   ├── server/        # HTTP server, router, middleware
│   │   ├── config/        # Environment configuration
│   │   ├── common/        # Shared types, DB, helpers
│   │   ├── auth/          # Authentication & authorization
│   │   ├── infra/         # Server/infrastructure management
│   │   ├── container/     # Docker container management
│   │   ├── registry/      # Container registry integration
│   │   ├── repository/    # GitHub repository integration
│   │   ├── deployment/    # Deployment pipeline
│   │   ├── dashboard/     # Dashboard aggregation
│   │   └── admin/         # User & permission management
│   └── migrations/        # PostgreSQL migrations
├── frontend/              # SvelteKit SPA
│   └── src/
│       ├── lib/           # Components, stores, API
│       └── routes/        # Pages
├── grafana/               # Grafana provisioning
├── docker-compose.yml     # All services
├── Dockerfile.backend     # Multi-stage Go build
├── Dockerfile.frontend    # Nginx static SPA
├── Makefile               # Developer commands
└── .env.example           # Environment variables
```

## License

MIT
