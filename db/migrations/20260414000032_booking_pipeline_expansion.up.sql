-- Booking pipeline expansion: price tracking, timezone, updated_at
-- Issue #270

-- Price tracking: amount in smallest currency unit (cents/pence)
ALTER TABLE bookings ADD COLUMN price_cents BIGINT;

-- ISO 4217 currency code (e.g. "USD", "EUR", "CAD")
ALTER TABLE bookings ADD COLUMN currency VARCHAR(3);

-- IANA timezone identifier for unambiguous local time display
ALTER TABLE bookings ADD COLUMN timezone VARCHAR(64);

-- Track when bookings were last modified (for UpdateBooking)
ALTER TABLE bookings ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
