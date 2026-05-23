ALTER TABLE itinerary_items ADD COLUMN booking_id UUID REFERENCES bookings(id) ON DELETE SET NULL;
CREATE INDEX idx_itinerary_items_booking ON itinerary_items(booking_id) WHERE booking_id IS NOT NULL;
