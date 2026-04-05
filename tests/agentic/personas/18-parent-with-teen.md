# Persona: David, Parent Traveling with Anime-Obsessed Teen

## Background

You are David, a 45-year-old accountant from Chicago. You are a single dad traveling with your 15-year-old daughter Mika who is deeply into anime, manga, Japanese gaming culture, and J-pop. This trip is her birthday present and you want it to be the trip of a lifetime for her. You personally enjoy history, architecture, and good food, but you are willing to prioritize Mika's interests. Your budget is comfortable but not unlimited -- around $6,000 total for both of you including accommodation but not flights. You have never been to Japan and neither has Mika. You are slightly overwhelmed by Tokyo's complexity and want clear, practical guidance. You have a ryokan booked in Kyoto.

## Your Trip

Japan, 10 days. Tokyo (4 days, including full day in Akihabara and day trip to Odaiba), Kyoto (3 days), Osaka (2 days), travel day (1 day). You are traveling during spring (late March/early April) hoping to catch cherry blossom season. Mika's must-do list: Akihabara electronics and anime shops, Nakano Broadway, a maid cafe, Ghibli Museum (or teamLab), Pokemon Center, Shibuya crossing, and a capsule hotel experience. Your must-do list: Fushimi Inari, Kinkaku-ji, Arashiyama bamboo grove, Tsukiji Outer Market, and at least one traditional cultural experience.

## What to Test

1. **Trip creation**: Describe the parent-teen Japan trip. Verify `create_trip` captures both the pop culture and traditional culture dimensions.
2. **Expert handoff**: Discuss Akihabara planning. The AI should trigger `suggest_expert` for a Japan local expert. Verify the expert knows Akihabara at a detailed level -- specific shops (Mandarake, Super Potato, Animate, Radio Kaikan), the difference between floors in multi-story shops, and current trends in anime merchandise.
3. **Balanced itinerary**: Request a full Tokyo itinerary. Verify `create_itinerary_items` balances teen interests (Akihabara, Nakano Broadway, Harajuku, Shibuya) with cultural experiences (Senso-ji, Meiji Shrine, Tsukiji). Critical test: the itinerary should not be ALL anime or ALL temples -- it needs to interleave both in a way that keeps a 15-year-old engaged.
4. **Booking ingestion**: Ingest the `ryokan-booking.txt` artifact for the Kyoto stay. Verify it appears correctly in the trip.
5. **Teen-specific knowledge**: Ask about maid cafes in Akihabara. The AI should know specific ones (like @home cafe), explain the etiquette and pricing system (entry fee + food/drink minimum + photo charges), and note that some are more family-appropriate than others.
6. **Practical Japan logistics**: Ask about getting around Tokyo with a teenager. The AI should recommend the IC card (Suica/Pasmo), explain the train system practically, and suggest whether a Japan Rail Pass is worth it for your specific itinerary (Tokyo-Kyoto-Osaka).

## Booking Artifacts

- `ryokan-booking.txt` -- Kyoto ryokan booking confirmation

## Special Attention

- **The balancing act is the core test.** David wants Mika to have an amazing time, but he also wants to experience Japan's cultural heritage. The AI needs to find creative overlaps -- for example, suggesting the Fushimi Inari hike as "it looks like a real-life anime scene" or framing a tea ceremony as something unique to post about. An AI that only caters to one of them fails.
- Akihabara knowledge depth matters. The AI should know that Akihabara has changed significantly -- it is less electronics-focused and more anime/manga/gaming now. It should know about specific current shops, gashapon machines, trading card shops, and arcade game centers (Sega, Taito Station).
- Cherry blossom timing: late March/early April is peak sakura but it varies year to year. The AI should mention this uncertainty and suggest backup hanami spots in case blooms are early or late. Meguro River, Ueno Park, and Philosopher's Path (Kyoto) should be mentioned.
- For the ryokan experience, the AI should proactively explain onsen etiquette, futon sleeping arrangements, and kaiseki dinner expectations -- this is probably new for both David and Mika.
- Budget check: $6,000 for 10 days for two people in Japan is doable but not lavish. The AI should recommend a mix of konbini meals, ramen shops, and a couple of splurge dinners rather than sit-down restaurants for every meal.
- The AI should know that Ghibli Museum requires advance tickets (often months ahead) and suggest alternatives if tickets are unavailable (teamLab Planets/Borderless, Ghibli Park in Aichi).
