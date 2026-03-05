# Toqui Backend

AI-powered travel companion platform. Go backend with ConnectRPC, PostgreSQL, Firestore, and Claude/OpenAI.

## Project Structure

This is a 3-repo project under `github.com/gallowaysoftware`:
- **toqui-backend** (this repo) — Go backend, gRPC API, AI orchestration
- **toqui** — Next.js TypeScript web frontend
- **toqui-site** — Astro static marketing site

## Architecture

```mermaid
graph TB
    FE[Frontend - Next.js] -->|ConnectRPC| BE[Backend - Go :8090]
    BE --> PG[(PostgreSQL + PostGIS)]
    BE --> FS[(Firestore)]
    BE --> AI[Claude API]

    subgraph Backend Services
        BE --> Auth[AuthService]
        BE --> Trip[TripService]
        BE --> Chat[ChatService]
        BE --> Book[BookingService]
        BE --> Loc[LocationService]
        BE --> Pers[PersonaService]
    end
```

### Key Packages

| Package | Purpose |
|---------|---------|
| `cmd/server` | Main API server entry point |
| `cmd/migrate` | Database migration runner |
| `internal/handlers/` | ConnectRPC service handlers (auth, trip, chat, booking, location, persona) |
| `internal/chat/` | Chat service — AI streaming, tool execution, persona resolution |
| `internal/persona/` | Persona composition — 24 locations x 15 themes = 360+ experts |
| `internal/ai/` | AI provider abstraction (Claude primary, OpenAI fallback) |
| `internal/chatstore/` | Firestore chat message persistence |
| `internal/lifecycle/` | GDPR deletion, archival, data export |
| `internal/auth/` | Google OAuth + JWT + auth interceptor |
| `internal/trip/` | Trip CRUD, status transitions, destination management |
| `internal/theme/` | Trip theme tagging (AI-driven) |
| `internal/aitest/` | AI integration test harness (build tag: `aitest`) |
| `internal/dbgen/` | Generated sqlc query code (gitignored) |
| `proto/toqui/v1/` | Protobuf service definitions |
| `gen/toqui/v1/` | Generated Go proto code (gitignored) |

### Services (proto/toqui/v1/)

- **AuthService** — Google OAuth, JWT refresh, account deletion/export
- **TripService** — Trip CRUD, itinerary management
- **ChatService** — Streaming chat with AI, history, sessions
- **BookingService** — Booking ingestion (AI parsing), CRUD
- **PersonaService** — List/resolve/set default persona
- **LocationService** — Ephemeral location updates, nearby places

## Conventions

- **Logging**: Use `log/slog` for all Go logging. Structured key-value pairs, not `log.Printf` or `fmt.Printf`.
- **Imports**: Alias proto types as `toquiv1`, connect stubs as `toquiv1connect`.
- **ConnectRPC routes**: `/toqui.v1.ServiceName/MethodName`
- **Firestore paths**: `users/{uid}/trips/{tripId}/chatSessions/{sessionId}/messages`
- **SQL**: Use `sqlc.arg(name)` named parameters (not positional `$N`) for COALESCE-heavy queries.

## Development

```bash
make proto          # Generate Go proto code + lint
make sqlc           # Generate Go from SQL queries
make build          # Build server binary
make run            # Run server
make test           # Run unit tests
make docker-up      # Start Postgres + Firestore emulator
make docker-down    # Tear down
```

TS proto bindings are generated in the frontend repo (`pnpm generate` in `../toqui`).

### Database

PostgreSQL 16 + PostGIS. Migrations in `db/migrations/`, queries in `db/queries/`.

```bash
make migrate-up     # Apply migrations
make migrate-down   # Rollback one
make migrate-create # Create new migration files
```

### Environment Variables

Required: `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `ANTHROPIC_API_KEY` (or `OPENAI_API_KEY`)

See `.env.example` for full list. Docker compose sets defaults for local dev.

## Trip Mode System

```mermaid
stateDiagram-v2
    [*] --> Selection: App starts / no trip selected
    Selection --> Planning: Create or select trip
    Planning --> Traveling: Start trip (status=active)
    Traveling --> Planning: Pause trip
    Planning --> Selection: Close trip
    Traveling --> Selection: Complete trip
```

- **Selection mode** — No trip selected. Chat-first interface: user describes what they want, AI creates or selects trips via tools (`create_trip`, `select_trip`). The AI matches vague references ("my Greece trip") to existing trips.
- **Planning mode** — Trip selected, `status=planning`. Talk to personas, build itinerary, add bookings. AI has full trip context (title, description, destination, themes) injected as system context.
- **Companion mode** — Trip started, `status=active`. Location-aware responses. The AI knows you're traveling (not just planning) which changes how personas respond.

## Persona System

```mermaid
graph LR
    T[Toqui - Global Orchestrator] --> C{Composition}
    L[Location Profile] --> C
    TH[Theme Profile] --> C
    C --> E[Composed Expert]

    subgraph "24 Locations"
        L1[Japan]
        L2[Italy]
        L3[Scotland]
        L4[...]
    end

    subgraph "15 Themes"
        TH1[Food]
        TH2[History]
        TH3[Distilleries]
        TH4[...]
    end
```

Toqui (the global orchestrator) hands off to composed experts. Each expert is dynamically built from a location profile + theme profile(s). Persona identities (names, descriptions, greetings) are AI-generated and cached for consistency.

## AI Integration Tests

End-to-end test harness that exercises the full trip lifecycle through the AI. Uses real LLM calls.

```bash
docker compose up -d                    # Start Postgres + Firestore emulator
make ai-test                            # Run regression scenarios
make ai-test-generative                 # Run regression + LLM-generated scenarios
go test -tags=aitest -v -timeout=30m \
  ./internal/aitest/... -run TestAIScenarios/alice  # Run specific scenario
```

### Regression Scenarios

| Scenario | What it tests |
|----------|---------------|
| `alice-backpacker-lifecycle` | Full lifecycle: selection → planning → companion → complete |
| `bob-family-planner` | Planning context injection — AI must know destination without asking |
| `carol-returning-user` | Multi-trip: select_trip matching, trip switching, new trip creation |
| `update-regression` | UpdateTrip COALESCE — status change must not wipe title/description |

### Design

- **Structural assertions are hard failures** (tool called, response contains, trip status) — these fail the test.
- **LLM evaluations are informational** (response quality scored 1-5 by a judge LLM) — these log warnings but don't fail.
- Each scenario gets its own isolated test user.
- Reports written to `testdata/aitest-reports/` as JSON.

## Auth Flow

**TODO: Replace token-in-URL redirect with secure cookies + cookie auth middleware + GetTokens RPC.**

Current flow: Google OAuth -> backend callback -> redirect to frontend with tokens in URL params.
Target flow: Google OAuth -> backend callback -> set secure HttpOnly cookie -> frontend calls GetTokens RPC with cookie.

## Data Lifecycle

- **Location data**: Ephemeral (request-scoped only, never stored)
- **Trip archival**: 90 days after completion, chat messages purged from Firestore
- **User deletion**: GDPR Article 17 — CASCADE deletes in Postgres + Firestore purge, within 30 days
- **Data export**: GDPR Article 20 — async job generates downloadable archive
