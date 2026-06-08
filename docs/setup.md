# Development Setup

## Prerequisites

- **Go** 1.25+
- **Node.js** 22+ (with npm)
- **Docker** & **Docker Compose**
- **Air** (for backend hot-reload): `go install github.com/air-verse/air@latest`

## 1. Clone & Configure

```bash
git clone git@github.com:edsuwarna/anjungan.git
cd anjungan
cp .env.example .env
```

Edit `.env` if needed (defaults work for local dev).

## 2. Start Dependencies (PostgreSQL + Redis)

```bash
make dev-db
# or: docker compose up -d postgres redis
```

## 3. Run Backend (with hot-reload)

```bash
make dev-backend
```

The server starts on `localhost:8080` and auto-runs database migrations on startup.

## 4. Run Frontend (dev server)

```bash
make dev-frontend
```

Opens on `localhost:5173` with HMR. Proxies `/api/*` to the backend.

## 5. Full Stack (Docker only)

```bash
make dev
# or: docker compose up -d
```

Runs everything in containers. Frontend at `http://localhost`, backend at `http://localhost:8080`.

## Makefile Commands

| Command | Description |
|---------|-------------|
| `make dev` | Start all services |
| `make dev-backend` | Backend with hot-reload |
| `make dev-frontend` | Frontend dev server |
| `make dev-db` | Start databases only |
| `make build` | Build Docker images |
| `make up` | Start Docker services |
| `make down` | Stop Docker services |
| `make restart` | Restart Docker services |
| `make logs` | Tail all service logs |
| `make migrate-up` | Restart backend to run pending migrations |
| `make migrate-create` | Create new migration files |
| `make build-backend` | Build Go binary locally |
| `make build-frontend` | Build frontend SPA locally |
| `make go-test` | Run Go tests |
| `make go-lint` | Run `go vet` |
| `make go-tidy` | Tidy Go modules |

## Environment Variables

Key variables (see `.env.example` for all):

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTGRES_USER` | `anjungan` | Database user |
| `POSTGRES_PASSWORD` | `anjungan` | Database password |
| `POSTGRES_DB` | `anjungan` | Database name |
| `JWT_SECRET` | `change-me-in-production` | JWT signing key |
| `GITHUB_TOKEN` | — | GitHub PAT for repository features |
| `REGISTRY_URL` | `http://zot:5000` | Internal Zot URL |
| `REGISTRY_EXTERNAL_URL` | `registry.anjungan.io` | External Zot URL |
| `LOG_LEVEL` | `info` | Log level: debug, info, warn, error |
| `SELF_SERVER_ENABLED` | `false` | Auto-register host server (set `"true"` to enable) |
| `SELF_SERVER_NAME` | `anjungan-host` | Display name for the self-server |
| `SELF_HOST_NETWORK` | — | Host IP from inside container (e.g. `host.docker.internal`) |

## Connecting to the Database

```bash
make db
# or: docker compose exec postgres psql -U anjungan -d anjungan
```
