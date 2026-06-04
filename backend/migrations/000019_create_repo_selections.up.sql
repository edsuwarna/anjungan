CREATE TABLE IF NOT EXISTS repo_selections (
    id         UUID PRIMARY KEY,
    user_id    UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider   VARCHAR(50)  NOT NULL,
    owner      VARCHAR(255) NOT NULL,
    repo_name  VARCHAR(255) NOT NULL,
    selected   BOOLEAN      NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, provider, owner, repo_name)
);
CREATE INDEX IF NOT EXISTS idx_repo_selections_user ON repo_selections(user_id);
