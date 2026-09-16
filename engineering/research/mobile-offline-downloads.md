# Best-in-class downloads for the official Jellyfin mobile apps

Research snapshot: 2026-08-22. This note covers the two apps Jellyfin lists as its official phone/tablet clients: **Jellyfin for Android** (`jellyfin/jellyfin-android`) and **Jellyfin for iOS** (`jellyfin/jellyfin-ios`). It does not cover Swiftfin, Android TV, Plex, or other third-party clients. [Official Jellyfin client list](https://jellyfin.org/downloads/clients/)

Reviewed releases and immutable source revisions:

- Jellyfin for Android 2.7.1, commit `13abdff1cf2c157d8354b02f8e1f7d35e923260e` ([release](https://github.com/jellyfin/jellyfin-android/releases/tag/v2.7.1)); and
- Jellyfin for iOS 1.8.0.5, commit `78bc20a0a63aec2b0f96d7ed2eaea4a2f9f9d40f` ([release](https://github.com/jellyfin/jellyfin-ios/releases/tag/v1.8.0.5)).

## Decision

Optimize Kinosail for the **unmodified official apps**. Do not make a custom Kinosail protocol or a client fork part of the backend roadmap.

The highest-value sequence is:

1. Make the existing Jellyfin item contract exact enough that both apps can enqueue a download: honor `Ids`, return a non-host-leaking `Path` with the real extension, and include the stable Kinosail `ServerId`.
2. Make original bytes immutable at a download URL and implement Android's exact range behavior, including an empty-body `416` response.
3. Return complete, probe-backed media metadata and truthful `PlaybackInfo` decisions so downloaded originals play where they should and iOS never mistakes an incompatible file for playable.
4. Add progressive, finalized, cached remux/transcode renditions through the standard iOS `PlaybackInfo`/`TranscodingUrl` flow. Android will still receive originals because that is what its official client requests.
5. Prove the result with the real apps under interruption, relaunch, server restart, source replacement, low bandwidth, and airplane mode.

This is the maximum useful backend scope. Queue UI, local storage policy, client-side integrity checks, and offline subtitle bundling cannot be added from the server.

## The two clients have different contracts

| Behavior | Android 2.7.1 | iOS 1.8.0.5 | Backend consequence |
| --- | --- | --- | --- |
| Enqueue lookup | `GET /Items?Ids=...&Fields=MediaSources,Path`, in batches of 25 | Download object arrives from the embedded web UI | `Ids`, `Path`, `ServerId`, and `MediaSources` must be correct |
| Default media request | Original `GET /Items/{id}/Download` | Original URL supplied by the web UI, normally `/Items/{id}/Download` | Never silently change the standard route to a rendition |
| Alternative rendition | None | Optional alpha flow posts `PlaybackInfo` and follows a progressive stream/transcode URL | Device-profile work improves only iOS downloads |
| Queue | Persistent Room records; one item/file at a time in a foreground WorkManager worker | Persisted item queue; up to three in-flight Expo tasks | Protect one Android transfer and three parallel iOS transfers without server overload |
| Resume | Explicit `Range: bytes=<local-size>-`; no `If-Range` or ETag | Expo background download task; app does not save resumable state | Android requires URL-level byte immutability; iOS benefits from stable validators but cannot resume across app relaunch reliably |
| Completion check | File exists and stored length matches | Expo promise resolved; later UI checks only existence | Neither app verifies a server digest or opens every file before declaring it complete |
| Downloaded bundle | Original plus primary image | One media file | External subtitle files are not made offline by either client |

## Verified Android behavior

Jellyfin 2.7 redesigned downloads into an in-app, persistent workflow. The app can show and play downloads, uses its own player for video, opens other media types externally, and lets the person select a storage folder. [Official Android 2.7 announcement](https://jellyfin.org/posts/android-v2.7/)

### Enqueue requires exact item DTOs

The native bridge receives one or more item IDs. `DownloadManager` chunks them in groups of 25 and calls `GET /Items` with `Ids=<ids>` and `Fields=MediaSources,Path`. It rejects the entire chunk when the returned item count differs from the requested count. The stored item is later used for its media source, server identity, filename, extension, and offline metadata. [Android `DownloadManager`](https://github.com/jellyfin/jellyfin-android/blob/13abdff1cf2c157d8354b02f8e1f7d35e923260e/app/src/main/java/org/jellyfin/mobile/downloads/DownloadManager.kt#L24-L82)

`DownloadQueue` hard-fails with `Missing item path` when `item.Path` is absent. It only uses the basename, so Kinosail should return a stable synthetic client path such as `/media/<safe-original-basename.ext>`, not disclose the owner's absolute filesystem path. [Android filename and URL construction](https://github.com/jellyfin/jellyfin-android/blob/13abdff1cf2c157d8354b02f8e1f7d35e923260e/app/src/main/java/org/jellyfin/mobile/downloads/DownloadQueue.kt#L147-L160)

### The queue is persistent, constrained, and serial

The app stores downloads and files in Room, runs a foreground `WorkManager` job, and maps its Wi-Fi/cellular/roaming setting to `UNMETERED`, `NOT_ROAMING`, or `CONNECTED`. Network I/O failures requeue work for WorkManager retry. [Android worker](https://github.com/jellyfin/jellyfin-android/blob/13abdff1cf2c157d8354b02f8e1f7d35e923260e/app/src/main/java/org/jellyfin/mobile/downloads/DownloadWorker.kt#L22-L82)

The queue processes items serially. Within each item it downloads `primary.webp` first, then the original from `/Items/{id}/Download`; no download-time `PlaybackInfo` call, quality choice, remux, transcode, or external subtitle fetch occurs. [Android queue](https://github.com/jellyfin/jellyfin-android/blob/13abdff1cf2c157d8354b02f8e1f7d35e923260e/app/src/main/java/org/jellyfin/mobile/downloads/DownloadQueue.kt#L34-L73), [file list and URLs](https://github.com/jellyfin/jellyfin-android/blob/13abdff1cf2c157d8354b02f8e1f7d35e923260e/app/src/main/java/org/jellyfin/mobile/downloads/DownloadQueue.kt#L132-L181)

### Android's resume behavior dictates the server contract

For every request—even a new zero-byte file—the downloader sends `Range: bytes=<local statSize>-`. It requires:

- `Content-Length` on a `200`;
- `Content-Range` on a `206` or `416`; and
- a known total length in `Content-Range`.

It sends no `If-Range` and no validator. It opens the destination as `rw`, seeks to the response range start, and writes without truncating the old file. [Android `FileDownloader`](https://github.com/jellyfin/jellyfin-android/blob/13abdff1cf2c157d8354b02f8e1f7d35e923260e/app/src/main/java/org/jellyfin/mobile/downloads/FileDownloader.kt#L30-L58), [range parsing and writing](https://github.com/jellyfin/jellyfin-android/blob/13abdff1cf2c157d8354b02f8e1f7d35e923260e/app/src/main/java/org/jellyfin/mobile/downloads/FileDownloader.kt#L81-L123)

This creates two non-negotiable backend rules:

1. Bytes at a given `/Items/{id}/Download` URL must never change. If a source is replaced, assign a new content revision/item ID and retain the old immutable revision long enough for queued downloads, or make the old URL fail. Never let a resumed request append bytes from a new representation.
2. `Range: bytes=<exact-length>-` must return `416` with `Content-Range: bytes */<length>` and a **zero-byte body**. Android accepts that response as an already-finished file, then passes it through its save path. Its parser maps the `*` range to start zero, so a textual error body would overwrite the beginning of the completed media. [Android `416` special case](https://github.com/jellyfin/jellyfin-android/blob/13abdff1cf2c157d8354b02f8e1f7d35e923260e/app/src/main/java/org/jellyfin/mobile/downloads/FileDownloader.kt#L50-L58), [`ContentRange` parser](https://github.com/jellyfin/jellyfin-android/blob/13abdff1cf2c157d8354b02f8e1f7d35e923260e/app/src/main/java/org/jellyfin/mobile/downloads/ContentRange.kt#L16-L42)

RFC 9110 recommends ranges for recovery from partial failures, but standards-compliant response metadata alone is insufficient here because this client omits `If-Range`. URL-level immutability is the safety mechanism. [RFC 9110 range requests](https://www.rfc-editor.org/rfc/rfc9110.html#section-14)

Android considers an existing file valid when its stored completed length still matches; it does not hash it. Its offline video player opens only the local media file and does not attach the remote external-subtitle URLs recorded in metadata. Embedded tracks remain the reliable way to preserve offline audio/subtitle choices. [Android size verification](https://github.com/jellyfin/jellyfin-android/blob/13abdff1cf2c157d8354b02f8e1f7d35e923260e/app/src/main/java/org/jellyfin/mobile/app/StorageManager.kt#L46-L59), [Android local playback source](https://github.com/jellyfin/jellyfin-android/blob/13abdff1cf2c157d8354b02f8e1f7d35e923260e/app/src/main/java/org/jellyfin/mobile/player/queue/QueueManager.kt#L101-L135), [local stream preparation](https://github.com/jellyfin/jellyfin-android/blob/13abdff1cf2c157d8354b02f8e1f7d35e923260e/app/src/main/java/org/jellyfin/mobile/player/queue/QueueManager.kt#L306-L320)

## Verified iOS behavior

Jellyfin introduced direct iOS downloads in 1.7. The app describes transcoded downloads as alpha: there is no quality control and output can be larger than the original. [Official iOS 1.7 announcement](https://jellyfin.org/posts/ios-v1.7.0/)

### DTO identity and filename are backend-controlled

The iOS download key is `${ServerId}_${ItemId}`. Its filename uses the item metadata and derives the original extension from `item.Path`; when that information is missing, the legacy fallback forces `.mp4`. Kinosail therefore must include its stable existing Jellyfin server ID and a safe synthetic path with the true extension on every downloadable item. [iOS `DownloadModel`](https://github.com/jellyfin/jellyfin-ios/blob/78bc20a0a63aec2b0f96d7ed2eaea4a2f9f9d40f/models/DownloadModel.ts#L44-L156)

The native shell also extracts the token from the supplied URL and accepts the current `ApiKey` or legacy `api_key` parameter name. Kinosail must accept both query spellings. This especially matters for a static-stream retry after the play-session authorization has expired. [iOS native-shell download intake](https://github.com/jellyfin/jellyfin-ios/blob/78bc20a0a63aec2b0f96d7ed2eaea4a2f9f9d40f/components/NativeShellWebView.tsx#L106-L145), [Jellyfin SDK authorization parameter](https://github.com/jellyfin/jellyfin-sdk-typescript/blob/592747ce7add446b9a14ad56aba8a7441a2e2618/src/constants.ts#L1-L12)

### Direct and alpha transcoded flows

By default, iOS downloads the original URL supplied by its embedded web UI. With alpha transcoded downloads enabled for audio/video, it:

1. posts a device profile to `/Items/{id}/PlaybackInfo` after removing HLS transcoding profiles and requiring encoded subtitles;
2. selects `MediaSources[0]`;
3. for direct play/direct stream, builds `/Videos|Audio/{id}/stream.<container>?ApiKey=...&playSessionId=...&mediaSourceId=...&Tag=...&Static=true`;
4. otherwise follows `TranscodingUrl`; and
5. falls back to the original download URL if the response has no usable source.

[iOS download-profile conversion](https://github.com/jellyfin/jellyfin-ios/blob/78bc20a0a63aec2b0f96d7ed2eaea4a2f9f9d40f/features/downloads/utils/profile.ts#L12-L32), [iOS source selection and request construction](https://github.com/jellyfin/jellyfin-ios/blob/78bc20a0a63aec2b0f96d7ed2eaea4a2f9f9d40f/features/downloads/hooks/useDownloadHandler.ts#L36-L102)

Because the Expo download request passes no headers, a returned `TranscodingUrl` must be a progressive, self-authorizing URL scoped by the play session or an equivalent short-lived capability. It must never appear in access logs. HLS is not a valid download rendition for this client because the app deliberately filters HLS profiles out.

### Queue, background execution, persistence, and completion

iOS starts at most three transfers. It calls Expo `createDownloadResumable` using the default background session, but saves neither the resumable task's `savable()`/resume data nor byte progress. Its persisted item store deliberately restores a previously `Downloading` item as `Pending` because cross-relaunch resume is not implemented. It marks a transfer complete when `downloadAsync()` resolves and does not request or verify a digest. [iOS queue and transfer](https://github.com/jellyfin/jellyfin-ios/blob/78bc20a0a63aec2b0f96d7ed2eaea4a2f9f9d40f/features/downloads/hooks/useDownloadHandler.ts#L104-L215), [iOS persisted-state restoration](https://github.com/jellyfin/jellyfin-ios/blob/78bc20a0a63aec2b0f96d7ed2eaea4a2f9f9d40f/stores/DownloadStore.ts#L50-L113)

Expo's matching filesystem API says background transfers continue while the app is backgrounded, progress callbacks wait for foreground, and cross-restart resume requires saving and reconstructing the `savable()` state. The app does not perform those steps. [Expo SDK 45 filesystem documentation](https://github.com/expo/expo/blob/sdk-45/docs/pages/versions/v45.0.0/sdk/filesystem.md#L42-L99), [Expo background and resume semantics](https://github.com/expo/expo/blob/sdk-45/docs/pages/versions/v45.0.0/sdk/filesystem.md#L463-L487)

Ordinary original downloads remain Files/share-oriented. In-app video playback is enabled only when the alpha device-profile path selected a direct or transcoded source. The app checks that a file exists before opening it, not that its length, digest, or media structure is correct. [iOS download actions](https://github.com/jellyfin/jellyfin-ios/blob/78bc20a0a63aec2b0f96d7ed2eaea4a2f9f9d40f/features/downloads/utils/downloadItemActions.ts#L14-L47), [iOS local-file check and playback](https://github.com/jellyfin/jellyfin-ios/blob/78bc20a0a63aec2b0f96d7ed2eaea4a2f9f9d40f/screens/DownloadScreen.tsx#L94-L157)

## The standard Jellyfin boundary

Jellyfin's stable `GET /Items/{itemId}/Download` checks the user's download permission and returns the original file with range processing enabled. It does not select a device-compatible rendition. [Jellyfin 10.11.11 `LibraryController.GetDownload`](https://github.com/jellyfin/jellyfin/blob/1fbd8739292cce610231be93daf43368733edf63/Jellyfin.Api/Controllers/LibraryController.cs#L657-L715)

Kinosail must preserve these meanings:

- `/Items/{id}/Download` returns the immutable original represented by that item revision.
- `/Items/{id}/PlaybackInfo` plus static stream or `TranscodingUrl` is the existing iOS path for a compatible alternative.
- Android receives the original. Returning a transcode from its `/Download` request would silently violate the API and could change extension, tracks, and quality without consent.

## Historical gaps found in the initial Kinosail checkout

The initial checkout exposed the expected download, file, stream, image, subtitle, and `PlaybackInfo` routes, but it was not compatible enough for dependable downloads. These findings are retained to explain the acceptance criteria that drove the implementation:

1. **Android enqueue is currently broken.** `/Items` filters by hierarchy/type/search but ignores `Ids`, so Android can receive the whole library and reject the count. Item DTOs omit `Path`, causing `Missing item path`. ([Kinosail item query](../../apps/player/internal/server/jellyfin_items.go#L55-L87), [item DTO](../../apps/player/internal/server/jellyfin_items.go#L220-L255))
2. **iOS identity/extension is incomplete.** Item DTOs omit `ServerId` and `Path`, so iOS keys can collapse to `undefined_<item>` and non-MP4 downloads can receive a false `.mp4` extension. Kinosail already has a stable `api.id`; it should be emitted as `ServerId`. ([server identity](../../apps/player/internal/server/jellyfin.go#L63-L78))
3. **Current query authentication misses `ApiKey`.** The session lookup checks the `ApiKey` header and legacy `api_key` query, but not the current `ApiKey` query used by iOS static stream URLs. The temporary `playSessionId` public path can hide this until that session's eight-hour expiry. ([token lookup](../../apps/player/internal/server/sessions.go#L97-L128), [play-session lifetime](../../apps/player/internal/server/jellyfin_playback.go#L38-L58))
4. **Range behavior is close but unsafe at one edge.** `ServeFile` provides normal ranges, length, and last-modified metadata, but its standard `416` error has a text body. Android can write that body at offset zero after an exactly-complete interrupted transfer. The compatibility route needs a tested empty-body `416` response.
5. **Representations are not revision-stable.** `MediaSources[].ETag` is the path-derived item ID, which stays unchanged when bytes at that path are replaced. No strong response ETag is set. ([path-derived library ID](../../packages/library/library.go#L94-L104), [media source](../../apps/player/internal/server/jellyfin_playback.go#L61-L80))
6. **`PlaybackInfo` ignores the posted profile.** Kinosail always claims direct play and direct stream, exposes no transcode, and records no codecs, dimensions, HDR, channels, or embedded tracks with which to make a truthful decision. ([playback response](../../apps/player/internal/server/jellyfin_playback.go#L38-L80), [library item model](../../packages/library/library.go#L15-L45))
7. **Android's artwork request is not honored precisely.** Android asks for `format=Webp` and saves `primary.webp`; Kinosail ignores the format query and serves the original artwork bytes. ([Android image request](https://github.com/jellyfin/jellyfin-android/blob/13abdff1cf2c157d8354b02f8e1f7d35e923260e/app/src/main/java/org/jellyfin/mobile/downloads/DownloadQueue.kt#L162-L181), [Kinosail image handler](../../apps/player/internal/server/jellyfin_playback.go#L82-L89))

`Content-Disposition` and representation digests remain good HTTP hygiene, but neither reviewed app uses the response filename or verifies a digest. They should not outrank `Ids`, `Path`, `ServerId`, authentication, byte immutability, and exact range semantics.

## Implementation status

The server now closes all seven backend gaps: authorized exact `Ids` batches; safe `Path` and stable `ServerId`; both query-token spellings; empty-body complete-range `416`; per-session, seven-day immutable content-addressed snapshots with strong SHA-256 ETags and RFC 9530 `Repr-Digest`; probe-backed device-profile playback decisions; and real FFmpeg-generated WebP thumbnails when requested. The focused Go suite covers ordinary/suffix/malformed/oversized ranges, resume after source replacement, identity and permission isolation, validators/digest, profile-based playback, and Android artwork format.

The remaining M0–M4 text below is the release acceptance matrix. Server-side criteria are implemented; bullets requiring real apps, a 10 GiB transfer, constrained networks, process death, airplane mode, hardware decoding, or progressive iOS transcoded downloads remain external/device work and must not be reported as certified from unit tests.

## Backend roadmap and acceptance criteria

### M0 — Make both official apps enqueue and authenticate

- `GET /Items?Ids=<one-or-many>&Fields=MediaSources,Path` returns exactly the authorized requested items, with the correct `TotalRecordCount`; missing/unauthorized IDs do not leak and produce the behavior the official Jellyfin server uses.
- Every downloadable DTO includes `ServerId`, `MediaSources`, and a safe synthetic `Path` whose basename and extension match the original without exposing the host path.
- Tests cover one item, multiple items, 25-item and 26-item Android batches, mixed-case query names, an unknown ID, an unauthorized ID, and two servers with the same item ID.
- Both `ApiKey=<token>` and `api_key=<token>` authenticate download/static-stream requests. A static-stream request still succeeds after a simulated play-session expiry; invalid and revoked tokens fail.
- On real Android 2.7.1, selecting one movie, a season, and more than 25 items produces the exact queued count. On iOS 1.8.0.5, MKV, MP4, M4A, and FLAC downloads retain correct extensions and distinct server-scoped keys.

### M1 — Bulletproof original bytes and resume

- Make item/revision identity byte-stable: a source replacement gets a new revision identity. Retain an old immutable snapshot for an explicit queue grace period or make its old URL fail; never serve new bytes at the old download URL.
- For Android's fresh `Range: bytes=0-`, return `206`, exact `Content-Range: bytes 0-(N-1)/N`, exact `Content-Length: N`, `Accept-Ranges: bytes`, and the original bytes.
- For an interrupted file of length `K`, return `206`, `Content-Range: bytes K-(N-1)/N`, `Content-Length: N-K`, and exactly the suffix.
- For `Range: bytes=N-`, return `416`, `Content-Range: bytes */N`, and a zero-byte body. Also test `K>N`, malformed ranges, suffix ranges, and a zero-length source.
- Set a strong content-revision ETag before `ServeContent`, retain consistent `Last-Modified`, type, total length, and `Content-Disposition`, and use `Cache-Control: private, no-transform`. Validators help iOS/other HTTP stacks even though Android does not send `If-Range`.
- Run fault tests with at least a 10 GiB sparse fixture: disconnect three times, kill/restart Kinosail, suspend/relaunch each app, let a transfer finish just before the client records completion, replace the library source, revoke permission, and exhaust the old-revision grace period. The final file must be byte-identical or clearly failed—never mixed.

### M2 — Accurate metadata and offline-playable originals

- Probe at scan time and populate actual container, size, duration, video/audio codecs, profile/level, dimensions, bit depth/HDR, channels, language, disposition, and embedded/external stream metadata.
- Evaluate the posted iOS download `DeviceProfile` honestly. Put the best valid source first because iOS selects `MediaSources[0]`; never claim direct play/direct stream merely from a filename extension.
- Validate downloaded originals in both apps across H.264/AAC MP4, HEVC/HDR, MKV, incompatible video, incompatible audio, multiple embedded audio tracks, embedded text/image subtitles, and external subtitles. Record the expected hard limitation for external subtitles rather than reporting the item trip-ready.
- Honor Android's `format=Webp` with actual WebP bytes, correct type/length, and a content-revision image tag. Artwork failure should not leave the much larger media transfer in a misleading complete state.

Jellyfin's codec documentation explains why container, codec, profile, audio, HDR, and subtitle delivery all affect direct play/remux/transcode decisions. [Jellyfin codec support](https://jellyfin.org/docs/general/clients/codec-support/)

### M3 — Reliable iOS compatible renditions through standard routes

- Implement only the existing `PlaybackInfo` contract: original direct play, lossless remux when codecs are supported but the container is not, and transcode when video/audio/subtitle constraints require it.
- Return a progressive, non-HLS `TranscodingUrl` because iOS removes HLS profiles for downloads. The URL must remain authorized for the transfer/retry lifetime without exposing a durable account token and must be redacted from logs.
- Cache only finalized, probed, immutable renditions. Key the cache by source digest, selected streams, complete output profile/tone-map policy, and encoder build/options. Publish atomically after close, size check, probe, and hash.
- Prefer a ready cached rendition over a live encoder. On a miss, do not call a growing partial file resumable. Either finish preparation before serving bytes or state the unavoidable first-byte delay; precompute common mobile variants during idle time where the storage/CPU tradeoff is justified.
- A repeated identical iOS request is a cache hit with identical bytes, length, ETag, and URL revision. Encoder/server interruption never exposes a partial result as ready.

Research on multi-version video caching finds that mixed/adaptive retention performs better than blindly retaining every encoded version, while transcoding-cache research supports matching stored variants to heterogeneous client capabilities. Use source/profile popularity, size, and recomputation cost for eviction rather than keeping every permutation. [Hartanto et al., “Caching video objects: layers vs versions?”](https://asu.elsevierpure.com/en/publications/caching-video-objects-layers-vs-versions/), [Shen, Lee, and Basu, “Performance Evaluation of Transcoding-Enabled Streaming Media Caching System”](https://doi.org/10.1007/3-540-36389-0_29)

### M4 — Resource control and end-to-end proof

- Treat Android's single transfer and iOS's three transfers as fixed client behavior. Separate encoder admission from file serving so a cache miss cannot starve already-ready originals.
- Within the same user priority, favor short remaining preparation jobs while reserving fairness for large movies. SRPT research shows lower mean response time for static-file workloads, but it must remain inside fairness bands to avoid starvation. [Harchol-Balter et al., “SRPT Scheduling for Web Servers”](https://www.cs.cmu.edu/~harchol/Papers/jss.pdf)
- Measure queue-to-first-byte, effective throughput, resume count, bytes retransmitted, range/416 responses, cache hit rate, encoder wait/run time, source-revision failures, and completed-download rate. Never log titles, host paths, access tokens, or self-authorizing URLs.
- Run a repeatable constrained-WAN/airplane-mode matrix on the actual released apps: individual movie, full season, 10 GiB item, three iOS-parallel items, server restart, router drop, background/suspension, iOS relaunch, Android process death, and source replacement. A pass means the app's downloaded item opens with networking denied and its expected embedded tracks work.

## Hard client boundary

The Kinosail backend cannot make unmodified clients:

- offer Android transcode/quality choices or iOS alpha-download quality controls;
- change Android's serial queue or iOS's fixed concurrency of three;
- make iOS persist Expo resume data/progress across relaunch;
- make either app verify a digest or perform a local playback probe before showing complete;
- preflight and reserve local storage for a season;
- download external subtitle files or atomically install a media/artwork/subtitle bundle; or
- provide a deadline-aware “ready for the trip” state.

The backend should still send a strong ETag and whole-representation digest for correctness and future clients, but it must not label a transfer verified merely because it served all bytes. `Repr-Digest` describes the selected representation but does not replace TLS or authorization. [RFC 9530](https://www.rfc-editor.org/rfc/rfc9530.html)

## Implemented first move

This change implements exact `Ids` filtering, stable `ServerId`, safe synthetic `Path`, both iOS query-token spellings, expired-play-session fallback to valid token authentication, and Android-safe zero-body `416` handling, with captured protocol tests.

Next, make item revisions immutable across source replacement and server restart, then complete the large-file/interruption device matrix. Transcoding is valuable only after that original-byte path is trustworthy, and only the standard iOS alpha path can consume it.
