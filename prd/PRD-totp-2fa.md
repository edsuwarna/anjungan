# Anjungan — PRD: TOTP 2FA (Two-Factor Authentication)

> **Version:** 1.0
> **Status:** 🟡 Partially Implemented — schema ✅, login detection ✅, verify endpoint stub ❌
> **Author:** Endang Suwarna
> **Last Updated:** June 9, 2026

---

## 1. Executive Summary

### Problem Statement

Anjungan currently relies solely on **email + password authentication** for all users. This presents several security risks:

| Risk | Impact |
|------|--------|
| **Credential theft** | Stolen password = full access to infra dashboard |
| **Shared passwords** | Users may reuse passwords from other services |
| **Brute force** | Without 2FA, only password complexity and lockout protect against brute force |
| **Admin accounts** | Admin users control servers, deployments, users — single-factor protection is insufficient |
| **Compliance** | Many security frameworks (SOC 2, ISO 27001) require MFA for privileged access |

**TOTP 2FA solves this:**
- **Self-service** — users enable/disable 2FA from settings without admin
- **RFC 6238 compliant** — works with any authenticator app (Google Authenticator, Authy, 1Password, Bitwarden)
- **No SMS/email dependency** — TOTP works offline, no carrier costs
- **Phishing-resistant** — second factor is time-based, tied to device

### Target Audience

| Persona | Pain | Benefit |
|---------|------|---------|
| **Endang (admin)** | Controls all infra; weak password = catastrophe | Extra layer protecting admin actions |
| **Developer** | Reuses passwords | 2FA protects infra even if password is leaked |
| **All users** | No way to secure account | Self-service 2FA from settings page |

### Goals

| Goal | Metric |
|------|--------|
| Reduce account takeover risk | 100% users can enable 2FA |
| Self-service without admin | Enable/disable from settings, no ticket needed |
| Industry standard auth | RFC 6238 TOTP, SHA-1, 30-second window, 6-digit code |
| Zero user friction | QR code scan — no manual secret entry needed |
| Admin oversight | Admin can reset user's 2FA if device is lost |

### Current Status (June 2026)

🟡 **TOTP 2FA is partially implemented.** Database schema has `totp_secret` and `totp_enabled` columns. Login flow already detects TOTP users and returns `totp_required` status. What's missing:

| Component | Status |
|-----------|--------|
| DB schema (`totp_secret`, `totp_enabled`) | ✅ Done |
| Login flow detection (`totp_required` response) | ✅ Done |
| Frontend API client (`setupTOTP`, `verifyTOTPSetup`, `disableTOTP`) | ✅ Done |
| Backend `Verify2FA` endpoint | ❌ Stub (returns "not implemented yet") |
| Backend `SetupTOTP` endpoint | ❌ Missing |
| Backend `VerifyTOTPSetup` endpoint | ❌ Missing |
| Backend `DisableTOTP` endpoint | ❌ Missing |
| TOTP library dependencies | ❌ Missing |
| Frontend login page TOTP step | ❌ Missing |
| Frontend settings page 2FA section | ❌ Missing |
| Admin 2FA reset | ❌ Missing |

---

## 2. Feature Overview

### 2.1 Auth Flow — Login with 2FA

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│  Login Form  │     │  Backend     │     │  Authentica- │
│  (email +    │────►│  Check if    │     │  tor App    │
│   password)  │     │  2FA enabled │     │  (Google,   │
└─────────────┘     └──────┬───────┘     │  Authy, etc)│
                          │               └─────────────┘
                          ▼                      │
                    ┌──────────────┐             │
                    │  totp_required│◄────────────┘
                    │  + temp_token │        Scan QR
                    └──────┬───────┘        (first time)
                           │
                           ▼
                    ┌──────────────┐
                    │  TOTP Input  │
                    │  (6-digit    │
                    │   code)      │
                    └──────┬───────┘
                           │
                           ▼
                    ┌──────────────┐
                    │  Verify TOTP │
                    │  → JWT pair  │
                    │  → Dashboard │
                    └──────────────┘
```

**Step-by-step:**
1. User enters email + password → `POST /auth/login`
2. If user has `totp_enabled=true`, backend returns `{ status: "totp_required", email: "..." }`
3. Frontend shows a 6-digit code input
4. User enters code from authenticator app → `POST /auth/verify-totp`
5. Backend validates TOTP code → returns JWT access + refresh tokens
6. User redirected to dashboard

### 2.2 Setup Flow — Enable 2FA

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│  Settings   │     │  Backend     │     │  Authentica- │
│  Click      │────►│  Generate    │     │  tor App    │
│  "Enable"   │     │  secret + QR │     │             │
└─────────────┘     └──────┬───────┘     └─────────────┘
                           │                      ▲
                    ┌──────▼───────┐              │
                    │  Show QR     │──────────────►
                    │  Code +      │  Scan QR
                    │  Secret Key  │
                    └──────┬───────┘
                           │
                           ▼
                    ┌──────────────┐
                    │  Enter 6-digit│
                    │  code to      │
                    │  verify       │
                    └──────┬───────┘
                           │
                           ▼
                    ┌──────────────┐
                    │  Verify →    │
                    │  totp_enabled│
                    │  = true      │
                    └──────────────┘
```

**Step-by-step:**
1. User goes to Settings → "Two-Factor Authentication" section
2. Clicks "Enable 2FA" → `POST /auth/setup-totp` (authenticated)
3. Backend generates TOTP secret, saves it, returns QR code as base64 PNG + raw secret
4. Frontend displays QR code + manual secret key
5. User scans QR with authenticator app (or enters secret manually)
6. User enters 6-digit code from app → `POST /auth/verify-totp-setup`
7. Backend validates code → sets `totp_enabled=true`
8. Settings page shows "2FA is enabled" with disable option

### 2.3 Disable Flow

- User must provide **current password** to disable 2FA
- `POST /auth/disable-totp` with password in body
- Backend: clear `totp_secret`, set `totp_enabled=false`
- Audit log: record disable action

### 2.4 Admin Reset Flow

- Admin can reset any user's 2FA from Admin Users page (for lost device scenarios)
- `POST /admin/users/{id}/reset-2fa` (admin-only)
- Clears `totp_secret` and sets `totp_enabled=false`
- User must re-setup 2FA on next login
- Audit log: record admin reset action

---

## 3. Technical Specifications

### 3.1 Backend — Dependencies

| Library | Purpose | Version |
|---------|---------|---------|
| `github.com/pquerna/otp` | TOTP generation + validation (RFC 6238) | Latest |
| `github.com/pquerna/otp/totp` | TOTP sub-package | Latest |
| `github.com/skip2/go-qrcode` | QR code generation (PNG, base64) | Latest |

### 3.2 Backend — Endpoints

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| `POST` | `/auth/setup-totp` | JWT | Generate new TOTP secret + QR code. Saves secret but does NOT enable yet |
| `POST` | `/auth/verify-totp-setup` | JWT | Verify 6-digit code to confirm setup. Enables `totp_enabled=true` |
| `POST` | `/auth/disable-totp` | JWT | Disable 2FA. Requires current password in body |
| `POST` | `/auth/verify-totp` | None | Login step 2: validate TOTP code + return JWT pair (replaces current stub) |
| (remove) | ~~`/auth/verify-2fa`~~ | — | Removed — replaced by `/auth/verify-totp` |

### 3.3 Backend — Database

No new migrations needed. Existing `users` table already has:

```sql
totp_secret  TEXT    DEFAULT ''   -- RFC 6238 base32 secret
totp_enabled BOOLEAN DEFAULT false
```

### 3.4 Backend — Repository Methods

| Method | Description |
|--------|-------------|
| `UpdateUserTOTPSecret(ctx, id, secret)` | Update TOTP secret for user |
| `UpdateUserTOTPEnabled(ctx, id, enabled)` | Toggle TOTP enabled state |

### 3.5 Backend — Service Methods

| Method | Description |
|--------|-------------|
| `SetupTOTP(ctx, email)` | Generate key, save secret, return QR + URI |
| `VerifyTOTPSetup(ctx, email, token)` | Validate code, enable 2FA |
| `DisableTOTP(ctx, email, password)` | Validate password, clear secret + disable |
| `VerifyTOTPCode(ctx, email, token)` | Validate code during login, return JWT pair |

### 3.6 Frontend — Pages Modified

| Page | Changes |
|------|---------|
| `/login` | After `totp_required` response: show 6-digit TOTP input + verify button (replaces current form) |
| `/settings` | New "Two-Factor Authentication" section below Change Password: enable/disable UI |

### 3.7 Frontend — API Client

Already exists in `api.svelte.js`:
```javascript
setupTOTP()          // POST /auth/setup-totp
verifyTOTPSetup()    // POST /auth/verify-totp-setup
disableTOTP()        // POST /auth/disable-totp
verifyTOTP()         // POST /auth/verify-totp
```

### 3.8 Security Considerations

| Concern | Mitigation |
|---------|------------|
| **TOTP secret leak** | Secret stored as plaintext in DB — encrypted at rest via full-disk encryption. Consider AES-GCM encryption in future |
| **Brute force TOTP** | Rate limit verify endpoints (max 5 attempts per minute per user) |
| **Disable without password** | Disable requires current password — prevents stolen-session 2FA removal |
| **No backup codes** | Current scope: admin reset for lost device. Backup codes tracked as MEDIUM priority |
| **QR code in transit** | Served over HTTPS, in-memory only (not persisted on server) |
| **Clock drift** | TOTP library handles 30s window with 1-step skew by default |

### 3.9 Audit Events

| Event | Description |
|-------|-------------|
| `auth.2fa_setup` | User initiated 2FA setup (generated QR) |
| `auth.2fa_enable` | User successfully enabled 2FA |
| `auth.2fa_disable` | User disabled 2FA |
| `auth.2fa_reset` | Admin reset user's 2FA |
| `auth.login` | Updated: login event when 2FA is used now logged after TOTP verification |

---

## 4. UI/UX Design

### 4.1 Login Page — TOTP Step

After successful password verification for a 2FA-enabled user:

```
┌──────────────────────────────────┐
│    🔐 Anjungan                   │
│                                  │
│    Email: admin@example.com ✓    │
│                                  │
│    ┌──────────────────────────┐  │
│    │  2-Factor Authentication │  │
│    │                          │  │
│    │  Enter the 6-digit code  │  │
│    │  from your authenticator │  │
│    │                          │  │
│    │    ┌────────────────┐    │  │
│    │    │  _  _  _  _  _  _ │  │
│    │    └────────────────┘    │  │
│    │                          │  │
│    │  ┌────────────────────┐  │  │
│    │  │  Verify 2FA Code   │  │  │
│    │  └────────────────────┘  │  │
│    │                          │  │
│    │  ← Back to login         │  │
│    └──────────────────────────┘  │
└──────────────────────────────────┘
```

**States:**
- **Default:** Centered 6-digit input with tracking-widest spacing
- **Loading:** Button shows "Verifying..." with spinner
- **Error:** Red message below input ("Invalid 2FA code. Try again.")
- **Success:** Redirect to dashboard

### 4.2 Settings Page — 2FA Section

After password section, before admin registration toggle:

```
┌──────────────────────────────────┐
│  👤 User Settings                │
│                                  │
│  ─── Profile ───                │
│  Name: [...] Email: [...]        │
│                                  │
│  ─── Password ───               │
│  Current: [...] New: [...]       │
│                                  │
│  ─── Two-Factor Auth ───        │
│                                  │
│  [disabled state]                │
│  ● 2FA is not enabled            │
│  ┌────────────────────────────┐  │
│  │  Enable Two-Factor Auth    │  │
│  └────────────────────────────┘  │
│                                  │
│  [enabled state]                 │
│  ✓ 2FA is active                 │
│  Security code: XXXX XXXX XXXX   │
│  ┌────────────────────────────┐  │
│  │  Disable 2FA (red)         │  │
│  └────────────────────────────┘  │
│                                  │
│  [setup wizard: QR shown]        │
│  Scan this QR with your          │
│  authenticator app:              │
│  ┌──────────────────────────┐    │
│  │      [QR CODE IMG]       │    │
│  └──────────────────────────┘    │
│  Or enter manually: XXXX-XXXX    │
│                                  │
│  Verify with code: [_ _ _ _ _ _]│
│  ┌────────────────────────────┐  │
│  │  Verify & Enable           │  │
│  └────────────────────────────┘  │
└──────────────────────────────────┘
```

**States:**
- **idle:** Shows "Enable Two-Factor Auth" button
- **setup:** QR code displayed + verify input
- **enabled:** Shows active status + disable button
- **loading:** Button disabled with spinner

### 4.3 Admin Users — 2FA Reset

In the user edit modal or user row, add a "Reset 2FA" button (visible only when user has `totp_enabled=true`):

```
Modal / User Row:
  Name: admin@example.com
  Role: Admin
  2FA:  ✅ Enabled  [Reset 2FA]  ← red button
        ❌ Disabled  (no button)
```

Clicking "Reset 2FA" shows confirmation: "This will disable 2FA for this user. They will need to set it up again on next login."

---

## 5. Non-Functional Requirements

| Requirement | Target |
|-------------|--------|
| **TOTP code validity** | RFC 6238 compliant: SHA-1, 30-second window, 6 digits |
| **Clock skew tolerance** | ±1 step (30s) allowed |
| **QR code format** | Base64 PNG, 256×256px, Medium error correction |
| **Backend response time** | TOTP endpoints < 200ms |
| **Rate limiting** | Max 5 TOTP verify attempts per minute per user |
| **Audit trail** | Every 2FA action logged (setup, enable, disable, reset) |
| **No secrets in transit** | QR code not persisted to disk, only returned in API response |

---

## 6. Implementation Phases

### 🔴 Phase 1 (P0) — Core Implementation — June 2026

| # | Task | Backend/Frontend | Effort |
|---|------|------------------|--------|
| 1 | Add Go dependencies: `pquerna/otp` + `skip2/go-qrcode` | Backend | 15 min |
| 2 | Add repository methods: `UpdateUserTOTPSecret`, `UpdateUserTOTPEnabled` | Backend | 15 min |
| 3 | Add service methods: `SetupTOTP`, `VerifyTOTPSetup`, `DisableTOTP`, `VerifyTOTPCode` | Backend | 1 hr |
| 4 | Add handler endpoints + replace `Verify2FA` stub | Backend | 1 hr |
| 5 | Register new routes, remove old `POST /auth/verify-2fa` | Backend | 15 min |
| 6 | Login page: TOTP challenge step after `totp_required` | Frontend | 1 hr |
| 7 | Settings page: 2FA section (enable/disable with QR) | Frontend | 2 hr |

### 🟡 Phase 2 (P1) — Admin & Polish

| # | Task | Effort |
|---|------|--------|
| 8 | Admin reset 2FA endpoint + UI button | 1 hr |
| 9 | Rate limiting on TOTP verify endpoints | 30 min |
| 10 | Update PRD status → ✅ Done | 15 min |

### 🟢 Phase 3 (P2) — Future

| Task | Priority |
|------|----------|
| Backup/recovery codes (8 one-time codes) | P2 |
| WebAuthn / FIDO2 passkey support | P3 |
| TOTP secret encryption at rest (AES-GCM) | P2 |
| Email notification on 2FA enabled/disabled | P2 |
| Force 2FA for admin users (configurable) | P2 |

---

## 7. Dependencies & Risks

| Dependency | Risk | Mitigation |
|------------|------|------------|
| `pquerna/otp` library | Unmaintained | Last release 2023, but stable API. Can self-host if needed |
| `skip2/go-qrcode` library | Pure Go, may be slow | QR generation < 10ms — acceptable |
| Clock sync on server | TOTP fails if server clock drifts | NTP already configured on VPS |
| User loses device | Locked out of account | Admin reset 2FA flow |
| Frontend QR rendering | base64 PNG may not render in all email clients | Only rendered in web app — no email delivery |

---

## 8. Success Criteria

| Criterion | How to Verify |
|-----------|---------------|
| User can enable 2FA | Go to Settings → Enable → Scan QR → Verify → 2FA active |
| User can login with 2FA | Login with email/password → TOTP input → 6-digit code → Dashboard |
| User can disable 2FA | Settings → Disable → Enter password → 2FA disabled |
| Invalid TOTP rejected | Wrong code → error message, no JWT issued |
| Admin can reset 2FA | Admin Users → Reset 2FA → User's 2FA disabled |
| Audit trail complete | Audit log shows `auth.2fa_setup`, `auth.2fa_enable`, `auth.2fa_disable` events |

---

## 9. Appendix

### 9.1 Comparison: TOTP vs Other 2FA Methods

| Method | Pros | Cons | Verdict |
|--------|------|------|---------|
| **TOTP (this PRD)** | Offline, works globally, no carrier cost, open standard | Requires authenticator app | ✅ Best for self-hosted |
| SMS OTP | Familiar, no app needed | Carrier cost, SIM swap risk, 7-min delay | ❌ |
| Email OTP | No app needed | Requires email access, delay, phishing | ❌ |
| WebAuthn | Phishing-resistant, no code to type | Requires hardware key or platform biometric | 🟢 Future |
| Push notification | Convenient | Requires app + internet | ❌ Overkill |

### 9.2 Glossary

| Term | Definition |
|------|------------|
| **TOTP** | Time-based One-Time Password — RFC 6238 |
| **2FA** | Two-Factor Authentication |
| **MFA** | Multi-Factor Authentication |
| **OTP** | One-Time Password — 6-digit code valid for 30 seconds |
| **Authenticator app** | App that generates TOTP codes (Google Authenticator, Authy, 1Password, Bitwarden) |
| **Provisioning URI** | `otpauth://totp/...` format used by authenticator apps |
| **QR Code** | QR code encoding the provisioning URI for easy scanning |
| **Shared Secret** | Base32-encoded random key shared between server and authenticator app |

### 9.3 References

- [RFC 6238 — TOTP: Time-Based One-Time Password Algorithm](https://datatracker.ietf.org/doc/html/rfc6238)
- [RFC 4226 — HOTP: An HMAC-Based One-Time Password Algorithm](https://datatracker.ietf.org/doc/html/rfc4226)
- [pquerna/otp — Go TOTP library](https://github.com/pquerna/otp)
- [skip2/go-qrcode — Go QR code library](https://github.com/skip2/go-qrcode)
- [PRD.md](./PRD.md) — Main Anjungan PRD
- [Implementation Plan](../.hermes/plans/2026-06-09_3-totp-2fa-implementation.md)
