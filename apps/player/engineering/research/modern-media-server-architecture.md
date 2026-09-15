# Modern media-server architecture for Kinosail

Research snapshot: 2026-08-22

Implementation baseline: committed `aa6ded98083ae48ef0377529b1000bfabd1ce6c0` (`main`)

Scope: the full current and clearly planned Kinosail feature surface, not only subtitles and transcoding

## Executive decision

Kinosail should remain a standards-first, API-driven monolith with one Server container, an embedded durable catalog, direct play as the preferred path, narrowly targeted remux/transcode fallbacks, and Owner-managed direct remote access. That architecture fits a private home server better than importing the distributed systems used by commercial video platforms.

The next work should not be “add every modern technique.” It should establish truthful media facts, deterministic playback decisions, progressive startup, immutable download resources, and explicit subtitle/accessibility semantics. Advanced research techniques—per-title encoding, learned adaptive bitrate control, automatic segment detection, or personalized ranking—only earn their complexity after representative corpus and device measurements show a material benefit.

The committed implementation and the product claims are presently out of sync in important places. At `aa6ded9`, the code does **not** extract embedded subtitles, does **not** honor Jellyfin device profiles in `PlaybackInfo`, serves downloads from mutable source files, publishes HLS only after two whole-file encodes finish, and refreshes libraries on startup/manual/periodic scans rather than filesystem events. Closing these truth gaps is priority zero because downstream design and testing otherwise target an assumed system rather than the shipped one.

## Evidence model

This report uses these labels:

| Label | Meaning |
|---|---|
| **Current** | Present in committed `aa6ded9`; uncommitted concurrent work is excluded. |
| **Claimed** | Described by the README or a research note but not supported by the committed implementation inspected here. |
| **Protocol-tested** | A fixture or handler test exists; this is not physical-client, device, GPU, network, tuner, or restore certification. |
| **Planned** | Clearly represented in repository docs, closed feature issues, API shape, or UI, but incomplete or unverified. |
| **Recommended** | Evidence-backed direction proposed by this research. |

The repository is the source of truth for current behavior. Standards and official implementation sources define interoperability requirements. Peer-reviewed and systems research informs choices only where it contributes something those sources cannot.

## Whole-product inventory and disposition

This matrix accounts for every capability family advertised in the committed [README at `aa6ded9`](https://github.com/MikeO7/kinosail/blob/aa6ded98083ae48ef0377529b1000bfabd1ce6c0/README.md). “Current” is intentionally conservative.

| Capability | Committed baseline | Recommended disposition | Maturity and validation need |
|---|---|---|---|
| Recursive Movie/Show/Music/Photo/Book/Mixed scanning | `WalkDir` scan, explicit extensions, no symlink traversal, path-derived IDs, startup/manual/10-minute refresh ([scanner at `aa6ded9`](https://github.com/MikeO7/kinosail/blob/aa6ded98083ae48ef0377529b1000bfabd1ce6c0/internal/library/library.go)) | Keep full scan as reconciliation; add a persisted catalog and debounced filesystem-event hints only when measured scan cost warrants it | Current core; README filesystem-change claim is not current. Test rename, atomic replace, permission failure, watcher overflow, large trees, and restart reconciliation. |
| Browse, filters, pagination, search | Server-side in-memory organization/filtering with API and web surfaces | Preserve one application operation per API/web capability; use embedded SQLite FTS5/BM25 when catalog persistence or query latency justifies it | Current breadth, not scale-certified. Benchmark representative 1k/10k/100k-item libraries. |
| Continue Watching, history, ratings, watched state | Per-profile JSON state with latest-write semantics | Add operation IDs or monotonically increasing client revisions before multi-client conflict behavior becomes user-visible | Current/protocol-tested; verify two devices, offline replay, clock skew, and source deletion. |
| Recommendations | Rule-oriented local recommendations | Begin with explainable recency/genre/creator/unfinished rules; evaluate ranking offline before personalization | Current/planned depth. Academic recommender work helps only after consent-safe local feedback and metrics exist. |
| My List, queues, shuffle, playlists, manual/smart collections | JSON-backed lists and API/web operations | Keep deterministic local rules and stable item membership semantics; migrate with the catalog if SQLite is adopted | Current/protocol-tested; verify concurrent mutation, deletion, rename, and backup/restore. |
| NFO and local metadata | Local sidecars take precedence in scanner/provider flow | Preserve local-owner authority and record provenance per field | Current; test malformed/partial NFO and owner override persistence. |
| Embedded music metadata | Advertised and partially represented in library organization | Normalize tags, multi-value artists, MusicBrainz identifiers, ReplayGain/R128 data, and artwork provenance | Claimed/current breadth uncertain; corpus-test FLAC/MP3/M4A/Opus, Unicode, multi-disc, and compilation albums. |
| TMDB enrichment | Owner token, local provider cache, first search result selection ([provider at `aa6ded9`](https://github.com/MikeO7/kinosail/blob/aa6ded98083ae48ef0377529b1000bfabd1ce6c0/internal/server/metadata_provider.go)) | Store provider ID, language, confidence/provenance, conditional-refresh metadata, and require disambiguation when ambiguous | Current but fragile matching. Contract-test fake provider responses; manually evaluate a diverse title/year corpus. |
| Artwork, cast, thumbnails, Show/Episode grouping | Local art variants, cast/provider art, generated WebP paths, filename grouping | Make generation jobs idempotent and recipe-versioned; retain original art and orientation metadata | Mixed current/planned. Validate cache invalidation, animation, EXIF rotation, and extreme images. |
| Chapters and named playback markers | ffprobe chapters; title heuristics for intro/credits ([probe at `aa6ded9`](https://github.com/MikeO7/kinosail/blob/aa6ded98083ae48ef0377529b1000bfabd1ce6c0/internal/server/probe.go)); separate marker research note | Keep embedded/manual markers authoritative; introduce automatic analysis behind confidence and review gates | Current basic behavior. See [automatic segment detection research](automatic-playback-segment-detection.md). |
| Lyrics, music disc/track ordering | Sidecar lyrics and basic disc/track fields | Preserve synchronized-vs-unsynchronized lyric type and language; use canonical album/artist IDs | Current/basic; corpus validation needed. |
| Photo dates/folders/albums/timeline/slideshow | Folder grouping; filesystem modification time is used as photo date | Prefer EXIF/XMP capture time with timezone and orientation, then filesystem fallback; never rewrite originals | Current/basic; validate RAW/HEIC policy, missing timezone, duplicates, and very large albums. |
| Direct playback and range seeking | `http.ServeFile` for `/media`, `/download`, and Jellyfin stream/download routes ([files](https://github.com/MikeO7/kinosail/blob/aa6ded98083ae48ef0377529b1000bfabd1ce6c0/internal/server/files.go), [Jellyfin playback](https://github.com/MikeO7/kinosail/blob/aa6ded98083ae48ef0377529b1000bfabd1ce6c0/internal/server/jellyfin_playback.go)) | Keep as the first choice; add strong representation identity for offline download and truthful codec/container declarations | Current byte ranges; mutable download identity remains unsafe. Validate RFC 9110 `206`, suffix/open ranges, `416`, validators, replacement during transfer, and 10+ GiB files. |
| Direct/remux/audio-transcode/full-transcode decision | Browser-oriented container/codec heuristics and a shallow ffprobe schema | Build one normalized MediaFacts × ClientCapabilities × Policy decision operation used by web and compatibility API | Current partial/claimed breadth. Needs golden decision fixtures and real-device probes. |
| HLS transcoding, bitrate, hardware acceleration, tone mapping, audio processing | Two fixed whole-title AVC/AAC/fMP4 variants; settings-based encoder/filter choices | Publish playable segments progressively, select the minimum sufficient representation, schedule by measured resources, and branch explicitly for HDR and subtitle cases | Current basic transcode; several README claims are broader than code. See detailed section below. |
| Sidecar/embedded subtitles and preferences | Sidecar VTT/SRT; minimal SRT-to-VTT rewrite; filename-based track labels; no embedded-stream inventory in committed probe | Model captions, subtitles, forced narrative, SDH, commentary, descriptions, and image subtitles distinctly; extract/text-convert or burn in according to type | Sidecar current; embedded extraction claimed, not current. Validate languages, dispositions, encodings, styling, timing, and assistive technology. |
| Audio tracks/preferences | Basic audio stream labels and selected HLS audio index | Preserve source stream index, language, role, default/forced flags, channels/layout, codec, and accessibility role; choose by profile plus client ability | Current/basic. Test commentary, audio description, Atmos/core fallbacks, and language subtags. |
| Next Episode | Organized episodic model and playback UI/API | Keep deterministic order with specials and multi-episode policy explicit | Current/protocol-tested; verify anime/absolute order, specials, missing episodes. |
| Offline downloads | Official Jellyfin-compatible routes exist, but bytes are the mutable library file | Introduce an immutable Download Resource with strong digest, length, lifecycle, resumable range semantics, and explicit ready-offline state | Current route, **not reliable offline contract**. See [mobile offline research](mobile-offline-downloads.md). |
| Owner and Viewer Profiles | Local profiles, 12-character minimum, Argon2id, sessions, passkeys, OIDC, API keys, scoped policies | Retain local-first auth; make authorization deny-by-default at application operations, rotate/revoke sessions and keys, and log security events without secrets | Current broad surface; requires adversarial authorization matrix and passkey/OIDC deployment testing. |
| Library/rating/remote/transcode/bitrate/Live-TV/download policies | Profile fields and request enforcement at multiple handlers | Centralize policy evaluation in the shared operation layer; expose reason-coded denials consistently to API/web/client adapters | Current breadth; prove every media-bearing route, indirect image/subtitle route, and compatibility alias. |
| Live TV, guide, stream limits, DVR | M3U/XMLTV ingest, Server proxy, scheduled FFmpeg copy recordings, polling/conflict count | Normalize channel/guide identity, bound upstream/network work, persist scheduler state transactionally, and verify discontinuity/error recovery | Current/basic; no tuner/network certification. |
| SyncPlay / Watch Together | In-memory rooms and leader-broadcast WebSocket behavior | Add monotonic room revision, authoritative snapshot, server timestamps, reconnect/resync, bounded queues, and drift correction | Current basic. Measure real WAN jitter; do not add consensus/distributed infrastructure. |
| Scheduled encrypted backups | State archive, encryption option, validation, seven-file retention | Version manifests, use authenticated encryption, add off-host copy guidance, and make restore drills a release gate | Current/protocol-tested. A backup is not proven until restored into a clean installation. |
| Mutation audit, diagnostics, cache maintenance | JSONL audit and administrative diagnostics/cache actions | Use structured redacted events, stable low-cardinality metrics, bounded retention, and separate health/readiness | Current/basic; inspect leakage and cardinality under load. |
| Installable PWA and authenticated JSON API | PWA/web plus versioned local API | Keep all capabilities in the API-driven monolith; add OpenAPI drift checks and accessibility/device matrix | Current broad surface. Live-TV route coverage and claimed behaviors need contract reconciliation. |
| Direct remote access | Public HTTPS and Owner-paired WireGuard; Server/Viewer connect directly | Make connection type and privacy properties visible; use standards-based HTTPS/VPN options | Test NAT classes, certificate renewal, peer revocation, and off-LAN devices. |
| Distribution, install, rollback | Multi-architecture OCI image, installer checksums, recovery backup, version pin | Sign release artifacts and OCI images, publish SBOM/provenance, digest-pin installs, and test rollback-compatible migrations | Current basic pipeline; checksums alone do not authenticate publisher. |
| Casting and DLNA | Browser Remote Playback/AirPlay affordances and an opt-in minimal UPnP ContentDirectory/direct stream | Describe these narrowly; expand only against official receiver/device protocols and certification fixtures | Experimental/basic, not broad Chromecast/DLNA parity. Physical-device tests required. |

Repository evidence for the matrix is pinned to the baseline commit: [browse/API operations](https://github.com/MikeO7/kinosail/tree/aa6ded98083ae48ef0377529b1000bfabd1ce6c0/internal/server), including `api_browse.go`, `home.go`, `lists.go`, `playlist.go`, `collection.go`, `recent.go`, and `progress.go`; [playback and compatibility adapters](https://github.com/MikeO7/kinosail/tree/aa6ded98083ae48ef0377529b1000bfabd1ce6c0/internal/server), including `player.go`, `transcoder.go`, `hls.go`, `probe.go`, `files.go`, `subtitle_provider.go`, `trickplay.go`, and `jellyfin_*.go`; [identity and policy](https://github.com/MikeO7/kinosail/tree/aa6ded98083ae48ef0377529b1000bfabd1ce6c0/internal/server), including `profiles.go`, `sessions.go`, `credentials.go`, `passkeys.go`, `oidc.go`, `access.go`, and `api_keys.go`; and [operational/extended features](https://github.com/MikeO7/kinosail/tree/aa6ded98083ae48ef0377529b1000bfabd1ce6c0/internal/server), including `live_tv.go`, `watch_together.go`, `backup_manager.go`, `audit.go`, `dlna.go`, and `api_openapi.json`. The baseline [container and installer files](https://github.com/MikeO7/kinosail/tree/aa6ded98083ae48ef0377529b1000bfabd1ce6c0) support the deployment/distribution rows. Tests in the same tree support only the explicitly labeled protocol-tested statements.

## Priority-zero truth and contract reconciliation

Before expanding the surface, reconcile these committed-code facts with the README, OpenAPI, tests, and older research notes:

1. **Media probe and embedded tracks.** The committed ffprobe query captures video/audio codec names, video dimensions, audio language/title, duration, and chapters. It does not capture subtitle streams, codec profile/level, pixel format/bit depth, frame rate, channel layout, dispositions, color primaries/transfer/matrix, mastering metadata, content-light metadata, Dolby Vision, or HDR10+ side data ([probe at `aa6ded9`](https://github.com/MikeO7/kinosail/blob/aa6ded98083ae48ef0377529b1000bfabd1ce6c0/internal/server/probe.go)). Those facts are prerequisites for truthful playback choices.
2. **Jellyfin `PlaybackInfo`.** The committed handler ignores the submitted device profile, emits a synthetic stream inventory, declares direct play/direct stream for every non-photo item, and declares transcoding unsupported ([Jellyfin playback at `aa6ded9`](https://github.com/MikeO7/kinosail/blob/aa6ded98083ae48ef0377529b1000bfabd1ce6c0/internal/server/jellyfin_playback.go)). This is a compatibility scaffold, not device-driven negotiation.
3. **Offline immutability.** Both Kinosail and Jellyfin download paths call `ServeFile` on the mutable library path. `ETag` in Jellyfin media-source metadata is an item ID derived from path, not a digest of the bytes. Replacement at the same path can corrupt an Android resume that sends only `Range`.
4. **HLS startup.** The master playlist appears only after the two fixed renditions complete whole-title encoding. A client cannot start after the first segments become available.
5. **Refresh triggers.** The committed scanner reconciles on startup, manual request, and periodic interval; no filesystem-event watcher is present.
6. **Admission and fallback.** Two per-item rendition goroutines are not global two-job admission control. Encoder selection does not establish automatic retry on a software encoder after a runtime hardware failure.
7. **API description.** User-visible Live TV routes exist in handlers but are absent from the embedded versioned OpenAPI surface inspected for this baseline. Architecture policy requires every capability in the API contract.

The corrective method should be mechanical: a generated route/operation inventory, golden compatibility contracts, README assertions linked to evidence, and explicit “protocol-tested” versus “device-certified” badges. Closed issues and passing handler tests are not substitutes for end-to-end proof.

## 1. Ingest, identity, catalog, metadata, browse, and search

### Current architecture

The scanner recursively walks configured roots, skips symlinks, recognizes an explicit media extension list, parses filename/NFO/sidecar conventions, and produces an in-memory index. Item identity is the first 64 bits of SHA-256 over namespaced relative path, so renames change identity while replacing bytes at the same path retains identity. JSON stores are written by temporary-file rename ([state persistence at `aa6ded9`](https://github.com/MikeO7/kinosail/blob/aa6ded98083ae48ef0377529b1000bfabd1ce6c0/internal/server/state.go)); they are not a transactional catalog spanning library, metadata, jobs, and user state.

### Recommendation

- Keep full reconciliation scans. Filesystem events are **hints**, not a source of truth: Linux inotify queues can overflow and applications must detect loss and rescan ([Linux inotify, man-pages 6.16, 2025](https://www.kernel.org/pub/linux/docs/man-pages/book/man-pages-6.16.pdf)). Debounce directory events, avoid watching outside roots, and retain periodic/startup reconciliation.
- Separate `ItemID` (Kinosail identity), `FileVersion` (size, modification/change identity, and optional digest), and external metadata IDs. This makes rename/move reconciliation and cache invalidation explicit. Content hashing should be asynchronous and demand-driven for large media, not mandatory during every scan.
- Adopt embedded SQLite only when current JSON/in-memory limits become material. It fits the one-container boundary, gives atomic cross-record changes, migrations, indexes, and FTS5. SQLite WAL permits concurrent readers with one writer but is single-host storage; use a SQLite version containing the 2026 WAL-reset fix and keep the database on a supported local filesystem ([SQLite WAL, updated 2026-03](https://www.sqlite.org/wal.html)).
- Use FTS5 Unicode tokenization and BM25 ranking for local title/person/album/plot search rather than adding Elasticsearch or another sidecar ([SQLite FTS5](https://www.sqlite.org/fts5.html)). Add normalized exact/prefix fields and filters; benchmark before adding fuzzy/semantic search.
- Store metadata per field with source, provider ID, language, fetched time, source revision/ETag, and owner-lock state. Local NFO and owner edits should win deterministically. TMDB recommends search followed by detail lookup; selecting the first fuzzy result without confirming year/provider ID is unsafe ([TMDB search/detail guidance, current 2026](https://developer.themoviedb.org/docs/search-and-query-for-details)). Preserve attribution/licensing requirements.
- For music, store MusicBrainz identifiers when present rather than treating names as identity; MBIDs are stable UUIDs intended to identify database entities ([MusicBrainz Identifier documentation, current 2026](https://musicbrainz.org/doc/MusicBrainz_Identifier)). Local tags remain authoritative unless the owner chooses enrichment.
- For photos, parse embedded EXIF/XMP date/time, timezone, orientation, and camera metadata with strict resource bounds; filesystem modification time is only a fallback.

### Validation

Build corpus fixtures for rename, atomic source replacement, duplicate titles/years, multi-disc compilations, Unicode normalization, NFO/provider conflicts, timezone-less photos, corrupt metadata, deep trees, unreadable directories, and watcher overflow. Record scan duration, peak memory, changed-items count, false metadata matches, and search latency.

## 2. Playback negotiation: one truthful decision model

The durable seam should be one application operation:

`DecidePlayback(MediaFacts, ClientCapabilities, ViewerPolicy, NetworkIntent) -> PlaybackPlan`

`PlaybackPlan` should state the selected source/streams, direct/remux/transcode mode, container, codecs and actual codec strings, subtitle delivery, audio transform, color/HDR behavior, bitrate/resolution, reasons, cache recipe, and authorization result. The web player, versioned API, and Jellyfin adapter should translate to and from this operation; no adapter should invent compatibility.

Media facts need source identity plus container; video codec/profile/level/bit depth/pixel format/frame rate/resolution/SAR; color primaries/transfer/matrix/range and HDR static/dynamic metadata; audio codec/profile/channels/layout/sample rate/language/role/default; subtitle codec/language/role/dispositions/text-vs-image/styling; chapters; duration; and seekability. [ffprobe](https://ffmpeg.org/ffprobe.html) can emit the underlying stream, frame, format, and side-data fields; the normalization and validation belong to Kinosail.

Client capability evidence should be layered:

1. explicit official-client device profiles or server-maintained device fixtures;
2. browser Media Capabilities queries where available—useful but still a W3C Working Draft as of 2026-06-09 ([Media Capabilities](https://www.w3.org/TR/media-capabilities/));
3. conservative platform defaults; and
4. observed failure telemetry that never exposes media titles or paths.

The direct-play URL and declared MIME `codecs` parameters must describe the actual representation. Hard-coded generic `avc1`/`hvc1` labels cannot safely represent arbitrary profile, level, tier, or bit depth.

## 3. Transcoding, HLS, adaptive streaming, and cache economics

### What the committed transcoder does

At `aa6ded9`, one HLS request creates a shared job keyed by item ID and selected audio index. It launches both of these full-title encodes concurrently:

| Rendition | Scale | Video | Audio |
|---|---:|---:|---:|
| `high` | width 1920 | AVC, 6,000 kbit/s | AAC, 160 kbit/s |
| `low` | width 1280 | AVC, 2,500 kbit/s | AAC, 128 kbit/s |

The output is fragmented MP4 HLS with four-second forced keyframes/segments. Each media playlist starts as `EVENT`, is rewritten to `VOD` only after FFmpeg exits, and the master playlist is written only when **both** renditions are fresh ([HLS manager at `aa6ded9`](https://github.com/MikeO7/kinosail/blob/aa6ded98083ae48ef0377529b1000bfabd1ce6c0/internal/server/hls.go)). Therefore playback waits for both concurrent whole-title encodes to finish, disk consumption is incurred for both variants even if the client needs one, and the player cannot consume already-produced segments. The cache freshness check is source modification time plus a transcoder-setting string; it is not a full source/recipe identity. Jobs are shared per item/audio choice, but there is no global resource scheduler or bounded eviction policy.

### Recommended near-term design

1. **Direct play first, remux second.** On a home LAN, copying compatible streams into a supported container is cheaper and preserves quality. Only transform the incompatible dimensions of the representation.
2. **Publish progressively.** Atomically publish a master and growing media playlist as soon as the initialization fragment and a safe startup buffer exist. HLS explicitly supports Media Playlists that change over time; segment availability, target duration, media sequence, and end-list behavior must remain conformant ([RFC 8216, August 2017](https://www.rfc-editor.org/rfc/rfc8216)). Do not rewrite a growing playlist as final until the job is complete.
3. **Generate the minimum useful output.** For a known local client and stable network, create one tailored representation. Create a small aligned ladder only when the caller requests adaptive remote playback or measurements show sustained throughput variation. Never upscale; include source resolution/bitrate as a ceiling.
4. **Align closed GOP boundaries.** Every rendition in an adaptive set must switch at matching independent boundaries. Apple’s current authoring specification recommends two-second video segment/keyframe cadence and supplies codec, HDR, audio, caption, and bandwidth requirements ([HLS Authoring Specification for Apple Devices, revision history through 2025-06-26](https://developer.apple.com/documentation/http-live-streaming/hls-authoring-specification-for-apple-devices/)). Kinosail should tune the segment duration from measured startup/overhead rather than assume four seconds is universally best.
5. **Schedule real resources.** Admission should account for decoder, filter, encoder, GPU-session, CPU, memory, disk, and I/O demand. One source feeding two renditions is not equivalent to two independent jobs. Bound queues; make cancellation, owner priority, live-TV deadlines, and fairness explicit.
6. **Make recipes immutable.** Cache keys should include source `FileVersion`, selected streams, crop/scale/tone-map/subtitle/audio transforms, encoder and relevant FFmpeg version, target codec parameters, segment settings, and recipe schema version. Publish through a staging directory/atomic manifest switch. Track bytes, last access, recompute cost, and pin count; evict by quota and pressure, never `rm` an active representation.

This is deliberately less elaborate than commercial encoding pipelines. A fixed two-rung ladder is easy to reason about but wasteful for many local plays; a dozen-rung per-title ladder is operationally excessive for a single household. First measure time-to-first-frame, rebuffer ratio, failed starts, CPU/GPU saturation, bytes encoded-but-never-read, cache hit rate, and quality at representative displays/networks.

### Where per-title analysis and adaptive-bitrate research help

Netflix’s production per-title work shows why one ladder is inefficient across animation, grainy film, and high-motion content ([Netflix, “Per-Title Encode Optimization,” 2015](https://netflixtechblog.com/per-title-encode-optimization-7e99442b62a2)). [VMAF](https://github.com/Netflix/vmaf) supplies an open perceptual-quality measurement implementation. Kinosail should use these **offline to evaluate a representative corpus** and, later, to choose a small content-aware representation—not run expensive multi-pass optimization by default on every home-server play.

Academic ABR work such as Robust MPC ([SIGCOMM 2015 paper](https://www.cs.cmu.edu/~xia/resources/Documents/Yin_sigcomm15.pdf)) and Pensieve ([SIGCOMM 2017 paper](https://web.mit.edu/pensieve/content/pensieve-sigcomm17.pdf)) is useful for understanding the quality/rebuffer/startup trade space. It does not justify a learned controller in Kinosail before a conventional throughput/buffer strategy is measured on its actual clients.

Media over QUIC (MoQ) should be monitored, not adopted for ordinary VOD. The IETF working group targets low-latency live ingest and distribution and remains active/in development ([MoQ charter, updated 2025-04-09](https://datatracker.ietf.org/doc/charter-ietf-moq/), [working-group drafts, current 2026](https://datatracker.ietf.org/wg/moq/)). Its own priority discussion marks VOD as a nice-to-have after live/realtime work ([IETF MoQ priorities, 2024 interim](https://datatracker.ietf.org/meeting/interim-2024-moq-05/materials/slides-interim-2024-moq-05-sessa-moq-priorities-interim-victor-00)). HTTP byte ranges and HLS/DASH are mature and sufficient for Kinosail VOD.

## 4. Codecs, hardware acceleration, HDR/color, and audio

Hardware acceleration is not one Boolean. Decode, upload/download, filters, tone mapping, subtitle composition, and encode must be validated as a pipeline. The official Jellyfin documentation correctly separates codec support from hardware-acceleration configuration and notes platform/device variation ([Jellyfin codec support, current 2026](https://jellyfin.org/docs/general/clients/codec-support/), [hardware acceleration, current 2026](https://jellyfin.org/docs/general/post-install/transcoding/hardware-acceleration/)).

Recommended rules:

- Detect available FFmpeg decoders/encoders/filters and perform a short functional startup probe, not only name detection. On hardware failure, retry once with an explicitly safe software recipe when policy/capacity allows; expose the fallback reason.
- Keep codec choices client-led. AVC/AAC is a conservative fallback. HEVC, AV1, VP9, Opus, E-AC-3, TrueHD, DTS, and lossless audio require exact client/container/profile checks and licensing/distribution review.
- Preserve source frame rate and aspect ratio unless a plan explicitly changes them. Avoid unconditional `yuv420p` if the selected path is HDR or higher bit depth.
- Separate loudness metadata from destructive normalization. [EBU R 128 version 5, 2023-11-21](https://tech.ebu.ch/publications/r128) provides the broadcast loudness framework; profile playback gain should be reversible and should avoid clipping. Preserve channel layout and downmix metadata.

### HDR branches must be explicit

1. **Direct play/remux HDR preservation:** preserve bit depth, color primaries, transfer, matrix/range, mastering display and content-light metadata, and the format-specific dynamic metadata. HDR10 static metadata, HLG, Dolby Vision profiles/compatibility layers, and HDR10+ dynamic metadata are distinct cases.
2. **HDR-to-HDR transcode:** not a baseline promise. It can require dynamic metadata handling that common filter/encoder paths do not preserve. Only expose combinations proven by bitstream inspection and target-device playback.
3. **HDR-to-SDR fallback:** when the display/client cannot consume the source HDR representation, run an explicit colorspace-linearize/tone-map/gamut-map/output-conversion pipeline with a declared target. FFmpeg documents `zscale`, `tonemap`, `tonemap_opencl`, `tonemap_vaapi`, and related filters, but availability and metadata behavior are build/device dependent ([FFmpeg filters, current 2026](https://ffmpeg.org/ffmpeg-filters.html)). Compare against reference frames and multiple displays; a hard-coded operator and nominal peak are not universally correct.
4. **Never silently strip metadata.** If safe preservation or conversion is unavailable, report the incompatible plan rather than emit incorrectly tagged or washed-out video.

## 5. Subtitles, captions, audio tracks, and accessibility semantics

Captions are not merely foreign-language subtitles. The domain/API model should preserve at least:

| Track purpose | Meaning | Preferred handling |
|---|---|---|
| Translation subtitles | Dialogue translated for a listener who can hear | Select by language preference; external text where supported. |
| Captions / SDH | Dialogue **and meaningful non-speech audio** for deaf/hard-of-hearing viewers | Preserve caption/SDH role, styling, speaker/sound cues, and accessibility label. |
| Forced narrative | Only text needed to understand foreign/fictional/on-screen language | Auto-select according to audio language and explicit forced disposition. |
| Audio description | Narrated visual information, normally an audio track | Preserve role separately from language and commentary. |
| Commentary | Alternate audio, not a default-language substitute | Never auto-select merely because language matches. |
| Chapters/metadata | Navigation or machine metadata | Do not expose as captions/subtitles. |

[WebVTT](https://www.w3.org/TR/webvtt/) (W3C Candidate Recommendation Draft, 2026-05-20) represents captions, subtitles, descriptions, chapters, and metadata but those kinds remain semantically distinct; its draft status also argues for testing actual clients. For richer professional timed text, [IMSC 1.2](https://www.w3.org/TR/ttml-imsc1.2/) is a W3C Recommendation (2020-08-04). Track language must use valid BCP 47 language tags, not only two/three lowercase letters ([RFC 5646, September 2009](https://www.rfc-editor.org/rfc/rfc5646.html)). WCAG 2.2 requires captions for prerecorded synchronized media at Level A, live captions at Level AA, and audio description for prerecorded video at Level AA, alongside keyboard, focus, contrast, and status requirements ([WCAG 2.2, W3C Recommendation 2023-10-05](https://www.w3.org/TR/WCAG22/)).

Kinosail needs three delivery paths:

1. **External text track:** WebVTT for the web/HLS clients; preserve cue timing/settings and map timestamps correctly. A comma-to-period substitution is not a complete SRT parser. Bound file size, reject invalid encodings/control data, and convert charset to UTF-8.
2. **Text extraction/conversion:** extract embedded SRT/ASS/SSA/WebVTT/TTML where client policy permits. ASS styling may not map losslessly to WebVTT; disclose fallback.
3. **Image/styled subtitle composition:** PGS/VobSub and fidelity-critical ASS require burn-in when the client cannot render them. This is a distinct full video-transcode plan and must participate in HWA/filter compatibility and cache identity. “Subtitle transcode” must not be treated as a cheap text conversion.

Provider search should prefer strong media identifiers or filename/hash plus title/year/episode, then language, forced/SDH flags, release match, provider provenance, and owner confirmation. “First result” is not a reliable choice. Downloaded content is untrusted: retain the existing allowlist/size controls, add robust parsing/encoding validation, keep credentials out of logs/backups, and store provider/license attribution.

Validation needs a corpus of VTT, SRT encodings, ASS styling, TTML/IMSC, PGS, VobSub, overlapping/bidi/vertical cues, long lines, malformed timing, forced/SDH/default flags, and audio description/commentary tracks. Test screen readers, keyboard-only selection, captions on/off persistence, and official mobile clients—not just HTML tokens.

## 6. Playback state, SyncPlay, and automatic markers

Progress updates should be idempotent and ordered per playback session. Carry a session ID plus client event sequence or revision, source duration/version, position, watched decision, and server timestamp. Reject stale replays after newer events; define the finish threshold and restart behavior rather than inferring “not watched” from every positive position.

SyncPlay needs a small state machine, not distributed consensus: authoritative leader/member state, monotonic revision, media/file version, play/pause/seek event, effective server time, reconnect snapshot, bounded outbound queues, and periodic drift measurement/correction. Measure behavior over latency/jitter/packet loss before tuning.

For markers, embedded chapters and owner edits remain authoritative. Automatic detection should be confidence-gated and reversible. Cross-episode audio fingerprints and visual/temporal features are supported by production and academic evidence, but false skips are more harmful than missed skips. The detailed evidence, including WACV 2021 intro/recap work and Chromaprint/Comskip production sources, is in [automatic playback segment detection](automatic-playback-segment-detection.md).

## 7. Official Jellyfin Mobile and offline downloads

Compatibility should target the actual official Android/iOS source contracts frozen in [mobile offline downloads](mobile-offline-downloads.md), not an abstract Jellyfin resemblance. The important current observations are:

- Android fetches batch item details through `/Items?Ids=...&Fields=MediaSources,Path`, persists `Path`/`ServerId`, downloads with `ApiKey`/`api_key`, and resumes with `Range` without relying on `If-Range`.
- Therefore the same download URL must keep immutable bytes and length for its lifecycle. A completed-size range request needs correct `416 Range Not Satisfiable`, `Content-Range: bytes */length`, and an empty body.
- iOS and Android differ in queueing and progressive-rendition behavior; a web handler fixture does not certify either app.

HTTP conditional and range semantics are defined by [RFC 9110, June 2022](https://www.rfc-editor.org/rfc/rfc9110.html). A reliable design is an immutable Download Resource identified by source version plus representation recipe, with strong digest, strong ETag, fixed length, original/transcoded type, readiness state, creation/last-access/expiry, and authorization. Download generation uses staging plus atomic publication; the client must not see “ready offline” until bytes, digest, and metadata are durable.

Kinosail should implement the smallest exact Jellyfin contract required by official Mobile and return a clear unsupported response for unimplemented playback profiles. Do not claim full Jellyfin server compatibility. Certification gates: pinned Android/iOS versions, fresh download, pause/resume, app/server restart, exact-completion range, 10+ GiB file, constrained/lost network, source replacement, storage exhaustion, revoke/delete, transcoded rendition, and actual offline playback after the server disappears. The official Jellyfin server’s download endpoint is a useful comparison, not a complete Kinosail contract ([Jellyfin 10.11.11 `GetDownload` source](https://github.com/jellyfin/jellyfin/blob/1fbd8739292cce610231be93daf43368733edf63/Jellyfin.Api/Controllers/LibraryController.cs#L657-L715)).

## 8. Authentication, profiles, permissions, and application security

The baseline’s Argon2id parameters align with the memory-constrained option in OWASP guidance, and RFC 9106 standardizes Argon2 ([RFC 9106, September 2021](https://www.rfc-editor.org/rfc/rfc9106.html); [OWASP Password Storage Cheat Sheet, current 2026](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html)). Retain versioned encoded parameters so hashes can be upgraded after successful login.

Recommendations:

- Centralize authorization in application operations and use deny-by-default route tests. Profiles, sessions, API keys, Jellyfin aliases, images, subtitles, HLS fragments, Live TV, recordings, downloads, WebSockets, diagnostics, and remote-access pairings all require object- and operation-level checks.
- Use opaque high-entropy session tokens stored only as hashes; rotate on authentication/privilege change; revoke on password reset/profile disablement; enforce idle/absolute lifetimes; set `Secure`, `HttpOnly`, and an appropriate `SameSite` policy behind trusted proxy configuration. Follow [OWASP session guidance](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html).
- Prefer passkeys/WebAuthn for phishing resistance while keeping deliberate recovery. WebAuthn Level 3 is a W3C Candidate Recommendation Snapshot dated 2026-05-26 ([WebAuthn Level 3](https://www.w3.org/TR/webauthn-3/)). Verify RP ID/origin, user verification policy, signature counters as signals, credential backup state, and recovery. Treat synced passkeys according to the assurance and sync-fabric requirements in [NIST SP 800-63B-4](https://pages.nist.gov/800-63-4/sp800-63b.html) and its normative [Syncable Authenticators appendix](https://pages.nist.gov/800-63-4/sp800-63b/syncable/), rather than assuming every passkey has identical assurance.
- For external identity, implement the OIDC authorization-code flow with exact issuer/audience/nonce/state/PKCE validation and stable subject binding; do not use display/email as identity ([OpenID Connect Core 1.0, 2014 with errata](https://openid.net/specs/openid-connect-core-1_0.html)). Apply the current OAuth security baseline in [RFC 9700, January 2025](https://www.rfc-editor.org/rfc/rfc9700.html); for browser-based clients, follow authorization-code plus PKCE and the architecture-specific token-handling guidance in [RFC 10017, 2026](https://www.rfc-editor.org/rfc/rfc10017.html), rather than older implicit-flow advice.
- Rate-limit authentication and expensive/transcoding endpoints by account and network with privacy-safe logs. Avoid account enumeration. Require CSRF defenses on cookie-authenticated mutations and a restrictive CSP/Fetch Metadata policy for the web adapter.
- Use [OWASP ASVS 5.0](https://owasp.org/www-project-application-security-verification-standard/) as a verification checklist, not as proof by declaration. Threat-model malicious media/subtitle metadata, FFmpeg/ffprobe invocation, archives, SSRF in provider/playlist/XMLTV URLs, path traversal, XML entity/resource exhaustion, and decompression/image bombs.

## 9. Direct remote connectivity, discovery, and privacy

Kinosail exposes direct remote access through public HTTPS or Owner-paired WireGuard. The Server remains authoritative for authentication and authorization.

Recommended connection order:

1. verified direct HTTPS hostname with ACME-managed certificate ([RFC 8555, March 2019](https://www.rfc-editor.org/rfc/rfc8555.html));
2. user-owned WireGuard/VPN connection; WireGuard’s protocol has formal verification work, though deployment configuration remains part of the trusted system ([WireGuard formal verification, 2018](https://www.wireguard.com/formal-verification/));
3. explicit compatibility/manual direct endpoint.

ICE and STUN can improve reachability diagnostics and candidate selection ([RFC 8445, July 2018](https://www.rfc-editor.org/rfc/rfc8445.html); [RFC 8489, February 2020](https://www.rfc-editor.org/rfc/rfc8489.html)). TURN is a media relay by definition ([RFC 8656, February 2020](https://www.rfc-editor.org/rfc/rfc8656.html)) and does not fit the direct-only design. Do not silently open router ports with UPnP; make every connectivity/privacy mode visible and owner-controlled.

Test common NAT types, IPv4/IPv6, carrier-grade NAT failure, DNS rebinding, forwarded-header trust, certificate issuance/renewal, WireGuard peer revocation, and connection-type truth in the UI.

## 10. Live TV, DVR, casting, and DLNA

M3U and XMLTV inputs are hostile network/file inputs. Apply URL-scheme and redirect policy, SSRF protections, response/time/decompression limits, streaming XML parsing without external entities, normalized channel identity, and explicit timezone handling. Refresh into a staged guide and atomically swap it so a failed provider does not erase a good guide.

Live playback has deadline priority over background VOD/cache jobs. Track per-upstream/tuner stream capacity and one shared source where lawful/appropriate rather than counting only handler requests. DVR scheduling needs persisted state transitions, overlap/conflict reasons, padding, partial-recording status, restart catch-up, disk reservation/low-space behavior, and safe filenames. `-c copy` is efficient but must tolerate discontinuities and container/timestamp defects; verify recordings before marking complete.

Watch/Cast labels must reflect implemented protocols. Browser Remote Playback and AirPlay-picker hooks do not equal a native Chromecast receiver. The current DLNA subset is useful experimental direct play but not broad DLNA interoperability; real protocolInfo profiles, seek/range behavior, device quirk fixtures, eventing, SSDP network isolation, and physical certification are needed before a parity claim.

## 11. Persistence, jobs, cache, backups, observability, and updates

### Persistence and jobs

If JSON state remains small, retain atomic rename and add directory `fsync`/crash tests where durability matters. If cross-file consistency, catalog size, or job recovery becomes material, migrate to one embedded SQLite database with versioned transactional migrations. Use its online backup API for consistent snapshots ([SQLite Online Backup API](https://www.sqlite.org/backup.html)). Do not introduce a Kinosail-managed database/cache/queue sidecar.

Represent durable work—scan, metadata fetch, thumbnail, immutable download, optional analysis—as an idempotent job with type, normalized recipe, state, priority, attempts/backoff, timestamps, progress, owner/profile scope, lease/heartbeat if required, and cancel reason. Keep ephemeral per-request direct-play work out of the durable queue. Enforce global resource pools; retry only classified transient failures; make restart recovery deterministic.

### Backups

Backups should contain a schema/version manifest, integrity metadata, settings/auth/user/catalog state, and no Library Content or derived caches. Encrypt with authenticated encryption, document key-loss consequences, support owner-chosen off-host copy, and validate paths/types/sizes before restore. Recovery objectives are proven by automated clean-install restore plus periodic human-readable drills, consistent with contingency-testing guidance in [NIST SP 800-34 Rev. 1, May 2010](https://csrc.nist.gov/pubs/sp/800/34/r1/upd1/final). Include rollback/migration compatibility in the drill.

### Observability

Default to structured stdout events with timestamp, severity, component, operation, outcome/reason, duration, and opaque correlation ID. Never log tokens, subtitle/provider credentials, grant material, media URLs, titles, search text, full paths, or profile names by default. Metrics should be stable and low-cardinality: scan/job counts/durations, playback-plan modes/reasons, time-to-first-frame, transcode speed/queue/cache bytes, HLS failures, Live-TV upstream failures, backup/restore result, DB latency, and auth denials. Item/profile IDs do not belong in metric labels.

Keep liveness (“process can serve”) distinct from readiness (“safe to accept new operations”); do not make a long scan or optional provider failure restart the container. OpenTelemetry offers a vendor-neutral data model and protocol; the specification is 1.60.0 at this snapshot, while some semantic conventions remain Development ([OpenTelemetry specifications](https://opentelemetry.io/docs/specs/otel/), [semantic-convention status](https://opentelemetry.io/docs/specs/otel/semantic-conventions/)). Start with local logs/metrics and allow direct OTLP export later—no required sidecar.

### Updates and supply chain

Checksums detect accidental corruption but do not authenticate the publisher unless delivered through a trusted signed channel. Sign release bundles and OCI image digests, publish SBOM and provenance, pin installed artifacts by digest, and record the installed version. OCI Image and Distribution 1.1 were released 2024-03-13 and support artifact/referrer use cases such as signatures and SBOMs ([OCI 1.1 announcement](https://opencontainers.org/posts/blog/2024-03-13-image-and-distribution-1-1/)). Target [SLSA 1.2](https://slsa.dev/spec/v1.2/) provenance incrementally. TUF ([specification 1.0.36, 2026-08-05](https://theupdateframework.github.io/specification/latest/)) is appropriate only if Kinosail later needs delegated roles, compromise resilience, and rollback/freeze protection beyond a simple signed release channel.

Every release gate should verify upgrade from the oldest supported state, backup, migration interruption, rollback policy, health after update, image signature/provenance, and restored clean installation. Automatic update should remain owner-controlled.

## 12. Accessibility, localization, privacy, and product quality

Accessibility is cross-cutting, not a subtitle checkbox. Apply WCAG 2.2 to setup, auth, library grids, filters, dialogs, player controls, scrubber, track menus, marker-skip notices, Live TV guide, SyncPlay state, errors, and offline state. Required evidence includes keyboard-only completion, visible focus, logical focus after HTMX swaps, programmatic names/states, status announcements without focus theft, zoom/reflow, contrast, reduced motion, touch target size, captions, and audio description. Test VoiceOver and TalkBack on real clients.

Use BCP 47 tags throughout API/state; localize UI strings without translating stable machine enums; use locale-aware collation/display while preserving deterministic IDs. Bidirectional text, long translations, plural rules, time zones, and 12/24-hour preferences need fixtures. Avoid encoding English labels into domain meaning.

Privacy defaults should keep playback, searches, filenames, and household relationships local. Make every outbound provider call enumerable in diagnostics, with purpose and redacted destination. Telemetry should be opt-in, data-minimized, and independently disableable from security/update checks.

## Where academic search adds value—and where it does not

| Problem | Best evidence | Kinosail use |
|---|---|---|
| Perceptual quality and content-aware encoding | Peer-reviewed/production perceptual research plus codec tooling | Evaluate VMAF/quality-speed-size on a local corpus; later select a small per-title rendition. |
| ABR control under variable networks | Systems papers plus client telemetry | Inform a conventional buffer/throughput policy; learned ABR is premature. |
| Intro/recap/credits/commercial detection | Peer-reviewed datasets/models plus production fingerprint tools | Confidence-gated optional analysis after embedded/manual markers. |
| Audio/video fingerprints, duplicate/version matching | Signal-processing research and mature open implementations | Aid rename/version reconciliation without making full-file hashing a scan bottleneck. |
| Local recommendations | Recommender/evaluation literature | Only after privacy-safe signals, baselines, and offline metrics exist; keep explanations. |
| Cache/job scheduling | Systems research (cost-aware caching, deadline/fairness scheduling) | Useful after measurements show contention; simple bounded policies first. |
| HTTP range/cache semantics, HLS manifests | IETF/W3C/platform specifications and conformance tools | Specifications dominate; a paper cannot override wire contracts. |
| Codec/container/device compatibility | Official clients, platform docs/source, device tests, bitstream probes | Production evidence dominates. Maintain pinned fixtures and real-device matrix. |
| Captions/accessibility/language | W3C standards, WCAG, BCP 47, platform assistive technology | Standards and user testing dominate. Semantic roles must survive every adapter. |
| Auth, sessions, OIDC, WebAuthn, supply chain | Standards and authoritative security guidance | Follow protocols/checklists and adversarial tests; novel schemes are a liability. |
| Jellyfin Mobile compatibility | Pinned official Android/iOS/server source plus device tests | Exact observed contracts dominate; “similar API” is insufficient. |
| NAT/direct connectivity/privacy | IETF protocols, deployment measurements, threat model | Standards and real networks dominate; relay proposals must respect product boundary. |

## Decision-oriented roadmap

### P0 — make current behavior truthful and safe

1. Freeze a capability/route inventory from committed code; reconcile README, OpenAPI, tests, and research notes. Label protocol tests separately from device certification.
2. Expand normalized media probing and create the single playback decision operation. Make Jellyfin `PlaybackInfo` consume real device capabilities or return an honest unsupported plan.
3. Define immutable source/download identity and correct full HTTP range/validator behavior. Do not advertise reliable official-Mobile offline until the physical-client fault matrix passes.
4. Change HLS from two whole-title blocking encodes to progressive publication of the minimum useful plan; add global resource admission, cancellation, atomic recipes, and bounded cache eviction.
5. Close authorization coverage across every media-bearing route and compatibility alias; threat-model provider, M3U/XMLTV, subtitle, artwork, FFmpeg, archive, and proxy inputs.

### P1 — complete daily-use media fidelity

1. Model and expose all audio/subtitle stream facts and semantic roles. Implement embedded text extraction; make image-subtitle burn-in a distinct full-transcode case.
2. Establish explicit HDR passthrough/remux versus SDR tone-map behavior and certify each HWA path with bitstream/frame/device evidence.
3. Persist the catalog/jobs transactionally when benchmarks justify migration; add watcher hints with reconciliation, metadata provenance/disambiguation, and durable cache/download recipes.
4. Harden progress ordering, SyncPlay reconnect/drift, Live-TV ingest/DVR recovery, backup restore drills, redacted metrics, and signed/provenanced updates.
5. Run the accessibility/localization matrix in web, PWA, and official mobile contexts.

### P2 — measured enhancements

Evaluate per-title quality selection, automatic playback-segment detection, fingerprint-assisted identity, and recommendation ranking behind owner-controlled experiments. Expand native casting/DLNA/tuner coverage only with target protocols/devices. Monitor MoQ for live use; do not adopt it for ordinary VOD.

## Release evidence required before “best modern way” claims

- Golden MediaFacts and PlaybackPlan fixtures covering containers/codecs/profiles/HDR/audio/subtitles plus fuzzed malformed probe data.
- Bitstream/manifest validation and real playback for direct, remux, progressive HLS, seek, audio selection, text extraction, image-subtitle burn-in, HDR preservation, and HDR-to-SDR.
- Measured time-to-first-frame, rebuffering, quality, cache waste/hit rate, CPU/GPU/disk saturation, queue fairness, and cancellation on representative home hardware.
- Official Jellyfin Android/iOS physical-client download and playback fault matrix, not only handler tests.
- Authorization matrix, security abuse tests, dependency/container scan, signed artifacts, SBOM/provenance, and secret/privacy log inspection.
- Large-library scan/search benchmarks, watcher-overflow reconciliation, concurrent-state tests, crash consistency, upgrade/rollback, and clean restore.
- Keyboard, screen-reader, caption/description, contrast/reflow, localization/bidi, mobile, TV/remote-control, Live-TV/tuner, NAT/IPv6, and hardware-encoder matrices.

## Source policy and maintenance

This note records decisions as of 2026-08-22. Standards and official source links are primary. “Current” repository claims are tied to `aa6ded9` because unrelated implementation work was uncommitted during research. Refresh the pinned official-client revisions, platform codec tables, FFmpeg build/device matrix, W3C drafts, MoQ drafts, SQLite security/correctness fixes, OWASP guidance, and supply-chain specifications before implementing the associated phase.
