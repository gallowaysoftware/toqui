-- Add multi-country support for trips that span multiple destinations
-- (e.g. "Greece + Turkey"). The legacy destination_country column is retained
-- as the "primary" country for backward compatibility and continues to drive
-- single-country persona resolution. (#133)
ALTER TABLE trips ADD COLUMN destination_countries TEXT[] NOT NULL DEFAULT '{}';

-- Backfill: copy any existing single-country trips into the new array column.
UPDATE trips
SET destination_countries = ARRAY[destination_country]
WHERE destination_country IS NOT NULL AND destination_country <> '';
