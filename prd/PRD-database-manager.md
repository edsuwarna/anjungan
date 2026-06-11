# Anjungan — PRD: Database Manager

> **Version:** 1.0
> **Status:** 🟡 Not Started
> **Author:** Endang Suwarna
> **Last Updated:** June 11, 2026

---

## 1. Executive Summary

### Problem Statement

Engineers frequently SSH into production servers just to run a `psql` query or check migration status. There is no centralized view of all PostgreSQL instances, their schemas, migration states, or backup health. This creates friction:

- **SSH-driven DB access** — every quick `SELECT` requires a separate SSH session
- **No migration visibility** — applied vs pending migrations invisible without checking each server manually
- **Backup orphaned** — no central record of backup timestamps, sizes, or failure status
- **Schema exploration tedious** — discovering tables, columns, and constraints requires `\dt` + `\d+` sequences
- **No audit trail** — no record of who ran what queries against which database

### What This Solves

| Problem | Solution |
|---------|----------|
| SSH into server just to run `psql` | **Read-only query executor** from the Anjungan UI |
| Migration status invisible | **Migration viewer** — green (applied) / yellow (pending) at a glance |
| No backup governance | **Backup dashboard** — last timestamp, size, status, plus "Backup Now" |
| Schema exploration slow | **Schema browser** — collapsible tree: DB → Schemas → Tables → Columns |
| Queries not audited | **Query history** — last 50 queries per connection, never lost |

### Target Audience

- **Endang** (platform/backend engineer) — primary user, manages all PostgreSQL instances
- **Infra engineers** (team members) — view migration status, run read-only queries
- **Developers** (read-only access) — explore schema, verify data without SSH access to production

### Goals

| Goal | Metric |
|------|--------|
| Lihat daftar database per server | All PostgreSQL instances discovered and displayed |
| Execute SQL read-only dari UI | Query returns results in < 10s |
| Migration status viewer | Applied/pending visible per database |
| Backup schedule + last status | Per-database backup record with timestamp and size |

### Non-Goals

- ❌ Not a schema editor or user management tool
- ❌ Not a query analyzer (no `EXPLAIN ANALYZE` viewer, no query plan visualization)
- ❌ Not a pgAdmin replacement — no visual query builder, no server-level config editing
- ❌ Not a migration runner — migrations are still executed externally (via CI/CD or Alembic/Flyway)
- ❌ Not a cross-DB query engine — only PostgreSQL target databases

---

## 2. Product Overview

### Architecture

```
┌──────────────────────────────────────────────────────────┐
│                   Anjungan Platform                        │
│                                                            │
│  ┌──────────────────────────────────────────────────────┐  │
│  │                  SvelteKit Frontend                    │  │
│  │  ┌─────────────┐  ┌──────────┐  ┌───────────────┐    │  │
│  │  │ Query       │  │ Schema   │  │ Migration     │    │  │
│  │  │ Editor      │  │ Browser  │  │ Viewer        │    │  │
│  │  └──────┬──────┘  └─────┬────┘  └───────┬───────┘    │  │
│  │         │                │               │             │  │
│  │  ┌──────┴────────────────┴───────────────┴────────┐   │  │
│  │  │           API Client (REST)                     │   │  │
│  │  └─────────────────────┬─────────────────────────┘   │  │
│  └────────────────────────┼─────────────────────────────┘  │
│                           │                                 │
│  ┌────────────────────────┼─────────────────────────────┐  │
│  │           Go Backend (Chi + pgx)                     │  │
│  │                                                       │  │
│  │  ┌──────────────────────────────────────────────────┐ │  │
│  │  │  DB Connection Manager (CRUD + Encryption)       │ │  │
│  │  │  ┌─────────┐ ┌──────────┐ ┌──────────────────┐  │ │  │
│  │  │  │ Config  │ │Password  │ │ Query History    │  │ │  │
│  │  │  │ Store   │ │Crypto    │ │ (local storage)   │  │ │  │
│  │  │  └─────────┘ └──────────┘ └──────────────────┘  │ │  │
│  │  └──────────────────────────────────────────────────┘ │  │
│  │                                                       │  │
│  │  ┌──────────────────────────────────────────────────┐ │  │
│  │  │  SSH Executor Layer                               │ │  │
│  │  │  ┌───────────┐ ┌──────────┐ ┌───────────────┐   │ │  │
│  │  │  │ psql      │ │ pg_isready│ │ pg_dump       │   │ │  │
│  │  │  │ Wrapper   │ │ Wrapper  │ │ Wrapper       │   │ │  │
│  │  │  └───────────┘ └──────────┘ └───────────────┘   │ │  │
│  │  └──────────────────────────────────────────────────┘ │  │
│  │                                                       │  │
│  │  ┌──────────────────────────────────────────────────┐ │  │
│  │  │  Infrastructure: servers table (existing SSH)    │ │  │
│  │  └──────────────────────────────────────────────────┘ │  │
│  └────────────────────────┬─────────────────────────────┘  │
└───────────────────────────┼─────────────────────────────────┘
                            │
                 ┌──────────┴──────────┐
                 │   Target Servers     │
                 │  (SSH + psql)        │
                 │  ┌────────────────┐  │
                 │  │ PostgreSQL     │  │
                 │  │ Instances      │  │
                 │  └────────────────┘  │
                 └─────────────────────┘
```

### Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| **Read-only enforced at DB level** | `SET TRANSACTION READ ONLY` is executed before every query — safety guarantee even if UI has a bug |
| **SSH-based execution** | Leverages existing server credentials stored in Anjungan — no separate DB network access needed |
| **Query history stored locally** | Stored in Anjungan's own PostgreSQL (`db_connections`-related history table), not in the target database |
| **Password encryption at rest** | AES-256-GCM for `password_encrypted` column — never stored in plaintext |
| **Timeout at Go layer** | 10s hard limit on `context.WithTimeout` — query is cancelled even if SSH/psql hangs |

---

## 3. Feature Specifications

### F1 — Database Discovery (P0)

| Aspect | Detail |
|--------|--------|
| **Priority** | P0 — Foundation |
| **Status** | 🟡 Not started |
| **Description** | Auto-detect PostgreSQL instances on known servers, with manual add as fallback |
| **Auto-discovery** | SSH into each server and run `psql --version` + `pg_isready`. Parse output to detect running PostgreSQL + listening ports |
| **Manual add** | Form: server dropdown (from existing `servers` table), host, port, dbname, username, password, SSL mode |
| **Connection test** | "Test Connection" button that SSH-executes `psql -h host -p port -U user -d dbname -c 'SELECT 1'` |
| **Connection status** | Green badge = reachable, Red = unreachable, Grey = untested |
| **List view** | Card grid showing: connection name, server name, host:port, database name, status badge, last used |

### F2 — Read-Only Query Executor (P0)

| Aspect | Detail |
|--------|--------|
| **Priority** | P0 — Core feature |
| **Status** | 🟡 Not started |
| **Editor** | Dark monospace `<textarea>` with syntax highlighting for SQL keywords |
| **Run button** | Green, sends query to backend |
| **Cancel button** | Red, cancels running query by aborting the Go context |
| **Read-only enforcement** | Backend prepends `SET TRANSACTION READ ONLY;` or wraps query in a read-only transaction |
| **Timeout** | 10s hard limit enforced via Go `context.WithTimeout` |
| **Results display** | HTML `<table>` with striped rows, auto-detected column headers, scrollable for large resultsets |
| **Error display** | Red banner with psql error message (parsed from stderr) |
| **Result limits** | Max 1,000 rows returned (configurable). Shows "Results truncated" notice if exceeded |
| **Multi-statement** | Only first `SELECT` result returned. Multiple statements separated by `;` not supported (safety) |

### F3 — Schema Browser (P1)

| Aspect | Detail |
|--------|--------|
| **Priority** | P1 — Important |
| **Status** | 🟡 Not started |
| **Tree structure** | Database → Schemas → Tables → Columns (type, nullable, default, is_primary_key) |
| **Additional objects** | Views (with definition), Indexes (columns, unique), Constraints (type, columns) |
| **Backend** | Queries `information_schema.columns`, `pg_indexes`, `pg_constraint`, `pg_views` via psql |
| **Refresh** | Manual refresh button per connection (schema doesn't auto-refresh) |
| **Search** | Filter input to search tables/columns by name |

### F4 — Migration Status Viewer (P1)

| Aspect | Detail |
|--------|--------|
| **Priority** | P1 — Important |
| **Status** | 🟡 Not started |
| **Approach** | SSH into server, list migration files in known directory (configurable per connection, e.g. `/app/migrations/`), compare against `schema_migrations` table or equivalent |
| **Display** | Two-column table: **Applied** (green rows) and **Pending** (yellow rows) |
| **Applied column** | Migration filename, version, applied timestamp, checksum |
| **Pending column** | Migration filename, version, file modification time |
| **Config per connection** | Migration directory path, migration table name, filename pattern (e.g. `*.sql`, `V*__*.sql`) |
| **Extensible** | Support Flyway-style (`V1__init.sql`), Alembic-style (`0001_init.py`), and simple directory listing |

### F5 — Backup Status & Trigger (P1)

| Aspect | Detail |
|--------|--------|
| **Priority** | P1 — Important |
| **Status** | 🟡 Not started |
| **Backup history** | Display list of past `pg_dump` runs: timestamp, file size, status (success/failed), output file path |
| **Backup storage** | Backup stored on the target server in a configurable directory (e.g. `/var/backups/postgres/`) |
| **"Backup Now" button** | Triggers `pg_dump` via SSH with `--format=custom` — async execution with progress indicator |
| **Backup metadata** | Stored in Anjungan's own database: `backup_id, connection_id, started_at, completed_at, file_size_bytes, file_path, status, error_message` |
| **Status polling** | Frontend polls `/api/v1/db-connections/{id}/backups` until the latest backup completes |

---

## 4. API Design

### Connection CRUD

```
GET    /api/v1/db-connections                                    — List all connections
POST   /api/v1/db-connections                                    — Add new connection
GET    /api/v1/db-connections/{id}                               — Get connection detail
PUT    /api/v1/db-connections/{id}                               — Update connection
DELETE /api/v1/db-connections/{id}                               — Remove connection
POST   /api/v1/db-connections/{id}/test                          — Test connection
```

**Request (POST/PUT):**
```json
{
  "name": "Production Main DB",
  "server_id": "uuid-of-server",
  "host": "10.0.0.5",
  "port": 5432,
  "dbname": "acme_production",
  "username": "app_user",
  "password": "secret123",
  "ssl_mode": "require",
  "is_active": true
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "uuid-here",
    "name": "Production Main DB",
    "server_id": "uuid-of-server",
    "server_name": "web-prod-01",
    "host": "10.0.0.5",
    "port": 5432,
    "dbname": "acme_production",
    "username": "app_user",
    "ssl_mode": "require",
    "is_active": true,
    "status": "reachable",
    "created_at": "2026-06-11T10:00:00Z",
    "updated_at": "2026-06-11T10:00:00Z"
  }
}
```

### Query Execution

```
POST   /api/v1/db-connections/{id}/query                         — Execute read-only query
```

**Request:**
```json
{
  "query": "SELECT id, name, email, created_at FROM users LIMIT 10"
}
```

**Response (success):**
```json
{
  "success": true,
  "data": {
    "columns": ["id", "name", "email", "created_at"],
    "rows": [
      ["1", "Alice", "alice@example.com", "2026-01-15T10:00:00Z"],
      ["2", "Bob", "bob@example.com", "2026-02-20T11:30:00Z"]
    ],
    "row_count": 2,
    "execution_time_ms": 45,
    "truncated": false
  }
}
```

**Response (error):**
```json
{
  "success": false,
  "error": {
    "code": "QUERY_ERROR",
    "message": "relation \"users\" does not exist",
    "detail": "LINE 1: SELECT * FROM users WHERE ...",
    "hint": "Check the table name and schema"
  }
}
```

### Schema Browser

```
GET    /api/v1/db-connections/{id}/schemas                        — List schemas
GET    /api/v1/db-connections/{id}/schemas/{schema}/tables        — List tables in schema
GET    /api/v1/db-connections/{id}/schemas/{schema}/tables/{table}/columns  — List columns
GET    /api/v1/db-connections/{id}/schemas/{schema}/views         — List views
GET    /api/v1/db-connections/{id}/schemas/{schema}/indexes       — List indexes
GET    /api/v1/db-connections/{id}/tables                         — Quick list: all tables across schemas
```

**Response (tables):**
```json
{
  "success": true,
  "data": [
    {
      "schema": "public",
      "name": "users",
      "type": "table",
      "owner": "postgres",
      "row_estimate": 12500,
      "description": "User accounts"
    }
  ]
}
```

**Response (columns):**
```json
{
  "success": true,
  "data": [
    {
      "column_name": "id",
      "data_type": "uuid",
      "is_nullable": false,
      "column_default": "gen_random_uuid()",
      "is_primary_key": true,
      "is_unique": true,
      "character_maximum_length": null
    }
  ]
}
```

### Migration Status

```
GET    /api/v1/db-connections/{id}/migrations                     — Migration status
```

**Response:**
```json
{
  "success": true,
  "data": {
    "applied": [
      {
        "version": "001",
        "filename": "V001__create_users.sql",
        "applied_at": "2026-01-10T08:00:00Z",
        "checksum": "abc123"
      }
    ],
    "pending": [
      {
        "version": "002",
        "filename": "V002__add_projects.sql",
        "file_mtime": "2026-02-01T10:00:00Z"
      }
    ],
    "migration_type": "flyway",
    "migration_dir": "/app/migrations",
    "migration_table": "schema_migrations"
  }
}
```

### Backups

```
GET    /api/v1/db-connections/{id}/backups                        — Backup history
POST   /api/v1/db-connections/{id}/backups                        — Trigger backup now
```

**List response:**
```json
{
  "success": true,
  "data": [
    {
      "id": "backup-uuid",
      "started_at": "2026-06-11T06:00:00Z",
      "completed_at": "2026-06-11T06:01:23Z",
      "file_size_bytes": 268435456,
      "file_size_human": "256 MB",
      "file_path": "/var/backups/postgres/acme_production_20260611_060000.dump",
      "status": "success",
      "error_message": null,
      "triggered_by": "manual"
    }
  ]
}
```

**Trigger response:**
```json
{
  "success": true,
  "data": {
    "id": "backup-uuid",
    "status": "running",
    "started_at": "2026-06-11T12:00:00Z"
  }
}
```

### Query History

```
GET    /api/v1/db-connections/{id}/query-history                  — Past queries (last 50)
```

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "id": "history-uuid",
      "query": "SELECT * FROM users LIMIT 10",
      "executed_at": "2026-06-11T11:30:00Z",
      "execution_time_ms": 45,
      "row_count": 10,
      "status": "success",
      "error_message": null
    }
  ]
}
```

---

## 5. Database Schema

### New Tables

#### `db_connections`

```sql
CREATE TABLE db_connections (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name              VARCHAR(100) NOT NULL,
    server_id         UUID REFERENCES servers(id),
    host              VARCHAR(255) NOT NULL,
    port              INT DEFAULT 5432,
    dbname            VARCHAR(100) NOT NULL,
    username          VARCHAR(100) NOT NULL,
    password_encrypted TEXT NOT NULL,
    ssl_mode          VARCHAR(20) DEFAULT 'require',
    is_active         BOOLEAN DEFAULT true,
    created_at        TIMESTAMPTZ DEFAULT now(),
    updated_at        TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_db_connections_server ON db_connections(server_id);
CREATE INDEX idx_db_connections_active ON db_connections(is_active);
```

#### `db_query_history`

```sql
CREATE TABLE db_query_history (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    connection_id     UUID NOT NULL REFERENCES db_connections(id) ON DELETE CASCADE,
    query             TEXT NOT NULL,
    executed_at       TIMESTAMPTZ DEFAULT now(),
    execution_time_ms INT,
    row_count         INT,
    status            VARCHAR(20) NOT NULL DEFAULT 'success',
    error_message     TEXT,
    executed_by       UUID REFERENCES users(id)
);

CREATE INDEX idx_query_history_connection ON db_query_history(connection_id);
CREATE INDEX idx_query_history_time ON db_query_history(executed_at DESC);

-- Keep only last 50 per connection via trigger or periodic cleanup
```

#### `db_backups`

```sql
CREATE TABLE db_backups (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    connection_id     UUID NOT NULL REFERENCES db_connections(id) ON DELETE CASCADE,
    started_at        TIMESTAMPTZ,
    completed_at      TIMESTAMPTZ,
    file_size_bytes   BIGINT,
    file_path         TEXT,
    status            VARCHAR(20) NOT NULL DEFAULT 'pending',
    error_message     TEXT,
    triggered_by      VARCHAR(50) DEFAULT 'manual',
    created_at        TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_backups_connection ON db_backups(connection_id);
```

### Migration: Config Per Connection

```sql
-- Optional: migration_config per connection
ALTER TABLE db_connections ADD COLUMN migration_dir TEXT DEFAULT '/app/migrations';
ALTER TABLE db_connections ADD COLUMN migration_table VARCHAR(100) DEFAULT 'schema_migrations';
ALTER TABLE db_connections ADD COLUMN migration_pattern VARCHAR(100) DEFAULT 'V*__*.sql';

-- Optional: backup_dir per connection
ALTER TABLE db_connections ADD COLUMN backup_dir TEXT DEFAULT '/var/backups/postgres';
```

### Design: Encryption at Rest

```sql
-- password_encrypted uses AES-256-GCM
-- Go: ciphertext = encrypt(plaintext_password, master_key)
-- Encryption happens in application layer before INSERT
-- Decryption only happens in-memory during SSH psql execution
```

---

## 6. UX Flow

### Connection List Page

```
┌─────────────────────────────────────────────────────────────┐
│  Database Manager  [➕ New Connection]                       │
│                                                              │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │  🔍 Search connections...                                │ │
│  └─────────────────────────────────────────────────────────┘ │
│                                                              │
│  ┌────────────────────┐  ┌────────────────────┐              │
│  │  Production Main   │  │  Staging Replica    │              │
│  │  🟢 Reachable      │  │  🟢 Reachable       │              │
│  │  web-prod-01:5432  │  │  web-stg-01:5432    │              │
│  │  acme_production   │  │  acme_staging       │              │
│  │  [Query] [Schema]  │  │  [Query] [Schema]   │              │
│  │  [Migrations]      │  │  [Migrations]       │              │
│  │  [Backups ▾]       │  │  [Backups ▾]        │              │
│  └────────────────────┘  └────────────────────┘              │
│                                                              │
│  ┌────────────────────┐  ┌────────────────────┐              │
│  │  Analytics DB      │  │  Legacy DB          │              │
│  │  🔴 Unreachable    │  │  ⚫ Untested         │              │
│  │  ...               │  │  ...                 │              │
│  └────────────────────┘  └────────────────────┘              │
└─────────────────────────────────────────────────────────────┘
```

### Connection Detail — Query Tab

```
┌─────────────────────────────────────────────────────────────┐
│  ← DB Connections  /  Production Main DB                    │
│  [Query] [Schema] [Migrations] [Backups]                    │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────┐│
│  │  SQL Editor (dark, monospace)                           ││
│  │                                                         ││
│  │  SELECT id, name, email, created_at                     ││
│  │  FROM users                                              ││
│  │  WHERE status = 'active'                                 ││
│  │  ORDER BY created_at DESC                                ││
│  │  LIMIT 10;                                               ││
│  │                                                         ││
│  │  [▶ Run (green)]  [■ Cancel (red)]                      ││
│  └─────────────────────────────────────────────────────────┘│
│                                                              │
│  Execution time: 45ms  •  Rows: 10                          │
│  ┌─────────────────────────────────────────────────────────┐│
│  │  id │ name  │ email              │ created_at           ││
│  │  ───┼───────┼────────────────────┼──────────────────────││
│  │  1  │ Alice │ alice@example.com  │ 2026-01-15 10:00:00 ││
│  │  2  │ Bob   │ bob@example.com    │ 2026-02-20 11:30:00 ││
│  │  3  │ Carol │ carol@example.com  │ 2026-03-10 09:15:00 ││
│  │  ...│ ...   │ ...                │ ...                  ││
│  └─────────────────────────────────────────────────────────┘│
│                                                              │
│  Query History (last 5)                                      │
│  ┌─────────────────────────────────────────────────────────┐│
│  │ 🔍 SELECT * FROM users LIMIT 10        -- 2m ago  ✓    ││
│  │ 🔍 SELECT count(*) FROM orders          -- 15m ago ✓   ││
│  │ 🔍 SELECT id FROM...                   -- 1h ago  ✗    ││
│  └─────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
```

### Connection Detail — Schema Tab

```
┌─────────────────────────────────────────────────────────────┐
│  ← DB Connections  /  Production Main DB                    │
│  [Query] [Schema] [Migrations] [Backups]                    │
├─────────────────────────────────────────────────────────────┤
│  🔍 Filter tables/columns...                                │
│                                                              │
│  📁 public                                                   │
│  │  📋 users                                                 │
│  │  │  🔑 id (uuid, NOT NULL, DEFAULT gen_random_uuid())     │
│  │  │  📝 name (varchar(255), NOT NULL)                     │
│  │  │  📝 email (varchar(255), NOT NULL, UNIQUE)            │
│  │  │  📝 status (varchar(50), DEFAULT 'active')            │
│  │  │  🕐 created_at (timestamptz)                          │
│  │  ├── 🔍 idx_users_email (UNIQUE)                         │
│  │  └── 🔗 fk_users_roles (roles.id)                        │
│  │                                                          │
│  │  📋 orders                                                │
│  │  │  🔑 id (uuid, NOT NULL)                               │
│  │  │  📝 user_id (uuid, NOT NULL, FK → users.id)           │
│  │  │  📝 total (decimal, NOT NULL)                          │
│  │  │  ...                                                  │
│  │  └── 🔍 idx_orders_user (user_id)                        │
│  │                                                          │
│  │  👁️  active_users (VIEW)                                  │
│  │     SELECT ... FROM users WHERE status = 'active'        │
│  │                                                          │
│  └── 📁 audit                                                │
│      └── 📋 audit_logs                                       │
│                                                              │
│  [🔄 Refresh Schema]                                         │
└─────────────────────────────────────────────────────────────┘
```

### Connection Detail — Migrations Tab

```
┌─────────────────────────────────────────────────────────────┐
│  ← DB Connections  /  Production Main DB                    │
│  [Query] [Schema] [Migrations] [Backups]                    │
├─────────────────────────────────────────────────────────────┤
│  Migration directory: /app/migrations                       │
│  Migration table: schema_migrations                         │
│                                                              │
│  ✅ Applied (3)                                              │
│  ┌─────────────────────────────────────────────────────────┐│
│  │  Version │ Filename                    │ Applied At      ││
│  │  ────────┼─────────────────────────────┼────────────────││
│  │  001     │ V001__create_users.sql      │ 2026-01-10     ││
│  │  002     │ V002__add_projects.sql      │ 2026-02-01     ││
│  │  003     │ V003__add_indexes.sql       │ 2026-03-15     ││
│  └─────────────────────────────────────────────────────────┘│
│                                                              │
│  🟡 Pending (2)                                              │
│  ┌─────────────────────────────────────────────────────────┐│
│  │  Version │ Filename                    │ Modified At     ││
│  │  ────────┼─────────────────────────────┼────────────────││
│  │  004     │ V004__add_notifications.sql │ 2026-04-01     ││
│  │  005     │ V005__add_audit_logs.sql    │ 2026-05-10     ││
│  └─────────────────────────────────────────────────────────┘│
│                                                              │
│  [🔄 Refresh]                                                │
└─────────────────────────────────────────────────────────────┘
```

### Connection Detail — Backups Tab

```
┌─────────────────────────────────────────────────────────────┐
│  ← DB Connections  /  Production Main DB                    │
│  [Query] [Schema] [Migrations] [Backups]                    │
├─────────────────────────────────────────────────────────────┤
│  Backup directory: /var/backups/postgres                    │
│                                                              │
│  [🗄️ Backup Now]                                            │
│                                                              │
│  ┌─────────────────────────────────────────────────────────┐│
│  │  Started At           │ Size    │ Status   │ Actions    ││
│  │  ─────────────────────┼─────────┼──────────┼─────────── ││
│  │  Today, 06:00 UTC     │ 256 MB  │ ✅ Done  │ [Download] ││
│  │  Yesterday, 06:00 UTC │ 250 MB  │ ✅ Done  │ [Download] ││
│  │  Jun 09, 06:00 UTC    │ 248 MB  │ ✅ Done  │ [Download] ││
│  │  Jun 08, 06:00 UTC    │ —       │ ❌ Failed │ [Retry]   ││
│  │                       │         │ disk full │            ││
│  └─────────────────────────────────────────────────────────┘│
│                                                              │
│  Latest backup: 256 MB (Today at 06:00 UTC)                  │
│  Total backups stored: 12                                    │
│  Total size: 3.2 GB                                          │
└─────────────────────────────────────────────────────────────┘
```

### Add Connection Flow

```
┌─────────────────────────────────────────────────────────────┐
│  New Database Connection                                     │
│                                                              │
│  Name:           [Production Main DB                    ]   │
│  Server:         [web-prod-01 (10.0.0.5) ▼             ]   │
│  ── or enter manually ──                                    │
│  Host:           [10.0.0.5                              ]   │
│  Port:           [5432                                  ]   │
│  Database name:  [acme_production                       ]   │
│  Username:       [app_user                              ]   │
│  Password:       [••••••••••••••••                      ]   │
│  SSL Mode:       [require ▼                             ]   │
│                                                              │
│  [Test Connection]  [🔄 Test]                               │
│                                                              │
│  ● Connection successful (1.2ms)                             │
│                                                              │
│  [Cancel]                              [Save Connection]     │
└─────────────────────────────────────────────────────────────┘
```

---

## 7. Mockup Screenshots

> Mockup sketches to be created in `sketches/database-manager/` on branch `feat/database-manager`.

### 7.1 — Connection List

Card-based layout showing all saved DB connections. Each card shows: connection name, server name, host:port, database name, status badge (green/red/grey). Action buttons at bottom: Query, Schema, Migrations, Backups.

### 7.2 — Query Tab

Split layout: top half is the dark monospace SQL editor with Run (green) + Cancel (red) buttons. Bottom half shows results as a striped HTML table. Execution time + row count badge above results. Query history dropdown at the very bottom.

### 7.3 — Schema Tab

Collapsible tree panel on the left with full width on mobile. Icons: 📁 schema, 📋 table, 🔑 primary key, 📝 column, 🔍 index, 👁️ view, 🔗 foreign key. Search/filter input at top. Clicking a column shows its properties inline.

### 7.4 — Migrations Tab

Two-column split: left green section "Applied" with version, filename, applied_at. Right yellow section "Pending" with version, filename, modified_at. Summary bar at top showing migration directory and table name.

### 7.5 — Backups Tab

Timeline-style list with date, size (human-readable), status badge (success/failed/pending/running). "Backup Now" button at top. Summary footer showing latest backup info and total storage used.

---

## 8. Implementation Roadmap

### Phase 1 — Foundation (P0)

| # | Task | Files | Effort | Depends On |
|---|------|-------|--------|------------|
| 1 | Create `db_connections` migration + CRUD | `backend/migrations/000034_db_connections.sql`, model, repository, handler | 1 day | — |
| 2 | SSH-based psql executor (read-only, timeout, cancel) | `backend/internal/dbmanager/executor.go` | 2 days | #1 |
| 3 | Query UI — editor + results table | `frontend/src/routes/db/+page.svelte`, query editor component | 2 days | #2 |
| 4 | Connection CRUD frontend | `frontend/src/routes/db/manage/` , add/edit/list pages | 1 day | #1 |
| 5 | Register API routes | `backend/internal/server/server.go` | Small | #1 |

### Phase 2 — Schema & Migrations (P1)

| # | Task | Files | Effort | Depends On |
|---|------|-------|--------|------------|
| 6 | Schema browser — information_schema queries + tree UI | `backend/internal/dbmanager/schema.go`, frontend tree component | 1 day | #2 |
| 7 | Migration status — file listing + compare | `backend/internal/dbmanager/migrations.go`, frontend table | 1 day | #2 |
| 8 | Query history — `db_query_history` table + API | migration, handler, frontend dropdown | 1 day | #2 |
| 9 | Schema tab + Migrations tab in frontend | Tab panel component, detail page routing | 1 day | #6, #7 |

### Phase 3 — Backups & Discovery (P1)

| # | Task | Files | Effort | Depends On |
|---|------|-------|--------|------------|
| 10 | Backup executor — pg_dump via SSH + metadata storage | `backend/internal/dbmanager/backups.go`, `db_backups` migration | 1 day | #2 |
| 11 | Backups tab UI — timeline + "Backup Now" button | Frontend backup timeline component | 1 day | #10 |
| 12 | Auto-discovery — `pg_isready` scan on all servers | `backend/internal/dbmanager/discovery.go` | 1 day | #1 |
| 13 | Test connection button + status badge | Frontend + backend endpoint | 0.5 day | #1 |

---

## 9. Non-Functional Requirements

| Requirement | Target | Notes |
|-------------|--------|-------|
| **Query timeout** | 10s max | Hard limit enforced via Go `context.WithTimeout` |
| **Read-only enforcement** | Enforced at DB level | `SET TRANSACTION READ ONLY` before every query, not just UI check |
| **Credential storage** | Encrypted at rest | AES-256-GCM for `password_encrypted` column; master key via environment variable |
| **Query history retention** | Last 50 queries per connection | Auto-prune on INSERT; cap enforced in application layer |
| **Resultset limit** | 1,000 rows max | Returned rows capped; "truncated" flag in response |
| **Concurrent queries** | 1 active query per connection | Subsequent Run clicks queue or warn "query already running" |
| **Page load** | < 1s for connection list | Cache schema metadata; lazy-load migration/backup status |
| **API response time** | < 200ms for non-query endpoints | Queries excluded from this target due to variable DB performance |
| **Backup storage** | Remote (target server) | Backups stored on the database server itself, not streamed through Anjungan |
| **Audit** | All query executions logged | `db_query_history` captures who, what, when, execution time, row count |

---

## 10. Dependencies & Integration Points

| Dependency | Type | Notes |
|-----------|------|-------|
| `servers` table | Existing | Auto-discovery iterates over known servers; connection form references `server_id` |
| SSH executor module | Existing | Reuses `infra/ssh` package for all psql/pg_dump operations |
| `pgx` (Go PostgreSQL driver) | Existing | Already used by Anjungan for its own DB; not used for target DBs (those go via psql SSH) |
| `crypto/aes` + `crypto/cipher` | New (in app) | AES-256-GCM encryption for stored DB passwords |
| SvelteKit routing | Existing | New route `/db` with subroutes |
| Authentication middleware | Existing | All endpoints require valid JWT; reuse existing auth check |
| Audit log | Existing | DB connection CRUD operations logged to `audit_logs` |

### Affected Modules

| Module | Change |
|--------|--------|
| `backend/internal/server/server.go` | Register new route group under `/api/v1/db-connections` |
| `frontend/src/lib/api.svelte.js` | Add API client methods for DB manager endpoints |
| `frontend/src/lib/components/layout/Sidebar.svelte` | Add "Database" navigation item |
| `backend/internal/common/model/model.go` | Add `DBConnection`, `DBQueryHistory`, `DBBackup` structs |

---

## 11. Edge Cases & Error Handling

| Scenario | Behavior |
|----------|----------|
| **Connection unreachable** | Status badge shows 🔴 red. Query returns error "connection refused" with retry suggestion. Test connection gives specific error detail |
| **Query timeout (10s)** | Backend returns 408 Timeout error. Frontend shows "Query timed out after 10s" banner with option to simplify query |
| **Write query attempted** | `SET TRANSACTION READ ONLY` fails — PostgreSQL returns error. Backend returns error: "Write statements are not allowed" |
| **Empty resultset** | Table renders with headers only, shows "0 rows returned" message |
| **Large resultset (>1,000 rows)** | First 1,000 rows returned with `"truncated": true` flag. Frontend shows notice: "Showing first 1,000 rows of N total" |
| **Query syntax error** | Parse psql stderr, extract line number + hint. Frontend shows red banner with error, highlights error position in editor |
| **Concurrent query on same connection** | Show warning "A query is already running" — queue second or cancel first |
| **Password decryption failure** | Backend returns 500 with generic error (no details leaked). Logs internal error details |
| **SSL mode mismatch** | Connection test catches SSL errors early — specific error message about SSL configuration |
| **Backup disk full** | pg_dump fails — error captured in `db_backups` table with status "failed" and disk space detail |
| **No migrations directory** | Migration status returns empty applied + pending with info "Migration directory not found" |
| **Connection deleted with history** | `ON DELETE CASCADE` on `db_query_history` and `db_backups` — cleanup is automatic |
| **Server deleted with active connections** | Connections become orphaned (server_id nullable). Show "Server deleted" badge. Ask user to reassign or delete connection |

---

## 12. Future Considerations

| Feature | When | Why Skip Now |
|---------|------|-------------|
| **Multiple result tabs** | v2 | Allow running multiple queries and switching between results |
| **EXPLAIN ANALYZE viewer** | v2 | Query plan visualization — needs parser for PostgreSQL plan output |
| **CSV/JSON export** | v2 | Download query results as CSV or JSON file |
| **Visual query builder** | v2.5 | Drag-and-drop table/column selector for non-SQL users |
| **Scheduled backups** | v2 | Cron-style backup schedule (daily at 2AM, retain 7 days) |
| **Backup download** | v2 | Stream backup file from target server through Anjungan to browser |
| **Cross-DB query (federated)** | v3 | Join across PostgreSQL instances (via dblink/postgres_fdw) |
| **Schema diff** | v3 | Compare schema between two databases (production vs staging) |
| **Connection pooling** | v3 | Persistent connection pool instead of SSH psql per query (performance) |
| **Multi-engine support** | v3 | MySQL, MariaDB, SQLite support beyond PostgreSQL |
| **Slow query detection** | v3 | Auto-detect queries taking > 5s and log them separately |

---

## 13. PRD Cross-References

| PRD | Relationship |
|-----|-------------|
| [PRD.md](PRD.md) | Master PRD — Database Manager extends the server management capabilities from Phase 1 |
| [PRD-uptime-monitoring.md](PRD-uptime-monitoring.md) | Shared server infrastructure — DB connections live on servers listed in uptime monitoring |
| [PRD-compliance.md](PRD-compliance.md) | Backup governance aligns with compliance requirements for data retention |

---

## 14. References

- **PostgreSQL psql documentation:** https://www.postgresql.org/docs/current/app-psql.html
- **pg_dump custom format:** https://www.postgresql.org/docs/current/app-pgdump.html
- **AES-256-GCM in Go:** https://pkg.go.dev/crypto/cipher#NewGCM
- **Anjungan SSH executor:** `backend/internal/infra/ssh/`
- **Existing server model:** `backend/internal/common/model/model.go`
- **Environment variable pattern for secrets:** `backend/internal/config/config.go`
- **Migration examples:** `backend/migrations/`
