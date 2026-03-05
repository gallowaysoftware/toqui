# Toqui Implementation Tasks

Prioritized backlog. Update status as work progresses.

## P0 — Must have for MVP launch

### Data Privacy & Compliance
- [x] CASCADE deletes on all FK relationships (users->trips->itinerary/bookings/themes)
- [x] **Backend: User deletion endpoint** — DeleteAccount RPC, lifecycle service purges Postgres + Firestore
- [x] **Backend: Trip deletion endpoint** — DeleteTrip uses lifecycle service to purge Firestore + Postgres
- [x] **Backend: Data export endpoint** — ExportData RPC + export_requests table (async worker TODO)
- [x] **Backend: Location data policy** — lat/lng injected into system prompt only, never stored in messages or Firestore
- [x] **Backend: Trip archival** — completed_at/archive_after columns, ArchiveCompletedTrips job, chat purge on archive
- [x] **Backend: Firestore TTL** — set TTL on chat messages for completed trips
- [x] **Privacy policy page** — on brand site, covering GDPR + CCPA, data retention, location data handling, AI data usage, right to deletion

### Core Backend
- [x] **Docker compose full stack** — backend + Postgres + Firestore emulator, healthchecks, auto-migrate
- [x] **Wire up theme tagger** — call after trip creation and during planning chat when no themes exist
- [x] **Persona handler** — implement PersonaService RPCs (ListPersonas, GetPersona, SetDefaultPersona, ResolvePersona)
- [x] **Proto sync script** — `make proto-sync` generates TS bindings and copies to frontend
- [x] **Auth token refresh** — implement RefreshToken RPC, frontend auto-refresh
- [x] **Integration tests** — devcontainer with Postgres + Firestore emulator, test auth flow, trip CRUD, chat streaming

### Core Frontend
- [x] **Wire generated ConnectRPC clients** — replace raw fetch in useChat.ts and useTrips.ts with generated clients
- [x] **Persona UI** — show active persona avatar/name in chat, handle PersonaSwitch events, animate transitions
- [x] **next-intl setup** — middleware, locale routing, message files (start with en)
- [x] **Auth token refresh** — auto-refresh before expiry, retry failed requests
- [x] **Trip creation flow** — modal/page with title, description, dates, destination country
- [x] **Chat streaming improvements** — markdown rendering, rich cards for places, loading states

### Brand Site (toqui-site repo, Astro + Tailwind)
- [x] **Scaffold repo** — Astro static site, Tailwind, Dockerfile (nginx:alpine)
- [x] **Landing page** — hero, features, expert guides showcase, CTAs
- [x] **Privacy policy** — GDPR/CCPA compliant, AI usage, location data, retention, deletion rights
- [x] **Terms of service** — AI disclaimers, liability, acceptable use
- [x] **Cookie policy** — covered in privacy policy (minimal cookies section)
- [x] **Update brand name** — Toqui (toqui.com, toqui.ca, toqui.travel — all available). Cheeky Canadian toque reference.

## P1 — Important for good UX

### Testing & Quality
- [x] **AI integration test harness** — `internal/aitest/` with `go test -tags=aitest`. 4 regression scenarios (alice-backpacker, bob-family, carol-returning, update-regression) + generative LLM scenarios. Structural assertions + LLM-as-judge evaluation. `make ai-test`.
- [x] **Chat-first selection mode** — AI creates/selects trips via tools (`create_trip`, `select_trip`). Vague reference matching ("my Greece trip"). COALESCE fix for partial UpdateTrip.

### Backend
- [ ] **Booking ingestion** — wire IngestBooking to AI parsing, store structured result
- [ ] **Email ingestion pipeline** — receive forwarded emails, parse with AI
- [ ] **Itinerary from chat** — AI generates structured itinerary items from planning conversation
- [ ] **Toqui handoff detection** — analyze chat to detect when Toqui should suggest an expert (could be rule-based initially, AI-based later)
- [x] **Rate limiting** — per-user rate limits on AI calls
- [x] **Prompt caching** — Anthropic prompt caching for system prompts (biggest cost lever)

### Frontend
- [ ] **Map integration** — MapLibre GL JS, show itinerary on map, POI markers
- [ ] **Booking management** — list bookings, link to itinerary items
- [x] **Trip settings** — edit title/dates/description, delete trip (with confirmation)
- [x] **User settings** — account page, delete account (with confirmation), data export
- [ ] **PWA setup** — installable, basic offline itinerary cache
- [ ] **Dark mode** — using Toqui color palette

## P2 — Nice to have / Phase 2

### Features
- [ ] **More location profiles** — Spain, Greece, Thailand, Mexico, etc.
- [ ] **More theme profiles** — wine, art, architecture, nature, adventure, etc.
- [ ] **Companion mode location tracking** — Geolocation API, auto-resolve local expert on arrival
- [ ] **Group trips** — shared itineraries, multiple users
- [ ] **Push notifications** — trip reminders, flight status
- [ ] **Offline support** — cached itinerary data for companion mode

### Infrastructure
- [ ] **Terraform GCP** — Cloud Run, Cloud SQL, Firestore, Cloud Storage, IAM
- [ ] **CI/CD** — GitHub Actions, build + test + deploy
- [ ] **Monitoring** — Cloud Logging, error alerting, AI cost tracking dashboard
- [ ] **Model routing** — Haiku for simple queries, Sonnet for complex (biggest cost optimization)

### Platform
- [ ] **Native apps** — React Native or Flutter
- [ ] **WhatsApp integration** — Business API for text-only chat
- [ ] **Affiliate revenue** — booking links with commission tracking

## Data Lifecycle

### Location Data (SENSITIVE — never persist)
- Request-scoped only: user sends lat/lng with message in companion mode
- Used to resolve nearby places and local expert persona
- NOT stored in chat messages, user profiles, or any persistent storage
- Itinerary items can have location (user explicitly set), but user's real-time location is ephemeral

### Trip Lifecycle
1. **Planning** — full data, all chat history retained
2. **Active** — full data, companion mode chats retained during trip
3. **Completed** — enters archive mode
4. **Archived** — after configurable retention (default 90 days post-completion):
   - Chat messages purged from Firestore
   - Itinerary and bookings retained (user's reference)
   - Persona cache for this trip cleared
5. **Deleted** — all data purged (Postgres CASCADE + Firestore deletion)

### User Deletion (GDPR Article 17)
1. All trips deleted (CASCADE handles Postgres)
2. All Firestore chat sessions/messages deleted
3. All cached persona identities cleared
4. User row deleted
5. Confirmation email sent
6. Completed within 30 days of request (GDPR requirement)
