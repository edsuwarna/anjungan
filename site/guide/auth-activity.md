---
title: Auth Activity & Brute Force Protection
description: Login monitoring, brute force detection, IP blocking, and security event tracking.
---

# Auth Activity & Brute Force Protection

Anjungan provides comprehensive authentication monitoring to detect and respond to suspicious login activity. All features require **admin** privileges.

> These features are accessible via the **Admin → Login Activity** section in the dashboard.

## Dashboard Overview

The Login Activity page provides:

- **Summary Cards** — today's KPIs at a glance: total logins, failures, success rate, lockouts, unique IPs, blocked IPs, active brute force alerts
- **Events Table** — paginated, filterable list of all auth events
- **Trend Chart** — daily success/failure counts over time
- **Heatmap** — hourly distribution of events (see attack patterns by time of day)
- **Top IPs** — IPs with the most failed attempts
- **Top Users** — accounts with the most failed attempts
- **Blocked IPs** — currently blocked IPs with block/unblock actions
- **Lockouts** — currently locked user accounts
- **Brute Force Alerts** — detected attacks with source details
- **Configuration** — threshold and notification settings

## Auth Events

Every authentication action is logged as an event with rich metadata:

| Field | Description |
|-------|-------------|
| `id` | Unique event ID (`aev_xxx`) |
| `email` | User email (if identified) |
| `event_type` | `login_success`, `login_failure`, `logout`, `register`, `lockout`, `totp_setup`, `totp_disable` |
| `status` | `success` or `failure` |
| `failure_reason` | `invalid_password`, `totp_invalid`, `account_locked`, `rate_limited` |
| `ip_address` | Source IP with geolocation (country, ASN, ISP) |
| `user_agent` | Browser/client identifier |
| `auth_method` | `password` or `totp` |
| `created_at` | Timestamp |

### Filtering Events

```
GET /api/v1/auth-activity/events?page=1&limit=50&event_type=login_failure&status=failure&ip_address=185.220.101.23&email=admin@example.com&search=&start_date=2026-06-01&end_date=2026-06-15&sort=created_at&order=desc
```

### CSV Export

For audit purposes, export events as CSV:

```
GET /api/v1/auth-activity/events/export
```

## Brute Force Detection

Anjungan runs a brute force detection scheduler every 60 seconds that analyzes recent failed login attempts.

### How It Works

1. **Configurable threshold + window** — default: 20 failures in 5 minutes = brute force alert
2. **IP-based detection** — counts failures per source IP, across all user accounts
3. **Automatic alerting** — when threshold is exceeded, a security event is created and notifications are sent
4. **Credential stuffing detection** — if > 5 different user accounts are targeted from one IP in the window, the event type changes from `brute_force` to `credential_stuffing`

### Configuration

```
GET  /api/v1/auth-activity/config
PUT  /api/v1/auth-activity/config
```

```json
{
  "threshold": 20,
  "window_minutes": 5,
  "notification_target_ids": ["nt_xxx"]
}
```

| Parameter | Default | Description |
|-----------|---------|-------------|
| `threshold` | 20 | Number of failed attempts to trigger alert |
| `window_minutes` | 5 | Time window (in minutes) for counting attempts |
| `notification_target_ids` | [] | Notification targets to alert on detection |

### Viewing Active Alerts

```
GET /api/v1/auth-activity/brute-force
```

Returns current alerts with: IP address, failure count, affected users, time window, and geolocation data.

## IP Blocking

Block malicious IPs to prevent further login attempts.

### Block an IP

```
POST /api/v1/auth-activity/block-ip
```
```json
{
  "ip_address": "185.220.101.23",
  "reason": "Brute force attack detected"
}
```

### Unblock an IP

```
POST /api/v1/auth-activity/unblock-ip
```
```json
{
  "ip_address": "185.220.101.23"
}
```

### List Blocked IPs

```
GET /api/v1/auth-activity/blocked-ips
```

### How Blocking Works

- Blocked IPs are stored in both PostgreSQL (persistent) and Redis (fast lookup)
- On backend restart, blocked IPs are synced from DB to Redis automatically
- Blocked IPs receive an immediate `403 Forbidden` response on login attempt
- Geolocation data (country, ASN, ISP) is attached to each event for analysis

## Lockouts

User accounts are automatically locked after exceeding the rate limit (configurable: default 5 attempts per 15 minutes, 30-minute lockout).

### View Locked Accounts

```
GET /api/v1/auth-activity/lockouts
```

### Unlock via Admin

From the Admin panel:
```
POST /api/v1/admin/users/{id}/unlock
```

Or use the Lockouts page in the dashboard — click "Unlock" next to the locked account.

## Trend & Analytics

### Daily Trend

```
GET /api/v1/auth-activity/trend?days=7
```

Returns daily aggregated stats for charting: date, total attempts, successes, failures, success rate.

### Top IPs

```
GET /api/v1/auth-activity/top-ips?days=7
```

Returns IPs ranked by failure count — useful for identifying attack sources.

### Top Users

```
GET /api/v1/auth-activity/top-users?days=7
```

Returns user accounts ranked by failure count — useful for spotting compromised accounts or forgotten passwords.

### Hourly Heatmap

```
GET /api/v1/auth-activity/heatmap?days=7
```

Returns hourly distribution showing attack patterns by time of day. Each entry: day of week, hour, event count.

## Security Events

When brute force or credential stuffing is detected, a `SecurityEvent` is persisted with:

- Event type: `brute_force` or `credential_stuffing`
- Source IP with geolocation
- Severity: `high`
- Detailed metadata (failure count, user count, time window, first/last attempt timestamps)

These events appear in the security events view and trigger configured notification targets.

## Best Practices

### Recommended Configuration

| Environment | Threshold | Window | Notes |
|-------------|-----------|--------|-------|
| **Internal/VPN only** | 20 failures | 15 min | Low traffic, fewer false positives |
| **Public-facing** | 10 failures | 5 min | Tighter, more aggressive |
| **High-traffic apps** | 30 failures | 5 min | Avoid false alerts from legitimate traffic |

### Notification Setup

1. Create a Telegram/Discord notification target for security alerts
2. Assign it in the brute force config
3. You'll receive instant alerts with IP, failure count, and geo data

### Regular Review

- Check the **Top IPs** report weekly for unusual patterns
- Review **Lockouts** to ensure no legitimate users are locked out
- Export auth events to CSV for compliance audits
