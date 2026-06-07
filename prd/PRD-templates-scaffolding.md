# Anjungan — PRD: Service Templates & Scaffolding

> **Version:** 1.0
> **Status:** 🔴 Not Implemented — Proposed for Phase 2
> **Author:** Endang Suwarna
> **Last Updated:** June 5, 2026

---

## 1. Executive Summary

### Problem Statement

Creating a new service from scratch is currently manual every time:
1. `mkdir project` + init code
2. Write `Dockerfile` from an existing template (or copy-paste from an old project)
3. Write `docker-compose.yml` — fill in service name, port, env, Traefik labels, volume
4. Push to GitHub
5. Open Dokploy → deploy new stack
6. Check logs — debug if it fails

Tedious, repetitive, and easy to miss steps (like forgetting to add Traefik labels or health check endpoints).

**Templates solving this:**
- **One click → service ready to deploy** — pick template, fill name, domain, deploy
- **Consistency** — all services use the same folder structure + compose + Dockerfile
- **No copy-paste errors** — automatic variable injection (no hardcoded IPs or ports left behind)
- **Custom template** — if there's an existing project, it can be saved as a template for the next project

### Target Audience

- **Endang himself** — creates new services every 2-3 weeks (whatilearned, opsterm, stem-lab, etc.)
- **Developer (future)** — self-service scaffold without asking the infra engineer

### Goals

| Goal | Metric |
|------|--------|
| New service from click → deploy | < 30 seconds (exclude build time) |
| Consistent project structure | 100% of services use the same template |
| Custom template from existing project | < 1 minute to save as template |
| Built-in templates ready to use | Minimum 4 templates: Go, SvelteKit, FastAPI, Static |

### Non-Goals

- ❌ Not a full code generator — just scaffold project skeleton + infra config
- ❌ Not a CI/CD pipeline builder — just compose + Dockerfile, GitHub Action workflow later
- ❌ Not a no-code app builder — still need to write your own logic

---

## 2. Product Overview

### This Feature in the Context of Anjungan

```
┌─ Pilih Template ─┐    ┌─ Isi Vars ──────┐    ┌─ Scaffold ──────────────┐
│                   │    │                  │    │                         │
│  Go API + PG      │    │ 🔤 Nama: my-api  │    │ ✅ ~/projects/my-api/   │
│  SvelteKit + Node │ →  │ 🌐 Domain: ...   │ →  │ ✅ my-api/docker-compose│
│  FastAPI + SQLite │    │ 🔑 DB Pass: ***  │    │ ✅ my-api/Dockerfile    │
│  Static HTML SPA  │    │ ☑ Deploy now    │    │ ✅ Deploy ke Dokploy   │
│  Custom           │    │                  │    │ ✅ Webhook auto         │
└───────────────────┘    └──────────────────┘    └─────────────────────────┘
```

### Concept: Template = Project Blueprint

A template is not just a compose file — it's the **entire project structure**:

```
my-api/                          ← project name
├── docker-compose.yml            ← Main compose (with placeholders)
├── deploy/                       ← Optional: deployment config
│   └── nginx.conf
├── Dockerfile                    ← Optional: can be from an image registry
├── .env.example                  ← Example env vars
└── src/                          ← Optional: source code skeleton
    ├── main.go
    ├── go.mod
    ├── handlers/
    └── migrations/
```

Each placeholder (`{{service_name}}`, `{{domain}}`, `{{db_password}}`) is replaced during scaffold.

---

## 3. Feature Specifications

> **Priority Key:** P0 = Must have (blocker), P1 = Should have (important), P2 = Nice to have

### F1 — Template Registry

| | |
|---|---|
| **Priority** | P0 |
| **Backend** | New `service_templates` table. Columns: id, name (unique), label, description, icon (emoji), category (backend/frontend/fullstack/static), engine (go/sveltekit/fastapi/static/custom), definition JSONB (template.yaml content — but stored parsed + reconstructed as JSON), tags (text[]), is_builtin (boolean — false for custom templates), created_by (FK users), created_at, updated_at. CRUD: `GET/POST/PUT/DELETE /api/v1/templates`. Filter: `?category=`, `?search=`. |
| **Frontend** | Route `/infra/templates`. Grid card — each template: big icon, name, description, tags chips. Highlight effect on hover. "+ New Template" button. Click card → detail + scaffold flow. Tab: "Built-in" (4 default) + "Custom" (user-created). |
| **UX** | Cards use consistent icons per engine: ⚡ Go, 🟦 SvelteKit, 🐍 FastAPI, 📄 Static, ＋ Custom. Tags as chips (Go, Postgres, Docker). Hover → emerald border + name changes color. |

**Built-in Templates (required in v1):**

| Template | Engine | Image | Stack Tags |
|----------|--------|-------|-----------|
| ⚡ Go API + Postgres | Go 1.23 | golang:1.23-alpine | Go, Postgres 16, REST, Chi |
| 🟦 SvelteKit + Node | Svelte 5 | node:22-alpine | Svelte 5, Node 22, Docker |
| 🐍 FastAPI + SQLite | Python 3.12 | python:3.12-slim | FastAPI, SQLite, Pydantic |
| 📄 Static HTML SPA | Static | nginx:alpine | Vanilla JS, Tailwind CDN |

---

### F2 — Template Engine (Scaffold)

| | |
|---|---|
| **Priority** | P0 |
| **Backend** | Scaffold engine: `POST /api/v1/templates/{id}/scaffold`. Input: `{ service_name, domain, env_vars: {...}, deploy_now: bool }`. Process: (1) Read template definition + files from storage. (2) Replace placeholders (`{{service_name}}`, `{{domain}}`, etc.). (3) Generate random secrets (db_password, jwt_secret). (4) Write files to `~/projects/{{service_name}}/`. (5) Generate `docker-compose.yml` with proper service name, domain, port, Traefik labels. (6) If `deploy_now=true` → post to Dokploy API or compose up. (7) Return output path + deploy status. |
| **Frontend** | Wizard 3-step: (1) Select template → (2) Fill vars → (3) Confirm + deploy. Step 2: dynamic form based on template definition (service_name, domain, db_user, etc.). Step 3: summary — project path, compose preview, deploy status (pending/done). |
| **UX** | Progress stepper: "Template → Configure → Deploy". Form validation inline. Service name: lowercase, no spaces. Domain: domain format validation. Password auto-generate with reveal toggle. Live compose preview in step 3. Loading state during scaffold + deploy. |

### F3 — Variable & Secrets Injection

| | |
|---|---|
| **Priority** | P1 |
| **Backend** | Template definition has a `variables` array — each variable has: name, label, type (string/secret/select), required, default, pattern (regex). Auto-generate for type=secret. Inject into Secrets backend (F3.1 vault) if type=secret. Template engine: Go `text/template` parsing + replace. Escaping: URL encode for connection strings. |
| **Frontend** | Dynamic form — render from `definition.variables`. Each type has different input: string → text input, secret → password + generate button + strength indicator, select → dropdown. Required fields marked with *. |
| **UX** | Secret fields: auto-generate 32-char random string + copy button. DB password auto-generated unless user overrides. Validation before submit. |

### F4 — Deploy Integration

| | |
|---|---|
| **Priority** | P1 |
| **Backend** | Two deploy modes: (1) **Dokploy API** — post compose to Dokploy via API. (2) **Direct SSH** — copy files to target server via SCP + `docker compose up -d`. Endpoint: `POST /api/v1/templates/{id}/scaffold-and-deploy`. Option: `deploy_method` (dokploy/ssh). Return: deploy_log, service_url, status. |
| **Frontend** | Toggle: "Deploy now" checkbox (default: checked). If deploy: show progress log (real-time via WebSocket or polling). Done → link to service. No deploy → "Scaffold only — deploy later from /deployments". |
| **UX** | Deploy progress: ⏳ Pulling image → ⏳ Creating containers → ✅ Service live at notes.edsuwarna.id. If failed: error message + link to logs. Quick retry button. |

### F5 — Custom Template (Save Existing Service)

| | |
|---|---|
| **Priority** | P1 |
| **Backend** | `POST /api/v1/services/{id}/save-as-template`. Read service's compose + env → extract as template definition. Replace concrete values with placeholders: service name → `{{service_name}}`, domain → `{{domain}}`, etc. Save as new template with `is_builtin=false`. |
| **Frontend** | Button "Save as Template" on service detail page. Modal: template name, description, icon picker, tags. Preview: list of files that will become the template. Confirm → template saved. |
| **UX** | Auto-detect placeholders from common values: domain names, port numbers, service names. User can override variable name. Success toast + link to template. |

### F6 — Template Versioning (Optional)

| | |
|---|---|
| **Priority** | P2 |
| **Backend** | Template version table: `template_versions` — id, template_id, version (semver), definition JSONB, changelog, created_at. `GET /api/v1/templates/{id}/versions`. `POST /api/v1/templates/{id}/versions`. Scaffold with specific version: `?version=1.2.0`. Default: latest. |
| **Frontend** | Version history on template detail: timeline, version number, changelog. "Use v1.1" button — scaffold with old version. |
| **UX** | Latest marked. If there's a breaking change — show warning when using old version. |

---

## 4. Template Definition Format (`template.yaml`)

Each template has a YAML definition that describes what will be scaffolded:

```yaml
name: go-api-postgres
label: Go API + Postgres
description: "REST API with Chi router, Postgres, migrations, health check"
icon: ⚡
version: 1.0.0

category: backend
engine: go

images:
  app: golang:1.23-alpine
  db: postgres:16-alpine

variables:
  - name: service_name
    label: Service Name
    type: string
    required: true
    pattern: "^[a-z0-9-]+$"
    placeholder: "my-api"
  - name: domain
    label: Domain
    type: string
    required: false
    placeholder: "my-api.edsuwarna.id"
  - name: app_port
    label: Application Port
    type: number
    default: 8080
  - name: db_user
    label: Database Username
    type: string
    default: "app"
  - name: db_password
    label: Database Password
    type: secret
    auto_generate: true
    length: 32
  - name: jwt_secret
    label: JWT Secret
    type: secret
    auto_generate: true
  - name: environment
    label: Environment
    type: select
    options: ["development", "staging", "production"]
    default: "development"

tags:
  - Go 1.23
  - Postgres 16
  - Docker
  - REST API

files:
  - source: docker-compose.yml
    target: "{{service_name}}/docker-compose.yml"
    template: true
  - source: Dockerfile
    target: "{{service_name}}/Dockerfile"
    template: true
  - source: main.go
    target: "{{service_name}}/cmd/server/main.go"
    template: true
  - source: handlers/health.go
    target: "{{service_name}}/internal/handlers/health.go"
    template: true
  - source: .env.example
    target: "{{service_name}}/.env"
    template: true
  - source: migrations/001_init.sql
    target: "{{service_name}}/migrations/001_init.sql"
    template: true
```

### Placeholder Convention

Placeholders in template files use `{{variable_name}}` — just like Go templates:

```
# docker-compose.yml (template)
services:
  {{service_name}}-api:
    build:
      context: .
      dockerfile: Dockerfile
    ports:
      - "{{app_port}}"
    environment:
      - DATABASE_URL=postgresql://{{db_user}}:{{db_password}}@db:5432/{{service_name}}
      - JWT_SECRET={{jwt_secret}}
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.{{service_name}}.rule=Host(`{{domain}}`)"
```

---

## 5. API Design

### New Endpoints

```go
// === Templates ===
GET    /api/v1/templates                               // List templates (?category=&search=&builtin=)
POST   /api/v1/templates                               // Create template (built-in or custom)
GET    /api/v1/templates/{id}                          // Template detail (definition, files, version)
PUT    /api/v1/templates/{id}                          // Update template
DELETE /api/v1/templates/{id}                          // Delete template
POST   /api/v1/templates/{id}/scaffold                 // Scaffold only (generate files)
POST   /api/v1/templates/{id}/scaffold-and-deploy      // Scaffold + deploy
GET    /api/v1/templates/{id}/preview                  // Preview compose YAML (dry-run)

// === Custom Template from Service ===
POST   /api/v1/services/{id}/save-as-template          // Save existing service as template

// === Template Versions ===
GET    /api/v1/templates/{id}/versions                 // List versions
POST   /api/v1/templates/{id}/versions                 // Create new version
GET    /api/v1/templates/{id}/versions/{version}       // Get specific version

// === Deploy Status (from scaffold) ===
GET    /api/v1/deployments/{id}/log                    // Deploy progress (real-time)
```

---

## 6. Database Schema

### New Tables

```sql
-- 000016_create_service_templates.up.sql
CREATE TABLE service_templates (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name VARCHAR(100) NOT NULL UNIQUE,
  label VARCHAR(255) NOT NULL,
  description TEXT,
  icon VARCHAR(10) DEFAULT '📦',
  category VARCHAR(50) NOT NULL,            -- backend, frontend, fullstack, static
  engine VARCHAR(50) NOT NULL,              -- go, sveltekit, fastapi, static, custom
  definition JSONB NOT NULL,                 -- parsed template.yaml content
  tags TEXT[] DEFAULT '{}',
  is_builtin BOOLEAN DEFAULT FALSE,
  version_current VARCHAR(20) DEFAULT '1.0.0',
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW()
);

-- 000017_create_template_versions.up.sql (optional — P2)
CREATE TABLE template_versions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  template_id UUID NOT NULL REFERENCES service_templates(id) ON DELETE CASCADE,
  version VARCHAR(20) NOT NULL,
  definition JSONB NOT NULL,
  changelog TEXT,
  created_at TIMESTAMP DEFAULT NOW(),
  UNIQUE(template_id, version)
);

-- 000018_create_scaffold_logs.up.sql
CREATE TABLE scaffold_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  template_id UUID REFERENCES service_templates(id),
  template_version VARCHAR(20),
  service_name VARCHAR(255) NOT NULL,
  vars_used JSONB,                          -- snapshot — variable values used
  deploy_status VARCHAR(50),                -- pending, scaffolding, deploying, success, failed
  deploy_url TEXT,                          -- link to service if successful
  project_path TEXT,                        -- ~/projects/my-api/
  error_message TEXT,
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMP DEFAULT NOW()
);
```

---

## 7. Storage (Where Files Live)

Template files are stored on Anjungan's filesystem:

```
~/.anjungan/
└── templates/
    ├── go-api-postgres/           ← Built-in (bundled with binary)
    │   ├── template.yaml
    │   ├── docker-compose.yml
    │   ├── Dockerfile
    │   ├── main.go
    │   ├── handlers/
    │   └── migrations/
    ├── sveltekit-node/
    ├── fastapi-sqlite/
    ├── static-html/
    └── custom/                    ← Custom template (user-created)
        ├── my-service/
        │   ├── template.yaml
        │   └── ...
```

Scaffold output:

```
~/projects/
├── my-api/                        ← Generated from template
│   ├── docker-compose.yml
│   ├── Dockerfile
│   ├── cmd/server/main.go
│   ├── internal/handlers/health.go
│   └── .env
├── other-project/
└── ...
```

---

## 8. UX Flow Detail

### Flow: Scaffold + Deploy New Service

```
1. Open /infra/templates
2. View grid — 4 template cards + 1 custom (dashed border)
3. Click "⚡ Go API + Postgres"
4. Step 1 — Configure:
   [Service Name]   my-api
   [Domain]         my-api.edsuwarna.id     (optional)
   [DB User]        app                     (auto-filled)
   [DB Password]    ••••••••••••••••••••    (auto-generated 32 chars) [Reveal]
   [JWT Secret]     ••••••••••••••••••••    (auto-generated)
   [Environment]    [development ▼]
5. Options:
   ☑ Create repository (GitHub)
   ☑ Deploy immediately
   ☐ CI/CD pipeline (coming soon)
6. Click "Scaffold & Deploy"
7. Progress:
   ✅ Creating project structure → ~/projects/my-api/
   ✅ Generating docker-compose.yml
   ✅ Writing .env
   ✅ Pushing to GitHub: edsuwarna/my-api
   ✅ Deploying via Dokploy...
   ✅ Service live at my-api.edsuwarna.id
8. Done → link to service detail + link to project folder
```

### Flow: Save Existing Service as Template

```
1. Open service detail (e.g., whatilearned)
2. Click ⋮ → "Save as Template"
3. Modal:
   Template Name: whatilearned
   Description: Go API + Postgres with Redis caching
   Icon: [⚡ ▼]
   Tags: Go, Postgres, Redis, Chi
4. Preview: list of files that will be templatized
5. Click "Save"
6. Template appears in /infra/templates tab "Custom"
7. When used: variables {{service_name}}, {{domain}}, {{db_password}} are replaced automatically
```

### Flow: Scaffold Only (No Deploy)

```
1. Select template → configure → uncheck "Deploy immediately"
2. Click "Scaffold"
3. Result: ~/projects/my-api/ with all files
4. Notification: "Service scaffolded — deploy later from /deployments"
5. Can edit files first, then deploy manually
```

---

## 9. Non-Functional Requirements

| Requirement | Target |
|-------------|--------|
| **Scaffold time** | < 3 seconds (file write + variable injection) |
| **Deploy time** | Depends on image pull + build — target < 2 minutes |
| **Built-in templates** | Minimum 4 (Go, SvelteKit, FastAPI, Static) |
| **Template size limit** | Max 10MB per template definition (files + metadata) |
| **Concurrent scaffold** | 3 concurrent — queue via asynq |
| **File safety** | If target path already exists → error (won't overwrite existing project) |
| **Secret generation** | crypto/rand — 32+ chars, alphanumeric + special |

---

## 10. Implementation Roadmap

### 🟢 Phase 1 — Core Scaffold

**Goal:** Able to scaffold + deploy new service from template

| Order | Feature | Effort | Dependencies |
|-------|---------|--------|-------------|
| 1 | `service_templates` table + migration | 0.5 day | — |
| 2 | Template CRUD backend | 1 day | #1 |
| 3 | Template engine (Go text/template + placeholder) | 1 day | #2 |
| 4 | Built-in template files (Go API + PG) | 1 day | #3 |
| 5 | Template grid UI | 0.5 day | #2 |
| 6 | Scaffold wizard (configure step) | 1.5 days | #5 |
| 7 | File generator + output to ~/projects/ | 0.5 day | #3 |
| **Total** | | **6 days** | |

### 🟡 Phase 2 — Multi-Template + Deploy

**Goal:** All built-in templates ready + deploy integration

| Order | Feature | Effort | Dependencies |
|-------|---------|--------|-------------|
| 8 | Built-in: SvelteKit + Node template | 1 day | #4 |
| 9 | Built-in: FastAPI + SQLite template | 0.5 day | #4 |
| 10 | Built-in: Static HTML SPA template | 0.5 day | #4 |
| 11 | Deploy integration (Dokploy API) | 1.5 days | #7 |
| 12 | Deploy progress UI (real-time log) | 1 day | #11 |
| 13 | Scaffold logs table + history | 0.5 day | #7 |
| **Total** | | **5 days** | |

### 🔵 Phase 3 — Custom Template + Polish

**Goal:** User-defined template from existing service

| Order | Feature | Effort | Dependencies |
|-------|---------|--------|-------------|
| 14 | Save-as-template backend (placeholder auto-detect) | 1.5 days | #7 |
| 15 | Custom template UI + modal | 0.5 day | #14 |
| 16 | Template category tabs (Built-in vs Custom) | 0.5 day | #15 |
| 17 | Dynamic form from template definition | 1 day | #3 |
| 18 | Template preview (YAML render) | 0.5 day | #7 |
| **Total** | | **4 days** | |

### ⚪ Phase 4 — Enhancement

**Goal:** Production-ready

| Order | Feature | Effort |
|-------|---------|--------|
| 19 | Template versioning | 1.5 days |
| 20 | GitHub repo create on scaffold | 1 day |
| 21 | CI/CD template (GitHub Actions workflow) | 0.5 day |
| 22 | Variable validation (regex pattern) | 0.5 day |
| 23 | Export/import template | 0.5 day |
| **Total** | | **4 days** |

---

## 11. Glossary

| Term | Definition |
|------|------------|
| **Template** | Project blueprint — compose, Dockerfile, source code skeleton with placeholders |
| **Scaffold** | Process to generate files from template → replace placeholders → output to ~/projects/ |
| **Placeholder** | `{{variable_name}}` — replaced with concrete values during scaffold |
| **Built-in Template** | Template bundled with Anjungan binary — cannot be deleted |
| **Custom Template** | Template saved from an existing service — can be edited/deleted |
| **Engine** | Tech stack template (go, sveltekit, fastapi, static) — determines image + Dockerfile |
| **Variable** | Input field in wizard form — each template has its own variable definitions |
| **Secret Variable** | Variable with type=secret — auto-generated + stored in vault, not in compose |

## 12. References

- [PRD.md](./PRD.md) — Main Anjungan PRD (Phase 2 IDP Core: F2.2 environments, F2.5 GitHub integration)
- [PRD-domain-management.md](./PRD-domain-management.md) — Domain & multi-server routing
- [PRD-repositories-deployments.md](./PRD-repositories-deployments.md) — Repos & deployments
- [DECISIONS.md](../docs/DECISIONS.md) — Architectural decisions
