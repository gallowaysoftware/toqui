#!/usr/bin/env bash
#
# One-command local iOS TestFlight build — the "lightweight build server".
#
# Run on the macOS build box (the M3) from the repo root, either directly or
# over SSH:
#
#   ./scripts/ios-testflight.sh
#
# It installs JS deps, bumps the iOS build number, regenerates the native
# project from a clean slate, and archives + uploads a Release build to
# TestFlight via Fastlane. The one-time signing/tooling setup this relies on
# (Xcode account, automatic signing, CocoaPods, Fastlane) is documented in
# IOS_SUBMISSION_GUIDE.md.
#
# Optional non-interactive upload (e.g. when triggered over SSH): export an
# App Store Connect API key so Fastlane doesn't prompt for your Apple ID + 2FA:
#   export ASC_KEY_ID=XXXXXXXXXX
#   export ASC_ISSUER_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
#   export ASC_KEY_P8_PATH=/path/to/AuthKey_XXXXXXXXXX.p8
# And, so gym can create/refresh the provisioning profile without the Xcode UI:
#   export APPLE_TEAM_ID=XXXXXXXXXX
set -euo pipefail

cd "$(dirname "$0")/.."

if [[ "$(uname)" != "Darwin" ]]; then
  echo "error: iOS builds must run on macOS (the M3 build box)." >&2
  exit 1
fi

echo "==> Installing JS dependencies"
pnpm install --frozen-lockfile

echo "==> Bumping ios.buildNumber in app.json (TestFlight rejects duplicates)"
node -e '
  const fs = require("fs"), p = "app.json";
  const j = JSON.parse(fs.readFileSync(p, "utf8"));
  const next = String((parseInt(j.expo.ios.buildNumber || "0", 10) || 0) + 1);
  j.expo.ios.buildNumber = next;
  fs.writeFileSync(p, JSON.stringify(j, null, 2) + "\n");
  console.log("    ios.buildNumber -> " + next);
'

echo "==> Regenerating the native iOS project (expo prebuild --clean)"
npx expo prebuild --platform ios --clean

echo "==> pod install"
( cd ios && pod install )

echo "==> Archiving + uploading to TestFlight (fastlane beta)"
fastlane beta

echo
echo "==> Submitted. TestFlight processing takes ~15-30 min; watch for the email."
echo "    Commit the app.json buildNumber bump so the next build increments from it."
