# Anjungan — PRD: Domain Management & Multi-Server Routing

> **Version:** 1.0
> **Status:** 🔴 Not Implemented — Proposed for Phase 2
> **Author:** Endang Suwarna
> **Last Updated:** June 5, 2026

---

## 1. Executive Summary

### Problem Statement

Anjungan runs on **peladen-central** — one server that has a public IP (203.0.113.1). But Endang has **4-5 other servers** (peladen-ml, peladen-cache, peladen-backup) that **don't have public IPs**. They can only be accessed via internal network (10.0.0.0/24).

Currently, applications that need to be accessed from the internet must:
1. Be forwarded manually via internal IP (`10.0.0.2:8080`)
2. SSL cert managed manually
3. No single pane of glass to view *who owns which domain, on which server, and its health status*
4. Every time a domain is added, compose must be edited manually + Traefik labels added

**Domain Management solves this:**
- Anjungan becomes the **central ingress manager** — all domains managed from one dashboard
- Traefik on peladen-central becomes the gateway → forward to any internal server
- SSL auto via Let's Encrypt
- Health check monitoring from Anjungan
- No more SSH-ing for domain matters

### Target Audience

- **Endang** (platform engineer) — stop manual Traefik config
- **Developer** — self-service adding domains for their services
- **Future infra team** — audit trail of who added which domain

### Goals

| Goal | Metric |
|------|--------|
| Add domain from UI → apply to Traefik | < 30 seconds from click → live |
| Register all servers in one place | 5 servers registered |
| View status of all domains on all servers | Health green/red per domain |
| SSL auto-renew without manual intervention | 100% cert auto-renewed |
| Health check remote server from Traefik | Detection < 30s |

### Non-Goals

- ❌ Not a DNS registrar — replacing Cloudflare DNS. Domains still managed in Cloudflare.
- ❌ Not a load balancer replacement — uses existing Traefik.
- ❌ Not a WireGuard manager built-in — tunnel setup remains manual.
- ❌ Not an alternative cert manager — uses existing Traefik + Let's Encrypt.

---

## 2. Product Overview

### This Feature in the Context of Anjungan

```
Internet ──▶ peladen-central (203.0.113.1)
               │
               ├── Traefik (ingress gateway di server A)
               │   ├── notes.edsuwarna.id      → local container   (peladen-central)
               │   ├── app1.edsuwarna.id        → http://10.0.0.2:8080 (peladen-ml)
               │   ├── app2.edsuwarna.id        → http://10.0.0.3:3000 (peladen-cache)
               │   └── registry.edsuwarna.id    → local container   (peladen-central)
               │
               ├── Anjungan Dashboard
               │   └── Domain Management UI ←→ Traefik File Provider (domains.yml)
               │
               └── Internal Network (10.0.0.0/24)
                    ├── peladen-ml    (10.0.0.2) — 🟢 online
                    ├── peladen-cache  (10.0.0.3) — 🟢 online
                    └── peladen-backup (10.0.0.4) — 🔴 offline
```

### Flow: Add Domain

```
User input                    Anjungan                      Traefik
┌──────────────┐           ┌──────────────────┐         ┌──────────────┐
│ Domain: X    │           │ 1. Simpan ke DB   │         │ 5. Auto-reload│
│ Server: ml   │ ────────▶ │ 2. Generate YAML  │ ──────▶│ 6. Request SSL│
│ Port: 8080   │           │ 3. Tulis ke disk  │    YAML │ 7. Route live │
│ SSL: ✅      │           │ 4. Traefik detect │         │               │
└──────────────┘           └──────────────────┘         └──────────────┘
```

### Existing Architecture Prerequisite

This is not a standalone feature — it **attaches to the existing Traefik** already running on peladen-central. Anjungan doesn't need to install a new Traefik. It only needs **Frontend: UI** + **Backend: File Provider generator** + **New DB schema**.

---

## 3. Feature Specifications

> **Priority Key:** P0 = Must have (blocker), P1 = Should have (important), P2 = Nice to have

### Phase 6 — Networking & Ingress

#### F6.1 — Server Registry

| | |
|---|---|
| **Priority** | P0 |
| **Backend** | New `servers_cluster` table (not the existing `servers` — this is for cluster nodes, not SSH targets). Columns: id, name, public_ip, internal_ip, status (online/maintenance/offline), labels (text[]), specs (jsonb: cpu, ram, disk), uptime_seconds, created_at, updated_at. CRUD: `GET/POST/PUT/DELETE /api/v1/cluster/servers`. Health check endpoint: `POST /api/v1/cluster/servers/{id}/health` (ping via internal IP) |
| **Frontend** | Route `/infra/servers`. Grid card layout — each server: status dot 🟢/🟡/🔴, name, public/internal IP, spec, uptime, label badges. "Register New Server" card (dashed border, plus icon). Edit/Delete modal. |
| **UX** | Status dot live-update (15s polling). Click card → expand detail (services running, CPU/RAM mini). Register form: name, internal IP, public IP (optional), spec, labels/tags. Delete confirmation modal. |

**DB Schema:**

```sql
CREATE TABLE cluster_servers (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name VARCHAR(100) NOT NULL UNIQUE,
  public_ip VARCHAR(15),                          -- nullable — hanya server gateway yang punya
  internal_ip VARCHAR(15) NOT NULL,
  status VARCHAR(20) DEFAULT 'offline',           -- online, maintenance, offline
  labels TEXT[] DEFAULT '{}',
  specs JSONB DEFAULT '{}',                       -- {"cpu": 4, "ram": 8, "disk": 40, "disk_unit": "GB"}
  uptime_seconds BIGINT DEFAULT 0,
  last_heartbeat TIMESTAMP,
  is_gateway BOOLEAN DEFAULT FALSE,               -- server A = Traefik + Anjungan
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW()
);
```

---

#### F6.2 — Domain CRUD

| | |
|---|---|
| **Priority** | P0 |
| **Backend** | New `domains` table. Columns: id, domain (unique), cluster_server_id (FK), target_url (VARCHAR), target_port (INT), service_name (VARCHAR — optional display), ssl_enabled, cert_expires_at, basic_auth_enabled, basic_auth_hash, health_check_path, health_check_interval, redirect_www, labels_override (JSONB — extra Traefik labels), status (active/pending_ssl/error/expired), created_at, updated_at. CRUD: `GET/POST/PUT/DELETE /api/v1/domains`. Filter: `?server_id=`, `?status=active`, `?search=domain`. |
| **Frontend** | Route `/infra/domains`. List all domains: domain name, server badge (local/remote → internal IP), SSL status, health status 🟢/🟡/🔴, cert expiry, Edit/Delete. **Gateway diagram card** at the top — visual flow Internet → Traefik → server. **Add Domain** expandable form: domain, service name, server dropdown (from cluster_servers), conditional — if local: port only; if remote: URL + health check. Options: auto SSL, basic auth, www redirect, health check path+interval. Traefik config preview panel (live YAML preview). |
| **UX** | Inline form validation. Domain format check. Server dropdown uses status indicator. Basic auth password auto-generated using htpasswd. Preview YAML updates live when changing fields. Deployment confirmation — "Config will be written to /etc/traefik/dynamic/domains.yml and Traefik will auto-reload." |

**DB Schema:**

```sql
CREATE TABLE domains (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  domain VARCHAR(255) NOT NULL UNIQUE,
  cluster_server_id UUID NOT NULL REFERENCES cluster_servers(id),
  service_name VARCHAR(255),
  target_port INTEGER DEFAULT 80,                 -- untuk local server
  target_url VARCHAR(512),                        -- untuk remote server (http://10.0.0.2:8080)
  ssl_enabled BOOLEAN DEFAULT TRUE,
  cert_expires_at TIMESTAMP,
  basic_auth_enabled BOOLEAN DEFAULT FALSE,
  basic_auth_hash TEXT,
  health_check_path VARCHAR(100) DEFAULT '/health',
  health_check_interval INTEGER DEFAULT 30,
  redirect_www BOOLEAN DEFAULT FALSE,
  labels_override JSONB DEFAULT '{}',
  status VARCHAR(20) DEFAULT 'active',            -- active, pending_ssl, error, expired
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW()
);
```

---

#### F6.3 — Traefik Config Generator

| | |
|---|---|
| **Priority** | P0 |
| **Backend** | Core service: `TraefikConfigGenerator` — reads all domains from DB → generates YAML in Traefik File Provider format. Logic: if `cluster_server.is_gateway = true` → `loadBalancer.servers[0].port = target_port`, if `is_gateway = false` → `loadBalancer.servers[0].url = target_url`. SSL: auto set `tls.certResolver: letsencrypt`. Basic auth: generate middleware with htpasswd format ($$ double dollar!). Health check for remote server: `loadBalancer.healthCheck.path + interval`. Write to `/etc/traefik/dynamic/domains.yml`. Auto-reload Traefik config. |
| **Frontend** | No dedicated page — backend process. But preview is shown in the Add/Edit Domain form (read-only YAML). Validation errors shown inline. |
| **UX** | If YAML is malformed, backend returns error before writing to disk. Rollback to previous config. |

**Generated YAML Format:**

```yaml
# Auto-generated by Anjungan — /etc/traefik/dynamic/domains.yml
http:
  routers:
    notes-https:
      rule: "Host(`notes.edsuwarna.id`)"
      entrypoints: ["websecure"]
      tls:
        certResolver: letsencrypt
      service: notes-svc

    app1-https:
      rule: "Host(`app1.edsuwarna.id`)"
      entrypoints: ["websecure"]
      tls:
        certResolver: letsencrypt
      service: app1-svc
      middlewares: ["app1-health"]

  services:
    notes-svc:
      loadBalancer:
        servers:
          - port: 3000              # local — Traefik resolve via Docker network

    app1-svc:
      loadBalancer:
        servers:
          - url: "http://10.0.0.2:8080"   # remote — forward ke internal IP
        healthCheck:
          path: /health
          interval: 30s
          timeout: 3s

  middlewares:
    app1-health:
      healthCheck:
        path: /health
        interval: 30s
```

---

#### F6.4 — SSL Certificate Monitoring

| | |
|---|---|
| **Priority** | P1 |
| **Backend** | Scan each domain → check `cert_expires_at`. If < 30 days: update status to `expiring_soon`. If expired: status `expired`. Cron job: daily SSL expiry check. Notification trigger if < 14 days. Endpoint: `GET /api/v1/domains/ssl-summary` — count active, expiring_soon, expired. |
| **Frontend** | Badge in domain list: 🟢 Active (68d), 🟡 Expiring (10d), 🔴 Expired. SSL summary card on dashboard: "2 certs expiring within 30 days". Click → filter domain list. |
| **UX** | Badge color matches urgency. If < 30 days show warning icon. If expired — bright red + tooltip "Traefik cert expired — auto-renew via ACME" or "Custom cert — manual renew". |

---

#### F6.5 — Health Check Dashboard

| | |
|---|---|
| **Priority** | P1 |
| **Backend** | Traefik health check status is built-in to the Traefik API. Anjungan reads from Traefik API (`/api/http/services`) to parse health per service. Endpoint: `GET /api/v1/domains/{id}/health` — proxy call to Traefik API or parse from file. If Traefik API is not accessible (Dokploy doesn't expose it), fallback: cron `docker exec traefik traefik healthcheck` parsing. |
| **Frontend** | Each domain in the list has a health badge: 🟢 OK, 🟡 Degraded, 🔴 Down, ⚪ Unknown. Health detail card (optional): last check, response time, status code. Filter domain list by health status. |
| **UX** | Auto-refresh every 30s. If remote server is down → red badge + "Server unreachable" tooltip. If health check path not configured → gray badge "—". |

---

#### F6.6 — WireGuard Integration (Optional)

| | |
|---|---|
| **Priority** | P2 |
| **Backend** | WireGuard status checker: `wg show` parsing. Check interface status, peer handshake, transfer. Endpoint: `GET /api/v1/cluster/servers/{id}/tunnel`. Endpoint: `POST /api/v1/cluster/servers/{id}/tunnel/test` — ping via WireGuard IP. |
| **Frontend** | Tunnel indicator on server card: "WG 🟢" or "WG 🔴". Tunnel detail modal: interface, peer public key, handshake age, transfer in/out, endpoint. |
| **UX** | If the server uses WireGuard, show tunnel status on server card. If not using, hidden. Test tunnel button — ping + latency. |

---

## 4. API Design

### New Endpoints

```go
// === Cluster Servers ===
GET    /api/v1/cluster/servers                    // List all servers
POST   /api/v1/cluster/servers                    // Register new server
GET    /api/v1/cluster/servers/{id}               // Server detail
PUT    /api/v1/cluster/servers/{id}               // Update server
DELETE /api/v1/cluster/servers/{id}               // Remove server
POST   /api/v1/cluster/servers/{id}/health        // Ping health check
POST   /api/v1/cluster/servers/{id}/heartbeat     // Update status + uptime

// === Domains ===
GET    /api/v1/domains                            // List domains (?server_id=&status=&search=)
POST   /api/v1/domains                            // Add domain
GET    /api/v1/domains/{id}                       // Domain detail
PUT    /api/v1/domains/{id}                       // Update domain
DELETE /api/v1/domains/{id}                       // Remove domain
POST   /api/v1/domains/{id}/apply                 // Force regenerate Traefik config
GET    /api/v1/domains/{id}/health                // Health check status
GET    /api/v1/domains/ssl-summary                // SSL expiry summary stats

// === Traefik Config ===
POST   /api/v1/traefik/regenerate                 // Regenerate domains.yml from all DB domains
GET    /api/v1/traefik/config                     // Preview current generated YAML
GET    /api/v1/traefik/status                     // Traefik API proxy: router/service health
```

### Response Format

```json
// GET /api/v1/cluster/servers
{
  "success": true,
  "data": [
    {
      "id": "uuid-1",
      "name": "peladen-central",
      "public_ip": "203.0.113.1",
      "internal_ip": "10.0.0.1",
      "status": "online",
      "labels": ["traefik", "anjungan", "dokploy", "zot"],
      "specs": {"cpu": 4, "ram": 8, "disk": 40, "disk_unit": "GB"},
      "uptime_seconds": 4060800,
      "is_gateway": true,
      "last_heartbeat": "2026-06-05T15:30:00Z"
    },
    {
      "id": "uuid-2",
      "name": "peladen-ml",
      "public_ip": null,
      "internal_ip": "10.0.0.2",
      "status": "online",
      "labels": ["worker", "ml"],
      "specs": {"cpu": 8, "ram": 32, "disk": 100, "disk_unit": "GB"},
      "uptime_seconds": 1036800,
      "is_gateway": false,
      "last_heartbeat": "2026-06-05T15:30:05Z"
    }
  ]
}

// GET /api/v1/domains
{
  "success": true,
  "data": [
    {
      "id": "uuid-10",
      "domain": "app1.edsuwarna.id",
      "server": {
        "id": "uuid-2",
        "name": "peladen-ml",
        "internal_ip": "10.0.0.2"
      },
      "service_name": "app-1-api",
      "target_url": "http://10.0.0.2:8080",
      "ssl_enabled": true,
      "cert_expires_at": "2026-09-20T00:00:00Z",
      "health_status": "ok",
      "status": "active"
    }
  ]
}
```

---

## 5. Database Schema Summary

### New Tables

| Table | Purpose | Key Columns |
|-------|---------|-------------|
| `cluster_servers` | Server registry — semua node di cluster | name, public_ip, internal_ip, status, specs, is_gateway |
| `domains` | Domain routing rules | domain, server_id, target_url, port, ssl, health_check, status |

### Existing Tables Referenced

| Table | Used For |
|-------|----------|
| `users` | Audit log — who added/modified domain |
| `audit_logs` | Records each domain action (create, update, delete, apply) |

### Migration

```sql
-- 000014_create_cluster_servers.up.sql
CREATE TABLE cluster_servers (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name VARCHAR(100) NOT NULL UNIQUE,
  public_ip VARCHAR(15),
  internal_ip VARCHAR(15) NOT NULL,
  status VARCHAR(20) DEFAULT 'offline',
  labels TEXT[] DEFAULT '{}',
  specs JSONB DEFAULT '{}',
  uptime_seconds BIGINT DEFAULT 0,
  last_heartbeat TIMESTAMP,
  is_gateway BOOLEAN DEFAULT FALSE,
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW()
);

-- 000015_create_domains.up.sql
CREATE TABLE domains (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  domain VARCHAR(255) NOT NULL UNIQUE,
  cluster_server_id UUID NOT NULL REFERENCES cluster_servers(id),
  service_name VARCHAR(255),
  target_port INTEGER DEFAULT 80,
  target_url VARCHAR(512),
  ssl_enabled BOOLEAN DEFAULT TRUE,
  cert_expires_at TIMESTAMP,
  basic_auth_enabled BOOLEAN DEFAULT FALSE,
  basic_auth_hash TEXT,
  health_check_path VARCHAR(100) DEFAULT '/health',
  health_check_interval INTEGER DEFAULT 30,
  redirect_www BOOLEAN DEFAULT FALSE,
  labels_override JSONB DEFAULT '{}',
  status VARCHAR(20) DEFAULT 'active',
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW()
);
```

---

## 6. Non-Functional Requirements

| Requirement | Target |
|-------------|--------|
| **Config generation** | < 100ms untuk regenerate semua domain |
| **Traefik reload** | < 1s setelah domains.yml ditulis |
| **SSL cert monitoring** | Daily check, notify if < 14 days |
| **Health check polling** | 30s interval (by Traefik) |
| **File write** | Atomic write — if crash, old config is safe |
| **Rollback** | Backup config before overwrite — restore if error |
| **Concurrent domain operations** | Lock per-write — no race condition |

---

## 7. UX Flow Detail

### Flow: Add Domain (Remote Server)

```
1. Click "+ Add Domain"
2. Fill form:
   [Domain]         app1.edsuwarna.id
   [Service Name]   app-1-api
   [Target Server]  [peladen-ml (10.0.0.2) ▼]     ← from cluster_servers
3. Because server != gateway, form automatically switches to REMOTE mode:
   [Target URL]     http://10.0.0.2:8080           ← auto-suggest from server IP
   [Health Check]   /health  |  interval: 30s
4. Optional:
   ☑ Auto SSL
   ☐ Basic Auth
   ☐ Redirect www
5. Preview YAML appears in bottom panel — real-time update
6. Click "Add Domain & Deploy" → loading → success/error
7. Domain live in < 30 seconds ✅
```

### Flow: Add Domain (Local Server — Single Server A)

```
1. Click "+ Add Domain"
2. Fill form:
   [Domain]         internal-tools.edsuwarna.id
   [Service Name]   internal-tools
   [Target Server]  [peladen-central (local) ▼]    ← is_gateway = true
3. Because server == gateway, form automatically switches to LOCAL mode:
   [Container Port] 8080
4. Optional same as remote
5. Preview YAML — port: 8080 (not URL)
6. Click "Add Domain & Deploy"
7. Traefik resolves via Docker network ✅
```

### Flow: Server Down Detection

```
1. Traefik health check interval 30s to http://10.0.0.2:8080/health
2. 3 consecutive failures → Traefik mark service unhealthy
3. Anjungan reads from Traefik API:
   - Domain status: 🔴 Down
   - Server status: 🔴 Offline (if all domains on that server are down)
4. Dashboard show: Server B (🔴) — "All services unreachable"
```

---

## 8. Implementation Roadmap

### 🟢 Phase 6.1 — Foundation (Server Registry + Domain CRUD)

**Goal:** Able to register servers + add domains from UI

| Order | Feature | Effort | Dependencies |
|-------|---------|--------|-------------|
| 1 | `cluster_servers` table + migration | 0.5 day | — |
| 2 | Server CRUD backend + frontend | 1 day | #1 |
| 3 | `domains` table + migration | 0.5 day | #2 |
| 4 | Domain CRUD backend | 1 day | #3 |
| 5 | Domain form UI (local + remote conditional) | 1.5 days | #4 |
| 6 | Domain list + status badges UI | 0.5 day | #5 |
| **Total** | | **5 days** | |

### 🟡 Phase 6.2 — Traefik Integration

**Goal:** Config actually affects Traefik

| Order | Feature | Effort | Dependencies |
|-------|---------|--------|-------------|
| 7 | TraefikConfigGenerator (YAML engine) | 1.5 days | #4 |
| 8 | File write + atomic backup + rollback | 0.5 day | #7 |
| 9 | Config preview (real-time YAML) | 0.5 day | #7 |
| 10 | Apply/Regenerate endpoint | 0.5 day | #8 |
| 11 | SSL cert tracking (expiry col + UI badge) | 0.5 day | #7 |
| **Total** | | **3.5 days** | |

### 🔵 Phase 6.3 — Health & Monitoring

**Goal:** Know which server is having issues

| Order | Feature | Effort | Dependencies |
|-------|---------|--------|-------------|
| 12 | Traefik API proxy (health check status) | 1 day | #7 |
| 13 | Health badge auto-refresh on domain list | 0.5 day | #12 |
| 14 | SSL expiry monitoring + notif | 0.5 day | #11 |
| 15 | Server heartbeat + uptime tracking | 0.5 day | #2 |
| **Total** | | **2.5 days** | |

### ⚪ Phase 6.4 — Enhancement

**Goal:** Production-ready hardening

| Order | Feature | Effort |
|-------|---------|--------|
| 16 | Basic auth integration (htpasswd generator) | 0.5 day |
| 17 | WireGuard tunnel status (read-only) | 1 day |
| 18 | Audit log for domain operations | 0.5 day |
| 19 | Export/import domain config | 0.5 day |
| **Total** | | **2.5 days** |

---

## 9. Glossary

| Term | Definition |
|------|------------|
| **Gateway Server** | Server that has public IP + Traefik — the only entry point from the internet |
| **Remote Server** | Server without public IP — can only be accessed via internal network |
| **Traefik File Provider** | Dynamic config method — Traefik reads YAML file and auto-reloads without restart |
| **File Provider** | Traefik feature: read routing config from `.yml` file — replaces labels-based config |
| **Health Check** | Periodic probe to HTTP endpoint — Traefik marks service unhealthy if N consecutive failures |
| **Internal Network** | Private network between servers — usually 10.0.0.0/24 or 192.168.x.x |
| **ACME** | Automatic Certificate Management Environment — protocol for auto-renewing SSL via Let's Encrypt |
| **WireGuard** | Lightweight VPN tunnel — encrypted link between servers, faster than OpenVPN |

## 10. References

- [PRD.md](./PRD.md) — Main Anjungan PRD
- [PRD-repositories-deployments.md](./PRD-repositories-deployments.md) — Repos & deployments PRD
- [DECISIONS.md](../docs/DECISIONS.md) — Architectural decisions
- `sketches/domain-management/` — UI mockup files (see `../sketches/domain-management/`)
