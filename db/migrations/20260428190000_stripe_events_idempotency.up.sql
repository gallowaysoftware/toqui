-- Idempotency table for Stripe webhook events.
--
-- Stripe occasionally redelivers events (network blips, our 5xx, queue
-- replay). Without an idempotency record a duplicate `checkout.session.completed`
-- could double-record a payment, double-unlock a trip, or violate a unique
-- constraint downstream. Recording the event ID here lets the webhook
-- handler short-circuit duplicates and (separately) lets us return HTTP
-- 500 on transient processing failures so Stripe will retry, without
-- re-running successfully completed work.
CREATE TABLE stripe_events (
    id          TEXT PRIMARY KEY,
    event_type  TEXT NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    retry_count INT NOT NULL DEFAULT 0,
    last_error  TEXT
);

CREATE INDEX idx_stripe_events_received_at ON stripe_events(received_at);
