---
title: Notification Targets
description: Multi-platform alerting for SSL monitoring, uptime checks, and brute force detection — Telegram, Discord, Slack, and webhooks.
---

# Notification Targets

Anjungan provides a shared notification system used by multiple features:
- **SSL Monitoring** — cert expiry alerts
- **Uptime Monitoring** — incident open/resolve notifications
- **Brute Force Detection** — security event alerts

## Supported Platforms

| Platform | Features | Use Case |
|----------|----------|----------|
| **Telegram** | Bot messages with markdown formatting, auto-thread support | General team alerts |
| **Discord** | Webhook embeds with colored status indicators | DevOps channels |
| **Slack** | Webhook messages with attachment formatting | Corporate/enterprise |
| **Generic Webhook** | JSON POST to any URL | Custom integrations, PagerDuty, OpsGenie |

## Creating a Notification Target

### Via Dashboard

1. Navigate to **Notifications** in the sidebar
2. Click **Add Target**
3. Select the platform and fill in the config
4. Click **Test** to verify the connection

### Via API

#### Telegram

```bash
curl -X POST -H "Authorization: Bearer ***" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Team Alerts",
    "platform": "telegram",
    "config": {
      "bot_token": "123456:ABC-DEF1234",
      "chat_id": "-1001234567890"
    },
    "enabled": true
  }' \
  https://your-instance/api/v1/notification-targets
```

#### Discord

```bash
curl -X POST -H "Authorization: Bearer ***" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "DevOps Channel",
    "platform": "discord",
    "config": {
      "webhook_url": "https://discord.com/api/webhooks/.../..."
    },
    "enabled": true
  }' \
  https://your-instance/api/v1/notification-targets
```

#### Slack

```bash
curl -X POST -H "Authorization: Bearer ***" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "#ops-alerts",
    "platform": "slack",
    "config": {
      "webhook_url": "https://hooks.slack.com/services/..."
    },
    "enabled": true
  }' \
  https://your-instance/api/v1/notification-targets
```

#### Generic Webhook

```bash
curl -X POST -H "Authorization: Bearer ***" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "PagerDuty",
    "platform": "generic",
    "config": {
      "url": "https://events.pagerduty.com/v2/enqueue",
      "headers": {
        "Authorization": "Token token=..."
      }
    },
    "enabled": true
  }' \
  https://your-instance/api/v1/notification-targets
```

## Testing Delivery

### Via Dashboard

Every notification target has a **Test** button that sends a sample alert to verify the configuration.

### Via API

```bash
curl -X POST -H "Authorization: Bearer ***" \
  https://your-instance/api/v1/notification-targets/{id}/test
```

## Managing Targets

### List All
```
GET /api/v1/notification-targets
```

### Get Detail
```
GET /api/v1/notification-targets/{id}
```

### Update
```
PUT /api/v1/notification-targets/{id}
```
Send only the fields you want to change (partial update).

### Delete
```
DELETE /api/v1/notification-targets/{id}
```

## Assigning Targets to Features

### SSL Monitoring
When creating or editing an SSL monitor, add target IDs to `webhook_ids`:
```json
{
  "domain": "app.example.com",
  "webhook_ids": ["nt_xxx", "nt_yyy"],
  "notify_before": "14d"
}
```

### Uptime Monitoring
When creating or editing an uptime monitor:
```json
{
  "name": "My App",
  "url": "https://app.example.com/health",
  "notification_target_ids": ["nt_xxx", "nt_yyy"]
}
```

### Brute Force Detection (Admin)
Configure which targets receive brute force alerts:
```
PUT /api/v1/auth-activity/config
```
```json
{
  "notification_target_ids": ["nt_xxx", "nt_yyy"],
  "threshold": 20,
  "window_minutes": 5
}
```

## Formatting & Behavior

| Platform | Format | Message Content |
|----------|--------|-----------------|
| **Telegram** | Markdown | Feature name in bold + details per line |
| **Discord** | Embed | Colored sidebar (red=error, yellow=warning, green=ok) |
| **Slack** | Attachment | Similar to Discord — colored attachment |
| **Generic** | JSON POST | Full event payload in JSON body |

### Telegram Bot Setup

To create a Telegram bot for notifications:

1. Open Telegram and search for [@BotFather](https://t.me/BotFather)
2. Send `/newbot` and follow the prompts
3. Copy the bot token (format: `123456:ABC-DEF1234`)
4. Add the bot to your group chat
5. Send any message in the group
6. Visit `https://api.telegram.org/bot<YOUR_TOKEN>/getUpdates`
7. Find your `chat_id` from the response (for groups, it starts with `-100`)
8. Use both values in the notification target config

## Troubleshooting

### Test notification fails

- **Telegram:** Verify the bot token is correct and the bot is in the chat group
- **Discord/Slack:** Check the webhook URL hasn't been revoked
- **Generic:** Ensure the endpoint is reachable and accepts POST requests
- Check the Anjungan backend logs for delivery error details

### Notifications not sent during incidents

- Verify the feature (SSL/Uptime) has notification targets assigned
- Check that the notification target is enabled
- SSL: verify `notify_before` is set to a reasonable value (e.g., `14d`)
- Uptime: maintenance windows suppress notifications
- Brute force: verify the threshold and window are reasonable for your traffic
