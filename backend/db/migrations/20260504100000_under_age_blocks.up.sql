-- under_age_blocks records OAuth identities that have been refused account
-- creation because they failed the 18+ age verification gate.
--
-- Why a separate table: Kyle's redesign of the age gate (May 2026) hard-deletes
-- under-18 users via lifecycle.DeleteUser the moment they fail verification.
-- That CASCADE wipes the users row, so any audit trail tied to user_id
-- becomes orphaned (and Cloud Logging retention is bounded). This table
-- persists the refusal independently of the (now-deleted) user row.
--
-- Privacy stance: we never store the email plaintext. Only SHA-256 of the
-- lowercased email. The hash is sufficient to (a) prove a refusal happened
-- when audited and (b) recognise the same OAuth identity if they retry, but
-- it isn't a usable email list for any other purpose. The DOB itself is
-- never stored anywhere — it's parsed in-memory at /auth/verify-age and
-- discarded.
CREATE TABLE under_age_blocks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- SHA-256 of strings.ToLower(strings.TrimSpace(email)). 64 hex chars.
    -- UNIQUE so re-attempts from the same email are no-ops on the insert
    -- (the conflict tells the OAuth handler we already have a refusal on
    -- file and to short-circuit without creating a user).
    email_sha256    CHAR(64) NOT NULL UNIQUE,

    -- Which OAuth provider was used the first time we refused them. Useful
    -- for forensic queries ("did Apple Relay introduce a wave of underage
    -- attempts?") but not used for any policy decision today.
    oauth_provider  TEXT NOT NULL,

    blocked_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Lookup index for the OAuth login pre-check: hash the incoming email,
-- check if it's in here, refuse if so.
CREATE INDEX idx_under_age_blocks_email_sha256 ON under_age_blocks(email_sha256);
