# Anjungan — PRD: Registry (Zot Integration)

> **Version:** 1.0
> **Status:** Draft ✅ Implemented
> **Author:** Endang Suwarna
> **Last Updated:** June 5, 2026

---

## 1. Executive Summary

### Problem Statement

Anjungan udah punya **Zot registry** sebagai container image registry self-hosted. Tapi aksesnya selama ini lewat CLI doang (`docker pull/push registry.edsuwarna.id/...`) atau langsung API Zot. Developer yang mau:

- Lihat daftar image apa aja yang tersedia
- Browse tags di suatu image
- Lihat detail image (config, layers, history)
- Delete image/tag yang ga dipake
- Trigger garbage collection
- Manage user credentials buat docker login

...harus akses langsung ke Zot API atau SSH ke server.

**Registry feature solving this:**
- **Browse repos + tags** dari UI Anjungan
- **Image detail** — config, layers, history — visual
- **Delete & GC** — clean up image ga kepake dari UI
- **User management** — buat user registry, reset password, htpasswd sync
- **Self-service credentials** — tiap user Anjungan auto dapet credential registry

### Target Audience

- **Endang** — manage image registry harian
- **Developer** — browse image, pull tag, push image baru
- **CI/CD** — robot accounts buat automation

### Goals

| Goal | Metric |
|------|--------|
| Browse semua repos dari UI | ✅ Done |
| Lihat detail image (config + layers) | ✅ Done |
| Delete image/tag + GC | ✅ Done |
| User registry management (CRUD) | ✅ Done |
| Self-service credential per user | ✅ Done |

### Current Status (June 2026)

✅ **Registry feature is FULLY implemented** di Anjungan. PRD ini dokumentasi fitur existing + future roadmap.

---

## 2. Product Overview

### Arsitektur

```
Anjungan UI                        Zot Registry
┌──────────────┐                  ┌──────────────────┐
│ Registry Page │ ──API/S3──▶      │ Zot API           │
│ - Repo list   │    (via backend) │ - /v2/_catalog    │
│ - Tag browser │                  │ - /v2/{repo}/tags │
│ - Image detail│                  │ - /v2/{repo}/man  │
│ - User mgmt   │                  │ - /v2/{repo}/blobs│
│ - GC trigger  │                  └──────────────────┘
└──────┬───────┘                           │
       │                                    │
       │ htpasswd sync via SSH              │
       ▼                                    ▼
┌────────────────────────────────────────────────────┐
│  Anjungan Backend (registry handler)                │
│  - Proxy Zot API (auth passthrough)                 │
│  - Registry users CRUD (registry_users table)       │
│  - htpasswd file management                         │
│  - GC orchestration (Zot API + fallback)            │
└────────────────────────────────────────────────────┘
```

### Implemented Detail

| Component | Status | Detail |
|-----------|--------|--------|
| Backend handler | ✅ Done | 1085 lines — 15 endpoints |
| User handler | ✅ Done | 375 lines — 5 endpoints |
| DB migration | ✅ Done | 000012 + 000013 |
| Frontend registry page | ✅ Done | 982 lines — repo list, tags, credentials |
| Frontend image detail | ✅ Done | 480 lines — config, layers, history tabs |
| Sidebar link | ✅ Done | Under "Artifact" category |

---

## 3. Feature Specifications

> **Legend:** ✅ Implemented | 🟡 Partial | 🔴 Planned

### F1 — Repository Browser

| | |
|---|---|
| **Priority** | P0 |
| **Status** | ✅ **Done** |
| **Backend** | `GET /api/v1/registry/repos` — List repos dari Zot API. Paginated (page + limit). Enrich tiap repo dengan tag count via parallel manifest fetch. Sort by: last_modified, name. `GET /api/v1/registry/repos/{name}/tags` — List tags. Detail per tag: digest, size, media_type, config_digest, layers count, history count, created_at. |
| **Frontend** | Route `/registry`. Tabel repos: name, tag count, last modified. Click → expand accordion tag list: tiap tag punya digest (truncated), size, created time. Search/filter tag. |
| **UX** | Loading skeleton pas fetch. Empty state: "No images — push your first image with `docker push registry.edsuwarna.id/...`" dengan copy command. Error state: "Registry unreachable — check if Zot is running." |

### F2 — Image Detail

| | |
|---|---|
| **Priority** | P0 |
| **Status** | ✅ **Done** |
| **Backend** | `GET /api/v1/registry/repos/{name}/{tag}` — Fetch manifest + config + layers dari Zot API. Parse config JSON (env, cmd, entrypoint, exposed ports, volumes, labels, created_at). Parse layers: digest, size, command (from history), created_at. Kalo ga ada config (OCI index/manifest list), return error. |
| **Frontend** | Route `/registry/[name]/[tag]`. Tabs: **Config** — env vars, cmd, entrypoint, ports, volumes, labels, architecture, OS, created. **Layers** — timeline: tiap layer punya size bar + command (from `history`). Accordion per layer: raw config diff, created timestamp. **History** — list per layer: created by, created since, comment. |
| **UX** | Loading: skeleton + spinner. Config tab — table layout (key: value). Layers — horizontal bar chart per layer (proportional size). History — timeline UI with created time, command, comment. Error kalo image bukan OCI/Docker v2 format. |

### F3 — Image Deletion & Garbage Collection

| | |
|---|---|
| **Priority** | P0 |
| **Status** | ✅ **Done** |
| **Backend** | `DELETE /api/v1/registry/repos/{name}/manifests/{digest}` — Delete manifest by digest (admin). `DELETE /api/v1/registry/repos/{name}/tags/{tag}` — Delete tag (admin). `POST /api/v1/registry/gc` — Trigger GC on Zot. Via SSH ke server registry: `curl -X POST http://localhost:5000/v2/_zot_ext/gc` atau direct API call. Return GC status: started/running/completed/failed + log. |
| **Frontend** | Delete button tiap tag (hover). Confirmation modal: "Delete tag `v1.0.0`? This action cannot be undone." Delete multiple tags (checkbox). GC button di header registry page: "Run Garbage Collection" → confirm → progress log. |

### F4 — Registry Credentials & Self-Service

| | |
|---|---|
| **Priority** | P0 |
| **Status** | ✅ **Done** |
| **Backend** | `POST /api/v1/registry/my-credentials` — Auto-create registry user buat Anjungan user yang login. Generate username `u-{uid[:8]}` + random password. `POST /api/v1/registry/my-credentials/reset-password` — Reset password self-service. Simpan di `registry_users` table + sync ke htpasswd file via SSH. |
| **Frontend** | Card di registry page: "Your Registry Credentials" — username (read-only), password (masked 🔴••••• + reveal toggle). Copy button. "Reset Password" button. Login command: `docker login registry.edsuwarna.id -u <username>` — copyable. |
| **UX** | Password cuma ditampilkan sekali pas create/reset — tooltip: "Save this password — it won't be shown again after page reload." Copy all command to clipboard. |

### F5 — Registry User Management (Admin)

| | |
|---|---|
| **Priority** | P1 |
| **Status** | ✅ **Done** |
| **Backend** | Full CRUD: `GET/POST/PUT/DELETE /api/v1/registry/users`. `POST /api/v1/registry/users/{id}/reset-password`. `POST /api/v1/registry/sync-htpasswd` — manual sync sync htpasswd file ke server registry. Linked ke Anjungan user: `anjungan_user_id` nullable — kalo ter-link, auto-create pas login pertama. |
| **Frontend** | Route `/registry` → "Users" tab (admin only). Table: username, linked anjungan user, created, last login, status. Create user modal: username, link ke Anjungan user (optional), password auto-generate. Reset password modal. Delete confirmation. |
| **UX** | Admin-only — hidden kalo bukan admin. Sync htpasswd status: "Last synced: 2m ago ✅" atau "Sync needed!" warning badge. |

### F6 — Registry Config (Admin)

| | |
|---|---|
| **Priority** | P2 |
| **Status** | ✅ **Done** |
| **Backend** | `GET /api/v1/registry/config` — Return Zot config dari config file (parsed) atau env. Includes: public URL, storage backend (R2/local), GC policy, auth type. |
| **Frontend** | Read-only config display di registry page bawah. |

---

## 4. Future Roadmap

### F7 — Webhook Notifications (P2 - 🔴 Planned)

| | |
|---|---|
| **Backend** | Webhook receiver: `POST /api/v1/registry/webhooks` — terima notifikasi dari Zot (push, pull, delete). Simpan di `registry_events` table. Notifikasi ke chat (Telegram): "Image `my-api:v2.1.0` pushed to registry." |
| **Frontend** | Webhook log page: event timeline. Filter by repo, event type. |

### F8 — Multi-Registry Support (P2 - 🔴 Planned)

| | |
|---|---|
| **Backend** | `registry_instances` table — support multiple registry endpoints. Bisa nambah registry lain (Docker Hub proxy, GHCR, self-hosted lain). `GET /api/v1/registry/instances` — list all. `POST /api/v1/registry/instances` — add instance (name, url, auth). |
| **Frontend** | Registry switcher di sidebar/header. Pindah registry → browse image dari registry lain. |

### F9 — Registry Sync / Mirror (P3 - 🔴 Planned)

| | |
|---|---|
| **Backend** | Sync config: source registry (Docker Hub), target (Zot), image list, schedule (cron). Background sync worker via asynq. `POST /api/v1/registry/sync` — trigger sync job. |
| **Frontend** | Sync config UI: source, target, schedule. Sync log: status, last sync, images synced, bytes transferred. |

### F10 — Cleanup Policies (P3 - 🔴 Planned)

| | |
|---|---|
| **Backend** | Policy engine: `DELETE images where tag = latest AND age > 30d`. `KEEP last 10 tags per repo`. `DELETE untagged images`. Cron-based. Config per repo. |
| **Frontend** | Policy editor: select repo, condition (age, tag count, regex), action (delete, gc). Policy list: name, scope, rule, schedule, last run. |

### F11 — Built-in Vulnerability Scan (P3 - 🔴 Planned)

| | |
|---|---|
| **Backend** | Integrasi Zot's built-in `zot-ext-cve` atau call Trivy pas image push. Simpan CVE results di `registry_cve` table. |
| **Frontend** | Vulnerability badge di tiap tag: "🔴 12 Critical". Click → detail CVE list (affected package, fix version, severity). |

---

## 5. API Design (Existing)

```go
// === Registry (Implemented) ===
GET    /api/v1/registry/config
GET    /api/v1/registry/my-credentials
POST   /api/v1/registry/my-credentials/reset-password
GET    /api/v1/registry/repos
GET    /api/v1/registry/repos/{name}/tags
GET    /api/v1/registry/repos/{name}/{tag}
DELETE /api/v1/registry/repos/{name}/manifests/{digest}
DELETE /api/v1/registry/repos/{name}/tags/{tag}
POST   /api/v1/registry/gc
GET    /api/v1/registry/users
POST   /api/v1/registry/users
PUT    /api/v1/registry/users/{id}
DELETE /api/v1/registry/users/{id}
POST   /api/v1/registry/users/{id}/reset-password
POST   /api/v1/registry/sync-htpasswd
```

### Future API

```go
// === Future: Multi-Registry ===
GET    /api/v1/registry/instances
POST   /api/v1/registry/instances
PUT    /api/v1/registry/instances/{id}
DELETE /api/v1/registry/instances/{id}

// === Future: Webhooks ===
GET    /api/v1/registry/webhooks
POST   /api/v1/registry/webhooks
GET    /api/v1/registry/events?instance_id=&repo=

// === Future: Sync ===
POST   /api/v1/registry/sync
GET    /api/v1/registry/sync/jobs
GET    /api/v1/registry/sync/jobs/{id}

// === Future: Cleanup Policies ===
GET    /api/v1/registry/policies
POST   /api/v1/registry/policies
PUT    /api/v1/registry/policies/{id}
DELETE /api/v1/registry/policies/{id}

// === Future: Vulnerability ===
GET    /api/v1/registry/repos/{name}/{tag}/vulns
```

---

## 6. Database Schema

### Existing Tables

```sql
-- 000012: Registry users
CREATE TABLE registry_users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  username VARCHAR(255) NOT NULL UNIQUE,
  password_hash VARCHAR(255) NOT NULL,
  status VARCHAR(20) DEFAULT 'active',
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW()
);

-- 000013: Link registry user to Anjungan user
ALTER TABLE registry_users ADD COLUMN anjungan_user_id UUID REFERENCES users(id);
```

### Future Tables (Roadmap)

```sql
-- Future: Multi-registry
CREATE TABLE registry_instances (
  id UUID PRIMARY KEY,
  name VARCHAR(100) NOT NULL,
  url VARCHAR(512) NOT NULL,
  auth_type VARCHAR(20),                -- none, basic, token
  auth_config JSONB,
  is_default BOOLEAN DEFAULT FALSE,
  created_at TIMESTAMP DEFAULT NOW()
);

-- Future: Registry events
CREATE TABLE registry_events (
  id BIGSERIAL PRIMARY KEY,
  instance_id UUID REFERENCES registry_instances(id),
  event_type VARCHAR(50),               -- push, pull, delete, sync
  repo_name VARCHAR(255),
  tag_name VARCHAR(255),
  digest VARCHAR(255),
  size_bytes BIGINT,
  metadata JSONB,
  created_at TIMESTAMP DEFAULT NOW()
);

-- Future: Cleanup policies
CREATE TABLE registry_policies (
  id UUID PRIMARY KEY,
  instance_id UUID REFERENCES registry_instances(id),
  name VARCHAR(255) NOT NULL,
  scope VARCHAR(50),                    -- repo, tag_pattern, all
  scope_value VARCHAR(255),             -- regex or specific name
  conditions JSONB,                     -- [{field: "age", op: ">", value: 30}]
  action VARCHAR(50),                   -- delete, gc, notify
  schedule VARCHAR(100),                -- cron expression, null = manual
  enabled BOOLEAN DEFAULT TRUE,
  last_run TIMESTAMP,
  created_at TIMESTAMP DEFAULT NOW()
);
```

---

## 7. UX Flow

### Existing Flow: Browse + Delete Image

```
1. Buka /registry
2. Liat daftar semua repos: whatilearned, anjungan-backend, opsterm, nginx
3. Klik "whatilearned" → accordion expand: tag list (v3.2.1, v3.2.0, v3.1.0)
4. Klik tag "v3.2.1" → buka /registry/whatilearned/v3.2.1
5. Liat Config tab: env vars, cmd, ports, labels
6. Switch ke Layers tab: 12 layers, size distribution bar chart
7. Back ke tag list → centang "v3.1.0" → klik "Delete Tags"
8. Modal: "Delete tag v3.1.0?" → Confirm → ✅ Deleted
9. Klik "Run Garbage Collection" → progress: "GC started → cleaning blobs..."
10. ✅ Freed 245MB disk space
```

---

## 8. Non-Functional Requirements

| Requirement | Target | Status |
|-------------|--------|--------|
| Repo list load | < 2 detik (100 repos) | ✅ Tested |
| Image detail load | < 3 detik (50 layers) | ✅ Tested |
| Delete operation | < 1 detik | ✅ |
| GC trigger | < 5 detik (start signal) | ✅ |
| htpasswd sync | < 3 detik | ✅ |
| Concurrent user credential | 100+ users | ✅ |
| Zot proxy latency | < 100ms overhead | ✅ |

---

## 9. References

- [PRD.md](./PRD.md) — Main Anjungan PRD (Phase 2 Registry)
- [PRD-repositories-deployments.md](./PRD-repositories-deployments.md)
- [DECISIONS.md](../DECISIONS.md)
