ALTER TABLE brute_force_config
  DROP COLUMN IF EXISTS threshold,
  DROP COLUMN IF EXISTS window_minutes;
