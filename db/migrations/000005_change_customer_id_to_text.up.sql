-- Change customer_id from UUID to TEXT to support MongoDB ObjectIDs
-- This allows storing MongoDB ObjectIDs (24 hex characters) as org_id

-- First, drop the index on customer_id
DROP INDEX IF EXISTS idx_webhook_config_customer_id;

-- Alter the column type from UUID to TEXT
-- Note: This will convert existing UUIDs to their string representation
ALTER TABLE webhook_config 
ALTER COLUMN customer_id TYPE TEXT USING customer_id::TEXT;

-- Recreate the index on customer_id (now TEXT)
CREATE INDEX IF NOT EXISTS idx_webhook_config_customer_id ON webhook_config(customer_id) WHERE active = true;

