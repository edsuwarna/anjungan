CREATE TABLE IF NOT EXISTS user_server_groups (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    server_group TEXT NOT NULL,
    PRIMARY KEY (user_id, server_group)
);

CREATE INDEX IF NOT EXISTS idx_user_server_groups_user_id ON user_server_groups(user_id);
CREATE INDEX IF NOT EXISTS idx_user_server_groups_group ON user_server_groups(server_group);
