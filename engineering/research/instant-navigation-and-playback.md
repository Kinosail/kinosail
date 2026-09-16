# Instant navigation and playback

- **Research date:** 2026-08-26
- **Scope:** Kinosail's Go + HTMX web application, direct media, and compatibility HLS.
- **Primary outcome:** minimize time from a deliberate Play action to the second visibly presented video frame without sacrificing direct-first playback, privacy, correctness, or server responsiveness.

## Executive decision

The shortest reliable Kinosail playback path is:

> one user intent -> one shared playback decision -> one player surface -> direct byte-range delivery -> first two composited frames

The current movie path is already close: a movie card links directly to `/watch/{id}`. A show card first opens `/show/{id}`, so playing the next episode normally takes a second click and a second document navigation. More importantly, a cold `/watch/{id}` request can synchronously launch `ffprobe`, and every video player page downloads/parses HLS.js even when direct playback succeeds. Those are higher-value targets than micro-optimizing templates.

The priority order is:

1. Measure the actual critical path through a second composited video frame, including tail latency and cold state.
2. Remove request-path probing and unrelated detail work from player startup.
3. Give shows a direct Play-next action and, if testing justifies the larger change, activate a persistent player surface inside the original click handler.
4. Make direct files progressively startable and prove byte-range behavior.
5. Load HLS machinery only when the playback plan or a direct-play failure needs it.
6. Make navigation feel instant with shell-preserving swaps, bounded intent prefetching, correctly prioritized images, immutable static assets, and back/forward cache compatibility.
7. Optimize cold HLS around the first independently decodable segment, without replacing direct-first playback or exhausting a small self-hosted server.

These changes should be evaluated as a latency distribution, not a demo. Google's production latency paper explains why rare slow paths dominate user experience at scale; Kinosail should apply the p95/p99 lesson even though the paper's distributed-service architecture is not directly comparable to a home server ([Dean and Barroso, *The Tail at Scale*](https://research.google/pubs/the-tail-at-scale/)).

## What the repository does today

This is a source inspection, not a fresh runtime benchmark.

| Path or behavior | Current implementation | Startup implication |
|---|---|---|
| Movie card | Links directly to `/watch/{id}` | One navigation and one explicit intent |
| Show card | Links to `/show/{id}`; the show page offers Play next | Two clicks/document navigations for the common “resume or play next” intent |
| Watch handler | `buildPlayerData` calls the media probe | A cold video can launch `ffprobe` with a request-scoped timeout of up to 10 seconds |
| Probe cache | Process-local map keyed by item ID | Restart or cache miss returns probing to the request path; source changes need stronger cache identity |
| Direct video | `<video autoplay playsinline preload="metadata">` with `/media/{id}` | Direct-first is preserved, but new-document autoplay is policy-dependent and metadata-only is merely a hint |
| Compatibility playback | HLS.js is configured with `startLevel: -1`, `testBandwidth: true`, and `startFragPrefetch: true` | Sensible generic defaults; bandwidth testing can add a lowest-rendition fragment before selected playback |
| Player scripts | HLS.js is included on every video player page | Roughly 618 KB of source is transferred/parsed even when direct playback wins; compressed transfer will be smaller but parse/compile still exists |
| HLS publish readiness | Master becomes visible after every advertised variant has init data and a first segment | A truthful, stable ladder, but cold startup waits for the slowest advertised first segment |
| HLS segmentation | fMP4 event playlists, four-second segment target, forced keyframes every two seconds for multi-rendition presentations and every four seconds for individual transcode variants | Reasonable current point; shorter is not automatically faster because it increases request, mux, and encoder overhead |
| Direct file delivery | `http.ServeFile` | Go supplies useful HTTP semantics, but browser and API tests should prove the exact range, conditional, cancellation, and seek behavior Kinosail depends on |
| Static assets | One-day public cache lifetime; HTML has version query parameters in several places | Repeat navigation still permits revalidation after a short lifetime; a content-addressed immutable policy can remove it safely |
| Primary navigation | Mostly full-document links; HTMX is used for search, title filtering, and incremental content | Document startup and shell reparsing are repeated between library surfaces |
| Artwork | Several early movie posters use high fetch priority; much artwork remains lazy and original-size delivery is possible | Blanket priority loses its meaning; oversized art competes with HTML, CSS, script, and media |
| Startup regression test | Measures until the media `playing` event | `playing` is useful but does not prove that the user saw a new frame |

Relevant implementation seams include `internal/server/player.go`, `internal/server/player_playback.go`, `internal/server/playback_plan.go`, `internal/server/probe.go`, `internal/server/hls.go`, `internal/server/files.go`, `internal/server/assets.go`, `internal/server/static/player.js`, `internal/server/static/pwa.js`, and `e2e/playback-startup.spec.ts`.

## Define “instant” with the correct clock

### Measurement endpoints

Measure all of these independently:

1. **Intent acknowledgment:** `pointerdown`/keyboard activation to visible pressed/loading state.
2. **Navigation response:** activation to response start and first useful player markup.
3. **Media request:** player source assignment to first media request and first media byte.
4. **Ready to decode:** init metadata and the first independently decodable sample available.
5. **First presented frame:** first `requestVideoFrameCallback` callback.
6. **First moving picture:** a subsequent callback whose `mediaTime` or `presentedFrames` advanced.
7. **Stable playback:** moving picture followed by a defined interval with no rebuffer.

The HTML media event names are not equivalent to visible playback. `loadeddata` means the user agent can render the current frame, while `canplay` and `playing` describe media readiness/playback state ([HTML media elements](https://html.spec.whatwg.org/multipage/media.html)). WebKit's implementation discussion describes `requestVideoFrameCallback` as firing when a video frame is sent to the compositor, and notes that registering it before load is the most reliable way to observe the earliest available frame ([WebKit bug 211945](https://bugs.webkit.org/show_bug.cgi?id=211945), [WebKit bug 236604](https://bugs.webkit.org/show_bug.cgi?id=236604)).

Kinosail should therefore keep event telemetry for diagnosis but make “second distinct composited frame” its primary click-to-first-moving-frame measure. This also prevents a poster frame, a seek that updates `currentTime`, or a premature `playing` event from being counted as success.

### Instrumentation plan

- Capture `performance.now()` in the activation handler, before any navigation or asynchronous work.
- Use the Navigation Timing API for request, response, and document milestones ([Navigation Timing Level 2](https://www.w3.org/TR/navigation-timing-2/)).
- Add a same-origin `Server-Timing` breakdown for authenticated diagnostics: authorization, lookup, media-facts lookup, playback decision, template, and response flush ([Server Timing](https://www.w3.org/TR/server-timing/)). Do not include file paths, titles, user identifiers, tokens, host internals, or high-cardinality values.
- Record media request timing and video frame callbacks in the player.
- Record direct/HLS mode, cold/warm state, browser engine, coarse network class, fallback reason, first-frame time, second-moving-frame time, and rebuffer-before-stable-playback.
- Report p50, p75, p95, p99, maximum, and sample count. Segment by path instead of averaging direct and transcoded playback together.
- Keep measurements local by default. Detailed media capabilities and timing can add fingerprinting or privacy surface; the Media Capabilities specification explicitly calls out fingerprinting considerations ([Media Capabilities](https://www.w3.org/TR/media-capabilities/)).

The Event Timing specification can help correlate the initiating interaction with main-thread processing for same-document playback ([Event Timing](https://www.w3.org/TR/event-timing/)). It is supporting evidence, not a replacement for the media-frame clock.

### Initial engineering budgets

These are proposed product budgets to validate against current hardware, not browser standards or claims from the academic papers:

| Scenario | p75 target | p95 target |
|---|---:|---:|
| Intent acknowledgment | < 50 ms | < 100 ms |
| Warm LAN library navigation to useful content | < 200 ms | < 400 ms |
| Warm LAN direct play to moving picture | < 750 ms | < 1.5 s |
| Cold LAN compatibility play to moving picture | < 2 s | < 4 s |
| Seek to moving picture, direct | < 500 ms | < 1 s |

The classic RAIL guidance treats feedback within 100 ms as effectively immediate, which is appropriate for the visible acknowledgment budget, not as a blanket media-start requirement ([RAIL model](https://web.dev/articles/rail)). A large observational study of 23 million online video views found that abandonment increased after roughly two seconds of startup delay and that every additional second increased abandonment; it also found substantial engagement loss from rebuffering ([Krishnan and Sitaraman](https://people.cs.umass.edu/~ramesh/Site/IN_THE_News_files/imc208-krishnan.pdf), [DOI](https://doi.org/10.1109/TNET.2013.2281542)). That population and public-Internet delivery are old and different from Kinosail, so the result is evidence for prioritizing startup and continuity, not a direct Kinosail SLO.

## The click-to-play critical path

```text
explicit Play intent
    -> shared cached playback decision
    -> player shell becomes visible and play() is requested
    -> direct media request OR HLS manifest request
    -> init metadata + independently decodable sample
    -> decode
    -> first compositor callback
    -> second advancing compositor callback
```

Anything that does not contribute to those steps should happen after playback begins: cast and crew, neighboring episodes, collection membership, full description, recommendations, nonessential artwork, activity enrichment, and background analysis.

### P0: move probing and planning off the request path

Persist normalized media facts during scan/import and key them by a source version such as stable media identity plus size and modification time. Derive the playback plan through the existing shared operation. The watch page and versioned API should read the same result.

On a missing or stale record:

- coalesce concurrent requests for the same source;
- run one bounded, cancelable analysis job;
- keep it at lower priority than active browsing and playback;
- return a truthful pending or conservative plan instead of launching duplicate external processes;
- invalidate only the changed source; and
- capture the probe duration and miss reason.

The objective is a player response whose critical work is authentication, item lookup, a database/cache read, the shared playback decision, and minimal markup—not an external process.

### P0: make Play distinct from Open details

Movies already support a one-intent path. Show cards should expose a permission-appropriate Play-next/Resume action that goes directly to the resolved episode, while the poster/title continues to open show details. The target episode must come from the same application operation exposed through the versioned API; do not duplicate “next episode” rules in template code.

The low-risk first step is a direct `/watch/{episode}` link and an immediate loading state. The highest-speed design to prototype is a persistent player surface in the current document:

1. Cards receive a compact, authorized playback result derived from persisted media facts.
2. The Play handler synchronously assigns the direct source to an already available video element and calls `play()`.
3. The application updates history and retrieves noncritical player metadata after the media request has started.
4. HLS is loaded only when the known plan or direct failure requires it.

This matters because audible autoplay policies generally require playback to be directly attributable to a user gesture. WebKit documents that `play()` for audible video must directly result from a click or similar handler and that code must handle the returned promise being rejected ([WebKit autoplay policy](https://webkit.org/blog/7734/auto-play-policy-changes-for-macos/), [WebKit iOS video policies](https://webkit.org/blog/6784/new-video-policies-for-ios/)). A click followed by a new document's `autoplay` is not as reliable as calling `play()` in the original handler. The persistent-player design is a hypothesis that needs a focused cross-browser prototype because it changes navigation and lifecycle behavior.

Security constraints:

- Never put a bearer token, share secret, or filesystem path in card markup.
- Use only already-authorized, short-lived or session-bound URLs consistent with the current application model.
- A pointer hover must not start playback, create an activity record, or start a transcode.
- Only the explicit Play action should cross the playback side-effect boundary.

### P0: return the player before the details

The first response bytes should contain the player element, its selected source, its MIME type, and the minimum controls/status surface. Load secondary information through the versioned API after the media request begins. If streaming the HTML response is considered, verify actual parser and network timing; a flush is not automatically useful if required CSS or blocking scripts still precede the player.

Do not report the player as ready merely because the shell rendered. Continue the same measurement through moving picture.

## Direct media startup

### MP4 initialization placement

An ISO BMFF initialization segment contains `ftyp` followed by `moov`; the decoder needs that initialization information before media segments can be decoded ([W3C ISO BMFF byte-stream format](https://www.w3.org/TR/mse-byte-stream-format-isobmff/)). FFmpeg's `faststart` flag performs a second pass to move MP4 index metadata to the beginning of the file specifically for better playback; metadata is otherwise commonly written at the end ([FFmpeg formats documentation](https://ffmpeg.org/ffmpeg-formats.html)).

At scan time, classify compatible MP4/MOV sources as start-optimized or tail-indexed. Do not silently rewrite user media. Options, in descending order of preference:

1. Direct-serve already start-optimized media.
2. Verify the browser's initial and tail byte-range pattern is fast enough on the target storage.
3. For selected high-probability titles, build a Kinosail-owned cached stream-copy remux with front-loaded metadata during idle time.
4. Offer an explicit library optimization operation if a user wants broader precomputation.

A cached fast-start remux consumes storage and FFmpeg I/O and can compete with playback. It must be bounded, source-versioned, atomic, cancelable, and evictable. It is not useful for every file and should be driven by measured misses or predicted next/resume content.

### Byte-range contract

HTTP byte ranges allow a client to request part of a selected representation and receive `206 Partial Content` with `Content-Range`; a client may send a range even when the server did not advertise `Accept-Ranges` ([RFC 9110 section 14](https://www.rfc-editor.org/rfc/rfc9110.html#section-14)). Kinosail should make this an explicit regression contract rather than assume `http.ServeFile` behavior covers every surrounding handler and proxy condition.

Test at least:

- full response and content type;
- `bytes=0-N` and suffix ranges;
- valid `206`, `Content-Range`, `Content-Length`, and body boundaries;
- invalid and unsatisfiable ranges;
- conditional requests;
- cancellation on navigation/seek;
- first play and representative forward/back seeks in Chromium, Firefox, and WebKit; and
- behavior through the supported HTTPS/reverse-proxy topology.

### Preload and autoplay

The HTML `preload` attribute is a hint; `autoplay` can cause the browser to start fetching regardless of the hint ([HTML media elements](https://html.spec.whatwg.org/multipage/media.html)). Therefore changing `metadata` to `auto` is an experiment, not a guaranteed fix. Measure bytes, media-request start, first frame, and wasted transfer when a user immediately backs out.

For an explicit Play action, invoke `play()` and handle rejection by leaving a large visible Play control. Preserve `playsinline`. Never convert the privacy-preserving direct path into mandatory transcoding simply to make autoplay behavior uniform.

### Capability decisions

Use persisted server-side container/stream facts plus the browser's real outcome. `canPlayType()` is only a hint. Where useful, `navigator.mediaCapabilities.decodingInfo()` can report `supported`, `smooth`, and `powerEfficient` for an exact file or Media Source configuration ([Media Capabilities](https://www.w3.org/TR/media-capabilities/)).

Persist only a coarse, local capability result with an expiry and browser-version key. Do not build or transmit a detailed hardware fingerprint. A failed direct attempt should fall back once, with an explicit reason, and later avoid repeating the exact known-bad combination until its capability key changes.

## Compatibility HLS startup

### Keep what is already correct

Kinosail's HLS.js configuration already combines `startLevel: -1`, bandwidth testing, and `startFragPrefetch`. HLS.js documents that the initial bandwidth test uses the lowest level when `startLevel` is `-1`, and that `startFragPrefetch` begins loading the first fragment before media attachment when possible ([HLS.js 1.7.1 API](https://github.com/video-dev/hls.js/blob/v1.7.1/docs/API.md)). Preserve these defaults for unknown remote links unless measurement proves a better segmentation.

Do not load the full HLS.js payload on a direct-success path. Use a small loader that imports it when:

- the server plan is compatibility HLS;
- the browser lacks a usable native HLS path; or
- Automatic falls back from direct playback.

When the plan already requires HLS, start loading the HLS code and manifest immediately; lazy loading should not serialize two known prerequisites.

### Remove avoidable first-fragment work

For a high-confidence LAN class, test whether a viewport-appropriate initial level without the lowest-level bandwidth-test fragment improves time to moving picture and total startup bytes. Keep bandwidth testing for remote, unknown, or constrained links. The network class must be an observed, coarse property—not inferred from a private IP alone.

Test these independently:

- current `startLevel: -1` plus bandwidth test;
- fixed conservative first level;
- a cached recent estimate with a safe maximum age;
- native HLS where supported; and
- direct-first followed by HLS fallback.

Reject any version that improves the median by creating more startup stalls or quality oscillation at p95.

### First independently decodable segment

Apple's HLS authoring specification requires video segments to begin with an IDR frame, recommends a nominal six-second target duration, requires aligned boundaries across variants, and requires `EXT-X-MAP` for fMP4 playlists ([Apple HLS Authoring Specification](https://developer.apple.com/documentation/http-live-streaming/hls-authoring-specification-for-apple-devices/)). Kinosail's current four-second segment target, two-second forced-keyframe cadence for multi-rendition presentations, and four-second cadence for individual transcode variants should be benchmarked, not changed by folklore.

Evaluate 2 s, 4 s, and 6 s targets on the smallest supported server and representative codecs. Measure:

- time until init plus the first usable segment is published;
- first and second composited frames;
- encoder CPU and memory;
- segment/request overhead;
- quality at the fixed bitrate;
- seek latency;
- recovery after a rendition switch; and
- interactive request latency during transcode.

Shorter segments can publish sooner but create more requests, mux work, and keyframe overhead. A shorter GOP can improve random access while reducing compression efficiency. Treat segment duration and keyframe cadence as separate variables.

Validate outputs with Apple's HLS tooling where applicable ([HTTP Live Streaming resources](https://developer.apple.com/streaming/)) and with the project's browser matrix.

### Do not wait on a cold full ladder unless it wins

Publishing only after all advertised variants have valid init data and a first segment keeps the manifest truthful. A cold full-ladder encode can nevertheless make startup wait for work the first frame does not need.

Measure three bounded designs:

1. Current full ladder, published atomically.
2. A single conservative starter presentation for the current session.
3. Idle prewarming of the most likely resume/next title, with strict CPU, process, storage, and cancellation budgets.

Do not mutate an already-consumed master playlist into a partially incompatible ladder. Do not start a transcode on hover or speculative navigation. Prewarming is useful only when the hit rate and saved p95 latency exceed wasted compute and interference with active users.

Low-Latency HLS addresses live glass-to-glass latency; it is not a default answer for on-demand startup. Kinosail should first reduce analysis, encode-to-first-segment, and player dependencies.

## Navigation and poster/detail loading

### Preserve the shell selectively

HTMX `hx-boost` progressively enhances ordinary links into AJAX navigation and pushes history while retaining a normal-link fallback ([HTMX `hx-boost`](https://htmx.org/attributes/hx-boost/)). Apply it first to stable library/details navigation where the shared application shell dominates repeated work. Keep full-document escape behavior and verify title, focus, scroll, landmarks, errors, offline behavior, and browser history.

HTMX history snapshots can be stored in `localStorage`; the project must not persist sensitive player pages, shared-link material, or personalized history unintentionally ([HTMX `hx-history`](https://htmx.org/attributes/hx-history/), [`hx-history-elt`](https://htmx.org/attributes/hx-history-elt/)). If safe snapshotting cannot be guaranteed, use native navigation plus back/forward cache rather than broad HTMX history.

### Preserve back/forward cache eligibility

The back/forward cache can restore a prior document immediately without reloading it. Use `pageshow` and `pagehide`, avoid `unload`, and measure `event.persisted` plus `PerformanceNavigationTiming.notRestoredReasons` where available ([web.dev bfcache guidance](https://web.dev/articles/bfcache), [Chrome `notRestoredReasons`](https://developer.chrome.com/docs/web-platform/bfcache-notrestoredreasons)).

Kinosail already uses `pagehide` rather than `unload` in the player. Confirm whether destroying the source on `pagehide` causes a restored player to reload or lose its place, and distinguish navigation away from actual page destruction. A restored player must never resume audible playback unexpectedly.

### Use intent prefetching, never speculative playback

The HTMX preload extension can preload safe GET content on `mousedown`, gaining the normal 100–200 ms between press and release; its mouseover mode waits 100 ms by default and warns about server/bandwidth cost ([HTMX preload extension](https://htmx.org/extensions/preload/)). This is a useful progressive experiment for show detail fragments and a minimal player shell only if prefetched requests are side-effect-free.

Browser Speculation Rules are still not a universal baseline. Chrome supports eagerness levels including conservative pointer/touch activation and moderate hover behavior; it also warns about the resource cost of over-speculation ([Chrome prerender guidance](https://developer.chrome.com/docs/web-platform/prerender-pages)). The underlying proposal documents security and privacy concerns ([WICG navigation speculation](https://wicg.github.io/nav-speculation/prefetch.html)). WebKit's same-origin prefetch work is newer and should be verified against the actual supported release ([WebKit bug 295193](https://bugs.webkit.org/show_bug.cgi?id=295193)).

Rules for Kinosail:

- use same-origin document/fragment prefetch only;
- cap it at one highly likely target;
- prefer pointer-down over broad hover grids;
- respect reduced-data/browser policy;
- make prefetched GETs idempotent and side-effect-free;
- identify prefetch requests server-side and skip activity, progress, probe, and transcode work;
- cancel or deprioritize when active playback begins; and
- never prerender `/watch`, fetch media, or launch HLS from prediction alone.

### Send the right image bytes

The likely largest-content image must not be lazy-loaded, and only one or two high-priority images should normally be marked high because prioritizing everything makes the signal ineffective ([Optimize LCP](https://web.dev/articles/optimize-lcp)). Generate Kinosail-owned poster/backdrop derivatives during scan or low-priority background work, with source-versioned identities. Emit intrinsic width/height and accurate `srcset`/`sizes`; choose a modern encoding only after testing decode support and server-generation cost ([Image performance](https://web.dev/learn/performance/image-performance)).

Do not perform large resizes in the interactive request. Do not add a hosted image service or remote asset dependency. On a Play action, cancel/deprioritize offscreen art so media and player resources win.

### Cache validators and immutable assets

HTTP validators allow a stale cached representation to be revalidated and reused after a `304 Not Modified`; personalized responses need private cache semantics and correct variation ([RFC 9111](https://www.rfc-editor.org/rfc/rfc9111.html)). Use generation-based ETags for library/detail fragments where a correct validator is cheap. Keep user-scoped content private and do not cache shared or authenticated representations publicly.

For static CSS, JavaScript, fonts, and bundled artwork, prefer content-hashed URLs with a long `max-age` and `immutable`. RFC 8246 describes this versioned-URL pattern and recommends secure transport because long-lived poisoned content is difficult to dislodge ([RFC 8246](https://www.rfc-editor.org/rfc/rfc8246.html)). Do not mark a mutable URL immutable.

### Keep the service worker out of dynamic critical paths

A service worker is justified for offline shell behavior and content-hashed static assets, not as a new cache for personalized navigation or media. Keep navigations network-authoritative, and never cache watch HTML, direct media, byte-range responses, HLS manifests/segments, secrets, or shared-link content. A W3C performance workshop presentation specifically recommends avoiding service-worker navigation interception when the goal is only subresource caching ([W3C service-worker performance breakout](https://www.w3.org/2024/Talks/TPAC/breakouts/sw-for-performance.pdf)); this is engineering guidance, not a standard.

## Transport and server scheduling

For the supported local HTTPS topology, first verify connection reuse, TLS session resumption, HTTP/2 multiplexing, correct compression of text assets, and that media is not compressed dynamically. `preconnect` to the same already-connected origin is unlikely to help after the first document.

QUIC combines secure transport and multiplexed streams and can reduce some handshake and loss-related costs, as documented in its large deployment evaluation ([QUIC design and deployment](https://research.google/pubs/the-quic-transport-protocol-design-and-internet-scale-deployment/)). HTTP/3 should remain a remote/high-loss experiment unless measurements show transport setup or TCP head-of-line blocking in Kinosail's supported topology. It is not a first-order LAN startup fix and adds proxy, certificate, observability, and support complexity.

Every background optimization must yield to interaction:

- bounded queues and process counts;
- cancellation on source changes and explicit playback priority;
- separate admission budgets for scans, probes, derivatives, remuxes, and transcodes;
- storage I/O awareness, not only CPU accounting;
- no unbounded goroutines; and
- request-latency and race tests while background work is saturated.

## Experiments in priority order

| Priority | Experiment | Primary metric | Guardrail |
|---|---|---|---|
| P0 | Add second-moving-frame and `Server-Timing` instrumentation | End-to-end p75/p95/p99 by playback mode | No sensitive/high-cardinality telemetry |
| P0 | Persist scan-time media facts; eliminate request-path `ffprobe` | `/watch` response start and moving-frame latency, cold/warm | Correct invalidation; one coalesced fallback job |
| P0 | Add direct Play-next/Resume on show cards | Actions and time to moving picture | Details remain accessible; API and web share operation |
| P0 | Minimal player response before secondary details | First media request and first frame | Correct auth/errors; no hidden side effects |
| P0 | Prove ranges and classify MP4 metadata placement | First media byte/frame and seek time | Never rewrite user originals |
| P0 | Conditional HLS.js loading | Direct-path bytes, parse time, first frame | HLS-required path does not become serially slower |
| P1 | Persistent same-document player prototype | Autoplay success and click-to-frame across engines | Accessible focus/history; recoverable failure; no token exposure |
| P1 | HLS first-level and first-segment matrix | Cold HLS p95, stalls, CPU, quality | Preserve truthful manifest and stable playback |
| P1 | Bounded next/resume prewarm | Hit rate and latency saved minus wasted work | Zero interference with active requests/playback |
| P1 | HTMX shell-preserving browse navigation | Useful-content navigation p75/p95 | Safe history storage; bfcache; full-page fallback |
| P1 | Pointer-down detail/player-shell prefetch | Navigation/response latency saved | One target; GET-only; no probe/transcode/activity |
| P1 | Poster/backdrop derivatives and priority audit | LCP, transferred image bytes, media request contention | Correct art, aspect ratio, fallback, and cache identity |
| P1 | Content-hashed immutable assets and fragment validators | Repeat-nav requests/bytes/latency | No immutable mutable URLs; private personalized caching |
| P2 | HTTP/3 remote/loss trial | Handshake and p95 under induced loss | No added default complexity without a win |

For each experiment, run cold process/cache and warm cases, representative spinning disk and SSD/NAS storage if supported, idle and background-load cases, and Chromium/Firefox/WebKit. Include direct start-optimized MP4, tail-indexed MP4, WebM where supported, remux-only, audio-transcode, full-transcode, text subtitles, bitmap subtitles, resume, seek, direct failure, and HLS failure/recovery.

## Changes to reject unless new evidence overturns them

- Prerendering or autoplaying watch pages based on hover.
- Starting FFmpeg, recording activity, or advancing progress from speculative requests.
- Preloading full media files from a poster grid.
- Replacing direct-first playback with universal HLS.
- Transcoding all media into a normalized library by default.
- Lowering HLS segment duration without measuring encoder, request, quality, and tail-latency costs.
- Marking every above-fold poster `fetchpriority="high"`.
- Publicly caching authenticated HTML or API responses.
- Caching media ranges, HLS, player pages, or secrets in the service worker.
- Treating `playing`, `currentTime`, or a successful `play()` promise as proof of moving picture.
- Deploying HTTP/3 or Low-Latency HLS as a substitute for fixing request-path probes and first-segment work.

## Recommended delivery sequence

1. Land instrumentation and a reproducible baseline first.
2. Persist media facts and remove cold probe subprocesses from `/watch`.
3. Split Play from Details for shows; send the minimal player before secondary content.
4. Validate range delivery, classify MP4 startup layout, and conditionally load HLS.js.
5. Prototype persistent-player activation and choose it only if the cross-browser gain is material.
6. Run the HLS segment/initial-level/starter-presentation matrix on minimum hardware.
7. Improve navigation, assets, artwork, and conservative pointer-down prefetch.
8. Consider remote transport work only after the remaining timing breakdown identifies it as a material tail.

The expected largest wins are eliminating synchronous probing, preserving the original Play gesture, initiating direct bytes before decorative/detail work, and removing HLS code from successful direct playback. Those should be proven before tuning smaller parsing, CSS, or transport effects.
