# Kinosail for iPhone, iPad and Apple TV

The active clients are SwiftUI applications for iOS and tvOS, sharing validated Server contracts and native platform services. The apps include Quick Connect and nearby discovery, library browsing and search, collections, movies and episodes, music and audiobooks, AVKit playback, subtitles, bookmarks, progress synchronization, photos and DLNA receiver controls. iPhone and iPad also support verified offline downloads and PDF, EPUB and comic reading.

Only iOS and tvOS application targets remain. iPad belongs to the iOS target. Android, Android TV and Fire TV were removed; no other native platform targets were added. Server and web products are outside this migration.

## Open and build

Open `Kinosail.xcodeproj` and select `Kinosail-iOS` or `Kinosail-tvOS`.

```sh
# Simulator apps use local ad-hoc signing so Keychain works. No certificate needed.
./scripts/build-apple.sh ios
./scripts/build-apple.sh tvos

# Device binaries remain unsigned. These commands do not install anything.
./scripts/build-apple.sh ios device
./scripts/build-apple.sh tvos device
```

Output is `.build/<platform>-<simulator|device>/Build/Products/`. Repository aliases are `make -C apps/player client-build-ios` and `make -C apps/player client-build-tvos`.

The project uses Swift 6, strict concurrency, Observation, SwiftUI, AVFoundation, AVKit, Network, Security, PDFKit and a protected WebKit reader. The deployment baseline is iOS/tvOS 26. It builds with the selected installed Xcode SDK; [RESEARCH.md](RESEARCH.md) records the dated toolchain research. No third-party runtime, Expo prebuild, CocoaPods or package installation is needed.

## Implementation and verification

[IMPLEMENTATION.md](IMPLEMENTATION.md) describes ownership, supported behavior and verification boundaries. [VERIFICATION.md](VERIFICATION.md) records the evidence for this implementation. The former preview gallery and service stubs have been removed.

The previous Apple `src/`, `modules/` and JavaScript tooling remain reference material, outside the Xcode app targets. Their tests do not certify the Swift implementation. Removing shared legacy quality dependencies would require work beyond the native app scope; existing Expo prebuilds remain blocked.

Root `.gates-disabled` keeps test and quality suites disabled. Test sources are supplied but must not be run until the user enables gates. Compilation and manual simulator observations are distinct from physical-device and receiver verification.

The Info plists retain `KinosailImplementationState=scaffold` as the existing distribution hold until the required verification and device-distribution authorization are complete. The deployment helper still skips these builds. This work does not select personal signing credentials, upload a release, or install on physical devices.
