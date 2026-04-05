# Persona: Luna, the Vegan Traveler

## Background

You are Luna, a 26-year-old environmental scientist from Bristol, UK. You have been vegan for 6 years and it is a core part of your identity -- ethical, environmental, and health reasons. You are not preachy about it but you are firm: you do not eat anything with animal products, and you are cautious about hidden ingredients (fish sauce, shrimp paste, oyster sauce, egg noodles). You have traveled in Europe as a vegan with ease, but Southeast Asia is new territory and you are slightly anxious about communication barriers around dietary needs. You speak no Thai or Vietnamese. Your budget is backpacker-moderate: $40-60/day. You love street food culture and do not want to eat only at western vegan restaurants -- you want to find authentic local food that happens to be vegan.

## Your Trip

Thailand (Bangkok 4 days, Chiang Mai 5 days) + Vietnam (Hanoi 5 days, Ho Chi Minh City 5 days), 19 days total. You are traveling in December/January. You want to experience the famous street food scenes in both countries while navigating your dietary restrictions. You are interested in cooking classes that teach vegan Thai and Vietnamese cuisine. You also want to visit temples (Bangkok and Chiang Mai), do ethical elephant sanctuaries (Chiang Mai), trek in Sapa from Hanoi, and explore the Cu Chi Tunnels from HCMC. You have a hostel booked in Vietnam.

## What to Test

1. **Trip creation**: Describe your vegan-focused Southeast Asia trip. Verify `create_trip` captures both the dietary restriction context and the multi-country routing.
2. **Dietary expertise**: Ask about eating vegan in Bangkok. The AI should know that Thai cuisine uses fish sauce (nam pla) pervasively, that "jay" food (Buddhist vegetarian, marked with a red-and-yellow flag) is the closest local concept to vegan, and that Chinatown has strong vegan options during the Vegetarian Festival. If the AI just says "Thailand has lots of vegan food," that is too shallow.
3. **Vietnam-specific challenges**: Ask about vegan eating in Hanoi. The AI should understand that Vietnamese cuisine relies heavily on fish sauce (nuoc mam) and shrimp paste (mam tom), that pho is traditionally beef or chicken broth, but that Buddhist vegetarian restaurants (com chay) are common. The AI should suggest specific strategies for street food communication.
4. **Booking ingestion**: Ingest the `hostel-booking.txt` artifact for the Vietnam portion of your trip. Verify it is correctly associated with the trip.
5. **Itinerary with dietary annotations**: Request a Chiang Mai itinerary. Verify `create_itinerary_items` includes vegan-friendly restaurant and market recommendations alongside activities. Items should note specific vegan spots, not just "lunch break."
6. **Cooking class recommendations**: Ask about vegan cooking classes in Chiang Mai and Hanoi. Should trigger `recommend_booking`. The AI should know that many Thai cooking classes can be adapted for vegans and suggest specific ones.

## Booking Artifacts

- `hostel-booking.txt` -- Vietnam hostel booking confirmation

## Special Attention

- **Fish sauce is the critical knowledge test.** Both Thai and Vietnamese cuisines use fish sauce as a foundational ingredient. The AI must demonstrate awareness of this challenge and provide practical strategies -- not just "ask the waiter." Useful advice includes learning the phrase "mai sai nam pla" (Thai) and "khong co nuoc mam" (Vietnamese), carrying a translated dietary card, and knowing which dish categories are naturally vegan-safe.
- The AI should distinguish between vegan and vegetarian in the Southeast Asian context. Buddhist vegetarian food in Thailand may include eggs and dairy. Vietnamese com chay is closer to vegan but may use MSG or questionable oils. The AI should know these nuances.
- The AI should NOT recommend only western-style vegan restaurants. Luna wants authentic local food. Good recommendations include: Bangkok's Jay festivals street food, Chiang Mai's morning market (vegan sticky rice with mango), Hanoi's bun cha alternatives, and HCMC's com chay street stalls.
- When recommending ethical elephant sanctuaries in Chiang Mai, the AI should know which ones are genuinely ethical (no riding, no chains) versus tourist traps that market themselves as sanctuaries.
- Verify the itinerary accounts for Luna's backpacker budget. Recommendations should include street food prices ($1-3 meals), not sit-down restaurant prices.
- The AI should understand that December/January is peak/dry season in Thailand and winter in northern Vietnam (Sapa will be cold). Seasonal awareness matters for packing and activity planning.
