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

GitHub Actions on push to `main` and all PRs (GitHub-hosted runners, `ubuntu-latest`):

- **lint+typecheck**, **test**, **build** run in parallel → **deploy-staging** (main only, Cloud Run)
- **deploy-prod** — manual trigger via `workflow_dispatch` (requires main branch + `production` environment approval)

Staging deploy: Builds Docker image (with `NEXT_PUBLIC_API_URL=https://staging-api.toqui.travel`), pushes to Artifact Registry, deploys to Cloud Run via `gcloud run deploy`. Uses Workload Identity Federation (keyless GCP auth).

Prod deploy: Same pattern but with `NEXT_PUBLIC_API_URL=https://api.toqui.travel`, `toqui-prod` project, and separate WIF credentials (`GCP_PROD_WIF_PROVIDER`, `GCP_PROD_SERVICE_ACCOUNT`).

Staging URL: `https://staging-app.toqui.travel`
Prod URL: `https://app.toqui.travel`

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
- **AuthProvider** — Auth state, cookie-based token refresh via `POST /auth/refresh`, stores only user info in localStorage (no tokens)
- **GrpcProvider** — ConnectRPC transport with `credentials: "include"` (HttpOnly cookies), auto-retry on 401 via cookie refresh

## Auth Flow

1. User clicks "Sign in with Google" → navigates to backend `/auth/google/login`
2. Backend handles OAuth, sets temporary HttpOnly cookie, redirects to `/auth/callback`
3. Frontend `/auth/callback` page calls `POST /auth/exchange` (credentials: include)
4. Backend returns user info + `expires_at` in JSON body, sets `toqui_access` and `toqui_refresh` HttpOnly cookies, clears OAuth cookie
5. Frontend stores only user info in localStorage (`toqui_user`) — **no tokens in JavaScript**
6. All API calls use `credentials: "include"` — browser sends HttpOnly cookies automatically
7. Backend `cookieAuth` middleware reads the cookie and sets `Authorization: Bearer` header for handlers
8. Auto-refresh: frontend calls `POST /auth/refresh` 5 minutes before `expires_at` — backend rotates cookies
9. Logout: frontend calls `POST /auth/logout` — backend revokes refresh token and clears cookies

## Age Gate

DOB-based age verification in `src/components/auth/AgeGate.tsx`:

- Prompts for date of birth on first visit
- Validates age >= 18 with round-trip date validation (catches invalid dates like Feb 30)
- Stores verification in localStorage (`toqui_age_verified`)
- Exempts `/privacy` and `/terms` routes so users can read legal pages before verifying
- Shows denial screen for under-18 users

## Pre-Commit Requirements

### Documentation Updates

**MANDATORY**: Before every commit/push, update all relevant documentation:

1. **CLAUDE.md** — Update this file and any other repo CLAUDE.md files affected by the changes (architecture, deployment, security patterns, new components)
2. **MEMORY.md** — Update the shared memory file at `/Users/pequalsnp/.claude/projects/-Users-pequalsnp-src-github-com-pequalsnp-travelchat-backend/memory/MEMORY.md` with completed work, status changes, and any new patterns
3. **Cross-repo consistency** — If changes affect shared documentation topics (deployment, CI/CD, staging/prod status, security), update CLAUDE.md in ALL 4 repos

### Adversarial Review

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

## Security

### Auth Token Storage

Auth tokens are stored in HttpOnly cookies (`toqui_access`, `toqui_refresh`) set by the backend. Tokens are never accessible to JavaScript, eliminating the XSS token theft vector ([#57](https://github.com/gallowaysoftware/toqui-backend/issues/57) — fixed). localStorage stores only user display info (`toqui_user`) with no sensitive data.

Additional mitigations:
- Strict CSP headers set by the backend
- No `dangerouslySetInnerHTML` usage
- All user-generated content is escaped by React's default rendering
- CSRF protection via Origin/Referer validation on the backend
- SameSite=Lax cookies prevent cross-site request forgery

### Known Open Issues

See [GitHub Issues with `security` label](https://github.com/gallowaysoftware/toqui/issues?q=label:security) and [design issues](https://github.com/gallowaysoftware/toqui/issues?q=label:design).

Key security-relevant design gaps:
- Age gate is client-side only (#85 in backend repo) — can be bypassed via localStorage

### Security Checklist for New Components

1. **Never use `dangerouslySetInnerHTML`** — React's default escaping prevents XSS
2. **No inline event handlers from user data** — always use React event handlers
3. **Validate redirects** — never redirect to URLs from user input without validation
4. **Sanitize URL params** — `useSearchParams()` values are untrusted input
5. **No secrets in client code** — `NEXT_PUBLIC_*` env vars are embedded in the build

## Cross-Repo Consistency

**IMPORTANT**: This project spans 4 repos. When making changes that affect shared documentation (architecture, deployment, CI/CD, security patterns, staging/prod status), update CLAUDE.md in ALL repos to keep them consistent:

- `/Users/pequalsnp/src/github.com/gallowaysoftware/toqui-backend/CLAUDE.md`
- `/Users/pequalsnp/src/github.com/gallowaysoftware/toqui/CLAUDE.md` (this file)
- `/Users/pequalsnp/src/github.com/gallowaysoftware/toqui-terraform/CLAUDE.md`
- `/Users/pequalsnp/src/github.com/gallowaysoftware/toqui-site/CLAUDE.md`

Also update the shared memory file: `/Users/pequalsnp/.claude/projects/-Users-pequalsnp-src-github-com-pequalsnp-travelchat-backend/memory/MEMORY.md`

## Related Repos

- **toqui-backend** (`github.com/gallowaysoftware/toqui-backend`) — Go API server
- **toqui-terraform** (`github.com/gallowaysoftware/toqui-terraform`) — Terraform infrastructure
- **toqui-site** (`github.com/gallowaysoftware/toqui-site`) — Astro marketing site
