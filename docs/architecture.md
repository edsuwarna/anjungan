# Architecture

## High-Level Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    Frontend (SvelteKit)                       │
│                  Emerald theme · Dark/Light                   │
└──────────────────────┬──────────────────────────────────────┘
                       │ HTTP /api/*
┌──────────────────────▼──────────────────────────────────────┐
│                   Backend (Go / Chi)                          │
│                                                               │
│  ┌──────┐ ┌────────┐ ┌──────────┐ ┌──────────┐ ┌────────┐   │
│  │ Auth │ │ Infra  │ │ Container│ │ Deploy   │ │ Repo   │   │
│  └──────┘ └────────┘ └──────────┘ └──────────┘ └────────┘   │
│  ┌────────┐ ┌────────┐ ┌──────────┐ ┌──────────┐            │
│  │Admin   │ │Comply  │ │ Registry │ │Settings  │            │
│  └────────┘ └────────┘ └──────────┘ └──────────┘            │
│                                                               │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │  Infrastructure Layer: Docker · SSH · Zot Registry     │  │
│  └─────────────────────────────────────────────────────────┘  │
└──────────────────────┬──────────────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────────────┐
│  PostgreSQL 17                    Redis 7                     │
│  (persistent data, migrations)    (sessions, cache, rate)     │
└─────────────────────────────────────────────────────────────┘
```

## Backend (Go)

**Pattern:** Modular monolith — each domain is a self-contained package inside `internal/`.

### Router Structure (Chi)

```
GET  /health

/api/v1/
├── /auth              (no auth required)
│   ├── POST /login
│   ├── POST /register
│   ├── POST /refresh
│   ├── POST /verify-2fa
│   ├── GET  /me
│   └── POST /logout
└── (auth middleware ↓)
    ├── /servers       — Server CRUD + SSH terminal WebSocket
    ├── /ssh-keys      — SSH key management
    ├── /containers    — Docker container listing & stats
    ├── /registry      — Zot registry integration
    ├── /repositories  — GitHub/Forgejo repository integration
    ├── /deployments   — Deployment lifecycle
    ├── /compliance    — Security scanning & checks
    ├── /admin         — User management & audit logs
    ├── /settings      — Application settings (thresholds)
    └── GET /dashboard — Aggregated summary
```

### Middleware

- Request ID, Real IP, Logger, Recoverer (Chi defaults)
- 30s request timeout
- JWT authentication (required for all `/api/v1/*` except `/auth`)
- Admin role gate on `/admin` routes
- Rate limiting on `/auth/login` and `/auth/register`

## Frontend (SvelteKit)

- **SSR:** Disabled — runs as pure SPA (static adapter)
- **Build output:** Static files served by nginx
- **Routing:** Client-side, nginx fallback to `index.html`
- **Styling:** Tailwind CSS with Emerald (`#10b981`) color scheme
- **Icons:** Iconify (unified icon framework)

### Frontend Architecture

```
frontend/src/
├── lib/
│   ├── api.svelte.js         — API client (fetch wrapper)
│   ├── auth.svelte.js        — Auth store (JWT, login state)
│   ├── thresholds.svelte.js  — Compliance thresholds store
│   └── components/
│       ├── ui/               — Reusable UI primitives
│       └── ...               — Feature components
└── routes/
    ├── +layout.svelte        — App shell (sidebar, theme)
    ├── +page.svelte          — Dashboard overview
    ├── servers/              — Server management
    ├── containers/           — Container list & detail
    ├── registry/             — Image browser
    ├── compliance/           — Compliance dashboard & scans
    ├── deployment/           — Deployment tracking
    ├── admin/                — Admin panel
    └── settings/             — Settings page
```

The frontend proxies `/api/*` requests to the backend via nginx (`proxy_pass http://backend:8080`). Static assets (`/_app/`) are cached with 1-year immutable headers.

## Container Registry (Zot)

Zot runs as a Docker container with:
- **htpasswd authentication** (users managed by Anjungan)
- **Access control** with roles: `readonly`, `deploy`, `admin`
- **Auto-sync:** When registry users are created/deleted in Anjungan, htpasswd is regenerated and Zot restarted
- **Self-service:** Every Anjungan user auto-gets a personal registry account
