-- Trip notes field for freeform user annotations
ALTER TABLE trips ADD COLUMN notes TEXT;

-- Expand valid booking types with ferry, bus, cruise, transfer
-- (No schema change needed — validBookingTypes map in Go code handles this)
