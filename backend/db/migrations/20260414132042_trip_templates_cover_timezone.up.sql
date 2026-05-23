-- Trip template flag: marks trips that serve as pre-built templates for cloning
ALTER TABLE trips ADD COLUMN is_template BOOLEAN NOT NULL DEFAULT FALSE;

-- Cover image URL for trip cards (auto-assigned from destination or user-uploaded)
ALTER TABLE trips ADD COLUMN cover_image_url TEXT;

-- IANA timezone for the trip's primary destination (e.g. 'Europe/Athens')
ALTER TABLE trips ADD COLUMN timezone TEXT;

-- Index for efficient template listing (only template trips, ordered by creation)
CREATE INDEX idx_trips_templates ON trips(is_template) WHERE is_template = TRUE;
