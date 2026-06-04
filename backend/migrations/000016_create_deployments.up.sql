CREATE TABLE IF NOT EXISTS deployments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(200) NOT NULL,
    environment_id  UUID REFERENCES environments(id) ON DELETE SET NULL,
    repo_provider   VARCHAR(20) NOT NULL DEFAULT '',
    repo_owner      VARCHAR(100) NOT NULL DEFAULT '',
    repo_name       VARCHAR(100) NOT NULL DEFAULT '',
    branch          VARCHAR(200) NOT NULL DEFAULT '',
    commit_sha      VARCHAR(40) DEFAULT '',
    server_id       UUID REFERENCES servers(id) ON DELETE SET NULL,
    service_name    VARCHAR(200) DEFAULT '',
    image           VARCHAR(500) DEFAULT '',
    status          VARCHAR(20) NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','deploying','running','success','failed','rolled_back')),
    deployed_by     UUID REFERENCES users(id) ON DELETE SET NULL,
    deployed_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    rollback_from   UUID REFERENCES deployments(id) ON DELETE SET NULL
);

CREATE INDEX idx_deployments_env ON deployments(environment_id);
CREATE INDEX idx_deployments_server ON deployments(server_id);
CREATE INDEX idx_deployments_repo ON deployments(repo_provider, repo_owner, repo_name);
CREATE INDEX idx_deployments_status ON deployments(status);
