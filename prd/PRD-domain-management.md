# Anjungan — PRD: Domain Management & Multi-Server Routing

> **Version:** 1.0
> **Status:** Draft
> **Author:** Endang Suwarna
> **Last Updated:** June 5, 2026

---

## 1. Executive Summary

### Problem Statement

Anjungan jalan di **peladen-central** — satu server yang punya public IP (203.0.113.1). Tapi Endang punya **4-5 server** lain (peladen-ml, peladen-cache, peladen-backup) yang **ga punya public IP**. Mereka cuma bisa diakses lewat internal network (10.0.0.0/24).

Sekarang, aplikasi yang pengen dibuka dari internet harus:
1. Di-forward manual lewat internal IP (`10.0.0.2:8080`)
2. SSL cert diurus manual
3. Ga ada single pane of glass buat liat *siapa punya domain apa, di server mana, status sehat apa enggak*
4. Setiap nambah domain harus edit compose manual + nambah Traefik labels

**Domain Management solving this:**
- Anjungan jadi **central ingress manager** — semua domain diatur dari satu dashboard
- Traefik di peladen-central jadi gateway → forward ke server internal mana pun
- SSL auto via Let's Encrypt
- Health check monitoring dari Anjungan
- Ga perlu SSH-SSH lagi buat urusan domain

### Target Audience

- **Endang** (platform engineer) — stop manual Traefik config
- **Developer** — self-service nambah domain buat service mereka
- **Future infra team** — audit trail siapa nambah domain apa

### Goals

| Goal | Metric |
|------|--------|
| Tambah domain dari UI → apply ke Traefik | < 30 detik dari klik → live |
| Daftarin semua server dalam satu tempat | 5 server registered |
| Lihat status semua domain di semua server | Health green/red per domain |
| SSL auto-renew tanpa intervensi manual | 100% cert auto-renewed |
| Health check remote server dari Traefik | Detection < 30s |

### Non-Goals

- ❌ Bukan DNS registrar — gantiin Cloudflare DNS. Domain tetap di-manage di Cloudflare.
- ❌ Bukan load balancer replacement — pake Traefik yang udah jalan.
- ❌ Bukan WireGuard manager built-in — setup tunnel tetep manual.
- ❌ Bukan cert manager alternatif — pake Traefik + Let's Encrypt yang udah ada.

---

## 2. Product Overview

### Fitur Ini Dalam Konteks Anjungan

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

Ini bukan fitur yang berdiri sendiri — dia **nempel di Traefik** yang udah jalan di peladen-central. Anjungan ga perlu install Traefik baru. Cuma perlu **Frontend: UI** + **Backend: File Provider generator** + **DB schema baru**.

---

## 3. Feature Specifications

> **Priority Key:** P0 = Must have (blocker), P1 = Should have (important), P2 = Nice to have

### Phase 6 — Networking & Ingress

#### F6.1 — Server Registry

| | |
|---|---|
| **Priority** | P0 |
| **Backend** | `servers_cluster` table baru (bukan `servers` yang existing — ini buat cluster nodes, bukan target SSH). Kolom: id, name, public_ip, internal_ip, status (online/maintenance/offline), labels (text[]), specs (jsonb: cpu, ram, disk), uptime_seconds, created_at, updated_at. CRUD: `GET/POST/PUT/DELETE /api/v1/cluster/servers`. Health check endpoint: `POST /api/v1/cluster/servers/{id}/health` (ping via internal IP) |
| **Frontend** | Route `/infra/servers`. Grid card layout — tiap server: status dot 🟢/🟡/🔴, name, public/internal IP, spec, uptime, label badges. "Register New Server" card (dashed border, plus icon). Edit/Delete modal. |
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
| **Backend** | `domains` table baru. Kolom: id, domain (unique), cluster_server_id (FK), target_url (VARCHAR), target_port (INT), service_name (VARCHAR — optional display), ssl_enabled, cert_expires_at, basic_auth_enabled, basic_auth_hash, health_check_path, health_check_interval, redirect_www, labels_override (JSONB — extra Traefik labels), status (active/pending_ssl/error/expired), created_at, updated_at. CRUD: `GET/POST/PUT/DELETE /api/v1/domains`. Filter: `?server_id=`, `?status=active`, `?search=domain`. |
| **Frontend** | Route `/infra/domains`. List semua domain: domain name, server badge (local/remote → internal IP), SSL status, health status 🟢/🟡/🔴, cert expiry, Edit/Delete. **Gateway diagram card** di atas — flow visual Internet → Traefik → server. **Add Domain** expandable form: domain, service name, server dropdown (dari cluster_servers), conditional — kalo local: port doang; kalo remote: URL + health check. Options: auto SSL, basic auth, www redirect, health check path+interval. Traefik config preview panel (live YAML preview). |
| **UX** | Form validation inline. Domain format check. Server dropdown pake status indicator. Basic auth password hash auto-generated pake htpasswd. Preview YAML update live pas ganti field. Deployment confirmation — "Config akan ditulis ke /etc/traefik/dynamic/domains.yml dan Traefik akan auto-reload." |

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
| **Backend** | Service core: `TraefikConfigGenerator` — baca semua domain dari DB → generate YAML sesuai format Traefik File Provider. Logic: kalo `cluster_server.is_gateway = true` → `loadBalancer.servers[0].port = target_port`, kalo `is_gateway = false` → `loadBalancer.servers[0].url = target_url`. SSL: auto set `tls.certResolver: letsencrypt`. Basic auth: generate middleware dengan htpasswd format ($$ double dollar!). Health check buat remote server: `loadBalancer.healthCheck.path + interval`. Write ke `/etc/traefik/dynamic/domains.yml`. Auto-reload Traefik config. |
| **Frontend** | No dedicated page — backend process. Tapi preview ditampilkan di form Add/Edit Domain (read-only YAML). Validation errors ditampilkan inline. |
| **UX** | Kalo YAML malformed, backend return error sebelum nulis ke disk. Rollback ke config sebelumnya. |

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
| **Backend** | Scan tiap domain → cek `cert_expires_at`. Kalo < 30 hari: update status jadi `expiring_soon`. Kalo expired: status `expired`. Cron job: daily check SSL expiry. Notification trigger kalo < 14 hari. Endpoint: `GET /api/v1/domains/ssl-summary` — count active, expiring_soon, expired. |
| **Frontend** | Badge di domain list: 🟢 Active (68d), 🟡 Expiring (10d), 🔴 Expired. SSL summary card di dashboard: "2 certs expiring within 30 days". Click → filter domain list. |
| **UX** | Warna badge sesuai urgency. Kalo < 30 hari munculin warning icon. Kalo expired — merah terang + tooltip "Traefik cert expired — otomatis renew via ACME" atau "Custom cert — renew manual". |

---

#### F6.5 — Health Check Dashboard

| | |
|---|---|
| **Priority** | P1 |
| **Backend** | Traefik health check status udah built-in di Traefik API. Anjungan baca dari Traefik API (`/api/http/services`) untuk parse health per service. Endpoint: `GET /api/v1/domains/{id}/health` — proxy call ke Traefik API atau parse dari file. Kalo Traefik API ga accessible (Dokploy ga expose), fallback: cron `docker exec traefik traefik healthcheck` parsing. |
| **Frontend** | Tiap domain di list punya health badge: 🟢 OK, 🟡 Degraded, 🔴 Down, ⚪ Unknown. Health detail card (optional): last check, response time, status code. Filter domain list by health status. |
| **UX** | Auto-refresh tiap 30s. Kalo remote server down → badge merah + "Server unreachable" tooltip. Kalo health check path not configured → badge abu "—". |

---

#### F6.6 — WireGuard Integration (Optional)

| | |
|---|---|
| **Priority** | P2 |
| **Backend** | WireGuard status checker: `wg show` parsing. Cek interface status, peer handshake, transfer. Endpoint: `GET /api/v1/cluster/servers/{id}/tunnel`. Endpoint: `POST /api/v1/cluster/servers/{id}/tunnel/test` — ping via WireGuard IP. |
| **Frontend** | Tunnel indicator di server card: "WG 🟢" atau "WG 🔴". Tunnel detail modal: interface, peer public key, handshake age, transfer in/out, endpoint. |
| **UX** | Kalo server pake WireGuard, tampilkan tunnel status di server card. Kalo ga pake, hidden. Test tunnel button — ping + latency. |

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
| `users` | Audit log — siapa nambah/ubah domain |
| `audit_logs` | Catat tiap domain action (create, update, delete, apply) |

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
| **SSL cert monitoring** | Daily check, notif kalo < 14 hari |
| **Health check polling** | 30s interval (by Traefik) |
| **File write** | Atomic write — kalo crash, config lama aman |
| **Rollback** | Backup config sebelum overwrite — restore kalo error |
| **Concurrent domain operations** | Lock per-write — ga ada race condition |

---

## 7. UX Flow Detail

### Flow: Add Domain (Remote Server)

```
1. Klik "+ Add Domain"
2. Isi form:
   [Domain]         app1.edsuwarna.id
   [Service Name]   app-1-api
   [Target Server]  [peladen-ml (10.0.0.2) ▼]     ← dari cluster_servers
3. Karena server != gateway, form otomatis switch ke mode REMOTE:
   [Target URL]     http://10.0.0.2:8080           ← auto-suggest dari server IP
   [Health Check]   /health  |  interval: 30s
4. Opsional:
   ☑ Auto SSL
   ☐ Basic Auth
   ☐ Redirect www
5. Preview YAML muncul di panel bawah — real-time update
6. Klik "Add Domain & Deploy" → loading → success/error
7. Domain live di < 30 detik ✅
```

### Flow: Add Domain (Local Server — Satu Server A)

```
1. Klik "+ Add Domain"
2. Isi form:
   [Domain]         internal-tools.edsuwarna.id
   [Service Name]   internal-tools
   [Target Server]  [peladen-central (local) ▼]    ← is_gateway = true
3. Karena server == gateway, form otomatis switch ke mode LOCAL:
   [Container Port] 8080
4. Opsional sama seperti remote
5. Preview YAML — port: 8080 (bukan URL)
6. Klik "Add Domain & Deploy"
7. Traefik resolve via Docker network ✅
```

### Flow: Server Down Detection

```
1. Traefik health check interval 30s ke http://10.0.0.2:8080/health
2. 3 consecutive failures → Traefik mark service unhealthy
3. Anjungan baca dari Traefik API:
   - Domain status: 🔴 Down
   - Server status: 🔴 Offline (kalo semua domain di server itu down)
4. Dashboard show: Server B (🔴) — "All services unreachable"
```

---

## 8. Implementation Roadmap

### 🟢 Phase 6.1 — Foundation (Server Registry + Domain CRUD)

**Goal:** Bisa daftarin server + nambah domain dari UI

| Order | Feature | Effort | Dependencies |
|-------|---------|--------|-------------|
| 1 | `cluster_servers` table + migration | 0.5 hari | — |
| 2 | Server CRUD backend + frontend | 1 hari | #1 |
| 3 | `domains` table + migration | 0.5 hari | #2 |
| 4 | Domain CRUD backend | 1 hari | #3 |
| 5 | Domain form UI (local + remote conditional) | 1.5 hari | #4 |
| 6 | Domain list + status badges UI | 0.5 hari | #5 |
| **Total** | | **5 hari** | |

### 🟡 Phase 6.2 — Traefik Integration

**Goal:** Config beneran ngefek ke Traefik

| Order | Feature | Effort | Dependencies |
|-------|---------|--------|-------------|
| 7 | TraefikConfigGenerator (YAML engine) | 1.5 hari | #4 |
| 8 | File write + atomic backup + rollback | 0.5 hari | #7 |
| 9 | Config preview (real-time YAML) | 0.5 hari | #7 |
| 10 | Apply/Regenerate endpoint | 0.5 hari | #8 |
| 11 | SSL cert tracking (expiry col + UI badge) | 0.5 hari | #7 |
| **Total** | | **3.5 hari** | |

### 🔵 Phase 6.3 — Health & Monitoring

**Goal:** Tau mana server yang bermasalah

| Order | Feature | Effort | Dependencies |
|-------|---------|--------|-------------|
| 12 | Traefik API proxy (health check status) | 1 hari | #7 |
| 13 | Health badge auto-refresh di domain list | 0.5 hari | #12 |
| 14 | SSL expiry monitoring + notif | 0.5 hari | #11 |
| 15 | Server heartbeat + uptime tracking | 0.5 hari | #2 |
| **Total** | | **2.5 hari** | |

### ⚪ Phase 6.4 — Enhancement

**Goal:** Production-ready hardening

| Order | Feature | Effort |
|-------|---------|--------|
| 16 | Basic auth integration (htpasswd generator) | 0.5 hari |
| 17 | WireGuard tunnel status (read-only) | 1 hari |
| 18 | Audit log for domain operations | 0.5 hari |
| 19 | Export/import domain config | 0.5 hari |
| **Total** | | **2.5 hari** |

---

## 9. Glossary

| Term | Definition |
|------|------------|
| **Gateway Server** | Server yang punya public IP + Traefik — satu-satunya pintu masuk dari internet |
| **Remote Server** | Server tanpa public IP — cuma bisa diakses lewat internal network |
| **Traefik File Provider** | Dynamic config method — Traefik baca YAML file dan auto-reload tanpa restart |
| **File Provider** | Traefik feature: baca routing config dari file `.yml` — ganti labels-based config |
| **Health Check** | Periodic probe ke endpoint HTTP — Traefik mark service unhealthy kalo gagal N kali |
| **Internal Network** | Private network antar server — biasanya 10.0.0.0/24 atau 192.168.x.x |
| **ACME** | Automatic Certificate Management Environment — protocol buat auto-renew SSL via Let's Encrypt |
| **WireGuard** | Lightweight VPN tunnel — encrypted link antar server, lebih cepet dari OpenVPN |

## 10. References

- [PRD.md](./PRD.md) — Main Anjungan PRD
- [PRD-repositories-deployments.md](./PRD-repositories-deployments.md) — Repos & deployments PRD
- [DECISIONS.md](../DECISIONS.md) — Architectural decisions
- `sketches/domain-management/` — UI mockup files (see `../sketches/domain-management/`)
