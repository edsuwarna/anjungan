# Anjungan — PRD: Backup Manager

> **Version:** 1.0
> **Status:** Draft
> **Author:** Endang Suwarna
> **Last Updated:** June 2026

---

## 1. Executive Summary

### Problem Statement

- Backup tidak terpusat — harus SSH ke tiap server untuk cek status
- Tidak ada visibilitas kapan terakhir backup berjalan
- Tidak ada peringatan ketika backup gagal
- Recovery test tidak pernah dilakukan — backup bisa corrupt tanpa diketahui
- Tidak ada history run — siapa yang trigger, berapa size, berapa lama

### Target Audience

- **Endang** (platform engineer) — melihat semua backup tasks dalam satu dashboard, trigger on-demand backup, dan mendapat notifikasi jika gagal
- **DevOps / Platform Engineers** — mengelola konfigurasi backup untuk berbagai services (PostgreSQL, directory, Docker volume, Zot registry)
- **System Administrators** — memonitor disk usage per backup, menjalankan retention cleanup

### Goals

| Goal | Metric |
|------|--------|
| Lihat semua backup tasks + last run dalam satu dashboard | All targets shown in card grid |
| Trigger on-demand backup dari UI | 1-click "Run Now" per target |
| Lihat status real-time saat backup berjalan | Polling-based progress |
| Disk usage per backup | Size tracked per run + aggregated per target |
| Notifikasi ketika backup gagal | Integrated with Notification Engine |
| History timeline per backup target | Lihat semua run — status, size, duration, trigger |

### Non-Goals

- ❌ Bukan backup engine sendiri — tetap menggunakan pg_dump, tar, rclone, docker cp
- ❌ Bukan file storage sendiri — backup disimpan di local disk atau rclone remote
- ❌ Bukan backup scheduling engine baru — cron expression dari Go scheduler, bukan replacement infrastruktur existing
- ❌ Bukan backup-recovery wizard — tidak termasuk fitur restore dari UI (hanya backup creation)
- ❌ Bukan cross-server replication — backup disimpan per server, tidak otomatis di-transfer ke central location

---

## 2. Product Overview

### Tech Stack

| Layer | Technology | Reason |
|-------|-----------|--------|
| Backend | Go (existing) | Executor via SSH, cron scheduler, async execution |
| Frontend | SvelteKit (existing) | Backup target cards, history timeline, status polling |
| DB | PostgreSQL (existing) | `backup_targets`, `backup_runs` tables |
| Storage | Local disk / rclone remote | Menggunakan storage infra existing |
| Notification | Notification Engine (existing) | Failure notification integration |
| Execution | SSH | Leverage existing SSH infra untuk menjalankan backup command di target server |

### This Feature in the Context of Anjungan

```
                        ┌──────────────────────────────────────┐
                        │          Backup Manager              │
                        │                                      │
                        │  ┌─────────────────────────┐        │
                        │  │     backup_targets       │        │
                        │  │  - PostgreSQL (pg_dump)  │        │
                        │  │  - Directory (tar)       │        │
                        │  │  - Docker Volume (cp)    │        │
                        │  │  - Zot Registry (export) │        │
                        │  └──────────┬──────────────┘        │
                        │             │                         │
                        │  ┌──────────▼──────────────┐        │
                        │  │      backup_runs         │        │
                        │  │  - Status per run        │        │
                        │  │  - Size, duration        │        │
                        │  │  - Error tracking        │        │
                        │  └─────────────────────────┘        │
                        └──────────────────────────────────────┘
                                    │
                                    ▼
                        ┌──────────────────────┐
                        │  Notification Engine  │
                        │  (failure alerts)     │
                        └──────────────────────┘
                                    │
                                    ▼
                        ┌──────────────────────┐
                        │  SSH Executor         │
                        │  pg_dump / tar /      │
                        │  docker cp / rclone   │
                        └──────────────────────┘
```

Backup Manager berfungsi sebagai orchestrator dan dashboard untuk semua backup task. Ia tidak menggantikan tools backup yang sudah ada (pg_dump, tar, rclone), melainkan menyediakan UI terpusat dan scheduler untuk menjalankannya secara konsisten.

---

## 3. Feature Requirements

### 3.1 Feature Inventory

| Domain | Backend | Frontend | Status |
|--------|---------|---------|--------|
| Backup Targets CRUD | CRUD endpoints, per-type config validation | Card grid layout, create/edit modal | 🟡 Not started |
| Backup Execution (pg_dump, tar, docker cp, zot) | SSH-based executor, command builder per type | Status updates via polling | 🟡 Not started |
| Backup Status + History | run tracking, history endpoint | Timeline table, status badges | 🟡 Not started |
| On-Demand Trigger | POST /targets/{id}/run, async execution | "Run Now" button per card | 🟡 Not started |
| Retention Cleanup | Cron job, file deletion, disk usage tracking | Storage usage display per target | 🟡 Not started |
| Failure Notification | Integration with Notification Engine | Toast + notification log | 🟡 Not started |

### 3.2 Database Schema

```sql
-- backup target configuration
CREATE TABLE backup_targets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    type VARCHAR(30) NOT NULL, -- 'postgres', 'directory', 'docker-volume', 'zot'
    server_id UUID REFERENCES servers(id),
    config JSONB NOT NULL, -- {dbname, path, volume_name, registry_url, ...}
    storage_path TEXT NOT NULL, -- local path or rclone remote:path
    schedule VARCHAR(100), -- cron expression, NULL = manual only
    retention_days INT DEFAULT 30,
    is_active BOOLEAN DEFAULT true,
    notification_target_ids UUID[], -- link to notification engine targets
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

-- individual backup run history
CREATE TABLE backup_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    target_id UUID REFERENCES backup_targets(id),
    status VARCHAR(20) DEFAULT 'pending', -- pending, running, success, failed
    size_bytes BIGINT,
    duration_ms INT,
    path TEXT,
    error TEXT,
    triggered_by VARCHAR(30), -- 'cron', 'manual'
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ
);

-- indexes for performance
CREATE INDEX idx_backup_runs_target_id ON backup_runs(target_id);
CREATE INDEX idx_backup_runs_status ON backup_runs(status);
CREATE INDEX idx_backup_runs_started_at ON backup_runs(started_at);
CREATE INDEX idx_backup_targets_server_id ON backup_targets(server_id);
```

### 3.3 Feature Specs

#### F1 — Backup Targets CRUD (P0)

| Aspect | Detail |
|--------|--------|
| **Priority** | P0 — Foundation feature |
| **Backend** | CRUD endpoints: `GET /api/v1/backup-targets`, `POST`, `PUT /{id}`, `DELETE /{id}`. Validation: name required, type must be valid enum, config validated per type. Server ID must reference existing server. Storage path required. Schedule is optional cron expression (NULL = manual only). Retention days default 30. |
| **Frontend** | Card grid layout. Each card shows: icon by type, name, last backup status (● green / ✕ red), size, next run time. Click card → expand detail with tabs: Overview (config summary), History (timeline table), Settings (edit/delete). "+ Add Target" button top right. |
| **UX** | Type icons: 🗄️ postgres, 📁 directory, 🛢️ docker-volume, 📦 zot. Config fields dynamically swap based on type selection in create/edit modal. Empty state: illustration + "Add your first backup target" CTA. Active/inactive toggle visible on card. |

**Per-Type Config Fields:**

| Type | Config Fields |
|------|--------------|
| `postgres` | dbname, host, port (default 5432), username, password (encrypted), extra_flags (optional) |
| `directory` | source_path, exclude_patterns (optional), compress (boolean, default true) |
| `docker-volume` | volume_name, container_name (optional), compress (boolean, default true) |
| `zot` | registry_url, repository, username, password (encrypted), include_tags (optional) |

#### F2 — Backup Execution (P0)

| Aspect | Detail |
|--------|--------|
| **Priority** | P0 — Core functionality |
| **Backend** | SSH-based executor runs backup command on target server. Command templates per type. Creates `backup_runs` row with status `running`, updates on completion. |
| **Frontend** | Status updates via polling (every 5 seconds). Running state: spinner + "Backing up..." text. Completed: auto-refresh card. |
| **UX** | Run button disabled during execution. Progress indicator shown. Toast on completion: ✅ "Backup [name] completed — [size]" or ❌ "Backup [name] failed — [error]". Sound optional. |

**Command Templates:**

| Type | Command |
|------|---------|
| **PostgreSQL** | `pg_dump -h {host} -U {user} -d {dbname} -Fc -f {storage_path}/{filename}_{timestamp}.dump` |
| **Directory** | `tar -czf {storage_path}/{filename}_{timestamp}.tar.gz -C {source_dir} .` |
| **Docker Volume** | `docker run --rm -v {volume_name}:/source -v {storage_path}:/dest alpine tar -czf /dest/{filename}_{timestamp}.tar.gz -C /source .` |
| **Zot** | `oras copy --to-plain-http {registry_url}/{repository}:{tag} file://{storage_path}/{filename}_{timestamp}.tar` |

**Timeout:** 30 minutes per execution. If exceeded, process killed, status set to `failed` with error "Execution timeout".

#### F3 — Backup Status + History (P0)

| Aspect | Detail |
|--------|--------|
| **Priority** | P0 — Visibility |
| **Backend** | `GET /api/v1/backup-targets/{id}/history` — paginated list of backup_runs for target, ordered by started_at DESC. Each run: status, size_bytes, duration_ms, path, error, triggered_by, started_at, completed_at. |
| **Frontend** | Tab "History" in target detail. Timeline table: timestamp, status badge (🟢 success, 🔴 failed, 🟡 running, ⚪ pending), size, duration, trigger (cron/manual), download path (if applicable). Click row → expand detail (error message, full path). |
| **UX** | Status badges color-coded. Relative timestamps ("2 hours ago") with hover tooltip for exact time. Pagination or infinite scroll for long histories. Filter by status, date range. |

#### F4 — On-Demand Trigger (P1)

| Aspect | Detail |
|--------|--------|
| **Priority** | P1 — Valuable but not blocking MVP |
| **Backend** | `POST /api/v1/backup-targets/{id}/run`. Validates target exists and is_active. Creates backup_run row with status `pending` → immediately changes to `running`. Executes backup command asynchronously via Go goroutine. Updates run on completion (success/failed, size, duration, path, error). Returns `{ run_id }` immediately. |
| **Frontend** | "Run Now" button on each card + in target detail. On click → button becomes disabled with spinner. Polls `GET /api/v1/backup-targets/{id}/history?limit=1` every 5 seconds until status is `success` or `failed`. Card updates: last backup status badge refreshes, size shown. |
| **UX** | Cannot run if already running. Button text: "Run Now" → "Running..." (spinner) → "Run Again". Cooldown: 10 seconds minimum between manual runs (prevent spam). Confirmation dialog if last run was < 5 minutes ago. |

#### F5 — Retention Cleanup (P1)

| Aspect | Detail |
|--------|--------|
| **Priority** | P1 — Important for storage management |
| **Backend** | Cron job runs daily. For each backup_target, queries backup_runs completed before `now() - retention_days`. Deletes physical files from disk. Deletes/archives corresponding backup_runs rows. Logs cleanup: target_id, files_deleted_count, space_freed_bytes. |
| **Frontend** | Tab "Storage" in target detail: shows total size of all runs, size trend, retention days. Storage overview card: total used, available, usage percentage. Warning banner if usage > 80%. |
| **UX** | Storage section shows: "30 GB used across 12 runs (30 day retention)". Cleanup job status: "Last cleanup: 2 hours ago — 3 files deleted, 2.1 GB freed". Color-coded usage bar (green < 60%, yellow < 80%, red ≥ 80%). |

#### F6 — Failure Notification (P1)

| Aspect | Detail |
|--------|--------|
| **Priority** | P1 — Important for reliability |
| **Backend** | On backup run failure: check `backup_target.notification_target_ids` for configured notification targets. Send notification via Notification Engine with: target name, error message, last success time (if any), run ID. Log to notification_logs. |
| **Frontend** | Notification target selector in target settings (multi-select, filtered by scope 'backup'). Toast shown on failure if user is viewing the page. |
| **UX** | Notification message format: "❌ Backup failed — {target_name}\nError: {error}\nLast success: {last_success_time}\nServer: {server_name}\nRun ID: {run_id}". Link to run detail in Anjungan (if accessible). |

---

## 4. API Design

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | /api/v1/backup-targets | List all backup targets | ✅ |
| POST | /api/v1/backup-targets | Create backup target | ✅ |
| PUT | /api/v1/backup-targets/{id} | Update backup target | ✅ |
| DELETE | /api/v1/backup-targets/{id} | Delete backup target | ✅ |
| POST | /api/v1/backup-targets/{id}/run | Trigger on-demand backup | ✅ |
| GET | /api/v1/backup-targets/{id}/history | List backup run history | ✅ |
| GET | /api/v1/backup-stats | Aggregated backup stats | ✅ |

### Request / Response Examples

**POST /api/v1/backup-targets**
```json
{
  "name": "anjungan-postgres",
  "type": "postgres",
  "server_id": "550e8400-e29b-41d4-a716-446655440000",
  "config": {
    "dbname": "anjungan",
    "host": "localhost",
    "port": 5432,
    "username": "postgres",
    "password": "encrypted_value_here"
  },
  "storage_path": "/var/backups/postgres",
  "schedule": "0 2 * * *",
  "retention_days": 30,
  "notification_target_ids": ["uuid-target-1", "uuid-target-2"]
}
```

**POST /api/v1/backup-targets/{id}/run**
```json
{
  "run_id": "660e8400-e29b-41d4-a716-446655440001",
  "status": "running"
}
```

**GET /api/v1/backup-targets/{id}/history**
```json
{
  "runs": [
    {
      "id": "660e8400-e29b-41d4-a716-446655440001",
      "status": "success",
      "size_bytes": 2147483648,
      "duration_ms": 45000,
      "path": "/var/backups/postgres/anjungan_20260611_020000.dump",
      "error": null,
      "triggered_by": "cron",
      "started_at": "2026-06-11T02:00:00Z",
      "completed_at": "2026-06-11T02:00:45Z"
    }
  ],
  "total": 45,
  "page": 1,
  "per_page": 20
}
```

**GET /api/v1/backup-stats**
```json
{
  "total_targets": 12,
  "active_targets": 10,
  "total_runs": 450,
  "successful_runs": 432,
  "failed_runs": 18,
  "total_size_bytes": 107374182400,
  "last_run_at": "2026-06-11T02:00:45Z",
  "targets_over_80pct": 2
}
```

---

## 5. UI/UX Design Guidelines

### Key Layout

```
┌──────────────────────────────────────────────────────────────────┐
│  Anjungan  ●  Backup Manager                        [+ Add Target] │
│  Dashboard                                                     │
│  Servers                                                       │
│  Projects                                                      │
│  Monitoring                                                    │
│  ├─ Uptime                                                     │
│  ├─ SSL                                                        │
│  └─ Notifications                                              │
│  Ops                                                           │
│  └─ Backup Manager     ○ ←                                     │
│                                                                │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ 🗄️ anjungan-postgres              📁 app-configs       │  │
│  │ ● Last: 2 hours ago (2.1 GB)      ● Last: 6 hours ago  │  │
│  │ Next: today 02:00                  Next: today 04:00    │  │
│  │ [Run Now] [▸ View]               [Run Now] [▸ View]   │  │
│  └──────────────────────────────────────────────────────────┘  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ 🛢️ prometheus-data                 📦 ghcr-images       │  │
│  │ ✕ Last: 1 day ago (FAILED)         ● Last: 1 hour ago   │  │
│  │ Next: today 03:00                  Next: tomorrow 01:00 │  │
│  │ [Run Now] [▸ View]               [Run Now] [▸ View]   │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                │
│  ─── Storage Overview ──────────────────────────────────────   │
│  [████████████████░░░░░░░░] 85.2 GB / 500 GB used (17%)       │
│  ⚠️ 2 targets over 80% storage threshold                      │
│                                                                │
└──────────────────────────────────────────────────────────────────┘
```

### Target Detail View (Expanded)

```
┌──────────────────────────────────────────────────────────────────┐
│  ← Backup Targets              🗄️ anjungan-postgres             │
│                                  ● Active                [Run Now] │
│                                                                │
│  ┌── Tab: [Overview] [History] [Settings] ──────────────────┐  │
│  │                                                          │  │
│  │  Overview:                                               │  │
│  │  ┌──────────────────────────────────────────────────┐    │  │
│  │  │ Type:        PostgreSQL                         │    │  │
│  │  │ Server:      anju-db-01 (192.168.1.10)          │    │  │
│  │  │ Database:    anjungan                           │    │  │
│  │  │ Storage:     /var/backups/postgres              │    │  │
│  │  │ Schedule:    0 2 * * * (daily 02:00)            │    │  │
│  │  │ Retention:   30 days                            │    │  │
│  │  │ Notify:      DevOps Telegram, Admin Email       │    │  │
│  │  └──────────────────────────────────────────────────┘    │  │
│  │                                                          │  │
│  │  ┌── History ──────────────────────────────────────┐    │  │
│  │  │ Timestamp       │ Status  │ Size   │ Duration │    │  │
│  │  │─────────────────│─────────│────────│──────────│    │  │
│  │  │ 2 hours ago     │ 🟢 succ │ 2.1 GB │ 45s      │    │  │
│  │  │ Yesterday 02:00 │ 🟢 succ │ 2.0 GB │ 42s      │    │  │
│  │  │ 2 days ago      │ 🔴 fail │ —      │ 30s      │    │  │
│  │  │                 │         │        │ conn ref │    │  │
│  │  └────────────────────────────────────────────────┘    │  │
│  │                                                          │  │
│  └──────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────┘
```

### Creating/Editing Modal

```
┌──────────────────────────────────────────────────────┐
│  Add Backup Target                                   │
│                                                      │
│  Name: [______________________________]              │
│                                                      │
│  Type:         [PostgreSQL ▾]                        │
│                                                      │
│  Server:       [anju-db-01 ▾]                        │
│                                                      │
│  ┌── PostgreSQL Config ─────────────────────────┐   │
│  │ Database:    [____________________________]  │   │
│  │ Host:        [____________________________]  │   │
│  │ Port:        [5432]                          │   │
│  │ Username:    [____________________________]  │   │
│  │ Password:    [____________________________]  │   │
│  └──────────────────────────────────────────────┘   │
│                                                      │
│  Storage Path: [/var/backups/postgres___________]    │
│                                                      │
│  Schedule (cron): [0 2 * * *_______________]        │
│                                                      │
│  Retention (days): [30]                              │
│                                                      │
│  Notification Targets:                               │
│  ☑ DevOps Telegram                                   │
│  ☐ Admin Email                                       │
│  ☐ Slack Channel                                     │
│                                                      │
│  ☑ Active                                            │
│                                                      │
│                [Cancel]  [Save Target]                │
└──────────────────────────────────────────────────────┘
```

### Key UX Principles

1. **Card grid layout** — setiap backup target adalah card dengan visual identity: icon per type, status color, size info.
2. **Status at a glance** — ● hijau (success), ✕ merah (failed), 🟡 berputar (running), ⚪ abu (pending).
3. **Type icons** — 🗄️ postgres, 📁 directory, 🛢️ docker-volume, 📦 zot — recognizable at a glance.
4. **Inline actions** — "Run Now" button langsung di card, tidak perlu masuk detail.
5. **Real-time updates** — polling 5 detik saat backup running, card refresh otomatis.
6. **Expandable detail** — klik card → expand untuk history timeline dan settings, tanpa page navigation.
7. **Storage visibility** — overview bar di bottom page, per-target di tab Settings.
8. **Empty states** — illustration + CTA ketika belum ada backup targets.
9. **Failure prominence** — card failure tampil beda (red border/background), notification otomatis.

---

## 6. Non-Functional Requirements

| Aspect | Target | Notes |
|--------|--------|-------|
| Backup timeout | 30 minutes per target | Process killed if exceeded, marked as failed |
| Storage limit warning | Warn at 80% disk usage | Per-server, shown in storage overview |
| Retention cleanup | Daily cron job | Deletes files older than `retention_days` |
| Backup file security | Readable by root only | `chmod 600`, `chown root:root` |
| Concurrent executions | Max 3 per server | Prevent resource exhaustion on target server |
| Polling interval | 5 seconds | For status updates during running backup |
| Execution log | All commands logged | stdout, stderr captured for debugging |
| Rate limit (manual run) | Minimum 10s between runs | Prevent accidental spam-clicking |
| DB connection (pg_dump) | Via pgpass file | Password not exposed in process list |
| Notification delivery | < 10s after failure | Async via Notification Engine |
| API pagination default | 20 items per page | For history endpoints |

---

## 7. Implementation Roadmap

### Phase 1: Foundation (4 days)

| Order | Feature | Effort | Dependency |
|-------|---------|--------|------------|
| 1 | `backup_targets` + `backup_runs` tables, migration | 1 day | — |
| 2 | Backup executor: SSH command builder per type (pg_dump, tar, docker cp, zot) | 2 days | #1 |
| 3 | CRUD endpoints (`GET/POST/PUT/DELETE /api/v1/backup-targets`) | 0.5 day | #2 |
| 4 | Frontend: card grid layout, create/edit modal, type-based config form | 1 day | #3 |
| 5 | Frontend: target detail view (Overview tab) | 0.5 day | #4 |

### Phase 2: Execution & History (3 days)

| Order | Feature | Effort | Dependency |
|-------|---------|--------|------------|
| 1 | `POST /api/v1/backup-targets/{id}/run` — async execution | 1 day | Phase 1 |
| 2 | Frontend: "Run Now" button + status polling | 1 day | #1 |
| 3 | `GET /api/v1/backup-targets/{id}/history` — paginated endpoint | 0.5 day | #1 |
| 4 | Frontend: History tab — timeline table with filters | 1 day | #3 |
| 5 | `GET /api/v1/backup-stats` — aggregated stats endpoint | 0.5 day | #3 |

### Phase 3: Scheduler & Reliability (3 days)

| Order | Feature | Effort | Dependency |
|-------|---------|--------|------------|
| 1 | Cron scheduler — auto-run backup based on schedule field | 1 day | Phase 2 |
| 2 | Retention cleanup — daily cron job, file deletion, space tracking | 1 day | Phase 2 |
| 3 | Storage overview — frontend component, usage bar, warning at 80% | 0.5 day | #2 |
| 4 | Failure notification — Notification Engine integration | 1 day | #1 |
| 5 | Notification target selector in backup target settings | 0.5 day | #4 |

---

## 8. Design Decisions

### 8.1 SSH-Based Execution, Not Agent-Based

**Why:** Anjungan sudah memiliki SSH infrastructure yang teruji untuk menjalankan command di server remote. Menambahkan agent di setiap server akan meningkatkan kompleksitas deployment dan maintenance.

**Pattern:** Setiap backup execution membuka SSH session ke target server, menjalankan command (pg_dump, tar, docker cp), dan menunggu completion. Output (stdout/stderr) ditangkap untuk logging.

**Trade-off:** SSH key management jadi critical — keys harus terdistribusi dengan aman. Jika SSH host key berubah, execution gagal. Mitigasi: SSH config dengan `StrictHostKeyChecking=accept-new` untuk first-connection.

### 8.2 Backup Stored Locally Per Server, Not Central Backup Server

**Why:** Backup besar (PostgreSQL dump, Docker volume) memakan waktu dan bandwidth jika harus di-transfer ke server pusat. Menyimpan secara lokal mengurangi bottleneck jaringan.

**Pattern:** Setiap target menentukan `storage_path` yang merupakan path lokal di target server. Opsional: rclone remote:path untuk sync ke storage eksternal (S3, Google Drive, etc.).

**Trade-off:** Jika server mati total, backup ikut hilang. Mitigasi: rclone sync sebagai post-execution hook opsional. Retention cleanup tetap berjalan di server lokal untuk menghindari disk full.

### 8.3 pg_dump -Fc Format for PostgreSQL

**Why:** Format custom (-Fc) memberikan kompresi built-in, ukuran lebih kecil, dan parallel restore support. Format ini portable dan bisa direstore di environment lain.

**Pattern:** `pg_dump -h {host} -U {user} -d {dbname} -Fc -f {path}/{name}_{timestamp}.dump`. File `.dump` adalah compressed archive, bukan plain SQL.

**Trade-off:** Hanya bisa direstore dengan `pg_restore`, bukan `psql` langsung. Untuk backup kecil/konfigurasi ringan, plain SQL mungkin lebih simple. Tapi untuk production database, -Fc adalah standard best practice.

### 8.4 Go Scheduler for Cron, Not External Cron

**Why:** Semua state (targets, schedule, last run) ada di database. Go scheduler bisa membaca schedule dari DB, execute tepat waktu, dan update status langsung. External cron membutuhkan bash script yang membaca DB atau environment variable — fragile.

**Pattern:** In-process scheduler yang membaca semua `backup_targets` dengan schedule IS NOT NULL dan `is_active = true`. Setiap menit cek apakah ada target yang scheduled. Execution via goroutine pool (max 3 concurrent per server).

**Trade-off:** Scheduler mati jika aplikasi restart (tapi karena Anjungan service-based, restart jarang). Mitigasi: missed schedule detection — jika scheduler down > 5 menit, jalankan missed jobs saat startup.

### 8.5 Notification Integration via `notification_target_ids`

**Why:** Menggunakan Notification Engine yang sudah ada di Anjungan, bukan membuat sistem notifikasi sendiri. Backup failure notifications menggunakan target yang sudah dikonfigurasi user.

**Pattern:** `backup_targets.notification_target_ids` adalah array UUID yang refer ke `notification_targets.id`. Pada failure event, iterate semua target ID, dispatch via Notification Engine dengan scope `backup`.

**Trade-off:** Dependensi pada Notification Engine — jika engine down, failure notification tidak terkirim. Tapi backup execution tetap berjalan (hanya notifikasi yang skip). Ini acceptable karena notification engine juga bukan critical path.

---

## 9. Glossary

| Term | Definition |
|------|-----------|
| Backup Target | Konfigurasi backup untuk satu sumber data (database, direktori, volume, registry) |
| Backup Run | Satu eksekusi backup — dari trigger hingga selesai (success/failed) |
| Retention | Jumlah hari backup disimpan sebelum dihapus oleh cleanup job |
| pg_dump | PostgreSQL native backup tool (custom format = -Fc) |
| Rclone | CLI tool untuk sync file ke berbagai cloud storage providers |
| Storage Path | Lokasi penyimpanan file backup (local disk atau remote rclone) |
| Notification Target | Destination notifikasi yang dikonfigurasi di Notification Engine (Telegram, Email, Webhook) |
| SSH Executor | Komponen yang menjalankan command backup di remote server via SSH |

---

## 10. Related Documents

- [PRD-notification-engine.md](./PRD-notification-engine.md) — Notification Engine untuk failure alerts backup
- [PRD-database-manager.md](./PRD-database-manager.md) — Database management feature (mungkin shared schema/infra)
- [PRD-traefik-dashboard.md](./PRD-traefik-dashboard.md) — Traefik dashboard (opsional: backup config traefik)
- [Database Schema](./../backend/internal/common/db/repository.go) — Existing repository pattern untuk referensi implementasi
