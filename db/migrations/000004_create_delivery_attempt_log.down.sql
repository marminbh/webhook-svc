-- Drop indexes before dropping the table
DROP INDEX IF EXISTS idx_attempt_created_at;
DROP INDEX IF EXISTS idx_attempt_job;

DROP TABLE IF EXISTS delivery_attempt_log;

