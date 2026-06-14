# Anjungan — Key Decisions

> Mengapa kita membangun Anjungan, pilihan arsitektur, dan alasan di balik setiap keputusan.

---

## 🏗️ Keputusan #1: Build vs Buy

### Konteks
Kita butuh platform yang bisa ngelola server (infra) AND jadi Internal Developer Platform (service lifecycle). Ada banyak tools existing: Backstage, Portainer, Kubernetes Dashboard, Rancher, Dokploy, Coolify.

### Keputusan: **Build (Anjungan)**

### Alternatif yang Dipertimbangkan

| Tool | Kenapa Ga Dipake |
|------|-----------------|
| **Backstage** (Spotify) | Terlalu berat — butuh Kubernetes, plugin ecosystem ribet, architecture complex. Overkill untuk team kecil. |
| **Portainer** | Murni container management — ga ada service catalog, ga ada deployment pipeline. Cuma level infra. |
| **Dokploy** | Udah dipake (peladen-central) — tapi hanya Docker Compose deploy, no server management, no observability. |
| **Coolify** | Self-hosted PaaS — mirip Heroku, tapi ga bisa manage bare-metal server, ga ada SSH access. |
| **Rancher / Harvester** | Heavy Kubernetes focus. Kita ga pake K8s. |
| **Serversphere (existing)** | Udah ada server management + SSH + Docker — tapi frontend vanilla JS, ga ada service catalog, perlu redesign total. Better rebuild from scratch. |

### Alasan
1. **Kontrol penuh** — kita bisa custom fitur sesuai kebutuhan, tanpa dependen ke upstream
2. **Integrasi mulus** — server management + IDP dalam satu platform, bukan dua tools terpisah
3. **Ringan** — Go + SvelteKit, ga perlu Kubernetes, jalan di VPS 2GB
4. **Evolusi dari Serversphere** — Anjungan adalah penerus Serversphere dengan arsitektur yang lebih proper + frontend modern
5. **Portabilitas** — bisa jalan di VPS murah, bare-metal, atau cloud

---

## ⚙️ Keputusan #2: Backend Language & Stack

### Konteks
Framework backend untuk REST API yang handle auth, server management, deployment pipeline.

### Keputusan: **Go** (Chi router + pgx database driver + asynq untuk queue)

### Alternatif

| Opsi | Kenapa Ga Dipake |
|------|-----------------|
| **Python / FastAPI** | Udah dipake di Serversphere — terlalu banyak runtime overhead, dependency hell, performance kurang untuk real-time terminal/log streaming. |
| **Go** ✅ | **Alasan pilih**: binary tunggal, performa tinggi (cocok buat SSH/terminal proxy + log streaming), startup instant, memory rendah. Chi router ringan, pgx buat PostgreSQL native. |
| **Rust** | Learning curve tinggi, butuh waktu lebih lama untuk deliver. Ga ada urgent need. |

### Dampak
- Binary kecil (~15MB), deploy tinggal copy + jalanin
- Cocok buat background job (asynq Redis queue)
- Mudah di-containerize

---

## 🎨 Keputusan #3: Frontend Framework

### Konteks
SPA yang interaktif, real-time updates, theme support, fitur dashboard.

### Keputusan: **SvelteKit + Tailwind CSS**

### Alternatif

| Opsi | Kenapa Ga Dipake |
|------|-----------------|
| **React / Next.js** | React bundle size besar, hydration complexity. Untuk dashboard SPA, Svelte lebih ringan. |
| **Vue / Nuxt** | Mirip Svelte dalam simplicity, tapi Svelte punya bundle size lebih kecil + reaktivitas lebih natural. |
| **SvelteKit** ✅ | **Alasan pilih**: bundle kecil, reaktivitas native (no virtual DOM), Tailwind integration seamless, build cepat. Bisa jalan sebagai SPA static. |
| **Vanilla JS (Serversphere)** | Terbukti susah maintain — banyak inline event handler, CSS campur aduk. Butuh framework proper. |

### Lessons Learned (Serversphere)
Serversphere pake vanilla JS → makin besar makin chaotic. Anjungan harus pake framework dari awal.

---

## 🧩 Keputusan #4: Monolith vs Microservices

### Konteks
Aplikasi ini punya banyak domain: auth, server management, containers, deployments, observability.

### Keputusan: **Modular Monolith** (satu binary, banyak package)

### Alasan
1. Tim kecil (satu orang) — microservices cuma nambah complexity
2. Deployment simpel — satu Docker container vs 5-6 container
3. Go package structure udah cukup buat separation of concerns: `internal/auth`, `internal/server`, `internal/deploy`
4. Kalau nanti perlu scaling, Go module boundary tinggal dipisah
5. Performa masih oke — Go concurrent handling bagus

---

## 👥 Keputusan #5: Dual-Role Architecture

### Konteks
Serversphere cuma buat infra engineer. Anjungan harus serving dua persona: infra engineer + developer.

### Keputusan: **Role-based UI + Permission system**

### Detail
- **Infra Engineer** — liat servers, containers, Docker logs, SSH terminal, resource monitoring
- **Developer** — liat services, deployments, environments, health status, logs per service
- **Hybrid** — satu akun bisa punya dua peran
- **UI dinamis** — sidebar, dashboard, notifikasi berubah sesuai role

### Dampak
- Backend RBAC harus granular — bukan cuma admin/user
- Frontend harus aware of permissions — show/hide menu based on roles
- Lebih kompleks dari single-role, tapi lebih valuable

---

## 🟢 Keputusan #6: Brand Identity & Theme

### Konteks
Visual identity yang membedakan Anjungan dari dashboard tools lain.

### Keputusan: **Emerald Green (`#10b981`) sebagai primary color**

### Alasan
1. **Emerald = growth, platform, stability** — cocok untuk platform engineering
2. **Kontras dari tools mainstream** — Portainer biru, Dokploy hitam/ungu, Backstage biru
3. **Mata nyaman** — green tones cocok untuk dipake seharian (dashboard)
4. **Legacy dari Serversphere** — emerald udah jadi ciri khas project sebelumnya

### Implementasi
- Sidebar: dark emerald gradient (bukan putih)
- Primary accent: emerald untuk tombol, active nav, badges
- Dark mode: softer emerald (`#34d399`)
- Surface: off-white (`#f4f5f7`) biar ga terlalu stark

---

## 🔧 Keputusan #7: Reverse Proxy Architecture (Server Access)

### Konteks
Cara Anjungan berkomunikasi dengan managed servers (SSH terminal, Docker API, system metrics).

### Keputusan: **Agent-based reverse proxy** (Anjungan Agent di setiap server target)

### Alternatif

| Opsi | Kenapa Ga Dipake |
|------|-----------------|
| **SSH langsung** | Port harus terbuka, credential management ribet, slow untuk real-time. |
| **VPN Mesh** | Overhead network, complexity tinggi untuk setup. |
| **Agent-based** ✅ | **Alasan pilih**: Agent kecil (Go binary) di install di setiap server, outbound connection ke Anjungan hub. Ga perlu port terbuka. Bisa tunnel SSH + Docker API + metrics sekaligus. |

### Dampak
- Lebih aman (inbound ga perlu port)
- Real-time WebSocket tunnel untuk terminal
- Agent auto-register + auto-update

---

## 📜 Keputusan #8: API-First Design

### Konteks
Frontend butuh API, CLI butuh API, webhook butuh API. Semua harus konsisten.

### Keputusan: **REST API sebagai satu-satunya interface**

### Detail
- Semua fitur harus accessible via API (termasuk deploy, rollback, SSH)
- Frontend cuma API consumer — ga ada logic bisnis di frontend
- API versioning (`/api/v1/...`)
- Swagger/OpenAPI documentation otomatis
- API token untuk CI/CD integration

---

## 🔐 Keputusan #9: Auth & Security Stack

### Konteks
Platform ini handle credential server, Docker socket, deployment — security harus kuat.

### Keputusan
- **JWT (access + refresh token)** — stateless auth, cocok untuk API
- **TOTP 2FA** — opsional per user
- **OIDC / SSO** — Google, GitHub OAuth
- **Granular RBAC** — permission per fitur (bukan cuma admin/user)
- **Audit Log** — semua action tercatat: siapa, apa, kapan, dari mana

---

## 📦 Keputusan #10: Database & Queue

### Konteks
Storage untuk data aplikasi + background job processing.

### Keputusan
- **PostgreSQL** — main database. Relational, mature, fitur lengkap.
- **Redis** — session store + queue backend (asynq)
- **Asynq** — Redis-backed task queue buat background job: deploy, health check, metric collection

### Alternatif
| Opsi | Kenapa Ga Dipake |
|------|-----------------|
| **SQLite** | Ga support concurrent write dari banyak agent |
| **RabbitMQ** | Redis + asynq lebih simple untuk kebutuhan kita, ga perlu broker terpisah |
| **Celery** | Python-only |

---

## Ringkasan

```
Layer              Pilihan
───                ───────
Bahasa             Go (Chi + pgx + asynq)
Frontend           SvelteKit + Tailwind
Database           PostgreSQL + Redis
Arsitektur         Modular monolith
Auth               JWT + TOTP + OIDC
Komunikasi Server  Agent-based reverse proxy
Deployment         Docker Compose
Design System      Emerald green (#10b981)
Target User        Infra Engineer + Developer (dual-role)
```

---

*Setiap keputusan di atas adalah hasil diskusi dan pertimbangan. Bisa berubah seiring waktu — tapi harus ada alasan jelas untuk berubah.*

*Last updated: June 2026*
