# Feature Toggle System — Referensi Arsitektur

> **Tujuan:** Dokumentasi arsitektur modular Anjungan — sistem feature toggle yang memungkinkan user mengaktifkan/menonaktifkan fitur sesuai kebutuhan dan resource yang tersedia.
>
> **Status:** Direncanakan (tahap framework registry)

---

## Daftar Isi

- [1. Konsep Dasar](#1-konsep-dasar)
- [2. Konfigurasi](#2-konfigurasi)
- [3. Backend — Feature Registry](#3-backend--feature-registry)
- [4. Frontend — Dynamic Sidebar](#4-frontend--dynamic-sidebar)
- [5. Database & Migrations](#5-database--migrations)
- [6. Feature Dependencies](#6-feature-dependencies)
- [7. Directory Layout](#7-directory-layout)
- [8. Implementasi — Tahapan](#8-implementasi--tahapan)
- [9. Decision Log](#9-decision-log)

---

## 1. Konsep Dasar

Target utama: **setiap user bisa milih fitur yang mereka butuh aja**, sisanya disable — hemat resource, sidebar bersih, kode mati gak di-load.

```
User A (MiniPC 4c/8GB)          User B (VPS 8c/32GB)
┌────────────────────┐          ┌──────────────────────────┐
│ ✅ Core            │          │ ✅ Core                  │
│ ✅ Uptime          │          │ ✅ Uptime                │
│ ✅ SSL Monitors    │          │ ✅ SSL Monitors          │
│ ✅ Compliance      │          │ ✅ Compliance            │
│ ❌ CrowdSec        │          │ ✅ CrowdSec              │
│ ❌ Container Sec   │          │ ✅ Container Security    │
│ ❌ Trivy           │          │ ✅ Trivy                 │
│ ❌ Netdata         │          │ ✅ Netdata               │
│                     │          │ ✅ Secret Scanning       │
│ RAM: ~3GB total     │          │ ❌ Ansible Semaphore     │
└────────────────────┘          └──────────────────────────┘
```

### Prinsip Desain

| Prinsip | Penjelasan |
|---------|-----------|
| **Modular by design, not retrofit** | Arsitektur dari awal mikirin modular, bukan nambahin toggle di sistem yang monolithic |
| **Runtime, not build-time** | Fitur enable/disable via config, bukan compile-time flags. Gak perlu rebuild image buat matiin fitur |
| **Self-contained features** | Satu folder fitur = semua kode yang dibutuhkan: handler, scheduler, migration, test |
| **Always-run migrations** | Semua migration jalan di startup — disable cuma hide UI + stop cron, bukan drop table |

---

## 2. Konfigurasi

### Config File (`config.yaml`)

```yaml
# Core — selalu ON, gak bisa disable
core:
  features: [auth, servers, containers, audit_log]

# Optional modules — user pilih sesuai kebutuhan
features:
  uptime: true
  ssl_monitoring: true
  compliance: true
  login_activity: true
  crowdsec: false
  container_security: false
  trivy: false
  secret_scanning: false
  netdata: false
  runbooks: false
  renovate: false
```

File diletakkan di `config/` atau path yang bisa di-mount ke container, di-load saat startup backend.

### Default Behavior

- **Missing key** → dianggap `false` (opt-in, bukan opt-out)
- **Core** — gak bisa di-disable, akan direject kalau user coba matikan
- **Runtime reload** — opsional (fase 2). Untuk fase 1, restart backend diperlukan setelah ubah config

### Env Override (Opsional)

Untuk deployment yang gak bisa mount file:

```yaml
# config.yaml bisa di-override env
# Format: ANJUNGAN_FEATURES_UPTIME=true
anjungan:
  features:
    uptime: ${ANJUNGAN_FEATURES_UPTIME:-true}
    crowdsec: ${ANJUNGAN_FEATURES_CROWDSEC:-false}
```

---

## 3. Backend — Feature Registry

### Core Struct

```go
// pkg/feature/registry.go

type Feature struct {
    Name        string
    Enabled     bool
    Priority    int              // Load order (lower = first)
    Routes      func(r chi.Router)
    CronJobs    []CronJob
    Migrations  []Migration
    Dependencies []string        // Required features (e.g., ["core.servers"])
    InitFunc    func() error     // Startup initialization (start goroutines, connect ke service external)
}

type CronJob struct {
    Name     string
    Interval time.Duration
    Func     func()
}

type Migration struct {
    Version int
    Up      string // SQL
    Down    string // SQL (opsional, untuk rollback)
}

var registry = map[string]Feature{}

func Register(f Feature) {
    registry[f.Name] = f
}
```

### Registration Pattern

Setiap fitur register dirinya sendiri di `init()`:

```go
// pkg/features/uptime/register.go
package uptime

import "anjungan/pkg/feature"

func init() {
    feature.Register(feature.Feature{
        Name:       "uptime",
        Priority:   10,
        Routes:     func(r chi.Router) {
            r.Get("/api/v1/uptime-monitors", ListMonitors)
            r.Post("/api/v1/uptime-monitors", CreateMonitor)
            r.Delete("/api/v1/uptime-monitors/{id}", DeleteMonitor)
        },
        CronJobs: []feature.CronJob{
            {Name: "uptime-check", Interval: 5 * time.Minute, Func: CheckAllMonitors},
        },
        Dependencies: []string{"core.servers"},
        InitFunc: func() error {
            // Initialize HTTP client, connect to external service, etc.
            return nil
        },
    })
}
```

### Startup Sequence

```
1. Load config.yaml
2. Baca daftar enabled features dari config
3. For each enabled feature:
   a. Validasi dependencies (semua dependency harus enabled juga)
   b. Panggil InitFunc()
   c. Register routes ke Chi router
   d. Register cron jobs ke scheduler
4. For each disabled feature:
   a. Skip — gak ada route, gak ada cron, gak ada goroutine
5. Start HTTP server
6. Start cron scheduler (hanya job dari enabled features)
```

```go
// cmd/server/main.go (simplified)
func main() {
    cfg := config.Load("config.yaml")
    registry := feature.GetRegistry()

    r := chi.NewRouter()
    sched := cron.NewScheduler()

    for _, f := range registry.SortedByPriority() {
        if !cfg.Features[f.Name] {
            continue // skip feature yang disabled
        }

        // Validasi dependencies
        for _, dep := range f.Dependencies {
            if !cfg.Features[dep] {
                log.Fatalf("feature %s requires %s which is disabled", f.Name, dep)
            }
        }

        // Init
        if err := f.InitFunc(); err != nil {
            log.Fatalf("failed to init feature %s: %v", f.Name, err)
        }

        // Routes
        f.Routes(r)

        // Cron jobs
        for _, job := range f.CronJobs {
            sched.Every(job.Interval).Do(job.Func)
        }
    }

    // API endpoint buat frontend — return daftar enabled features
    r.Get("/api/features", func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(registry.EnabledNames())
    })

    go sched.Start()
    http.ListenAndServe(":8080", r)
}
```

### API Endpoint `/api/features`

Response:

```json
{
  "enabled": ["uptime", "ssl_monitoring", "compliance", "login_activity"],
  "all": [
    {"name": "uptime", "enabled": true, "label": "Uptime Monitoring", "icon": "..."},
    {"name": "crowdsec", "enabled": false, "label": "Security Events", "icon": "..."},
    {"name": "netdata", "enabled": false, "label": "Resource Monitoring", "icon": "..."}
  ]
}
```

Frontend pake data ini untuk:
1. Render dynamic sidebar (hanya enabled features)
2. Route guard — redirect ke `/features/disabled` kalo akses fitur yang disable
3. Settings page — tampilin toggle enable/disable (future: UI-based config)

### Feature Metadata (Opsional)

Setiap feature bisa bawa metadata tambahan:

```go
type FeatureMeta struct {
    Name        string   `json:"name"`
    Label       string   `json:"label"`
    Description string   `json:"description"`
    Icon        string   `json:"icon"`       // Iconify icon name
    Category    string   `json:"category"`   // sidebar grouping
    ResourceEstimate string `json:"resource_estimate"` // "~30MB RAM"
    DocURL      string   `json:"doc_url"`    // link ke dokumentasi
    Dependencies []string `json:"dependencies"`
}
```

Ini dikirim bareng `/api/features` response buat frontend render.

---

## 4. Frontend — Dynamic Sidebar

### Data Flow

```
Browser                  SvelteKit                   Backend
  │                         │                          │
  │── GET / ───────────────►│                          │
  │                         │── GET /api/features ────►│
  │                         │◄── JSON features ────────│
  │◄── Render sidebar ──────│                          │
  │   (only enabled items)  │                          │
```

### Implementation

```svelte
<!-- src/routes/+layout.svelte -->
<script>
  let enabledFeatures = $state([]);
  let allFeatures = $state([]);

  onMount(async () => {
    const res = await fetch('/api/features');
    const data = await res.json();
    enabledFeatures = data.enabled;
    allFeatures = data.all;
  });

  const navItems = [
    // Core — selalu visible
    { href: '/',             label: 'Overview',       icon: 'solar:chart-line-duotone', feature: 'core' },
    { href: '/servers',      label: 'Servers',        icon: 'solar:server-square-duotone', feature: 'core' },
    { href: '/containers',   label: 'Containers',     icon: 'solar:box-duotone', feature: 'core' },

    // Optional features — filter by enabledFeatures
    { href: '/uptime',       label: 'Uptime',         icon: 'solar:heart-pulse-duotone', feature: 'uptime' },
    { href: '/ssl-monitors', label: 'SSL Monitors',   icon: 'solar:shield-check-duotone', feature: 'ssl_monitoring' },
    { href: '/compliance',   label: 'Compliance',     icon: 'solar:shield-warning-duotone', feature: 'compliance' },
    { href: '/security-events', label: 'Security Events', icon: 'solar:bug-duotone', feature: 'crowdsec' },
    { href: '/container-security', label: 'Container Security', icon: 'solar:skull-duotone', feature: 'container_security' },
    { href: '/monitoring',   label: 'Resource Usage', icon: 'solar:graph-new-duotone', feature: 'netdata' },
    // ...
  ];

  $: visibleItems = navItems.filter(
    item => item.feature === 'core' || enabledFeatures.includes(item.feature)
  );
</script>

<nav>
  {#each visibleItems as item}
    <a href={item.href}>
      <span class="icon">{@html item.icon}</span>
      {item.label}
    </a>
  {/each}
</nav>
```

### Route Guard

Buat halaman yang tergantung feature tertentu:

```svelte
<!-- src/routes/uptime/+layout.svelte -->
<script>
  import { page } from '$app/stores';

  onMount(async () => {
    const res = await fetch('/api/features');
    const { enabled } = await res.json();
    if (!enabled.includes('uptime')) {
      goto('/features/disabled');
    }
  });
</script>

<slot />
```

Atau pake centralized guard:

```svelte
<!-- src/lib/guard.svelte.js -->
export function requireFeature(featureName) {
  return async () => {
    const res = await fetch('/api/features');
    const { enabled } = await res.json();
    if (!enabled.includes(featureName)) {
      goto('/features/disabled');
    }
  };
}

// Usage in +page.svelte:
// export const load = requireFeature('uptime');
```

### Disabled Page

Halaman informatif kalo user akses fitur yang disable:

```svelte
<!-- src/routes/features/disabled/+page.svelte -->
<script>
  import { page } from '$app/stores';
</script>

<div class="disabled-page">
  <h1>🔒 Feature Disabled</h1>
  <p>
    The feature you're trying to access is not enabled in your configuration.
  </p>
  <p>
    To enable it, set <code>features.{feature_name}: true</code> in your config.yaml
    and restart the backend.
  </p>
  <a href="/">← Back to Overview</a>
</div>
```

---

## 5. Database & Migrations

### Opsi yang Dipertimbangkan

| Opsi | Mekanisme | Kelebihan | Kekurangan |
|------|-----------|-----------|------------|
| **A — Always-run** (✅ Pilihan) | Semua migration jalan di startup. Fitur disable = hide UI + stop cron, tabel tetap ada | Simpel, data siap kapanpun fitur di-enable, gak ada migration hell | Tabel kosong untuk fitur disable (negligible overhead) |
| **B — Conditional** | Migration jalan per feature. Tabel dibuat pas enable, di-drop pas disable | Zero DB objects untuk fitur disable | Ribet: perlu migration tracker per feature, risk data loss |

### Keputusan: **Opsi A — Always-run**

PostgreSQL gak masalah punya tabel kosong. Overhead tabel kosong di Postgres praktis nol.

```
Seluruh migration jalan di setiap startup ──► Feature disable = hide + stop cron
                                                  Bukan drop table
```

```go
// Start example:
func runMigrations(db *sql.DB, registry []feature.Feature) error {
    // Semua migration jalan — gak peduli enabled/disable
    for _, f := range registry {
        for _, m := range f.Migrations {
            _, err := db.Exec(m.Up)
            if err != nil {
                return fmt.Errorf("migration %s/%d: %w", f.Name, m.Version, err)
            }
        }
    }
    return nil
}
```

### Catatan Penting

- **Jangan drop table pas disable** — data hilang, user komplen pas enable balik
- **Add column di migration selalu aman** — walau feature disable, column baru masuk DB tanpa efek samping
- **Kalau mau true zero-overhead** — bisa pake PostgreSQL partitioning + deferred table creation, tapi gak worth it untuk skala Anjungan

---

## 6. Feature Dependencies

### Konsep

Beberapa fitur bisa bergantung ke fitur lain. Contoh:

```
Container Security ──dep──► Compliance (butuh scoring engine)
CrowdSec            ──dep──► Core.Servers (butuh server list)
Uptime              ──dep──► Core.Servers (butuh server list)
Login Activity      ──dep──► Core.Auth (butuh user auth)
```

### Startup Validation

Waktu startup, system validasi DAG dependencies:

```go
func validateDependencies(cfg Config, registry map[string]Feature) error {
    for name, f := range registry {
        if !cfg.Features[name] {
            continue
        }
        for _, dep := range f.Dependencies {
            if !cfg.Features[dep] {
                return fmt.Errorf(
                    "feature '%s' requires '%s' which is disabled",
                    name, dep,
                )
            }
        }
    }
    return nil
}
```

### Circular Dependency Detection

```go
func detectCycles(registry map[string]Feature) error {
    visited := make(map[string]bool)
    inStack := make(map[string]bool)

    var dfs func(name string) error
    dfs = func(name string) error {
        visited[name] = true
        inStack[name] = true

        for _, dep := range registry[name].Dependencies {
            if !visited[dep] {
                if err := dfs(dep); err != nil {
                    return err
                }
            } else if inStack[dep] {
                return fmt.Errorf("circular dependency: %s <-> %s", name, dep)
            }
        }

        inStack[name] = false
        return nil
    }

    for name := range registry {
        if !visited[name] {
            if err := dfs(name); err != nil {
                return err
            }
        }
    }
    return nil
}
```

---

## 7. Directory Layout

### Target Layout (Setelah Refactor)

```
backend/
├── cmd/server/main.go           # Entry point — feature.Start()
├── pkg/
│   ├── feature/                 # Framework
│   │   ├── registry.go          # Feature, Register(), Start()
│   │   ├── config.go            # Baca config.yaml
│   │   ├── middleware.go        # Inject enabled features ke request context
│   │   └── api.go               # Handler buat GET /api/features
│   ├── core/                    # Core features — selalu ON
│   │   ├── auth/                # Auth handler, middleware
│   │   ├── servers/             # Server management
│   │   ├── containers/          # Docker container
│   │   └── audit_log/           # Audit logging
│   └── features/                # Optional features — masing-masing self-contained
│       ├── uptime/
│       │   ├── register.go      # init() → feature.Register(...)
│       │   ├── handler.go       # HTTP handler
│       │   ├── scheduler.go     # Cron job logic
│       │   └── migration.go     # SQL migrations (embedded)
│       ├── ssl_monitoring/
│       │   └── ...
│       ├── compliance/
│       │   └── ...
│       ├── crowdsec/
│       │   └── ...
│       ├── container_security/
│       │   └── ...
│       ├── netdata/
│       │   └── ...
│       └── trivy/
│           └── ...
├── config/
│   └── config.yaml              # Feature toggle config
└── migrations/                  # Global migrations (shared)
```

### Per Feature — Checklist Minimal

Setiap folder fitur minimal punya:

| File | Responsibility | Wajib? |
|------|---------------|--------|
| `register.go` | `init()` → `feature.Register(...)` | ✅ |
| `handler.go` | HTTP handlers, request/response types | ✅ |
| `scheduler.go` | Cron job definitions + implementation | ⚠️ (kalo ada background job) |
| `migration.go` | Embedded SQL migrations | ⚠️ (kalo butuh tabel baru) |
| `handler_test.go` | Integration tests | ⚠️ (best practice) |

---

## 8. Implementasi — Tahapan

### Fase 1: Framework Registry (Estimasi: 1-2 hari)

1. Bikin `pkg/feature/` — struct, Register(), Start(), validasi dependencies
2. Implementasi baca config dari YAML
3. Endpoint `GET /api/features`
4. POC: pindahin 1 fitur (misal: `uptime`) ke format baru
5. Validasi: `features.uptime: true` → uptime jalan. `features.uptime: false` → sidebar ilang, cron mati

### Fase 2: Migrasi Fitur Existing (Estimasi: per fitur 2-4 jam)

Pindahin fitur satu per satu dari `internal/` ke `pkg/features/`:

```
Urutan:
1. uptime           (POC — udah proven)
2. ssl_monitoring   (sama polanya)
3. compliance       (mungkin paling kompleks)
4. login_activity   (ringan)
5. crowdsec         (butuh external service)
6. container_security
7. trivy
8. netdata
9. secret_scanning
```

### Fase 3: Frontend Adaptasi (Estimasi: 1 hari)

1. Dynamic sidebar — filter by enabled features
2. Route guard — redirect kalo akses disabled feature
3. `/features/disabled` page
4. `+layout.svelte` fetch `/api/features` di onMount

### Fase 4: Advanced (Estimasi: 2-3 hari, opsional)

1. **Per-user features** — user A bisa enable CrowdSec, user B gak
2. **UI-based config** — toggle feature dari Settings page, bukan manual edit YAML
3. **Runtime reload** — gak perlu restart backend pas ubah config
4. **Build tags** — `go build -tags="crowdsec,netdata"` buat binary lebih kecil

---

## 9. Decision Log

### D-1: Always-run migrations (Opsi A)

**Keputusan:** Seluruh migration jalan di startup, gak peduli feature enabled/disable.

**Alasan:**
- Simplicity — gak perlu tracking migration per-feature
- Data safety — disable fitur gak berarti drop data
- PostgreSQL zero-cost untuk tabel kosong

**Kapan bisa di-revisit:** Kalau ada fitur dengan volume data massive yang bikin bloat (puluhan juta rows). Untuk skala Anjungan saat ini, gak relevan.

### D-2: Runtime filter di frontend (bukan tree-shake)

**Keputusan:** Sidebar filter pake JS runtime, bukan SvelteKit build-time tree-shaking.

**Alasan:**
- Build-time tree-shake complex — perlu compile per kombinasi config
- Config bisa berubah tanpa rebuild frontend
- Bundle size masih acceptable untuk SPA

**Konsekuensi:** Komponen fitur yang disable tetap ter-bundle di JS. Trade-off yang acceptable (~KB tambahan vs kompleksitas build pipeline).

### D-3: YAML config (bukan env vars)

**Keputusan:** YAML file sebagai primary config source, env vars sebagai override.

**Alasan:**
- YAML lebih readable untuk konfigurasi kompleks
- Versionable — bisa di-commit ke repo
- Bisa dikelola dari Anjungan UI (future)

### D-4: Core features tidak bisa di-disable

**Keputusan:** Auth, Servers, Containers, Audit Log selalu ON.

**Alasan:**
- Anjungan gak berguna tanpa fitur dasar ini
- Banyak fitur lain depend ke core
- Mencegah user mengunci diri sendiri (misal: disable auth)

---

## Referensi

- [Architecture Overview](architecture.md) — arsitektur umum Anjungan
- [Key Decisions](DECISIONS.md) — keputusan arsitektural lainnya
- [Integration Roadmap](../prd/PRD-integration-roadmap.md) — roadmap integrasi tools eksternal
- `anjungan-development` skill — reference patterns untuk implementasi

---

*Dokumen ini adalah referensi arsitektur untuk sistem feature toggle Anjungan. Update sesuai implementasi aktual.*
