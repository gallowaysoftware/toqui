# Toqui Manual QA Runbook

Step-by-step guide for running a manual QA session against the local stack.
No Google OAuth required — we use `testctl` to create fake users with valid JWTs.

---

## Quick Start (TL;DR)

```bash
# 1. Start infra + backend
cd toqui-backend
make docker-up && make migrate-up   # if not already running
make run                             # starts backend on :8090

# 2. Start frontend (separate terminal)
cd toqui
EXPO_PUBLIC_API_URL=http://localhost:8090 pnpm web   # opens :8081

# 3. Create test user + inject token
cd toqui-backend
./scripts/qa-start.sh
# → prints a browser console snippet — paste it into DevTools at localhost:8081
```

---

## 1. Infrastructure Setup

### Prerequisites

| Tool | Install |
|------|---------|
| Docker Desktop | Running |
| Go 1.22+ | `brew install go` |
| grpcurl | `brew install grpcurl` |
| Bruno (optional) | https://www.usebruno.com |
| gcloud CLI | For GCP Secret Manager (`gcloud auth application-default login`) |

### Start Docker containers

```bash
cd toqui-backend
make docker-up       # postgres:5432, firestore emulator:8080
make migrate-up      # apply DB migrations (idempotent)
```

Verify containers:

```bash
docker ps | grep toqui-backend
# Should show: toqui-backend-postgres-1, toqui-backend-firestore-1
```

### Start the backend

```bash
make run             # loads env/.env.local, serves on :8090
```

Verify:

```bash
curl -s http://localhost:8090/healthz
# → {"status":"ok"}
```

CORS is already configured in `env/.env.local` for both `:3000` and `:8081` — no extra flags needed.

### Start the frontend

```bash
cd toqui
EXPO_PUBLIC_API_URL=http://localhost:8090 pnpm web
# Opens Expo web dev server on http://localhost:8081
```

---

## 2. OAuth Bypass (testctl + localStorage)

We bypass Google OAuth by creating a real DB user with a valid JWT.

### Create a test user

```bash
cd toqui-backend
go run ./cmd/testctl create-user \
  --name "Kyle QA" \
  --email "kyle-qa@toqui-test.local" \
  --ttl 8h

# Output:
# {"user_id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx", "token": "eyJ..."}
```

`testctl` creates the user in Postgres and sets `age_verified_at = NOW()` — this satisfies the backend's age interceptor without needing the `POST /auth/verify-age` flow.

### Use the script instead (recommended)

```bash
./scripts/qa-start.sh
# Checks Docker, waits for backend, creates user, prints snippet
```

Options:

```bash
./scripts/qa-start.sh --user-only              # Skip backend checks
./scripts/qa-start.sh --ttl 2h                 # Short-lived token
./scripts/qa-start.sh --name "Jane" --email "jane@toqui-test.local"
```

### Inject the token in Chrome DevTools

Open `http://localhost:8081` → **DevTools** (F12) → **Console** tab → paste:

```javascript
localStorage.clear();
localStorage.setItem('toqui_access_token', '<token from testctl>');
// DO NOT set toqui_refresh_token — see pitfalls below
localStorage.setItem('toqui_user', JSON.stringify({
  id: '<user_id from testctl>',
  email: 'kyle-qa@toqui-test.local',
  name: 'Kyle QA',
  tier: 'free'
}));
localStorage.setItem('toqui_age_verified', 'true');
localStorage.setItem('toqui_age_synced', 'true');
window.location.reload();
```

Or use the standalone file in `toqui/scripts/qa-inject.js` — fill in the values and paste.

### Critical pitfalls

| Pitfall | Detail |
|---------|--------|
| **Refresh token trap** | Setting `toqui_refresh_token` to ANY value causes `refreshTokens()` to call the backend, fail (invalid token), and wipe ALL auth state including the access token. **Leave it unset entirely.** |
| **Age gate** | Requires BOTH `toqui_age_verified=true` in localStorage AND `age_verified_at` in the DB. `testctl` sets the DB side; the snippet sets the client side. |
| **CORS** | Fixed in `env/.env.local` — both `:3000` and `:8081` are allowed. No extra flags needed for `make run`. |
| **Token TTL** | testctl generates access tokens only (no refresh). Use `--ttl 8h` for a full workday. |

### Cleanup

```bash
go run ./cmd/testctl cleanup-user --user-id <user-id>
```

---

## 3. API Testing with grpcurl

gRPC reflection is enabled — you can explore the API without a proto file.

### Discover services

```bash
TOKEN="eyJ..."   # from testctl
grpcurl -plaintext localhost:8090 list
grpcurl -plaintext localhost:8090 describe toqui.v1.TripService
```

### TripService

```bash
# Create
grpcurl -plaintext -H "Authorization: Bearer $TOKEN" \
  -d '{"title": "Tokyo QA", "start_date": "2025-10-01", "end_date": "2025-10-07"}' \
  localhost:8090 toqui.v1.TripService/CreateTrip

# → save trip_id as TRIP_ID=...

# List
grpcurl -plaintext -H "Authorization: Bearer $TOKEN" \
  -d '{}' localhost:8090 toqui.v1.TripService/ListTrips

# Get — use "id" not "trip_id"
grpcurl -plaintext -H "Authorization: Bearer $TOKEN" \
  -d "{\"id\": \"$TRIP_ID\"}" localhost:8090 toqui.v1.TripService/GetTrip

# Update — use "id" not "trip_id"
grpcurl -plaintext -H "Authorization: Bearer $TOKEN" \
  -d "{\"id\": \"$TRIP_ID\", \"title\": \"Tokyo QA — Updated\"}" \
  localhost:8090 toqui.v1.TripService/UpdateTrip

# Delete — use "id" not "trip_id"
grpcurl -plaintext -H "Authorization: Bearer $TOKEN" \
  -d "{\"id\": \"$TRIP_ID\"}" localhost:8090 toqui.v1.TripService/DeleteTrip

# Get itinerary — uses "trip_id" (consistent with other trip-scoped RPCs)
grpcurl -plaintext -H "Authorization: Bearer $TOKEN" \
  -d "{\"trip_id\": \"$TRIP_ID\"}" localhost:8090 toqui.v1.TripService/GetItinerary
```

> **Field naming quirk**: `GetTripRequest`, `UpdateTripRequest`, `DeleteTripRequest` use `id`.
> `GetItineraryRequest`, `UpdateItineraryRequest` use `trip_id`. This is inconsistent by design
> in the proto — don't guess, check with `grpcurl describe`.

### ChatService (streaming)

```bash
# Send message — streaming response
grpcurl -plaintext -H "Authorization: Bearer $TOKEN" \
  -d "{\"trip_id\": \"$TRIP_ID\", \"content\": \"Plan a full day in Tokyo focused on street food\"}" \
  localhost:8090 toqui.v1.ChatService/SendMessage

# Correct mode enum values:
# CHAT_MODE_PLANNING, CHAT_MODE_COMPANION, CHAT_MODE_SELECTION, CHAT_MODE_UNSPECIFIED
# CHAT_MODE_DEFAULT does NOT exist.

# List sessions (save session_id from SendMessage response)
grpcurl -plaintext -H "Authorization: Bearer $TOKEN" \
  -d "{\"trip_id\": \"$TRIP_ID\"}" localhost:8090 toqui.v1.ChatService/ListChatSessions

# Get history
grpcurl -plaintext -H "Authorization: Bearer $TOKEN" \
  -d "{\"trip_id\": \"$TRIP_ID\", \"session_id\": \"$SESSION_ID\"}" \
  localhost:8090 toqui.v1.ChatService/GetChatHistory
```

### BookingService

```bash
# Ingest (AI parses the raw_text)
grpcurl -plaintext -H "Authorization: Bearer $TOKEN" \
  -d "{\"trip_id\": \"$TRIP_ID\", \"raw_text\": \"Hotel: Park Hyatt Tokyo\nCheck-in: 2025-10-01\nCheck-out: 2025-10-04\nConf: PH-12345\"}" \
  localhost:8090 toqui.v1.BookingService/IngestBooking

# → save id as BOOKING_ID=...

# Get — use "id"
grpcurl -plaintext -H "Authorization: Bearer $TOKEN" \
  -d "{\"id\": \"$BOOKING_ID\"}" localhost:8090 toqui.v1.BookingService/GetBooking

# List
grpcurl -plaintext -H "Authorization: Bearer $TOKEN" \
  -d "{\"trip_id\": \"$TRIP_ID\"}" localhost:8090 toqui.v1.BookingService/ListBookings

# Extract a specific field with AI
grpcurl -plaintext -H "Authorization: Bearer $TOKEN" \
  -d "{\"booking_id\": \"$BOOKING_ID\", \"question\": \"What is the check-in date?\"}" \
  localhost:8090 toqui.v1.BookingService/ExtractBookingField

# Delete — use "id" not "booking_id"
grpcurl -plaintext -H "Authorization: Bearer $TOKEN" \
  -d "{\"id\": \"$BOOKING_ID\"}" localhost:8090 toqui.v1.BookingService/DeleteBooking
```

### PersonaService

```bash
# List available personas
grpcurl -plaintext -H "Authorization: Bearer $TOKEN" \
  -d '{}' localhost:8090 toqui.v1.PersonaService/ListPersonas

# Resolve a persona for a trip
grpcurl -plaintext -H "Authorization: Bearer $TOKEN" \
  -d "{\"trip_id\": \"$TRIP_ID\", \"latitude\": 35.6762, \"longitude\": 139.6503, \"mode\": \"CHAT_MODE_PLANNING\", \"themes\": [\"food\", \"history\"]}" \
  localhost:8090 toqui.v1.PersonaService/ResolvePersona

# Get persona by ID (format: "LOCATION_theme" e.g. "JP_food")
grpcurl -plaintext -H "Authorization: Bearer $TOKEN" \
  -d '{"persona_id": "JP_food"}' localhost:8090 toqui.v1.PersonaService/GetPersona
```

### LocationService

```bash
# Update location — "location" is a nested LatLng message (NOT flat fields)
grpcurl -plaintext -H "Authorization: Bearer $TOKEN" \
  -d "{\"location\": {\"latitude\": 35.6762, \"longitude\": 139.6503}, \"trip_id\": \"$TRIP_ID\"}" \
  localhost:8090 toqui.v1.LocationService/UpdateLocation

# Get nearby — same nested format
grpcurl -plaintext -H "Authorization: Bearer $TOKEN" \
  -d '{"location": {"latitude": 35.6762, "longitude": 139.6503}, "category": "restaurant", "radius_meters": 1000}' \
  localhost:8090 toqui.v1.LocationService/GetNearby
```

### REST endpoints

```bash
# Usage limits
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8090/api/usage

# Referral
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8090/api/referral

# Share trip
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"trip_id\": \"$TRIP_ID\"}" \
  http://localhost:8090/api/trips/share

# View shared trip (public, no auth)
curl -s http://localhost:8090/shared/<share-token>

# Unshare
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"trip_id\": \"$TRIP_ID\"}" \
  http://localhost:8090/api/trips/unshare

# Destination guides (public)
curl -s http://localhost:8090/api/guides
```

---

## 4. API Testing with Bruno

Bruno is a GUI REST/gRPC client. The collection lives in `tests/bruno/`.

### Setup

1. Download [Bruno](https://www.usebruno.com/)
2. **Open Collection** → select `tests/bruno/`
3. Select the **local** environment (top-right dropdown)
4. Set `auth_token` to your JWT from testctl

### Folder structure

| Folder | RPCs covered |
|--------|-------------|
| `auth/` | GetCurrentUser, RefreshToken, DeleteAccount, ExportData |
| `trips/` | CreateTrip, ListTrips, GetTrip, GetItinerary, UpdateTrip, UpdateItinerary, DeleteTrip |
| `bookings/` | IngestBooking, ListBookings, GetBooking, LinkBookingToTrip, ExtractBookingField, DeleteBooking |
| `personas/` | ListPersonas, ResolvePersona, GetPersona, SetDefaultPersona |
| `location/` | UpdateLocation, GetNearby |
| `chat/` | SendMessage, GetChatHistory, ListChatSessions |
| `rest/` | GetUsage, GetReferral, RedeemReferral, PostFeedback, ShareTrip, UnshareTrip, ListGuides, GetGuide |

### Known field name quirks (caught in testing)

| RPC | Wrong | Correct |
|-----|-------|---------|
| GetTrip | `trip_id` | `id` |
| UpdateTrip | `trip_id` | `id` |
| DeleteTrip | `trip_id` | `id` |
| DeleteBooking | `booking_id` | `id` |
| GetBooking | `booking_id` | `id` |
| GetItinerary | ~~`id`~~ | `trip_id` ✓ |
| UpdateLocation | `latitude`, `longitude` (flat) | `location: {latitude, longitude}` (nested) |
| GetNearby | `latitude`, `longitude` (flat) | `location: {latitude, longitude}` (nested) |
| ResolvePersona | `location_code`, `destination` | `trip_id`, `latitude`, `longitude`, `mode`, `themes` |

---

## 5. Frontend QA Checklist

Walk through these flows after injecting the token:

### Auth & onboarding

- [ ] App loads without redirect to login
- [ ] User name appears in settings/profile
- [ ] Age gate does NOT appear (age_verified=true)
- [ ] No console errors on first load

### Trip creation

- [ ] Quick create: type destination, tap "Start Planning" → navigates to chat
- [ ] Advanced form: expand via "Advanced Options", fill title + dates → creates trip
- [ ] Date validation: end date before start date shows error
- [ ] New trip appears in trip list

### Chat (planning mode)

- [ ] Messages send and stream back correctly
- [ ] Itinerary updates appear as cards in chat
- [ ] Persona switch card appears when AI calls `suggest_expert`
- [ ] Recommendation cards link to affiliate partners
- [ ] Follow-up suggestion chips appear
- [ ] Loading/typing indicator shows during AI response

### Bookings

- [ ] Paste raw booking text → ingests and shows booking card
- [ ] Booking appears in trip's booking list
- [ ] Booking details are parseable (dates, confirmation number)

### Trip sharing

- [ ] Share button generates a link
- [ ] `/shared/<token>` is accessible without auth
- [ ] Unshare revokes access (shared link 404s)

### Settings

- [ ] Theme toggle (light/dark) persists
- [ ] Feedback modal submits without error
- [ ] Usage counter shows correct values

---

## 6. Known Issues & Gotchas

See [GitHub Issues](https://github.com/gallowaysoftware/toqui/issues) for the
full list. Issues found during QA that aren't yet filed:

### Expo/React Native web

- `form_input` from Chrome MCP doesn't trigger `onChangeText` — React Native's TextInput
  uses a synthetic event system that isn't compatible with DOM `.value` injection. Use
  `nativeInputValueSetter` + `dispatchEvent('input')` trick, or create trips via grpcurl
  and navigate directly: `window.location.replace('/trips/<id>/chat')`.

### Chat history

- `GetChatHistory` may return 0 messages when queried immediately after `SendMessage`.
  There's a Firestore write delay. Wait a moment and retry. See issue #178.

### Proto field naming

- `GetTripRequest`, `UpdateTripRequest`, `DeleteTripRequest` use `id` (not `trip_id`).
- `GetItineraryRequest`, `UpdateItineraryRequest` use `trip_id`.
- `GetBookingRequest`, `DeleteBookingRequest` use `id` (not `booking_id`).
- `ExtractBookingFieldRequest` uses `booking_id` (consistent with the field it refers to).

### Chat mode enum

Valid values for `mode` in `SendMessage` and `ResolvePersona`:
- `CHAT_MODE_PLANNING`
- `CHAT_MODE_COMPANION`
- `CHAT_MODE_SELECTION`
- `CHAT_MODE_UNSPECIFIED`

`CHAT_MODE_DEFAULT` does **not** exist.

---

## 7. Agentic Testing

For broader coverage, use the agentic test suite:

```bash
# Start infra (same as above)
make docker-up && make migrate-up && make run &
curl -s http://localhost:8090/healthz   # wait for ok

# Then from Claude Code, run the agentic test suite
# (see CLAUDE.md "Agentic Testing" section for full instructions)
```

The agentic suite runs 20 AI traveler personas in batches of 2, covering:
- Full trip lifecycle (create → plan → activate → complete)
- Booking ingestion (7 artifact types)
- Persona handoffs and expert composition
- Trip sharing lifecycle
- Edge cases (dietary restrictions, budget constraints, cultural sensitivity)

---

## 8. Reporting Bugs

File issues at https://github.com/gallowaysoftware/toqui/issues (monorepo —
frontend + backend live in the same tracker).

Labels to apply:
- `P0` — crash / data loss / auth failure
- `P1` — major feature broken, no workaround
- `P2` — minor issue, workaround exists
- `qa-session` — found during manual QA
