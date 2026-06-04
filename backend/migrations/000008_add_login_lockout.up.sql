ALTER TABLE users
  ADD COLUMN locked_until          TIMESTAMPTZ DEFAULT NULL,
  ADD COLUMN failed_login_attempts INT         DEFAULT 0;
