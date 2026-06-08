# Deployment Guide

## Docker Compose (Recommended)

### Option A: From Source (with clone)

```bash
git clone git@github.com:edsuwarna/anjungan.git
cd anjungan
cp .env.example .env
docker compose up -d
```

This builds images locally from the source code.

### Option B: Without Clone (pre-built images)

Deploy directly using pre-built images from the Zot registry — no need to clone the repo.

Create `docker-compose.yml` on your server:

```yaml
services:
  postgres:
    image: postgres:17-alpine
    container_name: anjungan-postgres
    environment:
      POSTGRES_USER: ${POSTGRES_USER:-anjungan}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-anjungan}
      POSTGRES_DB: ${POSTGRES_DB:-anjungan}
    ports:
      - "5433:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER:-anjungan}"]
      interval: 5s
      timeout: 3s
      retries: 5

  redis:
    image: redis:7-alpine
    container_name: anjungan-redis
    ports:
      - "6379:6379"
    volumes:
      - redisdata:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 5

  backend:
    image: registry.edsuwarna.xyz/anjungan-backend:main-latest
    container_name: anjungan-backend
    environment:
      SERVER_HOST: 0.0.0.0
      SERVER_PORT: 8080
      POSTGRES_HOST: postgres
      POSTGRES_USER: ${POSTGRES_USER:-anjungan}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-anjungan}
      POSTGRES_DB: ${POSTGRES_DB:-anjungan}
      REDIS_HOST: redis
      JWT_SECRET: ${JWT_SECRET:-change-me-in-production}
      GITHUB_TOKEN: ${GITHUB_TOKEN:-}
      REGISTRY_URL: ${REGISTRY_URL:-http://zot:5000}
      REGISTRY_EXTERNAL_URL: ${REGISTRY_EXTERNAL_URL:-registry.anjungan.io}
      ZOT_ADMIN_USER: ${ZOT_ADMIN_USER:-admin}
      ZOT_ADMIN_PASS: ${ZOT_ADMIN_PASS:-z0t_4dm1n_p4ss}
      ZOT_HTPASSWD_PATH: ${ZOT_HTPASSWD_PATH:-/data/zot/htpasswd}
      ZOT_CONTAINER_NAME: ${ZOT_CONTAINER_NAME:-anjungan-zot}
      LOG_LEVEL: ${LOG_LEVEL:-info}
      MIGRATIONS_PATH: /migrations
      # Self-server — auto-registers the host for container/metrics visibility
      SELF_SERVER_ENABLED: "true"
      SELF_HOST_NETWORK: "host.docker.internal"
    extra_hosts:
      - "host.docker.internal:host-gateway"
    ports:
      - "8080:8080"
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    group_add:
      - "988"  # Docker group GID — gives container access to /var/run/docker.sock
    volumes:
      - sshkeys:/data/ssh
      - ./zot:/data/zot:rw
      - /var/run/docker.sock:/var/run/docker.sock:rw

  frontend:
    image: registry.edsuwarna.xyz/anjungan-frontend:main-latest
    container_name: anjungan-frontend
    ports:
      - "80:80"
    depends_on:
      - backend

  zot:
    image: ghcr.io/project-zot/zot-linux-amd64:latest
    container_name: anjungan-zot
    expose:
      - "5000"
    volumes:
      - zotdata:/var/lib/zot
      - ./zot/config.json:/etc/zot/config.json:ro
      - ./zot/htpasswd:/etc/zot/htpasswd:ro
    command:
      - serve
      - /etc/zot/config.json
    restart: unless-stopped

volumes:
  pgdata:
  redisdata:
  sshkeys:
  zotdata:
```

Create the required Zot config:

```bash
mkdir -p zot
cat > zot/config.json << 'EOF'
{
  "storage": {
    "rootDirectory": "/var/lib/zot",
    "gc": true,
    "gcDelay": "168h",
    "gcInterval": "24h"
  },
  "http": {
    "address": "0.0.0.0",
    "port": "5000",
    "auth": {
      "htpasswd": {
        "path": "/etc/zot/htpasswd"
      }
    },
    "accessControl": {
      "repositories": {
        "**": {
          "policies": [
            {"users": ["**"], "actions": ["read"]},
            {"users": ["deploy"], "actions": ["read", "create"]},
            {"users": ["admin"], "actions": ["read", "create", "update", "delete"]}
          ]
        }
      }
    }
  },
  "log": {"level": "info"}
}
EOF
touch zot/htpasswd
```

Create `.env` and start:

```bash
cat > .env << 'EOF'
JWT_SECRET=your-strong-secret-here
POSTGRES_PASSWORD=your-db-password
EOF

docker compose up -d
```

> **Login to the registry first** to pull private images:
> ```bash
> docker login registry.edsuwarna.xyz -u deploy
> ```
> You'll need registry credentials configured. Contact your admin for access.

This boots up: PostgreSQL, Redis, Backend, Frontend, and Zot (registry).

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

### 6. Self-Server (Host Auto-Registration)

On startup, Anjungan can auto-detect and register the host server where it runs — no manual add needed. See [docs/self-server.md](self-server.md) for details.

To enable, ensure your `docker-compose.yml` has:

```yaml
backend:
  environment:
    SELF_SERVER_ENABLED: "true"
    SELF_HOST_NETWORK: "host.docker.internal"
  group_add:
    - "988"  # docker GID — run `getent group docker | cut -d: -f3` to verify
  volumes:
    - /var/run/docker.sock:/var/run/docker.sock:rw
```

## CI/CD

The project uses GitHub Actions for Docker builds:

- **Push to `main`** → builds & pushes images with `main-latest` + `main-{sha}` tags
- **Tag push `v*`** → builds & pushes with `release-latest` + version tags

Images are pushed to the Zot registry at `registry.edsuwarna.xyz`.
