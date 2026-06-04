ALTER TABLE repo_connections
ADD COLUMN affiliations VARCHAR(100) NOT NULL DEFAULT 'owner,collaborator,organization_member';
