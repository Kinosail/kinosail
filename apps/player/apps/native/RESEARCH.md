# Apple client technology research

Checked 2026-09-10 using Apple and Swift primary sources. This note covers only native iOS/tvOS clients.

## What cutting edge means now

Apple released **Xcode 27 RC (27A266a), iOS/iPadOS 27 RC and tvOS 27 RC on September 9, 2026**. These are release candidates. The current Xcode requirements table lists Swift 6.4 in Xcode 27 RC and requires macOS Tahoe 26.6 or later. [Apple releases](https://developer.apple.com/news/releases/), [Xcode requirements](https://developer.apple.com/xcode/system-requirements/).

This Mac has Xcode 26.6 (17F113), Apple Swift 6.3.3 and the iOS/tvOS 26.5 SDKs. Swift's release announcement confirms that Xcode 26.6 includes Swift 6.3.3. These are the tools used for the initial scaffold builds; **Xcode 27 compilation has not been performed**. [Swift 6.3.3 announcement](https://forums.swift.org/t/announcing-swift-6-3-3/87888).

## Choices in the scaffold

- Use Swift 6 language mode with complete concurrency checking and approachable-concurrency settings. Keep view state explicitly on `@MainActor`, use Observation for state changes and actors for service ownership. Swift 6.4 is the next toolchain; do not invent a `SWIFT_VERSION=6.4` language-mode setting. Apple's table distinguishes compiler version from Swift 6 language mode. [Xcode requirements](https://developer.apple.com/xcode/system-requirements/), [Swift 6.4 release process](https://forums.swift.org/t/swift-6-4-release-process/85421).
- Use SwiftUI `Tab`, `NavigationStack`, adaptive iPad tabs/sidebar, standard controls and standard tvOS focus. Native navigation and controls adopt the system's Liquid Glass treatment. Apple's guidance confines glass to controls/navigation and emphasizes native focus on tvOS. [Adopting Liquid Glass](https://developer.apple.com/documentation/TechnologyOverviews/adopting-liquid-glass), [SwiftUI group lab, WWDC26](https://developer.apple.com/videos/play/wwdc2026/8120/). The user granted complete creative freedom for the apps' visual identity and content hierarchy; the old colors, typography and layouts are optional reference.
- Host `AVPlayerViewController` through SwiftUI for familiar playback controls, tracks, AirPlay and picture-in-picture. Implement authenticated loading with documented APIs and the existing protected transport as reference. `AVURLAssetHTTPCookiesKey` is a documented cookie mechanism; private HTTP-header option strings are not a substitute for a reviewed credential boundary. HLS may issue subresource requests to several paths/hosts, so origin and cookie scope need explicit tests. [AVURLAsset HTTP cookies](https://developer.apple.com/documentation/avfoundation/avurlassethttpcookieskey).
- Use current SDKs when installed, with an iOS/tvOS 26 deployment baseline for the shared modern UI. Do not install a new Xcode or upgrade macOS as part of this scaffold task. SDK 27's new authenticated `AsyncImage` request initializers are a future option; the current scaffold provides an `ArtworkLoader` seam and makes no claim to have tested those APIs. [tvOS 27 release notes](https://developer.apple.com/documentation/tvos-release-notes/tvos-27-release-notes).

These are architecture choices for the rewrite, not evidence that the stubbed features are implemented or that any hardware playback capability is certified.
