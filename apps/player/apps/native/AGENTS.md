# Apple Swift client

- The active native build is `Kinosail.xcodeproj`. Its application targets are iOS (iPhone/iPad), tvOS and a companion watchOS playback remote. Mac Catalyst, iPhone/iPad-on-Mac, visionOS and Android are not targets.
- For native feature changes, use `IMPLEMENTATION.md` and `VERIFICATION.md`. Reuse the existing `Sources/Core`, `Sources/Services`, `Sources/Platform` and feature-screen ownership. Keep build, simulator, and physical-device evidence distinct.
- For native UI changes, follow the current request and preserve API compatibility, privacy, security, and accessibility.
- The previous Expo/React Native implementation, configuration, bridges and tests have been removed. Keep native changes in `Sources/`, `Tests/`, the Xcode project and its resources. Shared JavaScript quality tools belong in the repository’s `scripts/quality` package.
- The web apps and all server implementation are outside this migration's scope. Existing `/api/v1` contracts are authoritative; do not change the server to fit an invented Swift schema.
- Never return fabricated server state, empty success, saved-state success or a fake media URL to make a feature look done. Keep disposable fixtures outside production app targets.
- `Core` owns input and response validation. `ServerClient` owns HTTP, origin and credentials. UI state lives on `@MainActor`; asynchronous services and mutable persistent state use actors. Do not make the whole app unchecked Sendable or globally disable Swift concurrency checking.
- Build from this directory with `./scripts/build-apple.sh ios`, `./scripts/build-apple.sh tvos` and `./scripts/build-apple.sh watchos`. Simulators use local ad-hoc signing for Keychain. Add `device` for unsigned device binaries. No personal signing certificate, CocoaPods, Expo prebuild, JavaScript runtime or package installation is needed.
- If root `.gates-disabled` exists, report paused test and quality suites as unrun. Otherwise run the applicable checks and preserve their thresholds.
- `Configuration/*-Info.plist` declares `KinosailImplementationState=implemented` so the configured signed-device deployment helper can install these clients. Keep installation evidence separate from physical feature checks.
