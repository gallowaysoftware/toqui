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

| Variable                  | Required | Default                                                       | Description                                 |
| ------------------------- | -------- | ------------------------------------------------------------- | ------------------------------------------- |
| `TARGET_ENV`              | No       | `local`                                                       | Environment: `local`, `staging`, `prod`     |
| `GOOGLE_CLIENT_ID`        | Yes      | —                                                             | Google OAuth client ID                      |
| `GOOGLE_CLIENT_SECRET`    | Yes      | —                                                             | Google OAuth client secret                  |
| `ANTHROPIC_API_KEY`       | Yes\*    | —                                                             | Claude API key (primary AI provider)        |
| `VERTEX_AI_PROJECT_ID`    | Yes\*    | —                                                             | GCP project for Vertex AI Gemini (fallback) |
| `VERTEX_AI_LOCATION`      | No       | `us-central1`                                                 | Vertex AI region                            |
| `DAILY_AI_TOKEN_BUDGET`   | No       | `0`                                                           | Max total AI tokens/day (0 = unlimited)     |
| `DATABASE_URL`            | No       | `postgres://toqui:toqui@localhost:5432/toqui?sslmode=disable` | PostgreSQL connection                       |
| `PORT`                    | No       | `8090`                                                        | Server port                                 |
| `JWT_SECRET`              | No       | dev default                                                   | JWT signing secret                          |
| `FIRESTORE_PROJECT_ID`    | No       | `toqui-dev`                                                   | Firestore project                           |
| `FIRESTORE_EMULATOR_HOST` | No       | —                                                             | Firestore emulator address                  |
| `FRONTEND_URL`            | No       | `http://localhost:3000`                                       | CORS origin                                 |

\*At least one AI provider is required. If `ANTHROPIC_API_KEY` is set, Claude is used. Otherwise, Gemini via Vertex AI is used (requires `gcloud auth application-default login` and `VERTEX_AI_PROJECT_ID` or `FIRESTORE_PROJECT_ID`).

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

End-to-end tests that exercise the full trip lifecycle through the AI with real LLM calls. Requires Docker services running and an AI provider (`ANTHROPIC_API_KEY` for Claude, or `VERTEX_AI_PROJECT_ID` for Gemini via Vertex AI).

```bash
docker compose up -d    # Start Postgres + Firestore emulator
make ai-test            # Run 7 regression scenarios
make ai-test-generative # Run regression + LLM-generated exploratory scenarios
```

Run a specific scenario:

```bash
go test -tags=aitest -v -timeout=30m ./internal/aitest/... -run TestAIScenarios/alice
```

Reports are written to `testdata/aitest-reports/`.

## CI/CD

GitHub Actions runs on push to `main` and all PRs (self-hosted Linux runners):

1. **Build & Test** — `go build`, `go vet`, `go test` with coverage
2. **Deploy to Staging** (main only) — build Docker image → push to Artifact Registry → redeploy GCE VM → run migrations

Uses **Workload Identity Federation** for keyless GCP auth — no service account keys.

### Deploying to Staging

**Automatic**: Push to `main`. That's it. GitHub Actions handles everything.

**Manual deploy** (if CI is broken):

```bash
IMAGE=us-central1-docker.pkg.dev/toqui-staging/toqui-backend/toqui-backend

# Build and push
docker build --platform linux/amd64 -t $IMAGE:latest .
docker push $IMAGE:latest

# Redeploy on the VM
gcloud compute instances update-container toqui-staging-vm \
  --zone=us-central1-a --project=toqui-staging --container-image=$IMAGE:latest

# Run migrations
DB_URL=$(gcloud secrets versions access latest --secret=staging-database-url --project=toqui-staging)
gcloud compute ssh toqui-staging-vm \
  --project=toqui-staging --zone=us-central1-a --tunnel-through-iap \
  -- "docker exec -e DATABASE_URL='${DB_URL}' \$(docker ps -q --filter name=klt) /migrate -direction up"
```

### Rolling Back

```bash
IMAGE=us-central1-docker.pkg.dev/toqui-staging/toqui-backend/toqui-backend

# List available image tags
gcloud artifacts docker tags list \
  us-central1-docker.pkg.dev/toqui-staging/toqui-backend/toqui-backend \
  --project=toqui-staging

# Redeploy to a previous version
gcloud compute instances update-container toqui-staging-vm \
  --zone=us-central1-a --project=toqui-staging \
  --container-image=$IMAGE:<previous-sha>

# Roll back one database migration
DB_URL=$(gcloud secrets versions access latest --secret=staging-database-url --project=toqui-staging)
gcloud compute ssh toqui-staging-vm \
  --project=toqui-staging --zone=us-central1-a --tunnel-through-iap \
  -- "docker exec -e DATABASE_URL='${DB_URL}' \$(docker ps -q --filter name=klt) /migrate -direction down -steps 1"
```

### Database Migrations

```bash
# Create a new migration
make migrate-create
# Produces: db/migrations/YYYYMMDDHHMMSS_name.up.sql + .down.sql

# Apply locally
make migrate-up

# Rollback locally
make migrate-down

# Run on staging
DB_URL=$(gcloud secrets versions access latest --secret=staging-database-url --project=toqui-staging)
gcloud compute ssh toqui-staging-vm \
  --project=toqui-staging --zone=us-central1-a --tunnel-through-iap \
  -- "docker exec -e DATABASE_URL='${DB_URL}' \$(docker ps -q --filter name=klt) /migrate -direction up"
```

**Note**: The `cmd/migrate` binary auto-detects migration files at `/migrations` (Docker) or `db/migrations/` (local).

### Checking Logs

```bash
# App container logs on staging
gcloud compute ssh toqui-staging-vm \
  --project=toqui-staging --zone=us-central1-a --tunnel-through-iap \
  -- "docker logs --tail 100 -f \$(docker ps -q --filter name=klt)"

# Container status
gcloud compute ssh toqui-staging-vm \
  --project=toqui-staging --zone=us-central1-a --tunnel-through-iap \
  -- "docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Image}}'"

# Cloud Logging
gcloud logging read 'resource.type="gce_instance"' --project=toqui-staging --limit=50
```

### Accessing Staging

Staging is behind Tailscale VPN at `toqui-staging:8090`. No public IP.

```bash
# API request (requires Tailscale connected)
curl http://toqui-staging:8090/toqui.v1.TripService/ListTrips

# SSH via GCP IAP tunnel (no Tailscale needed)
gcloud compute ssh toqui-staging-vm --project=toqui-staging --zone=us-central1-a --tunnel-through-iap
```

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
  -e FIRESTORE_EMULATOR_HOST=host.docker.internal:8080 \
  toqui-backend
```

## Project Structure

```
cmd/
  server/           # API server entry point
  migrate/          # Database migration runner
internal/
  handlers/         # ConnectRPC service handlers (auth, trip, chat, booking, location, persona)
  chat/             # Chat service — AI streaming, tool execution, persona resolution
  persona/          # Persona composition (24 locations × 15 themes)
  ai/               # AI provider abstraction (Claude, Gemini/Vertex AI)
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
