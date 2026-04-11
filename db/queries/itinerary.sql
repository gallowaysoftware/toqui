-- name: CreateItineraryItem :one
INSERT INTO itinerary_items (trip_id, day_number, order_in_day, type, title, description, location, start_time, end_time, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: ListItineraryItemsByTrip :many
SELECT * FROM itinerary_items
WHERE trip_id = $1
ORDER BY day_number, order_in_day, start_time, id;

-- name: UpdateItineraryItem :one
UPDATE itinerary_items
SET day_number = $2, order_in_day = $3, type = $4, title = $5, description = $6,
    location = $7, start_time = $8, end_time = $9, metadata = $10
WHERE itinerary_items.id = $1
  AND trip_id IN (SELECT trips.id FROM trips WHERE trips.id = itinerary_items.trip_id AND trips.user_id = $11)
RETURNING *;

-- name: DeleteItineraryItem :exec
DELETE FROM itinerary_items
WHERE itinerary_items.id = $1
  AND trip_id IN (SELECT trips.id FROM trips WHERE trips.id = itinerary_items.trip_id AND trips.user_id = $2);

-- name: DeleteItineraryItemsByTrip :exec
DELETE FROM itinerary_items
WHERE trip_id = $1
  AND trip_id IN (SELECT trips.id FROM trips WHERE trips.id = $1 AND trips.user_id = $2);

-- name: CreateItineraryItemFromBooking :one
INSERT INTO itinerary_items (trip_id, day_number, order_in_day, type, title, description, start_time, end_time, booking_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetItineraryItemByBooking :one
SELECT * FROM itinerary_items
WHERE booking_id = $1 AND trip_id = $2
LIMIT 1;

-- name: CloneItineraryItems :exec
INSERT INTO itinerary_items (trip_id, day_number, order_in_day, type, title, description, metadata)
SELECT sqlc.arg(new_trip_id)::uuid, day_number, order_in_day, type, title, description, metadata
FROM itinerary_items
WHERE trip_id = sqlc.arg(source_trip_id)::uuid;
