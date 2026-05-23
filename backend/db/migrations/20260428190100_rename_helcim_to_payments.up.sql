-- Rename Helcim-shaped tables/columns to processor-neutral names. Helcim was
-- never wired up; the payments path runs through Stripe (see internal/payment/stripe.go).
-- Carrying "helcim" in column names + struct fields cost-of-readability with no
-- corresponding benefit, so we collapse the abstraction down to one provider.
--
-- Helcim-only columns (approval_code, card_token, response_hash, secret_token)
-- are dropped: Stripe doesn't populate them and prod has no real Helcim
-- payments to preserve. The down migration re-adds them as nullable.

-- helcim_payments → payments
ALTER TABLE helcim_payments RENAME TO payments;
ALTER TABLE payments RENAME COLUMN helcim_transaction_id TO external_payment_id;
ALTER TABLE payments DROP COLUMN IF EXISTS approval_code;
ALTER TABLE payments DROP COLUMN IF EXISTS card_token;
ALTER TABLE payments DROP COLUMN IF EXISTS response_hash;
ALTER INDEX IF EXISTS idx_helcim_payments_user RENAME TO idx_payments_user;
ALTER INDEX IF EXISTS idx_helcim_payments_trip RENAME TO idx_payments_trip;
-- Postgres auto-renames the unique constraint on helcim_transaction_id to
-- payments_external_payment_id_key when the column is renamed. Index name
-- on the unique constraint follows.

-- helcim_checkout_sessions → checkout_sessions
ALTER TABLE helcim_checkout_sessions RENAME TO checkout_sessions;
ALTER TABLE checkout_sessions DROP COLUMN IF EXISTS secret_token;
ALTER INDEX IF EXISTS idx_helcim_sessions_user RENAME TO idx_checkout_sessions_user;
ALTER INDEX IF EXISTS idx_helcim_sessions_trip RENAME TO idx_checkout_sessions_trip;
ALTER INDEX IF EXISTS idx_helcim_sessions_token RENAME TO idx_checkout_sessions_token;
