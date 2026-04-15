-- name: GetRetentionCohorts :many
-- Weekly cohort retention: users who signed up in a given week and were active N days later.
-- "Active" = sent at least one chat message (has a row in daily_usage for that period).
SELECT
  date_trunc('week', u.created_at)::date AS cohort_week,
  COUNT(DISTINCT u.id)::bigint AS cohort_size,
  COUNT(DISTINCT CASE WHEN du.date BETWEEN u.created_at::date AND u.created_at::date + 1 THEN du.user_id END)::bigint AS d1_active,
  COUNT(DISTINCT CASE WHEN du.date BETWEEN u.created_at::date AND u.created_at::date + 7 THEN du.user_id END)::bigint AS d7_active,
  COUNT(DISTINCT CASE WHEN du.date BETWEEN u.created_at::date AND u.created_at::date + 30 THEN du.user_id END)::bigint AS d30_active
FROM users u
LEFT JOIN daily_usage du ON du.user_id = u.id
WHERE u.created_at >= sqlc.arg(since)::timestamptz
GROUP BY cohort_week
ORDER BY cohort_week DESC;

-- name: GetFunnelMetrics :one
-- Conversion funnel: signups -> activated (created a trip) -> engaged (5+ itinerary items) -> paid.
SELECT
  (SELECT COUNT(*) FROM users WHERE created_at >= sqlc.arg(since)::timestamptz)::bigint AS total_signups,
  (SELECT COUNT(DISTINCT user_id) FROM trips WHERE created_at >= sqlc.arg(since)::timestamptz)::bigint AS activated,
  (SELECT COUNT(DISTINCT t.user_id) FROM trips t WHERE t.created_at >= sqlc.arg(since)::timestamptz AND (SELECT COUNT(*) FROM itinerary_items ii WHERE ii.trip_id = t.id) >= 5)::bigint AS engaged,
  (SELECT COUNT(DISTINCT user_id) FROM trip_unlocks WHERE source = 'purchase' AND unlocked_at >= sqlc.arg(since)::timestamptz)::bigint AS paid_trip_pro,
  (SELECT COUNT(DISTINCT user_id) FROM subscriptions WHERE status = 'active' AND created_at >= sqlc.arg(since)::timestamptz)::bigint AS paid_subscription;
