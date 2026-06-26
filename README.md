# Anjungan — Internal Developer Platform

[![Build](https://github.com/edsuwarna/anjungan/actions/workflows/docker-build-push.yml/badge.svg)](https://github.com/edsuwarna/anjungan/actions/workflows/docker-build-push.yml)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-Apache%202.0-10b981.svg)](LICENSE)

> **Anjungan** (Indonesian: *platform*) — A modular internal developer platform (IDP) for managing servers, containers, deployments, container registries, and infrastructure compliance through a unified dashboard.

## Documentation

📚 **[docs/](docs/)** — Full documentation:

| File | Contents |
||------|----------|
|| [README.md](docs/README.md) | Overview, tech stack, project structure |
|| [setup.md](docs/setup.md) | Development setup, prerequisites, make commands |
|| [deployment.md](docs/deployment.md) | Docker Compose (with & without clone), production guide |
|| [architecture.md](docs/architecture.md) | System architecture, router, frontend, Zot |
|| [api.md](docs/api.md) | Full API reference (all endpoints) |
|| [docker.md](docs/docker.md) | Build, tagging convention, CI/CD pipeline |
|| [compliance.md](docs/compliance.md) | CIS scanning, Lynis, scoring system, thresholds |
|| [registry.md](docs/registry.md) | Zot self-service credentials, roles, usage |
|| [self-server.md](docs/self-server.md) | Host auto-registration, Docker socket, configuration |
|| [software-katalog.md](prd/PRD-software-katalog.md) | Software Catalog PRD — app store for Docker Compose deployments |

## Quick Start

### Option A: From Source (clone)

```bash
git clone git@github.com:edsuwarna/anjungan.git
cd anjungan
cp .env.example .env
docker compose up -d
# Access: http://localhost
```

### Option B: Deploy without Clone (pre-built images)

```bash
# Just need Docker & a compose file (see docs/deployment.md)
docker login registry.edsuwarna.xyz -u deploy
docker compose up -d
```

## Stack

| Layer | Tech |
|-------|------|
| Frontend | SvelteKit 5, Svelte 5 (runes), Tailwind CSS 3, Iconify |
| Backend | Go 1.25, Chi router, pgx (PostgreSQL driver) |
| Database | PostgreSQL 17, Redis 7 |
| Container Registry | Zot (OCI-distribution compliant) |
| Auth | JWT (access + refresh tokens), TOTP 2FA |
| Deployment | Docker Compose, multi-stage Dockerfiles, GitHub Actions |

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
│   │   ├── authactivity/  # Login activity & brute force detection
│   │   ├── bookmark/      # Tool shortcut bookmarks
│   │   ├── infra/         # Server/infrastructure management
│   │   ├── container/     # Docker container management
│   │   ├── registry/      # Container registry integration
│   │   ├── repository/    # Git repository integration
│   │   ├── deployment/    # Deployment pipeline
│   │   ├── dashboard/     # Dashboard aggregation
│   │   ├── compliance/    # Security compliance checks
│   │   ├── sslmonitor/    # SSL certificate monitoring
│   │   ├── uptime/        # Uptime monitoring
│   │   ├── notification/  # Notification targets & delivery
│   │   ├── settings/      # Application settings
│   │   ├── admin/         # User & permission management
│   │   └── audit/         # Audit logging
│   └── migrations/        # PostgreSQL migrations
├── frontend/              # SvelteKit SPA
│   └── src/
│       ├── lib/           # Components, stores, API client
│       └── routes/        # Pages
├── docs/                  # Documentation
├── zot/                   # Zot registry config
├── docker-compose.yml     # All services
├── Dockerfile.backend     # Multi-stage Go build
├── Dockerfile.frontend    # Nginx static SPA (npm build)
├── Makefile               # Developer commands
└── .env.example           # Environment variables
```

## License

[![License](https://img.shields.io/badge/License-Apache%202.0-10b981.svg)](LICENSE)

Apache License 2.0 — see [LICENSE](LICENSE).
