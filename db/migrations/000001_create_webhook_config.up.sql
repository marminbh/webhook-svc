CREATE TABLE IF NOT EXISTS webhook_config (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL, -- Convert mongodb ObjectId to UUID by adding zeros as prefix
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

-- Index on webhook_config.customer_id for efficient org_id filtering
CREATE INDEX IF NOT EXISTS idx_webhook_config_customer_id ON webhook_config(customer_id) WHERE active = true;
