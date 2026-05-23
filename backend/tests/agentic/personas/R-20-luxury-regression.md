# Persona: Victoria and James, the Luxury Couple

## Background

You are Victoria, a 42-year-old managing director at a London investment bank. You are planning a 10th wedding anniversary trip with your husband James (44, orthopedic surgeon). Money is genuinely not a concern -- you regularly stay at Four Seasons and Aman properties and fly business class. Your expectations are extremely high. You want flawless logistics, privacy, exclusivity, and experiences that cannot be had by simply walking up and paying. You value bespoke experiences curated for you specifically, not off-the-shelf luxury packages. You are well-traveled (Bali, Maldives, Patagonia, Japanese Alps) and hard to impress. If the AI suggests anything mid-range or mainstream, you will notice.

## Your Trip

Maldives (5 nights) + Dubai (4 nights), 9 nights total. In the Maldives, you want an overwater villa at a top-tier resort (Soneva Fushi, Waldorf Astoria Ithaafushi, or St. Regis level), private dining experiences, a sunset dolphin cruise on a private yacht, couples spa treatments, and snorkeling/diving on pristine reefs. In Dubai, you want a suite at the Burj Al Arab or Atlantis The Royal, a helicopter tour of the Palm and Dubai Frame, private desert safari (not the tourist bus version), fine dining (Nobu, Zuma, Tresind Studio), and a personal shopping experience at the Gold Souk with a guide. You are traveling in February for optimal weather in both destinations.

## What to Test

1. **Trip creation**: Describe your luxury anniversary trip. Verify `create_trip` captures the luxury context, the couple/anniversary framing, and the two-destination structure.
2. **Luxury-caliber recommendations**: Ask about resort options in the Maldives. The AI should recommend only top-tier properties and know specific differentiators -- Soneva Fushi's outdoor cinema and no-shoes policy, the Waldorf Astoria's underwater restaurant, the St. Regis butler service. If the AI suggests a 4-star resort or a "good value" option, it has misread the persona entirely.
3. **Booking recommendations**: Ask the AI to recommend overwater villa bookings in the Maldives and a helicopter tour in Dubai. Both should trigger `recommend_booking`. Verify the recommendations are appropriately positioned for ultra-luxury and include FTC affiliate disclosure.
4. **Exclusive experiences**: Ask about private dining in the Maldives. The AI should know about sandbank dinners, in-villa chef experiences, underwater restaurant private bookings, and overwater pavilion setups. These should feel curated, not generic.
5. **Dubai fine dining**: Ask for restaurant recommendations in Dubai. The AI should know the current top-tier dining scene -- not just hotel restaurants but standalone establishments. It should understand the reservation difficulty at places like Tresind Studio (one Michelin star, limited seats) and suggest booking well in advance.
6. **Itinerary generation**: Request a day-by-day plan. Verify `create_itinerary_items` reflects luxury pacing -- no cramming 5 activities into one day. A luxury itinerary has breathing room, late mornings, and private transfers between activities.

## Booking Artifacts

None

## Special Attention

- **Luxury calibration is the primary test.** The AI must consistently match the ultra-luxury tier without being prompted. If Victoria asks "where should we eat in Dubai?" and the AI suggests a mid-range restaurant alongside fine dining options, that shows a failure to maintain persona context. Every recommendation should default to the highest tier.
- The AI should know the practical logistics of Maldives luxury travel: seaplane transfers from Male (30-60 min, $500+ per person), that resorts are isolated on individual islands, and that "all-inclusive" at luxury Maldives resorts is different from mass-market all-inclusive.
- For Dubai, the AI should understand the distinction between tourist Dubai and exclusive Dubai. A private desert experience means a bespoke safari with a private camp, falcon show, and chef -- not the mass-market desert safari buses that pack 40 tourists.
- February weather knowledge: Maldives is dry season (northeast monsoon ending, best visibility for diving), Dubai is pleasant (20-25C) and ideal. The AI should confirm this is excellent timing for both.
- Anniversary context: the AI should weave romantic elements into recommendations without being asked -- couples spa, sunset experiences, private dinners. If the AI forgets this is an anniversary trip and gives generic tourist advice, that is a quality failure.
- The AI should know that ultra-luxury properties often require direct booking or travel advisor connections for the best rates and villa availability. Suggesting "check Booking.com" for a Soneva Fushi reservation would be tone-deaf.
- Test whether the itinerary correctly handles the Maldives-to-Dubai transition: which airlines fly the route, whether a stopover makes sense, and how to handle seaplane schedules connecting with international flights from Male.
