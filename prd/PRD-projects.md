# Anjungan — PRD: Projects (Multi-Tenant Isolation)

> **Version:** 1.0
> **Status:** 🟡 Active — Branch `feat/projects`
> **Author:** Endang Suwarna
> **Last Updated:** June 10, 2026

---

## 1. Executive Summary

### Problem Statement

Anjungan is currently single-tenant — all servers, SSL monitors, uptime monitors, deployments, and notification targets live in one flat namespace. This creates problems:

- **No isolation** — a developer in Team A sees all servers belonging to Team B
- **Global RBAC is too blunt** — a "viewer" role gives access to every resource, not just the team's
- **No resource grouping** — `server_group` is an ad-hoc text label, not a real entity
- **Can't onboard multiple teams** — every team shares the same pool of servers, monitors, and deployments
- **Deployment boundaries unclear** — which environment belongs to which team?

### What This Solves

| Problem | Solution |
|---------|----------|
| Team A sees Team B's servers | **Project isolation** — each project is a resource boundary |
| Global roles are too broad | **Per-project roles** (admin/developer/viewer) + global super-admin |
| `server_group` is just a string | **Proper `projects` entity** with its own table, members, and settings |
| Can't onboard multiple teams | Each team gets their own project with their own resources |
| No clear deployment ownership | Each deployment, environment, monitor belongs to a project |

### Target Audience

- **Super-admin** (Endang) — create/manage all projects, assign members, cross-project visibility
- **Project admin** — manage servers, monitors, members within their project
- **Project developer** — deploy services, view monitors within their project
- **Project viewer** — read-only access within their project

### Goals

| Goal | Acceptance |
|------|-----------|
| Admins can create/delete projects | Works via UI + API |
| Resources belong to a project | servers, ssl_monitors, uptime_monitors, deployments, environments, notification_targets |
| Users have roles per project | project_members table with admin/developer/viewer |
| Project switcher in top bar | User can switch context without re-login |
| URL scoped to project | `/projects/{slug}/servers`, `/projects/{slug}/uptime`, etc. |
| Default project for existing resources | Migrate all existing data to "Default" project |
| Dashboard per project | Overview scoped to active project |

### Non-Goals

- ❌ No cross-project resource sharing (e.g., server used by two projects)
- ❌ No project hierarchies / sub-projects (v2 consideration)
- ❌ No project-level billing/quota (future)
- ❌ No project-level SSO/provider config (future)

---

## 2. Product Overview

### Architecture

```
┌─────────────────────────────────────────────────────────┐
│                   Anjungan Platform                      │
│                                                         │
│  ┌──────────────────────────────────────────────────┐   │
│  │           Global Super-Admin Area                │   │
│  │  ┌─────────┐ ┌──────────┐ ┌────────────┐        │   │
│  │  │ Users   │ │ Audit    │ │ Projects   │        │   │
│  │  │         │ │ Log      │ │ List       │        │   │
│  │  └─────────┘ └──────────┘ └────────────┘        │   │
│  └──────────────────────────────────────────────────┘   │
│                                                         │
│  ┌────────────┐ ┌────────────┐ ┌────────────┐          │
│  │ Project A  │ │ Project B  │ │ Project C  │          │
│  │ ┌────────┐ │ │ ┌────────┐ │ │ ┌────────┐ │          │
│  │ │Servers │ │ │ │Servers │ │ │ │Servers │ │          │
│  │ │SSL     │ │ │ │SSL     │ │ │ │SSL     │ │          │
│  │ │Uptime  │ │ │ │Uptime  │ │ │ │Uptime  │ │          │
│  │ │Deploy  │ │ │ │Deploy  │ │ │ │Deploy  │ │          │
│  │ │Members │ │ │ │Members │ │ │ │Members │ │          │
│  │ └────────┘ │ │ └────────┘ │ │ └────────┘ │          │
│  └────────────┘ └────────────┘ └────────────┘          │
│                                                         │
│  ┌──────────────────────────────────────────────────┐   │
│  │              Default Project                     │   │
│  │  (migration target for orphaned resources)       │   │
│  └──────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

### Project Switcher Flow

```
┌─────────────────────────────────────────────────┐
│  [Anjungan]  [Project: Acme Corp ▼]  [👤 User]  │
│              ┌─────────────────────┐             │
│              │ ✓ Acme Corp         │             │
│              │   Internal Tools    │             │
│              │   Default Project   │             │
│              │ ────────────────── │             │
│              │   Manage Projects   │             │
│              └─────────────────────┘             │
├─────────────────────────────────────────────────┤
│  /projects/acme-corp/servers                     │
│  /projects/acme-corp/ssl-monitors                │
│  /projects/acme-corp/uptime                      │
└─────────────────────────────────────────────────┘
```

### Key Design Decision: Flat URL with Middleware

All resource routes sit under `/projects/{slug}/...`. A middleware extracts the `slug`, looks up the project, injects `project_id` into request context. This keeps existing handler logic clean — just add `WHERE project_id = $ctx.ProjectID` to all repository queries.

---

## 3. Feature Specifications

### F1 — Project CRUD (Admin Only)

| Aspect | Detail |
|--------|--------|
| **Priority** | P0 — Foundation |
| **Backend** | `projects` table + CRUD handler under `/api/v1/projects` |
| **Frontend** | Admin page at `/admin/projects` with list/create/edit/delete |
| **Auth** | Admin-only for create/update/delete. All users can list projects they're members of |
| **Delete behavior** | Move resources to Default Project (not cascade), show warning with resource count |

### F2 — Project Members (Admin + Project Admin)

| Aspect | Detail |
|--------|--------|
| **Priority** | P0 — Foundation |
| **Backend** | `project_members` table, CRUD under `/api/v1/projects/{id}/members` |
| **Frontend** | Member management in project settings page |
| **Roles** | `admin` (manage project resources + members), `developer` (use resources), `viewer` (read-only) |
| **Inheritance** | Global admin (super-admin) auto-has access to all projects |

### F3 — Resource Scoping

| Aspect | Detail |
|--------|--------|
| **Priority** | P0 — Foundation |
| **Affected models** | `servers`, `ssl_monitors`, `uptime_monitors`, `deployments`, `environments`, `notification_targets` |
| **Migration** | `ALTER TABLE ... ADD COLUMN project_id UUID REFERENCES projects(id) NOT NULL DEFAULT 'default-project-id'` |
| **Query filter** | Every list/get endpoint filters by `project_id` from context |
| **API layer** | All resource endpoints scoped to `/projects/{slug}/...` |

### F4 — Project Dashboard

| Aspect | Detail |
|--------|--------|
| **Priority** | P1 |
| **Backend** | Dashboard summary scoped to project_id |
| **Frontend** | `/projects/{slug}` — overview with project stats |
| **Content** | Server count (by status), uptime summary, SSL summary, recent deployments |

### F5 — Default Project

| Aspect | Detail |
|--------|--------|
| **Priority** | P0 — Migration |
| **Backend** | Created on first migration or system init. Cannot be deleted |
| **Behavior** | All existing resources reassigned to Default Project during migration |

---

## 4. API Design

### Project CRUD

```
GET    /api/v1/projects                                  — List projects (admin: all, user: member of)
POST   /api/v1/projects                                  — Create project (admin only)
GET    /api/v1/projects/{id}                             — Get project detail
PUT    /api/v1/projects/{id}                             — Update project (admin + project admin)
DELETE /api/v1/projects/{id}                             — Delete project (admin only, moves resources to default)
```

**Request (POST/PUT):**
```json
{
  "name": "Acme Corp Platform",
  "slug": "acme-corp",
  "description": "Production infrastructure for Acme Corp services"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "uuid-here",
    "name": "Acme Corp Platform",
    "slug": "acme-corp",
    "description": "Production infrastructure for Acme Corp services",
    "resource_count": {
      "servers": 0,
      "ssl_monitors": 0,
      "uptime_monitors": 0,
      "deployments": 0
    },
    "created_by": "user-uuid",
    "created_at": "2026-06-10T10:00:00Z",
    "updated_at": "2026-06-10T10:00:00Z"
  }
}
```

### Project Members

```
GET    /api/v1/projects/{id}/members                     — List members
POST   /api/v1/projects/{id}/members                     — Add member (admin + project admin)
PUT    /api/v1/projects/{id}/members/{userId}            — Update member role
DELETE /api/v1/projects/{id}/members/{userId}            — Remove member
```

**Request (POST):**
```json
{
  "user_id": "uuid",
  "role": "developer"
}
```

### Scoped Resource Routes

All existing resource endpoints mirrored under project scope:

```
GET    /api/v1/projects/{slug}/servers                   — List servers in project
POST   /api/v1/projects/{slug}/servers                   — Create server in project
GET    /api/v1/projects/{slug}/servers/{id}              — Get server detail
...
GET    /api/v1/projects/{slug}/ssl-monitors              — SSL monitors scoped
GET    /api/v1/projects/{slug}/uptime                    — Uptime monitors scoped
GET    /api/v1/projects/{slug}/deployments               — Deployments scoped
GET    /api/v1/projects/{slug}/environments              — Environments scoped
GET    /api/v1/projects/{slug}/notification-targets      — Notification targets scoped
GET    /api/v1/projects/{slug}/dashboard                 — Project-level dashboard KPI
```

**Middleware pattern in chi:**
```go
r.Route("/api/v1/projects/{slug}", func(r chi.Router) {
    r.Use(h.ProjectContextMiddleware) // extracts slug → project_id → ctx

    r.Route("/servers", func(r chi.Router) {
        r.Get("/", h.ServerHandler.List)
        r.Post("/", h.ServerHandler.Create)
        r.Get("/{id}", h.ServerHandler.Get)
        // ...
    })

    r.Route("/ssl-monitors", func(r chi.Router) {
        r.Get("/", h.SSLHandler.List)
        r.Post("/", h.SSLHandler.Create)
        // ...
    })
})
```

### Delete Project Response

```
DELETE /api/v1/projects/{id}
```
```json
{
  "success": true,
  "data": {
    "project_id": "deleted-uuid",
    "project_name": "Acme Corp Platform",
    "resources_moved": {
      "servers": 5,
      "ssl_monitors": 3,
      "uptime_monitors": 2,
      "deployments": 4,
      "environments": 3,
      "notification_targets": 1
    },
    "moved_to_project_name": "Default Project"
  }
}
```

---

## 5. Database Schema

### New Tables

#### `projects`
```sql
CREATE TABLE projects (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255) NOT NULL,
    slug        VARCHAR(255) NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    created_by  UUID NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_projects_slug ON projects(slug);
```

#### `project_members`
```sql
CREATE TABLE project_members (
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       VARCHAR(50) NOT NULL DEFAULT 'developer',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (project_id, user_id)
);

CREATE INDEX idx_project_members_user ON project_members(user_id);
```

### Migration: Add `project_id` to Existing Tables

```sql
-- Migration 000033
-- UP:
INSERT INTO projects (id, name, slug, description, created_by)
VALUES ('00000000-0000-0000-0000-000000000001', 'Default Project', 'default', 'System default project for legacy resources', (SELECT id FROM users ORDER BY created_at LIMIT 1));

ALTER TABLE servers ADD COLUMN project_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES projects(id);
ALTER TABLE ssl_monitors ADD COLUMN project_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES projects(id);
ALTER TABLE uptime_monitors ADD COLUMN project_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES projects(id);
ALTER TABLE deployments ADD COLUMN project_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES projects(id);
ALTER TABLE environments ADD COLUMN project_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES projects(id);
ALTER TABLE notification_targets ADD COLUMN project_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES projects(id);

CREATE INDEX idx_servers_project ON servers(project_id);
CREATE INDEX idx_ssl_monitors_project ON ssl_monitors(project_id);
CREATE INDEX idx_uptime_monitors_project ON uptime_monitors(project_id);
CREATE INDEX idx_deployments_project ON deployments(project_id);
CREATE INDEX idx_environments_project ON environments(project_id);
CREATE INDEX idx_notification_targets_project ON notification_targets(project_id);

-- DOWN:
DROP INDEX idx_notification_targets_project;
DROP INDEX idx_environments_project;
DROP INDEX idx_deployments_project;
DROP INDEX idx_uptime_monitors_project;
DROP INDEX idx_ssl_monitors_project;
DROP INDEX idx_servers_project;

ALTER TABLE servers DROP COLUMN project_id;
ALTER TABLE ssl_monitors DROP COLUMN project_id;
ALTER TABLE uptime_monitors DROP COLUMN project_id;
ALTER TABLE deployments DROP COLUMN project_id;
ALTER TABLE environments DROP COLUMN project_id;
ALTER TABLE notification_targets DROP COLUMN project_id;

DELETE FROM projects WHERE id = '00000000-0000-0000-0000-000000000001';
DROP TABLE project_members;
DROP TABLE projects;
```

### Authz Check

Every repository method that reads/writes project-scoped resources MUST include `AND project_id = $1` in its WHERE clause, where `$1` comes from request context:

```go
func (r *Repository) ListServers(ctx context.Context, projectID string, query model.ServerListQuery) ([]model.ServerResponse, int, error) {
    baseQuery := `FROM servers WHERE project_id = $1`
    // ... rest of query with projectID as first param
}
```

---

## 6. UX Flow

### Create Project (Admin)

```
Admin → Admin Panel → Projects → "New Project"

┌───────────────────────────────────────────┐
│  Create New Project                       │
│                                           │
│  Name:        [Acme Corp Platform     ]   │
│  Slug:        [acme-corp              ]   │
│  Description: [Production infra for   ]   │
│               [Acme Corp services     ]   │
│                                           │
│  [Cancel]                    [Create]     │
└───────────────────────────────────────────┘

→ Backend validates slug uniqueness
→ Creates project + auto-adds admin as member with "admin" role
→ Redirect to project detail page
```

### Project Switcher

```
Top Bar: [Anjungan] [Acme Corp ▼] [👤]

Dropdown shows:
  ✓ Acme Corp          ← current
    Internal Tools     ← member
    ──────────────
    Manage Projects    ← link (admin only)
```

### Project Overview Page (`/projects/{slug}`)

```
┌──────────────────────────────────────────────────────┐
│  [← All Projects]  Acme Corp Platform  [⚙ Settings] │
│  Description: Production infra for Acme Corp         │
├──────────────────────────────────────────────────────┤
│  ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐                │
│  │  12  │ │  3   │ │  8   │ │  24  │                │
│  │ Srv  │ │ SSL  │ │Uptime│ │Depl  │                │
│  └──────┘ └──────┘ └──────┘ └──────┘                │
│                                                      │
│  Recent Deployments     │  Member Activity           │
│  ┌─────────────────┐   │  ┌─────────────────┐       │
│  │ api-gateway v2.1│   │  │ budi joined     │       │
│  │ web-app v3.0.1  │   │  │ sari added srv X│       │
│  └─────────────────┘   │  └─────────────────┘       │
└──────────────────────────────────────────────────────┘
```

### Delete Project Flow

```
Admin → "/admin/projects" → Click trash icon

┌──────────────────────────────────────────────────────┐
│  ⚠️ Delete "Acme Corp Platform"?                      │
│                                                       │
│  This project has:                                    │
│  • 5 servers                                          │
│  • 3 SSL monitors                                     │
│  • 2 uptime monitors                                  │
│  • 4 deployments                                      │
│                                                       │
│  All resources will be moved to "Default Project".    │
│  This cannot be undone.                               │
│                                                       │
│  Type "delete" to confirm: [____________]             │
│                                                       │
│  [Cancel]                    [Delete Project]         │
└──────────────────────────────────────────────────────┘

→ Backend moves all resources to Default Project
→ Deletes project + member associations
→ Shows success with resource count summary
```

---

## 7. Implementation Roadmap

### Phase 1 — Foundation (P0)

| # | Task | Files | Effort |
|---|------|-------|--------|
| 1 | Migration 000033 — `projects` + `project_members` tables + `project_id` columns | `backend/migrations/000033_*.sql` | Small |
| 2 | Model: `Project`, `ProjectMember` structs | `backend/internal/common/model/model.go` | Small |
| 3 | Repository: Project CRUD + member CRUD | `backend/internal/common/db/repository.go` | Medium |
| 4 | Handler: Project CRUD endpoints + member endpoints | `backend/internal/project/handler.go` | Medium |
| 5 | Register routes in server.go | `backend/internal/server/server.go` | Small |
| 6 | `ProjectContextMiddleware` — slug → project_id → ctx | `backend/internal/project/middleware.go` | Small |
| 7 | API client: project + member methods | `frontend/src/lib/api.svelte.js` | Small |

### Phase 2 — Resource Scoping (P0)

| # | Task | Files | Effort |
|---|------|-------|--------|
| 8 | Add `project_id` filter to server repository queries | `backend/internal/common/db/repository.go` | Medium |
| 9 | Add `project_id` filter to SSL monitor repository queries | `backend/internal/common/db/repository.go` | Medium |
| 10 | Add `project_id` filter to uptime monitor repository queries | `backend/internal/common/db/repository.go` | Medium |
| 11 | Add `project_id` filter to deployment repository queries | `backend/internal/common/db/repository.go` | Medium |
| 12 | Add `project_id` filter to environment repository queries | `backend/internal/common/db/repository.go` | Medium |
| 13 | Add `project_id` filter to notification target queries | `backend/internal/common/db/repository.go` | Medium |
| 14 | Register scoped routes under `/api/v1/projects/{slug}/...` | `backend/internal/server/server.go` | Medium |

### Phase 3 — Frontend (P0)

| # | Task | Files | Effort |
|---|------|-------|--------|
| 15 | Project list page (admin) | `frontend/src/routes/admin/projects/+page.svelte` | Medium |
| 16 | Create/edit project modal | Shared component | Medium |
| 17 | Project switcher in top bar | `frontend/src/lib/components/layout/TopBar.svelte` | Medium |
| 18 | Project overview/dashboard page | `frontend/src/routes/projects/[slug]/+page.svelte` | Medium |
| 19 | Scoped server page at `/projects/{slug}/servers` | Route + adapter | Medium |
| 20 | Scoped SSL monitors page at `/projects/{slug}/ssl-monitors` | Route + adapter | Medium |
| 21 | Scoped uptime page at `/projects/{slug}/uptime` | Route + adapter | Medium |
| 22 | Scoped deployments page at `/projects/{slug}/deployments` | Route + adapter | Medium |
| 23 | Project settings (members, edit) | `frontend/src/routes/projects/[slug]/settings/+page.svelte` | Medium |

### Phase 4 — Deletion & Cleanup (P1)

| # | Task | Files | Effort |
|---|------|-------|--------|
| 24 | Delete project handler — move resources + warning | `backend/internal/project/handler.go` | Medium |
| 25 | Delete project frontend flow — confirm modal | Frontend confirmation dialog | Small |

---

## 8. Non-Functional Requirements

| Requirement | Target |
|-------------|--------|
| **Performance** | Project context lookup < 5ms (cached after first fetch per request) |
| **Isolation** | A user in Project A MUST NOT see Project B resources unless they're a member |
| **Backward compatibility** | All existing API routes continue working (default project context) |
| **URL stability** | Project slug is immutable after creation (to prevent broken bookmarks) |
| **Data safety** | Delete project NEVER cascade-deletes resources — always rehome to default |
| **Audit** | All project CRUD + member changes logged to audit_log |

---

## 9. Dependencies & Integration Points

| Dependency | Type | Notes |
|-----------|------|-------|
| `project_ctx` middleware | New | Must run before auth middleware on scoped routes |
| Existing handlers | Modify | All resource list/get/create handlers need project_id param |
| Existing repository | Modify | All resource queries need `WHERE project_id = $N` |
| Frontend stores | Modify | Add `currentProject` store alongside `user` store |
| Sidebar + TopBar | Modify | Project switcher in top bar, sidebar filters by project |

---

## 10. Edge Cases & Error Handling

| Scenario | Behavior |
|----------|----------|
| **Slug collision** | Return 409 Conflict with message "slug already exists" |
| **Delete project with resources** | Show warning modal with resource count, require typing "delete" to confirm |
| **Delete Default Project** | 400 Bad Request — "Cannot delete the Default Project" |
| **User removed from project** | Immediately lose access — next page navigation shows "project not found" |
| **Slug contains invalid chars** | Validate: only lowercase alphanumeric + hyphens |
| **Project context without slug** | Fall back to Default Project for backward-compatible API routes |
| **Super-admin in any project** | Bypass member check — global `role = admin` auto-grants access to all projects |
| **Member with same role added twice** | Upsert — update role if already exists |

---

## 11. Future Considerations

| Feature | When | Why Skip Now |
|---------|------|-------------|
| **Project hierarchy (sub-projects)** | v2 | Not needed yet — flat model works for current scale |
| **Cross-project resource sharing** | v2 | Complex permission model — start with strict isolation |
| **Project-level compliance policies** | v2 | E.g., "all servers in Project X must pass CIS Level 1" |
| **Project-level SSO/OIDC** | v2 | Not needed until multi-org support |
| **Project import/export** | v2 | Zip export of project config for backup |
| **Usage quotas per project** | v3 | Track server count, monitor count, deployment frequency |

---

## 12. PRD Cross-References

| PRD | Relationship |
|-----|-------------|
| [PRD.md](PRD.md) | Master PRD — Project isolation extends the multi-user model from Phase 1 |
| [PRD-uptime-monitoring.md](PRD-uptime-monitoring.md) | Uptime monitors become project-scoped |
| [PRD-ssl-monitoring.md](PRD-ssl-monitoring.md) | SSL monitors become project-scoped |
| [PRD-repositories-deployments.md](PRD-repositories-deployments.md) | Deployments + environments become project-scoped |

---

## 13. References

- **Google Cloud Projects:** https://cloud.google.com/docs/overview#projects
- **AWS Organizations:** https://docs.aws.amazon.com/organizations/latest/userguide/orgs_introduction.html
- **Current Anjungan models:** `backend/internal/common/model/model.go`
- **Current sidebar categories:** `frontend/src/lib/components/layout/Sidebar.svelte`
- **TopBar layout:** `frontend/src/lib/components/layout/TopBar.svelte`
- **Existing route pattern:** `backend/internal/server/server.go`
