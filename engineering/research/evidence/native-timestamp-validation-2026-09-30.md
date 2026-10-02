# Native timestamp validation performance — 2026-09-30

Catalog validation created ISO8601DateFormatter instances for each timestamp. Whole-second timestamps also created a second formatter after the fractional parser failed. Native catalog pages contain `added` and `progress.updated`, so this repeats throughout a page.

The candidate keeps two existing formatter configurations inside a private `Synchronization.Mutex`. Validation retains the same size bound, regular expression, parser order, date results, and error type. No formatter reference escapes the lock. This is a fixed two-object cache, without response-derived keys or retained timestamp strings.

Matched optimized host controls reduce complete timestamped catalog decoding and validation by about two-thirds. They measure preparation work, not focus animation, input-to-frame latency, physical TV smoothness, energy, or deployment performance.

## Experiment and source

Base: `d046078e893fab952a9c536eb0836e7db6ba0801`. Branch: `codex/native-timestamp-performance`.
Host: ARM64 Mac, macOS 27.0 (26A428), Apple Swift 6.4 (swiftlang-6.4.0.34.1). Drivers compile with `swiftc -O -swift-version 6`.

The [preceding text-validation experiment](native-text-validation-2026-09-30.md#production-shaped-timestamp-control) supplies the deterministic catalog fixtures and complete decode driver. This phase uses its timestamped forms. Nine fixtures contain 36, 60, or 200 synthetic movies in ASCII, raw Unicode, or escaped Unicode. Every item includes `added: 2026-09-29T12:34:56Z` and `progress.updated: 2026-09-30T13:45:10.123456789Z`. Cached forms pass the same JSON tree through JSONEncoder with sorted keys.

The original and candidate binaries compile actual `Contract.swift`, `StrictJSON.swift`, `Media.swift`, and `ActorContract.swift`. Only StrictJSON differs. Before timing, each input tree matches Foundation decoding, and LibraryPage has the expected item count and first identity. File reads, encoding, and those checks are outside timing. Timed library operations perform strict JSON decoding, domain validation, and title-byte consumption. JSON-only rows are retained as controls.

Each scenario has three rounds, one untimed warmup, and five timed decodes per round. Runs are serial: original, candidate, original again. Both original phases use the same binary. Each phase produces 108 rows and the same consumed checksum. No native builds run during these controls. Shared-host interference remains possible.

## Current primary sources and rejected alternatives

[Apple recommends Date.ISO8601FormatStyle](https://developer.apple.com/documentation/foundation/iso8601dateformatter) and explains that Foundation caches identical format styles. The API is worth considering for a new contract. Here, a direct replacement changes observable results in a probe of the current contract: nine fractional digits produce a different Date, and `2026-09-30T23:59:60Z` becomes accepted. The existing formatter rejects that leap-second input. These are local observations on this SDK, not universal claims about every OS version.

[Apple documents changed fractional parsing after Swift 6.2](https://developer.apple.com/documentation/foundation/date/iso8601formatstyle/includingfractionalseconds). The [Foundation proposal](https://github.com/swiftlang/swift-foundation/blob/main/Proposals/0021-ISO8601ComponentsStyle.md) also describes the behavior change. A modern API replacement therefore needs a separate compatibility decision and cross-version evidence. This performance change retains current behavior without a new date grammar or conversion layer.

An unprotected static ISO8601DateFormatter cache fails strict Swift 6 compilation because the formatter is not Sendable. The candidate uses the standard [Mutex API](https://developer.apple.com/documentation/synchronization/mutex). [SE-0433](https://github.com/swiftlang/swift-evolution/blob/main/proposals/0433-mutex.md) explains synchronous exclusive state access and lock release after throwing. The critical section calls only the two local formatters, has no suspension, and returns a Date value. It cannot recursively acquire this lock.

The lock serializes valid parsing across callers. Its fairness and physical-device tail latency remain unmeasured. Invalid syntax fails before acquiring it. Inputs passing syntax but failing parsing throw inside the lock; subsequent valid calls must still succeed. The concurrent contract test exercises that path.

The current formatter also normalizes some calendar inputs, including February 30 and hour 24, in the probe. This is preexisting behavior, retained here. This optimization does not claim stricter calendar validation or full RFC 3339 conformance.

The first private cache overlay failed compilation because its throwing lock call lacked `return`. That draft is excluded from measurements. The corrected overlay passes strict Swift 6 compilation and produces byte-identical probe output to the original. Initial two-iteration candidate timings are exploratory and excluded from the final matrix below.

## Verification and remaining limits

Timestamp tests precede the production edit. They cover UTC meaning, fractions, numeric offsets, the server's zero timestamp, bounded syntax, malformed inputs, concurrent callers, and recovery after a parse failure. The existing catalog persistence test is extended with rejected `added` and `progress.updated` values. It saves a valid page, rejects reload, closes the client, then reopens and reads the unchanged saved page through the public client API.

Before the production edit, the Release tvOS timestamp/cache selection passes 16 tests in two suites. Two additional fractional precision arguments then pass on original source in the macOS harness. The candidate full Release tvOS suite passes 271 tests, with no failures or skips (267 Swift Testing tests in 54 suites and four XCTest tests). Independent review finds no production issue. It catches duplicate title keys in five extended cache-fixture cases. Corrected inputs replace the original title, and the affected Release tvOS cache suite passes all 12 tests. The initial full run is retained but cannot prove those five text-validation paths.

The first candidate macOS harness compile fails because its implicit deployment target is macOS 12, below Mutex availability. This is a private harness configuration error; the application supports iOS/tvOS 26. The harness declares macOS 15, then both original and candidate controls pass 20 tests in two suites. No production availability bypass is added. Initial failure output remains in the task-private evidence. Release iOS simulator compilation also passes. Repository tooling, both architecture snapshot checks, native copy validation, source caps, and whitespace checks pass. Final required post-commit and hosted checks are recorded separately in the pull request. iOS tests, watchOS compilation, physical-device frame traces, energy, and deployment are not run in this phase. macOS command-line timings do not certify iOS or tvOS UI smoothness. Physical-device frame traces and deployed evidence remain separate work in the active performance goal.

## Matched samples

The following generated sections retain summaries, every completed sample, source bindings, and reproducible commands.

### Decode and validation medians

Milliseconds per page. Each median uses three samples; each sample averages five timed decodes.

| Fixture | Original before | Candidate | Original after | Reduction versus before |
| --- | ---: | ---: | ---: | ---: |
| 36-ascii-network | 34.011 | 11.092 | 31.681 | 67.4% |
| 36-ascii-cached | 34.697 | 11.350 | 31.909 | 67.3% |
| 36-utf8-network | 34.688 | 11.360 | 32.276 | 67.3% |
| 36-utf8-cached | 34.693 | 11.231 | 32.146 | 67.6% |
| 36-escaped-network | 33.446 | 11.078 | 33.650 | 66.9% |
| 36-escaped-cached | 33.561 | 11.318 | 32.453 | 66.3% |
| 60-ascii-network | 57.020 | 18.529 | 53.115 | 67.5% |
| 60-ascii-cached | 61.907 | 18.848 | 54.647 | 69.6% |
| 60-utf8-network | 58.539 | 18.594 | 52.459 | 68.2% |
| 60-utf8-cached | 54.228 | 18.948 | 53.502 | 65.1% |
| 60-escaped-network | 55.901 | 18.578 | 53.110 | 66.8% |
| 60-escaped-cached | 57.277 | 18.776 | 53.148 | 67.2% |
| 200-ascii-network | 187.598 | 64.740 | 177.214 | 65.5% |
| 200-ascii-cached | 195.639 | 62.328 | 176.934 | 68.1% |
| 200-utf8-network | 190.898 | 61.899 | 178.102 | 67.6% |
| 200-utf8-cached | 183.365 | 62.849 | 177.277 | 65.7% |
| 200-escaped-network | 194.862 | 62.869 | 174.745 | 67.7% |
| 200-escaped-cached | 188.737 | 61.637 | 177.521 | 67.3% |

### Concurrent page workload

Each trial decodes and validates 16 complete 60-item pages, split evenly across 1, 2, 4, or 8 Swift task-group workers. Three rounds cover ASCII and Unicode inputs. The page count and first identity are checked outside timing. Every trial consumes exactly 960 items. These are elapsed batch time divided by page count, measuring aggregate throughput cost; they are not individual request latency. All three phases consume the same checksum. Original/candidate/original runs are serial, with no native builds overlapping. The complete driver follows below.

| Fixture | Workers | Original before ms/page | Candidate ms/page | Original after ms/page |
| --- | ---: | ---: | ---: | ---: |
| 60-ascii | 1 | 63.119 | 21.933 | 69.868 |
| 60-ascii | 2 | 61.723 | 18.724 | 62.343 |
| 60-ascii | 4 | 54.271 | 19.130 | 54.125 |
| 60-ascii | 8 | 54.168 | 19.627 | 52.900 |
| 60-utf8 | 1 | 63.075 | 21.270 | 63.265 |
| 60-utf8 | 2 | 59.880 | 20.266 | 58.669 |
| 60-utf8 | 4 | 57.988 | 19.217 | 53.585 |
| 60-utf8 | 8 | 54.440 | 19.814 | 51.880 |

The candidate improves every measured worker count. This establishes a throughput gain for this workload despite serialization. It does not establish lock fairness, parallel scaling, or a physical-device latency ceiling.

### Complete decode samples

All 324 completed rows, including JSON-only controls. Values are nanoseconds per decode.

<details>
<summary>Raw decode CSV</summary>

```csv
phase,round,fixture,bytes,operation,iterations,nanoseconds_per_decode
before,0,36-ascii-network,31087,json,5,1603750.0
before,0,36-ascii-network,31087,library,5,34011191.6
before,0,36-ascii-cached,31375,json,5,1549516.8
before,0,36-ascii-cached,31375,library,5,37706550.0
before,0,36-utf8-network,29143,json,5,1582550.0
before,0,36-utf8-network,29143,library,5,42509600.0
before,0,36-utf8-cached,29431,json,5,1950816.6
before,0,36-utf8-cached,29431,library,5,36836650.0
before,0,36-escaped-network,30475,json,5,2421141.6
before,0,36-escaped-network,30475,library,5,47827458.4
before,0,36-escaped-cached,29431,json,5,2540758.4
before,0,36-escaped-cached,29431,library,5,35152450.0
before,0,60-ascii-network,51751,json,5,2643158.4
before,0,60-ascii-network,51751,library,5,57019608.4
before,0,60-ascii-cached,52231,json,5,2548341.6
before,0,60-ascii-cached,52231,library,5,63547383.4
before,0,60-utf8-network,48511,json,5,2562408.4
before,0,60-utf8-network,48511,library,5,55715816.8
before,0,60-utf8-cached,48991,json,5,2651058.2
before,0,60-utf8-cached,48991,library,5,60330716.8
before,0,60-escaped-network,50731,json,5,2696950.0
before,0,60-escaped-network,50731,library,5,58260308.4
before,0,60-escaped-cached,48991,json,5,2633516.8
before,0,60-escaped-cached,48991,library,5,63954366.6
before,0,200-ascii-network,172493,json,5,9485675.0
before,0,200-ascii-network,172493,library,5,194183533.4
before,0,200-ascii-cached,174093,json,5,8806366.8
before,0,200-ascii-cached,174093,library,5,195638950.0
before,0,200-utf8-network,161693,json,5,8914383.4
before,0,200-utf8-network,161693,library,5,187226433.4
before,0,200-utf8-cached,163293,json,5,9187700.0
before,0,200-utf8-cached,163293,library,5,183364858.4
before,0,200-escaped-network,169093,json,5,8608650.0
before,0,200-escaped-network,169093,library,5,194861833.4
before,0,200-escaped-cached,163293,json,5,9913875.0
before,0,200-escaped-cached,163293,library,5,194051841.6
before,1,36-ascii-network,31087,json,5,1555566.6
before,1,36-ascii-network,31087,library,5,35897350.0
before,1,36-ascii-cached,31375,json,5,1554758.4
before,1,36-ascii-cached,31375,library,5,34313683.2
before,1,36-utf8-network,29143,json,5,1622425.0
before,1,36-utf8-network,29143,library,5,34688375.0
before,1,36-utf8-cached,29431,json,5,1595250.0
before,1,36-utf8-cached,29431,library,5,34295041.6
before,1,36-escaped-network,30475,json,5,1565241.6
before,1,36-escaped-network,30475,library,5,33157008.2
before,1,36-escaped-cached,29431,json,5,1580200.0
before,1,36-escaped-cached,29431,library,5,32762025.0
before,1,60-ascii-network,51751,json,5,2531758.4
before,1,60-ascii-network,51751,library,5,59294966.6
before,1,60-ascii-cached,52231,json,5,2635950.0
before,1,60-ascii-cached,52231,library,5,61906883.2
before,1,60-utf8-network,48511,json,5,2739200.0
before,1,60-utf8-network,48511,library,5,58664533.4
before,1,60-utf8-cached,48991,json,5,2673783.2
before,1,60-utf8-cached,48991,library,5,54227933.4
before,1,60-escaped-network,50731,json,5,2648258.4
before,1,60-escaped-network,50731,library,5,55121366.6
before,1,60-escaped-cached,48991,json,5,2584033.2
before,1,60-escaped-cached,48991,library,5,55923600.0
before,1,200-ascii-network,172493,json,5,8319775.0
before,1,200-ascii-network,172493,library,5,187598025.0
before,1,200-ascii-cached,174093,json,5,10134541.8
before,1,200-ascii-cached,174093,library,5,198461741.8
before,1,200-utf8-network,161693,json,5,8582625.0
before,1,200-utf8-network,161693,library,5,198835616.6
before,1,200-utf8-cached,163293,json,5,8905241.6
before,1,200-utf8-cached,163293,library,5,188559083.4
before,1,200-escaped-network,169093,json,5,9217441.6
before,1,200-escaped-network,169093,library,5,198106616.6
before,1,200-escaped-cached,163293,json,5,8787416.8
before,1,200-escaped-cached,163293,library,5,188737116.8
before,2,36-ascii-network,31087,json,5,1529075.0
before,2,36-ascii-network,31087,library,5,33543358.2
before,2,36-ascii-cached,31375,json,5,1546908.2
before,2,36-ascii-cached,31375,library,5,34697091.6
before,2,36-utf8-network,29143,json,5,1537116.6
before,2,36-utf8-network,29143,library,5,33154341.6
before,2,36-utf8-cached,29431,json,5,1578358.4
before,2,36-utf8-cached,29431,library,5,34693233.4
before,2,36-escaped-network,30475,json,5,1599608.2
before,2,36-escaped-network,30475,library,5,33445575.0
before,2,36-escaped-cached,29431,json,5,1552025.0
before,2,36-escaped-cached,29431,library,5,33560825.0
before,2,60-ascii-network,51751,json,5,2564133.4
before,2,60-ascii-network,51751,library,5,55444650.0
before,2,60-ascii-cached,52231,json,5,2576075.0
before,2,60-ascii-cached,52231,library,5,59358225.0
before,2,60-utf8-network,48511,json,5,2561950.0
before,2,60-utf8-network,48511,library,5,58538775.0
before,2,60-utf8-cached,48991,json,5,2558950.0
before,2,60-utf8-cached,48991,library,5,53112041.6
before,2,60-escaped-network,50731,json,5,3361908.4
before,2,60-escaped-network,50731,library,5,55900725.0
before,2,60-escaped-cached,48991,json,5,2672691.6
before,2,60-escaped-cached,48991,library,5,57276841.6
before,2,200-ascii-network,172493,json,5,8876208.4
before,2,200-ascii-network,172493,library,5,182252175.0
before,2,200-ascii-cached,174093,json,5,8471600.0
before,2,200-ascii-cached,174093,library,5,181821483.2
before,2,200-utf8-network,161693,json,5,9985416.6
before,2,200-utf8-network,161693,library,5,190897508.4
before,2,200-utf8-cached,163293,json,5,8606750.0
before,2,200-utf8-cached,163293,library,5,180638100.0
before,2,200-escaped-network,169093,json,5,8775250.0
before,2,200-escaped-network,169093,library,5,178908941.6
before,2,200-escaped-cached,163293,json,5,8765366.8
before,2,200-escaped-cached,163293,library,5,187878400.0
candidate,0,36-ascii-network,31087,json,5,1488516.6
candidate,0,36-ascii-network,31087,library,5,11393816.6
candidate,0,36-ascii-cached,31375,json,5,1561700.0
candidate,0,36-ascii-cached,31375,library,5,11392566.6
candidate,0,36-utf8-network,29143,json,5,1582650.0
candidate,0,36-utf8-network,29143,library,5,12430433.4
candidate,0,36-utf8-cached,29431,json,5,1584566.8
candidate,0,36-utf8-cached,29431,library,5,11230650.0
candidate,0,36-escaped-network,30475,json,5,1577658.4
candidate,0,36-escaped-network,30475,library,5,11220200.0
candidate,0,36-escaped-cached,29431,json,5,1573625.0
candidate,0,36-escaped-cached,29431,library,5,11457475.0
candidate,0,60-ascii-network,51751,json,5,2552575.0
candidate,0,60-ascii-network,51751,library,5,18720483.2
candidate,0,60-ascii-cached,52231,json,5,2514716.6
candidate,0,60-ascii-cached,52231,library,5,19692541.8
candidate,0,60-utf8-network,48511,json,5,2562600.0
candidate,0,60-utf8-network,48511,library,5,18594391.6
candidate,0,60-utf8-cached,48991,json,5,2578766.8
candidate,0,60-utf8-cached,48991,library,5,18441041.6
candidate,0,60-escaped-network,50731,json,5,2541583.2
candidate,0,60-escaped-network,50731,library,5,18488650.0
candidate,0,60-escaped-cached,48991,json,5,2603591.8
candidate,0,60-escaped-cached,48991,library,5,18775566.8
candidate,0,200-ascii-network,172493,json,5,8387125.0
candidate,0,200-ascii-network,172493,library,5,64740433.4
candidate,0,200-ascii-cached,174093,json,5,8457416.8
candidate,0,200-ascii-cached,174093,library,5,63174216.6
candidate,0,200-utf8-network,161693,json,5,8706650.0
candidate,0,200-utf8-network,161693,library,5,61086616.6
candidate,0,200-utf8-cached,163293,json,5,8861016.8
candidate,0,200-utf8-cached,163293,library,5,61380975.0
candidate,0,200-escaped-network,169093,json,5,8591825.0
candidate,0,200-escaped-network,169093,library,5,62868600.0
candidate,0,200-escaped-cached,163293,json,5,8571300.0
candidate,0,200-escaped-cached,163293,library,5,61340258.4
candidate,1,36-ascii-network,31087,json,5,1473033.4
candidate,1,36-ascii-network,31087,library,5,11042725.0
candidate,1,36-ascii-cached,31375,json,5,1514908.4
candidate,1,36-ascii-cached,31375,library,5,11018508.2
candidate,1,36-utf8-network,29143,json,5,1521333.4
candidate,1,36-utf8-network,29143,library,5,11359591.6
candidate,1,36-utf8-cached,29431,json,5,1556841.6
candidate,1,36-utf8-cached,29431,library,5,11073075.0
candidate,1,36-escaped-network,30475,json,5,1570083.4
candidate,1,36-escaped-network,30475,library,5,11077908.4
candidate,1,36-escaped-cached,29431,json,5,1521616.6
candidate,1,36-escaped-cached,29431,library,5,11074508.4
candidate,1,60-ascii-network,51751,json,5,2452875.0
candidate,1,60-ascii-network,51751,library,5,18529208.4
candidate,1,60-ascii-cached,52231,json,5,2555916.6
candidate,1,60-ascii-cached,52231,library,5,18284383.4
candidate,1,60-utf8-network,48511,json,5,2508133.4
candidate,1,60-utf8-network,48511,library,5,18338408.4
candidate,1,60-utf8-cached,48991,json,5,2727925.0
candidate,1,60-utf8-cached,48991,library,5,19576525.0
candidate,1,60-escaped-network,50731,json,5,2711700.0
candidate,1,60-escaped-network,50731,library,5,18578275.0
candidate,1,60-escaped-cached,48991,json,5,2539716.8
candidate,1,60-escaped-cached,48991,library,5,18324750.0
candidate,1,200-ascii-network,172493,json,5,8299016.6
candidate,1,200-ascii-network,172493,library,5,63427866.6
candidate,1,200-ascii-cached,174093,json,5,8473533.4
candidate,1,200-ascii-cached,174093,library,5,62328375.0
candidate,1,200-utf8-network,161693,json,5,8589175.0
candidate,1,200-utf8-network,161693,library,5,62404675.0
candidate,1,200-utf8-cached,163293,json,5,8630958.2
candidate,1,200-utf8-cached,163293,library,5,62848558.4
candidate,1,200-escaped-network,169093,json,5,8600950.0
candidate,1,200-escaped-network,169093,library,5,63608366.8
candidate,1,200-escaped-cached,163293,json,5,8656158.2
candidate,1,200-escaped-cached,163293,library,5,61636958.2
candidate,2,36-ascii-network,31087,json,5,1518033.2
candidate,2,36-ascii-network,31087,library,5,11092441.6
candidate,2,36-ascii-cached,31375,json,5,1475566.6
candidate,2,36-ascii-cached,31375,library,5,11349866.6
candidate,2,36-utf8-network,29143,json,5,1573475.0
candidate,2,36-utf8-network,29143,library,5,11170533.4
candidate,2,36-utf8-cached,29431,json,5,1530533.2
candidate,2,36-utf8-cached,29431,library,5,11594541.6
candidate,2,36-escaped-network,30475,json,5,1583825.0
candidate,2,36-escaped-network,30475,library,5,10961800.0
candidate,2,36-escaped-cached,29431,json,5,1632625.0
candidate,2,36-escaped-cached,29431,library,5,11317750.0
candidate,2,60-ascii-network,51751,json,5,2500025.0
candidate,2,60-ascii-network,51751,library,5,18298525.0
candidate,2,60-ascii-cached,52231,json,5,2607075.0
candidate,2,60-ascii-cached,52231,library,5,18848016.8
candidate,2,60-utf8-network,48511,json,5,2627500.0
candidate,2,60-utf8-network,48511,library,5,19406158.2
candidate,2,60-utf8-cached,48991,json,5,2580475.0
candidate,2,60-utf8-cached,48991,library,5,18947791.8
candidate,2,60-escaped-network,50731,json,5,2587133.4
candidate,2,60-escaped-network,50731,library,5,19055991.6
candidate,2,60-escaped-cached,48991,json,5,2717808.2
candidate,2,60-escaped-cached,48991,library,5,19368608.4
candidate,2,200-ascii-network,172493,json,5,8510066.6
candidate,2,200-ascii-network,172493,library,5,66669716.8
candidate,2,200-ascii-cached,174093,json,5,9458741.6
candidate,2,200-ascii-cached,174093,library,5,62089658.4
candidate,2,200-utf8-network,161693,json,5,8455800.0
candidate,2,200-utf8-network,161693,library,5,61898691.6
candidate,2,200-utf8-cached,163293,json,5,8699183.4
candidate,2,200-utf8-cached,163293,library,5,64199533.2
candidate,2,200-escaped-network,169093,json,5,8747325.0
candidate,2,200-escaped-network,169093,library,5,62673666.6
candidate,2,200-escaped-cached,163293,json,5,8534216.6
candidate,2,200-escaped-cached,163293,library,5,61684783.2
after,0,36-ascii-network,31087,json,5,1501091.6
after,0,36-ascii-network,31087,library,5,31310233.4
after,0,36-ascii-cached,31375,json,5,1516375.0
after,0,36-ascii-cached,31375,library,5,32456475.0
after,0,36-utf8-network,29143,json,5,1618875.0
after,0,36-utf8-network,29143,library,5,31734275.0
after,0,36-utf8-cached,29431,json,5,1525141.6
after,0,36-utf8-cached,29431,library,5,31271141.8
after,0,36-escaped-network,30475,json,5,1588091.8
after,0,36-escaped-network,30475,library,5,33222350.0
after,0,36-escaped-cached,29431,json,5,1550833.4
after,0,36-escaped-cached,29431,library,5,32234466.6
after,0,60-ascii-network,51751,json,5,2499383.4
after,0,60-ascii-network,51751,library,5,52987241.6
after,0,60-ascii-cached,52231,json,5,2557350.0
after,0,60-ascii-cached,52231,library,5,53837066.6
after,0,60-utf8-network,48511,json,5,2615441.6
after,0,60-utf8-network,48511,library,5,52328758.4
after,0,60-utf8-cached,48991,json,5,2571983.4
after,0,60-utf8-cached,48991,library,5,52882375.0
after,0,60-escaped-network,50731,json,5,2617966.8
after,0,60-escaped-network,50731,library,5,55660108.4
after,0,60-escaped-cached,48991,json,5,2617900.0
after,0,60-escaped-cached,48991,library,5,54981583.2
after,0,200-ascii-network,172493,json,5,8301616.8
after,0,200-ascii-network,172493,library,5,180265366.6
after,0,200-ascii-cached,174093,json,5,10522833.4
after,0,200-ascii-cached,174093,library,5,176087275.0
after,0,200-utf8-network,161693,json,5,8693875.0
after,0,200-utf8-network,161693,library,5,178101516.6
after,0,200-utf8-cached,163293,json,5,8727591.6
after,0,200-utf8-cached,163293,library,5,182930233.4
after,0,200-escaped-network,169093,json,5,8584750.0
after,0,200-escaped-network,169093,library,5,177767900.0
after,0,200-escaped-cached,163293,json,5,8554241.8
after,0,200-escaped-cached,163293,library,5,177857541.6
after,1,36-ascii-network,31087,json,5,1494775.0
after,1,36-ascii-network,31087,library,5,31680833.4
after,1,36-ascii-cached,31375,json,5,1511175.0
after,1,36-ascii-cached,31375,library,5,31536241.6
after,1,36-utf8-network,29143,json,5,1510750.0
after,1,36-utf8-network,29143,library,5,32276466.6
after,1,36-utf8-cached,29431,json,5,2088300.0
after,1,36-utf8-cached,29431,library,5,32978616.6
after,1,36-escaped-network,30475,json,5,1878700.0
after,1,36-escaped-network,30475,library,5,33649683.4
after,1,36-escaped-cached,29431,json,5,1774525.0
after,1,36-escaped-cached,29431,library,5,33855900.0
after,1,60-ascii-network,51751,json,5,2485591.6
after,1,60-ascii-network,51751,library,5,53114833.4
after,1,60-ascii-cached,52231,json,5,2509808.2
after,1,60-ascii-cached,52231,library,5,56024725.0
after,1,60-utf8-network,48511,json,5,2708075.0
after,1,60-utf8-network,48511,library,5,52458566.6
after,1,60-utf8-cached,48991,json,5,2585200.0
after,1,60-utf8-cached,48991,library,5,54665466.6
after,1,60-escaped-network,50731,json,5,2669250.0
after,1,60-escaped-network,50731,library,5,52254066.6
after,1,60-escaped-cached,48991,json,5,2544191.6
after,1,60-escaped-cached,48991,library,5,53147541.6
after,1,200-ascii-network,172493,json,5,8331108.2
after,1,200-ascii-network,172493,library,5,173469833.4
after,1,200-ascii-cached,174093,json,5,8439200.0
after,1,200-ascii-cached,174093,library,5,176934158.2
after,1,200-utf8-network,161693,json,5,8635366.8
after,1,200-utf8-network,161693,library,5,175621400.0
after,1,200-utf8-cached,163293,json,5,8712891.8
after,1,200-utf8-cached,163293,library,5,177277308.4
after,1,200-escaped-network,169093,json,5,8683258.4
after,1,200-escaped-network,169093,library,5,173176150.0
after,1,200-escaped-cached,163293,json,5,8833283.4
after,1,200-escaped-cached,163293,library,5,177521091.8
after,2,36-ascii-network,31087,json,5,1536275.0
after,2,36-ascii-network,31087,library,5,31886183.2
after,2,36-ascii-cached,31375,json,5,1509866.6
after,2,36-ascii-cached,31375,library,5,31909058.2
after,2,36-utf8-network,29143,json,5,1579508.4
after,2,36-utf8-network,29143,library,5,32358225.0
after,2,36-utf8-cached,29431,json,5,1557350.0
after,2,36-utf8-cached,29431,library,5,32145858.4
after,2,36-escaped-network,30475,json,5,1477200.0
after,2,36-escaped-network,30475,library,5,33926141.6
after,2,36-escaped-cached,29431,json,5,1572191.6
after,2,36-escaped-cached,29431,library,5,32453408.4
after,2,60-ascii-network,51751,json,5,2601950.0
after,2,60-ascii-network,51751,library,5,57403541.8
after,2,60-ascii-cached,52231,json,5,2673441.6
after,2,60-ascii-cached,52231,library,5,54646533.2
after,2,60-utf8-network,48511,json,5,2623175.0
after,2,60-utf8-network,48511,library,5,53635166.6
after,2,60-utf8-cached,48991,json,5,2571691.8
after,2,60-utf8-cached,48991,library,5,53502300.0
after,2,60-escaped-network,50731,json,5,2566191.6
after,2,60-escaped-network,50731,library,5,53110400.0
after,2,60-escaped-cached,48991,json,5,2648341.6
after,2,60-escaped-cached,48991,library,5,52947508.4
after,2,200-ascii-network,172493,json,5,8368183.4
after,2,200-ascii-network,172493,library,5,177214133.4
after,2,200-ascii-cached,174093,json,5,8792641.8
after,2,200-ascii-cached,174093,library,5,178899975.0
after,2,200-utf8-network,161693,json,5,8698091.8
after,2,200-utf8-network,161693,library,5,182101958.4
after,2,200-utf8-cached,163293,json,5,8621316.8
after,2,200-utf8-cached,163293,library,5,175133141.8
after,2,200-escaped-network,169093,json,5,8540666.6
after,2,200-escaped-network,169093,library,5,174744975.0
after,2,200-escaped-cached,163293,json,5,8660716.6
after,2,200-escaped-cached,163293,library,5,174353141.6
```

</details>

### Complete concurrent samples

All 72 completed trials.

<details>
<summary>Raw concurrent CSV</summary>

```csv
phase,round,fixture,workers,pages,elapsed_ns,nanoseconds_per_page
before,0,60-ascii,1,16,981953375,61372085.9375
before,0,60-ascii,2,16,987575667,61723479.1875
before,0,60-ascii,4,16,864795125,54049695.3125
before,0,60-ascii,8,16,885561959,55347622.4375
before,0,60-utf8,1,16,960997666,60062354.125
before,0,60-utf8,2,16,975264792,60954049.5
before,0,60-utf8,4,16,927802041,57987627.5625
before,0,60-utf8,8,16,900174791,56260924.4375
before,1,60-ascii,1,16,1020010583,63750661.4375
before,1,60-ascii,2,16,908952208,56809513.0
before,1,60-ascii,4,16,871633334,54477083.375
before,1,60-ascii,8,16,824756291,51547268.1875
before,1,60-utf8,1,16,1009204250,63075265.625
before,1,60-utf8,2,16,958082500,59880156.25
before,1,60-utf8,4,16,957829042,59864315.125
before,1,60-utf8,8,16,858875292,53679705.75
before,2,60-ascii,1,16,1009903458,63118966.125
before,2,60-ascii,2,16,1006883167,62930197.9375
before,2,60-ascii,4,16,868331875,54270742.1875
before,2,60-ascii,8,16,866684291,54167768.1875
before,2,60-utf8,1,16,1038781500,64923843.75
before,2,60-utf8,2,16,846359750,52897484.375
before,2,60-utf8,4,16,841513417,52594588.5625
before,2,60-utf8,8,16,871032084,54439505.25
candidate,0,60-ascii,1,16,331701917,20731369.8125
candidate,0,60-ascii,2,16,289509167,18094322.9375
candidate,0,60-ascii,4,16,307175917,19198494.8125
candidate,0,60-ascii,8,16,314025375,19626585.9375
candidate,0,60-utf8,1,16,353531709,22095731.8125
candidate,0,60-utf8,2,16,324249208,20265575.5
candidate,0,60-utf8,4,16,316339458,19771216.125
candidate,0,60-utf8,8,16,298372500,18648281.25
candidate,1,60-ascii,1,16,358312375,22394523.4375
candidate,1,60-ascii,2,16,299584958,18724059.875
candidate,1,60-ascii,4,16,297465292,18591580.75
candidate,1,60-ascii,8,16,326047083,20377942.6875
candidate,1,60-utf8,1,16,340319292,21269955.75
candidate,1,60-utf8,2,16,294459417,18403713.5625
candidate,1,60-utf8,4,16,307465042,19216565.125
candidate,1,60-utf8,8,16,317021667,19813854.1875
candidate,2,60-ascii,1,16,350927667,21932979.1875
candidate,2,60-ascii,2,16,310302084,19393880.25
candidate,2,60-ascii,4,16,306085916,19130369.75
candidate,2,60-ascii,8,16,262192416,16387026.0
candidate,2,60-utf8,1,16,331387375,20711710.9375
candidate,2,60-utf8,2,16,333936500,20871031.25
candidate,2,60-utf8,4,16,285526750,17845421.875
candidate,2,60-utf8,8,16,341509000,21344312.5
after,0,60-ascii,1,16,1127889667,70493104.1875
after,0,60-ascii,2,16,997482625,62342664.0625
after,0,60-ascii,4,16,900314958,56269684.875
after,0,60-ascii,8,16,862989542,53936846.375
after,0,60-utf8,1,16,991257583,61953598.9375
after,0,60-utf8,2,16,954689875,59668117.1875
after,0,60-utf8,4,16,883029375,55189335.9375
after,0,60-utf8,8,16,944164042,59010252.625
after,1,60-ascii,1,16,1046232625,65389539.0625
after,1,60-ascii,2,16,893719333,55857458.3125
after,1,60-ascii,4,16,833892250,52118265.625
after,1,60-ascii,8,16,846403167,52900197.9375
after,1,60-utf8,1,16,1012243875,63265242.1875
after,1,60-utf8,2,16,938707500,58669218.75
after,1,60-utf8,4,16,833990125,52124382.8125
after,1,60-utf8,8,16,830066833,51879177.0625
after,2,60-ascii,1,16,1117880292,69867518.25
after,2,60-ascii,2,16,1024006250,64000390.625
after,2,60-ascii,4,16,865998375,54124898.4375
after,2,60-ascii,8,16,814969834,50935614.625
after,2,60-utf8,1,16,1112339875,69521242.1875
after,2,60-utf8,2,16,909589042,56849315.125
after,2,60-utf8,4,16,857360542,53585033.875
after,2,60-utf8,8,16,830076750,51879796.875
```

</details>

## Source bindings and reproduction

The following manifests bind the measured source overlays, binaries, shared driver, and input fixtures. The candidate StrictJSON overlay is byte-identical to production source after the edit. Other compiled Core inputs match the base revision. Unchanged compiled Core source bindings:

```json
{
  "apps/player/apps/native/Sources/Core/Contract.swift": "ef213d8ed4b4dd1fbdf320839d7493b3f7cf1865cc19656af671757fe3efd891",
  "apps/player/apps/native/Sources/Core/Media.swift": "5dcc5b24cd556a340383ac0c5a41d389ae4b1b2a180de512198ee7a21a8c368e",
  "apps/player/apps/native/Sources/Core/ActorContract.swift": "56798d719321f16dcefafd49829d04cb0d7cb5d7795713ff2608e728176b9af1"
}
```

The [preceding reproduction record](native-text-validation-2026-09-30.md#reproduction) includes the full common fixture generator and decode driver. Regenerate its dated fixtures, retain that driver, then compile with the commands below. Paths are task-private and contain synthetic data.

<details>
<summary>Measured manifests</summary>

```json
{
  "decode": {
    "revision": "d046078e893fab952a9c536eb0836e7db6ba0801",
    "created": "2026-10-01T02:41:40.796019+00:00",
    "runs": [
      {
        "phase": "before",
        "command": [
          ".verification/native-timestamps/decode-baseline",
          ".verification/native-interaction/dated",
          "3",
          "5"
        ],
        "exitCode": 0,
        "seconds": 37.62830216699513
      },
      {
        "phase": "candidate",
        "command": [
          ".verification/native-timestamps/decode-candidate",
          ".verification/native-interaction/dated",
          "3",
          "5"
        ],
        "exitCode": 0,
        "seconds": 13.413032207987271
      },
      {
        "phase": "after",
        "command": [
          ".verification/native-timestamps/decode-baseline",
          ".verification/native-interaction/dated",
          "3",
          "5"
        ],
        "exitCode": 0,
        "seconds": 34.956573208997725
      }
    ],
    "bindings": {
      ".verification/native-timestamps/StrictJSON-baseline.swift": "b48a9cfb9828c11ff8bf8a8ea80705d0722b3b63e27b147bf0c476fed0af1e62",
      ".verification/native-timestamps/StrictJSON-candidate.swift": "b7c51b69766099781e791be4eb3a897196bc9208e7683cfe8ee55799f2390111",
      ".verification/native-timestamps/decode-baseline": "47bcd1c2f9c316402cc8c47abbf2d11f199f961e95fefaa25fca2c7382779c73",
      ".verification/native-timestamps/decode-candidate": "bd7811962b9c74921b314c546fd6848f7e3fae5b22a556ba10755a862054605f",
      ".verification/native-interaction/DecodeBenchmark.swift": "86ea5c04dfe9b621b18f7f7a4e93467f6300a95af9c66f9fc9c87d10c614eb06",
      ".verification/native-interaction/dated/200-utf8.json": "2893b5b05f5cef8f8114324baa63e48d9d3d8ade6cf60f563e985ccea33b75fb",
      ".verification/native-interaction/dated/200-escaped.json": "f834ed2780b2069d6d81ba48ed43649028d28c95fdf80c32e6f19a919fd66f62",
      ".verification/native-interaction/dated/36-ascii.json": "dec9024a8d0b4cd156c0ece07b2f67a493316fccca91889dafde06de209a81b3",
      ".verification/native-interaction/dated/36-utf8.json": "7e7a059e210bfbc69022527776e845c2d7d6aaca86760800e1365344d4051014",
      ".verification/native-interaction/dated/60-utf8.json": "abc43e4abc079c9b1dad0cf8da04d0f0eda621da1ff2782db4958241986753c6",
      ".verification/native-interaction/dated/60-escaped.json": "f5f0857db5fe1548fa36b6310efe72b25c9e58dd14ea076eaacc8db0b5cf1619",
      ".verification/native-interaction/dated/36-escaped.json": "4e5db0660ae3fdbb4bd87ad761d4fe0d46adf107fefd7cf9da5a8050a68188fe",
      ".verification/native-interaction/dated/200-ascii.json": "7760dcab7d64bd12d4ab81b7d7b932e6c820ee5f9289232b6970bace8d44b9a2",
      ".verification/native-interaction/dated/60-ascii.json": "f3f125814160e5266caa1865ee15e4939896238ccc3756e959f1e9fb3c4a8981"
    }
  },
  "concurrent": {
    "runs": [
      {
        "phase": "before",
        "command": [
          ".verification/native-timestamps/concurrent-baseline",
          ".verification/native-interaction/dated"
        ],
        "exitCode": 0,
        "seconds": 23.173329124983866
      },
      {
        "phase": "candidate",
        "command": [
          ".verification/native-timestamps/concurrent-candidate",
          ".verification/native-interaction/dated"
        ],
        "exitCode": 0,
        "seconds": 7.8821702919958625
      },
      {
        "phase": "after",
        "command": [
          ".verification/native-timestamps/concurrent-baseline",
          ".verification/native-interaction/dated"
        ],
        "exitCode": 0,
        "seconds": 22.83207241698983
      }
    ],
    "bindings": {
      ".verification/native-timestamps/ConcurrentDecodeBenchmark.swift": "f49b4b034ed1d2018123a3a583a56bd2323b9563cbce86cfef802faa33f75617",
      ".verification/native-timestamps/concurrent-baseline": "a75b2562840c4a7b5dfc94ce26cfc459141b55387d0019be97d402ddafc8bb99",
      ".verification/native-timestamps/concurrent-candidate": "5e7ecd6c402bc21588afbfc180ffb13256daa2b281f527fe90a24a3a67392bc8"
    }
  }
}
```

</details>

```sh
mkdir -p .verification/native-timestamps
git show d046078e893fab952a9c536eb0836e7db6ba0801:apps/player/apps/native/Sources/Core/StrictJSON.swift > .verification/native-timestamps/StrictJSON-baseline.swift
cp apps/player/apps/native/Sources/Core/StrictJSON.swift .verification/native-timestamps/StrictJSON-candidate.swift
for variant in baseline candidate; do
  swiftc -O -swift-version 6 \
    apps/player/apps/native/Sources/Core/Contract.swift \
    .verification/native-timestamps/StrictJSON-$variant.swift \
    apps/player/apps/native/Sources/Core/Media.swift \
    apps/player/apps/native/Sources/Core/ActorContract.swift \
    .verification/native-interaction/DecodeBenchmark.swift \
    -o .verification/native-timestamps/decode-$variant
  swiftc -O -swift-version 6 \
    apps/player/apps/native/Sources/Core/Contract.swift \
    .verification/native-timestamps/StrictJSON-$variant.swift \
    apps/player/apps/native/Sources/Core/Media.swift \
    apps/player/apps/native/Sources/Core/ActorContract.swift \
    .verification/native-timestamps/ConcurrentDecodeBenchmark.swift \
    -o .verification/native-timestamps/concurrent-$variant
done
for variant in baseline candidate baseline; do
  .verification/native-timestamps/decode-$variant .verification/native-interaction/dated 3 5
done
for variant in baseline candidate baseline; do
  .verification/native-timestamps/concurrent-$variant .verification/native-interaction/dated
done
```

ConcurrentDecodeBenchmark.swift:

```swift
import Foundation

@main struct ConcurrentDecodeBenchmark {
    static func main() async throws {
        let root = URL(fileURLWithPath: CommandLine.arguments[1])
        let server = try ServerAddress("https://benchmark.example.invalid")
        var checksum = 0
        print("round,fixture,workers,pages,elapsed_ns,nanoseconds_per_page"); fflush(stdout)
        for round in 0..<3 {
            for encoding in ["ascii", "utf8"] {
                let data = try Data(contentsOf: root.appendingPathComponent("60-\(encoding).json"))
                let page = try LibraryPage(StrictJSON.decode(data), server: server)
                guard page.items.count == 60, page.items.first?.id == "movie-000" else { fatalError("contract mismatch") }
                for workers in [1, 2, 4, 8] {
                    let start = DispatchTime.now().uptimeNanoseconds
                    let sum = try await withThrowingTaskGroup(of: Int.self) { group in
                        for _ in 0..<workers {
                            group.addTask {
                                var count = 0
                                for _ in 0..<(16 / workers) {
                                    count += try LibraryPage(StrictJSON.decode(data), server: server).items.count
                                }
                                return count
                            }
                        }
                        var sum = 0
                        for try await count in group { sum += count }
                        return sum
                    }
                    let elapsed = DispatchTime.now().uptimeNanoseconds - start
                    guard sum == 960 else { fatalError("concurrent contract mismatch") }
                    checksum += sum
                    print("\(round),60-\(encoding),\(workers),16,\(elapsed),\(Double(elapsed) / 16)"); fflush(stdout)
                }
            }
        }
        fputs("checksum=\(checksum)\n", stderr)
    }
}
```

## Native verification commands and bindings

The baseline selection and full candidate commands are retained below. After correcting only the cache fixture, the same candidate command runs with `-only-testing:Kinosail-tvOSTests/CatalogCacheTests` and a separate `corrected-cache.xcresult` output. It passes 12 tests. The initial full result is Passed, 271 total, zero failed, zero skipped. Production source remains byte-identical across the correction. Tests use synthetic fixture data and the isolated Kinosail Speed TV simulator on tvOS 27.

<details>
<summary>Native test manifests</summary>

```json
{
  "baseline-native-manifest.json": {
    "time": "2026-10-01T02:47:30.546507+00:00",
    "command": [
      "xcodebuild",
      "-project",
      "apps/player/apps/native/Kinosail.xcodeproj",
      "-scheme",
      "Kinosail-tvOS",
      "-configuration",
      "Release",
      "-destination",
      "id=64468BFE-9114-4FBF-860B-79DF45F70011",
      "-derivedDataPath",
      "apps/player/apps/native/.build/speed-native-validation",
      "-resultBundlePath",
      ".verification/native-timestamps/baseline-contract-cache.xcresult",
      "-jobs",
      "4",
      "-parallel-testing-enabled",
      "NO",
      "-collect-test-diagnostics",
      "never",
      "-only-testing:Kinosail-tvOSTests/TimestampContractTests",
      "-only-testing:Kinosail-tvOSTests/CatalogCacheTests",
      "CODE_SIGNING_ALLOWED=YES",
      "CODE_SIGN_IDENTITY=-",
      "ENABLE_TESTABILITY=YES",
      "ONLY_ACTIVE_ARCH=YES",
      "KINOSAIL_SOURCE_REVISION=d046078e893fab952a9c536eb0836e7db6ba0801-timestamp-tests",
      "test"
    ],
    "sourceBindings": {
      "apps/player/apps/native/Sources/Core/StrictJSON.swift": "b48a9cfb9828c11ff8bf8a8ea80705d0722b3b63e27b147bf0c476fed0af1e62",
      "apps/player/apps/native/Tests/TimestampContractTests.swift": "0631613467ec3e39b9e35fec591a9c48685e5d463056dc485b6531f783fdc33f",
      "apps/player/apps/native/Tests/CatalogCacheTests.swift": "25be2e5b1e75ba187f6eefa48c54a946695ff705b338504da03aa83d0eb7efff"
    },
    "exitCode": 0
  },
  "candidate-native-manifest.json": {
    "time": "2026-10-01T02:50:35.106089+00:00",
    "command": [
      "xcodebuild",
      "-project",
      "apps/player/apps/native/Kinosail.xcodeproj",
      "-scheme",
      "Kinosail-tvOS",
      "-configuration",
      "Release",
      "-destination",
      "id=64468BFE-9114-4FBF-860B-79DF45F70011",
      "-derivedDataPath",
      "apps/player/apps/native/.build/speed-native-validation",
      "-resultBundlePath",
      ".verification/native-timestamps/candidate-full-tv.xcresult",
      "-jobs",
      "4",
      "-parallel-testing-enabled",
      "NO",
      "-collect-test-diagnostics",
      "never",
      "CODE_SIGNING_ALLOWED=YES",
      "CODE_SIGN_IDENTITY=-",
      "ENABLE_TESTABILITY=YES",
      "ONLY_ACTIVE_ARCH=YES",
      "KINOSAIL_SOURCE_REVISION=d046078e893fab952a9c536eb0836e7db6ba0801-timestamp-cache",
      "test"
    ],
    "sourceBindings": {
      "apps/player/apps/native/Sources/Core/StrictJSON.swift": "b7c51b69766099781e791be4eb3a897196bc9208e7683cfe8ee55799f2390111",
      "apps/player/apps/native/Tests/TimestampContractTests.swift": "b475fb4a0604828ce0b108d46e269796346faa5054f6b242b3930782d27f3b17",
      "apps/player/apps/native/Tests/CatalogCacheTests.swift": "25be2e5b1e75ba187f6eefa48c54a946695ff705b338504da03aa83d0eb7efff"
    },
    "exitCode": 0
  },
  "corrected-cache-manifest.json": {
    "command": [
      "xcodebuild",
      "-project",
      "apps/player/apps/native/Kinosail.xcodeproj",
      "-scheme",
      "Kinosail-tvOS",
      "-configuration",
      "Release",
      "-destination",
      "id=64468BFE-9114-4FBF-860B-79DF45F70011",
      "-derivedDataPath",
      "apps/player/apps/native/.build/speed-native-validation",
      "-resultBundlePath",
      ".verification/native-timestamps/corrected-cache.xcresult",
      "-jobs",
      "4",
      "-parallel-testing-enabled",
      "NO",
      "-collect-test-diagnostics",
      "never",
      "-only-testing:Kinosail-tvOSTests/CatalogCacheTests",
      "CODE_SIGNING_ALLOWED=YES",
      "CODE_SIGN_IDENTITY=-",
      "ENABLE_TESTABILITY=YES",
      "ONLY_ACTIVE_ARCH=YES",
      "KINOSAIL_SOURCE_REVISION=d046078e893fab952a9c536eb0836e7db6ba0801-timestamp-cache",
      "test"
    ],
    "exitCode": 0,
    "testSHA256": "71d2253fb6bbd364f88a749a363ec14aadbff2d491928cd46d8987bca6e617a1"
  }
}
```

</details>

Release iOS compilation:

```sh
xcodebuild -project apps/player/apps/native/Kinosail.xcodeproj \
  -scheme Kinosail-iOS -configuration Release \
  -destination "generic/platform=iOS Simulator" \
  -derivedDataPath apps/player/apps/native/.build/speed-ios-validation -jobs 4 \
  CODE_SIGNING_ALLOWED=YES CODE_SIGN_IDENTITY=- ONLY_ACTIVE_ARCH=YES \
  KINOSAIL_SOURCE_REVISION=d046078e893fab952a9c536eb0836e7db6ba0801-timestamp-cache build
```

The private macOS package copies actual Contract, StrictJSON, Media, and ActorContract sources plus ContractTests and TimestampContractTests. Its Swift tools version is 6.2 and its platform is macOS 15. `swift test --package-path .verification/native-timestamps/ContractHarness` passes 20 tests on each source variant after correcting that platform declaration. This is supplemental Core evidence, not a supported macOS application target.

## Format-style contract probe

The original and synchronized-cache Input.date probes emit identical output. Columns are input, Input.date epoch seconds (or rejection), Date.ISO8601FormatStyle epoch seconds, and equality. Historical and normalized-calendar values are retained as observations, not asserted as newly desired contract behavior.

```text
0000-01-01T00:00:00Z -62167392000.0 -62167392000.0 true
0001-01-01T00:00:00Z -62135769600.0 -62135769600.0 true
1582-10-10T00:00:00Z -12218860800.0 -12218860800.0 true
1970-01-01T00:00:00Z 0.0 0.0 true
2026-09-30T13:45:10.123456789Z 1790775910.123 1790775910.123457 false
2026-09-30T13:45:10.999999999Z 1790775910.999 1790775911.0 false
2026-09-30T13:45:10-06:30 1790799310.0 1790799310.0 true
2026-02-30T00:00:00Z 1772409600.0 1772409600.0 true
2026-09-30T24:00:00Z 1790812800.0 1790812800.0 true
2026-09-30T23:59:60Z rejected 1790812800.0 false
```

DateProbe.swift:

```swift
import Foundation
@main struct DateProbe {
    static func main() {
        for raw in ["0000-01-01T00:00:00Z", "0001-01-01T00:00:00Z", "1582-10-10T00:00:00Z", "1970-01-01T00:00:00Z", "2026-09-30T13:45:10.123456789Z", "2026-09-30T13:45:10.999999999Z", "2026-09-30T13:45:10-06:30", "2026-02-30T00:00:00Z", "2026-09-30T24:00:00Z", "2026-09-30T23:59:60Z"] {
            let old = try? Input.date(raw)
            let style = try? Date.ISO8601FormatStyle().parse(raw)
            print(raw, old?.timeIntervalSince1970.description ?? "rejected", style?.timeIntervalSince1970.description ?? "rejected", old == style)
        }
    }
}
```

Compile and run each StrictJSON source variant with the same Contract.swift and probe:

```sh
for variant in baseline candidate; do
  swiftc -swift-version 6 apps/player/apps/native/Sources/Core/Contract.swift \
    .verification/native-timestamps/StrictJSON-$variant.swift \
    .verification/native-timestamps/DateProbe.swift \
    -o .verification/native-timestamps/date-$variant
  .verification/native-timestamps/date-$variant
done
```
