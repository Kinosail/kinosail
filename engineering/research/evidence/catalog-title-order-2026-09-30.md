# Immutable catalog title-order experiment

Date: September 30, 2026. Baseline: `77494fdff25d5e97a6d1f8e87a0659eb31dc990a`.

This record evaluates repeated server work shared by web, iOS, and tvOS. It measures Go HTTP handlers on a shared macOS host. It does not measure network latency, physical-device frames, or deployed load.

## Why this cache

The 100,000-title native JSON browse CPU profile attributes 53.47% of sampled CPU to `sortTitles` and its callees. The allocation profile includes process setup; its percentages are not per-request allocation percentages.

The final candidate retains completed title ordering for a normalized locale and catalog version. Each request still reads current visibility, progress, and list state. Search, added, year, and history retain their existing sorting paths. Shows still group and sort their aggregate records.

A cold request performs its existing selected sort. Only a successful complete-catalog projection can seed shared order. Restricted profiles and sparse views cannot trigger extra global collation. Concurrent cold requests can duplicate their existing work; the implementation adds no shared builders or cancellation coupling. Completed results own a private reference array with no mutating consumers. Admission retains that array. Cache hits still copy the reference slice before projection. The private loader returns its order flag by value; it does not force a browse struct to escape through a pointer.

Retained order is capped at four locales and 1,048,576 item references total. This is at most 8 MiB of pointer storage on supported 64-bit targets, plus small entry metadata. It is not a cap on the catalog, active request arrays, or temporary publication snapshots. Larger catalogs and duplicate IDs keep the uncached path.

Successful scan or decorator publication clears entries under the index mutex and advances a version. A late request cannot publish an order from an older version. Failed scans keep the last ready catalog. Decorators receive a detached top-level item array, so previously returned title fields remain stable.

The locale behavior follows the pinned `golang.org/x/text v0.42.0` [collation API](https://pkg.go.dev/golang.org/x/text/collate). Cache entries contain finalized references rather than shared collators or scratch keys. Mutable cache metadata and publication use the same index mutex, consistent with the [Go memory model](https://go.dev/ref/mem).

## Controls and limits

Both comparison binaries compile before timing. The baseline uses the exact baseline production source with the same cold benchmark fixture. Its new-method HTTP regression is excluded from baseline compilation. A `finally` block restores every temporary source edit byte-for-byte. Source and binary hashes bind the measured controls.

Warm sequences include the first cold request and then repeated navigation. Their averages amortize admission. Cold sequences construct a fresh index outside timing for every request. The fixture contains 100,000 items: 99,999 movies and one book. Response validation outside timing checks exact totals, page sizes, and the book ID. Index construction and response validation are excluded from request timing and allocation counts. Their garbage collection can still affect the shared process.

Forward controls use five samples; reverse controls use three. Each warm sample runs for one second. Each cold sample uses 20 requests. No task-owned builds, lint, or tests overlap timing. Unrelated GUI and background services remain running. Raw samples and reverse controls define the practical limits of the wall-time comparison.

## Admission coverage

Warm measurements use a movie-only catalog. Admission needs every indexed item before paging, rather than every item visible to one profile. Native `warmCatalog` prepares movies and shows; native home loads added-order and history pages. Those home requests do not seed title ordering. A movie-only request cannot seed the cache in a mixed movie/episode catalog. A complete all-title request can seed it for later filtered views. These limits follow the current native [catalog requests](../../../apps/player/apps/native/Sources/Services/CatalogAPI.swift).

Complete intrinsic-view partitions are a follow-up candidate for mixed libraries. They need separate whole-partition coverage checks, bounded total storage, and current profile filtering. Caching a restricted profile's partial order would be incorrect. This phase does not establish a gain for every native entry route.

## Discarded admission copy

The first candidate, `d1454d09d49f0c5d41564bc5e9b5cc62ae6d2409`, copied references again at admission. Cold full-catalog requests allocated about 800 KB more per request. Forward median time changed from 54.632 to 55.791 ms; reverse medians changed from 54.580 to 60.719 ms. This prompted the ownership reuse in the final candidate. The first copy-based warm controls showed repeatable browse gains, but they are not the final candidate's measurements. All first-pass logs, hashes, and parsed samples remain in the private experiment directory with the `copy-admission-` prefix.

## Regression evidence

Before production edits, `TestDecoratorPublicationPreservesPreviousReferences` failed because installing a decorator changed the old snapshot to Zebra and Apple. The repaired owner preserves Alpha and Beta in that snapshot.

The new index-owned public contracts were written before implementation. They cover locales, views, pages, letters, current profile state, failed and successful scans, rejected queries, already canceled requests, duplicate IDs, and concurrent publication.

A deterministic HTTP regression pauses a cold request after it captures references. It publishes new titles, then finishes the old request and sends a new request. The old response retains Alpha/Beta; the new response returns Apple/Zebra. Removing only the admission version check makes the new response return stale Alpha/Beta with status 200, and the regression fails. The exact source was restored after that negative control.

Independent read-only review found no concrete correctness issue in the final cache or its lifecycle regression. Review did not run tests independently.

## Final measured results

Candidate: `1ce5ef46b3333f78b540c7035f8bbe692e3d50c4`.

### Warm sequences

| Case | Forward baseline → candidate (ms) | Reverse baseline → candidate (ms) | Forward bytes/op | Response bytes |
| --- | ---: | ---: | ---: | ---: |
| `LargeLibraryBrowse` | 6.240 → 3.116 | 6.138 → 2.916 | 2,428,521 → 1,446,038 | 25319 |
| `LargeLibraryKnownTitleNavigation/letter` | 6.438 → 2.483 | 6.108 → 2.244 | 2,429,265 → 1,444,302 | not reported |
| `LargeLibraryKnownTitleNavigation/search` | 3.142 → 3.147 | 2.896 → 2.904 | 996,234 → 996,225 | not reported |
| `NativeCatalogNavigation/10000/browse` | 6.457 → 1.373 | 5.071 → 1.316 | 1,767,650 → 781,635 | 19286 |
| `NativeCatalogNavigation/10000/search` | 2.545 → 2.207 | 2.174 → 2.140 | 494,822 → 494,821 | 316 |
| `NativeCatalogNavigation/100000/browse` | 67.630 → 18.017 | 57.503 → 16.321 | 15,456,146 → 5,831,358 | 19668 |
| `NativeCatalogNavigation/100000/search` | 21.629 → 21.800 | 21.154 → 21.038 | 4,820,261 → 4,820,289 | 321 |

### Cold sequences

| Case | Forward baseline → candidate (ms) | Reverse baseline → candidate (ms) | Forward bytes/op | Response bytes |
| --- | ---: | ---: | ---: | ---: |
| `NativeCatalogColdBrowse/all` | 54.547 → 52.851 | 56.102 → 53.099 | 15,464,143 → 15,464,657 | 19664 |
| `NativeCatalogColdBrowse/books` | 11.735 → 11.458 | 11.669 → 11.596 | 4,827,792 → 4,828,016 | 324 |
| `NativeCatalogColdBrowse/movies` | 58.767 → 56.101 | 57.027 → 56.441 | 15,464,144 → 15,464,382 | 19682 |

Repeated 100,000-title JSON browse time drops by 73.4% forward and 71.6% reverse. Allocated bytes drop by about 9.62 MB per request. Repeated 10,000-title web browse time drops by 50.1% forward and 52.5% reverse. Letter navigation drops by 61.4% and 63.3%.

Search has no repeatable speed gain. Its final allocation counts match the baseline; the earlier extra escaping browse allocation is gone. Cold full-catalog allocation stays around 15.46 MB per request. Admission no longer adds the 800 KB copy. Cold wall-time differences are small relative to host and sample variation; do not infer a cold-start improvement from them.

The final 100,000-title CPU profile shifts the remaining work to current-state projection and filtering. `itemCandidates` accounts for 38.79% cumulative sampled CPU; `browseCandidates` accounts for 17.68%. Letter generation accounts for 11.87%. Percentages share callees and must not be added. Request-array reuse and complete intrinsic-view partitions are useful next measurements. This profile includes the first cold request and fixture setup; it is not a physical UI trace.

## Verification and delivery boundaries

Final shared-package and Subtitles Go suites passed. Player server tests passed, but the concurrent full Player run failed `TestHealthcheckUsesConfiguredAuthHost` after its 90-second HTTPS readiness wait. The helper logged readiness later. The isolated check passed in 16.232 seconds, and the full Player retry passed without source changes. The first failure remains preserved. Its cause is not established.

Catalog race tests passed. Changed-code lint reports zero issues for packages, Player, and Subtitles. Source caps and repository tooling passed. Both post-commit app checks passed compilation and focused tests, then stopped at the same 112 existing shared lint findings. Later local stages did not run. Required hosted CI remains the delivery gate.

The measured candidate predates reconciliation with `2a62a537af0b817f77010360da319168c5f8baed`. That main revision includes independent HLS and web-shell changes. Reconciliation regenerates the architecture snapshot. The catalog implementation and benchmark source hashes below remain unchanged. Focused adapter checks after reconciliation are recorded separately. These measurements do not certify later UI changes, physical devices, deployment, or production networks.

The primary checkout and 17 unrelated inactive worktrees were left unchanged. Local main cleanup remains blocked by unrelated dirty primary work. The active performance worktree remains leased for the ongoing goal.

## Environment

```json
{
  "go": "go version go1.27.1 darwin/arm64",
  "macos": "ProductName:\t\tmacOS\nProductVersion:\t\t27.0\nBuildVersion:\t\t26A428",
  "machine": "arm64",
  "chip": "Apple M1 Pro",
  "logical_cpus": "10",
  "baseline_revision": "77494fdff25d5e97a6d1f8e87a0659eb31dc990a",
  "dependency_x_text": "v0.42.0",
  "host_note": "Shared macOS host; unrelated GUI and background services remain running. No task-owned builds, lint, or tests overlap timing.",
  "candidate_revision": "1ce5ef46b3333f78b540c7035f8bbe692e3d50c4"
}
```

## Bound source and binaries

```json
[
  {
    "file": "apps/player/internal/server/browse.go",
    "sha256": "a13ebeb8994d5e39e721c390a2f2e92f7ca2391b9065f9a35061a27ab60d8c2d"
  },
  {
    "file": "apps/subtitles/internal/server/browse.go",
    "sha256": "a13ebeb8994d5e39e721c390a2f2e92f7ca2391b9065f9a35061a27ab60d8c2d"
  },
  {
    "file": "packages/catalog/browse.go",
    "sha256": "5f32935c388ddc040fa4069ce352e1b50bad55d329fa5978b0ff9796ee2575af"
  },
  {
    "file": "packages/catalog/browse_apply.go",
    "sha256": "0a425888c57297c4b9366da120cf12f31589d80ba39b1524b375c22b58a648cc"
  },
  {
    "file": "packages/catalog/index.go",
    "sha256": "09a5e95f3fa518b23f5604f12b7abdd6a136376b6338e94633ac3ab27a8e5377"
  },
  {
    "file": "packages/catalog/scan.go",
    "sha256": "d90c8275262ff8a0161543beec5ac74c8a6c9f54de06fcfdc0985b4aecf0c0e6"
  },
  {
    "file": "packages/catalog/index_order.go",
    "sha256": "4fd125ad973395e562a327435435bce55e129fcb941f0e022c2d6683d25a9f5a"
  },
  {
    "file": "apps/player/internal/server/catalog_order_http_test.go",
    "sha256": "5e724cc8361a386c0df890483c61b6d1de346c12dc302b72490f618a134a985e"
  },
  {
    "file": "apps/player/internal/server/catalog_cold_performance_benchmark_test.go",
    "sha256": "d7411356486374c3c8b1dcfcf13652d8d02203deec60ccb0fe8415adf767b5b2"
  },
  {
    "file": "packages/catalog/index_order_test.go",
    "sha256": "599b9772aff0ca97755d12666ed61fd481dbf6cc099d92b2ff93c03a1875e851"
  },
  {
    "file": "apps/player/internal/server/catalog_performance_benchmark_test.go",
    "sha256": "be7d1a6d6d4e7f618673c5602d68431cb62ceeb626706f17cb2d8b29454f401b"
  },
  {
    "file": "apps/player/internal/server/performance_benchmark_test.go",
    "sha256": "25e23c79c788868df664d59d9face0667dc0302efa39ef8cea8fd2422aa7f306"
  }
]
```

```json
[
  {
    "file": "matched-baseline.test",
    "sha256": "bc7c005fce12d517ea937d2342076fec9b8be06ae53bbbe612f7c9cb76f77611"
  },
  {
    "file": "candidate.test",
    "sha256": "6a57d6c054dbeef620269a3f7d24c3e22c49b67c2f90d365e546b682f141163f"
  },
  {
    "file": "baseline.test",
    "sha256": "c27debf8b21cfe910d889499760b159587725ca2532eec9fdacfdd91bcbb08a7"
  }
]
```

## Reproduction

Run from the repository root with Go 1.27.1. The scripts expect the task source and benchmark files at the recorded revisions. Preserve existing work before creating comparison binaries.


`build-controls.py`:

```python
from pathlib import Path
import subprocess,hashlib,json
root=Path.cwd()
out=root/".verification/catalog-order-profile"
tracked=["apps/player/internal/server/browse.go","apps/subtitles/internal/server/browse.go","packages/catalog/browse.go","packages/catalog/browse_apply.go","packages/catalog/index.go","packages/catalog/scan.go"]
removed=["packages/catalog/index_order.go","apps/player/internal/server/catalog_order_http_test.go"]
paths=tracked+removed
original={f:(root/f).read_bytes() for f in paths}
def build(name):
 with (out/(name+"-build.log")).open("w") as log:
  subprocess.run(["go","test","-c","-o",str(out/(name+".test")),"./internal/server"],cwd=root/"apps/player",stdout=log,stderr=subprocess.STDOUT,check=True)
try:
 build("candidate")
 for f in tracked:
  (root/f).write_bytes(subprocess.check_output(["git","show","77494fdff25d5e97a6d1f8e87a0659eb31dc990a:"+f]))
 for f in removed: (root/f).unlink()
 build("matched-baseline")
finally:
 for f,data in original.items(): (root/f).write_bytes(data)
 assert all((root/f).read_bytes()==data for f,data in original.items())
files=paths+["apps/player/internal/server/catalog_cold_performance_benchmark_test.go"]
(out/"final-source.json").write_text(json.dumps([{"file":f,"sha256":hashlib.sha256((root/f).read_bytes()).hexdigest()} for f in files],indent=2)+"\n")
(out/"binary-hashes.json").write_text(json.dumps([{"file":name+".test","sha256":hashlib.sha256((out/(name+".test")).read_bytes()).hexdigest()} for name in ["matched-baseline","candidate"]],indent=2)+"\n")
print("Both controls compiled; all temporary production edits restored byte-for-byte.")
```

`run-measurements.py`:

```python
from pathlib import Path
import subprocess,json,time
root=Path.cwd();out=root/".verification/catalog-order-profile"
# Both binaries are complete before this script starts; no builds or tests overlap.
commands=[]
def run(binary,label,pattern,duration,count):
 command=[str(out/(binary+".test")),"-test.run","^$","-test.bench",pattern,"-test.benchtime",duration,"-test.count",str(count)]
 start=time.time()
 with (out/(label+".log")).open("w") as log:
  subprocess.run(command,cwd=root/"apps/player",stdout=log,stderr=subprocess.STDOUT,check=True)
 commands.append({"label":label,"command":command,"started_unix":start,"finished_unix":time.time()})
 (out/"measurement-commands.json").write_text(json.dumps(commands,indent=2)+"\n")
 print(label,"complete",flush=True)
pattern="^Benchmark(NativeCatalogNavigation|LargeLibraryBrowse|LargeLibraryKnownTitleNavigation)$"
run("matched-baseline","warm-baseline",pattern,"1s",5)
run("candidate","warm-candidate",pattern,"1s",5)
run("candidate","warm-candidate-reverse",pattern,"1s",3)
run("matched-baseline","warm-baseline-reverse",pattern,"1s",3)
run("matched-baseline","cold-baseline","^BenchmarkNativeCatalogColdBrowse$","20x",5)
run("candidate","cold-candidate","^BenchmarkNativeCatalogColdBrowse$","20x",5)
run("candidate","cold-candidate-reverse","^BenchmarkNativeCatalogColdBrowse$","20x",3)
run("matched-baseline","cold-baseline-reverse","^BenchmarkNativeCatalogColdBrowse$","20x",3)
```

## Raw timing samples


`warm-baseline.log`:

```text
goos: darwin
goarch: arm64
pkg: github.com/MikeO7/kinosail-player/internal/server
cpu: Apple M1 Pro
BenchmarkNativeCatalogNavigation/10000/browse-10         	     182	   6469534 ns/op	     19286 response-bytes	 1767438 B/op	     486 allocs/op
BenchmarkNativeCatalogNavigation/10000/browse-10         	     132	   8618749 ns/op	     19286 response-bytes	 1768499 B/op	     484 allocs/op
BenchmarkNativeCatalogNavigation/10000/browse-10         	     195	   6456845 ns/op	     19286 response-bytes	 1767650 B/op	     484 allocs/op
BenchmarkNativeCatalogNavigation/10000/browse-10         	     189	   6299018 ns/op	     19286 response-bytes	 1767256 B/op	     484 allocs/op
BenchmarkNativeCatalogNavigation/10000/browse-10         	     208	   6001551 ns/op	     19286 response-bytes	 1767950 B/op	     484 allocs/op
BenchmarkNativeCatalogNavigation/10000/search-10         	     500	   2709976 ns/op	       316.0 response-bytes	  494824 B/op	      81 allocs/op
BenchmarkNativeCatalogNavigation/10000/search-10         	     489	   2478060 ns/op	       316.0 response-bytes	  494811 B/op	      81 allocs/op
BenchmarkNativeCatalogNavigation/10000/search-10         	     406	   2544894 ns/op	       316.0 response-bytes	  494814 B/op	      81 allocs/op
BenchmarkNativeCatalogNavigation/10000/search-10         	     498	   2363697 ns/op	       316.0 response-bytes	  494823 B/op	      81 allocs/op
BenchmarkNativeCatalogNavigation/10000/search-10         	     523	   2607310 ns/op	       316.0 response-bytes	  494822 B/op	      81 allocs/op
BenchmarkNativeCatalogNavigation/100000/browse-10        	      19	  67629741 ns/op	     19668 response-bytes	15464765 B/op	     489 allocs/op
BenchmarkNativeCatalogNavigation/100000/browse-10        	      18	  65943343 ns/op	     19668 response-bytes	15456146 B/op	     486 allocs/op
BenchmarkNativeCatalogNavigation/100000/browse-10        	      18	  70633815 ns/op	     19668 response-bytes	15456146 B/op	     486 allocs/op
BenchmarkNativeCatalogNavigation/100000/browse-10        	      19	  68858189 ns/op	     19668 response-bytes	15455790 B/op	     486 allocs/op
BenchmarkNativeCatalogNavigation/100000/browse-10        	      19	  65776053 ns/op	     19668 response-bytes	15460278 B/op	     488 allocs/op
BenchmarkNativeCatalogNavigation/100000/search-10        	      48	  23992011 ns/op	       321.0 response-bytes	 4820332 B/op	      81 allocs/op
BenchmarkNativeCatalogNavigation/100000/search-10        	      55	  21755841 ns/op	       321.0 response-bytes	 4820316 B/op	      81 allocs/op
BenchmarkNativeCatalogNavigation/100000/search-10        	      55	  21628572 ns/op	       321.0 response-bytes	 4820260 B/op	      81 allocs/op
BenchmarkNativeCatalogNavigation/100000/search-10        	      54	  21520881 ns/op	       321.0 response-bytes	 4820261 B/op	      81 allocs/op
BenchmarkNativeCatalogNavigation/100000/search-10        	      56	  21511979 ns/op	       321.0 response-bytes	 4820259 B/op	      81 allocs/op
BenchmarkLargeLibraryBrowse-10                           	     189	   6240312 ns/op	     25319 response-bytes	 2428573 B/op	   11573 allocs/op
BenchmarkLargeLibraryBrowse-10                           	     193	   6129317 ns/op	     25319 response-bytes	 2428521 B/op	   11572 allocs/op
BenchmarkLargeLibraryBrowse-10                           	     186	   6302436 ns/op	     25319 response-bytes	 2428501 B/op	   11572 allocs/op
BenchmarkLargeLibraryBrowse-10                           	     190	   6343041 ns/op	     25319 response-bytes	 2428524 B/op	   11572 allocs/op
BenchmarkLargeLibraryBrowse-10                           	     193	   6141674 ns/op	     25319 response-bytes	 2428441 B/op	   11572 allocs/op
BenchmarkLargeLibraryKnownTitleNavigation/search-10      	     402	   3017424 ns/op	  996234 B/op	    8961 allocs/op
BenchmarkLargeLibraryKnownTitleNavigation/search-10      	     376	   3030547 ns/op	  996211 B/op	    8961 allocs/op
BenchmarkLargeLibraryKnownTitleNavigation/search-10      	     351	   3142258 ns/op	  996228 B/op	    8961 allocs/op
BenchmarkLargeLibraryKnownTitleNavigation/search-10      	     376	   3182727 ns/op	  996239 B/op	    8961 allocs/op
BenchmarkLargeLibraryKnownTitleNavigation/search-10      	     338	   3216298 ns/op	  996245 B/op	    8961 allocs/op
BenchmarkLargeLibraryKnownTitleNavigation/letter-10      	     180	   6643067 ns/op	 2429277 B/op	   11587 allocs/op
BenchmarkLargeLibraryKnownTitleNavigation/letter-10      	     184	   6438231 ns/op	 2429271 B/op	   11587 allocs/op
BenchmarkLargeLibraryKnownTitleNavigation/letter-10      	     169	   6551541 ns/op	 2429127 B/op	   11587 allocs/op
BenchmarkLargeLibraryKnownTitleNavigation/letter-10      	     195	   6074688 ns/op	 2429265 B/op	   11587 allocs/op
BenchmarkLargeLibraryKnownTitleNavigation/letter-10      	     183	   6433045 ns/op	 2429216 B/op	   11587 allocs/op
PASS
```

`warm-candidate.log`:

```text
goos: darwin
goarch: arm64
pkg: github.com/MikeO7/kinosail-player/internal/server
cpu: Apple M1 Pro
BenchmarkNativeCatalogNavigation/10000/browse-10         	     795	   1372994 ns/op	     19286 response-bytes	  783030 B/op	     476 allocs/op
BenchmarkNativeCatalogNavigation/10000/browse-10         	     942	   1533787 ns/op	     19286 response-bytes	  781193 B/op	     476 allocs/op
BenchmarkNativeCatalogNavigation/10000/browse-10         	     957	   1356840 ns/op	     19286 response-bytes	  781635 B/op	     476 allocs/op
BenchmarkNativeCatalogNavigation/10000/browse-10         	     903	   1424385 ns/op	     19286 response-bytes	  781673 B/op	     476 allocs/op
BenchmarkNativeCatalogNavigation/10000/browse-10         	     892	   1347676 ns/op	     19286 response-bytes	  781490 B/op	     476 allocs/op
BenchmarkNativeCatalogNavigation/10000/search-10         	     546	   2168562 ns/op	       316.0 response-bytes	  494816 B/op	      81 allocs/op
BenchmarkNativeCatalogNavigation/10000/search-10         	     559	   2251467 ns/op	       316.0 response-bytes	  494821 B/op	      81 allocs/op
BenchmarkNativeCatalogNavigation/10000/search-10         	     566	   2181155 ns/op	       316.0 response-bytes	  494826 B/op	      81 allocs/op
BenchmarkNativeCatalogNavigation/10000/search-10         	     529	   2291363 ns/op	       316.0 response-bytes	  494826 B/op	      81 allocs/op
BenchmarkNativeCatalogNavigation/10000/search-10         	     564	   2206755 ns/op	       316.0 response-bytes	  494821 B/op	      81 allocs/op
BenchmarkNativeCatalogNavigation/100000/browse-10        	      63	  17912481 ns/op	     19668 response-bytes	 5982764 B/op	     477 allocs/op
BenchmarkNativeCatalogNavigation/100000/browse-10        	      68	  18419137 ns/op	     19668 response-bytes	 5829864 B/op	     476 allocs/op
BenchmarkNativeCatalogNavigation/100000/browse-10        	      64	  18017084 ns/op	     19668 response-bytes	 5831358 B/op	     477 allocs/op
BenchmarkNativeCatalogNavigation/100000/browse-10        	      64	  18236449 ns/op	     19668 response-bytes	 5831358 B/op	     477 allocs/op
BenchmarkNativeCatalogNavigation/100000/browse-10        	      68	  17394561 ns/op	     19668 response-bytes	 5832372 B/op	     477 allocs/op
BenchmarkNativeCatalogNavigation/100000/search-10        	      54	  21798268 ns/op	       321.0 response-bytes	 4820289 B/op	      81 allocs/op
BenchmarkNativeCatalogNavigation/100000/search-10        	      54	  21914420 ns/op	       321.0 response-bytes	 4820289 B/op	      81 allocs/op
BenchmarkNativeCatalogNavigation/100000/search-10        	      56	  21733874 ns/op	       321.0 response-bytes	 4820314 B/op	      81 allocs/op
BenchmarkNativeCatalogNavigation/100000/search-10        	      55	  21800389 ns/op	       321.0 response-bytes	 4820288 B/op	      81 allocs/op
BenchmarkNativeCatalogNavigation/100000/search-10        	      49	  21977041 ns/op	       321.0 response-bytes	 4820298 B/op	      81 allocs/op
BenchmarkLargeLibraryBrowse-10                           	     346	   3308886 ns/op	     25319 response-bytes	 1446400 B/op	   11564 allocs/op
BenchmarkLargeLibraryBrowse-10                           	     393	   3072979 ns/op	     25319 response-bytes	 1446038 B/op	   11564 allocs/op
BenchmarkLargeLibraryBrowse-10                           	     379	   3198098 ns/op	     25319 response-bytes	 1446146 B/op	   11564 allocs/op
BenchmarkLargeLibraryBrowse-10                           	     394	   3115556 ns/op	     25319 response-bytes	 1446004 B/op	   11564 allocs/op
BenchmarkLargeLibraryBrowse-10                           	     396	   3092951 ns/op	     25319 response-bytes	 1445991 B/op	   11564 allocs/op
BenchmarkLargeLibraryKnownTitleNavigation/search-10      	     380	   3157318 ns/op	  996265 B/op	    8961 allocs/op
BenchmarkLargeLibraryKnownTitleNavigation/search-10      	     374	   3146874 ns/op	  996225 B/op	    8961 allocs/op
BenchmarkLargeLibraryKnownTitleNavigation/search-10      	     372	   3133311 ns/op	  996225 B/op	    8961 allocs/op
BenchmarkLargeLibraryKnownTitleNavigation/search-10      	     384	   3221089 ns/op	  996250 B/op	    8961 allocs/op
BenchmarkLargeLibraryKnownTitleNavigation/search-10      	     386	   3140063 ns/op	  996210 B/op	    8961 allocs/op
BenchmarkLargeLibraryKnownTitleNavigation/letter-10      	     520	   2315122 ns/op	 1446174 B/op	   11579 allocs/op
BenchmarkLargeLibraryKnownTitleNavigation/letter-10      	     508	   2482894 ns/op	 1444290 B/op	   11579 allocs/op
BenchmarkLargeLibraryKnownTitleNavigation/letter-10      	     459	   2515814 ns/op	 1444299 B/op	   11579 allocs/op
BenchmarkLargeLibraryKnownTitleNavigation/letter-10      	     463	   2453535 ns/op	 1444309 B/op	   11579 allocs/op
BenchmarkLargeLibraryKnownTitleNavigation/letter-10      	     499	   2505889 ns/op	 1444302 B/op	   11579 allocs/op
PASS
```

`warm-candidate-reverse.log`:

```text
goos: darwin
goarch: arm64
pkg: github.com/MikeO7/kinosail-player/internal/server
cpu: Apple M1 Pro
BenchmarkNativeCatalogNavigation/10000/browse-10         	     897	   1365255 ns/op	     19286 response-bytes	  782613 B/op	     476 allocs/op
BenchmarkNativeCatalogNavigation/10000/browse-10         	     951	   1298281 ns/op	     19286 response-bytes	  781729 B/op	     476 allocs/op
BenchmarkNativeCatalogNavigation/10000/browse-10         	     961	   1315621 ns/op	     19286 response-bytes	  781721 B/op	     476 allocs/op
BenchmarkNativeCatalogNavigation/10000/search-10         	     560	   2225827 ns/op	       316.0 response-bytes	  494822 B/op	      81 allocs/op
BenchmarkNativeCatalogNavigation/10000/search-10         	     570	   2131853 ns/op	       316.0 response-bytes	  494818 B/op	      81 allocs/op
BenchmarkNativeCatalogNavigation/10000/search-10         	     570	   2139724 ns/op	       316.0 response-bytes	  494818 B/op	      81 allocs/op
BenchmarkNativeCatalogNavigation/100000/browse-10        	      66	  17028607 ns/op	     19668 response-bytes	 5975696 B/op	     477 allocs/op
BenchmarkNativeCatalogNavigation/100000/browse-10        	      72	  16320642 ns/op	     19668 response-bytes	 5832089 B/op	     477 allocs/op
BenchmarkNativeCatalogNavigation/100000/browse-10        	      74	  16182957 ns/op	     19668 response-bytes	 5828503 B/op	     476 allocs/op
BenchmarkNativeCatalogNavigation/100000/search-10        	      55	  21037752 ns/op	       321.0 response-bytes	 4820316 B/op	      81 allocs/op
BenchmarkNativeCatalogNavigation/100000/search-10        	      56	  21006740 ns/op	       321.0 response-bytes	 4820342 B/op	      81 allocs/op
BenchmarkNativeCatalogNavigation/100000/search-10        	      56	  21257287 ns/op	       321.0 response-bytes	 4820342 B/op	      81 allocs/op
BenchmarkLargeLibraryBrowse-10                           	     403	   2990536 ns/op	     25319 response-bytes	 1446022 B/op	   11564 allocs/op
BenchmarkLargeLibraryBrowse-10                           	     405	   2915955 ns/op	     25319 response-bytes	 1445934 B/op	   11564 allocs/op
BenchmarkLargeLibraryBrowse-10                           	     568	   2144677 ns/op	     25319 response-bytes	 1445269 B/op	   11564 allocs/op
BenchmarkLargeLibraryKnownTitleNavigation/search-10      	     416	   2904268 ns/op	  996273 B/op	    8961 allocs/op
BenchmarkLargeLibraryKnownTitleNavigation/search-10      	     412	   2894885 ns/op	  996224 B/op	    8961 allocs/op
BenchmarkLargeLibraryKnownTitleNavigation/search-10      	     415	   2968297 ns/op	  996260 B/op	    8961 allocs/op
BenchmarkLargeLibraryKnownTitleNavigation/letter-10      	     524	   2229779 ns/op	 1446168 B/op	   11579 allocs/op
BenchmarkLargeLibraryKnownTitleNavigation/letter-10      	     552	   2323275 ns/op	 1444302 B/op	   11579 allocs/op
BenchmarkLargeLibraryKnownTitleNavigation/letter-10      	     555	   2243983 ns/op	 1444304 B/op	   11579 allocs/op
PASS
```

`warm-baseline-reverse.log`:

```text
goos: darwin
goarch: arm64
pkg: github.com/MikeO7/kinosail-player/internal/server
cpu: Apple M1 Pro
BenchmarkNativeCatalogNavigation/10000/search-10         	     534	   2175355 ns/op	       316.0 response-bytes	  494862 B/op	      81 allocs/op
BenchmarkNativeCatalogNavigation/10000/search-10         	     560	   2173689 ns/op	       316.0 response-bytes	  494818 B/op	      81 allocs/op
BenchmarkNativeCatalogNavigation/10000/search-10         	     565	   2165643 ns/op	       316.0 response-bytes	  494818 B/op	      81 allocs/op
BenchmarkNativeCatalogNavigation/10000/browse-10         	     238	   5071086 ns/op	     19286 response-bytes	 1767709 B/op	     484 allocs/op
BenchmarkNativeCatalogNavigation/10000/browse-10         	     242	   5094279 ns/op	     19286 response-bytes	 1767316 B/op	     484 allocs/op
BenchmarkNativeCatalogNavigation/10000/browse-10         	     238	   4973035 ns/op	     19286 response-bytes	 1766970 B/op	     484 allocs/op
BenchmarkNativeCatalogNavigation/100000/browse-10        	      20	  58159012 ns/op	     19668 response-bytes	15455324 B/op	     486 allocs/op
BenchmarkNativeCatalogNavigation/100000/browse-10        	      20	  57462354 ns/op	     19668 response-bytes	15451061 B/op	     485 allocs/op
BenchmarkNativeCatalogNavigation/100000/browse-10        	      20	  57503023 ns/op	     19668 response-bytes	15459590 B/op	     488 allocs/op
BenchmarkNativeCatalogNavigation/100000/search-10        	      56	  21154104 ns/op	       321.0 response-bytes	 4820287 B/op	      81 allocs/op
BenchmarkNativeCatalogNavigation/100000/search-10        	      57	  21228295 ns/op	       321.0 response-bytes	 4820312 B/op	      81 allocs/op
BenchmarkNativeCatalogNavigation/100000/search-10        	      56	  21113423 ns/op	       321.0 response-bytes	 4820342 B/op	      81 allocs/op
BenchmarkLargeLibraryBrowse-10                           	     194	   6138244 ns/op	     25319 response-bytes	 2428513 B/op	   11573 allocs/op
BenchmarkLargeLibraryBrowse-10                           	     193	   6133596 ns/op	     25319 response-bytes	 2428467 B/op	   11572 allocs/op
BenchmarkLargeLibraryBrowse-10                           	     188	   6252821 ns/op	     25319 response-bytes	 2428499 B/op	   11572 allocs/op
BenchmarkLargeLibraryKnownTitleNavigation/search-10      	     402	   2905706 ns/op	  996247 B/op	    8961 allocs/op
BenchmarkLargeLibraryKnownTitleNavigation/search-10      	     420	   2894863 ns/op	  996223 B/op	    8961 allocs/op
BenchmarkLargeLibraryKnownTitleNavigation/search-10      	     406	   2896358 ns/op	  996234 B/op	    8961 allocs/op
BenchmarkLargeLibraryKnownTitleNavigation/letter-10      	     196	   6108398 ns/op	 2429260 B/op	   11587 allocs/op
BenchmarkLargeLibraryKnownTitleNavigation/letter-10      	     196	   6053974 ns/op	 2429207 B/op	   11587 allocs/op
BenchmarkLargeLibraryKnownTitleNavigation/letter-10      	     192	   6243578 ns/op	 2429236 B/op	   11587 allocs/op
PASS
```

`cold-baseline.log`:

```text
goos: darwin
goarch: arm64
pkg: github.com/MikeO7/kinosail-player/internal/server
cpu: Apple M1 Pro
BenchmarkNativeCatalogColdBrowse/all-10         	      20	  55071692 ns/op	     19664 response-bytes	15461038 B/op	     504 allocs/op
BenchmarkNativeCatalogColdBrowse/all-10         	      20	  54226800 ns/op	     19664 response-bytes	15464143 B/op	     489 allocs/op
BenchmarkNativeCatalogColdBrowse/all-10         	      20	  54473985 ns/op	     19664 response-bytes	15459878 B/op	     488 allocs/op
BenchmarkNativeCatalogColdBrowse/all-10         	      20	  54724786 ns/op	     19664 response-bytes	15464143 B/op	     489 allocs/op
BenchmarkNativeCatalogColdBrowse/all-10         	      20	  54546508 ns/op	     19664 response-bytes	15468410 B/op	     490 allocs/op
BenchmarkNativeCatalogColdBrowse/movies-10      	      20	  58767135 ns/op	     19682 response-bytes	15464144 B/op	     489 allocs/op
BenchmarkNativeCatalogColdBrowse/movies-10      	      20	  58416354 ns/op	     19682 response-bytes	15459878 B/op	     488 allocs/op
BenchmarkNativeCatalogColdBrowse/movies-10      	      20	  58714710 ns/op	     19682 response-bytes	15451352 B/op	     486 allocs/op
BenchmarkNativeCatalogColdBrowse/movies-10      	      20	  66546462 ns/op	     19682 response-bytes	15472671 B/op	     492 allocs/op
BenchmarkNativeCatalogColdBrowse/movies-10      	      20	  59403029 ns/op	     19682 response-bytes	15472671 B/op	     492 allocs/op
BenchmarkNativeCatalogColdBrowse/books-10       	      20	  11817562 ns/op	       324.0 response-bytes	 4827714 B/op	      87 allocs/op
BenchmarkNativeCatalogColdBrowse/books-10       	      20	  11734527 ns/op	       324.0 response-bytes	 4827714 B/op	      87 allocs/op
BenchmarkNativeCatalogColdBrowse/books-10       	      20	  12033038 ns/op	       324.0 response-bytes	 4827792 B/op	      88 allocs/op
BenchmarkNativeCatalogColdBrowse/books-10       	      20	  11623563 ns/op	       324.0 response-bytes	 4827792 B/op	      88 allocs/op
BenchmarkNativeCatalogColdBrowse/books-10       	      20	  11725835 ns/op	       324.0 response-bytes	 4827792 B/op	      88 allocs/op
PASS
```

`cold-candidate.log`:

```text
goos: darwin
goarch: arm64
pkg: github.com/MikeO7/kinosail-player/internal/server
cpu: Apple M1 Pro
BenchmarkNativeCatalogColdBrowse/all-10         	      20	  53476106 ns/op	     19664 response-bytes	15457296 B/op	     509 allocs/op
BenchmarkNativeCatalogColdBrowse/all-10         	      20	  52851079 ns/op	     19664 response-bytes	15464657 B/op	     495 allocs/op
BenchmarkNativeCatalogColdBrowse/all-10         	      20	  51994202 ns/op	     19664 response-bytes	15468920 B/op	     496 allocs/op
BenchmarkNativeCatalogColdBrowse/all-10         	      20	  52514314 ns/op	     19664 response-bytes	15460392 B/op	     494 allocs/op
BenchmarkNativeCatalogColdBrowse/all-10         	      20	  52923971 ns/op	     19664 response-bytes	15468920 B/op	     496 allocs/op
BenchmarkNativeCatalogColdBrowse/movies-10      	      20	  56057167 ns/op	     19682 response-bytes	15464382 B/op	     492 allocs/op
BenchmarkNativeCatalogColdBrowse/movies-10      	      20	  56418910 ns/op	     19682 response-bytes	15464382 B/op	     492 allocs/op
BenchmarkNativeCatalogColdBrowse/movies-10      	      20	  56338208 ns/op	     19682 response-bytes	15464382 B/op	     492 allocs/op
BenchmarkNativeCatalogColdBrowse/movies-10      	      20	  55931285 ns/op	     19682 response-bytes	15455855 B/op	     490 allocs/op
BenchmarkNativeCatalogColdBrowse/movies-10      	      20	  56100892 ns/op	     19682 response-bytes	15464382 B/op	     492 allocs/op
BenchmarkNativeCatalogColdBrowse/books-10       	      20	  11774957 ns/op	       324.0 response-bytes	 4828016 B/op	      91 allocs/op
BenchmarkNativeCatalogColdBrowse/books-10       	      20	  11657631 ns/op	       324.0 response-bytes	 4827939 B/op	      90 allocs/op
BenchmarkNativeCatalogColdBrowse/books-10       	      20	  11376929 ns/op	       324.0 response-bytes	 4827860 B/op	      90 allocs/op
BenchmarkNativeCatalogColdBrowse/books-10       	      20	  11458029 ns/op	       324.0 response-bytes	 4828016 B/op	      91 allocs/op
BenchmarkNativeCatalogColdBrowse/books-10       	      20	  11245158 ns/op	       324.0 response-bytes	 4828016 B/op	      91 allocs/op
PASS
```

`cold-candidate-reverse.log`:

```text
goos: darwin
goarch: arm64
pkg: github.com/MikeO7/kinosail-player/internal/server
cpu: Apple M1 Pro
BenchmarkNativeCatalogColdBrowse/all-10         	      20	  53099173 ns/op	     19664 response-bytes	15474361 B/op	     513 allocs/op
BenchmarkNativeCatalogColdBrowse/all-10         	      20	  52437196 ns/op	     19664 response-bytes	15468918 B/op	     496 allocs/op
BenchmarkNativeCatalogColdBrowse/all-10         	      20	  53574929 ns/op	     19664 response-bytes	15460392 B/op	     494 allocs/op
BenchmarkNativeCatalogColdBrowse/movies-10      	      20	  57415559 ns/op	     19682 response-bytes	15464382 B/op	     492 allocs/op
BenchmarkNativeCatalogColdBrowse/movies-10      	      20	  56111000 ns/op	     19682 response-bytes	15464382 B/op	     492 allocs/op
BenchmarkNativeCatalogColdBrowse/movies-10      	      20	  56441477 ns/op	     19682 response-bytes	15472909 B/op	     495 allocs/op
BenchmarkNativeCatalogColdBrowse/books-10       	      20	  11375656 ns/op	       324.0 response-bytes	 4827938 B/op	      90 allocs/op
BenchmarkNativeCatalogColdBrowse/books-10       	      20	  11595685 ns/op	       324.0 response-bytes	 4828017 B/op	      91 allocs/op
BenchmarkNativeCatalogColdBrowse/books-10       	      20	  11680435 ns/op	       324.0 response-bytes	 4828016 B/op	      91 allocs/op
PASS
```

`cold-baseline-reverse.log`:

```text
goos: darwin
goarch: arm64
pkg: github.com/MikeO7/kinosail-player/internal/server
cpu: Apple M1 Pro
BenchmarkNativeCatalogColdBrowse/all-10         	      20	  56101904 ns/op	     19664 response-bytes	15465320 B/op	     505 allocs/op
BenchmarkNativeCatalogColdBrowse/all-10         	      20	  54416450 ns/op	     19664 response-bytes	15468407 B/op	     490 allocs/op
BenchmarkNativeCatalogColdBrowse/all-10         	      20	  58417833 ns/op	     19664 response-bytes	15468410 B/op	     490 allocs/op
BenchmarkNativeCatalogColdBrowse/movies-10      	      20	  57432808 ns/op	     19682 response-bytes	15472671 B/op	     492 allocs/op
BenchmarkNativeCatalogColdBrowse/movies-10      	      20	  57026838 ns/op	     19682 response-bytes	15459880 B/op	     488 allocs/op
BenchmarkNativeCatalogColdBrowse/movies-10      	      20	  56638075 ns/op	     19682 response-bytes	15464144 B/op	     489 allocs/op
BenchmarkNativeCatalogColdBrowse/books-10       	      20	  11668794 ns/op	       324.0 response-bytes	 4827714 B/op	      87 allocs/op
BenchmarkNativeCatalogColdBrowse/books-10       	      20	  11825677 ns/op	       324.0 response-bytes	 4827636 B/op	      87 allocs/op
BenchmarkNativeCatalogColdBrowse/books-10       	      20	  11616090 ns/op	       324.0 response-bytes	 4827792 B/op	      88 allocs/op
PASS
```

## CPU samples


`cpu-top.txt`:

```text
File: baseline.test
Type: cpu
Time: 2026-09-30 09:37:14 MDT
Duration: 3.57s, Total samples = 3460ms (96.82%)
Active filters:
   focus=BenchmarkNativeCatalogNavigation|catalog\.
Showing nodes accounting for 2140ms, 61.85% of 3460ms total
Dropped 30 nodes (cum <= 17.30ms)
Showing top 16 nodes out of 67
      flat  flat%   sum%        cum   cum%
     420ms 12.14% 12.14%      420ms 12.14%  runtime.memmove
     230ms  6.65% 18.79%     1850ms 53.47%  github.com/MikeO7/kinosail/packages/catalog.sortTitles
     160ms  4.62% 23.41%      160ms  4.62%  cmpbody
     160ms  4.62% 28.03%      370ms 10.69%  github.com/MikeO7/kinosail/packages/catalog.itemCandidates
     160ms  4.62% 32.66%      320ms  9.25%  github.com/MikeO7/kinosail/packages/catalog.sortTitles.func1
     150ms  4.34% 36.99%      190ms  5.49%  github.com/MikeO7/kinosail/packages/catalog.browseCandidates
     140ms  4.05% 41.04%      260ms  7.51%  runtime.typedmemmove
     130ms  3.76% 44.80%      130ms  3.76%  strings.TrimSpace
     120ms  3.47% 48.27%      160ms  4.62%  golang.org/x/text/collate.(*Collator).keyFromElems
      90ms  2.60% 50.87%      150ms  4.34%  golang.org/x/text/internal/colltab.(*Table).appendNext
      80ms  2.31% 53.18%      410ms 11.85%  golang.org/x/text/internal/colltab.(*Iter).Next
      80ms  2.31% 55.49%       80ms  2.31%  golang.org/x/text/internal/colltab.(*Iter).done (inline)
      80ms  2.31% 57.80%      340ms  9.83%  internal/reflectlite.typedmemmove
      60ms  1.73% 59.54%      330ms  9.54%  golang.org/x/text/internal/colltab.(*Iter).appendNext
      40ms  1.16% 60.69%      170ms  4.91%  github.com/MikeO7/kinosail/packages/catalog.Browse.ApplyAccess.func1
      40ms  1.16% 61.85%       40ms  1.16%  github.com/MikeO7/kinosail/packages/catalog.viewMatches
```

`candidate-cpu-top.txt`:

```text
File: candidate.test
Type: cpu
Time: 2026-09-30 11:04:56 MDT
Duration: 3.76s, Total samples = 3790ms (100.92%)
Showing nodes accounting for 3130ms, 82.59% of 3790ms total
Dropped 62 nodes (cum <= 18.95ms)
Showing top 16 nodes out of 113
      flat  flat%   sum%        cum   cum%
     610ms 16.09% 16.09%     1470ms 38.79%  github.com/MikeO7/kinosail/packages/catalog.itemCandidates
     560ms 14.78% 30.87%      670ms 17.68%  github.com/MikeO7/kinosail/packages/catalog.browseCandidates
     360ms  9.50% 40.37%      360ms  9.50%  strings.TrimSpace
     250ms  6.60% 46.97%      420ms 11.08%  runtime.concatstrings
     190ms  5.01% 51.98%      280ms  7.39%  runtime.tryDeferToSpanScan
     170ms  4.49% 56.46%      180ms  4.75%  github.com/MikeO7/kinosail/packages/library.Policy.Allows
     160ms  4.22% 60.69%      160ms  4.22%  runtime.memmove
     150ms  3.96% 64.64%      410ms 10.82%  runtime.scanObjectsSmall
     130ms  3.43% 68.07%      130ms  3.43%  runtime.madvise
     120ms  3.17% 71.24%      120ms  3.17%  runtime.pthread_cond_signal
      90ms  2.37% 73.61%      450ms 11.87%  github.com/MikeO7/kinosail/packages/catalog.titleLetterWithCaser
      90ms  2.37% 75.99%       90ms  2.37%  github.com/MikeO7/kinosail/packages/catalog.viewMatches
      80ms  2.11% 78.10%      500ms 13.19%  runtime.concatstring3
      60ms  1.58% 79.68%      340ms  8.97%  github.com/MikeO7/kinosail/packages/catalog.ProfileProgress (inline)
      60ms  1.58% 81.27%       60ms  1.58%  runtime.(*spanScanOwnership).or (inline)
      50ms  1.32% 82.59%      660ms 17.41%  github.com/MikeO7/kinosail/packages/catalog.Browse.ApplyAccess.func1
```

## Reconciled checks

Reconciliation commit: `25e2a2f2` (merge of main `2a62a537af0b817f77010360da319168c5f8baed`). Every recorded catalog and benchmark source hash matched after reconciliation. Root source caps and both architecture-snapshot checks passed.

```text
cd apps/player
go test ./internal/server -run 'Test(CatalogHTTPPublication|LargeLibrary|LargeLetterBucket|ExplicitSortTitle|LetterJump|KnownTitleSearch|Navigation)' -count=1
PASS: internal/server, 5.098s

cd apps/subtitles
go test ./internal/server -run 'Test(LetterJump|ExplicitSortTitle|KnownTitleSearch|Navigation)' -count=1
PASS: internal/server, 6.535s
```

These focused checks cover the adapter and shell intersections. The source-identical catalog suite and race proof remain valid. Hosted quick Go and populated-browser jobs verify the combined revision before merge. No new native build, physical frame capture, Nox deployment, or production-network test is claimed for this server change.
