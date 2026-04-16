-- name: CreateBooking :one
INSERT INTO bookings (user_id, trip_id, type, confirmation_code, provider, title, start_time, end_time, location, address, details_json, raw_source, source, departure_location, arrival_location, num_guests, price_cents, currency, timezone)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
RETURNING *;

-- name: UpdateBooking :one
UPDATE bookings SET
  title = COALESCE(NULLIF(sqlc.arg(title), ''), title),
  type = COALESCE(NULLIF(sqlc.arg(type), ''), type),
  confirmation_code = COALESCE(NULLIF(sqlc.arg(confirmation_code), ''), confirmation_code),
  provider = COALESCE(NULLIF(sqlc.arg(provider), ''), provider),
  start_time = COALESCE(sqlc.arg(start_time), start_time),
  end_time = COALESCE(sqlc.arg(end_time), end_time),
  address = COALESCE(NULLIF(sqlc.arg(address), ''), address),
  departure_location = COALESCE(NULLIF(sqlc.arg(departure_location), ''), departure_location),
  arrival_location = COALESCE(NULLIF(sqlc.arg(arrival_location), ''), arrival_location),
  num_guests = COALESCE(sqlc.arg(num_guests), num_guests),
  price_cents = COALESCE(sqlc.arg(price_cents), price_cents),
  currency = COALESCE(NULLIF(sqlc.arg(currency), ''), currency),
  timezone = COALESCE(NULLIF(sqlc.arg(timezone), ''), timezone),
  updated_at = NOW()
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id)
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

-- name: FindBookingByConfirmationCode :one
SELECT * FROM bookings
WHERE user_id = sqlc.arg(user_id) AND trip_id = sqlc.arg(trip_id)
  AND confirmation_code = sqlc.arg(confirmation_code)
  AND confirmation_code != ''
LIMIT 1;

-- name: DeleteBooking :execrows
DELETE FROM bookings WHERE id = $1 AND user_id = $2;

-- name: SearchBookings :many
SELECT * FROM bookings
WHERE user_id = sqlc.arg(user_id)
  AND (title ILIKE '%' || sqlc.arg(query) || '%' OR provider ILIKE '%' || sqlc.arg(query) || '%' OR confirmation_code ILIKE '%' || sqlc.arg(query) || '%')
ORDER BY created_at DESC
LIMIT sqlc.arg(max_results);

-- name: GetTripCostSummary :many
SELECT
  COALESCE(currency, 'USD') AS currency,
  SUM(price_cents) AS total_cents,
  COUNT(*) AS booking_count
FROM bookings
WHERE trip_id = sqlc.arg(trip_id) AND user_id = sqlc.arg(user_id)
  AND price_cents IS NOT NULL AND price_cents > 0
GROUP BY currency
ORDER BY total_cents DESC;
