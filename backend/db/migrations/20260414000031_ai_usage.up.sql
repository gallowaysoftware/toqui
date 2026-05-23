-- Per-request AI usage tracking for cost dashboard.
-- Records every AI API call with token counts, model, and cost.
CREATE TABLE ai_usage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,         -- "claude" or "gemini"
    model_tier TEXT NOT NULL,       -- "fast", "smart", "best"
    input_tokens INT NOT NULL DEFAULT 0,
    output_tokens INT NOT NULL DEFAULT 0,
    cost_cents INT NOT NULL DEFAULT 0,
    user_tier TEXT NOT NULL DEFAULT 'free', -- subscription tier at time of request
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_ai_usage_user ON ai_usage(user_id);
CREATE INDEX idx_ai_usage_created ON ai_usage(created_at);
CREATE INDEX idx_ai_usage_user_tier ON ai_usage(user_tier);
