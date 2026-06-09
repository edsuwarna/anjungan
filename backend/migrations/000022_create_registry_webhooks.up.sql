CREATE TABLE IF NOT EXISTS registry_webhooks (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL DEFAULT '',
    url             TEXT NOT NULL,
    platform        TEXT NOT NULL DEFAULT 'generic',  -- telegram, discord, slack, generic
    events          TEXT NOT NULL DEFAULT '["push","pull","delete"]',  -- JSON array
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS registry_webhook_events (
    id              TEXT PRIMARY KEY,
    webhook_id      TEXT REFERENCES registry_webhooks(id) ON DELETE SET NULL,
    event_type      TEXT NOT NULL,  -- push, pull, delete
    repo            TEXT NOT NULL DEFAULT '',
    tag             TEXT NOT NULL DEFAULT '',
    digest          TEXT NOT NULL DEFAULT '',
    actor           TEXT NOT NULL DEFAULT '',
    description     TEXT NOT NULL DEFAULT '',
    payload         JSONB,
    status          TEXT NOT NULL DEFAULT 'pending',  -- pending, delivered, failed
    status_code     INT NOT NULL DEFAULT 0,
    response        TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    delivered_at    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_registry_webhook_events_created ON registry_webhook_events(created_at DESC);
