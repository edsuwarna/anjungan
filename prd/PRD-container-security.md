# Anjungan — PRD: Container Runtime Security Posture

> **Version:** 1.0
> **Status:** 🔴 Not Implemented — Proposed Extension to Existing Compliance
> **Author:** Endang Suwarna
> **Last Updated:** June 11, 2026

---

## 1. Executive Summary

### Problem Statement

Anjungan's existing Compliance module performs 10 basic runtime container checks (privileged, root, capabilities), but **container security posture goes far beyond those 10 checks**. Currently:

- Checks are binary pass/fail — no nuance, no scoring weight
- No visibility into **seccomp profiles**, **AppArmor/SELinux**, or **read-only root filesystem**
- No **network security** checks — which containers are exposed to public? What ports?
- No **resource limits** monitoring — containers without memory/CPU limits are DoS risks
- No **image provenance** — which containers run images from untrusted registries?
- Container drift detection — container config changed from deployment spec?
- **No trend** — is security posture improving or degrading over time?

The 10 checks in Compliance are a good start, but production container security requires a deeper, continuously monitored posture.

### What This Solves

| Problem | Solution |
|---------|----------|
| Only 10 basic container checks | **25+ runtime checks** across 6 categories |
| No seccomp/AppArmor visibility | **Linux capabilities profile detection** |
| Network exposure blind spot | **Port binding + host network detection** |
| No resource limit enforcement | **Memory/CPU/PID limit checks** |
| Untrusted image registry | **Registry origin validation** |
| No container drift detection | **Config snapshot vs runtime comparison** |
| No posture trend | **Score history + improvement tracking** |

### Current Status

| Aspect | Status |
|--------|--------|
| 10 basic runtime checks (privileged, root, basic caps) | ✅ Available in Compliance |
| Extended checks (seccomp, AppArmor, RO fs, networks, resources) | ❌ Not implemented — This PRD |
| Container security score | ❌ Not implemented |
| Score trend / history | ❌ Not implemented |
| Container-level vs image-level distinction | ❌ Not implemented |
| Network exposure analysis | ❌ Not implemented |
| Notification on security regression | ❌ Not implemented |

### Target Audience

- **Endang** (platform engineer) — know which containers are security risks
- **DevOps** — enforce baseline container security across all servers
- **Security-conscious teams** — runtime security is a core compliance requirement

### Goals

| Goal | Metric |
|------|--------|
| Expand from 10 to 25+ runtime checks | ✅ 6 categories: Privilege, Capabilities, Filesystem, Network, Resources, Image |
| Per-container security score | ✅ 0-100% weighted score |
| Score history & trend | ✅ Track per-container, per-server, globally |
| Network exposure mapping | ✅ Which containers have host ports, host network, or are publicly accessible |
| Image provenance tracking | ✅ Registry origin trust level |
| Drift detection | ✅ Config change alert between deployments |
| Notification on regression | ✅ Score drops below threshold → alert via notification targets |

### Non-Goals

- ❌ Not a vulnerability scanner — see PRD-container-image-scanning.md (Trivy)
- ❌ Not real-time runtime threat detection (falco, tracee) — that's separate
- ❌ Not a Kubernetes security posture manager (KSPM) — this is Docker Compose
- ❌ Not replacing the existing 10 checks in Compliance — extending them

---

## 2. Product Overview

### Architecture

```
Anjungan Backend
┌──────────────────────────────────────────────────────────────┐
│                                                              │
│  Container Security Scanner                                  │
│  ┌──────────────────────────────────────────────────────┐    │
│  │ Check Suites                                           │    │
│  │                                                       │    │
│  │ Privilege          Capabilities     Filesystem         │    │
│  │ ├─ privileged?     ├─ dangerous?     ├─ read-only fs  │    │
│  │ ├──host_pid?       ├─ CAP_SYS_ADMIN  ├─ tmpfs mounts  │    │
│  │ ├─host_net?        ├─ CAP_NET_RAW    ├─ volume mounts │    │
│  │ └─host_ipc?        ├─ CAP_DAC_OVERRIDE└─ bind mounts  │    │
│  │                    └─ CAP_SETUID                       │    │
│  │                    ┌──────────────────────────────┐    │    │
│  │    Network         │    Resources                 │    │    │
│  │ ├─ host ports      │ ├─ memory limit set?         │    │    │
│  │ ├─ expose all IPs  │ ├─ CPU limit set?            │    │    │
│  │ ├─ published ports │ ├─ PID limit set?            │    │    │
│  │ └─ network mode    │ └─ OOM score adjustment      │    │    │
│  │                    └──────────────────────────────┘    │    │
│  │                    ┌──────────────────────────────┐    │    │
│  │    Image           │    Drift Detection            │    │    │
│  │ ├─ registry trust  │ ├─ config vs runtime diff    │    │    │
│  │ ├─ latest tag?     │ ├─ env var changes?          │    │    │
│  │ ├─ image age       │ ├─ volume changes?           │    │    │
│  │ └─ unpacked size   │ └─ cmd/entrypoint changes    │    │    │
│  │                    └──────────────────────────────┘    │    │
│  └──────────────────────────────────────────────────────┘    │
│                           │                                  │
│                           ▼                                  │
│  ┌──────────────────────────────────────────────────────┐    │
│  │ Scoring Engine                                        │    │
│  │ ├─ Weight per check (critical/high/medium/low)        │    │
│  │ ├─ Per-container score → Per-server score → Global    │    │
│  │ └─ Trend calculation (7d/30d delta)                   │    │
│  └──────────────────────────────────────────────────────┘    │
│                           │                                  │
│                           ▼                                  │
│  ┌──────────────────────────────────────────────────────┐    │
│  │ DB: container_security_scans, container_security_findings │
│  └──────────────────────────────────────────────────────┘    │
│                                                              │
└──────────────────────────┬───────────────────────────────────┘
                           │
                           ▼
┌──────────────────────────────────────────────────────────────┐
│  Frontend (SvelteKit)                                        │
│  ┌───────────────────┐  ┌──────────────┐  ┌────────────┐    │
│  │ Container Security│  │ Score Trend  │  │ Findings   │    │
│  │ Overview          │  │ Charts       │  │ Detail     │    │
│  └───────────────────┘  └──────────────┘  └────────────┘    │
└──────────────────────────────────────────────────────────────┘
```

### Integration with Existing Compliance

This feature extends the existing Compliance module's container checks. Instead of living in a separate codebase, it adds new check suites to the existing scanner engine. The existing `RunContainerSecurity()` function gets upgraded to support the new check categories.

The key difference: existing compliance runs checks **on-demand** per server. The new Container Security Posture adds:
- **Continuous monitoring** — periodic scan (every 5min) on all running containers
- **Per-container identity** — track security posture per container across restart
- **Score history** — not just current state, but trend

---

## 3. Feature Specifications

### F1: Extended Check Suites (25+ checks)

#### Category A: Privilege Escalation (5 checks)

| # | Check | Severity | Description |
|---|-------|----------|-------------|
| A1 | `privileged_mode` | 🔴 Critical | Container running with `--privileged` |
| A2 | `host_pid` | 🔴 Critical | Container shares host PID namespace |
| A3 | `host_network` | 🟡 High | Container uses host network stack |
| A4 | `host_ipc` | 🟡 High | Container shares host IPC namespace |
| A5 | `host_uts` | 🟡 Medium | Container shares host UTS namespace |

#### Category B: Linux Capabilities (6 checks)

| # | Check | Severity | Description |
|---|-------|----------|-------------|
| B1 | `cap_sys_admin` | 🔴 Critical | CAP_SYS_ADMIN = near-root access |
| B2 | `cap_net_raw` | 🟡 High | CAP_NET_RAW = raw sockets, ARP spoofing |
| B3 | `cap_dac_override` | 🟡 High | Bypass file read/write permission checks |
| B4 | `cap_setuid_setgid` | 🟡 Medium | Privilege escalation via setuid/setgid binaries |
| B5 | `cap_sys_ptrace` | 🟡 High | Debug other processes, memory manipulation |
| B6 | `excessive_caps` | 🟢 Low | More than 5 capabilities total (overly permissive) |

#### Category C: Filesystem Security (5 checks)

| # | Check | Severity | Description |
|---|-------|----------|-------------|
| C1 | `read_only_root` | 🟢 Low | Root filesystem NOT read-only |
| C2 | `bind_mount_docker_sock` | 🔴 Critical | Docker socket mounted inside container |
| C3 | `bind_mount_host_procfs` | 🟡 High | Host /proc mounted (container escape risk) |
| C4 | `bind_mount_host_sensitive` | 🟡 High | /etc, /var/run, /root mounted |
| C5 | `tmpfs_noexec` | 🟢 Low | tmpfs mounted without noexec |

#### Category D: Network Security (5 checks)

| # | Check | Severity | Description |
|---|-------|----------|-------------|
| D1 | `host_port_exposed` | 🟡 Medium | Container publishes ports to host (0.0.0.0) |
| D2 | `port_0_0_0_0_binding` | 🟡 Medium | Service binds to all interfaces |
| D3 | `no_network_limits` | 🟢 Low | No network bandwidth limits set |
| D4 | `dns_not_custom` | 🟢 Low | Using default Docker DNS (no custom DNS) |
| D5 | `internal_only_preferable` | 🟢 Info | Container accessible from outside Docker network |

#### Category E: Resource Controls (4 checks)

| # | Check | Severity | Description |
|---|-------|----------|-------------|
| E1 | `memory_limit_unset` | 🟡 Medium | No memory limit → OOM killer or resource exhaustion |
| E2 | `cpu_limit_unset` | 🟡 Medium | No CPU limit → CPU starvation for other containers |
| E3 | `pid_limit_unset` | 🟢 Low | No PID limit → fork bomb risk |
| E4 | `restart_policy_missing` | 🟢 Low | No restart policy → container stays down after crash |

#### Category F: Image Hygiene (3 checks)

| # | Check | Severity | Description |
|---|-------|----------|-------------|
| F1 | `latest_tag` | 🟡 Medium | Using `:latest` tag → non-reproducible deployment |
| F2 | `untrusted_registry` | 🟡 High | Image from non-official/non-trusted registry |
| F3 | `image_older_30d` | 🟢 Low | Image was built > 30 days ago (stale base image) |

### F2: Scoring Engine

Each check has a weight based on severity:
- Critical = 25 points
- High = 15 points
- Medium = 5 points
- Low = 2 points
- Info = 0 points

**Score formula:**
```
score = Σ(passed_weight) / Σ(total_weight) × 100
```

Results in a 0-100% score where 100% = perfectly secure.

**Scoring levels:**
- 90-100%: ✅ Good
- 70-89%: ⚠️ Acceptable
- 50-69%: 🟡 Needs improvement
- 25-49%: 🔴 Poor
- 0-24%: ❌ Critical

### F3: Score Trend

- 7-day and 30-day score delta tracked per container
- "Improving" / "Stable" / "Declining" trend label
- List containers by worst trend (biggest regressions)

### F4: Network Exposure Map

Simple table showing which containers are exposed to what:
- Container name → host port binding → bound IP → accessible from internet?
- Containers using `host` network mode flagged
- Warning if port 0.0.0.0 binding AND container is web-facing

### F5: Drift Detection

On each scan, snapshot the container config and compare to previous snapshot:
- New volume mounts?
- Changed environment variables?
- Changed entrypoint/cmd?
- Changed network mode?
- Report differences as "drift events"

---

## 4. API Design

### REST Endpoints

```
GET    /api/container-security/overview       — Global summary (score, count, stats)
GET    /api/container-security/containers     — Per-container scores (paginated)
GET    /api/container-security/containers/:id — Single container detail + all findings
GET    /api/container-security/findings       — All findings (filterable by severity, category)
GET    /api/container-security/trend          — Score history (7d/30d)
GET    /api/container-security/drift          — Container config drift events
POST   /api/container-security/scan          — Trigger immediate scan
```

### Response Shape (GET /container-security/overview)

```json
{
  "total_containers": 24,
  "scanned_containers": 24,
  "avg_score": 72.4,
  "score_level": "acceptable",
  "distribution": {
    "good": 8,
    "acceptable": 10,
    "needs_improvement": 4,
    "poor": 2,
    "critical": 0
  },
  "critical_findings": 1,
  "high_findings": 7,
  "medium_findings": 15,
  "low_findings": 23,
  "trend_7d": -2.3,
  "trend_label": "declining",
  "top_issues": [
    {"check": "memory_limit_unset", "count": 12, "severity": "medium"},
    {"check": "latest_tag", "count": 8, "severity": "medium"},
    {"check": "privileged_mode", "count": 1, "severity": "critical"}
  ]
}
```

### Response Shape (GET /container-security/containers)

```json
{
  "containers": [
    {
      "id": "abc123def456",
      "name": "anjungan-frontend",
      "server": "server-01",
      "image": "anjungan-frontend:latest",
      "status": "running",
      "score": 88,
      "score_level": "good",
      "critical_findings": 0,
      "high_findings": 1,
      "findings": [
        {"check": "latest_tag", "severity": "medium", "status": "fail"}
      ],
      "last_scan": "2026-06-11T09:30:00Z"
    }
  ]
}
```

---

## 5. Database Schema

```sql
CREATE TABLE IF NOT EXISTS container_security_scans (
    id              TEXT PRIMARY KEY,
    server_id       TEXT NOT NULL,
    container_id    TEXT NOT NULL,
    container_name  TEXT NOT NULL,
    image           TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'running',
    score           INTEGER NOT NULL DEFAULT 0,
    score_level     TEXT NOT NULL DEFAULT 'pending',
    total_checks    INTEGER NOT NULL DEFAULT 0,
    passed_checks   INTEGER NOT NULL DEFAULT 0,
    failed_checks   INTEGER NOT NULL DEFAULT 0,
    config_snapshot JSONB,                                    -- docker inspect output at scan time
    scanned_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS container_security_findings (
    id              TEXT PRIMARY KEY,
    scan_id         TEXT NOT NULL REFERENCES container_security_scans(id) ON DELETE CASCADE,
    container_id    TEXT NOT NULL,
    check_id        TEXT NOT NULL,                           -- e.g. 'privileged_mode'
    category        TEXT NOT NULL,                           -- privilege, capabilities, filesystem, network, resources, image
    severity        TEXT NOT NULL,                           -- critical, high, medium, low, info
    status          TEXT NOT NULL DEFAULT 'fail',            -- pass, fail, warn
    detail          TEXT NOT NULL DEFAULT '',
    remediation     TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS container_drift_events (
    id              TEXT PRIMARY KEY,
    container_id    TEXT NOT NULL,
    server_id       TEXT NOT NULL,
    field_changed   TEXT NOT NULL,                           -- e.g. 'env', 'mounts', 'cmd', 'entrypoint'
    old_value       TEXT,
    new_value       TEXT,
    detected_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    acknowledged    BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX idx_cs_scans_container ON container_security_scans(container_id);
CREATE INDEX idx_cs_scans_server ON container_security_scans(server_id);
CREATE INDEX idx_cs_scans_score ON container_security_scans(score);
CREATE INDEX idx_cs_findings_check ON container_security_findings(check_id);
CREATE INDEX idx_cs_findings_severity ON container_security_findings(severity);
CREATE INDEX idx_cs_drift_container ON container_drift_events(container_id);
```

---

## 6. UX Flow

### Sidebar Placement

```
Security
├── Container Security       (new — main page)
├── Security Events          (future — PRD-security-events.md)
├── SSL Monitors             (existing)
└── Compliance               (existing)
```

### Page: Container Security Overview

```
┌──────────────────────────────────────────────────────────────┐
│  Container Security Posture                 [Scan All] 🔄    │
│  Last scan: 2m ago                                          │
├──────────────────────────────────────────────────────────────┤
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌─────────────────┐ │
│  │ 🛡️ 72    │ │ ✅ 8     │ │ ⚠️ 14    │ │ 🔴 2 poor      │ │
│  │ Avg Score│ │ Good     │ │ Accept   │ │ 0 critical      │ │
│  └──────────┘ └──────────┘ └──────────┘ └─────────────────┘ │
│                                                              │
│  Top Issues (most containers affected)                       │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ 🟡 No memory limit       12 containers  [View All]   │   │
│  │ 🟡 latest tag used       8  containers  [View All]   │   │
│  │ 🔴 Docker sock mounted   1  container   [View All]   │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                              │
│  Containers sorted by score (worst first)                    │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ 🔴 mysql-db     32%  4 failing   server-01  [Detail] │   │
│  │ 🟡 redis-cache  58%  2 failing   server-01  [Detail] │   │
│  │ ✅ anjungan-be  88%  1 failing   server-01  [Detail] │   │
│  │ ✅ postgres     92%  0 failing   server-02  [Detail] │   │
│  └──────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────┘
```

### Page: Container Security Detail

```
┌──────────────────────────────────────────────────────────────┐
│  Container: mysql-db                     Server: server-01  │
│  Image: mysql:8.0                        Score: 32% 🔴     │
├──────────────────────────────────────────────────────────────┤
│  ┌────────────────────────────────────────────────────────┐ │
│  │ 🔴 Critical Findings                                   │ │
│  │ ├─ privilege/privileged_mode  ─── Container is         │ │
│  │ │                                running in privileged │ │
│  │ │                                mode                  │ │
│  │ │                                → Remove --privileged │ │
│  │ │                                  and use granular    │ │
│  │ │                                  capabilities        │ │
│  │ └─ filesystem/bind_mount_docker  ─── /var/run/docker   │ │
│  │                                      .sock mounted     │ │
│  │                                      → Avoid mounting  │ │
│  │                                        docker socket   │ │
│  ├────────────────────────────────────────────────────────┤ │
│  │ 🟡 Medium Findings                                     │ │
│  │ ├─ network/host_port_exposed  ─── Port 3306 bound     │ │
│  │ │                                on 0.0.0.0            │ │
│  │ │                                → Bind to 127.0.0.1  │ │
│  │ └─ resources/memory_unset     ─── No memory limit     │ │
│  │                                  → Set memory_limit   │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  Score Trend                                                 │
│  100 ██████████████████████████████████████████████████      │
│   75 ████████████████████████████████████░░░░░░░░░░░░░░      │
│   50 ██████████████████████████░░░░░░░░░░░░░░░░░░░░░░░░      │
│   25 ████████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░      │
│    0 ████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░      │
│      Jun 05  06   07   08   09   10   11                     │
└──────────────────────────────────────────────────────────────┘
```

---

## 7. Implementation Roadmap

### Phase 1: Backend (3-4 days)

| Task | Effort | Depends On |
|------|--------|-----------|
| Install all check suites (A-F) | 1.5d | Existing compliance container checks |
| Scoring engine | 0.5d | Check suites |
| DB migrations (scans, findings, drift) | 0.5d | — |
| REST endpoints | 1d | Scoring + DB |
| Scan cron (periodic auto-scan) | 0.5d | REST endpoints |

### Phase 2: Frontend (2-3 days)

| Task | Effort | Depends On |
|------|--------|-----------|
| Container Security Overview page | 1d | API ready |
| Container detail page (findings + remediation) | 1d | Overview page |
| Score trend chart | 0.5d | Detail page |
| Drift events view | 0.5d | Detail page |

### Phase 3: Notifications & Polish (1-2 days)

| Task | Effort | Depends On |
|------|--------|-----------|
| Notification on score regression | 0.5d | Notification targets |
| Network exposure map | 1d | — |
| CSV export | 0.5d | Overview page |

**Total:** ~6-9 days

---

## 8. Non-Functional Requirements

| Category | Requirement |
|----------|-------------|
| **Performance** | Scan 100 containers in < 30s (parallel SSH + `docker inspect`) |
| **Resource** | Scan consumes < 10% CPU, < 50MB RAM per server |
| **Accuracy** | Checks based on Docker API response, not inference |
| **History** | Keep last 90 days of scans; keep latest scan indefinitely |
| **Security** | Results visible to all authenticated users; admin-only for unblock/drift ack |

---

## 9. Dependencies & Integration Points

| Dependency | Type | Purpose |
|------------|------|---------|
| **Existing Compliance scanner** | Code | Extend `RunContainerSecurity()` with new checks |
| **Docker API** (via SSH) | Infrastructure | `docker inspect` for runtime config |
| **Server list** (cluster_servers) | DB | Target servers to scan |
| **Notification targets** | Feature | Alert on security regression |
| **Containers page** (existing) | UI | Cross-link from Container Security detail |

### Integration with Other PRDs

| PRD | Integration |
|-----|-------------|
| **PRD-compliance.md** | Extends existing container checks; cross-link findings |
| **PRD-security-events.md** | Drift events + critical findings feed into security events |
| **PRD-container-image-scanning.md** | Combine image CVEs + runtime posture → full container risk view |
| **PRD-notification-engine.md** | Shared targets for security alerts |

---

## 10. Edge Cases & Error Handling

| Scenario | Behavior |
|----------|----------|
| Container restarts between scans | New container_id (Docker) → new tracking entry; old scan archived |
| Container stopped/paused | Mark as "not running", score = N/A, keep last scan for reference |
| Server unreachable | Retry 3x, mark scan failed, report in UI as "stale" |
| Docker API rate limiting | Respect Docker's rate limits; backoff and retry |
| New container started mid-scan | Next scan cycle picks it up (max 5min delay) |
| Check not applicable (e.g., no bind mounts) | Mark as N/A (not counted in scoring) |
| Container with intentionally privileged mode (e.g., monitoring agent) | Allow user to "acknowledge" / whitelist specific findings |

---

## 11. Mockup References

- See `sketches/container-security/` for wireframes
- Layout: similar to Compliance detail page but with per-container focus
- Finding line style: consistent with existing compliance pass/fail badges

---

## 12. Future Considerations

| Feature | Priority | Notes |
|---------|----------|-------|
| **Container image age tracking** | P2 | Alert when base image > 60 days without rebuild |
| **Runtime anomaly detection** | P3 | Falco integration → runtime behavior alerts |
| **Policy-as-code** | P3 | Define per-service security baseline, enforce via check config |
| **Auto-remediation** | P4 | Proposed fix: auto-apply `--read-only`, `--cap-drop=ALL` etc. |
| **Container SBOM generation** | P4 | Generate bill-of-materials per container |
| **Kubernetes support** | P4 | Adapt checks for K8s pod security context |

---

## 13. PRD Cross-References

| PRD | Relationship |
|-----|-------------|
| **PRD-compliance.md** | Parent PRD — this extends the container checks within Compliance |
| **PRD-container-image-scanning.md** | Image CVEs (Trivy) + runtime posture = complete container risk view |
| **PRD-security-events.md** | Critical findings auto-create security events |
| **PRD-uptime-monitoring.md** | Container restart without reason → cross-reference with drift |

---

## 14. References

- Docker Security Documentation: https://docs.docker.com/engine/security/
- CIS Docker Benchmark: https://www.cisecurity.org/benchmark/docker
- OWASP Docker Security: https://cheatsheetseries.owasp.org/cheatsheets/Docker_Security_Cheat_Sheet.html
- Docker Capabilities Reference: https://docs.docker.com/engine/security/security/#linux-kernel-capabilities
