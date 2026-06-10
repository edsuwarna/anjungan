DROP INDEX IF EXISTS idx_notification_targets_project;
DROP INDEX IF EXISTS idx_environments_project;
DROP INDEX IF EXISTS idx_deployments_project;
DROP INDEX IF EXISTS idx_uptime_monitors_project;
DROP INDEX IF EXISTS idx_ssl_monitors_project;
DROP INDEX IF EXISTS idx_servers_project;

ALTER TABLE notification_targets DROP COLUMN project_id;
ALTER TABLE environments DROP COLUMN project_id;
ALTER TABLE deployments DROP COLUMN project_id;
ALTER TABLE uptime_monitors DROP COLUMN project_id;
ALTER TABLE ssl_monitors DROP COLUMN project_id;
ALTER TABLE servers DROP COLUMN project_id;

DELETE FROM projects WHERE id = '00000000-0000-0000-0000-000000000001';
DROP TABLE IF EXISTS project_members;
DROP TABLE IF EXISTS projects;
