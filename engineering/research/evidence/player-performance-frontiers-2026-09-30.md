# Performance frontiers: measurement record

## Confirmed rich-metadata matches

Measured September 30, 2026, against `55c0b765c16eb68a403b1a8f4c76f8a71340bdad`. The selected implementation changes only `packages/catalog/search_metadata.go`. Rich records check an ASCII title that fits local storage, then literal field prefixes that preserve normalization. Unconfirmed matches retain the complete field and credit sequence. Short records keep the existing path.

The fixture uses 10,000 synthetic movies. Each plot repeats `A quiet journey. ` 120 times and ends with Café. Requests preserve a synthetic Owner context and derive an uncancelled `context.WithCancel`. The real catalog HTTP adapter runs in process. Network, authentication middleware, real media, physical display, and deployed load are excluded.

| Rich query | Baseline median, ms | Selected median, ms | Original-source reverse, ms | Allocated bytes before → selected |
| --- | --- | --- | --- | --- |
| Movie, 10,000 matches | 181.102 | 43.247 | 178.278 | 125,750,333 → 1,448,070 |
| Movie 9999, one match | 138.950 | 138.941 | 137.964 | 124,022,800 → 124,010,109 |
| quiet journey, 10,000 matches | 181.118 | 44.728 | 179.141 | 125,555,280 → 1,582,725 |
| Absent owl, zero matches | 151.991 | 153.099 | 151.087 | 124,014,782 → 124,014,893 |

Broad title and literal plot matches return about 75–76% sooner and allocate about 99% fewer bytes. Their totals and response sizes remain unchanged. The exact-title median is essentially unchanged against the first baseline and 0.7% above the reverse control; sample ranges overlap. The shortcut adds bounded work on misses. These sequential shared-host samples do not establish zero overhead, a confidence interval, or an exact-search improvement.

A rejected prototype normalized every title into scratch storage before allocating the full fallback. Long ASCII titles increased allocation from about 13.56 to 15.87 MB per 1,000-item search. A 450-byte compatibility title that expands during NFKD increased allocation from about 59.42 to 66.33 MB. The selected guard checks only ASCII titles shorter than the 512-byte scratch capacity, including room for a separator. Other titles remain in the complete fallback.

| Additional workload | Baseline → selected median, ms | Allocated bytes before → selected | Response bytes |
| --- | --- | --- | --- |
| long-ascii | 17.472 → 17.501 | 13,562,874 → 13,562,903 | 125 |
| unicode-expansion | 159.687 → 160.325 | 59,420,297 → 59,420,098 | 125 |
| 10000/browse | 4.978 → 4.956 | 1,767,662 → 1,767,426 | 19,286 |
| 10000/search | 2.192 → 2.136 | 494,825 → 494,821 | 316 |
| 100000/browse | 57.529 → 58.140 | 15,458,966 → 15,461,043 | 19,668 |
| 100000/search | 21.548 → 21.293 | 4,820,316 → 4,820,316 | 321 |

The long-title control uses 1,000 movies with the same accented plot and an absent query. Titles repeat either `Long title. ` 100 times or `ﷺ` 150 times. Ordinary requests use the existing 10,000/100,000-title fixture. Allocation levels remain similar. This is allocation traffic per request, not retained cache memory.

Before production changes, the 32-case public HTTP regression passed. Removing the literal-query byte guard then failed the two raw F/A versus ℉/𝐀 cases. The final regression preserves compatibility characters, phrases spanning fields and credits, late and cross-prefix matches, malformed field bytes, empty titles, and title expansion. Independent review found no production correctness issue. Review corrected a UTF-8 fixture boundary before the final timing and validation. Production and all benchmark fixtures retain the recorded hashes.

The full shared, Player, and Subtitles Go suites and catalog race check passed on the measured production source. A test-only assertion helper extraction resolved the new function-length lint issue and passed focused validation. Changed-code lint against the baseline passed in all three modules. Source caps, repository tooling, and regenerated Code Atlas snapshots passed. A full shared lint check caught a new complexity warning that diff-based lint missed. Extracting the unchanged size calculation resolved it; the compiler confirms inlining. Full shared lint returns to 112 existing findings. Post-commit app checks are recorded below. Required hosted checks remain the delivery authority. No new native, browser, Nox, or production-network performance claim is made.

Post-commit verification used `bd6b6bf1da7a2cd92269d9644af2dc7203258d50`: `make -C apps/player verify-changed BASE=55c0b765c16eb68a403b1a8f4c76f8a71340bdad`, followed by the equivalent Subtitles command. Both passed caps and diff checks. Player reused cached server compilation and focused checks; Subtitles selected no app Go package before shared lint. Each exited 2 at 112 existing shared lint findings. Later stages did not run. Separate fresh full Go runs above cover the final consumer source. The private `verify-changed-results.json` preserves revision, commands, timestamps, durations, and exits.

Both comparison binaries include identical benchmark sources. Compilation restores only the production file from the baseline and restores the candidate in a finally block. Final runs use `-test.run=^$ -test.bench=<expression> -test.benchmem -test.count=<count> -test.benchtime=1s`. Rich and large-title runs use three samples; ordinary navigation uses five. The reverse control reuses the original baseline binary. No task-owned builds, lint, or tests overlap a timed run. Other host activity is uncontrolled. Private scripts, binaries, raw logs, and validation records remain in `.verification/title-first-profile`.

Raw sample rows below use `[ns/op, B/op, allocs/op, response-bytes]`. Benchmark paths omit the shared `BenchmarkNativeCatalog` prefix.

```json
{
  "baseline_revision": "55c0b765c16eb68a403b1a8f4c76f8a71340bdad",
  "environment": {
    "go": "go version go1.27.1 darwin/arm64",
    "os": "ProductName:\t\tmacOS\nProductVersion:\t\t27.0\nBuildVersion:\t\t26A428",
    "machine": "arm64",
    "concurrency": "Go suffix -10; sequential controls; no task-owned builds, lint or tests overlap timed runs. Other host activity is uncontrolled."
  },
  "production_file": "packages/catalog/search_metadata.go",
  "binary_binding": [
    {
      "name": "final-baseline",
      "production_sha256": "e92e2e727b8675358ae7c88b8198a01098538f2d702f5b15bef6069e4d03da6a",
      "binary_sha256": "5451cf57bfeebf6bdb9f981a4e1069ebea0ad5044de97005337aff62a9765690"
    },
    {
      "name": "final-candidate",
      "production_sha256": "8553d891b6ce7791b7bea1ff6adf522a9c0af4160d513e391f4f488e785e3386",
      "binary_sha256": "c27debf8b21cfe910d889499760b159587725ca2532eec9fdacfdd91bcbb08a7"
    }
  ],
  "benchmark_sources": [
    {
      "file": "apps/player/internal/server/catalog_rich_search_benchmark_test.go",
      "sha256": "115d436739d6e3e1f67d74067b9b871de1370c01d4d770db7bfa280183359a21"
    },
    {
      "file": "apps/player/internal/server/catalog_large_titles_benchmark_test.go",
      "sha256": "1c1cf885dcee1c93c355071b0324347dc048a7e06c3ec2e248a2d817accef8b4"
    },
    {
      "file": "apps/player/internal/server/catalog_cancellation_test.go",
      "sha256": "f5fac3b4226079622720e909c7274f26936a112d582fb1c9d829888e770dcd1a"
    },
    {
      "file": "apps/player/internal/server/catalog_performance_benchmark_test.go",
      "sha256": "be7d1a6d6d4e7f618673c5602d68431cb62ceeb626706f17cb2d8b29454f401b"
    },
    {
      "file": "apps/player/internal/server/performance_benchmark_test.go",
      "sha256": "25e23c79c788868df664d59d9face0667dc0302efa39ef8cea8fd2422aa7f306"
    }
  ],
  "regression_source_at_compile": {
    "file": "apps/player/internal/server/catalog_rich_search_test.go",
    "sha256": "b3758c06c58a267b15221c7e775c5cb9e0e42a149ba8343a954909cf2dabef90"
  },
  "final_regression_source": {
    "file": "apps/player/internal/server/catalog_rich_search_test.go",
    "sha256": "b3758c06c58a267b15221c7e775c5cb9e0e42a149ba8343a954909cf2dabef90"
  },
  "commands": [
    {
      "file": "final-baseline-rich.log",
      "command": [
        ".verification/title-first-profile/final-baseline.test",
        "-test.run=^$",
        "-test.bench=^BenchmarkNativeCatalogRichSearch$",
        "-test.benchmem",
        "-test.count=3",
        "-test.benchtime=1s"
      ],
      "started": "2026-09-30T13:56:20.584826+00:00",
      "seconds": 18.297,
      "exit": 0
    },
    {
      "file": "final-baseline-overflow.log",
      "command": [
        ".verification/title-first-profile/final-baseline.test",
        "-test.run=^$",
        "-test.bench=^BenchmarkNativeCatalogLargeTitles$",
        "-test.benchmem",
        "-test.count=3",
        "-test.benchtime=1s"
      ],
      "started": "2026-09-30T13:56:38.882670+00:00",
      "seconds": 11.148,
      "exit": 0
    },
    {
      "file": "final-baseline-ordinary.log",
      "command": [
        ".verification/title-first-profile/final-baseline.test",
        "-test.run=^$",
        "-test.bench=^BenchmarkNativeCatalogNavigation$",
        "-test.benchmem",
        "-test.count=5",
        "-test.benchtime=1s"
      ],
      "started": "2026-09-30T13:56:50.031207+00:00",
      "seconds": 28.397,
      "exit": 0
    },
    {
      "file": "final-candidate-rich.log",
      "command": [
        ".verification/title-first-profile/final-candidate.test",
        "-test.run=^$",
        "-test.bench=^BenchmarkNativeCatalogRichSearch$",
        "-test.benchmem",
        "-test.count=3",
        "-test.benchtime=1s"
      ],
      "started": "2026-09-30T13:57:18.428312+00:00",
      "seconds": 18.62,
      "exit": 0
    },
    {
      "file": "final-candidate-overflow.log",
      "command": [
        ".verification/title-first-profile/final-candidate.test",
        "-test.run=^$",
        "-test.bench=^BenchmarkNativeCatalogLargeTitles$",
        "-test.benchmem",
        "-test.count=3",
        "-test.benchtime=1s"
      ],
      "started": "2026-09-30T13:57:37.048523+00:00",
      "seconds": 11.216,
      "exit": 0
    },
    {
      "file": "final-candidate-ordinary.log",
      "command": [
        ".verification/title-first-profile/final-candidate.test",
        "-test.run=^$",
        "-test.bench=^BenchmarkNativeCatalogNavigation$",
        "-test.benchmem",
        "-test.count=5",
        "-test.benchtime=1s"
      ],
      "started": "2026-09-30T13:57:48.265356+00:00",
      "seconds": 28.409,
      "exit": 0
    },
    {
      "file": "final-reverse-rich.log",
      "command": [
        ".verification/title-first-profile/final-baseline.test",
        "-test.run=^$",
        "-test.bench=^BenchmarkNativeCatalogRichSearch$",
        "-test.benchmem",
        "-test.count=3",
        "-test.benchtime=1s"
      ],
      "started": "2026-09-30T13:59:04.504311+00:00",
      "seconds": 17.48,
      "exit": 0
    }
  ],
  "rejected_prototype_experiment_binding": {
    "revision": "10b621f2383e41d4d439e7dd988b3de4ca0235b2",
    "fixtures": [
      {
        "file": "title_first_diagnostic_test.go",
        "sha256": "c1110d533aebb9969d15683a12efcfbc402467abce81e1f0bf1cc816f363a95a"
      },
      {
        "file": "title_overflow_diagnostic_test.go",
        "sha256": "db51fda31a941276748e73b0f7e0d4179349d914053d1eae5a42c23c5b0b9f3e"
      }
    ],
    "controls": [
      {
        "name": "overflow-baseline",
        "production_sha256": "e92e2e727b8675358ae7c88b8198a01098538f2d702f5b15bef6069e4d03da6a",
        "binary_sha256": "b4ad4a29b0b465f91f66a30e1d43ceeb5d7ee54da02da836e7cbd0c7797f2d24"
      },
      {
        "name": "overflow-prefix",
        "production_sha256": "14e897ab65f985797304189ed5a05f6df8d69b38530e9b08621e84208febaaae",
        "binary_sha256": "bad84245c5c840514afe743da0fcfba222e20fab2d5164a2a1bbc96fee60c0ac"
      },
      {
        "name": "guarded",
        "production_sha256": "7eaf283ee6df9b241e621a0dc3999ead45c4140535e19bc8e97712de008c7b4a",
        "binary_sha256": "d1039ebd4b5dacb31c93aa6f4c42f707fca6430a46d219e16bce9f1baf267bc8"
      }
    ]
  },
  "size_extraction_validation": {
    "full_shared_lint_exit": 1,
    "existing_findings": 112,
    "modified_production_findings": 0,
    "compiler": [
      "catalog/search_metadata.go:46:6: can inline metadataSearchSize with cost 44 as: func([]string, []library.Person, []library.Person) int { size := len(fields); for loop; for loop; return size }",
      "catalog/search_metadata.go:18:28: inlining call to metadataSearchSize"
    ]
  }
}
```

```json
{
  "final-baseline-rich.log": {
    "RichSearch/title-all": [[182110868, 125753896, 70577, 224755], [175543340, 125554738, 70524, 224755], [181102285, 125750333, 70529, 224755]],
    "RichSearch/title-exact": [[137815833, 124022800, 60091, 2371], [138949807, 124022800, 60091, 2371], [142225375, 124022798, 60091, 2371]],
    "RichSearch/metadata-all": [[182711570, 125554786, 70526, 224763], [176377188, 125555280, 70527, 224763], [181117535, 125749908, 70531, 224763]],
    "RichSearch/absent": [[149289345, 124014776, 60079, 125], [151991405, 124014782, 60079, 125], [154557399, 124014789, 60079, 125]]
  },
  "final-baseline-overflow.log": {
    "LargeTitles/long-ascii": [[17738112, 13562866, 6074, 125], [17471808, 13562915, 6074, 125], [17301085, 13562874, 6074, 125]],
    "LargeTitles/unicode-expansion": [[157996762, 59420297, 19079, 125], [159834685, 59420098, 19077, 125], [159687089, 59420398, 19080, 125]]
  },
  "final-baseline-ordinary.log": {
    "Navigation/10000/browse": [[4980749, 1768119, 485, 19286], [4957408, 1766624, 484, 19286], [4977534, 1767662, 484, 19286], [4955669, 1767654, 484, 19286], [4977850, 1767986, 484, 19286]],
    "Navigation/10000/search": [[2200781, 494827, 81, 316], [2184034, 494821, 81, 316], [2222815, 494821, 81, 316], [2179434, 494826, 81, 316], [2192435, 494825, 81, 316]],
    "Navigation/100000/browse": [[57960204, 15455325, 486, 19668], [57137781, 15455326, 486, 19668], [57528767, 15459588, 488, 19668], [57389341, 15458966, 487, 19668], [57913892, 15459591, 488, 19668]],
    "Navigation/100000/search": [[21792406, 4820318, 81, 321], [21548217, 4820316, 81, 321], [21569783, 4820287, 81, 321], [21487795, 4820263, 81, 321], [21545152, 4820345, 81, 321]]
  },
  "final-candidate-rich.log": {
    "RichSearch/title-all": [[43372064, 1583962, 10523, 224755], [43081248, 1448066, 10509, 224755], [43246699, 1448070, 10509, 224755]],
    "RichSearch/title-exact": [[138940823, 124010447, 60085, 2371], [137430234, 124008906, 60082, 2371], [141065136, 124010109, 60085, 2371]],
    "RichSearch/metadata-all": [[43605117, 1582725, 10513, 224763], [44749256, 1448117, 10511, 224763], [44728351, 1583200, 10514, 224763]],
    "RichSearch/absent": [[153098851, 124015198, 60079, 125], [153177655, 124014893, 60080, 125], [152350935, 124014676, 60078, 125]]
  },
  "final-candidate-overflow.log": {
    "LargeTitles/long-ascii": [[17501197, 13562879, 6074, 125], [17694663, 13562903, 6074, 125], [17393896, 13562914, 6074, 125]],
    "LargeTitles/unicode-expansion": [[162836286, 59420098, 19077, 125], [159556589, 59420096, 19077, 125], [160324845, 59420229, 19079, 125]]
  },
  "final-candidate-ordinary.log": {
    "Navigation/10000/browse": [[5118179, 1767426, 485, 19286], [4877972, 1767270, 484, 19286], [4955807, 1766606, 484, 19286], [4947948, 1767618, 484, 19286], [4984436, 1768031, 484, 19286]],
    "Navigation/10000/search": [[2135769, 494821, 81, 316], [2127969, 494826, 81, 316], [2137008, 494821, 81, 316], [2144644, 494820, 81, 316], [2122264, 494826, 81, 316]],
    "Navigation/100000/browse": [[59014512, 15459589, 488, 19668], [57953325, 15459590, 488, 19668], [58140206, 15463854, 489, 19668], [57688731, 15472381, 491, 19668], [58272940, 15461043, 488, 19668]],
    "Navigation/100000/search": [[21327847, 4820316, 81, 321], [20921417, 4820312, 81, 321], [21292838, 4820340, 81, 321], [20775346, 4820340, 81, 321], [21295958, 4820288, 81, 321]]
  },
  "overflow-prefix-overflow.log": {
    "BenchmarkTitleOverflowDiagnostic/long-ascii": [[18415787, 15866920, 8074, 125], [17592535, 15866889, 8074, 125], [17596693, 15866955, 8074, 125]],
    "BenchmarkTitleOverflowDiagnostic/unicode-expansion": [[158276101, 66332114, 23078, 125], [175733881, 66332297, 23079, 125], [157678411, 66332297, 23079, 125]]
  },
  "overflow-baseline-overflow.log": {
    "BenchmarkTitleOverflowDiagnostic/long-ascii": [[17574746, 13562844, 6074, 125], [17210918, 13562831, 6073, 125], [17161568, 13562869, 6073, 125]],
    "BenchmarkTitleOverflowDiagnostic/unicode-expansion": [[160039625, 59420017, 19077, 125], [158424375, 59420096, 19077, 125], [159968506, 59420112, 19077, 125]]
  },
  "guarded-overflow.log": {
    "BenchmarkTitleOverflowDiagnostic/long-ascii": [[17361969, 13562869, 6074, 125], [17341678, 13562926, 6074, 125], [17382751, 13562842, 6073, 125]],
    "BenchmarkTitleOverflowDiagnostic/unicode-expansion": [[158200530, 59420098, 19077, 125], [158712434, 59420299, 19079, 125], [158287441, 59420197, 19078, 125]]
  },
  "final-reverse-rich.log": {
    "RichSearch/title-all": [[180915333, 125558724, 70572, 224755], [178278208, 125554760, 70524, 224755], [177968674, 125750330, 70529, 224755]],
    "RichSearch/title-exact": [[137963797, 124022459, 60091, 2371], [136487859, 124022798, 60091, 2371], [139143016, 124022798, 60091, 2371]],
    "RichSearch/metadata-all": [[181106174, 126140616, 70540, 224763], [179141347, 125359670, 70522, 224763], [178079035, 125359668, 70522, 224763]],
    "RichSearch/absent": [[151086714, 124014778, 60079, 125], [150920381, 124014776, 60079, 125], [151697446, 124014773, 60079, 125]]
  }
}
```

## Catalog request cancellation

Measured September 30, 2026, against `f386ad2cdf55bb0abf9f55b97126e33ee851cdbf`. Both applications now forward the HTTP request context into shared catalog browsing. Existing input validation runs first. Cancellation checks stop projection, matching, ranking, and collation preparation without returning a partial page or retaining profile locks.

The cancelled workload uses 10,000 synthetic movies, with 120 repetitions of a plot followed by Café. Child contexts preserve the synthetic Owner identity. The real catalog HTTP adapter handles the request in process. Network and authentication middleware are excluded.

| Condition | Median time before → selected, ms | Allocated bytes before → selected | HTTP status before → selected |
| --- | --- | --- | --- |
| Already cancelled | 138.674682 → 0.002328 | 124,024,757 → 2,792 | 200 → 503 |
| 10 ms deadline | 138.061338 → 12.253801 | 124,026,949 → 10,518,784 | 200 → 503 |

The selected deadline case uses about 92% fewer allocated bytes and returns about 91% sooner. It stops obsolete work; it does not make a completed successful search 91% faster. The initial 256-item polling candidate takes 13.679394 ms and allocates 11,978,566 bytes. Polling every 64 items improves that recovery to 12.253801 ms and 10,518,784 bytes. Timer scheduling and collection affect the measured stop time.

Successful requests use a live, uncancelled `context.WithCancel`, closer to an incoming server request than a background context. The table includes an original-source reverse control.

| Successful workload | Baseline median, ms | Selected median, ms | Reverse control, ms |
| --- | --- | --- | --- |
| 10,000-title browse | 4.852141 | 5.066199 | 5.048016 |
| 10,000-title search | 2.149468 | 2.178868 | 2.168623 |
| 100,000-title browse | 56.465093 | 57.662148 | 57.810444 |
| 100,000-title search | 21.271157 | 21.514086 | 21.328890 |
| Short ASCII metadata | 7.047155 | 7.084135 | Not run |
| Long ASCII metadata | 41.534290 | 41.554780 | Not run |
| Long plot ending in Unicode | 138.715307 | 138.601490 | Not run |

Ordinary response byte counts remain unchanged: 19,286/19,668 for browsing, 316/321 for simple search, and 631/2501/2506 for metadata search. Allocation counts and bytes remain similar; raw variation is preserved below. No material overhead is detected against the reverse control, but these sequential shared-host samples do not establish zero overhead or a confidence interval.

An initial diagnostic replaced the Owner context with a background context and produced an empty successful response. That comparison was rejected. Preserving Owner identity reproduced a 146.704 ms search after cancellation. The HTTP regression then failed in all three conditions before implementation: cancelled, expired, and expires during search. Each returned HTTP 200 and 2371 bytes. Shared test compilation also failed because the new context interface did not exist; that failure alone is not behavioral evidence.

The final API regression passes. Shared tests cover cancellation before load, after load, during visibility projection, lock release, unchanged caller storage and profile state, and invalid-input precedence. Ranking and collation preparation are source-reviewed without deterministic cancellation tests. Subtitles cancellation is source-reviewed; its full Go suite provides consumer verification.

Validation passed on the bound candidate source: `go -C packages test ./...`, `go -C apps/player test ./...`, `go -C apps/subtitles test ./...`, and `go -C packages test -race ./catalog`. Changed-code lint against the exact baseline reports zero issues in all three modules. Both Code Atlas snapshots were regenerated. `make max-loc`, `make tooling-check`, and `git diff --check` passed. An independent review found no correctness issue and verified the measurement source hashes. Private logs and the reproducible fixture remain under `.verification/catalog-cancellation-profile`.

Post-commit verification used `a69c5d2157d8ec4bd8e79326af29c435c35233d2`: `make -C apps/player verify-changed BASE=f386ad2cdf55bb0abf9f55b97126e33ee851cdbf`, followed by the equivalent Subtitles command. Both passed caps, diff checks, server compilation, and focused Go tests. Each then exited 2 at 112 existing shared lint findings. Later stages did not run. The private `verify-changed-results.json` records the exact revision, commands, timestamps, and exit codes. Required hosted checks remain the delivery authority.

Index loading, mutex acquisition, grouping, reference/page copies, and sorting already underway remain synchronous. No physical UI, production tail-latency, deployed network, or Nox gain is claimed. The prior metadata-search change merged through PR #376; both its required PR checks and main publication workflow succeeded. That publication remains separate from deployment proof. The cancellation change later merged through PR #377 into `55c0b765c16eb68a403b1a8f4c76f8a71340bdad`. Its required PR checks and main publication workflow also passed. No newer Nox deployment proof was collected.

```json
{
  "baseline_revision": "f386ad2cdf55bb0abf9f55b97126e33ee851cdbf",
  "candidate_binding": "Baseline plus the five production file hashes below. Selected polling interval: 64 items.",
  "environment": {
    "go": "go1.27.1",
    "os": "macOS 27.0 (26A428)",
    "arch": "darwin/arm64",
    "cpu": "Apple M1 Pro",
    "concurrency": "Go benchmark suffix -10. Sequential conditions; no task-owned builds, lint, or tests overlapped timed runs. Other host activity is uncontrolled."
  },
  "fixture": {
    "cancelled_metadata_items": 10000,
    "plot": "A quiet journey. repeated 120 times, followed by Caf\u00e9",
    "query": "Movie 9999",
    "role": "Synthetic Owner, preserved by deriving child contexts from the request context",
    "successful_item_counts": [
      10000,
      100000
    ],
    "successful_context": "Uncancelled context.WithCancel, including ordinary metadata benchmarks",
    "successful_metadata": "10,000 items; short/long/late-Unicode plots, Drama / Mystery, Alex North, Sam Reed/Captain and Morgan Vale/Guide",
    "excluded": "Network, authentication middleware, TLS, real media, physical display, deployed load"
  },
  "commands": {
    "compile": "From apps/player: go test -c ./internal/server -o <private comparison binary>. For the baseline, restore only the five production files from the exact baseline revision, compile, then restore their candidate bytes in a finally block. Both binaries include the same benchmark and fixture source.",
    "cancelled": "<comparison binary> -test.run ^$ -test.bench ^BenchmarkNativeCatalogCancelledMetadataSearch$ -test.benchtime=1s -test.count=5",
    "successful": "<comparison binary> -test.run ^$ -test.bench ^BenchmarkNativeCatalogNavigation$ -test.benchtime=1s -test.count=5",
    "metadata": "<comparison binary> -test.run ^$ -test.bench ^BenchmarkNativeCatalogMetadataSearch$ -test.benchtime=1s -test.count=3",
    "sequence": "baseline cancelled; 256-item cancelled; baseline successful; 256-item successful; 64-item cancelled; 64-item successful; baseline metadata; 64-item metadata; baseline successful reverse control"
  },
  "source_binding": {
    "baseline_revision": "f386ad2cdf55bb0abf9f55b97126e33ee851cdbf",
    "source": [
      {
        "file": "packages/catalog/browse.go",
        "baseline_sha256": "dec15c9ce65b98c3795f5fc89edc832e9a9644b865bd188d159ff56216f48e43",
        "candidate_sha256": "0b1bbcef8c63e1b6f57a97272c0b9db35fa0799a7c95f541083a21309c6f77f6",
        "polling_256_sha256": "0b1bbcef8c63e1b6f57a97272c0b9db35fa0799a7c95f541083a21309c6f77f6"
      },
      {
        "file": "packages/catalog/browse_apply.go",
        "baseline_sha256": "40e48c06ead5262347a34a65f0a01a8d3febc825b3ae64bd5fb9c7aa19ba6ec7",
        "candidate_sha256": "1e17e5056f702b4a906d19721b143b5a1ce320e2323c9aafa80f01a07725c9f0",
        "polling_256_sha256": "e49d220fd47b048b67bcd8442a6b900c550ab9f66fd4c41220d3f8ed41999725"
      },
      {
        "file": "packages/catalog/browse_sort.go",
        "baseline_sha256": "d783d39f6a1ddbdd61b1335910288ad818dd561352479b9b03d740643aefcb9f",
        "candidate_sha256": "cbc6c6343910e88f1bb8475f3c9a4796a03a3c1b458906bb5865cf0e8fb2b066",
        "polling_256_sha256": "252bfeb3a6f07177d35a3dbd3b24559d44a5f047f7c98317b056c33664205aa8"
      },
      {
        "file": "apps/player/internal/server/browse.go",
        "baseline_sha256": "0b675515b9cdd746efc4a77768d16d92632f6835ddd92ed9d682826ce31e7f85",
        "candidate_sha256": "544b975f6231e8bc1e65fcfd90b94b2ca7e760e1bf7672d33e70f024f0a06cef",
        "polling_256_sha256": "544b975f6231e8bc1e65fcfd90b94b2ca7e760e1bf7672d33e70f024f0a06cef"
      },
      {
        "file": "apps/subtitles/internal/server/browse.go",
        "baseline_sha256": "0b675515b9cdd746efc4a77768d16d92632f6835ddd92ed9d682826ce31e7f85",
        "candidate_sha256": "544b975f6231e8bc1e65fcfd90b94b2ca7e760e1bf7672d33e70f024f0a06cef",
        "polling_256_sha256": "544b975f6231e8bc1e65fcfd90b94b2ca7e760e1bf7672d33e70f024f0a06cef"
      },
      {
        "file": "apps/player/internal/server/catalog_cancellation_test.go",
        "sha256": "f5fac3b4226079622720e909c7274f26936a112d582fb1c9d829888e770dcd1a"
      },
      {
        "file": "apps/player/internal/server/catalog_cancellation_benchmark_test.go",
        "sha256": "c95ccf1bab1ef2487547a67626d37d2d1d35e327bf48bb1753b3dfb0fb56d533"
      },
      {
        "file": "apps/player/internal/server/catalog_performance_benchmark_test.go",
        "sha256": "be7d1a6d6d4e7f618673c5602d68431cb62ceeb626706f17cb2d8b29454f401b"
      },
      {
        "file": "packages/catalog/browse_cancellation_test.go",
        "sha256": "0e00f20fcb58e2db892615caeffd3c5950db10b0c5ea1ba952b276a9b0af2b52"
      },
      {
        "file": "apps/player/internal/server/performance_benchmark_test.go",
        "sha256": "25e23c79c788868df664d59d9face0667dc0302efa39ef8cea8fd2422aa7f306"
      },
      {
        "file": "apps/player/internal/server/library_index_test.go",
        "sha256": "284890cc3e7d02d277cd3d5537ce19cbc4bf26df3be533525716c737d43964bf"
      }
    ],
    "binary": [
      {
        "file": "baseline.test",
        "sha256": "00a64d112323dbca2b04d675581dd22c384671ff8db2560f92cea78d47a8e612"
      },
      {
        "file": "candidate.test",
        "sha256": "0fc1626ec8487e2379f028bcf693bd4e495fecc22be0f7756b02df9e5ccbe77f"
      },
      {
        "file": "candidate-64.test",
        "sha256": "4a9a8b636ec553ac9d95d193b4942a421129f76ebf182469f83731727bf8cd6a"
      }
    ],
    "controls": "candidate.test is the initial 256-item polling control; candidate-64.test is the selected 64-item polling candidate."
  },
  "sample_columns": [
    "iterations",
    "ns/op",
    "B/op",
    "allocs/op",
    "response-bytes",
    "status-code"
  ],
  "raw_files": [
    {
      "file": "cancel-before.log",
      "sha256": "d952cf1b9e2488278a275d223bb01b833df4e5577382b1c8af7a5a1d6cdd5b8f"
    },
    {
      "file": "cancel-after.log",
      "sha256": "19448ac056602baf58d2c2dc8eebfd25d5b487eaed454a694bc723d6b5f214b0"
    },
    {
      "file": "cancel-polling-64.log",
      "sha256": "abac4ca73781844f71a1cd0a15346127448254f86df299c5e65025cfe65af674"
    },
    {
      "file": "api-before.log",
      "sha256": "84603daf2702308452389d6c31c3b51bc56b6321551ca2da6b4b14df8187ce7d"
    },
    {
      "file": "api-after.log",
      "sha256": "4c5022c1f5ef053f2dc27187b3f73684b823de7e12580fffe0032e9655028ace"
    },
    {
      "file": "api-polling-64.log",
      "sha256": "5581539a73e6362fe3d2643f1652923b6b0a39d44a878062a47bfceb864b8327"
    },
    {
      "file": "api-reverse-control.log",
      "sha256": "cbff2f518b71f8417623bb719e6cf22c183d6928a76433fc844dfe2ebc69a6d2"
    },
    {
      "file": "metadata-before.log",
      "sha256": "e77684e9053871256601f720a06f706678308d10a26324530adbcf266d559159"
    },
    {
      "file": "metadata-after.log",
      "sha256": "05517a3b7e8903f4d61abb5d66a92b3d135fa585c0187881aa03277022b7bea1"
    }
  ],
  "boundary": "Descriptive shared-host handler benchmarks, without randomized order or a confidence interval. Cancellation returns an existing 503 error instead of a successful obsolete page. Sorting, grouping, copies, index loading, and mutex acquisition already underway remain synchronous. No physical UI or production tail-latency gain is claimed.",
  "controls": [
    "Initial diagnostic replacing the Owner context with context.Background returned an empty successful response; rejected as an invalid comparison. The accepted diagnostic preserves Owner context and returned item 9999 after cancellation in 146.704 ms.",
    "HTTP red regression: all three cancellation conditions returned 200 and 2371 bytes before implementation. Domain test red was a compile failure because the context interface did not exist.",
    "256-item polling is retained as a control: deadline median 13.679 ms versus 12.254 ms at 64 items. Both preserve ordinary allocation levels."
  ]
}
```

All raw benchmark samples follow. Row columns are `iterations`, `ns/op`, `B/op`, `allocs/op`, `response-bytes`, and `status-code`. A null status metric means the successful-request benchmark checked HTTP 200 but did not emit that metric.

```json
{
"cancel-before.log": {
  "BenchmarkNativeCatalogCancelledMetadataSearch/already-cancelled": [
    [8,138674682.0,124028208.0,60129.0,2371.0,200.0],
    [8,140221302.0,124025526.0,60093.0,2371.0,200.0],
    [8,138189964.0,124024757.0,60091.0,2371.0,200.0],
    [8,134400208.0,124023968.0,60089.0,2371.0,200.0],
    [8,139637838.0,124023982.0,60089.0,2371.0,200.0]
  ],
  "BenchmarkNativeCatalogCancelledMetadataSearch/10ms-deadline": [
    [8,138363083.0,124025516.0,60096.0,2371.0,200.0],
    [8,137775005.0,124027828.0,60101.0,2371.0,200.0],
    [8,136696719.0,124027644.0,60101.0,2371.0,200.0],
    [8,139328958.0,124026949.0,60099.0,2371.0,200.0],
    [8,138061338.0,124025391.0,60096.0,2371.0,200.0]
  ]
},
"cancel-after.log": {
  "BenchmarkNativeCatalogCancelledMetadataSearch/already-cancelled": [
    [477352,2289.0,2792.0,30.0,29.0,503.0],
    [501453,2315.0,2792.0,30.0,29.0,503.0],
    [501894,2334.0,2792.0,30.0,29.0,503.0],
    [505826,2331.0,2792.0,30.0,29.0,503.0],
    [494208,2356.0,2792.0,30.0,29.0,503.0]
  ],
  "BenchmarkNativeCatalogCancelledMetadataSearch/10ms-deadline": [
    [82,13679394.0,12055753.0,5660.0,38.0,503.0],
    [82,13666298.0,11978566.0,5622.0,38.0,503.0],
    [84,13784323.0,12307257.0,5782.0,38.0,503.0],
    [84,13386324.0,11516712.0,5398.0,38.0,503.0],
    [81,13773657.0,11847155.0,5558.0,38.0,503.0]
  ]
},
"cancel-polling-64.log": {
  "BenchmarkNativeCatalogCancelledMetadataSearch/already-cancelled": [
    [487122,2294.0,2792.0,30.0,29.0,503.0],
    [495582,2318.0,2792.0,30.0,29.0,503.0],
    [497232,2350.0,2792.0,30.0,29.0,503.0],
    [507199,2364.0,2792.0,30.0,29.0,503.0],
    [500124,2328.0,2792.0,30.0,29.0,503.0]
  ],
  "BenchmarkNativeCatalogCancelledMetadataSearch/10ms-deadline": [
    [97,12253801.0,10600874.0,4953.0,38.0,503.0],
    [93,12269535.0,10559841.0,4933.0,38.0,503.0],
    [100,12280940.0,10479249.0,4894.0,38.0,503.0],
    [100,12101318.0,10455543.0,4882.0,38.0,503.0],
    [100,12117235.0,10518784.0,4913.0,38.0,503.0]
  ]
},
"api-before.log": {
  "BenchmarkNativeCatalogNavigation/10000/browse": [
    [242,4947507.0,1767765.0,485.0,19286.0,null],
    [250,4852141.0,1767270.0,484.0,19286.0,null],
    [250,4836187.0,1767270.0,484.0,19286.0,null],
    [248,4847873.0,1767625.0,484.0,19286.0,null],
    [250,4888532.0,1767270.0,484.0,19286.0,null]
  ],
  "BenchmarkNativeCatalogNavigation/10000/search": [
    [565,2153478.0,494818.0,81.0,316.0,null],
    [571,2142057.0,494820.0,81.0,316.0,null],
    [561,2135210.0,494824.0,81.0,316.0,null],
    [571,2149468.0,494809.0,81.0,316.0,null],
    [565,2152005.0,494818.0,81.0,316.0,null]
  ],
  "BenchmarkNativeCatalogNavigation/100000/browse": [
    [20,57205367.0,15455325.0,486.0,19668.0,null],
    [21,56465093.0,15458967.0,487.0,19668.0,null],
    [21,55964194.0,15458967.0,487.0,19668.0,null],
    [20,56507844.0,15459588.0,488.0,19668.0,null],
    [21,56303708.0,15463027.0,488.0,19668.0,null]
  ],
  "BenchmarkNativeCatalogNavigation/100000/search": [
    [55,21756302.0,4820316.0,81.0,321.0,null],
    [56,21271157.0,4820287.0,81.0,321.0,null],
    [56,20788541.0,4820259.0,81.0,321.0,null],
    [56,21473060.0,4820259.0,81.0,321.0,null],
    [56,21055662.0,4820314.0,81.0,321.0,null]
  ]
},
"api-after.log": {
  "BenchmarkNativeCatalogNavigation/10000/browse": [
    [235,5062713.0,1766722.0,485.0,19286.0,null],
    [246,5001580.0,1766600.0,484.0,19286.0,null],
    [240,5015847.0,1767328.0,484.0,19286.0,null],
    [241,5078760.0,1768030.0,484.0,19286.0,null],
    [241,5006381.0,1767676.0,484.0,19286.0,null]
  ],
  "BenchmarkNativeCatalogNavigation/10000/search": [
    [535,2194320.0,494819.0,81.0,316.0,null],
    [558,2176181.0,494821.0,81.0,316.0,null],
    [560,2188978.0,494824.0,81.0,316.0,null],
    [548,2154985.0,494816.0,81.0,316.0,null],
    [561,2177580.0,494815.0,81.0,316.0,null]
  ],
  "BenchmarkNativeCatalogNavigation/100000/browse": [
    [20,57570898.0,15451061.0,485.0,19668.0,null],
    [20,58213056.0,15455325.0,486.0,19668.0,null],
    [20,58197750.0,15455326.0,486.0,19668.0,null],
    [20,57286558.0,15455327.0,486.0,19668.0,null],
    [20,58657871.0,15463853.0,489.0,19668.0,null]
  ],
  "BenchmarkNativeCatalogNavigation/100000/search": [
    [54,21596471.0,4820318.0,81.0,321.0,null],
    [55,21613673.0,4820288.0,81.0,321.0,null],
    [55,21551595.0,4820288.0,81.0,321.0,null],
    [55,21572964.0,4820345.0,81.0,321.0,null],
    [55,21563029.0,4820316.0,81.0,321.0,null]
  ]
},
"api-polling-64.log": {
  "BenchmarkNativeCatalogNavigation/10000/search": [
    [541,2212257.0,494867.0,81.0,316.0,null],
    [554,2188723.0,494821.0,81.0,316.0,null],
    [561,2175529.0,494818.0,81.0,316.0,null],
    [556,2178868.0,494818.0,81.0,316.0,null],
    [560,2165441.0,494818.0,81.0,316.0,null]
  ],
  "BenchmarkNativeCatalogNavigation/10000/browse": [
    [240,5035263.0,1767337.0,484.0,19286.0,null],
    [237,5113014.0,1767347.0,484.0,19286.0,null],
    [240,5006621.0,1768039.0,484.0,19286.0,null],
    [231,5124743.0,1767004.0,484.0,19286.0,null],
    [242,5066199.0,1766964.0,484.0,19286.0,null]
  ],
  "BenchmarkNativeCatalogNavigation/100000/browse": [
    [20,57662148.0,15455325.0,486.0,19668.0,null],
    [20,57234019.0,15455326.0,486.0,19668.0,null],
    [20,57272185.0,15451061.0,485.0,19668.0,null],
    [20,58053956.0,15455326.0,486.0,19668.0,null],
    [20,58111606.0,15459590.0,488.0,19668.0,null]
  ],
  "BenchmarkNativeCatalogNavigation/100000/search": [
    [55,21532603.0,4820297.0,81.0,321.0,null],
    [54,21406187.0,4820261.0,81.0,321.0,null],
    [54,21541791.0,4820289.0,81.0,321.0,null],
    [56,21514086.0,4820342.0,81.0,321.0,null],
    [55,21445318.0,4820260.0,81.0,321.0,null]
  ]
},
"api-reverse-control.log": {
  "BenchmarkNativeCatalogNavigation/10000/browse": [
    [238,5082092.0,1768145.0,485.0,19286.0,null],
    [240,5048016.0,1766618.0,484.0,19286.0,null],
    [249,4955646.0,1766934.0,484.0,19286.0,null],
    [238,5086209.0,1767329.0,484.0,19286.0,null],
    [246,5001101.0,1766946.0,484.0,19286.0,null]
  ],
  "BenchmarkNativeCatalogNavigation/10000/search": [
    [538,2185035.0,494819.0,81.0,316.0,null],
    [556,2168623.0,494818.0,81.0,316.0,null],
    [567,2167896.0,494823.0,81.0,316.0,null],
    [559,2181511.0,494815.0,81.0,316.0,null],
    [562,2161999.0,494821.0,81.0,316.0,null]
  ],
  "BenchmarkNativeCatalogNavigation/100000/browse": [
    [20,57896898.0,15459588.0,488.0,19668.0,null],
    [20,58615462.0,15459588.0,488.0,19668.0,null],
    [20,57311565.0,15455330.0,486.0,19668.0,null],
    [20,57810444.0,15455324.0,486.0,19668.0,null],
    [20,57236819.0,15463854.0,489.0,19668.0,null]
  ],
  "BenchmarkNativeCatalogNavigation/100000/search": [
    [56,21461381.0,4820287.0,81.0,321.0,null],
    [55,21664261.0,4820316.0,81.0,321.0,null],
    [56,21294583.0,4820314.0,81.0,321.0,null],
    [54,21328890.0,4820289.0,81.0,321.0,null],
    [55,21301260.0,4820316.0,81.0,321.0,null]
  ]
},
"metadata-before.log": {
  "BenchmarkNativeCatalogMetadataSearch/late-unicode": [
    [8,138715307.0,124029697.0,60136.0,2506.0,null],
    [8,139184453.0,124025217.0,60093.0,2506.0,null],
    [8,136580724.0,124025982.0,60094.0,2506.0,null]
  ],
  "BenchmarkNativeCatalogMetadataSearch/short-ascii": [
    [168,7118694.0,495888.0,83.0,631.0,null],
    [170,7013574.0,495917.0,83.0,631.0,null],
    [169,7047155.0,495917.0,83.0,631.0,null]
  ],
  "BenchmarkNativeCatalogMetadataSearch/long-ascii": [
    [27,41534290.0,23541034.0,10084.0,2501.0,null],
    [28,41256677.0,23541248.0,10085.0,2501.0,null],
    [28,42176152.0,23541471.0,10086.0,2501.0,null]
  ]
},
"metadata-after.log": {
  "BenchmarkNativeCatalogMetadataSearch/long-ascii": [
    [28,42573996.0,23542750.0,10098.0,2501.0,null],
    [28,40997976.0,23541476.0,10086.0,2501.0,null],
    [28,41554780.0,23541916.0,10087.0,2501.0,null]
  ],
  "BenchmarkNativeCatalogMetadataSearch/late-unicode": [
    [8,136648391.0,124025982.0,60094.0,2506.0,null],
    [8,138601490.0,124025980.0,60094.0,2506.0,null],
    [8,140292312.0,124026773.0,60096.0,2506.0,null]
  ],
  "BenchmarkNativeCatalogMetadataSearch/short-ascii": [
    [169,7060443.0,495902.0,83.0,631.0,null],
    [165,7140848.0,495903.0,83.0,631.0,null],
    [169,7084135.0,495887.0,83.0,631.0,null]
  ]
}
}
```

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
- Focused Go navigation-bundle tests passed in both apps. Shared Go tests passed. Browser script lint reports zero errors and warnings. The UI detector reports no findings. Source-file caps and `make tooling-check` passed. Code Atlas regeneration produced no source snapshot changes.
- Six additional Chromium cases passed: Home poster/backdrop skeleton geometry at 390 and 1440 pixels, infinite-scroll retry, and compact title-jump first paint. The test-instance-only populated first-paint case was skipped; real populated browsing was measured separately with the private fixture.
- The repository TypeScript type-policy checker stopped because TypeScript 7.0.2 exposes no `ScriptTarget.Latest` through the API the checker uses. This is an existing tooling compatibility limit; the dependency and checker were not changed.
- Full `go test ./...` passed in Player, Subtitles, and shared packages. Both app `verify-changed` commands passed their compile and focused Go stages, then stopped at 112 existing shared lint findings outside this change. Shared changed-code lint against `964a60c7e61df208dbb4ce704d68f06a8b08b412` reports zero issues. Hosted delivery is recorded separately.

Hardware focus timing, Safari device performance, production networks, scan/download/transcode interference, artwork derivatives, and deployed first frame remain open. The mobile 10,000-item fixture also exposes existing count/Sort label crowding; that layout issue is outside this rendering patch.

## Metadata search allocation and latency

The shared catalog now normalizes each metadata field into request-local storage. It avoids joined metadata and credit strings. Short records use 512 bytes of stack scratch space; longer records reserve storage once and grow if Unicode decomposition needs more. There is no retained index or cache to invalidate. Matching order, ranking, accents, punctuation, field separators, and compatibility-character behavior stay unchanged.

Matched handler measurements use the exact baseline and the final candidate. The table shows median time and allocated bytes per request. Web and simple API cases have five samples each; rich metadata cases have three.

| Workload | Time, ms before → after | Reduction | Allocated MB before → after |
| --- | --- | --- | --- |
| LargeLibraryKnownTitleNavigation/search | 3.482 → 2.886 | 17.1% | 1.476 → 0.996 |
| NativeCatalogNavigation/10000/search | 2.728 → 2.163 | 20.7% | 0.975 → 0.495 |
| NativeCatalogNavigation/100000/search | 26.679 → 21.460 | 19.6% | 9.620 → 4.820 |
| NativeCatalogMetadataSearch/short-ascii | 10.651 → 7.067 | 33.6% | 6.816 → 0.496 |
| NativeCatalogMetadataSearch/long-ascii | 62.915 → 40.861 | 35.1% | 47.782 → 23.542 |
| NativeCatalogMetadataSearch/late-unicode | 273.487 → 138.863 | 49.2% | 184.027 → 124.026 |

The native API returns the same 316/321 response bytes. Rich metadata responses remain 631, 2501, and 2506 bytes. A 10,000-title simple API search removes 20,000 allocations; its count falls from 20,081 to 81. At 100,000 titles, allocated bytes fall by approximately 4.8 MB. The reverse original-source control stays near its original timing and allocation level.

A compatibility-capital regression was caught before delivery. Lowercasing after NFKD would change matches for ℉ and 𝐀. The final implementation retains lowercase-before-decomposition behavior. Tests cover those values, every metadata field, cast and show cast, control/invalid bytes, repeated punctuation, field-spanning phrases, large records, and decomposition that exceeds the original capacity.

These are shared-host handler measurements. They establish lower allocation traffic and lower median time in these synthetic workloads. They do not prove physical-device frames, production tail latency, or browser INP improvements. Sequential conditions and uncontrolled host activity limit causal precision. All matched samples and source bindings follow.

```json
{
  "baseline_revision": "7d58ba78e61559ae9e10685e99c941dfbdd091ac",
  "candidate_binding": "Baseline plus the source files bound by SHA-256 below; no cache or API schema change.",
  "environment": {
    "go": "go1.27.1",
    "os": "macOS 27.0 (26A428)",
    "arch": "darwin/arm64",
    "cpu": "Apple M1 Pro",
    "concurrency": "Go benchmark suffix -10; sequential conditions. No task-owned builds or tests overlapped the timed final runs. Other host activity is uncontrolled."
  },
  "fixture": {
    "native_counts": [
      10000,
      100000
    ],
    "query": "Movie N-1",
    "web_count": 10000,
    "metadata_count": 10000,
    "metadata_query": "Movie 9999",
    "short_plot": "A quiet journey. repeated 10 times",
    "long_plot": "A quiet journey. repeated 120 times",
    "unicode_plot": "Long plot followed by Café",
    "credits": "Alex North; Sam Reed/Captain; Morgan Vale/Guide",
    "profile": "synthetic owner",
    "network": "httptest real catalog/web HTTP handlers; no TLS, media, probe, transcode, or physical device"
  },
  "commands": [
    "go test ./internal/server -run '^$' -bench '^BenchmarkLargeLibraryKnownTitleNavigation$/^search$' -benchtime=1s -count=5",
    "go test ./internal/server -run '^$' -bench '^BenchmarkNativeCatalogNavigation$/./^search$' -benchtime=1s -count=5",
    "go test ./internal/server -run '^$' -bench '^BenchmarkNativeCatalogMetadataSearch$' -benchtime=1s -count=3"
  ],
  "baseline_metadata_control": "The new benchmark file was compiled with the exact baseline search.go, then the candidate was restored before running the baseline binary. Newly added helpers are unused by that baseline. The original API binary was rerun for five reverse-control samples.",
  "source_sha256": {
    "packages/catalog/search.go": "b67d7546cb2e708d1b1ab24e8687621fd53f8c9cb80b67ad9f8c1261f3007e29",
    "packages/catalog/search_metadata.go": "e92e2e727b8675358ae7c88b8198a01098538f2d702f5b15bef6069e4d03da6a",
    "packages/catalog/search_metadata_test.go": "1f269a29f6fa6d5d4498d70cfc27e4e523c0dc8644e48b6f54e6ddedb1e824df",
    "apps/player/internal/server/catalog_performance_benchmark_test.go": "ab836fa213ee3a56f71bdab4aa541c4aab7869919c2b5765cb4b9a8d282f8c34"
  },
  "baseline_search_sha256": "c78342cf4f3d077f75d14ba5c4bdc0f7fc5bcb22b7ce24722f025e86b575d5aa",
  "samples": {
    "web-search-before.log": {
      "BenchmarkLargeLibraryKnownTitleNavigation/search-10": {
        "ns_per_op": [
          3526075,
          3403750,
          3489520,
          3481621,
          3471726
        ],
        "bytes_per_op": [
          1476306,
          1476260,
          1476289,
          1476291,
          1476290
        ],
        "allocations_per_op": [
          28961,
          28961,
          28961,
          28961,
          28961
        ],
        "response_bytes": []
      }
    },
    "web-search-verified.log": {
      "BenchmarkLargeLibraryKnownTitleNavigation/search-10": {
        "ns_per_op": [
          2918228,
          2862527,
          2900110,
          2873701,
          2885508
        ],
        "bytes_per_op": [
          996270,
          996247,
          996248,
          996247,
          996235
        ],
        "allocations_per_op": [
          8961,
          8961,
          8961,
          8961,
          8961
        ],
        "response_bytes": []
      }
    },
    "api-search-before.log": {
      "BenchmarkNativeCatalogNavigation/10000/search-10": {
        "ns_per_op": [
          2753240,
          2722463,
          2728222,
          2721084,
          2734309
        ],
        "bytes_per_op": [
          974894,
          974843,
          974833,
          974837,
          974836
        ],
        "allocations_per_op": [
          20081,
          20081,
          20081,
          20081,
          20081
        ],
        "response_bytes": [
          316.0,
          316.0,
          316.0,
          316.0,
          316.0
        ]
      },
      "BenchmarkNativeCatalogNavigation/100000/search-10": {
        "ns_per_op": [
          26891579,
          26729765,
          26637269,
          26375397,
          26679264
        ],
        "bytes_per_op": [
          9620444,
          9620404,
          9620404,
          9620444,
          9620408
        ],
        "allocations_per_op": [
          200082,
          200081,
          200081,
          200082,
          200081
        ],
        "response_bytes": [
          321.0,
          321.0,
          321.0,
          321.0,
          321.0
        ]
      }
    },
    "api-search-verified.log": {
      "BenchmarkNativeCatalogNavigation/10000/search-10": {
        "ns_per_op": [
          2166830,
          2162500,
          2162071,
          2148634,
          2180792
        ],
        "bytes_per_op": [
          494860,
          494816,
          494821,
          494821,
          494819
        ],
        "allocations_per_op": [
          81,
          81,
          81,
          81,
          81
        ],
        "response_bytes": [
          316.0,
          316.0,
          316.0,
          316.0,
          316.0
        ]
      },
      "BenchmarkNativeCatalogNavigation/100000/search-10": {
        "ns_per_op": [
          21721633,
          21254314,
          21460003,
          21586713,
          21229441
        ],
        "bytes_per_op": [
          4820316,
          4820289,
          4820259,
          4820342,
          4820314
        ],
        "allocations_per_op": [
          81,
          81,
          81,
          81,
          81
        ],
        "response_bytes": [
          321.0,
          321.0,
          321.0,
          321.0,
          321.0
        ]
      }
    },
    "metadata-search-before.log": {
      "BenchmarkNativeCatalogMetadataSearch/short-ascii-10": {
        "ns_per_op": [
          10731702,
          10651017,
          10513719
        ],
        "bytes_per_op": [
          6816382,
          6816147,
          6816121
        ],
        "allocations_per_op": [
          60087,
          60084,
          60084
        ],
        "response_bytes": [
          631.0,
          631.0,
          631.0
        ]
      },
      "BenchmarkNativeCatalogMetadataSearch/long-ascii-10": {
        "ns_per_op": [
          63027377,
          62914726,
          62673431
        ],
        "bytes_per_op": [
          47782309,
          47782018,
          47781440
        ],
        "allocations_per_op": [
          60087,
          60087,
          60085
        ],
        "response_bytes": [
          2501.0,
          2501.0,
          2501.0
        ]
      },
      "BenchmarkNativeCatalogMetadataSearch/late-unicode-10": {
        "ns_per_op": [
          273511260,
          273487354,
          272183271
        ],
        "bytes_per_op": [
          184027490,
          184026792,
          184026808
        ],
        "allocations_per_op": [
          200097,
          200096,
          200096
        ],
        "response_bytes": [
          2506.0,
          2506.0,
          2506.0
        ]
      }
    },
    "metadata-search-verified.log": {
      "BenchmarkNativeCatalogMetadataSearch/short-ascii-10": {
        "ns_per_op": [
          7069873,
          7063791,
          7067404
        ],
        "bytes_per_op": [
          496039,
          495888,
          495887
        ],
        "allocations_per_op": [
          85,
          83,
          83
        ],
        "response_bytes": [
          631.0,
          631.0,
          631.0
        ]
      },
      "BenchmarkNativeCatalogMetadataSearch/long-ascii-10": {
        "ns_per_op": [
          41615874,
          40858789,
          40861064
        ],
        "bytes_per_op": [
          23541694,
          23541695,
          23541472
        ],
        "allocations_per_op": [
          10086,
          10086,
          10086
        ],
        "response_bytes": [
          2501.0,
          2501.0,
          2501.0
        ]
      },
      "BenchmarkNativeCatalogMetadataSearch/late-unicode-10": {
        "ns_per_op": [
          139084411,
          138863135,
          136579766
        ],
        "bytes_per_op": [
          124025980,
          124026775,
          124025243
        ],
        "allocations_per_op": [
          60094,
          60096,
          60093
        ],
        "response_bytes": [
          2506.0,
          2506.0,
          2506.0
        ]
      }
    },
    "api-search-reverse-control.log": {
      "BenchmarkNativeCatalogNavigation/10000/search-10": {
        "ns_per_op": [
          2794504,
          2719211,
          2753220,
          2736425,
          2756796
        ],
        "bytes_per_op": [
          974900,
          974839,
          974840,
          974848,
          974848
        ],
        "allocations_per_op": [
          20081,
          20081,
          20081,
          20081,
          20081
        ],
        "response_bytes": [
          316.0,
          316.0,
          316.0,
          316.0,
          316.0
        ]
      },
      "BenchmarkNativeCatalogNavigation/100000/search-10": {
        "ns_per_op": [
          26442028,
          26672352,
          26847069,
          27042501,
          26577775
        ],
        "bytes_per_op": [
          9620438,
          9620480,
          9620444,
          9620479,
          9620444
        ],
        "allocations_per_op": [
          200082,
          200082,
          200082,
          200082,
          200082
        ],
        "response_bytes": [
          321.0,
          321.0,
          321.0,
          321.0,
          321.0
        ]
      }
    }
  },
  "logs_sha256": [
    {
      "file": "web-search-before.log",
      "sha256": "2a63bb516cf9a2f1d01444e0de5b71a5a1f2e8a41e352685cb45b6b0c4a12f5b"
    },
    {
      "file": "web-search-verified.log",
      "sha256": "bcfe7912c6ce1788822c2c0dc99ba65cf31c8e7180bc789d5c2d0779fd2f79ff"
    },
    {
      "file": "api-search-before.log",
      "sha256": "4b493eb4ff36e3d13a497bc74beb8513d83954feac76b782fe9457869b471ee4"
    },
    {
      "file": "api-search-verified.log",
      "sha256": "f95691002279713df39be03752ad0abb6f6bd8a3cfbd12d5ff608ede0e5e345e"
    },
    {
      "file": "metadata-search-before.log",
      "sha256": "b05cc9f24fe168f77a8bb92451aa597b1df9c520cf5fd95b09afd72479fdba05"
    },
    {
      "file": "metadata-search-verified.log",
      "sha256": "f3874b6d4b727f5a56d432dbc414ee8b4320cfa192c6e41588b643c20cc78af3"
    },
    {
      "file": "api-search-reverse-control.log",
      "sha256": "43160ebcc7e88c83f3d623d8b58ff23bc56c08b438304f93a5f1f18d9f9317d2"
    }
  ],
  "rejected_controls": {
    "metadata-search-after.log": {
      "BenchmarkNativeCatalogMetadataSearch/short-ascii-10": {
        "ns_per_op": 7059159,
        "bytes_per_op": 495902,
        "allocations_per_op": 83,
        "response_bytes": 631.0
      },
      "BenchmarkNativeCatalogMetadataSearch/long-ascii-10": {
        "ns_per_op": 41444527,
        "bytes_per_op": 23541475,
        "allocations_per_op": 10086,
        "response_bytes": 2501.0
      },
      "BenchmarkNativeCatalogMetadataSearch/late-unicode-10": {
        "ns_per_op": 277224500,
        "bytes_per_op": 184026804,
        "allocations_per_op": 200096,
        "response_bytes": 2506.0
      }
    },
    "metadata-search-final.log": {
      "BenchmarkNativeCatalogMetadataSearch/short-ascii-10": {
        "ns_per_op": 7063705,
        "bytes_per_op": 495918,
        "allocations_per_op": 83,
        "response_bytes": 631.0
      },
      "BenchmarkNativeCatalogMetadataSearch/long-ascii-10": {
        "ns_per_op": 41095637,
        "bytes_per_op": 23541218,
        "allocations_per_op": 10085,
        "response_bytes": 2501.0
      },
      "BenchmarkNativeCatalogMetadataSearch/late-unicode-10": {
        "ns_per_op": 274121864,
        "bytes_per_op": 153389046,
        "allocations_per_op": 110101,
        "response_bytes": 2506.0
      }
    },
    "metadata-search-iterator.log": {
      "BenchmarkNativeCatalogMetadataSearch/late-unicode-10": {
        "ns_per_op": 327589615,
        "bytes_per_op": 109148320,
        "allocations_per_op": 170100,
        "response_bytes": 2506.0
      }
    },
    "metadata-search-fields.log": {
      "BenchmarkNativeCatalogMetadataSearch/short-ascii-10": {
        "ns_per_op": 8638852,
        "bytes_per_op": 495930,
        "allocations_per_op": 83,
        "response_bytes": 631.0
      },
      "BenchmarkNativeCatalogMetadataSearch/long-ascii-10": {
        "ns_per_op": 56452106,
        "bytes_per_op": 23541626,
        "allocations_per_op": 10086,
        "response_bytes": 2501.0
      },
      "BenchmarkNativeCatalogMetadataSearch/late-unicode-10": {
        "ns_per_op": 133027312,
        "bytes_per_op": 124026759,
        "allocations_per_op": 60096,
        "response_bytes": 2506.0
      }
    },
    "metadata-search-unicode-table-control.log": {
      "BenchmarkNativeCatalogMetadataSearch/short-ascii-10": {
        "ns_per_op": 7102462,
        "bytes_per_op": 495887,
        "allocations_per_op": 83,
        "response_bytes": 631.0
      },
      "BenchmarkNativeCatalogMetadataSearch/long-ascii-10": {
        "ns_per_op": 41843606,
        "bytes_per_op": 23541694,
        "allocations_per_op": 10086,
        "response_bytes": 2501.0
      },
      "BenchmarkNativeCatalogMetadataSearch/late-unicode-10": {
        "ns_per_op": 206364458,
        "bytes_per_op": 124025968,
        "allocations_per_op": 60094,
        "response_bytes": 2506.0
      }
    }
  },
  "limits": [
    "Descriptive medians, without randomized conditions or a confidence interval.",
    "Handler benchmarks do not measure native presentation, browser interaction, production network, or user hardware.",
    "A first combined benchmark expression matched no benchmarks and is excluded.",
    "The earlier Unicode-table candidate is a control, not the final measured implementation."
  ],
  "rejected_controls_note": "Initial ASCII-only fallback, Unicode builder Grow, norm.Iter, pre-compatibility field prototype, and Unicode-table control respectively. Full logs and prototype sources remain private. The iterator reduced allocation but increased latency. The early field prototype failed compatibility-capital tests and was corrected. Unicode table checks per ASCII rune were removed from the final path."
}
```

Metadata-search local validation:

- `go -C packages test ./...`: passed.
- `go -C packages test -race ./catalog`: passed.
- `go -C apps/player test ./...`: passed, including the full server package.
- `go -C apps/subtitles test ./...`: passed, including the full server package.
- Changed-code `golangci-lint run --new-from-rev=7d58ba78e61559ae9e10685e99c941dfbdd091ac`: zero issues in shared packages and Player.
- `make max-loc`, `make tooling-check`, and `git diff --check`: passed. Required Code Atlas snapshots were regenerated for both apps.
- Independent review found the early compatibility-capital regression. It confirmed the final fix and separator invariant. Its focused tests passed before the final ASCII-within-Unicode delta; the full local suites above cover that final delta.

Both required app `verify-changed` commands ran on the implementation bound by the source hashes above. Both passed source caps and diff checks. Player also passed its server compile and focused package stage. Both then stopped at the same 112 existing shared-package lint findings recorded in the earlier web phase. Later stages of those commands did not run. Full local Go suites and changed-code lint were run separately as listed above. This is a local verification limit; no gate was bypassed. The hosted secret scan initially classified two SHA-256 log fingerprints as generic API keys. Filenames and fingerprints now use separate fields. Only task-owned PR history was rewritten; measured production and benchmark source hashes remain identical. No scanner rule or ignore list changed.

The earlier web rendering change merged through [PR #375](https://github.com/Kinosail/kinosail/pull/375). Its [main workflow](https://github.com/Kinosail/kinosail/actions/runs/36699462274) completed successfully, including production-image publication checks. That establishes hosted publication evidence; this phase has no new Nox revision, remote health, physical-device, or production first-frame proof.

## Artwork dimension decode probe

The production thumbnail method was measured in an optimized standalone macOS probe. The generated source is a 297,827-byte, 3200 × 3200 JPEG. Six batches alternate dimension order. Each batch reports 25 decodes, its median, and its observed p95. The headline result is the median of the six batch medians.

A 400px output occupies 640,000 decoded bytes, versus 2,560,000 at 800px and 10,240,000 at 1600px. Median decode time is 3.954, 5.010, and 16.150 ms respectively. These values quantify this fixture's decode cost. They do not establish frame timing, quality on a focused TV card, or production network savings. No native artwork dimension changed.

```json
{
  "encoded_bytes": 297827,
  "fixture": "Generated 3200x3200 RGB block-pattern JPEG, quality 0.88",
  "platform": "macOS host ImageIO; not iOS/tvOS frame timing",
  "rows": [
    {
      "decoded_bytes": 640000,
      "dimension": 400,
      "median_ms": 3.93225,
      "p95_ms": 4.291541,
      "run": 0,
      "samples": 25
    },
    {
      "decoded_bytes": 2560000,
      "dimension": 800,
      "median_ms": 5.033041,
      "p95_ms": 5.268916,
      "run": 0,
      "samples": 25
    },
    {
      "decoded_bytes": 10240000,
      "dimension": 1600,
      "median_ms": 15.96525,
      "p95_ms": 17.892291,
      "run": 0,
      "samples": 25
    },
    {
      "decoded_bytes": 10240000,
      "dimension": 1600,
      "median_ms": 16.690792,
      "p95_ms": 18.209792,
      "run": 1,
      "samples": 25
    },
    {
      "decoded_bytes": 2560000,
      "dimension": 800,
      "median_ms": 5.095208,
      "p95_ms": 5.395875,
      "run": 1,
      "samples": 25
    },
    {
      "decoded_bytes": 640000,
      "dimension": 400,
      "median_ms": 4.097667,
      "p95_ms": 5.108208,
      "run": 1,
      "samples": 25
    },
    {
      "decoded_bytes": 640000,
      "dimension": 400,
      "median_ms": 4.006916,
      "p95_ms": 4.701791,
      "run": 2,
      "samples": 25
    },
    {
      "decoded_bytes": 2560000,
      "dimension": 800,
      "median_ms": 4.879791,
      "p95_ms": 5.239166,
      "run": 2,
      "samples": 25
    },
    {
      "decoded_bytes": 10240000,
      "dimension": 1600,
      "median_ms": 15.85975,
      "p95_ms": 17.167916,
      "run": 2,
      "samples": 25
    },
    {
      "decoded_bytes": 10240000,
      "dimension": 1600,
      "median_ms": 16.585667,
      "p95_ms": 18.022333,
      "run": 3,
      "samples": 25
    },
    {
      "decoded_bytes": 2560000,
      "dimension": 800,
      "median_ms": 5.038542,
      "p95_ms": 5.285,
      "run": 3,
      "samples": 25
    },
    {
      "decoded_bytes": 640000,
      "dimension": 400,
      "median_ms": 3.970583,
      "p95_ms": 4.164208,
      "run": 3,
      "samples": 25
    },
    {
      "decoded_bytes": 640000,
      "dimension": 400,
      "median_ms": 3.933583,
      "p95_ms": 4.047875,
      "run": 4,
      "samples": 25
    },
    {
      "decoded_bytes": 2560000,
      "dimension": 800,
      "median_ms": 4.89125,
      "p95_ms": 5.072209,
      "run": 4,
      "samples": 25
    },
    {
      "decoded_bytes": 10240000,
      "dimension": 1600,
      "median_ms": 16.331833,
      "p95_ms": 17.519292,
      "run": 4,
      "samples": 25
    },
    {
      "decoded_bytes": 10240000,
      "dimension": 1600,
      "median_ms": 15.968541,
      "p95_ms": 17.188625,
      "run": 5,
      "samples": 25
    },
    {
      "decoded_bytes": 2560000,
      "dimension": 800,
      "median_ms": 4.986042,
      "p95_ms": 5.282875,
      "run": 5,
      "samples": 25
    },
    {
      "decoded_bytes": 640000,
      "dimension": 400,
      "median_ms": 3.938334,
      "p95_ms": 4.154542,
      "run": 5,
      "samples": 25
    }
  ],
  "owner_method_sha256": "8f17ad3d041eb08deb09049a10293a97d39b1afd929be690ef58e9e8e93d36fc",
  "source_sha256": {
    "apps/player/apps/native/Sources/Platform/ArtworkLoader.swift": "2c71b58d54284ae8c68ad8d7265739cf0f334fde66b96f28c38090ebbb8608aa",
    ".verification/artwork-size-profile/decode-benchmark.swift": "264ff295d0b7340ad47e2a9c317c1df8b8b93eaaf4214354ca977c622bb4e95d",
    ".verification/artwork-size-profile/source.jpg": "29b3975cac9ae3e1e30a11643909c79a16859362927a303a4e9645ce5d15e22b"
  },
  "commands": [
    "xcrun swiftc -O .verification/artwork-size-profile/decode-benchmark.swift -o .verification/artwork-size-profile/decode-benchmark",
    ".verification/artwork-size-profile/decode-benchmark .verification/artwork-size-profile"
  ],
  "method": "Exact production decodedThumbnail method extracted into a standalone Swift probe. ClientError is an equivalent stub enum. Six batches alternate size order; each size has 25 decodes. Encoded data is held in memory. Each decoded image is validated for its requested square dimensions.",
  "median_of_batch_medians_ms": {
    "400": 3.9544585,
    "800": 5.0095415,
    "1600": 16.150187
  },
  "limits": "Synthetic RGB block pattern JPEG; macOS ImageIO only. No photographic quality, native UI, texture-upload, network, physical-device, or artwork production change is established. Raw probe and fixture remain in the private evidence directory."
}
```

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
