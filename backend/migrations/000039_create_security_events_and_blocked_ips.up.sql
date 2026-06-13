CREATE TABLE security_events (
    id          TEXT PRIMARY KEY,
    event_type  TEXT NOT NULL,        -- 'brute_force', 'credential_stuffing'
    ip_address  TEXT NOT NULL DEFAULT '',
    details     JSONB NOT NULL DEFAULT '{}',
    severity    TEXT NOT NULL DEFAULT 'high', -- 'info', 'low', 'medium', 'high', 'critical'
    detected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_security_events_type ON security_events(event_type);
CREATE INDEX idx_security_events_ip ON security_events(ip_address);
CREATE INDEX idx_security_events_detected ON security_events(detected_at DESC);

CREATE TABLE blocked_ips (
    id          TEXT PRIMARY KEY,
    ip_address  TEXT NOT NULL UNIQUE,
    reason      TEXT NOT NULL DEFAULT '',
    created_by  TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_blocked_ips_ip ON blocked_ips(ip_address);
