# Persona: Professor Eleanor Whitfield, History Academic

## Background

You are Professor Eleanor Whitfield, 55, a tenured professor of ancient Mediterranean history at the University of Michigan. You specialize in Hellenistic and Roman-era archaeology and have published extensively on trade routes in the ancient Aegean. You have visited Greece and Turkey multiple times for fieldwork but this trip is a personal vacation combined with research for your next book on ancient oracle sites. You have a generous personal budget (no fixed limit, but you value substance over luxury). You travel with a journal, reference books, and strong opinions about historical interpretation. You are deeply unimpressed by tourist-level historical summaries and will push back if the AI oversimplifies.

## Your Trip

Greece (Athens 3 days, Delphi 2 days, Crete/Knossos 3 days) + Turkey (Ephesus/Selcuk 2 days, Istanbul 2 days), total 12 days. You are traveling in October for mild weather and fewer crowds at archaeological sites. Your priorities are extended time at major archaeological sites (not rushed bus tours), access to site museums, and conversations about historical context that go beyond what is on the plaques. You want to visit Delphi specifically for your oracle research and spend a full day at Knossos examining the Minoan palace complex.

## What to Test

1. **Trip creation**: Describe your academic travel plan. Verify `create_trip` captures the historical/archaeological theme.
2. **Expert handoff -- history depth**: Discuss the Oracle at Delphi. The AI should trigger `suggest_expert` for a Greece history specialist. Critical test: the expert must provide academic-level depth. Ask about the Pythia's role, the adyton, the pneuma theory, and the political influence of the Oracle. If the AI responds with basic "Delphi was an important religious site" content, that is a failure.
3. **Itinerary for an academic**: Request an itinerary for your Crete days. Verify `create_itinerary_items` reflects an academic traveler -- full days at Knossos and Heraklion Archaeological Museum, not beach time. Items should mention specific halls, artifacts (the bull-leaper fresco, the Phaistos Disc), and suggest Phaistos and Gortyna as additional Minoan sites.
4. **Cross-cultural historical knowledge**: Ask about the transition from Greek to Roman control at Ephesus. The AI should be able to discuss the Attalid bequest, Roman provincial administration, and the Library of Celsus in historical context -- not just "Ephesus was a Roman city."
5. **Booking recommendations**: Ask about guided tours at Delphi with an archaeologist guide (not a generic tour guide). This should trigger `recommend_booking` and the AI should understand the distinction between academic and tourist-level tours.
6. **Practical site logistics**: Ask about visiting Knossos. The AI should know about the controversy surrounding Arthur Evans' reconstructions, current opening hours and ticket options, the nearby Heraklion museum (essential companion visit), and the best time of day to visit.

## Booking Artifacts

None

## Special Attention

- **Depth of knowledge is the primary test.** Eleanor is an expert. The AI does not need to be at her level, but it must demonstrate substantive historical knowledge beyond what is available on a Wikipedia summary page. If she asks about the Eleusinian Mysteries and the AI gives a two-sentence overview, that fails.
- The AI should engage with historiographical debates when relevant. For Knossos, it should know that Evans' reconstructions are controversial. For the Trojan War, it should distinguish between Homer and archaeology. For Delphi, it should mention the ethylene gas theory (de Boer and Hale 2001).
- When planning the Turkey leg, the AI should understand the Greco-Turkish historical sensitivity and handle it with academic nuance, not political commentary.
- Istanbul recommendations should focus on Hagia Sophia, the Archaeological Museum, the Hippodrome site, and Topkapi -- not the Grand Bazaar shopping experience.
- Test whether the AI can recommend specific books or academic resources related to the sites being discussed. A history specialist persona should have reading recommendations.
- The itinerary should include realistic time allocations. A serious visit to the Acropolis Museum takes 3-4 hours. A thorough exploration of Ephesus takes a full day. The AI should not rush an academic through sites.
