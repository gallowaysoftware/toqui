# Toqui

Self-hostable AI travel companion. Plan trips with destination-aware
AI experts. Bring your own AI provider API key.

Released under [AGPL-3.0-or-later](./LICENSE) — Copyright (C) 2026
Galloway Software Solutions Inc.

## Layout

- **`/`** — Expo React Native app. Web + iOS + Android from one codebase. See [CLAUDE.md](CLAUDE.md).
- **`/backend/`** — Go API (ConnectRPC, PostgreSQL + PostGIS, Firestore for chat). See [backend/CLAUDE.md](backend/CLAUDE.md).

A separate transition page lives at [toqui.travel](https://toqui.travel)
([gallowaysoftware/toqui-site](https://github.com/gallowaysoftware/toqui-site)).

## Quickstart (self-host)

```bash
git clone https://github.com/gallowaysoftware/toqui
cd toqui
cp .env.example .env
$EDITOR .env                  # set JWT_SECRET + one AI provider key
docker compose up -d --build  # frontend on :3000, backend on :8090
```

That's it — see [DEPLOYMENT.md](DEPLOYMENT.md) for Fly.io / Render / production notes.

## Quickstart (development)

Run the two halves separately for fast iteration:

```bash
# Shell 1 — backend (Postgres + Firestore emulator + Go server)
cd backend
docker compose up -d postgres firestore
make migrate-up
make run

# Shell 2 — frontend (Expo, with hot reload)
pnpm install
pnpm web
```

Frontend defaults to `http://localhost:8090` for the backend.

## License

[AGPL-3.0-or-later](./LICENSE). Anyone who hosts a modified version
must publish their changes — keeping the privacy story enforceable
downstream.
