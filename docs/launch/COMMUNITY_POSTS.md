# Launch Day Community Posts

Drafts for Hacker News and Reddit launch posts. Adapt tone and length to each community. Do not cross-post identical content.

---

## 1. Show HN Post

**Title:** Show HN: Toqui -- AI travel companion with 800+ expert personas, $19 per trip

**Body:**

Hey HN, I'm a solo founder from PEI, Canada. I built Toqui because I was tired of two things: generic AI travel advice that reads like a TripAdvisor top-10 list, and subscription-based travel tools that charge $10-20/month even when you're not traveling.

Toqui is an AI travel companion that gives you access to 800+ expert personas -- a Tokyo street food guide, a Patagonia trekking specialist, a budget backpacking advisor for Southeast Asia -- instead of one generic chatbot. You chat with these personas to build a full itinerary with maps, weather, and bookings. It works before and during your trip: flight delayed? Ask for a re-plan. Restaurant closed? Get an alternative in seconds.

The pricing model is per-trip: $19 CAD unlocks everything for one trip. No subscription. If you travel twice a year, you pay $38 instead of $120-240. Subscriptions are available for frequent travelers ($9.99/mo or $17.99/mo) but they're optional, not the default.

Tech stack for those interested: the frontend is a single Expo (React Native) codebase targeting web, iOS, and Android simultaneously. The backend is Go with ConnectRPC (protobuf-based RPC that works over HTTP/1.1 -- no gRPC proxy needed). AI orchestration uses Gemini as the primary model with Claude as fallback. Streaming chat uses server-sent events over ConnectRPC, which was a surprisingly clean pattern. Auth is stateless Bearer tokens with automatic refresh -- no cookies anywhere. The whole thing runs on Cloud Run with Cloudflare in front.

On privacy: I treat all travel data as potentially sensitive under GDPR Article 9 (destinations can reveal religion, health conditions, political activity). Analytics are PostHog EU-hosted, cookie-less, no Google Analytics. We never sell data. Session replay masks all text inputs. This is a Canadian company subject to PIPEDA, and I apply GDPR as the baseline for all users regardless of location.

The referral system gives both sides a free Pro trip unlock. Shared trip pages work as public landing pages even for non-users.

Try it at https://app.toqui.travel -- the free tier gives you 15 messages per trip so you can see the persona system and itinerary generation without paying.

Happy to answer questions about the ConnectRPC + Expo architecture, the persona system, or the per-trip pricing model.

---

## 2. Reddit r/travel Post

**Title:** I spent 6 months building a trip planner that actually works during the trip, not just before it

**Body:**

I travel a fair amount for a Canadian who lives on an island in the Atlantic, and the thing that always frustrated me about trip planning tools is that they disappear once you leave. You spend hours building an itinerary, then your flight gets delayed, a restaurant is closed, the weather changes -- and you're back to Googling on your phone.

I built Toqui to fix that. It's an AI travel companion that helps you plan and then stays with you during the trip. The part I'm most proud of is the persona system -- instead of one generic AI, you can talk to over 800 specialized travel experts. Planning food in Tokyo? Switch to a local food critic persona. Hiking in Patagonia? Talk to an adventure travel specialist. The recommendations are noticeably better than asking a general-purpose AI because each persona has deep context about its niche.

It generates day-by-day itineraries with maps, weather for your specific dates, and you can export everything to PDF or your calendar. If you're traveling with others, you can share the trip and everyone can contribute -- the AI synthesizes the group's preferences.

The pricing was important to me: it's $19 CAD per trip, not a monthly subscription. I take maybe 3-4 trips a year. Paying $10-20/month for a travel tool I use a few weeks out of twelve felt wrong. So I made it pay-per-trip. There's a free tier with 15 messages so you can try the planning flow before committing.

If anyone wants to try it: https://app.toqui.travel. I'd genuinely appreciate feedback from people who plan real trips -- that's who I built this for.

---

## 3. Reddit r/solotravel Post

**Title:** Built a trip planning tool with AI "local experts" -- solo travelers, would love your feedback

**Body:**

Solo travel planning has a specific problem that group travelers don't face: you don't have anyone to bounce ideas off. No one to say "actually, that neighborhood is sketchy at night" or "skip that tourist trap, the place two blocks over is better and half the price."

I built Toqui to be that person. It's an AI travel companion with 800+ expert personas -- not one generic chatbot, but specialized advisors. A nightlife persona that knows which bars in Bangkok are actually worth going to on a Tuesday. A solo female travel safety advisor. A budget street food guide for Mexico City. You switch between them as your questions change.

The part that matters most for solo travelers: it works during the trip. Plans change constantly when you're alone -- you meet people at a hostel and want to adjust tomorrow, or a place is closed and you need a backup in 30 seconds. Toqui handles that because it knows your full itinerary context.

It costs $19 CAD per trip (not monthly). Free tier lets you try 15 messages to see if the persona recommendations are actually useful before paying. You can export itineraries to PDF or calendar, which I use constantly when I'm offline or in spotty service areas.

I'm a solo founder in Canada and I built this because I solo travel and nothing else worked the way I wanted. Would love honest feedback from this community: https://app.toqui.travel

---

## 4. Reddit r/digitalnomad Post

**Title:** Per-trip AI travel planner with calendar export -- built for people who plan more than 2 trips a year

**Body:**

If you're moving between cities every few weeks, the subscription model for travel tools makes even less sense than usual. You're paying $10-20/month for something you need intensely for 2-3 days of planning, then not at all until the next move.

I built Toqui as a per-trip AI travel planner: $19 CAD unlocks everything for one trip. For frequent travelers there are optional subscriptions ($9.99/mo or $17.99/mo) but the per-trip option exists because not everyone wants another recurring charge.

What makes it useful for the nomad workflow: you can manage multiple trips simultaneously, export any itinerary to ICS (calendar) or PDF, and the AI has 800+ expert personas so you get actually specific advice -- not "visit the old town" but recommendations from personas who know the local transit system, the coworking scene, or which neighborhood has reliable wifi and good food within walking distance.

It works during the trip too. When your plans shift (which they always do), you can re-plan on the fly with full itinerary context. Shared trips let you coordinate with other nomads if you're meeting up somewhere.

The whole thing runs as a web app plus iOS and Android from a single codebase, so it works on whatever device you have handy. Privacy-first: GDPR baseline for everyone, no tracking cookies, no data selling, hosted in Canada and EU.

Free tier gives you 15 messages per trip to test it: https://app.toqui.travel

---

## 5. Reddit r/shoestring Post

**Title:** $19 per trip vs $10-20/month -- built a budget-friendly AI trip planner

**Body:**

Most AI travel planners charge a monthly subscription. If you travel twice a year, that's $120-240 for maybe 2 weeks of actual use. I built Toqui with per-trip pricing instead: $19 CAD (about $14 USD) unlocks everything for one trip. Use it, plan your trip, done. No recurring charges.

The free tier gives you 15 messages per trip, which is enough to get a basic itinerary with maps and weather. Pro unlocks unlimited messages, 800+ expert personas (including budget travel specialists who actually know cheap eats and free activities), and export to PDF or calendar.

One thing I want to be transparent about: Toqui includes affiliate booking links for hotels, flights, and activities. But the AI recommendations are never biased by commissions -- it recommends what fits your budget and preferences regardless. Pro users get the same unbiased recommendations. The affiliate links are clearly disclosed and you're never pushed toward a more expensive option because it pays us more.

Worth a look if you're planning a trip and don't want another subscription: https://app.toqui.travel
