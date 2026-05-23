# Persona: N-14 — Status Machine Enforcement (Structural)

## Purpose (PR #198 verification)

Explicit verification of the trip status machine enforcement and the
**new sentinel-error Connect code mapping**. Run 5 R-07 and N-08 both
confirmed the status machine rejected invalid transitions BUT returned
`CodeInternal` instead of `CodeFailedPrecondition`/`CodeInvalidArgument`.
PR #198 introduced `ErrInvalidStatusTransition` and
`ErrInvalidInitialStatus` sentinels with proper handler mapping.

This is a structural test with no AI chat.

## What to Test

All tests use direct ConnectRPC calls.

### Phase 1: Valid forward path (sanity)

1. `CreateTrip` title "N-14 status walk", no status field set
   - Expect: OK, trip created with `TRIP_STATUS_PLANNING`
2. `CreateTrip` title "N-14 active start", status `TRIP_STATUS_ACTIVE`
   - Expect: OK, trip created with `TRIP_STATUS_ACTIVE`
   - This verifies the PR #197 status-at-create feature still works post-PR #198
3. `UpdateTrip` trip-1 → `TRIP_STATUS_ACTIVE`
   - Expect: OK
4. `UpdateTrip` trip-1 → `TRIP_STATUS_COMPLETED`
   - Expect: OK
5. `UpdateTrip` trip-2 → `TRIP_STATUS_COMPLETED`
   - Expect: OK

### Phase 2: Rejected transitions — MUST return FailedPrecondition

For each of the following, assert the Connect error code is
`failed_precondition`, NOT `internal`. In the report, include the exact
error code string returned for each case.

6. `UpdateTrip` trip-1 (COMPLETED) → `TRIP_STATUS_PLANNING`
   - Expected code: `failed_precondition`
7. `UpdateTrip` trip-1 (COMPLETED) → `TRIP_STATUS_ACTIVE`
   - Expected code: `failed_precondition`
8. `UpdateTrip` trip-2 (COMPLETED) → `TRIP_STATUS_PLANNING`
   - Expected code: `failed_precondition`

### Phase 3: Invalid initial status — MUST return InvalidArgument

9. `CreateTrip` title "Cannot start completed", status `TRIP_STATUS_COMPLETED`
   - Expected code: `invalid_argument`

### Phase 4: Other unchanged behaviour (regression guards)

10. `GetTrip` with a valid but non-existent UUID
    - Expected code: `not_found`
11. `GetTrip` with `"not-a-uuid"`
    - Expected code: `invalid_argument` (buf.validate)

## Assertions

- Phase 2 tests return `internal` → **P1 bug** (regression of PR #198 fix). Include the exact API command and response body.
- Phase 3 returns anything other than `invalid_argument` → **P1 bug**.
- Phase 1.2 (CreateTrip with ACTIVE) fails or trip is not ACTIVE on GetTrip → **P0 bug** (regression of PR #197 fix).
- Phase 4 codes change from Run 5 baseline → **P1 regression**.

## Booking Artifacts

None.

## Report expectations

The report's `bugs[]` should be empty if PR #198 landed correctly. The
`feature_coverage` array must include
`["status_machine_enforcement", "create_trip_active_status", "status_error_codes"]`.

Use `usefulness_evaluation` as:
- All scores 0 except `trip_creation_score` and `overall_score`.
- `trip_creation_score` = 5 on pass, 3 on any Phase 2/3 failure, 1 on any Phase 1 failure.
- `would_use_again` = true regardless (this is a structural test, not a user flow).
