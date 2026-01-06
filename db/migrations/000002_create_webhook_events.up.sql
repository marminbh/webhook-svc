CREATE TABLE IF NOT EXISTS webhook_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_config_id UUID NOT NULL REFERENCES webhook_config(id),
    resource_id TEXT NOT NULL,
    resource_url TEXT NOT NULL,
    tenant_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    event_timestamp TIMESTAMPTZ NOT NULL,
    attempt_count INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 8,
    status TEXT NOT NULL DEFAULT 'pending', -- pending | queued | processing | succeeded | failed | paused
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_error TEXT,
    queued_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Index on webhook_config_id for joins
CREATE INDEX IF NOT EXISTS idx_webhook_events_webhook_config ON webhook_events(webhook_config_id);

-- Composite index for efficient sorted listing by created_at DESC
CREATE INDEX IF NOT EXISTS idx_webhook_events_config_created_at ON webhook_events(webhook_config_id, created_at DESC);

-- Index on tenant_id for direct filtering
CREATE INDEX IF NOT EXISTS idx_webhook_events_tenant_id ON webhook_events(tenant_id);

-- Partial index on next_attempt_at for pending events (used by worker to find events ready for delivery)
CREATE INDEX IF NOT EXISTS idx_webhook_events_next_pending ON webhook_events(next_attempt_at) WHERE status = 'pending';
