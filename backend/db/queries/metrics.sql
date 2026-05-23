-- name: CountTotalUsers :one
SELECT COUNT(*) FROM users;

-- name: CountActiveUsersLast7Days :one
SELECT COUNT(DISTINCT user_id) FROM daily_usage WHERE date >= CURRENT_DATE - INTERVAL '7 days';

-- name: CountTotalTrips :one
SELECT COUNT(*) FROM trips;

-- name: CountMessagesToday :one
SELECT COALESCE(SUM(message_count), 0)::bigint FROM daily_usage WHERE date = CURRENT_DATE;

-- name: CountTripProPurchases :one
SELECT COUNT(*) FROM payments;

-- name: CountProUsers :one
SELECT COUNT(*) FROM users WHERE subscription_tier = 'pro';

-- name: CountSignupsLast7Days :one
SELECT COUNT(*) FROM users WHERE created_at >= CURRENT_DATE - INTERVAL '7 days';

-- name: CountSignupsToday :one
SELECT COUNT(*) FROM users WHERE created_at >= CURRENT_DATE;

-- name: CountActiveSubscriptions :one
SELECT COUNT(*) FROM subscriptions WHERE status = 'active';

-- name: GetActiveSubscriptionsByTier :many
SELECT tier, COUNT(*)::bigint AS sub_count
FROM subscriptions
WHERE status = 'active'
GROUP BY tier;

-- name: GetTotalTripProRevenueCents :one
SELECT COALESCE(SUM(amount_cents), 0)::bigint
FROM payments
WHERE status = 'approved';

-- name: GetMonthlyTripProRevenueCents :one
SELECT COALESCE(SUM(amount_cents), 0)::bigint
FROM payments
WHERE status = 'approved' AND created_at >= CURRENT_DATE - INTERVAL '30 days';
