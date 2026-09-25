# Swift Apple implementation

The buildable scaffold has been replaced by functional Swift clients. All retained service operations call the existing Server APIs; no production screen depends on fixture data or fabricated success. The new design uses native tabs/sidebar navigation, artwork shelves, system typography, progressive settings and AVKit controls. Recreating the former clients is not a requirement.

## Ownership

- `Sources/Core`: bounded JSON, normalized input, validated Server and persistence contracts, media timelines, subtitle parsing and queue rules.
- `Sources/Services`: the ephemeral, origin-confined HTTP actor and API operations. Account credentials stay in headers and Keychain; redirects are rejected.
- `Sources/Platform`: Keychain, Bonjour, playback transport and coordination, Now Playing, artwork, persistent progress, downloads and protected reading.
- `Sources/Features`: main-actor screen state and native controls. Shared media views live in `Sources/Design`.
- `Tests`: Swift Testing regressions for contract failures, rejected input without network or file effects, timelines, transport, progress, downloads, reading, casting and subtitles. These sources have not been executed while gates are disabled.

The Xcode synchronized groups include new Swift files automatically. `Kinosail-iOS` and `Kinosail-tvOS` remain separate targets. Neither bundles React Native, Expo, Hermes, the old Apple modules, or a JavaScript application runtime.

## Implemented behavior

| Area | Behavior |
| --- | --- |
| Session | Manual Server address, bounded Bonjour discovery, local QR generation, six-digit Quick Connect, cancellation/expiry, device approval, Keychain restoration and sign-out. Cached viewer identity permits access to existing downloads when the Server is unreachable. |
| Library | Home, Continue watching, recent media, search, filters, sorting, paging, letter jumps, details, My List, collections, shows/seasons, albums and music queues. |
| Playback | Native AVKit controls, capability hints from Apple APIs, direct playback first, Server-compatible HLS after supported format failures, opt-in dialog boost and loudness normalization through effect-specific compatible streams, canonical timeline seeking, chapters, markers, next episode, speed and track selection. |
| Audio | Album/track queues, shuffle/repeat, sleep timer, mini player, background audio, Now Playing, remote commands, interruption and route-change handling. |
| Subtitles | Embedded native tracks plus bounded external WebVTT captions rendered as plain text. Language and title preferences are applied. External captions appear in the open app, not system Picture in Picture. |
| Progress | Viewer-scoped protected journal, local-first writes, Server compare-and-set synchronization and explicit conflict resolution. Bookmarks support playback and reading positions. |
| Downloads, iOS | Server preparation, original/compatible/video-size/audio choices, track selection, season downloads, verified chunk resume, whole-file SHA-256, an actual AVFoundation decode probe before Ready, pause/resume/delete, offline playback, Wi-Fi and quota settings, optional next episodes and watched-file cleanup. |
| Reading, iOS | Native PDFKit; EPUB and comic resources through a nonpersistent WebKit view with JavaScript disabled and same-book resource isolation. Contents, bookmarks, reading position, font/theme preferences and explicit position-conflict choices. |
| Photos | Bounded authenticated loading, pinch/double-tap zoom and pan on iOS; zoom and directional movement with the TV remote. |
| Receivers | Explicit DLNA discovery, start/status/play/pause/seek/stop and ticket revocation. iOS exposes Apple's audio route picker and directions for Control Center screen mirroring. |
| Siri and Shortcuts, iOS | Authenticated foreground actions for library search, exact-title playback and the first playable Continue watching title. Ambiguous or truncated exact-title matches are rejected. Actions use the active app session and existing media screens. |
| Apple TV Home | Default-on Top Shelf extension for Continue watching, My List and recently added videos; up to six posters per section. A one-time migration enables Top Shelf only when no preference exists and preserves saved opt-outs. Expired snapshots (24 hours), invalid input and failed refreshes expose no dynamic content. |
| Apple TV users | One connected Viewer Profile is shared by everyone using the TV. Apple TV user switching does not change the Kinosail profile. Tokens remain in the app Keychain; Top Shelf receives only the credential-free snapshot. |
| Apple Watch remote | The paired iPhone relays Watch controls for its local player and active Apple TV players in the same Viewer Profile. The TV publishes bounded state and polls a short-lived Server command queue. Heart graphs require an explicit Watch action and Health read permission, keep readings on the Watch, and leave gaps when samples cannot be mapped to known playback positions. |

## Security and persistence boundaries

HTTP input and remote JSON are bounded before use. The strict decoder rejects duplicate keys, excessive nesting, unknown object fields at consumed contracts, malformed identifiers, invalid numbers, oversized arrays and untrusted media origins. Saved data is revalidated on restoration. No account token is placed in a media, image, reader or QR URL.

AVFoundation reads through a per-playback loopback capability, with bounded HTTP requests, response backpressure and constrained HLS resource rewriting. The capability is unrelated to the account credential. HLS references remain in the same item's Server namespace. Video external playback is disabled because this transport is device-local; use screen mirroring or DLNA for a television.

Artwork uses a bounded decoded memory cache and a protected disk cache scoped by Server origin, Server identity and Viewer Profile. Catalog pages, movies, shows, collections and albums also persist locally: screens display saved content first, reuse catalog responses for five minutes, then refresh visible content in the background. Pull-to-refresh bypasses freshness; progress and My List changes invalidate freshness without removing the saved screen. Cached entries expire after 30 days, with per-profile disk limits of 64 MiB/512 catalog responses and 256 MiB/2,048 images. Sign-out purges the profile cache. Apple may reclaim this disposable cache under storage pressure; it is excluded from backups and does not contain credentials or playback capabilities. Artwork retains at most four fetches and bounded decoded image dimensions.

The download engine stores only verified media and bounded recovery journals in `Documents/kinosail-swift-offline`, separate from the former client's files. Metadata is scoped by normalized Server origin, Server identity and viewer identity. Credentials are kept in memory while authorized; background restoration reads the current Keychain session. Restored tasks and persisted URLs are checked against that authorization before credentials are attached. Signing out locks transfers and access without deleting another profile's downloads. The explicit device-storage reset deletes all Swift download profiles.

Reading resources are restricted to the active book. Each chapter has request, concurrency and byte budgets; PDF and image dimensions are bounded. PDF physical pages map to the Server's single logical PDF page using its offset. The existing reading API lacks atomic compare-and-set, so the client checks before writing and offers conflict choices, but simultaneous writes between those two requests remain a Server-contract limitation.

`Resources/PrivacyInfo.xcprivacy` declares disk-space reason E174.1, app-container metadata reason C617.1 and app-only preferences reason CA92.1. Disk-space values are not transmitted. The Top Shelf extension declares C617.1 for its App Group snapshot metadata. No analytics, tracking SDK or developer data collection was added. Apple documents these reasons in [NSPrivacyAccessedAPIType](https://developer.apple.com/documentation/bundleresources/app-privacy-configuration/nsprivacyaccessedapitypes/nsprivacyaccessedapitype).

`TopShelfShared` owns the credential-free snapshot contract, shared with `TopShelf`. The extension reuses Core's strict JSON parser, rejects unknown/duplicate fields and enforces byte, item, identifier, image and age limits before rendering. Snapshots refresh on foreground/library changes and clear on sign-out, profile change, disabled sharing or refresh failure. Changing connections preserves the sharing preference; an unset preference defaults to on. The app performs a one-time migration from older saved-off defaults, then preserves later opt-outs. Revocation while the app is not running cannot be detected by the offline extension; a cached snapshot expires after 24 hours. Playback always goes through current Server authorization. TV signing must provision both bundle identifiers with App Group `group.com.kinosail.player`. User Management is omitted so the shared-profile app can be provisioned by the configured Personal Team.

## Deliberate limits

- Google Cast has no native sender SDK in these targets and is not offered in the UI. Its existing Server wire protocol remains represented by the validated casting contract.
- Downloads and book reading are iPhone/iPad features. Apple TV uses Server streaming.
- Original media support is determined by AVFoundation and actual device capabilities. Unsupported originals need a compatible Server stream/download. Codec hints and simulator playback do not certify every codec, HDR profile, audio layout or receiver.
- Boost Dialog and Normalize Loudness are profile or title preferences backed by effect-specific Server streams. Enabling either intentionally converts audio and therefore requires transcoding permission and a Server connection; downloaded files retain their existing audio. The legacy volume-boost value remains preserved but is not exposed or applied.
- Observed smart-download completion events are persisted per Viewer Profile. Watched episodes remain until requested replacements are ready; events never observed by the app cannot be reconstructed.
- External captions do not appear in system Picture in Picture. Reader resources and artwork are not downloaded for offline use.

## Verification and delivery

See [VERIFICATION.md](VERIFICATION.md) for the actual evidence, including its limits. Run the ordinary build commands in [README.md](README.md); simulator builds use ad-hoc signing for Keychain and the ordinary device build script stays unsigned. The configured device deployment helper supplies the existing development team for authorized signed installs.

Only after the user enables quality gates, run the Swift test schemes against available simulators:

```sh
xcodebuild -project Kinosail.xcodeproj -scheme Kinosail-iOS \
  -destination 'platform=iOS Simulator,id=<simulator-uuid>' test
xcodebuild -project Kinosail.xcodeproj -scheme Kinosail-tvOS \
  -destination 'platform=tvOS Simulator,id=<simulator-uuid>' test
```

Do not run these while `.gates-disabled` exists. Physical verification still needs representative direct/HLS/HDR playback, interruptions, Picture in Picture, background transfers and recovery, physical PDF/EPUB/comic reading, Siri Remote/VoiceOver, and AirPlay/DLNA receivers.

On 2026-09-12 the user explicitly authorized completing the native fixes and installing on their phone while leaving tests disabled. `KinosailImplementationState=implemented` now permits the configured signed-device deployment. Do not bypass the deployment helper or use legacy Expo prebuilds. Keep signing, installation, release upload, source publication and physical behavior as separate delivery facts.
