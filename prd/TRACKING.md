# Anjungan — Feature Implementation Tracking

> Auto-tracked from `main` branch. Cross-references every PRD against implementation.
> Last updated: June 2026 | 19 DB migrations | 11 backend handler packages | 20 frontend route pages

---

## Status Key

| Icon | Meaning |
|------|---------|
| ✅ Done | Fully implemented on `main` — backend + frontend complete |
| 🟡 Partial | Partially implemented — backend done, frontend incomplete, or vice versa |
| ❌ Not Started | PRD exists but zero implementation on `main` |
| 🔴 Planned | Mentioned as future work in PRD, no PRD file yet |

---

## 1. Foundation — Auth & Core (PRD.md)

| Feature | PRD Source | Backend | Frontend | DB Migration | Notes |
|---------|-----------|---------|----------|-------------|-------|
| Auth JWT (login, register, refresh, logout, me) | PRD.md §3.1 | ✅ | ✅ | 000001 | JWT access+refresh, bcrypt |
| TOTP 2FA | PRD.md §3.1 | ✅ | 🟡 | 000001, 000008 | Backend: verify endpoint done. Frontend: login 2FA input not fully integrated |
| Dashboard (summary) | PRD.md §3.1 | ✅ | ✅ | — | Server count, user count, status dist |
| Admin Users CRUD | PRD.md §3.1 | ✅ | ✅ | 000001 | List/create/get/update/delete/unlock |
| Audit Log | PRD.md §3.1 | ✅ | ✅ | 000006 | Filter by action/entity/user/date, export CSV/JSON |
| SSH Terminal (WebSocket) | PRD.md §3.1 | ✅ | ✅ | — | Server-level + container-level, xterm.js |
| Docker Compose Management | PRD.md §F1.7 | ❌ | ❌ | — | Compose up/down/status/ps via SSH — not implemented |
| CLI Tool | PRD.md §F5.1 | ❌ | ❌ | — | `anjungan deploy` CLI — not implemented |
| Developer REST API / Swagger | PRD.md §F5.2 | ❌ | ❌ | — | OpenAPI spec at `/docs` — not implemented |
| Terraform / OpenTofu Integration | PRD.md §F5.3 | ❌ | ❌ | — | Not implemented |

---

## 2. Servers & Infrastructure (PRD.md)

| Feature | PRD Source | Backend | Frontend | DB Migration | Notes |
|---------|-----------|---------|----------|-------------|-------|
| Servers CRUD | PRD.md §3.1, §F1.1 | ✅ | ✅ | 000002–000004 | List/create/get/update/delete, test connection, groups/regions/types |
| Server Metrics (CPU, RAM, Disk) | PRD.md §3.1 | ✅ | ✅ | 000003, 000004 | Real-time + history with `server_metrics` table. Alerts system |
| Server Detect Info | PRD.md §F1.2 | ✅ | ✅ | — | OS info, CPU info auto-detection |
| Bulk Delete Servers | PRD.md §F1.1 | ✅ | ✅ | — | `POST /api/v1/servers/bulk-delete` |

---

## 3. Containers (PRD.md)

| Feature | PRD Source | Backend | Frontend | DB Migration | Notes |
|---------|-----------|---------|----------|-------------|-------|
| Containers Global List | PRD.md §3.1, §F1.3 | ✅ | ✅ | — | `GET /api/v1/containers/` |
| Containers by Server | PRD.md §F1.3 | ✅ | ✅ | — | `GET /api/v1/containers/by-server` + per-server |
| Container Actions (start/stop/restart) | PRD.md §F1.3 | ✅ | ✅ | — | Via SSH `docker` commands |
| Container Logs | PRD.md §F1.4 | ✅ | ✅ | — | Tail logs + WebSocket streaming |
| Container Exec | PRD.md §F1.4 | ✅ | ✅ | — | `POST /exec` + WebSocket interactive terminal |
| Container Inspect | PRD.md §F1.4 | ✅ | ✅ | — | `GET /inspect` |
| Container Stats | PRD.md §F1.3 | ✅ | ✅ | — | Per-container resource stats |
| Container Security Report | PRD.md §3.1 | ✅ | ✅ | 000009–000011 | 10 runtime checks per container |

---

## 4. Compliance & Security Scanning (PRD-compliance.md)

| Feature | PRD Source | Backend | Frontend | DB Migration | Notes |
|---------|-----------|---------|----------|-------------|-------|
| CIS Level 1 (58 checks, 7 categories) | PRD-compliance.md §F2 | ✅ | ✅ | 000009–000011 | SSH, Kernel, FS, Users, Services, Network, Logging |
| CIS Level 2 | PRD-compliance.md §F2 | ✅ | ✅ | 000009–000011 | Extended checks |
| CIS Docker Benchmark (22 checks) | PRD-compliance.md §F3 | ✅ | ✅ | 000009–000011 | 6 sections, auto-detect Docker |
| Lynis System Audit | PRD-compliance.md §F4 | ✅ | ✅ | 000009–000011 | SSH + JSON parse, hardening index |
| Container Security Scanner (10 checks) | PRD-compliance.md §F5 | ✅ | ✅ | 000009–000011 | Privileged, root, seccomp, capabilities, etc. |
| Compliance Dashboard | PRD-compliance.md §F6 | ✅ | ✅ | — | KPI cards, benchmark cards, server list |
| Scan History | PRD-compliance.md §F6 | ✅ | ✅ | 000009–000011 | Per-server, per-category, global |
| **Trivy Vulnerability Scanner** | PRD-container-image-scanning.md §F2 | ❌ | ❌ | — | Agent-based image vulnerability scanning — not implemented |
| **TruffleHog Secret Scanner** | PRD-secret-scanning.md §F6 | ❌ | ❌ | — | Git + filesystem + webhook — not implemented |
| Scheduled Scans | PRD-compliance.md §F8 | ❌ | ❌ | — | Cron-based — not implemented |
| Compliance Report Export (PDF) | PRD-compliance.md §F9 | ❌ | ❌ | — | Not implemented |
| Compliance Trend Graph | PRD-compliance.md §F10 | ❌ | ❌ | — | Time series chart — not implemented |
| Kubernetes Compliance | PRD-compliance.md §F11 | ❌ | ❌ | — | Not implemented |

---

## 5. Registry — Zot (PRD-registry.md)

| Feature | PRD Source | Backend | Frontend | DB Migration | Notes |
|---------|-----------|---------|----------|-------------|-------|
| Repository Browser | PRD-registry.md §F1 | ✅ | ✅ | — | List repos + tags, paginated |
| Image Detail (config, layers, history) | PRD-registry.md §F2 | ✅ | ✅ | — | Tabs: Config, Layers, History |
| Image Deletion | PRD-registry.md §F3 | ✅ | ✅ | — | Delete tag/manifest by digest |
| Garbage Collection | PRD-registry.md §F3 | ✅ | ✅ | — | Trigger Zot GC from UI |
| Self-Service Credentials | PRD-registry.md §F4 | ✅ | ✅ | 000012–000013 | Auto-create registry user per Anjungan user |
| Registry User Management (Admin) | PRD-registry.md §F5 | ✅ | ✅ | 000012–000013 | CRUD + reset password + htpasswd sync |
| Registry Config | PRD-registry.md §F6 | ✅ | ✅ | — | Read-only config display |
| Webhook Notifications | PRD-registry.md §F7 | ❌ | ❌ | — | Push/pull/delete events — not implemented |
| Multi-Registry Support | PRD-registry.md §F8 | ❌ | ❌ | — | Multiple registry endpoints — not implemented |
| Registry Sync / Mirror | PRD-registry.md §F9 | ❌ | ❌ | — | Docker Hub → Zot sync — not implemented |
| Cleanup Policies | PRD-registry.md §F10 | ❌ | ❌ | — | Auto-delete old tags — not implemented |
| Built-in Vulnerability Scan | PRD-registry.md §F11 | ❌ | ❌ | — | Zot-ext-cve or Trivy integration — not implemented |

---

## 6. Repositories & Deployments (PRD-repositories-deployments.md)

| Feature | PRD Source | Backend | Frontend | DB Migration | Notes |
|---------|-----------|---------|----------|-------------|-------|
| Multi-Provider Connection (GitHub + Forgejo) | PRD-repositories-deployments.md §F1 | ✅ | ✅ | 000015, 000018–000019 | PAT auth, encrypted tokens |
| Repository Listing (all providers) | PRD-repositories-deployments.md §F1 | ✅ | ✅ | — | Merged from GitHub + Forgejo |
| Branch Listing | PRD-repositories-deployments.md §F1 | ✅ | ✅ | — | Per-repo branch browser |
| CI Status Badge | PRD-repositories-deployments.md §F5 | ✅ | ✅ | — | Pass/fail/pending from GitHub checks |
| Repo ↔ Deployment Linkage (2-way) | PRD-repositories-deployments.md §F2 | ✅ | ✅ | 000016 | Cross-reference from both sides |
| Environments CRUD | PRD-repositories-deployments.md §F4 | ✅ | ✅ | 000014 | Color-coded, protected flag. Seeds: Production, Staging, Dev |
| Deployments Create | PRD-repositories-deployments.md §F3 | ✅ | 🟡 | 000016 | Backend done. Frontend basic — UI refinements pending |
| Deployments List (by env tabs) | PRD-repositories-deployments.md §F3 | ✅ | ✅ | — | Tab-based environment filter |
| Deployment Restart | PRD-repositories-deployments.md §F7 | ✅ | ✅ | — | Via server SSH `docker restart` |
| Deployment Redeploy | PRD-repositories-deployments.md §F7 | ✅ | ✅ | — | Same image/commit redeploy |
| Deployment Rollback | PRD-repositories-deployments.md §F6 | ✅ | ✅ | 000017 | Rollback to previous deployment |
| Deployment History | PRD-repositories-deployments.md §F6 | ✅ | ✅ | 000017 | Timeline with status + message per step |
| Quick Actions (Logs, Inspect) | PRD-repositories-deployments.md §F7 | ✅ | ✅ | — | Link to container logs, inspect |
| Workflow Trigger from UI | PRD-repositories-deployments.md §F1 | ✅ | ✅ | — | Trigger GitHub Actions workflow |
| Review Apps / Ephemeral Environments | PRD-repositories-deployments.md §F8 | ❌ | ❌ | — | Auto-deploy from PR, auto-cleanup — not implemented |
| Webhook Integration (auto-deploy) | PRD-repositories-deployments.md §Phase 3 | ❌ | ❌ | — | Auto-deploy on push — not implemented |
| Deployment Scheduling | PRD-repositories-deployments.md §Phase 3 | ❌ | ❌ | — | "Deploy at 2AM" — not implemented |
| GitLab Provider | PRD-repositories-deployments.md §Phase 3 | ❌ | ❌ | — | If needed later — not implemented |

---

## 7. Agent System (PRD-anj-agent.md)

| Feature | PRD Source | Backend | Frontend | DB Migration | Notes |
|---------|-----------|---------|----------|-------------|-------|
| Hybrid Connection Type (SSH + Agent) | PRD-anj-agent.md §F1 | ❌ | ❌ | — | Server model `connection_type` field — not implemented |
| Agent Registration Flow | PRD-anj-agent.md §F2 | ❌ | ❌ | — | One-time token, WS registration — not implemented |
| Agent Gateway (WebSocket Server) | PRD-anj-agent.md §F3 | ❌ | ❌ | — | `/ws/agent/{id}` — not implemented |
| Executor Abstraction | PRD-anj-agent.md §F4 | ❌ | ❌ | — | `Executor` interface (SSH + Agent) — not implemented |
| Deployment Options (binary/docker/compose) | PRD-anj-agent.md §F5 | ❌ | ❌ | — | One-liner install commands — not implemented |
| Agent Management UI | PRD-anj-agent.md §F6 | ❌ | ❌ | — | `/agents` page — not implemented |
| Capabilities Discovery | PRD-anj-agent.md §F7 | ❌ | ❌ | — | exec, docker, metrics, logs badges — not implemented |
| Heartbeat & Health Monitoring | PRD-anj-agent.md §F8 | ❌ | ❌ | — | 30s heartbeat, timeout detection — not implemented |
| Self-Update | PRD-anj-agent.md §F9 | ❌ | ❌ | — | `upgrade` message → binary download — not implemented |
| File Transfer | PRD-anj-agent.md §F10 | ❌ | ❌ | — | `file_push` / `file_pull` — not implemented |

---

## 8. Domain Management & Multi-Server Routing (PRD-domain-management.md)

| Feature | PRD Source | Backend | Frontend | DB Migration | Notes |
|---------|-----------|---------|----------|-------------|-------|
| Cluster Server Registry | PRD-domain-management.md §F6.1 | ❌ | ❌ | — | `cluster_servers` table — not implemented |
| Domain CRUD | PRD-domain-management.md §F6.2 | ❌ | ❌ | — | `domains` table — not implemented |
| Traefik Config Generator | PRD-domain-management.md §F6.3 | ❌ | ❌ | — | YAML generation + atomic write — not implemented |
| SSL Certificate Monitoring | PRD-domain-management.md §F6.4 | ❌ | ❌ | — | Expiry tracking, daily check — not implemented |
| Health Check Dashboard | PRD-domain-management.md §F6.5 | ❌ | ❌ | — | Per-domain health from Traefik API — not implemented |
| WireGuard Integration | PRD-domain-management.md §F6.6 | ❌ | ❌ | — | Tunnel status, handshake age — not implemented |

---

## 9. Resource Usage & Cost Tracking (PRD-resource-usage-cost.md)

| Feature | PRD Source | Backend | Frontend | DB Migration | Notes |
|---------|-----------|---------|----------|-------------|-------|
| Resource Collector (SSH `docker stats`) | PRD-resource-usage-cost.md §F1 | ❌ | ❌ | — | 30s polling goroutine — not implemented |
| Resource Dashboard | PRD-resource-usage-cost.md §F2 | ❌ | ❌ | — | KPI cards, server grids — not implemented |
| Cost Configuration | PRD-resource-usage-cost.md §F3 | ❌ | ❌ | — | `cost_config` table — not implemented |
| Per-Service Cost Breakdown | PRD-resource-usage-cost.md §F4 | ❌ | ❌ | — | Weighted formula — not implemented |
| Trend Analysis (7d/30d/90d) | PRD-resource-usage-cost.md §F5 | ❌ | ❌ | — | Line charts, hourly aggregation — not implemented |
| Optimization Suggestions | PRD-resource-usage-cost.md §F6 | ❌ | ❌ | — | Scale up/down rules — not implemented |
| Export Report (CSV) | PRD-resource-usage-cost.md §F7 | ❌ | ❌ | — | Monthly report — not implemented |

---

## 10. Service Templates & Scaffolding (PRD-templates-scaffolding.md)

| Feature | PRD Source | Backend | Frontend | DB Migration | Notes |
|---------|-----------|---------|----------|-------------|-------|
| Template Registry | PRD-templates-scaffolding.md §F1 | ❌ | ❌ | — | `service_templates` table — not implemented |
| Template Engine (Scaffold) | PRD-templates-scaffolding.md §F2 | ❌ | ❌ | — | Placeholder replacement + file generation — not implemented |
| Variable & Secrets Injection | PRD-templates-scaffolding.md §F3 | ❌ | ❌ | — | Dynamic form from template definition — not implemented |
| Deploy Integration (Dokploy/SSH) | PRD-templates-scaffolding.md §F4 | ❌ | ❌ | — | Scaffold-and-deploy flow — not implemented |
| Custom Template (Save Existing) | PRD-templates-scaffolding.md §F5 | ❌ | ❌ | — | Save service as template — not implemented |
| Template Versioning | PRD-templates-scaffolding.md §F6 | ❌ | ❌ | — | Semver, changelog — not implemented |

---

## 11. Phase 4 — Observability & Ecosystem (PRD.md Future)

| Feature | PRD Source | Backend | Frontend | DB Migration | Notes |
|---------|-----------|---------|----------|-------------|-------|
| Service Health Dashboard | PRD.md §F4.1 | ❌ | ❌ | — | Health check runner, color-coded — not implemented |
| Service Dependency Graph | PRD.md §F4.2 | ❌ | ❌ | — | D3.js force-directed — not implemented |
| Alert Routing (Telegram/email/webhook) | PRD.md §F4.3 | ❌ | ❌ | — | Rule engine + notification channels — not implemented |
| Service Catalog (IDP Core) | PRD.md §F2.1 | ❌ | ❌ | — | `services` table + card grid — not implemented |
| Centralized Vault / Secrets | PRD.md §F3.1 | ❌ | ❌ | — | AES-256-GCM encrypted secrets — not implemented |
| API Key Management | PRD.md §F3.10 | ❌ | ❌ | — | User/service scoped tokens — not implemented |
| Deployment Freeze | PRD.md §F3.11 | ❌ | ❌ | — | Freeze schedule, reject deploy — not implemented |

---

## 12. Database Migration Coverage

| # | Table | Status | PRD |
|---|-------|--------|-----|
| 000001 | `users` | ✅ | PRD.md |
| 000002 | `servers` | ✅ | PRD.md |
| 000003 | `server_metrics` + SSH columns | ✅ | PRD.md |
| 000004 | Server metadata (groups, regions, types) | ✅ | PRD.md |
| 000005 | `ssh_keys` | ✅ | PRD.md |
| 000006 | `audit_logs` | ✅ | PRD.md |
| 000007 | `user_server_groups` | ✅ | PRD.md |
| 000008 | Login lockout columns | ✅ | PRD.md |
| 000009 | `scan_results` + `scan_findings` | ✅ | PRD-compliance.md |
| 000010 | Scan profile column | ✅ | PRD-compliance.md |
| 000011 | Error message on scan results | ✅ | PRD-compliance.md |
| 000012 | `registry_users` | ✅ | PRD-registry.md |
| 000013 | Link registry to anjungan user | ✅ | PRD-registry.md |
| 000014 | `environments` | ✅ | PRD-repositories-deployments.md |
| 000015 | `repo_connections` | ✅ | PRD-repositories-deployments.md |
| 000016 | `deployments` | ✅ | PRD-repositories-deployments.md |
| 000017 | `deployment_history` | ✅ | PRD-repositories-deployments.md |
| 000018 | Repo connection affiliations | ✅ | PRD-repositories-deployments.md |
| 000019 | `repo_selections` | ✅ | PRD-repositories-deployments.md |

### ❌ Missing Tables (Planned in PRDs, Not Migrated)

| Table | Purpose | PRD |
|-------|---------|-----|
| `trivy_scans` | Trivy vulnerability results | PRD-container-image-scanning.md |
| `trufflehog_scans` / `trufflehog_findings` | Secret scan results | PRD-secret-scanning.md |
| `image_assets` | Docker image assets on servers | PRD-container-image-scanning.md |
| `image_scans` | Trivy scan results per image | PRD-container-image-scanning.md |
| `cve_findings` | Individual CVE entries | PRD-container-image-scanning.md |
| `image_scan_schedules` | Image scan schedule config | PRD-container-image-scanning.md |
| `secret_findings` | Individual secret findings | PRD-secret-scanning.md |
| `secret_finding_status_history` | Finding status audit trail | PRD-secret-scanning.md |
| `compliance_schedules` | Scheduled scan config | PRD-compliance.md |
| `cluster_servers` | Cluster node registry | PRD-domain-management.md |
| `domains` | Domain routing rules | PRD-domain-management.md |
| `secrets` | Encrypted vault entries | PRD.md |
| `api_keys` | Developer API tokens | PRD.md |
| `agents` | Agent registrations | PRD-anj-agent.md |
| `services` | Service catalog entries | PRD.md |
| `notifications` | Notification channel config | PRD.md |
| `resource_usage` | Resource usage snapshots | PRD-resource-usage-cost.md |
| `resource_hourly` | Hourly aggregated trends | PRD-resource-usage-cost.md |
| `cost_config` | Server cost configuration | PRD-resource-usage-cost.md |
| `optimization_suggestions` | Auto-generated suggestions | PRD-resource-usage-cost.md |
| `service_templates` | Scaffold templates | PRD-templates-scaffolding.md |
| `template_versions` | Template versioning | PRD-templates-scaffolding.md |
| `scaffold_logs` | Scaffold/deploy audit | PRD-templates-scaffolding.md |
| `deployment_templates` | Deployment scaffold templates | PRD.md |

---

## Summary Counts

| Status | Count |
|--------|-------|
| ✅ Done (fully implemented) | **43** features |
| 🟡 Partial (some gaps) | **3** features |
| ❌ Not Started (PRD exists) | **45** features |
| **Total PRD-documented features** | **91** |

### By Domain

| Domain | ✅ Done | 🟡 Partial | ❌ Not Started |
|--------|--------|-----------|-------------|
| Auth & Core (PRD.md) | 5 | 1 | 4 |
| Servers & Infra (PRD.md) | 4 | 0 | 0 |
| Containers (PRD.md) | 7 | 0 | 0 |
| Registry (PRD-registry.md) | 7 | 0 | 5 |
| Repos & Deployments (PRD-repositories-deployments.md) | 12 | 1 | 4 |
| Compliance & Scanning (PRD-compliance.md) | 6 | 0 | 4 |
| Container Image Scanning (PRD-container-image-scanning.md) | 0 | 0 | 6 |
| Secret Scanning (PRD-secret-scanning.md) | 0 | 0 | 7 |
| Agent System (PRD-anj-agent.md) | 0 | 0 | 10 |
| Domain Management (PRD-domain-management.md) | 0 | 0 | 6 |
| Resource & Cost (PRD-resource-usage-cost.md) | 0 | 0 | 7 |
| Templates (PRD-templates-scaffolding.md) | 0 | 0 | 6 |
| Observability & Ecosystem (PRD.md future) | 0 | 1 | 6 |

### Frontend Route Coverage

| Route | Status | Notes |
|-------|--------|-------|
| `/` (Dashboard) | ✅ | StatCards, server distribution |
| `/login` | ✅ | JWT login form |
| `/servers` | ✅ | Server list |
| `/servers/[id]` | ✅ | Server detail + metrics |
| `/servers/[id]/terminal` | ✅ | xterm.js WebSocket terminal |
| `/containers` | ✅ | Global container list |
| `/containers/[serverId]/[containerId]/security` | ✅ | Container security report |
| `/repositories` | ✅ | Repo list + CI status |
| `/deployments` | ✅ | Tab-based deployment list |
| `/registry` | ✅ | Repo browser + credentials |
| `/registry/[name]/[tag]` | ✅ | Image detail (config/layers/history) |
| `/compliance` | ✅ | Compliance dashboard |
| `/compliance/cis-level-1` | ✅ | CIS L1 detail |
| `/compliance/cis-level-2` | ✅ | CIS L2 detail |
| `/compliance/cis-docker` | ✅ | CIS Docker detail |
| `/compliance/lynis` | ✅ | Lynis hardening index |
| `/admin/users` | ✅ | User management |
| `/admin/audit-log` | ✅ | Audit log viewer |
| `/ssh-keys` | ✅ | SSH key management |
| `/infra/domains` | ❌ | Domain management — not created |
| `/infra/resources` | ❌ | Resource dashboard — not created |
| `/infra/templates` | ❌ | Template scaffold — not created |
| `/agents` | ❌ | Agent management — not created |
| `/services` | ❌ | Service catalog — not created |

---

*Generated from cross-referencing 10 PRD files against `main` branch implementation.
For detailed specs, see individual PRDs in this directory.*
