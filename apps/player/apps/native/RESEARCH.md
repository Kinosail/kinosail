# Apple client technology research

Updated 2026-09-19 using Apple and Swift primary sources. This note covers only native iOS/tvOS clients.

## What cutting edge means now

This Mac now reports **Xcode 27.0 (27A266a)** and has iOS/iPadOS/tvOS 27 SDKs and simulator runtimes. Both application targets compile with SDK 27 while retaining the iOS/tvOS 26 deployment baseline. Apple's requirements table distinguishes the Swift 6 language mode from the Swift 6.4 compiler. [Xcode requirements](https://developer.apple.com/xcode/system-requirements/).

The initial scaffold used Xcode 26.6. That historical toolchain is no longer the current verification boundary; see [VERIFICATION.md](VERIFICATION.md) for SDK 27 build and runtime evidence. Building successfully does not certify physical playback, user switching, Siri, accessibility or App Store acceptance.

## Choices in the scaffold

- Use Swift 6 language mode with complete concurrency checking and approachable-concurrency settings. Keep view state explicitly on `@MainActor`, use Observation for state changes and actors for service ownership. Do not invent a `SWIFT_VERSION=6.4` language-mode setting. [Xcode requirements](https://developer.apple.com/xcode/system-requirements/).
- Use SwiftUI `Tab`, `NavigationStack`, adaptive iPad tabs/sidebar, standard controls and standard tvOS focus. Native navigation and controls adopt the system's Liquid Glass treatment. Apple's guidance confines glass to controls/navigation and emphasizes native focus on tvOS. [Adopting Liquid Glass](https://developer.apple.com/documentation/TechnologyOverviews/adopting-liquid-glass), [SwiftUI group lab, WWDC26](https://developer.apple.com/videos/play/wwdc2026/8120/). The user granted complete creative freedom for the apps' visual identity and content hierarchy; the old colors, typography and layouts are optional reference.
- Host `AVPlayerViewController` through SwiftUI for familiar playback controls, tracks, AirPlay and picture-in-picture. Implement authenticated loading with documented APIs and the existing protected transport as reference. `AVURLAssetHTTPCookiesKey` is a documented cookie mechanism; private HTTP-header option strings are not a substitute for a reviewed credential boundary. HLS may issue subresource requests to several paths/hosts, so origin and cookie scope need explicit tests. [AVURLAsset HTTP cookies](https://developer.apple.com/documentation/avfoundation/avurlassethttpcookieskey).
- Use current SDKs when installed, with an iOS/tvOS 26 deployment baseline for the shared modern UI. Do not install a new Xcode or upgrade macOS as part of this scaffold task. SDK 27's new authenticated `AsyncImage` request initializers are a future option; the current scaffold provides an `ArtworkLoader` seam and makes no claim to have tested those APIs. [tvOS 27 release notes](https://developer.apple.com/documentation/tvos-release-notes/tvos-27-release-notes).

## System integrations

- Top Shelf uses a separate extension and a bounded, opt-in App Group snapshot. It never receives account credentials or contacts the Server. Play/detail links carry only a scoped item identifier, and the app reauthorizes the item. [TVTopShelfContentProvider](https://developer.apple.com/documentation/tvservices/tvtopshelfcontentprovider).
- Apple TV user separation uses `runs-as-current-user-with-user-independent-keychain` on both app and extension. Default Keychain access remains per-user; Viewer Profile tokens must not opt into the user-independent Keychain. Each TV user connects separately. This replaces the older manual user-identifier mapping API. [Apple's multi-user sample](https://developer.apple.com/documentation/tvservices/mapping-apple-tv-users-to-app-profiles).
- iOS App Intents require authentication and foreground execution; they resolve the app's session through `AppDependencyManager`. Do not use `OpenURLIntent` for `kinosail://` links: it requires universal links. [OpenURLIntent](https://developer.apple.com/documentation/appintents/openurlintent), [supportedModes](https://developer.apple.com/documentation/appintents/appintent/supportedmodes).
- A layered iOS icon remains a separate pending deliverable. Icon Composer's first-use license agreement requires user acceptance. Existing iOS icons and layered tvOS assets remain intact. [Icon Composer workflow](https://developer.apple.com/documentation/xcode/creating-your-app-icon-using-icon-composer).
