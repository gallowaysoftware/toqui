# iOS App Store Submission Guide

## Prerequisites

Before starting, you need:
1. **Apple Developer Program membership** ($99/year) — enroll at https://developer.apple.com/programs/
2. **App Store Connect access** — comes with the developer program
3. **Xcode 16+** installed (you have 16.4)
4. **Fastlane** installed (done: `/opt/homebrew/lib/ruby/gems/4.0.0/bin/fastlane`)

## One-Time Setup

### 1. Create App ID in Apple Developer Portal

1. Go to https://developer.apple.com/account/resources/identifiers/list
2. Click "+" to register a new identifier
3. Select "App IDs" → "App"
4. Description: "Toqui"
5. Bundle ID: `travel.toqui.app` (Explicit)
6. Capabilities: check "Associated Domains" (for deep links) and "Sign In with Apple" (if adding later)
7. Click Register

### 2. Create App in App Store Connect

1. Go to https://appstoreconnect.apple.com/apps
2. Click "+" → "New App"
3. Platform: iOS
4. Name: "Toqui — AI Travel Companion"
5. Primary Language: English (U.S.)
6. Bundle ID: Select `travel.toqui.app`
7. SKU: "toqui-ios-1"
8. User Access: Full Access

### 3. Configure Fastlane

Edit `ios/fastlane/Appfile` — uncomment and fill in:
```ruby
app_identifier("travel.toqui.app")
apple_id("your@email.com")        # Your Apple ID email
team_id("XXXXXXXXXX")             # Your Apple Developer Team ID
itc_team_id("XXXXXXXXXX")         # App Store Connect Team ID (usually same)
```

Find your Team ID at: https://developer.apple.com/account/#/membership

### 4. Set Up Code Signing

Fastlane can manage certificates automatically:

```bash
cd ios
export PATH="/opt/homebrew/opt/ruby/bin:/opt/homebrew/lib/ruby/gems/4.0.0/bin:/opt/homebrew/bin:$PATH"
export LANG=en_US.UTF-8
fastlane match init  # Choose "git" storage, point to a private repo for certs
fastlane match appstore  # Creates/downloads App Store distribution cert + profile
```

Or manage manually in Xcode:
1. Open `ios/Toqui.xcworkspace` in Xcode
2. Select the Toqui target → Signing & Capabilities
3. Check "Automatically manage signing"
4. Select your team
5. Xcode will create the provisioning profile automatically

## Building for TestFlight

### Option A: Fastlane (Recommended)

```bash
cd ios
export PATH="/opt/homebrew/opt/ruby/bin:/opt/homebrew/lib/ruby/gems/4.0.0/bin:/opt/homebrew/bin:$PATH"
export LANG=en_US.UTF-8
fastlane beta
```

This will:
1. Increment the build number
2. Build the Release archive
3. Upload to TestFlight
4. You'll get an email when processing is complete (~15-30 min)

### Option B: Xcode (Manual)

1. Open `ios/Toqui.xcworkspace` in Xcode
2. Select "Toqui" scheme, destination "Any iOS Device (arm64)"
3. Product → Archive (takes ~5 min)
4. When done, the Organizer window opens
5. Click "Distribute App" → "App Store Connect" → "Upload"
6. Follow the prompts (Xcode handles signing)
7. Wait for processing email (~15-30 min)

## App Store Listing Info

Fill these in App Store Connect:

| Field | Value |
|-------|-------|
| App Name | Toqui — AI Travel Companion |
| Subtitle | Plan smarter trips with expert AI personas |
| Category | Travel |
| Privacy Policy URL | https://toqui.travel/privacy |
| Support URL | https://toqui.travel/support |
| Marketing URL | https://toqui.travel |
| Description | See `store-metadata.md` in repo root |
| Keywords | travel,itinerary,ai,trip planner,vacation,travel companion,booking,travel guide |
| Age Rating | 17+ (has in-app purchases, web browsing) |
| Price | Free |
| In-App Purchases | Trip Pro — $12 CAD per trip |

### Screenshots Needed

App Store requires screenshots for each device size:
- **6.7" iPhone** (iPhone 15 Pro Max): 1290 x 2796
- **6.5" iPhone** (iPhone 14 Plus): 1284 x 2778
- **5.5" iPhone** (iPhone 8 Plus): 1242 x 2208 (optional)
- **12.9" iPad** (optional, since we support tablet)

You need 3-10 screenshots per device. Recommended:
1. Trip list with templates
2. Chat with AI planning a trip
3. Day-by-day itinerary
4. Weather card + trip detail
5. Companion mode with suggestions

### App Review Notes

Add this in the review notes field:
```
Test Account: Contact support@toqui.travel for a test account.
The app requires Google Sign-In for authentication.
Location permission is optional — used only in Companion mode for nearby recommendations.
```

## After Submission

1. TestFlight build processes in ~15-30 min
2. Add internal testers in App Store Connect → TestFlight → Internal Testing
3. External testing requires a brief Beta App Review (~24-48 hours)
4. Full App Store submission review takes 24-48 hours typically

## Updating the App

After the first submission, the flow is:

```bash
# 1. Make code changes on main
# 2. Regenerate native project
npx expo prebuild --platform ios --clean
cd ios && pod install

# 3. Build and upload
cd ios
fastlane beta
```

## Troubleshooting

### "No signing certificate" error
Run: `fastlane match appstore` or enable automatic signing in Xcode.

### "Bundle ID not available" error
Someone already registered `travel.toqui.app`. Check your Apple Developer account.

### Pod install fails
```bash
cd ios
export LANG=en_US.UTF-8
pod deintegrate && pod install
```

### Build fails after updating dependencies
```bash
npx expo prebuild --platform ios --clean
cd ios && pod install
```
