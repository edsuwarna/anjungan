ALTER TABLE brute_force_config
  ADD COLUMN IF NOT EXISTS threshold INT NOT NULL DEFAULT 20,
  ADD COLUMN IF NOT EXISTS window_minutes INT NOT NULL DEFAULT 5;

-- Update existing row to use defaults
UPDATE brute_force_config
SET threshold = 20, window_minutes = 5
WHERE threshold IS NULL OR window_minutes IS NULL;
