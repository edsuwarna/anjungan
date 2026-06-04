CREATE TABLE scan_results (
    id            UUID PRIMARY KEY,
    server_id     UUID         NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    scan_type     VARCHAR(50)  NOT NULL DEFAULT 'security',
    status        VARCHAR(20)  NOT NULL DEFAULT 'pending',
    score         INT          DEFAULT NULL,
    total_checks  INT          NOT NULL DEFAULT 0,
    passed        INT          NOT NULL DEFAULT 0,
    warnings      INT          NOT NULL DEFAULT 0,
    criticals     INT          NOT NULL DEFAULT 0,
    started_at    TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE scan_findings (
    id            UUID PRIMARY KEY,
    scan_id       UUID          NOT NULL REFERENCES scan_results(id) ON DELETE CASCADE,
    check_id      VARCHAR(100)  NOT NULL,
    category      VARCHAR(50)   NOT NULL,
    severity      VARCHAR(20)   NOT NULL,
    title         TEXT          NOT NULL,
    description   TEXT          DEFAULT '',
    remediation   TEXT          DEFAULT '',
    raw_output    TEXT          DEFAULT '',
    status        VARCHAR(20)   NOT NULL DEFAULT 'fail',
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_scan_results_server    ON scan_results(server_id);
CREATE INDEX idx_scan_results_created   ON scan_results(created_at DESC);
CREATE INDEX idx_scan_findings_scan     ON scan_findings(scan_id);
