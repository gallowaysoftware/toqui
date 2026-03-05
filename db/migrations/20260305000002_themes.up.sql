-- Trip themes: dynamic, expandable list of trip categories
CREATE TABLE themes (
  slug VARCHAR(64) PRIMARY KEY,         -- e.g., "food", "history", "distilleries"
  display_name VARCHAR(128) NOT NULL,   -- e.g., "Food & Cuisine", "History & Culture"
  description TEXT,
  icon VARCHAR(64),                     -- e.g., "utensils", "landmark", "glass-whiskey"
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Many-to-many: trips can have multiple themes
CREATE TABLE trip_themes (
  trip_id UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
  theme_slug VARCHAR(64) NOT NULL REFERENCES themes(slug) ON DELETE CASCADE,
  confidence REAL NOT NULL DEFAULT 1.0,  -- AI confidence score (0-1)
  source VARCHAR(32) NOT NULL DEFAULT 'ai',  -- 'ai' or 'user' (manual tag)
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (trip_id, theme_slug)
);

CREATE INDEX idx_trip_themes_theme ON trip_themes(theme_slug);

-- Add destination_country to trips
ALTER TABLE trips ADD COLUMN destination_country VARCHAR(2);

-- Seed initial themes
INSERT INTO themes (slug, display_name, description, icon) VALUES
  ('food', 'Food & Cuisine', 'Culinary experiences, restaurants, markets, cooking classes', 'utensils'),
  ('history', 'History & Culture', 'Historical sites, museums, cultural landmarks', 'landmark'),
  ('distilleries', 'Distilleries & Spirits', 'Whisky, wine, brewery, and distillery tours', 'wine-glass');
