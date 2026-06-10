CREATE TABLE uptime_check_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    monitor_id TEXT NOT NULL REFERENCES uptime_monitors(id) ON DELETE CASCADE,
    checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status VARCHAR(20) NOT NULL,
    status_code INTEGER,
    response_time_ms INTEGER,
    error_message TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_uptime_check_history_monitor_time
    ON uptime_check_history(monitor_id, checked_at DESC);
