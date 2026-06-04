-- Add extended metadata columns to servers table
ALTER TABLE servers ADD COLUMN IF NOT EXISTS tags          TEXT[] DEFAULT '{}';
ALTER TABLE servers ADD COLUMN IF NOT EXISTS labels        JSONB DEFAULT '{}';
ALTER TABLE servers ADD COLUMN IF NOT EXISTS server_group  VARCHAR(100) DEFAULT '';
ALTER TABLE servers ADD COLUMN IF NOT EXISTS region        VARCHAR(100) DEFAULT '';
ALTER TABLE servers ADD COLUMN IF NOT EXISTS server_type   VARCHAR(100) DEFAULT '';
ALTER TABLE servers ADD COLUMN IF NOT EXISTS description   TEXT DEFAULT '';
ALTER TABLE servers ADD COLUMN IF NOT EXISTS os_info       VARCHAR(255) DEFAULT '';
ALTER TABLE servers ADD COLUMN IF NOT EXISTS cpu_info      VARCHAR(255) DEFAULT '';
ALTER TABLE servers ADD COLUMN IF NOT EXISTS last_seen_at  TIMESTAMPTZ;
ALTER TABLE servers ADD COLUMN IF NOT EXISTS monitoring    BOOLEAN DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_servers_server_group ON servers(server_group);
CREATE INDEX IF NOT EXISTS idx_servers_region ON servers(region);
CREATE INDEX IF NOT EXISTS idx_servers_server_type ON servers(server_type);
CREATE INDEX IF NOT EXISTS idx_servers_last_seen ON servers(last_seen_at);

-- Create server_metrics table for historical data
CREATE TABLE IF NOT EXISTS server_metrics (
    id            BIGSERIAL PRIMARY KEY,
    server_id     UUID NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    cpu_load_1    DOUBLE PRECISION NOT NULL DEFAULT 0,
    cpu_load_5    DOUBLE PRECISION NOT NULL DEFAULT 0,
    cpu_load_15   DOUBLE PRECISION NOT NULL DEFAULT 0,
    mem_used_bytes  BIGINT NOT NULL DEFAULT 0,
    mem_total_bytes BIGINT NOT NULL DEFAULT 0,
    disk_used_bytes  BIGINT NOT NULL DEFAULT 0,
    disk_total_bytes BIGINT NOT NULL DEFAULT 0,
    disk_used_pct    DOUBLE PRECISION NOT NULL DEFAULT 0,
    net_rx_bytes  BIGINT NOT NULL DEFAULT 0,
    net_tx_bytes  BIGINT NOT NULL DEFAULT 0,
    collected_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_server_metrics_server_time ON server_metrics(server_id, collected_at DESC);

-- Create alerts table
CREATE TABLE IF NOT EXISTS alerts (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id     UUID REFERENCES servers(id) ON DELETE CASCADE,
    type          VARCHAR(50) NOT NULL, -- 'disk', 'memory', 'cpu', 'status'
    severity      VARCHAR(20) NOT NULL DEFAULT 'warning', -- 'info', 'warning', 'critical'
    message       TEXT NOT NULL,
    value         TEXT DEFAULT '',
    threshold     TEXT DEFAULT '',
    acknowledged  BOOLEAN DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_alerts_server ON alerts(server_id);
CREATE INDEX IF NOT EXISTS idx_alerts_ack ON alerts(acknowledged, created_at DESC);
