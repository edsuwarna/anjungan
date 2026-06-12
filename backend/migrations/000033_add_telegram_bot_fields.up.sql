ALTER TABLE notification_targets ADD COLUMN bot_token TEXT NOT NULL DEFAULT '';
ALTER TABLE notification_targets ADD COLUMN chat_id TEXT NOT NULL DEFAULT '';
