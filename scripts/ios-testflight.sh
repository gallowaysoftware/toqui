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
# For a fully non-interactive run (e.g. triggered over SSH), export an App Store
# Connect API key. It authenticates BOTH the provisioning-profile creation
# during signing AND the TestFlight upload, so nothing prompts for your Apple ID
# + 2FA. Without it, the first build signs via the Xcode GUI keychain session
# (which periodically needs an interactive 2FA re-login) and the upload prompts.
#   export ASC_KEY_ID=XXXXXXXXXX
#   export ASC_ISSUER_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
#   export ASC_KEY_P8_PATH=/path/to/AuthKey_XXXXXXXXXX.p8
# APPLE_TEAM_ID selects the signing team (it is not itself a credential):
#   export APPLE_TEAM_ID=XXXXXXXXXX
set -euo pipefail

cd "$(dirname "$0")/.."
# Guard against a wrong CWD (e.g. the script reached via a symlink): a clear
# message beats a confusing failure deep in prebuild.
if [[ ! -f app.json || ! -d scripts ]]; then
  echo "error: run this from the toqui repo (couldn't find app.json here)." >&2
  exit 1
fi

if [[ "$(uname)" != "Darwin" ]]; then
  echo "error: iOS builds must run on macOS (the M3 build box)." >&2
  exit 1
fi

echo "==> Installing JS dependencies"
pnpm install --frozen-lockfile

echo "==> Bumping ios.buildNumber in app.json (TestFlight rejects duplicates)"
# Edit the single buildNumber line in place (regex on the raw text) rather than
# re-serializing the whole JSON, so the bump diff touches only that one line
# and doesn't reformat unrelated fields.
node -e '
  const fs = require("fs"), p = "app.json";
  let s = fs.readFileSync(p, "utf8");
  const re = /("buildNumber"\s*:\s*")(\d+)(")/;
  const m = s.match(re);
  if (!m) { console.error("could not find a numeric ios.buildNumber in app.json"); process.exit(1); }
  const next = String((parseInt(m[2], 10) || 0) + 1);
  s = s.replace(re, `$1${next}$3`);
  fs.writeFileSync(p, s);
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
