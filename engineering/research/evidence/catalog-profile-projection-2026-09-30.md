# Catalog profile projection experiment

Date: September 30, 2026. Baseline: `96818c5e5b66796ab1a03a416e7b98c490225052`.

This experiment measures the shared server path used by web, iOS, and tvOS catalog requests. It evaluates in-process Go HTTP handlers with a trusted synthetic Viewer context. Routing and authentication middleware are outside timing. It does not measure network latency, native view updates, physical frames, or deployed load.

## Finding and implementation

The earlier title-order cache removed repeated collation. The remaining browse path still built a full Candidate array and read both list and playback state for every visible item. Production-length composite keys expose costs that the older short-ID benchmark did not.

The baseline warm 100,000-item Owner profile allocates about 15.45 MB and 200,579 objects per request. Its CPU profile attributes 53.47% cumulative sampled CPU to `itemCandidates`. Shared callees overlap; percentages must not be added. Profiles include fixture setup and an untimed cache seed, so their percentages are not pure steady-state request attribution.

The candidate builds an owned item-reference array for views that do not need history timestamps. Static-view projection does not read list or progress state. List reads membership; unwatched reads watched state. History retains its timestamp snapshot but skips unused list membership. Visibility is evaluated before state selection. The existing Owner fallback remains authoritative. The response adapter still reads progress for returned page items.

Metadata search, ordering, grouping, letter generation, and paging run after releasing both profile locks. Search compacts only request-owned references. Public `Browse.Apply` still copies caller candidates. Cancellation polling and generation-bound cache admission remain intact. No personalized response cache or additional retained budget is introduced.

The [Go garbage-collector guide](https://go.dev/doc/gc-guide#Eliminating_heap_allocations) identifies allocation rate as a driver of collection frequency and recommends heap profiles to locate allocation sites. This supports measuring reduced allocation traffic. It does not establish a latency improvement for every Kinosail workload. Collector settings remain unchanged.

## Fixture and controls

The new [HTTP benchmark](../../../apps/player/internal/server/catalog_projection_benchmark_test.go) uses 10,000 and 100,000 synthetic movies, 16-character hexadecimal item IDs, and a 26-character profile ID. Two libraries alternate items. List and progress records cover 20% of visible items; watched records cover 10%. Owner cases include legacy progress fallback. Restricted profiles see one library and must ignore additional legacy watched records.

Every workload validates HTTP status, total count, page size, and every returned ID's range and visibility. Exact search must return the last item. Validation runs outside timing. Response byte counts match each paired control.

Warm workloads seed complete all-title order through an Owner request before timing. Restricted cases therefore assume that complete order already exists. They do not prove that a restricted profile or a movie subset can seed shared order in a mixed catalog. Cold workloads construct a fresh index for each request outside timing. Warm timing includes recorder construction; cold timing excludes it. Compare each against its matching baseline, rather than treating warm and cold as identical timed work.

Both binaries were compiled before timing, from the same benchmark and unchanged dependencies. The baseline uses the exact baseline production owner. No task-owned build, test, or lint job overlaps timing. Unrelated host work remains running. Forward runs use baseline then candidate; reverse runs use candidate then baseline. Interleaved controls use baseline, candidate, candidate, baseline for each selected case.

Wall time varies substantially on this shared host. A mid-run snapshot recorded load averages above the ten logical CPUs and substantial compressed memory. It cannot identify the cause of an individual timing change. Initial history and unwatched samples include regressions; longer interleaved controls and profiles must remain visible beside them. Allocation reductions are more consistent than wall-time changes. These observations do not certify tail latency or device smoothness.

## Behavior coverage

Before production edits, the new public access contract passed on the baseline. Its literal expected outputs cover views, added ordering, show metadata, Unicode search, letters, visibility, fresh list and watched state, Owner legacy fallback, explicit current-state override, and caller reference ownership. This is a preservation contract, not a claimed preexisting functional bug.

Existing public contracts cover invalid browse input before loading or profile access, cancellation during projection with both locks released, concurrent publication, locale ordering, page boundaries, cache invalidation, and restricted cache admission. This change introduces no input or parser. Independent read-only review found no concrete correctness issue and did not run tests separately.

## Reproduction

Compile baseline and candidate binaries from one task checkout. Copy this benchmark into the baseline source state. Only `packages/catalog/browse_apply.go` differs in measured production source. Restore any temporary file replacement in a `finally` block. Never overwrite another task's work.

```sh
go -C apps/player test ./internal/server -run '^$' -c -o ../../.verification/catalog-projection/candidate.test
.verification/catalog-projection/candidate.test -test.run '^$' -test.bench '^BenchmarkNativeCatalogProfileProjection$' -test.benchtime 500ms -test.count 5
.verification/catalog-projection/candidate.test -test.run '^$' -test.bench '^BenchmarkNativeCatalogProfileCold$' -test.benchtime 10x -test.count 3
.verification/catalog-projection/candidate.test -test.run '^$' -test.bench '^BenchmarkNativeCatalogProfileProjection$/^100000$/^owner=true$/^all$' -test.benchtime 3s -test.cpuprofile .verification/catalog-projection/candidate-cpu.pprof -test.memprofile .verification/catalog-projection/candidate-heap.pprof
```

Run the same commands for the baseline binary. Repeat in reverse order. For interleaved cases, use two samples of two seconds per process. The raw record includes all samples, including slower candidate samples. Go profiles and original logs remain in the private `.verification/catalog-projection` directory. Source and binary bindings below make the durable record auditable.

## Matched results

All values below are medians. The [raw CSV](catalog-profile-projection-2026-09-30.csv) retains 584 individual samples. Batch names identify execution order. Warm forward/reverse samples use five/three samples per binary; cold uses three/two. Legacy uses three in each direction. Each interleaved aggregate contains four samples per binary.

### Warm controls

| Case | Forward baseline → candidate (ms) | Reverse baseline → candidate (ms) | Forward bytes/op | Forward allocations/op |
| --- | ---: | ---: | ---: | ---: |
| `10000/owner=true/all` | 5.890 → 1.839 | 3.296 → 2.236 | 1,754,619 → 391,611 | 20,576 → 575 |
| `10000/owner=true/movies` | 4.702 → 2.148 | 2.996 → 2.274 | 1,754,166 → 391,531 | 20,576 → 575 |
| `10000/owner=true/list` | 3.792 → 3.127 | 3.744 → 2.940 | 1,691,484 → 874,271 | 20,576 → 10,575 |
| `10000/owner=true/unwatched` | 3.543 → 4.428 | 3.178 → 6.194 | 1,746,568 → 872,647 | 20,576 → 10,575 |
| `10000/owner=true/history` | 5.058 → 7.409 | 5.162 → 4.882 | 2,155,691 → 1,674,670 | 22,587 → 12,587 |
| `10000/owner=true/exact` | 4.201 → 6.633 | 5.080 → 3.548 | 1,455,019 → 175,492 | 20,082 → 81 |
| `10000/owner=false/all` | 2.068 → 3.259 | 2.201 → 1.812 | 1,225,200 → 384,458 | 10,476 → 475 |
| `10000/owner=false/movies` | 2.110 → 3.203 | 2.570 → 1.617 | 1,224,973 → 383,625 | 10,476 → 475 |
| `10000/owner=false/list` | 1.978 → 4.119 | 2.685 → 1.912 | 1,193,854 → 625,767 | 10,476 → 5,475 |
| `10000/owner=false/unwatched` | 2.130 → 5.555 | 2.077 → 2.251 | 1,225,504 → 624,840 | 10,476 → 5,475 |
| `10000/owner=false/history` | 2.529 → 7.090 | 3.532 → 3.463 | 1,430,066 → 1,190,052 | 11,487 → 6,487 |
| `10000/owner=false/exact` | 2.815 → 5.746 | 2.940 → 2.962 | 974,898 → 175,405 | 10,081 → 80 |
| `100000/owner=true/all` | 35.428 → 30.063 | 42.409 → 19.091 | 15,445,885 → 1,838,005 | 200,577 → 576 |
| `100000/owner=true/movies` | 42.140 → 22.113 | 42.255 → 16.010 | 15,447,569 → 1,837,188 | 200,578 → 576 |
| `100000/owner=true/list` | 36.080 → 34.315 | 35.576 → 36.916 | 14,812,031 → 6,642,684 | 200,578 → 100,576 |
| `100000/owner=true/unwatched` | 37.169 → 50.608 | 40.633 → 36.680 | 15,356,878 → 6,644,386 | 200,576 → 100,577 |
| `100000/owner=true/history` | 52.579 → 65.990 | 52.876 → 69.441 | 19,334,478 → 14,506,112 | 220,593 → 120,587 |
| `100000/owner=true/exact` | 57.140 → 58.865 | 61.173 → 59.092 | 14,420,688 → 1,617,607 | 200,083 → 82 |
| `100000/owner=false/all` | 23.762 → 22.326 | 28.288 → 16.244 | 10,233,552 → 1,828,925 | 100,477 → 476 |
| `100000/owner=false/movies` | 26.869 → 21.480 | 40.890 → 19.797 | 10,234,077 → 1,828,664 | 100,477 → 476 |
| `100000/owner=false/list` | 31.759 → 33.525 | 23.385 → 21.700 | 9,916,078 → 4,231,319 | 100,477 → 50,476 |
| `100000/owner=false/unwatched` | 27.743 → 37.890 | 27.020 → 25.423 | 10,196,810 → 4,231,284 | 100,478 → 50,476 |
| `100000/owner=false/history` | 36.060 → 49.473 | 31.073 → 45.093 | 12,187,503 → 9,783,444 | 110,490 → 60,489 |
| `100000/owner=false/exact` | 39.952 → 36.732 | 40.248 → 29.945 | 9,620,651 → 1,617,438 | 100,083 → 81 |

### Cold controls

| Case | Forward baseline → candidate (ms) | Reverse baseline → candidate (ms) | Forward bytes/op | Forward allocations/op |
| --- | ---: | ---: | ---: | ---: |
| `owner=true/movies` | 106.268 → 66.726 | 98.151 → 66.957 | 37,927,698 → 24,301,937 | 300,603 → 100,600 |
| `owner=true/list` | 58.187 → 44.800 | 74.328 → 35.818 | 19,341,024 → 11,185,511 | 220,593 → 120,594 |
| `owner=true/exact` | 60.207 → 34.949 | 61.739 → 42.953 | 14,421,772 → 1,618,252 | 200,084 → 81 |
| `owner=false/movies` | 55.443 → 44.355 | 57.374 → 44.199 | 21,483,927 → 13,062,397 | 150,495 → 50,489 |
| `owner=false/list` | 33.827 → 29.408 | 34.391 → 27.273 | 12,211,864 → 6,518,060 | 110,495 → 60,491 |
| `owner=false/exact` | 37.249 → 26.600 | 40.504 → 36.062 | 9,621,392 → 1,618,312 | 100,083 → 82 |

### Legacy controls

| Case | Forward baseline → candidate (ms) | Reverse baseline → candidate (ms) | Forward bytes/op | Forward allocations/op |
| --- | ---: | ---: | ---: | ---: |
| `NativeCatalogNavigation/10000/browse` | 2.212 → 1.314 | 1.956 → 1.201 | 782,600 → 379,943 | 476 → 475 |
| `NativeCatalogNavigation/10000/search` | 3.239 → 2.752 | 3.369 → 2.930 | 494,833 → 175,325 | 81 → 80 |
| `NativeCatalogNavigation/100000/browse` | 25.527 → 12.970 | 25.025 → 13.543 | 5,837,221 → 1,825,325 | 478 → 476 |
| `NativeCatalogNavigation/100000/search` | 49.164 → 40.986 | 36.159 → 28.713 | 4,820,548 → 1,617,452 | 82 → 81 |
| `LargeLibraryBrowse` | 4.920 → 3.075 | 4.268 → 4.453 | 1,451,306 → 1,047,867 | 11,564 → 11,563 |
| `LargeLibraryKnownTitleNavigation/search` | 5.015 → 3.720 | 6.413 → 3.980 | 996,229 → 676,768 | 8,961 → 8,960 |
| `LargeLibraryKnownTitleNavigation/letter` | 3.385 → 2.827 | 3.511 → 2.516 | 1,444,340 → 1,042,859 | 11,579 → 11,578 |

### Interleaved 100,000-item controls

| Profile/view | Baseline → candidate (ms) | Bytes/op | Allocations/op |
| --- | ---: | ---: | ---: |
| `owner=true/all` | 157.502 → 23.728 | 15,459,353 → 1,833,870 | 200,580 → 575 |
| `owner=false/all` | 78.000 → 37.944 | 10,236,088 → 1,827,666 | 100,478 → 475 |
| `owner=true/exact` | 139.475 → 55.222 | 14,420,669 → 1,617,375 | 200,083 → 81 |
| `owner=true/history` | 188.690 → 164.564 | 19,322,869 → 14,523,131 | 220,590 → 120,591 |
| `owner=false/history` | 138.739 → 111.400 | 12,184,086 → 9,785,563 | 110,489 → 60,489 |
| `owner=true/unwatched` | 224.733 → 149.114 | 15,378,985 → 6,636,421 | 200,580 → 100,575 |
| `owner=false/unwatched` | 105.888 → 88.552 | 10,196,706 → 4,230,166 | 100,478 → 50,476 |

The ordinary warm Owner browse allocates about 88% fewer bytes and over 99% fewer objects. Restricted browsing allocates about 82% fewer bytes. Exact search removes nearly all state-key objects while retaining its metadata scan. History allocates about 25% fewer bytes for Owner and 20% fewer for restricted profiles. List and unwatched retain their required state lookup and allocate roughly half as many objects.

Initial warm 100,000-item history medians are slower in both orders. Several initial 10,000-item controls and unwatched samples also regress. Interleaved controls favor the candidate, but run during substantial host pressure. These contrasting timings do not support a single latency percentage for this change. Cold movie/list/exact controls favor the candidate in both directions; their synthetic results remain distinct from application startup and physical presentation.

The final Owner/all CPU profile leaves `accessItems` at 53.33% cumulative sampled CPU and letter generation at 14.25%. Cumulative percentages overlap. The next targets are visibility projection, letter reuse, and state-key construction. History still sorts timestamps and allocates title-order keys for its selected items. No performance ceiling has been established.

## Source and environment binding

environment.json

```json
{
  "recorded_at": "2026-09-30T18:30:20.994351+00:00",
  "go": "go version go1.27.1 darwin/arm64",
  "os": "macOS-27.0-arm64-arm-64bit-Mach-O",
  "cpu": "Apple M1 Pro",
  "logical_cpus": "10",
  "baseline_binary_sha256": "2be9db0da2f276d71213e3234311f4c05deb091cd78b667f6ca8fdb129e367f1",
  "benchmark_id_lengths": {
    "profile": 26,
    "item": 16
  },
  "fixture": "synthetic; two libraries; 20 percent progress/list; 10 percent watched; Owner legacy fallback; restricted non-Owner",
  "host": "shared M1 Pro; unrelated activity preserved"
}
```

binary-hashes.json

```json
[
  {
    "binary": "baseline.test",
    "sha256": "2be9db0da2f276d71213e3234311f4c05deb091cd78b667f6ca8fdb129e367f1"
  },
  {
    "binary": "candidate.test",
    "sha256": "cdbb1886ba9351d2f9091dbcc45b736a227b986ca6036b2e24c5b265b7763ed5"
  }
]
```

baseline-source.json

```json
{
  "head": "96818c5e5b66796ab1a03a416e7b98c490225052",
  "files": [
    {
      "path": "packages/catalog/browse.go",
      "sha256": "5f32935c388ddc040fa4069ce352e1b50bad55d329fa5978b0ff9796ee2575af"
    },
    {
      "path": "packages/catalog/browse_apply.go",
      "sha256": "0a425888c57297c4b9366da120cf12f31589d80ba39b1524b375c22b58a648cc"
    },
    {
      "path": "packages/catalog/index_order.go",
      "sha256": "4fd125ad973395e562a327435435bce55e129fcb941f0e022c2d6683d25a9f5a"
    },
    {
      "path": "packages/catalog/browse_sort.go",
      "sha256": "cbc6c6343910e88f1bb8475f3c9a4796a03a3c1b458906bb5865cf0e8fb2b066"
    },
    {
      "path": "packages/catalog/letters.go",
      "sha256": "3cc7f5877c528d52d393138ca0e0f07fa0b6963f349f740526d180db0b14df85"
    },
    {
      "path": "packages/catalog/browse_access_test.go",
      "sha256": "37d840508398cb9981f745fb6c59f13e46eb9395bb9beaf35da7081d4659a23d"
    },
    {
      "path": "apps/player/internal/server/browse.go",
      "sha256": "a13ebeb8994d5e39e721c390a2f2e92f7ca2391b9065f9a35061a27ab60d8c2d"
    },
    {
      "path": "apps/player/internal/server/catalog_projection_benchmark_test.go",
      "sha256": "c382b6a05ddec25583668a4fc3a20ea650718a328a8d04ee9523dbdb96a59d53"
    },
    {
      "path": "apps/player/internal/server/catalog_performance_benchmark_test.go",
      "sha256": "be7d1a6d6d4e7f618673c5602d68431cb62ceeb626706f17cb2d8b29454f401b"
    },
    {
      "path": "packages/library/access.go",
      "sha256": "766643c2ecabe018a38230bcd88ccde68e2f24c2e2d0d2ab391d01382c6ed9e1"
    },
    {
      "path": "packages/identitycore/profile.go",
      "sha256": "3265ddca470c7a66044813adaf752ec45e14a766993205f768f8e3d2ce8e8c91"
    }
  ]
}
```

candidate-source.json

```json
{
  "parent": "96818c5e5b66796ab1a03a416e7b98c490225052",
  "files": [
    {
      "path": "packages/catalog/browse.go",
      "sha256": "5f32935c388ddc040fa4069ce352e1b50bad55d329fa5978b0ff9796ee2575af"
    },
    {
      "path": "packages/catalog/browse_apply.go",
      "sha256": "7f11bae4a56c93473e547f6ce47f1fcc5ab31a5ba08c3851efb80292363d0e32"
    },
    {
      "path": "packages/catalog/index_order.go",
      "sha256": "4fd125ad973395e562a327435435bce55e129fcb941f0e022c2d6683d25a9f5a"
    },
    {
      "path": "packages/catalog/browse_sort.go",
      "sha256": "cbc6c6343910e88f1bb8475f3c9a4796a03a3c1b458906bb5865cf0e8fb2b066"
    },
    {
      "path": "packages/catalog/letters.go",
      "sha256": "3cc7f5877c528d52d393138ca0e0f07fa0b6963f349f740526d180db0b14df85"
    },
    {
      "path": "packages/catalog/browse_access_test.go",
      "sha256": "37d840508398cb9981f745fb6c59f13e46eb9395bb9beaf35da7081d4659a23d"
    },
    {
      "path": "apps/player/internal/server/browse.go",
      "sha256": "a13ebeb8994d5e39e721c390a2f2e92f7ca2391b9065f9a35061a27ab60d8c2d"
    },
    {
      "path": "apps/player/internal/server/catalog_projection_benchmark_test.go",
      "sha256": "c382b6a05ddec25583668a4fc3a20ea650718a328a8d04ee9523dbdb96a59d53"
    },
    {
      "path": "apps/player/internal/server/catalog_performance_benchmark_test.go",
      "sha256": "be7d1a6d6d4e7f618673c5602d68431cb62ceeb626706f17cb2d8b29454f401b"
    },
    {
      "path": "packages/library/access.go",
      "sha256": "766643c2ecabe018a38230bcd88ccde68e2f24c2e2d0d2ab391d01382c6ed9e1"
    },
    {
      "path": "packages/identitycore/profile.go",
      "sha256": "3265ddca470c7a66044813adaf752ec45e14a766993205f768f8e3d2ce8e8c91"
    }
  ]
}
```

The raw CSV SHA-256 is `6ca006bbcf60105c703273183e3e037dd234cb5b5d8f58099fb768f74ad5ed12`.

### Recorded host pressure

```text
13:00  up 2 days,  4:21, 2 users, load averages: 24.38 20.57 14.71
Mach Virtual Memory Statistics: (page size of 16384 bytes)
Pages free:                                    17906.
Pages active:                                 700769.
Pages inactive:                               682169.
Pages speculative:                             17792.
Pages throttled:                                   0.
Pages wired down:                             306377.
Pages purgeable:                               25778.
"Translation faults":                     4180783940.
Pages copy-on-write:                       296865808.
Pages zero filled:                        2006851417.
Pages reactivated:                         179349129.
Pages purged:                               15077075.
File-backed pages:                            513912.
Anonymous pages:                              886818.
Pages stored in compressor:                   737491.
Pages occupied by compressor:                 331048.
Decompressions:                            110971457.
Compressions:                              151758651.
Pageins:                                    86219199.
Pageouts:                                     133191.
Swapins:                                      216665.
Swapouts:                                     848684.
2026-09-30T19:00:31.844653+00:00
command: uptime
command: vm_stat
```

## Measurement sequence

```python
from pathlib import Path
import datetime, hashlib, json, subprocess, time

root = Path.cwd()
out = root / ".verification/catalog-projection"
paths = [entry["path"] for entry in json.loads((out / "baseline-source.json").read_text())["files"]]
(out / "candidate-source.json").write_text(json.dumps({"parent": subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip(), "files": [{"path": path, "sha256": hashlib.sha256((root / path).read_bytes()).hexdigest()} for path in paths]}, indent=2) + "\n")
(out / "binary-hashes.json").write_text(json.dumps([{ "binary": name, "sha256": hashlib.sha256((out / name).read_bytes()).hexdigest()} for name in ["baseline.test", "candidate.test"]], indent=2) + "\n")
commands = []

def run(binary, label, pattern, duration, count):
    command = [str(out / (binary + ".test")), "-test.run", "^$", "-test.bench", pattern, "-test.benchtime", duration, "-test.count", str(count)]
    start = time.time()
    with (out / (label + ".log")).open("w") as log:
        subprocess.run(command, cwd=root / "apps/player", stdout=log, stderr=subprocess.STDOUT, check=True)
    commands.append({"label": label, "command": command, "cwd": str(root / "apps/player"), "started_unix": start, "finished_unix": time.time()})
    (out / "measurement-commands.json").write_text(json.dumps(commands, indent=2) + "\n")
    print(label, "complete", flush=True)

warm = "^BenchmarkNativeCatalogProfileProjection$"
run("baseline", "warm-baseline", warm, "500ms", 5)
run("candidate", "warm-candidate", warm, "500ms", 5)
run("candidate", "warm-candidate-reverse", warm, "500ms", 3)
run("baseline", "warm-baseline-reverse", warm, "500ms", 3)
cold = "^BenchmarkNativeCatalogProfileCold$"
run("baseline", "cold-baseline", cold, "10x", 3)
run("candidate", "cold-candidate", cold, "10x", 3)
run("candidate", "cold-candidate-reverse", cold, "10x", 2)
run("baseline", "cold-baseline-reverse", cold, "10x", 2)
legacy = "^Benchmark(NativeCatalogNavigation|LargeLibraryBrowse|LargeLibraryKnownTitleNavigation)$"
run("baseline", "legacy-baseline", legacy, "500ms", 3)
run("candidate", "legacy-candidate", legacy, "500ms", 3)
run("candidate", "legacy-candidate-reverse", legacy, "500ms", 3)
run("baseline", "legacy-baseline-reverse", legacy, "500ms", 3)
```

```python
import json, subprocess, time
from pathlib import Path
root=Path('.verification/catalog-projection')
cases=[('owner=true','all'),('owner=false','all'),('owner=true','exact'),('owner=true','history'),('owner=false','history'),('owner=true','unwatched'),('owner=false','unwatched')]
records=[]
for owner,view in cases:
 for n,variant in enumerate(['baseline','candidate','candidate','baseline']):
  label='interleaved-'+owner.replace('=','-')+'-'+view+'-'+str(n)+'-'+variant
  cmd=[str(root/(variant+'.test')),'-test.run','^$','-test.bench','^BenchmarkNativeCatalogProfileProjection$/^100000$/^'+owner+'$/^'+view+'$','-test.benchtime','2s','-test.count','2']
  started=time.time()
  with (root/(label+'.log')).open('w') as output:
   subprocess.run(cmd,stdout=output,stderr=subprocess.STDOUT,check=True)
  records.append({'label':label,'command':cmd,'start_unix':started,'end_unix':time.time()})
  (root/'interleaved-commands.json').write_text(json.dumps(records,indent=2)+'\n')
 print(owner,view,'complete',flush=True)
```

## CPU samples

baseline-cpu-top.txt

```text
File: baseline.test
Type: cpu
Time: 2026-09-30 12:32:21 MDT
Duration: 3.96s, Total samples = 4470ms (112.90%)
Showing nodes accounting for 2840ms, 63.53% of 4470ms total
Dropped 62 nodes (cum <= 22.35ms)
Showing top 12 nodes out of 110
      flat  flat%   sum%        cum   cum%
     510ms 11.41% 11.41%     2390ms 53.47%  github.com/MikeO7/kinosail/packages/catalog.itemCandidates
     450ms 10.07% 21.48%      770ms 17.23%  runtime.mapaccess2_faststr
     380ms  8.50% 29.98%      450ms 10.07%  runtime.tryDeferToSpanScan
     330ms  7.38% 37.36%      330ms  7.38%  runtime.madvise
     280ms  6.26% 43.62%      820ms 18.34%  runtime.concatstrings
     220ms  4.92% 48.55%      620ms 13.87%  runtime.scanObjectsSmall
     180ms  4.03% 52.57%      180ms  4.03%  strings.TrimSpace
     120ms  2.68% 55.26%      120ms  2.68%  internal/runtime/maps.probeSeq.next (inline)
     100ms  2.24% 57.49%      100ms  2.24%  runtime.pthread_cond_signal
      90ms  2.01% 59.51%     1780ms 39.82%  github.com/MikeO7/kinosail/packages/catalog.Browse.ApplyAccess.func1
      90ms  2.01% 61.52%       90ms  2.01%  github.com/MikeO7/kinosail/packages/library.Policy.Allows
      90ms  2.01% 63.53%       90ms  2.01%  runtime.(*spanInlineMarkBits).init
```

candidate-cpu-top.txt

```text
File: candidate.test
Type: cpu
Time: 2026-09-30 12:55:00 MDT
Duration: 4.47s, Total samples = 4.35s (97.25%)
Showing nodes accounting for 3.99s, 91.72% of 4.35s total
Dropped 69 nodes (cum <= 0.02s)
Showing top 18 nodes out of 77
      flat  flat%   sum%        cum   cum%
     2.04s 46.90% 46.90%      2.32s 53.33%  github.com/MikeO7/kinosail/packages/catalog.Browse.accessItems
     0.31s  7.13% 54.02%      0.31s  7.13%  strings.TrimSpace
     0.22s  5.06% 59.08%      0.23s  5.29%  github.com/MikeO7/kinosail/packages/library.Policy.Allows
     0.22s  5.06% 64.14%      0.22s  5.06%  runtime.madvise
     0.19s  4.37% 68.51%      0.47s 10.80%  runtime.scanObjectsSmall
     0.18s  4.14% 72.64%      0.29s  6.67%  runtime.tryDeferToSpanScan
     0.17s  3.91% 76.55%      0.62s 14.25%  github.com/MikeO7/kinosail/packages/catalog.browseLetters
     0.10s  2.30% 78.85%      0.41s  9.43%  github.com/MikeO7/kinosail/packages/catalog.titleLetterWithCaser
     0.09s  2.07% 80.92%      0.09s  2.07%  runtime.(*spanScanOwnership).or (inline)
     0.09s  2.07% 82.99%      0.09s  2.07%  runtime.memmove
     0.09s  2.07% 85.06%      0.10s  2.30%  runtime.pthread_cond_signal
     0.05s  1.15% 86.21%      0.05s  1.15%  runtime.extractHeapBitsSmall
     0.05s  1.15% 87.36%      0.15s  3.45%  runtime.scanObject
     0.04s  0.92% 88.28%      0.04s  0.92%  github.com/MikeO7/kinosail/packages/catalog.viewMatches
     0.04s  0.92% 89.20%      0.04s  0.92%  runtime.greyobject
     0.04s  0.92% 90.11%      0.04s  0.92%  runtime.memequal
     0.04s  0.92% 91.03%      0.04s  0.92%  runtime.usleep
     0.03s  0.69% 91.72%      0.03s  0.69%  runtime.memclrNoHeapPointers
```

baseline-history-cpu-top.txt

```text
File: baseline.test
Type: cpu
Time: 2026-09-30 12:55:52 MDT
Duration: 6.24s, Total samples = 4.21s (67.42%)
Showing nodes accounting for 3.29s, 78.15% of 4.21s total
Dropped 79 nodes (cum <= 0.02s)
Showing top 20 nodes out of 108
      flat  flat%   sum%        cum   cum%
     0.84s 19.95% 19.95%      0.84s 19.95%  runtime.madvise
     0.39s  9.26% 29.22%      1.40s 33.25%  github.com/MikeO7/kinosail/packages/catalog.itemCandidates
     0.39s  9.26% 38.48%      0.54s 12.83%  runtime.tryDeferToSpanScan
     0.27s  6.41% 44.89%      0.84s 19.95%  runtime.scanObjectsSmall
     0.22s  5.23% 50.12%      0.22s  5.23%  runtime.(*spanInlineMarkBits).init
     0.17s  4.04% 54.16%      0.51s 12.11%  runtime.concatstrings
     0.15s  3.56% 57.72%      0.39s  9.26%  runtime.mapaccess2_faststr
     0.11s  2.61% 60.33%      0.40s  9.50%  github.com/MikeO7/kinosail/packages/catalog.sortTitles
     0.08s  1.90% 62.23%      0.08s  1.90%  internal/runtime/maps.(*groupReference).key (inline)
     0.08s  1.90% 64.13%      0.08s  1.90%  runtime.(*spanScanOwnership).or (inline)
     0.08s  1.90% 66.03%      0.08s  1.90%  runtime.spanSetScans
     0.07s  1.66% 67.70%      0.07s  1.66%  internal/runtime/maps.probeSeq.next (inline)
     0.07s  1.66% 69.36%      0.07s  1.66%  runtime.pthread_cond_signal
     0.06s  1.43% 70.78%      0.07s  1.66%  github.com/MikeO7/kinosail-player/internal/server.projectionBenchmarkFixture
     0.06s  1.43% 72.21%      0.06s  1.43%  github.com/MikeO7/kinosail/packages/library.Policy.Allows
     0.06s  1.43% 73.63%      0.08s  1.90%  golang.org/x/text/collate.(*Collator).keyFromElems
     0.06s  1.43% 75.06%      0.06s  1.43%  runtime.(*mspan).init
     0.06s  1.43% 76.48%      0.06s  1.43%  runtime.memmove
     0.04s  0.95% 77.43%      0.12s  2.85%  runtime.scanObject
     0.03s  0.71% 78.15%      0.94s 22.33%  github.com/MikeO7/kinosail/packages/catalog.Browse.ApplyAccess.func1
```

candidate-history-cpu-top.txt

```text
File: candidate.test
Type: cpu
Time: 2026-09-30 12:56:40 MDT
Duration: 5s, Total samples = 4240ms (84.79%)
Showing nodes accounting for 3110ms, 73.35% of 4240ms total
Dropped 90 nodes (cum <= 21.20ms)
Showing top 20 nodes out of 105
      flat  flat%   sum%        cum   cum%
     730ms 17.22% 17.22%      730ms 17.22%  runtime.madvise
     490ms 11.56% 28.77%     1310ms 30.90%  github.com/MikeO7/kinosail/packages/catalog.itemCandidates
     310ms  7.31% 36.08%      430ms 10.14%  runtime.tryDeferToSpanScan
     290ms  6.84% 42.92%      760ms 17.92%  runtime.scanObjectsSmall
     230ms  5.42% 48.35%      590ms 13.92%  github.com/MikeO7/kinosail/packages/catalog.sortTitles
     150ms  3.54% 51.89%      150ms  3.54%  runtime.(*spanInlineMarkBits).init
      90ms  2.12% 54.01%      300ms  7.08%  runtime.mapaccess2_faststr
      80ms  1.89% 55.90%       80ms  1.89%  runtime.memmove
      80ms  1.89% 57.78%       80ms  1.89%  runtime.pthread_cond_signal
      70ms  1.65% 59.43%       70ms  1.65%  github.com/MikeO7/kinosail-player/internal/server.projectionBenchmarkFixture
      70ms  1.65% 61.08%       80ms  1.89%  golang.org/x/text/collate.(*Collator).keyFromElems
      70ms  1.65% 62.74%       70ms  1.65%  internal/runtime/maps.probeSeq.next (inline)
      60ms  1.42% 64.15%       70ms  1.65%  github.com/MikeO7/kinosail/packages/library.Policy.Allows
      60ms  1.42% 65.57%       60ms  1.42%  internal/runtime/maps.(*groupReference).key (inline)
      60ms  1.42% 66.98%       60ms  1.42%  runtime.(*spanScanOwnership).or (inline)
      60ms  1.42% 68.40%       60ms  1.42%  runtime.extractHeapBitsSmall
      60ms  1.42% 69.81%     1050ms 24.76%  runtime.gcDrain
      50ms  1.18% 70.99%       50ms  1.18%  runtime.(*mspan).init
      50ms  1.18% 72.17%      390ms  9.20%  runtime.concatstrings
      50ms  1.18% 73.35%      270ms  6.37%  runtime.mallocgcSmallNoScanSC5
```

## Allocation samples

These sampled totals include setup and cache seeding. Use benchmark bytes/op for request allocations.

baseline-heap-top.txt

```text
File: baseline.test
Type: alloc_space
Time: 2026-09-30 12:32:25 MDT
Showing nodes accounting for 2.06GB, 88.43% of 2.33GB total
Dropped 203 nodes (cum <= 0.01GB)
Showing top 12 nodes out of 77
      flat  flat%   sum%        cum   cum%
    0.56GB 23.95% 23.95%     1.08GB 46.43%  github.com/MikeO7/kinosail/packages/catalog.Browse.ApplyAccess.func1
    0.52GB 22.48% 46.43%     0.52GB 22.48%  github.com/MikeO7/kinosail/packages/catalog.ProfileProgress (inline)
    0.45GB 19.38% 65.81%     1.53GB 65.81%  github.com/MikeO7/kinosail/packages/catalog.itemCandidates
    0.09GB  4.06% 69.87%     0.09GB  4.06%  strings.(*Builder).WriteString
    0.09GB  3.93% 73.80%     0.09GB  3.93%  github.com/MikeO7/kinosail/packages/catalog.candidateReferences
    0.09GB  3.77% 77.57%     0.09GB  3.77%  github.com/MikeO7/kinosail/packages/catalog.(*Index).titleReferences
    0.08GB  3.39% 80.95%     0.08GB  3.58%  github.com/MikeO7/kinosail-player/internal/server.projectionBenchmarkFixture
    0.07GB  2.97% 83.93%     0.07GB  3.15%  github.com/MikeO7/kinosail/packages/catalog.NewMemoryIndex
    0.04GB  1.53% 85.46%     0.04GB  1.53%  text/template/parse.(*Tree).newText
    0.03GB  1.13% 86.59%     0.03GB  1.13%  internal/bytealg.MakeNoZero
    0.02GB  1.03% 87.62%     0.02GB  1.03%  text/template/parse.(*ListNode).append
    0.02GB  0.82% 88.43%     0.02GB  0.82%  text/template/parse.(*Tree).newPipeline
```

candidate-heap-top.txt

```text
File: candidate.test
Type: alloc_space
Time: 2026-09-30 12:55:04 MDT
Showing nodes accounting for 651.22MB, 74.00% of 879.99MB total
Dropped 174 nodes (cum <= 4.40MB)
Showing top 12 nodes out of 104
      flat  flat%   sum%        cum   cum%
  126.02MB 14.32% 14.32%   126.02MB 14.32%  github.com/MikeO7/kinosail/packages/catalog.Browse.accessItems
  117.23MB 13.32% 27.64%   117.23MB 13.32%  github.com/MikeO7/kinosail/packages/catalog.(*Index).titleReferences
  102.65MB 11.66% 39.31%   102.65MB 11.66%  strings.(*Builder).WriteString
   78.18MB  8.88% 48.19%    82.68MB  9.40%  github.com/MikeO7/kinosail-player/internal/server.projectionBenchmarkFixture
   70.96MB  8.06% 56.25%    75.58MB  8.59%  github.com/MikeO7/kinosail/packages/catalog.NewMemoryIndex
   34.01MB  3.86% 60.12%    34.01MB  3.86%  text/template/parse.(*Tree).newText
   24.46MB  2.78% 62.90%    24.46MB  2.78%  internal/bytealg.MakeNoZero
      24MB  2.73% 65.63%       24MB  2.73%  text/template/parse.(*Tree).newPipeline
   21.01MB  2.39% 68.01%    21.01MB  2.39%  text/template/parse.(*ListNode).append
      20MB  2.27% 70.29%       20MB  2.27%  github.com/nicksnyder/go-i18n/v2/i18n.setPluralTemplate
   16.50MB  1.88% 72.16%    16.50MB  1.88%  github.com/nicksnyder/go-i18n/v2/i18n.stringSubmap
   16.20MB  1.84% 74.00%    16.20MB  1.84%  github.com/MikeO7/kinosail/packages/catalog.browsePage
```

## Comment and formatting changes after measurement

Lint required explanations for the benchmark matrix and positive fixture-count conversion, plus formatting and a complexity explanation in the preservation contract. These changes affect comments and formatting only. The measured production owner remains byte-identical through reconciliation with `05da53145bc05235aa510fecc6392ec5928de061`. Measured benchmark code before comments is bound above; delivery file hashes are recorded here.

```json
[
  {
    "path": "packages/catalog/browse_apply.go",
    "sha256": "7f11bae4a56c93473e547f6ce47f1fcc5ab31a5ba08c3851efb80292363d0e32"
  },
  {
    "path": "packages/catalog/browse_access_test.go",
    "sha256": "01a09ac2b421f9aa3b02efda62e3ccd87be8099a4cf829db12b9ee1ac025ca11"
  },
  {
    "path": "apps/player/internal/server/catalog_projection_benchmark_test.go",
    "sha256": "200aad261d047e43d4f67f2a851c2798a82457056e7c438452cba09b6da5b8d5"
  }
]
```

## Local verification

| Check | Result and boundary |
| --- | --- |
| `go -C packages test -count=1 -p 2 ./...` | PASS on the final production owner before comment/formatting-only test changes. |
| `go -C packages test -race -count=1 ./catalog` | PASS before and after test formatting; final run 5.635 seconds. |
| `go -C apps/subtitles test -count=1 -p 2 ./...` | PASS after reconciliation, 241.577 seconds for the command. |
| First full Player run | FAIL. It overlapped reconciliation. The compiled stylesheet differed from the filesystem stylesheet. A maintenance assertion also found its cache file missing. Do not count this as final revision evidence. |
| Reconciled focused maintenance and stylesheet checks, `-count=3` | PASS, 26.031 seconds. Later fixture discrimination is recorded below. |
| `go -C apps/player test -count=1 -p 2 -parallel 2 ./...` | FAIL on reconciled source. Command, backup, configuration, and database packages passed. Server failed two real HLS-speed cases at their request deadlines. Other server tests did not report failures. |
| Isolated `TestRealHLSGenerationKeepsAheadOfSupportedPlaybackSpeeds` | PASS, 22.878 seconds across all five cases. This does not certify loaded-host throughput. |
| Changed-code lint against `05da53145bc05235aa510fecc6392ec5928de061` | Player, Subtitles, and packages PASS with zero issues. No production lint finding. |
| `make max-loc`, `git diff --check`, regenerated Code Atlas, `make tooling-check` | PASS. |

The first Player run's stylesheet mismatch was caused by changing source files while its previously compiled test was running. The fresh run uses one reconciled source state and resolves that mismatch. Its remaining HLS deadline failure is distinct from catalog correctness. The isolated HLS run passes without a playback-source change. Full hosted checks remain the required delivery authority; this record does not turn a focused retry into a full-suite pass.

The primary checkout and unrelated worktrees remain untouched. Local main cleanup is blocked by unrelated dirty primary work. The active performance checkout remains leased for the ongoing goal. Physical-device hitch timing, deployment, production-network load, and mixed-library title-order admission remain separate work.


## Maintenance fixture diagnosis and guard control

Post-commit checks ran at `01b5c8c8a5a20aa8d377e2f5b97b88d47d49ce7b`. Subtitles stopped at 112 existing shared lint findings. Player passed source caps, diff checks, and compilation, then failed its full server package because the maintenance fixture cache was missing. Player did not reach shared lint in that run.

The unchanged streaming fixture failed once in 30 repetitions. It created an over-budget cache before starting its stream, while enabling background scheduling. Startup upkeep can run after 250 milliseconds even with a one-hour interval. A controlled 350-millisecond pause removed the cache before stream admission in all three repetitions. This reproduces a fixture failure mechanism; it does not identify the exact timing of every earlier failure.

Seeding after stream admission initially passed 30 repetitions and the delayed control. Independent review identified a remaining admission race: an idle pass can observe the guard before streaming, then prune after the cache is seeded. The final fixture uses the existing nil-lifecycle configuration to isolate explicit HTTP upkeep from background scheduling. Separate lifecycle tests still exercise automatic scheduling. The blocked request keeps its test context. Cleanup releases its writer before waiting, including after assertion failures.

Final checks passed 30 repetitions of the streaming guard and the existing maintenance/lifecycle cases. A 350-millisecond pre-stream pause also passed three repetitions. Removing only the production `Manager.Run` busy guard caused all three repetitions to fail at the expected cache-preservation assertion. Production bytes were restored immediately. Three repetitions then passed with the guard restored. These controls preserve the test's ability to catch an actual guard regression. They make no maintenance throughput or device claim.

Final fixture SHA-256: `284483d727245b8823646b322b2e1453988b9767226c9aa425459bfe91df7d3a`. Independent read-only review confirmed the startup-race finding was resolved and found no further issue. The reviewer did not execute tests independently. Original failures and control logs remain in `.verification/catalog-projection`.


## Final fixed-revision Player suite

At commit `6b94c0f5764a00889f44bbb29e94515d791152f9`, `go -C apps/player test -count=1 -p 2 -parallel 2 ./...` passed every package. The command took 252.426 seconds; the server package took 247.834 seconds. The measured catalog owner and final maintenance fixture stayed byte-identical throughout the run. Earlier full-suite failures remain recorded above. This pass establishes local suite behavior on this revision, not physical playback, native frames, or production load.


The required post-commit Player `make verify-changed` then passed caps, diff checks, server compilation, and all selected maintenance tests. It stopped at the same 112 existing shared lint findings, after 69.727 seconds. Shared consumer, container, and later stages did not run through this command. Separate full Player/shared/Subtitles and catalog race results above remain distinct. Changed-code lint passed with zero issues after the final fixture edit. No gate was disabled or bypassed.
