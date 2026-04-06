# Toqui Backend

AI-powered travel companion platform. Go backend with ConnectRPC, PostgreSQL, Firestore, and Claude/Gemini.

## Core Principles

### User Privacy — Non-Negotiable

Toqui exists to help travelers, not to exploit them. These rules are absolute and override any business or feature consideration:

**Data Collection:**
- Collect only what's needed to deliver the feature. If in doubt, don't collect it.
- Travel data is inherently sensitive — destinations can reveal religion, health conditions, sexuality, and political activity. Treat ALL travel data as potentially sensitive under GDPR Article 9.
- Never log, track, or store destination names, chat content, specific travel dates, hotel/flight names, or booking details in analytics. Track counts and categories, never content.
- Pseudonymize user IDs in any analytics or logging pipeline.

**Compliance:**
- Comply with EU GDPR as the baseline for ALL users, regardless of their location. Do not maintain separate privacy standards by region.
- Comply with Canadian PIPEDA. As a Canadian company, PIPEDA applies to all commercial activities.
- Apple App Tracking Transparency: use only first-party analytics. Never track users across apps or websites.
- Cookie-less analytics only. No tracking cookies, no fingerprinting, no IDFA/GAID collection.

**Monetization Ethics:**
- Affiliate revenue is acceptable and must be transparently disclosed (see toqui.travel/affiliate-disclosure).
- Never bias AI recommendations for revenue. The AI recommends what's best for the traveler, not what pays us the most.
- Never sell, share, or broker user data to third parties. Period.
- Never serve display advertising that tracks users.
- Sponsored/promoted placements, if ever introduced, must be clearly labeled and must not degrade recommendation quality.

**Analytics:**
- Session replay must mask all text inputs, chat content, and itinerary details.
- Analytics events track behavior patterns (user created a trip), never content (user planned a trip to Mecca).
- Self-hosted or EU-hosted analytics only. Google Analytics is explicitly prohibited (ruled non-compliant by multiple EU DPAs).
- Users must be informed about analytics in the privacy policy with a clear opt-out mechanism for EU users.

**Data Lifecycle:**
- GDPR Article 17 (right to deletion) and Article 20 (data portability) are implemented and must remain functional.
- Trip data is archived after 90 days of completion and eventually purged.
- Account deletion must be complete — no shadow profiles, no retained analytics, no "soft delete" that keeps data.

These principles are not aspirational. They are engineering requirements. Code that violates them must not be merged.

## Project Structure

This is a 5-repo project under `github.com/gallowaysoftware`:

- **toqui-backend** (this repo) — Go backend, ConnectRPC API, AI orchestration
- **toqui** — Expo React Native app (web + iOS + Android)
- **toqui-terraform** — Terraform GCP + Cloudflare infrastructure
- **toqui-site** — Astro static marketing site (Cloudflare Pages)
- **toqui-admin** — Vite React admin panel (Cloudflare Pages)

## Architecture

```mermaid
graph TB
    FE[Frontend - Expo React Native] -->|ConnectRPC| BE[Backend - Go :8090]
    BE --> PG[(PostgreSQL + PostGIS)]
    BE --> FS[(Firestore)]
    BE --> AI[Gemini / Claude API]

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

| Package                 | Purpose                                                                                   |
| ----------------------- | ----------------------------------------------------------------------------------------- |
| `cmd/server`            | Main API server entry point                                                               |
| `cmd/migrate`           | Database migration runner                                                                 |
| `internal/handlers/`    | ConnectRPC service handlers (auth, trip, chat, booking, location, persona)                |
| `internal/chat/`        | Chat service — AI streaming, tool execution, persona resolution                           |
| `internal/persona/`     | Persona composition — 40 locations × 20 themes = 800 expert combos                        |
| `internal/ai/`          | AI provider abstraction (Gemini primary, Claude fallback)                                 |
| `internal/ai/tools/`    | LLM-callable tool registry (WebSearch, Places)                                            |
| `internal/chatstore/`   | Firestore chat message persistence                                                        |
| `internal/lifecycle/`   | GDPR deletion, archival, data export                                                      |
| `internal/auth/`        | Google OAuth + JWT + auth interceptor + refresh token rotation (JTI/family tracking)      |
| `internal/trip/`        | Trip CRUD, status transitions, destination management                                     |
| `internal/booking/`     | Booking ingestion + AI parsing (email, paste, manual)                                     |
| `internal/location/`    | Location service — ephemeral location cache (30 min TTL), nearby places (Google Places)   |
| `internal/theme/`       | Trip theme tagging (AI-driven classification)                                             |
| `internal/affiliate/`   | Affiliate link builder — generates partner URLs for Skyscanner, Booking.com, GetYourGuide |
| `internal/config/`      | Three-layer config: env file → os.Getenv → GCP Secret Manager                             |
| `internal/db/`          | PostgreSQL connection pool + transaction helpers                                          |
| `internal/validate/`    | ConnectRPC interceptor for buf.validate constraints                                       |
| `internal/csrf/`        | CSRF protection middleware (Origin/Referer validation for state-changing requests)        |
| `internal/audit/`       | Structured audit logging for security-relevant events (via slog → Cloud Logging)         |
| `internal/middleware/`   | HTTP middleware (cookie-to-header auth bridge for web browser sessions)                   |
| `internal/ratelimit/`   | Per-user rate limiting interceptor + per-IP auth lockout (AuthLimiter)                    |
| `internal/email/`       | Transactional email via Resend API (waitlist verification, invite emails)                   |
| `internal/payment/`     | Stripe payment processing (Trip Pro checkout sessions, one-time purchases)                  |
| `internal/subscription/` | Stripe subscription management (Explorer/Voyager tiers, checkout, webhooks, portal)        |
| `internal/requestid/`   | HTTP middleware — generates unique request IDs, sets `X-Request-ID` response header         |
| `internal/tier/`        | User subscription tier logic (Free / Pro / Explorer / Voyager) and feature gating          |
| `internal/usage/`       | Daily usage tracking + message limit enforcement per user                                 |
| `tests/agentic/`        | Agentic test personas, booking artifacts, and orchestration                                |
| `cmd/testctl/`          | Test user/token management CLI for agentic testing                                        |
| `internal/integration/` | Integration test suite (build tag: `integration`)                                         |
| `internal/dbgen/`       | Generated sqlc query code (regenerate: `make sqlc`)                                       |
| `proto/toqui/v1/`       | Protobuf service definitions (7 files, 6 services, 28 RPCs)                               |
| `gen/toqui/v1/`         | Generated Go proto code (regenerate: `make proto`)                                        |

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

## Request Pipeline

Every ConnectRPC request passes through the interceptor chain:

```
Request → validate.Interceptor → auth.Interceptor → age.Interceptor → ratelimit.Interceptor → Handler
```

- **validate**: Enforces `buf.validate` constraints on request protos (string lengths, UUID format, lat/lng bounds). Returns `InvalidArgument` on failure.
- **auth**: Extracts JWT from `Authorization` header, validates, injects user ID into context. Returns `Unauthenticated` on failure.
- **age**: Enforces age verification gate — users who haven't completed `POST /auth/verify-age` cannot access gated RPCs. Returns `PermissionDenied` if age not verified.
- **ratelimit**: Per-user token bucket. Separate limits for AI RPCs (SendMessage) vs general RPCs. Returns `ResourceExhausted` when exceeded.

## Development

```bash
make run              # Run server (local, default)
make run-staging      # Run locally against staging infrastructure
make run-prod         # Run locally against prod infrastructure
make build            # Build server binary
make test             # Run unit tests
make lint             # Run golangci-lint
make proto            # Generate Go proto code + lint
make sqlc             # Generate Go from SQL queries
make docker-up        # Start Postgres + Firestore emulator
make docker-down      # Tear down
```

TS proto bindings are generated in the frontend repo (`pnpm generate` in `../toqui`).

### gRPC Reflection

gRPC reflection is enabled on the server. You can use `grpcurl` to explore and test RPCs:

```bash
# List all services
grpcurl -plaintext localhost:8090 list

# Describe a service
grpcurl -plaintext localhost:8090 describe toqui.v1.TripService

# Call an RPC (with auth)
grpcurl -plaintext -H "Authorization: Bearer <token>" \
  -d '{"trip_id":"..."}' localhost:8090 toqui.v1.TripService/GetTrip
```

### Bruno Test Collections

API test collections live in `tests/bruno/`. These are Bruno HTTP client collections for manual and semi-automated API testing:

- Import the collection into [Bruno](https://www.usebruno.com/)
- Collections cover auth flows, trip CRUD, chat, bookings, and admin endpoints
- Environment variables are configured per environment (local, staging, prod)

### CI/CD

GitHub Actions on push to `main` and all PRs (self-hosted runners on Unraid, 3 replicas):

- **toqui-backend**: lint, test (with coverage), build run in parallel → **deploy to prod** (main only, Cloud Run)
- **toqui**: lint+typecheck, test, build run in parallel → **deploy to prod** (main only, Cloud Run)
- **toqui-site**: build (Cloudflare Pages auto-deploys from main)
- **toqui-admin**: build (Cloudflare Pages auto-deploys from main)

**Prod auto-deploy**: Push to `main` triggers `deploy-prod` job: Docker build → push to Artifact Registry → run migrations via Cloud Run Jobs → deploy to Cloud Run. Uses Workload Identity Federation (keyless GCP auth). Migrations run BEFORE deploy to avoid schema mismatch.

**Staging**: Kept running (~$32/mo) for continuous testing. Can be torn down via `terraform destroy` in `environments/staging/` if needed. Shared infra (WIF, Artifact Registry) lives in `toqui-infra` GCP project, managed by `environments/infra/` in toqui-terraform.

### Task Tracking

All task tracking is in GitHub Issues: [toqui-backend issues](https://github.com/gallowaysoftware/toqui-backend/issues), [toqui issues](https://github.com/gallowaysoftware/toqui/issues). Labels: `P0`, `P1`, `P2`, `backend`, `frontend`, `infra`, `staging-launch`, `security`, `code-quality`, `design`, `compliance`.

### Database

PostgreSQL 16 + PostGIS. Migrations in `db/migrations/`, queries in `db/queries/`.

```bash
make migrate-up     # Apply migrations
make migrate-down   # Rollback one
make migrate-create # Create new migration files
```

### Environment Configuration

Config loads in three layers via `internal/config/`:

1. **Env file**: `env/.env.{TARGET_ENV}` parsed, sets missing env vars (no overwrite)
2. **os.Getenv with defaults**: Same as before, sane local defaults
3. **Secret Manager resolution**: `gcsm://` prefixed values replaced by GCP Secret Manager fetch

```bash
make run                                            # TARGET_ENV=local (default)
TARGET_ENV=staging make run                         # Uses staging infra + secrets
FIRESTORE_EMULATOR_HOST=localhost:8080 TARGET_ENV=staging make run  # Hybrid: staging DB, local Firestore
```

Env files: `env/.env.local`, `env/.env.staging`, `env/.env.prod`. All environments use `gcsm://secret-name` references resolved at startup via GCP Secret Manager (requires `gcloud auth application-default login`).

Required: `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `ANTHROPIC_API_KEY` (or `VERTEX_AI_PROJECT_ID` for Gemini fallback). See `env/.env.local` for the full local dev config.

**Additional environment variables** (see `internal/config/config.go` for full list):

| Env Var | Default | Description |
|---------|---------|-------------|
| `STRIPE_SECRET_KEY` | (none) | Stripe API secret key |
| `STRIPE_WEBHOOK_SECRET` | (none) | Stripe webhook signing secret |
| `STRIPE_TRIP_PRO_PRODUCT_ID` | (none) | Stripe product ID for Trip Pro one-time purchase |
| `STRIPE_EXPLORER_MONTHLY_PRODUCT` | (none) | Stripe product ID for Explorer monthly subscription |
| `STRIPE_EXPLORER_ANNUAL_PRODUCT` | (none) | Stripe product ID for Explorer annual subscription |
| `STRIPE_VOYAGER_MONTHLY_PRODUCT` | (none) | Stripe product ID for Voyager monthly subscription |
| `STRIPE_VOYAGER_ANNUAL_PRODUCT` | (none) | Stripe product ID for Voyager annual subscription |
| `TRIP_PRO_PRICE_CENTS` | `1900` | Trip Pro price in cents ($19.00) |
| `RESEND_API_KEY` | (none) | Resend transactional email API key |
| `EMAIL_FROM` | `Toqui <hello@toqui.travel>` | From address for outbound emails |
| `ADMIN_EMAILS` | (none) | Comma-separated list of admin email addresses |
| `ALLOWED_EMAILS` | (none) | Comma-separated allowlist bypassing capacity cap entirely |
| `CORS_ALLOWED_ORIGINS` | (falls back to FRONTEND_URL) | Comma-separated CORS allowed origins |
| `FIRESTORE_DATABASE_ID` | (none) | Firestore database ID (uses default if unset) |
| `SENDGRID_WEBHOOK_KEY` | (none) | ECDSA public key for SendGrid webhook verification |
| `DISCOVERCARS_AFFILIATE_ID` | (none) | DiscoverCars affiliate partner ID |
| `SAFETYWING_REFERENCE_ID` | (none) | SafetyWing affiliate reference ID |
| `POSTHOG_API_KEY` | (none) | PostHog project API key (EU instance, server-side events) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | (none) | OpenTelemetry collector endpoint |
| `OTEL_EXPORTER_OTLP_HEADERS` | (none) | OpenTelemetry exporter auth headers |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | (none) | OpenTelemetry protocol (grpc/http) |
| `OTEL_SERVICE_NAME` | (none) | OpenTelemetry service name |

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

    subgraph "40 Locations"
        L1[Japan]
        L2[Italy]
        L3[France]
        L4[...]
    end

    subgraph "20 Themes"
        TH1[Food]
        TH2[History]
        TH3[Distilleries]
        TH4[...]
    end
```

Toqui (the global orchestrator) hands off to composed experts. Each expert is dynamically built from a location profile + theme profile(s). Persona identities (names, descriptions, greetings) are AI-generated and cached for consistency.

**40 locations**: IT, JP, FR, GB, US, ES, DE, PT, GR, TH, MX, AU, BR, IN, KR, VN, MA, PE, NZ, TR, HR, ZA, CO, EG, ID, PH, CN, CZ, AT, CH, IE, SE, AR, CL, JO, TZ, IS, SG, HK, KH (4 core in `profiles.go`, 36 extended in `profiles_extended.go`).

**20 themes**: food, history, distilleries, adventure, wellness, wine, architecture, nightlife, shopping, family, photography, nature, romance, budget, luxury, art, music, craft-beer, diving, hiking (3 core, 17 extended).

## Chat Tool System

The AI in chat mode has access to tools injected by the handler layer. Tools are mode-specific and follow a callback pattern for emitting stream events to the frontend.

### Available Chat Tools

| Tool                     | Modes     | What it does                                                                                                                                           | Stream Event           |
| ------------------------ | --------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ | ---------------------- |
| `create_trip`            | selection | AI creates a new trip when user describes travel plans                                                                                                 | `TripCreated`          |
| `select_trip`            | selection | AI matches vague references to existing trips                                                                                                          | `TripSelected`         |
| `create_itinerary_items` | planning  | AI adds structured day-by-day itinerary items                                                                                                          | `ItineraryUpdate`      |
| `suggest_expert`         | all modes | Toqui hands off to a composed expert persona. **Free-tier gate**: limited to 3 expert handoffs per session, then returns an upgrade prompt directing the user to Trip Pro. | `PersonaSwitch`        |
| `recommend_booking`      | all modes | Generate affiliate-linked booking recommendations (flights, hotels, activities). AI sees result via tool loop and includes FTC disclosure in response. | — (inline in response) |
| `nearby_places`          | companion | Find nearby places using user's cached location (location-aware)                                                                                       | —                      |
| `web_search`             | all modes | Search the web for current info (global tool registry)                                                                                                 | —                      |
| `place_lookup`           | all modes | Google Places API lookup (global tool registry)                                                                                                        | —                      |

### Adding a New Chat Tool

Follow the pattern in `internal/handlers/tool_create_itinerary.go`:

1. **Create** `internal/handlers/tool_<name>.go` implementing `tools.Tool` interface:
   - `Definition() ai.ToolDefinition` — name, description, JSON Schema parameters
   - `Execute(ctx, args) (json.RawMessage, error)` — business logic + callback
2. **Wire** the tool in `internal/handlers/chat.go` `SendMessage()`:
   - Create a mutex-protected callback to collect results
   - Instantiate the tool with service dependencies + callback
   - Append to `params.ExtraTools`
3. **Emit** the stream event in the `tool_result` handler block in `chat.go`
4. **Write tests**:
   - Unit tests in `internal/handlers/tool_<name>_test.go` (arg parsing, edge cases)
   - Integration test in `internal/integration/` (DB operations with real Postgres)
   - Add an agentic test persona in `tests/agentic/personas/` if the tool introduces new behavior worth testing
5. **Update** system prompt in the relevant mode (e.g., `buildTripContext()` for planning)
6. **Update** this CLAUDE.md doc

### Tool Injection Pattern

```
ChatHandler.SendMessage()
  ├── Create mutex + callback slices
  ├── Instantiate tools with service deps + callbacks
  ├── params.ExtraTools = [tool1, tool2, ...]
  ├── chatSvc.SendMessage(params) → eventCh
  └── for event := range eventCh:
        case "tool_result":
          mu.Lock()
          if event.ToolName == "my_tool" && len(collected) > 0:
            stream.Send(MyProtoEvent{...})
          mu.Unlock()
```

### Tool Call → Result → Continue Loop

The chat service implements an agentic tool loop (`processEventsWithToolLoop` in `internal/chat/service.go`). When the AI makes a tool call:

1. Tool is executed immediately and `tool_call`/`tool_result` events are emitted to the frontend
2. The AI's stop reason is checked — if `"tool_use"`, the tool results are sent back to the AI
3. The AI continues generating with access to the tool results (e.g., including FTC disclosure text from `recommend_booking`)
4. This loops up to `maxToolLoopIterations` (5) until the AI produces a final response (`"end_turn"`)

This is critical for tools like `recommend_booking` where the AI must see the tool result to include disclosure text in its response. Side-effect tools (like `create_trip`, `create_itinerary_items`) also benefit — the AI can confirm what was created.

Both providers parse streaming events to extract stop reasons and serialize tool call/result content blocks for continuation. The Claude provider uses `message_delta` events; the Gemini provider uses `finishReason` in `candidates[]`.

## Pre-Commit Requirements

### Never Push Directly to Main — Use PRs

**MANDATORY**: All changes go through pull requests. Never push commits directly to `main`. This protects CI, enables review, and prevents broken deploys.

**Workflow:**
1. **Create a feature branch**: `git checkout -b feat/description` (or `fix/`, `chore/`, `docs/`)
2. **Run all checks locally before pushing**:
   ```bash
   go build ./... && go vet ./... && golangci-lint run ./... && go test ./...
   gofmt -w <edited files>
   ```
3. **Push the branch and open a PR**:
   ```bash
   git push -u origin feat/description
   gh pr create --title "feat: description" --body "## Summary\n..."
   ```
4. **Wait for CI to pass on the PR** — lint, tests, and build must all be green
5. **Run adversarial review** on the PR branch (spawn a review agent against the diff)
6. **Merge via squash**: `gh pr merge --squash`
7. **After merge, verify CI passes on `main`** — if it breaks, fix immediately with another PR
8. **Deploy to prod** (when ready): `gh workflow run CI --repo gallowaysoftware/toqui-backend --ref main`

### Keep CI Green — This Is Critical

**MANDATORY**: CI must stay green at all times on `main`. If a merge breaks CI, fix it immediately with a new PR before doing anything else.

- Push to `main` triggers `deploy-staging` (staging is kept running; infrastructure changes auto-apply via CI in toqui-terraform).
- `workflow_dispatch` triggers `deploy-prod`.
- **AI/prompt changes**: Run the agentic test suite or use `grpcurl` to verify AI behavior before merging any changes to system prompts, tool definitions, or persona profiles. See "Agentic Testing" section below.

### Documentation Updates

**MANDATORY**: Before opening a PR, update all relevant documentation:

1. **CLAUDE.md** — Update this file and any other repo CLAUDE.md files affected by the changes (architecture, deployment, security patterns, new packages)
2. **Cross-repo consistency** — If changes affect shared documentation topics (deployment, CI/CD, staging/prod status, security), update CLAUDE.md in ALL 5 repos

### Adversarial Review

**MANDATORY**: Before merging any PR, spawn a parallel adversarial review agent to audit all changes. This catches bugs, security issues, and logic errors before they reach `main`.

### How It Works

1. After all implementation and tests are passing on the PR branch, spawn a `general-purpose` Task agent with a prompt like:

   > You are an adversarial code reviewer. Your job is to find bugs, security issues, logic errors, and missing edge cases. Review all changes in these files: [list files]. For each issue found, classify as BLOCKING (must fix before commit) or WARNING (note but can ship). Be thorough and skeptical.

2. The agent reviews all changed files and returns findings classified as:
   - **BLOCKING** — Must fix before merge (bugs, security holes, logic errors, missing validation)
   - **WARNING** — Worth noting but acceptable to ship (style, minor improvements, future work)

3. Fix all BLOCKING issues, push to the PR branch, and re-run the adversarial review.

4. Only merge the PR after the adversarial review returns zero BLOCKING issues.

### What to Review

- All new files and modified files in the changeset
- Test coverage — are edge cases tested?
- Security — input validation, auth checks, injection risks
- Logic — off-by-one errors, race conditions, nil pointer dereferences
- API contracts — do request/response types match proto definitions?
- Error handling — are errors wrapped with context? Are they logged?

## Feature Implementation Checklist

Every new feature must include all of the following. Do not merge the PR without completing each item:

1. **Implementation** — The feature code itself
2. **Unit tests** — In the same package (`*_test.go`), test arg parsing, edge cases, error handling
3. **Integration tests** — In `internal/integration/` (build tag `integration`), test DB operations with real Postgres via docker-compose
4. **Agentic test persona** — If the feature introduces new user-facing behavior, add a persona prompt in `tests/agentic/personas/` or extend an existing one
5. **Adversarial review** — Run the adversarial review agent on the PR (see above)
6. **Documentation** — Update CLAUDE.md with the feature (tool table, any new patterns)
7. **Open PR + merge** — All of the above in one PR, squash-merged to `main`

### Testing Approach

- **Unit tests**: No DB required. Test JSON parsing, validation, error paths. Use `persona.NewComposer(nil)` for template-based persona tests.
- **Integration tests**: Real Postgres via `docker compose up -d`. Build tag `integration`. Use `TestEnv.CleanDB()` for isolation.
- **Agentic tests**: Claude agents test the running backend via grpcurl. See "Agentic Testing" section below.

## Agentic Testing

Black-box testing where Claude agents adopt traveler personas and interact with a running backend via grpcurl/curl. Each agent tests the full trip lifecycle (selection → planning → bookings → companion → sharing → completion) and evaluates both correctness and real-world usefulness.

### Architecture

```
Orchestrator (Claude Code session)
  ├── Start infra: make docker-up, migrate-up, run
  ├── For each batch of 2 personas:
  │     ├── cmd/testctl create-user × 2 → {user_id, token}
  │     ├── Launch 2 agents in parallel (background)
  │     ├── Wait for both to complete
  │     └── Collect reports
  ├── Repeat for all 10 batches (20 personas total)
  └── Synthesize all reports → gh issue create
```

**IMPORTANT**: Launch only **2 agents at a time** to avoid hitting the Anthropic API rate limit (each agent triggers multiple AI chat calls against the shared API key). Wait for both to complete before launching the next batch.

### Running Agentic Tests

```bash
# 1. Start infrastructure
make docker-up          # Postgres + Firestore emulator
make migrate-up         # Apply migrations
make run &              # Start backend on :8090
# Wait for: curl -s http://localhost:8090/healthz → {"status":"ok"}

# 2. In Claude Code, run the agentic test suite
# The orchestrator creates users and launches agents in batches of 2.
# Each agent uses the agentic-test skill with a persona prompt + JWT token.
# Wait for each batch to complete before launching the next.
# After all 20 personas complete, synthesize reports and create GitHub issues.
```

**Rate limit guidance**: The Anthropic API has per-org rate limits. Running more than 2 agents simultaneously causes 429 errors that degrade test quality. The orchestrator should launch agents in batches of 2, waiting for each batch to finish before starting the next. Persona 07 (update regression) does not use AI and can be included in any batch as a freebie.

### Test User Management (`cmd/testctl`)

```bash
# Create a test user with JWT token
go run ./cmd/testctl create-user --name "Alice" --email "alice@toqui-test.local"
# → {"user_id": "uuid", "token": "eyJ..."}

# Clean up after testing
go run ./cmd/testctl cleanup-user --user-id "uuid"
```

### Persona Catalog (20 personas: 8 regression + 12 edge cases)

**Regression suite (R-*)** — Core features, proven stable, catch regressions:

| # | Persona | Destination | Key Test Vectors |
|---|---------|-------------|------------------|
| R-02 | Family w/ kids | Costa Rica 10d | Context injection, safety, accessibility, companion mode |
| R-03 | Returning user | Multi-trip | select_trip matching, trip switching, multi-trip management |
| R-05 | Craft beer + hiker | CZ + Iceland | Extended profiles, niche themes, 2 trips |
| R-06 | Booking-heavy | Barcelona 5d | IngestBooking (3 types), ExtractBookingField, FTC disclosure |
| R-07 | Update regression | Structural | COALESCE partial updates (no AI, deterministic) |
| R-11 | Food blogger | Mexico City | Expert handoff (food), tour booking, recommend_booking |
| R-16 | History professor | Greece + Turkey | Academic depth, 3 expert handoffs, multi-country |
| R-20 | Luxury traveler | Maldives + Dubai | Luxury calibration, recommend_booking |

**Edge case & gap coverage (N-*)** — Target untested features and boundaries:

| # | Persona | Focus | Gap Targeted |
|---|---------|-------|-------------|
| N-01 | Companion power user | Bangkok, 5+ companion msgs | Companion mode (was 2/20) |
| N-02 | Chat history verifier | Structural | ListChatSessions, GetChatHistory (was 0/20) |
| N-03 | REST endpoint exerciser | Structural | /api/usage, /referral, /guides (was 0/20) |
| N-04 | Booking field extractor | All 7 artifacts | ExtractBookingField coverage (was 1/20) |
| N-05 | Trip sharing deep test | Share lifecycle | Sharing enable/disable/revoke/re-share (was 1/20) |
| N-06 | Budget enforcement | India $10/day | Budget constraint strictness |
| N-07 | Dietary stress test | Japan (vegan+GF+nut allergy) | Compound dietary restrictions |
| N-08 | Adversarial edge cases | Structural | Error handling, validation, boundary conditions |
| N-09 | Rapid fire conversation | Italy 10 msgs | Multi-turn context retention, modification handling |
| N-10 | Last-minute traveler | Lisbon 2 days | Companion-first flow, urgency, brevity |
| N-11 | Lifecycle stress test | Full CRUD | Create→update→activate→share→complete→delete |
| N-12 | Cultural sensitivity | Israel+Saudi+Myanmar | Political sensitivity, religious etiquette, ethical tourism |

### Booking Artifacts (`tests/agentic/artifacts/`)

Fake booking confirmation texts for ingestion testing:
- `flight-confirmation.txt` — Delta JFK→BCN round trip
- `hotel-confirmation.txt` — Hotel Arts Barcelona
- `activity-confirmation.txt` — Sagrada Familia tour
- `car-rental-confirmation.txt` — Hertz BCN→LIS one-way
- `hostel-booking.txt` — Vietnam 3-hostel chain
- `ryokan-booking.txt` — Kyoto traditional inn
- `tour-booking.txt` — Oaxaca food walking tour

### Adding New Personas

Create a new file in `tests/agentic/personas/NN-name.md` following the existing format (Background, Your Trip, What to Test, Booking Artifacts, Special Attention). The `agentic-test` skill (`.claude/skills/agentic-test/SKILL.md`) provides the testing framework.

### Report Format

Each agent returns a structured JSON report with: bugs (P0/P1/P2), UX issues, AI behavior issues, tool failures, and a usefulness evaluation (1-5 scores for trip creation, itinerary quality, persona handoff, booking parsing, companion mode, and overall). See the skill for the full schema.

## Infrastructure

GCP infrastructure is managed in the [toqui-terraform](https://github.com/gallowaysoftware/toqui-terraform) repo. Infrastructure changes go through PRs — CI auto-plans and posts the plan as a PR comment, then auto-applies on merge to `main`.

**Three GCP projects** under the Toqui folder in the `thegalloways.ca` org:

- **toqui-infra** — Shared singleton infra: Artifact Registry, WIF, GitHub branch protection. Managed by `environments/infra/` in toqui-terraform.
- **toqui-staging** — Runtime kept running (~$32/mo) for continuous testing. Can be torn down if needed. Pulls images from `toqui-infra` AR.
- **toqui-prod** — LIVE. Cloud Run (backend + frontend) + Global HTTPS LB + Cloud Armor WAF + Certificate Manager SSL. Cloud SQL `db-g1-small` (private IP, HA, backups). Domains: `api.toqui.travel`, `app.toqui.travel`. Marketing site + admin on Cloudflare Pages.

Prod uses Cloud SQL PostgreSQL 16 (private IP), Firestore (native mode), Secret Manager, Resend (email), Stripe (payments). Images pulled from `toqui-infra` Artifact Registry.

**Company**: Galloway Software Inc., Prince Edward Island, Canada.

### Deploying to Prod

**Automatic**: Push to `main` → GitHub Actions runs migrations via Cloud Run Jobs FIRST, then builds Docker image, pushes to Artifact Registry, and deploys to Cloud Run. Migrations run before deploy to avoid schema mismatch. Uses WIF (keyless GCP auth).

**Manual** (if needed):

```bash
IMAGE=us-central1-docker.pkg.dev/toqui-prod/toqui-backend/toqui-backend

# Build and push
docker build --platform linux/amd64 -t $IMAGE:latest .
docker push $IMAGE:latest

# Run migrations FIRST
gcloud run jobs deploy toqui-migrate --image=$IMAGE:latest \
  --region=us-central1 --project=toqui-prod \
  --command=/migrate --args="-direction,up" --execute-now

# Then deploy
gcloud run deploy toqui-backend --image=$IMAGE:latest --region=us-central1 --project=toqui-prod
```

### Rolling Back

```bash
# List available revisions
gcloud run revisions list --service=toqui-backend --region=us-central1 --project=toqui-prod

# Route traffic to previous revision
gcloud run services update-traffic toqui-backend \
  --to-revisions=<previous-revision>=100 --region=us-central1 --project=toqui-prod

# Roll back one database migration
gcloud run jobs deploy toqui-migrate --image=$IMAGE:<previous-sha> \
  --region=us-central1 --project=toqui-prod \
  --command=/migrate --args="-direction,down,-steps,1" --execute-now
```

### Checking Prod Logs

```bash
# Cloud Run logs (real-time)
gcloud run services logs read toqui-backend --region=us-central1 --project=toqui-prod --limit=100

# Or via Cloud Logging
gcloud logging read 'resource.type="cloud_run_revision" AND resource.labels.service_name="toqui-backend"' \
  --project=toqui-prod --limit=50 --format=json
```

Prod is publicly accessible at `https://api.toqui.travel` (backend) and `https://app.toqui.travel` (frontend).

### Docker Image

The Dockerfile produces a distroless image with two binaries:

- `/server` — main API server (entrypoint)
- `/migrate` — database migration runner (auto-detects `/migrations` in Docker, `db/migrations/` locally)

Migrations are copied to `/migrations` in the image. The `cmd/migrate` binary reads `DATABASE_URL` from the environment and uses `golang-migrate`.

## Auth Flow

**Dual-mode auth**: Web browsers use HttpOnly cookies; native apps use `Authorization: Bearer` header directly. The cookie-to-header middleware (`internal/middleware/cookieauth.go`) transparently bridges cookie auth into the existing Bearer token flow — all handlers and interceptors see `Authorization: Bearer` regardless of client type.

**Web browser flow**: Google OAuth → backend callback → set temporary HttpOnly cookie → redirect to frontend → frontend calls `POST /auth/exchange` (with `credentials: include`) → backend returns user info + `expires_at` in response body, sets `toqui_access` and `toqui_refresh` HttpOnly cookies, clears OAuth cookie → frontend stores only user info in localStorage (no tokens).

**Native app flow**: Same OAuth or direct token exchange → tokens returned in JSON response body → app stores tokens and sets `Authorization: Bearer` header on all requests.

**Auth cookies** (set by `internal/auth/cookies.go`):
- `toqui_access` — HttpOnly, Secure, SameSite=Lax, Path=/, MaxAge=3600 (1 hour)
- `toqui_refresh` — HttpOnly, Secure, SameSite=Lax, Path=/auth, MaxAge=2592000 (30 days)
- No Domain attribute (host-only cookies — only sent to exact API domain)

HTTP routes (outside ConnectRPC):

### Auth routes
- `GET /auth/google/login` — Initiates OAuth, sets state cookie, redirects to Google
- `GET /auth/google/callback` — Exchanges code, checks capacity cap, sets `toqui_oauth_result` cookie (60s TTL), redirects to frontend `/auth/callback`
- `GET /auth/facebook/login` — Initiates Facebook OAuth, sets state cookie, redirects to Facebook
- `GET /auth/facebook/callback` — Exchanges code, checks capacity cap, sets OAuth result cookie, redirects to frontend
- `POST /auth/exchange` — Reads OAuth cookie, returns `{user, expires_at}`, sets `toqui_access`/`toqui_refresh` HttpOnly cookies
- `POST /auth/refresh` — Cookie-based token refresh. Rotates tokens (JTI/family), sets new cookies, returns `{user, expires_at}`
- `POST /auth/logout` — Revokes refresh token, clears auth cookies, returns 204
- `POST /auth/verify-age` — Authenticated. JSON body `{"date_of_birth":"YYYY-MM-DD"}`. Verifies user is 18+, stores verification.

### Waitlist routes
- `POST /waitlist` — Public. JSON `{"email":"..."}`. Sends verification email via Resend, returns `{"message":"Check your email to verify your waitlist signup!"}`. Re-submission resends verification. In local dev (no RESEND_API_KEY), auto-verifies.
- `GET /waitlist/verify?token=TOKEN` — Public. Verifies email, completes waitlist signup, shows position.
- `GET /waitlist/status?email=...` — Public. Returns `{"position":N,"total":M}`

### Usage, feedback & guides (public/authenticated)
- `GET /api/usage` — Authenticated. Returns `{"used":N,"limit":M,"resets_at":"..."}`
- `POST /api/feedback` — Authenticated. Submit user feedback.
- `GET /api/guides` — Public. Lists all destination guides (slug, title, destination).
- `GET /api/guides/{slug}` — Public. Returns full destination guide content.

### Trip sharing & collaboration
- `POST /api/trips/share` — Authenticated. Enables trip sharing, returns share token.
- `POST /api/trips/unshare` — Authenticated. Disables trip sharing.
- `GET /shared/{token}` — Public. Returns shared trip view (no auth required).
- `POST /api/trips/accept-invite` — Authenticated. Accept a collaboration invite.

### Referral
- `GET /api/referral` — Authenticated. Returns user's referral code and stats.
- `POST /api/referral/redeem` — Authenticated. Redeems a referral code.

### Payment / checkout
- `POST /api/checkout` — Authenticated. Creates Stripe checkout session for Trip Pro purchase (`{"trip_id":"..."}`). Returns Stripe checkout URL.
- `GET /api/checkout/status?trip_id=...` — Authenticated. Returns `{"unlocked":true/false}`.

### Subscription
- `POST /api/subscription/checkout` — Authenticated. Creates Stripe checkout session for Explorer/Voyager subscription.
- `GET /api/subscription` — Authenticated. Returns current subscription tier and status.
- `POST /api/subscription/cancel` — Authenticated. Cancels active subscription at period end.
- `POST /api/subscription/portal` — Authenticated. Creates Stripe billing portal session URL.
- `POST /api/subscription/webhook` — Stripe webhook endpoint (signature verified). Processes subscription lifecycle events.

### Admin (requires admin auth: Bearer + email in ADMIN_EMAILS)
- `GET /admin/stats` — Dashboard stats (users, waitlist, trips, messages)
- `GET /admin/users?search=&limit=&offset=` — Paginated user list
- `GET /admin/waitlist?limit=&offset=` — Paginated waitlist entries
- `POST /admin/invite` — Generate invite code
- `POST /admin/send-invite` — Send invite email via Resend
- `POST /admin/revoke-invite` — Revoke invite code
- `POST /admin/delete-waitlist` — Delete waitlist entry
- `POST /admin/unlock-trip` — Admin unlock a trip (grant Pro access)
- `POST /admin/grant-pro` — Grant Pro status to a user
- `POST /admin/delete-user` — Delete a user and all associated data
- `GET /admin/feedback` — List user feedback submissions
- `GET /admin/metrics` — System metrics

### Webhooks
- `POST /webhooks/email/inbound` — SendGrid inbound email webhook (ECDSA signature verified). Processes forwarded booking emails.

### Health checks
- `GET /livez` — Liveness probe (always 200 if server is running)
- `GET /readyz` — Readiness probe (checks DB connectivity)
- `GET /healthz` — Health check with DB ping
- `GET /health` — Detailed health check (component statuses + uptime)

### Debug (local only)
- `GET /debug/pprof/*` — Go profiling endpoints (disabled in staging/prod)

## Security Hardening

### Middleware Chain

```
Request → recovery → requestID → requestLogging → securityHeaders → CORS → cookieAuth → ipRateLimit → CSRF → handler
```

- **Recovery**: Panic recovery with structured error logging
- **Request ID**: Generates unique request IDs for tracing
- **Request logging**: Structured HTTP request/response logging (method, path, status, duration) via slog
- **Security headers**: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: strict-origin-when-cross-origin`, `Permissions-Policy: camera=(), microphone=(), geolocation=()`, `Strict-Transport-Security` (HTTPS only)
- **CORS**: Cross-origin resource sharing for frontend origins. `Access-Control-Allow-Credentials: true` on all routes (required for browsers to send HttpOnly cookies on cross-origin same-site requests). CSRF middleware prevents abuse.
- **Cookie auth**: Reads `toqui_access` HttpOnly cookie and sets `Authorization: Bearer` header on the request. Passthrough if `Authorization` header already present (native apps). See `internal/middleware/cookieauth.go`.
- **IP rate limit**: Per-IP request rate limiting (separate from per-user ConnectRPC rate limiting). Runs AFTER cookieAuth so the limiter can use the user identity set by cookieAuth for smarter rate limiting.
- **CSRF**: Origin/Referer header validation for state-changing requests (POST/PUT/DELETE/PATCH). Exempt: webhooks (have their own ECDSA auth). Non-browser clients (no Origin/Referer) are allowed through.
- **Request body limits**: All REST POST handlers use `http.MaxBytesReader(w, r.Body, 1<<20)` — 1MB max

### JWT Token Types

- **Access tokens**: No `type` claim, 1-hour expiry. Used for `Authorization: Bearer`.
- **Refresh tokens**: `type: "refresh"` claim, 30-day expiry. Only accepted by `ValidateRefreshToken()`.
- **Important**: `ValidateToken()` explicitly rejects tokens with `type == "refresh"` to prevent token type confusion.

### Refresh Token Rotation

Refresh tokens are DB-backed with JTI (JWT ID) and family tracking:

1. **Login** creates a new token family (random UUID). The refresh token's JTI and family are stored in the `refresh_tokens` table.
2. **Refresh** validates the JTI against the database, revokes the used token, and issues a new token in the same family.
3. **Reuse detection**: If a revoked token is presented, the entire family is revoked (breach response). This catches token theft where both the attacker and legitimate user try to use the same token.
4. **Cleanup**: Expired tokens can be purged via `DeleteExpiredRefreshTokens` query.

### Auth Lockout

Per-IP failure tracking via `AuthLimiter` (in `internal/ratelimit/`):

- **5 failed attempts** within a 15-minute window → IP blocked for 15 minutes
- Applied to `POST /auth/exchange` (OAuth) and `RefreshToken` RPC
- Successful auth clears the failure counter
- Background goroutine cleans up stale entries

### Audit Logging

Structured audit events via `internal/audit/` package, written through `slog` for automatic Cloud Logging collection:

- `auth.login`, `auth.login_denied.domain`, `auth.login_denied.capacity`, `auth.login_admitted.invite`
- `auth.token_refresh`, `auth.token_refresh_denied`, `auth.token_reuse_detected`, `auth.lockout`
- `auth.logout`, `auth.account_delete`, `auth.data_export`
- `trip.share`, `trip.unshare`
- `security.csrf_rejected`
- `payment.trip_pro_purchase`, `payment.validation_failed` — Stripe payment audit trail
- `admin.invite`, `admin.trip_unlock`, `admin.grant_pro` — Admin action audit trail
- `referral.redeem` — Referral code redemption

### Cookie Encoding (OAuth)

The `toqui_oauth_result` cookie uses **base64url encoding** (`base64.RawURLEncoding`) because Go's `net/http` silently strips `"` characters from cookie values per RFC 6265. The JSON payload would be corrupted without encoding. The cookie uses `SameSite=None` + `Secure=true` because the frontend (`app.toqui.travel`) and backend (`api.toqui.travel`) are on different subdomains.

### Security Checklist for New Handlers

When adding new handlers, ensure:

1. **Ownership checks**: All data-access handlers verify `userID` from auth context before returning data
2. **Body limits**: REST POST handlers use `http.MaxBytesReader`
3. **Domain allowlist**: Any new auth paths must check `isEmailDomainAllowed()`
4. **Token type**: Only use `ValidateToken()` for access tokens, `ValidateRefreshToken()` for refresh
5. **JWT enforcement**: Non-local environments fail startup if `JWT_SECRET` is the default
6. **CSRF exempt**: If adding a new webhook endpoint, add its prefix to the CSRF exempt list in `main.go`
7. **Audit logging**: Security-relevant events (auth, data access, sharing) must use `audit.Log()`

### Known Open Security Issues

See [GitHub Issues with `security` label](https://github.com/gallowaysoftware/toqui-backend/issues?q=label:security) for the full list.

## Waitlist + Capacity Cap

New users are subject to a capacity cap controlled by `MAX_FREE_USERS` (default: 500). When the cap is reached:

- Existing users can still log in (upsert on google_id)
- New users without a valid invite code are redirected to `/waitlist?reason=at_capacity`
- New users with a valid invite code are admitted and their waitlist entry is marked as accepted

Tables: `waitlist` (email, invite_code, signed_up_at, invited_at, accepted_at)

### Email Verification Flow
Waitlist signups now require email verification:
1. `POST /waitlist` — accepts email, sends verification link via Resend, returns a "check your email" message
2. `GET /waitlist/verify?token=TOKEN` — verifies email, shows position in waitlist
3. Re-submission of an existing unverified email resends the verification email
4. In local dev (no `RESEND_API_KEY`), entries are auto-verified immediately

## Additional Features

### Payment & Trip Pro
The `internal/payment/` package handles Stripe payment integration for Trip Pro ($19/trip):
- `POST /api/checkout` creates a Stripe checkout session
- Stripe sends webhook on successful payment → backend unlocks the trip
- `GET /api/checkout/status` lets the frontend poll unlock status
- Unlocked trips have access to unlimited messages, all personas, and export features

### Referral System
Users get a referral code via `GET /api/referral`. Codes can be redeemed at `POST /api/referral/redeem`. Redemption is audit-logged. Referral stats (count of referred users) are returned with the code.

**Referral Rewards**: When a referred user signs up and creates their first trip, both the referrer and referee receive a free trip unlock (Trip Pro). The reward is granted automatically via the referral redemption flow. Rewards are capped at 10 per user.

### Destination Guides API
Static destination guide content served at `/api/guides` (list) and `/api/guides/{slug}` (detail). Used by the marketing site's guide pages. Public endpoints — no auth required.

### Trip Sharing
Users can share a trip publicly via `POST /api/trips/share`, which generates a share token. The public view is accessible at `/shared/{token}` without authentication. Sharing can be revoked via `POST /api/trips/unshare`.

### PostHog Analytics (Server-Side)

Server-side event tracking via PostHog (EU-hosted, `eu.i.posthog.com`). User IDs are SHA-256 hashed before sending. Five tracked events:

- `user_signed_up` — new account created
- `trip_created` — new trip created
- `trip_pro_purchased` — Trip Pro payment completed
- `referral_redeemed` — referral code redeemed
- `message_sent` — chat message sent (count only, no content)

Requires `POSTHOG_API_KEY` env var. Events are fire-and-forget (non-blocking). No PII or travel content is ever sent to PostHog.

### AI Prompt Engineering

The AI system prompt has been tuned for reliable tool usage. Key behaviors:

- **Always create itinerary items**: When a user discusses plans, the AI proactively calls `create_itinerary_items` to add structured items to the itinerary. It does not just describe plans in text.
- **Proactive expert handoff**: The AI calls `suggest_expert` when the conversation topic matches a location/theme combination, without waiting for the user to ask.
- **Clarifying questions**: The AI asks about travel preferences (dates, budget, interests, dietary restrictions) before making recommendations, rather than assuming.
- **Tool result confirmation**: After calling tools, the AI confirms what was created/updated in its response text.

When modifying system prompts in `internal/chat/` or `internal/persona/`, run the agentic test suite or use `grpcurl` to verify the AI still calls tools correctly.

### Deep Linking

Apple App Site Association (AASA) and Android Asset Links are served at well-known endpoints for universal links / app links:

- `GET /.well-known/apple-app-site-association` — AASA JSON for iOS universal links
- `GET /.well-known/assetlinks.json` — Android asset links for app links

These enable deep linking from `toqui.travel` URLs directly into the native app.

### Inbound Email Webhook
`POST /webhooks/email/inbound` processes emails forwarded to the platform (e.g., booking confirmations forwarded to the user's Toqui email address). Payload is ECDSA-signed by SendGrid and verified before processing.

## Daily Usage Limits

Daily message limits are tier-specific: free=10, pro=50, explorer/voyager=unlimited. Configurable via `DAILY_MESSAGE_LIMIT_FREE` and `DAILY_MESSAGE_LIMIT_PRO` env vars. The limit is enforced at the start of `SendMessage` in the chat handler. When exceeded, a `ResourceExhausted` error is returned with the reset time (midnight UTC).

Tables: `daily_usage` (user_id, date, message_count, ai_cost_cents)

## Cost Management

### AI Provider Architecture

**Gemini (primary, default)** — Google Vertex AI with Application Default Credentials (ADC). No API key needed — uses the same `gcloud auth application-default login` as Secret Manager. Billing is per-GCP-project, providing natural environment separation. Requires `aiplatform.googleapis.com` API enabled and `roles/aiplatform.user` IAM role on the calling identity.

**Claude (fallback)** — Anthropic API with API key. Set a monthly spend cap in the [Anthropic Console](https://console.anthropic.com) → Settings → Billing → Spend Limits. Recommended: $50/month staging, $500/month prod.

| Model Tier | Claude                     | Gemini (Vertex AI)      |
| ---------- | -------------------------- | ----------------------- |
| fast       | `claude-haiku-4-20250414`  | `gemini-2.5-flash-lite` |
| smart      | `claude-sonnet-4-20250514` | `gemini-2.5-flash`      |
| best       | `claude-sonnet-4-20250514` | `gemini-2.5-pro`        |

Override models via env vars: `AI_MODEL_FAST/SMART/BEST` (Claude), `AI_GEMINI_MODEL_FAST/SMART/BEST` (Gemini).

### Token Usage Tracking

Every AI request logs token usage with environment label via slog:

```
INFO ai request completed provider=gemini env=staging input_tokens=2662 output_tokens=200 total_tokens=2862 tool_loop_iterations=2
```

Token counts are accumulated across tool loop iterations. Usage is parsed from Claude's `message_start`/`message_delta` events and Gemini's `usageMetadata`.

### Cost Controls (implemented)

- **Daily token budget**: `DAILY_AI_TOKEN_BUDGET` — soft limit per environment (default: 0 = unlimited). 1M staging, 5M prod.
- **Prompt caching**: system prompts cached for 5 min (90% cheaper on cache hits, Claude only)
- **Model routing**: simple tasks → fast tier (2048 max tokens), complex → smart tier (8192 max tokens)
- **Daily message limit**: 30 msgs/user/day (configurable via `DAILY_MESSAGE_LIMIT`)
- **Response caching**: popular destination intros cached for 1 hour (configurable via `LLM_CACHE_TTL`)
- **Per-user rate limiting**: 10 requests per 60 seconds via ConnectRPC interceptor
- **GCP project separation**: toqui-staging vs toqui-prod — billing differentiation at the infra level

## Cross-Repo Consistency

**IMPORTANT**: This project spans 5 repos. When making changes that affect shared documentation (architecture, deployment, CI/CD, security patterns, staging/prod status), update CLAUDE.md in ALL repos to keep them consistent:

- `/Users/pequalsnp/src/github.com/gallowaysoftware/toqui-backend/CLAUDE.md` (this file)
- `/Users/pequalsnp/src/github.com/gallowaysoftware/toqui/CLAUDE.md`
- `/Users/pequalsnp/src/github.com/gallowaysoftware/toqui-terraform/CLAUDE.md`
- `/Users/pequalsnp/src/github.com/gallowaysoftware/toqui-site/CLAUDE.md`
- `/Users/pequalsnp/src/github.com/gallowaysoftware/toqui-admin/CLAUDE.md`


## Data Lifecycle

- **Location data**: Ephemeral in-memory cache (30 min TTL), never persisted to database
- **Trip archival**: 90 days after completion, chat messages purged from Firestore
- **User deletion**: GDPR Article 17 — CASCADE deletes in Postgres + Firestore purge, within 30 days
- **Data export**: GDPR Article 20 — async job generates downloadable archive
