CREATE TABLE uptime_daily_summary (
    monitor_id TEXT NOT NULL REFERENCES uptime_monitors(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    total_checks INTEGER NOT NULL DEFAULT 0,
    up_count INTEGER NOT NULL DEFAULT 0,
    down_count INTEGER NOT NULL DEFAULT 0,
    avg_response_ms INTEGER,
    min_response_ms INTEGER,
    max_response_ms INTEGER,
    uptime_percent DECIMAL(5,2),
    PRIMARY KEY (monitor_id, date)
);
