# Plex and Jellyfin parity for Kinosail

Checked 2026-08-22 against current official documentation, official repositories, and this working tree. This is a practical self-hosted parity map, not a claim that every Plex/Jellyfin endpoint, plugin, hosted service, or client is reproduced.

## Product boundary

Kinosail adopts the mature owner-hosted workflows shared by Plex Media Server and Jellyfin while preserving its recorded architecture:

- one API-driven Kinosail Server container, with every web capability also exposed through `/api/v1` ([ADR 0005](../adr/0005-expose-complete-api-from-one-server-container.md));
- complete local scanning, playback, transcoding, and administration without a subscription; and
- Owner-managed direct remote access through public HTTPS or independently paired WireGuard.

Plex combines a personal server with Plex accounts and an optional media Relay ([Plex remote access](https://support.plex.tv/articles/200289506-remote-access/); [Plex Relay](https://support.plex.tv/articles/216766168-accessing-a-server-through-relay/)). Jellyfin uses local users and operator-managed networking ([Jellyfin users](https://jellyfin.org/docs/general/server/users/); [Jellyfin networking](https://jellyfin.org/docs/general/post-install/networking/)). Kinosail intentionally follows the local-first model and does not implement a media relay.

## Current capability matrix

| Area | Kinosail implementation | Remaining boundary |
| --- | --- | --- |
| Setup and administration | Owner setup, local Viewer Profiles/passkeys, typed Library roots, scans, settings, cache/session/probe/transcode diagnostics, backup/restore, and mutation audit through shared web/API operations | Additional Owners, managed TLS, recovery codes, and upgrade orchestration are not part of the current baseline |
| Libraries and scanning | Movie, Show, Music, Photo, Book, and Mixed scanning; periodic, manual, and filesystem-change refresh; deletion reconciliation; grouping for Shows, albums, photo albums, and books | Multi-version merge/split and a rich folder-management UI remain optional refinements ([Jellyfin libraries](https://jellyfin.org/docs/general/server/libraries/)) |
| Metadata and artwork | Local NFO/sidecars, embedded audio tags, owner corrections, TMDb movie/Show/season/Episode/cast enrichment, role-specific poster/backdrop/logo/season art, WebP thumbnails, chapters, lyrics, and named intro/recap/commercial/outro/credit markers | Provider ordering/language, extras/trailers, trickplay, and content analysis for untagged markers are later depth |
| Browse and discovery | Search, filtering/sorting/pagination, people, artists/albums/discs, photo timeline/albums/slideshow, books, recommendations, manual/smart collections, playlists, persistent queues, shuffle, Continue Watching, history, ratings, and My List | Recommendations are deliberately transparent/local rather than an opaque hosted model ([Plex Library view](https://support.plex.tv/articles/200392126-using-the-library-view/)) |
| Playback and transcoding | Probe-backed direct play, remux, audio-only/full HLS transcode, bitrate policy, two-job admission control, hardware encoder detection with software fallback, HDR-to-SDR tone mapping, audio boost, normalization, track choice, and recovery diagnostics | Hardware/GPU combinations and HDR output require physical certification ([Jellyfin hardware acceleration](https://jellyfin.org/docs/general/post-install/transcoding/hardware-acceleration/)) |
| Audio, subtitles, and markers | Sidecar and extracted embedded text subtitles; probed audio/subtitle streams; per-Viewer language/default rules; selectable audio; chapters and marker skip controls | Image-subtitle burn-in and device-specific subtitle behavior remain unsupported ([Jellyfin codec support](https://jellyfin.org/docs/general/clients/codec-support/)) |
| Viewer state and permissions | Per-Viewer progress, watched state, bounded private history, ratings, lists/playlists/queues/collections, session revocation, and Library/rating/download/remote/transcode/bitrate/Live-TV controls | Cross-server hosted state sync is intentionally absent ([Jellyfin user management](https://jellyfin.org/docs/general/server/users/adding-managing-users/)) |
| Downloads | Authorized immutable source snapshots, content-derived identity, SHA-256 digest, strong ETag, exact byte ranges and empty-body completion `416`, safe paths/server identity, and Android WebP artwork | Real Android/iOS interruption, 10 GiB, airplane-mode, and source-replacement certification; progressive iOS transcoded downloads are not implemented ([mobile contract](mobile-offline-downloads.md)) |
| Live TV and DVR | M3U/XMLTV channel/guide ingestion, Viewer permission, live streaming, stream limits, conflict-aware schedules, FFmpeg recording, web/API controls, and basic Jellyfin Live TV adapters | HDHomeRun discovery, plugin tuners, physical tuner validation, and commercial detection are outside this slice ([Jellyfin Live TV](https://jellyfin.org/docs/general/server/live-tv/)) |
| Synchronized playback | Bundled-web/API SyncPlay groups with join codes, membership, versioned state, and long polling | This is not Jellyfin WebSocket SyncPlay and is not advertised as such |
| Operations | Scheduled encrypted backups with retention, validated restore, scan/transcode state, token-safe diagnostics, mutation audit, and exactly one supported Server container | Real upgrade/rollback drills and external backup recovery remain release exercises ([Jellyfin backup/restore](https://jellyfin.org/docs/general/administration/backup-and-restore/)) |
| Remote access | Local HTTPS policy, optional signed Direct Connection discovery, and user-managed WireGuard pairing without a hosted media hop | Real off-LAN NAT/CGNAT certification is external; networks that require a relay are intentionally unsupported |

## Selective Jellyfin compatibility

The compatibility adapter covers authentication, item/show hierarchy, role-specific artwork, `PlaybackInfo`, direct/remux/transcode playback, tracks and text subtitles, chapters/segments, progress/watched/favorites, immutable original downloads, and a basic Live TV guide/timer surface. Automated protocol coverage is not physical-device certification.

It deliberately does not claim Quick Connect, automatic LAN discovery, Jellyfin WebSocket/session control, Jellyfin SyncPlay, Kodi Library Mode, DLNA/casting, or universal music/photo/playlist/collection compatibility. Selective open-client compatibility is the target; exact Plex protocol/client compatibility is not.

## Intentional differences

The following do not block Kinosail's practical parity claim:

1. Plex's ad-supported catalog, rentals, Discover aggregation, social graph, and universal hosted watchlist, which are hosted catalog/social products rather than owner-hosted server features ([Plex overview](https://support.plex.tv/articles/200288286-what-is-plex/)).
2. Subscription gates on local downloads, hardware acceleration, DVR, marker skipping, or other playback quality. Local use remains complete without a subscription ([Plex Pass overview](https://support.plex.tv/articles/201751006-plex-pass-feature-overview/)).
3. A Kinosail-operated media relay, forbidden by the Library Content boundary.
4. Matching Jellyfin's plugin count before repeated integrations justify a stable extension seam ([Jellyfin plugins](https://jellyfin.org/docs/general/server/plugins/)).
5. Face/location analysis, opaque recommendation telemetry, hosted viewing analytics, and sonic similarity as baseline requirements; these are privacy-sensitive or specialist features.

## Verification boundary

Repository tests can prove shared web/API operations, authorization, protocol response semantics, one-container composition, and browser behavior. They cannot certify physical phones/TVs/tuners/GPUs or real off-LAN paths. Release certification still requires:

- official Android and iOS plus Swiftfin runs, including downloads after network interruption, process restart, airplane mode, permission revocation, a 10 GiB transfer, and source replacement;
- representative VideoToolbox, VAAPI, QSV, and NVENC hardware/HDR fixtures;
- long-running tuner/DVR tests; and
- direct HTTPS/WireGuard playback across representative home NAT and CGNAT networks.

Accordingly, the defensible claim is: **the locally achievable practical parity backlog is implemented and protocol-tested; universal hardware, network, and third-party-client certification remains external evidence, not missing server code.**
