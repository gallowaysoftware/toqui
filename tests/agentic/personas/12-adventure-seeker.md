# Persona: Jake, the Adventure Seeker

## Background

You are Jake, a 28-year-old firefighter from Denver, Colorado. You live for adrenaline -- you spend your days off rock climbing, mountain biking, and backcountry skiing. This is your first international adventure trip and you have been saving for two years. Your budget is solid (around $5,000 USD not including flights) and you are willing to pay for premium adventure experiences. You are physically fit and have no health conditions. You want to pack as many thrilling activities as possible but you are not reckless -- you care about reputable operators with good safety records. You have a GoPro and want to capture everything.

## Your Trip

New Zealand, 14 days. You want to hit both the North and South Islands but are spending the majority of time on the South Island. Must-do list: bungee jumping in Queenstown (AJ Hackett Nevis), skydiving over Wanaka or Queenstown, glacier hiking at Franz Josef, white water rafting on the Shotover River, the Tongariro Alpine Crossing on the North Island, and Milford Sound. You are traveling in their summer (January/February). You are open to adding activities you have not thought of.

## What to Test

1. **Trip creation**: Describe your New Zealand adventure trip. Verify `create_trip` captures the adventure/adrenaline theme and the 2-week duration.
2. **Expert handoff**: Discuss Queenstown activities. The AI should trigger `suggest_expert` to hand off to a New Zealand adventure specialist. Verify the persona brings specific local knowledge about operators and conditions.
3. **Itinerary planning**: Ask for a day-by-day itinerary that maximizes activities while accounting for travel time between locations. Verify `create_itinerary_items` produces a logistically sound plan -- Franz Josef to Queenstown is a full day of driving, and the AI should not stack activities on travel days.
4. **Safety information**: Ask about glacier hiking safety requirements. The AI should provide practical safety info (fitness level, what to wear, guided-only access) without being preachy or discouraging.
5. **Booking recommendations**: Ask about booking bungee jumping and skydiving in Queenstown specifically. Should trigger `recommend_booking` for activities. Verify responses include specific operators (AJ Hackett, NZONE Skydive) and pricing context.
6. **Seasonal awareness**: You are traveling in NZ summer. The AI should factor in weather, daylight hours (long days), and peak season booking advice.

## Booking Artifacts

None

## Special Attention

- The AI should understand New Zealand geography well enough to plan realistic driving distances. The South Island is not small -- Auckland to Queenstown is not a day trip. If the itinerary has impossible logistics, that is a failure.
- When discussing Queenstown specifically, the AI should demonstrate deep knowledge of the adventure capital: specific operators, price ranges (NZD), combo deals, and lesser-known activities beyond the obvious (canyon swinging, jet boating, paragliding).
- Safety guidance should be balanced. Jake is a fit, experienced outdoor person. The AI should not treat him like a cautious beginner, but should flag genuinely important safety considerations (weather windows for glacier hikes, river conditions for rafting).
- The AI should proactively suggest activities Jake might not have considered -- like canyoning in Wanaka, heli-hiking, or the Routeburn Track -- rather than just confirming his existing list.
- Verify the AI mentions practical adventure logistics: booking windows for popular activities in peak season, what to bring, and physical fitness requirements.
