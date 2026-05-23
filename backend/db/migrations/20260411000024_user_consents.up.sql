CREATE TABLE user_consents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    consent_type VARCHAR(64) NOT NULL,  -- 'terms', 'privacy_policy', 'analytics'
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    withdrawn_at TIMESTAMPTZ,
    ip_address VARCHAR(45),  -- IPv6 max length
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Only one active (non-withdrawn) consent per user per type
CREATE UNIQUE INDEX idx_user_consents_active ON user_consents(user_id, consent_type) WHERE withdrawn_at IS NULL;

-- Fast lookups by user
CREATE INDEX idx_user_consents_user_id ON user_consents(user_id);
