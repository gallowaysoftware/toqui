# Toqui

Next.js frontend for Toqui, an AI-powered travel companion. Built with Next.js 16, TypeScript, Tailwind CSS, and ConnectRPC.

## Prerequisites

- Node.js 22+
- [pnpm](https://pnpm.io/)
- [toqui-backend](https://github.com/gallowaysoftware/toqui-backend) cloned at `../toqui-backend` (for proto generation)

## Quick Start

```bash
# 1. Install dependencies
pnpm install

# 2. Generate proto TypeScript bindings (requires ../toqui-backend)
pnpm generate

# 3. Start dev server
pnpm dev
# App starts on http://localhost:3000
```

The backend must be running at `http://localhost:8090` for API calls to work. See the [backend README](https://github.com/gallowaysoftware/toqui-backend) for setup.

## Environment Variables

| Variable              | Default                 | Description     |
| --------------------- | ----------------------- | --------------- |
| `NEXT_PUBLIC_API_URL` | `http://localhost:8090` | Backend API URL |

## Scripts

```bash
pnpm dev            # Start dev server (Turbopack)
pnpm build          # Production build
pnpm start          # Start production server
pnpm lint           # Run ESLint
pnpm format         # Run Prettier
pnpm generate       # Generate TypeScript proto bindings from backend
```

## Proto Generation

This repo generates its own TypeScript proto bindings from the backend's proto definitions using [buf](https://buf.build/). The backend repo must be cloned next to this one:

```
gallowaysoftware/
  toqui-backend/    # Proto definitions live here
  toqui/            # This repo — generates TS bindings via buf
```

Run `pnpm generate` after any proto changes in the backend. The generated code goes to `src/gen/` (gitignored).

## Project Structure

```
src/
  app/              # Next.js app router pages
  components/
    providers/      # Auth, gRPC, React Query providers
    trip/           # Trip-related components
    booking/        # Booking components
  lib/
    hooks/          # useChat, useTrips custom hooks
  gen/              # Generated proto TS bindings (gitignored)
```

## Related Repos

- [toqui-backend](https://github.com/gallowaysoftware/toqui-backend) — Go backend
- [toqui-site](https://github.com/gallowaysoftware/toqui-site) — Astro marketing site
