CREATE TABLE notification_targets (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    platform TEXT NOT NULL DEFAULT 'generic',
    webhook_secret TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    scopes TEXT[] NOT NULL DEFAULT '{}',
    created_by TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Migrate existing SSL notification targets into shared table
INSERT INTO notification_targets (id, name, url, platform, webhook_secret, enabled, scopes, created_by, created_at, updated_at)
SELECT id, name, url, platform, webhook_secret, enabled, ARRAY['ssl'], created_by, created_at, updated_at
FROM ssl_notification_targets;

CREATE INDEX idx_notification_targets_scopes ON notification_targets USING GIN(scopes);
CREATE INDEX idx_notification_targets_enabled ON notification_targets(enabled);
