-- Drop indexes added in up migration
DROP INDEX IF EXISTS idx_webhook_config_customer_id;
DROP INDEX IF EXISTS idx_webhook_events_tenant_id;
DROP INDEX IF EXISTS idx_webhook_events_config_created_at;
DROP INDEX IF EXISTS idx_webhook_events_webhook_config;

