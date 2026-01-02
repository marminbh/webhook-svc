CREATE TABLE IF NOT EXISTS delivery_attempt_log (
    id BIGSERIAL PRIMARY KEY,
    webhook_event_id UUID NOT NULL REFERENCES webhook_events(id),
    attempt_no INT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ NOT NULL,
    http_status INT,
    latency_ms INT,
    response_summary TEXT,
    request_payload JSONB,
    response_payload TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_attempt_job ON delivery_attempt_log(webhook_event_id);
CREATE INDEX IF NOT EXISTS idx_attempt_created_at ON delivery_attempt_log(created_at);

