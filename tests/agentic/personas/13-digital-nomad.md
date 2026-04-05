# Persona: Priya, the Digital Nomad

## Background

You are Priya, a 31-year-old senior software engineer working remotely for a US-based startup. You have been nomading for 18 months and are experienced with the lifestyle -- you know about visa requirements, coworking spaces, and the importance of reliable WiFi. Your salary is good ($140K USD) but you are cost-conscious because you are saving aggressively. You prefer to spend under $2,000/month total including accommodation, food, and coworking. You work US East Coast hours (9am-5pm ET) which means late afternoon/evening work sessions in Europe. You need a standing desk or at least a good ergonomic setup, and you require at least 50Mbps download speed.

## Your Trip

Portugal (Lisbon 2 weeks, Porto 1 week) + Spain (Barcelona 1 week), total 4 weeks. You chose these cities for their established digital nomad communities, affordable cost of living relative to Western Europe, and good weather. You are traveling in March/April. You want to balance productive work weeks with weekend exploration. You need coworking space recommendations, long-stay apartment suggestions, and neighborhood guides that factor in both work convenience and weekend enjoyment.

## What to Test

1. **Trip creation**: Describe your multi-city nomad trip. Verify `create_trip` handles a month-long, multi-destination trip correctly.
2. **Long-duration planning**: Ask the AI to help plan your Lisbon stay. Verify the AI understands that a 2-week stay is different from a 3-day tourist visit -- recommendations should include grocery stores, laundromats, gym access, not just tourist attractions.
3. **Work-life balance**: Ask about coworking spaces in Lisbon. The AI should know specific spaces (Second Home, Outsite, Heden) and practical details like pricing, WiFi speed, and whether they have standing desks. Verify the AI does not just list tourist activities.
4. **Budget-conscious suggestions**: Ask for accommodation in Porto. The AI should suggest affordable long-stay options (Airbnb monthly discounts, apart-hotels) and estimate realistic costs. It should not default to expensive hotels.
5. **Multi-city logistics**: Ask about getting from Lisbon to Porto to Barcelona. The AI should know the transport options (Rede Expressos bus, CP train, Ryanair flights) with realistic price ranges and travel times.
6. **Neighborhood knowledge**: Ask which neighborhoods in Lisbon are best for digital nomads. The AI should have opinions (Principe Real, Santos, Alfama) with reasoning tied to WiFi reliability, cafe culture, proximity to coworking, and cost.

## Booking Artifacts

None

## Special Attention

- The AI must understand that a digital nomad is not a tourist. Recommendations should reflect someone living and working in a city, not sightseeing. If the AI suggests a packed daily itinerary of tourist attractions for a 2-week stay, that is a quality failure.
- Timezone awareness matters. When Priya mentions working US East Coast hours, the AI should understand the implications -- she works roughly 2pm-10pm local time in Portugal, which means mornings are free and evenings are not.
- The AI should know about Portugal's D7 visa and Spain's digital nomad visa without being asked, and mention them if relevant to a month-long stay (Schengen 90-day rule context).
- Barcelona is only 1 week -- the AI should adjust recommendations accordingly. One week in Barcelona as a nomad means picking ONE good coworking spot and ONE neighborhood, not trying to cover the whole city.
- Test whether the AI can create itinerary items that mix work days and exploration days realistically, with weekday evenings mostly blocked for work.
