---
title: Uptime Monitoring
description: Monitor service availability with HTTP, TCP, and ping checks — configurable schedules, incident timeline, and maintenance windows.
---

# Uptime Monitoring

Anjungan's uptime monitoring tracks service availability across your infrastructure with configurable health checks, real-time status via SSE, incident auto-grouping, and maintenance window support.

## Check Types

| Type | Description | Use Case |
|------|-------------|----------|
| **HTTP** | Sends a GET request to the URL, checks response status code range + optional body content | Web apps, APIs, health endpoints |
| **TCP** | Checks if a TCP port is open and accepting connections | Databases, Redis, custom services |
| **Ping** | ICMP echo request | Network-level reachability |

### HTTP Check Details

When you create an HTTP monitor, you can configure:

- **Expected status range** — `expected_status_min` / `expected_status_max` (e.g., 200–399 = any success/redirect)
- **Expected body** — if set, the response body must contain this string for the check to pass
- **Timeout** — max time to wait for a response (default: 10s)
- **Interval** — how often to check (default: 60s)

> Note: The expected body check is a simple substring match — the response body just needs to contain the string anywhere.

## Creating a Monitor

### Via Dashboard

1. Navigate to **Uptime** in the sidebar
2. Click **Add Monitor**
3. Fill in the details:
   - **Name** — a friendly label (e.g., "API Production")
   - **URL** — the endpoint to check (for TCP: `tcp://host:port`, for HTTP: `https://...`)
   - **Check Type** — HTTP, TCP, or Ping
   - **Interval** — how often to check
   - **Expected Status** — min/max HTTP status range (HTTP only)
   - **Expected Body** — optional text to match in response (HTTP only)
   - **Notification Targets** — choose which targets to alert when status changes

### Via API

```bash
curl -X POST -H "Authorization: Bearer ***" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "API Production",
    "url": "https://api.example.com/health",
    "check_type": "http",
    "interval_seconds": 60,
    "timeout_seconds": 10,
    "expected_status_min": 200,
    "expected_status_max": 399,
    "expected_body": "ok",
    "enabled": true,
    "notification_target_ids": ["nt_xxx"]
  }' \
  https://your-instance/api/v1/uptime-monitors
```

## Viewing Status

### Dashboard Overview

The uptime page shows:
- **KPI cards** — total monitors, up/down counts, paused monitors
- **Monitor list** — each monitor shows current status (green = up, red = down, yellow = paused), response time, last checked time
- **Real-time updates** — status updates via SSE (no page refresh needed)

### Monitor Detail

Click a monitor to see:
- **Current status** — with last response time and HTTP status code
- **Check history** — paginated list of all checks with timestamps
- **Response time trend** — daily min/avg/max/p95 chart for the last 7/30/90 days
- **Incident timeline** — auto-grouped consecutive failures
- **Maintenance windows** — scheduled downtime

## Incidents

When a monitor fails, Anjungan automatically groups consecutive failures into incidents. This means 50 consecutive failed checks = 1 incident, not 50 alerts.

### Incident States

| State | Description |
|-------|-------------|
| **Open** | Currently failing — started at first failure |
| **Resolved** | Service recovered — ended at first successful check after the failure |

### Viewing Incidents

```
GET /api/v1/uptime-monitors/{id}/incidents
```

Returns a timeline of incidents with start time, end time, duration, and failure count.

### Notifications During Incidents

- **Incident opened** — notification sent when first failure detected
- **Incident resolved** — notification sent when service recovers
- **Frequency** — only one notification per incident open/resolve cycle (no spam)

## Maintenance Windows

Use maintenance windows to suppress alerts during planned downtime (deployments, migrations, maintenance).

### Creating a Window

```
POST /api/v1/uptime-monitors/{id}/maintenance
```

```json
{
  "start_time": "2026-06-20T22:00:00Z",
  "end_time": "2026-06-21T06:00:00Z",
  "reason": "Scheduled database migration"
}
```

### Behavior During Maintenance

- Checks still run (so you can see the data later)
- FAILING checks do NOT trigger notifications
- Monitors show a "maintenance" badge in the UI
- No incident is created for failures during maintenance

## Real-time Events (SSE)

The uptime system broadcasts real-time status updates via Server-Sent Events:

```
GET /api/uptime/events?token=<jwt-token>
```

Example event:
```
event: check_result
data: {"monitor_id": "um_xxx", "status": "up", "response_time_ms": 234, "checked_at": "2026-06-15T10:30:00Z"}

event: monitor_status
data: {"monitor_id": "um_xxx", "status": "down", "previous_status": "up", "changed_at": "..."}
```

The token is your JWT access token passed as a query parameter (SSE/EventSource cannot set HTTP headers).

## Managing Monitors

### Pause / Resume

Temporarily stop checking a monitor without deleting it:

```bash
# Pause
curl -X POST -H "Authorization: Bearer ***" \
  https://your-instance/api/v1/uptime-monitors/{id}/pause

# Resume
curl -X POST -H "Authorization: Bearer ***" \
  https://your-instance/api/v1/uptime-monitors/{id}/resume
```

### Check All

Immediately check every enabled monitor:

```bash
curl -X POST -H "Authorization: Bearer ***" \
  https://your-instance/api/v1/uptime-monitors/check-all
```

### Test Notification

Send a test notification to the monitor's configured targets:

```bash
curl -X POST -H "Authorization: Bearer ***" \
  https://your-instance/api/v1/uptime-monitors/{id}/test-notification
```

## Troubleshooting

### Monitor stuck in "pending" status

Check that:
- The URL is reachable from the Anjungan server
- The check type matches the protocol (use `tcp://host:port` for TCP checks)
- The interval is set correctly (minimum 30 seconds)
- The timeout isn't too short for the endpoint

### False positives

If a monitor is flapping (up/down/up/down):
- Increase the timeout
- Check for rate limiting on the target endpoint
- Use `expected_body` to verify the response contains valid content
- Consider adding a maintenance window during known high-load periods

### Notifications not firing

- Verify the monitor has notification targets assigned
- Check that the notification target is enabled and configured correctly
- Test the notification target from the Notifications page
- Checks during maintenance windows don't trigger notifications
