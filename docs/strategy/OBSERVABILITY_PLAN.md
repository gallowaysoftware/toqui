# Toqui Observability Platform Evaluation

**Date:** 2026-04-02
**Author:** Engineering
**Status:** Proposal

## Context

Toqui needs unified observability covering frontend JS errors, backend logs/metrics/traces, and alerting. Currently we have GCP Cloud Monitoring uptime checks from Terraform and PostHog for product analytics (not operational monitoring). Monthly infra budget is ~$100; observability should not double it.

---

## Option 1: OpenObserve

**What it is:** Open-source observability platform (Rust) covering logs, metrics, traces, and RUM in a single binary. Claims 140x lower storage cost than Elasticsearch.

### Frontend Coverage

OpenObserve has a browser RUM SDK (`@openobserve/browser-rum` + `@openobserve/browser-logs`) that captures JS errors with stack traces, session context, Core Web Vitals, and session replay. This is surprisingly complete for an open-source tool. However, there is NO React Native or Expo SDK -- the RUM SDK is browser-only. Since Toqui serves the web build via nginx on Cloud Run, the browser SDK would work for web users but not for future native iOS/Android builds.

### Backend Coverage

Excellent. OpenTelemetry-native, so Go instrumentation via OTel SDK sends logs, metrics, and traces directly to OpenObserve. Structured JSON logs from `slog` can be ingested via OTel Collector or direct HTTP API.

### Self-Hosting Requirements

Single-binary deployment. Minimum viable: 1 vCPU, 2GB RAM for light usage. Can run on a single Cloud Run service or a small GCE VM (e2-small ~$15/month). Storage on GCS (object storage) keeps costs low. HA mode requires more resources but is unnecessary at current scale.

### Cloud Pricing

As of June 2025, OpenObserve eliminated the free tier. Cloud pricing is $0.30/GB ingested (logs, metrics, traces combined) with no per-host charges. No minimums.

### Cost Estimates

| Scale         | Self-Hosted (GCE)              | Cloud                     |
| ------------- | ------------------------------ | ------------------------- |
| 3 users (dev) | ~$15-20/month (e2-small + GCS) | ~$5-15/month (low volume) |
| 1K users      | ~$25-40/month (e2-medium)      | ~$30-60/month             |
| 10K users     | ~$50-80/month (e2-standard-2)  | ~$100-200/month           |

### Alerting

Built-in alerting with Slack, email, and webhook destinations. Functional but less mature than Grafana or Datadog.

### Verdict

Strong contender for single-pane-of-glass at low cost. The RUM SDK covers web frontend errors adequately. Main risks: (1) no React Native SDK for future native builds, (2) smaller community than Grafana ecosystem, (3) self-hosting adds operational burden. No free cloud tier is a downside.

| Criterion               | Score                  |
| ----------------------- | ---------------------- |
| Setup effort            | 4-8 hours              |
| Single pane of glass    | YES                    |
| OTel compatibility      | Excellent              |
| Go SDK quality          | Good (via OTel)        |
| React Native / Expo     | Web only (browser SDK) |
| Self-hosting difficulty | Low-moderate           |

---

## Option 2: Datadog

**What it is:** Industry-standard SaaS observability platform. Best-in-class UX for logs, metrics, traces, RUM, error tracking, and alerting.

### Frontend Coverage

Datadog RUM SDK works with React web apps. Has a React Native SDK for mobile. Source map upload is supported. Session replay, error tracking, and user journey analysis are all excellent.

### Backend Coverage

Best in class. Go `dd-trace-go` agent is mature. Structured logs, APM traces, custom metrics all work seamlessly. The correlation between logs, traces, and infrastructure metrics is where Datadog truly excels.

### Cloud Pricing (the problem)

Datadog's pricing is per-host, per-feature, and gets expensive fast:

- **Infrastructure:** $15-18/host/month
- **APM:** $31-36/host/month
- **Logs:** ingestion + retention-based (retention beyond 15 days costs extra)
- **RUM:** $1.50 per 1,000 sessions/month
- **Custom metrics:** $0.05 per custom metric per month (adds up fast)

| Scale         | Estimated Monthly Cost                                            |
| ------------- | ----------------------------------------------------------------- |
| 3 users (dev) | $50-80/month (1 host, APM, logs, minimal RUM)                     |
| 1K users      | $200-400/month (RUM sessions, log volume, APM)                    |
| 10K users     | $500-1500/month (RUM alone: ~$150-450/month at 10K-300K sessions) |

The free tier (5 hosts, 1-day log retention) is essentially useless -- 1-day retention means you cannot investigate issues from yesterday.

### Alerting

Gold standard. PagerDuty, Slack, email, webhook integrations. Anomaly detection, composite alerts, SLO tracking.

### Verdict

Datadog is the best product but the worst fit for Toqui's budget. At $50-80/month just for dev, it would consume half the entire infra budget. At 10K users it could easily exceed the entire current infra spend. The pricing model punishes growth. Not recommended unless Toqui raises funding and can justify $500+/month for observability.

| Criterion               | Score                              |
| ----------------------- | ---------------------------------- |
| Setup effort            | 2-4 hours                          |
| Single pane of glass    | YES (best in class)                |
| OTel compatibility      | Good (also has proprietary agents) |
| Go SDK quality          | Excellent                          |
| React Native / Expo     | Good (RUM + React Native SDK)      |
| Self-hosting difficulty | N/A (SaaS only)                    |

---

## Option 3: Grafana Cloud (Free Tier) + Sentry

**What it is:** Two tools that together cover the full observability stack. Grafana Cloud handles backend logs/metrics/traces; Sentry handles frontend error tracking.

### Grafana Cloud Free Tier

- 10,000 active metric series
- 50 GB logs/month
- 50 GB traces/month
- 3 users
- 14-day retention (metrics: 13 months)
- Includes Loki (logs), Prometheus (metrics), Tempo (traces)

### Sentry Free Tier

- 5,000 errors/month, 1 user
- Source map uploads (first-class Expo/React Native support via `@sentry/react-native`)
- Performance monitoring (transactions)
- Session replay (limited)

### Frontend Coverage

Sentry is THE gold standard for frontend error tracking. The `@sentry/react-native` SDK has first-class Expo support, including automatic source map upload via EAS Build. Stack traces, breadcrumbs, session context, device info -- all work out of the box. Expo's own documentation recommends Sentry.

### Backend Coverage

Grafana Cloud with OpenTelemetry: Go app instrumented with OTel SDK sends metrics to Prometheus (via remote write), logs to Loki (via OTel Collector or Grafana Agent/Alloy), and traces to Tempo. Structured `slog` JSON logs work naturally. Dashboards are highly customizable. Grafana's query language (LogQL, PromQL) has a learning curve but is very powerful.

### Cost Estimates

| Scale         | Grafana Cloud                       | Sentry                             | Total         |
| ------------- | ----------------------------------- | ---------------------------------- | ------------- |
| 3 users (dev) | $0 (within free tier)               | $0 (within free tier)              | $0            |
| 1K users      | $0-20/month (may stay in free tier) | $0-26/month (may exceed 5K errors) | $0-46/month   |
| 10K users     | $20-60/month (log volume increase)  | $26-80/month (Team/Business plan)  | $46-140/month |

### Alerting

Grafana has solid alerting (Slack, email, PagerDuty, webhook). Sentry has its own alerting for errors (spike detection, issue assignment). Two alert systems to configure, but both are competent.

### Downsides

- **Two dashboards, not one.** You look at Grafana for backend health and Sentry for frontend errors. Not a true single pane of glass.
- **Two tools to learn and maintain.** Two sets of alerts, two logins, two configurations.
- **Grafana Cloud free tier limits are per-organization.** If you exceed 10K metric series (easy to do with high-cardinality labels), you need the Pro plan ($0.008/series/month beyond included).
- **Sentry free tier is 1 user.** The Team plan ($26/month) is needed for multiple team members.

### Verdict

The best value proposition at early stage. Literally $0/month during dev with 3 users. Both tools are best-in-class in their respective domains. The tradeoff is no single pane of glass -- you check two places. At 10K users the combined cost is still very reasonable ($50-140/month). Sentry's Expo integration is unmatched.

| Criterion               | Score                                                |
| ----------------------- | ---------------------------------------------------- |
| Setup effort            | 4-6 hours (Grafana) + 1-2 hours (Sentry) = 6-8 hours |
| Single pane of glass    | NO (two tools)                                       |
| OTel compatibility      | Excellent (Grafana is OTel-native)                   |
| Go SDK quality          | Good (via OTel)                                      |
| React Native / Expo     | Excellent (Sentry is the standard)                   |
| Self-hosting difficulty | N/A (both SaaS free tiers)                           |

---

## Option 4: SigNoz

**What it is:** Open-source, OpenTelemetry-native observability platform (logs, metrics, traces). Competes with Datadog on features at a fraction of the cost.

### Frontend Coverage

Limited. SigNoz can ingest frontend logs/traces via OpenTelemetry browser SDK, but it lacks a dedicated RUM SDK, session replay, or source-map-aware error tracking. You would still need Sentry alongside for proper frontend error tracking, which negates the single-pane-of-glass advantage.

### Backend Coverage

Excellent. Built on ClickHouse for storage, fully OTel-native. Go instrumentation is standard OTel SDK. Structured logs, metrics, and traces all work well. The correlation between signals is good.

### Cloud Pricing

- Teams Cloud: $49/month base (includes $49 of usage)
- Usage: $0.30/GB logs, $0.30/GB traces, $0.10/million metric samples
- Startup Program: $19/month (if eligible)

### Self-Hosting

Requires ClickHouse + SigNoz services. Minimum: 4 vCPU, 8GB RAM, 100GB+ SSD. This is significantly more than OpenObserve's single-binary approach. On GCP, you would need at least an e2-standard-2 (~$50/month) plus persistent disk. Self-hosting SigNoz is a real operational commitment.

### Cost Estimates

| Scale         | Self-Hosted                | Cloud         |
| ------------- | -------------------------- | ------------- |
| 3 users (dev) | ~$50-70/month (GCE + disk) | $19-49/month  |
| 1K users      | ~$70-100/month             | $49-80/month  |
| 10K users     | ~$100-150/month            | $80-200/month |

### Verdict

SigNoz is a solid tool but occupies an awkward middle ground for Toqui. It costs more than Grafana Cloud free tier, requires Sentry alongside for frontend, and self-hosting demands meaningful resources. The cloud offering at $49/month base is reasonable but starts higher than alternatives that have free tiers. Best suited for teams that have outgrown Grafana Cloud limits and want to stay open-source.

| Criterion               | Score                                         |
| ----------------------- | --------------------------------------------- |
| Setup effort            | 6-10 hours (self-hosted) or 3-5 hours (cloud) |
| Single pane of glass    | Partial (needs Sentry for frontend)           |
| OTel compatibility      | Excellent (OTel-native)                       |
| Go SDK quality          | Good (via OTel)                               |
| React Native / Expo     | Poor (no dedicated SDK, need Sentry)          |
| Self-hosting difficulty | Moderate-high (ClickHouse is resource-hungry) |

---

## Option 5: Highlight.io

**DISQUALIFIED.** Highlight.io was acquired by LaunchDarkly in April 2025. The standalone Highlight.io service was deprecated on February 28, 2026. All infrastructure has been migrated to LaunchDarkly Observability. The self-hosted open-source version is no longer maintained.

LaunchDarkly Observability is a new product with uncertain pricing, feature parity, and long-term direction. Not recommended for new adoption. If Highlight's feature set (session replay + error tracking + logs) is appealing, evaluate LaunchDarkly Observability separately once it matures.

---

## Option 6: GCP-Native (Cloud Logging + Cloud Monitoring + Error Reporting)

**What it is:** Use what is already there. Cloud Run automatically sends structured logs to Cloud Logging. Cloud Monitoring provides metrics and uptime checks (already configured via Terraform). Cloud Error Reporting catches Go panics.

### Frontend Coverage

None. GCP has no frontend JS error tracking SDK. You would need Sentry alongside.

### Backend Coverage

Surprisingly good for the price:

- **Cloud Logging free tier:** 50 GB/month of log ingestion, 30-day retention. This is extremely generous for a startup. Toqui's Cloud Run logs are already flowing here.
- **Cloud Monitoring:** CPU, memory, request count, latency metrics are automatic for Cloud Run. Custom metrics: first 150 MB free, then $0.258/MB.
- **Cloud Error Reporting:** Free. Automatically catches Go panics and errors written to stderr.
- **Cloud Trace:** Distributed tracing with 5 million spans/month free.

### Cost Estimates

| Scale         | Monthly Cost                                 |
| ------------- | -------------------------------------------- |
| 3 users (dev) | $0 (well within free tiers)                  |
| 1K users      | $0-10/month (still within free tiers likely) |
| 10K users     | $10-30/month (may exceed some free tiers)    |

Plus Sentry free tier ($0) for frontend = still very cheap.

### Downsides

- **No unified dashboard.** You switch between Cloud Logging, Cloud Monitoring, Cloud Trace, Cloud Error Reporting -- four different GCP console pages.
- **GCP's observability UX is mediocre.** Log Explorer is functional but clunky compared to Grafana/Datadog. Building custom dashboards in Cloud Monitoring is tedious.
- **No session context for frontend.** Even with Sentry alongside, correlating a frontend error to a backend trace requires manual trace ID matching.
- **Alerting is basic.** Works for uptime checks and simple metric thresholds, but lacks anomaly detection or composite alerting.
- **Vendor lock-in.** Not OTel-native (though GCP is adding OTel support). Harder to migrate away later.

### Verdict

The cheapest option by far because most of it is already working. If the priority is "don't spend money and don't over-engineer," this is a defensible choice for the dev phase. Add Sentry for frontend. The tradeoff is poor UX and fragmented dashboards. As Toqui grows, you will want to migrate to a proper observability stack.

| Criterion               | Score                                      |
| ----------------------- | ------------------------------------------ |
| Setup effort            | 1-2 hours (mostly already done)            |
| Single pane of glass    | NO (4+ GCP console pages + Sentry)         |
| OTel compatibility      | Partial (improving but not native)         |
| Go SDK quality          | Decent (Cloud Logging client, Cloud Trace) |
| React Native / Expo     | None (need Sentry)                         |
| Self-hosting difficulty | N/A (managed)                              |

---

## Option 7: Better Stack (formerly Logtail)

**What it is:** Beautiful log aggregation + uptime monitoring + incident management SaaS. Good UX, reasonable pricing.

### Frontend Coverage

None. No JS error tracking, no RUM, no session replay. Needs Sentry alongside.

### Backend Coverage

Logs only. No metrics, no traces. You get searchable log aggregation with a nice UI, uptime monitoring, and incident alerting. But no request latency percentiles, no distributed tracing, no custom metrics dashboards.

### Free Tier

- 1 GB logs/month
- 10 uptime monitors (3-minute interval)
- Status page

### Cost Estimates

| Scale         | Better Stack              | Sentry       | Total         |
| ------------- | ------------------------- | ------------ | ------------- |
| 3 users (dev) | $0 (1GB free)             | $0           | $0            |
| 1K users      | $24-49/month (log volume) | $0-26/month  | $24-75/month  |
| 10K users     | $49-99/month              | $26-80/month | $75-179/month |

### Verdict

Better Stack is a good product for what it does (logs + uptime), but it leaves too many gaps for Toqui. No metrics, no traces, no frontend -- you would need Better Stack + Sentry + something for metrics + something for traces. That is four tools, which is worse than Option 3 (Grafana + Sentry at two tools). Not recommended as a primary observability solution.

| Criterion               | Score                    |
| ----------------------- | ------------------------ |
| Setup effort            | 2-3 hours                |
| Single pane of glass    | NO (logs only)           |
| OTel compatibility      | Partial (log ingestion)  |
| Go SDK quality          | N/A (log transport only) |
| React Native / Expo     | None                     |
| Self-hosting difficulty | N/A (SaaS only)          |

---

## Comparison Matrix

| Criterion        | OpenObserve | Datadog   | Grafana+Sentry | SigNoz    | Highlight | GCP-Native | Better Stack |
| ---------------- | ----------- | --------- | -------------- | --------- | --------- | ---------- | ------------ |
| Cost (3 users)   | $15-20      | $50-80    | $0             | $19-49    | DEAD      | $0         | $0           |
| Cost (1K users)  | $25-60      | $200-400  | $0-46          | $49-80    | DEAD      | $0-10      | $24-75       |
| Cost (10K users) | $50-200     | $500-1500 | $46-140        | $80-200   | DEAD      | $10-30     | $75-179      |
| Frontend errors  | Good (web)  | Excellent | Excellent      | Poor      | DEAD      | None       | None         |
| Backend logs     | Excellent   | Excellent | Excellent      | Excellent | DEAD      | Good       | Good         |
| Backend metrics  | Good        | Excellent | Excellent      | Excellent | DEAD      | Good       | None         |
| Backend traces   | Good        | Excellent | Excellent      | Excellent | DEAD      | Good       | None         |
| Single pane      | YES         | YES       | NO             | Partial   | DEAD      | NO         | NO           |
| Alerting         | Good        | Excellent | Good           | Good      | DEAD      | Basic      | Good         |
| OTel-native      | Yes         | Partial   | Yes            | Yes       | DEAD      | Partial    | Partial      |
| RN/Expo support  | Web only    | Good      | Excellent      | Poor      | DEAD      | None       | None         |
| Self-hostable    | Yes         | No        | Yes\*          | Yes       | DEAD      | No         | No           |
| Privacy/control  | Excellent   | Poor      | Good           | Excellent | DEAD      | Moderate   | Poor         |

\*Grafana stack is self-hostable but the free cloud tier is more practical at startup scale.

---

## Recommendation

### Phase 1 (Now, 3 users, dev/launch): Grafana Cloud + Sentry -- $0/month

**Why:** This is the pragmatic choice that optimizes for Toqui's actual constraints.

1. **$0/month.** Both tools have generous free tiers that will cover Toqui through launch and early growth. Grafana Cloud gives 50GB logs, 10K metrics, 50GB traces. Sentry gives 5K errors/month. This is more than enough for 3 users building a product.

2. **Sentry's Expo integration is unmatched.** The `@sentry/react-native` SDK has first-class Expo support with automatic source map upload via EAS Build. No other tool comes close for React Native error tracking. Since Toqui targets web + iOS + Android, this matters.

3. **Grafana is the industry standard for open-source observability.** PromQL and LogQL are transferable skills. Dashboards are infinitely customizable. The ecosystem (Loki, Prometheus, Tempo) is battle-tested at massive scale.

4. **OTel-native means no lock-in.** Instrument the Go backend with standard OpenTelemetry SDK once. If you later switch from Grafana Cloud to self-hosted Grafana, OpenObserve, SigNoz, or anything else, the instrumentation stays the same. Only the exporter config changes.

5. **Two tools is acceptable.** Yes, it is not a single pane of glass. But at 3 users, checking two dashboards is a 5-second context switch, not a productivity crisis. The $0 cost and best-in-class coverage in each domain outweighs the minor inconvenience.

### Implementation Plan

**Sentry (1-2 hours):**

1. Create Sentry project (React Native platform)
2. Install `@sentry/react-native` in Toqui
3. Initialize in `app/_layout.tsx` before other providers
4. Configure source map upload in `eas.json` / app config
5. Set up Slack alert for new error spikes

**Grafana Cloud (4-6 hours):**

1. Create Grafana Cloud account (free tier)
2. Instrument Go backend with OTel SDK (`go.opentelemetry.io/otel`)
3. Configure OTel exporter to send metrics (Prometheus remote write), logs (Loki), traces (Tempo) to Grafana Cloud endpoints
4. Add trace context propagation to the chat SSE streaming path
5. Build initial dashboard: request latency p50/p95/p99, error rate by endpoint, AI token usage
6. Set up alert rules: error rate spike, latency degradation, Cloud Run instance health

**Total setup: 6-8 hours of engineering.**

### Phase 2 (1K-10K users, if single pane becomes critical): Migrate to OpenObserve

If the two-tool approach becomes painful, OpenObserve is the natural upgrade path:

- Self-host on a small GCE VM ($20-40/month)
- It covers logs, metrics, traces, AND frontend RUM in one tool
- OTel instrumentation from Phase 1 transfers directly (change exporter config)
- Keep Sentry alongside for React Native native builds (OpenObserve RUM is browser-only)
- Total cost: $20-40/month (OpenObserve) + $0-26/month (Sentry) = $20-66/month

### Why NOT the other options

- **Datadog:** Too expensive. $50-80/month at dev stage is half the infra budget. Pricing punishes growth.
- **OpenObserve (as Phase 1):** Good tool but no free cloud tier, no React Native SDK. Spending $15-20/month to self-host when Grafana Cloud is free does not make sense at 3 users.
- **SigNoz:** Higher baseline cost ($49/month cloud or $50-70/month self-hosted). ClickHouse is resource-hungry. Still needs Sentry for frontend. Does not beat Grafana + Sentry on any dimension at startup scale.
- **GCP-Native:** Cheapest but worst UX. Fragmented across 4+ console pages. Not OTel-native. Would create technical debt that costs more to migrate away from later.
- **Better Stack:** Logs only. Too many gaps.
- **Highlight.io:** Dead. Acquired by LaunchDarkly, service deprecated Feb 2026.

---

## Key Decisions

| Decision                 | Choice                     | Rationale                                          |
| ------------------------ | -------------------------- | -------------------------------------------------- |
| Frontend error tracking  | Sentry                     | Gold standard for Expo/RN, free tier, source maps  |
| Backend logs             | Grafana Cloud (Loki)       | 50GB/month free, OTel-native                       |
| Backend metrics          | Grafana Cloud (Prometheus) | 10K series free, PromQL industry standard          |
| Backend traces           | Grafana Cloud (Tempo)      | 50GB/month free, correlates with Loki logs         |
| Alerting                 | Grafana + Sentry           | Both have Slack integration                        |
| Instrumentation standard | OpenTelemetry              | No vendor lock-in, portable across backends        |
| Self-hosting             | No (Phase 1)               | Free cloud tiers beat self-hosting cost at 3 users |

---

## Sources

- [OpenObserve Pricing (June 2025 update)](https://openobserve.ai/blog/june-25-pricing-policy-updates/)
- [OpenObserve RUM Documentation](https://openobserve.ai/docs/user-guide/rum/overview/)
- [Datadog Pricing](https://www.datadoghq.com/pricing/)
- [Datadog Cost Analysis (Last9)](https://last9.io/blog/datadog-pricing-all-your-questions-answered/)
- [Grafana Cloud Pricing](https://grafana.com/pricing/)
- [Grafana Cloud Usage Limits](https://grafana.com/docs/grafana-cloud/cost-management-and-billing/manage-invoices/understand-your-invoice/usage-limits/)
- [SigNoz Pricing](https://signoz.io/pricing/)
- [Sentry Expo Integration](https://docs.expo.dev/guides/using-sentry/)
- [Sentry Source Maps for Expo](https://docs.sentry.io/platforms/react-native/sourcemaps/uploading/expo/)
- [GCP Cloud Logging Pricing](https://cloud.google.com/stackdriver/pricing)
- [Better Stack Pricing](https://betterstack.com/pricing)
- [Highlight.io LaunchDarkly Migration](https://nodejs.highlight.io/blog/launchdarkly-migration)
