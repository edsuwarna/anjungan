# Anjungan — PRD: Service Templates & Scaffolding

> **Version:** 1.0
> **Status:** Draft
> **Author:** Endang Suwarna
> **Last Updated:** June 5, 2026

---

## 1. Executive Summary

### Problem Statement

Bikin service baru dari nol sekarang tiap kali manual:
1. `mkdir project` + init code
2. Tulis `Dockerfile` dari template yang udah ada (atau copy paste dari project lama)
3. Tulis `docker-compose.yml` — isi service name, port, env, labels Traefik, volume
4. Push ke GitHub
5. Buka Dokploy → deploy stack baru
6. Cek log — debug kalo gagal

Ribet, repetitive, dan gampang lupa step (kayak lupa nambah Traefik labels atau health check endpoint).

**Templates solving this:**
- **Satu klik → service siap deploy** — pilih template, isi nama, domain, deploy
- **Consistency** — semua service pake struktur folder + compose + Dockerfile yang sama
- **No copy-paste errors** — variable injection otomatis (ga ada hardcode IP atau port tersisa)
- **Custom template** — kalo ada project yang udah jalan, bisa disimpen jadi template buat project berikutnya

### Target Audience

- **Endang sendiri** — bikin service baru tiap 2-3 minggu (whatilearned, opsterm, stem-lab, dll)
- **Developer (future)** — self-service scaffold tanpa tanya infra engineer

### Goals

| Goal | Metric |
|------|--------|
| New service dari klik → deploy | < 30 detik (exclude build time) |
| Consistent project structure | 100% service pake template yang sama |
| Custom template dari existing project | < 1 menit simpen jadi template |
| Built-in templates siap pakai | Minimal 4 template: Go, SvelteKit, FastAPI, Static |

### Non-Goals

- ❌ Bukan code generator lengkap — cuma scaffold project skeleton + infra config
- ❌ Bukan CI/CD pipeline builder — compose + Dockerfile doang, workflow GitHub Action belakangan
- ❌ Bukan no-code app builder — tetep perlu nulis logic sendiri

---

## 2. Product Overview

### Fitur Ini Dalam Konteks Anjungan

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

### Konsep: Template = Project Blueprint

Template bukan cuma compose file — dia **seluruh struktur project**:

```
my-api/                          ← nama project
├── docker-compose.yml            ← Main compose (dengan placeholder)
├── deploy/                       ← Opsional: deployment config
│   └── nginx.conf
├── Dockerfile                    ← Opsional: bisa dari image registry
├── .env.example                  ← Example env vars
└── src/                          ← Opsional: source code skeleton
    ├── main.go
    ├── go.mod
    ├── handlers/
    └── migrations/
```

Setiap placeholder (`{{service_name}}`, `{{domain}}`, `{{db_password}}`) di-replace pas scaffold.

---

## 3. Feature Specifications

> **Priority Key:** P0 = Must have (blocker), P1 = Should have (important), P2 = Nice to have

### F1 — Template Registry

| | |
|---|---|
| **Priority** | P0 |
| **Backend** | `service_templates` table baru. Kolom: id, name (unique), label, description, icon (emoji), category (backend/frontend/fullstack/static), engine (go/sveltekit/fastapi/static/custom), definition JSONB (template.yaml content — tapi disimpen parsed + di-reconstruct jadi JSON), tags (text[]), is_builtin (boolean — false buat custom template), created_by (FK users), created_at, updated_at. CRUD: `GET/POST/PUT/DELETE /api/v1/templates`. Filter: `?category=`, `?search=`. |
| **Frontend** | Route `/infra/templates`. Grid card — tiap template: icon big, name, description, tags chips. Highlight effect on hover. "+ New Template" button. Klik card → detail + scaffold flow. Tab: "Built-in" (4 default) + "Custom" (user-created). |
| **UX** | Cards pake icon consistent per engine: ⚡ Go, 🟦 SvelteKit, 🐍 FastAPI, 📄 Static, ＋ Custom. Tags sebagai chips (Go, Postgres, Docker). Hover → border emerald + nama berubah warna. |

**Built-in Templates (wajib di v1):**

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
| **Backend** | Scaffold engine: `POST /api/v1/templates/{id}/scaffold`. Input: `{ service_name, domain, env_vars: {...}, deploy_now: bool }`. Process: (1) Baca template definition + files dari storage. (2) Replace placeholders (`{{service_name}}`, `{{domain}}`, dll). (3) Generate random secrets (db_password, jwt_secret). (4) Tulis file ke `~/projects/{{service_name}}/`. (5) Generate `docker-compose.yml` with proper service name, domain, port, Traefik labels. (6) Kalo `deploy_now=true` → post ke Dokploy API atau compose up. (7) Return output path + deploy status. |
| **Frontend** | Wizard 3-step: (1) Pilih template → (2) Isi vars → (3) Konfirmasi + deploy. Step 2: form dinamis sesuai template definition (service_name, domain, db_user, dll). Step 3: summary — project path, compose preview, deploy status (pending/done). |
| **UX** | Progress stepper: "Template → Configure → Deploy". Form validation inline. Service name: lowercase, no spaces. Domain: validasi format domain. Password auto-generate dengan reveal toggle. Live compose preview di step 3. Loading state pas scaffold + deploy. |

### F3 — Variable & Secrets Injection

| | |
|---|---|
| **Priority** | P1 |
| **Backend** | Template definition punya `variables` array — tiap variable punya: name, label, type (string/secret/select), required, default, pattern (regex). Auto-generate buat type=secret. Inject ke Secrets backend (F3.1 vault) kalo type=secret. Template engine: Go `text/template` parsing + replace. Escaping: URL encode untuk connection strings. |
| **Frontend** | Dynamic form — render dari `definition.variables`. Tiap type beda input: string → text input, secret → password + generate button + strength indicator, select → dropdown. Required fields marked with *. |
| **UX** | Secret fields: auto-generate 32-char random string + copy button. DB password auto-generated kecuali user override. Validation sebelum submit. |

### F4 — Deploy Integration

| | |
|---|---|
| **Priority** | P1 |
| **Backend** | Dua mode deploy: (1) **Dokploy API** — post compose ke Dokploy via API. (2) **Direct SSH** — copy file ke server target via SCP + `docker compose up -d`. Endpoint: `POST /api/v1/templates/{id}/scaffold-and-deploy`. Option: `deploy_method` (dokploy/ssh). Return: deploy_log, service_url, status. |
| **Frontend** | Toggle: "Deploy now" checkbox (default: checked). Kalo deploy: tampilkan progress log (real-time via WebSocket atau polling). Done → link ke service. Ga deploy → "Scaffold only — deploy later from /deployments". |
| **UX** | Deploy progress: ⏳ Pulling image → ⏳ Creating containers → ✅ Service live at notes.edsuwarna.id. Kalo gagal: error message + link ke logs. Quick retry button. |

### F5 — Custom Template (Save Existing Service)

| | |
|---|---|
| **Priority** | P1 |
| **Backend** | `POST /api/v1/services/{id}/save-as-template`. Baca service's compose + env → extract jadi template definition. Ganti value konkret jadi placeholder: service name → `{{service_name}}`, domain → `{{domain}}`, dll. Simpan sebagai template baru dengan `is_builtin=false`. |
| **Frontend** | Button "Save as Template" di service detail page. Modal: template name, description, icon picker, tags. Preview: daftar file yang bakal jadi template. Konfirmasi → template tersimpan. |
| **UX** | Auto-detect placeholder dari value umum: domain names, port numbers, service names. User bisa override variable name. Success toast + link ke template. |

### F6 — Template Versioning (Optional)

| | |
|---|---|
| **Priority** | P2 |
| **Backend** | Template version table: `template_versions` — id, template_id, version (semver), definition JSONB, changelog, created_at. `GET /api/v1/templates/{id}/versions`. `POST /api/v1/templates/{id}/versions`. Scaffold pake version tertentu: `?version=1.2.0`. Default: latest. |
| **Frontend** | Version history di template detail: timeline, version number, changelog. "Use v1.1" button — scaffold pake versi lama. |
| **UX** | Latest marked. Kalo ada breaking change — kasih warning pas pake versi lama. |

---

## 4. Template Definition Format (`template.yaml`)

Setiap template punya definisi YAML yang describe apa yang bakal di-scaffold:

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

Placeholder di file template pake `{{variable_name}}` — persis kayak Go template:

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
  deploy_url TEXT,                          -- link ke service kalo sukses
  project_path TEXT,                        -- ~/projects/my-api/
  error_message TEXT,
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMP DEFAULT NOW()
);
```

---

## 7. Storage (Where Files Live)

Template files disimpen di filesystem Anjungan:

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
1. Buka /infra/templates
2. Lihat grid — 4 template card + 1 custom (dashed border)
3. Klik "⚡ Go API + Postgres"
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
6. Klik "Scaffold & Deploy"
7. Progress:
   ✅ Creating project structure → ~/projects/my-api/
   ✅ Generating docker-compose.yml
   ✅ Writing .env
   ✅ Pushing to GitHub: edsuwarna/my-api
   ✅ Deploying via Dokploy...
   ✅ Service live at my-api.edsuwarna.id
8. Done → link ke service detail + link ke project folder
```

### Flow: Save Existing Service as Template

```
1. Buka service detail (misal: whatilearned)
2. Klik ⋮ → "Save as Template"
3. Modal:
   Template Name: whatilearned
   Description: Go API + Postgres with Redis caching
   Icon: [⚡ ▼]
   Tags: Go, Postgres, Redis, Chi
4. Preview: daftar file yang bakal di-template-kan
5. Klik "Save"
6. Template muncul di /infra/templates tab "Custom"
7. Pas dipake: variable {{service_name}}, {{domain}}, {{db_password}} diganti otomatis
```

### Flow: Scaffold Only (No Deploy)

```
1. Pilih template → configure → uncheck "Deploy immediately"
2. Klik "Scaffold"
3. Hasil: ~/projects/my-api/ dengan semua file
4. Notif: "Service scaffolded — deploy later from /deployments"
5. Bisa edit file dulu, baru deploy manual
```

---

## 9. Non-Functional Requirements

| Requirement | Target |
|-------------|--------|
| **Scaffold time** | < 3 detik (file write + variable injection) |
| **Deploy time** | Tergantung image pull + build — target < 2 menit |
| **Built-in templates** | Minimal 4 (Go, SvelteKit, FastAPI, Static) |
| **Template size limit** | Max 10MB per template definition (files + metadata) |
| **Concurrent scaffold** | 3 concurrent — queue via asynq |
| **File safety** | Kalo target path udah ada → error (ga overwrite existing project) |
| **Secret generation** | crypto/rand — 32+ chars, alphanumeric + special |

---

## 10. Implementation Roadmap

### 🟢 Phase 1 — Core Scaffold

**Goal:** Bisa scaffold + deploy service baru dari template

| Order | Feature | Effort | Dependencies |
|-------|---------|--------|-------------|
| 1 | `service_templates` table + migration | 0.5 hari | — |
| 2 | Template CRUD backend | 1 hari | #1 |
| 3 | Template engine (Go text/template + placeholder) | 1 hari | #2 |
| 4 | Built-in template files (Go API + PG) | 1 hari | #3 |
| 5 | Template grid UI | 0.5 hari | #2 |
| 6 | Scaffold wizard (configure step) | 1.5 hari | #5 |
| 7 | File generator + output to ~/projects/ | 0.5 hari | #3 |
| **Total** | | **6 hari** | |

### 🟡 Phase 2 — Multi-Template + Deploy

**Goal:** Semua built-in template siap + deploy integration

| Order | Feature | Effort | Dependencies |
|-------|---------|--------|-------------|
| 8 | Built-in: SvelteKit + Node template | 1 hari | #4 |
| 9 | Built-in: FastAPI + SQLite template | 0.5 hari | #4 |
| 10 | Built-in: Static HTML SPA template | 0.5 hari | #4 |
| 11 | Deploy integration (Dokploy API) | 1.5 hari | #7 |
| 12 | Deploy progress UI (real-time log) | 1 hari | #11 |
| 13 | Scaffold logs table + history | 0.5 hari | #7 |
| **Total** | | **5 hari** | |

### 🔵 Phase 3 — Custom Template + Polish

**Goal:** User-defined template dari existing service

| Order | Feature | Effort | Dependencies |
|-------|---------|--------|-------------|
| 14 | Save-as-template backend (placeholder auto-detect) | 1.5 hari | #7 |
| 15 | Custom template UI + modal | 0.5 hari | #14 |
| 16 | Template category tabs (Built-in vs Custom) | 0.5 hari | #15 |
| 17 | Dynamic form from template definition | 1 hari | #3 |
| 18 | Template preview (YAML render) | 0.5 hari | #7 |
| **Total** | | **4 hari** | |

### ⚪ Phase 4 — Enhancement

**Goal:** Production-ready

| Order | Feature | Effort |
|-------|---------|--------|
| 19 | Template versioning | 1.5 hari |
| 20 | GitHub repo create on scaffold | 1 hari |
| 21 | CI/CD template (GitHub Actions workflow) | 0.5 hari |
| 22 | Variable validation (regex pattern) | 0.5 hari |
| 23 | Export/import template | 0.5 hari |
| **Total** | | **4 hari** |

---

## 11. Glossary

| Term | Definition |
|------|------------|
| **Template** | Blueprint project — compose, Dockerfile, source code skeleton dengan placeholder |
| **Scaffold** | Proses generate file dari template → replace placeholder → output ke ~/projects/ |
| **Placeholder** | `{{variable_name}}` — diganti dengan nilai konkret pas scaffold |
| **Built-in Template** | Template yang dibundling sama binary Anjungan — ga bisa di-delete |
| **Custom Template** | Template hasil save dari existing service — bisa di-edit/delete |
| **Engine** | Tech stack template (go, sveltekit, fastapi, static) — nentuin image + Dockerfile |
| **Variable** | Input field di form wizard — tiap template punya variable definition sendiri |
| **Secret Variable** | Variable bertype=secret — auto-generate + disimpen di vault, bukan di compose |

## 12. References

- [PRD.md](./PRD.md) — Main Anjungan PRD (Phase 2 IDP Core: F2.2 environments, F2.5 GitHub integration)
- [PRD-domain-management.md](./PRD-domain-management.md) — Domain & multi-server routing
- [PRD-repositories-deployments.md](./PRD-repositories-deployments.md) — Repos & deployments
- [DECISIONS.md](../DECISIONS.md) — Architectural decisions
