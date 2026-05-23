-- Helcim payment checkout sessions (tracks initiated checkouts)
CREATE TABLE helcim_checkout_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    trip_id UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    checkout_token TEXT NOT NULL,
    secret_token TEXT NOT NULL,
    amount_cents INT NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'CAD',
    status VARCHAR(32) NOT NULL DEFAULT 'open',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);
CREATE INDEX idx_helcim_sessions_user ON helcim_checkout_sessions(user_id);
CREATE INDEX idx_helcim_sessions_trip ON helcim_checkout_sessions(user_id, trip_id);
CREATE INDEX idx_helcim_sessions_token ON helcim_checkout_sessions(checkout_token);

-- Helcim payment records (immutable record of successful payments)
CREATE TABLE helcim_payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    trip_id UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    helcim_transaction_id VARCHAR(256) UNIQUE NOT NULL,
    approval_code VARCHAR(64),
    card_token VARCHAR(256),
    amount_cents INT NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'CAD',
    status VARCHAR(32) NOT NULL DEFAULT 'approved',
    response_hash TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_helcim_payments_user ON helcim_payments(user_id);
CREATE INDEX idx_helcim_payments_trip ON helcim_payments(user_id, trip_id);

-- Trip unlock records (denormalized for fast access checks)
CREATE TABLE trip_unlocks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    trip_id UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    payment_id UUID REFERENCES helcim_payments(id) ON DELETE SET NULL,
    source VARCHAR(32) NOT NULL DEFAULT 'purchase',
    unlocked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, trip_id)
);
CREATE INDEX idx_trip_unlocks_user ON trip_unlocks(user_id);
