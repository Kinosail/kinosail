# Performance frontiers: measurement record

## Web rendering and delayed placeholders

Measured September 30, 2026. Baseline source: `964a60c7e61df208dbb4ce704d68f06a8b08b412`. The compiled comparison includes the shared placeholder delay and navigation bundle refresh. No production network or physical-device gain is claimed.

The real Go HTTP adapter served 10,000 synthetic video titles with local NFO metadata and 64 generated 600 × 900 JPEG posters. Each title uses a hard link to a generated local MP4. A private test holder injects the fixture Owner cookie. Real authentication middleware still executes; profile sign-in latency is excluded. There is no user media. FFmpeg, media probing, playback, and TLS are outside this fixture.

Chromium 154 ran in an isolated browser context on the shared macOS host. Dark mode, no network throttling, and a 4× CPU slowdown were used for interaction comparisons. The mobile viewport was 390 × 844 at device pixel ratio 3. Desktop was 1440 × 900 at ratio 1. The slowdown is not a calibrated physical-phone or TV simulation.

Trusted browser drag input selects letters on the actual title index. Each comparison contains six scrubs. DevTools reports one interaction per soft navigation. Their medians are descriptive lab statistics, not the field 75th percentile or a complete session INP score. Initial pages and static assets were warm. Artwork responses use `no-store`; later runs can still benefit from OS, decoder, and host warm-up.

| Condition | Letters | Observed interaction samples, ms | Median, ms | Soft-navigation LCP samples, ms |
| --- | --- | --- | --- | --- |
| Original compiled app | B–G | 100, 76, 85, 82, 84, 84 | 84 | 233, 226, 226, 232, 234, 226 |
| Browser prototype: postpone only the skeleton class by 120 ms | H–M | 51, 41, 42, 44, 43, 44 | 43.5 | 109, 116, 117, 119, 118, 119 |
| Restore original class behavior in the same browser | N–S | 84, 86, 84, 77, 85, 77 | 84 | 234, 227, 234, 227, 235, 218 |
| Compiled fix, restarted Go server, same generated files | B–G | 58, 43, 44, 43, 51, 49 | 46.5 | 142, 118, 111, 109, 117, 141 |

All listed navigations measured CLS 0.00. The compiled median is about 45% lower than the first compiled control. This is one shared-host session with sequential conditions, not a randomized trial or a confidence interval. The reverse control supports the rendering diagnosis. A first isolated scrub measured 285 ms, including 268 ms presentation delay. The repeated controls did not reproduce that result; it is not included in the table.

Separate observations:

- A warm desktop reload at normal CPU rate measured LCP 237 ms and CLS 0.00. A later reload of the changed app at 4× CPU rate measured 344 ms and CLS 0.00. These conditions differ and are not a speed comparison.
- Fifteen trusted End presses loaded 100 cards per page, followed by Home. The original app reached 1,600 cards and 5,222 DOM elements. DevTools reported observed interaction latency 52 ms and CLS 0.00 at 4× CPU rate. The trace includes idle gaps between automation calls.
- The scroll trace reported 15 long tasks of 53–75 ms. Long Animation Frames entries attribute some work to the main bundle and subsequent rendering. Their durations exclude final presentation; they are not physical display hitch measurements. See the [API documentation](https://developer.chrome.com/docs/web-platform/long-animation-frames).
- DevTools estimated 18 MB of image waste across that traversal. This is an optimization estimate, not measured saved traffic. Original 600-pixel images rendered at about 163 CSS pixels on desktop and 153 pixels at ratio 3 on mobile. Any responsive derivative must preserve visual quality, authorization, bounded decoding, and freshness.
- Retained detached DOMParser and template image probes caused no observed image requests. No parser replacement was made on that unproven hypothesis.

The implementation keeps immediate `aria-busy` and inert content. One timer belongs to each pending target state. It survives overlapping requests, clears on final completion, and checks target ownership and connection before displaying placeholders. Request dispatch, stale-response rejection, focus restoration, and error recovery remain in their existing owners. The 120 ms value is a local product choice; the [INP guide](https://web.dev/articles/optimize-inp) supports reducing avoidable event and presentation work, not this exact threshold.

The test holder ran with `KINOSAIL_WEB_PERFORMANCE_FIXTURE=<private fixture root> go test ./internal/server -run '^TestHoldWebPerformanceFixture$' -count=1 -timeout=45m -v`. It was stopped through its owned stop file. Its source was copied into the ignored evidence directory and removed from the production checkout. The browser page was closed after recording.

SHA-256 source bindings:

| Input | SHA-256 |
| --- | --- |
| Baseline `packages/webassets/static/pwa.js` | `43e54cc125735b6e2a224873ce7b938faa2555fa96edace972cd6a4b6415a769` |
| Compiled changed `packages/webassets/static/pwa.js` | `4ea43e5431973c9f6a6f7923248fcb63d7f0b401e44724539c0289b6ae1c0762` |
| Synthetic generator | `f41d432cb36d8b7982c4ce25d8a4be1c9a1a2b30a44c3bf9a7ad9651aafecb4d` |
| Private holder source | `56acbf08f024d0053fdc6ac03ca051b8121fb1446587446693e2356290e5c963` |
| Fixture manifest | `ccba90d86a0f8602ab1de98e12d4e4f68f04118cc35f5e1c7034b1cc89a65f82` |
| Final shared E2E helper | `2caaf5dbfa46b957f4cea88965608fd872a1e755e551a91abbb73c19393c9710` |

Private evidence is retained under `.verification/web-render-profile`: generator, fixture manifest, holder source, DevTools summaries, Event Timing and Long Animation Frames entries, compiled comparison, and red regression artifacts. Explicit raw-trace file export was rejected by the DevTools workspace-root configuration. Managed trace analysis succeeded; no standalone raw trace export is claimed.

Validation records:

- Before the production fix, the delayed-placeholder E2E failed because the current library immediately acquired `request-skeleton`. Its trace and screenshot are retained separately.
- Player and Subtitles each passed 72 expanded HTMX cases across Chromium, Firefox, and WebKit. Three overlap cases per app initially expected an older successful response to replace a newer failed request. The existing generation guard correctly preserved the original content. After correcting that test expectation, all six affected cases passed on recheck. This verifies 150 cases across both apps, including 390, 1440, and 1920 pixel pending, loaded, empty, and failed views; aborts; timeouts; fast completion; and overlaps.
- Commands: `KINOSAIL_BROWSER_MATRIX=full pnpm test htmx-migration.spec.ts --workers=1`, then the same command with `--grep 'delays visual placeholders'` and separate output directories. Each E2E attaches revision, diff hash, browser, command, fixture description, and HTMX hash.
- Focused Go navigation-bundle tests passed in both apps. Shared Go tests passed. Browser script lint reports zero errors and warnings. The UI detector reports no findings. Source-file caps passed.
- The repository TypeScript type-policy checker stopped because TypeScript 7.0.2 exposes no `ScriptTarget.Latest` through the API the checker uses. This is an existing tooling compatibility limit; the dependency and checker were not changed.
- Full app Go suites, root tooling checks, changed-app gates, and hosted delivery are recorded separately when complete.

Hardware focus timing, Safari device performance, production networks, scan/download/transcode interference, artwork derivatives, and deployed first frame remain open. The mobile 10,000-item fixture also exposes existing count/Sort label crowding; that layout issue is outside this rendering patch.

## Earlier catalog and native measurements

Synthetic data only. Source hashes bind the native results to the validated working tree. [Hosted verification](https://github.com/Kinosail/kinosail/pull/369/checks) is recorded independently.

```json
{
  "starting_revision": "9a45e9444613dbde5a277a8c7426919fcab97aa8",
  "measurement_revision": "starting revision plus the compact collation scratch-storage change",
  "environment": {
    "os": "macOS",
    "arch": "arm64",
    "cpu": "Apple M1 Pro",
    "notes": "Other native builds ran concurrently. Timings are host-dependent; no physical-device gain is claimed."
  },
  "fixture": {
    "titles": 10000,
    "names": "Movie 0 through Movie 9999",
    "kind": "video",
    "year": "2026",
    "profile": "synthetic owner",
    "page_size": 100,
    "response_bytes": 25319
  },
  "baseline": {
    "command": "go test ./internal/server -run '^$' -bench '^BenchmarkLargeLibraryBrowse$' -benchtime=2s -count=5 -cpuprofile=/tmp/kinosail-frontiers-before.cpu -memprofile=/tmp/kinosail-frontiers-before.mem",
    "result": "goos: darwin\ngoarch: arm64\npkg: github.com/MikeO7/kinosail-player/internal/server\ncpu: Apple M1 Pro\nBenchmarkLargeLibraryBrowse-10    \t     343\t   6782572 ns/op\t     25319 response-bytes\t 4708751 B/op\t   11590 allocs/op\nBenchmarkLargeLibraryBrowse-10    \t     348\t   6782943 ns/op\t     25319 response-bytes\t 4708623 B/op\t   11590 allocs/op\nBenchmarkLargeLibraryBrowse-10    \t     334\t   6930226 ns/op\t     25319 response-bytes\t 4708596 B/op\t   11590 allocs/op\nBenchmarkLargeLibraryBrowse-10    \t     342\t   7054822 ns/op\t     25319 response-bytes\t 4708527 B/op\t   11590 allocs/op\nBenchmarkLargeLibraryBrowse-10    \t     376\t   6445994 ns/op\t     25319 response-bytes\t 4708575 B/op\t   11590 allocs/op\nPASS\nok  \tgithub.com/MikeO7/kinosail-player/internal/server\t17.355s\n"
  },
  "compact_keys": {
    "command": "go test ./internal/server -run '^$' -bench '^BenchmarkLargeLibraryBrowse$' -benchtime=2s -count=5",
    "result": "goos: darwin\ngoarch: arm64\npkg: github.com/MikeO7/kinosail-player/internal/server\ncpu: Apple M1 Pro\nBenchmarkLargeLibraryBrowse-10    \t     345\t   6804355 ns/op\t     25319 response-bytes\t 2829872 B/op\t   11573 allocs/op\nBenchmarkLargeLibraryBrowse-10    \t     327\t   7039870 ns/op\t     25319 response-bytes\t 2829917 B/op\t   11573 allocs/op\nBenchmarkLargeLibraryBrowse-10    \t     338\t   6787313 ns/op\t     25319 response-bytes\t 2829878 B/op\t   11573 allocs/op\nBenchmarkLargeLibraryBrowse-10    \t     370\t   6628020 ns/op\t     25319 response-bytes\t 2829840 B/op\t   11573 allocs/op\nBenchmarkLargeLibraryBrowse-10    \t     354\t   6894637 ns/op\t     25319 response-bytes\t 2829813 B/op\t   11573 allocs/op\nPASS\nok  \tgithub.com/MikeO7/kinosail-player/internal/server\t17.165s\n",
    "source_sha256": "0bcb4d6bb1a9617c8922cbe0a3d1d32fef34e32d3fc82bb3dbf7a9c9926dc3bb"
  },
  "native_fixture": {
    "network": "URLProtocol controlled responses and held requests, no user server",
    "images": "800, 1600 and 4096 pixel squares generated in memory",
    "cache": "private temporary disk cache; 32 MiB decoded-image budget",
    "device": "dedicated Apple TV 1080p simulator",
    "os": "tvOS 27.0",
    "configuration": "Debug correctness tests; not release performance traces"
  },
  "native_red_controls": [
    {
      "stage": "original LRU admission",
      "result": "Displayed image was evicted and requested twice; promoted image lost identity."
    },
    {
      "stage": "refresh growth",
      "result": "Three previously displayed images lost identity after a larger background refresh."
    },
    {
      "stage": "URL-only foreground demand and retained old replacement",
      "result": "Held 800px demand promoted speculative 1600px disk pixels; three displayed identities changed. Next foreground refresh remained 800px after fresh 1600px disk bytes."
    },
    {
      "stage": "replacement above retention budget",
      "result": "4096px refresh retained obsolete 800px pixels. The 1600px case passed."
    }
  ],
  "verification": {
    "shared_packages": "go test ./... passed",
    "catalog_race": "go test -race ./catalog -count=1 passed",
    "large_library_regressions": "go test ./internal/server -run 'TestLarge(Library|Letter)' -count=1 passed",
    "max_loc": "make max-loc passed",
    "tooling": "make tooling-check passed",
    "native_final": "26 tests in five suites passed; refresh growth exercised 1600px and 4096px cases",
    "player_full": "go test ./... passed",
    "apple_builds": "iOS and tvOS simulator builds passed again after reconciling main 0b0fda9f1c3030b80ecd09c099f5add64403c05f",
    "required_hosted_checks": {
      "url": "https://github.com/Kinosail/kinosail/pull/369/checks",
      "note": "The pull request is the authoritative source for current hosted results; this measurement record does not pin a completion claim."
    },
    "subtitles_full": "go test ./... passed",
    "changed_code_lint": "golangci-lint run --new-from-rev=9a45e9444613dbde5a277a8c7426919fcab97aa8: 0 issues",
    "full_local_changed_gate": "Both Player and Subtitles verify-changed stop at 112 existing shared lint findings outside this patch. Three task findings were fixed; changed-code lint reports zero issues."
  },
  "boundaries": [
    "Physical iPhone and Apple TV frame timing",
    "Full local browser matrix",
    "Deployed first-frame timings",
    "Production network performance"
  ],
  "native_command": "xcodebuild -project Kinosail.xcodeproj -scheme Kinosail-tvOS -destination 'platform=tvOS Simulator,id=64468BFE-9114-4FBF-860B-79DF45F70011' -derivedDataPath .build/speed-tvos -jobs 4 -parallel-testing-enabled NO -only-testing:Kinosail-tvOSTests/ArtworkAdmissionTests -only-testing:Kinosail-tvOSTests/ArtworkLoaderTests -only-testing:Kinosail-tvOSTests/ArtworkCancellationTests -only-testing:Kinosail-tvOSTests/ArtworkSharingTests -only-testing:Kinosail-tvOSTests/ArtworkPrefetchTests -resultBundlePath .verification/frontiers-artwork-delivery.xcresult CODE_SIGNING_ALLOWED=YES CODE_SIGN_IDENTITY=- test",
  "native_source_sha256": {
    "apps/player/apps/native/Sources/Platform/ArtworkLoader.swift": "2c71b58d54284ae8c68ad8d7265739cf0f334fde66b96f28c38090ebbb8608aa",
    "apps/player/apps/native/Tests/ArtworkAdmissionTests.swift": "0a7a459c3c9e8c22c604d72d9c8fefc0fb4d6415357633b465727dc37d0c1d5f",
    "apps/player/apps/native/Tests/ArtworkFixtureImages.swift": "6b1211c0b73d2b3fbb210d9919e9f442ce985f50c952893b662bbacb129c8fad",
    "apps/player/apps/native/Tests/ArtworkLoaderTests.swift": "596de51884ab26b3c2b0cdfa500361ffbced85d9a067b9ef68ecaed67122b79c"
  },
  "reconciliation": {
    "origin_main": "0b0fda9f1c3030b80ecd09c099f5add64403c05f",
    "integration_revision": "3140f3be2edca4c3d897b0137f7f4632c3c53779",
    "native_artwork": "26 tests in five suites passed; 1600px and 4096px refresh cases exercised",
    "tooling": "make tooling-check passed again after merging current main"
  },
  "delivery_source_sha256": {
    "packages/catalog/browse_sort.go": "d783d39f6a1ddbdd61b1335910288ad818dd561352479b9b03d740643aefcb9f",
    "packages/catalog/sort_storage_test.go": "7fb6a37b83d21d7e8a92ac4cb9dcb995b3e61e800e9a9a64d1981fb2ccfe9147"
  },
  "formatting_note": "Collation measurement preceded gofumpt formatting. Go statements are unchanged; delivery hashes record the formatted source.",
  "known_title_navigation": {
    "command": "go test ./internal/server -run '^$' -bench '^BenchmarkLargeLibraryKnownTitleNavigation$' -benchtime=2s -count=3",
    "result": "goos: darwin\ngoarch: arm64\npkg: github.com/MikeO7/kinosail-player/internal/server\ncpu: Apple M1 Pro\nBenchmarkLargeLibraryKnownTitleNavigation/search-10         \t      78\t  36211266 ns/op\t 1877917 B/op\t   28964 allocs/op\nBenchmarkLargeLibraryKnownTitleNavigation/search-10         \t     100\t  38964268 ns/op\t 1877753 B/op\t   28962 allocs/op\nBenchmarkLargeLibraryKnownTitleNavigation/search-10         \t     159\t  18520208 ns/op\t 1877731 B/op\t   28962 allocs/op\nBenchmarkLargeLibraryKnownTitleNavigation/letter-10         \t      19\t 125337099 ns/op\t 2831106 B/op\t   11588 allocs/op\nBenchmarkLargeLibraryKnownTitleNavigation/letter-10         \t      18\t 114844516 ns/op\t 2830856 B/op\t   11588 allocs/op\nBenchmarkLargeLibraryKnownTitleNavigation/letter-10         \t      44\t  61831955 ns/op\t 2830873 B/op\t   11588 allocs/op\nPASS\nok  \tgithub.com/MikeO7/kinosail-player/internal/server\t61.077s\n",
    "note": "Concurrent host builds severely affect wall time. These are follow-up workload measurements, not a before/after speed comparison."
  },
  "physical_device_probe": {
    "models": [
      "iPhone 16 Pro Max",
      "Apple TV 4K (third generation)"
    ],
    "read_only_inventory": "Paired devices responded to process inventory queries.",
    "record_command": "xcrun xctrace record --template SwiftUI --device <private device identifier> --attach <observed app PID> --time-limit 30s --output <ignored private trace path> --no-prompt",
    "record_result": "Exit 21: Instruments could not find the observed phone process. No trace was created. The TV app was not running.",
    "boundary": "No installation, launch, termination, or interaction journey was performed. Installed source revision and physical frame timing remain unverified."
  },
  "catalog_projection": {
    "baseline_revision": "5c75b6e0ccddb2addefe73247d103e2144662c59",
    "fixture": "10,000 and 100,000 synthetic Movie titles, video kind, year 2026; owner viewer context; no user media or paths.",
    "web_command": "go test ./internal/server -run '^$' -bench '^(BenchmarkLargeLibraryBrowse|BenchmarkLargeLibraryKnownTitleNavigation)$' -benchtime=2s -count=3",
    "api_command": "go test ./internal/server -run '^$' -bench '^BenchmarkNativeCatalogNavigation$' -benchtime=1s -count=3",
    "environment": "macOS Darwin arm64, Apple M1 Pro, shared host. A probe observed three xcodebuild processes and load averages 30.69/30.78/34.69.",
    "web_before": "goos: darwin\ngoarch: arm64\npkg: github.com/MikeO7/kinosail-player/internal/server\ncpu: Apple M1 Pro\nBenchmarkLargeLibraryBrowse-10                  \t     193\t  11916899 ns/op\t     25319 response-bytes\t 2829949 B/op\t   11574 allocs/op\nBenchmarkLargeLibraryBrowse-10                  \t     230\t  11705988 ns/op\t     25319 response-bytes\t 2829924 B/op\t   11573 allocs/op\nBenchmarkLargeLibraryBrowse-10                  \t     142\t  15004129 ns/op\t     25319 response-bytes\t 2829895 B/op\t   11573 allocs/op\nBenchmarkLargeLibraryKnownTitleNavigation/search-10         \t     342\t   6937268 ns/op\t 1877736 B/op\t   28962 allocs/op\nBenchmarkLargeLibraryKnownTitleNavigation/search-10         \t     368\t   6707203 ns/op\t 1877740 B/op\t   28962 allocs/op\nBenchmarkLargeLibraryKnownTitleNavigation/search-10         \t     307\t   6681268 ns/op\t 1877682 B/op\t   28962 allocs/op\nBenchmarkLargeLibraryKnownTitleNavigation/letter-10         \t     258\t  10249557 ns/op\t 2830612 B/op\t   11588 allocs/op\nBenchmarkLargeLibraryKnownTitleNavigation/letter-10         \t     100\t  21545300 ns/op\t 2830777 B/op\t   11588 allocs/op\nBenchmarkLargeLibraryKnownTitleNavigation/letter-10         \t     100\t  23603113 ns/op\t 2830618 B/op\t   11588 allocs/op\nPASS\nok  \tgithub.com/MikeO7/kinosail-player/internal/server\t29.764s\n",
    "web_after": "goos: darwin\ngoarch: arm64\npkg: github.com/MikeO7/kinosail-player/internal/server\ncpu: Apple M1 Pro\nBenchmarkLargeLibraryBrowse-10                  \t     100\t  21717987 ns/op\t     25319 response-bytes\t 2428740 B/op\t   11574 allocs/op\nBenchmarkLargeLibraryBrowse-10                  \t      80\t  32284103 ns/op\t     25319 response-bytes\t 2428477 B/op\t   11572 allocs/op\nBenchmarkLargeLibraryBrowse-10                  \t      73\t  44479411 ns/op\t     25319 response-bytes\t 2428472 B/op\t   11572 allocs/op\nBenchmarkLargeLibraryKnownTitleNavigation/search-10         \t     100\t  25007155 ns/op\t 1476293 B/op\t   28961 allocs/op\nBenchmarkLargeLibraryKnownTitleNavigation/search-10         \t     100\t  32951698 ns/op\t 1476242 B/op\t   28961 allocs/op\nBenchmarkLargeLibraryKnownTitleNavigation/search-10         \t     100\t  25464912 ns/op\t 1476190 B/op\t   28961 allocs/op\nBenchmarkLargeLibraryKnownTitleNavigation/letter-10         \t      93\t  25049496 ns/op\t 2429166 B/op\t   11587 allocs/op\nBenchmarkLargeLibraryKnownTitleNavigation/letter-10         \t      63\t  32129001 ns/op\t 2429266 B/op\t   11587 allocs/op\nBenchmarkLargeLibraryKnownTitleNavigation/letter-10         \t     105\t  21513829 ns/op\t 2429255 B/op\t   11587 allocs/op\nPASS\nok  \tgithub.com/MikeO7/kinosail-player/internal/server\t46.514s\n",
    "api_before": "goos: darwin\ngoarch: arm64\npkg: github.com/MikeO7/kinosail-player/internal/server\ncpu: Apple M1 Pro\nBenchmarkNativeCatalogNavigation/10000/browse-10  \t     104\t  11378522 ns/op\t     19286 response-bytes\t 2170022 B/op\t     488 allocs/op\nBenchmarkNativeCatalogNavigation/10000/browse-10  \t     100\t  11483078 ns/op\t     19286 response-bytes\t 2170748 B/op\t     486 allocs/op\nBenchmarkNativeCatalogNavigation/10000/browse-10  \t      84\t  15870611 ns/op\t     19286 response-bytes\t 2171375 B/op\t     486 allocs/op\nBenchmarkNativeCatalogNavigation/10000/search-10  \t      96\t  10420548 ns/op\t       316.0 response-bytes\t 1376270 B/op\t   20082 allocs/op\nBenchmarkNativeCatalogNavigation/10000/search-10  \t     121\t   9315092 ns/op\t       316.0 response-bytes\t 1376231 B/op\t   20082 allocs/op\nBenchmarkNativeCatalogNavigation/10000/search-10  \t     178\t   7939916 ns/op\t       316.0 response-bytes\t 1376283 B/op\t   20082 allocs/op\nBenchmarkNativeCatalogNavigation/100000/browse-10 \t       6\t 535071583 ns/op\t     19668 response-bytes\t19481306 B/op\t     493 allocs/op\nBenchmarkNativeCatalogNavigation/100000/browse-10 \t       3\t 345875847 ns/op\t     19668 response-bytes\t19453362 B/op\t     486 allocs/op\nBenchmarkNativeCatalogNavigation/100000/browse-10 \t       2\t 718013896 ns/op\t     19668 response-bytes\t19539112 B/op\t     511 allocs/op\nBenchmarkNativeCatalogNavigation/100000/search-10 \t       3\t 363322736 ns/op\t       321.0 response-bytes\t13627570 B/op\t  200087 allocs/op\nBenchmarkNativeCatalogNavigation/100000/search-10 \t       4\t 264100260 ns/op\t       321.0 response-bytes\t13627202 B/op\t  200086 allocs/op\nBenchmarkNativeCatalogNavigation/100000/search-10 \t       7\t 155078286 ns/op\t       321.0 response-bytes\t13626950 B/op\t  200086 allocs/op\nPASS\nok  \tgithub.com/MikeO7/kinosail-player/internal/server\t35.435s\n",
    "api_after": "goos: darwin\ngoarch: arm64\npkg: github.com/MikeO7/kinosail-player/internal/server\ncpu: Apple M1 Pro\nBenchmarkNativeCatalogNavigation/10000/browse-10  \t      72\t  14994126 ns/op\t     19286 response-bytes\t 1769791 B/op\t     489 allocs/op\nBenchmarkNativeCatalogNavigation/10000/browse-10  \t      81\t  13261388 ns/op\t     19286 response-bytes\t 1765907 B/op\t     484 allocs/op\nBenchmarkNativeCatalogNavigation/10000/browse-10  \t     100\t  12674189 ns/op\t     19286 response-bytes\t 1767606 B/op\t     484 allocs/op\nBenchmarkNativeCatalogNavigation/10000/search-10  \t     157\t   8340358 ns/op\t       316.0 response-bytes\t  974831 B/op\t   20081 allocs/op\nBenchmarkNativeCatalogNavigation/10000/search-10  \t     126\t   9390310 ns/op\t       316.0 response-bytes\t  974847 B/op\t   20081 allocs/op\nBenchmarkNativeCatalogNavigation/10000/search-10  \t     100\t  12065837 ns/op\t       316.0 response-bytes\t  974829 B/op\t   20081 allocs/op\nBenchmarkNativeCatalogNavigation/100000/browse-10 \t       7\t 152632167 ns/op\t     19668 response-bytes\t15459104 B/op\t     488 allocs/op\nBenchmarkNativeCatalogNavigation/100000/browse-10 \t       6\t 203732820 ns/op\t     19668 response-bytes\t15475416 B/op\t     492 allocs/op\nBenchmarkNativeCatalogNavigation/100000/browse-10 \t       6\t 190936750 ns/op\t     19668 response-bytes\t15475416 B/op\t     492 allocs/op\nBenchmarkNativeCatalogNavigation/100000/search-10 \t       9\t 112314982 ns/op\t       321.0 response-bytes\t 9620583 B/op\t  200081 allocs/op\nBenchmarkNativeCatalogNavigation/100000/search-10 \t      13\t  84322003 ns/op\t       321.0 response-bytes\t 9620788 B/op\t  200084 allocs/op\nBenchmarkNativeCatalogNavigation/100000/search-10 \t      12\t  99902642 ns/op\t       321.0 response-bytes\t 9620446 B/op\t  200081 allocs/op\nPASS\nok  \tgithub.com/MikeO7/kinosail-player/internal/server\t29.708s\n",
    "source_sha256": {
      "packages/catalog/browse_apply.go": "40e48c06ead5262347a34a65f0a01a8d3febc825b3ae64bd5fb9c7aa19ba6ec7",
      "packages/catalog/catalog_test.go": "10a6a8f7e427db2f72a8115ffa4f1a4d63ab056646e98a1a97cafb1522f5e2a3",
      "apps/player/internal/server/performance_benchmark_test.go": "25e23c79c788868df664d59d9face0667dc0302efa39ef8cea8fd2422aa7f306",
      "apps/player/internal/server/catalog_performance_benchmark_test.go": "45f459da7aa67fd373ed08ac620af35e912f69eadbefcddc23c357162db1506d"
    },
    "control": "The new caller-input assertion passes on baseline. An intentional unsafe in-place filter without the public Apply copy fails with Apply changed caller-owned candidates. The ownership repair and catalog suite pass.",
    "checks": {
      "catalog": "go test ./catalog -count=1 passed",
      "catalog_race": "go test -race ./catalog -count=1 passed",
      "player_browse_contract": "Eight focused public HTTP/API pagination, authentication, invalid-query, and large-page tests passed.",
      "subtitles_browse_contract": "Six focused public HTTP/API pagination, authentication, and invalid-query tests passed.",
      "packages_full": "go test ./... in packages passed",
      "source_cap": "make max-loc passed",
      "hosted": "The delivery report records the pull request and hosted checks separately. This local measurement does not pin their completion.",
      "changed_code_lint": "golangci-lint run --new-from-rev=5c75b6e0ccddb2addefe73247d103e2144662c59: zero issues",
      "independent_review": "No findings; reviewer independently ran catalog tests and diff checks.",
      "player_contract_command": "go test ./internal/server -run '^(TestLargeLibraryWebResponseIsBounded|TestLargeLetterBucketRemainsBoundedAndPageable|TestClientCanAuthenticateAndBrowseLibraryWithoutFilesystemPaths|TestClientLibraryRequiresAuthentication|TestLibraryPaginationIsSharedByAPIAndWeb|TestDeepLibraryPageDoesNotRepeatHomeShelves|TestInfiniteLibraryPageReturnsOnlyTheBoundedFragment|TestLibraryPaginationRejectsAmbiguousAndOutOfRangeInput)$' -count=1",
      "subtitles_contract_command": "go test ./internal/server -run '^(TestClientCanAuthenticateAndBrowseLibraryWithoutFilesystemPaths|TestClientLibraryRequiresAuthentication|TestLibraryPaginationIsSharedByAPIAndWeb|TestDeepLibraryPageDoesNotRepeatHomeShelves|TestInfiniteLibraryPageReturnsOnlyTheBoundedFragment|TestLibraryPaginationRejectsAmbiguousAndOutOfRangeInput)$' -count=1"
    },
    "boundary": "Handler benchmarks include real browse and encoding/rendering operations, with synthetic fixtures and injected owner context. They exclude network, auth middleware, device decoding, and presentation. Byte reductions are measured; wall-clock latency is not established on this loaded host. Heap profiles also include initialization costs."
  },
  "native_release_navigation": {
    "revision": "eda10c42461d9985e868b0d03536e3ecc6c57c1f",
    "environment": {
      "host": "macOS 27.0, arm64 Apple M1 Pro",
      "xcode": "27.0 (27A266a)",
      "target": "Dedicated Apple TV 1080p simulator, tvOS 27.0 (24J360)",
      "configuration": "Release, -O, ENABLE_TESTABILITY=YES, arm64"
    },
    "fixture": {
      "transport": "Loopback HTTP only, synthetic saved session, private disposable fixture",
      "movies": 160,
      "artwork": "Generated 1600x900 landscape, 600x900 poster and 900x900 square JPEGs; 100ms response delay",
      "intentional_failure": "movie-005 artwork returns 404; its fallback is expected",
      "python": "Bundled runtime with Pillow 12.3.0",
      "sha256": {
        "server.py": "9dcfb092f29e9ddd4ea29c1527a465027efcd650101f76864c3314a13fb8b338",
        "remote-diagnostic.swift": "2a4dc8bd04a58180f11d0d4d1f12f7003b619bcdb880803c8b756885499bf2b5",
        "seed.swift": "16c136e8ae4fbc76b952932e3e4b3a2a9a7c8e8d42362456b81ec46518f7fb20"
      }
    },
    "source_sha256": {
      "apps/player/apps/native/Sources/Design/MediaViews.swift": "4cfe896857cb59524802bbf8e87440e7e72f5e8e32e34244cfe18152bd9dad44",
      "apps/player/apps/native/Sources/Design/Artwork.swift": "5c520575a5f052a98c3f66c047439951cb48c1a68d176c0a76423b2da062e01b",
      "apps/player/apps/native/Sources/Design/ArtworkPrefetch.swift": "0eefb3abfbfa22c2a78806e30e7361aac332868fb49df5cb4b5457424e938492",
      "apps/player/apps/native/Sources/Design/ResourceView.swift": "c912879bfc4610fdb858f955b3ebc4ea1cbbbcc1cd3220ed9ae791c4ea2ca5ec",
      "apps/player/apps/native/Sources/Features/Home/HomeScreen.swift": "888b39cbc16cd180e33c74d949efede266b4511715a0eca0b62778d0fd0c4c4d",
      "apps/player/apps/native/Sources/Platform/ArtworkLoader.swift": "2c71b58d54284ae8c68ad8d7265739cf0f334fde66b96f28c38090ebbb8608aa"
    },
    "journey": [
      "Activate the seeded app; assert Search and Continue watching exist; attach screenshot",
      "Down to watching shelf; hold Right then Left for 1s each, three times",
      "Down to Movies; assert focus; Select; wait for Sort: Title",
      "Down to grid; hold Right, Down, Left for 1s each, three times; attach screenshot",
      "Menu to Home; assert Movies exists and has focus; attach screenshot"
    ],
    "reproduction": "Restore the ignored fixture, seed test and diagnostic sources, and verify their hashes. Start the fixture in loaded mode. Seed the synthetic session through the tvOS test target. Build the remote scheme for testing in Release. Relaunch the installed app before attached recordings so the journey starts at Search. Keep trace files and result bundles private.",
    "ui_build_command": "xcodebuild -project Kinosail.xcodeproj -scheme Kinosail-tvOS-Remote -configuration Release -destination 'platform=tvOS Simulator,id=<task simulator>' -derivedDataPath .build/speed-tvos-release -jobs 2 -parallel-testing-enabled NO ONLY_ACTIVE_ARCH=YES CODE_SIGNING_ALLOWED=YES CODE_SIGN_IDENTITY=- ENABLE_TESTABILITY=YES build-for-testing",
    "ui_command": "xcodebuild -project Kinosail.xcodeproj -scheme Kinosail-tvOS-Remote -configuration Release -destination 'platform=tvOS Simulator,id=<task simulator>' -derivedDataPath .build/speed-tvos-release -jobs 2 -parallel-testing-enabled NO ONLY_ACTIVE_ARCH=YES CODE_SIGNING_ALLOWED=YES CODE_SIGN_IDENTITY=- ENABLE_TESTABILITY=YES -only-testing:Kinosail-tvOSRemoteUITests/RemotePerformanceDiagnosticTests -resultBundlePath <unique private result bundle> test-without-building",
    "ui_results": [
      {
        "recording": "combined all-process attempt",
        "result": "1 passed, 0 failures",
        "test_seconds": 26.701
      },
      {
        "recording": "app-only CPU",
        "result": "1 passed, 0 failures",
        "test_seconds": 26.351
      },
      {
        "recording": "app-only SwiftUI with layout tracing",
        "result": "1 passed, 0 failures",
        "test_seconds": 26.236
      },
      {
        "recording": "relative launch attempt, app activated by test",
        "result": "1 passed, 0 failures",
        "test_seconds": 28.384
      },
      {
        "recording": "absolute app launch with SwiftUI",
        "result": "1 passed, 0 failures",
        "test_seconds": 28.144
      }
    ],
    "cpu_record_command": "xcrun xctrace record --template 'Time Profiler' --device <task simulator> --attach <observed app PID> --time-limit 45s --output <private trace> --no-prompt",
    "cpu_export_command": "xcrun xctrace export --input <private trace> --xpath '/trace-toc/run[@number=\"1\"]/data/table[@schema=\"time-profile\"]' --output <private XML>",
    "cpu_result": {
      "record_exit": 0,
      "duration_seconds": 45.685679,
      "sample_weight_ms": 1,
      "running_thread_samples": 13281,
      "main_thread_samples": 9000,
      "non_main_thread_samples": 4281,
      "main_unresolved_leaf_samples": 7971,
      "app_and_dsym_uuid_match": true,
      "main_inclusive_ms": {
        "MediaCard.body.getter": 32,
        "Artwork.body.getter": 25,
        "AppSession.profileKey.getter": 31
      },
      "non_main_inclusive_ms": {
        "ArtworkLoader.decode closure": 2208,
        "ArtworkLoader.decodedThumbnail": 2152,
        "LibraryPage.init": 713
      },
      "note": "Inclusive stack counts overlap and must not be added. Samples include navigation and automation. Source names for unresolved app frames were recovered with atos and the matching Release dSYM."
    },
    "potential_hangs": {
      "threshold_ms": 250,
      "samples": [
        {
          "trace_start_seconds": 13.65992575,
          "duration_ms": 302.083791
        },
        {
          "trace_start_seconds": 14.213243958,
          "duration_ms": 283.819417
        },
        {
          "trace_start_seconds": 37.082390125,
          "duration_ms": 433.988708
        }
      ],
      "note": "The trace reports Microhang intervals. Automated activation, accessibility queries, navigation, and screenshots overlap the workload. No physical hitch or input-to-visible-feedback distribution is established."
    },
    "recording_limits": [
      "SwiftUI template: Hitches unsupported on this simulator; the saved output has no exportable run data.",
      "Combined all-process CPU/SwiftUI: overlapping dylib metadata prevents saving a usable trace.",
      "App-only CPU: recording and export succeed, with a missing-input-source table warning and unresolved framework symbols.",
      "App-only CPU plus SwiftUI, attached and launched with layout tracing enabled: CPU trace saved, but Instruments reports no SwiftUI data.",
      "First UI filter used the scheme name as a target; it failed before running tests. The actual Kinosail-tvOSRemoteUITests target succeeds.",
      "Relative app launch failed with posix_spawn file-not-found. Absolute app bundle launch succeeds, still without SwiftUI data."
    ],
    "raw_artifacts": "Ignored .verification/tvos-render-profile; native .verification/frontiers-navigation-*.xcresult; hashes retained in baseline-manifest.json. No raw process inventory, trace, credentials, or device identifiers are published.",
    "boundary": "Synthetic loopback fixture and optimized simulator correctness/profiling only. Warm encoded caches, instrumentation, accessibility automation, and shared-host load affect results. No production UI code changed in this phase. No physical-device, deployed-network, Safari, iOS frame-time, or before/after smoothness gain is claimed."
  }
}
```
