---
title: Self-Server Registration
description: Auto-register the host server running Anjungan for metrics, containers, and compliance.
---

# Self-Server (Host Auto-Registration)

Anjungan can automatically detect and register the **host server** where it's deployed — no manual server entry needed. This gives you visibility into the Docker host's containers, metrics, and compliance directly from the dashboard.

## How It Works

On startup, the backend runs a self-detection routine:

1. **Check** `SELF_SERVER_ENABLED=true` — skips if false/unset
2. **Access** the Docker socket (`/var/run/docker.sock`)
3. **Detect** host info — hostname, OS, kernel, CPU model, core count
4. **Find or Create** a server record in the database (matched by hostname)
5. **Register** with `connection_type: docker-socket` and `is_self: true`

The registered server appears as **"anjungan-host"** by default, visible in:
- **Servers list** — shows CPU/OS info
- **Container page** — lists all containers on the host
- **Dashboard** — host metrics (CPU, memory, disk)
- **Compliance scanning** — CIS Docker and Lynis scans run on the host

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `SELF_SERVER_ENABLED` | `false` | Set to `"true"` to enable auto-registration |
| `SELF_SERVER_NAME` | `"anjungan-host"` | Display name for the self-server |
| `DOCKER_SOCKET_PATH` | `/var/run/docker.sock` | Path to the Docker socket |
| `SELF_HOST_NETWORK` | — | Host IP from inside the container |

## Requirements

### Docker Socket Access

The backend container needs the Docker socket mounted and Docker CLI installed:

```yaml
backend:
  environment:
    SELF_SERVER_ENABLED: "true"
  volumes:
    - /var/run/docker.sock:/var/run/docker.sock:rw
  group_add:
    - "988"  # Docker group GID — run `getent group docker` to verify
```

> **Note:** The Docker group GID (`988` in the example) varies by OS. Check with:
> ```bash
> getent group docker | cut -d: -f3
> ```

## Connection Type: `docker-socket`

The self-server uses `connection_type: docker-socket` instead of SSH:
- Container management → via **Docker socket** directly
- Host metrics → via **`docker run --pid=host` nsenter**
- SSH Terminal → via **nsenter** by default

### Adding SSH Access (Optional)

If you want to SSH into the self-server:
1. Generate an SSH key pair on the host
2. Add the public key to `~/.ssh/authorized_keys`
3. Store the private key in Anjungan via **Settings → SSH Keys**
4. Assign the key to the self-server in **Server Settings**

## Troubleshooting

### Self-server not appearing

Check backend logs:

```bash
docker compose logs backend | grep "\[self\]"
```

Expected output:
```
[self] Docker host detected: my-server
[self] self-server updated: anjungan-host (abc-123...)
```

If you see `[self] Docker socket not accessible`:
- Verify `SELF_SERVER_ENABLED: "true"` is set
- Verify the Docker socket is mounted
- Verify the container user is in the docker group

### Container actions fail

Check that:
- The socket is mounted with `:rw` (read-write)
- The container has docker group access
- The `docker` CLI is installed in the backend container
