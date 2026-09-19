# Kinosail for iPhone, iPad and Apple TV

The active clients are SwiftUI applications for iOS and tvOS, sharing validated Server contracts and native platform services. The apps include Quick Connect and nearby discovery, library browsing and search, collections, movies and episodes, music and audiobooks, AVKit playback, subtitles, bookmarks, progress synchronization, photos and DLNA receiver controls. iPhone and iPad also support verified offline downloads and PDF, EPUB and comic reading.

Only iOS and tvOS application targets remain. iPad belongs to the iOS target. Android, Android TV and Fire TV were removed; no other native platform targets were added. Server and web products are outside this migration.

## Connect to a Server

Install and initialize [Kinosail Player](../../README.md) first. Give the device a reachable HTTPS Server address; simulator or phone `localhost` is not generally the Server host. Use nearby discovery or enter the address, then follow the sign-in or Quick Connect flow. Allow local-network access when connecting on your LAN. Certificate trust, Server reachability, and Viewer permissions still apply.

These instructions build the source clients. They do not imply an App Store release is available. Use the Server's web app when you do not have an installed native build.

## Open and build

From `apps/player/apps/native/`, open `Kinosail.xcodeproj` and select `Kinosail-iOS` or `Kinosail-tvOS`.

```sh
# Simulator apps use local ad-hoc signing so Keychain works. No certificate needed.
./scripts/build-apple.sh ios
./scripts/build-apple.sh tvos

# Device binaries remain unsigned. These commands do not install anything.
./scripts/build-apple.sh ios device
./scripts/build-apple.sh tvos device
```

Output is `.build/<platform>-<simulator|device>/Build/Products/`. Repository aliases are `make -C apps/player client-build-ios` and `make -C apps/player client-build-tvos`.

The project uses Swift 6, strict concurrency, Observation, SwiftUI, App Intents, AVFoundation, AVKit, Network, Security, PDFKit and a protected WebKit reader. The deployment baseline is iOS/tvOS 26; current builds use SDK 27. [RESEARCH.md](RESEARCH.md) records the dated toolchain research. No third-party runtime, Expo prebuild, CocoaPods or package installation is needed.

Siri/Shortcuts on iPhone and iPad offers Search library, Play title and Continue watching. Connect to a Server first; playback by title requires an unambiguous exact match. Apple TV users each connect to their own Viewer Profile. In TV Settings, enable **Show titles on Apple TV Home**, then put Kinosail in the top row to use dynamic Top Shelf. The TV app and `KinosailTopShelf` extension require provisioning with App Group `group.com.kinosail.player` and User Management; unsigned simulator builds do not prove those hardware capabilities.

## Implementation and verification

[IMPLEMENTATION.md](IMPLEMENTATION.md) describes ownership, supported behavior and verification boundaries. [VERIFICATION.md](VERIFICATION.md) records the evidence for this implementation. The former preview gallery and service stubs have been removed.

The retired Expo/React Native client, bridges, patches and browser fixtures have been removed. Swift source and tests live in `Sources/` and `Tests/`. Shared web quality tools live in the repository’s `scripts/quality` package.

Root `.gates-disabled` keeps test and quality suites disabled. Test sources are supplied but must not be run until the user enables gates. Compilation and manual simulator observations are distinct from physical-device and receiver verification.

The Info plists declare `KinosailImplementationState=implemented`. The configured signed-device deployment helper may install these apps when authorized and configured with signing credentials. That marker is not physical-device acceptance or App Store distribution evidence. See [VERIFICATION.md](VERIFICATION.md) for the recorded evidence and remaining boundaries.

## Troubleshooting

- **Cannot connect:** use the Server's reachable HTTPS address, confirm network permission and certificate trust, and check the Server from the same device's browser.
- **Quick Connect expires:** start a new request and approve it from a recently authenticated local or private-management session.
- **Xcode build cannot find an SDK:** select an Xcode installation with iOS/tvOS 27 SDKs using Xcode settings or `xcode-select`; inspect the script's error before changing signing.
- **Unsigned device build will not install:** building with `device` does not sign or provision an app for hardware. Configure an authorized development team in Xcode for physical deployment.

Report the client platform, OS, source commit, Server version, and redacted error through [Support](../../../../SUPPORT.md). Follow [Security](../../../../SECURITY.md) for vulnerability reports.
