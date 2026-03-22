-- name: CreateCheckoutSession :one
INSERT INTO helcim_checkout_sessions (user_id, trip_id, checkout_token, secret_token, amount_cents, currency)
VALUES (sqlc.arg(user_id), sqlc.arg(trip_id), sqlc.arg(checkout_token), sqlc.arg(secret_token), sqlc.arg(amount_cents), sqlc.arg(currency))
RETURNING *;

-- name: GetCheckoutSessionByToken :one
SELECT * FROM helcim_checkout_sessions WHERE checkout_token = sqlc.arg(checkout_token);

-- name: MarkCheckoutSessionComplete :exec
UPDATE helcim_checkout_sessions SET status = 'complete', completed_at = NOW()
WHERE checkout_token = sqlc.arg(checkout_token) AND status = 'open';

-- name: MarkCheckoutSessionExpired :exec
UPDATE helcim_checkout_sessions SET status = 'expired'
WHERE checkout_token = sqlc.arg(checkout_token) AND status = 'open';

-- name: CreatePayment :one
INSERT INTO helcim_payments (user_id, trip_id, helcim_transaction_id, approval_code, card_token, amount_cents, currency, status, response_hash)
VALUES (sqlc.arg(user_id), sqlc.arg(trip_id), sqlc.arg(helcim_transaction_id), sqlc.arg(approval_code), sqlc.arg(card_token), sqlc.arg(amount_cents), sqlc.arg(currency), sqlc.arg(status), sqlc.arg(response_hash))
RETURNING *;

-- name: GetPaymentByTransactionID :one
SELECT * FROM helcim_payments WHERE helcim_transaction_id = sqlc.arg(helcim_transaction_id);

-- name: CreateTripUnlock :one
INSERT INTO trip_unlocks (user_id, trip_id, payment_id, source)
VALUES (sqlc.arg(user_id), sqlc.arg(trip_id), sqlc.arg(payment_id), sqlc.arg(source))
ON CONFLICT (user_id, trip_id) DO UPDATE SET payment_id = EXCLUDED.payment_id, unlocked_at = NOW()
RETURNING *;

-- name: IsTripUnlocked :one
SELECT EXISTS(SELECT 1 FROM trip_unlocks WHERE user_id = sqlc.arg(user_id) AND trip_id = sqlc.arg(trip_id));

-- name: ListUserPayments :many
SELECT p.*, t.title as trip_title
FROM helcim_payments p
JOIN trips t ON t.id = p.trip_id
WHERE p.user_id = sqlc.arg(user_id)
ORDER BY p.created_at DESC
LIMIT sqlc.arg(page_size) OFFSET sqlc.arg(page_offset);

-- name: CountUserPayments :one
SELECT COUNT(*) FROM helcim_payments WHERE user_id = sqlc.arg(user_id);

-- name: ListUserTripUnlocks :many
SELECT u.*, t.title as trip_title
FROM trip_unlocks u
JOIN trips t ON t.id = u.trip_id
WHERE u.user_id = sqlc.arg(user_id)
ORDER BY u.unlocked_at DESC;
