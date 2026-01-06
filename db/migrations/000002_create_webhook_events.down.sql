-- Drop indexes before dropping the table
DROP INDEX IF EXISTS idx_webhook_events_next_pending;
DROP INDEX IF EXISTS idx_webhook_events_tenant_id;
DROP INDEX IF EXISTS idx_webhook_events_config_created_at;
DROP INDEX IF EXISTS idx_webhook_events_webhook_config;

DROP TABLE IF EXISTS webhook_events;
