# Anjungan — Development Roadmap

> **Vision:** A dual-role platform that serves both **Infrastructure Engineers** (managing servers, containers, networking) and **Application Developers** (deploying services, managing releases, observing health) — all through a unified dashboard with role-based access.

---

## 🎯 Platform Philosophy

Anjungan is designed to bridge two worlds:

| Peran | Fokus | Contoh Tugas |
|-------|-------|-------------|
| **Infra Engineer** | Server & infrastructure management | Add VPS, monitor resources, manage Docker, SSH access, audit logs |
| **Developer** | Application lifecycle management | Deploy service, view logs, rollback, check health, manage secrets |

Setiap user bisa punya satu atau kedua peran tergantung RBAC. Dashboard-nya berubah sesuai role — infra engineer liat server metrics, developer liat service health.

---

## 📦 Phase 1 — Foundation (Current)

> **Status: 🟡 Partial** — Kerangka dashboard udah jadi, backend dasar udah jalan

### Backend (Go — ✅ Running)
- [x] Auth: login, register, JWT + refresh token
- [x] TOTP 2FA
- [x] OIDC / SSO provider
- [x] RBAC (roles & permissions)
- [x] User management CRUD
- [x] Dashboard summary API
- [x] PostgreSQL + Redis

### Frontend (SvelteKit — 🟡 Partial)
- [x] Layout: sidebar + topbar + main area
- [x] Login page
- [x] Dark/light mode toggle
- [x] Responsive (mobile collapsible sidebar)
- [ ] **Dashboard real** — connect to backend API, show live data
- [ ] **Halaman Servers** — list, add, manage servers
- [ ] **Halaman Containers** — list containers per server
- [ ] **Halaman Registry** — Docker registry browser
- [ ] **Halaman Repositories** — Git repo integration
- [ ] **Halaman Deployments** — deployment history & status
- [ ] **Admin panel** — user, role, permission management

### Theme & UI
- [x] Emerald green primary palette (Tailwind config)
- [x] CSS variable system for theming
- [ ] **Emerald-green sidebar** — active state, header accent, hover effects biar ga putih doang
- [ ] Consistent component design system

---

## 🏗️ Phase 2 — Platform Engineering (IDP Core)

> **Goal:** Transform dari server management tool jadi Internal Developer Platform

### Service Catalog & Developer Portal
- [ ] **Service Registry** — setiap aplikasi punya halaman sendiri: health, owner, tech stack, dependencies
- [ ] **Environment Management** — Dev → Staging → Production, beda config per env
- [ ] **Self-Service Actions** — deploy, restart, rollback dari dashboard (no SSH)
- [ ] **Deployment History** — timeline: siapa deploy, versi apa, commit, rollback
- [ ] **Ownership & Team Mapping** — setiap service punya owner/team, filter by team

### Deployment Pipeline
- [ ] **GitHub/GitLab Webhook** — auto-trigger deployment pas push
- [ ] **Pipeline Visualization** — stages: build → test → deploy staging → approve → production
- [ ] **Manual Approval Gates** — staging auto, production butuh approve
- [ ] **Rollback Button** — one-click rollback ke versi sebelumnya
- [ ] **Deployment Templates** — Docker Compose, Docker Swarm, direct SSH pull

### Scaffolding
- [ ] **Service Scaffolder** — "Create new service" → pilih template (FastAPI, Go, Node.js) → langsung dapet repo + CI + deployment config
- [ ] **Config Generator** — generate docker-compose, nginx config, env vars sesuai environment

---

## 🔐 Phase 3 — Security & Governance

- [ ] **Centralized Vault** — simpan secrets (API keys, DB passwords), inject ke env pas deploy
- [ ] **Secret Rotation** — jadwal rotasi otomatis + audit akses
- [ ] **Environment-specific Configs** — beda config per env tanpa hardcode
- [ ] **Policy Engine** — enforce aturan: "all services must have healthcheck", "staging DB must be smaller than production"
- [ ] **Change Management** — developer submit change request → approval → deploy
- [ ] **Deployment Freeze** — set periode freeze, system reject deployment

---

## 📊 Phase 4 — Observability & Intelligence

- [ ] **Service Dependency Graph** — visual map: Service A → DB → Redis → Service B
- [x] **Health Dashboard** — per-service: uptime, response time charts, error rate via uptime monitoring (F1-F10)
- [x] **Alert Routing** — service down → notify via Telegram/Discord/Slack/Webhook (F5)
- [x] **Response Time Stats** — min/avg/max/p95 per 24h/7d/30d (F9)
- [x] **Incident Timeline** — auto-group consecutive down/error, paginated timeline (F10)
- [ ] **SLO / SLI Tracking** — apakah service memenuhi target uptime/response time
- [ ] **Centralized Logs per Service** — filter log by service name, bukan per-system
- [ ] **Postmortem Template** — standard template untuk blameless postmortem

---

## 🧩 Phase 5 — Ecosystem & Extensibility

- [ ] **Developer API** — deploy via `curl`, integrasi CI/CD external
- [ ] **CLI Tool** — `anjungan deploy my-service --env production`
- [ ] **Webhook Outgoing** — kirim event ke external system pas deployment berhasil/gagal
- [ ] **Plugin System** — biar team bisa extend sendiri
- [ ] **Terraform/OpenTofu Integration** — manage IaC dari dashboard

---

## 🧠 Design Principles

1. **Emerald-first** — `#10b981` bukan cuma aksen, tapi jadi identitas visual yang dominan
2. **Developer experience** — setiap action harus ≤2 klik, ga perlu buka terminal
3. **Role-aware** — UI berubah sesuai role: infra engineer liat hardware, developer liat service
4. **Self-service** — developer bisa deploy sendiri tanpa minta tolong infra team
5. **Observable by default** — setiap deployment, setiap action, harus ada log + metric

---

## Cara Berkontribusi ke Roadmap Ini

Roadmap ini live document — bisa berubah sesuai prioritas. Diskusi dan update dilakukan lewat:
- Issue GitHub
- Diskusi di grup
- Pull request langsung ke file ini

---

*Last updated: June 2026*
