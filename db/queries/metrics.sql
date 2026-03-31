-- name: CountTotalUsers :one
SELECT COUNT(*) FROM users;

-- name: CountActiveUsersLast7Days :one
SELECT COUNT(DISTINCT user_id) FROM daily_usage WHERE date >= CURRENT_DATE - INTERVAL '7 days';

-- name: CountTotalTrips :one
SELECT COUNT(*) FROM trips;

-- name: CountMessagesToday :one
SELECT COALESCE(SUM(message_count), 0)::bigint FROM daily_usage WHERE date = CURRENT_DATE;

-- name: CountTripProPurchases :one
SELECT COUNT(*) FROM helcim_payments;

-- name: CountProUsers :one
SELECT COUNT(*) FROM users WHERE subscription_tier = 'pro';

-- name: CountSignupsLast7Days :one
SELECT COUNT(*) FROM users WHERE created_at >= CURRENT_DATE - INTERVAL '7 days';

-- name: CountSignupsToday :one
SELECT COUNT(*) FROM users WHERE created_at >= CURRENT_DATE;
