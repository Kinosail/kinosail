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

A separate known-title navigation benchmark still allocates about 1.88 MB and 28,962–28,964 objects for an exact-title search across 10,000 titles. Concurrent builds heavily affected wall time. Its raw samples are preserved as a follow-up baseline, without a before/after latency claim.

### Reuse the owned catalog snapshot

The search profile also identifies two candidate arrays. Browsing already projects visibility, progress, and list state into detached request-owned storage. Filtering then allocates another array with the same capacity.

The application path now compacts its owned snapshot after releasing profile locks. The public `Browse.Apply` operation still copies caller-owned input before selection. This removes one array without adding shared mutable storage, cached pages, or invalidation rules. A focused regression detects input mutation by an unsafe implementation.

Three-sample handler benchmarks show approximately 400 KB fewer allocated bytes at 10,000 titles. Web browse changes from 2.83 MB to 2.43 MB, and web exact-title search from 1.88 MB to 1.48 MB. Native JSON browse changes from about 2.17 MB to 1.77 MB, and search from 1.38 MB to 0.97 MB. At 100,000 titles, the native API saves about 4 MB per request. Response byte counts stay unchanged.

The shared host was running other builds, and wall-clock samples vary substantially. These results prove allocation reductions, not interaction or frame latency. The [raw evidence](evidence/player-performance-frontiers-2026-09-30.md) preserves all samples, including slower ones. The [official Go GC guide](https://go.dev/doc/gc-guide) explains the connection between heap allocation and collection work; reduced pauses remain an inference requiring runtime measurements here.

### Retain completed catalog title order

Repeated native JSON browse across 100,000 titles now reuses immutable locale ordering. Forward medians change from 67.630 to 18.017 ms; reverse medians change from 57.503 to 16.321 ms. Both comparisons show about 72–73% less handler time. Allocated bytes drop by about 9.62 MB per request. Repeated 10,000-title web browse improves by 50–53%; letter navigation improves by 61–63%.

The cache retains at most four orders and 1,048,576 item references. Each request still reads current visibility, progress, and list state. Successful publication invalidates orders; late requests cannot admit old versions. Admission reuses the completed result's private array. This removes an 800 KB cold-admission copy found during the experiment. Search shows no repeatable improvement.

A request must select every indexed item before paging to seed an order. Sparse and restricted requests retain their existing cold sorting work. Mixed native movie/show catalogs need a complete all-title request first. Native added-order home pages do not seed this cache. The mixed-library phase below now admits complete Owner intrinsic-view partitions within the same budget. The [measurement record](evidence/catalog-title-order-2026-09-30.md) contains both control directions, cold cases, source hashes, and the discarded-copy evidence. These are handler measurements; physical UI frames remain unknown.


### Reuse intrinsic title order in mixed libraries

Movies and Music are subsets of a mixed catalog. Repeated native requests therefore missed complete-all admission. The cache now reuses complete Owner order for movies, music, books, audiobooks, and photos by locale. Every request still applies current visibility and profile state. Generation, completeness, unique IDs, four entries, and the existing reference budget remain guarded.

Interleaved 100,000-item Owner medians improve from 76.379 to 36.949 ms for Movies and 55.112 to 26.727 ms for Music. Allocated bytes fall about 42% and 23%, respectively. Guest requests benefit after an Owner seeds the order. Guest-only timing remains uncertain, and cold Owner admission adds a completeness count. These are synthetic handler results, with no physical smoothness claim.

A filtered-miss draft made Guest workloads slower and was rejected. A separate reference-lifetime proposal also showed no dependable gain. The [mixed-library record](evidence/catalog-mixed-title-order-2026-09-30.md) preserves both rejected experiments, all samples, source bindings, and verification limits. The overall goal remains active.
### Read only the profile state a catalog view needs

Production-length IDs expose additional projection costs after title-order reuse. A 100,000-item Owner browse still allocates about 15.45 MB and 200,579 objects per request. Its snapshot reads list and playback state even for static views.

Static views now project owned item references without those state lookups. List reads membership; unwatched reads watched state. History keeps timestamp ordering and skips unused list lookup. Every request still evaluates current visibility. Metadata search and sorting run after releasing profile locks. Public callers retain ownership of their input arrays.

The matched controls cover realistic ID lengths, populated state, Owner fallback, and restricted profiles. They show large, consistent allocation reductions. Wall-time results vary, including slower initial history and unwatched samples. The [projection record](evidence/catalog-profile-projection-2026-09-30.md) retains all controls and their limits. These server gains do not establish physical UI frame timing.

### Remove temporary strings from each metadata scan

Search previously joined title, plot, fields, and credits before normalizing the result. The shared catalog now appends normalized fields directly into local scratch storage. Short ASCII records avoid heap storage. Longer records reserve one buffer, with normal growth for Unicode expansion. This adds no retained search cache or invalidation rule.

Five matched samples reduce median handler time from 3.482 to 2.886 ms for 10,000-title web search. Native API search changes from 2.728 to 2.163 ms at 10,000 titles and 26.679 to 21.460 ms at 100,000 titles. Those reductions are about 17%, 21%, and 20%. The simple native API case removes 20,000 temporary allocations per 10,000-item request. At 100,000 titles, it saves about 4.8 MB per request.

Three-sample metadata workloads also improve. Short ASCII metadata changes from 10.651 to 7.067 ms. Long plots change from 62.915 to 40.861 ms. A long plot ending in an accented word changes from 273.487 to 138.863 ms, about 49% lower. Response byte counts stay unchanged. Original-source reverse controls remain close to the baseline.

A streaming normalization iterator reduced allocations but slowed the Unicode workload. It was rejected. A first field-based prototype also changed compatibility-character matching. Regression tests caught that before delivery. The final path preserves lowercase-before-decomposition behavior, including capitals produced by NFKD. ASCII characters within Unicode fields avoid expensive Unicode table checks.

These results are server handler measurements on a shared Mac. They do not establish device animation smoothness or production tail latency. The [measurement record](evidence/player-performance-frontiers-2026-09-30.md) contains all matched samples, source hashes, commands, rejected controls, and limits.

### Stop obsolete catalog work

Both servers now pass the request context into shared catalog browsing. Validation still runs before index or profile access. Cancelled work returns an error without a partial page. Projection, metadata matching, ranking, and collation preparation check cancellation every 64 items.

In a 10,000-title workload with long accented plots, an already-cancelled request previously completed a successful search in 138.675 ms. It now returns the existing unavailable response in 0.002328 ms. A request with a 10 ms deadline changes from 138.061 to 12.254 ms. That case reduces allocation traffic from about 124.027 MB to 10.519 MB, approximately 92%.

A 256-item polling control takes 13.679 ms in the deadline case. More frequent checks improve recovery without adding retained storage. Successful-request allocation levels remain similar. Their median times are close to the original-source reverse control; this shared-host experiment does not prove zero overhead.

[Go's request-context documentation](https://pkg.go.dev/net/http#Request.Context) describes cancellation when the client disconnects or cancels an HTTP/2 request. [Context propagation guidance](https://pkg.go.dev/context) supports carrying that signal through the application call chain. Reclaiming obsolete work should reduce competition with current requests, but smoother frames under load remain an inference.

Index loading, mutex acquisition, grouping, copies, and comparison sorting already underway remain synchronous. The change checks safe boundaries rather than abandoning goroutines or disrupting sort comparisons. The [measurement record](evidence/player-performance-frontiers-2026-09-30.md#catalog-request-cancellation) preserves every sample, controls, source hashes, and limits.

### Stop after a confirmed rich-metadata search match

Rich records now check short ASCII titles before allocating storage for other metadata. A bounded field-prefix check also admits literal matches that normalization preserves. Every other case keeps the complete matching path, including phrases spanning fields and credits.

For 10,000 movies with long accented plots, broad title search changes from 181.102 to 43.247 ms. A common plot phrase changes from 181.118 to 44.728 ms. Both reduce allocated bytes by about 99%. The real HTTP adapter returns the same totals and response sizes.

A first prototype grew temporary title storage on misses. Long ASCII titles added about 2.3 MB per 1,000-item search. Unicode expansion added about 6.9 MB. The selected guard retains baseline allocation levels for both controls.

The exact-title median changes from 138.950 to 138.941 ms; the original-source reverse control takes 137.964 ms. The shortcut adds bounded checks and shows no benefit on this path. Absent searches remain similar. Ordinary 10,000/100,000-title requests retain similar allocation levels and response sizes. These server measurements do not establish native frame timing.

The public regression covers 32 rich-metadata cases. An unsafe control fails compatibility-character matching. The [measurement record](evidence/player-performance-frontiers-2026-09-30.md#confirmed-rich-metadata-matches) preserves final samples, an original-source reverse control, rejected allocation growth, and source bindings. Exact searches and misses remain useful targets for further profiling.

### Release navigation now has a repeatable workload

A dedicated tvOS 27 simulator ran the optimized app against a loopback fixture with 160 synthetic movies. The journey reverses across the Home shelf, opens Movies, traverses several grid rows, and returns to Home. Five runs passed, including assertions that Movies regains focus. Screenshots confirm populated artwork after traversal. Test durations include automation and are not input-latency measurements.

The measured source revision is `eda10c42461d9985e868b0d03536e3ecc6c57c1f`. Later native changes need new measurements; merging this record does not validate their performance.

An app-only Time Profiler recording produced 13,281 running-thread samples with a 1 ms weight. The main thread accounts for 9,000 samples. App symbols identify thumbnail decoding and library-response construction on other threads. These observations support the existing executor placement; they do not establish a hardware frame-time improvement.

The trace also reports three potential microhangs, lasting approximately 284–434 ms. Test activation, accessibility queries, navigation, and screenshots overlap the recording. Most main-thread leaf symbols remain unresolved, even after matching the app's debug symbols. We cannot attribute these intervals confidently to a specific application update or to real-world remote latency.

The SwiftUI template fails because its hitch instrument is unsupported on this simulator. Adding SwiftUI without that instrument saves a CPU trace, but produces no SwiftUI update data, both when attached and when launched. The combined all-process recording also fails during symbol processing. These are recorded tool limitations, not successful frame measurements.

The next native measurement needs usable update causes and presentation timing. [Apple's performance documentation](https://developer.apple.com/documentation/xcode/understanding-and-improving-swiftui-performance) recommends investigating both expensive view bodies and excessive update frequency. A physical-device trace remains necessary before claiming smoother focus animation or choosing a framework rewrite. The [measurement record](evidence/player-performance-frontiers-2026-09-30.md) preserves configuration, hashes, commands, and failure boundaries.

### Avoid placeholder work on fast web requests

Chromium traces now exercise the real embedded web app with 10,000 synthetic movies and generated artwork. A warm desktop reload measured 237 ms LCP and zero layout shift. Traversing to 1,600 cards under 4× CPU throttling measured 52 ms observed interaction latency. These are local lab observations, not field or physical-device scores.

The mobile title scrub revealed a smaller opportunity. Starting each request immediately hid artwork and animated placeholders across the current library. Fast responses paid that rendering cost before replacing the page. The shared web handler now waits 120 ms before showing visual placeholders. Busy state and inert content still begin immediately. Final completion cancels the timer; detached targets cannot acquire stale placeholders.

Six comparable mobile scrubs measured a median observed interaction latency of 84 ms before the change and 46.5 ms with the compiled fix: approximately 45% lower. A browser-only prototype measured 43.5 ms. Restoring immediate placeholders returned the median to 84 ms. All measured layout shifts were zero. The [measurement record](evidence/player-performance-frontiers-2026-09-30.md) retains individual samples and limitations.

This follows the presentation-delay diagnosis in the [current INP optimization guide](https://web.dev/articles/optimize-inp). Delaying unnecessary placeholder work avoids a transient rendering pass. It does not postpone the request or change its response. The 120 ms threshold is an application choice validated here, not a threshold established by that guide.

One initial mobile scrub measured 285 ms, but subsequent controls did not reproduce it. It remains in the evidence and is excluded from the matched six-scrub comparison. The traces also flag oversized artwork. That is a candidate for responsive image derivatives, not a measured byte saving or a reason to weaken authenticated cache rules.

### Artwork dimensions have a measurable decode cost

A standalone optimized macOS ImageIO benchmark uses the production thumbnail method and one synthetic 3200 × 3200 JPEG. Six alternating batches measure each requested dimension. Median decode times are 3.954 ms at 400px, 5.010 ms at 800px, and 16.150 ms at 1600px. Decoded storage is 0.64 MB, 2.56 MB, and 10.24 MB respectively.

The 400px result uses 75% fewer decoded bytes and about 21% less decode time than 800px. This confirms the cost of unnecessary pixels in this fixture. It does not establish a native UI or network improvement. The source is a synthetic pattern, so it cannot establish photographic quality at focused TV size.

The 48-point MiniPlayer cover currently requests 800px. A native-loader follow-up compares cold, saved, and warm paths before changing that setting. Shelf dimensions, display scale, focus enlargement, and accessibility sizes still need separate checks.

### A smaller image can lose a decoded cache hit

An optimized tvOS simulator experiment runs the production artwork loader through ten controlled workloads. Cold 400px loads take about 10% less time than 800px loads through the URLProtocol transport fixture. Saved-image loads take about 11% less time. The smaller decoded image occupies 640,000 bytes instead of 2,560,000 bytes.

Warm reuse changes the result. An 800px request with its exact decoded image cached takes 0.025 ms and returns the same object. Requesting 400px instead takes 4.985 ms and retains an additional image. Combined held pixels increase from 2.56 MB to 3.20 MB. A matching 400px cache hit is also fast, at 0.028 ms.

Non-landscape media cards use 800px at standard text sizes. Landscape and accessibility variants use 1600px. The album grid, album detail, and full audio player use 1600px. A fixed MiniPlayer reduction could help a 1600px-only path while losing an 800px card hit. The experiment does not measure how often each path occurs in real navigation. The MiniPlayer setting remains unchanged.

[Apple’s latest SwiftUI session](https://developer.apple.com/videos/play/wwdc2026/269/) adds HTTP caching and configurable image sessions. Kinosail also needs decoded-size reuse, profile isolation, protected storage, freshness, and cancellation. Those contracts need verification before replacing its loader. [Apple’s performance lab](https://developer.apple.com/videos/play/wwdc2026/8003/) supports appropriate image sizes and narrower view updates; that guidance does not establish hardware gains here.

The [native artwork record](evidence/native-artwork-loader-2026-09-30.md) contains all 1,500 timed samples, source hashes, a complete replay fixture, startup contamination controls, and exact verification limits. ImageIO logged pixel-buffer errors during the untimed 1600px seed loads; images returned and checks passed. The cause remains unknown. These loader measurements exclude SwiftUI rendering, GPU upload, image quality, physical frames, and real network latency. An adaptive policy remains a candidate for navigation-trace and quality measurements.

## Current primary-source research

These findings are research inputs. Paper results are not Kinosail results.

| Source and evidence read | Useful finding | Kinosail application and limit |
| --- | --- | --- |
| [MAPP, IWQoS 2025: predictive UI view pre-caching](https://par.nsf.gov/servlets/purl/10598216). Full paper, including evaluation and overhead. | Predicting likely next views can avoid expensive Android view inflation. The evaluation uses 61 interaction traces from 18 volunteers and two Android devices. Prediction also costs energy and memory. | Experiment with local, bounded preparation of the next likely shelf or focused title. Start with focus direction and navigation history. Do not collect location or train a model without evidence that simpler rules fail. Its Android inflation measurements do not establish SwiftUI gains. |
| [PaperCache, HotStorage 2025](https://www.eecg.utoronto.ca/~stumm/Papers/Shakiba-HotStorage25.pdf). Full paper, including methods, evaluation, and limitations. | The cache reconstructs policy state from access history while sampled metadata handles temporary eviction. Evaluation uses large storage traces. Eight policies add 10.8–70% metadata memory overhead versus Redis at 100,000–1,000,000 objects. Asynchronous eviction can temporarily exceed the configured budget. | Replay local artwork sequences before adopting policy switching. Compare visible decode misses, bytes, latency, and policy CPU cost. Kinosail retains at most 96 decoded images with a strict byte budget. The paper's larger caches, reconstruction work, and hourly switches do not establish benefits for this workload. |
| [SCION, 2026 preprint](https://arxiv.org/html/2605.01055v1). Full methods and evaluation, sections 3–6 and tables 3, 15, 18, 24–26. | Thirty traces use 128–1024 MiB caches. A 200,000-request fingerprint costs about 1.6 seconds. Policy selection trades object misses against byte misses; improvements do not cover every latency percentile. Its HTTP prototype omits revalidation, invalidation, and distributed consistency. | Replay local artwork navigation and compare simple admission first. The paper's 256 MiB HTTP prototype and much larger request history do not establish gains for Kinosail's 32 MiB, 96-entry decoded cache. Keep orchestration off the request path unless measured benefits exceed its cost. |
| [Apple WWDC26: lazy stacks and scrolling](https://developer.apple.com/videos/play/wwdc2026/321/). Full transcript, layout, subview loading, prefetching, and programmatic scrolling. | Lazy stacks prepare view bodies and layout before appearance. Conditional leaf counts and layout changes on appearance can defeat that work. Off-screen size estimates also affect navigation. | Audit card geometry and resolved child counts. Compare shelf preparation during direction reversal and return navigation. The current home uses an outer `VStack`; changing it to a lazy container is a hypothesis that requires populated focus and hitch evidence. SwiftUI's view preparation does not replace bounded, cancellable network artwork prefetch. |
| [Apple WWDC26: profile, fix, and verify responsiveness](https://developer.apple.com/videos/play/wwdc2026/268/). Transcript. | High CPU, execution contention, and blocked threads require different diagnoses. A task created from UI code can inherit the main actor. Release-build traces matter. | Inspect SwiftUI, Time Profiler, Swift Concurrency, and System Trace together. Check executor placement for decode, JSON processing, disk work, and expensive model construction. Adding `async` alone does not prove work left the UI actor. |
| [WebKit: Safari 26.2 performance APIs](https://webkit.org/blog/17640/webkit-features-for-safari-26-2/). Official release notes. | Event Timing and Largest Contentful Paint support extend Safari performance measurements. | Collect interaction and paint evidence on supported Safari versions through feature detection. Keep measurements local and exclude media names, credentials, and private URLs. Older browsers still need alternative trace evidence. |
| [Video Streaming Over QUIC, revised November 2025](https://arxiv.org/abs/2505.21769). Abstract and revision metadata. | QUIC implementation, congestion control, queueing, and bitrate adaptation interact. Identical congestion-control algorithms can behave differently across implementations. | Compare HTTP/2 and HTTP/3 under loss and competing transfers before changing the gateway. Measure first frame, stalls, throughput, CPU, and recovery. A protocol upgrade alone is not evidence of improvement. |
| [QCON, NSDI 2026](https://www.usenix.org/conference/nsdi26/presentation/lee). Conference abstract and deployment scope. | QoE-aware radio scheduling addresses unstable links using 5G multi-connectivity. Its prototype requires RAN infrastructure. | Apply the measurement lesson: correlate stalls with queueing and other transfers. Kinosail cannot deploy carrier radio scheduling through an app patch. Its cloud-gaming results do not transfer to local movie playback. |

## Experiments that can still move the result

| Order | Experiment | Acceptance evidence | Main constraint |
| --- | --- | --- | --- |
| 1 | Profile remote input through focus, view update, decode, texture upload, and presentation on an older supported Apple TV. Try the [XCTHitchMetric route](https://developer.apple.com/videos/play/wwdc2025/247/) as well as Instruments. | Release-build traces; p50/p95/p99 input-to-visible-feedback; hitch duration and count during rapid traversal, reversal, and return from playback. | Simulator correctness does not establish hardware frame timing. The XCTest simulator route executed but emitted no hitch metric; see the [probe record](evidence/tvos-hitch-metric-2026-09-30.md). Use 16.67 ms and 8.33 ms as display intervals, not application CPU allowances. |
| 1 | Measure mixed-library intrinsic-order reuse under deployed load. | Cold and warm catalog journeys, current permissions/profile state, memory retention, concurrent refresh, and network tail latency. | Synthetic handler gains are measured in the mixed-library record. Physical presentation and deployment remain separate. |
| 1 | Right-size artwork and adapt look-ahead to viewport and direction. | Pixel dimensions, decode time, foreground misses, memory, and image-quality comparisons at normal and focused size. Test accessibility text sizes. | Twenty-four 1600 × 900 RGBA images need about 132 MiB before row-padding costs, far beyond the 32 MiB decoded cache. Do not decode an entire large window merely because it fits on disk. |
| 1 | Measure interference from scan, metadata, downloads, transcodes, disk writes, and artwork. | Repeat interaction journeys idle and under load. Compare tail latency and cancellation recovery. | Background concurrency can consume disk, CPU, memory bandwidth, and network capacity that visible work needs. |
| 2 | Profile remaining visibility scans, letter generation, and state-key construction after title-order reuse. | Representative IDs, populated maps, restricted profiles, cold and warm requests, cancellation, and allocation profiles. | Preserve current permissions, progress, lists, locales, and Owner fallback. Reduced allocation traffic alone does not certify lower tail latency. |
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

The metadata-search phase passed the full shared, Player, and Subtitles Go suites and the catalog race check. Changed-code lint reports zero issues in shared packages and Player. Source caps, tooling checks, and regenerated Code Atlas snapshots also passed. Its server benchmark gains remain separate from native presentation timing. Both app `verify-changed` commands stopped at 112 existing shared lint findings; later stages did not run. Hosted checks remain the delivery authority.

The cancellation phase passed those three full Go suites and the catalog race check. Changed-code lint reports zero issues in shared packages, Player, and Subtitles. Source caps, repository tooling, regenerated snapshots, and an independent source review also passed. Both post-commit app checks passed compilation and focused Go tests, then stopped at the same 112 existing shared lint findings. Later stages did not run. Its synthetic handler gains remain separate from device frames and deployed load.

The confirmed-match phase passed the three full Go suites and catalog race check on its production source. The final public regression passed after test-only review corrections. Changed-code lint, source caps, repository tooling, and regenerated snapshots passed. Its independent review found no production correctness issue. Both post-commit checks stopped at 112 existing shared lint findings; later stages did not run. The measurement record preserves cache reuse and fresh consumer validation separately. Physical frames and deployed load remain separate.

The paired iPhone 16 Pro Max and Apple TV 4K (third generation) are reachable through the local device tools. A read-only attempt to attach Instruments to the observed phone app process failed before recording. The TV app was not running. These probes establish no physical frame timing or verified build revision. Older supported hardware, deployed first-frame measurement, and production-network benchmarks still need evidence. The goal remains active while these measurement and optimization opportunities remain unresolved.

The title-order phase passed shared, Player, and Subtitles Go suites and the catalog race check. The concurrent Player run had a 90-second HTTPS-helper readiness failure; its isolated check and full retry passed without source changes. The failed run remains recorded. Changed-code lint, source caps, repository tooling, and independent ownership review passed. Both app post-commit checks stopped at 112 existing shared lint findings after compilation and focused tests. The measured catalog source stayed byte-identical through reconciliation with newer main work. Hosted delivery and post-reconciliation checks are recorded separately.

The profile-projection phase passed shared and Subtitles Go suites, the catalog race check, changed-code lint, source caps, and repository tooling. The first Player run overlapped reconciliation and had a stylesheet mismatch plus a maintenance assertion failure. Three focused repetitions passed. A fresh Player run then failed two real HLS speed cases at request deadlines; the isolated five-case HLS check passed. Both failures remain in the [projection record](evidence/catalog-profile-projection-2026-09-30.md). Required post-commit and hosted results remain separate from these local boundaries.


The projection phase also exposed a maintenance test fixture race. Controlled startup delay reproduced cache deletion before streaming. The corrected test isolates explicit upkeep from background scheduling through an existing configuration seam. Thirty repetitions and separate lifecycle checks passed. Removing the real busy guard still fails the assertion. Post-commit Subtitles verification stopped at 112 existing shared lint findings; Player first stopped at the fixture failure. The [projection record](evidence/catalog-profile-projection-2026-09-30.md) retains these failures and controls. The projection delivery results follow below.


The final fixed-revision Player Go suite passed every package after the fixture correction, in 252.426 seconds. Post-commit Player verification passed compilation and focused tests, then stopped at the same 112 existing shared lint findings. Later gate stages did not run. Earlier failures remain in the evidence. Hosted checks, image publication, deployment, and physical-device timing are separate delivery facts.


A Release tvOS XCTest probe completed real Movies-grid navigation and exported three CPU metrics. It emitted no hitch metric. A passing test does not mean zero hitches. The [probe record](evidence/tvos-hitch-metric-2026-09-30.md) preserves samples, commands, source bindings, and this unresolved presentation boundary.

The first projection PR run also found outdated OpenSSL packages in all four production images. PR #387 supplied the runtime dependency repair before final projection delivery. The final projection PR had no Containerfile diff. This repair has no claimed performance gain.


The projection phase merged through [PR #388](https://github.com/Kinosail/kinosail/pull/388) as `199bbcfaea2f6ccc15da68ae48ab8cbaa58f55b4`. Fetched ancestry confirms the reviewed task commit is included. Required PR checks passed. Exact-main [CI run 36778073906](https://github.com/Kinosail/kinosail/actions/runs/36778073906) passed 47 jobs with four skipped. Player and Subtitles SHA, main, and latest image indexes agreed at the recorded publication check. Both architecture child images have that exact OCI revision label. Player index digest is `sha256:bb8d801cfc98c969e537786d7318e598fbbc08274d39bdd7f4583e4442a7aa36`; Subtitles is `sha256:1d981c0cc64af392163cb3c12dd446b4e0a84d98963ff5622705c5b1f2a772e9`. Nox deployment and physical frame timing remain separate and unverified here.

The mixed-library phase passed full shared, Player, and Subtitles Go suites plus the catalog race check on stable production source. Source-bound controls reject incomplete and obsolete admission. Changed-code lint, source caps, regenerated snapshots, and tooling passed. Catalog tests and race checks passed after a byte-identical test-file split. Catalog source stayed byte-identical through reconciliation; focused shared and both-app HTTP checks passed. Post-commit checks and hosted delivery are recorded separately in the [mixed-library record](evidence/catalog-mixed-title-order-2026-09-30.md).


The mixed-library post-commit checks exposed a task environment error and a new complexity warning. A long TMPDIR prevented a private MCP Unix socket from binding. The failure reproduces there; three short-path controls pass. The short-path retry passes the full Player server suite, then stops at shared lint. Final count and key helper extractions resolve the two coordinator complexity warnings without changing admission semantics. Final changed-code lint passes in all three modules; full shared lint reports 112 findings outside the changed coordinator. Public catalog and race tests pass. All 30 final-source response hashes match the 824 timed samples. Paired timings still refer to their recorded earlier binary. Exact final post-commit and hosted boundaries are recorded in [PR #394](https://github.com/Kinosail/kinosail/pull/394) and the mixed-library evidence note.

The first mixed-library hosted run exposed an inherited API-key date-ordering bug at the UTC month boundary. A deterministic regression precedes its one-line timestamp comparator repair. This fixes settings/API chronology without a performance claim. The failed run remains in the mixed-library evidence record. The repaired source passes identity, identity/catalog race, both app API-key checks, the full shared suite, and changed-code shared lint. Snapshot checks and source caps pass. The repaired-revision app gates stop at 112 shared lint findings; later stages do not run. Exact hosted results are recorded in [PR #394](https://github.com/Kinosail/kinosail/pull/394).

Final main reconciliation includes `6ce161669ef9730dfa897123c2040375317561bf`. Its independent API-key comparator repair is identical. Its deterministic boundary/tie matrix supersedes the equivalent task regression. Catalog source hashes remain unchanged. The final PR scope is the mixed catalog cache, its public contracts, benchmarks, and findings artifacts.
