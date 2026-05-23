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

## Quickstart

Run both halves in two shells:

```bash
# Shell 1 — backend
cd backend
docker compose up -d postgres firestore
make migrate-up
make run

# Shell 2 — frontend (web)
pnpm install
pnpm web
```

Default frontend points at `http://localhost:8090` for the backend.

## License

[AGPL-3.0-or-later](./LICENSE). Anyone who hosts a modified version
must publish their changes — keeping the privacy story enforceable
downstream.
