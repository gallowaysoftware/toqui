# Toqui Growth Strategy

Last updated: 2026-04-02

---

## 1. Launch Checklist (Go-to-Market)

### Product Hunt Launch

**Target: Mid-May 2026** (gives ~6 weeks of prep)

- [ ] Create a Product Hunt "Coming Soon" page now and start collecting followers
- [ ] Build a 30-second demo video showing: paste a trip idea into chat -> get a full itinerary with map + weather + bookings in under 60 seconds
- [ ] Collect 3-5 testimonials from current users (real trips planned, real value received)
- [ ] Prepare a launch-day offer: first trip free (no Pro unlock needed) for PH users
- [ ] Launch on a **Thursday** -- competitive enough to get visibility, less brutal than Tuesday/Wednesday
- [ ] Line up 15-20 people to leave genuine first-hour comments (friends, indie hacker community, travel bloggers who tested it)
- [ ] Post a "maker comment" telling the story: solo founder, built because existing tools are subscription traps, per-trip pricing is fairer

**Tagline options for PH:**
- "Your AI travel companion that charges per trip, not per month"
- "Plan, book, and navigate trips with 800+ expert AI personas -- pay $12 only when you actually travel"

### Social Media Presence

**Priority platforms (in order):**

1. **TikTok** -- Travel content performs extremely well here. Short demos of "I planned a 7-day Japan trip in 90 seconds" format. Travel hacks are a massive trend.
2. **Instagram** -- Reels repurposed from TikTok. Carousel posts showing itinerary comparisons (Toqui vs. manual planning). Partner with micro-influencers.
3. **Twitter/X** -- Indie hacker community. Build-in-public updates. Engage with travel tech conversations.
4. **Reddit** -- Not for posting about Toqui directly. Be genuinely helpful in r/travel, r/solotravel, r/shoestring, r/digitalnomad. When someone asks "how do I plan X trip," answer thoroughly and mention Toqui naturally if relevant.

**Content cadence:**
- TikTok/Reels: 3x/week (batch-produce, can reuse across platforms)
- Twitter/X: Daily (build-in-public, engage with travel/AI community)
- Reddit: 2-3 genuinely helpful comments per week in travel subreddits

### SEO (toqui.travel)

- [ ] Add destination guide pages at `toqui.travel/guides/{destination}` (see Section 4)
- [ ] Target long-tail keywords: "3 day Tokyo itinerary," "Portugal road trip planner," "budget Bali trip plan"
- [ ] Each guide page should have a CTA: "Want a personalized version? Plan this trip with Toqui"
- [ ] Add structured data (FAQ schema, HowTo schema) to guide pages
- [ ] Set up Google Search Console and monitor impressions from day 1
- [ ] Blog at `toqui.travel/blog` with 2 posts/month: trip planning tips, AI travel planning comparisons, destination deep-dives

### App Store Submission

**Timeline:**
- [ ] **Now - April 2026**: iOS and Android builds working locally via Expo/EAS
- [ ] **May 2026**: Submit to Apple App Store (allow 2 weeks for review, rejection cycle)
- [ ] **May 2026**: Submit to Google Play (typically faster, 3-7 days)
- [ ] **Target**: Apps live before or concurrent with Product Hunt launch
- [ ] Prepare App Store Optimization (ASO): screenshots showing real itineraries, keyword-rich description, category = Travel

### Press & Influencer Outreach

**Angle: "The anti-subscription travel AI"**

The pitch: Every other AI travel tool either locks you into a monthly subscription you forget about, or is "free" because they take affiliate cuts on bad hotel recommendations. Toqui charges $12 per trip -- only when you actually have a real trip to plan. No recurring charges, no affiliate-driven recommendations.

**Who to contact:**

*Travel tech press:*
- PhocusWire (they cover AI travel startups actively)
- Skift (travel industry publication)
- The Points Guy, NomadList blog

*Tech/startup press:*
- TechCrunch (longshot, but the pricing angle is novel)
- Product Hunt newsletter editors
- Indie Hackers podcast / community spotlight

*Micro-influencers (1K-100K followers):*
- Solo travel TikTokers who post "how I planned my trip" content
- Budget travel YouTubers (the $12/trip angle resonates with budget travelers)
- Digital nomad content creators on Instagram
- Travel planning niche accounts on Twitter/X

**Outreach approach:**
- Give them a free Pro trip unlock code
- Ask them to plan a real upcoming trip with Toqui and share the experience
- No script -- authentic reactions perform better
- Budget: $0 for product gifting (free Pro codes cost nothing), $500-1000 for 5-10 paid micro-influencer posts if organic outreach is slow

---

## 2. Key Metrics (Day 1)

### Metrics That Matter at This Stage

Forget MAU, DAU, and download counts. At 3 users growing to 100, the only metrics that matter are:

| Metric | Why It Matters | Target |
|--------|---------------|--------|
| **Activation rate** | % of signups who send their first chat message within 24 hours | > 60% |
| **Trip completion rate** | % of users who generate a full itinerary (not just chat) | > 40% |
| **Pro conversion rate** | % of users who pay $12 for Pro on at least one trip | > 5% at launch, > 10% at scale |
| **Referral rate** | % of users who share their referral code | > 15% |
| **Referral conversion** | % of referred users who actually sign up | > 25% |
| **Retention (trip-based)** | % of users who create a second trip within 90 days | > 30% |
| **Time to first itinerary** | How long from signup to having a real itinerary | < 5 minutes |
| **NPS / qualitative feedback** | Would you recommend Toqui to a friend planning a trip? | > 50 NPS |

### Instrumentation

**Recommended tool: PostHog** (free tier covers 1M events/month, generous for early stage)

Why PostHog over Mixpanel: open-source, self-hostable if needed, includes session replay (critical for seeing where users get confused), feature flags for A/B testing the paywall, and funnels -- all in one tool. No credit card required for free tier.

**Events to track from day 1:**

```
// Authentication
user_signed_up          { method: "google", referral_code?: string }
user_logged_in          { method: "google" }

// Core loop
trip_created            { has_dates: bool, destination?: string }
chat_message_sent       { trip_id, message_length, is_first_message: bool }
itinerary_generated     { trip_id, num_days, num_items }
persona_switched        { trip_id, persona_name }
booking_added           { trip_id, booking_type }

// Monetization
pro_upgrade_viewed      { trip_id }
pro_upgrade_started     { trip_id }
pro_upgrade_completed   { trip_id, amount: 12 }

// Sharing & referral
trip_shared             { trip_id, method: "link" | "referral" }
referral_code_copied    {}
referral_redeemed       { referrer_id }

// Engagement
itinerary_exported      { format: "pdf" | "calendar" }
map_viewed              { trip_id }
```

**Session replay**: Enable PostHog session replay on web. Watch the first 50 users go through the app. This is the single most valuable thing you can do -- you will see exactly where people get confused, rage-click, or drop off.

### Benchmarks

For a travel app at launch (first 90 days):

- **Signup-to-activation**: 60%+ is good (travel apps with chat interfaces tend to have high activation because the first action is obvious: describe your trip)
- **Pro conversion at 5%**: This is realistic for a $12 one-time purchase. Compare: most SaaS free-to-paid is 2-5%. You have an advantage because the ask is small and non-recurring.
- **Week 1 retention**: Expect 20-30% (travel is inherently episodic -- people plan trips, then come back months later). This is fine. Do not optimize for daily engagement.
- **Referral rate at 15%**: Dropbox achieved ~35% with a strong incentive. Without a strong incentive, 15% is ambitious but achievable if the product genuinely impresses.

---

## 3. First 100 Users Playbook

### Where They Are

**Tier 1 (highest intent, go here first):**
- **r/travel** (12M+ members) -- People actively asking "help me plan my X trip"
- **r/solotravel** -- Solo travelers do more planning, more likely to try tools
- **r/shoestring** -- Budget travelers love the $12 per-trip model (vs. $10/month subscriptions they might not use)
- **r/digitalnomad** -- Frequent travelers who plan multiple trips per year
- **Facebook groups**: "Travel Planning," "Budget Travel," "Solo Female Travelers" (massive groups, 100K+ members each)

**Tier 2 (build presence over time):**
- Travel Twitter/X (follow and engage with @nomadicmatt, @expertvagabond, travel tech accounts)
- Indie Hackers community (founder story angle)
- Hacker News "Show HN" post
- Digital nomad forums: NomadList, remote work Slack communities

**Tier 3 (longer-term):**
- Travel blog comment sections (genuinely helpful, not spammy)
- Quora travel planning questions
- YouTube comments on trip planning videos

### The Hook

**Do NOT say:** "AI travel planner" -- every competitor says this and it means nothing.

**Instead, lead with the outcome and the pricing:**

> "I planned a 10-day Italy trip in 3 minutes -- with restaurant recs from a local food critic AI, weather forecasts for each day, and a map I could actually follow. It cost $12. Not per month. Just $12 for the whole trip."

**Hooks that work for different audiences:**

| Audience | Hook |
|----------|------|
| Budget travelers | "Every AI travel planner charges $10/month. Toqui charges $12 per trip. If you take 2 trips a year, you save $96." |
| Solo travelers | "Planning solo trips is exhausting. I asked Toqui's nightlife expert persona where to go in Bangkok on a Tuesday, and it knew which spots are dead mid-week." |
| Group trip planners | "We shared one Toqui trip with 4 friends. Everyone added their must-dos. The AI built an itinerary that actually worked for all of us." |
| Reddit/forum users | Don't pitch. Answer someone's "help me plan" post thoroughly with a real itinerary. Add "I used Toqui to put this together" at the bottom. |

### What Makes Someone Share Toqui

People share travel tools when:
1. **The output is visually impressive** -- A beautiful itinerary with a map that they can show friends ("look what I planned")
2. **It saved them real time** -- "I spent 3 minutes instead of 3 hours"
3. **Group trips** -- The shared trip feature is inherently viral: one person signs up, invites 3-4 friends
4. **The pricing surprises them** -- "$12 for all of this? Not per month?" is a genuinely shareable reaction

**Maximize sharing by:**
- Making the shared trip view (the `/shared/[token]` page) look incredible even for non-users. This is your best viral loop. Every shared trip is a landing page.
- Adding a "Plan a trip like this" CTA on every shared trip page
- Making PDF exports include a subtle "Made with Toqui" watermark + link

### Referral System Optimization

Current state: referral system exists (`POST /api/referral/redeem`, share link `toqui.travel?ref=CODE`).

**Improvements to consider:**

1. **Add a real incentive**: Give both referrer AND referred user a free Pro unlock on their next trip. Cost to you: $0 (it's digital). Value to user: $12. This is the single highest-leverage change.
2. **Prompt at the right moment**: Ask users to share their referral code immediately after they finish an itinerary (moment of maximum delight), not on a settings page.
3. **Make the referral link preview rich**: When someone shares a Toqui referral link on iMessage/WhatsApp/Slack, the Open Graph preview should show something compelling -- not just the Toqui logo, but "X invited you to plan your next trip with AI. Your first trip is on them."
4. **Track referral attribution end-to-end**: Know exactly which users came from referrals and whether they convert to Pro at a higher rate than organic signups.

---

## 4. Content Strategy

### Destination Guides (SEO Engine)

`toqui.travel/guides` can become the primary organic traffic driver. The play: rank for long-tail travel planning queries, then convert visitors into app users.

**Page structure for each guide:**

```
toqui.travel/guides/tokyo-3-day-itinerary
toqui.travel/guides/portugal-road-trip-7-days
toqui.travel/guides/bali-budget-guide
toqui.travel/guides/iceland-ring-road-10-days
```

Each guide should include:
- Day-by-day sample itinerary (shows the product's output quality)
- Estimated budget breakdown
- Best time to visit + weather summary
- Local tips (this is where the persona angle shines -- "our local food expert recommends...")
- CTA: "Want this itinerary customized for your dates and budget? Try Toqui"

### Topics to Prioritize

**Start with 20 guides targeting the highest-volume travel planning queries:**

1. Top destinations by search volume: Tokyo, Paris, Rome, Barcelona, Bali, Thailand, Iceland, Portugal, New Zealand, Peru
2. For each destination, create 2 variants:
   - "[Destination] [N]-day itinerary" (e.g., "Tokyo 5-day itinerary")
   - "Budget [destination] trip plan" (e.g., "Budget Bali trip plan")

**Blog content (2x/month):**
- "I planned 5 trips with AI -- here's what actually worked" (honest comparison, mention competitors fairly, show where Toqui wins)
- "Why we charge per trip instead of monthly" (pricing philosophy, builds trust)
- Seasonal content: "Best destinations for [season] 2026" with Toqui itinerary examples
- "How 800 expert personas make better travel recommendations than one generic AI"

### Using AI to Generate Guides at Scale (Responsibly)

**The approach:**
1. Use Toqui's own AI (the same system users interact with) to generate the base itinerary content for each guide
2. Have a human editor review every guide before publishing -- check for hallucinated restaurants/hotels, outdated info, and generic filler
3. Add real photos (Unsplash/Pexels for now, eventually user-submitted)
4. Update guides quarterly with fresh info (prices change, places close)

**What NOT to do:**
- Do not publish 500 AI-generated pages at once. Google penalizes thin, mass-produced content. Start with 20 high-quality guides.
- Do not fabricate reviews or recommendations. Every restaurant/hotel mentioned should be verifiable.
- Do not copy from other travel sites. The AI should generate original itineraries based on Toqui's planning logic.

**Scale plan:**
- Month 1: 20 guides (top destinations x 2 variants)
- Month 2-3: 20 more guides (expand to secondary destinations)
- Month 4+: Add user-generated guides -- let users publish their Toqui itineraries as public guides (with their permission). This is the long-term content flywheel.

---

## 5. Competitive Positioning

### Market Landscape (April 2026)

| Competitor | Pricing | Key Weakness |
|-----------|---------|-------------|
| **Layla** | $9.99/month or $49/year | Subscription model -- you pay even when not traveling. Struggles with complex multi-destination routes. |
| **Mindtrip** | Free (affiliate revenue) | Beautiful but recommendations are influenced by affiliate partnerships (Priceline, Viator). Not truly best-fit. |
| **Wonderplan** | Free / $49.99/year premium | Generic itineraries. No chat interface -- form-based input only. |
| **TripPlanner.ai** | Free / paid tiers | Basic output quality. No expert personas or during-trip features. |
| **ChatGPT/Gemini** | $20/month (or free tier) | General-purpose AI -- no travel-specific features, no itinerary maps, no booking integration, no group trips. Hallucination risk with restaurants/hotels. |

### Toqui's Positioning

**Primary message: "Pay per trip, not per month."**

This is the single clearest differentiator. Frame it as:
- $12 per trip vs. $120/year for Layla (if you take 2-3 trips/year, Toqui is 70% cheaper)
- $12 per trip vs. "free" tools that push affiliate hotel bookings at you
- $12 per trip vs. $240/year for ChatGPT Plus (which doesn't even have travel features)

**Secondary differentiators:**

1. **800+ expert personas** -- Not just "an AI." A local food critic for Tokyo, a budget backpacking expert for Southeast Asia, a luxury resort specialist for the Maldives. No competitor has this. This is the "wow" factor in demos.

2. **Works DURING the trip, not just before** -- Most AI travel tools help you plan, then disappear. Toqui is your companion throughout: weather changes? Re-plan. Found a closed restaurant? Get an alternative in 10 seconds. Flight delayed? Adjust the itinerary. This is underexplored messaging.

3. **Group trips that actually work** -- Share a trip, everyone contributes, the AI synthesizes everyone's preferences. This is inherently viral and no competitor does it well.

4. **No affiliate bias** -- Mindtrip is free because Priceline and Viator pay them. Their recommendations are influenced by who pays the highest commission. Toqui's AI always recommends what best fits the traveler, regardless of tier. Affiliate links support the platform but never influence what's recommended.

### Positioning by Channel

| Channel | Lead With |
|---------|----------|
| Product Hunt | "800+ expert AI travel personas. $12 per trip. No subscription." |
| Reddit/forums | Helpfulness first. Pricing second. "I used this tool that costs $12 per trip (not monthly) and it planned my whole Japan trip." |
| TikTok/Instagram | Visual output. "Watch me plan a 7-day Italy trip in 90 seconds." The persona switching is visually interesting content. |
| Press/blogs | The anti-subscription angle. "While Layla charges $10/month and Mindtrip sells your attention to hotels, Toqui built a different model." |
| App Store | "AI Travel Planner -- Expert Personas, Maps, Weather, Bookings. Pay per trip." |

---

## Appendix: 90-Day Execution Timeline

| Week | Action |
|------|--------|
| 1-2 | Set up PostHog analytics. Instrument all core events. Enable session replay. |
| 1-2 | Create Product Hunt "Coming Soon" page. Start building followers. |
| 1-2 | Set up Twitter/X account. Begin build-in-public posting. |
| 3-4 | Write and publish first 10 destination guides on toqui.travel. |
| 3-4 | Start engaging in r/travel, r/solotravel (helpful answers, not promotion). |
| 3-4 | Submit iOS app to Apple for review. Submit Android to Google Play. |
| 5-6 | Create 3-5 TikTok/Reels showing trip planning demos. |
| 5-6 | Reach out to 10 micro-influencers with free Pro codes. |
| 5-6 | Publish remaining 10 destination guides. |
| 7-8 | Product Hunt launch day (target Thursday). |
| 7-8 | Show HN post on Hacker News (same week, different day). |
| 7-8 | Press outreach to PhocusWire, Skift. |
| 9-10 | Analyze first 100 users. Where did they come from? What do they do? Where do they drop off? |
| 9-10 | Optimize referral system based on data (add incentive if conversion is low). |
| 11-12 | Double down on what worked. Kill what didn't. Write 20 more guides. |
