# Project Status

Last updated: 2026-03-05

## What's Done

### Auth

- Google OAuth login flow
- Auth token refresh (RefreshToken RPC, frontend auto-refresh before expiry, failed request retry)
- Rate limiting on AI calls (per-user)

### Booking

- Booking table with structured fields (confirmation code, provider, departure/arrival locations, guest count)
- Bookings linked to trips with CASCADE deletes

### Chat

- Firestore-backed chat messaging
- Chat streaming with markdown rendering, rich place cards, loading states
- ConnectRPC v2 clients wired up (frontend uses `createClient` from `_pb.ts`)
- Prompt caching for system prompts (Anthropic prompt caching)

### Personas

- Toqui orchestrator persona (hardcoded, always available)
- Persona composition system: 24 location profiles, 15 theme profiles
- Dynamic expert persona generation from location + theme combinations
- AI-based identity generation with template fallback
- Persona caching (same inputs = same persona)
- PersonaService RPCs (ListPersonas, GetPersona, SetDefaultPersona, ResolvePersona)
- Persona UI in frontend (avatar/name in chat, PersonaSwitch events, animated transitions)
- Theme tagger wired to trip creation and planning chat

### Data Lifecycle & Privacy

- CASCADE deletes on all FK relationships
- DeleteAccount RPC with full Postgres + Firestore purge
- DeleteTrip RPC with Firestore + Postgres purge
- ExportData RPC with export_requests table
- Location data policy: lat/lng ephemeral only, never persisted
- Trip archival: completed_at/archive_after columns, ArchiveCompletedTrips job, chat purge on archive
- Firestore TTL on chat messages for completed trips
- Deletion requests table (GDPR 30-day requirement)

### Frontend

- ConnectRPC generated clients replacing raw fetch
- next-intl setup (middleware, locale routing, message files, starting with en)
- Trip creation flow (modal/page with title, description, dates, destination country)
- Trip settings (edit title/dates/description, delete with confirmation)
- User settings (account page, delete account, data export)

### Infrastructure

- Docker compose full stack (backend + Postgres + Firestore emulator, healthchecks, auto-migrate)
- Proto sync script (`make proto-sync` generates TS and copies to frontend)
- Brand site (Astro + Tailwind, landing page, privacy policy, terms of service, cookie policy)
- Integration tests (devcontainer with Postgres + Firestore emulator, auth flow, trip CRUD, chat streaming)
- Terraform GCP infrastructure ([toqui-terraform](https://github.com/gallowaysoftware/toqui-terraform)):
  - Two GCP projects (toqui-staging, toqui-prod) under Toqui folder in thegalloways.ca org
  - Staging: GCE VM + Docker + Tailscale VPN, Cloud SQL db-f1-micro, Firestore
  - Prod: Cloud Run (public), Cloud SQL with HA + backups, Firestore multi-region
  - Shared modules: networking, cloudsql, firestore, secrets, artifact-registry
  - GCS remote state with versioning

---

## What's Remaining

### P0

All P0 items are complete.

### P1

- **AI trip simulation framework** -- `cmd/simulate/` binary for synthetic user testing (agent profiles, planning/booking/companion phases, structured reviews, parallel execution, JSON reports)
- **Booking ingestion** -- wire IngestBooking to AI parsing, store structured result
- **Email ingestion pipeline** -- receive forwarded emails, parse with AI
- **Itinerary from chat** -- AI generates structured itinerary items from planning conversation
- **Toqui handoff detection** -- analyze chat to detect when Toqui should suggest an expert
- **Map integration** -- MapLibre GL JS, itinerary on map, POI markers
- **Booking management UI** -- list bookings, link to itinerary items
- **PWA setup** -- installable, basic offline itinerary cache
- **Dark mode** -- Toqui color palette

### P2

- More location/theme profiles (ongoing)
- Companion mode location tracking (Geolocation API, auto-resolve local expert)
- Group trips (shared itineraries)
- Push notifications (trip reminders, flight status)
- CI/CD (GitHub Actions)
- Monitoring (Cloud Logging, error alerting, AI cost tracking)
- Model routing (Haiku for simple, Sonnet for complex)
- Native apps, WhatsApp integration, affiliate revenue

### Known Issues

- `buf/validate` TS types are not generated (protoc-gen-validate does not emit TS)

---

## Migration Checklist

All migrations live in `db/migrations/`. Run in order:

| Migration | File                                  | What It Does                                                                                                                                               |
| --------- | ------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 000001    | `20260305000001_initial`              | Creates core tables: `users`, `trips`, `itinerary_items`, `bookings`. Enables pgcrypto and PostGIS extensions. Sets up CASCADE deletes and indexes.        |
| 000002    | `20260305000002_themes`               | Creates `themes` and `trip_themes` tables (many-to-many). Adds `destination_country` column to trips. Seeds initial themes (food, history, distilleries).  |
| 000003    | `20260305000003_data_lifecycle`       | Adds trip archival columns (`completed_at`, `archive_after`, `archived_at`). Creates `deletion_requests` and `export_requests` tables for GDPR compliance. |
| 000004    | `20260305000004_booking_details`      | Adds `departure_location`, `arrival_location`, and `num_guests` columns to bookings.                                                                       |
| 000005    | `20260305000005_user_default_persona` | Adds `default_persona_id` column to users.                                                                                                                 |

After schema changes: run `sqlc generate` to regenerate Go query code.
After proto changes: run `buf generate` then `make proto-sync` to regenerate Go + TS bindings.

---

## Architecture Quick Reference

### Stack

| Layer      | Technology                                              |
| ---------- | ------------------------------------------------------- |
| Backend    | Go, ConnectRPC, sqlc, pgx/v5, Firestore (chat)          |
| Frontend   | Next.js, ConnectRPC, next-intl                          |
| Database   | PostgreSQL (structured data), Firestore (chat messages) |
| Protos     | Buf (local plugins only, no BSR)                        |
| Brand site | Astro + Tailwind (static, nginx:alpine)                 |

### Key Directories (backend)

```
cmd/            -- Service entry points
internal/       -- Application code (persona/, auth/, chat/, etc.)
proto/          -- Protobuf definitions
gen/            -- Generated Go code (buf generate)
gen-ts/         -- Generated TypeScript code (buf generate, synced to frontend)
db/migrations/  -- SQL migrations (golang-migrate format)
db/queries/     -- sqlc query files
third_party/    -- Vendored proto dependencies
```

### Key Commands

| Command             | What It Does                                                    |
| ------------------- | --------------------------------------------------------------- |
| `make proto-sync`   | Generates TS from protos and copies to frontend repo            |
| `buf generate`      | Generates Go + TS from proto definitions                        |
| `sqlc generate`     | Generates Go from SQL queries                                   |
| `docker compose up` | Starts full local stack (backend, Postgres, Firestore emulator) |
