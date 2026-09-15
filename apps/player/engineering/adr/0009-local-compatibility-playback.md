# Keep React Native and add an Apple local compatibility decoder

Status: accepted for implementation, 2026-09-07. Supersedes the platform-only decoder restriction in ADR 0008. React Native remains the shared interface for mobile and TV.

The platform player is still the first path for system integration, energy efficiency, HDR, and Picture in Picture. When that decoder fails, iOS and tvOS can open the same original file through the local compatibility engine. Buffering alone never selects server conversion. The Viewer must explicitly choose an offered compatible server stream after local recovery fails.

The Apple module pins VideoLAN's unified VLCKit `4.0.0a24` (August 31, 2026). This is a prerelease, selected in response to the request for cutting-edge format support. Stable TVVLCKit 3.7.3 bundles FFmpeg 4.4.5 and does not provide the requested VVC foundation. The newer binary contains a VVC decoder; this is not evidence of correct playback of every profile, HDR mode, audio route, disc layout, or subtitle combination. Its binary archive SHA-256 is `c61a42052ec4c1315325fba81f8893f4ccf639d92bf61dd1b3c37c3a2f26b8e3`.

A small Expo module implements the native view and an ephemeral loopback transport. The transport accepts one protected media source, binds only to 127.0.0.1, uses an unpredictable per-playback URL, bounds request headers and simultaneous streams, forwards only GET/HEAD and a single byte range, rejects redirects, and keeps authorization in URLSession headers. It writes no media cache or credentials to disk. This preserves Kinosail's existing session and media authorization without adding bearer tokens to URLs or adopting a second authentication scheme. Closing playback releases the source and connections.

The native view uses VideoLAN's PiP drawable protocol. The system PiP controller owns the floating window, and calls back into the same player's pause, play and seek operations. Background playback is enabled in the app configuration. Apple TV shares the decoder and React controls; focus guides remember shelf entry points. Android and browser playback retain the platform engine until a supported local transport and decoder adapter exist there.

Source facts exposed by `/api/v1/items/{id}/playback` reuse the existing normalized media contract. A filename extension establishes ingestion only. Measurement records are session-local, bounded, and omit media identifiers and credentials.

Upstream references: [VLCKit](https://code.videolan.org/videolan/VLCKit), [binary archive](https://download.videolan.org/cocoapods/unstable/VLCKit-4.0-20260831-1526.zip), [Expo TV compatibility](https://docs.expo.dev/guides/building-for-tv/). Dependency boundaries remain Expo 57 / React Native TV 0.86: RN TV 0.87 is newer but outside the stable Expo matrix. TypeScript 7 and ESLint 10 also exceed some installed tooling peer ranges; do not override those ranges to claim a newer stack.

Verification commands, only after gates are explicitly enabled: native `pnpm verify`, Player `make verify-changed`, and the benchmark protocol. While `.gates-disabled` exists these remain unrun. Operational app builds and manual rendering are reported separately from quality-suite, physical-device, and format-parity evidence.
