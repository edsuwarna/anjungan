---
title: Compliance & Security
description: Security scanning — CIS benchmarks, Lynis audits, and container image vulnerability scanning.
---

# Compliance & Security Scanning

Anjungan provides three types of security scanning for managed servers.

## 1. CIS Benchmark Checks

Checks are organized by category, mapped to CIS (Center for Internet Security) benchmarks:

| Category | Coverage | Example Checks |
|----------|----------|----------------|
| **kernel** | Kernel security | `kernel.kptr_restrict`, `randomize_va_space`, `dmesg_restrict` |
| **network** | Network security | Reverse path filtering, IPv4 forwarding, ICMP redirects, SYN cookies |
| **users** | User/group security | Empty password check, root UID, sudoers config |
| **ssh** | SSH hardening | Root login disabled, protocol 2, key auth |
| **filesystem** | File system security | `/tmp` noexec/nodev, sticky bit, SUID audit |
| **services** | Service hardening | Unnecessary services disabled |
| **logging** | Audit & logging | Auditd running, rsyslog, log permissions |
| **docker** | Docker daemon security | TLS verification, live-restore, user namespace remap |

### Available Profiles

| Profile | Query Param | Scope |
|---------|-------------|-------|
| **All Checks** | `profile=all` (default) | Full audit of all 8+ categories |
| **CIS Level 1** | `profile=cis_level_1` | CIS Level 1 recommendations |
| **CIS Level 2** | `profile=cis_level_2` | CIS Level 1 + 2 recommendations |
| **CIS Docker** | `profile=cis_docker` | Docker daemon security only |

## 2. Lynis Audit

[Lynis](https://cisofy.com/lynis/) is an open-source security auditing tool. Anjungan runs Lynis remotely via SSH and parses the results.

**Output includes:**
- **Hardening score** (0–100) — overall security posture
- **Warnings** — critical issues requiring attention
- **Suggestions** — configuration improvements
- **Category breakdown** — per-area test results

## 3. Container Image Scanning

Container images on managed servers can be scanned for vulnerabilities using Trivy.

- Per-container and bulk scanning
- Results stored per scan with findings history

## Scoring System

Each scan produces a **compliance score** (0–100):

| Status | Score |
|--------|-------|
| ✅ Compliant (green) | ≥ 90 (configurable) |
| ⚠️ Warning (yellow) | 70–89 (configurable) |
| 🔴 Critical (red) | < 70 (configurable) |

### Configuring Thresholds

Thresholds are dynamic and stored in the database:

```bash
# Get current thresholds
curl -H "Authorization: Bearer ***" \
  https://your-instance/api/v1/settings/compliance-thresholds

# Update thresholds
curl -X PUT -H "Authorization: Bearer ***" \
  -H "Content-Type: application/json" \
  -d '{"compliant": 90, "warning": 70}' \
  https://your-instance/api/v1/settings/compliance-thresholds
```

**Validation:** `compliant` must be > `warning` > 0. Changes take effect immediately.

## Scan Lifecycle

1. **Trigger** — scan request returns immediately with `scan_id` + `status: "running"`
2. **Background execution** — scan runs asynchronously via SSH
3. **Completion** — status updates to `completed`, score and findings saved
4. **Viewing** — poll `compliance/{serverID}/latest` or view scan history

## Dashboard UI

The compliance dashboard shows:
- **Overview** — aggregated compliance scores across all servers
- **Server-level cards** — score, scan type, last scan time
- **Detail page** — full finding list by category, remediation guidance
- **History** — scan history per server with trend tracking
