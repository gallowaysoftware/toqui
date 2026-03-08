# Toqui Frontend

AI-powered travel companion. Next.js 16 TypeScript frontend with ConnectRPC, Tailwind CSS v4, and React 19.

## Project Structure

This is a 4-repo project under `github.com/gallowaysoftware`:
- **toqui** (this repo) — Next.js TypeScript web frontend
- **toqui-backend** — Go backend, gRPC API, AI orchestration
- **toqui-terraform** — Terraform GCP infrastructure (staging + prod)
- **toqui-site** — Astro static marketing site

### Directory Layout

```
src/
  app/                    Next.js App Router pages
    auth/                 OAuth callback page
    companion/            Companion mode (active trip)
    privacy/              Privacy Policy (static)
    settings/             User settings
    shared/               Shared itinerary view
    terms/                Terms of Service (static)
    trips/                Trip management pages
    waitlist/             Waitlist page
    page.tsx              Landing page
    layout.tsx            Root layout (Inter font, theme init, providers)
  components/
    auth/                 AgeGate, auth UI
    booking/              Booking display components
    chat/                 Chat interface, message bubbles, typing indicator
    map/                  MapLibre GL map components
    providers/            App-wide providers (Theme, Auth, gRPC, QueryClient, AgeGate)
    pwa/                  Service worker registration
    theme/                Theme toggle (dark/light mode)
    trip/                 Trip cards, itinerary views
  gen/                    Generated protobuf TypeScript bindings (committed)
    toqui/v1/             Service + message types
    buf/validate/         Validation types
  hooks/                  Custom React hooks
  lib/                    Shared utilities (gRPC client, auth helpers)
  stores/                 Zustand state stores
```

## Development

```bash
pnpm install              # Install dependencies
pnpm dev                  # Dev server with Turbopack
pnpm build                # Production build
pnpm lint                 # ESLint
pnpm typecheck            # TypeScript type checking
pnpm test                 # Run tests (Vitest)
pnpm test:watch           # Watch mode
pnpm test:coverage        # Coverage report
pnpm generate             # Regenerate proto bindings from ../toqui-backend
```

### CI/CD

GitHub Actions on push to `main` and all PRs (self-hosted Linux runners):
- **lint** → **typecheck** → **test** → **build** → **deploy-staging** (main only)

Staging deploy: Builds Docker image, pushes to Artifact Registry, deploys to GCE VM via SSH. Uses Workload Identity Federation (keyless GCP auth).

## Conventions

- **Styling**: Tailwind CSS v4 with CSS custom properties for theming (`var(--color-*)`)
- **State management**: Zustand stores in `src/stores/`
- **Data fetching**: TanStack React Query with ConnectRPC transport
- **Proto types**: Import from `@/gen/toqui/v1/*_pb` (committed, regenerate with `pnpm generate`)
- **Testing**: Vitest + React Testing Library + jsdom
- **Components**: Functional components with TypeScript, `"use client"` directive for client components
- **Theme**: Dark/light mode via CSS class on `<html>`, persisted in localStorage (`toqui_theme`)

## Provider Stack

Providers wrap the entire app in `src/components/providers/Providers.tsx`:

```
ThemeProvider → AgeGate → QueryClientProvider → AuthProvider → GrpcProvider → {children}
```

- **ThemeProvider** — Dark/light mode, persists to localStorage
- **AgeGate** — DOB-based age verification (18+), exempts /privacy and /terms
- **QueryClientProvider** — TanStack React Query
- **AuthProvider** — JWT auth state, token refresh
- **GrpcProvider** — ConnectRPC transport configured with auth interceptor

## Auth Flow

1. User clicks "Sign in with Google" → navigates to backend `/auth/google/login`
2. Backend handles OAuth, sets HttpOnly cookie, redirects to `/auth/callback`
3. Frontend `/auth/callback` page calls `POST /auth/exchange` (credentials: include)
4. Backend returns tokens in JSON body, clears cookie
5. Frontend stores tokens in memory, uses `Authorization: Bearer` for all API calls

## Age Gate

DOB-based age verification in `src/components/auth/AgeGate.tsx`:
- Prompts for date of birth on first visit
- Validates age >= 18 with round-trip date validation (catches invalid dates like Feb 30)
- Stores verification in localStorage (`toqui_age_verified`)
- Exempts `/privacy` and `/terms` routes so users can read legal pages before verifying
- Shows denial screen for under-18 users

## Pre-Commit Adversarial Review

**MANDATORY**: Before every commit, spawn a parallel adversarial review agent to audit all staged changes. This catches bugs, security issues, and logic errors before they reach the repo.

### How It Works

1. After all implementation and tests are passing, spawn a `general-purpose` Task agent with a prompt like:

   > You are an adversarial code reviewer. Your job is to find bugs, security issues, logic errors, and missing edge cases. Review all changes in these files: [list files]. For each issue found, classify as BLOCKING (must fix before commit) or WARNING (note but can ship). Be thorough and skeptical.

2. The agent reviews all changed files and returns findings classified as:
   - **BLOCKING** — Must fix before commit (bugs, security holes, logic errors, missing validation)
   - **WARNING** — Worth noting but acceptable to ship (style, minor improvements, future work)

3. Fix all BLOCKING issues, then re-run the adversarial review to verify fixes pass.

4. Only commit after the adversarial review returns zero BLOCKING issues.

### What to Review

- All new files and modified files in the changeset
- Test coverage — are edge cases tested?
- Security — XSS, injection, auth bypass, sensitive data exposure
- React patterns — proper hook dependencies, memoization, client vs server components
- Accessibility — ARIA labels, keyboard navigation, semantic HTML
- Type safety — proper TypeScript types, no `any` unless justified
- Date/time handling — timezone issues, invalid date coercion

## Related Repos

- **toqui-backend** (`github.com/gallowaysoftware/toqui-backend`) — Go API server
- **toqui-terraform** (`github.com/gallowaysoftware/toqui-terraform`) — Terraform infrastructure
- **toqui-site** (`github.com/gallowaysoftware/toqui-site`) — Astro marketing site
