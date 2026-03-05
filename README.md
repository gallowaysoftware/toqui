# Toqui Backend

Go backend for Toqui, an AI-powered travel companion. Built with ConnectRPC, PostgreSQL + PostGIS, Firestore, and Claude.

## Prerequisites

- Go 1.25+
- [buf](https://buf.build/docs/installation) (proto generation)
- Docker & Docker Compose (local Postgres + Firestore emulator)
- [sqlc](https://sqlc.dev/) (SQL code generation)
- [golangci-lint](https://golangci-lint.run/) (optional, for linting)

## Quick Start

```bash
# 1. Start Postgres + Firestore emulator
make docker-up

# 2. Run database migrations
make migrate-up

# 3. Copy .env.example and fill in credentials
cp .env.example .env
# Edit .env with your GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, ANTHROPIC_API_KEY

# 4. Run the server
make run
# Server starts on http://localhost:8090
```

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
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
make proto              # Generate Go proto code + lint
make sqlc               # Generate Go from SQL queries
make build              # Build server binary to bin/server
make run                # Run server (go run)
make test               # Run unit tests
make lint               # Run golangci-lint
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
  handlers/         # ConnectRPC service handlers
  chat/             # Chat service (AI streaming, tools)
  persona/          # Persona composition (24 locations x 15 themes)
  ai/               # AI provider abstraction (Claude, OpenAI)
  chatstore/        # Firestore chat persistence
  auth/             # Google OAuth + JWT
  trip/             # Trip service
  booking/          # Booking ingestion + AI parsing
  lifecycle/        # GDPR deletion, archival
  config/           # Environment config
  aitest/           # AI integration test harness
  dbgen/            # Generated sqlc code
proto/toqui/v1/     # Protobuf service definitions
gen/toqui/v1/       # Generated Go proto code
db/
  migrations/       # SQL migrations (golang-migrate)
  queries/          # sqlc query definitions
```

## Related Repos

- [toqui](https://github.com/gallowaysoftware/toqui) — Next.js frontend
- [toqui-site](https://github.com/gallowaysoftware/toqui-site) — Astro marketing site
