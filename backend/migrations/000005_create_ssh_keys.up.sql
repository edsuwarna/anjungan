-- Create SSH keys management table
CREATE TABLE IF NOT EXISTS ssh_keys (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255) NOT NULL,
    key_type    VARCHAR(50) NOT NULL DEFAULT 'ed25519', -- ed25519, rsa, ecdsa
    private_key TEXT NOT NULL,
    public_key  TEXT DEFAULT '',
    fingerprint VARCHAR(255) DEFAULT '',
    created_by  UUID REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Add ssh_key_id reference to servers table
ALTER TABLE servers ADD COLUMN IF NOT EXISTS ssh_key_id UUID REFERENCES ssh_keys(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_servers_ssh_key_id ON servers(ssh_key_id);
CREATE INDEX IF NOT EXISTS idx_ssh_keys_created_by ON ssh_keys(created_by);
