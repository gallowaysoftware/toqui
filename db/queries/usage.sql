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
