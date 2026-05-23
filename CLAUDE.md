# Toqui

AI-powered travel companion. React Native (Expo) cross-platform app targeting web, iOS, and Android from a single codebase. Uses ConnectRPC, TypeScript, and Expo Router.

## Core Principles

### User Privacy — Non-Negotiable

Toqui exists to help travelers, not to exploit them. These rules are absolute and override any business or feature consideration:

**Data Collection:**
- Collect only what's needed to deliver the feature. If in doubt, don't collect it.
- Travel data is inherently sensitive — destinations can reveal religion, health conditions, sexuality, and political activity. Treat ALL travel data as potentially sensitive under GDPR Article 9.
- Never log, track, or store destination names, chat content, specific travel dates, hotel/flight names, or booking details in analytics. Track counts and categories, never content.
- Pseudonymize user IDs in any analytics or logging pipeline.

**Compliance:**
- Comply with EU GDPR as the baseline for ALL users, regardless of their location. Do not maintain separate privacy standards by region.
- Comply with Canadian PIPEDA. As a Canadian company, PIPEDA applies to all commercial activities.
- Apple App Tracking Transparency: use only first-party analytics. Never track users across apps or websites.
- Cookie-less analytics only. No tracking cookies, no fingerprinting, no IDFA/GAID collection.

**Monetization Ethics:**
- Affiliate revenue is acceptable and must be transparently disclosed (see toqui.travel/affiliate-disclosure).
- Never bias AI recommendations for revenue. The AI recommends what's best for the traveler, not what pays us the most.
- Never sell, share, or broker user data to third parties. Period.
- Never serve display advertising that tracks users.
- Sponsored/promoted placements, if ever introduced, must be clearly labeled and must not degrade recommendation quality.

**Analytics:**
- Session replay must mask all text inputs, chat content, and itinerary details.
- Analytics events track behavior patterns (user created a trip), never content (user planned a trip to Mecca).
- Self-hosted or EU-hosted analytics only. Google Analytics is explicitly prohibited (ruled non-compliant by multiple EU DPAs).
- Users must be informed about analytics in the privacy policy with a clear opt-out mechanism for EU users.

**Data Lifecycle:**
- GDPR Article 17 (right to deletion) and Article 20 (data portability) are implemented and must remain functional.
- Trip data is archived after 90 days of completion and eventually purged.
- Account deletion must be complete — no shadow profiles, no retained analytics, no "soft delete" that keeps data.

These principles are not aspirational. They are engineering requirements. Code that violates them must not be merged.

## Project Structure

Monorepo with two top-level apps:

- **`/` (Expo React Native app)** — web + iOS + Android frontend. Most of this CLAUDE.md.
- **`/backend/` (Go API)** — ConnectRPC service. See [`backend/CLAUDE.md`](backend/CLAUDE.md) for backend-specific architecture and dev commands.

The third repo in the project, **toqui-site**, is the transition page at `toqui.travel` (separate Astro site, separate Cloudflare Pages deploy).

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
    invite.tsx                Accept collaboration invite
    [tripId]/
      _layout.tsx             Trip detail stack navigator
      index.tsx               Trip overview
      chat.tsx                AI chat for trip planning
      bookings.tsx            Booking management
      settings.tsx            Trip settings
  auth/
    callback.tsx              Google OAuth callback handler
    email-login.tsx           Email + password sign-in screen
    email-register.tsx        Email + password registration screen
  shared/[token]/index.tsx    Public shared trip view
  onboarding.tsx              Onboarding flow
  privacy.tsx                 Privacy policy
  terms.tsx                   Terms of service
  _layout.tsx                 Root layout (providers)
components/                   Shared UI components
  DatePicker.tsx              Date picker component
  ErrorBoundary.tsx           React error boundary with console logging
  LocationPermission.tsx      Location permission request flow
  OfflineBanner.tsx           Network status banner (offline/reconnecting)
  ShareButton.tsx             Native/web share sheet integration
  auth/
    AIDisclaimerGate.tsx      One-time AI disclaimer acknowledgement modal
  chat/
    ChatInput.tsx             Message input with send button and typing state
    FollowUpSuggestions.tsx   AI-generated follow-up question chips
    MessageBubble.tsx         Single chat message (user/AI/tool-result variants)
    PersonaIntroCard.tsx      Persona introduction card on switch
    SharePromptCard.tsx       Prompt to share trip with collaborators
    SuggestionChips.tsx       Quick suggestion chips in chat
    TypingIndicator.tsx       Animated typing indicator while AI responds
  feedback/
    FeedbackModal.tsx         User feedback submission modal
  itinerary/
    ItineraryTimeline.tsx     Day-by-day itinerary timeline component
  map/
    ItineraryMap.tsx          Interactive map showing itinerary locations
  share/
    ShareNudgeBanner.tsx      Contextual nudge to share trip
  trips/
    TemplateBrowser.tsx       Browse and select trip templates
  weather/
    WeatherCard.tsx           Current weather for trip destination (Open-Meteo)
lib/                          Shared utilities
  auth.tsx                    Auth provider (SecureStore/localStorage + Bearer tokens)
  transport.tsx               ConnectRPC transport with Bearer auth interceptor
  i18n.tsx                    i18next configuration
  theme.tsx                   Light/dark/system theme with ThemeColors interface
  google-auth.ts              useGoogleAuth() hook — expo-auth-session PKCE wrapper
  authFetch.ts                Bearer-auth fetch wrapper for REST endpoints (checkout, referral)
  config.ts                   Runtime config (EXPO_PUBLIC_* env vars)
  hooks/
    useTrips.ts               Trip CRUD via ConnectRPC TripService
    useChat.ts                SSE streaming chat — tool activity, persona switches
    useBookings.ts            Booking CRUD via ConnectRPC BookingService
    useItinerary.ts           Itinerary fetch via ConnectRPC TripService
    useFeedback.ts            Submit user feedback via REST
    useDestinationGuide.ts    Fetch destination guides via REST
    useLocation.ts            Device location permission + tracking
    useWeather.ts             Current weather for trip destination (Open-Meteo)
    useCollaborators.ts       Trip collaborator management
    useOnboarding.ts          Onboarding flow state
    useNetworkStatus.ts       Online/offline detection, reconnection handling
    useAuthProviders.ts       Fetch server's enabled auth providers (email+password always; Google env-gated)
  data/
    tripTemplates.ts          Trip template data for onboarding
  export/
    pdf-export.ts             HTML itinerary → PDF (expo-print native, window.print web)
    calendar-export.ts        ICS calendar export (expo-file-system native, blob download web)
src/gen/                      Generated protobuf TypeScript bindings (committed)
  toqui/v1/                   Service + message types
  buf/validate/               Validation types
messages/                     i18n translation files
  en.json                     English translations
assets/                       App icons and splash screen
docs/
  strategy/                   Product strategy and planning docs
fastlane/                     iOS build automation (Fastlane config)
metro.config.js               Metro bundler config (custom resolvers, patches)
patches/                      patch-package patches (Xcode 16.4 compat, etc.)
plugins/                      Expo config plugins (custom native module tweaks)
tests/                        Test utilities and shared test helpers
store-metadata.md             App Store / Play Store listing metadata
IOS_SUBMISSION_GUIDE.md       Step-by-step iOS App Store submission guide
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
pnpm build:ios:dev        # EAS Build iOS (development profile)
pnpm build:ios:preview    # EAS Build iOS (preview profile)
pnpm build:ios:prod       # EAS Build iOS (production profile)
pnpm build:android:dev    # EAS Build Android (development profile)
pnpm build:android:preview # EAS Build Android (preview profile)
pnpm build:android:prod   # EAS Build Android (production profile)
pnpm submit:ios           # EAS Submit iOS
pnpm submit:android       # EAS Submit Android
pnpm typecheck            # TypeScript type checking
pnpm generate             # Regenerate proto bindings from ./backend/proto
pnpm lint                 # ESLint (typescript-eslint type-checked)
pnpm test                 # Unit tests (Vitest)
```

### Local Backend

The app connects to the backend API via `EXPO_PUBLIC_API_URL` (default: `http://localhost:8090`).

To run the backend locally (from the repo root):
```bash
cd backend
docker compose up -d postgres firestore   # Start Postgres + Firestore emulator
make migrate-up                            # Run migrations
CORS_ALLOWED_ORIGINS="http://localhost:3000,http://localhost:8081" make run
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `EXPO_PUBLIC_API_URL` | Backend API URL (default: `http://localhost:8090`) |
| `EXPO_PUBLIC_GOOGLE_CLIENT_ID` | Google OAuth client ID for `expo-auth-session` |

In production web (Cloud Run), a runtime `config.json` is generated by the Docker container entrypoint from environment variables, allowing config injection without rebuilding the app.

## Deployment

### Web (Cloud Run)
The web build (`pnpm build:web`) produces a static bundle in `dist/web/`. The Dockerfile uses `nginx:alpine` to serve it, with a custom entrypoint (`docker-entrypoint.sh`) that generates a runtime `config.json` from environment variables at container start.

CI auto-deploys to prod on push to `main`: Docker build → push to Artifact Registry → deploy to Cloud Run (`app.toqui.travel`). Uses Workload Identity Federation (keyless GCP auth).

### iOS / Android
`pnpm build:ios` and `pnpm build:android` produce native bundles via EAS Build (planned for production).

## Tech Stack

| Concern | Technology |
|---------|-----------|
| Framework | Expo SDK 55, Expo Router v55 (file-based routing) |
| Language | TypeScript, React 19, React Native 0.83 |
| API | ConnectRPC (`@connectrpc/connect-web`) with Bearer token auth |
| Auth | `expo-secure-store` (native) / `localStorage` (web) for JWT tokens |
| State | React Context (auth, transport), TanStack Query |
| i18n | `i18next` + `react-i18next` |
| Icons | `lucide-react-native` |
| Proto codegen | `@bufbuild/protoc-gen-es` (platform-agnostic TypeScript) |

## Auth Flow (Bearer Token)

Toqui ships as self-hostable OSS with **email + password as the primary login** and **Google OAuth as an optional, env-gated extra**. Facebook and Apple sign-in have been removed; their backend RPCs no longer exist. Operators decide whether Google is enabled at deploy time by setting `GOOGLE_CLIENT_ID` + `GOOGLE_CLIENT_SECRET` on the backend.

The signed-out home screen (`app/(tabs)/index.tsx`) calls `useAuthProviders()` once on mount to learn which providers the server has enabled. The Google sign-in button is only rendered when `googleOauth === true` in the response; the email options are always shown.

**Email + password (always available):**

1. User taps "Sign in with email" or "Create account" on `app/(tabs)/index.tsx` and lands on `app/auth/email-login.tsx` or `app/auth/email-register.tsx`.
2. Screen calls `useAuth().loginWithEmail(email, password)` or `registerWithEmail(email, password, name)`.
3. AuthProvider builds an unauthenticated transport and invokes `AuthService.EmailLogin` / `EmailRegister`.
4. Backend returns `{ access_token, refresh_token, user }` in the response body.
5. Tokens are persisted to `expo-secure-store` (native) or `localStorage` (web); the user is redirected to `/(tabs)`.

The screens map ConnectRPC error codes to user-facing messages: `Unauthenticated` on login → "Invalid email or password"; `AlreadyExists` on register → "Email already registered"; `InvalidArgument` on register → "Email must be valid; password must be at least 12 characters". The backend's `bcrypt` + proto validation is the single source of truth for password rules — there is no frontend strength meter.

**Google OAuth (optional, env-gated):**

1. User authenticates via Google OAuth (`expo-auth-session`).
2. App sends the auth code to `AuthService.GoogleLogin`.
3. Backend returns `{ access_token, refresh_token, user }`.
4. Same persistence + redirect path as the email flow.

**Shared (all flows):**

- `TransportProvider` interceptor attaches `Authorization: Bearer <token>` to every ConnectRPC request.
- On 401, interceptor calls `AuthService.RefreshToken`, stores new tokens, retries.
- Logout clears stored tokens.
- Password reset and email verification are deliberately out of scope for the OSS auth surface — operators manage their own user database directly when needed.

**No cookies are used.** The backend's `cookieAuth` middleware is bypassed — the auth interceptor reads the Bearer header directly.

## Conventions

- **Routing**: Expo Router file-based routing in `app/` directory
- **State management**: React Context for auth/transport, TanStack Query for server state
- **Proto types**: Import from `@gen/toqui/v1/*_pb` (committed, regenerate with `pnpm generate`)
- **Components**: Functional components with TypeScript, React Native primitives
- **Styling**: `StyleSheet.create` (NativeWind planned for Tailwind-like classes)

## Provider Stack

Providers wrap the entire app in `app/_layout.tsx`:

```
ThemeProvider → I18nProvider → QueryClientProvider → AuthProvider → TransportProvider → AIDisclaimerGate → {children}
```

- **ThemeProvider** — Light/dark/system theme management with persistence (`lib/theme.tsx`). Provides `ThemeColors` interface to all components.
- **I18nProvider** — i18next initialization with English translations
- **QueryClientProvider** — TanStack Query client for server state caching and mutations
- **AuthProvider** — JWT token management, SecureStore/localStorage persistence
- **TransportProvider** — ConnectRPC transport with Bearer auth interceptor + auto-refresh on 401.
- **AIDisclaimerGate** — One-time blocking modal on first sign-in per device that surfaces the "AI may be wrong, especially on visa/health/safety; verify before booking" disclaimer (toqui#197). Acceptance is stored per-user in `expo-secure-store` (native) or `localStorage` (web) under `toqui_ai_disclaimer_acked_v1_<userId>`.

## Hooks

All hooks live in `lib/hooks/`. Transport pattern: ConnectRPC hooks use `useTransport()` for proto RPCs; REST hooks use `authFetch` from `lib/authFetch.ts`.

| Hook | Transport | Purpose |
|------|-----------|---------|
| `useTrips` | ConnectRPC | Trip CRUD (list, create, update, delete) via TripService |
| `useChat` | ConnectRPC (SSE) | Streaming chat — handles tool events, persona switches |
| `useBookings` | ConnectRPC | Booking CRUD (list, create, update, delete) via BookingService |
| `useItinerary` | ConnectRPC | Fetch trip itinerary via TripService |
| `useLocation` | expo-location | Device location permission + tracking (companion mode) |
| `useWeather` | REST (Open-Meteo) | Current weather for trip destination coordinates |
| `useCollaborators` | ConnectRPC | Trip collaborator management (invite, remove, list) |
| `useOnboarding` | Local state | Onboarding flow state (age gate, template selection, first trip) |
| `useNetworkStatus` | expo-network | Online/offline detection, reconnection handling |
| `useFeedback` | REST | Submit user feedback (`POST /api/feedback`) |
| `useDestinationGuide` | REST | Fetch destination guides (`GET /api/guides`) |
| `useAuthProviders` | ConnectRPC | Fetch server's enabled auth providers (`AuthService.GetAuthProviders`) — `emailPassword` always true; `googleOauth` env-gated. Cached for the session (`staleTime: Infinity`). |

## Security

### Auth Token Storage

- **Native (iOS/Android):** Tokens in `expo-secure-store` (Keychain/Keystore)
- **Web:** Tokens in `localStorage` (persists across sessions; refresh token has 30-day server-side expiry)
- No tokens in URL params or query strings
- Auto-refresh before expiry via `AuthService.RefreshToken` RPC

## Export Utilities

`lib/export/` provides two export formats, with platform-specific implementations:

| Export | Native | Web |
|--------|--------|-----|
| PDF (`pdf-export.ts`) | `expo-print` → system PDF dialog, `expo-sharing` | `window.print()` with print stylesheet |
| Calendar ICS (`calendar-export.ts`) | `expo-file-system` + `expo-sharing` | Blob download |

Both generate from the trip itinerary and are accessible from the trip settings screen.

## iOS Build

Native iOS builds use `expo prebuild` + Xcode:

- **Xcode 16.4 compatibility**: Patches in `patches/` fix build issues with newer Xcode toolchains
- **Fastlane**: `fastlane/` contains match signing config and build lanes
- **Expo prebuild flow**: `npx expo prebuild --platform ios` generates the `ios/` directory, then build with Xcode or `xcodebuild`
- **Config plugins**: `plugins/` directory contains custom Expo config plugins for native module tweaks
- See `IOS_SUBMISSION_GUIDE.md` for the full App Store submission checklist

## Pre-Commit Requirements

### Never Push Directly to Main — Use PRs

**MANDATORY**: All changes go through pull requests. Never push commits directly to `main`. This protects CI, enables review, and prevents broken deploys.

**Workflow:**
1. **Create a feature branch**: `git checkout -b feat/description` (or `fix/`, `chore/`, `docs/`)
2. **Run all checks locally before pushing**:
   ```bash
   pnpm typecheck && pnpm test
   ```
3. **Push the branch and open a PR**:
   ```bash
   git push -u origin feat/description
   gh pr create --title "feat: description" --body "## Summary\n..."
   ```
4. **Wait for CI to pass on the PR** — typecheck, tests, and builds must all be green
5. **Run adversarial review** on the PR branch (spawn a review agent against the diff)
6. **Merge via squash**: `gh pr merge --squash`
7. **After merge, verify CI passes on `main`** — if it breaks, fix immediately with another PR

### Keep CI Green — This Is Critical

**MANDATORY**: CI must stay green at all times on `main`. If a merge breaks CI, fix it immediately with a new PR before doing anything else.

Common failure causes in this repo:
- Adding new icons/exports used in components without updating the `vi.mock("lucide-react-native", ...)` in `components/chat/__tests__/ChatInput.test.tsx`
- Adding new hooks (`useAuth`, etc.) to components that have tests rendering them without the provider — mock the hook instead
- Test assertions that check message counts or button roles when the UI structure has changed (e.g., adding a new button)
- TypeScript: `PressableStateCallbackType` only has `pressed` — not `focused` or `hovered`

### QA Testing for AI/Chat Changes

When modifying chat-related components, hooks, or AI behavior, test against the live backend before merging:

1. Start the backend locally: `cd backend && make run`
2. Test AI flows with `buf curl` or `grpcurl` against `localhost:8090` to verify tool calls, persona switches, and itinerary creation work correctly
3. Verify the frontend renders tool results (itinerary updates, persona switches, recommendations) properly

### Adversarial Review

**MANDATORY**: Before merging any PR, spawn a parallel adversarial review agent to audit all changes in the PR.

## Related Docs

- [`backend/CLAUDE.md`](backend/CLAUDE.md) — Go API architecture, env config, sqlc/proto codegen, AI provider abstraction.
- [`backend/README.md`](backend/README.md) — backend dev quickstart.
