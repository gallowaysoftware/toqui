ALTER TABLE trips ADD COLUMN search_vector tsvector
  GENERATED ALWAYS AS (
    to_tsvector('english', coalesce(title, '') || ' ' || coalesce(description, '') || ' ' || coalesce(destination_country, ''))
  ) STORED;

CREATE INDEX idx_trips_search ON trips USING GIN(search_vector);
