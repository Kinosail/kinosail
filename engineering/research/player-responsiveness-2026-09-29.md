# Kinosail responsiveness: research and measured findings

Date: September 29, 2026. Audited starting revision: `517fdf6b25047845adefd7bd9489214b82f0fc24`.

This note covers Player web, the native iOS and tvOS clients, shared catalog browsing, and playback startup. It records current research, source findings, verified changes, and work that still needs measurements. It does not establish smoothness on physical devices.

## Main findings

1. **Cold web loads transfer avoidable bytes.** Player serves embedded CSS and JavaScript without HTTP compression. Precompress public static text once, preserve immutable versioned caching, and negotiate `Accept-Encoding`. Do not extend this change to authenticated HTML, API responses, images, or media.
2. **Large-library browsing repeats inexpensive work thousands of times.** A populated 10,000-title benchmark allocates about 7.43 MB and 41,590 objects per browse. The CPU profile identifies sorting and garbage collection as material costs. ASCII letter classification can avoid Unicode normalization without changing the result. Caching complete browse results needs profile, query, library, progress, and list invalidation first.
3. **Current native clients already have important caching safeguards.** Catalog snapshots publish before refresh. Artwork uses bounded decoded and disk caches, coalesced requests, cancellation, background decode, and a foreground request reserve. These are present in current main; this task must not claim them as new fixes.
4. **The reported tvOS sluggishness still needs an on-device trace.** Source review and simulator correctness tests do not identify its frame stalls. Measure rapid focus traversal, direction reversal, populated shelves, return from playback, and refresh while interacting.

## Research that changes our approach

| Primary source | Finding | Application to Kinosail |
| --- | --- | --- |
| [Apple WWDC26: lazy stacks and scrolling](https://developer.apple.com/videos/play/wwdc2026/321/) | Stable identities, predictable child counts and sizes, and early data preparation support lazy loading and prefetch. Filtering inside lazy child construction can defeat these assumptions. | Keep lazy shelves and grids. Filter data before `ForEach`. Reserve artwork and text geometry. Check preparation before appearance, nested shelf heights, and focus identity in Instruments. |
| [Apple: understanding SwiftUI performance](https://developer.apple.com/documentation/xcode/understanding-and-improving-swiftui-performance) | SwiftUI tracing identifies expensive updates and their causes. | Inspect update groups and dependencies before changing view structure. Prefer small observable state changes; keep disk access and image decode away from the main actor. |
| [Google: optimize Interaction to Next Paint](https://web.dev/articles/optimize-inp) | Input delay, event processing, and presentation delay all contribute to responsiveness. Long tasks, forced layout, and excessive rendering can dominate. | Measure each stage. Defer nonvisual work after feedback paints. Batch DOM reads and writes. Avoid extra synchronous storage or DOM work in input handlers. |
| [Google: text encoding and transfer size](https://web.dev/articles/optimizing-content-efficiency-optimize-encoding-and-transfer) | Compress text; avoid recompressing already compressed media. Static compression avoids repeated request CPU work. | Add gzip to embedded static text. Evaluate prebuilt Brotli only if measured transfer gains justify its build and runtime costs. |
| [RFC 9110: Accept-Encoding](https://www.rfc-editor.org/rfc/rfc9110.html#section-12.5.3) | Quality values and exclusions determine acceptable encodings. `Vary` distinguishes cached representations. | Test gzip exclusions, malformed and bounded headers, representation length, HEAD behavior, and identical decoded bytes. |
| [NSDI 2025: Dissecting and Streamlining the Interactive Loop of Mobile Cloud Gaming](https://www.usenix.org/conference/nsdi25/presentation/li-yang) · [paper](https://www.usenix.org/system/files/nsdi25-li-yang.pdf) | The evaluated mobile cloud gaming systems contain multiple synchronization delays across the input-to-display loop. Network latency alone does not explain the response time. | Measure the complete remote-input, focus, view-update, artwork, and display path. This is an inference for Kinosail; the paper's cloud-gaming speedup does not transfer to movie browsing. |
| [2025: Improving UI responsiveness in Android by restructured rendering](https://research.polyu.edu.hk/en/publications/improving-ui-responsiveness-in-android-by-restructured-rendering/) · [DOI](https://doi.org/10.1016/j.sysarc.2025.103580) | The published abstract describes prioritizing visual work under UI load by separating visual and nonvisual events. | Apply the scheduling principle: show input feedback promptly and defer optional bookkeeping. Only the abstract was available; this is not evidence for replacing platform rendering. |

WWDC26 guidance is current, but the app must retain its supported deployment targets. Do not require an OS 27 API solely because it is new. [Apple's WWDC26 platform overview](https://developer.apple.com/videos/play/wwdc2026/102/) describes platform improvements; these are not measured Kinosail gains.

## Ranked follow-up work

| Priority | Surface | Action and acceptance evidence | Tradeoff |
| --- | --- | --- | --- |
| 1 | Web cold load | Compress static bundles. Compare wire bytes and identical decoded content. Check browser execution and warm cache reuse. | Small startup memory cost for compressed copies; no compression work per ordinary static request. |
| 1 | Shared browse | Remove repeated ASCII normalization. Compare allocation counts and the existing populated browse benchmark; preserve Unicode letters and locale behavior. | Keep the existing Unicode path for all other characters. |
| 1 | tvOS focus and scrolling | Record SwiftUI and Time Profiler traces on older supported Apple TV hardware with 10,000 titles. Correlate remote input with view updates and display hitches. | Simulator timing cannot substitute for hardware GPU, decode, memory, or thermal behavior. |
| 1 | Native artwork | Measure requested pixels against displayed points and scale. Reduce oversized card images only where the visual comparison passes. Keep selected and visible artwork ahead of bounded prefetch. | Smaller images can look soft during TV focus scaling. Aggressive prefetch wastes bandwidth and may delay foreground work. |
| 2 | Native refresh | Measure diff and publication costs when refreshing a populated cached library. Preserve stable IDs; publish only affected state. | Cache freshness and profile isolation remain required. Do not suppress necessary updates to improve a benchmark. |
| 2 | Server queries | Measure sorted-reference caching or indexes after classification costs are reduced. Define generation and visibility invalidation before implementation. | Global response caching can leak profile state or return stale progress and lists. |
| 2 | Web interactions | Trace cold and warm navigation, search, settings, and playback controls. Shorten long tasks, batch layout work, and defer bookkeeping. | `scheduler.yield()` needs a supported fallback; it is not universally available. See [MDN](https://developer.mozilla.org/en-US/docs/Web/API/Scheduler/yield). |
| 2 | Offscreen web content | Experiment with [content-visibility](https://web.dev/articles/content-visibility) only on measured rendering hotspots. Provide intrinsic sizes and verify keyboard focus, accessibility, search, and scrolling. | Incorrect estimates can shift layout; this is not a replacement for bounded pages. |
| 2 | Playback | Separately time tap-to-request, decision, first segment or direct response, decode readiness, and first displayed frame. Test direct play, transcode, AirPlay, and network recovery. | Preserve direct-first selection and the single obvious Play action. Speculative playback preparation must be cancellable and bounded. |
| 3 | Disk and network contention | Trace background scan, metadata, downloads, artwork, and transcodes during interaction. Reserve foreground capacity where evidence shows contention. | More concurrency can increase memory, disk queueing, and tail latency. |

## Current cache and loading architecture

- Native catalog storage is bounded to 64 MiB and 512 entries, scoped to server and Viewer Profile. Cached snapshots are available before automatic refresh. Current freshness logic uses 60 seconds.
- Native artwork has a 32 MiB decoded cache with 96 entries and a 256 MiB disk cache with 2,048 images. Request coalescing shares work. Cancelling the last consumer cancels the producer.
- The artwork loader permits four active requests and reserves capacity by limiting background requests to two. ImageIO decode runs outside the UI actor. A generation guard protects against obsolete work after cache clearing.
- Web browse pages are bounded to 100 items by default, with a maximum of 200. Static assets with a version token already use one-year immutable caching.
- Playback already prepares source information and preferences concurrently and retains short-lived preparation results. Wider prewarming requires evidence that it improves startup without competing with the active view.

Source seams: `apps/player/apps/native/Sources/ArtworkLoader.swift`, `LocalMediaCache.swift`, `CatalogAPI.swift`, `PlaybackCoordinator.swift`; `packages/catalog/letters.go`; `apps/player/internal/server/assets.go` and `performance_benchmark_test.go`. Verify source paths and constants when extending this work.

## Measurements and verification

Environment: macOS, Apple M1 Pro, arm64. Other builds ran on this host, so wall-clock benchmark timings are noisy. Allocation counts and response sizes are more stable.

| Check | Observed result |
| --- | --- |
| Baseline 10,000-title web browse, five 2-second samples | 17.62–33.09 ms/op; about 7,428,800 B/op; 41,590–41,591 allocations/op; 25,319 response bytes. |
| Baseline CPU profile | Sorting consumes a material share of cumulative CPU; garbage collection is also prominent. This does not prove sorting is the physical-device UI bottleneck. |
| Native artwork correctness on tvOS 27 Simulator | 17 tests passed across existing loader tests and cancellation tests. Includes shared-request survival when one consumer cancels and cancellation when the last consumer leaves. |
| Static compression regression | Failed against the starting implementation, then passed with compression. Checks gzip negotiation, exclusions, bounds, cache headers, MIME types, and identical decoded bytes. |
| Player Go suite | `go test ./...` passed before the subsequent catalog optimization. |
| Container-backed populated browser gate | Could not build: Podman storage reported no space left on device. No container storage was pruned. |

Final transfer measurements, post-change browse measurements, browser evidence, and delivery checks will be added before this task is delivered.

Repeat the browse measurement from `apps/player`:

```sh
go test ./internal/server -run '^$' -bench '^BenchmarkLargeLibraryBrowse$' -benchtime=2s -count=5
```

Repeat compression and letter regressions from their Go modules:

```sh
# apps/player
go test ./internal/server -run 'TestStatic(Bundle|Compression)' -count=1 -v
# packages
go test ./catalog -run TestBrowseLetterJumpsPreserveLocaleAndUnicode -count=1
```

For device profiling, record revision, model, OS, display refresh, library size, cache state, network conditions, and background work. Report p50/p95/p99 latency and hitches, not only averages. A 60 Hz display has 16.67 ms per frame; 120 Hz has 8.33 ms. These are frame intervals, not guarantees that all of that time is available to application work. Google's [good INP threshold is 200 ms](https://web.dev/articles/optimize-inp); use it as an interaction target, not evidence that Kinosail meets it.

Remaining boundaries: physical iPhone and Apple TV frame traces, full browser matrix, actual playback first-frame timings, deployed container performance, and production network measurements. CI, publication, deployed revision, and device smoothness must be reported separately.
