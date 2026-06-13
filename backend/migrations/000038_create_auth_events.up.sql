CREATE TABLE auth_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID REFERENCES users(id) ON DELETE SET NULL,
    email           TEXT NOT NULL,
    event_type      TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'success',
    failure_reason  TEXT NOT NULL DEFAULT '',
    ip_address      TEXT NOT NULL DEFAULT '',
    user_agent      TEXT NOT NULL DEFAULT '',
    country         TEXT NOT NULL DEFAULT '',
    asn             TEXT NOT NULL DEFAULT '',
    isp             TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_auth_events_user_id ON auth_events(user_id);
CREATE INDEX idx_auth_events_event_type ON auth_events(event_type);
CREATE INDEX idx_auth_events_created_at ON auth_events(created_at);
CREATE INDEX idx_auth_events_ip_address ON auth_events(ip_address);
CREATE INDEX idx_auth_events_status ON auth_events(status);
