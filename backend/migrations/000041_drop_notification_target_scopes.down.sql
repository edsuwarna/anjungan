-- Restore scopes column
ALTER TABLE notification_targets ADD COLUMN scopes TEXT[] NOT NULL DEFAULT '{}';
CREATE INDEX idx_notification_targets_scopes ON notification_targets USING GIN(scopes);
