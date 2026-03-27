# Toqui

AI-powered travel companion. React Native (Expo) cross-platform app targeting web, iOS, and Android from a single codebase. Uses ConnectRPC, TypeScript, and Expo Router.

## Project Structure

This is a 4-repo project under `github.com/gallowaysoftware`:

- **toqui** (this repo) — Expo React Native app (web + iOS + Android)
- **toqui-backend** — Go backend, gRPC API, AI orchestration
- **toqui-terraform** — Terraform GCP infrastructure (staging + prod)
- **toqui-site** — Astro static marketing site

### Directory Layout

```
app/                          Expo Router pages (file-based routing)
  (tabs)/                     Tab navigator (Trips, Companion, Settings)
    _layout.tsx               Tab bar configuration
    index.tsx                 Trips list (home screen)
    companion.tsx             Travel companion chat
    settings.tsx              User settings
  trips/
    new.tsx                   Create new trip
    [tripId]/
      _layout.tsx             Trip detail stack navigator
      index.tsx               Trip overview
      chat.tsx                AI chat for trip planning
      bookings.tsx            Booking management
      settings.tsx            Trip settings
  auth/callback.tsx           OAuth callback handler
  shared/[token]/index.tsx    Public shared trip view
  privacy.tsx                 Privacy policy
  terms.tsx                   Terms of service
  waitlist.tsx                Waitlist page
  _layout.tsx                 Root layout (providers)
lib/                          Shared utilities
  auth.tsx                    Auth provider (SecureStore + Bearer tokens)
  transport.tsx               ConnectRPC transport with Bearer auth interceptor
  i18n.tsx                    i18next configuration
src/gen/                      Generated protobuf TypeScript bindings (committed)
  toqui/v1/                   Service + message types
  buf/validate/               Validation types
messages/                     i18n translation files
  en.json                     English translations
assets/                       App icons and splash screen
```

## Development

```bash
pnpm install              # Install dependencies
pnpm start                # Expo dev server (all platforms)
pnpm web                  # Web only
pnpm ios                  # iOS simulator
pnpm android              # Android emulator
pnpm build:web            # Production web bundle
pnpm build:ios            # Production iOS bundle
pnpm build:android        # Production Android bundle
pnpm typecheck            # TypeScript type checking
pnpm generate             # Regenerate proto bindings from ../toqui-backend
```

### Local Backend

The app connects to the backend API via `EXPO_PUBLIC_API_URL` (default: `http://localhost:8090`).

To run the backend locally:
```bash
cd ../toqui-backend
docker compose up -d postgres firestore   # Start Postgres + Firestore emulator
make migrate-up                            # Run migrations
CORS_ALLOWED_ORIGINS="http://localhost:3000,http://localhost:8081" make run
```

## Tech Stack

| Concern | Technology |
|---------|-----------|
| Framework | Expo SDK 55, Expo Router v55 (file-based routing) |
| Language | TypeScript, React 19, React Native 0.83 |
| API | ConnectRPC (`@connectrpc/connect-web`) with Bearer token auth |
| Auth | `expo-secure-store` (native) / `localStorage` (web) for JWT tokens |
| State | React Context (auth, transport), TanStack Query (planned) |
| i18n | `i18next` + `react-i18next` |
| Icons | `lucide-react-native` |
| Proto codegen | `@bufbuild/protoc-gen-es` (platform-agnostic TypeScript) |

## Auth Flow (Bearer Token)

1. User authenticates via Google OAuth (`expo-auth-session`)
2. App sends auth code to backend's `AuthService.GoogleLogin` RPC
3. Backend returns `{ access_token, refresh_token, user }` in response body
4. Tokens stored in `expo-secure-store` (native) or `localStorage` (web)
5. `TransportProvider` interceptor attaches `Authorization: Bearer <token>` to every ConnectRPC request
6. On 401, interceptor calls `AuthService.RefreshToken` RPC, stores new tokens, retries
7. Logout clears stored tokens

**No cookies are used.** The backend's `cookieAuth` middleware is bypassed — the auth interceptor reads the Bearer header directly.

## Conventions

- **Routing**: Expo Router file-based routing in `app/` directory
- **State management**: React Context for auth/transport, TanStack Query for server state (planned)
- **Proto types**: Import from `@gen/toqui/v1/*_pb` (committed, regenerate with `pnpm generate`)
- **Components**: Functional components with TypeScript, React Native primitives
- **Styling**: `StyleSheet.create` (NativeWind planned for Tailwind-like classes)

## Provider Stack

Providers wrap the entire app in `app/_layout.tsx`:

```
I18nProvider → AuthProvider → TransportProvider → {children}
```

- **I18nProvider** — i18next initialization with English translations
- **AuthProvider** — JWT token management, SecureStore/localStorage persistence
- **TransportProvider** — ConnectRPC transport with Bearer auth interceptor + auto-refresh on 401

## Security

### Auth Token Storage

- **Native (iOS/Android):** Tokens in `expo-secure-store` (Keychain/Keystore)
- **Web:** Tokens in `localStorage` (same as before migration — HttpOnly cookies were web-only)
- No tokens in URL params or query strings
- Auto-refresh before expiry via `AuthService.RefreshToken` RPC

## Pre-Commit Requirements

### Adversarial Review

**MANDATORY**: Before every commit, spawn a parallel adversarial review agent to audit all staged changes.

## Cross-Repo Consistency

**IMPORTANT**: This project spans 4 repos. When making changes that affect shared documentation, update CLAUDE.md in ALL repos:

- `/Users/pequalsnp/src/github.com/gallowaysoftware/toqui-backend/CLAUDE.md`
- `/Users/pequalsnp/src/github.com/gallowaysoftware/toqui/CLAUDE.md` (this file)
- `/Users/pequalsnp/src/github.com/gallowaysoftware/toqui-terraform/CLAUDE.md`
- `/Users/pequalsnp/src/github.com/gallowaysoftware/toqui-site/CLAUDE.md`

## Related Repos

- **toqui-backend** (`github.com/gallowaysoftware/toqui-backend`) — Go API server
- **toqui-terraform** (`github.com/gallowaysoftware/toqui-terraform`) — Terraform infrastructure
- **toqui-site** (`github.com/gallowaysoftware/toqui-site`) — Astro marketing site
