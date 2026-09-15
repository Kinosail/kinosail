# Jellyfin and Swiftfin Apple TV playback contract

Research date: 2026-09-01

Primary-source snapshots:

- `jellyfin/jellyfin` at [`4910aafa1a8227a65a037d3d2d299a32691e4de3`](https://github.com/jellyfin/jellyfin/tree/4910aafa1a8227a65a037d3d2d299a32691e4de3)
- `jellyfin/Swiftfin` at [`2a09ef90a5a5081de3286f3bcd4d615c2e3b715f`](https://github.com/jellyfin/Swiftfin/tree/2a09ef90a5a5081de3286f3bcd4d615c2e3b715f)
- `jellyfin/jellyfin-sdk-swift` at [`50be9e583438be414a15d4bba933ff64b6769a91`](https://github.com/jellyfin/jellyfin-sdk-swift/tree/50be9e583438be414a15d4bba933ff64b6769a91). Swiftfin currently pins SDK 3.0.0 at [`37a2f5028bd24689b772effb559f72ec5388f021`](https://github.com/jellyfin/Swiftfin/blob/2a09ef90a5a5081de3286f3bcd4d615c2e3b715f/Swiftfin.xcodeproj/project.xcworkspace/xcshareddata/swiftpm/Package.resolved#L104-L110). The relevant generated models have the same field decoders in both revisions.
- Kinosail tested revision [`0aad41ec988da9516c5c638ac2084e2c0ef49a1d`](https://github.com/MikeO7/kinosail/tree/0aad41ec988da9516c5c638ac2084e2c0ef49a1d) and current deployed revision [`24395e04309687cd0689c2079ca0deb00fd4d3bd`](https://github.com/MikeO7/kinosail/tree/24395e04309687cd0689c2079ca0deb00fd4d3bd).

## Exact Swiftfin flow

1. `MediaPlayerItem.build` refreshes the full item and selects its first `MediaSourceInfo`. It builds a device profile and posts `PlaybackInfoDto` to `POST /Items/{id}/PlaybackInfo`. The request includes `DeviceProfile`, `MediaSourceId`, `MaxStreamingBitrate`, selected audio and subtitle indexes, `UserId`, and `AutoOpenLiveStream`. [Swiftfin request builder](https://github.com/jellyfin/Swiftfin/blob/2a09ef90a5a5081de3286f3bcd4d615c2e3b715f/Shared/Objects/MediaPlayerManager/MediaPlayerItem/MediaPlayerItem%2BBuild.swift#L44-L92), [SDK route](https://github.com/jellyfin/jellyfin-sdk-swift/blob/50be9e583438be414a15d4bba933ff64b6769a91/Sources/Paths/GetPostedPlaybackInfoAPI.swift#L12-L23).
2. Swiftfin selects a returned media source by matching `ETag`, then `OpenToken`, then `Id`, and finally the first source. It requires a non-empty top-level `PlaySessionId`. [Source selection](https://github.com/jellyfin/Swiftfin/blob/2a09ef90a5a5081de3286f3bcd4d615c2e3b715f/Shared/Objects/MediaPlayerManager/MediaPlayerItem/MediaPlayerItem%2BBuild.swift#L94-L130).
3. `MediaPlayerItem.streamURL` gives `TranscodingUrl` absolute priority. It resolves that server-relative path against the configured server URL and returns it unchanged. Without `TranscodingUrl`, normal non-live video uses `/Videos/{itemId}/stream?Static=true&Tag=...&PlaySessionId=...&MediaSourceId=...`. `Path` is only the final fallback for other media or live-stream cases. [Exact URL selection](https://github.com/jellyfin/Swiftfin/blob/2a09ef90a5a5081de3286f3bcd4d615c2e3b715f/Shared/Objects/MediaPlayerManager/MediaPlayerItem/MediaPlayerItem%2BBuild.swift#L187-L238).
4. Native playback creates `AVPlayerItem(url: item.url)` and replaces the current AVPlayer item. [AVPlayer handoff](https://github.com/jellyfin/Swiftfin/blob/2a09ef90a5a5081de3286f3bcd4d615c2e3b715f/Shared/Objects/MediaPlayerManager/MediaPlayerProxy/MediaPlayerProxy%2BAVPlayer.swift#L161-L168).
5. Swiftfin playback creates `VLCVideoPlayer.Configuration(url: item.url)`. It marks transcoding solely by `TranscodingUrl != nil`, then adds eligible subtitle sidecars as playback children. [VLCKit handoff](https://github.com/jellyfin/Swiftfin/blob/2a09ef90a5a5081de3286f3bcd4d615c2e3b715f/Shared/Objects/MediaPlayerManager/MediaPlayerProxy/MediaPlayerProxy%2BVLC.swift#L128-L163). Swiftfin is the default player type. [Default](https://github.com/jellyfin/Swiftfin/blob/2a09ef90a5a5081de3286f3bcd4d615c2e3b715f/Shared/Services/SwiftfinDefaults.swift#L286-L288).

Therefore, a request for the returned `TranscodingUrl` proves that response decoding, media-source selection, and URL construction succeeded. Repeated master requests with no child request locate failure at master-playlist acceptance or variant selection.

## Required response shape and consumption

| JSON field | Exact value or type | Swiftfin use |
|---|---|---|
| `MediaSources` | Array of decodable `MediaSourceInfo` | Required to select a source. An absent or empty array causes item construction to fail. |
| `PlaySessionId` | Non-empty string | Required. Swiftfin throws when absent. |
| `Id` | Stable string matching the item source | Used for source matching and direct-stream `MediaSourceId`. |
| `ETag` | Stable string | Preferred source match and direct-stream `Tag`. |
| `Protocol` | SDK enum spelling such as `Http` | Decoded. It is informational in current playback code. |
| `Type` | SDK enum spelling such as `Default` | Decoded. |
| `Path` | Valid URL string | Used only after `TranscodingUrl` and normal non-live video handling do not apply. |
| `TranscodingUrl` | Valid relative or absolute playback URL | Decisive field. Its presence selects transcoding and its URL goes directly to AVPlayer or VLCKit. |
| `TranscodingSubProtocol` | Lowercase `hls` or `http` | Exact SDK key and enum spelling. Current Swiftfin displays it but does not use it to choose the player URL. |
| `TranscodingContainer` | String such as `mp4` | Decoded, but current Swiftfin does not use it to construct playback. |
| `MediaStreams` | Decodable video, audio, and subtitle stream array | Builds track lists, defaults, and Jellyfin-to-player index mapping. Transcoded playback assumes one video and one selected audio track. |
| `DefaultAudioStreamIndex` / `DefaultSubtitleStreamIndex` | Stream indexes or `-1` for no subtitle | Select initial tracks. |

The exact SDK keys are case-sensitive: `TranscodingUrl`, `TranscodingSubProtocol`, `TranscodingContainer`, `Path`, and `MediaStreams`. [MediaSourceInfo decoder](https://github.com/jellyfin/jellyfin-sdk-swift/blob/50be9e583438be414a15d4bba933ff64b6769a91/Sources/Entities/MediaSourceInfo.swift#L157-L204), [top-level decoder](https://github.com/jellyfin/jellyfin-sdk-swift/blob/50be9e583438be414a15d4bba933ff64b6769a91/Sources/Entities/PlaybackInfoResponse.swift#L26-L31). Enum-backed fields must use exact values because an unknown value fails Codable decoding.

For an external text subtitle, Swiftfin needs a subtitle stream with `Type: "Subtitle"`, `DeliveryMethod: "External"`, a non-empty `DeliveryUrl`, and `IsTextSubtitleStream: true`. VLCKit resolves the delivery URL against the server and loads it as a subtitle child. [MediaStream decoder](https://github.com/jellyfin/jellyfin-sdk-swift/blob/50be9e583438be414a15d4bba933ff64b6769a91/Sources/Entities/MediaStream.swift#L276-L345), [sidecar filter](https://github.com/jellyfin/Swiftfin/blob/2a09ef90a5a5081de3286f3bcd4d615c2e3b715f/Shared/Extensions/JellyfinAPI/MediaStream.swift#L364-L369), [URL construction](https://github.com/jellyfin/Swiftfin/blob/2a09ef90a5a5081de3286f3bcd4d615c2e3b715f/Shared/Extensions/JellyfinAPI/MediaStream.swift#L21-L33). Native AVPlayer does not add those sidecar children.

## Official Jellyfin HLS contract

The official server evaluates the posted device profile, assigns a play session, and sets `TranscodingUrl`, `TranscodingContainer`, and `TranscodingSubProtocol` when direct play is unavailable. [Controller flow](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/Jellyfin.Api/Controllers/MediaInfoController.cs#L116-L218), [device-specific result](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/Jellyfin.Api/Helpers/MediaInfoHelper.cs#L267-L348).

For HLS video, `StreamInfo.ToUrl` returns `/videos/{id}/master.m3u8` with `MediaSourceId`, codecs, bitrates, `SegmentContainer`, `SegmentLength`, `MinSegments`, `PlaySessionId`, and `ApiKey` as applicable. [URL generator](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/MediaBrowser.Model/Dlna/StreamInfo.cs#L877-L909), [HLS and session query](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/MediaBrowser.Model/Dlna/StreamInfo.cs#L997-L1036).

The request chain is:

```text
/videos/{id}/master.m3u8?...       master
  -> main.m3u8?...                 variant playlist
     -> hls1/main/-1.mp4?...       fMP4 initialization segment
     -> hls1/main/0.mp4?...        media segment 0
     -> hls1/main/1.mp4?...        media segment 1
```

The master uses `#EXT-X-STREAM-INF` with bandwidth, precise `CODECS`, resolution, frame rate, and optional subtitles. [Master construction](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/Jellyfin.Api/Helpers/DynamicHlsHelper.cs#L187-L231), [variant declaration](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/Jellyfin.Api/Helpers/DynamicHlsHelper.cs#L345-L374). The variant is HLS version 7 for fMP4, is `VOD`, contains `#EXT-X-MAP`, six-decimal `#EXTINF` values, and ends with `#EXT-X-ENDLIST`. [Playlist generator](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/src/Jellyfin.MediaEncoding.Hls/Playlist/DynamicHlsPlaylistGenerator.cs#L34-L106). Text subtitles can be declared through `#EXT-X-MEDIA` and separate subtitle playlists. [HLS subtitles](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/Jellyfin.Api/Helpers/DynamicHlsHelper.cs#L674-L709).

## Kinosail comparison and highest-confidence defects

1. **Confirmed response-key defect in the tested revision.** Revision `0aad41e` returned `TranscodingProtocol` instead of the SDK's exact `TranscodingSubProtocol` key. [Tested response](https://github.com/MikeO7/kinosail/blob/0aad41ec988da9516c5c638ac2084e2c0ef49a1d/internal/server/jellyfin_playback.go#L177-L191). Current `main` and the current healthy Nox container at `31c67b3` correct the key. [Current response](https://github.com/MikeO7/kinosail/blob/31c67b363501680c4f18bb01840620a7898ceeba/internal/server/jellyfin_playback.go#L177-L191). This was a real contract defect, but current Swiftfin does not use the subprotocol field to select the URL. It cannot alone explain a trace that already requested `TranscodingUrl`.
2. **Confirmed strict-decoding defect for some HDR sources.** Kinosail normalizes internal values with `strings.ToUpper`, producing `DOLBY-VISION` and `HDR10+`. [Kinosail projection](https://github.com/MikeO7/kinosail/blob/31c67b363501680c4f18bb01840620a7898ceeba/internal/server/jellyfin_playback.go#L195-L202). The SDK only accepts `DOVI` and `HDR10Plus`, plus its other exact enum spellings. [SDK enum](https://github.com/jellyfin/jellyfin-sdk-swift/blob/50be9e583438be414a15d4bba933ff64b6769a91/Sources/Entities/VideoRangeType.swift#L11-L26). Such a source can make the whole `MediaSourceInfo` fail decoding before playback begins. Plain SDR, HLG, and HDR10 do not hit this mismatch.
3. **Confirmed native-subtitle contract gap.** Swiftfin native advertises VTT with HLS delivery and enables subtitles in the manifest. [Native profile](https://github.com/jellyfin/Swiftfin/blob/2a09ef90a5a5081de3286f3bcd4d615c2e3b715f/Shared/Objects/VideoPlayerType/VideoPlayerType%2BNative.swift#L107-L164). Kinosail only interprets `External` and `Embed` subtitle profiles and always projects text subtitles as external sidecars. [Capability parsing](https://github.com/MikeO7/kinosail/blob/31c67b363501680c4f18bb01840620a7898ceeba/internal/server/jellyfin_playback.go#L100-L123), [subtitle projection](https://github.com/MikeO7/kinosail/blob/31c67b363501680c4f18bb01840620a7898ceeba/internal/server/jellyfin_playback.go#L155-L217). It does not generate the official `#EXT-X-MEDIA` subtitle contract. This is a definite native AVPlayer subtitle failure, although it does not explain base video startup with subtitles disabled.
4. **The live trace localizes the remaining base-video fault to the master playlist.** The trace recorded a successful `PlaybackInfo`, then repeated 200 responses for `source-master`, with no `source-variant`, init, or segment requests. Swiftfin's source proves it only reaches that request after decoding a source and choosing `TranscodingUrl`. The failure is therefore after JSON selection and before child playlist loading. Item mapping, `Path`, segment routing, and segment bytes are not the active failure in that trace.
5. **Kinosail's HLS shape differs from the official server and its tests validate only that custom shape.** Kinosail returns `/Videos/{id}/p/{recipe}/index.m3u8`, then `{quality}/index.m3u8`, `init.mp4`, and `segment-00000.m4s`. [Response URL](https://github.com/MikeO7/kinosail/blob/31c67b363501680c4f18bb01840620a7898ceeba/internal/server/jellyfin_playback.go#L188-L191), [master format](https://github.com/MikeO7/kinosail/blob/31c67b363501680c4f18bb01840620a7898ceeba/internal/server/hls_playlist.go#L85-L99), [custom-chain test](https://github.com/MikeO7/kinosail/blob/31c67b363501680c4f18bb01840620a7898ceeba/internal/server/jellyfin_hls_delivery_test.go#L10-L42). Different URI names are valid HLS and are not, by themselves, a defect. However, a passing custom-chain test does not prove AVPlayer or VLCKit accepts the master. The observed no-child trace is the decisive missing evidence.

The next diagnostic must capture the exact served master bytes and the Swiftfin/AVPlayer or VLCKit error for one play session. Route success alone cannot distinguish a playlist parse rejection from a codec-variant rejection.

## Deep Jellyfin HLS comparison

This section pins official Jellyfin `master` at [`4910aafa1a8227a65a037d3d2d299a32691e4de3`](https://github.com/jellyfin/jellyfin/commit/4910aafa1a8227a65a037d3d2d299a32691e4de3). `git ls-remote` and a shallow checkout returned this commit on 2026-09-01. The compared Kinosail server and Nox container use [`24395e04309687cd0689c2079ca0deb00fd4d3bd`](https://github.com/MikeO7/kinosail/tree/24395e04309687cd0689c2079ca0deb00fd4d3bd).

### Corrected live failure boundary

The latest physical Apple TV attempt supersedes the earlier master-only trace. One play session produced this sequence on Nox:

```text
15:52:55.650  POST PlaybackInfo                 200
15:52:56.503  HLS transcode started
15:52:58.779  GET planned master                200  1317 bytes  application/vnd.apple.mpegurl
15:52:59.103  GET planned variant               200   406 bytes  application/vnd.apple.mpegurl
15:52:59.109  GET initialization segment        200  1376 bytes  video/mp4
15:52:59.142  GET media segment 0               200 133624 bytes  application/octet-stream
15:52:59.160  GET planned variant               200   406 bytes  application/vnd.apple.mpegurl
15:52:59.172  GET media segment 1               200 245453 bytes  application/octet-stream
```

Swiftfin sent no `Sessions/Playing`, progress, or stopped event after these requests. The player therefore failed after it selected a variant and fetched the input objects, but before playback started. The active boundary is now the media playlist, response metadata, or fragmented MP4 content. It is not item mapping, `PlaybackInfo` decoding, master routing, or variant routing. Kinosail records these safe route classes in [`observability.go`](https://github.com/MikeO7/kinosail/blob/24395e04309687cd0689c2079ca0deb00fd4d3bd/internal/server/observability.go#L28-L90) and [`observability.go`](https://github.com/MikeO7/kinosail/blob/24395e04309687cd0689c2079ca0deb00fd4d3bd/internal/server/observability.go#L134-L176).

### Complete official VOD flow

| Stage | Official Jellyfin behavior | Kinosail behavior at `24395e0` |
|---|---|---|
| `POST PlaybackInfo` | The controller accepts body and query compatibility fields. It resolves the item and calls `GetPlaybackInfo`. It applies the posted device profile to every returned source. [`MediaInfoController.cs`](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/Jellyfin.Api/Controllers/MediaInfoController.cs#L116-L218) | The handler resolves one visible item. It parses a smaller profile, creates one plan, stores an eight-hour play session, and returns one source. [`jellyfin_playback.go`](https://github.com/MikeO7/kinosail/blob/24395e04309687cd0689c2079ca0deb00fd4d3bd/internal/server/jellyfin_playback.go#L39-L65) |
| Session creation | `GetPlaybackInfo` clones sources and creates a random `PlaySessionId`. Device-specific planning later attaches it to `StreamInfo`. [`MediaInfoHelper.cs`](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/Jellyfin.Api/Helpers/MediaInfoHelper.cs#L93-L136), [`MediaInfoHelper.cs`](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/Jellyfin.Api/Helpers/MediaInfoHelper.cs#L267-L348) | `newPlaySession` stores the item, Viewer Profile revision, playback plan, network class, and expiry. [`jellyfin_playback.go`](https://github.com/MikeO7/kinosail/blob/24395e04309687cd0689c2079ca0deb00fd4d3bd/internal/server/jellyfin_playback.go#L139-L153) |
| `StreamInfo.ToUrl` | HLS uses `/videos/{id}/master.m3u8`. The query carries source, codecs, rates, dimensions, segment container, segment length, minimum segments, play session, API key, and options. [`StreamInfo.cs`](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/MediaBrowser.Model/Dlna/StreamInfo.cs#L877-L1036), [`StreamInfo.cs`](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/MediaBrowser.Model/Dlna/StreamInfo.cs#L1045-L1142) | The source returns `/Videos/{id}/p/{recipe}/index.m3u8?playSessionId=...`. The recipe token holds Kinosail plan fields. [`jellyfin_playback.go`](https://github.com/MikeO7/kinosail/blob/24395e04309687cd0689c2079ca0deb00fd4d3bd/internal/server/jellyfin_playback.go#L155-L192) |
| Master endpoint | `GetMasterHlsVideoPlaylist` creates an `HlsVideoRequestDto` and calls `DynamicHlsHelper`. It does not start FFmpeg. [`DynamicHlsController.cs`](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/Jellyfin.Api/Controllers/DynamicHlsController.cs#L404-L520) | The first master request starts one background FFmpeg presentation. It waits until every variant has an init file and two segments. [`jellyfin_delivery.go`](https://github.com/MikeO7/kinosail/blob/24395e04309687cd0689c2079ca0deb00fd4d3bd/internal/server/jellyfin_delivery.go#L14-L54), [`hls.go`](https://github.com/MikeO7/kinosail/blob/24395e04309687cd0689c2079ca0deb00fd4d3bd/internal/server/hls.go#L76-L121), [`hls_playlist.go`](https://github.com/MikeO7/kinosail/blob/24395e04309687cd0689c2079ca0deb00fd4d3bd/internal/server/hls_playlist.go#L14-L100) |
| Master syntax | The helper writes `#EXTM3U` and one or more `#EXT-X-STREAM-INF` entries. It derives bandwidth, codecs, range, resolution, frame rate, and subtitle group from `StreamState`. Each VOD child is `main.m3u8` plus the complete query. [`DynamicHlsHelper.cs`](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/Jellyfin.Api/Helpers/DynamicHlsHelper.cs#L138-L231), [`DynamicHlsHelper.cs`](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/Jellyfin.Api/Helpers/DynamicHlsHelper.cs#L345-L375) | Kinosail writes five variants for a full transcode. It adds a private `#KINOSAIL-TRANSCODER` line, HLS version 7, independent segments, measured bandwidth, a fixed codec string, dimensions, frame rate, SDR, and no captions. [`hls_playlist.go`](https://github.com/MikeO7/kinosail/blob/24395e04309687cd0689c2079ca0deb00fd4d3bd/internal/server/hls_playlist.go#L87-L100) |
| Variant endpoint | `main.m3u8` recreates `StreamState`. `DynamicHlsPlaylistGenerator` computes the complete VOD segment schedule before any segment is encoded. [`DynamicHlsController.cs`](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/Jellyfin.Api/Controllers/DynamicHlsController.cs#L745-L857), [`DynamicHlsController.cs`](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/Jellyfin.Api/Controllers/DynamicHlsController.cs#L1387-L1425) | Kinosail serves FFmpeg's current EVENT playlist. The initial response contains only ready segments. It adds the play session to each URI. [`hls_playlist_session.go`](https://github.com/MikeO7/kinosail/blob/24395e04309687cd0689c2079ca0deb00fd4d3bd/internal/server/hls_playlist_session.go#L18-L53), [`hls_playlist_session.go`](https://github.com/MikeO7/kinosail/blob/24395e04309687cd0689c2079ca0deb00fd4d3bd/internal/server/hls_playlist_session.go#L61-L99) |
| Variant syntax | fMP4 uses version 7, `PLAYLIST-TYPE:VOD`, target duration, media sequence, `EXT-X-MAP`, six-decimal `EXTINF` values, and `ENDLIST`. Init and segment URLs contain the original query, `runtimeTicks`, and `actualSegmentLengthTicks`. [`DynamicHlsPlaylistGenerator.cs`](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/src/Jellyfin.MediaEncoding.Hls/Playlist/DynamicHlsPlaylistGenerator.cs#L34-L106) | The live response uses `PLAYLIST-TYPE:EVENT`, `EXT-X-INDEPENDENT-SEGMENTS`, and `EXT-X-START:TIME-OFFSET=0,PRECISE=YES`. It has no `ENDLIST` until the full encode completes. Segment URLs have only the play session. [`hls_presentation.go`](https://github.com/MikeO7/kinosail/blob/24395e04309687cd0689c2079ca0deb00fd4d3bd/internal/server/hls_presentation.go#L69-L75), [`hls_playlist_session.go`](https://github.com/MikeO7/kinosail/blob/24395e04309687cd0689c2079ca0deb00fd4d3bd/internal/server/hls_playlist_session.go#L77-L99) |
| Init and segment endpoints | `-1.mp4` requests the init. Other numeric `.mp4` paths request media segments. The request includes exact runtime and duration values. A missing, backward, or distant segment can replace the active transcode under a playlist lock. [`DynamicHlsController.cs`](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/Jellyfin.Api/Controllers/DynamicHlsController.cs#L1086-L1205), [`DynamicHlsController.cs`](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/Jellyfin.Api/Controllers/DynamicHlsController.cs#L1427-L1543) | Init and media files are static `init.mp4` and `segment-00000.m4s` files below a quality directory. Kinosail cannot restart at an arbitrary segment from these URLs. [`jellyfin_delivery.go`](https://github.com/MikeO7/kinosail/blob/24395e04309687cd0689c2079ca0deb00fd4d3bd/internal/server/jellyfin_delivery.go#L34-L54), [`hls_playlist.go`](https://github.com/MikeO7/kinosail/blob/24395e04309687cd0689c2079ca0deb00fd4d3bd/internal/server/hls_playlist.go#L232-L258) |
| Segment readiness | Jellyfin waits until the requested file exists. It also waits for the next segment or FFmpeg exit before serving a media segment. [`DynamicHlsController.cs`](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/Jellyfin.Api/Controllers/DynamicHlsController.cs#L1915-L1987) | Kinosail uses FFmpeg's `temp_file` flag. It publishes the master after two complete files exist. Later delivery uses `http.ServeFile`. [`hls_presentation.go`](https://github.com/MikeO7/kinosail/blob/24395e04309687cd0689c2079ca0deb00fd4d3bd/internal/server/hls_presentation.go#L69-L75), [`hls_playlist.go`](https://github.com/MikeO7/kinosail/blob/24395e04309687cd0689c2079ca0deb00fd4d3bd/internal/server/hls_playlist.go#L54-L85) |
| Segment response | Jellyfin serves the `.mp4` through `PhysicalFileResult` with range processing. It derives MIME type from `.mp4`, which resolves to `video/mp4`. Completion updates the job's download position and active request count. [`DynamicHlsController.cs`](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/Jellyfin.Api/Controllers/DynamicHlsController.cs#L1990-L2007), [`FileStreamResponseHelpers.cs`](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/Jellyfin.Api/Helpers/FileStreamResponseHelpers.cs#L99-L110), [`MimeTypes.cs`](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/MediaBrowser.Model/Net/MimeTypes.cs#L145-L179) | Kinosail intends `video/iso.segment` for `.m4s`, but both live media segments returned `application/octet-stream`. The same response had `X-Content-Type-Options: nosniff`. The init segment returned `video/mp4`. [`hls_playlist_session.go`](https://github.com/MikeO7/kinosail/blob/24395e04309687cd0689c2079ca0deb00fd4d3bd/internal/server/hls_playlist_session.go#L41-L53) |

### `WaitForMinimumSegmentCount` is not in the VOD path

The earlier minimum-two-segment theory was incorrect for this flow. `WaitForMinimumSegmentCount` opens a changing playlist with read/write sharing. It counts `#EXTINF` lines until the requested threshold. [`HlsHelpers.cs`](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/Jellyfin.Api/Helpers/HlsHelpers.cs#L17-L72).

`DynamicHlsController` calls it only from `GetLiveHlsStream`, after that live endpoint starts FFmpeg. [`DynamicHlsController.cs`](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/Jellyfin.Api/Controllers/DynamicHlsController.cs#L278-L343). The VOD master delegates to `DynamicHlsHelper`. The VOD variant uses `DynamicHlsPlaylistGenerator`. Neither VOD endpoint waits for encoded segments.

Kinosail's two-segment gate now avoids exposing one incomplete segment. It does not match Jellyfin's VOD architecture. It also cannot explain the latest failure because Swiftfin fetched both gated segments.

### FFmpeg construction comparison

Official Jellyfin builds one seekable segment job. Its command includes these HLS controls:

```text
-copyts
-avoid_negative_ts disabled
-max_delay 5000000
-hls_time {SegmentLength}
-hls_segment_type fmp4
-start_number {requested segment}
-hls_segment_options movflags=+frag_discont+skip_sidx
-hls_playlist_type vod
-hls_list_size 0
```

For fMP4 video, Jellyfin states two reasons for the segment options. `frag_discont` carries the initial audio delay into `TFDT`. `skip_sidx` avoids FFmpeg rewriting open-GOP boundary timestamps. [`DynamicHlsController.cs`](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/Jellyfin.Api/Controllers/DynamicHlsController.cs#L1578-L1656). Jellyfin also aligns encoded keyframes to the HLS segment interval. [`DynamicHlsController.cs`](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/Jellyfin.Api/Controllers/DynamicHlsController.cs#L1788-L1905).

Kinosail builds one multi-output FFmpeg process for five variants. Its HLS arguments are:

```text
-hls_time 4
-hls_playlist_type event
-hls_segment_type fmp4
-hls_flags temp_file+independent_segments
-hls_fmp4_init_filename init.mp4
-hls_segment_filename segment-%05d.m4s
```

The command has no `copyts`, `avoid_negative_ts disabled`, `max_delay`, `frag_discont`, or `skip_sidx`. [`hls_presentation.go`](https://github.com/MikeO7/kinosail/blob/24395e04309687cd0689c2079ca0deb00fd4d3bd/internal/server/hls_presentation.go#L15-L75). The single-variant path uses the same segment helper. [`hls.go`](https://github.com/MikeO7/kinosail/blob/24395e04309687cd0689c2079ca0deb00fd4d3bd/internal/server/hls.go#L223-L282).

The captured Kinosail media segment contains two `sidx` boxes before `moof`. This proves `skip_sidx` is absent in the actual output. Its video track starts at `TFDT=0`. Its audio track also starts at `TFDT=0`. Segment 1 starts video at 96096 ticks on a 24000-Hz timescale. It starts audio at 189432 ticks on a 48000-Hz timescale. The audio boundary is approximately 57.5 ms earlier than the video's 4.004-second boundary. This evidence makes the missing Jellyfin fMP4 timestamp controls more important than a playlist-only theory.

### Cancellation, progress, and session handling

Official Jellyfin does not bind VOD FFmpeg directly to a disconnected master request. The master and variant are metadata operations. A segment request starts or joins the FFmpeg job. Each segment increments an active request count. Response completion advances `DownloadPositionTicks` and decrements that count. [`DynamicHlsController.cs`](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/Jellyfin.Api/Controllers/DynamicHlsController.cs#L1427-L1552), [`DynamicHlsController.cs`](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/Jellyfin.Api/Controllers/DynamicHlsController.cs#L1915-L2007), [`TranscodeManager.cs`](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/MediaBrowser.MediaEncoding/Transcoding/TranscodeManager.cs#L577-L621).

When no request is active, Jellyfin starts a 60-second HLS kill timer. Playback start and progress ping the job. `Sessions/Playing/Ping` also pings it. Playback stopped kills matching jobs and can delete their files. [`TranscodeManager.cs`](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/MediaBrowser.MediaEncoding/Transcoding/TranscodeManager.cs#L100-L216), [`TranscodeManager.cs`](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/MediaBrowser.MediaEncoding/Transcoding/TranscodeManager.cs#L688-L716), [`PlaystateController.cs`](https://github.com/jellyfin/jellyfin/blob/4910aafa1a8227a65a037d3d2d299a32691e4de3/Jellyfin.Api/Controllers/PlaystateController.cs#L195-L259).

Kinosail starts FFmpeg from the master request, but runs it with the server lifecycle context. A client disconnect does not stop it. The job is keyed by item and recipe, not by play session. It has no request count or download position. It encodes the complete presentation even after Swiftfin aborts. [`hls.go`](https://github.com/MikeO7/kinosail/blob/24395e04309687cd0689c2079ca0deb00fd4d3bd/internal/server/hls.go#L76-L167). Kinosail records progress and removes its capability on `Stopped`, but that action does not cancel the HLS job. [`jellyfin_progress.go`](https://github.com/MikeO7/kinosail/blob/24395e04309687cd0689c2079ca0deb00fd4d3bd/internal/server/jellyfin_progress.go#L21-L29), [`jellyfin_progress.go`](https://github.com/MikeO7/kinosail/blob/24395e04309687cd0689c2079ca0deb00fd4d3bd/internal/server/jellyfin_progress.go#L115-L139).

This lifecycle mismatch wastes work. It is not the immediate startup cause. Swiftfin never sent a playback-start event in the failing trace.

### Ranked exact differences

1. **Very high confidence: wrong live segment media type.** Swiftfin fetched init and two media segments. Init was `video/mp4`. Both media segments were `application/octet-stream` with `nosniff`. Official Jellyfin uses `.mp4` segment paths and serves `video/mp4` with range support. This is the narrowest confirmed response-contract failure at the exact abort boundary.
2. **High confidence: missing Jellyfin fMP4 timestamp controls.** Kinosail omits `frag_discont` and `skip_sidx`. The actual output contains two `sidx` boxes and a 57.5 ms audio/video boundary difference. Official Jellyfin added both controls for fMP4 timestamp correctness. This can make a strict Apple or VLC decoder reject the fragments after download.
3. **Medium-high confidence: partial EVENT playlist instead of a complete VOD map.** Official Jellyfin returns a complete `VOD` playlist before encoding. Kinosail returns a two-segment `EVENT` snapshot without `ENDLIST`, then expects polling. Swiftfin reloaded the same 406-byte playlist once and stopped after segment 1. The difference matches the request pattern, but the segment MIME and bytes are more direct failures.
4. **Medium confidence: unconditional zero-offset `EXT-X-START`.** Kinosail inserts `#EXT-X-START:TIME-OFFSET=0,PRECISE=YES` even without resume. Official Jellyfin does not put this tag in the VOD playlist. The tag is legal, so it is less likely than the first three differences.
5. **Low confidence for this SDR item: master metadata.** Kinosail hard-codes `avc1.64002a,mp4a.40.2`. The captured init has AVC profile `0x64`, compatibility `0x00`, and level `0x2a`, so this exact trace matches its advertised codec. Other codecs and remux paths can still be wrong.
6. **Eliminated for this attempt: source mapping, `TranscodingUrl`, child routing, and two-segment readiness.** The live trace proves each stage completed. These remain useful regressions, but they are not the current abort point.

### Highest-confidence next regression

Add one production-boundary playback test. It must follow `POST PlaybackInfo` through the returned master, selected variant, init, and first media segment. It must use the same full HTTP middleware as Nox, not only `httptest.ResponseRecorder` against the inner handler.

The first failing assertion must be:

```text
media segment Content-Type == video/mp4
```

The test must also assert `Content-Type` is not `application/octet-stream` when `X-Content-Type-Options: nosniff` is present. Existing inner-handler tests expect `video/iso.segment`, but the live server returns a different value. That gap is the highest-confidence red regression.

After this header regression is red, add an artifact assertion for the next likely defect. The first generated fMP4 media segment must not contain `sidx`. Its command must include `movflags=+frag_discont+skip_sidx`. Keep this second assertion separate so the response-contract failure stays easy to locate.
