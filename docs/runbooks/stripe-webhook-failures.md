# Runbook: Stripe webhook failures

Stripe webhook signature verification is failing or Stripe is repeatedly retrying — Trip Pro purchases or subscription state changes are not landing in our DB.

## Symptoms

- Customer email: "I bought Trip Pro / upgraded to Voyager but the trip didn't unlock / my plan still says Free."
- Spike in `payment.validation_failed` audit events in Cloud Logging.
- Stripe dashboard → Developers → Webhooks shows the endpoint with red "Failed" indicators and increasing retry counts.
- `payment.trip_pro_purchase` event volume drops while `checkout_initiated` (PostHog) volume stays normal — money is being taken but unlocks are not firing.
- `subscription_*` Stripe events stuck in "pending" with retry timer counting down.

## Triage

1. **Confirm the webhook is the problem, not the upstream code path.**
   ```bash
   gcloud logging read \
     'resource.type="cloud_run_revision" AND resource.labels.service_name="toqui-backend" AND (jsonPayload.event="payment.validation_failed" OR jsonPayload.event="payment.webhook_received")' \
     --project=toqui-prod --limit=100 --format=json --freshness=1h
   ```
   Look for `payment.validation_failed` clustered around the same minute Stripe shows retries. If you see `payment.webhook_received` followed by `payment.validation_failed`, signature verification is rejecting payloads. If you don't see `payment.webhook_received` at all, the request isn't reaching the handler — check Cloud Run + LB ingress instead.

2. **Compare the webhook signing secret in our env to Stripe's.**
   - In Stripe dashboard → Developers → Webhooks → click the endpoint → "Signing secret" → reveal.
   - In GCP: `gcloud secrets versions access latest --secret=stripe-webhook-secret --project=toqui-prod`
   - The two must match byte-for-byte. A common cause: the endpoint was rotated in the Stripe dashboard but the new secret was never written to Secret Manager.

3. **Check timestamp skew.** Stripe rejects webhook signatures where the request timestamp differs from server time by more than **5 minutes**. Check the Cloud Run revision's clock is sane:
   ```bash
   gcloud run services logs read toqui-backend \
     --region=northamerica-northeast1 --project=toqui-prod --limit=20 \
     | grep -E "time|timestamp|skew"
   ```
   Cloud Run hosts run NTP, so this is rare — but it's the right thing to rule out before doing anything destructive.

4. **Identify which events are failing.** In the Stripe dashboard, click the failing event ID and capture: `event.id` (`evt_…`), `customer.id` (`cus_…`), `event.type` (e.g. `checkout.session.completed`, `customer.subscription.updated`), and the user-impact window (first failure timestamp → last failure timestamp).

## Mitigations

1. **Rotate the webhook signing secret if it's mismatched.**
   - Stripe dashboard → Developers → Webhooks → endpoint → "Roll secret".
   - Update GCP Secret Manager: `gcloud secrets versions add stripe-webhook-secret --data-file=- --project=toqui-prod` (paste the new secret, Ctrl-D).
   - Force a Cloud Run revision restart so the new secret value is picked up:
     ```bash
     gcloud run services update toqui-backend \
       --region=northamerica-northeast1 --project=toqui-prod \
       --update-env-vars=STRIPE_WEBHOOK_SECRET_ROTATED=$(date +%s)
     ```
   - Verify the next webhook delivery succeeds (Stripe dashboard → Webhooks → endpoint → recent deliveries).

2. **Replay the missed webhook(s).** In the Stripe dashboard:
   - Open the failing event → top-right "..." → "Resend event".
   - For multiple failures: filter the events list by status=Failed and time range, then bulk-resend.
   - Watch the Cloud Run logs for `payment.webhook_received` followed by either `payment.trip_pro_purchase` (success) or `payment.validation_failed` (still broken).

3. **Manually unlock customer-impacting purchases while the webhook is being fixed.** If a customer is blocked and a fix isn't going to land quickly, use the admin endpoint to unblock them and reconcile later:
   ```bash
   curl -X POST https://api.toqui.travel/admin/unlock-trip \
     -H "Authorization: Bearer $ADMIN_BEARER" \
     -H "Content-Type: application/json" \
     -d '{"user_id": "<uuid>", "trip_id": "<uuid>"}'
   ```
   For a subscription customer, the equivalent is `POST /admin/grant-pro` (or run a one-off DB update if they bought Explorer/Voyager and `grant-pro` doesn't model the right tier — coordinate with engineering before doing this).

## Reference

- `internal/payment/` — Stripe checkout, webhook handler, signature verification (Trip Pro path).
- `internal/subscription/` — Subscription webhook handler (Explorer / Voyager path).
- `internal/audit/` — emits the `payment.*` audit events grepped above.
- Stripe dashboard, Developers → Webhooks → endpoint URL: `https://api.toqui.travel/api/subscription/webhook` and the Trip Pro endpoint.

## Postmortem

Capture in the incident issue:

- **Webhook event ID(s)** that failed (`evt_…`).
- **Stripe customer ID(s)** affected (`cus_…`) and our internal `user_id`(s) for cross-reference.
- **Impact window** — first failure → last failure → resolution timestamp (UTC).
- **Number of customers manually unlocked** vs. fixed by the webhook replay.
- **Root cause** — secret mismatch, code regression, infra issue, etc.
- **Revenue impact** — sum the affected line items so finance reconciliation has a clean number.

File a follow-up issue in `gallowaysoftware/toqui-backend` if a code change is needed (signature handler regression, missing webhook event type case, idempotency gap). Tag `P0` if customers were impacted; `P1` if it was caught before any customer-visible failure. Add the `security` label if the root cause involved secrets handling.
