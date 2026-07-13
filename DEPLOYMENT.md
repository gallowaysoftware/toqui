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
- One AI provider key: `GEMINI_API_KEY` (the default primary),
  `ANTHROPIC_API_KEY`, `OPENAI_API_KEY` (with optional `OPENAI_BASE_URL`
  to point at OpenRouter/Ollama/llama-swap/LM Studio/etc. — set
  `AI_PROVIDER=openai` and the `OPENAI_MODEL_FAST/SMART/BEST` tier names
  too), or Vertex AI credentials.

If the backend is reachable from the internet, also set
`ALLOWED_EMAIL_DOMAINS` — registration is otherwise open to anyone,
and every account chats on your AI key.

Google OAuth is **optional**. Without `GOOGLE_CLIENT_ID` +
`GOOGLE_CLIENT_SECRET`, the frontend hides the Google button and users
sign in with email + password only.

---

## Pattern 1: docker-compose on a single host

The repo ships with a top-level `docker-compose.yml` that runs the
backend, the Expo web frontend, and Postgres.

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
| `postgres` | `postgis/postgis:16-3.4` | ALL data (trips, bookings, chat) in named volume `postgres_data` |
| `migrate` | one-shot run of `./backend/cmd/migrate` | applies SQL migrations on startup |

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

---

## Migrations

Migrations live in `backend/db/migrations/` and run via a separate
binary in the same image: `backend/cmd/migrate`.

The docker-compose flow runs `migrate up` once at startup
(`migrate` service depends on `postgres`, `backend` depends on
`migrate` completing). For Fly / Render, you can either:

- Run `fly ssh console --app your-toqui-backend -C "/migrate -direction up"`
  once after each deploy (the binary reads `DATABASE_URL` from the app
  environment and auto-detects `/migrations` inside the image), or
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

> **Upgrading from a pre-Postgres-chat version:** chat history used to
> live in Firestore (the bundled emulator kept it in memory only). There
> is no automated migration — chat now starts fresh in Postgres. Trips,
> bookings, and itineraries are unaffected.

---

## Optional: email booking import

Let users forward booking confirmations to a mailbox and have Toqui import
them automatically. Point the backend at a **dedicated** IMAP mailbox — the
poller processes every message in it, matches each to the Toqui account
whose email is the sender (`From:`), and ingests it as a booking (same
AI parse + itinerary auto-link as the in-app paste flow).

Set these on the backend (see [`.env.example`](.env.example)); the poller
is enabled only when host, username, and password are all present:

```
IMAP_HOST=imap.example.com
IMAP_PORT=993
IMAP_USERNAME=bookings@example.com
IMAP_PASSWORD=...
IMAP_MAILBOX=INBOX          # default INBOX
IMAP_POLL_INTERVAL=60s      # default 60s
IMAP_TLS=true               # default true
```

Notes:
- **Matching is by the `From` address**, which — when a user forwards a
  confirmation — is the user's own address (mail clients set `From` to the
  forwarder; the original airline/hotel email is quoted in the body, which
  is what gets parsed). So the user simply forwards from the email account
  they signed up with. A message whose `From` has no Toqui account is
  skipped (and marked read so it isn't retried).
- Messages are marked `\Seen` after processing, so only unread messages are
  ever picked up. Use a mailbox reserved for this.
- A common setup is a plus-address or alias (e.g. `bookings+you@example.com`)
  that users forward to, filtered into a dedicated folder — set
  `IMAP_MAILBOX` to that folder.

---

## Backups

Postgres holds ALL user data — users, trips, bookings, itinerary,
refresh tokens, and chat history. `pg_dump` it however you'd back up
any Postgres database. For docker-compose:

```bash
docker compose exec postgres pg_dump -U toqui toqui > backup.sql
```

---

## Privacy defaults

The AGPL build has no third-party telemetry: no PostHog, no Sentry,
no Google Analytics. Errors go to stdout/stderr; logs are yours.
The AI provider you choose is the only third party that sees user
content (per their own terms).

If you re-add analytics, the AGPL obligates you to publish those
modifications. That's the point — the privacy story is enforceable
downstream.
