# Toqui Backend

Go backend for Toqui, an AI-powered travel companion. Built with ConnectRPC, PostgreSQL + PostGIS, Firestore, and Claude.

## Prerequisites

- Go 1.26+
- [buf](https://buf.build/docs/installation) (proto generation)
- Docker & Docker Compose (local Postgres + Firestore emulator)
- [sqlc](https://sqlc.dev/) (SQL code generation)
- [golangci-lint](https://golangci-lint.run/) (optional, for linting)
- [gcloud CLI](https://cloud.google.com/sdk/docs/install) — required for Secret Manager resolution (`gcloud auth application-default login`)

## Quick Start

```bash
# 1. Start Postgres + Firestore emulator
make docker-up

# 2. Run database migrations
make migrate-up

# 3. Authenticate with GCP (for Secret Manager)
gcloud auth application-default login

# 4. Run the server
make run
# Server starts on http://localhost:8090
```

## Environment Configuration

Config is loaded automatically based on `TARGET_ENV` (default: `local`):

```bash
make run                        # Local dev (loads env/.env.local, resolves gcsm:// secrets)
make run-staging                # Staging (loads env/.env.staging, resolves gcsm:// secrets)
make run-prod                   # Production (loads env/.env.prod, resolves gcsm:// secrets)
TARGET_ENV=staging make run     # Same as make run-staging
```

Env files live in `env/`:
- `env/.env.local` — Local dev (`gcsm://` secrets from `toqui-staging` project)
- `env/.env.staging` — Staging infrastructure + `gcsm://` secret references
- `env/.env.prod` — Production infrastructure + `gcsm://` secret references

All env files use `gcsm://` prefixed values which are resolved from GCP Secret Manager at startup. This means no secrets are stored in the repo. Requires `gcloud auth application-default login` for local dev. Real environment variables always take precedence over the env file.

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `TARGET_ENV` | No | `local` | Environment: `local`, `staging`, `prod` |
| `GOOGLE_CLIENT_ID` | Yes | — | Google OAuth client ID |
| `GOOGLE_CLIENT_SECRET` | Yes | — | Google OAuth client secret |
| `ANTHROPIC_API_KEY` | Yes* | — | Claude API key |
| `OPENAI_API_KEY` | Yes* | — | OpenAI API key (fallback) |
| `DATABASE_URL` | No | `postgres://toqui:toqui@localhost:5432/toqui?sslmode=disable` | PostgreSQL connection |
| `PORT` | No | `8090` | Server port |
| `JWT_SECRET` | No | dev default | JWT signing secret |
| `FIRESTORE_PROJECT_ID` | No | `toqui-dev` | Firestore project |
| `FIRESTORE_EMULATOR_HOST` | No | — | Firestore emulator address |
| `FRONTEND_URL` | No | `http://localhost:3000` | CORS origin |

*At least one AI provider key is required.

## Make Targets

```bash
make run                # Run server (local env, default)
make run-staging        # Run locally against staging
make run-prod           # Run locally against prod
make build              # Build server binary to bin/server
make test               # Run unit tests
make lint               # Run golangci-lint
make proto              # Generate Go proto code + lint
make sqlc               # Generate Go from SQL queries
make docker-up          # Start Postgres + Firestore emulator
make docker-down        # Stop Docker services
make migrate-up         # Apply pending migrations
make migrate-down       # Rollback one migration
make migrate-create     # Create new migration files
make integration-test   # Run integration tests (starts its own Docker)
make ai-test            # Run AI regression tests (needs Docker + AI key)
make ai-test-generative # Run AI regression + generative tests
```

## Testing

### Unit Tests

```bash
make test
```

### Integration Tests

Uses a dedicated Docker setup with Postgres and Firestore emulator:

```bash
make integration-test
```

### AI Integration Tests

End-to-end tests that exercise the full trip lifecycle through the AI with real LLM calls. Requires Docker services running and an AI provider key (`ANTHROPIC_API_KEY` or `OPENAI_API_KEY`).

```bash
docker compose up -d    # Start Postgres + Firestore emulator
make ai-test            # Run 4 regression scenarios
make ai-test-generative # Run regression + LLM-generated exploratory scenarios
```

Run a specific scenario:

```bash
go test -tags=aitest -v -timeout=30m ./internal/aitest/... -run TestAIScenarios/alice
```

Reports are written to `testdata/aitest-reports/`.

## Project Structure

```
cmd/
  server/           # API server entry point
  migrate/          # Database migration runner
internal/
  handlers/         # ConnectRPC service handlers (auth, trip, chat, booking, location, persona)
  chat/             # Chat service — AI streaming, tool execution, persona resolution
  persona/          # Persona composition (24 locations × 15 themes)
  ai/               # AI provider abstraction (Claude, OpenAI)
  ai/tools/         # LLM-callable tool registry (WebSearch, Places)
  chatstore/        # Firestore chat message persistence
  auth/             # Google OAuth + JWT + auth interceptor
  trip/             # Trip CRUD, status transitions
  booking/          # Booking ingestion + AI parsing
  location/         # Ephemeral location, nearby places
  theme/            # Trip theme tagging (AI-driven)
  lifecycle/        # GDPR deletion, archival, data export
  config/           # Three-layer config (env file → defaults → Secret Manager)
  db/               # PostgreSQL connection pool + transactions
  validate/         # Request validation interceptor (buf.validate)
  ratelimit/        # Per-user rate limiting interceptor
  aitest/           # AI integration test harness (build tag: aitest)
  integration/      # Integration test suite (build tag: integration)
  dbgen/            # Generated sqlc code
proto/toqui/v1/     # Protobuf service definitions
gen/toqui/v1/       # Generated Go proto code
env/                # Environment configs (.env.local, .env.staging, .env.prod)
db/
  migrations/       # SQL migrations (golang-migrate)
  queries/          # sqlc query definitions
```

## Related Repos

- [toqui](https://github.com/gallowaysoftware/toqui) — Next.js frontend
- [toqui-terraform](https://github.com/gallowaysoftware/toqui-terraform) — Terraform GCP infrastructure (staging + prod)
- [toqui-site](https://github.com/gallowaysoftware/toqui-site) — Astro marketing site
