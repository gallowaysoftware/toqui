# Runbook: GDPR DSAR fulfillment (export and deletion)

A user has asked us to exercise their GDPR Article 15 / 17 / 20 rights. This is the operator playbook for running an export, a deletion, or both — within the 30-day GDPR window and with an audit trail.

## Symptoms

- Email to `privacy@toqui.travel`: "Please send me a copy of all data you hold about me" (Art. 15 access / Art. 20 portability).
- Email to `privacy@toqui.travel`: "Please delete my account and all my data" (Art. 17 erasure).
- Email to `privacy@toqui.travel` requesting both ("send me a copy and then delete").
- Letter from a Supervisory Authority (e.g. CNIL, Information Commissioner's Office, Office of the Privacy Commissioner of Canada) forwarding a complaint that includes a DSAR component — treat as a DSAR plus supervisory notification, not just a DSAR.

## Triage

1. **Identify the user by their email.**
   ```bash
   gcloud sql connect toqui-prod-pg --user=postgres --project=toqui-prod
   ```
   ```sql
   SELECT id, email, display_name, created_at, deleted_at, age_verified_at
   FROM users
   WHERE email = '<requester@example.com>';
   ```
   - **No row**: ask the requester to confirm the email they signed up with (some users sign up via Google with one email and contact us from another). If still no match, send the templated "no record found" reply (do **not** confirm or deny on the basis of email guessing).
   - **`deleted_at IS NOT NULL`**: their account was already deleted; reply confirming the deletion timestamp and that no data remains beyond what is described in the privacy policy retention schedule (Section 7).
   - **One row, active**: continue.

2. **Verify the requester's identity.** Per `privacy.astro` §13, we challenge with a verification email if there is **any** doubt the request came from the account owner. This catches social-engineering deletion requests and doxx-by-DSAR.
   - **Always verify** if the request came from an email address that doesn't match the account email.
   - **Always verify** if the request includes unusual urgency, or asks us to send the export to a different email than the account email.
   - Verification template: send a one-line email to the account email asking the user to reply "yes, this is my request" within 7 days. The 30-day Art. 15/17 clock pauses while we wait for verification (Art. 12(6)).

3. **Classify the request.** Export, deletion, both, or restriction (Art. 18). For restriction or rectification (Art. 16) — those don't have a tooled path in `lifecycle`, do them manually via SQL after confirming with the requester exactly what to restrict / correct.

## Mitigations

### Export (Art. 15 access / Art. 20 portability)

**Preferred**: have the user trigger the export themselves while logged in — that way they see exactly the same artifact a self-service user would.
```
POST /auth/export-data
Authorization: Bearer <user-token>
```
The endpoint queues an async job, the GCS-signed-URL deliverable is emailed to the user, and the export is logged via `auth.data_export`.

**Server-side (when the user can't or won't log in)**: SSH-equivalent into a one-off Cloud Run job or run via `grpcurl` against an admin shell:
```bash
# Best path: write the user_id and call lifecycle.Service.ExportUserData via a
# small admin helper (no public RPC for this — by design, exports are
# user-initiated). Exec in via Cloud Run jobs:
gcloud run jobs execute toqui-admin-export \
  --region=northamerica-northeast1 --project=toqui-prod \
  --args="--user-id=<uuid>"
# (If the toqui-admin-export job doesn't exist yet, this is a one-time setup —
# the lifecycle.Service.ExportUserData function is the operative call site.)
```
The export goes to the GCS bucket named in `GCS_EXPORT_BUCKET`; generate a signed URL valid for 24 hours and email it to the user — do **not** attach the JSON to the reply email (DSAR exports often exceed mail-server attachment limits and email is not an appropriate channel for bulk personal data).

Privacy commitment: deliver within 24 hours of the request being verified (per `privacy.astro` §9). The hard GDPR deadline is 30 days but we promise faster.

### Deletion (Art. 17 erasure)

```bash
# Same pattern — the canonical call is lifecycle.Service.DeleteUser. Run via:
gcloud run jobs execute toqui-admin-delete \
  --region=northamerica-northeast1 --project=toqui-prod \
  --args="--user-id=<uuid> --reason=dsar"
```
What this does (synchronous, by design):
- Postgres CASCADE delete on `users.id` — purges trips, bookings, itinerary items, refresh tokens, audit-log foreign-keyed rows, payments, subscriptions, etc.
- Firestore chat-message purge for `users/{uid}/trips/{tripId}/chatSessions/...`.
- PostHog `distinct_id` deletion via the PostHog Personal API endpoint (the SHA-256 of the user UUID is the distinct_id we send).
- Sentry user purge — see issue #384 for the periodic-scrub follow-up; today this is best-effort and logged as a TODO comment in `internal/lifecycle/service.go`. If #384 is still open when you run this, do a manual sweep of Sentry for the user's pseudonymized ID.

Confirm completion to the user within **30 days** (Art. 17 deadline). Our internal SLA is 7 days — the lifecycle service runs synchronously so unless the job errored, the moment it returns the deletion is done.

### Both (export then delete)

Deliver the export first, get the user's confirmation that they have the file, then run deletion. Don't auto-chain the two — once the data is gone, we can't recover from a delivery failure on the export.

## Reference

- `internal/lifecycle/service.go` — `ExportUserData(ctx, userID)`, `DeleteUser(ctx, userID)`. Single source of truth for both flows.
- `internal/exportstorage/` — GCS-vs-local-FS abstraction. Prod uses GCS; dev falls back to `/tmp/toqui-exports`.
- `internal/audit/` — emits `auth.data_export` and `auth.account_delete` (the latter with a `reason` field — set to `dsar` for DSAR-driven deletions, distinct from `under_age` and from user-initiated app deletions).
- Privacy policy: `toqui-site/src/pages/privacy.astro` §8 (rights), §9 (how to exercise), §13 (identity verification).
- DPA: `toqui-site/src/pages/legal/dpa.astro` §8 (data subject rights assistance), §11 (retention and deletion).

## Postmortem

Log the request in `docs/compliance/dsar-log.md` (placeholder file — Kyle to create on first DSAR; structure suggested below):

| Received | Verified | Type (export / delete / both) | User pseudonym (first 8 chars of UUID) | Completed | Notes |
|---|---|---|---|---|---|
| _yyyy-mm-dd_ | _yyyy-mm-dd_ | export | `a1b2c3d4` | _yyyy-mm-dd_ | Delivered via 24h signed URL. |

Do **not** log the requester's email or full UUID in the in-repo log — pseudonymize. Keep the full mapping in the same Google Drive folder where DPAs are filed (`Toqui / Compliance / DSAR Mapping/`), restricted to the Privacy Officer.

If a deletion was performed, also confirm: PostHog `distinct_id` actually purged (check the PostHog UI for the hashed ID); Sentry record purged (per #384 follow-up); GCS export bucket scrubbed of any pre-deletion exports for this user (signed URLs expire but the underlying objects do not).

If the request originated from a Supervisory Authority complaint, file the SA's reference number and correspondence thread in the same Drive folder, and include the SA name in the "Notes" column.
