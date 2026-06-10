CREATE TABLE uptime_maintenance_windows (
    id TEXT PRIMARY KEY,
    monitor_id TEXT NOT NULL REFERENCES uptime_monitors(id) ON DELETE CASCADE,
    description TEXT NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    created_by TEXT DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_uptime_mw_monitor_id ON uptime_maintenance_windows(monitor_id);
CREATE INDEX idx_uptime_mw_starts_at ON uptime_maintenance_windows(starts_at);
