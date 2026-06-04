CREATE TABLE IF NOT EXISTS environments (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(100) NOT NULL,
    color       VARCHAR(7) NOT NULL DEFAULT '#10b981',
    description TEXT DEFAULT '',
    is_protected BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Default seed environments
INSERT INTO environments (name, color, description, is_protected) VALUES
    ('Production', '#ef4444', 'Production environment — live services', true),
    ('Staging',    '#eab308', 'Staging/pre-production environment', false),
    ('Development','#10b981', 'Development environment', false);
