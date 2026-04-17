-- name: CreateItineraryItem :one
INSERT INTO itinerary_items (trip_id, day_number, order_in_day, type, title, description, location, start_time, end_time, metadata, estimated_cost_cents, cost_currency)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: CreateItineraryItemForOwnerOrEditor :one
-- Authz-gated insert used by ReplaceItineraryForOwnerOrEditor (#346):
-- the WHERE clause re-checks ownership on every insert so a collaborator
-- who is demoted mid-transaction (after the outer CanEditTrip pre-check
-- but before the inserts land) cannot sneak new items into someone
-- else's trip. When the predicate fails the INSERT matches zero rows
-- and returns ErrNoRows; the service translates that to
-- trip.ErrNotOwnerOrEditor and rolls back the entire transaction.
--
-- Parameters: $1=trip_id $2=day_number $3=order_in_day $4=type
-- $5=title $6=description $7=location $8=start_time $9=end_time
-- $10=metadata $11=estimated_cost_cents $12=cost_currency $13=user_id
INSERT INTO itinerary_items (trip_id, day_number, order_in_day, type, title, description, location, start_time, end_time, metadata, estimated_cost_cents, cost_currency)
SELECT $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
FROM trips t
WHERE t.id = $1
  AND (
    t.user_id = $13
    OR EXISTS (
      SELECT 1 FROM trip_collaborators tc
      WHERE tc.trip_id = t.id
        AND tc.user_id = $13
        AND tc.accepted_at IS NOT NULL
        AND tc.role = 'editor'
    )
  )
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

-- name: DeleteItineraryItem :execrows
DELETE FROM itinerary_items
WHERE itinerary_items.id = $1
  AND trip_id IN (SELECT trips.id FROM trips WHERE trips.id = itinerary_items.trip_id AND trips.user_id = $2);

-- name: DeleteItineraryItemsByTrip :execrows
DELETE FROM itinerary_items
WHERE trip_id = $1
  AND trip_id IN (SELECT trips.id FROM trips WHERE trips.id = $1 AND trips.user_id = $2);

-- name: CreateItineraryItemFromBooking :one
INSERT INTO itinerary_items (trip_id, day_number, order_in_day, type, title, description, start_time, end_time, booking_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: CreateItineraryItemFromBookingForOwnerOrEditor :one
-- Authz-gated auto-link insert used by
-- BookingHandler.autoLinkBookingToItinerary (#361 P1 defence-in-depth).
-- Even when CreateBookingForOwnerOrEditor has already verified the
-- caller can edit the trip, this re-checks in SQL so any future
-- invocation with a mismatched (caller, trip) pair cannot plant
-- items into a foreign trip.
--
-- Parameters: $1=trip_id $2=day_number $3=order_in_day $4=type
-- $5=title $6=description $7=start_time $8=end_time $9=booking_id
-- $10=caller_user_id
INSERT INTO itinerary_items (trip_id, day_number, order_in_day, type, title, description, start_time, end_time, booking_id)
SELECT $1, $2, $3, $4, $5, $6, $7, $8, $9
FROM trips t
WHERE t.id = $1
  AND (
    t.user_id = $10
    OR EXISTS (
      SELECT 1 FROM trip_collaborators tc
      WHERE tc.trip_id = t.id
        AND tc.user_id = $10
        AND tc.accepted_at IS NOT NULL
        AND tc.role = 'editor'
    )
  )
RETURNING *;

-- name: GetItineraryItemByBooking :one
SELECT * FROM itinerary_items
WHERE booking_id = $1 AND trip_id = $2
LIMIT 1;

-- name: MoveItineraryItem :one
UPDATE itinerary_items
SET day_number = sqlc.arg(day_number), order_in_day = sqlc.arg(order_in_day)
WHERE itinerary_items.id = sqlc.arg(id)
  AND trip_id IN (SELECT trips.id FROM trips WHERE trips.id = itinerary_items.trip_id AND trips.user_id = sqlc.arg(user_id))
RETURNING *;

-- name: GetItineraryItemByID :one
SELECT * FROM itinerary_items
WHERE itinerary_items.id = $1
  AND trip_id IN (SELECT trips.id FROM trips WHERE trips.id = itinerary_items.trip_id AND trips.user_id = $2);

-- name: DeleteItineraryItemByOwnerOrEditor :execrows
DELETE FROM itinerary_items ii
WHERE ii.id = $1
  AND ii.trip_id IN (
    SELECT t.id FROM trips t WHERE t.id = ii.trip_id AND (
      t.user_id = $2
      OR EXISTS (
        SELECT 1 FROM trip_collaborators tc
        WHERE tc.trip_id = t.id AND tc.user_id = $2 AND tc.accepted_at IS NOT NULL AND tc.role = 'editor'
      )
    )
  );

-- name: DeleteItineraryItemsByTripForOwnerOrEditor :execrows
DELETE FROM itinerary_items ii
WHERE ii.trip_id = $1
  AND ii.trip_id IN (
    SELECT t.id FROM trips t WHERE t.id = $1 AND (
      t.user_id = $2
      OR EXISTS (
        SELECT 1 FROM trip_collaborators tc
        WHERE tc.trip_id = t.id AND tc.user_id = $2 AND tc.accepted_at IS NOT NULL AND tc.role = 'editor'
      )
    )
  );

-- name: CloneItineraryItems :exec
INSERT INTO itinerary_items (trip_id, day_number, order_in_day, type, title, description, metadata, estimated_cost_cents, cost_currency)
SELECT sqlc.arg(new_trip_id)::uuid, day_number, order_in_day, type, title, description, metadata, estimated_cost_cents, cost_currency
FROM itinerary_items
WHERE trip_id = sqlc.arg(source_trip_id)::uuid;

-- name: SearchItineraryItems :many
SELECT ii.* FROM itinerary_items ii
JOIN trips t ON t.id = ii.trip_id
WHERE t.user_id = sqlc.arg(user_id)
  AND (ii.title ILIKE '%' || sqlc.arg(query) || '%' OR ii.description ILIKE '%' || sqlc.arg(query) || '%')
ORDER BY ii.created_at DESC
LIMIT sqlc.arg(max_results);
