CREATE TABLE referrals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    referrer_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    referee_id UUID REFERENCES users(id) ON DELETE SET NULL,
    code VARCHAR(16) NOT NULL UNIQUE,
    redeemed_at TIMESTAMPTZ,
    referrer_reward_granted BOOLEAN NOT NULL DEFAULT false,
    referee_reward_granted BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_referrals_referrer_id ON referrals(referrer_id);
CREATE INDEX idx_referrals_code ON referrals(code);
