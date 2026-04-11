DROP INDEX IF EXISTS idx_trips_search;

ALTER TABLE trips DROP COLUMN IF EXISTS search_vector;
