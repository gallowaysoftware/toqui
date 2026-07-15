# iOS TestFlight & App Store Guide

Toqui is self-hostable OSS: you build and ship it under **your own** Apple
Developer account. Bundle id is `travel.toqui.app` (already set in `app.json`,
`fastlane/Appfile`, and `eas.json`).

**No server URL is baked into the build** — on first launch the app shows a
server-setup screen where you enter your API URL (the HTTPS host in front of
your backend); it verifies `/healthz` and you're in.

There are two goals, with very different requirements:

- **Part 1 — TestFlight (get it on your own phone):** a build, an upload, and
  adding yourself as a tester. **No screenshots, no listing metadata, no App
  Review, no age rating, no privacy URL.** This is the fast path.
- **Part 2 — Public App Store release (optional, later):** the full listing +
  App Review.

---

## Part 1 — TestFlight

### Prerequisites (on the macOS build box)

- Apple Developer Program membership (an org account works — e.g. Galloway
  Software Solutions).
- Xcode 16+ (the repo carries Xcode 16.4 compat patches in `patches/` +
  `plugins/`; a newer Xcode may need them refreshed).
- Node + `pnpm`, CocoaPods (`brew install cocoapods`), and Fastlane
  (`brew install fastlane`).
- The repo cloned, and you signed into your Apple account in **Xcode →
  Settings → Accounts** (this is what lets automatic signing mint the
  provisioning profile).

### One-time: register the app with Apple

1. **App ID** — https://developer.apple.com/account/resources/identifiers/list
   → **+** → App IDs → App → Bundle ID `travel.toqui.app` (Explicit) → enable
   **Associated Domains** (for deep links) → Register.
2. **App record** — https://appstoreconnect.apple.com/apps → **+** → New App →
   iOS → Bundle ID `travel.toqui.app`, SKU `toqui-ios-1`, Full Access.

### One-time: identity for Fastlane

`fastlane/Appfile` reads your identity from the environment (so nothing
personal is committed). Export these on the build box — e.g. in `~/.zshrc`:

```bash
export APPLE_ID="you@example.com"       # your Apple Developer account email
export APPLE_TEAM_ID="XXXXXXXXXX"       # Developer Team ID — developer.apple.com/account → Membership
# export ITC_TEAM_ID="XXXXXXXXXX"       # only if your App Store Connect team id differs
```

`APPLE_TEAM_ID` is also used by the build lane to drive automatic signing.

### Build + upload

From the **repo root** on the Mac (directly, or over SSH):

```bash
./scripts/ios-testflight.sh
```

That script: installs JS deps → bumps `ios.buildNumber` in `app.json`
(TestFlight rejects duplicate build numbers) → `expo prebuild --platform ios
--clean` → `pod install` → `fastlane beta` (archive + upload). Commit the
`app.json` build-number bump afterward so the next build increments from it.

On a fresh box, **two** steps need Apple auth: creating the distribution
provisioning profile during signing, and the TestFlight upload. Run directly on
the Mac with your Apple ID signed into Xcode, both use that session (the upload
prompts for a 2FA code in the terminal the first time). To make the whole run
**non-interactive** — required for SSH-triggered builds, where there's no Xcode
GUI session to lean on — create an **App Store Connect API key** (App Store
Connect → Users and Access → Integrations → App Store Connect API → **+**),
download the `.p8`, and export the three vars below. The key authenticates both
the signing step and the upload:

```bash
export ASC_KEY_ID="XXXXXXXXXX"
export ASC_ISSUER_ID="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
export ASC_KEY_P8_PATH="$HOME/.appstoreconnect/AuthKey_XXXXXXXXXX.p8"
```

The Fastfile picks these up automatically when present.

> Running the lanes directly: run `fastlane beta` **from the repo root**, not
> from `ios/` — the Fastfile lives at the repo root and its paths are
> `ios/`-relative. Always `expo prebuild` + `pod install` first (the script
> does this for you). The lane does **not** bump the build number — the script
> owns that (it edits `app.json`), so if you invoke `fastlane beta` by hand,
> bump `expo.ios.buildNumber` in `app.json` yourself first or TestFlight will
> reject the upload as a duplicate.

> The script runs `pnpm install --frozen-lockfile`, so it aborts if
> `pnpm-lock.yaml` is out of sync with `package.json` — commit an up-to-date
> lockfile before building.

### Install on your phone

App Store Connect → your app → **TestFlight → Internal Testing** → add yourself
(your Apple ID) as an internal tester (internal testing needs **no** Beta App
Review). Install the **TestFlight** app from the App Store on your iPhone,
accept the invite, install Toqui. On first launch, enter your API URL in the
server-setup screen.

---

## Part 2 — Public App Store release (optional)

Only if you want Toqui publicly listed. This path goes through App Review and
additionally needs:

- **Screenshots** per device size (6.7" = 1290×2796, 6.5" = 1284×2778), 3–10
  each. Suggested: trip list, AI chat planning a trip, day-by-day itinerary,
  weather + trip detail, companion suggestions.
- **Listing metadata** — see `store-metadata.md` in the repo root.
- A hosted **Privacy Policy URL**. (The app also ships in-app `/privacy` and
  `/terms` screens; App Review still requires a reachable public URL.)
- The **age-rating** questionnaire.
- **Review notes.** Auth is **email + password** (Google and generic OIDC/SSO
  are optional, operator-configured). Either provide a test account or note
  that reviewers can self-register. The app is **free with no in-app
  purchases**.

Build + submit with the release lane instead:

```bash
npx expo prebuild --platform ios --clean && (cd ios && pod install)
fastlane release
```

---

## Updating (subsequent builds)

```bash
git pull                     # get the latest main
./scripts/ios-testflight.sh  # bumps buildNumber, prebuilds, archives, uploads
# commit the app.json buildNumber bump
```

## Troubleshooting

- **No signing certificate / profile** — confirm you're signed into your Apple
  account in Xcode → Settings → Accounts and that `APPLE_TEAM_ID` is exported;
  the lane passes `-allowProvisioningUpdates` so Xcode can create the profile.
- **"Bundle ID not available"** — `travel.toqui.app` is registered under a
  different team; register it under yours.
- **Pod install fails** — `cd ios && pod deintegrate && pod install`.
- **Build fails after a dependency change** — the script already runs
  `expo prebuild --clean`; if you're building by hand, add `--clean` and
  re-run `pod install`.
- **Duplicate build number rejected** — the script bumps `app.json`; if you
  built by hand, bump `expo.ios.buildNumber` in `app.json` before prebuild.
