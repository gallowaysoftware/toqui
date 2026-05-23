# Runbook: Refresh-token reuse incident

A refresh token from a previously-rotated family was presented again, triggering family revocation. Either we caught real token theft, or a benign double-refresh race tripped the breach-response path.

> **Self-host note**: The example commands below use `gcloud logging read`
> and `gcloud sql connect` because this runbook was originally written for
> the GCP-hosted SaaS. The triage logic + SQL queries are infra-agnostic —
> the SQL works against any Postgres, and the log-search step works against
> whatever log aggregator you run (Loki / journalctl / `docker logs` /
> CloudWatch). Substitute the obvious equivalents.

## Symptoms

- User report: "I got logged out on all my devices at the same time" or "I have to keep logging in."
- Spike in `auth.token_reuse_detected` audit events in your log aggregator.
- Sudden surge in `auth.token_refresh_denied` (downstream effect — every device in the revoked family now fails to refresh).
- Sometimes correlated with `auth.lockout` if the user / attacker hammers `/auth/refresh` after revocation.
- For email+password setups: a wave of "I can't get back in" reports. For Google OAuth setups: the symptom is "Google sign-in works but the app immediately bounces me back to login."

## Triage

1. **Confirm the audit event signature.**
   ```bash
   gcloud logging read \
     'resource.type="cloud_run_revision" AND resource.labels.service_name="toqui-backend" AND jsonPayload.event="auth.token_reuse_detected"' \
     --project=toqui-prod --limit=50 --format=json --freshness=24h
   ```
   Capture the `user_id`, `family_id`, `jti`, and `ip` fields from each event.

2. **Pull the user's recent refresh-token activity from Postgres.**
   ```bash
   gcloud sql connect toqui-prod-pg --user=postgres --project=toqui-prod
   ```
   Then in psql:
   ```sql
   SELECT jti, family_id, issued_at, revoked_at, last_used_at, last_used_ip, user_agent
   FROM refresh_tokens
   WHERE user_id = '<uuid>'
   ORDER BY issued_at DESC
   LIMIT 50;
   ```
   What you're looking for:
   - **Real attack signal**: tokens in the same `family_id` with `last_used_ip` from two different ASNs / countries within a short window. Two distinct user-agents on the family is also a real signal.
   - **Benign signal**: same IP, same user-agent, two refresh attempts within a couple of seconds of each other (race between the app and a browser tab both refreshing on resume).

3. **Cross-reference with `auth.lockout` events.** If the same IP hit the AuthLimiter (5 failures / 15 min), an attacker is more likely than a benign race:
   ```bash
   gcloud logging read \
     'resource.type="cloud_run_revision" AND jsonPayload.event="auth.lockout" AND jsonPayload.user_id="<uuid>"' \
     --project=toqui-prod --limit=20 --format=json --freshness=24h
   ```

4. **Check OAuth provider login history.** Ask the user to check their Google / Facebook / Apple account security pages for unfamiliar device / location entries. If their OAuth identity itself is compromised, our refresh-token revocation is downstream of the real problem.

## Mitigations

1. **If benign (single IP, single UA, simultaneous refreshes — most common case)**:
   - Reassure the user: their account is safe, the family revocation is the breach-response path doing its job.
   - Have them log back in via OAuth — a new family is issued and they're back to normal.
   - No further action needed unless this is recurring (in which case file a follow-up: the client likely has a refresh-token race condition worth fixing in `toqui/lib/auth.ts`).

2. **If real attack suspected (multi-IP / multi-UA on the family, OAuth provider shows an unfamiliar device)**:
   - Force a full re-auth: revoke ALL of the user's refresh tokens (not just the one family):
     ```sql
     UPDATE refresh_tokens
     SET revoked_at = now(), revoked_reason = 'incident-response'
     WHERE user_id = '<uuid>' AND revoked_at IS NULL;
     ```
   - Tell the user (via `privacy@toqui.travel`) to:
     - Review their Google / Facebook / Apple account security and revoke any unfamiliar sessions.
     - Change their OAuth provider password.
     - Enable 2FA on the OAuth provider if not already on.
   - Audit the user's account for unauthorized actions in the last 24h: trips created, bookings added, sharing toggled on, profile changes. Use `auth_audit_log` plus the relevant per-entity audit events (`trip.share`, `admin.*` if the user happened to be an admin).
   - If there is evidence the attacker exfiltrated data via the in-app data export (`POST /auth/export-data`), file a personal-data-breach assessment per `dpa.astro` §7 (72-hour notification clock starts now).

3. **If unsure**: treat as real until proven benign — the cost of a forced re-auth on a benign event is one extra OAuth click; the cost of dismissing a real attack is account takeover.

## Reference

- `internal/auth/` — refresh-token rotation, JTI/family logic, reuse-detection breach response.
- `refresh_tokens` table — `jti`, `family_id`, `revoked_at`, `last_used_ip`, `last_used_ua`. Migrations in `db/migrations/`.
- `internal/audit/` — emits `auth.token_reuse_detected`, `auth.token_refresh_denied`, `auth.lockout`.
- `auth_audit_log` table — broader auth event history (login, refresh, logout, account_delete) for cross-referencing.

## Postmortem

Capture in the incident issue (or in `docs/compliance/dsar-log.md` if it crosses into a personal-data breach):

- **User ID** (pseudonymized in any public writeup — use first 8 chars of the UUID).
- **Verdict**: real attack vs. benign race vs. unknown.
- **Evidence summary**: IPs / ASNs / UAs observed on the family, with timestamps.
- **Containment actions**: which tokens revoked, when the user was notified, what they were asked to do.
- **Outcome**: did the user confirm OAuth-side compromise? did 2FA get enabled?
- **Pattern check**: how many `auth.token_reuse_detected` events have we seen in the last 30 days? Is this part of a wave?

If the verdict is "benign race" and we see this for more than ~3 distinct users in a month, file a `P1` issue against `toqui` (the Expo app) to fix the underlying refresh-race. If the verdict is "real attack" with confirmed data access, file a personal-data-breach assessment and start the 72-hour GDPR Art. 33 notification clock.
