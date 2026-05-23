DROP INDEX IF EXISTS idx_itinerary_items_booking;
ALTER TABLE itinerary_items DROP COLUMN IF EXISTS booking_id;
