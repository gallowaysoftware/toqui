# Self-hosting Toqui

Toqui is a single-tenant app: one operator, one (or a small group of)
user(s) sharing the same backend AI key. There's no multi-tenant
isolation, no per-user billing, and no admin panel — by design.

This guide covers three deploy patterns. Pick whichever fits your setup.

- [Pattern 1: docker-compose on a single host](#pattern-1-docker-compose-on-a-single-host) — easiest, runs everything in one place
- [Pattern 2: Fly.io](#pattern-2-flyio) — managed Postgres + auto-TLS, ~$5–15/mo
- [Pattern 3: Render](#pattern-3-render) — similar to Fly with a different price/feature mix

All three need the same env vars (see [`.env.example`](.env.example)).
At minimum you need:

- `JWT_SECRET` — generate with `openssl rand -hex 32`
- One AI provider key: `ANTHROPIC_API_KEY`, `OPENAI_API_KEY` (with
  optional `OPENAI_BASE_URL` to point at OpenRouter/Ollama/LM Studio/etc.),
  or Vertex AI credentials.

Google OAuth is **optional**. Without `GOOGLE_CLIENT_ID` +
`GOOGLE_CLIENT_SECRET`, the frontend hides the Google button and users
sign in with email + password only.

---

## Pattern 1: docker-compose on a single host

The repo ships with a top-level `docker-compose.yml` that runs the
backend, the Expo web frontend, Postgres, and a Firestore emulator.

```bash
git clone https://github.com/gallowaysoftware/toqui
cd toqui
cp .env.example .env
$EDITOR .env        # set JWT_SECRET + one AI provider key
docker compose up -d --build
```

That's it. Frontend on `http://localhost:3000`, backend on
`http://localhost:8090`.

### What's running

| Service | Image | Notes |
|---|---|---|
| `frontend` | built from `./Dockerfile` (multi-stage; Expo web → nginx) | port 3000 |
| `backend` | built from `./backend/Dockerfile` (distroless static Go) | port 8090 |
| `postgres` | `postgis/postgis:16-3.4` | data in named volume `postgres_data` |
| `firestore` | Google Cloud SDK emulator | chat history; **emulator data is ephemeral** |
| `migrate` | one-shot run of `./backend/cmd/migrate` | applies SQL migrations on startup |

### Pointing at a real Firestore (recommended for production)

The bundled Firestore emulator is fine for trying it out, but it loses
all chat history on restart. For production, point the backend at a
managed Firestore database:

1. Create a Firestore database in any GCP project.
2. Create a service account with `roles/datastore.user`, download its
   JSON key.
3. Mount the key into the backend container and set
   `GOOGLE_APPLICATION_CREDENTIALS`. Then unset
   `FIRESTORE_EMULATOR_HOST` so the SDK uses the real endpoint.

A drop-in compose override:

```yaml
# docker-compose.override.yml
services:
  backend:
    environment:
      FIRESTORE_EMULATOR_HOST: ""
      FIRESTORE_PROJECT_ID: "your-gcp-project-id"
      GOOGLE_APPLICATION_CREDENTIALS: "/run/secrets/gcp-sa.json"
    volumes:
      - ./secrets/gcp-sa.json:/run/secrets/gcp-sa.json:ro
  firestore:
    profiles: ["never"]   # don't start the emulator
```

### TLS

For internet-facing deploys, put Caddy or Traefik in front of the
frontend container and let it manage Let's Encrypt certs. Don't
expose Postgres or the backend directly.

---

## Pattern 2: Fly.io

Fly works well because:
- Fly Postgres is managed (regular backups, no operational toil)
- Fly Machines give you auto-TLS + global edge for cheap
- Two Machines (backend + frontend) cost ~$5–15/mo at low traffic

```bash
fly launch --no-deploy --name your-toqui-backend --image . \
  --dockerfile backend/Dockerfile
fly postgres create --name your-toqui-db
fly postgres attach your-toqui-db --app your-toqui-backend

# Set required secrets on the backend:
fly secrets set --app your-toqui-backend \
  JWT_SECRET="$(openssl rand -hex 32)" \
  ANTHROPIC_API_KEY="sk-ant-..."

fly deploy --app your-toqui-backend
```

Frontend is a separate app:

```bash
fly launch --no-deploy --name your-toqui-frontend --dockerfile Dockerfile
fly secrets set --app your-toqui-frontend \
  EXPO_PUBLIC_API_URL="https://your-toqui-backend.fly.dev"
fly deploy --app your-toqui-frontend
```

Firestore: see "real Firestore" section above. Fly doesn't have a
managed equivalent; you'll plug in a GCP Firestore + ADC.

---

## Pattern 3: Render

```
Backend: New > Web Service > "Build & deploy from a Git repository"
  - Root directory: backend/
  - Dockerfile path: ./Dockerfile
  - Add managed PostgreSQL, copy DATABASE_URL into env vars
  - Set JWT_SECRET + one AI key

Frontend: New > Web Service > same repo
  - Root directory: (leave empty)
  - Dockerfile path: ./Dockerfile
  - Set EXPO_PUBLIC_API_URL to the backend service URL
```

Same Firestore note as Fly.

---

## Migrations

Migrations live in `backend/db/migrations/` and run via a separate
binary in the same image: `backend/cmd/migrate`.

The docker-compose flow runs `migrate up` once at startup
(`migrate` service depends on `postgres`, `backend` depends on
`migrate` completing). For Fly / Render, you can either:

- Run `fly ssh console --app your-toqui-backend -C "/migrate -dir /migrations -db $DATABASE_URL up"` once after each deploy, or
- Add a release command that runs the migrate binary.

---

## Updating

When the upstream repo ships a new version:

```bash
git pull
docker compose up -d --build
```

`docker compose up` rebuilds images and rolls services. Migrations run
automatically via the `migrate` one-shot service.

---

## Backups

Two databases hold user data:

- **Postgres** — users, trips, bookings, itinerary, refresh tokens.
  `pg_dump` it however you'd back up any Postgres database. For
  docker-compose: `docker compose exec postgres pg_dump -U toqui toqui > backup.sql`
- **Firestore** — chat history. Use `gcloud firestore export` for the
  managed-Firestore case. The emulator's data is not meant to survive
  restarts; back it up by `cp`'ing the emulator's volume if you really
  need to.

---

## Privacy defaults

The AGPL build has no third-party telemetry: no PostHog, no Sentry,
no Google Analytics. Errors go to stdout/stderr; logs are yours.
The AI provider you choose is the only third party that sees user
content (per their own terms).

If you re-add analytics, the AGPL obligates you to publish those
modifications. That's the point — the privacy story is enforceable
downstream.
