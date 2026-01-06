-- Partial unique index to prevent duplicate events with the same event_type and resource_id
-- that are still in progress (pending, queued, or processing)
-- This allows the same event_type/resource_id to be created again after delivery/failure
CREATE UNIQUE INDEX IF NOT EXISTS idx_webhook_events_unique_pending 
ON webhook_events(event_type, resource_id) 
WHERE status IN ('pending', 'queued', 'processing');

