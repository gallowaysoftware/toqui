# Runbook: Cloud Run rollback (bad deploy)

A bad deploy reached prod and is breaking users — error rate spike, broken feature, or a known-bad SHA is current. Roll back to the previous good revision and, if a forward DB migration shipped with it, roll the migration back too.

## Symptoms

- Sudden error-rate spike on `https://api.toqui.travel` correlated in time with a recent merge to `main` + `gh workflow run CI`.
- Users report a broken feature within minutes of a deploy ("login fails", "chat returns 500", "trip won't save").
- `livez` is green (server is up) but `readyz` is red (DB connectivity broken — common signature of the 2026-04-17-style outage where `--vpc-connector` was dropped).
- Cloud Logging shows DB query timeouts at 10s after a fresh revision rolled out (private-IP Cloud SQL is unreachable from a no-VPC revision).
- Sentry / frontend error monitoring lights up with new error signatures matching the bad SHA.

## Triage

1. **Confirm the bad revision is current.**
   ```bash
   gcloud run services describe toqui-backend \
     --region=northamerica-northeast1 --project=toqui-prod \
     --format='value(status.traffic[].revisionName,status.traffic[].percent,spec.template.spec.containers[0].image)'
   ```
   The image tag is the git SHA. Compare to `git rev-parse origin/main` and to the SHA you suspect is bad.

2. **List recent revisions to find the previous good one.**
   ```bash
   gcloud run revisions list --service=toqui-backend \
     --region=northamerica-northeast1 --project=toqui-prod \
     --limit=10 --sort-by=~metadata.creationTimestamp \
     --format='table(metadata.name,metadata.creationTimestamp,spec.containers[0].image,status.conditions[0].type)'
   ```
   The previous-good revision is usually the one immediately before the current one — but verify it had `Ready=True` and was actually serving traffic at the expected timestamp (not a failed rollout that never received traffic).

3. **Confirm the previous-good revision still has the VPC connector attached.** Prod Cloud SQL is private-IP-only (toqui-terraform PR #24); a revision without `--vpc-connector=toqui-connector --vpc-egress=private-ranges-only` is unreachable from the DB and will time out at 10s. Without this check you risk rolling back to a different broken revision.
   ```bash
   gcloud run revisions describe <previous-revision> \
     --region=northamerica-northeast1 --project=toqui-prod \
     --format='value(spec.template.metadata.annotations."run.googleapis.com/vpc-access-connector",spec.template.metadata.annotations."run.googleapis.com/vpc-access-egress")'
   ```
   Both annotations must be set (`toqui-connector` and `private-ranges-only`).

4. **Check whether the bad deploy ran a forward DB migration.** Migrations run in a Cloud Run job before the service deploys (see the `deploy-prod` CI job).
   ```bash
   gcloud run jobs executions list --job=toqui-migrate \
     --region=northamerica-northeast1 --project=toqui-prod \
     --limit=5 --sort-by=~metadata.creationTimestamp
   ```
   If the most recent execution timestamp matches the bad deploy and ran `up`, you'll likely need to roll the schema back too (next section).

## Mitigations

1. **Route 100% of traffic back to the previous good revision.**
   ```bash
   gcloud run services update-traffic toqui-backend \
     --to-revisions=<previous-revision>=100 \
     --region=northamerica-northeast1 --project=toqui-prod
   ```
   Verify: `curl -fsS https://api.toqui.travel/healthz` returns 200 and the error-rate graph drops within 1-2 minutes.

2. **If the bad deploy ran a forward migration, roll it back.** Re-deploy the migration job pinned to the previous SHA's image and run `down -steps 1`:
   ```bash
   IMAGE=northamerica-northeast1-docker.pkg.dev/toqui-infra/toqui-backend/toqui-backend
   PREVIOUS_SHA=<sha-of-previous-good-revision>

   gcloud run jobs deploy toqui-migrate \
     --image=$IMAGE:$PREVIOUS_SHA \
     --region=northamerica-northeast1 --project=toqui-prod \
     --vpc-connector=toqui-connector --vpc-egress=private-ranges-only \
     --command=/migrate \
     --args="-direction,down,-steps,1" \
     --execute-now
   ```
   **VPC flags are mandatory** on every `gcloud run jobs deploy` to prod (see CLAUDE.md "outage history" — dropping them is exactly what caused the 2026-04-17 outage; CI was fixed in toqui-backend PR #359). If the rollback migration is non-trivial (data drop, irreversible transform), pause and call in another engineer before running `--execute-now`.

3. **Confirm staging matches before doing anything else.** If staging is up, deploy the same `<previous-good-revision>` image there too so the "running everywhere" state is consistent. If staging is down (so the push-to-main auto-deployed prod), this is a non-issue.

4. **Tell people what happened.** Post in the team channel with: bad SHA, good SHA we rolled back to, time-of-rollback, observed-recovery-time. If users were impacted for >5 min, also post on the status page.

## Reference

- `internal/db/` — PostgreSQL connection setup; the 10s timeout you'll see in logs comes from here.
- `db/migrations/` — `golang-migrate`-format migrations; numbered `NNNN_name.up.sql` / `NNNN_name.down.sql`. **Every up migration must have a working down** — that's the precondition for this runbook.
- `Dockerfile` — produces a single image with both `/server` and `/migrate` entrypoints; the migration job and the service deploy from the same image.
- `CLAUDE.md` "Deploying to Prod" and "Rolling Back" sections — the canonical command list this runbook is derived from.

## Postmortem

File an incident issue in `gallowaysoftware/toqui-backend` with the `P0` label (or `P1` if no customer impact). Capture:

- **Bad SHA** and **previous good SHA** (the one we rolled back to).
- **Trigger**: what merged to main / what `gh workflow run CI` invocation deployed it. Link the PR.
- **Detection**: how did we notice (alert, user report, manual smoke test)? What was the time-to-detect from deploy?
- **Time-to-mitigate**: deploy timestamp → rollback complete timestamp.
- **User-impact window**: error-rate-spike-start → error-rate-back-to-baseline (these can differ from time-to-mitigate if there's a CDN / cache lag).
- **Root cause**: what about the bad SHA broke things? (missing VPC flag, schema-mismatched migration, unhandled error, dependency upgrade, etc.)
- **Migration rollback**: did we have to roll the schema back too? Was the down migration well-tested?
- **Followups**: regressions to add to CI (e.g. a smoke test against `/readyz` post-deploy that fails the workflow on regression), runbook gaps (e.g. if a step in this file was wrong or missing).

If the root cause was a dropped `--vpc-connector` flag (the historical pattern), specifically check whether anyone hand-deployed via raw `gcloud` outside the CI workflow and refresh the team on the "always use CI" guidance from CLAUDE.md.
