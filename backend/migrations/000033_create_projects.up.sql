CREATE TABLE projects (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255) NOT NULL,
    slug        VARCHAR(255) NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    created_by  UUID NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_projects_slug ON projects(slug);

CREATE TABLE project_members (
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       VARCHAR(50) NOT NULL DEFAULT 'developer',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (project_id, user_id)
);
CREATE INDEX idx_project_members_user ON project_members(user_id);

-- Seed Default Project (uses first admin user as creator)
INSERT INTO projects (id, name, slug, description, created_by)
SELECT '00000000-0000-0000-0000-000000000001', 'Default Project', 'default',
       'System default project for legacy resources', id
FROM users ORDER BY created_at LIMIT 1;

-- Add project_id columns to existing resource tables
ALTER TABLE servers ADD COLUMN project_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES projects(id);
ALTER TABLE ssl_monitors ADD COLUMN project_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES projects(id);
ALTER TABLE uptime_monitors ADD COLUMN project_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES projects(id);
ALTER TABLE deployments ADD COLUMN project_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES projects(id);
ALTER TABLE environments ADD COLUMN project_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES projects(id);
ALTER TABLE notification_targets ADD COLUMN project_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES projects(id);

CREATE INDEX idx_servers_project ON servers(project_id);
CREATE INDEX idx_ssl_monitors_project ON ssl_monitors(project_id);
CREATE INDEX idx_uptime_monitors_project ON uptime_monitors(project_id);
CREATE INDEX idx_deployments_project ON deployments(project_id);
CREATE INDEX idx_environments_project ON environments(project_id);
CREATE INDEX idx_notification_targets_project ON notification_targets(project_id);
