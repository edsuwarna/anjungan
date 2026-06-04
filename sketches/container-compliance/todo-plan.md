# Plan: UI Sesuai Mockup — Perubahan yang Dibutuhkan

Berdasarkan mockup Tab A, B, C:

## ⚡ 1. Compliance Dashboard (Tab A)
**File:** `routes/compliance/+page.svelte`

| Sekarang | Harusnya |
|----------|----------|
| KPI: Total Servers, Average Score, Passing, Findings | KPI: Total Servers, **Total Containers**, **Compliance Score**, **Vulnerabilities** |
| Card CIS Docker: — score, 0% progress | Card CIS Docker: **81%** (blue), 128 checks · 104 pass · 16 warn · 8 fail, 6 sections · 18 containers |
| Server rows: ada **CIS** + **Lynis** scan button | Server rows: **NO scan buttons** — pure info. Scan di container page |
| Server rows: cuma server info | Server rows: punya **container sub-rows** nested |
| — | Container sub-row: nama, image, **compliance score**, **Trivy button** |
| Filter: All/Passing/Warning/Critical/Unscanned | Sama ✅ |

### Yang diubah:
- [x] Tambah KPI "Total Containers" (ambil dari kontainer API)
- [ ] Hapus scan button dari server rows (CIS + Lynis)
- [ ] Add container sub-rows di bawah tiap server (pakai data dari containers API)

## ⚡ 2. Container Page
**File:** `routes/containers/+page.svelte`

| Sekarang | Harusnya |
|----------|----------|
| Server cards dengan container **tags** | Server cards dengan container **rows** |
| Klik card → /containers/[id] | Klik container → detail, ada action buttons |
| Gak ada scan trigger | Ada **Scan CIS Docker** button per server |
| Container cuma nama | Container: nama, image, status, uptime, ports |

### Yang diubah:
- [ ] Container tags → rows (name, image, status badge, action icons)
- [ ] Add "Scan CIS Docker" button di header tiap server card
- [ ] Add quick actions: start/stop/restart/logs per container row

## ⚡ 3. CIS Docker Detail Page (Tab B)
**File:** `routes/compliance/cis-docker/+page.svelte`

| Sekarang | Harusnya |
|----------|----------|
| Show check **definitions** from /checks API | Show **scan results** per server/container |
| Hero: profile info + desc | Hero: "Container runtime hardening — {container} ({server})" |
| Stats: — | Stats: 104 Passed, 16 Warning, 8 Failed, 0 N/A, 5.2 Risk Score |
| Filter chips tapi dari definitions | Filter chips dari sections dengan hitungan real (128, 14, 22, etc) |
| Table: definition list | Table: grouped by section dengan status badges (Pass/Fail/Warn) |

### Yang diubah:
- [ ] Ganti data source dari /checks jadi /latest?scan_type=CIS+Docker
- [ ] Hero: tampilin server + container name
- [ ] Stats cards: parse dari hasil scan
- [ ] Filter chips: dinamis dari sections hasil scan
- [ ] Table: grouped by section, per-check status

## ⚡ 4. Trivy Page (Tab C) — Future
Belum ada backend, skip dulu.
