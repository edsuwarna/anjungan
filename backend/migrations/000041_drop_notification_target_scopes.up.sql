-- Remove scopes column from notification_targets (replaced by per-config notification_target_ids)
ALTER TABLE notification_targets DROP COLUMN IF EXISTS scopes;
DROP INDEX IF EXISTS idx_notification_targets_scopes;
