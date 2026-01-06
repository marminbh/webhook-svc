-- Revert customer_id back to UUID
-- Note: This will fail if there are non-UUID values in customer_id

-- Drop the index
DROP INDEX IF EXISTS idx_webhook_config_customer_id;

-- Alter the column type back to UUID
ALTER TABLE webhook_config 
ALTER COLUMN customer_id TYPE UUID USING customer_id::UUID;

-- Recreate the index
CREATE INDEX IF NOT EXISTS idx_webhook_config_customer_id ON webhook_config(customer_id) WHERE active = true;

