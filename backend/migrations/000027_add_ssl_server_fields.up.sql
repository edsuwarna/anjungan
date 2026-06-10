-- Migration 007: Add server association fields to ssl_monitors

ALTER TABLE ssl_monitors
  ADD COLUMN IF NOT EXISTS server_id UUID REFERENCES servers(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS source_provider VARCHAR(32) NOT NULL DEFAULT 'manual',
  ADD COLUMN IF NOT EXISTS last_crt_lookup TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_ssl_monitors_server_id ON ssl_monitors(server_id);
CREATE INDEX IF NOT EXISTS idx_ssl_monitors_source_provider ON ssl_monitors(source_provider);
