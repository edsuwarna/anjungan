# Anjungan — PRD: Registry (Zot Integration)

> **Version:** 1.0
> **Status:** ✅ Fully Implemented — all backend + frontend features complete
> **Author:** Endang Suwarna
> **Last Updated:** June 5, 2026

---

## 1. Executive Summary

### Problem Statement

Anjungan already has a **Zot registry** as a self-hosted container image registry. But access has only been through the CLI (`docker pull/push registry.edsuwarna.id/...`) or directly via the Zot API. Developers who want to:

- View the list of available images
- Browse tags in an image
- View image details (config, layers, history)
- Delete unused images/tags
- Trigger garbage collection
- Manage user credentials for docker login

...must access the Zot API directly or SSH into the server.

**Registry feature solves this:**
- **Browse repos + tags** from the Anjungan UI
- **Image detail** — config, layers, history — visual
- **Delete & GC** — clean up unused images from UI
- **User management** — create registry users, reset password, htpasswd sync
- **Self-service credentials** — each Anjungan user automatically gets registry credentials

### Target Audience

- **Endang** — manage daily image registry
- **Developer** — browse images, pull tags, push new images
- **CI/CD** — robot accounts for automation

### Goals

| Goal | Metric |
|------|--------|
| Browse all repos from UI | ✅ Done |
| View image details (config + layers) | ✅ Done |
| Delete image/tag + GC | ✅ Done |
| User registry management (CRUD) | ✅ Done |
| Self-service credential per user | ✅ Done |

### Current Status (June 2026)

✅ **Registry feature is FULLY implemented** in Anjungan. This PRD documents existing features + future roadmap.

---

## 2. Product Overview

### Architecture

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
| **Backend** | `GET /api/v1/registry/repos` — List repos from Zot API. Paginated (page + limit). Enrich each repo with tag count via parallel manifest fetch. Sort by: last_modified, name. `GET /api/v1/registry/repos/{name}/tags` — List tags. Detail per tag: digest, size, media_type, config_digest, layers count, history count, created_at. |
| **Frontend** | Route `/registry`. Table of repos: name, tag count, last modified. Click → expand accordion tag list: each tag has digest (truncated), size, created time. Search/filter tag. |
| **UX** | Loading skeleton while fetching. Empty state: "No images — push your first image with `docker push registry.edsuwarna.id/...`" with copy command. Error state: "Registry unreachable — check if Zot is running." |

### F2 — Image Detail

| | |
|---|---|
| **Priority** | P0 |
| **Status** | ✅ **Done** |
| **Backend** | `GET /api/v1/registry/repos/{name}/{tag}` — Fetch manifest + config + layers from Zot API. Parse config JSON (env, cmd, entrypoint, exposed ports, volumes, labels, created_at). Parse layers: digest, size, command (from history), created_at. If no config (OCI index/manifest list), return error. |
| **Frontend** | Route `/registry/[name]/[tag]`. Tabs: **Config** — env vars, cmd, entrypoint, ports, volumes, labels, architecture, OS, created. **Layers** — timeline: each layer has a size bar + command (from `history`). Accordion per layer: raw config diff, created timestamp. **History** — list per layer: created by, created since, comment. |
| **UX** | Loading: skeleton + spinner. Config tab — table layout (key: value). Layers — horizontal bar chart per layer (proportional size). History — timeline UI with created time, command, comment. Error if image is not OCI/Docker v2 format. |

### F3 — Image Deletion & Garbage Collection

| | |
|---|---|
| **Priority** | P0 |
| **Status** | ✅ **Done** |
| **Backend** | `DELETE /api/v1/registry/repos/{name}/manifests/{digest}` — Delete manifest by digest (admin). `DELETE /api/v1/registry/repos/{name}/tags/{tag}` — Delete tag (admin). `POST /api/v1/registry/gc` — Trigger GC on Zot. Via SSH to registry server: `curl -X POST http://localhost:5000/v2/_zot_ext/gc` or direct API call. Return GC status: started/running/completed/failed + log. |
| **Frontend** | Delete button on each tag (hover). Confirmation modal: "Delete tag `v1.0.0`? This action cannot be undone." Delete multiple tags (checkbox). GC button in registry page header: "Run Garbage Collection" → confirm → progress log. |

### F4 — Registry Credentials & Self-Service

| | |
|---|---|
| **Priority** | P0 |
| **Status** | ✅ **Done** |
| **Backend** | `POST /api/v1/registry/my-credentials` — Auto-create registry user for the logged-in Anjungan user. Generate username `u-{uid[:8]}` + random password. `POST /api/v1/registry/my-credentials/reset-password` — Reset password self-service. Save in `registry_users` table + sync to htpasswd file via SSH. |
| **Frontend** | Card on registry page: "Your Registry Credentials" — username (read-only), password (masked 🔴••••• + reveal toggle). Copy button. "Reset Password" button. Login command: `docker login registry.edsuwarna.id -u <username>` — copyable. |
| **UX** | Password only displayed once at create/reset — tooltip: "Save this password — it won't be shown again after page reload." Copy all command to clipboard. |

### F5 — Registry User Management (Admin)

| | |
|---|---|
| **Priority** | P1 |
| **Status** | ✅ **Done** |
| **Backend** | Full CRUD: `GET/POST/PUT/DELETE /api/v1/registry/users`. `POST /api/v1/registry/users/{id}/reset-password`. `POST /api/v1/registry/sync-htpasswd` — manual sync htpasswd file to registry server. Linked to Anjungan user: `anjungan_user_id` nullable — if linked, auto-create on first login. |
| **Frontend** | Route `/registry` → "Users" tab (admin only). Table: username, linked anjungan user, created, last login, status. Create user modal: username, link to Anjungan user (optional), password auto-generate. Reset password modal. Delete confirmation. |
| **UX** | Admin-only — hidden if not admin. Sync htpasswd status: "Last synced: 2m ago ✅" or "Sync needed!" warning badge. |

### F6 — Registry Config (Admin)

| | |
|---|---|
| **Priority** | P2 |
| **Status** | ✅ **Done** |
| **Backend** | `GET /api/v1/registry/config` — Return Zot config from config file (parsed) or env. Includes: public URL, storage backend (R2/local), GC policy, auth type. |
| **Frontend** | Read-only config display at the bottom of the registry page. |

---

## 4. Future Roadmap

### F7 — Webhook Notifications (P2 - 🔴 Planned)

| | |
|---|---|
| **Backend** | Webhook receiver: `POST /api/v1/registry/webhooks` — receive notifications from Zot (push, pull, delete). Save to `registry_events` table. Notify to chat (Telegram): "Image `my-api:v2.1.0` pushed to registry." |
| **Frontend** | Webhook log page: event timeline. Filter by repo, event type. |

### F8 — Multi-Registry Support (P2 - 🔴 Planned)

| | |
|---|---|
| **Backend** | `registry_instances` table — support multiple registry endpoints. Can add other registries (Docker Hub proxy, GHCR, other self-hosted). `GET /api/v1/registry/instances` — list all. `POST /api/v1/registry/instances` — add instance (name, url, auth). |
| **Frontend** | Registry switcher in sidebar/header. Switch registry → browse images from another registry. |

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
| **Backend** | Integrate Zot's built-in `zot-ext-cve` or call Trivy on image push. Save CVE results in `registry_cve` table. |
| **Frontend** | Vulnerability badge on each tag: "🔴 12 Critical". Click → detail CVE list (affected package, fix version, severity). |

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
1. Open /registry
2. View list of all repos: whatilearned, anjungan-backend, opsterm, nginx
3. Click "whatilearned" → accordion expand: tag list (v3.2.1, v3.2.0, v3.1.0)
4. Click tag "v3.2.1" → open /registry/whatilearned/v3.2.1
5. View Config tab: env vars, cmd, ports, labels
6. Switch to Layers tab: 12 layers, size distribution bar chart
7. Back to tag list → check "v3.1.0" → click "Delete Tags"
8. Modal: "Delete tag v3.1.0?" → Confirm → ✅ Deleted
9. Click "Run Garbage Collection" → progress: "GC started → cleaning blobs..."
10. ✅ Freed 245MB disk space
```

---

## 8. Non-Functional Requirements

| Requirement | Target | Status |
|-------------|--------|--------|
| Repo list load | < 2s (100 repos) | ✅ Tested |
| Image detail load | < 3s (50 layers) | ✅ Tested |
| Delete operation | < 1s | ✅ |
| GC trigger | < 5s (start signal) | ✅ |
| htpasswd sync | < 3s | ✅ |
| Concurrent user credential | 100+ users | ✅ |
| Zot proxy latency | < 100ms overhead | ✅ |

---

## 9. References

- [PRD.md](./PRD.md) — Main Anjungan PRD (Phase 2 Registry)
- [PRD-repositories-deployments.md](./PRD-repositories-deployments.md)
- [DECISIONS.md](../docs/DECISIONS.md)
