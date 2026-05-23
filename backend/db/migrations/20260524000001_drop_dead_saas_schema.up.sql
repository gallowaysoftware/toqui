-- Drop tables + columns left behind after the SaaS-to-OSS strip
-- (Waves 3.1–3.5 deleted the code; this drops the schema).
--
-- All of these were unused as of the OSS pivot:
--   payments, subscriptions, trip_unlocks, stripe_events      → slice 1 (Stripe)
--   referrals                                                  → slice 1 (referral codes)
--   daily_usage, ai_usage                                      → slice 1 (usage caps)
--   user_consents, under_age_blocks                            → slice 2 (gates)
--   waitlist                                                   → slice 2.5 (operator gatekeeping)
--   users.subscription_tier, users.apple_sub,
--     users.age_verified_at, trips.trial_*                     → various
--
-- IF EXISTS guards keep this safe to run against any DB shape that's
-- partway through the cleanup history. Self-hosters who installed the
-- app post-OSS-release won't have these tables at all and will skip
-- every statement.

DROP TABLE IF EXISTS stripe_events CASCADE;
DROP TABLE IF EXISTS trip_unlocks CASCADE;
DROP TABLE IF EXISTS payments CASCADE;
DROP TABLE IF EXISTS subscriptions CASCADE;
DROP TABLE IF EXISTS referrals CASCADE;
DROP TABLE IF EXISTS daily_usage CASCADE;
DROP TABLE IF EXISTS ai_usage CASCADE;
DROP TABLE IF EXISTS user_consents CASCADE;
DROP TABLE IF EXISTS under_age_blocks CASCADE;
DROP TABLE IF EXISTS waitlist CASCADE;

ALTER TABLE users DROP COLUMN IF EXISTS subscription_tier;
ALTER TABLE users DROP COLUMN IF EXISTS apple_sub;
ALTER TABLE users DROP COLUMN IF EXISTS age_verified_at;

ALTER TABLE trips DROP COLUMN IF EXISTS trial_started_at;
ALTER TABLE trips DROP COLUMN IF EXISTS trial_ends_at;
ALTER TABLE trips DROP COLUMN IF EXISTS is_unlocked;
