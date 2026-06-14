# Self-Server (Host Auto-Registration)

Anjungan can automatically detect and register the **host server** where it's deployed — no manual server entry needed. This gives you visibility into the Docker host's containers, metrics, and compliance directly from the Anjungan dashboard.

## How It Works

On startup, the backend runs a self-detection routine:

1. **Check** `SELF_SERVER_ENABLED=true` — skips if false/unset
2. **Access** the Docker socket (`/var/run/docker.sock`)
3. **Detect** host info — hostname, OS, kernel, CPU model, core count
4. **Find or Create** a server record in the database (matched by hostname)
5. **Register** with `connection_type: docker-socket` and `is_self: true`

The registered server appears as **"anjungan-host"** by default, visible in:

- **Servers list** — shows CPU/OS info
- **Container page** — lists all containers on the host (including Anjungan's own services)
- **Dashboard** — host metrics (CPU, memory, disk)
- **SSH Terminal** — if SSH credentials are configured (falls back to Docker nsenter otherwise)
- **Compliance scanning** — CIS Docker and Lynis scans run on the host

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `SELF_SERVER_ENABLED` | `false` | Set to `"true"` to enable auto-registration |
| `SELF_SERVER_NAME` | `"anjungan-host"` | Display name for the self-server |
| `DOCKER_SOCKET_PATH` | `/var/run/docker.sock` | Path to the Docker socket |
| `SELF_HOST_NETWORK` | — | Host IP from inside the container (e.g. `172.22.0.1` for Docker Compose networks, or `host.docker.internal`) |

## Requirements

### Docker Socket Access

The backend container needs the Docker socket mounted and the Docker CLI installed:

```yaml
backend:
  environment:
    SELF_SERVER_ENABLED: "true"
  volumes:
    - /var/run/docker.sock:/var/run/docker.sock:rw
  group_add:
    - "988"  # Docker group GID on the host — run `getent group docker` to verify
```

> **Note:** The Docker group GID (`988` in the example) varies by OS. Check with:
> ```bash
> getent group docker | cut -d: -f3
> ```

### Docker CLI

The Docker CLI must be available inside the backend container. The multi-stage Dockerfile includes it by default. If using a custom image, ensure `docker` is installed.

## Connection Type: `docker-socket`

The self-server uses `connection_type: docker-socket` instead of SSH. This means:

- Container listing and management → via **Docker socket** directly
- Host metrics → via **docker run --pid=host --privileged alpine nsenter**
- SSH Terminal → via **Docker nsenter** by default, or **SSH** if SSH credentials are configured

### Adding SSH Access (Optional)

If you want to SSH into the self-server (e.g. for file editing, package management), generate an SSH key pair and add it to the server:

1. **Generate a key pair** on the host (outside the container)
2. **Add the public key** to `~/.ssh/authorized_keys` on the host
3. **Store the private key** in Anjungan via **Settings → SSH Keys**
4. **Assign the key** to the self-server in **Server Settings** with SSH user `root`

Once configured, the SSH Terminal will use SSH instead of Docker nsenter.

## Deployment

### Fresh Install

When deploying Anjungan on a new server:

1. Include the self-server config in your `docker-compose.yml` (see [deployment.md](deployment.md))
2. Set `SELF_SERVER_ENABLED: "true"` in the backend environment
3. Mount the Docker socket
4. Start the stack

The self-server registers automatically on first startup.

### Re-deployment / DB Restore

- **Same host, same hostname** → updates the existing record
- **Different host** → creates a new record (matched by hostname)
- **Same DB, different hostname** → creates a new record alongside the old one (old one stays as offline)

## Troubleshooting

### Self-server not appearing

Check the backend logs:

```bash
docker compose logs backend | grep "\[self\]"
```

Expected output:
```
[self] Docker host detected: my-server
[self] self-server updated: anjungan-host (abc-123...)
```

If you see `[self] Docker socket not accessible` or `[self] self-server detection disabled`:

- Verify `SELF_SERVER_ENABLED: "true"` is set
- Verify the Docker socket is mounted (`/var/run/docker.sock`)
- Verify the container user is in the docker group (check `group_add` GID)

### Self-server shows 0 containers

This was a known issue where shell-quoted `--format` arguments were improperly parsed. Fixed in `container/handler.go` with a shell-aware argument splitter (`shellSplit`). Ensure you're running the latest build.

### Container actions fail (start/stop/restart)

Docker socket operations run directly via the socket. Check that:
- The socket is mounted with `:rw` (read-write)
- The container has docker group access
- The `docker` CLI is installed in the backend container
