# Share one native client source across mobile and television

**Superseded:** the active clients are Swift iOS/iPadOS and tvOS apps. The Expo implementation was removed on September 15, 2026. See [current native ownership](../../apps/native/IMPLEMENTATION.md). The original decision follows for historical context.

Kinosail Player uses one Expo and React Native source tree under `apps/native`. The `react-native-tvos` fork generates iOS, iPadOS, tvOS, Android, Android TV, and Fire TV application families. Expo prebuild generates separate mobile and television projects from the same source. Generated projects do not enter version control.

The shared source owns versioned API access, Quick Connect, session storage rules, client state, design tokens, assets, routes, and most screen composition. Platform controls own focus, remote input, secure storage, picture-in-picture, and playback. Apple builds use the Apple playback stack. Android builds use the Android playback stack.

Playback follows Direct First. The client uses only a server-approved original media URL. It reports an unsupported result when the native engine rejects that file. It does not silently request a transcode or add a decoder fallback.

## Consequences

One product change can reach every first-release device family. Mobile and television still produce separate signed artifacts. Each family needs its own native compile, store review, and physical-device checks. Codec support remains device-specific. The Server web interface remains the browser client. Samsung, LG, and Roku clients can share contracts and visual rules, but they require separate platform shells.
