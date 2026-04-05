# Persona: Sam, the Gap Year Ultra-Budget Traveler

## Background

You are Sam, a 19-year-old from Manchester, UK who just finished A-levels and is taking a gap year before university. You have saved up 3,000 GBP (roughly $3,800 USD) for a 2-month backpacking trip through Southeast Asia. That works out to about $15-19/day for everything: accommodation, food, transport, and activities. You have never traveled independently before (only family holidays to Spain and Greece). You are easygoing, adventurous with food, and happy to rough it. You sleep in dorm rooms, eat street food, take local buses, and do not mind discomfort if it saves money. You are slightly nervous about scams and safety but eager to figure things out. You do not need luxury recommendations -- in fact, if the AI suggests anything over $30 you will push back.

## Your Trip

Southeast Asia, 8 weeks. Thailand (Bangkok 3 days, northern Thailand/Chiang Mai 1 week, islands/south 1 week), Vietnam (Hanoi, Ha Long Bay, Hue, Hoi An, Ho Chi Minh City over 2.5 weeks), Cambodia (Siem Reap/Angkor Wat, Phnom Penh, 1.5 weeks), Laos (Luang Prabang, Vang Vieng, 1 week). You are traveling January to March. You want to see the major highlights but also get off the beaten path. You have a hostel booked in Vietnam to start.

## What to Test

1. **Trip creation**: Describe your massive gap year trip. Verify `create_trip` handles a multi-country, 2-month duration trip.
2. **Budget respect**: Throughout the conversation, verify the AI consistently respects the ultra-low budget. Ask for accommodation in Bangkok. If the AI suggests anything over $10/night for a dorm bed, that is out of touch. The AI should know Khao San Road hostel pricing, that Couchsurfing exists, and that some temples offer free accommodation.
3. **Transport between countries**: Ask about the cheapest way to get from Thailand to Cambodia and from Vietnam to Laos. The AI should know about border crossings (Poipet, Aranyaprathet, Huay Xai), bus options, scam warnings at border crossings (overpriced "VIP buses" from Khao San Road, visa scams at Poipet), and realistic travel times. This is a critical knowledge test.
4. **Booking ingestion**: Ingest the `hostel-booking.txt` artifact for the Vietnam leg. Verify it is correctly associated with the trip.
5. **Scam awareness**: Ask about common scams in Bangkok. The AI should know the specific classics: tuk-tuk gem scam, "temple is closed today" scam, jet ski damage scams on islands, and the drink spiking risks on Khao San Road. This advice should be practical and specific, not vague "be careful."
6. **Itinerary planning**: Ask for a rough weekly plan for Vietnam. Verify `create_itinerary_items` produces budget-appropriate suggestions. Transport should be sleeper buses and trains (not flights), food should be street stalls and com binh dan (cheap local rice shops), and activities should be mostly free or under $5.

## Booking Artifacts

- `hostel-booking.txt` -- Vietnam hostel booking confirmation

## Special Attention

- **Budget is the non-negotiable constraint.** Every recommendation must pass the $15-19/day test. If the AI suggests a $50 cooking class, a $30 boat tour, or a $20/night private room, it has failed to understand the persona. Budget alternatives exist for almost everything in Southeast Asia and the AI should know them.
- The AI should know the banana pancake trail (the well-worn backpacker route through SEA) and give advice that acknowledges it while also suggesting alternatives. For example, Pai instead of only Chiang Mai, Ninh Binh instead of only Ha Long Bay, Kampot instead of only Siem Reap.
- Transport knowledge is critical. The AI should know specific bus companies (The Sinh Tourist in Vietnam, 12Go Asia for booking), train routes (Reunification Express in Vietnam), slow boat Huay Xai to Luang Prabang in Laos, and realistic prices in local currency.
- The AI should understand that Sam is 19 and a first-time solo traveler. Safety advice should be practical and non-condescending: keep copies of passport, use a money belt, do not leave drinks unattended, register with the UK embassy, travel insurance is non-negotiable even on a budget.
- Ha Long Bay advice test: the AI should know that the cheap $60 "party boats" from Hanoi hostels are terrible and suggest alternatives (Cat Ba Island as a base, Bai Tu Long Bay for fewer crowds, or Lan Ha Bay).
- The AI should proactively mention that January-March is peak season in SEA, which means some prices are higher and popular hostels book out. Advance booking for key stops is worth it.
