ALTER TABLE trips ADD COLUMN budget_cents BIGINT;
ALTER TABLE trips ADD COLUMN currency VARCHAR(3) DEFAULT 'USD';
ALTER TABLE itinerary_items ADD COLUMN estimated_cost_cents BIGINT;
ALTER TABLE itinerary_items ADD COLUMN cost_currency VARCHAR(3);
