-- name: CountTotalUsers :one
SELECT COUNT(*) FROM users;

-- name: CountTotalTrips :one
SELECT COUNT(*) FROM trips;

-- name: CountSignupsLast7Days :one
SELECT COUNT(*) FROM users WHERE created_at >= CURRENT_DATE - INTERVAL '7 days';

-- name: CountSignupsToday :one
SELECT COUNT(*) FROM users WHERE created_at >= CURRENT_DATE;
