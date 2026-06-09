CREATE TABLE IF NOT EXISTS ssl_check_history (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ssl_monitor_id  UUID NOT NULL REFERENCES ssl_monitors(id) ON DELETE CASCADE,
    checked_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status          VARCHAR(20) NOT NULL,
    days_remaining  INTEGER,
    cipher_grade    VARCHAR(2),
    tls_version     VARCHAR(20),
    cipher_suite    VARCHAR(100),
    response_time_ms INTEGER,
    issuer          TEXT NOT NULL DEFAULT '',
    subject         TEXT NOT NULL DEFAULT '',
    error_message   TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_ssl_check_history_monitor_id ON ssl_check_history(ssl_monitor_id);
CREATE INDEX IF NOT EXISTS idx_ssl_check_history_checked_at ON ssl_check_history(checked_at);
