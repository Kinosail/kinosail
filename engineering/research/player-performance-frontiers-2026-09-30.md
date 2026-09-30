# Kinosail performance frontiers

Date: September 30, 2026. Starting revision: `9a45e9444613dbde5a277a8c7426919fcab97aa8`.

This continues the [first responsiveness investigation](player-responsiveness-2026-09-29.md). The active goal is to pursue every measurable performance gain across native clients, web, catalog, and playback. We have not established a performance ceiling. Physical-device frame timing remains unknown.

## New findings and changes

### Artwork admission matters as much as cache size

The native decoded cache has a 32 MiB budget. Background prefetch previously competed with displayed images under the same least-recently-used eviction rule. A controlled workload shows the consequence: five speculative 1600 × 1600 images evict a displayed image. Revisiting it downloads and decodes it again.

The change gives speculative decoded images lower retention priority. Background work can replace other speculative images. Foreground requests can reclaim either class. A cache hit promotes the exact requested size. Encoded downloads remain shared across sizes, but decoded ownership includes client identity, URL, and dimension.

Background refresh needs a separate admission rule. Refreshing a previously displayed image must not evict other foreground images merely because its new source has more pixels. The fresh encoded response still belongs on disk. The next foreground request must be able to load it; keeping an obsolete decoded entry indefinitely would violate freshness.

This keeps the existing limits and foreground network reserve. It does not increase concurrency or reduce image quality. When foreground pixels occupy the budget, fewer speculative images remain decoded. Encoded disk reuse still reduces later network work.

### Collation scratch storage creates avoidable allocation traffic

The populated 10,000-title web browse profile attributes about 39% of sampled CPU and 57% of allocated bytes to title sorting and its callees. The original collator buffer grows across the complete library. Earlier keys retain older backing allocations after buffer growth.

The change resets collation scratch space for each title and copies completed keys into separate compact storage. The collation algorithm, locale, tie breaks, and page contents stay the same. A large Swedish-title regression spans multiple pages, includes equal-title ID ties, and exceeds the scratch and initial key-storage capacities.

Five samples reduce allocated bytes from approximately 4.71 MB to 2.83 MB per browse: about 40%. Allocation counts change from 11,590 to 11,573. The response remains 25,319 bytes. Wall-clock samples overlap; this is a measured memory improvement, not proof of lower interaction latency. See the [raw evidence](evidence/player-performance-frontiers-2026-09-30.md).

## Current primary-source research

These findings are research inputs. Paper results are not Kinosail results.

| Source and evidence read | Useful finding | Kinosail application and limit |
| --- | --- | --- |
| [MAPP, IWQoS 2025: predictive UI view pre-caching](https://par.nsf.gov/servlets/purl/10598216). Full paper, including evaluation and overhead. | Predicting likely next views can avoid expensive Android view inflation. The evaluation uses 61 interaction traces from 18 volunteers and two Android devices. Prediction also costs energy and memory. | Experiment with local, bounded preparation of the next likely shelf or focused title. Start with focus direction and navigation history. Do not collect location or train a model without evidence that simpler rules fail. Its Android inflation measurements do not establish SwiftUI gains. |
| [PaperCache, HotStorage 2025](https://www.eecg.utoronto.ca/~stumm/Shakiba-HotStorage25.html). Author-hosted abstract and publication details. | Different eviction policies suit different workloads. The work switches policies as observed behavior changes. | Replay local artwork access sequences against byte-aware policies before changing the current small cache. Measure visible-image decode misses and latency, not just aggregate hit rate. Runtime policy switching is not justified yet. |
| [SCION, 2026 preprint](https://arxiv.org/abs/2605.01055). Abstract and evaluation scope. | Object size, reuse, cacheability, and workload changes influence policy choice. Its policy selection occurs outside the request hot path. Evaluation is CPU-only and trace-driven. | Explore size-aware admission and a small workload fingerprint in offline experiments. Compare against simple admission first. This is a preprint; neither its simulator results nor learned selection prove device UI gains. |
| [Apple WWDC26: profile, fix, and verify responsiveness](https://developer.apple.com/videos/play/wwdc2026/268/). Transcript. | High CPU, execution contention, and blocked threads require different diagnoses. A task created from UI code can inherit the main actor. Release-build traces matter. | Inspect SwiftUI, Time Profiler, Swift Concurrency, and System Trace together. Check executor placement for decode, JSON processing, disk work, and expensive model construction. Adding `async` alone does not prove work left the UI actor. |
| [WebKit: Safari 26.2 performance APIs](https://webkit.org/blog/17640/webkit-features-for-safari-26-2/). Official release notes. | Event Timing and Largest Contentful Paint support extend Safari performance measurements. | Collect interaction and paint evidence on supported Safari versions through feature detection. Keep measurements local and exclude media names, credentials, and private URLs. Older browsers still need alternative trace evidence. |
| [Video Streaming Over QUIC, revised November 2025](https://arxiv.org/abs/2505.21769). Abstract and revision metadata. | QUIC implementation, congestion control, queueing, and bitrate adaptation interact. Identical congestion-control algorithms can behave differently across implementations. | Compare HTTP/2 and HTTP/3 under loss and competing transfers before changing the gateway. Measure first frame, stalls, throughput, CPU, and recovery. A protocol upgrade alone is not evidence of improvement. |
| [QCON, NSDI 2026](https://www.usenix.org/conference/nsdi26/presentation/lee). Conference abstract and deployment scope. | QoE-aware radio scheduling addresses unstable links using 5G multi-connectivity. Its prototype requires RAN infrastructure. | Apply the measurement lesson: correlate stalls with queueing and other transfers. Kinosail cannot deploy carrier radio scheduling through an app patch. Its cloud-gaming results do not transfer to local movie playback. |

## Experiments that can still move the result

| Order | Experiment | Acceptance evidence | Main constraint |
| --- | --- | --- | --- |
| 1 | Profile remote input through focus, view update, decode, texture upload, and presentation on an older supported Apple TV. | Release-build traces; p50/p95/p99 input-to-visible-feedback; hitch duration and count during rapid traversal, reversal, and return from playback. | Simulator correctness does not establish hardware frame timing. Use 16.67 ms and 8.33 ms as display intervals, not application CPU allowances. |
| 1 | Right-size artwork and adapt look-ahead to viewport and direction. | Pixel dimensions, decode time, foreground misses, memory, and image-quality comparisons at normal and focused size. Test accessibility text sizes. | Twenty-four 1600 × 900 RGBA images need about 132 MiB before row-padding costs, far beyond the 32 MiB decoded cache. Do not decode an entire large window merely because it fits on disk. |
| 1 | Measure interference from scan, metadata, downloads, transcodes, disk writes, and artwork. | Repeat interaction journeys idle and under load. Compare tail latency and cancellation recovery. | Background concurrency can consume disk, CPU, memory bandwidth, and network capacity that visible work needs. |
| 2 | Cache immutable title ordering per library generation and bounded locale set. | Cold and warm 10,000/100,000-title browsing; scan and decorator invalidation; concurrent reads; unchanged visibility, progress, lists, and locale ordering. | Cache immutable ordering, not personalized response pages. A scan or metadata change must publish the new ordering atomically. Show aggregation needs its own title projection. |
| 2 | Precompute normalized searchable metadata and evaluate an inverted index. | Search latency and allocated bytes across title, cast, plot, and accented text; index-build cost; mutation correctness. | A full index increases memory and refresh work. Preserve existing ranking and validation. |
| 2 | Trace native catalog refresh and observation dependencies. | Populated cached refresh with active focus/scrolling; expensive view-body causes; affected-state publication counts. | Stable IDs alone do not prevent broad view invalidation. Publish changes at the state owner rather than suppressing necessary updates. |
| 2 | Trace web render, layout, and input handlers in Chromium and Safari. | Pending, loaded, empty, and failed journeys; responsive sizes; interaction latency; forced layout; long tasks; accessibility. | `content-visibility` and scheduling APIs need browser-specific verification and fallbacks. Bounded pages already limit DOM growth. |
| 2 | Prepare the next likely title's metadata and playback decision. | Tap-to-first-frame breakdown, speculative waste, cancellation, stale decision rejection, and active-playback interference. | Keep direct-first selection and one obvious Play action. Do not start speculative transcodes for every focused card. |
| 3 | Measure first-GOP and segment packaging costs for transcodes. | Cold startup, random seek, codec/device compatibility, rebuffering, and encoder load. | Shorter segments or more frequent keyframes can increase bitrate and compute. Live latency and movie startup are different objectives. |
| 3 | Compare Brotli/static packaging, connection reuse, and HTTP/3. | Transfer bytes, decode CPU, cold/warm navigation, first frame, and lossy-network tests. | Keep personalized data private and preserve TLS and authentication. Avoid speculative transport complexity when local decode or view updates dominate. |

## Choices that require evidence first

- A framework rewrite or custom renderer needs a trace showing a framework bottleneck that smaller changes cannot remove.
- Learned prefetch needs to outperform a bounded focus-direction rule after accounting for its CPU, energy, memory, and prediction waste.
- Increasing cache sizes needs device memory-pressure evidence. More retained pixels can cause eviction elsewhere or application termination.
- Broad response caching needs complete library, profile, permission, locale, query, progress, and list invalidation. Do not trade freshness or isolation for a faster warm path.
- SIMD, parallel decode, and hardware-specific paths need full-pipeline benchmarks and a portable fallback before adoption. A faster inner loop may have little effect on playback or UI latency.

## Verification boundaries

The measurement record holds raw samples, fixture descriptions, commands, source hashes, and test results. Required hosted checks remain separate from local evidence. The native tests exercise real HTTP adapters, decoded image identity, cache budgets, stale disk data, shared downloads, and cancellation.

Local verification passed the full Player, Subtitles, and shared Go suites, the catalog race check, source-file caps, and repository tooling checks. The reconciled tvOS simulator run passed 26 tests in five artwork suites. Its growth regression covers both 1600px replacement and 4096px replacement above the retention budget. The independent review findings have regression controls and are resolved. Changed-code Go lint reports zero issues; full local shared-package lint has existing findings outside this patch.

No physical iPhone or Apple TV frame trace, deployed first-frame measurement, or production-network benchmark is established by this follow-up. Until device models are supplied, the working target is older supported Apple hardware. The goal remains active while these measurement and optimization opportunities remain unresolved.
