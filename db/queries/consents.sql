-- name: RecordConsent :one
INSERT INTO user_consents (user_id, consent_type, ip_address, user_agent)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id, consent_type) WHERE withdrawn_at IS NULL
DO UPDATE SET granted_at = NOW(), ip_address = EXCLUDED.ip_address, user_agent = EXCLUDED.user_agent
RETURNING *;

-- name: WithdrawConsent :exec
UPDATE user_consents
SET withdrawn_at = NOW()
WHERE user_id = $1 AND consent_type = $2 AND withdrawn_at IS NULL;

-- name: GetActiveConsents :many
SELECT * FROM user_consents
WHERE user_id = $1 AND withdrawn_at IS NULL
ORDER BY granted_at DESC;

-- name: HasActiveConsent :one
SELECT EXISTS(
    SELECT 1 FROM user_consents
    WHERE user_id = $1 AND consent_type = $2 AND withdrawn_at IS NULL
) AS has_consent;

-- name: HasRequiredConsents :one
-- Returns true when the user has active (non-withdrawn) consents for both
-- 'terms' and 'privacy_policy'. Used to determine if consent_pending should
-- be returned during login flows.
SELECT (
    COUNT(DISTINCT consent_type) FILTER (
        WHERE consent_type IN ('terms', 'privacy_policy') AND withdrawn_at IS NULL
    ) = 2
) AS has_required FROM user_consents
WHERE user_id = $1;
