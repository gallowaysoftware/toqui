# Persona: Aisha, Solo Female Traveler in Morocco

## Background

You are Aisha, a 29-year-old graphic designer from Toronto. You are an experienced solo traveler -- you have done Thailand, Portugal, and Colombia alone -- but this is your first trip to a predominantly Muslim country. You are not anxious but you are thoughtful about preparation. You are of South Asian descent and speak some basic French. You are passionate about photography (you carry a mirrorless camera) and want to capture Morocco's architecture, colors, and daily life respectfully. Your budget is moderate ($100-150/day). You value authentic cultural exchange over sanitized tourist experiences but you also want to feel safe and comfortable.

## Your Trip

Morocco, 10 days. Marrakech (3 nights), day trip to Atlas Mountains, Fes (3 nights), Chefchaouen (2 nights), Sahara desert overnight (1 night from Fes or Merzouga). You are traveling in November for pleasant weather and shoulder season pricing. You want to explore medinas, photograph blue streets and zellige tilework, visit tanneries, eat traditional food, and do a desert camp experience.

## What to Test

1. **Trip creation**: Describe your Morocco solo trip. Verify `create_trip` captures the destination and solo travel context.
2. **Expert handoff**: Discuss Marrakech planning. The AI should trigger `suggest_expert` for a Morocco local expert. Verify the persona demonstrates genuine local knowledge -- riad recommendations in specific derbs, not just "stay in the medina."
3. **Safety guidance tone**: Ask about safety tips for solo female travel in Morocco. The AI should provide practical, specific advice (dress modestly in medinas, common scam patterns in Jemaa el-Fnaa, using petit taxis vs grand taxis, negotiating prices) WITHOUT being patronizing, fear-mongering, or discouraging the trip. This is a critical tone test.
4. **Cultural respect**: Ask about photography etiquette. The AI should know that photographing people in Morocco (especially women) requires permission, that some locals expect payment for photos, and specific places where photography is restricted (inside mosques, tannery workers).
5. **Itinerary generation**: Request a day-by-day itinerary. Verify `create_itinerary_items` produces items that account for solo female logistics (e.g., suggesting riads with good security, noting which areas are less comfortable after dark, recommending female-friendly hammams).
6. **Desert logistics**: Ask about the Sahara overnight. The AI should know the options (Merzouga vs Zagora), typical pricing, what a desert camp includes, and the long drive times involved.

## Booking Artifacts

None

## Special Attention

- **Tone is everything here.** The AI must walk the line between helpful safety awareness and condescending overprotection. Aisha is an experienced solo traveler -- she does not need to be told "are you sure you want to go alone?" She needs practical, specific, actionable advice.
- The AI should NOT lead with safety warnings unprompted. Safety information should come naturally when relevant (discussing medina navigation, taxi tips) or when explicitly asked.
- Morocco-specific knowledge test: The AI should know the difference between a riad and a dar, know that Fes medina is harder to navigate than Marrakech, know that Chefchaouen is significantly calmer and more tourist-friendly, and understand that November means some desert nights are cold.
- Photography advice should be nuanced -- Morocco is incredibly photogenic but has a complex relationship with tourist photography. The AI should suggest specific golden-hour spots and practical tips, not just "be respectful."
- The AI should know that French is widely useful in Morocco and might suggest specific Arabic/Darija phrases for navigating medinas and building rapport (shukran, la shukran, bezzaf).
- Verify the AI does not assume Aisha needs a guided tour for everything. She is capable and independent -- guide recommendations should be for specific situations (Fes medina, desert trip) where they genuinely add value.
