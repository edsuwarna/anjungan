# Anjungan — PRD: Traefik / Reverse Proxy Dashboard

> **Version:** 1.0
> **Status:** 🟡 Proposed — Phase 1
> **Author:** Endang Suwarna
> **Last Updated:** June 11, 2026

---

## 1. Executive Summary

### Problem Statement

Anjungan manages **Traefik via Dokploy** on peladen-central, but visibility into the reverse proxy layer is nearly zero:

- **No centralized routing view** — seeing which domain routes to which container requires SSH-ing into the server and reading Traefik file config or Docker labels
- **TLS certificate status is scattered** — cert expiry, issuer, and SAN info is only visible by opening individual browser tabs and checking the padlock icon
- **Middleware chains are invisible** — rate limits, authentication, redirects, and header manipulation rules are configured in YAML but never surfaced in any UI
- **No service health from load balancer** — Traefik's built-in health checker works, but its status (servers up/down) is hidden unless you curl the Traefik API directly

### What This Solves

| Problem | Solution |
|---------|----------|
| Domain→container routing hidden in YAML | Read-only router list from Traefik API + config parsing |
| TLS cert info per domain invisible | acme.json parser → cert card with expiry, issuer, SANs |
| Middleware chain impossible to audit | Visual middleware chain per router with type + config |
| No visibility into LB health status | Traefik health check status per service |

### Target Audience

- **Endang** (platform engineer) — need one dashboard to see all routing, certs, and middleware at a glance
- **DevOps** — audit middleware configuration, verify TLS coverage, debug routing issues

### Goals

| Goal | Metric |
|------|--------|
| View all Traefik routers (domain→service) | All routers listed in UI |
| See TLS cert status per domain | issuer, expiry, SAN visible per cert |
| Visualize middleware chains | Each route's middleware chain shown with type + config |
| Service health from Traefik | Load balancer server status (up/down) per service |

### Non-Goals

- ❌ **Not a replacement for Traefik dashboard** — Anjungan reads from Traefik API/config, doesn't replace the native Traefik dashboard
- ❌ **Not a configurator** — no CRUD for routers, services, or middlewares. Read-only view only
- ❌ **Not for managing Traefik** — no restart, reload, or Traefik config file editing
- ❌ **Not a DNS manager** — domain records remain in Cloudflare

---

## 2. Product Overview

### This Feature in the Context of Anjungan

```
Anjungan Dashboard
│
├── Traefik Dashboard Menu (/traefik)
│   ├── Routers Tab → searchable table with rule, service, TLS, middleware
│   ├── TLS Certs Tab → card grid with expiry, issuer, SANs
│   ├── Middleware Tab → accordion grouped by type with config detail
│   └── Services Tab → service list with load balancer health
│
├── Traefik Connection (Backend)
│   ├── Traefik API (http://traefik:8080/api)   ← primary
│   └── Config parser via SSH (acme.json)        ← fallback / TLS
│
└── Cache Layer (60s TTL)
    ├── Router list cache
    ├── Certificate cache
    └── Middleware cache
```

### Flow: Data Pipeline

```
Traefik API                              Anjungan                              User
┌──────────────────┐                  ┌────────────────────┐             ┌────────────┐
│ GET /api/http/routers  │                 │ 1. Fetch routers      │             │ Routers tab│
│ GET /api/http/services │ ─── HTTP ───▶ │ 2. Parse + transform  │ ──────────▶ │            │
│ GET /api/http/middlewares│ (60s cached) │ 3. Cache 60s          │    UI       │ Certs tab │
└──────────────────┘                  └────────────────────┘             │ Middleware │
                                                                             └────────────┘
SSH to peladen-central
┌──────────────────────────┐
│ /etc/traefik/acme.json   │ ─── SSH ───▶  TLS Parser
│   → Let's Encrypt certs  │                → expiry, issuer, SANs
└──────────────────────────┘
```

### Key Architecture Decisions

1. **Traefik API as primary source** — Dokploy runs Traefik with API enabled at `http://localhost:8080/api`. Anjungan reads from this API directly via HTTP from the same host.
2. **acme.json via SSH for TLS** — Traefik stores Let's Encrypt certificates in `/etc/traefik/acme.json`. This file is encrypted with Traefik's key and needs specific parsing. Anjungan uses existing server SSH credentials to read and parse it.
3. **Read-only by design** — this dashboard is strictly observational. No write operations to Traefik config. If Endang needs to change routing, that happens through Domains feature or directly on the server.
4. **60s cache** — Traefik config is relatively stable. Changes only happen when a new service is deployed or domain is added. No need for real-time.

---

## 3. Feature Specifications

> **Priority Key:** P0 = Must have (blocker), P1 = Should have (important), P2 = Nice to have

### F1 — Router / Service List (P0)

A searchable, sortable table listing all Traefik HTTP routers with their associated service targets.

| | |
|---|---|
| **Backend** | `GET /api/v1/traefik/routers` — fetch from Traefik API `/api/http/routers`. Parse and return: name, rule (Host:domain.com), entrypoints (web/websecure), service name, TLS status (enabled, cert resolver, cert details), middleware names (string[]), provider (Docker/file), priority, status. `GET /api/v1/traefik/routers/{id}` — single router detail with full middleware chain. `GET /api/v1/traefik/services` — list all services: name, type (loadBalancer, etc.), servers (target URLs), load balancer status, health check config. |
| **Frontend** | Tab "Routers" — table columns: Priority, Rule (Host), Service, TLS (🟢/🔴 badge), EntryPoints, Middlewares (tag pills). Sortable by priority, rule, service. Search bar filters by domain, service name. Row click → slide-out detail panel: full router config, TLS detail, middleware chain with ordering, service targets. Service tab (P1): list all services with health status per server. |
| **UX** | TLS badge: 🟢 TLS enabled, 🔴 no TLS. Click service name → expand service detail with server URLs and health status. Middleware pills with badge count (e.g., "3 middlewares"). Empty state when no routers found. |

**Traefik API response (reference):**
```json
// GET /api/http/routers — Traefik native
[
  {
    "name": "my-router-https@file",
    "provider": "file",
    "rule": "Host(`app1.edsuwarna.id`)",
    "priority": 0,
    "entryPoints": ["websecure"],
    "middlewares": ["rate-limit@file", "auth@file"],
    "service": "app1-svc@file",
    "tls": {
      "certResolver": "letsencrypt",
      "domains": { "main": "app1.edsuwarna.id" }
    },
    "status": "enabled",
    "using": [{"name": "my-router-https@file", "type": "router"}]
  }
]
```

---

### F2 — TLS Certificate Status (P0)

Parse Traefik's acme.json to show all managed Let's Encrypt certificates — issuer, expiry, SANs, days remaining.

| | |
|---|---|
| **Backend** | `GET /api/v1/traefik/certificates` — SSH into peladen-central, read `/etc/traefik/acme.json`. Parse Let's Encrypt certificate storage format (v2/v3). Extract per-domain: domain (CN), issuer (Let's Encrypt authority), expiry date (notAfter), subject, SANs (Subject Alternative Names), certificate fingerprint (SHA-256). Compute `days_remaining` from now to expiry. Cache result 60s. **Error handling**: if SSH fails or acme.json unreadable, return empty array with warning status. Integration point with SSL Monitoring feature (PRD-ssl-monitoring.md) — cross-reference monitored domains. |
| **Frontend** | Tab "TLS Certs" — card grid layout. Each card: domain name (large, bold), issuer (e.g., "R3 · Let's Encrypt"), expiry date, days remaining countdown with color coding, SAN tags (small pill badges). Link/button → associated SSL Monitor record if exists. **KPIs**: total certs, expiring within 30 days, expired. |
| **UX** | Color coding for days remaining: 🟢 **>30 days** (green), 🟡 **7–30 days** (yellow), 🔴 **<7 days** (red). Card click → expand cert detail: full SAN list, fingerprint, certificate chain info. If a cert is monitored in SSL Monitoring feature, show 🔗 link icon to the monitor record. |

---

### F3 — Middleware Overview (P1)

List all middlewares defined in Traefik, grouped by type, with config summary and per-router association.

| | |
|---|---|
| **Backend** | `GET /api/v1/traefik/middlewares` — fetch from Traefik API `/api/http/middlewares`. Parse and return per middleware: name, provider (Docker/file), type (rateLimit, basicAuth, redirectRegex, headers, ipWhiteList, etc.), config summary (e.g., rate limit: 100 req/s, auth: basic realm, redirect: https://). Also compute which routers use each middleware (reverse-lookup from router list). |
| **Frontend** | Tab "Middlewares" — accordion/collapsible sections grouped by type (Rate Limit, Auth, Redirect, Headers, IP Whitelist, etc.). Each middleware card: name, type badge, config summary (key-value pairs), "Used by N routers" with router name links. Expand → full config detail, full router list, YAML/JSON config preview. |
| **UX** | Type badges with icons: ⏱ rate-limit, 🔐 basicAuth, 🔀 redirect, 📋 headers, 🚦 ipWhiteList. Click router link → navigate to Routers tab with that router highlighted. Empty state if no middlewares configured. |

---

### F4 — Service Health from Traefik (P1)

Show Traefik's load balancer health check status per service — which backend servers (containers) are up/down.

| | |
|---|---|
| **Backend** | `GET /api/v1/traefik/services` — fetch from Traefik API `/api/http/services`. Parse per service: name, provider, type (loadBalancer), servers (URLs), health status (serverStatus map: URL → status string), health check config (path, interval, timeout). Compute aggregate health per service: allUp, allDown, degraded (mixed). |
| **Frontend** | Services sub-tab under Routers or standalone card in Routers detail panel. Service list table: name, type, server count, health aggregate badge (🟢 all up, 🟡 degraded, 🔴 down, ⚪ unknown). Click → expand server detail: each server URL with health status dot. Health check config shown if configured. |
| **UX** | Auto-refresh behavior tied to main 60s cache. Health status dots: 🟢 UP, 🔴 DOWN, ⚪ unknown/not checked. If health check not configured, show "—" for health with tooltip "No health check configured". |

---

## 4. API Design

### New Endpoints

```go
// === Traefik Routers ===
GET    /api/v1/traefik/routers                    // List all routers (?search=&sort=&order=)
GET    /api/v1/traefik/routers/{id}               // Router detail (URL-encoded name, e.g. my-router%40file)

// === Traefik Services ===
GET    /api/v1/traefik/services                   // List all services (?search=)
GET    /api/v1/traefik/services/{id}              // Service detail with server health

// === Traefik TLS Certificates ===
GET    /api/v1/traefik/certificates               // TLS cert status from acme.json
GET    /api/v1/traefik/certificates/{domain}      // Single cert detail

// === Traefik Middlewares ===
GET    /api/v1/traefik/middlewares                // List all middlewares (?type=)
GET    /api/v1/traefik/middlewares/{id}           // Middleware detail

// === Traefik Connection ===
GET    /api/v1/traefik/status                     // Traefik connection status (API reachable? acme.json readable?)
POST   /api/v1/traefik/refresh                    // Force invalidate cache and re-fetch
```

### Response Format

```json
// GET /api/v1/traefik/routers
{
  "success": true,
  "data": [
    {
      "id": "my-router-https@file",
      "name": "my-router-https",
      "provider": "file",
      "rule": "Host(`app1.edsuwarna.id`)",
      "priority": 0,
      "entrypoints": ["websecure"],
      "service": "app1-svc",
      "service_id": "app1-svc@file",
      "tls": {
        "enabled": true,
        "cert_resolver": "letsencrypt",
        "cert_domain": "app1.edsuwarna.id",
        "cert_expires_at": "2026-09-20T00:00:00Z"
      },
      "middlewares": [
        {"name": "rate-limit", "provider": "file", "type": "rateLimit"},
        {"name": "auth", "provider": "file", "type": "basicAuth"}
      ],
      "status": "enabled"
    }
  ],
  "meta": {
    "source": "traefik_api",
    "cached_at": "2026-06-11T10:00:00Z",
    "expires_at": "2026-06-11T10:01:00Z"
  }
}

// GET /api/v1/traefik/certificates
{
  "success": true,
  "data": [
    {
      "domain": "app1.edsuwarna.id",
      "issuer_cn": "R3",
      "issuer_org": "Let's Encrypt",
      "not_before": "2026-06-20T00:00:00Z",
      "not_after": "2026-09-20T00:00:00Z",
      "days_remaining": 101,
      "fingerprint_sha256": "AB:CD:EF:...",
      "sans": ["app1.edsuwarna.id", "www.app1.edsuwarna.id"],
      "key_type": "RSA",
      "key_size": 2048
    }
  ],
  "meta": {
    "source": "acme_json",
    "cached_at": "2026-06-11T10:00:00Z",
    "expires_at": "2026-06-11T10:01:00Z"
  }
}

// GET /api/v1/traefik/middlewares
{
  "success": true,
  "data": [
    {
      "id": "rate-limit@file",
      "name": "rate-limit",
      "provider": "file",
      "type": "rateLimit",
      "config": {
        "average": 100,
        "burst": 200,
        "period": "1s"
      },
      "used_by_routers": ["my-router-https@file", "api-router@file"]
    }
  ]
}

// GET /api/v1/traefik/status
{
  "success": true,
  "data": {
    "api_reachable": true,
    "acme_json_readable": true,
    "last_api_fetch": "2026-06-11T10:00:00Z",
    "last_acme_parse": "2026-06-11T10:00:00Z",
    "errors": []
  }
}
```

---

## 5. Database Schema

### New Tables

No new tables required — this feature is **read-only** from Traefik API + acme.json parsing. Data is cached in-memory, not persisted in DB.

### Cache Layer

| Aspect | Detail |
|--------|--------|
| **Storage** | In-memory Go struct (sync.RWMutex-protected map) |
| **TTL** | 60 seconds |
| **Invalidation** | `POST /api/v1/traefik/refresh` — force clear cache |
| **Fallback** | If fetch fails, serve stale cache + `cached_at` timestamp in response |

---

## 6. UX Flow

### Flow: Routers Tab

```
┌─────────────────────────────────────────────────────────────┐
│  🔁 Traefik Dashboard                                       │
│                                                             │
│  [🔃 Routers] [🔒 TLS Certs] [⚙ Middlewares] [📊 Services] │
│  ─────────────────────────────────────────────────────────── │
│                                                             │
│  🔍 [ Search by domain or service name...       ]           │
│                                                             │
│  ┌──────┬──────────────────────────┬─────────┬─────┬───────┐│
│  │ Pri  │ Rule                     │ Service │ TLS │ Mdlwr ││
│  ├──────┼──────────────────────────┼─────────┼─────┼───────┤│
│  │ 0    │ Host(`app1.edsuwarna.id`)│ app1-svc│ 🟢  │ ⏱🔐   ││
│  │ 0    │ Host(`notes.edsuwarna.id`)│ notes-svc│ 🟢  │ —     ││
│  │ 10   │ Host(`api.edsuwarna.id`) │ api-svc │ 🔴  │ 🔀    ││
│  └──────┴──────────────────────────┴─────────┴─────┴───────┘│
│                                                             │
│  ─── Slide-out Detail Panel (click row) ─────               │
│  ┌─────────────────────────────────────┐                    │
│  │ Router: app1-https@file             │                    │
│  │ Rule: Host(`app1.edsuwarna.id`)     │                    │
│  │ EntryPoints: websecure              │                    │
│  │ Priority: 0                         │                    │
│  │ Status: enabled                     │                    │
│  │                                    │                    │
│  │ 🔒 TLS: certResolver=letsencrypt   │                    │
│  │   → Cert: app1.edsuwarna.id        │                    │
│  │   → Expires: 20 Sep 2026 (101d)    │                    │
│  │                                    │                    │
│  │ ⚙ Middleware Chain:                │                    │
│  │   1. rate-limit (rateLimit)        │                    │
│  │      → 100 req/s, burst 200       │                    │
│  │   2. auth (basicAuth)             │                    │
│  │      → realm: Restricted Area     │                    │
│  │                                    │                    │
│  │ 🎯 Service: app1-svc               │                    │
│  │   → http://10.0.0.2:8080 🟢 UP     │                    │
│  │   → Health: /health every 30s     │                    │
│  └─────────────────────────────────────┘                    │
└─────────────────────────────────────────────────────────────┘
```

### Flow: TLS Certs Tab

```
┌─────────────────────────────────────────────────────────────┐
│  🔒 TLS Certificates                                       │
│                                                             │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐                    │
│  │ 🟢 5 Valid│ │ 🟡 2 Exp. │ │ 🔴 0 Exp.│                    │
│  │          │ │  Soon    │ │          │                    │
│  └──────────┘ └──────────┘ └──────────┘                    │
│                                                             │
│  ┌──────────────────────┐  ┌──────────────────────┐        │
│  │ 🟢 app1.edsuwarna.id │  │ 🟢 notes.edsuwarna.id│        │
│  │    Issuer: R3        │  │    Issuer: R3        │        │
│  │    Expires: 20 Sep   │  │    Expires: 15 Nov   │        │
│  │    ⏳ 101 days left  │  │    ⏳ 157 days left  │        │
│  │    [app1, www.app1]  │  │    [notes]           │        │
│  │    🔗 SSL Monitor    │  │                      │        │
│  ├──────────────────────┤  ├──────────────────────┤        │
│  │ 🟡 staging.app1.id   │  │ 🔴 old-app1.id      │        │
│  │    Issuer: R3        │  │    Issuer: ZeroSSL   │        │
│  │    Expires: 28 Jun   │  │    Expires: 05 Jun   │        │
│  │    ⏳ 17 days left   │  │    ⏳ EXPIRED        │        │
│  │    [staging.app1.id] │  │    [old-app1.id]    │        │
│  └──────────────────────┘  └──────────────────────┘        │
└─────────────────────────────────────────────────────────────┘
```

### Flow: Middleware Tab

```
┌─────────────────────────────────────────────────────────────┐
│  ⚙ Middlewares                                              │
│                                                             │
│  ▶ ⏱ Rate Limit (2)                                        │
│    ┌────────────────────────────────────┐                   │
│    │ rate-limit (file)                  │                   │
│    │   Average: 100 req/s               │                   │
│    │   Burst: 200                       │                   │
│    │   Period: 1s                       │                   │
│    │   Used by: app1-router, api-router │                   │
│    └────────────────────────────────────┘                   │
│                                                             │
│  ▶ 🔐 Auth (1)                                              │
│    ┌────────────────────────────────────┐                   │
│    │ auth (file)                        │                   │
│    │   Type: basicAuth                  │                   │
│    │   Realm: Restricted Area           │                   │
│    │   Users: 3 configured              │                   │
│    │   Used by: app1-router             │                   │
│    └────────────────────────────────────┘                   │
│                                                             │
│  ▶ 🔀 Redirect (1)                                          │
│    ┌────────────────────────────────────┐                   │
│    │ www-redirect (file)                │                   │
│    │   Type: redirectRegex              │                   │
│    │   Regex: ^(.*)://www.(.*)$        │                   │
│    │   Replacement: ${1}://${2}        │                   │
│    │   Used by: www-router             │                   │
│    └────────────────────────────────────┘                   │
└─────────────────────────────────────────────────────────────┘
```

---

## 7. Non-Functional Requirements

| Aspect | Target | Notes |
|--------|--------|-------|
| **Cache TTL** | 60s | Traefik config is relatively static after deployment |
| **Refresh** | Manual button + auto every 60s | User can force-refresh via button |
| **Traefik API timeout** | 5s | If Traefik API is unreachable, fall back to cached data |
| **SSH timeout** | 10s | For acme.json parsing |
| **Concurrent access** | Safe | Cache protected with RWMutex |
| **Error resilience** | Serve stale cache | Never error-out if cache exists, show `cached_at` timestamp |
| **Memory** | < 50 MB | Config data is small (hundreds of KB at most) |
| **Connection status** | Visible | Status indicator showing if Traefik API is reachable |

---

## 8. Implementation Roadmap

### 🟡 Phase 1 — Core (Sprint 1)

**Goal:** Routers list + TLS certs visible in UI

| Order | Feature | Effort | Dependencies |
|-------|---------|--------|-------------|
| 1 | Traefik API client (Go HTTP client to Traefik API) | 1 day | — |
| 2 | Router list endpoint + cache layer | 1 day | #1 |
| 3 | Frontend: Routers tab with searchable table | 1.5 days | #2 |
| 4 | Frontend: Router detail slide-out panel | 1 day | #3 |
| 5 | acme.json parser via SSH (Go SSH client + JSON decoding) | 1.5 days | — |
| 6 | TLS certificates endpoint | 0.5 day | #5 |
| 7 | Frontend: TLS Certs tab with card grid | 1 day | #6 |
| 8 | Connection status + cache invalidation endpoint | 0.5 day | #1, #5 |
| | **Total** | **8 days** | |

### 🔵 Phase 2 — Middleware & Health (Sprint 2)

**Goal:** Middleware visualization + service health

| Order | Feature | Effort | Dependencies |
|-------|---------|--------|-------------|
| 9 | Middlewares endpoint (from Traefik API) | 0.5 day | #1 |
| 10 | Frontend: Middlewares tab with accordion + type grouping | 1 day | #9 |
| 11 | Services endpoint with health status | 1 day | #1 |
| 12 | Frontend: Services sub-tab with health badges | 1 day | #11 |
| 13 | Auto-refresh every 60s on all tabs | 0.5 day | #3, #7, #10, #12 |
| 14 | Integrate with SSL Monitoring (cross-link certs → monitors) | 0.5 day | #6, SSL PRD |
| | **Total** | **4.5 days** | |

### ⚪ Phase 3 — Polish (Sprint 3)

**Goal:** Production-ready hardening

| Order | Feature | Effort |
|-------|---------|--------|
| 15 | Auto-detect Traefik on servers (discover which servers run Traefik) | 1 day |
| 16 | Support multiple Traefik instances (multi-server) | 1 day |
| 17 | Export router list as CSV | 0.5 day |
| 18 | Audit log for manual refresh actions | 0.5 day |
| 19 | Traefik version display in dashboard header | 0.5 day |
| | **Total** | **3.5 days** |

### Total Estimated Effort: ~16 days (All 3 phases)

---

## 9. Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Data source | Traefik API (primary) + acme.json via SSH (TLS) | API is structured and reliable; acme.json needed because TLS cert info is not fully exposed via Traefik API |
| Cache storage | In-memory (Go map) | No persistence needed, small data size, avoids DB dependency |
| Cache TTL | 60s | Balances freshness vs. load on Traefik API. Config doesn't change frequently |
| Stale serving | Yes | If API is down, serve cached data with stale indicator |
| acme.json parsing | Via SSH (existing server creds) | Traefik's acme.json is encrypted at rest with Traefik's key. Need to read the file after Traefik has decrypted it in memory. Leverages existing `cluster_servers` SSH access. |
| Frontend routing | Tab-based single page | Routers, Certs, Middlewares, Services are logically related — tabs keep them connected |
| Detail panel | Slide-out drawer | Avoids page navigation, keeps context visible while exploring details |
| Read-only | Strictly enforced | This is a dashboard, not a management tool. No POST/PUT/DELETE to Traefik config |

---

## 10. Glossary

| Term | Definition |
|------|------------|
| **Router** | Traefik routing rule — matches incoming request (by Host, Path, etc.) and forwards to a service |
| **Service** | Load balancer configuration — defines how to reach backend containers (URL, port, health check) |
| **Middleware** | Request modifier — applied between router and service (rate limiting, auth, redirects, headers) |
| **EntryPoint** | Network listener — web (HTTP :80) or websecure (HTTPS :443) |
| **Provider** | Configuration source — Docker (labels) or File (YAML) |
| **acme.json** | Traefik's Let's Encrypt certificate storage — JSON file with encrypted cert bundles |
| **File Provider** | Traefik dynamic config via YAML files in `/etc/traefik/dynamic/` |
| **Load Balancer** | Traefik's built-in reverse proxy — distributes requests to backend servers |
| **Health Check** | Periodic probe to backend server endpoint — marks server UP/DOWN based on response |

---

## 11. References

- [PRD.md](./PRD.md) — Main Anjungan PRD
- [PRD-domain-management.md](./PRD-domain-management.md) — Domain Management & Multi-Server Routing (Traefik config generator context)
- [PRD-ssl-monitoring.md](./PRD-ssl-monitoring.md) — SSL Certificate Monitoring (integration target for TLS certs)
- [Traefik API Documentation](https://doc.traefik.io/traefik/operations/api/) — Official Traefik API reference
- [Traefik acme.json Format](https://doc.traefik.io/traefik/https/acme/) — Let's Encrypt certificate storage
