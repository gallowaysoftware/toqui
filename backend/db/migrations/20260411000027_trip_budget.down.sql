ALTER TABLE itinerary_items DROP COLUMN IF EXISTS cost_currency;
ALTER TABLE itinerary_items DROP COLUMN IF EXISTS estimated_cost_cents;
ALTER TABLE trips DROP COLUMN IF EXISTS currency;
ALTER TABLE trips DROP COLUMN IF EXISTS budget_cents;
