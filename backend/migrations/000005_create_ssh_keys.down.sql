DROP INDEX IF EXISTS idx_ssh_keys_created_by;
DROP INDEX IF EXISTS idx_servers_ssh_key_id;
ALTER TABLE servers DROP COLUMN IF EXISTS ssh_key_id;
DROP TABLE IF EXISTS ssh_keys;
