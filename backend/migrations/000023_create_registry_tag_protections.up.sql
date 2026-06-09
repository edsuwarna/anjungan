CREATE TABLE IF NOT EXISTS registry_tag_protections (
    id         TEXT PRIMARY KEY,
    repo       TEXT NOT NULL,
    tag        TEXT NOT NULL,
    created_by TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(repo, tag)
);

CREATE INDEX IF NOT EXISTS idx_tag_protections_repo ON registry_tag_protections(repo);
