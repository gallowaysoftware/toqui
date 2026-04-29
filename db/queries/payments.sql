-- name: CreateCheckoutSession :one
INSERT INTO checkout_sessions (user_id, trip_id, checkout_token, amount_cents, currency)
VALUES (sqlc.arg(user_id), sqlc.arg(trip_id), sqlc.arg(checkout_token), sqlc.arg(amount_cents), sqlc.arg(currency))
RETURNING *;

-- name: GetCheckoutSessionByToken :one
SELECT * FROM checkout_sessions WHERE checkout_token = sqlc.arg(checkout_token);

-- name: MarkCheckoutSessionComplete :exec
UPDATE checkout_sessions SET status = 'complete', completed_at = NOW()
WHERE checkout_token = sqlc.arg(checkout_token) AND status = 'open';

-- name: MarkCheckoutSessionExpired :exec
UPDATE checkout_sessions SET status = 'expired'
WHERE checkout_token = sqlc.arg(checkout_token) AND status = 'open';

-- name: CreatePayment :one
INSERT INTO payments (user_id, trip_id, external_payment_id, amount_cents, currency, status)
VALUES (sqlc.arg(user_id), sqlc.arg(trip_id), sqlc.arg(external_payment_id), sqlc.arg(amount_cents), sqlc.arg(currency), sqlc.arg(status))
RETURNING *;

-- name: GetPaymentByTransactionID :one
SELECT * FROM payments WHERE external_payment_id = sqlc.arg(external_payment_id);

-- name: CreateTripUnlock :one
INSERT INTO trip_unlocks (user_id, trip_id, payment_id, source)
VALUES (sqlc.arg(user_id), sqlc.arg(trip_id), sqlc.arg(payment_id), sqlc.arg(source))
ON CONFLICT (user_id, trip_id) DO UPDATE SET payment_id = EXCLUDED.payment_id, unlocked_at = NOW()
RETURNING *;

-- name: IsTripUnlocked :one
SELECT EXISTS(SELECT 1 FROM trip_unlocks WHERE user_id = sqlc.arg(user_id) AND trip_id = sqlc.arg(trip_id));

-- name: ListUserPayments :many
SELECT p.*, t.title as trip_title
FROM payments p
JOIN trips t ON t.id = p.trip_id
WHERE p.user_id = sqlc.arg(user_id)
ORDER BY p.created_at DESC
LIMIT sqlc.arg(page_size) OFFSET sqlc.arg(page_offset);

-- name: CountUserPayments :one
SELECT COUNT(*) FROM payments WHERE user_id = sqlc.arg(user_id);

-- name: ListUserTripUnlocks :many
SELECT u.*, t.title as trip_title
FROM trip_unlocks u
JOIN trips t ON t.id = u.trip_id
WHERE u.user_id = sqlc.arg(user_id)
ORDER BY u.unlocked_at DESC;

-- name: RecordStripeEvent :one
-- Idempotency record for Stripe webhook events. Returns the row's
-- processed_at to let the handler decide whether to short-circuit.
-- ON CONFLICT bumps retry_count so we can tell repeat retries apart from
-- first-time deliveries; if processed_at IS NOT NULL on conflict, the
-- handler returns 200 immediately without re-running side effects.
INSERT INTO stripe_events (id, event_type)
VALUES (sqlc.arg(id), sqlc.arg(event_type))
ON CONFLICT (id) DO UPDATE SET retry_count = stripe_events.retry_count + 1
RETURNING *;

-- name: MarkStripeEventProcessed :exec
UPDATE stripe_events SET processed_at = NOW(), last_error = NULL
WHERE id = sqlc.arg(id);

-- name: MarkStripeEventFailed :exec
UPDATE stripe_events SET last_error = sqlc.arg(last_error)
WHERE id = sqlc.arg(id);
