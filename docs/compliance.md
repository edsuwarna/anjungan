# Compliance & Security Scanning

Anjungan provides three types of security scanning for managed servers.

## 1. CIS Benchmark Checks

Checks are organized by category, mapped to CIS (Center for Internet Security) benchmarks:

| Category | Coverage | Example Checks |
|----------|----------|----------------|
| **kernel** | Kernel security | `kernel.kptr_restrict`, `kernel.randomize_va_space`, `kernel.dmesg_restrict` |
| **network** | Network security | Reverse path filtering, IPv4 forwarding disabled, ICMP redirects, SYN cookies, TCP timestamps |
| **users** | User/group security | Empty password check, root UID check, non-root UID range, sudoers config |
| **ssh** | SSH hardening | Root login disabled, protocol 2, password auth vs key auth, port hardening |
| **filesystem** | File system security | `/tmp` noexec/nodev, sticky bit on world-writable dirs, SUID file audit |
| **services** | Service hardening | Unnecessary services disabled, service container checks |
| **logging** | Audit & logging | Auditd running, rsyslog forwarding, log permissions |
| **docker** | Docker daemon security | TLS verification, live-restore, user namespace remap, no raw/dangerous capabilities |

### Available Profiles

| Profile | Query Param | Scope |
|---------|-------------|-------|
| **All Checks** | `profile=all` (default) | Full audit of all 8+ categories |
| **CIS Level 1** | `profile=cis_level_1` | CIS Level 1 recommendations |
| **CIS Level 2** | `profile=cis_level_2` | CIS Level 1 + 2 recommendations |
| **CIS Docker** | `profile=cis_docker` | Docker daemon security only |

## 2. Lynis Audit

[Lynis](https://cisofy.com/lynis/) is an open-source security auditing tool. The platform runs Lynis remotely via SSH and parses the results.

**Output includes:**
- **Hardening score** (0–100) — overall security posture
- **Warnings** — critical issues requiring attention
- **Suggestions** — configuration improvements
- **Category breakdown** — per-area test results (passed, warnings, suggestions)

## 3. Container Image Scanning

Container images on managed servers can be scanned for vulnerabilities.

- Scans trivy on the target host
- Per-container and bulk scanning
- Results stored per scan with findings history

## Scoring System

Each scan produces a **compliance score** (0–100):

- **Compliant (green)** — score ≥ threshold (default: 90)
- **Warning (yellow)** — score between warning threshold and compliant threshold (default: 70–89)
- **Critical (red)** — score < warning threshold (default: < 70)

### Configuring Thresholds

Thresholds are dynamic and stored in the database. Update via API:

```bash
# Get current thresholds
curl -H "Authorization: Bearer <token>" \
  https://your-instance/api/v1/settings/compliance-thresholds

# Update thresholds
curl -X PUT -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"compliant": 90, "warning": 70}' \
  https://your-instance/api/v1/settings/compliance-thresholds
```

**Validation rules:**
- `compliant` must be > `warning` > 0
- Default: compliant=90, warning=70
- Changes take effect immediately (no restart needed)

## Scan Lifecycle

1. **Trigger** — scan request returns immediately with `scan_id` + `status: "running"`
2. **Background execution** — scan runs asynchronously via SSH
3. **Completion** — status updates to `completed`, score and findings saved
4. **Viewing** — poll `compliance/{serverID}/latest` or `compliance/{serverID}/history/{scanID}`

## Dashboard UI

The compliance dashboard shows:

- **Overview** — aggregated compliance scores across all servers with color-coded status
- **Server-level cards** — score, scan type, last scan time, distribution bar
- **Detail page** — full finding list by category, remediation guidance
- **History** — scan history per server with trend tracking
