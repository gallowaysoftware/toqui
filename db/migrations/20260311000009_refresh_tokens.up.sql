-- Refresh token rotation: track valid refresh tokens server-side.
-- When a token is used, it's revoked and a new one is issued.
-- If a revoked token is reused, all tokens in the family are revoked (breach detection).
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    jti        TEXT NOT NULL UNIQUE,              -- JWT ID claim (unique per token)
    family     UUID NOT NULL,                     -- Token family (shared across rotations)
    expires_at TIMESTAMPTZ NOT NULL,
    revoked    BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_jti ON refresh_tokens(jti);
CREATE INDEX idx_refresh_tokens_family ON refresh_tokens(family);
