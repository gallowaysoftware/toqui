-- name: CreateSubscription :one
INSERT INTO subscriptions (user_id, stripe_customer_id, stripe_subscription_id, tier, status, current_period_start, current_period_end, billing_period)
VALUES (sqlc.arg(user_id), sqlc.arg(stripe_customer_id), sqlc.arg(stripe_subscription_id), sqlc.arg(tier), sqlc.arg(status), sqlc.arg(current_period_start), sqlc.arg(current_period_end), sqlc.arg(billing_period))
ON CONFLICT (user_id) DO UPDATE SET
    stripe_customer_id = EXCLUDED.stripe_customer_id,
    stripe_subscription_id = EXCLUDED.stripe_subscription_id,
    tier = EXCLUDED.tier,
    status = EXCLUDED.status,
    current_period_start = EXCLUDED.current_period_start,
    current_period_end = EXCLUDED.current_period_end,
    billing_period = EXCLUDED.billing_period,
    updated_at = NOW()
RETURNING *;

-- name: GetSubscriptionByUserID :one
SELECT * FROM subscriptions WHERE user_id = sqlc.arg(user_id);

-- name: GetSubscriptionByStripeCustomer :one
SELECT * FROM subscriptions WHERE stripe_customer_id = sqlc.arg(stripe_customer_id);

-- name: GetSubscriptionByStripeSubscriptionID :one
SELECT * FROM subscriptions WHERE stripe_subscription_id = sqlc.arg(stripe_subscription_id);

-- name: UpdateSubscriptionStatus :exec
UPDATE subscriptions SET status = sqlc.arg(status), updated_at = NOW()
WHERE stripe_subscription_id = sqlc.arg(stripe_subscription_id);

-- name: UpdateSubscriptionTier :exec
UPDATE subscriptions SET tier = sqlc.arg(tier), updated_at = NOW()
WHERE stripe_subscription_id = sqlc.arg(stripe_subscription_id);

-- name: UpdateSubscriptionPeriod :exec
UPDATE subscriptions SET
    current_period_start = sqlc.arg(current_period_start),
    current_period_end = sqlc.arg(current_period_end),
    updated_at = NOW()
WHERE stripe_subscription_id = sqlc.arg(stripe_subscription_id);

-- name: SetSubscriptionCancelAtPeriodEnd :exec
UPDATE subscriptions SET cancel_at_period_end = sqlc.arg(cancel_at_period_end), updated_at = NOW()
WHERE stripe_subscription_id = sqlc.arg(stripe_subscription_id);

-- name: UpdateSubscriptionBillingPeriod :exec
UPDATE subscriptions SET billing_period = sqlc.arg(billing_period), updated_at = NOW()
WHERE stripe_subscription_id = sqlc.arg(stripe_subscription_id);

-- name: DeleteSubscriptionByUserID :exec
DELETE FROM subscriptions WHERE user_id = sqlc.arg(user_id);
