# Anjungan — PRD: Repositories & Deployments

> **Version:** 1.0
> **Status:** ✅ Fully Implemented — deployments CRUD + environments + repositories frontend + backend all done
> **Author:** Endang Suwarna
> **Last Updated:** June 4, 2026

---

## 1. Executive Summary

### Problem Statement

Anjungan currently has sidebar menus for **Repositories** and **Deployments** that are still placeholders — "Coming Soon." Yet these are two core features of an Internal Developer Platform (IDP):

- **Repositories:** Teams/devs definitely have code on GitHub and/or self-hosted git (Gitea/Forgejo). But to view repo status, CI, and deployment linkage, they have to open separate tabs.
- **Deployments:** Every service running on Anjungan servers originates from a repo. But currently there's no record of who deployed what, from which branch, to which environment.

These two features are **interrelated** — a repo is deployed to a specific environment as a service. This linkage is what makes the IDP a *single pane of glass*.

### Target Audience

- **Endang himself** (platform engineer / only user)
- **Future team members** (if there are collaborators later)
- **Self-hosted infra** — Dokploy, VPS, Zot registry

### Goals

| Goal | Metric |
|------|--------|
| View all repos from GitHub + Forgejo in one place | 2 provider connected |
| View CI status without opening GitHub/Forgejo | Badge pass/fail/pending |
| View which deployment comes from which repo | 2-way linkage (repo→deploy, deploy→repo) |
| Create your own environments (not hardcoded) | Environment CRUD |
| Deploy new service from UI | 1-click deploy from modal |

### Non-Goals

- ❌ Not a git client — replacing GitHub UI (commit, branch management, PR review)
- ❌ Not a CI/CD pipeline engine — trigger workflows only, not run them ourselves
- ❌ Not a replacement for ArgoCD / GitOps — manual deploy first, auto-deploy later

---

## 2. Product Overview

### This Feature in Anjungan's Context

```
┌────────────────────────────────────────────────────┐
│                   Anjungan IDP                      │
├────────────┬────────────┬───────────┬──────────────┤
│  Servers   │ Containers │ Registry  │ Compliance   │
│  SSH Keys  │            │           │              │
├────────────┴─────┬──────┴─────┬─────┴──────────────┤
│   Repositories   │           Deployments            │
│  (GitHub/Forgejo)│    (Environment-based)           │
└──────────────────┴──────────────────────────────────┘
         │                    │
         └────── 🔗 2-way linkage ──────┘
```

### Tech Stack

| Layer | Technology | Reason |
|-------|-----------|--------|
| Backend | Go (existing Anjungan API) | Uses `GitProvider` interface adapter pattern |
| Frontend | SvelteKit (existing) | New routes at `/repositories` and `/deployments` |
| Database | PostgreSQL (existing) | New tables: `environments`, `deployments`, `repo_connections` |
| Auth | JWT (existing) | Per-user token for provider connection |
| Git Provider API | GitHub REST API v3, Forgejo/Gitea API | Direct communication from backend |

---

## 3. Feature Requirements

### 3.1 Feature Inventory — Current State (June 2026)

| Domain | Backend | Frontend | Status |
|--------|---------|----------|--------|
| Repositories — List repos | ✅ Done | ✅ Done | ✅ Fully implemented |
| Repositories — Multi-provider (GitHub + Forgejo) | ✅ Done | ✅ Done | ✅ GitHub + Forgejo connections |
| Repositories — CI status | ✅ Done | ✅ Done | ✅ Badge pass/fail/pending |
| Repositories — Detail page | ✅ Done | ✅ Done | ✅ Full repo detail with branches, deployments |
| Deployments — List | ✅ Done | ✅ Done | ✅ Filter by environment |
| Deployments — Create | ✅ Done | ✅ Done | ✅ New deployment modal |
| Deployments — Detail/Get | ✅ Done | ✅ Done | ✅ Full deployment detail view |
| Deployments — Rollback | ✅ Done | ✅ Done | ✅ Rollback with confirmation |
| Deployments — History | ✅ Done | ✅ Done | ✅ Timeline per deployment |
| Deployments — Environment CRUD | ✅ Done | ✅ Done | ✅ Full CRUD + color-coded |
| Repo ↔ Deployment linkage | ✅ Done | ✅ Done | ✅ 2-way linkage |
| Review Apps / Ephemeral Environments | ❌ Not implemented | ❌ Not implemented | 🔴 Future (Phase 3) |
| Workflow Trigger from UI | 🟡 Partial | 🟡 Partial | Route exists, UI integration pending |

### 3.2 Database Schema (New Tables)

```sql
-- Connected git provider accounts (per-user)
CREATE TABLE repo_connections (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id),
    provider    VARCHAR(20) NOT NULL,  -- 'github', 'forgejo'
    label       VARCHAR(100),           -- e.g. "GitHub Personal"
    base_url    VARCHAR(255),           -- Forgejo instance URL, NULL for GitHub
    token_encrypted TEXT NOT NULL,      -- encrypted PAT
    is_active   BOOLEAN DEFAULT true,
    created_at  TIMESTAMPTZ DEFAULT now(),
    updated_at  TIMESTAMPTZ DEFAULT now()
);

-- Environments (user-defined)
CREATE TABLE environments (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(100) NOT NULL,
    color       VARCHAR(7) NOT NULL DEFAULT '#10b981',  -- hex color
    description TEXT,
    is_protected BOOLEAN DEFAULT false,  -- true = can't delete (e.g. Production)
    created_at  TIMESTAMPTZ DEFAULT now(),
    updated_at  TIMESTAMPTZ DEFAULT now()
);

-- Deployments
CREATE TABLE deployments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(200) NOT NULL,         -- e.g. "Anjungan Backend"
    environment_id  UUID REFERENCES environments(id),
    repo_provider   VARCHAR(20) NOT NULL,
    repo_owner      VARCHAR(100) NOT NULL,
    repo_name       VARCHAR(100) NOT NULL,
    branch          VARCHAR(200) NOT NULL,
    commit_sha      VARCHAR(40),
    server_id       UUID REFERENCES servers(id),
    service_name    VARCHAR(200),                   -- Docker service / container name
    image           VARCHAR(500),                   -- full image ref
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
                    -- 'pending', 'deploying', 'running', 'success', 'failed', 'rolled_back'
    deployed_by     UUID REFERENCES users(id),
    deployed_at     TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now(),
    rollback_from   UUID REFERENCES deployments(id) -- previous version
);

-- Deployment history (audit trail)
CREATE TABLE deployment_history (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deployment_id   UUID REFERENCES deployments(id),
    status          VARCHAR(20) NOT NULL,
    message         TEXT,
    created_at      TIMESTAMPTZ DEFAULT now()
);
```

### 3.3 Feature Specs

#### F1 — Repository Multi-Provider Connection (P0)

| Aspect | Detail |
|--------|--------|
| **Priority** | P0 — Must have for v1 |
| **Backend** | `GitProvider` interface: `ListRepos()`, `ListBranches()`, `GetCommitStatus()`, `ListPRs()`, `ListWorkflows()`. Implement `GitHubAdapter` (REST API v3, PAT auth) and `ForgejoAdapter` (Gitea API, PAT + instance URL). Save encrypted token per-user in `repo_connections` table. |
| **Frontend** | Page `/repositories` — grid card layout. Each card: repo name, provider badge (GitHub/Forgejo), branch, last commit, CI status badge, PR count, linked deployments. Filter by provider. Search by name. |
| **UX** | Provider connection via Settings → Connected Accounts → GitHub (PAT) / Forgejo (PAT + instance URL). Connection status visible on repositories page. |

#### F2 — Repository ↔ Deployment Linkage (P0)

| Aspect | Detail |
|--------|--------|
| **Priority** | P0 — Core value of IDP |
| **Backend** | Join query: deployments + repo_connections, grouped by `(repo_provider, repo_owner, repo_name)`. Endpoint: `GET /repositories/{id}/deployments` and `GET /deployments/{id}/repository`. |
| **Frontend** | In repo card: badge "🚀 2 deployments" that can be clicked → see which deployments. In deployment card: source chain `repo/owner → commit → branch → environment` visible. |
| **UX** | Two-way linkage. From repo see deployments. From deployment see repo. |

#### F3 — Tab-Based Deployments Page (P0)

| Aspect | Detail |
|--------|--------|
| **Priority** | P0 |
| **Backend** | `GET /deployments` — list all deployments, filterable by environment_id. `GET /deployments/{id}` — detail. `POST /deployments` — create new. `POST /deployments/{id}/rollback` — rollback. `GET /deployments/history` — audit trail. |
| **Frontend** | Page `/deployments` — tabs by environment. Each tab displays a card-grid of deployments in that environment. Each card: service name, status badge, source chain (repo → branch → environment), server, container count, quick actions (Restart, Logs, Rollback, Inspect). Summary bar per environment (running count, uptime). |
| **UX** | Filter chips inline in tab bar: All / Running / Failed. Search bar. New Deployment modal (flow: environment → repo → branch → commit → server → service name → deploy). |

#### F4 — Custom Environments (P0)

| Aspect | Detail |
|--------|--------|
| **Priority** | P0 |
| **Backend** | CRUD endpoints: `GET /environments`, `POST /environments`, `PUT /environments/{id}`, `DELETE /environments/{id}` (soft delete — deployments become orphaned). Protected flag prevents delete. |
| **Frontend** | Manage Environments panel on deployments page. "+ Add Environment" tab on the far right. Create modal: name, color (color picker), description. Edit modal same. Delete with confirmation. |
| **UX** | Default seed: Production (#ef4444, protected: true), Staging (#eab308), Development (#10b981). Additional: Review Apps (#8b5cf6, auto-cleanup). |

#### F5 — CI Status Badge (P1)

| Aspect | Detail |
|--------|--------|
| **Priority** | P1 |
| **Backend** | `GetCommitStatus(owner, repo, ref)` → return latest status checks from GitHub/Forgejo. Cache 30s. |
| **Frontend** | Badge pass (● Pass / ✅), fail (✕ Fail / ❌), pending (◐ Pending) on each repo card. Color: green/red/yellow. |
| **UX** | Badge can be clicked → detail of failed workflow (link to GitHub/Forgejo). |

#### F6 — Deployment History & Rollback (P1)

| Aspect | Detail |
|--------|--------|
| **Priority** | P1 |
| **Backend** | `deployment_history` table automatically records each status change. Rollback: set deployment status to `rolled_back`, update deployment with `rollback_from` pointing to previous deployment, restore image/commit. |
| **Frontend** | "History" tab in deployment detail panel — chronological timeline. Rollback button on card + detail panel. |
| **UX** | Rollback confirmation: "Rollback Anjungan Backend to commit a3f2c1d from 2h ago?" |

#### F7 — Quick Actions on Deployments (P1)

| Aspect | Detail |
|--------|--------|
| **Priority** | P1 |
| **Backend** | `POST /deployments/{id}/restart` → restart container via server SSH. `POST /deployments/{id}/redeploy` → redeploy with the same image/commit. |
| **Frontend** | Button: ⟳ Restart, 📋 Logs (link to container logs), ↩ Rollback, 🔍 Inspect, ↗ Open Repo. |

#### F8 — Review Apps / Ephemeral Environments (P2)

| Aspect | Detail |
|--------|--------|
| **Priority** | P2 — Future |
| **Backend** | Environment with `auto_cleanup: true`. Auto-create from webhook PR. Auto-delete on PR merge/close. |
| **Frontend** | Special Review Apps tab. Badge "⏳ auto-removed after PR merge". |
| **UX** | Each PR on GitHub → auto-deploy to review-apps environment named `pr-{number}`. |

---

## 4. API Design

### 4.1 Repositories

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | `/api/v1/repositories` | List all repos across all providers | ✅ |
| GET | `/api/v1/repositories/{id}` | Single repo detail | ✅ |
| GET | `/api/v1/repositories/{id}/deployments` | Deployments linked to this repo | ✅ |
| GET | `/api/v1/repositories/{id}/workflows` | List workflows/Actions | ✅ |
| POST | `/api/v1/repositories/{id}/workflows/{workflow_id}/trigger` | Trigger workflow run | ✅ |
| GET | `/api/v1/repo-connections` | List connected provider accounts | ✅ |
| POST | `/api/v1/repo-connections` | Connect new provider | ✅ |
| PUT | `/api/v1/repo-connections/{id}` | Update connection | ✅ |
| DELETE | `/api/v1/repo-connections/{id}` | Disconnect provider | ✅ |

### 4.2 Deployments

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | `/api/v1/environments` | List all environments | ✅ |
| POST | `/api/v1/environments` | Create environment | ✅ Admin |
| PUT | `/api/v1/environments/{id}` | Update environment | ✅ Admin |
| DELETE | `/api/v1/environments/{id}` | Delete (soft) environment | ✅ Admin |
| GET | `/api/v1/deployments` | List deployments (filter: environment_id, status) | ✅ |
| POST | `/api/v1/deployments` | Create new deployment | ✅ |
| GET | `/api/v1/deployments/{id}` | Detail deployment | ✅ |
| POST | `/api/v1/deployments/{id}/restart` | Restart deployment | ✅ |
| POST | `/api/v1/deployments/{id}/rollback` | Rollback deployment | ✅ |
| POST | `/api/v1/deployments/{id}/redeploy` | Redeploy with same config | ✅ |
| GET | `/api/v1/deployments/{id}/history` | Deployment history/timeline | ✅ |

### 4.3 Response Format (Standard)

```json
{
  "success": true,
  "data": { ... },
  "meta": { "total": 10, "page": 1, "limit": 20 }
}
```

### 4.4 Deployment Status Flow

```
pending → deploying → running (success)
                    → failed
                    → rolled_back (from running)
```

---

## 5. UI/UX Design Guidelines

### 5.1 Key Layout

**Repositories Page:**
```
┌─────────────────────────────────────────────────────┐
│ [All] [GitHub] [Forgejo]  [🔍 Search...]           │
│                                                      │
│ Connected: ● GitHub (edsuwarna) ● Forgejo (internal)│
│                                                      │
│ ┌─ edsuwarna/anjungan ─────────────┐ ┌─ ops/infra─┐│
│ │ ● GitHub   ● Pass                │ │ ● Forgejo  ││
│ │ main · a3f2c1d · 2h ago          │ │ main · e5f6g││
│ │ 🔀 3 PRs   🚀 2 deployments    │ │ 🚀 2 deploys││
│ │ [⟳] [↗]                        │ │ [⟳] [↗]    ││
│ └──────────────────────────────────┘ └────────────┘│
└─────────────────────────────────────────────────────┘
```

**Deployments Page:**
```
┌─────────────────────────────────────────────────────┐
│ [Pro] [Stg] [Dev] [Review] [+ Add]  [🔍 Search...] │
│ ─────────────────────────────────────────────────── │
│                                                        │
│ 🔴 Production: 2 deployments                          │
│                                                        │
│ ┌─ Anjungan Backend ─────────────────────────────┐    │
│ │ ● Running                                       │    │
│ │ 📎 edsuwarna/anjungan → a3f2c1d → main → prod  │    │
│ │ 📦 peladen-central · ⎔ 3 containers · 🕐 2h    │    │
│ │ [⟳ Restart] [📋 Logs] [↩ Rollback] [🔍 Inspect]│    │
│ └────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────┘
```

### 5.2 Color Semantics (Environment)

| Environment | Color | Hex | Usage |
|------------|-------|-----|-------|
| Production | Red | #ef4444 | dot, tab underline, border |
| Staging | Yellow | #eab308 | dot, tab underline |
| Development | Green | #10b981 | dot, tab underline |
| Review Apps | Purple | #8b5cf6 | dot, tab underline |

### 5.3 Status Badge Semantics

| Status | Badge Style | Color |
|--------|------------|-------|
| Running / Success | ● Pass / ✓ | #10b981 |
| Failed | ✕ Fail | #ef4444 |
| Pending / Deploying | ◐ Pending | #818cf8 |
| Rolled Back | ↩ Rolled Back | #eab308 |

### 5.4 Mockup Screenshots

#### Repositories — Varian A: Card Explorer

![Card Explorer — repo list](sketches/repositories/screenshots/variant-a-card-explorer.png)

Card-based layout, each repo as a card with complete info (CI status, branch, PRs, linked deployments). Filter by provider, search, connected accounts status bar.

![Card Explorer — detail expanded](sketches/repositories/screenshots/variant-a-detail-expanded.png)

Click card → expand detail panel: source info, deployment linkage, recent commits, quick actions (Trigger Workflow, Deploy Branch, Open on GitHub).

#### Repositories — Variant B: Compact Workspace

![Compact Workspace — table view](sketches/repositories/screenshots/variant-b-compact-table.png)

Table-based layout, more compact, multi-select checkbox + bulk actions. Provider tabs (All/GitHub/Forgejo) + count per provider.

#### Deployments (v2) — Tab-based + Custom Environments

![Production tab](sketches/deployments-v2/screenshots/01-production-tab.png)

Environment tabs (🔴 Production, 🟡 Staging, 🟢 Dev, 🟣 Review Apps, + Add). Summary bar (running count, uptime). Each card: service name, status badge, source chain (repo → commit → branch → environment), server, containers, quick actions.

![Staging tab](sketches/deployments-v2/screenshots/02-staging-tab.png)

Switch tab → content changes immediately. Deploying and Failed status visible with actionable buttons (Retry, Logs).

![Manage Environments panel](sketches/deployments-v2/screenshots/03-manage-environments.png)

CRUD panel: color-coded environment list, edit/delete buttons. Protected environments (Production) cannot be deleted. "+ Add Environment" button.

![New Deployment modal](sketches/deployments-v2/screenshots/04-new-deployment-modal.png)

Modal: pilih Environment → Repository → Branch → Commit → Server → Service Name → Deploy.

#### Mockup HTML Files

- `sketches/repositories/mockup.html` — 2 varian (Card Explorer + Compact Workspace)
- `sketches/deployments/mockup.html` — 2 varian (Pipeline Cards + Timeline)
- `sketches/deployments-v2/mockup.html` — Final: Tab-based + Custom Environments

---

## 6. Non-Functional Requirements

| Aspect | Target | Notes |
|--------|--------|-------|
| API latency | < 500ms | GitHub/Forgejo API call async, cache 30s |
| Token storage | Encrypted at rest | AES-256-GCM in PostgreSQL |
| Rate limit | Respect GitHub API rate limit | 5000 req/hour, cache aggressively |
| Error handling | Graceful fallback | Provider offline → "Connection lost" not error 500 |
| Deployment count | Scalable to 50+ deployments | Virtual scrolling or pagination |

---

## 7. Implementation Roadmap

### Phase 1: Foundation (v1.0)

> **Goal:** Repo list + basic deployments with environments

| Order | Feature | Effort | Dependency |
|-------|---------|--------|-----------|
| 1 | Database migrations (environments, deployments, repo_connections) | 1 day | — |
| 2 | `GitProvider` interface + `GitHubAdapter` + `ForgejoAdapter` | 3 days | #1 |
| 3 | `GET /repositories` — list repos, merge GitHub + Forgejo | 2 days | #2 |
| 4 | Repo connection flow (per-user PAT + instance URL) | 2 days | #2 |
| 5 | Repositories frontend page (card layout + provider filter) | 2 days | #3, #4 |
| 6 | Environments CRUD (backend + frontend panel) | 2 days | #1 |
| 7 | Deployments CRUD (backend) | 3 days | #1 |
| 8 | Deployments frontend page (tabs by environment) | 2 days | #6, #7 |
| 9 | CI status badge (GitHub check runs) | 1 day | #2 |
| 10 | Repo ↔ Deployment linkage (2-way) | 2 days | #3, #7 |
| 11 | New Deployment modal flow | 1 day | #7, #8 |

**Total Phase 1:** ~19 days

### Phase 2: Operations (v1.1)

| Order | Feature | Effort |
|-------|---------|--------|
| 1 | Deployment history + audit trail | 2 days |
| 2 | Rollback flow | 2 days |
| 3 | Quick actions (Restart, Redeploy, Logs) | 2 days |
| 4 | Workflow trigger from UI | 1 day |

### Phase 3: Future

| Feature | Notes |
|---------|-------|
| Review Apps / ephemeral environments | Auto-deploy from PR |
| Webhook integration | Auto-deploy on push to branch |
| Deployment scheduling | "Deploy at 2AM" |
| Rollback comparison | Diff between current and previous |
| GitLab provider | If needed later |

---

## 8. Design Decisions

### 8.1 Multi-Provider, Not Just GitHub

- **Why:** Endang uses GitHub + possibly self-hosted Forgejo on his own infra
- **Pattern:** `GitProvider` interface in Go, each provider implements a different adapter
- **Trade-off:** More initial effort, but scalable

### 8.2 Per-User Auth, Not Global Token

- **Why:** More secure, each user only sees repos they have access to
- **Pattern:** `repo_connections` table with encrypted PAT per user
- **Trade-off:** Each user must connect their own accounts

### 8.3 Tabs, Not Grouped Sections

- **Why:** More scalable when there are many environments, focus per environment
- **Pattern:** Tabs row with color-coded dot, "+ Add Environment" tab at the end
- **Trade-off:** Cross-environment comparison needs toggle/split view later

### 8.4 Custom Environments, Not Hardcoded

- **Why:** Everyone's setup is different
- **Pattern:** CRUD with color picker, protected flag for delete protection
- **Default seeds:** Production (red, protected), Staging (yellow), Development (green)

### 8.5 Soft Delete for Environments

- **Why:** If Production environment gets deleted = disaster. Orphaned deployments must not disappear.
- **Pattern:** `is_protected` flag + orphaned deployment status
- **Trade-off:** Needs cleanup task to clean up orphaned deployments

---

## 9. Glossary

|| Term | Definition |
||------|-----------|
|| **Provider** | Git hosting service (GitHub, Forgejo, GitLab) |
|| **Adapter** | Go interface implementation for a single provider |
|| **Environment** | Logical deployment target (Production, Staging, Dev, etc.) |
|| **Deployment** | A service running on a server from a repo |
|| **Source Chain** | Visual path: `repo → commit → branch → environment` |
|| **Protected Environment** | Environment that cannot be deleted (usually Production) |
|| **Orphaned Deployment** | Deployment whose environment has been deleted |
|| **Review App** | Ephemeral deployment from a PR, auto-cleanup on merge |

---

## 10. Related Documents

- [README.md](../README.md)
- [Sidebar.svelte](../frontend/src/lib/components/layout/Sidebar.svelte) — existing sidebar structure
- [api.svelte.js](../frontend/src/lib/api.svelte.js) — existing API client stubs
- [repository/handler.go](../backend/internal/repository/handler.go) — existing backend stubs
- [deployment/handler.go](../backend/internal/deployment/handler.go) — existing backend stubs
- `sketches/repositories/mockup.html` — UI mockup repositori
- `sketches/deployments-v2/mockup.html` — UI mockup deployment final
