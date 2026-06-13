CREATE TABLE brute_force_config (
    id              TEXT PRIMARY KEY DEFAULT 'default',
    notification_target_ids TEXT[] NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed default config row
INSERT INTO brute_force_config (id, notification_target_ids)
VALUES ('default', '{}')
ON CONFLICT (id) DO NOTHING;
