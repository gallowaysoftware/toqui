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

-- name: GrantReferralTripUnlock :one
-- Finds the user's most recent trip that isn't already unlocked and creates a
-- trip_unlocks row with source='referral'. Returns NULL if user has no eligible trip.
INSERT INTO trip_unlocks (user_id, trip_id, source)
SELECT sqlc.arg(user_id), t.id, 'referral'
FROM trips t
WHERE t.user_id = sqlc.arg(user_id)
  AND NOT EXISTS (
    SELECT 1 FROM trip_unlocks tu WHERE tu.user_id = sqlc.arg(user_id) AND tu.trip_id = t.id
  )
ORDER BY t.created_at DESC
LIMIT 1
ON CONFLICT (user_id, trip_id) DO NOTHING
RETURNING *;

-- name: HasPendingReferralCredit :one
-- Checks if a user has a referral reward granted (as referrer or referee) but
-- no trip unlock with source='referral' yet.
SELECT EXISTS(
  SELECT 1 FROM referrals r
  WHERE (r.referrer_id = sqlc.arg(user_id) AND r.referrer_reward_granted = true)
) AND NOT EXISTS(
  SELECT 1 FROM trip_unlocks tu WHERE tu.user_id = sqlc.arg(user_id) AND tu.source = 'referral'
);

-- name: GetReferralByReferee :one
SELECT * FROM referrals WHERE referee_id = sqlc.arg(referee_id) LIMIT 1;
