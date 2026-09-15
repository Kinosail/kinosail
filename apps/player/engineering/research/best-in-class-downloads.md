# Best-in-class offline downloads for Kinosail

Research snapshot: 2026-08-22.

This note defines the best download experience Kinosail can build while preserving two product constraints: media flows directly from the owner-hosted Kinosail Server to the viewer, and the supported installation remains one Kinosail Server container. It uses primary specifications, platform documentation, first-party client/server source, and original research papers. The existing [official Jellyfin client research](mobile-offline-downloads.md) remains the detailed compatibility record; this note turns that evidence into a broader product and protocol decision.

## Executive decision

Build downloads in two lanes, in this order:

1. **Jellyfin compatibility lane:** make `/Items/{id}/Download` an immutable, revision-bound original; preserve exact range behavior; retain old revisions long enough for real queues; publish correct validators and digests; and add only finalized progressive renditions through the existing iOS `PlaybackInfo` flow. This is the most Kinosail can do for unmodified official Jellyfin apps.
2. **Kinosail Download v1 lane:** add a versioned download-plan API and a native Kinosail client transfer engine. A plan names one immutable asset, its exact size, a whole-file SHA-256, fixed-size per-chunk SHA-256 values, selected tracks, and a renewable retention lease. The client persists a verified-chunk ledger, repairs only missing or corrupt chunks, atomically installs the completed bundle, opens it through a network-denying local playback probe, and only then reports **Ready offline**.

The second lane is the real product leap. Neither HTTP range support nor an `ETag` alone makes a download reliable: the URL must keep identifying the same bytes, the client must remember which bytes it verified, and completion must mean locally playable rather than merely “the request ended.” RFC 9110 explicitly positions byte ranges as recovery for partial transfers and defines `If-Range` as the validator-gated resume mechanism. [RFC 9110 §§13.1.5, 14](https://www.rfc-editor.org/rfc/rfc9110.html#section-13.1.5)

The intended promise is:

> Every verified chunk survives interruption. A stuck or corrupt chunk is retried by itself. A source replacement can never splice new bytes onto old bytes. “Ready offline” means the selected media and tracks were fully installed, verified, and opened locally with networking unavailable.

Do not market this as better than Plex or Jellyfin until the real-device fault matrix in this note passes. Plex's published mobile experience already separates preparation from transfer, shows remaining storage, allows pause/retry, and permits two concurrent items; those are useful baseline behaviors to match. Its public documentation does not establish a chunk-integrity or restart-safe protocol. [Plex mobile downloads](https://support.plex.tv/articles/download-ios-android/), [Plex Downloads FAQ](https://support.plex.tv/articles/downloads-sync-faq/)

## What the current clients prove

### Official Jellyfin apps

The official Android 2.7.1 downloader persists queue records and uses a foreground WorkManager worker, but processes media serially. It resumes with `Range: bytes=<local-size>-`, sends no `If-Range`, writes at the returned offset without truncating, and considers a stored file valid when its length matches. This makes byte immutability at the URL—not just a response validator—mandatory. [Android worker](https://github.com/jellyfin/jellyfin-android/blob/13abdff1cf2c157d8354b02f8e1f7d35e923260e/app/src/main/java/org/jellyfin/mobile/downloads/DownloadWorker.kt#L22-L82), [Android range writer](https://github.com/jellyfin/jellyfin-android/blob/13abdff1cf2c157d8354b02f8e1f7d35e923260e/app/src/main/java/org/jellyfin/mobile/downloads/FileDownloader.kt#L30-L123), [Android size check](https://github.com/jellyfin/jellyfin-android/blob/13abdff1cf2c157d8354b02f8e1f7d35e923260e/app/src/main/java/org/jellyfin/mobile/app/StorageManager.kt#L46-L59)

The official iOS 1.8.0.5 app starts up to three Expo downloads but does not persist the resumable task state or byte progress. On relaunch it converts `Downloading` back to `Pending`, and completion is based on the download promise plus later file existence rather than a digest or playback probe. Its optional rendition flow deliberately removes HLS profiles and follows a progressive static or transcoding URL. [iOS transfer loop](https://github.com/jellyfin/jellyfin-ios/blob/78bc20a0a63aec2b0f96d7ed2eaea4a2f9f9d40f/features/downloads/hooks/useDownloadHandler.ts#L36-L215), [iOS persisted-state restoration](https://github.com/jellyfin/jellyfin-ios/blob/78bc20a0a63aec2b0f96d7ed2eaea4a2f9f9d40f/stores/DownloadStore.ts#L50-L113), [iOS local-file check](https://github.com/jellyfin/jellyfin-ios/blob/78bc20a0a63aec2b0f96d7ed2eaea4a2f9f9d40f/screens/DownloadScreen.tsx#L94-L157)

Jellyfin Server's standard endpoint checks download permission and returns the original file with range processing. It does not select a device-compatible representation. Kinosail must preserve that meaning for unmodified clients rather than silently returning a different-quality file from `/Items/{id}/Download`. [Jellyfin 10.11.11 `GetDownload`](https://github.com/jellyfin/jellyfin/blob/1fbd8739292cce610231be93daf43368733edf63/Jellyfin.Api/Controllers/LibraryController.cs#L657-L715)

### Current Kinosail work in progress

The checkout's uncommitted snapshot implementation is directionally useful but is not yet the required identity model:

- the binding is derived from the current session token or remote address, not stable profile and device identity ([`downloads.go`](../../internal/server/downloads.go#L117));
- a binding is reused only for seven days and successful hits do not refresh `BoundAt`, after which the same logical URL can bind to different source bytes ([`downloads.go`](../../internal/server/downloads.go#L47));
- source reuse trusts only path + size + modification time rather than the already-computed content digest, and the store has no snapshot/binding garbage collection ([`downloads.go`](../../internal/server/downloads.go#L60));
- one global mutex is held across the complete copy and hash, so unrelated first downloads serialize and no first byte can be served until a multi-gigabyte duplicate is complete ([`downloads.go`](../../internal/server/downloads.go#L47));
- when no cache directory is configured, it serves the mutable source path and emits no content hash ([`downloads.go`](../../internal/server/downloads.go#L68)); and
- `Repr-Digest` currently places hexadecimal text between Structured Field Byte Sequence delimiters ([`downloads.go`](../../internal/server/downloads.go#L143)). RFC 9530 requires the raw digest bytes to be base64-encoded in that field, for example `sha-256=:<base64>:`. [RFC 9530 §3](https://www.rfc-editor.org/rfc/rfc9530.html#section-3)

A strong `ETag` can validate a representation, but it does not force a mutable URL to retain old bytes. For Android, which sends no validator, the compatibility route needs a persistent profile + device + item-revision binding or a revision-specific item ID. Expiry or eviction must return a clear failure such as `410 Gone`; it must never silently rebind and append a different representation. RFC 9110 defines `410` for a target resource that is no longer available and is likely permanently unavailable. [RFC 9110 §15.5.11](https://www.rfc-editor.org/rfc/rfc9110.html#section-15.5.11)

Replace that global lock with per-revision preparation jobs, per-key singleflight, and explicit admission control. One caller prepares a revision while other callers observe the same durable `preparing` job; unrelated ready assets continue to serve immediately.

## Target user experience

The product should make the state and the next action obvious without requiring the person to understand networking:

- A movie, episode, season, playlist, or “trip pack” can be queued in one action.
- Before work begins, Kinosail shows the exact or bounded device space required, the selected quality, included audio/subtitle tracks, the server preparation requirement, the current network policy, and an estimated ready time.
- **Original**, **Compatible**, and **Space saver** are explicit choices. Kinosail never silently changes quality.
- Preparation and transfer are separate progress bars. A cache miss that is remuxing or transcoding never looks like a stuck network transfer.
- Every waiting state says why: `Waiting for Wi-Fi`, `Server preparing a compatible copy`, `Phone storage needed`, `Server offline`, `Sign in again`, or `Download copy expired—prepare again`.
- Pause, resume, reprioritize, cancel, and retry are durable operations. Restarting the app, phone, container, or router does not reset verified work.
- Collection progress is derived from item states and sizes, not just item count.
- Completion has two levels: **Transferred** while verification/install is running, and **Ready offline** only after verification plus a local playback probe.
- A “Check trip readiness” action revalidates the local index, files, expected tracks, and playback probe without contacting the server. It reports exactly which items are ready.

These states apply to both the API and UI. The web adapter and future native client must call the same versioned application operations, consistent with Kinosail's API-driven monolith decision. [ADR 0005](../adr/0005-expose-complete-api-from-one-server-container.md)

## Durable state machine

```text
queued
  -> planning
  -> preparing             (only for snapshot/remux/transcode work)
  -> ready_to_transfer
  -> transferring <-------> paused
         |        <-------> waiting_network
         |        <-------> waiting_power
         |        <-------> waiting_storage
         |        <-------> waiting_server
         |        <-------> waiting_auth
         v
     verifying -> repairing -> transferring
         v
     installing
         v
     ready_offline
```

`canceled` is terminal. A permanent authorization denial, expired/evicted revision, incompatible requested track set, or unrecoverable storage error becomes `needs_attention` with a stable reason code and an explicit recovery operation. Transient errors remain in a waiting state with the next retry time; they do not become a vague terminal `failed` state.

Every transition is journaled before the side effect it authorizes. Server state survives container restarts in the same mounted Kinosail data directory; client state survives process death and device restart in the platform database. Coda's disconnected-operation work demonstrated the value of explicit durable local state through involuntary disconnection, although Kinosail's immutable, read-only media case is much simpler than Coda's writable-file reintegration problem. [Kistler and Satyanarayanan, “Disconnected Operation in the Coda File System”](https://www.cs.cmu.edu/~coda/docs-coda.html)

## Architecture and protocol

### One-container server components

All required server responsibilities remain modules inside the Kinosail Server process and its mounted data/cache directories:

- **Plan service:** validates viewer/download permission, device profile, requested tracks, quality, deadline, and storage budget; returns or reuses an idempotent plan.
- **Revision store:** creates and retains immutable originals and finalized renditions, keyed internally by content digest and rendition recipe.
- **Preparation scheduler:** admits snapshot, remux, and transcode jobs without starving ready-file serving or interactive playback.
- **Transfer service:** serves exact immutable assets with range support, validators, digest metadata, backpressure, and stable error semantics.
- **Lease and garbage collector:** renews active-plan retention independently from authorization expiry and evicts only unleased assets according to quota.
- **Event/metrics journal:** records state transitions and low-cardinality operational measurements without media titles, source paths, tokens, or asset URLs.

Remote downloads remain direct from the owner-hosted Server; no intermediary proxies, relays, caches, or inspects media bytes.

### Download v1 resources

The exact shape should be settled with API tests, but the protocol needs these semantics:

| Operation | Semantics |
| --- | --- |
| `POST /api/v1/download-plans` | Idempotently choose item revision, rendition, tracks, and deadline; return current plan state and size estimate. |
| `GET /api/v1/download-plans/{plan}` | Return durable state, preparation/transfer totals, reason code, next retry, asset manifest when sealed, and renewable authorization metadata. |
| `POST /api/v1/download-plans/{plan}/lease` | Renew retention for an active device without changing asset identity. Authorization expiry and asset retention are separate fields. |
| `POST /api/v1/download-plans/{plan}/pause` / `resume` | Change durable intent; safe to repeat. |
| `DELETE /api/v1/download-plans/{plan}` | Cancel future work and release the device's lease; local deletion remains a separate client action. |
| `GET /api/v1/download-assets/sha256/{digest}` | Serve only the named immutable bytes, with `HEAD`, single byte ranges, strong `ETag`, and RFC 9530 `Repr-Digest`. |

The sealed manifest contains at least:

- logical item ID and immutable item/content revision;
- rendition recipe/version and selected audio/subtitle/artwork entries;
- exact filename, media type, length, and whole-file SHA-256;
- fixed chunk size plus ordered `{offset, length, sha256}` records;
- strong ETag and immutable asset URL;
- authorization expiry, renewable asset-retention deadline, and minimum supported client protocol version; and
- preparation provenance needed to avoid cache collisions: source digest, streams, container/codecs, resolution/bitrate/HDR policy, and encoder build/options.

Start with fixed **8 MiB verification chunks** and transfer them in larger contiguous **64–256 MiB extents**. Those numbers are benchmark hypotheses, not standards: the fixed chunks cap corruption/retry cost, while larger extents avoid giving iOS thousands of background tasks. Apple explicitly recommends one small set of background sessions and fewer, larger transfers because every task has overhead and repeated background relaunches are rate-limited. [Apple, “Downloading files in the background”](https://developer.apple.com/documentation/foundation/downloading-files-in-the-background)

The client keeps a transactional bitmap or row per verified chunk, plus the manifest digest and destination generation. It may request an extent containing several chunks, but it marks each chunk durable only after hashing the written bytes. Reissuing an extent or chunk is therefore idempotent.

Venti's content-addressed archival design is strong evidence that hash-addressed immutable blocks make writes idempotent and independently verifiable; Kinosail should use SHA-256 rather than Venti's historical SHA-1 and keep a flat manifest because it has one trusted source. A Merkle tree is unnecessary unless manifests become too large, proofs must be independently fetched, or multiple sources are introduced. [Quinlan and Dorward, “Venti”](https://www.usenix.org/conference/fast-02/venti-new-approach-archival-data-storage)

### Revision and lease rules

- A logical library item and a byte representation are separate identities. Replacing a source produces a new content revision and new immutable asset, even if title and library identity remain unchanged.
- A sealed asset never changes. Its URL, length, ETag, whole digest, chunk list, and bytes remain identical for its lifetime.
- Authorization credentials can be short-lived and refreshed without changing the plan or asset. A `401` pauses for reauthentication; it does not discard verified chunks.
- Retention is renewed while a plan is nonterminal or recently active. It is not “seven days from first request.” Quota pressure may refuse a new plan or evict an unleased old asset; a request for an evicted revision returns `410`, never new bytes.
- Source deletion or replacement after sealing does not affect an active leased asset. Once all leases and grace periods end, the revision store may delete it.
- A hard link is not an immutable snapshot because another process can modify the linked inode in place. Prefer an atomic copy-on-write clone when the backing filesystem and container mount prove it safe; otherwise copy, hash, re-stat the source, and atomically publish the completed snapshot. A failed or changing source remains `preparing`/`needs_attention`; no partial file becomes transferable.

## Performance strategy

### Fix stalls before chasing headline throughput

The largest reliability gains come from eliminating repeated work and hidden preparation:

1. Seal and hash the representation once.
2. Resume only the missing suffix for official Android or only missing/corrupt chunks for Download v1.
3. Persist progress before process exit.
4. Distinguish server preparation, network transfer, verification, and installation timings.
5. Cache only finalized popular renditions; never stream a growing transcode as if it were immutable.

Cache a rendition by the complete source digest and recipe, publish it only after close + probe + hash + atomic rename, and reuse it across authorized plans. Research comparing cached video versions/layers found that mixed/adaptive strategies outperform blindly retaining every possible version; that supports popularity-, size-, and recomputation-aware retention rather than precomputing every permutation. [Hartanto et al., “Caching video objects: layers vs versions?”](https://doi.org/10.1007/s11042-006-0037-z)

Keep file serving and preparation in separate resource pools. Ready originals must not wait behind encoders. Within the preparation pool, use short-remaining-job preference inside age/fairness bands so episodes and remuxes finish quickly without starving a large movie. SRPT research supports reduced mean response time for static-file workloads, while its fairness analysis is why Kinosail must add aging rather than implement pure SRPT. [Harchol-Balter et al., “SRPT Scheduling for Web Servers”](https://www.cs.cmu.edu/~harchol/Papers/jss.pdf)

### Adaptive, bounded concurrency

Default to one extent request. After a measurement window, allow 2, then at most 4 concurrent non-overlapping extents only while aggregate verified goodput materially improves and error rate, server queueing, device thermal/power state, and interactive playback remain healthy. Reduce concurrency immediately on timeouts, repeated retransmission, `429`/`503`, constrained cellular, low power, or negligible gain. Persist a conservative endpoint + network-class estimate, but relearn because mobile paths change.

Parallel-transfer studies show why the number is adaptive: a few streams can approach capacity, while additional streams give marginal gain, can reduce throughput after congestion, and can take an unfair share. They do not establish one universal best count for mobile networks. [Altman et al., “Parallel TCP Sockets”](https://www.microsoft.com/en-us/research/publication/parallel-tcp-sockets-simple-model-throughput-and-validation/), [Yildirim, Balman, and Kosar, “Dynamically Tuning Level of Parallelism”](https://citeseerx.ist.psu.edu/document?doi=47637d3e91d46831db0023ba6adb7d915106ba22&repid=rep1&type=pdf)

Concurrency is also a recovery tool, not merely a speed claim: one slow or corrupt extent need not hold every other independently verifiable extent. Reuse one HTTP/2 or HTTP/3 connection when the platform permits; do not open a new TCP connection per chunk.

### HTTP/2, HTTP/3, and priorities

HTTP/3 provides multiplexed, independently flow-controlled streams and QUIC can migrate a connection across address changes such as Wi-Fi to cellular. Those capabilities can reduce cross-stream blocking and soften some network transitions, but they do not replace the durable chunk ledger. [RFC 9114](https://www.rfc-editor.org/rfc/rfc9114.html), [RFC 9000 §9](https://www.rfc-editor.org/rfc/rfc9000.html#section-9)

Google's original QUIC deployment paper reported mobile video improvements, but it studied pre-standard QUIC at Google scale and streaming rather than Kinosail downloads. Treat HTTP/3 as a measured optional transport after the HTTP/1.1/2 correctness path passes, with automatic fallback. [Langley et al., “The QUIC Transport Protocol”](https://research.google/pubs/the-quic-transport-protocol-design-and-internet-scale-deployment/)

Use RFC 9218 priority only as a hint: foreground “download now” work can have greater urgency than overnight trip preparation, and interactive playback always outranks background downloads. Endpoints cannot assume the peer honored a priority signal. The specification reserves urgency 7 for background work and warns that concurrent non-incremental responses divide completion progress. [RFC 9218 §§4, 10](https://www.rfc-editor.org/rfc/rfc9218.html#section-4)

Do not make a container silently change host congestion control. BBR and PCC Vivace show that modern sender algorithms can address utilization/bufferbloat tradeoffs, but they are host-level deployment choices with fairness and platform consequences. Offer a diagnostic benchmark and owner guidance only after controlled testing. [Cardwell et al., “BBR”](https://research.google/pubs/bbr-congestion-based-congestion-control-2/), [Dong et al., “PCC Vivace”](https://www.usenix.org/conference/nsdi18/presentation/dong)

## Integrity and resume protocol

### HTTP compatibility contract

For every immutable representation of length `N`:

- `HEAD` and `GET` return the same strong `ETag`, length, type, disposition, and whole-representation digest.
- `Range: bytes=K-` with `0 <= K < N` returns `206`, `Content-Range: bytes K-(N-1)/N`, `Content-Length: N-K`, and exactly that suffix.
- `Range: bytes=N-` returns `416`, `Content-Range: bytes */N`, and a zero-byte body for Android safety.
- `If-Range` with the matching strong ETag permits the range; a mismatch follows RFC 9110 for generic clients. Download v1 treats an unexpected `200` to a chunk/range request as a revision mismatch and replans instead of overwriting staged bytes.
- `Cache-Control: private, no-transform` prevents shared-cache reuse and tells intermediaries not to transform the representation. [RFC 9111 §§5.2.2.6–7](https://www.rfc-editor.org/rfc/rfc9111.html#section-5.2.2.6)
- `Repr-Digest` contains the base64 encoding of the raw whole-representation SHA-256 bytes. A `Content-Digest` may describe the transferred partial content, but it is not a substitute for the manifest's chunk hashes. [RFC 9530 §§2–3](https://www.rfc-editor.org/rfc/rfc9530.html#section-2)

### Download v1 client rules

1. Persist plan ID, immutable asset identity, manifest digest, target generation, and chunk ledger before starting I/O.
2. Preallocate or otherwise verify enough device space for the expected bundle plus working headroom.
3. Write only to a staging generation. Never expose it to the offline library as complete.
4. After every extent, SHA-256 each covered chunk and transactionally mark only matching chunks verified.
5. On mismatch, discard that chunk's staged bytes, record `integrity_retry`, and fetch only that chunk. Repeated mismatches stop automatic retries and surface a server/storage integrity error.
6. When every chunk is verified, compute the whole-file SHA-256. A mismatch rebuilds the ledger from per-chunk verification; it never blindly accepts the file.
7. Probe the staged local media and required tracks through a data source that rejects network access.
8. Atomically rename/install media, artwork, subtitles, and metadata as one bundle generation, then mark `ready_offline`.

Fixed chunks are the right first implementation for the same immutable compressed media file. FastCDC and rsync-style content-defined matching can reuse shifted data across revisions, but compressed/remuxed media may share little byte-level content and first downloads have nothing to deduplicate. Benchmark actual Kinosail media before accepting that CPU and manifest complexity. [FastCDC, USENIX ATC 2016](https://www.usenix.org/conference/atc16/technical-sessions/presentation/xia), [Tridgell and Mackerras, rsync technical report](https://rsync.samba.org/tech_report/)

## Mobile operating-system execution

### iOS and iPadOS

Use one stable background `URLSessionConfiguration` identifier and recreate it at launch. Apple transfers background download tasks in a separate process, can continue after app suspension or system termination, and reconnects tasks when the app recreates the same session. User force-quit cancels background transfers and prevents automatic relaunch; the app must state this limit and recover from the chunk ledger on the next manual launch. [Apple background session documentation](https://developer.apple.com/documentation/foundation/urlsessionconfiguration/background%28withidentifier%3A%29)

Persist task identifiers and any `resumeData`. Apple says a download is resumable only when the resource is unchanged, the request is HTTP(S) GET, the server supplies `ETag` or `Last-Modified`, byte ranges are supported, and the temporary file has survived storage pressure. Download v1's immutable URL, strong ETag, range support, and independent ledger satisfy the server side and provide a fallback when resume data is absent. [Apple `cancel(byProducingResumeData:)`](https://developer.apple.com/documentation/foundation/urlsessiondownloadtask/cancel%28byproducingresumedata%3A%29), [Apple, “Pausing and resuming downloads”](https://developer.apple.com/documentation/foundation/pausing-and-resuming-downloads)

Use a small number of larger background extent tasks, schedule the full ready set together, and let the system choose timing when the user selected an overnight/discretionary policy. Move completed temporary files before returning from the delegate. Never make the app's foreground process the sole owner of progress.

Apple also provides `AVAssetDownloadURLSession` and `AVAssetCache.isPlayableOffline` for offline HLS assets and selected media tracks. This is worth a later prototype for multi-track packages, but not the Download v1 dependency: unmodified Jellyfin iOS removes HLS download profiles, and a single progressive bundle keeps the protocol and integrity path common across platforms. [Apple `AVAssetDownloadURLSession`](https://developer.apple.com/documentation/avfoundation/avassetdownloadurlsession), [Apple `AVAssetCache`](https://developer.apple.com/documentation/avfoundation/avassetcache)

### Android

For user-tapped, long downloads on Android 14+, use a user-initiated data-transfer job: Android documents it for long, user-started file transfers, requires a visible notification, starts it immediately, and can allow extended runtime subject to system health. Use WorkManager's foreground-worker path below API 34, and for deferred/short interruptible maintenance. [Android UIDT guidance](https://developer.android.com/develop/background-work/background-tasks/uidt), [Android data-transfer decision guide](https://developer.android.com/develop/background-work/background-tasks/data-transfer-options)

Persist the plan/ledger in a local database independently from the job object. Job stop, app process death, reboot, or notification cancellation changes execution state, not transfer truth. Restore from the ledger, reauthenticate/renew the lease, and request only unverified chunks.

Android Media3 already supplies persistent download state, network/charging requirements, pause/resume controls, and background `DownloadService` support for progressive and segmented media. Reuse its player/cache integration where it can honor Kinosail's immutable asset and verification contract; otherwise keep the Download v1 ledger authoritative and use Media3 only after atomic installation. [Android Media3 download guide](https://developer.android.com/media/media3/exoplayer/downloading-media), [Media3 offline package](https://developer.android.com/reference/androidx/media3/exoplayer/offline/package-summary)

### Shared mobile policy

- Default to Wi-Fi, allow explicit cellular use, and distinguish expensive/constrained/roaming paths.
- Honor pause immediately at a chunk boundary; preserve verified chunks.
- Surface OS/user cancellation rather than quietly restarting against the user's intent.
- Keep a foreground-visible progress notification when the platform requires it.
- Re-estimate remaining time from verified goodput, not raw socket bytes or transcoder progress.
- Apply low-power/thermal constraints to concurrency and preparation, but do not discard progress.

## Rendition and track bundle

Offer three stable intents:

| Intent | Behavior |
| --- | --- |
| Original | Exact immutable source bytes when the device can play the container/codecs and the desired tracks are embedded or bundleable. |
| Compatible | Lossless remux first; transcode only streams the target device cannot decode or when subtitle burn-in is explicitly required. |
| Space saver | Explicit fixed output profile with a size estimate and no chunk-by-chunk quality changes. |

Probe source media at library scan and keep actual container, codecs, profile/level, dimensions, bit depth/HDR, channels, languages, disposition, and subtitle type. Plan against the requesting device profile and desired tracks. Never claim direct play based only on the filename extension. Jellyfin's codec documentation shows that container, video/audio codec, HDR, and subtitle delivery all affect whether media can direct play, remux, or transcode. [Jellyfin codec support](https://jellyfin.org/docs/general/clients/codec-support/)

A bundle generation contains one immutable media file plus any artwork, sidecar subtitle, and local metadata entries required by the Kinosail player. The manifest hashes every entry and installation is atomic at the bundle level. Prefer embedded selected tracks when that preserves compatibility; otherwise the future Kinosail player must resolve local sidecars without any remote URL. For unmodified official Jellyfin apps, embedded tracks remain the only dependable cross-client choice because the reviewed Android and iOS flows do not atomically install a server-defined external-subtitle bundle. [Android offline playback source](https://github.com/jellyfin/jellyfin-android/blob/13abdff1cf2c157d8354b02f8e1f7d35e923260e/app/src/main/java/org/jellyfin/mobile/player/queue/QueueManager.kt#L101-L135), [iOS transfer loop](https://github.com/jellyfin/jellyfin-ios/blob/78bc20a0a63aec2b0f96d7ed2eaea4a2f9f9d40f/features/downloads/hooks/useDownloadHandler.ts#L104-L215)

Transcode/remux into a temporary generation, then close, probe, verify expected tracks/duration, hash, and atomically publish. Never expose a growing encoder output to range clients. A repeated recipe must resolve to identical finalized bytes within the same encoder recipe version. On cache miss, say `preparing`; prewarm likely trip content during idle owner-approved windows rather than hide first-byte latency.

Streaming ABR research supports using throughput history, future chunk sizes, and buffer/state estimates rather than one fixed rate guess, but offline quality must remain stable for the entire asset. Use those ideas only to estimate time-to-ready and suggest one explicit rendition; do not vary offline quality by chunk. [Mao et al., “Pensieve”](https://web.mit.edu/pensieve/content/pensieve-sigcomm17.pdf), [Yin et al., “Robust MPC”](https://www.cs.cmu.edu/~xia/resources/Documents/Yin_sigcomm15.pdf)

## Security and privacy

- Require TLS for remote transfers. HTTP/3 also includes TLS 1.3 protection, but transport security does not replace application authorization. [RFC 9114 §3](https://www.rfc-editor.org/rfc/rfc9114.html#section-3)
- Authorize every plan, lease, manifest, and range request against the viewer, server, device, item, and download permission. Asset knowledge is never authorization.
- Use authorization headers for Download v1. Avoid durable account tokens and capabilities in URLs because URLs leak into logs, history, and diagnostics.
- When unmodified iOS requires a self-authorizing progressive `TranscodingUrl`, scope it to one viewer/device/asset, make it refreshable but short-lived, redact it from every log, and keep asset retention independent from token expiry.
- Revocation stops new server reads and plan renewal immediately. It cannot claw back bytes already installed on a viewer-controlled device; the product must state that boundary.
- Keep all content, chunks, rendition caches, plans, and activity on the owner-hosted Server.
- Store server snapshots/renditions under private permissions and device downloads in platform-protected app storage. Do not expose host source paths in client metadata.
- Logs and metrics use random plan correlation IDs and coarse error/size buckets. Never record media titles, source paths, raw digests where they become cross-user correlators, credentials, authorization headers, or self-authorizing URLs.

`Cache-Control: private` limits shared-cache storage but is not a security boundary; RFC 9111 explicitly notes that cache directives cannot guarantee privacy. Authorization and encrypted transport remain mandatory. [RFC 9111 §5.2.2.7](https://www.rfc-editor.org/rfc/rfc9111.html#section-5.2.2.7)

## Failure handling

| Failure | Required behavior |
| --- | --- |
| Connection timeout/reset | Persist current verified boundary; retry only missing work with bounded backoff and jitter. |
| Wi-Fi to cellular/IP change | QUIC migration may preserve the connection; otherwise reopen against the same asset and ledger. Respect cellular policy before continuing. |
| App suspension/process death/device reboot | OS task may continue; on launch reconcile OS tasks, staging files, and ledger before scheduling missing chunks. |
| Server/container restart | Reload plans, leases, sealed assets, and preparation journal. Resume immutable reads; restart only unpublished preparation work. |
| Auth expiry | Enter `waiting_auth`, refresh credentials/lease, resume the same asset. Do not delete chunks. |
| Permission revoked | Stop new reads with `403`; retain or garbage-collect server data by policy. Explain that already installed device bytes remain. |
| Source replaced/deleted | Existing leased asset remains unchanged. A new plan gets a new revision. If the old asset was already evicted, return `410`, never replacement bytes. |
| Range/validator mismatch | Official generic clients follow HTTP semantics; Download v1 rejects unexpected identity/length/status and replans without merging. |
| Chunk hash mismatch | Re-fetch only that chunk. After a small retry budget, stop and report integrity failure with sanitized diagnostics. |
| Whole-file hash mismatch | Rebuild verification from chunk hashes and repair mismatches; never mark ready. |
| Device out of space | Pause before destructive cleanup, preserve verified chunks where possible, show exact additional space needed, and let the user choose deletion/location/quality. |
| Snapshot/transcode failure | Keep the asset unpublished, return a stable preparation error, and preserve source/original availability. |
| Server overload | Return `429` or `503` with `Retry-After`; the client lowers concurrency and waits. RFC 6585 allows `Retry-After` with `429`, and RFC 9110 defines it for `503`. [RFC 6585 §4](https://www.rfc-editor.org/rfc/rfc6585.html#section-4), [RFC 9110 §10.2.3](https://www.rfc-editor.org/rfc/rfc9110.html#section-10.2.3) |
| Client force-quit/cancel | Honor the explicit stop; keep verified state and lease for the visible grace period. Resume only after user intent or configured policy. |

Apply retries in one layer, cap consecutive automatic attempts, honor server `Retry-After`, and use exponential backoff with jitter so many queued items do not synchronize into a retry storm. AWS's first-party reliability guidance specifically warns that retries consume server resources, can compound across layers, and should be bounded with jitter. [AWS retry guidance](https://docs.aws.amazon.com/wellarchitected/latest/framework/rel_mitigate_interaction_failure_limit_retries.html)

## Observability and acceptance targets

Measure product outcomes, not just HTTP request success:

- plan-to-sealed time, broken down into snapshot/remux/transcode/probe/hash;
- queue-to-first-byte and time to first verified chunk;
- verified goodput, raw goodput, stall time, retry count, and re-downloaded-byte ratio;
- range status counts, validator mismatches, `410`, `416`, `429`, and `503` by low-cardinality reason;
- chunk-integrity repair count and whole-file verification failures;
- client process/OS resumes and server-restart resumes;
- rendition cache hit, preparation wait/run time, CPU, disk read/write, and eviction reason;
- transfer-to-install time, local probe failures, and `ready_offline` completion rate; and
- collection/trip-pack completion before the user-selected deadline.

Initial release gates should be expressed as testable targets:

- zero mixed-revision or falsely-ready files in the complete fault matrix;
- after interruption, retransmit no already-verified Download v1 chunk and lose at most one in-flight verification chunk per active worker when the platform retains the staging file; separately record any larger loss caused by the OS deleting background-task temporary data under storage pressure;
- after client or server restart, restore the same plan and verified-byte count;
- a no-byte-progress watchdog moves a transfer to a named waiting/retry state within its configured stall window; no item can remain “downloading” indefinitely without byte or preparation progress;
- the deterministic 10 GiB fixture completes byte-identically after app kill, container restart, and three router drops in one run;
- every non-progressing item exposes one stable reason and next action;
- `ready_offline` is impossible until every required bundle asset hashes correctly and playback plus selected tracks pass with all networking denied;
- a sealed prepared asset begins delivery no materially slower than the same server's direct immutable-file baseline, with the numeric first-byte gate fixed per reference hardware before implementation sign-off; and
- adaptive concurrency must match or beat the single-stream verified-goodput baseline in its release test and must not reduce active-playback acceptance metrics; otherwise that network class stays at one worker; and
- on each reference network, Download v1 reaches at least 85% of a same-path single-file baseline unless server preparation, storage, or explicit policy is the measured bottleneck.

The 85% number is a Kinosail engineering gate, not a literature claim. Record the baseline tool, device, route, transport, file, thermals, and server load so results are comparable.

## Staged implementation plan

### Stage 0 — Correct the current HTTP work

- Encode RFC 9530 digests as base64 raw digest bytes.
- Remove the mutable-source fallback from release configurations or explicitly report downloads unavailable until an immutable snapshot can be made.
- Separate logical item identity, content revision, and viewer/device binding.
- Replace fixed first-bind expiry with persisted active/idle lease semantics and safe `410` on eviction.
- Preserve exact Android `206`/empty-body `416` behavior and add source-replacement + expiry tests.

### Stage 1 — Finish the official Jellyfin compatibility lane

- Persist profile + device + item-revision mappings across token changes and container restarts.
- Make source replacement create a new revision without changing bytes behind an old mapping.
- Add probe-backed metadata and truthful iOS `PlaybackInfo` decisions.
- Publish only finalized progressive cached renditions for iOS; keep Android `/Download` as the original.
- Run the released Android/iOS device matrix and document the hard client limits accurately.

### Stage 2 — Download v1 server protocol

- Add plan, lease, immutable asset, manifest, pause/resume/cancel, and state-event application operations through `/api/v1`.
- Add embedded durable journals, cache quotas, preparation admission, atomic publication, and garbage collection inside the Server container.
- Add whole + per-chunk hashes, idempotency keys, exact error codes, and sanitized metrics.
- Ship protocol/fault tests before building a second implementation against unstable seams.

### Stage 3 — One native-client vertical slice

- Implement one movie, Original quality, one track set, fixed chunks/extents, background OS execution, ledger recovery, verification, atomic install, local probe, and `ready_offline`.
- Prove app kill, device restart, server restart, Wi-Fi loss, auth refresh, and source replacement before adding seasons or transcoding.
- Start on the platform whose background execution can be tested on available physical devices; keep the protocol platform-neutral.

### Stage 4 — Trip packs and rendition bundles

- Add season/playlist planning, exact aggregate storage, per-item prioritization, deadline estimation, and trip readiness.
- Add Compatible and Space saver, selected audio/subtitle/artwork bundles, rendition reuse, preparation scheduling, and quota-aware cache eviction.
- Add “download next N unwatched” as a durable policy only after manual packs are reliable.

### Stage 5 — Measured performance options

- Tune chunk/extent sizes and adaptive 1–4 concurrency on real LAN, remote broadband, cellular, lossy, high-RTT, and bandwidth-variable paths.
- A/B HTTP/2 versus HTTP/3 with fallback and measure device power/thermal effects.
- Prototype iOS AVAssetDownload/Android Media3 segmented packages only if multi-track UX materially beats the common progressive bundle.
- Consider content-defined chunk reuse, multipath, or host congestion-control guidance only after data shows the simpler path is insufficient.

## Test matrix

### Protocol and property tests

- Fresh `HEAD`, full `GET`, `Range: 0-`, middle/open/suffix ranges, exact-end `416`, beyond-end, malformed, multiple-range rejection/handling, zero-byte asset, and integer-overflow inputs.
- Strong/weak/missing/mismatched `If-Range`, ETag stability, exact `Content-Length`/`Content-Range`, and 200/206/416 bodies.
- RFC 9530 parser round-trip proving the base64 bytes equal the asset SHA-256; reject the current hex-shaped form.
- Same logical item with two revisions; same revision across token refresh, process restart, server restart, and device identity.
- Plan idempotency, state-transition legality, lease renewal/expiry, multiple devices, quota refusal, eviction, and safe `410`.
- Manifest canonicalization, chunk coverage with no gaps/overlaps, final partial chunk, per-chunk and whole hash, corrupted manifest, wrong length, and recipe-cache collision resistance.
- Authorization matrix: owner/viewer, download disabled, library hidden, device mismatch, revoked/expired token, guessed digest/plan ID, and log redaction.

### Server fault tests

- Kill at every snapshot/transcode publication boundary: before temp create, mid-copy, after hash, before/after rename, before/after journal commit.
- Mutate/replace/delete the library source before, during, and after sealing.
- Kill/restart the container during plan creation, preparation, active range reads, lease renewal, and garbage collection.
- Inject short reads, I/O errors, disk full, read-only cache, corrupt cache bytes, clock shifts, and concurrent eviction/read.
- Saturate playback + original download + transcode preparation; verify playback/file serving admission and preparation fairness.
- Verify two devices share a safe immutable rendition but never share authorization or private plan state.

### Client fault tests

- Kill and relaunch the app at every state; reboot the device; force-quit; cancel from the OS task UI; revoke notification permission where relevant.
- Drop the network repeatedly, blackhole without closing sockets, change Wi-Fi access point/IP, move Wi-Fi ↔ cellular, toggle airplane mode, and use captive portal/DNS/TLS failures.
- Expire auth mid-extent, revoke download permission, stop the server overnight, and let the content lease approach expiry.
- Corrupt one complete chunk, one in-flight chunk, the whole-file staging metadata, and the local database independently.
- Exhaust storage before planning, mid-transfer, during verify, and during atomic install; remove external Android storage.
- Suspend/background under low power, thermal pressure, metered/constrained network, and OS job throttling.

### Media and UX tests

- Small audio, photo, 100 MiB episode, ordinary movie, deterministic 10 GiB fixture, and a season larger than available storage.
- MP4 H.264/AAC, MKV, HEVC/HDR, AV1 where supported, incompatible video/audio, multichannel audio, multiple languages, embedded text/image subtitles, external subtitles, and subtitle burn-in.
- Original/remux/transcode/space-saver recipes, hardware/software encoder changes, encoder crash, and repeated cache hit identity.
- Accurate preparation/transfer/verification/install progress, pause/resume, reason strings, retry timing, storage/time estimate, reprioritization, cancellation, and accessibility.
- With all network interfaces denied, open every `ready_offline` item, seek beginning/middle/end, select every promised audio/subtitle track, relaunch the player, and update local progress.

### Performance and longevity tests

- LAN, 10/50/200 Mbps constrained WAN, 50–300 ms RTT, 0–5% injected loss, variable bandwidth, bufferbloat, and low-end server disk/CPU.
- Compare 1/2/3/4 extents, HTTP/1.1/2/3 where available, 4/8/16 MiB chunks, and 64/128/256 MiB extents using verified goodput and power—not raw request throughput.
- Queue 100 items, run for 72 hours with randomized faults, restart both ends, and verify no leaked temp files, orphan leases, duplicate jobs, high-cardinality metrics, or cache-accounting drift.
- Repeat against the exact released official Jellyfin Android/iOS versions for the compatibility lane and supported Kinosail client/OS versions for Download v1.

## Tradeoffs and rejected ideas

- **Blind high parallelism:** rejected. It can help some high-bandwidth-delay paths, but too many flows can reduce throughput and be unfair. Use measured adaptive 1–4 extent concurrency.
- **A new TCP connection per chunk:** rejected. Reuse HTTP/2/3 connections; chunking is for verification/recovery, not congestion-control evasion.
- **ETag-only correctness:** rejected. Android does not send `If-Range`, and a validator does not make a URL immutable.
- **Seven-day first-bind TTL:** rejected. Long queues and trips cross arbitrary dates. Renew active leases and fail safely after actual eviction.
- **Live-growing transcode downloads:** rejected. Length, validators, ranges, and bytes are not stable. Finalize then publish.
- **Whole-file hash only:** rejected for the native protocol. It detects corruption only after all bytes arrive and cannot identify the repair range.
- **Merkle tree in v1:** deferred. A flat chunk list is simpler for one trusted origin; add a tree only for manifest scaling/proofs/multiple sources.
- **Content-defined chunking in v1:** deferred. FastCDC is excellent for shifted-data deduplication, but fixed chunks map directly to HTTP ranges and compressed media revisions may offer little reuse.
- **Always-on erasure/FEC:** rejected. Reliable HTTP transports already retransmit; QUIC-FEC experiments found redundancy can hurt long or low-loss transfers, and Google's QUIC deployment removed XOR FEC after it increased video latency/rebuffering under bandwidth pressure. Retry one verified chunk instead. [Michel et al., “QUIC-FEC”](https://arxiv.org/abs/1904.11326), [Langley et al., QUIC deployment](https://research.google/pubs/the-quic-transport-protocol-design-and-internet-scale-deployment/)
- **P2P or multiple media sources:** rejected. It expands privacy, authorization, NAT, availability, and integrity scope and conflicts with the direct owner-server model.
- **WebSocket/WebTransport transfer protocol:** rejected. Standard authenticated HTTP GET/range/background APIs already cover the required operation and integrate better with mobile OS transfer services.
- **HTTP compression for media:** rejected by default. The source/rendition is already compressed media; extra server CPU and unstable encoded bytes undermine direct resumability without a demonstrated size win.
- **Mandatory HLS/DASH offline packages:** deferred. Platform stacks support them and they may improve selected-track integration, but they create a second package/install path and do not help unmodified Jellyfin iOS downloads. Prove the progressive immutable bundle first.
- **Automatic Wi-Fi + cellular multipath:** rejected as a default. MPTCP can survive path failure, but smartphone research found it can be slower than the best single path and use more energy. Make any future aggregation explicit, metered-data-aware, and measured. [Raiciu et al., MPTCP deployment](https://discovery.ucl.ac.uk/id/eprint/1389795), [Saha et al., smartphone MPTCP](https://www.cse.buffalo.edu/faculty/dimitrio/publications/mobiwac17.pdf)
- **Container-controlled BBR/PCC:** rejected. Host networking is owner/platform policy; Kinosail can diagnose and document, not silently mutate it.

## Final product boundary

With unmodified official Jellyfin apps, Kinosail can provide immutable server bytes, exact resume/range behavior, correct metadata/authentication, long-lived revision safety, and finalized cached iOS renditions. It cannot make iOS persist resume state, make either app verify chunks or probe local playback, change Android's serial queue, atomically install a custom track bundle, or expose a truthful trip-ready state.

The best possible experience therefore requires a Kinosail client speaking Download v1. That client is justified only after the compatibility lane is solid, but the server should adopt revision identity, plan/lease semantics, correct digests, and immutable finalized assets now so none of that work is thrown away.
