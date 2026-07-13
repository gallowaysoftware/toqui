# Toqui Backend

Go backend for Toqui, an AI-powered travel companion. Built with ConnectRPC, PostgreSQL + PostGIS, and Gemini/Claude/OpenAI-compatible AI providers.

> **Status:** This repo is part of Toqui's transition from a hosted SaaS to a
> self-hostable open source project under [AGPL-3.0-or-later](./LICENSE).
> SaaS surfaces (Stripe, subscription, referral, analytics, age gate, etc.)
> are actively being stripped; AI orchestration is moving to a BYO-API-key
> model. See [toqui.travel](https://toqui.travel) for context.
>
> Copyright (C) 2026 Galloway Software Solutions Inc.

## Prerequisites

- Go 1.26+
- [buf](https://buf.build/docs/installation) (proto generation)
- Docker & Docker Compose (local Postgres)
- [sqlc](https://sqlc.dev/) (SQL code generation)
- [golangci-lint](https://golangci-lint.run/) (optional, for linting)
- [gcloud CLI](https://cloud.google.com/sdk/docs/install) — only needed if you use `gcsm://` Secret Manager references in env files

## Quick Start

```bash
# 1. Start Postgres
docker compose up -d postgres

# 2. Run database migrations
make migrate-up

# 3. Run the server (needs an AI provider key, e.g. GEMINI_API_KEY,
#    ANTHROPIC_API_KEY, or OPENAI_API_KEY + OPENAI_BASE_URL)
make run
# Server starts on http://localhost:8090
```

## Environment Configuration

Config is loaded automatically based on `TARGET_ENV` (default: `local`). If `env/.env.{TARGET_ENV}` exists it is parsed first (real environment variables take precedence); there is no `env/.env.local` — local dev runs on plain env vars and defaults. Values prefixed `gcsm://` are resolved from GCP Secret Manager at startup (requires `gcloud auth application-default login`); plain values never touch GCP.

### Environment Variables

| Variable                  | Required | Default                                                       | Description                                 |
| ------------------------- | -------- | ------------------------------------------------------------- | ------------------------------------------- |
| `TARGET_ENV`              | No       | `local`                                                       | Environment: `local`, `staging`, `prod`     |
| `GOOGLE_CLIENT_ID`        | No       | —                                                             | Optional Google OAuth client ID (unset = email+password only) |
| `GOOGLE_CLIENT_SECRET`    | No       | —                                                             | Optional Google OAuth client secret         |
| `ANTHROPIC_API_KEY`       | Yes\*    | —                                                             | Claude API key                              |
| `GEMINI_API_KEY`          | Yes\*    | —                                                             | Gemini Developer API key (default primary provider) |
| `OPENAI_API_KEY`          | Yes\*    | —                                                             | OpenAI or OpenAI-compatible endpoint key    |
| `VERTEX_AI_PROJECT_ID`    | Yes\*    | —                                                             | GCP project for Vertex AI Gemini (ADC auth) |
| `VERTEX_AI_LOCATION`      | No       | `us-central1`                                                 | Vertex AI region                            |
| `DAILY_AI_TOKEN_BUDGET`   | No       | `0`                                                           | Max total AI tokens/day (0 = unlimited)     |
| `DATABASE_URL`            | No       | `postgres://toqui:toqui@localhost:5432/toqui?sslmode=disable` | PostgreSQL connection                       |
| `PORT`                    | No       | `8090`                                                        | Server port                                 |
| `JWT_SECRET`              | No       | dev default                                                   | JWT signing secret                          |
| `FIRESTORE_PROJECT_ID`    | No       | `toqui-dev`                                                   | GCP project (Secret Manager, Vertex fallback) |
| `FRONTEND_URL`            | No       | `http://localhost:3000`                                       | CORS origin                                 |

\*At least one AI provider is required: `GEMINI_API_KEY` (default primary), `ANTHROPIC_API_KEY`, or `OPENAI_API_KEY` (+ `OPENAI_BASE_URL` for OpenAI-compatible endpoints — set `AI_PROVIDER=openai`). Vertex AI works via ADC + `VERTEX_AI_PROJECT_ID`. See `backend/CLAUDE.md` for the full variable table.

## Make Targets

```bash
make run                # Run server (local env, default)
make build              # Build server binary to bin/server
make test               # Run unit tests
make lint               # Run golangci-lint
make proto              # Generate Go proto code + lint
make sqlc               # Generate Go from SQL queries
make docker-up          # docker compose up -d (postgres + migrate + backend)
make docker-down        # Stop Docker services
make migrate-up         # Apply pending migrations
make migrate-down       # Rollback one migration
make migrate-create     # Create new migration files
make integration-test   # Run integration tests (starts compose Postgres itself)
make genguides          # Regenerate destination guides (needs AI key)
```

## Testing

### Unit Tests

```bash
make test
```

### Integration Tests

Runs against the dev-compose Postgres:

```bash
make integration-test
```

### Agentic Tests

Black-box tests where Claude agents adopt traveler personas against a running backend. See the "Agentic Testing" section in [`CLAUDE.md`](./CLAUDE.md) and `tests/agentic/`.

## CI

GitHub Actions (`.github/workflows/ci.yml` at the repo root) runs on push to `main` and all PRs: backend build, test, lint, and integration test (against a PostGIS service container), plus the frontend jobs. There are no deploy jobs — deployment for self-hosters is documented in [`/DEPLOYMENT.md`](../DEPLOYMENT.md).

### Database Migrations

```bash
# Create a new migration
make migrate-create
# Produces: db/migrations/YYYYMMDDHHMMSS_name.up.sql + .down.sql

# Apply locally
make migrate-up

# Rollback locally
make migrate-down
```

**Note**: The `cmd/migrate` binary auto-detects migration files at `/migrations` (Docker) or `db/migrations/` (local).

### Docker Image

The Dockerfile produces a distroless image with two binaries:

- `/server` — main API server (entrypoint)
- `/migrate` — database migration runner

Migrations are copied to `/migrations` in the image.

```bash
# Build locally (for Apple Silicon, cross-compile for Linux)
docker build --platform linux/amd64 -t toqui-backend .

# Test locally
docker run -p 8090:8090 \
  -e DATABASE_URL=postgres://toqui:toqui@host.docker.internal:5432/toqui?sslmode=disable \
  toqui-backend
```

## Project Structure

```
cmd/
  server/           # API server entry point
  migrate/          # Database migration runner
  testctl/          # Test user/token CLI for agentic testing
  genguides/        # Destination guide generator (dev machine)
internal/
  handlers/         # ConnectRPC service handlers + REST handlers + chat tools
  chat/             # Chat service — AI streaming, tool loop, persona resolution
  persona/          # Persona composition (43 locations × 23 themes)
  ai/               # AI provider abstraction (Gemini, Claude, OpenAI-compatible)
  ai/tools/         # Global LLM tool registry (web_search)
  chatstore/        # Postgres chat session/message persistence
  auth/             # Email+password + optional Google OAuth + JWT + refresh rotation
  trip/             # Trip CRUD, status transitions
  booking/          # Booking ingestion + AI parsing
  location/         # Ephemeral location, nearby places
  theme/            # Trip theme tagging (AI-driven)
  lifecycle/        # GDPR deletion, archival, chat purge, data export
  config/           # Three-layer config (env file → defaults → Secret Manager)
  db/               # PostgreSQL connection pool + transactions
  validate/         # Request validation interceptor (buf.validate)
  ratelimit/        # Per-user + per-IP rate limiting, auth lockout
  integration/      # Integration test suite (build tag: integration)
  dbgen/            # Generated sqlc code
proto/toqui/v1/     # Protobuf service definitions
gen/toqui/v1/       # Generated Go proto code
db/
  migrations/       # SQL migrations (golang-migrate)
  queries/          # sqlc query definitions
tests/
  agentic/          # Agentic test personas, artifacts, baselines
  bruno/            # Bruno HTTP client collections
```

## Related

- Monorepo root — Expo React Native frontend (web + iOS + Android)
- [`/DEPLOYMENT.md`](../DEPLOYMENT.md) — self-host deployment guide
- [toqui-site](https://github.com/gallowaysoftware/toqui-site) — Astro site at toqui.travel
