-- Reverse 20260428190100_rename_helcim_to_payments. Restores the Helcim-shaped
-- table and column names plus the dropped Helcim-specific columns (re-added
-- as nullable since prod has no Helcim payments to constrain).

ALTER TABLE checkout_sessions ADD COLUMN secret_token TEXT NOT NULL DEFAULT '';
ALTER INDEX IF EXISTS idx_checkout_sessions_user RENAME TO idx_helcim_sessions_user;
ALTER INDEX IF EXISTS idx_checkout_sessions_trip RENAME TO idx_helcim_sessions_trip;
ALTER INDEX IF EXISTS idx_checkout_sessions_token RENAME TO idx_helcim_sessions_token;
ALTER TABLE checkout_sessions RENAME TO helcim_checkout_sessions;

ALTER TABLE payments ADD COLUMN response_hash TEXT;
ALTER TABLE payments ADD COLUMN card_token VARCHAR(256);
ALTER TABLE payments ADD COLUMN approval_code VARCHAR(64);
ALTER INDEX IF EXISTS idx_payments_trip RENAME TO idx_helcim_payments_trip;
ALTER INDEX IF EXISTS idx_payments_user RENAME TO idx_helcim_payments_user;
ALTER TABLE payments RENAME COLUMN external_payment_id TO helcim_transaction_id;
ALTER TABLE payments RENAME TO helcim_payments;
