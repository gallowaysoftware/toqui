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

This is a 5-repo project under `github.com/gallowaysoftware`:

- **toqui** (this repo) — Expo React Native app (web + iOS + Android)
- **toqui-backend** — Go backend, ConnectRPC API, AI orchestration
- **toqui-terraform** — Terraform GCP + Cloudflare infrastructure (CI auto-plans on PR, auto-applies on merge)
- **toqui-site** — Astro static marketing site (Cloudflare Pages)
- **toqui-admin** — Vite React admin panel (Cloudflare Pages)

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
  auth/callback.tsx           OAuth callback handler
  shared/[token]/index.tsx    Public shared trip view
  onboarding.tsx              Onboarding flow
  privacy.tsx                 Privacy policy
  terms.tsx                   Terms of service
  _layout.tsx                 Root layout (providers)
components/                   Shared UI components
  DatePicker.tsx              Date picker component
  ErrorBoundary.tsx           React error boundary with Sentry reporting
  LocationPermission.tsx      Location permission request flow
  OfflineBanner.tsx           Network status banner (offline/reconnecting)
  ShareButton.tsx             Native/web share sheet integration
  auth/
    AgeGate.tsx               Age verification gate (18+ enforcement)
  bookings/
    ForwardingCard.tsx        Email forwarding setup card for booking import
  chat/
    ChatInput.tsx             Message input with send button and typing state
    FollowUpSuggestions.tsx   AI-generated follow-up question chips
    MessageBubble.tsx         Single chat message (user/AI/tool-result variants)
    PersonaIntroCard.tsx      Persona introduction card on switch
    RecommendationCard.tsx    Affiliate recommendation card with booking link
    SharePromptCard.tsx       Prompt to share trip with collaborators
    SuggestionChips.tsx       Quick suggestion chips in chat
    TypingIndicator.tsx       Animated typing indicator while AI responds
  checkout/
    ProUpgrade.tsx            Stripe hosted checkout redirect (all platforms)
  feedback/
    FeedbackModal.tsx         User feedback submission modal
  itinerary/
    ItineraryTimeline.tsx     Day-by-day itinerary timeline component
  map/
    ItineraryMap.tsx          Interactive map showing itinerary locations
  referral/
    ReferralCard.tsx          Referral code sharing with stats
  share/
    ShareNudgeBanner.tsx      Contextual nudge to share trip
  subscription/
    SubscriptionCard.tsx      Subscription tier selection and management
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
  attribution.ts              Read UTM/ref attribution cookie/AsyncStorage on signup; clears after
  analytics.tsx               PostHog privacy-first analytics provider
  config.ts                   Runtime config (EXPO_PUBLIC_* env vars)
  hooks/
    useTrips.ts               Trip CRUD via ConnectRPC TripService
    useChat.ts                SSE streaming chat — tool activity, personas, recommendations
    useBookings.ts            Booking CRUD via ConnectRPC BookingService
    useItinerary.ts           Itinerary fetch via ConnectRPC TripService
    useCheckout.ts            Stripe checkout init/status via REST
    useTrialStatus.ts         Trial expiration tracking via REST
    useReferral.ts            Referral code, stats, redemption via REST
    useFeedback.ts            Submit user feedback via REST
    useDestinationGuide.ts    Fetch destination guides via REST
    useUsage.ts               Daily message usage tracking via REST
    useLocation.ts            Device location permission + tracking
    useWeather.ts             Current weather for trip destination (Open-Meteo)
    useCollaborators.ts       Trip collaborator management
    useOnboarding.ts          Onboarding flow state
    useNetworkStatus.ts       Online/offline detection, reconnection handling
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
pnpm generate             # Regenerate proto bindings from ../toqui-backend
pnpm lint                 # ESLint (typescript-eslint type-checked)
pnpm test                 # Unit tests (Vitest)
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

## Environment Variables

| Variable | Description |
|----------|-------------|
| `EXPO_PUBLIC_API_URL` | Backend API URL (default: `http://localhost:8090`) |
| `EXPO_PUBLIC_GOOGLE_CLIENT_ID` | Google OAuth client ID for `expo-auth-session` |
| `EXPO_PUBLIC_POSTHOG_KEY` | PostHog project API key (EU instance) |
| `EXPO_PUBLIC_SENTRY_DSN` | Sentry DSN for error tracking |

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
| Analytics | PostHog (`posthog-react-native`, EU-hosted, privacy-first) |
| Error tracking | Sentry (`@sentry/react-native`, privacy-hardened, session replay) |

## Auth Flow (Bearer Token)

1. User authenticates via Google OAuth (`expo-auth-session`) or Facebook OAuth.
2. App sends auth code to backend's `AuthService.GoogleLogin` / `FacebookLogin` RPC.
3. Backend returns `{ access_token, refresh_token, user }` in response body.
4. Tokens stored in `expo-secure-store` (native) or `localStorage` (web).
5. `TransportProvider` interceptor attaches `Authorization: Bearer <token>` to every ConnectRPC request.
6. On 401, interceptor calls `AuthService.RefreshToken` RPC, stores new tokens, retries.
7. Logout clears stored tokens.

**Apple Sign-In (planned, native only):** the backend exposes `AuthService.AppleLogin` (returns `Unimplemented` until Apple Developer enrollment completes). Frontend integration pending — will use `expo-apple-authentication` to obtain `authorization_code` + `id_token`, then POST both to the RPC. Apple is a third login provider alongside Google and Facebook; same token-bearer flow applies once it lands.

**No cookies are used.** The backend's `cookieAuth` middleware is bypassed — the auth interceptor reads the Bearer header directly.

## Payment & Trip Pro

Trip Pro ($19/trip) is purchased via Stripe hosted checkout (all platforms):

1. User taps "Upgrade" → `useCheckout.initCheckout(tripId)` → `POST /api/checkout` → backend creates Stripe checkout session
2. `ProUpgrade.tsx` redirects to Stripe via `Linking.openURL(url)` (works on web and native)
3. User completes payment on Stripe's hosted page
4. Stripe sends webhook to backend → backend verifies and unlocks the trip in the database
5. `useCheckout.checkStatus(tripId)` polls `GET /api/checkout/status` to confirm unlock

Unlocked trips get: unlimited messages, all 989 expert personas (43 locations × 23 themes), email forwarding, export, best-fit recommendations.

## Referral
`ReferralCard.tsx` and `useReferral` hook:
- `GET /api/referral` — fetch user's referral code and referred-user count
- `POST /api/referral/redeem` — redeem another user's referral code
- Share link: `https://toqui.travel?ref=CODE`

## Conventions

- **Routing**: Expo Router file-based routing in `app/` directory
- **State management**: React Context for auth/transport, TanStack Query for server state
- **Proto types**: Import from `@gen/toqui/v1/*_pb` (committed, regenerate with `pnpm generate`)
- **Components**: Functional components with TypeScript, React Native primitives
- **Styling**: `StyleSheet.create` (NativeWind planned for Tailwind-like classes)

## Provider Stack

Providers wrap the entire app in `app/_layout.tsx`:

```
ThemeProvider → I18nProvider → QueryClientProvider → AuthProvider → AnalyticsProvider → TransportProvider → AgeGate → ConsentGate → AIDisclaimerGate → {children}
```

- **ThemeProvider** — Light/dark/system theme management with persistence (`lib/theme.tsx`). Provides `ThemeColors` interface to all components.
- **I18nProvider** — i18next initialization with English translations
- **QueryClientProvider** — TanStack Query client for server state caching and mutations
- **AuthProvider** — JWT token management, SecureStore/localStorage persistence
- **AnalyticsProvider** — PostHog initialization, user identification, feature flags (`lib/analytics.tsx`)
- **TransportProvider** — ConnectRPC transport with Bearer auth interceptor + auto-refresh on 401. Also detects the backend `FailedPrecondition("consent_required")` sentinel and exposes it via `useConsentSignal()`.
- **AgeGate** — Wraps the app to enforce 18+ verification, but **only post-OAuth** (toqui-backend#420 redesign). Behaviour: logged-out users are not gated (the `accessToken` short-circuit at the top of the component renders children unchanged so marketing/sign-in screens are visible without a DOB demand). Logged-in users with `user.ageVerifiedAt` set pass through. Logged-in users without it see the DOB form. On submit, the backend is the single enforcement point: `>= 18` returns 200 and the user proceeds; `< 18` triggers a backend-side hard delete (`lifecycle.DeleteUser`) plus a SHA-256 email entry in `under_age_blocks` for anti-evasion, and the frontend renders a deletion-confirmation screen + calls `logout()` to clear local tokens. The legacy `toqui_age_verified` localStorage key was dropped — `user.ageVerifiedAt` is the source of truth on every render.
- **ConsentGate** — Pops a blocking modal when the transport interceptor sees `FailedPrecondition("consent_required")` from the backend (toqui-backend PR #374). Calls `POST /auth/consent` to record `terms` + `privacy_policy`, invalidates React Query caches so errored fetches refire, and clears the signal on success.
- **AIDisclaimerGate** — One-time blocking modal on first sign-in per device that surfaces the "AI may be wrong, especially on visa/health/safety; verify before booking" disclaimer (toqui#197). Acceptance is stored per-user in `expo-secure-store` (native) or `localStorage` (web) under `toqui_ai_disclaimer_acked_v1_<userId>` and audit-trailed via the PostHog `ai_disclaimer_acknowledged` event.

## Hooks

All hooks live in `lib/hooks/`. Transport pattern: ConnectRPC hooks use `useTransport()` for proto RPCs; REST hooks use `authFetch` from `lib/authFetch.ts`.

| Hook | Transport | Purpose |
|------|-----------|---------|
| `useTrips` | ConnectRPC | Trip CRUD (list, create, update, delete) via TripService |
| `useChat` | ConnectRPC (SSE) | Streaming chat — handles tool events, persona switches, recommendations |
| `useBookings` | ConnectRPC | Booking CRUD (list, create, update, delete) via BookingService |
| `useItinerary` | ConnectRPC | Fetch trip itinerary via TripService |
| `useCheckout` | REST | Init checkout (`POST /api/checkout`), validate payment, poll unlock status |
| `useTrialStatus` | REST | Poll trial expiration via checkout status endpoint |
| `useReferral` | REST | Get referral code/stats, redeem codes (`POST /api/referral/redeem`) |
| `useLocation` | expo-location | Device location permission + tracking (companion mode) |
| `useWeather` | REST (Open-Meteo) | Current weather for trip destination coordinates |
| `useCollaborators` | ConnectRPC | Trip collaborator management (invite, remove, list) |
| `useOnboarding` | Local state | Onboarding flow state (age gate, template selection, first trip) |
| `useNetworkStatus` | expo-network | Online/offline detection, reconnection handling |
| `useAnalytics` | PostHog | Privacy-first event tracking (EU-hosted, via `lib/analytics.tsx`) |
| `useFeedback` | REST | Submit user feedback (`POST /api/feedback`) |
| `useDestinationGuide` | REST | Fetch destination guides (`GET /api/guides`) |
| `useUsage` | REST | Daily message usage tracking (`GET /api/usage`) |
| `useSubscription` | REST | Subscription management (tier, status, checkout, cancel, portal) via REST |

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

## Analytics (PostHog)

EU-hosted PostHog instance (`eu.i.posthog.com`). Privacy-first setup compliant with Core Principles:

- **Tracked events (acquisition)**: session_start, return_visit, signup_started, signup_completed, signin_completed, age_gate_passed, ai_disclaimer_acknowledged, consent_recorded, onboarding_completed
- **Tracked events (engagement)**: trip_created, first_trip_created, second_trip_created, trip_shared, shared_trip_viewed, shared_trip_signup_clicked, first_message_sent, first_itinerary_generated, itinerary_generated, error_encountered
- **Tracked events (monetization)**: upgrade_viewed, upgrade_prompt_shown, upgrade_started, checkout_initiated, payment_completed, recommendation_clicked, subscription_started, subscription_cancel_started, subscription_manage_opened
- All events track behavior patterns only — no destination names, chat content, or PII
- User IDs are pseudonymized (SHA-256 hashed)
- Session replay enabled with all text inputs and chat content masked
- Autocapture disabled — only explicit events are tracked
- `useAnalytics` hook provides `track()`, `identify()`, and `getFeatureFlag()` across the app

## Error Tracking (Sentry)

Privacy-hardened Sentry setup (`@sentry/react-native`):

- Session replay with text masking (all inputs, chat, itinerary details)
- User feedback widget for crash reports
- Breadcrumbs for navigation and network requests (URLs only, no bodies)
- Source maps uploaded during CI build
- No PII in error context — user IDs are pseudonymized

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

1. Start the backend locally: `cd ../toqui-backend && make run`
2. Test AI flows with `buf curl` or `grpcurl` against `localhost:8090` to verify tool calls, persona switches, and itinerary creation work correctly
3. Verify the frontend renders tool results (itinerary updates, persona switches, recommendations) properly

### Adversarial Review

**MANDATORY**: Before merging any PR, spawn a parallel adversarial review agent to audit all changes in the PR.

## Cross-Repo Consistency

**IMPORTANT**: This project spans 5 repos. When making changes that affect shared documentation, update CLAUDE.md in ALL repos:

- `/Users/pequalsnp/src/github.com/gallowaysoftware/toqui/CLAUDE.md` (this file)
- `/Users/pequalsnp/src/github.com/gallowaysoftware/toqui-backend/CLAUDE.md`
- `/Users/pequalsnp/src/github.com/gallowaysoftware/toqui-terraform/CLAUDE.md`
- `/Users/pequalsnp/src/github.com/gallowaysoftware/toqui-site/CLAUDE.md`
- `/Users/pequalsnp/src/github.com/gallowaysoftware/toqui-admin/CLAUDE.md`

## Related Repos

- **toqui-backend** (`github.com/gallowaysoftware/toqui-backend`) — Go API server
- **toqui-terraform** (`github.com/gallowaysoftware/toqui-terraform`) — Terraform infrastructure
- **toqui-site** (`github.com/gallowaysoftware/toqui-site`) — Astro marketing site
- **toqui-admin** (`github.com/gallowaysoftware/toqui-admin`) — Vite React admin panel
