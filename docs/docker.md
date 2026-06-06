# Docker Build & Deployment

## Building Images

### Backend (Go multi-stage build)

```bash
# Using Dockerfile.backend (context: ./backend)
docker compose build backend

# Or manually:
docker build -t anjungan-backend -f Dockerfile.backend ./backend
```

The backend Dockerfile:
1. **Build stage** — `golang:1.25-alpine` compiles with `-ldflags="-s -w"` for smaller binary
2. **Runtime stage** — `alpine:3.20` includes `ca-certificates`, `openssh-client`, `docker-cli`, `curl`
3. Runs as non-root user `anjungan` (UID 1000)

### Frontend (npm build + nginx)

```bash
# Using Dockerfile.frontend (context: project root)
docker compose build frontend

# Or manually:
docker build -t anjungan-frontend -f Dockerfile.frontend .
```

The frontend Dockerfile:
1. **Build stage** — `node:22-alpine` runs `npm install --legacy-peer-deps` then `npm run build`
2. **Runtime stage** — `nginx:1.27-alpine` serves the static build output
3. Nginx config: SPA fallback (`try_files $uri /index.html`), immutable cache for `/_app/` assets, proxy `/api/*` to backend

## Tagging Convention

Images are tagged for the private Zot registry at `reg.edsuwarna.xyz`.

| Tag Pattern | When | Example |
|-------------|------|---------|
| `release-latest` | Tag push `v*` | `reg.edsuwarna.xyz/anjungan-backend:release-latest` |
| `v0.3.0` | Tag push `v*` | `reg.edsuwarna.xyz/anjungan-backend:v0.3.0` |
| `v0.3.0+release` | Tag push `v*` | `reg.edsuwarna.xyz/anjungan-backend:v0.3.0+release` |
| `main-latest` | Push to `main` | `reg.edsuwarna.xyz/anjungan-frontend:main-latest` |
| `main-{sha}` | Push to `main` | `reg.edsuwarna.xyz/anjungan-frontend:main-a1b2c3d` |

## CI/CD Pipeline (GitHub Actions)

The workflow `.github/workflows/docker-build-push.yml`:

1. **On push to `main`** — builds both images, pushes with `main-latest` + `main-{sha}` tags
2. **On tag push `v*`** — builds both images, pushes with `release-latest` + `v{version}` + `v{version}+release` tags

### Prerequisites for CI

Set these GitHub Actions secrets:

| Secret | Description |
|--------|-------------|
| `REGISTRY_URL` | Zot registry URL (e.g. `reg.edsuwarna.xyz`) |
| `REGISTRY_USER` | Registry deploy user |
| `REGISTRY_PASSWORD` | Registry deploy password |

## Rebuild & Deploy on Server

```bash
# Pull latest code
git pull origin main

# Rebuild and restart
docker compose build
docker compose up -d --force-recreate

# Verify
docker compose ps
docker compose logs --tail=20 backend
```
