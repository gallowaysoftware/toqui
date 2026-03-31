# iOS Setup Guide

Step-by-step guide for building and distributing Toqui on iOS via TestFlight. Assumes you have never published an iOS app before.

Bundle ID: `travel.toqui.app` | Expo SDK 55

## 1. Prerequisites

Install the following before starting:

```bash
# Xcode 16+ from the Mac App Store
# Then install command line tools:
xcode-select --install

# CocoaPods
sudo gem install cocoapods

# EAS CLI
npm install -g eas-cli

# Verify pnpm and Node
pnpm --version   # any recent version
node --version    # 24+
```

Open Xcode at least once and accept the license agreement. Install any additional platform components it prompts for (iOS simulator runtimes).

## 2. Apple Developer Program Enrollment

1. Go to <https://developer.apple.com/programs/> and click "Enroll".
2. Sign in with your Apple ID (or create one).
3. Choose account type:
   - **Individual** ($99/year) -- approved in 1-48 hours.
   - **Organization** ($99/year) -- requires a D-U-N-S number, takes 5+ business days.
4. Complete payment and wait for the approval email.
5. After approval, go to **Account > Membership** and note your **Team ID** (a 10-character alphanumeric string).
6. Update `eas.json` submit section with your Team ID:

```json
{
  "submit": {
    "production": {
      "ios": {
        "appleTeamId": "YOUR_TEAM_ID"
      }
    }
  }
}
```

## 3. App Store Connect Setup

### Register the Bundle ID

1. Go to <https://developer.apple.com/account/resources/identifiers/list>.
2. Click **+** to register a new identifier.
3. Select **App IDs**, then **App** (not App Clip). Click Continue.
4. Platform: **iOS**.
5. Description: `Toqui`.
6. Bundle ID: select **Explicit**, enter `travel.toqui.app`.
7. Under Capabilities, enable **Associated Domains**.
8. Click **Continue**, then **Register**.

### Create the App in App Store Connect

1. Go to <https://appstoreconnect.apple.com> > **My Apps** > **+** > **New App**.
2. Fill in:
   - **Name:** `Toqui`
   - **Primary Language:** English (U.S.)
   - **Bundle ID:** select `travel.toqui.app` from the dropdown
   - **SKU:** `toqui`
3. Click **Create**.
4. Go to **App Information > General Information** and note the numeric **Apple ID**.
5. Update `eas.json` with the Apple ID:

```json
{
  "submit": {
    "production": {
      "ios": {
        "appleTeamId": "YOUR_TEAM_ID",
        "ascAppId": "YOUR_APPLE_ID"
      }
    }
  }
}
```

## 4. Local Development Setup

```bash
# Install dependencies
pnpm install

# Generate the native iOS project
pnpm prebuild:ios
```

This runs `npx expo prebuild --platform ios --clean`, producing an `ios/` directory with an Xcode workspace.

```bash
# Open in Xcode (always use .xcworkspace, NOT .xcodeproj)
open ios/toqui.xcworkspace
```

In Xcode:

1. Select the **toqui** target in the project navigator.
2. Go to **Signing & Capabilities**.
3. Check **Automatically manage signing**.
4. Select your team from the **Team** dropdown. Xcode creates provisioning profiles automatically.

### Run on Simulator

Select a simulator device from the toolbar (e.g., iPhone 16), then press **Cmd+R**.

### Run on Physical Device

1. Connect your iPhone via USB.
2. On the device, trust the computer when prompted.
3. In Xcode, select your device from the target dropdown.
4. Press **Cmd+R**. On first run, you may need to trust the developer certificate on the device: **Settings > General > VPN & Device Management**.

## 5. Building for TestFlight

### Option A: EAS Local Build

```bash
pnpm build:ios:local
```

This runs `eas build --platform ios --local`, producing a `.ipa` file in the project root. Requires Xcode and CocoaPods installed locally.

### Option B: Xcode Archive

1. In Xcode, set the target device to **Any iOS Device (arm64)**.
2. **Product > Archive**.
3. When the archive completes, the Organizer window opens automatically.
4. Select the archive and click **Distribute App > App Store Connect**.
5. Follow the prompts to upload.

### Upload to TestFlight

If you used EAS local build (Option A):

```bash
pnpm submit:ios
```

This runs `eas submit --platform ios` and uploads the `.ipa` to App Store Connect. You will be prompted for your Apple ID credentials (or can use an app-specific password).

If you used Xcode Archive (Option B), the upload happens as part of the "Distribute App" step above.

## 6. TestFlight Distribution

After uploading, wait 5-30 minutes for App Store Connect to process the build. You will receive an email when it is ready.

### Internal Testing (up to 100 testers, no review needed)

1. In App Store Connect, go to **TestFlight > Internal Testing**.
2. Create a group (e.g., "Team") if one does not exist.
3. Add testers by their Apple ID email address.
4. Select the build to test.
5. Testers receive an email invite and install the app via the **TestFlight** app on their device.

### External Testing (up to 10,000 testers, requires Beta App Review)

1. In App Store Connect, go to **TestFlight > External Testing**.
2. Create a group and add testers by email.
3. Select a build and submit it for **Beta App Review** (usually approved in 24-48 hours).
4. After approval, testers receive an email invite.

### Notes

- Each TestFlight build expires after **90 days**.
- You must increment `buildNumber` in `app.json` for each new upload, or rely on `autoIncrement` in `eas.json` (already configured in the `production` profile).
- The `version` field in `app.json` is the user-facing version (e.g., `0.1.0`). The `buildNumber` is the internal build number (e.g., `1`, `2`, `3`).

## 7. Google OAuth iOS Client ID

The app uses Google OAuth via `expo-auth-session`. iOS requires a separate OAuth client ID.

1. Go to [Google Cloud Console > APIs & Credentials](https://console.cloud.google.com/apis/credentials).
2. Ensure you are in the **same GCP project** as the existing web OAuth client.
3. Click **+ Create Credentials > OAuth 2.0 Client ID**.
4. Application type: **iOS**.
5. Name: `Toqui iOS`.
6. Bundle ID: `travel.toqui.app`.
7. No redirect URIs needed -- iOS uses bundle ID verification.
8. Click **Create** and copy the client ID.
9. Set in your environment:

```bash
EXPO_PUBLIC_GOOGLE_IOS_CLIENT_ID=<your-client-id>
```

For EAS builds, add to `eas.json` env section or use [EAS Secrets](https://docs.expo.dev/build-reference/variables/#using-secrets-in-environment-variables):

```json
{
  "build": {
    "production": {
      "env": {
        "EXPO_PUBLIC_GOOGLE_IOS_CLIENT_ID": "<your-client-id>"
      }
    }
  }
}
```

The backend token exchange works without changes. Google allows cross-client auth code exchange within the same GCP project, so the existing `AuthService.GoogleLogin` RPC accepts auth codes from the iOS client.

## 8. Associated Domains (Universal Links)

This enables `https://toqui.travel/shared/TOKEN` links to open directly in the app.

The `app.json` `ios` section should include:

```json
{
  "ios": {
    "associatedDomains": ["applinks:toqui.travel"]
  }
}
```

You must host an Apple App Site Association (AASA) file at:

```
https://toqui.travel/.well-known/apple-app-site-association
```

Contents (replace `TEAMID` with your Apple Team ID):

```json
{
  "applinks": {
    "apps": [],
    "details": [
      {
        "appIDs": ["TEAMID.travel.toqui.app"],
        "paths": ["/shared/*"]
      }
    ]
  }
}
```

Requirements:
- Served over HTTPS with `Content-Type: application/json`.
- No redirects -- Apple fetches this URL directly.
- Host it on the toqui-site (Astro static site) or via a Cloudflare Worker.

## 9. Environment Variables Reference

| Variable | Where | Purpose |
|----------|-------|---------|
| `EXPO_PUBLIC_API_URL` | eas.json env / .env.local | Backend API URL |
| `EXPO_PUBLIC_GOOGLE_CLIENT_ID` | eas.json env / .env.local | Web Google OAuth client ID |
| `EXPO_PUBLIC_GOOGLE_IOS_CLIENT_ID` | eas.json env / .env.local | iOS Google OAuth client ID |

For local development, set these in a `.env.local` file (git-ignored). For EAS builds, configure them in `eas.json` under the appropriate build profile's `env` key, or use EAS Secrets for sensitive values.

## 10. Troubleshooting

**"No signing certificate"**
Ensure you selected a team in Xcode under Signing & Capabilities. If the team dropdown is empty, verify your Apple Developer Program membership is active.

**"Provisioning profile doesn't include signing certificate"**
Delete the `ios/` directory and re-run `pnpm prebuild:ios`. If that does not help, go to Xcode > Settings > Accounts, select your team, and click "Download Manual Profiles".

**CocoaPods errors**
```bash
cd ios && pod install --repo-update
```

**MapLibre build errors**
Ensure the minimum iOS deployment target is 13.0+ in Xcode. Check `ios/Podfile` for the `platform :ios` line.

**"The build number has already been used"**
Increment `buildNumber` in `app.json` (or let `autoIncrement` in `eas.json` handle it for EAS builds).

**Prebuild generates stale project**
The `prebuild:ios` script uses the `--clean` flag, which deletes and regenerates the `ios/` directory. If you see stale state, run it again.

**"Unable to install" on physical device**
Go to **Settings > General > VPN & Device Management** on the device and trust your developer certificate.

**Expo modules not linking**
Run `pnpm prebuild:ios` again. Expo's autolinking resolves native module dependencies during prebuild.
