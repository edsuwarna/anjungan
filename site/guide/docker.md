---
title: Docker
description: Docker build and deployment reference for Anjungan.
---

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
1. **Build stage** — `golang:1.25-alpine` compiles with `-ldflags="-s -w"` for a smaller binary
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
1. **Build stage** — `node:22-alpine` runs `npm install` then `npm run build`
2. **Runtime stage** — `nginx:1.27-alpine` serves the static build output
3. Nginx config: SPA fallback, immutable cache for `/_app/`, proxy `/api/*` to backend

## Tagging Convention

Images are tagged for the private Zot registry at `registry.edsuwarna.xyz`:

| Tag Pattern | When | Example |
|-------------|------|---------|
| `main-latest` | Push to `main` | `registry.edsuwarna.xyz/anjungan-backend:main-latest` |
| `main-{sha}` | Push to `main` | `registry.edsuwarna.xyz/anjungan-backend:main-a1b2c3d` |
| `release-latest` | Tag push `v*` | `registry.edsuwarna.xyz/anjungan-backend:release-latest` |
| `v0.3.0` | Tag push `v*` | `registry.edsuwarna.xyz/anjungan-backend:v0.3.0` |

## CI/CD Pipeline (GitHub Actions)

The workflow `.github/workflows/docker-build-push.yml`:

1. **On push to `main`** — builds both images, pushes with `main-latest` + `main-{sha}` tags
2. **On tag push `v*`** — builds both images, pushes with `release-latest` + version tags

### Prerequisites

Set these GitHub Actions secrets:

| Secret | Description |
|--------|-------------|
| `REGISTRY_URL` | Zot registry URL (e.g. `registry.edsuwarna.xyz`) |
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
