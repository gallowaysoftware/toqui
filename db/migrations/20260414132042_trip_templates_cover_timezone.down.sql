DROP INDEX IF EXISTS idx_trips_templates;
ALTER TABLE trips DROP COLUMN IF EXISTS timezone;
ALTER TABLE trips DROP COLUMN IF EXISTS cover_image_url;
ALTER TABLE trips DROP COLUMN IF EXISTS is_template;
