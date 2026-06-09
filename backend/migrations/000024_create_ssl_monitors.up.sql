CREATE TABLE IF NOT EXISTS ssl_monitors (
    id              TEXT PRIMARY KEY,
    domain          TEXT NOT NULL,
    display_name    TEXT NOT NULL DEFAULT '',
    port            INTEGER NOT NULL DEFAULT 443,
    check_interval  TEXT NOT NULL DEFAULT '1h',
    notify_before   TEXT NOT NULL DEFAULT '14d',
    webhook_ids     TEXT[] NOT NULL DEFAULT '{}',

    -- Last check results (updated by TLS check engine)
    last_status     TEXT NOT NULL DEFAULT 'pending',   -- pending, valid, expiring_soon, expired, error
    last_check_at   TIMESTAMPTZ,
    last_error      TEXT NOT NULL DEFAULT '',

    -- Certificate info (updated by TLS check engine)
    issuer          TEXT NOT NULL DEFAULT '',
    subject         TEXT NOT NULL DEFAULT '',
    cert_expires_at TIMESTAMPTZ,
    days_remaining  INTEGER NOT NULL DEFAULT 0,

    -- Enhanced checks: chain validation
    chain_valid     BOOLEAN,
    chain_error     TEXT NOT NULL DEFAULT '',

    -- Enhanced checks: cipher grade (A+ / A / B / C / D / E / F)
    cipher_grade    TEXT NOT NULL DEFAULT '',
    cipher_error    TEXT NOT NULL DEFAULT '',

    -- Enhanced checks: OCSP revocation
    ocsp_status     TEXT NOT NULL DEFAULT '',  -- good, revoked, unknown, error
    ocsp_error      TEXT NOT NULL DEFAULT '',

    -- Enhanced checks: SAN coverage
    san_names       TEXT[] NOT NULL DEFAULT '{}',
    san_mismatch    BOOLEAN NOT NULL DEFAULT FALSE,

    -- Metadata
    created_by      TEXT NOT NULL DEFAULT '',
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(domain, port)
);

CREATE INDEX IF NOT EXISTS idx_ssl_monitors_status ON ssl_monitors(last_status);
CREATE INDEX IF NOT EXISTS idx_ssl_monitors_expires ON ssl_monitors(cert_expires_at) WHERE enabled = TRUE;
