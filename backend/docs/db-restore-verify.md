# DB restore verification runbook

GDPR Article 32 and PIPEDA Schedule 1 Principle 4.7 (Safeguards) both require
that backups be **tested**, not just configured. Cloud SQL `toqui-db` has daily
backups enabled (7-day retention as of `toqui-terraform#30`) but **no
restore has ever been tested in production**. This runbook is the
on-demand fix for that gap; the long-term automated version is tracked
in `toqui-backend#266`.

Run this monthly. The first execution is the compliance-gating one —
once you've confirmed restore works, the recurring runs are cheap
sanity checks.

## What it does

`scripts/db-restore-verify.sh` automates a clone-from-backup +
read-only verification + teardown loop. End-to-end, ~15-25 minutes.

1. Picks the latest successful backup of `toqui-prod:toqui-db`
   (or a specific backup ID via `--backup`).
2. Clones it into a temporary instance
   `toqui-db-restore-verify-<utc-timestamp>` (db-f1-micro, public IP,
   no HA — meant to live ~30 min).
3. Starts `cloud-sql-proxy`, connects via `psql`, and runs:
   - `pgcrypto` and `postgis` extension presence checks.
   - Row counts on six critical tables (`users`, `trips`,
     `itinerary_items`, `bookings`, `subscriptions`, `refresh_tokens`).
   - Recency probes (`MAX(created_at)` on users, `MAX(updated_at)` on
     trips) so a backup that's stuck in time gets caught.
4. Appends the result + timestamp to `docs/restore-verify-log.md`.
5. Tears down the temp instance (skip with `--keep` if you want to poke
   at it manually first — but **do delete it after** or you're paying
   ~$25/month for a forgotten test instance).

## Prerequisites

- `gcloud auth login` and `gcloud auth application-default login`
  (for Secret Manager access to pull `prod-database-url`).
- IAM on `toqui-prod`: `roles/cloudsql.admin` (clone, patch, delete) and
  `roles/secretmanager.secretAccessor` on `prod-database-url`.
- `cloud-sql-proxy` v2 on `$PATH`. Install:
  `gcloud components install cloud-sql-proxy` or
  `brew install cloud-sql-proxy`.
- `psql` v15+ on `$PATH` (any libpq client works; `psql` is what the
  script invokes).
- Budget: db-f1-micro for ~30 min ≈ $0.04. Round up to a buck.

## First run (compliance-gating)

```bash
cd /path/to/toqui-backend
./scripts/db-restore-verify.sh
```

The script is interactive on first run — it'll show you the backup ID
it picked and ask before creating the clone. Approve, wait ~10 min for
the clone to finish, watch the health check, watch the teardown.

Expected output ends with `[info] Restore verification PASSED.` and an
appended entry in `docs/restore-verify-log.md`.

## Recurring runs

Once a month, in CI mode:

```bash
./scripts/db-restore-verify.sh --yes
```

Add to a Cloud Scheduler job (see `toqui-backend#266`) once you're
comfortable.

## Failure modes and what to do

### "No successful backup found"

Either the backup schedule isn't running (check
`gcloud sql backups list --instance=toqui-db --project=toqui-prod`) or
all recent backups failed. Both are P1 — investigate the backup
schedule's GCP operations log first
(`gcloud logging read 'resource.type="cloudsql_database"' --limit=20`).

### Clone hangs > 30 min

`gcloud sql instances clone` can take a long time on large instances.
Ours is db-f1-micro (~50 GB allocated, much less in use), so anything
past 20 minutes means the operation is wedged. Kill the script (Ctrl+C
will hit the trap, kill the proxy, and skip teardown) — then either
wait it out via `gcloud sql operations list --project=toqui-prod` or
delete the half-clone manually.

### Health check fails on extensions

If `pgcrypto` or `postgis` isn't loaded after clone, the cloned
instance came up without our schema migrations applied. That's a
known GCP behavior on cross-project clones; if it happens within the
same project (which is what we do), it's a real bug — file a P1.

### Health check passes but row counts look wrong

Compare against the source: `gcloud sql connect toqui-db ...` then run
the same row-count query. If the clone is missing data (e.g. last few
hours of inserts), point-in-time-recovery may not be configured the
way we think it is. Check the `pointInTimeRecoveryEnabled` flag on the
prod instance.

### "Don't forget to delete it manually"

Means you ran with `--keep`. Tidy up:

```bash
gcloud sql instances delete <name> --project=toqui-prod
```

## Why not VPC-private?

Production `toqui-db` is private-IP-only (per `toqui-terraform` PR #24)
and reachable only through the `toqui-connector` VPC connector. The
restore-verify temp instance gets public IP because (a) it's a
short-lived sanity check, not a production candidate, and (b) running
this script from your laptop is impractical otherwise — laptop isn't
on the VPC. The cloud-sql-proxy + Cloud SQL Admin API still encrypts
the connection end-to-end; we just skip the additional VPC-level
isolation for ~30 minutes.

If you want to do a real DR drill (point an app at the restored
instance), DON'T use this script — clone manually, attach the VPC
connector, repoint the app. This script is a verification tool, not a
DR-drill harness.

## Long-term automation (separate work)

`toqui-backend#266` tracks the monthly Cloud Scheduler job that runs
this end-to-end. Implementation sketch when you're ready:

- Cloud Scheduler → Pub/Sub → Cloud Run Job (containerized version of
  this script).
- Result alert → email if `FAIL`.
- Result archive → GCS bucket with monthly retention.
- Optionally: clone-from-backup happens in a dedicated
  `toqui-restore-verify` GCP project so a misconfigured script can
  never accidentally delete prod.

The recurring job is the compliance baseline, but until the manual
script proves restore works at all, automating it is putting the cart
before the horse.

## Restore-verify log

Append-only history of restore-verify runs lives in
`docs/restore-verify-log.md` (created on first run by the script).
Auditors (or future-you, six months later) will appreciate the
timestamped trail.
