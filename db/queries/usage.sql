-- name: IncrementDailyUsage :one
-- Conditionally increment: only bumps the counter if it's below the limit.
-- Returns the new row. If no row is returned, the limit was already reached.
INSERT INTO daily_usage (user_id, date, message_count)
VALUES (sqlc.arg(user_id), CURRENT_DATE, 1)
ON CONFLICT (user_id, date)
DO UPDATE SET message_count = daily_usage.message_count + 1, updated_at = NOW()
  WHERE daily_usage.message_count < sqlc.arg(max_count)
RETURNING *;

-- name: GetDailyUsage :one
SELECT * FROM daily_usage
WHERE user_id = sqlc.arg(user_id) AND date = sqlc.arg(date);

-- name: RecordAICost :exec
UPDATE daily_usage SET ai_cost_cents = ai_cost_cents + sqlc.arg(cost_cents), updated_at = NOW()
WHERE user_id = sqlc.arg(user_id) AND date = CURRENT_DATE;

-- name: DecrementDailyUsage :exec
UPDATE daily_usage SET message_count = GREATEST(message_count - 1, 0), updated_at = NOW()
WHERE user_id = sqlc.arg(user_id) AND date = CURRENT_DATE;

-- name: CountDailyMessages :one
SELECT COALESCE(SUM(message_count), 0)::bigint FROM daily_usage WHERE date = CURRENT_DATE;

-- name: GetDailyAICostTotal :one
SELECT COALESCE(SUM(ai_cost_cents), 0)::bigint FROM daily_usage WHERE date = CURRENT_DATE;

-- name: GetWeeklyAICostTotal :one
SELECT COALESCE(SUM(ai_cost_cents), 0)::bigint FROM daily_usage WHERE date >= CURRENT_DATE - INTERVAL '7 days';

-- name: GetMonthlyAICostTotal :one
SELECT COALESCE(SUM(ai_cost_cents), 0)::bigint FROM daily_usage WHERE date >= CURRENT_DATE - INTERVAL '30 days';

-- name: GetAICostByTier :many
SELECT
    COALESCE(u.subscription_tier, 'free') AS tier,
    COUNT(DISTINCT du.user_id)::bigint AS user_count,
    COALESCE(SUM(du.ai_cost_cents), 0)::bigint AS total_cents
FROM daily_usage du
JOIN users u ON u.id = du.user_id
WHERE du.date >= CURRENT_DATE - INTERVAL '30 days'
  AND du.ai_cost_cents > 0
GROUP BY COALESCE(u.subscription_tier, 'free');

-- name: GetTopAICostUsers :many
SELECT
    du.user_id,
    u.email,
    COALESCE(SUM(du.ai_cost_cents), 0)::bigint AS total_cents,
    COALESCE(SUM(du.message_count), 0)::bigint AS message_count
FROM daily_usage du
JOIN users u ON u.id = du.user_id
WHERE du.date >= CURRENT_DATE - INTERVAL '30 days'
  AND du.ai_cost_cents > 0
GROUP BY du.user_id, u.email
ORDER BY total_cents DESC
LIMIT 10;
