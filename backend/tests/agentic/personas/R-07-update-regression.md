# Persona: Structural Test — UpdateTrip COALESCE Regression

## Background

This is NOT a conversational persona. This is a pure structural regression test that exercises the TripService UpdateTrip RPC directly, without any AI chat interaction. The purpose is to verify that partial updates via UpdateTrip do not wipe fields that were not included in the update request. This is a known regression area due to COALESCE handling in the SQL queries.

You are acting as a QA automation engineer running a deterministic test sequence against the API. There is no personality, no travel story, and no chat. Every step uses direct RPC calls, and every step has an explicit expected outcome that must be verified.

## Your Trip

A synthetic test trip created purely for the purpose of exercising the UpdateTrip partial update logic.

## What to Test

Execute the following steps IN EXACT ORDER. Each step has explicit assertions.

### Step 1: Create a Trip with Full Details

Call **CreateTrip** with:
- Title: "COALESCE Regression Test Trip"
- Description: "This trip tests that partial updates do not wipe unrelated fields."

Verify the trip is created. Record the trip_id.

Call **GetTrip** and verify:
- title = "COALESCE Regression Test Trip"
- description = "This trip tests that partial updates do not wipe unrelated fields."
- status = PLANNING (default)

### Step 2: Update ONLY Status to ACTIVE

Call **UpdateTrip** with:
- trip_id: (from step 1)
- status: ACTIVE
- Do NOT set title or description in the request.

Call **GetTrip** and verify ALL of the following:
- status = ACTIVE (updated)
- title = "COALESCE Regression Test Trip" (preserved, NOT empty)
- description = "This trip tests that partial updates do not wipe unrelated fields." (preserved, NOT empty)

If title or description is empty or missing, this is the COALESCE regression bug. Report as CRITICAL FAILURE.

### Step 3: Update ONLY the Title

Call **UpdateTrip** with:
- trip_id: (from step 1)
- title: "Updated Title Only"
- Do NOT set status or description in the request.

Call **GetTrip** and verify ALL of the following:
- title = "Updated Title Only" (updated)
- status = ACTIVE (preserved from step 2, NOT reset to PLANNING)
- description = "This trip tests that partial updates do not wipe unrelated fields." (preserved)

### Step 4: Update Status to COMPLETED

Call **UpdateTrip** with:
- trip_id: (from step 1)
- status: COMPLETED

Call **GetTrip** and verify ALL of the following:
- status = COMPLETED (updated)
- title = "Updated Title Only" (preserved from step 3)
- description = "This trip tests that partial updates do not wipe unrelated fields." (preserved from step 1)

### Step 5: Verify via ListTrips

Call **ListTrips** and find the test trip in the list. Verify the final state matches step 4 assertions.

## Booking Artifacts

None — this test does not involve bookings.

## Special Attention

- This test has ZERO tolerance for field-wiping. If any field that was not explicitly updated loses its value, the test fails. This is the most important assertion in the entire test.
- The test must use GetTrip after EVERY UpdateTrip call to verify state. Do not batch verifications.
- Status transitions must be valid: PLANNING -> ACTIVE -> COMPLETED. Do not attempt invalid transitions.
- This test should complete quickly since it does not involve AI calls. If it takes more than 30 seconds, something is wrong.
- Report exact field values in every assertion (what was expected vs what was returned) so debugging is straightforward if the test fails.
- This regression was previously identified in the `update-regression` AI test scenario. This structural test provides deterministic coverage without depending on AI behavior.
