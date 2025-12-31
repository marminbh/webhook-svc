CREATE TABLE IF NOT EXISTS webhook_config (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL,
    url TEXT NOT NULL,
    secret TEXT,
    active BOOLEAN DEFAULT TRUE,
    max_concurrency INT DEFAULT 4,
    max_rate_per_min INT DEFAULT 60,
    paused_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_webhook_config_deleted_at ON webhook_config(deleted_at);
