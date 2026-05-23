CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "postgis";

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(256) UNIQUE NOT NULL,
    name VARCHAR(256),
    google_id VARCHAR(256) UNIQUE NOT NULL,
    avatar_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE trips (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(512) NOT NULL,
    description TEXT,
    status VARCHAR(32) NOT NULL DEFAULT 'planning',
    start_date DATE,
    end_date DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_trips_user_id ON trips(user_id);
CREATE INDEX idx_trips_user_status ON trips(user_id, status);

CREATE TABLE itinerary_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    day_number INT,
    order_in_day INT,
    type VARCHAR(32),
    title VARCHAR(512),
    description TEXT,
    location GEOGRAPHY(POINT, 4326),
    start_time TIMESTAMPTZ,
    end_time TIMESTAMPTZ,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_itinerary_items_trip ON itinerary_items(trip_id, day_number, order_in_day);

CREATE TABLE bookings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id UUID REFERENCES trips(id) ON DELETE SET NULL,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type VARCHAR(32) NOT NULL,
    confirmation_code VARCHAR(256),
    provider VARCHAR(256),
    title VARCHAR(512) NOT NULL,
    start_time TIMESTAMPTZ,
    end_time TIMESTAMPTZ,
    location GEOGRAPHY(POINT, 4326),
    address TEXT,
    details_json JSONB DEFAULT '{}',
    raw_source TEXT,
    source VARCHAR(32) NOT NULL DEFAULT 'manual',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_bookings_trip ON bookings(trip_id);
CREATE INDEX idx_bookings_user ON bookings(user_id);
