-- Waitlist: capacity cap for early access
CREATE TABLE waitlist (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    invite_code TEXT UNIQUE,
    signed_up_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    invited_at TIMESTAMPTZ,
    accepted_at TIMESTAMPTZ
);
CREATE INDEX idx_waitlist_email ON waitlist(email);
CREATE INDEX idx_waitlist_invite_code ON waitlist(invite_code) WHERE invite_code IS NOT NULL;

-- Daily usage tracking for per-user rate limits
CREATE TABLE daily_usage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    date DATE NOT NULL DEFAULT CURRENT_DATE,
    message_count INT NOT NULL DEFAULT 0,
    ai_cost_cents INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, date)
);
CREATE INDEX idx_daily_usage_user_date ON daily_usage(user_id, date);
