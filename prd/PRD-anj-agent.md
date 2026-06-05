# Anj-Agent — PRD

> **Versi:** 1.0
> **Status:** Draft
> **Author:** Endang Suwarna

---

## 1. Executive Summary

### Problem Statement

Anjungan saat ini connect ke server target via **SSH langsung** (`ssh.Dial("tcp", host:port)`). Ini masalah buat server yang:

- **Ga punya public IP** — di private network / VPC internal
- **SSH diprotek firewall** — cuma allow dari IP tertentu
- **NAT / dynamic IP** — hostnya berubah-ubah
- **Ephemeral** — container/VM yang naik-turun

SSH model ini **client-server klasik** dimana Anjungan jadi **client** yang initiate koneksi ke server. Masalahnya, banyak server production yang **ga bisa di-initiate dari luar**.

### Solution

**Anj-Agent** — lightweight agent yang dipasang di target server. Agentnya yang **nginisiasi koneksi outbound** ke Anjungan (reverse connection), jadi firewall friendly dan ga perlu public IP.

### Target Audience

- **DevOps / Platform Engineer** yang manage server di private network
- **Anjungan Admin** yang pengen unified server management tanpa mikirin SSH accessibility
- **Tim Infra** yang punya server tersebar di berbagai environment (on-prem, cloud, VPC)

### Goals

1. **Unified server management** — Admin bisa manage semua server lewat Anjungan, baik yg reachable via SSH maupun yg private
2. **Zero public SSH required** — Server private cukup jalanin agent, ga perlu buka port 22
3. **Backward compatible** — SSH method tetep ada, user bisa milih sesuai use case
4. **Minimal friction** — Setup agent cukup 1 command (curl pipe bash)

### Non-Goals

- Bukan replacement total buat SSH — SSH tetep jadi option utama buat server reachable
- Bukan remote desktop / VNC
- Bukan VPN atau tunnel replacement
- Ga nangani file sync atau backup

---

## 2. Product Overview

### Architecture

```
                         ┌──────────────────────┐
                         │    Anjungan Server     │
                         │                        │
                         │  ┌──────────────────┐  │
                         │  │   Backend API     │  │
                         │  │  (chi router)     │  │
                         │  └──────┬───────────┘  │
                         │         │               │
                         │  ┌──────▼───────────┐  │
                         │  │  Agent Gateway    │  │
                         │  │  (WebSocket)      │  │
                         │  └──────┬───────────┘  │
                         │         │               │
                         │  ┌──────▼───────────┐  │
                         │  │   Executor Abst.  │  │
                         │  │ SSH │ Agent       │  │
                         │  └──────────────────┘  │
                         └──────────┬────────────┘
                                    │
              ┌─────────────────────┼─────────────────────┐
              │                     │                     │
     ┌────────▼──────┐    ┌────────▼──────┐    ┌────────▼──────┐
     │  Server A      │    │  Server B      │    │  Server C      │
     │  (SSH)         │    │  (Agent)       │    │  (Agent)       │
     │                │    │                │    │                │
     │  SSH:22 open   │    │  No public IP  │    │  Behind NAT    │
     │  Reachable     │    │  Agent ───WS──►│    │  Agent ───WS──►│
     └────────────────┘    └────────────────┘    └────────────────┘
```

### Tech Stack

| Layer | Teknologi | Alasan |
|-------|-----------|--------|
| Agent | **Go** (same as backend) | Static binary, cross-compile, single binary deploy |
| Transport | **WebSocket** (gorilla/websocket) | HTTP upgrade, simple, ga perlu protobuf/proto compiler |
| Backend Gateway | **Go + gorilla/websocket** | Existing dep di go.mod, proven |
| Agent Protocol | **JSON over WS** | Human readable, gampang debug |
| Agent Deployment | **Binary / Docker / Docker Compose** | Flexibel sesuai environment |
| Auth | **HMAC-SHA256 + registration token** | Simple, no external dep |

### Data Model

#### Server (tambah field)

```go
type Server struct {
    // ... existing fields ...

    ConnectionType  string  `json:"connection_type"`  // "ssh" atau "agent"
    AgentID         string  `json:"agent_id,omitempty"`
    AgentVersion    string  `json:"agent_version,omitempty"`
    LastHeartbeat   *time.Time `json:"last_heartbeat,omitempty"`
}
```

#### Agent (table baru)

```go
type Agent struct {
    ID              string    `json:"id"`               // "ag_xxx"
    ServerID        string    `json:"server_id,omitempty"`
    Hostname        string    `json:"hostname"`
    OS              string    `json:"os"`
    Arch            string    `json:"arch"`
    AgentVersion    string    `json:"agent_version"`
    Capabilities    []string  `json:"capabilities"`     // ["exec","docker","metrics","logs","file","self-update"]
    Status          string    `json:"status"`            // "pending", "connected", "disconnected", "revoked"
    ConnectedAt     *time.Time `json:"connected_at,omitempty"`
    LastHeartbeat   *time.Time `json:"last_heartbeat,omitempty"`
    RegistrationToken string  `json:"-"`                 // one-time token, hashed in DB
    SecretKey       string    `json:"-"`                 // shared secret for HMAC
    RegisteredBy    string    `json:"registered_by"`
    CreatedAt       time.Time `json:"created_at"`
    UpdatedAt       time.Time `json:"updated_at"`
}
```

---

## 3. Feature Requirements

### F1 — Hybrid Connection Type

| | |
|---|---|
| **Priority** | **P0** |
| **Backend** | Server model tambah field `connection_type` ("ssh" / "agent"). Pas create server, admin milih connection type. Kalo "agent", field host/port/ssh_user disembunyiin, ganti pilih agent ID dari list registered agents |
| **Frontend** | Server create/edit form tambah radio/select "Connection Type". Conditional fields tergantung pilihan. Server card badge: 🔌 SSH / 📡 Agent |
| **UX** | Default: "SSH" (backward compat). Agent option harus ada agents available, kalo belum ada tampilin link ke halaman register agent |

### F2 — Agent Registration Flow

| | |
|---|---|
| **Priority** | **P0** |
| **Backend** | 1. Admin generate registration token via API → dapet `REG_TOKEN` (one-time, expired 1 jam) <br> 2. Agent connect via WebSocket ke `/ws/agent/register` pake `REG_TOKEN` → backend validasi, generate `agent_id` + `secret_key` <br> 3. Agent dapet `agent_id` + `secret_key` → disconnect, reconnect pake auth <br> 4. Agent reconnect ke `/ws/agent/{agent_id}` pake HMAC signature → status jadi "connected" |
| **Frontend** | Halaman "Register Agent" → form isi hostname, labels, notes → klik generate → muncul command buat di-copy paste (binary / docker / docker compose). List registered agents dengan status |
| **UX** | Registration command langsung muncul, tinggal copy paste di server target. Agent muncul di list dengan status "pending" sebelum connect, "connected" setelah connect |

### F3 — Agent Gateway (WebSocket Server)

| | |
|---|---|
| **Priority** | **P0** |
| **Backend** | 1. WebSocket endpoint `/ws/agent/{agent_id}` — handle persistent connection <br> 2. HMAC auth tiap pesan pake `secret_key` <br> 3. Message router: `exec`, `exec_result`, `heartbeat`, `ping/pong`, `exec_stream`, `exec_cancel`, `file_push/pull` <br> 4. Heartbeat timeout: 60 detik tanpa heartbeat → status "disconnected" <br> 5. Redis pub/sub buat routing dari HTTP handler ke agent connection |
| **UX** | Invisible to user — backend infrastructure |

### F4 — Executor Abstraction

| | |
|---|---|
| **Priority** | **P0** |
| **Backend** | Interface `Executor` dengan method `RunCommand`, `RunCommandWithStream`, `FileTransfer`, `Close`. Dua implementasi: `SSHExecutor` (existing) dan `AgentExecutor` (baru lewat WebSocket). Handler di infra package pake `getExecutor(ctx, srv)` — otomatis pilih SSH atau Agent berdasarkan `connection_type` |
| **Frontend** | No changes needed — semua endpoint (container, metrics, terminal) tetep sama |
| **UX** | Transparan — user ga perlu tau method koneksi mana yang dipake |

### F5 — Deployment Options

| | |
|---|---|
| **Priority** | **P0** |
| **Backend** | API endpoint `/servers/{id}/agent/commands` return command buat tiap deployment method |
| **Frontend** | Pas register/assign agent, tampilin 3 tab: "Binary", "Docker Run", "Docker Compose". Masing-masing isi command siap copy. Kalo pilih Docker / Docker Compose, form tambah opsi mount Docker socket |
| **UX** | 3 deployment option, user pilih sesuai kebutuhan. Command auto-generated sesuai data server |

#### Binary

```bash
curl -L https://releases.anjungan.io/anj-agent/v1.0.0/linux/amd64/anj-agent \
  -o /usr/local/bin/anj-agent
chmod +x /usr/local/bin/anj-agent

anj-agent register \
  --server https://anjungan.company.com \
  --token ag_reg_xxxx \
  --hostname server-prod-01
```

#### Docker Run

```bash
docker run -d \
  --name anj-agent \
  --restart unless-stopped \
  -v /var/run/docker.sock:/var/run/docker.sock:rw \
  -e ANJ_SERVER=https://anjungan.company.com \
  -e ANJ_TOKEN=ag_reg_xxxx \
  -e ANJ_HOSTNAME=server-prod-01 \
  ghcr.io/edsuwarna/anj-agent:latest
```

#### Docker Compose

```yaml
services:
  anj-agent:
    image: ghcr.io/edsuwarna/anj-agent:latest
    container_name: anj-agent
    restart: unless-stopped
    environment:
      ANJ_SERVER: https://anjungan.company.com
      ANJ_TOKEN: ag_reg_xxxx
      ANJ_HOSTNAME: server-prod-01
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:rw
      - /proc:/host/proc:ro
    logging:
      driver: json-file
      options:
        max-size: 10m
        max-file: 3
```

### F6 — Agent Management UI

| | |
|---|---|
| **Priority** | **P1** |
| **Frontend** | Halaman `/agents` — List semua agent, filter by status (connected/disconnected/pending/revoked), search by hostname. Tiap agent card: hostname, OS, IP seen by server, version, capabilities badges, last heartbeat, connection status. Action: Assign ke server, Revoke, View logs |
| **UX** | Expandable cards (sama kaya container page). Status indicator: 🟢 connected, 🟡 pending, 🔴 disconnected, ⚫ revoked. Assign flow: pilih server dari dropdown |

### F7 — Agent Capabilities Discovery

| | |
|---|---|
| **Priority** | **P1** |
| **Backend** | Pas register, agent ngirim `capabilities` array. Backend simpen di DB. Nanti pas server di-view, tampilin capabilities yg available |
| **Frontend** | Badges di agent card: 🖥 exec, 🐳 docker, 📊 metrics, 📋 logs, 📁 file, 🔄 self-update |
| **UX** | Capabilities yang ga available jadi grayed out |

### F8 — Heartbeat & Health Monitoring

| | |
|---|---|
| **Priority** | **P1** |
| **Backend** | Agent kirim heartbeat tiap 30 detik (CPU%, mem%, uptime, agent version). Backend update `last_heartbeat`, `agent_version`. Kalo 60 detik tanpa heartbeat → status "disconnected". Kalo reconnect → "connected". Bisa send alert via webhook? |
| **UX** | Admin liat status real-time di halaman agent & server |

### F9 — Self-Update

| | |
|---|---|
| **Priority** | **P2** |
| **Backend** | Admin trigger update dari UI → backend kirim `upgrade` message ke agent → agent download binary baru, restart |
| **Frontend** | Button "Update Agent" di agent detail. Available version dari backend config |
| **UX** | Progress: "Updating..." → "Restarting..." → "Connected (v1.1.0)" |

### F10 — File Transfer

| | |
|---|---|
| **Priority** | **P2** |
| **Backend** | Message type `file_push` / `file_pull`. Agent terima file stream → simpen ke path yg ditentukan, atau baca file → kirim ke Anjungan |
| **Frontend** | Upload/download file dari server detail page |
| **UX** | Drag & drop file untuk upload |

---

## 4. Agent Protocol (JSON over WebSocket)

### Message Envelope

```json
{
  "type": "exec",
  "id": "msg_001",
  "agent_id": "ag_abc123",
  "timestamp": 1712345678,
  "payload": { ... }
}
```

### Message Types

| Type | Direction | Payload | Deskripsi |
|------|-----------|---------|-----------|
| `register` | → | `{token, hostname, os, arch, version, capabilities}` | Registrasi awal |
| `register_ack` | ← | `{agent_id, secret_key, server_url}` | Konfirmasi + credential |
| `heartbeat` | → | `{cpu_pct, mem_pct, uptime, version}` | Keepalive tiap 30s |
| `exec` | ← | `{command, timeout, env}` | Jalankan command |
| `exec_result` | → | `{exit_code, stdout, stderr, duration_ms}` | Hasil command |
| `exec_stream` | → | `{id, stream:"stdout"/"stderr", data, seq}` | Live output |
| `exec_cancel` | ← | `{}` | Cancel running command |
| `file_push` | ← | `{path, data_base64, mode}` | Upload file |
| `file_pull` | ← | `{path}` | Download file |
| `file_result` | → | `{path, size, error}` | Hasil transfer |
| `upgrade` | ← | `{version, download_url, checksum}` | Trigger self-update |
| `upgrade_progress` | → | `{status, progress_pct, error}` | Progress update |
| `disconnect` | ← | `{reason}` | Graceful disconnect |

### Security

- **Registration**: One-time token (`ag_reg_xxx`), expired setelah dipake / 1 jam timeout
- **Transport**: WSS (WebSocket over TLS) — **wajib** di production
- **Message auth**: Setiap pesan pake HMAC-SHA256 signature di header WebSocket
- **Agent secret**: Disimpan di `AGENT_SECRET` env variable, ga boleh hardcoded
- **Revocation**: Admin revoke → backend kirim `disconnect` + hapus secret key
- **Docker socket**: Mount `/var/run/docker.sock` optional (cuma kalo butuh container management)

---

## 5. Non-Functional Requirements

| Aspect | Target |
|--------|--------|
| **Agent binary size** | < 10MB (stripped) |
| **Agent memory** | < 20MB idle, < 50MB peak |
| **Agent CPU** | < 0.5% idle, burst sampe 5% pas exec |
| **Reconnect** | Exponential backoff: 1s → 2s → 4s → ... → max 60s |
| **Heartbeat interval** | 30 detik |
| **Heartbeat timeout** | 60 detik tanpa heartbeat → disconnected |
| **Command timeout default** | 30 detik |
| **Max concurrent exec per agent** | 5 |
| **Transport latency** | < 100ms overhead (WS vs SSH) |
| **Update mechanism** | Rolling update, zero-downtime target |
| **Support lifecycle** | backward compat min 2 major version |

---

## 6. Milestone & Timeline

### Phase 1: Foundation (v1.0)

- [ ] **F4** — Executor interface & SSHExecutor refactor
- [ ] **F3** — WebSocket Gateway backend
- [ ] **F2** — Agent registration flow
- [ ] **F1** — Hybrid connection type (backend model + API)
- [ ] Agent binary skeleton (register + heartbeat + basic exec)

### Phase 2: Core Feature (v1.1)

- [ ] **F5** — All 3 deployment options (binary, Docker run, Docker Compose)
- [ ] **F6** — Agent management UI page
- [ ] Agent full capabilities: docker, shell exec
- [ ] Frontend: server form connection type selector

### Phase 3: Production Readiness (v1.2)

- [ ] **F7** — Capabilities discovery & display
- [ ] **F8** — Heartbeat & health monitoring
- [ ] Security hardening: WSS, HMAC, revocation

### Phase 4: Advanced (v2.0)

- [ ] **F9** — Self-update mechanism
- [ ] **F10** — File transfer
- [ ] Agent log streaming (real-time `docker logs` via WS)
- [ ] Agent metrics streaming (real-time CPU/mem via WS)

---

## 7. FAQs

**Q: Kenapa ga pake SSH reverse tunnel aja?**
A: SSH reverse perlu manage SSH keys, port forwarding, dan maintain koneksi. Agent lebih lightweight — Go binary kecil, auto-reconnect built-in, protocol terdefinisi jelas, ga perlu config SSH daemon. Plus agent bisa push command kapan aja.

**Q: Kenapa WebSocket bukan gRPC?**
A: WebSocket simpler — HTTP upgrade, ga perlu protobuf compiler, gampang lewat proxy/load balancer. gRPC lebih powerful (streaming, multiplexing) tapi complexity overhead lumayan. Bisa upgrade ke gRPC nanti kalo perlu.

**Q: Apa bedanya sama Prometheus Node Exporter?**
A: Node Exporter cuma ngirim metrics. Agent kita bisa **execute command**, **manage containers**, **stream logs**, **transfer files** — dua arah komunikasi. Agent bisa dibilang "SSH replacement", bukan cuma monitoring tool.

**Q: Aman ga ngasih akses Docker socket ke agent?**
A: Opsional. Agent bisa jalan tanpa Docker socket — fitur container management aja yg butuh. Di Docker deployment, user bisa milih mau mount socket atau enggak. Kalo ga di-mount, agent tetep bisa execute command dan system metrics.

**Q: Gimana kalo agent kena network partition?**
A: Exponential backoff reconnect. Agent queue operation kalo perlu (future). Backend deteksi disconnected via heartbeat timeout. Pas reconnect, agent kirim heartbeat langsung buat update status.

---

## 8. Related Documents

- [PRD.md](./PRD.md) — PRD utama Anjungan
- [ROADMAP.md](../ROADMAP.md) — Roadmap Anjungan
- [DECISIONS.md](../DECISIONS.md) — Architecture decisions
- [README.md](../README.md) — Technical docs
