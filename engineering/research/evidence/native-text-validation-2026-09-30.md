# Native text validation performance — 2026-09-30

Repeated catalog decoding rebuilt a Unicode character set for each scalar in every validated text field. Native caption decoding did the same. Both callers now reuse one immutable set in `Input`. The set expression and acceptance rules remain identical.

This removes 93–95% of decode-and-validation time in the matched synthetic catalog workloads below. It does not establish input-to-frame latency, physical TV smoothness, or deployed performance.

## Experiment and source

Base: `84911f10718a546215efffd6144b99d469a3ef5d`. Branch: `codex/native-interaction-performance`.
Host: ARM64 Mac, macOS 27.0 (26A428), Apple Swift 6.4 (swiftlang-6.4.0.34.1). Builds use `swiftc -O -swift-version 6`.

The driver compiles the real `Contract.swift`, `StrictJSON.swift`, `Media.swift`, and `ActorContract.swift`. Its control substitutes the base revision's unmodified Contract source. The final candidate uses the pending production source. `StrictJSON` remains byte-identical. There is no replacement model or production test seam.

Nine deterministic fixtures contain 36, 60, or 200 synthetic movies. Each has IDs, titles, 2026 years, plots, ratings, genres, same-origin resource paths, progress, and one cast member. ASCII plots repeat a 57-character sentence four times. Unicode fixtures use accented, Japanese, and emoji titles with quoted multiline plots. The escaped variant encodes non-ASCII characters as JSON escapes. Cached forms pass the same tree through `JSONEncoder` with sorted keys. Every operation first checks the whole JSON tree against Foundation decoding, then checks the actual LibraryPage item count and first identity outside timing.

Each row has three rounds, one untimed warmup, and two timed decodes per round. `json` consumes the real strict decoder's item count. `library` additionally constructs the real validated LibraryPage and consumes title byte lengths. Fixture generation, file reads, reference decoding, cache encoding, and correctness checks are outside timed intervals. Runs are serial: original source, final source, then original source again. Both original runs use the same binary.

The first longer run was stopped after a CPU sample. It has no completed timings and is excluded. The first candidate experiment overlapped simulator startup and is also excluded from the final matrix. Both experiments remain in the private evidence directory. The initial sample placed 418 of 432 main-thread samples under LibraryPage construction; one hot branch rebuilt the character set. This is a standalone command-line driver thread, not the application's UI thread.

## Results

Median milliseconds per page, including strict decoding and domain validation:

| Fixture | Original before | Final | Original after | Reduction versus before |
| --- | ---: | ---: | ---: | ---: |
| 36-ascii-network | 61.796 | 3.442 | 64.040 | 94.4% |
| 36-ascii-cached | 63.724 | 3.483 | 63.924 | 94.5% |
| 36-utf8-network | 49.726 | 3.402 | 51.390 | 93.2% |
| 36-utf8-cached | 50.483 | 3.395 | 52.396 | 93.3% |
| 36-escaped-network | 51.547 | 3.315 | 51.840 | 93.6% |
| 36-escaped-cached | 51.051 | 3.345 | 51.443 | 93.4% |
| 60-ascii-network | 108.821 | 5.649 | 107.586 | 94.8% |
| 60-ascii-cached | 104.955 | 5.802 | 106.893 | 94.5% |
| 60-utf8-network | 86.353 | 5.660 | 85.407 | 93.4% |
| 60-utf8-cached | 85.774 | 5.520 | 89.215 | 93.6% |
| 60-escaped-network | 84.368 | 5.667 | 86.602 | 93.3% |
| 60-escaped-cached | 83.725 | 5.682 | 85.181 | 93.2% |
| 200-ascii-network | 350.276 | 18.876 | 359.278 | 94.6% |
| 200-ascii-cached | 354.783 | 19.039 | 358.819 | 94.6% |
| 200-utf8-network | 279.742 | 18.503 | 284.629 | 93.4% |
| 200-utf8-cached | 291.780 | 18.727 | 287.257 | 93.6% |
| 200-escaped-network | 278.985 | 18.722 | 290.680 | 93.3% |
| 200-escaped-cached | 282.744 | 18.577 | 291.302 | 93.4% |

Strict JSON parsing alone stays broadly similar; the retained matrix includes its slower samples too. The set reuse explains most of the improvement in this workload. Page revalidation still happens on every read. ServerClient catalog work runs through a service actor, so reduced actor occupancy is a plausible benefit, not measured UI presentation evidence.

The caption control compiles real `SubtitleDocument.swift` with the same original/final Contract sources. Ten and 100 nonoverlapping cues each contain two identical ASCII lines. It checks rendered text at 0.25 seconds, then times three complete parses in each of three rounds. Original/final/original medians are:

| Cues | Original before | Final | Original after |
| --- | ---: | ---: | ---: |
| 10 | 5.709 | 0.266 | 6.208 |
| 100 | 55.682 | 2.809 | 59.570 |

These gains concern caption preparation. They do not measure timed subtitle display or playback frames.

## Primary-source research

[Apple's WWDC26 profiling guidance](https://developer.apple.com/videos/play/wwdc2026/268/) distinguishes CPU cost, actor contention, and blocked threads. It recommends Release builds and comparable measurement intervals. Our host experiment identifies CPU cost; actor and presentation traces remain separate work.

[Swift's New Codable prototype announcement](https://forums.swift.org/t/new-codable-prototype-available-for-feedback/85186), March 2026, describes experimental format-specific serialization APIs. The authors report promising bespoke benchmarks while acknowledging incomplete APIs and failing tests. Its reported throughput does not predict this application's performance. Replacing the strict decoder needs separate validation, duplicate-key, input-budget, and full-pipeline evidence.

[Blaze, 2025](https://arxiv.org/html/2503.02770v1), sections 4–6, moves stable JSON Schema work into compilation. Its evaluation uses public configuration schemas, multiple validator implementations, and single-threaded Xeon measurements. It motivates removing repeated invariant work; it does not justify replacing Kinosail's domain contracts or skipping validation. This change reuses a fixed rule without introducing schema compilation or a new dependency.

[Apple's SwiftUI Instruments guidance](https://developer.apple.com/videos/play/wwdc2025/306/) supports profiling long view-body updates and tracing their causes. The remaining UI investigation must connect focus input, state publication, view work, artwork, and presentation rather than infer fluidity from this decode benchmark.

## Verification

Tests were written before the production edit. The baseline Release tvOS run passes 33 tests in ContractTests, CatalogCacheTests, and CastingSubtitleTests. Positive cases preserve Unicode and all existing newline exceptions. Negative cases reject C0, C1, format controls, missing required text, wrong types, and excessive UTF-8 byte lengths. The public catalog test proves a rejected reload cannot replace the persisted valid page after reopening the client. Caption negatives exercise the real VTT parser.

The candidate Release tvOS run passes 267 tests: 263 Swift Testing tests in 53 suites plus four XCTest cases. Xcode exits 0 and the exported xcresult reports Passed, 267 passed, zero failed, zero skipped. Its optional simulator diagnostics child stalled after the assertions completed. We stopped only that task-owned diagnostics subprocess; Xcode then exported the successful result. The incomplete diagnostics and sample remain private. No test was terminated or suppressed.

Release iOS simulator compilation passes for ARM64, with ad-hoc signing. This is compilation, not executed iOS tests or device installation. A private macOS harness also passes all 16 ContractTests on both sources. The independent review reports no findings; it ran no tests. It confirms the installed SDK declares CharacterSet Sendable and the static value is immutable.

`make max-loc`, `git diff --check`, and the Player architecture snapshot check pass. Required post-commit and hosted checks are recorded in the delivery section when available. No native UI layout changed. No new visual, remote-journey, physical-device, Nox, hitch, energy, or production-network result is claimed.

Private logs, xcresults, profiles, binaries, source overlays, fixtures, scripts, and manifests are retained in `.verification/native-interaction/` in the active leased worktree. The measured source bindings follow. Local source hashes are evidence, not credentials.

| Source | SHA-256 |
| --- | --- |
| apps/player/apps/native/Sources/Core/Contract.swift | `ef213d8ed4b4dd1fbdf320839d7493b3f7cf1865cc19656af671757fe3efd891` |
| apps/player/apps/native/Sources/Core/StrictJSON.swift | `b48a9cfb9828c11ff8bf8a8ea80705d0722b3b63e27b147bf0c476fed0af1e62` |
| apps/player/apps/native/Sources/Core/Media.swift | `5dcc5b24cd556a340383ac0c5a41d389ae4b1b2a180de512198ee7a21a8c368e` |
| apps/player/apps/native/Sources/Core/ActorContract.swift | `56798d719321f16dcefafd49829d04cb0d7cb5d7795713ff2608e728176b9af1` |
| .verification/native-interaction/Contract-baseline.swift | `280ff84cb669c8ee5a88874a60646e2c1c16070bea7156e71770365bbd28e4a9` |
| .verification/native-interaction/DecodeBenchmark.swift | `86ea5c04dfe9b621b18f7f7a4e93467f6300a95af9c66f9fc9c87d10c614eb06` |
| .verification/native-interaction/make-fixtures.py | `61cb4da35c1bd211ea9a8aa6eddd9331a1510b545e9b9b21bc73d4814a68b0ff` |
| .verification/native-interaction/SubtitleDocument-baseline.swift | `54a26d864935d2bf1267455e02bba3dac74aa75296d9b2109a064741a6248d6f` |
| .verification/native-interaction/CaptionBenchmark.swift | `672a63e5d1aae160b66e58e08bf01591ffff9241815f3e27204e81e8edd5674c` |
| apps/player/apps/native/Sources/Core/SubtitleDocument.swift | `6dd248ee73d59cfa9be5d191207b6e14677aa36296fc0d9c40707d1abea77b62` |

## Complete timed samples

Nanoseconds per decode; each cell retains rounds 0, 1, and 2. There are 324 catalog samples. None are discarded from these three completed matched runs.

| Fixture | Operation | Original before | Final | Original after |
| --- | --- | --- | --- | --- |
| 36-ascii-network | json | 1432521.0 / 1392458.5 / 1356187.5 | 2247062.5 / 1395812.5 / 1366542.0 | 1401708.0 / 1429791.5 / 1391083.0 |
| 36-ascii-network | library | 61796021.0 / 61633020.5 / 61829187.5 | 4589604.0 / 3442041.5 / 3201437.5 | 63624625.0 / 64039625.0 / 64121083.5 |
| 36-ascii-cached | json | 1364500.0 / 1365000.0 / 1449062.5 | 1660187.5 / 1439145.5 / 1339146.0 | 1410562.5 / 1424041.5 / 1386667.0 |
| 36-ascii-cached | library | 62959354.5 / 63723729.5 / 64588937.5 | 3722562.5 / 3374208.5 / 3483312.5 | 64583416.5 / 62340187.5 / 63923708.5 |
| 36-utf8-network | json | 1449979.0 / 1466542.0 / 1483416.5 | 1449791.5 / 1444396.0 / 1411250.0 | 1381771.0 / 1389583.5 / 1385333.5 |
| 36-utf8-network | library | 51643396.0 / 49524021.0 / 49726041.5 | 3475041.5 / 3402187.5 / 3279854.5 | 49094854.0 / 51390479.0 / 53096875.0 |
| 36-utf8-cached | json | 1434604.0 / 1546458.0 / 1476541.5 | 1478437.5 / 1456354.0 / 1421021.0 | 1480729.5 / 1482250.0 / 1431187.5 |
| 36-utf8-cached | library | 49061041.5 / 50483229.0 / 51899812.5 | 3456583.5 / 3288167.0 / 3394916.5 | 53308708.5 / 52396229.0 / 52280833.0 |
| 36-escaped-network | json | 1399375.0 / 1460271.0 / 1487458.5 | 1513104.0 / 1426396.0 / 1431687.5 | 1387062.5 / 1448479.0 / 1475104.5 |
| 36-escaped-network | library | 51547167.0 / 50447292.0 / 52943812.5 | 3402021.0 / 3287354.0 / 3314541.5 | 51839979.5 / 53939958.0 / 51519458.0 |
| 36-escaped-cached | json | 1415062.5 / 1486312.5 / 1524729.0 | 1470437.5 / 1439729.5 / 1431687.5 | 1414958.5 / 1397896.0 / 1406229.0 |
| 36-escaped-cached | library | 51050625.0 / 50418271.0 / 52919708.0 | 3350708.0 / 3344750.0 / 3249541.5 | 53260562.5 / 50206854.0 / 51443020.5 |
| 60-ascii-network | json | 2270375.0 / 2410145.5 / 2932520.5 | 2220687.5 / 2373812.5 / 2327958.5 | 2290958.5 / 2326458.5 / 2296354.5 |
| 60-ascii-network | library | 108821041.5 / 104249583.5 / 114310125.0 | 5648583.5 / 5629979.0 / 5700083.5 | 103766041.5 / 107586395.5 / 112744645.5 |
| 60-ascii-cached | json | 2331041.5 / 2265396.0 / 2298479.0 | 2277750.0 / 2422562.5 / 2310958.0 | 2409645.5 / 2333604.5 / 2472625.0 |
| 60-ascii-cached | library | 104098729.5 / 106842583.0 / 104955208.5 | 5801770.5 / 5811312.5 / 5659291.5 | 106893437.5 / 104080104.0 / 108642667.0 |
| 60-utf8-network | json | 2374687.5 / 2355396.0 / 2373625.0 | 2378354.5 / 2381708.0 / 2391312.5 | 2408958.0 / 2423125.0 / 2453771.0 |
| 60-utf8-network | library | 86550292.0 / 83098104.0 / 86352791.5 | 5652729.5 / 5659645.5 / 5731437.5 | 83818729.0 / 90035396.0 / 85406625.0 |
| 60-utf8-cached | json | 2408458.5 / 2565104.0 / 2386416.5 | 2379812.5 / 2409042.0 / 2359541.5 | 2529437.5 / 6465687.5 / 2388916.5 |
| 60-utf8-cached | library | 85774125.0 / 109559958.0 / 84763146.0 | 7411875.0 / 5438354.5 / 5520416.5 | 92914812.5 / 89214750.0 / 87734625.0 |
| 60-escaped-network | json | 2414667.0 / 2404687.5 / 2434625.0 | 2574416.5 / 2405042.0 / 2408958.5 | 2455479.5 / 2479708.0 / 2424916.5 |
| 60-escaped-network | library | 84009250.0 / 84368020.5 / 85131396.0 | 5986042.0 / 5588958.5 / 5667396.0 | 84505521.0 / 89546270.5 / 86601958.5 |
| 60-escaped-cached | json | 2454021.0 / 2399250.0 / 2377042.0 | 2511937.5 / 2406208.5 / 2411104.0 | 2466542.0 / 2424167.0 / 2443979.5 |
| 60-escaped-cached | library | 83725187.5 / 83327416.5 / 85788000.0 | 5702792.0 / 5682354.5 / 5560250.0 | 85374395.5 / 84084854.5 / 85180750.0 |
| 200-ascii-network | json | 7667520.5 / 7774750.0 / 7818750.0 | 7644854.5 / 7877292.0 / 7886416.5 | 7757916.5 / 7706125.0 / 7801708.5 |
| 200-ascii-network | library | 350275750.0 / 349860750.0 / 366077000.0 | 18876458.5 / 18887791.5 / 18672396.0 | 359277604.5 / 358090916.5 / 362500271.0 |
| 200-ascii-cached | json | 7760791.5 / 8057375.0 / 8124875.0 | 7686166.5 / 7571771.0 / 7626062.5 | 8047896.0 / 7729833.5 / 8040270.5 |
| 200-ascii-cached | library | 353188750.0 / 354782583.0 / 357228625.0 | 18666041.5 / 19039416.5 / 19162625.0 | 358818729.0 / 349580833.5 / 364354125.0 |
| 200-utf8-network | json | 7998583.5 / 8071604.5 / 8033562.5 | 7811916.5 / 7724187.5 / 7872208.5 | 7829500.0 / 7917395.5 / 7805208.5 |
| 200-utf8-network | library | 279741729.0 / 275754104.0 / 282212479.0 | 18386333.5 / 18834979.5 / 18503041.5 | 284647604.0 / 284628583.5 / 281362312.5 |
| 200-utf8-cached | json | 8036187.5 / 7969125.0 / 8134750.0 | 8040937.5 / 8077396.0 / 8013229.5 | 8093146.0 / 8236562.5 / 8014083.0 |
| 200-utf8-cached | library | 304128062.5 / 291780479.0 / 281906020.5 | 18745083.5 / 18696604.0 / 18727250.0 | 287257312.5 / 283470812.5 / 288132604.5 |
| 200-escaped-network | json | 8189187.5 / 8058479.5 / 8117271.0 | 8059645.5 / 7879292.0 / 7904458.5 | 8372291.5 / 7929646.0 / 8255896.0 |
| 200-escaped-network | library | 278733271.0 / 278984646.0 / 293734021.0 | 18575208.5 / 18722312.5 / 18964250.0 | 290679666.5 / 290821792.0 / 280796541.5 |
| 200-escaped-cached | json | 7914291.5 / 8033750.0 / 8061833.0 | 8079667.0 / 7869229.0 / 7881125.0 | 8169875.0 / 8232312.5 / 8051458.0 |
| 200-escaped-cached | library | 276133500.0 / 284210708.5 / 282744271.0 | 18576875.0 / 18696000.0 / 18560271.0 | 288562896.0 / 291854354.0 / 291301562.5 |

Caption samples, nanoseconds per decode:

| Phase | Cues | Rounds 0 / 1 / 2 |
| --- | ---: | --- |
| before | 10 | 8781319.333333334 / 5542611.0 / 5708541.666666667 |
| before | 100 | 55757750.0 / 55600680.666666664 / 55681791.666666664 |
| final | 10 | 317361.0 / 266263.6666666667 / 264902.6666666667 |
| final | 100 | 2792597.3333333335 / 3160958.3333333335 / 2809222.0 |
| after | 10 | 6208000.0 / 6222555.666666667 / 5631861.0 |
| after | 100 | 59569833.333333336 / 60995041.666666664 / 58868708.333333336 |

## Reproduction

From the retained worktree, run `python3 .verification/native-interaction/make-fixtures.py`. Compile each of the four Core sources plus `.verification/native-interaction/DecodeBenchmark.swift` with `xcrun swiftc -O -swift-version 6`. For the original control substitute `.verification/native-interaction/Contract-baseline.swift`; leave the other Core sources unchanged. Run each binary with `.verification/native-interaction 3 2`. The exact commands, source bindings, timestamps, exit codes, and checksums are in `controls-manifest.json` and the three logs. The following retained source makes fixture generation and the measurement loop reviewable.

Fixture generator:

```python
import json, pathlib
root = pathlib.Path(__file__).parent
for count in (36, 60, 200):
    items = []
    for i in range(count):
        ident = f'movie-{i:03}'
        items.append(dict(id=ident, kind='video', title=f'Night Sky {i}', sortTitle=f'Night Sky {i}', year='2026',
                          plot='A family follows the coast through a changing landscape. ' * 4,
                          rating='PG', genres='Adventure, Drama', artist='', album='', show='', showId='',
                          season=0, episode=0, artwork=f'/art/{ident}', backdrop=f'/backdrop/{ident}',
                          container='mp4', stream=f'/media/{ident}', download=f'/download/{ident}', size=32000000,
                          progress=dict(seconds=3.5,watched=False,dismissed=False,session='synthetic-session',revision=1),
                          cast=[dict(name='Alex Example',role='Lead',image='')], track=0, subtitles=0))
    for encoding in ('ascii', 'utf8', 'escaped'):
        if encoding != 'ascii':
            for i, item in enumerate(items):
                item['title'] = f'Étoiles 日本語 🌌 {i}'
                item['plot'] = 'A quoted "journey" across the café.\n' * 4
        page = dict(items=items, total=count, offset=0, limit=count, letters=[],view='movies',sort='title',query='',letter='')
        (root/f'{count}-{encoding}.json').write_bytes(json.dumps(page, ensure_ascii=encoding=='escaped',separators=(',',':')).encode())
```

Catalog driver:

```swift
import Foundation

@main struct DecodeBenchmark {
    static func main() throws {
        let root = URL(fileURLWithPath: CommandLine.arguments[1])
        let rounds = Int(CommandLine.arguments[2])!
        let iterations = Int(CommandLine.arguments[3])!
        let server = try ServerAddress("https://benchmark.example.invalid")
        var checksum = 0
        print("round,fixture,bytes,operation,iterations,nanoseconds_per_decode"); fflush(stdout)
        for round in 0..<rounds {
            for count in [36, 60, 200] {
                for encoding in ["ascii", "utf8", "escaped"] {
                    let name = "\(count)-\(encoding)"
                    let original = try Data(contentsOf: root.appendingPathComponent(name + ".json"))
                    let expected = try JSONDecoder().decode(JSONValue.self, from: original)
                    let encoder = JSONEncoder(); encoder.outputFormatting = [.sortedKeys]
                    let cached = try encoder.encode(expected)
                    for (form, data) in [("network", original), ("cached", cached)] {
                        guard try StrictJSON.decode(data) == expected else { fatalError("JSON mismatch") }
                        let typed = try LibraryPage(expected, server: server)
                        guard typed.items.count == count, typed.items.first?.id == "movie-000" else { fatalError("contract mismatch") }
                        for operation in ["json", "library"] {
                            for _ in 0..<1 { checksum += try consume(data, operation, server) }
                            let start = DispatchTime.now().uptimeNanoseconds
                            for _ in 0..<iterations { checksum += try consume(data, operation, server) }
                            let elapsed = DispatchTime.now().uptimeNanoseconds - start
                            print("\(round),\(name)-\(form),\(data.count),\(operation),\(iterations),\(Double(elapsed) / Double(iterations))"); fflush(stdout)
                        }
                    }
                }
            }
        }
        fputs("checksum=\(checksum)\n", stderr)
    }
    @inline(never) static func consume(_ data: Data, _ operation: String, _ server: ServerAddress) throws -> Int {
        let value = try StrictJSON.decode(data)
        if operation == "library" { return try LibraryPage(value, server: server).items.reduce(0) { $0 + $1.title.utf8.count } }
        guard case .object(let object) = value, case .array(let items) = object["items"] else { fatalError() }
        return items.count
    }
}
```

Caption driver:

```swift
import Foundation
@main struct CaptionBenchmark {
    static func main() throws {
        print("round,cues,iterations,nanoseconds_per_decode")
        let line = "A caption follows the quiet coast through a changing landscape."
        for round in 0..<3 {
            for count in [10, 100] {
                var vtt = "WEBVTT\n\n"
                for i in 0..<count {
                    let start = String(format: "00:%02d:%02d.000", i / 60, i % 60)
                    let end = String(format: "00:%02d:%02d.500", i / 60, i % 60)
                    vtt += "\(start) --> \(end)\n\(line)\n\(line)\n\n"
                }
                let data = Data(vtt.utf8)
                guard try SubtitleDocument(data: data).text(at: 0.25) == line + "\n" + line else { fatalError("caption mismatch") }
                let start = DispatchTime.now().uptimeNanoseconds
                var checksum = 0
                for _ in 0..<3 { checksum += try SubtitleDocument(data: data).text(at: 0.25).utf8.count }
                let elapsed = DispatchTime.now().uptimeNanoseconds - start
                guard checksum == 3 * (2 * line.utf8.count + 1) else { fatalError("caption checksum") }
                print("\(round),\(count),3,\(Double(elapsed)/3)")
            }
        }
    }
}
```

## Delivery

[PR #397](https://github.com/Kinosail/kinosail/pull/397) contains the native change, its tests, and these findings. Initial task commit: `f81ea642b97f072d6c1d58ab052c2f23b9ffe445`.

The post-commit Player gate passes max-loc, diff-check, and both native simulator builds in 59 seconds. Its first attempt is invalid: ENOSPC prevented shell check selection despite exit 0. The complete error log is retained. Only task-owned rebuildable compiler caches were removed; the unchanged-source retry passes. Native Products, source, profiles, logs, and xcresults remain. Default gitleaks scans the introduced commit and reports no leaks. Native copy validation passes.

Main reconciliation includes `2f8f8e140` (the Android photo overlay PR). The measured native Core sources remain byte-identical. No native source or test conflict occurs. Current hosted checks and protected-main delivery are recorded in [PR #397](https://github.com/Kinosail/kinosail/pull/397). The broader performance goal remains active.
