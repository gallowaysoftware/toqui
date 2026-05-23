# Persona: N-15 — UpdateItinerary Day-Metadata Round Trip (Structural)

## Purpose (PR #198 verification)

Run 5 R-07 and N-05 confirmed `TripService/UpdateItinerary` was newly
implemented (PR #197) but dropped the `ItineraryDay.summary` and
`ItineraryDay.date` fields on the wire. PR #198 threads those through a
`ReplaceItineraryItem` struct and stashes them in the
`itinerary_items.metadata` JSONB under `day_summary`/`day_date` keys.

This persona verifies end-to-end round-trip: write → read → assert
equality for BOTH day-level fields.

## What to Test

Structural test with no AI chat.

### Phase 1: Setup

1. `CreateTrip` title "N-15 day-metadata roundtrip"
2. No initial itinerary — go straight to UpdateItinerary.

### Phase 2: Single-day write + read

3. `UpdateItinerary` with this Itinerary proto:
   ```json
   {
     "trip_id": "<TRIP_ID>",
     "itinerary": {
       "trip_id": "<TRIP_ID>",
       "days": [
         {
           "day_number": 1,
           "date": "2026-09-01",
           "summary": "Arrival in Lisbon",
           "items": [
             {"order_in_day": 1, "type": "flight", "title": "Arrive TAP 207", "description": "Land LIS 10:15"},
             {"order_in_day": 2, "type": "meal", "title": "Lunch at Cervejaria Ramiro", "description": "Classic Lisbon seafood"}
           ]
         }
       ]
     }
   }
   ```
4. `GetItinerary` and assert:
   - `itinerary.days[0].day_number == 1`
   - `itinerary.days[0].summary == "Arrival in Lisbon"` — NOT `"Day 1"` fallback
   - `itinerary.days[0].date == "2026-09-01"`
   - Two items present with correct titles and order

### Phase 3: Multi-day write

5. `UpdateItinerary` with three days, each with distinct summary and date:
   - Day 1: summary `"Arrival in Lisbon"`, date `"2026-09-01"`, 2 items
   - Day 2: summary `"Sintra day trip"`, date `"2026-09-02"`, 3 items
   - Day 3: summary `"Belém and the waterfront"`, date `"2026-09-03"`, 2 items
6. `GetItinerary` and assert ALL three days round-trip:
   - Correct `summary` for each day
   - Correct `date` for each day
   - Correct item count per day
   - Items appear in the declared `order_in_day`

### Phase 4: Summary-only (no date) and date-only (no summary)

7. `UpdateItinerary` with:
   - Day 1: summary `"Summary only"`, date empty, 1 item
   - Day 2: summary empty, date `"2026-09-05"`, 1 item
8. `GetItinerary` and assert:
   - Day 1 summary = `"Summary only"`, date = `""`
   - Day 2 summary = `"Day 2"` (the fallback, since none was provided), date = `"2026-09-05"`

### Phase 5: Empty itinerary (truncate semantics)

9. `UpdateItinerary` with `days: []`
10. `GetItinerary` and assert itinerary has zero days / zero items. This
    documents the truncate-on-empty semantics the proto comment promises.

### Phase 6: Replacement overwrites prior metadata

11. After Phase 3, `UpdateItinerary` with a fresh two-day itinerary with
    summaries "New plan day 1" / "New plan day 2".
12. `GetItinerary` — assert the old three-day content is GONE and only
    the new two days with new summaries are present. No stale items
    from the Phase 3 write.

## Assertions

- Any day returning the fallback `"Day N"` summary when a non-empty
  summary was sent → **P1 bug** (regression of PR #198 fix).
- Any day returning an empty `date` when a non-empty date was sent → **P1 bug**.
- Phase 5 not truncating to zero items → **P2 bug** (truncate semantics
  documented in proto but not enforced).
- Phase 6 returning stale items from the first UpdateItinerary → **P1 bug**
  (partial-replace instead of full-rewrite — breaks the RPC's contract).

## Report expectations

- `feature_coverage` must include `update_itinerary`, `day_summary_roundtrip`, `day_date_roundtrip`, `replace_semantics`.
- `usefulness_evaluation`: all AI scores 0 (no AI was used). `overall_score` = 5 on full pass, drops 1 per P1 violation.
