CREATE TABLE IF NOT EXISTS repo_connections (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider        VARCHAR(20) NOT NULL CHECK (provider IN ('github', 'forgejo')),
    label           VARCHAR(100) DEFAULT '',
    base_url        VARCHAR(255) DEFAULT '',
    token_encrypted TEXT NOT NULL DEFAULT '',
    is_active       BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_repo_connections_user ON repo_connections(user_id);
