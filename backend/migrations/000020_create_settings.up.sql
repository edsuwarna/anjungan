CREATE TABLE IF NOT EXISTS settings (
    key         TEXT PRIMARY KEY,
    value       TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Default compliance thresholds (JSON)
INSERT INTO settings (key, value, description) VALUES
    ('compliance_thresholds', '{"compliant": 90, "warning": 70}', 'Compliance score thresholds: compliant (green) and warning (yellow) minimum percentages. Below warning = critical (red).');

CREATE INDEX IF NOT EXISTS idx_settings_key ON settings(key);
