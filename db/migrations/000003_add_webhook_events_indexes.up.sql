-- Add indexes for efficient querying of webhook_events
-- These indexes support filtering by org_id (via webhook_config) and sorting by created_at

-- Index on webhook_config_id for joins (mentioned in design doc)
CREATE INDEX IF NOT EXISTS idx_webhook_events_webhook_config ON webhook_events(webhook_config_id);

-- Composite index for efficient sorted listing by created_at DESC (recommended in MAT-2043)
CREATE INDEX IF NOT EXISTS idx_webhook_events_config_created_at ON webhook_events(webhook_config_id, created_at DESC);

-- Index on tenant_id for direct filtering (if needed)
CREATE INDEX IF NOT EXISTS idx_webhook_events_tenant_id ON webhook_events(tenant_id);

-- Index on webhook_config.customer_id for efficient org_id filtering
CREATE INDEX IF NOT EXISTS idx_webhook_config_customer_id ON webhook_config(customer_id) WHERE active = true;

