-- name: CreateBooking :one
INSERT INTO bookings (user_id, trip_id, type, confirmation_code, provider, title, start_time, end_time, location, address, details_json, raw_source, source, departure_location, arrival_location, num_guests)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
RETURNING *;

-- name: GetBookingByID :one
SELECT * FROM bookings WHERE id = $1 AND user_id = $2;

-- name: ListBookingsByTrip :many
SELECT * FROM bookings
WHERE trip_id = $1 AND user_id = $2
ORDER BY start_time;

-- name: ListBookingsByUser :many
SELECT * FROM bookings
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: LinkBookingToTrip :one
UPDATE bookings SET trip_id = $2
WHERE id = $1 AND user_id = $3
RETURNING *;

-- name: DeleteBooking :exec
DELETE FROM bookings WHERE id = $1 AND user_id = $2;
