# Persona: Structural — REST Endpoint Exerciser

## Background

This is a structural test persona with no AI chat interaction. The goal is to exercise every public REST endpoint on the backend, verify correct response shapes, and confirm that error handling works as expected. Use curl (not grpcurl) for all HTTP endpoints. This test validates the REST API surface independently from the gRPC/ConnectRPC layer.

## Your Trip

A minimal trip created via the CreateTrip RPC solely to support sharing tests. Title: "REST Test Trip", destination: "Spain." No itinerary or chat needed.

## What to Test

### Phase 1: Health Endpoints (No Auth Required)

1. `GET /healthz` — Verify response body contains `"status":"ok"` or equivalent. Verify HTTP 200.
2. `GET /health` — Verify response contains component-level status information (database connectivity, etc.). Verify HTTP 200. Record the response shape for baseline documentation.
3. `GET /livez` — Verify HTTP 200 (liveness probe).
4. `GET /readyz` — Verify HTTP 200 (readiness probe, confirms DB is reachable).

### Phase 2: Public Endpoints (No Auth Required)

5. `GET /api/guides` — Verify response is a JSON array (or object containing an array) of guides. Each guide should have at minimum a `slug` and `title` field. Record the number of guides returned.
6. `GET /api/guides/{first-slug}` — Take the first slug from the guides list and fetch the full guide. Verify it contains content (non-empty body/content field) and the title matches the list entry.
7. `GET /shared/{invalid-token}` — Use a fake token like `nonexistent-token-12345`. Verify the endpoint returns an error (HTTP 404 or equivalent), not a 500 or a crash.

### Phase 3: Authenticated Endpoints

All of these require the Bearer token from testctl.

8. `GET /api/usage` — Verify response contains `used` (integer), `limit` (integer), and `resets_at` (timestamp string) fields. Verify `used` >= 0 and `limit` > 0.
9. `GET /api/referral` — Verify response contains a `code` field (non-empty string). This is the user's referral code. Record the code for potential future tests.

### Phase 4: Trip Sharing Flow

Create the trip via CreateTrip RPC first, then test the sharing REST endpoints.

10. `POST /api/trips/share` — Send JSON body with the trip_id. Verify response contains a `share_token` or `token` field (non-empty string). Record the token.
11. `GET /shared/{token}` — Use the token from step 10. Verify response contains the trip data (title should be "REST Test Trip"). Verify HTTP 200.
12. `POST /api/trips/unshare` — Send JSON body with the trip_id. Verify response indicates success (HTTP 200, body contains status ok or similar).
13. `GET /shared/{token}` — Re-fetch with the same token after unsharing. Verify it now returns an error (HTTP 404 or equivalent). The old token must be invalidated.

### Phase 5: Error Handling

14. `GET /api/usage` without auth — verify HTTP 401, not 500.
15. `POST /api/trips/share` without auth — verify HTTP 401.
16. `POST /api/trips/share` with auth but invalid trip_id — verify HTTP 400 or 404, not 500.

## Booking Artifacts

None — this is a REST endpoint test with no booking data.

## Special Attention

- Every endpoint must return a well-formed JSON response (or appropriate HTTP status for errors). Any HTTP 500 response is a P0 bug — it means an unhandled error reached the client.
- Response shape consistency matters. Document the exact fields returned by each endpoint so future regressions can be caught. If a field is missing that the frontend depends on, flag as P1.
- Auth enforcement is critical. Every authenticated endpoint MUST return 401 when called without a token. If any authenticated endpoint returns data without auth, flag as P0 security issue.
- The sharing flow (share -> fetch -> unshare -> fetch-fails) must work as a complete lifecycle. If unsharing does not invalidate the token, flag as P1 security issue (shared data still accessible after user revokes sharing).
- Health endpoints must respond quickly (under 500ms). If /readyz or /healthz is slow, it could cause container orchestration issues in production.
- Record every response body shape in the report. This serves as living API documentation and a regression baseline.
