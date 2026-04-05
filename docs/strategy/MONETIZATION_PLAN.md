# Toqui Monetization Plan

**Document owner:** CFO, Toqui
**Last updated:** 2026-04-02
**Status:** Strategic planning document
**Currency:** All figures in CAD unless noted

---

## Executive Summary

Toqui currently generates revenue through a single channel: a $12 CAD per-trip unlock (Trip Pro). This plan lays out a phased approach to diversifying revenue across 23 streams, moving from transactional pricing toward a layered model of subscriptions, affiliate commissions, B2B licensing, and marketplace fees. The overarching principle: **never compromise the AI planning experience for monetization**. Every revenue stream must either improve or be invisible to the user's core journey.

**Target revenue mix at scale:**

- 35% Subscriptions (consumer + family/team)
- 25% Affiliate commissions (bookings, insurance, activities)
- 15% B2B / white-label licensing
- 10% Sponsored placements and destination partnerships
- 10% Marketplace and API fees
- 5% Premium add-ons (exports, concierge, priority AI)

---

## Phase 1: Foundation (0-1,000 users)

**Objective:** Validate willingness to pay, establish affiliate revenue baseline, keep engineering investment minimal. Focus on revenue streams that require little infrastructure beyond what already exists.

---

### 1. Trip Pro Per-Trip Unlock (CURRENT)

**What it is:** $12 CAD one-time unlock per trip. Grants unlimited messages, all 800+ expert personas, email forwarding, export, and best-fit recommendations.

| Metric            | Estimate                                                           |
| ----------------- | ------------------------------------------------------------------ |
| Revenue potential | $3,600-12,000/yr at 1K users (25-80% conversion, 1-1.3 trips/user) |
| Eng effort        | 0 days (already built)                                             |
| User impact       | Positive -- clear value exchange, no surprise charges              |
| Phase             | **1 (current)**                                                    |

**Action items:**

- A/B test $9.99 vs $12 vs $14.99 price points.
- Track conversion funnel: free trial start -> message limit hit -> upgrade prompt shown -> payment completed.
- Experiment with "first trip free" to reduce friction and build habit.

---

### 2. Affiliate Commissions on Bookings

**What it is:** Earn commissions when users book through recommendation links surfaced by the AI. Already partially built -- `RecommendationCard` renders links to Skyscanner, Booking.com, GetYourGuide, Viator, DiscoverCars, and SafetyWing.

| Metric            | Estimate                                                                                                   |
| ----------------- | ---------------------------------------------------------------------------------------------------------- |
| Revenue potential | $2-15 per booking click-through, 3-8% commission on hotels, 1-3% on flights. At 1K users: $5,000-20,000/yr |
| Eng effort        | 5-10 days (tracking pixels, attribution, partner dashboard integration, conversion analytics)              |
| User impact       | Positive if recommendations are genuinely best-fit; negative if they feel biased                           |
| Phase             | **1**                                                                                                      |

**Current partners (from RecommendationCard):**

- Skyscanner (flights)
- Booking.com (hotels)
- GetYourGuide (activities)
- Viator (activities)
- DiscoverCars (car rentals)
- SafetyWing (travel insurance)

**Action items:**

- Sign up for affiliate programs with each partner (most have self-serve signup).
- Replace placeholder URLs with proper affiliate-tagged deep links.
- Build conversion tracking: click -> partner site -> booking completed (postback URLs or API polling).
- Add a revenue dashboard in toqui-admin to track affiliate earnings by partner, trip, and user cohort.
- Ensure disclosure text on every recommendation card ("Toqui may earn a commission").
- Critical: AI must recommend based on fit first, affiliate second. The trust moat is everything.

---

### 3. Travel Insurance Partnerships

**What it is:** Contextual insurance upsell during trip planning. SafetyWing affiliate link already exists. Expand to include per-trip or annual travel insurance from partners like World Nomads, Allianz, or SafetyWing.

| Metric            | Estimate                                                                                                    |
| ----------------- | ----------------------------------------------------------------------------------------------------------- |
| Revenue potential | $5-25 per policy sold. Insurance has high affiliate commissions (15-30%). At 1K users: $2,000-8,000/yr      |
| Eng effort        | 3-5 days (contextual prompt in chat when user finalizes dates, deep link with pre-filled dates/destination) |
| User impact       | Positive -- genuinely useful reminder that users appreciate                                                 |
| Phase             | **1**                                                                                                       |

**Action items:**

- Trigger insurance recommendation when user sets international trip dates.
- Pre-fill partner links with destination, dates, and traveler count from the trip.
- Compare SafetyWing vs World Nomads vs Allianz affiliate terms.

---

### 4. Referral Program with Incentives

**What it is:** Already built (ReferralCard, `/api/referral`). Currently tracks referrals but doesn't offer monetary incentives.

| Metric            | Estimate                                                                                             |
| ----------------- | ---------------------------------------------------------------------------------------------------- |
| Revenue potential | Indirect -- reduces CAC. Each referred user worth $12-50 LTV. Target 20% of new users from referrals |
| Eng effort        | 2-3 days (add reward logic: e.g., free Trip Pro for referrer + referred)                             |
| User impact       | Positive -- users love rewarding their friends                                                       |
| Phase             | **1**                                                                                                |

**Action items:**

- Offer referrer: 1 free Trip Pro unlock per successful referral.
- Offer referred: first trip at $6 (50% off).
- Add referral leaderboard for power users.

---

### Phase 1 Revenue Summary

| Stream                  | Low Estimate/yr | High Estimate/yr |
| ----------------------- | --------------- | ---------------- |
| Trip Pro                | $3,600          | $12,000          |
| Affiliate bookings      | $5,000          | $20,000          |
| Travel insurance        | $2,000          | $8,000           |
| Referrals (CAC savings) | $1,000          | $5,000           |
| **Total**               | **$11,600**     | **$45,000**      |

**Total eng effort for Phase 1 additions: 10-18 days**

---

## Phase 2: Growth (1,000-10,000 users)

**Objective:** Introduce recurring revenue via subscriptions, expand affiliate coverage, and begin monetizing the social/shared trip features.

---

### 5. Subscription Tiers (Monthly/Annual)

**What it is:** Replace or complement per-trip pricing with subscription plans. Per-trip pricing has a ceiling -- subscribers generate predictable recurring revenue.

| Metric            | Estimate                                                                                                         |
| ----------------- | ---------------------------------------------------------------------------------------------------------------- |
| Revenue potential | At 10K users, 15% converting to $8/mo or $60/yr: $72,000-144,000/yr                                              |
| Eng effort        | 10-15 days (subscription management, Stripe/Helcim recurring billing, plan enforcement, upgrade/downgrade flows) |
| User impact       | Positive for frequent travelers (cheaper than per-trip); watch for "subscription fatigue"                        |
| Phase             | **2**                                                                                                            |

**Proposed tiers:**

| Tier     | Price                   | Includes                                                                              |
| -------- | ----------------------- | ------------------------------------------------------------------------------------- |
| Free     | $0                      | 1 active trip, 20 messages/day, basic personas, no export                             |
| Explorer | $7.99/mo or $59.99/yr   | 5 active trips, unlimited messages, all personas, PDF/calendar export                 |
| Voyager  | $14.99/mo or $119.99/yr | Unlimited trips, priority AI, premium personas, group trip hosting, concierge credits |

**Action items:**

- Keep per-trip unlock as an option for infrequent travelers (don't force subscriptions).
- Offer annual billing at 2 months free to boost retention.
- Grandfather early adopters into Explorer for life at $4.99/mo.
- Migrate from Helcim to Stripe for subscription management (Helcim lacks robust recurring billing APIs), or build recurring on top of Helcim if fees are materially lower.

---

### 6. Premium Export Formats

**What it is:** Upgrade the existing PDF/ICS export with polished, branded outputs. Free tier gets a basic text export; paid users get magazine-quality PDFs, printable mini-guidebooks, and shareable web pages.

| Metric            | Estimate                                                                                      |
| ----------------- | --------------------------------------------------------------------------------------------- |
| Revenue potential | $2-5 per premium export or included in subscription. At 10K users: $5,000-15,000/yr           |
| Eng effort        | 8-12 days (design templates, branded PDF generation with images/maps, print-optimized layout) |
| User impact       | Positive -- tangible deliverable users can share and print                                    |
| Phase             | **2**                                                                                         |

**Export tiers:**

- **Free:** Plain text itinerary, basic ICS calendar file.
- **Explorer:** Styled PDF with day-by-day layout, embedded maps, weather forecasts.
- **Voyager:** "Guidebook" format with cover page, restaurant/activity photos, packing checklist, offline-capable.

---

### 7. Family and Team Plans

**What it is:** Shared subscription for households or friend groups traveling together. Leverages existing group trip infrastructure.

| Metric            | Estimate                                                                                              |
| ----------------- | ----------------------------------------------------------------------------------------------------- |
| Revenue potential | $19.99/mo family (up to 5), $29.99/mo team (up to 10). At 10K users, 5% converting: $12,000-36,000/yr |
| Eng effort        | 8-10 days (seat management, shared billing, invitation flow, permissions)                             |
| User impact       | Positive -- solves the "my partner also needs Pro" pain point                                         |
| Phase             | **2**                                                                                                 |

---

### 8. Priority AI (Faster Responses, Better Models)

**What it is:** Free tier uses a cost-optimized model; paid tiers get faster inference, longer context windows, and access to the most capable models for complex multi-city planning.

| Metric            | Estimate                                                                    |
| ----------------- | --------------------------------------------------------------------------- |
| Revenue potential | Included in subscription tiers as a differentiator. Reduces churn by 10-15% |
| Eng effort        | 3-5 days (model routing based on plan tier, queue priority)                 |
| User impact       | Positive -- users notice speed and quality differences                      |
| Phase             | **2**                                                                       |

---

### 9. Gift Cards ("Give Someone a Trip Plan")

**What it is:** Purchasable gift codes that grant Trip Pro or subscription time. Perfect for holidays, weddings, honeymoons.

| Metric            | Estimate                                                                           |
| ----------------- | ---------------------------------------------------------------------------------- |
| Revenue potential | Seasonal spikes. At 10K users: $5,000-15,000/yr (heavily Q4-weighted)              |
| Eng effort        | 5-7 days (gift code generation, redemption flow, email delivery with branded card) |
| User impact       | Positive -- new acquisition channel with zero CAC                                  |
| Phase             | **2**                                                                              |

**Action items:**

- Sell on toqui.travel/gift with amounts: $12 (1 trip), $60 (1 year Explorer), $120 (1 year Voyager).
- Partner with travel bloggers for holiday gift guide placements.
- Physical gift cards via print-on-demand for retail partnerships (Phase 3).

---

### 10. Sponsored Recommendations (Pay-to-Appear)

**What it is:** Allow hotels, tour operators, and restaurants to pay for placement in AI-generated recommendations. Clearly labeled as "Sponsored" alongside organic results.

| Metric            | Estimate                                                                                                                                                       |
| ----------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Revenue potential | $0.50-5.00 per impression, $5-50 per click. At 10K users: $10,000-50,000/yr                                                                                    |
| Eng effort        | 12-15 days (sponsor management system, bid/targeting engine, disclosure compliance, reporting dashboard)                                                       |
| User impact       | **Risk of negative impact** if poorly executed. Must be clearly labeled, limited to 1 sponsored result per conversation, and genuinely relevant to user's trip |
| Phase             | **2 (late)**                                                                                                                                                   |

**Guardrails:**

- Maximum 1 sponsored recommendation per chat session.
- Must match user's destination, dates, budget, and preferences.
- Always labeled "Sponsored" with distinct visual treatment.
- Users can opt out of sponsored content (included in Voyager tier).
- AI never pretends sponsored content is organic.

---

### 11. In-Trip Companion Purchases

**What it is:** During an active trip, the companion mode can facilitate real-time bookings: restaurant reservations, last-minute tour tickets, museum passes, transportation. The AI becomes a concierge that can transact.

| Metric            | Estimate                                                                                                                     |
| ----------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| Revenue potential | $2-10 per transaction (booking fee or affiliate commission). At 10K users: $10,000-40,000/yr                                 |
| Eng effort        | 15-20 days (real-time availability APIs, booking confirmation flow, payment for third-party services, cancellation handling) |
| User impact       | Highly positive -- solves a real pain point during travel                                                                    |
| Phase             | **2**                                                                                                                        |

**Priority integrations:**

- OpenTable / Resy (restaurant reservations)
- GetYourGuide / Viator (same-day activities)
- Uber / local transit APIs (transportation)
- Google Places for real-time hours/availability

---

### Phase 2 Revenue Summary

| Stream                             | Low Estimate/yr | High Estimate/yr |
| ---------------------------------- | --------------- | ---------------- |
| Phase 1 streams (scaled 10x users) | $50,000         | $200,000         |
| Subscriptions                      | $72,000         | $144,000         |
| Premium exports                    | $5,000          | $15,000          |
| Family/team plans                  | $12,000         | $36,000          |
| Gift cards                         | $5,000          | $15,000          |
| Sponsored recommendations          | $10,000         | $50,000          |
| In-trip purchases                  | $10,000         | $40,000          |
| **Total**                          | **$164,000**    | **$500,000**     |

**Total eng effort for Phase 2 additions: 61-84 days**

---

## Phase 3: Scale (10,000-100,000 users)

**Objective:** Unlock B2B revenue, deepen partnerships, and introduce high-margin premium services.

---

### 12. White-Label / B2B Licensing

**What it is:** License the Toqui planning engine to travel agencies, corporate travel departments, airlines, and hotel chains. They embed Toqui's AI planner under their own brand.

| Metric            | Estimate                                                                                                      |
| ----------------- | ------------------------------------------------------------------------------------------------------------- |
| Revenue potential | $2,000-10,000/mo per client. 10 clients = $240,000-1,200,000/yr                                               |
| Eng effort        | 30-40 days (multi-tenant architecture, white-label theming, client onboarding, SLA monitoring, API isolation) |
| User impact       | Neutral to consumer users; expands total addressable market                                                   |
| Phase             | **3**                                                                                                         |

**Target clients:**

- Boutique travel agencies wanting AI differentiation.
- Corporate travel platforms (SAP Concur, TripActions) needing a planning layer.
- Airlines offering trip planning as a loyalty perk.
- Hotel chains wanting guests to plan around their properties.

**Pricing model:**

- Setup fee: $5,000-25,000.
- Monthly platform fee: $2,000-10,000 based on seat count.
- Per-query fee for usage-based clients: $0.05-0.15 per AI interaction.

---

### 13. Destination Marketing Partnerships (Tourism Boards)

**What it is:** Tourism boards and destination marketing organizations (DMOs) pay to promote their region within Toqui's recommendations. Unlike sponsored recommendations (which are individual businesses), these are regional campaigns.

| Metric            | Estimate                                                                                               |
| ----------------- | ------------------------------------------------------------------------------------------------------ |
| Revenue potential | $5,000-50,000 per campaign. 10 campaigns/yr = $50,000-500,000/yr                                       |
| Eng effort        | 8-10 days (campaign management dashboard, geo-targeted insertion, analytics reporting for DMO clients) |
| User impact       | Positive if done well -- users discover destinations they wouldn't have considered                     |
| Phase             | **3**                                                                                                  |

**How it works:**

- DMO pays for a campaign (e.g., "Visit Nova Scotia" for 3 months).
- When users are planning trips to eastern Canada or haven't decided on a destination, the AI naturally suggests Nova Scotia with authentic local knowledge.
- DMO gets analytics: impressions, trip plans created for their region, estimated visitor intent.
- Content is always authentic and useful, never feel like an ad.

---

### 14. Premium Personas / Expert Packs

**What it is:** Exclusive expert personas developed in partnership with real travel personalities, guidebook authors, or local experts. Think "Anthony Bourdain's food guide to Tokyo" level of specificity.

| Metric            | Estimate                                                                                       |
| ----------------- | ---------------------------------------------------------------------------------------------- |
| Revenue potential | $2-5 per persona pack or included in Voyager. At 100K users: $20,000-80,000/yr                 |
| Eng effort        | 5-8 days (persona marketplace UI, purchase/unlock flow, content pipeline for persona creation) |
| User impact       | Positive for enthusiasts; basic personas remain free                                           |
| Phase             | **3**                                                                                          |

**Ideas:**

- "Local Insider" packs for specific cities (curated by actual locals).
- "Luxury Travel" pack with high-end hotel/restaurant expertise.
- "Budget Backpacker" pack optimized for hostel/street food recommendations.
- "Family Travel" pack with kid-friendly activity specialization.
- Co-branded packs with travel influencers (revenue share).

---

### 15. API Access (Planning Engine as a Service)

**What it is:** Expose Toqui's trip planning, itinerary generation, and destination intelligence as a developer API. Third-party apps can integrate AI trip planning without building it themselves.

| Metric            | Estimate                                                                                         |
| ----------------- | ------------------------------------------------------------------------------------------------ |
| Revenue potential | $0.05-0.50 per API call. At scale: $100,000-500,000/yr                                           |
| Eng effort        | 20-25 days (public API gateway, rate limiting, API key management, documentation, usage billing) |
| User impact       | Neutral to consumer users; creates ecosystem lock-in                                             |
| Phase             | **3**                                                                                            |

**API products:**

- `/plan` -- Generate a full itinerary from destination + dates + preferences.
- `/recommend` -- Get contextual activity/hotel/restaurant recommendations.
- `/weather` -- Destination weather intelligence for travel dates.
- `/budget` -- Cost estimation for a trip plan.

**Pricing:**

- Free tier: 100 calls/month.
- Starter: $49/mo for 5,000 calls.
- Pro: $199/mo for 25,000 calls.
- Enterprise: custom pricing.

---

### 16. Concierge Service (Human-Assisted Planning)

**What it is:** For complex trips (multi-country, luxury, group incentive travel), offer a human travel expert who works alongside the AI. The AI handles 80% of the work; the human adds the final 20% of nuance and handles bookings that require phone calls.

| Metric            | Estimate                                                                                                  |
| ----------------- | --------------------------------------------------------------------------------------------------------- |
| Revenue potential | $50-200 per concierge session. At 100K users, 1% using: $50,000-200,000/yr                                |
| Eng effort        | 10-12 days (concierge queue, agent dashboard in toqui-admin, handoff from AI to human, session recording) |
| User impact       | Highly positive for the users who need it; premium feel                                                   |
| Phase             | **3**                                                                                                     |

**Model:**

- Start with contract travel advisors (not full-time hires).
- AI pre-fills the trip plan; human reviews and customizes.
- Charge per session or offer as Voyager perk (N concierge hours/month).

---

### 17. Data and Insights (Anonymized Travel Trends)

**What it is:** Aggregate anonymized travel planning data into trend reports sold to tourism boards, airlines, hotel chains, and travel industry analysts.

| Metric            | Estimate                                                                                |
| ----------------- | --------------------------------------------------------------------------------------- |
| Revenue potential | $10,000-50,000 per report. Quarterly reports to 10 clients: $40,000-200,000/yr          |
| Eng effort        | 12-15 days (data pipeline, anonymization/aggregation, report generation, client portal) |
| User impact       | Neutral if properly anonymized; requires clear privacy policy and opt-out               |
| Phase             | **3**                                                                                   |

**Report types:**

- "Emerging Destinations" -- where are users planning trips that they weren't 6 months ago?
- "Budget Trends" -- how are travel budgets changing by demographic/region?
- "Seasonal Demand Forecasting" -- when are users planning trips to specific destinations?
- "Activity Preferences" -- what experiences are trending (food tours, hiking, cultural)?

**Privacy guardrails:**

- All data aggregated and anonymized (minimum cohort size of 100).
- Users must opt in (default opt-out).
- No individual trip data ever shared.
- Comply with PIPEDA, GDPR, CCPA.

---

### 18. Travel Rewards / Loyalty Program

**What it is:** Users earn "Toqui Miles" for planning trips, booking through affiliate links, referring friends, and writing reviews. Miles redeemable for Trip Pro credits, subscription discounts, or partner perks.

| Metric            | Estimate                                                                            |
| ----------------- | ----------------------------------------------------------------------------------- |
| Revenue potential | Indirect -- increases retention by 20-30%, boosts affiliate click-through by 15-25% |
| Eng effort        | 12-15 days (points ledger, earning rules, redemption catalog, tier status)          |
| User impact       | Positive -- gamification increases engagement                                       |
| Phase             | **3**                                                                               |

**Earning:**

- 100 miles per trip planned.
- 500 miles per booking completed through Toqui.
- 250 miles per friend referred.
- 50 miles per trip review/rating.

**Redemption:**

- 1,000 miles = $5 credit toward Trip Pro or subscription.
- Partner perks: airport lounge passes, hotel upgrades (negotiated with partners).

---

### 19. Content Licensing (AI-Generated Destination Guides)

**What it is:** Package Toqui's AI-generated destination knowledge into structured content sold to publishers, travel magazines, and media companies.

| Metric            | Estimate                                                                             |
| ----------------- | ------------------------------------------------------------------------------------ |
| Revenue potential | $1,000-10,000 per content package. At scale: $20,000-100,000/yr                      |
| Eng effort        | 8-10 days (content generation pipeline, editorial review workflow, licensing portal) |
| User impact       | Neutral -- content is derivative of what users already get                           |
| Phase             | **3**                                                                                |

---

### Phase 3 Revenue Summary

| Stream                         | Low Estimate/yr | High Estimate/yr |
| ------------------------------ | --------------- | ---------------- |
| Phase 1+2 streams (scaled 10x) | $500,000        | $1,500,000       |
| White-label B2B                | $240,000        | $1,200,000       |
| Destination marketing          | $50,000         | $500,000         |
| Premium personas               | $20,000         | $80,000          |
| API access                     | $100,000        | $500,000         |
| Concierge                      | $50,000         | $200,000         |
| Data/insights                  | $40,000         | $200,000         |
| Loyalty program (indirect)     | $50,000         | $150,000         |
| Content licensing              | $20,000         | $100,000         |
| **Total**                      | **$1,070,000**  | **$4,430,000**   |

**Total eng effort for Phase 3 additions: 105-135 days**

---

## Phase 4: Platform (100,000+ users)

**Objective:** Transform Toqui from a product into a platform. Create network effects where more users and partners make the platform more valuable for everyone.

---

### 20. Local Experience Marketplace

**What it is:** Allow local guides, tour operators, chefs, and experience providers to list their offerings directly on Toqui. The AI recommends them contextually during trip planning. Toqui takes a 15-20% platform fee.

| Metric            | Estimate                                                                                                      |
| ----------------- | ------------------------------------------------------------------------------------------------------------- |
| Revenue potential | 15-20% take rate on GMV. $50 avg booking, 100K+ users: $500,000-2,000,000/yr                                  |
| Eng effort        | 40-50 days (provider onboarding, listing management, booking/payment processing, reviews, dispute resolution) |
| User impact       | Highly positive -- unique experiences not available on major platforms                                        |
| Phase             | **4**                                                                                                         |

**Differentiation from GetYourGuide/Viator:**

- AI-curated matching (not search-based browsing).
- Hyper-local providers too small for major platforms.
- Integrated into the trip timeline seamlessly.
- Real-time availability during the trip via companion mode.

---

### 21. Advertising (Display Ads on Free Tier)

**What it is:** Tasteful, travel-relevant display ads shown to free-tier users. Removed for all paid tiers.

| Metric            | Estimate                                                                             |
| ----------------- | ------------------------------------------------------------------------------------ |
| Revenue potential | $2-8 RPM (revenue per thousand impressions). At 100K+ free users: $50,000-200,000/yr |
| Eng effort        | 5-8 days (ad integration SDK, placement logic, frequency capping)                    |
| User impact       | **Negative** -- ads degrade the premium feel. Must be used carefully                 |
| Phase             | **4 (only if needed)**                                                               |

**Guardrails:**

- Never in the chat interface. Only on trip list, settings, and shared trip pages.
- Only travel-relevant ads (no random display ads).
- Maximum 1 ad per screen.
- Immediate removal path: upgrade to any paid tier.
- Consider this a "last resort" revenue stream. The subscription conversion pressure from ads may be more valuable than the ad revenue itself.

---

### 22. Corporate Travel Program

**What it is:** Enterprise version of Toqui for corporate travel management. Employees use Toqui to plan business trips within company policy guardrails. Integrates with expense management.

| Metric            | Estimate                                                                                       |
| ----------------- | ---------------------------------------------------------------------------------------------- |
| Revenue potential | $5-15 per employee/month. 50 companies, 100 employees avg: $300,000-900,000/yr                 |
| Eng effort        | 35-45 days (SSO/SAML, policy engine, expense integration, admin dashboard, approval workflows) |
| User impact       | Opens entirely new market segment                                                              |
| Phase             | **4**                                                                                          |

**Features:**

- Policy-aware AI (knows company travel policies, preferred vendors, budget limits).
- Automatic expense report generation.
- Integration with SAP Concur, Expensify, Brex.
- Admin dashboard for travel managers.
- Consolidated booking and invoicing.

---

### 23. Travel Creator / Influencer Platform

**What it is:** Travel influencers and creators publish their curated trip plans on Toqui. Users can "clone" a creator's trip and customize it. Creators earn a share of revenue generated.

| Metric            | Estimate                                                                                                         |
| ----------------- | ---------------------------------------------------------------------------------------------------------------- |
| Revenue potential | $3-10 per cloned trip plan. Revenue share: 70% Toqui, 30% creator. At scale: $100,000-500,000/yr                 |
| Eng effort        | 20-25 days (creator profiles, trip publishing flow, clone/fork functionality, revenue tracking, creator payouts) |
| User impact       | Positive -- social proof and inspiration drive engagement                                                        |
| Phase             | **4**                                                                                                            |

---

### 24. Embedded Travel Planning Widget

**What it is:** A lightweight embeddable widget that travel blogs, destination websites, and hotel sites can add. Visitors enter their dates and get an instant AI trip plan. Leads funnel into the full Toqui app.

| Metric            | Estimate                                                                                           |
| ----------------- | -------------------------------------------------------------------------------------------------- |
| Revenue potential | $500-2,000/mo per enterprise embed. Affiliate commissions from widget bookings. $50,000-200,000/yr |
| Eng effort        | 15-20 days (embeddable iframe/web component, widget configuration API, lead attribution)           |
| User impact       | Neutral to existing users; strong acquisition channel                                              |
| Phase             | **4**                                                                                              |

---

### 25. Dynamic Pricing Intelligence

**What it is:** Use aggregated booking data to offer users "best time to book" alerts and price predictions. Partner with airlines and hotels to offer exclusive Toqui-user pricing.

| Metric            | Estimate                                                                                                             |
| ----------------- | -------------------------------------------------------------------------------------------------------------------- |
| Revenue potential | Premium feature included in Voyager. Increases booking affiliate conversion by 25-40%. Indirect: $100,000-300,000/yr |
| Eng effort        | 20-25 days (price tracking infrastructure, prediction model, alert system, partner pricing agreements)               |
| User impact       | Highly positive -- users save real money                                                                             |
| Phase             | **4**                                                                                                                |

---

### Phase 4 Revenue Summary

| Stream                         | Low Estimate/yr | High Estimate/yr |
| ------------------------------ | --------------- | ---------------- |
| Phase 1-3 streams (scaled 10x) | $3,000,000      | $10,000,000      |
| Local experience marketplace   | $500,000        | $2,000,000       |
| Display advertising            | $50,000         | $200,000         |
| Corporate travel               | $300,000        | $900,000         |
| Creator platform               | $100,000        | $500,000         |
| Embedded widget                | $50,000         | $200,000         |
| Dynamic pricing                | $100,000        | $300,000         |
| **Total**                      | **$4,100,000**  | **$14,100,000**  |

---

## Revenue Stream Master Matrix

| #   | Stream                 | Phase | Eng Days | Annual Rev (Low) | Annual Rev (High) | User Impact | Priority |
| --- | ---------------------- | ----- | -------- | ---------------- | ----------------- | ----------- | -------- |
| 1   | Trip Pro per-trip      | 1     | 0        | $3,600           | $12,000           | Positive    | LIVE     |
| 2   | Affiliate bookings     | 1     | 5-10     | $5,000           | $20,000           | Positive    | P0       |
| 3   | Travel insurance       | 1     | 3-5      | $2,000           | $8,000            | Positive    | P0       |
| 4   | Referral incentives    | 1     | 2-3      | $1,000           | $5,000            | Positive    | P1       |
| 5   | Subscription tiers     | 2     | 10-15    | $72,000          | $144,000          | Positive    | P0       |
| 6   | Premium exports        | 2     | 8-12     | $5,000           | $15,000           | Positive    | P2       |
| 7   | Family/team plans      | 2     | 8-10     | $12,000          | $36,000           | Positive    | P1       |
| 8   | Priority AI            | 2     | 3-5      | --               | --                | Positive    | P1       |
| 9   | Gift cards             | 2     | 5-7      | $5,000           | $15,000           | Positive    | P2       |
| 10  | Sponsored recs         | 2     | 12-15    | $10,000          | $50,000           | Mixed       | P2       |
| 11  | In-trip purchases      | 2     | 15-20    | $10,000          | $40,000           | Positive    | P1       |
| 12  | White-label B2B        | 3     | 30-40    | $240,000         | $1,200,000        | Neutral     | P0       |
| 13  | Destination marketing  | 3     | 8-10     | $50,000          | $500,000          | Positive    | P1       |
| 14  | Premium personas       | 3     | 5-8      | $20,000          | $80,000           | Positive    | P2       |
| 15  | API access             | 3     | 20-25    | $100,000         | $500,000          | Neutral     | P1       |
| 16  | Concierge              | 3     | 10-12    | $50,000          | $200,000          | Positive    | P1       |
| 17  | Data/insights          | 3     | 12-15    | $40,000          | $200,000          | Neutral     | P2       |
| 18  | Loyalty program        | 3     | 12-15    | $50,000          | $150,000          | Positive    | P2       |
| 19  | Content licensing      | 3     | 8-10     | $20,000          | $100,000          | Neutral     | P3       |
| 20  | Experience marketplace | 4     | 40-50    | $500,000         | $2,000,000        | Positive    | P0       |
| 21  | Display advertising    | 4     | 5-8      | $50,000          | $200,000          | Negative    | P3       |
| 22  | Corporate travel       | 4     | 35-45    | $300,000         | $900,000          | Neutral     | P1       |
| 23  | Creator platform       | 4     | 20-25    | $100,000         | $500,000          | Positive    | P1       |
| 24  | Embedded widget        | 4     | 15-20    | $50,000          | $200,000          | Neutral     | P2       |
| 25  | Dynamic pricing        | 4     | 20-25    | $100,000         | $300,000          | Positive    | P2       |

---

## Key Strategic Principles

### 1. Trust is the moat

The AI's recommendations must always serve the user first. The moment users suspect recommendations are biased toward revenue, the product is dead. Every monetization stream must pass the test: "Would I recommend this to a user even if we earned nothing from it?"

### 2. Free tier must be genuinely useful

The free tier should be good enough that users tell their friends about Toqui. Paid tiers should feel like "more of something great," not "finally something usable." Conversion comes from delight, not frustration.

### 3. Per-trip pricing is a bridge, not the destination

$12/trip is a great early signal of willingness to pay, but it caps revenue per user. The transition to subscriptions should happen in Phase 2 before per-trip pricing becomes an expectation that's hard to unwind.

### 4. B2B is the margin play

Consumer subscriptions build the base; B2B licensing and partnerships deliver outsized margins. A single white-label deal can equal thousands of consumer subscriptions. Start B2B conversations in Phase 2, even if deals close in Phase 3.

### 5. Affiliate revenue should be invisible

Users should never feel like they're being "sold to." Booking recommendations should feel like helpful suggestions from a knowledgeable friend. The affiliate link is just how the friend gets thanked.

### 6. Data is valuable but dangerous

Anonymized travel trend data is genuinely valuable to the industry. But one privacy incident could destroy the brand. Over-invest in anonymization, be conservative about what data leaves the system, and make data sharing opt-in.

### 7. Resist advertising as long as possible

Display ads are the easiest revenue to implement and the fastest way to cheapen the product. Only introduce them if other streams prove insufficient, and only on the free tier, and never in the chat interface.

---

## Financial Projections Summary

| Phase | Users    | Timeline     | Annual Revenue (Low) | Annual Revenue (High) |
| ----- | -------- | ------------ | -------------------- | --------------------- |
| 1     | 0-1K     | Months 0-12  | $11,600              | $45,000               |
| 2     | 1K-10K   | Months 6-18  | $164,000             | $500,000              |
| 3     | 10K-100K | Months 12-30 | $1,070,000           | $4,430,000            |
| 4     | 100K+    | Months 24-48 | $4,100,000           | $14,100,000           |

**Note:** Phases overlap. Phase 2 engineering begins during Phase 1 user growth. Revenue estimates are conservative on the low end and assume strong product-market fit on the high end.

---

## Immediate Next Steps (Next 30 Days)

1. **Sign affiliate agreements** with Skyscanner, Booking.com, GetYourGuide, Viator, DiscoverCars, and SafetyWing. Replace placeholder URLs with tagged affiliate links. (Eng: 5 days)

2. **Build conversion tracking** for affiliate clicks and bookings. Add revenue reporting to toqui-admin. (Eng: 5 days)

3. **A/B test Trip Pro pricing** at $9.99, $12, and $14.99. Run for 4 weeks with statistical significance. (Eng: 2 days)

4. **Activate referral rewards** -- give referrer a free Trip Pro and referred user 50% off first trip. (Eng: 2 days)

5. **Begin subscription tier design** -- wireframe upgrade flows, draft pricing page, research Stripe vs Helcim for recurring billing. (Eng: 0 days, product/design work)

6. **Start B2B outreach** -- identify 10 travel agencies and 5 corporate travel managers for early conversations about white-label interest. (Eng: 0 days, business development)

---

_This document should be revisited quarterly and updated as actual revenue data replaces estimates._
