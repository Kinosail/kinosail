# Mixed-library title-order experiment

Date: September 30, 2026. Baseline: `199bbcfaea2f6ccc15da68ae48ab8cbaa58f55b4`.

## Finding and implementation

Native Movies and Music requests select subsets of a mixed catalog. They could not seed the earlier complete-all title-order cache. Added-order home requests also do not seed title order. Repeated requests therefore continued collation work even after the [profile projection improvement](catalog-profile-projection-2026-09-30.md).

The candidate keys completed order by canonical locale and intrinsic view: movies, music, books, audiobooks, and photos. A complete Owner projection can seed its order. A restricted projection cannot seed an intrinsic order. Every request still projects current visibility; progress and list state remain fresh. Existing complete-all entries provide a fallback. Shows retain grouping after visibility and do not seed an order.

Admission retains generation and unique-ID checks. A cold Owner intrinsic request counts matching items in its captured snapshot after profile locks are released. The count polls cancellation every 64 items. Cancellation during optional admission skips caching and returns the already completed result. Search, history, added ordering, validation, public response shape, and existing cancellation during selection remain unchanged.

The cache keeps its four-entry limit and 1,048,576-reference budget. Admission uses full index size as the budget denominator because completed arrays retain full-index capacity. Intrinsic entries share that limit with complete-all entries. This can evict a different useful order. There is no personalized response cache or larger memory budget.

## Fixture and controls

The [HTTP benchmark](../../../apps/player/internal/server/catalog_mixed_benchmark_test.go) uses 10,000 and 100,000 items: half movies, one quarter episodes, and one quarter music. Two libraries alternate groups of four. Restricted profiles see one library. Show groups contain four episodes; each restricted group retains two. Item IDs have 16 hexadecimal characters; profile IDs have 26 characters. List and progress records populate 20% of items. Owner cases include legacy progress fallback.

English-locale titles begin with Ängel, Alpha, Åland, or Élan and use reverse numeric order. These Unicode-heavy synthetic titles create substantial letter-generation allocations. Results need confirmation against representative user metadata. Audiobooks, books, and photos have public behavior coverage, but this fixture does not benchmark them.

The benchmark calls the real native catalog JSON handler with a trusted Viewer context. It excludes routing, authentication middleware, network, storage, native rendering, and physical frames. Warm timing includes recorder construction. Cold timing excludes index and recorder construction. Each cold request uses a fresh index; warm cases use one seed request without an all-title seed. Guest cases distinguish a Guest seed from an Owner seed.

Every case checks status, total, page size, permitted IDs, intrinsic kinds, and body equality outside timing. All 30 case body hashes match across 824 retained samples. Both binaries use the same benchmark and dependencies. Forward controls run baseline then candidate; reverse controls run candidate then baseline. Each direction has three samples per case: 500 ms warm or 10 requests cold. Interleaved controls run baseline, candidate, candidate, baseline with two samples of two seconds per process.

The shared M1 Pro has ten logical CPUs and runs macOS 27 with Go 1.27.1. Other tasks remained running. A recorded load snapshot was 10.46/11.42/11.70. These medians are handler measurements, not tail-latency or smoothness certification. Allocation reductions are more consistent than timing.

## Rejected experiments and measurement limits

The first draft filtered cold references before profile projection. Guest-only Movies and Music became slower in both control directions. At 100,000 items, Movies changed from 28.895 to 35.947 ms forward and 28.037 to 32.776 ms reverse. Music changed from 20.967 to 23.256 ms and 19.583 to 22.628 ms. Draft CPU sampling attributes 10.21% cumulative CPU to titleReferences, including shared callees. It supports investigating the extra pass; it does not explain every timing difference. All 360 draft samples remain in the CSV. The shipped candidate preserves original cold reference construction and limits new admission work to Owner requests.

Guest-only Music remains uncertain. Its forward median favors the candidate; reverse and one interleaved median do not. An unchanged Shows path also slows sharply in one direction and recovers in reverse. Retain those observations rather than claiming a universal latency gain.

A further private overlay cleared captured reference slices when admission did not need them. Public contracts passed before timing. Its four-case interleaved controls show inconsistent timing and no allocation reduction. This lifetime proposal is rejected; final production source retains the simpler Owner-only candidate. Releasing references can affect garbage collection in principle, but these controls establish no dependable gain here.

Two Guest-only Movies batches overlapped cleanup of this task's rebuildable tvOS DerivedData. Their original rows remain. A separate clean recheck follows below. Its runner accidentally overwrote only the initial interleaved process ledger. Exact commands and order were recovered from the saved runner; process timestamps and durations were reconstructed approximately from log file times. Raw timed rows were intact. During lifetime controls, disk exhaustion interrupted one zero-byte log before a sample. The ledger retained nine complete processes. The empty log was preserved and the missing processes resumed with atomic ledger replacement. The pause and changing host load limit that comparison further.

## Results

All values are medians. The [CSV](catalog-mixed-title-order-2026-09-30.csv) keeps every individual row, including regressions and rejected proposals. Phase and variant identify which source was measured. The lifetime phase compares the Owner-only candidate against the rejected overlay, not against original main.

### Owner-only candidate: forward

| Case | baseline-final → candidate (ms) | Bytes/op | Allocations/op |
| --- | ---: | ---: | ---: |
| `10000/owner=false/movies/cold=false/seed-owner=false` | 7.786 → 4.233 | 2,247,578 → 2,246,368 | 11,737 → 11,737 |
| `10000/owner=false/movies/cold=false/seed-owner=true` | 6.673 → 3.156 | 2,246,158 → 1,383,265 | 11,737 → 9,225 |
| `10000/owner=false/movies/cold=true/seed-owner=false` | 3.982 → 4.879 | 2,244,264 → 2,244,264 | 11,734 → 11,734 |
| `10000/owner=false/music/cold=false/seed-owner=false` | 3.007 → 2.588 | 1,758,112 → 1,758,153 | 10,486 → 10,486 |
| `10000/owner=false/music/cold=false/seed-owner=true` | 3.279 → 1.456 | 1,758,230 → 1,341,541 | 10,486 → 9,225 |
| `10000/owner=false/music/cold=true/seed-owner=false` | 3.104 → 3.257 | 1,756,584 → 1,756,584 | 10,483 → 10,483 |
| `10000/owner=false/shows/cold=false/seed-owner=false` | 3.439 → 2.997 | 1,484,378 → 1,484,028 | 2,580 → 2,580 |
| `10000/owner=false/shows/cold=false/seed-owner=true` | 2.894 → 2.800 | 1,484,450 → 1,483,188 | 2,580 → 2,580 |
| `10000/owner=false/shows/cold=true/seed-owner=false` | 2.734 → 3.249 | 1,482,240 → 1,482,240 | 2,577 → 2,577 |
| `10000/owner=true/movies/cold=false/seed-owner=true` | 10.243 → 3.172 | 4,097,531 → 2,471,040 | 23,087 → 18,075 |
| `10000/owner=true/movies/cold=true/seed-owner=true` | 6.671 → 7.284 | 4,093,928 → 4,094,360 | 23,084 → 23,087 |
| `10000/owner=true/music/cold=false/seed-owner=true` | 8.018 → 3.276 | 3,130,750 → 2,430,533 | 20,586 → 18,075 |
| `10000/owner=true/music/cold=true/seed-owner=true` | 4.831 → 5.596 | 3,126,760 → 3,127,192 | 20,583 → 20,586 |
| `10000/owner=true/shows/cold=false/seed-owner=true` | 5.624 → 3.357 | 1,934,758 → 1,934,273 | 3,934 → 3,934 |
| `10000/owner=true/shows/cold=true/seed-owner=true` | 3.487 → 4.058 | 1,931,472 → 1,931,472 | 3,931 → 3,931 |
| `100000/owner=false/movies/cold=false/seed-owner=false` | 41.414 → 39.390 | 20,263,529 → 20,257,236 | 112,990 → 112,988 |
| `100000/owner=false/movies/cold=false/seed-owner=true` | 39.297 → 28.373 | 20,251,333 → 11,822,584 | 112,987 → 87,975 |
| `100000/owner=false/movies/cold=true/seed-owner=false` | 41.802 → 42.835 | 20,268,892 → 20,268,892 | 112,990 → 112,990 |
| `100000/owner=false/music/cold=false/seed-owner=false` | 30.349 → 26.605 | 15,444,519 → 15,450,777 | 100,487 → 100,489 |
| `100000/owner=false/music/cold=false/seed-owner=true` | 31.907 → 19.022 | 15,443,941 → 11,437,453 | 100,487 → 87,977 |
| `100000/owner=false/music/cold=true/seed-owner=false` | 28.821 → 27.785 | 15,457,629 → 15,466,158 | 100,489 → 100,491 |
| `100000/owner=false/shows/cold=false/seed-owner=false` | 26.209 → 33.425 | 12,148,242 → 12,156,952 | 19,484 → 19,485 |
| `100000/owner=false/shows/cold=false/seed-owner=true` | 36.045 → 32.291 | 12,161,341 → 12,149,260 | 19,486 → 19,484 |
| `100000/owner=false/shows/cold=true/seed-owner=false` | 33.944 → 28.118 | 12,172,173 → 12,172,173 | 19,486 → 19,486 |
| `100000/owner=true/movies/cold=false/seed-owner=true` | 59.759 → 40.807 | 38,670,523 → 22,639,613 | 225,590 → 175,577 |
| `100000/owner=true/movies/cold=true/seed-owner=true` | 89.130 → 72.261 | 38,681,028 → 38,704,250 | 225,590 → 225,598 |
| `100000/owner=true/music/cold=false/seed-owner=true` | 50.300 → 26.743 | 29,074,190 → 22,253,081 | 200,591 → 175,578 |
| `100000/owner=true/music/cold=true/seed-owner=true` | 39.710 → 53.698 | 29,074,882 → 29,086,708 | 200,589 → 200,594 |
| `100000/owner=true/shows/cold=false/seed-owner=true` | 33.571 → 65.420 | 16,501,876 → 16,502,232 | 32,118 → 32,118 |
| `100000/owner=true/shows/cold=true/seed-owner=true` | 42.526 → 36.056 | 16,514,184 → 16,514,186 | 32,118 → 32,118 |

### Owner-only candidate: reverse

| Case | baseline-final → candidate (ms) | Bytes/op | Allocations/op |
| --- | ---: | ---: | ---: |
| `10000/owner=false/movies/cold=false/seed-owner=false` | 3.896 → 6.294 | 2,245,833 → 2,245,303 | 11,737 → 11,737 |
| `10000/owner=false/movies/cold=false/seed-owner=true` | 4.018 → 3.505 | 2,245,720 → 1,382,720 | 11,737 → 9,225 |
| `10000/owner=false/movies/cold=true/seed-owner=false` | 4.648 → 3.924 | 2,244,264 → 2,244,264 | 11,734 → 11,734 |
| `10000/owner=false/music/cold=false/seed-owner=false` | 3.918 → 4.192 | 1,757,974 → 1,759,330 | 10,486 → 10,486 |
| `10000/owner=false/music/cold=false/seed-owner=true` | 2.604 → 1.802 | 1,757,534 → 1,341,986 | 10,486 → 9,225 |
| `10000/owner=false/music/cold=true/seed-owner=false` | 3.791 → 3.054 | 1,756,584 → 1,756,584 | 10,483 → 10,483 |
| `10000/owner=false/shows/cold=false/seed-owner=false` | 2.663 → 2.724 | 1,483,080 → 1,484,795 | 2,580 → 2,580 |
| `10000/owner=false/shows/cold=false/seed-owner=true` | 3.172 → 2.729 | 1,483,328 → 1,484,028 | 2,580 → 2,580 |
| `10000/owner=false/shows/cold=true/seed-owner=false` | 3.680 → 3.154 | 1,482,240 → 1,482,240 | 2,577 → 2,577 |
| `10000/owner=true/movies/cold=false/seed-owner=true` | 6.582 → 3.095 | 4,097,248 → 2,471,606 | 23,087 → 18,075 |
| `10000/owner=true/movies/cold=true/seed-owner=true` | 9.199 → 10.219 | 4,093,928 → 4,094,360 | 23,084 → 23,087 |
| `10000/owner=true/music/cold=false/seed-owner=true` | 4.905 → 3.702 | 3,130,313 → 2,430,130 | 20,586 → 18,075 |
| `10000/owner=true/music/cold=true/seed-owner=true` | 4.963 → 4.432 | 3,126,760 → 3,127,192 | 20,583 → 20,586 |
| `10000/owner=true/shows/cold=false/seed-owner=true` | 3.769 → 3.621 | 1,933,250 → 1,933,408 | 3,934 → 3,934 |
| `10000/owner=true/shows/cold=true/seed-owner=true` | 4.980 → 3.838 | 1,931,472 → 1,946,962 | 3,931 → 3,933 |
| `100000/owner=false/movies/cold=false/seed-owner=false` | 45.968 → 43.716 | 20,258,251 → 20,257,704 | 112,989 → 112,988 |
| `100000/owner=false/movies/cold=false/seed-owner=true` | 41.753 → 23.080 | 20,257,020 → 11,826,137 | 112,988 → 87,976 |
| `100000/owner=false/movies/cold=true/seed-owner=false` | 51.407 → 41.174 | 20,277,420 → 20,268,890 | 112,992 → 112,990 |
| `100000/owner=false/music/cold=false/seed-owner=false` | 30.356 → 31.388 | 15,449,513 → 15,444,776 | 100,488 → 100,487 |
| `100000/owner=false/music/cold=false/seed-owner=true` | 27.134 → 25.292 | 15,443,552 → 11,435,685 | 100,487 → 87,976 |
| `100000/owner=false/music/cold=true/seed-owner=false` | 34.689 → 56.106 | 15,466,153 → 15,457,628 | 100,491 → 100,489 |
| `100000/owner=false/shows/cold=false/seed-owner=false` | 27.820 → 28.293 | 12,171,621 → 12,150,824 | 19,488 → 19,484 |
| `100000/owner=false/shows/cold=false/seed-owner=true` | 35.447 → 29.760 | 12,161,342 → 12,159,199 | 19,486 → 19,486 |
| `100000/owner=false/shows/cold=true/seed-owner=false` | 38.565 → 32.744 | 12,172,175 → 12,187,662 | 19,486 → 19,489 |
| `100000/owner=true/movies/cold=false/seed-owner=true` | 79.385 → 36.222 | 38,673,840 → 22,649,030 | 225,590 → 175,579 |
| `100000/owner=true/movies/cold=true/seed-owner=true` | 86.548 → 72.463 | 38,692,425 → 38,704,271 | 225,592 → 225,598 |
| `100000/owner=true/music/cold=false/seed-owner=true` | 44.786 → 27.313 | 29,062,796 → 22,258,210 | 200,588 → 175,579 |
| `100000/owner=true/music/cold=true/seed-owner=true` | 46.618 → 46.586 | 29,086,278 → 29,075,316 | 200,591 → 200,592 |
| `100000/owner=true/shows/cold=false/seed-owner=true` | 39.182 → 32.547 | 16,498,968 → 16,508,488 | 32,117 → 32,119 |
| `100000/owner=true/shows/cold=true/seed-owner=true` | 64.896 → 38.771 | 16,529,676 → 16,514,188 | 32,121 → 32,118 |

### Interleaved 100,000-item controls

| Case | baseline-final → candidate (ms) | Bytes/op | Allocations/op |
| --- | ---: | ---: | ---: |
| `100000/owner=false/movies/cold=false/seed-owner=false` | 53.845 → 54.967 | 20,257,424 → 20,264,037 | 112,989 → 112,990 |
| `100000/owner=false/movies/cold=false/seed-owner=true` | 45.194 → 28.243 | 20,256,382 → 11,828,406 | 112,988 → 87,976 |
| `100000/owner=false/music/cold=false/seed-owner=false` | 37.822 → 47.180 | 15,447,580 → 15,448,132 | 100,488 → 100,488 |
| `100000/owner=false/music/cold=false/seed-owner=true` | 36.895 → 22.732 | 15,447,458 → 11,433,984 | 100,488 → 87,976 |
| `100000/owner=true/movies/cold=false/seed-owner=true` | 76.379 → 36.949 | 38,677,528 → 22,640,906 | 225,591 → 175,577 |
| `100000/owner=true/music/cold=false/seed-owner=true` | 55.112 → 26.727 | 29,066,512 → 22,246,776 | 200,589 → 175,577 |
| `100000/owner=true/shows/cold=false/seed-owner=true` | 37.544 → 37.927 | 16,495,246 → 16,493,739 | 32,117 → 32,116 |

### Clean Guest-only Movies recheck

| Case | baseline-final → candidate (ms) | Bytes/op | Allocations/op |
| --- | ---: | ---: | ---: |
| `100000/owner=false/movies/cold=false/seed-owner=false` | 111.769 → 105.857 | 20,266,700 → 20,263,888 | 112,992 → 112,990 |

### Guest-only Music recheck

| Case | baseline-final → candidate (ms) | Bytes/op | Allocations/op |
| --- | ---: | ---: | ---: |
| `100000/owner=false/music/cold=false/seed-owner=false` | 124.211 → 106.516 | 15,454,650 → 15,452,392 | 100,490 → 100,490 |

This separate ABBA recheck follows the interrupted lifetime proposal. It retains all samples and does not erase the earlier uncertain result. No task-owned build, lint, or cleanup overlaps its timing.

### Rejected lifetime proposal

| Case | candidate → lifetime (ms) | Bytes/op | Allocations/op |
| --- | ---: | ---: | ---: |
| `100000/owner=false/movies/cold=false/seed-owner=false` | 105.632 → 57.524 | 20,258,181 → 20,261,492 | 112,989 → 112,990 |
| `100000/owner=false/music/cold=false/seed-owner=false` | 36.018 → 75.932 | 15,448,294 → 15,448,862 | 100,488 → 100,488 |
| `100000/owner=true/movies/cold=false/seed-owner=true` | 25.794 → 28.219 | 22,643,421 → 22,642,612 | 175,578 → 175,578 |
| `100000/owner=true/music/cold=false/seed-owner=true` | 19.648 → 19.172 | 22,248,372 → 22,247,639 | 175,577 → 175,577 |

Warm Owner Movies allocates about 42% fewer bytes and 22% fewer objects. Music allocates about 23% fewer bytes and 12% fewer objects. Interleaved medians favor the candidate by about 52% in these two cases. Restricted warm requests benefit after an Owner seeds their intrinsic order. Guest-only requests have no new admission count or filtered miss pass; their timing remains noisy. Cold Owner requests add a completeness count and do not establish a uniform speedup.

## Behavior and verification

The new partial-view and late-publication contracts passed on original production source before their corresponding edits. The full public matrix covers all five intrinsic views, repeated hits before eviction, canonical locale ordering, ties, letters, URLs, caller ownership, fresh visibility/list/progress state, restricted first reads, partial Owner reads, and publication during projection. Removing completeness admission makes the partial-Owner contract fail. Removing generation admission makes the late-publication contract fail. Both controls use private overlays, without tracked source mutation.

Existing contracts preserve parser rejection before index loading or access, cancellation with profile locks released, and concurrent publication/readers. This change introduces no new input. Independent read-only production and artifact review found no material issue. It checked all 824 CSV samples and 73 reported median rows, but ran no tests or benchmarks itself.

Before the final count and key helper extractions, source-bound full Go checks passed: shared packages (131.255 seconds), catalog race (28.965), Player (292.190), and Subtitles (168.066). Source remained stable through those runs. Shared changed-code lint passed. Player lint first found test-only complexity and conversion findings. An ordinary retry hit the shared linter lock; task-owned temporary/cache directories isolated it. Final local gates, post-commit checks, and hosted delivery are recorded separately below.

## Reproduction and source binding

Build both binaries from a leased task checkout containing this benchmark. Use a Go overlay for original index.go and index_order.go from the baseline commit; keep all other source and dependencies identical. Do not overwrite production files for timing.

```sh
go -C apps/player test ./internal/server -run '^$' -c -o ../../.verification/catalog-mixed/candidate.test
# The baseline overlay maps only packages/catalog/index.go and index_order.go
# to files exported with git show 199bbcfaea2f6ccc15da68ae48ab8cbaa58f55b4:<path>.
go -C apps/player test -overlay ../../.verification/catalog-mixed/baseline-overlay.json ./internal/server -run '^$' -c -o ../../.verification/catalog-mixed/baseline-final.test
.verification/catalog-mixed/candidate.test -test.run '^$' -test.bench '^BenchmarkNativeMixedCatalog/.*/.*/.*/cold=false' -test.benchtime 500ms -test.count 3
.verification/catalog-mixed/candidate.test -test.run '^$' -test.bench '^BenchmarkNativeMixedCatalog/.*/.*/.*/cold=true' -test.benchtime 10x -test.count 3
```

Run matching commands for the baseline. Repeat in reverse order. For each selected interleaved case, use its complete slash-separated name, two-second samples, and two samples per process. Original logs, source overlays, command ledgers, CPU profiles, rejected source, and raw verification outputs remain in the private `.verification/catalog-mixed` directory.

The measured candidate binary predates a successful-path-neutral cancellation branch repair, later benchmark lint comments, and the final count and key helper extractions. countTitleView retains identical membership and 64-item cancellation checks. Warm hits do not call that helper. browseTitleOrderKey retains the existing eligibility conditions, locale limit, canonicalization, and intrinsic/all mapping. It only reads its borrowed Browse pointer. The paired timing tables identify the earlier measured binary; they are not new final-source latency measurements. Public tests were extended after timing. Full suites use the repaired production source. Final source hashes below bind those differences explicitly. The first source-cap run found the expanded test file above 300 lines. The two new admission tests moved into index_intrinsic_order_test.go with byte-identical bodies; final catalog and race checks cover that split. The original cap failure remains recorded. The first tooling run then detected the snapshot made before this test-file move. Both snapshots were regenerated for the retry.

Measured source and binaries:

```json
{
  "base": "199bbcfaea2f6ccc15da68ae48ab8cbaa58f55b4",
  "platform": "macOS-27.0-arm64-arm-64bit-Mach-O",
  "go": "go version go1.27.1 darwin/arm64",
  "load": "15:55  up 2 days,  7:16, 2 users, load averages: 10.46 11.42 11.70",
  "candidate_sources": {
    "packages/catalog/index.go": "ce9955e333e052c2b79d1e46a3069dec1cf4418168ea6962d9e4acb8d7d499a6",
    "packages/catalog/index_order.go": "28e5b6d015971d4894cec0a76e7573722c94b73dba97096fe3f7b9d261186dac",
    "packages/catalog/browse_apply.go": "7f11bae4a56c93473e547f6ce47f1fcc5ab31a5ba08c3851efb80292363d0e32",
    "apps/player/internal/server/catalog_mixed_benchmark_test.go": "b338fd5a1a4a6e60042bb6587405aa15cfaa7fafc37840d95a089d4918090519",
    "packages/catalog/index_order_test.go": "799a40e10b7dbaa5c0418bc508135bcf6c740c71845cba843955bf5fd110936a"
  },
  "baseline_overlay": {
    "packages/catalog/index.go": "09a5e95f3fa518b23f5604f12b7abdd6a136376b6338e94633ac3ab27a8e5377",
    "packages/catalog/index_order.go": "4fd125ad973395e562a327435435bce55e129fcb941f0e022c2d6683d25a9f5a"
  },
  "binary_hashes": {
    "baseline-final.test": "7a1cf60c270ef69c7316075a822c001bdb0b126ace790c9767de116a889122e0",
    "candidate.test": "1ec24dfb5bd38a77ac5e75d881b7f8aaaa1992b9dd4aa50058609b7f98e67b52"
  },
  "candidate": "Owner-only new intrinsic admission, original cold references"
}
```

Rejected filtered-miss source and binaries:

```json
{
  "base": "199bbcfaea2f6ccc15da68ae48ab8cbaa58f55b4",
  "platform": "macOS-27.0-arm64-arm-64bit-Mach-O",
  "go": "go version go1.27.1 darwin/arm64",
  "load": "15:44  up 2 days,  7:05, 2 users, load averages: 10.79 14.04 12.56",
  "candidate_sources": {
    "packages/catalog/index.go": "ce9955e333e052c2b79d1e46a3069dec1cf4418168ea6962d9e4acb8d7d499a6",
    "packages/catalog/index_order.go": "f64fe8749ade65177447d97982482a1d334343628d1ad76657f3a2ae2eea1ca6",
    "packages/catalog/browse_apply.go": "7f11bae4a56c93473e547f6ce47f1fcc5ab31a5ba08c3851efb80292363d0e32",
    "apps/player/internal/server/catalog_mixed_benchmark_test.go": "b338fd5a1a4a6e60042bb6587405aa15cfaa7fafc37840d95a089d4918090519",
    "packages/catalog/index_order_test.go": "35eb12a3a2c3a0636af230bd6c8d25c100a506fc295adb22b5410a1f3e72958a"
  },
  "baseline_overlay": {
    "packages/catalog/index.go": "09a5e95f3fa518b23f5604f12b7abdd6a136376b6338e94633ac3ab27a8e5377",
    "packages/catalog/index_order.go": "4fd125ad973395e562a327435435bce55e129fcb941f0e022c2d6683d25a9f5a"
  },
  "binary_hashes": {
    "baseline-final.test": "7a1cf60c270ef69c7316075a822c001bdb0b126ace790c9767de116a889122e0",
    "candidate.test": "a7aa7556053566fbb10eeec121fb6a06df37fb9895bc93716d27273d5d75c791"
  }
}
```

Rejected lifetime source and binaries:

```json
{
  "base": "199bbcfaea2f6ccc15da68ae48ab8cbaa58f55b4",
  "overlay_source_sha256": "8d6b27ad71188f13eb5106c1994f2783adf7032e92c0959958b57b92aae04aa1",
  "intermediate_index_order_sha256": "ed7ca0b2c16cc4a794937c229560319d04615f598419f929d9917c4548700823",
  "binaries": {
    "candidate": "1ec24dfb5bd38a77ac5e75d881b7f8aaaa1992b9dd4aa50058609b7f98e67b52",
    "lifetime": "97f29675fa8131ba7fdb05b00202081c2f2b964ba96d8a5974d78deaf74c6517"
  },
  "code_boundary": "intermediate timing binary predates only cancellation-branch lint repair and benchmark comment annotations"
}
```

Final task-owned source:

```json
{
  "packages/catalog/index.go": "ce9955e333e052c2b79d1e46a3069dec1cf4418168ea6962d9e4acb8d7d499a6",
  "packages/catalog/index_order.go": "7831e51ab5a5d886335b05f360df95f3879ee59b569d68a9fbc958e25fafbea1",
  "packages/catalog/index_order_test.go": "952b5164af47fb8d23ded0772fba794566a48afc4b2b305437600920a6529b81",
  "packages/catalog/index_intrinsic_order_test.go": "22579a94ec2f7c715145585e3e6c496cccffce12fb50cc0beb66be4ef6e3edab",
  "apps/player/internal/server/catalog_mixed_benchmark_test.go": "633b1b0d85b6618f6413b5e8e6f5abe03567834ef7bc138433316ad1bb0e4e65"
}
```

The [Go garbage-collector guide](https://go.dev/doc/gc-guide) explains pointer reachability and the distinction between allocated and retained memory. Its model states Go 1.19; it does not predict exact Go 1.27.1 timings. [Go 1.27 release notes](https://go.dev/doc/go1.27) describe size-specialized small allocations and the existing v1 JSON implementation's use of v2. Changing JSON API names alone is not evidence of another speedup. Collector settings and JSON APIs remain unchanged.

## Delivery boundary

Changed-code lint passed for shared packages, Player, and Subtitles. Both regenerated snapshots, the source cap, and repository tooling passed. The test-file split preserves both new test bodies byte-for-byte.

Reconciliation with origin/main `74e18c46afbb06b35cf835ace8556deb89eb7cd0` preserved all five task-owned Go files. The later merge of `1c0ddabe68724556390971409ccb9bbd50ae75e6` includes launch-readiness main `b28ccfebefca59744241c24821c83a09249f9919` and the native reader polish. It conflicted only in two generated snapshots. Regeneration preserves both tasks. Focused shared catalog/assets and both-app HTTP checks passed before the final key helper extraction. These checks do not certify unrelated native changes from main.

The first post-commit Player run passed compilation and failed only TestMCPStdioUsesSoleOwnerAndSharedAPI. Our long TMPDIR exceeded the Unix socket path limit. The isolated control reproduces bind: invalid argument there. The same test passes three times in a short task TMPDIR. The short-path retry passed the full Player server suite in 278 seconds, then stopped at shared lint.

Full lint first exposed cognitive complexity, then cyclomatic complexity on the same coordinator. The count and key helper extractions resolve both without production suppressions. Final full shared lint reports 112 findings outside index_order.go; changed-code lint reports zero issues in all three modules. Both app gate retries stopped at shared lint; later stages did not run.

The actual final source passes public catalog tests and race checks. A newly compiled binary passes all 30 one-iteration benchmark cases. Their response hashes match all 824 timed samples. The first comparison script omitted a case-name prefix and failed after both benchmark commands passed. The corrected parser checks saved output without rerunning or changing data. This preflight verifies behavior; the paired timing tables remain bound to their earlier binary.

Independent source review found no issue in either helper and ran no checks separately. The reconciled post-commit Player gate passed its full server suite in 313 seconds, then stopped at 112 shared lint findings. Subtitles stopped at the same lint stage. Later gate stages did not run. Exact hosted delivery results are recorded in [PR #394](https://github.com/Kinosail/kinosail/pull/394) and its linked Actions runs. Nox revision, deployed load, first-frame latency, and physical UI timing remain unverified for this phase. The overall performance goal stays active.

## Inherited date-ordering CI failure

The first reconciled hosted run failed TestCloneAndFormatAPIKeys as UTC crossed into October 1. APIKeyViews sorted the formatted Created date as text. Sep 29 incorrectly preceded Oct 1. This is an inherited production bug, not a reason to weaken the check.

A deterministic public regression fails on original source for day, month, and same-day ordering. It also covers year rollover. A one-line comparator repair uses original CreatedAt timestamps. Stable sorting retains ascending ID order for equal timestamps. Formatting and caller-owned maps remain unchanged. Both app adapters retain their read locks. Independent source review found no issue and ran no checks.

This ancillary repair fixes settings/API chronology. It is not evidence of catalog or UI speed gains. The failed hosted run and test-first regression remain recorded privately. Repaired-source verification passes: identity (3.660 seconds), identity/catalog race (12.250), Player API-key checks (33.911), Subtitles API-key checks (24.792), and the full shared suite (46.223). Changed-code shared lint, regenerated snapshot checks, source caps, and whitespace checks pass. All five catalog/preflight source hashes remain identical after this ancillary repair. Both repaired-revision post-commit app gates stopped at the same 112 shared lint findings. Player compilation and server stages reused cached results from the previous revision; these are not fresh full repaired-source app checks. The focused API checks and full shared suite above exercise the repair. Later gate stages did not run. Exact hosted results are recorded in [PR #394](https://github.com/Kinosail/kinosail/pull/394).

The duplicate API repair fingerprint block was removed from this note because the secret detector interpreted adjacent API-key filenames and hashes as credentials. Both values exactly match source SHA-256 fingerprints. They remain in the private validation records. The default secret-scanner configuration is unchanged.
