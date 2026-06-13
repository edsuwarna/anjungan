# Anjungan — Internal Developer Platform

> **Anjungan** (Indonesian: *platform*) — A modular internal developer platform (IDP) for managing servers, containers, deployments, container registries, and infrastructure compliance through a unified dashboard.

## Overview

Anjungan provides a single-pane-of-glass for DevOps teams:

- **Server management** — SSH-key-based connection to remote servers, terminal access via WebSocket
- **Container management** — view containers across all servers, inspect details, monitor status
- **Compliance scanning** — CIS-based security audits, Lynis hardening scans, container image vulnerability scanning
- **Container registry** — integrated Zot private registry with self-service user credentials
- **Deployments** — track and manage deployments across environments
- **Repository management** — GitHub & Forgejo repository integration
- **SSL Certificate Monitoring** — monitor SSL/TLS certificate expiry for any domain, with automated TLS checks, cipher grading, chain validation, OCSP status, check history with trend chart, and deduped notifications via Telegram/Discord/Slack
- **Server-side certificate discovery** — auto-detect SSL certs from connected servers (Traefik, Nginx, Caddy, Let's Encrypt, filesystem scan)
- **Admin console** — user management, audit logging

## Tech Stack

| Layer | Technology |
|-------|-----------|
| **Frontend** | SvelteKit 5, Svelte 5 (runes), Tailwind CSS 3, Iconify |
| **Backend** | Go 1.25, Chi router, pgx (PostgreSQL driver) |
| **Database** | PostgreSQL 17, Redis 7 |
| **Container Registry** | Zot (OCI-distribution compliant) |
| **Auth** | JWT (access + refresh tokens), TOTP 2FA |
| **Infrastructure** | Docker Compose, multi-stage Dockerfiles |

## Services

| Service | Container | Port | Description |
|---------|-----------|------|-------------|
| `backend` | `anjungan-backend` | 8080 | Go API server |
| `frontend` | `anjungan-frontend` | 80 | SvelteKit SPA (nginx) |
| `postgres` | `anjungan-postgres` | 5433 | Database |
| `redis` | `anjungan-redis` | 6379 | Cache + rate limiter |
| `zot` | `anjungan-zot` | 5000 | OCI registry (internal) |

## Project Structure

```
anjungan/
├── backend/                  # Go modular monolith
│   ├── cmd/server/           # Entry point
│   ├── internal/
│   │   ├── server/           # HTTP server, router, middleware
│   │   ├── config/           # Environment configuration
│   │   ├── common/           # Shared types, DB, helpers
│   │   ├── auth/             # Authentication & authorization
│   │   ├── authactivity/     # Login activity & brute force detection
│   │   ├── bookmark/         # Tool shortcut bookmarks
│   │   ├── infra/            # Server/infrastructure management
│   │   ├── container/        # Docker container management
│   │   ├── registry/         # Container registry integration
│   │   ├── repository/       # Git repository integration
│   │   ├── deployment/       # Deployment pipeline
│   │   ├── dashboard/        # Dashboard aggregation
│   │   ├── compliance/       # Security compliance checks
│   │   ├── sslmonitor/       # SSL certificate monitoring
│   │   ├── uptime/           # Uptime monitoring
│   │   ├── notification/     # Notification targets & delivery
│   │   ├── settings/         # Application settings (thresholds)
│   │   ├── admin/            # User & permission management
│   │   └── audit/            # Audit logging
│   └── migrations/           # PostgreSQL migrations
├── frontend/                 # SvelteKit SPA
│   └── src/
│       ├── lib/              # Components, stores, API client
│       └── routes/           # Pages
├── docs/                     # Documentation
├── zot/                      # Zot configuration
│   ├── config.json           # Zot config with htpasswd auth
│   ├── htpasswd.example      # Example htpasswd format (auto-generated at runtime)
├── docker-compose.yml        # All services
├── Dockerfile.backend        # Multi-stage Go build
├── Dockerfile.frontend       # Nginx static SPA (npm build)
├── Makefile                  # Developer commands
└── .env.example              # Environment variables template
```

## Quick Start

```bash
# Clone
git clone git@github.com:edsuwarna/anjungan.git
cd anjungan

# Copy env vars
cp .env.example .env

# Start all services
docker compose up -d

# Access the dashboard
open http://localhost
```

## License

MIT
