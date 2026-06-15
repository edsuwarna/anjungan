---
title: Container Registry
description: Self-service OCI-compatible container registry powered by Zot.
---

# Container Registry (Zot Self-Service)

Anjungan includes an integrated OCI-compliant container registry powered by [Zot](https://zotregistry.io/).

## Getting Your Credentials

Every authenticated Anjungan user gets a personal registry account automatically.

### Via Dashboard

1. Navigate to **Registry** > **My Credentials**
2. Your username and password are shown once (auto-created on first access)
3. Save the password — it is **not stored in plaintext** and cannot be recovered

### Via API

```bash
# Auto-create and get credentials
curl -H "Authorization: Bearer ***" \
  https://your-instance/api/v1/registry/my-credentials
```

### Reset My Password

```bash
curl -X POST -H "Authorization: Bearer ***" \
  -H "Content-Type: application/json" \
  -d '{"password": "my-new-strong-password"}' \
  https://your-instance/api/v1/registry/my-credentials/reset-password
```

Password requirements: min 8 characters.

## Using the Registry

```bash
# Login
docker login reg.edsuwarna.xyz -u user@example.com

# Tag and push
docker tag my-app:latest reg.edsuwarna.xyz/my-app:latest
docker push reg.edsuwarna.xyz/my-app:latest

# Pull
docker pull reg.edsuwarna.xyz/my-app:latest
```

## Browsing Images

The **Registry** page in the dashboard provides:

### Repo List (Dashboard)

- **KPI Header Cards** — always-visible stats: total repos, total tags, total size
- **Health Status Badge** — real-time Zot connectivity indicator
- **4-tab layout**: Repos, Credentials, Activity, Admin
- **Repository list** — all repos with tag counts, size, last updated

### Repo Detail (`/registry/[name]`)

- **Sortable tag table** — click headers to sort by name, size, date, digest
- **Tag search** — real-time text filter
- **Pagination** — "Load More" button for large repos
- **Pull command** — copy-to-clipboard per tag
- **Multi-arch display** — platform badges for multi-arch images
- **Bulk operations** — multi-select tags, sticky bottom bar

### Tag Detail (`/registry/[name]/[tag]`)

| Tab | Content |
|-----|---------|
| **Info** | OS/Arch, layer count, platforms, digest, pull command |
| **Configuration** | Env vars, CMD, Entrypoint, Exposed Ports, Volumes, Labels |
| **Layers** | Layer timeline with size bars and commands |
| **History** | Layer creation history with timestamps |
| **Vulnerabilities** | CVE severity summary, filter chips, CVE detail list |
| **Raw JSON** | Pretty-printed manifest + config blob viewer |

### Deleting Images

- **Per-tag delete** — delete individual tags (admin)
- **Bulk delete** — select multiple tags, delete all at once
- **Delete repo** — delete entire repository
- **Garbage collection** — manual GC trigger from UI

> **Tag Protection:** Tags can be locked to prevent accidental deletion. Protected tags show a lock icon.

## CVE Vulnerability Scanning

Anjungan integrates with Zot's built-in `zot-ext-cve` extension.

### Checking Vulnerabilities

1. Navigate to a tag detail page (`/registry/[name]/[tag]`)
2. Click the **Vulnerabilities** tab
3. View severity breakdown: Critical, High, Medium, Low
4. Filter by severity using filter chips
5. Browse CVE list with affected packages and fixed versions

## Webhook Notifications

Anjungan can send webhook notifications on registry events.

### Supported Event Types

| Event | Description |
|-------|-------------|
| `tag.push` | A new tag is pushed to the registry |
| `tag.delete` | A tag is deleted from the registry |
| `repo.delete` | An entire repository is deleted |
| `manifest.delete` | A manifest is deleted by digest |

## Role-Based Access Control

| Role | Read | Push | Delete Users | Manage |
|------|------|------|-------------|--------|
| `readonly` | ✅ | ❌ | ❌ | ❌ |
| `deploy` (default) | ✅ | ✅ | ❌ | ❌ |
| `admin` | ✅ | ✅ | ✅ | ✅ |

## Cleanup Policies

Auto-delete old images based on configurable policies (max age, min tags to keep). Configurable via API or Admin tab in the UI.

## How It Works

1. When a user fetches their credentials, Anjungan checks if a linked Zot user exists
2. If not, it creates one with a generated password
3. The htpasswd file is regenerated and Zot restarted
4. Zot reads htpasswd for authentication
5. Access control policies enforce role-based permissions
6. CVE scanning uses Zot's GraphQL search extension
7. Webhooks fire on tag/repo delete events
8. Cleanup policies run on a configurable background ticker

> **Note:** Passwords are stored as bcrypt hashes. The plaintext password is shown **only once** on creation.
