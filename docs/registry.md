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

# Response
{
  "url": "reg.edsuwarna.xyz",
  "username": "user@example.com",
  "password": "aB3xK9mP2..." // shown once on creation
}
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

The Registry page in the dashboard provides:

### Repo List (Dashboard)

- **KPI Header Cards** — always-visible stats: total repos, total tags, total size
- **Health Status Badge** — real-time Zot connectivity indicator (green = online, red = down)
- **4-tab layout**: Repos, Credentials, Activity, Admin
- **Repository list** — all repos with tag counts, size, last updated
- **Size Dashboard** — per-repo storage breakdown sorted by size

### Repo Detail (`/registry/[name]`)

- **Sortable tag table** — click header columns to sort by name, size, created date, digest
- **Tag search** — real-time text filter against backend (`?q=` parameter)
- **Pagination** — "Load More" button for large repos
- **Pull command** — copy-to-clipboard per tag (`docker pull ...`)
- **Multi-arch display** — platform badges for multi-arch images
- **Bulk operations** — multi-select tags → Protect All / Unprotect All / Delete All via sticky bottom bar

### Tag Detail (`/registry/[name]/[tag]`)

| Tab | Content |
|-----|---------|
| **Info** | OS/Arch, layer count, platforms, digest, config digest, pull command |
| **Configuration** | Environment variables, CMD, Entrypoint, Exposed Ports, Volumes, Labels |
| **Layers** | Layer timeline with size bars and commands |
| **History** | Layer creation history with timestamps |
| **Vulnerabilities** | CVE severity summary cards (Critical/High/Medium/Low), severity filter chips, CVE detail list with package info, NVD links, Load More pagination |
| **Raw JSON** | Pretty-printed manifest + config blob viewer with copy-to-clipboard |

### Deleting Images

- **Per-tag delete** — delete individual tags (admin)
- **Bulk delete** — select multiple tags, delete all at once
- **Delete repo** — delete entire repository (all tags), skips protected tags
- **Garbage collection** — manual GC trigger from UI

> **Tag Protection:** Tags can be locked to prevent accidental deletion. Protected tags show a lock icon and cannot be deleted until unprotected.

## CVE Vulnerability Scanning

Anjungan integrates with Zot's built-in `zot-ext-cve` extension to provide vulnerability scanning for container images.

### Checking Vulnerabilities

1. Navigate to a tag detail page (`/registry/[name]/[tag]`)
2. Click the **Vulnerabilities** tab
3. View severity breakdown: Critical (🔴), High (🟠), Medium (🟡), Low (🟢)
4. Filter by severity using the filter chips
5. Browse CVE list with affected packages, installed version, and fixed version
6. Click any CVE ID to open the NVD detail page

### API

```bash
# Check if CVE extension is available
curl https://your-instance/api/v1/registry/cve/check

# Get CVE details for a specific tag (with pagination)
curl https://your-instance/api/v1/registry/cve/{name}/{tag}?skip=0
```

## Webhook Notifications

Anjungan can send webhook notifications when registry events occur (tag pushes, tag deletes, repo deletes).

### Managing Webhooks

1. Navigate to **Registry** > **Admin** tab > **Webhooks** section
2. Click **Add Webhook** — provide a name, target URL, and select event types
3. Toggle enable/disable per webhook

### Supported Event Types

| Event | Description |
|-------|-------------|
| `tag.push` | A new tag is pushed to the registry |
| `tag.delete` | A tag is deleted from the registry |
| `repo.delete` | An entire repository is deleted |
| `manifest.delete` | A manifest is deleted by digest |

### Webhook Payload

```json
{
  "event": "tag.delete",
  "repo": "nginx",
  "tag": "v1.0.0",
  "digest": "sha256:abc...",
  "actor": "admin@example.com",
  "timestamp": "2026-06-09T12:00:00Z"
}
```

Webhooks support Telegram, Discord, Slack, and any generic HTTP endpoint.

## Tag Protection

Tags can be locked to prevent accidental deletion — useful for production images.

### Via UI

1. Navigate to a repo detail page (`/registry/[name]`)
2. Click the shield-up icon next to a tag to protect it
3. Protected tags show a **lock icon** and **"Protected" badge**
4. Delete button is disabled/grayed out for protected tags
5. Click shield-minus icon to unprotect

### Bulk Operations

1. Select multiple tags using checkboxes
2. Sticky bottom bar appears with: **Protect All**, **Unprotect All**, **Delete All**
3. Protected tags are skipped during bulk delete

### Via API

```bash
# List protected tags
curl https://your-instance/api/v1/registry/protections

# Protect a tag
curl -X POST -H "Content-Type: application/json" \
  -d '{"repo": "nginx", "tag": "latest"}' \
  https://your-instance/api/v1/registry/protections

# Unprotect
curl -X DELETE https://your-instance/api/v1/registry/protections/{id}
```

## Cleanup Policies

Auto-delete old images based on configurable policies.

### Via API

```bash
# View current cleanup config
curl https://your-instance/api/v1/registry/cleanup/config

# Update cleanup config
curl -X PUT -H "Content-Type: application/json" \
  -d '{"max_age_days": 30, "min_tags": 5}' \
  https://your-instance/api/v1/registry/cleanup/config

# Manual trigger
curl -X POST https://your-instance/api/v1/registry/cleanup/run
```

### Via UI

1. Navigate to **Registry** > **Admin** tab > **Cleanup** section
2. View current policy (max age, min tags to keep)
3. Edit policy or click **Run Cleanup Now**

Cleanup runs on a background ticker (configurable interval) and can also be triggered manually.

## Admin Management

Admin users can manage registry users:

```
GET    /api/v1/registry/users              — List all users
POST   /api/v1/registry/users              — Create user
PUT    /api/v1/registry/users/{id}         — Update user (role, username, password)
DELETE /api/v1/registry/users/{id}         — Delete user
POST   /api/v1/registry/users/{id}/reset-password  — Reset any user's password
POST   /api/v1/registry/sync-htpasswd      — Force sync htpasswd + restart Zot
```

## How It Works

1. When a user fetches their credentials, Anjungan checks if a linked Zot user exists
2. If not, it creates one in the database with a generated password
3. The htpasswd file is regenerated from all registry users and Zot restarts
4. Zot reads the htpasswd file for authentication
5. Access control policies in `zot/config.json` enforce role-based permissions
6. CVE scanning uses Zot's built-in GraphQL search extension (`/v2/_zot/ext/search`)
7. Webhooks fire via background goroutines on tag/repo delete events
8. Cleanup policies run on a configurable background ticker
9. Tag protection is enforced at both backend (delete check) and frontend (UI disable)

> **Note:** Passwords are stored as bcrypt hashes. The plaintext password is shown **only once** on creation. Users should reset their password if they lose it.

## Role-Based Access Control

| Role | Read | Push | Delete Users | Trigger GC | Manage Webhooks | Manage Protections |
|------|------|------|-------------|------------|----------------|-------------------|
| `readonly` | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| `deploy` (default) | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| `admin` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

By default, all auto-created user accounts get the `deploy` role. Admin users can change roles via the admin interface.
