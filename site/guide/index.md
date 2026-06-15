---
title: Overview
description: Anjungan is a modular internal developer platform (IDP) for managing servers, containers, registries, and compliance.
---

# Anjungan — Internal Developer Platform

> **Anjungan** (Indonesian: *platform*) — A modular internal developer platform (IDP) for managing servers, containers, container registries, and infrastructure compliance through a unified dashboard.

## Overview

Anjungan provides a single-pane-of-glass for DevOps teams:

- **Server management** — SSH-key-based connection to remote servers, terminal access via WebSocket
- **Container management** — view containers across all servers, inspect details, monitor status
- **Compliance scanning** — CIS-based security audits, Lynis hardening scans, container image vulnerability scanning
- **Container registry** — integrated Zot private registry with self-service user credentials
- **SSL Certificate Monitoring** — monitor SSL/TLS certificate expiry for any domain, with automated TLS checks, cipher grading, chain validation, OCSP status, and deduped notifications
- **Server-side certificate discovery** — auto-detect SSL certs from connected servers
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

Default admin credentials: `admin@anjungan.id` / `admin123`

## Project Structure

```
anjungan/
├── backend/                  # Go modular monolith
│   ├── cmd/server/           # Entry point
│   ├── internal/             # Feature packages
│   │   ├── auth/             # Authentication & authorization
│   │   ├── infra/            # Server/infrastructure management
│   │   ├── container/        # Docker container management
│   │   ├── registry/         # Container registry integration
│   │   ├── dashboard/        # Dashboard aggregation
│   │   ├── compliance/       # Security compliance checks
│   │   ├── sslmonitor/       # SSL certificate monitoring
│   │   ├── uptime/           # Uptime monitoring
│   │   ├── notification/     # Notification targets & delivery
│   │   ├── settings/         # Application settings
│   │   ├── admin/            # User & permission management
│   │   └── audit/            # Audit logging
│   └── migrations/           # PostgreSQL migrations
├── frontend/                 # SvelteKit SPA
├── docs/                     # Internal documentation
└── docker-compose.yml        # All services
```

## License

MIT
