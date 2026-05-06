# Sub-Processor DPA Register

**Purpose.** Track the signed Data Processing Agreement (DPA) status with each sub-processor that handles personal data on Toqui's behalf. This register exists to satisfy:

- **GDPR Article 28** — controllers (us, in the customer-facing relationship) must only use processors who provide sufficient guarantees of compliance, and the engagement must be governed by a written contract / DPA. Article 28(2) further requires us to track sub-processor changes and notify customers of additions or replacements.
- **PIPEDA Principle 4.1.3 (Accountability)** — we remain accountable for personal information transferred to a third party for processing, and must use contractual or other means to provide a comparable level of protection.

This file is the canonical operational tracker. The customer-facing sub-processor list lives in two places — `toqui-site/src/pages/privacy.astro` §5 and `toqui-site/src/pages/legal/dpa.astro` §5 — and both must be kept in sync with this register whenever a row changes.

This is a **scaffold**. Most rows are `not started` until the corresponding signed DPA PDF is filed. Update each row as DPAs are chased and signed.

---

## Sub-processor table

| Sub-processor | Service / Role | Data categories | DPA status | DPA URL or filename | Signed date | Renewal due | Notes |
|---|---|---|---|---|---|---|---|
| Anthropic, PBC | Claude API — chat, planner, classifier (incl. CompanionGate) | Chat message content, trip context | not started | _file PDF_ | _yyyy-mm-dd_ | _yyyy-mm-dd_ | API-only access; per Anthropic API terms inputs are not used to train models. 30-day default API request retention until ZDR amendment is signed (see privacy.astro §4). |
| Google LLC (API products) | Gemini Developer API + Vertex AI (Gemini 3 Preview) + Google Places + Google OAuth + Google Maps grounding | Chat message content, trip context, OAuth identity (email, name, profile photo, sub) | not started | _file PDF_ | _yyyy-mm-dd_ | _yyyy-mm-dd_ | Covers all Google API products that are not GCP infra (those are a separate row below). Gemini 3 on Vertex uses the global endpoint. Google's published Cloud DPA / Generative AI APIs Additional Terms apply. |
| Google Cloud (GCP) | Cloud Run, Cloud SQL (Postgres 16, private IP), Firestore, Artifact Registry, Secret Manager, GCS (data exports), Cloud Logging, Cloud Monitoring | All persisted personal data — account, trips, chat (Firestore), bookings, audit logs, GDPR exports | not started | _file PDF_ | _yyyy-mm-dd_ | _yyyy-mm-dd_ | Infra DPA — distinct from the API-products row above. Google publishes the [Cloud Data Processing Addendum](https://cloud.google.com/terms/data-processing-addendum) — accept it via the GCP Console once and keep a copy. Region: `northamerica-northeast1`. |
| Stripe, Inc. | Payments — Trip Pro one-time purchase, Explorer / Voyager subscriptions, billing portal, webhooks | Payment method data (handled by Stripe directly, not stored by us), customer email, Stripe customer ID, line-item metadata | not started | _file PDF_ | _yyyy-mm-dd_ | _yyyy-mm-dd_ | Stripe DPA is downloadable from the Stripe dashboard → Settings → Legal. Card data never touches our servers (Stripe Checkout / Billing Portal hosted by Stripe). |
| Resend (Resend, Inc.) | Transactional email — waitlist verification, invite emails, account notifications | Email address, name, message content | not started | _file PDF_ | _yyyy-mm-dd_ | _yyyy-mm-dd_ | DPA available on request from Resend; sign before opening up to EU users at scale. |
| PostHog (PostHog Inc., EU instance) | Product analytics — server-side events from `internal/analytics/` and client-side events from app + marketing site (cookie-consented) | Pseudonymized (SHA-256) user IDs, event names + categorical properties, IP for geo (truncated). NEVER trip content per code-enforced privacy guardrails (see toqui-backend CLAUDE.md). | not started | _file PDF_ | _yyyy-mm-dd_ | _yyyy-mm-dd_ | EU-hosted (`eu.i.posthog.com`) — data residency in the EU. PostHog publishes a standard DPA on their site. `process_person_profile=identified_only` is set so anonymous rollups remain possible. |
| Functional Software, Inc. (Sentry) | Error tracking — frontend (Expo app + admin) only | Stack traces, device info, pseudonymized user IDs (PII masking enabled) | not currently used by backend — add when wired | _file PDF_ | _yyyy-mm-dd_ | _yyyy-mm-dd_ | Sentry is wired in the **frontend** (`toqui` Expo app — see `app/_layout.tsx`, `components/ErrorBoundary.tsx`) but **not** in the Go backend (`toqui-backend/internal/`). Privacy policy + DPA both list Sentry already — that's accurate today because the frontend reports errors. Move this row to `not started` (chase signature) and revisit data categories if/when backend Sentry instrumentation lands. The lifecycle service has TODO comments about needing a periodic Sentry user purge — track that under #384. |
| Cloudflare, Inc. | Cloudflare Pages (toqui.travel marketing site, toqui-admin panel) + DNS for `toqui.travel` | Standard CDN access logs, IP, request metadata. No application data. | not started | _file PDF_ | _yyyy-mm-dd_ | _yyyy-mm-dd_ | **NOT currently listed in privacy.astro §5 or dpa.astro §5** — flagged as a separate disclosure gap (see "Findings" below). DPA available via Cloudflare dashboard → Manage Account → Configurations → Data Processing Addendum (click-through). |
| Performance Horizon Group Ltd. (Partnerize) | Affiliate click tracking for Expedia Group brands — VRBO, Expedia Hotels, Hotels.com | Destination URL, click metadata, hashed sub-ID (`tripIDHash`). No account data, no PII. | not started | _file PDF_ | _yyyy-mm-dd_ | _yyyy-mm-dd_ | Click attribution only; users hit `prf.hn` then are forwarded to the partner site. Confirm the publisher-program DPA scope covers all three Expedia brands under our single camref. |
| Skyscanner Ltd. | Affiliate — flights | Outbound click metadata, `associateid` + `utm_content` (hashed `tripIDHash`) | not started | _file PDF_ | _yyyy-mm-dd_ | _yyyy-mm-dd_ | No personal data shared at click time; standard affiliate-program terms apply. |
| Booking.com B.V. | Affiliate — hotels | Outbound click metadata, `aid` + `label` (hashed `tripIDHash`) | not started | _file PDF_ | _yyyy-mm-dd_ | _yyyy-mm-dd_ | Affiliate program terms include data-processing language; verify a separate DPA isn't required for our use case. |
| GetYourGuide Deutschland GmbH | Affiliate — activities | Outbound click metadata, `partner_id` + `cmp` (hashed `tripIDHash`) | not started | _file PDF_ | _yyyy-mm-dd_ | _yyyy-mm-dd_ | EU-based partner — DPA expectation is higher; chase early. |
| Viator (Tripadvisor LLC) | Affiliate — activities (currently scaffolded; `PartnerViator` enum exists, no live URL builder integration yet but `viatorID` is plumbed in `internal/affiliate/affiliate.go`) | Will be: outbound click metadata once the Viator URL builder is added | not started | _file PDF_ | _yyyy-mm-dd_ | _yyyy-mm-dd_ | Listed in privacy.astro §6.1 ("GetYourGuide / Viator") but **not** listed in dpa.astro §5 — disclosure inconsistency, flagged in "Findings" below. |
| DiscoverCars (Discover Car Hire SIA) | Affiliate — car rentals | Outbound click metadata, `a_aid` | not started | _file PDF_ | _yyyy-mm-dd_ | _yyyy-mm-dd_ | EU-based (Latvia) — chase DPA early. |
| SafetyWing Inc. | Affiliate — travel insurance | Outbound click metadata, `referenceID` | not started | _file PDF_ | _yyyy-mm-dd_ | _yyyy-mm-dd_ | Insurance is the weakest non-affiliate-alternative category (see `internal/affiliate/sources.go::InsuranceSources`); SafetyWing remains the default free-tier affiliate. |

---

## Playbook

### 1. When to add a new sub-processor

Use this checklist any time you wire a new third-party integration that touches personal data (or any third party at all that's plausibly going to see request metadata, IPs, or user content):

- [ ] **Get the DPA.** Download from the vendor's legal page or request from their privacy team. Verify it references GDPR Art. 28 obligations, lists Standard Contractual Clauses for any non-EEA transfer, and includes a sub-sub-processor clause.
- [ ] **Sign and file the PDF.** Save to the canonical filing location (see §3 below). Filename convention: `YYYY-MM-DD-<vendor-slug>-dpa.pdf` (e.g. `2026-05-12-stripe-dpa.pdf`).
- [ ] **Update `privacy.astro` §5** ("Third-Party Services" table) with the vendor row — name (with privacy-policy link), purpose, data shared.
- [ ] **Update `dpa.astro` §5** ("Sub-Processors" table) with the same vendor row — name, purpose, location.
- [ ] **Update this register** (`docs/compliance/dpa-register.md`) — set DPA status, signed date, renewal due, file path.
- [ ] **Honor the customer DPA's "Sub-processor change notification" obligation** (`dpa.astro` §5, last paragraph: 14-day advance notice with a right-to-object). For an additive change, send notice to all customers with a signed customer-facing DPA via `privacy@toqui.travel`. Track the notice date in the "Notes" column of this register.
- [ ] **Open a PR** that bundles all four file updates so the change is reviewable as one diff.

### 2. Annual review (Q1)

Each Q1, walk this register top-to-bottom:

- [ ] Confirm each `signed` row is still in force (vendor hasn't terminated; we haven't migrated off).
- [ ] Re-check renewal-due dates — most DPAs auto-renew, but a few vendors (Stripe historically, Cloudflare since the May 2024 update) require accepting a refreshed addendum.
- [ ] Re-fetch each vendor's published DPA and diff against the on-file copy. If terms have materially changed (sub-sub-processors, transfer mechanism, retention defaults), re-sign and update the row.
- [ ] Sweep `pending` and `not started` rows — for each, decide: chase the signature now, or remove the integration if it isn't load-bearing.
- [ ] Verify privacy.astro and dpa.astro still match this register row-for-row.
- [ ] Update the "Last reviewed" line below.

**Last reviewed**: _yyyy-mm-dd by Privacy Officer_

### 3. Where to file the signed PDFs

Recommended canonical store, in priority order:

1. **Google Drive folder** — `Toqui / Compliance / Sub-Processor DPAs/` (private, restricted to Galloway Software Solutions Inc. internal Workspace users).
   Placeholder Drive path: `https://drive.google.com/drive/folders/<TODO-create-folder-and-paste-link>`
2. **1Password vault** — vault name `Toqui — Compliance`, item type "Document", one item per signed DPA. Cross-reference the Google Drive URL in the 1Password item's notes field so the two stores stay linked.

Both stores, never just one. Drive is the working copy used during incident response and customer DPA reviews; 1Password is the survivable backup if Workspace access is ever lost.

The git repo (`toqui-backend/docs/compliance/`) is the right home for this register and any redacted excerpts (e.g. for sharing with auditors) but is **not** the right home for raw signed PDFs — DPA PDFs sometimes contain counterparty business contact details we should not commit.

---

## Findings to follow up

The following inconsistencies were noted while assembling this register. They are documented here so the next person to touch this file has the context, but **fixing them is out of scope for this scaffold PR** — they belong in a separate compliance-content PR against `toqui-site`:

1. **Cloudflare missing from public sub-processor lists.** Cloudflare Pages serves `toqui.travel` and `app.toqui.travel`'s marketing site + admin panel, and CDN access logs are personal-data-adjacent (IPs, request paths). Cloudflare must be added to both `privacy.astro` §5 and `dpa.astro` §5.
2. **Viator listed in privacy.astro but not dpa.astro.** `privacy.astro` §6.1 says "GetYourGuide / Viator". The `dpa.astro` §5 sub-processor table doesn't include Viator. Either add Viator to the DPA sub-processor table, or remove the Viator mention from privacy.astro until the integration goes live (the `PartnerViator` enum and `viatorID` config field exist in `internal/affiliate/affiliate.go` but no `ViatorURL` builder is wired into the source pools yet).
3. **Sentry listed as a backend processor in privacy/DPA but only used by the frontend.** Privacy.astro §1.8 and the §5 table list Sentry as if the backend reports to it. Today only the Expo app and admin panel do (`toqui/app/_layout.tsx`, `toqui/components/ErrorBoundary.tsx`). The disclosure is still accurate (the user-facing app does send errors to Sentry) but the row description should make it clear the backend is not in scope yet, or the backend should actually be wired to Sentry to match the disclosure.
