CREATE TABLE IF NOT EXISTS deployment_history (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deployment_id   UUID NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
    status          VARCHAR(20) NOT NULL,
    message         TEXT DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_deployment_history_deploy ON deployment_history(deployment_id);
CREATE INDEX idx_deployment_history_created ON deployment_history(created_at DESC);
