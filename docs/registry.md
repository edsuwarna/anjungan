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
curl -H "Authorization: Bearer <token>" \
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
curl -X POST -H "Authorization: Bearer <token>" \
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

## Role-Based Access Control

| Role | Read | Push | Delete Users | Trigger GC |
|------|------|------|-------------|------------|
| `readonly` | ✅ | ❌ | ❌ | ❌ |
| `deploy` (default) | ✅ | ✅ | ❌ | ❌ |
| `admin` | ✅ | ✅ | ✅ | ✅ |

By default, all auto-created user accounts get the `deploy` role. Admin users can change roles via the admin interface.

## Browsing Images

The Registry page in the dashboard provides:

- **Repository list** — all repos with tag counts
- **Tag details** — size, digest, OS/arch, layer history
- **Image inspection** — config, environment variables, exposed ports, entrypoint
- **Multi-arch support** — platform-specific manifests

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

> **Note:** Passwords are stored as bcrypt hashes. The plaintext password is shown **only once** on creation. Users should reset their password if they lose it.
