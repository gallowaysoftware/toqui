# Toqui Analytics Plan

## Executive Summary

**Recommendation: PostHog Cloud (free tier) + lightweight backend event forwarding.**

PostHog is the clear winner for Toqui's current stage. It offers the best combination of React Native/Expo support, privacy controls, backend Go SDK, session replay, and a free tier that will last well past 10K users. The alternatives either lack critical features (Plausible, Umami), carry GDPR risk (GA4), or solve a narrower problem (Mixpanel, Amplitude).

This document covers the full evaluation, the specific events Toqui should track, what NOT to track, and the first dashboard to build.

---

## Tool Comparison

### 1. PostHog Cloud

**What it is:** All-in-one product analytics platform: events, funnels, session replay, feature flags, A/B testing, surveys.

**React Native + Expo compatibility:** Strong. The `posthog-react-native` SDK (~29K weekly npm downloads) has first-class Expo support. Installation is straightforward with `npx expo install posthog-react-native expo-file-system expo-application expo-device expo-localization`. For React Native Web, swap `expo-file-system` for `@react-native-async-storage/async-storage`. The SDK has been actively maintained through 2026.

**Session replay on mobile:** Yes, it actually works. PostHog ships a separate `posthog-react-native-session-replay` package. It supports both wireframe mode and screenshot mode. Requires development builds (no Expo Go). Android API 26+ and iOS 13+ required. Keyboard input is not captured (which is actually a privacy benefit for a travel app). 2,500 mobile recordings/month free.

**Backend Go integration:** Official Go SDK (`github.com/posthog/posthog-go`). Uses internal queue with async batching. Non-blocking. Pass `distinct_id` + event name + properties. Production-ready for Cloud Run.

**GDPR compliance:** EU Cloud hosting available (Frankfurt). Supports cookieless tracking mode. Property redaction via `sanitize_properties`. You can mask sensitive fields before they leave the device. PostHog is privacy-friendly by design but requires configuration to be fully GDPR-compliant. **Grade: B+** (good tooling, but you must configure it correctly).

**Self-hosted option:** Exists but NOT recommended for Toqui. Requires minimum 4 vCPU / 16GB RAM. There are documented cases of PostHog becoming unresponsive on 8GB/2-core GCP instances within 30 minutes. This would cost $150-300/month on GCP just for the analytics VM, which exceeds Toqui's entire current infrastructure budget. The "hobby" Docker deployment (4GB RAM) handles ~100K events/month but is unsupported and unreliable.

**Pricing:**

| Scale             | Events/mo | Cost/mo | Notes                   |
| ----------------- | --------- | ------- | ----------------------- |
| Current (3 users) | ~5K       | $0      | Well within free tier   |
| 1K users          | ~200K     | $0      | Still free              |
| 10K users         | ~2M       | ~$50    | $0.00005/event after 1M |
| 100K users        | ~20M      | ~$950   | Volume discounts apply  |

Session replay adds $0.005/recording after 5K/month. Feature flags add $0.0001/request after 1M/month.

**Setup effort:** 4-8 hours for full integration (frontend SDK + backend Go SDK + first dashboard).

| Criterion              | Rating                 |
| ---------------------- | ---------------------- |
| Setup effort           | 4-8 hours              |
| Cost (3 users)         | $0                     |
| Cost (10K users)       | ~$50/mo                |
| Cost (100K users)      | ~$950/mo               |
| GDPR compliance        | B+                     |
| RN + Expo compat       | A                      |
| Backend Go integration | A                      |
| Insight richness       | A (full behavioral)    |
| Session replay         | A (web), B+ (mobile)   |
| Self-hosting           | D (too resource-heavy) |

---

### 2. Plausible

**What it is:** Lightweight, privacy-first web analytics. Page views, referrers, UTM tracking, basic custom events. No user-level tracking.

**React Native compatibility:** None. No SDK exists. Web-only via a `<script>` tag. You could hit the Plausible API directly from React Native, but you would be building your own SDK from scratch with no session management, no device context, no offline queuing.

**Backend Go integration:** HTTP API exists but it is designed for page-view-like events, not arbitrary backend events.

**GDPR compliance:** A+ by design. No cookies, no personal data, no consent banner needed. EU-hosted cloud. This is its primary selling point.

**Self-hosted:** Easy. Runs on a $5/month VPS. Docker Compose. Minimal resources (512MB RAM is fine). Community Edition is free under AGPL.

**What it cannot do:** Funnels, cohort analysis, user-level tracking, session replay, feature flags, A/B testing. You get aggregate page-level data only. You cannot answer "what did user X do?" or "where do users drop off in onboarding?"

**Pricing:** Cloud starts at $9/month for 10K page views. Self-hosted is free (just hosting costs).

| Criterion              | Rating                    |
| ---------------------- | ------------------------- |
| Setup effort           | 1-2 hours (web only)      |
| Cost (3 users)         | $0 (self-hosted) or $9/mo |
| Cost (10K users)       | $19/mo                    |
| Cost (100K users)      | $69/mo                    |
| GDPR compliance        | A+                        |
| RN + Expo compat       | F                         |
| Backend Go integration | D                         |
| Insight richness       | D (page views only)       |
| Session replay         | F                         |
| Self-hosting           | A                         |

**Verdict:** Plausible solves the wrong problem. At 3 users, you need to understand _what users do_, not _how many visitors you have_. Plausible is excellent for marketing sites (and could be used on `toqui-site`), but it is insufficient as the primary analytics tool for the app.

---

### 3. Mixpanel

**What it is:** Event-based product analytics with excellent funnels, cohorts, and retention analysis.

**React Native compatibility:** Mature SDK (`mixpanel-react-native`). Wraps native iOS/Android SDKs. Supports offline tracking. Works with Expo.

**Backend Go integration:** Official Go SDK (`github.com/mixpanel/mixpanel-go`). Track events, set user profiles, group analytics. One caveat: server-side geolocation defaults to your server's IP (not the user's), so you need to forward the user's IP or set geo properties manually.

**GDPR compliance:** US-based. EU data residency available on paid plans. No cookieless mode by default (uses device ID). Requires consent management. **Grade: B.** Workable but requires more effort than PostHog for GDPR.

**Session replay:** None. This is the biggest gap. You cannot see what users actually do.

**Free tier:** 20M events/month. This is extremely generous and will cover Toqui well past 100K users.

**Pricing:** Free up to 20M events. Growth plan starts at $24/month. The free tier is so generous that cost is essentially a non-issue until Toqui is a substantial business.

| Criterion              | Rating                          |
| ---------------------- | ------------------------------- |
| Setup effort           | 4-6 hours                       |
| Cost (3 users)         | $0                              |
| Cost (10K users)       | $0                              |
| Cost (100K users)      | $0 (still under 20M)            |
| GDPR compliance        | B                               |
| RN + Expo compat       | A                               |
| Backend Go integration | A                               |
| Insight richness       | A (funnels, cohorts, retention) |
| Session replay         | F                               |
| Self-hosting           | F (no option)                   |

**Verdict:** Strong contender. The 20M free tier is hard to beat. If session replay is not important, Mixpanel is the most cost-effective choice. However, for a travel app with complex multi-screen flows (onboarding, trip creation, AI chat, itinerary editing, checkout), session replay is extremely valuable for debugging and understanding behavior. Mixpanel loses on this.

---

### 4. Amplitude

**What it is:** Enterprise-grade product analytics. Similar to Mixpanel but more focused on behavioral cohorting and experimentation.

**React Native compatibility:** Official SDK exists (`@amplitude/react-native`). Works with Expo.

**GDPR compliance:** EU data center option available. Similar posture to Mixpanel. **Grade: B.**

**Free tier:** Starter plan includes basic analytics, session replay (limited), and unlimited feature flags. Free plan details are opaque compared to competitors. Paid plans start at $49/month.

**Session replay:** Available on the free plan (limited) and paid plans. However, mobile session replay support is less mature than PostHog's.

| Criterion              | Rating                           |
| ---------------------- | -------------------------------- |
| Setup effort           | 4-6 hours                        |
| Cost (3 users)         | $0                               |
| Cost (10K users)       | ~$49/mo                          |
| Cost (100K users)      | ~$500+/mo (opaque)               |
| GDPR compliance        | B                                |
| RN + Expo compat       | B+                               |
| Backend Go integration | B (HTTP API, no official Go SDK) |
| Insight richness       | A                                |
| Session replay         | B (mobile less mature)           |
| Self-hosting           | F                                |

**Verdict:** Amplitude is overkill for Toqui's stage. The pricing is opaque and enterprise-oriented. The sales-driven model means you will spend time in demos and negotiations before understanding true costs. PostHog and Mixpanel both offer more transparent pricing and better developer experience for a startup.

---

### 5. Google Analytics 4

**What it is:** Google's analytics platform. Free, unlimited events. Deeply integrated with Google Ads ecosystem.

**React Native compatibility:** Works via Firebase SDK (`@react-native-firebase/analytics`). Firebase adds significant native dependencies and complexity to an Expo project. Requires custom dev builds, config files for each platform, and Firebase project setup.

**GDPR compliance: F.** Multiple EU DPAs (Austria, France, Italy, Norway, Sweden) have ruled GA4 usage non-compliant with GDPR. The core issue is that data is transferred to US servers and used by Google for ad targeting. The EU-US Data Privacy Framework remains in jeopardy. For a travel app that handles destination data (which can reveal religion, health status, political affiliation, sexuality), using GA4 is a material legal risk.

**Session replay:** None.

| Criterion              | Rating                      |
| ---------------------- | --------------------------- |
| Setup effort           | 8-16 hours (Firebase setup) |
| Cost (3 users)         | $0                          |
| Cost (10K users)       | $0                          |
| Cost (100K users)      | $0                          |
| GDPR compliance        | F                           |
| RN + Expo compat       | C (Firebase complexity)     |
| Backend Go integration | B (Measurement Protocol)    |
| Insight richness       | B+                          |
| Session replay         | F                           |
| Self-hosting           | F                           |

**Verdict:** Hard no. The GDPR risk alone disqualifies GA4 for a travel app with global users. Travel destination data is quasi-sensitive. A pilgrimage to Mecca reveals religion. A trip to a fertility clinic reveals health data. EU regulators have specifically targeted GA4. The "free" pricing is subsidized by Google using your users' data for advertising. This is antithetical to Toqui's interests.

---

### 6. Custom Backend Logging (Cloud Logging + BigQuery)

**What it is:** Log structured events from the Go backend to Cloud Logging, pipe them to BigQuery, build dashboards in Looker Studio.

**Advantages:** Full data ownership, zero external dependencies, no vendor lock-in, no privacy concerns (data stays in your GCP project). You already have structured `slog` in the backend.

**Disadvantages:** No frontend event tracking without building a custom ingestion endpoint. No session replay. No pre-built funnels, cohorts, or retention charts. Every report must be hand-built in SQL. No autocapture. Significant ongoing engineering cost.

**Cost:** Cloud Logging free tier is 50 GiB/month. BigQuery first 1 TB of queries/month is free, 10 GB storage free. Effectively $0 at Toqui's scale.

| Criterion              | Rating               |
| ---------------------- | -------------------- |
| Setup effort           | 40-80 hours          |
| Cost (3 users)         | $0                   |
| Cost (10K users)       | ~$5/mo               |
| Cost (100K users)      | ~$30/mo              |
| GDPR compliance        | A (full control)     |
| RN + Expo compat       | N/A (custom)         |
| Backend Go integration | A (native)           |
| Insight richness       | C (what you build)   |
| Session replay         | F                    |
| Self-hosting           | A (it IS your infra) |

**Verdict:** This is a trap. It sounds appealing ("own your data!") but the engineering cost is enormous and grows over time. Every new question requires writing SQL. You will either spend weeks building a mediocre version of PostHog or you will stop looking at analytics because the friction is too high. The free tiers of PostHog and Mixpanel make this approach hard to justify. Use Cloud Logging for ops/debugging (you already do), but do not build an analytics platform.

---

### 7. Umami

**What it is:** Open-source, self-hosted analytics. Privacy-first like Plausible but with custom event support.

**React Native compatibility:** No SDK. Web only. Several community React packages exist, but nothing for React Native. You would need to hit the HTTP API directly.

**Backend Go integration:** HTTP API only. No SDK.

**Custom events:** Supported via JavaScript API or HTTP API. More capable than Plausible.

**Self-hosted:** Easy. Runs on minimal resources. PostgreSQL or MySQL backend. Docker Compose.

| Criterion              | Rating                                 |
| ---------------------- | -------------------------------------- |
| Setup effort           | 4-8 hours (web), 16+ hours (RN custom) |
| Cost (3 users)         | $0 (self-hosted)                       |
| Cost (10K users)       | ~$5/mo (hosting)                       |
| Cost (100K users)      | ~$15/mo (hosting)                      |
| GDPR compliance        | A                                      |
| RN + Expo compat       | D (no SDK)                             |
| Backend Go integration | C (HTTP only)                          |
| Insight richness       | C+                                     |
| Session replay         | F                                      |
| Self-hosting           | A                                      |

**Verdict:** Similar problem to Plausible. Good for a marketing site, insufficient for an app. The lack of a React Native SDK is a dealbreaker for Toqui's primary use case.

---

## Comparative Summary

| Tool        | RN/Expo | Go Backend | GDPR | Session Replay | Free Tier     | Best For                          |
| ----------- | ------- | ---------- | ---- | -------------- | ------------- | --------------------------------- |
| **PostHog** | A       | A          | B+   | A              | 1M events/mo  | **Full-stack product analytics**  |
| Plausible   | F       | D          | A+   | F              | Self-host     | Marketing site traffic            |
| Mixpanel    | A       | A          | B    | F              | 20M events/mo | Event analytics without replay    |
| Amplitude   | B+      | B          | B    | B              | Opaque        | Enterprise product teams          |
| GA4         | C       | B          | F    | F              | Unlimited     | Ad-driven businesses (not travel) |
| Custom      | N/A     | A          | A    | F              | $0            | Ops logging (already doing this)  |
| Umami       | D       | C          | A    | F              | Self-host     | Privacy-first web analytics       |

---

## The Recommendation

### Phase 1 (Now): PostHog Cloud, free tier

**Why PostHog over Mixpanel?** Mixpanel's 20M free events is generous, but PostHog gives you session replay, feature flags, and A/B testing in one tool. At 3 users becoming 1,000 users, you need to _watch real sessions_ to understand behavior, not just count events. Session replay is how you discover that users tap the wrong button, get confused by the itinerary layout, or abandon the onboarding flow. Mixpanel cannot show you this.

**Why cloud over self-hosted?** Self-hosted PostHog needs 4 vCPU / 16GB RAM minimum, which costs more than Toqui's entire infrastructure. PostHog Cloud free tier gives you 1M events/month and 5K session recordings. Toqui will not exceed this until well past 10K active users.

**Why not both PostHog + Plausible?** Plausible is excellent for `toqui-site` (the marketing site). If you want privacy-first page-level analytics on the marketing site without any cookies, self-host Plausible CE alongside it. But for the app itself, PostHog is the tool.

### Phase 2 (10K+ users): Evaluate costs

At 10K users (~2M events/month), PostHog will cost ~$50/month. This is still reasonable. If costs become a concern, you can:

1. Reduce event volume by being more selective about what you track
2. Reduce session replay sample rate (record 10% of sessions instead of 100%)
3. Consider Mixpanel for event analytics + a separate session replay tool

### Phase 3 (100K+ users): Re-evaluate

At this scale ($950+/month), analytics costs become material. This is the point to consider whether the all-in-one approach still makes sense or whether a specialized stack (Mixpanel events + a cheaper replay tool) would be more cost-effective. But this is a problem for a much later stage.

---

## Implementation Plan

### Frontend Setup (PostHog React Native SDK)

Install the SDK:

```bash
npx expo install posthog-react-native expo-file-system expo-application expo-device expo-localization
```

For web support, also install:

```bash
npx expo install @react-native-async-storage/async-storage
```

Add the PostHog provider in `app/_layout.tsx` (inside the existing provider stack, after `ThemeProvider` and before `AuthProvider`):

```typescript
import { PostHogProvider } from "posthog-react-native";

// In the provider stack:
// ThemeProvider -> I18nProvider -> QueryClientProvider -> PostHogProvider -> AuthProvider -> ...
```

### Backend Setup (PostHog Go SDK)

Install in `toqui-backend`:

```bash
go get github.com/posthog/posthog-go
```

Initialize once in the backend's main/startup:

```go
import "github.com/posthog/posthog-go"

client := posthog.NewWithConfig(
    os.Getenv("POSTHOG_API_KEY"),
    posthog.Config{Endpoint: "https://eu.i.posthog.com"}, // EU endpoint
)
defer client.Close()
```

### GDPR Configuration

1. Use PostHog's **EU Cloud** (Frankfurt data center)
2. Enable **cookieless tracking** for web (uses session ID instead of cookies)
3. Configure `sanitize_properties` to strip any PII from event properties before they leave the device
4. Implement a consent mechanism: only enable PostHog after user consents to analytics
5. Add a "Delete my data" button in Settings that calls PostHog's deletion API

---

## What to Track (and Why)

### Core Events

These are the events that answer the questions that matter for Toqui's growth.

#### Onboarding & Activation

| Event Name           | Properties                                                       | Why It Matters                                                         |
| -------------------- | ---------------------------------------------------------------- | ---------------------------------------------------------------------- |
| `onboarding_started` | `source` (organic, referral, ad)                                 | Measures top-of-funnel. How are people finding Toqui?                  |
| `age_gate_completed` | `age_verified: bool`                                             | What percentage of users bounce at the age gate? Is it too aggressive? |
| `signup_completed`   | `method` (google), `referral_code`                               | Conversion from visit to signup. Referral attribution.                 |
| `first_trip_created` | `destination_count`, `date_range_days`, `time_to_create_seconds` | THE activation metric. A user who creates a trip is activated.         |
| `first_message_sent` | `trip_id`, `time_since_trip_creation_seconds`                    | Second activation signal. User engages with the AI companion.          |

#### Trip Lifecycle

| Event Name           | Properties                                                      | Why It Matters                                      |
| -------------------- | --------------------------------------------------------------- | --------------------------------------------------- |
| `trip_created`       | `trip_number` (1st, 2nd, etc), `has_dates`, `destination_count` | Volume and repeat usage.                            |
| `trip_deleted`       | `trip_age_days`, `message_count`, `itinerary_item_count`        | Are users deleting trips? Why? Low-value indicator. |
| `trip_shared`        | `method` (link, copy)                                           | Sharing is the primary growth loop. Track it.       |
| `shared_trip_viewed` | `has_account: bool`                                             | Do shared trip viewers convert to signups?          |

#### AI Chat Engagement

| Event Name               | Properties                                            | Why It Matters                                                                   |
| ------------------------ | ----------------------------------------------------- | -------------------------------------------------------------------------------- |
| `message_sent`           | `trip_number`, `message_number_in_session`, `persona` | Chat volume and depth. Are users having long conversations or one-off questions? |
| `recommendation_shown`   | `category` (hotel, flight, activity), `position`      | How often does the AI surface recommendations?                                   |
| `recommendation_clicked` | `category`, `position`, `affiliate_partner`           | Revenue-critical. Which recommendations convert?                                 |
| `persona_switched`       | `from_persona`, `to_persona`                          | Are users using different expert personas? Which ones?                           |

#### Monetization

| Event Name             | Properties                                        | Why It Matters                       |
| ---------------------- | ------------------------------------------------- | ------------------------------------ |
| `upgrade_prompt_shown` | `trigger` (message_limit, persona_locked, export) | What pushes users toward paid?       |
| `upgrade_started`      | `trip_id`                                         | Checkout funnel entry.               |
| `upgrade_completed`    | `trip_id`, `payment_method`                       | Revenue event.                       |
| `upgrade_abandoned`    | `trip_id`, `step` (payment_form, confirmation)    | Where do users drop off in checkout? |

#### Retention & Export

| Event Name             | Properties                                 | Why It Matters                                               |
| ---------------------- | ------------------------------------------ | ------------------------------------------------------------ |
| `itinerary_viewed`     | `trip_id`, `item_count`, `days_until_trip` | Are users returning to review their itinerary before travel? |
| `itinerary_exported`   | `format` (pdf, ics), `item_count`          | Export signals high intent and satisfaction.                 |
| `app_opened`           | `days_since_last_open`, `trip_count`       | Retention tracking.                                          |
| `referral_code_shared` | `medium` (copy, share_sheet)               | Referral loop health.                                        |
| `referral_redeemed`    | `referrer_trip_count`                      | Do power users generate more referrals?                      |

#### Backend-Only Events (Go SDK)

| Event Name                 | Properties                                                   | Why It Matters                                                   |
| -------------------------- | ------------------------------------------------------------ | ---------------------------------------------------------------- |
| `ai_response_generated`    | `persona`, `token_count`, `latency_ms`, `has_recommendation` | AI quality and cost monitoring. Cannot be tracked from frontend. |
| `ai_tool_executed`         | `tool_name`, `success: bool`, `latency_ms`                   | Which AI tools are being used? Are they failing?                 |
| `itinerary_auto_created`   | `item_count`, `trip_id`                                      | AI-generated itinerary tracking.                                 |
| `payment_webhook_received` | `status`, `amount`                                           | Server-side payment confirmation (source of truth).              |
| `token_refreshed`          | `user_id`                                                    | Auth health monitoring.                                          |

---

## What NOT to Track

This is critical for a travel app. Travel data is inherently sensitive.

### Hard Rules (Never Track These)

| Data                             | Why Not                                                                                                                                                                    |
| -------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Destination names/cities**     | Reveals religion (Mecca, Vatican), health (Mayo Clinic), politics (protest sites), sexuality (Pride events). Track `destination_count`, never the destinations themselves. |
| **Chat message content**         | Users discuss personal travel plans, health needs, dietary restrictions, family situations. Track `message_count`, never the text.                                         |
| **Specific dates of travel**     | Combined with destination, reveals detailed personal life. Track `date_range_days` (duration), not actual dates.                                                           |
| **Hotel/flight names**           | Reveals economic status and location patterns. Track `category` (hotel, flight, activity), not specific names.                                                             |
| **User IP addresses**            | Configure PostHog to discard IPs. Not needed for analytics.                                                                                                                |
| **Precise geolocation**          | Never track user's current location. Not relevant for analytics.                                                                                                           |
| **Booking confirmation numbers** | Financial/PII data. Not needed.                                                                                                                                            |
| **Travel companion names**       | PII of third parties who have not consented.                                                                                                                               |

### Soft Rules (Track Aggregates Only)

| Instead Of                                  | Track                                           |
| ------------------------------------------- | ----------------------------------------------- |
| Which persona the user chatted with by name | `persona_category` (food, history, adventure)   |
| Exact itinerary items                       | `itinerary_item_count`, `category_distribution` |
| Referral code values                        | `has_referral: bool`, `referral_source_type`    |

### Session Replay Privacy

Configure PostHog session replay to:

1. **Mask all text inputs** by default (chat messages, search queries, trip names)
2. **Mask itinerary content** (destination names, hotel names, dates)
3. **Only show UI interaction patterns** (taps, scrolls, navigation flow)
4. The goal is to see _how_ users interact with the UI, not _what_ they type or plan

---

## First Dashboard: "Toqui Pulse"

Build this dashboard in PostHog on day one. It answers the five questions that matter most right now.

### Row 1: Acquisition

- **Signups this week** (number, trend line)
- **Signup source breakdown** (pie: organic vs referral vs shared_trip)
- **Age gate pass rate** (percentage)

### Row 2: Activation

- **Trip creation rate** (signups who create a trip within 24 hours)
- **First message rate** (trip creators who send a message within 1 hour)
- **Time to first trip** (median seconds from signup to trip creation)

### Row 3: Engagement

- **Messages per user per day** (line chart, 7-day rolling)
- **Active users** (DAU / WAU ratio)
- **Recommendation click-through rate** (recommendations clicked / shown)

### Row 4: Monetization

- **Upgrade funnel** (prompt shown -> started -> completed, funnel chart)
- **Revenue this month** (number)
- **Upgrade trigger breakdown** (what feature limit pushed them to pay)

### Row 5: Retention

- **Retention cohort** (week-over-week return rate, heatmap)
- **Trips per user distribution** (histogram: 1 trip, 2 trips, 3+ trips)
- **Export rate** (users who export itinerary / users with 3+ day trips)

---

## Implementation Checklist

1. [ ] Create PostHog account (EU Cloud)
2. [ ] Install `posthog-react-native` in Toqui app
3. [ ] Add PostHogProvider to `app/_layout.tsx` provider stack
4. [ ] Configure cookieless tracking for web
5. [ ] Configure session replay with text masking
6. [ ] Add `POSTHOG_API_KEY` env var to backend
7. [ ] Install `posthog-go` in toqui-backend
8. [ ] Implement 5 onboarding events (frontend)
9. [ ] Implement 4 trip lifecycle events (frontend)
10. [ ] Implement 4 chat engagement events (frontend)
11. [ ] Implement 4 monetization events (frontend)
12. [ ] Implement 5 backend-only events (Go SDK)
13. [ ] Build "Toqui Pulse" dashboard
14. [ ] Add consent mechanism (GDPR opt-in before PostHog initializes)
15. [ ] Add "Delete my data" button in Settings screen
16. [ ] Document privacy configuration in CLAUDE.md

**Estimated total effort:** 2-3 days of engineering.
**Estimated monthly cost:** $0 for the foreseeable future.
