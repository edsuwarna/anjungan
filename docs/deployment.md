# Deployment Guide

## Docker Compose (Recommended)

The entire platform runs via Docker Compose. One command starts everything:

```bash
docker compose up -d
```

This boots up: PostgreSQL, Redis, Backend, Frontend, and optionally Zot (registry).

### Services

| Service | Container Name | Host Port | Purpose |
|---------|---------------|-----------|---------|
| `postgres` | `anjungan-postgres` | 5433 | Primary database |
| `redis` | `anjungan-redis` | 6379 | Caching, rate limiting |
| `backend` | `anjungan-backend` | 8080 | Go API |
| `frontend` | `anjungan-frontend` | 80 | SPA (nginx) |
| `zot` | `anjungan-zot` | — | OCI registry (internal) |

> Note: PostgreSQL is mapped to port **5433** externally to avoid conflicts with local PostgreSQL on port 5432.

### Persistent Volumes

| Volume | Mount | Contents |
|--------|-------|----------|
| `pgdata` | PostgreSQL | Database files |
| `redisdata` | Redis | Cache data |
| `sshkeys` | Backend | SSH host keys |
| `zotdata` | Zot | Container images |

### Quick Commands

```bash
# Start
docker compose up -d

# Stop
docker compose down

# Restart a single service
docker compose restart backend

# View logs
docker compose logs -f

# Rebuild and restart (after code changes)
docker compose build && docker compose up -d
```

## Production Considerations

### 1. Environment Variables

Set these in production (never use defaults):

```env
JWT_SECRET=<random-64-char-string>
POSTGRES_PASSWORD=<strong-password>
GITHUB_TOKEN=<your-github-pat>
```

### 2. Security

- Change `ZOT_ADMIN_PASS` from the default
- Use SSH key-based auth for managed servers
- Rate limiting is enabled by default: 5 attempts per 15 min window, 30 min lockout

### 3. Zot Registry

Zot runs as an internal-only service in Docker. For external access, configure your reverse proxy to route `/v2/*` to `zot:5000` with basic auth.

See [docs/registry.md](registry.md) for self-service credential management.

### 4. Database Migrations

Migrations auto-run on backend startup. To trigger manually:

```bash
docker compose restart backend
```

### 5. Backups

Back up the PostgreSQL volume and `zot/` directory regularly.

## CI/CD

The project uses GitHub Actions for Docker builds:

- **Push to `main`** → builds & pushes images with `main-latest` + `main-{sha}` tags
- **Tag push `v*`** → builds & pushes with `release-latest` + version tags

Images are pushed to the Zot registry at `reg.edsuwarna.xyz`.
