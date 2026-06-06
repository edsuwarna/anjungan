# Compliance Page & CIS Docker Improvement Plan

**Date:** 2026-06-06
**Context:** Diskusi dari obrolan — server cards compliance, scan history CIS Docker kosong, container scan vs CIS Docker scan terpisah

---

## Problem Summary

### 1. Server Cards di Compliance Page — Basic
Sekarang pake `server-card-row` layout: left border + icon + name + score (kanan) + severity dots + last scan. Fungsional tapi kurang rich dibanding overview page yg udah pake expandable cards.

### 2. CIS Docker Scan History Kosong — 3 lapis masalah
| Layer | Issue |
|-------|-------|
| **UI** | Ga ada tombol scan CIS Docker di manapun (compliance page, server detail, CIS Docker page) |
| **API** | `api.compliance.scanDocker()` nge-point ke `/scan/docker` yg ga exist — harusnya `/scan?profile=cis_docker` |
| **Data** | Container scan disimpen sebagai `scan_type: "Container Security"`, query global history filter ketat `WHERE scan_type = 'CIS Docker'` — jadi 6 container scan yg udah ada ga pernah muncul |

### 3. CIS Docker Card Hardcoded 0
Di compliance overview, CIS Docker card pake literal `0` buat pass/warn/fail. CIS L1/L2 udah dynamic dari category data. Docker ga ada category loading sama sekali.

---

## Proposed Approach

**Unify Docker security scans** — jangan pisah "Container Security" vs "CIS Docker". Semua Docker-related scan muncul di satu tempat: CIS Docker page.

### Approach detail:

1. **Backend: Unify query** — `ListGlobalScanHistory` kalo `scanType = "CIS Docker"`, query juga `"Container Security"`
2. **Frontend: Fix API route** — `scanDocker` method pake `/scan?profile=cis_docker`
3. **Frontend: Tambah scan buttons** — di compliance page card + CIS Docker detail page
4. **Frontend: Load Docker category data** — compliance overview load Docker categories, ganti hardcoded 0
5. **Frontend: Improve server cards** — bikin lebih rich dengan score badge prominent + expandable quick actions

---

## Step-by-Step Plan

### Phase 1: Backend — Unify Scan Types (1 file)

**File:** `backend/internal/common/db/repository.go`
- `ListGlobalScanHistory()`: Ubah query dari `WHERE scan_type = $1` jadi `WHERE scan_type IN ($1, $2)` kalo `scanType == "CIS Docker"` — include `"Container Security"`
- Adjust count query juga

**File:** `backend/internal/compliance/handler.go`
- Tambahin route: `POST /compliance/{serverID}/scan/docker` — biar cocok sama API client frontend
- Atau: fix di frontend aja, jangan nambah route baru (prefer this — keep backend clean)

### Phase 2: Frontend — Fix API Route (1 file)

**File:** `frontend/src/lib/api.svelte.js`
- Fix `scanDocker`: ubah dari `/scan/docker` jadi `/scan?profile=cis_docker`

### Phase 3: Frontend — Scan Buttons (2 files)

**File:** `frontend/src/routes/compliance/+page.svelte`
- Di CIS Docker card, tambahin "Scan Docker" button yg trigger `api.compliance.scanDocker(serverId)`
- Atau: ubah "Scan All" jadi scan semua profile termasuk docker (atau tambahin dropdown)

**File:** `frontend/src/routes/compliance/cis-docker/+page.svelte`
- Di hero section, tambahin "Run Docker Scan" button yg trigger scan ke server(s)
- Bisa pake select server dropdown + scan button, atau scan semua server

### Phase 4: Frontend — Load Docker Category Data (1 file)

**File:** `frontend/src/routes/compliance/+page.svelte`
- Di `loadCategoryBreakdowns()`, tambahin load Docker categories
- Ganti hardcoded `0` di CIS Docker card jadi real data dari docker categories
- Update `profileScore` derived — currently only combines L1+L2, need per-profile

### Phase 5: Frontend — Improve Server Cards (1 file)

**File:** `frontend/src/routes/compliance/+page.svelte`
- Redesign `server-card-row`:
  - Score jadi badge besar di kiri bareng icon (bukan di kanan kecil)
  - Expandable — munculin per-profile mini stats + quick actions (Scan, View)
  - Lebih mirip overview page server cards

---

## Files Likely to Change

| File | Phase | Change |
|------|-------|--------|
| `backend/internal/common/db/repository.go` | 1 | Unify query WHERE clause |
| `backend/internal/compliance/handler.go` | 1 | (optional) tambah `/scan/docker` route |
| `frontend/src/lib/api.svelte.js` | 2 | Fix `scanDocker` route |
| `frontend/src/routes/compliance/+page.svelte` | 3,4,5 | Scan button, Docker data, server cards |
| `frontend/src/routes/compliance/cis-docker/+page.svelte` | 3 | Scan button |

---

## Validation

1. **Backend**: `GET /compliance/history?scan_type=CIS%20Docker` harus return container scan results yg udah ada (6 scans)
2. **Frontend API**: `api.compliance.scanDocker(serverId)` harus trigger scan dgn profile `cis_docker`
3. **Compliance page**: CIS Docker card nunjukin real numbers (bukan 0)
4. **CIS Docker page**: Scan button muncul, bisa trigger scan
5. **Server cards**: Lebih rich, expandable, ada quick actions

---

## Risks & Tradeoffs

- **Mixing scan types**: Container Security dan CIS Docker punya struktur findings beda (container punya `container_name`, CIS Docker host-level). Tapi buat global history, field yg ditampilin (score, passed, warnings, criticals) sama semua — aman.
- **Scan All behavior**: Sekarang `scanAll()` cuma scan `?profile=all` (default). Kalo mau include Docker, perlu diubah atau bikin terpisah.
- **Docker daemon config checks**: Ini blm pernah di-run. Check definitions ada di `checks_docker.go` tapi scanner-nya beda — pake `h.scanner.Run()` untuk CIS vs `RunContainerScan()` untuk container. Jangan dicampur dulu — fokus di unify data display, bukan execution.

---

## Open Questions

1. Apakah "Scan All" button di compliance page harus include CIS Docker? Atau bikin button terpisah?
2. Server cards: perlu expand semua atau satu-satu? (Overview page pake multi-expand)
3. CIS Docker page scan button: scan semua server atau pilih server dulu?
