-- name: CreateReferral :one
INSERT INTO referrals (referrer_id, code) VALUES ($1, $2)
RETURNING *;

-- name: GetReferralByReferrer :one
SELECT * FROM referrals WHERE referrer_id = $1 AND referee_id IS NULL
ORDER BY created_at DESC LIMIT 1;

-- name: GetReferralByCode :one
SELECT * FROM referrals WHERE code = $1;

-- name: RedeemReferral :exec
UPDATE referrals SET referee_id = $1, redeemed_at = NOW()
WHERE code = $2 AND referee_id IS NULL;

-- name: ListReferralsByUser :many
SELECT * FROM referrals WHERE referrer_id = $1
ORDER BY created_at DESC;

-- name: CountSuccessfulReferrals :one
SELECT COUNT(*) FROM referrals WHERE referrer_id = $1 AND referee_id IS NOT NULL;

-- name: CountRewardsEarned :one
SELECT COUNT(*) FROM referrals WHERE referrer_id = $1 AND referrer_reward_granted = true;

-- name: GrantReferrerReward :exec
UPDATE referrals SET referrer_reward_granted = true WHERE id = $1;

-- name: GrantRefereeReward :exec
UPDATE referrals SET referee_reward_granted = true WHERE id = $1;
